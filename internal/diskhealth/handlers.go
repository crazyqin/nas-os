// Package diskhealth 硬盘健康监控 - HTTP 处理器
package diskhealth

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/disk-health/disks", h.handleDisks)
	mux.HandleFunc("/disk-health/disks/", h.handleDiskDetail)
	mux.HandleFunc("/disk-health/alerts", h.handleAlerts)
	mux.HandleFunc("/disk-health/scan", h.handleScan)
	mux.HandleFunc("/disk-health/config", h.handleConfig)
}

// handleDisks 获取磁盘列表
func (h *Handler) handleDisks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	disks := h.manager.ScanDisks()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"disks": disks,
	})
}

// handleDiskDetail 获取磁盘详情
func (h *Handler) handleDiskDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析路径: /disk-health/disks/:device 或 /disk-health/disks/:device/smart 或 /disk-health/disks/:device/report
	path := strings.TrimPrefix(r.URL.Path, "/disk-health/disks/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Device required", http.StatusBadRequest)
		return
	}

	device := parts[0]

	if len(parts) == 1 {
		// 获取磁盘详情
		disk, exists := h.manager.GetDiskInfo(device)
		if !exists {
			http.Error(w, "Disk not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, disk)
		return
	}

	switch parts[1] {
	case "smart":
		// 获取 S.M.A.R.T. 属性
		report, exists := h.manager.GetHealthReport(device)
		if !exists {
			http.Error(w, "Disk not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"device":     report.Device,
			"attributes": report.Attributes,
		})

	case "report":
		// 获取健康报告
		report, exists := h.manager.GetHealthReport(device)
		if !exists {
			http.Error(w, "Disk not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, report)

	default:
		http.Error(w, "Unknown endpoint", http.StatusNotFound)
	}
}

// handleAlerts 获取告警列表
func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	alerts := h.manager.CheckAlerts()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
	})
}

// handleScan 扫描磁盘
func (h *Handler) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	disks := h.manager.ScanDisks()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "completed",
		"message": "扫描完成",
		"count":   len(disks),
	})
}

// handleConfig 获取/更新告警配置
func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config := h.manager.GetConfig()
		writeJSON(w, http.StatusOK, config)

	case http.MethodPut:
		var config AlertConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		h.manager.UpdateConfig(config)
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
