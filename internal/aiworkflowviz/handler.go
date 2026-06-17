package aiworkflowviz

import (
	"encoding/json"
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
	mux.HandleFunc("/api/aiworkflowviz/workflows", h.handleWorkflows)
	mux.HandleFunc("/api/aiworkflowviz/workflow", h.handleWorkflow)
	mux.HandleFunc("/api/aiworkflowviz/node", h.handleNode)
	mux.HandleFunc("/api/aiworkflowviz/edge", h.handleEdge)
}

func (h *Handler) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.engine.ListWorkflows())
	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		wf := h.engine.CreateWorkflow(req.Name, req.Description)
		writeJSON(w, http.StatusCreated, wf)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleWorkflow(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		wf, exists := h.engine.GetWorkflow(id)
		if !exists {
			writeError(w, http.StatusNotFound, "workflow not found")
			return
		}
		writeJSON(w, http.StatusOK, wf)
	case http.MethodDelete:
		if err := h.engine.DeleteWorkflow(id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		WorkflowID string `json:"workflow_id"`
		Node       Node   `json:"node"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.engine.AddNode(req.WorkflowID, &req.Node); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, req.Node)
}

func (h *Handler) handleEdge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		WorkflowID string `json:"workflow_id"`
		Edge       Edge   `json:"edge"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.engine.AddEdge(req.WorkflowID, &req.Edge); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, req.Edge)
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
