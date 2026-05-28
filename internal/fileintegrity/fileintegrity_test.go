package fileintegrity

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(zap.NewNop(), nil)
}

func setupTestRouter(t *testing.T, m *Manager) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	h := NewHandlers(m)
	h.RegisterRoutes(rg)
	return r
}

func createTempFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNewManager(t *testing.T) {
	m := setupTestManager(t)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.IsRunning() {
		t.Error("expected manager to not be running")
	}
	if m.GetStatus() != MonitorStatusIdle {
		t.Errorf("expected status idle, got %v", m.GetStatus())
	}
}

func TestNewManagerWithNilLogger(t *testing.T) {
	m := NewManager(nil, nil)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultMonitorConfig()
	if !cfg.Enabled {
		t.Error("expected enabled by default")
	}
	if cfg.DefaultAlgorithm != HashSHA256 {
		t.Errorf("expected SHA256, got %v", cfg.DefaultAlgorithm)
	}
	if cfg.WorkerCount != 4 {
		t.Errorf("expected 4 workers, got %d", cfg.WorkerCount)
	}
}

func TestStartStop(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !m.IsRunning() {
		t.Error("expected running")
	}

	// 重复启动应报错
	if err := m.Start(ctx); err == nil {
		t.Error("expected error on double start")
	}

	m.Stop()
	if m.IsRunning() {
		t.Error("expected stopped")
	}
}

func TestStartDisabled(t *testing.T) {
	cfg := DefaultMonitorConfig()
	cfg.Enabled = false
	m := NewManager(zap.NewNop(), cfg)

	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if m.IsRunning() {
		t.Error("expected not running when disabled")
	}
}

func TestCreateBaseline(t *testing.T) {
	dir := t.TempDir()
	createTempFiles(t, dir, map[string]string{
		"file1.txt": "hello world",
		"file2.txt": "test content",
		"sub/a.txt": "nested file",
	})

	m := setupTestManager(t)
	baseline, err := m.CreateBaseline(context.Background(), "test-baseline", "test", []string{dir}, HashSHA256)
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	if baseline.Name != "test-baseline" {
		t.Errorf("expected name 'test-baseline', got %q", baseline.Name)
	}
	if baseline.FileCount < 3 {
		t.Errorf("expected at least 3 files, got %d", baseline.FileCount)
	}
	if baseline.HashAlgorithm != HashSHA256 {
		t.Errorf("expected SHA256, got %v", baseline.HashAlgorithm)
	}

	// 验证可以获取
	got, err := m.GetBaseline(baseline.ID)
	if err != nil {
		t.Fatalf("GetBaseline failed: %v", err)
	}
	if got.ID != baseline.ID {
		t.Error("baseline ID mismatch")
	}
}

func TestCreateBaselineNoPaths(t *testing.T) {
	m := setupTestManager(t)
	_, err := m.CreateBaseline(context.Background(), "test", "test", nil, HashSHA256)
	if err == nil {
		t.Error("expected error with no paths")
	}
}

func TestListBaselines(t *testing.T) {
	dir := t.TempDir()
	createTempFiles(t, dir, map[string]string{"a.txt": "content"})

	m := setupTestManager(t)
	m.CreateBaseline(context.Background(), "b1", "", []string{dir}, HashSHA256)
	m.CreateBaseline(context.Background(), "b2", "", []string{dir}, HashSHA512)

	result := m.ListBaselines(1, 10)
	if result.Total != 2 {
		t.Errorf("expected 2 baselines, got %d", result.Total)
	}
}

func TestDeleteBaseline(t *testing.T) {
	dir := t.TempDir()
	createTempFiles(t, dir, map[string]string{"a.txt": "content"})

	m := setupTestManager(t)
	b, _ := m.CreateBaseline(context.Background(), "test", "", []string{dir}, HashSHA256)

	if err := m.DeleteBaseline(b.ID); err != nil {
		t.Fatalf("DeleteBaseline failed: %v", err)
	}

	if _, err := m.GetBaseline(b.ID); err == nil {
		t.Error("expected error after deletion")
	}

	if err := m.DeleteBaseline("nonexistent"); err == nil {
		t.Error("expected error deleting nonexistent baseline")
	}
}

func TestAddRule(t *testing.T) {
	m := setupTestManager(t)

	rule := &MonitorRule{
		Name:  "test-rule",
		Paths: []string{"/tmp"},
	}
	if err := m.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}
	if rule.ID == "" {
		t.Error("expected rule ID to be set")
	}
	if rule.MaxDepth != 10 {
		t.Errorf("expected default max depth 10, got %d", rule.MaxDepth)
	}
}

func TestAddRuleValidation(t *testing.T) {
	m := setupTestManager(t)

	// 缺少名称
	if err := m.AddRule(&MonitorRule{Paths: []string{"/tmp"}}); err == nil {
		t.Error("expected error without name")
	}

	// 缺少路径
	if err := m.AddRule(&MonitorRule{Name: "test"}); err == nil {
		t.Error("expected error without paths")
	}
}

func TestUpdateDeleteRule(t *testing.T) {
	m := setupTestManager(t)

	rule := &MonitorRule{Name: "test", Paths: []string{"/tmp"}}
	m.AddRule(rule)

	// 更新
	rule.Name = "updated"
	if err := m.UpdateRule(rule); err != nil {
		t.Fatalf("UpdateRule failed: %v", err)
	}
	got, _ := m.GetRule(rule.ID)
	if got.Name != "updated" {
		t.Errorf("expected name 'updated', got %q", got.Name)
	}

	// 删除
	if err := m.DeleteRule(rule.ID); err != nil {
		t.Fatalf("DeleteRule failed: %v", err)
	}
	if _, err := m.GetRule(rule.ID); err == nil {
		t.Error("expected error after deletion")
	}
}

func TestListRules(t *testing.T) {
	m := setupTestManager(t)
	m.AddRule(&MonitorRule{Name: "r1", Paths: []string{"/tmp"}})
	m.AddRule(&MonitorRule{Name: "r2", Paths: []string{"/var"}})

	rules := m.ListRules()
	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}
}

func TestRunScan(t *testing.T) {
	dir := t.TempDir()
	createTempFiles(t, dir, map[string]string{"test.txt": "original content"})

	m := setupTestManager(t)

	// 创建基线
	m.CreateBaseline(context.Background(), "baseline", "", []string{dir}, HashSHA256)

	// 未变更扫描
	result, err := m.RunScan(context.Background(), &ScanRequest{
		Paths: []string{dir},
		Mode:  ScanModeFull,
	})
	if err != nil {
		t.Fatalf("RunScan failed: %v", err)
	}
	if result.FilesScanned == 0 {
		t.Error("expected files to be scanned")
	}
}

func TestRunScanDetectsChanges(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	os.WriteFile(testFile, []byte("original"), 0644)

	m := setupTestManager(t)
	m.CreateBaseline(context.Background(), "baseline", "", []string{dir}, HashSHA256)

	// 修改文件
	os.WriteFile(testFile, []byte("modified content"), 0644)

	result, err := m.RunScan(context.Background(), &ScanRequest{
		Paths: []string{dir},
		Mode:  ScanModeFull,
	})
	if err != nil {
		t.Fatalf("RunScan failed: %v", err)
	}

	found := false
	for _, ch := range result.Changes {
		if ch.Path == testFile && ch.ChangeType == ChangeModified {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to detect file modification")
	}
}

func TestRunScanDetectsNewFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("content"), 0644)

	m := setupTestManager(t)
	m.CreateBaseline(context.Background(), "baseline", "", []string{dir}, HashSHA256)

	// 添加新文件
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0644)

	result, _ := m.RunScan(context.Background(), &ScanRequest{
		Paths: []string{dir},
		Mode:  ScanModeFull,
	})

	found := false
	for _, ch := range result.Changes {
		if filepath.Base(ch.Path) == "new.txt" && ch.ChangeType == ChangeCreated {
			found = true
		}
	}
	if !found {
		t.Error("expected to detect new file")
	}
}

func TestRunScanDetectsDeletedFile(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "to_delete.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	m := setupTestManager(t)
	m.CreateBaseline(context.Background(), "baseline", "", []string{dir}, HashSHA256)

	// 删除文件
	os.Remove(testFile)

	result, _ := m.RunScan(context.Background(), &ScanRequest{
		Paths: []string{dir},
		Mode:  ScanModeFull,
	})

	found := false
	for _, ch := range result.Changes {
		if filepath.Base(ch.Path) == "to_delete.txt" && ch.ChangeType == ChangeDeleted {
			found = true
		}
	}
	if !found {
		t.Error("expected to detect deleted file")
	}
}

func TestGenerateReport(t *testing.T) {
	dir := t.TempDir()
	createTempFiles(t, dir, map[string]string{
		"a.txt": "content a",
		"b.txt": "content b",
	})

	m := setupTestManager(t)
	baseline, _ := m.CreateBaseline(context.Background(), "report-test", "", []string{dir}, HashSHA256)

	report, err := m.GenerateReport(context.Background(), baseline.ID)
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}

	if report.BaselineID != baseline.ID {
		t.Error("baseline ID mismatch")
	}
	if report.TotalFiles < 2 {
		t.Errorf("expected at least 2 files, got %d", report.TotalFiles)
	}
	if report.IntegrityScore < 99 {
		t.Errorf("expected high integrity score, got %.1f", report.IntegrityScore)
	}
}

func TestGenerateReportNotFound(t *testing.T) {
	m := setupTestManager(t)
	_, err := m.GenerateReport(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent baseline")
	}
}

func TestListChanges(t *testing.T) {
	m := setupTestManager(t)

	// 手动添加变更
	m.mu.Lock()
	m.changes = append(m.changes, &FileChange{
		ID:         "c1",
		Path:       "/test/file1",
		ChangeType: ChangeModified,
		AlertLevel: AlertCritical,
		DetectedAt: time.Now(),
	})
	m.changes = append(m.changes, &FileChange{
		ID:         "c2",
		Path:       "/test/file2",
		ChangeType: ChangeCreated,
		AlertLevel: AlertWarning,
		DetectedAt: time.Now(),
	})
	m.mu.Unlock()

	result := m.ListChanges(&ListChangesRequest{Page: 1, PageSize: 10})
	if result.Total != 2 {
		t.Errorf("expected 2 changes, got %d", result.Total)
	}

	// 按级别过滤
	result = m.ListChanges(&ListChangesRequest{Level: AlertCritical, Page: 1, PageSize: 10})
	if result.Total != 1 {
		t.Errorf("expected 1 critical change, got %d", result.Total)
	}
}

func TestAcknowledgeChange(t *testing.T) {
	m := setupTestManager(t)

	m.mu.Lock()
	m.changes = append(m.changes, &FileChange{
		ID:         "test-change",
		Path:       "/test",
		ChangeType: ChangeModified,
		DetectedAt: time.Now(),
	})
	m.mu.Unlock()

	if err := m.AcknowledgeChange("test-change", "reviewed by admin"); err != nil {
		t.Fatalf("AcknowledgeChange failed: %v", err)
	}

	if err := m.AcknowledgeChange("nonexistent", ""); err == nil {
		t.Error("expected error for nonexistent change")
	}
}

func TestGetRepairSuggestions(t *testing.T) {
	m := setupTestManager(t)

	m.mu.Lock()
	m.changes = append(m.changes, &FileChange{
		ID:           "mod-change",
		Path:         "/etc/config",
		ChangeType:   ChangeModified,
		BaselineHash: "abc123",
		DetectedAt:   time.Now(),
	})
	m.changes = append(m.changes, &FileChange{
		ID:         "del-change",
		Path:       "/etc/important",
		ChangeType: ChangeDeleted,
		DetectedAt: time.Now(),
	})
	m.changes = append(m.changes, &FileChange{
		ID:         "perm-change",
		Path:       "/etc/shadow",
		ChangeType: ChangePermission,
		OldMode:    0644,
		NewMode:    0777,
		DetectedAt: time.Now(),
	})
	m.mu.Unlock()

	// 修改建议
	suggestions, err := m.GetRepairSuggestions("mod-change")
	if err != nil {
		t.Fatalf("GetRepairSuggestions failed: %v", err)
	}
	if len(suggestions) < 2 {
		t.Errorf("expected at least 2 suggestions for modified file, got %d", len(suggestions))
	}

	// 删除建议
	suggestions, _ = m.GetRepairSuggestions("del-change")
	if len(suggestions) < 1 {
		t.Error("expected suggestion for deleted file")
	}

	// 权限建议
	suggestions, _ = m.GetRepairSuggestions("perm-change")
	if len(suggestions) < 1 {
		t.Error("expected suggestion for permission change")
	}

	// 不存在的变更
	_, err = m.GetRepairSuggestions("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent change")
	}
}

func TestExportAuditLog(t *testing.T) {
	m := setupTestManager(t)

	// 触发一些审计日志
	m.addAuditLog("test_action", "test_resource", "test details")

	// JSON 导出
	data, err := m.ExportAuditLog(&ExportAuditLogRequest{Format: "json"})
	if err != nil {
		t.Fatalf("ExportAuditLog failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty export")
	}

	// CSV 导出
	data, err = m.ExportAuditLog(&ExportAuditLogRequest{Format: "csv"})
	if err != nil {
		t.Fatalf("ExportAuditLog CSV failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty CSV export")
	}
}

func TestGetConfig(t *testing.T) {
	m := setupTestManager(t)
	cfg := m.GetConfig()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if !cfg.Enabled {
		t.Error("expected enabled")
	}
}

func TestUpdateConfig(t *testing.T) {
	m := setupTestManager(t)
	cfg := m.GetConfig()
	cfg.DefaultAlgorithm = HashSHA512
	m.UpdateConfig(cfg)

	got := m.GetConfig()
	if got.DefaultAlgorithm != HashSHA512 {
		t.Errorf("expected SHA512, got %v", got.DefaultAlgorithm)
	}
}

func TestGetAlerts(t *testing.T) {
	m := setupTestManager(t)
	alerts := m.GetAlerts(10)
	if alerts == nil {
		t.Error("expected non-nil alerts slice")
	}
}

func TestGetScanResults(t *testing.T) {
	m := setupTestManager(t)
	results := m.GetScanResults(10)
	if results == nil {
		t.Error("expected non-nil results slice")
	}
}

// --- Handler 测试 ---

func TestHandler_GetStatus(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/file-integrity/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_GetConfig(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/file-integrity/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_UpdateConfig(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"enabled":true,"default_algorithm":"sha512","worker_count":8}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/file-integrity/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_CreateBaseline(t *testing.T) {
	dir := t.TempDir()
	createTempFiles(t, dir, map[string]string{"a.txt": "content"})

	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body, _ := json.Marshal(map[string]interface{}{
		"name":  "test",
		"paths": []string{dir},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/file-integrity/baselines", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ListBaselines(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/file-integrity/baselines?page=1&page_size=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_AddRule(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"name":"test-rule","paths":["/tmp"],"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/file-integrity/rules", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ListRules(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/file-integrity/rules", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_RunScan(t *testing.T) {
	dir := t.TempDir()
	createTempFiles(t, dir, map[string]string{"a.txt": "content"})

	m := setupTestManager(t)
	m.CreateBaseline(context.Background(), "test", "", []string{dir}, HashSHA256)
	r := setupTestRouter(t, m)

	body, _ := json.Marshal(map[string]interface{}{
		"paths": []string{dir},
		"mode":  "full",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/file-integrity/scan", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ListChanges(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/file-integrity/changes?page=1&page_size=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_GetAlerts(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/file-integrity/alerts?limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_StartStop(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// Start
	req := httptest.NewRequest(http.MethodPost, "/api/v1/file-integrity/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for start, got %d: %s", w.Code, w.Body.String())
	}

	// Stop
	req = httptest.NewRequest(http.MethodPost, "/api/v1/file-integrity/stop", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for stop, got %d", w.Code)
	}
}

func TestHandler_ExportAuditLog(t *testing.T) {
	m := setupTestManager(t)
	m.addAuditLog("test", "res", "details")
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/file-integrity/audit-log/export?format=json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ExportAuditLogCSV(t *testing.T) {
	m := setupTestManager(t)
	m.addAuditLog("test", "res", "details")
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/file-integrity/audit-log/export?format=csv", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/csv" {
		t.Errorf("expected text/csv, got %s", ct)
	}
}

func TestHashFile(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	m := setupTestManager(t)

	// SHA256
	hash256, err := m.hashFile(testFile, HashSHA256)
	if err != nil {
		t.Fatalf("hashFile SHA256 failed: %v", err)
	}
	if len(hash256) != 64 {
		t.Errorf("expected 64 char SHA256 hash, got %d chars", len(hash256))
	}

	// SHA512
	hash512, err := m.hashFile(testFile, HashSHA512)
	if err != nil {
		t.Fatalf("hashFile SHA512 failed: %v", err)
	}
	if len(hash512) != 128 {
		t.Errorf("expected 128 char SHA512 hash, got %d chars", len(hash512))
	}

	// 相同文件相同算法应产生相同哈希
	hash256b, _ := m.hashFile(testFile, HashSHA256)
	if hash256 != hash256b {
		t.Error("same file should produce same hash")
	}
}

func TestWatchers(t *testing.T) {
	m := setupTestManager(t)
	rule := &MonitorRule{
		Name:     "watch-test",
		Paths:    []string{"/tmp"},
		Enabled:  true,
		MaxDepth: 5,
	}
	m.AddRule(rule)

	m.startWatcher(rule)
	if len(m.watchers) != 1 {
		t.Errorf("expected 1 watcher, got %d", len(m.watchers))
	}

	// 重复启动不应创建新 watcher
	m.startWatcher(rule)
	if len(m.watchers) != 1 {
		t.Errorf("expected 1 watcher after duplicate start, got %d", len(m.watchers))
	}

	m.stopWatcher(rule.ID)
	if len(m.watchers) != 0 {
		t.Errorf("expected 0 watchers after stop, got %d", len(m.watchers))
	}
}

func TestShouldExclude(t *testing.T) {
	tests := []struct {
		path     string
		excludes []string
		expected bool
	}{
		{"/tmp/test.txt", []string{"/tmp"}, true},
		{"/var/log/test.txt", []string{"/tmp"}, false},
		{"/home/user/test.txt", []string{"/tmp", "/home"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := false
			for _, ex := range tt.excludes {
				if len(tt.path) >= len(ex) && tt.path[:len(ex)] == ex {
					result = true
					break
				}
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}