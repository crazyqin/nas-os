// Package dedup 数据去重模块配置
package dedup

import (
	"encoding/json"
	"os"
	"time"
)

// ChunkSize 块大小常量
const (
	DefaultChunkSize = 4 * 1024         // 4KB 默认块大小
	MinChunkSize     = 512              // 最小块大小
	MaxChunkSize     = 64 * 1024 * 1024 // 最大块大小 64MB
)

// Strategy 去重策略
type Strategy string

const (
	StrategyInline Strategy = "inline" // 内联去重 - 写入时实时去重
	StrategyBatch  Strategy = "batch"  // 批量去重 - 定期批量处理
	StrategyHybrid Strategy = "hybrid" // 混合模式 - 结合内联和批量
)

// RetentionPolicy 保留策略
type RetentionPolicy string

const (
	RetentionKeepOldest  RetentionPolicy = "keep_oldest"  // 保留最旧的文件
	RetentionKeepNewest  RetentionPolicy = "keep_newest"  // 保留最新的文件
	RetentionKeepLargest RetentionPolicy = "keep_largest" // 保留最大的文件
	RetentionKeepFirst   RetentionPolicy = "keep_first"   // 保留扫描到的第一个
	RetentionManual      RetentionPolicy = "manual"       // 手动选择
)

// DedupAction 去重操作类型
type DedupAction string

const (
	ActionReport   DedupAction = "report"   // 仅报告
	ActionSoftlink DedupAction = "softlink" // 创建软链接
	ActionHardlink DedupAction = "hardlink" // 创建硬链接
	ActionRemove   DedupAction = "remove"   // 直接删除重复文件
)

// DedupMode 去重模式
type DedupMode string

const (
	ModeFile  DedupMode = "file"  // 文件级去重
	ModeChunk DedupMode = "chunk" // 块级去重
	ModeAuto  DedupMode = "auto"  // 自动选择
)

// ChunkStoreConfig 块存储配置
type ChunkStoreConfig struct {
	Enabled    bool   `json:"enabled"`
	BasePath   string `json:"basePath"`
	MaxSize    int64  `json:"maxSize"`    // 最大存储大小 (0 = 无限制)
	CleanupAge int    `json:"cleanupAge"` // 清理超过此天数未访问的块 (0 = 不清理)
}

// Config 去重配置
type Config struct {
	// 基本配置
	Enabled     bool  `json:"enabled"`
	ChunkSize   int64 `json:"chunkSize"`   // 块大小 (字节)
	MinFileSize int64 `json:"minFileSize"` // 最小文件大小

	// 扫描配置
	ScanPaths       []string `json:"scanPaths"`
	ExcludePaths    []string `json:"excludePaths"`
	ExcludePatterns []string `json:"excludePatterns"`

	// 去重策略
	Strategy        Strategy        `json:"strategy"`        // 去重策略
	DedupMode       DedupMode       `json:"dedupMode"`       // 去重模式
	DedupAction     DedupAction     `json:"dedupAction"`     // 去重操作
	RetentionPolicy RetentionPolicy `json:"retentionPolicy"` // 保留策略

	// 自动去重
	AutoDedup     bool   `json:"autoDedup"`
	AutoDedupCron string `json:"autoDedupCron"`

	// 高级选项
	CrossUser   bool              `json:"crossUser"`   // 允许跨用户去重
	Compression bool              `json:"compression"` // 启用压缩
	DryRun      bool              `json:"dryRun"`      // 试运行模式
	ChunkStore  *ChunkStoreConfig `json:"chunkStore,omitempty"`

	// 性能配置
	MaxWorkers     int  `json:"maxWorkers"`     // 最大并行工作数
	MaxMemoryMB    int  `json:"maxMemoryMB"`    // 最大内存使用 (MB)
	ProgressReport bool `json:"progressReport"` // 启用进度报告

	// 安全配置
	VerifyAfterDedup bool `json:"verifyAfterDedup"` // 去重后验证
	CreateBackup     bool `json:"createBackup"`     // 创建备份
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:         true,
		ChunkSize:       DefaultChunkSize,
		MinFileSize:     1024, // 1KB
		Strategy:        StrategyBatch,
		DedupMode:       ModeFile,
		DedupAction:     ActionReport,
		RetentionPolicy: RetentionKeepOldest,
		ScanPaths:       []string{},
		ExcludePaths:    []string{"/proc", "/sys", "/dev", "/tmp", "/run"},
		ExcludePatterns: []string{"*.tmp", "*.log", "*.cache", "*.swp", "*.bak"},
		AutoDedup:       false,
		AutoDedupCron:   "0 3 * * *", // 每天凌晨 3 点
		CrossUser:       true,
		Compression:     true,
		DryRun:          false,
		ChunkStore: &ChunkStoreConfig{
			Enabled:    false,
			BasePath:   "/var/lib/nas-os/chunks",
			MaxSize:    0,
			CleanupAge: 30,
		},
		MaxWorkers:       4,
		MaxMemoryMB:      512,
		ProgressReport:   true,
		VerifyAfterDedup: true,
		CreateBackup:     false,
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.ChunkSize < MinChunkSize {
		c.ChunkSize = MinChunkSize
	}
	if c.ChunkSize > MaxChunkSize {
		c.ChunkSize = MaxChunkSize
	}

	if c.MaxWorkers <= 0 {
		c.MaxWorkers = 4
	}
	if c.MaxWorkers > 32 {
		c.MaxWorkers = 32
	}

	if c.MaxMemoryMB <= 0 {
		c.MaxMemoryMB = 512
	}

	// 验证策略
	switch c.Strategy {
	case StrategyInline, StrategyBatch, StrategyHybrid:
	default:
		c.Strategy = StrategyBatch
	}

	// 验证模式
	switch c.DedupMode {
	case ModeFile, ModeChunk, ModeAuto:
	default:
		c.DedupMode = ModeFile
	}

	// 验证操作
	switch c.DedupAction {
	case ActionReport, ActionSoftlink, ActionHardlink, ActionRemove:
	default:
		c.DedupAction = ActionReport
	}

	// 验证保留策略
	switch c.RetentionPolicy {
	case RetentionKeepOldest, RetentionKeepNewest, RetentionKeepLargest, RetentionKeepFirst, RetentionManual:
	default:
		c.RetentionPolicy = RetentionKeepOldest
	}

	return nil
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := DefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}

	config.Validate()
	return config, nil
}

// SaveConfig 保存配置到文件
func (c *Config) SaveConfig(path string) error {
	c.Validate()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Clone 克隆配置
func (c *Config) Clone() *Config {
	data, _ := json.Marshal(c)
	newConfig := &Config{}
	json.Unmarshal(data, newConfig)
	return newConfig
}

// DedupPolicy 去重策略（运行时策略）
type DedupPolicy struct {
	Mode          DedupMode       `json:"mode"`          // 去重模式
	Action        DedupAction     `json:"action"`        // 去重操作
	Retention     RetentionPolicy `json:"retention"`     // 保留策略
	MinMatchCount int             `json:"minMatchCount"` // 最小匹配数量
	PreserveAttrs bool            `json:"preserveAttrs"` // 保留文件属性
	CrossUser     bool            `json:"crossUser"`     // 允许跨用户去重
	DryRun        bool            `json:"dryRun"`        // 试运行模式
}

// DefaultPolicy 默认策略
func DefaultPolicy() *DedupPolicy {
	return &DedupPolicy{
		Mode:          ModeFile,
		Action:        ActionReport,
		Retention:     RetentionKeepOldest,
		MinMatchCount: 2,
		PreserveAttrs: true,
		CrossUser:     true,
		DryRun:        false,
	}
}

// ToPolicy 从配置创建策略
func (c *Config) ToPolicy() *DedupPolicy {
	return &DedupPolicy{
		Mode:          c.DedupMode,
		Action:        c.DedupAction,
		Retention:     c.RetentionPolicy,
		MinMatchCount: 2,
		PreserveAttrs: true,
		CrossUser:     c.CrossUser,
		DryRun:        c.DryRun,
	}
}

// ScheduleConfig 调度配置
type ScheduleConfig struct {
	Enabled  bool          `json:"enabled"`
	Cron     string        `json:"cron"`
	Timezone string        `json:"timezone"`
	Timeout  time.Duration `json:"timeout"`
}

// DefaultScheduleConfig 默认调度配置
func DefaultScheduleConfig() *ScheduleConfig {
	return &ScheduleConfig{
		Enabled:  false,
		Cron:     "0 3 * * *",
		Timezone: "Local",
		Timeout:  2 * time.Hour,
	}
}
