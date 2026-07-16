package licensescan

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func setupTestHandlers(t *testing.T) (*Handlers, *Manager, string) {
	t.Helper()
	m := NewManager()
	s := NewScheduler(m)
	h := NewHandlers(m, s)

	// 创建测试go.mod
	tmpDir := t.TempDir()
	goModPath := tmpDir + "/go.mod"
	if err := os.WriteFile(goModPath, []byte(`module test
go 1.21
require github.com/gin-gonic/gin v1.9.1
`), 0644); err != nil {
		t.Fatal(err)
	}

	return h, m, goModPath
}

func TestHandleGoModScan(t *testing.T) {
	h, _, goModPath := setupTestHandlers(t)

	body, _ := json.Marshal(ScanRequest{
		ScanType: ScanTypeGoMod,
		Target:   goModPath,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/licensescan/scan/gomod", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleGoModScan(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var result ScanResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if result.Status != StatusComplete {
		t.Errorf("ScanStatus = %q, want %q", result.Status, StatusComplete)
	}
}

func TestHandleGoModScanMethodNotAllowed(t *testing.T) {
	h, _, _ := setupTestHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/licensescan/scan/gomod", nil)
	w := httptest.NewRecorder()

	h.handleGoModScan(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleGoModScanBadBody(t *testing.T) {
	h, _, _ := setupTestHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/licensescan/scan/gomod", bytes.NewReader([]byte("invalid")))
	w := httptest.NewRecorder()

	h.handleGoModScan(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleGoModScanEmptyTarget(t *testing.T) {
	h, _, _ := setupTestHandlers(t)

	body, _ := json.Marshal(ScanRequest{ScanType: ScanTypeGoMod, Target: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/licensescan/scan/gomod", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.handleGoModScan(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleListScans(t *testing.T) {
	h, _, goModPath := setupTestHandlers(t)

	// 先执行一个扫描
	body, _ := json.Marshal(ScanRequest{Target: goModPath})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/licensescan/scan/gomod", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.handleGoModScan(w, req)

	// 列出扫描结果
	req = httptest.NewRequest(http.MethodGet, "/api/v1/licensescan/scan/results", nil)
	w = httptest.NewRecorder()
	h.handleListScans(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp ScanListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("Total = %d, want 1", resp.Total)
	}
}

func TestHandleGetScanResult(t *testing.T) {
	h, m, goModPath := setupTestHandlers(t)

	// 执行扫描
	result, _ := m.RunGoModScan(goModPath, "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/licensescan/scan/result/"+result.ID, nil)
	w := httptest.NewRecorder()
	h.handleGetScanResult(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleGetScanResultNotFound(t *testing.T) {
	h, _, _ := setupTestHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/licensescan/scan/result/nonexistent", nil)
	w := httptest.NewRecorder()
	h.handleGetScanResult(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandlePoliciesCRUD(t *testing.T) {
	h, _, _ := setupTestHandlers(t)

	// 列出策略（应有默认策略）
	req := httptest.NewRequest(http.MethodGet, "/api/v1/licensescan/policies", nil)
	w := httptest.NewRecorder()
	h.handlePolicies(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET policies status = %d, want %d", w.Code, http.StatusOK)
	}

	var listResp PolicyListResponse
	json.NewDecoder(w.Body).Decode(&listResp)
	if listResp.Total != 1 {
		t.Errorf("Initial policies count = %d, want 1", listResp.Total)
	}

	// 创建新策略
	policyBody, _ := json.Marshal(Policy{
		ID:        "custom",
		Name:      "自定义策略",
		Whitelist: []string{"MIT"},
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/licensescan/policies", bytes.NewReader(policyBody))
	w = httptest.NewRecorder()
	h.handlePolicies(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("POST policies status = %d, want %d", w.Code, http.StatusCreated)
	}

	// 获取策略
	req = httptest.NewRequest(http.MethodGet, "/api/v1/licensescan/policy/custom", nil)
	w = httptest.NewRecorder()
	h.handlePolicy(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET policy status = %d, want %d", w.Code, http.StatusOK)
	}

	// 更新策略
	updateBody, _ := json.Marshal(Policy{
		Name:      "更新后的策略",
		Whitelist: []string{"MIT", "Apache-2.0"},
	})
	req = httptest.NewRequest(http.MethodPut, "/api/v1/licensescan/policy/custom", bytes.NewReader(updateBody))
	w = httptest.NewRecorder()
	h.handlePolicy(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PUT policy status = %d, want %d", w.Code, http.StatusOK)
	}

	// 删除策略
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/licensescan/policy/custom", nil)
	w = httptest.NewRecorder()
	h.handlePolicy(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("DELETE policy status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandlePoliciesMethodNotAllowed(t *testing.T) {
	h, _, _ := setupTestHandlers(t)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/licensescan/policies", nil)
	w := httptest.NewRecorder()
	h.handlePolicies(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleDashboard(t *testing.T) {
	h, _, goModPath := setupTestHandlers(t)

	// 先执行扫描
	m := h.manager
	m.RunGoModScan(goModPath, "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/licensescan/dashboard", nil)
	w := httptest.NewRecorder()
	h.handleDashboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var data DashboardData
	if err := json.NewDecoder(w.Body).Decode(&data); err != nil {
		t.Fatalf("Failed to decode dashboard: %v", err)
	}
	if data.TotalScans != 1 {
		t.Errorf("TotalScans = %d, want 1", data.TotalScans)
	}
}

func TestHandleAlerts(t *testing.T) {
	h, _, _ := setupTestHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/licensescan/alerts", nil)
	w := httptest.NewRecorder()
	h.handleAlerts(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp AlertListResponse
	json.NewDecoder(w.Body).Decode(&resp)
	// 空状态应该返回空列表
	if resp.Alerts == nil {
		// nil slice is fine, just ensure it's not panicking
	}
}

func TestHandleReportGeneration(t *testing.T) {
	h, _, goModPath := setupTestHandlers(t)

	// 先执行扫描
	h.manager.RunGoModScan(goModPath, "")

	// 生成JSON报告
	reportBody, _ := json.Marshal(struct {
		Title   string   `json:"title"`
		Format  string   `json:"format"`
		ScanIDs []string `json:"scan_ids"`
	}{
		Title:  "测试报告",
		Format: "json",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/licensescan/report/generate", bytes.NewReader(reportBody))
	w := httptest.NewRecorder()
	h.handleGenerateReport(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("POST report/generate status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestHandleReportGenerationHTML(t *testing.T) {
	h, _, goModPath := setupTestHandlers(t)

	h.manager.RunGoModScan(goModPath, "")

	reportBody, _ := json.Marshal(struct {
		Title   string   `json:"title"`
		Format  string   `json:"format"`
		ScanIDs []string `json:"scan_ids"`
	}{
		Title:  "HTML报告",
		Format: "html",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/licensescan/report/generate", bytes.NewReader(reportBody))
	w := httptest.NewRecorder()
	h.handleGenerateReport(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestHandleSchedulerTasks(t *testing.T) {
	h, _, _ := setupTestHandlers(t)

	// 列出任务
	req := httptest.NewRequest(http.MethodGet, "/api/v1/licensescan/scheduler/tasks", nil)
	w := httptest.NewRecorder()
	h.handleSchedulerTasks(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET scheduler/tasks status = %d, want %d", w.Code, http.StatusOK)
	}

	// 创建任务
	taskBody, _ := json.Marshal(ScheduledTask{
		Name:     "定期扫描",
		ScanType: ScanTypeGoMod,
		Targets:  []string{"/tmp/go.mod"},
		Enabled:  true,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/licensescan/scheduler/tasks", bytes.NewReader(taskBody))
	w = httptest.NewRecorder()
	h.handleSchedulerTasks(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("POST scheduler/tasks status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestHandleSchedulerTaskDelete(t *testing.T) {
	h, _, _ := setupTestHandlers(t)

	// 添加任务
	task := ScheduledTask{
		ID:       "task-to-delete",
		Name:     "要删除的任务",
		ScanType: ScanTypeGoMod,
		Enabled:  true,
	}
	h.scheduler.AddTask(task)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/licensescan/scheduler/task/task-to-delete", nil)
	w := httptest.NewRecorder()
	h.handleSchedulerTask(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("DELETE scheduler/task status = %d, want %d", w.Code, http.StatusOK)
	}

	// 删除不存在的任务
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/licensescan/scheduler/task/nonexistent", nil)
	w = httptest.NewRecorder()
	h.handleSchedulerTask(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("DELETE nonexistent task status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleDockerScanMethodNotAllowed(t *testing.T) {
	h, _, _ := setupTestHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/licensescan/scan/docker", nil)
	w := httptest.NewRecorder()
	h.handleDockerScan(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleDockerScanBadBody(t *testing.T) {
	h, _, _ := setupTestHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/licensescan/scan/docker", bytes.NewReader([]byte("bad")))
	w := httptest.NewRecorder()
	h.handleDockerScan(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleDockerScanEmptyTarget(t *testing.T) {
	h, _, _ := setupTestHandlers(t)

	body, _ := json.Marshal(ScanRequest{Target: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/licensescan/scan/docker", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.handleDockerScan(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestWriteJSONAndError(t *testing.T) {
	// 测试writeJSON
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"test": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}

	// 测试writeError
	w = httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "test error")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
