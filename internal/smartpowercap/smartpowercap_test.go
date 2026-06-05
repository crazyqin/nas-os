package smartpowercap

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}
	if m.currentMode != PowerModeBalanced {
		t.Errorf("expected balanced mode, got %s", m.currentMode)
	}
	if m.currentState != PowerStateNormal {
		t.Errorf("expected normal state, got %s", m.currentState)
	}
	if m.maxAlerts != 100 {
		t.Errorf("expected maxAlerts 100, got %d", m.maxAlerts)
	}
}

func TestGetCurrentReading(t *testing.T) {
	m := NewManager()

	// 初始状态
	reading := m.GetCurrentReading()
	if reading == nil {
		t.Fatal("expected reading")
	}

	// 启动后
	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(200 * time.Millisecond)

	reading = m.GetCurrentReading()
	if reading.TotalPower == 0 {
		t.Error("expected non-zero total power")
	}
}

func TestGetState(t *testing.T) {
	m := NewManager()

	if m.GetState() != PowerStateNormal {
		t.Errorf("expected normal state, got %s", m.GetState())
	}
}

func TestGetMode(t *testing.T) {
	m := NewManager()

	if m.GetMode() != PowerModeBalanced {
		t.Errorf("expected balanced mode, got %s", m.GetMode())
	}
}

func TestSetMode(t *testing.T) {
	m := NewManager()

	// 有效模式
	err := m.SetMode(PowerModeEco)
	if err != nil {
		t.Fatalf("set mode failed: %v", err)
	}
	if m.GetMode() != PowerModeEco {
		t.Errorf("expected eco mode, got %s", m.GetMode())
	}

	// 无效模式
	err = m.SetMode(PowerMode("invalid"))
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestAddBudget(t *testing.T) {
	m := NewManager()

	budget := &PowerBudget{
		ID:       "test-budget",
		Name:     "测试预算",
		MaxPower: 1000,
		Enabled:  true,
	}

	err := m.AddBudget(budget)
	if err != nil {
		t.Fatalf("add budget failed: %v", err)
	}

	// 重复添加
	err = m.AddBudget(budget)
	if err == nil {
		t.Error("expected error for duplicate budget")
	}

	// 空ID
	err = m.AddBudget(&PowerBudget{})
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestUpdateBudget(t *testing.T) {
	m := NewManager()

	m.AddBudget(&PowerBudget{ID: "b1", Name: "B1", MaxPower: 1000, Enabled: true})

	err := m.UpdateBudget("b1", 2000)
	if err != nil {
		t.Fatalf("update budget failed: %v", err)
	}

	budget, _ := m.GetBudget("b1")
	if budget.MaxPower != 2000 {
		t.Errorf("expected 2000, got %.1f", budget.MaxPower)
	}

	// 不存在的
	err = m.UpdateBudget("nonexistent", 100)
	if err == nil {
		t.Error("expected error for nonexistent budget")
	}
}

func TestGetBudget(t *testing.T) {
	m := NewManager()

	m.AddBudget(&PowerBudget{ID: "b1", Name: "B1", MaxPower: 1000, Enabled: true})

	budget, err := m.GetBudget("b1")
	if err != nil {
		t.Fatalf("get budget failed: %v", err)
	}
	if budget.Name != "B1" {
		t.Errorf("expected B1, got %s", budget.Name)
	}

	_, err = m.GetBudget("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent budget")
	}
}

func TestListBudgets(t *testing.T) {
	m := NewManager()

	// 默认有一个
	budgets := m.ListBudgets()
	if len(budgets) < 1 {
		t.Errorf("expected at least 1 budget, got %d", len(budgets))
	}

	m.AddBudget(&PowerBudget{ID: "b2", Name: "B2", MaxPower: 2000, Enabled: true})
	budgets = m.ListBudgets()
	if len(budgets) < 2 {
		t.Errorf("expected at least 2 budgets, got %d", len(budgets))
	}
}

func TestRemoveBudget(t *testing.T) {
	m := NewManager()

	m.AddBudget(&PowerBudget{ID: "b1", Name: "B1", MaxPower: 1000, Enabled: true})

	err := m.RemoveBudget("b1")
	if err != nil {
		t.Fatalf("remove budget failed: %v", err)
	}

	err = m.RemoveBudget("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent budget")
	}
}

func TestAddLimit(t *testing.T) {
	m := NewManager()

	limit := &PowerLimit{
		ID:        "test-limit",
		Name:      "测试限制",
		PeakPower: 300,
		Sustained: 250,
		Duration:  10,
		Enabled:   true,
	}

	err := m.AddLimit(limit)
	if err != nil {
		t.Fatalf("add limit failed: %v", err)
	}

	// 重复添加
	err = m.AddLimit(limit)
	if err == nil {
		t.Error("expected error for duplicate limit")
	}

	// 空ID
	err = m.AddLimit(&PowerLimit{})
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestUpdateLimit(t *testing.T) {
	m := NewManager()

	m.AddLimit(&PowerLimit{ID: "l1", Name: "L1", PeakPower: 300, Sustained: 250, Duration: 10, Enabled: true})

	err := m.UpdateLimit("l1", 400, 350)
	if err != nil {
		t.Fatalf("update limit failed: %v", err)
	}

	limit, _ := m.GetLimit("l1")
	if limit.PeakPower != 400 {
		t.Errorf("expected 400, got %.1f", limit.PeakPower)
	}

	// 不存在的
	err = m.UpdateLimit("nonexistent", 100, 80)
	if err == nil {
		t.Error("expected error for nonexistent limit")
	}
}

func TestGetLimit(t *testing.T) {
	m := NewManager()

	m.AddLimit(&PowerLimit{ID: "l1", Name: "L1", PeakPower: 300, Sustained: 250, Duration: 10, Enabled: true})

	limit, err := m.GetLimit("l1")
	if err != nil {
		t.Fatalf("get limit failed: %v", err)
	}
	if limit.Name != "L1" {
		t.Errorf("expected L1, got %s", limit.Name)
	}

	_, err = m.GetLimit("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent limit")
	}
}

func TestListLimits(t *testing.T) {
	m := NewManager()

	// 默认有一个
	limits := m.ListLimits()
	if len(limits) < 1 {
		t.Errorf("expected at least 1 limit, got %d", len(limits))
	}
}

func TestAddPolicy(t *testing.T) {
	m := NewManager()

	policy := &PowerPolicy{
		ID:          "test-policy",
		Name:        "测试策略",
		Mode:        PowerModeEco,
		MaxPower:    150,
		CPUThrottle: 70,
		GPUThrottle: 60,
		Enabled:     true,
		AutoApply:   true,
	}

	err := m.AddPolicy(policy)
	if err != nil {
		t.Fatalf("add policy failed: %v", err)
	}

	// 重复添加
	err = m.AddPolicy(policy)
	if err == nil {
		t.Error("expected error for duplicate policy")
	}

	// 空ID
	err = m.AddPolicy(&PowerPolicy{})
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestUpdatePolicy(t *testing.T) {
	m := NewManager()

	m.AddPolicy(&PowerPolicy{ID: "p1", Name: "P1", Mode: PowerModeEco, MaxPower: 150, Enabled: true})

	policy := &PowerPolicy{
		Name:     "Updated P1",
		Mode:     PowerModeBalanced,
		MaxPower: 200,
		Enabled:  true,
	}

	err := m.UpdatePolicy("p1", policy)
	if err != nil {
		t.Fatalf("update policy failed: %v", err)
	}

	got, _ := m.GetPolicy("p1")
	if got.MaxPower != 200 {
		t.Errorf("expected 200, got %.1f", got.MaxPower)
	}

	// 不存在的
	err = m.UpdatePolicy("nonexistent", policy)
	if err == nil {
		t.Error("expected error for nonexistent policy")
	}
}

func TestGetPolicy(t *testing.T) {
	m := NewManager()

	m.AddPolicy(&PowerPolicy{ID: "p1", Name: "P1", Mode: PowerModeEco, MaxPower: 150, Enabled: true})

	policy, err := m.GetPolicy("p1")
	if err != nil {
		t.Fatalf("get policy failed: %v", err)
	}
	if policy.Name != "P1" {
		t.Errorf("expected P1, got %s", policy.Name)
	}

	_, err = m.GetPolicy("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent policy")
	}
}

func TestListPolicies(t *testing.T) {
	m := NewManager()

	// 默认有两个
	policies := m.ListPolicies()
	if len(policies) < 2 {
		t.Errorf("expected at least 2 policies, got %d", len(policies))
	}
}

func TestApplyPolicy(t *testing.T) {
	m := NewManager()

	m.AddPolicy(&PowerPolicy{ID: "p1", Name: "P1", Mode: PowerModeEco, MaxPower: 150, Enabled: true})

	err := m.ApplyPolicy("p1")
	if err != nil {
		t.Fatalf("apply policy failed: %v", err)
	}

	if m.GetMode() != PowerModeEco {
		t.Errorf("expected eco mode, got %s", m.GetMode())
	}

	// 不存在的
	err = m.ApplyPolicy("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent policy")
	}
}

func TestGetReport(t *testing.T) {
	m := NewManager()

	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(500 * time.Millisecond)

	report := m.GetReport("hourly")
	if report == nil {
		t.Fatal("expected report")
	}
	if report.Period != "hourly" {
		t.Errorf("expected hourly, got %s", report.Period)
	}
	if report.AvgPower == 0 {
		t.Error("expected non-zero avg power")
	}
}

func TestGetTrends(t *testing.T) {
	m := NewManager()

	trends := m.GetTrends(1 * time.Hour)
	if trends == nil {
		t.Error("expected empty trends, got nil")
	}
}

func TestGetAlerts(t *testing.T) {
	m := NewManager()

	alerts := m.GetAlerts()
	if alerts == nil {
		t.Error("expected empty alerts, got nil")
	}
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}
}

func TestClearAlerts(t *testing.T) {
	m := NewManager()

	m.ClearAlerts()
	alerts := m.GetAlerts()
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}
}

func TestStartStop(t *testing.T) {
	m := NewManager()

	m.Start(100 * time.Millisecond)
	time.Sleep(200 * time.Millisecond)

	reading := m.GetCurrentReading()
	if reading == nil {
		t.Fatal("expected reading")
	}

	m.Stop()

	// 重复停止不应 panic
	m.Stop()
}

func TestEstimateCost(t *testing.T) {
	m := NewManager()

	// 1000Wh = 1kWh, 价格0.5元/kWh
	cost := m.EstimateCost(1000, 0.5)
	if cost != 0.5 {
		t.Errorf("expected 0.5, got %.2f", cost)
	}

	// 2000Wh = 2kWh, 价格0.8元/kWh
	cost = m.EstimateCost(2000, 0.8)
	if cost != 1.6 {
		t.Errorf("expected 1.6, got %.2f", cost)
	}
}

func TestUpdateReading(t *testing.T) {
	m := NewManager()

	reading := &PowerReading{
		Timestamp:  time.Now(),
		TotalPower: 200,
		CPUPower:   100,
		GPUPower:   60,
		DrivePower: 30,
		FanPower:   5,
		OtherPower: 5,
	}

	m.UpdateReading(reading)

	got := m.GetCurrentReading()
	if got.TotalPower != 200 {
		t.Errorf("expected 200, got %.1f", got.TotalPower)
	}
}

func TestSetThresholds(t *testing.T) {
	m := NewManager()

	// 正常状态
	m.SetThresholds(200, 300)
	if m.GetState() != PowerStateNormal {
		t.Errorf("expected normal state, got %s", m.GetState())
	}

	// 设置高功耗
	m.UpdateReading(&PowerReading{TotalPower: 250})
	m.SetThresholds(200, 300)
	if m.GetState() != PowerStateWarning {
		t.Errorf("expected warning state, got %s", m.GetState())
	}

	// 严重状态
	m.UpdateReading(&PowerReading{TotalPower: 350})
	m.SetThresholds(200, 300)
	if m.GetState() != PowerStateCritical {
		t.Errorf("expected critical state, got %s", m.GetState())
	}
}
