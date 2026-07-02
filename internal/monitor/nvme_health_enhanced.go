// Package monitor 提供NVMe增强健康监控功能
// Version: v2.211.0 - NVMe SSD三级健康预警系统增强
// 支持主流NVMe品牌（三星、西部数据、Intel）的TBW寿命预测
package monitor

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"nas-os/internal/disk"
)

// NVMeEnhancedMonitor NVMe增强监控器.
type NVMeEnhancedMonitor struct {
	mu              sync.RWMutex
	nvmeMonitor     *disk.NVMeMonitor
	tbwDatabase     *disk.ManufacturerTBWSpec
	alertManager    *AlertingManager
	healthHistory   map[string][]NVMeHealthPoint
	predictionCache map[string]*NVMeLifePredictionEnhanced
}

// NVMeHealthPoint NVMe健康数据点.
type NVMeHealthPoint struct {
	Timestamp        time.Time `json:"timestamp"`
	HealthPercentage uint8     `json:"healthPercentage"`
	Temperature      uint8     `json:"temperature"`
	TBWUsedPercent   float64   `json:"tbwUsedPercent"`
	WriteRate        float64   `json:"writeRate"` // GB/day
	AlertLevel       string    `json:"alertLevel"`
}

// NVMeLifePredictionEnhanced NVMe寿命预测增强版.
type NVMeLifePredictionEnhanced struct {
	Device       string `json:"device"`
	Model        string `json:"model"`
	Manufacturer string `json:"manufacturer"`

	// 寿命预测
	RemainingTBW      float64   `json:"remainingTBW"`      // 剩余TBW (TB)
	RemainingLifeDays int       `json:"remainingLifeDays"` // 预估剩余天数
	RemainingLifePct  float64   `json:"remainingLifePct"`  // 剩余寿命百分比
	EstimatedEndDate  time.Time `json:"estimatedEndDate"`  // 预估寿命终结日期

	// TBW规格
	TBWTotal       float64 `json:"tbwTotal"`       // 厂商额定TBW (TB)
	TBWUsed        float64 `json:"tbwUsed"`        // 已使用TBW (TB)
	TBWUsedPercent float64 `json:"tbwUsedPercent"` // TBW使用百分比
	DWPD           float64 `json:"dwpd"`           // 每日写入驱动次数

	// 影响因子
	TempImpactFactor float64 `json:"tempImpactFactor"` // 温度影响因子 (0-2)
	WriteAmpFactor   float64 `json:"writeAmpFactor"`   // 写放大因子 (1-3)
	WearImpactFactor float64 `json:"wearImpactFactor"` // 磨损影响因子

	// 预警级别
	AlertLevel   string   `json:"alertLevel"` // normal/warning/critical/emergency
	AlertReasons []string `json:"alertReasons"`

	// 置信度
	ConfidenceLevel string `json:"confidenceLevel"` // high/medium/low
	DataPoints      int    `json:"dataPoints"`      // 用于预测的数据点数

	// 时间戳
	LastUpdated time.Time `json:"lastUpdated"`
}

// NVMeHealthScoreEnhanced NVMe健康评分增强版.
type NVMeHealthScoreEnhanced struct {
	Device string `json:"device"`
	Model  string `json:"model"`

	// 综合评分 (0-100)
	TotalScore float64 `json:"totalScore"`
	Grade      string  `json:"grade"` // A/B/C/D/F

	// 分项评分 (基于剩余寿命、温度、错误计数)
	LifeScore        disk.ComponentScore `json:"lifeScore"`        // 寿命评分 (权重40%)
	TemperatureScore disk.ComponentScore `json:"temperatureScore"` // 温度评分 (权重25%)
	ErrorScore       disk.ComponentScore `json:"errorScore"`       // 错误评分 (权重20%)
	SpareScore       disk.ComponentScore `json:"spareScore"`       // 备用块评分 (权重10%)
	StabilityScore   disk.ComponentScore `json:"stabilityScore"`   // 稳定性评分 (权重5%)

	// 三级预警状态
	AlertLevel   string   `json:"alertLevel"`
	AlertReasons []string `json:"alertReasons"`

	// 建议操作
	Recommendations []string `json:"recommendations"`

	// 寿命预测
	LifePrediction *NVMeLifePredictionEnhanced `json:"lifePrediction,omitempty"`

	// 时间戳
	Timestamp time.Time `json:"timestamp"`
}

// NewNVMeEnhancedMonitor 创建NVMe增强监控器.
func NewNVMeEnhancedMonitor(alertMgr *AlertingManager) *NVMeEnhancedMonitor {
	return &NVMeEnhancedMonitor{
		nvmeMonitor:     disk.NewNVMeMonitor(),
		alertManager:    alertMgr,
		healthHistory:   make(map[string][]NVMeHealthPoint),
		predictionCache: make(map[string]*NVMeLifePredictionEnhanced),
	}
}

// CheckNVMeDevice 检查NVMe设备健康状态.
func (m *NVMeEnhancedMonitor) CheckNVMeDevice(device string) (*NVMeHealthScoreEnhanced, error) {
	// 获取基础健康信息
	info, err := m.nvmeMonitor.GetNVMeHealth(device)
	if err != nil {
		return nil, fmt.Errorf("获取NVMe健康数据失败: %w", err)
	}

	score := &NVMeHealthScoreEnhanced{
		Device:       device,
		Model:        info.Model,
		Timestamp:    time.Now(),
		AlertLevel:   string(info.AlertLevel),
		AlertReasons: info.AlertReasons,
	}

	// 识别厂商品牌
	manufacturer := identifyManufacturer(info.Model)

	// 获取TBW规格
	tbwSpec := disk.LookupTBWSpec(info.Model, info.Size)

	// 计算寿命评分 (权重40%)
	score.LifeScore = m.calculateLifeScore(info, tbwSpec)

	// 计算温度评分 (权重25%)
	score.TemperatureScore = m.calculateTemperatureScore(info)

	// 计算错误评分 (权重20%)
	score.ErrorScore = m.calculateErrorScore(info)

	// 计算备用块评分 (权重10%)
	score.SpareScore = m.calculateSpareScore(info)

	// 计算稳定性评分 (权重5%)
	score.StabilityScore = m.calculateStabilityScore(device)

	// 计算综合评分
	weights := struct {
		Life        float64
		Temperature float64
		Error       float64
		Spare       float64
		Stability   float64
	}{
		Life:        0.40,
		Temperature: 0.25,
		Error:       0.20,
		Spare:       0.10,
		Stability:   0.05,
	}

	score.TotalScore = float64(score.LifeScore.Score)*weights.Life +
		float64(score.TemperatureScore.Score)*weights.Temperature +
		float64(score.ErrorScore.Score)*weights.Error +
		float64(score.SpareScore.Score)*weights.Spare +
		float64(score.StabilityScore.Score)*weights.Stability

	// 确定等级
	score.Grade = scoreToGrade(score.TotalScore)

	// 三级预警评估
	alertLevel, alertReasons := m.evaluateEnhancedAlertLevel(info, score)
	score.AlertLevel = alertLevel
	score.AlertReasons = alertReasons

	// 寿命预测
	prediction := m.predictEnhancedLife(info, tbwSpec, manufacturer)
	score.LifePrediction = prediction

	// 生成建议
	score.Recommendations = m.generateEnhancedRecommendations(info, score)

	// 记录历史数据点
	m.recordHealthPoint(device, info, prediction)

	// 触发告警
	if m.alertManager != nil && score.AlertLevel != "normal" {
		m.triggerNVMeAlert(device, score)
	}

	return score, nil
}

// calculateLifeScore 计算寿命评分 (基于TBW).
func (m *NVMeEnhancedMonitor) calculateLifeScore(info *disk.NVMeHealthInfo, tbwSpec *disk.TBWSpec) disk.ComponentScore {
	score := disk.ComponentScore{
		Weight: 0.40,
	}

	if info.Usage == nil {
		score.Score = 100
		score.Status = "healthy"
		score.Message = "无使用数据"
		return score
	}

	// 计算TBW使用百分比
	pctUsed := float64(info.Usage.PercentageUsed)

	// 如果有TBW规格，精确计算
	if tbwSpec != nil && tbwSpec.TBWTotal > 0 {
		tbwUsed := info.Usage.TBW
		tbwUsedPercent := (tbwUsed / tbwSpec.TBWTotal) * 100

		// TBW使用百分比优先
		if tbwUsedPercent > pctUsed {
			pctUsed = tbwUsedPercent
		}

		score.Value = tbwUsedPercent
		score.Message = fmt.Sprintf("TBW使用 %.1f%% (额定 %.0f TB)", tbwUsedPercent, tbwSpec.TBWTotal)
	} else {
		score.Value = pctUsed
		score.Message = fmt.Sprintf("使用 %.1f%%", pctUsed)
	}

	// 三级阈值评估
	switch {
	case pctUsed >= 90:
		score.Score = 10
		score.Status = "critical"
		score.Message = "使用寿命即将耗尽"
	case pctUsed >= 80:
		score.Score = 30
		score.Status = "warning"
		score.Message = "使用寿命严重下降"
	case pctUsed >= 70:
		score.Score = 50
		score.Status = "warning"
		score.Message = "使用寿命下降"
	case pctUsed >= 50:
		score.Score = 70
		score.Status = "healthy"
		score.Message = "使用寿命正常"
	default:
		score.Score = 100
		score.Status = "healthy"
		score.Message = "使用寿命充足"
	}

	return score
}

// calculateTemperatureScore 计算温度评分.
func (m *NVMeEnhancedMonitor) calculateTemperatureScore(info *disk.NVMeHealthInfo) disk.ComponentScore {
	score := disk.ComponentScore{
		Weight: 0.25,
	}

	if info.Temperature == nil {
		score.Score = 100
		score.Status = "healthy"
		score.Message = "无温度数据"
		return score
	}

	temp := info.Temperature.Current
	score.Value = float64(temp)

	// NVMe温度阈值 (比普通磁盘更严格)
	switch {
	case temp >= 75:
		score.Score = 10
		score.Status = "critical"
		score.Message = fmt.Sprintf("温度严重过高: %d°C", temp)
	case temp >= 65:
		score.Score = 30
		score.Status = "warning"
		score.Message = fmt.Sprintf("温度过高: %d°C", temp)
	case temp >= 55:
		score.Score = 60
		score.Status = "warning"
		score.Message = fmt.Sprintf("温度偏高: %d°C", temp)
	case temp >= 45:
		score.Score = 85
		score.Status = "healthy"
		score.Message = fmt.Sprintf("温度正常: %d°C", temp)
	default:
		score.Score = 100
		score.Status = "healthy"
		score.Message = fmt.Sprintf("温度理想: %d°C", temp)
	}

	return score
}

// calculateErrorScore 计算错误评分.
func (m *NVMeEnhancedMonitor) calculateErrorScore(info *disk.NVMeHealthInfo) disk.ComponentScore {
	score := disk.ComponentScore{
		Weight: 0.20,
	}

	var totalErrors uint64
	var errorTypes []string

	if info.MediaErrors > 0 {
		totalErrors += info.MediaErrors
		errorTypes = append(errorTypes, "媒体错误")
	}
	if info.IntegrityErrors > 0 {
		totalErrors += info.IntegrityErrors
		errorTypes = append(errorTypes, "完整性错误")
	}
	if info.ErrorLogEntries > 0 {
		totalErrors += info.ErrorLogEntries
		errorTypes = append(errorTypes, "错误日志")
	}
	if info.CriticalWarnings > 0 {
		totalErrors += uint64(info.CriticalWarnings) * 10 // 关键警告加权
		errorTypes = append(errorTypes, "关键警告")
	}

	score.Value = float64(totalErrors)

	switch {
	case info.CriticalWarnings > 0:
		score.Score = 0
		score.Status = "critical"
		score.Message = "存在关键警告标志"
	case totalErrors > 100:
		score.Score = 20
		score.Status = "critical"
		score.Message = fmt.Sprintf("错误过多: %s", strings.Join(errorTypes, ", "))
	case totalErrors > 10:
		score.Score = 50
		score.Status = "warning"
		score.Message = fmt.Sprintf("存在错误: %s", strings.Join(errorTypes, ", "))
	case totalErrors > 0:
		score.Score = 80
		score.Status = "healthy"
		score.Message = "少量错误"
	default:
		score.Score = 100
		score.Status = "healthy"
		score.Message = "无错误"
	}

	return score
}

// calculateSpareScore 计算备用块评分.
func (m *NVMeEnhancedMonitor) calculateSpareScore(info *disk.NVMeHealthInfo) disk.ComponentScore {
	score := disk.ComponentScore{
		Weight: 0.10,
	}

	if info.AvailableSpare == nil {
		score.Score = 100
		score.Status = "healthy"
		score.Message = "无备用块数据"
		return score
	}

	spare := info.AvailableSpare.Percentage
	threshold := info.AvailableSpare.Threshold
	score.Value = float64(spare)

	switch {
	case spare < threshold:
		score.Score = 0
		score.Status = "critical"
		score.Message = fmt.Sprintf("备用块低于阈值: %d%% < %d%%", spare, threshold)
	case spare < 20:
		score.Score = 30
		score.Status = "warning"
		score.Message = fmt.Sprintf("备用块偏低: %d%%", spare)
	case spare < 50:
		score.Score = 70
		score.Status = "healthy"
		score.Message = fmt.Sprintf("备用块正常: %d%%", spare)
	default:
		score.Score = 100
		score.Status = "healthy"
		score.Message = fmt.Sprintf("备用块充足: %d%%", spare)
	}

	return score
}

// calculateStabilityScore 计算稳定性评分.
func (m *NVMeEnhancedMonitor) calculateStabilityScore(device string) disk.ComponentScore {
	score := disk.ComponentScore{
		Weight: 0.05,
	}

	m.mu.RLock()
	history := m.healthHistory[device]
	m.mu.RUnlock()

	if len(history) < 2 {
		score.Score = 100
		score.Status = "healthy"
		score.Message = "无历史数据"
		return score
	}

	// 计算健康度变化趋势
	var healthDrop float64
	for i := 1; i < len(history); i++ {
		drop := float64(history[i-1].HealthPercentage) - float64(history[i].HealthPercentage)
		if drop > 0 {
			healthDrop += drop
		}
	}

	score.Value = healthDrop

	switch {
	case healthDrop > 20:
		score.Score = 30
		score.Status = "critical"
		score.Message = "健康度下降明显"
	case healthDrop > 10:
		score.Score = 50
		score.Status = "warning"
		score.Message = "健康度有下降趋势"
	case healthDrop > 5:
		score.Score = 80
		score.Status = "healthy"
		score.Message = "轻微波动"
	default:
		score.Score = 100
		score.Status = "healthy"
		score.Message = "状态稳定"
	}

	return score
}

// predictEnhancedLife 预测NVMe剩余寿命.
func (m *NVMeEnhancedMonitor) predictEnhancedLife(info *disk.NVMeHealthInfo, tbwSpec *disk.TBWSpec, manufacturer string) *NVMeLifePredictionEnhanced {
	if info.Usage == nil {
		return nil
	}

	prediction := &NVMeLifePredictionEnhanced{
		Device:       info.Device,
		Model:        info.Model,
		Manufacturer: manufacturer,
		LastUpdated:  time.Now(),
	}

	// TBW基础计算
	if tbwSpec != nil {
		prediction.TBWTotal = tbwSpec.TBWTotal
		prediction.DWPD = tbwSpec.DWPD
	}

	prediction.TBWUsed = info.Usage.TBW

	// 计算TBW使用百分比
	if prediction.TBWTotal > 0 {
		prediction.TBWUsedPercent = (prediction.TBWUsed / prediction.TBWTotal) * 100
		prediction.RemainingTBW = prediction.TBWTotal - prediction.TBWUsed
	} else {
		// 使用健康百分比估算
		prediction.TBWUsedPercent = float64(info.Usage.PercentageUsed)
		prediction.RemainingTBW = 0
	}

	// 温度影响因子计算
	if info.Temperature != nil {
		temp := float64(info.Temperature.Current)
		// NVMe在高温下磨损加速更明显
		if temp > 40 {
			prediction.TempImpactFactor = 1.0 + ((temp-40)/10)*0.2
		} else {
			prediction.TempImpactFactor = 1.0
		}
	} else {
		prediction.TempImpactFactor = 1.0
	}

	// 写放大因子 (NVMe通常1.5-2.5)
	prediction.WriteAmpFactor = 2.0

	// 磨损影响因子
	prediction.WearImpactFactor = 1.0 + prediction.TBWUsedPercent/100*0.3

	// 计算平均日写入量
	var avgDailyWrites float64
	if info.PowerOnHours > 24 {
		totalWritesGB := info.Usage.TotalWrites
		days := float64(info.PowerOnHours) / 24
		avgDailyWrites = totalWritesGB / days
	}

	// 预估剩余天数
	if avgDailyWrites > 0 && prediction.RemainingTBW > 0 {
		effectiveDailyWrites := avgDailyWrites * prediction.WriteAmpFactor * prediction.TempImpactFactor * prediction.WearImpactFactor
		remainingTBWGB := prediction.RemainingTBW * 1024
		prediction.RemainingLifeDays = int(remainingTBWGB / effectiveDailyWrites)
	} else if prediction.TBWUsedPercent < 100 {
		// 基于TBW百分比估算 (假设平均寿命3-5年)
		remainingPct := 100 - prediction.TBWUsedPercent
		prediction.RemainingLifeDays = int(remainingPct / 100 * 365 * 4)
	} else {
		prediction.RemainingLifeDays = 0
	}

	// 确保天数不为负数
	if prediction.RemainingLifeDays < 0 {
		prediction.RemainingLifeDays = 0
	}

	// 计算剩余寿命百分比
	prediction.RemainingLifePct = 100 - prediction.TBWUsedPercent
	if prediction.RemainingLifePct < 0 {
		prediction.RemainingLifePct = 0
	}

	// 预估结束日期
	prediction.EstimatedEndDate = time.Now().AddDate(0, 0, prediction.RemainingLifeDays)

	// 确定预警级别
	switch {
	case prediction.TBWUsedPercent >= 90:
		prediction.AlertLevel = "emergency"
		prediction.AlertReasons = append(prediction.AlertReasons, "TBW使用超过90%")
	case prediction.TBWUsedPercent >= 80:
		prediction.AlertLevel = "critical"
		prediction.AlertReasons = append(prediction.AlertReasons, "TBW使用超过80%")
	case prediction.TBWUsedPercent >= 70:
		prediction.AlertLevel = "warning"
		prediction.AlertReasons = append(prediction.AlertReasons, "TBW使用超过70%")
	default:
		prediction.AlertLevel = "normal"
	}

	// 确定置信度
	m.mu.RLock()
	historyLen := len(m.healthHistory[info.Device])
	m.mu.RUnlock()

	prediction.DataPoints = historyLen
	if historyLen > 30 {
		prediction.ConfidenceLevel = "high"
	} else if historyLen > 7 {
		prediction.ConfidenceLevel = "medium"
	} else {
		prediction.ConfidenceLevel = "low"
	}

	// 缓存预测结果
	m.mu.Lock()
	m.predictionCache[info.Device] = prediction
	m.mu.Unlock()

	return prediction
}

// evaluateEnhancedAlertLevel 评估增强三级预警.
func (m *NVMeEnhancedMonitor) evaluateEnhancedAlertLevel(info *disk.NVMeHealthInfo, score *NVMeHealthScoreEnhanced) (string, []string) {
	alertLevel := string(info.AlertLevel)
	reasons := info.AlertReasons

	// 综合评分阈值
	if score.TotalScore < 30 {
		alertLevel = "emergency"
		reasons = append(reasons, "健康评分低于30分")
	} else if score.TotalScore < 50 && alertLevel != "emergency" {
		if alertLevel != "critical" {
			alertLevel = "critical"
		}
		reasons = append(reasons, "健康评分低于50分")
	} else if score.TotalScore < 70 && alertLevel != "emergency" && alertLevel != "critical" {
		alertLevel = "warning"
		reasons = append(reasons, "健康评分低于70分")
	}

	// 寿命预测预警
	if score.LifePrediction != nil {
		if score.LifePrediction.RemainingLifeDays < 90 {
			if alertLevel != "emergency" {
				alertLevel = "critical"
			}
			reasons = append(reasons, fmt.Sprintf("预估剩余寿命少于90天 (%d天)", score.LifePrediction.RemainingLifeDays))
		} else if score.LifePrediction.RemainingLifeDays < 180 && alertLevel != "emergency" && alertLevel != "critical" {
			alertLevel = "warning"
			reasons = append(reasons, fmt.Sprintf("预估剩余寿命少于180天 (%d天)", score.LifePrediction.RemainingLifeDays))
		}
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "所有指标正常")
	}

	return alertLevel, reasons
}

// generateEnhancedRecommendations 生成增强建议.
func (m *NVMeEnhancedMonitor) generateEnhancedRecommendations(info *disk.NVMeHealthInfo, score *NVMeHealthScoreEnhanced) []string {
	var recommendations []string

	// 寿命建议
	if score.LifePrediction != nil {
		switch score.LifePrediction.AlertLevel {
		case "emergency":
			recommendations = append(recommendations, "⚠️ NVMe寿命即将耗尽，立即备份数据并更换设备")
		case "critical":
			recommendations = append(recommendations, "🔴 NVMe寿命严重下降，建议尽快更换")
		case "warning":
			recommendations = append(recommendations, "🟡 NVMe寿命下降，建议规划更换方案")
		}
	}

	// 温度建议
	switch score.TemperatureScore.Status {
	case "critical":
		recommendations = append(recommendations, "🌡️ NVMe温度严重过高，立即安装散热片或改善散热")
	case "warning":
		recommendations = append(recommendations, "🌡️ NVMe温度偏高，建议安装散热片")
	}

	// 错误建议
	switch score.ErrorScore.Status {
	case "critical":
		recommendations = append(recommendations, "❌ 检测到严重错误，建议立即备份并更换设备")
	case "warning":
		recommendations = append(recommendations, "⚠️ 存在错误日志，建议运行完整诊断测试")
	}

	// 备用块建议
	switch score.SpareScore.Status {
	case "critical":
		recommendations = append(recommendations, "💾 备用块空间低于阈值，设备可能即将失效")
	case "warning":
		recommendations = append(recommendations, "💾 备用块空间偏低，密切关注")
	}

	// 品牌特定建议
	manufacturer := identifyManufacturer(info.Model)
	switch manufacturer {
	case "Samsung":
		if score.LifePrediction != nil && score.LifePrediction.TBWUsedPercent > 70 {
			recommendations = append(recommendations, "💡 Samsung NVMe建议使用Magician软件进行优化")
		}
	case "Western Digital":
		if score.LifePrediction != nil && score.LifePrediction.TBWUsedPercent > 70 {
			recommendations = append(recommendations, "💡 WD NVMe建议使用Dashboard监控工具")
		}
	case "Intel":
		if score.LifePrediction != nil && score.LifePrediction.TBWUsedPercent > 50 {
			recommendations = append(recommendations, "💡 Intel NVMe Optane系列耐久性高，但仍需监控")
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "✅ NVMe设备状态良好，继续保持定期备份")
	}

	return recommendations
}

// recordHealthPoint 记录健康数据点.
func (m *NVMeEnhancedMonitor) recordHealthPoint(device string, info *disk.NVMeHealthInfo, prediction *NVMeLifePredictionEnhanced) {
	point := NVMeHealthPoint{
		Timestamp:        time.Now(),
		HealthPercentage: info.HealthPercentage,
		AlertLevel:       string(info.AlertLevel),
	}

	if info.Temperature != nil {
		point.Temperature = info.Temperature.Current
	}

	if prediction != nil {
		point.TBWUsedPercent = prediction.TBWUsedPercent
		if prediction.DataPoints > 0 {
			point.WriteRate = prediction.RemainingTBW * 1024 / float64(prediction.RemainingLifeDays)
		}
	}

	m.mu.Lock()
	history := m.healthHistory[device]
	history = append(history, point)

	// 保留最近1000个数据点
	if len(history) > 1000 {
		history = history[len(history)-1000:]
	}
	m.healthHistory[device] = history
	m.mu.Unlock()
}

// triggerNVMeAlert 触发NVMe告警.
func (m *NVMeEnhancedMonitor) triggerNVMeAlert(device string, score *NVMeHealthScoreEnhanced) {
	if m.alertManager == nil {
		return
	}

	severity := "warning"
	switch score.AlertLevel {
	case "emergency":
		severity = "critical"
	case "critical":
		severity = "critical"
	}

	message := fmt.Sprintf("NVMe设备 %s 健康预警 [%s]", device, score.AlertLevel)
	if len(score.AlertReasons) > 0 {
		message += ": " + strings.Join(score.AlertReasons[:3], ", ")
	}

	m.alertManager.triggerAlert(
		"nvme_health",
		severity,
		message,
		device,
		map[string]interface{}{
			"healthScore":     score.TotalScore,
			"alertLevel":      score.AlertLevel,
			"lifePrediction":  score.LifePrediction,
			"recommendations": score.Recommendations,
		},
	)
}

// GetAllNVMeDevices 获取所有NVMe设备健康状态.
func (m *NVMeEnhancedMonitor) GetAllNVMeDevices() ([]*NVMeHealthScoreEnhanced, error) {
	devices, err := m.nvmeMonitor.ScanNVMeDevices()
	if err != nil {
		return nil, err
	}

	scores := make([]*NVMeHealthScoreEnhanced, 0, len(devices))
	for _, device := range devices {
		score, err := m.CheckNVMeDevice(device)
		if err != nil {
			continue
		}
		scores = append(scores, score)
	}

	return scores, nil
}

// GetPredictionCache 获取预测缓存.
func (m *NVMeEnhancedMonitor) GetPredictionCache(device string) *NVMeLifePredictionEnhanced {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.predictionCache[device]
}

// GetHealthHistory 获取健康历史.
func (m *NVMeEnhancedMonitor) GetHealthHistory(device string, limit int) []NVMeHealthPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := m.healthHistory[device]
	if limit > 0 && len(history) > limit {
		return history[len(history)-limit:]
	}
	return history
}

// RunNVMeTest 运行NVMe测试.
func (m *NVMeEnhancedMonitor) RunNVMeTest(device string, testType disk.NVMeTestType) (*disk.NVMeTestResult, error) {
	return m.nvmeMonitor.RunNVMeTest(device, testType)
}

// identifyManufacturer 识别NVMe厂商.
func identifyManufacturer(model string) string {
	modelUpper := strings.ToUpper(model)

	if strings.Contains(modelUpper, "SAMSUNG") || strings.Contains(modelUpper, "SM") {
		return "Samsung"
	}
	if strings.Contains(modelUpper, "WD") || strings.Contains(modelUpper, "WESTERN") || strings.Contains(modelUpper, "SN") {
		return "Western Digital"
	}
	if strings.Contains(modelUpper, "INTEL") || strings.Contains(modelUpper, "SSDPEK") || strings.Contains(modelUpper, "OPTANE") {
		return "Intel"
	}
	if strings.Contains(modelUpper, "SEAGATE") || strings.Contains(modelUpper, "FIRECUDA") || strings.Contains(modelUpper, "ST") {
		return "Seagate"
	}
	if strings.Contains(modelUpper, "KINGSTON") || strings.Contains(modelUpper, "KC") {
		return "Kingston"
	}
	if strings.Contains(modelUpper, "CRUCIAL") || strings.Contains(modelUpper, "CT") {
		return "Crucial"
	}
	if strings.Contains(modelUpper, "CORSAIR") || strings.Contains(modelUpper, "MP") {
		return "Corsair"
	}
	if strings.Contains(modelUpper, "SABRENT") || strings.Contains(modelUpper, "ROCKET") {
		return "Sabrent"
	}

	return "Unknown"
}

// scoreToGrade 分数转等级.
func scoreToGrade(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 50:
		return "D"
	default:
		return "F"
	}
}

// NVMeSummaryReport NVMe汇总报告.
type NVMeSummaryReport struct {
	TotalDevices       int                        `json:"totalDevices"`
	HealthyDevices     int                        `json:"healthyDevices"`
	WarningDevices     int                        `json:"warningDevices"`
	CriticalDevices    int                        `json:"criticalDevices"`
	EmergencyDevices   int                        `json:"emergencyDevices"`
	AvgHealthScore     float64                    `json:"avgHealthScore"`
	AvgTemperature     float64                    `json:"avgTemperature"`
	TotalTBWUsed       float64                    `json:"totalTBWUsed"`
	TotalRemainingTBW  float64                    `json:"totalRemainingTBW"`
	AlertCounts        map[string]int             `json:"alertCounts"`
	ManufacturerCounts map[string]int             `json:"manufacturerCounts"`
	TopRecommendations []string                   `json:"topRecommendations"`
	DeviceReports      []*NVMeHealthScoreEnhanced `json:"deviceReports"`
	Timestamp          time.Time                  `json:"timestamp"`
}

// GenerateSummaryReport 生成汇总报告.
func (m *NVMeEnhancedMonitor) GenerateSummaryReport() (*NVMeSummaryReport, error) {
	devices, err := m.GetAllNVMeDevices()
	if err != nil {
		return nil, err
	}

	report := &NVMeSummaryReport{
		Timestamp:          time.Now(),
		AlertCounts:        make(map[string]int),
		ManufacturerCounts: make(map[string]int),
		DeviceReports:      devices,
	}

	var totalHealthScore, totalTemp float64
	var healthCount, tempCount int

	for _, dev := range devices {
		report.TotalDevices++

		switch dev.AlertLevel {
		case "normal":
			report.HealthyDevices++
		case "warning":
			report.WarningDevices++
		case "critical":
			report.CriticalDevices++
		case "emergency":
			report.EmergencyDevices++
		}

		report.AlertCounts[dev.AlertLevel]++

		totalHealthScore += dev.TotalScore
		healthCount++

		if tempVal, ok := dev.TemperatureScore.Value.(float64); ok && tempVal > 0 {
			totalTemp += tempVal
			tempCount++
		}

		if dev.LifePrediction != nil {
			report.TotalTBWUsed += dev.LifePrediction.TBWUsed
			report.TotalRemainingTBW += dev.LifePrediction.RemainingTBW
		}

		manufacturer := identifyManufacturer(dev.Model)
		report.ManufacturerCounts[manufacturer]++

		// 收集关键建议
		if dev.AlertLevel != "normal" && len(dev.Recommendations) > 0 {
			report.TopRecommendations = append(report.TopRecommendations, dev.Recommendations...)
		}
	}

	if healthCount > 0 {
		report.AvgHealthScore = totalHealthScore / float64(healthCount)
	}
	if tempCount > 0 {
		report.AvgTemperature = totalTemp / float64(tempCount)
	}

	// 去重建议
	seen := make(map[string]bool)
	uniqueRecs := make([]string, 0)
	for _, rec := range report.TopRecommendations {
		if !seen[rec] {
			seen[rec] = true
			uniqueRecs = append(uniqueRecs, rec)
		}
	}
	report.TopRecommendations = uniqueRecs

	return report, nil
}
