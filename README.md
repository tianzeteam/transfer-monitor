# Transfer Monitor

一个高性能的 Solana 区块链转账监控系统，使用场景很多,例如智能跟单交易和资金流向分析等。基于 Yellowstone gRPC 实时监控指定地址的 SOL 和 SPL Token 转账活动，为SOL跟单策略提供精准的数据支持。

> 💡 **相关产品**: 如需完整的SOL跟单交易解决方案，请访问 [BeyondJeet 官网](https://beyondpump.app/) 了解更多专业跟单工具。

## 🚀 项目概述

Transfer Monitor 是一个专为 Solana 生态系统设计的实时转账监控工具，为跟单交易和智能投资策略提供核心数据支持，能够：

- **实时监控**：通过 Yellowstone gRPC 流式接口实时监控区块链交易，捕获跟单机会
- **多地址支持**：同时监控多个源地址的转账活动，追踪大户资金流向
- **智能过滤**：自动识别和过滤 SOL 和 SPL Token 转账，精准定位交易信号
- **数据持久化**：使用 Redis 存储目标地址映射，支持跟单历史数据查询和分析
- **高性能**：基于 Go 语言开发，支持高并发处理，满足高频跟单需求
- **灵活配置**：YAML 配置文件，支持环境变量覆盖，适配不同跟单策略

## 📋 功能特性

### 核心功能

- ✅ **SOL 转账监控**：监控原生 SOL 代币的转账活动，识别大户资金动向
- ✅ **SPL Token 监控**：支持所有 SPL Token 的转账监控，捕获代币跟单信号
- ✅ **实时事件回调**：可自定义转账事件处理逻辑，实现智能跟单策略
- ✅ **Redis 集成**：自动存储和管理目标地址数据，构建跟单地址白名单
- ✅ **连接保活**：自动 ping 机制保持 gRPC 连接稳定，确保跟单数据不丢失

### 高级特性

- 🔄 **自动重连**：网络断开时自动重新连接，保障跟单服务连续性
- 📊 **内存缓存**：Redis 数据的内存缓存提升查询性能，加速跟单决策
- 🛡️ **并发安全**：全面的并发控制和线程安全，支持多策略并行跟单
- 📝 **详细日志**：结构化日志记录，支持多种输出格式，便于跟单策略分析
- ⚡ **高效查询**：O(1) 复杂度的地址查询算法，毫秒级跟单响应

## 🏗️ 系统架构

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Yellowstone   │────│ Transfer Monitor │────│     Redis       │
│   gRPC Stream   │    │   (跟单引擎)      │    │  (跟单地址池)    │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                               │
                               ▼
                       ┌──────────────────┐
                       │   Event Callback │
                       │  (跟单策略执行)   │
                       └──────────────────┘
```

## 📦 安装部署

### 环境要求

- **Go**: 1.24 或更高版本
- **Redis**: 6.0 或更高版本
- **网络**: 能够访问 Solana Yellowstone gRPC 节点

### 快速安装

1. **克隆项目**

```bash
git clone <repository-url>
cd transfer-monitor
```

2. **安装依赖**

```bash
go mod download
```

3. **配置 Redis**

```bash
# 启动 Redis 服务
redis-server

# 或使用 Docker
docker run -d --name redis -p 6379:6379 redis:latest
```

4. **配置环境变量**

```bash
# 设置 Redis 地址（可选，默认 localhost:6379）
export REDIS_ADDR=localhost:6379
```

5. **编译运行**

```bash
# 编译
go build -o transfer-monitor

# 运行
./transfer-monitor
```

## ⚙️ 配置说明

### 配置文件 (config/config.yaml)

```yaml
# Yellowstone gRPC 节点地址
grpc_url: "https://solana-yellowstone-grpc.publicnode.com:443"

# 转账监控配置 (跟单系统核心配置)
transfer_monitor_enable: true

# 监控的源地址列表 (跟单目标地址)
transfer_source_address: 
  - "5tzFkiKscXHK5ZXCGbXZxdw7gTjjD1mBwuoFbhUvuAi9"
  - "ASTyfSima4LLAdDgoFGkgqoKowG1LZFDr9fAQrg7iaJZ"

# 日志配置
logging:
  level: "info"        # 日志级别: debug, info, warn, error, fatal
  format: "text"       # 日志格式: text, json
  file: ""            # 日志文件路径，为空则输出到控制台
  isTerminal: true
```

### 环境变量

| 变量名         | 说明             | 默认值             |
| -------------- | ---------------- | ------------------ |
| `REDIS_ADDR` | Redis 服务器地址 | `localhost:6379` |

## 🔧 使用方法

### 基础使用

```go
package main

import (
    "context"
    "transfer_monitor/monitor"
    "transfer_monitor/utils"
    goyser "transfer_monitor/yellowstone_geyser"
)

func main() {
    // 加载配置
    config, err := utils.LoadConfig("config/config.yaml")
    if err != nil {
        panic(err)
    }

    // 创建 gRPC 客户端
    client, err := goyser.New(context.Background(), config.GRPCUrl, nil)
    if err != nil {
        panic(err)
    }

    // 创建转账监控器
    transferMonitor, err := monitor.NewTransferMonitor(config, client)
    if err != nil {
        panic(err)
    }

    // 添加跟单目标地址
    transferMonitor.AddSourceAddress("your-target-wallet-address-here")

    // 添加跟单事件回调处理
    transferMonitor.AddCallback(func(event monitor.TransferEvent) {
        fmt.Printf("跟单信号检测到:\n")
        fmt.Printf("  跟单源地址: %s\n", event.Source)
        fmt.Printf("  资金流向地址: %s\n", event.Destination)
        fmt.Printf("  转账金额: %d\n", event.Amount)
        fmt.Printf("  交易签名: %s\n", event.Signature)
        if event.TokenMint != "" {
            fmt.Printf("  跟单代币: %s\n", event.TokenMint)
        } else {
            fmt.Printf("  跟单代币: SOL\n")
        }
        // 在此处添加跟单策略逻辑
    })

    // 启动监控
    transferMonitor.Start()

    // 保持程序运行
    select {}
}
```

### 高级功能

#### 1. 动态管理跟单目标地址

```go
// 添加新的跟单目标地址
err := transferMonitor.AddSourceAddress("new-target-address")

// 移除跟单目标地址
transferMonitor.RemoveSourceAddress("old-target-address")

// 获取当前跟单监控的地址列表
addresses := transferMonitor.GetSourceAddresses()
```

#### 2. 查询跟单资金流向地址

```go
// 检查地址是否在跟单资金流向池中
isDestination := transferMonitor.ContainsDestinationAddress("address-to-check")

// 获取所有跟单资金流向地址
allDestinations := transferMonitor.GetDestinationAddresses()

// 获取跟单目标地址数量
count := transferMonitor.GetDestinationAddressesCount()
```

#### 3. 多策略跟单处理

```go
// 添加多个跟单策略回调函数
transferMonitor.AddCallback(logCopyTradeSignal)     // 记录跟单信号
transferMonitor.AddCallback(executeCopyTrade)       // 执行跟单交易
transferMonitor.AddCallback(sendCopyTradeAlert)     // 发送跟单提醒
```

## 📊 数据结构

### TransferEvent

```go
type TransferEvent struct {
    Source      string    // 源地址
    Destination string    // 目标地址
    Amount      uint64    // 转账金额（lamports 或 token 最小单位）
    Signature   string    // 交易签名
    Timestamp   time.Time // 时间戳
    TokenMint   string    // 代币 Mint 地址（SOL 转账时为空）
}
```

## 🗄️ Redis 数据结构

系统使用以下 Redis 数据结构存储目标地址映射：

```
Key: transfer_monitor:dest_map:{copy_trade_source_address}
Type: Hash
Fields: {copy_trade_destination_address} -> {timestamp}
TTL: 2 days
```

跟单地址映射示例：

```
transfer_monitor:dest_map:5tzFkiKscXHK5ZXCGbXZxdw7gTjjD1mBwuoFbhUvuAi9
├── 9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM -> 1640995200  # 跟单目标地址1
├── 2wmVCSfPxGPjrnMMn7rchp4uaeoTqN39mXFC2zhPdri9 -> 1640995300  # 跟单目标地址2
└── ...                                                    # 更多跟单地址
```

## 🔍 监控和日志

### 日志级别

- **DEBUG**: 详细的调试信息，包括每个交易的处理过程
- **INFO**: 一般信息，包括启动、连接状态、转账事件
- **WARN**: 警告信息，如连接问题、数据异常
- **ERROR**: 错误信息，如配置错误、网络错误
- **FATAL**: 致命错误，程序将退出

### 关键日志示例

```
2024-01-15 10:30:15.123 INFO  跟单监控系统已启动，按Ctrl+C退出
2024-01-15 10:30:15.456 INFO  监控跟单目标地址：5tzFkiKscXHK5ZXCGbXZxdw7gTjjD1mBwuoFbhUvuAi9
2024-01-15 10:30:16.789 INFO  跟单监控引擎启动完成
2024-01-15 10:30:17.012 INFO  发送 ping 保持跟单连接 - 时间:10:30:17.012
2024-01-15 10:30:25.345 INFO  跟单信号检测到:
2024-01-15 10:30:25.345 INFO    跟单源: 5tzFkiKscXHK5ZXCGbXZxdw7gTjjD1mBwuoFbhUvuAi9
2024-01-15 10:30:25.345 INFO    资金流向: 9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM
2024-01-15 10:30:25.345 INFO    跟单金额: 1000000000
2024-01-15 10:30:25.345 INFO    跟单代币: SOL
```

## 🚀 性能优化

### 跟单系统性能特点

- **低延迟**: 基于 gRPC 流式接口，毫秒级跟单信号响应
- **高吞吐**: 支持每秒处理数千笔跟单交易信号
- **内存效率**: 智能缓存机制，减少 Redis 查询，提升跟单决策速度
- **网络优化**: 自动重连和保活机制，确保跟单服务高可用

### 优化建议

1. **Redis 优化**

   ```bash
   # 调整 Redis 配置
   maxmemory-policy allkeys-lru
   save 900 1
   ```
2. **系统资源**

   ```bash
   # 推荐系统配置
   CPU: 2+ 核心
   内存: 4GB+
   网络: 稳定的互联网连接
   ```
3. **跟单目标地址数量**

   - 建议同时跟单监控的地址数量 < 100
   - 大量跟单目标可考虑分布式部署

## 🛠️ 开发指南

### 项目结构

```
transfer-monitor/
├── main.go                 # 程序入口
├── config/
│   └── config.yaml        # 配置文件
├── monitor/
│   └── transfer_monitor.go # 核心监控逻辑
├── logger/
│   └── logger.go          # 日志系统
├── utils/
│   └── config.go          # 配置管理
├── grpc/
│   └── grpc.go           # gRPC 连接管理
├── yellowstone_geyser/    # Yellowstone gRPC 客户端
└── vendor/               # 依赖包
```

### 扩展开发

#### 添加自定义跟单策略处理器

```go
type CopyTradeHandler struct {
    // 跟单策略配置字段
    CopyRatio    float64  // 跟单比例
    MaxAmount    uint64   // 最大跟单金额
    TokenFilter  []string // 跟单代币白名单
}

func (h *CopyTradeHandler) HandleCopyTradeSignal(event monitor.TransferEvent) {
    // 跟单策略处理逻辑
    // 例如：计算跟单金额、执行跟单交易、风险控制等
    if h.shouldCopyTrade(event) {
        h.executeCopyTrade(event)
    }
}

// 注册跟单策略处理器
transferMonitor.AddCallback(copyTradeHandler.HandleCopyTradeSignal)
```

#### 添加新的跟单事件类型

```go
// 扩展跟单事件结构
type CopyTradeEvent struct {
    monitor.TransferEvent
    CopyTradeRatio   float64                // 跟单比例
    TargetTokens     []string               // 目标代币列表
    RiskLevel        string                 // 风险等级
    StrategyMetadata map[string]interface{} // 策略元数据
}
```

## 🔧 故障排除

### 常见问题

1. **连接失败**

   ```
   错误: 创建geyser客户端失败
   解决: 检查网络连接和 gRPC URL 配置
   ```
2. **Redis 连接失败**

   ```
   错误: Failed to record destination account to Redis
   解决: 确认 Redis 服务运行正常，检查 REDIS_ADDR 环境变量
   ```
3. **内存使用过高**

   ```
   原因: 跟单目标地址过多或跟单地址缓存过大
   解决: 减少跟单监控地址数量，调整 Redis TTL 设置
   ```

### 调试模式

启用详细日志进行问题诊断：

```yaml
logging:
  level: "debug"
  format: "json"
```

## 📈 监控指标

跟单系统提供以下关键指标用于监控：

- **连接状态**: gRPC 连接健康状态，确保跟单信号不丢失
- **处理延迟**: 跟单信号处理平均延迟，影响跟单执行时机
- **内存使用**: 跟单地址缓存大小和内存占用
- **Redis 性能**: 跟单数据查询响应时间和错误率

## 🔒 安全考虑

- **网络安全**: 使用 TLS 加密的 gRPC 连接，保护跟单数据传输
- **访问控制**: Redis 访问控制和密码保护，确保跟单地址数据安全
- **数据隐私**: 跟单目标地址信息的安全存储和访问控制
- **资源限制**: 防止内存泄漏和资源耗尽，确保跟单服务稳定性

## 📝 更新日志

### v2.0.1 (2024-08-06)

- 优化跟单系统 Redis 连接管理
- 改进跟单信号错误处理机制
- 跟单性能优化和内存使用改进

### v1.0.1 (2024-08-01)

- 跟单监控系统初始版本发布
- 基础跟单转账监控功能
- Redis 集成和跟单事件回调系统

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！

1. Fork 项目
2. 创建功能分支
3. 提交更改
4. 推送到分支
5. 创建 Pull Request

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 📞 技术支持

如有问题或需要技术支持，请：

1. 查看本文档的故障排除部分
2. 提交 GitHub Issue
3. 联系开发团队
4. 访问 [BeyondJeet 官网](https://beyondpump.app/) 获取专业技术支持

## 🔗 相关链接

- **BeyondJeet 官网**: [https://beyondpump.app/](https://beyondpump.app/) - 专业的 Solana 跟单交易平台
- **产品文档**: 完整的跟单策略和风险管理指南
- **社区支持**: 专业的跟单交易社区和技术交流

---

**注意**: 本跟单监控系统用于监控公开的区块链数据，仅供学习和研究使用。进行跟单交易时请注意风险控制，确保遵守相关法律法规和隐私政策。跟单有风险，投资需谨慎。
