package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// 安全 HTTP 客户端，设置合理的超时时间
var safeHTTPClient = &http.Client{
	Timeout: 30 * time.Minute, // 下载可能需要较长时间
	Transport: &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  false,
		TLSHandshakeTimeout: 10 * time.Second,
	},
}

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
	transmissionURL      string
	transmissionUsername string
	transmissionPassword string
	qbittorrentURL       string
	qbittorrentUsername  string
	qbittorrentPassword  string

	// BT 客户端实例
	transmissionClient *TransmissionClient
	qbittorrentClient  *QBittorrentClient

	logger *zap.Logger
}

// NewManager 创建下载管理器
func NewManager(dataDir string, logger *zap.Logger) (*Manager, error) {
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
		logger:     logger,
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

// SetTransmissionAuth 设置 Transmission 认证信息
func (m *Manager) SetTransmissionAuth(username, password string) {
	m.transmissionUsername = username
	m.transmissionPassword = password
}

// SetQbittorrentURL 设置 qBittorrent 地址
func (m *Manager) SetQbittorrentURL(url string) {
	m.qbittorrentURL = url
}

// SetQbittorrentAuth 设置 qBittorrent 认证信息
func (m *Manager) SetQbittorrentAuth(username, password string) {
	m.qbittorrentUsername = username
	m.qbittorrentPassword = password
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
		// 构建完整文件路径
		filePath := filepath.Join(task.DestPath, task.Name)

		// 检查文件是否存在
		if info, err := os.Stat(filePath); err == nil {
			if info.IsDir() {
				// 删除目录（BT 下载可能是文件夹）
				if err := os.RemoveAll(filePath); err != nil {
					return fmt.Errorf("删除目录失败：%w", err)
				}
			} else {
				// 删除文件
				if err := os.Remove(filePath); err != nil {
					return fmt.Errorf("删除文件失败：%w", err)
				}
			}
		}

		// 如果配置了 Transmission/qBittorrent，从客户端也删除种子
		if task.Type == TypeBT || task.Type == TypeMagnet {
			if m.transmissionClient != nil && task.ClientRef != "" {
				// 从 ClientRef 解析 ID
				parts := strings.Split(task.ClientRef, ":")
				if len(parts) == 2 && parts[0] == "transmission" {
					var tid int
					if _, err := fmt.Sscanf(parts[1], "%d", &tid); err == nil {
						_ = m.transmissionClient.RemoveTorrent(tid, deleteFiles)
					}
				}
			}
			if m.qbittorrentClient != nil && task.DownloadID != "" {
				_ = m.qbittorrentClient.DeleteTorrent(task.DownloadID, deleteFiles)
			}
		}
	}

	delete(m.tasks, id)

	// 保存
	if err := m.saveTasks(); err != nil {
		return err
	}

	return nil
}

// StartTask 启动下载任务
func (m *Manager) StartTask(id string) error {
	m.mu.Lock()
	task, exists := m.tasks[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("任务不存在：%s", id)
	}

	if task.Status == StatusDownloading {
		m.mu.Unlock()
		return fmt.Errorf("任务已在下载中")
	}

	task.Status = StatusDownloading
	task.UpdatedAt = time.Now()
	m.mu.Unlock()

	// 根据类型启动不同的下载逻辑
	switch task.Type {
	case TypeHTTP, TypeFTP:
		go m.downloadHTTP(m.ctx, task)
	case TypeBT, TypeMagnet:
		go m.downloadBittorrent(m.ctx, task)
	}

	if m.onTaskUpdate != nil {
		m.onTaskUpdate(task)
	}

	return m.saveTasks()
}

// downloadBittorrent 启动 BT 下载
func (m *Manager) downloadBittorrent(ctx context.Context, task *DownloadTask) {
	// 如果配置了 Transmission
	if m.transmissionURL != "" {
		if err := m.addToTransmission(task); err != nil {
			m.logger.Error("添加到 Transmission 失败", zap.Error(err), zap.String("taskId", task.ID))
			m.mu.Lock()
			task.Status = StatusError
			task.ErrorMessage = err.Error()
			m.mu.Unlock()
			return
		}
	} else if m.qbittorrentURL != "" {
		// 如果配置了 qBittorrent
		if err := m.addToQbittorrent(task); err != nil {
			m.logger.Error("添加到 qBittorrent 失败", zap.Error(err), zap.String("taskId", task.ID))
			m.mu.Lock()
			task.Status = StatusError
			task.ErrorMessage = err.Error()
			m.mu.Unlock()
			return
		}
	} else {
		// 没有 BT 客户端，使用 aria2c 作为后备
		if err := m.downloadWithAria2c(ctx, task); err != nil {
			m.logger.Error("aria2c 下载失败", zap.Error(err), zap.String("taskId", task.ID))
			m.mu.Lock()
			task.Status = StatusError
			task.ErrorMessage = err.Error()
			m.mu.Unlock()
			return
		}
	}
}

// addToTransmission 添加任务到 Transmission
func (m *Manager) addToTransmission(task *DownloadTask) error {
	// 初始化客户端
	if m.transmissionClient == nil {
		m.transmissionClient = NewTransmissionClient(
			m.transmissionURL,
			m.transmissionUsername,
			m.transmissionPassword,
		)
	}

	// 添加种子
	hash, id, err := m.transmissionClient.AddTorrent(task.URL, task.DestPath)
	if err != nil {
		return fmt.Errorf("添加种子到 Transmission 失败: %w", err)
	}

	// 更新任务信息
	m.mu.Lock()
	task.DownloadID = hash
	task.ClientRef = fmt.Sprintf("transmission:%d", id)
	m.mu.Unlock()

	m.logger.Info("已添加种子到 Transmission",
		zap.String("taskId", task.ID),
		zap.String("hash", hash),
		zap.Int("transmissionId", id))

	return nil
}

// addToQbittorrent 添加任务到 qBittorrent
func (m *Manager) addToQbittorrent(task *DownloadTask) error {
	// 初始化客户端
	if m.qbittorrentClient == nil {
		m.qbittorrentClient = NewQBittorrentClient(
			m.qbittorrentURL,
			m.qbittorrentUsername,
			m.qbittorrentPassword,
		)
	}

	// 添加种子
	if err := m.qbittorrentClient.AddTorrent(task.URL, task.DestPath); err != nil {
		return fmt.Errorf("添加种子到 qBittorrent 失败: %w", err)
	}

	// qBittorrent 添加后需要查询获取 hash
	// 这里先标记为已添加，后续在 updateBittorrentTask 中更新
	m.mu.Lock()
	task.ClientRef = "qbittorrent:pending"
	m.mu.Unlock()

	m.logger.Info("已添加种子到 qBittorrent",
		zap.String("taskId", task.ID))

	return nil
}

// downloadWithAria2c 使用 aria2c 下载 BT
func (m *Manager) downloadWithAria2c(ctx context.Context, task *DownloadTask) error {
	args := []string{
		"--dir=" + task.DestPath,
		"--seed-time=0", // 下载完成后不做种
	}

	if task.SpeedLimit != nil && task.SpeedLimit.DownloadLimit > 0 {
		args = append(args, fmt.Sprintf("--max-download-limit=%dK", task.SpeedLimit.DownloadLimit))
	}

	args = append(args, task.URL)

	cmd := exec.CommandContext(ctx, "aria2c", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("aria2c 下载失败：%w, output: %s", err, string(output))
	}

	m.mu.Lock()
	task.Status = StatusCompleted
	task.Progress = 100
	completedTime := time.Now()
	task.CompletedAt = &completedTime
	m.mu.Unlock()

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
		if task.Status == StatusDownloading {
			switch task.Type {
			case TypeHTTP, TypeFTP:
				// HTTP/FTP 下载已在 StartTask 中启动，这里只更新进度
				// 实际进度由 downloadHTTP  goroutine 更新
			case TypeBT, TypeMagnet:
				// BT 下载 - 从 Transmission/qBittorrent 获取状态
				m.updateBittorrentTask(task, now)
			}
		}
	}

	// 定期保存
	_ = m.saveTasks()
}

// updateBittorrentTask 从 BT 客户端获取任务状态
func (m *Manager) updateBittorrentTask(task *DownloadTask, now time.Time) {
	if m.transmissionURL != "" {
		stats, err := m.getTransmissionStats(task.ID)
		if err == nil {
			task.Progress = stats.Progress
			task.Speed = stats.Speed
			task.Downloaded = stats.Downloaded
			task.Uploaded = stats.Uploaded
			task.Peers = stats.Peers
			task.Seeds = stats.Seeds
			task.UpdatedAt = now

			if stats.Progress >= 100 {
				task.Status = StatusSeeding
				completedTime := time.Now()
				task.CompletedAt = &completedTime
			}
		}
	} else if m.qbittorrentURL != "" {
		stats, err := m.getQbittorrentStats(task.ID)
		if err == nil {
			task.Progress = stats.Progress
			task.Speed = stats.Speed
			task.Downloaded = stats.Downloaded
			task.Uploaded = stats.Uploaded
			task.Peers = stats.Peers
			task.Seeds = stats.Seeds
			task.UpdatedAt = now

			if stats.Progress >= 100 {
				task.Status = StatusSeeding
				completedTime := time.Now()
				task.CompletedAt = &completedTime
			}
		}
	}
}

// downloadHTTP 执行 HTTP/FTP 下载
func (m *Manager) downloadHTTP(ctx context.Context, task *DownloadTask) {
	m.mu.Lock()
	task.Status = StatusDownloading
	task.UpdatedAt = time.Now()
	m.mu.Unlock()

	if m.onTaskUpdate != nil {
		m.onTaskUpdate(task)
	}

	// 创建目标目录
	if err := os.MkdirAll(task.DestPath, 0755); err != nil {
		m.logger.Error("创建下载目录失败", zap.Error(err), zap.String("taskId", task.ID))
		m.mu.Lock()
		task.Status = StatusError
		task.ErrorMessage = err.Error()
		m.mu.Unlock()
		return
	}

	// 创建文件
	filePath := filepath.Join(task.DestPath, task.Name)
	file, err := os.Create(filePath)
	if err != nil {
		m.logger.Error("创建文件失败", zap.Error(err), zap.String("taskId", task.ID))
		m.mu.Lock()
		task.Status = StatusError
		task.ErrorMessage = err.Error()
		m.mu.Unlock()
		return
	}
	defer func() { _ = file.Close() }()

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "GET", task.URL, nil)
	if err != nil {
		m.logger.Error("创建请求失败", zap.Error(err), zap.String("taskId", task.ID))
		m.mu.Lock()
		task.Status = StatusError
		task.ErrorMessage = err.Error()
		m.mu.Unlock()
		return
	}

	// 执行请求
	resp, err := safeHTTPClient.Do(req)
	if err != nil {
		m.logger.Error("下载失败", zap.Error(err), zap.String("taskId", task.ID))
		m.mu.Lock()
		task.Status = StatusError
		task.ErrorMessage = err.Error()
		m.mu.Unlock()
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("HTTP 错误：%s", resp.Status)
		m.logger.Error("下载失败", zap.Error(err), zap.String("taskId", task.ID))
		m.mu.Lock()
		task.Status = StatusError
		task.ErrorMessage = err.Error()
		m.mu.Unlock()
		return
	}

	// 获取文件大小
	task.TotalSize = resp.ContentLength

	// 下载文件
	var downloaded int64
	buffer := make([]byte, 32*1024)
	lastUpdate := time.Now()
	var lastDownloaded int64

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			_, writeErr := file.Write(buffer[:n])
			if writeErr != nil {
				m.logger.Error("写入文件失败", zap.Error(writeErr), zap.String("taskId", task.ID))
				m.mu.Lock()
				task.Status = StatusError
				task.ErrorMessage = writeErr.Error()
				m.mu.Unlock()
				return
			}
			downloaded += int64(n)
			task.Downloaded = downloaded

			// 计算进度和速度
			if task.TotalSize > 0 {
				task.Progress = float64(downloaded) / float64(task.TotalSize) * 100
			}

			// 每秒更新速度
			now := time.Now()
			if now.Sub(lastUpdate) >= time.Second {
				duration := now.Sub(lastUpdate).Seconds()
				task.Speed = int64(float64(downloaded-lastDownloaded) / duration)
				lastDownloaded = downloaded
				lastUpdate = now

				m.mu.Lock()
				task.UpdatedAt = now
				if m.onTaskUpdate != nil {
					m.onTaskUpdate(task)
				}
				m.mu.Unlock()
			}
		}

		if err != nil {
			if err == io.EOF {
				// 下载完成
				m.mu.Lock()
				task.Progress = 100
				task.Status = StatusCompleted
				completedTime := time.Now()
				task.CompletedAt = &completedTime
				task.UpdatedAt = completedTime
				if m.onTaskUpdate != nil {
					m.onTaskUpdate(task)
				}
				m.mu.Unlock()
				m.logger.Info("HTTP 下载完成", zap.String("taskId", task.ID), zap.String("name", task.Name))
			} else {
				m.logger.Error("下载出错", zap.Error(err), zap.String("taskId", task.ID))
				m.mu.Lock()
				task.Status = StatusError
				task.ErrorMessage = err.Error()
				m.mu.Unlock()
			}
			return
		}
	}
}

// TransmissionStats Transmission 统计信息
type TransmissionStats struct {
	Progress   float64
	Speed      int64
	Downloaded int64
	Uploaded   int64
	Peers      int
	Seeds      int
}

// getTransmissionStats 从 Transmission 获取统计信息
func (m *Manager) getTransmissionStats(taskID string) (*TransmissionStats, error) {
	if m.transmissionClient == nil {
		return nil, fmt.Errorf("transmission 客户端未初始化")
	}

	// 获取任务信息
	m.mu.RLock()
	task, exists := m.tasks[taskID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}

	// 通过 hash 查询种子
	if task.DownloadID == "" && task.ClientRef != "" {
		// 尝试从 ClientRef 解析 ID
		parts := strings.Split(task.ClientRef, ":")
		if len(parts) == 2 && parts[0] == "transmission" {
			var id int
			if _, err := fmt.Sscanf(parts[1], "%d", &id); err == nil {
				torrents, err := m.transmissionClient.GetTorrents(id)
				if err == nil && len(torrents) > 0 {
					m.mu.Lock()
					task.DownloadID = torrents[0].HashString
					m.mu.Unlock()
				}
			}
		}
	}

	// 获取种子信息
	torrents, err := m.transmissionClient.GetTorrents()
	if err != nil {
		return nil, err
	}

	// 查找对应的种子
	for _, t := range torrents {
		if t.HashString == task.DownloadID {
			return &TransmissionStats{
				Progress:   t.PercentDone * 100,
				Speed:      t.RateDownload,
				Downloaded: t.DownloadedEver,
				Uploaded:   t.UploadedEver,
				Peers:      t.PeersConnected,
				Seeds:      t.Seeders,
			}, nil
		}
	}

	return nil, fmt.Errorf("未找到对应的种子: %s", task.DownloadID)
}

// QbittorrentStats qBittorrent 统计信息
type QbittorrentStats struct {
	Progress   float64
	Speed      int64
	Downloaded int64
	Uploaded   int64
	Peers      int
	Seeds      int
}

// getQbittorrentStats 从 qBittorrent 获取统计信息
func (m *Manager) getQbittorrentStats(taskID string) (*QbittorrentStats, error) {
	if m.qbittorrentClient == nil {
		return nil, fmt.Errorf("qBittorrent 客户端未初始化")
	}

	// 获取任务信息
	m.mu.RLock()
	task, exists := m.tasks[taskID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}

	// 获取所有种子
	torrents, err := m.qbittorrentClient.GetTorrents()
	if err != nil {
		return nil, err
	}

	// 通过名字匹配（因为添加时可能还没有 hash）
	var torrent *QBittorrentTorrentInfo
	for i := range torrents {
		if task.DownloadID != "" && torrents[i].Hash == task.DownloadID {
			torrent = &torrents[i]
			break
		}
		// 尝试通过名字匹配
		if task.Name != "" && strings.Contains(torrents[i].Name, task.Name) {
			torrent = &torrents[i]
			// 更新 hash
			m.mu.Lock()
			task.DownloadID = torrents[i].Hash
			m.mu.Unlock()
			break
		}
	}

	if torrent == nil {
		return nil, fmt.Errorf("未找到对应的种子")
	}

	return &QbittorrentStats{
		Progress:   torrent.Progress * 100,
		Speed:      torrent.Dlspeed,
		Downloaded: torrent.Downloaded,
		Uploaded:   torrent.Uploaded,
		Peers:      torrent.NumLeechs,
		Seeds:      torrent.NumSeeds,
	}, nil
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
