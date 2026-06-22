// Package immutableaudit HTTP API 处理器
package immutableaudit

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// HTTPHandler 不可变审计日志 HTTP 处理器
type HTTPHandler struct {
	audit *ImmutableAuditLog
}

// NewHTTPHandler 创建 HTTP 处理器
func NewHTTPHandler(audit *ImmutableAuditLog) *HTTPHandler {
	return &HTTPHandler{audit: audit}
}

// RegisterRoutes 注册路由
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/audit/record", h.handleRecord)
	mux.HandleFunc("/api/audit/events", h.handleGetEvents)
	mux.HandleFunc("/api/audit/event", h.handleGetEvent)
	mux.HandleFunc("/api/audit/verify", h.handleVerify)
	mux.HandleFunc("/api/audit/chain-state", h.handleChainState)
	mux.HandleFunc("/api/audit/stats", h.handleStats)
	mux.HandleFunc("/api/audit/merkle-tree", h.handleMerkleTree)
	mux.HandleFunc("/api/audit/export", h.handleExport)
}

func (h *HTTPHandler) handleRecord(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		EventType string                 `json:"event_type"`
		Actor     string                 `json:"actor"`
		Resource  string                 `json:"resource"`
		Action    string                 `json:"action"`
		Result    string                 `json:"result"`
		Severity  string                 `json:"severity"`
		Details   map[string]interface{} `json:"details,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	event, err := h.audit.Record(req.EventType, req.Actor, req.Resource, req.Action, req.Result, req.Severity, req.Details)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(event)
}

func (h *HTTPHandler) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	eventType := r.URL.Query().Get("type")
	severity := r.URL.Query().Get("severity")

	events := h.audit.GetEvents(offset, limit, eventType, severity)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

func (h *HTTPHandler) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "缺少 id 参数", http.StatusBadRequest)
		return
	}

	event := h.audit.GetEvent(id)
	if event == nil {
		http.Error(w, "事件未找到", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(event)
}

func (h *HTTPHandler) handleVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	report := h.audit.Verify()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

func (h *HTTPHandler) handleChainState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := h.audit.GetChainState()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func (h *HTTPHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.audit.GetStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *HTTPHandler) handleMerkleTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tree := h.audit.BuildMerkleTree()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tree)
}

func (h *HTTPHandler) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := h.audit.ExportJSON()
	if err != nil {
		http.Error(w, "导出失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=audit-export.json")
	w.Write(data)
}
