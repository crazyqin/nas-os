package dedupadvisor

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP处理器
type Handler struct {
	advisor *Advisor
}

// NewHandler 创建处理器
func NewHandler(a *Advisor) *Handler {
	return &Handler{advisor: a}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/dedup-advisor/scan", h.handleScan)
	mux.HandleFunc("/api/v1/dedup-advisor/results/", h.handleResults)
	mux.HandleFunc("/api/v1/dedup-advisor/report", h.handleReport)
}

func (h *Handler) handleScan(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req struct {
			Paths []string `json:"paths"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		result, err := h.advisor.Scan(req.Paths)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, result)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handleResults(w http.ResponseWriter, r *http.Request) {
	scanID := r.URL.Path[len("/api/v1/dedup-advisor/results/"):]
	if scanID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing scan id"})
		return
	}

	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	result, err := h.advisor.GetScanResult(scanID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	lastScan := h.advisor.GetLastScan()
	if lastScan == nil {
		writeJSON(w, http.StatusOK, map[string]string{"message": "尚未进行扫描"})
		return
	}

	writeJSON(w, http.StatusOK, lastScan)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
