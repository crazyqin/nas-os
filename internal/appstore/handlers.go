package appstore

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler handles HTTP requests for app store operations
type Handler struct {
	manager *Manager
}

// NewHandler creates a new app store handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/apps", h.handleApps)
	mux.HandleFunc("/api/v1/apps/", h.handleAppByID)
	mux.HandleFunc("/api/v1/apps/search", h.handleSearch)
	mux.HandleFunc("/api/v1/apps/categories", h.handleCategories)
	mux.HandleFunc("/api/v1/apps/stats", h.handleStats)
	mux.HandleFunc("/api/v1/installed", h.handleInstalled)
	mux.HandleFunc("/api/v1/installed/", h.handleInstalledByID)
	mux.HandleFunc("/api/v1/install", h.handleInstall)
	mux.HandleFunc("/api/v1/install/", h.handleInstallStatus)
}

func (h *Handler) handleApps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	page := 1
	pageSize := 20

	req := &AppSearchRequest{
		Page:     page,
		PageSize: pageSize,
	}

	result := h.manager.SearchApps(req)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleAppByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/apps/")
	if id == "" {
		http.Error(w, "Missing app ID", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	app, ok := h.manager.GetApp(id)
	if !ok {
		http.Error(w, "App not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, app)
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AppSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	result := h.manager.SearchApps(&req)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	categories := h.manager.GetCategories()
	writeJSON(w, http.StatusOK, categories)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.manager.GetStats()
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) handleInstalled(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	apps := h.manager.GetInstalledApps()
	writeJSON(w, http.StatusOK, apps)
}

func (h *Handler) handleInstalledByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/installed/")
	if id == "" {
		http.Error(w, "Missing app ID", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.manager.UninstallApp(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "uninstalled"})
}

func (h *Handler) handleInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req InstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	status, err := h.manager.InstallApp(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusAccepted, status)
}

func (h *Handler) handleInstallStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/install/")
	if id == "" {
		http.Error(w, "Missing install ID", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status, ok := h.manager.GetInstallStatus(id)
	if !ok {
		http.Error(w, "Install not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
