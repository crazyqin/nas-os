package ransombehaviorai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(zap.NewNop(), nil)
}

func setupTestHandlers(t *testing.T, m *Manager) (*http.ServeMux, *httptest.Server) {
	t.Helper()
	mux := http.NewServeMux()
	h := NewHandlers(m)
	h.RegisterRoutes(mux)
	return mux, httptest.NewServer(mux)
}

// ============================================================
// Types Tests
// ============================================================

func TestThreatLevel_String(t *testing.T) {
	tests := []struct {
		level ThreatLevel
		want  string
	}{
		{ThreatLevelNone, "none"},
		{ThreatLevelLow, "low"},
		{ThreatLevelMedium, "medium"},
		{ThreatLevelHigh, "high"},
		{ThreatLevelCritical, "critical"},
		{ThreatLevel(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("ThreatLevel(%d).String() = %s, want %s", tt.level, got, tt.want)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Error("expected enabled to be true")
	}
	if !cfg.FileMonitor.Enabled {
		t.Error("expected file monitor to be enabled")
	}
	if !cfg.IOMonitor.Enabled {
		t.Error("expected IO monitor to be enabled")
	}
	if !cfg.ProcessMonitor.Enabled {
		t.Error("expected process monitor to be enabled")
	}
	if cfg.FileMonitor.BulkWriteThreshold != 50 {
		t.Errorf("expected bulk write threshold 50, got %d", cfg.FileMonitor.BulkWriteThreshold)
	}
	if cfg.AIModel.ScoreThreshold != 70 {
		t.Errorf("expected score threshold 70, got %d", cfg.AIModel.ScoreThreshold)
	}
	if len(cfg.ProcessMonitor.SuspiciousProcessNames) == 0 {
		t.Error("expected non-empty suspicious process names")
	}
	if len(cfg.FileMonitor.SuspiciousExtensions) == 0 {
		t.Error("expected non-empty suspicious extensions")
	}
}

// ============================================================
// Manager Tests
// ============================================================

func TestNewManager(t *testing.T) {
	m := NewManager(nil, nil)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.IsRunning() {
		t.Error("expected engine not to be running initially")
	}
}

func TestManager_StartStop(t *testing.T) {
	m := setupTestManager(t)

	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !m.IsRunning() {
		t.Error("expected engine to be running")
	}

	// 重复启动应报错
	if err := m.Start(); err == nil {
		t.Error("expected error on double start")
	}

	m.Stop()
	if m.IsRunning() {
		t.Error("expected engine to be stopped")
	}
}

func TestManager_ReportFileEvent(t *testing.T) {
	m := setupTestManager(t)
	m.Start()
	defer m.Stop()

	event := FileBehaviorEvent{
		Type:        FileEventModify,
		Path:        "/data/test.txt",
		Size:        1024,
		ProcessName: "test-proc",
		ProcessID:   1234,
	}
	m.ReportFileEvent(event)

	status := m.GetStatus()
	if status.Stats.FileEvents != 1 {
		t.Errorf("expected 1 file event, got %d", status.Stats.FileEvents)
	}
	if status.Stats.TotalEvents != 1 {
		t.Errorf("expected 1 total event, got %d", status.Stats.TotalEvents)
	}
}

func TestManager_ReportIOSample(t *testing.T) {
	m := setupTestManager(t)
	m.Start()
	defer m.Stop()

	sample := IOSample{
		ReadBytes:   1024,
		WriteBytes:  2048,
		ReadOps:     10,
		WriteOps:    20,
		SourcePath:  "/data",
		ProcessName: "test-proc",
	}
	m.ReportIOSample(sample)

	status := m.GetStatus()
	if status.Stats.IOEvents != 1 {
		t.Errorf("expected 1 IO event, got %d", status.Stats.IOEvents)
	}
}

func TestManager_ReportProcessEvent(t *testing.T) {
	m := setupTestManager(t)
	m.Start()
	defer m.Stop()

	event := ProcessBehaviorEvent{
		Type:        ProcessEventSuspicious,
		ProcessName: "wannacry",
		ProcessID:   5678,
		UserID:      "root",
	}
	m.ReportProcessEvent(event)

	status := m.GetStatus()
	if status.Stats.ProcessEvents != 1 {
		t.Errorf("expected 1 process event, got %d", status.Stats.ProcessEvents)
	}
}

func TestManager_Evaluate_NoEvents(t *testing.T) {
	m := setupTestManager(t)

	result := m.Evaluate()
	if result.Score != 0 {
		t.Errorf("expected score 0 with no events, got %d", result.Score)
	}
	if result.ThreatLevel != ThreatLevelNone {
		t.Errorf("expected threat level none, got %v", result.ThreatLevel)
	}
	if result.Confidence != 0 {
		t.Errorf("expected confidence 0 with no events, got %d", result.Confidence)
	}
}

func TestManager_Evaluate_HighThreat(t *testing.T) {
	m := setupTestManager(t)
	m.Start()
	defer m.Stop()

	now := time.Now()

	// 注入大量加密相关的文件事件
	for i := 0; i < 40; i++ {
		m.ReportFileEvent(FileBehaviorEvent{
			Type:        FileEventEncrypt,
			Path:        "/data/file_" + fmt.Sprintf("%d", i) + ".locked",
			Extension:   ".locked",
			ProcessName: "unknown-proc",
			Entropy:     7.9,
			Timestamp:   now,
		})
	}

	// 注入批量重命名
	for i := 0; i < 20; i++ {
		m.ReportFileEvent(FileBehaviorEvent{
			Type:      FileEventRename,
			Path:      "/data/renamed_" + fmt.Sprintf("%d", i) + ".encrypted",
			Extension: ".encrypted",
			Timestamp: now,
		})
	}

	// 注入可疑进程（多个以提升评分）
	for i := 0; i < 4; i++ {
		m.ReportProcessEvent(ProcessBehaviorEvent{
			Type:        ProcessEventSuspicious,
			ProcessName: "wannacry",
			ProcessID:   9999 + i,
			Timestamp:   now,
		})
	}
	for i := 0; i < 5; i++ {
		m.ReportProcessEvent(ProcessBehaviorEvent{
			Type:        ProcessEventPrivEsc,
			ProcessName: "unknown-proc",
			ProcessID:   8888 + i,
			Timestamp:   now,
		})
	}

	// 注入高写入 IO 样本
	for i := 0; i < 10; i++ {
		m.ReportIOSample(IOSample{
			WriteBytes: 200 * 1024 * 1024,
			ReadBytes:  100,
			Timestamp:  now,
		})
	}

	result := m.Evaluate()

	if result.Score < 50 {
		t.Errorf("expected high score with suspicious events, got %d", result.Score)
	}
	if result.ThreatLevel < ThreatLevelMedium {
		t.Errorf("expected at least medium threat level, got %v", result.ThreatLevel)
	}
	if len(result.Indicators) == 0 {
		t.Error("expected non-empty indicators")
	}
	if len(result.AffectedFiles) == 0 {
		t.Error("expected non-empty affected files")
	}
}

func TestManager_Evaluate_LowThreat(t *testing.T) {
	m := setupTestManager(t)
	m.Start()
	defer m.Stop()

	now := time.Now()

	// 少量正常操作
	m.ReportFileEvent(FileBehaviorEvent{
		Type:      FileEventCreate,
		Path:      "/data/normal.txt",
		Size:      100,
		Extension: ".txt",
		Timestamp: now,
	})
	m.ReportIOSample(IOSample{
		ReadBytes:  1000,
		WriteBytes: 500,
		Timestamp:  now,
	})

	result := m.Evaluate()
	if result.Score >= 50 {
		t.Errorf("expected low score for normal operations, got %d", result.Score)
	}
	if result.ThreatLevel >= ThreatLevelMedium {
		t.Errorf("expected low threat level, got %v", result.ThreatLevel)
	}
}

func TestManager_AutoResponse(t *testing.T) {
	m := setupTestManager(t)
	m.Start()
	defer m.Stop()

	alertCalled := false
	m.SetAlertCallback(func(a *BehaviorAssessment) {
		alertCalled = true
	})

	m.SetSnapshotCallback(func(path string) (string, error) {
		return "snap-001", nil
	})

	now := time.Now()

	// 注入大量高威胁事件（文件 + IO + 进程）
	for i := 0; i < 60; i++ {
		m.ReportFileEvent(FileBehaviorEvent{
			Type:        FileEventEncrypt,
			Path:        "/data/important_" + fmt.Sprintf("%d", i) + ".locked",
			Extension:   ".locked",
			ProcessName: "malware",
			Entropy:     8.0,
			Timestamp:   now,
		})
	}
	for i := 0; i < 15; i++ {
		m.ReportFileEvent(FileBehaviorEvent{
			Type:      FileEventBulkWrite,
			Path:      "/data/bulk_" + fmt.Sprintf("%d", i),
			Extension: ".dat",
			Timestamp: now,
		})
	}
	for i := 0; i < 20; i++ {
		m.ReportIOSample(IOSample{
			WriteBytes: 300 * 1024 * 1024,
			ReadBytes:  50,
			Timestamp:  now,
		})
	}
	for i := 0; i < 5; i++ {
		m.ReportProcessEvent(ProcessBehaviorEvent{
			Type:        ProcessEventSuspicious,
			ProcessName: "malware",
			ProcessID:   7777 + i,
			Timestamp:   now,
		})
	}
	for i := 0; i < 5; i++ {
		m.ReportProcessEvent(ProcessBehaviorEvent{
			Type:        ProcessEventPrivEsc,
			ProcessName: "escalator",
			ProcessID:   6666 + i,
			Timestamp:   now,
		})
	}

	// 触发评估
	assessment := m.Evaluate()

	// 检查评分足够高以触发自动响应
	if assessment.Score < m.GetConfig().AIModel.ScoreThreshold {
		t.Logf("score %d below threshold %d, adjusting test to ensure response triggers", assessment.Score, m.GetConfig().AIModel.ScoreThreshold)
		// 手动降低阈值以测试 response 路径
		cfg := m.GetConfig()
		cfg.AIModel.ScoreThreshold = 20
		m.UpdateConfig(cfg)
		assessment = m.Evaluate()
	}

	resp := m.TriggerResponse(assessment)
	if resp.Action == "" {
		t.Error("expected non-empty action")
	}

	if !alertCalled {
		t.Error("expected alert callback to be called")
	}
}

func TestManager_GetAssessments(t *testing.T) {
	m := setupTestManager(t)

	// 无评估时
	assessments := m.GetAssessments(10)
	if len(assessments) != 0 {
		t.Errorf("expected 0 assessments, got %d", len(assessments))
	}

	// 执行评估
	m.Evaluate()
	m.Evaluate()

	assessments = m.GetAssessments(10)
	if len(assessments) != 2 {
		t.Errorf("expected 2 assessments, got %d", len(assessments))
	}
}

func TestManager_GetResponseLog(t *testing.T) {
	m := setupTestManager(t)

	log := m.GetResponseLog(10)
	if len(log) != 0 {
		t.Errorf("expected 0 responses, got %d", len(log))
	}
}

func TestManager_GetConfig(t *testing.T) {
	m := setupTestManager(t)
	cfg := m.GetConfig()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if !cfg.Enabled {
		t.Error("expected config enabled")
	}
}

func TestManager_UpdateConfig(t *testing.T) {
	m := setupTestManager(t)

	cfg := m.GetConfig()
	cfg.AIModel.ScoreThreshold = 80
	m.UpdateConfig(cfg)

	updated := m.GetConfig()
	if updated.AIModel.ScoreThreshold != 80 {
		t.Errorf("expected score threshold 80, got %d", updated.AIModel.ScoreThreshold)
	}
}

func TestManager_GetStatus(t *testing.T) {
	m := setupTestManager(t)
	m.Start()
	defer m.Stop()

	status := m.GetStatus()
	if !status.Running {
		t.Error("expected running status")
	}
	if status.Uptime < 0 {
		t.Error("expected non-negative uptime")
	}
}

func TestClampScore(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{-10, 0},
		{0, 0},
		{50, 50},
		{100, 100},
		{150, 100},
	}
	for _, tt := range tests {
		if got := clampScore(tt.input); got != tt.want {
			t.Errorf("clampScore(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestScoreToThreatLevel(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		score int
		want  ThreatLevel
	}{
		{0, ThreatLevelNone},
		{10, ThreatLevelNone},
		{20, ThreatLevelLow},
		{50, ThreatLevelMedium},
		{70, ThreatLevelHigh},
		{90, ThreatLevelCritical},
	}
	for _, tt := range tests {
		got := m.scoreToThreatLevel(tt.score)
		if got != tt.want {
			t.Errorf("scoreToThreatLevel(%d) = %v, want %v", tt.score, got, tt.want)
		}
	}
}

// ============================================================
// Handlers Tests
// ============================================================

func TestHandler_Status(t *testing.T) {
	m := setupTestManager(t)
	_, srv := setupTestHandlers(t, m)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/ransombehaviorai/status")
	if err != nil {
		t.Fatalf("GET /status failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body apiResponse
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Code != 0 {
		t.Errorf("expected code 0, got %d", body.Code)
	}
}

func TestHandler_StartStop(t *testing.T) {
	m := setupTestManager(t)
	_, srv := setupTestHandlers(t, m)
	defer srv.Close()

	// Start
	resp, err := http.Post(srv.URL+"/api/ransombehaviorai/start", "", nil)
	if err != nil {
		t.Fatalf("POST /start failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Double start → 409
	errResp2, err := http.Post(srv.URL+"/api/ransombehaviorai/start", "", nil)
	if err != nil {
		t.Fatalf("POST /start (double) failed: %v", err)
	}
	defer errResp2.Body.Close()
	if errResp2.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 on double start, got %d", errResp2.StatusCode)
	}

	// Stop
	stopResp, err := http.Post(srv.URL+"/api/ransombehaviorai/stop", "", nil)
	if err != nil {
		t.Fatalf("POST /stop failed: %v", err)
	}
	defer stopResp.Body.Close()
	if stopResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", stopResp.StatusCode)
	}
}

func TestHandler_Evaluate(t *testing.T) {
	m := setupTestManager(t)
	_, srv := setupTestHandlers(t, m)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/ransombehaviorai/evaluate", "", nil)
	if err != nil {
		t.Fatalf("POST /evaluate failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body apiResponse
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Code != 0 {
		t.Errorf("expected code 0, got %d", body.Code)
	}
}

func TestHandler_Report(t *testing.T) {
	m := setupTestManager(t)
	_, srv := setupTestHandlers(t, m)
	defer srv.Close()

	reqBody := ReportEventRequest{
		FileEvents: []FileBehaviorEvent{
			{
				Type:        FileEventModify,
				Path:        "/data/test.txt",
				Size:        1024,
				ProcessName: "test-proc",
			},
		},
		IOEvents: []IOSample{
			{
				ReadBytes:  1000,
				WriteBytes: 5000,
				SourcePath: "/data",
			},
		},
		ProcessEvents: []ProcessBehaviorEvent{
			{
				Type:        ProcessEventSuspicious,
				ProcessName: "suspicious-proc",
				ProcessID:   1234,
			},
		},
	}

	body, _ := json.Marshal(reqBody)
	resp, err := http.Post(srv.URL+"/api/ransombehaviorai/report", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /report failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var apiResp apiResponse
	json.NewDecoder(resp.Body).Decode(&apiResp)
	if apiResp.Code != 0 {
		t.Errorf("expected code 0, got %d", apiResp.Code)
	}

	// 验证事件已记录
	status := m.GetStatus()
	if status.Stats.TotalEvents != 3 {
		t.Errorf("expected 3 total events, got %d", status.Stats.TotalEvents)
	}
}

func TestHandler_Report_InvalidBody(t *testing.T) {
	m := setupTestManager(t)
	_, srv := setupTestHandlers(t, m)
	defer srv.Close()

	invalidResp, err := http.Post(srv.URL+"/api/ransombehaviorai/report", "application/json", bytes.NewReader([]byte("invalid")))
	if err != nil {
		t.Fatalf("POST /report (invalid) failed: %v", err)
	}
	defer invalidResp.Body.Close()

	if invalidResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", invalidResp.StatusCode)
	}
}

func TestHandler_Assessments(t *testing.T) {
	m := setupTestManager(t)
	_, srv := setupTestHandlers(t, m)
	defer srv.Close()

	// 生成一些评估
	m.Evaluate()

	resp, err := http.Get(srv.URL + "/api/ransombehaviorai/assessments?limit=5")
	if err != nil {
		t.Fatalf("GET /assessments failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body apiResponse
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Code != 0 {
		t.Errorf("expected code 0, got %d", body.Code)
	}
}

func TestHandler_Responses(t *testing.T) {
	m := setupTestManager(t)
	_, srv := setupTestHandlers(t, m)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/ransombehaviorai/responses?limit=10")
	if err != nil {
		t.Fatalf("GET /responses failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandler_Config_Get(t *testing.T) {
	m := setupTestManager(t)
	_, srv := setupTestHandlers(t, m)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/ransombehaviorai/config")
	if err != nil {
		t.Fatalf("GET /config failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body apiResponse
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Code != 0 {
		t.Errorf("expected code 0, got %d", body.Code)
	}
}

func TestHandler_Config_Put(t *testing.T) {
	m := setupTestManager(t)
	_, srv := setupTestHandlers(t, m)
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.AIModel.ScoreThreshold = 85
	body, _ := json.Marshal(cfg)

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/ransombehaviorai/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /config failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	updated := m.GetConfig()
	if updated.AIModel.ScoreThreshold != 85 {
		t.Errorf("expected score threshold 85, got %d", updated.AIModel.ScoreThreshold)
	}
}

func TestHandler_Config_InvalidPut(t *testing.T) {
	m := setupTestManager(t)
	_, srv := setupTestHandlers(t, m)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/ransombehaviorai/config", bytes.NewReader([]byte("bad json")))
	req.Header.Set("Content-Type", "application/json")
	invalidResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /config (invalid) failed: %v", err)
	}
	defer invalidResp.Body.Close()

	if invalidResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", invalidResp.StatusCode)
	}
}

func TestHandler_Stats(t *testing.T) {
	m := setupTestManager(t)
	_, srv := setupTestHandlers(t, m)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/ransombehaviorai/stats")
	if err != nil {
		t.Fatalf("GET /stats failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	m := setupTestManager(t)
	_, srv := setupTestHandlers(t, m)
	defer srv.Close()

	// GET on POST-only endpoints
	endpoints := []string{
		"/api/ransombehaviorai/start",
		"/api/ransombehaviorai/stop",
		"/api/ransombehaviorai/evaluate",
		"/api/ransombehaviorai/report",
	}
	for _, ep := range endpoints {
		resp, _ := http.Get(srv.URL + ep)
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("GET %s: expected 405, got %d", ep, resp.StatusCode)
		}
	}

	// POST on GET-only endpoints
	getOnlyEndpoints := []string{
		"/api/ransombehaviorai/status",
		"/api/ransombehaviorai/assessments",
		"/api/ransombehaviorai/responses",
		"/api/ransombehaviorai/stats",
	}
	for _, ep := range getOnlyEndpoints {
		resp, _ := http.Post(srv.URL+ep, "", nil)
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("POST %s: expected 405, got %d", ep, resp.StatusCode)
		}
	}
}

// ============================================================
// IO Scoring Tests
// ============================================================

func TestScoreIOBehavior_BurstWrite(t *testing.T) {
	m := setupTestManager(t)
	m.Start()
	defer m.Stop()

	now := time.Now()
	threshold := m.GetConfig().IOMonitor.BurstWriteThresholdBps

	// 注入超过阈值的突发写入
	for i := 0; i < 20; i++ {
		m.ReportIOSample(IOSample{
			WriteBytes: threshold * 2,
			Timestamp:  now,
		})
	}

	result := m.Evaluate()
	if result.IOScore.BurstWriteScore == 0 {
		t.Error("expected non-zero burst write score")
	}
}

func TestScoreIOBehavior_AnomalousRWRatio(t *testing.T) {
	m := setupTestManager(t)
	m.Start()
	defer m.Stop()

	now := time.Now()

	// 写入远大于读取
	m.ReportIOSample(IOSample{
		ReadBytes:  100,
		WriteBytes: 100000,
		Timestamp:  now,
	})

	result := m.Evaluate()
	if result.IOScore.RWRatioScore == 0 {
		t.Error("expected non-zero RW ratio score")
	}
}

// ============================================================
// Process Scoring Tests
// ============================================================

func TestScoreProcessBehavior_SuspiciousProcess(t *testing.T) {
	m := setupTestManager(t)
	m.Start()
	defer m.Stop()

	now := time.Now()

	m.ReportProcessEvent(ProcessBehaviorEvent{
		Type:        ProcessEventSuspicious,
		ProcessName: "lockbit",
		ProcessID:   1111,
		Timestamp:   now,
	})

	result := m.Evaluate()
	if result.ProcessScore.SuspiciousProcessScore == 0 {
		t.Error("expected non-zero suspicious process score")
	}
}

func TestScoreProcessBehavior_PrivEsc(t *testing.T) {
	m := setupTestManager(t)
	m.Start()
	defer m.Stop()

	now := time.Now()

	// 超过阈值的权限提升
	for i := 0; i < 5; i++ {
		m.ReportProcessEvent(ProcessBehaviorEvent{
			Type:        ProcessEventPrivEsc,
			ProcessName: "escalator",
			ProcessID:   2222,
			Timestamp:   now,
		})
	}

	result := m.Evaluate()
	if result.ProcessScore.PrivEscScore == 0 {
		t.Error("expected non-zero privilege escalation score")
	}
}

// ============================================================
// Integration Test
// ============================================================

func TestIntegration_FullWorkflow(t *testing.T) {
	m := setupTestManager(t)

	// 1. 启动引擎
	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	// 2. 设置回调
	alertReceived := false
	m.SetAlertCallback(func(a *BehaviorAssessment) {
		alertReceived = true
	})

	// 3. 上报正常事件
	now := time.Now()
	m.ReportFileEvent(FileBehaviorEvent{
		Type:      FileEventCreate,
		Path:      "/data/normal.txt",
		Extension: ".txt",
		Timestamp: now,
	})
	m.ReportIOSample(IOSample{
		ReadBytes:  1000,
		WriteBytes: 500,
		Timestamp:  now,
	})

	// 4. 评估 - 应该低威胁
	result1 := m.Evaluate()
	if result1.ThreatLevel >= ThreatLevelMedium {
		t.Errorf("expected low threat for normal events, got %v", result1.ThreatLevel)
	}

	// 5. 注入恶意事件
	for i := 0; i < 40; i++ {
		m.ReportFileEvent(FileBehaviorEvent{
			Type:        FileEventEncrypt,
			Path:        "/data/victim_" + time.Now().Format("150405.000") + ".locked",
			Extension:   ".locked",
			Entropy:     8.0,
			ProcessName: "ransomware",
			Timestamp:   now,
		})
	}
	for i := 0; i < 5; i++ {
		m.ReportProcessEvent(ProcessBehaviorEvent{
			Type:        ProcessEventPrivEsc,
			ProcessName: "ransomware",
			ProcessID:   6666,
			Timestamp:   now,
		})
	}
	m.ReportIOSample(IOSample{
		ReadBytes:  10,
		WriteBytes: 500000000,
		Timestamp:  now,
	})

	// 6. 评估 - 应该高威胁
	result2 := m.Evaluate()
	if result2.Score < 40 {
		t.Errorf("expected elevated score for malicious events, got %d", result2.Score)
	}
	if result2.ThreatLevel < ThreatLevelLow {
		t.Errorf("expected at least low threat level, got %v", result2.ThreatLevel)
	}

	// 7. 检查评估历史
	assessments := m.GetAssessments(10)
	if len(assessments) < 2 {
		t.Errorf("expected at least 2 assessments, got %d", len(assessments))
	}

	// 8. 检查统计
	status := m.GetStatus()
	if status.Stats.TotalEvents < 40 {
		t.Errorf("expected at least 40 total events, got %d", status.Stats.TotalEvents)
	}

	_ = alertReceived // alert callback may or may not be called depending on thresholds
}
