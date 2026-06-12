package webdav

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ========== BatchManager 测试 ==========

func TestBatchManager_ExecuteBatchUpload(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webdav-batch-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建源文件
	srcDir, _ := os.MkdirTemp("", "batch-src-*")
	defer os.RemoveAll(srcDir)
	for i := 0; i < 3; i++ {
		name := filepath.Join(srcDir, fmt.Sprintf("file%d.txt", i))
		os.WriteFile(name, []byte(fmt.Sprintf("content %d", i)), 0644)
	}

	pool := NewConnectionPool()
	bm := NewBatchManager(pool)

	req := &BatchRequest{
		Operation: "upload",
		Items: []BatchItem{
			{Source: filepath.Join(srcDir, "file0.txt"), Destination: "dest0.txt"},
			{Source: filepath.Join(srcDir, "file1.txt"), Destination: "dest1.txt"},
			{Source: filepath.Join(srcDir, "file2.txt"), Destination: "dest2.txt"},
		},
		Options: BatchOptions{Concurrency: 2},
	}

	result, err := bm.ExecuteBatch(context.Background(), tmpDir, req)
	if err != nil {
		t.Fatalf("批量上传失败: %v", err)
	}

	if result.Total != 3 {
		t.Errorf("期望 total=3, 实际=%d", result.Total)
	}
	if result.Success != 3 {
		t.Errorf("期望 success=3, 实际=%d", result.Success)
	}
	if result.Failed != 0 {
		t.Errorf("期望 failed=0, 实际=%d", result.Failed)
	}
}

func TestBatchManager_ExecuteBatchDelete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webdav-batch-del-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建文件
	for i := 0; i < 3; i++ {
		name := filepath.Join(tmpDir, fmt.Sprintf("del%d.txt", i))
		os.WriteFile(name, []byte("content"), 0644)
	}

	pool := NewConnectionPool()
	bm := NewBatchManager(pool)

	req := &BatchRequest{
		Operation: "delete",
		Items: []BatchItem{
			{Source: "del0.txt"},
			{Source: "del1.txt"},
			{Source: "del2.txt"},
			{Source: "nonexistent.txt"},
		},
		Options: BatchOptions{Concurrency: 2, ContinueOnError: true},
	}

	result, err := bm.ExecuteBatch(context.Background(), tmpDir, req)
	if err != nil {
		t.Fatalf("批量删除失败: %v", err)
	}

	if result.Total != 4 {
		t.Errorf("期望 total=4, 实际=%d", result.Total)
	}
	// 删除不存在的文件不会报错（os.RemoveAll 不返回错误）
	if result.Success != 4 {
		t.Errorf("期望 success=4, 实际=%d", result.Success)
	}
}

func TestBatchManager_InvalidOperation(t *testing.T) {
	pool := NewConnectionPool()
	bm := NewBatchManager(pool)

	req := &BatchRequest{
		Operation: "invalid",
		Items:     []BatchItem{{Source: "test"}},
	}

	_, err := bm.ExecuteBatch(context.Background(), "/tmp", req)
	if err == nil {
		t.Fatal("期望错误，实际为 nil")
	}
}

func TestBatchManager_EmptyRequest(t *testing.T) {
	pool := NewConnectionPool()
	bm := NewBatchManager(pool)

	_, err := bm.ExecuteBatch(context.Background(), "/tmp", nil)
	if err == nil {
		t.Fatal("期望错误，实际为 nil")
	}
}

func TestBatchManager_SetChunkSize(t *testing.T) {
	pool := NewConnectionPool()
	bm := NewBatchManager(pool)

	bm.SetChunkSize(512 * 1024)
	bm.mu.RLock()
	if bm.chunkSize != 512*1024 {
		t.Errorf("期望 chunkSize=512KB, 实际=%d", bm.chunkSize)
	}
	bm.mu.RUnlock()

	// 零值不应更新
	bm.SetChunkSize(0)
	bm.mu.RLock()
	if bm.chunkSize != 512*1024 {
		t.Errorf("chunkSize 应保持不变, 实际=%d", bm.chunkSize)
	}
	bm.mu.RUnlock()
}

// ========== ConnectionPool 测试 ==========

func TestConnectionPool_AcquireRelease(t *testing.T) {
	pool := NewConnectionPool(WithMaxConnections(5))

	ctx := context.Background()
	err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("获取连接失败: %v", err)
	}

	stats := pool.GetStats()
	if stats.Active != 1 {
		t.Errorf("期望 active=1, 实际=%d", stats.Active)
	}

	pool.Release()
	stats = pool.GetStats()
	if stats.Active != 0 {
		t.Errorf("释放后期望 active=0, 实际=%d", stats.Active)
	}
}

func TestConnectionPool_MaxConnections(t *testing.T) {
	pool := NewConnectionPool(WithMaxConnections(2))

	ctx := context.Background()
	_ = pool.Acquire(ctx)
	_ = pool.Acquire(ctx)

	// 第三个获取应该超时
	ctx2, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := pool.Acquire(ctx2)
	if err == nil {
		t.Fatal("期望超时错误，实际为 nil")
	}

	pool.Release()
	pool.Release()
}

func TestConnectionPool_Resize(t *testing.T) {
	pool := NewConnectionPool(WithMaxConnections(5))
	pool.Resize(10)

	stats := pool.GetStats()
	if stats.MaxConn != 10 {
		t.Errorf("期望 max_conn=10, 实际=%d", stats.MaxConn)
	}

	// 零值不应更新
	pool.Resize(0)
	stats = pool.GetStats()
	if stats.MaxConn != 10 {
		t.Errorf("max_conn 应保持 10, 实际=%d", stats.MaxConn)
	}
}

func TestConnectionPool_Options(t *testing.T) {
	acquired := false
	released := false

	pool := NewConnectionPool(
		WithMaxConnections(5),
		WithPoolTimeout(5*time.Second),
		WithOnAcquire(func() { acquired = true }),
		WithOnRelease(func() { released = true }),
	)

	ctx := context.Background()
	_ = pool.Acquire(ctx)
	if !acquired {
		t.Error("onAcquire 回调未调用")
	}

	pool.Release()
	if !released {
		t.Error("onRelease 回调未调用")
	}
}

// ========== PerfMetrics 测试 ==========

func TestPerfMetrics_RecordRequest(t *testing.T) {
	m := NewPerfMetrics()

	m.RecordRequest("GET", 100, 0, 50, nil)
	m.RecordRequest("PUT", 0, 200, 100, nil)
	m.RecordRequest("DELETE", 0, 0, 30, fmt.Errorf("error"))

	snapshot := m.GetSnapshot()
	if snapshot["total_requests"] != int64(3) {
		t.Errorf("期望 total_requests=3, 实际=%v", snapshot["total_requests"])
	}
	if snapshot["success_requests"] != int64(2) {
		t.Errorf("期望 success_requests=2, 实际=%v", snapshot["success_requests"])
	}
	if snapshot["error_requests"] != int64(1) {
		t.Errorf("期望 error_requests=1, 实际=%v", snapshot["error_requests"])
	}

	ops := snapshot["ops_breakdown"].(map[string]int64)
	if ops["get"] != 1 {
		t.Errorf("期望 get=1, 实际=%d", ops["get"])
	}
	if ops["put"] != 1 {
		t.Errorf("期望 put=1, 实际=%d", ops["put"])
	}
	if ops["delete"] != 1 {
		t.Errorf("期望 delete=1, 实际=%d", ops["delete"])
	}
}

func TestPerfMetrics_Reset(t *testing.T) {
	m := NewPerfMetrics()
	m.RecordRequest("GET", 100, 0, 50, nil)
	m.Reset()

	snapshot := m.GetSnapshot()
	if snapshot["total_requests"] != int64(0) {
		t.Errorf("重后期望 total_requests=0, 实际=%v", snapshot["total_requests"])
	}
}

func TestPerfMetrics_StartEndRequest(t *testing.T) {
	m := NewPerfMetrics()

	start := m.StartRequest()
	if m.ActiveRequests != 1 {
		t.Errorf("期望 active=1, 实际=%d", m.ActiveRequests)
	}

	time.Sleep(10 * time.Millisecond)
	latency := m.EndRequest(start)
	if latency < 10 {
		t.Errorf("期望 latency>=10ms, 实际=%d", latency)
	}
	if m.ActiveRequests != 0 {
		t.Errorf("期望 active=0, 实际=%d", m.ActiveRequests)
	}
}

func TestPerfMetrics_ErrorRate(t *testing.T) {
	m := NewPerfMetrics()

	// 空状态错误率
	if m.errorRate() != 0 {
		t.Errorf("空状态错误率应为 0, 实际=%f", m.errorRate())
	}

	m.RecordRequest("GET", 0, 0, 10, nil)
	m.RecordRequest("GET", 0, 0, 10, fmt.Errorf("err"))
	m.RecordRequest("GET", 0, 0, 10, fmt.Errorf("err"))

	rate := m.errorRate()
	if rate < 66.0 || rate > 67.0 {
		t.Errorf("期望错误率约 66.7%%, 实际=%f", rate)
	}
}

// ========== StreamTransmitter 测试 ==========

func TestStreamTransmitter_StreamWrite(t *testing.T) {
	transmitter := NewStreamTransmitter(&StreamConfig{
		BufferSize: 1024,
	}, nil)

	src := bytes.NewReader([]byte("hello world streaming test"))
	dst := &bytes.Buffer{}

	n, err := transmitter.StreamWrite(context.Background(), dst, src)
	if err != nil {
		t.Fatalf("流式写入失败: %v", err)
	}

	if n != 26 {
		t.Errorf("期望写入 26 字节, 实际=%d", n)
	}
	if dst.String() != "hello world streaming test" {
		t.Errorf("内容不匹配: %s", dst.String())
	}
}

func TestStreamTransmitter_StreamRead(t *testing.T) {
	transmitter := NewStreamTransmitter(&StreamConfig{
		BufferSize: 8,
	}, nil)

	src := bytes.NewReader([]byte("small buffer test"))
	dst := &bytes.Buffer{}

	n, err := transmitter.StreamRead(context.Background(), dst, src)
	if err != nil {
		t.Fatalf("流式读取失败: %v", err)
	}

	if n != 17 {
		t.Errorf("期望写入 17 字节, 实际=%d", n)
	}
	if dst.String() != "small buffer test" {
		t.Errorf("内容不匹配: %s", dst.String())
	}
}

func TestStreamTransmitter_ContextCancel(t *testing.T) {
	transmitter := NewStreamTransmitter(&StreamConfig{
		BufferSize: 1024,
	}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	src := bytes.NewReader([]byte("data"))
	dst := &bytes.Buffer{}

	_, err := transmitter.StreamWrite(ctx, dst, src)
	if err == nil {
		t.Fatal("期望上下文取消错误，实际为 nil")
	}
}

// ========== PerfManager 测试 ==========

func TestPerfManager_GetStatus(t *testing.T) {
	pm := NewPerfManager()

	status := pm.GetStatus()
	if status["enabled"] != true {
		t.Error("期望 enabled=true")
	}
	if status["chunk_size"] != int64(DefaultChunkSize) {
		t.Errorf("期望 chunk_size=%d, 实际=%v", DefaultChunkSize, status["chunk_size"])
	}
}

func TestPerfManager_SetEnabled(t *testing.T) {
	pm := NewPerfManager()

	pm.SetEnabled(false)
	if pm.IsEnabled() {
		t.Error("期望 enabled=false")
	}

	pm.SetEnabled(true)
	if !pm.IsEnabled() {
		t.Error("期望 enabled=true")
	}
}

func TestPerfManager_UpdateConfig(t *testing.T) {
	pm := NewPerfManager()

	router := gin.New()
	pm.RegisterPerfRoutes(router.Group("/api"))

	// 更新配置
	body := `{"chunk_size": 524288, "max_concurrent": 8}`
	req := httptest.NewRequestWithContext(context.Background(), "PUT", "/api/webdav/perf/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d", w.Code)
	}
}

func TestPerfManager_GetMetrics(t *testing.T) {
	pm := NewPerfManager()

	router := gin.New()
	pm.RegisterPerfRoutes(router.Group("/api"))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/webdav/perf/metrics", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["code"] != float64(0) {
		t.Errorf("期望 code=0, 实际=%v", resp["code"])
	}
}

func TestPerfManager_ResizePool(t *testing.T) {
	pm := NewPerfManager()

	router := gin.New()
	pm.RegisterPerfRoutes(router.Group("/api"))

	body := `{"max_conn": 200}`
	req := httptest.NewRequestWithContext(context.Background(), "PUT", "/api/webdav/perf/pool/resize", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d", w.Code)
	}

	stats := pm.GetPool().GetStats()
	if stats.MaxConn != 200 {
		t.Errorf("期望 max_conn=200, 实际=%d", stats.MaxConn)
	}
}

func TestPerfManager_ResetMetrics(t *testing.T) {
	pm := NewPerfManager()
	pm.GetMetrics().RecordRequest("GET", 100, 0, 50, nil)

	router := gin.New()
	pm.RegisterPerfRoutes(router.Group("/api"))

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/webdav/perf/metrics/reset", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d", w.Code)
	}

	snapshot := pm.GetMetrics().GetSnapshot()
	if snapshot["total_requests"] != int64(0) {
		t.Errorf("期望 total_requests=0, 实际=%v", snapshot["total_requests"])
	}
}

func TestPerfManager_GetPoolStats(t *testing.T) {
	pm := NewPerfManager()

	router := gin.New()
	pm.RegisterPerfRoutes(router.Group("/api"))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/webdav/perf/pool", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d", w.Code)
	}
}

// ========== 延迟桶测试 ==========

func TestPerfMetrics_LatencyDistribution(t *testing.T) {
	m := NewPerfMetrics()

	// 录入不同延迟
	m.RecordRequest("GET", 0, 0, 5, nil)   // 0-10ms
	m.RecordRequest("GET", 0, 0, 30, nil)  // 10-50ms
	m.RecordRequest("GET", 0, 0, 80, nil)  // 50-100ms
	m.RecordRequest("GET", 0, 0, 200, nil) // 100-500ms
	m.RecordRequest("GET", 0, 0, 800, nil) // 500-1000ms

	snapshot := m.GetSnapshot()
	buckets := snapshot["latency_dist"].([]LatencyBucket)

	if buckets[0].Count != 1 {
		t.Errorf("0-10ms 桶: 期望 count=1, 实际=%d", buckets[0].Count)
	}
	if buckets[1].Count != 1 {
		t.Errorf("10-50ms 桶: 期望 count=1, 实际=%d", buckets[1].Count)
	}
	if buckets[2].Count != 1 {
		t.Errorf("50-100ms 桶: 期望 count=1, 实际=%d", buckets[2].Count)
	}
	if buckets[3].Count != 1 {
		t.Errorf("100-500ms 桶: 期望 count=1, 实际=%d", buckets[3].Count)
	}
	if buckets[4].Count != 1 {
		t.Errorf("500-1000ms 桶: 期望 count=1, 实际=%d", buckets[4].Count)
	}
}

// ========== StreamConfig 测试 ==========

func TestDefaultStreamConfig(t *testing.T) {
	cfg := DefaultStreamConfig()
	if cfg.ChunkSize != DefaultChunkSize {
		t.Errorf("期望 chunk_size=%d, 实际=%d", DefaultChunkSize, cfg.ChunkSize)
	}
	if cfg.MaxConcurrent != 4 {
		t.Errorf("期望 max_concurrent=4, 实际=%d", cfg.MaxConcurrent)
	}
	if cfg.BufferSize != 64*1024 {
		t.Errorf("期望 buffer_size=65536, 实际=%d", cfg.BufferSize)
	}
}
