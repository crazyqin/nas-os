// Package vpnmanager 提供 REST API 处理器
package vpnmanager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Handler VPN API 处理器
type Handler struct {
	manager *VPNManager
}

// NewHandler 创建处理器
func NewHandler(manager *VPNManager) *Handler {
	return &Handler{manager: manager}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/vpn/tunnels", h.handleTunnels)
	mux.HandleFunc("/api/vpn/tunnels/", h.handleTunnelByID)
	mux.HandleFunc("/api/vpn/import", h.handleImport)
	mux.HandleFunc("/api/vpn/export/", h.handleExport)
}

// handleTunnels 处理 /api/vpn/tunnels 路由
func (h *Handler) handleTunnels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listTunnels(w, r)
	case http.MethodPost:
		h.createTunnel(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleTunnelByID 处理 /api/vpn/tunnels/:id 路由
func (h *Handler) handleTunnelByID(w http.ResponseWriter, r *http.Request) {
	// 提取 ID 和子路由
	path := strings.TrimPrefix(r.URL.Path, "/api/vpn/tunnels/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]

	if id == "" {
		writeError(w, http.StatusBadRequest, "tunnel ID is required")
		return
	}

	// 检查是否有子路由
	subPath := ""
	if len(parts) > 1 {
		subPath = parts[1]
	}

	switch subPath {
	case "start":
		h.startTunnel(w, r, id)
	case "stop":
		h.stopTunnel(w, r, id)
	case "traffic":
		h.getTrafficStats(w, r, id)
	case "":
		// 无子路由，根据方法处理
		switch r.Method {
		case http.MethodGet:
			h.getTunnel(w, r, id)
		case http.MethodPut:
			h.updateTunnel(w, r, id)
		case http.MethodDelete:
			h.deleteTunnel(w, r, id)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	default:
		writeError(w, http.StatusNotFound, "endpoint not found")
	}
}

// listTunnels 列出隧道
func (h *Handler) listTunnels(w http.ResponseWriter, r *http.Request) {
	tunnels := h.manager.ListTunnels()
	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    tunnels,
	})
}

// createTunnel 创建隧道
func (h *Handler) createTunnel(w http.ResponseWriter, r *http.Request) {
	var req TunnelCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	tunnel, err := h.manager.CreateTunnel(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, response{
		Code:    0,
		Message: "tunnel created",
		Data:    tunnel,
	})
}

// getTunnel 获取隧道详情
func (h *Handler) getTunnel(w http.ResponseWriter, r *http.Request, id string) {
	tunnel, err := h.manager.GetTunnel(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    tunnel,
	})
}

// updateTunnel 更新隧道
func (h *Handler) updateTunnel(w http.ResponseWriter, r *http.Request, id string) {
	var req TunnelUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	tunnel, err := h.manager.UpdateTunnel(id, &req)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "tunnel updated",
		Data:    tunnel,
	})
}

// deleteTunnel 删除隧道
func (h *Handler) deleteTunnel(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.manager.DeleteTunnel(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "tunnel deleted",
	})
}

// startTunnel 启动隧道
func (h *Handler) startTunnel(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := h.manager.StartTunnel(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "tunnel starting",
	})
}

// stopTunnel 停止隧道
func (h *Handler) stopTunnel(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := h.manager.StopTunnel(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "tunnel stopped",
	})
}

// getTrafficStats 获取流量统计
func (h *Handler) getTrafficStats(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	stats, err := h.manager.GetTrafficStats(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// handleImport 处理导入配置
func (h *Handler) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ImportConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	tunnel, err := h.manager.ImportConfig(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, response{
		Code:    0,
		Message: "config imported",
		Data:    tunnel,
	})
}

// handleExport 处理导出配置
func (h *Handler) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// 提取 ID
	id := strings.TrimPrefix(r.URL.Path, "/api/vpn/export/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "tunnel ID is required")
		return
	}

	config, err := h.manager.ExportConfig(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: map[string]string{
			"config": config,
		},
	})
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应
func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, response{
		Code:    1,
		Message: message,
	})
}

// SetupHTTPHandler 设置 HTTP 处理器（便捷函数）
func SetupHTTPHandler(manager *VPNManager) http.Handler {
	mux := http.NewServeMux()
	handler := NewHandler(manager)
	handler.RegisterRoutes(mux)
	return mux
}

// StartHTTPServer 启动 HTTP 服务器（便捷函数）
func StartHTTPServer(manager *VPNManager, addr string) error {
	handler := SetupHTTPHandler(manager)
	fmt.Printf("VPN Manager HTTP server starting on %s\n", addr)
	return http.ListenAndServe(addr, handler)
}
