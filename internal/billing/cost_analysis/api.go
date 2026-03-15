// Package cost_analysis 提供成本分析 API
package cost_analysis

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// APIHandler 成本分析 API 处理器
type APIHandler struct {
	engine *CostAnalysisEngine
}

// NewAPIHandler 创建 API 处理器
func NewAPIHandler(engine *CostAnalysisEngine) *APIHandler {
	return &APIHandler{engine: engine}
}

// RegisterRoutes 注册路由
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	// 成本报告
	mux.HandleFunc("/api/cost/report/storage-trend", h.HandleStorageTrendReport)
	mux.HandleFunc("/api/cost/report/resource-util", h.HandleResourceUtilReport)
	mux.HandleFunc("/api/cost/report/optimization", h.HandleOptimizationReport)
	mux.HandleFunc("/api/cost/report/comprehensive", h.HandleComprehensiveReport)

	// 预算管理
	mux.HandleFunc("/api/cost/budget", h.HandleBudgetList)
	mux.HandleFunc("/api/cost/budget/create", h.HandleBudgetCreate)
	mux.HandleFunc("/api/cost/budget/", h.HandleBudgetOperation)
	mux.HandleFunc("/api/cost/budget/tracking/", h.HandleBudgetTracking)

	// 告警
	mux.HandleFunc("/api/cost/alerts", h.HandleAlertList)
	mux.HandleFunc("/api/cost/alerts/acknowledge/", h.HandleAlertAcknowledge)
}

// ========== 成本报告 API ==========

// HandleStorageTrendReport 处理存储成本趋势报告请求
func (h *APIHandler) HandleStorageTrendReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	// 解析参数
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}

	// 生成报告
	report, err := h.engine.GenerateStorageTrendReport(days)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, report)
}

// HandleResourceUtilReport 处理资源利用率报告请求
func (h *APIHandler) HandleResourceUtilReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	// 生成报告
	report, err := h.engine.GenerateResourceUtilizationReport()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, report)
}

// HandleOptimizationReport 处理成本优化建议报告请求
func (h *APIHandler) HandleOptimizationReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	// 生成报告
	report, err := h.engine.GenerateOptimizationReport()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, report)
}

// HandleComprehensiveReport 处理综合成本分析报告请求
func (h *APIHandler) HandleComprehensiveReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	// 生成报告
	report, err := h.engine.GenerateComprehensiveReport()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, report)
}

// ========== 预算管理 API ==========

// HandleBudgetList 处理预算列表请求
func (h *APIHandler) HandleBudgetList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	budgets := h.engine.ListBudgets()
	h.writeJSON(w, budgets)
}

// HandleBudgetCreate 处理预算创建请求
func (h *APIHandler) HandleBudgetCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	var config BudgetConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		h.writeError(w, http.StatusBadRequest, "无效的请求数据")
		return
	}

	budget, err := h.engine.CreateBudget(config)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, budget)
}

// HandleBudgetOperation 处理预算操作（获取、更新、删除）
func (h *APIHandler) HandleBudgetOperation(w http.ResponseWriter, r *http.Request) {
	// 提取预算ID
	budgetID := r.URL.Path[len("/api/cost/budget/"):]
	if budgetID == "" {
		h.writeError(w, http.StatusBadRequest, "缺少预算ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		// 获取预算
		budget, err := h.engine.GetBudget(budgetID)
		if err != nil {
			h.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		h.writeJSON(w, budget)

	case http.MethodPut:
		// 更新预算
		var config BudgetConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			h.writeError(w, http.StatusBadRequest, "无效的请求数据")
			return
		}

		budget, err := h.engine.UpdateBudget(budgetID, config)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.writeJSON(w, budget)

	case http.MethodDelete:
		// 删除预算
		if err := h.engine.DeleteBudget(budgetID); err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// HandleBudgetTracking 处理预算跟踪请求
func (h *APIHandler) HandleBudgetTracking(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	// 提取预算ID
	budgetID := r.URL.Path[len("/api/cost/budget/tracking/"):]
	if budgetID == "" {
		h.writeError(w, http.StatusBadRequest, "缺少预算ID")
		return
	}

	// 生成预算跟踪报告
	report, err := h.engine.GenerateBudgetTrackingReport(budgetID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, report)
}

// ========== 告警 API ==========

// HandleAlertList 处理告警列表请求
func (h *APIHandler) HandleAlertList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	alerts := h.engine.GetAlerts()
	h.writeJSON(w, alerts)
}

// HandleAlertAcknowledge 处理告警确认请求
func (h *APIHandler) HandleAlertAcknowledge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	// 提取告警ID
	alertID := r.URL.Path[len("/api/cost/alerts/acknowledge/"):]
	if alertID == "" {
		h.writeError(w, http.StatusBadRequest, "缺少告警ID")
		return
	}

	if err := h.engine.AcknowledgeAlert(alertID); err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, map[string]string{"status": "ok"})
}

// ========== 辅助方法 ==========

func (h *APIHandler) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *APIHandler) writeError(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// ========== 数据导出 API ==========

// ExportFormat 导出格式
type ExportFormat string

const (
	ExportJSON ExportFormat = "json"
	ExportCSV  ExportFormat = "csv"
	ExportPDF  ExportFormat = "pdf"
)

// ExportRequest 导出请求
type ExportRequest struct {
	ReportID     string       `json:"report_id"`
	Format       ExportFormat `json:"format"`
	IncludeChart bool         `json:"include_chart"`
}

// ExportResponse 导出响应
type ExportResponse struct {
	DownloadURL string       `json:"download_url"`
	Format      ExportFormat `json:"format"`
	ExpiresAt   time.Time    `json:"expires_at"`
}

// HandleExportReport 处理报告导出请求
func (h *APIHandler) HandleExportReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	var req ExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "无效的请求数据")
		return
	}

	// TODO: 实现实际的导出逻辑
	// 这里返回一个模拟的下载链接
	response := ExportResponse{
		DownloadURL: fmt.Sprintf("/api/cost/download/%s.%s", req.ReportID, req.Format),
		Format:      req.Format,
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}

	h.writeJSON(w, response)
}
