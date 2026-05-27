package diskspace

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setupTestHandler() *Handler {
	manager := NewDiskSpaceManager()
	return NewHandler(manager)
}

func TestStartScan(t *testing.T) {
	handler := setupTestHandler()
	
	reqBody := `{"path": "/home", "config": {"max_depth": 3}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/diskspace/scan", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	
	handler.handleScan(w, req)
	
	if w.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d", http.StatusAccepted, w.Code)
	}
	
	var resp SuccessResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	
	if !resp.Success {
		t.Error("expected success to be true")
	}
}

func TestGetScanProgress(t *testing.T) {
	handler := setupTestHandler()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/diskspace/scan/progress", nil)
	w := httptest.NewRecorder()
	
	handler.handleGetScanProgress(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	
	var progress ScanProgress
	if err := json.NewDecoder(w.Body).Decode(&progress); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	
	// Status should be empty initially
	if progress.Status != "" {
		t.Errorf("expected empty status, got '%s'", progress.Status)
	}
}

func TestGetDiskUsage(t *testing.T) {
	handler := setupTestHandler()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/diskspace/usage?path=/home", nil)
	w := httptest.NewRecorder()
	
	handler.handleGetDiskUsage(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	
	var usage DiskUsage
	if err := json.NewDecoder(w.Body).Decode(&usage); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	
	if usage.Total <= 0 {
		t.Error("expected total to be greater than 0")
	}
	if usage.Used <= 0 {
		t.Error("expected used to be greater than 0")
	}
	if usage.Free <= 0 {
		t.Error("expected free to be greater than 0")
	}
	if usage.UsagePercent <= 0 || usage.UsagePercent > 100 {
		t.Errorf("expected usage percent between 0 and 100, got %f", usage.UsagePercent)
	}
}

func TestGetDirectoryTree(t *testing.T) {
	handler := setupTestHandler()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/diskspace/tree?path=/home&max_depth=2", nil)
	w := httptest.NewRecorder()
	
	handler.handleGetDirectoryTree(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	
	var tree DirectoryNode
	if err := json.NewDecoder(w.Body).Decode(&tree); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	
	if tree.Path != "/home" {
		t.Errorf("expected path '/home', got '%s'", tree.Path)
	}
	if tree.Name != "home" {
		t.Errorf("expected name 'home', got '%s'", tree.Name)
	}
	if len(tree.Children) == 0 {
		t.Error("expected children to be non-empty")
	}
	if tree.Size <= 0 {
		t.Error("expected size to be greater than 0")
	}
}

func TestGetFileTypeStats(t *testing.T) {
	handler := setupTestHandler()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/diskspace/filetypes?path=/home", nil)
	w := httptest.NewRecorder()
	
	handler.handleGetFileTypeStats(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	
	var stats []FileTypeStats
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	
	if len(stats) == 0 {
		t.Error("expected stats to be non-empty")
	}
	
	for _, stat := range stats {
		if stat.Extension == "" {
			t.Error("expected extension to be non-empty")
		}
		if stat.Count <= 0 {
			t.Errorf("expected count to be greater than 0 for extension '%s'", stat.Extension)
		}
		if stat.TotalSize <= 0 {
			t.Errorf("expected total size to be greater than 0 for extension '%s'", stat.Extension)
		}
	}
}

func TestFindLargeFiles(t *testing.T) {
	handler := setupTestHandler()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/diskspace/large-files?path=/home&min_size=1048576&limit=5", nil)
	w := httptest.NewRecorder()
	
	handler.handleFindLargeFiles(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	
	var files []LargeFileInfo
	if err := json.NewDecoder(w.Body).Decode(&files); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	
	if len(files) == 0 {
		t.Error("expected files to be non-empty")
	}
	
	for _, file := range files {
		if file.Path == "" {
			t.Error("expected path to be non-empty")
		}
		if file.Size < 1048576 {
			t.Errorf("expected file size to be at least 1MB, got %d", file.Size)
		}
		if file.Extension == "" {
			t.Error("expected extension to be non-empty")
		}
	}
}

func TestFindDuplicates(t *testing.T) {
	handler := setupTestHandler()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/diskspace/duplicates?path=/home", nil)
	w := httptest.NewRecorder()
	
	handler.handleFindDuplicates(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	
	var duplicates []DuplicateFile
	if err := json.NewDecoder(w.Body).Decode(&duplicates); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	
	if len(duplicates) == 0 {
		t.Error("expected duplicates to be non-empty")
	}
	
	for _, dup := range duplicates {
		if dup.Hash == "" {
			t.Error("expected hash to be non-empty")
		}
		if len(dup.Files) < 2 {
			t.Error("expected at least 2 files in duplicate group")
		}
		if dup.TotalWastedSpace <= 0 {
			t.Error("expected total wasted space to be greater than 0")
		}
	}
}

func TestGetTreemapData(t *testing.T) {
	handler := setupTestHandler()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/diskspace/treemap?path=/home&max_depth=2", nil)
	w := httptest.NewRecorder()
	
	handler.handleGetTreemapData(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	
	var treemap TreemapData
	if err := json.NewDecoder(w.Body).Decode(&treemap); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	
	if treemap.Name == "" {
		t.Error("expected name to be non-empty")
	}
	if treemap.Path == "" {
		t.Error("expected path to be non-empty")
	}
	if treemap.Size <= 0 {
		t.Error("expected size to be greater than 0")
	}
	if len(treemap.Children) == 0 {
		t.Error("expected children to be non-empty")
	}
	if treemap.Color == "" {
		t.Error("expected color to be non-empty")
	}
}

func TestGetGrowthTrend(t *testing.T) {
	handler := setupTestHandler()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/diskspace/growth?days=7", nil)
	w := httptest.NewRecorder()
	
	handler.handleGetGrowthTrend(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	
	var trend []GrowthTrend
	if err := json.NewDecoder(w.Body).Decode(&trend); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	
	if len(trend) != 7 {
		t.Errorf("expected 7 trend entries, got %d", len(trend))
	}
	
	for _, entry := range trend {
		if entry.UsedSpace <= 0 {
			t.Error("expected used space to be greater than 0")
		}
		if entry.FileCount <= 0 {
			t.Error("expected file count to be greater than 0")
		}
	}
}

func TestExportReportJSON(t *testing.T) {
	handler := setupTestHandler()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/diskspace/export?format=json", nil)
	w := httptest.NewRecorder()
	
	handler.handleExportReport(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected content type 'application/json', got '%s'", w.Header().Get("Content-Type"))
	}
	
	var report map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	
	if report["generated_at"] == nil {
		t.Error("expected generated_at to be present")
	}
	if report["summary"] == nil {
		t.Error("expected summary to be present")
	}
}

func TestExportReportText(t *testing.T) {
	handler := setupTestHandler()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/diskspace/export?format=text", nil)
	w := httptest.NewRecorder()
	
	handler.handleExportReport(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	
	if w.Header().Get("Content-Type") != "text/plain" {
		t.Errorf("expected content type 'text/plain', got '%s'", w.Header().Get("Content-Type"))
	}
	
	body := w.Body.String()
	if !strings.Contains(body, "Disk Usage Report") {
		t.Error("expected report to contain 'Disk Usage Report'")
	}
	if !strings.Contains(body, "Total Space") {
		t.Error("expected report to contain 'Total Space'")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	handler := setupTestHandler()
	
	// Test POST on GET-only endpoint
	req := httptest.NewRequest(http.MethodPost, "/api/v1/diskspace/usage", nil)
	w := httptest.NewRecorder()
	
	handler.handleGetDiskUsage(w, req)
	
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestMissingPath(t *testing.T) {
	handler := setupTestHandler()
	
	reqBody := `{"config": {"max_depth": 3}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/diskspace/scan", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	
	handler.handleScan(w, req)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	
	var errResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	
	if !strings.Contains(errResp.Message, "path is required") {
		t.Errorf("expected error message to contain 'path is required', got '%s'", errResp.Message)
	}
}
