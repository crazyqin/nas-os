package downloadstation

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 下载站管理器.
type Manager struct {
	mu          sync.RWMutex
	tasks       map[string]*DownloadTask
	taskURLs    map[string]string // URL -> taskID 映射，用于重复检测
	history     []HistoryEntry
	queue       *DownloadQueue
	limiter     *SpeedLimiter
	config      QueueConfig
	feeds       map[string]*RSSFeed
	feedItems   map[string][]RSSItem
	schedules   map[string]*DownloadSchedule
	ctx         context.Context
	cancel      context.CancelFunc
	downloadDir string
	autoClassify bool // 是否启用自动分类
}

// NewManager 创建下载站管理器.
func NewManager(config QueueConfig) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		tasks:        make(map[string]*DownloadTask),
		taskURLs:     make(map[string]string),
		history:      make([]HistoryEntry, 0),
		queue:        NewDownloadQueue(config),
		limiter:      NewSpeedLimiter(config.MaxSpeedTotal, config.MaxSpeedPerTask),
		config:       config,
		feeds:        make(map[string]*RSSFeed),
		feedItems:    make(map[string][]RSSItem),
		schedules:    make(map[string]*DownloadSchedule),
		ctx:          ctx,
		cancel:       cancel,
		downloadDir:  config.DownloadDir,
		autoClassify: true,
	}

	// 确保下载目录存在
	_ = os.MkdirAll(config.DownloadDir, 0755)

	return m
}

// CreateTask 创建下载任务.
func (m *Manager) CreateTask(req CreateTaskRequest) (*DownloadTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检测下载类型
	downloadType := req.Type
	if downloadType == "" {
		downloadType = detectDownloadType(req.URL)
	}

	// 生成任务 ID
	taskID := uuid.New().String()

	// 确定文件名
	fileName := req.Name
	if fileName == "" {
		fileName = generateFileName(req.URL, downloadType)
	}

	// 确定保存路径
	filePath := req.FilePath
	if filePath == "" {
		filePath = filepath.Join(m.downloadDir, fileName)
	}

	// 设置重试次数
	maxRetries := req.MaxRetries
	if maxRetries == 0 {
		maxRetries = m.config.MaxRetries
	}

	task := &DownloadTask{
		ID:           taskID,
		Name:         fileName,
		URL:          req.URL,
		Type:         downloadType,
		Status:       TaskStatusPending,
		FilePath:     filePath,
		FileName:     fileName,
		Priority:     req.Priority,
		MaxSpeed:     req.MaxSpeed,
		MaxRetries:   maxRetries,
		Checksum:     req.Checksum,
		ChecksumType: req.ChecksumType,
		TorrentPath:  req.TorrentPath,
		SeedTime:     req.SeedTime,
		SeedRatio:    req.SeedRatio,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	m.tasks[taskID] = task

	// 如果自动开始，加入队列
	if m.config.AutoStart {
		m.queue.Push(task)
	}

	return task, nil
}

// GetTask 获取下载任务.
func (m *Manager) GetTask(taskID string) (*DownloadTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	return task, nil
}

// ListTasks 列出所有下载任务.
func (m *Manager) ListTasks() []*DownloadTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*DownloadTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}

// UpdateTask 更新下载任务.
func (m *Manager) UpdateTask(taskID string, req UpdateTaskRequest) (*DownloadTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	if req.Priority != 0 {
		task.Priority = req.Priority
	}
	if req.MaxSpeed > 0 {
		task.MaxSpeed = req.MaxSpeed
	}
	if req.MaxRetries > 0 {
		task.MaxRetries = req.MaxRetries
	}

	task.UpdatedAt = time.Now()

	return task, nil
}

// DeleteTask 删除下载任务.
func (m *Manager) DeleteTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// 如果任务正在下载，先取消
	if task.Status == TaskStatusDownloading {
		task.Status = TaskStatusFailed
		task.ErrorMsg = "任务已取消"
		m.queue.CompleteTask(taskID)
	}

	// 从队列中移除
	m.queue.Remove(taskID)

	// 删除任务
	delete(m.tasks, taskID)

	return nil
}

// StartTask 开始下载任务.
func (m *Manager) StartTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.Status == TaskStatusDownloading {
		return fmt.Errorf("task is already downloading")
	}

	task.Status = TaskStatusPending
	task.UpdatedAt = time.Now()

	// 加入队列
	m.queue.Push(task)

	return nil
}

// PauseTask 暂停下载任务.
func (m *Manager) PauseTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.Status != TaskStatusDownloading {
		return fmt.Errorf("task is not downloading")
	}

	task.Status = TaskStatusPaused
	task.UpdatedAt = time.Now()

	// 从活跃任务中移除
	m.queue.CompleteTask(taskID)

	return nil
}

// ResumeTask 恢复下载任务.
func (m *Manager) ResumeTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.Status != TaskStatusPaused {
		return fmt.Errorf("task is not paused")
	}

	task.Status = TaskStatusPending
	task.UpdatedAt = time.Now()

	// 加入队列
	m.queue.Push(task)

	return nil
}

// CancelTask 取消下载任务.
func (m *Manager) CancelTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Status = TaskStatusFailed
	task.ErrorMsg = "任务已取消"
	task.UpdatedAt = time.Now()

	// 从队列中移除
	m.queue.Remove(taskID)

	return nil
}

// BatchStart 批量开始任务.
func (m *Manager) BatchStart(taskIDs []string) []error {
	m.mu.Lock()
	defer m.mu.Unlock()

	errors := make([]error, 0)
	for _, taskID := range taskIDs {
		task, ok := m.tasks[taskID]
		if !ok {
			errors = append(errors, fmt.Errorf("task not found: %s", taskID))
			continue
		}

		if task.Status == TaskStatusDownloading {
			continue
		}

		task.Status = TaskStatusPending
		task.UpdatedAt = time.Now()
		m.queue.Push(task)
	}

	return errors
}

// BatchPause 批量暂停任务.
func (m *Manager) BatchPause(taskIDs []string) []error {
	m.mu.Lock()
	defer m.mu.Unlock()

	errors := make([]error, 0)
	for _, taskID := range taskIDs {
		task, ok := m.tasks[taskID]
		if !ok {
			errors = append(errors, fmt.Errorf("task not found: %s", taskID))
			continue
		}

		if task.Status != TaskStatusDownloading {
			continue
		}

		task.Status = TaskStatusPaused
		task.UpdatedAt = time.Now()
		m.queue.CompleteTask(taskID)
	}

	return errors
}

// BatchDelete 批量删除任务.
func (m *Manager) BatchDelete(taskIDs []string) []error {
	m.mu.Lock()
	defer m.mu.Unlock()

	errors := make([]error, 0)
	for _, taskID := range taskIDs {
		task, ok := m.tasks[taskID]
		if !ok {
			errors = append(errors, fmt.Errorf("task not found: %s", taskID))
			continue
		}

		if task.Status == TaskStatusDownloading {
			task.Status = TaskStatusFailed
			task.ErrorMsg = "任务已取消"
			m.queue.CompleteTask(taskID)
		}

		m.queue.Remove(taskID)
		delete(m.tasks, taskID)
	}

	return errors
}

// GetStats 获取下载统计.
func (m *Manager) GetStats() DownloadStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := DownloadStats{
		TotalTasks: len(m.tasks),
	}

	var totalDownloaded int64
	var totalSize int64
	var totalTime int64

	for _, task := range m.tasks {
		switch task.Status {
		case TaskStatusDownloading:
			stats.ActiveTasks++
		case TaskStatusCompleted:
			stats.CompletedTasks++
		case TaskStatusFailed:
			stats.FailedTasks++
		}

		totalDownloaded += task.Downloaded
		totalSize += task.FileSize
		stats.CurrentSpeed += task.Speed
	}

	stats.TotalDownloaded = totalDownloaded
	stats.TotalSize = totalSize

	if stats.TotalTasks > 0 {
		stats.SuccessRate = float64(stats.CompletedTasks) / float64(stats.TotalTasks) * 100
	}

	// 计算平均速度
	if totalTime > 0 {
		stats.AverageSpeed = totalDownloaded / totalTime
	}

	return stats
}

// GetHistory 获取下载历史.
func (m *Manager) GetHistory() []HistoryEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := make([]HistoryEntry, len(m.history))
	copy(history, m.history)
	return history
}

// ClearHistory 清空下载历史.
func (m *Manager) ClearHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.history = make([]HistoryEntry, 0)
}

// ProcessQueue 处理下载队列.
func (m *Manager) ProcessQueue() {
	for {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		// 检查是否可以开始新任务
		if !m.queue.CanStart() {
			time.Sleep(time.Second)
			continue
		}

		// 从队列取出任务
		task := m.queue.Pop()
		if task == nil {
			time.Sleep(time.Second)
			continue
		}

		// 开始下载
		m.queue.StartTask(task)
		go m.executeDownload(task)
	}
}

// executeDownload 执行下载任务.
func (m *Manager) executeDownload(task *DownloadTask) {
	m.mu.Lock()
	task.Status = TaskStatusDownloading
	now := time.Now()
	task.StartedAt = &now
	task.UpdatedAt = now
	m.mu.Unlock()

	var err error

	switch task.Type {
	case DownloadTypeHTTP, DownloadTypeFTP:
		err = m.downloadHTTP(task)
	case DownloadTypeMagnet, DownloadTypeBT:
		// BT/磁力链接下载（模拟实现）
		err = m.downloadBT(task)
	default:
		err = fmt.Errorf("unsupported download type: %s", task.Type)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err != nil {
		task.Status = TaskStatusFailed
		task.ErrorMsg = err.Error()

		// 重试逻辑
		if task.RetryCount < task.MaxRetries {
			task.RetryCount++
			task.Status = TaskStatusPending
			m.queue.Push(task)
			return
		}
	} else {
		task.Status = TaskStatusCompleted
		completedAt := time.Now()
		task.CompletedAt = &completedAt
		task.Progress = 100

		// 校验文件
		if task.Checksum != "" {
			if err := m.verifyChecksum(task); err != nil {
				task.Status = TaskStatusFailed
				task.ErrorMsg = fmt.Sprintf("校验失败: %v", err)
			}
		}
	}

	task.UpdatedAt = time.Now()

	// 添加到历史记录
	m.addToHistory(task)

	// 从活跃任务中移除
	m.queue.CompleteTask(task.ID)
}

// downloadHTTP 下载 HTTP/FTP 文件.
func (m *Manager) downloadHTTP(task *DownloadTask) error {
	// 创建请求
	req, err := http.NewRequestWithContext(m.ctx, http.MethodGet, task.URL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 如果是续传，添加 Range 头
	if task.Downloaded > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", task.Downloaded))
	}

	// 发送请求
	client := &http.Client{
		Timeout: 30 * time.Minute,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("请求失败: %s", resp.Status)
	}

	// 获取文件大小
	if resp.ContentLength > 0 {
		task.FileSize = resp.ContentLength + task.Downloaded
	}

	// 创建目录
	dir := filepath.Dir(task.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 打开文件
	flags := os.O_CREATE | os.O_WRONLY
	if task.Downloaded > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	file, err := os.OpenFile(task.FilePath, flags, 0644)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	// 创建速度限制的 reader
	reader := m.createLimitedReader(resp.Body, task)

	// 复制数据
	buf := make([]byte, 32*1024)
	startTime := time.Now()
	lastUpdate := time.Now()

	for {
		select {
		case <-m.ctx.Done():
			return fmt.Errorf("下载已取消")
		default:
		}

		n, err := reader.Read(buf)
		if n > 0 {
			_, writeErr := file.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("写入文件失败: %w", writeErr)
			}

			task.Downloaded += int64(n)

			// 更新进度
			if task.FileSize > 0 {
				task.SetProgress(float64(task.Downloaded) / float64(task.FileSize) * 100)
			}

			// 更新速度（每秒更新一次）
			if time.Since(lastUpdate) >= time.Second {
				elapsed := time.Since(startTime).Seconds()
				if elapsed > 0 {
					task.SetSpeed(int64(float64(task.Downloaded) / elapsed))
				}
				lastUpdate = time.Now()
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("读取数据失败: %w", err)
		}
	}

	return nil
}

// downloadBT 下载 BT/磁力链接.
func (m *Manager) downloadBT(task *DownloadTask) error {
	// 模拟 BT 下载
	// 实际实现需要集成 BT 客户端（如 libtorrent）
	totalSize := int64(1024 * 1024 * 100) // 模拟 100MB
	task.FileSize = totalSize

	chunkSize := totalSize / 100
	for i := 0; i < 100; i++ {
		select {
		case <-m.ctx.Done():
			return fmt.Errorf("下载已取消")
		default:
		}

		// 模拟下载进度
		time.Sleep(100 * time.Millisecond)
		task.Downloaded += chunkSize
		task.SetProgress(float64(i + 1))
		task.SetSpeed(chunkSize * 10) // 模拟速度

		// 模拟做种
		if i > 50 {
			task.Peers = 10
			task.Seeds = 5
		}
	}

	// 创建模拟文件
	dir := filepath.Dir(task.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(task.FilePath, []byte("simulated bt content"), 0644)
}

// createLimitedReader 创建速度限制的 reader.
func (m *Manager) createLimitedReader(reader io.Reader, task *DownloadTask) io.Reader {
	// 如果有单任务速度限制，使用单任务限制
	// 否则使用总速度限制
	limit := task.MaxSpeed
	if limit <= 0 {
		limit = m.config.MaxSpeedPerTask
	}

	if limit <= 0 {
		return reader
	}

	return &limitedReader{
		reader:  reader,
		limiter: m.limiter,
		limit:   limit,
	}
}

// limitedReader 速度限制的 reader.
type limitedReader struct {
	reader  io.Reader
	limiter *SpeedLimiter
	limit   int64
}

func (r *limitedReader) Read(p []byte) (n int, err error) {
	// 限制每次读取的大小
	maxLen := int(r.limit / 10) // 每 100ms 的数据量
	if maxLen <= 0 {
		maxLen = 1024
	}
	if len(p) > maxLen {
		p = p[:maxLen]
	}

	// 等待令牌桶
	r.limiter.Wait(int64(len(p)))

	return r.reader.Read(p)
}

// verifyChecksum 校验文件.
func (m *Manager) verifyChecksum(task *DownloadTask) error {
	file, err := os.Open(task.FilePath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	var hash string
	switch task.ChecksumType {
	case ChecksumMD5:
		h := md5.New()
		if _, err := io.Copy(h, file); err != nil {
			return err
		}
		hash = hex.EncodeToString(h.Sum(nil))
	case ChecksumSHA256:
		h := sha256.New()
		if _, err := io.Copy(h, file); err != nil {
			return err
		}
		hash = hex.EncodeToString(h.Sum(nil))
	default:
		return fmt.Errorf("不支持的校验类型: %s", task.ChecksumType)
	}

	if hash != task.Checksum {
		return fmt.Errorf("校验值不匹配: 期望 %s, 实际 %s", task.Checksum, hash)
	}

	return nil
}

// addToHistory 添加到历史记录.
func (m *Manager) addToHistory(task *DownloadTask) {
	entry := HistoryEntry{
		ID:           uuid.New().String(),
		TaskID:       task.ID,
		Name:         task.Name,
		URL:          task.URL,
		Type:         task.Type,
		FilePath:     task.FilePath,
		FileSize:     task.FileSize,
		Checksum:     task.Checksum,
		ChecksumType: task.ChecksumType,
		Status:       task.Status,
		ErrorMsg:     task.ErrorMsg,
		StartedAt:    *task.StartedAt,
		CompletedAt:  time.Now(),
	}

	if task.StartedAt != nil {
		entry.Duration = int64(time.Since(*task.StartedAt).Seconds())
	}

	if entry.Duration > 0 {
		entry.AverageSpeed = task.Downloaded / entry.Duration
	}

	m.history = append(m.history, entry)
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(config QueueConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	m.queue.UpdateConfig(config)
	m.limiter.UpdateLimits(config.MaxSpeedTotal, config.MaxSpeedPerTask)
	m.downloadDir = config.DownloadDir
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() QueueConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	m.cancel()
	m.queue.Stop()
}

// detectDownloadType 检测下载类型.
func detectDownloadType(rawURL string) DownloadType {
	u, err := url.Parse(rawURL)
	if err != nil {
		return DownloadTypeHTTP
	}

	switch {
	case strings.HasPrefix(u.Scheme, "magnet"):
		return DownloadTypeMagnet
	case strings.HasSuffix(strings.ToLower(u.Path), ".torrent"):
		return DownloadTypeBT
	case strings.HasPrefix(u.Scheme, "ftp"):
		return DownloadTypeFTP
	case strings.HasPrefix(u.Scheme, "http"):
		return DownloadTypeHTTP
	default:
		return DownloadTypeHTTP
	}
}

// generateFileName 生成文件名.
func generateFileName(rawURL string, downloadType DownloadType) string {
	switch downloadType {
	case DownloadTypeMagnet:
		// 从磁力链接提取名称
		if name := extractMagnetName(rawURL); name != "" {
			return name
		}
		return fmt.Sprintf("magnet_%d", time.Now().Unix())
	case DownloadTypeBT:
		// 从 URL 提取种子文件名
		u, err := url.Parse(rawURL)
		if err == nil {
			parts := strings.Split(u.Path, "/")
			if len(parts) > 0 {
				return parts[len(parts)-1]
			}
		}
		return fmt.Sprintf("torrent_%d", time.Now().Unix())
	default:
		// 从 URL 提取文件名
		u, err := url.Parse(rawURL)
		if err == nil {
			parts := strings.Split(u.Path, "/")
			if len(parts) > 0 {
				name := parts[len(parts)-1]
				if name != "" {
					return name
				}
			}
		}
		return fmt.Sprintf("download_%d", time.Now().Unix())
	}
}

// extractMagnetName 从磁力链接提取名称.
func extractMagnetName(magnetURI string) string {
	// 解析磁力链接中的 dn 参数
	re := regexp.MustCompile(`dn=([^&]+)`)
	matches := re.FindStringSubmatch(magnetURI)
	if len(matches) > 1 {
		name, _ := url.QueryUnescape(matches[1])
		return name
	}
	return ""
}

// CalculateChecksum 计算文件校验值.
func CalculateChecksum(filePath string, checksumType ChecksumType) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	var hashBytes []byte
	switch checksumType {
	case ChecksumMD5:
		h := md5.New()
		if _, err := io.Copy(h, file); err != nil {
			return "", err
		}
		hashBytes = h.Sum(nil)
	case ChecksumSHA256:
		h := sha256.New()
		if _, err := io.Copy(h, file); err != nil {
			return "", err
		}
		hashBytes = h.Sum(nil)
	default:
		return "", fmt.Errorf("不支持的校验类型: %s", checksumType)
	}

	return hex.EncodeToString(hashBytes), nil
}
