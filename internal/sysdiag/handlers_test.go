// Package sysdiag 单元测试
package sysdiag

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== Manager Tests ==========

func TestNewManager(t *testing.T) {
	m := NewManager()
	assert.NotNil(t, m)
	assert.NotNil(t, m.healthItems)
	assert.Nil(t, m.lastTask)
	assert.Nil(t, m.lastReport)
}

func TestManager_RunDiagnostics(t *testing.T) {
	m := NewManager()

	task := m.RunDiagnostics()

	require.NotNil(t, task)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "系统全面诊断", task.Name)
	assert.Equal(t, DiagStatusPass, task.Status)
	assert.False(t, task.StartTime.IsZero())
	assert.False(t, task.EndTime.IsZero())
	assert.True(t, task.EndTime.After(task.StartTime))
	assert.NotEmpty(t, task.Results)

	// 验证各类检查都有结果
	categories := make(map[DiagCategory]bool)
	for _, r := range task.Results {
		categories[r.Category] = true
	}
	assert.True(t, categories[CategoryHardware])
	assert.True(t, categories[CategoryStorage])
	assert.True(t, categories[CategoryFilesystem])
	assert.True(t, categories[CategoryNetwork])
	assert.True(t, categories[CategoryService])
	assert.True(t, categories[CategoryPerformance])
}

func TestManager_GetLastTask(t *testing.T) {
	m := NewManager()

	// 初始为空
	assert.Nil(t, m.GetLastTask())

	// 运行诊断后有结果
	task := m.RunDiagnostics()
	lastTask := m.GetLastTask()
	require.NotNil(t, lastTask)
	assert.Equal(t, task.ID, lastTask.ID)
}

func TestManager_GetLastReport(t *testing.T) {
	m := NewManager()

	// 初始为空
	assert.Nil(t, m.GetLastReport())

	// 运行诊断后有报告
	m.RunDiagnostics()
	report := m.GetLastReport()
	require.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.NotNil(t, report.Summary)
	assert.NotEmpty(t, report.Results)
	assert.NotEmpty(t, report.HealthItems)
}

func TestManager_GetHealthStatus(t *testing.T) {
	m := NewManager()

	// 运行诊断以初始化健康检查项
	m.RunDiagnostics()

	health := m.GetHealthStatus()
	assert.NotNil(t, health)

	// 验证有基本的健康检查项
	assert.Contains(t, health, "cpu_temp")
	assert.Contains(t, health, "memory_usage")
	assert.Contains(t, health, "disk_usage")
	assert.Contains(t, health, "network_status")

	// 验证状态都是 pass
	for _, item := range health {
		assert.Equal(t, DiagStatusPass, item.Status)
	}
}

// ========== Handlers Tests ==========

func setupHandlers(t *testing.T) (*gin.Engine, *Manager) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	m := NewManager()
	h := NewHandlers(m)

	r := gin.New()
	api := r.Group("/api")
	h.RegisterRoutes(api)

	return r, m
}

func TestHandlers_RunDiagnostics(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/api/sysdiag/run", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "diagnostics completed")
	assert.Contains(t, w.Body.String(), "系统全面诊断")
}

func TestHandlers_GetResults_Initial(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sysdiag/results", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 未运行过诊断时返回 404
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_GetResults_AfterRun(t *testing.T) {
	r, m := setupHandlers(t)

	// 先运行诊断
	m.RunDiagnostics()

	req := httptest.NewRequest(http.MethodGet, "/api/sysdiag/results", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "系统全面诊断")
}

func TestHandlers_GetHealth(t *testing.T) {
	r, m := setupHandlers(t)

	// 运行诊断以初始化健康检查项
	m.RunDiagnostics()

	req := httptest.NewRequest(http.MethodGet, "/api/sysdiag/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "pass")
	assert.Contains(t, w.Body.String(), "cpu_temp")
	assert.Contains(t, w.Body.String(), "memory_usage")
}

func TestHandlers_GetReport_Initial(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sysdiag/report", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 未运行过诊断时返回 404
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_GetReport_AfterRun(t *testing.T) {
	r, m := setupHandlers(t)

	// 先运行诊断
	m.RunDiagnostics()

	req := httptest.NewRequest(http.MethodGet, "/api/sysdiag/report", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "summary")
	assert.Contains(t, w.Body.String(), "total_checks")
}

// ========== Types Tests ==========

func TestTypes_Constants(t *testing.T) {
	assert.Equal(t, DiagStatus("pass"), DiagStatusPass)
	assert.Equal(t, DiagStatus("warn"), DiagStatusWarn)
	assert.Equal(t, DiagStatus("fail"), DiagStatusFail)
	assert.Equal(t, DiagStatus("running"), DiagStatusRunning)
	assert.Equal(t, DiagStatus("pending"), DiagStatusPending)

	assert.Equal(t, DiagCategory("hardware"), CategoryHardware)
	assert.Equal(t, DiagCategory("storage"), CategoryStorage)
	assert.Equal(t, DiagCategory("filesystem"), CategoryFilesystem)
	assert.Equal(t, DiagCategory("network"), CategoryNetwork)
	assert.Equal(t, DiagCategory("service"), CategoryService)
	assert.Equal(t, DiagCategory("performance"), CategoryPerformance)

	assert.Equal(t, Severity("low"), SeverityLow)
	assert.Equal(t, Severity("medium"), SeverityMedium)
	assert.Equal(t, Severity("high"), SeverityHigh)
	assert.Equal(t, Severity("critical"), SeverityCritical)
}

// ========== Integration Test ==========

func TestIntegration_FullFlow(t *testing.T) {
	m := NewManager()

	// 运行诊断
	task := m.RunDiagnostics()
	require.NotNil(t, task)
	assert.Equal(t, DiagStatusPass, task.Status)

	// 获取报告
	report := m.GetLastReport()
	require.NotNil(t, report)
	assert.Equal(t, task.ID, report.TaskID)
	assert.True(t, report.Summary.Passed > 0)

	// 获取健康状态
	health := m.GetHealthStatus()
	assert.NotEmpty(t, health)

	// 再次运行诊断（验证幂等性）
	task2 := m.RunDiagnostics()
	require.NotNil(t, task2)
	assert.NotEqual(t, task.ID, task2.ID)
}
