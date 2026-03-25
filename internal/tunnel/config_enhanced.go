// Package tunnel 提供内网穿透服务 - 配置增强
package tunnel

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"
)

// EnhancedConfig 增强配置
type EnhancedConfig struct {
	// 基础配置
	*Config

	// 带宽配置
	Bandwidth BandwidthConfig `json:"bandwidth"`

	// 质量监控配置
	Quality QualityMonitorConfig `json:"quality"`

	// 连接优化配置
	Optimization OptimizationConfig `json:"optimization"`

	// 安全配置
	Security SecurityConfig `json:"security"`

	// 日志配置
	Logging LoggingConfig `json:"logging"`

	// 高级配置
	Advanced AdvancedConfig `json:"advanced"`
}

// OptimizationConfig 连接优化配置
type OptimizationConfig struct {
	// 自动模式切换
	AutoModeSwitch bool `json:"auto_mode_switch"`
	// P2P 优先
	P2PPriority bool `json:"p2p_priority"`
	// 模式切换阈值
	SwitchThreshold int `json:"switch_threshold"`
	// 连接重试次数
	ConnectRetries int `json:"connect_retries"`
	// 重试间隔
	RetryInterval time.Duration `json:"retry_interval"`
	// 保活间隔
	KeepaliveInterval time.Duration `json:"keepalive_interval"`
	// 连接超时
	ConnectTimeout time.Duration `json:"connect_timeout"`
	// 空闲超时
	IdleTimeout time.Duration `json:"idle_timeout"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	// 启用加密
	EnableEncryption bool `json:"enable_encryption"`
	// 加密算法 (aes-256-gcm, chacha20-poly1305)
	CipherAlgorithm string `json:"cipher_algorithm"`
	// 密钥交换算法
	KeyExchange string `json:"key_exchange"`
	// 认证超时
	AuthTimeout time.Duration `json:"auth_timeout"`
	// 允许的 IP 白名单
	AllowedIPs []string `json:"allowed_ips"`
	// 禁止的 IP 黑名单
	BlockedIPs []string `json:"blocked_ips"`
	// 最大认证失败次数
	MaxAuthFailures int `json:"max_auth_failures"`
	// 认证失败锁定时间
	AuthLockDuration time.Duration `json:"auth_lock_duration"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	// 日志级别 (debug, info, warn, error)
	Level string `json:"level"`
	// 日志格式 (json, text)
	Format string `json:"format"`
	// 日志文件路径
	OutputPath string `json:"output_path"`
	// 最大日志文件大小 (MB)
	MaxSize int `json:"max_size"`
	// 最大日志文件数量
	MaxBackups int `json:"max_backups"`
	// 日志保留天数
	MaxAge int `json:"max_age"`
	// 是否压缩旧日志
	Compress bool `json:"compress"`
}

// AdvancedConfig 高级配置
type AdvancedConfig struct {
	// MTU 大小
	MTU int `json:"mtu"`
	// 发送缓冲区大小
	SendBuffer int `json:"send_buffer"`
	// 接收缓冲区大小
	RecvBuffer int `json:"recv_buffer"`
	// 并发连接数
	MaxConnections int `json:"max_connections"`
	// 连接队列大小
	ConnectionQueue int `json:"connection_queue"`
	// 多路复用
	EnableMultiplexing bool `json:"enable_multiplexing"`
	// 多路复用流数量
	MuxStreams int `json:"mux_streams"`
	// 启用压缩
	EnableCompression bool `json:"enable_compression"`
	// 压缩级别 (1-9)
	CompressionLevel int `json:"compression_level"`
	// 启用 TCP Fast Open
	EnableTCPFastOpen bool `json:"enable_tcp_fast_open"`
}

// DefaultEnhancedConfig 默认增强配置
func DefaultEnhancedConfig() *EnhancedConfig {
	return &EnhancedConfig{
		Config: &Config{
			Mode:         ModeAuto,
			HeartbeatInt: 30,
			ReconnectInt: 5,
			MaxReconnect: 10,
			Timeout:      30,
			STUNServers: []string{
				"stun:stun.l.google.com:19302",
				"stun:stun1.l.google.com:19302",
			},
		},
		Bandwidth: BandwidthConfig{
			StatsInterval:    time.Second,
			BucketMultiplier: 2,
		},
		Quality: QualityMonitorConfig{
			ProbeInterval: 5 * time.Second,
			ProbeTimeout:  2 * time.Second,
			ProbeCount:    5,
			HistorySize:   10,
			Thresholds: QualityThresholds{
				ExcellentLatency:    50,
				GoodLatency:         100,
				FairLatency:         200,
				ExcellentPacketLoss: 0.1,
				GoodPacketLoss:      1.0,
				FairPacketLoss:      5.0,
			},
		},
		Optimization: OptimizationConfig{
			AutoModeSwitch:    true,
			P2PPriority:       true,
			SwitchThreshold:   50,
			ConnectRetries:    3,
			RetryInterval:     time.Second,
			KeepaliveInterval: 25 * time.Second,
			ConnectTimeout:    10 * time.Second,
			IdleTimeout:       5 * time.Minute,
		},
		Security: SecurityConfig{
			EnableEncryption: true,
			CipherAlgorithm:  "chacha20-poly1305",
			KeyExchange:      "x25519",
			AuthTimeout:      10 * time.Second,
			MaxAuthFailures:  5,
			AuthLockDuration: 5 * time.Minute,
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			MaxSize:    100,
			MaxBackups: 5,
			MaxAge:     30,
			Compress:   true,
		},
		Advanced: AdvancedConfig{
			MTU:                1400,
			SendBuffer:         1024 * 1024,
			RecvBuffer:         1024 * 1024,
			MaxConnections:     100,
			ConnectionQueue:    128,
			EnableMultiplexing: true,
			MuxStreams:         16,
			EnableCompression:  false,
			CompressionLevel:   6,
			EnableTCPFastOpen:  true,
		},
	}
}

// LoadEnhancedConfig 从文件加载增强配置
func LoadEnhancedConfig(path string) (*EnhancedConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := DefaultEnhancedConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}

	if err := ValidateEnhancedConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

// SaveEnhancedConfig 保存增强配置到文件
func SaveEnhancedConfig(path string, config *EnhancedConfig) error {
	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// ValidateEnhancedConfig 验证增强配置
func ValidateEnhancedConfig(config *EnhancedConfig) error {
	if config.Config == nil {
		config.Config = &Config{}
	}

	// 验证基础配置
	if config.ServerAddr == "" {
		return errors.New("server address is required")
	}

	if config.ServerPort <= 0 || config.ServerPort > 65535 {
		return errors.New("invalid server port")
	}

	// 验证带宽配置
	if config.Bandwidth.UploadLimit < 0 {
		return errors.New("upload limit cannot be negative")
	}
	if config.Bandwidth.DownloadLimit < 0 {
		return errors.New("download limit cannot be negative")
	}

	// 验证优化配置
	if config.Optimization.SwitchThreshold < 0 || config.Optimization.SwitchThreshold > 100 {
		return errors.New("switch threshold must be between 0 and 100")
	}
	if config.Optimization.ConnectRetries < 0 {
		return errors.New("connect retries cannot be negative")
	}

	// 验证安全配置
	validCiphers := map[string]bool{
		"aes-256-gcm":       true,
		"chacha20-poly1305": true,
	}
	if !validCiphers[config.Security.CipherAlgorithm] {
		return errors.New("invalid cipher algorithm")
	}

	// 验证高级配置
	if config.Advanced.MTU < 576 || config.Advanced.MTU > 9000 {
		return errors.New("MTU must be between 576 and 9000")
	}
	if config.Advanced.MaxConnections < 1 {
		return errors.New("max connections must be at least 1")
	}

	return nil
}

// ConfigProfile 配置模板.
type ConfigProfile string

const (
	// ProfileBalanced 平衡模式.
	ProfileBalanced ConfigProfile = "balanced"
	// ProfilePerformance 性能优先.
	ProfilePerformance ConfigProfile = "performance"
	// ProfileReliable 稳定性优先.
	ProfileReliable ConfigProfile = "reliable"
	// ProfileLowLatency 低延迟模式.
	ProfileLowLatency ConfigProfile = "low_latency"
	// ProfileSecure 安全优先.
	ProfileSecure ConfigProfile = "secure"
)

// ApplyProfile 应用配置模板
func (c *EnhancedConfig) ApplyProfile(profile ConfigProfile) {
	switch profile {
	case ProfileBalanced:
		// 默认配置已经平衡
	case ProfilePerformance:
		c.Optimization.P2PPriority = true
		c.Advanced.EnableMultiplexing = true
		c.Advanced.MuxStreams = 32
		c.Bandwidth.UploadLimit = 0 // 不限速
		c.Bandwidth.DownloadLimit = 0
	case ProfileReliable:
		c.Optimization.AutoModeSwitch = true
		c.Optimization.ConnectRetries = 5
		c.Quality.ProbeInterval = 3 * time.Second
		c.Quality.Thresholds.ExcellentLatency = 100
	case ProfileLowLatency:
		c.Optimization.P2PPriority = true
		c.Quality.Thresholds.ExcellentLatency = 30
		c.Quality.Thresholds.GoodLatency = 50
		c.Optimization.KeepaliveInterval = 10 * time.Second
	case ProfileSecure:
		c.Security.EnableEncryption = true
		c.Security.CipherAlgorithm = "aes-256-gcm"
		c.Security.MaxAuthFailures = 3
		c.Security.AuthLockDuration = 15 * time.Minute
		c.Logging.Level = "warn"
	}
}

// ConfigDiff 配置差异
type ConfigDiff struct {
	Field    string      `json:"field"`
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
}

// CompareConfigs 比较两个配置
func CompareConfigs(old, new *EnhancedConfig) []ConfigDiff {
	var diffs []ConfigDiff

	// 比较基础配置
	if old.ServerAddr != new.ServerAddr {
		diffs = append(diffs, ConfigDiff{
			Field:    "server_addr",
			OldValue: old.ServerAddr,
			NewValue: new.ServerAddr,
		})
	}

	if old.ServerPort != new.ServerPort {
		diffs = append(diffs, ConfigDiff{
			Field:    "server_port",
			OldValue: old.ServerPort,
			NewValue: new.ServerPort,
		})
	}

	if old.Mode != new.Mode {
		diffs = append(diffs, ConfigDiff{
			Field:    "mode",
			OldValue: old.Mode,
			NewValue: new.Mode,
		})
	}

	// 比较带宽配置
	if old.Bandwidth.UploadLimit != new.Bandwidth.UploadLimit {
		diffs = append(diffs, ConfigDiff{
			Field:    "upload_limit",
			OldValue: old.Bandwidth.UploadLimit,
			NewValue: new.Bandwidth.UploadLimit,
		})
	}

	if old.Bandwidth.DownloadLimit != new.Bandwidth.DownloadLimit {
		diffs = append(diffs, ConfigDiff{
			Field:    "download_limit",
			OldValue: old.Bandwidth.DownloadLimit,
			NewValue: new.Bandwidth.DownloadLimit,
		})
	}

	return diffs
}

// ConfigValidator 配置验证器接口
type ConfigValidator interface {
	Validate(config *EnhancedConfig) error
}

// NetworkValidator 网络配置验证器.
type NetworkValidator struct{}

// Validate 验证配置.
func (v *NetworkValidator) Validate(config *EnhancedConfig) error {
	// 验证 STUN 服务器地址
	for _, server := range config.STUNServers {
		// 支持 stun: 前缀
		addr := server
		if len(addr) > 5 && addr[:5] == "stun:" {
			addr = addr[5:]
		}
		if _, err := net.ResolveUDPAddr("udp", addr); err != nil {
			return errors.New("invalid STUN server: " + server)
		}
	}

	// 验证 TURN 服务器地址
	for _, server := range config.TURNServers {
		if server == "" {
			continue
		}
		// 支持 turn: 前缀
		addr := server
		if len(addr) > 5 && addr[:5] == "turn:" {
			addr = addr[5:]
		}
		if _, err := net.ResolveUDPAddr("udp", addr); err != nil {
			return errors.New("invalid TURN server: " + server)
		}
	}

	return nil
}

// ConfigManager 配置管理器
type ConfigManager struct {
	configPath string
	config     *EnhancedConfig
	validators []ConfigValidator
}

// NewConfigManager 创建配置管理器
func NewConfigManager(configPath string) *ConfigManager {
	return &ConfigManager{
		configPath: configPath,
		config:     DefaultEnhancedConfig(),
		validators: []ConfigValidator{
			&NetworkValidator{},
		},
	}
}

// Load 加载配置
func (m *ConfigManager) Load() error {
	config, err := LoadEnhancedConfig(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 配置文件不存在，使用默认配置
			return nil
		}
		return err
	}

	m.config = config
	return nil
}

// Save 保存配置
func (m *ConfigManager) Save() error {
	return SaveEnhancedConfig(m.configPath, m.config)
}

// Get 获取当前配置
func (m *ConfigManager) Get() *EnhancedConfig {
	return m.config
}

// Set 更新配置
func (m *ConfigManager) Set(config *EnhancedConfig) error {
	// 验证配置
	for _, validator := range m.validators {
		if err := validator.Validate(config); err != nil {
			return err
		}
	}

	m.config = config
	return nil
}

// Reload 重新加载配置
func (m *ConfigManager) Reload() error {
	return m.Load()
}

// WatchConfig 监听配置文件变化（简化实现）
func (m *ConfigManager) WatchConfig(callback func(*EnhancedConfig)) {
	// 实际实现应使用 fsnotify 或类似库
	// 这里只提供接口框架
}
