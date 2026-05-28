// Package smartrecycle 智能回收清理 - 自动识别大文件/重复/过期文件
package smartrecycle

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"
)

// FileType 文件类型
type FileType string

const (
	FileLarge     FileType = "large"
	FileDuplicate FileType = "duplicate"
	FileExpired   FileType = "expired"
	FileTemp      FileType = "temp"
	FileEmpty     FileType = "empty"
)

// CleanupItem 清理项
type CleanupItem struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	Type        FileType  `json:"type"`
	Hash        string    `json:"hash,omitempty"`
	LastAccess  time.Time `json:"last_access"`
	CreatedAt   time.Time `json:"created_at"`
	Score       int       `json:"score"` // 0-100, 越高越建议清理
	Selected    bool      `json:"selected"`
	Reason      string    `json:"reason"`
}

// ScanPolicy 扫描策略
type ScanPolicy struct {
	MinFileSize    int64    `json:"min_file_size"`    // 最小文件大小(bytes)
	MaxAge         int      `json:"max_age"`          // 最大保留天数
	TempExtensions []string `json:"temp_extensions"`  // 临时文件扩展名
	ExcludePaths   []string `json:"exclude_paths"`    // 排除路径
	IncludeHidden  bool     `json:"include_hidden"`
}

// ScanResult 扫描结果
type ScanResult struct {
	ID          string         `json:"id"`
	ScanPath    string         `json:"scan_path"`
	TotalFiles  int            `json:"total_files"`
	TotalSize   int64          `json:"total_size"`
	Items       []*CleanupItem `json:"items"`
	Saveable    int64          `json:"saveable"`
	ScannedAt   time.Time      `json:"scanned_at"`
	Duration    int64          `json:"duration_ms"`
}

// CleanupReport 清理报告
type CleanupReport struct {
	ID          string    `json:"id"`
	ScanID      string    `json:"scan_id"`
	Deleted     int       `json:"deleted"`
	FreedBytes  int64     `json:"freed_bytes"`
	Errors      []string  `json:"errors,omitempty"`
	CompletedAt time.Time `json:"completed_at"`
}

// Config 配置
type Config struct {
	AutoClean      bool   `json:"auto_clean"`
	ScheduleCron   string `json:"schedule_cron"`
	MaxScanDepth   int    `json:"max_scan_depth"`
	RecycleBinPath string `json:"recycle_bin_path"`
	RetentionDays  int    `json:"retention_days"`
}

// Manager 管理器
type Manager struct {
	mu       sync.RWMutex
	config   *Config
	results  map[string]*ScanResult
	reports  []*CleanupReport
	dataFile string
}

var (
	ErrScanNotFound  = errors.New("scan result not found")
	ErrPathRequired  = errors.New("scan path is required")
	ErrInvalidPolicy = errors.New("invalid scan policy")
)

// NewManager 创建管理器
func NewManager(dataFile string) *Manager {
	return &Manager{
		config: &Config{
			AutoClean:      false,
			MaxScanDepth:   10,
			RecycleBinPath: "/tmp/nas-recycle",
			RetentionDays:  30,
		},
		results:  make(map[string]*ScanResult),
		dataFile: dataFile,
	}
}

// Initialize 初始化
func (m *Manager) Initialize() error {
	return m.load()
}

// ScanPath 扫描路径
func (m *Manager) ScanPath(path string, policy *ScanPolicy) (*ScanResult, error) {
	if path == "" {
		return nil, ErrPathRequired
	}
	if policy == nil {
		policy = m.DefaultPolicy()
	}

	result := &ScanResult{
		ID:        fmt.Sprintf("scan-%d", time.Now().UnixNano()),
		ScanPath:  path,
		ScannedAt: time.Now(),
	}

	// 模拟扫描结果
	items := m.simulateScan(path, policy)
	result.Items = items
	result.TotalFiles = len(items)
	var totalSize, saveable int64
	for _, item := range items {
		totalSize += item.Size
		if item.Score >= 60 {
			saveable += item.Size
		}
	}
	result.TotalSize = totalSize
	result.Saveable = saveable

	m.mu.Lock()
	m.results[result.ID] = result
	m.mu.Unlock()

	return result, nil
}

func (m *Manager) simulateScan(path string, policy *ScanPolicy) []*CleanupItem {
	var items []*CleanupItem

	// 大文件
	items = append(items, &CleanupItem{
		ID: "large-1", Path: path + "/large-video.mkv", Size: 1024 * 1024 * 4500,
		Type: FileLarge, Score: 40, Reason: "大文件(4.5GB)",
	})
	items = append(items, &CleanupItem{
		ID: "large-2", Path: path + "/backup-2024.tar.gz", Size: 1024 * 1024 * 2000,
		Type: FileLarge, Score: 65, Reason: "旧备份文件(2GB)",
	})

	// 重复文件
	items = append(items, &CleanupItem{
		ID: "dup-1", Path: path + "/photos/IMG_001.jpg", Size: 1024 * 5000, Hash: "abc123",
		Type: FileDuplicate, Score: 80, Reason: "与 /backup/IMG_001.jpg 重复",
	})
	items = append(items, &CleanupItem{
		ID: "dup-2", Path: path + "/docs/report-copy.pdf", Size: 1024 * 2000, Hash: "def456",
		Type: FileDuplicate, Score: 75, Reason: "与 /docs/report.pdf 重复",
	})

	// 过期文件
	items = append(items, &CleanupItem{
		ID: "exp-1", Path: path + "/tmp/cache-2024.dat", Size: 1024 * 800,
		Type: FileExpired, Score: 90, Reason: "超过365天未访问",
	})

	// 临时文件
	items = append(items, &CleanupItem{
		ID: "tmp-1", Path: path + "/downloads/temp.part", Size: 1024 * 300,
		Type: FileTemp, Score: 95, Reason: "临时下载文件",
	})

	// 空文件
	items = append(items, &CleanupItem{
		ID: "empty-1", Path: path + "/old/empty.txt", Size: 0,
		Type: FileEmpty, Score: 100, Reason: "空文件",
	})

	sort.Slice(items, func(i, j int) bool { return items[i].Score > items[j].Score })
	return items
}

// GetScanResult 获取扫描结果
func (m *Manager) GetScanResult(id string) (*ScanResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result, ok := m.results[id]
	if !ok {
		return nil, ErrScanNotFound
	}
	return result, nil
}

// ListScanResults 列出扫描结果
func (m *Manager) ListScanResults() []*ScanResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var results []*ScanResult
	for _, r := range m.results {
		results = append(results, r)
	}
	return results
}

// ExecuteCleanup 执行清理
func (m *Manager) ExecuteCleanup(scanID string, itemIDs []string) (*CleanupReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result, ok := m.results[scanID]
	if !ok {
		return nil, ErrScanNotFound
	}

	report := &CleanupReport{
		ID:          fmt.Sprintf("cleanup-%d", time.Now().UnixNano()),
		ScanID:      scanID,
		CompletedAt: time.Now(),
	}

	idSet := make(map[string]bool)
	for _, id := range itemIDs {
		idSet[id] = true
	}

	for _, item := range result.Items {
		if idSet[item.ID] {
			report.Deleted++
			report.FreedBytes += item.Size
			item.Selected = true
		}
	}

	m.reports = append(m.reports, report)
	return report, m.save()
}

// DefaultPolicy 默认策略
func (m *Manager) DefaultPolicy() *ScanPolicy {
	return &ScanPolicy{
		MinFileSize:    1024 * 1024, // 1MB
		MaxAge:         180,         // 180天
		TempExtensions: []string{".tmp", ".temp", ".part", ".bak", ".swp"},
		ExcludePaths:   []string{"/proc", "/sys", "/dev"},
		IncludeHidden:  false,
	}
}

// GetStats 获取统计
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalFreed int64
	for _, r := range m.reports {
		totalFreed += r.FreedBytes
	}

	return map[string]interface{}{
		"total_scans":   len(m.results),
		"total_cleans":  len(m.reports),
		"total_freed":   totalFreed,
	}
}

func (m *Manager) load() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := os.ReadFile(m.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &m.results)
}

func (m *Manager) save() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := json.MarshalIndent(m.results, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.dataFile, data, 0644)
}

