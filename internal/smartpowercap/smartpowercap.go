// Package smartpowercap 提供功耗智能封顶功能
// 实时功耗监控、功耗预算分配、峰值功耗限制、功耗报表和趋势、节能策略自动应用
package smartpowercap

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// PowerMode 功耗模式
type PowerMode string

const (
	PowerModeEco         PowerMode = "eco"         // 节能模式
	PowerModeBalanced    PowerMode = "balanced"    // 均衡模式
	PowerModePerformance PowerMode = "performance" // 性能模式
	PowerModeCustom      PowerMode = "custom"      // 自定义模式
)

// PowerState 电源状态
type PowerState string

const (
	PowerStateNormal    PowerState = "normal"    // 正常
	PowerStateWarning   PowerState = "warning"   // 警告
	PowerStateThrottled PowerState = "throttled" // 限流
	PowerStateCritical  PowerState = "critical"  // 严重
)

// PowerReading 功耗读数
type PowerReading struct {
	Timestamp  time.Time `json:"timestamp"`
	TotalPower float64   `json:"totalPower"` // 总功耗 (W)
	CPUPower   float64   `json:"cpuPower"`   // CPU功耗
	GPUPower   float64   `json:"gpuPower"`   // GPU功耗
	DrivePower float64   `json:"drivePower"` // 硬盘功耗
	FanPower   float64   `json:"fanPower"`   // 风扇功耗
	OtherPower float64   `json:"otherPower"` // 其他功耗
}

// PowerBudget 功耗预算
type PowerBudget struct {
	ID           string    `json:"id"`           // 预算ID
	Name         string    `json:"name"`         // 预算名称
	MaxPower     float64   `json:"maxPower"`     // 最大功耗 (W)
	CurrentPower float64   `json:"currentPower"` // 当前功耗
	UsedPercent  float64   `json:"usedPercent"`  // 使用率 (%)
	StartTime    time.Time `json:"startTime"`    // 开始时间
	EndTime      time.Time `json:"endTime"`      // 结束时间
	Enabled      bool      `json:"enabled"`      // 是否启用
}

// PowerLimit 功耗限制
type PowerLimit struct {
	ID        string    `json:"id"`        // 限制ID
	Name      string    `json:"name"`      // 限制名称
	PeakPower float64   `json:"peakPower"` // 峰值功耗限制 (W)
	Sustained float64   `json:"sustained"` // 持续功耗限制 (W)
	Duration  int       `json:"duration"`  // 持续时间 (秒)
	Enabled   bool      `json:"enabled"`   // 是否启用
	UpdatedAt time.Time `json:"updatedAt"`
}

// PowerPolicy 节能策略
type PowerPolicy struct {
	ID          string    `json:"id"`          // 策略ID
	Name        string    `json:"name"`        // 策略名称
	Mode        PowerMode `json:"mode"`        // 功耗模式
	MaxPower    float64   `json:"maxPower"`    // 最大功耗
	CPUThrottle float64   `json:"cpuThrottle"` // CPU限流 (%)
	GPUThrottle float64   `json:"gpuThrottle"` // GPU限流 (%)
	Enabled     bool      `json:"enabled"`     // 是否启用
	AutoApply   bool      `json:"autoApply"`   // 自动应用
	UpdatedAt   time.Time `json:"updatedAt"`
}

// PowerReport 功耗报表
type PowerReport struct {
	Timestamp     time.Time      `json:"timestamp"`
	Period        string         `json:"period"`        // 报表周期
	TotalEnergy   float64        `json:"totalEnergy"`   // 总能耗 (Wh)
	AvgPower      float64        `json:"avgPower"`      // 平均功耗 (W)
	PeakPower     float64        `json:"peakPower"`     // 峰值功耗 (W)
	MinPower      float64        `json:"minPower"`      // 最低功耗 (W)
	Readings      []PowerReading `json:"readings"`      // 功耗读数
	BudgetUsage   float64        `json:"budgetUsage"`   // 预算使用率 (%)
	EstimatedCost float64        `json:"estimatedCost"` // 预估电费
}

// PowerTrend 功耗趋势
type PowerTrend struct {
	Timestamp   time.Time `json:"timestamp"`
	AvgPower    float64   `json:"avgPower"`
	PeakPower   float64   `json:"peakPower"`
	EnergyUsage float64   `json:"energyUsage"`
}

// PowerAlert 功耗告警
type PowerAlert struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // 告警类型 (over_budget/throttled/critical)
	Message   string    `json:"message"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Timestamp time.Time `json:"timestamp"`
}

// ========== Manager ==========

// Manager 功耗智能封顶管理器
type Manager struct {
	mu           sync.RWMutex
	currentMode  PowerMode
	currentState PowerState
	reading      *PowerReading
	budgets      map[string]*PowerBudget
	limits       map[string]*PowerLimit
	policies     map[string]*PowerPolicy
	alerts       []PowerAlert
	maxAlerts    int
	history      []PowerReading
	maxHistory   int
	trends       []PowerTrend
	stopCh       chan struct{}
	running      bool
}

// NewManager 创建管理器
func NewManager() *Manager {
	m := &Manager{
		currentMode:  PowerModeBalanced,
		currentState: PowerStateNormal,
		budgets:      make(map[string]*PowerBudget),
		limits:       make(map[string]*PowerLimit),
		policies:     make(map[string]*PowerPolicy),
		maxAlerts:    100,
		maxHistory:   360, // 最多6小时 (10秒一次)
		stopCh:       make(chan struct{}),
	}

	// 初始化默认配置
	m.initDefaults()

	return m
}

// initDefaults 初始化默认配置
func (m *Manager) initDefaults() {
	// 默认功耗预算
	m.budgets["daily"] = &PowerBudget{
		ID:       "daily",
		Name:     "每日预算",
		MaxPower: 5000, // 5000Wh
		Enabled:  true,
	}

	// 默认功耗限制
	m.limits["peak"] = &PowerLimit{
		ID:        "peak",
		Name:      "峰值限制",
		PeakPower: 300, // 300W
		Sustained: 250, // 250W
		Duration:  10,  // 10秒
		Enabled:   true,
	}

	// 默认节能策略
	m.policies["eco"] = &PowerPolicy{
		ID:          "eco",
		Name:        "节能策略",
		Mode:        PowerModeEco,
		MaxPower:    150,
		CPUThrottle: 70,
		GPUThrottle: 60,
		Enabled:     false,
		AutoApply:   false,
	}

	m.policies["balanced"] = &PowerPolicy{
		ID:          "balanced",
		Name:        "均衡策略",
		Mode:        PowerModeBalanced,
		MaxPower:    250,
		CPUThrottle: 90,
		GPUThrottle: 85,
		Enabled:     true,
		AutoApply:   true,
	}
}

// GetCurrentReading 获取当前功耗读数
func (m *Manager) GetCurrentReading() *PowerReading {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.reading == nil {
		return &PowerReading{Timestamp: time.Now()}
	}
	return m.reading
}

// GetState 获取当前状态
func (m *Manager) GetState() PowerState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentState
}

// GetMode 获取当前模式
func (m *Manager) GetMode() PowerMode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentMode
}

// SetMode 设置功耗模式
func (m *Manager) SetMode(mode PowerMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch mode {
	case PowerModeEco, PowerModeBalanced, PowerModePerformance, PowerModeCustom:
		m.currentMode = mode
		log.Printf("[功耗管理] 切换模式: %s", mode)
		return nil
	default:
		return fmt.Errorf("invalid power mode: %s", mode)
	}
}

// AddBudget 添加功耗预算
func (m *Manager) AddBudget(budget *PowerBudget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if budget.ID == "" {
		return fmt.Errorf("budget ID is required")
	}

	if _, exists := m.budgets[budget.ID]; exists {
		return fmt.Errorf("budget already exists: %s", budget.ID)
	}

	budget.StartTime = time.Now()
	m.budgets[budget.ID] = budget
	log.Printf("[功耗管理] 添加预算: %s (最大: %.1fW)", budget.Name, budget.MaxPower)
	return nil
}

// UpdateBudget 更新功耗预算
func (m *Manager) UpdateBudget(budgetID string, maxPower float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	budget, ok := m.budgets[budgetID]
	if !ok {
		return fmt.Errorf("budget not found: %s", budgetID)
	}

	budget.MaxPower = maxPower
	log.Printf("[功耗管理] 更新预算: %s -> %.1fW", budgetID, maxPower)
	return nil
}

// GetBudget 获取功耗预算
func (m *Manager) GetBudget(budgetID string) (*PowerBudget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	budget, ok := m.budgets[budgetID]
	if !ok {
		return nil, fmt.Errorf("budget not found: %s", budgetID)
	}
	return budget, nil
}

// ListBudgets 列出所有预算
func (m *Manager) ListBudgets() []*PowerBudget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var budgets []*PowerBudget
	for _, b := range m.budgets {
		budgets = append(budgets, b)
	}
	return budgets
}

// RemoveBudget 移除预算
func (m *Manager) RemoveBudget(budgetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.budgets[budgetID]; !exists {
		return fmt.Errorf("budget not found: %s", budgetID)
	}

	delete(m.budgets, budgetID)
	log.Printf("[功耗管理] 移除预算: %s", budgetID)
	return nil
}

// AddLimit 添加功耗限制
func (m *Manager) AddLimit(limit *PowerLimit) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if limit.ID == "" {
		return fmt.Errorf("limit ID is required")
	}

	if _, exists := m.limits[limit.ID]; exists {
		return fmt.Errorf("limit already exists: %s", limit.ID)
	}

	m.limits[limit.ID] = limit
	log.Printf("[功耗管理] 添加限制: %s (峰值: %.1fW, 持续: %.1fW)", limit.Name, limit.PeakPower, limit.Sustained)
	return nil
}

// UpdateLimit 更新功耗限制
func (m *Manager) UpdateLimit(limitID string, peakPower, sustained float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	limit, ok := m.limits[limitID]
	if !ok {
		return fmt.Errorf("limit not found: %s", limitID)
	}

	limit.PeakPower = peakPower
	limit.Sustained = sustained
	limit.UpdatedAt = time.Now()
	log.Printf("[功耗管理] 更新限制: %s", limitID)
	return nil
}

// GetLimit 获取功耗限制
func (m *Manager) GetLimit(limitID string) (*PowerLimit, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit, ok := m.limits[limitID]
	if !ok {
		return nil, fmt.Errorf("limit not found: %s", limitID)
	}
	return limit, nil
}

// ListLimits 列出所有限制
func (m *Manager) ListLimits() []*PowerLimit {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var limits []*PowerLimit
	for _, l := range m.limits {
		limits = append(limits, l)
	}
	return limits
}

// AddPolicy 添加节能策略
func (m *Manager) AddPolicy(policy *PowerPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		return fmt.Errorf("policy ID is required")
	}

	if _, exists := m.policies[policy.ID]; exists {
		return fmt.Errorf("policy already exists: %s", policy.ID)
	}

	policy.UpdatedAt = time.Now()
	m.policies[policy.ID] = policy
	log.Printf("[功耗管理] 添加策略: %s", policy.Name)
	return nil
}

// UpdatePolicy 更新节能策略
func (m *Manager) UpdatePolicy(policyID string, policy *PowerPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[policyID]; !exists {
		return fmt.Errorf("policy not found: %s", policyID)
	}

	policy.ID = policyID
	policy.UpdatedAt = time.Now()
	m.policies[policyID] = policy
	log.Printf("[功耗管理] 更新策略: %s", policyID)
	return nil
}

// GetPolicy 获取节能策略
func (m *Manager) GetPolicy(policyID string) (*PowerPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[policyID]
	if !ok {
		return nil, fmt.Errorf("policy not found: %s", policyID)
	}
	return policy, nil
}

// ListPolicies 列出所有策略
func (m *Manager) ListPolicies() []*PowerPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var policies []*PowerPolicy
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// ApplyPolicy 应用节能策略
func (m *Manager) ApplyPolicy(policyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, ok := m.policies[policyID]
	if !ok {
		return fmt.Errorf("policy not found: %s", policyID)
	}

	// 应用策略
	m.currentMode = policy.Mode
	log.Printf("[功耗管理] 应用策略: %s (模式: %s)", policy.Name, policy.Mode)
	return nil
}

// GetReport 获取功耗报表
func (m *Manager) GetReport(period string) *PowerReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &PowerReport{
		Timestamp: time.Now(),
		Period:    period,
	}

	if len(m.history) == 0 {
		return report
	}

	// 计算统计数据
	var totalPower float64
	var peakPower float64
	minPower := m.history[0].TotalPower

	for _, r := range m.history {
		totalPower += r.TotalPower
		if r.TotalPower > peakPower {
			peakPower = r.TotalPower
		}
		if r.TotalPower < minPower {
			minPower = r.TotalPower
		}
	}

	report.AvgPower = totalPower / float64(len(m.history))
	report.PeakPower = peakPower
	report.MinPower = minPower
	report.Readings = m.history

	// 计算能耗 (Wh)
	duration := time.Duration(len(m.history)) * 10 * time.Second
	report.TotalEnergy = report.AvgPower * duration.Hours()

	return report
}

// GetTrends 获取功耗趋势
func (m *Manager) GetTrends(duration time.Duration) []PowerTrend {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	var trends []PowerTrend
	for _, t := range m.trends {
		if t.Timestamp.After(cutoff) {
			trends = append(trends, t)
		}
	}
	return trends
}

// GetAlerts 获取告警
func (m *Manager) GetAlerts() []PowerAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]PowerAlert, len(m.alerts))
	copy(alerts, m.alerts)
	return alerts
}

// ClearAlerts 清除告警
func (m *Manager) ClearAlerts() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.alerts = nil
	log.Println("[功耗管理] 清除所有告警")
}

// collect 采集一次功耗数据
func (m *Manager) collect() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 模拟功耗数据
	m.reading = &PowerReading{
		Timestamp:  now,
		TotalPower: 180.0 + float64(now.Second()%20),
		CPUPower:   80.0 + float64(now.Second()%10),
		GPUPower:   50.0 + float64(now.Second()%5),
		DrivePower: 30.0 + float64(now.Second()%3),
		FanPower:   10.0,
		OtherPower: 10.0,
	}

	// 记录历史
	m.history = append(m.history, *m.reading)
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}

	// 更新预算使用情况
	for _, budget := range m.budgets {
		if budget.Enabled {
			budget.CurrentPower = m.reading.TotalPower
			budget.UsedPercent = (budget.CurrentPower / budget.MaxPower) * 100
		}
	}

	// 检查限制和告警
	m.checkLimits()
	m.checkBudgets()

	// 自动应用策略
	m.autoApplyPolicy()
}

// checkLimits 检查功耗限制
func (m *Manager) checkLimits() {
	if m.reading == nil {
		return
	}

	for _, limit := range m.limits {
		if !limit.Enabled {
			continue
		}

		if m.reading.TotalPower > limit.PeakPower {
			m.currentState = PowerStateThrottled
			alert := PowerAlert{
				ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
				Type:      "over_limit",
				Message:   fmt.Sprintf("功耗超过峰值限制: %.1fW > %.1fW", m.reading.TotalPower, limit.PeakPower),
				Value:     m.reading.TotalPower,
				Threshold: limit.PeakPower,
				Timestamp: time.Now(),
			}
			m.addAlert(alert)
		}
	}
}

// checkBudgets 检查预算
func (m *Manager) checkBudgets() {
	for _, budget := range m.budgets {
		if !budget.Enabled {
			continue
		}

		if budget.UsedPercent > 90 {
			alert := PowerAlert{
				ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
				Type:      "over_budget",
				Message:   fmt.Sprintf("预算使用率超过90%%: %.1f%%", budget.UsedPercent),
				Value:     budget.UsedPercent,
				Threshold: 90,
				Timestamp: time.Now(),
			}
			m.addAlert(alert)
		}
	}
}

// autoApplyPolicy 自动应用策略
func (m *Manager) autoApplyPolicy() {
	for _, policy := range m.policies {
		if policy.Enabled && policy.AutoApply {
			if m.reading != nil && m.reading.TotalPower > policy.MaxPower {
				m.currentMode = policy.Mode
				log.Printf("[功耗管理] 自动应用策略: %s", policy.Name)
			}
		}
	}
}

// addAlert 添加告警
func (m *Manager) addAlert(alert PowerAlert) {
	m.alerts = append(m.alerts, alert)
	if len(m.alerts) > m.maxAlerts {
		m.alerts = m.alerts[len(m.alerts)-m.maxAlerts:]
	}
}

// Start 启动定时采集
func (m *Manager) Start(interval time.Duration) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopCh = make(chan struct{})
	m.mu.Unlock()

	go func() {
		// 立即采集一次
		m.collect()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.collect()
			case <-m.stopCh:
				return
			}
		}
	}()

	log.Printf("[功耗管理] 启动定时采集，间隔 %v", interval)
}

// Stop 停止定时采集
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.stopCh)
	log.Println("[功耗管理] 停止定时采集")
}

// EstimateCost 估算电费 (供外部调用)
func (m *Manager) EstimateCost(energyWh float64, pricePerKWh float64) float64 {
	return (energyWh / 1000) * pricePerKWh
}
