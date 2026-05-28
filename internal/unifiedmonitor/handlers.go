package unifiedmonitor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Handler HTTP API handlers
type Handler struct {
	monitor *Monitor
}

// NewHandler 创建HTTP handler
func NewHandler(monitor *Monitor) *Handler {
	return &Handler{monitor: monitor}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/monitor/health", h.handleHealth)
	mux.HandleFunc("/api/v1/monitor/metrics", h.handleMetrics)
	mux.HandleFunc("/api/v1/monitor/alerts", h.handleAlerts)
	mux.HandleFunc("/api/v1/monitor/rules", h.handleRules)
	mux.HandleFunc("/api/v1/monitor/dashboard", h.handleDashboard)
}

// APIResponse 统一API响应
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// handleHealth 系统健康评分
// GET /api/v1/monitor/health
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	score, err := h.monitor.GetHealthScore(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get health score: %v", err))
		return
	}
	
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    score,
	})
}

// handleMetrics 指标查询
// GET /api/v1/monitor/metrics?name=cpu_usage&node_id=node1&start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z
func (h *Handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	query := r.URL.Query()
	name := query.Get("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "name parameter is required")
		return
	}
	
	nodeID := query.Get("node_id")
	
	startStr := query.Get("start")
	endStr := query.Get("end")
	
	var start, end time.Time
	var err error
	
	if startStr != "" {
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid start time: %v", err))
			return
		}
	} else {
		start = time.Now().Add(-1 * time.Hour)
	}
	
	if endStr != "" {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid end time: %v", err))
			return
		}
	} else {
		end = time.Now()
	}
	
	points, err := h.monitor.QueryMetrics(r.Context(), name, nodeID, start, end)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("query metrics: %v", err))
		return
	}
	
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    points,
	})
}

// handleAlerts 告警列表
// GET /api/v1/monitor/alerts?status=firing&limit=100
func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getAlerts(w, r)
	case http.MethodPost:
		h.updateAlert(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) getAlerts(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	statusStr := query.Get("status")
	
	var status AlertStatus
	switch statusStr {
	case "firing":
		status = AlertStatusFiring
	case "resolved":
		status = AlertStatusResolved
	case "silenced":
		status = AlertStatusSilenced
	case "":
		status = AlertStatusFiring
	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid status: %s", statusStr))
		return
	}
	
	limitStr := query.Get("limit")
	limit := 100
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			writeError(w, http.StatusBadRequest, "invalid limit parameter")
			return
		}
	}
	
	alerts, err := h.monitor.GetAlerts(r.Context(), status, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get alerts: %v", err))
		return
	}
	
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    alerts,
	})
}

// AlertUpdateRequest 告警更新请求
type AlertUpdateRequest struct {
	Action  string `json:"action"`  // acknowledge/resolve
	AlertID string `json:"alert_id"`
}

func (h *Handler) updateAlert(w http.ResponseWriter, r *http.Request) {
	var req AlertUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	
	if req.AlertID == "" {
		writeError(w, http.StatusBadRequest, "alert_id is required")
		return
	}
	
	var err error
	switch req.Action {
	case "acknowledge":
		err = h.monitor.AcknowledgeAlert(r.Context(), req.AlertID)
	case "resolve":
		err = h.monitor.ResolveAlert(r.Context(), req.AlertID)
	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid action: %s", req.Action))
		return
	}
	
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("update alert: %v", err))
		return
	}
	
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]string{"status": "updated"},
	})
}

// handleRules 告警规则管理
// POST /api/v1/monitor/rules - 创建规则
// GET /api/v1/monitor/rules - 获取规则列表
// DELETE /api/v1/monitor/rules?id=xxx - 删除规则
func (h *Handler) handleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getRules(w, r)
	case http.MethodPost:
		h.createRule(w, r)
	case http.MethodDelete:
		h.deleteRule(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) getRules(w http.ResponseWriter, r *http.Request) {
	h.monitor.mu.RLock()
	defer h.monitor.mu.RUnlock()
	
	rules := make([]*AlertRule, 0, len(h.monitor.rules))
	for _, rule := range h.monitor.rules {
		rules = append(rules, rule)
	}
	
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    rules,
	})
}

func (h *Handler) createRule(w http.ResponseWriter, r *http.Request) {
	var rule AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	
	// 验证必填字段
	if rule.ID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if rule.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if rule.Metric == "" {
		writeError(w, http.StatusBadRequest, "metric is required")
		return
	}
	
	// 设置默认值
	if rule.Type == "" {
		rule.Type = RuleTypeThreshold
	}
	if rule.Condition == "" {
		rule.Condition = ConditionAbove
	}
	if rule.Severity == "" {
		rule.Severity = SeverityWarning
	}
	if rule.Duration == 0 {
		rule.Duration = 5 * time.Minute
	}
	rule.Enabled = true
	
	if err := h.monitor.AddRule(&rule); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("add rule: %v", err))
		return
	}
	
	writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    rule,
	})
}

func (h *Handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	ruleID := r.URL.Query().Get("id")
	if ruleID == "" {
		writeError(w, http.StatusBadRequest, "id parameter is required")
		return
	}
	
	h.monitor.RemoveRule(ruleID)
	
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]string{"status": "deleted"},
	})
}

// handleDashboard 仪表板数据
// GET /api/v1/monitor/dashboard
func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	data, err := h.monitor.GetDashboard(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get dashboard: %v", err))
		return
	}
	
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
	})
}

// writeJSON 写入JSON响应
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应
func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, APIResponse{
		Success: false,
		Error:   message,
	})
}
