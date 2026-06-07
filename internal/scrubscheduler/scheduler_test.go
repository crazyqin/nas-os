package scrubscheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// mockExecutor 用于测试的 mock ZFS 执行器
type mockExecutor struct {
	mu           sync.Mutex
	startCalled  int
	stopCalled   int
	progress     float64
	state        ScrubState
	startErr     error
	stopErr      error
	progressErr  error
	startedPools []string
	stoppedPools []string
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		state: ScrubStateRunning,
	}
}

func (m *mockExecutor) StartScrub(ctx context.Context, pool string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startCalled++
	m.startedPools = append(m.startedPools, pool)
	return m.startErr
}

func (m *mockExecutor) StopScrub(ctx context.Context, pool string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopCalled++
	m.stoppedPools = append(m.stoppedPools, pool)
	return m.stopErr
}

func (m *mockExecutor) GetScrubProgress(ctx context.Context, pool string) (float64, ScrubState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.progress, m.state, m.progressErr
}

func setupTestScheduler(t *testing.T) (*ScrubScheduler, *mockExecutor) {
	t.Helper()
	exec := newMockExecutor()
	cfg := DefaultSchedulerConfig()
	cfg.PollIntervalSeconds = 3600 // 不要真的轮询
	s := NewScheduler(slog.Default(), cfg, exec)
	return s, exec
}

func setupTestRouter(t *testing.T, s *ScrubScheduler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	h := NewHandlers(s)
	h.RegisterRoutes(rg)
	return r
}

// ==================== Scheduler 单元测试 ====================

func TestNewScheduler(t *testing.T) {
	s, _ := setupTestScheduler(t)
	if s == nil {
		t.Fatal("expected non-nil scheduler")
	}
	if len(s.schedules) != 0 {
		t.Errorf("expected 0 schedules, got %d", len(s.schedules))
	}
}

func TestNewScheduler_NilConfig(t *testing.T) {
	exec := newMockExecutor()
	s := NewScheduler(nil, nil, exec)
	if s == nil {
		t.Fatal("expected non-nil scheduler")
	}
	if s.config.DefaultMaxDuration != 28800 {
		t.Errorf("expected default max duration 28800, got %d", s.config.DefaultMaxDuration)
	}
}

func TestAddSchedule(t *testing.T) {
	s, _ := setupTestScheduler(t)

	sch := &ScrubSchedule{
		PoolName: "tank",
		Schedule: "0 2 * * 0", // 每周日 2:00
		Enabled:  true,
	}

	if err := s.AddSchedule(sch); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sch.ID == "" {
		t.Error("expected ID to be generated")
	}
	if !sch.CreatedAt.IsZero() == false {
		t.Error("expected CreatedAt to be set")
	}

	schedules := s.ListSchedules()
	if len(schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(schedules))
	}
	if schedules[0].PoolName != "tank" {
		t.Errorf("expected pool 'tank', got %q", schedules[0].PoolName)
	}
}

func TestAddSchedule_InvalidCron(t *testing.T) {
	s, _ := setupTestScheduler(t)

	sch := &ScrubSchedule{
		PoolName: "tank",
		Schedule: "not-a-cron",
		Enabled:  true,
	}

	if err := s.AddSchedule(sch); err == nil {
		t.Error("expected error for invalid cron expression")
	}
}

func TestGetSchedule(t *testing.T) {
	s, _ := setupTestScheduler(t)

	sch := &ScrubSchedule{
		PoolName: "tank",
		Schedule: "0 2 * * 0",
	}
	s.AddSchedule(sch)

	got, err := s.GetSchedule(sch.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.PoolName != "tank" {
		t.Errorf("expected pool 'tank', got %q", got.PoolName)
	}
}

func TestGetSchedule_NotFound(t *testing.T) {
	s, _ := setupTestScheduler(t)

	_, err := s.GetSchedule("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent schedule")
	}
}

func TestUpdateSchedule(t *testing.T) {
	s, _ := setupTestScheduler(t)

	sch := &ScrubSchedule{
		PoolName: "tank",
		Schedule: "0 2 * * 0",
		Enabled:  true,
	}
	s.AddSchedule(sch)

	newSchedule := "0 3 * * 1"
	newMaxDuration := 7200
	req := &UpdateScheduleRequest{
		Schedule:    &newSchedule,
		MaxDuration: &newMaxDuration,
	}

	updated, err := s.UpdateSchedule(sch.ID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Schedule != newSchedule {
		t.Errorf("expected schedule %q, got %q", newSchedule, updated.Schedule)
	}
	if updated.MaxDuration != 7200 {
		t.Errorf("expected max duration 7200, got %d", updated.MaxDuration)
	}
}

func TestUpdateSchedule_NotFound(t *testing.T) {
	s, _ := setupTestScheduler(t)

	newSchedule := "0 3 * * 1"
	req := &UpdateScheduleRequest{
		Schedule: &newSchedule,
	}

	_, err := s.UpdateSchedule("nonexistent", req)
	if err == nil {
		t.Error("expected error for non-existent schedule")
	}
}

func TestUpdateSchedule_InvalidCron(t *testing.T) {
	s, _ := setupTestScheduler(t)

	sch := &ScrubSchedule{
		PoolName: "tank",
		Schedule: "0 2 * * 0",
		Enabled:  true,
	}
	s.AddSchedule(sch)

	badSchedule := "invalid-cron"
	req := &UpdateScheduleRequest{
		Schedule: &badSchedule,
	}

	_, err := s.UpdateSchedule(sch.ID, req)
	if err == nil {
		t.Error("expected error for invalid cron expression")
	}
}

func TestDeleteSchedule(t *testing.T) {
	s, _ := setupTestScheduler(t)

	sch := &ScrubSchedule{
		PoolName: "tank",
		Schedule: "0 2 * * 0",
	}
	s.AddSchedule(sch)

	if err := s.DeleteSchedule(sch.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(s.ListSchedules()) != 0 {
		t.Errorf("expected 0 schedules after delete, got %d", len(s.ListSchedules()))
	}
}

func TestDeleteSchedule_NotFound(t *testing.T) {
	s, _ := setupTestScheduler(t)

	if err := s.DeleteSchedule("nonexistent"); err == nil {
		t.Error("expected error for non-existent schedule")
	}
}

func TestStartScrub(t *testing.T) {
	s, exec := setupTestScheduler(t)

	err := s.StartScrub(context.Background(), "tank")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	exec.mu.Lock()
	if exec.startCalled != 1 {
		t.Errorf("expected StartScrub called once, got %d", exec.startCalled)
	}
	if len(exec.startedPools) != 1 || exec.startedPools[0] != "tank" {
		t.Errorf("expected StartScrub called with 'tank', got %v", exec.startedPools)
	}
	exec.mu.Unlock()
}

func TestStartScrub_AlreadyRunning(t *testing.T) {
	s, exec := setupTestScheduler(t)

	// 第一次启动
	s.StartScrub(context.Background(), "tank")
	// 第二次应该跳过
	s.StartScrub(context.Background(), "tank")

	exec.mu.Lock()
	if exec.startCalled != 1 {
		t.Errorf("expected StartScrub called once, got %d", exec.startCalled)
	}
	exec.mu.Unlock()
}

func TestStartScrub_AuthorFailed(t *testing.T) {
	s, exec := setupTestScheduler(t)
	exec.startErr = fmt.Errorf("zpool not found")

	err := s.StartScrub(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error when start scrub fails")
	}
}

func TestGetPoolStatus_Idle(t *testing.T) {
	s, _ := setupTestScheduler(t)

	status, err := s.GetPoolStatus("tank")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.State != ScrubStateIdle {
		t.Errorf("expected idle, got %q", status.State)
	}
}

func TestGetPoolStatus_Running(t *testing.T) {
	s, _ := setupTestScheduler(t)

	s.StartScrub(context.Background(), "tank")

	status, err := s.GetPoolStatus("tank")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.State != ScrubStateRunning {
		t.Errorf("expected running, got %q", status.State)
	}
}

func TestGetHistory(t *testing.T) {
	s, _ := setupTestScheduler(t)

	// 手动写入历史记录
	s.mu.Lock()
	for i := 0; i < 5; i++ {
		s.history = append(s.history, &ScrubHistory{
			ID:       fmt.Sprintf("h%d", i),
			PoolName: "tank",
			State:    ScrubStateCompleted,
		})
	}
	s.mu.Unlock()

	history := s.GetHistory(3)
	if len(history) != 3 {
		t.Errorf("expected 3 records, got %d", len(history))
	}
	// 应该是最后 3 条 (h2, h3, h4)
	if history[0].ID != "h2" {
		t.Errorf("expected first record ID 'h2', got %q", history[0].ID)
	}
}

func TestGetHistory_NoLimit(t *testing.T) {
	s, _ := setupTestScheduler(t)

	s.mu.Lock()
	for i := 0; i < 3; i++ {
		s.history = append(s.history, &ScrubHistory{
			ID:    fmt.Sprintf("h%d", i),
			State: ScrubStateCompleted,
		})
	}
	s.mu.Unlock()

	history := s.GetHistory(0) // limit=0 返回全部
	if len(history) != 3 {
		t.Errorf("expected 3 records, got %d", len(history))
	}
}

func TestInMaintenanceWindow(t *testing.T) {
	s, _ := setupTestScheduler(t)

	tests := []struct {
		name   string
		window MaintenanceWindow
	}{
		{"empty window (always allowed)", MaintenanceWindow{}},
		{"configured window", MaintenanceWindow{Start: "00:00", End: "23:59"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.inMaintenanceWindow(tt.window)
			if !result {
				t.Errorf("expected true for %s", tt.name)
			}
		})
	}
}

func TestParseHHMM(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"00:00", 0},
		{"01:30", 90},
		{"12:00", 720},
		{"23:59", 1439},
		{"bad", -1},
		{"25:00", -1},
		{"12:60", -1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseHHMM(tt.input)
			if got != tt.want {
				t.Errorf("parseHHMM(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestStartStop(t *testing.T) {
	s, _ := setupTestScheduler(t)

	s.Start()
	if !s.running {
		t.Error("expected scheduler to be running")
	}

	// 重复启动不应该 panic
	s.Start()

	s.Stop()
	if s.running {
		t.Error("expected scheduler to be stopped")
	}

	// 重复停止不应该 panic
	s.Stop()
}

func TestSchedulerWithCronJob(t *testing.T) {
	s, exec := setupTestScheduler(t)

	// 添加一个每分钟的调度
	sch := &ScrubSchedule{
		PoolName: "tank",
		Schedule: "* * * * *",
		Enabled:  true,
		MaintenanceWindow: MaintenanceWindow{
			Start: "00:00",
			End:   "23:59", // 整天都允许
		},
	}
	s.AddSchedule(sch)

	s.Start()
	defer s.Stop()

	// cron 每分钟才执行一次，这里只验证注册成功
	if len(s.cronEntries) != 1 {
		t.Errorf("expected 1 cron entry, got %d", len(s.cronEntries))
	}

	// 手动触发一次验证执行器被调用
	s.onScheduleTrigger(sch.ID)

	exec.mu.Lock()
	if exec.startCalled != 1 {
		t.Errorf("expected executor called 1 time, got %d", exec.startCalled)
	}
	exec.mu.Unlock()
}

func TestSchedulerOutsideMaintenanceWindow(t *testing.T) {
	s, exec := setupTestScheduler(t)

	// 使用一个与当前时间不重叠的维护窗口
	now := time.Now()
	currentMinutes := now.Hour()*60 + now.Minute()
	// 选择一个当前时间不在其中的窗口
	var windowStart, windowEnd string
	if currentMinutes < 12*60 {
		// 当前在中午之前，使用下午窗口
		windowStart = "18:00"
		windowEnd = "19:00"
	} else {
		// 当前在中午之后，使用上午窗口
		windowStart = "03:00"
		windowEnd = "04:00"
	}

	sch := &ScrubSchedule{
		PoolName: "tank",
		Schedule: "* * * * *",
		Enabled:  true,
		MaintenanceWindow: MaintenanceWindow{
			Start: windowStart,
			End:   windowEnd,
		},
	}
	s.AddSchedule(sch)

	// 当前时间不在维护窗口内，应该跳过
	s.onScheduleTrigger(sch.ID)

	exec.mu.Lock()
	if exec.startCalled != 0 {
		t.Errorf("expected executor not called (outside window %s-%s), got %d", windowStart, windowEnd, exec.startCalled)
	}
	exec.mu.Unlock()
}

// ==================== Handler HTTP 测试 ====================

func TestHandler_ListSchedules_Empty(t *testing.T) {
	s, _ := setupTestScheduler(t)
	r := setupTestRouter(t, s)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/scrub/schedules", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestHandler_CreateSchedule(t *testing.T) {
	s, _ := setupTestScheduler(t)
	r := setupTestRouter(t, s)

	body := CreateScheduleRequest{
		PoolName: "tank",
		Schedule: "0 2 * * 0",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/scrub/schedules", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}
	if data["pool_name"] != "tank" {
		t.Errorf("expected pool_name 'tank', got %v", data["pool_name"])
	}
}

func TestHandler_CreateSchedule_InvalidBody(t *testing.T) {
	s, _ := setupTestScheduler(t)
	r := setupTestRouter(t, s)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/scrub/schedules",
		bytes.NewBufferString(`{"pool_name": ""}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_CreateSchedule_InvalidCron(t *testing.T) {
	s, _ := setupTestScheduler(t)
	r := setupTestRouter(t, s)

	body := CreateScheduleRequest{
		PoolName: "tank",
		Schedule: "not-valid-cron",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/scrub/schedules", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_UpdateSchedule(t *testing.T) {
	s, _ := setupTestScheduler(t)
	r := setupTestRouter(t, s)

	// 先创建
	sch := &ScrubSchedule{
		PoolName: "tank",
		Schedule: "0 2 * * 0",
		Enabled:  true,
	}
	s.AddSchedule(sch)

	newSchedule := "0 3 * * 1"
	body := UpdateScheduleRequest{
		Schedule: &newSchedule,
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/scrub/schedules/"+sch.ID, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UpdateSchedule_NotFound(t *testing.T) {
	s, _ := setupTestScheduler(t)
	r := setupTestRouter(t, s)

	newSchedule := "0 3 * * 1"
	body := UpdateScheduleRequest{
		Schedule: &newSchedule,
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/scrub/schedules/nonexistent", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandler_DeleteSchedule(t *testing.T) {
	s, _ := setupTestScheduler(t)
	r := setupTestRouter(t, s)

	sch := &ScrubSchedule{
		PoolName: "tank",
		Schedule: "0 2 * * 0",
	}
	s.AddSchedule(sch)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/scrub/schedules/"+sch.ID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_DeleteSchedule_NotFound(t *testing.T) {
	s, _ := setupTestScheduler(t)
	r := setupTestRouter(t, s)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/scrub/schedules/nonexistent", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandler_StartScrub(t *testing.T) {
	s, _ := setupTestScheduler(t)
	r := setupTestRouter(t, s)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/scrub/pools/tank/start", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_GetPoolStatus(t *testing.T) {
	s, _ := setupTestScheduler(t)
	r := setupTestRouter(t, s)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/scrub/pools/tank/status", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}
	if data["pool_name"] != "tank" {
		t.Errorf("expected pool_name 'tank', got %v", data["pool_name"])
	}
	if data["state"] != "idle" {
		t.Errorf("expected state 'idle', got %v", data["state"])
	}
}

func TestHandler_GetHistory(t *testing.T) {
	s, _ := setupTestScheduler(t)
	r := setupTestRouter(t, s)

	// 写入一些历史
	s.mu.Lock()
	for i := 0; i < 5; i++ {
		s.history = append(s.history, &ScrubHistory{
			ID:    fmt.Sprintf("h%d", i),
			State: ScrubStateCompleted,
		})
	}
	s.mu.Unlock()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/scrub/history?limit=3", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	records, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("expected data to be an array, got %T", resp.Data)
	}
	if len(records) != 3 {
		t.Errorf("expected 3 records, got %d", len(records))
	}
}

func TestHandler_GetHistory_DefaultLimit(t *testing.T) {
	s, _ := setupTestScheduler(t)
	r := setupTestRouter(t, s)

	s.mu.Lock()
	for i := 0; i < 3; i++ {
		s.history = append(s.history, &ScrubHistory{
			ID:    fmt.Sprintf("h%d", i),
			State: ScrubStateCompleted,
		})
	}
	s.mu.Unlock()

	// 不带 limit 参数
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/scrub/history", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	records, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("expected data to be an array, got %T", resp.Data)
	}
	if len(records) != 3 {
		t.Errorf("expected 3 records (default limit 50 returns all 3), got %d", len(records))
	}
}

// ==================== Type 辅助函数测试 ====================

func TestDefaultSchedulerConfig(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.DefaultMaintenanceWindow.Start != "00:00" {
		t.Errorf("expected start '00:00', got %q", cfg.DefaultMaintenanceWindow.Start)
	}
	if cfg.DefaultMaintenanceWindow.End != "06:00" {
		t.Errorf("expected end '06:00', got %q", cfg.DefaultMaintenanceWindow.End)
	}
	if cfg.DefaultMaxDuration != 28800 {
		t.Errorf("expected max duration 28800, got %d", cfg.DefaultMaxDuration)
	}
	if cfg.DefaultRetryCount != 3 {
		t.Errorf("expected retry count 3, got %d", cfg.DefaultRetryCount)
	}
	if cfg.PollIntervalSeconds != 30 {
		t.Errorf("expected poll interval 30, got %d", cfg.PollIntervalSeconds)
	}
	if cfg.MaxHistoryRecords != 500 {
		t.Errorf("expected max history 500, got %d", cfg.MaxHistoryRecords)
	}
}
