package smartnotify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Handler provides HTTP handlers for the smartnotify module
type Handler struct {
	router *Router
}

// NewHandler creates a new Handler
func NewHandler(router *Router) *Handler {
	return &Handler{router: router}
}

// RegisterRoutes registers HTTP routes on the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/notifications/send", h.handleSend)
	mux.HandleFunc("GET /api/notifications/history", h.handleHistory)
	mux.HandleFunc("POST /api/rules", h.handleCreateRule)
	mux.HandleFunc("GET /api/rules", h.handleListRules)
	mux.HandleFunc("GET /api/rules/{id}", h.handleGetRule)
	mux.HandleFunc("DELETE /api/rules/{id}", h.handleDeleteRule)
	mux.HandleFunc("PUT /api/preferences/{userId}", h.handleSetPreference)
	mux.HandleFunc("GET /api/preferences/{userId}", h.handleGetPreference)
	mux.HandleFunc("DELETE /api/preferences/{userId}", h.handleDeletePreference)
}

// APIResponse wraps API responses
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// SendRequest is the request body for sending notifications
type SendRequest struct {
	Title    string            `json:"title"`
	Content  string            `json:"content"`
	Priority string            `json:"priority"`
	Source   string            `json:"source"`
	Labels   map[string]string `json:"labels"`
}

// handleSend handles POST /api/notifications/send
func (h *Handler) handleSend(w http.ResponseWriter, r *http.Request) {
	var req SendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid request body: %v", err),
		})
		return
	}

	if req.Title == "" || req.Content == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "title and content are required",
		})
		return
	}

	notif := &Notification{
		ID:        fmt.Sprintf("notif-%d", time.Now().UnixNano()),
		Title:     req.Title,
		Content:   req.Content,
		Priority:  ParsePriority(req.Priority),
		Source:    req.Source,
		Labels:    req.Labels,
		Status:    StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.router.Send(notif); err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    notif,
	})
}

// handleHistory handles GET /api/notifications/history
func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if _, err := fmt.Sscanf(l, "%d", &limit); err != nil {
			limit = 50
		}
	}

	history := h.router.History(limit)
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    history,
	})
}

// handleCreateRule handles POST /api/rules
func (h *Handler) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	var rule RoutingRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid request body: %v", err),
		})
		return
	}

	if rule.ID == "" {
		rule.ID = fmt.Sprintf("rule-%d", time.Now().UnixNano())
	}

	h.router.AddRule(&rule)
	writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    rule,
	})
}

// handleListRules handles GET /api/rules
func (h *Handler) handleListRules(w http.ResponseWriter, r *http.Request) {
	rules := h.router.ListRules()
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    rules,
	})
}

// handleGetRule handles GET /api/rules/{id}
func (h *Handler) handleGetRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "rule id is required",
		})
		return
	}

	rule, ok := h.router.GetRule(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("rule %s not found", id),
		})
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    rule,
	})
}

// handleDeleteRule handles DELETE /api/rules/{id}
func (h *Handler) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "rule id is required",
		})
		return
	}

	h.router.RemoveRule(id)
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
	})
}

// handleSetPreference handles PUT /api/preferences/{userId}
func (h *Handler) handleSetPreference(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "userId is required",
		})
		return
	}

	var pref UserPreference
	if err := json.NewDecoder(r.Body).Decode(&pref); err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid request body: %v", err),
		})
		return
	}

	pref.UserID = userID
	h.router.SetUserPreference(&pref)
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    pref,
	})
}

// handleGetPreference handles GET /api/preferences/{userId}
func (h *Handler) handleGetPreference(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "userId is required",
		})
		return
	}

	pref, ok := h.router.GetUserPreference(userID)
	if !ok {
		writeJSON(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("preferences for user %s not found", userID),
		})
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    pref,
	})
}

// handleDeletePreference handles DELETE /api/preferences/{userId}
func (h *Handler) handleDeletePreference(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "userId is required",
		})
		return
	}

	h.router.DeleteUserPreference(userID)
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
	})
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// validateChannels validates that channels are supported
func validateChannels(channels []Channel) error {
	supported := map[Channel]bool{
		ChannelEmail:    true,
		ChannelWebhook:  true,
		ChannelTelegram: true,
		ChannelDiscord:  true,
		ChannelWeChat:   true,
		ChannelDingTalk: true,
		ChannelSMS:      true,
	}
	var invalid []string
	for _, ch := range channels {
		if !supported[ch] {
			invalid = append(invalid, string(ch))
		}
	}
	if len(invalid) > 0 {
		return fmt.Errorf("unsupported channels: %s", strings.Join(invalid, ", "))
	}
	return nil
}
