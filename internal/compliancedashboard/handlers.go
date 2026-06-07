// Package compliancedashboard - REST API handlers
package compliancedashboard

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(m *Manager) *Handler {
	return &Handler{manager: m}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/compliance/status", h.handleStatus)
	mux.HandleFunc("/api/v1/compliance/stats", h.handleStats)
	mux.HandleFunc("/api/v1/compliance/config", h.handleConfig)
	mux.HandleFunc("/api/v1/compliance/scan", h.handleScan)
	mux.HandleFunc("/api/v1/compliance/report", h.handleReport)
	mux.HandleFunc("/api/v1/compliance/reports", h.handleReports)
	mux.HandleFunc("/api/v1/compliance/checks", h.handleChecks)
	mux.HandleFunc("/api/v1/compliance/findings", h.handleFindings)
	mux.HandleFunc("/api/v1/compliance/audit", h.handleAuditLog)
	mux.HandleFunc("/api/v1/compliance/audit/event", h.handleLogEvent)
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := h.manager.GetStats()
	writeJSON(w, map[string]interface{}{
		"running":          true,
		"overallScore":     stats.OverallScore,
		"totalChecks":      stats.TotalChecks,
		"passedChecks":     stats.PassedChecks,
		"failedChecks":     stats.FailedChecks,
		"openFindings":     stats.OpenFindings,
		"criticalFindings": stats.CriticalFindings,
	})
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, h.manager.GetStats())
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, h.manager.GetConfig())
	case http.MethodPut:
		var cfg ComplianceConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.UpdateConfig(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "updated"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Framework ComplianceFramework `json:"framework"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	report, err := h.manager.RunScan(req.Framework)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, report)
}

func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	report, err := h.manager.GetReport(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, report)
}

func (h *Handler) handleReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	framework := ComplianceFramework(r.URL.Query().Get("framework"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 {
		pageSize = 20
	}
	reports, total := h.manager.ListReports(framework, page, pageSize)
	writeJSON(w, map[string]interface{}{
		"reports":  reports,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *Handler) handleChecks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	framework := ComplianceFramework(r.URL.Query().Get("framework"))
	status := ComplianceStatus(r.URL.Query().Get("status"))
	writeJSON(w, h.manager.GetChecks(framework, status))
}

func (h *Handler) handleFindings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status := r.URL.Query().Get("status")
	writeJSON(w, h.manager.GetFindings(status))
}

func (h *Handler) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID := r.URL.Query().Get("userId")
	action := r.URL.Query().Get("action")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 {
		pageSize = 50
	}
	events, total := h.manager.GetAuditLog(userID, action, page, pageSize)
	writeJSON(w, map[string]interface{}{
		"events":   events,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *Handler) handleLogEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var event AuditEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	h.manager.LogAuditEvent(event)
	writeJSON(w, map[string]string{"status": "logged"})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
