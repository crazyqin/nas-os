// Package diskhealth 提供 SMART 磁盘健康监测和故障预测功能
package diskhealth

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// DiskHealthMonitor 磁盘健康监控器
type DiskHealthMonitor struct {
	// mu 互斥锁，保护并发访问
	mu sync.RWMutex
	// diskStates 磁盘状态缓存，key 为设备名
	diskStates map[string]*DiskHealthStatus
	// history 健康历史记录，key 为设备名
	history map[string]*HealthHistory
	// maxHistoryPoints 最大历史记录数
	maxHistoryPoints int
	// scanInterval 扫描间隔
	scanInterval time.Duration
	// stopChan 停止信号
	stopChan chan struct{}
	// logger 日志记录器
	logger *log.Logger
}

// NewDiskHealthMonitor 创建新的磁盘健康监控器
func NewDiskHealthMonitor(logger *log.Logger) *DiskHealthMonitor {
	return &DiskHealthMonitor{
		diskStates:       make(map[string]*DiskHealthStatus),
		history:          make(map[string]*HealthHistory),
		maxHistoryPoints: 365, // 保留一年的历史数据
		scanInterval:     time.Hour * 6,
		stopChan:         make(chan struct{}),
		logger:           logger,
	}
}

// Start 启动监控器
func (m *DiskHealthMonitor) Start(ctx context.Context) error {
	m.logger.Println("[DiskHealth] 启动磁盘健康监控器")

	// 启动时执行一次扫描
	if err := m.ScanAllDisks(); err != nil {
		m.logger.Printf("[DiskHealth] 初始扫描失败: %v", err)
	}

	// 启动定期扫描
	go m.runPeriodicScan(ctx)

	return nil
}

// Stop 停止监控器
func (m *DiskHealthMonitor) Stop() {
	m.logger.Println("[DiskHealth] 停止磁盘健康监控器")
	close(m.stopChan)
}

// runPeriodicScan 运行定期扫描
func (m *DiskHealthMonitor) runPeriodicScan(ctx context.Context) {
	ticker := time.NewTicker(m.scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			if err := m.ScanAllDisks(); err != nil {
				m.logger.Printf("[DiskHealth] 定期扫描失败: %v", err)
			}
		}
	}
}

// ScanAllDisks 扫描所有磁盘
func (m *DiskHealthMonitor) ScanAllDisks() error {
	m.logger.Println("[DiskHealth] 开始扫描所有磁盘")

	// 获取磁盘列表
	devices, err := m.getDiskDevices()
	if err != nil {
		return fmt.Errorf("获取磁盘列表失败: %w", err)
	}

	for _, device := range devices {
		if err := m.ScanDisk(device); err != nil {
			m.logger.Printf("[DiskHealth] 扫描磁盘 %s 失败: %v", device, err)
		}
	}

	return nil
}

// ScanDisk 扫描指定磁盘
func (m *DiskHealthMonitor) ScanDisk(device string) error {
	m.logger.Printf("[DiskHealth] 扫描磁盘: %s", device)

	// 读取 SMART 数据
	smartData, err := m.readSMARTData(device)
	if err != nil {
		return fmt.Errorf("读取 SMART 数据失败: %w", err)
	}

	// 计算健康评分
	healthScore := m.CalculateHealthScore(smartData)

	// 确定风险等级
	riskLevel := m.DetermineRiskLevel(healthScore, smartData)

	// 执行故障预测
	predictedDays, predictedDate := m.PredictFailure(smartData)

	// 更新状态
	m.mu.Lock()
	m.diskStates[device] = &DiskHealthStatus{
		Device:               device,
		Model:                smartData.Model,
		Serial:               smartData.Serial,
		Capacity:             smartData.Capacity,
		HealthScore:          healthScore,
		RiskLevel:            riskLevel,
		SmartAttributes:      smartData.Attributes,
		PredictedLifeDays:    predictedDays,
		PredictedFailureDate: predictedDate,
		LastScanTime:         time.Now(),
		IsSMARTEnabled:       true,
		Temperature:          smartData.Temperature,
		PowerOnHours:         smartData.PowerOnHours,
		WarningMessage:       m.generateWarningMessage(riskLevel, smartData),
	}

	// 更新历史记录
	m.updateHistory(device, healthScore, smartData)
	m.mu.Unlock()

	m.logger.Printf("[DiskHealth] 磁盘 %s 扫描完成，评分: %d, 风险: %s", device, healthScore, riskLevel)
	return nil
}

// CalculateHealthScore 计算健康评分 (0-100)
func (m *DiskHealthMonitor) CalculateHealthScore(data *SmartData) int {
	score := 100

	// 关键属性及其权重
	criticalAttrs := map[int]float64{
		5:  30.0, // Reallocated_Sector_Ct 重分配扇区计数
		197: 25.0, // Current_Pending_Sector 当前待处理扇区
		198: 25.0, // Offline_Uncorrectable 离线不可纠正扇区
	}

	// 检查关键属性
	for _, attr := range data.Attributes {
		if weight, ok := criticalAttrs[attr.ID]; ok {
			// 根据原始值计算扣分
			deduction := m.calculateAttributeDeduction(attr, weight)
			score -= int(deduction)
		}
	}

	// 温度扣分（最佳温度 25-45°C）
	tempDeduction := m.calculateTemperatureDeduction(data.Temperature)
	score -= tempDeduction

	// 通电时间扣分（超过 3 年开始扣分）
	if data.PowerOnHours > 26280 { // 3年 * 365 * 24
		yearsOver := float64(data.PowerOnHours-26280) / 8760.0
		score -= int(yearsOver * 5) // 每年扣 5 分
	}

	// 确保分数在 0-100 范围内
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// calculateAttributeDeduction 计算单个属性的扣分
func (m *DiskHealthMonitor) calculateAttributeDeduction(attr SmartAttribute, weight float64) float64 {
	if attr.IsFailed {
		return weight
	}

	// 如果原始值为 0，不扣分
	if attr.RawValue == 0 {
		return 0
	}

	// 根据原始值的大小计算扣分
	// 1-10 个坏扇区扣 20% 权重
	// 10-100 个坏扇区扣 50% 权重
	// 100+ 个坏扇区扣 100% 权重
	switch {
	case attr.RawValue <= 10:
		return weight * 0.2 * float64(attr.RawValue) / 10.0
	case attr.RawValue <= 100:
		return weight*0.2 + weight*0.3*float64(attr.RawValue-10)/90.0
	default:
		return weight*0.5 + weight*0.5*float64(math.Min(float64(attr.RawValue-100), 900))/900.0
	}
}

// calculateTemperatureDeduction 计算温度扣分
func (m *DiskHealthMonitor) calculateTemperatureDeduction(temp int) int {
	// 最佳温度范围 25-45°C
	switch {
	case temp < 10:
		return 15 // 过冷
	case temp < 25:
		return 5 // 偏冷
	case temp <= 45:
		return 0 // 正常
	case temp <= 55:
		return 10 // 偏热
	case temp <= 65:
		return 20 // 过热
	default:
		return 30 // 危险温度
	}
}

// DetermineRiskLevel 确定风险等级
func (m *DiskHealthMonitor) DetermineRiskLevel(score int, data *SmartData) RiskLevel {
	// 检查是否有关键属性失败
	for _, attr := range data.Attributes {
		if attr.IsCritical && attr.IsFailed {
			return RiskCritical
		}
	}

	// 根据评分确定风险等级
	switch {
	case score < 30:
		return RiskCritical
	case score < 60:
		return RiskWarning
	case score < 80:
		return RiskNormal
	default:
		return RiskHealthy
	}
}

// PredictFailure 预测故障，返回预测剩余天数和故障日期
func (m *DiskHealthMonitor) PredictFailure(data *SmartData) (int, *time.Time) {
	// 收集关键属性的历史趋势
	trends := m.analyzeAttributeTrends(data)

	if len(trends) == 0 {
		// 无历史数据，无法预测
		return -1, nil
	}

	// 找到最短剩余寿命
	minDays := math.MaxInt32
	for _, trend := range trends {
		if trend.Slope > 0 { // 属性值在增长（如坏扇区增多）
			// 使用线性回归预测何时达到阈值
			days := m.predictDaysToThreshold(trend)
			if days > 0 && days < minDays {
				minDays = days
			}
		}
	}

	if minDays == math.MaxInt32 {
		// 无法预测或趋势良好
		return -1, nil
	}

	// 限制最大预测时间为 10 年
	if minDays > 3650 {
		minDays = 3650
	}

	failureDate := time.Now().AddDate(0, 0, minDays)
	return minDays, &failureDate
}

// analyzeAttributeTrends 分析属性趋势
func (m *DiskHealthMonitor) analyzeAttributeTrends(data *SmartData) []LinearRegressionResult {
	var results []LinearRegressionResult

	// 分析关键属性的趋势
	criticalAttrIDs := []int{5, 197, 198} // Reallocated, Pending, Offline Uncorrectable

	for _, attrID := range criticalAttrIDs {
		// 获取历史记录中的该属性值
		historyData := m.getAttrHistory(data.Serial, attrID)
		if len(historyData) < 3 {
			continue // 数据点不足，无法进行回归分析
		}

		// 执行线性回归
		result := m.linearRegression(historyData)
		if result.Slope > 0 { // 值在增长
			results = append(results, result)
		}
	}

	return results
}

// getAttrHistory 获取属性历史数据
func (m *DiskHealthMonitor) getAttrHistory(serial string, attrID int) []struct {
	days  float64
	value float64
} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 查找对应设备的历史记录
	var historyData []struct {
		days  float64
		value float64
	}

	// 遍历历史记录查找匹配的设备
	for _, hist := range m.history {
		if len(hist.Records) > 0 {
			// 使用第一个记录的时间作为基准
			baseTime := hist.Records[0].Timestamp

			for _, record := range hist.Records {
				days := record.Timestamp.Sub(baseTime).Hours() / 24.0

				var value uint64
				switch attrID {
				case 5:
					value = record.ReallocatedSectors
				case 197:
					value = record.CurrentPendingSectors
				case 198:
					value = record.OfflineUncorrectable
				}

				historyData = append(historyData, struct {
					days  float64
					value float64
				}{days, float64(value)})
			}
			break
		}
	}

	return historyData
}

// linearRegression 线性回归
func (m *DiskHealthMonitor) linearRegression(data []struct {
	days  float64
	value float64
}) LinearRegressionResult {
	n := float64(len(data))
	if n < 2 {
		return LinearRegressionResult{}
	}

	// 计算均值
	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for _, d := range data {
		sumX += d.days
		sumY += d.value
		sumXY += d.days * d.value
		sumX2 += d.days * d.days
		sumY2 += d.value * d.value
	}

	meanX := sumX / n
	meanY := sumY / n

	// 计算斜率和截距
	slope := (sumXY - n*meanX*meanY) / (sumX2 - n*meanX*meanX)
	intercept := meanY - slope*meanX

	// 计算 R²
	ssRes := 0.0
	ssTot := 0.0
	for _, d := range data {
		predicted := slope*d.days + intercept
		ssRes += (d.value - predicted) * (d.value - predicted)
		ssTot += (d.value - meanY) * (d.value - meanY)
	}

	rSquared := 0.0
	if ssTot > 0 {
		rSquared = 1 - ssRes/ssTot
	}

	// 预测当前值（假设当前时间为最新数据点的下一天）
	lastDay := data[len(data)-1].days
	predictedValue := slope*(lastDay+1) + intercept

	return LinearRegressionResult{
		Slope:          slope,
		Intercept:      intercept,
		RSquared:       rSquared,
		PredictedValue: predictedValue,
	}
}

// predictDaysToThreshold 预测达到阈值的天数
func (m *DiskHealthMonitor) predictDaysToThreshold(trend LinearRegressionResult) int {
	// 设定阈值为 100（坏扇区数量）
	threshold := 100.0

	if trend.Slope <= 0 {
		return -1 // 趋势向好或平稳
	}

	// 计算达到阈值的天数
	// threshold = slope * days + intercept
	// days = (threshold - intercept) / slope
	days := (threshold - trend.Intercept) / trend.Slope

	if days < 0 {
		return -1
	}

	return int(days)
}

// generateWarningMessage 生成警告信息
func (m *DiskHealthMonitor) generateWarningMessage(level RiskLevel, data *SmartData) string {
	switch level {
	case RiskCritical:
		return fmt.Sprintf("磁盘 %s 健康状况严重，建议立即备份数据并更换磁盘", data.Model)
	case RiskWarning:
		return fmt.Sprintf("磁盘 %s 存在潜在问题，建议密切关注并定期备份", data.Model)
	case RiskNormal:
		return fmt.Sprintf("磁盘 %s 有轻微异常，建议定期检查", data.Model)
	default:
		return ""
	}
}

// updateHistory 更新历史记录
func (m *DiskHealthMonitor) updateHistory(device string, score int, data *SmartData) {
	// 查找或创建历史记录
	hist, exists := m.history[device]
	if !exists {
		hist = &HealthHistory{
			Device: device,
		}
		m.history[device] = hist
	}

	// 获取 SMART 属性中的原始值
	var reallocated, pending, offline uint64
	for _, attr := range data.Attributes {
		switch attr.ID {
		case 5:
			reallocated = attr.RawValue
		case 197:
			pending = attr.RawValue
		case 198:
			offline = attr.RawValue
		}
	}

	// 添加新记录
	record := HealthRecord{
		Timestamp:            time.Now(),
		HealthScore:          score,
		Temperature:          data.Temperature,
		ReallocatedSectors:   reallocated,
		CurrentPendingSectors: pending,
		OfflineUncorrectable: offline,
	}

	hist.Records = append(hist.Records, record)

	// 限制历史记录数量
	if len(hist.Records) > m.maxHistoryPoints {
		hist.Records = hist.Records[len(hist.Records)-m.maxHistoryPoints:]
	}

	// 计算趋势
	if len(hist.Records) >= 2 {
		m.calculateTrends(hist)
	}
}

// calculateTrends 计算趋势
func (m *DiskHealthMonitor) calculateTrends(hist *HealthHistory) {
	n := len(hist.Records)
	if n < 2 {
		return
	}

	// 计算健康评分趋势（最近 30 个点）
	start := n - 30
	if start < 0 {
		start = 0
	}
	recent := hist.Records[start:]

	var sumScoreDiff, sumTempDiff float64
	count := float64(len(recent) - 1)

	for i := 1; i < len(recent); i++ {
		sumScoreDiff += float64(recent[i].HealthScore - recent[i-1].HealthScore)
		sumTempDiff += float64(recent[i].Temperature - recent[i-1].Temperature)
	}

	if count > 0 {
		hist.TrendScore = sumScoreDiff / count
		hist.TrendTemperature = sumTempDiff / count
	}
}

// GetAllDiskHealth 获取所有磁盘健康状态
func (m *DiskHealthMonitor) GetAllDiskHealth() []*DiskHealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*DiskHealthStatus, 0, len(m.diskStates))
	for _, status := range m.diskStates {
		result = append(result, status)
	}
	return result
}

// GetDiskHealth 获取指定磁盘健康状态
func (m *DiskHealthMonitor) GetDiskHealth(device string) (*DiskHealthStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, exists := m.diskStates[device]
	return status, exists
}

// GetDiskHistory 获取磁盘健康历史
func (m *DiskHealthMonitor) GetDiskHistory(device string) (*HealthHistory, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hist, exists := m.history[device]
	return hist, exists
}

// TriggerScan 触发扫描
func (m *DiskHealthMonitor) TriggerScan(devices []string, force bool) (*ScanResponse, error) {
	if force || len(devices) == 0 {
		// 扫描所有磁盘
		if err := m.ScanAllDisks(); err != nil {
			return &ScanResponse{
				Status:  "error",
				Message: fmt.Sprintf("扫描失败: %v", err),
			}, err
		}
		return &ScanResponse{
			Status:         "success",
			Message:        "扫描完成",
			DevicesScanned: len(m.diskStates),
		}, nil
	}

	// 扫描指定磁盘
	scanned := 0
	for _, device := range devices {
		if err := m.ScanDisk(device); err != nil {
			m.logger.Printf("[DiskHealth] 扫描磁盘 %s 失败: %v", device, err)
			continue
		}
		scanned++
	}

	return &ScanResponse{
		Status:         "success",
		Message:        fmt.Sprintf("扫描完成，成功扫描 %d/%d 个设备", scanned, len(devices)),
		DevicesScanned: scanned,
	}, nil
}
