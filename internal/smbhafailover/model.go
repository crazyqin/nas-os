// Package smbhafailover 提供 SMB 有状态高可用故障转移功能，
// 支持会话状态跨故障转移保持，客户端重连无需重新认证。
// 对标 TrueNAS 26 的 SMB HA 能力。
package smbhafailover

import "time"

// SessionState SMB 会话状态
type SessionState struct {
	// 会话唯一标识
	SessionID string `json:"session_id"`
	// 客户端 IP 地址
	ClientIP string `json:"client_ip"`
	// 用户名
	Username string `json:"username"`
	// 域名
	Domain string `json:"domain"`
	// 会话密钥
	SessionKey string `json:"session_key"`
	// 最后访问时间
	LastAccess time.Time `json:"last_access"`
	// 挂载的共享列表
	Shares []string `json:"shares,omitempty"`
	// 打开文件句柄数
	OpenHandles int `json:"open_handles"`
}

// Snapshot 会话状态快照
type Snapshot struct {
	// 快照唯一标识
	ID string `json:"id"`
	// 快照创建时间
	CreatedAt time.Time `json:"created_at"`
	// 快照包含的会话列表
	Sessions []*SessionState `json:"sessions"`
	// 快照状态
	Status SnapshotStatus `json:"status"`
	// 来源节点
	SourceNode string `json:"source_node"`
}

// SnapshotStatus 快照状态
type SnapshotStatus string

const (
	SnapshotStatusActive   SnapshotStatus = "active"
	SnapshotStatusRestored SnapshotStatus = "restored"
	SnapshotStatusFailed   SnapshotStatus = "failed"
)

// FailoverStatus 故障转移状态
type FailoverStatus string

const (
	FailoverStatusIdle      FailoverStatus = "idle"
	FailoverStatusCapture   FailoverStatus = "capturing"
	FailoverStatusRestore   FailoverStatus = "restoring"
	FailoverStatusCompleted FailoverStatus = "completed"
	FailoverStatusFailed    FailoverStatus = "failed"
)

// FailoverState 故障转移状态信息
type FailoverState struct {
	Status    FailoverStatus `json:"status"`
	LastFail  *time.Time      `json:"last_fail,omitempty"`
	ActiveNode string         `json:"active_node"`
	SnapshotID string        `json:"snapshot_id,omitempty"`
}

// CreateSnapshotRequest 创建快照请求
type CreateSnapshotRequest struct {
	SourceNode string `json:"source_node,omitempty"`
}