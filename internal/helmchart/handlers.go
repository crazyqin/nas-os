package helmchart

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handlers 提供 Helm Chart HTTP API
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建 Helm Chart API 处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/helm/repos", h.handleRepos)
	mux.HandleFunc("/api/v1/helm/repos/", h.handleRepoByName)
	mux.HandleFunc("/api/v1/helm/charts", h.handleCharts)
	mux.HandleFunc("/api/v1/helm/installed", h.handleInstalled)
	mux.HandleFunc("/api/v1/helm/installed/", h.handleInstalledByName)
	mux.HandleFunc("/api/v1/helm/stats", h.handleStats)
}

func (h *Handlers) handleRepos(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listRepos(w, r)
	case http.MethodPost:
		h.addRepo(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handlers) handleRepoByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/helm/repos/")

	switch r.Method {
	case http.MethodDelete:
		h.removeRepo(w, name)
	case http.MethodPost:
		if strings.HasSuffix(name, "/sync") {
			repoName := strings.TrimSuffix(name, "/sync")
			h.syncRepo(w, repoName)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handlers) handleCharts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	keyword := r.URL.Query().Get("q")
	charts := h.manager.SearchChart(keyword)
	writeJSON(w, charts)
}

func (h *Handlers) handleInstalled(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listInstalled(w, r)
	case http.MethodPost:
		h.installChart(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handlers) handleInstalledByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/helm/installed/")

	switch r.Method {
	case http.MethodGet:
		h.getInstalled(w, name)
	case http.MethodDelete:
		h.uninstallChart(w, name)
	case http.MethodPut:
		h.upgradeChart(w, r, name)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.manager.GetStats()
	writeJSON(w, stats)
}

func (h *Handlers) listRepos(w http.ResponseWriter, _ *http.Request) {
	repos := h.manager.ListRepos()
	writeJSON(w, repos)
}

type addRepoRequest struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

func (h *Handlers) addRepo(w http.ResponseWriter, r *http.Request) {
	var req addRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	repo, err := h.manager.AddRepo(req.Name, req.URL, req.Description)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, repo)
}

func (h *Handlers) removeRepo(w http.ResponseWriter, name string) {
	if err := h.manager.RemoveRepo(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) syncRepo(w http.ResponseWriter, name string) {
	if err := h.manager.SyncRepo(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]string{"status": "syncing"})
}

type installChartRequest struct {
	Name      string            `json:"name"`
	Chart     string            `json:"chart"`
	Version   string            `json:"version"`
	Namespace string            `json:"namespace"`
	Values    map[string]string `json:"values"`
}

func (h *Handlers) installChart(w http.ResponseWriter, r *http.Request) {
	var req installChartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	chart, err := h.manager.InstallChart(req.Name, req.Chart, req.Version, req.Namespace, req.Values)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, chart)
}

func (h *Handlers) listInstalled(w http.ResponseWriter, _ *http.Request) {
	installed := h.manager.ListInstalled()
	writeJSON(w, installed)
}

func (h *Handlers) getInstalled(w http.ResponseWriter, name string) {
	chart, err := h.manager.GetInstalled(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, chart)
}

func (h *Handlers) uninstallChart(w http.ResponseWriter, name string) {
	if err := h.manager.UninstallChart(name); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type upgradeChartRequest struct {
	Version string            `json:"version"`
	Values  map[string]string `json:"values"`
}

func (h *Handlers) upgradeChart(w http.ResponseWriter, r *http.Request, name string) {
	var req upgradeChartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	chart, err := h.manager.UpgradeChart(name, req.Version, req.Values)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, chart)
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
