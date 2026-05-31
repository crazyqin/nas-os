package virtualdesktop

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP处理器
type Handler struct {
	manager *VirtualDesktopManager
}

// NewHandler 创建处理器
func NewHandler(manager *VirtualDesktopManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/vdi/desktops", h.handleDesktops)
	mux.HandleFunc("/api/vdi/desktops/", h.handleDesktop)
	mux.HandleFunc("/api/vdi/templates", h.handleTemplates)
	mux.HandleFunc("/api/vdi/snapshots", h.handleSnapshots)
}

func (h *Handler) handleDesktops(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		owner := r.URL.Query().Get("owner")
		desktops := h.manager.ListDesktops(r.Context(), owner)
		writeJSON(w, desktops)
	case http.MethodPost:
		var desktop VirtualDesktop
		if err := json.NewDecoder(r.Body).Decode(&desktop); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := h.manager.CreateDesktop(r.Context(), &desktop); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, desktop)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleDesktop(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/vdi/desktops/"):]
	if id == "" {
		http.Error(w, "Desktop ID required", http.StatusBadRequest)
		return
	}
	
	switch r.Method {
	case http.MethodGet:
		desktop, err := h.manager.GetDesktop(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, desktop)
	case http.MethodDelete:
		if err := h.manager.DeleteDesktop(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	templates := h.manager.ListTemplates(r.Context())
	writeJSON(w, templates)
}

func (h *Handler) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	desktopID := r.URL.Query().Get("desktop_id")
	snapshots := h.manager.ListSnapshots(r.Context(), desktopID)
	writeJSON(w, snapshots)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
