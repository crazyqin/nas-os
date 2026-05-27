package datagovernance

import (
	"encoding/json"
	"net/http"
)

// Handlers HTTP处理器
type Handlers struct {
	manager *DataGovernanceManager
}

// NewHandlers 创建处理器
func NewHandlers(manager *DataGovernanceManager) *Handlers {
	return &Handlers{manager: manager}
}

// CreatePolicy 创建策略
func (h *Handlers) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	var policy Policy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.manager.CreatePolicy(&policy); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(policy)
}

// GetPolicy 获取策略
func (h *Handlers) GetPolicy(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "策略ID必填", http.StatusBadRequest)
		return
	}

	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(policy)
}

// ListPolicies 列出策略
func (h *Handlers) ListPolicies(w http.ResponseWriter, r *http.Request) {
	policies := h.manager.ListPolicies()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(policies)
}

// UpdatePolicy 更新策略
func (h *Handlers) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "策略ID必填", http.StatusBadRequest)
		return
	}

	var policy Policy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.manager.UpdatePolicy(id, &policy); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// DeletePolicy 删除策略
func (h *Handlers) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "策略ID必填", http.StatusBadRequest)
		return
	}

	if err := h.manager.DeletePolicy(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// ListClassifications 列出分类
func (h *Handlers) ListClassifications(w http.ResponseWriter, r *http.Request) {
	classes := h.manager.ListClassifications()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(classes)
}

// ListRetentionRules 列出保留规则
func (h *Handlers) ListRetentionRules(w http.ResponseWriter, r *http.Request) {
	rules := h.manager.ListRetentionRules()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rules)
}

// GetAuditRecords 获取审计记录
func (h *Handlers) GetAuditRecords(w http.ResponseWriter, r *http.Request) {
	records := h.manager.GetAuditRecords(100)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(records)
}

// GetStats 获取统计
func (h *Handlers) GetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
