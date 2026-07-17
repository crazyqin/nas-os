// Package storagehealth 提供存储健康预测系统。
// 基于 SMART 数据、温度趋势、错误率分析进行故障预测。
// 参考：群晖 Active Insight、TrueNAS SMART 监控
package storagehealth

import (
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
)

// PredictorHealthLevel 预测器健康等级.
type PredictorHealthLevel string

// PredictorHealthLevel 预测器健康等级
// 使用 Predictor 前缀避免与 HealthStatus 常量冲突.
const (
	PredictorExcellent PredictorHealthLevel = "excellent" // 健康状态优秀
	PredictorGood      PredictorHealthLevel = "good"      // 健康状态良好
	PredictorWarning   PredictorHealthLevel = "warning"   // 需要关注
	PredictorCritical  PredictorHealthLevel = "critical"  // 严重警告
	PredictorFailure   PredictorHealthLevel = "failure"   // 预测故障
)

// DiskSMARTData 磁盘 SMART 数据.
type DiskSMARTData struct {
	DeviceID           string    `json:"device_id"`
	Model              string    `json:"model"`
	Serial             string    `json:"serial"`
	Capacity           int64     `json:"capacity"`
	Temperature        int       `json:"temperature"`           // 当前温度（℃）
	TemperatureMax     int       `json:"temperature_max"`       // 历史最高温度
	PowerOnHours       int64     `json:"power_on_hours"`        // 通电时间
	PowerCycleCount    int64     `json:"power_cycle_count"`     // 通电次数
	ReallocatedSects   int64     `json:"reallocated_sectors"`   // 重映射扇区
	PendingSects       int64     `json:"pending_sectors"`       // 待映射扇区
	UncorrectableSects int64     `json:"uncorrectable_sectors"` // 不可修正扇区
	SeekErrorRate      float64   `json:"seek_error_rate"`       // 寻道错误率
	ReadErrorRate      float64   `json:"read_error_rate"`       // 读取错误率
	SpinRetryCount     int64     `json:"spin_retry_count"`      // 主轴重试次数
	CRCTErrorCount     int64     `json:"crc_error_count"`       // CRC 错误计数
	TotalWritten       int64     `json:"total_written"`         // 总写入量（TB）
	TotalRead          int64     `json:"total_read"`            // 总读取量（TB）
	WearLeveling       int       `json:"wear_leveling"`         // 磨损均衡（SSD，0-100%）
	MediaErrors        int64     `json:"media_errors"`          // 介质错误
	LastChecked        time.Time `json:"last_checked"`
}

// HealthReport 健康报告.
type HealthReport struct {
	DeviceID        string               `json:"device_id"`
	Level           PredictorHealthLevel `json:"level"`
	Score           int                  `json:"score"`          // 0-100 健康评分
	PredictedLife   int                  `json:"predicted_life"` // 预测剩余寿命（天）
	FailureProb     float64              `json:"failure_prob"`   // 90天内故障概率（0-1）
	Warnings        []Warning            `json:"warnings"`
	Recommendations []string             `json:"recommendations"`
	GeneratedAt     time.Time            `json:"generated_at"`
}

// Warning 告警信息.
type Warning struct {
	Code      string `json:"code"`
	Severity  string `json:"severity"` // info, warning, critical
	Message   string `json:"message"`
	Value     string `json:"value"`
	Threshold string `json:"threshold"`
}

// PredictiveMetrics 预测指标.
type PredictiveMetrics struct {
	TemperatureTrend  float64 `json:"temperature_trend"`  // 温度趋势（℃/天）
	ErrorRateTrend    float64 `json:"error_rate_trend"`   // 错误率趋势
	PerformanceDegrad float64 `json:"performance_degrad"` // 性能退化百分比
	RemainingLifePct  float64 `json:"remaining_life_pct"` // 剩余寿命百分比
	MTBFHours         int64   `json:"mtbf_hours"`         // 平均故障间隔时间
}

// HealthPredictor 健康预测器.
type HealthPredictor struct {
	mu      sync.RWMutex
	logger  *slog.Logger
	history map[string][]DiskSMARTData // 设备ID -> 历史数据
	reports map[string]*HealthReport   // 设备ID -> 最新报告
	alerts  []Alert
}

// Alert 告警记录.
type Alert struct {
	DeviceID  string               `json:"device_id"`
	Level     PredictorHealthLevel `json:"level"`
	Message   string               `json:"message"`
	CreatedAt time.Time            `json:"created_at"`
	Acked     bool                 `json:"acked"`
}

// NewPredictor 创建健康预测器.
func NewPredictor(logger *slog.Logger) *HealthPredictor {
	if logger == nil {
		logger = slog.Default()
	}
	return &HealthPredictor{
		logger:  logger,
		history: make(map[string][]DiskSMARTData),
		reports: make(map[string]*HealthReport),
	}
}

// IngestSMARTData 输入 SMART 数据并更新预测.
func (p *HealthPredictor) IngestSMARTData(data DiskSMARTData) *HealthReport {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 保存历史数据
	p.history[data.DeviceID] = append(p.history[data.DeviceID], data)
	// 保留最近 1000 条记录
	if len(p.history[data.DeviceID]) > 1000 {
		p.history[data.DeviceID] = p.history[data.DeviceID][len(p.history[data.DeviceID])-1000:]
	}

	// 生成健康报告
	report := p.evaluate(data)
	p.reports[data.DeviceID] = report

	// 检查是否需要告警
	if report.Level == PredictorCritical || report.Level == PredictorFailure {
		p.alerts = append(p.alerts, Alert{
			DeviceID:  data.DeviceID,
			Level:     report.Level,
			Message:   fmt.Sprintf("磁盘 %s 健康状态: %s (评分: %d)", data.DeviceID, report.Level, report.Score),
			CreatedAt: time.Now(),
		})
		p.logger.Warn("存储健康告警", "device", data.DeviceID, "level", report.Level, "score", report.Score)
	}

	return report
}

// evaluate 评估磁盘健康状态.
func (p *HealthPredictor) evaluate(data DiskSMARTData) *HealthReport {
	report := &HealthReport{
		DeviceID:    data.DeviceID,
		Score:       100,
		GeneratedAt: time.Now(),
	}

	var warnings []Warning

	// 1. 温度评估
	if data.Temperature > 60 {
		report.Score -= 30
		warnings = append(warnings, Warning{
			Code: "TEMP_HIGH", Severity: "critical",
			Message: "磁盘温度过高", Value: fmt.Sprintf("%d℃", data.Temperature), Threshold: "60℃",
		})
	} else if data.Temperature > 50 {
		report.Score -= 15
		warnings = append(warnings, Warning{
			Code: "TEMP_WARN", Severity: "warning",
			Message: "磁盘温度偏高", Value: fmt.Sprintf("%d℃", data.Temperature), Threshold: "50℃",
		})
	}

	// 2. 重映射扇区评估
	if data.ReallocatedSects > 100 {
		report.Score -= 40
		warnings = append(warnings, Warning{
			Code: "REALLOC_HIGH", Severity: "critical",
			Message: "重映射扇区过多", Value: fmt.Sprintf("%d", data.ReallocatedSects), Threshold: "100",
		})
	} else if data.ReallocatedSects > 10 {
		report.Score -= 20
		warnings = append(warnings, Warning{
			Code: "REALLOC_WARN", Severity: "warning",
			Message: "存在重映射扇区", Value: fmt.Sprintf("%d", data.ReallocatedSects), Threshold: "10",
		})
	}

	// 3. 待映射扇区评估
	if data.PendingSects > 0 {
		report.Score -= 25
		warnings = append(warnings, Warning{
			Code: "PENDING_SECTS", Severity: "warning",
			Message: "存在待映射扇区", Value: fmt.Sprintf("%d", data.PendingSects), Threshold: "0",
		})
	}

	// 4. 通电时间评估
	if data.PowerOnHours > 50000 {
		report.Score -= 15
		warnings = append(warnings, Warning{
			Code: "POWER_HOURS_HIGH", Severity: "info",
			Message: "磁盘通电时间较长", Value: fmt.Sprintf("%d小时", data.PowerOnHours), Threshold: "50000小时",
		})
	}

	// 5. SSD 磨损均衡评估
	if data.WearLeveling > 0 && data.WearLeveling < 10 {
		report.Score -= 35
		warnings = append(warnings, Warning{
			Code: "WEAR_LOW", Severity: "critical",
			Message: "SSD 磨损严重", Value: fmt.Sprintf("%d%%", data.WearLeveling), Threshold: "10%",
		})
	}

	// 6. CRC 错误评估
	if data.CRCTErrorCount > 0 {
		report.Score -= 10
		warnings = append(warnings, Warning{
			Code: "CRC_ERRORS", Severity: "warning",
			Message: "存在 CRC 错误", Value: fmt.Sprintf("%d", data.CRCTErrorCount), Threshold: "0",
		})
	}

	// 确保评分不低于 0
	if report.Score < 0 {
		report.Score = 0
	}

	// 确定健康等级
	report.Level = p.scoreToLevel(report.Score)
	report.Warnings = warnings

	// 计算预测寿命
	history := p.history[data.DeviceID]
	report.PredictedLife = p.predictRemainingLife(data, history)
	report.FailureProb = p.calculateFailureProb(data, history, report.Score)

	// 生成建议
	report.Recommendations = p.generateRecommendations(report, data)

	return report
}

// scoreToLevel 根据评分确定健康等级.
func (p *HealthPredictor) scoreToLevel(score int) PredictorHealthLevel {
	switch {
	case score >= 90:
		return PredictorExcellent
	case score >= 70:
		return PredictorGood
	case score >= 50:
		return PredictorWarning
	case score >= 30:
		return PredictorCritical
	default:
		return PredictorFailure
	}
}

// predictRemainingLife 预测剩余寿命（天）.
func (p *HealthPredictor) predictRemainingLife(data DiskSMARTData, history []DiskSMARTData) int {
	if data.PowerOnHours == 0 {
		return 3650 // 默认 10 年
	}

	// 基于 SMART 数据的简化预测模型
	baseLifeHours := int64(43800) // 5 年

	// 温度因子
	tempFactor := 1.0
	if data.Temperature > 45 {
		tempFactor = 0.8
	}
	if data.Temperature > 55 {
		tempFactor = 0.5
	}

	// 重映射扇区因子
	reallocFactor := 1.0
	if data.ReallocatedSects > 0 {
		reallocFactor = math.Max(0.3, 1.0-float64(data.ReallocatedSects)/1000.0)
	}

	adjustedLife := float64(baseLifeHours) * tempFactor * reallocFactor
	remaining := adjustedLife - float64(data.PowerOnHours)
	if remaining < 0 {
		return 0
	}
	return int(remaining / 24) // 转换为天数
}

// calculateFailureProb 计算 90 天内故障概率.
func (p *HealthPredictor) calculateFailureProb(data DiskSMARTData, history []DiskSMARTData, score int) float64 {
	prob := 0.01 // 基础概率 1%

	// 根据评分调整
	prob += float64(100-score) * 0.005

	// 重映射扇区大幅增加故障概率
	if data.ReallocatedSects > 100 {
		prob += 0.3
	} else if data.ReallocatedSects > 10 {
		prob += 0.1
	}

	// 待映射扇区增加故障概率
	if data.PendingSects > 0 {
		prob += 0.15
	}

	// 温度过高增加故障概率
	if data.Temperature > 60 {
		prob += 0.2
	}

	// 限制在 0-1 范围
	if prob > 1.0 {
		prob = 1.0
	}
	return prob
}

// generateRecommendations 生成建议.
func (p *HealthPredictor) generateRecommendations(report *HealthReport, data DiskSMARTData) []string {
	var recs []string

	if report.Level == PredictorCritical || report.Level == PredictorFailure {
		recs = append(recs, "⚠️ 建议立即备份重要数据")
		recs = append(recs, "🔄 考虑更换磁盘")
	}

	if data.Temperature > 50 {
		recs = append(recs, "🌡️ 检查散热系统，确保通风良好")
	}

	if data.ReallocatedSects > 0 {
		recs = append(recs, "📊 密切监控重映射扇区增长趋势")
	}

	if data.PowerOnHours > 40000 {
		recs = append(recs, "⏰ 磁盘服役时间较长，建议制定更换计划")
	}

	if len(recs) == 0 {
		recs = append(recs, "✅ 磁盘状态良好，继续正常监控")
	}

	return recs
}

// GetReport 获取设备健康报告.
func (p *HealthPredictor) GetReport(deviceID string) (*HealthReport, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	report, found := p.reports[deviceID]
	return report, found
}

// GetAllReports 获取所有设备健康报告.
func (p *HealthPredictor) GetAllReports() map[string]*HealthReport {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*HealthReport, len(p.reports))
	for k, v := range p.reports {
		result[k] = v
	}
	return result
}

// GetAlerts 获取未确认的告警.
func (p *HealthPredictor) GetAlerts() []Alert {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var unacked []Alert
	for _, a := range p.alerts {
		if !a.Acked {
			unacked = append(unacked, a)
		}
	}
	return unacked
}

// AckAlert 确认告警.
func (p *HealthPredictor) AckAlert(deviceID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range p.alerts {
		if p.alerts[i].DeviceID == deviceID {
			p.alerts[i].Acked = true
		}
	}
}
