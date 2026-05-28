// Package filededup 提供文件去重引擎，支持智能文件去重和空间回收。
package filededup

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// HashAlgorithm 哈希算法类型
type HashAlgorithm string

const (
	// HashMD5 使用 MD5 哈希算法
	HashMD5 HashAlgorithm = "md5"
	// HashSHA256 使用 SHA256 哈希算法
	HashSHA256 HashAlgorithm = "sha256"
)

// DedupStrategy 去重策略类型
type DedupStrategy string

const (
	// StrategyKeepNewest 保留最新文件
	StrategyKeepNewest DedupStrategy = "keep_newest"
	// StrategyKeepOldest 保留最旧文件
	StrategyKeepOldest DedupStrategy = "keep_oldest"
	// StrategyKeepShortestPath 保留最短路径的文件
	StrategyKeepShortestPath DedupStrategy = "keep_shortest_path"
	// StrategyKeepLongestPath 保留最长路径的文件
	StrategyKeepLongestPath DedupStrategy = "keep_longest_path"
)

// ScanMode 扫描模式
type ScanMode string

const (
	// ScanModeFull 全量扫描
	ScanModeFull ScanMode = "full"
	// ScanModeIncremental 增量扫描（基于上次扫描时间）
	ScanModeIncremental ScanMode = "incremental"
)

// FileStatus 文件状态
type FileStatus string

const (
	// StatusActive 活跃状态
	StatusActive FileStatus = "active"
	// StatusSoftDeleted 软删除状态
	StatusSoftDeleted FileStatus = "soft_deleted"
	// StatusConfirmedDeleted 已确认删除
	StatusConfirmedDeleted FileStatus = "confirmed_deleted"
)

// ErrNoDuplicates 没有找到重复文件
var ErrNoDuplicates = errors.New("没有找到重复文件")

// ErrFileNotFound 文件未找到
var ErrFileNotFound = errors.New("文件未找到")

// ErrInvalidStrategy 无效的去重策略
var ErrInvalidStrategy = errors.New("无效的去重策略")

// ErrInvalidHashAlgorithm 无效的哈希算法
var ErrInvalidHashAlgorithm = errors.New("无效的哈希算法")

// ErrTaskRunning 已有任务正在运行
var ErrTaskRunning = errors.New("已有任务正在运行")

// ErrNoSoftDeleted 没有软删除的文件可确认
var ErrNoSoftDeleted = errors.New("没有软删除的文件可确认")

// ExcludeRule 排除规则
type ExcludeRule struct {
	// Path 路径模式（支持通配符 * 和 ?）
	Path string `json:"path"`
	// MinSize 最小文件大小（字节），0 表示不限制
	MinSize int64 `json:"min_size"`
	// MaxSize 最大文件大小（字节），0 表示不限制
	MaxSize int64 `json:"max_size"`
	// Extensions 排除的文件扩展名列表（如 [".tmp", ".log"]）
	Extensions []string `json:"extensions"`
	// Description 规则描述
	Description string `json:"description"`
}

// FileInfo 文件信息
type FileInfo struct {
	// Path 文件绝对路径
	Path string `json:"path"`
	// Size 文件大小（字节）
	Size int64 `json:"size"`
	// Hash 文件哈希值
	Hash string `json:"hash"`
	// ModTime 文件修改时间
	ModTime time.Time `json:"mod_time"`
	// Status 文件状态
	Status FileStatus `json:"status"`
	// ScanTime 扫描时间
	ScanTime time.Time `json:"scan_time"`
}

// DuplicateGroup 重复文件组
type DuplicateGroup struct {
	// GroupID 组唯一标识
	GroupID string `json:"group_id"`
	// Hash 共同的哈希值
	Hash string `json:"hash"`
	// Size 文件大小
	Size int64 `json:"size"`
	// Files 重复文件列表
	Files []*FileInfo `json:"files"`
	// WastedSpace 浪费的空间（除保留文件外）
	WastedSpace int64 `json:"wasted_space"`
}

// DedupTask 去重任务
type DedupTask struct {
	// TaskID 任务唯一标识
	TaskID string `json:"task_id"`
	// ScanPaths 扫描路径列表
	ScanPaths []string `json:"scan_paths"`
	// Mode 扫描模式
	Mode ScanMode `json:"mode"`
	// Strategy 去重策略
	Strategy DedupStrategy `json:"strategy"`
	// Algorithm 哈希算法
	Algorithm HashAlgorithm `json:"algorithm"`
	// Status 任务状态
	Status string `json:"status"`
	// StartTime 开始时间
	StartTime time.Time `json:"start_time"`
	// EndTime 结束时间
	EndTime time.Time `json:"end_time"`
	// Progress 扫描进度（0-100）
	Progress float64 `json:"progress"`
	// Error 错误信息
	Error string `json:"error,omitempty"`
}

// DedupReport 去重报告
type DedupReport struct {
	// TaskID 关联任务ID
	TaskID string `json:"task_id"`
	// TotalFiles 扫描的总文件数
	TotalFiles int64 `json:"total_files"`
	// DuplicateFiles 重复文件数
	DuplicateFiles int64 `json:"duplicate_files"`
	// DuplicateGroups 重复文件组数
	DuplicateGroups int64 `json:"duplicate_groups"`
	// TotalSize 扫描文件总大小
	TotalSize int64 `json:"total_size"`
	// WastedSpace 浪费的总空间
	WastedSpace int64 `json:"wasted_space"`
	// RecoveredSpace 已回收的空间
	RecoveredSpace int64 `json:"recovered_space"`
	// SoftDeletedCount 软删除文件数
	SoftDeletedCount int64 `json:"soft_deleted_count"`
	// ConfirmedDeletedCount 已确认删除文件数
	ConfirmedDeletedCount int64 `json:"confirmed_deleted_count"`
	// StartTime 开始时间
	StartTime time.Time `json:"start_time"`
	// EndTime 结束时间
	EndTime time.Time `json:"end_time"`
	// Duration 耗时
	Duration time.Duration `json:"duration"`
	// Groups 重复文件组详情
	Groups []*DuplicateGroup `json:"groups"`
}

// Manager 文件去重管理器
type Manager struct {
	mu sync.RWMutex

	// config 配置
	config *ManagerConfig

	// files 文件信息缓存 (path -> FileInfo)
	files map[string]*FileInfo

	// hashGroups 哈希分组 (hash -> 文件路径列表)
	hashGroups map[string][]string

	// duplicateGroups 重复文件组
	duplicateGroups []*DuplicateGroup

	// excludeRules 排除规则列表
	excludeRules []*ExcludeRule

	// tasks 任务列表
	tasks map[string]*DedupTask

	// reports 报告列表
	reports []*DedupReport

	// lastScanTime 上次扫描时间
	lastScanTime time.Time

	// running 是否有任务在运行
	running bool
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	// DefaultAlgorithm 默认哈希算法
	DefaultAlgorithm HashAlgorithm `json:"default_algorithm"`
	// DefaultStrategy 默认去重策略
	DefaultStrategy DedupStrategy `json:"default_strategy"`
	// MinFileSize 最小扫描文件大小（字节）
	MinFileSize int64 `json:"min_file_size"`
	// MaxFileSize 最大扫描文件大小（字节）
	MaxFileSize int64 `json:"max_file_size"`
	// SoftDeleteDir 软删除文件存放目录
	SoftDeleteDir string `json:"soft_delete_dir"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *ManagerConfig {
	return &ManagerConfig{
		DefaultAlgorithm: HashSHA256,
		DefaultStrategy:  StrategyKeepNewest,
		MinFileSize:      0,
		MaxFileSize:      0,
		SoftDeleteDir:    "",
	}
}

// NewManager 创建新的文件去重管理器
func NewManager(config *ManagerConfig) *Manager {
	if config == nil {
		config = DefaultConfig()
	}
	return &Manager{
		config:      config,
		files:       make(map[string]*FileInfo),
		hashGroups:  make(map[string][]string),
		tasks:       make(map[string]*DedupTask),
		excludeRules: make([]*ExcludeRule, 0),
		reports:     make([]*DedupReport, 0),
	}
}

// AddExcludeRule 添加排除规则
func (m *Manager) AddExcludeRule(rule *ExcludeRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.excludeRules = append(m.excludeRules, rule)
}

// RemoveExcludeRule 移除指定索引的排除规则
func (m *Manager) RemoveExcludeRule(index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index < 0 || index >= len(m.excludeRules) {
		return fmt.Errorf("无效的规则索引: %d", index)
	}
	m.excludeRules = append(m.excludeRules[:index], m.excludeRules[index+1:]...)
	return nil
}

// GetExcludeRules 获取所有排除规则
func (m *Manager) GetExcludeRules() []*ExcludeRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rules := make([]*ExcludeRule, len(m.excludeRules))
	copy(rules, m.excludeRules)
	return rules
}

// shouldExclude 判断文件是否应被排除
func (m *Manager) shouldExclude(path string, size int64) bool {
	for _, rule := range m.excludeRules {
		// 检查大小限制
		if rule.MinSize > 0 && size < rule.MinSize {
			continue
		}
		if rule.MaxSize > 0 && size > rule.MaxSize {
			continue
		}

		// 检查扩展名
		if len(rule.Extensions) > 0 {
			ext := strings.ToLower(filepath.Ext(path))
			matched := false
			for _, ruleExt := range rule.Extensions {
				if strings.ToLower(ruleExt) == ext {
					matched = true
					break
				}
			}
			if matched {
				return true
			}
		}

		// 检查路径模式
		if rule.Path != "" {
			matched, _ := filepath.Match(rule.Path, filepath.Base(path))
			if !matched {
				// 也尝试匹配完整路径
				matched, _ = filepath.Match(rule.Path, path)
			}
			if matched {
				return true
			}
		}

		// 如果规则没有设置任何过滤条件，则不排除
		if rule.Path == "" && len(rule.Extensions) == 0 && rule.MinSize == 0 && rule.MaxSize == 0 {
			continue
		}

		// 如果只有大小条件且通过了，排除
		if rule.Path == "" && len(rule.Extensions) == 0 && (rule.MinSize > 0 || rule.MaxSize > 0) {
			return true
		}
	}

	// 检查全局大小限制
	if m.config.MinFileSize > 0 && size < m.config.MinFileSize {
		return true
	}
	if m.config.MaxFileSize > 0 && size > m.config.MaxFileSize {
		return true
	}

	return false
}

// computeHash 计算文件哈希值
func computeHash(path string, algorithm HashAlgorithm) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	var h io.Writer
	switch algorithm {
	case HashMD5:
		md5Hash := md5.New()
		h = md5Hash
		if _, err := io.Copy(h, f); err != nil {
			return "", fmt.Errorf("计算哈希失败: %w", err)
		}
		return hex.EncodeToString(md5Hash.Sum(nil)), nil
	case HashSHA256:
		shaHash := sha256.New()
		h = shaHash
		if _, err := io.Copy(h, f); err != nil {
			return "", fmt.Errorf("计算哈希失败: %w", err)
		}
		return hex.EncodeToString(shaHash.Sum(nil)), nil
	default:
		return "", ErrInvalidHashAlgorithm
	}
}

// Scan 执行文件扫描
func (m *Manager) Scan(paths []string, mode ScanMode, algorithm HashAlgorithm) (*DedupTask, error) {
	m.mu.Lock()

	if m.running {
		m.mu.Unlock()
		return nil, ErrTaskRunning
	}

	if algorithm == "" {
		algorithm = m.config.DefaultAlgorithm
	}

	if algorithm != HashMD5 && algorithm != HashSHA256 {
		m.mu.Unlock()
		return nil, ErrInvalidHashAlgorithm
	}

	taskID := fmt.Sprintf("scan_%d", time.Now().UnixNano())
	task := &DedupTask{
		TaskID:    taskID,
		ScanPaths: paths,
		Mode:      mode,
		Algorithm: algorithm,
		Status:    "running",
		StartTime: time.Now(),
		Progress:  0,
	}
	m.tasks[taskID] = task
	m.running = true
	m.mu.Unlock()

	// 执行扫描
	err := m.doScan(paths, mode, algorithm, task)

	m.mu.Lock()
	m.running = false
	task.EndTime = time.Now()
	if err != nil {
		task.Status = "failed"
		task.Error = err.Error()
	} else {
		task.Status = "completed"
		task.Progress = 100
	}
	m.mu.Unlock()

	return task, err
}

// doScan 执行实际的扫描逻辑
func (m *Manager) doScan(paths []string, mode ScanMode, algorithm HashAlgorithm, task *DedupTask) error {
	// 收集文件列表
	var fileList []string
	for _, root := range paths {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // 跳过无法访问的文件
			}
			if info.IsDir() {
				return nil
			}

			// 增量模式下跳过已扫描且未修改的文件
			if mode == ScanModeIncremental && !m.lastScanTime.IsZero() {
				if info.ModTime().Before(m.lastScanTime) {
					if _, exists := m.files[path]; exists {
						return nil
					}
				}
			}

			if m.shouldExclude(path, info.Size()) {
				return nil
			}

			fileList = append(fileList, path)
			return nil
		})
		if err != nil {
			return fmt.Errorf("遍历路径 %s 失败: %w", root, err)
		}
	}

	totalFiles := len(fileList)

	// 清空旧的分组（全量模式）
	if mode == ScanModeFull {
		m.mu.Lock()
		m.files = make(map[string]*FileInfo)
		m.hashGroups = make(map[string][]string)
		m.duplicateGroups = nil
		m.mu.Unlock()
	}

	// 逐文件计算哈希并分组
	for i, path := range fileList {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		hash, err := computeHash(path, algorithm)
		if err != nil {
			continue
		}

		m.mu.Lock()
		fileInfo := &FileInfo{
			Path:     path,
			Size:     info.Size(),
			Hash:     hash,
			ModTime:  info.ModTime(),
			Status:   StatusActive,
			ScanTime: time.Now(),
		}
		m.files[path] = fileInfo
		m.hashGroups[hash] = append(m.hashGroups[hash], path)
		m.mu.Unlock()

		// 更新进度
		if totalFiles > 0 {
			m.mu.Lock()
			task.Progress = float64(i+1) / float64(totalFiles) * 100
			m.mu.Unlock()
		}
	}

	// 识别重复文件组
	m.mu.Lock()
	m.duplicateGroups = nil
	groupID := 0
	for hash, paths := range m.hashGroups {
		if len(paths) < 2 {
			continue
		}

		files := make([]*FileInfo, 0, len(paths))
		var size int64
		for _, p := range paths {
			if fi, ok := m.files[p]; ok {
				files = append(files, fi)
				size = fi.Size
			}
		}

		if len(files) < 2 {
			continue
		}

		// 按修改时间排序
		sort.Slice(files, func(i, j int) bool {
			return files[i].ModTime.After(files[j].ModTime)
		})

		groupID++
		group := &DuplicateGroup{
			GroupID:     fmt.Sprintf("group_%d", groupID),
			Hash:        hash,
			Size:        size,
			Files:       files,
			WastedSpace: size * int64(len(files)-1),
		}
		m.duplicateGroups = append(m.duplicateGroups, group)
	}
	m.lastScanTime = time.Now()
	m.mu.Unlock()

	return nil
}

// GetDuplicateGroups 获取所有重复文件组
func (m *Manager) GetDuplicateGroups() []*DuplicateGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()
	groups := make([]*DuplicateGroup, len(m.duplicateGroups))
	copy(groups, m.duplicateGroups)
	return groups
}

// GetDuplicateGroupByHash 根据哈希值获取重复文件组
func (m *Manager) GetDuplicateGroupByHash(hash string) *DuplicateGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, group := range m.duplicateGroups {
		if group.Hash == hash {
			return group
		}
	}
	return nil
}

// Deduplicate 执行去重操作
func (m *Manager) Deduplicate(strategy DedupStrategy, confirmDelete bool) (*DedupReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if strategy == "" {
		strategy = m.config.DefaultStrategy
	}

	if !isValidStrategy(strategy) {
		return nil, ErrInvalidStrategy
	}

	if len(m.duplicateGroups) == 0 {
		return nil, ErrNoDuplicates
	}

	report := &DedupReport{
		TaskID:    fmt.Sprintf("dedup_%d", time.Now().UnixNano()),
		StartTime: time.Now(),
		Groups:    make([]*DuplicateGroup, 0),
	}

	var totalWasted int64
	var totalRecovered int64
	var duplicateFileCount int64
	var softDeletedCount int64
	var confirmedDeletedCount int64

	for _, group := range m.duplicateGroups {
		if len(group.Files) < 2 {
			continue
		}

		// 根据策略选择保留的文件
		keepIndex := selectFileToKeep(group.Files, strategy)
		report.Groups = append(report.Groups, group)

		totalWasted += group.WastedSpace
		duplicateFileCount += int64(len(group.Files) - 1)

		// 处理需要删除的文件
		for i, file := range group.Files {
			if i == keepIndex {
				continue
			}

			if confirmDelete {
				// 直接确认删除
				if err := os.Remove(file.Path); err == nil {
					file.Status = StatusConfirmedDeleted
					totalRecovered += file.Size
					confirmedDeletedCount++
					delete(m.files, file.Path)
				}
			} else {
				// 软删除
				file.Status = StatusSoftDeleted
				softDeletedCount++
			}
		}
	}

	// 统计总文件数和总大小
	var totalFiles int64
	var totalSize int64
	for _, f := range m.files {
		totalFiles++
		totalSize += f.Size
	}

	report.TotalFiles = totalFiles
	report.DuplicateFiles = duplicateFileCount
	report.DuplicateGroups = int64(len(report.Groups))
	report.TotalSize = totalSize
	report.WastedSpace = totalWasted
	report.RecoveredSpace = totalRecovered
	report.SoftDeletedCount = softDeletedCount
	report.ConfirmedDeletedCount = confirmedDeletedCount
	report.EndTime = time.Now()
	report.Duration = report.EndTime.Sub(report.StartTime)

	m.reports = append(m.reports, report)
	return report, nil
}

// selectFileToKeep 根据策略选择保留的文件索引
func selectFileToKeep(files []*FileInfo, strategy DedupStrategy) int {
	if len(files) == 0 {
		return 0
	}

	switch strategy {
	case StrategyKeepNewest:
		// 文件已按修改时间降序排列，保留第一个（最新）
		return 0
	case StrategyKeepOldest:
		// 保留最后一个（最旧）
		return len(files) - 1
	case StrategyKeepShortestPath:
		minIdx := 0
		for i := 1; i < len(files); i++ {
			if len(files[i].Path) < len(files[minIdx].Path) {
				minIdx = i
			}
		}
		return minIdx
	case StrategyKeepLongestPath:
		maxIdx := 0
		for i := 1; i < len(files); i++ {
			if len(files[i].Path) > len(files[maxIdx].Path) {
				maxIdx = i
			}
		}
		return maxIdx
	default:
		return 0
	}
}

// isValidStrategy 检查策略是否有效
func isValidStrategy(strategy DedupStrategy) bool {
	switch strategy {
	case StrategyKeepNewest, StrategyKeepOldest, StrategyKeepShortestPath, StrategyKeepLongestPath:
		return true
	default:
		return false
	}
}

// SoftDeleteFile 软删除指定文件
func (m *Manager) SoftDeleteFile(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fi, ok := m.files[path]
	if !ok {
		return ErrFileNotFound
	}

	fi.Status = StatusSoftDeleted
	return nil
}

// ConfirmDelete 确认删除所有软删除的文件
func (m *Manager) ConfirmDelete() (int64, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var count int64
	var recovered int64
	found := false

	for _, fi := range m.files {
		if fi.Status == StatusSoftDeleted {
			found = true
			if err := os.Remove(fi.Path); err == nil {
				fi.Status = StatusConfirmedDeleted
				recovered += fi.Size
				count++
				delete(m.files, fi.Path)
			}
		}
	}

	if !found {
		return 0, 0, ErrNoSoftDeleted
	}

	return count, recovered, nil
}

// GetSoftDeletedFiles 获取所有软删除文件
func (m *Manager) GetSoftDeletedFiles() []*FileInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*FileInfo
	for _, fi := range m.files {
		if fi.Status == StatusSoftDeleted {
			result = append(result, fi)
		}
	}
	return result
}

// RestoreFile 恢复软删除的文件
func (m *Manager) RestoreFile(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fi, ok := m.files[path]
	if !ok {
		return ErrFileNotFound
	}

	if fi.Status != StatusSoftDeleted {
		return fmt.Errorf("文件 %s 不是软删除状态", path)
	}

	fi.Status = StatusActive
	return nil
}

// GetTask 获取任务信息
func (m *Manager) GetTask(taskID string) *DedupTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tasks[taskID]
}

// GetTasks 获取所有任务
func (m *Manager) GetTasks() []*DedupTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tasks := make([]*DedupTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// GetReports 获取所有报告
func (m *Manager) GetReports() []*DedupReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	reports := make([]*DedupReport, len(m.reports))
	copy(reports, m.reports)
	return reports
}

// GenerateReport 生成当前状态的去重报告
func (m *Manager) GenerateReport() *DedupReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &DedupReport{
		TaskID:    fmt.Sprintf("report_%d", time.Now().UnixNano()),
		StartTime: time.Now(),
	}

	var totalSize int64
	var duplicateFileCount int64
	var wastedSpace int64

	for _, group := range m.duplicateGroups {
		duplicateFileCount += int64(len(group.Files) - 1)
		wastedSpace += group.WastedSpace
		report.Groups = append(report.Groups, group)
	}

	for _, f := range m.files {
		report.TotalFiles++
		totalSize += f.Size
	}

	report.TotalSize = totalSize
	report.DuplicateFiles = duplicateFileCount
	report.DuplicateGroups = int64(len(m.duplicateGroups))
	report.WastedSpace = wastedSpace
	report.EndTime = time.Now()
	report.Duration = report.EndTime.Sub(report.StartTime)

	return report
}

// GetFileInfo 获取指定文件信息
func (m *Manager) GetFileInfo(path string) (*FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fi, ok := m.files[path]
	if !ok {
		return nil, ErrFileNotFound
	}
	return fi, nil
}

// GetAllFiles 获取所有文件信息
func (m *Manager) GetAllFiles() map[string]*FileInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*FileInfo, len(m.files))
	for k, v := range m.files {
		result[k] = v
	}
	return result
}

// GetFileCount 获取扫描的文件总数
func (m *Manager) GetFileCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.files)
}

// GetDuplicateCount 获取重复文件组数
func (m *Manager) GetDuplicateCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.duplicateGroups)
}

// GetWastedSpace 获取浪费的总空间
func (m *Manager) GetWastedSpace() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var total int64
	for _, g := range m.duplicateGroups {
		total += g.WastedSpace
	}
	return total
}

// ExportReportAsJSON 导出报告为 JSON 格式
func (m *Manager) ExportReportAsJSON(report *DedupReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// ImportExcludeRules 从 JSON 导入排除规则
func (m *Manager) ImportExcludeRules(data []byte) error {
	var rules []*ExcludeRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return fmt.Errorf("解析排除规则失败: %w", err)
	}
	m.mu.Lock()
	m.excludeRules = append(m.excludeRules, rules...)
	m.mu.Unlock()
	return nil
}

// ExportExcludeRules 导出排除规则为 JSON
func (m *Manager) ExportExcludeRules() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return json.MarshalIndent(m.excludeRules, "", "  ")
}

// Clear 清空所有数据
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files = make(map[string]*FileInfo)
	m.hashGroups = make(map[string][]string)
	m.duplicateGroups = nil
	m.tasks = make(map[string]*DedupTask)
	m.reports = nil
	m.lastScanTime = time.Time{}
}

// GetLastScanTime 获取上次扫描时间
func (m *Manager) GetLastScanTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastScanTime
}

// ScanSingleFile 扫描单个文件并返回哈希值
func (m *Manager) ScanSingleFile(path string, algorithm HashAlgorithm) (string, error) {
	if algorithm == "" {
		algorithm = m.config.DefaultAlgorithm
	}

	if algorithm != HashMD5 && algorithm != HashSHA256 {
		return "", ErrInvalidHashAlgorithm
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("获取文件信息失败: %w", err)
	}

	if m.shouldExclude(path, info.Size()) {
		return "", fmt.Errorf("文件 %s 被排除规则过滤", path)
	}

	hash, err := computeHash(path, algorithm)
	if err != nil {
		return "", err
	}

	// 更新缓存
	m.mu.Lock()
	m.files[path] = &FileInfo{
		Path:     path,
		Size:     info.Size(),
		Hash:     hash,
		ModTime:  info.ModTime(),
		Status:   StatusActive,
		ScanTime: time.Now(),
	}
	m.hashGroups[hash] = append(m.hashGroups[hash], path)
	m.mu.Unlock()

	return hash, nil
}

// FindDuplicatesByHash 根据哈希值查找重复文件
func (m *Manager) FindDuplicatesByHash(hash string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	paths, ok := m.hashGroups[hash]
	if !ok {
		return nil
	}
	result := make([]string, len(paths))
	copy(result, paths)
	return result
}
