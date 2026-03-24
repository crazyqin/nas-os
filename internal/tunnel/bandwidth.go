// Package tunnel 提供内网穿透服务 - 带宽监控与限速
package tunnel

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// BandwidthConfig 带宽配置
type BandwidthConfig struct {
	// 上传限速（字节/秒），0 表示不限速
	UploadLimit int64 `json:"upload_limit"`
	// 下载限速（字节/秒），0 表示不限速
	DownloadLimit int64 `json:"download_limit"`
	// 统计间隔
	StatsInterval time.Duration `json:"stats_interval"`
	// 令牌桶容量倍数（相对于速率）
	BucketMultiplier int `json:"bucket_multiplier"`
}

// DefaultBandwidthConfig 默认带宽配置
func DefaultBandwidthConfig() BandwidthConfig {
	return BandwidthConfig{
		StatsInterval:    time.Second,
		BucketMultiplier: 2,
	}
}

// BandwidthStats 带宽统计
type BandwidthStats struct {
	// 当前上传速率（字节/秒）
	UploadRate int64 `json:"upload_rate"`
	// 当前下载速率（字节/秒）
	DownloadRate int64 `json:"download_rate"`
	// 总上传字节数
	TotalUpload int64 `json:"total_upload"`
	// 总下载字节数
	TotalDownload int64 `json:"total_download"`
	// 峰值上传速率
	PeakUploadRate int64 `json:"peak_upload_rate"`
	// 峰值下载速率
	PeakDownloadRate int64 `json:"peak_download_rate"`
	// 当前连接数
	Connections int `json:"connections"`
	// 统计时间
	Timestamp time.Time `json:"timestamp"`
}

// TokenBucket 令牌桶限速器
type TokenBucket struct {
	rate      int64 // 令牌产生速率（字节/秒）
	capacity  int64 // 桶容量
	tokens    int64 // 当前令牌数
	lastTime  time.Time
	mu        sync.Mutex
}

// NewTokenBucket 创建令牌桶
func NewTokenBucket(rate, capacity int64) *TokenBucket {
	return &TokenBucket{
		rate:     rate,
		capacity: capacity,
		tokens:   capacity,
		lastTime: time.Now(),
	}
}

// Allow 检查是否允许发送指定字节数
func (b *TokenBucket) Allow(bytes int64) bool {
	if b.rate <= 0 {
		return true
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastTime).Seconds()
	b.lastTime = now

	// 补充令牌
	b.tokens += int64(float64(b.rate) * elapsed)
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}

	if b.tokens >= bytes {
		b.tokens -= bytes
		return true
	}

	return false
}

// Wait 等待直到有足够的令牌
func (b *TokenBucket) Wait(ctx context.Context, bytes int64) error {
	if b.rate <= 0 {
		return nil
	}

	for {
		if b.Allow(bytes) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			continue
		}
	}
}

// BandwidthMonitor 带宽监控器
type BandwidthMonitor struct {
	config BandwidthConfig
	logger *zap.Logger

	// 统计数据
	stats BandwidthStats

	// 令牌桶限速器
	uploadLimiter   *TokenBucket
	downloadLimiter *TokenBucket

	// 统计追踪
	lastUpload    int64
	lastDownload  int64
	lastStatTime  time.Time

	// 历史记录（用于计算平均速率）
	uploadHistory  []int64
	downloadHistory []int64
	historySize    int

	mu sync.RWMutex
}

// NewBandwidthMonitor 创建带宽监控器
func NewBandwidthMonitor(config BandwidthConfig, logger *zap.Logger) *BandwidthMonitor {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &BandwidthMonitor{
		config:         config,
		logger:         logger,
		lastStatTime:   time.Now(),
		uploadHistory:  make([]int64, 0, 60),
		downloadHistory: make([]int64, 0, 60),
		historySize:    60,
	}

	// 初始化限速器
	m.updateLimiters()

	return m
}

// updateLimiters 更新限速器
func (m *BandwidthMonitor) updateLimiters() {
	bucketMultiplier := m.config.BucketMultiplier
	if bucketMultiplier <= 0 {
		bucketMultiplier = 2
	}

	if m.config.UploadLimit > 0 {
		m.uploadLimiter = NewTokenBucket(
			m.config.UploadLimit,
			m.config.UploadLimit*int64(bucketMultiplier),
		)
	}

	if m.config.DownloadLimit > 0 {
		m.downloadLimiter = NewTokenBucket(
			m.config.DownloadLimit,
			m.config.DownloadLimit*int64(bucketMultiplier),
		)
	}
}

// RecordUpload 记录上传字节数
func (m *BandwidthMonitor) RecordUpload(bytes int64) {
	atomic.AddInt64(&m.stats.TotalUpload, bytes)
}

// RecordDownload 记录下载字节数
func (m *BandwidthMonitor) RecordDownload(bytes int64) {
	atomic.AddInt64(&m.stats.TotalDownload, bytes)
}

// AllowUpload 检查是否允许上传
func (m *BandwidthMonitor) AllowUpload(bytes int64) bool {
	if m.uploadLimiter == nil {
		return true
	}
	return m.uploadLimiter.Allow(bytes)
}

// WaitUpload 等待上传令牌
func (m *BandwidthMonitor) WaitUpload(ctx context.Context, bytes int64) error {
	if m.uploadLimiter == nil {
		return nil
	}
	return m.uploadLimiter.Wait(ctx, bytes)
}

// AllowDownload 检查是否允许下载
func (m *BandwidthMonitor) AllowDownload(bytes int64) bool {
	if m.downloadLimiter == nil {
		return true
	}
	return m.downloadLimiter.Allow(bytes)
}

// WaitDownload 等待下载令牌
func (m *BandwidthMonitor) WaitDownload(ctx context.Context, bytes int64) error {
	if m.downloadLimiter == nil {
		return nil
	}
	return m.downloadLimiter.Wait(ctx, bytes)
}

// UpdateStats 更新统计信息
func (m *BandwidthMonitor) UpdateStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(m.lastStatTime).Seconds()
	if elapsed <= 0 {
		return
	}

	currentUpload := atomic.LoadInt64(&m.stats.TotalUpload)
	currentDownload := atomic.LoadInt64(&m.stats.TotalDownload)

	// 计算当前速率
	uploadDiff := currentUpload - m.lastUpload
	downloadDiff := currentDownload - m.lastDownload

	uploadRate := int64(float64(uploadDiff) / elapsed)
	downloadRate := int64(float64(downloadDiff) / elapsed)

	// 更新历史记录
	m.uploadHistory = append(m.uploadHistory, uploadRate)
	m.downloadHistory = append(m.downloadHistory, downloadRate)

	if len(m.uploadHistory) > m.historySize {
		m.uploadHistory = m.uploadHistory[1:]
	}
	if len(m.downloadHistory) > m.historySize {
		m.downloadHistory = m.downloadHistory[1:]
	}

	// 更新统计数据
	m.stats.UploadRate = uploadRate
	m.stats.DownloadRate = downloadRate
	m.stats.Timestamp = now

	// 更新峰值
	if uploadRate > m.stats.PeakUploadRate {
		m.stats.PeakUploadRate = uploadRate
	}
	if downloadRate > m.stats.PeakDownloadRate {
		m.stats.PeakDownloadRate = downloadRate
	}

	m.lastUpload = currentUpload
	m.lastDownload = currentDownload
	m.lastStatTime = now
}

// GetStats 获取统计信息
func (m *BandwidthMonitor) GetStats() BandwidthStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := m.stats
	stats.TotalUpload = atomic.LoadInt64(&m.stats.TotalUpload)
	stats.TotalDownload = atomic.LoadInt64(&m.stats.TotalDownload)
	stats.Timestamp = time.Now()

	return stats
}

// GetAverageRates 获取平均速率
func (m *BandwidthMonitor) GetAverageRates() (avgUpload, avgDownload int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.uploadHistory) == 0 {
		return 0, 0
	}

	var totalUpload, totalDownload int64
	for _, v := range m.uploadHistory {
		totalUpload += v
	}
	for _, v := range m.downloadHistory {
		totalDownload += v
	}

	avgUpload = totalUpload / int64(len(m.uploadHistory))
	avgDownload = totalDownload / int64(len(m.downloadHistory))

	return
}

// SetUploadLimit 设置上传限速
func (m *BandwidthMonitor) SetUploadLimit(limit int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.UploadLimit = limit
	m.updateLimiters()
}

// SetDownloadLimit 设置下载限速
func (m *BandwidthMonitor) SetDownloadLimit(limit int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.DownloadLimit = limit
	m.updateLimiters()
}

// SetConnections 设置当前连接数
func (m *BandwidthMonitor) SetConnections(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats.Connections = count
}

// Reset 重置统计数据
func (m *BandwidthMonitor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats = BandwidthStats{
		Timestamp: time.Now(),
	}
	m.lastUpload = 0
	m.lastDownload = 0
	m.lastStatTime = time.Now()
	m.uploadHistory = m.uploadHistory[:0]
	m.downloadHistory = m.downloadHistory[:0]
}

// BandwidthMonitorService 带宽监控服务
type BandwidthMonitorService struct {
	monitor *BandwidthMonitor
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running atomic.Bool
}

// NewBandwidthMonitorService 创建带宽监控服务
func NewBandwidthMonitorService(config BandwidthConfig, logger *zap.Logger) *BandwidthMonitorService {
	return &BandwidthMonitorService{
		monitor: NewBandwidthMonitor(config, logger),
	}
}

// Start 启动监控服务
func (s *BandwidthMonitorService) Start(ctx context.Context) error {
	if s.running.Swap(true) {
		return nil
	}

	s.ctx, s.cancel = context.WithCancel(ctx)

	interval := s.monitor.config.StatsInterval
	if interval <= 0 {
		interval = time.Second
	}

	s.wg.Add(1)
	go s.monitorLoop(interval)

	return nil
}

// Stop 停止监控服务
func (s *BandwidthMonitorService) Stop() {
	if !s.running.Swap(false) {
		return
	}

	s.cancel()
	s.wg.Wait()
}

// monitorLoop 监控循环
func (s *BandwidthMonitorService) monitorLoop(interval time.Duration) {
	defer s.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.monitor.UpdateStats()
		}
	}
}

// GetMonitor 获取监控器
func (s *BandwidthMonitorService) GetMonitor() *BandwidthMonitor {
	return s.monitor
}

// FormatBandwidth 格式化带宽显示
func FormatBandwidth(bytesPerSec int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytesPerSec >= GB:
		return formatFloat(float64(bytesPerSec)/float64(GB), 2) + " GB/s"
	case bytesPerSec >= MB:
		return formatFloat(float64(bytesPerSec)/float64(MB), 2) + " MB/s"
	case bytesPerSec >= KB:
		return formatFloat(float64(bytesPerSec)/float64(KB), 2) + " KB/s"
	default:
		return formatFloat(float64(bytesPerSec), 0) + " B/s"
	}
}

func formatFloat(f float64, prec int) string {
	return time.Now().Format("2006-01-02 15:04:05") // placeholder
}