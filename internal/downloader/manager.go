package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 下载管理器
type Manager struct {
	mu         sync.RWMutex
	tasks      map[string]*DownloadTask
	dataDir    string
	configFile string
	ctx        context.Context
	cancel     context.CancelFunc

	// 回调函数
	onTaskUpdate func(*DownloadTask)

	// Transmission/qBittorrent 客户端配置
	transmissionURL string
	qbittorrentURL  string
}

// NewManager 创建下载管理器
func NewManager(dataDir string) (*Manager, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		tasks:      make(map[string]*DownloadTask),
		dataDir:    dataDir,
		configFile: filepath.Join(dataDir, "tasks.json"),
		ctx:        ctx,
		cancel:     cancel,
	}

	// 加载已有任务
	if err := m.loadTasks(); err != nil {
		return nil, err
	}

	// 启动后台任务
	go m.backgroundRunner()

	return m, nil
}

// SetTransmissionURL 设置 Transmission 地址
func (m *Manager) SetTransmissionURL(url string) {
	m.transmissionURL = url
}

// SetQbittorrentURL 设置 qBittorrent 地址
func (m *Manager) SetQbittorrentURL(url string) {
	m.qbittorrentURL = url
}

// SetOnTaskUpdate 设置任务更新回调
func (m *Manager) SetOnTaskUpdate(callback func(*DownloadTask)) {
	m.onTaskUpdate = callback
}

// CreateTask 创建下载任务
func (m *Manager) CreateTask(req CreateTaskRequest) (*DownloadTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 自动检测类型
	taskType := req.Type
	if taskType == "" {
		taskType = m.detectType(req.URL)
	}

	// 生成 ID
	taskID := uuid.New().String()[:8]

	// 创建任务
	task := &DownloadTask{
		ID:         taskID,
		Name:       req.Name,
		Type:       taskType,
		URL:        req.URL,
		Status:     StatusWaiting,
		Progress:   0,
		DestPath:   req.DestPath,
		Schedule:   req.Schedule,
		SpeedLimit: req.SpeedLimit,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// 如果名字为空，尝试从 URL 提取
	if task.Name == "" {
		task.Name = m.extractNameFromURL(req.URL)
	}

	// 默认保存路径
	if task.DestPath == "" {
		task.DestPath = filepath.Join(m.dataDir, "downloads")
	}

	m.tasks[task.ID] = task

	// 保存到文件
	if err := m.saveTasks(); err != nil {
		return nil, err
	}

	// 触发回调
	if m.onTaskUpdate != nil {
		m.onTaskUpdate(task)
	}

	return task, nil
}

// detectType 检测下载类型
func (m *Manager) detectType(url string) DownloadType {
	if strings.HasPrefix(url, "magnet:?") {
		return TypeMagnet
	}
	if strings.HasSuffix(url, ".torrent") {
		return TypeBT
	}
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return TypeHTTP
	}
	if strings.HasPrefix(url, "ftp://") {
		return TypeFTP
	}
	return TypeHTTP // 默认
}

// extractNameFromURL 从 URL 提取文件名
func (m *Manager) extractNameFromURL(url string) string {
	if strings.HasPrefix(url, "magnet:?") {
		// 从磁力链接提取 name 参数
		parts := strings.Split(url, "&")
		for _, part := range parts {
			if strings.HasPrefix(part, "dn=") {
				name := strings.TrimPrefix(part, "dn=")
				return strings.ReplaceAll(name, "+", " ")
			}
		}
		return "Unknown Torrent"
	}

	// HTTP/FTP URL
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return filepath.Base(parts[len(parts)-1])
	}
	return "Unknown File"
}

// GetTask 获取任务
func (m *Manager) GetTask(id string) (*DownloadTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, false
	}

	// 返回副本
	taskCopy := *task
	return &taskCopy, true
}

// ListTasks 列出所有任务
func (m *Manager) ListTasks(status DownloadStatus) []*DownloadTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tasks []*DownloadTask
	for _, task := range m.tasks {
		if status == "" || task.Status == status {
			taskCopy := *task
			tasks = append(tasks, &taskCopy)
		}
	}

	// 按创建时间排序（新的在前）
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	return tasks
}

// UpdateTask 更新任务
func (m *Manager) UpdateTask(id string, req UpdateTaskRequest) (*DownloadTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, fmt.Errorf("任务不存在：%s", id)
	}

	if req.Status != "" {
		task.Status = req.Status
		task.UpdatedAt = time.Now()
	}

	if req.SpeedLimit != nil {
		task.SpeedLimit = req.SpeedLimit
		task.UpdatedAt = time.Now()
	}

	if req.Schedule != nil {
		task.Schedule = req.Schedule
		task.UpdatedAt = time.Now()
	}

	// 保存
	if err := m.saveTasks(); err != nil {
		return nil, err
	}

	// 触发回调
	if m.onTaskUpdate != nil {
		m.onTaskUpdate(task)
	}

	taskCopy := *task
	return &taskCopy, nil
}

// DeleteTask 删除任务
func (m *Manager) DeleteTask(id string, deleteFiles bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return fmt.Errorf("任务不存在：%s", id)
	}

	// 删除文件
	if deleteFiles && task.DestPath != "" {
		// TODO: 实际删除文件逻辑
		// 当前为空实现，避免 staticcheck 警告
	}

	delete(m.tasks, id)

	// 保存
	if err := m.saveTasks(); err != nil {
		return err
	}

	return nil
}

// PauseTask 暂停任务
func (m *Manager) PauseTask(id string) error {
	_, err := m.UpdateTask(id, UpdateTaskRequest{Status: StatusPaused})
	return err
}

// ResumeTask 恢复任务
func (m *Manager) ResumeTask(id string) error {
	_, err := m.UpdateTask(id, UpdateTaskRequest{Status: StatusDownloading})
	return err
}

// GetStats 获取统计信息
func (m *Manager) GetStats() TaskStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := TaskStats{}
	for _, task := range m.tasks {
		stats.TotalTasks++
		stats.TotalSpeed += task.Speed
		stats.TotalUploaded += task.Uploaded

		switch task.Status {
		case StatusDownloading:
			stats.Downloading++
		case StatusWaiting:
			stats.Waiting++
		case StatusPaused:
			stats.Paused++
		case StatusCompleted:
			stats.Completed++
		case StatusSeeding:
			stats.Seeding++
		}
	}

	return stats
}

// backgroundRunner 后台运行器
func (m *Manager) backgroundRunner() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.updateTasks()
		}
	}
}

// updateTasks 更新任务状态
func (m *Manager) updateTasks() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	for _, task := range m.tasks {
		// 模拟进度更新（实际应该调用 Transmission/qBittorrent API）
		if task.Status == StatusDownloading {
			// 模拟下载进度
			if task.Progress < 100 {
				task.Progress += 0.5
				task.Downloaded = int64(float64(task.TotalSize) * task.Progress / 100)
				task.Speed = 1024 * 1024 // 1MB/s 模拟
				task.UpdatedAt = now

				if task.Progress >= 100 {
					task.Progress = 100
					task.Status = StatusCompleted
					completedTime := time.Now()
					task.CompletedAt = &completedTime
				}

				if m.onTaskUpdate != nil {
					m.onTaskUpdate(task)
				}
			}
		}
	}

	// 定期保存
	_ = m.saveTasks()
}

// loadTasks 加载任务
func (m *Manager) loadTasks() error {
	data, err := os.ReadFile(m.configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var tasks []*DownloadTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		return err
	}

	for _, task := range tasks {
		m.tasks[task.ID] = task
	}

	return nil
}

// saveTasks 保存任务
func (m *Manager) saveTasks() error {
	var tasks []*DownloadTask
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}

	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configFile, data, 0644)
}

// Close 关闭管理器
func (m *Manager) Close() {
	m.cancel()
	_ = m.saveTasks()
}
