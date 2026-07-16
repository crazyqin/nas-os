// Package consensus HTTP API 处理器
package consensus

import (
	"encoding/json"
	"io"
	"net/http"
)

// HTTPHandler 共识引擎 HTTP 处理器.
type HTTPHandler struct {
	engine *ConsensusEngine
}

// NewHTTPHandler 创建 HTTP 处理器.
func NewHTTPHandler(engine *ConsensusEngine) *HTTPHandler {
	return &HTTPHandler{engine: engine}
}

// RegisterRoutes 注册路由.
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/consensus/status", h.handleStatus)
	mux.HandleFunc("/api/consensus/members", h.handleMembers)
	mux.HandleFunc("/api/consensus/propose", h.handlePropose)
	mux.HandleFunc("/api/consensus/members/add", h.handleAddMember)
	mux.HandleFunc("/api/consensus/members/remove", h.handleRemoveMember)
}

func (h *HTTPHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.engine.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *HTTPHandler) handleMembers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	members := h.engine.GetMembers()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}

func (h *HTTPHandler) handlePropose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "读取请求体失败", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	result, err := h.engine.Propose(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *HTTPHandler) handleAddMember(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var member ClusterMember
	if err := json.NewDecoder(r.Body).Decode(&member); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if err := h.engine.AddMember(&member); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "added", "id": member.ID})
}

func (h *HTTPHandler) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		NodeID string `json:"node_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if err := h.engine.RemoveMember(req.NodeID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "removed", "id": req.NodeID})
}
