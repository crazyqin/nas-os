// Package healthpredictor 故障预测引擎
package healthpredictor

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// Predictor 故障预测器
type Predictor struct {
	mu     sync.RWMutex
	config HealthPredictorConfig
	preds  map[string]*Prediction
}

// NewPredictor 创建故障预测器
func NewPredictor(config HealthPredictorConfig) *Predictor {
	return &Predictor{
		config: config,
		preds:  make(map[string]*Prediction),
	}
}

// Predict 执行故障预测
func (p *Predictor) Predict(collector *Collector) []Prediction {
	var predictions []Prediction

	// 磁盘故障预测
	if pred := p.predictDiskFailure(collector); pred != nil {
		predictions = append(predictions, *pred)
	}

	// 内存泄漏预测
	if pred := p.predictMemoryLeak(collector); pred != nil {
		predictions = append(predictions, *pred)
	}

	// CPU 饱和预测
	if pred := p.predictCPUSaturation(collector); pred != nil {
		predictions = append(predictions, *pred)
	}

	// 磁盘空间不足预测
	if pred := p.predictDiskFull(collector); pred != nil {
		predictions = append(predictions, *pred)
	}

	// 网络流量异常预测
	if pred := p.predictNetworkSpike(collector); pred != nil {
		predictions = append(predictions, *pred)
	}

	// 存储预测结果
	p.mu.Lock()
	for i := range predictions {
		p.preds[predictions[i].ID] = &predictions[i]
	}
	// 清理过期预测
	p.cleanupOldPredictions()
	p.mu.Unlock()

	if len(predictions) > 0 {
		log.Printf("[HealthPredictor] 生成 %d 个故障预测", len(predictions))
	}

	return predictions
}

// predictDiskFailure 预测磁盘故障
// 基于磁盘温度趋势和 SMART 数据
func (p *Predictor) predictDiskFailure(collector *Collector) *Prediction {
	series := collector.GetTimeSeries(MetricDiskTemp)
	if series == nil || len(series.Points) < 20 {
		return nil
	}

	// 计算温度趋势
	slope := calcSlope(series.Points)
	latest := series.Points[len(series.Points)-1]
	mean := calcMean(series.Points)

	// 温度持续上升且接近阈值
	if slope > 0.1 && latest.Value > p.config.Thresholds.DiskTempWarning {
		// 预测到达临界温度的时间
		timeToCritical := (p.config.Thresholds.DiskTempCritical - latest.Value) / slope
		if timeToCritical < 0 {
			timeToCritical = 0
		}

		prob := math.Min(0.9, 0.3+slope*0.1+(latest.Value-mean)/20.0)
		if prob < 0.1 {
			prob = 0.1
		}

		return &Prediction{
			ID:            fmt.Sprintf("pred-disk-fail-%d", time.Now().UnixNano()),
			Type:          PredDiskFailure,
			Severity:      HealthPoor,
			Probability:   prob,
			Confidence:    0.7,
			MetricType:    MetricDiskTemp,
			CurrentValue:  latest.Value,
			PredictedValue: latest.Value + slope*100,
			TimeToImpact:  time.Duration(timeToCritical) * time.Minute,
			Description:   fmt.Sprintf("磁盘温度持续上升 (%.1f°C → 趋势 %.2f°C/min)，可能预示磁盘故障", latest.Value, slope),
			Suggestions:   []string{"检查磁盘 SMART 数据", "确保散热良好", "准备磁盘更换", "备份关键数据"},
			CreatedAt:     time.Now(),
		}
	}

	return nil
}

// predictMemoryLeak 预测内存泄漏
func (p *Predictor) predictMemoryLeak(collector *Collector) *Prediction {
	series := collector.GetTimeSeries(MetricMemoryUsage)
	if series == nil || len(series.Points) < 30 {
		return nil
	}

	// 分析内存使用趋势
	slope := calcSlope(series.Points)
	latest := series.Points[len(series.Points)-1]

	// 内存持续增长
	if slope > 0.05 && latest.Value > 60 {
		// 计算到达 100% 的时间
		timeToFull := (100 - latest.Value) / slope
		if timeToFull < 0 {
			timeToFull = 0
		}

		// 检查是否有周期性回收模式
		hasGC := p.detectGCCycle(series.Points)
		confidence := 0.6
		if hasGC {
			confidence = 0.4 // GC 会缓解泄漏
		}

		prob := math.Min(0.85, 0.2+slope*0.05+(latest.Value-60)/100.0)
		if prob < 0.1 {
			prob = 0.1
		}

		return &Prediction{
			ID:            fmt.Sprintf("pred-mem-leak-%d", time.Now().UnixNano()),
			Type:          PredMemoryLeak,
			Severity:      HealthFair,
			Probability:   prob,
			Confidence:    confidence,
			MetricType:    MetricMemoryUsage,
			CurrentValue:  latest.Value,
			PredictedValue: 100,
			TimeToImpact:  time.Duration(timeToFull) * time.Minute,
			Description:   fmt.Sprintf("内存使用率持续上升 (%.1f%%, 趋势 +%.2f%%/min)，疑似内存泄漏", latest.Value, slope),
			Suggestions:   []string{"检查应用内存使用", "重启高内存服务", "配置内存限制", "检查容器资源限制"},
			CreatedAt:     time.Now(),
		}
	}

	return nil
}

// predictCPUSaturation 预测 CPU 饱和
func (p *Predictor) predictCPUSaturation(collector *Collector) *Prediction {
	series := collector.GetTimeSeries(MetricCPUUsage)
	if series == nil || len(series.Points) < 20 {
		return nil
	}

	slope := calcSlope(series.Points)
	latest := series.Points[len(series.Points)-1]

	// CPU 使用率持续上升
	if slope > 0.1 && latest.Value > p.config.Thresholds.CPUWarning {
		timeToFull := (100 - latest.Value) / slope
		prob := math.Min(0.8, 0.2+slope*0.08)

		return &Prediction{
			ID:            fmt.Sprintf("pred-cpu-sat-%d", time.Now().UnixNano()),
			Type:          PredCPUSaturation,
			Severity:      HealthFair,
			Probability:   prob,
			Confidence:    0.65,
			MetricType:    MetricCPUUsage,
			CurrentValue:  latest.Value,
			PredictedValue: 100,
			TimeToImpact:  time.Duration(timeToFull) * time.Minute,
			Description:   fmt.Sprintf("CPU 使用率持续上升 (%.1f%%, 趋势 +%.2f%%/min)，系统可能饱和", latest.Value, slope),
			Suggestions:   []string{"检查高 CPU 进程", "考虑限流", "优化计算密集型任务", "增加 CPU 资源"},
			CreatedAt:     time.Now(),
		}
	}

	return nil
}

// predictDiskFull 预测磁盘空间不足
func (p *Predictor) predictDiskFull(collector *Collector) *Prediction {
	series := collector.GetTimeSeries(MetricDiskUsage)
	if series == nil || len(series.Points) < 20 {
		return nil
	}

	slope := calcSlope(series.Points)
	latest := series.Points[len(series.Points)-1]

	if slope > 0.01 && latest.Value > p.config.Thresholds.DiskWarning {
		timeToFull := (100 - latest.Value) / slope
		prob := math.Min(0.9, 0.3+slope*0.1+(latest.Value-80)/50.0)

		severity := HealthFair
		if latest.Value >= p.config.Thresholds.DiskCritical {
			severity = HealthPoor
		}

		return &Prediction{
			ID:            fmt.Sprintf("pred-disk-full-%d", time.Now().UnixNano()),
			Type:          PredDiskFull,
			Severity:      severity,
			Probability:   prob,
			Confidence:    0.8,
			MetricType:    MetricDiskUsage,
			CurrentValue:  latest.Value,
			PredictedValue: 100,
			TimeToImpact:  time.Duration(timeToFull) * time.Minute,
			Description:   fmt.Sprintf("磁盘使用率持续上升 (%.1f%%, 趋势 +%.2f%%/min)，预计 %.0f 分钟后空间耗尽", latest.Value, slope, timeToFull),
			Suggestions:   []string{"清理日志文件", "删除临时文件", "扩展存储空间", "归档旧数据"},
			CreatedAt:     time.Now(),
		}
	}

	return nil
}

// predictNetworkSpike 预测网络流量异常
func (p *Predictor) predictNetworkSpike(collector *Collector) *Prediction {
	series := collector.GetTimeSeries(MetricNetworkIn)
	if series == nil || len(series.Points) < 20 {
		return nil
	}

	latest := series.Points[len(series.Points)-1]
	mean := calcMean(series.Points)

	// 检查流量突增
	if mean > 0 && latest.Value > mean*p.config.Thresholds.NetSpikePercent/100 {
		return &Prediction{
			ID:            fmt.Sprintf("pred-net-spike-%d", time.Now().UnixNano()),
			Type:          PredNetworkSpike,
			Severity:      HealthFair,
			Probability:   0.6,
			Confidence:    0.5,
			MetricType:    MetricNetworkIn,
			CurrentValue:  latest.Value,
			PredictedValue: latest.Value * 1.5,
			TimeToImpact:  5 * time.Minute,
			Description:   fmt.Sprintf("网络入站流量突增 (%.1f KB/s, 均值 %.1f KB/s)", latest.Value/1024, mean/1024),
			Suggestions:   []string{"检查异常连接", "检查 DDoS 攻击", "检查大文件传输", "配置流量限制"},
			CreatedAt:     time.Now(),
		}
	}

	return nil
}

// detectGCCycle 检测 GC 周期性回收模式
func (p *Predictor) detectGCCycle(points []MetricPoint) bool {
	if len(points) < 30 {
		return false
	}

	// 检查是否有周期性的下降（GC 回收）
	drops := 0
	for i := 1; i < len(points); i++ {
		if points[i].Value < points[i-1].Value-2.0 {
			drops++
		}
	}

	// 如果有多个下降点，可能有 GC
	return drops >= 3
}

// GetPredictions 获取所有预测
func (p *Predictor) GetPredictions() []Prediction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]Prediction, 0, len(p.preds))
	for _, pred := range p.preds {
		result = append(result, *pred)
	}
	return result
}

// GetPrediction 获取指定预测
func (p *Predictor) GetPrediction(id string) (*Prediction, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	pred, ok := p.preds[id]
	if !ok {
		return nil, false
	}
	return pred, true
}

// cleanupOldPredictions 清理过期预测（超过 1 小时）
func (p *Predictor) cleanupOldPredictions() {
	cutoff := time.Now().Add(-1 * time.Hour)
	for id, pred := range p.preds {
		if pred.CreatedAt.Before(cutoff) {
			delete(p.preds, id)
		}
	}
}
