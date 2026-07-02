// Package vaultencryption - 保险库加密 HTTP 处理器
package vaultencryption

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// VaultEncryptionHandler HTTP 处理器.
type VaultEncryptionHandler struct {
	manager *VaultEncryptionManager
}

// NewVaultEncryptionHandler 创建处理器.
func NewVaultEncryptionHandler(manager *VaultEncryptionManager) *VaultEncryptionHandler {
	return &VaultEncryptionHandler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *VaultEncryptionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/vault/unlock", h.handleUnlock)
	mux.HandleFunc("/api/v1/vault/lock", h.handleLock)
	mux.HandleFunc("/api/v1/vault/volumes", h.handleListVolumes)
	mux.HandleFunc("/api/v1/vault/volume/get", h.handleGetVolume)
	mux.HandleFunc("/api/v1/vault/volume/register", h.handleRegisterVolume)
	mux.HandleFunc("/api/v1/vault/volume/unregister", h.handleUnregisterVolume)
	mux.HandleFunc("/api/v1/vault/keys", h.handleListKeys)
	mux.HandleFunc("/api/v1/vault/keys/create", h.handleCreateKey)
	mux.HandleFunc("/api/v1/vault/keys/delete", h.handleDeleteKey)
	mux.HandleFunc("/api/v1/vault/keys/change-password", h.handleChangePassword)
	mux.HandleFunc("/api/v1/vault/stats", h.handleStats)
	mux.HandleFunc("/api/v1/vault/audit", h.handleAuditLogs)
	mux.HandleFunc("/api/v1/vault/config", h.handleConfig)
}

// handleUnlock 处理解锁请求.
func (h *VaultEncryptionHandler) handleUnlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UnlockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, APIResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if req.VolumeID == "" {
		writeJSON(w, APIResponse{
			Code:    400,
			Message: "缺少volume_id参数",
		})
		return
	}

	if req.Password == "" {
		writeJSON(w, APIResponse{
			Code:    400,
			Message: "缺少password参数",
		})
		return
	}

	result, err := h.manager.UnlockVolume(req)
	if err != nil {
		writeJSON(w, APIResponse{
			Code:    500,
			Message: result.Message,
			Data:    result,
		})
		return
	}

	writeJSON(w, APIResponse{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// handleLock 处理锁定请求.
func (h *VaultEncryptionHandler) handleLock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, APIResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if req.VolumeID == "" {
		writeJSON(w, APIResponse{
			Code:    400,
			Message: "缺少volume_id参数",
		})
		return
	}

	err := h.manager.LockVolume(req)
	if err != nil {
		writeJSON(w, APIResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, APIResponse{
		Code:    0,
		Message: "success",
	})
}

// handleListVolumes 处理列出卷请求.
func (h *VaultEncryptionHandler) handleListVolumes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	volumes := h.manager.ListVolumes()

	writeJSON(w, VolumeListResponse{
		Code:    0,
		Message: "success",
		Data:    volumes,
	})
}

// handleGetVolume 处理获取卷请求.
func (h *VaultEncryptionHandler) handleGetVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	volumeID := r.URL.Query().Get("volume_id")
	if volumeID == "" {
		writeJSON(w, APIResponse{
			Code:    400,
			Message: "缺少volume_id参数",
		})
		return
	}

	volume, err := h.manager.GetVolume(volumeID)
	if err != nil {
		writeJSON(w, APIResponse{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, VolumeResponse{
		Code:    0,
		Message: "success",
		Data:    *volume,
	})
}

// handleRegisterVolume 处理注册卷请求.
func (h *VaultEncryptionHandler) handleRegisterVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var vol EncryptedVolume
	if err := json.NewDecoder(r.Body).Decode(&vol); err != nil {
		writeJSON(w, APIResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if err := h.manager.RegisterVolume(&vol); err != nil {
		writeJSON(w, APIResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, APIResponse{
		Code:    0,
		Message: "success",
		Data:    vol,
	})
}

// handleUnregisterVolume 处理注销卷请求.
func (h *VaultEncryptionHandler) handleUnregisterVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		VolumeID string `json:"volume_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, APIResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if err := h.manager.UnregisterVolume(req.VolumeID); err != nil {
		writeJSON(w, APIResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, APIResponse{
		Code:    0,
		Message: "success",
	})
}

// handleListKeys 处理列出密钥请求.
func (h *VaultEncryptionHandler) handleListKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	keys := h.manager.ListKeys()

	writeJSON(w, KeyListResponse{
		Code:    0,
		Message: "success",
		Data:    keys,
	})
}

// handleCreateKey 处理创建密钥请求.
func (h *VaultEncryptionHandler) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, APIResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	key, err := h.manager.CreateKey(req)
	if err != nil {
		writeJSON(w, APIResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, KeyResponse{
		Code:    0,
		Message: "success",
		Data:    *key,
	})
}

// handleDeleteKey 处理删除密钥请求.
func (h *VaultEncryptionHandler) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		KeyID string `json:"key_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, APIResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if err := h.manager.DeleteKey(req.KeyID); err != nil {
		writeJSON(w, APIResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, APIResponse{
		Code:    0,
		Message: "success",
	})
}

// handleChangePassword 处理修改密码请求.
func (h *VaultEncryptionHandler) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, APIResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if err := h.manager.ChangePassword(req); err != nil {
		writeJSON(w, APIResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, APIResponse{
		Code:    0,
		Message: "success",
	})
}

// handleStats 处理统计请求.
func (h *VaultEncryptionHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.manager.GetStats()

	writeJSON(w, StatsResponse{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// handleAuditLogs 处理审计日志请求.
func (h *VaultEncryptionHandler) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取limit参数
	limitStr := r.URL.Query().Get("limit")
	limit := 100 // 默认100条
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	logs := h.manager.GetAuditLogs(limit)

	writeJSON(w, AuditLogResponse{
		Code:    0,
		Message: "success",
		Data:    logs,
	})
}

// handleConfig 处理配置请求.
func (h *VaultEncryptionHandler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config := h.manager.GetConfig()
		writeJSON(w, APIResponse{
			Code:    0,
			Message: "success",
			Data:    config,
		})
	case http.MethodPost:
		var config VaultConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			writeJSON(w, APIResponse{
				Code:    400,
				Message: "无效的请求体",
			})
			return
		}
		h.manager.SetConfig(config)
		writeJSON(w, APIResponse{
			Code:    0,
			Message: "success",
		})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// writeJSON 写入JSON响应.
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
