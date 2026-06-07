package stigcompliance

import (
	"encoding/json"
	"net/http"
)

// APIHandler HTTP API处理器
type APIHandler struct {
	checker *STIGComplianceChecker
}

// NewAPIHandler 创建API处理器
func NewAPIHandler(checker *STIGComplianceChecker) *APIHandler {
	return &APIHandler{checker: checker}
}

// RegisterRoutes 注册路由
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/stig/rules", h.handleRules)
	mux.HandleFunc(prefix+"/stig/audit", h.handleAudit)
	mux.HandleFunc(prefix+"/stig/report", h.handleReport)
	mux.HandleFunc(prefix+"/stig/history", h.handleHistory)
}

func (h *APIHandler) handleRules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.checker.ListRules())
}

func (h *APIHandler) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	report := h.checker.RunAudit()
	writeJSON(w, http.StatusOK, report)
}

func (h *APIHandler) handleReport(w http.ResponseWriter, r *http.Request) {
	report := h.checker.GetLatestReport()
	if report == nil {
		http.Error(w, "no audit report available", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *APIHandler) handleHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.checker.GetReportHistory())
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
