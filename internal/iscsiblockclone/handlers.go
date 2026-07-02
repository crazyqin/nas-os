package iscsiblockclone

import (
	"encoding/json"
	"net/http"
)

// APIHandler HTTP API处理器.
type APIHandler struct {
	manager *BlockCloneManager
}

// NewAPIHandler 创建API处理器.
func NewAPIHandler(manager *BlockCloneManager) *APIHandler {
	return &APIHandler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/blockclone/luns", h.handleLUNs)
	mux.HandleFunc(prefix+"/blockclone/clone", h.handleClone)
	mux.HandleFunc(prefix+"/blockclone/tasks", h.handleTasks)
	mux.HandleFunc(prefix+"/blockclone/stats", h.handleStats)
}

func (h *APIHandler) handleLUNs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.manager.ListLUNs())
}

func (h *APIHandler) handleClone(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		SourceID   string    `json:"source_id"`
		TargetName string    `json:"target_name"`
		CloneType  CloneType `json:"clone_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.CloneType == "" {
		req.CloneType = CloneLinked
	}
	task, err := h.manager.CloneLUN(req.SourceID, req.TargetName, req.CloneType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (h *APIHandler) handleTasks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.manager.ListTasks())
}

func (h *APIHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.manager.GetStats())
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
