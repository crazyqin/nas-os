package smartnas

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler provides HTTP endpoints for SmartNAS health scoring.
type Handler struct {
	manager *Manager
}

// NewHandler creates a new Handler.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers SmartNAS HTTP routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/smartnas/score", h.handleScore)
	mux.HandleFunc("/api/v1/smartnas/subsystems", h.handleSubsystems)
	mux.HandleFunc("/api/v1/smartnas/recommendations", h.handleRecommendations)
	mux.HandleFunc("/api/v1/smartnas/history", h.handleHistory)
	mux.HandleFunc("/api/v1/smartnas/refresh", h.handleRefresh)
	mux.HandleFunc("/api/v1/smartnas/dismiss", h.handleDismiss)
}

// GET /api/v1/smartnas/score - Get overall NAS health score.
func (h *Handler) handleScore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	score := h.manager.GetScore()
	writeJSON(w, http.StatusOK, score)
}

// GET /api/v1/smartnas/subsystems - List all subsystem health data.
func (h *Handler) handleSubsystems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	score := h.manager.GetScore()
	writeJSON(w, http.StatusOK, score.Subsystems)
}

// GET /api/v1/smartnas/recommendations - Get active recommendations.
func (h *Handler) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	includeDismissed := r.URL.Query().Get("include_dismissed") == "true"
	recs := h.manager.GetRecommendations(includeDismissed)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"recommendations": recs,
		"count":           len(recs),
	})
}

// GET /api/v1/smartnas/history - Get historical health scores.
func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	history := h.manager.GetHistory(limit)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"history": history,
		"count":   len(history),
	})
}

// POST /api/v1/smartnas/refresh - Trigger a full health refresh.
func (h *Handler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	err := h.manager.RefreshAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	score := h.manager.GetScore()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "refreshed",
		"overall": score.Overall,
		"level":   score.Level,
	})
}

// POST /api/v1/smartnas/dismiss - Dismiss a recommendation.
func (h *Handler) handleDismiss(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.ID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	ok := h.manager.DismissRecommendation(req.ID)
	if !ok {
		http.Error(w, "recommendation not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "dismissed"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
