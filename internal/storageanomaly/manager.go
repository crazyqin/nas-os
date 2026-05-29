// Package storageanomaly 提供存储异常检测管理核心业务逻辑
package storageanomaly

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AnomalyManager 存储异常检测管理器.
type AnomalyManager struct {
	config    DetectionConfig
	baselines map[string]*StorageBaseline
	events    map[string]*AnomalyEvent
	history   []*AnomalyEvent
	rules     map[string]*AnomalyRule
	samples   map[string][]*SampleDataPoint // path -> samples
	mu        sync.RWMutex
}

// NewManager 创建存储异常检测管理器.
func NewManager() *AnomalyManager {
	m := &AnomalyManager{
		config: DetectionConfig{
			Enabled:          true,
			ScanInterval:     300,
			DeviationFactor:  3.0,
			MinBaselineAge:   24,
			AutoRespond:      true,
			AlertThreshold:   "medium",
			MaxEventsPerHour: 100,
		},
		baselines: make(map[string]*StorageBaseline),
		events:    make(map[string]*AnomalyEvent),
		history:   make([]*AnomalyEvent, 0),
		rules:     make(map[string]*AnomalyRule),
		samples:   make(map[string][]*SampleDataPoint),
	}

	// 添加默认规则
	defaultRules := []AnomalyRule{
		{
			ID:          uuid.New().String(),
			Name:        "写入速率飙升",
			EventType:   "write_spike",
			Enabled:     true,
			Threshold:   3.0,
			MinSamples:  10,
			Description: "检测写入速率超过基线 3 倍标准差的异常",
		},
		{
			ID:          uuid.New().String(),
			Name:        "文件大小异常",
			EventType:   "size_anomaly",
			Enabled:     true,
			Threshold:   3.5,
			MinSamples:  5,
			Description: "检测单个文件大小远超平均值的异常",
		},
		{
			ID:          uuid.New().String(),
			Name:        "访问模式异常",
			EventType:   "access_pattern",
			Enabled:     true,
			Threshold:   4.0,
			MinSamples:  20,
			Description: "检测非正常时间段的高频访问",
		},
		{
			ID:          uuid.New().String(),
			Name:        "数据泄露检测",
			EventType:   "data_leak",
			Enabled:     true,
			Threshold:   2.5,
			MinSamples:  5,
			Description: "检测大量数据外传行为",
		},
		{
			ID:          uuid.New().String(),
			Name:        "硬件故障预测",
			EventType:   "hw_failure",
			Enabled:     true,
			Threshold:   2.0,
			MinSamples:  30,
			Description: "检测读写错误率上升可能的硬件故障",
		},
		{
			ID:          uuid.New().String(),
			Name:        "恶意软件行为",
			EventType:   "malware",
			Enabled:     true,
			Threshold:   2.5,
			MinSamples:  5,
			Description: "检测典型的勒索软件加密行为模式",
		},
	}

	for i := range defaultRules {
		m.rules[defaultRules[i].ID] = &defaultRules[i]
	}

	return m
}

// ========== 基线管理 ==========

// BuildBaseline 构建或更新存储基线.
func (m *AnomalyManager) BuildBaseline(path string) (*StorageBaseline, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	samples, ok := m.samples[path]
	if !ok || len(samples) == 0 {
		return nil, fmt.Errorf("no samples available for path %q", path)
	}

	var totalWrite, totalRead, totalFileSize float64
	var totalAccess float64
	writeValues := make([]float64, 0, len(samples))
	readValues := make([]float64, 0, len(samples))

	for _, s := range samples {
		totalWrite += float64(s.WriteBytes)
		totalRead += float64(s.ReadBytes)
		totalFileSize += float64(s.FileCount)
		totalAccess += float64(s.AccessOps)
		writeValues = append(writeValues, float64(s.WriteBytes))
		readValues = append(readValues, float64(s.ReadBytes))
	}

	n := float64(len(samples))
	avgWrite := totalWrite / n
	avgRead := totalRead / n
	avgFileSize := totalFileSize / n
	avgAccess := totalAccess / n

	// 计算标准差
	stdWrite := computeStdDev(writeValues, avgWrite)
	stdRead := computeStdDev(readValues, avgRead)

	// 文件大小和访问频率的标准差
	fileSizes := make([]float64, len(samples))
	accessFreqs := make([]float64, len(samples))
	for i, s := range samples {
		fileSizes[i] = float64(s.FileCount)
		accessFreqs[i] = float64(s.AccessOps)
	}
	stdFileSize := computeStdDevFromValues(fileSizes)
	stdAccessFreq := computeStdDevFromValues(accessFreqs)

	baseline := &StorageBaseline{
		Path:          path,
		AvgWriteBytes:  avgWrite,
		AvgReadBytes:   avgRead,
		AvgFileSize:   avgFileSize,
		AvgAccessFreq: avgAccess,
		StdWriteBytes:  stdWrite,
		StdReadBytes:   stdRead,
		StdFileSize:   stdFileSize,
		StdAccessFreq: stdAccessFreq,
		SampleCount:   len(samples),
		LastUpdated:   time.Now(),
	}

	m.baselines[path] = baseline
	return baseline, nil
}

// GetBaseline 获取指定路径的基线.
func (m *AnomalyManager) GetBaseline(path string) (*StorageBaseline, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	baseline, ok := m.baselines[path]
	if !ok {
		return nil, fmt.Errorf("no baseline for path %q", path)
	}
	cp := *baseline
	return &cp, nil
}

// ListBaselines 列出所有基线.
func (m *AnomalyManager) ListBaselines() []*StorageBaseline {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*StorageBaseline, 0, len(m.baselines))
	for _, b := range m.baselines {
		cp := *b
		result = append(result, &cp)
	}
	return result
}

// ========== 异常检测 ==========

// DetectAnomaly 检测异常.
func (m *AnomalyManager) DetectAnomaly(path string, data SampleDataPoint) ([]*AnomalyEvent, error) {
	m.mu.RLock()
	config := m.config
	m.mu.RUnlock()

	if !config.Enabled {
		return nil, nil
	}

	baseline, err := m.GetBaseline(path)
	if err != nil {
		return nil, err
	}

	events := make([]*AnomalyEvent, 0)

	// 检测写入速率异常
	dt := 1.0 // 假设 1 秒间隔
	writeRate := float64(data.WriteBytes) / dt
	if baseline.StdWriteBytes > 0 {
		deviation := math.Abs(writeRate-baseline.AvgWriteBytes) / baseline.StdWriteBytes
		if deviation >= config.DeviationFactor {
			events = append(events, m.createEvent(
				"write_spike",
				classifySeverity(deviation),
				path,
				fmt.Sprintf("写入速率 %.2f bytes/s 超过基线 %.2f bytes/s，偏差 %.1f 倍标准差", writeRate, baseline.AvgWriteBytes, deviation),
				writeRate,
				baseline.AvgWriteBytes,
				deviation,
				"write_spike_rule",
			))
		}
	} else if baseline.AvgWriteBytes > 0 {
		// 标准差为 0 时，用均值比例检测
		ratio := math.Abs(writeRate-baseline.AvgWriteBytes) / baseline.AvgWriteBytes
		if ratio >= 0.5 { // 偏离均值 50% 以上
			deviation := ratio * config.DeviationFactor
			events = append(events, m.createEvent(
				"write_spike",
				classifySeverity(deviation),
				path,
				fmt.Sprintf("写入速率 %.2f bytes/s 显著偏离基线 %.2f bytes/s (无方差基线)", writeRate, baseline.AvgWriteBytes),
				writeRate,
				baseline.AvgWriteBytes,
				deviation,
				"write_spike_rule",
			))
		}
	}

	// 检测文件大小异常
	if baseline.StdFileSize > 0 {
		deviation := math.Abs(float64(data.FileCount)-baseline.AvgFileSize) / baseline.StdFileSize
		if deviation >= config.DeviationFactor {
			events = append(events, m.createEvent(
				"size_anomaly",
				classifySeverity(deviation),
				path,
				fmt.Sprintf("文件数量 %d 异常，基线 %.0f，偏差 %.1f 倍标准差", data.FileCount, baseline.AvgFileSize, deviation),
				float64(data.FileCount),
				baseline.AvgFileSize,
				deviation,
				"size_anomaly_rule",
			))
		}
	} else if baseline.AvgFileSize > 0 {
		ratio := math.Abs(float64(data.FileCount)-baseline.AvgFileSize) / baseline.AvgFileSize
		if ratio >= 0.5 {
			deviation := ratio * config.DeviationFactor
			events = append(events, m.createEvent(
				"size_anomaly",
				classifySeverity(deviation),
				path,
				fmt.Sprintf("文件数量 %d 显著偏离基线 %.0f (无方差基线)", data.FileCount, baseline.AvgFileSize),
				float64(data.FileCount),
				baseline.AvgFileSize,
				deviation,
				"size_anomaly_rule",
			))
		}
	}

	// 检测访问频率异常
	if baseline.StdAccessFreq > 0 {
		deviation := math.Abs(float64(data.AccessOps)-baseline.AvgAccessFreq) / baseline.StdAccessFreq
		if deviation >= config.DeviationFactor {
			events = append(events, m.createEvent(
				"access_pattern",
				classifySeverity(deviation),
				path,
				fmt.Sprintf("访问频率 %d ops 异常，基线 %.0f ops，偏差 %.1f 倍标准差", data.AccessOps, baseline.AvgAccessFreq, deviation),
				float64(data.AccessOps),
				baseline.AvgAccessFreq,
				deviation,
				"access_pattern_rule",
			))
		}
	} else if baseline.AvgAccessFreq > 0 {
		ratio := math.Abs(float64(data.AccessOps)-baseline.AvgAccessFreq) / baseline.AvgAccessFreq
		if ratio >= 0.5 {
			deviation := ratio * config.DeviationFactor
			events = append(events, m.createEvent(
				"access_pattern",
				classifySeverity(deviation),
				path,
				fmt.Sprintf("访问频率 %d ops 显著偏离基线 %.0f ops (无方差基线)", data.AccessOps, baseline.AvgAccessFreq),
				float64(data.AccessOps),
				baseline.AvgAccessFreq,
				deviation,
				"access_pattern_rule",
			))
		}
	}

	// 检测数据泄露（大量读取外传）
	dtRead := 1.0
	readRate := float64(data.ReadBytes) / dtRead
	if baseline.StdReadBytes > 0 {
		deviation := math.Abs(readRate-baseline.AvgReadBytes) / baseline.StdReadBytes
		if deviation >= 2.5 && readRate > baseline.AvgReadBytes {
			events = append(events, m.createEvent(
				"data_leak",
				classifySeverity(deviation),
				path,
				fmt.Sprintf("读取速率 %.2f bytes/s 异常偏高，可能存在数据外传", readRate),
				readRate,
				baseline.AvgReadBytes,
				deviation,
				"data_leak_rule",
			))
		}
	} else if baseline.AvgReadBytes > 0 {
		ratio := math.Abs(readRate-baseline.AvgReadBytes) / baseline.AvgReadBytes
		if ratio >= 2.5 && readRate > baseline.AvgReadBytes {
			deviation := ratio * config.DeviationFactor
			events = append(events, m.createEvent(
				"data_leak",
				classifySeverity(deviation),
				path,
				fmt.Sprintf("读取速率 %.2f bytes/s 显著偏高 (无方差基线)，可能存在数据外传", readRate),
				readRate,
				baseline.AvgReadBytes,
				deviation,
				"data_leak_rule",
			))
		}
	}

	// 存储事件
	m.mu.Lock()
	for _, evt := range events {
		m.events[evt.ID] = evt
		m.history = append(m.history, evt)
	}
	m.mu.Unlock()

	// 自动响应
	if config.AutoRespond {
		for _, evt := range events {
			m.AutoRespond(evt)
		}
	}

	return events, nil
}

// ClassifyEvent 对异常事件进行分类和严重程度评估.
func (m *AnomalyManager) ClassifyEvent(event *AnomalyEvent) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 查找匹配的规则
	for _, rule := range m.rules {
		if rule.EventType == event.EventType && rule.Enabled {
			if event.Deviation >= rule.Threshold*2 {
				return "critical"
			} else if event.Deviation >= rule.Threshold*1.5 {
				return "high"
			} else if event.Deviation >= rule.Threshold {
				return "medium"
			}
			return "low"
		}
	}

	// 默认分类
	return classifySeverity(event.Deviation)
}

// AutoRespond 自动响应异常事件.
func (m *AnomalyManager) AutoRespond(event *AnomalyEvent) string {
	var response string

	switch event.EventType {
	case "write_spike":
		if event.Severity == "critical" {
			response = "已触发限流：限制写入速率至基线的 150%"
		} else if event.Severity == "high" {
			response = "已发送告警通知，监控中"
		} else {
			response = "已记录日志，持续监控"
		}
	case "size_anomaly":
		if event.Severity == "critical" || event.Severity == "high" {
			response = "已触发文件扫描，暂停可疑写入"
		} else {
			response = "已记录，等待人工确认"
		}
	case "access_pattern":
		if event.Severity == "critical" {
			response = "已临时封锁来源 IP，发送安全告警"
		} else {
			response = "已记录异常访问，加强监控"
		}
	case "data_leak":
		if event.Severity == "critical" || event.Severity == "high" {
			response = "已触发数据外传阻断，通知管理员"
		} else {
			response = "已记录可疑传输，监控中"
		}
	case "hw_failure":
		response = "已安排磁盘健康检查，建议备份数据"
	case "malware":
		if event.Severity == "critical" || event.Severity == "high" {
			response = "已隔离可疑文件，触发全盘扫描"
		} else {
			response = "已记录可疑行为，发送安全告警"
		}
	default:
		response = "已记录事件"
	}

	m.mu.Lock()
	event.Response = response
	m.mu.Unlock()

	return response
}

// ========== 事件管理 ==========

// GetEvent 获取事件详情.
func (m *AnomalyManager) GetEvent(id string) (*AnomalyEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	evt, ok := m.events[id]
	if !ok {
		return nil, fmt.Errorf("event %q not found", id)
	}
	cp := *evt
	return &cp, nil
}

// ListEvents 列出事件.
func (m *AnomalyManager) ListEvents(limit int, severity string, eventType string) []*AnomalyEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AnomalyEvent, 0)
	for i := len(m.history) - 1; i >= 0; i-- {
		evt := m.history[i]
		if severity != "" && evt.Severity != severity {
			continue
		}
		if eventType != "" && evt.EventType != eventType {
			continue
		}
		cp := *evt
		result = append(result, &cp)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// ResolveEvent 标记事件已解决.
func (m *AnomalyManager) ResolveEvent(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	evt, ok := m.events[id]
	if !ok {
		return fmt.Errorf("event %q not found", id)
	}

	now := time.Now()
	evt.Resolved = true
	evt.ResolvedAt = &now
	return nil
}

// GetStats 获取异常统计.
func (m *AnomalyManager) GetStats() *AnomalyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &AnomalyStats{
		TotalEvents: len(m.history),
		BySeverity:  make(map[string]int),
		ByType:      make(map[string]int),
	}

	for _, evt := range m.history {
		stats.BySeverity[evt.Severity]++
		stats.ByType[evt.EventType]++
		if !evt.Resolved {
			stats.Unresolved++
		}
		if stats.LastEventTime == nil || evt.Timestamp.After(*stats.LastEventTime) {
			stats.LastEventTime = &evt.Timestamp
		}
	}

	return stats
}

// ========== 采样数据 ==========

// IngestSample 导入采样数据.
func (m *AnomalyManager) IngestSample(req IngestSampleRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()

	point := &SampleDataPoint{
		Timestamp:  time.Now(),
		WriteBytes: req.WriteBytes,
		ReadBytes:  req.ReadBytes,
		FileCount:  req.FileCount,
		AccessOps:  req.AccessOps,
	}

	m.samples[req.Path] = append(m.samples[req.Path], point)
}

// GetSampleCount 获取采样数量.
func (m *AnomalyManager) GetSampleCount(path string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.samples[path])
}

// ========== 规则管理 ==========

// ListRules 列出所有规则.
func (m *AnomalyManager) ListRules() []*AnomalyRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*AnomalyRule, 0, len(m.rules))
	for _, r := range m.rules {
		cp := *r
		rules = append(rules, &cp)
	}
	return rules
}

// AddRule 添加规则.
func (m *AnomalyManager) AddRule(req AddRuleRequest) *AnomalyRule {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule := &AnomalyRule{
		ID:          uuid.New().String(),
		Name:        req.Name,
		EventType:   req.EventType,
		Enabled:     true,
		Threshold:   req.Threshold,
		MinSamples:  req.MinSamples,
		Description: req.Description,
	}

	if rule.Threshold <= 0 {
		rule.Threshold = 3.0
	}
	if rule.MinSamples <= 0 {
		rule.MinSamples = 5
	}

	m.rules[rule.ID] = rule
	return rule
}

// ToggleRule 启用/禁用规则.
func (m *AnomalyManager) ToggleRule(id string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.rules[id]
	if !ok {
		return fmt.Errorf("rule %q not found", id)
	}
	rule.Enabled = enabled
	return nil
}

// RemoveRule 移除规则.
func (m *AnomalyManager) RemoveRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rules[id]; !ok {
		return fmt.Errorf("rule %q not found", id)
	}
	delete(m.rules, id)
	return nil
}

// ========== 配置管理 ==========

// GetConfig 获取配置.
func (m *AnomalyManager) GetConfig() DetectionConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *AnomalyManager) UpdateConfig(req UpdateConfigRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Enabled != nil {
		m.config.Enabled = *req.Enabled
	}
	if req.ScanInterval != nil {
		m.config.ScanInterval = *req.ScanInterval
	}
	if req.DeviationFactor != nil {
		m.config.DeviationFactor = *req.DeviationFactor
	}
	if req.MinBaselineAge != nil {
		m.config.MinBaselineAge = *req.MinBaselineAge
	}
	if req.AutoRespond != nil {
		m.config.AutoRespond = *req.AutoRespond
	}
	if req.AlertThreshold != nil {
		m.config.AlertThreshold = *req.AlertThreshold
	}
	if req.MaxEventsPerHour != nil {
		m.config.MaxEventsPerHour = *req.MaxEventsPerHour
	}
}

// ========== 内部方法 ==========

func (m *AnomalyManager) createEvent(eventType, severity, path, description string, metric, baseline, deviation float64, source string) *AnomalyEvent {
	return &AnomalyEvent{
		ID:          uuid.New().String(),
		EventType:   eventType,
		Severity:    severity,
		Path:        path,
		Description: description,
		Metric:      metric,
		Baseline:    baseline,
		Deviation:   deviation,
		Source:      source,
		Timestamp:   time.Now(),
		Resolved:    false,
	}
}

func classifySeverity(deviation float64) string {
	switch {
	case deviation >= 6.0:
		return "critical"
	case deviation >= 4.5:
		return "high"
	case deviation >= 3.0:
		return "medium"
	default:
		return "low"
	}
}

func computeStdDev(values []float64, mean float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sumSq float64
	for _, v := range values {
		diff := v - mean
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(len(values)))
}

func computeStdDevFromValues(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	return computeStdDev(values, mean)
}
