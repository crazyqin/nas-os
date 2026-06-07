// Package microsvcmesh 可观测性，支持分布式追踪和指标收集
package microsvcmesh

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Tracer 分布式追踪器
type Tracer struct {
	mu         sync.RWMutex
	logger     *zap.Logger
	sampleRate float64
	spans      []*TraceSpan
	maxSpans   int
}

// NewTracer 创建追踪器
func NewTracer(logger *zap.Logger, sampleRate float64) *Tracer {
	if logger == nil {
		logger = zap.NewNop()
	}
	if sampleRate > 1 {
		sampleRate = 1.0
	}
	return &Tracer{
		logger:     logger,
		sampleRate: sampleRate,
		spans:      make([]*TraceSpan, 0),
		maxSpans:   10000,
	}
}

// StartSpan 开始一个追踪跨度
func (t *Tracer) StartSpan(name, service string) *TraceSpan {
	return &TraceSpan{
		TraceID:   generateTraceID(),
		SpanID:    generateTraceID(),
		Name:      name,
		Service:   service,
		StartTime: time.Now(),
		Tags:      make(map[string]string),
		Events:    make([]TraceEvent, 0),
	}
}

// StartChildSpan 开始子跨度
func (t *Tracer) StartChildSpan(parent *TraceSpan, name, service string) *TraceSpan {
	return &TraceSpan{
		TraceID:   parent.TraceID,
		SpanID:    generateTraceID(),
		ParentID:  parent.SpanID,
		Name:      name,
		Service:   service,
		StartTime: time.Now(),
		Tags:      make(map[string]string),
		Events:    make([]TraceEvent, 0),
	}
}

// RecordSpan 记录追踪跨度
func (t *Tracer) RecordSpan(span *TraceSpan) {
	// 采样判断
	if !t.shouldSample() {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// 限制存储量
	if len(t.spans) >= t.maxSpans {
		t.spans = t.spans[1:]
	}

	t.spans = append(t.spans, span)
}

// shouldSample 是否采样
func (t *Tracer) shouldSample() bool {
	if t.sampleRate >= 1.0 {
		return true
	}
	if t.sampleRate <= 0 {
		return false
	}
	// 简单随机采样
	b := make([]byte, 1)
	rand.Read(b)
	return float64(b[0])/255.0 < t.sampleRate
}

// GetTrace 获取完整追踪链
func (t *Tracer) GetTrace(traceID string) []*TraceSpan {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*TraceSpan, 0)
	for _, span := range t.spans {
		if span.TraceID == traceID {
			result = append(result, span)
		}
	}
	return result
}

// GetRecentSpans 获取最近的跨度
func (t *Tracer) GetRecentSpans(limit int) []*TraceSpan {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if limit <= 0 || limit > len(t.spans) {
		limit = len(t.spans)
	}

	start := len(t.spans) - limit
	result := make([]*TraceSpan, limit)
	copy(result, t.spans[start:])
	return result
}

// AddEvent 添加追踪事件
func (t *Tracer) AddEvent(span *TraceSpan, name string, fields map[string]string) {
	span.Events = append(span.Events, TraceEvent{
		Name:      name,
		Timestamp: time.Now(),
		Fields:    fields,
	})
}

// GetStats 获取追踪统计
func (t *Tracer) GetStats() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return map[string]interface{}{
		"total_spans": len(t.spans),
		"sample_rate": t.sampleRate,
	}
}

// ClearSpans 清除所有跨度
func (t *Tracer) ClearSpans() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.spans = t.spans[:0]
}

// MetricsCollector 指标收集器
type MetricsCollector struct {
	mu      sync.RWMutex
	logger  *zap.Logger
	metrics []MetricPoint
	maxSize int
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector(logger *zap.Logger) *MetricsCollector {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &MetricsCollector{
		logger:  logger,
		metrics: make([]MetricPoint, 0),
		maxSize: 10000,
	}
}

// Record 记录指标
func (mc *MetricsCollector) Record(point MetricPoint) {
	if point.Timestamp.IsZero() {
		point.Timestamp = time.Now()
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	if len(mc.metrics) >= mc.maxSize {
		mc.metrics = mc.metrics[1:]
	}

	mc.metrics = append(mc.metrics, point)
}

// GetMetrics 获取所有指标
func (mc *MetricsCollector) GetMetrics(name string) []MetricPoint {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make([]MetricPoint, 0)
	for _, m := range mc.metrics {
		if name == "" || m.Name == name {
			result = append(result, m)
		}
	}
	return result
}

// GetMetricsSummary 获取指标摘要
func (mc *MetricsCollector) GetMetricsSummary() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	nameCounts := make(map[string]int)
	for _, m := range mc.metrics {
		nameCounts[m.Name]++
	}

	return map[string]interface{}{
		"total_points": len(mc.metrics),
		"metric_names": nameCounts,
	}
}

// RecordRequestMetrics 记录请求级指标
func (mc *MetricsCollector) RecordRequestMetrics(service string, status int, duration time.Duration) {
	mc.Record(MetricPoint{
		Name:  "http_requests_total",
		Type:  "counter",
		Value: 1,
		Labels: map[string]string{
			"service": service,
			"status":  httpStatusClass(status),
		},
	})

	mc.Record(MetricPoint{
		Name:  "http_request_duration_ms",
		Type:  "histogram",
		Value: float64(duration.Milliseconds()),
		Labels: map[string]string{
			"service": service,
		},
	})
}

// Clear 清除所有指标
func (mc *MetricsCollector) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics = mc.metrics[:0]
}

// generateTraceID 生成追踪 ID
func generateTraceID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// httpStatusClass HTTP 状态码分类
func httpStatusClass(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500:
		return "5xx"
	default:
		return "other"
	}
}
