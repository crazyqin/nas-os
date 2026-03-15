// Package project provides project template functionality
package project

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ProjectTemplate 项目模板
type ProjectTemplate struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Category    string                 `json:"category,omitempty"` // software, marketing, operations, etc.
	IsPublic    bool                   `json:"is_public"`
	CreatedBy   string                 `json:"created_by"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Config      TemplateConfig         `json:"config"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// TemplateConfig 模板配置
type TemplateConfig struct {
	// 默认项目设置
	DefaultName        string `json:"default_name,omitempty"`
	DefaultDescription string `json:"default_description,omitempty"`
	KeyPrefix          string `json:"key_prefix,omitempty"`

	// 默认成员角色
	DefaultRoles []TemplateRole `json:"default_roles,omitempty"`

	// 默认里程碑
	DefaultMilestones []TemplateMilestone `json:"default_milestones,omitempty"`

	// 默认任务
	DefaultTasks []TemplateTask `json:"default_tasks,omitempty"`

	// 默认标签
	DefaultTags []string `json:"default_tags,omitempty"`

	// 默认标签
	DefaultLabels []string `json:"default_labels,omitempty"`

	// 工作流配置
	Workflows []TemplateWorkflow `json:"workflows,omitempty"`

	// 通知配置
	Notifications TemplateNotifications `json:"notifications,omitempty"`
}

// TemplateRole 模板角色
type TemplateRole struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions,omitempty"`
	IsDefault   bool     `json:"is_default,omitempty"`
}

// TemplateMilestone 模板里程碑
type TemplateMilestone struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	OffsetDays  int    `json:"offset_days"` // 相对于项目开始的天数
	Duration    int    `json:"duration"`    // 持续天数
}

// TemplateTask 模板任务
type TemplateTask struct {
	Title          string       `json:"title"`
	Description    string       `json:"description,omitempty"`
	Priority       TaskPriority `json:"priority,omitempty"`
	MilestoneIndex int          `json:"milestone_index,omitempty"` // 关联的里程碑索引
	EstimatedHours float64      `json:"estimated_hours,omitempty"`
	Tags           []string     `json:"tags,omitempty"`
	AssigneeRole   string       `json:"assignee_role,omitempty"` // 按角色分配
}

// TemplateWorkflow 模板工作流
type TemplateWorkflow struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Steps       []WorkflowStep    `json:"steps"`
	Triggers    []WorkflowTrigger `json:"triggers,omitempty"`
}

// WorkflowStep 工作流步骤
type WorkflowStep struct {
	Name       string            `json:"name"`
	Action     string            `json:"action"`
	Parameters map[string]string `json:"parameters,omitempty"`
	AssignTo   string            `json:"assign_to,omitempty"`
	Auto       bool              `json:"auto,omitempty"`
}

// WorkflowTrigger 工作流触发器
type WorkflowTrigger struct {
	Event   string `json:"event"`
	Step    int    `json:"step"`
	Enabled bool   `json:"enabled"`
}

// TemplateNotifications 模板通知配置
type TemplateNotifications struct {
	OnTaskAssigned    bool `json:"on_task_assigned"`
	OnTaskCompleted   bool `json:"on_task_completed"`
	OnMilestoneStart  bool `json:"on_milestone_start"`
	OnMilestoneDue    bool `json:"on_milestone_due"`
	OnProjectComplete bool `json:"on_project_complete"`
}

// TemplateManager 模板管理器
type TemplateManager struct {
	mu        sync.RWMutex
	templates map[string]*ProjectTemplate
	manager   *Manager
}

// NewTemplateManager 创建模板管理器
func NewTemplateManager(mgr *Manager) *TemplateManager {
	return &TemplateManager{
		templates: make(map[string]*ProjectTemplate),
		manager:   mgr,
	}
}

// CreateTemplate 创建模板
func (tm *TemplateManager) CreateTemplate(name, description, category, createdBy string, isPublic bool, config TemplateConfig) (*ProjectTemplate, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()
	template := &ProjectTemplate{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Category:    category,
		IsPublic:    isPublic,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
		Config:      config,
		Metadata:    make(map[string]interface{}),
	}

	tm.templates[template.ID] = template
	return template, nil
}

// GetTemplate 获取模板
func (tm *TemplateManager) GetTemplate(id string) (*ProjectTemplate, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	template, exists := tm.templates[id]
	if !exists {
		return nil, ErrTemplateNotFound
	}
	return template, nil
}

// UpdateTemplate 更新模板
func (tm *TemplateManager) UpdateTemplate(id string, updates map[string]interface{}) (*ProjectTemplate, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	template, exists := tm.templates[id]
	if !exists {
		return nil, ErrTemplateNotFound
	}

	now := time.Now()
	template.UpdatedAt = now

	if name, ok := updates["name"].(string); ok {
		template.Name = name
	}
	if desc, ok := updates["description"].(string); ok {
		template.Description = desc
	}
	if category, ok := updates["category"].(string); ok {
		template.Category = category
	}
	if isPublic, ok := updates["is_public"].(bool); ok {
		template.IsPublic = isPublic
	}

	return template, nil
}

// DeleteTemplate 删除模板
func (tm *TemplateManager) DeleteTemplate(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.templates[id]; !exists {
		return ErrTemplateNotFound
	}

	delete(tm.templates, id)
	return nil
}

// ListTemplates 列出模板
func (tm *TemplateManager) ListTemplates(userID, category string, publicOnly bool) []*ProjectTemplate {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]*ProjectTemplate, 0)
	for _, template := range tm.templates {
		// 筛选条件
		if publicOnly && !template.IsPublic {
			continue
		}
		if category != "" && template.Category != category {
			continue
		}
		if userID != "" && !template.IsPublic && template.CreatedBy != userID {
			continue
		}
		result = append(result, template)
	}

	// 按创建时间倒序
	tm.sortTemplates(result)
	return result
}

// ApplyTemplate 应用模板创建项目
func (tm *TemplateManager) ApplyTemplate(templateID, projectName, projectKey, ownerID, createdBy string) (*Project, error) {
	template, err := tm.GetTemplate(templateID)
	if err != nil {
		return nil, err
	}

	// 创建项目
	project, err := tm.manager.CreateProject(projectName, projectKey, template.Config.DefaultDescription, ownerID, createdBy)
	if err != nil {
		return nil, err
	}

	// 应用模板配置
	tm.applyMilestones(project.ID, template, createdBy)
	tm.applyTasks(project.ID, template, createdBy)

	return project, nil
}

// applyMilestones 应用里程碑
func (tm *TemplateManager) applyMilestones(projectID string, template *ProjectTemplate, createdBy string) {
	now := time.Now()
	for _, ms := range template.Config.DefaultMilestones {
		dueDate := now.AddDate(0, 0, ms.OffsetDays+ms.Duration)
		_, err := tm.manager.CreateMilestone(ms.Name, ms.Description, projectID, createdBy, &dueDate)
		if err != nil {
			continue
		}
	}
}

// applyTasks 应用任务
func (tm *TemplateManager) applyTasks(projectID string, template *ProjectTemplate, createdBy string) {
	for _, task := range template.Config.DefaultTasks {
		priority := task.Priority
		if priority == "" {
			priority = PriorityMedium
		}
		_, err := tm.manager.CreateTask(task.Title, task.Description, projectID, createdBy, priority)
		if err != nil {
			continue
		}
	}
}

// ExportTemplate 导出模板为JSON
func (tm *TemplateManager) ExportTemplate(id string) ([]byte, error) {
	template, err := tm.GetTemplate(id)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(template, "", "  ")
}

// ImportTemplate 从JSON导入模板
func (tm *TemplateManager) ImportTemplate(data []byte, createdBy string) (*ProjectTemplate, error) {
	var template ProjectTemplate
	if err := json.Unmarshal(data, &template); err != nil {
		return nil, err
	}

	// 重置ID和时间
	template.ID = uuid.New().String()
	template.CreatedBy = createdBy
	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()

	tm.mu.Lock()
	tm.templates[template.ID] = &template
	tm.mu.Unlock()

	return &template, nil
}

// CloneTemplate 克隆模板
func (tm *TemplateManager) CloneTemplate(id, newName, createdBy string) (*ProjectTemplate, error) {
	original, err := tm.GetTemplate(id)
	if err != nil {
		return nil, err
	}

	return tm.CreateTemplate(
		newName,
		original.Description,
		original.Category,
		createdBy,
		false, // 克隆的模板默认私有
		original.Config,
	)
}

// GetTemplatesByCategory 按分类获取模板
func (tm *TemplateManager) GetTemplatesByCategory(category string) []*ProjectTemplate {
	return tm.ListTemplates("", category, true)
}

// GetPublicTemplates 获取公开模板
func (tm *TemplateManager) GetPublicTemplates() []*ProjectTemplate {
	return tm.ListTemplates("", "", true)
}

// GetUserTemplates 获取用户模板
func (tm *TemplateManager) GetUserTemplates(userID string) []*ProjectTemplate {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]*ProjectTemplate, 0)
	for _, template := range tm.templates {
		if template.CreatedBy == userID {
			result = append(result, template)
		}
	}
	return result
}

// sortTemplates 按时间排序模板
func (tm *TemplateManager) sortTemplates(templates []*ProjectTemplate) {
	n := len(templates)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if templates[j].CreatedAt.Before(templates[j+1].CreatedAt) {
				templates[j], templates[j+1] = templates[j+1], templates[j]
			}
		}
	}
}

// 预定义模板

// GetDefaultTemplates 获取系统默认模板
func GetDefaultTemplates() []*ProjectTemplate {
	now := time.Now()
	return []*ProjectTemplate{
		{
			ID:          "tpl-software-dev",
			Name:        "软件开发项目",
			Description: "适用于软件开发项目的模板，包含需求、开发、测试、发布等阶段",
			Category:    "software",
			IsPublic:    true,
			CreatedBy:   "system",
			CreatedAt:   now,
			UpdatedAt:   now,
			Config: TemplateConfig{
				DefaultMilestones: []TemplateMilestone{
					{Name: "需求分析", Description: "收集和分析需求", OffsetDays: 0, Duration: 7},
					{Name: "设计阶段", Description: "系统设计和技术方案", OffsetDays: 7, Duration: 5},
					{Name: "开发阶段", Description: "功能开发", OffsetDays: 12, Duration: 14},
					{Name: "测试阶段", Description: "功能测试和修复", OffsetDays: 26, Duration: 7},
					{Name: "发布上线", Description: "部署和上线", OffsetDays: 33, Duration: 3},
				},
				DefaultTasks: []TemplateTask{
					{Title: "需求文档编写", Priority: PriorityHigh, MilestoneIndex: 0},
					{Title: "技术方案设计", Priority: PriorityHigh, MilestoneIndex: 1},
					{Title: "数据库设计", Priority: PriorityMedium, MilestoneIndex: 1},
					{Title: "核心功能开发", Priority: PriorityHigh, MilestoneIndex: 2},
					{Title: "单元测试编写", Priority: PriorityMedium, MilestoneIndex: 2},
					{Title: "集成测试", Priority: PriorityHigh, MilestoneIndex: 3},
					{Title: "性能测试", Priority: PriorityMedium, MilestoneIndex: 3},
					{Title: "部署文档", Priority: PriorityMedium, MilestoneIndex: 4},
				},
				DefaultTags: []string{"development", "software"},
			},
		},
		{
			ID:          "tpl-marketing",
			Name:        "营销活动项目",
			Description: "适用于营销活动项目的模板",
			Category:    "marketing",
			IsPublic:    true,
			CreatedBy:   "system",
			CreatedAt:   now,
			UpdatedAt:   now,
			Config: TemplateConfig{
				DefaultMilestones: []TemplateMilestone{
					{Name: "策划阶段", Description: "活动策划和方案", OffsetDays: 0, Duration: 5},
					{Name: "准备阶段", Description: "物料准备和渠道对接", OffsetDays: 5, Duration: 7},
					{Name: "执行阶段", Description: "活动执行", OffsetDays: 12, Duration: 3},
					{Name: "复盘阶段", Description: "数据分析和总结", OffsetDays: 15, Duration: 2},
				},
				DefaultTasks: []TemplateTask{
					{Title: "活动方案编写", Priority: PriorityHigh, MilestoneIndex: 0},
					{Title: "预算申请", Priority: PriorityHigh, MilestoneIndex: 0},
					{Title: "设计物料准备", Priority: PriorityMedium, MilestoneIndex: 1},
					{Title: "渠道对接", Priority: PriorityHigh, MilestoneIndex: 1},
					{Title: "活动执行监控", Priority: PriorityHigh, MilestoneIndex: 2},
					{Title: "数据分析报告", Priority: PriorityHigh, MilestoneIndex: 3},
				},
				DefaultTags: []string{"marketing", "campaign"},
			},
		},
		{
			ID:          "tpl-operations",
			Name:        "运维项目",
			Description: "适用于运维和基础设施项目的模板",
			Category:    "operations",
			IsPublic:    true,
			CreatedBy:   "system",
			CreatedAt:   now,
			UpdatedAt:   now,
			Config: TemplateConfig{
				DefaultMilestones: []TemplateMilestone{
					{Name: "需求评估", Description: "评估需求和资源", OffsetDays: 0, Duration: 3},
					{Name: "方案设计", Description: "设计和规划", OffsetDays: 3, Duration: 5},
					{Name: "实施部署", Description: "部署和配置", OffsetDays: 8, Duration: 7},
					{Name: "验证上线", Description: "测试和上线", OffsetDays: 15, Duration: 3},
				},
				DefaultTasks: []TemplateTask{
					{Title: "资源需求评估", Priority: PriorityHigh, MilestoneIndex: 0},
					{Title: "技术方案编写", Priority: PriorityHigh, MilestoneIndex: 1},
					{Title: "环境准备", Priority: PriorityMedium, MilestoneIndex: 2},
					{Title: "配置部署", Priority: PriorityHigh, MilestoneIndex: 2},
					{Title: "监控配置", Priority: PriorityMedium, MilestoneIndex: 2},
					{Title: "功能验证", Priority: PriorityHigh, MilestoneIndex: 3},
					{Title: "文档更新", Priority: PriorityMedium, MilestoneIndex: 3},
				},
				DefaultTags: []string{"operations", "infrastructure"},
			},
		},
	}
}

// InitializeDefaultTemplates 初始化默认模板
func (tm *TemplateManager) InitializeDefaultTemplates() {
	for _, template := range GetDefaultTemplates() {
		tm.mu.Lock()
		if _, exists := tm.templates[template.ID]; !exists {
			// 复制模板避免引用问题
			t := *template
			tm.templates[template.ID] = &t
		}
		tm.mu.Unlock()
	}
}

// 错误定义
var ErrTemplateNotFound = errors.New("模板不存在")
