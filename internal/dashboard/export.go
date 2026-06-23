package dashboard

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ExportFormat 导出格式.
type ExportFormat string

const (
	ExportJSON ExportFormat = "json"
	ExportCSV  ExportFormat = "csv"
)

// ExportRequest 导出请求.
type ExportRequest struct {
	Format    ExportFormat `json:"format"`
	Metrics   []string     `json:"metrics"`   // cpu/memory/disk/network/all
	StartTime time.Time    `json:"startTime"`
	EndTime   time.Time    `json:"endTime"`
	Limit     int          `json:"limit"`
}

// MetricSample 指标采样数据.
type MetricSample struct {
	Timestamp time.Time              `json:"timestamp"`
	Metric    string                 `json:"metric"`
	Values    map[string]interface{} `json:"values"`
}

// ExportResult 导出结果.
type ExportResult struct {
	Format    ExportFormat `json:"format"`
	Count     int          `json:"count"`
	Data      []byte       `json:"-"`
	StartTime time.Time    `json:"startTime"`
	EndTime   time.Time    `json:"endTime"`
}

// MetricsExporter 指标数据导出器.
type MetricsExporter struct {
	mu       sync.RWMutex
	samples  []MetricSample
	maxSize  int
}

// NewMetricsExporter 创建指标导出器.
func NewMetricsExporter(maxSamples int) *MetricsExporter {
	if maxSamples <= 0 {
		maxSamples = 10000
	}
	return &MetricsExporter{
		samples: make([]MetricSample, 0),
		maxSize: maxSamples,
	}
}

// Record 记录指标数据.
func (me *MetricsExporter) Record(sample MetricSample) {
	me.mu.Lock()
	defer me.mu.Unlock()
	if sample.Timestamp.IsZero() {
		sample.Timestamp = time.Now()
	}
	me.samples = append(me.samples, sample)
	if len(me.samples) > me.maxSize {
		me.samples = me.samples[len(me.samples)-me.maxSize:]
	}
}

// Export 导出数据.
func (me *MetricsExporter) Export(req ExportRequest) (*ExportResult, error) {
	me.mu.RLock()
	defer me.mu.RUnlock()

	// 过滤数据
	filtered := me.filterSamples(req.Metrics, req.StartTime, req.EndTime)
	if req.Limit > 0 && req.Limit < len(filtered) {
		filtered = filtered[len(filtered)-req.Limit:]
	}

	if len(filtered) == 0 {
		return &ExportResult{
			Format:    req.Format,
			Count:     0,
			Data:      []byte("[]"),
			StartTime: req.StartTime,
			EndTime:   req.EndTime,
		}, nil
	}

	var data []byte
	var err error

	switch req.Format {
	case ExportJSON:
		data, err = json.MarshalIndent(filtered, "", "  ")
	case ExportCSV:
		data, err = me.toCSV(filtered)
	default:
		return nil, fmt.Errorf("不支持的导出格式: %s", req.Format)
	}

	if err != nil {
		return nil, fmt.Errorf("导出失败: %w", err)
	}

	return &ExportResult{
		Format:    req.Format,
		Count:     len(filtered),
		Data:      data,
		StartTime: filtered[0].Timestamp,
		EndTime:   filtered[len(filtered)-1].Timestamp,
	}, nil
}

// filterSamples 按条件过滤.
func (me *MetricsExporter) filterSamples(metrics []string, start, end time.Time) []MetricSample {
	var result []MetricSample
	metricSet := make(map[string]bool)
	allMetrics := len(metrics) == 0
	for _, m := range metrics {
		metricSet[m] = true
	}

	for _, s := range me.samples {
		if !allMetrics && !metricSet[s.Metric] {
			continue
		}
		if !start.IsZero() && s.Timestamp.Before(start) {
			continue
		}
		if !end.IsZero() && s.Timestamp.After(end) {
			continue
		}
		result = append(result, s)
	}
	return result
}

// toCSV 转换为 CSV 格式.
func (me *MetricsExporter) toCSV(samples []MetricSample) ([]byte, error) {
	var buf strings.Builder
	w := csv.NewWriter(&buf)

	// 收集所有字段名
	header := []string{"timestamp", "metric"}
	fieldSet := make(map[string]bool)
	for _, s := range samples {
		for k := range s.Values {
			if !fieldSet[k] {
				fieldSet[k] = true
				header = append(header, k)
			}
		}
	}
	w.Write(header)

	// 写入数据行
	for _, s := range samples {
		row := []string{s.Timestamp.Format(time.RFC3339), s.Metric}
		for _, h := range header[2:] {
			val, ok := s.Values[h]
			if !ok {
				row = append(row, "")
			} else {
				row = append(row, fmt.Sprintf("%v", val))
			}
		}
		w.Write(row)
	}
	w.Flush()
	return []byte(buf.String()), w.Error()
}

// SampleCount 返回采样数.
func (me *MetricsExporter) SampleCount() int {
	me.mu.RLock()
	defer me.mu.RUnlock()
	return len(me.samples)
}

// Clear 清除所有采样.
func (me *MetricsExporter) Clear() {
	me.mu.Lock()
	defer me.mu.Unlock()
	me.samples = me.samples[:0]
}
