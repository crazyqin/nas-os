package aideidentification

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// DeidentificationHandler HTTP 处理器
type DeidentificationHandler struct {
	manager *DeidentificationManager
}

// NewDeidentificationHandler 创建处理器
func NewDeidentificationHandler(manager *DeidentificationManager) *DeidentificationHandler {
	return &DeidentificationHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *DeidentificationHandler) RegisterRoutes(mux *http.ServeMux) {
	// 规则管理
	mux.HandleFunc("/api/v1/aideidentification/rules", h.handleListRules)
	mux.HandleFunc("/api/v1/aideidentification/rule/create", h.handleCreateRule)
	mux.HandleFunc("/api/v1/aideidentification/rule/update", h.handleUpdateRule)
	mux.HandleFunc("/api/v1/aideidentification/rule/delete", h.handleDeleteRule)
	mux.HandleFunc("/api/v1/aideidentification/rule/get", h.handleGetRule)

	// 脱敏处理
	mux.HandleFunc("/api/v1/aideidentification/deidentify", h.handleDeidentify)
	mux.HandleFunc("/api/v1/aideidentification/deidentify/batch", h.handleDeidentifyBatch)

	// 统计与日志
	mux.HandleFunc("/api/v1/aideidentification/stats", h.handleGetStats)
	mux.HandleFunc("/api/v1/aideidentification/audit", h.handleGetAuditLog)
}

// handleListRules 处理列出所有规则请求
func (h *DeidentificationHandler) handleListRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rules := h.manager.ListRules()

	writeJSON(w, RuleListResponse{
		Code:    0,
		Message: "success",
		Data:    rules,
	})
}

// handleCreateRule 处理创建规则请求
func (h *DeidentificationHandler) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, DeidentificationResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	rule, err := h.manager.CreateRule(&req)
	if err != nil {
		writeJSON(w, DeidentificationResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, DeidentificationResponse{
		Code:    0,
		Message: "success",
		Data:    rule,
	})
}

// handleUpdateRule 处理更新规则请求
func (h *DeidentificationHandler) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, DeidentificationResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	rule, err := h.manager.UpdateRule(&req)
	if err != nil {
		writeJSON(w, DeidentificationResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, DeidentificationResponse{
		Code:    0,
		Message: "success",
		Data:    rule,
	})
}

// handleDeleteRule 处理删除规则请求
func (h *DeidentificationHandler) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, DeidentificationResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if err := h.manager.DeleteRule(req.ID); err != nil {
		writeJSON(w, DeidentificationResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, DeidentificationResponse{
		Code:    0,
		Message: "success",
	})
}

// handleGetRule 处理获取单个规则请求
func (h *DeidentificationHandler) handleGetRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ruleID := r.URL.Query().Get("id")
	if ruleID == "" {
		writeJSON(w, DeidentificationResponse{
			Code:    400,
			Message: "缺少id参数",
		})
		return
	}

	rule, err := h.manager.GetRule(ruleID)
	if err != nil {
		writeJSON(w, DeidentificationResponse{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, DeidentificationResponse{
		Code:    0,
		Message: "success",
		Data:    rule,
	})
}

// handleDeidentify 处理脱敏请求
func (h *DeidentificationHandler) handleDeidentify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DeidentificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, DeidentificationResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	result, err := h.manager.Deidentify(req.Text, req.RuleID)
	if err != nil {
		writeJSON(w, DeidentificationResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, DeidentificationResponse{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// handleDeidentifyBatch 处理批量脱敏请求
func (h *DeidentificationHandler) handleDeidentifyBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BatchDeidentificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, DeidentificationResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	result, err := h.manager.DeidentifyBatch(&req)
	if err != nil {
		writeJSON(w, DeidentificationResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, DeidentificationResponse{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// handleGetStats 处理获取统计信息请求
func (h *DeidentificationHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
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

// handleGetAuditLog 处理获取审计日志请求
func (h *DeidentificationHandler) handleGetAuditLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取 limit 参数
	limitStr := r.URL.Query().Get("limit")
	limit := 100 // 默认100条
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	logs := h.manager.GetAuditLog(limit)

	writeJSON(w, AuditLogResponse{
		Code:    0,
		Message: "success",
		Data:    logs,
	})
}

// writeJSON 写入JSON响应
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
