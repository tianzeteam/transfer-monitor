package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"transfer_monitor/utils"

	"transfer_monitor/logger"
	"transfer_monitor/monitor"

	goyser "transfer_monitor/yellowstone_geyser"

	_ "github.com/joho/godotenv/autoload"
)

func getRootDir() string {
	// 使用os.Executable()获取当前运行的可执行文件路径
	execPath, err := os.Executable()
	if err != nil {
		// 如果获取失败，回退到使用runtime.Caller
		_, b, _, _ := runtime.Caller(0)
		return filepath.Dir(b)
	}
	// 返回可执行文件所在目录
	return filepath.Dir(execPath)
}
func main() {
	// 获取配置文件的绝对路径
	rootDir := getRootDir()
	configPath := filepath.Join(rootDir, "config", "config.yaml")
	logger.Infof("加载配置文件: %s", configPath)

	// 加载配置
	config, err := utils.LoadConfig(configPath)
	if err != nil {
		logger.Errorf("加载配置失败: %v", err)
		os.Exit(1)
	}

	//-----------------------------------------------------------
	// get the geyser rpc address
	grpcUrl := config.GRPCUrl
	// create geyser client
	client, err := goyser.New(context.Background(), grpcUrl, nil)
	if err != nil {
		logger.Fatalf("创建geyser客户端失败: %v", err)
	}

	//启动transfer monitor
	// Create a new transfer monitor
	transferMonitor, errTM := monitor.NewTransferMonitor(config, client)
	if errTM != nil {
		logger.Fatalf("Failed to create transfer monitor: %v\n", errTM)
		return
	}

	// Add  source addresses to monitor
	transferSourecAddress := config.TransferSourceAddress
	for _, sourceAddrss := range transferSourecAddress {
		transferMonitor.AddSourceAddress(sourceAddrss) //

	}

	// Add a callback function to handle transfer events
	transferMonitor.AddCallback(func(event monitor.TransferEvent) {
		logger.Infof("Transfer detected:\n")
		logger.Infof("  Source: %s\n", event.Source)
		logger.Infof("  Destination: %s\n", event.Destination)
		logger.Infof("  Amount: %d\n", event.Amount)
		logger.Infof("  Signature: %s\n", event.Signature)
		logger.Infof("  Timestamp: %s\n", event.Timestamp)
		if event.TokenMint != "" {
			logger.Infof("  Token Mint: %s\n", event.TokenMint)
		} else {
			logger.Infof("  Token: SOL\n")
		}
		logger.Infof("\n")
	})

	transferMonitor.Start()

	// 添加阻塞机制，确保主线程不会退出
	logger.Infof("程序已启动，按Ctrl+C退出")

	// 创建一个通道来捕获信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 阻塞直到收到信号
	sig := <-sigCh
	logger.Infof("收到信号 %v，程序即将退出", sig)

}
