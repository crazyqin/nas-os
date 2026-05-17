package userwizard

import "time"

// TemplateRole 预定义角色模板.
type TemplateRole string

const (
	// RoleAdmin 管理员模板：全部权限.
	RoleAdmin TemplateRole = "admin"
	// RoleStandard 标准用户模板：常用服务权限.
	RoleStandard TemplateRole = "standard"
	// RoleReadOnly 只读用户模板：只读访问.
	RoleReadOnly TemplateRole = "readonly"
	// RoleGuest 访客模板：临时有限访问.
	RoleGuest TemplateRole = "guest"
)

// UserTemplate 用户模板定义.
type UserTemplate struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Description     string       `json:"description"`
	Role            TemplateRole `json:"role"`
	StorageQuota    int64        `json:"storage_quota"`              // 字节
	AllowedServices []string     `json:"allowed_services,omitempty"`
	DeniedServices  []string     `json:"denied_services,omitempty"`
	Groups          []string     `json:"groups,omitempty"`
	IsDefault       bool         `json:"is_default"` // 是否为该角色默认模板
}

// QuickCreateRequest 快速创建用户请求.
type QuickCreateRequest struct {
	Username    string       `json:"username" binding:"required"`
	Password    string       `json:"password" binding:"required,min=6"`
	Email       string       `json:"email,omitempty"`
	TemplateID  string       `json:"template_id,omitempty"`  // 使用模板 ID
	TemplateRole TemplateRole `json:"template_role,omitempty"` // 或使用角色模板
	HomeDir     string       `json:"home_dir,omitempty"`
	Quota       int64        `json:"quota,omitempty"` // 覆盖模板配额
	Groups      []string     `json:"groups,omitempty"`
}

// QuickCreateResponse 快速创建用户响应.
type QuickCreateResponse struct {
	Username        string   `json:"username"`
	Role            string   `json:"role"`
	StorageQuota    int64    `json:"storage_quota"`
	AllowedServices []string `json:"allowed_services,omitempty"`
	Groups          []string `json:"groups,omitempty"`
	HomeDir         string   `json:"home_dir"`
	CreatedAt       time.Time `json:"created_at"`
}

// BatchOperationType 批量操作类型.
type BatchOperationType string

const (
	// BatchCreate 批量创建用户.
	BatchCreate BatchOperationType = "create"
	// BatchUpdatePermissions 批量更新权限.
	BatchUpdatePermissions BatchOperationType = "update_permissions"
	// BatchEnable 批量启用用户.
	BatchEnable BatchOperationType = "enable"
	// BatchDisable 批量禁用用户.
	BatchDisable BatchOperationType = "disable"
	// BatchDelete 批量删除用户.
	BatchDelete BatchOperationType = "delete"
)

// BatchRequest 批量操作请求.
type BatchRequest struct {
	Operation BatchOperationType `json:"operation" binding:"required"`
	// 以下字段根据 operation 类型选择使用
	Users       []string `json:"users,omitempty"`       // 目标用户名列表 (enable/disable/delete)
	CreateItems []QuickCreateRequest `json:"create_items,omitempty"` // 批量创建的用户列表
	Permission  *PermissionUpdate    `json:"permission,omitempty"`   // 权限更新配置
}

// PermissionUpdate 权限更新.
type PermissionUpdate struct {
	Role            string   `json:"role,omitempty"`
	StorageQuota    *int64   `json:"storage_quota,omitempty"`    // 使用指针以区分 0 和未设置
	AllowedServices []string `json:"allowed_services,omitempty"`
	DeniedServices  []string `json:"denied_services,omitempty"`
	AddGroups       []string `json:"add_groups,omitempty"`
	RemoveGroups    []string `json:"remove_groups,omitempty"`
}

// BatchResultItem 批量操作单个结果.
type BatchResultItem struct {
	Username string `json:"username"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

// BatchResponse 批量操作响应.
type BatchResponse struct {
	Total   int               `json:"total"`
	Success int               `json:"success"`
	Failed  int               `json:"failed"`
	Results []BatchResultItem `json:"results"`
}

// UserProfile 用户画像.
type UserProfile struct {
	Username        string         `json:"username"`
	Role            string         `json:"role"`
	Email           string         `json:"email,omitempty"`
	HomeDir         string         `json:"home_dir,omitempty"`
	StorageUsed     int64          `json:"storage_used"`     // 已用存储 (字节)
	StorageQuota    int64          `json:"storage_quota"`    // 存储配额 (字节)
	QuotaUsagePct   float64        `json:"quota_usage_pct"`  // 配额使用率 %
	Groups          []string       `json:"groups"`
	AllowedServices []string       `json:"allowed_services,omitempty"`
	DeniedServices  []string       `json:"denied_services,omitempty"`
	Disabled        bool           `json:"disabled"`
	CreatedAt       time.Time      `json:"created_at"`
	LastLoginAt     *time.Time     `json:"last_login_at,omitempty"`
	LastLoginIP     string         `json:"last_login_ip,omitempty"`
	Activity        UserActivity   `json:"activity"`
}

// UserActivity 用户活跃度信息.
type UserActivity struct {
	TotalLogins    int        `json:"total_logins"`
	LastLoginAt    *time.Time `json:"last_login_at,omitempty"`
	LastActiveIP   string     `json:"last_active_ip,omitempty"`
	RecentLogins   int        `json:"recent_logins"`    // 最近 30 天登录次数
}

// TemplateListResponse 模板列表响应.
type TemplateListResponse struct {
	Templates []UserTemplate `json:"templates"`
}
