package smartbandwidth

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler 智能带宽HTTP处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/bandwidth/stats", h.handleGetStats)
	mux.HandleFunc("/api/bandwidth/rules", h.handleRules)
	mux.HandleFunc("/api/bandwidth/classes", h.handleGetClasses)
	mux.HandleFunc("/api/bandwidth/usage", h.handleGetUsage)
	mux.HandleFunc("/api/bandwidth/qos", h.handleQoS)
	mux.HandleFunc("/api/bandwidth/profiles", h.handleProfiles)
	mux.HandleFunc("/api/bandwidth/adjust", h.handleAdjust)
}

// handleGetStats GET /api/bandwidth/stats
func (h *Handler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ruleID := r.URL.Query().Get("rule_id")
	class := r.URL.Query().Get("class")

	if ruleID != "" {
		stats, err := h.manager.GetBandwidthStats(ruleID)
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
		return
	}

	if class != "" {
		trafficClass := TrafficClass(class)
		stats := h.manager.GetBandwidthStatsByClass(trafficClass)
		writeJSON(w, map[string]interface{}{
			"code":    0,
			"message": "success",
			"data":    stats,
		})
		return
	}

	stats := h.manager.GetAllBandwidthStats()
	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// handleRules POST /api/bandwidth/rules, PUT /api/bandwidth/rules/:id
func (h *Handler) handleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleListRules(w, r)
	case http.MethodPost:
		h.handleCreateRule(w, r)
	case http.MethodPut:
		h.handleUpdateRule(w, r)
	case http.MethodDelete:
		h.handleDeleteRule(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleListRules 列出所有规则
func (h *Handler) handleListRules(w http.ResponseWriter, r *http.Request) {
	rules := h.manager.ListBandwidthRules()
	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    rules,
	})
}

// handleCreateRule 创建规则
func (h *Handler) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	var rule BandwidthRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "无效的请求体",
		})
		return
	}

	created, err := h.manager.SetBandwidthLimit(&rule)
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

// handleUpdateRule 更新规则
func (h *Handler) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	// 从URL路径中提取ID
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "缺少规则ID",
		})
		return
	}
	id := parts[4]

	var update BandwidthRule
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "无效的请求体",
		})
		return
	}

	rule, err := h.manager.UpdateBandwidthRule(id, &update)
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
func (h *Handler) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
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

	if err := h.manager.DeleteBandwidthRule(req.ID); err != nil {
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

// handleGetClasses GET /api/bandwidth/classes
func (h *Handler) handleGetClasses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	summary := h.manager.GetClassSummary()
	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    summary,
	})
}

// handleGetUsage GET /api/bandwidth/usage
func (h *Handler) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	usage := h.manager.GetBandwidthUsage()
	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    usage,
	})
}

// handleQoS QoS策略管理
func (h *Handler) handleQoS(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleListQoSPolicies(w, r)
	case http.MethodPost:
		h.handleCreateQoSPolicy(w, r)
	case http.MethodPut:
		h.handleUpdateQoSPolicy(w, r)
	case http.MethodDelete:
		h.handleDeleteQoSPolicy(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleListQoSPolicies 列出QoS策略
func (h *Handler) handleListQoSPolicies(w http.ResponseWriter, r *http.Request) {
	policies := h.manager.ListQoSPolicies()
	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    policies,
	})
}

// handleCreateQoSPolicy 创建QoS策略
func (h *Handler) handleCreateQoSPolicy(w http.ResponseWriter, r *http.Request) {
	var policy QoSPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "无效的请求体",
		})
		return
	}

	created, err := h.manager.ApplyQoSPolicy(&policy)
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

// handleUpdateQoSPolicy 更新QoS策略
func (h *Handler) handleUpdateQoSPolicy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string    `json:"id"`
		Update QoSPolicy `json:"update"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "无效的请求体",
		})
		return
	}

	policy, err := h.manager.UpdateQoSPolicy(req.ID, &req.Update)
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
		"data":    policy,
	})
}

// handleDeleteQoSPolicy 删除QoS策略
func (h *Handler) handleDeleteQoSPolicy(w http.ResponseWriter, r *http.Request) {
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

	if err := h.manager.DeleteQoSPolicy(req.ID); err != nil {
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

// handleProfiles 流量配置文件管理
func (h *Handler) handleProfiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleListProfiles(w, r)
	case http.MethodPost:
		h.handleCreateProfile(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleListProfiles 列出流量配置文件
func (h *Handler) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	profiles := h.manager.GetTrafficProfiles()
	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    profiles,
	})
}

// handleCreateProfile 创建流量配置文件
func (h *Handler) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	var profile TrafficProfile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "无效的请求体",
		})
		return
	}

	created, err := h.manager.CreateTrafficProfile(&profile)
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

// handleAdjust POST /api/bandwidth/adjust
func (h *Handler) handleAdjust(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.manager.AdjustDynamic(); err != nil {
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

// writeJSON 写入JSON响应
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
