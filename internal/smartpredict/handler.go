package smartpredict

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/smartpredict/disks", h.handleGetDisks)
	mux.HandleFunc("/api/smartpredict/disk", h.handleGetDisk)
	mux.HandleFunc("/api/smartpredict/summary", h.handleGetSummary)
	mux.HandleFunc("/api/smartpredict/analyze", h.handleAnalyze)
}

func (h *Handler) handleGetDisks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	disks := h.manager.GetAllDisks()
	writeJSON(w, disks)
}

func (h *Handler) handleGetDisk(w http.ResponseWriter, r *http.Request) {
	device := r.URL.Query().Get("device")
	if device == "" {
		http.Error(w, "device parameter required", http.StatusBadRequest)
		return
	}
	disk, ok := h.manager.GetDisk(device)
	if !ok {
		http.Error(w, "disk not found", http.StatusNotFound)
		return
	}
	writeJSON(w, disk)
}

func (h *Handler) handleGetSummary(w http.ResponseWriter, r *http.Request) {
	summary := h.manager.GetRiskSummary()
	writeJSON(w, summary)
}

func (h *Handler) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var health DiskHealth
	if err := json.NewDecoder(r.Body).Decode(&health); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	h.manager.UpdateDisk(&health)
	writeJSON(w, health)
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
