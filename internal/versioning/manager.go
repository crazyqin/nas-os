// Package versioning 文件版本控制模块
package versioning

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Version 表示文件的一个版本
type Version struct {
	ID           string    `json:"id"`
	FilePath     string    `json:"filePath"`
	VersionPath  string    `json:"versionPath"`
	Checksum     string    `json:"checksum"`
	Size         int64     `json:"size"`
	CreatedAt    time.Time `json:"createdAt"`
	CreatedBy    string    `json:"createdBy"`
	Description  string    `json:"description"`
	TriggerType  string    `json:"triggerType"` // manual, time, change
	Tags         []string  `json:"tags"`
	ExpiresAt    time.Time `json:"expiresAt"`
	IsCompressed bool      `json:"isCompressed"`
}

// RetentionPolicy 版本保留策略
type RetentionPolicy struct {
	MaxVersions    int   `json:"maxVersions"`    // 最大版本数量，0 表示无限制
	MaxAge         int   `json:"maxAge"`         // 最大保留天数，0 表示无限制
	MaxSpace       int64 `json:"maxSpace"`       // 最大占用空间 (字节)，0 表示无限制
	AutoCleanup    bool  `json:"autoCleanup"`    // 自动清理过期版本
	SnapshotOnSave bool  `json:"snapshotOnSave"` // 保存时自动创建快照
}

// SnapshotConfig 快照配置
type SnapshotConfig struct {
	Enabled       bool   `json:"enabled"`
	TriggerMode   string `json:"triggerMode"`   // time, change, manual
	Interval      int    `json:"interval"`      // 时间间隔（分钟）
	MinChangeSize int64  `json:"minChangeSize"` // 最小变更大小触发快照
}

// Config 版本控制配置
type Config struct {
	Enabled      bool            `json:"enabled"`
	VersionRoot  string          `json:"versionRoot"`
	Retention    RetentionPolicy `json:"retention"`
	Snapshot     SnapshotConfig  `json:"snapshot"`
	ExcludePaths []string        `json:"excludePaths"`
	MaxFileSize  int64           `json:"maxFileSize"` // 最大支持的文件大小
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:     true,
		VersionRoot: "/var/lib/nas-os/versions",
		Retention: RetentionPolicy{
			MaxVersions:    50,
			MaxAge:         30,
			MaxSpace:       10 * 1024 * 1024 * 1024, // 10GB
			AutoCleanup:    true,
			SnapshotOnSave: false,
		},
		Snapshot: SnapshotConfig{
			Enabled:     true,
			TriggerMode: "manual",
			Interval:    60,
		},
		ExcludePaths: []string{"/tmp", "/var/tmp", "/proc", "/sys"},
		MaxFileSize:  5 * 1024 * 1024 * 1024, // 5GB
	}
}

// Manager 版本控制管理器
type Manager struct {
	mu          sync.RWMutex
	config      *Config
	versions    map[string][]*Version // filePath -> versions
	allVersions map[string]*Version   // id -> version
	configPath  string
	totalSize   int64
	watchers    map[string]time.Time // 监控的文件 -> 上次修改时间
	stopChan    chan struct{}
}

// NewManager 创建版本控制管理器
func NewManager(configPath string, config *Config) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	m := &Manager{
		config:      config,
		versions:    make(map[string][]*Version),
		allVersions: make(map[string]*Version),
		configPath:  configPath,
		watchers:    make(map[string]time.Time),
		stopChan:    make(chan struct{}),
	}

	// 创建版本存储根目录
	if err := os.MkdirAll(config.VersionRoot, 0755); err != nil {
		return nil, fmt.Errorf("创建版本目录失败：%w", err)
	}

	// 加载配置
	if err := m.loadConfig(); err != nil {
		if os.IsNotExist(err) {
			if err := m.saveConfig(); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// 加载版本索引
	if err := m.loadVersions(); err != nil {
		return nil, err
	}

	// 启动自动清理
	if config.Retention.AutoCleanup {
		go m.startAutoCleanup()
	}

	// 启动自动快照
	m.StartAutoSnapshot()

	return m, nil
}

// CreateVersion 创建新版本
func (m *Manager) CreateVersion(filePath, userID, description string, triggerType string) (*Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil, fmt.Errorf("版本控制已禁用")
	}

	// 检查文件是否存在
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在：%w", err)
	}

	// 检查文件大小限制
	if info.Size() > m.config.MaxFileSize {
		return nil, fmt.Errorf("文件超过大小限制")
	}

	// 检查是否在排除路径中
	for _, exclude := range m.config.ExcludePaths {
		if strings.HasPrefix(filePath, exclude) {
			return nil, fmt.Errorf("路径在排除列表中")
		}
	}

	// 计算文件校验和
	checksum, err := m.calculateChecksum(filePath)
	if err != nil {
		return nil, fmt.Errorf("计算校验和失败：%w", err)
	}

	// 检查是否有相同内容的版本（避免重复）
	for _, v := range m.versions[filePath] {
		if v.Checksum == checksum {
			return v, nil // 返回已存在的版本
		}
	}

	// 生成版本 ID
	versionID := m.generateVersionID(filePath)
	versionPath := filepath.Join(m.config.VersionRoot, versionID)

	// 复制文件到版本存储
	if err := m.copyFile(filePath, versionPath); err != nil {
		return nil, fmt.Errorf("复制文件失败：%w", err)
	}

	// 计算过期时间
	var expiresAt time.Time
	if m.config.Retention.MaxAge > 0 {
		expiresAt = time.Now().AddDate(0, 0, m.config.Retention.MaxAge)
	}

	// 创建版本记录
	version := &Version{
		ID:          versionID,
		FilePath:    filePath,
		VersionPath: versionPath,
		Checksum:    checksum,
		Size:        info.Size(),
		CreatedAt:   time.Now(),
		CreatedBy:   userID,
		Description: description,
		TriggerType: triggerType,
		ExpiresAt:   expiresAt,
	}

	// 添加到索引
	m.versions[filePath] = append(m.versions[filePath], version)
	m.allVersions[versionID] = version
	m.totalSize += info.Size()

	// 保存索引
	if err := m.saveVersions(); err != nil {
		// 回滚
		if err := os.Remove(versionPath); err != nil && !os.IsNotExist(err) {
			log.Printf("删除版本文件失败: %v", err)
		}
		delete(m.allVersions, versionID)
		// 从 versions 中移除
		vs := m.versions[filePath]
		for i, v := range vs {
			if v.ID == versionID {
				m.versions[filePath] = append(vs[:i], vs[i+1:]...)
				break
			}
		}
		m.totalSize -= info.Size()
		return nil, err
	}

	// 检查保留策略
	m.enforceRetentionPolicy(filePath)

	return version, nil
}

// GetVersions 获取文件的所有版本
func (m *Manager) GetVersions(filePath string) ([]*Version, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, exists := m.versions[filePath]
	if !exists {
		return []*Version{}, nil
	}

	// 返回副本，按创建时间倒序
	result := make([]*Version, len(versions))
	copy(result, versions)

	// 反转顺序（最新的在前）
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}

// GetVersion 获取指定版本
func (m *Manager) GetVersion(versionID string) (*Version, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	version, exists := m.allVersions[versionID]
	if !exists {
		return nil, fmt.Errorf("版本不存在：%s", versionID)
	}

	return version, nil
}

// RestoreVersion 恢复到指定版本
func (m *Manager) RestoreVersion(versionID, targetPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	version, exists := m.allVersions[versionID]
	if !exists {
		return fmt.Errorf("版本不存在：%s", versionID)
	}

	// 确定目标路径
	if targetPath == "" {
		targetPath = version.FilePath
	}

	// 确保目标目录存在
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败：%w", err)
	}

	// 如果目标文件存在，先创建当前版本快照
	if _, err := os.Stat(targetPath); err == nil {
		if m.config.Retention.SnapshotOnSave {
			m.createVersionInternal(targetPath, "system", "恢复前自动快照", "auto")
		}
	}

	// 复制版本文件到目标位置
	if err := m.copyFile(version.VersionPath, targetPath); err != nil {
		return fmt.Errorf("恢复文件失败：%w", err)
	}

	return nil
}

// DeleteVersion 删除指定版本
func (m *Manager) DeleteVersion(versionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	version, exists := m.allVersions[versionID]
	if !exists {
		return fmt.Errorf("版本不存在：%s", versionID)
	}

	// 删除版本文件
	if err := os.Remove(version.VersionPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除版本文件失败：%w", err)
	}

	// 从索引中移除
	delete(m.allVersions, versionID)
	m.totalSize -= version.Size

	// 从文件版本列表中移除
	versions := m.versions[version.FilePath]
	for i, v := range versions {
		if v.ID == versionID {
			m.versions[version.FilePath] = append(versions[:i], versions[i+1:]...)
			break
		}
	}

	// 如果文件没有版本了，删除键
	if len(m.versions[version.FilePath]) == 0 {
		delete(m.versions, version.FilePath)
	}

	return m.saveVersions()
}

// GetDiff 获取版本差异
func (m *Manager) GetDiff(versionID string) (*VersionDiff, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	version, exists := m.allVersions[versionID]
	if !exists {
		return nil, fmt.Errorf("版本不存在：%s", versionID)
	}

	// 获取文件扩展名
	ext := filepath.Ext(version.FilePath)

	diff := &VersionDiff{
		VersionID: versionID,
		FilePath:  version.FilePath,
		CreatedAt: version.CreatedAt,
		FileType:  ext,
	}

	// 对于文本文件，生成差异
	if m.isTextFile(ext) {
		// 读取当前文件
		currentContent, err := os.ReadFile(version.FilePath)
		if err != nil {
			currentContent = []byte{}
		}

		// 读取版本文件
		versionContent, err := os.ReadFile(version.VersionPath)
		if err != nil {
			versionContent = []byte{}
		}

		diff.DiffType = "text"
		diff.CurrentSize = int64(len(currentContent))
		diff.VersionSize = version.Size
		diff.ChangedLines = m.countChangedLines(string(currentContent), string(versionContent))
	} else {
		// 对于二进制文件，只返回元数据差异
		diff.DiffType = "binary"
		diff.VersionSize = version.Size

		// 获取当前文件大小
		if info, err := os.Stat(version.FilePath); err == nil {
			diff.CurrentSize = info.Size()
		}
	}

	return diff, nil
}

// VersionDiff 版本差异
type VersionDiff struct {
	VersionID    string    `json:"versionId"`
	FilePath     string    `json:"filePath"`
	DiffType     string    `json:"diffType"` // text, binary
	CreatedAt    time.Time `json:"createdAt"`
	FileType     string    `json:"fileType"`
	CurrentSize  int64     `json:"currentSize"`
	VersionSize  int64     `json:"versionSize"`
	ChangedLines int       `json:"changedLines,omitempty"`
	DiffContent  string    `json:"diffContent,omitempty"`
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalFiles := len(m.versions)
	totalVersions := len(m.allVersions)

	return map[string]interface{}{
		"enabled":       m.config.Enabled,
		"totalFiles":    totalFiles,
		"totalVersions": totalVersions,
		"totalSize":     m.totalSize,
		"maxSpace":      m.config.Retention.MaxSpace,
		"usagePercent":  float64(m.totalSize) / float64(m.config.Retention.MaxSpace) * 100,
		"maxVersions":   m.config.Retention.MaxVersions,
		"maxAge":        m.config.Retention.MaxAge,
	}
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	return m.saveConfig()
}

// Close 关闭管理器
func (m *Manager) Close() {
	close(m.stopChan)
}

// ========== 自动快照触发功能 ==========

// WatchFile 添加文件监控，当文件变更时自动创建快照
func (m *Manager) WatchFile(filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查文件是否存在
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("文件不存在：%w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("不支持监控目录")
	}

	m.watchers[filePath] = info.ModTime()
	return nil
}

// UnwatchFile 移除文件监控
func (m *Manager) UnwatchFile(filePath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.watchers, filePath)
}

// GetWatchedFiles 获取监控的文件列表
func (m *Manager) GetWatchedFiles() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	files := make([]string, 0, len(m.watchers))
	for f := range m.watchers {
		files = append(files, f)
	}
	return files
}

// StartAutoSnapshot 启动自动快照服务
// 根据配置的触发模式启动相应的自动快照功能
func (m *Manager) StartAutoSnapshot() {
	if !m.config.Snapshot.Enabled {
		return
	}

	switch m.config.Snapshot.TriggerMode {
	case "time":
		go m.startTimeBasedSnapshot()
	case "change":
		go m.startChangeBasedSnapshot()
	}
}

// startTimeBasedSnapshot 基于时间的自动快照
func (m *Manager) startTimeBasedSnapshot() {
	if m.config.Snapshot.Interval <= 0 {
		m.config.Snapshot.Interval = 60 // 默认 60 分钟
	}

	ticker := time.NewTicker(time.Duration(m.config.Snapshot.Interval) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.snapshotWatchedFiles("time")
		case <-m.stopChan:
			return
		}
	}
}

// startChangeBasedSnapshot 基于变更的自动快照
func (m *Manager) startChangeBasedSnapshot() {
	ticker := time.NewTicker(30 * time.Second) // 每 30 秒检查一次变更
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkAndSnapshotChanges()
		case <-m.stopChan:
			return
		}
	}
}

// snapshotWatchedFiles 为所有监控的文件创建快照
func (m *Manager) snapshotWatchedFiles(triggerType string) {
	m.mu.RLock()
	files := make([]string, 0, len(m.watchers))
	for f := range m.watchers {
		files = append(files, f)
	}
	m.mu.RUnlock()

	for _, filePath := range files {
		_, err := m.CreateVersion(filePath, "system", "自动快照", triggerType)
		if err != nil {
			// 记录错误但不中断
			continue
		}
	}
}

// checkAndSnapshotChanges 检查文件变更并创建快照
func (m *Manager) checkAndSnapshotChanges() {
	// 首先收集需要创建快照的文件
	m.mu.Lock()
	var toSnapshot []struct {
		path    string
		modTime time.Time
	}

	for filePath, lastModTime := range m.watchers {
		info, err := os.Stat(filePath)
		if err != nil {
			// 文件可能被删除，移除监控
			delete(m.watchers, filePath)
			continue
		}

		// 检查修改时间是否变化
		if info.ModTime().After(lastModTime) {
			// 检查变更大小是否达到阈值
			if m.config.Snapshot.MinChangeSize > 0 {
				// 获取最新版本的文件大小进行比较
				versions := m.versions[filePath]
				if len(versions) > 0 {
					latestVersion := versions[len(versions)-1]
					sizeDiff := info.Size() - latestVersion.Size
					if sizeDiff < 0 {
						sizeDiff = -sizeDiff
					}
					if sizeDiff < m.config.Snapshot.MinChangeSize {
						m.watchers[filePath] = info.ModTime()
						continue
					}
				}
			}

			toSnapshot = append(toSnapshot, struct {
				path    string
				modTime time.Time
			}{filePath, info.ModTime()})
		}
	}
	m.mu.Unlock()

	// 在锁外创建快照
	for _, item := range toSnapshot {
		version, _ := m.CreateVersion(item.path, "system", "变更触发快照", "change")
		if version != nil {
			m.mu.Lock()
			m.watchers[item.path] = item.modTime
			m.mu.Unlock()
		}
	}
}

// ========== 内部方法 ==========

func (m *Manager) createVersionInternal(filePath, userID, description, triggerType string) *Version {
	version, _ := m.CreateVersion(filePath, userID, description, triggerType)
	return version
}

func (m *Manager) calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (m *Manager) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func (m *Manager) generateVersionID(filePath string) string {
	timestamp := time.Now().UnixNano()
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", filePath, timestamp)))
	return fmt.Sprintf("v-%s-%d", hex.EncodeToString(hash[:8]), timestamp)
}

func (m *Manager) enforceRetentionPolicy(filePath string) {
	versions := m.versions[filePath]

	// 按版本数量限制
	if m.config.Retention.MaxVersions > 0 && len(versions) > m.config.Retention.MaxVersions {
		// 删除最旧的版本
		toDelete := len(versions) - m.config.Retention.MaxVersions
		for i := 0; i < toDelete; i++ {
			oldest := versions[i]
			if removeErr := os.Remove(oldest.VersionPath); removeErr != nil && !os.IsNotExist(removeErr) {
				log.Printf("删除版本文件 %s 失败: %v", oldest.VersionPath, removeErr)
			}
			m.totalSize -= oldest.Size
			delete(m.allVersions, oldest.ID)
		}
		m.versions[filePath] = versions[toDelete:]
	}

	// 按空间限制
	if m.config.Retention.MaxSpace > 0 && m.totalSize > m.config.Retention.MaxSpace {
		m.cleanupOldestVersions()
	}
}

func (m *Manager) cleanupOldestVersions() {
	// 收集所有版本并按时间排序
	allVersions := make([]*Version, 0, len(m.allVersions))
	for _, v := range m.allVersions {
		allVersions = append(allVersions, v)
	}

	// 按创建时间排序
	for i := 0; i < len(allVersions); i++ {
		for j := i + 1; j < len(allVersions); j++ {
			if allVersions[i].CreatedAt.After(allVersions[j].CreatedAt) {
				allVersions[i], allVersions[j] = allVersions[j], allVersions[i]
			}
		}
	}

	// 删除最旧的版本直到空间足够
	for _, v := range allVersions {
		if m.totalSize <= m.config.Retention.MaxSpace {
			break
		}
		if removeErr := os.Remove(v.VersionPath); removeErr != nil && !os.IsNotExist(removeErr) {
			log.Printf("删除版本文件 %s 失败: %v", v.VersionPath, removeErr)
		}
		m.totalSize -= v.Size
		delete(m.allVersions, v.ID)

		// 从文件版本列表中移除
		versions := m.versions[v.FilePath]
		for i, ver := range versions {
			if ver.ID == v.ID {
				m.versions[v.FilePath] = append(versions[:i], versions[i+1:]...)
				break
			}
		}
	}
}

func (m *Manager) startAutoCleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupExpiredVersions()
		case <-m.stopChan:
			return
		}
	}
}

func (m *Manager) cleanupExpiredVersions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var toDelete []string

	for id, v := range m.allVersions {
		if !v.ExpiresAt.IsZero() && now.After(v.ExpiresAt) {
			toDelete = append(toDelete, id)
		}
	}

	for _, id := range toDelete {
		v := m.allVersions[id]
		if err := os.Remove(v.VersionPath); err != nil && !os.IsNotExist(err) {
			log.Printf("删除版本文件 %s 失败: %v", v.VersionPath, err)
		}
		m.totalSize -= v.Size
		delete(m.allVersions, id)

		versions := m.versions[v.FilePath]
		for i, ver := range versions {
			if ver.ID == id {
				m.versions[v.FilePath] = append(versions[:i], versions[i+1:]...)
				break
			}
		}
	}

	if len(toDelete) > 0 {
		if err := m.saveVersions(); err != nil {
			log.Printf("保存版本索引失败: %v", err)
		}
	}
}

func (m *Manager) isTextFile(ext string) bool {
	textExts := map[string]bool{
		".txt": true, ".md": true, ".json": true, ".xml": true,
		".yaml": true, ".yml": true, ".html": true, ".css": true,
		".js": true, ".go": true, ".py": true, ".java": true,
		".c": true, ".cpp": true, ".h": true, ".sh": true,
		".bash": true, ".zsh": true, ".conf": true, ".cfg": true,
		".ini": true, ".log": true, ".csv": true, ".tsv": true,
	}
	return textExts[ext]
}

func (m *Manager) countChangedLines(current, version string) int {
	currentLines := splitLines(current)
	versionLines := splitLines(version)

	// 简单的差异行数计算
	changed := 0
	maxLen := len(currentLines)
	if len(versionLines) > maxLen {
		maxLen = len(versionLines)
	}

	for i := 0; i < maxLen; i++ {
		if i >= len(currentLines) || i >= len(versionLines) {
			changed++
		} else if currentLines[i] != versionLines[i] {
			changed++
		}
	}

	return changed
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, m.config)
}

func (m *Manager) saveConfig() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.configPath, data, 0644)
}

func (m *Manager) loadVersions() error {
	indexPath := filepath.Join(m.config.VersionRoot, "index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var versions []*Version
	if err := json.Unmarshal(data, &versions); err != nil {
		return err
	}

	m.totalSize = 0
	for _, v := range versions {
		// 验证版本文件是否存在
		if _, err := os.Stat(v.VersionPath); err == nil {
			m.versions[v.FilePath] = append(m.versions[v.FilePath], v)
			m.allVersions[v.ID] = v
			m.totalSize += v.Size
		}
	}

	return nil
}

func (m *Manager) saveVersions() error {
	indexPath := filepath.Join(m.config.VersionRoot, "index.json")

	var allVersions []*Version
	for _, v := range m.allVersions {
		allVersions = append(allVersions, v)
	}

	data, err := json.MarshalIndent(allVersions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(indexPath, data, 0644)
}
