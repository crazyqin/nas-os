package ransomware

import (
	"math"
	"sync"
	"time"
)

// ========== 熵值异常检测器 ==========

// EntropyDetectorConfig 熵值检测器配置.
type EntropyDetectorConfig struct {
	// 窗口大小：滑动窗口内保留的采样数
	WindowSize int `json:"window_size"`
	// 高熵阈值：超过此值视为可疑加密行为（默认7.0，对应近似均匀分布的字节数据）
	HighEntropyThreshold float64 `json:"high_entropy_threshold"`
	// 异常偏移：当前熵值偏离基线超过此倍数标准差时触发告警
	AnomalyStdDev float64 `json:"anomaly_std_dev"`
	// 采样间隔：两次采样之间的最小时间间隔
	SampleInterval time.Duration `json:"sample_interval"`
	// 最小采样数：至少积累此数量的采样后才开始检测
	MinSamples int `json:"min_samples"`
	// 批量写入阈值：时间窗口内高熵写入次数达到此值触发告警
	BatchThreshold int `json:"batch_threshold"`
	// 批量检测时间窗口
	BatchWindow time.Duration `json:"batch_window"`
	// 基线学习率：用于指数移动平均的alpha（0~1，越大越敏感）
	BaselineAlpha float64 `json:"baseline_alpha"`
	// 最大文件路径数：追踪的不同文件路径上限
	MaxPaths int `json:"max_paths"`
}

// DefaultEntropyDetectorConfig 返回默认熵值检测配置.
func DefaultEntropyDetectorConfig() EntropyDetectorConfig {
	return EntropyDetectorConfig{
		WindowSize:           100,
		HighEntropyThreshold: 7.0,
		AnomalyStdDev:        2.5,
		SampleInterval:       1 * time.Second,
		MinSamples:           5,
		BatchThreshold:       10,
		BatchWindow:          2 * time.Minute,
		BaselineAlpha:        0.05,
		MaxPaths:             5000,
	}
}

// EntropySample 熵值采样点.
type EntropySample struct {
	Path      string        `json:"path"`       // 文件路径
	Entropy   float64       `json:"entropy"`    // Shannon 熵值 (0~8)
	Timestamp time.Time     `json:"timestamp"`  // 采样时间
	Size      int64         `json:"size"`       // 文件大小
	Operation FileOperation `json:"operation"`  // 操作类型
}

// EntropyAlert 熵值告警.
type EntropyAlert struct {
	Timestamp   time.Time       `json:"timestamp"`
	AlertType   string          `json:"alert_type"`   // high_entropy, batch_detected, trend_anomaly
	Severity    ThreatLevel     `json:"severity"`
	Path        string          `json:"path"`
	Entropy     float64         `json:"entropy"`
	Baseline    float64         `json:"baseline"`  // 历史基线均值
	StdDev      float64         `json:"std_dev"`   // 历史标准差
	Description string          `json:"description"`
	Samples     []EntropySample `json:"samples,omitempty"` // 关联采样
}

// pathState 单个文件路径的追踪状态.
type pathState struct {
	samples   []EntropySample // 滑动窗口采样
	baseline  float64         // 指数移动平均基线
	variance  float64         // 指数移动平均方差
	count     int             // 总采样计数
	lastWrite time.Time       // 最后一次高熵写入时间
	highCount int             // 批量窗口内高熵写入计数
}

// EntropyDetector 基于信息熵的文件异常变化检测器.
type EntropyDetector struct {
	config    EntropyDetectorConfig
	paths     map[string]*pathState // path -> state
	mu        sync.RWMutex
	alertChan chan EntropyAlert
	stopCh    chan struct{}
	stopped   bool
	stats     EntropyDetectorStats
	statsMu   sync.RWMutex
}

// EntropyDetectorStats 检测器运行统计.
type EntropyDetectorStats struct {
	TotalSamples    int64   `json:"total_samples"`
	TotalAlerts     int64   `json:"total_alerts"`
	TrackedPaths    int     `json:"tracked_paths"`
	AvgEntropy      float64 `json:"avg_entropy"`       // 全局平均熵值
	HighEntropyHits int64   `json:"high_entropy_hits"` // 高熵事件计数
}

// NewEntropyDetector 创建熵值检测器.
func NewEntropyDetector(config EntropyDetectorConfig) *EntropyDetector {
	def := DefaultEntropyDetectorConfig()

	// 补全零值配置
	if config.WindowSize <= 0 {
		config.WindowSize = def.WindowSize
	}
	if config.HighEntropyThreshold <= 0 {
		config.HighEntropyThreshold = def.HighEntropyThreshold
	}
	if config.AnomalyStdDev <= 0 {
		config.AnomalyStdDev = def.AnomalyStdDev
	}
	// SampleInterval: 0 表示不限制采样频率，负值才使用默认值
	if config.SampleInterval < 0 {
		config.SampleInterval = def.SampleInterval
	}
	if config.MinSamples <= 0 {
		config.MinSamples = def.MinSamples
	}
	if config.BatchThreshold <= 0 {
		config.BatchThreshold = def.BatchThreshold
	}
	if config.BatchWindow <= 0 {
		config.BatchWindow = def.BatchWindow
	}
	if config.BaselineAlpha <= 0 || config.BaselineAlpha >= 1 {
		config.BaselineAlpha = def.BaselineAlpha
	}
	if config.MaxPaths <= 0 {
		config.MaxPaths = def.MaxPaths
	}

	return &EntropyDetector{
		config:    config,
		paths:     make(map[string]*pathState),
		alertChan: make(chan EntropyAlert, 100),
		stopCh:    make(chan struct{}),
	}
}

// Start 启动检测器后台清理协程.
func (ed *EntropyDetector) Start() {
	go ed.cleanupLoop()
}

// Stop 停止检测器.
func (ed *EntropyDetector) Stop() {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	if ed.stopped {
		return
	}
	ed.stopped = true
	close(ed.stopCh)
	close(ed.alertChan)
}

// Alerts 返回告警通道（只读）.
func (ed *EntropyDetector) Alerts() <-chan EntropyAlert {
	return ed.alertChan
}

// GetStats 获取检测器统计信息.
func (ed *EntropyDetector) GetStats() EntropyDetectorStats {
	ed.statsMu.RLock()
	defer ed.statsMu.RUnlock()

	stats := ed.stats
	ed.mu.RLock()
	stats.TrackedPaths = len(ed.paths)
	ed.mu.RUnlock()
	return stats
}

// OnFileChange 处理文件变更事件，计算熵值并检测异常.
// 返回值：如果检测到异常则返回告警，否则返回nil.
func (ed *EntropyDetector) OnFileChange(event FileEvent) *EntropyAlert {
	// 仅处理写入/修改/创建类事件
	if !isWriteOperation(event.Operation) {
		return nil
	}

	// 获取或计算文件熵值
	entropy := ed.extractEntropy(event)
	if entropy < 0 {
		return nil
	}

	// 创建采样点
	sample := EntropySample{
		Path:      event.Path,
		Entropy:   entropy,
		Timestamp: event.Timestamp,
		Size:      event.Size,
		Operation: event.Operation,
	}

	// 采样间隔过滤
	ed.mu.Lock()
	defer ed.mu.Unlock()

	state, exists := ed.paths[event.Path]
	if !exists {
		// 路径数上限控制
		if len(ed.paths) >= ed.config.MaxPaths {
			return nil
		}
		state = &pathState{
			baseline: entropy,
		}
		ed.paths[event.Path] = state
	}

	// 采样间隔检查：避免同一路径过于频繁采样
	if exists && len(state.samples) > 0 {
		last := state.samples[len(state.samples)-1]
		if sample.Timestamp.Sub(last.Timestamp) < ed.config.SampleInterval {
			return nil
		}
	}

	// 追加到滑动窗口
	state.samples = append(state.samples, sample)
	if len(state.samples) > ed.config.WindowSize {
		state.samples = state.samples[len(state.samples)-ed.config.WindowSize:]
	}
	state.count++

	// 更新自适应基线（指数移动平均）
	ed.updateBaseline(state, entropy)

	// 更新统计
	ed.statsMu.Lock()
	ed.stats.TotalSamples++
	if entropy > ed.config.HighEntropyThreshold {
		ed.stats.HighEntropyHits++
	}
	// 滚动更新全局平均
	n := float64(ed.stats.TotalSamples)
	ed.stats.AvgEntropy = ed.stats.AvgEntropy*(n-1)/n + entropy/n
	ed.statsMu.Unlock()

	// 检测异常
	if alert := ed.detectAnomaly(state, sample); alert != nil {
		ed.statsMu.Lock()
		ed.stats.TotalAlerts++
		ed.statsMu.Unlock()
		ed.emitAlert(*alert)
		return alert
	}

	return nil
}

// calculateEntropy 计算字节数据的Shannon信息熵.
// 熵值范围：0（完全确定）~ 8（完全随机，256个字节等概率分布）.
func (ed *EntropyDetector) calculateEntropy(data []byte) float64 {
	return CalculateEntropy(data)
}

// extractEntropy 从事件中提取熵值.
// 优先使用事件自带的entropy字段，否则使用全局计算函数.
func (ed *EntropyDetector) extractEntropy(event FileEvent) float64 {
	if event.Entropies != nil {
		if e, ok := event.Entropies["file"]; ok {
			return e
		}
		if e, ok := event.Entropies["entropy"]; ok {
			return e
		}
	}
	// 事件未携带熵值时返回-1，由调用方决定是否读取文件
	return -1
}

// updateBaseline 使用指数移动平均更新自适应基线和方差.
func (ed *EntropyDetector) updateBaseline(state *pathState, entropy float64) {
	alpha := ed.config.BaselineAlpha
	if state.count == 1 {
		// 第一个样本直接初始化
		state.baseline = entropy
		state.variance = 0
		return
	}
	// EMA for mean
	state.baseline = alpha*entropy + (1-alpha)*state.baseline
	// EMA for variance: var = alpha*(x-mean)^2 + (1-alpha)*var
	diff := entropy - state.baseline
	state.variance = alpha*diff*diff + (1-alpha)*state.variance
}

// detectAnomaly 检测单个路径的异常.
// 综合判断：绝对高熵 + 相对偏离基线 + 批量高熵写入.
func (ed *EntropyDetector) detectAnomaly(state *pathState, sample EntropySample) *EntropyAlert {
	cfg := ed.config

	// 采样数不足时不触发检测
	if state.count < cfg.MinSamples {
		return nil
	}

	now := sample.Timestamp

	// 1. 批量高熵写入检测
	if sample.Entropy >= cfg.HighEntropyThreshold {
		// 清理过期的高熵计数
		if now.Sub(state.lastWrite) > cfg.BatchWindow {
			state.highCount = 0
		}
		state.highCount++
		state.lastWrite = now

		if state.highCount >= cfg.BatchThreshold {
			return &EntropyAlert{
				Timestamp:   now,
				AlertType:   "batch_detected",
				Severity:    ThreatLevelCritical,
				Path:        sample.Path,
				Entropy:     sample.Entropy,
				Baseline:    state.baseline,
				StdDev:      math.Sqrt(state.variance),
				Description: "检测到批量高熵写入，疑似勒索软件加密行为",
				Samples:     recentSamples(state.samples, 10),
			}
		}
	}

	// 2. 单次绝对高熵检测
	if sample.Entropy >= cfg.HighEntropyThreshold {
		stddev := math.Sqrt(state.variance)
		// 仅在基线明显低于阈值时告警（避免正常高熵文件的误报）
		if state.baseline < cfg.HighEntropyThreshold-1.0 {
			return &EntropyAlert{
				Timestamp:   now,
				AlertType:   "high_entropy",
				Severity:    ThreatLevelHigh,
				Path:        sample.Path,
				Entropy:     sample.Entropy,
				Baseline:    state.baseline,
				StdDev:      stddev,
				Description: "文件熵值异常偏高，可能存在加密行为",
			}
		}
	}

	// 3. 趋势异常检测：熵值突然大幅上升
	if state.variance > 0 {
		stddev := math.Sqrt(state.variance)
		zScore := (sample.Entropy - state.baseline) / stddev
		if zScore >= cfg.AnomalyStdDev {
			return &EntropyAlert{
				Timestamp:   now,
				AlertType:   "trend_anomaly",
				Severity:    ThreatLevelHigh,
				Path:        sample.Path,
				Entropy:     sample.Entropy,
				Baseline:    state.baseline,
				StdDev:      stddev,
				Description: "文件熵值突变，偏离历史基线",
			}
		}
	}

	return nil
}

// analyzeTrend 分析指定路径的熵值变化趋势.
// 返回值：trend（趋势斜率，正值表示上升），currentEntropy（当前熵值），samples（窗口采样）.
func (ed *EntropyDetector) analyzeTrend(path string) (trend float64, currentEntropy float64, samples []EntropySample) {
	ed.mu.RLock()
	defer ed.mu.RUnlock()

	state, exists := ed.paths[path]
	if !exists || len(state.samples) < 2 {
		return 0, 0, nil
	}

	// 使用最近窗口数据做简单线性回归
	n := len(state.samples)
	samples = make([]EntropySample, n)
	copy(samples, state.samples)

	currentEntropy = samples[n-1].Entropy

	// 最小二乘法计算斜率
	var sumX, sumY, sumXY, sumX2 float64
	for i, s := range samples {
		x := float64(i)
		y := s.Entropy
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denom := float64(n)*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, currentEntropy, samples
	}
	trend = (float64(n)*sumXY - sumX*sumY) / denom

	return trend, currentEntropy, samples
}

// emitAlert 发送告警到通道（非阻塞）.
func (ed *EntropyDetector) emitAlert(alert EntropyAlert) {
	select {
	case ed.alertChan <- alert:
	default:
		// 通道满则丢弃，避免阻塞检测流程
	}
}

// cleanupLoop 定期清理长时间未活动的路径.
func (ed *EntropyDetector) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ed.stopCh:
			return
		case <-ticker.C:
			ed.cleanup()
		}
	}
}

// cleanup 清理超过10分钟未更新的路径.
func (ed *EntropyDetector) cleanup() {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for path, state := range ed.paths {
		if len(state.samples) == 0 {
			delete(ed.paths, path)
			continue
		}
		last := state.samples[len(state.samples)-1]
		if last.Timestamp.Before(cutoff) {
			delete(ed.paths, path)
		}
	}
}

// GetPathState 返回指定路径的当前状态（用于调试/测试）.
func (ed *EntropyDetector) GetPathState(path string) (baseline float64, variance float64, sampleCount int, ok bool) {
	ed.mu.RLock()
	defer ed.mu.RUnlock()

	state, exists := ed.paths[path]
	if !exists {
		return 0, 0, 0, false
	}
	return state.baseline, state.variance, len(state.samples), true
}

// ========== 辅助函数 ==========

// isWriteOperation 判断是否为写入类操作.
func isWriteOperation(op FileOperation) bool {
	switch op {
	case FileOpCreate, FileOpModify, FileOpWrite, FileOpTruncate:
		return true
	default:
		return false
	}
}

// recentSamples 获取最近n个采样.
func recentSamples(samples []EntropySample, n int) []EntropySample {
	if len(samples) <= n {
		out := make([]EntropySample, len(samples))
		copy(out, samples)
		return out
	}
	out := make([]EntropySample, n)
	copy(out, samples[len(samples)-n:])
	return out
}
