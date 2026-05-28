// Package powermanager 单元测试
package powermanager

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== Manager Tests ==========

func TestNewManager(t *testing.T) {
	m := NewManager()
	assert.NotNil(t, m)
	assert.NotNil(t, m.currentPlan)
	assert.NotNil(t, m.schedules)
	assert.NotNil(t, m.upsInfo)
	assert.Equal(t, PowerPlanBalanced, m.currentPlan.Plan)
}

func TestManager_GetPlans(t *testing.T) {
	m := NewManager()
	plans := m.GetPlans()

	assert.Len(t, plans, 3)

	// 验证三种计划都存在
	planTypes := make(map[PowerPlan]bool)
	for _, p := range plans {
		planTypes[p.Plan] = true
	}
	assert.True(t, planTypes[PowerPlanHighPerf])
	assert.True(t, planTypes[PowerPlanBalanced])
	assert.True(t, planTypes[PowerPlanPowerSave])
}

func TestManager_SetPlan(t *testing.T) {
	m := NewManager()

	// 设置高性能计划
	err := m.SetPlan(PowerPlanHighPerf)
	require.NoError(t, err)

	plan := m.GetCurrentPlan()
	assert.Equal(t, PowerPlanHighPerf, plan.Plan)
	assert.Equal(t, "performance", plan.CPUGovernor)
	assert.Equal(t, 0, plan.HDDStandby)
	assert.True(t, plan.WoLEnabled)

	// 设置节能计划
	err = m.SetPlan(PowerPlanPowerSave)
	require.NoError(t, err)

	plan = m.GetCurrentPlan()
	assert.Equal(t, PowerPlanPowerSave, plan.Plan)
	assert.Equal(t, "powersave", plan.CPUGovernor)
	assert.False(t, plan.WoLEnabled)
}

func TestManager_SetPlan_Invalid(t *testing.T) {
	m := NewManager()

	err := m.SetPlan("invalid_plan")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown power plan")
}

func TestManager_Schedules(t *testing.T) {
	m := NewManager()

	// 添加定时任务
	schedule := &PowerSchedule{
		Name:    "daily_shutdown",
		Action:  "power_off",
		Time:    "23:00",
		Days:    []string{"mon", "tue", "wed", "thu", "fri"},
		Enabled: true,
	}

	err := m.AddSchedule(schedule)
	require.NoError(t, err)
	assert.NotEmpty(t, schedule.ID)

	// 获取定时任务列表
	schedules := m.GetSchedules()
	assert.Len(t, schedules, 1)
	assert.Equal(t, "daily_shutdown", schedules[0].Name)

	// 删除定时任务
	err = m.RemoveSchedule(schedule.ID)
	require.NoError(t, err)

	schedules = m.GetSchedules()
	assert.Len(t, schedules, 0)
}

func TestManager_RemoveSchedule_NotFound(t *testing.T) {
	m := NewManager()

	err := m.RemoveSchedule("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_GetUPSStatus(t *testing.T) {
	m := NewManager()

	ups := m.GetUPSStatus()
	assert.NotNil(t, ups)
	assert.Equal(t, UPSStatusUnknown, ups.Status)
}

func TestManager_GetConsumptionStats(t *testing.T) {
	m := NewManager()

	stats := m.GetConsumptionStats()
	assert.NotNil(t, stats)
	assert.Nil(t, stats.Current) // 初始无数据
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

func TestHandlers_GetPlans(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/power/plans", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "high_performance")
	assert.Contains(t, w.Body.String(), "balanced")
	assert.Contains(t, w.Body.String(), "power_save")
}

func TestHandlers_SetPlan(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `{"plan":"high_performance"}`
	req := httptest.NewRequest(http.MethodPost, "/api/power/plan", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "power plan updated")
}

func TestHandlers_SetPlan_Invalid(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `{"plan":"invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/power/plan", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_GetSchedules(t *testing.T) {
	r, m := setupHandlers(t)

	// 添加一个定时任务
	m.AddSchedule(&PowerSchedule{
		Name:    "test_schedule",
		Action:  "power_off",
		Time:    "22:00",
		Days:    []string{"mon"},
		Enabled: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/power/schedules", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test_schedule")
}

func TestHandlers_AddSchedule(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `{"name":"daily_power_off","action":"power_off","time":"23:00","days":["mon","tue"],"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/power/schedule", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "schedule added")
}

func TestHandlers_RemoveSchedule(t *testing.T) {
	r, m := setupHandlers(t)

	// 先添加一个定时任务
	schedule := &PowerSchedule{
		Name:    "to_remove",
		Action:  "power_off",
		Time:    "22:00",
		Days:    []string{"mon"},
		Enabled: true,
	}
	m.AddSchedule(schedule)

	req := httptest.NewRequest(http.MethodDelete, "/api/power/schedule/"+schedule.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "schedule removed")
}

func TestHandlers_RemoveSchedule_NotFound(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/power/schedule/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_GetUPSStatus(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/power/ups", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "unknown")
}

func TestHandlers_GetConsumption(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/power/consumption", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_SendWoL(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `{"mac_address":"00:11:22:33:44:55"}`
	req := httptest.NewRequest(http.MethodPost, "/api/power/wake", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "WoL packet sent")
}

func TestHandlers_SendWoL_InvalidMAC(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `{"mac_address":"invalid_mac"}`
	req := httptest.NewRequest(http.MethodPost, "/api/power/wake", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandlers_SendWoL_MissingMAC(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/power/wake", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ========== Types Tests ==========

func TestTypes_Constants(t *testing.T) {
	assert.Equal(t, PowerPlan("high_performance"), PowerPlanHighPerf)
	assert.Equal(t, PowerPlan("balanced"), PowerPlanBalanced)
	assert.Equal(t, PowerPlan("power_save"), PowerPlanPowerSave)

	assert.Equal(t, UPSStatus("online"), UPSStatusOnline)
	assert.Equal(t, UPSStatus("battery"), UPSStatusBattery)
	assert.Equal(t, UPSStatus("low"), UPSStatusLow)
	assert.Equal(t, UPSStatus("critical"), UPSStatusCritical)
	assert.Equal(t, UPSStatus("unknown"), UPSStatusUnknown)
}
