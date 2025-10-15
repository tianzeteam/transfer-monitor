package utils

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LogConfig 表示日志配置
type LogConfig struct {
	Level  string `yaml:"level"`  // 日志级别: debug, info, warn, error, fatal
	Format string `yaml:"format"` // 日志格式: text, json
	File   string `yaml:"file"`   // 日志文件路径，为空则输出到控制台
}

// Config 表示整个配置文件结构
type Config struct {
	GRPCUrl               string    `yaml:"grpc_url"`
	TransferMonitorEnable bool      `yaml:"transfer_monitor_enable"`
	TransferSourceAddress []string  `yaml:"transfer_source_address"`
	Logging               LogConfig `yaml:"logging"` // 日志配置
}

// LoadConfig 从文件加载配置
func LoadConfig(filename string) (*Config, error) {
	// 读取文件
	buf, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	config := &Config{}
	err = yaml.Unmarshal(buf, config)
	if err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return config, nil
}
