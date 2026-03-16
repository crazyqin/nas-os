package notify

import (
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

func TestNewHandlers(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "notify.json")

	manager := NewManager()
	h := NewHandlers(manager, configPath)

	if h == nil {
		t.Fatal("NewHandlers() returned nil")
	}
	if h.manager != manager {
		t.Error("Manager not set correctly")
	}
	if h.configPath != configPath {
		t.Error("Config path not set correctly")
	}
}

func TestNewHandlersWithExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "notify.json")

	// Create existing config
	config := Config{
		Email: EmailConfig{
			Enabled:  true,
			SMTP:     "smtp.test.com",
			Port:     587,
			Username: "test@test.com",
			Password: "password",
			From:     "test@test.com",
			To:       []string{"to@test.com"},
		},
		WeChat: WeChatConfig{
			Enabled:    true,
			WebhookURL: "https://test.webhook.url",
		},
	}
	data, _ := json.MarshalIndent(config, "", "  ")
	_ = os.WriteFile(configPath, data, 0644)

	manager := NewManager()
	h := NewHandlers(manager, configPath)

	if h == nil {
		t.Fatal("NewHandlers() returned nil")
	}
	if !h.config.Email.Enabled {
		t.Error("Email config not loaded correctly")
	}
	if !h.config.WeChat.Enabled {
		t.Error("WeChat config not loaded correctly")
	}
	// Check that notifiers were added
	if len(h.manager.notifiers) < 2 {
		t.Errorf("Expected at least 2 notifiers, got %d", len(h.manager.notifiers))
	}
}

func TestHandlersSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "notify.json")

	manager := NewManager()
	h := NewHandlers(manager, configPath)

	// Modify config
	h.config.Email.Enabled = true
	h.config.Email.SMTP = "smtp.example.com"
	h.config.Email.Port = 465

	err := h.saveConfig()
	if err != nil {
		t.Fatalf("saveConfig() returned error: %v", err)
	}

	// Read and verify
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	if !loaded.Email.Enabled {
		t.Error("Email.Enabled not saved correctly")
	}
	if loaded.Email.SMTP != "smtp.example.com" {
		t.Errorf("Email.SMTP = %s, expected smtp.example.com", loaded.Email.SMTP)
	}
}

func TestHandlersApplyConfig(t *testing.T) {
	manager := NewManager()
	h := &Handlers{
		manager: manager,
		config: &Config{
			Email: EmailConfig{
				Enabled:  true,
				SMTP:     "smtp.test.com",
				Port:     587,
				Username: "test",
				Password: "pass",
				From:     "from@test.com",
				To:       []string{"to@test.com"},
			},
			WeChat: WeChatConfig{
				Enabled:    true,
				WebhookURL: "https://test.webhook.url",
			},
			Webhooks: []WebhookConfig{
				{Enabled: true, Name: "test", URL: "https://hook.url"},
			},
		},
	}

	h.applyConfig()

	if len(h.manager.notifiers) != 3 {
		t.Errorf("Expected 3 notifiers, got %d", len(h.manager.notifiers))
	}

	// Verify notifier names
	names := make(map[string]bool)
	for _, n := range h.manager.notifiers {
		names[n.Name()] = true
	}
	if !names["email"] {
		t.Error("Email notifier not added")
	}
	if !names["wechat"] {
		t.Error("WeChat notifier not added")
	}
	if !names["webhook"] {
		t.Error("Webhook notifier not added")
	}
}

func TestHandlersApplyConfigDisabled(t *testing.T) {
	manager := NewManager()
	h := &Handlers{
		manager: manager,
		config: &Config{
			Email:  EmailConfig{Enabled: false},
			WeChat: WeChatConfig{Enabled: false},
		},
	}

	h.applyConfig()

	if len(h.manager.notifiers) != 0 {
		t.Errorf("Expected 0 notifiers for disabled config, got %d", len(h.manager.notifiers))
	}
}

func TestHandlersGetConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "notify.json")

	manager := NewManager()
	h := NewHandlers(manager, configPath)
	h.config.Email = EmailConfig{
		Enabled:  true,
		SMTP:     "smtp.test.com",
		Port:     587,
		Username: "user",
		Password: "secret",
		From:     "from@test.com",
		To:       []string{"to@test.com"},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.getConfig(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	email, ok := data["email"].(map[string]interface{})
	if !ok {
		t.Fatal("Email config not found in response")
	}

	// Password should be empty (not returned)
	if pwd, exists := email["password"]; exists && pwd != "" {
		t.Errorf("Password should be empty in config, got %v", pwd)
	}

	if email["smtp_server"] != "smtp.test.com" {
		t.Errorf("SMTP = %v, expected smtp.test.com", email["smtp_server"])
	}
}

func TestHandlersUpdateConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "notify.json")

	manager := NewManager()
	h := NewHandlers(manager, configPath)

	newConfig := Config{
		Email: EmailConfig{
			Enabled:  true,
			SMTP:     "smtp.new.com",
			Port:     587,
			Username: "newuser",
			Password: "newpass",
			From:     "new@test.com",
			To:       []string{"newto@test.com"},
		},
	}
	body, _ := json.Marshal(newConfig)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/config", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.updateConfig(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify config was updated
	if !h.config.Email.Enabled {
		t.Error("Email config not updated")
	}
	if h.config.Email.SMTP != "smtp.new.com" {
		t.Errorf("SMTP = %s, expected smtp.new.com", h.config.Email.SMTP)
	}

	// Verify config file was saved
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not saved")
	}
}

func TestHandlersUpdateConfigInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "notify.json")

	manager := NewManager()
	h := NewHandlers(manager, configPath)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/config", bytes.NewReader([]byte("invalid json")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.updateConfig(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandlersTestNotificationEmail(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "notify.json")

	manager := NewManager()
	h := NewHandlers(manager, configPath)
	h.config.Email = EmailConfig{
		Enabled:  true,
		SMTP:     "smtp.test.com",
		Port:     587,
		Username: "test",
		Password: "pass",
		From:     "from@test.com",
		To:       []string{"to@test.com"},
	}

	body, _ := json.Marshal(map[string]string{"channel": "email"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.testNotification(c)

	// Note: Email send will likely fail without real SMTP server
	// But we're testing the handler logic, not actual email delivery
	// The handler should attempt to send
}

func TestHandlersTestNotificationEmailNotEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "notify.json")

	manager := NewManager()
	h := NewHandlers(manager, configPath)
	h.config.Email = EmailConfig{Enabled: false}

	body, _ := json.Marshal(map[string]string{"channel": "email"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.testNotification(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandlersTestNotificationWeChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "notify.json")

	manager := NewManager()
	h := NewHandlers(manager, configPath)
	h.config.WeChat = WeChatConfig{
		Enabled:    true,
		WebhookURL: server.URL,
	}

	body, _ := json.Marshal(map[string]string{"channel": "wechat"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.testNotification(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlersTestNotificationWeChatNotEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "notify.json")

	manager := NewManager()
	h := NewHandlers(manager, configPath)
	h.config.WeChat = WeChatConfig{Enabled: false}

	body, _ := json.Marshal(map[string]string{"channel": "wechat"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.testNotification(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandlersTestNotificationInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "notify.json")

	manager := NewManager()
	h := NewHandlers(manager, configPath)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte("invalid")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.testNotification(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandlersTestNotificationAllChannels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "notify.json")

	manager := NewManager()
	h := NewHandlers(manager, configPath)
	h.config.WeChat = WeChatConfig{
		Enabled:    true,
		WebhookURL: server.URL,
	}
	h.applyConfig()

	body, _ := json.Marshal(map[string]string{"channel": "all"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.testNotification(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlersGetChannels(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "notify.json")

	manager := NewManager()
	h := NewHandlers(manager, configPath)
	h.config.Email = EmailConfig{
		Enabled:  true,
		SMTP:     "smtp.test.com",
		Port:     587,
		Username: "test",
		From:     "test@test.com",
		To:       []string{"to@test.com"},
	}
	h.config.WeChat = WeChatConfig{
		Enabled:    true,
		WebhookURL: "https://test.webhook.url",
	}
	h.config.Webhooks = []WebhookConfig{
		{Enabled: true, Name: "custom", URL: "https://hook.url"},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.getChannels(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data, ok := response["data"].([]interface{})
	if !ok {
		t.Fatal("Response data is not an array")
	}

	if len(data) != 3 {
		t.Errorf("Expected 3 channels, got %d", len(data))
	}
}

func TestHandlersGetChannelsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "notify.json")

	manager := NewManager()
	h := NewHandlers(manager, configPath)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.getChannels(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data, ok := response["data"].([]interface{})
	if !ok {
		t.Fatal("Response data is not an array")
	}

	if len(data) != 0 {
		t.Errorf("Expected 0 channels, got %d", len(data))
	}
}

func TestRegisterRoutes(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "notify.json")

	manager := NewManager()
	h := NewHandlers(manager, configPath)

	router := gin.New()
	group := router.Group("/api")
	h.RegisterRoutes(group)

	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, route := range routes {
		routeMap[route.Path] = true
	}

	expectedRoutes := []string{
		"/api/notify/config",
		"/api/notify/test",
		"/api/notify/channels",
	}

	for _, expected := range expectedRoutes {
		if !routeMap[expected] {
			t.Errorf("Expected route %s not found", expected)
		}
	}
}
