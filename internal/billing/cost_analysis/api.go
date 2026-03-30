// Package cost_analysis 提供成本分析API处理程序
package cost_analysis

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// APIHandler 成本分析API处理程序.
type APIHandler struct {
	engine *CostAnalysisEngine
}

// NewAPIHandler 创建API处理程序.
func NewAPIHandler(engine *CostAnalysisEngine) *APIHandler {
	return &APIHandler{engine: engine}
}

// RegisterRoutes 注册路由.
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	// 报告生成
	mux.HandleFunc("/api/cost/reports/storage-trend", h.HandleStorageTrendReport)
	mux.HandleFunc("/api/cost/reports/resource-utilization", h.HandleResourceUtilizationReport)
	mux.HandleFunc("/api/cost/reports/optimization", h.HandleOptimizationReport)
	mux.HandleFunc("/api/cost/reports/budget-tracking", h.HandleBudgetTrackingReport)
	mux.HandleFunc("/api/cost/reports/comprehensive", h.HandleComprehensiveReport)

	// 预算管理
	mux.HandleFunc("/api/cost/budgets", h.HandleBudgets)
	mux.HandleFunc("/api/cost/budgets/", h.HandleBudgetByID)

	// 告警管理
	mux.HandleFunc("/api/cost/alerts", h.HandleAlerts)
	mux.HandleFunc("/api/cost/alerts/", h.HandleAlertByID)

	// 趋势数据
	mux.HandleFunc("/api/cost/trends", h.HandleTrends)
}

// HandleStorageTrendReport 处理存储成本趋势报告请求.
func (h *APIHandler) HandleStorageTrendReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 解析参数
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if val, err := strconv.Atoi(d); err == nil && val > 0 {
			days = val
		}
	}

	report, err := h.engine.GenerateStorageTrendReport(days)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, report)
}

// HandleResourceUtilizationReport 处理资源利用率报告请求.
func (h *APIHandler) HandleResourceUtilizationReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	report, err := h.engine.GenerateResourceUtilizationReport()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, report)
}

// HandleOptimizationReport 处理成本优化建议报告请求.
func (h *APIHandler) HandleOptimizationReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	report, err := h.engine.GenerateOptimizationReport()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, report)
}

// HandleBudgetTrackingReport 处理预算跟踪报告请求.
func (h *APIHandler) HandleBudgetTrackingReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	budgetID := r.URL.Query().Get("budget_id")
	if budgetID == "" {
		h.writeError(w, http.StatusBadRequest, "budget_id is required")
		return
	}

	report, err := h.engine.GenerateBudgetTrackingReport(budgetID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, report)
}

// HandleComprehensiveReport 处理综合成本分析报告请求.
func (h *APIHandler) HandleComprehensiveReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	report, err := h.engine.GenerateComprehensiveReport()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, report)
}

// HandleBudgets 处理预算列表请求.
func (h *APIHandler) HandleBudgets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		budgets := h.engine.ListBudgets()
		h.writeJSON(w, http.StatusOK, budgets)

	case http.MethodPost:
		var config BudgetConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			h.writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		budget, err := h.engine.CreateBudget(config)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		h.writeJSON(w, http.StatusCreated, budget)

	default:
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// HandleBudgetByID 处理单个预算请求.
func (h *APIHandler) HandleBudgetByID(w http.ResponseWriter, r *http.Request) {
	// 提取预算ID
	id := r.URL.Path[len("/api/cost/budgets/"):]
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "Budget ID is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		budget, err := h.engine.GetBudget(id)
		if err != nil {
			h.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		h.writeJSON(w, http.StatusOK, budget)

	case http.MethodPut:
		var config BudgetConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			h.writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		budget, err := h.engine.UpdateBudget(id, config)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		h.writeJSON(w, http.StatusOK, budget)

	case http.MethodDelete:
		if err := h.engine.DeleteBudget(id); err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// HandleAlerts 处理告警列表请求.
func (h *APIHandler) HandleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	alerts := h.engine.GetAlerts()
	h.writeJSON(w, http.StatusOK, alerts)
}

// HandleAlertByID 处理单个告警请求.
func (h *APIHandler) HandleAlertByID(w http.ResponseWriter, r *http.Request) {
	// 提取告警ID
	id := r.URL.Path[len("/api/cost/alerts/"):]
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "Alert ID is required")
		return
	}

	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 解析动作
	var req struct {
		Action string `json:"action"` // acknowledge
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	switch req.Action {
	case "acknowledge":
		if err := h.engine.AcknowledgeAlert(id); err != nil {
			h.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)

	default:
		h.writeError(w, http.StatusBadRequest, "Invalid action")
	}
}

// HandleTrends 处理趋势数据请求.
func (h *APIHandler) HandleTrends(w http.ResponseWriter, r *http.Request) {
	// POST 方法用于记录新的趋势数据点
	if r.Method == http.MethodPost {
		var trend CostTrend
		if err := json.NewDecoder(r.Body).Decode(&trend); err != nil {
			h.writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if trend.Date.IsZero() {
			trend.Date = time.Now()
		}

		h.engine.RecordTrendData(trend)
		h.writeJSON(w, http.StatusCreated, trend)
		return
	}

	// GET 方法用于获取趋势数据
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 返回趋势数据
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	start, _ := time.Parse(time.RFC3339, startStr) // 忽略解析错误，使用默认值
	end, _ := time.Parse(time.RFC3339, endStr)     // 忽略解析错误，使用默认值

	if start.IsZero() {
		start = time.Now().AddDate(0, 0, -30)
	}
	if end.IsZero() {
		end = time.Now()
	}

	report, err := h.engine.GenerateStorageTrendReport(int(end.Sub(start).Hours() / 24))
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, report.Trends)
}

// 辅助方法.
func (h *APIHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("encode error", "error", err)
	}
}

func (h *APIHandler) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		slog.Error("encode error", "error", err)
	}
}
