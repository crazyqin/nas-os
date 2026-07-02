// Package cluster 集群管理器 HTTP 处理器
package cluster

import (
	"encoding/json"
	"net/http"
	"strings"
)

// HTTPHandler 集群管理器 HTTP 处理器.
type HTTPHandler struct {
	manager *ClusterManager
}

// NewHTTPHandler 创建 HTTP 处理器.
func NewHTTPHandler(manager *ClusterManager) *HTTPHandler {
	return &HTTPHandler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/cluster/status", h.handleStatus)
	mux.HandleFunc("/api/cluster/nodes", h.handleNodes)
	mux.HandleFunc("/api/cluster/nodes/", h.handleNodeByID)
	mux.HandleFunc("/api/cluster/metrics", h.handleMetrics)
	mux.HandleFunc("/api/cluster/tasks", h.handleTasks)
	mux.HandleFunc("/api/cluster/primary", h.handlePrimary)
}

// handleStatus 处理集群状态请求.
func (h *HTTPHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := h.manager.GetClusterStatus()
	json.NewEncoder(w).Encode(status)
}

// handleNodes 处理节点列表请求.
func (h *HTTPHandler) handleNodes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		nodes := h.manager.ListNodes()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"nodes": nodes,
			"total": len(nodes),
		})

	case http.MethodPost:
		var node ClusterNode
		if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := h.manager.AddNode(&node); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(node)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleNodeByID 处理单个节点请求.
func (h *HTTPHandler) handleNodeByID(w http.ResponseWriter, r *http.Request) {
	// 提取节点 ID
	id := strings.TrimPrefix(r.URL.Path, "/api/cluster/nodes/")
	if id == "" {
		http.Error(w, "Node ID is required", http.StatusBadRequest)
		return
	}

	// 处理子路径
	if strings.Contains(id, "/") {
		parts := strings.SplitN(id, "/", 2)
		nodeID := parts[0]
		action := parts[1]

		switch action {
		case "promote":
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if err := h.manager.PromoteNode(nodeID); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "promoted", "node_id": nodeID})
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		node, err := h.manager.GetNode(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(node)

	case http.MethodDelete:
		if err := h.manager.RemoveNode(id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "removed", "node_id": id})

	case http.MethodPut:
		var update struct {
			Status NodeStatus `json:"status"`
			UsedGB int        `json:"used_gb"`
		}
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if update.Status != "" {
			if err := h.manager.UpdateNodeStatus(id, update.Status); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		if update.UsedGB > 0 {
			if err := h.manager.UpdateNodeMetrics(id, update.UsedGB); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "updated", "node_id": id})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMetrics 处理指标请求.
func (h *HTTPHandler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metrics := h.manager.GetMetrics()
	json.NewEncoder(w).Encode(metrics)
}

// handleTasks 处理任务请求.
func (h *HTTPHandler) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tasks := h.manager.GetTasks()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tasks": tasks,
			"total": len(tasks),
		})

	case http.MethodPost:
		var task ClusterTask
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := h.manager.SubmitTask(task); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(task)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePrimary 处理主节点请求.
func (h *HTTPHandler) handlePrimary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	primary := h.manager.GetPrimaryNode()
	if primary == nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "no_primary"})
		return
	}

	json.NewEncoder(w).Encode(primary)
}
