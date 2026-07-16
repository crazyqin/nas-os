// Package familydash 提供家庭仪表板功能，为不同家庭成员提供个性化的 NAS 使用视图。
package familydash

import "time"

// MemberRole 成员角色.
type MemberRole string

const (
	RoleAdmin  MemberRole = "admin"
	RoleParent MemberRole = "parent"
	RoleChild  MemberRole = "child"
	RoleGuest  MemberRole = "guest"
)

// MemberStatus 成员状态.
type MemberStatus string

const (
	StatusOnline  MemberStatus = "online"
	StatusOffline MemberStatus = "offline"
	StatusAway    MemberStatus = "away"
	StatusBusy    MemberStatus = "busy"
)

// PermissionLevel 权限级别.
type PermissionLevel string

const (
	PermissionFull     PermissionLevel = "full"
	PermissionLimited  PermissionLevel = "limited"
	PermissionReadOnly PermissionLevel = "readonly"
	PermissionCustom   PermissionLevel = "custom"
)

// ActivityType 活动类型.
type ActivityType string

const (
	ActivityFileAccess   ActivityType = "file_access"
	ActivityFileUpload   ActivityType = "file_upload"
	ActivityFileDownload ActivityType = "file_download"
	ActivityStream       ActivityType = "stream"
	ActivityBackup       ActivityType = "backup"
	ActivityApp          ActivityType = "app"
	ActivityLogin        ActivityType = "login"
	ActivityLogout       ActivityType = "logout"
)

// WidgetType 仪表板组件类型.
type WidgetType string

const (
	WidgetStorage     WidgetType = "storage"
	WidgetActivity    WidgetType = "activity"
	WidgetQuickAccess WidgetType = "quick_access"
	WidgetWeather     WidgetType = "weather"
	WidgetCalendar    WidgetType = "calendar"
	WidgetTasks       WidgetType = "tasks"
	WidgetPhotos      WidgetType = "photos"
	WidgetMusic       WidgetType = "music"
	WidgetDownloads   WidgetType = "downloads"
	WidgetSystem      WidgetType = "system"
)

// FamilyMember 家庭成员.
type FamilyMember struct {
	ID         string       `json:"id"`
	Name       string       `json:"name" binding:"required"`
	Email      string       `json:"email,omitempty"`
	Avatar     string       `json:"avatar,omitempty"`
	Role       MemberRole   `json:"role"`
	Status     MemberStatus `json:"status"`
	IsChild    bool         `json:"is_child"`
	BirthYear  int          `json:"birth_year,omitempty"`
	LastActive *time.Time   `json:"last_active,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

// MemberProfile 成员个人资料.
type MemberProfile struct {
	MemberID        string            `json:"member_id"`
	DisplayName     string            `json:"display_name"`
	Bio             string            `json:"bio,omitempty"`
	Theme           string            `json:"theme,omitempty"`
	Language        string            `json:"language,omitempty"`
	Timezone        string            `json:"timezone,omitempty"`
	Notifications   bool              `json:"notifications"`
	DashboardLayout []DashboardWidget `json:"dashboard_layout"`
	Favorites       []FavoriteItem    `json:"favorites"`
	RecentFiles     []RecentFile      `json:"recent_files"`
	StorageUsed     int64             `json:"storage_used"`
	StorageQuota    int64             `json:"storage_quota"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// DashboardWidget 仪表板组件.
type DashboardWidget struct {
	ID       string                 `json:"id"`
	Type     WidgetType             `json:"type" binding:"required"`
	Title    string                 `json:"title"`
	Position int                    `json:"position"`
	Size     string                 `json:"size"` // small, medium, large
	Config   map[string]interface{} `json:"config,omitempty"`
	Visible  bool                   `json:"visible"`
}

// FavoriteItem 收藏项目.
type FavoriteItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // file, folder, app
	Path string `json:"path"`
	Icon string `json:"icon,omitempty"`
}

// RecentFile 最近文件.
type RecentFile struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	Type       string    `json:"type"`
	AccessedAt time.Time `json:"accessed_at"`
}

// Permissions 成员权限.
type Permissions struct {
	MemberID       string          `json:"member_id"`
	Level          PermissionLevel `json:"level"`
	CanAccessFiles bool            `json:"can_access_files"`
	CanUpload      bool            `json:"can_upload"`
	CanDownload    bool            `json:"can_download"`
	CanShare       bool            `json:"can_share"`
	CanStream      bool            `json:"can_stream"`
	CanDelete      bool            `json:"can_delete"`
	CanManage      bool            `json:"can_manage"`
	AllowedApps    []string        `json:"allowed_apps,omitempty"`
	BlockedApps    []string        `json:"blocked_apps,omitempty"`
	StorageQuota   int64           `json:"storage_quota"`
	TimeLimit      int             `json:"time_limit_minutes,omitempty"`
	AllowedHours   *TimeRange      `json:"allowed_hours,omitempty"`
}

// TimeRange 时间范围.
type TimeRange struct {
	Start string `json:"start"` // HH:MM
	End   string `json:"end"`   // HH:MM
}

// ActivityEntry 活动记录.
type ActivityEntry struct {
	ID        string       `json:"id"`
	MemberID  string       `json:"member_id"`
	Type      ActivityType `json:"type"`
	Action    string       `json:"action"`
	Resource  string       `json:"resource,omitempty"`
	Details   string       `json:"details,omitempty"`
	IPAddress string       `json:"ip_address,omitempty"`
	Device    string       `json:"device,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
}

// ActivitySummary 活动摘要.
type ActivitySummary struct {
	MemberID       string          `json:"member_id"`
	Period         string          `json:"period"`
	TotalActions   int             `json:"total_actions"`
	Uploads        int             `json:"uploads"`
	Downloads      int             `json:"downloads"`
	Streams        int             `json:"streams"`
	StorageUsed    int64           `json:"storage_used"`
	MostActiveHour int             `json:"most_active_hour"`
	TopActivities  []ActivityCount `json:"top_activities"`
}

// ActivityCount 活动计数.
type ActivityCount struct {
	Type  ActivityType `json:"type"`
	Count int          `json:"count"`
}

// FamilyStats 家庭统计.
type FamilyStats struct {
	FamilyID      string        `json:"family_id"`
	TotalMembers  int           `json:"total_members"`
	OnlineMembers int           `json:"online_members"`
	TotalStorage  int64         `json:"total_storage"`
	UsedStorage   int64         `json:"used_storage"`
	TotalFiles    int           `json:"total_files"`
	TotalStreams  int           `json:"total_streams"`
	MemberStats   []MemberStats `json:"member_stats"`
	GeneratedAt   time.Time     `json:"generated_at"`
}

// MemberStats 成员统计.
type MemberStats struct {
	MemberID      string     `json:"member_id"`
	Name          string     `json:"name"`
	StorageUsed   int64      `json:"storage_used"`
	FileCount     int        `json:"file_count"`
	ActivityCount int        `json:"activity_count"`
	LastActive    *time.Time `json:"last_active"`
}

// CreateMemberRequest 创建成员请求.
type CreateMemberRequest struct {
	Name      string     `json:"name" binding:"required"`
	Email     string     `json:"email,omitempty"`
	Avatar    string     `json:"avatar,omitempty"`
	Role      MemberRole `json:"role"`
	IsChild   bool       `json:"is_child"`
	BirthYear int        `json:"birth_year,omitempty"`
}

// UpdateMemberRequest 更新成员请求.
type UpdateMemberRequest struct {
	Name      string     `json:"name,omitempty"`
	Email     string     `json:"email,omitempty"`
	Avatar    string     `json:"avatar,omitempty"`
	Role      MemberRole `json:"role,omitempty"`
	BirthYear int        `json:"birth_year,omitempty"`
}

// UpdateProfileRequest 更新个人资料请求.
type UpdateProfileRequest struct {
	DisplayName     string            `json:"display_name,omitempty"`
	Bio             string            `json:"bio,omitempty"`
	Theme           string            `json:"theme,omitempty"`
	Language        string            `json:"language,omitempty"`
	Timezone        string            `json:"timezone,omitempty"`
	Notifications   *bool             `json:"notifications,omitempty"`
	DashboardLayout []DashboardWidget `json:"dashboard_layout,omitempty"`
}

// UpdatePermissionsRequest 更新权限请求.
type UpdatePermissionsRequest struct {
	Level          PermissionLevel `json:"level,omitempty"`
	CanAccessFiles *bool           `json:"can_access_files,omitempty"`
	CanUpload      *bool           `json:"can_upload,omitempty"`
	CanDownload    *bool           `json:"can_download,omitempty"`
	CanShare       *bool           `json:"can_share,omitempty"`
	CanStream      *bool           `json:"can_stream,omitempty"`
	CanDelete      *bool           `json:"can_delete,omitempty"`
	CanManage      *bool           `json:"can_manage,omitempty"`
	AllowedApps    []string        `json:"allowed_apps,omitempty"`
	BlockedApps    []string        `json:"blocked_apps,omitempty"`
	StorageQuota   int64           `json:"storage_quota,omitempty"`
	TimeLimit      int             `json:"time_limit_minutes,omitempty"`
	AllowedHours   *TimeRange      `json:"allowed_hours,omitempty"`
}

// AddFavoriteRequest 添加收藏请求.
type AddFavoriteRequest struct {
	Name string `json:"name" binding:"required"`
	Type string `json:"type" binding:"required"`
	Path string `json:"path" binding:"required"`
	Icon string `json:"icon,omitempty"`
}

// ActivityQuery 活动查询.
type ActivityQuery struct {
	MemberID string       `json:"member_id"`
	Type     ActivityType `json:"type,omitempty"`
	FromDate string       `json:"from_date,omitempty"`
	ToDate   string       `json:"to_date,omitempty"`
	Limit    int          `json:"limit,omitempty"`
}

// DefaultDashboardLayout 默认仪表板布局.
func DefaultDashboardLayout() []DashboardWidget {
	return []DashboardWidget{
		{ID: "w1", Type: WidgetStorage, Title: "存储空间", Position: 0, Size: "medium", Visible: true},
		{ID: "w2", Type: WidgetActivity, Title: "最近活动", Position: 1, Size: "large", Visible: true},
		{ID: "w3", Type: WidgetQuickAccess, Title: "快速访问", Position: 2, Size: "small", Visible: true},
		{ID: "w4", Type: WidgetSystem, Title: "系统状态", Position: 3, Size: "small", Visible: true},
	}
}

// DefaultPermissions 默认权限.
func DefaultPermissions(role MemberRole) *Permissions {
	base := &Permissions{
		Level:          PermissionLimited,
		CanAccessFiles: true,
		CanUpload:      true,
		CanDownload:    true,
		CanShare:       false,
		CanStream:      true,
		CanDelete:      false,
		CanManage:      false,
		StorageQuota:   10 * 1024 * 1024 * 1024, // 10GB
	}

	switch role {
	case RoleAdmin:
		base.Level = PermissionFull
		base.CanShare = true
		base.CanDelete = true
		base.CanManage = true
		base.StorageQuota = 0 // 无限制
	case RoleParent:
		base.Level = PermissionFull
		base.CanShare = true
		base.CanDelete = true
		base.StorageQuota = 100 * 1024 * 1024 * 1024 // 100GB
	case RoleChild:
		base.Level = PermissionLimited
		base.StorageQuota = 5 * 1024 * 1024 * 1024 // 5GB
		base.TimeLimit = 120                       // 2小时
		base.AllowedHours = &TimeRange{Start: "08:00", End: "21:00"}
	case RoleGuest:
		base.Level = PermissionReadOnly
		base.CanUpload = false
		base.StorageQuota = 1 * 1024 * 1024 * 1024 // 1GB
	}

	return base
}
