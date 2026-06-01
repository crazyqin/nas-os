package smartdedup

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Handler HTTP API 处理器。
type Handler struct {
	engine *Engine
}

// NewHandler 创建新的 HTTP 处理器。
func NewHandler(engine *Engine) *Handler {
	return &Handler{engine: engine}
}

// RegisterRoutes 注册 HTTP 路由。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/smartdedup/scan", h.handleScan)
	mux.HandleFunc("/api/smartdedup/dedup", h.handleDedup)
	mux.HandleFunc("/api/smartdedup/stats", h.handleStats)
	mux.HandleFunc("/api/smartdedup/config", h.handleConfig)
	mux.HandleFunc("/api/smartdedup/status", h.handleStatus)
}

// handleScan 处理扫描请求。
func (h *Handler) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	result, err := h.engine.Scan()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleDedup 处理去重请求。
func (h *Handler) handleDedup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	result, err := h.engine.Dedup()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleStats 处理统计查询。
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	stats := h.engine.Stats()
	writeJSON(w, http.StatusOK, stats)
}

// handleConfig 处理配置查询和更新。
func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config := h.engine.Config()
		writeJSON(w, http.StatusOK, config)
	case http.MethodPut, http.MethodPatch:
		var config Config
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid config: %v", err))
			return
		}
		h.engine.UpdateConfig(&config)
		writeJSON(w, http.StatusOK, h.engine.Config())
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleStatus 处理状态查询。
func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	status := map[string]interface{}{
		"enabled":  h.engine.config.Enabled,
		"scanning": h.engine.IsScanning(),
		"stats":    h.engine.Stats(),
		"config":   h.engine.Config(),
	}
	writeJSON(w, http.StatusOK, status)
}

// writeJSON 写入 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// 日志记录但不改变响应状态码
		_ = err
	}
}

// writeError 写入错误响应。
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
