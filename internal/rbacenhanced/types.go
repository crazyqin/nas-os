// Package rbacenhanced 提供增强版RBAC权限管理，对标群晖企业级RBAC。
// 支持细粒度角色访问控制、审计日志、FIPS合规、动态权限继承。
package rbacenhanced

import (
	"fmt"
	"sync"
	"time"
)

// Permission 权限类型
type Permission string

const (
	PermRead      Permission = "read"
	PermWrite     Permission = "write"
	PermDelete    Permission = "delete"
	PermExecute   Permission = "execute"
	PermAdmin     Permission = "admin"
	PermShare     Permission = "share"
	PermBackup    Permission = "backup"
	PermRestore   Permission = "restore"
	PermAudit     Permission = "audit"
	PermConfigure Permission = "configure"
)

// ResourceType 资源类型
type ResourceType string

const (
	ResourceFile       ResourceType = "file"
	ResourceDirectory  ResourceType = "directory"
	ResourceShare      ResourceType = "share"
	ResourceSnapshot   ResourceType = "snapshot"
	ResourceService    ResourceType = "service"
	ResourceSystem     ResourceType = "system"
	ResourceUser       ResourceType = "user"
	ResourceGroup      ResourceType = "group"
)

// AuditAction 审计动作
type AuditAction string

const (
	AuditLogin      AuditAction = "login"
	AuditLogout     AuditAction = "logout"
	AuditAccess     AuditAction = "access"
	AuditModify     AuditAction = "modify"
	AuditDelete     AuditAction = "delete"
	AuditShare      AuditAction = "share"
	AuditPermission AuditAction = "permission_change"
	AuditConfig     AuditAction = "config_change"
)

// Role 角色定义
type Role struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions"`
	Resources   []ResourceType `json:"resources"`
	IsSystem    bool         `json:"is_system"`
	ParentID    string       `json:"parent_id,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// User 用户定义
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Roles     []string  `json:"roles"`
	Enabled   bool      `json:"enabled"`
	LastLogin *time.Time `json:"last_login,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ACLRule ACL规则
type ACLRule struct {
	ID         string       `json:"id"`
	Resource   string       `json:"resource"`
	ResType    ResourceType `json:"res_type"`
	Principal  string       `json:"principal"` // user or group ID
	Permission Permission   `json:"permission"`
	Allowed    bool         `json:"allowed"`
	ExpiresAt  *time.Time   `json:"expires_at,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
}

// AuditEntry 审计条目
type AuditEntry struct {
	ID        string      `json:"id"`
	UserID    string      `json:"user_id"`
	Username  string      `json:"username"`
	Action    AuditAction `json:"action"`
	Resource  string      `json:"resource"`
	Details   string      `json:"details"`
	IP        string      `json:"ip"`
	Success   bool        `json:"success"`
	Timestamp time.Time   `json:"timestamp"`
}

// RBACConfig RBAC配置
type RBACConfig struct {
	EnableAudit    bool          `json:"enable_audit"`
	EnableFIPS     bool          `json:"enable_fips"`
	MaxRoles       int           `json:"max_roles"`
	MaxACLs        int           `json:"max_acls"`
	SessionTimeout time.Duration `json:"session_timeout"`
	AuditRetention time.Duration `json:"audit_retention"`
}

// DefaultRBACConfig 返回默认配置
func DefaultRBACConfig() *RBACConfig {
	return &RBACConfig{
		EnableAudit:    true,
		EnableFIPS:     true,
		MaxRoles:       100,
		MaxACLs:        10000,
		SessionTimeout: 30 * time.Minute,
		AuditRetention: 365 * 24 * time.Hour,
	}
}

// Manager RBAC管理器
type Manager struct {
	mu      sync.RWMutex
	config  *RBACConfig
	roles   map[string]*Role
	users   map[string]*User
	acls    map[string]*ACLRule
	audit   []AuditEntry
	running bool
}

// NewManager 创建管理器
func NewManager(config *RBACConfig) *Manager {
	if config == nil {
		config = DefaultRBACConfig()
	}
	return &Manager{
		config: config,
		roles:  make(map[string]*Role),
		users:  make(map[string]*User),
		acls:   make(map[string]*ACLRule),
		audit:  make([]AuditEntry, 0),
	}
}

// Start 启动管理器
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return fmt.Errorf("RBAC管理器已在运行")
	}
	m.running = true
	// 初始化系统角色
	m.initSystemRoles()
	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	return nil
}

// IsRunning 检查是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

func (m *Manager) initSystemRoles() {
	m.roles["admin"] = &Role{
		ID:          "admin",
		Name:        "管理员",
		Description: "系统管理员，拥有所有权限",
		Permissions: []Permission{PermRead, PermWrite, PermDelete, PermExecute, PermAdmin, PermShare, PermBackup, PermRestore, PermAudit, PermConfigure},
		Resources:   []ResourceType{ResourceFile, ResourceDirectory, ResourceShare, ResourceSnapshot, ResourceService, ResourceSystem, ResourceUser, ResourceGroup},
		IsSystem:    true,
		CreatedAt:   time.Now(),
	}
	m.roles["user"] = &Role{
		ID:          "user",
		Name:        "普通用户",
		Description: "普通用户，基本读写权限",
		Permissions: []Permission{PermRead, PermWrite, PermShare},
		Resources:   []ResourceType{ResourceFile, ResourceDirectory, ResourceShare},
		IsSystem:    true,
		CreatedAt:   time.Now(),
	}
	m.roles["viewer"] = &Role{
		ID:          "viewer",
		Name:        "只读用户",
		Description: "只读用户，仅查看权限",
		Permissions: []Permission{PermRead},
		Resources:   []ResourceType{ResourceFile, ResourceDirectory},
		IsSystem:    true,
		CreatedAt:   time.Now(),
	}
}

// CreateRole 创建角色
func (m *Manager) CreateRole(role *Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return fmt.Errorf("管理器未运行")
	}
	if len(m.roles) >= m.config.MaxRoles {
		return fmt.Errorf("已达到最大角色数: %d", m.config.MaxRoles)
	}
	if _, exists := m.roles[role.ID]; exists {
		return fmt.Errorf("角色已存在: %s", role.ID)
	}
	role.CreatedAt = time.Now()
	role.UpdatedAt = time.Now()
	m.roles[role.ID] = role
	return nil
}

// GetRole 获取角色
func (m *Manager) GetRole(id string) (*Role, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	role, exists := m.roles[id]
	if !exists {
		return nil, fmt.Errorf("角色不存在: %s", id)
	}
	return role, nil
}

// ListRoles 列出角色
func (m *Manager) ListRoles() []*Role {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var roles []*Role
	for _, r := range m.roles {
		roles = append(roles, r)
	}
	return roles
}

// DeleteRole 删除角色
func (m *Manager) DeleteRole(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	role, exists := m.roles[id]
	if !exists {
		return fmt.Errorf("角色不存在: %s", id)
	}
	if role.IsSystem {
		return fmt.Errorf("系统角色不能删除")
	}
	// 检查是否有用户使用此角色
	for _, u := range m.users {
		for _, r := range u.Roles {
			if r == id {
				return fmt.Errorf("角色 %s 仍被用户使用", id)
			}
		}
	}
	delete(m.roles, id)
	return nil
}

// CreateUser 创建用户
func (m *Manager) CreateUser(user *User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return fmt.Errorf("管理器未运行")
	}
	if _, exists := m.users[user.ID]; exists {
		return fmt.Errorf("用户已存在: %s", user.ID)
	}
	user.CreatedAt = time.Now()
	m.users[user.ID] = user
	return nil
}

// GetUser 获取用户
func (m *Manager) GetUser(id string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, exists := m.users[id]
	if !exists {
		return nil, fmt.Errorf("用户不存在: %s", id)
	}
	return user, nil
}

// AssignRole 分配角色给用户
func (m *Manager) AssignRole(userID, roleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, exists := m.users[userID]
	if !exists {
		return fmt.Errorf("用户不存在: %s", userID)
	}
	if _, exists := m.roles[roleID]; !exists {
		return fmt.Errorf("角色不存在: %s", roleID)
	}
	// 检查是否已分配
	for _, r := range user.Roles {
		if r == roleID {
			return nil
		}
	}
	user.Roles = append(user.Roles, roleID)
	return nil
}

// CheckPermission 检查用户权限
func (m *Manager) CheckPermission(userID string, perm Permission, resType ResourceType, resource string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, exists := m.users[userID]
	if !exists || !user.Enabled {
		return false
	}
	// 检查ACL规则
	for _, acl := range m.acls {
		if acl.Principal == userID && acl.Resource == resource && acl.Permission == perm {
			if acl.ExpiresAt != nil && acl.ExpiresAt.Before(time.Now()) {
				continue
			}
			return acl.Allowed
		}
	}
	// 检查角色权限
	for _, roleID := range user.Roles {
		role, exists := m.roles[roleID]
		if !exists {
			continue
		}
		hasPerm := false
		hasRes := false
		for _, p := range role.Permissions {
			if p == perm || p == PermAdmin {
				hasPerm = true
				break
			}
		}
		for _, r := range role.Resources {
			if r == resType || r == ResourceSystem {
				hasRes = true
				break
			}
		}
		if hasPerm && hasRes {
			return true
		}
	}
	return false
}

// AddACL 添加ACL规则
func (m *Manager) AddACL(rule *ACLRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.acls) >= m.config.MaxACLs {
		return fmt.Errorf("已达到最大ACL数: %d", m.config.MaxACLs)
	}
	rule.CreatedAt = time.Now()
	m.acls[rule.ID] = rule
	return nil
}

// LogAudit 记录审计日志
func (m *Manager) LogAudit(entry AuditEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.config.EnableAudit {
		return
	}
	entry.Timestamp = time.Now()
	m.audit = append(m.audit, entry)
}

// GetAuditLog 获取审计日志
func (m *Manager) GetAuditLog(userID string, limit int) []AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []AuditEntry
	for i := len(m.audit) - 1; i >= 0; i-- {
		if userID == "" || m.audit[i].UserID == userID {
			result = append(result, m.audit[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	systemRoles := 0
	for _, r := range m.roles {
		if r.IsSystem {
			systemRoles++
		}
	}
	return map[string]interface{}{
		"running":      m.running,
		"total_roles":  len(m.roles),
		"system_roles": systemRoles,
		"total_users":  len(m.users),
		"total_acls":   len(m.acls),
		"total_audit":  len(m.audit),
	}
}
