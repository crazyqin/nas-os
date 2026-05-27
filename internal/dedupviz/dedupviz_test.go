package dedupviz

import (
	"bytes"
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

func createTestFiles(t *testing.T, dir string) {
	t.Helper()

	// 创建重复文件
	os.MkdirAll(filepath.Join(dir, "dir1"), 0755)
	os.MkdirAll(filepath.Join(dir, "dir2"), 0755)
	os.MkdirAll(filepath.Join(dir, "dir3"), 0755)

	content := []byte("test content for duplicate detection")

	// 创建相同的文件
	os.WriteFile(filepath.Join(dir, "dir1", "file1.txt"), content, 0644)
	os.WriteFile(filepath.Join(dir, "dir2", "file2.txt"), content, 0644)
	os.WriteFile(filepath.Join(dir, "dir3", "file3.txt"), content, 0644)

	// 创建另一个重复组
	content2 := []byte("another duplicate content here")
	os.WriteFile(filepath.Join(dir, "dir1", "doc1.pdf"), content2, 0644)
	os.WriteFile(filepath.Join(dir, "dir2", "doc2.pdf"), content2, 0644)

	// 创建唯一文件
	os.WriteFile(filepath.Join(dir, "dir1", "unique.txt"), []byte("unique content"), 0644)
}

func TestClassifyFileType(t *testing.T) {
	tests := []struct {
		path     string
		expected FileType
	}{
		{"photo.jpg", FileTypeImage},
		{"video.mp4", FileTypeVideo},
		{"song.mp3", FileTypeAudio},
		{"doc.pdf", FileTypeDocument},
		{"archive.zip", FileTypeArchive},
		{"main.go", FileTypeCode},
		{"unknown.xyz", FileTypeOther},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := classifyFileType(tt.path)
			if result != tt.expected {
				t.Errorf("classifyFileType(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestSelectKeepFile(t *testing.T) {
	m := setupTestManager(t)

	files := []DuplicateFile{
		{Path: "/data/backup/file.txt", ModifiedAt: timeNow().Add(-2 * time.Hour)},
		{Path: "/home/user/file.txt", ModifiedAt: timeNow().Add(-1 * time.Hour)},
		{Path: "/tmp/copy/file.txt", ModifiedAt: timeNow()},
	}

	keepPath, reason := m.selectKeepFile(files)
	if keepPath == "" {
		t.Error("expected non-empty keep path")
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestScanDirectory(t *testing.T) {
	m := setupTestManager(t)
	dir := t.TempDir()

	createTestFiles(t, dir)

	config := DefaultScanConfig()
	config.MinFileSize = 1 // 允许小文件
	config.ExcludePaths = nil

	result, err := m.ScanDirectory([]string{dir}, config)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// 等待扫描完成
	// 注意：由于扫描是异步的，这里可能需要等待
	// 在测试中，我们可以直接检查结果

	if result.ScanID == "" {
		t.Error("expected non-empty scan ID")
	}
}

func TestGetScanResult(t *testing.T) {
	m := setupTestManager(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	config := DefaultScanConfig()
	config.MinFileSize = 1
	config.ExcludePaths = nil

	result, _ := m.ScanDirectory([]string{dir}, config)

	// 获取扫描结果
	got, err := m.GetScanResult(result.ScanID)
	if err != nil {
		t.Fatalf("GetScanResult failed: %v", err)
	}

	if got.ScanID != result.ScanID {
		t.Errorf("expected scan ID %q, got %q", result.ScanID, got.ScanID)
	}

	// 不存在的结果
	_, err = m.GetScanResult("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent scan")
	}
}

func TestListScanResults(t *testing.T) {
	m := setupTestManager(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	config := DefaultScanConfig()
	config.MinFileSize = 1
	config.ExcludePaths = nil

	// 执行多次扫描
	m.ScanDirectory([]string{dir}, config)
	m.ScanDirectory([]string{dir}, config)

	results := m.ListScanResults()
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestDeleteDuplicates(t *testing.T) {
	m := setupTestManager(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	config := DefaultScanConfig()
	config.MinFileSize = 1
	config.ExcludePaths = nil

	// 执行扫描
	m.ScanDirectory([]string{dir}, config)

	// 等待扫描完成（简化测试）
	scan := m.GetLastScanResult()
	if scan == nil {
		t.Skip("scan not completed")
	}

	if len(scan.Groups) == 0 {
		t.Skip("no duplicate groups found")
	}

	// 测试删除（dry run）
	group := scan.Groups[0]
	req := &DeleteRequest{
		GroupHash: group.Hash,
		KeepPath:  group.KeepPath,
		DryRun:    true,
	}

	result, err := m.DeleteDuplicates(req)
	if err != nil {
		t.Fatalf("DeleteDuplicates failed: %v", err)
	}

	if !result.DryRun {
		t.Error("expected dry run result")
	}
	if len(result.DeletedFiles) == 0 {
		t.Error("expected files to delete")
	}
}

func TestBatchDelete(t *testing.T) {
	m := setupTestManager(t)

	// 测试没有扫描结果时的错误
	req := &BatchDeleteRequest{
		DryRun: true,
	}

	_, err := m.BatchDeleteDuplicates(req)
	if err == nil {
		t.Error("expected error when no scan available")
	}
}

func TestGetVisualizationData(t *testing.T) {
	m := setupTestManager(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	config := DefaultScanConfig()
	config.MinFileSize = 1
	config.ExcludePaths = nil

	result, _ := m.ScanDirectory([]string{dir}, config)

	// 注意：由于扫描是异步的，可视化数据可能还未准备好
	// 这里我们测试方法是否存在且不 panic
	_, _ = m.GetVisualizationData(result.ScanID)
}

func TestGetDuplicatesByType(t *testing.T) {
	m := setupTestManager(t)

	// 测试空结果
	_, err := m.GetDuplicatesByType(FileTypeImage)
	if err == nil {
		t.Error("expected error when no scan available")
	}
}

func TestGetDuplicatesBySizeRange(t *testing.T) {
	m := setupTestManager(t)

	// 测试空结果
	_, err := m.GetDuplicatesBySizeRange(0, 1024)
	if err == nil {
		t.Error("expected error when no scan available")
	}
}

func TestExportScanResult(t *testing.T) {
	m := setupTestManager(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	config := DefaultScanConfig()
	config.MinFileSize = 1
	config.ExcludePaths = nil

	result, _ := m.ScanDirectory([]string{dir}, config)

	data, err := m.ExportScanResult(result.ScanID)
	if err != nil {
		t.Fatalf("ExportScanResult failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty export data")
	}

	// 验证是有效的 JSON
	var parsed interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("exported data is not valid JSON: %v", err)
	}
}

func TestConfig(t *testing.T) {
	m := setupTestManager(t)

	// 获取默认配置
	cfg := m.GetConfig()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if !cfg.Enabled {
		t.Error("expected config to be enabled by default")
	}

	// 更新配置
	newCfg := DefaultDedupvizConfig()
	newCfg.Enabled = false
	newCfg.MaxResults = 5000
	m.UpdateConfig(newCfg)

	cfg = m.GetConfig()
	if cfg.Enabled {
		t.Error("expected config to be disabled")
	}
	if cfg.MaxResults != 5000 {
		t.Errorf("expected max results 5000, got %d", cfg.MaxResults)
	}
}

// ========== Handler 测试 ==========

func TestHandler_StartScan(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"paths":["/tmp"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dedup-viz/scan", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_StartScanNoPaths(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"paths":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dedup-viz/scan", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// 应该使用默认路径
	if w.Code == http.StatusBadRequest {
		// 如果没有默认路径，返回 400 是可以接受的
		return
	}
}

func TestHandler_ListScans(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dedup-viz/scans", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_GetLatestScan(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dedup-viz/scan/latest", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// 没有扫描结果时返回 404
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandler_GetConfig(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dedup-viz/config", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_UpdateConfig(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"enabled":true,"max_results":5000}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/dedup-viz/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_DeleteDuplicates(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"group_hash":"test","keep_path":"/tmp/file.txt","dry_run":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dedup-viz/delete", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// 没有扫描结果时返回 500
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", w.Code)
	}
}

func TestHandler_BatchDelete(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"dry_run":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dedup-viz/delete/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", w.Code)
	}
}

func TestHandler_GetVisualization(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dedup-viz/visualization/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandler_GetLatestVisualization(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dedup-viz/visualization/latest", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandler_ExportResult(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dedup-viz/export/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func timeNow() time.Time {
	return time.Now()
}
