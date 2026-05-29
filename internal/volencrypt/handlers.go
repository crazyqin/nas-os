package volencrypt

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Handlers HTTP 处理器
type Handlers struct {
	mgr *Manager
}

// NewHandlers 创建 HTTP 处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/volumes", h.handleVolumes)
	mux.HandleFunc(prefix+"/volumes/encrypt", h.handleEncrypt)
	mux.HandleFunc(prefix+"/volumes/decrypt", h.handleDecrypt)
	mux.HandleFunc(prefix+"/volumes/lock", h.handleLock)
	mux.HandleFunc(prefix+"/volumes/unlock", h.handleUnlock)
	mux.HandleFunc(prefix+"/keys/rotate", h.handleRotateKey)
	mux.HandleFunc(prefix+"/audit", h.handleAudit)
	mux.HandleFunc(prefix+"/stats", h.handleStats)
}

type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

// handleVolumes 管理卷
func (h *Handlers) handleVolumes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		volumes := h.mgr.ListVolumes()
		writeJSON(w, http.StatusOK, response{
			Code:    200,
			Message: "ok",
			Data:    volumes,
		})

	case http.MethodPost:
		var req struct {
			Name string `json:"name"`
			Path string `json:"path"`
			Size int64  `json:"size"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "无效的请求"})
			return
		}

		if req.Name == "" || req.Path == "" {
			writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "名称和路径不能为空"})
			return
		}

		volume, err := h.mgr.CreateVolume(req.Name, req.Path, req.Size)
		if err != nil {
			writeJSON(w, http.StatusConflict, response{Code: 409, Message: err.Error()})
			return
		}

		writeJSON(w, http.StatusCreated, response{
			Code:    201,
			Message: "卷创建成功",
			Data:    volume,
		})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
	}
}

// handleEncrypt 加密卷
func (h *Handlers) handleEncrypt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	var req struct {
		VolumeID string `json:"volume_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "无效的请求"})
		return
	}

	if err := h.mgr.EncryptVolume(r.Context(), req.VolumeID); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "加密已开始",
	})
}

// handleDecrypt 解密卷
func (h *Handlers) handleDecrypt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	var req struct {
		VolumeID string `json:"volume_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "无效的请求"})
		return
	}

	if err := h.mgr.DecryptVolume(r.Context(), req.VolumeID); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "解密已开始",
	})
}

// handleLock 锁定卷
func (h *Handlers) handleLock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	var req struct {
		VolumeID string `json:"volume_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "无效的请求"})
		return
	}

	if err := h.mgr.LockVolume(req.VolumeID); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "卷已锁定",
	})
}

// handleUnlock 解锁卷
func (h *Handlers) handleUnlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	var req struct {
		VolumeID   string `json:"volume_id"`
		MountPoint string `json:"mount_point"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "无效的请求"})
		return
	}

	if err := h.mgr.UnlockVolume(req.VolumeID, req.MountPoint); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "卷已解锁",
	})
}

// handleRotateKey 轮换密钥
func (h *Handlers) handleRotateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	var req struct {
		VolumeID string `json:"volume_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "无效的请求"})
		return
	}

	if err := h.mgr.RotateKey(req.VolumeID); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "密钥已轮换",
	})
}

// handleAudit 获取审计日志
func (h *Handlers) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	limit := 100
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)

	auditLog := h.mgr.GetAuditLog(limit)
	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    auditLog,
	})
}

// handleStats 获取统计信息
func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	stats := h.mgr.GetStats()
	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    stats,
	})
}

// GetVolumeIDFromPath 从路径获取卷 ID
func (h *Handlers) GetVolumeIDFromPath(path string) (string, error) {
	volumes := h.mgr.ListVolumes()
	for _, v := range volumes {
		if strings.HasPrefix(path, v.Path) {
			return v.ID, nil
		}
	}
	return "", fmt.Errorf("路径 %s 不属于任何加密卷", path)
}
