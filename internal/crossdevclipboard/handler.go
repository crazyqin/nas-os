package crossdevclipboard

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP API handler.
type Handler struct {
	manager *ClipboardManager
}

// NewHandler 创建 handler.
func NewHandler(manager *ClipboardManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/clipboard/push", h.handlePush)
	mux.HandleFunc("/api/v1/clipboard/pull", h.handlePull)
	mux.HandleFunc("/api/v1/clipboard/history", h.handleHistory)
	mux.HandleFunc("/api/v1/clipboard/devices", h.handleDevices)
	mux.HandleFunc("/api/v1/clipboard/stats", h.handleStats)
	mux.HandleFunc("/api/v1/clipboard/register", h.handleRegisterDevice)
	mux.HandleFunc("/api/v1/clipboard/remove", h.handleRemoveDevice)
}

func (h *Handler) handlePush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		DeviceID string   `json:"deviceId"`
		Type     ClipType `json:"type"`
		Content  string   `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	item, err := h.manager.PushContent(req.DeviceID, req.Type, req.Content)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) handlePull(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("deviceId")
	if deviceID == "" {
		http.Error(w, "deviceId required", http.StatusBadRequest)
		return
	}
	item, err := h.manager.PullLatest(deviceID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("deviceId")
	if deviceID == "" {
		http.Error(w, "deviceId required", http.StatusBadRequest)
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		json.Unmarshal([]byte(l), &limit)
	}
	items, err := h.manager.GetHistory(deviceID, limit)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) handleDevices(w http.ResponseWriter, r *http.Request) {
	devices := h.manager.ListDevices()
	writeJSON(w, http.StatusOK, devices)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetStats()
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) handleRegisterDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var device Device
	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.manager.RegisterDevice(device); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "registered"})
}

func (h *Handler) handleRemoveDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("deviceId")
	if deviceID == "" {
		http.Error(w, "deviceId required", http.StatusBadRequest)
		return
	}
	if err := h.manager.RemoveDevice(deviceID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
