package auth

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

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
