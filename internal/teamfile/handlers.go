package teamfile

import (
	"encoding/json"
	"net/http"
)

// APIHandler HTTP API处理器
type APIHandler struct {
	manager *TeamFileManager
}

// NewAPIHandler 创建API处理器
func NewAPIHandler(manager *TeamFileManager) *APIHandler {
	return &APIHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/teamfile/folders", h.handleFolders)
	mux.HandleFunc(prefix+"/teamfile/members", h.handleMembers)
	mux.HandleFunc(prefix+"/teamfile/share", h.handleShare)
	mux.HandleFunc(prefix+"/teamfile/audit", h.handleAudit)
}

func (h *APIHandler) handleFolders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.manager.ListFolders())
	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			OwnerTeam   string `json:"owner_team"`
			Path        string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		folder, err := h.manager.CreateFolder(req.Name, req.Description, req.OwnerTeam, req.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, folder)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *APIHandler) handleMembers(w http.ResponseWriter, r *http.Request) {
	folderID := r.URL.Query().Get("folder_id")
	if folderID == "" {
		http.Error(w, "folder_id required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		members, err := h.manager.GetMembers(folderID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, members)
	case http.MethodPost:
		var req struct {
			UserID     string           `json:"user_id"`
			Role       MemberRole       `json:"role"`
			Permission FolderPermission `json:"permission"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := h.manager.AddMember(folderID, req.UserID, req.Role, req.Permission); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"status": "added"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *APIHandler) handleShare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		FolderID  string           `json:"folder_id"`
		CreatedBy string           `json:"created_by"`
		Permission FolderPermission `json:"permission"`
		ExpiryDays int             `json:"expiry_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.ExpiryDays == 0 {
		req.ExpiryDays = 30
	}
	link, err := h.manager.CreateShareLink(req.FolderID, req.CreatedBy, req.Permission, req.ExpiryDays)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

func (h *APIHandler) handleAudit(w http.ResponseWriter, r *http.Request) {
	folderID := r.URL.Query().Get("folder_id")
	writeJSON(w, http.StatusOK, h.manager.GetAuditLog(folderID))
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
