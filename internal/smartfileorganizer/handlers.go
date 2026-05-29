package smartfileorganizer

import (
	"encoding/json"
	"net/http"
)

// Handler provides HTTP handlers for smart file organization.
type Handler struct {
	organizer *Organizer
}

// NewHandler creates a new organizer HTTP handler.
func NewHandler(o *Organizer) *Handler {
	return &Handler{organizer: o}
}

// RegisterRoutes registers organizer API routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/fileorganizer/scan", h.handleScan)
	mux.HandleFunc("/api/v1/fileorganizer/duplicates", h.handleDuplicates)
	mux.HandleFunc("/api/v1/fileorganizer/organize", h.handleOrganize)
	mux.HandleFunc("/api/v1/fileorganizer/stats", h.handleStats)
	mux.HandleFunc("/api/v1/fileorganizer/rules", h.handleRules)
	mux.HandleFunc("/api/v1/fileorganizer/category", h.handleCategory)
}

func (h *Handler) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	report, err := h.organizer.Scan()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *Handler) handleDuplicates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	groups := h.organizer.FindDuplicates()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"groups": groups,
		"total":  len(groups),
	})
}

func (h *Handler) handleOrganize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	dryRun := r.URL.Query().Get("dryRun") == "true"
	report, err := h.organizer.Organize(dryRun)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, h.organizer.GetStats())
}

func (h *Handler) handleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.organizer.GetRules())
	case http.MethodPost:
		var rule OrganizationRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		h.organizer.AddRule(rule)
		writeJSON(w, http.StatusCreated, map[string]string{"status": "created", "id": rule.ID})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if h.organizer.RemoveRule(id) {
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		} else {
			http.Error(w, "rule not found", http.StatusNotFound)
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cat := FileCategory(r.URL.Query().Get("type"))
	entries := h.organizer.GetByCategory(cat)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"category": cat,
		"files":    entries,
		"count":    len(entries),
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
