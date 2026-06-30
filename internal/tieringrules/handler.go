// Package tieringrules 提供数据分层自定义规则功能。
package tieringrules

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP 处理器
type Handler struct {
	engine *Engine
}

// NewHandler 创建 HTTP 处理器
func NewHandler(engine *Engine) *Handler {
	return &Handler{engine: engine}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/tieringrules", h.Rules)
	mux.HandleFunc("/api/tieringrules/history", h.History)
}

// Rules 处理 CRUD /api/tieringrules
func (h *Handler) Rules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listRules(w, r)
	case http.MethodPost:
		h.createRule(w, r)
	case http.MethodPut:
		h.updateRule(w, r)
	case http.MethodDelete:
		h.deleteRule(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse("method not allowed"))
	}
}

func (h *Handler) listRules(w http.ResponseWriter, _ *http.Request) {
	rules := h.engine.ListRules()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rules": rules,
		"count": len(rules),
	})
}

func (h *Handler) createRule(w http.ResponseWriter, r *http.Request) {
	var req CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse("invalid request body"))
		return
	}
	rule, err := h.engine.CreateRule(&req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse(err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (h *Handler) updateRule(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse("id parameter is required"))
		return
	}
	var req CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse("invalid request body"))
		return
	}
	rule, err := h.engine.UpdateRule(id, &req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (h *Handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse("id parameter is required"))
		return
	}
	if err := h.engine.DeleteRule(id); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// History 处理 GET /api/tieringrules/history
func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse("method not allowed"))
		return
	}
	history := h.engine.GetHistory()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"records": history,
		"count":   len(history),
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func errorResponse(msg string) map[string]string {
	return map[string]string{"error": msg}
}