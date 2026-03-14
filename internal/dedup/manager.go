// Package dedup 数据去重管理器
package dedup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager 去重管理器
type Manager struct {
	mu          sync.RWMutex
	config      *Config
	configPath  string
	chunks      map[string]*Chunk         // hash -> chunk
	fileRecords map[string]*FileRecord    // path -> record
	checksums   map[string][]*FileRecord  // checksum -> records
	userIndexes map[string]*UserFileIndex // user -> index
	duplicates  []*DuplicateGroup
	stats       DedupStats
	scanning    bool
	scanCancel  chan struct{}
	autoTask    *AutoDedupTask
	storagePath string // 存储根路径
}

// UserFileIndex 用户文件索引
type UserFileIndex struct {
	User       string            `json:"user"`
	Files      map[string]string `json:"files"` // path -> checksum
	TotalSize  int64             `json:"totalSize"`
	FileCount  int               `json:"fileCount"`
	SharedSize int64             `json:"sharedSize"` // 共享数据大小
	SavedSize  int64             `json:"savedSize"`  // 通过去重节省的空间
}

// ScanResult 扫描结果
type ScanResult struct {
	FilesScanned     int           `json:"filesScanned"`
	TotalSize        int64         `json:"totalSize"`
	DuplicateGroups  int           `json:"duplicateGroups"`
	DuplicatesFound  int           `json:"duplicatesFound"`
	SavingsPotential int64         `json:"savingsPotential"`
	CrossUserGroups  int           `json:"crossUserGroups"`
	CrossUserSavings int64         `json:"crossUserSavings"`
	Duration         time.Duration `json:"duration"`
	Errors           []ScanError   `json:"errors"`
}

// AutoDedupTask 自动去重任务
type AutoDedupTask struct {
	ID       string      `json:"id"`
	Enabled  bool        `json:"enabled"`
	Schedule string      `json:"schedule"`
	LastRun  time.Time   `json:"lastRun"`
	NextRun  time.Time   `json:"nextRun"`
	Status   string      `json:"status"` // pending, running, completed, failed
	Result   *ScanResult `json:"result,omitempty"`
	Error    string      `json:"error,omitempty"`
}

// NewManager 创建去重管理器
func NewManager(configPath string, config *Config) (*Manager, error) {
	return NewManagerWithStorage(configPath, config, "")
}

// NewManagerWithStorage 创建去重管理器（指定存储路径）
func NewManagerWithStorage(configPath string, config *Config, storagePath string) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	m := &Manager{
		config:      config,
		configPath:  configPath,
		storagePath: storagePath,
		chunks:      make(map[string]*Chunk),
		fileRecords: make(map[string]*FileRecord),
		checksums:   make(map[string][]*FileRecord),
		userIndexes: make(map[string]*UserFileIndex),
		duplicates:  make([]*DuplicateGroup, 0),
		scanCancel:  make(chan struct{}),
		autoTask: &AutoDedupTask{
			ID:       "auto-dedup",
			Enabled:  config.AutoDedup,
			Schedule: config.AutoDedupCron,
			Status:   "pending",
		},
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

	// 加载索引
	if err := m.loadIndex(); err != nil {
		// 索引不存在时忽略
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	// 初始化块存储目录
	if m.config.ChunkStore != nil && m.config.ChunkStore.Enabled {
		if err := os.MkdirAll(m.config.ChunkStore.BasePath, 0755); err != nil {
			return nil, fmt.Errorf("创建块存储目录失败：%w", err)
		}
	}

	return m, nil
}

// Scan 扫描重复文件
func (m *Manager) Scan(paths []string) (*ScanResult, error) {
	return m.ScanForUser(paths, "")
}

// ScanForUser 扫描指定用户的重复文件
func (m *Manager) ScanForUser(paths []string, user string) (*ScanResult, error) {
	m.mu.Lock()
	if m.scanning {
		m.mu.Unlock()
		return nil, fmt.Errorf("扫描正在进行中")
	}
	m.scanning = true
	m.scanCancel = make(chan struct{})
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.scanning = false
		m.mu.Unlock()
	}()

	startTime := time.Now()
	result := &ScanResult{
		Errors: make([]ScanError, 0),
	}

	// 重置状态
	m.mu.Lock()
	m.checksums = make(map[string][]*FileRecord)
	m.duplicates = make([]*DuplicateGroup, 0)
	if user == "" {
		m.stats = DedupStats{}
		m.userIndexes = make(map[string]*UserFileIndex)
	}
	m.mu.Unlock()

	// 使用传入的路径或配置中的路径
	scanPaths := paths
	if len(scanPaths) == 0 {
		scanPaths = m.config.ScanPaths
	}

	// 扫描文件
	for _, rootPath := range scanPaths {
		select {
		case <-m.scanCancel:
			return result, fmt.Errorf("扫描已取消")
		default:
			m.scanPath(rootPath, result, user)
		}
	}

	// 分析重复文件
	m.analyzeDuplicates()

	// 计算统计信息
	m.mu.RLock()
	result.DuplicateGroups = len(m.duplicates)
	for _, group := range m.duplicates {
		result.DuplicatesFound += len(group.Files) - 1
		result.SavingsPotential += group.Savings
		// 统计跨用户重复
		if len(group.Users) > 1 {
			result.CrossUserGroups++
			result.CrossUserSavings += group.Savings
		}
	}
	result.TotalSize = m.stats.TotalSize
	result.FilesScanned = int(m.stats.TotalFiles)
	result.Duration = time.Since(startTime)
	m.stats.LastScanTime = startTime
	m.mu.RUnlock()

	// 保存索引
	m.saveIndex()

	return result, nil
}

// scanPath 扫描单个路径
func (m *Manager) scanPath(rootPath string, result *ScanResult, currentUser string) {
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, ScanError{
				Path:    path,
				Message: err.Error(),
			})
			return nil
		}

		// 跳过目录
		if info.IsDir() {
			// 检查是否在排除路径中
			for _, exclude := range m.config.ExcludePaths {
				if strings.HasPrefix(path, exclude) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// 检查文件大小
		if info.Size() < m.config.MinFileSize {
			return nil
		}

		// 检查排除模式
		for _, pattern := range m.config.ExcludePatterns {
			matched, _ := filepath.Match(pattern, filepath.Base(path))
			if matched {
				return nil
			}
		}

		// 计算文件校验和
		checksum, err := m.calculateFileChecksum(path)
		if err != nil {
			result.Errors = append(result.Errors, ScanError{
				Path:    path,
				Message: err.Error(),
			})
			return nil
		}

		// 提取用户信息（从路径中推断）
		user := currentUser
		if user == "" {
			user = m.extractUserFromPath(path)
		}

		m.mu.Lock()
		// 创建文件记录
		record := &FileRecord{
			Path:       path,
			Size:       info.Size(),
			Checksum:   checksum,
			User:       user,
			CreatedAt:  time.Now(),
			ModifiedAt: info.ModTime(),
		}

		// 添加到索引
		m.fileRecords[path] = record
		m.checksums[checksum] = append(m.checksums[checksum], record)

		// 更新用户索引
		if user != "" {
			if _, exists := m.userIndexes[user]; !exists {
				m.userIndexes[user] = &UserFileIndex{
					User:  user,
					Files: make(map[string]string),
				}
			}
			m.userIndexes[user].Files[path] = checksum
			m.userIndexes[user].TotalSize += info.Size()
			m.userIndexes[user].FileCount++
		}

		// 更新统计
		m.stats.TotalFiles++
		m.stats.TotalSize += info.Size()
		if user != "" {
			m.stats.UserCount = len(m.userIndexes)
		}
		m.mu.Unlock()

		return nil
	})

	if err != nil {
		result.Errors = append(result.Errors, ScanError{
			Path:    rootPath,
			Message: err.Error(),
		})
	}
}

// extractUserFromPath 从路径提取用户信息
func (m *Manager) extractUserFromPath(path string) string {
	// 常见的用户目录模式
	patterns := []string{
		"/home/",
		"/Users/",
		"/data/users/",
	}

	for _, pattern := range patterns {
		idx := strings.Index(path, pattern)
		if idx != -1 {
			// 找到模式后的路径部分
			afterPattern := path[idx+len(pattern):]
			parts := strings.SplitN(afterPattern, "/", 2)
			if len(parts) > 0 && parts[0] != "" {
				return parts[0]
			}
		}
	}

	return ""
}

// analyzeDuplicates 分析重复文件
func (m *Manager) analyzeDuplicates() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.duplicates = make([]*DuplicateGroup, 0)

	for checksum, records := range m.checksums {
		if len(records) > 1 {
			// 发现重复文件
			files := make([]string, len(records))
			userFiles := make(map[string][]string)
			users := make(map[string]bool)

			for i, r := range records {
				files[i] = r.Path
				if r.User != "" {
					users[r.User] = true
					userFiles[r.User] = append(userFiles[r.User], r.Path)
				}
			}

			// 计算可节省空间：保留一个，其余都是重复
			size := records[0].Size
			savings := size * int64(len(records)-1)

			group := &DuplicateGroup{
				Checksum:  checksum,
				Size:      size,
				Files:     files,
				Savings:   savings,
				UserFiles: userFiles,
			}

			// 添加用户列表
			for u := range users {
				group.Users = append(group.Users, u)
			}

			m.duplicates = append(m.duplicates, group)

			// 更新统计
			m.stats.DuplicateFiles += int64(len(records) - 1)
			m.stats.DuplicateSize += size * int64(len(records)-1)
			m.stats.SavingsPotential += savings

			// 跨用户统计
			if len(users) > 1 {
				m.stats.CrossUserSavings += savings
			}
		}
	}
}

// GetDuplicates 获取重复文件列表
func (m *Manager) GetDuplicates() ([]*DuplicateGroup, error) {
	return m.GetDuplicatesForUser("")
}

// GetDuplicatesForUser 获取指定用户的重复文件列表
func (m *Manager) GetDuplicatesForUser(user string) ([]*DuplicateGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if user == "" {
		result := make([]*DuplicateGroup, len(m.duplicates))
		copy(result, m.duplicates)
		return result, nil
	}

	// 过滤特定用户的重复文件
	var result []*DuplicateGroup
	for _, group := range m.duplicates {
		if files, exists := group.UserFiles[user]; exists && len(files) > 1 {
			// 创建用户特定的重复组
			userGroup := &DuplicateGroup{
				Checksum:  group.Checksum,
				Size:      group.Size,
				Files:     files,
				Savings:   group.Size * int64(len(files)-1),
				UserFiles: map[string][]string{user: files},
			}
			result = append(result, userGroup)
		}
	}

	return result, nil
}

// GetCrossUserDuplicates 获取跨用户重复文件
func (m *Manager) GetCrossUserDuplicates() ([]*DuplicateGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*DuplicateGroup
	for _, group := range m.duplicates {
		if len(group.Users) > 1 {
			result = append(result, group)
		}
	}

	return result, nil
}

// Deduplicate 执行去重操作
func (m *Manager) Deduplicate(checksum string, keepPath string, policy DedupPolicy) error {
	return m.DeduplicateForUser(checksum, keepPath, policy, "")
}

// DeduplicateForUser 为指定用户执行去重操作
func (m *Manager) DeduplicateForUser(checksum string, keepPath string, policy DedupPolicy, currentUser string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	records, exists := m.checksums[checksum]
	if !exists || len(records) <= 1 {
		return fmt.Errorf("没有找到重复文件")
	}

	// 找到要保留的文件
	var keepRecord *FileRecord
	for _, r := range records {
		if r.Path == keepPath {
			keepRecord = r
			break
		}
	}

	if keepRecord == nil {
		return fmt.Errorf("保留路径不在重复文件列表中")
	}

	// 检查跨用户权限
	if currentUser != "" && !policy.CrossUser {
		// 只处理当前用户的文件
		if keepRecord.User != currentUser {
			return fmt.Errorf("无权操作其他用户的文件")
		}
	}

	var savedSize int64

	// 对其他文件执行去重
	for _, record := range records {
		if record.Path == keepPath {
			continue
		}

		// 检查用户权限
		if currentUser != "" && !policy.CrossUser && record.User != currentUser {
			continue
		}

		switch policy.Action {
		case ActionSoftlink:
			// 删除文件并创建软链接
			if err := os.Remove(record.Path); err != nil {
				return fmt.Errorf("删除文件失败：%w", err)
			}
			if err := os.Symlink(keepPath, record.Path); err != nil {
				return fmt.Errorf("创建软链接失败：%w", err)
			}
			savedSize += record.Size

		case ActionHardlink:
			// 删除文件并创建硬链接
			if err := os.Remove(record.Path); err != nil {
				return fmt.Errorf("删除文件失败：%w", err)
			}
			if err := os.Link(keepPath, record.Path); err != nil {
				return fmt.Errorf("创建硬链接失败：%w", err)
			}
			savedSize += record.Size

		case ActionReport:
			// 只报告，不执行操作
			savedSize += record.Size

		default:
			return fmt.Errorf("不支持的去重操作：%s", policy.Action)
		}
	}

	// 更新统计
	m.stats.SavingsActual += savedSize
	m.stats.LastDedupTime = time.Now()

	return nil
}

// DeduplicateAll 批量去重
func (m *Manager) DeduplicateAll(policy DedupPolicy, dryRun bool) (*DedupResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := &DedupResult{
		Groups:    make([]DedupGroupResult, 0),
		StartTime: time.Now(),
	}

	for _, group := range m.duplicates {
		if len(group.Files) < 2 {
			continue
		}

		groupResult := DedupGroupResult{
			Checksum:  group.Checksum,
			Size:      group.Size,
			FileCount: len(group.Files),
			KeepPath:  group.Files[0], // 默认保留第一个文件
			Savings:   group.Size * int64(len(group.Files)-1),
		}

		// 计算潜在节省空间（无论是 dry run 还是实际执行）
		result.TotalSaved += groupResult.Savings

		if !dryRun {
			// 执行去重
			for i := 1; i < len(group.Files); i++ {
				switch policy.Action {
				case ActionSoftlink:
					if err := os.Remove(group.Files[i]); err == nil {
						if err := os.Symlink(group.Files[0], group.Files[i]); err == nil {
							groupResult.Processed++
						}
					}
				case ActionHardlink:
					if err := os.Remove(group.Files[i]); err == nil {
						if err := os.Link(group.Files[0], group.Files[i]); err == nil {
							groupResult.Processed++
						}
					}
				}
			}
		}

		result.Groups = append(result.Groups, groupResult)
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if !dryRun {
		m.stats.SavingsActual = result.TotalSaved
		m.stats.LastDedupTime = time.Now()
	}

	return result, nil
}

// DedupResult 批量去重结果
type DedupResult struct {
	Groups     []DedupGroupResult `json:"groups"`
	TotalSaved int64              `json:"totalSaved"`
	StartTime  time.Time          `json:"startTime"`
	EndTime    time.Time          `json:"endTime"`
	Duration   time.Duration      `json:"duration"`
}

// DedupGroupResult 单组去重结果
type DedupGroupResult struct {
	Checksum  string `json:"checksum"`
	Size      int64  `json:"size"`
	FileCount int    `json:"fileCount"`
	KeepPath  string `json:"keepPath"`
	Savings   int64  `json:"savings"`
	Processed int    `json:"processed"`
}

// GetReport 获取去重报告
func (m *Manager) GetReport() (*DedupReport, error) {
	return m.GetReportForUser("")
}

// GetReportForUser 获取指定用户的去重报告
func (m *Manager) GetReportForUser(user string) (*DedupReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &DedupReport{
		GeneratedAt:     time.Now(),
		Stats: DedupStatsSnapshot{
			TotalFiles:       m.stats.TotalFiles,
			TotalSize:        m.stats.TotalSize,
			DuplicateFiles:   m.stats.DuplicateFiles,
			DuplicateSize:    m.stats.DuplicateSize,
			SavingsPotential: m.stats.SavingsPotential,
			SavingsActual:    m.stats.SavingsActual,
			ChunksStored:     m.stats.ChunksStored,
			ChunkDataSize:    m.stats.ChunkDataSize,
			SharedChunks:     m.stats.SharedChunks,
			SharedDataSize:   m.stats.SharedDataSize,
			CrossUserSavings: m.stats.CrossUserSavings,
		},
		DuplicateGroups: make([]DuplicateGroupSummary, 0),
		UserReports:     make(map[string]*UserDedupReport),
	}

	// 按大小排序重复组
	sortedDuplicates := make([]*DuplicateGroup, len(m.duplicates))
	copy(sortedDuplicates, m.duplicates)

	sort.Slice(sortedDuplicates, func(i, j int) bool {
		return sortedDuplicates[i].Savings > sortedDuplicates[j].Savings
	})

	// 生成摘要
	for _, group := range sortedDuplicates {
		// 用户过滤
		if user != "" {
			if _, exists := group.UserFiles[user]; !exists {
				continue
			}
		}

		checksumDisplay := group.Checksum
		if len(checksumDisplay) > 16 {
			checksumDisplay = checksumDisplay[:16] + "..."
		}

		summary := DuplicateGroupSummary{
			Checksum:     checksumDisplay,
			Size:         group.Size,
			FileCount:    len(group.Files),
			Savings:      group.Savings,
			ExampleFiles: group.Files,
			Users:        group.Users,
		}
		if len(summary.ExampleFiles) > 3 {
			summary.ExampleFiles = group.Files[:3]
		}
		report.DuplicateGroups = append(report.DuplicateGroups, summary)
	}

	// 生成用户报告
	for u, index := range m.userIndexes {
		userReport := &UserDedupReport{
			User:       u,
			FileCount:  index.FileCount,
			TotalSize:  index.TotalSize,
			SharedSize: index.SharedSize,
			SavedSize:  index.SavedSize,
		}
		report.UserReports[u] = userReport
	}

	// 计算建议
	report.Recommendations = m.generateRecommendations()

	return report, nil
}

// generateRecommendations 生成去重建议
func (m *Manager) generateRecommendations() []DedupRecommendation {
	var recommendations []DedupRecommendation

	if m.stats.SavingsPotential > 0 {
		recommendations = append(recommendations, DedupRecommendation{
			Type:        "savings",
			Priority:    "high",
			Title:       "发现重复文件",
			Description: fmt.Sprintf("检测到 %d 个重复文件，可节省 %.2f GB 空间", m.stats.DuplicateFiles, float64(m.stats.SavingsPotential)/1024/1024/1024),
			Action:      "deduplicate",
		})
	}

	if m.stats.CrossUserSavings > 0 && m.config.CrossUser {
		recommendations = append(recommendations, DedupRecommendation{
			Type:        "cross_user",
			Priority:    "medium",
			Title:       "跨用户重复数据",
			Description: fmt.Sprintf("发现跨用户重复数据，可节省 %.2f GB 空间", float64(m.stats.CrossUserSavings)/1024/1024/1024),
			Action:      "cross_user_dedup",
		})
	}

	if !m.config.AutoDedup && m.stats.SavingsPotential > 1024*1024*1024 { // > 1GB
		recommendations = append(recommendations, DedupRecommendation{
			Type:        "auto",
			Priority:    "low",
			Title:       "启用自动去重",
			Description: "建议启用自动去重功能，定期清理重复文件",
			Action:      "enable_auto_dedup",
		})
	}

	return recommendations
}

// DedupReport 去重报告
type DedupReport struct {
	GeneratedAt     time.Time                   `json:"generatedAt"`
	Stats           DedupStatsSnapshot          `json:"stats"`
	DuplicateGroups []DuplicateGroupSummary     `json:"duplicateGroups"`
	UserReports     map[string]*UserDedupReport `json:"userReports,omitempty"`
	Recommendations []DedupRecommendation       `json:"recommendations,omitempty"`
}

// UserDedupReport 用户去重报告
type UserDedupReport struct {
	User       string `json:"user"`
	FileCount  int    `json:"fileCount"`
	TotalSize  int64  `json:"totalSize"`
	SharedSize int64  `json:"sharedSize"`
	SavedSize  int64  `json:"savedSize"`
}

// DuplicateGroupSummary 重复组摘要
type DuplicateGroupSummary struct {
	Checksum     string   `json:"checksum"`
	Size         int64    `json:"size"`
	FileCount    int      `json:"fileCount"`
	Savings      int64    `json:"savings"`
	ExampleFiles []string `json:"exampleFiles"`
	Users        []string `json:"users,omitempty"`
}

// DedupRecommendation 去重建议
type DedupRecommendation struct {
	Type        string `json:"type"`
	Priority    string `json:"priority"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action"`
}

// GetStats 获取统计信息快照（不含锁）
func (m *Manager) GetStats() DedupStatsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return DedupStatsSnapshot{
		TotalFiles:       m.stats.TotalFiles,
		TotalSize:        m.stats.TotalSize,
		DuplicateFiles:   m.stats.DuplicateFiles,
		DuplicateSize:    m.stats.DuplicateSize,
		SavingsPotential: m.stats.SavingsPotential,
		SavingsActual:    m.stats.SavingsActual,
		ChunksStored:     m.stats.ChunksStored,
		ChunkDataSize:    m.stats.ChunkDataSize,
		SharedChunks:     m.stats.SharedChunks,
		SharedDataSize:   m.stats.SharedDataSize,
		CrossUserSavings: m.stats.CrossUserSavings,
	}
}

// GetUserStats 获取用户统计信息
func (m *Manager) GetUserStats(user string) (*UserFileIndex, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	index, exists := m.userIndexes[user]
	if !exists {
		return nil, fmt.Errorf("用户不存在：%s", user)
	}

	return index, nil
}

// CancelScan 取消扫描
func (m *Manager) CancelScan() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.scanning {
		close(m.scanCancel)
	}
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	return m.saveConfig()
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// ========== 自动去重任务 ==========

// GetAutoTask 获取自动去重任务
func (m *Manager) GetAutoTask() *AutoDedupTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.autoTask
}

// EnableAutoDedup 启用自动去重
func (m *Manager) EnableAutoDedup(enabled bool, schedule string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.AutoDedup = enabled
	if schedule != "" {
		m.config.AutoDedupCron = schedule
		m.autoTask.Schedule = schedule
	}
	m.autoTask.Enabled = enabled

	return m.saveConfig()
}

// RunAutoDedup 执行自动去重任务
func (m *Manager) RunAutoDedup() (*DedupResult, error) {
	m.mu.Lock()
	if m.autoTask.Status == "running" {
		m.mu.Unlock()
		return nil, fmt.Errorf("自动去重任务正在运行")
	}
	m.autoTask.Status = "running"
	m.autoTask.LastRun = time.Now()
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.autoTask.Status = "completed"
		m.mu.Unlock()
	}()

	// 执行扫描
	result, err := m.Scan(nil)
	if err != nil {
		m.mu.Lock()
		m.autoTask.Status = "failed"
		m.autoTask.Error = err.Error()
		m.mu.Unlock()
		return nil, err
	}

	// 执行去重
	policy := DedupPolicy{
		Mode:          m.config.DedupMode,
		Action:        m.config.DedupAction,
		MinMatchCount: 2,
		PreserveAttrs: true,
		CrossUser:     m.config.CrossUser,
	}

	dedupResult, err := m.DeduplicateAll(policy, false)
	if err != nil {
		m.mu.Lock()
		m.autoTask.Status = "failed"
		m.autoTask.Error = err.Error()
		m.mu.Unlock()
		return nil, err
	}

	m.mu.Lock()
	m.autoTask.Result = result
	m.mu.Unlock()

	return dedupResult, nil
}

// ========== 块级去重 ==========

// ChunkFile 将文件分块
func (m *Manager) ChunkFile(filePath string) ([]*Chunk, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var chunks []*Chunk
	buf := make([]byte, m.config.ChunkSize)
	offset := int64(0)

	for {
		n, err := file.Read(buf)
		if n == 0 {
			break
		}

		data := buf[:n]
		hash := sha256.Sum256(data)
		hashStr := hex.EncodeToString(hash[:])

		chunk := &Chunk{
			Hash:     hashStr,
			Size:     int64(n),
			Offset:   offset,
			RefCount: 1,
		}

		chunks = append(chunks, chunk)
		offset += int64(n)

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	return chunks, nil
}

// CreateChunk 创建数据块
func (m *Manager) CreateChunk(data []byte) (*Chunk, error) {
	return m.CreateChunkForUser(data, "")
}

// CreateChunkForUser 为用户创建数据块
func (m *Manager) CreateChunkForUser(data []byte, user string) (*Chunk, error) {
	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查块是否已存在
	if chunk, exists := m.chunks[hashStr]; exists {
		chunk.RefCount++
		if user != "" {
			if chunk.Users == nil {
				chunk.Users = make(map[string]bool)
			}
			chunk.Users[user] = true
		}
		return chunk, nil
	}

	// 创建新块
	chunk := &Chunk{
		Hash:     hashStr,
		Size:     int64(len(data)),
		RefCount: 1,
		Users:    make(map[string]bool),
	}

	if user != "" {
		chunk.Users[user] = true
	}

	// 存储块数据
	if m.config.ChunkStore != nil && m.config.ChunkStore.Enabled {
		storePath := filepath.Join(m.config.ChunkStore.BasePath, hashStr[:2], hashStr[2:4])
		if err := os.MkdirAll(storePath, 0755); err != nil {
			return nil, fmt.Errorf("创建块存储目录失败：%w", err)
		}

		chunkFile := filepath.Join(storePath, hashStr)
		if err := os.WriteFile(chunkFile, data, 0644); err != nil {
			return nil, fmt.Errorf("写入块数据失败：%w", err)
		}
		chunk.StorePath = chunkFile
	}

	m.chunks[hashStr] = chunk
	m.stats.ChunksStored++
	m.stats.ChunkDataSize += int64(len(data))

	return chunk, nil
}

// GetChunk 获取数据块
func (m *Manager) GetChunk(hash string) (*Chunk, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chunk, exists := m.chunks[hash]
	if !exists {
		return nil, fmt.Errorf("块不存在：%s", hash)
	}

	return chunk, nil
}

// GetChunkData 获取块数据
func (m *Manager) GetChunkData(hash string) ([]byte, error) {
	m.mu.RLock()
	chunk, exists := m.chunks[hash]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("块不存在：%s", hash)
	}

	if chunk.StorePath == "" {
		return nil, fmt.Errorf("块数据未存储")
	}

	return os.ReadFile(chunk.StorePath)
}

// DeleteChunk 删除数据块
func (m *Manager) DeleteChunk(hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	chunk, exists := m.chunks[hash]
	if !exists {
		return fmt.Errorf("块不存在：%s", hash)
	}

	chunk.RefCount--
	if chunk.RefCount < 0 {
		chunk.RefCount = 0
	}

	// 只有当引用计数为0且明确需要清理时才删除
	// 这里保持块存在但引用计数为0，以便后续可能的重新引用

	return nil
}

// ForceDeleteChunk 强制删除数据块（无论引用计数）
func (m *Manager) ForceDeleteChunk(hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	chunk, exists := m.chunks[hash]
	if !exists {
		return fmt.Errorf("块不存在：%s", hash)
	}

	// 删除存储的块数据
	if chunk.StorePath != "" {
		os.Remove(chunk.StorePath)
	}
	delete(m.chunks, hash)
	m.stats.ChunksStored--
	m.stats.ChunkDataSize -= chunk.Size

	return nil
}

// GetSharedChunks 获取共享块列表
func (m *Manager) GetSharedChunks() ([]*Chunk, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var shared []*Chunk
	for _, chunk := range m.chunks {
		if len(chunk.Users) > 1 {
			shared = append(shared, chunk)
		}
	}

	return shared, nil
}

// ========== 内部方法 ==========

func (m *Manager) calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
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

func (m *Manager) loadIndex() error {
	indexPath := m.configPath + ".index"
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}

	var index struct {
		FileRecords map[string]*FileRecord    `json:"fileRecords"`
		Checksums   map[string][]*FileRecord  `json:"checksums"`
		Stats       DedupStats                `json:"stats"`
		UserIndexes map[string]*UserFileIndex `json:"userIndexes"`
		Chunks      map[string]*Chunk         `json:"chunks"`
	}

	if err := json.Unmarshal(data, &index); err != nil {
		return err
	}

	m.fileRecords = index.FileRecords
	m.checksums = index.Checksums
	// 手动复制统计字段，避免复制锁
	m.stats.TotalFiles = index.Stats.TotalFiles
	m.stats.TotalSize = index.Stats.TotalSize
	m.stats.DuplicateFiles = index.Stats.DuplicateFiles
	m.stats.DuplicateSize = index.Stats.DuplicateSize
	m.stats.SavingsPotential = index.Stats.SavingsPotential
	m.stats.SavingsActual = index.Stats.SavingsActual
	m.stats.ChunksStored = index.Stats.ChunksStored
	m.stats.ChunkDataSize = index.Stats.ChunkDataSize
	m.stats.SharedChunks = index.Stats.SharedChunks
	m.stats.SharedDataSize = index.Stats.SharedDataSize
	m.stats.CrossUserSavings = index.Stats.CrossUserSavings
	m.userIndexes = index.UserIndexes
	m.chunks = index.Chunks

	if m.fileRecords == nil {
		m.fileRecords = make(map[string]*FileRecord)
	}
	if m.checksums == nil {
		m.checksums = make(map[string][]*FileRecord)
	}
	if m.userIndexes == nil {
		m.userIndexes = make(map[string]*UserFileIndex)
	}
	if m.chunks == nil {
		m.chunks = make(map[string]*Chunk)
	}

	return nil
}

func (m *Manager) saveIndex() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	indexPath := m.configPath + ".index"

	// 创建统计快照以避免复制锁
	statsSnapshot := DedupStatsSnapshot{
		TotalFiles:       m.stats.TotalFiles,
		TotalSize:        m.stats.TotalSize,
		DuplicateFiles:   m.stats.DuplicateFiles,
		DuplicateSize:    m.stats.DuplicateSize,
		SavingsPotential: m.stats.SavingsPotential,
		SavingsActual:    m.stats.SavingsActual,
		ChunksStored:     m.stats.ChunksStored,
		ChunkDataSize:    m.stats.ChunkDataSize,
		SharedChunks:     m.stats.SharedChunks,
		SharedDataSize:   m.stats.SharedDataSize,
		CrossUserSavings: m.stats.CrossUserSavings,
	}

	data, err := json.MarshalIndent(struct {
		FileRecords map[string]*FileRecord    `json:"fileRecords"`
		Checksums   map[string][]*FileRecord  `json:"checksums"`
		Stats       DedupStatsSnapshot        `json:"stats"`
		UserIndexes map[string]*UserFileIndex `json:"userIndexes"`
		Chunks      map[string]*Chunk         `json:"chunks"`
	}{
		FileRecords: m.fileRecords,
		Checksums:   m.checksums,
		Stats:       statsSnapshot,
		UserIndexes: m.userIndexes,
		Chunks:      m.chunks,
	}, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(indexPath, data, 0644)
}
