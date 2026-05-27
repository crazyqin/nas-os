package energymanager

import (
	"encoding/json"
	"net/http"
)

// Handler handles HTTP requests for energy management
type Handler struct {
	manager *Manager
}

// NewHandler creates a new energy manager handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers the HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/energy/stats", h.handleStats)
	mux.HandleFunc("/api/v1/energy/reading", h.handleReading)
	mux.HandleFunc("/api/v1/energy/profiles", h.handleProfiles)
	mux.HandleFunc("/api/v1/energy/budget", h.handleBudget)
	mux.HandleFunc("/api/v1/energy/carbon", h.handleCarbon)
	mux.HandleFunc("/api/v1/energy/state", h.handlePowerState)
}

// handleStats returns energy statistics
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := h.manager.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleReading records a power reading
func (h *Handler) handleReading(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var reading PowerReading
	if err := json.NewDecoder(r.Body).Decode(&reading); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	h.manager.RecordReading(&reading)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "recorded"})
}

// handleProfiles handles power profile operations
func (h *Handler) handleProfiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id != "" {
			profile, err := h.manager.GetProfile(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(profile)
			return
		}
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
	case http.MethodPost:
		var profile PowerProfile
		if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.CreateProfile(&profile); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "created"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleBudget returns estimated power budget
func (h *Handler) handleBudget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	budget := h.manager.EstimateBill(30)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(budget)
}

// handleCarbon returns carbon footprint metrics
func (h *Handler) handleCarbon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	metrics := h.manager.GetCarbonMetrics()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// handlePowerState handles power state changes
func (h *Handler) handlePowerState(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		stats := h.manager.GetStats()
		json.NewEncoder(w).Encode(map[string]string{"state": string(stats.PowerState)})
	case http.MethodPut:
		var req struct {
			State PowerState `json:"state"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.SetPowerState(req.State); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"state": string(req.State)})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
