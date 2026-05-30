package disklifecycle

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler provides HTTP API handlers for disk lifecycle management.
type Handler struct {
	manager *Manager
}

// NewHandler creates a new disk lifecycle handler.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers disk lifecycle API routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/disklifecycle/disks", h.handleDisks)
	mux.HandleFunc("/api/disklifecycle/disks/", h.handleDiskByID)
	mux.HandleFunc("/api/disklifecycle/alerts", h.handleAlerts)
	mux.HandleFunc("/api/disklifecycle/report", h.handleReport)
}

func (h *Handler) handleDisks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	disks := h.manager.ListDisks()
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": disks})
}

func (h *Handler) handleDiskByID(w http.ResponseWriter, r *http.Request) {
	// Extract disk ID from path: /api/disklifecycle/disks/{id} or /api/disklifecycle/disks/{id}/predict or /api/disklifecycle/disks/{id}/retire
	path := strings.TrimPrefix(r.URL.Path, "/api/disklifecycle/disks/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]

	if id == "" {
		http.Error(w, "missing disk id", http.StatusBadRequest)
		return
	}

	// Check for sub-actions
	if len(parts) > 1 {
		switch parts[1] {
		case "predict":
			h.handlePredict(w, r, id)
			return
		case "retire":
			h.handleRetire(w, r, id)
			return
		default:
			http.Error(w, "unknown action", http.StatusNotFound)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		disk, err := h.manager.GetDisk(id)
		if err != nil {
			writeJSON(w, map[string]interface{}{"code": 404, "message": err.Error()})
			return
		}
		writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": disk})
	case http.MethodDelete:
		if err := h.manager.UnregisterDisk(id); err != nil {
			writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
			return
		}
		writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handlePredict(w http.ResponseWriter, r *http.Request, diskID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	result, err := h.manager.GetPrediction(diskID)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 404, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": result})
}

func (h *Handler) handleRetire(w http.ResponseWriter, r *http.Request, diskID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "invalid request body"})
		return
	}

	if err := h.manager.RetireDisk(diskID, req.Reason); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	dismissed := r.URL.Query().Get("dismissed") == "true"
	alerts := h.manager.GetAlerts(dismissed)
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": alerts})
}

func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	summary := h.manager.GetFleetSummary()
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": summary})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
