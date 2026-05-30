// Package selectiveadsync - 选择性AD同步 HTTP 处理器
package selectiveadsync

import (
	"encoding/json"
	"net/http"
)

// SelectiveADSyncHandler HTTP 处理器
type SelectiveADSyncHandler struct {
	manager *SelectiveADSyncManager
}

// NewSelectiveADSyncHandler 创建处理器
func NewSelectiveADSyncHandler(manager *SelectiveADSyncManager) *SelectiveADSyncHandler {
	return &SelectiveADSyncHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *SelectiveADSyncHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/ad/ous", h.handleListOUs)
	mux.HandleFunc("/api/v1/ad/ous/discover", h.handleDiscoverOUs)
	mux.HandleFunc("/api/v1/ad/ous/select", h.handleSelectOUs)
	mux.HandleFunc("/api/v1/ad/ous/deselect", h.handleDeselectOUs)
	mux.HandleFunc("/api/v1/ad/ous/selected", h.handleGetSelectedOUs)
	mux.HandleFunc("/api/v1/ad/sync", h.handleSync)
	mux.HandleFunc("/api/v1/ad/sync/status", h.handleSyncStatus)
	mux.HandleFunc("/api/v1/ad/sync/history", h.handleSyncHistory)
	mux.HandleFunc("/api/v1/ad/sync/stats", h.handleSyncStats)
	mux.HandleFunc("/api/v1/ad/rules", h.handleListRules)
	mux.HandleFunc("/api/v1/ad/rules/create", h.handleCreateRule)
	mux.HandleFunc("/api/v1/ad/rules/update", h.handleUpdateRule)
	mux.HandleFunc("/api/v1/ad/rules/delete", h.handleDeleteRule)
	mux.HandleFunc("/api/v1/ad/config", h.handleConfig)
}

// handleListOUs 处理列出所有OU请求
func (h *SelectiveADSyncHandler) handleListOUs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ous := h.manager.ListOUs()

	writeJSON(w, OUListResponse{
		Code:    0,
		Message: "success",
		Data:    ous,
	})
}

// handleDiscoverOUs 处理发现OU请求
func (h *SelectiveADSyncHandler) handleDiscoverOUs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ous, err := h.manager.DiscoverOUs()
	if err != nil {
		writeJSON(w, SyncResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, OUListResponse{
		Code:    0,
		Message: "success",
		Data:    ous,
	})
}

// handleSelectOUs 处理选择OU请求
func (h *SelectiveADSyncHandler) handleSelectOUs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req OUSelectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, SyncResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if len(req.OUDNs) == 0 {
		writeJSON(w, SyncResponse{
			Code:    400,
			Message: "OU列表不能为空",
		})
		return
	}

	err := h.manager.SelectOUs(req.OUDNs, req.Replace)
	if err != nil {
		writeJSON(w, SyncResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, SyncResponse{
		Code:    0,
		Message: "success",
	})
}

// handleDeselectOUs 处理取消选择OU请求
func (h *SelectiveADSyncHandler) handleDeselectOUs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		OUDNs []string `json:"ou_dns"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, SyncResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	err := h.manager.DeselectOUs(req.OUDNs)
	if err != nil {
		writeJSON(w, SyncResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, SyncResponse{
		Code:    0,
		Message: "success",
	})
}

// handleGetSelectedOUs 处理获取已选择OU请求
func (h *SelectiveADSyncHandler) handleGetSelectedOUs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ous := h.manager.GetSelectedOUs()

	writeJSON(w, OUListResponse{
		Code:    0,
		Message: "success",
		Data:    ous,
	})
}

// handleSync 处理同步请求
func (h *SelectiveADSyncHandler) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// 如果没有请求体，使用默认值
		req = SyncRequest{}
	}

	result, err := h.manager.Sync(req)
	if err != nil {
		writeJSON(w, SyncResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, SyncResultResponse{
		Code:    0,
		Message: "success",
		Data:    *result,
	})
}

// handleSyncStatus 处理同步状态请求
func (h *SelectiveADSyncHandler) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	result := h.manager.GetLastSyncResult()
	if result == nil {
		writeJSON(w, SyncResponse{
			Code:    0,
			Message: "尚未执行同步",
			Data:    nil,
		})
		return
	}

	writeJSON(w, SyncResultResponse{
		Code:    0,
		Message: "success",
		Data:    *result,
	})
}

// handleSyncHistory 处理同步历史请求
func (h *SelectiveADSyncHandler) handleSyncHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	history := h.manager.GetSyncHistory()

	writeJSON(w, SyncHistoryResponse{
		Code:    0,
		Message: "success",
		Data:    history,
	})
}

// handleSyncStats 处理同步统计请求
func (h *SelectiveADSyncHandler) handleSyncStats(w http.ResponseWriter, r *http.Request) {
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

// handleListRules 处理列出规则请求
func (h *SelectiveADSyncHandler) handleListRules(w http.ResponseWriter, r *http.Request) {
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
func (h *SelectiveADSyncHandler) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, SyncResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	rule, err := h.manager.CreateRule(req.Rule)
	if err != nil {
		writeJSON(w, SyncResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, SyncResponse{
		Code:    0,
		Message: "success",
		Data:    rule,
	})
}

// handleUpdateRule 处理更新规则请求
func (h *SelectiveADSyncHandler) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, SyncResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	err := h.manager.UpdateRule(req.Rule)
	if err != nil {
		writeJSON(w, SyncResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, SyncResponse{
		Code:    0,
		Message: "success",
	})
}

// handleDeleteRule 处理删除规则请求
func (h *SelectiveADSyncHandler) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		RuleID string `json:"rule_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, SyncResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	err := h.manager.DeleteRule(req.RuleID)
	if err != nil {
		writeJSON(w, SyncResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, SyncResponse{
		Code:    0,
		Message: "success",
	})
}

// handleConfig 处理配置请求
func (h *SelectiveADSyncHandler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config := h.manager.GetConfig()
		writeJSON(w, SyncResponse{
			Code:    0,
			Message: "success",
			Data:    config,
		})
	case http.MethodPost:
		var config OUSyncConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			writeJSON(w, SyncResponse{
				Code:    400,
				Message: "无效的请求体",
			})
			return
		}
		h.manager.SetConfig(config)
		writeJSON(w, SyncResponse{
			Code:    0,
			Message: "success",
		})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// writeJSON 写入JSON响应
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
