package smartwearleveling

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP处理器.
type Handler struct {
	manager *SmartWearLevelingManager
}

// NewHandler 创建处理器.
func NewHandler(manager *SmartWearLevelingManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/smartwearleveling/ssds", h.handleListSSDs)
	mux.HandleFunc("/api/smartwearleveling/ssd/get", h.handleGetSSD)
	mux.HandleFunc("/api/smartwearleveling/ssd/register", h.handleRegisterSSD)
	mux.HandleFunc("/api/smartwearleveling/ssd/unregister", h.handleUnregisterSSD)
	mux.HandleFunc("/api/smartwearleveling/ssd/update", h.handleUpdateSSD)
	mux.HandleFunc("/api/smartwearleveling/ssd/predict", h.handlePredictWear)
	mux.HandleFunc("/api/smartwearleveling/stats", h.handleGetStats)
	mux.HandleFunc("/api/smartwearleveling/alerts", h.handleGetAlerts)
	mux.HandleFunc("/api/smartwearleveling/alert/resolve", h.handleResolveAlert)
	mux.HandleFunc("/api/smartwearleveling/alert-config", h.handleAlertConfig)
	mux.HandleFunc("/api/smartwearleveling/policies", h.handlePolicies)
	mux.HandleFunc("/api/smartwearleveling/policy/add", h.handleAddPolicy)
	mux.HandleFunc("/api/smartwearleveling/jobs", h.handleGetJobs)
	mux.HandleFunc("/api/smartwearleveling/evaluate", h.handleEvaluate)
}

func (h *Handler) handleListSSDs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.ListSSDs()})
}

func (h *Handler) handleGetSSD(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "缺少id参数"})
		return
	}
	ssd, err := h.manager.GetSSD(id)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 404, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": ssd})
}

func (h *Handler) handleRegisterSSD(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var ssd SSDInfo
	if err := json.NewDecoder(r.Body).Decode(&ssd); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.RegisterSSD(&ssd); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": ssd})
}

func (h *Handler) handleUnregisterSSD(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.UnregisterSSD(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *Handler) handleUpdateSSD(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID    string  `json:"id"`
		Stats SSDInfo `json:"stats"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.UpdateSSDStats(req.ID, &req.Stats); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *Handler) handlePredictWear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "缺少id参数"})
		return
	}
	prediction, err := h.manager.PredictWear(id)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": prediction})
}

func (h *Handler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.GetStats()})
}

func (h *Handler) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resolved := r.URL.Query().Get("resolved") == "true"
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.GetAlerts(resolved)})
}

func (h *Handler) handleResolveAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.ResolveAlert(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *Handler) handleAlertConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.GetAlertConfig()})
	case http.MethodPost:
		var cfg AlertConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
			return
		}
		h.manager.UpdateAlertConfig(&cfg)
		writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handlePolicies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.GetPolicies()})
}

func (h *Handler) handleAddPolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var policy MigrationPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	h.manager.AddPolicy(&policy)
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": policy})
}

func (h *Handler) handleGetJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.GetJobs()})
}

func (h *Handler) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobs := h.manager.EvaluateMigrations()
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": jobs})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
