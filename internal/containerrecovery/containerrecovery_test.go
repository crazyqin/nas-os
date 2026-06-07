// Package containerrecovery 单元测试
package containerrecovery

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

// ========== Mock 实现 ==========

// mockContainerOperator 模拟容器操作器.
type mockContainerOperator struct {
	restartErr     error
	stopErr        error
	startErr       error
	status         string
	statusErr      error
	healthStatus   HealthStatus
	healthErr      error
	rollbackErr    error
	scaleErr       error
	restartCalled  bool
	rollbackCalled bool
	scaleCalled    bool
}

func newMockContainerOperator() *mockContainerOperator {
	return &mockContainerOperator{
		status:       "running",
		healthStatus: HealthStatusHealthy,
	}
}

func (m *mockContainerOperator) Rename(container string) error {
	m.restartCalled = true
	return m.restartErr
}

func (m *mockContainerOperator) Stop(container string) error {
	return m.stopErr
}

func (m *mockContainerOperator) Start(container string) error {
	return m.startErr
}

func (m *mockContainerOperator) GetStatus(container string) (string, error) {
	return m.status, m.statusErr
}

func (m *mockContainerOperator) GetHealthCheck(container string) (HealthStatus, error) {
	return m.healthStatus, m.healthErr
}

func (m *mockContainerOperator) Rollback(container string) error {
	m.rollbackCalled = true
	return m.rollbackErr
}

func (m *mockContainerOperator) Scale(container string, count int) error {
	m.scaleCalled = true
	return m.scaleErr
}

// mockStore 模拟存储.
type mockStore struct {
	records    []*RecoveryRecord
	saveErr    error
	queryErr   error
	stats      *RecoveryStats
	statsErr   error
	updateErr  error
	cleanupErr error
}

func newMockStore() *mockStore {
	return &mockStore{
		records: make([]*RecoveryRecord, 0),
		stats: &RecoveryStats{
			TotalRecoveries:  0,
			SuccessfulCount:  0,
			FailedCount:      0,
			SuccessRate:      0,
			FailureFrequency: make(map[string]int64),
			ContainerStats:   make(map[string]*ContainerStats),
			LastUpdated:      time.Now(),
		},
	}
}

func (m *mockStore) SaveRecord(record *RecoveryRecord) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.records = append(m.records, record)
	return nil
}

func (m *mockStore) GetRecords(container string, limit int) ([]*RecoveryRecord, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}

	if container == "" {
		if limit > 0 && limit < len(m.records) {
			return m.records[:limit], nil
		}
		return m.records, nil
	}

	var result []*RecoveryRecord
	for _, r := range m.records {
		if r.Container == container {
			result = append(result, r)
		}
	}
	if limit > 0 && limit < len(result) {
		return result[:limit], nil
	}
	return result, nil
}

func (m *mockStore) GetStats() (*RecoveryStats, error) {
	return m.stats, m.statsErr
}

func (m *mockStore) UpdateStats(record *RecoveryRecord) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.stats.TotalRecoveries++
	if record.Status == RecoveryStatusSuccess {
		m.stats.SuccessfulCount++
	} else {
		m.stats.FailedCount++
	}
	return nil
}

func (m *mockStore) Cleanup(olderThan time.Duration) error {
	return m.cleanupErr
}

// mockAlertSender 模拟告警发送器.
type mockAlertSender struct {
	alerts  []*Alert
	sendErr error
}

func newMockAlertSender() *mockAlertSender {
	return &mockAlertSender{
		alerts: make([]*Alert, 0),
	}
}

func (m *mockAlertSender) Send(alert *Alert) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.alerts = append(m.alerts, alert)
	return nil
}

// ========== Types Tests ==========

func TestDefaultRecoveryStrategy(t *testing.T) {
	strategy := DefaultRecoveryStrategy()

	assert.Equal(t, RecoveryActionRestart, strategy.Action)
	assert.Equal(t, 3, strategy.MaxRetries)
	assert.Equal(t, 5*time.Second, strategy.InitialBackoff)
	assert.Equal(t, 5*time.Minute, strategy.MaxBackoff)
	assert.Equal(t, 2.0, strategy.BackoffMultiplier)
	assert.Equal(t, 10*time.Minute, strategy.CooldownPeriod)
	assert.True(t, strategy.NotifyOnFailure)
}

func TestDefaultEngineConfig(t *testing.T) {
	config := DefaultEngineConfig()

	assert.True(t, config.Enabled)
	assert.Equal(t, 3, config.Concurrency)
	assert.Equal(t, 30*time.Second, config.HealthCheckInterval)
	assert.Equal(t, 5*time.Minute, config.RecoveryTimeout)
	assert.Equal(t, 1000, config.HistoryLimit)
}

func TestTypes_Constants(t *testing.T) {
	// 健康检查类型
	assert.Equal(t, HealthCheckType("http"), HealthCheckHTTP)
	assert.Equal(t, HealthCheckType("tcp"), HealthCheckTCP)
	assert.Equal(t, HealthCheckType("command"), HealthCheckCommand)
	assert.Equal(t, HealthCheckType("container"), HealthCheckContainer)

	// 健康状态
	assert.Equal(t, HealthStatus("healthy"), HealthStatusHealthy)
	assert.Equal(t, HealthStatus("unhealthy"), HealthStatusUnhealthy)
	assert.Equal(t, HealthStatus("unknown"), HealthStatusUnknown)
	assert.Equal(t, HealthStatus("starting"), HealthStatusStarting)

	// 恢复动作
	assert.Equal(t, RecoveryAction("restart"), RecoveryActionRestart)
	assert.Equal(t, RecoveryAction("notify"), RecoveryActionNotify)
	assert.Equal(t, RecoveryAction("rollback"), RecoveryActionRollback)
	assert.Equal(t, RecoveryAction("scale_up"), RecoveryActionScaleUp)

	// 故障模式
	assert.Equal(t, FailureMode("oom_killed"), FailureModeOOMKilled)
	assert.Equal(t, FailureMode("crash_loop_backoff"), FailureModeCrashLoopBackOff)
	assert.Equal(t, FailureMode("image_pull_backoff"), FailureModeImagePullBackOff)

	// 恢复状态
	assert.Equal(t, RecoveryStatus("pending"), RecoveryStatusPending)
	assert.Equal(t, RecoveryStatus("running"), RecoveryStatusRunning)
	assert.Equal(t, RecoveryStatus("success"), RecoveryStatusSuccess)
	assert.Equal(t, RecoveryStatus("failed"), RecoveryStatusFailed)

	// 钩子阶段
	assert.Equal(t, HookPhase("pre_recovery"), HookPhasePreRecovery)
	assert.Equal(t, HookPhase("post_recovery"), HookPhasePostRecovery)

	// 告警级别
	assert.Equal(t, AlertLevel("info"), AlertLevelInfo)
	assert.Equal(t, AlertLevel("warning"), AlertLevelWarning)
	assert.Equal(t, AlertLevel("error"), AlertLevelError)
	assert.Equal(t, AlertLevel("critical"), AlertLevelCritical)
}

// ========== Engine Tests ==========

func TestNewEngine(t *testing.T) {
	config := DefaultEngineConfig()
	engine := NewEngine(config, nil, nil, nil)

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.containers)
	assert.NotNil(t, engine.depGraph)
	assert.NotNil(t, engine.hooks)
	assert.False(t, engine.running)
}

func TestEngine_RegisterContainer(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig(), nil, nil, nil)

	cfg := &ContainerConfig{
		ContainerName: "test-container",
		Enabled:       true,
		HealthCheck: HealthCheckConfig{
			Type:     HealthCheckHTTP,
			Interval: 30 * time.Second,
			Timeout:  5 * time.Second,
		},
		Strategy: DefaultRecoveryStrategy(),
	}

	engine.RegisterContainer(cfg)

	registered, ok := engine.GetContainer("test-container")
	assert.True(t, ok)
	assert.Equal(t, "test-container", registered.ContainerName)
	assert.True(t, registered.Enabled)
}

func TestEngine_UnregisterContainer(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig(), nil, nil, nil)

	cfg := &ContainerConfig{
		ContainerName: "test-container",
		Enabled:       true,
		Strategy:      DefaultRecoveryStrategy(),
	}

	engine.RegisterContainer(cfg)
	engine.UnregisterContainer("test-container")

	_, ok := engine.GetContainer("test-container")
	assert.False(t, ok)
}

func TestEngine_ListContainers(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig(), nil, nil, nil)

	engine.RegisterContainer(&ContainerConfig{
		ContainerName: "container-1",
		Enabled:       true,
		Strategy:      DefaultRecoveryStrategy(),
	})
	engine.RegisterContainer(&ContainerConfig{
		ContainerName: "container-2",
		Enabled:       true,
		Strategy:      DefaultRecoveryStrategy(),
	})

	containers := engine.ListContainers()
	assert.Len(t, containers, 2)
}

func TestEngine_StartStop(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig(), nil, nil, nil)

	engine.Start()
	assert.True(t, engine.IsRunning())

	// 重复启动不 panic
	engine.Start()
	assert.True(t, engine.IsRunning())

	engine.Stop()
	assert.False(t, engine.IsRunning())

	// 重复停止不 panic
	engine.Stop()
	assert.False(t, engine.IsRunning())
}

func TestEngine_UpdateConfig(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig(), nil, nil, nil)

	newConfig := EngineConfig{
		Enabled:             false,
		Concurrency:         5,
		HealthCheckInterval: 1 * time.Minute,
		RecoveryTimeout:     10 * time.Minute,
		HistoryLimit:        500,
	}

	engine.UpdateConfig(newConfig)

	config := engine.GetConfig()
	assert.False(t, config.Enabled)
	assert.Equal(t, 5, config.Concurrency)
	assert.Equal(t, 1*time.Minute, config.HealthCheckInterval)
}

func TestEngine_TriggerRecovery_NotRegistered(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig(), nil, nil, nil)

	_, err := engine.TriggerRecovery("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestEngine_TriggerRecovery_Notify(t *testing.T) {
	store := newMockStore()
	operator := newMockContainerOperator()
	sender := newMockAlertSender()

	engine := NewEngine(DefaultEngineConfig(), store, operator, nil)
	engine.SetAlertSender(sender)

	cfg := &ContainerConfig{
		ContainerName: "test-container",
		Enabled:       true,
		Strategy: RecoveryStrategy{
			Action:          RecoveryActionNotify,
			MaxRetries:      3,
			NotifyOnFailure: true,
		},
	}
	engine.RegisterContainer(cfg)

	record, err := engine.TriggerRecovery("test-container")
	require.NoError(t, err)
	assert.NotNil(t, record)
	assert.Equal(t, RecoveryStatusSuccess, record.Status)
	assert.Equal(t, RecoveryActionNotify, record.Action)

	// 验证告警已发送
	assert.Len(t, sender.alerts, 1)
	assert.Equal(t, "test-container", sender.alerts[0].Container)
}

func TestEngine_TriggerRecovery_Restart_Success(t *testing.T) {
	store := newMockStore()
	operator := newMockContainerOperator()
	operator.healthStatus = HealthStatusHealthy

	engine := NewEngine(DefaultEngineConfig(), store, operator, nil)

	cfg := &ContainerConfig{
		ContainerName: "test-container",
		Enabled:       true,
		Strategy: RecoveryStrategy{
			Action:            RecoveryActionRestart,
			MaxRetries:        3,
			InitialBackoff:    10 * time.Millisecond,
			MaxBackoff:        100 * time.Millisecond,
			BackoffMultiplier: 2.0,
			CooldownPeriod:    1 * time.Second,
		},
	}
	engine.RegisterContainer(cfg)

	record, err := engine.TriggerRecovery("test-container")
	require.NoError(t, err)
	assert.NotNil(t, record)
	assert.Equal(t, RecoveryStatusSuccess, record.Status)
	assert.True(t, operator.restartCalled)
}

func TestEngine_TriggerRecovery_Rollback(t *testing.T) {
	store := newMockStore()
	operator := newMockContainerOperator()

	engine := NewEngine(DefaultEngineConfig(), store, operator, nil)

	cfg := &ContainerConfig{
		ContainerName: "test-container",
		Enabled:       true,
		Strategy: RecoveryStrategy{
			Action: RecoveryActionRollback,
		},
	}
	engine.RegisterContainer(cfg)

	record, err := engine.TriggerRecovery("test-container")
	require.NoError(t, err)
	assert.NotNil(t, record)
	assert.Equal(t, RecoveryStatusSuccess, record.Status)
	assert.True(t, operator.rollbackCalled)
}

func TestEngine_TriggerRecovery_ScaleUp(t *testing.T) {
	store := newMockStore()
	operator := newMockContainerOperator()

	engine := NewEngine(DefaultEngineConfig(), store, operator, nil)

	cfg := &ContainerConfig{
		ContainerName: "test-container",
		Enabled:       true,
		Strategy: RecoveryStrategy{
			Action: RecoveryActionScaleUp,
		},
	}
	engine.RegisterContainer(cfg)

	record, err := engine.TriggerRecovery("test-container")
	require.NoError(t, err)
	assert.NotNil(t, record)
	assert.Equal(t, RecoveryStatusSuccess, record.Status)
	assert.True(t, operator.scaleCalled)
}

func TestEngine_GetRecoveryRecords(t *testing.T) {
	store := newMockStore()
	store.records = []*RecoveryRecord{
		{ID: "1", Container: "test", Status: RecoveryStatusSuccess},
		{ID: "2", Container: "test", Status: RecoveryStatusFailed},
	}

	engine := NewEngine(DefaultEngineConfig(), store, nil, nil)

	records, err := engine.GetRecoveryRecords("test", 10)
	require.NoError(t, err)
	assert.Len(t, records, 2)
}

func TestEngine_GetStats(t *testing.T) {
	store := newMockStore()
	engine := NewEngine(DefaultEngineConfig(), store, nil, nil)

	stats, err := engine.GetStats()
	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, int64(0), stats.TotalRecoveries)
}

// ========== DependencyGraph Tests ==========

func TestDependencyGraph_Add(t *testing.T) {
	graph := NewDependencyGraph()

	graph.Add("app", []string{"db", "redis"}, 1)
	graph.Add("db", nil, 0)
	graph.Add("redis", nil, 0)

	deps := graph.GetDependencies("app")
	assert.Len(t, deps, 2)
	assert.Contains(t, deps, "db")
	assert.Contains(t, deps, "redis")
}

func TestDependencyGraph_Remove(t *testing.T) {
	graph := NewDependencyGraph()

	graph.Add("app", []string{"db"}, 1)
	graph.Add("db", nil, 0)

	graph.Remove("app")

	deps := graph.GetDependencies("app")
	assert.Nil(t, deps)
}

func TestDependencyGraph_GetRecoveryOrder(t *testing.T) {
	graph := NewDependencyGraph()

	graph.Add("app", []string{"db", "redis"}, 2)
	graph.Add("db", nil, 0)
	graph.Add("redis", nil, 1)

	order := graph.GetRecoveryOrder([]string{"app", "db", "redis"})

	// db 和 redis 应该在 app 之前
	assert.Len(t, order, 3)

	appIdx := -1
	dbIdx := -1
	redisIdx := -1
	for i, name := range order {
		switch name {
		case "app":
			appIdx = i
		case "db":
			dbIdx = i
		case "redis":
			redisIdx = i
		}
	}

	assert.True(t, dbIdx < appIdx, "db should be before app")
	assert.True(t, redisIdx < appIdx, "redis should be before app")
}

func TestDependencyGraph_GetRecoveryOrder_NoDeps(t *testing.T) {
	graph := NewDependencyGraph()

	graph.Add("a", nil, 1)
	graph.Add("b", nil, 0)
	graph.Add("c", nil, 2)

	order := graph.GetRecoveryOrder([]string{"a", "b", "c"})

	// 无依赖时按优先级排序
	assert.Len(t, order, 3)
	assert.Equal(t, "b", order[0]) // 优先级 0
	assert.Equal(t, "a", order[1]) // 优先级 1
	assert.Equal(t, "c", order[2]) // 优先级 2
}

func TestDependencyGraph_GetDependents(t *testing.T) {
	graph := NewDependencyGraph()

	graph.Add("app", []string{"db"}, 1)
	graph.Add("worker", []string{"db"}, 2)
	graph.Add("db", nil, 0)

	dependents := graph.GetDependents("db")
	assert.Len(t, dependents, 2)
	assert.Contains(t, dependents, "app")
	assert.Contains(t, dependents, "worker")
}

// ========== Handlers Tests ==========

func setupHandlers(t *testing.T) (*gin.Engine, *Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	store := newMockStore()
	operator := newMockContainerOperator()
	engine := NewEngine(DefaultEngineConfig(), store, operator, nil)

	h := NewHandlers(engine)

	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)

	return r, engine
}

func TestHandlers_GetStatus(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/container-recovery/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "running")
}

func TestHandlers_ListContainers_Empty(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/container-recovery/containers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "total")
}

func TestHandlers_RegisterContainer(t *testing.T) {
	r, engine := setupHandlers(t)
	_ = engine

	body := `{
		"container_name": "test-app",
		"enabled": true,
		"health_check": {
			"type": "http",
			"interval": 30000000000,
			"timeout": 5000000000
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/container-recovery/containers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test-app")
}

func TestHandlers_RegisterContainer_Invalid(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `invalid json`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/container-recovery/containers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_GetContainer(t *testing.T) {
	r, engine := setupHandlers(t)

	engine.RegisterContainer(&ContainerConfig{
		ContainerName: "test-app",
		Enabled:       true,
		Strategy:      DefaultRecoveryStrategy(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/container-recovery/containers/test-app", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test-app")
}

func TestHandlers_GetContainer_NotFound(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/container-recovery/containers/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_UnregisterContainer(t *testing.T) {
	r, engine := setupHandlers(t)

	engine.RegisterContainer(&ContainerConfig{
		ContainerName: "test-app",
		Enabled:       true,
		Strategy:      DefaultRecoveryStrategy(),
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/container-recovery/containers/test-app", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证已删除
	_, ok := engine.GetContainer("test-app")
	assert.False(t, ok)
}

func TestHandlers_TriggerRecovery(t *testing.T) {
	r, engine := setupHandlers(t)

	engine.RegisterContainer(&ContainerConfig{
		ContainerName: "test-app",
		Enabled:       true,
		Strategy: RecoveryStrategy{
			Action:         RecoveryActionRestart,
			MaxRetries:     3,
			InitialBackoff: 10 * time.Millisecond,
			MaxBackoff:     100 * time.Millisecond,
			CooldownPeriod: 1 * time.Second,
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/container-recovery/containers/test-app/recover", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "recovery triggered")
}

func TestHandlers_TriggerRecovery_NotFound(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/container-recovery/containers/nonexistent/recover", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_GetRecords(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/container-recovery/records", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "records")
}

func TestHandlers_GetStats(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/container-recovery/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "total_recoveries")
}

func TestHandlers_GetConfig(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/container-recovery/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "enabled")
}

func TestHandlers_UpdateConfig(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `{"enabled":true,"concurrency":5,"health_check_interval":60000000000}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/container-recovery/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "config updated")
}

func TestHandlers_UpdateConfig_Invalid(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `invalid json`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/container-recovery/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_StartEngine(t *testing.T) {
	r, engine := setupHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/container-recovery/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, engine.IsRunning())

	// 清理
	engine.Stop()
}

func TestHandlers_StopEngine(t *testing.T) {
	r, engine := setupHandlers(t)

	engine.Start()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/container-recovery/stop", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, engine.IsRunning())
}

func TestHandlers_StartEngine_AlreadyRunning(t *testing.T) {
	r, engine := setupHandlers(t)

	engine.Start()
	defer engine.Stop()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/container-recovery/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "already running")
}

func TestHandlers_StopEngine_AlreadyStopped(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/container-recovery/stop", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "already stopped")
}

func TestHandlers_GetContainerRecords(t *testing.T) {
	r, engine := setupHandlers(t)

	engine.RegisterContainer(&ContainerConfig{
		ContainerName: "test-app",
		Enabled:       true,
		Strategy:      DefaultRecoveryStrategy(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/container-recovery/containers/test-app/records", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test-app")
}

// ========== Failure Mode Detection Tests ==========

func TestDetectFailureMode_OOMKilled(t *testing.T) {
	operator := newMockContainerOperator()
	operator.status = "oom_killed"

	engine := NewEngine(DefaultEngineConfig(), nil, operator, nil)

	mode := engine.detectFailureMode("test-container")
	assert.Equal(t, FailureModeOOMKilled, mode)
}

func TestDetectFailureMode_CrashLoop(t *testing.T) {
	operator := newMockContainerOperator()
	operator.status = "crash_loop_backoff"

	engine := NewEngine(DefaultEngineConfig(), nil, operator, nil)

	mode := engine.detectFailureMode("test-container")
	assert.Equal(t, FailureModeCrashLoopBackOff, mode)
}

func TestDetectFailureMode_ImagePull(t *testing.T) {
	operator := newMockContainerOperator()
	operator.status = "image_pull_error"

	engine := NewEngine(DefaultEngineConfig(), nil, operator, nil)

	mode := engine.detectFailureMode("test-container")
	assert.Equal(t, FailureModeImagePullBackOff, mode)
}

func TestDetectFailureMode_Unknown(t *testing.T) {
	operator := newMockContainerOperator()
	operator.status = "some_other_status"

	engine := NewEngine(DefaultEngineConfig(), nil, operator, nil)

	mode := engine.detectFailureMode("test-container")
	assert.Equal(t, FailureModeUnknown, mode)
}

func TestDetectFailureMode_NilOperator(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig(), nil, nil, nil)

	mode := engine.detectFailureMode("test-container")
	assert.Equal(t, FailureModeUnknown, mode)
}

// ========== Integration Tests ==========

func TestIntegration_FullRecoveryFlow(t *testing.T) {
	store := newMockStore()
	operator := newMockContainerOperator()
	sender := newMockAlertSender()

	engine := NewEngine(DefaultEngineConfig(), store, operator, nil)
	engine.SetAlertSender(sender)

	// 注册多个有依赖关系的容器
	engine.RegisterContainer(&ContainerConfig{
		ContainerName: "database",
		Enabled:       true,
		Priority:      0,
		Strategy: RecoveryStrategy{
			Action:         RecoveryActionRestart,
			MaxRetries:     3,
			InitialBackoff: 10 * time.Millisecond,
			CooldownPeriod: 1 * time.Second,
		},
	})

	engine.RegisterContainer(&ContainerConfig{
		ContainerName: "app",
		Enabled:       true,
		Priority:      1,
		Dependencies:  []string{"database"},
		Strategy: RecoveryStrategy{
			Action:         RecoveryActionRestart,
			MaxRetries:     3,
			InitialBackoff: 10 * time.Millisecond,
			CooldownPeriod: 1 * time.Second,
		},
	})

	// 验证容器已注册
	containers := engine.ListContainers()
	assert.Len(t, containers, 2)

	// 触发恢复
	record, err := engine.TriggerRecovery("app")
	require.NoError(t, err)
	assert.NotNil(t, record)
	assert.Equal(t, RecoveryStatusSuccess, record.Status)

	// 验证记录已保存
	assert.GreaterOrEqual(t, len(store.records), 1)

	// 验证统计已更新
	assert.Equal(t, int64(1), store.stats.TotalRecoveries)
	assert.Equal(t, int64(1), store.stats.SuccessfulCount)
}

func TestIntegration_DependencyOrder(t *testing.T) {
	graph := NewDependencyGraph()

	graph.Add("web", []string{"api"}, 2)
	graph.Add("api", []string{"db", "cache"}, 1)
	graph.Add("db", nil, 0)
	graph.Add("cache", nil, 0)

	order := graph.GetRecoveryOrder([]string{"web", "api", "db", "cache"})

	// 验证顺序：db 和 cache 最先，然后 api，最后 web
	assert.Len(t, order, 4)

	webIdx := indexOf(order, "web")
	apiIdx := indexOf(order, "api")
	dbIdx := indexOf(order, "db")
	cacheIdx := indexOf(order, "cache")

	assert.True(t, dbIdx < apiIdx, "db should be before api")
	assert.True(t, cacheIdx < apiIdx, "cache should be before api")
	assert.True(t, apiIdx < webIdx, "api should be before web")
}

func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}

// ========== Benchmark ==========

func BenchmarkEngine_TriggerRecovery(b *testing.B) {
	store := newMockStore()
	operator := newMockContainerOperator()

	engine := NewEngine(DefaultEngineConfig(), store, operator, nil)

	engine.RegisterContainer(&ContainerConfig{
		ContainerName: "bench-container",
		Enabled:       true,
		Strategy: RecoveryStrategy{
			Action:         RecoveryActionRestart,
			MaxRetries:     1,
			InitialBackoff: 1 * time.Millisecond,
			CooldownPeriod: 1 * time.Second,
		},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.TriggerRecovery("bench-container")
	}
}

func BenchmarkDependencyGraph_GetRecoveryOrder(b *testing.B) {
	graph := NewDependencyGraph()

	graph.Add("web", []string{"api"}, 2)
	graph.Add("api", []string{"db", "cache"}, 1)
	graph.Add("db", nil, 0)
	graph.Add("cache", nil, 0)

	containers := []string{"web", "api", "db", "cache"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		graph.GetRecoveryOrder(containers)
	}
}
