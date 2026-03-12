package backup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// ExtendedBackupConfig 扩展备份配置
type ExtendedBackupConfig struct {
	BackupConfig

	// 增量备份配置
	Incremental      bool   `json:"incremental"`          // 启用增量备份
	IncrementalBase  string `json:"incrementalBase"`      // 增量基准目录

	// 云端备份配置
	CloudBackup      bool            `json:"cloudBackup"` // 启用云端备份
	CloudConfig      *CloudConfig    `json:"cloudConfig,omitempty"`

	// 加密配置
	Encryption       bool            `json:"encryption"`  // 启用加密
	EncryptionType   string          `json:"encryptionType"` // openssl, gpg, aes-gcm
	EncryptionKey    string          `json:"encryptionKey,omitempty"` // 密钥或密码
	EncryptionKeyFile string         `json:"encryptionKeyFile,omitempty"` // 密钥文件路径

	// 恢复配置
	AutoVerify       bool            `json:"autoVerify"`  // 自动验证备份
	RetentionCount   int             `json:"retentionCount"` // 保留备份数量
	RetentionDays    int             `json:"retentionDays"`  // 保留天数

	// 通知配置
	NotifyOnSuccess  bool            `json:"notifyOnSuccess"`
	NotifyOnFailure  bool            `json:"notifyOnFailure"`
	NotifyWebhook    string          `json:"notifyWebhook,omitempty"`

	// 高级选项
	ExcludePatterns  []string        `json:"excludePatterns"` // 排除模式
	IncludePatterns  []string        `json:"includePatterns"` // 包含模式
	MaxBandwidth     int64           `json:"maxBandwidth"`    // 带宽限制 (bytes/s)
	CompressionLevel int             `json:"compressionLevel"` // 压缩级别 1-9
}

// BackupStrategy 备份策略
type BackupStrategy struct {
	Name           string            `json:"name"`
	Type           BackupType        `json:"type"`
	Schedule       string            `json:"schedule"`       // cron 表达式
	Incremental    bool              `json:"incremental"`    // 是否增量
	CloudSync      bool              `json:"cloudSync"`      // 同步到云端
	Encryption     bool              `json:"encryption"`     // 是否加密
	Retention      RetentionPolicy   `json:"retention"`      // 保留策略
}

// RetentionPolicy 保留策略
type RetentionPolicy struct {
	KeepLast      int `json:"keepLast"`      // 保留最近 N 个
	KeepDaily     int `json:"keepDaily"`     // 保留最近 N 天的每日备份
	KeepWeekly    int `json:"keepWeekly"`    // 保留最近 N 周的每周备份
	KeepMonthly   int `json:"keepMonthly"`   // 保留最近 N 月的每月备份
	KeepYearly    int `json:"keepYearly"`    // 保留最近 N 年的每年备份
}

// DefaultRetentionPolicy 默认保留策略
func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		KeepLast:    7,    // 保留最近 7 个
		KeepDaily:   7,    // 保留最近 7 天
		KeepWeekly:  4,    // 保留最近 4 周
		KeepMonthly: 12,   // 保留最近 12 个月
		KeepYearly:  0,    // 不保留年度备份
	}
}

// CloudSyncConfig 云端同步配置
type CloudSyncConfig struct {
	Enabled        bool          `json:"enabled"`
	Provider       CloudProvider `json:"provider"`
	Bucket         string        `json:"bucket"`
	Region         string        `json:"region"`
	Endpoint       string        `json:"endpoint"`
	AccessKey      string        `json:"accessKey"`
	SecretKey      string        `json:"secretKey"`
	Prefix         string        `json:"prefix"`
	EncryptBefore  bool          `json:"encryptBefore"`
	VerifyAfter    bool          `json:"verifyAfter"`
	DeleteRemote   bool          `json:"deleteRemote"` // 本地删除时同步删除云端
}

// EncryptionConfig 加密配置
type EncryptionConfig struct {
	Enabled     bool   `json:"enabled"`
	Method      string `json:"method"` // openssl-aes-256-cbc, gpg, aes-gcm
	Key         string `json:"key"`
	KeyFile     string `json:"keyFile"`
	Salt        bool   `json:"salt"`        // 使用盐值
	Iterations  int    `json:"iterations"`  // KDF 迭代次数
}

// RestorePreset 恢复预设
type RestorePreset struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TargetPath  string `json:"targetPath"`
	Overwrite   bool   `json:"overwrite"`
	Verify      bool   `json:"verify"`
	Files       []string `json:"files,omitempty"` // 空表示全部
}

// DefaultRestorePresets 默认恢复预设
func DefaultRestorePresets() []RestorePreset {
	return []RestorePreset{
		{
			Name:        "full-restore",
			Description: "完整恢复到原路径",
			TargetPath:  "/restore/full",
			Overwrite:   true,
			Verify:      true,
		},
		{
			Name:        "single-file",
			Description: "恢复单个文件",
			TargetPath:  "/restore/single",
			Overwrite:   true,
			Verify:      false,
		},
		{
			Name:        "dry-run",
			Description: "仅预览，不实际恢复",
			TargetPath:  "",
			Overwrite:   false,
			Verify:      false,
		},
	}
}

// BackupStats 备份统计
type BackupStats struct {
	TotalBackups     int     `json:"totalBackups"`
	TotalSize        int64   `json:"totalSize"`
	TotalSizeHuman   string  `json:"totalSizeHuman"`
	LastBackupTime   string  `json:"lastBackupTime"`
	NextBackupTime   string  `json:"nextBackupTime"`
	AverageDuration  string  `json:"averageDuration"`
	SuccessRate      float64 `json:"successRate"` // 成功率 0-100
	IncrementalRatio float64 `json:"incrementalRatio"` // 增量备份节省空间比例
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	Status      string   `json:"status"` // healthy, warning, critical
	Checks      []Check  `json:"checks"`
	Recommendations []string `json:"recommendations"`
}

// Check 检查项
type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // pass, fail, warn
	Message string `json:"message"`
}

// HealthCheck 执行健康检查
func (m *Manager) HealthCheck() *HealthCheckResult {
	result := &HealthCheckResult{
		Status: "healthy",
		Checks: []Check{},
	}

	// 检查 1: 备份目录可写
	if err := m.checkBackupDir(); err != nil {
		result.Checks = append(result.Checks, Check{
			Name:    "backup_directory",
			Status:  "fail",
			Message: err.Error(),
		})
		result.Status = "critical"
	} else {
		result.Checks = append(result.Checks, Check{
			Name:    "backup_directory",
			Status:  "pass",
			Message: "备份目录可写",
		})
	}

	// 检查 2: 磁盘空间
	if space, err := m.checkDiskSpace(); err != nil || space < 10 { // 少于 10%
		result.Checks = append(result.Checks, Check{
			Name:    "disk_space",
			Status:  "warn",
			Message: fmt.Sprintf("磁盘空间不足：剩余 %.1f%%", space),
		})
		if result.Status == "healthy" {
			result.Status = "warning"
		}
	} else {
		result.Checks = append(result.Checks, Check{
			Name:    "disk_space",
			Status:  "pass",
			Message: fmt.Sprintf("磁盘空间充足：剩余 %.1f%%", space),
		})
	}

	// 检查 3: 最近备份状态
	if err := m.checkRecentBackups(); err != nil {
		result.Checks = append(result.Checks, Check{
			Name:    "recent_backups",
			Status:  "warn",
			Message: err.Error(),
		})
		result.Recommendations = append(result.Recommendations, "检查最近的备份任务是否成功")
		if result.Status == "healthy" {
			result.Status = "warning"
		}
	} else {
		result.Checks = append(result.Checks, Check{
			Name:    "recent_backups",
			Status:  "pass",
			Message: "最近备份正常",
		})
	}

	// 检查 4: 云端连接（如果配置）
	// TODO: 实现云端连接检查

	return result
}

func (m *Manager) checkBackupDir() error {
	testFile := filepath.Join(m.storagePath, ".health-check-"+fmt.Sprintf("%d", time.Now().UnixNano()))
	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		return err
	}
	os.Remove(testFile)
	return nil
}

func (m *Manager) checkDiskSpace() (float64, error) {
	// 使用 df 命令检查磁盘空间
	cmd := exec.Command("df", "--output=pcent", m.storagePath)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	var usage float64
	_, _ = fmt.Sscanf(string(output), "%f", &usage)
	return 100 - usage, nil
}

func (m *Manager) checkRecentBackups() error {
	// 检查最近 24 小时是否有成功备份
	// TODO: 实现详细检查
	return nil
}
