package capacityforecast

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// Handlers HTTP 处理器.
type Handlers struct {
	mgr *Manager
}

// NewHandlers 创建 HTTP 处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/forecast", h.handleForecast)
	mux.HandleFunc(prefix+"/growth", h.handleGrowth)
	mux.HandleFunc(prefix+"/alerts", h.handleAlerts)
	mux.HandleFunc(prefix+"/alerts/dismiss", h.handleDismissAlert)
	mux.HandleFunc(prefix+"/alerts/acknowledge", h.handleAcknowledgeAlert)
	mux.HandleFunc(prefix+"/whatif", h.handleWhatIf)
	mux.HandleFunc(prefix+"/whatif/list", h.handleListScenarios)
	mux.HandleFunc(prefix+"/whatif/get", h.handleGetScenario)
	mux.HandleFunc(prefix+"/expansion", h.handleExpansion)
	mux.HandleFunc(prefix+"/snapshots", h.handleSnapshots)
	mux.HandleFunc(prefix+"/snapshots/add", h.handleAddSnapshot)
	mux.HandleFunc(prefix+"/stats", h.handleStats)
	mux.HandleFunc(prefix+"/config", h.handleConfig)
}

type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

// handleForecast 容量预测.
func (h *Handlers) handleForecast(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	daysStr := r.URL.Query().Get("days")
	days := 90 // 默认 90 天
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	result, err := h.mgr.PredictCapacity(days)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    result,
	})
}

// handleGrowth 增长率分析.
func (h *Handlers) handleGrowth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	result, err := h.mgr.AnalyzeGrowth()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    result,
	})
}

// handleAlerts 获取告警.
func (h *Handlers) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	showDismissed := r.URL.Query().Get("dismissed") == "true"
	alerts := h.mgr.CheckAlerts(showDismissed)

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    alerts,
	})
}

// handleDismissAlert 忽略告警.
func (h *Handlers) handleDismissAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	var req struct {
		AlertID string `json:"alert_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "无效的请求"})
		return
	}

	if err := h.mgr.DismissAlert(req.AlertID); err != nil {
		writeJSON(w, http.StatusNotFound, response{Code: 404, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "告警已忽略",
	})
}

// handleAcknowledgeAlert 确认告警.
func (h *Handlers) handleAcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	var req struct {
		AlertID string `json:"alert_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "无效的请求"})
		return
	}

	if err := h.mgr.AcknowledgeAlert(req.AlertID); err != nil {
		writeJSON(w, http.StatusNotFound, response{Code: 404, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "告警已确认",
	})
}

// handleWhatIf What-If 模拟.
func (h *Handlers) handleWhatIf(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	var scenario WhatIfScenario
	if err := json.NewDecoder(r.Body).Decode(&scenario); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "无效的请求"})
		return
	}

	if scenario.Name == "" {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "场景名称不能为空"})
		return
	}

	result, err := h.mgr.SimulateWhatIf(scenario)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "模拟完成",
		Data:    result,
	})
}

// handleListScenarios 列出模拟场景.
func (h *Handlers) handleListScenarios(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	scenarios := h.mgr.ListScenarios()
	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    scenarios,
	})
}

// handleGetScenario 获取模拟场景.
func (h *Handlers) handleGetScenario(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "id 参数不能为空"})
		return
	}

	scenario, err := h.mgr.GetScenario(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, response{Code: 404, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    scenario,
	})
}

// handleExpansion 扩容建议.
func (h *Handlers) handleExpansion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	result, err := h.mgr.RecommendExpansion()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    result,
	})
}

// handleSnapshots 获取快照.
func (h *Handlers) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	hoursStr := r.URL.Query().Get("hours")
	hours := 24
	if hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 {
			hours = h
		}
	}

	duration := time.Duration(hours) * time.Hour
	snapshots := h.mgr.GetSnapshots(duration)

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    snapshots,
	})
}

// handleAddSnapshot 添加快照.
func (h *Handlers) handleAddSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	var snapshot CapacitySnapshot
	if err := json.NewDecoder(r.Body).Decode(&snapshot); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "无效的请求"})
		return
	}

	if snapshot.TotalBytes <= 0 {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "total_bytes 必须大于 0"})
		return
	}

	h.mgr.AddSnapshot(snapshot)

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "快照已添加",
	})
}

// handleStats 获取统计.
func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	stats := h.mgr.GetStats()
	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    stats,
	})
}

// handleConfig 管理配置.
func (h *Handlers) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config := h.mgr.GetConfig()
		writeJSON(w, http.StatusOK, response{
			Code:    200,
			Message: "ok",
			Data:    config,
		})

	case http.MethodPut:
		var config ForecastConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "无效的请求"})
			return
		}
		h.mgr.UpdateConfig(config)
		writeJSON(w, http.StatusOK, response{
			Code:    200,
			Message: "配置已更新",
		})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
	}
}
