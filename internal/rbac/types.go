// Package rbac 提供基于角色的访问控制 (Role-Based Access Control)
// 实现最小权限原则，支持用户组权限继承
package rbac

import (
	"time"
)

// ========== 角色定义 ==========

// Role 系统角色
type Role string

const (
	// RoleAdmin 管理员 - 完全控制权限
	RoleAdmin Role = "admin"
	// RoleOperator 运维员 - 系统操作权限，无用户管理
	RoleOperator Role = "operator"
	// RoleReadOnly 只读用户 - 只能查看，不能修改
	RoleReadOnly Role = "readonly"
	// RoleGuest 访客 - 最小权限，仅基本访问
	RoleGuest Role = "guest"
)

// RoleInfo 角色信息
type RoleInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	Priority    int      `json:"priority"` // 角色优先级，数字越大权限越高
}

// DefaultRoles 默认角色定义
var DefaultRoles = map[Role]RoleInfo{
	RoleAdmin: {
		Name:        "admin",
		Description: "系统管理员，拥有完全控制权限",
		Permissions: []string{"*:*"}, // 所有权限
		Priority:    100,
	},
	RoleOperator: {
		Name:        "operator",
		Description: "运维员，可以操作系统但不能管理用户",
		Permissions: []string{
			"system:read", "system:write",
			"storage:read", "storage:write",
			"share:read", "share:write",
			"network:read", "network:write",
			"service:read", "service:write",
			"backup:read", "backup:write",
			"log:read",
			"monitor:read",
		},
		Priority: 75,
	},
	RoleReadOnly: {
		Name:        "readonly",
		Description: "只读用户，只能查看系统状态",
		Permissions: []string{
			"system:read",
			"storage:read",
			"share:read",
			"network:read",
			"service:read",
			"log:read",
			"monitor:read",
		},
		Priority: 50,
	},
	RoleGuest: {
		Name:        "guest",
		Description: "访客用户，最小权限",
		Permissions: []string{
			"system:read",
		},
		Priority: 25,
	},
}

// ========== 权限模型 ==========

// Permission 权限定义（资源:操作）
type Permission struct {
	Resource   string `json:"resource"`   // 资源类型
	Action     string `json:"action"`     // 操作类型
	Desc       string `json:"desc"`       // 描述
	DependsOn  string `json:"depends_on"` // 依赖的权限（如 write 依赖 read）
	IsWildcard bool   `json:"is_wildcard"`
}

// 预定义权限
var (
	// 系统管理
	PermSystemRead  = Permission{Resource: "system", Action: "read", Desc: "查看系统信息"}
	PermSystemWrite = Permission{Resource: "system", Action: "write", Desc: "修改系统配置", DependsOn: "system:read"}
	PermSystemAdmin = Permission{Resource: "system", Action: "admin", Desc: "系统管理操作", DependsOn: "system:write"}

	// 用户管理
	PermUserRead  = Permission{Resource: "user", Action: "read", Desc: "查看用户信息"}
	PermUserWrite = Permission{Resource: "user", Action: "write", Desc: "创建/修改用户", DependsOn: "user:read"}
	PermUserAdmin = Permission{Resource: "user", Action: "admin", Desc: "删除用户/修改角色", DependsOn: "user:write"}

	// 存储管理
	PermStorageRead  = Permission{Resource: "storage", Action: "read", Desc: "查看存储状态"}
	PermStorageWrite = Permission{Resource: "storage", Action: "write", Desc: "管理存储池/卷", DependsOn: "storage:read"}
	PermStorageAdmin = Permission{Resource: "storage", Action: "admin", Desc: "删除存储/格式化", DependsOn: "storage:write"}

	// 共享管理
	PermShareRead  = Permission{Resource: "share", Action: "read", Desc: "查看共享列表"}
	PermShareWrite = Permission{Resource: "share", Action: "write", Desc: "创建/修改共享", DependsOn: "share:read"}
	PermShareAdmin = Permission{Resource: "share", Action: "admin", Desc: "删除共享/权限管理", DependsOn: "share:write"}

	// 网络管理
	PermNetworkRead  = Permission{Resource: "network", Action: "read", Desc: "查看网络配置"}
	PermNetworkWrite = Permission{Resource: "network", Action: "write", Desc: "修改网络配置", DependsOn: "network:read"}

	// 服务管理
	PermServiceRead  = Permission{Resource: "service", Action: "read", Desc: "查看服务状态"}
	PermServiceWrite = Permission{Resource: "service", Action: "write", Desc: "启动/停止服务", DependsOn: "service:read"}

	// 备份管理
	PermBackupRead  = Permission{Resource: "backup", Action: "read", Desc: "查看备份任务"}
	PermBackupWrite = Permission{Resource: "backup", Action: "write", Desc: "创建/执行备份", DependsOn: "backup:read"}

	// 日志查看
	PermLogRead = Permission{Resource: "log", Action: "read", Desc: "查看系统日志"}

	// 监控
	PermMonitorRead = Permission{Resource: "monitor", Action: "read", Desc: "查看系统监控"}

	// 审计
	PermAuditRead  = Permission{Resource: "audit", Action: "read", Desc: "查看审计日志"}
	PermAuditAdmin = Permission{Resource: "audit", Action: "admin", Desc: "管理审计配置"}

	// 快照管理
	PermSnapshotRead  = Permission{Resource: "snapshot", Action: "read", Desc: "查看快照"}
	PermSnapshotWrite = Permission{Resource: "snapshot", Action: "write", Desc: "创建/删除快照", DependsOn: "snapshot:read"}

	// 权限管理
	PermPermissionRead  = Permission{Resource: "permission", Action: "read", Desc: "查看权限配置"}
	PermPermissionWrite = Permission{Resource: "permission", Action: "write", Desc: "修改权限配置", DependsOn: "permission:read"}
)

// AllPermissions 所有预定义权限
var AllPermissions = []Permission{
	PermSystemRead, PermSystemWrite, PermSystemAdmin,
	PermUserRead, PermUserWrite, PermUserAdmin,
	PermStorageRead, PermStorageWrite, PermStorageAdmin,
	PermShareRead, PermShareWrite, PermShareAdmin,
	PermNetworkRead, PermNetworkWrite,
	PermServiceRead, PermServiceWrite,
	PermBackupRead, PermBackupWrite,
	PermLogRead,
	PermMonitorRead,
	PermAuditRead, PermAuditAdmin,
	PermSnapshotRead, PermSnapshotWrite,
	PermPermissionRead, PermPermissionWrite,
}

// PermissionString 权限字符串格式 (resource:action)
func PermissionString(resource, action string) string {
	return resource + ":" + action
}

// ParsePermission 解析权限字符串
func ParsePermission(perm string) (resource, action string) {
	for i := 0; i < len(perm); i++ {
		if perm[i] == ':' {
			return perm[:i], perm[i+1:]
		}
	}
	return perm, ""
}

// ========== 用户组权限继承 ==========

// GroupPermission 用户组权限
type GroupPermission struct {
	GroupID     string    `json:"group_id"`
	GroupName   string    `json:"group_name"`
	Permissions []string  `json:"permissions"`
	Roles       []Role    `json:"roles,omitempty"` // 组内角色
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UserPermission 用户权限（包含直接权限和继承权限）
type UserPermission struct {
	UserID           string         `json:"user_id"`
	Username         string         `json:"username"`
	Role             Role           `json:"role"`
	DirectPerms      []string       `json:"direct_permissions"`    // 直接授予的权限
	InheritedPerms   []string       `json:"inherited_permissions"` // 从用户组继承的权限
	EffectivePerms   []string       `json:"effective_permissions"` // 最终有效权限
	GroupMemberships []GroupMember  `json:"group_memberships"`     // 所属用户组
	LastChecked      time.Time      `json:"last_checked"`
	CustomPolicies   []PolicyEffect `json:"custom_policies,omitempty"` // 自定义策略
}

// GroupMember 用户组成员关系
type GroupMember struct {
	GroupID   string `json:"group_id"`
	GroupName string `json:"group_name"`
	IsOwner   bool   `json:"is_owner"` // 是否为组管理员
}

// ========== 权限策略 ==========

// Policy 权限策略
type Policy struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Effect      PolicyEffect `json:"effect"`     // allow 或 deny
	Principals  []string     `json:"principals"` // 应用的用户/组
	Resources   []string     `json:"resources"`  // 资源
	Actions     []string     `json:"actions"`    // 操作
	Conditions  []Condition  `json:"conditions"` // 条件
	Priority    int          `json:"priority"`   // 优先级
	Enabled     bool         `json:"enabled"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// PolicyEffect 策略效果
type PolicyEffect string

const (
	EffectAllow PolicyEffect = "allow"
	EffectDeny  PolicyEffect = "deny"
)

// Condition 策略条件
type Condition struct {
	Type     string   `json:"type"`     // 条件类型: time, ip, resource, etc.
	Key      string   `json:"key"`      // 条件键
	Operator string   `json:"operator"` // 操作符: eq, neq, in, not_in, like, etc.
	Values   []string `json:"values"`   // 条件值
}

// ========== 权限检查结果 ==========

// CheckResult 权限检查结果
type CheckResult struct {
	Allowed      bool     `json:"allowed"`
	Reason       string   `json:"reason"`
	MatchedBy    string   `json:"matched_by,omitempty"`    // 匹配的策略/角色
	DeniedBy     string   `json:"denied_by,omitempty"`     // 拒绝的策略
	MissingPerms []string `json:"missing_perms,omitempty"` // 缺少的权限
}

// ========== 权限缓存 ==========

// PermissionCache 权限缓存
type PermissionCache struct {
	UserID         string    `json:"user_id"`
	EffectivePerms []string  `json:"effective_permissions"`
	Groups         []string  `json:"groups"`
	Role           Role      `json:"role"`
	UpdatedAt      time.Time `json:"updated_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}
