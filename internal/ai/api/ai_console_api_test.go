// Package api provides HTTP API handlers for AI Console
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nas-os/internal/ai"

	"github.com/stretchr/testify/assert"
)

// ==================== Tests ====================

func TestNewAIConsoleAPI(t *testing.T) {
	console := ai.NewConsole(ai.DefaultConsoleConfig())
	desensitizer := ai.NewDesensitizer()
	api := NewAIConsoleAPI(console, desensitizer)

	assert.NotNil(t, api)
	assert.NotNil(t, api.console)
	assert.NotNil(t, api.desensitizer)
}

func TestAIConsoleAPI_Chat(t *testing.T) {
	console := ai.NewConsole(ai.DefaultConsoleConfig())
	api := NewAIConsoleAPI(console, ai.NewDesensitizer())

	tests := []struct {
		name       string
		body       ChatRequest
		wantStatus int
	}{
		{
			name: "valid request (returns 500 without provider)",
			body: ChatRequest{
				Prompt:   "Hello",
				Provider: ai.ProviderOpenAI,
			},
			wantStatus: http.StatusInternalServerError, // No provider registered
		},
		{
			name: "empty prompt",
			body: ChatRequest{
				Provider: ai.ProviderOpenAI,
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/ai/chat", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			api.Chat(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestAIConsoleAPI_Summarize(t *testing.T) {
	console := ai.NewConsole(ai.DefaultConsoleConfig())
	api := NewAIConsoleAPI(console, ai.NewDesensitizer())

	tests := []struct {
		name       string
		body       SummarizeRequest
		wantStatus int
	}{
		{
			name: "valid request (returns 500 without provider)",
			body: SummarizeRequest{
				Content:   "This is a long piece of content that needs to be summarized.",
				MaxLength: 100,
			},
			wantStatus: http.StatusInternalServerError, // No provider registered
		},
		{
			name: "empty content",
			body: SummarizeRequest{
				MaxLength: 100,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "default max length (returns 500 without provider)",
			body: SummarizeRequest{
				Content: "Some content",
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/ai/summarize", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			api.Summarize(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestAIConsoleAPI_Translate(t *testing.T) {
	console := ai.NewConsole(ai.DefaultConsoleConfig())
	api := NewAIConsoleAPI(console, ai.NewDesensitizer())

	tests := []struct {
		name       string
		body       TranslateRequest
		wantStatus int
	}{
		{
			name: "valid request (returns 500 without provider)",
			body: TranslateRequest{
				Text:       "Hello",
				TargetLang: "zh",
			},
			wantStatus: http.StatusInternalServerError, // No provider registered
		},
		{
			name: "missing text",
			body: TranslateRequest{
				TargetLang: "zh",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing targetLang",
			body: TranslateRequest{
				Text: "Hello",
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/ai/translate", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			api.Translate(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestAIConsoleAPI_Sentiment(t *testing.T) {
	console := ai.NewConsole(ai.DefaultConsoleConfig())
	api := NewAIConsoleAPI(console, ai.NewDesensitizer())

	tests := []struct {
		name       string
		body       map[string]string
		wantStatus int
	}{
		{
			name:       "valid request (returns 500 without provider)",
			body:       map[string]string{"text": "I love this product!"},
			wantStatus: http.StatusInternalServerError, // No provider registered
		},
		{
			name:       "empty text",
			body:       map[string]string{},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/ai/sentiment", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			api.Sentiment(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestAIConsoleAPI_GetProviders(t *testing.T) {
	console := ai.NewConsole(ai.DefaultConsoleConfig())
	api := NewAIConsoleAPI(console, ai.NewDesensitizer())

	req := httptest.NewRequest(http.MethodGet, "/api/ai/providers", nil)
	rec := httptest.NewRecorder()

	api.GetProviders(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.Contains(t, resp, "providers")
}

func TestAIConsoleAPI_GetUsage(t *testing.T) {
	console := ai.NewConsole(ai.DefaultConsoleConfig())
	api := NewAIConsoleAPI(console, ai.NewDesensitizer())

	req := httptest.NewRequest(http.MethodGet, "/api/ai/usage", nil)
	rec := httptest.NewRecorder()

	api.GetUsage(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAIConsoleAPI_GetAuditLogs(t *testing.T) {
	console := ai.NewConsole(ai.DefaultConsoleConfig())
	api := NewAIConsoleAPI(console, ai.NewDesensitizer())

	req := httptest.NewRequest(http.MethodGet, "/api/ai/audit-logs", nil)
	rec := httptest.NewRecorder()

	api.GetAuditLogs(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAIConsoleAPI_Desensitize(t *testing.T) {
	console := ai.NewConsole(ai.DefaultConsoleConfig())
	api := NewAIConsoleAPI(console, ai.NewDesensitizer())

	tests := []struct {
		name       string
		body       ai.DesensitizeRequest
		wantStatus int
	}{
		{
			name: "valid request",
			body: ai.DesensitizeRequest{
				Text: "My phone is 13812345678",
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "empty text",
			body:       ai.DesensitizeRequest{},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/ai/desensitize", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			api.Desensitize(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestAIConsoleAPI_Restore(t *testing.T) {
	console := ai.NewConsole(ai.DefaultConsoleConfig())
	api := NewAIConsoleAPI(console, ai.NewDesensitizer())

	tests := []struct {
		name       string
		body       ai.RestoreRequest
		wantStatus int
	}{
		{
			name: "valid request",
			body: ai.RestoreRequest{
				Text: "My phone is [PHONE_1]",
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "empty text",
			body:       ai.RestoreRequest{},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/ai/restore", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			api.Restore(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestAIConsoleAPI_GetRules(t *testing.T) {
	console := ai.NewConsole(ai.DefaultConsoleConfig())
	api := NewAIConsoleAPI(console, ai.NewDesensitizer())

	req := httptest.NewRequest(http.MethodGet, "/api/ai/rules", nil)
	rec := httptest.NewRecorder()

	api.GetRules(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.Contains(t, resp, "rules")
}

func TestAIConsoleAPI_AddRule(t *testing.T) {
	console := ai.NewConsole(ai.DefaultConsoleConfig())
	api := NewAIConsoleAPI(console, ai.NewDesensitizer())

	rule := ai.DesensitizationRule{
		Name:        "test_rule",
		Pattern:     `\d{6}`,
		Replacement: "[CODE]",
		Enabled:     true,
	}

	body, _ := json.Marshal(rule)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.AddRule(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAIConsoleAPI_ProcessPII(t *testing.T) {
	console := ai.NewConsole(ai.DefaultConsoleConfig())
	api := NewAIConsoleAPI(console, ai.NewDesensitizer())

	tests := []struct {
		name       string
		body       map[string]string
		wantStatus int
	}{
		{
			name:       "valid request",
			body:       map[string]string{"text": "Email: test@example.com, Phone: 13812345678"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "empty text",
			body:       map[string]string{},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/ai/process-pii", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			api.ProcessPII(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusBadRequest, "test error")

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.Equal(t, "Bad Request", resp.Error)
	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, "test error", resp.Message)
}

func TestTimestamp(t *testing.T) {
	ts := Timestamp()
	assert.NotEmpty(t, ts)
	// Should be valid RFC3339 format
	_, err := time.Parse(time.RFC3339, ts)
	assert.NoError(t, err)
}