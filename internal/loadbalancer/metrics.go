// Package loadbalancer - 负载均衡指标收集
package loadbalancer

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// MetricsCollector 指标收集器.
type MetricsCollector struct {
	// 流量窗口
	windows    []*TrafficWindow
	windowSize time.Duration
	maxWindows int

	// 延迟统计
	latencies  []time.Duration
	maxLatency int

	// 后端指标
	backendMetrics map[string]*BackendMetrics

	mu sync.RWMutex
}

// BackendMetrics 后端指标.
type BackendMetrics struct {
	BackendID     string        `json:"backend_id"`
	Requests      int64         `json:"requests"`
	Errors        int64         `json:"errors"`
	BytesSent     int64         `json:"bytes_sent"`
	BytesRecv     int64         `json:"bytes_recv"`
	TotalLatency  time.Duration `json:"total_latency"`
	MinLatency    time.Duration `json:"min_latency"`
	MaxLatency    time.Duration `json:"max_latency"`
	AvgLatency    time.Duration `json:"avg_latency"`
	P50Latency    time.Duration `json:"p50_latency"`
	P95Latency    time.Duration `json:"p95_latency"`
	P99Latency    time.Duration `json:"p99_latency"`
	LastRequestAt time.Time     `json:"last_request_at"`

	latencySamples []time.Duration
	mu             sync.RWMutex
}

// NewMetricsCollector 创建指标收集器.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		windowSize:     1 * time.Minute,
		maxWindows:     60, // 保留1小时
		maxLatency:     10000,
		backendMetrics: make(map[string]*BackendMetrics),
	}
}

// RecordRequest 记录请求.
func (mc *MetricsCollector) RecordRequest(backendID string, latency time.Duration, bytesSent, bytesRecv int64, err error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// 更新后端指标
	bm := mc.getOrCreateBackendMetrics(backendID)
	bm.record(latency, bytesSent, bytesRecv, err)

	// 更新流量窗口
	mc.updateWindow(1, bytesSent, bytesRecv, err != nil, latency)
}

// getOrCreateBackendMetrics 获取或创建后端指标.
func (mc *MetricsCollector) getOrCreateBackendMetrics(backendID string) *BackendMetrics {
	bm, exists := mc.backendMetrics[backendID]
	if !exists {
		bm = &BackendMetrics{
			BackendID:  backendID,
			MinLatency: time.Duration(^uint64(0) >> 1), // MaxDuration
		}
		mc.backendMetrics[backendID] = bm
	}
	return bm
}

// record 记录后端请求.
func (bm *BackendMetrics) record(latency time.Duration, bytesSent, bytesRecv int64, err error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.Requests++
	if err != nil {
		bm.Errors++
	}
	bm.BytesSent += bytesSent
	bm.BytesRecv += bytesRecv
	bm.TotalLatency += latency
	bm.LastRequestAt = time.Now()

	// 更新延迟统计
	if latency < bm.MinLatency {
		bm.MinLatency = latency
	}
	if latency > bm.MaxLatency {
		bm.MaxLatency = latency
	}

	// 保存延迟样本
	bm.latencySamples = append(bm.latencySamples, latency)
	if len(bm.latencySamples) > 1000 {
		bm.latencySamples = bm.latencySamples[1:]
	}

	// 计算平均延迟
	if bm.Requests > 0 {
		bm.AvgLatency = bm.TotalLatency / time.Duration(bm.Requests)
	}

	// 计算百分位延迟
	if len(bm.latencySamples) > 0 {
		sorted := make([]time.Duration, len(bm.latencySamples))
		copy(sorted, bm.latencySamples)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

		p50Index := int(float64(len(sorted)) * 0.5)
		p95Index := int(float64(len(sorted)) * 0.95)
		p99Index := int(float64(len(sorted)) * 0.99)

		if p50Index < len(sorted) {
			bm.P50Latency = sorted[p50Index]
		}
		if p95Index < len(sorted) {
			bm.P95Latency = sorted[p95Index]
		}
		if p99Index < len(sorted) {
			bm.P99Latency = sorted[p99Index]
		}
	}
}

// updateWindow 更新流量窗口.
func (mc *MetricsCollector) updateWindow(requests int64, bytesSent, bytesRecv int64, isError bool, latency time.Duration) {
	now := time.Now()

	// 获取或创建当前窗口
	if len(mc.windows) == 0 || now.Sub(mc.windows[len(mc.windows)-1].StartTime) >= mc.windowSize {
		// 创建新窗口
		window := &TrafficWindow{
			Duration:  mc.windowSize,
			StartTime: now,
			EndTime:   now.Add(mc.windowSize),
		}
		mc.windows = append(mc.windows, window)

		// 限制窗口数量
		if len(mc.windows) > mc.maxWindows {
			mc.windows = mc.windows[1:]
		}
	}

	window := mc.windows[len(mc.windows)-1]
	window.Requests += requests
	window.BytesSent += bytesSent
	window.BytesRecv += bytesRecv
	if isError {
		window.Errors++
	}
	window.Latencies = append(window.Latencies, latency)

	// 限制延迟样本数
	if len(window.Latencies) > 1000 {
		window.Latencies = window.Latencies[1:]
	}
}

// GetMetrics 获取指标快照.
func (mc *MetricsCollector) GetMetrics() TrafficMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if len(mc.windows) == 0 {
		return TrafficMetrics{Timestamp: time.Now()}
	}

	// 获取最近的窗口
	window := mc.windows[len(mc.windows)-1]
	elapsed := time.Since(window.StartTime).Seconds()
	if elapsed == 0 {
		elapsed = 1
	}

	metrics := TrafficMetrics{
		Timestamp:      time.Now(),
		RequestsPerSec: float64(window.Requests) / elapsed,
		BytesPerSec:    float64(window.BytesSent+window.BytesRecv) / elapsed,
		ErrorRate:      float64(window.Errors) / float64(window.Requests),
	}

	// 计算延迟百分位
	if len(window.Latencies) > 0 {
		sorted := make([]time.Duration, len(window.Latencies))
		copy(sorted, window.Latencies)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

		p50Index := int(float64(len(sorted)) * 0.5)
		p95Index := int(float64(len(sorted)) * 0.95)
		p99Index := int(float64(len(sorted)) * 0.99)

		if p50Index < len(sorted) {
			metrics.P50LatencyMs = float64(sorted[p50Index].Microseconds()) / 1000
		}
		if p95Index < len(sorted) {
			metrics.P95LatencyMs = float64(sorted[p95Index].Microseconds()) / 1000
		}
		if p99Index < len(sorted) {
			metrics.P99LatencyMs = float64(sorted[p99Index].Microseconds()) / 1000
		}
	}

	return metrics
}

// GetBackendMetrics 获取后端指标.
func (mc *MetricsCollector) GetBackendMetrics(backendID string) *BackendMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	bm, exists := mc.backendMetrics[backendID]
	if !exists {
		return nil
	}

	// 返回副本
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	result := &BackendMetrics{
		BackendID:     bm.BackendID,
		Requests:      bm.Requests,
		Errors:        bm.Errors,
		BytesSent:     bm.BytesSent,
		BytesRecv:     bm.BytesRecv,
		TotalLatency:  bm.TotalLatency,
		MinLatency:    bm.MinLatency,
		MaxLatency:    bm.MaxLatency,
		AvgLatency:    bm.AvgLatency,
		P50Latency:    bm.P50Latency,
		P95Latency:    bm.P95Latency,
		P99Latency:    bm.P99Latency,
		LastRequestAt: bm.LastRequestAt,
	}

	return result
}

// GetAllBackendMetrics 获取所有后端指标.
func (mc *MetricsCollector) GetAllBackendMetrics() map[string]*BackendMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make(map[string]*BackendMetrics, len(mc.backendMetrics))
	for id, bm := range mc.backendMetrics {
		bm.mu.RLock()
		result[id] = &BackendMetrics{
			BackendID:     bm.BackendID,
			Requests:      bm.Requests,
			Errors:        bm.Errors,
			BytesSent:     bm.BytesSent,
			BytesRecv:     bm.BytesRecv,
			TotalLatency:  bm.TotalLatency,
			MinLatency:    bm.MinLatency,
			MaxLatency:    bm.MaxLatency,
			AvgLatency:    bm.AvgLatency,
			P50Latency:    bm.P50Latency,
			P95Latency:    bm.P95Latency,
			P99Latency:    bm.P99Latency,
			LastRequestAt: bm.LastRequestAt,
		}
		bm.mu.RUnlock()
	}

	return result
}

// GetTrafficWindows 获取流量窗口.
func (mc *MetricsCollector) GetTrafficWindows(duration time.Duration) []TrafficWindow {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if len(mc.windows) == 0 {
		return nil
	}

	cutoff := time.Now().Add(-duration)
	var result []TrafficWindow

	for _, window := range mc.windows {
		if window.StartTime.After(cutoff) {
			result = append(result, TrafficWindow{
				Duration:  window.Duration,
				StartTime: window.StartTime,
				EndTime:   window.EndTime,
				Requests:  window.Requests,
				Errors:    window.Errors,
				BytesSent: window.BytesSent,
				BytesRecv: window.BytesRecv,
			})
		}
	}

	return result
}

// Reset 重置指标.
func (mc *MetricsCollector) Reset() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.windows = nil
	mc.backendMetrics = make(map[string]*BackendMetrics)
}

// ============================================================
// Prometheus格式指标
// ============================================================

// ToPrometheusMetrics 导出Prometheus格式指标.
func (mc *MetricsCollector) ToPrometheusMetrics() string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var result string

	// 全局指标
	metrics := mc.GetMetrics()
	result += "# HELP lb_requests_per_sec Requests per second\n"
	result += "# TYPE lb_requests_per_sec gauge\n"
	result += fmt.Sprintf("lb_requests_per_sec %.2f\n", metrics.RequestsPerSec)

	result += "# HELP lb_bytes_per_sec Bytes per second\n"
	result += "# TYPE lb_bytes_per_sec gauge\n"
	result += fmt.Sprintf("lb_bytes_per_sec %.2f\n", metrics.BytesPerSec)

	result += "# HELP lb_error_rate Error rate\n"
	result += "# TYPE lb_error_rate gauge\n"
	result += fmt.Sprintf("lb_error_rate %.4f\n", metrics.ErrorRate)

	result += "# HELP lb_p50_latency_ms P50 latency in milliseconds\n"
	result += "# TYPE lb_p50_latency_ms gauge\n"
	result += fmt.Sprintf("lb_p50_latency_ms %.2f\n", metrics.P50LatencyMs)

	result += "# HELP lb_p95_latency_ms P95 latency in milliseconds\n"
	result += "# TYPE lb_p95_latency_ms gauge\n"
	result += fmt.Sprintf("lb_p95_latency_ms %.2f\n", metrics.P95LatencyMs)

	result += "# HELP lb_p99_latency_ms P99 latency in milliseconds\n"
	result += "# TYPE lb_p99_latency_ms gauge\n"
	result += fmt.Sprintf("lb_p99_latency_ms %.2f\n", metrics.P99LatencyMs)

	// 后端指标
	for id, bm := range mc.backendMetrics {
		bm.mu.RLock()
		result += fmt.Sprintf("lb_backend_requests{backend=\"%s\"} %d\n", id, bm.Requests)
		result += fmt.Sprintf("lb_backend_errors{backend=\"%s\"} %d\n", id, bm.Errors)
		result += fmt.Sprintf("lb_backend_avg_latency_ms{backend=\"%s\"} %.2f\n", id, float64(bm.AvgLatency.Microseconds())/1000)
		bm.mu.RUnlock()
	}

	return result
}
