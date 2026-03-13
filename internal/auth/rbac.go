package auth

import (
	"errors"
	"sync"
	"time"
)

// Role 用户角色
type Role string

const (
	RoleAdmin  Role = "admin"  // 管理员：全部权限
	RoleUser   Role = "user"   // 普通用户：受限访问
	RoleGuest  Role = "guest"  // 访客：只读访问
	RoleSystem Role = "system" // 系统服务账号
)

// Permission 权限定义
type Permission struct {
	Resource string `json:"resource"` // 资源类型
	Action   string `json:"action"`   // 操作：read, write, delete, admin
}

// Resource 资源类型
type Resource string

const (
	ResourceVolume    Resource = "volume"    // 存储卷
	ResourceShare     Resource = "share"     // 共享目录
	ResourceUser      Resource = "user"      // 用户管理
	ResourceGroup     Resource = "group"     // 用户组
	ResourceSystem    Resource = "system"    // 系统设置
	ResourceContainer Resource = "container" // 容器管理
	ResourceVM        Resource = "vm"        // 虚拟机
	ResourceFile      Resource = "file"      // 文件管理
	ResourceSnapshot  Resource = "snapshot"  // 快照
)

// Action 操作类型
type Action string

const (
	ActionRead   Action = "read"   // 读取
	ActionWrite  Action = "write"  // 写入
	ActionDelete Action = "delete" // 删除
	ActionAdmin  Action = "admin"  // 管理
	ActionExec   Action = "exec"   // 执行（容器/VM）
)

// RoleDefinition 角色定义
type RoleDefinition struct {
	Name        Role         `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions"`
	Inherits    []Role       `json:"inherits,omitempty"` // 继承的角色
}

// RBACManager RBAC 管理器
type RBACManager struct {
	mu           sync.RWMutex
	roles        map[Role]*RoleDefinition // 角色定义
	userRoles    map[string][]Role        // userID -> roles
	groupRoles   map[string][]Role        // groupID -> roles
	resourceACLs map[string]*ResourceACL  // resourceID -> ACL
	defaultRole  Role
	sessionCache map[string]*SessionCache // token -> cached permissions
	cacheExpiry  time.Duration
}

// ResourceACL 资源访问控制列表
type ResourceACL struct {
	ResourceID   string     `json:"resource_id"`
	ResourceType Resource   `json:"resource_type"`
	OwnerID      string     `json:"owner_id"`
	GroupACLs    []GroupACL `json:"group_acls,omitempty"`
	UserACLs     []UserACL  `json:"user_acls,omitempty"`
	Inherit      bool       `json:"inherit"` // 是否继承父级权限
	ParentID     string     `json:"parent_id,omitempty"`
}

// GroupACL 用户组 ACL
type GroupACL struct {
	GroupID     string       `json:"group_id"`
	Permissions []Permission `json:"permissions"`
}

// UserACL 用户 ACL
type UserACL struct {
	UserID      string       `json:"user_id"`
	Permissions []Permission `json:"permissions"`
}

// SessionCache 会话缓存
type SessionCache struct {
	UserID      string
	Roles       []Role
	Permissions []Permission
	ExpiresAt   time.Time
}

// NewRBACManager 创建 RBAC 管理器
func NewRBACManager() *RBACManager {
	mgr := &RBACManager{
		roles:        make(map[Role]*RoleDefinition),
		userRoles:    make(map[string][]Role),
		groupRoles:   make(map[string][]Role),
		resourceACLs: make(map[string]*ResourceACL),
		defaultRole:  RoleGuest,
		sessionCache: make(map[string]*SessionCache),
		cacheExpiry:  5 * time.Minute,
	}

	// 初始化内置角色
	mgr.initBuiltInRoles()

	return mgr
}

// initBuiltInRoles 初始化内置角色
func (m *RBACManager) initBuiltInRoles() {
	// Admin 角色 - 全部权限
	m.roles[RoleAdmin] = &RoleDefinition{
		Name:        RoleAdmin,
		Description: "系统管理员，拥有所有权限",
		Permissions: []Permission{
			{Resource: string(ResourceVolume), Action: string(ActionRead)},
			{Resource: string(ResourceVolume), Action: string(ActionWrite)},
			{Resource: string(ResourceVolume), Action: string(ActionDelete)},
			{Resource: string(ResourceVolume), Action: string(ActionAdmin)},
			{Resource: string(ResourceShare), Action: string(ActionRead)},
			{Resource: string(ResourceShare), Action: string(ActionWrite)},
			{Resource: string(ResourceShare), Action: string(ActionDelete)},
			{Resource: string(ResourceShare), Action: string(ActionAdmin)},
			{Resource: string(ResourceUser), Action: string(ActionRead)},
			{Resource: string(ResourceUser), Action: string(ActionWrite)},
			{Resource: string(ResourceUser), Action: string(ActionDelete)},
			{Resource: string(ResourceUser), Action: string(ActionAdmin)},
			{Resource: string(ResourceGroup), Action: string(ActionRead)},
			{Resource: string(ResourceGroup), Action: string(ActionWrite)},
			{Resource: string(ResourceGroup), Action: string(ActionDelete)},
			{Resource: string(ResourceGroup), Action: string(ActionAdmin)},
			{Resource: string(ResourceSystem), Action: string(ActionRead)},
			{Resource: string(ResourceSystem), Action: string(ActionWrite)},
			{Resource: string(ResourceSystem), Action: string(ActionAdmin)},
			{Resource: string(ResourceContainer), Action: string(ActionRead)},
			{Resource: string(ResourceContainer), Action: string(ActionWrite)},
			{Resource: string(ResourceContainer), Action: string(ActionDelete)},
			{Resource: string(ResourceContainer), Action: string(ActionExec)},
			{Resource: string(ResourceVM), Action: string(ActionRead)},
			{Resource: string(ResourceVM), Action: string(ActionWrite)},
			{Resource: string(ResourceVM), Action: string(ActionDelete)},
			{Resource: string(ResourceVM), Action: string(ActionExec)},
			{Resource: string(ResourceFile), Action: string(ActionRead)},
			{Resource: string(ResourceFile), Action: string(ActionWrite)},
			{Resource: string(ResourceFile), Action: string(ActionDelete)},
			{Resource: string(ResourceSnapshot), Action: string(ActionRead)},
			{Resource: string(ResourceSnapshot), Action: string(ActionWrite)},
			{Resource: string(ResourceSnapshot), Action: string(ActionDelete)},
		},
	}

	// User 角色 - 普通用户权限
	m.roles[RoleUser] = &RoleDefinition{
		Name:        RoleUser,
		Description: "普通用户，受限访问",
		Permissions: []Permission{
			{Resource: string(ResourceVolume), Action: string(ActionRead)},
			{Resource: string(ResourceShare), Action: string(ActionRead)},
			{Resource: string(ResourceShare), Action: string(ActionWrite)},
			{Resource: string(ResourceUser), Action: string(ActionRead)}, // 查看自己信息
			{Resource: string(ResourceContainer), Action: string(ActionRead)},
			{Resource: string(ResourceContainer), Action: string(ActionWrite)},
			{Resource: string(ResourceVM), Action: string(ActionRead)},
			{Resource: string(ResourceFile), Action: string(ActionRead)},
			{Resource: string(ResourceFile), Action: string(ActionWrite)},
			{Resource: string(ResourceSnapshot), Action: string(ActionRead)},
		},
	}

	// Guest 角色 - 访客权限（只读）
	m.roles[RoleGuest] = &RoleDefinition{
		Name:        RoleGuest,
		Description: "访客，只读访问",
		Permissions: []Permission{
			{Resource: string(ResourceShare), Action: string(ActionRead)},
			{Resource: string(ResourceFile), Action: string(ActionRead)},
		},
	}

	// System 角色 - 系统服务账号
	m.roles[RoleSystem] = &RoleDefinition{
		Name:        RoleSystem,
		Description: "系统服务账号",
		Permissions: []Permission{
			{Resource: string(ResourceSystem), Action: string(ActionRead)},
			{Resource: string(ResourceSystem), Action: string(ActionWrite)},
		},
	}
}

// AddRole 添加自定义角色
func (m *RBACManager) AddRole(role Role, description string, permissions []Permission, inherits []Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.roles[role] = &RoleDefinition{
		Name:        role,
		Description: description,
		Permissions: permissions,
		Inherits:    inherits,
	}

	return nil
}

// AssignRoleToUser 给用户分配角色
func (m *RBACManager) AssignRoleToUser(userID string, role Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.roles[role]; !exists {
		return ErrRoleNotFound
	}

	m.userRoles[userID] = append(m.userRoles[userID], role)
	return nil
}

// AssignRoleToGroup 给用户组分配角色
func (m *RBACManager) AssignRoleToGroup(groupID string, role Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.roles[role]; !exists {
		return ErrRoleNotFound
	}

	m.groupRoles[groupID] = append(m.groupRoles[groupID], role)
	return nil
}

// CheckPermission 检查用户是否有指定权限
func (m *RBACManager) CheckPermission(userID string, userGroups []string, resource Resource, action Action) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 收集用户所有角色
	userRoles := m.userRoles[userID]
	for _, groupID := range userGroups {
		userRoles = append(userRoles, m.groupRoles[groupID]...)
	}

	// 如果没有角色，使用默认角色
	if len(userRoles) == 0 {
		userRoles = []Role{m.defaultRole}
	}

	// 检查每个角色的权限
	for _, role := range userRoles {
		roleDef, exists := m.roles[role]
		if !exists {
			continue
		}

		// 检查角色直接权限
		for _, perm := range roleDef.Permissions {
			if perm.Resource == string(resource) && perm.Action == string(action) {
				return true
			}
		}

		// 检查继承角色的权限
		for _, inheritedRole := range roleDef.Inherits {
			inheritedDef, exists := m.roles[inheritedRole]
			if !exists {
				continue
			}
			for _, perm := range inheritedDef.Permissions {
				if perm.Resource == string(resource) && perm.Action == string(action) {
					return true
				}
			}
		}
	}

	// 检查资源 ACL
	if acl, exists := m.resourceACLs[string(resource)+":"+userID]; exists {
		for _, userACL := range acl.UserACLs {
			if userACL.UserID == userID {
				for _, perm := range userACL.Permissions {
					if perm.Resource == string(resource) && perm.Action == string(action) {
						return true
					}
				}
			}
		}
	}

	return false
}

// GetPermissions 获取用户所有权限
func (m *RBACManager) GetPermissions(userID string, userGroups []string) []Permission {
	m.mu.RLock()
	defer m.mu.RUnlock()

	permissionsMap := make(map[string]bool)
	var result []Permission

	// 收集用户所有角色
	userRoles := m.userRoles[userID]
	for _, groupID := range userGroups {
		userRoles = append(userRoles, m.groupRoles[groupID]...)
	}

	// 如果没有角色，使用默认角色
	if len(userRoles) == 0 {
		userRoles = []Role{m.defaultRole}
	}

	// 收集所有权限
	for _, role := range userRoles {
		roleDef, exists := m.roles[role]
		if !exists {
			continue
		}

		for _, perm := range roleDef.Permissions {
			key := perm.Resource + ":" + perm.Action
			if !permissionsMap[key] {
				permissionsMap[key] = true
				result = append(result, perm)
			}
		}
	}

	return result
}

// SetResourceACL 设置资源 ACL
func (m *RBACManager) SetResourceACL(resourceID string, resourceType Resource, ownerID string, groupACLs []GroupACL, userACLs []UserACL) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.resourceACLs[resourceID] = &ResourceACL{
		ResourceID:   resourceID,
		ResourceType: resourceType,
		OwnerID:      ownerID,
		GroupACLs:    groupACLs,
		UserACLs:     userACLs,
	}
}

// CacheSession 缓存会话权限
func (m *RBACManager) CacheSession(token string, userID string, groups []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	permissions := m.GetPermissions(userID, groups)
	roles := m.userRoles[userID]

	m.sessionCache[token] = &SessionCache{
		UserID:      userID,
		Roles:       roles,
		Permissions: permissions,
		ExpiresAt:   time.Now().Add(m.cacheExpiry),
	}
}

// GetCachedSession 获取缓存的会话
func (m *RBACManager) GetCachedSession(token string) *SessionCache {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessionCache[token]
	if !exists {
		return nil
	}

	if time.Now().After(session.ExpiresAt) {
		delete(m.sessionCache, token)
		return nil
	}

	return session
}

// InvalidateSession 使会话缓存失效
func (m *RBACManager) InvalidateSession(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessionCache, token)
}

// CleanupExpiredSessions 清理过期会话
func (m *RBACManager) CleanupExpiredSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for token, session := range m.sessionCache {
		if now.After(session.ExpiresAt) {
			delete(m.sessionCache, token)
		}
	}
}

// GetRoles 获取所有角色定义
func (m *RBACManager) GetRoles() []*RoleDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*RoleDefinition, 0, len(m.roles))
	for _, role := range m.roles {
		result = append(result, role)
	}

	return result
}

// GetUserRoles 获取用户的所有角色
func (m *RBACManager) GetUserRoles(userID string) []Role {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]Role{}, m.userRoles[userID]...)
}

// RemoveUserRole 移除用户角色
func (m *RBACManager) RemoveUserRole(userID string, role Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	roles := m.userRoles[userID]
	for i, r := range roles {
		if r == role {
			m.userRoles[userID] = append(roles[:i], roles[i+1:]...)
			return nil
		}
	}

	return ErrRoleNotFound
}

// 错误定义
var (
	ErrRoleNotFound     = errors.New("角色不存在")
	ErrPermissionDenied = errors.New("权限不足")
	ErrInvalidResource  = errors.New("无效的资源类型")
	ErrInvalidAction    = errors.New("无效的操作类型")
)
