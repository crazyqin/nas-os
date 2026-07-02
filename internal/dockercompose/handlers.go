package dockercompose

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handlers 提供 Docker Compose HTTP API.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建 Compose API 处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/compose/projects", h.handleProjects)
	mux.HandleFunc("/api/v1/compose/projects/", h.handleProjectByName)
}

func (h *Handlers) handleProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listProjects(w, r)
	case http.MethodPost:
		h.createProject(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handlers) handleProjectByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/compose/projects/")

	// 处理子路径
	parts := strings.SplitN(name, "/", 2)
	projectName := parts[0]

	if len(parts) > 1 {
		action := parts[1]
		switch action {
		case "start":
			h.startProject(w, projectName)
		case "stop":
			h.stopProject(w, projectName)
		case "restart":
			h.restartProject(w, projectName)
		case "stats":
			h.getProjectStats(w, projectName)
		default:
			http.Error(w, "Unknown action", http.StatusBadRequest)
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getProject(w, projectName)
	case http.MethodDelete:
		h.deleteProject(w, projectName)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handlers) listProjects(w http.ResponseWriter, _ *http.Request) {
	projects := h.manager.ListProjects()
	writeJSON(w, projects)
}

type createProjectRequest struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (h *Handlers) createProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	project, err := h.manager.CreateProject(req.Name, req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, project)
}

func (h *Handlers) getProject(w http.ResponseWriter, name string) {
	project, err := h.manager.GetProject(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, project)
}

func (h *Handlers) deleteProject(w http.ResponseWriter, name string) {
	if err := h.manager.DeleteProject(name); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) startProject(w http.ResponseWriter, name string) {
	if err := h.manager.StartProject(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]string{"status": "started"})
}

func (h *Handlers) stopProject(w http.ResponseWriter, name string) {
	if err := h.manager.StopProject(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]string{"status": "stopped"})
}

func (h *Handlers) restartProject(w http.ResponseWriter, name string) {
	if err := h.manager.RestartProject(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]string{"status": "restarting"})
}

func (h *Handlers) getProjectStats(w http.ResponseWriter, name string) {
	stats, err := h.manager.GetProjectStats(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, stats)
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
