package api

import (
	"encoding/json"
	"net/http"
	"time"

	"nas-os/internal/automation/engine"
	"nas-os/internal/automation/templates"
	"github.com/gorilla/mux"
)

// AutomationAPI 自动化 API 处理器
type AutomationAPI struct {
	engine *engine.WorkflowEngine
}

// NewAutomationAPI 创建新的 API 处理器
func NewAutomationAPI(eng *engine.WorkflowEngine) *AutomationAPI {
	return &AutomationAPI{
		engine: eng,
	}
}

// RegisterRoutes 注册 API 路由
func (a *AutomationAPI) RegisterRoutes(r *mux.Router) {
	s := r.PathPrefix("/api/automation").Subrouter()
	
	// 工作流 CRUD
	s.HandleFunc("/workflows", a.ListWorkflows).Methods("GET")
	s.HandleFunc("/workflows", a.CreateWorkflow).Methods("POST")
	s.HandleFunc("/workflows/{id}", a.GetWorkflow).Methods("GET")
	s.HandleFunc("/workflows/{id}", a.UpdateWorkflow).Methods("PUT")
	s.HandleFunc("/workflows/{id}", a.DeleteWorkflow).Methods("DELETE")
	s.HandleFunc("/workflows/{id}/toggle", a.ToggleWorkflow).Methods("POST")
	s.HandleFunc("/workflows/{id}/execute", a.ExecuteWorkflow).Methods("POST")
	s.HandleFunc("/workflows/export/{id}", a.ExportWorkflow).Methods("GET")
	s.HandleFunc("/workflows/import", a.ImportWorkflow).Methods("POST")
	
	// 模板
	s.HandleFunc("/templates", a.ListTemplates).Methods("GET")
	s.HandleFunc("/templates/{id}", a.GetTemplate).Methods("GET")
	s.HandleFunc("/templates/{id}/use", a.UseTemplate).Methods("POST")
	
	// 统计
	s.HandleFunc("/stats", a.GetStats).Methods("GET")
}

// WorkflowRequest 工作流请求
type WorkflowRequest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Enabled     bool                   `json:"enabled"`
	Trigger     map[string]interface{} `json:"trigger"`
	Actions     []map[string]interface{} `json:"actions"`
}

// ListWorkflows 列出所有工作流
func (a *AutomationAPI) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	workflows := a.engine.ListWorkflows()
	a.writeJSON(w, http.StatusOK, workflows)
}

// GetWorkflow 获取单个工作流
func (a *AutomationAPI) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	wf, err := a.engine.GetWorkflow(id)
	if err != nil {
		a.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	
	a.writeJSON(w, http.StatusOK, wf)
}

// CreateWorkflow 创建工作流
func (a *AutomationAPI) CreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var req WorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	wf := &engine.Workflow{
		Name:        req.Name,
		Description: req.Description,
		Enabled:     req.Enabled,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	// TODO: 解析 trigger 和 actions
	if err := a.engine.CreateWorkflow(wf); err != nil {
		a.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	a.writeJSON(w, http.StatusCreated, wf)
}

// UpdateWorkflow 更新工作流
func (a *AutomationAPI) UpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	var req WorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	wf := &engine.Workflow{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Enabled:     req.Enabled,
		UpdatedAt:   time.Now(),
	}
	
	if err := a.engine.UpdateWorkflow(id, wf); err != nil {
		a.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	a.writeJSON(w, http.StatusOK, wf)
}

// DeleteWorkflow 删除工作流
func (a *AutomationAPI) DeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	if err := a.engine.DeleteWorkflow(id); err != nil {
		a.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	
	a.writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ToggleWorkflow 切换工作流状态
func (a *AutomationAPI) ToggleWorkflow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	wf, err := a.engine.GetWorkflow(id)
	if err != nil {
		a.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	
	if wf.Enabled {
		err = a.engine.DisableWorkflow(id)
	} else {
		err = a.engine.EnableWorkflow(id)
	}
	
	if err != nil {
		a.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	a.writeJSON(w, http.StatusOK, map[string]string{"status": "toggled"})
}

// ExecuteWorkflow 手动执行工作流
func (a *AutomationAPI) ExecuteWorkflow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	var eventData map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&eventData)
	
	if err := a.engine.ExecuteWorkflow(id, eventData); err != nil {
		a.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	a.writeJSON(w, http.StatusOK, map[string]string{"status": "executing"})
}

// ExportWorkflow 导出工作流
func (a *AutomationAPI) ExportWorkflow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	data, err := a.engine.ExportWorkflow(id)
	if err != nil {
		a.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=workflow_"+id+".json")
	_, _ = w.Write(data)
}

// ImportWorkflow 导入工作流
func (a *AutomationAPI) ImportWorkflow(w http.ResponseWriter, r *http.Request) {
	var data []byte
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		a.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	
	wf, err := a.engine.ImportWorkflow(data)
	if err != nil {
		a.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	a.writeJSON(w, http.StatusCreated, wf)
}

// ListTemplates 列出所有模板
func (a *AutomationAPI) ListTemplates(w http.ResponseWriter, r *http.Request) {
	tpls := templates.GetTemplates()
	a.writeJSON(w, http.StatusOK, tpls)
}

// GetTemplate 获取单个模板
func (a *AutomationAPI) GetTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	tpl, err := templates.GetTemplate(id)
	if err != nil {
		a.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	
	if tpl == nil {
		a.writeError(w, http.StatusNotFound, "Template not found")
		return
	}
	
	a.writeJSON(w, http.StatusOK, tpl)
}

// UseTemplate 使用模板创建工作流
func (a *AutomationAPI) UseTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	tpl, err := templates.GetTemplate(id)
	if err != nil || tpl == nil {
		a.writeError(w, http.StatusNotFound, "Template not found")
		return
	}
	
	wf := &tpl.Workflow
	wf.ID = ""
	wf.CreatedAt = time.Now()
	wf.UpdatedAt = time.Now()
	wf.LastRun = nil
	wf.RunCount = 0
	
	if err := a.engine.CreateWorkflow(wf); err != nil {
		a.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	a.writeJSON(w, http.StatusCreated, wf)
}

// GetStats 获取统计信息
func (a *AutomationAPI) GetStats(w http.ResponseWriter, r *http.Request) {
	workflows := a.engine.ListWorkflows()
	
	total := len(workflows)
	active := 0
	totalRuns := 0
	
	for _, wf := range workflows {
		if wf.Enabled {
			active++
		}
		totalRuns += wf.RunCount
	}
	
	stats := map[string]interface{}{
		"total_workflows": total,
		"active_workflows": active,
		"total_runs": totalRuns,
		"success_rate": 95, // TODO: 实现实际的成功率计算
	}
	
	a.writeJSON(w, http.StatusOK, stats)
}

// 辅助函数
func (a *AutomationAPI) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (a *AutomationAPI) writeError(w http.ResponseWriter, status int, message string) {
	a.writeJSON(w, status, map[string]string{"error": message})
}
