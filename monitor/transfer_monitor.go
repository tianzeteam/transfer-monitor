package monitor

import (
	"slices"
	"transfer_monitor/logger"
	"transfer_monitor/utils"

	"context"
	"fmt"
	"os"
	"sync"
	"time"
	goyser "transfer_monitor/yellowstone_geyser"
	yellowstone_geyser_pb "transfer_monitor/yellowstone_geyser/pb"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/go-redis/redis/v8"
	"github.com/mr-tron/base58"
)

// TransferEvent represents a transfer event from a monitored source address
type TransferEvent struct {
	Source      string
	Destination string
	Amount      uint64
	Signature   string
	Timestamp   time.Time
	TokenMint   string // Empty for SOL transfers
}

// TransferCallback is a function that will be called when a transfer is detected
type TransferCallback func(event TransferEvent)

// TransferMonitor monitors Solana transfers from specific source addresses
type TransferMonitor struct {
	geyserClient *goyser.Client
	streamClient *goyser.StreamClient
	logger       logger.Logger
	config       *utils.Config
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	running      bool
	runningMu    sync.Mutex

	// Source addresses to monitor
	sourceAddresses   map[string]bool
	sourceAddressesMu sync.RWMutex

	// Callbacks for transfer events
	callbacks   []TransferCallback
	callbacksMu sync.RWMutex

	// Redis client for storing destination accounts
	redisClient *redis.Client

	// In-memory cache of destination addresses
	destAddresses   []string // list of destination addresses
	destAddressesMu sync.RWMutex

	// Redis refresh interval
	refreshInterval time.Duration
}

// NewTransferMonitor creates a new TransferMonitor
func NewTransferMonitor(config *utils.Config, client *goyser.Client) (*TransferMonitor, error) {
	// Get logger instance
	log := logger.GetLogger()

	// Create context
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize Redis client
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // Default Redis address
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
		DB:   0, // Default DB
	})

	// Default refresh interval is 10 seconds
	refreshInterval := 10 * time.Second

	return &TransferMonitor{
		geyserClient:    client,
		logger:          log,
		config:          config,
		ctx:             ctx,
		cancel:          cancel,
		sourceAddresses: make(map[string]bool),
		callbacks:       make([]TransferCallback, 0),
		redisClient:     redisClient,
		destAddresses:   make([]string, 0),
		refreshInterval: refreshInterval,
	}, nil
}

// AddSourceAddress adds a source address to monitor
func (m *TransferMonitor) AddSourceAddress(address string) error {
	logger.Infof("监控source address：%s", address)
	// Validate the address
	_, err := solana.PublicKeyFromBase58(address)
	if err != nil {
		return fmt.Errorf("invalid Solana address: %w", err)
	}

	m.sourceAddressesMu.Lock()
	defer m.sourceAddressesMu.Unlock()

	m.sourceAddresses[address] = true

	// If already running, update subscription
	if m.streamClient != nil {
		err := m.updateSubscription()
		if err != nil {
			return fmt.Errorf("failed to update subscription: %w", err)
		}
	}

	return nil
}

// RemoveSourceAddress removes a source address from monitoring
func (m *TransferMonitor) RemoveSourceAddress(address string) {
	m.sourceAddressesMu.Lock()
	defer m.sourceAddressesMu.Unlock()

	delete(m.sourceAddresses, address)

	// If already running, update subscription
	if m.streamClient != nil {
		err := m.updateSubscription()
		if err != nil {
			m.logger.Errorf("Failed to update subscription: %v", err)
		}
	}
}

// GetSourceAddresses returns a copy of the current source addresses being monitored
func (m *TransferMonitor) GetSourceAddresses() []string {
	m.sourceAddressesMu.RLock()
	defer m.sourceAddressesMu.RUnlock()

	addresses := make([]string, 0, len(m.sourceAddresses))
	for addr := range m.sourceAddresses {
		addresses = append(addresses, addr)
	}

	return addresses
}

// AddCallback adds a callback function that will be called when a transfer is detected
func (m *TransferMonitor) AddCallback(callback TransferCallback) {
	m.callbacksMu.Lock()
	defer m.callbacksMu.Unlock()

	m.callbacks = append(m.callbacks, callback)
}

// Start starts the transfer monitor
func (m *TransferMonitor) Start() {

	// Create a stream client for transaction monitoring
	if err := m.geyserClient.AddStreamClient(m.ctx, "transfer_monitor", yellowstone_geyser_pb.CommitmentLevel_CONFIRMED); err != nil {
		logger.Fatalf("failed to create stream client: %v", err)
	}

	// get the stream client
	m.streamClient = m.geyserClient.GetStreamClient("transfer_monitor")
	if m.streamClient == nil {
		logger.Fatalf("failed to get stream client")
	}

	// Set up transaction subscription
	if err := m.updateSubscription(); err != nil {
		logger.Fatalf("failed to set up transaction subscription: %v", err)
	}

	// Start processing updates
	m.wg.Add(1)
	go m.processUpdates()

	// Start the Redis refresh job
	m.wg.Add(1)
	go m.startRedisRefreshJob()

	m.logger.Infof("Transfer monitor started")
	m.StartPingService(m.streamClient)

}

// updateSubscription updates the transaction subscription based on current source addresses
func (m *TransferMonitor) updateSubscription() error {
	m.sourceAddressesMu.RLock()
	defer m.sourceAddressesMu.RUnlock()

	// If no addresses to monitor, unsubscribe
	if len(m.sourceAddresses) == 0 {
		return m.streamClient.UnsubscribeTransaction("transfer_filter")
	}

	// Create transaction filter for the source addresses
	accounts := make([]string, 0, len(m.sourceAddresses))
	for addr := range m.sourceAddresses {
		accounts = append(accounts, addr)
	}
	isVote := false
	isFailed := false
	// Subscribe to transactions with these accounts
	txFilter := &yellowstone_geyser_pb.SubscribeRequestFilterTransactions{
		AccountInclude: accounts,
		Vote:           &isVote, // Exclude vote transactions
		Failed:         &isFailed,
	}

	return m.streamClient.SubscribeTransaction("transfer_filter", txFilter)
}

// Stop stops the transfer monitor
func (m *TransferMonitor) Stop() {
	m.runningMu.Lock()
	defer m.runningMu.Unlock()

	if !m.running {
		return
	}

	m.cancel()
	m.wg.Wait()

	if m.geyserClient != nil {
		m.geyserClient.Close()
	}

	// Close Redis client
	if m.redisClient != nil {
		if err := m.redisClient.Close(); err != nil {
			m.logger.Errorf("Error closing Redis client: %v", err)
		}
	}

	m.logger.Infof("Transfer monitor stopped")
}

// IsRunning returns whether the transfer monitor is running
func (m *TransferMonitor) IsRunning() bool {
	m.runningMu.Lock()
	defer m.runningMu.Unlock()

	return m.running
}

// processUpdates processes transaction updates from the stream
func (m *TransferMonitor) processUpdates() {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return

		case update := <-m.streamClient.Ch:
			if update == nil {
				continue
			}

			// Process transaction update
			if update.GetTransaction() != nil {
				m.processTransaction(update.GetTransaction())
			}

		case err := <-m.streamClient.ErrCh:
			if err != nil {
				m.logger.Errorf("Error from Yellowstone Geyser stream: %v", err)
			}
		}
	}
}

// processTransaction processes a transaction update and detects transfers
func (m *TransferMonitor) processTransaction(txUpdate *yellowstone_geyser_pb.SubscribeUpdateTransaction) {
	// Check if the transaction has signatures
	if txUpdate == nil || txUpdate.GetTransaction() == nil || txUpdate.GetTransaction().Transaction == nil ||
		len(txUpdate.GetTransaction().Transaction.Signatures) == 0 {
		m.logger.Warnf("Received transaction with no signatures, skipping")
		return
	}

	// Convert the transaction to a more usable format
	signature := base58.Encode(txUpdate.GetTransaction().Transaction.Signatures[0])

	logger.Debugf("Received transaction: %s", signature)

	// Process SOL transfers
	txResult := txUpdate.GetTransaction()

	m.processSOLTransfers(txResult, signature)

}

// processSOLTransfers processes SOL transfers in a transaction
func (m *TransferMonitor) processSOLTransfers(txResult *yellowstone_geyser_pb.SubscribeUpdateTransactionInfo, signature string) {
	if txResult.Meta == nil || txResult.Meta.PreBalances == nil || txResult.Meta.PostBalances == nil {
		return
	}

	// Get account keys from the transaction
	accountKeys := txResult.Transaction.Message.AccountKeys

	// Check for SOL transfers by comparing pre and post balances
	m.sourceAddressesMu.RLock()
	defer m.sourceAddressesMu.RUnlock()

	for i, preBalance := range txResult.Meta.PreBalances {
		if i >= len(accountKeys) || i >= len(txResult.Meta.PostBalances) {
			continue
		}

		account := base58.Encode(accountKeys[i])
		postBalance := txResult.Meta.PostBalances[i]

		// Check if this is a monitored source address and the balance decreased
		if m.sourceAddresses[account] && preBalance > postBalance {
			// Find the destination(s) where balance increased
			for j, postDstBalance := range txResult.Meta.PostBalances {
				if j >= len(accountKeys) || j >= len(txResult.Meta.PreBalances) {
					continue
				}

				dstAccount := base58.Encode(accountKeys[j])
				preDstBalance := txResult.Meta.PreBalances[j]

				// Skip if it's the same account or the balance didn't increase
				if dstAccount == account || postDstBalance <= preDstBalance {
					continue
				}
				// Skip if the destination is a monitored source address
				if m.sourceAddresses[dstAccount] {
					continue
				}

				// Key format: transfer_monitor:dest_map:{source_address}
				destMapKey := fmt.Sprintf("transfer_monitor:dest_map:%s", account)

				// Calculate the transfer amount for Redis recording

				// Current timestamp for tracking last transfer
				now := time.Now()
				timestamp := now.Unix()

				// Add destination to a hash in Redis with timestamp as value				// This allows for O(1) lookups of specific destinations
				err := m.redisClient.HSet(m.ctx, destMapKey, dstAccount, timestamp).Err()
				if err != nil {
					m.logger.Errorf("Failed to record destination account to Redis: %v", err)
				}

				// Set expiration on the hash if not already set (optional, based on retention needs)
				// This helps prevent unbounded growth - adjust TTL as needed
				m.redisClient.Expire(m.ctx, destMapKey, 2*24*time.Hour) // 2 days retention

				// Calculate the transfer amount
				transferAmount := postDstBalance - preDstBalance

				// Create and emit transfer event
				event := TransferEvent{
					Source:      account,
					Destination: dstAccount,
					Amount:      transferAmount,
					Signature:   signature,
					Timestamp:   time.Now(),
					TokenMint:   "", // Empty for SOL transfers
				}
				m.emitTransferEvent(event)
			}
		}
	}
}

// processSPLTokenTransfers processes SPL token transfers in a transaction
func (m *TransferMonitor) processSPLTokenTransfers(txResult *rpc.GetTransactionResult, signature string) {
	if txResult.Meta == nil || txResult.Meta.InnerInstructions == nil {
		return
	}

	// Get account keys from the transaction
	transaction, err := txResult.Transaction.GetTransaction()
	if err != nil {
		m.logger.Errorf("Failed to get transaction: %v", err)
		return
	}
	accountKeys := transaction.Message.AccountKeys

	// SPL token program ID
	tokenProgramID := "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"

	m.sourceAddressesMu.RLock()
	defer m.sourceAddressesMu.RUnlock()

	// Check all instructions and inner instructions for token transfers
	for _, ix := range transaction.Message.Instructions {
		if ix.ProgramIDIndex >= uint16(len(accountKeys)) {
			continue
		}

		programID := accountKeys[ix.ProgramIDIndex].String()

		// Check if this is the token program
		if programID == tokenProgramID {
			// Check if this is a transfer instruction (3 is the token transfer instruction)
			if len(ix.Data) >= 1 && ix.Data[0] == 3 {
				m.processTokenTransferInstruction(ix, accountKeys, signature)
			}
		}
	}

	// Check inner instructions
	for _, inner := range txResult.Meta.InnerInstructions {
		for _, ix := range inner.Instructions {
			if ix.ProgramIDIndex >= uint16(len(accountKeys)) {
				continue
			}

			programID := accountKeys[ix.ProgramIDIndex].String()

			// Check if this is the token program
			if programID == tokenProgramID {
				// Check if this is a transfer instruction (3 is the token transfer instruction)
				if len(ix.Data) >= 1 && ix.Data[0] == 3 {
					m.processTokenTransferInstruction(ix, accountKeys, signature)
				}
			}
		}
	}
}

// processTokenTransferInstruction processes a token transfer instruction
func (m *TransferMonitor) processTokenTransferInstruction(ix solana.CompiledInstruction, accountKeys solana.PublicKeySlice, signature string) {
	// Token transfer instruction has at least 3 accounts: source, destination, owner
	if len(ix.Accounts) < 3 {
		return
	}

	// Get the source, destination, and authority accounts
	sourceIndex := ix.Accounts[0]
	destIndex := ix.Accounts[1]
	ownerIndex := ix.Accounts[2]

	if sourceIndex >= uint16(len(accountKeys)) || destIndex >= uint16(len(accountKeys)) || ownerIndex >= uint16(len(accountKeys)) {
		return
	}

	destAccount := accountKeys[destIndex].String()
	ownerAccount := accountKeys[ownerIndex].String()

	// Check if the owner is a monitored source address
	if m.sourceAddresses[ownerAccount] {
		// Extract the amount from the instruction data
		// Token transfer data format: [3, amount (8 bytes, little endian)]
		if len(ix.Data) < 9 {
			return
		}

		// Parse the amount (8 bytes, little endian)
		amount := uint64(0)
		for i := 0; i < 8; i++ {
			amount |= uint64(ix.Data[i+1]) << (8 * i)
		}

		// For simplicity, we'll leave it empty for now
		tokenMint := ""

		// Create and emit transfer event
		event := TransferEvent{
			Source:      ownerAccount,
			Destination: destAccount,
			Amount:      amount,
			Signature:   signature,
			Timestamp:   time.Now(),
			TokenMint:   tokenMint,
		}

		m.emitTransferEvent(event)
	}
}

// emitTransferEvent sends the transfer event to all registered callbacks
func (m *TransferMonitor) emitTransferEvent(event TransferEvent) {
	m.logger.Debugf("Detected transfer: %s -> %s, amount: %d, signature: %s",
		event.Source, event.Destination, event.Amount, event.Signature)

	m.callbacksMu.RLock()
	defer m.callbacksMu.RUnlock()

	for _, callback := range m.callbacks {
		go callback(event)
	}
}

// startRedisRefreshJob starts a job that periodically refreshes destination addresses from Redis to memory
func (m *TransferMonitor) startRedisRefreshJob() {
	defer m.wg.Done()
	m.logger.Infof("Starting Redis refresh job with interval %v", m.refreshInterval)

	// Do an initial refresh
	m.refreshDestAddressesFromRedis()

	ticker := time.NewTicker(m.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			m.logger.Infof("Redis refresh job stopped")
			return
		case <-ticker.C:
			m.refreshDestAddressesFromRedis()
		}
	}
}

// refreshDestAddressesFromRedis refreshes the in-memory cache of destination addresses from Redis
func (m *TransferMonitor) refreshDestAddressesFromRedis() {
	m.sourceAddressesMu.RLock()
	sources := make([]string, 0, len(m.sourceAddresses))
	for source := range m.sourceAddresses {
		sources = append(sources, source)
	}
	m.sourceAddressesMu.RUnlock()

	// Create a map to deduplicate destination addresses
	destMap := make(map[string]struct{})

	// For each source address, get its destination addresses from Redis
	for _, source := range sources {
		destMapKey := fmt.Sprintf("transfer_monitor:dest_map:%s", source)

		// Get all destination addresses for this source using HKEYS
		destinations, err := m.redisClient.HKeys(m.ctx, destMapKey).Result()
		if err != nil {
			m.logger.Errorf("Failed to get destination addresses from Redis for source %s: %v", source, err)
			continue
		}

		// Add destinations to the map for deduplication
		for _, dest := range destinations {
			destMap[dest] = struct{}{}
		}

		m.logger.Debugf("Retrieved %d destination addresses for source %s", len(destinations), source)
	}

	// Convert map keys to slice
	allDestinations := make([]string, 0, len(destMap))
	for dest := range destMap {
		allDestinations = append(allDestinations, dest)
	}

	// Update the in-memory cache
	m.destAddressesMu.Lock()
	m.destAddresses = allDestinations
	m.destAddressesMu.Unlock()

	m.logger.Infof("Refreshed %d unique destination addresses from Redis", len(allDestinations))
}

// GetDestinationAddresses returns all destination addresses
func (m *TransferMonitor) GetDestinationAddresses() []string {
	m.destAddressesMu.RLock()
	defer m.destAddressesMu.RUnlock()

	// Return a copy of the slice to prevent external modification
	result := make([]string, len(m.destAddresses))
	copy(result, m.destAddresses)
	return result
}

// GetDestinationAddressesCount returns the number of destination addresses
func (m *TransferMonitor) GetDestinationAddressesCount() int {
	m.destAddressesMu.RLock()
	defer m.destAddressesMu.RUnlock()

	return len(m.destAddresses)
}

// ContainsDestinationAddress checks if a specific address is in the destination addresses
func (m *TransferMonitor) ContainsDestinationAddress(address string) bool {
	// First check in-memory cache for quick response
	m.destAddressesMu.RLock()
	inMemoryResult := slices.Contains(m.destAddresses, address)
	m.destAddressesMu.RUnlock()

	if inMemoryResult {
		return true
	}

	// If not found in memory, check directly in Redis for the specific address
	// This provides a more efficient lookup for specific addresses
	m.sourceAddressesMu.RLock()
	sources := make([]string, 0, len(m.sourceAddresses))
	for source := range m.sourceAddresses {
		sources = append(sources, source)
	}
	m.sourceAddressesMu.RUnlock()

	// Check each source's hash for this specific destination
	for _, source := range sources {
		destMapKey := fmt.Sprintf("transfer_monitor:dest_map:%s", source)
		exists, err := m.redisClient.HExists(m.ctx, destMapKey, address).Result()
		if err != nil {
			m.logger.Errorf("Error checking destination in Redis: %v", err)
			continue
		}
		if exists {
			return true
		}
	}

	return false
}

// startPingService 启动一个定时ping服务，保持与服务器的连接活跃
func (m *TransferMonitor) StartPingService(streamClient *goyser.StreamClient) {
	go func() {
		ticker := time.NewTicker(time.Second * 15)
		defer ticker.Stop()
		for range ticker.C {
			lastPingTime := time.Now()
			err := streamClient.SendCustomRequest(&yellowstone_geyser_pb.SubscribeRequest{
				Ping: &yellowstone_geyser_pb.SubscribeRequestPing{},
			})
			if err != nil {
				logger.Errorf("发送 ping 失败: %v", err)
				return
			}

			logger.Infof("发送 ping - 时间:%v", lastPingTime.Format("15:04:05.000"))
		}
	}()
}
