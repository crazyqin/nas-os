// Package remotesupport 提供远程支持隧道功能
package remotesupport

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrSessionNotFound 会话不存在.
	ErrSessionNotFound = errors.New("会话不存在")
	// ErrSessionExpired 会话已过期.
	ErrSessionExpired = errors.New("会话已过期")
	// ErrSessionClosed 会话已关闭.
	ErrSessionClosed = errors.New("会话已关闭")
	// ErrTokenInvalid 令牌无效.
	ErrTokenInvalid = errors.New("令牌无效")
	// ErrTokenUsed 令牌已使用.
	ErrTokenUsed = errors.New("令牌已使用")
	// ErrPermissionDenied 权限不足.
	ErrPermissionDenied = errors.New("权限不足")
	// ErrBandwidthLimit 带宽超限.
	ErrBandwidthLimit = errors.New("带宽超限")
	// ErrTunnelFailed 隧道建立失败.
	ErrTunnelFailed = errors.New("隧道建立失败")
)

// ========== 会话状态 ==========

// SessionStatus 会话状态.
type SessionStatus string

const (
	// SessionStatusPending 等待连接.
	SessionStatusPending SessionStatus = "pending"
	// SessionStatusActive 活跃中.
	SessionStatusActive SessionStatus = "active"
	// SessionStatusSuspended 已暂停.
	SessionStatusSuspended SessionStatus = "suspended"
	// SessionStatusClosed 已关闭.
	SessionStatusClosed SessionStatus = "closed"
	// SessionStatusExpired 已过期.
	SessionStatusExpired SessionStatus = "expired"
)

// ========== 访问级别 ==========

// AccessLevel 访问级别.
type AccessLevel string

const (
	// AccessLevelReadOnly 只读.
	AccessLevelReadOnly AccessLevel = "readonly"
	// AccessLevelReadWrite 读写.
	AccessLevelReadWrite AccessLevel = "readwrite"
	// AccessLevelAdmin 管理员.
	AccessLevelAdmin AccessLevel = "admin"
)

// ========== 数据结构 ==========

// Session 远程支持会话.
type Session struct {
	ID           string        `json:"id"`            // 会话 ID
	Token        string        `json:"token"`         // 一次性访问令牌
	Status       SessionStatus `json:"status"`        // 会话状态
	AccessLevel  AccessLevel   `json:"access_level"`  // 访问级别
	ClientIP     string        `json:"client_ip"`     // 客户端 IP
	ClientName   string        `json:"client_name"`   // 客户端名称
	TargetHost   string        `json:"target_host"`   // 目标主机
	TargetPort   int           `json:"target_port"`   // 目标端口
	BandwidthKB  int           `json:"bandwidth_kb"`  // 带宽限制（KB/s）
	MaxDuration  time.Duration `json:"max_duration"`  // 最大持续时间
	StartedAt    time.Time     `json:"started_at"`    // 开始时间
	EndedAt      *time.Time    `json:"ended_at"`      // 结束时间
	ExpiresAt    time.Time     `json:"expires_at"`    // 过期时间
	BytesUp      int64         `json:"bytes_up"`      // 上传字节数
	BytesDown    int64         `json:"bytes_down"`    // 下载字节数
	Recorded     bool          `json:"recorded"`      // 是否录制
	RecordingPath string       `json:"recording_path"` // 录制文件路径
	AuditLog     []AuditEntry  `json:"audit_log"`     // 审计日志
}

// AuditEntry 审计日志条目.
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"` // 时间
	Action    string    `json:"action"`    // 操作
	Detail    string    `json:"detail"`    // 详情
	Source    string    `json:"source"`    // 来源
}

// AccessToken 访问令牌.
type AccessToken struct {
	Token      string      `json:"token"`       // 令牌值
	SessionID  string      `json:"session_id"`  // 关联会话
	Used       bool        `json:"used"`        // 是否已使用
	ExpiresAt  time.Time   `json:"expires_at"`  // 过期时间
	CreatedAt  time.Time   `json:"created_at"`  // 创建时间
	UsedAt     *time.Time  `json:"used_at"`     // 使用时间
}

// TunnelInfo 隧道信息.
type TunnelInfo struct {
	SessionID  string    `json:"session_id"`  // 会话 ID
	LocalAddr  string    `json:"local_addr"`  // 本地地址
	RemoteAddr string    `json:"remote_addr"` // 远程地址
	Status     string    `json:"status"`      // 隧道状态
	CreatedAt  time.Time `json:"created_at"`  // 创建时间
}

// ========== 请求/响应 ==========

// SessionCreateRequest 创建会话请求.
type SessionCreateRequest struct {
	ClientName  string      `json:"client_name" binding:"required"` // 客户端名称
	TargetHost  string      `json:"target_host" binding:"required"` // 目标主机
	TargetPort  int         `json:"target_port"`                    // 目标端口
	AccessLevel AccessLevel `json:"access_level"`                   // 访问级别
	BandwidthKB int         `json:"bandwidth_kb"`                   // 带宽限制（KB/s）
	MaxDuration int         `json:"max_duration_sec"`               // 最大持续时间（秒）
	Recorded    bool        `json:"recorded"`                       // 是否录制
}

// SessionUpdateRequest 更新会话请求.
type SessionUpdateRequest struct {
	Status      *SessionStatus `json:"status"`       // 状态
	AccessLevel *AccessLevel   `json:"access_level"` // 访问级别
	BandwidthKB *int           `json:"bandwidth_kb"` // 带宽限制
}

// TokenValidateRequest 验证令牌请求.
type TokenValidateRequest struct {
	Token   string `json:"token" binding:"required"` // 令牌值
	ClientIP string `json:"client_ip"`               // 客户端 IP
}

// SessionStats 会话统计.
type SessionStats struct {
	TotalSessions   int   `json:"total_sessions"`   // 总会话数
	ActiveSessions  int   `json:"active_sessions"`  // 活跃会话数
	TotalBytesUp    int64 `json:"total_bytes_up"`   // 总上传字节
	TotalBytesDown  int64 `json:"total_bytes_down"` // 总下载字节
	TotalAuditEntries int `json:"total_audit_entries"` // 总审计条目
}
