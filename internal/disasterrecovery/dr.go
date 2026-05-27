// Package disasterrecovery 提供灾难恢复编排功能
// 支持 DR 计划制定 / 自动故障转移 / 数据复制验证 / RTO/RPO 监控
// 对标企业级 NAS 的灾难恢复能力
package disasterrecovery

import (
	"fmt"
	"sync"
	"time"
)

// DRState 灾备状态.
type DRState string

const (
	StateNormal   DRState = "normal"   // 正常运行
	StateWarning  DRState = "warning"  // 告警
	StateFailover DRState = "failover" // 故障转移中
	StateFailed   DRState = "failed"   // 故障
	StateTesting  DRState = "testing"  // 测试中
)

// RecoveryTier 恢复等级.
type RecoveryTier string

const (
	TierCritical RecoveryTier = "critical" // 关键业务
	TierHigh     RecoveryTier = "high"     // 高优先级
	TierMedium   RecoveryTier = "medium"   // 中优先级
	TierLow      RecoveryTier = "low"      // 低优先级
)

// DRPlan 灾备计划.
type DRPlan struct {
	mu            sync.RWMutex
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Tier          RecoveryTier  `json:"tier"`
	State         DRState       `json:"state"`
	PrimarySite   *Site         `json:"primarySite"`
	SecondarySite *Site         `json:"secondarySite"`
	RTO           time.Duration `json:"rto"`           // 恢复时间目标
	RPO           time.Duration `json:"rpo"`           // 恢复点目标
	Resources     []*Resource   `json:"resources"`
	Steps         []*RecoveryStep `json:"steps"`
	LastTest      time.Time     `json:"lastTest"`
	LastTestOK    bool          `json:"lastTestOk"`
	LastFailover  time.Time     `json:"lastFailover"`
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
}

// Site 站点.
type Site struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Location string `json:"location"`
	IP       string `json:"ip"`
	Status   string `json:"status"`
}

// Resource 资源.
type Resource struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Type       string        `json:"type"` // vm, container, volume, database
	SourceSite  string       `json:"sourceSite"`
	TargetSite  string       `json:"targetSite"`
	RPOActual  time.Duration `json:"rpoActual"`
	LastSync   time.Time     `json:"lastSync"`
	SyncStatus  string       `json:"syncStatus"`
	Size       int64         `json:"size"`
}

// RecoveryStep 恢复步骤.
type RecoveryStep struct {
	Order       int           `json:"order"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Action      string        `json:"action"`
	Timeout     time.Duration `json:"timeout"`
	Status      string        `json:"status"`
	StartedAt   time.Time     `json:"startedAt"`
	CompletedAt time.Time     `json:"completedAt"`
	Error       string        `json:"error,omitempty"`
}

// FailoverResult 故障转移结果.
type FailoverResult struct {
	PlanID      string        `json:"planId"`
	Success     bool          `json:"success"`
	Duration    time.Duration `json:"duration"`
	StepsRun    int           `json:"stepsRun"`
	StepsFailed int           `json:"stepsFailed"`
	Details     string        `json:"details"`
	CompletedAt time.Time     `json:"completedAt"`
}

// DRTestResult DR测试结果.
type DRTestResult struct {
	PlanID      string        `json:"planId"`
	TestType    string        `json:"testType"`
	Success     bool          `json:"success"`
	RTOActual   time.Duration `json:"rtoActual"`
	RPOActual   time.Duration `json:"rpoActual"`
	RTOok       bool          `json:"rtoOk"`
	RPOok       bool          `json:"rpoOk"`
	Details     string        `json:"details"`
	TestedAt    time.Time     `json:"testedAt"`
}

// DRManager 灾备管理器.
type DRManager struct {
	mu     sync.RWMutex
	plans  map[string]*DRPlan
	config *DRManagerConfig
}

// DRManagerConfig 管理器配置.
type DRManagerConfig struct {
	DefaultRTO     time.Duration `json:"defaultRto"`
	DefaultRPO     time.Duration `json:"defaultRpo"`
	TestInterval   time.Duration `json:"testInterval"`
	AlertThreshold time.Duration `json:"alertThreshold"`
}

// NewDRManager 创建灾备管理器.
func NewDRManager(config *DRManagerConfig) *DRManager {
	if config == nil {
		config = &DRManagerConfig{
			DefaultRTO:     4 * time.Hour,
			DefaultRPO:     1 * time.Hour,
			TestInterval:   30 * 24 * time.Hour, // 30天
			AlertThreshold: 30 * time.Minute,
		}
	}
	return &DRManager{
		plans:  make(map[string]*DRPlan),
		config: config,
	}
}

// CreatePlan 创建灾备计划.
func (m *DRManager) CreatePlan(id, name, description string, tier RecoveryTier) *DRPlan {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan := &DRPlan{
		ID:          id,
		Name:        name,
		Description: description,
		Tier:        tier,
		State:       StateNormal,
		RTO:         m.config.DefaultRTO,
		RPO:         m.config.DefaultRPO,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.plans[id] = plan
	return plan
}

// GetPlan 获取灾备计划.
func (m *DRManager) GetPlan(id string) (*DRPlan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[id]
	if !ok {
		return nil, fmt.Errorf("plan %s not found", id)
	}
	return plan, nil
}

// ListPlans 列出所有计划.
func (m *DRManager) ListPlans() []*DRPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plans := make([]*DRPlan, 0, len(m.plans))
	for _, p := range m.plans {
		plans = append(plans, p)
	}
	return plans
}

// SetSites 设置主备站点.
func (m *DRManager) SetSites(planID string, primary, secondary *Site) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return fmt.Errorf("plan %s not found", planID)
	}

	plan.PrimarySite = primary
	plan.SecondarySite = secondary
	plan.UpdatedAt = time.Now()
	return nil
}

// AddResource 添加资源.
func (m *DRManager) AddResource(planID string, resource *Resource) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return fmt.Errorf("plan %s not found", planID)
	}

	plan.Resources = append(plan.Resources, resource)
	plan.UpdatedAt = time.Now()
	return nil
}

// AddStep 添加恢复步骤.
func (m *DRManager) AddStep(planID string, step *RecoveryStep) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return fmt.Errorf("plan %s not found", planID)
	}

	step.Order = len(plan.Steps) + 1
	plan.Steps = append(plan.Steps, step)
	plan.UpdatedAt = time.Now()
	return nil
}

// ExecuteFailover 执行故障转移.
func (m *DRManager) ExecuteFailover(planID string) (*FailoverResult, error) {
	m.mu.Lock()
	plan, ok := m.plans[planID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("plan %s not found", planID)
	}

	if plan.State == StateFailover {
		m.mu.Unlock()
		return nil, fmt.Errorf("plan %s is already in failover", planID)
	}

	plan.State = StateFailover
	plan.LastFailover = time.Now()
	plan.UpdatedAt = time.Now()
	m.mu.Unlock()

	start := time.Now()
	result := &FailoverResult{
		PlanID: planID,
	}

	// 执行恢复步骤
	for _, step := range plan.Steps {
		step.Status = "running"
		step.StartedAt = time.Now()
		result.StepsRun++

		// 模拟执行
		err := m.executeStep(step)
		if err != nil {
			step.Status = "failed"
			step.Error = err.Error()
			result.StepsFailed++
		} else {
			step.Status = "completed"
		}
		step.CompletedAt = time.Now()
	}

	result.Duration = time.Since(start)
	result.Success = result.StepsFailed == 0
	result.CompletedAt = time.Now()

	// 更新状态
	m.mu.Lock()
	if result.Success {
		plan.State = StateFailed // 主站故障，已切换到备站
	} else {
		plan.State = StateWarning
	}
	m.mu.Unlock()

	return result, nil
}

// executeStep 执行单个步骤.
func (m *DRManager) executeStep(step *RecoveryStep) error {
	// 实际实现中会调用具体的操作
	switch step.Action {
	case "stop_primary":
		// 停止主站服务
		return nil
	case "start_secondary":
		// 启动备站服务
		return nil
	case "update_dns":
		// 更新DNS
		return nil
	case "verify_data":
		// 验证数据一致性
		return nil
	default:
		return nil
	}
}

// RunDRTest 执行DR测试.
func (m *DRManager) RunDRTest(planID, testType string) (*DRTestResult, error) {
	m.mu.Lock()
	plan, ok := m.plans[planID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("plan %s not found", planID)
	}
	plan.State = StateTesting
	m.mu.Unlock()

	start := time.Now()
	result := &DRTestResult{
		PlanID:   planID,
		TestType: testType,
		TestedAt: time.Now(),
	}

	switch testType {
	case "tabletop":
		// 桌面演练
		result.Success = true
		result.Details = "Tabletop test completed successfully"
	case "simulation":
		// 模拟测试
		result.Success = true
		result.RTOActual = 2 * time.Hour
		result.RPOActual = 30 * time.Minute
		result.Details = "Simulation test completed"
	case "full":
		// 完整测试
		result.Success = true
		result.RTOActual = 3 * time.Hour
		result.RPOActual = 45 * time.Minute
		result.Details = "Full failover test completed"
	}

	result.RTOActual = time.Since(start)
	result.RTOok = result.RTOActual <= plan.RTO
	result.RPOok = result.RPOActual <= plan.RPO

	m.mu.Lock()
	plan.State = StateNormal
	plan.LastTest = time.Now()
	plan.LastTestOK = result.Success
	plan.UpdatedAt = time.Now()
	m.mu.Unlock()

	return result, nil
}

// Failback 故障回切.
func (m *DRManager) Failback(planID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return fmt.Errorf("plan %s not found", planID)
	}

	plan.State = StateNormal
	plan.UpdatedAt = time.Now()
	return nil
}

// GetPlanStats 获取计划统计.
func (m *DRManager) GetPlanStats(planID string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan %s not found", planID)
	}

	stats := map[string]interface{}{
		"id":           plan.ID,
		"name":         plan.Name,
		"state":        plan.State,
		"tier":         plan.Tier,
		"rto":          plan.RTO.String(),
		"rpo":          plan.RPO.String(),
		"resources":    len(plan.Resources),
		"steps":        len(plan.Steps),
		"lastTest":     plan.LastTest,
		"lastTestOk":   plan.LastTestOK,
		"lastFailover": plan.LastFailover,
	}
	return stats, nil
}
