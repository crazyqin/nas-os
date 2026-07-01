// Package filelock 提供文件锁定功能，防止协作同步错误。
// 对标群晖 DSM 7.3 的 File Lock 功能，支持文件级锁定/解锁、
// 锁定超时自动释放（默认 30 分钟）及锁定列表查询。
package filelock

import (
	"time"

	"github.com/google/uuid"
)

// LockStatus 锁状态
type LockStatus string

const (
	// LockStatusActive 活跃
	LockStatusActive LockStatus = "active"
	// LockStatusReleased 已释放
	LockStatusReleased LockStatus = "released"
	// LockStatusExpired 已过期
	LockStatusExpired LockStatus = "expired"
)

// LockInfo 文件锁信息
type LockInfo struct {
	// 锁唯一标识
	ID string `json:"id"`
	// 被锁定的文件路径
	FilePath string `json:"file_path"`
	// 持有锁的用户 ID
	UserID string `json:"user_id"`
	// 持有锁的用户名
	UserName string `json:"user_name"`
	// 锁状态
	Status LockStatus `json:"status"`
	// 获取锁的时间
	AcquiredAt time.Time `json:"acquired_at"`
	// 过期时间
	ExpiresAt time.Time `json:"expires_at"`
	// 释放时间（已释放时有效）
	ReleasedAt *time.Time `json:"released_at,omitempty"`
	// 锁定备注
	Comment string `json:"comment,omitempty"`
	// 创建时间
	CreatedAt time.Time `json:"created_at"`
	// 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// LockRequest 锁定请求
// API: POST /api/v1/filelock/lock
type LockRequest struct {
	// 文件路径
	FilePath string `json:"file_path" binding:"required"`
	// 用户 ID
	UserID string `json:"user_id" binding:"required"`
	// 用户名
	UserName string `json:"user_name" binding:"required"`
	// 锁定时长（分钟），可选，默认 30
	DurationMinutes int `json:"duration_minutes,omitempty"`
	// 备注
	Comment string `json:"comment,omitempty"`
}

// UnlockRequest 解锁请求
// API: POST /api/v1/filelock/unlock
type UnlockRequest struct {
	// 锁 ID（与 file_path 二选一）
	LockID string `json:"lock_id,omitempty"`
	// 文件路径（与 lock_id 二选一，指定则解锁该文件上该用户的所有锁）
	FilePath string `json:"file_path,omitempty"`
	// 用户 ID
	UserID string `json:"user_id" binding:"required"`
}

// ListLocksResponse 锁定列表响应
// API: GET /api/v1/filelock/list
type ListLocksResponse struct {
	// 活跃锁列表
	Locks []*LockInfo `json:"locks"`
	// 活跃锁总数
	Total int `json:"total"`
}

// LockResponse 锁定操作响应
type LockResponse struct {
	// 锁信息
	Lock *LockInfo `json:"lock"`
	// 是否成功
	Success bool `json:"success"`
}

// UnlockResponse 解锁操作响应
type UnlockResponse struct {
	// 已释放的锁数量
	ReleasedCount int `json:"released_count"`
	// 是否成功
	Success bool `json:"success"`
}

// Config 文件锁配置
type Config struct {
	// 是否启用文件锁定
	Enabled bool `json:"enabled"`
	// 默认锁定时长（分钟）
	DefaultDurationMinutes int `json:"default_duration_minutes"`
	// 锁清理间隔（分钟）
	CleanupIntervalMinutes int `json:"cleanup_interval_minutes"`
	// 每用户最大锁数量
	MaxLocksPerUser int `json:"max_locks_per_user"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:                true,
		DefaultDurationMinutes: 30, // 默认 30 分钟
		CleanupIntervalMinutes: 5,  // 每 5 分钟清理过期锁
		MaxLocksPerUser:        50,
	}
}

// newLockInfo 创建锁信息（内部辅助函数）
func newLockInfo(req *LockRequest, cfg *Config) *LockInfo {
	now := time.Now()
	duration := cfg.DefaultDurationMinutes
	if req.DurationMinutes > 0 {
		duration = req.DurationMinutes
	}

	return &LockInfo{
		ID:          uuid.New().String(),
		FilePath:    req.FilePath,
		UserID:      req.UserID,
		UserName:    req.UserName,
		Status:      LockStatusActive,
		AcquiredAt:  now,
		ExpiresAt:   now.Add(time.Duration(duration) * time.Minute),
		Comment:     req.Comment,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
