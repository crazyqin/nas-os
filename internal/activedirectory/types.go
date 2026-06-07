package activedirectory

import "time"

// ADConfig Active Directory 配置.
type ADConfig struct {
	Servers           []string      `json:"servers"`
	BaseDN            string        `json:"base_dn"`
	BindDN            string        `json:"bind_dn"`
	BindPassword      string        `json:"-"` // 不序列化
	Port              int           `json:"port"`
	UseSSL            bool          `json:"use_ssl"`
	UseTLS            bool          `json:"use_tls"`
	SyncInterval      time.Duration `json:"sync_interval"`
	ConnectionTimeout time.Duration `json:"connection_timeout"`
	MaxConnections    int           `json:"max_connections"`
	SearchLimit       int           `json:"search_limit"`
	UserSearchBase    string        `json:"user_search_base"`
	GroupSearchBase   string        `json:"group_search_base"`
	UserFilter        string        `json:"user_filter"`
	GroupFilter       string        `json:"group_filter"`
}

// Domain AD 域.
type Domain struct {
	Name         string       `json:"name"`
	Server       string       `json:"server"`
	Port         int          `json:"port"`
	BaseDN       string       `json:"base_dn"`
	Status       DomainStatus `json:"status"`
	LastSync     time.Time    `json:"last_sync"`
	UserCount    int          `json:"user_count"`
	GroupCount   int          `json:"group_count"`
	ErrorMessage string       `json:"error_message,omitempty"`
}

// DomainStatus 域状态.
type DomainStatus string

const (
	DomainStatusConnected    DomainStatus = "connected"
	DomainStatusDisconnected DomainStatus = "disconnected"
	DomainStatusError        DomainStatus = "error"
	DomainStatusSyncing      DomainStatus = "syncing"
)

// ADUser AD 用户.
type ADUser struct {
	ID              string            `json:"id"`
	Username        string            `json:"username"`
	Domain          string            `json:"domain"`
	Email           string            `json:"email"`
	DisplayName     string            `json:"display_name"`
	FirstName       string            `json:"first_name"`
	LastName        string            `json:"last_name"`
	Department      string            `json:"department"`
	Title           string            `json:"title"`
	Enabled         bool              `json:"enabled"`
	Locked          bool              `json:"locked"`
	PasswordExpired bool              `json:"password_expired"`
	LastLogon       time.Time         `json:"last_logon"`
	MemberOf        []string          `json:"member_of"`
	Attributes      map[string]string `json:"attributes"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// ADGroup AD 组.
type ADGroup struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Domain      string    `json:"domain"`
	Description string    `json:"description"`
	Members     []string  `json:"members"`
	Type        string    `json:"type"`
	CreatedAt   time.Time `json:"created_at"`
}

// SyncJob 同步任务.
type SyncJob struct {
	ID            string        `json:"id"`
	Domain        string        `json:"domain"`
	Type          SyncType      `json:"type"`
	Status        SyncStatus    `json:"status"`
	StartedAt     time.Time     `json:"started_at"`
	CompletedAt   time.Time     `json:"completed_at"`
	RecordsSynced int           `json:"records_synced"`
	ErrorMessage  string        `json:"error_message,omitempty"`
	Duration      time.Duration `json:"duration"`
}

// SyncType 同步类型.
type SyncType string

const (
	SyncTypeUsers  SyncType = "users"
	SyncTypeGroups SyncType = "groups"
	SyncTypeAll    SyncType = "all"
)

// SyncStatus 同步状态.
type SyncStatus string

const (
	SyncStatusPending   SyncStatus = "pending"
	SyncStatusRunning   SyncStatus = "running"
	SyncStatusCompleted SyncStatus = "completed"
	SyncStatusFailed    SyncStatus = "failed"
)

// SyncResult 同步结果.
type SyncResult struct {
	JobID         string        `json:"job_id"`
	Domain        string        `json:"domain"`
	RecordsSynced int           `json:"records_synced"`
	Duration      time.Duration `json:"duration"`
	ErrorMessage  string        `json:"error_message,omitempty"`
}

// ADStats 统计信息.
type ADStats struct {
	TotalDomains  int `json:"total_domains"`
	TotalUsers    int `json:"total_users"`
	EnabledUsers  int `json:"enabled_users"`
	TotalGroups   int `json:"total_groups"`
	TotalSyncJobs int `json:"total_sync_jobs"`
}
