// Package powerbudget 提供功率预算核心管理逻辑
package powerbudget

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 功率预算管理器
type Manager struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	config   *PowerBudgetConfig
	readings []*PowerReading
	budgets  map[string]*PowerBudget
	plans    map[string]*SavingsPlan
	alerts   []*PowerAlert
	stopChan chan struct{}
	running  bool
}

// NewManager 创建功率预算管理器
func NewManager(logger *zap.Logger, config *PowerBudgetConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultPowerBudgetConfig()
	}

	return &Manager{
		logger:   logger,
		config:   config,
		readings: make([]*PowerReading, 0),
		budgets:  make(map[string]*PowerBudget),
		plans:    make(map[string]*SavingsPlan),
		alerts:   make([]*PowerAlert, 0),
		stopChan: make(chan struct{}),
	}
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GetReadings 获取功率读数
func (m *Manager) GetReadings(deviceID string, limit int) ([]*PowerReading, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*PowerReading
	for _, r := range m.readings {
		if deviceID != "" && r.DeviceID != deviceID {
			continue
		}
		result = append(result, r)
	}

	// 如果没有数据，生成模拟数据
	if len(result) == 0 {
		result = m.generateMockReadings(deviceID)
	}

	if limit > 0 && limit < len(result) {
		result = result[len(result)-limit:]
	}

	return result, nil
}

// SetBudget 设置功率预算
func (m *Manager) SetBudget(budget *PowerBudget) (*PowerBudget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil, fmt.Errorf("power budget is disabled")
	}

	if budget.ID == "" {
		budget.ID = generateID()
	}

	budget.IsActive = true
	budget.UpdatedAt = time.Now()
	if budget.CreatedAt.IsZero() {
		budget.CreatedAt = time.Now()
	}

	m.budgets[budget.ID] = budget
	m.logger.Info("budget set",
		zap.String("id", budget.ID),
		zap.String("name", budget.Name),
		zap.Float64("max_watts", budget.MaxWatts))

	return budget, nil
}

// CalculateCost 计算能源成本
func (m *Manager) CalculateCost(periodStart, periodEnd time.Time) (*EnergyCost, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.config.Enabled {
		return nil, fmt.Errorf("power budget is disabled")
	}

	// 计算周期内的读数
	var periodReadings []*PowerReading
	for _, r := range m.readings {
		if r.Timestamp.After(periodStart) && r.Timestamp.Before(periodEnd) {
			periodReadings = append(periodReadings, r)
		}
	}

	// 如果没有数据，使用模拟数据
	if len(periodReadings) == 0 {
		periodReadings = m.generateMockReadings("")
	}

	totalWatts := 0.0
	peakWatts := 0.0
	for _, r := range periodReadings {
		totalWatts += r.Watts
		if r.Watts > peakWatts {
			peakWatts = r.Watts
		}
	}

	averageWatts := totalWatts / float64(len(periodReadings))
	hours := periodEnd.Sub(periodStart).Hours()
	totalKWh := averageWatts * hours / 1000

	// 计算成本（区分峰谷电价）
	peakHours := hours * 0.4 // 假设 40% 峰时
	offPeakHours := hours * 0.6
	peakCost := peakWatts * peakHours / 1000 * m.config.PeakRate
	offPeakCost := averageWatts * offPeakHours / 1000 * m.config.OffPeakRate
	totalCost := peakCost + offPeakCost

	// 计算碳排放
	carbonFootprint := totalKWh * m.config.CarbonFactor

	// 确定周期类型
	period := "daily"
	hoursDiff := periodEnd.Sub(periodStart).Hours()
	if hoursDiff > 168 { // 超过一周
		period = "monthly"
	} else if hoursDiff > 24 {
		period = "weekly"
	}

	energyCost := &EnergyCost{
		ID:              generateID(),
		Period:          period,
		TotalKWh:        totalKWh,
		AverageWatts:    averageWatts,
		PeakWatts:       peakWatts,
		TotalCost:       totalCost,
		Currency:        m.config.DefaultCurrency,
		CostPerKWh:      m.config.ElectricityRate,
		PeakCost:        peakCost,
		OffPeakCost:     offPeakCost,
		CarbonFootprint: carbonFootprint,
		PeriodStart:     periodStart,
		PeriodEnd:       periodEnd,
		CreatedAt:       time.Now(),
	}

	return energyCost, nil
}

// CreateSavingsPlan 创建节能计划
func (m *Manager) CreateSavingsPlan(plan *SavingsPlan) (*SavingsPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil, fmt.Errorf("power budget is disabled")
	}

	if plan.ID == "" {
		plan.ID = generateID()
	}

	plan.IsActive = true
	plan.UpdatedAt = time.Now()
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = time.Now()
	}

	// 计算预估节省
	if plan.EstimatedSaving == 0 {
		plan.EstimatedSaving = m.estimateSavings(plan.Type, plan.TargetDevice)
	}
	if plan.EstimatedCostSaving == 0 {
		plan.EstimatedCostSaving = plan.EstimatedSaving * m.config.ElectricityRate
	}

	m.plans[plan.ID] = plan
	m.logger.Info("savings plan created",
		zap.String("id", plan.ID),
		zap.String("name", plan.Name),
		zap.String("type", string(plan.Type)))

	return plan, nil
}

// GetAlerts 获取告警
func (m *Manager) GetAlerts(level AlertLevel, limit int) ([]*PowerAlert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 检查预算并生成告警
	m.checkBudgets()

	var result []*PowerAlert
	for _, a := range m.alerts {
		if level != "" && a.Level != level {
			continue
		}
		result = append(result, a)
	}

	if limit > 0 && limit < len(result) {
		result = result[len(result)-limit:]
	}

	return result, nil
}

// checkBudgets 检查预算
func (m *Manager) checkBudgets() {
	// 获取当前总功率
	totalWatts := 0.0
	for _, r := range m.readings {
		totalWatts += r.Watts
	}

	for _, budget := range m.budgets {
		if !budget.IsActive {
			continue
		}

		// 检查是否超出预算
		if totalWatts > budget.MaxWatts {
			alert := &PowerAlert{
				ID:             generateID(),
				Level:          AlertCritical,
				Type:           AlertOverBudget,
				Title:          "功率超出预算",
				Message:        fmt.Sprintf("当前功率 %.1fW 超出预算 %.1fW", totalWatts, budget.MaxWatts),
				CurrentWatts:   totalWatts,
				ThresholdWatts: budget.MaxWatts,
				BudgetID:       budget.ID,
				CreatedAt:      time.Now(),
			}
			m.alerts = append(m.alerts, alert)
		} else if totalWatts > budget.WarningWatts {
			alert := &PowerAlert{
				ID:             generateID(),
				Level:          AlertWarning,
				Type:           AlertHighUsage,
				Title:          "功率接近预算",
				Message:        fmt.Sprintf("当前功率 %.1fW 接近警告阈值 %.1fW", totalWatts, budget.WarningWatts),
				CurrentWatts:   totalWatts,
				ThresholdWatts: budget.WarningWatts,
				BudgetID:       budget.ID,
				CreatedAt:      time.Now(),
			}
			m.alerts = append(m.alerts, alert)
		}
	}

	// 限制告警数量
	if len(m.alerts) > m.config.MaxAlerts {
		m.alerts = m.alerts[len(m.alerts)-m.config.MaxAlerts:]
	}
}

// generateMockReadings 生成模拟读数
func (m *Manager) generateMockReadings(deviceID string) []*PowerReading {
	devices := []struct {
		ID        string
		Name      string
		Type      string
		BaseWatts float64
	}{
		{"dev-001", "NAS 主机", "server", 150},
		{"dev-002", "交换机", "network", 30},
		{"dev-003", "UPS", "power", 50},
		{"dev-004", "路由器", "network", 15},
		{"dev-005", "监控摄像头", "security", 10},
	}

	if deviceID != "" {
		for _, d := range devices {
			if d.ID == deviceID {
				devices = []struct {
					ID        string
					Name      string
					Type      string
					BaseWatts float64
				}{d}
				break
			}
		}
	}

	readings := make([]*PowerReading, 0)
	for _, d := range devices {
		for i := 0; i < 24; i++ {
			watts := d.BaseWatts * (1 + float64(i%3)*0.1) // 模拟波动
			readings = append(readings, &PowerReading{
				ID:          generateID(),
				DeviceID:    d.ID,
				DeviceName:  d.Name,
				DeviceType:  d.Type,
				Watts:       watts,
				Voltage:     220,
				Current:     watts / 220,
				PowerFactor: 0.95,
				Frequency:   50,
				Timestamp:   time.Now().Add(-time.Duration(24-i) * time.Hour),
			})
		}
	}

	return readings
}

// estimateSavings 估算节省
func (m *Manager) estimateSavings(savingsType SavingsType, deviceID string) float64 {
	switch savingsType {
	case SavingsSchedule:
		return 50.0 // 定时调度节省 50 kWh/月
	case SavingsIdle:
		return 30.0 // 空闲关机节省 30 kWh/月
	case SavingsEfficiency:
		return 20.0 // 效率优化节省 20 kWh/月
	case SavingsPeakShift:
		return 40.0 // 峰值转移节省 40 kWh/月
	default:
		return 25.0
	}
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *PowerBudgetConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *PowerBudgetConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// ListBudgets 列出所有预算
func (m *Manager) ListBudgets() []*PowerBudget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	budgets := make([]*PowerBudget, 0, len(m.budgets))
	for _, b := range m.budgets {
		budgets = append(budgets, b)
	}
	return budgets
}

// GetBudget 获取预算
func (m *Manager) GetBudget(id string) (*PowerBudget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	budget, ok := m.budgets[id]
	if !ok {
		return nil, fmt.Errorf("budget not found: %s", id)
	}
	return budget, nil
}

// DeleteBudget 删除预算
func (m *Manager) DeleteBudget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.budgets[id]; !ok {
		return fmt.Errorf("budget not found: %s", id)
	}
	delete(m.budgets, id)
	return nil
}

// ListPlans 列出所有节能计划
func (m *Manager) ListPlans() []*SavingsPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plans := make([]*SavingsPlan, 0, len(m.plans))
	for _, p := range m.plans {
		plans = append(plans, p)
	}
	return plans
}

// GetPlan 获取节能计划
func (m *Manager) GetPlan(id string) (*SavingsPlan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[id]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", id)
	}
	return plan, nil
}

// DeletePlan 删除节能计划
func (m *Manager) DeletePlan(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.plans[id]; !ok {
		return fmt.Errorf("plan not found: %s", id)
	}
	delete(m.plans, id)
	return nil
}
