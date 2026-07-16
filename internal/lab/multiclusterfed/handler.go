package multiclusterfed

import (
	"encoding/json"
	"net/http"
)

// Handler 多集群联邦管理 HTTP 处理器.
type Handler struct {
	mgr *ClusterFederationManager
}

// NewHandler 创建处理器.
func NewHandler(mgr *ClusterFederationManager) *Handler {
	return &Handler{mgr: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/multiclusterfed/status", h.handleStatus)
	mux.HandleFunc("/api/v1/multiclusterfed/clusters", h.handleClusters)
	mux.HandleFunc("/api/v1/multiclusterfed/add", h.handleAdd)
	mux.HandleFunc("/api/v1/multiclusterfed/remove", h.handleRemove)
	mux.HandleFunc("/api/v1/multiclusterfed/sync", h.handleSync)
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"running":  h.mgr.IsRunning(),
		"clusters": h.mgr.GetAllClusterStatus(),
	})
}

func (h *Handler) handleClusters(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.mgr.GetAllClusterStatus())
}

func (h *Handler) handleAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var cluster Cluster
	if err := json.NewDecoder(r.Body).Decode(&cluster); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.mgr.AddCluster(&cluster); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
}

func (h *Handler) handleRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ClusterID string `json:"cluster_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.mgr.RemoveCluster(req.ClusterID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (h *Handler) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sync_started"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
