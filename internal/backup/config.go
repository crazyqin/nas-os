package backup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ExtendedBackupConfig 扩展备份配置
type ExtendedBackupConfig struct {
	BackupConfig

	// 增量备份配置
	Incremental     bool   `json:"incremental"`     // 启用增量备份
	IncrementalBase string `json:"incrementalBase"` // 增量基准目录

	// 云端备份配置
	CloudBackup bool         `json:"cloudBackup"` // 启用云端备份
	CloudConfig *CloudConfig `json:"cloudConfig,omitempty"`

	// 加密配置
	Encryption        bool   `json:"encryption"`                  // 启用加密
	EncryptionType    string `json:"encryptionType"`              // openssl, gpg, aes-gcm
	EncryptionKey     string `json:"encryptionKey,omitempty"`     // 密钥或密码
	EncryptionKeyFile string `json:"encryptionKeyFile,omitempty"` // 密钥文件路径

	// 恢复配置
	AutoVerify     bool `json:"autoVerify"`     // 自动验证备份
	RetentionCount int  `json:"retentionCount"` // 保留备份数量
	RetentionDays  int  `json:"retentionDays"`  // 保留天数

	// 通知配置
	NotifyOnSuccess bool   `json:"notifyOnSuccess"`
	NotifyOnFailure bool   `json:"notifyOnFailure"`
	NotifyWebhook   string `json:"notifyWebhook,omitempty"`

	// 高级选项
	ExcludePatterns  []string `json:"excludePatterns"`  // 排除模式
	IncludePatterns  []string `json:"includePatterns"`  // 包含模式
	MaxBandwidth     int64    `json:"maxBandwidth"`     // 带宽限制 (bytes/s)
	CompressionLevel int      `json:"compressionLevel"` // 压缩级别 1-9
}

// BackupStrategy 备份策略
type BackupStrategy struct {
	Name        string          `json:"name"`
	Type        BackupType      `json:"type"`
	Schedule    string          `json:"schedule"`    // cron 表达式
	Incremental bool            `json:"incremental"` // 是否增量
	CloudSync   bool            `json:"cloudSync"`   // 同步到云端
	Encryption  bool            `json:"encryption"`  // 是否加密
	Retention   RetentionPolicy `json:"retention"`   // 保留策略
}

// RetentionPolicy 保留策略
type RetentionPolicy struct {
	KeepLast    int `json:"keepLast"`    // 保留最近 N 个
	KeepDaily   int `json:"keepDaily"`   // 保留最近 N 天的每日备份
	KeepWeekly  int `json:"keepWeekly"`  // 保留最近 N 周的每周备份
	KeepMonthly int `json:"keepMonthly"` // 保留最近 N 月的每月备份
	KeepYearly  int `json:"keepYearly"`  // 保留最近 N 年的每年备份
}

// DefaultRetentionPolicy 默认保留策略
func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		KeepLast:    7,  // 保留最近 7 个
		KeepDaily:   7,  // 保留最近 7 天
		KeepWeekly:  4,  // 保留最近 4 周
		KeepMonthly: 12, // 保留最近 12 个月
		KeepYearly:  0,  // 不保留年度备份
	}
}

// CloudSyncConfig 云端同步配置
type CloudSyncConfig struct {
	Enabled       bool          `json:"enabled"`
	Provider      CloudProvider `json:"provider"`
	Bucket        string        `json:"bucket"`
	Region        string        `json:"region"`
	Endpoint      string        `json:"endpoint"`
	AccessKey     string        `json:"accessKey"`
	SecretKey     string        `json:"secretKey"`
	Prefix        string        `json:"prefix"`
	EncryptBefore bool          `json:"encryptBefore"`
	VerifyAfter   bool          `json:"verifyAfter"`
	DeleteRemote  bool          `json:"deleteRemote"` // 本地删除时同步删除云端
}

// EncryptionConfig 加密配置
type EncryptionConfig struct {
	Enabled    bool   `json:"enabled"`
	Method     string `json:"method"` // openssl-aes-256-cbc, gpg, aes-gcm
	Key        string `json:"key"`
	KeyFile    string `json:"keyFile"`
	Salt       bool   `json:"salt"`       // 使用盐值
	Iterations int    `json:"iterations"` // KDF 迭代次数
}

// RestorePreset 恢复预设
type RestorePreset struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	TargetPath  string   `json:"targetPath"`
	Overwrite   bool     `json:"overwrite"`
	Verify      bool     `json:"verify"`
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
	SuccessRate      float64 `json:"successRate"`      // 成功率 0-100
	IncrementalRatio float64 `json:"incrementalRatio"` // 增量备份节省空间比例
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	Status          string   `json:"status"` // healthy, warning, critical
	Checks          []Check  `json:"checks"`
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

	// 检查 4: 云端连接（遍历所有配置）
	cloudConfigsChecked := 0
	cloudConfigsPassed := 0
	m.mu.RLock()
	for _, cfg := range m.configs {
		if cfg.CloudBackup && cfg.CloudConfig != nil {
			cloudConfigsChecked++
			if err := m.checkCloudConnection(cfg.CloudConfig); err != nil {
				result.Checks = append(result.Checks, Check{
					Name:    "cloud_connection",
					Status:  "warn",
					Message: fmt.Sprintf("云端连接异常 (%s): %v", cfg.Name, err),
				})
				result.Recommendations = append(result.Recommendations, fmt.Sprintf("检查 %s 的云端配置和网络连接", cfg.Name))
				if result.Status == "healthy" {
					result.Status = "warning"
				}
			} else {
				cloudConfigsPassed++
			}
		}
	}
	m.mu.RUnlock()

	if cloudConfigsChecked > 0 {
		if cloudConfigsPassed == cloudConfigsChecked {
			result.Checks = append(result.Checks, Check{
				Name:    "cloud_connection",
				Status:  "pass",
				Message: fmt.Sprintf("云端连接正常 (%d/%d)", cloudConfigsPassed, cloudConfigsChecked),
			})
		}
	}

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
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	cutoff := now.Add(-24 * time.Hour)

	for _, task := range m.tasks {
		if task.Status == TaskStatusCompleted && task.EndTime.After(cutoff) {
			return nil // 找到 24 小时内的成功备份
		}
	}

	// 如果没有找到，检查历史记录
	entries, err := os.ReadDir(m.storagePath)
	if err != nil {
		return fmt.Errorf("读取备份目录失败：%w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "backup-") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(cutoff) {
				return nil // 找到 24 小时内的备份目录
			}
		}
	}

	return fmt.Errorf("最近 24 小时内没有成功备份")
}

// CheckConfigDetailed 详细检查备份配置
func (m *Manager) CheckConfigDetailed(configID string) (*DetailedConfigCheck, error) {
	cfg, err := m.GetConfig(configID)
	if err != nil {
		return nil, err
	}

	result := &DetailedConfigCheck{
		ConfigID: configID,
		Status:   "pass",
		Checks:   []ConfigCheckItem{},
	}

	// 检查 1: 源目录是否存在
	if err := m.checkSourceDirectory(cfg.Source); err != nil {
		result.Checks = append(result.Checks, ConfigCheckItem{
			Name:    "source_directory",
			Status:  "fail",
			Message: fmt.Sprintf("源目录不存在：%v", err),
		})
		result.Status = "fail"
	} else {
		result.Checks = append(result.Checks, ConfigCheckItem{
			Name:    "source_directory",
			Status:  "pass",
			Message: "源目录存在且可访问",
		})
	}

	// 检查 2: 备份目标是否可写
	if err := m.checkBackupDestination(cfg.Destination); err != nil {
		result.Checks = append(result.Checks, ConfigCheckItem{
			Name:    "backup_destination",
			Status:  "fail",
			Message: fmt.Sprintf("备份目标不可写：%v", err),
		})
		result.Status = "fail"
	} else {
		result.Checks = append(result.Checks, ConfigCheckItem{
			Name:    "backup_destination",
			Status:  "pass",
			Message: "备份目标可写",
		})
	}

	// 检查 3: 云端连接（如果启用）
	if cfg.CloudBackup && cfg.CloudConfig != nil {
		if err := m.checkCloudConnection(cfg.CloudConfig); err != nil {
			result.Checks = append(result.Checks, ConfigCheckItem{
				Name:    "cloud_connection",
				Status:  "warn",
				Message: fmt.Sprintf("云端连接检查失败：%v", err),
			})
			if result.Status == "pass" {
				result.Status = "warn"
			}
		} else {
			result.Checks = append(result.Checks, ConfigCheckItem{
				Name:    "cloud_connection",
				Status:  "pass",
				Message: "云端连接正常",
			})
		}
	}

	// 检查 4: 磁盘空间
	if err := m.checkBackupDiskSpace(*cfg); err != nil {
		result.Checks = append(result.Checks, ConfigCheckItem{
			Name:    "disk_space",
			Status:  "warn",
			Message: fmt.Sprintf("磁盘空间可能不足：%v", err),
		})
		if result.Status == "pass" {
			result.Status = "warn"
		}
	} else {
		result.Checks = append(result.Checks, ConfigCheckItem{
			Name:    "disk_space",
			Status:  "pass",
			Message: "磁盘空间充足",
		})
	}

	// 检查 5: 加密配置（如果启用）
	if cfg.Encryption {
		if cfg.EncryptionKey == "" && cfg.EncryptionKeyFile == "" {
			result.Checks = append(result.Checks, ConfigCheckItem{
				Name:    "encryption_config",
				Status:  "fail",
				Message: "启用加密但未配置密钥",
			})
			result.Status = "fail"
		} else {
			result.Checks = append(result.Checks, ConfigCheckItem{
				Name:    "encryption_config",
				Status:  "pass",
				Message: "加密配置正确",
			})
		}
	}

	return result, nil
}

// DetailedConfigCheck 详细配置检查结果
type DetailedConfigCheck struct {
	ConfigID string            `json:"configId"`
	Status   string            `json:"status"` // pass, warn, fail
	Checks   []ConfigCheckItem `json:"checks"`
}

// ConfigCheckItem 配置检查项
type ConfigCheckItem struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // pass, warn, fail
	Message string `json:"message"`
}

func (m *Manager) checkSourceDirectory(source string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("源路径不是目录")
	}
	// 检查是否可读
	testFile := filepath.Join(source, ".read-test-"+fmt.Sprintf("%d", time.Now().UnixNano()))
	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		return err
	}
	os.Remove(testFile)
	return nil
}

func (m *Manager) checkBackupDestination(dest string) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	testFile := filepath.Join(dest, ".write-test-"+fmt.Sprintf("%d", time.Now().UnixNano()))
	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		return err
	}
	os.Remove(testFile)
	return nil
}

func (m *Manager) checkCloudConnection(cfg *CloudConfig) error {
	cloud, err := NewCloudBackup(*cfg)
	if err != nil {
		return err
	}
	result, err := cloud.CheckConnection()
	if err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("%s", result.Message)
	}
	return nil
}

func (m *Manager) checkBackupDiskSpace(config BackupConfig) error {
	// 估算源目录大小
	var totalSize int64
	err := filepath.Walk(config.Source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("估算源目录大小失败：%w", err)
	}

	// 检查目标磁盘空间
	cmd := exec.Command("df", "--output=avail", "-B", "1", config.Destination)
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	var availSpace int64
	_, _ = fmt.Sscanf(string(output), "%d", &availSpace)

	if availSpace < totalSize {
		return fmt.Errorf("可用空间 (%.2f GB) 小于源目录大小 (%.2f GB)",
			float64(availSpace)/1024/1024/1024,
			float64(totalSize)/1024/1024/1024)
	}

	// 如果启用压缩，所需空间会更小，这里保守估计
	return nil
}
