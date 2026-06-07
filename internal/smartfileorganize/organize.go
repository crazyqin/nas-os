// Package smartfileorganize 提供AI驱动的智能文件整理功能
// 学习群晖 Synology Drive 智能分类与 TrueNAS 文件管理最佳实践
// 支持自动分类、智能标签、重复检测、整理建议

package smartfileorganize

import (
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// FileType 文件类型
type FileType string

const (
	FileTypeDocument FileType = "document"
	FileTypeImage    FileType = "image"
	FileTypeVideo    FileType = "video"
	FileTypeAudio    FileType = "audio"
	FileTypeArchive  FileType = "archive"
	FileTypeCode     FileType = "code"
	FileTypeOther    FileType = "other"
)

// OrganizationStrategy 整理策略
type OrganizationStrategy string

const (
	StrategyByType    OrganizationStrategy = "by_type"
	StrategyByDate    OrganizationStrategy = "by_date"
	StrategyByProject OrganizationStrategy = "by_project"
	StrategyBySize    OrganizationStrategy = "by_size"
	StrategySmart     OrganizationStrategy = "smart"
)

// FileInfo 文件信息
type FileInfo struct {
	Path        string            `json:"path"`
	Name        string            `json:"name"`
	Size        int64             `json:"size"`
	ModTime     time.Time         `json:"mod_time"`
	FileType    FileType          `json:"file_type"`
	Extension   string            `json:"extension"`
	Hash        string            `json:"hash"`
	Tags        []string          `json:"tags"`
	Category    string            `json:"category"`
	Metadata    map[string]string `json:"metadata"`
	IsDuplicate bool              `json:"is_duplicate"`
	DuplicateOf string            `json:"duplicate_of,omitempty"`
	Similarity  float64           `json:"similarity"`
}

// OrganizationTask 整理任务
type OrganizationTask struct {
	ID          string               `json:"id"`
	Strategy    OrganizationStrategy `json:"strategy"`
	SourcePaths []string             `json:"source_paths"`
	TargetPath  string               `json:"target_path"`
	Status      TaskStatus           `json:"status"`
	Progress    float64              `json:"progress"`
	Results     *OrganizationResult  `json:"results,omitempty"`
	CreatedAt   time.Time            `json:"created_at"`
	CompletedAt *time.Time           `json:"completed_at,omitempty"`
	Error       string               `json:"error,omitempty"`
}

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusAnalyzing  TaskStatus = "analyzing"
	TaskStatusOrganizing TaskStatus = "organizing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
)

// OrganizationResult 整理结果
type OrganizationResult struct {
	TotalFiles     int                `json:"total_files"`
	OrganizedFiles int                `json:"organized_files"`
	Duplicates     int                `json:"duplicates"`
	SpaceSaved     int64              `json:"space_saved"`
	Categories     map[string]int     `json:"categories"`
	Tags           map[string]int     `json:"tags"`
	Suggestions    []Suggestion       `json:"suggestions"`
	Timeline       []OrganizationStep `json:"timeline"`
}

// OrganizationStep 整理步骤
type OrganizationStep struct {
	Action    string    `json:"action"`
	Source    string    `json:"source"`
	Target    string    `json:"target"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
}

// Suggestion 整理建议
type Suggestion struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
	Priority    int      `json:"priority"`
	Impact      string   `json:"impact"`
}

// DuplicateGroup 重复文件组
type DuplicateGroup struct {
	Hash      string     `json:"hash"`
	Size      int64      `json:"size"`
	Count     int        `json:"count"`
	Files     []FileInfo `json:"files"`
	SpaceUsed int64      `json:"space_used"`
	CanDelete []string   `json:"can_delete"`
}

// Manager 智能文件整理管理器
type Manager struct {
	mu              sync.RWMutex
	tasks           map[string]*OrganizationTask
	files           map[string]*FileInfo
	duplicates      map[string]*DuplicateGroup
	categories      map[FileType][]string
	tags            map[string][]string
	autoOrganize    bool
	scanInterval    time.Duration
	maxFileSize     int64
	excludePatterns []string
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		tasks:           make(map[string]*OrganizationTask),
		files:           make(map[string]*FileInfo),
		duplicates:      make(map[string]*DuplicateGroup),
		categories:      make(map[FileType][]string),
		tags:            make(map[string][]string),
		autoOrganize:    false,
		scanInterval:    1 * time.Hour,
		maxFileSize:     10 * 1024 * 1024 * 1024, // 10GB
		excludePatterns: []string{".git", ".svn", "node_modules", ".DS_Store"},
	}
}

// ScanDirectory 扫描目录
func (m *Manager) ScanDirectory(path string, recursive bool) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟扫描
	count := 0
	extensions := map[string]FileType{
		".pdf": FileTypeDocument, ".doc": FileTypeDocument, ".docx": FileTypeDocument,
		".jpg": FileTypeImage, ".png": FileTypeImage, ".gif": FileTypeImage, ".webp": FileTypeImage,
		".mp4": FileTypeVideo, ".avi": FileTypeVideo, ".mkv": FileTypeVideo,
		".mp3": FileTypeAudio, ".flac": FileTypeAudio, ".wav": FileTypeAudio,
		".zip": FileTypeArchive, ".tar": FileTypeArchive, ".gz": FileTypeArchive,
		".go": FileTypeCode, ".py": FileTypeCode, ".js": FileTypeCode,
	}

	for ext, ft := range extensions {
		info := &FileInfo{
			Path:      filepath.Join(path, fmt.Sprintf("sample%s", ext)),
			Name:      fmt.Sprintf("sample%s", ext),
			Size:      1024 * 1024,
			ModTime:   time.Now(),
			FileType:  ft,
			Extension: ext,
			Tags:      []string{},
			Metadata:  make(map[string]string),
		}
		m.files[info.Path] = info
		m.categories[ft] = append(m.categories[ft], info.Path)
		count++
	}

	return count, nil
}

// AnalyzeDuplicates 分析重复文件
func (m *Manager) AnalyzeDuplicates() []*DuplicateGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hashMap := make(map[string][]FileInfo)
	for _, f := range m.files {
		if f.Hash != "" {
			hashMap[f.Hash] = append(hashMap[f.Hash], *f)
		}
	}

	var groups []*DuplicateGroup
	for hash, files := range hashMap {
		if len(files) > 1 {
			group := &DuplicateGroup{
				Hash:  hash,
				Size:  files[0].Size,
				Count: len(files),
				Files: files,
			}
			for i := 1; i < len(files); i++ {
				group.CanDelete = append(group.CanDelete, files[i].Path)
				group.SpaceUsed += files[i].Size
			}
			groups = append(groups, group)
		}
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].SpaceUsed > groups[j].SpaceUsed
	})

	return groups
}

// CreateTask 创建整理任务
func (m *Manager) CreateTask(strategy OrganizationStrategy, sourcePaths []string, targetPath string) *OrganizationTask {
	m.mu.Lock()
	defer m.mu.Unlock()

	task := &OrganizationTask{
		ID:          fmt.Sprintf("task_%d", time.Now().UnixNano()),
		Strategy:    strategy,
		SourcePaths: sourcePaths,
		TargetPath:  targetPath,
		Status:      TaskStatusPending,
		CreatedAt:   time.Now(),
	}

	m.tasks[task.ID] = task
	return task
}

// GetSuggestions 获取整理建议
func (m *Manager) GetSuggestions() []Suggestion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var suggestions []Suggestion

	// 检查大文件
	for _, f := range m.files {
		if f.Size > 100*1024*1024 { // > 100MB
			suggestions = append(suggestions, Suggestion{
				ID:          fmt.Sprintf("large_%s", f.Name),
				Type:        "large_file",
				Title:       fmt.Sprintf("大文件: %s", f.Name),
				Description: fmt.Sprintf("文件大小 %d MB，建议归档或压缩", f.Size/1024/1024),
				Files:       []string{f.Path},
				Priority:    2,
				Impact:      "medium",
			})
		}
	}

	// 检查旧文件
	threshold := time.Now().AddDate(0, -6, 0)
	for _, f := range m.files {
		if f.ModTime.Before(threshold) {
			suggestions = append(suggestions, Suggestion{
				ID:          fmt.Sprintf("old_%s", f.Name),
				Type:        "old_file",
				Title:       fmt.Sprintf("长期未访问: %s", f.Name),
				Description: "超过6个月未访问，建议归档",
				Files:       []string{f.Path},
				Priority:    1,
				Impact:      "low",
			})
		}
	}

	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Priority > suggestions[j].Priority
	})

	return suggestions
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_files": len(m.files),
		"categories":  make(map[string]int),
		"total_size":  int64(0),
		"duplicates":  len(m.duplicates),
	}

	categories := stats["categories"].(map[string]int)
	for ft, paths := range m.categories {
		categories[string(ft)] = len(paths)
	}

	var totalSize int64
	for _, f := range m.files {
		totalSize += f.Size
	}
	stats["total_size"] = totalSize

	return stats
}

// Close 关闭管理器
func (m *Manager) Close() error {
	return nil
}
