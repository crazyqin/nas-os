// Package aiops 提供 AIOps HTTP API 处理器
package aiops

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handlers AIOps API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册 HTTP 路由.
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	// 诊断
	mux.HandleFunc("/api/v1/aiops/diagnose", h.handleDiagnose)

	// 告警
	mux.HandleFunc("/api/v1/aiops/alerts", h.handleAlerts)
	mux.HandleFunc("/api/v1/aiops/alerts/groups", h.handleAlertGroups)
	mux.HandleFunc("/api/v1/aiops/alerts/groups/suppress", h.handleSuppressAlertGroup)

	// 修复
	mux.HandleFunc("/api/v1/aiops/remediate", h.handleRemediate)

	// SLA
	mux.HandleFunc("/api/v1/aiops/sla", h.handleSLA)
	mux.HandleFunc("/api/v1/aiops/sla/list", h.handleSLAList)

	// 事件
	mux.HandleFunc("/api/v1/aiops/incidents", h.handleIncidents)
	mux.HandleFunc("/api/v1/aiops/incidents/resolve", h.handleResolveIncident)

	// 知识库
	mux.HandleFunc("/api/v1/aiops/knowledge", h.handleKnowledge)
	mux.HandleFunc("/api/v1/aiops/knowledge/search", h.handleKnowledgeSearch)

	// 统计
	mux.HandleFunc("/api/v1/aiops/stats", h.handleStats)
}

// apiResponse 标准 API 响应.
type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// writeJSON 写入 JSON 响应.
func writeJSON(w http.ResponseWriter, status int, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// writeError 写入错误响应.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiResponse{Code: 1, Message: msg})
}

// handleDiagnose 处理诊断请求.
func (h *Handlers) handleDiagnose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req DiagnoseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	incident, err := h.manager.Diagnose(&req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "diagnosis completed",
		Data:    incident,
	})
}

// handleAlerts 处理告警请求.
func (h *Handlers) handleAlerts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req AlertIngestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}

		groups := h.manager.AggregateAlerts(req.Alerts)
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "alerts aggregated",
			Data:    groups,
		})

	case http.MethodGet:
		groups := h.manager.ListAlertGroups()
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    groups,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAlertGroups 列出告警组.
func (h *Handlers) handleAlertGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	groups := h.manager.ListAlertGroups()
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    groups,
	})
}

// handleSuppressAlertGroup 静默告警组.
func (h *Handlers) handleSuppressAlertGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		GroupID string `json:"group_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if err := h.manager.SuppressAlertGroup(req.GroupID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "alert group suppressed",
	})
}

// handleRemediate 处理修复请求.
func (h *Handlers) handleRemediate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req RemediateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	action, err := h.manager.AutoRemediate(req.IncidentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "remediation executed",
		Data:    action,
	})
}

// handleSLA 处理 SLA 请求.
func (h *Handlers) handleSLA(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		service := r.URL.Query().Get("service")
		sla, err := h.manager.GetSLA(service)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    sla,
		})

	case http.MethodPost:
		var req SLATargetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}
		sla := h.manager.UpdateSLA(&req)
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "SLA target updated",
			Data:    sla,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleSLAList 列出 SLA 目标.
func (h *Handlers) handleSLAList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	slas := h.manager.ListSLAs()
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    slas,
	})
}

// handleIncidents 处理事件请求.
func (h *Handlers) handleIncidents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		status := r.URL.Query().Get("status")
		id := r.URL.Query().Get("id")

		if id != "" {
			inc, err := h.manager.GetIncident(id)
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{
				Code:    0,
				Message: "success",
				Data:    inc,
			})
			return
		}

		incidents := h.manager.ListIncidents(status)
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    incidents,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleResolveIncident 解决事件.
func (h *Handlers) handleResolveIncident(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		IncidentID string `json:"incident_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if err := h.manager.ResolveIncident(req.IncidentID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "incident resolved",
	})
}

// handleKnowledge 处理知识库请求.
func (h *Handlers) handleKnowledge(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id != "" {
			entry, err := h.manager.GetKnowledge(id)
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{
				Code:    0,
				Message: "success",
				Data:    entry,
			})
			return
		}

		entries := h.manager.ListKnowledge()
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    entries,
		})

	case http.MethodPost:
		var entry KnowledgeEntry
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}
		h.manager.AddKnowledge(&entry)
		writeJSON(w, http.StatusCreated, apiResponse{
			Code:    0,
			Message: "knowledge entry added",
			Data:    entry,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleKnowledgeSearch 搜索知识库.
func (h *Handlers) handleKnowledgeSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	results := h.manager.SearchKnowledge(query)
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    results,
	})
}

// handleStats 处理统计请求.
func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	stats := h.manager.GetStats()
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}
