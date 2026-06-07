// Package filetimemachine2 测试
package filetimemachine2

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// setupTestEnv 创建测试环境
func setupTestEnv(t *testing.T) (*Handlers, string, func()) {
	t.Helper()

	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "filetimemachine2_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}

	// 创建测试文件
	testFiles := map[string]string{
		"file1.txt":       "Hello World\nLine 2\nLine 3",
		"file2.txt":       "Test content",
		"subdir/file3.go": "package main\n\nfunc main() {}",
	}
	for name, content := range testFiles {
		path := filepath.Join(tmpDir, name)
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte(content), 0644)
	}

	// 创建引擎
	storageDir, _ := os.MkdirTemp("", "storage_test_*")
	logger, _ := zap.NewDevelopment()
	engine, err := NewEngine(storageDir, logger)
	if err != nil {
		t.Fatalf("创建引擎失败: %v", err)
	}

	handlers := NewHandlers(engine)

	cleanup := func() {
		engine.Stop()
		os.RemoveAll(tmpDir)
		os.RemoveAll(storageDir)
	}

	return handlers, tmpDir, cleanup
}

// setupRouter 创建测试路由器
func setupRouter(h *Handlers) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)
	return r
}

// TestCreateSnapshot 测试创建快照
func TestCreateSnapshot(t *testing.T) {
	h, tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	r := setupRouter(h)

	reqBody := CreateSnapshotRequest{
		Name:     "测试快照",
		RootPath: tmpDir,
		Tags:     []string{"test", "unit"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/filetimemachine2/snapshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("创建快照失败: %d, body: %s", w.Code, w.Body.String())
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Fatalf("响应码不正确: %d", resp.Code)
	}

	data, _ := json.Marshal(resp.Data)
	var snapshot Snapshot
	json.Unmarshal(data, &snapshot)

	if snapshot.Name != "测试快照" {
		t.Errorf("快照名称不匹配: %s", snapshot.Name)
	}
	if snapshot.Status != SnapshotCompleted {
		t.Errorf("快照状态不正确: %s", snapshot.Status)
	}
}

// TestListSnapshots 测试列出快照
func TestListSnapshots(t *testing.T) {
	h, tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	r := setupRouter(h)

	// 先创建两个快照
	for i := 0; i < 2; i++ {
		reqBody := CreateSnapshotRequest{
			Name:     "快照" + string(rune('A'+i)),
			RootPath: tmpDir,
		}
		body, _ := json.Marshal(reqBody)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/filetimemachine2/snapshots", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
	}

	// 获取列表
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/filetimemachine2/snapshots", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("获取列表失败: %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)

	data, _ := json.Marshal(resp.Data)
	var snapshots []SnapshotListItem
	json.Unmarshal(data, &snapshots)

	if len(snapshots) != 2 {
		t.Fatalf("快照数量不正确: %d", len(snapshots))
	}
}

// TestDeleteSnapshot 测试删除快照
func TestDeleteSnapshot(t *testing.T) {
	h, tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	r := setupRouter(h)

	// 创建快照
	reqBody := CreateSnapshotRequest{
		Name:     "待删除快照",
		RootPath: tmpDir,
	}
	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/filetimemachine2/snapshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := json.Marshal(resp.Data)
	var snapshot Snapshot
	json.Unmarshal(data, &snapshot)

	// 删除快照
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v1/filetimemachine2/snapshots/"+snapshot.ID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("删除快照失败: %d", w.Code)
	}

	// 验证已删除
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/filetimemachine2/snapshots/"+snapshot.ID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("删除后仍可访问: %d", w.Code)
	}
}

// TestBrowseSnapshot 测试浏览快照
func TestBrowseSnapshot(t *testing.T) {
	h, tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	r := setupRouter(h)

	// 创建快照
	reqBody := CreateSnapshotRequest{
		Name:     "浏览测试",
		RootPath: tmpDir,
	}
	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/filetimemachine2/snapshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := json.Marshal(resp.Data)
	var snapshot Snapshot
	json.Unmarshal(data, &snapshot)

	// 浏览根目录
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/filetimemachine2/snapshots/"+snapshot.ID+"/browse", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("浏览快照失败: %d", w.Code)
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ = json.Marshal(resp.Data)
	var content SnapshotContent
	json.Unmarshal(data, &content)

	if content.TotalCount < 2 {
		t.Errorf("根目录条目数量不足: %d", content.TotalCount)
	}
}

// TestDiffSnapshots 测试快照对比
func TestDiffSnapshots(t *testing.T) {
	h, tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	r := setupRouter(h)

	// 创建第一个快照
	reqBody := CreateSnapshotRequest{
		Name:     "快照A",
		RootPath: tmpDir,
	}
	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/filetimemachine2/snapshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := json.Marshal(resp.Data)
	var snapshotA Snapshot
	json.Unmarshal(data, &snapshotA)

	// 修改文件
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("Modified content\nLine 2 changed"), 0644)

	// 创建第二个快照
	reqBody.Name = "快照B"
	body, _ = json.Marshal(reqBody)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/filetimemachine2/snapshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ = json.Marshal(resp.Data)
	var snapshotB Snapshot
	json.Unmarshal(data, &snapshotB)

	// 对比快照
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/filetimemachine2/diff?snapshot_a="+snapshotA.ID+"&snapshot_b="+snapshotB.ID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("对比快照失败: %d", w.Code)
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ = json.Marshal(resp.Data)
	var diffResult DiffResult
	json.Unmarshal(data, &diffResult)

	if diffResult.Stats.Modified < 1 {
		t.Errorf("修改文件数量不正确: %d", diffResult.Stats.Modified)
	}
}

// TestRestoreSnapshot 测试恢复快照
func TestRestoreSnapshot(t *testing.T) {
	h, tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	r := setupRouter(h)

	// 创建快照
	reqBody := CreateSnapshotRequest{
		Name:     "恢复测试",
		RootPath: tmpDir,
	}
	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/filetimemachine2/snapshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := json.Marshal(resp.Data)
	var snapshot Snapshot
	json.Unmarshal(data, &snapshot)

	// 恢复到新目录
	restoreDir, _ := os.MkdirTemp("", "restore_test_*")
	defer os.RemoveAll(restoreDir)

	restoreReq := RestoreRequest{
		TargetPath:    restoreDir,
		OverwriteMode: "overwrite",
	}
	body, _ = json.Marshal(restoreReq)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/filetimemachine2/snapshots/"+snapshot.ID+"/restore", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("恢复失败: %d, body: %s", w.Code, w.Body.String())
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ = json.Marshal(resp.Data)
	var result RestoreResult
	json.Unmarshal(data, &result)

	if result.RestoredFiles < 2 {
		t.Errorf("恢复文件数量不足: %d", result.RestoredFiles)
	}

	// 验证文件存在
	if _, err := os.Stat(filepath.Join(restoreDir, "file1.txt")); os.IsNotExist(err) {
		t.Error("恢复的文件不存在: file1.txt")
	}
}

// TestTimeline 测试时间线
func TestTimeline(t *testing.T) {
	h, tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	r := setupRouter(h)

	// 创建多个快照
	for i := 0; i < 3; i++ {
		reqBody := CreateSnapshotRequest{
			Name:     "时间线快照",
			RootPath: tmpDir,
		}
		body, _ := json.Marshal(reqBody)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/filetimemachine2/snapshots", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
	}

	// 获取时间线
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/filetimemachine2/timeline?granularity=hour", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("获取时间线失败: %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := json.Marshal(resp.Data)
	var timeline TimelineData
	json.Unmarshal(data, &timeline)

	if timeline.Total != 3 {
		t.Errorf("时间线总数不正确: %d", timeline.Total)
	}
	if len(timeline.Buckets) < 1 {
		t.Error("时间线桶为空")
	}
}

// TestSearchFiles 测试搜索文件
func TestSearchFiles(t *testing.T) {
	h, tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	r := setupRouter(h)

	// 创建快照
	reqBody := CreateSnapshotRequest{
		Name:     "搜索测试",
		RootPath: tmpDir,
	}
	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/filetimemachine2/snapshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// 搜索文件
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/filetimemachine2/search?file_name=file1*", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("搜索失败: %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := json.Marshal(resp.Data)
	var result SearchResult
	json.Unmarshal(data, &result)

	if result.Total < 1 {
		t.Errorf("搜索结果为空")
	}
}

// TestStorageStats 测试存储统计
func TestStorageStats(t *testing.T) {
	h, tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	r := setupRouter(h)

	// 创建快照
	reqBody := CreateSnapshotRequest{
		Name:     "统计测试",
		RootPath: tmpDir,
	}
	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/filetimemachine2/snapshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// 获取统计
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/filetimemachine2/stats", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("获取统计失败: %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := json.Marshal(resp.Data)
	var stats StorageStats
	json.Unmarshal(data, &stats)

	if stats.TotalSnapshots != 1 {
		t.Errorf("快照总数不正确: %d", stats.TotalSnapshots)
	}
	if stats.TotalSize <= 0 {
		t.Error("总大小应大于0")
	}
}

// TestRetentionConfig 测试保留策略
func TestRetentionConfig(t *testing.T) {
	h, _, cleanup := setupTestEnv(t)
	defer cleanup()

	r := setupRouter(h)

	// 获取当前配置
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/filetimemachine2/retention", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("获取保留策略失败: %d", w.Code)
	}

	// 更新配置
	updateReq := UpdateRetentionRequest{
		Enabled: true,
		Rules: []RetentionRule{
			{Name: "测试规则", Interval: "1d", Count: 7, Priority: 1},
		},
		MaxSnapshots: 50,
		AutoCleanup:  true,
	}
	body, _ := json.Marshal(updateReq)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("PUT", "/api/v1/filetimemachine2/retention", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("更新保留策略失败: %d", w.Code)
	}

	// 验证更新
	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := json.Marshal(resp.Data)
	var config RetentionConfig
	json.Unmarshal(data, &config)

	if config.MaxSnapshots != 50 {
		t.Errorf("最大快照数不正确: %d", config.MaxSnapshots)
	}
}

// TestTagSnapshot 测试标签管理
func TestTagSnapshot(t *testing.T) {
	h, tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	r := setupRouter(h)

	// 创建快照
	reqBody := CreateSnapshotRequest{
		Name:     "标签测试",
		RootPath: tmpDir,
	}
	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/filetimemachine2/snapshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := json.Marshal(resp.Data)
	var snapshot Snapshot
	json.Unmarshal(data, &snapshot)

	// 添加标签
	tagReq := TagRequest{Tags: []string{"important", "backup"}}
	body, _ = json.Marshal(tagReq)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/filetimemachine2/snapshots/"+snapshot.ID+"/tag", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("添加标签失败: %d", w.Code)
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ = json.Marshal(resp.Data)
	var updated Snapshot
	json.Unmarshal(data, &updated)

	if len(updated.Tags) < 2 {
		t.Errorf("标签数量不足: %d", len(updated.Tags))
	}
}

// TestCreateSnapshotInvalidPath 测试无效路径
func TestCreateSnapshotInvalidPath(t *testing.T) {
	h, _, cleanup := setupTestEnv(t)
	defer cleanup()

	r := setupRouter(h)

	reqBody := CreateSnapshotRequest{
		Name:     "无效路径",
		RootPath: "/nonexistent/path",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/filetimemachine2/snapshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("应返回错误: %d", w.Code)
	}
}
