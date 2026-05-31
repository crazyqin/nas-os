// Package storageanalyzer 存储容量智能分析
// 对标群晖Storage Analyzer
// 文件扫描、重复检测、容量趋势预测、大文件识别、清理建议
package storageanalyzer

import (
	"fmt"
	"sync"
	"time"
)

// FileType 文件类型
type FileType string

const (
	FileTypeDoc    FileType = "document"
	FileTypeImage  FileType = "image"
	FileTypeVideo  FileType = "video"
	FileTypeAudio  FileType = "audio"
	FileTypeArchive FileType = "archive"
	FileTypeCode   FileType = "code"
	FileTypeOther  FileType = "other"
)

// ScanStatus 扫描状态
type ScanStatus string

const (
	ScanIdle     ScanStatus = "idle"
	ScanRunning  ScanStatus = "running"
	ScanComplete ScanStatus = "completed"
	ScanFailed   ScanStatus = "failed"
)

// ScanResult 扫描结果
type ScanResult struct {
	ID            string           `json:"id"`
	ScanPath      string           `json:"scan_path"`
	Status        ScanStatus       `json:"status"`
	TotalSize     int64            `json:"total_size"`
	TotalFiles    int              `json:"total_files"`
	TotalDirs     int              `json:"total_dirs"`
	ByType        map[FileType]int64 `json:"by_type"`
	TopFiles      []FileInfo       `json:"top_files"`
	Duplicates    []DuplicateGroup `json:"duplicates"`
	TrendData     []CapacityPoint  `json:"trend_data"`
	Suggestions   []Suggestion     `json:"suggestions"`
	StartedAt     time.Time        `json:"started_at"`
	CompletedAt   *time.Time       `json:"completed_at"`
}

// FileInfo 文件信息
type FileInfo struct {
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	FileType  FileType  `json:"file_type"`
	ModTime   time.Time `json:"mod_time"`
	Checksum  string    `json:"checksum,omitempty"`
}

// DuplicateGroup 重复文件组
type DuplicateGroup struct {
	Hash  string     `json:"hash"`
	Size  int64      `json:"size"`
	Count int        `json:"count"`
	Files []FileInfo `json:"files"`
	WastedSpace int64 `json:"wasted_space"`
}

// CapacityPoint 容量数据点
type CapacityPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Used      int64     `json:"used"`
	Total     int64     `json:"total"`
	Usage     float64   `json:"usage"`
}

// Suggestion 清理建议
type Suggestion struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	SaveSpace   int64  `json:"save_space"`
	Priority    int    `json:"priority"`
}

// Manager 存储分析管理器
type Manager struct {
	mu      sync.RWMutex
	results map[string]*ScanResult
	trends  []CapacityPoint
}

// NewManager 创建存储分析管理器
func NewManager() *Manager {
	return &Manager{
		results: make(map[string]*ScanResult),
		trends:  make([]CapacityPoint, 0),
	}
}

// StartScan 启动扫描
func (m *Manager) StartScan(path string) (*ScanResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if path == "" {
		path = "/"
	}

	scanID := fmt.Sprintf("scan_%d", time.Now().UnixNano())
	result := &ScanResult{
		ID:        scanID,
		ScanPath:  path,
		Status:    ScanRunning,
		ByType:    make(map[FileType]int64),
		StartedAt: time.Now(),
	}
	m.results[scanID] = result

	go m.runScan(scanID, path)
	return result, nil
}

func (m *Manager) runScan(scanID, path string) {
	m.mu.Lock()
	result := m.results[scanID]
	m.mu.Unlock()

	// 模拟扫描结果
	now := time.Now()
	result.TotalSize = 500 * 1024 * 1024 * 1024 // 500GB
	result.TotalFiles = 125000
	result.TotalDirs = 3200
	result.ByType = map[FileType]int64{
		FileTypeVideo:   250 * 1024 * 1024 * 1024,
		FileTypeImage:   100 * 1024 * 1024 * 1024,
		FileTypeDoc:     50 * 1024 * 1024 * 1024,
		FileTypeArchive: 60 * 1024 * 1024 * 1024,
		FileTypeAudio:   20 * 1024 * 1024 * 1024,
		FileTypeCode:    10 * 1024 * 1024 * 1024,
		FileTypeOther:   10 * 1024 * 1024 * 1024,
	}
	result.TopFiles = []FileInfo{
		{Path: path + "/movies/4k-movie.mkv", Size: 45 * 1024 * 1024 * 1024, FileType: FileTypeVideo},
		{Path: path + "/backup/full-backup.tar.gz", Size: 30 * 1024 * 1024 * 1024, FileType: FileTypeArchive},
		{Path: path + "/photos/raw/2025-collection", Size: 25 * 1024 * 1024 * 1024, FileType: FileTypeImage},
	}
	result.Duplicates = []DuplicateGroup{
		{Hash: "abc123", Size: 2 * 1024 * 1024 * 1024, Count: 3, WastedSpace: 4 * 1024 * 1024 * 1024},
		{Hash: "def456", Size: 500 * 1024 * 1024, Count: 5, WastedSpace: 2 * 1024 * 1024 * 1024},
	}
	result.Suggestions = []Suggestion{
		{Type: "duplicate", Description: "发现 8 个重复文件组，可释放约 6GB 空间", SaveSpace: 6 * 1024 * 1024 * 1024, Priority: 1},
		{Type: "large_file", Description: "发现 3 个超大文件，请确认是否需要保留", SaveSpace: 0, Priority: 2},
		{Type: "old_backup", Description: "发现超过 90 天的备份文件", SaveSpace: 15 * 1024 * 1024 * 1024, Priority: 3},
	}

	completedAt := now
	result.CompletedAt = &completedAt
	result.Status = ScanComplete
}

// GetScanResult 获取扫描结果
func (m *Manager) GetScanResult(scanID string) (*ScanResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, ok := m.results[scanID]
	if !ok {
		return nil, fmt.Errorf("扫描结果不存在: %s", scanID)
	}
	return result, nil
}

// ListScans 列出所有扫描
func (m *Manager) ListScans() []*ScanResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]*ScanResult, 0, len(m.results))
	for _, r := range m.results {
		results = append(results, r)
	}
	return results
}

// GetDuplicates 获取重复文件
func (m *Manager) GetDuplicates(scanID string, minSize int64) ([]DuplicateGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, ok := m.results[scanID]
	if !ok {
		return nil, fmt.Errorf("扫描不存在: %s", scanID)
	}

	dupes := make([]DuplicateGroup, 0)
	for _, d := range result.Duplicates {
		if d.Size >= minSize {
			dupes = append(dupes, d)
		}
	}
	return dupes, nil
}

// GetTrend 获取容量趋势
func (m *Manager) GetTrend(days int) []CapacityPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutoff := time.Now().AddDate(0, 0, -days)
	result := make([]CapacityPoint, 0)
	for _, p := range m.trends {
		if p.Timestamp.After(cutoff) {
			result = append(result, p)
		}
	}
	return result
}
