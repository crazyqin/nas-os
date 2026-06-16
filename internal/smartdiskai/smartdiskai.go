// Package smartdiskai - SMART 数据采集引擎、线性回归分析、温度趋势分析
package smartdiskai

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ============================================================
// SMARTCollector - SMART 数据采集与解析引擎
// ============================================================

// SMARTCollector SMART 数据采集与解析引擎
type SMARTCollector struct {
	mu            sync.RWMutex
	logger        *zap.Logger
	history       map[string][]SMARTData
	analysisCache map[string]*SMARTAnalysisResult
	tempCache     map[string]*TemperatureTrend
	maxHistoryLen int
}

// NewSMARTCollector 创建 SMART 采集引擎
func NewSMARTCollector(logger *zap.Logger, maxHistory int) *SMARTCollector {
	if logger == nil {
		logger = zap.NewNop()
	}
	if maxHistory <= 0 {
		maxHistory = 365
	}
	return &SMARTCollector{
		logger:        logger,
		history:       make(map[string][]SMARTData),
		analysisCache: make(map[string]*SMARTAnalysisResult),
		tempCache:     make(map[string]*TemperatureTrend),
		maxHistoryLen: maxHistory,
	}
}

// RecordData 记录 SMART 数据
func (c *SMARTCollector) RecordData(data SMARTData) {
	c.mu.Lock()
	defer c.mu.Unlock()

	device := data.Device
	c.history[device] = append(c.history[device], data)
	if len(c.history[device]) > c.maxHistoryLen {
		c.history[device] = c.history[device][len(c.history[device])-c.maxHistoryLen:]
	}
	// 清除缓存
	delete(c.analysisCache, device)
	delete(c.tempCache, device)

	c.logger.Debug("记录 SMART 数据",
		zap.String("device", device),
		zap.Int("temperature", data.Temperature),
		zap.Uint64("reallocated_sectors", data.ReallocatedSects),
		zap.Uint64("pending_sectors", data.PendingSects),
	)
}

// GetLatestData 获取设备最新 SMART 数据
func (c *SMARTCollector) GetLatestData(device string) (*SMARTData, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, ok := c.history[device]
	if !ok || len(data) == 0 {
		return nil, fmt.Errorf("设备 %s 无 SMART 数据", device)
	}
	latest := data[len(data)-1]
	return &latest, nil
}

// GetHistory 获取设备历史数据
func (c *SMARTCollector) GetHistory(device string) []SMARTData {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data := c.history[device]
	result := make([]SMARTData, len(data))
	copy(result, data)
	return result
}

// GetDevices 获取所有设备列表
func (c *SMARTCollector) GetDevices() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	devices := make([]string, 0, len(c.history))
	for d := range c.history {
		devices = append(devices, d)
	}
	sort.Strings(devices)
	return devices
}

// ============================================================
// SMART 分析器 - 线性回归与趋势分析
// ============================================================

// Analyze 对设备进行 SMART 分析（含线性回归）
func (c *SMARTCollector) Analyze(device string) (*SMARTAnalysisResult, error) {
	c.mu.RLock()
	if cached, ok := c.analysisCache[device]; ok {
		c.mu.RUnlock()
		return cached, nil
	}
	c.mu.RUnlock()

	history := c.GetHistory(device)
	if len(history) == 0 {
		return nil, fmt.Errorf("设备 %s 无 SMART 数据", device)
	}

	result := &SMARTAnalysisResult{
		Device:     device,
		AnalyzedAt: time.Now(),
	}

	// 分析所有关键属性
	attributeIDs := []SMARTAttributeID{
		SMARTIDReallocatedSectorCt, SMARTIDCurrentPendingSector,
		SMARTIDOfflineUncorrectable, SMARTIDTemperatureCelsius,
		SMARTIDPowerOnHours, SMARTIDWearLevelingCount,
		SMARTIDSeekErrorRate, SMARTIDSpinRetryCount,
		SMARTIDTotalLBAsWritten, SMARTIDTotalLBAsRead,
		SMARTIDUDMAErrorCount, SMARTIDLoadUnloadCycleCount,
		SMARTIDStartStopCount, SMARTIDSpinUpTime,
		SMARTIDSoftReadErrorRate, SMARTIDSSDLifeLeft,
		SMARTIDNANDWrites, SMARTIDUnsafeShutdownCount,
		SMARTIDMultiZoneErrorRate, SMARTIDGSENSEErrorRate,
		SMARTIDHardwareECCRecovered, SMARTIDHeadFlyingHours,
		SMARTIDReallocatedEventCount, SMARTIDPowerCycleCount,
	}

	for _, attrID := range attributeIDs {
		values := c.extractAttributeHistory(history, attrID)
		if len(values) == 0 {
			continue
		}

		attrName := GetAttributeName(attrID)
		trend := AttributeTrend{
			AttributeID:   attrID,
			AttributeName: attrName,
			Current:       values[len(values)-1],
		}

		if len(values) >= 3 {
			regression := c.LinearRegression(values)
			trend.Regression = regression
			trend.Trend = c.DetermineTrend(attrID, regression.Slope)
		} else {
			trend.Trend = TrendStable
		}

		result.Attributes = append(result.Attributes, trend)
	}

	result.OverallTrend = c.determineOverallTrend(result.Attributes)

	c.mu.Lock()
	c.analysisCache[device] = result
	c.mu.Unlock()

	return result, nil
}

// LinearRegression 线性回归分析（导出供测试使用）
func (c *SMARTCollector) LinearRegression(values []uint64) *LinearRegressionResult {
	n := float64(len(values))
	if n < 2 {
		return nil
	}

	var sumX, sumY, sumXY, sumX2 float64
	for i, v := range values {
		x := float64(i)
		y := float64(v)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return nil
	}

	slope := (n*sumXY - sumX*sumY) / denom
	intercept := (sumY - slope*sumX) / n

	// 计算 R²
	meanY := sumY / n
	var ssTot, ssRes float64
	for i, v := range values {
		y := float64(v)
		yPred := slope*float64(i) + intercept
		ssTot += (y - meanY) * (y - meanY)
		ssRes += (y - yPred) * (y - yPred)
	}

	rSquared := 0.0
	if ssTot > 0 {
		rSquared = 1 - ssRes/ssTot
	}

	// 预测未来值（基于每天一个数据点的假设）
	lastVal := float64(values[len(values)-1])
	return &LinearRegressionResult{
		Slope:         slope,
		Intercept:     intercept,
		RSquared:      rSquared,
		Projected90D:  slope*90 + lastVal,
		Projected180D: slope*180 + lastVal,
		Projected365D: slope*365 + lastVal,
	}
}

// DetermineTrend 根据属性类型和斜率确定趋势（导出供测试使用）
func (c *SMARTCollector) DetermineTrend(attrID SMARTAttributeID, slope float64) TrendDirection {
	// 值越低越好的属性
	lowerIsBetter := map[SMARTAttributeID]bool{
		SMARTIDReallocatedSectorCt: true, SMARTIDCurrentPendingSector: true,
		SMARTIDOfflineUncorrectable: true, SMARTIDSeekErrorRate: true,
		SMARTIDSpinRetryCount: true, SMARTIDUDMAErrorCount: true,
		SMARTIDMultiZoneErrorRate: true, SMARTIDGSENSEErrorRate: true,
		SMARTIDReallocatedEventCount: true, SMARTIDUnsafeShutdownCount: true,
		SMARTIDSoftReadErrorRate: true, SMARTIDTemperatureCelsius: true,
		SMARTIDTemperature2: true, SMARTIDCalibrationRetryCount: true,
	}

	if lowerIsBetter[attrID] {
		if slope > 0.1 {
			return TrendDeclining
		} else if slope > 0.01 {
			return TrendStable
		}
		return TrendImproving
	}

	// 值越高越好的属性（如 SSD Life Left、Wear Leveling）
	if attrID == SMARTIDWearLevelingCount || attrID == SMARTIDSSDLifeLeft {
		if slope < -0.1 {
			return TrendDeclining
		} else if slope < 0.01 {
			return TrendStable
		}
		return TrendImproving
	}

	return TrendStable
}

// ============================================================
// 温度趋势分析引擎
// ============================================================

// AnalyzeTemperature 分析设备温度趋势
func (c *SMARTCollector) AnalyzeTemperature(device string) (*TemperatureTrend, error) {
	c.mu.RLock()
	if cached, ok := c.tempCache[device]; ok {
		c.mu.RUnlock()
		return cached, nil
	}
	c.mu.RUnlock()

	history := c.GetHistory(device)
	if len(history) == 0 {
		return nil, fmt.Errorf("设备 %s 无数据", device)
	}

	// 提取温度序列
	temps := make([]int, 0, len(history))
	for _, snapshot := range history {
		temps = append(temps, snapshot.Temperature)
	}

	result := &TemperatureTrend{
		Device:      device,
		CurrentTemp: temps[len(temps)-1],
		AnalyzedAt:  time.Now(),
	}

	// 统计分析
	var sum float64
	maxT, minT := temps[0], temps[0]
	for _, t := range temps {
		sum += float64(t)
		if t > maxT {
			maxT = t
		}
		if t < minT {
			minT = t
		}
	}
	n := float64(len(temps))
	result.AvgTemp = sum / n
	result.MaxTemp = maxT
	result.MinTemp = minT

	// 标准差
	var sumSqDiff float64
	for _, t := range temps {
		diff := float64(t) - result.AvgTemp
		sumSqDiff += diff * diff
	}
	result.TempStdDev = math.Sqrt(sumSqDiff / n)

	// 线性回归分析温度趋势
	if len(temps) >= 3 {
		values := make([]uint64, len(temps))
		for i, t := range temps {
			values[i] = uint64(t)
		}
		regression := c.LinearRegression(values)
		result.Regression = regression
		result.Trend = c.determineTempTrend(regression.Slope)

		// 预测峰值
		predictedPeak := int(regression.Projected90D)
		if predictedPeak > maxT {
			result.PredictedPeak = predictedPeak
		} else {
			result.PredictedPeak = maxT
		}
	} else {
		result.Trend = TrendStable
		result.PredictedPeak = maxT
	}

	// 生成温度告警
	result.Alerts = c.generateTempAlerts(result)

	c.mu.Lock()
	c.tempCache[device] = result
	c.mu.Unlock()

	return result, nil
}

// determineTempTrend 确定温度趋势
func (c *SMARTCollector) determineTempTrend(slope float64) TrendDirection {
	if slope > 0.5 {
		return TrendDeclining // 温度上升 = 恶化
	} else if slope > 0.1 {
		return TrendStable
	} else if slope < -0.1 {
		return TrendImproving // 温度下降 = 改善
	}
	return TrendStable
}

// generateTempAlerts 生成温度告警
func (c *SMARTCollector) generateTempAlerts(trend *TemperatureTrend) []TemperatureAlert {
	var alerts []TemperatureAlert
	now := time.Now()

	// 当前温度告警
	if trend.CurrentTemp >= 60 {
		alerts = append(alerts, TemperatureAlert{
			Level:     "critical",
			Message:   fmt.Sprintf("温度严重过高: %d℃ (阈值: 60℃)", trend.CurrentTemp),
			Temp:      trend.CurrentTemp,
			Threshold: 60,
			CreatedAt: now,
		})
	} else if trend.CurrentTemp >= 50 {
		alerts = append(alerts, TemperatureAlert{
			Level:     "warning",
			Message:   fmt.Sprintf("温度偏高: %d℃ (阈值: 50℃)", trend.CurrentTemp),
			Temp:      trend.CurrentTemp,
			Threshold: 50,
			CreatedAt: now,
		})
	}

	// 温度趋势告警
	if trend.Regression != nil && trend.Regression.Slope > 0.5 {
		alerts = append(alerts, TemperatureAlert{
			Level:   "warning",
			Message: fmt.Sprintf("温度持续上升趋势 (斜率: %.2f℃/天)", trend.Regression.Slope),
			Temp:    trend.CurrentTemp,
			CreatedAt: now,
		})
	}

	// 温度波动告警
	if trend.TempStdDev > 10 {
		alerts = append(alerts, TemperatureAlert{
			Level:   "warning",
			Message: fmt.Sprintf("温度波动过大 (标准差: %.1f℃)", trend.TempStdDev),
			Temp:    trend.CurrentTemp,
			CreatedAt: now,
		})
	}

	// 预测峰值告警
	if trend.PredictedPeak >= 60 {
		alerts = append(alerts, TemperatureAlert{
			Level:     "warning",
			Message:   fmt.Sprintf("预计90天内峰值温度将达: %d℃", trend.PredictedPeak),
			Temp:      trend.PredictedPeak,
			Threshold: 60,
			CreatedAt: now,
		})
	}

	return alerts
}

// ============================================================
// 辅助函数
// ============================================================

// extractAttributeHistory 提取属性历史值
func (c *SMARTCollector) extractAttributeHistory(history []SMARTData, attrID SMARTAttributeID) []uint64 {
	var values []uint64
	for _, snapshot := range history {
		for _, attr := range snapshot.Attributes {
			if attr.ID == attrID {
				values = append(values, attr.RawValue)
				break
			}
		}
	}
	return values
}

// determineOverallTrend 确定整体趋势
func (c *SMARTCollector) determineOverallTrend(trends []AttributeTrend) TrendDirection {
	if len(trends) == 0 {
		return TrendStable
	}
	declining, critical := 0, 0
	for _, t := range trends {
		if t.Trend == TrendDeclining {
			declining++
		}
		if t.Trend == TrendCritical {
			critical++
		}
	}
	if critical > 0 {
		return TrendCritical
	}
	if declining > len(trends)/3 {
		return TrendDeclining
	}
	return TrendStable
}

// GetAttributeName 获取 SMART 属性名称
func GetAttributeName(id SMARTAttributeID) string {
	names := map[SMARTAttributeID]string{
		SMARTIDReallocatedSectorCt: "Reallocated_Sector_Ct", SMARTIDSpinRetryCount: "Spin_Retry_Count",
		SMARTIDCalibrationRetryCount: "Calibration_Retry_Count", SMARTIDPowerCycleCount: "Power_Cycle_Count",
		SMARTIDSoftReadErrorRate: "Soft_Read_Error_Rate", SMARTIDCurrentPendingSector: "Current_Pending_Sector",
		SMARTIDOfflineUncorrectable: "Offline_Uncorrectable", SMARTIDTemperatureCelsius: "Temperature_Celsius",
		SMARTIDPowerOnHours: "Power_On_Hours", SMARTIDWearLevelingCount: "Wear_Leveling_Count",
		SMARTIDTotalLBAsWritten: "Total_LBAs_Written", SMARTIDTotalLBAsRead: "Total_LBAs_Read",
		SMARTIDSeekErrorRate: "Seek_Error_Rate", SMARTIDSpinUpTime: "Spin_Up_Time",
		SMARTIDStartStopCount: "Start_Stop_Count", SMARTIDReallocatedEventCount: "Reallocated_Event_Count",
		SMARTIDUDMAErrorCount: "UDMA_Error_Count",
		SMARTIDMultiZoneErrorRate: "Multi_Zone_Error_Rate", SMARTIDGSENSEErrorRate: "G_Sense_Error_Rate",
		SMARTIDLoadUnloadCycleCount: "Load_Unload_Cycle_Count", SMARTIDHeadFlyingHours: "Head_Flying_Hours",
		SMARTIDTotalHostWrites: "Total_Host_Writes", SMARTIDTotalHostReads: "Total_Host_Reads",
		SMARTIDNANDWrites: "NAND_Writes", SMARTIDSSDLifeLeft: "SSD_Life_Left",
		SMARTIDUnsafeShutdownCount: "Unsafe_Shutdown_Count", SMARTIDTemperature2: "Temperature2",
		SMARTIDHardwareECCRecovered: "Hardware_ECC_Recovered", SMARTIDReportedUncorrect: "Reported_Uncorrect",
	}
	if name, ok := names[id]; ok {
		return name
	}
	return fmt.Sprintf("Unknown_%d", id)
}

// getAttributeValue 从 SMART 数据中获取属性原始值
func getAttributeValue(data *SMARTData, attrID SMARTAttributeID) uint64 {
	for _, attr := range data.Attributes {
		if attr.ID == attrID {
			return attr.RawValue
		}
	}
	return 0
}
