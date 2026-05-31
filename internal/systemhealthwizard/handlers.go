package systemhealthwizard

import (
	"encoding/json"
	"net/http"
)

// Handler 健康检查向导 HTTP 处理器。
type Handler struct {
	wizard *Wizard
}

// NewHandler 创建处理器。
func NewHandler(wizard *Wizard) *Handler {
	return &Handler{wizard: wizard}
}

// RegisterRoutes 注册路由。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/system-health-wizard/start", h.Start)
	mux.HandleFunc("POST /api/system-health-wizard/{sessionId}/next", h.RunNext)
	mux.HandleFunc("POST /api/system-health-wizard/{sessionId}/run-all", h.RunAll)
	mux.HandleFunc("GET /api/system-health-wizard/{sessionId}", h.GetSession)
	mux.HandleFunc("GET /api/system-health-wizard/{sessionId}/report", h.GetReport)
	mux.HandleFunc("GET /api/system-health-wizard/steps", h.GetSteps)
}

// Start 启动检查向导。
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Steps []CheckStep `json:"steps,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// 使用默认步骤
		req.Steps = nil
	}

	session, err := h.wizard.StartSession(req.Steps)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, session)
}

// RunNext 执行下一步。
func (h *Handler) RunNext(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	result, err := h.wizard.RunNextStep(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// RunAll 执行所有步骤。
func (h *Handler) RunAll(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	report, err := h.wizard.RunAll(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// GetSession 获取会话信息。
func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	session, err := h.wizard.GetSession(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

// GetReport 获取报告。
func (h *Handler) GetReport(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	report, err := h.wizard.GetReport(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// GetSteps 获取所有步骤信息。
func (h *Handler) GetSteps(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, DefaultSteps())
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
