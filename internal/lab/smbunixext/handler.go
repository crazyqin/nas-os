// Package smbunixext 提供 SMB3 Unix Extensions 支持。
package smbunixext

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP 处理器.
type Handler struct {
	manager *ExtensionManager
}

// NewHandler 创建 HTTP 处理器.
func NewHandler(manager *ExtensionManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/smb/unix-extensions", h.UnixExtensions)
}

// UnixExtensions 处理 GET/POST /api/smb/unix-extensions.
func (h *Handler) UnixExtensions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getExtensions(w, r)
	case http.MethodPost:
		h.setExtension(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse("method not allowed"))
	}
}

// getExtensions 获取所有扩展配置或指定共享的配置.
func (h *Handler) getExtensions(w http.ResponseWriter, r *http.Request) {
	shareName := r.URL.Query().Get("share")
	if shareName != "" {
		status, err := h.manager.GetExtensionStatus(shareName)
		if err != nil {
			writeJSON(w, http.StatusNotFound, errorResponse(err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, status)
		return
	}

	configs := h.manager.ListExtensions()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"configs": configs,
		"count":   len(configs),
	})
}

// setExtension 设置共享的 Unix Extensions 配置.
func (h *Handler) setExtension(w http.ResponseWriter, r *http.Request) {
	var req SetExtensionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse("invalid request body"))
		return
	}
	if req.ShareName == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse("share_name is required"))
		return
	}

	cfg, err := h.manager.SetExtension(&req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse(err.Error()))
		return
	}

	// 保存到数据库
	if err := h.manager.SaveToDB(req.ShareName); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse(err.Error()))
		return
	}

	writeJSON(w, http.StatusCreated, cfg)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func errorResponse(msg string) map[string]string {
	return map[string]string{"error": msg}
}
