// Package behaviorransom 提供基于行为分析的勒索软件检测功能
// handlers.go - HTTP API handlers
package behaviorransom

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// Handlers HTTP API handlers
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建新的HTTP handlers
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/security/behaviorransom", h.handleRequest)
	mux.HandleFunc("/api/v1/security/behaviorransom/status", h.handleStatus)
	mux.HandleFunc("/api/v1/security/behaviorransom/threats", h.handleThreats)
	mux.HandleFunc("/api/v1/security/behaviorransom/config", h.handleConfig)
	mux.HandleFunc("/api/v1/security/behaviorransom/quarantine", h.handleQuarantine)
	mux.HandleFunc("/api/v1/security/behaviorransom/patterns", h.handlePatterns)
}

// apiResponse API响应
type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// handleRequest 处理主请求（GET状态/POST活动记录）
func (h *Handlers) handleRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleStatus(w, r)
	case http.MethodPost:
		h.handleRecordActivity(w, r)
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

// handleStatus 处理状态查询
func (h *Handlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	status := h.manager.GetStatus()
	h.writeJSON(w, http.StatusOK, apiResponse{
		Code: 0,
		Data: status,
	})
}

// handleThreats 处理威胁查询
func (h *Handlers) handleThreats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	threats := h.manager.GetThreats()
	h.writeJSON(w, http.StatusOK, apiResponse{
		Code: 0,
		Data: threats,
	})
}

// handleConfig 处理配置查询/更新
func (h *Handlers) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config := h.manager.GetConfig()
		h.writeJSON(w, http.StatusOK, apiResponse{
			Code: 0,
			Data: config,
		})
	case http.MethodPut, http.MethodPost:
		var config DetectorConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			h.writeError(w, http.StatusBadRequest, "无效的配置数据: "+err.Error())
			return
		}
		h.manager.UpdateConfig(config)
		h.writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "配置已更新",
		})
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

// handleQuarantine 处理隔离记录查询
func (h *Handlers) handleQuarantine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	records := h.manager.GetQuarantineRecords()
	h.writeJSON(w, http.StatusOK, apiResponse{
		Code: 0,
		Data: records,
	})
}

// handlePatterns 处理行为模式查询
func (h *Handlers) handlePatterns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	patterns := h.manager.GetPatterns()
	h.writeJSON(w, http.StatusOK, apiResponse{
		Code: 0,
		Data: patterns,
	})
}

// handleRecordActivity 处理活动记录提交
func (h *Handlers) handleRecordActivity(w http.ResponseWriter, r *http.Request) {
	var activity FileActivity
	if err := json.NewDecoder(r.Body).Decode(&activity); err != nil {
		h.writeError(w, http.StatusBadRequest, "无效的活动数据: "+err.Error())
		return
	}

	if activity.Timestamp.IsZero() {
		activity.Timestamp = time.Now()
	}

	h.manager.RecordActivity(activity)

	h.writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "活动已记录",
	})
}

// writeJSON 写入JSON响应
func (h *Handlers) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("JSON编码错误: %v", err)
	}
}

// writeError 写入错误响应
func (h *Handlers) writeError(w http.ResponseWriter, statusCode int, message string) {
	h.writeJSON(w, statusCode, apiResponse{
		Code:    statusCode,
		Message: message,
	})
}
