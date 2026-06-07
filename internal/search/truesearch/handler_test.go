package truesearch

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
)

func setupTestEngine(t *testing.T) (*Engine, *gin.Engine) {
	t.Helper()

	dir := t.TempDir()
	logger := newTestLogger(t)

	cfg := Config{
		IndexPath:     filepath.Join(dir, "test.bleve"),
		MaxFileSize:   10 * 1024 * 1024,
		BatchSize:     10,
		SupportedExts: []string{".txt", ".md"},
		ExcludeDirs:   []string{},
	}

	engine, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })

	// 索引一些测试文件
	testFiles := map[string]string{
		"hello.txt":  "Hello World, this is a greeting",
		"search.txt": "Full text search is a powerful feature",
		"readme.md":  "# Project\n\nWelcome to the NAS-OS project",
	}
	for name, content := range testFiles {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		if err := engine.IndexFile(path); err != nil {
			t.Fatal(err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	engine.APIHandler().RegisterRoutes(r.Group("/api/v1"))

	return engine, r
}

func TestHandlerSearch(t *testing.T) {
	_, r := setupTestEngine(t)

	body := SearchRequest{
		Query:      "search",
		MaxResults: 10,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/truesearch", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["code"].(float64) != 0 {
		t.Errorf("expected code 0, got %v", resp["code"])
	}

	data := resp["data"].(map[string]interface{})
	if data["total"].(float64) == 0 {
		t.Error("expected at least 1 search result")
	}
	t.Logf("Search returned %v results in %vms", data["total"], data["took_ms"])
}

func TestHandlerSearchEmptyQuery(t *testing.T) {
	_, r := setupTestEngine(t)

	body := SearchRequest{Query: ""}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/truesearch", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandlerSearchInvalidBody(t *testing.T) {
	_, r := setupTestEngine(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/truesearch", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandlerStatus(t *testing.T) {
	_, r := setupTestEngine(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/truesearch/status", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["code"].(float64) != 0 {
		t.Errorf("expected code 0, got %v", resp["code"])
	}

	data := resp["data"].(map[string]interface{})
	totalFiles := data["total_files"].(float64)
	if totalFiles < 1 {
		t.Errorf("expected at least 1 indexed file, got %v", totalFiles)
	}
	t.Logf("Status: %d files indexed", int(totalFiles))
}

func TestHandlerReindex(t *testing.T) {
	_, r := setupTestEngine(t)

	body := reindexRequest{Force: true}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/truesearch/reindex", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// 等待异步reindex goroutine完成，避免TempDir清理时索引未关闭
	time.Sleep(500 * time.Millisecond)
}

func TestHandlerIndexFiles(t *testing.T) {
	dir := t.TempDir()
	logger := newTestLogger(t)

	cfg := Config{
		IndexPath:     filepath.Join(dir, "test.bleve"),
		MaxFileSize:   10 * 1024 * 1024,
		BatchSize:     10,
		SupportedExts: []string{".txt"},
		ExcludeDirs:   []string{},
	}

	engine, err := New(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close() }()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	engine.APIHandler().RegisterRoutes(r.Group("/api/v1"))

	// 创建测试文件
	testFile := filepath.Join(dir, "new_file.txt")
	if err := os.WriteFile(testFile, []byte("new file content"), 0644); err != nil {
		t.Fatal(err)
	}

	body := indexFilesRequest{Paths: []string{testFile}}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/truesearch/index", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerIndexDirectory(t *testing.T) {
	dir := t.TempDir()
	logger := newTestLogger(t)

	cfg := Config{
		IndexPath:     filepath.Join(dir, "test.bleve"),
		MaxFileSize:   10 * 1024 * 1024,
		BatchSize:     10,
		SupportedExts: []string{".txt"},
		ExcludeDirs:   []string{},
	}

	engine, err := New(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close() }()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	engine.APIHandler().RegisterRoutes(r.Group("/api/v1"))

	// 创建测试目录
	subdir := filepath.Join(dir, "data")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "a.txt"), []byte("aaa"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "b.txt"), []byte("bbb"), 0644); err != nil {
		t.Fatal(err)
	}

	body := indexDirRequest{Path: subdir}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/truesearch/index/dir", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// 等待异步索引完成
	time.Sleep(200 * time.Millisecond)

	status := engine.Status()
	if status.TotalFiles < 2 {
		t.Errorf("expected at least 2 indexed files, got %d", status.TotalFiles)
	}
}
