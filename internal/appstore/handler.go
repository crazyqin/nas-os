package appstore

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP处理器
type Handler struct {
	store *AppStore
}

// NewHandler 创建处理器
func NewHandler(store *AppStore) *Handler {
	return &Handler{store: store}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/apps", h.handleApps)
	mux.HandleFunc("/api/apps/", h.handleApp)
	mux.HandleFunc("/api/apps/search", h.handleSearch)
}

func (h *Handler) handleApps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	category := AppCategory(r.URL.Query().Get("category"))
	apps := h.store.ListApps(r.Context(), category)
	writeJSON(w, apps)
}

func (h *Handler) handleApp(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/apps/"):]
	if id == "" {
		http.Error(w, "App ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		app, err := h.store.GetApp(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, app)
	case http.MethodPost:
		// 安装应用
		if err := h.store.InstallApp(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	case http.MethodDelete:
		// 卸载应用
		if err := h.store.UninstallApp(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	apps := h.store.SearchApps(r.Context(), query)
	writeJSON(w, apps)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
