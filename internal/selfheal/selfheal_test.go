// Package selfheal 单元测试
package selfheal

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

// ========== Mock Checker ==========

// mockChecker 模拟检查器.
type mockChecker struct {
	name        string
	category    CheckCategory
	description string
	status      Status
	message     string
	details     map[string]interface{}
	healAction  HealAction
	healSuccess bool
	healMsg     string
	checkCalled bool
	healCalled  bool
}

func newMockChecker(name string, status Status) *mockChecker {
	return &mockChecker{
		name:        name,
		category:    CategoryCustom,
		description: "mock checker: " + name,
		status:      status,
		message:     "mock message",
		details:     map[string]interface{}{"test": true},
		healAction:  HealActionNone,
		healSuccess: false,
		healMsg:     "not healed",
	}
}

func (c *mockChecker) Name() string            { return c.name }
func (c *mockChecker) Category() CheckCategory { return c.category }
func (c *mockChecker) Description() string     { return c.description }
func (c *mockChecker) HealAction() HealAction  { return c.healAction }

func (c *mockChecker) Check(ctx *CheckContext) *CheckResult {
	c.checkCalled = true
	return &CheckResult{
		Name:      c.name,
		Category:  c.category,
		Status:    c.status,
		Message:   c.message,
		Details:   c.details,
		Timestamp: time.Now(),
	}
}

func (c *mockChecker) Heal(ctx *CheckContext, result *CheckResult) *HealResult {
	c.healCalled = true
	return &HealResult{
		Success: c.healSuccess,
		Action:  "mock_heal",
		Message: c.healMsg,
	}
}

// ========== Manager Tests ==========

func TestNewManager(t *testing.T) {
	m := NewManager(nil, nil)
	assert.NotNil(t, m)
	assert.NotNil(t, m.checkers)
	assert.NotNil(t, m.config)
	assert.True(t, m.config.Enabled)
	assert.Equal(t, HealActionNone, m.config.DefaultAction)
}

func TestManager_RegisterChecker(t *testing.T) {
	m := NewManager(nil, nil)
	checker := newMockChecker("test_check", StatusHealthy)

	m.RegisterChecker(checker)

	// 验证已注册
	c, ok := m.GetChecker("test_check")
	assert.True(t, ok)
	assert.Equal(t, "test_check", c.Name())
}

func TestManager_UnregisterChecker(t *testing.T) {
	m := NewManager(nil, nil)
	checker := newMockChecker("test_check", StatusHealthy)

	m.RegisterChecker(checker)
	m.UnregisterChecker("test_check")

	_, ok := m.GetChecker("test_check")
	assert.False(t, ok)
}

func TestManager_ListCheckers(t *testing.T) {
	m := NewManager(nil, nil)
	m.RegisterChecker(newMockChecker("check1", StatusHealthy))
	m.RegisterChecker(newMockChecker("check2", StatusDegraded))

	list := m.ListCheckers()
	assert.Len(t, list, 2)

	names := make([]string, 0, 2)
	for _, info := range list {
		names = append(names, info.Name)
	}
	assert.Contains(t, names, "check1")
	assert.Contains(t, names, "check2")
}

func TestManager_RunAll_Healthy(t *testing.T) {
	m := NewManager(nil, nil)
	m.RegisterChecker(newMockChecker("ok1", StatusHealthy))
	m.RegisterChecker(newMockChecker("ok2", StatusHealthy))

	status := m.RunAll(context.Background())

	assert.Equal(t, StatusHealthy, status.Status)
	assert.Equal(t, 2, status.Summary.Total)
	assert.Equal(t, 2, status.Summary.Healthy)
	assert.Equal(t, 0, status.Summary.Unhealthy)
}

func TestManager_RunAll_Unhealthy(t *testing.T) {
	m := NewManager(nil, nil)
	m.RegisterChecker(newMockChecker("ok1", StatusHealthy))
	m.RegisterChecker(newMockChecker("bad1", StatusUnhealthy))

	status := m.RunAll(context.Background())

	assert.Equal(t, StatusUnhealthy, status.Status)
	assert.Equal(t, 1, status.Summary.Healthy)
	assert.Equal(t, 1, status.Summary.Unhealthy)
}

func TestManager_RunAll_Degraded(t *testing.T) {
	m := NewManager(nil, nil)
	m.RegisterChecker(newMockChecker("ok1", StatusHealthy))
	m.RegisterChecker(newMockChecker("warn1", StatusDegraded))

	status := m.RunAll(context.Background())

	assert.Equal(t, StatusDegraded, status.Status)
	assert.Equal(t, 1, status.Summary.Healthy)
	assert.Equal(t, 1, status.Summary.Degraded)
}

func TestManager_RunSingle(t *testing.T) {
	m := NewManager(nil, nil)
	checker := newMockChecker("single_test", StatusHealthy)
	m.RegisterChecker(checker)

	result, err := m.RunSingle(context.Background(), "single_test")
	require.NoError(t, err)
	assert.Equal(t, StatusHealthy, result.Status)
	assert.True(t, checker.checkCalled)
}

func TestManager_RunSingle_NotFound(t *testing.T) {
	m := NewManager(nil, nil)

	_, err := m.RunSingle(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_GetLastStatus(t *testing.T) {
	m := NewManager(nil, nil)
	m.RegisterChecker(newMockChecker("test", StatusHealthy))

	// 初始为空
	assert.Nil(t, m.GetLastStatus())

	// 执行后有缓存
	m.RunAll(context.Background())
	status := m.GetLastStatus()
	assert.NotNil(t, status)
	assert.Equal(t, StatusHealthy, status.Status)
}

func TestManager_UpdateConfig(t *testing.T) {
	m := NewManager(nil, nil)

	// 更新配置
	newCfg := &StrategyConfig{
		DefaultAction: HealActionAuto,
		CheckInterval: 10 * time.Minute,
		Enabled:       false,
		Overrides: map[string]HealAction{
			"special_check": HealActionManual,
		},
	}
	m.UpdateConfig(newCfg)

	cfg := m.GetConfig()
	assert.Equal(t, HealActionAuto, cfg.DefaultAction)
	assert.Equal(t, 10*time.Minute, cfg.CheckInterval)
	assert.False(t, cfg.Enabled)
	assert.Equal(t, HealActionManual, cfg.Overrides["special_check"])
}

func TestManager_GetAction(t *testing.T) {
	m := NewManager(nil, nil)
	m.UpdateConfig(&StrategyConfig{
		DefaultAction: HealActionAuto,
		Overrides: map[string]HealAction{
			"special": HealActionManual,
		},
	})

	assert.Equal(t, HealActionAuto, m.getAction("normal"))
	assert.Equal(t, HealActionManual, m.getAction("special"))
}

func TestManager_AutoHeal(t *testing.T) {
	m := NewManager(nil, nil)
	m.UpdateConfig(&StrategyConfig{
		DefaultAction: HealActionAuto,
	})

	checker := newMockChecker("auto_heal_test", StatusUnhealthy)
	checker.healAction = HealActionAuto
	checker.healSuccess = true
	checker.healMsg = "fixed"
	m.RegisterChecker(checker)

	status := m.RunAll(context.Background())

	assert.Equal(t, 1, status.Healed)
	assert.True(t, checker.healCalled)
}

func TestManager_NoHeal_WhenHealthy(t *testing.T) {
	m := NewManager(nil, nil)
	m.UpdateConfig(&StrategyConfig{
		DefaultAction: HealActionAuto,
	})

	checker := newMockChecker("healthy_test", StatusHealthy)
	m.RegisterChecker(checker)

	m.RunAll(context.Background())

	// 健康状态不应触发 heal
	assert.False(t, checker.healCalled)
}

func TestManager_Scheduler(t *testing.T) {
	m := NewManager(nil, nil)
	m.UpdateConfig(&StrategyConfig{
		CheckInterval: 50 * time.Millisecond,
		Enabled:       true,
	})

	checker := newMockChecker("scheduler_test", StatusHealthy)
	m.RegisterChecker(checker)

	m.Start()
	assert.True(t, m.IsRunning())

	// 等待至少一次调度执行
	time.Sleep(100 * time.Millisecond)

	status := m.GetLastStatus()
	assert.NotNil(t, status)

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

// ========== Checker Type Tests ==========

func TestTypes_Constants(t *testing.T) {
	assert.Equal(t, Status("healthy"), StatusHealthy)
	assert.Equal(t, Status("unhealthy"), StatusUnhealthy)
	assert.Equal(t, Status("degraded"), StatusDegraded)

	assert.Equal(t, HealAction("none"), HealActionNone)
	assert.Equal(t, HealAction("auto"), HealActionAuto)
	assert.Equal(t, HealAction("manual"), HealActionManual)

	assert.Equal(t, CheckCategory("disk"), CategoryDisk)
	assert.Equal(t, CheckCategory("filesystem"), CategoryFilesystem)
	assert.Equal(t, CheckCategory("service"), CategoryService)
	assert.Equal(t, CheckCategory("config"), CategoryConfig)
	assert.Equal(t, CheckCategory("certificate"), CategoryCert)
	assert.Equal(t, CheckCategory("custom"), CategoryCustom)
}

func TestCheckResult_Fields(t *testing.T) {
	now := time.Now()
	result := &CheckResult{
		Name:      "test",
		Category:  CategoryDisk,
		Status:    StatusHealthy,
		Message:   "all good",
		Details:   map[string]interface{}{"key": "value"},
		Timestamp: now,
		Duration:  100 * time.Millisecond,
	}

	assert.Equal(t, "test", result.Name)
	assert.Equal(t, CategoryDisk, result.Category)
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Equal(t, "all good", result.Message)
	assert.Equal(t, "value", result.Details["key"])
}

func TestHealResult_Fields(t *testing.T) {
	r := &HealResult{
		Success:       true,
		Action:        "restart",
		Message:       "restarted service",
		NeedsApproval: false,
	}

	assert.True(t, r.Success)
	assert.Equal(t, "restart", r.Action)
}

func TestStrategyConfig_DefaultValues(t *testing.T) {
	cfg := &StrategyConfig{
		DefaultAction: HealActionNone,
		Overrides:     make(map[string]HealAction),
		CheckInterval: 30 * time.Minute,
		Enabled:       true,
	}

	assert.Equal(t, HealActionNone, cfg.DefaultAction)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 30*time.Minute, cfg.CheckInterval)
}

// ========== SMART Checker Tests ==========

func TestSMARTChecker_Interface(t *testing.T) {
	var _ Checker = &SMARTChecker{}

	c := NewSMARTChecker("/dev/sda")
	assert.Equal(t, "disk_smart", c.Name())
	assert.Equal(t, CategoryDisk, c.Category())
	assert.Equal(t, HealActionNone, c.HealAction())
	assert.NotEmpty(t, c.Description())
}

func TestSMARTChecker_Heal(t *testing.T) {
	c := NewSMARTChecker()
	result := &CheckResult{
		Name:     "disk_smart",
		Category: CategoryDisk,
		Status:   StatusUnhealthy,
	}
	healResult := c.Heal(&CheckContext{}, result)
	assert.False(t, healResult.Success)
	assert.Contains(t, healResult.Message, "无法自动修复")
}

// ========== Filesystem Checker Tests ==========

func TestFilesystemChecker_Interface(t *testing.T) {
	var _ Checker = &FilesystemChecker{}

	c := NewFilesystemChecker("/")
	assert.Equal(t, "filesystem_consistency", c.Name())
	assert.Equal(t, CategoryFilesystem, c.Category())
	assert.Equal(t, HealActionNone, c.HealAction())
}

func TestFilesystemChecker_Heal(t *testing.T) {
	c := NewFilesystemChecker()
	healResult := c.Heal(&CheckContext{}, &CheckResult{})
	assert.False(t, healResult.Success)
	assert.True(t, healResult.NeedsApproval)
}

// ========== Service Checker Tests ==========

func TestServiceChecker_Interface(t *testing.T) {
	var _ Checker = &ServiceChecker{}

	c := NewServiceChecker("sshd")
	assert.Equal(t, "service_liveness", c.Name())
	assert.Equal(t, CategoryService, c.Category())
	assert.Equal(t, HealActionAuto, c.HealAction())
}

func TestServiceChecker_DefaultServices(t *testing.T) {
	c := NewServiceChecker()
	assert.Len(t, c.services, 5)
	assert.Contains(t, c.services, "sshd")
}

func TestServiceChecker_Check(t *testing.T) {
	c := NewServiceChecker("sshd")
	result := c.Check(&CheckContext{})

	assert.Equal(t, "service_liveness", result.Name)
	assert.NotEmpty(t, result.Message)
	// sshd 在 CI 环境中可能运行也可能不运行，只验证结构
	assert.Contains(t, result.Details, "services")
}

// ========== Config Checker Tests ==========

func TestConfigChecker_Interface(t *testing.T) {
	var _ Checker = &ConfigChecker{}

	c := NewConfigChecker("/etc/hostname")
	assert.Equal(t, "config_integrity", c.Name())
	assert.Equal(t, CategoryConfig, c.Category())
}

func TestConfigChecker_Heal(t *testing.T) {
	c := NewConfigChecker()
	healResult := c.Heal(&CheckContext{}, &CheckResult{})
	assert.True(t, healResult.NeedsApproval)
}

func TestConfigChecker_Check_NonExistent(t *testing.T) {
	c := NewConfigChecker("/nonexistent/path/config.yaml")
	result := c.Check(&CheckContext{})

	assert.Equal(t, StatusDegraded, result.Status)
	assert.Contains(t, result.Message, "配置文件问题")
}

// ========== Cert Checker Tests ==========

func TestCertChecker_Interface(t *testing.T) {
	var _ Checker = &CertChecker{}

	c := NewCertChecker(nil, nil, 30)
	assert.Equal(t, "cert_expiry", c.Name())
	assert.Equal(t, CategoryCert, c.Category())
	assert.Equal(t, 30, c.warnDays)
}

func TestCertChecker_DefaultWarnDays(t *testing.T) {
	c := NewCertChecker(nil, nil, 0)
	assert.Equal(t, 30, c.warnDays)
}

func TestCertChecker_Heal(t *testing.T) {
	c := NewCertChecker(nil, nil, 30)
	healResult := c.Heal(&CheckContext{}, &CheckResult{})
	assert.True(t, healResult.NeedsApproval)
}

func TestCertChecker_Check_NoCerts(t *testing.T) {
	c := NewCertChecker(nil, nil, 30)
	result := c.Check(&CheckContext{})

	assert.Equal(t, StatusHealthy, result.Status)
	assert.Contains(t, result.Message, "全部 0 个证书有效")
}

func TestCertChecker_Check_LocalCert_Missing(t *testing.T) {
	c := NewCertChecker([]string{"/nonexistent/cert.pem"}, nil, 30)
	result := c.Check(&CheckContext{})

	// 证书文件不存在会报 error
	assert.Equal(t, StatusDegraded, result.Status)
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

func TestHandlers_GetStatus_Initial(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/self-heal/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 无 checker 时返回 healthy
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
}

func TestHandlers_ListChecks(t *testing.T) {
	r, m := setupHandlers(t)

	m.RegisterChecker(newMockChecker("check1", StatusHealthy))
	m.RegisterChecker(newMockChecker("check2", StatusDegraded))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/self-heal/checks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "check1")
	assert.Contains(t, w.Body.String(), "check2")
}

func TestHandlers_RunChecks_All(t *testing.T) {
	r, m := setupHandlers(t)

	m.RegisterChecker(newMockChecker("run1", StatusHealthy))
	m.RegisterChecker(newMockChecker("run2", StatusHealthy))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/self-heal/run", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
}

func TestHandlers_RunChecks_Single(t *testing.T) {
	r, m := setupHandlers(t)

	m.RegisterChecker(newMockChecker("target_check", StatusHealthy))

	body := `{"name":"target_check"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/self-heal/run", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "target_check")
}

func TestHandlers_RunChecks_Single_NotFound(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `{"name":"nonexistent"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/self-heal/run", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_GetConfig(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/self-heal/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "default_action")
}

func TestHandlers_UpdateConfig(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `{"default_action":"auto","check_interval":600000000000,"enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/self-heal/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "config updated")
}

func TestHandlers_UpdateConfig_Invalid(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `invalid json`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/self-heal/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_GetHistory_NoStore(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/self-heal/history", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 无 store 时返回错误
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandlers_GetStatus_Unhealthy(t *testing.T) {
	r, m := setupHandlers(t)

	m.RegisterChecker(newMockChecker("bad", StatusUnhealthy))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/self-heal/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandlers_RunChecks_EmptyBody(t *testing.T) {
	r, m := setupHandlers(t)
	m.RegisterChecker(newMockChecker("test", StatusHealthy))

	// POST 无 body（或空 JSON）应执行全部检查
	req := httptest.NewRequest(http.MethodPost, "/api/v1/self-heal/run", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ========== Store Tests ==========

func TestStore_Init(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// 验证表已创建
	var tableName string
	err := store.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='self_heal_records'").Scan(&tableName)
	require.NoError(t, err)
	assert.Equal(t, "self_heal_records", tableName)
}

func TestStore_SaveAndQuery(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// 保存记录
	result := &CheckResult{
		Name:      "test_check",
		Category:  CategoryDisk,
		Status:    StatusHealthy,
		Message:   "all good",
		Details:   map[string]interface{}{"key": "value"},
		Timestamp: time.Now(),
	}
	err := store.SaveRecord(result, HealActionNone)
	require.NoError(t, err)

	// 查询历史
	records, err := store.GetHistory(10)
	require.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, "test_check", records[0].CheckName)
	assert.Equal(t, "healthy", records[0].Status)
}

func TestStore_UpdateHealResult(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// 先保存记录
	result := &CheckResult{
		Name:      "heal_test",
		Category:  CategoryService,
		Status:    StatusUnhealthy,
		Message:   "service down",
		Timestamp: time.Now(),
	}
	err := store.SaveRecord(result, HealActionAuto)
	require.NoError(t, err)

	// 更新修复结果
	healResult := &HealResult{
		Success: true,
		Action:  "restart",
		Message: "restarted successfully",
	}
	err = store.UpdateHealResult("heal_test", healResult)
	require.NoError(t, err)

	// 验证更新
	records, err := store.GetHistory(10)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.NotNil(t, records[0].HealSuccess)
	assert.True(t, *records[0].HealSuccess)
	assert.Equal(t, "restarted successfully", records[0].HealMessage)
}

func TestStore_GetHistoryByCheck(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// 保存多条记录
	for i := 0; i < 5; i++ {
		store.SaveRecord(&CheckResult{
			Name:      "check_a",
			Category:  CategoryDisk,
			Status:    StatusHealthy,
			Message:   "ok",
			Timestamp: time.Now(),
		}, HealActionNone)
	}
	store.SaveRecord(&CheckResult{
		Name:      "check_b",
		Category:  CategoryService,
		Status:    StatusDegraded,
		Message:   "warn",
		Timestamp: time.Now(),
	}, HealActionNone)

	// 查询 check_a 的历史
	records, err := store.GetHistoryByCheck("check_a", 10)
	require.NoError(t, err)
	assert.Len(t, records, 5)

	// 查询 check_b 的历史
	records, err = store.GetHistoryByCheck("check_b", 10)
	require.NoError(t, err)
	assert.Len(t, records, 1)
}

func TestStore_Cleanup(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// 插入一条旧记录
	_, err := store.db.Exec(`
		INSERT INTO self_heal_records (check_name, category, status, message, heal_action, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "old_check", "disk", "healthy", "ok", "none",
		time.Now().Add(-48*time.Hour).Format(time.RFC3339))
	require.NoError(t, err)

	// 插入一条新记录
	store.SaveRecord(&CheckResult{
		Name:      "new_check",
		Category:  CategoryDisk,
		Status:    StatusHealthy,
		Message:   "ok",
		Timestamp: time.Now(),
	}, HealActionNone)

	// 清理 24 小时前的记录
	err = store.Cleanup(24 * time.Hour)
	require.NoError(t, err)

	// 应该只剩新记录
	records, err := store.GetHistory(100)
	require.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, "new_check", records[0].CheckName)
}

func TestStore_GetHistory_Limit(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	for i := 0; i < 20; i++ {
		store.SaveRecord(&CheckResult{
			Name:      "limit_test",
			Category:  CategoryCustom,
			Status:    StatusHealthy,
			Message:   "ok",
			Timestamp: time.Now(),
		}, HealActionNone)
	}

	records, err := store.GetHistory(5)
	require.NoError(t, err)
	assert.Len(t, records, 5)
}

func TestStore_GetHistory_DefaultLimit(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	records, err := store.GetHistory(0)
	require.NoError(t, err)
	assert.Len(t, records, 0) // 空表返回空结果
}

// ========== Integration Test ==========

func TestIntegration_FullFlow(t *testing.T) {
	m := NewManager(nil, nil)

	// 注册多种检查器
	m.RegisterChecker(newMockChecker("healthy_check", StatusHealthy))
	m.RegisterChecker(newMockChecker("degraded_check", StatusDegraded))
	m.RegisterChecker(newMockChecker("unhealthy_check", StatusUnhealthy))

	// 配置自动修复
	m.UpdateConfig(&StrategyConfig{
		DefaultAction: HealActionAuto,
	})

	// 执行所有检查
	status := m.RunAll(context.Background())

	assert.Equal(t, StatusUnhealthy, status.Status)
	assert.Equal(t, 3, status.Summary.Total)
	assert.Equal(t, 1, status.Summary.Healthy)
	assert.Equal(t, 1, status.Summary.Degraded)
	assert.Equal(t, 1, status.Summary.Unhealthy)

	// 验证列表
	list := m.ListCheckers()
	assert.Len(t, list, 3)

	// 验证配置
	cfg := m.GetConfig()
	assert.Equal(t, HealActionAuto, cfg.DefaultAction)

	// 验证单个执行
	result, err := m.RunSingle(context.Background(), "healthy_check")
	require.NoError(t, err)
	assert.Equal(t, StatusHealthy, result.Status)
}

// ========== Benchmark ==========

func BenchmarkManager_RunAll(b *testing.B) {
	m := NewManager(nil, nil)
	for i := 0; i < 10; i++ {
		m.RegisterChecker(newMockChecker("bench_check", StatusHealthy))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RunAll(context.Background())
	}
}

// ========== Test Helpers ==========

// setupTestStore 创建临时 SQLite 测试存储.
func setupTestStore(t *testing.T) (*Store, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "selfheal_test_*.db")
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
