package backup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Config 备份配置
type Config struct {
	// 基本配置
	BackupPath string `json:"backup_path"`
	ChunkPath  string `json:"chunk_path"`
	TempPath   string `json:"temp_path"`

	// 调度配置
	Schedule      string `json:"schedule"`       // cron 表达式
	RetentionDays int    `json:"retention_days"` // 保留天数
	MaxBackups    int    `json:"max_backups"`    // 最大备份数

	// 增量备份配置
	IncrementalEnabled bool `json:"incremental_enabled"`
	FullBackupInterval int  `json:"full_backup_interval"` // 每多少次增量后执行完整备份

	// 加密配置
	EncryptionEnabled bool   `json:"encryption_enabled"`
	EncryptionKeyID   string `json:"encryption_key_id"` // 只存储 KeyID，不存储实际密钥

	// 压缩配置
	CompressionEnabled bool   `json:"compression_enabled"`
	CompressionLevel   int    `json:"compression_level"`
	CompressionAlgo    string `json:"compression_algo"`

	// 验证配置
	VerifyAfterBackup bool `json:"verify_after_backup"`

	// 性能配置
	MaxParallelFiles int           `json:"max_parallel_files"`
	ChunkSize        int64         `json:"chunk_size"` // 块大小（字节）
	IOBufferSize     int           `json:"io_buffer_size"`
	Timeout          time.Duration `json:"timeout"`

	// 目标配置
	Targets []Target `json:"targets"`
}

// Sanitize 返回脱敏后的配置副本（用于日志和调试）
func (bc *Config) Sanitize() map[string]interface{} {
	targets := make([]map[string]interface{}, len(bc.Targets))
	for i, t := range bc.Targets {
		targets[i] = t.SanitizeConfig()
	}

	return map[string]interface{}{
		"backup_path":         bc.BackupPath,
		"chunk_path":          bc.ChunkPath,
		"schedule":            bc.Schedule,
		"retention_days":      bc.RetentionDays,
		"max_backups":         bc.MaxBackups,
		"incremental_enabled": bc.IncrementalEnabled,
		"encryption_enabled":  bc.EncryptionEnabled,
		"encryption_key_id":   bc.EncryptionKeyID,
		"compression_enabled": bc.CompressionEnabled,
		"verify_after_backup": bc.VerifyAfterBackup,
		"max_parallel_files":  bc.MaxParallelFiles,
		"timeout":             bc.Timeout.String(),
		"targets":             targets,
	}
}

// Target 备份目标
type Target struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"` // local, s3, sftp
	Path        string            `json:"path"`
	Credentials map[string]string `json:"-"` // 敏感信息，不序列化到 JSON
	Enabled     bool              `json:"enabled"`
}

// SanitizeConfig 返回脱敏后的配置副本（用于日志和调试）
func (bt *Target) SanitizeConfig() map[string]interface{} {
	return map[string]interface{}{
		"name":            bt.Name,
		"type":            bt.Type,
		"path":            bt.Path,
		"enabled":         bt.Enabled,
		"has_credentials": len(bt.Credentials) > 0,
	}
}

// 兼容类型别名（保持向后兼容）
type BackupConfig = Config
type BackupTarget = Target

// ConfigManager 配置管理器
type ConfigManager struct {
	configPath string
	config     *Config
	mu         sync.RWMutex
}

// NewConfigManager 创建配置管理器
func NewConfigManager(configPath string) *ConfigManager {
	cm := &ConfigManager{
		configPath: configPath,
		config:     DefaultConfig(),
	}
	_ = cm.load() // 使用默认配置，忽略加载错误
	return cm
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		BackupPath:         "/var/lib/nas-os/backups",
		ChunkPath:          "/var/lib/nas-os/backups/chunks",
		TempPath:           "/var/lib/nas-os/backups/temp",
		RetentionDays:      30,
		MaxBackups:         10,
		IncrementalEnabled: true,
		FullBackupInterval: 7,
		CompressionEnabled: true,
		CompressionLevel:   6,
		CompressionAlgo:    "gzip",
		VerifyAfterBackup:  true,
		MaxParallelFiles:   10,
		ChunkSize:          4 * 1024 * 1024, // 4MB
		IOBufferSize:       64 * 1024,       // 64KB
		Timeout:            30 * time.Minute,
		Targets:            make([]Target, 0),
	}
}

// load 加载配置
func (cm *ConfigManager) load() error {
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cm.Save()
		}
		return err
	}

	return json.Unmarshal(data, cm.config)
}

// Save 保存配置
func (cm *ConfigManager) Save() error {
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(cm.configPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cm.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cm.configPath, data, 0644)
}

// Get 获取配置
func (cm *ConfigManager) Get() *Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}

// Update 更新配置
func (cm *ConfigManager) Update(config *Config) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.config = config
	return cm.Save()
}

// SetBackupPath 设置备份路径
func (cm *ConfigManager) SetBackupPath(path string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.config.BackupPath = path
	return cm.Save()
}

// AddTarget 添加备份目标
func (cm *ConfigManager) AddTarget(target Target) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.config.Targets = append(cm.config.Targets, target)
	return cm.Save()
}

// RemoveTarget 移除备份目标
func (cm *ConfigManager) RemoveTarget(name string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for i, t := range cm.config.Targets {
		if t.Name == name {
			cm.config.Targets = append(cm.config.Targets[:i], cm.config.Targets[i+1:]...)
			break
		}
	}
	return cm.Save()
}

// RetentionPolicy 保留策略
type RetentionPolicy struct {
	KeepDaily   int `json:"keep_daily"`   // 保留每日备份数
	KeepWeekly  int `json:"keep_weekly"`  // 保留每周备份数
	KeepMonthly int `json:"keep_monthly"` // 保留每月备份数
	KeepYearly  int `json:"keep_yearly"`  // 保留每年备份数
	MaxAge      int `json:"max_age"`      // 最大保留天数
}

// DefaultRetentionPolicy 默认保留策略
func DefaultRetentionPolicy() *RetentionPolicy {
	return &RetentionPolicy{
		KeepDaily:   7,
		KeepWeekly:  4,
		KeepMonthly: 12,
		KeepYearly:  3,
		MaxAge:      365,
	}
}

// ShouldKeep 判断是否应该保留
func (rp *RetentionPolicy) ShouldKeep(snapshot *Snapshot, now time.Time) bool {
	age := now.Sub(snapshot.CreatedAt)

	// 超过最大年龄
	if age.Hours() > float64(rp.MaxAge*24) {
		return false
	}

	// 当天备份保留
	if age.Hours() < 24 {
		return true
	}

	return false
}

// Policy 备份策略
type Policy struct {
	Name        string               `json:"name"`
	Source      string               `json:"source"`
	Schedule    string               `json:"schedule"`
	Type        SnapshotType         `json:"type"`
	Retention   *RetentionPolicy     `json:"retention"`
	Encryption  *EncryptionSettings  `json:"encryption,omitempty"`
	Compression *CompressionSettings `json:"compression,omitempty"`
	Enabled     bool                 `json:"enabled"`
}

// EncryptionSettings 加密设置
type EncryptionSettings struct {
	Enabled   bool   `json:"enabled"`
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"key_id"`
}

// CompressionSettings 压缩设置
type CompressionSettings struct {
	Algorithm string `json:"algorithm"` // gzip, lz4, zstd
	Level     int    `json:"level"`     // 压缩级别
}

// PolicyManager 策略管理器
type PolicyManager struct {
	policies map[string]*Policy
	mu       sync.RWMutex
}

// NewPolicyManager 创建策略管理器
func NewPolicyManager() *PolicyManager {
	return &PolicyManager{
		policies: make(map[string]*Policy),
	}
}

// Add 添加策略
func (pm *PolicyManager) Add(policy *Policy) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.policies[policy.Name] = policy
}

// Get 获取策略
func (pm *PolicyManager) Get(name string) *Policy {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.policies[name]
}

// List 列出策略
func (pm *PolicyManager) List() []*Policy {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	policies := make([]*Policy, 0, len(pm.policies))
	for _, p := range pm.policies {
		policies = append(policies, p)
	}
	return policies
}

// Remove 移除策略
func (pm *PolicyManager) Remove(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.policies, name)
}

// Update 更新策略
func (pm *PolicyManager) Update(policy *Policy) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.policies[policy.Name] = policy
}

// 兼容类型别名
type BackupPolicy = Policy
