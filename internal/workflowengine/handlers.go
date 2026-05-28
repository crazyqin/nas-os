// Package workflowengine 提供工作流引擎 HTTP API
package workflowengine

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// APIResponse API 响应结构
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// WorkflowHandler 工作流 API 处理器
type WorkflowHandler struct {
	engine *WorkflowEngine
}

// NewWorkflowHandler 创建处理器实例
func NewWorkflowHandler(engine *WorkflowEngine) *WorkflowHandler {
	return &WorkflowHandler{engine: engine}
}

// RegisterRoutes 注册路由
func (h *WorkflowHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/workflows", h.handleWorkflows)
	mux.HandleFunc("/api/v1/workflows/", h.handleWorkflowByID)
	mux.HandleFunc("/api/v1/engine/status", h.handleEngineStatus)
}

// handleWorkflows 处理工作流列表和创建
func (h *WorkflowHandler) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listWorkflows(w, r)
	case http.MethodPost:
		h.createWorkflow(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleWorkflowByID 处理单个工作流操作
func (h *WorkflowHandler) handleWorkflowByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/workflows/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		h.handleWorkflows(w, r)
		return
	}

	workflowID := parts[0]

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			h.getWorkflow(w, r, workflowID)
			return
		case http.MethodDelete:
			h.deleteWorkflow(w, r, workflowID)
			return
		case http.MethodPut:
			h.updateWorkflow(w, r, workflowID)
			return
		}
	}

	if len(parts) >= 2 {
		action := parts[1]
		switch action {
		case "execute":
			if r.Method == http.MethodPost {
				h.executeWorkflow(w, r, workflowID)
				return
			}
		case "status":
			if r.Method == http.MethodGet {
				h.getWorkflowStatus(w, r, workflowID)
				return
			}
		case "history":
			if r.Method == http.MethodGet {
				h.getWorkflowHistory(w, r, workflowID)
				return
			}
		case "cancel":
			if r.Method == http.MethodPost {
				h.cancelWorkflow(w, r, workflowID)
				return
			}
		}
	}

	writeError(w, http.StatusNotFound, "endpoint not found")
}

// handleEngineStatus 处理引擎状态查询
func (h *WorkflowHandler) handleEngineStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	status := map[string]interface{}{
		"status": h.engine.GetStatus(),
		"config": h.engine.GetConfig(),
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    status,
	})
}

// createWorkflow 创建工作流
func (h *WorkflowHandler) createWorkflow(w http.ResponseWriter, r *http.Request) {
	var req CreateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	// 从请求头或上下文获取用户ID
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "system"
	}

	workflow, err := h.engine.GetManager().CreateWorkflow(&req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    workflow,
	})
}

// listWorkflows 列出所有工作流
func (h *WorkflowHandler) listWorkflows(w http.ResponseWriter, r *http.Request) {
	filter := &WorkflowFilter{
		Page:     1,
		PageSize: 100,
	}

	// 解析查询参数
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = WorkflowStatus(status)
	}
	if keyword := r.URL.Query().Get("keyword"); keyword != "" {
		filter.Keyword = keyword
	}

	workflows := h.engine.GetManager().ListWorkflows(filter)

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"workflows": workflows,
			"total":     len(workflows),
			"page":      filter.Page,
			"pageSize":  filter.PageSize,
		},
	})
}

// getWorkflow 获取单个工作流
func (h *WorkflowHandler) getWorkflow(w http.ResponseWriter, r *http.Request, id string) {
	workflow, err := h.engine.GetManager().GetWorkflow(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    workflow,
	})
}

// updateWorkflow 更新工作流
func (h *WorkflowHandler) updateWorkflow(w http.ResponseWriter, r *http.Request, id string) {
	var req UpdateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "system"
	}

	workflow, err := h.engine.GetManager().UpdateWorkflow(id, &req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    workflow,
	})
}

// deleteWorkflow 删除工作流
func (h *WorkflowHandler) deleteWorkflow(w http.ResponseWriter, r *http.Request, id string) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "system"
	}

	if err := h.engine.GetManager().DeleteWorkflow(id, userID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]string{"message": "workflow deleted"},
	})
}

// executeWorkflow 执行工作流
func (h *WorkflowHandler) executeWorkflow(w http.ResponseWriter, r *http.Request, id string) {
	var req ExecuteWorkflowRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
			return
		}
	}

	if req.TriggeredBy == "" {
		req.TriggeredBy = r.Header.Get("X-User-ID")
	}
	if req.TriggeredBy == "" {
		req.TriggeredBy = "api"
	}

	exec, err := h.engine.ExecuteWorkflow(r.Context(), id, req.Input, req.TriggeredBy)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, APIResponse{
		Success: true,
		Data:    exec,
	})
}

// getWorkflowStatus 获取工作流执行状态
func (h *WorkflowHandler) getWorkflowStatus(w http.ResponseWriter, r *http.Request, id string) {
	// 获取该工作流的最新执行
	executions := h.engine.GetManager().ListExecutions(&ExecutionFilter{
		WorkflowID: id,
		Page:       1,
		PageSize:   1,
	})

	if len(executions) == 0 {
		writeError(w, http.StatusNotFound, "no executions found")
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    executions[0],
	})
}

// getWorkflowHistory 获取工作流执行历史
func (h *WorkflowHandler) getWorkflowHistory(w http.ResponseWriter, r *http.Request, id string) {
	filter := &ExecutionFilter{
		WorkflowID: id,
		Page:       1,
		PageSize:   50,
	}

	// 解析查询参数
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = ExecutionStatus(status)
	}

	executions := h.engine.GetManager().ListExecutions(filter)

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"executions": executions,
			"total":      len(executions),
			"page":       filter.Page,
			"pageSize":   filter.PageSize,
		},
	})
}

// cancelWorkflow 取消工作流执行
func (h *WorkflowHandler) cancelWorkflow(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		ExecutionID string `json:"executionId"`
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}

	if req.ExecutionID == "" {
		writeError(w, http.StatusBadRequest, "executionId is required")
		return
	}

	if err := h.engine.GetManager().CancelExecution(req.ExecutionID, "api"); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]string{"message": "execution cancelled"},
	})
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应
func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, APIResponse{
		Success: false,
		Error:   message,
	})
}
