package budgetmgr

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newTestBudget(id, name, dept string, amount float64) *Budget {
	return &Budget{
		ID:             id,
		Name:           name,
		Department:     dept,
		TotalAmount:    amount,
		AlertThreshold: 0.8,
		StartDate:      time.Now(),
		EndDate:        time.Now().AddDate(0, 1, 0),
	}
}

// ========== 预算 CRUD ==========

func TestCreateBudget(t *testing.T) {
	m := NewManager()
	b := newTestBudget("b1", "测试预算", "技术部", 10000)
	err := m.CreateBudget(b)
	assert.NoError(t, err)
	assert.True(t, b.IsActive)
	assert.Equal(t, "CNY", b.Currency)
	assert.Equal(t, DegradationNone, b.DegradationStrategy)
}

func TestCreateBudget_InvalidParams(t *testing.T) {
	m := NewManager()

	// 缺少ID
	err := m.CreateBudget(&Budget{Name: "test", Department: "dept", TotalAmount: 100})
	assert.ErrorIs(t, err, ErrInvalidParams)

	// 缺少Name
	err = m.CreateBudget(&Budget{ID: "b1", Department: "dept", TotalAmount: 100})
	assert.ErrorIs(t, err, ErrInvalidParams)

	// 缺少Department
	err = m.CreateBudget(&Budget{ID: "b1", Name: "test", TotalAmount: 100})
	assert.ErrorIs(t, err, ErrInvalidParams)

	// 金额为0
	err = m.CreateBudget(&Budget{ID: "b1", Name: "test", Department: "dept", TotalAmount: 0})
	assert.ErrorIs(t, err, ErrInvalidParams)
}

func TestGetBudget(t *testing.T) {
	m := NewManager()
	b := newTestBudget("b1", "测试预算", "技术部", 10000)
	_ = m.CreateBudget(b)

	got, err := m.GetBudget("b1")
	assert.NoError(t, err)
	assert.Equal(t, "b1", got.ID)
	assert.Equal(t, "测试预算", got.Name)
}

func TestGetBudget_NotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetBudget("nonexistent")
	assert.ErrorIs(t, err, ErrBudgetNotFound)
}

func TestUpdateBudget(t *testing.T) {
	m := NewManager()
	_ = m.CreateBudget(newTestBudget("b1", "旧名称", "技术部", 10000))

	updated, err := m.UpdateBudget("b1", &Budget{
		Name:        "新名称",
		TotalAmount: 20000,
	})
	assert.NoError(t, err)
	assert.Equal(t, "新名称", updated.Name)
	assert.Equal(t, 20000.0, updated.TotalAmount)
}

func TestUpdateBudget_NotFound(t *testing.T) {
	m := NewManager()
	_, err := m.UpdateBudget("nonexistent", &Budget{Name: "x"})
	assert.ErrorIs(t, err, ErrBudgetNotFound)
}

func TestDeleteBudget(t *testing.T) {
	m := NewManager()
	_ = m.CreateBudget(newTestBudget("b1", "测试预算", "技术部", 10000))

	err := m.DeleteBudget("b1")
	assert.NoError(t, err)

	_, err = m.GetBudget("b1")
	assert.ErrorIs(t, err, ErrBudgetNotFound)
}

func TestDeleteBudget_NotFound(t *testing.T) {
	m := NewManager()
	err := m.DeleteBudget("nonexistent")
	assert.ErrorIs(t, err, ErrBudgetNotFound)
}

func TestListBudgets(t *testing.T) {
	m := NewManager()
	_ = m.CreateBudget(newTestBudget("b1", "预算1", "技术部", 10000))
	_ = m.CreateBudget(newTestBudget("b2", "预算2", "市场部", 20000))

	list := m.ListBudgets()
	assert.Len(t, list, 2)
}

func TestListBudgetsByDepartment(t *testing.T) {
	m := NewManager()
	_ = m.CreateBudget(newTestBudget("b1", "预算1", "技术部", 10000))
	_ = m.CreateBudget(newTestBudget("b2", "预算2", "技术部", 20000))
	_ = m.CreateBudget(newTestBudget("b3", "预算3", "市场部", 30000))

	techBudgets := m.ListBudgetsByDepartment("技术部")
	assert.Len(t, techBudgets, 2)
}

// ========== 使用率和告警 ==========

func TestRecordUsage(t *testing.T) {
	m := NewManager()
	_ = m.CreateBudget(newTestBudget("b1", "测试预算", "技术部", 10000))

	err := m.RecordUsage("b1", 3000)
	assert.NoError(t, err)

	b, _ := m.GetBudget("b1")
	assert.Equal(t, 3000.0, b.UsedAmount)

	_ = m.RecordUsage("b1", 2000)
	b, _ = m.GetBudget("b1")
	assert.Equal(t, 5000.0, b.UsedAmount)
}

func TestRecordUsage_NotFound(t *testing.T) {
	m := NewManager()
	err := m.RecordUsage("nonexistent", 1000)
	assert.ErrorIs(t, err, ErrBudgetNotFound)
}

func TestGetUtilization(t *testing.T) {
	m := NewManager()
	_ = m.CreateBudget(newTestBudget("b1", "测试预算", "技术部", 10000))
	_ = m.RecordUsage("b1", 5000)

	rate, err := m.GetUtilization("b1")
	assert.NoError(t, err)
	assert.InDelta(t, 0.5, rate, 0.001)
}

func TestCheckOverBudget(t *testing.T) {
	m := NewManager()
	// 阈值0.8，使用85%应触发告警
	_ = m.CreateBudget(newTestBudget("b1", "接近超支", "技术部", 10000))
	_ = m.RecordUsage("b1", 8500)

	// 未超支的预算
	_ = m.CreateBudget(newTestBudget("b2", "正常预算", "市场部", 10000))
	_ = m.RecordUsage("b2", 3000)

	alerts := m.CheckOverBudget()
	assert.Len(t, alerts, 1)
	assert.Equal(t, "b1", alerts[0].ID)
}

func TestGetDegradationActions(t *testing.T) {
	m := NewManager()
	// 创建超支预算，策略为限流
	b := newTestBudget("b1", "测试", "技术部", 10000)
	b.DegradationStrategy = DegradationThrottle
	_ = m.CreateBudget(b)
	_ = m.RecordUsage("b1", 10000) // 恰好用完

	actions := m.GetDegradationActions()
	assert.Len(t, actions, 1)
	assert.Equal(t, DegradationThrottle, actions["b1"])
}

func TestGetUtilizationReport(t *testing.T) {
	m := NewManager()
	_ = m.CreateBudget(newTestBudget("b1", "测试预算", "技术部", 10000))
	_ = m.RecordUsage("b1", 9000)

	report, err := m.GetUtilizationReport("b1")
	assert.NoError(t, err)
	assert.Equal(t, "b1", report.BudgetID)
	assert.Equal(t, 10000.0, report.TotalAmount)
	assert.Equal(t, 9000.0, report.UsedAmount)
	assert.Equal(t, 1000.0, report.RemainingAmount)
	assert.Equal(t, "接近超支", report.Status)
	assert.False(t, report.IsOverBudget)
}

func TestGetUtilizationReport_OverBudget(t *testing.T) {
	m := NewManager()
	_ = m.CreateBudget(newTestBudget("b1", "测试预算", "技术部", 10000))
	_ = m.RecordUsage("b1", 11000)

	report, err := m.GetUtilizationReport("b1")
	assert.NoError(t, err)
	assert.True(t, report.IsOverBudget)
	assert.Equal(t, "超支", report.Status)
}

// ========== 审批流程 ==========

func TestCreateRequest(t *testing.T) {
	m := NewManager()
	req := &BudgetRequest{
		ID:         "req1",
		BudgetID:   "b1",
		Department: "技术部",
		Amount:     5000,
		Reason:     "服务器扩容",
	}
	err := m.CreateRequest(req)
	assert.NoError(t, err)
	assert.Equal(t, StatusPending, req.Status)
}

func TestCreateRequest_InvalidParams(t *testing.T) {
	m := NewManager()

	// 缺少ID
	err := m.CreateRequest(&BudgetRequest{Department: "dept", Amount: 100})
	assert.ErrorIs(t, err, ErrInvalidParams)

	// 缺少Department
	err = m.CreateRequest(&BudgetRequest{ID: "req1", Amount: 100})
	assert.ErrorIs(t, err, ErrInvalidParams)

	// 金额为0
	err = m.CreateRequest(&BudgetRequest{ID: "req1", Department: "dept", Amount: 0})
	assert.ErrorIs(t, err, ErrInvalidParams)
}

func TestApproveRequest(t *testing.T) {
	m := NewManager()
	_ = m.CreateRequest(&BudgetRequest{ID: "req1", Department: "技术部", Amount: 5000})

	err := m.ApproveRequest("req1", "admin", "同意")
	assert.NoError(t, err)

	req, _ := m.GetRequest("req1")
	assert.Equal(t, StatusApproved, req.Status)
	assert.Equal(t, "admin", req.Approver)
}

func TestApproveRequest_AlreadyProcessed(t *testing.T) {
	m := NewManager()
	_ = m.CreateRequest(&BudgetRequest{ID: "req1", Department: "技术部", Amount: 5000})
	_ = m.ApproveRequest("req1", "admin", "同意")

	err := m.ApproveRequest("req1", "admin2", "再次审批")
	assert.ErrorIs(t, err, ErrAlreadyProcessed)
}

func TestRejectRequest(t *testing.T) {
	m := NewManager()
	_ = m.CreateRequest(&BudgetRequest{ID: "req1", Department: "技术部", Amount: 5000})

	err := m.RejectRequest("req1", "admin", "预算不足")
	assert.NoError(t, err)

	req, _ := m.GetRequest("req1")
	assert.Equal(t, StatusRejected, req.Status)
}

func TestAllocateRequest(t *testing.T) {
	m := NewManager()
	_ = m.CreateBudget(newTestBudget("b1", "测试预算", "技术部", 10000))
	_ = m.CreateRequest(&BudgetRequest{ID: "req1", BudgetID: "b1", Department: "技术部", Amount: 5000})
	_ = m.ApproveRequest("req1", "admin", "同意")

	err := m.AllocateRequest("req1")
	assert.NoError(t, err)

	b, _ := m.GetBudget("b1")
	assert.Equal(t, 15000.0, b.TotalAmount) // 10000 + 5000
}

func TestAllocateRequest_InvalidTransition(t *testing.T) {
	m := NewManager()
	_ = m.CreateRequest(&BudgetRequest{ID: "req1", Department: "技术部", Amount: 5000})

	// 未批准直接分配
	err := m.AllocateRequest("req1")
	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestListPendingRequests(t *testing.T) {
	m := NewManager()
	_ = m.CreateRequest(&BudgetRequest{ID: "req1", Department: "技术部", Amount: 5000})
	_ = m.CreateRequest(&BudgetRequest{ID: "req2", Department: "市场部", Amount: 3000})
	_ = m.ApproveRequest("req1", "admin", "同意")

	pending := m.ListPendingRequests()
	assert.Len(t, pending, 1)
	assert.Equal(t, "req2", pending[0].ID)
}

// ========== 模板 ==========

func TestCreateTemplate(t *testing.T) {
	m := NewManager()
	tmpl := &BudgetTemplate{
		ID:       "t1",
		Name:     "月度模板",
		Period:   PeriodMonthly,
		DefaultAmount: 50000,
	}
	err := m.CreateTemplate(tmpl)
	assert.NoError(t, err)
	assert.Equal(t, 0.8, tmpl.AlertThreshold)
}

func TestCreateTemplate_InvalidParams(t *testing.T) {
	m := NewManager()

	err := m.CreateTemplate(&BudgetTemplate{Name: "test"})
	assert.ErrorIs(t, err, ErrInvalidParams)

	err = m.CreateTemplate(&BudgetTemplate{ID: "t1"})
	assert.ErrorIs(t, err, ErrInvalidParams)
}

func TestCreateBudgetFromTemplate(t *testing.T) {
	m := NewManager()
	_ = m.CreateTemplate(&BudgetTemplate{
		ID:              "t1",
		Name:            "月度模板",
		Period:          PeriodMonthly,
		DefaultAmount:   50000,
		AlertThreshold:  0.9,
	})

	b, err := m.CreateBudgetFromTemplate("t1", "b1", "6月预算", "技术部", time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local))
	assert.NoError(t, err)
	assert.Equal(t, 50000.0, b.TotalAmount)
	assert.Equal(t, 0.9, b.AlertThreshold)
	assert.Equal(t, PeriodMonthly, b.Period)
}

func TestCreateBudgetFromTemplate_NotFound(t *testing.T) {
	m := NewManager()
	_, err := m.CreateBudgetFromTemplate("nonexistent", "b1", "测试", "技术部", time.Now())
	assert.ErrorIs(t, err, ErrTemplateNotFound)
}

func TestCompareBudgets(t *testing.T) {
	m := NewManager()
	_ = m.CreateBudget(&Budget{
		ID: "b1", Name: "Q1", Department: "技术部",
		TotalAmount: 10000, UsedAmount: 6000,
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local),
		EndDate:   time.Date(2026, 3, 31, 0, 0, 0, 0, time.Local),
	})
	_ = m.CreateBudget(&Budget{
		ID: "b2", Name: "Q2", Department: "技术部",
		TotalAmount: 10000, UsedAmount: 8000,
		StartDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.Local),
		EndDate:   time.Date(2026, 6, 30, 0, 0, 0, 0, time.Local),
	})

	comp := m.CompareBudgets("技术部")
	assert.Equal(t, "技术部", comp.Department)
	assert.Len(t, comp.Periods, 2)
	assert.Equal(t, "increasing", comp.Trend) // 0.6 -> 0.8，增长趋势
}
