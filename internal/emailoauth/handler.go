// Package emailoauth 提供邮件 OAuth 通知功能。
package emailoauth

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP 处理器
type Handler struct {
	notifier *MailNotifier
}

// NewHandler 创建 HTTP 处理器
func NewHandler(notifier *MailNotifier) *Handler {
	return &Handler{notifier: notifier}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/emailoauth/config", h.Config)
	mux.HandleFunc("/api/emailoauth/test", h.Test)
	mux.HandleFunc("/api/emailoauth/status", h.Status)
}

// Config 处理 POST /api/emailoauth/config
func (h *Handler) Config(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse("method not allowed"))
		return
	}
	var req SetConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse("invalid request body"))
		return
	}
	cfg := &OAuthConfig{
		Provider:     req.Provider,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		RefreshToken: req.RefreshToken,
		FromEmail:    req.FromEmail,
		Method:       req.Method,
		SMTPHost:     req.SMTPHost,
		SMTPPort:     req.SMTPPort,
	}
	if cfg.Method == "" {
		cfg.Method = SendMethodOAuth2
	}
	if err := h.notifier.SetConfig(cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "configured",
		"provider":  req.Provider,
		"method":    cfg.Method,
		"from_email": req.FromEmail,
	})
}

// Test 处理 POST /api/emailoauth/test
func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse("method not allowed"))
		return
	}
	var req TestMailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse("invalid request body"))
		return
	}
	if !h.notifier.IsConfigured() {
		writeJSON(w, http.StatusBadRequest, errorResponse("email not configured"))
		return
	}
	result, err := h.notifier.SendTestMail(req.To, req.Subject, req.Body)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  err.Error(),
			"result": result,
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Status 处理 GET /api/emailoauth/status
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse("method not allowed"))
		return
	}
	cfg := h.notifier.GetConfig()
	if cfg == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"configured":  false,
			"token_valid": false,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"configured":  true,
		"provider":    cfg.Provider,
		"method":      cfg.Method,
		"from_email":  cfg.FromEmail,
		"token_valid": h.notifier.IsTokenValid(),
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func errorResponse(msg string) map[string]string {
	return map[string]string{"error": msg}
}