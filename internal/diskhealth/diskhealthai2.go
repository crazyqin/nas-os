// Package diskhealthai2 - 主模块
// SMART 数据分析引擎、健康评分系统、贝叶斯故障预测、维护建议引擎、磁盘组管理
package diskhealth

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ============================================================
// SMARTAnalyzer - SMART 数据分析引擎
// ============================================================

// SMARTAnalyzer SMART 数据分析引擎.
type SMARTAnalyzer struct {
	mu            sync.RWMutex
	history       map[string][]SMARTData
	analysisCache map[string]*SMARTAnalysisResult
	maxHistoryLen int
}

// NewSMARTAnalyzer 创建 SMART 分析器.
func NewSMARTAnalyzer(maxHistory int) *SMARTAnalyzer {
	if maxHistory <= 0 {
		maxHistory = 365
	}
	return &SMARTAnalyzer{
		history:       make(map[string][]SMARTData),
		analysisCache: make(map[string]*SMARTAnalysisResult),
		maxHistoryLen: maxHistory,
	}
}

// RecordData 记录 SMART 数据.
func (a *SMARTAnalyzer) RecordData(data SMARTData) {
	a.mu.Lock()
	defer a.mu.Unlock()

	device := data.Device
	a.history[device] = append(a.history[device], data)
	if len(a.history[device]) > a.maxHistoryLen {
		a.history[device] = a.history[device][len(a.history[device])-a.maxHistoryLen:]
	}
	delete(a.analysisCache, device)
}

// GetLatestData 获取设备最新 SMART 数据.
func (a *SMARTAnalyzer) GetLatestData(device string) (*SMARTData, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	data, ok := a.history[device]
	if !ok || len(data) == 0 {
		return nil, fmt.Errorf("设备 %s 无 SMART 数据", device)
	}
	latest := data[len(data)-1]
	return &latest, nil
}

// GetHistory 获取设备历史数据.
func (a *SMARTAnalyzer) GetHistory(device string) []SMARTData {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.history[device]
}

// GetDevices 获取所有设备列表.
func (a *SMARTAnalyzer) GetDevices() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var devices []string
	for d := range a.history {
		devices = append(devices, d)
	}
	sort.Strings(devices)
	return devices
}

// Analyze 对设备进行 SMART 分析.
func (a *SMARTAnalyzer) Analyze(device string) (*SMARTAnalysisResult, error) {
	a.mu.RLock()
	if cached, ok := a.analysisCache[device]; ok {
		a.mu.RUnlock()
		return cached, nil
	}
	a.mu.RUnlock()

	history := a.GetHistory(device)
	if len(history) == 0 {
		return nil, fmt.Errorf("设备 %s 无 SMART 数据", device)
	}

	result := &SMARTAnalysisResult{
		Device:     device,
		AnalyzedAt: time.Now(),
	}

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
		SMARTIDReallocatedEventCount, SMARTIDTotalHostWrites,
		SMARTIDTotalHostReads, SMARTIDTemperature2,
		SMARTIDCalibrationRetryCount, SMARTIDPowerCycleCount,
	}

	for _, attrID := range attributeIDs {
		values := a.extractAttributeHistory(history, attrID)
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
			regression := a.linearRegression(values)
			trend.Regression = regression
			trend.Trend = a.determineTrend(attrID, regression.Slope)
		} else {
			trend.Trend = TrendStable
		}

		if len(values) >= 5 {
			anomaly := a.zScoreDetection(attrID, attrName, values)
			trend.Anomaly = anomaly
			if anomaly != nil && anomaly.IsAnomaly {
				result.Anomalies = append(result.Anomalies, *anomaly)
			}
		}

		result.Attributes = append(result.Attributes, trend)
	}

	result.OverallTrend = a.determineOverallTrend(result.Attributes)

	a.mu.Lock()
	a.analysisCache[device] = result
	a.mu.Unlock()

	return result, nil
}

// extractAttributeHistory 提取属性历史值.
func (a *SMARTAnalyzer) extractAttributeHistory(history []SMARTData, attrID SMARTAttributeID) []uint64 {
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

// linearRegression 线性回归分析.
func (a *SMARTAnalyzer) linearRegression(values []uint64) *LinearRegressionResult {
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

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / n

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

	projected90D := slope*90 + float64(values[len(values)-1])

	return &LinearRegressionResult{
		Slope:        slope,
		Intercept:    intercept,
		RSquared:     rSquared,
		Projected90D: projected90D,
	}
}

// determineTrend 根据属性类型和斜率确定趋势.
func (a *SMARTAnalyzer) determineTrend(attrID SMARTAttributeID, slope float64) TrendDirection {
	lowerIsBetter := map[SMARTAttributeID]bool{
		SMARTIDReallocatedSectorCt: true, SMARTIDCurrentPendingSector: true,
		SMARTIDOfflineUncorrectable: true, SMARTIDSeekErrorRate: true,
		SMARTIDSpinRetryCount: true, SMARTIDUDMAErrorCount: true,
		SMARTIDMultiZoneErrorRate: true, SMARTIDGSENSEErrorRate: true,
		SMARTIDReallocatedEventCount: true, SMARTIDUnsafeShutdownCount: true,
		SMARTIDSoftReadErrorRate: true, SMARTIDTemperatureCelsius: true,
		SMARTIDTemperature2: true,
	}

	if lowerIsBetter[attrID] {
		if slope > 0.1 {
			return TrendDeclining
		} else if slope > 0.01 {
			return TrendStable
		}
		return TrendImproving
	}

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

// zScoreDetection Z-score 异常检测.
func (a *SMARTAnalyzer) zScoreDetection(attrID SMARTAttributeID, attrName string, values []uint64) *ZScoreAnomaly {
	n := float64(len(values))
	if n < 5 {
		return nil
	}

	var sum float64
	for _, v := range values {
		sum += float64(v)
	}
	mean := sum / n

	var sumSqDiff float64
	for _, v := range values {
		diff := float64(v) - mean
		sumSqDiff += diff * diff
	}
	stdDev := math.Sqrt(sumSqDiff / n)

	current := float64(values[len(values)-1])
	zScore := 0.0
	if stdDev > 0 {
		zScore = (current - mean) / stdDev
	}

	isAnomaly := math.Abs(zScore) > 2.0
	severity := "low"
	if math.Abs(zScore) > 3.0 {
		severity = "high"
	} else if math.Abs(zScore) > 2.5 {
		severity = "medium"
	}

	return &ZScoreAnomaly{
		AttributeID:   attrID,
		AttributeName: attrName,
		CurrentValue:  values[len(values)-1],
		Mean:          mean,
		StdDev:        stdDev,
		ZScore:        zScore,
		IsAnomaly:     isAnomaly,
		Severity:      severity,
	}
}

// determineOverallTrend 确定整体趋势.
func (a *SMARTAnalyzer) determineOverallTrend(trends []AttributeTrend) TrendDirection {
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

// GetAttributeName 获取属性名称.
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
		SMARTIDCurrentPendingECC: "Current_Pending_ECC", SMARTIDUDMAErrorCount: "UDMA_Error_Count",
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
