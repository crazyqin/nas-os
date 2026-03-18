package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"nas-os/internal/automation/action"
	"nas-os/internal/automation/engine"
	"nas-os/internal/automation/templates"
	"nas-os/internal/automation/trigger"

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

	// 模板 - 注意：具体路由必须放在参数化路由之前
	s.HandleFunc("/templates", a.ListTemplates).Methods("GET")
	s.HandleFunc("/templates/categories", a.ListTemplateCategories).Methods("GET")
	s.HandleFunc("/templates/export-all", a.ExportAllTemplates).Methods("GET")
	s.HandleFunc("/templates/import", a.ImportTemplate).Methods("POST")
	s.HandleFunc("/templates/{id}", a.GetTemplate).Methods("GET")
	s.HandleFunc("/templates/{id}/validate", a.ValidateTemplate).Methods("GET")
	s.HandleFunc("/templates/{id}/params", a.GetTemplateParams).Methods("GET")
	s.HandleFunc("/templates/{id}/use", a.UseTemplate).Methods("POST")
	s.HandleFunc("/templates/export/{id}", a.ExportTemplate).Methods("GET")

	// 统计
	s.HandleFunc("/stats", a.GetStats).Methods("GET")
}

// WorkflowRequest 工作流请求
type WorkflowRequest struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Enabled     bool                     `json:"enabled"`
	Trigger     map[string]interface{}   `json:"trigger"`
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

	// 解析 trigger 配置
	if len(req.Trigger) > 0 {
		triggerConfig, err := parseTriggerConfig(req.Trigger)
		if err != nil {
			a.writeError(w, http.StatusBadRequest, "Invalid trigger config: "+err.Error())
			return
		}
		trig, err := trigger.NewTriggerFromConfig(triggerConfig)
		if err != nil {
			a.writeError(w, http.StatusBadRequest, "Failed to create trigger: "+err.Error())
			return
		}
		wf.Trigger = trig
	}

	// 解析 actions 配置
	for i, actionData := range req.Actions {
		actionConfig, err := parseActionConfig(actionData)
		if err != nil {
			a.writeError(w, http.StatusBadRequest, "Invalid action config at index "+strconv.Itoa(i)+": "+err.Error())
			return
		}
		act, err := action.NewActionFromConfig(actionConfig)
		if err != nil {
			a.writeError(w, http.StatusBadRequest, "Failed to create action at index "+strconv.Itoa(i)+": "+err.Error())
			return
		}
		wf.Actions = append(wf.Actions, act)
	}

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

	// 获取现有工作流以保留运行计数等信息
	existing, err := a.engine.GetWorkflow(id)
	if err != nil {
		a.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	wf := &engine.Workflow{
		ID:           id,
		Name:         req.Name,
		Description:  req.Description,
		Enabled:      req.Enabled,
		CreatedAt:    existing.CreatedAt,
		LastRun:      existing.LastRun,
		RunCount:     existing.RunCount,
		SuccessCount: existing.SuccessCount,
		FailCount:    existing.FailCount,
		UpdatedAt:    time.Now(),
	}

	// 解析 trigger 配置
	if len(req.Trigger) > 0 {
		triggerConfig, err := parseTriggerConfig(req.Trigger)
		if err != nil {
			a.writeError(w, http.StatusBadRequest, "Invalid trigger config: "+err.Error())
			return
		}
		trig, err := trigger.NewTriggerFromConfig(triggerConfig)
		if err != nil {
			a.writeError(w, http.StatusBadRequest, "Failed to create trigger: "+err.Error())
			return
		}
		wf.Trigger = trig
	}

	// 解析 actions 配置
	for i, actionData := range req.Actions {
		actionConfig, err := parseActionConfig(actionData)
		if err != nil {
			a.writeError(w, http.StatusBadRequest, "Invalid action config at index "+strconv.Itoa(i)+": "+err.Error())
			return
		}
		act, err := action.NewActionFromConfig(actionConfig)
		if err != nil {
			a.writeError(w, http.StatusBadRequest, "Failed to create action at index "+strconv.Itoa(i)+": "+err.Error())
			return
		}
		wf.Actions = append(wf.Actions, act)
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
	data, err := io.ReadAll(r.Body)
	if err != nil {
		a.writeError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	if len(data) == 0 {
		a.writeError(w, http.StatusBadRequest, "Empty request body")
		return
	}

	wf, err := a.engine.ImportWorkflow(data)
	if err != nil {
		a.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid workflow data: %v", err))
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

	// 解析可选的参数
	var params map[string]string
	_ = json.NewDecoder(r.Body).Decode(&params)

	wf := templates.CreateWorkflowFromTemplate(tpl, params)

	if err := a.engine.CreateWorkflow(wf); err != nil {
		a.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.writeJSON(w, http.StatusCreated, wf)
}

// ListTemplateCategories 列出模板分类
func (a *AutomationAPI) ListTemplateCategories(w http.ResponseWriter, r *http.Request) {
	categories := templates.GetCategories()
	a.writeJSON(w, http.StatusOK, map[string]interface{}{
		"categories": categories,
	})
}

// ValidateTemplate 验证模板
func (a *AutomationAPI) ValidateTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	tpl, err := templates.GetTemplate(id)
	if err != nil || tpl == nil {
		a.writeError(w, http.StatusNotFound, "Template not found")
		return
	}

	result := templates.ValidateTemplate(tpl)
	a.writeJSON(w, http.StatusOK, result)
}

// GetTemplateParams 获取模板参数
func (a *AutomationAPI) GetTemplateParams(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	tpl, err := templates.GetTemplate(id)
	if err != nil || tpl == nil {
		a.writeError(w, http.StatusNotFound, "Template not found")
		return
	}

	params := templates.GetTemplateParams(tpl)
	a.writeJSON(w, http.StatusOK, map[string]interface{}{
		"template_id":   tpl.ID,
		"template_name": tpl.Name,
		"params":        params,
	})
}

// ExportTemplate 导出模板
func (a *AutomationAPI) ExportTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	tpl, err := templates.GetTemplate(id)
	if err != nil || tpl == nil {
		a.writeError(w, http.StatusNotFound, "Template not found")
		return
	}

	data, err := templates.ExportTemplate(tpl)
	if err != nil {
		a.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=template_"+id+".json")
	_, _ = w.Write(data)
}

// ExportAllTemplates 导出所有模板
func (a *AutomationAPI) ExportAllTemplates(w http.ResponseWriter, r *http.Request) {
	tpls := templates.GetTemplates()

	allTemplates := make(map[string]json.RawMessage)
	for _, tpl := range tpls {
		data, err := templates.ExportTemplate(&tpl)
		if err != nil {
			continue
		}
		allTemplates[tpl.ID] = data
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=all_templates.json")
	_ = json.NewEncoder(w).Encode(allTemplates)
}

// ImportTemplate 导入模板
func (a *AutomationAPI) ImportTemplate(w http.ResponseWriter, r *http.Request) {
	var data []byte
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		a.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	tpl, err := templates.ImportTemplate(data)
	if err != nil {
		a.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	a.writeJSON(w, http.StatusCreated, tpl)
}

// GetStats 获取统计信息
func (a *AutomationAPI) GetStats(w http.ResponseWriter, r *http.Request) {
	workflows := a.engine.ListWorkflows()

	total := len(workflows)
	active := 0
	totalRuns := 0
	totalSuccess := 0
	totalFail := 0

	for _, wf := range workflows {
		if wf.Enabled {
			active++
		}
		totalRuns += wf.RunCount
		totalSuccess += wf.SuccessCount
		totalFail += wf.FailCount
	}

	// 计算成功率
	var successRate float64
	if totalRuns > 0 {
		successRate = float64(totalSuccess) / float64(totalRuns) * 100
	}

	stats := map[string]interface{}{
		"total_workflows":  total,
		"active_workflows": active,
		"total_runs":       totalRuns,
		"success_count":    totalSuccess,
		"fail_count":       totalFail,
		"success_rate":     int(successRate),
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

// parseTriggerConfig 从 map 解析触发器配置
func parseTriggerConfig(data map[string]interface{}) (trigger.Config, error) {
	config := trigger.Config{}

	// 解析 type
	if t, ok := data["type"].(string); ok {
		config.Type = trigger.Type(t)
	} else {
		return config, fmt.Errorf("missing or invalid trigger type")
	}

	// File trigger 字段
	if v, ok := data["path"].(string); ok {
		config.Path = v
	}
	if v, ok := data["pattern"].(string); ok {
		config.Pattern = v
	}
	if v, ok := data["events"].([]interface{}); ok {
		for _, e := range v {
			if s, ok := e.(string); ok {
				config.Events = append(config.Events, s)
			}
		}
	}
	if v, ok := data["recursive"].(bool); ok {
		config.Recursive = v
	}

	// Time trigger 字段
	if v, ok := data["schedule"].(string); ok {
		config.Schedule = v
	}
	if v, ok := data["timezone"].(string); ok {
		config.Timezone = v
	}
	if v, ok := data["once"].(bool); ok {
		config.Once = v
	}

	// Event trigger 字段
	if v, ok := data["event_type"].(string); ok {
		config.EventType = v
	}
	if v, ok := data["filter"].(map[string]interface{}); ok {
		config.Filter = v
	}

	// Webhook trigger 字段
	if v, ok := data["method"].(string); ok {
		config.Method = v
	}
	if v, ok := data["secret"].(string); ok {
		config.Secret = v
	}
	if v, ok := data["headers"].(map[string]interface{}); ok {
		config.Headers = make(map[string]string)
		for key, val := range v {
			if s, ok := val.(string); ok {
				config.Headers[key] = s
			}
		}
	}

	return config, nil
}

// parseActionConfig 从 map 解析动作配置
func parseActionConfig(data map[string]interface{}) (action.Config, error) {
	config := action.Config{}

	// 解析 type
	if t, ok := data["type"].(string); ok {
		config.Type = action.Type(t)
	} else {
		return config, fmt.Errorf("missing or invalid action type")
	}

	// 通用字段
	if v, ok := data["source"].(string); ok {
		config.Source = v
	}
	if v, ok := data["destination"].(string); ok {
		config.Destination = v
	}
	if v, ok := data["path"].(string); ok {
		config.Path = v
	}
	if v, ok := data["overwrite"].(bool); ok {
		config.Overwrite = v
	}
	if v, ok := data["recursive"].(bool); ok {
		config.Recursive = v
	}

	// Rename 字段
	if v, ok := data["new_name"].(string); ok {
		config.NewName = v
	}

	// Convert 字段
	if v, ok := data["format"].(string); ok {
		config.Format = v
	}
	if v, ok := data["options"].(map[string]interface{}); ok {
		config.Options = v
	}

	// Notify 字段
	if v, ok := data["channel"].(string); ok {
		config.Channel = v
	}
	if v, ok := data["message"].(string); ok {
		config.Message = v
	}
	if v, ok := data["title"].(string); ok {
		config.Title = v
	}
	if v, ok := data["to"].(string); ok {
		config.To = v
	}

	// Command 字段
	if v, ok := data["command"].(string); ok {
		config.Command = v
	}
	if v, ok := data["args"].([]interface{}); ok {
		for _, arg := range v {
			if s, ok := arg.(string); ok {
				config.Args = append(config.Args, s)
			}
		}
	}
	if v, ok := data["work_dir"].(string); ok {
		config.WorkDir = v
	}
	if v, ok := data["env"].([]interface{}); ok {
		for _, e := range v {
			if s, ok := e.(string); ok {
				config.Env = append(config.Env, s)
			}
		}
	}

	// Webhook 字段
	if v, ok := data["url"].(string); ok {
		config.URL = v
	}
	if v, ok := data["method"].(string); ok {
		config.Method = v
	}
	if v, ok := data["headers"].(map[string]interface{}); ok {
		config.Headers = make(map[string]string)
		for key, val := range v {
			if s, ok := val.(string); ok {
				config.Headers[key] = s
			}
		}
	}
	if v, ok := data["body"].(string); ok {
		config.Body = v
	}

	// Email 字段
	if v, ok := data["subject"].(string); ok {
		config.Subject = v
	}
	if v, ok := data["body"].(string); ok {
		config.Body = v
	}
	if v, ok := data["html"].(bool); ok {
		config.HTML = v
	}
	if v, ok := data["attachments"].([]interface{}); ok {
		for _, a := range v {
			if s, ok := a.(string); ok {
				config.Attachments = append(config.Attachments, s)
			}
		}
	}

	return config, nil
}
