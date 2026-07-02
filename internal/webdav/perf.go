// Package webdav 提供 WebDAV 性能增强功能，包括批量操作、流式传输优化、
// 并发连接池管理和性能监控指标。
package webdav

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// ========== 批量操作 ==========

// BatchRequest 批量操作请求.
type BatchRequest struct {
	Operation string       `json:"operation"` // upload, download, delete
	Items     []BatchItem  `json:"items"`
	Options   BatchOptions `json:"options,omitempty"`
}

// BatchItem 批量操作项.
type BatchItem struct {
	Source      string `json:"source"`
	Destination string `json:"destination,omitempty"`
	Overwrite   bool   `json:"overwrite,omitempty"`
}

// BatchOptions 批量操作选项.
type BatchOptions struct {
	Concurrency     int  `json:"concurrency,omitempty"`       // 并发数，0=自动
	ContinueOnError bool `json:"continue_on_error,omitempty"` // 遇错继续
	ChunkSize       int  `json:"chunk_size,omitempty"`        // 分块大小（字节）
}

// BatchResult 批量操作结果.
type BatchResult struct {
	Operation  string        `json:"operation"`
	Total      int           `json:"total"`
	Success    int           `json:"success"`
	Failed     int           `json:"failed"`
	Skipped    int           `json:"skipped"`
	Errors     []BatchError  `json:"errors,omitempty"`
	Duration   time.Duration `json:"duration"`
	TotalBytes int64         `json:"total_bytes"`
}

// BatchError 批量操作错误.
type BatchError struct {
	Item  string `json:"item"`
	Error string `json:"error"`
	Index int    `json:"index"`
}

// BatchManager 批量操作管理器.
type BatchManager struct {
	mu        sync.RWMutex
	pool      *ConnectionPool
	chunkSize int64
}

// NewBatchManager 创建批量操作管理器.
func NewBatchManager(pool *ConnectionPool) *BatchManager {
	return &BatchManager{
		pool:      pool,
		chunkSize: DefaultChunkSize,
	}
}

// SetChunkSize 设置分块大小.
func (bm *BatchManager) SetChunkSize(size int64) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if size > 0 {
		bm.chunkSize = size
	}
}

// ExecuteBatch 执行批量操作.
func (bm *BatchManager) ExecuteBatch(ctx context.Context, rootPath string, req *BatchRequest) (*BatchResult, error) {
	if req == nil || len(req.Items) == 0 {
		return nil, fmt.Errorf("批量操作请求为空")
	}

	start := time.Now()
	result := &BatchResult{
		Operation: req.Operation,
		Total:     len(req.Items),
	}

	// 确定并发数
	concurrency := req.Options.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultBatchConcurrency
	}
	if concurrency > len(req.Items) {
		concurrency = len(req.Items)
	}

	switch req.Operation {
	case "upload":
		bm.executeBatchUpload(ctx, rootPath, req, result, concurrency)
	case "download":
		bm.executeBatchDownload(ctx, rootPath, req, result, concurrency)
	case "delete":
		bm.executeBatchDelete(ctx, rootPath, req, result, concurrency)
	default:
		return nil, fmt.Errorf("不支持的操作类型: %s", req.Operation)
	}

	result.Duration = time.Since(start)
	return result, nil
}

// executeBatchUpload 并发批量上传.
func (bm *BatchManager) executeBatchUpload(ctx context.Context, rootPath string, req *BatchRequest, result *BatchResult, concurrency int) {
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, item := range req.Items {
		select {
		case <-ctx.Done():
			mu.Lock()
			result.Skipped += len(req.Items) - i
			mu.Unlock()
			return
		default:
		}

		wg.Add(1)
		go func(idx int, it BatchItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			destPath := filepath.Join(rootPath, it.Destination)
			err := bm.streamUpload(ctx, destPath, it.Source, it.Overwrite)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if req.Options.ContinueOnError {
					result.Failed++
					result.Errors = append(result.Errors, BatchError{
						Item:  it.Source,
						Error: err.Error(),
						Index: idx,
					})
					return
				}
				result.Failed++
				result.Errors = append(result.Errors, BatchError{
					Item:  it.Source,
					Error: err.Error(),
					Index: idx,
				})
			} else {
				result.Success++
			}
		}(i, item)
	}

	wg.Wait()
}

// executeBatchDownload 并发批量下载.
func (bm *BatchManager) executeBatchDownload(ctx context.Context, rootPath string, req *BatchRequest, result *BatchResult, concurrency int) {
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, item := range req.Items {
		select {
		case <-ctx.Done():
			mu.Lock()
			result.Skipped += len(req.Items) - i
			mu.Unlock()
			return
		default:
		}

		wg.Add(1)
		go func(idx int, it BatchItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			srcPath := filepath.Join(rootPath, it.Source)
			writer := &countingWriter{}
			err := bm.streamDownload(ctx, srcPath, writer)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, BatchError{
					Item:  it.Source,
					Error: err.Error(),
					Index: idx,
				})
			} else {
				result.Success++
				result.TotalBytes += writer.written
			}
		}(i, item)
	}

	wg.Wait()
}

// executeBatchDelete 并发批量删除.
func (bm *BatchManager) executeBatchDelete(ctx context.Context, rootPath string, req *BatchRequest, result *BatchResult, concurrency int) {
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, item := range req.Items {
		select {
		case <-ctx.Done():
			mu.Lock()
			result.Skipped += len(req.Items) - i
			mu.Unlock()
			return
		default:
		}

		wg.Add(1)
		go func(idx int, it BatchItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			targetPath := filepath.Join(rootPath, it.Source)
			err := os.RemoveAll(targetPath)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, BatchError{
					Item:  it.Source,
					Error: err.Error(),
					Index: idx,
				})
			} else {
				result.Success++
			}
		}(i, item)
	}

	wg.Wait()
}

// streamUpload 流式上传（分块写入）.
func (bm *BatchManager) streamUpload(ctx context.Context, destPath, source string, overwrite bool) error {
	// 检查目标是否已存在
	if _, err := os.Stat(destPath); err == nil && !overwrite {
		return fmt.Errorf("目标文件已存在: %s", destPath)
	}

	// 确保目录存在
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 创建临时文件
	tmpPath := destPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer func() { _ = file.Close() }()

	// 模拟流式写入（实际中 source 会是 io.Reader）
	// 此处写入 source 路径的内容
	srcFile, err := os.Open(source)
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer func() { _ = srcFile.Close() }()

	// 使用缓冲区进行分块复制
	buf := make([]byte, bm.chunkSize)
	for {
		select {
		case <-ctx.Done():
			_ = os.Remove(tmpPath)
			return ctx.Err()
		default:
		}

		n, readErr := srcFile.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				_ = os.Remove(tmpPath)
				return fmt.Errorf("写入失败: %w", writeErr)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("读取失败: %w", readErr)
		}
	}

	// 同步到磁盘
	if err := file.Sync(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("同步磁盘失败: %w", err)
	}

	// 重命名
	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("重命名失败: %w", err)
	}

	return nil
}

// streamDownload 流式下载.
func (bm *BatchManager) streamDownload(ctx context.Context, srcPath string, dst io.Writer) error {
	file, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer func() { _ = file.Close() }()

	buf := make([]byte, bm.chunkSize)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, readErr := file.Read(buf)
		if n > 0 {
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("写入失败: %w", writeErr)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("读取失败: %w", readErr)
		}
	}

	return nil
}

// countingWriter 用于统计写入字节数.
type countingWriter struct {
	written int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.written += int64(len(p))
	return len(p), nil
}

// ========== 并发连接池 ==========

// PoolStats 连接池统计.
type PoolStats struct {
	Active   int   `json:"active"`
	Idle     int   `json:"idle"`
	Total    int   `json:"total"`
	MaxConn  int   `json:"max_conn"`
	Waiting  int   `json:"waiting"`
	Errors   int64 `json:"errors"`
	Timeouts int64 `json:"timeouts"`
}

// ConnectionPool 并发连接池.
type ConnectionPool struct {
	mu        sync.RWMutex
	maxConn   int
	active    int
	idle      int
	waitQueue chan struct{}
	stats     PoolStats
	timeout   time.Duration
	onAcquire func()
	onRelease func()
}

// PoolOption 连接池选项.
type PoolOption func(*ConnectionPool)

// WithMaxConnections 设置最大连接数.
func WithMaxConnections(max int) PoolOption {
	return func(p *ConnectionPool) {
		if max > 0 {
			p.maxConn = max
		}
	}
}

// WithPoolTimeout 设置连接池超时.
func WithPoolTimeout(timeout time.Duration) PoolOption {
	return func(p *ConnectionPool) {
		p.timeout = timeout
	}
}

// WithOnAcquire 设置获取连接回调.
func WithOnAcquire(fn func()) PoolOption {
	return func(p *ConnectionPool) {
		p.onAcquire = fn
	}
}

// WithOnRelease 设置释放连接回调.
func WithOnRelease(fn func()) PoolOption {
	return func(p *ConnectionPool) {
		p.onRelease = fn
	}
}

// NewConnectionPool 创建连接池.
func NewConnectionPool(opts ...PoolOption) *ConnectionPool {
	p := &ConnectionPool{
		maxConn:   DefaultMaxConnections,
		timeout:   30 * time.Second,
		waitQueue: make(chan struct{}, DefaultMaxConnections*2),
	}

	for _, opt := range opts {
		opt(p)
	}

	p.stats.MaxConn = p.maxConn
	return p
}

// Acquire 获取连接.
func (p *ConnectionPool) Acquire(ctx context.Context) error {
	p.mu.Lock()

	if p.active < p.maxConn {
		p.active++
		p.idle--
		if p.idle < 0 {
			p.idle = 0
		}
		p.stats.Active = p.active
		p.stats.Idle = p.idle
		p.mu.Unlock()

		if p.onAcquire != nil {
			p.onAcquire()
		}
		return nil
	}

	p.stats.Waiting++
	p.mu.Unlock()

	// 等待连接可用
	select {
	case <-ctx.Done():
		p.mu.Lock()
		p.stats.Waiting--
		p.stats.Timeouts++
		p.mu.Unlock()
		return fmt.Errorf("等待连接超时")
	case <-time.After(p.timeout):
		p.mu.Lock()
		p.stats.Waiting--
		p.stats.Timeouts++
		p.mu.Unlock()
		return fmt.Errorf("连接池超时")
	}
}

// Release 释放连接.
func (p *ConnectionPool) Release() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.active > 0 {
		p.active--
		p.idle++
		p.stats.Active = p.active
		p.stats.Idle = p.idle
	}

	if p.onRelease != nil {
		p.onRelease()
	}
}

// GetStats 获取连接池统计.
func (p *ConnectionPool) GetStats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	stats := p.stats
	stats.Total = p.active + p.idle
	return stats
}

// Resize 动态调整连接池大小.
func (p *ConnectionPool) Resize(newMax int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if newMax <= 0 {
		return
	}
	p.maxConn = newMax
	p.stats.MaxConn = newMax
	log.Printf("[ConnectionPool] 池大小调整为 %d", newMax)
}

// ========== 性能监控 ==========

// PerfMetrics 性能指标.
type PerfMetrics struct {
	mu sync.RWMutex

	// 请求统计
	TotalRequests   int64 `json:"total_requests"`
	ActiveRequests  int64 `json:"active_requests"`
	SuccessRequests int64 `json:"success_requests"`
	ErrorRequests   int64 `json:"error_requests"`

	// 传输统计
	TotalBytesSent     int64 `json:"total_bytes_sent"`
	TotalBytesReceived int64 `json:"total_bytes_received"`
	AvgLatencyMs       int64 `json:"avg_latency_ms"`
	MaxLatencyMs       int64 `json:"max_latency_ms"`
	P99LatencyMs       int64 `json:"p99_latency_ms"`

	// 操作分类统计
	GetOps      int64 `json:"get_ops"`
	PutOps      int64 `json:"put_ops"`
	DeleteOps   int64 `json:"delete_ops"`
	PropfindOps int64 `json:"propfind_ops"`
	LockOps     int64 `json:"lock_ops"`
	BatchOps    int64 `json:"batch_ops"`

	// 连接统计
	ConnectionsActive int64 `json:"connections_active"`
	ConnectionsTotal  int64 `json:"connections_total"`
	ConnectionErrors  int64 `json:"connection_errors"`

	// 延迟分布
	latencies    []int64
	latencyIndex int

	// 时间窗口
	StartTime       time.Time  `json:"start_time"`
	LastRequestTime *time.Time `json:"last_request_time,omitempty"`
}

// LatencyBucket 延迟桶.
type LatencyBucket struct {
	MinMs int64   `json:"min_ms"`
	MaxMs int64   `json:"max_ms"`
	Count int64   `json:"count"`
	Pct   float64 `json:"pct"`
}

// NewPerfMetrics 创建性能监控.
func NewPerfMetrics() *PerfMetrics {
	return &PerfMetrics{
		StartTime: time.Now(),
		latencies: make([]int64, 1000),
	}
}

// RecordRequest 记录请求.
func (pm *PerfMetrics) RecordRequest(method string, bytesSent, bytesRecv int64, latencyMs int64, err error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.TotalRequests++

	// 更新最后请求时间
	now := time.Now()
	pm.LastRequestTime = &now

	// 记录操作类型
	switch strings.ToUpper(method) {
	case "GET", "HEAD":
		pm.GetOps++
	case "PUT":
		pm.PutOps++
	case "DELETE":
		pm.DeleteOps++
	case "PROPFIND":
		pm.PropfindOps++
	case "LOCK", "UNLOCK":
		pm.LockOps++
	case "POST":
		pm.BatchOps++
	}

	// 传输统计
	pm.TotalBytesSent += bytesSent
	pm.TotalBytesReceived += bytesRecv

	// 错误统计
	if err != nil {
		pm.ErrorRequests++
	} else {
		pm.SuccessRequests++
	}

	// 延迟统计
	pm.latencies[pm.latencyIndex%1000] = latencyMs
	pm.latencyIndex++

	if latencyMs > pm.MaxLatencyMs {
		pm.MaxLatencyMs = latencyMs
	}

	// 计算平均延迟
	total := int64(0)
	count := int64(0)
	for _, l := range pm.latencies {
		if l > 0 {
			total += l
			count++
		}
	}
	if count > 0 {
		pm.AvgLatencyMs = total / count
	}

	// P99 延迟
	pm.P99LatencyMs = pm.calculateP99()
}

// calculateP99 计算 P99 延迟.
func (pm *PerfMetrics) calculateP99() int64 {
	var sorted []int64
	for _, l := range pm.latencies {
		if l > 0 {
			sorted = append(sorted, l)
		}
	}
	if len(sorted) == 0 {
		return 0
	}

	// 简单排序
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	idx := int(float64(len(sorted)) * 0.99)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// StartRequest 标记请求开始.
func (pm *PerfMetrics) StartRequest() int64 {
	atomic.AddInt64(&pm.ActiveRequests, 1)
	atomic.AddInt64(&pm.ConnectionsActive, 1)
	atomic.AddInt64(&pm.ConnectionsTotal, 1)
	return time.Now().UnixMilli()
}

// EndRequest 标记请求结束.
func (pm *PerfMetrics) EndRequest(startTimeMs int64) int64 {
	atomic.AddInt64(&pm.ActiveRequests, -1)
	atomic.AddInt64(&pm.ConnectionsActive, -1)
	return time.Now().UnixMilli() - startTimeMs
}

// GetSnapshot 获取指标快照.
func (pm *PerfMetrics) GetSnapshot() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	uptime := time.Since(pm.StartTime)

	// 延迟分布
	buckets := []LatencyBucket{
		{MinMs: 0, MaxMs: 10},
		{MinMs: 10, MaxMs: 50},
		{MinMs: 50, MaxMs: 100},
		{MinMs: 100, MaxMs: 500},
		{MinMs: 500, MaxMs: 1000},
		{MinMs: 1000, MaxMs: 0},
	}

	for _, l := range pm.latencies {
		if l <= 0 {
			continue
		}
		for i := range buckets {
			if l >= buckets[i].MinMs && (buckets[i].MaxMs == 0 || l < buckets[i].MaxMs) {
				buckets[i].Count++
			}
		}
	}

	total := pm.TotalRequests
	for i := range buckets {
		if total > 0 {
			buckets[i].Pct = float64(buckets[i].Count) / float64(total) * 100
		}
	}

	return map[string]interface{}{
		"uptime_seconds":   int64(uptime.Seconds()),
		"total_requests":   pm.TotalRequests,
		"active_requests":  pm.ActiveRequests,
		"success_requests": pm.SuccessRequests,
		"error_requests":   pm.ErrorRequests,
		"error_rate":       pm.errorRate(),
		"bytes_sent":       pm.TotalBytesSent,
		"bytes_received":   pm.TotalBytesReceived,
		"avg_latency_ms":   pm.AvgLatencyMs,
		"max_latency_ms":   pm.MaxLatencyMs,
		"p99_latency_ms":   pm.P99LatencyMs,
		"latency_dist":     buckets,
		"ops_breakdown": map[string]int64{
			"get":      pm.GetOps,
			"put":      pm.PutOps,
			"delete":   pm.DeleteOps,
			"propfind": pm.PropfindOps,
			"lock":     pm.LockOps,
			"batch":    pm.BatchOps,
		},
		"connections_active": pm.ConnectionsActive,
		"connections_total":  pm.ConnectionsTotal,
		"connection_errors":  pm.ConnectionErrors,
		"last_request":       pm.LastRequestTime,
	}
}

// errorRate 计算错误率.
func (pm *PerfMetrics) errorRate() float64 {
	if pm.TotalRequests == 0 {
		return 0
	}
	return float64(pm.ErrorRequests) / float64(pm.TotalRequests) * 100
}

// Reset 重置统计.
func (pm *PerfMetrics) Reset() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.TotalRequests = 0
	pm.ActiveRequests = 0
	pm.SuccessRequests = 0
	pm.ErrorRequests = 0
	pm.TotalBytesSent = 0
	pm.TotalBytesReceived = 0
	pm.AvgLatencyMs = 0
	pm.MaxLatencyMs = 0
	pm.P99LatencyMs = 0
	pm.GetOps = 0
	pm.PutOps = 0
	pm.DeleteOps = 0
	pm.PropfindOps = 0
	pm.LockOps = 0
	pm.BatchOps = 0
	pm.ConnectionsActive = 0
	pm.ConnectionsTotal = 0
	pm.ConnectionErrors = 0
	pm.latencies = make([]int64, 1000)
	pm.latencyIndex = 0
	pm.StartTime = time.Now()
	pm.LastRequestTime = nil
}

// ========== 流式传输优化 ==========

// StreamConfig 流式传输配置.
type StreamConfig struct {
	ChunkSize      int64 `json:"chunk_size"`      // 分块大小
	MaxConcurrent  int   `json:"max_concurrent"`  // 最大并发数
	BufferSize     int   `json:"buffer_size"`     // 缓冲区大小
	EnableCompress bool  `json:"enable_compress"` // 启用压缩
}

// DefaultStreamConfig 默认流式传输配置.
func DefaultStreamConfig() *StreamConfig {
	return &StreamConfig{
		ChunkSize:      DefaultChunkSize,
		MaxConcurrent:  4,
		BufferSize:     64 * 1024,
		EnableCompress: false,
	}
}

// StreamTransmitter 流式传输器.
type StreamTransmitter struct {
	config  *StreamConfig
	metrics *PerfMetrics
}

// NewStreamTransmitter 创建流式传输器.
func NewStreamTransmitter(config *StreamConfig, metrics *PerfMetrics) *StreamTransmitter {
	if config == nil {
		config = DefaultStreamConfig()
	}
	return &StreamTransmitter{
		config:  config,
		metrics: metrics,
	}
}

// StreamWrite 流式写入文件.
func (st *StreamTransmitter) StreamWrite(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	startTime := time.Now().UnixMilli()
	if st.metrics != nil {
		st.metrics.StartRequest()
	}

	buf := make([]byte, st.config.BufferSize)
	var total int64

	for {
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		default:
		}

		n, readErr := src.Read(buf)
		if n > 0 {
			written, writeErr := dst.Write(buf[:n])
			total += int64(written)
			if writeErr != nil {
				if st.metrics != nil {
					latency := st.metrics.EndRequest(startTime)
					st.metrics.RecordRequest("PUT", 0, total, latency, writeErr)
				}
				return total, fmt.Errorf("写入失败: %w", writeErr)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			if st.metrics != nil {
				latency := st.metrics.EndRequest(startTime)
				st.metrics.RecordRequest("PUT", 0, total, latency, readErr)
			}
			return total, fmt.Errorf("读取失败: %w", readErr)
		}
	}

	if st.metrics != nil {
		latency := st.metrics.EndRequest(startTime)
		st.metrics.RecordRequest("PUT", total, 0, latency, nil)
	}
	return total, nil
}

// StreamRead 流式读取文件.
func (st *StreamTransmitter) StreamRead(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	startTime := time.Now().UnixMilli()
	if st.metrics != nil {
		st.metrics.StartRequest()
	}

	buf := make([]byte, st.config.BufferSize)
	var total int64

	for {
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		default:
		}

		n, readErr := src.Read(buf)
		if n > 0 {
			written, writeErr := dst.Write(buf[:n])
			total += int64(written)
			if writeErr != nil {
				if st.metrics != nil {
					latency := st.metrics.EndRequest(startTime)
					st.metrics.RecordRequest("GET", total, 0, latency, writeErr)
				}
				return total, fmt.Errorf("写入失败: %w", writeErr)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			if st.metrics != nil {
				latency := st.metrics.EndRequest(startTime)
				st.metrics.RecordRequest("GET", total, 0, latency, readErr)
			}
			return total, fmt.Errorf("读取失败: %w", readErr)
		}
	}

	if st.metrics != nil {
		latency := st.metrics.EndRequest(startTime)
		st.metrics.RecordRequest("GET", 0, total, latency, nil)
	}
	return total, nil
}

// ========== 性能增强管理器 ==========

// PerfManager 性能增强管理器.
type PerfManager struct {
	mu          sync.RWMutex
	pool        *ConnectionPool
	batchMgr    *BatchManager
	transmitter *StreamTransmitter
	metrics     *PerfMetrics
	enabled     bool
}

// NewPerfManager 创建性能增强管理器.
func NewPerfManager() *PerfManager {
	pool := NewConnectionPool()
	metrics := NewPerfMetrics()
	batchMgr := NewBatchManager(pool)
	transmitter := NewStreamTransmitter(nil, metrics)

	return &PerfManager{
		pool:        pool,
		batchMgr:    batchMgr,
		transmitter: transmitter,
		metrics:     metrics,
		enabled:     true,
	}
}

// SetEnabled 设置启用状态.
func (pm *PerfManager) SetEnabled(enabled bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.enabled = enabled
}

// IsEnabled 是否启用.
func (pm *PerfManager) IsEnabled() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.enabled
}

// GetPool 获取连接池.
func (pm *PerfManager) GetPool() *ConnectionPool {
	return pm.pool
}

// GetBatchManager 获取批量管理器.
func (pm *PerfManager) GetBatchManager() *BatchManager {
	return pm.batchMgr
}

// GetTransmitter 获取流式传输器.
func (pm *PerfManager) GetTransmitter() *StreamTransmitter {
	return pm.transmitter
}

// GetMetrics 获取性能指标.
func (pm *PerfManager) GetMetrics() *PerfMetrics {
	return pm.metrics
}

// GetStatus 获取性能增强状态.
func (pm *PerfManager) GetStatus() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return map[string]interface{}{
		"enabled":        pm.enabled,
		"pool_stats":     pm.pool.GetStats(),
		"metrics":        pm.metrics.GetSnapshot(),
		"chunk_size":     pm.batchMgr.chunkSize,
		"buffer_size":    pm.transmitter.config.BufferSize,
		"max_concurrent": pm.transmitter.config.MaxConcurrent,
	}
}

// Reset 重置所有性能指标.
func (pm *PerfManager) Reset() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.metrics.Reset()
}

// ========== 默认常量 ==========

const (
	// DefaultChunkSize 默认分块大小 256KB.
	DefaultChunkSize = 256 * 1024
	// DefaultMaxConnections 默认最大连接数.
	DefaultMaxConnections = 100
	// DefaultBatchConcurrency 默认批量并发数.
	DefaultBatchConcurrency = 10
)

// ========== HTTP 处理器集成 ==========

// RegisterPerfRoutes 注册性能增强路由.
func (pm *PerfManager) RegisterPerfRoutes(apiGroup *gin.RouterGroup) {
	perf := apiGroup.Group("/webdav/perf")
	{
		perf.GET("/status", pm.handleGetStatus)
		perf.GET("/metrics", pm.handleGetMetrics)
		perf.POST("/metrics/reset", pm.handleResetMetrics)
		perf.GET("/pool", pm.handleGetPoolStats)
		perf.PUT("/pool/resize", pm.handleResizePool)
		perf.POST("/batch", pm.handleBatch)
		perf.PUT("/config", pm.handleUpdateConfig)
	}
}

func (pm *PerfManager) handleGetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    pm.GetStatus(),
	})
}

func (pm *PerfManager) handleGetMetrics(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    pm.metrics.GetSnapshot(),
	})
}

func (pm *PerfManager) handleResetMetrics(c *gin.Context) {
	pm.Reset()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "性能指标已重置",
	})
}

func (pm *PerfManager) handleGetPoolStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    pm.pool.GetStats(),
	})
}

func (pm *PerfManager) handleResizePool(c *gin.Context) {
	var req struct {
		MaxConn int `json:"max_conn" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	pm.pool.Resize(req.MaxConn)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": fmt.Sprintf("连接池大小已调整为 %d", req.MaxConn),
	})
}

func (pm *PerfManager) handleBatch(c *gin.Context) {
	var req BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	rootPath := c.Query("root_path")
	if rootPath == "" {
		rootPath = "/data"
	}

	result, err := pm.batchMgr.ExecuteBatch(c.Request.Context(), rootPath, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "批量操作完成",
		"data":    result,
	})
}

func (pm *PerfManager) handleUpdateConfig(c *gin.Context) {
	var req struct {
		ChunkSize      *int64 `json:"chunk_size"`
		MaxConcurrent  *int   `json:"max_concurrent"`
		BufferSize     *int   `json:"buffer_size"`
		EnableCompress *bool  `json:"enable_compress"`
		Enabled        *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if req.ChunkSize != nil && *req.ChunkSize > 0 {
		pm.batchMgr.SetChunkSize(*req.ChunkSize)
	}
	if req.MaxConcurrent != nil && *req.MaxConcurrent > 0 {
		pm.transmitter.config.MaxConcurrent = *req.MaxConcurrent
	}
	if req.BufferSize != nil && *req.BufferSize > 0 {
		pm.transmitter.config.BufferSize = *req.BufferSize
	}
	if req.EnableCompress != nil {
		pm.transmitter.config.EnableCompress = *req.EnableCompress
	}
	if req.Enabled != nil {
		pm.enabled = *req.Enabled
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "配置已更新",
		"data": map[string]interface{}{
			"enabled":         pm.enabled,
			"chunk_size":      pm.batchMgr.chunkSize,
			"max_concurrent":  pm.transmitter.config.MaxConcurrent,
			"buffer_size":     pm.transmitter.config.BufferSize,
			"enable_compress": pm.transmitter.config.EnableCompress,
		},
	})
}
