package diskpredict

import (
	"fmt"
	"sync"
	"time"
)

// DiskPredictManager 磁盘故障预测管理器
type DiskPredictManager struct {
	mu          sync.RWMutex
	disks       map[string]*DiskInfo         // 磁盘信息
	smartData   map[string]*SMARTData        // SMART数据
	predictions map[string]*PredictionResult // 预测结果
	alerts      []AlertInfo                  // 告警信息
	analyzer    *Analyzer                    // 分析器
	scorer      *Scorer                      // 评分器
}

// NewDiskPredictManager 创建管理器
func NewDiskPredictManager() *DiskPredictManager {
	return &DiskPredictManager{
		disks:       make(map[string]*DiskInfo),
		smartData:   make(map[string]*SMARTData),
		predictions: make(map[string]*PredictionResult),
		alerts:      make([]AlertInfo, 0),
		analyzer:    NewAnalyzer(),
		scorer:      NewScorer(),
	}
}

// RegisterDisk 注册磁盘
func (m *DiskPredictManager) RegisterDisk(disk *DiskInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if disk.Device == "" {
		return fmt.Errorf("设备名称不能为空")
	}

	// 设置注册时间
	if disk.RegisteredAt.IsZero() {
		disk.RegisteredAt = time.Now()
	}

	m.disks[disk.Device] = disk
	return nil
}

// UnregisterDisk 注销磁盘
func (m *DiskPredictManager) UnregisterDisk(device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.disks[device]; !exists {
		return fmt.Errorf("磁盘不存在: %s", device)
	}

	delete(m.disks, device)
	delete(m.smartData, device)
	delete(m.predictions, device)

	return nil
}

// GetDisk 获取磁盘信息
func (m *DiskPredictManager) GetDisk(device string) (*DiskInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, exists := m.disks[device]
	if !exists {
		return nil, fmt.Errorf("磁盘不存在: %s", device)
	}

	return disk, nil
}

// ListDisks 列出所有磁盘
func (m *DiskPredictManager) ListDisks() []DiskInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disks := make([]DiskInfo, 0, len(m.disks))
	for _, disk := range m.disks {
		disks = append(disks, *disk)
	}

	return disks
}

// UpdateSMARTData 更新SMART数据
func (m *DiskPredictManager) UpdateSMARTData(data *SMARTData) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if data.Device == "" {
		return fmt.Errorf("设备名称不能为空")
	}

	// 检查磁盘是否已注册
	if _, exists := m.disks[data.Device]; !exists {
		return fmt.Errorf("磁盘未注册: %s", data.Device)
	}

	// 更新SMART数据
	data.CollectedAt = time.Now()
	m.smartData[data.Device] = data

	// 更新磁盘信息
	m.disks[data.Device].LastScanAt = time.Now()

	return nil
}

// PredictFailure 预测磁盘故障
func (m *DiskPredictManager) PredictFailure(device string) (*PredictionResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 检查磁盘是否存在
	disk, exists := m.disks[device]
	if !exists {
		return nil, fmt.Errorf("磁盘不存在: %s", device)
	}

	// 获取SMART数据
	smartData, exists := m.smartData[device]
	if !exists {
		return nil, fmt.Errorf("SMART数据不存在: %s", device)
	}

	// 分析SMART数据
	analyses := m.analyzer.AnalyzeSMARTData(smartData)

	// 计算健康评分
	healthScore := m.scorer.CalculateHealthScore(
		analyses,
		smartData.Temperature,
		smartData.PowerOnHours,
	)

	// 识别风险因素
	riskFactors := m.analyzer.IdentifyRiskFactors(
		analyses,
		smartData.Temperature,
		smartData.PowerOnHours,
	)

	// 确定状态
	status := m.scorer.DetermineStatus(healthScore)

	// 估算剩余寿命
	estimatedLifeDays := m.scorer.EstimateRemainingLifeDays(
		healthScore,
		status,
		riskFactors,
	)

	// 估算故障日期
	estimatedFailDate := m.scorer.EstimateFailureDate(estimatedLifeDays)

	// 创建预测结果
	result := &PredictionResult{
		Device:             device,
		Model:              disk.Model,
		Serial:             disk.Serial,
		HealthScore:        healthScore,
		Status:             status,
		EstimatedLifeDays:  estimatedLifeDays,
		EstimatedFailDate:  estimatedFailDate,
		RiskFactors:        riskFactors,
		AnalyzedAttributes: analyses,
		PredictedAt:        time.Now(),
	}

	// 缓存预测结果
	m.predictions[device] = result

	// 生成告警
	m.generateAlert(device, status, healthScore)

	return result, nil
}

// PredictAll 预测所有磁盘
func (m *DiskPredictManager) PredictAll() []PredictionResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	results := make([]PredictionResult, 0)

	for device := range m.disks {
		// 检查是否有SMART数据
		if _, exists := m.smartData[device]; !exists {
			continue
		}

		// 分析SMART数据
		analyses := m.analyzer.AnalyzeSMARTData(m.smartData[device])

		// 计算健康评分
		healthScore := m.scorer.CalculateHealthScore(
			analyses,
			m.smartData[device].Temperature,
			m.smartData[device].PowerOnHours,
		)

		// 识别风险因素
		riskFactors := m.analyzer.IdentifyRiskFactors(
			analyses,
			m.smartData[device].Temperature,
			m.smartData[device].PowerOnHours,
		)

		// 确定状态
		status := m.scorer.DetermineStatus(healthScore)

		// 估算剩余寿命
		estimatedLifeDays := m.scorer.EstimateRemainingLifeDays(
			healthScore,
			status,
			riskFactors,
		)

		// 估算故障日期
		estimatedFailDate := m.scorer.EstimateFailureDate(estimatedLifeDays)

		// 创建预测结果
		result := PredictionResult{
			Device:             device,
			Model:              m.disks[device].Model,
			Serial:             m.disks[device].Serial,
			HealthScore:        healthScore,
			Status:             status,
			EstimatedLifeDays:  estimatedLifeDays,
			EstimatedFailDate:  estimatedFailDate,
			RiskFactors:        riskFactors,
			AnalyzedAttributes: analyses,
			PredictedAt:        time.Now(),
		}

		// 缓存预测结果
		m.predictions[device] = &result

		// 生成告警
		m.generateAlert(device, status, healthScore)

		results = append(results, result)
	}

	return results
}

// GetPrediction 获取预测结果
func (m *DiskPredictManager) GetPrediction(device string) (*PredictionResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	prediction, exists := m.predictions[device]
	if !exists {
		return nil, fmt.Errorf("预测结果不存在: %s", device)
	}

	return prediction, nil
}

// ListPredictions 列出所有预测结果
func (m *DiskPredictManager) ListPredictions() []PredictionResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	predictions := make([]PredictionResult, 0, len(m.predictions))
	for _, pred := range m.predictions {
		predictions = append(predictions, *pred)
	}

	return predictions
}

// GetStats 获取统计信息
func (m *DiskPredictManager) GetStats() *DiskStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &DiskStats{
		TotalDisks: len(m.disks),
	}

	totalScore := 0.0
	scoreCount := 0

	for _, disk := range m.disks {
		switch disk.Status {
		case StatusHealthy:
			stats.HealthyDisks++
		case StatusWarning:
			stats.WarningDisks++
		case StatusCritical:
			stats.CriticalDisks++
		case StatusFailed:
			stats.FailedDisks++
		}

		// 如果有预测结果，累加分数
		if pred, exists := m.predictions[disk.Device]; exists {
			totalScore += pred.HealthScore
			scoreCount++
		}
	}

	// 计算平均分数
	if scoreCount > 0 {
		stats.AvgHealthScore = totalScore / float64(scoreCount)
	}

	return stats
}

// GetAlerts 获取告警信息
func (m *DiskPredictManager) GetAlerts(resolved bool) []AlertInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]AlertInfo, 0)
	for _, alert := range m.alerts {
		if alert.Resolved == resolved {
			alerts = append(alerts, alert)
		}
	}

	return alerts
}

// generateAlert 生成告警
func (m *DiskPredictManager) generateAlert(device string, status DiskStatus, score float64) {
	// 只为警告和临界状态生成告警
	if status == StatusHealthy {
		return
	}

	level := "warning"
	if status == StatusCritical || status == StatusFailed {
		level = "critical"
	}

	alert := AlertInfo{
		Device:    device,
		Level:     level,
		Message:   fmt.Sprintf("磁盘健康评分: %.1f, 状态: %s", score, status),
		CreatedAt: time.Now(),
		Resolved:  false,
	}

	m.alerts = append(m.alerts, alert)
}

// ResolveAlert 解决告警
func (m *DiskPredictManager) ResolveAlert(device string, createdAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, alert := range m.alerts {
		if alert.Device == device && alert.CreatedAt.Equal(createdAt) {
			m.alerts[i].Resolved = true
			return nil
		}
	}

	return fmt.Errorf("告警不存在")
}

// ClearResolvedAlerts 清除已解决的告警
func (m *DiskPredictManager) ClearResolvedAlerts() {
	m.mu.Lock()
	defer m.mu.Unlock()

	activeAlerts := make([]AlertInfo, 0)
	for _, alert := range m.alerts {
		if !alert.Resolved {
			activeAlerts = append(activeAlerts, alert)
		}
	}

	m.alerts = activeAlerts
}

// ExportReport 导出健康报告
func (m *DiskPredictManager) ExportReport() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := map[string]interface{}{
		"generated_at": time.Now(),
		"total_disks":  len(m.disks),
		"disks":        make([]map[string]interface{}, 0),
	}

	disks := make([]map[string]interface{}, 0)
	for _, disk := range m.disks {
		diskReport := map[string]interface{}{
			"device":        disk.Device,
			"model":         disk.Model,
			"serial":        disk.Serial,
			"capacity":      disk.Capacity,
			"status":        disk.Status,
			"smart_enabled": disk.SMARTEnabled,
		}

		// 添加预测结果（如果有）
		if pred, exists := m.predictions[disk.Device]; exists {
			diskReport["health_score"] = pred.HealthScore
			diskReport["estimated_life_days"] = pred.EstimatedLifeDays
			diskReport["risk_factors"] = pred.RiskFactors
		}

		disks = append(disks, diskReport)
	}

	report["disks"] = disks
	return report
}
