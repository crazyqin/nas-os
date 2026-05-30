package nasdiscovery

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP 请求处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/discovery/devices", h.handleDevices)
	mux.HandleFunc("/api/v1/discovery/device", h.handleDevice)
	mux.HandleFunc("/api/v1/discovery/scan", h.handleScan)
	mux.HandleFunc("/api/v1/discovery/config", h.handleConfig)
	mux.HandleFunc("/api/v1/discovery/start", h.handleStart)
	mux.HandleFunc("/api/v1/discovery/stop", h.handleStop)
}

func (h *Handler) handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	online := r.URL.Query().Get("online") == "true"
	var devices []*NASDevice
	if online {
		devices = h.manager.GetOnlineDevices()
	} else {
		devices = h.manager.GetDevices()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

func (h *Handler) handleDevice(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		deviceID := r.URL.Query().Get("id")
		if deviceID == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}
		device, err := h.manager.GetDevice(deviceID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(device)

	case http.MethodDelete:
		deviceID := r.URL.Query().Get("id")
		if deviceID == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}
		if err := h.manager.RemoveDevice(deviceID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	case http.MethodPost:
		var req struct {
			IP   string `json:"ip"`
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		device, err := h.manager.AddManualDevice(r.Context(), req.IP, req.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(device)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	result, err := h.manager.ScanNow(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config := h.manager.GetConfig()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(config)

	case http.MethodPut:
		var config DiscoveryConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		h.manager.UpdateDeviceConfig(config)
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.manager.StartDiscovery(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (h *Handler) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.manager.StopDiscovery()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}
