package networkqos

import (
	"encoding/json"
	"net/http"
)

// QoSHandler QoS HTTP处理器
type QoSHandler struct {
	manager *QoSManager
}

// NewQoSHandler 创建处理器
func NewQoSHandler(manager *QoSManager) *QoSHandler {
	return &QoSHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *QoSHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/networkqos/rules", h.handleListRules)
	mux.HandleFunc("/api/networkqos/rule/create", h.handleCreateRule)
	mux.HandleFunc("/api/networkqos/rule/get", h.handleGetRule)
	mux.HandleFunc("/api/networkqos/rule/update", h.handleUpdateRule)
	mux.HandleFunc("/api/networkqos/rule/delete", h.handleDeleteRule)
	mux.HandleFunc("/api/networkqos/rule/enable", h.handleEnableRule)
	mux.HandleFunc("/api/networkqos/rule/disable", h.handleDisableRule)
	mux.HandleFunc("/api/networkqos/stats", h.handleGetStats)
	mux.HandleFunc("/api/networkqos/stats/all", h.handleGetAllStats)
}

// handleListRules 列出所有规则
func (h *QoSHandler) handleListRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rules := h.manager.ListRules()
	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    rules,
	})
}

// handleCreateRule 创建规则
func (h *QoSHandler) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rule QoSRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "无效的请求体",
		})
		return
	}

	created, err := h.manager.CreateRule(&rule)
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    created,
	})
}

// handleGetRule 获取规则
func (h *QoSHandler) handleGetRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "缺少id参数",
		})
		return
	}

	rule, err := h.manager.GetRule(id)
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    rule,
	})
}

// handleUpdateRule 更新规则
func (h *QoSHandler) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID     string  `json:"id"`
		Update QoSRule `json:"update"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "无效的请求体",
		})
		return
	}

	rule, err := h.manager.UpdateRule(req.ID, &req.Update)
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    rule,
	})
}

// handleDeleteRule 删除规则
func (h *QoSHandler) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "无效的请求体",
		})
		return
	}

	if err := h.manager.DeleteRule(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
	})
}

// handleEnableRule 启用规则
func (h *QoSHandler) handleEnableRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "无效的请求体",
		})
		return
	}

	if err := h.manager.EnableRule(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
	})
}

// handleDisableRule 禁用规则
func (h *QoSHandler) handleDisableRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "无效的请求体",
		})
		return
	}

	if err := h.manager.DisableRule(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
	})
}

// handleGetStats 获取统计
func (h *QoSHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "缺少id参数",
		})
		return
	}

	stats, err := h.manager.GetStats(id)
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// handleGetAllStats 获取所有统计
func (h *QoSHandler) handleGetAllStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.manager.GetAllStats()
	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// writeJSON 写入JSON响应
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
