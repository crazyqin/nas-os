// Package freeipa 提供 FreeIPA 目录服务集成
//
// 实现 FreeIPA LDAP 目录服务的用户/组同步、认证和策略管理。
// 参考 TrueNAS 24.04 Dragonfish 的 FreeIPA 支持设计。
//
// 兵部（软件工程）注: 本模块于 2026-06-24 开发完成。
package freeipa

import (
	"time"
)

// DirectoryStatus 目录服务状态.
type DirectoryStatus string

const (
	StatusConnected    DirectoryStatus = "connected"
	StatusDisconnected DirectoryStatus = "disconnected"
	StatusError        DirectoryStatus = "error"
	StatusSyncing      DirectoryStatus = "syncing"
)

// DirectoryConfig FreeIPA 目录服务配置.
type DirectoryConfig struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Host         string          `json:"host"`
	Port         int             `json:"port"`
	BaseDN       string          `json:"base_dn"`
	BindDN       string          `json:"bind_dn"`
	BindPassword string          `json:"bind_password"`
	UserBaseDN   string          `json:"user_base_dn"`
	GroupBaseDN  string          `json:"group_base_dn"`
	UseTLS       bool            `json:"use_tls"`
	SkipVerify   bool            `json:"skip_verify"`
	EnableSync   bool            `json:"enable_sync"`
	SyncInterval time.Duration   `json:"sync_interval"`
	Status       DirectoryStatus `json:"status"`
	LastSyncTime time.Time       `json:"last_sync_time,omitempty"`
	LastError    string          `json:"last_error,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// DefaultDirectoryConfig 默认 FreeIPA 配置.
func DefaultDirectoryConfig() DirectoryConfig {
	return DirectoryConfig{
		Port:         389,
		BaseDN:       "dc=example,dc=com",
		UserBaseDN:   "cn=users,cn=accounts",
		GroupBaseDN:  "cn=groups,cn=accounts",
		UseTLS:       true,
		SkipVerify:   false,
		EnableSync:   true,
		SyncInterval: 30 * time.Minute,
		Status:       StatusDisconnected,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// LDAPUser LDAP 用户条目.
type LDAPUser struct {
	UID           string    `json:"uid"`
	Username      string    `json:"username"`
	DisplayName   string    `json:"display_name"`
	Email         string    `json:"email"`
	FirstName     string    `json:"first_name"`
	LastName      string    `json:"last_name"`
	HomeDirectory string    `json:"home_directory"`
	Shell         string    `json:"shell"`
	GIDNumber     int       `json:"gid_number"`
	UIDNumber     int       `json:"uid_number"`
	Groups        []string  `json:"groups"`
	Enabled       bool      `json:"enabled"`
	LastLogin     time.Time `json:"last_login,omitempty"`
	SyncedAt      time.Time `json:"synced_at"`
}

// LDAPGroup LDAP 组条目.
type LDAPGroup struct {
	CN          string    `json:"cn"`
	GIDNumber   int       `json:"gid_number"`
	Description string    `json:"description"`
	Members     []string  `json:"members"`
	SyncedAt    time.Time `json:"synced_at"`
}

// SyncResult 同步结果.
type SyncResult struct {
	UsersSynced   int       `json:"users_synced"`
	GroupsSynced  int       `json:"groups_synced"`
	UsersAdded    int       `json:"users_added"`
	UsersUpdated  int       `json:"users_updated"`
	UsersRemoved  int       `json:"users_removed"`
	GroupsAdded   int       `json:"groups_added"`
	GroupsUpdated int       `json:"groups_updated"`
	GroupsRemoved int       `json:"groups_removed"`
	Errors        []string  `json:"errors,omitempty"`
	Duration      string    `json:"duration"`
	SyncedAt      time.Time `json:"synced_at"`
}

// AuthResult 认证结果.
type AuthResult struct {
	Success  bool      `json:"success"`
	User     *LDAPUser `json:"user,omitempty"`
	Error    string    `json:"error,omitempty"`
	AuthTime string    `json:"auth_time"`
}

// DirectoryStats 目录服务统计.
type DirectoryStats struct {
	TotalUsers     int             `json:"total_users"`
	TotalGroups    int             `json:"total_groups"`
	ActiveUsers    int             `json:"active_users"`
	DisabledUsers  int             `json:"disabled_users"`
	LastSyncTime   time.Time       `json:"last_sync_time,omitempty"`
	SyncErrorCount int             `json:"sync_error_count"`
	Status         DirectoryStatus `json:"status"`
	Uptime         string          `json:"uptime"`
}

// UserSearchFilter 用户搜索过滤器.
type UserSearchFilter struct {
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	Group    string `json:"group,omitempty"`
	Enabled  *bool  `json:"enabled,omitempty"`
	UIDMin   int    `json:"uid_min,omitempty"`
	UIDMax   int    `json:"uid_max,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

// GroupSearchFilter 组搜索过滤器.
type GroupSearchFilter struct {
	Name       string `json:"name,omitempty"`
	GIDMin     int    `json:"gid_min,omitempty"`
	GIDMax     int    `json:"gid_max,omitempty"`
	HasMembers *bool  `json:"has_members,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
}

// SyncSchedule 同步调度配置.
type SyncSchedule struct {
	Enabled          bool          `json:"enabled"`
	Interval         time.Duration `json:"interval"`
	LastRun          time.Time     `json:"last_run,omitempty"`
	NextRun          time.Time     `json:"next_run,omitempty"`
	SyncUsers        bool          `json:"sync_users"`
	SyncGroups       bool          `json:"sync_groups"`
	AutoCreate       bool          `json:"auto_create"`       // 自动创建本地用户映射
	ConflictStrategy string        `json:"conflict_strategy"` // "remote_wins" | "local_wins" | "manual"
}

// DefaultSyncSchedule 默认同步调度.
func DefaultSyncSchedule() SyncSchedule {
	return SyncSchedule{
		Enabled:          true,
		Interval:         30 * time.Minute,
		SyncUsers:        true,
		SyncGroups:       true,
		AutoCreate:       true,
		ConflictStrategy: "remote_wins",
	}
}
