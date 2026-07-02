package projectcenter

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// TemplateManager 项目模板管理器.
type TemplateManager struct {
	mu        sync.RWMutex
	templates map[string]*ProjectTemplate
	nextID    int
}

// NewTemplateManager 创建模板管理器.
func NewTemplateManager() *TemplateManager {
	m := &TemplateManager{
		templates: make(map[string]*ProjectTemplate),
		nextID:    1,
	}

	// 加载内置模板
	m.loadBuiltinTemplates()
	return m
}

// loadBuiltinTemplates 加载内置模板.
func (m *TemplateManager) loadBuiltinTemplates() {
	builtinTemplates := []ProjectTemplate{
		{
			Name:        "软件开发项目",
			Description: "适用于软件开发的完整项目模板",
			Category:    "software",
			Columns: []TemplateColumn{
				{Name: "待办", Status: TaskStatusTodo, Order: 1},
				{Name: "开发中", Status: TaskStatusInProgress, Order: 2},
				{Name: "测试中", Status: TaskStatusReview, Order: 3},
				{Name: "已完成", Status: TaskStatusDone, Order: 4},
			},
			Tasks: []TemplateTask{
				{Title: "需求分析", Priority: PriorityHigh, Phase: "规划", Order: 1, EstimateHours: 8},
				{Title: "技术方案设计", Priority: PriorityHigh, Phase: "规划", Order: 2, EstimateHours: 16},
				{Title: "开发环境搭建", Priority: PriorityMedium, Phase: "开发", Order: 3, EstimateHours: 4},
				{Title: "核心功能开发", Priority: PriorityHigh, Phase: "开发", Order: 4, EstimateHours: 40},
				{Title: "单元测试编写", Priority: PriorityMedium, Phase: "测试", Order: 5, EstimateHours: 16},
				{Title: "集成测试", Priority: PriorityMedium, Phase: "测试", Order: 6, EstimateHours: 8},
				{Title: "文档编写", Priority: PriorityLow, Phase: "收尾", Order: 7, EstimateHours: 8},
				{Title: "部署上线", Priority: PriorityHigh, Phase: "收尾", Order: 8, EstimateHours: 4},
			},
			Tags:      []string{"software", "development", "agile"},
			IsDefault: true,
		},
		{
			Name:        "市场营销活动",
			Description: "适用于市场推广和营销活动",
			Category:    "marketing",
			Columns: []TemplateColumn{
				{Name: "计划中", Status: TaskStatusTodo, Order: 1},
				{Name: "执行中", Status: TaskStatusInProgress, Order: 2},
				{Name: "审核中", Status: TaskStatusReview, Order: 3},
				{Name: "已完成", Status: TaskStatusDone, Order: 4},
			},
			Tasks: []TemplateTask{
				{Title: "市场调研", Priority: PriorityHigh, Phase: "准备", Order: 1, EstimateHours: 16},
				{Title: "目标受众分析", Priority: PriorityHigh, Phase: "准备", Order: 2, EstimateHours: 8},
				{Title: "营销策略制定", Priority: PriorityHigh, Phase: "策划", Order: 3, EstimateHours: 16},
				{Title: "创意内容制作", Priority: PriorityMedium, Phase: "执行", Order: 4, EstimateHours: 24},
				{Title: "渠道投放", Priority: PriorityMedium, Phase: "执行", Order: 5, EstimateHours: 8},
				{Title: "效果追踪分析", Priority: PriorityMedium, Phase: "分析", Order: 6, EstimateHours: 8},
				{Title: "总结报告", Priority: PriorityLow, Phase: "分析", Order: 7, EstimateHours: 4},
			},
			Tags: []string{"marketing", "campaign"},
		},
		{
			Name:        "研究项目",
			Description: "适用于学术研究或技术研究项目",
			Category:    "research",
			Columns: []TemplateColumn{
				{Name: "待研究", Status: TaskStatusTodo, Order: 1},
				{Name: "研究中", Status: TaskStatusInProgress, Order: 2},
				{Name: "评审中", Status: TaskStatusReview, Order: 3},
				{Name: "已完成", Status: TaskStatusDone, Order: 4},
			},
			Tasks: []TemplateTask{
				{Title: "文献综述", Priority: PriorityHigh, Phase: "调研", Order: 1, EstimateHours: 24},
				{Title: "研究方案设计", Priority: PriorityHigh, Phase: "规划", Order: 2, EstimateHours: 16},
				{Title: "数据收集", Priority: PriorityMedium, Phase: "执行", Order: 3, EstimateHours: 40},
				{Title: "数据分析", Priority: PriorityMedium, Phase: "执行", Order: 4, EstimateHours: 32},
				{Title: "论文撰写", Priority: PriorityHigh, Phase: "撰写", Order: 5, EstimateHours: 40},
				{Title: "同行评审", Priority: PriorityMedium, Phase: "评审", Order: 6, EstimateHours: 16},
				{Title: "修改完善", Priority: PriorityMedium, Phase: "收尾", Order: 7, EstimateHours: 16},
			},
			Tags: []string{"research", "academic"},
		},
		{
			Name:        "通用项目",
			Description: "通用的项目管理模板",
			Category:    "general",
			Columns: []TemplateColumn{
				{Name: "待办", Status: TaskStatusTodo, Order: 1},
				{Name: "进行中", Status: TaskStatusInProgress, Order: 2},
				{Name: "已完成", Status: TaskStatusDone, Order: 3},
			},
			Tasks: []TemplateTask{
				{Title: "项目启动", Priority: PriorityHigh, Phase: "启动", Order: 1, EstimateHours: 4},
				{Title: "任务分配", Priority: PriorityMedium, Phase: "规划", Order: 2, EstimateHours: 2},
				{Title: "执行阶段", Priority: PriorityHigh, Phase: "执行", Order: 3, EstimateHours: 0},
				{Title: "项目收尾", Priority: PriorityMedium, Phase: "收尾", Order: 4, EstimateHours: 4},
			},
			Tags:      []string{"general"},
			IsDefault: true,
		},
	}

	for _, tmpl := range builtinTemplates {
		id := fmt.Sprintf("tmpl_%d", m.nextID)
		m.nextID++
		tmpl.ID = id
		tmpl.CreatedAt = time.Now()
		m.templates[id] = &tmpl
	}
}

// CreateTemplate 创建自定义模板.
func (m *TemplateManager) CreateTemplate(req ProjectTemplate) (*ProjectTemplate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("template name is required")
	}

	id := fmt.Sprintf("tmpl_%d", m.nextID)
	m.nextID++

	req.ID = id
	req.CreatedAt = time.Now()
	req.UsageCount = 0

	m.templates[id] = &req
	return &req, nil
}

// GetTemplate 获取模板.
func (m *TemplateManager) GetTemplate(templateID string) (*ProjectTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tmpl, exists := m.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("template %s not found", templateID)
	}
	return tmpl, nil
}

// UpdateTemplate 更新模板.
func (m *TemplateManager) UpdateTemplate(templateID string, req ProjectTemplate) (*ProjectTemplate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tmpl, exists := m.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("template %s not found", templateID)
	}

	if req.Name != "" {
		tmpl.Name = req.Name
	}
	if req.Description != "" {
		tmpl.Description = req.Description
	}
	if req.Category != "" {
		tmpl.Category = req.Category
	}
	if req.Columns != nil {
		tmpl.Columns = req.Columns
	}
	if req.Tasks != nil {
		tmpl.Tasks = req.Tasks
	}
	if req.Tags != nil {
		tmpl.Tags = req.Tags
	}

	return tmpl, nil
}

// DeleteTemplate 删除模板.
func (m *TemplateManager) DeleteTemplate(templateID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tmpl, exists := m.templates[templateID]
	if !exists {
		return fmt.Errorf("template %s not found", templateID)
	}

	if tmpl.IsDefault {
		return fmt.Errorf("cannot delete default template")
	}

	delete(m.templates, templateID)
	return nil
}

// ListTemplates 列出模板.
func (m *TemplateManager) ListTemplates(category string) []*ProjectTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var templates []*ProjectTemplate
	for _, tmpl := range m.templates {
		if category == "" || tmpl.Category == category {
			templates = append(templates, tmpl)
		}
	}

	sort.Slice(templates, func(i, j int) bool {
		return templates[i].UsageCount > templates[j].UsageCount
	})

	return templates
}

// GetTemplateCategories 获取模板分类列表.
func (m *TemplateManager) GetTemplateCategories() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	categorySet := make(map[string]bool)
	for _, tmpl := range m.templates {
		categorySet[tmpl.Category] = true
	}

	var categories []string
	for cat := range categorySet {
		categories = append(categories, cat)
	}
	sort.Strings(categories)
	return categories
}

// IncrementUsage 增加模板使用次数.
func (m *TemplateManager) IncrementUsage(templateID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tmpl, exists := m.templates[templateID]; exists {
		tmpl.UsageCount++
	}
}

// CloneTemplate 克隆模板.
func (m *TemplateManager) CloneTemplate(templateID, newName string) (*ProjectTemplate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	original, exists := m.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("template %s not found", templateID)
	}

	id := fmt.Sprintf("tmpl_%d", m.nextID)
	m.nextID++

	cloned := &ProjectTemplate{
		ID:          id,
		Name:        newName,
		Description: original.Description,
		Category:    original.Category,
		Columns:     make([]TemplateColumn, len(original.Columns)),
		Tasks:       make([]TemplateTask, len(original.Tasks)),
		Tags:        make([]string, len(original.Tags)),
		IsDefault:   false,
		UsageCount:  0,
		CreatedAt:   time.Now(),
	}

	copy(cloned.Columns, original.Columns)
	copy(cloned.Tasks, original.Tasks)
	copy(cloned.Tags, original.Tags)

	m.templates[id] = cloned
	return cloned, nil
}

// GetDefaultTemplate 获取默认模板.
func (m *TemplateManager) GetDefaultTemplate(category string) (*ProjectTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, tmpl := range m.templates {
		if tmpl.IsDefault && (category == "" || tmpl.Category == category) {
			return tmpl, nil
		}
	}
	return nil, fmt.Errorf("no default template found for category %s", category)
}

// SearchTemplates 搜索模板.
func (m *TemplateManager) SearchTemplates(keyword string) []*ProjectTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*ProjectTemplate
	keyword = toLower(keyword)

	for _, tmpl := range m.templates {
		if containsIgnoreCase(tmpl.Name, keyword) ||
			containsIgnoreCase(tmpl.Description, keyword) ||
			containsTag(tmpl.Tags, keyword) {
			results = append(results, tmpl)
		}
	}
	return results
}

// helper functions

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + 32
		}
		result[i] = c
	}
	return string(result)
}

func containsIgnoreCase(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && toLower(s) == toLower(substr) || len(s) > 0 && contains(toLower(s), toLower(substr))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func containsTag(tags []string, keyword string) bool {
	for _, tag := range tags {
		if toLower(tag) == keyword {
			return true
		}
	}
	return false
}
