// Package smartdedup 提供内容感知的智能文件去重功能
package smartdedup

import (
	"encoding/json"
	"os"
	"time"
)

// Config 去重配置.
type Config struct {
	// 基本配置
	Enabled  bool           `json:"enabled"`
	Backend  StorageBackend `json:"backend"`  // 存储后端
	Mode     DedupMode      `json:"mode"`     // 去重模式
	Action   DedupAction    `json:"action"`   // 去重动作

	// 扫描配置
	ScanPaths       []string `json:"scanPaths"`       // 扫描路径
	ExcludePaths    []string `json:"excludePaths"`    // 排除路径
	ExcludePatterns []string `json:"excludePatterns"` // 排除模式（glob）
	MinFileSize     int64    `json:"minFileSize"`     // 最小文件大小（字节）
	MaxFileSize     int64    `json:"maxFileSize"`     // 最大文件大小（字节）

	// 定时扫描配置
	ScheduleCron    string `json:"scheduleCron"`    // Cron 表达式
	ScheduleEnabled bool   `json:"scheduleEnabled"` // 启用定时扫描

	// 实时去重配置
	RealtimeEnabled  bool          `json:"realtimeEnabled"`  // 启用实时去重
	DebounceDuration time.Duration `json:"debounceDuration"` // 防抖时长

	// 性能配置
	MaxWorkers  int  `json:"maxWorkers"`  // 最大并行数
	MaxMemoryMB int  `json:"maxMemoryMB"` // 最大内存使用（MB）
	HashCache   bool `json:"hashCache"`   // 启用哈希缓存

	// 安全配置
	DryRun       bool `json:"dryRun"`       // 试运行模式
	VerifyAfter  bool `json:"verifyAfter"`  // 去重后验证
	MaxRefPerFile int `json:"maxRefPerFile"` // 每个文件最大引用数（0=无限制）
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		Enabled:          true,
		Backend:          BackendAuto,
		Mode:             ModeHybrid,
		Action:           ActionReflink,
		MinFileSize:      4096,        // 4KB
		MaxFileSize:      1024 * 1024 * 1024, // 1GB
		ScheduleCron:     "0 2 * * *", // 每天凌晨 2 点
		ScheduleEnabled:  true,
		RealtimeEnabled:  false,       // 默认关闭实时去重
		DebounceDuration: 5 * time.Second,
		MaxWorkers:       4,
		MaxMemoryMB:      512,
		HashCache:        true,
		DryRun:           false,
		VerifyAfter:      true,
		MaxRefPerFile:    1000,
	}
}

// Validate 验证配置.
func (c *Config) Validate() error {
	if c.Backend == "" {
		c.Backend = BackendAuto
	}
	if c.Mode == "" {
		c.Mode = ModeHybrid
	}
	if c.Action == "" {
		c.Action = ActionReflink
	}
	if c.MinFileSize < 0 {
		c.MinFileSize = 0
	}
	if c.MaxFileSize < 0 {
		c.MaxFileSize = 0
	}
	if c.MaxFileSize > 0 && c.MinFileSize > c.MaxFileSize {
		c.MinFileSize = c.MaxFileSize
	}
	if c.MaxWorkers <= 0 {
		c.MaxWorkers = 1
	}
	if c.MaxMemoryMB <= 0 {
		c.MaxMemoryMB = 256
	}
	if c.DebounceDuration <= 0 {
		c.DebounceDuration = 5 * time.Second
	}
	if c.MaxRefPerFile < 0 {
		c.MaxRefPerFile = 0
	}
	return nil
}

// LoadConfig 从文件加载配置.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// SaveConfig 保存配置到文件.
func SaveConfig(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
