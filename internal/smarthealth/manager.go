package smarthealth

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager 智能磁盘健康管理器.
type Manager struct {
	mu sync.RWMutex

	logger *zap.Logger
	config *ManagerConfig

	// 磁盘健康数据
	disks map[string]*DiskHealth

	// 告警
	alerts map[string]*HealthAlert

	// 历史数据
	history map[string][]DiskTrend

	// 预测器
	predictor *FailurePredictor

	// 告警管理器
	alertManager *AlertManager
}

// ManagerConfig 管理器配置.
type ManagerConfig struct {
	ScanInterval    time.Duration `json:"scanInterval"`    // 扫描间隔
	TemperatureWarn int           `json:"temperatureWarn"` // 温度告警阈值 (°C)
	TemperatureCrit int           `json:"temperatureCrit"` // 温度严重阈值 (°C)
	ReallocatedWarn int64         `json:"reallocatedWarn"` // 重分配扇区告警阈值
	PendingWarn     int64         `json:"pendingWarn"`     // 待映射扇区告警阈值
	RetentionDays   int           `json:"retentionDays"`   // 历史数据保留天数
	CPUThreshold    float64       `json:"cpuThreshold"`    // CPU阈值
	MemThreshold    float64       `json:"memThreshold"`    // 内存阈值
	DiskThreshold   float64       `json:"diskThreshold"`   // 磁盘阈值
	TempThreshold   float64       `json:"tempThreshold"`   // 温度阈值
	Enabled         bool          `json:"enabled"`         // 是否启用
	Interval        int           `json:"interval"`        // 检查间隔
}

// DefaultManagerConfig 默认配置.
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		ScanInterval:    1 * time.Hour,
		TemperatureWarn: 50,
		TemperatureCrit: 60,
		ReallocatedWarn: 10,
		PendingWarn:     5,
		RetentionDays:   90,
		CPUThreshold:    90,
		MemThreshold:    90,
		DiskThreshold:   90,
		TempThreshold:   70,
		Enabled:         true,
		Interval:        3600,
	}
}

// NewManager 创建磁盘健康管理器.
func NewManager(logger *zap.Logger, config *ManagerConfig) *Manager {
	if config == nil {
		config = DefaultManagerConfig()
	}

	m := &Manager{
		logger:       logger,
		config:       config,
		disks:        make(map[string]*DiskHealth),
		alerts:       make(map[string]*HealthAlert),
		history:      make(map[string][]DiskTrend),
		predictor:    NewFailurePredictor(),
		alertManager: NewAlertManager(logger),
	}

	return m
}

// Initialize 初始化管理器.
func (m *Manager) Initialize() error {
	m.logger.Info("初始化智能磁盘健康管理器")

	// 启动定期扫描
	go m.startPeriodicScan()

	// 启动告警检查
	go m.startAlertChecker()

	return nil
}

// ScanDisks 扫描磁盘.
func (m *Manager) ScanDisks(req *ScanRequest) ([]DiskHealth, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("开始扫描磁盘")

	// 获取磁盘列表
	disks, err := m.detectDisks(req.Devices)
	if err != nil {
		return nil, fmt.Errorf("检测磁盘失败: %w", err)
	}

	// 扫描每个磁盘的 SMART 信息
	for i := range disks {
		if err := m.scanSMART(&disks[i]); err != nil {
			m.logger.Warn("扫描 SMART 失败",
				zap.String("device", disks[i].Device),
				zap.Error(err),
			)
			continue
		}

		// 计算健康评分
		disks[i].HealthScore = m.calculateHealthScore(&disks[i])
		disks[i].Status = m.determineStatus(disks[i].HealthScore)
		disks[i].LastScanned = time.Now()
		disks[i].LastUpdated = time.Now()

		// 存储结果
		m.disks[disks[i].DiskID] = &disks[i]

		// 记录历史
		m.recordHistory(&disks[i])

		// 检查告警
		m.checkAlerts(&disks[i])

		// 运行故障预测
		prediction := m.predictor.Predict(&disks[i], m.getHistory(disks[i].DiskID))
		disks[i].Prediction = prediction
	}

	return disks, nil
}

// GetDiskHealth 获取磁盘健康信息.
func (m *Manager) GetDiskHealth(diskID string) (*DiskHealth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, exists := m.disks[diskID]
	if !exists {
		return nil, fmt.Errorf("磁盘不存在: %s", diskID)
	}

	return disk, nil
}

// GetAllDisksHealth 获取所有磁盘健康信息.
func (m *Manager) GetAllDisksHealth() []*DiskHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disks := make([]*DiskHealth, 0, len(m.disks))
	for _, disk := range m.disks {
		disks = append(disks, disk)
	}

	return disks
}

// PredictFailure 预测故障.
func (m *Manager) PredictFailure(diskID string) (*FailurePrediction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, exists := m.disks[diskID]
	if !exists {
		return nil, fmt.Errorf("磁盘不存在: %s", diskID)
	}

	history := m.getHistory(diskID)
	prediction := m.predictor.Predict(disk, history)

	return prediction, nil
}

// GetHealthAlerts 获取健康告警.
func (m *Manager) GetHealthAlerts(level AlertLevel, unresolved bool) []*HealthAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]*HealthAlert, 0)
	for _, alert := range m.alerts {
		if level != "" && alert.Level != level {
			continue
		}
		if unresolved && alert.Resolved {
			continue
		}
		alerts = append(alerts, alert)
	}

	return alerts
}

// AcknowledgeAlert 确认告警.
func (m *Manager) AcknowledgeAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.alerts[alertID]
	if !exists {
		return fmt.Errorf("告警不存在: %s", alertID)
	}

	alert.AckedAt = time.Now()
	return nil
}

// ResolveAlert 解决告警.
func (m *Manager) ResolveAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.alerts[alertID]
	if !exists {
		return fmt.Errorf("告警不存在: %s", alertID)
	}

	alert.Resolved = true
	alert.ResolvedAt = time.Now()
	return nil
}

// GenerateReport 生成健康报告.
func (m *Manager) GenerateReport() *HealthReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &HealthReport{
		GeneratedAt: time.Now(),
		Summary:     m.generateSummary(),
		Disks:       make([]DiskHealth, 0),
		Alerts:      make([]HealthAlert, 0),
		Trends:      make([]DiskTrend, 0),
	}

	for _, disk := range m.disks {
		report.Disks = append(report.Disks, *disk)
	}

	for _, alert := range m.alerts {
		if !alert.Resolved {
			report.Alerts = append(report.Alerts, *alert)
		}
	}

	// 获取最近的趋势数据
	for diskID, trends := range m.history {
		if len(trends) > 0 {
			report.Trends = append(report.Trends, trends[len(trends)-1])
		}
		_ = diskID
	}

	return report
}

// GetDiskTrend 获取磁盘趋势.
func (m *Manager) GetDiskTrend(diskID string, days int) []DiskTrend {
	m.mu.RLock()
	defer m.mu.RUnlock()

	trends := m.history[diskID]
	if len(trends) == 0 {
		return nil
	}

	// 过滤最近 N 天的数据
	cutoff := time.Now().AddDate(0, 0, -days)
	filtered := make([]DiskTrend, 0)
	for _, t := range trends {
		if t.Timestamp.After(cutoff) {
			filtered = append(filtered, t)
		}
	}

	return filtered
}

// detectDisks 检测磁盘.
func (m *Manager) detectDisks(devices []string) ([]DiskHealth, error) {
	// 简化版本：返回模拟数据
	// 实际实现应该读取 /sys/block 或使用 lsblk
	disks := []DiskHealth{
		{
			DiskID:   uuid.New().String(),
			Device:   "/dev/sda",
			Model:    "Samsung SSD 870 EVO",
			Serial:   "S123456789",
			Type:     DiskTypeSSD,
			Capacity: 1024 * 1024 * 1024 * 1024, // 1TB
		},
		{
			DiskID:   uuid.New().String(),
			Device:   "/dev/sdb",
			Model:    "WD Red Plus",
			Serial:   "WD987654321",
			Type:     DiskTypeHDD,
			Capacity: 4 * 1024 * 1024 * 1024 * 1024, // 4TB
		},
	}

	return disks, nil
}

// scanSMART 扫描 SMART 信息.
func (m *Manager) scanSMART(disk *DiskHealth) error {
	// 简化版本：填充模拟 SMART 数据
	// 实际实现应该调用 smartctl
	disk.SMARTAttributes = []SMARTAttribute{
		{ID: 5, Name: "Reallocated_Sector_Ct", Value: 100, Worst: 100, Threshold: 10, RawValue: 0},
		{ID: 9, Name: "Power_On_Hours", Value: 95, Worst: 95, Threshold: 0, RawValue: 8760},
		{ID: 12, Name: "Power_Cycle_Count", Value: 99, Worst: 99, Threshold: 0, RawValue: 100},
		{ID: 194, Name: "Temperature_Celsius", Value: 67, Worst: 50, Threshold: 0, RawValue: 33},
		{ID: 197, Name: "Current_Pending_Sector", Value: 100, Worst: 100, Threshold: 0, RawValue: 0},
		{ID: 198, Name: "Offline_Uncorrectable", Value: 100, Worst: 100, Threshold: 0, RawValue: 0},
	}

	disk.Temperature = 33
	disk.PowerOnHours = 8760
	disk.PowerCycleCount = 100
	disk.ReallocatedSectors = 0
	disk.PendingSectors = 0
	disk.UncorrectableErrors = 0
	disk.TotalReads = 10 * 1024 * 1024 * 1024 * 1024 // 10TB
	disk.TotalWrites = 5 * 1024 * 1024 * 1024 * 1024 // 5TB

	return nil
}

// calculateHealthScore 计算健康评分.
func (m *Manager) calculateHealthScore(disk *DiskHealth) int {
	score := 100

	// 温度影响
	if disk.Temperature > m.config.TemperatureCrit {
		score -= 30
	} else if disk.Temperature > m.config.TemperatureWarn {
		score -= 15
	}

	// 重分配扇区影响
	if disk.ReallocatedSectors > 0 {
		score -= min(30, int(disk.ReallocatedSectors)*5)
	}

	// 待映射扇区影响
	if disk.PendingSectors > 0 {
		score -= min(20, int(disk.PendingSectors)*3)
	}

	// 不可纠正错误影响
	if disk.UncorrectableErrors > 0 {
		score -= min(25, int(disk.UncorrectableErrors)*10)
	}

	// 通电时间影响（超过5年扣分）
	if disk.PowerOnHours > 5*365*24 {
		score -= 10
	}

	if score < 0 {
		score = 0
	}

	return score
}

// determineStatus 确定状态.
func (m *Manager) determineStatus(score int) DiskStatus {
	switch {
	case score >= 80:
		return DiskStatusHealthy
	case score >= 60:
		return DiskStatusWarning
	case score >= 40:
		return DiskStatusCritical
	default:
		return DiskStatusFailed
	}
}

// recordHistory 记录历史.
func (m *Manager) recordHistory(disk *DiskHealth) {
	trend := DiskTrend{
		DiskID:             disk.DiskID,
		Device:             disk.Device,
		Timestamp:          time.Now(),
		Score:              disk.HealthScore,
		Temperature:        disk.Temperature,
		ReallocatedSectors: disk.ReallocatedSectors,
	}

	m.history[disk.DiskID] = append(m.history[disk.DiskID], trend)

	// 保留最近 N 天的数据
	cutoff := time.Now().AddDate(0, 0, -m.config.RetentionDays)
	history := m.history[disk.DiskID]
	for i, h := range history {
		if h.Timestamp.After(cutoff) {
			m.history[disk.DiskID] = history[i:]
			break
		}
	}
}

// getHistory 获取历史数据.
func (m *Manager) getHistory(diskID string) []DiskTrend {
	return m.history[diskID]
}

// checkAlerts 检查告警.
func (m *Manager) checkAlerts(disk *DiskHealth) {
	// 温度告警
	if disk.Temperature >= m.config.TemperatureCrit {
		m.createAlert(disk, AlertLevelCritical, "temperature",
			"磁盘温度过高",
			fmt.Sprintf("磁盘 %s 温度达到 %d°C，超过严重阈值 %d°C", disk.Device, disk.Temperature, m.config.TemperatureCrit))
	} else if disk.Temperature >= m.config.TemperatureWarn {
		m.createAlert(disk, AlertLevelWarning, "temperature",
			"磁盘温度警告",
			fmt.Sprintf("磁盘 %s 温度达到 %d°C，超过警告阈值 %d°C", disk.Device, disk.Temperature, m.config.TemperatureWarn))
	}

	// 重分配扇区告警
	if disk.ReallocatedSectors >= m.config.ReallocatedWarn {
		m.createAlert(disk, AlertLevelWarning, "reallocated",
			"重分配扇区警告",
			fmt.Sprintf("磁盘 %s 重分配扇区数达到 %d", disk.Device, disk.ReallocatedSectors))
	}

	// 待映射扇区告警
	if disk.PendingSectors >= m.config.PendingWarn {
		m.createAlert(disk, AlertLevelWarning, "pending",
			"待映射扇区警告",
			fmt.Sprintf("磁盘 %s 待映射扇区数达到 %d", disk.Device, disk.PendingSectors))
	}
}

// createAlert 创建告警.
func (m *Manager) createAlert(disk *DiskHealth, level AlertLevel, alertType, title, message string) {
	alert := &HealthAlert{
		ID:        uuid.New().String(),
		DiskID:    disk.DiskID,
		Device:    disk.Device,
		Level:     level,
		Type:      alertType,
		Title:     title,
		Message:   message,
		CreatedAt: time.Now(),
	}

	m.alerts[alert.ID] = alert
	m.logger.Warn("创建磁盘告警",
		zap.String("alertId", alert.ID),
		zap.String("device", disk.Device),
		zap.String("level", string(level)),
		zap.String("message", message),
	)
}

// generateSummary 生成摘要.
func (m *Manager) generateSummary() ReportSummary {
	summary := ReportSummary{
		TotalDisks: len(m.disks),
	}

	totalScore := 0
	for _, disk := range m.disks {
		totalScore += disk.HealthScore
		switch disk.Status {
		case DiskStatusHealthy:
			summary.HealthyDisks++
		case DiskStatusWarning:
			summary.WarningDisks++
		case DiskStatusCritical:
			summary.CriticalDisks++
		case DiskStatusFailed:
			summary.FailedDisks++
		}
	}

	if summary.TotalDisks > 0 {
		summary.AvgHealthScore = float64(totalScore) / float64(summary.TotalDisks)
	}

	for _, alert := range m.alerts {
		if !alert.Resolved {
			summary.TotalAlerts++
		}
	}

	return summary
}

// startPeriodicScan 启动定期扫描.
func (m *Manager) startPeriodicScan() {
	ticker := time.NewTicker(m.config.ScanInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.ScanDisks(&ScanRequest{})
	}
}

// startAlertChecker 启动告警检查器.
func (m *Manager) startAlertChecker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.checkAndResolveAlerts()
	}
}

// checkAndResolveAlerts 检查并解决告警.
func (m *Manager) checkAndResolveAlerts() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, alert := range m.alerts {
		if alert.Resolved {
			continue
		}

		disk, exists := m.disks[alert.DiskID]
		if !exists {
			continue
		}

		// 检查是否应该自动解决
		switch alert.Type {
		case "temperature":
			if disk.Temperature < m.config.TemperatureWarn {
				alert.Resolved = true
				alert.ResolvedAt = time.Now()
			}
		case "reallocated":
			if disk.ReallocatedSectors < m.config.ReallocatedWarn {
				alert.Resolved = true
				alert.ResolvedAt = time.Now()
			}
		case "pending":
			if disk.PendingSectors < m.config.PendingWarn {
				alert.Resolved = true
				alert.ResolvedAt = time.Now()
			}
		}
	}
}

// FailurePredictor 故障预测器.
type FailurePredictor struct {
}

// NewFailurePredictor 创建故障预测器.
func NewFailurePredictor() *FailurePredictor {
	return &FailurePredictor{}
}

// Predict 预测故障.
func (fp *FailurePredictor) Predict(disk *DiskHealth, history []DiskTrend) *FailurePrediction {
	prediction := &FailurePrediction{
		DiskID:      disk.DiskID,
		Device:      disk.Device,
		PredictedAt: time.Now(),
	}

	// 基于健康评分计算故障概率
	switch {
	case disk.HealthScore >= 80:
		prediction.FailureProbability = 0.05
		prediction.EstimatedDaysLeft = 365
		prediction.RiskLevel = "low"
	case disk.HealthScore >= 60:
		prediction.FailureProbability = 0.2
		prediction.EstimatedDaysLeft = 180
		prediction.RiskLevel = "medium"
	case disk.HealthScore >= 40:
		prediction.FailureProbability = 0.5
		prediction.EstimatedDaysLeft = 90
		prediction.RiskLevel = "high"
	default:
		prediction.FailureProbability = 0.8
		prediction.EstimatedDaysLeft = 30
		prediction.RiskLevel = "critical"
	}

	// 趋势分析
	if len(history) >= 2 {
		last := history[len(history)-1]
		prev := history[len(history)-2]

		prediction.TemperatureTrend = TrendData{
			Direction: getDirection(last.Temperature, prev.Temperature),
			Current:   float64(last.Temperature),
			Previous:  float64(prev.Temperature),
			Since:     prev.Timestamp,
		}

		prediction.ReallocatedTrend = TrendData{
			Direction: getDirection(int(last.ReallocatedSectors), int(prev.ReallocatedSectors)),
			Current:   float64(last.ReallocatedSectors),
			Previous:  float64(prev.ReallocatedSectors),
			Since:     prev.Timestamp,
		}
	}

	// 添加建议
	if prediction.RiskLevel == "high" || prediction.RiskLevel == "critical" {
		prediction.Recommendations = append(prediction.Recommendations,
			"建议尽快备份数据",
			"考虑更换磁盘",
		)
	}
	if disk.Temperature > 50 {
		prediction.Recommendations = append(prediction.Recommendations,
			"检查散热系统",
		)
	}

	return prediction
}

// getDirection 获取趋势方向.
func getDirection(current, previous int) string {
	if current > previous {
		return "up"
	} else if current < previous {
		return "down"
	}
	return "stable"
}

// AlertManager 告警管理器.
type AlertManager struct {
	logger *zap.Logger
}

// NewAlertManager 创建告警管理器.
func NewAlertManager(logger *zap.Logger) *AlertManager {
	return &AlertManager{
		logger: logger,
	}
}

// min 返回两个整数中的较小值.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
