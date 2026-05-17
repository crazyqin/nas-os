package userwizard

import (
	"fmt"
	"sync"
)

// Engine 用户管理向导引擎.
type Engine struct {
	mu        sync.RWMutex
	templates map[string]*UserTemplate
}

// NewEngine 创建向导引擎.
func NewEngine() *Engine {
	e := &Engine{
		templates: make(map[string]*UserTemplate),
	}
	e.initDefaultTemplates()
	return e
}

// initDefaultTemplates 初始化默认模板.
func (e *Engine) initDefaultTemplates() {
	defaults := []*UserTemplate{
		{
			ID:           "tpl_admin",
			Name:         "管理员",
			Description:  "系统管理员，拥有全部权限",
			Role:         RoleAdmin,
			StorageQuota: 0, // 无限
			Groups:       []string{"admin"},
			IsDefault:    true,
		},
		{
			ID:           "tpl_standard",
			Name:         "标准用户",
			Description:  "普通用户，可访问常用服务",
			Role:         RoleStandard,
			StorageQuota: 100 * 1024 * 1024 * 1024, // 100GB
			AllowedServices: []string{
				"smb", "webdav", "ftp", "photos", "drive",
			},
			Groups:    []string{"users"},
			IsDefault: true,
		},
		{
			ID:           "tpl_readonly",
			Name:         "只读用户",
			Description:  "只读访问，适合查看共享内容",
			Role:         RoleReadOnly,
			StorageQuota: 10 * 1024 * 1024 * 1024, // 10GB
			AllowedServices: []string{
				"smb", "webdav", "photos",
			},
			Groups:    []string{"users"},
			IsDefault: true,
		},
		{
			ID:           "tpl_guest",
			Name:         "访客",
			Description:  "临时访客，有限访问权限",
			Role:         RoleGuest,
			StorageQuota: 1 * 1024 * 1024 * 1024, // 1GB
			AllowedServices: []string{
				"smb",
			},
			Groups:    []string{"guests"},
			IsDefault: true,
		},
	}

	for _, t := range defaults {
		e.templates[t.ID] = t
	}
}

// GetTemplates 获取所有模板.
func (e *Engine) GetTemplates() []UserTemplate {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]UserTemplate, 0, len(e.templates))
	for _, t := range e.templates {
		result = append(result, *t)
	}
	return result
}

// GetTemplate 获取指定模板.
func (e *Engine) GetTemplate(id string) (*UserTemplate, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	t, ok := e.templates[id]
	if !ok {
		return nil, fmt.Errorf("模板不存在：%s", id)
	}
	return t, nil
}

// GetTemplateByRole 根据角色获取默认模板.
func (e *Engine) GetTemplateByRole(role TemplateRole) (*UserTemplate, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, t := range e.templates {
		if t.Role == role && t.IsDefault {
			return t, nil
		}
	}
	return nil, fmt.Errorf("未找到角色 %s 的默认模板", role)
}

// AddTemplate 添加自定义模板.
func (e *Engine) AddTemplate(t *UserTemplate) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if t.ID == "" {
		return fmt.Errorf("模板 ID 不能为空")
	}
	if _, exists := e.templates[t.ID]; exists {
		return fmt.Errorf("模板已存在：%s", t.ID)
	}
	e.templates[t.ID] = t
	return nil
}

// UpdateTemplate 更新模板.
func (e *Engine) UpdateTemplate(id string, t *UserTemplate) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.templates[id]; !exists {
		return fmt.Errorf("模板不存在：%s", id)
	}
	t.ID = id
	e.templates[id] = t
	return nil
}

// DeleteTemplate 删除模板（不允许删除默认模板）.
func (e *Engine) DeleteTemplate(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	t, exists := e.templates[id]
	if !exists {
		return fmt.Errorf("模板不存在：%s", id)
	}
	if t.IsDefault {
		return fmt.Errorf("不能删除默认模板")
	}
	delete(e.templates, id)
	return nil
}

// ResolveTemplate 解析模板（通过 ID 或角色）.
func (e *Engine) ResolveTemplate(templateID string, role TemplateRole) (*UserTemplate, error) {
	if templateID != "" {
		return e.GetTemplate(templateID)
	}
	if role != "" {
		return e.GetTemplateByRole(role)
	}
	// 默认使用标准用户模板
	return e.GetTemplateByRole(RoleStandard)
}

// MapTemplateRoleToUserRole 将模板角色映射到系统角色.
func MapTemplateRoleToUserRole(role TemplateRole) string {
	switch role {
	case RoleAdmin:
		return "admin"
	case RoleStandard, RoleReadOnly:
		return "user"
	case RoleGuest:
		return "guest"
	default:
		return "user"
	}
}
