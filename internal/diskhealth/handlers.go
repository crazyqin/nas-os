// Package diskhealth 硬盘健康监控 - HTTP 处理器
package diskhealth

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler HTTP 处理器
type Handler struct {
	service *Service
}

// NewHandler 创建处理器
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/disk-health/disks", h.handleDisks)
	mux.HandleFunc("/disk-health/disks/", h.handleDiskDetail)
	mux.HandleFunc("/disk-health/alerts", h.handleAlerts)
	mux.HandleFunc("/disk-health/scan", h.handleScan)
	mux.HandleFunc("/disk-health/summary", h.handleSummary)
}

// handleDisks 获取磁盘列表
func (h *Handler) handleDisks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	disks := h.service.GetAllDisks()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"code": 0,
		"data": disks,
	})
}

// handleDiskDetail 获取磁盘详情
func (h *Handler) handleDiskDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析路径: /disk-health/disks/:device 或 /disk-health/disks/:device/smart
	path := strings.TrimPrefix(r.URL.Path, "/disk-health/disks/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Device required", http.StatusBadRequest)
		return
	}

	device := "/dev/" + parts[0]

	if len(parts) == 1 {
		// 获取磁盘详情
		disk, exists := h.service.GetDiskInfo(device)
		if !exists {
			http.Error(w, "Disk not found", http.StatusNotFound)
			return
		}
		assessment, _ := h.service.GetAssessment(device)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{
				"disk":       disk,
				"assessment": assessment,
			},
		})
		return
	}

	switch parts[1] {
	case "smart":
		// 获取 S.M.A.R.T. 属性
		disk, exists := h.service.GetDiskInfo(device)
		if !exists {
			http.Error(w, "Disk not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{
				"device":     device,
				"attributes": disk.SMARTAttrs,
			},
		})

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

	device := r.URL.Query().Get("device")
	includeAcked := r.URL.Query().Get("includeAcked") == "true"

	alerts := h.service.GetAlerts(device, includeAcked)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"code": 0,
		"data": alerts,
	})
}

// handleScan 扫描磁盘
func (h *Handler) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 后台执行扫描
	go h.service.scanAllDisks()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "扫描已启动",
	})
}

// handleSummary 获取健康摘要
func (h *Handler) handleSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	summary := h.service.GetHealthSummary()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"code": 0,
		"data": summary,
	})
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// getRiskLevel 获取风险等级
func getRiskLevel(prob float64) string {
	switch {
	case prob >= 0.5:
		return "critical"
	case prob >= 0.2:
		return "high"
	case prob >= 0.1:
		return "medium"
	case prob >= 0.05:
		return "low"
	default:
		return "minimal"
	}
}

// getRecommendations 获取建议
func getRecommendations(prob float64) []string {
	var recs []string
	if prob >= 0.5 {
		recs = append(recs, "立即备份所有重要数据")
		recs = append(recs, "准备替换磁盘")
		recs = append(recs, "减少对磁盘的写入操作")
	} else if prob >= 0.2 {
		recs = append(recs, "尽快备份重要数据")
		recs = append(recs, "监控磁盘状态")
		recs = append(recs, "考虑购买备用磁盘")
	} else if prob >= 0.1 {
		recs = append(recs, "定期备份数据")
		recs = append(recs, "保持磁盘良好散热")
	} else {
		recs = append(recs, "磁盘状态良好")
		recs = append(recs, "继续定期检查")
	}
	return recs
}
