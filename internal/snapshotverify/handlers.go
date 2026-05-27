package snapshotverify

import (
	"encoding/json"
	"net/http"
)

// SnapshotVerifyHandler 快照验证HTTP处理器
type SnapshotVerifyHandler struct {
	manager *SnapshotVerifyManager
}

// NewSnapshotVerifyHandler 创建处理器
func NewSnapshotVerifyHandler(manager *SnapshotVerifyManager) *SnapshotVerifyHandler {
	return &SnapshotVerifyHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *SnapshotVerifyHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/snapshotverify/verify", h.handleVerify)
	mux.HandleFunc("/api/snapshotverify/batch-verify", h.handleBatchVerify)
	mux.HandleFunc("/api/snapshotverify/integrity", h.handleIntegrity)
	mux.HandleFunc("/api/snapshotverify/repair", h.handleRepair)
	mux.HandleFunc("/api/snapshotverify/verifiers", h.handleGetVerifiers)
	mux.HandleFunc("/api/snapshotverify/history", h.handleGetHistory)
	mux.HandleFunc("/api/snapshotverify/stats", h.handleGetStats)
	mux.HandleFunc("/api/snapshotverify/policy", h.handlePolicy)
}

// handleVerify 处理验证请求
func (h *SnapshotVerifyHandler) handleVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SnapshotID string `json:"snapshot_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	job, err := h.manager.StartVerification(req.SnapshotID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, job)
}

// handleBatchVerify 处理批量验证请求
func (h *SnapshotVerifyHandler) handleBatchVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Snapshots []*SnapshotInfo `json:"snapshots"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	results, err := h.manager.BatchVerify(req.Snapshots)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, results)
}

// handleIntegrity 处理完整性检查请求
func (h *SnapshotVerifyHandler) handleIntegrity(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	report, err := h.manager.VerifyIntegrity(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, report)
}

// handleRepair 处理修复请求
func (h *SnapshotVerifyHandler) handleRepair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SnapshotID string `json:"snapshot_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result, err := h.manager.RepairSnapshot(req.SnapshotID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, result)
}

// handleGetVerifiers 处理获取验证器请求
func (h *SnapshotVerifyHandler) handleGetVerifiers(w http.ResponseWriter, r *http.Request) {
	verifiers := h.manager.GetVerifiers()
	respondJSON(w, verifiers)
}

// handleGetHistory 处理获取历史请求
func (h *SnapshotVerifyHandler) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	snapshotID := r.URL.Query().Get("snapshot_id")
	if snapshotID == "" {
		http.Error(w, "Snapshot ID is required", http.StatusBadRequest)
		return
	}

	history := h.manager.GetVerificationHistory(snapshotID)
	respondJSON(w, history)
}

// handleGetStats 处理获取统计请求
func (h *SnapshotVerifyHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetVerificationStats()
	respondJSON(w, stats)
}

// handlePolicy 处理策略请求
func (h *SnapshotVerifyHandler) handlePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var policy VerificationPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.CreateVerificationPolicy(&policy); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]string{"status": "created"})
}

// respondJSON 响应JSON
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
