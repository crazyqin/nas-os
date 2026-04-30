// Package permtemplate 提供用户权限模板系统
// 对标群晖简化用户管理，减少配置复杂度
package permtemplate

import (
	"fmt"
	"sync"
	"time"
)

// Permission 权限定义
type Permission struct {
	Resource string `json:"resource"` // resource type: share|app|service|admin
	Access   string `json:"access"`   // read|write|admin|none
}

// PermissionTemplate 权限模板
type PermissionTemplate struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Category    string       `json:"category"` // home|office|media|developer|admin
	IsBuiltin   bool         `json:"isBuiltin"`
	Permissions []Permission `json:"permissions"`
	Quotas      *UserQuota   `json:"quotas,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// UserQuota 用户配额
type UserQuota struct {
	MaxStorageBytes  int64 `json:"maxStorageBytes"`  // 最大存储空间
	MaxFiles         int64 `json:"maxFiles"`         // 最大文件数
	MaxBandwidth     int64 `json:"maxBandwidthMbps"` // 最大带宽(Mbps)
	MaxSessions      int   `json:"maxSessions"`      // 最大并发会话
}

// TemplateApplication 模板应用记录
type TemplateApplication struct {
	TemplateID string    `json:"templateId"`
	UserID     string    `json:"userId"`
	AppliedAt  time.Time `json:"appliedAt"`
	AppliedBy  string    `json:"appliedBy"`
}

// TemplateManager 权限模板管理器
type TemplateManager struct {
	templates    map[string]*PermissionTemplate
	applications []TemplateApplication
	mu           sync.RWMutex
}

// NewTemplateManager 创建模板管理器
func NewTemplateManager() *TemplateManager {
	m := &TemplateManager{
		templates: make(map[string]*PermissionTemplate),
	}
	m.registerBuiltinTemplates()
	return m
}

// Create 创建模板
func (m *TemplateManager) Create(tmpl *PermissionTemplate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if tmpl.ID == "" {
		return fmt.Errorf("template ID is required")
	}
	if _, exists := m.templates[tmpl.ID]; exists {
		return fmt.Errorf("template %s already exists", tmpl.ID)
	}
	tmpl.CreatedAt = time.Now()
	tmpl.UpdatedAt = time.Now()
	m.templates[tmpl.ID] = tmpl
	return nil
}

// Update 更新模板
func (m *TemplateManager) Update(tmpl *PermissionTemplate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.templates[tmpl.ID]
	if !ok {
		return fmt.Errorf("template %s not found", tmpl.ID)
	}
	if existing.IsBuiltin {
		return fmt.Errorf("cannot modify builtin template")
	}
	tmpl.UpdatedAt = time.Now()
	m.templates[tmpl.ID] = tmpl
	return nil
}

// Delete 删除模板
func (m *TemplateManager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tmpl, ok := m.templates[id]
	if !ok {
		return fmt.Errorf("template %s not found", id)
	}
	if tmpl.IsBuiltin {
		return fmt.Errorf("cannot delete builtin template")
	}
	delete(m.templates, id)
	return nil
}

// Get 获取模板
func (m *TemplateManager) Get(id string) (*PermissionTemplate, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tmpl, ok := m.templates[id]
	return tmpl, ok
}

// List 列出所有模板
func (m *TemplateManager) List(category string) []*PermissionTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*PermissionTemplate
	for _, tmpl := range m.templates {
		if category != "" && tmpl.Category != category {
			continue
		}
		result = append(result, tmpl)
	}
	return result
}

// Apply 应用模板到用户
func (m *TemplateManager) Apply(templateID, userID, appliedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.templates[templateID]; !ok {
		return fmt.Errorf("template %s not found", templateID)
	}
	m.applications = append(m.applications, TemplateApplication{
		TemplateID: templateID,
		UserID:     userID,
		AppliedAt:  time.Now(),
		AppliedBy:  appliedBy,
	})
	return nil
}

// GetApplications 获取应用记录
func (m *TemplateManager) GetApplications(userID string) []TemplateApplication {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []TemplateApplication
	for _, app := range m.applications {
		if userID != "" && app.UserID != userID {
			continue
		}
		result = append(result, app)
	}
	return result
}

// registerBuiltinTemplates 注册内置模板
func (m *TemplateManager) registerBuiltinTemplates() {
	m.templates["home_user"] = &PermissionTemplate{
		ID:          "home_user",
		Name:        "家庭用户",
		Description: "基础家庭用户权限，可访问个人文件和媒体",
		Category:    "home",
		IsBuiltin:   true,
		Permissions: []Permission{
			{Resource: "share:home", Access: "write"},
			{Resource: "share:media", Access: "read"},
			{Resource: "share:photos", Access: "write"},
			{Resource: "app:photos", Access: "read"},
			{Resource: "app:drive", Access: "write"},
			{Resource: "service:ftp", Access: "none"},
		},
		Quotas: &UserQuota{
			MaxStorageBytes:  500 * 1024 * 1024 * 1024, // 500GB
			MaxFiles:         100000,
			MaxBandwidth:     100,
			MaxSessions:      3,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.templates["office_user"] = &PermissionTemplate{
		ID:          "office_user",
		Name:        "办公用户",
		Description: "办公环境权限，可访问共享文档和协作工具",
		Category:    "office",
		IsBuiltin:   true,
		Permissions: []Permission{
			{Resource: "share:documents", Access: "write"},
			{Resource: "share:projects", Access: "write"},
			{Resource: "share:public", Access: "read"},
			{Resource: "app:office", Access: "write"},
			{Resource: "app:drive", Access: "write"},
			{Resource: "app:email", Access: "write"},
			{Resource: "service:webdav", Access: "read"},
		},
		Quotas: &UserQuota{
			MaxStorageBytes:  200 * 1024 * 1024 * 1024, // 200GB
			MaxFiles:         50000,
			MaxBandwidth:     200,
			MaxSessions:      5,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.templates["media_user"] = &PermissionTemplate{
		ID:          "media_user",
		Name:        "媒体用户",
		Description: "媒体管理权限，可管理视频、音乐、照片",
		Category:    "media",
		IsBuiltin:   true,
		Permissions: []Permission{
			{Resource: "share:video", Access: "write"},
			{Resource: "share:music", Access: "write"},
			{Resource: "share:photos", Access: "write"},
			{Resource: "app:media_server", Access: "read"},
			{Resource: "app:photos", Access: "write"},
			{Resource: "app:transcoding", Access: "read"},
		},
		Quotas: &UserQuota{
			MaxStorageBytes:  2 * 1024 * 1024 * 1024 * 1024, // 2TB
			MaxFiles:         500000,
			MaxBandwidth:     500,
			MaxSessions:      10,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.templates["developer"] = &PermissionTemplate{
		ID:          "developer",
		Name:        "开发者",
		Description: "开发者权限，可访问开发工具和Docker",
		Category:    "developer",
		IsBuiltin:   true,
		Permissions: []Permission{
			{Resource: "share:code", Access: "write"},
			{Resource: "share:data", Access: "write"},
			{Resource: "app:docker", Access: "write"},
			{Resource: "app:terminal", Access: "write"},
			{Resource: "service:ssh", Access: "write"},
			{Resource: "service:git", Access: "write"},
		},
		Quotas: &UserQuota{
			MaxStorageBytes:  1 * 1024 * 1024 * 1024 * 1024, // 1TB
			MaxFiles:         200000,
			MaxBandwidth:     1000,
			MaxSessions:      10,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.templates["admin"] = &PermissionTemplate{
		ID:          "admin",
		Name:        "管理员",
		Description: "完全管理权限",
		Category:    "admin",
		IsBuiltin:   true,
		Permissions: []Permission{
			{Resource: "*", Access: "admin"},
		},
		Quotas: &UserQuota{
			MaxStorageBytes:  0, // 无限制
			MaxFiles:         0,
			MaxBandwidth:     0,
			MaxSessions:      0,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
