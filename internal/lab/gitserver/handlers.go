package gitserver

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler API 处理器.
type Handler struct {
	svc *Service
}

// NewHandler 创建 API 处理器.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// HandleRepos GET /api/v1/gitserver/repos.
func (h *Handler) HandleRepos(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	repos := h.svc.ListRepos(owner)
	writeJSON(w, repos)
}

// HandleCreateRepo POST /api/v1/gitserver/repos.
func (h *Handler) HandleCreateRepo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req CreateRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	repo, err := h.svc.CreateRepo(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, repo)
}

// HandleRepo GET/DELETE /api/v1/gitserver/repos/{id}.
func (h *Handler) HandleRepo(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/v1/gitserver/repos/")
	switch r.Method {
	case http.MethodGet:
		repo, err := h.svc.GetRepo(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, repo)
	case http.MethodDelete:
		if err := h.svc.DeleteRepo(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleBranches GET /api/v1/gitserver/repos/{id}/branches.
func (h *Handler) HandleBranches(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/v1/gitserver/repos/")
	if idx := strings.Index(id, "/branches"); idx > 0 {
		id = id[:idx]
	}
	branches, err := h.svc.ListBranches(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, branches)
}

// HandleCollaborators GET/POST/DELETE /api/v1/gitserver/repos/{id}/collaborators.
func (h *Handler) HandleCollaborators(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/gitserver/repos/"), "/")
	if len(parts) < 2 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	repoID := parts[0]

	switch r.Method {
	case http.MethodGet:
		svc := h.svc
		svc.mu.RLock()
		collabs := svc.collabs[repoID]
		svc.mu.RUnlock()
		writeJSON(w, collabs)
	case http.MethodPost:
		var c Collaborator
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := h.svc.AddCollaborator(repoID, c.Username, c.Role); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	case http.MethodDelete:
		username := r.URL.Query().Get("username")
		if err := h.svc.RemoveCollaborator(repoID, username); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleWebhooks GET/POST /api/v1/gitserver/repos/{id}/webhooks.
func (h *Handler) HandleWebhooks(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/gitserver/repos/"), "/")
	if len(parts) < 2 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	repoID := parts[0]

	switch r.Method {
	case http.MethodGet:
		hooks := h.svc.ListWebhooks(repoID)
		writeJSON(w, hooks)
	case http.MethodPost:
		var wh WebHook
		if err := json.NewDecoder(r.Body).Decode(&wh); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		hook, err := h.svc.CreateWebhook(repoID, wh.URL, wh.Secret, wh.Events)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, hook)
	}
}

// HandleStats GET /api/v1/gitserver/repos/{id}/stats.
func (h *Handler) HandleStats(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/gitserver/repos/"), "/")
	if len(parts) < 2 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	repoID := parts[0]

	stats, err := h.svc.GetRepoStats(repoID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, stats)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func extractID(path, prefix string) string {
	return strings.TrimPrefix(path, prefix)
}
