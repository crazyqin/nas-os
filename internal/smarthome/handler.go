package smarthome

import (
	"encoding/json"
	"net/http"
)

// Handler handles HTTP requests for smart home
type Handler struct {
	manager *Manager
}

// NewHandler creates a new smart home handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers the HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/smarthome/devices", h.handleDevices)
	mux.HandleFunc("/api/v1/smarthome/device", h.handleDevice)
	mux.HandleFunc("/api/v1/smarthome/rooms", h.handleRooms)
	mux.HandleFunc("/api/v1/smarthome/scenes", h.handleScenes)
	mux.HandleFunc("/api/v1/smarthome/automations", h.handleAutomations)
	mux.HandleFunc("/api/v1/smarthome/stats", h.handleStats)
}

// handleDevices handles device listing and creation
func (h *Handler) handleDevices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		devices := h.manager.ListDevices()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"devices": devices,
			"total":   len(devices),
		})
	case http.MethodPost:
		var device Device
		if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.AddDevice(&device); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(device)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleDevice handles single device operations
func (h *Handler) handleDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("id")
	if deviceID == "" {
		http.Error(w, "id parameter is required", http.StatusBadRequest)
		return
	}
	
	switch r.Method {
	case http.MethodGet:
		device, err := h.manager.GetDevice(deviceID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(device)
	case http.MethodDelete:
		if err := h.manager.RemoveDevice(deviceID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPut:
		var properties map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&properties); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.controlDevice(deviceID, properties); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRooms handles room listing
func (h *Handler) handleRooms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	h.manager.mu.RLock()
	rooms := make([]*Room, 0, len(h.manager.rooms))
	for _, r := range h.manager.rooms {
		rooms = append(rooms, r)
	}
	h.manager.mu.RUnlock()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rooms": rooms,
		"total": len(rooms),
	})
}

// handleScenes handles scene listing and creation
func (h *Handler) handleScenes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.manager.mu.RLock()
		scenes := make([]*Scene, 0, len(h.manager.scenes))
		for _, s := range h.manager.scenes {
			scenes = append(scenes, s)
		}
		h.manager.mu.RUnlock()
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"scenes": scenes,
			"total":  len(scenes),
		})
	case http.MethodPost:
		var scene Scene
		if err := json.NewDecoder(r.Body).Decode(&scene); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.AddScene(&scene); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(scene)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAutomations handles automation listing and creation
func (h *Handler) handleAutomations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.manager.mu.RLock()
		automations := make([]*Automation, 0, len(h.manager.automations))
		for _, a := range h.manager.automations {
			automations = append(automations, a)
		}
		h.manager.mu.RUnlock()
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"automations": automations,
			"total":       len(automations),
		})
	case http.MethodPost:
		var auto Automation
		if err := json.NewDecoder(r.Body).Decode(&auto); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.AddAutomation(&auto); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(auto)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleStats handles statistics requests
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	stats := h.manager.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
