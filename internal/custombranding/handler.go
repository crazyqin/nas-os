package custombranding

import (
	"encoding/json"
	"net/http"
)

// Handler 品牌定制引擎 HTTP 处理器
type Handler struct {
	engine *BrandingEngine
}

// NewHandler 创建处理器
func NewHandler(engine *BrandingEngine) *Handler {
	return &Handler{engine: engine}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/custombranding/themes", h.handleThemes)
	mux.HandleFunc("/api/v1/custombranding/config", h.handleConfig)
	mux.HandleFunc("/api/v1/custombranding/apply", h.handleApply)
	mux.HandleFunc("/api/v1/custombranding/assets", h.handleAssets)
	mux.HandleFunc("/api/v1/custombranding/export", h.handleExport)
	mux.HandleFunc("/api/v1/custombranding/import", h.handleImport)
}

func (h *Handler) handleThemes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.engine.GetThemes())
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		data, err := h.engine.ExportConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	case http.MethodPut:
		var updates BrandingConfig
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}
		if err := h.engine.UpdateConfig(&updates); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ThemeID string `json:"theme_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.engine.ApplyTheme(req.ThemeID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "applied"})
}

func (h *Handler) handleAssets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleExport(w http.ResponseWriter, r *http.Request) {
	data, err := h.engine.ExportConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=branding-config.json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (h *Handler) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Data []byte `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.engine.ImportConfig(req.Data); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "imported"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
