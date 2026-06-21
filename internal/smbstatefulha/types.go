package smbstatefulha

import (
	"sync"
	"time"
)

// HANodeState HA 节点状态
type HANodeState string

const (
	NodeStateActive   HANodeState = "active"
	NodeStateStandby  HANodeState = "standby"
	NodeStateFailed   HANodeState = "failed"
	NodeStateSyncing  HANodeState = "syncing"
)

// FailoverTrigger 故障转移触发条件
type FailoverTrigger string

const (
	TriggerHeartbeat FailoverTrigger = "heartbeat"
	TriggerManual    FailoverTrigger = "manual"
	TriggerScheduled FailoverTrigger = "scheduled"
)

// SMBSession SMB 会话信息
type SMBSession struct {
	ID            string    `json:"id"`
	ClientIP      string    `json:"clientIP"`
	Username      string    `json:"username"`
	ShareName     string    `json:"shareName"`
	TreeID        uint32    `json:"treeID"`
	FileHandles   []string  `json:"fileHandles"`
	Authenticated bool      `json:"authenticated"`
	CreatedAt     time.Time `json:"createdAt"`
	LastActivity  time.Time `json:"lastActivity"`
	State         string    `json:"state"` // active, disconnected, migrating
}

// HANode HA 节点信息
type HANode struct {
	mu           sync.RWMutex
	ID           string      `json:"id"`
	Hostname     string      `json:"hostname"`
	IPAddress    string      `json:"ipAddress"`
	VirtualIP    string      `json:"virtualIP"`
	State        HANodeState `json:"state"`
	Priority     int         `json:"priority"` // 优先级，数字越小优先级越高
	LastHeartbeat time.Time   `json:"lastHeartbeat"`
	Sessions     map[string]*SMBSession `json:"sessions"`
	SyncStatus   string      `json:"syncStatus"`
	Uptime       time.Duration `json:"uptime"`
}

// HAConfig HA 配置
type HAConfig struct {
	VirtualIP         string        `json:"virtualIP"`         // 虚拟 IP
	HeartbeatInterval time.Duration `json:"heartbeatInterval"` // 心跳间隔
	FailoverTimeout   time.Duration `json:"failoverTimeout"`   // 故障转移超时
	MaxSessions       int           `json:"maxSessions"`       // 最大会话数
	SessionSyncEnabled bool         `json:"sessionSyncEnabled"` // 启用会话同步
	AutoFailback      bool          `json:"autoFailback"`      // 自动回切
	PreemptMode       bool          `json:"preemptMode"`       // 抢占模式
}

// DefaultHAConfig 默认配置
func DefaultHAConfig() *HAConfig {
	return &HAConfig{
		VirtualIP:         "192.168.1.100",
		HeartbeatInterval: 1 * time.Second,
		FailoverTimeout:   10 * time.Second,
		MaxSessions:       1000,
		SessionSyncEnabled: true,
		AutoFailback:      true,
		PreemptMode:       false,
	}
}

// HAManager HA 管理器
type HAManager struct {
	mu           sync.RWMutex
	config       *HAConfig
	localNode    *HANode
	remoteNode   *HANode
	sessions     map[string]*SMBSession
	failoverLog  []FailoverEvent
	running      bool
	stopCh       chan struct{}
	onFailover   func(event FailoverEvent)
}

// FailoverEvent 故障转移事件
type FailoverEvent struct {
	ID          string         `json:"id"`
	Timestamp   time.Time      `json:"timestamp"`
	Trigger     FailoverTrigger `json:"trigger"`
	FromNode    string         `json:"fromNode"`
	ToNode      string         `json:"toNode"`
	Reason      string         `json:"reason"`
	SessionsAffected int       `json:"sessionsAffected"`
	TotalSessions int           `json:"totalSessions"`
	Duration    time.Duration  `json:"duration"`
	Success     bool           `json:"success"`
}

// SyncState 同步状态
type SyncState struct {
	LastSyncTime   time.Time `json:"lastSyncTime"`
	SyncedSessions int       `json:"syncedSessions"`
	PendingSync    int       `json:"pendingSync"`
	SyncErrors     int       `json:"syncErrors"`
	BytesTransfered int64    `json:"bytesTransfered"`
}

// FailoverState 故障转移状态
type FailoverState struct {
	InProgress    bool          `json:"inProgress"`
	StartTime     time.Time     `json:"startTime"`
	SourceNode    string        `json:"sourceNode"`
	TargetNode    string        `json:"targetNode"`
	Progress      int           `json:"progress"` // 0-100
	CurrentStep   string        `json:"currentStep"`
	SessionsMigrated int       `json:"sessionsMigrated"`
	TotalSessions int           `json:"totalSessions"`
}
