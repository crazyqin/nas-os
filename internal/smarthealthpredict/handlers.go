package smarthealthpredict

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
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
	mux.HandleFunc("/api/v1/storage/health/scan", h.handleScan)
	mux.HandleFunc("/api/v1/storage/health/disks", h.handleDiskList)
	mux.HandleFunc("/api/v1/storage/health/history", h.handleHistory)
	mux.HandleFunc("/api/v1/storage/health/alerts", h.handleAlerts)
	mux.HandleFunc("/api/v1/storage/health/status", h.handleStatus)
}

// handleScan 处理扫描请求
func (h *Handler) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	device := r.URL.Query().Get("device")
	if device == "" {
		http.Error(w, "device parameter required", http.StatusBadRequest)
		return
	}

	report, err := h.manager.ScanDisk(r.Context(), device)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

// handleDiskList 处理磁盘列表请求
func (h *Handler) handleDiskList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	disks := h.manager.GetDiskList()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"disks": disks,
		"total": len(disks),
	})
}

// handleHistory 处理历史数据请求
func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	device := r.URL.Query().Get("device")
	if device == "" {
		http.Error(w, "device parameter required", http.StatusBadRequest)
		return
	}

	daysStr := r.URL.Query().Get("days")
	days := 30
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	history := h.manager.GetDiskHistory(device, days)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"device":  device,
		"history": history,
		"days":    days,
		"count":   len(history),
	})
}

// handleAlerts 处理告警请求
func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	alerts := h.manager.GetAlerts()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

// handleStatus 处理状态请求
func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	disks := h.manager.GetDiskList()

	// 统计各状态磁盘数量
	statusCount := map[string]int{
		"excellent": 0,
		"good":      0,
		"fair":      0,
		"poor":      0,
		"critical":  0,
	}

	var totalScore int
	for _, disk := range disks {
		// 获取追踪器获取健康分
		h.manager.mu.RLock()
		if tracker, ok := h.manager.disks[disk.Device]; ok {
			statusCount[string(tracker.Status)]++
			totalScore += tracker.HealthScore
		}
		h.manager.mu.RUnlock()
	}

	avgScore := 0
	if len(disks) > 0 {
		avgScore = totalScore / len(disks)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"totalDisks":   len(disks),
		"averageScore": avgScore,
		"statusCount":  statusCount,
		"timestamp":    time.Now(),
	})
}
