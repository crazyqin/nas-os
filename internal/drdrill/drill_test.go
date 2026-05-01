package drdrill

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ─────────────────────── Mock ───────────────────────

type mockSnapshotter struct {
	createErr    error
	restoreErr   error
	snapshotID   string
	restored     bool
}

func (m *mockSnapshotter) CreateSnapshot(_ context.Context, _ string) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	if m.snapshotID == "" {
		m.snapshotID = "snap-test-001"
	}
	return m.snapshotID, nil
}

func (m *mockSnapshotter) RestoreSnapshot(_ context.Context, _ string) error {
	m.restored = true
	return m.restoreErr
}

type mockExecutor struct {
	executeErr  error
	rollbackErr error
	executed    []string
	rolledBack  []string
}

func (m *mockExecutor) Execute(_ context.Context, _ *DrillPlan, step StepDef) error {
	m.executed = append(m.executed, step.Name)
	return m.executeErr
}

func (m *mockExecutor) Rollback(_ context.Context, _ *DrillPlan, step StepDef) error {
	m.rolledBack = append(m.rolledBack, step.Name)
	return m.rollbackErr
}

func newTestManager() (*Manager, *mockSnapshotter, *mockExecutor) {
	logger, _ := zap.NewDevelopment()
	snap := &mockSnapshotter{}
	exec := &mockExecutor{}
	mgr := NewManager(logger, snap, exec)
	return mgr, snap, exec
}

func setupRouter(h *Handlers) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)
	return r
}

// ─────────────────────── 计划 CRUD 测试 ───────────────────────

func TestCreatePlan(t *testing.T) {
	mgr, _, _ := newTestManager()

	req := CreatePlanRequest{
		Name:  "季度磁盘故障演练",
		Type:  string(DrillTypeDiskFault),
		Scope: string(ScopeSystem),
		Mode:  string(ModeDryRun),
		Steps: []StepDef{
			{Name: "模拟磁盘故障", Description: "拔除磁盘", Timeout: 30 * time.Second, MaxRetries: 1},
			{Name: "验证降级", Description: "检查存储池状态"},
			{Name: "恢复磁盘", Description: "重新接入磁盘"},
		},
	}

	plan, err := mgr.CreatePlan(req)
	require.NoError(t, err)
	assert.NotEmpty(t, plan.ID)
	assert.Equal(t, "季度磁盘故障演练", plan.Name)
	assert.Equal(t, DrillTypeDiskFault, plan.Type)
	assert.Equal(t, ScopeSystem, plan.Scope)
	assert.Equal(t, ModeDryRun, plan.Mode)
	assert.Len(t, plan.Steps, 3)
	assert.True(t, plan.Enabled)
}

func TestCreatePlan_InvalidType(t *testing.T) {
	mgr, _, _ := newTestManager()

	req := CreatePlanRequest{
		Name:  "test",
		Type:  "invalid_type",
		Scope: string(ScopeSystem),
		Mode:  string(ModeDryRun),
		Steps: []StepDef{{Name: "step1"}},
	}

	_, err := mgr.CreatePlan(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid drill type")
}

func TestCreatePlan_InvalidScope(t *testing.T) {
	mgr, _, _ := newTestManager()

	req := CreatePlanRequest{
		Name:  "test",
		Type:  string(DrillTypeDiskFault),
		Scope: "invalid_scope",
		Mode:  string(ModeDryRun),
		Steps: []StepDef{{Name: "step1"}},
	}

	_, err := mgr.CreatePlan(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid drill scope")
}

func TestCreatePlan_InvalidMode(t *testing.T) {
	mgr, _, _ := newTestManager()

	req := CreatePlanRequest{
		Name:  "test",
		Type:  string(DrillTypeDiskFault),
		Scope: string(ScopeSystem),
		Mode:  "turbo",
		Steps: []StepDef{{Name: "step1"}},
	}

	_, err := mgr.CreatePlan(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid drill mode")
}

func TestListPlans(t *testing.T) {
	mgr, _, _ := newTestManager()

	assert.Empty(t, mgr.ListPlans())

	mgr.CreatePlan(CreatePlanRequest{
		Name:  "plan1",
		Type:  string(DrillTypeDiskFault),
		Scope: string(ScopeSystem),
		Mode:  string(ModeDryRun),
		Steps: []StepDef{{Name: "s1"}},
	})
	mgr.CreatePlan(CreatePlanRequest{
		Name:  "plan2",
		Type:  string(DrillTypeServiceDown),
		Scope: string(ScopeService),
		Mode:  string(ModeReal),
		Steps: []StepDef{{Name: "s1"}},
	})

	plans := mgr.ListPlans()
	assert.Len(t, plans, 2)
}

func TestGetPlan(t *testing.T) {
	mgr, _, _ := newTestManager()

	plan, _ := mgr.CreatePlan(CreatePlanRequest{
		Name:  "test-plan",
		Type:  string(DrillTypeNetworkDown),
		Scope: string(ScopePool),
		Mode:  string(ModeDryRun),
		Steps: []StepDef{{Name: "s1"}},
	})

	got, err := mgr.GetPlan(plan.ID)
	require.NoError(t, err)
	assert.Equal(t, "test-plan", got.Name)

	_, err = mgr.GetPlan("nonexistent")
	assert.Error(t, err)
}

// ─────────────────────── 执行测试 ───────────────────────

func TestExecutePlan_Success(t *testing.T) {
	mgr, snap, _ := newTestManager()

	plan, _ := mgr.CreatePlan(CreatePlanRequest{
		Name:  "test",
		Type:  string(DrillTypeDiskFault),
		Scope: string(ScopeSystem),
		Mode:  string(ModeDryRun),
		Steps: []StepDef{
			{Name: "step1", MaxRetries: 0},
			{Name: "step2", MaxRetries: 0},
		},
	})

	exec, err := mgr.ExecutePlan(context.Background(), plan.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, exec.ID)
	assert.Equal(t, ExecRunning, exec.Status)

	// 等待异步完成
	time.Sleep(200 * time.Millisecond)

	exec, _ = mgr.GetExecution(exec.ID)
	assert.Equal(t, ExecSuccess, exec.Status)
	assert.True(t, snap.restored) // 执行后恢复保护点
	assert.Len(t, exec.StepResults, 2)
	for _, sr := range exec.StepResults {
		assert.Equal(t, StepSuccess, sr.Status)
	}
}

func TestExecutePlan_StepFailure(t *testing.T) {
	mgr, _, exec := newTestManager()
	exec.executeErr = errors.New("step failed")

	plan, _ := mgr.CreatePlan(CreatePlanRequest{
		Name:  "fail-test",
		Type:  string(DrillTypeDiskFault),
		Scope: string(ScopeSystem),
		Mode:  string(ModeDryRun),
		Steps: []StepDef{
			{Name: "step1", MaxRetries: 0, Rollback: "undo step1"},
			{Name: "step2", MaxRetries: 0},
		},
	})

	drillExec, err := mgr.ExecutePlan(context.Background(), plan.ID)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	drillExec, _ = mgr.GetExecution(drillExec.ID)
	assert.Equal(t, ExecFailed, drillExec.Status)
	assert.Equal(t, StepRolledBack, drillExec.StepResults[0].Status) // 回滚成功后状态变为 rolled_back
	assert.Equal(t, StepPending, drillExec.StepResults[1].Status) // 第二步未执行
	assert.Contains(t, drillExec.StepResults[0].Error, "step failed")
	assert.Contains(t, exec.rolledBack, "step1")
}

func TestExecutePlan_PlanNotFound(t *testing.T) {
	mgr, _, _ := newTestManager()
	_, err := mgr.ExecutePlan(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestExecutePlan_SnapshotCreateError(t *testing.T) {
	mgr, snap, _ := newTestManager()
	snap.createErr = errors.New("snapshot failed")

	plan, _ := mgr.CreatePlan(CreatePlanRequest{
		Name:  "snap-fail",
		Type:  string(DrillTypeDiskFault),
		Scope: string(ScopeSystem),
		Mode:  string(ModeDryRun),
		Steps: []StepDef{{Name: "step1"}},
	})

	drillExec, err := mgr.ExecutePlan(context.Background(), plan.ID)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	drillExec, _ = mgr.GetExecution(drillExec.ID)
	assert.Equal(t, ExecFailed, drillExec.Status)
	assert.Contains(t, drillExec.ErrorMessage, "创建保护点失败")
}

func TestExecutePlan_WithRetries(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	snap := &mockSnapshotter{}
	retryExec := &retryExecutor{failUntil: 2}
	mgr := NewManager(logger, snap, retryExec)

	plan, _ := mgr.CreatePlan(CreatePlanRequest{
		Name:  "retry-test",
		Type:  string(DrillTypeDiskFault),
		Scope: string(ScopeSystem),
		Mode:  string(ModeDryRun),
		Steps: []StepDef{{Name: "flaky-step", MaxRetries: 3}},
	})

	drillExec, err := mgr.ExecutePlan(context.Background(), plan.ID)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	drillExec, _ = mgr.GetExecution(drillExec.ID)
	assert.Equal(t, ExecSuccess, drillExec.Status)
	assert.Equal(t, StepSuccess, drillExec.StepResults[0].Status)
	assert.Equal(t, 2, drillExec.StepResults[0].Retried) // 重试了2次
}

type retryExecutor struct {
	callCount int
	failUntil int
}

func (r *retryExecutor) Execute(_ context.Context, _ *DrillPlan, _ StepDef) error {
	r.callCount++
	if r.callCount <= r.failUntil {
		return errors.New("transient error")
	}
	return nil
}

func (r *retryExecutor) Rollback(_ context.Context, _ *DrillPlan, _ StepDef) error {
	return nil
}

// ─────────────────────── 查询测试 ───────────────────────

func TestListExecutions(t *testing.T) {
	mgr, _, _ := newTestManager()
	assert.Empty(t, mgr.ListExecutions())

	plan, _ := mgr.CreatePlan(CreatePlanRequest{
		Name:  "test",
		Type:  string(DrillTypeDiskFault),
		Scope: string(ScopeSystem),
		Mode:  string(ModeDryRun),
		Steps: []StepDef{{Name: "s1"}},
	})

	mgr.ExecutePlan(context.Background(), plan.ID)
	time.Sleep(100 * time.Millisecond)

	execs := mgr.ListExecutions()
	assert.Len(t, execs, 1)
}

func TestGetReport(t *testing.T) {
	mgr, _, _ := newTestManager()

	plan, _ := mgr.CreatePlan(CreatePlanRequest{
		Name:  "report-test",
		Type:  string(DrillTypeDataRecovery),
		Scope: string(ScopeSystem),
		Mode:  string(ModeDryRun),
		Steps: []StepDef{{Name: "verify-data"}},
	})

	exec, _ := mgr.ExecutePlan(context.Background(), plan.ID)
	time.Sleep(200 * time.Millisecond)

	report, err := mgr.GetReport(exec.ID)
	require.NoError(t, err)
	assert.Equal(t, ExecSuccess, report.Status)
	assert.Equal(t, "report-test", report.PlanName)
	assert.NotNil(t, report.Suggestions)
	assert.NotNil(t, report.Trend)
}

func TestGetMetrics(t *testing.T) {
	mgr, _, _ := newTestManager()

	metrics := mgr.GetMetrics()
	assert.Equal(t, 0, metrics.TotalExecs)

	plan, _ := mgr.CreatePlan(CreatePlanRequest{
		Name:  "metrics-test",
		Type:  string(DrillTypeDiskFault),
		Scope: string(ScopeSystem),
		Mode:  string(ModeDryRun),
		Steps: []StepDef{{Name: "s1"}},
	})

	mgr.ExecutePlan(context.Background(), plan.ID)
	time.Sleep(200 * time.Millisecond)

	metrics = mgr.GetMetrics()
	assert.Equal(t, 1, metrics.TotalExecs)
	assert.Equal(t, 1, metrics.TotalPlans)
	assert.Equal(t, 100.0, metrics.SuccessRate)
	assert.False(t, metrics.LastDrillTime.IsZero())
}

// ─────────────────────── HTTP Handler 测试 ───────────────────────

func TestHTTP_CreatePlan(t *testing.T) {
	mgr, _, _ := newTestManager()
	h := NewHandlers(mgr, zap.NewNop())
	r := setupRouter(h)

	body := `{
		"name": "HTTP演练",
		"type": "disk_fault",
		"scope": "system",
		"mode": "dry_run",
		"steps": [{"name": "step1", "description": "test"}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dr-drill/plans", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 0, resp.Code)
	assert.NotNil(t, resp.Data)
}

func TestHTTP_CreatePlan_Invalid(t *testing.T) {
	mgr, _, _ := newTestManager()
	h := NewHandlers(mgr, zap.NewNop())
	r := setupRouter(h)

	body := `{"name": "bad"}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dr-drill/plans", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHTTP_ListPlans(t *testing.T) {
	mgr, _, _ := newTestManager()
	h := NewHandlers(mgr, zap.NewNop())
	r := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dr-drill/plans", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTP_GetPlan(t *testing.T) {
	mgr, _, _ := newTestManager()
	h := NewHandlers(mgr, zap.NewNop())
	r := setupRouter(h)

	plan, _ := mgr.CreatePlan(CreatePlanRequest{
		Name:  "http-plan",
		Type:  string(DrillTypeDiskFault),
		Scope: string(ScopeSystem),
		Mode:  string(ModeDryRun),
		Steps: []StepDef{{Name: "s1"}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dr-drill/plans/"+plan.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 不存在的
	req = httptest.NewRequest(http.MethodGet, "/api/v1/dr-drill/plans/nonexistent", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHTTP_ExecutePlan(t *testing.T) {
	mgr, _, _ := newTestManager()
	h := NewHandlers(mgr, zap.NewNop())
	r := setupRouter(h)

	plan, _ := mgr.CreatePlan(CreatePlanRequest{
		Name:  "exec-plan",
		Type:  string(DrillTypeDiskFault),
		Scope: string(ScopeSystem),
		Mode:  string(ModeDryRun),
		Steps: []StepDef{{Name: "s1"}},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dr-drill/plans/"+plan.ID+"/execute", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestHTTP_ListExecutions(t *testing.T) {
	mgr, _, _ := newTestManager()
	h := NewHandlers(mgr, zap.NewNop())
	r := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dr-drill/executions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTP_GetExecution(t *testing.T) {
	mgr, _, _ := newTestManager()
	h := NewHandlers(mgr, zap.NewNop())
	r := setupRouter(h)

	plan, _ := mgr.CreatePlan(CreatePlanRequest{
		Name:  "get-exec",
		Type:  string(DrillTypeDiskFault),
		Scope: string(ScopeSystem),
		Mode:  string(ModeDryRun),
		Steps: []StepDef{{Name: "s1"}},
	})

	exec, _ := mgr.ExecutePlan(context.Background(), plan.ID)
	time.Sleep(200 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dr-drill/executions/"+exec.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTP_GetReport(t *testing.T) {
	mgr, _, _ := newTestManager()
	h := NewHandlers(mgr, zap.NewNop())
	r := setupRouter(h)

	plan, _ := mgr.CreatePlan(CreatePlanRequest{
		Name:  "report",
		Type:  string(DrillTypeDataRecovery),
		Scope: string(ScopeSystem),
		Mode:  string(ModeDryRun),
		Steps: []StepDef{{Name: "s1"}},
	})

	exec, _ := mgr.ExecutePlan(context.Background(), plan.ID)
	time.Sleep(200 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dr-drill/executions/"+exec.ID+"/report", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTP_GetMetrics(t *testing.T) {
	mgr, _, _ := newTestManager()
	h := NewHandlers(mgr, zap.NewNop())
	r := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dr-drill/metrics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 0, resp.Code)
}

// ─────────────────────── 校验函数测试 ───────────────────────

func TestValidateType(t *testing.T) {
	assert.NoError(t, validateType("disk_fault"))
	assert.NoError(t, validateType("network_down"))
	assert.NoError(t, validateType("pool_degrade"))
	assert.NoError(t, validateType("service_down"))
	assert.NoError(t, validateType("data_recovery"))
	assert.Error(t, validateType("unknown"))
}

func TestValidateScope(t *testing.T) {
	assert.NoError(t, validateScope("system"))
	assert.NoError(t, validateScope("pool"))
	assert.NoError(t, validateScope("service"))
	assert.Error(t, validateScope("global"))
}

func TestValidateMode(t *testing.T) {
	assert.NoError(t, validateMode("dry_run"))
	assert.NoError(t, validateMode("real"))
	assert.Error(t, validateMode("simulation"))
}
