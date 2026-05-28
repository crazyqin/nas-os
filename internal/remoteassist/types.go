// Package remoteassist 提供远程协助系统
// 参考群晖 Active Assist 和飞牛远程访问，实现安全的远程协助
package remoteassist

import (
	"time"
)

// ========== 远程会话类型 ==========

// SessionStatus 会话状态.
type SessionStatus string

const (
	StatusPending    SessionStatus = "pending"    // 待连接
	StatusConnecting SessionStatus = "connecting" // 连接中
	StatusActive     SessionStatus = "active"     // 活跃
	StatusPaused     SessionStatus = "paused"     // 已暂停
	StatusCompleted  SessionStatus = "completed"  // 已完成
	StatusFailed     SessionStatus = "failed"     // 失败
	StatusCancelled  SessionStatus = "cancelled"  // 已取消
)

// AssistType 协助类型.
type AssistType string

const (
	AssistTypeScreen   AssistType = "screen"   // 屏幕共享
	AssistTypeTerminal AssistType = "terminal"  // 终端接入
	AssistTypeFile     AssistType = "file"      // 文件传输
	AssistTypeChat     AssistType = "chat"      // 文字聊天
	AssistTypeFull     AssistType = "full"      // 完整协助
)

// PermissionLevel 权限级别.
type PermissionLevel string

const (
	PermissionView     PermissionLevel = "view"     // 仅查看
	PermissionControl  PermissionLevel = "control"  // 控制
	PermissionAdmin    PermissionLevel = "admin"     // 管理员
)

// Session 远程协助会话.
type Session struct {
	ID              string          `json:"id"`               // 会话ID
	Name            string          `json:"name"`             // 会话名称
	Type            AssistType      `json:"type"`             // 协助类型
	Status          SessionStatus   `json:"status"`           // 会话状态
	HostID          string          `json:"host_id"`          // 主机ID
	HostAddress     string          `json:"host_address"`     // 主机地址
	GuestID         string          `json:"guest_id"`         // 访客ID
	GuestName       string          `json:"guest_name"`       // 访客名称
	GuestAddress    string          `json:"guest_address"`    // 访客地址
	Permission      PermissionLevel `json:"permission"`       // 权限级别
	Token           string          `json:"token"`            // 连接令牌
	ExpiresAt       time.Time       `json:"expires_at"`       // 过期时间
	StartedAt       *time.Time      `json:"started_at"`       // 开始时间
	EndedAt         *time.Time      `json:"ended_at"`         // 结束时间
	Duration        int64           `json:"duration"`         // 持续时间(秒)
	BytesSent       int64           `json:"bytes_sent"`       // 发送字节数
	BytesReceived   int64           `json:"bytes_received"`   // 接收字节数
	IsRecording     bool            `json:"is_recording"`     // 是否录制中
	RecordingPath   string          `json:"recording_path"`   // 录制路径
	Tags            []string        `json:"tags"`             // 标签
	Metadata        map[string]string `json:"metadata"`       // 元数据
	CreatedAt       time.Time       `json:"created_at"`       // 创建时间
	UpdatedAt       time.Time       `json:"updated_at"`       // 更新时间
}

// AssistRequest 协助请求.
type AssistRequest struct {
	ID          string          `json:"id"`           // 请求ID
	SessionID   string          `json:"session_id"`   // 会话ID
	Type        AssistType      `json:"type"`         // 协助类型
	HostID      string          `json:"host_id"`      // 主机ID
	GuestID     string          `json:"guest_id"`     // 访客ID
	Permission  PermissionLevel `json:"permission"`   // 请求权限
	Message     string          `json:"message"`      // 请求消息
	ExpiresIn   int             `json:"expires_in"`   // 过期时间(秒)
	AutoApprove bool            `json:"auto_approve"` // 自动批准
	CreatedAt   time.Time       `json:"created_at"`   // 创建时间
}

// ========== 屏幕共享 ==========

// ScreenShare 屏幕共享.
type ScreenShare struct {
	ID          string         `json:"id"`           // 共享ID
	SessionID   string         `json:"session_id"`   // 会话ID
	Width       int            `json:"width"`        // 宽度
	Height      int            `json:"height"`       // 高度
	FPS         int            `json:"fps"`          // 帧率
	Quality     int            `json:"quality"`      // 质量(1-100)
	Bitrate     int            `json:"bitrate"`      // 码率
	Codec       string         `json:"codec"`        // 编码格式
	Status      string         `json:"status"`       // 状态
	StartedAt   time.Time      `json:"started_at"`   // 开始时间
	Cursor      *CursorPosition `json:"cursor"`       // 光标位置
}

// CursorPosition 光标位置.
type CursorPosition struct {
	X     int    `json:"x"`     // X坐标
	Y     int    `json:"y"`     // Y坐标
	Type  string `json:"type"`  // 类型(normal/text/pointer)
	Click bool   `json:"click"` // 是否点击
}

// ScreenFrame 屏幕帧.
type ScreenFrame struct {
	ID        string `json:"id"`         // 帧ID
	SessionID string `json:"session_id"` // 会话ID
	Sequence  int64  `json:"sequence"`   // 序列号
	Data      []byte `json:"data"`       // 帧数据
	Width     int    `json:"width"`      // 宽度
	Height    int    `json:"height"`     // 高度
	Format    string `json:"format"`     // 格式
	Timestamp int64  `json:"timestamp"`  // 时间戳
	Size      int    `json:"size"`       // 数据大小
}

// ========== 终端接入 ==========

// TerminalSession 终端会话.
type TerminalSession struct {
	ID          string    `json:"id"`           // 会话ID
	SessionID   string    `json:"session_id"`   // 父会话ID
	Shell       string    `json:"shell"`        // Shell类型
	Rows        int       `json:"rows"`         // 行数
	Cols        int       `json:"cols"`         // 列数
	WorkingDir  string    `json:"working_dir"`  // 工作目录
	Env         map[string]string `json:"env"`  // 环境变量
	Status      string    `json:"status"`       // 状态
	StartedAt   time.Time `json:"started_at"`   // 开始时间
	LastInputAt time.Time `json:"last_input_at"` // 最后输入时间
}

// TerminalCommand 终端命令.
type TerminalCommand struct {
	ID        string    `json:"id"`         // 命令ID
	SessionID string    `json:"session_id"` // 会话ID
	Command   string    `json:"command"`    // 命令内容
	Output    string    `json:"output"`     // 输出内容
	ExitCode  int       `json:"exit_code"`  // 退出码
	StartedAt time.Time `json:"started_at"` // 开始时间
	EndedAt   time.Time `json:"ended_at"`   // 结束时间
	Duration  int64     `json:"duration"`   // 执行时间(毫秒)
}

// ========== 文件传输 ==========

// FileTransfer 文件传输.
type FileTransfer struct {
	ID          string        `json:"id"`           // 传输ID
	SessionID   string        `json:"session_id"`   // 会话ID
	Direction   string        `json:"direction"`    // 方向(upload/download)
	FileName    string        `json:"file_name"`    // 文件名
	FilePath    string        `json:"file_path"`    // 文件路径
	FileSize    int64         `json:"file_size"`    // 文件大小
	Transferred int64         `json:"transferred"`  // 已传输字节数
	Progress    float64       `json:"progress"`     // 进度(0-100)
	Speed       int64         `json:"speed"`        // 传输速度(bytes/s)
	Status      string        `json:"status"`       // 状态
	Error       string        `json:"error"`        // 错误信息
	StartedAt   time.Time     `json:"started_at"`   // 开始时间
	CompletedAt *time.Time    `json:"completed_at"` // 完成时间
	Hash        string        `json:"hash"`         // 文件哈希
}

// FileInfo 文件信息.
type FileInfo struct {
	Name      string    `json:"name"`       // 文件名
	Path      string    `json:"path"`       // 文件路径
	Size      int64     `json:"size"`       // 文件大小
	IsDir     bool      `json:"is_dir"`     // 是否目录
	ModTime   time.Time `json:"mod_time"`   // 修改时间
	Mode      string    `json:"mode"`       // 权限
	MimeType  string    `json:"mime_type"`  // MIME类型
}

// ========== 认证和授权 ==========

// Credential 认证凭证.
type Credential struct {
	ID          string    `json:"id"`           // 凭证ID
	UserID      string    `json:"user_id"`      // 用户ID
	Username    string    `json:"username"`      // 用户名
	Token       string    `json:"token"`        // 认证令牌
	RefreshToken string   `json:"refresh_token"` // 刷新令牌
	ExpiresAt   time.Time `json:"expires_at"`   // 过期时间
	Permissions []string  `json:"permissions"`  // 权限列表
	IPAddress   string    `json:"ip_address"`   // IP地址
	UserAgent   string    `json:"user_agent"`   // 用户代理
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
	LastUsedAt  time.Time `json:"last_used_at"` // 最后使用时间
}

// AccessPolicy 访问策略.
type AccessPolicy struct {
	ID          string          `json:"id"`           // 策略ID
	Name        string          `json:"name"`         // 策略名称
	Description string          `json:"description"`  // 描述
	Users       []string        `json:"users"`        // 用户列表
	Groups      []string        `json:"groups"`       // 组列表
	Permission  PermissionLevel `json:"permission"`   // 权限级别
	Resources   []string        `json:"resources"`    // 资源列表
	TimeRange   *TimeRange      `json:"time_range"`   // 时间范围
	IPWhitelist []string        `json:"ip_whitelist"` // IP白名单
	Enabled     bool            `json:"enabled"`      // 是否启用
	CreatedAt   time.Time       `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time       `json:"updated_at"`   // 更新时间
}

// TimeRange 时间范围.
type TimeRange struct {
	StartTime string `json:"start_time"` // 开始时间(HH:MM)
	EndTime   string `json:"end_time"`   // 结束时间(HH:MM)
	Days      []int  `json:"days"`       // 星期几(0-6)
	Timezone  string `json:"timezone"`   // 时区
}

// ========== 会话录制 ==========

// Recording 会话录制.
type Recording struct {
	ID          string        `json:"id"`           // 录制ID
	SessionID   string        `json:"session_id"`   // 会话ID
	FileName    string        `json:"file_name"`    // 文件名
	FilePath    string        `json:"file_path"`    // 文件路径
	FileSize    int64         `json:"file_size"`    // 文件大小
	Duration    int64         `json:"duration"`     // 持续时间(秒)
	Format      string        `json:"format"`       // 格式
	Resolution  string        `json:"resolution"`   // 分辨率
	Status      string        `json:"status"`       // 状态
	StartedAt   time.Time     `json:"started_at"`   // 开始时间
	EndedAt     *time.Time    `json:"ended_at"`     // 结束时间
	CreatedBy   string        `json:"created_by"`   // 创建者
	Tags        []string      `json:"tags"`         // 标签
}

// RecordingEvent 录制事件.
type RecordingEvent struct {
	ID          string    `json:"id"`           // 事件ID
	RecordingID string    `json:"recording_id"` // 录制ID
	Type        string    `json:"type"`         // 事件类型
	Data        []byte    `json:"data"`         // 事件数据
	Timestamp   int64     `json:"timestamp"`    // 时间戳
	Sequence    int64     `json:"sequence"`     // 序列号
}

// ========== 文字聊天 ==========

// ChatMessage 聊天消息.
type ChatMessage struct {
	ID        string    `json:"id"`         // 消息ID
	SessionID string    `json:"session_id"` // 会话ID
	SenderID  string    `json:"sender_id"`  // 发送者ID
	SenderName string   `json:"sender_name"` // 发送者名称
	Type      string    `json:"type"`       // 消息类型(text/file/system)
	Content   string    `json:"content"`    // 消息内容
	Timestamp time.Time `json:"timestamp"`  // 时间戳
	Metadata  map[string]string `json:"metadata"` // 元数据
}

// ========== 操作审计 ==========

// AuditEvent 审计事件.
type AuditEvent struct {
	ID          string                 `json:"id"`           // 事件ID
	SessionID   string                 `json:"session_id"`   // 会话ID
	UserID      string                 `json:"user_id"`      // 用户ID
	Username    string                 `json:"username"`      // 用户名
	Action      string                 `json:"action"`       // 操作类型
	Resource    string                 `json:"resource"`     // 资源
	Details     map[string]interface{} `json:"details"`      // 详情
	IPAddress   string                 `json:"ip_address"`   // IP地址
	UserAgent   string                 `json:"user_agent"`   // 用户代理
	Status      string                 `json:"status"`       // 状态(success/failure)
	Timestamp   time.Time              `json:"timestamp"`    // 时间戳
	RiskLevel   string                 `json:"risk_level"`   // 风险级别(low/medium/high)
}

// ========== 统计信息 ==========

// Stats 远程协助统计.
type Stats struct {
	TotalSessions   int   `json:"total_sessions"`   // 总会话数
	ActiveSessions  int   `json:"active_sessions"`  // 活跃会话数
	TotalRecordings int   `json:"total_recordings"` // 总录制数
	TotalTransfers  int   `json:"total_transfers"`  // 总传输数
	TotalBytes      int64 `json:"total_bytes"`      // 总传输字节数
	AvgDuration     int64 `json:"avg_duration"`     // 平均会话时长
	TotalUsers      int   `json:"total_users"`      // 总用户数
	OnlineUsers     int   `json:"online_users"`     // 在线用户数
}

// ========== 配置 ==========

// Config 远程协助配置.
type Config struct {
	Enabled         bool              `json:"enabled"`           // 是否启用
	BindAddress     string            `json:"bind_address"`      // 绑定地址
	BindPort        int               `json:"bind_port"`         // 绑定端口
	ExternalURL     string            `json:"external_url"`      // 外部URL
	MaxSessions     int               `json:"max_sessions"`      // 最大会话数
	MaxDuration     int               `json:"max_duration"`      // 最大持续时间(秒)
	TokenExpiry     int               `json:"token_expiry"`      // 令牌过期时间(秒)
	Recording       *RecordingConfig  `json:"recording"`         // 录制配置
	Security        *SecurityConfig   `json:"security"`          // 安全配置
	RateLimit       *RateLimitConfig  `json:"rate_limit"`        // 限流配置
	Storage         *StorageConfig    `json:"storage"`           // 存储配置
}

// RecordingConfig 录制配置.
type RecordingConfig struct {
	Enabled       bool   `json:"enabled"`        // 是否启用
	AutoRecord    bool   `json:"auto_record"`    // 自动录制
	Format        string `json:"format"`         // 格式
	Resolution    string `json:"resolution"`     // 分辨率
	MaxSize       int64  `json:"max_size"`       // 最大文件大小
	RetentionDays int    `json:"retention_days"` // 保留天数
	StoragePath   string `json:"storage_path"`   // 存储路径
}

// SecurityConfig 安全配置.
type SecurityConfig struct {
	Encryption    bool     `json:"encryption"`     // 是否加密
	TLS           bool     `json:"tls"`            // 是否TLS
	CertFile      string   `json:"cert_file"`      // 证书文件
	KeyFile       string   `json:"key_file"`       // 密钥文件
	AllowedIPs    []string `json:"allowed_ips"`    // 允许的IP
	BlockedIPs    []string `json:"blocked_ips"`    // 阻止的IP
	MaxAttempts   int      `json:"max_attempts"`   // 最大尝试次数
	LockoutTime   int      `json:"lockout_time"`   // 锁定时间(秒)
}

// RateLimitConfig 限流配置.
type RateLimitConfig struct {
	Enabled       bool `json:"enabled"`        // 是否启用
	RequestsPerMin int  `json:"requests_per_min"` // 每分钟请求数
	BurstSize     int  `json:"burst_size"`     // 突发大小
}

// StorageConfig 存储配置.
type StorageConfig struct {
	Type      string `json:"type"`       // 类型(local/s3/oss)
	Path      string `json:"path"`       // 路径
	Bucket    string `json:"bucket"`     // 存储桶
	Region    string `json:"region"`     // 区域
	Endpoint  string `json:"endpoint"`   // 端点
	AccessKey string `json:"access_key"` // 访问密钥
	SecretKey string `json:"secret_key"` // 秘密密钥
}
