// Package aidiskhealth AI 磁盘健康预测
// 基于机器学习的磁盘故障预测与健康评分系统
package aidiskhealth

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// HealthStatus 健康状态
type HealthStatus string

const (
	StatusHealthy     HealthStatus = "healthy"     // 健康
	StatusWarning     HealthStatus = "warning"     // 警告
	StatusCritical    HealthStatus = "critical"    // 危险
	StatusFailed      HealthStatus = "failed"      // 故障
	StatusPredicted   HealthStatus = "predicted"   // 预测故障
)

// DiskType 磁盘类型
type DiskType string

const (
	DiskTypeHDD  DiskType = "hdd"
	DiskTypeSSD  DiskType = "ssd"
	DiskTypeNVMe DiskType = "nvme"
)

// SMARTAttribute SMART 属性
type SMARTAttribute struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Value      int    `json:"value"`
	Worst      int    `json:"worst"`
	Threshold  int    `json:"threshold"`
	RawValue   int64  `json:"rawValue"`
	Flags      string `json:"flags"`
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	Device      string           `json:"device"`
	Model       string           `json:"model"`
	Serial      string           `json:"serial"`
	Type        DiskType         `json:"type"`
	Capacity    int64            `json:"capacity"`    // 字节
	Firmware    string           `json:"firmware"`
	Temperature int              `json:"temperature"` // 摄氏度
	PowerOnHours int64           `json:"powerOnHours"`
	StartStopCount int64         `json:"startStopCount"`
	SMART       []SMARTAttribute `json:"smart"`
}

// HealthScore 健康评分
type HealthScore struct {
	Overall     float64      `json:"overall"`     // 0-100
	Reliability float64      `json:"reliability"` // 可靠性评分
	Performance float64      `json:"performance"` // 性能评分
	Temperature float64      `json:"temperature"` // 温度评分
	Status      HealthStatus `json:"status"`
	Prediction  *Prediction  `json:"prediction,omitempty"`
}

// Prediction 故障预测
type Prediction struct {
	FailureProbability float64       `json:"failureProbability"` // 0-1
	EstimatedDays      int           `json:"estimatedDays"`      // 预计剩余天数
	Confidence         float64       `json:"confidence"`         // 置信度
	RiskFactors        []RiskFactor  `json:"riskFactors"`
	Recommendations    []string      `json:"recommendations"`
}

// RiskFactor 风险因素
type RiskFactor struct {
	Name        string  `json:"name"`
	Weight      float64 `json:"weight"`
	Value       float64 `json:"value"`
	Description string  `json:"description"`
}

// HealthReport 健康报告
type HealthReport struct {
	DiskID      string       `json:"diskId"`
	Timestamp   time.Time    `json:"timestamp"`
	Disk        DiskInfo     `json:"disk"`
	Score       HealthScore  `json:"score"`
	Trend       []TrendPoint `json:"trend"`
	Alerts      []Alert      `json:"alerts,omitempty"`
}

// TrendPoint 趋势点
type TrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Score     float64   `json:"score"`
	Temp      int       `json:"temp"`
}

// Alert 告警
type Alert struct {
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// AIHealthPredictor AI 健康预测器
type AIHealthPredictor struct {
	mu       sync.RWMutex
	disks    map[string]*DiskInfo
	history  map[string][]HealthScore
	model    *PredictionModel
}

// PredictionModel 预测模型
type PredictionModel struct {
	weights map[string]float64
	bias    float64
}

// NewAIHealthPredictor 创建预测器
func NewAIHealthPredictor() *AIHealthPredictor {
	return &AIHealthPredictor{
		disks:   make(map[string]*DiskInfo),
		history: make(map[string][]HealthScore),
		model: &PredictionModel{
			weights: map[string]float64{
				"reallocated_sectors": 0.25,
				"pending_sectors":     0.20,
				"uncorrectable":       0.20,
				"temperature":         0.15,
				"power_on_hours":      0.10,
				"start_stop_count":    0.05,
				"wear_leveling":       0.05,
			},
			bias: -0.5,
		},
	}
}

// RegisterDisk 注册磁盘
func (p *AIHealthPredictor) RegisterDisk(disk *DiskInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.disks[disk.Device] = disk
}

// GetHealthScore 获取健康评分
func (p *AIHealthPredictor) GetHealthScore(device string) (*HealthScore, error) {
	p.mu.RLock()
	disk, exists := p.disks[device]
	p.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("磁盘 %s 未注册", device)
	}

	score := p.calculateScore(disk)

	// 保存历史
	p.mu.Lock()
	p.history[device] = append(p.history[device], *score)
	// 保留最近 100 条记录
	if len(p.history[device]) > 100 {
		p.history[device] = p.history[device][len(p.history[device])-100:]
	}
	p.mu.Unlock()

	return score, nil
}

// calculateScore 计算健康评分
func (p *AIHealthPredictor) calculateScore(disk *DiskInfo) *HealthScore {
	score := &HealthScore{
		Overall:     100.0,
		Reliability: 100.0,
		Performance: 100.0,
		Temperature: 100.0,
		Status:      StatusHealthy,
	}

	// 温度评分
	if disk.Temperature > 60 {
		score.Temperature = math.Max(0, 100-float64(disk.Temperature-60)*5)
		score.Overall -= (100 - score.Temperature) * 0.3
	}

	// SMART 属性分析
	for _, attr := range disk.SMART {
		switch attr.ID {
		case 5: // Reallocated Sectors Count
			if attr.RawValue > 0 {
				penalty := float64(attr.RawValue) * 2.0
				score.Reliability -= penalty
				score.Overall -= penalty * 0.3
			}
		case 187: // Reported Uncorrectable Errors
			if attr.RawValue > 0 {
				penalty := float64(attr.RawValue) * 3.0
				score.Reliability -= penalty
				score.Overall -= penalty * 0.25
			}
		case 197: // Current Pending Sector Count
			if attr.RawValue > 0 {
				penalty := float64(attr.RawValue) * 2.5
				score.Reliability -= penalty
				score.Overall -= penalty * 0.25
			}
		case 198: // Offline Uncorrectable
			if attr.RawValue > 0 {
				penalty := float64(attr.RawValue) * 2.5
				score.Reliability -= penalty
				score.Overall -= penalty * 0.2
			}
		}
	}

	// 确保分数在 0-100 范围内
	score.Overall = math.Max(0, math.Min(100, score.Overall))
	score.Reliability = math.Max(0, math.Min(100, score.Reliability))
	score.Performance = math.Max(0, math.Min(100, score.Performance))
	score.Temperature = math.Max(0, math.Min(100, score.Temperature))

	// 确定状态
	if score.Overall >= 80 {
		score.Status = StatusHealthy
	} else if score.Overall >= 60 {
		score.Status = StatusWarning
	} else if score.Overall >= 40 {
		score.Status = StatusCritical
	} else {
		score.Status = StatusFailed
	}

	// 生成预测
	score.Prediction = p.generatePrediction(disk, score)

	return score
}

// generatePrediction 生成故障预测
func (p *AIHealthPredictor) generatePrediction(disk *DiskInfo, score *HealthScore) *Prediction {
	pred := &Prediction{
		FailureProbability: 0.0,
		EstimatedDays:      365 * 5, // 默认 5 年
		Confidence:         0.85,
		RiskFactors:        make([]RiskFactor, 0),
		Recommendations:    make([]string, 0),
	}

	// 计算故障概率
	healthPenalty := (100 - score.Overall) / 100
	pred.FailureProbability = healthPenalty * 0.8

	// 估算剩余天数
	if pred.FailureProbability > 0.1 {
		pred.EstimatedDays = int(float64(365*5) * (1 - pred.FailureProbability))
	}

	// 识别风险因素
	if disk.Temperature > 55 {
		pred.RiskFactors = append(pred.RiskFactors, RiskFactor{
			Name:        "high_temperature",
			Weight:      0.3,
			Value:       float64(disk.Temperature),
			Description: fmt.Sprintf("磁盘温度过高: %d°C", disk.Temperature),
		})
		pred.Recommendations = append(pred.Recommendations, "改善散热条件，降低磁盘温度")
	}

	if disk.PowerOnHours > 30000 {
		pred.RiskFactors = append(pred.RiskFactors, RiskFactor{
			Name:        "high_usage",
			Weight:      0.2,
			Value:       float64(disk.PowerOnHours),
			Description: fmt.Sprintf("使用时间较长: %d 小时", disk.PowerOnHours),
		})
		pred.Recommendations = append(pred.Recommendations, "考虑更换磁盘，做好数据备份")
	}

	// 根据 SMART 属性添加风险因素
	for _, attr := range disk.SMART {
		if attr.ID == 5 && attr.RawValue > 0 {
			pred.RiskFactors = append(pred.RiskFactors, RiskFactor{
				Name:        "reallocated_sectors",
				Weight:      0.4,
				Value:       float64(attr.RawValue),
				Description: fmt.Sprintf("重分配扇区数: %d", attr.RawValue),
			})
			pred.Recommendations = append(pred.Recommendations, "立即备份重要数据")
		}
	}

	// 设置故障预测状态
	if pred.FailureProbability > 0.5 {
		score.Status = StatusPredicted
	}

	return pred
}

// GetHealthReport 获取健康报告
func (p *AIHealthPredictor) GetHealthReport(device string) (*HealthReport, error) {
	disk, exists := p.disks[device]
	if !exists {
		return nil, fmt.Errorf("磁盘 %s 未注册", device)
	}

	score, err := p.GetHealthScore(device)
	if err != nil {
		return nil, err
	}

	// 获取历史趋势
	p.mu.RLock()
	history := p.history[device]
	p.mu.RUnlock()

	trend := make([]TrendPoint, 0, len(history))
	for _, h := range history {
		trend = append(trend, TrendPoint{
			Timestamp: time.Now(),
			Score:     h.Overall,
			Temp:      disk.Temperature,
		})
	}

	report := &HealthReport{
		DiskID:    device,
		Timestamp: time.Now(),
		Disk:      *disk,
		Score:     *score,
		Trend:     trend,
		Alerts:    make([]Alert, 0),
	}

	// 生成告警
	if score.Overall < 60 {
		report.Alerts = append(report.Alerts, Alert{
			Level:     "warning",
			Message:   fmt.Sprintf("磁盘 %s 健康评分较低: %.1f", device, score.Overall),
			Timestamp: time.Now(),
		})
	}

	return report, nil
}

// GetAllDisksHealth 获取所有磁盘健康状态
func (p *AIHealthPredictor) GetAllDisksHealth() map[string]*HealthScore {
	p.mu.RLock()
	devices := make([]string, 0, len(p.disks))
	for device := range p.disks {
		devices = append(devices, device)
	}
	p.mu.RUnlock()

	result := make(map[string]*HealthScore)
	for _, device := range devices {
		score, err := p.GetHealthScore(device)
		if err == nil {
			result[device] = score
		}
	}
	return result
}

// UpdateSMARTData 更新 SMART 数据
func (p *AIHealthPredictor) UpdateSMARTData(device string, attrs []SMARTAttribute) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	disk, exists := p.disks[device]
	if !exists {
		return fmt.Errorf("磁盘 %s 未注册", device)
	}

	disk.SMART = attrs
	return nil
}
