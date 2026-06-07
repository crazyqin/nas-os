package voicehub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestProcessVoiceCommand(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{"movie mode", "激活观影模式", false},
		{"security mode", "安防模式", false},
		{"energy saving", "节能模式", false},
		{"turn on tv", "打开电视", false},
		{"turn off light", "关闭灯光", false},
		{"volume up", "音量调大", false},
		{"volume down", "音量调小", false},
		{"play", "播放音乐", false},
		{"pause", "暂停", false},
		{"speak", "说你好世界", false},
		{"general", "你好", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := m.ProcessVoiceCommand(ctx, &VoiceCommand{
				Query: tt.query,
			})
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Status != CommandStatusSuccess {
				t.Errorf("expected status success, got %v", resp.Status)
			}
			if resp.TextReply == "" {
				t.Error("expected non-empty text reply")
			}
		})
	}
}

func TestProcessVoiceCommandDisabled(t *testing.T) {
	cfg := DefaultVoiceHubConfig()
	cfg.Enabled = false
	m := NewManager(zap.NewNop(), cfg)

	_, err := m.ProcessVoiceCommand(context.Background(), &VoiceCommand{
		Query: "test",
	})
	if err == nil {
		t.Error("expected error when disabled")
	}
}

func TestSceneManagement(t *testing.T) {
	m := setupTestManager(t)

	// List default scenes
	scenes := m.ListScenes()
	if len(scenes) < 5 {
		t.Errorf("expected at least 5 default scenes, got %d", len(scenes))
	}

	// Create scene
	scene, err := m.CreateScene(&SceneRequest{
		Name:        "测试场景",
		Type:        SceneCustom,
		Description: "测试用场景",
		Devices: []DeviceAction{
			{DeviceID: "test-001", DeviceName: "测试设备", CommandType: CmdPowerOn},
		},
	})
	if err != nil {
		t.Fatalf("CreateScene failed: %v", err)
	}
	if scene.ID == "" {
		t.Error("expected non-empty scene ID")
	}

	// Get scene
	got, err := m.GetScene(scene.ID)
	if err != nil {
		t.Fatalf("GetScene failed: %v", err)
	}
	if got.Name != "测试场景" {
		t.Errorf("expected name '测试场景', got '%s'", got.Name)
	}

	// Update scene
	updated, err := m.UpdateScene(scene.ID, &SceneRequest{
		Name:        "更新后的场景",
		Type:        SceneCustom,
		Description: "更新描述",
		Devices: []DeviceAction{
			{DeviceID: "test-002", DeviceName: "更新设备", CommandType: CmdPowerOff},
		},
	})
	if err != nil {
		t.Fatalf("UpdateScene failed: %v", err)
	}
	if updated.Name != "更新后的场景" {
		t.Errorf("expected name '更新后的场景', got '%s'", updated.Name)
	}

	// Delete scene
	if err := m.DeleteScene(scene.ID); err != nil {
		t.Fatalf("DeleteScene failed: %v", err)
	}

	_, err = m.GetScene(scene.ID)
	if err == nil {
		t.Error("expected error for deleted scene")
	}
}

func TestActivateScene(t *testing.T) {
	m := setupTestManager(t)

	// Activate default movie scene
	resp, err := m.ActivateScene(context.Background(), "scene-movie")
	if err != nil {
		t.Fatalf("ActivateScene failed: %v", err)
	}
	if resp.Status != CommandStatusSuccess {
		t.Errorf("expected status success, got %v", resp.Status)
	}
	if resp.Scene == nil {
		t.Error("expected non-nil scene")
	}
	if len(resp.Actions) == 0 {
		t.Error("expected actions")
	}
}

func TestCustomCommandManagement(t *testing.T) {
	m := setupTestManager(t)

	// Register custom command
	cmd, err := m.RegisterCustomCommand(&CustomCommandRequest{
		Name:     "测试命令",
		Pattern:  "测试.*命令",
		Response: "这是测试回复",
		Actions: []DeviceAction{
			{DeviceID: "test-001", DeviceName: "测试设备", CommandType: CmdPowerOn},
		},
	})
	if err != nil {
		t.Fatalf("RegisterCustomCommand failed: %v", err)
	}
	if cmd.ID == "" {
		t.Error("expected non-empty command ID")
	}

	// Get command
	got, err := m.GetCustomCommand(cmd.ID)
	if err != nil {
		t.Fatalf("GetCustomCommand failed: %v", err)
	}
	if got.Name != "测试命令" {
		t.Errorf("expected name '测试命令', got '%s'", got.Name)
	}

	// List commands
	cmds := m.ListCustomCommands()
	if len(cmds) < 1 {
		t.Errorf("expected at least 1 custom command, got %d", len(cmds))
	}

	// Update command
	updated, err := m.UpdateCustomCommand(cmd.ID, &CustomCommandRequest{
		Name:     "更新后的命令",
		Pattern:  "更新.*命令",
		Response: "更新后的回复",
	})
	if err != nil {
		t.Fatalf("UpdateCustomCommand failed: %v", err)
	}
	if updated.Name != "更新后的命令" {
		t.Errorf("expected name '更新后的命令', got '%s'", updated.Name)
	}

	// Delete command
	if err := m.DeleteCustomCommand(cmd.ID); err != nil {
		t.Fatalf("DeleteCustomCommand failed: %v", err)
	}

	_, err = m.GetCustomCommand(cmd.ID)
	if err == nil {
		t.Error("expected error for deleted command")
	}
}

func TestWakeWordManagement(t *testing.T) {
	m := setupTestManager(t)

	// List default wake words
	words := m.ListWakeWords()
	if len(words) < 1 {
		t.Errorf("expected at least 1 wake word, got %d", len(words))
	}

	// Register new wake word
	word := m.RegisterWakeWord(&WakeWordConfig{
		WakeWord:    "测试唤醒词",
		Platform:    PlatformLocal,
		Language:    LangChinese,
		Sensitivity: 0.9,
	})
	if word.ID == "" {
		t.Error("expected non-empty wake word ID")
	}

	// Delete wake word
	if err := m.DeleteWakeWord(word.ID); err != nil {
		t.Fatalf("DeleteWakeWord failed: %v", err)
	}
}

func TestReplyTemplateManagement(t *testing.T) {
	m := setupTestManager(t)

	// List default templates
	tpls := m.ListReplyTemplates()
	if len(tpls) < 3 {
		t.Errorf("expected at least 3 templates, got %d", len(tpls))
	}

	// Create template
	tpl := m.CreateReplyTemplate(&ReplyTemplateRequest{
		Name:     "测试模板",
		Template: "你好，{{name}}！欢迎使用{{product}}。",
		Category: "greeting",
	})
	if tpl.ID == "" {
		t.Error("expected non-empty template ID")
	}
	if len(tpl.Variables) != 2 {
		t.Errorf("expected 2 variables, got %d", len(tpl.Variables))
	}

	// Get template
	got, err := m.GetReplyTemplate(tpl.ID)
	if err != nil {
		t.Fatalf("GetReplyTemplate failed: %v", err)
	}
	if got.Name != "测试模板" {
		t.Errorf("expected name '测试模板', got '%s'", got.Name)
	}

	// Update template
	updated, err := m.UpdateReplyTemplate(tpl.ID, &ReplyTemplateRequest{
		Name:     "更新后的模板",
		Template: "欢迎，{{user}}！",
		Category: "welcome",
	})
	if err != nil {
		t.Fatalf("UpdateReplyTemplate failed: %v", err)
	}
	if updated.Name != "更新后的模板" {
		t.Errorf("expected name '更新后的模板', got '%s'", updated.Name)
	}

	// Render template
	rendered, err := m.RenderTemplate(tpl.ID, map[string]string{"user": "张三"})
	if err != nil {
		t.Fatalf("RenderTemplate failed: %v", err)
	}
	if rendered != "欢迎，张三！" {
		t.Errorf("expected '欢迎，张三！', got '%s'", rendered)
	}

	// Delete template
	if err := m.DeleteReplyTemplate(tpl.ID); err != nil {
		t.Fatalf("DeleteReplyTemplate failed: %v", err)
	}
}

func TestTTS(t *testing.T) {
	m := setupTestManager(t)

	// Basic TTS
	resp, err := m.ConvertTTS(&TTSRequest{
		Text: "你好世界",
	})
	if err != nil {
		t.Fatalf("ConvertTTS failed: %v", err)
	}
	if resp.ID == "" {
		t.Error("expected non-empty TTS ID")
	}
	if resp.AudioURL == "" {
		t.Error("expected non-empty audio URL")
	}

	// TTS with options
	resp, err = m.ConvertTTS(&TTSRequest{
		Text:     "Hello World",
		Language: LangEnglish,
		Voice:    "en-US-Neural2",
		Speed:    1.5,
		Pitch:    1.2,
	})
	if err != nil {
		t.Fatalf("ConvertTTS with options failed: %v", err)
	}
	if resp.AudioURL == "" {
		t.Error("expected non-empty audio URL")
	}
}

func TestCommandHistory(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	// Execute multiple commands
	for i := 0; i < 5; i++ {
		m.ProcessVoiceCommand(ctx, &VoiceCommand{
			Query: fmt.Sprintf("test command %d", i),
		})
	}

	history := m.GetCommandHistory(3)
	if len(history) != 3 {
		t.Errorf("expected 3 history items, got %d", len(history))
	}
}

func TestDisabledVoiceHub(t *testing.T) {
	cfg := DefaultVoiceHubConfig()
	cfg.Enabled = false
	m := NewManager(zap.NewNop(), cfg)

	_, err := m.ProcessVoiceCommand(context.Background(), &VoiceCommand{
		Query: "test",
	})
	if err == nil {
		t.Error("expected error when disabled")
	}
}

func TestHandler_ProcessCommand(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"query":"打开电视"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/voicehub/command", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestHandler_ListScenes(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/voicehub/scenes", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestHandler_CreateScene(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"name":"测试场景","type":"custom","description":"测试","devices":[{"device_id":"test-001","device_name":"测试设备","command_type":"power_on"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/voicehub/scenes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ActivateScene(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/voicehub/scenes/scene-movie/activate", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_CustomCommands(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// Register command
	body := `{"name":"测试","pattern":"测试.*命令","response":"测试回复"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/voicehub/commands", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	cmdData, _ := json.Marshal(resp.Data)
	var cmd CustomCommand
	json.Unmarshal(cmdData, &cmd)

	// Get command
	req = httptest.NewRequest(http.MethodGet, "/api/v1/voicehub/commands/"+cmd.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// List commands
	req = httptest.NewRequest(http.MethodGet, "/api/v1/voicehub/commands", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Delete command
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/voicehub/commands/"+cmd.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_WakeWords(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// Register wake word
	body := `{"wake_word":"测试唤醒","platform":"local","language":"zh-CN","sensitivity":0.8}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/voicehub/wake-words", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// List wake words
	req = httptest.NewRequest(http.MethodGet, "/api/v1/voicehub/wake-words", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_Templates(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// Create template
	body := `{"name":"测试模板","template":"你好，{{name}}！","category":"greeting"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/voicehub/templates", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	tplData, _ := json.Marshal(resp.Data)
	var tpl ReplyTemplate
	json.Unmarshal(tplData, &tpl)

	// Render template
	renderBody := `{"name":"世界"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/voicehub/templates/"+tpl.ID+"/render", bytes.NewBufferString(renderBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Delete template
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/voicehub/templates/"+tpl.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_TTS(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"text":"你好世界","language":"zh-CN"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/voicehub/tts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_History(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/voicehub/history?limit=10", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_Config(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// Get config
	req := httptest.NewRequest(http.MethodGet, "/api/v1/voicehub/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Update config
	body := `{"enabled":true,"default_platform":"alexa","default_language":"en-US"}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/voicehub/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_PlatformsAndLanguages(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// Get platforms
	req := httptest.NewRequest(http.MethodGet, "/api/v1/voicehub/platforms", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Get languages
	req = httptest.NewRequest(http.MethodGet, "/api/v1/voicehub/languages", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestParseIntent(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		query      string
		lang       VoiceLanguage
		wantType   string
		wantAction string
	}{
		{"激活观影模式", LangChinese, "scene", "activate"},
		{"打开电视", LangChinese, "device", "power_on"},
		{"关闭灯光", LangChinese, "device", "power_off"},
		{"音量调大", LangChinese, "device", "volume_up"},
		{"音量调小", LangChinese, "device", "volume_down"},
		{"播放音乐", LangChinese, "device", "play"},
		{"暂停", LangChinese, "device", "pause"},
		{"说你好世界", LangChinese, "tts", "speak"},
		{"你好", LangChinese, "general", "query"},
		{"activate movie mode", LangEnglish, "scene", "activate"},
		{"turn on tv", LangEnglish, "device", "power_on"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			intent := m.parseIntent(tt.query, tt.lang)
			if intent.Type != tt.wantType {
				t.Errorf("expected type %s, got %s", tt.wantType, intent.Type)
			}
			if intent.Action != tt.wantAction {
				t.Errorf("expected action %s, got %s", tt.wantAction, intent.Action)
			}
		})
	}
}

func TestSupportedPlatformsAndLanguages(t *testing.T) {
	platforms := SupportedPlatforms()
	if len(platforms) != 5 {
		t.Errorf("expected 5 platforms, got %d", len(platforms))
	}

	languages := SupportedLanguages()
	if len(languages) != 3 {
		t.Errorf("expected 3 languages, got %d", len(languages))
	}

	if !IsValidPlatform(PlatformAlexa) {
		t.Error("expected alexa to be valid platform")
	}
	if !IsValidLanguage(LangChinese) {
		t.Error("expected zh-CN to be valid language")
	}
	if IsValidPlatform("invalid") {
		t.Error("expected 'invalid' to be invalid platform")
	}
	if IsValidLanguage("invalid") {
		t.Error("expected 'invalid' to be invalid language")
	}
}

func TestDefaultVoiceHubConfig(t *testing.T) {
	cfg := DefaultVoiceHubConfig()

	if !cfg.Enabled {
		t.Error("expected enabled to be true")
	}
	if cfg.DefaultPlatform != PlatformLocal {
		t.Errorf("expected default platform %s, got %s", PlatformLocal, cfg.DefaultPlatform)
	}
	if cfg.DefaultLanguage != LangChinese {
		t.Errorf("expected default language %s, got %s", LangChinese, cfg.DefaultLanguage)
	}
	if !cfg.TTSEnabled {
		t.Error("expected TTS to be enabled")
	}
	if cfg.MaxHistory != 1000 {
		t.Errorf("expected max history 1000, got %d", cfg.MaxHistory)
	}
}
