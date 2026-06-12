package nfsv4

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ACLHandlers 提供 NFSv4 ACL HTTP API
type ACLHandlers struct {
	manager *ACLManager
}

// NewACLHandlers 创建 ACL API 处理器
func NewACLHandlers(manager *ACLManager) *ACLHandlers {
	return &ACLHandlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *ACLHandlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/nfs/acl", h.handleACL)
	mux.HandleFunc("/api/v1/nfs/acl/check", h.handleCheckPermission)
	mux.HandleFunc("/api/v1/nfs/acl/stats", h.handleStats)
	mux.HandleFunc("/api/v1/nfs/acl/ace/", h.handleACE)
}

func (h *ACLHandlers) handleACL(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listACLs(w, r)
	case http.MethodPost:
		h.setACL(w, r)
	case http.MethodDelete:
		h.deleteACL(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ACLHandlers) handleACE(w http.ResponseWriter, r *http.Request) {
	aceID := strings.TrimPrefix(r.URL.Path, "/api/v1/nfs/acl/ace/")

	switch r.Method {
	case http.MethodPut:
		h.updateACE(w, r, aceID)
	case http.MethodDelete:
		h.removeACE(w, aceID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ACLHandlers) handleCheckPermission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Path      string `json:"path"`
		Principal string `json:"principal"`
		Permission int   `json:"permission"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	allowed, err := h.manager.CheckPermission(req.Path, req.Principal, ACLPermission(req.Permission))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"allowed": allowed,
		"path":    req.Path,
		"principal": req.Principal,
	})
}

func (h *ACLHandlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.manager.GetStats()
	writeJSON(w, stats)
}

func (h *ACLHandlers) listACLs(w http.ResponseWriter, _ *http.Request) {
	acls := h.manager.ListACLs()
	writeJSON(w, acls)
}

type setACLRequest struct {
	Path   string      `json:"path"`
	Owner  string      `json:"owner"`
	Group  string      `json:"group"`
	ACEs   []*NFSv4ACE `json:"aces"`
}

func (h *ACLHandlers) setACL(w http.ResponseWriter, r *http.Request) {
	var req setACLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if err := h.manager.SetACL(req.Path, req.Owner, req.Group, req.ACEs); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]string{"status": "created"})
}

func (h *ACLHandlers) deleteACL(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "需要指定路径", http.StatusBadRequest)
		return
	}

	if err := h.manager.DeleteACL(path); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type updateACERequest struct {
	Type       ACLType        `json:"type"`
	Flags      ACLFlag        `json:"flags"`
	Principal  string         `json:"principal"`
	Permissions ACLPermission `json:"permissions"`
}

func (h *ACLHandlers) updateACE(w http.ResponseWriter, r *http.Request, aceID string) {
	var req updateACERequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	err := h.manager.UpdateACE(aceID, func(ace *NFSv4ACE) {
		if req.Type != "" {
			ace.Type = req.Type
		}
		if req.Flags != 0 {
			ace.Flags = req.Flags
		}
		if req.Principal != "" {
			ace.Principal = req.Principal
		}
		if req.Permissions != 0 {
			ace.Permissions = req.Permissions
		}
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, map[string]string{"status": "updated"})
}

func (h *ACLHandlers) removeACE(w http.ResponseWriter, aceID string) {
	if err := h.manager.RemoveACE(aceID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
