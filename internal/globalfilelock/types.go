// Package globalfilelock 提供全局文件锁定功能，防止跨站点文件冲突。
// 参考群晖 Hybrid Share 的全局文件锁定特性，支持分布式文件锁管理、
// 跨站点锁同步、冲突检测与解决、锁范围控制和统计分析。
package globalfilelock

import (
	"fmt"
	"time"
)

// ============================================================
// 锁类型定义
// ============================================================

// LockType 锁类型
type LockType string

const (
	// LockTypeRead 读锁：允许多个用户同时读取
	LockTypeRead LockType = "read"
	// LockTypeWrite 写锁：独占写入，阻止其他读写操作
	LockTypeWrite LockType = "write"
)

// IsValid 检查锁类型是否有效
func (lt LockType) IsValid() bool {
	switch lt {
	case LockTypeRead, LockTypeWrite:
		return true
	}
	return false
}

// String 返回锁类型的字符串表示
func (lt LockType) String() string {
	return string(lt)
}

// ============================================================
// 锁范围定义
// ============================================================

// LockScope 锁范围
type LockScope string

const (
	// LockScopeLocal 本地锁：仅在当前站点有效
	LockScopeLocal LockScope = "local"
	// LockScopeGlobal 全局锁：跨所有站点生效
	LockScopeGlobal LockScope = "global"
	// LockScopeSite 站点锁：仅在指定站点生效
	LockScopeSite LockScope = "site"
)

// IsValid 检查锁范围是否有效
func (ls LockScope) IsValid() bool {
	switch ls {
	case LockScopeLocal, LockScopeGlobal, LockScopeSite:
		return true
	}
	return false
}

// String 返回锁范围的字符串表示
func (ls LockScope) String() string {
	return string(ls)
}

// ============================================================
// 锁状态定义
// ============================================================

// LockStatus 锁状态
type LockStatus string

const (
	// LockStatusActive 活跃状态
	LockStatusActive LockStatus = "active"
	// LockStatusExpired 已过期
	LockStatusExpired LockStatus = "expired"
	// LockStatusReleased 已释放
	LockStatusReleased LockStatus = "released"
	// LockStatusForceReleased 被管理员强制释放
	LockStatusForceReleased LockStatus = "force_released"
)

// ============================================================
// 冲突解决策略
// ============================================================

// ConflictResolution 冲突解决策略
type ConflictResolution string

const (
	// ResolutionLastWriteWins 最后写入胜出
	ResolutionLastWriteWins ConflictResolution = "last_write_wins"
	// ResolutionManual 手动解决
	ResolutionManual ConflictResolution = "manual"
	// ResolutionAutoMerge 自动合并
	ResolutionAutoMerge ConflictResolution = "auto_merge"
)

// ============================================================
// 核心数据结构
// ============================================================

// FileLock 文件锁信息
type FileLock struct {
	// 唯一标识
	ID string `json:"id"`
	// 锁定的文件路径
	FilePath string `json:"file_path"`
	// 锁定者ID
	HolderID string `json:"holder_id"`
	// 锁定者名称
	HolderName string `json:"holder_name"`
	// 锁类型
	LockType LockType `json:"lock_type"`
	// 锁范围
	LockScope LockScope `json:"lock_scope"`
	// 站点ID（LockScopeSite 时有效）
	SiteID string `json:"site_id,omitempty"`
	// 获取锁的时间
	AcquiredAt time.Time `json:"acquired_at"`
	// 过期时间
	ExpiresAt time.Time `json:"expires_at"`
	// 最后续期时间
	LastRenewedAt time.Time `json:"last_renewed_at"`
	// 锁状态
	Status LockStatus `json:"status"`
	// 释放时间
	ReleasedAt *time.Time `json:"released_at,omitempty"`
	// 释放者ID
	ReleasedBy string `json:"released_by,omitempty"`
	// 备注
	Comment string `json:"comment,omitempty"`
	// 创建时间
	CreatedAt time.Time `json:"created_at"`
	// 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// LockConflict 锁冲突信息
type LockConflict struct {
	// 冲突ID
	ID string `json:"id"`
	// 文件路径
	FilePath string `json:"file_path"`
	// 冲突的锁列表
	ConflictingLocks []*FileLock `json:"conflicting_locks"`
	// 冲突发生时间
	DetectedAt time.Time `json:"detected_at"`
	// 冲突解决状态
	Resolved bool `json:"resolved"`
	// 解决策略
	Resolution ConflictResolution `json:"resolution"`
	// 解决时间
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	// 解决者ID
	ResolvedBy string `json:"resolved_by,omitempty"`
	// 解决详情
	ResolutionDetail string `json:"resolution_detail,omitempty"`
}

// LockStatistics 锁统计信息
type LockStatistics struct {
	// 活跃锁总数
	ActiveLocks int `json:"active_locks"`
	// 读锁数量
	ReadLocks int `json:"read_locks"`
	// 写锁数量
	WriteLocks int `json:"write_locks"`
	// 本地锁数量
	LocalLocks int `json:"local_locks"`
	// 全局锁数量
	GlobalLocks int `json:"global_locks"`
	// 站点锁数量
	SiteLocks int `json:"site_locks"`
	// 等待获取锁的请求数
	WaitingCount int `json:"waiting_count"`
	// 冲突总数
	TotalConflicts int `json:"total_conflicts"`
	// 已解决冲突数
	ResolvedConflicts int `json:"resolved_conflicts"`
	// 未解决冲突数
	UnresolvedConflicts int `json:"unresolved_conflicts"`
	// 平均锁持有时间（秒）
	AverageHoldTime float64 `json:"average_hold_time_seconds"`
	// 今日获取锁次数
	TodayAcquisitions int `json:"today_acquisitions"`
	// 今日释放锁次数
	TodayReleases int `json:"today_releases"`
	// 活跃站点数
	ActiveSites int `json:"active_sites"`
}

// SiteInfo 站点信息
type SiteInfo struct {
	// 站点ID
	ID string `json:"id"`
	// 站点名称
	Name string `json:"name"`
	// 站点地址
	Address string `json:"address"`
	// 是否在线
	Online bool `json:"online"`
	// 最后心跳时间
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

// SyncMessage 跨站点同步消息
type SyncMessage struct {
	// 消息ID
	ID string `json:"id"`
	// 消息类型
	Type string `json:"type"`
	// 源站点ID
	SourceSiteID string `json:"source_site_id"`
	// 目标站点ID（空表示广播）
	TargetSiteID string `json:"target_site_id,omitempty"`
	// 锁信息
	Lock *FileLock `json:"lock,omitempty"`
	// 时间戳
	Timestamp time.Time `json:"timestamp"`
}

// ============================================================
// 请求/响应结构
// ============================================================

// AcquireLockRequest 获取锁请求
type AcquireLockRequest struct {
	// 文件路径
	FilePath string `json:"file_path"`
	// 锁定者ID
	HolderID string `json:"holder_id"`
	// 锁定者名称
	HolderName string `json:"holder_name"`
	// 锁类型
	LockType LockType `json:"lock_type"`
	// 锁范围
	LockScope LockScope `json:"lock_scope"`
	// 站点ID（LockScopeSite 时必填）
	SiteID string `json:"site_id,omitempty"`
	// 锁定时长（分钟）
	Duration int `json:"duration"`
	// 备注
	Comment string `json:"comment,omitempty"`
}

// ReleaseLockRequest 释放锁请求
type ReleaseLockRequest struct {
	// 锁ID
	LockID string `json:"lock_id"`
	// 释放者ID
	HolderID string `json:"holder_id"`
}

// RenewLockRequest 续期请求
type RenewLockRequest struct {
	// 锁ID
	LockID string `json:"lock_id"`
	// 续期者ID
	HolderID string `json:"holder_id"`
	// 续期时长（分钟）
	Duration int `json:"duration"`
}

// UpgradeLockRequest 升级锁请求（读锁升级为写锁）
type UpgradeLockRequest struct {
	// 锁ID
	LockID string `json:"lock_id"`
	// 持有者ID
	HolderID string `json:"holder_id"`
}

// DowngradeLockRequest 降级锁请求（写锁降级为读锁）
type DowngradeLockRequest struct {
	// 锁ID
	LockID string `json:"lock_id"`
	// 持有者ID
	HolderID string `json:"holder_id"`
}

// ResolveConflictRequest 解决冲突请求
type ResolveConflictRequest struct {
	// 冲突ID
	ConflictID string `json:"conflict_id"`
	// 解决策略
	Resolution ConflictResolution `json:"resolution"`
	// 解决者ID
	ResolvedBy string `json:"resolved_by"`
	// 保留的锁ID（手动解决时）
	PreferredLockID string `json:"preferred_lock_id,omitempty"`
	// 解决详情
	Detail string `json:"detail,omitempty"`
}

// APIResponse 标准 API 响应
type APIResponse struct {
	// 状态码
	Code int `json:"code"`
	// 消息
	Message string `json:"message"`
	// 数据
	Data interface{} `json:"data,omitempty"`
}

// ============================================================
// 策略配置
// ============================================================

// LockPolicy 锁策略配置
type LockPolicy struct {
	// 是否启用全局文件锁
	Enabled bool `json:"enabled"`
	// 默认锁类型
	DefaultLockType LockType `json:"default_lock_type"`
	// 默认锁范围
	DefaultLockScope LockScope `json:"default_lock_scope"`
	// 读锁最大持续时间（分钟）
	ReadLockMaxDuration int `json:"read_lock_max_duration"`
	// 写锁最大持续时间（分钟）
	WriteLockMaxDuration int `json:"write_lock_max_duration"`
	// 是否允许续期
	AllowRenewal bool `json:"allow_renewal"`
	// 是否允许升级/降级
	AllowUpgradeDowngrade bool `json:"allow_upgrade_downgrade"`
	// 默认冲突解决策略
	DefaultResolution ConflictResolution `json:"default_resolution"`
	// 同步超时（秒）
	SyncTimeout int `json:"sync_timeout"`
	// 心跳间隔（秒）
	HeartbeatInterval int `json:"heartbeat_interval"`
	// 站点离线超时（秒）
	SiteOfflineTimeout int `json:"site_offline_timeout"`
	// 每个用户最大锁数量
	MaxLocksPerUser int `json:"max_locks_per_user"`
	// 锁历史保留天数
	HistoryRetentionDays int `json:"history_retention_days"`
}

// DefaultLockPolicy 返回默认锁策略
func DefaultLockPolicy() *LockPolicy {
	return &LockPolicy{
		Enabled:                true,
		DefaultLockType:        LockTypeWrite,
		DefaultLockScope:       LockScopeGlobal,
		ReadLockMaxDuration:    1440, // 24小时
		WriteLockMaxDuration:   480,  // 8小时
		AllowRenewal:           true,
		AllowUpgradeDowngrade:  true,
		DefaultResolution:      ResolutionLastWriteWins,
		SyncTimeout:            30,
		HeartbeatInterval:      60,
		SiteOfflineTimeout:     300,
		MaxLocksPerUser:        20,
		HistoryRetentionDays:   90,
	}
}

// ============================================================
// 辅助函数
// ============================================================

// ValidateAcquireRequest 验证获取锁请求
func ValidateAcquireRequest(req *AcquireLockRequest) error {
	if req.FilePath == "" {
		return fmt.Errorf("文件路径不能为空")
	}
	if req.HolderID == "" {
		return fmt.Errorf("锁定者ID不能为空")
	}
	if req.HolderName == "" {
		return fmt.Errorf("锁定者名称不能为空")
	}
	if req.LockType != "" && !req.LockType.IsValid() {
		return fmt.Errorf("无效的锁类型: %s", req.LockType)
	}
	if req.LockScope != "" && !req.LockScope.IsValid() {
		return fmt.Errorf("无效的锁范围: %s", req.LockScope)
	}
	if req.LockScope == LockScopeSite && req.SiteID == "" {
		return fmt.Errorf("站点锁必须指定站点ID")
	}
	return nil
}
