package accesspattern

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// AccessPatternHandler HTTP 处理器
type AccessPatternHandler struct {
	manager *AccessPatternManager
}

// NewAccessPatternHandler 创建处理器
func NewAccessPatternHandler(manager *AccessPatternManager) *AccessPatternHandler {
	return &AccessPatternHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *AccessPatternHandler) RegisterRoutes(mux *http.ServeMux) {
	// 访问记录
	mux.HandleFunc("/api/v1/accesspattern/record", h.handleRecordAccess)

	// 模式分析
	mux.HandleFunc("/api/v1/accesspattern/analyze", h.handleAnalyze)
	mux.HandleFunc("/api/v1/accesspattern/analyze/all", h.handleAnalyzeAll)
	mux.HandleFunc("/api/v1/accesspattern/analysis/get", h.handleGetAnalysis)

	// 热力图
	mux.HandleFunc("/api/v1/accesspattern/heatmap", h.handleGetHeatMap)

	// 统计
	mux.HandleFunc("/api/v1/accesspattern/stats", h.handleGetStats)

	// 分层建议
	mux.HandleFunc("/api/v1/accesspattern/tiering/report", h.handleGetTieringReport)

	// 清理
	mux.HandleFunc("/api/v1/accesspattern/cleanup", h.handleCleanup)
}

// handleRecordAccess 处理记录访问请求
func (h *AccessPatternHandler) handleRecordAccess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RecordAccessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, AccessPatternResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	record, err := h.manager.RecordAccess(&req)
	if err != nil {
		writeJSON(w, AccessPatternResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, AccessPatternResponse{
		Code:    0,
		Message: "success",
		Data:    record,
	})
}

// handleAnalyze 处理分析请求
func (h *AccessPatternHandler) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AnalysisRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, AccessPatternResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if req.FilePath == "" {
		writeJSON(w, AccessPatternResponse{
			Code:    400,
			Message: "缺少file_path参数",
		})
		return
	}

	analysis, err := h.manager.AnalyzeFile(req.FilePath)
	if err != nil {
		writeJSON(w, AccessPatternResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, AccessPatternResponse{
		Code:    0,
		Message: "success",
		Data:    analysis,
	})
}

// handleAnalyzeAll 处理分析所有文件请求
func (h *AccessPatternHandler) handleAnalyzeAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	results := h.manager.AnalyzeAll()

	writeJSON(w, AnalysisListResponse{
		Code:    0,
		Message: "success",
		Data:    results,
	})
}

// handleGetAnalysis 处理获取分析结果请求
func (h *AccessPatternHandler) handleGetAnalysis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filePath := r.URL.Query().Get("file_path")
	if filePath == "" {
		writeJSON(w, AccessPatternResponse{
			Code:    400,
			Message: "缺少file_path参数",
		})
		return
	}

	analysis, err := h.manager.GetAnalysis(filePath)
	if err != nil {
		writeJSON(w, AccessPatternResponse{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, AccessPatternResponse{
		Code:    0,
		Message: "success",
		Data:    analysis,
	})
}

// handleGetHeatMap 处理获取热力图请求
func (h *AccessPatternHandler) handleGetHeatMap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取时间范围参数
	startTimeStr := r.URL.Query().Get("start_time")
	endTimeStr := r.URL.Query().Get("end_time")
	limitStr := r.URL.Query().Get("limit")

	var startTime, endTime time.Time
	var err error

	if startTimeStr != "" {
		startTime, err = time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			writeJSON(w, AccessPatternResponse{
				Code:    400,
				Message: "无效的start_time格式",
			})
			return
		}
	} else {
		startTime = time.Now().AddDate(0, 0, -30) // 默认最近30天
	}

	if endTimeStr != "" {
		endTime, err = time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			writeJSON(w, AccessPatternResponse{
				Code:    400,
				Message: "无效的end_time格式",
			})
			return
		}
	} else {
		endTime = time.Now()
	}

	limit := 100 // 默认100条
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	heatMap := h.manager.GenerateHeatMap(startTime, endTime, limit)

	writeJSON(w, HeatMapResponse{
		Code:    0,
		Message: "success",
		Data:    heatMap,
	})
}

// handleGetStats 处理获取统计信息请求
func (h *AccessPatternHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.manager.GetStats()

	writeJSON(w, StatsResponse{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// handleGetTieringReport 处理获取分层报告请求
func (h *AccessPatternHandler) handleGetTieringReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	report := h.manager.GenerateTieringReport()

	writeJSON(w, TieringResponse{
		Code:    0,
		Message: "success",
		Data:    report,
	})
}

// handleCleanup 处理清理请求
func (h *AccessPatternHandler) handleCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	removed := h.manager.Cleanup()

	writeJSON(w, AccessPatternResponse{
		Code:    0,
		Message: "success",
		Data: map[string]int{
			"removed_records": removed,
		},
	})
}

// writeJSON 写入JSON响应
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
