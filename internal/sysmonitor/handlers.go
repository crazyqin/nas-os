package sysmonitor

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP API 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建 HTTP API 处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/sys/overview", h.handleOverview)
	mux.HandleFunc("/api/v1/sys/processes", h.handleProcesses)
	mux.HandleFunc("/api/v1/sys/diskusage", h.handleDiskUsage)
	mux.HandleFunc("/api/v1/sys/network", h.handleNetwork)
	mux.HandleFunc("/api/v1/sys/load", h.handleLoad)
	mux.HandleFunc("/api/v1/sys/uptime", h.handleUptime)
	mux.HandleFunc("/api/v1/sys/alerts", h.handleAlerts)
	mux.HandleFunc("/api/v1/sys/history", h.handleHistory)
}

// handleOverview GET /api/v1/sys/overview - 系统概览
func (h *Handler) handleOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	overview := h.manager.GetOverview()
	if overview == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "系统数据尚未采集，请稍后重试",
		})
		return
	}

	writeJSON(w, http.StatusOK, overview)
}

// handleProcesses GET /api/v1/sys/processes - 进程列表
func (h *Handler) handleProcesses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	processes := h.manager.GetProcesses()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"processes": processes,
		"count":     len(processes),
	})
}

// handleDiskUsage GET /api/v1/sys/diskusage - 磁盘使用
func (h *Handler) handleDiskUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	diskUsage := h.manager.GetDiskUsage()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"disks": diskUsage,
		"count": len(diskUsage),
	})
}

// handleNetwork GET /api/v1/sys/network - 网络连接
func (h *Handler) handleNetwork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	network := h.manager.GetNetwork()
	writeJSON(w, http.StatusOK, network)
}

// handleLoad GET /api/v1/sys/load - 系统负载
func (h *Handler) handleLoad(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	loadInfo := h.manager.GetLoad()
	writeJSON(w, http.StatusOK, loadInfo)
}

// handleUptime GET /api/v1/sys/uptime - 运行时间
func (h *Handler) handleUptime(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uptime := h.manager.GetUptime()
	writeJSON(w, http.StatusOK, uptime)
}

// handleAlerts GET /api/v1/sys/alerts - 当前告警
func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	alerts := h.manager.GetAlerts()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

// handleHistory GET /api/v1/sys/history - 历史趋势
func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	history := h.manager.GetHistory()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"history": history,
		"count":   len(history),
	})
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
