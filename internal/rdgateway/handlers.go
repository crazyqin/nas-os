// Package rdgateway 提供 HTTP API 处理器
package rdgateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Handlers 远程桌面网关 API 处理器.
type Handlers struct {
	sessions     *SessionManager
	tunnel       *WSTunnel
	clipboard    *ClipboardSync
	fileTransfer *FileTransfer
}

// NewHandlers 创建处理器.
func NewHandlers(sm *SessionManager, tunnel *WSTunnel, cs *ClipboardSync, ft *FileTransfer) *Handlers {
	return &Handlers{
		sessions:     sm,
		tunnel:       tunnel,
		clipboard:    cs,
		fileTransfer: ft,
	}
}

// RegisterRoutes 注册路由到 /api/v1/rdgateway.
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/rdgateway/sessions", h.handleSessions)
	mux.HandleFunc("/api/v1/rdgateway/sessions/", h.handleSessionByID)
	mux.HandleFunc("/api/v1/rdgateway/tunnel/", h.handleTunnel)
	mux.HandleFunc("/api/v1/rdgateway/clipboard/", h.handleClipboard)
	mux.HandleFunc("/api/v1/rdgateway/transfers", h.handleTransfers)
	mux.HandleFunc("/api/v1/rdgateway/transfers/", h.handleTransferByID)
	mux.HandleFunc("/api/v1/rdgateway/audit", h.handleAudit)
	mux.HandleFunc("/api/v1/rdgateway/test", h.handleTestConnection)
}

// response 标准 API 响应.
type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// handleSessions 处理 /api/v1/rdgateway/sessions.
func (h *Handlers) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		userID := r.URL.Query().Get("user_id")
		sessions := h.sessions.ListSessions(userID)
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"total":    len(sessions),
				"sessions": sessions,
			},
		})
	case http.MethodPost:
		var req CreateSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid request: " + err.Error()})
			return
		}
		session, err := h.sessions.CreateSession(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{Code: 1, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, apiResponse{Code: 0, Message: "created", Data: session})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSessionByID 处理 /api/v1/rdgateway/sessions/{id} 及子路径.
func (h *Handlers) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	// 提取 session ID: /api/v1/rdgateway/sessions/{id}[/action]
	path := r.URL.Path[len("/api/v1/rdgateway/sessions/"):]
	if path == "" {
		http.NotFound(w, r)
		return
	}

	// 解析 id 和 action
	id := path
	action := ""
	for i, c := range path {
		if c == '/' {
			id = path[:i]
			if i+1 < len(path) {
				action = path[i+1:]
			}
			break
		}
	}

	switch action {
	case "disconnect":
		h.handleDisconnect(w, r, id)
	case "reconnect":
		h.handleReconnect(w, r, id)
	case "":
		h.handleSession(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handlers) handleSession(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		session, err := h.sessions.GetSession(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: session})
	case http.MethodDelete:
		if err := h.sessions.DeleteSession(id); err != nil {
			writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "deleted"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handlers) handleDisconnect(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.sessions.DisconnectSession(id); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Code: 1, Message: err.Error()})
		return
	}
	_ = h.tunnel.CloseTunnel(id)
	writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "disconnected"})
}

func (h *Handlers) handleReconnect(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.sessions.ReconnectSession(id); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Code: 1, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "reconnecting"})
}

// handleTunnel 处理 /api/v1/rdgateway/tunnel/{sessionID}.
func (h *Handlers) handleTunnel(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Path[len("/api/v1/rdgateway/tunnel/"):]
	if sessionID == "" {
		http.NotFound(w, r)
		return
	}

	// 校验 session 存在
	if _, err := h.sessions.GetSession(sessionID); err != nil {
		writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
		return
	}

	// WebSocket 升级由 HandleTunnel 内部处理
	if err := h.tunnel.HandleTunnel(w, r, sessionID); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Code: 1, Message: "websocket upgrade failed: " + err.Error()})
		return
	}
}

// handleClipboard 处理 /api/v1/rdgateway/clipboard/{sessionID}.
func (h *Handlers) handleClipboard(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Path[len("/api/v1/rdgateway/clipboard/"):]
	if sessionID == "" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		state, err := h.clipboard.GetClipboard(sessionID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: state})
	case http.MethodPut:
		var payload ClipboardPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid request: " + err.Error()})
			return
		}
		h.clipboard.UpdateClipboard(sessionID, payload.Content, payload.Format, "api")
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "clipboard updated"})
	case http.MethodDelete:
		h.clipboard.ClearClipboard(sessionID)
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "clipboard cleared"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTransfers 处理 /api/v1/rdgateway/transfers.
func (h *Handlers) handleTransfers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID := r.URL.Query().Get("session_id")
	transfers := h.fileTransfer.ListTransfers(sessionID)
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"total":     len(transfers),
			"transfers": transfers,
		},
	})
}

// handleTransferByID 处理 /api/v1/rdgateway/transfers/{id} 及子路径.
func (h *Handlers) handleTransferByID(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/api/v1/rdgateway/transfers/"):]
	if path == "" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		transfer, err := h.fileTransfer.GetTransfer(path)
		if err != nil {
			writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: transfer})
	case http.MethodDelete:
		if err := h.fileTransfer.DeleteTransfer(path); err != nil {
			writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "deleted"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAudit 处理 /api/v1/rdgateway/audit.
func (h *Handlers) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	entries := h.sessions.GetAuditLog(sessionID, limit)
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"total":   len(entries),
			"entries": entries,
		},
	})
}

// handleTestConnection 处理 /api/v1/rdgateway/test.
func (h *Handlers) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	host := r.URL.Query().Get("host")
	portStr := r.URL.Query().Get("port")
	protocol := Protocol(r.URL.Query().Get("protocol"))
	useTLS := r.URL.Query().Get("tls") == "true"

	if host == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{Code: 1, Message: "host is required"})
		return
	}

	port := 3389
	if protocol == ProtocolVNC {
		port = 5900
	}
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	ok, err := h.sessions.TestConnection(host, port, protocol, useTLS)
	if err != nil {
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "connection failed",
			Data:    map[string]interface{}{"reachable": false, "error": err.Error()},
		})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: fmt.Sprintf("connection to %s:%d successful", host, port),
		Data:    map[string]interface{}{"reachable": ok},
	})
}
