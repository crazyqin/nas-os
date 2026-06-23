package ransomware_honeypot

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler API 处理器.
type Handler struct {
	manager *HoneypotManager
}

// NewHandler 创建处理器.
func NewHandler(manager *HoneypotManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/ransomware/honeypot/create", h.handleCreate)
	mux.HandleFunc("/api/v1/ransomware/honeypot/list", h.handleList)
	mux.HandleFunc("/api/v1/ransomware/honeypot/", h.handleGet)
	mux.HandleFunc("/api/v1/ransomware/scan", h.handleScan)
	mux.HandleFunc("/api/v1/ransomware/alerts", h.handleAlerts)
	mux.HandleFunc("/api/v1/ransomware/alerts/", h.handleRespondAlert)
}

// handleCreate 处理创建蜜罐请求.
func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req CreateHoneypotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}

	hp, err := h.manager.Create(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, hp)
}

// handleList 处理列出蜜罐请求.
func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	honeypots := h.manager.List()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"honeypots": honeypots,
		"total":     len(honeypots),
	})
}

// handleGet 处理获取单个蜜罐请求.
func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	// 从路径提取 ID
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/ransomware/honeypot/")
	if path == "" || path == "list" {
		h.handleList(w, r)
		return
	}

	hp, err := h.manager.Get(path)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, hp)
}

// handleScan 处理扫描请求.
func (h *Handler) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req struct {
		HoneypotID string `json:"honeypot_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}

	result, err := h.manager.Scan(req.HoneypotID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleAlerts 处理告警列表请求.
func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	honeypotID := r.URL.Query().Get("honeypot_id")
	if honeypotID == "" {
		// 返回所有告警
		allAlerts := make([]*Alert, 0)
		for _, hp := range h.manager.List() {
			alerts := h.manager.GetAlerts(hp.ID)
			allAlerts = append(allAlerts, alerts...)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"alerts": allAlerts,
			"total":  len(allAlerts),
		})
		return
	}

	alerts := h.manager.GetAlerts(honeypotID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

// handleRespondAlert 处理响应告警请求.
func (h *Handler) handleRespondAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	// 从路径提取告警 ID
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/ransomware/alerts/")
	if !strings.HasSuffix(path, "/respond") {
		writeError(w, http.StatusBadRequest, "路径格式错误")
		return
	}
	alertID := strings.TrimSuffix(path, "/respond")

	var resp AlertResponse
	if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}

	if err := h.manager.RespondAlert(alertID, resp); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "告警已响应",
	})
}

// writeJSON 写入 JSON 响应.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
