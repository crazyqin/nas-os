// Package api provides HTTP API handlers for AI Console
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"nas-os/internal/ai"
)

// AIConsoleAPI provides API handlers for AI Console
type AIConsoleAPI struct {
	console       *ai.Console
	desensitizer  *ai.Desensitizer
}

// NewAIConsoleAPI creates a new AI Console API
func NewAIConsoleAPI(console *ai.Console, desensitizer *ai.Desensitizer) *AIConsoleAPI {
	return &AIConsoleAPI{
		console:      console,
		desensitizer: desensitizer,
	}
}

// RegisterRoutes registers API routes
func (api *AIConsoleAPI) RegisterRoutes(mux *http.ServeMux) {
	// AI Console
	mux.HandleFunc("POST /api/ai/chat", api.Chat)
	mux.HandleFunc("POST /api/ai/summarize", api.Summarize)
	mux.HandleFunc("POST /api/ai/translate", api.Translate)
	mux.HandleFunc("POST /api/ai/sentiment", api.Sentiment)
	mux.HandleFunc("GET /api/ai/providers", api.GetProviders)
	mux.HandleFunc("GET /api/ai/usage", api.GetUsage)
	mux.HandleFunc("GET /api/ai/audit-logs", api.GetAuditLogs)

	// Desensitization
	mux.HandleFunc("POST /api/ai/desensitize", api.Desensitize)
	mux.HandleFunc("POST /api/ai/restore", api.Restore)
	mux.HandleFunc("GET /api/ai/rules", api.GetRules)
	mux.HandleFunc("POST /api/ai/rules", api.AddRule)
	mux.HandleFunc("PUT /api/ai/rules/{id}", api.UpdateRule)
	mux.HandleFunc("DELETE /api/ai/rules/{id}", api.DeleteRule)

	// PII Processing
	mux.HandleFunc("POST /api/ai/process-pii", api.ProcessPII)
}

// ChatRequest represents a chat request
type ChatRequest struct {
	Prompt   string         `json:"prompt"`
	Model    string         `json:"model,omitempty"`
	Provider ai.Provider    `json:"provider,omitempty"`
	Options  ai.RequestOptions `json:"options,omitempty"`
	UserID   string         `json:"userId,omitempty"`
}

// Chat handles chat requests
func (api *AIConsoleAPI) Chat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Prompt == "" {
		http.Error(w, "prompt required", http.StatusBadRequest)
		return
	}

	resp, err := api.console.Process(r.Context(), &ai.ConsoleRequest{
		TaskType:   ai.TaskPhotoSearch, // Default
		Capability: ai.CapabilityChat,
		Provider:   req.Provider,
		Model:      req.Model,
		Prompt:     req.Prompt,
		Options:    req.Options,
		UserID:     req.UserID,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp)
}

// SummarizeRequest represents a summarize request
type SummarizeRequest struct {
	Content  string `json:"content"`
	MaxLength int   `json:"maxLength,omitempty"`
}

// Summarize handles summarization requests
func (api *AIConsoleAPI) Summarize(w http.ResponseWriter, r *http.Request) {
	var req SummarizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		http.Error(w, "content required", http.StatusBadRequest)
		return
	}

	maxLength := req.MaxLength
	if maxLength <= 0 {
		maxLength = 200
	}

	summary, err := api.console.Summarize(r.Context(), req.Content, maxLength)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"summary": summary,
	})
}

// TranslateRequest represents a translate request
type TranslateRequest struct {
	Text       string `json:"text"`
	TargetLang string `json:"targetLang"`
}

// Translate handles translation requests
func (api *AIConsoleAPI) Translate(w http.ResponseWriter, r *http.Request) {
	var req TranslateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Text == "" || req.TargetLang == "" {
		http.Error(w, "text and targetLang required", http.StatusBadRequest)
		return
	}

	translation, err := api.console.Translate(r.Context(), req.Text, req.TargetLang)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"translation": translation,
	})
}

// Sentiment handles sentiment analysis requests
func (api *AIConsoleAPI) Sentiment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Text == "" {
		http.Error(w, "text required", http.StatusBadRequest)
		return
	}

	result, err := api.console.AnalyzeSentiment(r.Context(), req.Text)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

// GetProviders returns available AI providers
func (api *AIConsoleAPI) GetProviders(w http.ResponseWriter, r *http.Request) {
	providers := api.console.GetAvailableProviders()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"providers": providers,
	})
}

// GetUsage returns usage statistics
func (api *AIConsoleAPI) GetUsage(w http.ResponseWriter, r *http.Request) {
	stats := api.console.GetUsageStats()
	json.NewEncoder(w).Encode(stats)
}

// GetAuditLogs returns audit logs
func (api *AIConsoleAPI) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	sinceStr := r.URL.Query().Get("since")

	var since time.Time
	if sinceStr != "" {
		since, _ = time.Parse(time.RFC3339, sinceStr)
	} else {
		since = time.Now().AddDate(0, 0, -7) // Default: last 7 days
	}

	logs := api.console.GetAuditLogs(userID, since, 100)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs": logs,
	})
}

// Desensitize handles data desensitization requests
func (api *AIConsoleAPI) Desensitize(w http.ResponseWriter, r *http.Request) {
	var req ai.DesensitizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Text == "" {
		http.Error(w, "text required", http.StatusBadRequest)
		return
	}

	dapi := ai.NewDesensitizationAPI()
	resp := dapi.Desensitize(&req)

	json.NewEncoder(w).Encode(resp)
}

// Restore handles text restoration requests
func (api *AIConsoleAPI) Restore(w http.ResponseWriter, r *http.Request) {
	var req ai.RestoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Text == "" {
		http.Error(w, "text required", http.StatusBadRequest)
		return
	}

	dapi := ai.NewDesensitizationAPI()
	resp := dapi.Restore(&req)

	json.NewEncoder(w).Encode(resp)
}

// GetRules returns all desensitization rules
func (api *AIConsoleAPI) GetRules(w http.ResponseWriter, r *http.Request) {
	dapi := ai.NewDesensitizationAPI()
	rules := dapi.GetRules()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"rules": rules,
	})
}

// AddRule adds a new desensitization rule
func (api *AIConsoleAPI) AddRule(w http.ResponseWriter, r *http.Request) {
	var rule ai.DesensitizationRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dapi := ai.NewDesensitizationAPI()
	dapi.AddRule(rule)

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// UpdateRule updates a desensitization rule
func (api *AIConsoleAPI) UpdateRule(w http.ResponseWriter, r *http.Request) {
	ruleID := r.PathValue("id")
	if ruleID == "" {
		http.Error(w, "rule id required", http.StatusBadRequest)
		return
	}

	var rule ai.DesensitizationRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rule.ID = ruleID
	// In production, update in database

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// DeleteRule deletes a desensitization rule
func (api *AIConsoleAPI) DeleteRule(w http.ResponseWriter, r *http.Request) {
	ruleID := r.PathValue("id")
	if ruleID == "" {
		http.Error(w, "rule id required", http.StatusBadRequest)
		return
	}

	// In production, delete from database

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// ProcessPII processes text with PII protection
func (api *AIConsoleAPI) ProcessPII(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Text == "" {
		http.Error(w, "text required", http.StatusBadRequest)
		return
	}

	processed, redactions := api.console.ProcessPII(req.Text)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"processed":  processed,
		"redactions": redactions,
	})
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// WriteError writes an error response
func WriteError(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   http.StatusText(code),
		Code:    code,
		Message: message,
	})
}

// Timestamp is a helper for consistent timestamp formatting
func Timestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}