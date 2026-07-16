// Package ldap 提供 LDAP/Active Directory 集成功能
package ldap

import (
	"errors"
	"time"
)

// 基本错误定义.
var (
	ErrConfigNotFound    = errors.New("LDAP 配置未找到")
	ErrConnectionFailed  = errors.New("LDAP 连接失败")
	ErrBindFailed        = errors.New("LDAP 绑定失败")
	ErrUserNotFound      = errors.New("LDAP 用户未找到")
	ErrAuthFailed        = errors.New("LDAP 认证失败")
	ErrSearchFailed      = errors.New("LDAP 搜索失败")
	ErrGroupNotFound     = errors.New("LDAP 组未找到")
	ErrSyncFailed        = errors.New("LDAP 同步失败")
	ErrInvalidConfig     = errors.New("无效的 LDAP 配置")
	ErrTLSCertInvalid    = errors.New("TLS 证书无效")
	ErrTimeout           = errors.New("LDAP 操作超时")
	ErrServerUnavailable = errors.New("LDAP 服务器不可用")
	ErrPermissionDenied  = errors.New("权限不足")
	ErrAlreadyExists     = errors.New("记录已存在")
	ErrNotImplemented    = errors.New("功能未实现")
	ErrOperationFailed   = errors.New("操作失败")
)

// ServerType LDAP 服务器类型.
type ServerType string

// LDAP 服务器类型常量.
const (
	ServerTypeOpenLDAP ServerType = "openldap"
	ServerTypeAD       ServerType = "ad" // Active Directory
	ServerTypeFreeIPA  ServerType = "freeipa"
	ServerTypeGeneric  ServerType = "generic"
)

// SyncMode 同步模式.
type SyncMode string

// 同步模式常量.
const (
	SyncModeFull        SyncMode = "full"        // 全量同步
	SyncModeIncremental SyncMode = "incremental" // 增量同步
	SyncModeOneTime     SyncMode = "onetime"     // 一次性同步
)

// SyncDirection 同步方向.
type SyncDirection string

// 同步方向常量.
const (
	SyncDirectionImport SyncDirection = "import" // 从 LDAP 导入
	SyncDirectionExport SyncDirection = "export" // 导出到 LDAP
	SyncDirectionBoth   SyncDirection = "both"   // 双向同步
)

// Config LDAP/AD 服务器配置.
type Config struct {
	// 基本信息
	Name        string     `json:"name" yaml:"name"`
	Description string     `json:"description,omitempty" yaml:"description,omitempty"`
	ServerType  ServerType `json:"server_type" yaml:"server_type"`
	Enabled     bool       `json:"enabled" yaml:"enabled"`

	// 连接配置
	URL          string `json:"url" yaml:"url"`                                         // ldap://host:port 或 ldaps://host:port
	BindDN       string `json:"bind_dn,omitempty" yaml:"bind_dn,omitempty"`             // 绑定 DN
	BindPassword string `json:"bind_password,omitempty" yaml:"bind_password,omitempty"` // 绑定密码
	BaseDN       string `json:"base_dn" yaml:"base_dn"`                                 // 搜索基础 DN

	// TLS 配置
	UseTLS         bool   `json:"use_tls" yaml:"use_tls"`
	SkipTLSVerify  bool   `json:"skip_tls_verify" yaml:"skip_tls_verify"` // 仅测试用
	CACertPath     string `json:"ca_cert_path,omitempty" yaml:"ca_cert_path,omitempty"`
	ClientCertPath string `json:"client_cert_path,omitempty" yaml:"client_cert_path,omitempty"`
	ClientKeyPath  string `json:"client_key_path,omitempty" yaml:"client_key_path,omitempty"`

	// 搜索配置
	UserSearchDN  string `json:"user_search_dn,omitempty" yaml:"user_search_dn,omitempty"`
	GroupSearchDN string `json:"group_search_dn,omitempty" yaml:"group_search_dn,omitempty"`
	UserFilter    string `json:"user_filter" yaml:"user_filter"` // 如 (uid=%s)
	GroupFilter   string `json:"group_filter,omitempty" yaml:"group_filter,omitempty"`

	// 属性映射
	Attributes AttributeMapping `json:"attributes" yaml:"attributes"`

	// AD 特定配置
	DomainName    string `json:"domain_name,omitempty" yaml:"domain_name,omitempty"`       // 如 example.com
	NetBIOSName   string `json:"netbios_name,omitempty" yaml:"netbios_name,omitempty"`     // 如 EXAMPLE
	GlobalCatalog string `json:"global_catalog,omitempty" yaml:"global_catalog,omitempty"` // 全局编录服务器

	// 同步配置
	SyncConfig SyncConfig `json:"sync_config" yaml:"sync_config"`

	// 连接池配置
	PoolSize         int           `json:"pool_size" yaml:"pool_size"`
	IdleTimeout      time.Duration `json:"idle_timeout" yaml:"idle_timeout"`
	ConnectTimeout   time.Duration `json:"connect_timeout" yaml:"connect_timeout"`
	OperationTimeout time.Duration `json:"operation_timeout" yaml:"operation_timeout"`

	// 重试配置
	MaxRetries int           `json:"max_retries" yaml:"max_retries"`
	RetryDelay time.Duration `json:"retry_delay" yaml:"retry_delay"`
}

// AttributeMapping 属性映射配置.
type AttributeMapping struct {
	// 用户属性
	Username    string `json:"username" yaml:"username"`
	Email       string `json:"email" yaml:"email"`
	FirstName   string `json:"first_name" yaml:"first_name"`
	LastName    string `json:"last_name" yaml:"last_name"`
	FullName    string `json:"full_name" yaml:"full_name"`
	DisplayName string `json:"display_name" yaml:"display_name"`
	Phone       string `json:"phone" yaml:"phone"`
	Mobile      string `json:"mobile" yaml:"mobile"`
	Department  string `json:"department" yaml:"department"`
	Title       string `json:"title" yaml:"title"`
	EmployeeID  string `json:"employee_id" yaml:"employee_id"`

	// 组属性
	GroupName        string `json:"group_name" yaml:"group_name"`
	GroupDescription string `json:"group_description" yaml:"group_description"`
	GroupMember      string `json:"group_member" yaml:"group_member"`

	// 成员关系
	MemberAttribute   string `json:"member_attribute" yaml:"member_attribute"`       // member 或 memberUid
	MemberOfAttribute string `json:"member_of_attribute" yaml:"member_of_attribute"` // memberOf

	// 对象类
	UserObjectClass  string `json:"user_object_class" yaml:"user_object_class"`
	GroupObjectClass string `json:"group_object_class" yaml:"group_object_class"`
}

// SyncConfig 同步配置.
type SyncConfig struct {
	Enabled   bool          `json:"enabled" yaml:"enabled"`
	Mode      SyncMode      `json:"mode" yaml:"mode"`
	Direction SyncDirection `json:"direction" yaml:"direction"`
	Interval  time.Duration `json:"interval" yaml:"interval"`

	// 用户同步选项
	SyncUsers         bool   `json:"sync_users" yaml:"sync_users"`
	UserFilter        string `json:"user_filter,omitempty" yaml:"user_filter,omitempty"`
	UserExcludeFilter string `json:"user_exclude_filter,omitempty" yaml:"user_exclude_filter,omitempty"`
	CreateUsers       bool   `json:"create_users" yaml:"create_users"`
	UpdateUsers       bool   `json:"update_users" yaml:"update_users"`
	DeactivateUsers   bool   `json:"deactivate_users" yaml:"deactivate_users"`
	DefaultUserRole   string `json:"default_user_role" yaml:"default_user_role"`

	// 组同步选项
	SyncGroups   bool   `json:"sync_groups" yaml:"sync_groups"`
	GroupFilter  string `json:"group_filter,omitempty" yaml:"group_filter,omitempty"`
	CreateGroups bool   `json:"create_groups" yaml:"create_groups"`
	UpdateGroups bool   `json:"update_groups" yaml:"update_groups"`
	DeleteGroups bool   `json:"delete_groups" yaml:"delete_groups"`

	// 映射规则
	GroupRoleMapping map[string]string `json:"group_role_mapping,omitempty" yaml:"group_role_mapping,omitempty"`

	// 冲突处理
	ConflictResolution string `json:"conflict_resolution" yaml:"conflict_resolution"` // skip, overwrite, merge
}

// User LDAP 用户信息.
type User struct {
	// 基本信息
	DN          string `json:"dn"`
	Username    string `json:"username"`
	Email       string `json:"email,omitempty"`
	FirstName   string `json:"first_name,omitempty"`
	LastName    string `json:"last_name,omitempty"`
	FullName    string `json:"full_name,omitempty"`
	DisplayName string `json:"display_name,omitempty"`

	// 联系信息
	Phone  string `json:"phone,omitempty"`
	Mobile string `json:"mobile,omitempty"`

	// 组织信息
	Department string `json:"department,omitempty"`
	Title      string `json:"title,omitempty"`
	Company    string `json:"company,omitempty"`

	// 标识
	EmployeeID string `json:"employee_id,omitempty"`
	UID        string `json:"uid,omitempty"`
	GID        string `json:"gid,omitempty"`

	// 状态
	Disabled        bool `json:"disabled,omitempty"`
	Locked          bool `json:"locked,omitempty"`
	PasswordExpired bool `json:"password_expired,omitempty"`

	// 时间
	LastLogin      *time.Time `json:"last_login,omitempty"`
	PasswordSet    *time.Time `json:"password_set,omitempty"`
	AccountCreated *time.Time `json:"account_created,omitempty"`

	// 组信息
	Groups []string `json:"groups,omitempty"`

	// 原始属性
	RawAttributes map[string][]string `json:"raw_attributes,omitempty"`
}

// Group LDAP 组信息.
type Group struct {
	DN          string   `json:"dn"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	GID         string   `json:"gid,omitempty"`
	Members     []string `json:"members,omitempty"`      // 成员 DN 列表
	MemberUsers []string `json:"member_users,omitempty"` // 成员用户名列表
	Type        string   `json:"type,omitempty"`         // security, distribution
	Scope       string   `json:"scope,omitempty"`        // global, domain_local, universal (AD)

	// 管理
	ManagedBy string `json:"managed_by,omitempty"`

	// 原始属性
	RawAttributes map[string][]string `json:"raw_attributes,omitempty"`
}

// SearchResult 搜索结果.
type SearchResult struct {
	Users  []*User  `json:"users,omitempty"`
	Groups []*Group `json:"groups,omitempty"`
	Total  int      `json:"total"`
}

// SyncResult 同步结果.
type SyncResult struct {
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`

	// 用户同步统计
	UsersCreated     int         `json:"users_created"`
	UsersUpdated     int         `json:"users_updated"`
	UsersDeactivated int         `json:"users_deactivated"`
	UsersSkipped     int         `json:"users_skipped"`
	UsersFailed      int         `json:"users_failed"`
	UserErrors       []SyncError `json:"user_errors,omitempty"`

	// 组同步统计
	GroupsCreated int         `json:"groups_created"`
	GroupsUpdated int         `json:"groups_updated"`
	GroupsDeleted int         `json:"groups_deleted"`
	GroupsSkipped int         `json:"groups_skipped"`
	GroupsFailed  int         `json:"groups_failed"`
	GroupErrors   []SyncError `json:"group_errors,omitempty"`

	// 总体状态
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// SyncError 同步错误.
type SyncError struct {
	DN    string `json:"dn"`
	Name  string `json:"name"`
	Type  string `json:"type"` // user, group
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// ConnectionStatus 连接状态.
type ConnectionStatus struct {
	ConfigName  string        `json:"config_name"`
	Connected   bool          `json:"connected"`
	LastChecked time.Time     `json:"last_checked"`
	Latency     time.Duration `json:"latency"`
	Error       string        `json:"error,omitempty"`
	ServerInfo  *ServerInfo   `json:"server_info,omitempty"`
}

// ServerInfo 服务器信息.
type ServerInfo struct {
	Vendor          string `json:"vendor,omitempty"`
	Version         string `json:"version,omitempty"`
	ProductName     string `json:"product_name,omitempty"`
	FunctionalLevel string `json:"functional_level,omitempty"` // AD 功能级别
	ForestName      string `json:"forest_name,omitempty"`      // AD 林名称
	DomainName      string `json:"domain_name,omitempty"`
	Realm           string `json:"realm,omitempty"`
}

// AuthResult 认证结果.
type AuthResult struct {
	Success    bool   `json:"success"`
	User       *User  `json:"user,omitempty"`
	Token      string `json:"token,omitempty"`
	Message    string `json:"message,omitempty"`
	AuthMethod string `json:"auth_method"` // simple, sasl, gssapi
}

// OperationResult 操作结果.
type OperationResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   error  `json:"error,omitempty"`
}

// DefaultConfig 获取默认配置.
func DefaultConfig() Config {
	return Config{
		ServerType:       ServerTypeGeneric,
		Enabled:          true,
		UseTLS:           true,
		PoolSize:         10,
		IdleTimeout:      30 * time.Minute,
		ConnectTimeout:   10 * time.Second,
		OperationTimeout: 30 * time.Second,
		MaxRetries:       3,
		RetryDelay:       1 * time.Second,
		Attributes:       DefaultAttributeMapping(),
		SyncConfig:       DefaultSyncConfig(),
	}
}

// DefaultAttributeMapping 获取默认属性映射.
func DefaultAttributeMapping() AttributeMapping {
	return AttributeMapping{
		Username:          "uid",
		Email:             "mail",
		FirstName:         "givenName",
		LastName:          "sn",
		FullName:          "cn",
		DisplayName:       "displayName",
		Phone:             "telephoneNumber",
		Mobile:            "mobile",
		Department:        "departmentNumber",
		Title:             "title",
		EmployeeID:        "employeeNumber",
		GroupName:         "cn",
		GroupDescription:  "description",
		GroupMember:       "member",
		MemberAttribute:   "member",
		MemberOfAttribute: "memberOf",
		UserObjectClass:   "inetOrgPerson",
		GroupObjectClass:  "groupOfNames",
	}
}

// ADAttributeMapping 获取 Active Directory 属性映射.
func ADAttributeMapping() AttributeMapping {
	return AttributeMapping{
		Username:          "sAMAccountName",
		Email:             "mail",
		FirstName:         "givenName",
		LastName:          "sn",
		FullName:          "cn",
		DisplayName:       "displayName",
		Phone:             "telephoneNumber",
		Mobile:            "mobile",
		Department:        "department",
		Title:             "title",
		EmployeeID:        "employeeID",
		GroupName:         "cn",
		GroupDescription:  "description",
		GroupMember:       "member",
		MemberAttribute:   "member",
		MemberOfAttribute: "memberOf",
		UserObjectClass:   "user",
		GroupObjectClass:  "group",
	}
}

// DefaultSyncConfig 获取默认同步配置.
func DefaultSyncConfig() SyncConfig {
	return SyncConfig{
		Enabled:            false,
		Mode:               SyncModeFull,
		Direction:          SyncDirectionImport,
		Interval:           1 * time.Hour,
		SyncUsers:          true,
		CreateUsers:        true,
		UpdateUsers:        true,
		DeactivateUsers:    true,
		DefaultUserRole:    "user",
		SyncGroups:         true,
		CreateGroups:       true,
		UpdateGroups:       true,
		DeleteGroups:       false,
		ConflictResolution: "merge",
	}
}
