// Package powermanager 单元测试
package powermanager

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	assert.NotNil(t, m.disks)
	assert.NotNil(t, m.cpuInfo)
	assert.Equal(t, PowerPlanBalanced, m.currentPlan.Plan)
	assert.Equal(t, 30, m.diskIdleMin)
	assert.False(t, m.running)
}

func TestManager_StartStop(t *testing.T) {
	m := NewManager()

	assert.False(t, m.IsRunning())

	m.Start()
	assert.True(t, m.IsRunning())

	// 重复启动不应出错
	m.Start()
	assert.True(t, m.IsRunning())

	m.Stop()
	assert.False(t, m.IsRunning())

	// 重复停止不应出错
	m.Stop()
	assert.False(t, m.IsRunning())
}

func TestManager_GetPlans(t *testing.T) {
	m := NewManager()
	plans := m.GetPlans()

	assert.Len(t, plans, 3)

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

	// 设置均衡计划
	err = m.SetPlan(PowerPlanBalanced)
	require.NoError(t, err)

	plan = m.GetCurrentPlan()
	assert.Equal(t, PowerPlanBalanced, plan.Plan)
	assert.Equal(t, "ondemand", plan.CPUGovernor)

	// 设置节能计划
	err = m.SetPlan(PowerPlanPowerSave)
	require.NoError(t, err)

	plan = m.GetCurrentPlan()
	assert.Equal(t, PowerPlanPowerSave, plan.Plan)
	assert.Equal(t, "powersave", plan.CPUGovernor)
	assert.Equal(t, 10, plan.HDDStandby)
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

func TestManager_ScheduleWithCron(t *testing.T) {
	m := NewManager()

	// 添加 cron 定时任务
	schedule := &PowerSchedule{
		Name:     "cron_shutdown",
		Action:   "power_off",
		CronExpr: "0 23 * * *",
		Enabled:  true,
	}

	err := m.AddSchedule(schedule)
	require.NoError(t, err)
	assert.NotEmpty(t, schedule.ID)

	schedules := m.GetSchedules()
	assert.Len(t, schedules, 1)
	assert.Equal(t, "0 23 * * *", schedules[0].CronExpr)
}

func TestManager_ScheduleWithInvalidCron(t *testing.T) {
	m := NewManager()

	schedule := &PowerSchedule{
		Name:     "bad_cron",
		Action:   "power_off",
		CronExpr: "invalid",
		Enabled:  true,
	}

	err := m.AddSchedule(schedule)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid cron expression")
}

// ========== Disk Tests ==========

func TestManager_RegisterDisk(t *testing.T) {
	m := NewManager()

	m.RegisterDisk("/dev/sda", "Samsung SSD 870")

	disks := m.GetDiskStatus()
	assert.Len(t, disks, 1)
	assert.Equal(t, "/dev/sda", disks[0].Device)
	assert.Equal(t, "Samsung SSD 870", disks[0].Model)
	assert.Equal(t, DiskStateActive, disks[0].State)
}

func TestManager_HibernateDisk(t *testing.T) {
	m := NewManager()

	m.RegisterDisk("/dev/sda", "Test Disk")

	err := m.HibernateDisk("/dev/sda")
	require.NoError(t, err)

	disks := m.GetDiskStatus()
	assert.Len(t, disks, 1)
	assert.Equal(t, DiskStateStandby, disks[0].State)
	assert.False(t, disks[0].IdleSince.IsZero())
}

func TestManager_WakeDisk(t *testing.T) {
	m := NewManager()

	m.RegisterDisk("/dev/sda", "Test Disk")

	err := m.HibernateDisk("/dev/sda")
	require.NoError(t, err)

	err = m.WakeDisk("/dev/sda")
	require.NoError(t, err)

	disks := m.GetDiskStatus()
	assert.Len(t, disks, 1)
	assert.Equal(t, DiskStateActive, disks[0].State)
}

func TestManager_HibernateDisk_NotFound(t *testing.T) {
	m := NewManager()

	err := m.HibernateDisk("/dev/nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_WakeDisk_NotFound(t *testing.T) {
	m := NewManager()

	err := m.WakeDisk("/dev/nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ========== CPU Tests ==========

func TestManager_GetCPUInfo(t *testing.T) {
	m := NewManager()

	info := m.GetCPUInfo()
	assert.NotNil(t, info)
	assert.Equal(t, "ondemand", info.Governor)
	assert.Equal(t, 4, info.CoreCount)
	assert.Equal(t, 800, info.MinFreq)
	assert.Equal(t, 3000, info.MaxFreq)
}

func TestManager_SetCPUGovernor(t *testing.T) {
	m := NewManager()

	validGovernors := []string{"performance", "powersave", "ondemand", "conservative", "schedutil"}
	for _, gov := range validGovernors {
		err := m.SetCPUGovernor(gov)
		require.NoError(t, err)
		assert.Equal(t, gov, m.GetCPUInfo().Governor)
	}
}

func TestManager_SetCPUGovernor_Invalid(t *testing.T) {
	m := NewManager()

	err := m.SetCPUGovernor("invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid CPU governor")
}

func TestManager_SetCPUFrequency(t *testing.T) {
	m := NewManager()

	err := m.SetCPUFrequency(2000)
	require.NoError(t, err)
	assert.Equal(t, 2000, m.GetCPUInfo().CurrentFreq)
	assert.Equal(t, "userspace", m.GetCPUInfo().Governor)
}

func TestManager_SetCPUFrequency_OutOfRange(t *testing.T) {
	m := NewManager()

	err := m.SetCPUFrequency(100) // 低于最小值
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")

	err = m.SetCPUFrequency(5000) // 高于最大值
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

// ========== UPS & Consumption Tests ==========

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
	assert.Nil(t, stats.Current)
	assert.Equal(t, float64(0), stats.Average24h)
}

func TestManager_GetConsumptionHistory(t *testing.T) {
	m := NewManager()

	history := m.GetConsumptionHistory()
	assert.NotNil(t, history)
	assert.Len(t, history, 0)
}

// ========== Cron Tests ==========

func TestParseCronExpr(t *testing.T) {
	tests := []struct {
		expr    string
		wantErr bool
	}{
		{"* * * * *", false},
		{"0 23 * * *", false},
		{"*/5 * * * *", false},
		{"0 9 * * 1-5", false},
		{"invalid", true},
		{"* * * *", true},      // 只有4个字段
		{"* * * * * *", true},  // 6个字段
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			parts, err := ParseCronExpr(tt.expr)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, parts, 5)
			}
		})
	}
}

func TestMatchCron(t *testing.T) {
	// 2024-01-15 14:30:00 Monday
	testTime := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		expr  string
		match bool
	}{
		{"* * * * *", true},
		{"30 14 * * *", true},
		{"31 14 * * *", false},
		{"*/5 * * * *", true},   // 30 % 5 == 0
		{"*/7 * * * *", false},  // 30 % 7 != 0
		{"0 14 * * *", false},   // 分钟不匹配
		{"30 14 * * 1", true},   // 周一
		{"30 14 * * 2", false},  // 不是周二
		{"30 14 15 * *", true},  // 15号
		{"30 14 16 * *", false}, // 不是16号
		{"30 14 * 1 *", true},   // 1月
		{"30 14 1-15 * *", true}, // 范围
		{"30 14 16-31 * *", false}, // 范围外
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result := MatchCron(tt.expr, testTime)
			assert.Equal(t, tt.match, result, "expr: %s", tt.expr)
		})
	}
}

func TestMatchCron_StepValues(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)

	assert.True(t, MatchCron("*/10 * * * *", testTime))  // 30 % 10 == 0
	assert.True(t, MatchCron("*/15 * * * *", testTime))   // 30 % 15 == 0
	assert.True(t, MatchCron("*/30 * * * *", testTime))   // 30 % 30 == 0
}

func TestSplitFields(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"* * * * *", 5},
		{"a  b  c  d  e", 5},
		{"single", 1},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitFields(tt.input)
			assert.Len(t, result, tt.want)
		})
	}
}

func TestSplitComma(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"1,2,3", 3},
		{"1", 1},
		{"", 0},
		{"a,b", 2},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitComma(tt.input)
			assert.Len(t, result, tt.want)
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"0", 0},
		{"123", 123},
		{"abc", -1},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, parseInt(tt.input))
		})
	}
}

// ========== Types Constants Tests ==========

func TestTypes_Constants(t *testing.T) {
	assert.Equal(t, PowerPlan("high_performance"), PowerPlanHighPerf)
	assert.Equal(t, PowerPlan("balanced"), PowerPlanBalanced)
	assert.Equal(t, PowerPlan("power_save"), PowerPlanPowerSave)

	assert.Equal(t, UPSStatus("online"), UPSStatusOnline)
	assert.Equal(t, UPSStatus("battery"), UPSStatusBattery)
	assert.Equal(t, UPSStatus("low"), UPSStatusLow)
	assert.Equal(t, UPSStatus("critical"), UPSStatusCritical)
	assert.Equal(t, UPSStatus("unknown"), UPSStatusUnknown)

	assert.Equal(t, DiskState("active"), DiskStateActive)
	assert.Equal(t, DiskState("idle"), DiskStateIdle)
	assert.Equal(t, DiskState("standby"), DiskStateStandby)
}

// ========== Handler Tests ==========

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

func TestHandlers_SetPlan_BadJSON(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/api/power/plan", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_GetSchedules(t *testing.T) {
	r, m := setupHandlers(t)

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

func TestHandlers_AddSchedule_WithCron(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `{"name":"cron_job","action":"power_off","cron_expr":"0 23 * * *","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/power/schedule", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "schedule added")
}

func TestHandlers_AddSchedule_InvalidCron(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `{"name":"bad_cron","action":"power_off","cron_expr":"invalid","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/power/schedule", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid cron expression")
}

func TestHandlers_RemoveSchedule(t *testing.T) {
	r, m := setupHandlers(t)

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

func TestHandlers_GetConsumptionHistory(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/power/consumption/history", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "history")
}

// ========== Disk Handler Tests ==========

func TestHandlers_GetDiskStatus(t *testing.T) {
	r, m := setupHandlers(t)

	m.RegisterDisk("/dev/sda", "Test Disk")

	req := httptest.NewRequest(http.MethodGet, "/api/power/disks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "sda")
	assert.Contains(t, w.Body.String(), "Test Disk")
}

func TestHandlers_HibernateDisk(t *testing.T) {
	r, m := setupHandlers(t)

	m.RegisterDisk("/dev/sda", "Test Disk")

	req := httptest.NewRequest(http.MethodPost, "/api/power/disk/hibernate/sda", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "disk hibernated")
}

func TestHandlers_WakeDisk(t *testing.T) {
	r, m := setupHandlers(t)

	m.RegisterDisk("/dev/sda", "Test Disk")
	m.HibernateDisk("/dev/sda")

	req := httptest.NewRequest(http.MethodPost, "/api/power/disk/wake/sda", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "disk woken up")
}

func TestHandlers_HibernateDisk_NotFound(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/api/power/disk/hibernate/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ========== CPU Handler Tests ==========

func TestHandlers_GetCPUInfo(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/power/cpu", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "governor")
	assert.Contains(t, w.Body.String(), "current_freq")
}

func TestHandlers_SetCPUGovernor(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `{"governor":"performance"}`
	req := httptest.NewRequest(http.MethodPost, "/api/power/cpu/governor", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CPU governor updated")
}

func TestHandlers_SetCPUGovernor_Invalid(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `{"governor":"invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/power/cpu/governor", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_SetCPUFrequency(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `{"frequency":2000}`
	req := httptest.NewRequest(http.MethodPost, "/api/power/cpu/frequency", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CPU frequency updated")
}

func TestHandlers_SetCPUFrequency_OutOfRange(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `{"frequency":100}`
	req := httptest.NewRequest(http.MethodPost, "/api/power/cpu/frequency", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ========== WoL Handler Tests ==========

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
