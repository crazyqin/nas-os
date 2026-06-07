// Package workflowengine 提供工作流引擎 HTTP API
// 支持工作流 CRUD、模板管理、执行控制、统计查询等功能
package workflowengine

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// APIResponse API 响应结构
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// PaginatedResponse 分页响应结构
type PaginatedResponse struct {
	Items      interface{} `json:"items"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"pageSize"`
	TotalPages int         `json:"totalPages"`
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
// 路由设计遵循 RESTful 规范，支持以下端点：
//   - /api/v1/workflows: 工作流列表和创建
//   - /api/v1/workflows/{id}: 单个工作流操作
//   - /api/v1/workflows/{id}/execute: 执行工作流
//   - /api/v1/workflows/{id}/status: 获取执行状态
//   - /api/v1/workflows/{id}/history: 获取执行历史
//   - /api/v1/workflows/{id}/cancel: 取消执行
//   - /api/v1/workflows/{id}/dag: 获取 DAG 结构
//   - /api/v1/workflows/{id}/activate: 激活工作流
//   - /api/v1/workflows/{id}/deactivate: 停用工作流
//   - /api/v1/workflows/{id}/copy: 复制工作流
//   - /api/v1/workflow/templates: 模板列表
//   - /api/v1/workflow/templates/{id}: 模板操作
//   - /api/v1/workflow/templates/{id}/apply: 应用模板
//   - /api/v1/engine/status: 引擎状态
//   - /api/v1/engine/stats: 引擎统计
//   - /api/v1/engine/audit-logs: 审计日志
func (h *WorkflowHandler) RegisterRoutes(mux *http.ServeMux) {
	// 工作流 CRUD
	mux.HandleFunc("/api/v1/workflows", h.handleWorkflows)
	mux.HandleFunc("/api/v1/workflows/", h.handleWorkflowByID)

	// 模板管理
	mux.HandleFunc("/api/v1/workflow/templates", h.handleTemplates)
	mux.HandleFunc("/api/v1/workflow/templates/", h.handleTemplateByID)

	// 引擎状态和统计
	mux.HandleFunc("/api/v1/engine/status", h.handleEngineStatus)
	mux.HandleFunc("/api/v1/engine/stats", h.handleEngineStats)
	mux.HandleFunc("/api/v1/engine/audit-logs", h.handleAuditLogs)
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
// 支持的子路由：
//   - /{id}: GET/PUT/DELETE 单个工作流
//   - /{id}/execute: POST 执行工作流
//   - /{id}/status: GET 获取执行状态
//   - /{id}/history: GET 获取执行历史
//   - /{id}/cancel: POST 取消执行
//   - /{id}/dag: GET 获取 DAG 结构
//   - /{id}/activate: POST 激活工作流
//   - /{id}/deactivate: POST 停用工作流
//   - /{id}/copy: POST 复制工作流
func (h *WorkflowHandler) handleWorkflowByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/workflows/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		h.handleWorkflows(w, r)
		return
	}

	workflowID := parts[0]

	// 单个工作流操作
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

	// 子资源操作
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
		case "dag":
			if r.Method == http.MethodGet {
				h.getWorkflowDAG(w, r, workflowID)
				return
			}
		case "activate":
			if r.Method == http.MethodPost {
				h.activateWorkflow(w, r, workflowID)
				return
			}
		case "deactivate":
			if r.Method == http.MethodPost {
				h.deactivateWorkflow(w, r, workflowID)
				return
			}
		case "copy":
			if r.Method == http.MethodPost {
				h.copyWorkflow(w, r, workflowID)
				return
			}
		}
	}

	writeError(w, http.StatusNotFound, "endpoint not found")
}

// handleTemplates 处理模板列表
func (h *WorkflowHandler) handleTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	templates := h.engine.GetManager().ListTemplates()
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"templates": templates,
			"total":     len(templates),
		},
	})
}

// handleTemplateByID 处理模板操作（应用模板）
func (h *WorkflowHandler) handleTemplateByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/workflow/templates/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		h.handleTemplates(w, r)
		return
	}

	templateID := parts[0]

	if len(parts) >= 2 && parts[1] == "apply" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		h.applyTemplate(w, r, templateID)
		return
	}

	// 获取单个模板
	if r.Method == http.MethodGet {
		tmpl, err := h.engine.GetManager().GetTemplate(templateID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: tmpl})
		return
	}

	writeError(w, http.StatusNotFound, "endpoint not found")
}

// applyTemplate 应用模板创建工作流
func (h *WorkflowHandler) applyTemplate(w http.ResponseWriter, r *http.Request, templateID string) {
	var req struct {
		Name string `json:"name"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
			return
		}
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "system"
	}

	workflow, err := h.engine.GetManager().CreateFromTemplate(templateID, req.Name, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    workflow,
	})
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

// ========== 新增处理函数 ==========

// getWorkflowDAG 获取工作流 DAG 结构
// 返回工作流的节点和边信息，用于前端可视化
func (h *WorkflowHandler) getWorkflowDAG(w http.ResponseWriter, r *http.Request, id string) {
	dag, err := h.engine.GetManager().GetWorkflowDAG(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    dag,
	})
}

// activateWorkflow 激活工作流
// 将工作流状态设置为 active，使其可以被执行
func (h *WorkflowHandler) activateWorkflow(w http.ResponseWriter, r *http.Request, id string) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "system"
	}

	if err := h.engine.GetManager().ActivateWorkflow(id, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]string{"message": "workflow activated"},
	})
}

// deactivateWorkflow 停用工作流
// 将工作流状态设置为 disabled，使其暂时不可执行
func (h *WorkflowHandler) deactivateWorkflow(w http.ResponseWriter, r *http.Request, id string) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "system"
	}

	if err := h.engine.GetManager().DeactivateWorkflow(id, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]string{"message": "workflow deactivated"},
	})
}

// copyWorkflow 复制工作流
// 创建一个现有工作流的副本，可以指定新名称
func (h *WorkflowHandler) copyWorkflow(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Name string `json:"name"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
			return
		}
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "system"
	}

	// 获取原工作流
	original, err := h.engine.GetManager().GetWorkflow(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// 创建副本
	newReq := &CreateWorkflowRequest{
		Name:        req.Name,
		Description: original.Description,
		Nodes:       original.Nodes,
		Triggers:    original.Triggers,
		Variables:   original.Variables,
		Tags:        original.Tags,
	}

	workflow, err := h.engine.GetManager().CreateWorkflow(newReq, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    workflow,
	})
}

// handleEngineStats 处理引擎统计信息
// 返回工作流和执行的统计数据
func (h *WorkflowHandler) handleEngineStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	stats := h.engine.GetManager().GetStats()
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    stats,
	})
}

// handleAuditLogs 处理审计日志查询
// 支持按 entityType、entityId、action 筛选，支持分页
func (h *WorkflowHandler) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	filter := &AuditLogFilter{
		Page:     1,
		PageSize: 100,
	}

	// 解析查询参数
	if entityType := r.URL.Query().Get("entityType"); entityType != "" {
		filter.EntityType = entityType
	}
	if entityID := r.URL.Query().Get("entityId"); entityID != "" {
		filter.EntityID = entityID
	}
	if action := r.URL.Query().Get("action"); action != "" {
		filter.Action = action
	}
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			filter.Page = page
		}
	}
	if pageSizeStr := r.URL.Query().Get("pageSize"); pageSizeStr != "" {
		if pageSize, err := strconv.Atoi(pageSizeStr); err == nil && pageSize > 0 {
			filter.PageSize = pageSize
		}
	}

	logs := h.engine.GetManager().GetAuditLogs(filter)
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"logs":     logs,
			"total":    len(logs),
			"page":     filter.Page,
			"pageSize": filter.PageSize,
		},
	})
}
