// Package immutablestore 不可变存储模块 - HTTP 处理器
package immutablestore

import (
	"encoding/json"
	"net/http"
)

// HTTPHandler HTTP 处理器
type HTTPHandler struct {
	manager *ImmutableStorageManager
}

// NewHTTPHandler 创建 HTTP 处理器
func NewHTTPHandler(manager *ImmutableStorageManager) *HTTPHandler {
	return &HTTPHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/immutable/policies", h.handlePolicies)
	mux.HandleFunc("/api/immutable/policies/create", h.handleCreatePolicy)
	mux.HandleFunc("/api/immutable/files/lock", h.handleLockFile)
	mux.HandleFunc("/api/immutable/files", h.handleListFiles)
	mux.HandleFunc("/api/immutable/files/verify", h.handleVerifyFile)
	mux.HandleFunc("/api/immutable/audit", h.handleAuditLog)
	mux.HandleFunc("/api/immutable/stats", h.handleStats)
}

func (h *HTTPHandler) handlePolicies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.manager.ListPolicies())
}

func (h *HTTPHandler) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var policy ImmutablePolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.manager.CreatePolicy(&policy); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(policy)
}

func (h *HTTPHandler) handleLockFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FilePath string `json:"file_path"`
		PolicyID string `json:"policy_id"`
		UserID   string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	file, err := h.manager.LockFile(req.FilePath, req.PolicyID, req.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(file)
}

func (h *HTTPHandler) handleListFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.manager.ListLockedFiles())
}

func (h *HTTPHandler) handleVerifyFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FileID string `json:"file_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	valid, err := h.manager.VerifyIntegrity(req.FileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"file_id": req.FileID,
		"valid":   valid,
	})
}

func (h *HTTPHandler) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.manager.GetAuditLog(100))
}

func (h *HTTPHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.manager.GetStats())
}
