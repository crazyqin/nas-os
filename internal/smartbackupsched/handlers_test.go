package smartbackupsched

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestManager(t *testing.T) (*Manager, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "configs.json")
	auditPath := filepath.Join(tmpDir, "audit.json")
	mgr := NewManager(configPath, auditPath)
	require.NoError(t, mgr.Initialize())
	return mgr, func() {}
}

func setupTestRouter(mgr *Manager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	grp := r.Group("")
	h := NewHandlers(mgr)
	h.RegisterRoutes(grp)
	return r
}

func createTestConfig(t *testing.T, router *gin.Engine, name string) ScheduleConfig {
	t.Helper()
	config := ScheduleConfig{
		Name:       name,
		SourcePath: "/data/test",
		Strategy:   StrategyAuto,
		TargetPaths: []TargetPath{
			{Tier: TierLocal, Path: "/backup/local", Priority: 1, Enabled: true},
			{Tier: TierCloud, Path: "/backup/cloud", Priority: 2, Enabled: true},
		},
		BackupWindows: []BackupWindow{
			{Name: "夜间窗口", StartTime: "22:00", EndTime: "06:00", Days: []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}},
		},
		PeakHours:    []TimeRange{{Start: "09:00", End: "18:00"}},
		MaxRetries:   3,
		RetentionDays: 30,
	}
	body, _ := json.Marshal(config)
	req := httptest.NewRequest(http.MethodPost, "/smart-backup-sched/configs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	return config
}

// TestListConfigs 测试列出配置.
func TestListConfigs(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()
	router := setupTestRouter(mgr)

	// 初始应为空
	req := httptest.NewRequest(http.MethodGet, "/smart-backup-sched/configs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(0), resp["code"])

	// 创建一个配置后再列出
	createTestConfig(t, router, "测试配置1")

	req = httptest.NewRequest(http.MethodGet, "/smart-backup-sched/configs", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].([]interface{})
	assert.Len(t, data, 1)
}

// TestCreateAndGetConfig 测试创建和获取配置.
func TestCreateAndGetConfig(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()
	router := setupTestRouter(mgr)

	config := createTestConfig(t, router, "全量备份配置")

	// 从 manager 获取配置列表找到 ID
	configs := mgr.ListConfigs()
	require.Len(t, configs, 1)
	configID := configs[0].ID

	req := httptest.NewRequest(http.MethodGet, "/smart-backup-sched/configs/"+configID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, config.Name, data["name"])
	assert.Equal(t, "/data/test", data["sourcePath"])
}

// TestDeleteConfig 测试删除配置.
func TestDeleteConfig(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()
	router := setupTestRouter(mgr)

	createTestConfig(t, router, "待删除配置")

	configs := mgr.ListConfigs()
	require.Len(t, configs, 1)
	configID := configs[0].ID

	req := httptest.NewRequest(http.MethodDelete, "/smart-backup-sched/configs/"+configID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 确认已删除
	configs = mgr.ListConfigs()
	assert.Len(t, configs, 0)
}

// TestEnableConfig 测试启用/禁用配置.
func TestEnableConfig(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()
	router := setupTestRouter(mgr)

	createTestConfig(t, router, "开关测试")

	configs := mgr.ListConfigs()
	require.Len(t, configs, 1)
	configID := configs[0].ID

	// 启用
	body := `{"enabled": true}`
	req := httptest.NewRequest(http.MethodPost, "/smart-backup-sched/configs/"+configID+"/enable", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	cfg, err := mgr.GetConfig(configID)
	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
}

// TestRunBackup 测试手动触发备份.
func TestRunBackup(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()
	router := setupTestRouter(mgr)

	createTestConfig(t, router, "备份测试")

	configs := mgr.ListConfigs()
	require.Len(t, configs, 1)
	configID := configs[0].ID

	req := httptest.NewRequest(http.MethodPost, "/smart-backup-sched/configs/"+configID+"/run", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 验证任务已创建
	tasks := mgr.ListTasks()
	assert.GreaterOrEqual(t, len(tasks), 1)
}

// TestListAndGetTasks 测试任务列表和详情.
func TestListAndGetTasks(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()
	router := setupTestRouter(mgr)

	// 任务列表初始为空
	req := httptest.NewRequest(http.MethodGet, "/smart-backup-sched/tasks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].([]interface{})
	assert.Len(t, data, 0)
}

// TestRecommendStrategy 测试 AI 策略推荐.
func TestRecommendStrategy(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()
	router := setupTestRouter(mgr)

	createTestConfig(t, router, "推荐测试")

	configs := mgr.ListConfigs()
	require.Len(t, configs, 1)
	configID := configs[0].ID

	// 设置变更模式
	mgr.UpdateChangePattern(configID, &ChangePattern{
		Path:            "/data/test",
		ChangeFrequency: 0.5,
		ChangeRate:      0.02,
		AvgChangeSize:   1024 * 512,
		TotalChanges:    100,
	})

	req := httptest.NewRequest(http.MethodGet, "/smart-backup-sched/configs/"+configID+"/recommend", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.NotEmpty(t, data["recommended"])
	assert.NotNil(t, data["confidence"])
}

// TestAssessRisk 测试风险评估.
func TestAssessRisk(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()
	router := setupTestRouter(mgr)

	createTestConfig(t, router, "风险测试")

	configs := mgr.ListConfigs()
	require.Len(t, configs, 1)
	configID := configs[0].ID

	req := httptest.NewRequest(http.MethodGet, "/smart-backup-sched/configs/"+configID+"/risk", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.NotEmpty(t, data["level"])
	assert.NotNil(t, data["score"])
	assert.NotNil(t, data["successRate"])
}

// TestForecastCapacity 测试容量预测.
func TestForecastCapacity(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()
	router := setupTestRouter(mgr)

	createTestConfig(t, router, "容量测试")

	configs := mgr.ListConfigs()
	require.Len(t, configs, 1)
	configID := configs[0].ID

	req := httptest.NewRequest(http.MethodGet, "/smart-backup-sched/configs/"+configID+"/capacity", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.NotNil(t, data["currentUsage"])
	assert.NotNil(t, data["totalCapacity"])
	assert.NotNil(t, data["usagePercent"])
}

// TestAuditLog 测试审计日志.
func TestAuditLog(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()
	router := setupTestRouter(mgr)

	// 创建一个配置会生成审计日志
	createTestConfig(t, router, "审计测试")

	req := httptest.NewRequest(http.MethodGet, "/smart-backup-sched/audit", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].([]interface{})
	assert.GreaterOrEqual(t, len(data), 1) // 至少有 create 审计
}

// TestStatsAndHealth 测试统计和健康检查.
func TestStatsAndHealth(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()
	router := setupTestRouter(mgr)

	// 统计
	req := httptest.NewRequest(http.MethodGet, "/smart-backup-sched/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var statsResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &statsResp))
	stats := statsResp["data"].(map[string]interface{})
	assert.Equal(t, float64(0), stats["totalConfigs"])

	// 健康检查
	req = httptest.NewRequest(http.MethodGet, "/smart-backup-sched/health", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var healthResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &healthResp))
	health := healthResp["data"].(map[string]interface{})
	assert.Equal(t, "healthy", health["status"])
}

// TestCleanupTasks 测试清理任务.
func TestCleanupTasks(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()
	router := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodPost, "/smart-backup-sched/cleanup", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(0), data["cleaned"])
}

// TestCheckWindow 测试备份窗口检查.
func TestCheckWindow(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()
	router := setupTestRouter(mgr)

	createTestConfig(t, router, "窗口测试")

	configs := mgr.ListConfigs()
	require.Len(t, configs, 1)
	configID := configs[0].ID

	req := httptest.NewRequest(http.MethodGet, "/smart-backup-sched/configs/"+configID+"/window", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.NotNil(t, data["inWindow"])
}

// TestGetConfigNotFound 测试获取不存在的配置.
func TestGetConfigNotFound(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()
	router := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/smart-backup-sched/configs/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestCancelTask 测试取消任务.
func TestCancelTask(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()
	router := setupTestRouter(mgr)

	// 取消不存在的任务
	req := httptest.NewRequest(http.MethodDelete, "/smart-backup-sched/tasks/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestRecommendStrategyWithHighChangeRate 测试高变更率下的策略推荐.
func TestRecommendStrategyWithHighChangeRate(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	configPath := filepath.Join(t.TempDir(), "configs.json")
	auditPath := filepath.Join(t.TempDir(), "audit.json")
	mgr2 := NewManager(configPath, auditPath)
	_ = mgr2.Initialize()

	_ = mgr // suppress unused

	// 创建配置
	cfg := ScheduleConfig{
		Name:       "高变更率测试",
		SourcePath: "/data/high-change",
		Strategy:   StrategyAuto,
	}
	require.NoError(t, mgr2.CreateConfig(cfg))
	configs := mgr2.ListConfigs()
	require.Len(t, configs, 1)

	// 设置高变更率模式
	mgr2.UpdateChangePattern(configs[0].ID, &ChangePattern{
		Path:            "/data/high-change",
		ChangeFrequency: 15,
		ChangeRate:      0.25,
		AvgChangeSize:   1024 * 1024,
		TotalChanges:    10000,
		PeakChangeHour:  14,
	})

	rec := mgr2.RecommendStrategy(configs[0].ID)
	assert.Equal(t, StrategyIncremental, rec.Recommended)
	assert.Greater(t, rec.Confidence, 0.0)
	assert.NotEmpty(t, rec.Reasons)
}

// TestRiskAssessmentWithPatterns 测试带变更模式的风险评估.
func TestRiskAssessmentWithPatterns(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	cfg := ScheduleConfig{
		Name:       "风险评估测试",
		SourcePath: "/data/risk-test",
	}
	require.NoError(t, mgr.CreateConfig(cfg))

	configs := mgr.ListConfigs()
	require.Len(t, configs, 1)
	configID := configs[0].ID

	// 设置极高变更率
	mgr.UpdateChangePattern(configID, &ChangePattern{
		Path:            "/data/risk-test",
		ChangeFrequency: 20,
		ChangeRate:      0.30,
		AvgChangeSize:   1024 * 1024 * 10,
		TotalChanges:    50000,
	})

	risk := mgr.AssessRisk(configID)
	assert.NotNil(t, risk)
	assert.NotEmpty(t, risk.Factors)
	// 高变更率应导致较高风险分
	assert.Greater(t, risk.Score, 0.0)
}

// TestCapacityForecastWithPattern 测试带变更模式的容量预测.
func TestCapacityForecastWithPattern(t *testing.T) {
	mgr, cleanup := setupTestManager(t)
	defer cleanup()

	cfg := ScheduleConfig{
		Name:       "容量预测测试",
		SourcePath: "/data/cap-test",
	}
	require.NoError(t, mgr.CreateConfig(cfg))

	configs := mgr.ListConfigs()
	require.Len(t, configs, 1)
	configID := configs[0].ID

	mgr.UpdateChangePattern(configID, &ChangePattern{
		Path:            "/data/cap-test",
		ChangeFrequency: 2,
		AvgChangeSize:   1024 * 1024 * 5, // 5MB per change
		TotalChanges:    500,
	})

	forecast, err := mgr.ForecastCapacity(configID)
	require.NoError(t, err)
	assert.Greater(t, forecast.TotalCapacity, int64(0))
	assert.Greater(t, forecast.CurrentUsage, int64(0))
	assert.GreaterOrEqual(t, forecast.DaysUntilFull, 0)
}

// TestMain 用于测试环境初始化.
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}
