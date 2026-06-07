// Package storage 提供存储管理功能
// health_score.go - 存储设备健康评分系统
package storage

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// 各评分维度权重常量
const (
	// WeightSMART SMART属性评分权重
	WeightSMART float64 = 0.40
	// WeightUsage 磁盘使用率评分权重
	WeightUsage float64 = 0.15
	// WeightAge 磁盘年龄评分权重
	WeightAge float64 = 0.15
	// WeightIOError I/O错误率评分权重
	WeightIOError float64 = 0.15
	// WeightTemperature 温度评分权重
	WeightTemperature float64 = 0.15

	// 健康等级阈值
	// ThresholdExcellent 优秀等级最低分
	ThresholdExcellent float64 = 90.0
	// ThresholdGood 良好等级最低分
	ThresholdGood float64 = 70.0
	// ThresholdFair 一般等级最低分
	ThresholdFair float64 = 50.0
	// ThresholdPoor 较差等级最低分
	ThresholdPoor float64 = 30.0

	// 温度阈值
	// TempOptimal 最佳运行温度上限
	TempOptimal float64 = 45.0
	// TempWarning 温度警告阈值
	TempWarning float64 = 55.0
	// TempCritical 温度严重阈值
	TempCritical float64 = 65.0

	// 使用率阈值
	// UsageWarning 使用率警告阈值（百分比）
	UsageWarning float64 = 90.0
	// UsageCritical 使用率严重阈值（百分比）
	UsageCritical float64 = 95.0

	// 年龄阈值（年）
	// AgeOptimal 最佳使用年限
	AgeOptimal float64 = 3.0
	// AgeWarning 使用年限警告
	AgeWarning float64 = 5.0
	// AgeCritical 使用年限严重
	AgeCritical float64 = 7.0
)

// HealthLevel 健康等级类型
type HealthLevel string

const (
	// HealthLevelExcellent 优秀（90-100分）
	HealthLevelExcellent HealthLevel = "Excellent"
	// HealthLevelGood 良好（70-89分）
	HealthLevelGood HealthLevel = "Good"
	// HealthLevelFair 一般（50-69分）
	HealthLevelFair HealthLevel = "Fair"
	// HealthLevelPoor 较差（30-49分）
	HealthLevelPoor HealthLevel = "Poor"
	// HealthLevelCritical 严重（<30分）
	HealthLevelCritical HealthLevel = "Critical"
)

// SMARTMetrics SMART属性指标
// 用于健康评分的SMART数据提取
type SMARTMetrics struct {
	// 重分配扇区数
	ReallocatedSectors uint64 `json:"reallocatedSectors"`
	// 待映射扇区数
	PendingSectors uint64 `json:"pendingSectors"`
	// 离线不可修正扇区数
	OfflineUncorrectable uint64 `json:"offlineUncorrectable"`
	// UDMA CRC错误计数
	UDMACRCErrorCount uint64 `json:"udmaCrcErrorCount"`
	// 寻道错误率
	SeekErrorRate float64 `json:"seekErrorRate"`
	// 读取错误率
	ReadErrorRate float64 `json:"readErrorRate"`
	// 写入错误率
	WriteErrorRate float64 `json:"writeErrorRate"`
	// 通电时间（小时）
	PowerOnHours uint64 `json:"powerOnHours"`
	// 通电周期数
	PowerCycleCount uint64 `json:"powerCycleCount"`
	// 当前温度（摄氏度）
	Temperature int `json:"temperature"`
	// SMART整体状态
	SMARTStatus SMARTStatus `json:"smartStatus"`
	// NVMe可用备用空间百分比
	NVMeAvailableSpare int `json:"nvmeAvailableSpare"`
	// NVMe使用百分比
	NVMePercentageUsed int `json:"nvmePercentageUsed"`
}

// HealthScoreResult 健康评分结果
type HealthScoreResult struct {
	// 总分（0-100）
	Total float64 `json:"total"`
	// SMART属性评分
	SMARTScore float64 `json:"smartScore"`
	// 磁盘使用率评分
	UsageScore float64 `json:"usageScore"`
	// 磁盘年龄评分
	AgeScore float64 `json:"ageScore"`
	// I/O错误率评分
	IOErrorScore float64 `json:"ioErrorScore"`
	// 温度评分
	TemperatureScore float64 `json:"temperatureScore"`
	// 健康等级
	Level HealthLevel `json:"level"`
	// 评分时间
	Timestamp time.Time `json:"timestamp"`
	// 扣分原因列表
	Deductions []string `json:"deductions,omitempty"`
	// 告警建议
	Alerts []string `json:"alerts,omitempty"`
}

// HealthTrend 健康趋势数据
// 用于预测性故障分析
type HealthTrend struct {
	// 设备标识
	Device string `json:"device"`
	// 历史评分序列（按时间排序）
	Scores []TrendPoint `json:"scores"`
	// 评分变化速率（负值表示下降趋势）
	TrendRate float64 `json:"trendRate"`
	// 评分标准差（波动性）
	StdDeviation float64 `json:"stdDeviation"`
	// 预测故障概率（0.0-1.0）
	FailureProbability float64 `json:"failureProbability"`
	// 预计到达故障阈值的时间
	EstimatedFailureTime *time.Time `json:"estimatedFailureTime,omitempty"`
	// 预测置信度
	Confidence float64 `json:"confidence"`
}

// TrendPoint 趋势数据点
type TrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Score     float64   `json:"score"`
}

// DiskHealthReport 磁盘健康报告
type DiskHealthReport struct {
	// 设备标识
	Device string `json:"device"`
	// 磁盘型号
	Model string `json:"model"`
	// 序列号
	Serial string `json:"serial"`
	// 磁盘大小（字节）
	Size uint64 `json:"size"`
	// 健康评分
	Score HealthScoreResult `json:"score"`
	// SMART指标
	SMARTMetrics SMARTMetrics `json:"smartMetrics"`
	// 磁盘使用率（百分比）
	DiskUsagePercent float64 `json:"diskUsagePercent"`
	// 磁盘使用年限
	DiskAgeYears float64 `json:"diskAgeYears"`
	// I/O错误率
	IOErrorRate float64 `json:"ioErrorRate"`
	// 健康趋势
	Trend *HealthTrend `json:"trend,omitempty"`
	// 报告生成时间
	GeneratedAt time.Time `json:"generatedAt"`
	// 维护建议
	Recommendations []string `json:"recommendations,omitempty"`
}

// HealthScoreEngine 健康评分引擎
type HealthScoreEngine struct {
	mu sync.RWMutex

	// 历史评分数据，key为设备名
	history map[string][]TrendPoint

	// 最大历史记录数
	maxHistorySize int

	// 故障预测阈值分数（低于此分触发预测告警）
	failureThreshold float64
}

// NewHealthScoreEngine 创建健康评分引擎
func NewHealthScoreEngine() *HealthScoreEngine {
	return &HealthScoreEngine{
		history:          make(map[string][]TrendPoint),
		maxHistorySize:   288, // 每30分钟一次，保留6天
		failureThreshold: 30.0,
	}
}

// CalculateScore 计算磁盘健康评分
// disk: 磁盘健康信息
// usagePercent: 磁盘使用率百分比
// manufactureDate: 磁盘出厂日期（用于计算年龄，可选）
func (e *HealthScoreEngine) CalculateScore(disk *DiskHealth, usagePercent float64, manufactureDate *time.Time) *HealthScoreResult {
	if disk == nil {
		return &HealthScoreResult{
			Total:     0,
			Level:     HealthLevelCritical,
			Timestamp: time.Now(),
			Alerts:    []string{"磁盘信息为空"},
		}
	}

	// 提取SMART指标
	metrics := extractSMARTMetrics(disk)

	// 计算各维度评分
	smartScore := e.calculateSMARTScore(metrics)
	usageScore := e.calculateUsageScore(usagePercent)
	ageScore := e.calculateAgeScore(disk.PowerOnHours, manufactureDate)
	ioErrorScore := e.calculateIOErrorScore(metrics)
	tempScore := e.calculateTemperatureScore(float64(metrics.Temperature))

	// 加权计算总分
	total := smartScore*WeightSMART +
		usageScore*WeightUsage +
		ageScore*WeightAge +
		ioErrorScore*WeightIOError +
		tempScore*WeightTemperature

	// 确保总分在0-100范围内
	total = clampScore(total)

	// 收集扣分原因和告警
	deductions := e.collectDeductions(metrics, usagePercent, disk.PowerOnHours, manufactureDate)
	alerts := e.collectAlerts(metrics, usagePercent, disk.PowerOnHours, manufactureDate, total)

	score := &HealthScoreResult{
		Total:            math.Round(total*10) / 10, // 保留一位小数
		SMARTScore:       math.Round(smartScore*10) / 10,
		UsageScore:       math.Round(usageScore*10) / 10,
		AgeScore:         math.Round(ageScore*10) / 10,
		IOErrorScore:     math.Round(ioErrorScore*10) / 10,
		TemperatureScore: math.Round(tempScore*10) / 10,
		Level:            e.GetHealthLevel(total),
		Timestamp:        time.Now(),
		Deductions:       deductions,
		Alerts:           alerts,
	}

	// 记录历史数据
	e.recordHistory(disk.Device, score.Total)

	return score
}

// GetHealthLevel 根据分数获取健康等级
func (e *HealthScoreEngine) GetHealthLevel(score float64) HealthLevel {
	switch {
	case score >= ThresholdExcellent:
		return HealthLevelExcellent
	case score >= ThresholdGood:
		return HealthLevelGood
	case score >= ThresholdFair:
		return HealthLevelFair
	case score >= ThresholdPoor:
		return HealthLevelPoor
	default:
		return HealthLevelCritical
	}
}

// PredictFailure 预测磁盘故障概率
// 基于历史评分趋势进行线性回归预测
func (e *HealthScoreEngine) PredictFailure(device string) *HealthTrend {
	e.mu.RLock()
	defer e.mu.RUnlock()

	points, ok := e.history[device]
	if !ok || len(points) < 3 {
		// 数据不足，无法预测
		return &HealthTrend{
			Device:             device,
			Scores:             points,
			FailureProbability: 0,
			Confidence:         0,
		}
	}

	trend := &HealthTrend{
		Device: device,
		Scores: points,
	}

	// 计算线性回归参数
	n := float64(len(points))
	var sumX, sumY, sumXY, sumX2 float64
	baseTime := points[0].Timestamp.Unix()

	for i, p := range points {
		x := float64(p.Timestamp.Unix()-baseTime) / 3600.0 // 转换为小时
		y := p.Score
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
		_ = i
	}

	// 线性回归: y = slope*x + intercept
	denominator := n*sumX2 - sumX*sumX
	if math.Abs(denominator) < 1e-10 {
		// 无法计算斜率
		return trend
	}

	slope := (n*sumXY - sumX*sumY) / denominator
	intercept := (sumY - slope*sumX) / n

	trend.TrendRate = slope

	// 计算标准差（波动性）
	var sumSquaredResiduals float64
	for _, p := range points {
		x := float64(p.Timestamp.Unix()-baseTime) / 3600.0
		predicted := slope*x + intercept
		residual := p.Score - predicted
		sumSquaredResiduals += residual * residual
	}
	trend.StdDeviation = math.Sqrt(sumSquaredResiduals / n)

	// 计算故障概率
	// 基于当前趋势分数和下降速率
	currentScore := points[len(points)-1].Score
	trend.FailureProbability = e.calculateFailureProbability(currentScore, slope, trend.StdDeviation)

	// 预测到达故障阈值的时间
	if slope < 0 && currentScore > e.failureThreshold {
		// 计算到达阈值的时间
		hoursToFailure := (e.failureThreshold - currentScore) / slope
		if hoursToFailure > 0 {
			failureTime := time.Now().Add(time.Duration(hoursToFailure) * time.Hour)
			trend.EstimatedFailureTime = &failureTime
		}
	}

	// 计算置信度（基于数据点数量和一致性）
	trend.Confidence = e.calculateConfidence(n, trend.StdDeviation)

	return trend
}

// GenerateReport 生成磁盘健康报告
func (e *HealthScoreEngine) GenerateReport(disk *DiskHealth, usagePercent float64, manufactureDate *time.Time, ioErrorRate float64) *DiskHealthReport {
	if disk == nil {
		return &DiskHealthReport{
			GeneratedAt: time.Now(),
		}
	}

	metrics := extractSMARTMetrics(disk)
	score := e.CalculateScore(disk, usagePercent, manufactureDate)
	trend := e.PredictFailure(disk.Device)

	// 计算使用年限
	var ageYears float64
	if manufactureDate != nil {
		ageYears = time.Since(*manufactureDate).Hours() / (24 * 365.25)
	} else {
		// 通过通电时间估算（假设平均每天通电16小时）
		ageYears = float64(disk.PowerOnHours) / (16 * 365.25)
	}

	report := &DiskHealthReport{
		Device:           disk.Device,
		Model:            disk.Model,
		Serial:           disk.Serial,
		Size:             disk.Size,
		Score:            *score,
		SMARTMetrics:     metrics,
		DiskUsagePercent: usagePercent,
		DiskAgeYears:     math.Round(ageYears*100) / 100,
		IOErrorRate:      ioErrorRate,
		Trend:            trend,
		GeneratedAt:      time.Now(),
		Recommendations:  e.generateRecommendations(score, metrics, usagePercent, ageYears, ioErrorRate, trend),
	}

	return report
}

// ============ 内部评分计算方法 ============

// calculateSMARTScore 计算SMART属性评分（满分100）
func (e *HealthScoreEngine) calculateSMARTScore(m SMARTMetrics) float64 {
	score := 100.0

	// SMART状态检查 - 最关键的指标
	switch m.SMARTStatus {
	case SMARTStatusFAILING:
		return 0 // 直接判0分
	case SMARTStatusWARNING:
		score -= 30
	case SMARTStatusUNKNOWN, SMARTStatusUNSUPPORTED:
		score -= 10
	}

	// 重分配扇区 - 最重要的衰减指标
	switch {
	case m.ReallocatedSectors > 0:
		// 每个重分配扇区扣2分，最多扣50分
		deduction := math.Min(float64(m.ReallocatedSectors)*2, 50)
		score -= deduction
	}

	// 待映射扇区
	if m.PendingSectors > 0 {
		deduction := math.Min(float64(m.PendingSectors)*3, 40)
		score -= deduction
	}

	// 离线不可修正扇区
	if m.OfflineUncorrectable > 0 {
		deduction := math.Min(float64(m.OfflineUncorrectable)*2, 30)
		score -= deduction
	}

	// UDMA CRC错误
	if m.UDMACRCErrorCount > 0 {
		deduction := math.Min(float64(m.UDMACRCErrorCount)*1.5, 20)
		score -= deduction
	}

	// 通电时间影响（超过50000小时开始扣分）
	if m.PowerOnHours > 50000 {
		excessHours := float64(m.PowerOnHours-50000) / 10000
		score -= math.Min(excessHours*3, 15)
	}

	// NVMe特有属性
	if m.NVMeAvailableSpare > 0 {
		// 可用备用空间低于50%开始扣分
		if m.NVMeAvailableSpare < 50 {
			score -= float64(50-m.NVMeAvailableSpare) * 0.5
		}
	}

	return clampScore(score)
}

// calculateUsageScore 计算磁盘使用率评分（满分100）
func (e *HealthScoreEngine) calculateUsageScore(usagePercent float64) float64 {
	switch {
	case usagePercent <= 70:
		// 70%以下满分
		return 100
	case usagePercent <= 80:
		// 70-80%轻微扣分
		return 100 - (usagePercent-70)*2
	case usagePercent <= UsageWarning:
		// 80-90%中度扣分
		return 80 - (usagePercent-80)*3
	case usagePercent <= UsageCritical:
		// 90-95%严重扣分
		return 50 - (usagePercent-UsageWarning)*5
	default:
		// 95%以上极度危险
		return math.Max(0, 25-(usagePercent-UsageCritical)*5)
	}
}

// calculateAgeScore 计算磁盘年龄评分（满分100）
func (e *HealthScoreEngine) calculateAgeScore(powerOnHours uint64, manufactureDate *time.Time) float64 {
	var ageYears float64

	if manufactureDate != nil {
		ageYears = time.Since(*manufactureDate).Hours() / (24 * 365.25)
	} else {
		// 通过通电时间估算
		ageYears = float64(powerOnHours) / (24 * 365.25)
	}

	switch {
	case ageYears <= AgeOptimal:
		return 100
	case ageYears <= AgeWarning:
		// 3-5年缓慢扣分
		return 100 - (ageYears-AgeOptimal)*10
	case ageYears <= AgeCritical:
		// 5-7年中度扣分
		return 80 - (ageYears-AgeWarning)*15
	default:
		// 7年以上严重扣分
		return math.Max(0, 50-(ageYears-AgeCritical)*10)
	}
}

// calculateIOErrorScore 计算I/O错误率评分（满分100）
func (e *HealthScoreEngine) calculateIOErrorScore(m SMARTMetrics) float64 {
	// 综合评估各种错误率
	totalErrorRate := m.SeekErrorRate + m.ReadErrorRate + m.WriteErrorRate

	switch {
	case totalErrorRate == 0:
		return 100
	case totalErrorRate <= 1.0:
		// 轻微错误
		return 100 - totalErrorRate*20
	case totalErrorRate <= 5.0:
		// 中度错误
		return 80 - (totalErrorRate-1.0)*10
	case totalErrorRate <= 10.0:
		// 严重错误
		return 40 - (totalErrorRate-5.0)*6
	default:
		// 极度严重
		return math.Max(0, 10-(totalErrorRate-10.0)*2)
	}
}

// calculateTemperatureScore 计算温度评分（满分100）
func (e *HealthScoreEngine) calculateTemperatureScore(tempCelsius float64) float64 {
	switch {
	case tempCelsius <= TempOptimal:
		return 100
	case tempCelsius <= TempWarning:
		// 45-55°C线性扣分
		return 100 - (tempCelsius-TempOptimal)*5
	case tempCelsius <= TempCritical:
		// 55-65°C快速扣分
		return 50 - (tempCelsius-TempWarning)*4
	default:
		// >65°C极度危险
		return math.Max(0, 10-(tempCelsius-TempCritical)*2)
	}
}

// calculateFailureProbability 计算故障概率
func (e *HealthScoreEngine) calculateFailureProbability(currentScore, slope, stdDev float64) float64 {
	// 基于当前分数的权重
	scoreWeight := 0.0
	switch {
	case currentScore >= ThresholdExcellent:
		scoreWeight = 0.01
	case currentScore >= ThresholdGood:
		scoreWeight = 0.05
	case currentScore >= ThresholdFair:
		scoreWeight = 0.15
	case currentScore >= ThresholdPoor:
		scoreWeight = 0.40
	default:
		scoreWeight = 0.80
	}

	// 基于下降速率的权重
	slopeWeight := 0.0
	if slope < 0 {
		// 斜率越负，下降越快
		slopeWeight = math.Min(-slope*0.1, 0.5)
	}

	// 基于波动性的权重
	volatilityWeight := math.Min(stdDev*0.02, 0.3)

	// 综合概率
	probability := scoreWeight + slopeWeight + volatilityWeight

	return clamp(probability, 0, 1)
}

// calculateConfidence 计算预测置信度
func (e *HealthScoreEngine) calculateConfidence(dataPoints float64, stdDev float64) float64 {
	// 数据点越多，置信度越高
	pointConfidence := math.Min(dataPoints/24.0, 1.0) // 24个数据点达到满置信度

	// 标准差越小，置信度越高
	deviationConfidence := 1.0 / (1.0 + stdDev*0.1)

	return clamp(pointConfidence*0.6+deviationConfidence*0.4, 0, 1)
}

// ============ 辅助方法 ============

// extractSMARTMetrics 从DiskHealth提取SMART指标
func extractSMARTMetrics(disk *DiskHealth) SMARTMetrics {
	return SMARTMetrics{
		ReallocatedSectors:   disk.ReallocatedSectors,
		PendingSectors:       disk.PendingSectors,
		OfflineUncorrectable: disk.OfflineUncorrectable,
		UDMACRCErrorCount:    disk.UDMACRCErrorCount,
		SeekErrorRate:        disk.SeekErrorRate,
		ReadErrorRate:        disk.ReadErrorRate,
		WriteErrorRate:       disk.WriteErrorRate,
		PowerOnHours:         disk.PowerOnHours,
		PowerCycleCount:      disk.PowerCycleCount,
		Temperature:          disk.Temperature,
		SMARTStatus:          disk.SMARTStatus,
		NVMeAvailableSpare:   disk.NVMeAvailableSpare,
		NVMePercentageUsed:   disk.NVMePercentageUsed,
	}
}

// recordHistory 记录评分历史
func (e *HealthScoreEngine) recordHistory(device string, score float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	point := TrendPoint{
		Timestamp: time.Now(),
		Score:     score,
	}

	e.history[device] = append(e.history[device], point)

	// 限制历史记录大小
	if len(e.history[device]) > e.maxHistorySize {
		e.history[device] = e.history[device][len(e.history[device])-e.maxHistorySize:]
	}
}

// collectDeductions 收集扣分原因
func (e *HealthScoreEngine) collectDeductions(m SMARTMetrics, usagePercent float64, powerOnHours uint64, manufactureDate *time.Time) []string {
	var deductions []string

	// SMART扣分
	if m.SMARTStatus == SMARTStatusFAILING {
		deductions = append(deductions, "SMART自检失败")
	}
	if m.SMARTStatus == SMARTStatusWARNING {
		deductions = append(deductions, "SMART存在警告")
	}
	if m.ReallocatedSectors > 0 {
		deductions = append(deductions, fmt.Sprintf("存在%d个重分配扇区", m.ReallocatedSectors))
	}
	if m.PendingSectors > 0 {
		deductions = append(deductions, fmt.Sprintf("存在%d个待映射扇区", m.PendingSectors))
	}
	if m.OfflineUncorrectable > 0 {
		deductions = append(deductions, fmt.Sprintf("存在%d个离线不可修正扇区", m.OfflineUncorrectable))
	}
	if m.UDMACRCErrorCount > 0 {
		deductions = append(deductions, fmt.Sprintf("UDMA CRC错误%d次", m.UDMACRCErrorCount))
	}

	// 使用率扣分
	if usagePercent > UsageWarning {
		deductions = append(deductions, fmt.Sprintf("磁盘使用率%.1f%%超过警告阈值%.0f%%", usagePercent, UsageWarning))
	}

	// 年龄扣分
	ageYears := e.getAgeYears(powerOnHours, manufactureDate)
	if ageYears > AgeWarning {
		deductions = append(deductions, fmt.Sprintf("磁盘使用%.1f年超过警告年限%.0f年", ageYears, AgeWarning))
	}

	// I/O错误扣分
	totalErrors := m.SeekErrorRate + m.ReadErrorRate + m.WriteErrorRate
	if totalErrors > 1.0 {
		deductions = append(deductions, fmt.Sprintf("I/O错误率%.2f%%偏高", totalErrors))
	}

	// 温度扣分
	if float64(m.Temperature) > TempWarning {
		deductions = append(deductions, fmt.Sprintf("温度%d°C超过警告阈值%.0f°C", m.Temperature, TempWarning))
	}

	return deductions
}

// collectAlerts 收集告警建议
func (e *HealthScoreEngine) collectAlerts(m SMARTMetrics, usagePercent float64, powerOnHours uint64, manufactureDate *time.Time, totalScore float64) []string {
	var alerts []string

	if m.SMARTStatus == SMARTStatusFAILING {
		alerts = append(alerts, "⚠️ 紧急：磁盘SMART自检失败，建议立即备份数据并更换磁盘")
	}
	if m.ReallocatedSectors > 100 {
		alerts = append(alerts, "⚠️ 重分配扇区数量严重，磁盘可能即将故障")
	}
	if usagePercent > UsageCritical {
		alerts = append(alerts, fmt.Sprintf("磁盘使用率%.1f%%接近满载，建议清理或扩容", usagePercent))
	}
	if float64(m.Temperature) > TempCritical {
		alerts = append(alerts, fmt.Sprintf("温度%d°C过高，检查散热系统", m.Temperature))
	}
	if totalScore < ThresholdPoor {
		alerts = append(alerts, "健康评分低于30分，建议制定更换计划")
	}

	ageYears := e.getAgeYears(powerOnHours, manufactureDate)
	if ageYears > AgeCritical {
		alerts = append(alerts, fmt.Sprintf("磁盘已使用%.1f年，超过建议更换年限", ageYears))
	}

	return alerts
}

// generateRecommendations 生成维护建议
func (e *HealthScoreEngine) generateRecommendations(score *HealthScoreResult, m SMARTMetrics, usagePercent, ageYears, ioErrorRate float64, trend *HealthTrend) []string {
	var recs []string

	// 根据健康等级给出建议
	switch score.Level {
	case HealthLevelCritical:
		recs = append(recs, "立即备份所有重要数据")
		recs = append(recs, "联系供应商更换磁盘")
	case HealthLevelPoor:
		recs = append(recs, "尽快安排数据备份")
		recs = append(recs, "准备备用磁盘")
	case HealthLevelFair:
		recs = append(recs, "定期检查磁盘健康状态")
		recs = append(recs, "增加备份频率")
	}

	// 使用率建议
	if usagePercent > UsageWarning {
		recs = append(recs, fmt.Sprintf("清理磁盘空间，当前使用率%.1f%%过高", usagePercent))
	}

	// 温度建议
	if float64(m.Temperature) > TempWarning {
		recs = append(recs, "检查服务器散热和通风系统")
	}

	// I/O错误建议
	if ioErrorRate > 5.0 {
		recs = append(recs, "检查磁盘线缆和接口连接")
	}

	// 趋势建议
	if trend != nil && trend.FailureProbability > 0.5 {
		recs = append(recs, fmt.Sprintf("基于趋势分析，故障概率为%.0f%%，建议提前规划更换", trend.FailureProbability*100))
		if trend.EstimatedFailureTime != nil {
			recs = append(recs, fmt.Sprintf("预计故障时间：%s", trend.EstimatedFailureTime.Format("2006-01-02")))
		}
	}

	// SMART特定建议
	if m.ReallocatedSectors > 0 && m.ReallocatedSectors < 10 {
		recs = append(recs, "监测重分配扇区增长趋势")
	}

	if m.PendingSectors > 0 {
		recs = append(recs, "运行磁盘自检修复待映射扇区")
	}

	// 通用建议
	if score.Level == HealthLevelExcellent || score.Level == HealthLevelGood {
		recs = append(recs, "磁盘状态良好，继续保持定期监控")
	}

	return recs
}

// getAgeYears 获取磁盘使用年限
func (e *HealthScoreEngine) getAgeYears(powerOnHours uint64, manufactureDate *time.Time) float64 {
	if manufactureDate != nil {
		return time.Since(*manufactureDate).Hours() / (24 * 365.25)
	}
	return float64(powerOnHours) / (16 * 365.25) // 假设每天通电16小时
}

// clampScore 将分数限制在0-100范围内
func clampScore(score float64) float64 {
	return clamp(score, 0, 100)
}

// clamp 将值限制在指定范围内
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ClearHistory 清除指定设备的历史记录
func (e *HealthScoreEngine) ClearHistory(device string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.history, device)
}

// GetHistory 获取指定设备的历史记录
func (e *HealthScoreEngine) GetHistory(device string) []TrendPoint {
	e.mu.RLock()
	defer e.mu.RUnlock()
	points, ok := e.history[device]
	if !ok {
		return nil
	}
	// 返回副本
	result := make([]TrendPoint, len(points))
	copy(result, points)
	return result
}

// SetFailureThreshold 设置故障预测阈值分数
func (e *HealthScoreEngine) SetFailureThreshold(threshold float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.failureThreshold = clamp(threshold, 0, 100)
}
