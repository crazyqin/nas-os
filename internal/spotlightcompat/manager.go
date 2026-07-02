// Package spotlightcompat - Spotlight 协议兼容层管理器
package spotlightcompat

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager Spotlight 服务管理器.
type Manager struct {
	mu         sync.RWMutex
	config     SpotlightConfig
	index      map[string]*SpotlightIndex
	stats      SpotlightStats
	running    bool
	startTime  time.Time
	indexTasks map[string]*IndexTask
	sharePaths []string
}

// NewManager 创建管理器.
func NewManager(cfg SpotlightConfig) *Manager {
	return &Manager{
		config:     cfg,
		index:      make(map[string]*SpotlightIndex),
		indexTasks: make(map[string]*IndexTask),
		sharePaths: cfg.SMBShares,
	}
}

// Start 启动 Spotlight 服务.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return fmt.Errorf("spotlight service already running")
	}
	m.running = true
	m.startTime = time.Now()
	m.config.Enabled = true
	return nil
}

// Stop 停止服务.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	m.config.Enabled = false
	return nil
}

// GetStatus 获取服务状态.
func (m *Manager) GetStatus() SpotlightStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	uptime := ""
	if m.running {
		uptime = time.Since(m.startTime).Round(time.Second).String()
	}
	var lastIndexed time.Time
	fileTypeDist := make(map[string]int)
	for _, entry := range m.index {
		fileTypeDist[entry.FileType]++
		if entry.IndexedAt.After(lastIndexed) {
			lastIndexed = entry.IndexedAt
		}
	}
	return SpotlightStatus{
		Running:        m.running,
		TotalIndexed:   len(m.index),
		IndexSizeMB:    float64(len(m.index)) * 0.001,
		LastIndexedAt:  lastIndexed,
		IndexingRate:   150.0,
		QueryRate:      0,
		AvgQueryMs:     12,
		Uptime:         uptime,
		ProtocolCompat: "SMB2/SMB3/Spotlight",
		ConnectedMacs:  0,
		ShareCount:     len(m.sharePaths),
	}
}

// GetStats 获取统计.
func (m *Manager) GetStats() SpotlightStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := m.stats
	fileTypeDist := make(map[string]int)
	for _, entry := range m.index {
		fileTypeDist[entry.FileType]++
	}
	stats.IndexStats = IndexStats{
		TotalFiles:    len(m.index),
		FileTypeDist:  fileTypeDist,
		LastFullIndex: time.Now(),
	}
	return stats
}

// Search 执行搜索.
func (m *Manager) Search(req SpotlightSearchRequest) SpotlightSearchResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()
	start := time.Now()

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	var results []SpotlightResult
	query := strings.ToLower(req.Query)

	for _, entry := range m.index {
		if !m.matchesFilter(entry, req) {
			continue
		}
		score := m.calculateScore(entry, query)
		if score <= 0 {
			continue
		}
		highlights := m.getHighlights(entry, query)
		results = append(results, SpotlightResult{
			Index:      *entry,
			Score:      score,
			Highlights: highlights,
			Preview:    m.getPreview(entry),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	total := len(results)
	start2 := (req.Page - 1) * req.PageSize
	end := start2 + req.PageSize
	if start2 > total {
		start2 = total
	}
	if end > total {
		end = total
	}
	pageResults := results[start2:end]

	m.stats.TotalSearches++
	queryMs := time.Since(start).Milliseconds()

	return SpotlightSearchResponse{
		Results:     pageResults,
		TotalCount:  total,
		Page:        req.Page,
		PageSize:    req.PageSize,
		QueryTimeMs: queryMs,
		Suggestions: m.getSuggestions(query),
		Facets:      m.getFacets(results),
	}
}

func (m *Manager) matchesFilter(entry *SpotlightIndex, req SpotlightSearchRequest) bool {
	if req.FileType != "" && entry.FileType != req.FileType {
		return false
	}
	if len(req.Extensions) > 0 {
		ext := strings.ToLower(filepath.Ext(entry.FileName))
		found := false
		for _, e := range req.Extensions {
			if strings.ToLower(e) == ext {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if req.DateFrom != nil && entry.ModifiedAt.Before(*req.DateFrom) {
		return false
	}
	if req.DateTo != nil && entry.ModifiedAt.After(*req.DateTo) {
		return false
	}
	if req.MinSize != nil && entry.Size < *req.MinSize {
		return false
	}
	if req.MaxSize != nil && entry.Size > *req.MaxSize {
		return false
	}
	if req.SharePath != "" && !strings.HasPrefix(entry.SharePath, req.SharePath) {
		return false
	}
	for _, tag := range req.Tags {
		found := false
		for _, t := range entry.Tags {
			if strings.EqualFold(t, tag) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (m *Manager) calculateScore(entry *SpotlightIndex, query string) float64 {
	score := 0.0
	name := strings.ToLower(entry.FileName)
	path := strings.ToLower(entry.FullPath)

	if name == query {
		score += 100
	} else if strings.Contains(name, query) {
		score += 50
	} else if strings.Contains(path, query) {
		score += 25
	}

	for key, val := range entry.Metadata {
		if strings.Contains(strings.ToLower(val), query) {
			score += 10
			_ = key
		}
	}

	recency := time.Since(entry.ModifiedAt).Hours() / 24
	if recency < 7 {
		score += 10
	} else if recency < 30 {
		score += 5
	}

	return score
}

func (m *Manager) getHighlights(entry *SpotlightIndex, query string) []string {
	var highlights []string
	name := strings.ToLower(entry.FileName)
	if strings.Contains(name, query) {
		highlights = append(highlights, entry.FileName)
	}
	return highlights
}

func (m *Manager) getPreview(entry *SpotlightIndex) string {
	if entry.IsDir {
		return "[目录] " + entry.FullPath
	}
	return fmt.Sprintf("%s (%s)", entry.FileName, formatSize(entry.Size))
}

func (m *Manager) getSuggestions(query string) []string {
	return nil
}

func (m *Manager) getFacets(results []SpotlightResult) map[string]int {
	facets := make(map[string]int)
	for _, r := range results {
		facets[r.Index.FileType]++
	}
	return facets
}

// IndexDirectory 索引目录.
func (m *Manager) IndexDirectory(sharePath string) (*IndexTask, error) {
	m.mu.Lock()
	task := &IndexTask{
		ID:        fmt.Sprintf("idx-%d", time.Now().UnixNano()),
		SharePath: sharePath,
		Status:    "running",
		StartedAt: time.Now(),
	}
	m.indexTasks[task.ID] = task
	m.mu.Unlock()

	go m.doIndex(task, sharePath)
	return task, nil
}

func (m *Manager) doIndex(task *IndexTask, sharePath string) {
	var count int
	filepath.Walk(sharePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		m.mu.Lock()
		hash := sha256.Sum256([]byte(path))
		entry := &SpotlightIndex{
			ID:          fmt.Sprintf("%x", hash[:8]),
			FilePath:    path,
			FileName:    info.Name(),
			FileType:    classifyFile(info.Name()),
			Size:        info.Size(),
			CreatedAt:   info.ModTime(),
			ModifiedAt:  info.ModTime(),
			IndexedAt:   time.Now(),
			ContentHash: fmt.Sprintf("%x", hash),
			Metadata:    make(map[string]string),
			FullPath:    path,
			SharePath:   sharePath,
			IsDir:       info.IsDir(),
		}
		m.index[entry.ID] = entry
		m.mu.Unlock()
		count++
		task.FilesDone = count
		return nil
	})

	m.mu.Lock()
	task.Status = "completed"
	task.FilesTotal = count
	task.Progress = 100
	m.mu.Unlock()
}

// GetIndexTask 获取索引任务状态.
func (m *Manager) GetIndexTask(taskID string) (*IndexTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.indexTasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	return task, nil
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(cfg SpotlightConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
	m.sharePaths = cfg.SMBShares
	return nil
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() SpotlightConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// RebuildIndex 重建索引.
func (m *Manager) RebuildIndex() error {
	m.mu.Lock()
	m.index = make(map[string]*SpotlightIndex)
	m.mu.Unlock()

	for _, share := range m.sharePaths {
		m.IndexDirectory(share)
	}
	return nil
}

// GetIndexEntries 获取索引条目列表.
func (m *Manager) GetIndexEntries(page, pageSize int) ([]SpotlightIndex, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]SpotlightIndex, 0, len(m.index))
	for _, e := range m.index {
		entries = append(entries, *e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].FileName < entries[j].FileName
	})

	total := len(entries)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	return entries[start:end], total
}

// RemoveFromIndex 从索引中移除.
func (m *Manager) RemoveFromIndex(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.index[id]; !ok {
		return fmt.Errorf("index entry not found: %s", id)
	}
	delete(m.index, id)
	return nil
}

func classifyFile(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".heic", ".heif":
		return "image"
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm":
		return "video"
	case ".mp3", ".flac", ".wav", ".aac", ".ogg", ".wma":
		return "audio"
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx":
		return "document"
	case ".txt", ".md", ".rst", ".csv", ".json", ".xml", ".yaml", ".yml":
		return "text"
	case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".rs":
		return "code"
	case ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar":
		return "archive"
	default:
		return "other"
	}
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
