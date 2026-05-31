package diskhibernate

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(m *Manager) *Handler {
	return &Handler{manager: m}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/disk-hibernate/disks", h.handleDisks)
	mux.HandleFunc("/api/v1/disk-hibernate/disks/", h.handleDiskByID)
	mux.HandleFunc("/api/v1/disk-hibernate/patterns/", h.handlePatterns)
	mux.HandleFunc("/api/v1/disk-hibernate/report", h.handleReport)
}

func (h *Handler) handleDisks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		disks := h.manager.ListDisks()
		writeJSON(w, http.StatusOK, disks)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handleDiskByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/disk-hibernate/disks/"):]
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing disk id"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		disk, err := h.manager.GetDisk(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, disk)
	case http.MethodPost:
		var req struct {
			Action string `json:"action"` // hibernate, wake
			State  string `json:"state"`  // standby, sleep, spindown
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		switch req.Action {
		case "hibernate":
			if err := h.manager.HibernateDisk(id, DiskState(req.State)); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		case "wake":
			if err := h.manager.WakeDisk(id); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid action"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handlePatterns(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/disk-hibernate/patterns/"):]
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing disk id"})
		return
	}

	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	pattern, err := h.manager.GetPattern(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, pattern)
}

func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	report := h.manager.GetHibernateReport()
	writeJSON(w, http.StatusOK, report)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
