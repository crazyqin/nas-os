package backup

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ConfigBackupManager 配置备份管理器
// v2.29.0 新增 - 自动化配置备份功能
type ConfigBackupManager struct {
	mu sync.RWMutex

	// 配置路径
	configDir    string
	backupDir    string
	registryPath string

	// 备份配置
	maxBackups    int
	retentionDays int
	compress      bool

	// 备份记录
	registry *ConfigBackupRegistry

	// 定时任务
	ticker   *time.Ticker
	stopChan chan struct{}
	running  bool
}

// ConfigBackupConfig 配置备份设置
type ConfigBackupConfig struct {
	ConfigDir     string `json:"configDir"`
	BackupDir     string `json:"backupDir"`
	MaxBackups    int    `json:"maxBackups"`
	RetentionDays int    `json:"retentionDays"`
	Compress      bool   `json:"compress"`
	Schedule      string `json:"schedule"` // cron 表达式
}

// ConfigBackupRegistry 备份注册表
type ConfigBackupRegistry struct {
	Version   string                `json:"version"`
	Backups   []*ConfigBackupRecord `json:"backups"`
	UpdatedAt time.Time             `json:"updatedAt"`
}

// ConfigBackupRecord 备份记录
type ConfigBackupRecord struct {
	ID          string             `json:"id"`
	Filename    string             `json:"filename"`
	Size        int64              `json:"size"`
	Checksum    string             `json:"checksum"`
	CreatedAt   time.Time          `json:"createdAt"`
	Type        ConfigBackupType   `json:"type"`
	Description string             `json:"description"`
	Metadata    map[string]string  `json:"metadata,omitempty"`
	Status      ConfigBackupStatus `json:"status"`
}

// ConfigBackupType 备份类型
type ConfigBackupType string

const (
	ConfigBackupAuto      ConfigBackupType = "auto"       // 自动备份
	ConfigBackupManual    ConfigBackupType = "manual"     // 手动备份
	ConfigBackupPreUpdate ConfigBackupType = "pre_update" // 更新前备份
	ConfigBackupScheduled ConfigBackupType = "scheduled"  // 定时备份
)

// ConfigBackupStatus 配置备份状态
type ConfigBackupStatus string

const (
	ConfigBackupStatusCreating  ConfigBackupStatus = "creating"
	ConfigBackupStatusCompleted ConfigBackupStatus = "completed"
	ConfigBackupStatusFailed    ConfigBackupStatus = "failed"
	ConfigBackupStatusCorrupted ConfigBackupStatus = "corrupted"
)

// DefaultConfigBackupConfig 默认配置
func DefaultConfigBackupConfig() *ConfigBackupConfig {
	return &ConfigBackupConfig{
		ConfigDir:     "/etc/nas-os",
		BackupDir:     "/var/lib/nas-os/config-backups",
		MaxBackups:    30,
		RetentionDays: 90,
		Compress:      true,
		Schedule:      "0 3 * * *", // 每天凌晨 3 点
	}
}

// NewConfigBackupManager 创建配置备份管理器
func NewConfigBackupManager(config *ConfigBackupConfig) *ConfigBackupManager {
	if config == nil {
		config = DefaultConfigBackupConfig()
	}

	return &ConfigBackupManager{
		configDir:     config.ConfigDir,
		backupDir:     config.BackupDir,
		maxBackups:    config.MaxBackups,
		retentionDays: config.RetentionDays,
		compress:      config.Compress,
		registry: &ConfigBackupRegistry{
			Version: "1.0",
			Backups: make([]*ConfigBackupRecord, 0),
		},
		stopChan: make(chan struct{}),
	}
}

// Initialize 初始化
func (m *ConfigBackupManager) Initialize() error {
	// 创建备份目录
	if err := os.MkdirAll(m.backupDir, 0755); err != nil {
		return fmt.Errorf("创建备份目录失败: %w", err)
	}

	// 加载注册表
	m.registryPath = filepath.Join(m.backupDir, "registry.json")
	if err := m.loadRegistry(); err != nil {
		// 注册表不存在是正常的
		m.registry = &ConfigBackupRegistry{
			Version: "1.0",
			Backups: make([]*ConfigBackupRecord, 0),
		}
	}

	return nil
}

// CreateBackup 创建配置备份
func (m *ConfigBackupManager) CreateBackup(backupType ConfigBackupType, description string) (*ConfigBackupRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 生成备份 ID
	backupID := generateBackupID()
	timestamp := time.Now().Format("20060102_150405")

	// 创建备份文件名
	var filename string
	if m.compress {
		filename = fmt.Sprintf("config_%s_%s.tar.gz", timestamp, backupID[:8])
	} else {
		filename = fmt.Sprintf("config_%s_%s.tar", timestamp, backupID[:8])
	}

	backupPath := filepath.Join(m.backupDir, filename)

	// 创建备份记录
	record := &ConfigBackupRecord{
		ID:          backupID,
		Filename:    filename,
		CreatedAt:   time.Now(),
		Type:        backupType,
		Description: description,
		Metadata:    make(map[string]string),
		Status:      ConfigBackupStatusCreating,
	}

	// 执行备份
	size, checksum, err := m.createTarBackup(backupPath)
	if err != nil {
		record.Status = ConfigBackupStatusFailed
		record.Metadata["error"] = err.Error()
		m.registry.Backups = append(m.registry.Backups, record)
		_ = m.saveRegistry()
		return nil, fmt.Errorf("创建备份失败: %w", err)
	}

	record.Size = size
	record.Checksum = checksum
	record.Status = ConfigBackupStatusCompleted

	// 添加到注册表
	m.registry.Backups = append(m.registry.Backups, record)
	m.registry.UpdatedAt = time.Now()

	// 保存注册表
	if err := m.saveRegistry(); err != nil {
		return nil, fmt.Errorf("保存注册表失败: %w", err)
	}

	// 清理旧备份
	go m.cleanupOldBackups()

	return record, nil
}

// createTarBackup 创建 tar 备份
func (m *ConfigBackupManager) createTarBackup(backupPath string) (int64, string, error) {
	// 创建临时文件
	tmpPath := backupPath + ".tmp"

	var writer io.WriteCloser
	var err error

	if m.compress {
		writer, err = os.Create(tmpPath)
	} else {
		writer, err = os.Create(tmpPath)
	}
	if err != nil {
		return 0, "", fmt.Errorf("创建备份文件失败: %w", err)
	}
	defer func() { _ = writer.Close() }()

	var tarWriter *tar.Writer

	if m.compress {
		gzWriter := gzip.NewWriter(writer)
		defer func() { _ = gzWriter.Close() }()
		tarWriter = tar.NewWriter(gzWriter)
	} else {
		tarWriter = tar.NewWriter(writer)
	}
	defer func() { _ = tarWriter.Close() }()

	// 遍历配置目录
	err = filepath.Walk(m.configDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录本身
		if path == m.configDir {
			return nil
		}

		// 计算相对路径
		relPath, err := filepath.Rel(m.configDir, path)
		if err != nil {
			return err
		}

		// 创建 tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		// 写入 header
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// 如果是普通文件，写入内容
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer func() { _ = file.Close() }()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		_ = os.Remove(tmpPath)
		return 0, "", fmt.Errorf("打包配置失败: %w", err)
	}

	// 确保所有数据写入
	_ = tarWriter.Close()
	if m.compress {
		// gzWriter 已在 defer 中关闭
		_ = m.compress // 显式使用，避免空分支警告
	}
	_ = writer.Close()

	// 重命名临时文件
	if err := os.Rename(tmpPath, backupPath); err != nil {
		_ = os.Remove(tmpPath)
		return 0, "", fmt.Errorf("重命名备份文件失败: %w", err)
	}

	// 获取文件信息
	info, err := os.Stat(backupPath)
	if err != nil {
		return 0, "", err
	}

	// 计算校验和
	checksum, err := m.calculateChecksum(backupPath)
	if err != nil {
		return 0, "", err
	}

	return info.Size(), checksum, nil
}

// RestoreBackup 恢复配置备份
func (m *ConfigBackupManager) RestoreBackup(backupID string, dryRun bool) (*ConfigRestoreResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 查找备份记录
	var record *ConfigBackupRecord
	for _, r := range m.registry.Backups {
		if r.ID == backupID {
			record = r
			break
		}
	}

	if record == nil {
		return nil, fmt.Errorf("备份不存在: %s", backupID)
	}

	backupPath := filepath.Join(m.backupDir, record.Filename)

	// 验证备份文件
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("备份文件不存在: %s", backupPath)
	}

	// 验证校验和
	if record.Checksum != "" {
		checksum, err := m.calculateChecksum(backupPath)
		if err != nil {
			return nil, fmt.Errorf("计算校验和失败: %w", err)
		}
		if checksum != record.Checksum {
			return nil, fmt.Errorf("备份文件校验失败")
		}
	}

	result := &ConfigRestoreResult{
		BackupID:   backupID,
		RestoredAt: time.Now(),
		DryRun:     dryRun,
		Files:      make([]string, 0),
	}

	if dryRun {
		// 仅预览
		files, err := m.previewBackup(backupPath)
		if err != nil {
			return nil, err
		}
		result.Files = files
		result.TotalFiles = len(files)
		return result, nil
	}

	// 创建当前配置的备份（以防万一）
	preRestoreBackup, err := m.CreateBackup(ConfigBackupManual, "恢复前自动备份")
	if err != nil {
		return nil, fmt.Errorf("创建恢复前备份失败: %w", err)
	}
	result.PreRestoreBackupID = preRestoreBackup.ID

	// 执行恢复
	files, err := m.extractBackup(backupPath, m.configDir)
	if err != nil {
		return nil, fmt.Errorf("恢复备份失败: %w", err)
	}

	result.Files = files
	result.TotalFiles = len(files)

	return result, nil
}

// previewBackup 预览备份内容
func (m *ConfigBackupManager) previewBackup(backupPath string) ([]string, error) {
	file, err := os.Open(backupPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var reader io.Reader = file

	// 检查是否是 gzip 压缩
	if strings.HasSuffix(backupPath, ".gz") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		defer func() { _ = gzReader.Close() }()
		reader = gzReader
	}

	tarReader := tar.NewReader(reader)
	var files []string

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		files = append(files, header.Name)
	}

	return files, nil
}

// extractBackup 解压备份
func (m *ConfigBackupManager) extractBackup(backupPath, destDir string) ([]string, error) {
	file, err := os.Open(backupPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var reader io.Reader = file

	// 检查是否是 gzip 压缩
	if strings.HasSuffix(backupPath, ".gz") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		defer func() { _ = gzReader.Close() }()
		reader = gzReader
	}

	tarReader := tar.NewReader(reader)
	var files []string

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// 构建目标路径
		// #nosec G305 -- Path traversal is prevented by the check below
		targetPath := filepath.Join(destDir, header.Name)

		// 安全检查：防止路径遍历
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(destDir)) {
			return nil, fmt.Errorf("检测到路径遍历攻击: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return nil, err
			}
		case tar.TypeReg:
			// 确保目录存在
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return nil, err
			}

			// 创建文件
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return nil, err
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				_ = outFile.Close()
				return nil, err
			}
			_ = outFile.Close()

			files = append(files, header.Name)
		case tar.TypeSymlink:
			// 安全检查：防止符号链接指向目标目录之外
			linkTarget := header.Linkname
			// 解析符号链接的绝对路径
			var resolvedLink string
			if filepath.IsAbs(linkTarget) {
				resolvedLink = filepath.Clean(linkTarget)
			} else {
				resolvedLink = filepath.Clean(filepath.Join(destDir, linkTarget))
			}
			// 验证符号链接目标在目标目录内
			if !strings.HasPrefix(resolvedLink, filepath.Clean(destDir)) {
				return nil, fmt.Errorf("检测到符号链接攻击: %s -> %s", header.Name, linkTarget)
			}
			// 创建符号链接
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return nil, err
			}
		}
	}

	return files, nil
}

// ListBackups 列出所有备份
func (m *ConfigBackupManager) ListBackups() []*ConfigBackupRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ConfigBackupRecord, len(m.registry.Backups))
	copy(result, m.registry.Backups)

	// 按时间倒序排序
	for i := 0; i < len(result)/2; i++ {
		result[i], result[len(result)-1-i] = result[len(result)-1-i], result[i]
	}

	return result
}

// GetBackup 获取备份信息
func (m *ConfigBackupManager) GetBackup(backupID string) (*ConfigBackupRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, r := range m.registry.Backups {
		if r.ID == backupID {
			return r, nil
		}
	}

	return nil, fmt.Errorf("备份不存在: %s", backupID)
}

// DeleteBackup 删除备份
func (m *ConfigBackupManager) DeleteBackup(backupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, r := range m.registry.Backups {
		if r.ID == backupID {
			// 删除文件
			backupPath := filepath.Join(m.backupDir, r.Filename)
			_ = os.Remove(backupPath)

			// 从注册表移除
			m.registry.Backups = append(m.registry.Backups[:i], m.registry.Backups[i+1:]...)
			m.registry.UpdatedAt = time.Now()

			return m.saveRegistry()
		}
	}

	return fmt.Errorf("备份不存在: %s", backupID)
}

// cleanupOldBackups 清理旧备份
func (m *ConfigBackupManager) cleanupOldBackups() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	cutoff := now.AddDate(0, 0, -m.retentionDays)

	var validBackups []*ConfigBackupRecord

	for _, r := range m.registry.Backups {
		// 检查是否过期
		if r.CreatedAt.Before(cutoff) {
			// 删除过期备份
			backupPath := filepath.Join(m.backupDir, r.Filename)
			_ = os.Remove(backupPath)
			continue
		}
		validBackups = append(validBackups, r)
	}

	// 检查备份数量限制
	if len(validBackups) > m.maxBackups {
		// 按时间排序，保留最新的
		// 假设已经按时间排序
		toRemove := validBackups[:len(validBackups)-m.maxBackups]
		for _, r := range toRemove {
			backupPath := filepath.Join(m.backupDir, r.Filename)
			_ = os.Remove(backupPath)
		}
		validBackups = validBackups[len(validBackups)-m.maxBackups:]
	}

	m.registry.Backups = validBackups
	m.registry.UpdatedAt = now
	_ = m.saveRegistry()
}

// calculateChecksum 计算校验和
func (m *ConfigBackupManager) calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	// 使用 SHA256
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil))[:16], nil
}

// loadRegistry 加载注册表
func (m *ConfigBackupManager) loadRegistry() error {
	data, err := os.ReadFile(m.registryPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &m.registry)
}

// saveRegistry 保存注册表
func (m *ConfigBackupManager) saveRegistry() error {
	data, err := json.MarshalIndent(m.registry, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.registryPath, data, 0644)
}

// StartScheduledBackup 启动定时备份
func (m *ConfigBackupManager) StartScheduledBackup(interval time.Duration) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	m.ticker = time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-m.ticker.C:
				_, _ = m.CreateBackup(ConfigBackupScheduled, "定时配置备份")
			case <-m.stopChan:
				return
			}
		}
	}()
}

// StopScheduledBackup 停止定时备份
func (m *ConfigBackupManager) StopScheduledBackup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		m.ticker.Stop()
		close(m.stopChan)
		m.running = false
	}
}

// ConfigRestoreResult 配置恢复结果
type ConfigRestoreResult struct {
	BackupID           string    `json:"backupId"`
	PreRestoreBackupID string    `json:"preRestoreBackupId,omitempty"`
	RestoredAt         time.Time `json:"restoredAt"`
	DryRun             bool      `json:"dryRun"`
	Files              []string  `json:"files"`
	TotalFiles         int       `json:"totalFiles"`
}

// generateBackupID 生成备份 ID
func generateBackupID() string {
	return fmt.Sprintf("cfg_%d_%s", time.Now().Unix(), randomString(8))
}
