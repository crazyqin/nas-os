// Package aiagentorch - REST API handlers
package aiagentorch

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Handler HTTP 处理器.
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器.
func NewHandler(m *Manager) *Handler {
	return &Handler{manager: m}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/agents", h.handleAgents)
	mux.HandleFunc("/api/v1/agents/", h.handleAgentByID)
	mux.HandleFunc("/api/v1/agents/execute", h.handleExecuteAgent)
	mux.HandleFunc("/api/v1/tasks", h.handleTasks)
	mux.HandleFunc("/api/v1/tasks/", h.handleTaskByID)
	mux.HandleFunc("/api/v1/tasks/execute", h.handleExecuteTask)
	mux.HandleFunc("/api/v1/executions", h.handleExecutions)
	mux.HandleFunc("/api/v1/executions/", h.handleExecutionByID)
	mux.HandleFunc("/api/v1/messages", h.handleMessages)
	mux.HandleFunc("/api/v1/messages/read", h.handleMarkRead)
	mux.HandleFunc("/api/v1/stats", h.handleStats)
}

// ========== Agent Handlers ==========

func (h *Handler) handleAgents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listAgents(w, r)
	case http.MethodPost:
		h.createAgent(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleAgentByID(w http.ResponseWriter, r *http.Request) {
	// Extract ID from /api/v1/agents/{id} or sub-paths
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]

	if id == "" {
		http.Error(w, "agent id required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getAgent(w, id)
	case http.MethodPut:
		h.updateAgent(w, r, id)
	case http.MethodDelete:
		h.deleteAgent(w, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) listAgents(w http.ResponseWriter, r *http.Request) {
	agentType := AgentType(r.URL.Query().Get("type"))
	status := AgentStatus(r.URL.Query().Get("status"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 {
		pageSize = 20
	}

	agents, total := h.manager.ListAgents(agentType, status, page, pageSize)
	writeJSON(w, map[string]interface{}{
		"agents":   agents,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *Handler) createAgent(w http.ResponseWriter, r *http.Request) {
	var agent Agent
	if err := json.NewDecoder(r.Body).Decode(&agent); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.manager.CreateAgent(&agent); err != nil {
		status := http.StatusInternalServerError
		if err == ErrAgentNameExists {
			status = http.StatusConflict
		} else if err == ErrInvalidConfig {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, agent)
}

func (h *Handler) getAgent(w http.ResponseWriter, id string) {
	agent, err := h.manager.GetAgent(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, agent)
}

func (h *Handler) updateAgent(w http.ResponseWriter, r *http.Request, id string) {
	var update Agent
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.manager.UpdateAgent(id, &update); err != nil {
		status := http.StatusInternalServerError
		if err == ErrAgentNotFound {
			status = http.StatusNotFound
		} else if err == ErrAgentNameExists {
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}
	agent, _ := h.manager.GetAgent(id)
	writeJSON(w, agent)
}

func (h *Handler) deleteAgent(w http.ResponseWriter, id string) {
	if err := h.manager.DeleteAgent(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "deleted"})
}

func (h *Handler) handleExecuteAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		AgentID string `json:"agentId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	log, err := h.manager.ExecuteAgent(req.AgentID)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrAgentNotFound {
			status = http.StatusNotFound
		} else if err == ErrAgentNotActive {
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, log)
}

// ========== Task Handlers ==========

func (h *Handler) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listTasks(w, r)
	case http.MethodPost:
		h.createTask(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/tasks/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]

	if id == "" {
		http.Error(w, "task id required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getTask(w, id)
	case http.MethodDelete:
		h.deleteTask(w, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) listTasks(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agentId")
	tasks := h.manager.ListTasks(agentID)
	writeJSON(w, map[string]interface{}{
		"tasks": tasks,
		"total": len(tasks),
	})
}

func (h *Handler) createTask(w http.ResponseWriter, r *http.Request) {
	var task AgentTask
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.manager.CreateTask(&task); err != nil {
		status := http.StatusInternalServerError
		if err == ErrAgentNotFound {
			status = http.StatusNotFound
		} else if err == ErrInvalidConfig {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, task)
}

func (h *Handler) getTask(w http.ResponseWriter, id string) {
	task, err := h.manager.GetTask(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, task)
}

func (h *Handler) deleteTask(w http.ResponseWriter, id string) {
	if err := h.manager.DeleteTask(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "deleted"})
}

func (h *Handler) handleExecuteTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		TaskID string `json:"taskId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	log, err := h.manager.ExecuteTask(req.TaskID)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrTaskNotFound {
			status = http.StatusNotFound
		} else if err == ErrAgentNotActive {
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, log)
}

// ========== Execution Handlers ==========

func (h *Handler) handleExecutions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	agentID := r.URL.Query().Get("agentId")
	status := ExecutionStatus(r.URL.Query().Get("status"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 {
		pageSize = 20
	}

	logs, total := h.manager.GetExecutionLogs(agentID, status, page, pageSize)
	writeJSON(w, map[string]interface{}{
		"logs":     logs,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *Handler) handleExecutionByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/executions/")
	id := strings.TrimSuffix(path, "/")
	if id == "" {
		http.Error(w, "execution id required", http.StatusBadRequest)
		return
	}
	log, err := h.manager.GetExecutionLog(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, log)
}

// ========== Message Handlers ==========

func (h *Handler) handleMessages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listMessages(w, r)
	case http.MethodPost:
		h.sendMessage(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) listMessages(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agentId")
	unreadOnly := r.URL.Query().Get("unread") == "true"
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 {
		pageSize = 20
	}

	msgs, total := h.manager.GetMessages(agentID, unreadOnly, page, pageSize)
	writeJSON(w, map[string]interface{}{
		"messages": msgs,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *Handler) sendMessage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FromAgentID string          `json:"fromAgentId"`
		ToAgentID   string          `json:"toAgentId"`
		MessageType string          `json:"messageType"`
		Content     string          `json:"content"`
		Priority    MessagePriority `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Priority == "" {
		req.Priority = PriorityNormal
	}
	msg, err := h.manager.SendMessage(req.FromAgentID, req.ToAgentID, req.MessageType, req.Content, req.Priority)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrAgentNotFound {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, msg)
}

func (h *Handler) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		MessageID string `json:"messageId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.manager.MarkMessageRead(req.MessageID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "read"})
}

// ========== Stats Handler ==========

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, h.manager.GetStats())
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
