// Package remoteaccess 提供 HTTP API 处理器
package remoteaccess

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Handler 远程访问 API 处理器
type Handler struct {
	manager *RemoteAccessManager
}

// NewHandler 创建处理器
func NewHandler(manager *RemoteAccessManager) *Handler {
	return &Handler{manager: manager}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// writeJSON 写入JSON响应
func writeJSON(w http.ResponseWriter, status int, resp response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// writeError 写入错误响应
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, response{
		Code:    1,
		Message: message,
	})
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// 活跃会话列表
	mux.HandleFunc("/api/remote/sessions", h.handleSessions)

	// 建立连接
	mux.HandleFunc("/api/remote/connect", h.handleConnect)

	// 断开会话（需要解析路径中的ID）
	mux.HandleFunc("/api/remote/sessions/", h.handleSessionByID)

	// 远程访问状态
	mux.HandleFunc("/api/remote/status", h.handleStatus)

	// 更新配置
	mux.HandleFunc("/api/remote/config", h.handleConfig)

	// DDNS状态和更新
	mux.HandleFunc("/api/remote/ddns", h.handleDDNS)

	// 证书列表
	mux.HandleFunc("/api/remote/certificates", h.handleCertificates)

	// 续期证书
	mux.HandleFunc("/api/remote/certificates/renew", h.handleCertificateRenew)

	// 访问日志
	mux.HandleFunc("/api/remote/access-log", h.handleAccessLog)
}

// handleSessions 处理会话相关请求
func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listSessions(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// listSessions 列出活跃会话
func (h *Handler) listSessions(w http.ResponseWriter, r *http.Request) {
	sessions := h.manager.ListActiveSessions()
	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    sessions,
	})
}

// handleConnect 处理连接请求
func (h *Handler) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		UserID     string        `json:"user_id"`
		DeviceName string        `json:"device_name"`
		Protocol   AccessProtocol `json:"protocol"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if req.UserID == "" || req.DeviceName == "" || req.Protocol == "" {
		writeError(w, http.StatusBadRequest, "user_id, device_name, and protocol are required")
		return
	}

	clientIP := r.RemoteAddr
	// 尝试从X-Forwarded-For获取真实IP
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		clientIP = strings.Split(forwarded, ",")[0]
	} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		clientIP = realIP
	}

	session, err := h.manager.CreateSession(req.UserID, req.DeviceName, clientIP, req.Protocol)
	if err != nil {
		if strings.Contains(err.Error(), "rate limit") {
			writeError(w, http.StatusTooManyRequests, err.Error())
		} else if strings.Contains(err.Error(), "denied") {
			writeError(w, http.StatusForbidden, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, response{
		Code:    0,
		Message: "session created",
		Data:    session,
	})
}

// handleSessionByID 处理单个会话操作
func (h *Handler) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	// 提取会话ID：/api/remote/sessions/{id}
	path := r.URL.Path
	prefix := "/api/remote/sessions/"
	if !strings.HasPrefix(path, prefix) {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	sessionID := strings.TrimPrefix(path, prefix)
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getSession(w, r, sessionID)
	case http.MethodDelete:
		h.closeSession(w, r, sessionID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// getSession 获取会话详情
func (h *Handler) getSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	session, err := h.manager.GetSession(sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    session,
	})
}

// closeSession 关闭会话
func (h *Handler) closeSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	if err := h.manager.CloseSession(sessionID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "session closed",
	})
}

// handleStatus 处理状态请求
func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	status := h.manager.GetRemoteAccessStatus()
	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    status,
	})
}

// handleConfig 处理配置请求
func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getConfig(w, r)
	case http.MethodPut:
		h.updateConfig(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// getConfig 获取配置
func (h *Handler) getConfig(w http.ResponseWriter, r *http.Request) {
	config := h.manager.GetConfig()
	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    config,
	})
}

// updateConfig 更新配置
func (h *Handler) updateConfig(w http.ResponseWriter, r *http.Request) {
	var config RemoteAccessConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	h.manager.UpdateConfig(&config)
	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "config updated",
	})
}

// handleDDNS 处理DDNS请求
func (h *Handler) handleDDNS(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getDDNSStatus(w, r)
	case http.MethodPut:
		h.updateDDNS(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// getDDNSStatus 获取DDNS状态
func (h *Handler) getDDNSStatus(w http.ResponseWriter, r *http.Request) {
	status := h.manager.GetDDNSStatus()
	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    status,
	})
}

// updateDDNS 更新DDNS配置
func (h *Handler) updateDDNS(w http.ResponseWriter, r *http.Request) {
	var config DDNSConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if err := h.manager.UpdateDDNS(&config); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "ddns config updated",
	})
}

// handleCertificates 处理证书请求
func (h *Handler) handleCertificates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	certs := h.manager.ListCertificates()
	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    certs,
	})
}

// handleCertificateRenew 处理证书续期请求
func (h *Handler) handleCertificateRenew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := h.manager.RenewCertificate(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "certificate renewed",
	})
}

// handleAccessLog 处理访问日志请求
func (h *Handler) handleAccessLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50 // 默认50条
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	logs := h.manager.GetAccessLog(limit)
	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    logs,
	})
}

// StartServer 启动HTTP服务器
func (h *Handler) StartServer(addr string) error {
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	fmt.Printf("Remote access API server starting on %s\n", addr)
	return http.ListenAndServe(addr, mux)
}

// StartTLSServer 启动HTTPS服务器
func (h *Handler) StartTLSServer(addr, certFile, keyFile string) error {
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	fmt.Printf("Remote access API server starting on %s (TLS)\n", addr)
	return http.ListenAndServeTLS(addr, certFile, keyFile, mux)
}
