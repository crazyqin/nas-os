package clustermanager

import (
	"encoding/json"
	"net/http"
)

// Handlers HTTP处理器
type Handlers struct {
	manager *ClusterManager
}

// NewHandlers 创建处理器
func NewHandlers(manager *ClusterManager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterNode 注册节点
func (h *Handlers) RegisterNode(w http.ResponseWriter, r *http.Request) {
	var req AddNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	node, err := h.manager.RegisterNode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(node)
}

// GetNode 获取节点
func (h *Handlers) GetNode(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "节点ID必填", http.StatusBadRequest)
		return
	}

	node, err := h.manager.GetNode(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

// ListNodes 列出节点
func (h *Handlers) ListNodes(w http.ResponseWriter, r *http.Request) {
	nodes := h.manager.ListNodes()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
}

// UnregisterNode 注销节点
func (h *Handlers) UnregisterNode(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "节点ID必填", http.StatusBadRequest)
		return
	}

	if err := h.manager.UnregisterNode(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "unregistered"})
}

// Heartbeat 心跳
func (h *Handlers) Heartbeat(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "节点ID必填", http.StatusBadRequest)
		return
	}

	if err := h.manager.Heartbeat(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// CreateCluster 创建集群
func (h *Handlers) CreateCluster(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cluster, err := h.manager.CreateCluster(req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cluster)
}

// ListClusters 列出集群
func (h *Handlers) ListClusters(w http.ResponseWriter, r *http.Request) {
	clusters := h.manager.ListClusters()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clusters)
}

// GetClusterStats 获取集群统计
func (h *Handlers) GetClusterStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetClusterStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
