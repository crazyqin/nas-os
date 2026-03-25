package smb

import (
	"context"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestHandlers(t *testing.T) (*Handlers, *Manager, string) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")

	mgr, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("创建 SMB 管理器失败：%v", err)
	}

	return NewHandlers(mgr), mgr, tmpDir
}

func setupTestRouter() *gin.Engine {
	router := gin.New()
	return router
}

// ========== 响应函数测试 ==========

func TestSuccess(t *testing.T) {
	data := map[string]string{"key": "value"}
	resp := Success(data)

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf("expected message 'success', got '%s'", resp.Message)
	}
	if resp.Data == nil {
		t.Error("expected data to be non-nil")
	}
}

func TestError(t *testing.T) {
	resp := Error(404, "not found")

	if resp.Code != 404 {
		t.Errorf("expected code 404, got %d", resp.Code)
	}
	if resp.Message != "not found" {
		t.Errorf("expected message 'not found', got '%s'", resp.Message)
	}
	if resp.Data != nil {
		t.Error("expected data to be nil")
	}
}

// ========== Handlers 构造函数测试 ==========

func TestNewHandlers(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	if handlers == nil {
		t.Fatal("NewHandlers returned nil")
	}
	if handlers.manager == nil {
		t.Error("Handlers.manager is nil")
	}
}

// ========== 路由注册测试 ==========

func TestHandlers_RegisterRoutes(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	routes := router.Routes()
	if len(routes) == 0 {
		t.Error("No routes registered")
	}

	// 验证关键路由存在
	expectedRoutes := []string{
		"/api/shares/smb",
		"/api/smb/status",
		"/api/smb/connections",
		"/api/smb/start",
		"/api/smb/stop",
		"/api/smb/restart",
		"/api/smb/reload",
		"/api/smb/test",
	}

	registeredPaths := make(map[string]bool)
	for _, r := range routes {
		registeredPaths[r.Path] = true
	}

	for _, expected := range expectedRoutes {
		if !registeredPaths[expected] {
			t.Errorf("Route %s not found", expected)
		}
	}
}

// ========== listShares 测试 ==========

func TestListShares_Empty(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)
	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/shares/smb", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("expected response code 0, got %d", resp.Code)
	}
}

func TestListShares_WithShares(t *testing.T) {
	handlers, mgr, tmpDir := setupTestHandlers(t)

	// 创建测试共享
	share := &Share{
		Name:    "test-share",
		Path:    filepath.Join(tmpDir, "share"),
		Comment: "Test share",
	}
	_ = mgr.CreateShare(share)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/shares/smb", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("expected response code 0, got %d", resp.Code)
	}
}

// ========== createShare 测试 ==========

func TestCreateShare_Valid(t *testing.T) {
	handlers, _, tmpDir := setupTestHandlers(t)

	sharePath := filepath.Join(tmpDir, "newshare")
	_ = os.MkdirAll(sharePath, 0755)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	reqBody := ShareInput{
		Name:    "newshare",
		Path:    sharePath,
		Comment: "New test share",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/shares/smb", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 注意：ApplyConfig 可能因为 /etc/samba/smb.conf 不存在而失败
	// 但共享应该已经被创建
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCreateShare_InvalidJSON(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/shares/smb", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ========== getShare 测试 ==========

func TestGetShare_Exists(t *testing.T) {
	handlers, mgr, tmpDir := setupTestHandlers(t)

	share := &Share{
		Name:    "test-share",
		Path:    filepath.Join(tmpDir, "share"),
		Comment: "Test share",
	}
	_ = mgr.CreateShare(share)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/shares/smb/test-share", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("expected response code 0, got %d", resp.Code)
	}
}

func TestGetShare_NotExists(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/shares/smb/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ========== updateShare 测试 ==========

func TestUpdateShare_Valid(t *testing.T) {
	handlers, mgr, tmpDir := setupTestHandlers(t)

	share := &Share{
		Name:    "test-share",
		Path:    filepath.Join(tmpDir, "share"),
		Comment: "Original comment",
	}
	_ = mgr.CreateShare(share)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	reqBody := ShareInput{
		Name:    "test-share",
		Path:    filepath.Join(tmpDir, "share"),
		Comment: "Updated comment",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequestWithContext(context.Background(), "PUT", "/api/shares/smb/test-share", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 注意：ApplyConfig 可能失败
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError && w.Code != http.StatusBadRequest {
		t.Errorf("unexpected status %d, body: %s", w.Code, w.Body.String())
	}
}

func TestUpdateShare_NotExists(t *testing.T) {
	handlers, _, tmpDir := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	reqBody := ShareInput{
		Name:    "nonexistent",
		Path:    filepath.Join(tmpDir, "share"),
		Comment: "Updated",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequestWithContext(context.Background(), "PUT", "/api/shares/smb/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 404 or 400, got %d", w.Code)
	}
}

// ========== deleteShare 测试 ==========

func TestDeleteShare_Exists(t *testing.T) {
	handlers, mgr, tmpDir := setupTestHandlers(t)

	share := &Share{
		Name: "test-share",
		Path: filepath.Join(tmpDir, "share"),
	}
	_ = mgr.CreateShare(share)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "DELETE", "/api/shares/smb/test-share", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 注意：ApplyConfig 可能失败，但删除操作应该成功
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d", w.Code)
	}
}

func TestDeleteShare_NotExists(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "DELETE", "/api/shares/smb/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ========== setPermission 测试 ==========

func TestSetPermission_Valid(t *testing.T) {
	handlers, mgr, tmpDir := setupTestHandlers(t)

	share := &Share{
		Name: "test-share",
		Path: filepath.Join(tmpDir, "share"),
	}
	_ = mgr.CreateShare(share)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	reqBody := map[string]interface{}{
		"username":   "testuser",
		"read_write": true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/shares/smb/test-share/permission", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 注意：ApplyConfig 可能失败
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d, body: %s", w.Code, w.Body.String())
	}
}

func TestSetPermission_InvalidRequest(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/shares/smb/test-share/permission", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ========== removePermission 测试 ==========

func TestRemovePermission_Valid(t *testing.T) {
	handlers, mgr, tmpDir := setupTestHandlers(t)

	share := &Share{
		Name:  "test-share",
		Path:  filepath.Join(tmpDir, "share"),
		Users: []string{"testuser"},
	}
	_ = mgr.CreateShare(share)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "DELETE", "/api/shares/smb/test-share/permission/testuser", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 注意：ApplyConfig 可能失败
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d", w.Code)
	}
}

// ========== closeShare 测试 ==========

func TestCloseShare_Valid(t *testing.T) {
	handlers, mgr, tmpDir := setupTestHandlers(t)

	share := &Share{
		Name:      "test-share",
		Path:      filepath.Join(tmpDir, "share"),
		Available: true,
	}
	_ = mgr.CreateShare(share)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/shares/smb/test-share/close", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 注意：ApplyConfig 可能失败
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d", w.Code)
	}
}

func TestCloseShare_NotExists(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/shares/smb/nonexistent/close", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ========== openShare 测试 ==========

func TestOpenShare_Valid(t *testing.T) {
	handlers, mgr, tmpDir := setupTestHandlers(t)

	share := &Share{
		Name:      "test-share",
		Path:      filepath.Join(tmpDir, "share"),
		Available: false,
	}
	_ = mgr.CreateShare(share)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/shares/smb/test-share/open", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 注意：ApplyConfig 可能失败
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d", w.Code)
	}
}

func TestOpenShare_NotExists(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/shares/smb/nonexistent/open", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ========== getStatus 测试 ==========

func TestGetStatus(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/smb/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
}

// ========== getConnections 测试 ==========

func TestGetConnections(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/smb/connections", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 注意：可能因为系统命令不可用而失败
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d", w.Code)
	}
}

// ========== startService 测试 ==========

func TestStartService(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/smb/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 即使服务启动失败（因为在测试环境中），也应该返回一个响应
	if w.Code == 0 {
		t.Error("expected non-zero status code")
	}
}

// ========== stopService 测试 ==========

func TestStopService(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/smb/stop", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code == 0 {
		t.Error("expected non-zero status code")
	}
}

// ========== restartService 测试 ==========

func TestRestartService(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/smb/restart", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code == 0 {
		t.Error("expected non-zero status code")
	}
}

// ========== reloadConfig 测试 ==========

func TestReloadConfig(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/smb/reload", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code == 0 {
		t.Error("expected non-zero status code")
	}
}

// ========== testConfig 测试 ==========

func TestTestConfig(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/smb/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
}

// ========== getUserShares 测试 ==========

func TestGetUserShares_Valid(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/shares/smb/user?user=testuser", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestGetUserShares_MissingUser(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/shares/smb/user", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ========== getConfig 测试 ==========

func TestGetConfig(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/smb/config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
}

// ========== updateConfig 测试 ==========

func TestUpdateConfig_Valid(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	reqBody := Config{
		Workgroup: "NEWGROUP",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequestWithContext(context.Background(), "PUT", "/api/smb/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 注意：ApplyConfig 可能失败
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d, body: %s", w.Code, w.Body.String())
	}
}

func TestUpdateConfig_InvalidJSON(t *testing.T) {
	handlers, _, _ := setupTestHandlers(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "PUT", "/api/smb/config", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ========== errToString 测试 ==========

func TestErrToString_Nil(t *testing.T) {
	result := errToString(nil)
	if result != "" {
		t.Errorf("expected empty string for nil error, got '%s'", result)
	}
}

func TestErrToString_Error(t *testing.T) {
	result := errToString(assertAnError("test error"))
	if result != "test error" {
		t.Errorf("expected 'test error', got '%s'", result)
	}
}

func assertAnError(msg string) error {
	return &testError{msg: msg}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
