// Package permissioncenter 提供集中式RBAC权限管理功能
package permissioncenter

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrRoleNotFound 角色不存在.
	ErrRoleNotFound = errors.New("角色不存在")
	// ErrPermissionNotFound 权限不存在.
	ErrPermissionNotFound = errors.New("权限不存在")
	// ErrUserNotFound 用户不存在.
	ErrUserNotFound = errors.New("用户不存在")
	// ErrRoleAlreadyExists 角色已存在.
	ErrRoleAlreadyExists = errors.New("角色已存在")
	// ErrPermissionAlreadyExists 权限已存在.
	ErrPermissionAlreadyExists = errors.New("权限已存在")
	// ErrCircularInheritance 检测到循环继承.
	ErrCircularInheritance = errors.New("检测到循环继承")
	// ErrInvalidDelegation 无效的委托.
	ErrInvalidDelegation = errors.New("无效的委托")
	// ErrDelegationExpired 委托已过期.
	ErrDelegationExpired = errors.New("委托已过期")
	// ErrDelegationNotFound 委托不存在.
	ErrDelegationNotFound = errors.New("委托不存在")
	// ErrTempGrantExpired 临时授权已过期.
	ErrTempGrantExpired = errors.New("临时授权已过期")
	// ErrTempGrantNotFound 临时授权不存在.
	ErrTempGrantNotFound = errors.New("临时授权不存在")
	// ErrAccessDenied 访问被拒绝.
	ErrAccessDenied = errors.New("访问被拒绝")
	// ErrInvalidParams 无效的参数.
	ErrInvalidParams = errors.New("无效的参数")
)

// ========== 权限类型 ==========

// PermissionType 权限类型.
type PermissionType string

const (
	// PermTypeResource 资源权限.
	PermTypeResource PermissionType = "resource"
	// PermTypeAPI API权限.
	PermTypeAPI PermissionType = "api"
	// PermTypeData 数据权限.
	PermTypeData PermissionType = "data"
)

// ========== 访问操作 ==========

// AccessAction 访问操作.
type AccessAction string

const (
	// ActionRead 读取.
	ActionRead AccessAction = "read"
	// ActionWrite 写入.
	ActionWrite AccessAction = "write"
	// ActionDelete 删除.
	ActionDelete AccessAction = "delete"
	// ActionExecute 执行.
	ActionExecute AccessAction = "execute"
	// ActionManage 管理.
	ActionManage AccessAction = "manage"
)

// ========== 数据范围 ==========

// DataScope 数据权限范围.
type DataScope string

const (
	// ScopeAll 全部数据.
	ScopeAll DataScope = "all"
	// ScopeDepartment 部门数据.
	ScopeDepartment DataScope = "department"
	// ScopeTeam 团队数据.
	ScopeTeam DataScope = "team"
	// ScopeSelf 个人数据.
	ScopeSelf DataScope = "self"
	// ScopeCustom 自定义数据.
	ScopeCustom DataScope = "custom"
)

// ========== 审计操作类型 ==========

// AuditAction 审计操作类型.
type AuditAction string

const (
	// AuditRoleCreated 角色创建.
	AuditRoleCreated AuditAction = "role_created"
	// AuditRoleUpdated 角色更新.
	AuditRoleUpdated AuditAction = "role_updated"
	// AuditRoleDeleted 角色删除.
	AuditRoleDeleted AuditAction = "role_deleted"
	// AuditPermAssigned 权限分配.
	AuditPermAssigned AuditAction = "permission_assigned"
	// AuditPermRevoked 权限回收.
	AuditPermRevoked AuditAction = "permission_revoked"
	// AuditUserRoleAssigned 用户角色分配.
	AuditUserRoleAssigned AuditAction = "user_role_assigned"
	// AuditUserRoleRevoked 用户角色回收.
	AuditUserRoleRevoked AuditAction = "user_role_revoked"
	// AuditDelegationCreated 委托创建.
	AuditDelegationCreated AuditAction = "delegation_created"
	// AuditDelegationRevoked 委托撤销.
	AuditDelegationRevoked AuditAction = "delegation_revoked"
	// AuditTempGrantCreated 临时授权创建.
	AuditTempGrantCreated AuditAction = "temp_grant_created"
	// AuditTempGrantRevoked 临时授权撤销.
	AuditTempGrantRevoked AuditAction = "temp_grant_revoked"
	// AuditAccessChecked 访问检查.
	AuditAccessChecked AuditAction = "access_checked"
)

// ========== 核心数据结构 ==========

// Permission 权限定义.
type Permission struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Type        PermissionType `json:"type"`
	Resource    string         `json:"resource"`
	Action      AccessAction   `json:"action"`
	// DataScope 仅对数据权限有效.
	DataScope   DataScope      `json:"data_scope,omitempty"`
	// Conditions 额外条件（如IP白名单、时间范围等）.
	Conditions  map[string]string `json:"conditions,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// Role 角色定义.
type Role struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	// ParentID 父角色ID，用于权限继承.
	ParentID    string   `json:"parent_id,omitempty"`
	// Permissions 该角色拥有的权限ID列表.
	Permissions []string `json:"permissions"`
	// IsSystem 是否系统内置角色（不可删除）.
	IsSystem    bool     `json:"is_system"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UserRole 用户-角色关联.
type UserRole struct {
	UserID    string    `json:"user_id"`
	RoleID    string    `json:"role_id"`
	GrantedBy string    `json:"granted_by,omitempty"`
	GrantedAt time.Time `json:"granted_at"`
}

// Delegation 权限委托.
type Delegation struct {
	ID          string    `json:"id"`
	FromUserID  string    `json:"from_user_id"`
	ToUserID    string    `json:"to_user_id"`
	RoleID      string    `json:"role_id"`
	// DelegatedPermissions 委托的权限ID列表（为空表示委托角色的所有权限）.
	DelegatedPermissions []string `json:"delegated_permissions,omitempty"`
	Reason      string    `json:"reason,omitempty"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

// TempGrant 临时授权.
type TempGrant struct {
	ID          string       `json:"id"`
	UserID      string       `json:"user_id"`
	PermissionID string      `json:"permission_id"`
	// Resource 覆盖的资源（可选，为空使用权限定义的资源）.
	Resource    string       `json:"resource,omitempty"`
	// Conditions 覆盖的条件.
	Conditions  map[string]string `json:"conditions,omitempty"`
	Reason      string       `json:"reason,omitempty"`
	GrantedBy   string       `json:"granted_by"`
	StartTime   time.Time    `json:"start_time"`
	EndTime     time.Time    `json:"end_time"`
	IsActive    bool         `json:"is_active"`
	CreatedAt   time.Time    `json:"created_at"`
}

// AuditLog 权限审计日志.
type AuditLog struct {
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	UserID    string      `json:"user_id"`
	Action    AuditAction `json:"action"`
	TargetID  string      `json:"target_id,omitempty"`
	Details   string      `json:"details,omitempty"`
	// Before 变更前状态.
	Before    string      `json:"before,omitempty"`
	// After 变更后状态.
	After     string      `json:"after,omitempty"`
	IPAddress string      `json:"ip_address,omitempty"`
	UserAgent string      `json:"user_agent,omitempty"`
}

// AccessRequest 访问检查请求.
type AccessRequest struct {
	UserID   string       `json:"user_id"`
	Resource string       `json:"resource"`
	Action   AccessAction `json:"action"`
	// Context 额外上下文（如请求IP、时间等）.
	Context    map[string]string `json:"context,omitempty"`
}

// AccessResult 访问检查结果.
type AccessResult struct {
	Allowed    bool     `json:"allowed"`
	Reason     string   `json:"reason,omitempty"`
	// MatchedPermissions 匹配到的权限ID列表.
	MatchedPermissions []string `json:"matched_permissions,omitempty"`
	// AppliedScopes 应用的数据范围.
	AppliedScopes []DataScope `json:"applied_scopes,omitempty"`
	// IsDelegated 是否通过委托授权.
	IsDelegated bool   `json:"is_delegated"`
	// IsTempGrant 是否通过临时授权.
	IsTempGrant bool   `json:"is_temp_grant"`
}

// QueryParams 通用查询参数.
type QueryParams struct {
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// RoleListResult 角色列表结果.
type RoleListResult struct {
	Roles  []*Role `json:"roles"`
	Total  int     `json:"total"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
}

// PermissionListResult 权限列表结果.
type PermissionListResult struct {
	Permissions []*Permission `json:"permissions"`
	Total       int           `json:"total"`
	Limit       int           `json:"limit"`
	Offset      int           `json:"offset"`
}

// AuditLogListResult 审计日志列表结果.
type AuditLogListResult struct {
	Logs   []*AuditLog `json:"logs"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

// UserPermissionSummary 用户权限汇总.
type UserPermissionSummary struct {
	UserID          string        `json:"user_id"`
	Roles           []*Role       `json:"roles"`
	DirectPermissions []*Permission `json:"direct_permissions"`
	DelegatedPermissions []*DelegatedPermission `json:"delegated_permissions"`
	TempPermissions []*TempGrant  `json:"temp_permissions"`
	AllPermissions  []*Permission `json:"all_permissions"`
}

// DelegatedPermission 委托的权限信息.
type DelegatedPermission struct {
	Delegation *Delegation  `json:"delegation"`
	Permission *Permission  `json:"permission"`
}

// PermissionCheckResult 权限检查结果（批量检查用）.
type PermissionCheckResult struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Allowed  bool   `json:"allowed"`
	Reason   string `json:"reason,omitempty"`
}
