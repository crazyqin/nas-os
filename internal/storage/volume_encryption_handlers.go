package storage

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// VolumeEncryptionHandlers 全卷加密HTTP处理器.
type VolumeEncryptionHandlers struct {
	manager *VolumeEncryptionManager
}

// NewVolumeEncryptionHandlers 创建HTTP处理器.
func NewVolumeEncryptionHandlers(manager *VolumeEncryptionManager) *VolumeEncryptionHandlers {
	return &VolumeEncryptionHandlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *VolumeEncryptionHandlers) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/volumes", h.handleVolumes)
	mux.HandleFunc(prefix+"/volumes/", h.handleVolumeByID)
	mux.HandleFunc(prefix+"/volumes/stats", h.handleStats)
}

// handleVolumes 处理 /api/v1/encryption/volumes.
func (h *VolumeEncryptionHandlers) handleVolumes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listVolumes(w, r)
	case http.MethodPost:
		h.createVolume(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleVolumeByID 处理 /api/v1/encryption/volumes/{id}.
func (h *VolumeEncryptionHandlers) handleVolumeByID(w http.ResponseWriter, r *http.Request) {
	// 提取ID
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/encryption/volumes/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Missing volume ID", http.StatusBadRequest)
		return
	}
	volumeID := parts[0]

	// 检查是否有子操作
	if len(parts) > 1 {
		action := parts[1]
		switch action {
		case "unlock":
			h.unlockVolume(w, r, volumeID)
		case "lock":
			h.lockVolume(w, r, volumeID)
		case "rekey":
			h.rekeyVolume(w, r, volumeID)
		default:
			http.Error(w, "Unknown action", http.StatusBadRequest)
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getVolume(w, r, volumeID)
	case http.MethodDelete:
		h.deleteVolume(w, r, volumeID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *VolumeEncryptionHandlers) listVolumes(w http.ResponseWriter, r *http.Request) {
	volumes := h.manager.ListVolumes()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"volumes": volumes,
		"total":   len(volumes),
	})
}

func (h *VolumeEncryptionHandlers) createVolume(w http.ResponseWriter, r *http.Request) {
	var req CreateEncryptedVolumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	vol, err := h.manager.CreateVolume(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, vol)
}

func (h *VolumeEncryptionHandlers) getVolume(w http.ResponseWriter, r *http.Request, volumeID string) {
	vol, err := h.manager.GetVolume(volumeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, vol)
}

func (h *VolumeEncryptionHandlers) unlockVolume(w http.ResponseWriter, r *http.Request, volumeID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.UnlockVolume(volumeID, req.Password); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "unlocked"})
}

func (h *VolumeEncryptionHandlers) lockVolume(w http.ResponseWriter, r *http.Request, volumeID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.manager.LockVolume(volumeID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "locked"})
}

func (h *VolumeEncryptionHandlers) deleteVolume(w http.ResponseWriter, r *http.Request, volumeID string) {
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.DeleteVolume(volumeID, req.Password); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *VolumeEncryptionHandlers) rekeyVolume(w http.ResponseWriter, r *http.Request, volumeID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.RekeyVolume(volumeID, req.OldPassword, req.NewPassword); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "rekeyed"})
}

func (h *VolumeEncryptionHandlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := h.manager.GetStats()
	writeJSON(w, http.StatusOK, stats)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Printf("JSON encode error: %v\n", err)
	}
}
