package systemcopilot

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler handles HTTP requests for SystemCopilot
type Handler struct {
	manager *Manager
}

// NewHandler creates a new SystemCopilot HTTP handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

// RegisterRoutes registers all SystemCopilot HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/systemcopilot/process", h.handleProcess)
	mux.HandleFunc("/api/systemcopilot/confirm", h.handleConfirm)
	mux.HandleFunc("/api/systemcopilot/suggestions", h.handleSuggestions)
	mux.HandleFunc("/api/systemcopilot/history", h.handleHistory)
	mux.HandleFunc("/api/systemcopilot/stats", h.handleStats)
	mux.HandleFunc("/api/systemcopilot/session", h.handleSession)
}

// handleProcess handles natural language command processing
func (h *Handler) handleProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req ProcessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Input == "" {
		writeError(w, http.StatusBadRequest, "Input is required")
		return
	}

	cmd, result, err := h.manager.ProcessCommand(req.Input, req.SessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := ProcessResponse{
		Command: cmd,
		Result:  result,
		Message: "Command processed successfully",
	}

	if cmd.NeedsConfirm && result == nil {
		resp.NeedConfirm = true
		resp.Message = "此操作需要确认，请确认后执行"
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleConfirm handles command confirmation
func (h *Handler) handleConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		CommandID string `json:"command_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.CommandID == "" {
		writeError(w, http.StatusBadRequest, "Command ID is required")
		return
	}

	result, err := h.manager.ConfirmCommand(req.CommandID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"result":  result,
		"message": "Command confirmed and executed",
	})
}

// handleSuggestions returns AI-generated suggestions
func (h *Handler) handleSuggestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	suggestions := h.manager.GetSuggestions()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"suggestions": suggestions,
		"total":       len(suggestions),
	})
}

// handleHistory returns command history
func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse pagination parameters
	page := 1
	pageSize := 20

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= 100 {
			pageSize = v
		}
	}

	commands, results, total := h.manager.GetHistory(page, pageSize)

	writeJSON(w, http.StatusOK, HistoryResponse{
		Commands: commands,
		Results:  results,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// handleStats returns copilot usage statistics
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	stats := h.manager.GetStats()
	writeJSON(w, http.StatusOK, stats)
}

// handleSession returns session details
func (h *Handler) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	sessionID := r.URL.Query().Get("id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "Session ID is required")
		return
	}

	session, err := h.manager.GetSession(sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, session)
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}
