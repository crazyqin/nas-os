// Package smartdownloader 提供智能下载管理系统
// 支持 HTTP/HTTPS/FTP/BT/PT 下载，智能队列管理，限速调度，下载通知
package smartdownloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ========== 常量 ==========

const (
	// Version 模块版本
	Version = "1.0.0"

	// MaxConcurrentDownloads 最大并发下载数
	MaxConcurrentDownloads = 10

	// DefaultChunkSize 默认分块大小 (1MB)
	DefaultChunkSize = 1024 * 1024

	// MaxRetries 最大重试次数
	MaxRetries = 3

	// RetryDelay 重试延迟
	RetryDelay = 5 * time.Second

	// ProgressUpdateInterval 进度更新间隔
	ProgressUpdateInterval = 500 * time.Millisecond

	// DownloadTimeout 下载超时时间
	DownloadTimeout = 30 * time.Minute
)

// ========== 下载协议 ==========

// DownloadProtocol 下载协议类型
type DownloadProtocol string

const (
	ProtocolHTTP   DownloadProtocol = "http"
	ProtocolHTTPS  DownloadProtocol = "https"
	ProtocolFTP    DownloadProtocol = "ftp"
	ProtocolBT     DownloadProtocol = "bt"
	ProtocolPT     DownloadProtocol = "pt"
	ProtocolMagnet DownloadProtocol = "magnet"
)

// ========== 下载状态 ==========

// DownloadStatus 下载状态
type DownloadStatus string

const (
	StatusPending     DownloadStatus = "pending"
	StatusQueued      DownloadStatus = "queued"
	StatusConnecting  DownloadStatus = "connecting"
	StatusDownloading DownloadStatus = "downloading"
	StatusPaused      DownloadStatus = "paused"
	StatusCompleted   DownloadStatus = "completed"
	StatusFailed      DownloadStatus = "failed"
	StatusCancelled   DownloadStatus = "cancelled"
	StatusSeeding     DownloadStatus = "seeding"
)

// ========== 下载优先级 ==========

// DownloadPriority 下载优先级
type DownloadPriority int

const (
	PriorityLow    DownloadPriority = 0
	PriorityNormal DownloadPriority = 1
	PriorityHigh   DownloadPriority = 2
	PriorityUrgent DownloadPriority = 3
)

// ========== 通知方式 ==========

// NotifyMethod 通知方式
type NotifyMethod string

const (
	NotifyWebhook  NotifyMethod = "webhook"
	NotifyEmail    NotifyMethod = "email"
	NotifyTelegram NotifyMethod = "telegram"
)

// ========== 数据结构 ==========

// DownloadItem 下载项
type DownloadItem struct {
	ID             string            `json:"id"`
	URL            string            `json:"url"`
	Protocol       DownloadProtocol  `json:"protocol"`
	FileName       string            `json:"file_name"`
	FilePath       string            `json:"file_path"`
	FileSize       int64             `json:"file_size"`
	DownloadedSize int64             `json:"downloaded_size"`
	Status         DownloadStatus    `json:"status"`
	Priority       DownloadPriority  `json:"priority"`
	Progress       float64           `json:"progress"`
	Speed          int64             `json:"speed"`        // bytes per second
	ETA            time.Duration     `json:"eta"`          // estimated time remaining
	Segments       int               `json:"segments"`     // 分段数
	SegmentSize    int64             `json:"segment_size"` // 分段大小
	Headers        map[string]string `json:"headers,omitempty"`
	Cookies        map[string]string `json:"cookies,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	StartedAt      *time.Time        `json:"started_at,omitempty"`
	CompletedAt    *time.Time        `json:"completed_at,omitempty"`
	Error          string            `json:"error,omitempty"`
	RetryCount     int               `json:"retry_count"`
	Checksum       string            `json:"checksum,omitempty"`
	SpeedLimit     int64             `json:"speed_limit"`            // bytes per second, 0=unlimited
	ScheduledAt    *time.Time        `json:"scheduled_at,omitempty"` // 定时下载
	Metadata       DownloadMetadata  `json:"metadata"`
}

// DownloadMetadata 下载元数据
type DownloadMetadata struct {
	Source    string            `json:"source,omitempty"`
	Category  string            `json:"category,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	Referer   string            `json:"referer,omitempty"`
	UserAgent string            `json:"user_agent,omitempty"`
	Extra     map[string]string `json:"extra,omitempty"`
}

// BTDownloadInfo BT下载信息
type BTDownloadInfo struct {
	TorrentPath   string   `json:"torrent_path,omitempty"`
	MagnetURI     string   `json:"magnet_uri,omitempty"`
	InfoHash      string   `json:"info_hash"`
	Name          string   `json:"name"`
	TotalSize     int64    `json:"total_size"`
	Downloaded    int64    `json:"downloaded"`
	Uploaded      int64    `json:"uploaded"`
	DownloadSpeed int64    `json:"download_speed"`
	UploadSpeed   int64    `json:"upload_speed"`
	Seeders       int      `json:"seeders"`
	Leechers      int      `json:"leechers"`
	Ratio         float64  `json:"ratio"`
	Progress      float64  `json:"progress"`
	Status        string   `json:"status"`
	Files         []BTFile `json:"files,omitempty"`
}

// BTFile BT文件信息
type BTFile struct {
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Progress float64 `json:"progress"`
}

// TransmissionConfig Transmission配置
type TransmissionConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	RPCPath  string `json:"rpc_path"`
}

// SpeedLimitConfig 限速配置
type SpeedLimitConfig struct {
	GlobalDownload int64  `json:"global_download"` // 全局下载限速 bytes/s
	GlobalUpload   int64  `json:"global_upload"`   // 全局上传限速 bytes/s
	PerTask        int64  `json:"per_task"`        // 单任务限速 bytes/s
	ScheduleStart  string `json:"schedule_start"`  // 限速时段开始 HH:MM
	ScheduleEnd    string `json:"schedule_end"`    // 限速时段结束 HH:MM
	ScheduleLimit  int64  `json:"schedule_limit"`  // 限速时段限速 bytes/s
}

// ScheduleConfig 调度配置
type ScheduleConfig struct {
	MaxConcurrent  int    `json:"max_concurrent"`   // 最大并发数
	QueueSize      int    `json:"queue_size"`       // 队列大小
	SortBy         string `json:"sort_by"`          // 排序方式: priority, created_at, size
	RetryOnFailure bool   `json:"retry_on_failure"` // 失败重试
	AutoStart      bool   `json:"auto_start"`       // 自动开始
	DownloadPath   string `json:"download_path"`    // 默认下载路径
	TempPath       string `json:"temp_path"`        // 临时文件路径
}

// NotifyConfig 通知配置
type NotifyConfig struct {
	Enabled  bool           `json:"enabled"`
	Methods  []NotifyMethod `json:"methods"`
	Webhook  WebhookConfig  `json:"webhook"`
	Email    EmailConfig    `json:"email"`
	Telegram TelegramConfig `json:"telegram"`
}

// WebhookConfig Webhook配置
type WebhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
}

// EmailConfig 邮件配置
type EmailConfig struct {
	SMTPHost string   `json:"smtp_host"`
	SMTPPort int      `json:"smtp_port"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	From     string   `json:"from"`
	To       []string `json:"to"`
}

// TelegramConfig Telegram配置
type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

// DownloadRequest 下载请求
type DownloadRequest struct {
	URL         string            `json:"url"`
	Protocol    DownloadProtocol  `json:"protocol"`
	FileName    string            `json:"file_name,omitempty"`
	FilePath    string            `json:"file_path,omitempty"`
	Priority    DownloadPriority  `json:"priority"`
	Segments    int               `json:"segments,omitempty"`
	SpeedLimit  int64             `json:"speed_limit,omitempty"`
	ScheduledAt *time.Time        `json:"scheduled_at,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Cookies     map[string]string `json:"cookies,omitempty"`
	Metadata    DownloadMetadata  `json:"metadata,omitempty"`
}

// DownloadStats 下载统计
type DownloadStats struct {
	TotalDownloads     int                      `json:"total_downloads"`
	ActiveDownloads    int                      `json:"active_downloads"`
	CompletedDownloads int                      `json:"completed_downloads"`
	FailedDownloads    int                      `json:"failed_downloads"`
	TotalSize          int64                    `json:"total_size"`
	DownloadedSize     int64                    `json:"downloaded_size"`
	AverageSpeed       int64                    `json:"average_speed"`
	CurrentSpeed       int64                    `json:"current_speed"`
	ProtocolStats      map[DownloadProtocol]int `json:"protocol_stats"`
}

// DownloadHistory 下载历史
type DownloadHistory struct {
	ID           string           `json:"id"`
	URL          string           `json:"url"`
	Protocol     DownloadProtocol `json:"protocol"`
	FileName     string           `json:"file_name"`
	FilePath     string           `json:"file_path"`
	FileSize     int64            `json:"file_size"`
	Status       DownloadStatus   `json:"status"`
	StartedAt    time.Time        `json:"started_at"`
	CompletedAt  *time.Time       `json:"completed_at,omitempty"`
	Duration     time.Duration    `json:"duration"`
	AverageSpeed int64            `json:"average_speed"`
	Error        string           `json:"error,omitempty"`
}

// ========== 管理器 ==========

// DownloadManager 下载管理器
type DownloadManager struct {
	mu             sync.RWMutex
	downloads      map[string]*DownloadItem
	queue          []*DownloadItem
	history        []DownloadHistory
	stats          DownloadStats
	scheduleConfig ScheduleConfig
	speedLimit     SpeedLimitConfig
	notifyConfig   NotifyConfig
	transmission   *TransmissionConfig
	running        bool
	cancelFunc     context.CancelFunc
	workerPool     chan struct{}
	activeWorkers  int
}

// NewDownloadManager 创建下载管理器
func NewDownloadManager(scheduleConfig ScheduleConfig, speedLimit SpeedLimitConfig, notifyConfig NotifyConfig) *DownloadManager {
	if scheduleConfig.MaxConcurrent <= 0 {
		scheduleConfig.MaxConcurrent = MaxConcurrentDownloads
	}
	if scheduleConfig.QueueSize <= 0 {
		scheduleConfig.QueueSize = 1000
	}
	if scheduleConfig.DownloadPath == "" {
		scheduleConfig.DownloadPath = filepath.Join(os.Getenv("HOME"), "Downloads")
	}
	if scheduleConfig.TempPath == "" {
		scheduleConfig.TempPath = filepath.Join(os.TempDir(), "smartdownloader")
	}

	dm := &DownloadManager{
		downloads:      make(map[string]*DownloadItem),
		queue:          make([]*DownloadItem, 0),
		history:        make([]DownloadHistory, 0),
		scheduleConfig: scheduleConfig,
		speedLimit:     speedLimit,
		notifyConfig:   notifyConfig,
		workerPool:     make(chan struct{}, scheduleConfig.MaxConcurrent),
	}

	// 确保目录存在
	os.MkdirAll(scheduleConfig.DownloadPath, 0755)
	os.MkdirAll(scheduleConfig.TempPath, 0755)

	return dm
}

// SetTransmissionConfig 设置Transmission配置
func (dm *DownloadManager) SetTransmissionConfig(config *TransmissionConfig) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.transmission = config
}

// Start 启动下载管理器
func (dm *DownloadManager) Start(ctx context.Context) error {
	dm.mu.Lock()
	if dm.running {
		dm.mu.Unlock()
		return errors.New("download manager already running")
	}
	dm.running = true
	ctx, dm.cancelFunc = context.WithCancel(ctx)
	dm.mu.Unlock()

	go dm.processQueue(ctx)
	go dm.monitorDownloads(ctx)
	go dm.scheduleChecker(ctx)

	return nil
}

// Stop 停止下载管理器
func (dm *DownloadManager) Stop() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.cancelFunc != nil {
		dm.cancelFunc()
	}
	dm.running = false
}

// IsRunning 是否运行中
func (dm *DownloadManager) IsRunning() bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.running
}

// AddDownload 添加下载任务
func (dm *DownloadManager) AddDownload(req DownloadRequest) (*DownloadItem, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 检测协议
	protocol := req.Protocol
	if protocol == "" {
		protocol = dm.detectProtocol(req.URL)
	}

	// 验证URL
	if err := dm.validateURL(req.URL, protocol); err != nil {
		return nil, err
	}

	// 处理BT/PT下载
	if protocol == ProtocolBT || protocol == ProtocolPT || protocol == ProtocolMagnet {
		return dm.addBTDownload(req)
	}

	// 创建下载项
	item := &DownloadItem{
		ID:          generateID(),
		URL:         req.URL,
		Protocol:    protocol,
		FileName:    req.FileName,
		FilePath:    req.FilePath,
		Status:      StatusPending,
		Priority:    req.Priority,
		Segments:    req.Segments,
		Headers:     req.Headers,
		Cookies:     req.Cookies,
		SpeedLimit:  req.SpeedLimit,
		ScheduledAt: req.ScheduledAt,
		Metadata:    req.Metadata,
		CreatedAt:   time.Now(),
	}

	// 设置默认文件名
	if item.FileName == "" {
		item.FileName = dm.extractFileName(req.URL)
	}

	// 设置默认路径
	if item.FilePath == "" {
		item.FilePath = dm.scheduleConfig.DownloadPath
	}

	// 设置默认分段数
	if item.Segments <= 0 {
		item.Segments = 4
	}

	dm.downloads[item.ID] = item

	// 如果是定时下载，不加入队列
	if item.ScheduledAt != nil && item.ScheduledAt.After(time.Now()) {
		return item, nil
	}

	// 加入队列
	dm.enqueue(item)

	return item, nil
}

// addBTDownload 添加BT/PT下载
func (dm *DownloadManager) addBTDownload(req DownloadRequest) (*DownloadItem, error) {
	if dm.transmission == nil {
		return nil, errors.New("transmission not configured")
	}

	item := &DownloadItem{
		ID:        generateID(),
		URL:       req.URL,
		Protocol:  req.Protocol,
		Status:    StatusPending,
		Priority:  req.Priority,
		Headers:   req.Headers,
		Cookies:   req.Cookies,
		Metadata:  req.Metadata,
		CreatedAt: time.Now(),
	}

	// 对于BT下载，URL可能是magnet链接或torrent文件路径
	if req.Protocol == ProtocolMagnet || strings.HasPrefix(req.URL, "magnet:") {
		item.Protocol = ProtocolMagnet
	}

	dm.downloads[item.ID] = item

	// 调用Transmission API添加下载
	go dm.startBTDownload(item)

	return item, nil
}

// startBTDownload 启动BT下载
func (dm *DownloadManager) startBTDownload(item *DownloadItem) {
	dm.mu.Lock()
	item.Status = StatusConnecting
	now := time.Now()
	item.StartedAt = &now
	dm.mu.Unlock()

	// 模拟Transmission API调用
	// 实际实现中应使用Transmission RPC客户端
	err := dm.transmissionAddTorrent(item.URL)
	if err != nil {
		dm.mu.Lock()
		item.Status = StatusFailed
		item.Error = err.Error()
		dm.mu.Unlock()
		dm.addToHistory(item)
		return
	}

	dm.mu.Lock()
	item.Status = StatusDownloading
	dm.mu.Unlock()
}

// transmissionAddTorrent 添加种子到Transmission
func (dm *DownloadManager) transmissionAddTorrent(uri string) error {
	if dm.transmission == nil {
		return errors.New("transmission not configured")
	}

	// 构建RPC请求
	rpcURL := fmt.Sprintf("http://%s:%d%s",
		dm.transmission.Host,
		dm.transmission.Port,
		dm.transmission.RPCPath,
	)

	payload := map[string]interface{}{
		"method": "torrent-add",
		"arguments": map[string]interface{}{
			"filename": uri,
		},
	}

	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", rpcURL, strings.NewReader(string(jsonData)))
	req.Header.Set("Content-Type", "application/json")
	if dm.transmission.Username != "" {
		req.SetBasicAuth(dm.transmission.Username, dm.transmission.Password)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("transmission RPC error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("transmission RPC returned status %d", resp.StatusCode)
	}

	return nil
}

// enqueue 入队
func (dm *DownloadManager) enqueue(item *DownloadItem) {
	item.Status = StatusQueued
	dm.queue = append(dm.queue, item)
	dm.sortQueue()
}

// sortQueue 排序队列
func (dm *DownloadManager) sortQueue() {
	sort.Slice(dm.queue, func(i, j int) bool {
		// 按优先级排序（高优先）
		if dm.queue[i].Priority != dm.queue[j].Priority {
			return dm.queue[i].Priority > dm.queue[j].Priority
		}
		// 按创建时间排序（早优先）
		return dm.queue[i].CreatedAt.Before(dm.queue[j].CreatedAt)
	})
}

// processQueue 处理下载队列
func (dm *DownloadManager) processQueue(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dm.processNext()
		}
	}
}

// processNext 处理下一个下载任务
func (dm *DownloadManager) processNext() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if len(dm.queue) == 0 {
		return
	}

	// 检查是否可以启动新任务
	select {
	case dm.workerPool <- struct{}{}:
		// 可以启动
	default:
		// 已达最大并发
		return
	}

	// 取出最高优先级任务
	item := dm.queue[0]
	dm.queue = dm.queue[1:]
	dm.activeWorkers++

	item.Status = StatusDownloading
	now := time.Now()
	item.StartedAt = &now

	go dm.executeDownload(item)
}

// executeDownload 执行下载
func (dm *DownloadManager) executeDownload(item *DownloadItem) {
	defer func() {
		<-dm.workerPool
		dm.mu.Lock()
		dm.activeWorkers--
		dm.mu.Unlock()
	}()

	// 重试逻辑
	for retry := 0; retry <= MaxRetries; retry++ {
		item.RetryCount = retry

		err := dm.doDownload(item)
		if err == nil {
			// 下载成功
			dm.mu.Lock()
			item.Status = StatusCompleted
			now := time.Now()
			item.CompletedAt = &now
			item.Progress = 100
			dm.mu.Unlock()

			// 添加到历史
			dm.addToHistory(item)

			// 发送通知
			dm.sendNotification(item)
			return
		}

		// 检查是否被取消
		if item.Status == StatusCancelled {
			return
		}

		// 重试
		if retry < MaxRetries {
			time.Sleep(RetryDelay)
		}
	}

	// 所有重试失败
	dm.mu.Lock()
	item.Status = StatusFailed
	item.Error = "max retries exceeded"
	dm.mu.Unlock()

	dm.addToHistory(item)
	dm.sendNotification(item)
}

// doDownload 执行实际下载
func (dm *DownloadManager) doDownload(item *DownloadItem) error {
	ctx, cancel := context.WithTimeout(context.Background(), DownloadTimeout)
	defer cancel()

	// 获取文件大小
	req, _ := http.NewRequestWithContext(ctx, "HEAD", item.URL, nil)
	for k, v := range item.Headers {
		req.Header.Set(k, v)
	}
	if item.Metadata.UserAgent != "" {
		req.Header.Set("User-Agent", item.Metadata.UserAgent)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	defer resp.Body.Close()
	item.FileSize = resp.ContentLength

	// 多线程下载
	if item.FileSize > 0 && item.Segments > 1 {
		return dm.multiThreadDownload(ctx, item)
	}

	// 单线程下载
	return dm.singleThreadDownload(ctx, item)
}

// singleThreadDownload 单线程下载
func (dm *DownloadManager) singleThreadDownload(ctx context.Context, item *DownloadItem) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", item.URL, nil)
	for k, v := range item.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// 创建输出文件
	outPath := filepath.Join(item.FilePath, item.FileName)
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// 带限速的读取
	var reader io.Reader = resp.Body
	if item.SpeedLimit > 0 || dm.speedLimit.PerTask > 0 {
		limit := item.SpeedLimit
		if limit == 0 {
			limit = dm.speedLimit.PerTask
		}
		reader = &rateLimitedReader{
			reader:    resp.Body,
			rateLimit: limit,
		}
	}

	// 写入并更新进度
	buf := make([]byte, 32*1024)
	startTime := time.Now()
	lastUpdate := time.Now()
	var lastBytes int64

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if _, writeErr := outFile.Write(buf[:n]); writeErr != nil {
				return writeErr
			}

			item.DownloadedSize += int64(n)

			// 更新进度
			now := time.Now()
			if now.Sub(lastUpdate) >= ProgressUpdateInterval {
				elapsed := now.Sub(startTime).Seconds()
				if elapsed > 0 {
					item.Speed = int64(float64(item.DownloadedSize) / elapsed)
				}
				if item.FileSize > 0 {
					item.Progress = float64(item.DownloadedSize) / float64(item.FileSize) * 100
					if item.Speed > 0 {
						item.ETA = time.Duration((item.FileSize-item.DownloadedSize)/item.Speed) * time.Second
					}
				}
				lastUpdate = now
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	_ = lastBytes
	return nil
}

// multiThreadDownload 多线程下载
func (dm *DownloadManager) multiThreadDownload(ctx context.Context, item *DownloadItem) error {
	segmentSize := item.FileSize / int64(item.Segments)
	var wg sync.WaitGroup
	errCh := make(chan error, item.Segments)

	// 创建临时文件
	tmpPath := filepath.Join(dm.scheduleConfig.TempPath, item.ID+".tmp")
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer tmpFile.Close()

	// 预分配空间
	tmpFile.Truncate(item.FileSize)

	for i := 0; i < item.Segments; i++ {
		wg.Add(1)
		start := int64(i) * segmentSize
		end := start + segmentSize - 1
		if i == item.Segments-1 {
			end = item.FileSize - 1
		}

		go func(segment int, start, end int64) {
			defer wg.Done()
			err := dm.downloadSegment(ctx, item, tmpFile, start, end)
			if err != nil {
				errCh <- err
			}
		}(i, start, end)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	// 移动到最终位置
	outPath := filepath.Join(item.FilePath, item.FileName)
	return os.Rename(tmpPath, outPath)
}

// downloadSegment 下载分段
func (dm *DownloadManager) downloadSegment(ctx context.Context, item *DownloadItem, file *os.File, start, end int64) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", item.URL, nil)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	for k, v := range item.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// 带限速的读取
	var reader io.Reader = resp.Body
	if item.SpeedLimit > 0 {
		perSegment := item.SpeedLimit / int64(item.Segments)
		reader = &rateLimitedReader{
			reader:    resp.Body,
			rateLimit: perSegment,
		}
	}

	buf := make([]byte, 32*1024)
	offset := start

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			file.WriteAt(buf[:n], offset)
			offset += int64(n)

			dm.mu.Lock()
			item.DownloadedSize += int64(n)
			if item.FileSize > 0 {
				item.Progress = float64(item.DownloadedSize) / float64(item.FileSize) * 100
			}
			dm.mu.Unlock()
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	return nil
}

// rateLimitedReader 限速读取器
type rateLimitedReader struct {
	reader    io.Reader
	rateLimit int64 // bytes per second
	bytes     int64
	start     time.Time
}

func (r *rateLimitedReader) Read(p []byte) (int, error) {
	if r.start.IsZero() {
		r.start = time.Now()
	}

	// 计算允许读取的字节数
	elapsed := time.Since(r.start).Seconds()
	expectedBytes := int64(elapsed * float64(r.rateLimit))
	waitBytes := r.bytes - expectedBytes

	if waitBytes > 0 {
		// 需要等待
		waitTime := time.Duration(float64(waitBytes) / float64(r.rateLimit) * float64(time.Second))
		time.Sleep(waitTime)
	}

	// 限制每次读取量
	maxRead := int(r.rateLimit / 10) // 每次最多读取1/10秒的量
	if maxRead < 1024 {
		maxRead = 1024
	}
	if len(p) > maxRead {
		p = p[:maxRead]
	}

	n, err := r.reader.Read(p)
	r.bytes += int64(n)
	return n, err
}

// monitorDownloads 监控下载状态
func (dm *DownloadManager) monitorDownloads(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dm.updateStats()
			dm.checkBTDownloads()
		}
	}
}

// checkBTDownloads 检查BT下载状态
func (dm *DownloadManager) checkBTDownloads() {
	if dm.transmission == nil {
		return
	}

	dm.mu.RLock()
	var btItems []*DownloadItem
	for _, item := range dm.downloads {
		if item.Protocol == ProtocolBT || item.Protocol == ProtocolPT || item.Protocol == ProtocolMagnet {
			if item.Status == StatusDownloading || item.Status == StatusConnecting {
				btItems = append(btItems, item)
			}
		}
	}
	dm.mu.RUnlock()

	for _, item := range btItems {
		info, err := dm.getBTStatus(item.URL)
		if err != nil {
			continue
		}

		dm.mu.Lock()
		item.DownloadedSize = info.Downloaded
		item.FileSize = info.TotalSize
		item.Progress = info.Progress
		item.Speed = info.DownloadSpeed

		if info.Progress >= 100 {
			item.Status = StatusCompleted
			now := time.Now()
			item.CompletedAt = &now
			dm.addToHistory(item)
			dm.sendNotification(item)
		}
		dm.mu.Unlock()
	}
}

// getBTStatus 获取BT下载状态
func (dm *DownloadManager) getBTStatus(hash string) (*BTDownloadInfo, error) {
	if dm.transmission == nil {
		return nil, errors.New("transmission not configured")
	}

	// 构建RPC请求
	rpcURL := fmt.Sprintf("http://%s:%d%s",
		dm.transmission.Host,
		dm.transmission.Port,
		dm.transmission.RPCPath,
	)

	payload := map[string]interface{}{
		"method": "torrent-get",
		"arguments": map[string]interface{}{
			"fields": []string{
				"hashString", "name", "totalSize", "downloadedEver",
				"uploadedEver", "rateDownload", "rateUpload",
				"peersSendingToUs", "peersGettingFromUs",
				"uploadRatio", "percentDone", "status", "files",
			},
		},
	}

	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", rpcURL, strings.NewReader(string(jsonData)))
	req.Header.Set("Content-Type", "application/json")
	if dm.transmission.Username != "" {
		req.SetBasicAuth(dm.transmission.Username, dm.transmission.Password)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Arguments struct {
			Torrents []BTDownloadInfo `json:"torrents"`
		} `json:"arguments"`
	}

	json.NewDecoder(resp.Body).Decode(&result)

	for _, torrent := range result.Arguments.Torrents {
		if torrent.InfoHash == hash {
			return &torrent, nil
		}
	}

	return nil, errors.New("torrent not found")
}

// scheduleChecker 调度检查器
func (dm *DownloadManager) scheduleChecker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dm.checkScheduledDownloads()
		}
	}
}

// checkScheduledDownloads 检查定时下载
func (dm *DownloadManager) checkScheduledDownloads() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	now := time.Now()
	for _, item := range dm.downloads {
		if item.ScheduledAt != nil && item.ScheduledAt.Before(now) && item.Status == StatusPending {
			dm.enqueue(item)
		}
	}
}

// updateStats 更新统计信息
func (dm *DownloadManager) updateStats() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	stats := DownloadStats{
		ProtocolStats: make(map[DownloadProtocol]int),
	}

	var totalSpeed int64
	activeCount := 0

	for _, item := range dm.downloads {
		stats.TotalDownloads++
		stats.TotalSize += item.FileSize
		stats.DownloadedSize += item.DownloadedSize
		stats.ProtocolStats[item.Protocol]++

		switch item.Status {
		case StatusDownloading, StatusConnecting:
			stats.ActiveDownloads++
			totalSpeed += item.Speed
			activeCount++
		case StatusCompleted:
			stats.CompletedDownloads++
		case StatusFailed:
			stats.FailedDownloads++
		}
	}

	stats.CurrentSpeed = totalSpeed
	if activeCount > 0 {
		stats.AverageSpeed = totalSpeed / int64(activeCount)
	}

	dm.stats = stats
}

// addToHistory 添加到历史记录
func (dm *DownloadManager) addToHistory(item *DownloadItem) {
	history := DownloadHistory{
		ID:       item.ID,
		URL:      item.URL,
		Protocol: item.Protocol,
		FileName: item.FileName,
		FilePath: filepath.Join(item.FilePath, item.FileName),
		FileSize: item.FileSize,
		Status:   item.Status,
		Error:    item.Error,
	}

	if item.StartedAt != nil {
		history.StartedAt = *item.StartedAt
	}
	if item.CompletedAt != nil {
		history.CompletedAt = item.CompletedAt
		history.Duration = item.CompletedAt.Sub(*item.StartedAt)
		if history.Duration > 0 {
			history.AverageSpeed = int64(float64(item.DownloadedSize) / history.Duration.Seconds())
		}
	}

	dm.history = append(dm.history, history)
}

// sendNotification 发送通知
func (dm *DownloadManager) sendNotification(item *DownloadItem) {
	if !dm.notifyConfig.Enabled {
		return
	}

	message := dm.buildNotificationMessage(item)

	for _, method := range dm.notifyConfig.Methods {
		switch method {
		case NotifyWebhook:
			go dm.sendWebhookNotification(message)
		case NotifyEmail:
			go dm.sendEmailNotification(message)
		case NotifyTelegram:
			go dm.sendTelegramNotification(message)
		}
	}
}

// buildNotificationMessage 构建通知消息
func (dm *DownloadManager) buildNotificationMessage(item *DownloadItem) string {
	var status string
	if item.Status == StatusCompleted {
		status = "✅ 下载完成"
	} else {
		status = "❌ 下载失败"
	}

	return fmt.Sprintf("%s\n文件: %s\n大小: %s\n状态: %s",
		status,
		item.FileName,
		formatSize(item.FileSize),
		item.Status,
	)
}

// sendWebhookNotification 发送Webhook通知
func (dm *DownloadManager) sendWebhookNotification(message string) {
	payload := map[string]interface{}{
		"text":      message,
		"timestamp": time.Now().Unix(),
	}

	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest(
		dm.notifyConfig.Webhook.Method,
		dm.notifyConfig.Webhook.URL,
		strings.NewReader(string(jsonData)),
	)

	for k, v := range dm.notifyConfig.Webhook.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	client.Do(req)
}

// sendEmailNotification 发送邮件通知
func (dm *DownloadManager) sendEmailNotification(message string) {
	// 邮件发送实现
	// 实际应使用 net/smtp 包
}

// sendTelegramNotification 发送Telegram通知
func (dm *DownloadManager) sendTelegramNotification(message string) {
	if dm.notifyConfig.Telegram.BotToken == "" {
		return
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage",
		dm.notifyConfig.Telegram.BotToken)

	payload := map[string]interface{}{
		"chat_id": dm.notifyConfig.Telegram.ChatID,
		"text":    message,
	}

	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", apiURL, strings.NewReader(string(jsonData)))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	client.Do(req)
}

// ========== 公共API方法 ==========

// GetDownload 获取下载项
func (dm *DownloadManager) GetDownload(id string) (*DownloadItem, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	item, ok := dm.downloads[id]
	return item, ok
}

// ListDownloads 列出所有下载项
func (dm *DownloadManager) ListDownloads() []*DownloadItem {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	items := make([]*DownloadItem, 0, len(dm.downloads))
	for _, item := range dm.downloads {
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	return items
}

// GetQueue 获取下载队列
func (dm *DownloadManager) GetQueue() []*DownloadItem {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	queue := make([]*DownloadItem, len(dm.queue))
	copy(queue, dm.queue)
	return queue
}

// GetHistory 获取下载历史
func (dm *DownloadManager) GetHistory() []DownloadHistory {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	history := make([]DownloadHistory, len(dm.history))
	copy(history, dm.history)
	return history
}

// GetStats 获取下载统计
func (dm *DownloadManager) GetStats() DownloadStats {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.stats
}

// PauseDownload 暂停下载
func (dm *DownloadManager) PauseDownload(id string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	item, ok := dm.downloads[id]
	if !ok {
		return errors.New("download not found")
	}

	if item.Status != StatusDownloading && item.Status != StatusQueued {
		return errors.New("download cannot be paused")
	}

	item.Status = StatusPaused
	return nil
}

// ResumeDownload 恢复下载
func (dm *DownloadManager) ResumeDownload(id string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	item, ok := dm.downloads[id]
	if !ok {
		return errors.New("download not found")
	}

	if item.Status != StatusPaused {
		return errors.New("download is not paused")
	}

	dm.enqueue(item)
	return nil
}

// CancelDownload 取消下载
func (dm *DownloadManager) CancelDownload(id string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	item, ok := dm.downloads[id]
	if !ok {
		return errors.New("download not found")
	}

	if item.Status == StatusCompleted || item.Status == StatusCancelled {
		return errors.New("download cannot be cancelled")
	}

	item.Status = StatusCancelled

	// 从队列中移除
	for i, q := range dm.queue {
		if q.ID == id {
			dm.queue = append(dm.queue[:i], dm.queue[i+1:]...)
			break
		}
	}

	return nil
}

// DeleteDownload 删除下载记录
func (dm *DownloadManager) DeleteDownload(id string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	item, ok := dm.downloads[id]
	if !ok {
		return errors.New("download not found")
	}

	if item.Status == StatusDownloading || item.Status == StatusConnecting {
		return errors.New("cannot delete active download")
	}

	// 删除文件
	outPath := filepath.Join(item.FilePath, item.FileName)
	os.Remove(outPath)

	delete(dm.downloads, id)
	return nil
}

// UpdateSpeedLimit 更新限速配置
func (dm *DownloadManager) UpdateSpeedLimit(config SpeedLimitConfig) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.speedLimit = config
}

// UpdateNotifyConfig 更新通知配置
func (dm *DownloadManager) UpdateNotifyConfig(config NotifyConfig) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.notifyConfig = config
}

// ========== 辅助方法 ==========

// detectProtocol 检测协议
func (dm *DownloadManager) detectProtocol(rawURL string) DownloadProtocol {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ProtocolHTTP
	}

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "http":
		return ProtocolHTTP
	case "https":
		return ProtocolHTTPS
	case "ftp":
		return ProtocolFTP
	case "magnet":
		return ProtocolMagnet
	default:
		return ProtocolHTTP
	}
}

// validateURL 验证URL
func (dm *DownloadManager) validateURL(rawURL string, protocol DownloadProtocol) error {
	if protocol == ProtocolMagnet || strings.HasPrefix(rawURL, "magnet:") {
		if !strings.HasPrefix(rawURL, "magnet:?xt=urn:") {
			return errors.New("invalid magnet link")
		}
		return nil
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme == "" || u.Host == "" {
		return errors.New("URL must have scheme and host")
	}

	return nil
}

// extractFileName 从URL提取文件名
func (dm *DownloadManager) extractFileName(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "download_" + generateID()
	}

	path := u.Path
	if path == "" || path == "/" {
		return "index.html"
	}

	fileName := filepath.Base(path)
	if fileName == "." || fileName == "/" {
		return "download_" + generateID()
	}

	return fileName
}

// calculateChecksum 计算文件校验和
func (dm *DownloadManager) calculateChecksum(filePath string) (string, error) {
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

// formatSize 格式化文件大小
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

// generateID 生成唯一ID
func generateID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomHex(8))
}

// randomHex 生成随机十六进制字符串
func randomHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = "0123456789abcdef"[time.Now().UnixNano()%16]
		time.Sleep(1) // 确保不同值
	}
	return string(b)
}
