// Package optimizer 提供资源配额优化 API
package optimizer

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// APIHandler 配额优化 API 处理器
type APIHandler struct {
	optimizer *QuotaOptimizer
}

// NewAPIHandler 创建 API 处理器
func NewAPIHandler(optimizer *QuotaOptimizer) *APIHandler {
	return &APIHandler{optimizer: optimizer}
}

// RegisterRoutes 注册路由
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	// 优化建议
	mux.HandleFunc("/api/quota/optimizer/suggestions", h.HandleSuggestions)
	mux.HandleFunc("/api/quota/optimizer/suggestions/generate", h.HandleGenerateSuggestions)
	mux.HandleFunc("/api/quota/optimizer/suggestions/apply/", h.HandleApplySuggestion)
	mux.HandleFunc("/api/quota/optimizer/suggestions/dismiss/", h.HandleDismissSuggestion)

	// 使用预测
	mux.HandleFunc("/api/quota/optimizer/prediction/", h.HandlePrediction)
	mux.HandleFunc("/api/quota/optimizer/predictions", h.HandleAllPredictions)

	// 违规检测
	mux.HandleFunc("/api/quota/optimizer/violations", h.HandleViolations)
	mux.HandleFunc("/api/quota/optimizer/violations/detect", h.HandleDetectViolations)
	mux.HandleFunc("/api/quota/optimizer/violations/resolve/", h.HandleResolveViolation)

	// 优化报告
	mux.HandleFunc("/api/quota/optimizer/report", h.HandleOptimizationReport)

	// 调整历史
	mux.HandleFunc("/api/quota/optimizer/history", h.HandleAdjustHistory)
}

// ========== 优化建议 API ==========

// HandleSuggestions 处理建议列表请求
func (h *APIHandler) HandleSuggestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	status := r.URL.Query().Get("status")
	suggestions := h.optimizer.GetSuggestions(status)

	h.writeJSON(w, suggestions)
}

// HandleGenerateSuggestions 处理生成建议请求
func (h *APIHandler) HandleGenerateSuggestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	suggestions, err := h.optimizer.GenerateAdjustmentSuggestions()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, map[string]interface{}{
		"generated_count": len(suggestions),
		"suggestions":     suggestions,
	})
}

// HandleApplySuggestion 处理应用建议请求
func (h *APIHandler) HandleApplySuggestion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	// 提取建议ID
	suggestionID := r.URL.Path[len("/api/quota/optimizer/suggestions/apply/"):]
	if suggestionID == "" {
		h.writeError(w, http.StatusBadRequest, "缺少建议ID")
		return
	}

	if err := h.optimizer.ApplySuggestion(suggestionID); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, map[string]string{"status": "ok"})
}

// HandleDismissSuggestion 处理忽略建议请求
func (h *APIHandler) HandleDismissSuggestion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	// 提取建议ID
	suggestionID := r.URL.Path[len("/api/quota/optimizer/suggestions/dismiss/"):]
	if suggestionID == "" {
		h.writeError(w, http.StatusBadRequest, "缺少建议ID")
		return
	}

	if err := h.optimizer.DismissSuggestion(suggestionID); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, map[string]string{"status": "ok"})
}

// ========== 使用预测 API ==========

// HandlePrediction 处理单个配额预测请求
func (h *APIHandler) HandlePrediction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	// 提取配额ID
	quotaID := r.URL.Path[len("/api/quota/optimizer/prediction/"):]
	if quotaID == "" {
		h.writeError(w, http.StatusBadRequest, "缺少配额ID")
		return
	}

	prediction, err := h.optimizer.PredictUsage(quotaID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, prediction)
}

// HandleAllPredictions 处理所有配额预测请求
func (h *APIHandler) HandleAllPredictions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	predictions, err := h.optimizer.PredictAllUsage()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, predictions)
}

// ========== 违规检测 API ==========

// HandleViolations 处理违规列表请求
func (h *APIHandler) HandleViolations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	status := r.URL.Query().Get("status")
	violations := h.optimizer.GetViolations(status)

	h.writeJSON(w, violations)
}

// HandleDetectViolations 处理检测违规请求
func (h *APIHandler) HandleDetectViolations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	violations, err := h.optimizer.DetectViolations()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, map[string]interface{}{
		"detected_count": len(violations),
		"violations":     violations,
	})
}

// HandleResolveViolation 处理解决违规请求
func (h *APIHandler) HandleResolveViolation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	// 提取违规ID
	violationID := r.URL.Path[len("/api/quota/optimizer/violations/resolve/"):]
	if violationID == "" {
		h.writeError(w, http.StatusBadRequest, "缺少违规ID")
		return
	}

	var req struct {
		ResolvedBy string `json:"resolved_by"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.ResolvedBy = "system"
	}

	if err := h.optimizer.ResolveViolation(violationID, req.ResolvedBy); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, map[string]string{"status": "ok"})
}

// ========== 优化报告 API ==========

// HandleOptimizationReport 处理优化报告请求
func (h *APIHandler) HandleOptimizationReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	report, err := h.optimizer.GenerateOptimizationReport()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, report)
}

// ========== 调整历史 API ==========

// HandleAdjustHistory 处理调整历史请求
func (h *APIHandler) HandleAdjustHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	limit := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if _, err := fmt.Sscanf(l, "%d", &limit); err != nil {
			limit = 0
		}
	}

	history := h.optimizer.GetAdjustHistory(limit)
	h.writeJSON(w, history)
}

// ========== 辅助方法 ==========

func (h *APIHandler) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// 编码失败时记录日志，但无法修改响应头
	}
}

func (h *APIHandler) writeError(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		// 编码失败时无法修改响应
	}
}
