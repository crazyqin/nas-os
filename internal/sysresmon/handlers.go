package sysresmon

import (
	"encoding/json"
	"net/http"
	"time"
)

// Handler HTTP API 处理器
type Handler struct {
	monitor   *ResourceMonitor
	dashboard *Dashboard
}

// NewHandler 创建处理器
func NewHandler(monitor *ResourceMonitor) *Handler {
	return &Handler{
		monitor:   monitor,
		dashboard: NewDashboard(monitor),
	}
}

// APIResponse 通用 API 响应
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/system/resources", h.handleResources)
	mux.HandleFunc("/api/v1/system/resources/history", h.handleHistory)
	mux.HandleFunc("/api/v1/system/resources/bottleneck", h.handleBottleneck)
	mux.HandleFunc("/api/v1/system/resources/predict", h.handlePredict)
}

// handleResources 处理实时资源概览
// GET /api/v1/system/resources
func (h *Handler) handleResources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, APIResponse{
			Code:    405,
			Message: "Method not allowed",
		})
		return
	}

	latest := h.monitor.GetLatest()
	if latest == nil {
		writeJSON(w, http.StatusServiceUnavailable, APIResponse{
			Code:    503,
			Message: "监控数据尚未就绪",
		})
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Code:    200,
		Message: "success",
		Data:    latest,
	})
}

// handleHistory 处理历史趋势查询
// GET /api/v1/system/resources/history?range=1h
func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, APIResponse{
			Code:    405,
			Message: "Method not allowed",
		})
		return
	}

	// 解析时间范围参数
	rangeParam := r.URL.Query().Get("range")
	if rangeParam == "" {
		rangeParam = "1h"
	}

	rangeType := TimeRange(rangeParam)
	switch rangeType {
	case Range1H, Range6H, Range24H, Range7D:
		// 有效范围
	default:
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "无效的时间范围，支持: 1h, 6h, 24h, 7d",
		})
		return
	}

	// 获取仪表盘数据
	data := h.dashboard.GetDashboardData(rangeType)

	writeJSON(w, http.StatusOK, APIResponse{
		Code:    200,
		Message: "success",
		Data:    data,
	})
}

// handleBottleneck 处理性能瓶颈分析
// GET /api/v1/system/resources/bottleneck
func (h *Handler) handleBottleneck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, APIResponse{
			Code:    405,
			Message: "Method not allowed",
		})
		return
	}

	// 默认分析最近 1 小时
	data := h.dashboard.GetDashboardData(Range1H)

	writeJSON(w, http.StatusOK, APIResponse{
		Code:    200,
		Message: "success",
		Data:    data.Bottleneck,
	})
}

// handlePredict 处理资源使用预测
// GET /api/v1/system/resources/predict
func (h *Handler) handlePredict(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, APIResponse{
			Code:    405,
			Message: "Method not allowed",
		})
		return
	}

	// 默认预测基于最近 6 小时数据
	data := h.dashboard.GetDashboardData(Range6H)

	writeJSON(w, http.StatusOK, APIResponse{
		Code:    200,
		Message: "success",
		Data:    data.Prediction,
	})
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// HealthCheck 健康检查处理器
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
		"uptime":    h.monitor.GetLatest().Uptime,
	}

	writeJSON(w, http.StatusOK, status)
}
