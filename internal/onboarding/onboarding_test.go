package onboarding

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupRouter() (*gin.Engine, *Onboarding) {
	gin.SetMode(gin.TestMode)
	ob := NewOnboarding()
	h := NewHandler(ob)
	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)
	return r, ob
}

func TestOnboardingLifecycle(t *testing.T) {
	ob := NewOnboarding()

	// 初始状态应该是 not_started
	if state := ob.GetState(); state != StateNotStarted {
		t.Errorf("expected state not_started, got %s", state)
	}

	// 开始引导
	if err := ob.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if state := ob.GetState(); state != StateInProgress {
		t.Errorf("expected state in_progress, got %s", state)
	}

	// 重复开始应报错
	if err := ob.Start(); err == nil {
		t.Error("expected error for duplicate start")
	}

	// 按顺序完成所有步骤
	stepIDs := []string{"storage_pool", "network_config", "user_creation", "share_setup", "app_install"}
	for _, id := range stepIDs {
		if err := ob.CompleteStep(id); err != nil {
			t.Fatalf("complete step %s failed: %v", id, err)
		}
	}

	// 完成后状态应为 completed
	if state := ob.GetState(); state != StateCompleted {
		t.Errorf("expected state completed, got %s", state)
	}

	// 检查统计
	stats := ob.GetCompletionStats()
	if stats.TotalCompleted != 1 {
		t.Errorf("expected 1 total completed, got %d", stats.TotalCompleted)
	}
}

func TestPreConditionCheck(t *testing.T) {
	ob := NewOnboarding()
	ob.Start()

	// 跳过 storage_pool 直接完成 network_config，应该失败
	err := ob.CompleteStep("network_config")
	if err == nil {
		t.Error("expected precondition failure for network_config without storage_pool")
	}

	// 先完成 storage_pool
	if err := ob.CompleteStep("storage_pool"); err != nil {
		t.Fatalf("complete storage_pool failed: %v", err)
	}

	// 现在可以完成 network_config
	if err := ob.CompleteStep("network_config"); err != nil {
		t.Fatalf("complete network_config failed: %v", err)
	}

	// 跳过 user_creation 直接完成 share_setup，应该失败
	err = ob.CompleteStep("share_setup")
	if err == nil {
		t.Error("expected precondition failure for share_setup without user_creation")
	}
}

func TestSkipAndReset(t *testing.T) {
	ob := NewOnboarding()
	ob.Start()

	// 跳过引导
	if err := ob.Skip(); err != nil {
		t.Fatalf("skip failed: %v", err)
	}
	if state := ob.GetState(); state != StateSkipped {
		t.Errorf("expected state skipped, got %s", state)
	}

	// 重置
	ob.Reset()
	if state := ob.GetState(); state != StateNotStarted {
		t.Errorf("expected state not_started after reset, got %s", state)
	}

	// 重置后可以重新开始
	if err := ob.Start(); err != nil {
		t.Fatalf("start after reset failed: %v", err)
	}
}

func TestTutorialsAndQuickStart(t *testing.T) {
	ob := NewOnboarding()

	// 获取快速入门卡片
	cards := ob.GetQuickStartCards()
	if len(cards) < 4 {
		t.Errorf("expected at least 4 quick start cards, got %d", len(cards))
	}

	// 获取教程列表
	tutorials := ob.GetTutorials()
	if len(tutorials) < 4 {
		t.Errorf("expected at least 4 tutorials, got %d", len(tutorials))
	}

	// 获取特定教程
	tutorial, err := ob.GetTutorial("smb_share")
	if err != nil {
		t.Fatalf("get tutorial smb_share failed: %v", err)
	}
	if tutorial.Title != "SMB共享入门" {
		t.Errorf("expected title 'SMB共享入门', got %s", tutorial.Title)
	}
	if len(tutorial.Steps) == 0 {
		t.Error("expected tutorial steps")
	}

	// 不存在的教程
	_, err = ob.GetTutorial("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent tutorial")
	}
}

func TestHandlerStatusAPI(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/onboarding/status", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["state"] != string(StateNotStarted) {
		t.Errorf("expected state not_started, got %v", resp["state"])
	}
}

func TestHandlerStartAndCompleteAPI(t *testing.T) {
	r, _ := setupRouter()

	// Start
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/onboarding/start", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("start: expected 200, got %d", w.Code)
	}

	// Complete storage_pool
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/onboarding/complete/storage_pool", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("complete: expected 200, got %d", w.Code)
	}

	// Steps
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/onboarding/steps", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("steps: expected 200, got %d", w.Code)
	}
	var steps []Step
	json.Unmarshal(w.Body.Bytes(), &steps)
	if len(steps) != 5 {
		t.Errorf("expected 5 steps, got %d", len(steps))
	}
}

func TestHandlerTutorialsAPI(t *testing.T) {
	r, _ := setupRouter()

	// Tutorials list
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/onboarding/tutorials", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("tutorials: expected 200, got %d", w.Code)
	}

	// Tutorial detail
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/onboarding/tutorials/docker_app", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("tutorial detail: expected 200, got %d", w.Code)
	}

	// Not found
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/onboarding/tutorials/nonexistent", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("not found: expected 404, got %d", w.Code)
	}
}

func TestHandlerQuickStartAPI(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/onboarding/quickstart", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("quickstart: expected 200, got %d", w.Code)
	}
	var cards []QuickStartCard
	json.Unmarshal(w.Body.Bytes(), &cards)
	if len(cards) < 4 {
		t.Errorf("expected at least 4 cards, got %d", len(cards))
	}
}

func TestHandlerResetAPI(t *testing.T) {
	r, _ := setupRouter()

	// Start first
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/onboarding/start", nil)
	r.ServeHTTP(w, req)

	// Reset
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/onboarding/reset", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("reset: expected 200, got %d", w.Code)
	}

	// Verify state
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/onboarding/status", nil)
	r.ServeHTTP(w, req)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["state"] != string(StateNotStarted) {
		t.Errorf("after reset expected not_started, got %v", resp["state"])
	}
}

func TestHandlerSkipAPI(t *testing.T) {
	r, _ := setupRouter()

	// Start
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/onboarding/start", nil)
	r.ServeHTTP(w, req)

	// Skip
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/onboarding/skip", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("skip: expected 200, got %d", w.Code)
	}
}

func TestHandlerStartWithBody(t *testing.T) {
	r, _ := setupRouter()
	body, _ := json.Marshal(map[string]string{"key": "value"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/onboarding/start", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
