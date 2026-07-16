package firewall

import (
	"encoding/json"
	"net/http"
)

// Handler 防火墙 HTTP 处理器.
type Handler struct {
	mgr *Manager
}

// NewHandler 创建处理器.
func NewHandler(mgr *Manager) *Handler {
	return &Handler{mgr: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/firewall/rules", h.handleRules)
	mux.HandleFunc("/api/firewall/rules/", h.handleRuleByID)
	mux.HandleFunc("/api/firewall/config", h.handleConfig)
	mux.HandleFunc("/api/firewall/stats", h.handleStats)
	mux.HandleFunc("/api/firewall/traffic-log", h.handleTrafficLog)
	mux.HandleFunc("/api/firewall/zones", h.handleZones)
}

func (h *Handler) handleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rules := h.mgr.ListRules()
		writeJSON(w, http.StatusOK, rules)
	case http.MethodPost:
		var rule Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.mgr.AddRule(&rule); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, rule)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleRuleByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/firewall/rules/"):]
	if id == "" {
		writeErr(w, http.StatusBadRequest, "missing rule id")
		return
	}

	switch r.Method {
	case http.MethodGet:
		rule, err := h.mgr.GetRule(id)
		if err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, rule)
	case http.MethodPut:
		var rule Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.mgr.UpdateRule(id, &rule); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, rule)
	case http.MethodDelete:
		if err := h.mgr.DeleteRule(id); err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.mgr.GetConfig())
	case http.MethodPut:
		var cfg FirewallConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		h.mgr.UpdateConfig(&cfg)
		writeJSON(w, http.StatusOK, cfg)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, h.mgr.GetStats())
}

func (h *Handler) handleTrafficLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	logs := h.mgr.GetTrafficLog(100)
	writeJSON(w, http.StatusOK, logs)
}

func (h *Handler) handleZones(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.mgr.ListZones())
	case http.MethodPost:
		var zone Zone
		if err := json.NewDecoder(r.Body).Decode(&zone); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		h.mgr.AddZone(&zone)
		writeJSON(w, http.StatusCreated, zone)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
