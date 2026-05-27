package containresmon

import (
	"encoding/json"
	"net/http"
)

// Handler handles HTTP requests for container resource monitor
type Handler struct {
	manager *Manager
}

// NewHandler creates a new container resource monitor handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers the HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/containresmon/containers", h.handleContainers)
	mux.HandleFunc("/api/v1/containresmon/container/register", h.handleRegister)
	mux.HandleFunc("/api/v1/containresmon/container/unregister", h.handleUnregister)
	mux.HandleFunc("/api/v1/containresmon/usage", h.handleUsage)
	mux.HandleFunc("/api/v1/containresmon/alerts", h.handleAlerts)
	mux.HandleFunc("/api/v1/containresmon/alert/resolve", h.handleResolveAlert)
	mux.HandleFunc("/api/v1/containresmon/stats", h.handleStats)
}

func (h *Handler) handleContainers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	containers := h.manager.ListContainers()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(containers)
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var container Container
	if err := json.NewDecoder(r.Body).Decode(&container); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	h.manager.RegisterContainer(&container)
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) handleUnregister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}
	h.manager.UnregisterContainer(id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}
	usage := h.manager.GetUsageHistory(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(usage)
}

func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	alerts := h.manager.ListAlerts()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(alerts)
}

func (h *Handler) handleResolveAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.manager.ResolveAlert(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := h.manager.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
