package storageforecast

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// Handlers HTTP 处理器
type Handlers struct {
	mgr *Manager
}

// NewHandlers 创建 HTTP 处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/pools", h.handlePools)
	mux.HandleFunc(prefix+"/pools/usage", h.handleUpdateUsage)
	mux.HandleFunc(prefix+"/forecast", h.handleForecast)
	mux.HandleFunc(prefix+"/forecast/all", h.handleAllForecasts)
	mux.HandleFunc(prefix+"/trends", h.handleTrends)
	mux.HandleFunc(prefix+"/expansion", h.handleExpansion)
	mux.HandleFunc(prefix+"/cost", h.handleCost)
	mux.HandleFunc(prefix+"/alerts", h.handleAlerts)
	mux.HandleFunc(prefix+"/alerts/dismiss", h.handleDismissAlert)
	mux.HandleFunc(prefix+"/snapshots", h.handleSnapshots)
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

// handlePools 管理存储池
func (h *Handlers) handlePools(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		pools := h.mgr.ListPools()
		writeJSON(w, http.StatusOK, response{
			Code:    200,
			Message: "ok",
			Data:    pools,
		})

	case http.MethodPost:
		var pool StoragePool
		if err := json.NewDecoder(r.Body).Decode(&pool); err != nil {
			writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "无效的请求"})
			return
		}

		if pool.ID == "" || pool.Name == "" {
			writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "ID 和名称不能为空"})
			return
		}

		h.mgr.RegisterPool(pool)
		writeJSON(w, http.StatusCreated, response{
			Code:    201,
			Message: "存储池注册成功",
		})

	case http.MethodDelete:
		poolID := r.URL.Query().Get("pool_id")
		if poolID == "" {
			writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "pool_id 参数不能为空"})
			return
		}

		if err := h.mgr.UnregisterPool(poolID); err != nil {
			writeJSON(w, http.StatusNotFound, response{Code: 404, Message: err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, response{
			Code:    200,
			Message: "存储池已注销",
		})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
	}
}

// handleUpdateUsage 更新使用量
func (h *Handlers) handleUpdateUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	var req struct {
		PoolID    string `json:"pool_id"`
		UsedBytes int64  `json:"used_bytes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "无效的请求"})
		return
	}

	if err := h.mgr.UpdatePoolUsage(req.PoolID, req.UsedBytes); err != nil {
		writeJSON(w, http.StatusNotFound, response{Code: 404, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "使用量已更新",
	})
}

// handleForecast 获取单个池预测
func (h *Handlers) handleForecast(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	poolID := r.URL.Query().Get("pool_id")
	if poolID == "" {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "pool_id 参数不能为空"})
		return
	}

	result, err := h.mgr.GetForecast(poolID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, response{Code: 404, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    result,
	})
}

// handleAllForecasts 获取所有预测
func (h *Handlers) handleAllForecasts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	results := h.mgr.GetAllForecasts()
	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    results,
	})
}

// handleTrends 获取趋势数据
func (h *Handlers) handleTrends(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	poolID := r.URL.Query().Get("pool_id")
	if poolID == "" {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "pool_id 参数不能为空"})
		return
	}

	granularity := TimeGranularity(r.URL.Query().Get("granularity"))
	if granularity == "" {
		granularity = GranularityDay
	}

	hoursStr := r.URL.Query().Get("hours")
	hours := 720 // 默认 30 天
	if hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 {
			hours = h
		}
	}

	duration := time.Duration(hours) * time.Hour
	series, err := h.mgr.GetTrendSeries(poolID, granularity, duration)
	if err != nil {
		writeJSON(w, http.StatusNotFound, response{Code: 404, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    series,
	})
}

// handleExpansion 获取扩容建议
func (h *Handlers) handleExpansion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	poolID := r.URL.Query().Get("pool_id")
	if poolID == "" {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "pool_id 参数不能为空"})
		return
	}

	rec, err := h.mgr.GetExpansionRecommendation(poolID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, response{Code: 404, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    rec,
	})
}

// handleCost 获取成本估算
func (h *Handlers) handleCost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	poolID := r.URL.Query().Get("pool_id")
	if poolID == "" {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "pool_id 参数不能为空"})
		return
	}

	estimate, err := h.mgr.GetCostEstimate(poolID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, response{Code: 404, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    estimate,
	})
}

// handleAlerts 管理告警
func (h *Handlers) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	showDismissed := r.URL.Query().Get("dismissed") == "true"
	alerts := h.mgr.GetAlerts(showDismissed)

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    alerts,
	})
}

// handleDismissAlert 忽略告警
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

// handleSnapshots 获取快照
func (h *Handlers) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	poolID := r.URL.Query().Get("pool_id")
	if poolID == "" {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "pool_id 参数不能为空"})
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
	snapshots := h.mgr.GetSnapshots(poolID, duration)

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    snapshots,
	})
}

// handleStats 获取统计
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

// handleConfig 管理配置
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

// GetForecastSummary 获取预测摘要（用于 Dashboard）
func (h *Handlers) GetForecastSummary() map[string]interface{} {
	forecasts := h.mgr.GetAllForecasts()

	summary := map[string]interface{}{
		"total_pools": len(forecasts),
		"warnings":    0,
		"critical":    0,
		"full":        0,
		"pools":       forecasts,
	}

	for _, f := range forecasts {
		switch f.AlertLevel {
		case AlertWarning:
			summary["warnings"] = summary["warnings"].(int) + 1
		case AlertCritical:
			summary["critical"] = summary["critical"].(int) + 1
		case AlertFull:
			summary["full"] = summary["full"].(int) + 1
		}
	}

	return summary
}
