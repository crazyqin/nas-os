package dsmagent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// WorkflowEngine 工作流引擎，支持多步骤自动化任务编排
// 提供动态工作流创建、条件分支、并行执行等高级能力
type WorkflowEngine struct {
	mu        sync.RWMutex
	templates map[string]*WorkflowTemplate // 工作流模板库
	instances map[string]*WorkflowInstance // 运行中的工作流实例
	registry  *ToolRegistry               // 工具注册中心引用
	guard     *Guardrails                  // 安全护栏引用
}

// WorkflowTemplate 工作流模板，定义可复用的工作流蓝图
type WorkflowTemplate struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Category    WorkflowCategory    `json:"category"`
	Steps       []StepDefinition    `json:"steps"`
	Variables   map[string]string   `json:"variables,omitempty"` // 模板变量定义
	RequiredRole AgentRole          `json:"required_role"`
	Version     string              `json:"version"`
	Enabled     bool                `json:"enabled"`
	CreatedAt   time.Time           `json:"created_at"`
}

// WorkflowCategory 工作流分类
type WorkflowCategory string

const (
	CategoryMaintenance WorkflowCategory = "maintenance" // 系统维护
	CategorySecurity    WorkflowCategory = "security"     // 安全运维
	CategoryBackup      WorkflowCategory = "backup"       // 备份恢复
	CategoryMonitor     WorkflowCategory = "monitor"      // 监控告警
	CategoryOptimize    WorkflowCategory = "optimize"      // 性能优化
)

// StepDefinition 步骤定义，支持条件和依赖
type StepDefinition struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Action      string                 `json:"action"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	DependsOn   []string               `json:"depends_on,omitempty"`   // 依赖的前置步骤ID
	Condition   string                 `json:"condition,omitempty"`    // 执行条件表达式
	Parallel    bool                   `json:"parallel,omitempty"`     // 是否可并行执行
	Timeout     time.Duration          `json:"timeout"`
	RetryCount  int                    `json:"retry_count"`
	Critical    bool                   `json:"critical,omitempty"`     // 是否为关键步骤（失败则终止）
	Rollback    string                 `json:"rollback,omitempty"`     // 回滚动作
}

// WorkflowInstance 工作流运行实例
type WorkflowInstance struct {
	ID          string                 `json:"id"`
	TemplateID  string                 `json:"template_id"`
	Status      WorkflowStatus         `json:"status"`
	Variables   map[string]interface{} `json:"variables,omitempty"` // 运行时变量
	StepResults map[string]*StepResult `json:"step_results"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
	ctx         context.Context
	cancel      context.CancelFunc
}

// WorkflowStatus 工作流状态
type WorkflowStatus string

const (
	WfStatusPending   WorkflowStatus = "pending"
	WfStatusRunning   WorkflowStatus = "running"
	WfStatusPaused    WorkflowStatus = "paused"
	WfStatusCompleted WorkflowStatus = "completed"
	WfStatusFailed    WorkflowStatus = "failed"
	WfStatusCancelled WorkflowStatus = "cancelled"
	WfStatusRollingBack WorkflowStatus = "rolling_back"
)

// StepResult 步骤执行结果
type StepResult struct {
	StepID    string                 `json:"step_id"`
	Status    TaskStatus             `json:"status"`
	Output    map[string]interface{} `json:"output,omitempty"`
	Error     string                 `json:"error,omitempty"`
	StartedAt time.Time              `json:"started_at"`
	EndedAt   *time.Time             `json:"ended_at,omitempty"`
	Retries   int                    `json:"retries"`
}

// NewWorkflowEngine 创建工作流引擎实例
func NewWorkflowEngine(registry *ToolRegistry, guard *Guardrails) *WorkflowEngine {
	engine := &WorkflowEngine{
		templates: make(map[string]*WorkflowTemplate),
		instances: make(map[string]*WorkflowInstance),
		registry:  registry,
		guard:     guard,
	}

	// 注册默认工作流模板
	engine.registerDefaultTemplates()

	return engine
}

// registerDefaultTemplates 注册内置工作流模板
func (e *WorkflowEngine) registerDefaultTemplates() {
	// 系统健康巡检模板
	e.RegisterTemplate(&WorkflowTemplate{
		ID:          "tpl_health_patrol",
		Name:        "系统健康巡检",
		Description: "全面检查系统CPU、内存、磁盘、网络、温度等指标，生成健康报告",
		Category:    CategoryMonitor,
		RequiredRole: RoleSystemAdmin,
		Version:     "1.0",
		Enabled:     true,
		CreatedAt:   time.Now(),
		Steps: []StepDefinition{
			{ID: "step_cpu", Name: "检查CPU", Action: "check_cpu", Timeout: 10 * time.Second},
			{ID: "step_mem", Name: "检查内存", Action: "check_memory", Timeout: 10 * time.Second},
			{ID: "step_disk", Name: "检查磁盘", Action: "check_disk", Timeout: 30 * time.Second, DependsOn: []string{"step_cpu"}},
			{ID: "step_net", Name: "检查网络", Action: "check_network", Timeout: 15 * time.Second, Parallel: true},
			{ID: "step_temp", Name: "检查温度", Action: "check_temperature", Timeout: 10 * time.Second, Parallel: true},
			{ID: "step_report", Name: "生成报告", Action: "generate_report", Timeout: 30 * time.Second, DependsOn: []string{"step_cpu", "step_mem", "step_disk", "step_net", "step_temp"}},
		},
	})

	// 安全加固模板
	e.RegisterTemplate(&WorkflowTemplate{
		ID:          "tpl_security_hardening",
		Name:        "安全加固检查",
		Description: "扫描系统安全漏洞，检查权限配置，加固系统安全",
		Category:    CategorySecurity,
		RequiredRole: RoleSecurityAdmin,
		Version:     "1.0",
		Enabled:     true,
		CreatedAt:   time.Now(),
		Steps: []StepDefinition{
			{ID: "step_port_scan", Name: "端口扫描", Action: "port_scan", Timeout: 120 * time.Second},
			{ID: "step_perm_check", Name: "权限检查", Action: "check_permissions", Timeout: 60 * time.Second, Parallel: true},
			{ID: "step_log_analysis", Name: "日志分析", Action: "analyze_logs", Timeout: 180 * time.Second, Parallel: true},
			{ID: "step_vuln_scan", Name: "漏洞扫描", Action: "vulnerability_scan", Timeout: 300 * time.Second, DependsOn: []string{"step_port_scan"}},
			{ID: "step_harden", Name: "自动加固", Action: "auto_harden", Timeout: 120 * time.Second, DependsOn: []string{"step_vuln_scan", "step_perm_check"}, Critical: true},
		},
	})

	// 备份验证模板
	e.RegisterTemplate(&WorkflowTemplate{
		ID:          "tpl_backup_verify",
		Name:        "备份完整性验证",
		Description: "验证最近备份的完整性和可恢复性，确保数据安全",
		Category:    CategoryBackup,
		RequiredRole: RoleBackupAdmin,
		Version:     "1.0",
		Enabled:     true,
		CreatedAt:   time.Now(),
		Steps: []StepDefinition{
			{ID: "step_list", Name: "列出备份", Action: "list_backups", Timeout: 60 * time.Second},
			{ID: "step_checksum", Name: "校验和验证", Action: "verify_checksums", Timeout: 300 * time.Second, DependsOn: []string{"step_list"}},
			{ID: "step_restore_test", Name: "恢复测试", Action: "test_restore", Timeout: 600 * time.Second, DependsOn: []string{"step_checksum"}, Critical: true},
		},
	})

	// 存储优化模板
	e.RegisterTemplate(&WorkflowTemplate{
		ID:          "tpl_storage_optimize",
		Name:        "存储空间优化",
		Description: "分析存储使用情况，清理冗余数据，优化存储结构",
		Category:    CategoryOptimize,
		RequiredRole: RoleStorageAdmin,
		Version:     "1.0",
		Enabled:     true,
		CreatedAt:   time.Now(),
		Steps: []StepDefinition{
			{ID: "step_analyze", Name: "存储分析", Action: "analyze_storage", Timeout: 300 * time.Second},
			{ID: "step_duplicates", Name: "查找重复", Action: "find_duplicates", Timeout: 600 * time.Second, DependsOn: []string{"step_analyze"}},
			{ID: "step_clean_temp", Name: "清理临时文件", Action: "clean_temp", Timeout: 120 * time.Second, Parallel: true},
			{ID: "step_compress", Name: "压缩归档", Action: "compress_archive", Timeout: 600 * time.Second, DependsOn: []string{"step_duplicates"}},
		},
	})
}

// RegisterTemplate 注册工作流模板
func (e *WorkflowEngine) RegisterTemplate(tmpl *WorkflowTemplate) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if tmpl.ID == "" {
		return fmt.Errorf("模板ID不能为空")
	}
	if _, exists := e.templates[tmpl.ID]; exists {
		return fmt.Errorf("模板已存在: %s", tmpl.ID)
	}

	e.templates[tmpl.ID] = tmpl
	log.Printf("[WorkflowEngine] 注册工作流模板: %s (%s)", tmpl.Name, tmpl.ID)
	return nil
}

// UnregisterTemplate 注销工作流模板
func (e *WorkflowEngine) UnregisterTemplate(templateID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.templates[templateID]; !exists {
		return fmt.Errorf("模板不存在: %s", templateID)
	}

	delete(e.templates, templateID)
	log.Printf("[WorkflowEngine] 注销工作流模板: %s", templateID)
	return nil
}

// GetTemplate 获取工作流模板
func (e *WorkflowEngine) GetTemplate(templateID string) (*WorkflowTemplate, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tmpl, exists := e.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("模板不存在: %s", templateID)
	}
	return tmpl, nil
}

// ListTemplates 列出所有工作流模板
func (e *WorkflowEngine) ListTemplates(category *WorkflowCategory) []*WorkflowTemplate {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var templates []*WorkflowTemplate
	for _, tmpl := range e.templates {
		if category == nil || tmpl.Category == *category {
			templates = append(templates, tmpl)
		}
	}
	return templates
}

// Execute 启动执行工作流实例
func (e *WorkflowEngine) Execute(templateID string, variables map[string]interface{}) (*WorkflowInstance, error) {
	e.mu.RLock()
	tmpl, exists := e.templates[templateID]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("模板不存在: %s", templateID)
	}

	if !tmpl.Enabled {
		return nil, fmt.Errorf("模板已禁用: %s", templateID)
	}

	// 安全护栏检查
	if e.guard != nil {
		if err := e.guard.CheckWorkflowExecution(tmpl); err != nil {
			return nil, fmt.Errorf("安全护栏拒绝执行: %w", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	instance := &WorkflowInstance{
		ID:          fmt.Sprintf("wf_%s_%d", templateID, time.Now().UnixNano()),
		TemplateID:  templateID,
		Status:      WfStatusRunning,
		Variables:   variables,
		StepResults: make(map[string]*StepResult),
		StartedAt:   time.Now(),
		ctx:         ctx,
		cancel:      cancel,
	}

	e.mu.Lock()
	e.instances[instance.ID] = instance
	e.mu.Unlock()

	log.Printf("[WorkflowEngine] 启动工作流实例: %s (模板: %s)", instance.ID, templateID)

	// 异步执行工作流
	go e.runInstance(instance, tmpl)

	return instance, nil
}

// runInstance 执行工作流实例
func (e *WorkflowEngine) runInstance(instance *WorkflowInstance, tmpl *WorkflowTemplate) {
	// 构建步骤依赖图
	stepMap := make(map[string]*StepDefinition)
	for i := range tmpl.Steps {
		stepMap[tmpl.Steps[i].ID] = &tmpl.Steps[i]
	}

	// 按拓扑顺序执行步骤
	completed := make(map[string]bool)
	failed := false

	for _, step := range tmpl.Steps {
		select {
		case <-instance.ctx.Done():
			instance.Status = WfStatusCancelled
			now := time.Now()
			instance.CompletedAt = &now
			return
		default:
		}

		// 检查依赖是否已完成
		if !e.dependenciesMet(step, completed) {
			continue
		}

		// 检查执行条件
		if step.Condition != "" && !e.evaluateCondition(step.Condition, instance.Variables) {
			instance.StepResults[step.ID] = &StepResult{
				StepID: step.ID,
				Status: TaskCompleted,
				Output: map[string]interface{}{"skipped": true, "reason": "条件不满足"},
			}
			completed[step.ID] = true
			continue
		}

		// 执行步骤
		result := e.executeStep(instance, &step)
		instance.StepResults[step.ID] = result

		if result.Status == TaskCompleted {
			completed[step.ID] = true
		} else if step.Critical {
			// 关键步骤失败，终止工作流
			instance.Status = WfStatusFailed
			instance.Error = fmt.Sprintf("关键步骤失败: %s - %s", step.Name, result.Error)
			now := time.Now()
			instance.CompletedAt = &now

			// 执行回滚
			e.rollback(instance, tmpl, step.ID)
			log.Printf("[WorkflowEngine] 工作流实例失败: %s", instance.ID)
			return
		} else {
			failed = true
		}
	}

	// 设置最终状态
	now := time.Now()
	instance.CompletedAt = &now
	if failed {
		instance.Status = WfStatusFailed
		instance.Error = "部分步骤执行失败"
	} else {
		instance.Status = WfStatusCompleted
	}

	log.Printf("[WorkflowEngine] 工作流实例完成: %s (状态: %s)", instance.ID, instance.Status)
}

// dependenciesMet 检查步骤依赖是否满足
func (e *WorkflowEngine) dependenciesMet(step StepDefinition, completed map[string]bool) bool {
	for _, dep := range step.DependsOn {
		if !completed[dep] {
			return false
		}
	}
	return true
}

// evaluateCondition 评估条件表达式
func (e *WorkflowEngine) evaluateCondition(condition string, variables map[string]interface{}) bool {
	// 简化实现：检查变量是否存在且非空
	// 实际实现应支持更复杂的表达式解析
	if val, ok := variables[condition]; ok {
		return val != nil && val != ""
	}
	return false
}

// executeStep 执行单个工作流步骤
func (e *WorkflowEngine) executeStep(instance *WorkflowInstance, step *StepDefinition) *StepResult {
	result := &StepResult{
		StepID:    step.ID,
		Status:    TaskRunning,
		StartedAt: time.Now(),
	}

	var lastErr error
	for attempt := 0; attempt <= step.RetryCount; attempt++ {
		if attempt > 0 {
			result.Retries = attempt
			log.Printf("[WorkflowEngine] 重试步骤 %s (第%d次)", step.Name, attempt)
			time.Sleep(time.Duration(attempt) * time.Second) // 指数退避
		}

		// 通过工具注册中心执行动作
		if e.registry != nil {
			err := e.registry.ExecuteAction(step.Action, step.Parameters)
			if err == nil {
				result.Status = TaskCompleted
				now := time.Now()
				result.EndedAt = &now
				return result
			}
			lastErr = err
		} else {
			// 无工具注册中心时的默认行为
			result.Status = TaskCompleted
			now := time.Now()
			result.EndedAt = &now
			return result
		}
	}

	result.Status = TaskFailed
	result.Error = lastErr.Error()
	now := time.Now()
	result.EndedAt = &now
	return result
}

// rollback 回滚已执行的步骤
func (e *WorkflowEngine) rollback(instance *WorkflowInstance, tmpl *WorkflowTemplate, failedStepID string) {
	instance.Status = WfStatusRollingBack
	log.Printf("[WorkflowEngine] 开始回滚工作流实例: %s", instance.ID)

	// 逆序执行回滚动作
	for i := len(tmpl.Steps) - 1; i >= 0; i-- {
		step := tmpl.Steps[i]
		if step.ID == failedStepID {
			break
		}
		if step.Rollback != "" {
			log.Printf("[WorkflowEngine] 回滚步骤: %s", step.Name)
			if e.registry != nil {
				e.registry.ExecuteAction(step.Rollback, nil)
			}
		}
	}

	instance.Status = WfStatusFailed
}

// GetInstance 获取工作流实例状态
func (e *WorkflowEngine) GetInstance(instanceID string) (*WorkflowInstance, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	instance, exists := e.instances[instanceID]
	if !exists {
		return nil, fmt.Errorf("工作流实例不存在: %s", instanceID)
	}
	return instance, nil
}

// CancelInstance 取消正在运行的工作流实例
func (e *WorkflowEngine) CancelInstance(instanceID string) error {
	e.mu.RLock()
	instance, exists := e.instances[instanceID]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("工作流实例不存在: %s", instanceID)
	}

	if instance.Status != WfStatusRunning && instance.Status != WfStatusPaused {
		return fmt.Errorf("工作流实例不在运行状态: %s", instance.Status)
	}

	instance.cancel()
	instance.Status = WfStatusCancelled
	now := time.Now()
	instance.CompletedAt = &now

	log.Printf("[WorkflowEngine] 工作流实例已取消: %s", instanceID)
	return nil
}

// ListInstances 列出工作流实例
func (e *WorkflowEngine) ListInstances(status *WorkflowStatus) []*WorkflowInstance {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var instances []*WorkflowInstance
	for _, inst := range e.instances {
		if status == nil || inst.Status == *status {
			instances = append(instances, inst)
		}
	}
	return instances
}
