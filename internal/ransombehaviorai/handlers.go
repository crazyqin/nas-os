// Package ransombehaviorai REST API 处理器
package ransombehaviorai

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handlers REST API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到标准 http.ServeMux
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/ransombehaviorai/status", h.handleStatus)
	mux.HandleFunc("/api/ransombehaviorai/start", h.handleStart)
	mux.HandleFunc("/api/ransombehaviorai/stop", h.handleStop)
	mux.HandleFunc("/api/ransombehaviorai/evaluate", h.handleEvaluate)
	mux.HandleFunc("/api/ransombehaviorai/report", h.handleReport)
	mux.HandleFunc("/api/ransombehaviorai/assessments", h.handleAssessments)
	mux.HandleFunc("/api/ransombehaviorai/responses", h.handleResponses)
	mux.HandleFunc("/api/ransombehaviorai/config", h.handleConfig)
	mux.HandleFunc("/api/ransombehaviorai/stats", h.handleStats)
}

// apiResponse 标准 API 响应
type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// GET /api/ransombehaviorai/status
func (h *Handlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := h.manager.GetStatus()
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    status,
	})
}

// POST /api/ransombehaviorai/start
func (h *Handlers) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.manager.Start(); err != nil {
		writeJSON(w, http.StatusConflict, apiResponse{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "engine started",
	})
}

// POST /api/ransombehaviorai/stop
func (h *Handlers) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.manager.Stop()
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "engine stopped",
	})
}

// POST /api/ransombehaviorai/evaluate - 手动触发评估
func (h *Handlers) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	assessment := h.manager.Evaluate()

	// 检查是否需要触发自动响应
	var respEvent *ResponseEvent
	if assessment.Score >= h.manager.GetConfig().AIModel.ScoreThreshold {
		respEvent = h.manager.TriggerResponse(assessment)
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "evaluation completed",
		Data: AssessmentResponse{
			Assessment: assessment,
			Action:     assessment.RecommendedAction,
			Message: func() string {
				if respEvent != nil {
					return respEvent.Message
				}
				return ""
			}(),
		},
	})
}

// POST /api/ransombehaviorai/report - 上报事件
func (h *Handlers) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ReportEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	for _, e := range req.FileEvents {
		h.manager.ReportFileEvent(e)
	}
	for _, s := range req.IOEvents {
		h.manager.ReportIOSample(s)
	}
	for _, e := range req.ProcessEvents {
		h.manager.ReportProcessEvent(e)
	}

	total := len(req.FileEvents) + len(req.IOEvents) + len(req.ProcessEvents)
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "events reported",
		Data: map[string]int{"count": total},
	})
}

// GET /api/ransombehaviorai/assessments
func (h *Handlers) handleAssessments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	assessments := h.manager.GetAssessments(limit)
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    assessments,
	})
}

// GET /api/ransombehaviorai/responses
func (h *Handlers) handleResponses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	responses := h.manager.GetResponseLog(limit)
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    responses,
	})
}

// GET/PUT /api/ransombehaviorai/config
func (h *Handlers) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := h.manager.GetConfig()
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    cfg,
		})
	case http.MethodPut:
		var cfg Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{
				Code:    1,
				Message: "invalid request: " + err.Error(),
			})
			return
		}
		h.manager.UpdateConfig(&cfg)
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "config updated",
		})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// GET /api/ransombehaviorai/stats
func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := h.manager.GetStatus()
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    status.Stats,
	})
}
