// Package complianceaudit 单元测试
package complianceaudit

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// ========== Mock Check ==========

// mockComplianceCheck 模拟合规检查项
type mockComplianceCheck struct {
	name        string
	standard    ComplianceStandard
	category    CheckCategory
	description string
	status      CheckStatus
	riskLevel   RiskLevel
	message     string
	passed      bool
	checkCalled bool
}

func newMockCheck(name string, standard ComplianceStandard, status CheckStatus) *mockComplianceCheck {
	return &mockComplianceCheck{
		name:        name,
		standard:    standard,
		category:    CategoryPasswordPolicy,
		description: "mock check: " + name,
		status:      status,
		riskLevel:   RiskMedium,
		message:     "mock result",
		passed:      status == StatusPass,
	}
}

func (c *mockComplianceCheck) Name() string                { return c.name }
func (c *mockComplianceCheck) Standard() ComplianceStandard { return c.standard }
func (c *mockComplianceCheck) Category() CheckCategory      { return c.category }
func (c *mockComplianceCheck) Description() string          { return c.description }

func (c *mockComplianceCheck) Check(ctx *CheckContext) *CheckResult {
	c.checkCalled = true
	return &CheckResult{
		Name:      c.name,
		Standard:  c.standard,
		Category:  c.category,
		Status:    c.status,
		RiskLevel: c.riskLevel,
		Message:   c.message,
		Timestamp: time.Now(),
	}
}

func (c *mockComplianceCheck) GetRemediation(result *CheckResult) *Remediation {
	if result.Status == StatusFail {
		return &Remediation{
			Title:       "修复 " + c.name,
			Description: "请按照以下步骤修复",
			Steps:       []string{"步骤1", "步骤2"},
			Priority:    3,
			Deadline:    30,
		}
	}
	return nil
}

// ========== Manager Tests ==========

func TestNewManager(t *testing.T) {
	m := NewManager(nil, nil)
	assert.NotNil(t, m)
	assert.NotNil(t, m.checks)
	assert.NotNil(t, m.config)
	assert.True(t, m.config.Enabled)
	assert.True(t, m.config.NotifyOnFail)
}

func TestManager_RegisterCheck(t *testing.T) {
	m := NewManager(nil, nil)
	check := newMockCheck("test_check", StandardGDPR, StatusPass)

	m.RegisterCheck(check)

	c, ok := m.GetCheck("test_check")
	assert.True(t, ok)
	assert.Equal(t, "test_check", c.Name())
}

func TestManager_UnregisterCheck(t *testing.T) {
	m := NewManager(nil, nil)
	check := newMockCheck("test_check", StandardGDPR, StatusPass)

	m.RegisterCheck(check)
	m.UnregisterCheck("test_check")

	_, ok := m.GetCheck("test_check")
	assert.False(t, ok)
}

func TestManager_ListChecks(t *testing.T) {
	m := NewManager(nil, nil)
	m.RegisterCheck(newMockCheck("check1", StandardGDPR, StatusPass))
	m.RegisterCheck(newMockCheck("check2", StandardISO27001, StatusPass))

	list := m.ListChecks()
	assert.Len(t, list, 2)

	names := make([]string, 0, 2)
	for _, info := range list {
		names = append(names, info.Name)
	}
	assert.Contains(t, names, "check1")
	assert.Contains(t, names, "check2")
}

func TestManager_RunFullScan_AllPass(t *testing.T) {
	m := NewManager(nil, nil)
	m.RegisterCheck(newMockCheck("ok1", StandardGDPR, StatusPass))
	m.RegisterCheck(newMockCheck("ok2", StandardISO27001, StatusPass))

	report := m.RunFullScan(context.Background())

	assert.Equal(t, 2, report.Summary.TotalChecks)
	assert.Equal(t, 2, report.Summary.Passed)
	assert.Equal(t, 0, report.Summary.Failed)
	assert.Equal(t, 100.0, report.Summary.OverallScore)
	assert.Equal(t, RiskLow, report.Summary.RiskLevel)
}

func TestManager_RunFullScan_WithFailures(t *testing.T) {
	m := NewManager(nil, nil)
	m.RegisterCheck(newMockCheck("ok", StandardGDPR, StatusPass))
	m.RegisterCheck(newMockCheck("fail", StandardGDPR, StatusFail))

	report := m.RunFullScan(context.Background())

	assert.Equal(t, 2, report.Summary.TotalChecks)
	assert.Equal(t, 1, report.Summary.Passed)
	assert.Equal(t, 1, report.Summary.Failed)
	assert.Equal(t, 50.0, report.Summary.OverallScore)
	assert.Equal(t, RiskHigh, report.Summary.RiskLevel)
	assert.Len(t, report.Findings, 1)
	assert.Len(t, report.Remediations, 1)
}

func TestManager_RunStandardScan(t *testing.T) {
	m := NewManager(nil, nil)
	m.RegisterCheck(newMockCheck("gdpr1", StandardGDPR, StatusPass))
	m.RegisterCheck(newMockCheck("gdpr2", StandardGDPR, StatusPass))
	m.RegisterCheck(newMockCheck("iso1", StandardISO27001, StatusPass))

	report := m.RunStandardScan(context.Background(), StandardGDPR)

	assert.Equal(t, 2, report.Summary.TotalChecks)
	assert.Equal(t, 2, report.Summary.Passed)
	assert.Len(t, report.Standards, 1)
	assert.Equal(t, StandardGDPR, report.Standards[0].Standard)
}

func TestManager_RunSingleCheck(t *testing.T) {
	m := NewManager(nil, nil)
	check := newMockCheck("single_test", StandardGDPR, StatusPass)
	m.RegisterCheck(check)

	result, err := m.RunSingleCheck(context.Background(), "single_test")
	require.NoError(t, err)
	assert.Equal(t, StatusPass, result.Status)
	assert.True(t, check.checkCalled)
}

func TestManager_RunSingleCheck_NotFound(t *testing.T) {
	m := NewManager(nil, nil)

	_, err := m.RunSingleCheck(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_GetLastReport(t *testing.T) {
	m := NewManager(nil, nil)
	m.RegisterCheck(newMockCheck("test", StandardGDPR, StatusPass))

	// 初始为空
	assert.Nil(t, m.GetLastReport())

	// 执行后有缓存
	m.RunFullScan(context.Background())
	report := m.GetLastReport()
	assert.NotNil(t, report)
	assert.Equal(t, 1, report.Summary.TotalChecks)
}

func TestManager_UpdateConfig(t *testing.T) {
	m := NewManager(nil, nil)

	newCfg := &ScanConfig{
		Standards:     []ComplianceStandard{StandardGDPR},
		Schedule:      "0 0 * * 1",
		Enabled:       false,
		NotifyOnFail:  false,
		AutoRemediate: true,
	}
	m.UpdateConfig(newCfg)

	cfg := m.GetConfig()
	assert.Equal(t, []ComplianceStandard{StandardGDPR}, cfg.Standards)
	assert.Equal(t, "0 0 * * 1", cfg.Schedule)
	assert.False(t, cfg.Enabled)
	assert.True(t, cfg.AutoRemediate)
}

func TestManager_GetComplianceScore(t *testing.T) {
	m := NewManager(nil, nil)
	m.RegisterCheck(newMockCheck("test", StandardGDPR, StatusPass))

	// 先执行扫描生成报告
	m.RunFullScan(context.Background())

	score := m.GetComplianceScore()
	assert.NotNil(t, score)
	assert.Equal(t, 100.0, score.Overall)
	assert.NotZero(t, score.LastUpdated)
}

func TestManager_GetDashboard(t *testing.T) {
	m := NewManager(nil, nil)
	m.RegisterCheck(newMockCheck("test", StandardGDPR, StatusPass))

	// 先执行扫描
	m.RunFullScan(context.Background())

	dashboard := m.GetDashboard()
	assert.NotNil(t, dashboard)
	assert.NotNil(t, dashboard.Score)
	assert.NotZero(t, dashboard.LastScanTime)
	assert.NotNil(t, dashboard.StandardsStatus)
}

func TestManager_Scheduler(t *testing.T) {
	m := NewManager(nil, nil)
	check := newMockCheck("scheduler_test", StandardGDPR, StatusPass)
	m.RegisterCheck(check)

	m.Start()
	assert.True(t, m.IsRunning())

	// 等待调度执行
	time.Sleep(100 * time.Millisecond)

	report := m.GetLastReport()
	assert.NotNil(t, report)

	m.Stop()
	time.Sleep(10 * time.Millisecond)
	assert.False(t, m.IsRunning())
}

func TestManager_DoubleStartStop(t *testing.T) {
	m := NewManager(nil, nil)

	// 双重 start 不应 panic
	m.Start()
	m.Start()
	assert.True(t, m.IsRunning())

	// 双重 stop 不应 panic
	m.Stop()
	m.Stop()
	assert.False(t, m.IsRunning())
}

func TestManager_CalculateRiskLevel(t *testing.T) {
	m := NewManager(nil, nil)

	assert.Equal(t, RiskLow, m.calculateRiskLevel(95))
	assert.Equal(t, RiskMedium, m.calculateRiskLevel(80))
	assert.Equal(t, RiskHigh, m.calculateRiskLevel(60))
	assert.Equal(t, RiskCritical, m.calculateRiskLevel(40))
}

func TestManager_IsStandardEnabled(t *testing.T) {
	m := NewManager(nil, nil)

	config := &ScanConfig{
		Standards: []ComplianceStandard{StandardGDPR, StandardISO27001},
	}

	assert.True(t, m.isStandardEnabled(config, StandardGDPR))
	assert.True(t, m.isStandardEnabled(config, StandardISO27001))
	assert.False(t, m.isStandardEnabled(config, StandardSOC2))
}

// ========== Types Tests ==========

func TestTypes_Constants(t *testing.T) {
	assert.Equal(t, ComplianceStandard("gdpr"), StandardGDPR)
	assert.Equal(t, ComplianceStandard("mlps2"), StandardMLPS2)
	assert.Equal(t, ComplianceStandard("iso27001"), StandardISO27001)
	assert.Equal(t, ComplianceStandard("soc2"), StandardSOC2)

	assert.Equal(t, CheckStatus("pass"), StatusPass)
	assert.Equal(t, CheckStatus("fail"), StatusFail)
	assert.Equal(t, CheckStatus("warn"), StatusWarn)
	assert.Equal(t, CheckStatus("skip"), StatusSkip)

	assert.Equal(t, RiskLevel("low"), RiskLow)
	assert.Equal(t, RiskLevel("medium"), RiskMedium)
	assert.Equal(t, RiskLevel("high"), RiskHigh)
	assert.Equal(t, RiskLevel("critical"), RiskCritical)
}

func TestCheckResult_Fields(t *testing.T) {
	now := time.Now()
	result := &CheckResult{
		Name:      "test",
		Standard:  StandardGDPR,
		Category:  CategoryPasswordPolicy,
		Status:    StatusPass,
		RiskLevel: RiskLow,
		Message:   "all good",
		Timestamp: now,
		Duration:  100 * time.Millisecond,
	}

	assert.Equal(t, "test", result.Name)
	assert.Equal(t, StandardGDPR, result.Standard)
	assert.Equal(t, StatusPass, result.Status)
	assert.Equal(t, RiskLow, result.RiskLevel)
}

func TestRemediation_Fields(t *testing.T) {
	r := &Remediation{
		Title:       "修复密码策略",
		Description: "加强密码复杂度要求",
		Steps:       []string{"修改配置", "重启服务"},
		Priority:    4,
		Deadline:    7,
	}

	assert.Equal(t, "修复密码策略", r.Title)
	assert.Len(t, r.Steps, 2)
	assert.Equal(t, 4, r.Priority)
}

func TestScanConfig_DefaultValues(t *testing.T) {
	cfg := &ScanConfig{
		Standards:     []ComplianceStandard{StandardGDPR, StandardMLPS2, StandardISO27001, StandardSOC2},
		Schedule:      "0 0 * * *",
		Enabled:       true,
		NotifyOnFail:  true,
		AutoRemediate: false,
	}

	assert.Len(t, cfg.Standards, 4)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, "0 0 * * *", cfg.Schedule)
}

// ========== Handlers Tests ==========

func setupHandlers(t *testing.T) (*gin.Engine, *Manager) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	m := NewManager(nil, nil)
	h := NewHandlers(m)

	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)

	return r, m
}

func TestHandlers_GetDashboard(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/compliance/dashboard", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "score")
}

func TestHandlers_GetScore(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/compliance/score", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "overall")
}

func TestHandlers_ListChecks(t *testing.T) {
	r, m := setupHandlers(t)

	m.RegisterCheck(newMockCheck("check1", StandardGDPR, StatusPass))
	m.RegisterCheck(newMockCheck("check2", StandardISO27001, StatusPass))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/compliance/checks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "check1")
	assert.Contains(t, w.Body.String(), "check2")
}

func TestHandlers_RunScan(t *testing.T) {
	r, m := setupHandlers(t)

	m.RegisterCheck(newMockCheck("scan1", StandardGDPR, StatusPass))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/compliance/scan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "scan completed")
}

func TestHandlers_RunStandardScan(t *testing.T) {
	r, m := setupHandlers(t)

	m.RegisterCheck(newMockCheck("gdpr1", StandardGDPR, StatusPass))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/compliance/scan/gdpr", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "scan completed")
}

func TestHandlers_RunStandardScan_Invalid(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/compliance/scan/invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_GetLatestReport(t *testing.T) {
	r, m := setupHandlers(t)

	// 无报告时返回 404
	req := httptest.NewRequest(http.MethodGet, "/api/v1/compliance/reports/latest", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// 执行扫描后可获取
	m.RegisterCheck(newMockCheck("test", StandardGDPR, StatusPass))
	m.RunFullScan(context.Background())

	req = httptest.NewRequest(http.MethodGet, "/api/v1/compliance/reports/latest", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_GetConfig(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/compliance/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "enabled")
}

func TestHandlers_UpdateConfig(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `{"standards":["gdpr"],"schedule":"0 0 * * 1","enabled":true,"notify_on_fail":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/compliance/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "config updated")
}

func TestHandlers_UpdateConfig_Invalid(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `invalid json`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/compliance/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_CollectAuditLog(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `{"actor":"admin","action":"login","resource":"/api","result":"success"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/compliance/audit-logs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 无 store 时返回错误
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandlers_GetAuditLogs(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/compliance/audit-logs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 无 store 时返回错误
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandlers_GetReports(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/compliance/reports", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 无 store 时返回错误
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ========== Store Tests ==========

func setupTestStore(t *testing.T) (*Store, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "compliance_test_*.db")
	require.NoError(t, err)
	_ = tmpFile.Close()

	db, err := sql.Open("sqlite", tmpFile.Name())
	require.NoError(t, err)

	store := NewStore(db, nil)
	require.NoError(t, store.Init())

	cleanup := func() {
		_ = db.Close()
		_ = os.Remove(tmpFile.Name())
	}

	return store, cleanup
}

func TestStore_Init(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// 验证表已创建
	var tableName string
	err := store.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='compliance_reports'").Scan(&tableName)
	require.NoError(t, err)
	assert.Equal(t, "compliance_reports", tableName)

	err = store.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='audit_logs'").Scan(&tableName)
	require.NoError(t, err)
	assert.Equal(t, "audit_logs", tableName)
}

func TestStore_SaveAndQueryReport(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	report := &ComplianceReport{
		ID:          "test_report_1",
		Title:       "测试报告",
		GeneratedAt: time.Now(),
		Period: ReportPeriod{
			Start: time.Now().AddDate(0, 0, -1),
			End:   time.Now(),
		},
		Summary: &ReportSummary{
			TotalChecks:  10,
			Passed:       8,
			Failed:       2,
			OverallScore: 80.0,
			RiskLevel:    RiskMedium,
		},
		Standards:    make([]*StandardReport, 0),
		Findings:     make([]*Finding, 0),
		Remediations: make([]*RemediationItem, 0),
		Format:       FormatJSON,
	}

	err := store.SaveReport(report)
	require.NoError(t, err)

	reports, err := store.GetReports(10)
	require.NoError(t, err)
	assert.Len(t, reports, 1)
	assert.Equal(t, "test_report_1", reports[0].ID)
	assert.Equal(t, 80.0, reports[0].Summary.OverallScore)
}

func TestStore_SaveAndQueryAuditLog(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	log := &AuditLog{
		Timestamp: time.Now(),
		Actor:     "admin",
		Action:    "login",
		Resource:  "/api",
		Result:    "success",
		IPAddress: "192.168.1.1",
	}

	err := store.SaveAuditLog(log)
	require.NoError(t, err)

	logs, err := store.GetAuditLogs("", 10)
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "admin", logs[0].Actor)
	assert.Equal(t, "login", logs[0].Action)
}

func TestStore_GetAuditLogs_ByActor(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// 保存多条日志
	store.SaveAuditLog(&AuditLog{
		Timestamp: time.Now(),
		Actor:     "admin",
		Action:    "login",
		Result:    "success",
	})
	store.SaveAuditLog(&AuditLog{
		Timestamp: time.Now(),
		Actor:     "user1",
		Action:    "view",
		Result:    "success",
	})
	store.SaveAuditLog(&AuditLog{
		Timestamp: time.Now(),
		Actor:     "admin",
		Action:    "logout",
		Result:    "success",
	})

	// 查询 admin 的日志
	logs, err := store.GetAuditLogs("admin", 10)
	require.NoError(t, err)
	assert.Len(t, logs, 2)

	// 查询 user1 的日志
	logs, err = store.GetAuditLogs("user1", 10)
	require.NoError(t, err)
	assert.Len(t, logs, 1)
}

func TestStore_Cleanup(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// 插入旧日志
	_, err := store.db.Exec(`
		INSERT INTO audit_logs (timestamp, actor, action, result)
		VALUES (?, ?, ?, ?)
	`, time.Now().Add(-48*time.Hour), "old_user", "old_action", "success")
	require.NoError(t, err)

	// 插入新日志
	store.SaveAuditLog(&AuditLog{
		Timestamp: time.Now(),
		Actor:     "new_user",
		Action:    "new_action",
		Result:    "success",
	})

	// 清理 24 小时前的数据
	err = store.Cleanup(24 * time.Hour)
	require.NoError(t, err)

	// 应该只剩新日志
	logs, err := store.GetAuditLogs("", 100)
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "new_user", logs[0].Actor)
}

// ========== Integration Test ==========

func TestIntegration_FullFlow(t *testing.T) {
	// 创建带 store 的 manager
	tmpFile, err := os.CreateTemp("", "compliance_integ_*.db")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	_ = tmpFile.Close()

	db, err := sql.Open("sqlite", tmpFile.Name())
	require.NoError(t, err)
	defer db.Close()

	store := NewStore(db, nil)
	require.NoError(t, store.Init())

	m := NewManager(store, nil)

	// 注册多种检查项
	m.RegisterCheck(newMockCheck("password_policy", StandardGDPR, StatusPass))
	m.RegisterCheck(newMockCheck("access_control", StandardISO27001, StatusPass))
	m.RegisterCheck(newMockCheck("encryption", StandardSOC2, StatusFail))
	m.RegisterCheck(newMockCheck("audit_log", StandardMLPS2, StatusPass))

	// 执行全量扫描
	report := m.RunFullScan(context.Background())

	assert.Equal(t, 4, report.Summary.TotalChecks)
	assert.Equal(t, 3, report.Summary.Passed)
	assert.Equal(t, 1, report.Summary.Failed)
	assert.Equal(t, 75.0, report.Summary.OverallScore)
	assert.Equal(t, RiskMedium, report.Summary.RiskLevel)
	assert.Len(t, report.Findings, 1)
	assert.Len(t, report.Remediations, 1)

	// 验证列表
	list := m.ListChecks()
	assert.Len(t, list, 4)

	// 验证配置
	cfg := m.GetConfig()
	assert.True(t, cfg.Enabled)

	// 验证评分
	score := m.GetComplianceScore()
	assert.Equal(t, 75.0, score.Overall)

	// 验证仪表盘
	dashboard := m.GetDashboard()
	assert.NotNil(t, dashboard)
	assert.Equal(t, 1, dashboard.ActiveRemediations)

	// 验证报告持久化
	reports, err := m.GetReports(10)
	require.NoError(t, err)
	assert.Len(t, reports, 1)

	// 验证单个标准扫描
	report = m.RunStandardScan(context.Background(), StandardGDPR)
	assert.Equal(t, 1, report.Summary.TotalChecks)
	assert.Equal(t, 100.0, report.Summary.OverallScore)
}

// ========== Benchmark ==========

func BenchmarkManager_RunFullScan(b *testing.B) {
	m := NewManager(nil, nil)
	for i := 0; i < 10; i++ {
		m.RegisterCheck(newMockCheck("bench_check", StandardGDPR, StatusPass))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RunFullScan(context.Background())
	}
}
