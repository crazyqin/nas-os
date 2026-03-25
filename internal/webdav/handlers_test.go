package webdav

import (
	"context"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ========== Handlers 测试 ==========

func TestHandlers_GetConfig(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "webdav-handler-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &Config{
		Enabled:    true,
		Port:       8081,
		RootPath:   tmpDir,
		AllowGuest: false,
	}

	srv, err := NewServer(config)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	handlers := NewHandlers(srv)

	// 创建测试路由
	router := gin.New()
	router.GET("/config", handlers.GetConfig)

	// 创建请求
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/config", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 验证响应
	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}

	var resp api.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("期望 Code 为 0, 实际为 %d", resp.Code)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("期望 Data 为 map[string]interface{}")
	}

	if int(data["port"].(float64)) != 8081 {
		t.Errorf("期望端口 8081, 实际为 %v", data["port"])
	}
}

func TestHandlers_UpdateConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webdav-handler-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &Config{
		Enabled:    true,
		Port:       8081,
		RootPath:   tmpDir,
		AllowGuest: false,
	}

	srv, err := NewServer(config)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	handlers := NewHandlers(srv)

	router := gin.New()
	router.PUT("/config", handlers.UpdateConfig)

	// 更新配置
	newPort := 9090
	enabled := true
	updateReq := UpdateConfigRequest{
		Port:    &newPort,
		Enabled: &enabled,
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequestWithContext(context.Background(), "PUT", "/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}

	// 验证配置已更新
	updatedConfig := srv.GetConfig()
	if updatedConfig.Port != 9090 {
		t.Errorf("期望端口 9090, 实际为 %d", updatedConfig.Port)
	}
}

func TestHandlers_GetStatus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webdav-handler-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &Config{
		Enabled:    true,
		Port:       8081,
		RootPath:   tmpDir,
		AllowGuest: false,
	}

	srv, err := NewServer(config)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	handlers := NewHandlers(srv)

	router := gin.New()
	router.GET("/status", handlers.GetStatus)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/status", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp["code"].(float64) != 0 {
		t.Errorf("期望 Code 为 0, 实际为 %v", resp["code"])
	}

	data := resp["data"].(map[string]interface{})
	if _, ok := data["enabled"]; !ok {
		t.Error("状态应该包含 enabled")
	}
	if _, ok := data["port"]; !ok {
		t.Error("状态应该包含 port")
	}
}

func TestHandlers_GetLocks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webdav-handler-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &Config{
		Enabled:    true,
		Port:       8081,
		RootPath:   tmpDir,
		AllowGuest: false,
	}

	srv, err := NewServer(config)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	handlers := NewHandlers(srv)

	router := gin.New()
	router.GET("/locks", handlers.GetLocks)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/locks", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}
}

func TestHandlers_DeleteLock(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webdav-handler-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &Config{
		Enabled:    true,
		Port:       8081,
		RootPath:   tmpDir,
		AllowGuest: false,
	}

	srv, err := NewServer(config)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	// 先创建一个锁
	lock, err := srv.lockManager.CreateLock("/test/file.txt", "owner-1", 0, "exclusive", 3600)
	if err != nil {
		t.Fatalf("创建锁失败: %v", err)
	}

	handlers := NewHandlers(srv)

	router := gin.New()
	router.DELETE("/locks/:token", handlers.DeleteLock)

	req := httptest.NewRequestWithContext(context.Background(), "DELETE", "/locks/"+lock.Token, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}

	// 验证锁已删除
	_, exists := srv.lockManager.GetLock(lock.Token)
	if exists {
		t.Errorf("期望锁已删除，但还能找到")
	}
}

func TestHandlers_DeleteLock_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webdav-handler-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &Config{
		Enabled:    true,
		Port:       8081,
		RootPath:   tmpDir,
		AllowGuest: false,
	}

	srv, err := NewServer(config)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	handlers := NewHandlers(srv)

	router := gin.New()
	router.DELETE("/locks/:token", handlers.DeleteLock)

	req := httptest.NewRequestWithContext(context.Background(), "DELETE", "/locks/nonexistent-token", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusNotFound, w.Code)
	}
}

// ========== 集成测试 ==========

func TestHandlers_FullWorkflow(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webdav-workflow-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &Config{
		Enabled:    true,
		Port:       8081,
		RootPath:   tmpDir,
		AllowGuest: false,
	}

	srv, err := NewServer(config)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	handlers := NewHandlers(srv)

	router := gin.New()
	router.GET("/config", handlers.GetConfig)
	router.PUT("/config", handlers.UpdateConfig)
	router.GET("/status", handlers.GetStatus)
	router.GET("/locks", handlers.GetLocks)
	router.DELETE("/locks/:token", handlers.DeleteLock)

	// 1. 获取初始配置
	req1 := httptest.NewRequestWithContext(context.Background(), "GET", "/config", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Errorf("步骤 1 失败: 状态码 %d", w1.Code)
	}

	// 2. 更新配置
	newPort := 9090
	updateReq := UpdateConfigRequest{Port: &newPort}
	body, _ := json.Marshal(updateReq)
	req2 := httptest.NewRequestWithContext(context.Background(), "PUT", "/config", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("步骤 2 失败: 状态码 %d", w2.Code)
	}

	// 3. 获取状态
	req3 := httptest.NewRequestWithContext(context.Background(), "GET", "/status", nil)
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Errorf("步骤 3 失败: 状态码 %d", w3.Code)
	}

	// 4. 创建锁
	lock, err := srv.lockManager.CreateLock("/test/workflow.txt", "test-owner", 0, "exclusive", 3600)
	if err != nil {
		t.Fatalf("创建锁失败: %v", err)
	}

	// 5. 获取锁列表
	req5 := httptest.NewRequestWithContext(context.Background(), "GET", "/locks", nil)
	w5 := httptest.NewRecorder()
	router.ServeHTTP(w5, req5)
	if w5.Code != http.StatusOK {
		t.Errorf("步骤 5 失败: 状态码 %d", w5.Code)
	}

	// 6. 删除锁
	req6 := httptest.NewRequestWithContext(context.Background(), "DELETE", "/locks/"+lock.Token, nil)
	w6 := httptest.NewRecorder()
	router.ServeHTTP(w6, req6)
	if w6.Code != http.StatusOK {
		t.Errorf("步骤 6 失败: 状态码 %d", w6.Code)
	}
}

// ========== 并发测试 ==========

func TestHandlers_ConcurrentAccess(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webdav-concurrent-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &Config{
		Enabled:    true,
		Port:       8081,
		RootPath:   tmpDir,
		AllowGuest: false,
	}

	srv, err := NewServer(config)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	handlers := NewHandlers(srv)

	router := gin.New()
	router.GET("/config", handlers.GetConfig)
	router.GET("/status", handlers.GetStatus)

	// 并发请求测试
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				req := httptest.NewRequestWithContext(context.Background(), "GET", "/config", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Errorf("并发请求 %d-%d 失败: 状态码 %d", id, j, w.Code)
				}
			}
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestHandlers_InvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webdav-invalid-json-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &Config{
		Enabled:    true,
		Port:       8081,
		RootPath:   tmpDir,
		AllowGuest: false,
	}

	srv, err := NewServer(config)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	handlers := NewHandlers(srv)

	router := gin.New()
	router.PUT("/config", handlers.UpdateConfig)

	// 发送无效的 JSON
	req := httptest.NewRequestWithContext(context.Background(), "PUT", "/config", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusBadRequest, w.Code)
	}
}
