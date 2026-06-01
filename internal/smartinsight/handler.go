// Package smartinsight 提供 HTTP API 处理器
package smartinsight

import (
	"encoding/json"
	"net/http"
)

// Handler 智能洞察 HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由到 http.ServeMux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/smartinsight/insights", h.handleInsights)
	mux.HandleFunc("/api/v1/smartinsight/recommendations", h.handleRecommendations)
	mux.HandleFunc("/api/v1/smartinsight/anomalies", h.handleAnomalies)
	mux.HandleFunc("/api/v1/smartinsight/report", h.handleReport)
	mux.HandleFunc("/api/v1/smartinsight/report/latest", h.handleLatestReport)
	mux.HandleFunc("/api/v1/smartinsight/cost", h.handleCost)
	mux.HandleFunc("/api/v1/smartinsight/stats", h.handleStats)
}

// apiResponse 标准 API 响应
type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// handleInsights 处理洞察查询
// GET /api/v1/smartinsight/insights?category=storage
func (h *Handler) handleInsights(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Code: 1, Message: "method not allowed"})
		return
	}

	category := r.URL.Query().Get("category")
	period := r.URL.Query().Get("period")

	trends := h.manager.AnalyzeUsage(category, period)

	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    trends,
	})
}

// handleRecommendations 处理推荐查询
// GET /api/v1/smartinsight/recommendations?category=storage_optimize
func (h *Handler) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Code: 1, Message: "method not allowed"})
		return
	}

	category := r.URL.Query().Get("category")
	recs := h.manager.GetRecommendations(category)

	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    recs,
	})
}

// handleAnomalies 处理异常查询
// GET /api/v1/smartinsight/anomalies?type=file_access
func (h *Handler) handleAnomalies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Code: 1, Message: "method not allowed"})
		return
	}

	anomalyType := r.URL.Query().Get("type")
	anomalies := h.manager.DetectAnomalies(anomalyType)

	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    anomalies,
	})
}

// handleReport 处理报告生成
// POST /api/v1/smartinsight/report - 生成新报告
// GET /api/v1/smartinsight/report - 获取所有报告列表
func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		report := h.manager.GenerateReport()
		writeJSON(w, http.StatusCreated, apiResponse{
			Code:    0,
			Message: "report generated",
			Data:    report,
		})
	case http.MethodGet:
		reports := h.manager.GetAllReports()
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    reports,
		})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Code: 1, Message: "method not allowed"})
	}
}

// handleLatestReport 获取最新报告
// GET /api/v1/smartinsight/report/latest
func (h *Handler) handleLatestReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Code: 1, Message: "method not allowed"})
		return
	}

	report := h.manager.GetLatestReport()
	if report == nil {
		writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: "no report available, generate one first"})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    report,
	})
}

// handleCost 成本分析
// GET /api/v1/smartinsight/cost
func (h *Handler) handleCost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Code: 1, Message: "method not allowed"})
		return
	}

	cost := h.manager.AnalyzeCost()

	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    cost,
	})
}

// handleStats 系统统计概览
// GET /api/v1/smartinsight/stats
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Code: 1, Message: "method not allowed"})
		return
	}

	stats := h.manager.GetStats()

	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}
