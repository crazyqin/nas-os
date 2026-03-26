package auth

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
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestMFAManager(t *testing.T) (*MFAManager, string) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mfa.json")

	mgr, err := NewMFAManager(configPath, "test-issuer", nil)
	if err != nil {
		t.Fatalf("创建 MFA 管理器失败：%v", err)
	}

	return mgr, tmpDir
}

func setupTestHandlersWithManager(t *testing.T) (*Handlers, *MFAManager) {
	mgr, _ := setupTestMFAManager(t)
	return NewHandlers(mgr), mgr
}

func setupTestRouter() *gin.Engine {
	router := gin.New()
	return router
}

// ========== 响应结构测试 ==========

func TestNewHandlers(t *testing.T) {
	manager := &MFAManager{}
	handlers := NewHandlers(manager)

	if handlers == nil {
		t.Fatal("NewHandlers returned nil")
	}
	if handlers.manager == nil {
		t.Error("Handlers.manager is nil")
	}
}

func TestHandlers_RegisterRoutes(t *testing.T) {
	manager := &MFAManager{}
	handlers := NewHandlers(manager)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	routes := router.Routes()
	if len(routes) == 0 {
		t.Error("No routes registered")
	}

	// Check specific routes exist
	expectedRoutes := []string{
		"/api/mfa/status",
		"/api/mfa/totp/setup",
		"/api/mfa/totp/enable",
		"/api/mfa/totp/disable",
		"/api/mfa/sms/send",
		"/api/mfa/sms/enable",
		"/api/mfa/sms/disable",
		"/api/mfa/backup/generate",
		"/api/mfa/backup/status",
		"/api/mfa/verify",
	}

	for _, route := range expectedRoutes {
		found := false
		for _, r := range routes {
			if r.Path == route {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Route %s not found", route)
		}
	}
}

// ========== getStatus 测试 ==========

func TestGetStatus_Unauthorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	// 不设置 user_id
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/mfa/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestGetStatus_Authorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	api.Use(func(c *gin.Context) {
		c.Set("user_id", "test-user")
		c.Next()
	})
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/mfa/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// ========== setupTOTP 测试 ==========

func TestSetupTOTP_Unauthorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/totp/setup", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestSetupTOTP_Authorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	api.Use(func(c *gin.Context) {
		c.Set("user_id", "test-user")
		c.Set("username", "testuser")
		c.Next()
	})
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/totp/setup", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
}

// ========== enableTOTP 测试 ==========

func TestEnableTOTP_Unauthorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	body, _ := json.Marshal(EnableTOTPRequest{Code: "123456"})
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/totp/enable", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestEnableTOTP_MissingCode(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	api.Use(func(c *gin.Context) {
		c.Set("user_id", "test-user")
		c.Next()
	})
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/totp/enable", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ========== disableTOTP 测试 ==========

func TestDisableTOTP_Unauthorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	body, _ := json.Marshal(EnableTOTPRequest{Code: "123456"})
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/totp/disable", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// ========== sendSMS 测试 ==========

func TestSendSMS_Unauthorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	body, _ := json.Marshal(SendSMSRequest{Phone: "+8613800138000"})
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/sms/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestSendSMS_MissingPhone(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	api.Use(func(c *gin.Context) {
		c.Set("user_id", "test-user")
		c.Next()
	})
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/sms/send", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ========== enableSMS 测试 ==========

func TestEnableSMS_Unauthorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	body, _ := json.Marshal(EnableSMSRequest{Phone: "+8613800138000", Code: "123456"})
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/sms/enable", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestEnableSMS_MissingFields(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	api.Use(func(c *gin.Context) {
		c.Set("user_id", "test-user")
		c.Next()
	})
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/sms/enable", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ========== disableSMS 测试 ==========

func TestDisableSMS_Unauthorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	body, _ := json.Marshal(EnableTOTPRequest{Code: "123456"})
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/sms/disable", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// ========== generateBackupCodes 测试 ==========

func TestGenerateBackupCodes_Unauthorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/backup/generate", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestGenerateBackupCodes_MFAEnabled(t *testing.T) {
	handlers, mgr := setupTestHandlersWithManager(t)

	// 设置用户配置，启用 MFA
	mgr.mu.Lock()
	mgr.configs["test-user"] = &MFAConfig{
		UserID:      "test-user",
		Enabled:     true,
		TOTPEnabled: true,
		CreatedAt:   time.Now(),
	}
	mgr.mu.Unlock()

	router := setupTestRouter()
	api := router.Group("/api")
	api.Use(func(c *gin.Context) {
		c.Set("user_id", "test-user")
		c.Next()
	})
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/backup/generate", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestGenerateBackupCodes_MFANotEnabled(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	api.Use(func(c *gin.Context) {
		c.Set("user_id", "test-user")
		c.Next()
	})
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/backup/generate", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 应该返回 400，因为 MFA 未启用
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ========== getBackupStatus 测试 ==========

func TestGetBackupStatus_Unauthorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/mfa/backup/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestGetBackupStatus_Authorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	api.Use(func(c *gin.Context) {
		c.Set("user_id", "test-user")
		c.Next()
	})
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/mfa/backup/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// ========== beginWebAuthnRegistration 测试 ==========

func TestBeginWebAuthnRegistration_Unauthorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	body, _ := json.Marshal(WebAuthnRegisterStartRequest{DisplayName: "My Key"})
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/webauthn/register/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// ========== finishWebAuthnRegistration 测试 ==========

func TestFinishWebAuthnRegistration_Unauthorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/webauthn/register/finish", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// ========== beginWebAuthnAuthentication 测试 ==========

func TestBeginWebAuthnAuthentication_Unauthorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/webauthn/authenticate/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// ========== finishWebAuthnAuthentication 测试 ==========

func TestFinishWebAuthnAuthentication_Unauthorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/webauthn/authenticate/finish", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// ========== getWebAuthnCredentials 测试 ==========

func TestGetWebAuthnCredentials_Unauthorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/mfa/webauthn/credentials", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestGetWebAuthnCredentials_Authorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	api.Use(func(c *gin.Context) {
		c.Set("user_id", "test-user")
		c.Next()
	})
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/mfa/webauthn/credentials", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// ========== removeWebAuthnCredential 测试 ==========

func TestRemoveWebAuthnCredential_Unauthorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "DELETE", "/api/mfa/webauthn/credentials/cred-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// ========== verifyMFA 测试 ==========

func TestVerifyMFA_Unauthorized(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	body, _ := json.Marshal(VerifyMFARequest{MFAType: "totp", Code: "123456"})
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestVerifyMFA_MissingMFAType(t *testing.T) {
	handlers, _ := setupTestHandlersWithManager(t)

	router := setupTestRouter()
	api := router.Group("/api")
	api.Use(func(c *gin.Context) {
		c.Set("user_id", "test-user")
		c.Next()
	})
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/mfa/verify", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ========== TOTPSetupResponse_Fields 测试 ==========

func TestTOTPSetupResponse_Fields(t *testing.T) {
	resp := TOTPSetupResponse{
		Secret:      "JBSWY3DPEHPK3PXP",
		URI:         "otpauth://totp/test:user?secret=JBSWY3DPEHPK3PXP",
		QRCode:      "base64encodedpng",
		Issuer:      "test",
		AccountName: "user",
	}

	if resp.Secret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("Secret mismatch: %s", resp.Secret)
	}
	if resp.Issuer != "test" {
		t.Errorf("Issuer mismatch: %s", resp.Issuer)
	}
}

func TestEnableTOTPRequest_Binding(t *testing.T) {
	req := EnableTOTPRequest{
		Code: "123456",
	}

	if req.Code != "123456" {
		t.Errorf("Code mismatch: %s", req.Code)
	}
}

func TestSendSMSRequest_Binding(t *testing.T) {
	req := SendSMSRequest{
		Phone: "+8613800138000",
	}

	if req.Phone != "+8613800138000" {
		t.Errorf("Phone mismatch: %s", req.Phone)
	}
}

func TestEnableSMSRequest_Binding(t *testing.T) {
	req := EnableSMSRequest{
		Phone: "+8613800138000",
		Code:  "123456",
	}

	if req.Phone != "+8613800138000" {
		t.Errorf("Phone mismatch: %s", req.Phone)
	}
	if req.Code != "123456" {
		t.Errorf("Code mismatch: %s", req.Code)
	}
}

func TestWebAuthnRegisterStartRequest_Binding(t *testing.T) {
	req := WebAuthnRegisterStartRequest{
		DisplayName: "My Key",
	}

	if req.DisplayName != "My Key" {
		t.Errorf("DisplayName mismatch: %s", req.DisplayName)
	}
}

func TestWebAuthnRegisterStartResponse_Fields(t *testing.T) {
	resp := WebAuthnRegisterStartResponse{
		SessionID: "session-123",
		Options:   map[string]interface{}{"challenge": "abc"},
	}

	if resp.SessionID != "session-123" {
		t.Errorf("SessionID mismatch: %s", resp.SessionID)
	}
}

func TestVerifyMFARequest_Binding(t *testing.T) {
	req := VerifyMFARequest{
		MFAType: "totp",
		Code:    "123456",
	}

	if req.MFAType != "totp" {
		t.Errorf("MFAType mismatch: %s", req.MFAType)
	}
	if req.Code != "123456" {
		t.Errorf("Code mismatch: %s", req.Code)
	}
}

func TestBackupCodesResponse_Fields(t *testing.T) {
	resp := BackupCodesResponse{
		Codes: []string{"code1", "code2", "code3"},
	}

	if len(resp.Codes) != 3 {
		t.Errorf("Codes count mismatch: %d", len(resp.Codes))
	}
}

// ========== 辅助函数测试 ==========

func TestSetupTestMFAManager(t *testing.T) {
	mgr, tmpDir := setupTestMFAManager(t)

	if mgr == nil {
		t.Error("MFA manager is nil")
	}
	if tmpDir == "" {
		t.Error("Temp dir is empty")
	}

	// 检查临时目录是否存在
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Error("Temp dir does not exist")
	}
}
