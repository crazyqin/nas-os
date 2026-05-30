// Package storageanomaly - HTTP 处理器
// 处理存储异常检测相关的 HTTP 请求
package storageanomaly

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// AnomalyHandler HTTP 处理器
type AnomalyHandler struct {
	manager *AnomalyManager
}

// NewAnomalyHandler 创建处理器
func NewAnomalyHandler(manager *AnomalyManager) *AnomalyHandler {
	return &AnomalyHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *AnomalyHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/anomaly/detect", h.handleDetect)
	mux.HandleFunc("/api/v1/anomaly/metrics/collect", h.handleCollectMetrics)
	mux.HandleFunc("/api/v1/anomaly/metrics/get", h.handleGetMetrics)
	mux.HandleFunc("/api/v1/anomaly/events", h.handleListEvents)
	mux.HandleFunc("/api/v1/anomaly/event/get", h.handleGetEvent)
	mux.HandleFunc("/api/v1/anomaly/event/ack", h.handleAckEvent)
	mux.HandleFunc("/api/v1/anomaly/event/resolve", h.handleResolveEvent)
	mux.HandleFunc("/api/v1/anomaly/rules", h.handleListRules)
	mux.HandleFunc("/api/v1/anomaly/rule/create", h.handleCreateRule)
	mux.HandleFunc("/api/v1/anomaly/rule/get", h.handleGetRule)
	mux.HandleFunc("/api/v1/anomaly/rule/update", h.handleUpdateRule)
	mux.HandleFunc("/api/v1/anomaly/rule/delete", h.handleDeleteRule)
	mux.HandleFunc("/api/v1/anomaly/stats", h.handleGetStats)
}

// handleDetect 处理异常检测请求
func (h *AnomalyHandler) handleDetect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var deviceID string

	if r.Method == http.MethodGet {
		deviceID = r.URL.Query().Get("device_id")
	} else {
		var req struct {
			DeviceID string `json:"device_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, AnomalyResponse{
				Code:    400,
				Message: "无效的请求体",
			})
			return
		}
		deviceID = req.DeviceID
	}

	if deviceID == "" {
		writeJSON(w, AnomalyResponse{
			Code:    400,
			Message: "缺少device_id参数",
		})
		return
	}

	result, err := h.manager.DetectAnomalies(deviceID)
	if err != nil {
		writeJSON(w, AnomalyResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, AnomalyResponse{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// handleCollectMetrics 处理指标收集请求
func (h *AnomalyHandler) handleCollectMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var metrics StorageMetrics
	if err := json.NewDecoder(r.Body).Decode(&metrics); err != nil {
		writeJSON(w, AnomalyResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if err := h.manager.CollectMetrics(&metrics); err != nil {
		writeJSON(w, AnomalyResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, AnomalyResponse{
		Code:    0,
		Message: "success",
	})
}

// handleGetMetrics 处理获取指标请求
func (h *AnomalyHandler) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		writeJSON(w, AnomalyResponse{
			Code:    400,
			Message: "缺少device_id参数",
		})
		return
	}

	h.manager.mu.RLock()
	history, exists := h.manager.metrics[deviceID]
	h.manager.mu.RUnlock()

	if !exists || len(history) == 0 {
		writeJSON(w, MetricsResponse{
			Code:    404,
			Message: "无指标数据",
		})
		return
	}

	// 返回最新指标
	latest := history[len(history)-1]
	writeJSON(w, MetricsResponse{
		Code:    0,
		Message: "success",
		Data:    latest,
	})
}

// handleListEvents 处理列出事件请求
func (h *AnomalyHandler) handleListEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deviceID := r.URL.Query().Get("device_id")
	severity := AnomalySeverity(r.URL.Query().Get("severity"))
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	events := h.manager.ListEvents(deviceID, severity, limit)

	writeJSON(w, EventListResponse{
		Code:    0,
		Message: "success",
		Data:    events,
	})
}

// handleGetEvent 处理获取事件请求
func (h *AnomalyHandler) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, AnomalyResponse{
			Code:    400,
			Message: "缺少id参数",
		})
		return
	}

	event, err := h.manager.GetEvent(id)
	if err != nil {
		writeJSON(w, AnomalyResponse{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, AnomalyResponse{
		Code:    0,
		Message: "success",
		Data:    event,
	})
}

// handleAckEvent 处理确认事件请求
func (h *AnomalyHandler) handleAckEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AckEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, AnomalyResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if req.EventID == "" || req.UserID == "" {
		writeJSON(w, AnomalyResponse{
			Code:    400,
			Message: "缺少参数",
		})
		return
	}

	if err := h.manager.AckEvent(req.EventID, req.UserID); err != nil {
		writeJSON(w, AnomalyResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, AnomalyResponse{
		Code:    0,
		Message: "success",
	})
}

// handleResolveEvent 处理解决事件请求
func (h *AnomalyHandler) handleResolveEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ResolveEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, AnomalyResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if req.EventID == "" {
		writeJSON(w, AnomalyResponse{
			Code:    400,
			Message: "缺少event_id参数",
		})
		return
	}

	if err := h.manager.ResolveEvent(req.EventID); err != nil {
		writeJSON(w, AnomalyResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, AnomalyResponse{
		Code:    0,
		Message: "success",
	})
}

// handleListRules 处理列出规则请求
func (h *AnomalyHandler) handleListRules(w http.ResponseWriter, r *http.Request) {
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
func (h *AnomalyHandler) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, AnomalyResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	rule, err := h.manager.CreateRule(&req)
	if err != nil {
		writeJSON(w, AnomalyResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, AnomalyResponse{
		Code:    0,
		Message: "success",
		Data:    rule,
	})
}

// handleGetRule 处理获取规则请求
func (h *AnomalyHandler) handleGetRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, AnomalyResponse{
			Code:    400,
			Message: "缺少id参数",
		})
		return
	}

	rule, err := h.manager.GetRule(id)
	if err != nil {
		writeJSON(w, AnomalyResponse{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, AnomalyResponse{
		Code:    0,
		Message: "success",
		Data:    rule,
	})
}

// handleUpdateRule 处理更新规则请求
func (h *AnomalyHandler) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, AnomalyResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if req.ID == "" {
		writeJSON(w, AnomalyResponse{
			Code:    400,
			Message: "缺少id参数",
		})
		return
	}

	if err := h.manager.UpdateRule(&req); err != nil {
		writeJSON(w, AnomalyResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, AnomalyResponse{
		Code:    0,
		Message: "success",
	})
}

// handleDeleteRule 处理删除规则请求
func (h *AnomalyHandler) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var id string
	if r.Method == http.MethodPost {
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, AnomalyResponse{
				Code:    400,
				Message: "无效的请求体",
			})
			return
		}
		id = req.ID
	} else {
		id = r.URL.Query().Get("id")
	}

	if id == "" {
		writeJSON(w, AnomalyResponse{
			Code:    400,
			Message: "缺少id参数",
		})
		return
	}

	if err := h.manager.DeleteRule(id); err != nil {
		writeJSON(w, AnomalyResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, AnomalyResponse{
		Code:    0,
		Message: "success",
	})
}

// handleGetStats 处理获取统计信息请求
func (h *AnomalyHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
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

// writeJSON 写入JSON响应
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
