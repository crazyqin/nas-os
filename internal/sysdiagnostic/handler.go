package sysdiagnostic

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Handler 系统诊断 API 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册 HTTP 路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/sysdiagnostic/diagnose", h.handleDiagnose)
	mux.HandleFunc("/api/v1/sysdiagnostic/reports", h.handleReports)
	mux.HandleFunc("/api/v1/sysdiagnostic/reports/", h.handleReportByID)
	mux.HandleFunc("/api/v1/sysdiagnostic/history", h.handleHistory)
	mux.HandleFunc("/api/v1/sysdiagnostic/baseline", h.handleBaseline)
	mux.HandleFunc("/api/v1/sysdiagnostic/schedule", h.handleSchedule)
	mux.HandleFunc("/api/v1/sysdiagnostic/health-score", h.handleHealthScore)
}

// apiResponse 标准 API 响应
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

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiResponse{Code: 1, Message: msg})
}

// handleDiagnose 处理诊断请求 POST /api/v1/sysdiagnostic/diagnose
func (h *Handler) handleDiagnose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req DiagnosticRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// 允许空 body，执行全面诊断
		req = DiagnosticRequest{}
	}

	report, err := h.manager.Diagnose(&req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "diagnosis completed", Data: report,
	})
}

// handleReports 处理报告列表 GET /api/v1/sysdiagnostic/reports
func (h *Handler) handleReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	reports := h.manager.ListReports()
	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "success", Data: reports,
	})
}

// handleReportByID 处理单个报告 GET /api/v1/sysdiagnostic/reports/:id
// 同时处理基线对比 POST /api/v1/sysdiagnostic/reports/:id/compare
func (h *Handler) handleReportByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/sysdiagnostic/reports/")
	if path == "" || path == r.URL.Path {
		writeError(w, http.StatusBadRequest, "report id required")
		return
	}

	// 处理子路径如 /reports/:id/compare
	parts := strings.SplitN(path, "/", 2)
	reportID := parts[0]

	switch r.Method {
	case http.MethodGet:
		report, err := h.manager.GetReport(reportID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code: 0, Message: "success", Data: report,
		})

	case http.MethodPost:
		if len(parts) > 1 && parts[1] == "compare" {
			comparison, err := h.manager.CompareWithBaseline(reportID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{
				Code: 0, Message: "comparison completed", Data: comparison,
			})
			return
		}
		writeError(w, http.StatusBadRequest, "unknown action")

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleHistory 处理诊断历史 GET /api/v1/sysdiagnostic/history
func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// 支持 limit 参数
	limit := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	history := h.manager.GetHistory()
	if limit > 0 && limit < len(history) {
		history = history[len(history)-limit:]
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "success", Data: history,
	})
}

// handleBaseline 处理基线 GET/POST /api/v1/sysdiagnostic/baseline
func (h *Handler) handleBaseline(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		baseline, err := h.manager.GetBaseline()
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code: 0, Message: "success", Data: baseline,
		})

	case http.MethodPost:
		var req struct {
			ReportID string `json:"reportId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}
		if req.ReportID == "" {
			writeError(w, http.StatusBadRequest, "reportId is required")
			return
		}
		if err := h.manager.UpdateBaseline(req.ReportID); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code: 0, Message: "baseline updated",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleSchedule 处理调度 GET/POST/PUT /api/v1/sysdiagnostic/schedule
func (h *Handler) handleSchedule(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		schedules := h.manager.ListSchedules()
		writeJSON(w, http.StatusOK, apiResponse{
			Code: 0, Message: "success", Data: schedules,
		})

	case http.MethodPost:
		var req struct {
			Name       string          `json:"name"`
			Interval   int64           `json:"interval"`
			Categories []CheckCategory `json:"categories"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}
		if req.Name == "" || req.Interval <= 0 {
			writeError(w, http.StatusBadRequest, "name and positive interval are required")
			return
		}
		schedule := h.manager.CreateSchedule(req.Name, req.Interval, req.Categories)
		writeJSON(w, http.StatusCreated, apiResponse{
			Code: 0, Message: "schedule created", Data: schedule,
		})

	case http.MethodPut:
		var req struct {
			ID       string `json:"id"`
			Enabled  *bool  `json:"enabled"`
			Interval *int64 `json:"interval"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}
		if req.ID == "" {
			writeError(w, http.StatusBadRequest, "id is required")
			return
		}
		schedule, err := h.manager.UpdateSchedule(req.ID, req.Enabled, req.Interval)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code: 0, Message: "schedule updated", Data: schedule,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleHealthScore 处理健康评分 GET /api/v1/sysdiagnostic/health-score
func (h *Handler) handleHealthScore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	result, err := h.manager.QuickHealthCheck()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "success", Data: result,
	})
}
