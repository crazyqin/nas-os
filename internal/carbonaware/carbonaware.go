// Package carbonaware 碳感知调度模块
// 支持电网碳强度 API 集成、任务碳足迹计算、碳感知任务调度
package carbonaware

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// CarbonIntensitySource 碳强度数据源.
type CarbonIntensitySource string

const (
	SourceWattTime        CarbonIntensitySource = "watttime"
	SourceElectricityMap  CarbonIntensitySource = "electricitymap"
	SourceCarbonInterface CarbonIntensitySource = "carboninterface"
	SourceCustom          CarbonIntensitySource = "custom"
)

// GridRegion 电网区域.
type GridRegion struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Country         string    `json:"country"`
	CarbonIntensity float64   `json:"carbon_intensity"` // gCO2/kWh
	GreenEnergyPct  float64   `json:"green_energy_pct"`
	LastUpdated     time.Time `json:"last_updated"`
}

// CarbonIntensityData 碳强度数据.
type CarbonIntensityData struct {
	Region    string                `json:"region"`
	Intensity float64               `json:"intensity"` // gCO2/kWh
	Forecast  []ForecastPoint       `json:"forecast"`
	Source    CarbonIntensitySource `json:"source"`
	Timestamp time.Time             `json:"timestamp"`
}

// ForecastPoint 预测点.
type ForecastPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Intensity float64   `json:"intensity"`
}

// TaskCarbonFootprint 任务碳足迹.
type TaskCarbonFootprint struct {
	TaskID      string        `json:"task_id"`
	TaskName    string        `json:"task_name"`
	EnergyKWh   float64       `json:"energy_kwh"`
	CarbonKg    float64       `json:"carbon_kg"`
	Duration    time.Duration `json:"duration"`
	Region      string        `json:"region"`
	ScheduledAt time.Time     `json:"scheduled_at"`
	CompletedAt time.Time     `json:"completed_at"`
}

// CarbonAwareTask 碳感知任务.
type CarbonAwareTask struct {
	ID              string               `json:"id"`
	Name            string               `json:"name"`
	Description     string               `json:"description"`
	Priority        int                  `json:"priority"` // 1-10
	EstimatedEnergy float64              `json:"estimated_energy_kwh"`
	MaxDelay        time.Duration        `json:"max_delay"`
	Deadline        time.Time            `json:"deadline"`
	Status          TaskStatus           `json:"status"`
	CarbonBudget    float64              `json:"carbon_budget_kg"`
	Region          string               `json:"region"`
	Metadata        map[string]string    `json:"metadata"`
	CreatedAt       time.Time            `json:"created_at"`
	ScheduledAt     *time.Time           `json:"scheduled_at,omitempty"`
	CompletedAt     *time.Time           `json:"completed_at,omitempty"`
	CarbonFootprint *TaskCarbonFootprint `json:"carbon_footprint,omitempty"`
}

// TaskStatus 任务状态.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusScheduled TaskStatus = "scheduled"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// SchedulingStrategy 调度策略.
type SchedulingStrategy string

const (
	StrategyLowCarbon   SchedulingStrategy = "low_carbon"
	StrategyGreenFirst  SchedulingStrategy = "green_first"
	StrategyBalanced    SchedulingStrategy = "balanced"
	StrategyCostOptimal SchedulingStrategy = "cost_optimal"
)

// CarbonAwareConfig 碳感知调度配置.
type CarbonAwareConfig struct {
	Enabled            bool                    `json:"enabled"`
	DefaultRegion      string                  `json:"default_region"`
	Strategy           SchedulingStrategy      `json:"strategy"`
	MaxCarbonIntensity float64                 `json:"max_carbon_intensity"` // gCO2/kWh
	GreenThreshold     float64                 `json:"green_threshold"`      // gCO2/kWh
	CheckInterval      int                     `json:"check_interval"`       // seconds
	Sources            []CarbonIntensitySource `json:"sources"`
	Regions            []GridRegion            `json:"regions"`
}

// CarbonReport 碳排放报告.
type CarbonReport struct {
	ID             string                `json:"id"`
	Period         string                `json:"period"`
	StartDate      time.Time             `json:"start_date"`
	EndDate        time.Time             `json:"end_date"`
	TotalTasks     int                   `json:"total_tasks"`
	TotalEnergyKWh float64               `json:"total_energy_kwh"`
	TotalCarbonKg  float64               `json:"total_carbon_kg"`
	AvgIntensity   float64               `json:"avg_intensity"`
	GreenTasksPct  float64               `json:"green_tasks_pct"`
	CarbonSaved    float64               `json:"carbon_saved_kg"`
	Tasks          []TaskCarbonFootprint `json:"tasks"`
	GeneratedAt    time.Time             `json:"generated_at"`
}

// Manager 碳感知调度管理器.
type Manager struct {
	config      *CarbonAwareConfig
	tasks       map[string]*CarbonAwareTask
	regions     map[string]*GridRegion
	intensities map[string]*CarbonIntensityData
	reports     []*CarbonReport
	mu          sync.RWMutex
	stopCh      chan struct{}
}

// NewManager 创建碳感知调度管理器.
func NewManager(config *CarbonAwareConfig) *Manager {
	m := &Manager{
		config:      config,
		tasks:       make(map[string]*CarbonAwareTask),
		regions:     make(map[string]*GridRegion),
		intensities: make(map[string]*CarbonIntensityData),
		stopCh:      make(chan struct{}),
	}

	// 初始化区域数据
	for _, region := range config.Regions {
		m.regions[region.ID] = &GridRegion{
			ID:              region.ID,
			Name:            region.Name,
			Country:         region.Country,
			CarbonIntensity: region.CarbonIntensity,
			GreenEnergyPct:  region.GreenEnergyPct,
			LastUpdated:     time.Now(),
		}
	}

	return m
}

// Start 启动碳感知调度.
func (m *Manager) Start() error {
	if !m.config.Enabled {
		return nil
	}

	go m.monitorCarbonIntensity()
	go m.scheduleTasks()

	return nil
}

// Stop 停止碳感知调度.
func (m *Manager) Stop() {
	close(m.stopCh)
}

// monitorCarbonIntensity 监控碳强度.
func (m *Manager) monitorCarbonIntensity() {
	ticker := time.NewTicker(time.Duration(m.config.CheckInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.updateCarbonIntensity()
		}
	}
}

// updateCarbonIntensity 更新碳强度数据.
func (m *Manager) updateCarbonIntensity() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, region := range m.regions {
		// 模拟碳强度变化
		intensity := m.fetchCarbonIntensity(region.ID)
		m.intensities[region.ID] = &CarbonIntensityData{
			Region:    region.ID,
			Intensity: intensity,
			Source:    SourceCustom,
			Timestamp: time.Now(),
		}
		region.CarbonIntensity = intensity
		region.LastUpdated = time.Now()
	}
}

// fetchCarbonIntensity 获取碳强度.
func (m *Manager) fetchCarbonIntensity(regionID string) float64 {
	// 模拟碳强度数据
	base := 400.0
	hour := time.Now().Hour()

	// 低谷期（凌晨）碳强度较低
	if hour >= 0 && hour <= 6 {
		base = 250.0
	} else if hour >= 22 || hour <= 23 {
		base = 350.0
	}

	return base + float64(len(regionID))*10
}

// scheduleTasks 调度任务.
func (m *Manager) scheduleTasks() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.schedulePendingTasks()
		}
	}
}

// schedulePendingTasks 调度待处理任务.
func (m *Manager) schedulePendingTasks() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, task := range m.tasks {
		if task.Status == TaskStatusPending {
			if m.shouldScheduleNow(task) {
				now := time.Now()
				task.ScheduledAt = &now
				task.Status = TaskStatusScheduled
			}
		}
	}
}

// shouldScheduleNow 判断是否应该立即调度.
func (m *Manager) shouldScheduleNow(task *CarbonAwareTask) bool {
	region := m.regions[task.Region]
	if region == nil {
		return true
	}

	switch m.config.Strategy {
	case StrategyLowCarbon:
		return region.CarbonIntensity <= m.config.GreenThreshold
	case StrategyGreenFirst:
		return region.CarbonIntensity <= m.config.MaxCarbonIntensity
	case StrategyBalanced:
		if task.Priority >= 8 {
			return true
		}
		return region.CarbonIntensity <= m.config.MaxCarbonIntensity
	case StrategyCostOptimal:
		return region.CarbonIntensity <= m.config.MaxCarbonIntensity
	default:
		return true
	}
}

// SubmitTask 提交碳感知任务.
func (m *Manager) SubmitTask(task *CarbonAwareTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task.ID == "" {
		task.ID = fmt.Sprintf("task_%d", time.Now().UnixNano())
	}

	if task.Region == "" {
		task.Region = m.config.DefaultRegion
	}

	task.Status = TaskStatusPending
	task.CreatedAt = time.Now()

	m.tasks[task.ID] = task
	return nil
}

// CompleteTask 完成任务.
func (m *Manager) CompleteTask(taskID string, energyKWh float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	now := time.Now()
	task.Status = TaskStatusCompleted
	task.CompletedAt = &now

	// 计算碳足迹
	region := m.regions[task.Region]
	intensity := 400.0
	if region != nil {
		intensity = region.CarbonIntensity
	}

	task.CarbonFootprint = &TaskCarbonFootprint{
		TaskID:      task.ID,
		TaskName:    task.Name,
		EnergyKWh:   energyKWh,
		CarbonKg:    energyKWh * intensity / 1000,
		Region:      task.Region,
		ScheduledAt: *task.ScheduledAt,
		CompletedAt: now,
	}

	if task.ScheduledAt != nil {
		task.CarbonFootprint.Duration = now.Sub(*task.ScheduledAt)
	}

	return nil
}

// GetTask 获取任务.
func (m *Manager) GetTask(taskID string) (*CarbonAwareTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	return task, nil
}

// ListTasks 列出任务.
func (m *Manager) ListTasks(status TaskStatus) []*CarbonAwareTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tasks []*CarbonAwareTask
	for _, task := range m.tasks {
		if status == "" || task.Status == status {
			tasks = append(tasks, task)
		}
	}

	return tasks
}

// GetCarbonIntensity 获取碳强度.
func (m *Manager) GetCarbonIntensity(regionID string) (*CarbonIntensityData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, ok := m.intensities[regionID]
	if !ok {
		return nil, fmt.Errorf("region not found: %s", regionID)
	}

	return data, nil
}

// GetGreenestRegion 获取最绿色区域.
func (m *Manager) GetGreenestRegion() *GridRegion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var greenest *GridRegion
	for _, region := range m.regions {
		if greenest == nil || region.CarbonIntensity < greenest.CarbonIntensity {
			greenest = region
		}
	}

	return greenest
}

// GenerateReport 生成碳排放报告.
func (m *Manager) GenerateReport(startDate, endDate time.Time) *CarbonReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &CarbonReport{
		ID:          fmt.Sprintf("report_%d", time.Now().UnixNano()),
		StartDate:   startDate,
		EndDate:     endDate,
		GeneratedAt: time.Now(),
	}

	var totalEnergy, totalCarbon float64
	greenTasks := 0

	for _, task := range m.tasks {
		if task.CompletedAt != nil &&
			task.CompletedAt.After(startDate) &&
			task.CompletedAt.Before(endDate) {

			report.TotalTasks++

			if task.CarbonFootprint != nil {
				totalEnergy += task.CarbonFootprint.EnergyKWh
				totalCarbon += task.CarbonFootprint.CarbonKg
				report.Tasks = append(report.Tasks, *task.CarbonFootprint)

				region := m.regions[task.Region]
				if region != nil && region.CarbonIntensity <= m.config.GreenThreshold {
					greenTasks++
				}
			}
		}
	}

	report.TotalEnergyKWh = totalEnergy
	report.TotalCarbonKg = totalCarbon

	if report.TotalTasks > 0 {
		report.AvgIntensity = totalCarbon / totalEnergy * 1000
		report.GreenTasksPct = float64(greenTasks) / float64(report.TotalTasks) * 100
	}

	// 计算碳减排（与基准对比）
	baselineCarbon := totalEnergy * 500 / 1000 // 假设基准 500 gCO2/kWh
	report.CarbonSaved = baselineCarbon - totalCarbon

	m.reports = append(m.reports, report)
	return report
}

// GetDashboard 获取仪表盘数据.
func (m *Manager) GetDashboard() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pendingTasks := 0
	runningTasks := 0
	completedTasks := 0

	for _, task := range m.tasks {
		switch task.Status {
		case TaskStatusPending:
			pendingTasks++
		case TaskStatusRunning:
			runningTasks++
		case TaskStatusCompleted:
			completedTasks++
		}
	}

	return map[string]interface{}{
		"total_tasks":     len(m.tasks),
		"pending_tasks":   pendingTasks,
		"running_tasks":   runningTasks,
		"completed_tasks": completedTasks,
		"regions":         len(m.regions),
		"strategy":        m.config.Strategy,
		"green_threshold": m.config.GreenThreshold,
	}
}

// MarshalJSON 序列化.
func (m *Manager) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return json.Marshal(struct {
		Config  *CarbonAwareConfig          `json:"config"`
		Tasks   map[string]*CarbonAwareTask `json:"tasks"`
		Regions map[string]*GridRegion      `json:"regions"`
		Reports int                         `json:"reports_count"`
	}{
		Config:  m.config,
		Tasks:   m.tasks,
		Regions: m.regions,
		Reports: len(m.reports),
	})
}
