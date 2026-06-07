package smartdownloadmanager

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DownloadStatus 下载状态
type DownloadStatus string

const (
	StatusPending     DownloadStatus = "pending"
	StatusQueued      DownloadStatus = "queued"
	StatusDownloading DownloadStatus = "downloading"
	StatusPaused      DownloadStatus = "paused"
	StatusCompleted   DownloadStatus = "completed"
	StatusFailed      DownloadStatus = "failed"
	StatusCancelled   DownloadStatus = "cancelled"
)

// DownloadProtocol 下载协议
type DownloadProtocol string

const (
	ProtocolHTTP   DownloadProtocol = "http"
	ProtocolHTTPS  DownloadProtocol = "https"
	ProtocolFTP    DownloadProtocol = "ftp"
	ProtocolMagnet DownloadProtocol = "magnet"
)

// DownloadPriority 下载优先级
type DownloadPriority string

const (
	PriorityLow    DownloadPriority = "low"
	PriorityMedium DownloadPriority = "medium"
	PriorityHigh   DownloadPriority = "high"
	PriorityUrgent DownloadPriority = "urgent"
)

// FileCategory 文件分类
type FileCategory string

const (
	CategoryVideo    FileCategory = "video"
	CategoryMusic    FileCategory = "music"
	CategoryDocument FileCategory = "document"
	CategoryArchive  FileCategory = "archive"
	CategoryImage    FileCategory = "image"
	CategoryOther    FileCategory = "other"
)

// SpeedLimit 限速规则
type SpeedLimit struct {
	StartTime string `json:"start_time"` // HH:MM 格式
	EndTime   string `json:"end_time"`   // HH:MM 格式
	MaxSpeed  int64  `json:"max_speed"`  // 字节/秒
	Enabled   bool   `json:"enabled"`
}

// DownloadTask 下载任务
type DownloadTask struct {
	ID             string           `json:"id"`
	URL            string           `json:"url"`
	Protocol       DownloadProtocol `json:"protocol"`
	Filename       string           `json:"filename"`
	SavePath       string           `json:"save_path"`
	Category       FileCategory     `json:"category"`
	Status         DownloadStatus   `json:"status"`
	Priority       DownloadPriority `json:"priority"`
	TotalSize      int64            `json:"total_size"`
	DownloadedSize int64            `json:"downloaded_size"`
	Progress       float64          `json:"progress"`  // 0-100
	Speed          int64            `json:"speed"`     // 当前速度 字节/秒
	MaxSpeed       int64            `json:"max_speed"` // 限速 字节/秒，0=无限制
	ErrorMsg       string           `json:"error_msg,omitempty"`
	RetryCount     int              `json:"retry_count"`
	MaxRetries     int              `json:"max_retries"`
	ResumeSupport  bool             `json:"resume_support"` // 是否支持断点续传
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	StartedAt      *time.Time       `json:"started_at,omitempty"`
	CompletedAt    *time.Time       `json:"completed_at,omitempty"`
}

// DownloadStats 下载统计
type DownloadStats struct {
	TotalTasks      int            `json:"total_tasks"`
	ActiveTasks     int            `json:"active_tasks"`
	CompletedTasks  int            `json:"completed_tasks"`
	FailedTasks     int            `json:"failed_tasks"`
	TotalDownloaded int64          `json:"total_downloaded"`
	TasksByStatus   map[string]int `json:"tasks_by_status"`
	TasksByCategory map[string]int `json:"tasks_by_category"`
}

// DownloadManager 智能下载管理器
type DownloadManager struct {
	mu            sync.RWMutex
	tasks         map[string]*DownloadTask
	queue         []string                // 按优先级排序的任务ID队列
	maxConcurrent int                     // 最大并发下载数
	activeCount   int                     // 当前活跃下载数
	speedLimits   []SpeedLimit            // 限速规则
	downloadPath  string                  // 默认下载路径
	categories    map[FileCategory]string // 分类对应的子目录
}

// NewDownloadManager 创建下载管理器
func NewDownloadManager(downloadPath string, maxConcurrent int) *DownloadManager {
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}

	return &DownloadManager{
		tasks:         make(map[string]*DownloadTask),
		queue:         make([]string, 0),
		maxConcurrent: maxConcurrent,
		speedLimits:   make([]SpeedLimit, 0),
		downloadPath:  downloadPath,
		categories: map[FileCategory]string{
			CategoryVideo:    "videos",
			CategoryMusic:    "music",
			CategoryDocument: "documents",
			CategoryArchive:  "archives",
			CategoryImage:    "images",
			CategoryOther:    "others",
		},
	}
}

// AddTask 添加下载任务
func (dm *DownloadManager) AddTask(task *DownloadTask) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if _, exists := dm.tasks[task.ID]; exists {
		return fmt.Errorf("任务 %s 已存在", task.ID)
	}

	// 自动识别协议
	if task.Protocol == "" {
		task.Protocol = detectProtocol(task.URL)
	}

	// 自动分类
	if task.Category == "" {
		task.Category = classifyFile(task.Filename)
	}

	// 设置默认值
	if task.Priority == "" {
		task.Priority = PriorityMedium
	}
	if task.MaxRetries == 0 {
		task.MaxRetries = 3
	}
	if task.SavePath == "" {
		subDir := dm.categories[task.Category]
		task.SavePath = filepath.Join(dm.downloadPath, subDir)
	}

	task.Status = StatusQueued
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()

	dm.tasks[task.ID] = task
	dm.enqueueTask(task.ID)

	return nil
}

// RemoveTask 移除下载任务
func (dm *DownloadManager) RemoveTask(taskID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	task, exists := dm.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	if task.Status == StatusDownloading {
		dm.activeCount--
	}

	delete(dm.tasks, taskID)
	dm.removeFromQueue(taskID)

	return nil
}

// GetTask 获取下载任务
func (dm *DownloadManager) GetTask(taskID string) (*DownloadTask, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	task, exists := dm.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("任务 %s 不存在", taskID)
	}
	return task, nil
}

// ListTasks 列出下载任务
func (dm *DownloadManager) ListTasks(status DownloadStatus, category FileCategory, priority DownloadPriority) []*DownloadTask {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	tasks := make([]*DownloadTask, 0)
	for _, task := range dm.tasks {
		if status != "" && task.Status != status {
			continue
		}
		if category != "" && task.Category != category {
			continue
		}
		if priority != "" && task.Priority != priority {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks
}

// StartTask 开始下载任务
func (dm *DownloadManager) StartTask(taskID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	task, exists := dm.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	if task.Status != StatusQueued && task.Status != StatusPaused {
		return fmt.Errorf("任务 %s 当前状态 %s 无法启动", taskID, task.Status)
	}

	if dm.activeCount >= dm.maxConcurrent {
		return fmt.Errorf("已达到最大并发数 %d", dm.maxConcurrent)
	}

	task.Status = StatusDownloading
	task.UpdatedAt = time.Now()
	now := time.Now()
	task.StartedAt = &now
	dm.activeCount++

	return nil
}

// PauseTask 暂停下载任务
func (dm *DownloadManager) PauseTask(taskID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	task, exists := dm.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	if task.Status != StatusDownloading {
		return fmt.Errorf("任务 %s 当前状态 %s 无法暂停", taskID, task.Status)
	}

	task.Status = StatusPaused
	task.UpdatedAt = time.Now()
	dm.activeCount--

	return nil
}

// ResumeTask 恢复下载任务
func (dm *DownloadManager) ResumeTask(taskID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	task, exists := dm.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	if task.Status != StatusPaused {
		return fmt.Errorf("任务 %s 当前状态 %s 无法恢复", taskID, task.Status)
	}

	if !task.ResumeSupport {
		return fmt.Errorf("任务 %s 不支持断点续传", taskID)
	}

	if dm.activeCount >= dm.maxConcurrent {
		return fmt.Errorf("已达到最大并发数 %d", dm.maxConcurrent)
	}

	task.Status = StatusDownloading
	task.UpdatedAt = time.Now()
	dm.activeCount++

	return nil
}

// CancelTask 取消下载任务
func (dm *DownloadManager) CancelTask(taskID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	task, exists := dm.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	if task.Status == StatusCompleted {
		return fmt.Errorf("任务 %s 已完成，无法取消", taskID)
	}

	if task.Status == StatusDownloading {
		dm.activeCount--
	}

	task.Status = StatusCancelled
	task.UpdatedAt = time.Now()
	dm.removeFromQueue(taskID)

	return nil
}

// UpdateProgress 更新下载进度
func (dm *DownloadManager) UpdateProgress(taskID string, downloadedSize int64, speed int64) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	task, exists := dm.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	task.DownloadedSize = downloadedSize
	task.Speed = speed
	task.UpdatedAt = time.Now()

	if task.TotalSize > 0 {
		task.Progress = float64(downloadedSize) / float64(task.TotalSize) * 100
	}

	return nil
}

// CompleteTask 完成下载任务
func (dm *DownloadManager) CompleteTask(taskID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	task, exists := dm.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	if task.Status != StatusDownloading {
		return fmt.Errorf("任务 %s 当前状态 %s 无法完成", taskID, task.Status)
	}

	task.Status = StatusCompleted
	task.Progress = 100
	task.Speed = 0
	task.DownloadedSize = task.TotalSize
	task.UpdatedAt = time.Now()
	now := time.Now()
	task.CompletedAt = &now
	dm.activeCount--

	return nil
}

// FailTask 标记任务失败
func (dm *DownloadManager) FailTask(taskID string, errorMsg string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	task, exists := dm.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	task.ErrorMsg = errorMsg
	task.RetryCount++
	task.Speed = 0
	task.UpdatedAt = time.Now()

	if task.RetryCount >= task.MaxRetries {
		task.Status = StatusFailed
		dm.activeCount--
	} else {
		// 重新入队等待重试
		task.Status = StatusQueued
		dm.activeCount--
		dm.enqueueTask(task.ID)
	}

	return nil
}

// SetSpeedLimit 设置限速规则
func (dm *DownloadManager) SetSpeedLimit(limit SpeedLimit) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.speedLimits = append(dm.speedLimits, limit)
}

// ClearSpeedLimits 清除所有限速规则
func (dm *DownloadManager) ClearSpeedLimits() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.speedLimits = make([]SpeedLimit, 0)
}

// GetCurrentSpeedLimit 获取当前生效的限速
func (dm *DownloadManager) GetCurrentSpeedLimit() int64 {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	now := time.Now()
	currentTime := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	for _, limit := range dm.speedLimits {
		if !limit.Enabled {
			continue
		}
		if currentTime >= limit.StartTime && currentTime <= limit.EndTime {
			return limit.MaxSpeed
		}
	}

	return 0 // 无限制
}

// GetStats 获取下载统计
func (dm *DownloadManager) GetStats() *DownloadStats {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	stats := &DownloadStats{
		TasksByStatus:   make(map[string]int),
		TasksByCategory: make(map[string]int),
	}

	for _, task := range dm.tasks {
		stats.TotalTasks++
		stats.TasksByStatus[string(task.Status)]++
		stats.TasksByCategory[string(task.Category)]++

		switch task.Status {
		case StatusDownloading:
			stats.ActiveTasks++
		case StatusCompleted:
			stats.CompletedTasks++
			stats.TotalDownloaded += task.DownloadedSize
		case StatusFailed:
			stats.FailedTasks++
		}
	}

	return stats
}

// GetNextTask 获取下一个待下载任务（按优先级）
func (dm *DownloadManager) GetNextTask() *DownloadTask {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if len(dm.queue) == 0 {
		return nil
	}

	if dm.activeCount >= dm.maxConcurrent {
		return nil
	}

	taskID := dm.dequeueTask()
	if taskID == "" {
		return nil
	}

	task, exists := dm.tasks[taskID]
	if !exists {
		return nil
	}

	if task.Status != StatusQueued {
		return nil
	}

	return task
}

// SetMaxConcurrent 设置最大并发数
func (dm *DownloadManager) SetMaxConcurrent(max int) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if max > 0 {
		dm.maxConcurrent = max
	}
}

// GetMaxConcurrent 获取最大并发数
func (dm *DownloadManager) GetMaxConcurrent() int {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	return dm.maxConcurrent
}

// GetActiveCount 获取当前活跃下载数
func (dm *DownloadManager) GetActiveCount() int {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	return dm.activeCount
}

// enqueueTask 按优先级入队（内部方法，调用者需持有锁）
func (dm *DownloadManager) enqueueTask(taskID string) {
	task, exists := dm.tasks[taskID]
	if !exists {
		return
	}

	priorityOrder := map[DownloadPriority]int{
		PriorityUrgent: 0,
		PriorityHigh:   1,
		PriorityMedium: 2,
		PriorityLow:    3,
	}

	insertPos := len(dm.queue)
	for i, id := range dm.queue {
		if t, ok := dm.tasks[id]; ok {
			if priorityOrder[task.Priority] < priorityOrder[t.Priority] {
				insertPos = i
				break
			}
		}
	}

	dm.queue = append(dm.queue[:insertPos], append([]string{taskID}, dm.queue[insertPos:]...)...)
}

// dequeueTask 出队（内部方法，调用者需持有锁）
func (dm *DownloadManager) dequeueTask() string {
	if len(dm.queue) == 0 {
		return ""
	}

	taskID := dm.queue[0]
	dm.queue = dm.queue[1:]
	return taskID
}

// removeFromQueue 从队列移除（内部方法，调用者需持有锁）
func (dm *DownloadManager) removeFromQueue(taskID string) {
	for i, id := range dm.queue {
		if id == taskID {
			dm.queue = append(dm.queue[:i], dm.queue[i+1:]...)
			return
		}
	}
}

// detectProtocol 自动检测下载协议
func detectProtocol(url string) DownloadProtocol {
	urlLower := strings.ToLower(url)
	switch {
	case strings.HasPrefix(urlLower, "magnet:"):
		return ProtocolMagnet
	case strings.HasPrefix(urlLower, "ftp://"):
		return ProtocolFTP
	case strings.HasPrefix(urlLower, "https://"):
		return ProtocolHTTPS
	case strings.HasPrefix(urlLower, "http://"):
		return ProtocolHTTP
	default:
		return ProtocolHTTP
	}
}

// classifyFile 自动分类文件
func classifyFile(filename string) FileCategory {
	ext := strings.ToLower(filepath.Ext(filename))

	videoExts := map[string]bool{
		".mp4": true, ".avi": true, ".mkv": true, ".mov": true,
		".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
		".ts": true, ".rmvb": true, ".rm": true,
	}

	musicExts := map[string]bool{
		".mp3": true, ".flac": true, ".wav": true, ".aac": true,
		".ogg": true, ".wma": true, ".m4a": true, ".ape": true,
	}

	docExts := map[string]bool{
		".pdf": true, ".doc": true, ".docx": true, ".xls": true,
		".xlsx": true, ".ppt": true, ".pptx": true, ".txt": true,
		".md": true, ".csv": true, ".epub": true,
	}

	archiveExts := map[string]bool{
		".zip": true, ".rar": true, ".7z": true, ".tar": true,
		".gz": true, ".bz2": true, ".xz": true, ".tgz": true,
	}

	imageExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".bmp": true, ".webp": true, ".svg": true, ".ico": true,
	}

	switch {
	case videoExts[ext]:
		return CategoryVideo
	case musicExts[ext]:
		return CategoryMusic
	case docExts[ext]:
		return CategoryDocument
	case archiveExts[ext]:
		return CategoryArchive
	case imageExts[ext]:
		return CategoryImage
	default:
		return CategoryOther
	}
}

// ParseMagnetLink 解析磁力链接
func ParseMagnetLink(magnetURI string) (infoHash string, name string, err error) {
	if !strings.HasPrefix(magnetURI, "magnet:") {
		return "", "", fmt.Errorf("无效的磁力链接格式")
	}

	params := strings.Split(strings.TrimPrefix(magnetURI, "magnet:?"), "&")
	for _, param := range params {
		parts := strings.SplitN(param, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		value := parts[1]

		switch key {
		case "xt":
			if strings.HasPrefix(value, "urn:btih:") {
				infoHash = strings.TrimPrefix(value, "urn:btih:")
			}
		case "dn":
			name = value
		}
	}

	if infoHash == "" {
		return "", "", fmt.Errorf("磁力链接缺少 info hash")
	}

	return infoHash, name, nil
}

// FormatFileSize 格式化文件大小
func FormatFileSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FormatSpeed 格式化下载速度
func FormatSpeed(bytesPerSecond int64) string {
	return FormatFileSize(bytesPerSecond) + "/s"
}
