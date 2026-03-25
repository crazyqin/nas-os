package auth

import (
	"sync"
	"time"
)

// LoginAttempt 登录尝试记录.
type LoginAttempt struct {
	Username  string    `json:"username"`
	IP        string    `json:"ip"`
	Success   bool      `json:"success"`
	Timestamp time.Time `json:"timestamp"`
}

// LoginAttemptTracker 登录尝试跟踪器.
type LoginAttemptTracker struct {
	mu                sync.RWMutex
	attempts          map[string]*UserAttempts // username -> attempts
	ipAttempts        map[string]*IPAttempts   // ip -> attempts
	maxAttempts       int                      // 最大尝试次数
	lockoutDuration   time.Duration            // 锁定时长
	attemptWindow     time.Duration            // 尝试计数窗口
	ipMaxAttempts     int                      // IP 最大尝试次数
	ipLockoutDuration time.Duration            // IP 锁定时长
}

// UserAttempts 用户登录尝试记录.
type UserAttempts struct {
	Attempts    []time.Time `json:"attempts"`
	LockedUntil *time.Time  `json:"locked_until,omitempty"`
	LastAttempt time.Time   `json:"last_attempt"`
	FailedCount int         `json:"failed_count"`
}

// IPAttempts IP 登录尝试记录.
type IPAttempts struct {
	Attempts    []time.Time `json:"attempts"`
	LockedUntil *time.Time  `json:"locked_until,omitempty"`
	FailedCount int         `json:"failed_count"`
}

// LoginAttemptConfig 登录尝试配置.
type LoginAttemptConfig struct {
	MaxAttempts       int           `json:"max_attempts"`        // 用户最大尝试次数（默认 5）
	LockoutDuration   time.Duration `json:"lockout_duration"`    // 锁定时长（默认 15 分钟）
	AttemptWindow     time.Duration `json:"attempt_window"`      // 尝试计数窗口（默认 30 分钟）
	IPMaxAttempts     int           `json:"ip_max_attempts"`     // IP 最大尝试次数（默认 20）
	IPLockoutDuration time.Duration `json:"ip_lockout_duration"` // IP 锁定时长（默认 1 小时）
}

// DefaultLoginAttemptConfig 默认配置.
var DefaultLoginAttemptConfig = LoginAttemptConfig{
	MaxAttempts:       5,
	LockoutDuration:   15 * time.Minute,
	AttemptWindow:     30 * time.Minute,
	IPMaxAttempts:     20,
	IPLockoutDuration: 1 * time.Hour,
}

// NewLoginAttemptTracker 创建登录尝试跟踪器.
func NewLoginAttemptTracker(config LoginAttemptConfig) *LoginAttemptTracker {
	return &LoginAttemptTracker{
		attempts:          make(map[string]*UserAttempts),
		ipAttempts:        make(map[string]*IPAttempts),
		maxAttempts:       config.MaxAttempts,
		lockoutDuration:   config.LockoutDuration,
		attemptWindow:     config.AttemptWindow,
		ipMaxAttempts:     config.IPMaxAttempts,
		ipLockoutDuration: config.IPLockoutDuration,
	}
}

// RecordAttempt 记录登录尝试.
func (t *LoginAttemptTracker) RecordAttempt(username, ip string, success bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	// 记录用户尝试
	userAttempts, exists := t.attempts[username]
	if !exists {
		userAttempts = &UserAttempts{
			Attempts: make([]time.Time, 0),
		}
		t.attempts[username] = userAttempts
	}

	userAttempts.LastAttempt = now

	if success {
		// 登录成功，清除失败计数
		userAttempts.FailedCount = 0
		userAttempts.LockedUntil = nil
		userAttempts.Attempts = make([]time.Time, 0)
	} else {
		// 登录失败
		userAttempts.FailedCount++
		userAttempts.Attempts = append(userAttempts.Attempts, now)

		// 清理过期的尝试记录
		t.cleanupUserAttempts(userAttempts, now)

		// 检查是否需要锁定
		if userAttempts.FailedCount >= t.maxAttempts {
			lockedUntil := now.Add(t.lockoutDuration)
			userAttempts.LockedUntil = &lockedUntil
		}
	}

	// 记录 IP 尝试
	ipAttempts, exists := t.ipAttempts[ip]
	if !exists {
		ipAttempts = &IPAttempts{
			Attempts: make([]time.Time, 0),
		}
		t.ipAttempts[ip] = ipAttempts
	}

	if !success {
		ipAttempts.FailedCount++
		ipAttempts.Attempts = append(ipAttempts.Attempts, now)

		// 清理过期的尝试记录
		t.cleanupIPAttempts(ipAttempts, now)

		// 检查是否需要锁定 IP
		if ipAttempts.FailedCount >= t.ipMaxAttempts {
			lockedUntil := now.Add(t.ipLockoutDuration)
			ipAttempts.LockedUntil = &lockedUntil
		}
	} else {
		// 登录成功，清除 IP 失败计数
		ipAttempts.FailedCount = 0
		ipAttempts.Attempts = make([]time.Time, 0)
	}
}

// IsLocked 检查用户是否被锁定.
func (t *LoginAttemptTracker) IsLocked(username string) (bool, time.Time) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	userAttempts, exists := t.attempts[username]
	if !exists || userAttempts.LockedUntil == nil {
		return false, time.Time{}
	}

	if time.Now().Before(*userAttempts.LockedUntil) {
		return true, *userAttempts.LockedUntil
	}

	return false, time.Time{}
}

// IsIPLocked 检查 IP 是否被锁定.
func (t *LoginAttemptTracker) IsIPLocked(ip string) (bool, time.Time) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	ipAttempts, exists := t.ipAttempts[ip]
	if !exists || ipAttempts.LockedUntil == nil {
		return false, time.Time{}
	}

	if time.Now().Before(*ipAttempts.LockedUntil) {
		return true, *ipAttempts.LockedUntil
	}

	return false, time.Time{}
}

// GetFailedAttempts 获取用户失败尝试次数.
func (t *LoginAttemptTracker) GetFailedAttempts(username string) int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if userAttempts, exists := t.attempts[username]; exists {
		return userAttempts.FailedCount
	}
	return 0
}

// GetRemainingAttempts 获取剩余尝试次数.
func (t *LoginAttemptTracker) GetRemainingAttempts(username string) int {
	failed := t.GetFailedAttempts(username)
	remaining := t.maxAttempts - failed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ResetAttempts 重置用户的登录尝试记录.
func (t *LoginAttemptTracker) ResetAttempts(username string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if userAttempts, exists := t.attempts[username]; exists {
		userAttempts.FailedCount = 0
		userAttempts.LockedUntil = nil
		userAttempts.Attempts = make([]time.Time, 0)
	}
}

// Unlock 手动解锁用户.
func (t *LoginAttemptTracker) Unlock(username string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if userAttempts, exists := t.attempts[username]; exists {
		userAttempts.LockedUntil = nil
		userAttempts.FailedCount = 0
	}
}

// UnlockIP 手动解锁 IP.
func (t *LoginAttemptTracker) UnlockIP(ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if ipAttempts, exists := t.ipAttempts[ip]; exists {
		ipAttempts.LockedUntil = nil
		ipAttempts.FailedCount = 0
	}
}

// Cleanup 清理过期记录.
func (t *LoginAttemptTracker) Cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	// 清理用户记录
	for username, userAttempts := range t.attempts {
		t.cleanupUserAttempts(userAttempts, now)
		if len(userAttempts.Attempts) == 0 && userAttempts.LockedUntil == nil {
			delete(t.attempts, username)
		}
	}

	// 清理 IP 记录
	for ip, ipAttempts := range t.ipAttempts {
		t.cleanupIPAttempts(ipAttempts, now)
		if len(ipAttempts.Attempts) == 0 && ipAttempts.LockedUntil == nil {
			delete(t.ipAttempts, ip)
		}
	}
}

// cleanupUserAttempts 清理用户过期的尝试记录.
func (t *LoginAttemptTracker) cleanupUserAttempts(userAttempts *UserAttempts, now time.Time) {
	windowStart := now.Add(-t.attemptWindow)
	validAttempts := make([]time.Time, 0)

	for _, attempt := range userAttempts.Attempts {
		if attempt.After(windowStart) {
			validAttempts = append(validAttempts, attempt)
		}
	}

	userAttempts.Attempts = validAttempts
	userAttempts.FailedCount = len(validAttempts)

	// 如果锁定已过期，清除锁定
	if userAttempts.LockedUntil != nil && now.After(*userAttempts.LockedUntil) {
		userAttempts.LockedUntil = nil
	}
}

// cleanupIPAttempts 清理 IP 过期的尝试记录.
func (t *LoginAttemptTracker) cleanupIPAttempts(ipAttempts *IPAttempts, now time.Time) {
	windowStart := now.Add(-t.attemptWindow)
	validAttempts := make([]time.Time, 0)

	for _, attempt := range ipAttempts.Attempts {
		if attempt.After(windowStart) {
			validAttempts = append(validAttempts, attempt)
		}
	}

	ipAttempts.Attempts = validAttempts
	ipAttempts.FailedCount = len(validAttempts)

	// 如果锁定已过期，清除锁定
	if ipAttempts.LockedUntil != nil && now.After(*ipAttempts.LockedUntil) {
		ipAttempts.LockedUntil = nil
	}
}

// GetStats 获取统计信息.
func (t *LoginAttemptTracker) GetStats() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	now := time.Now()
	lockedUsers := 0
	lockedIPs := 0

	for _, userAttempts := range t.attempts {
		if userAttempts.LockedUntil != nil && now.Before(*userAttempts.LockedUntil) {
			lockedUsers++
		}
	}

	for _, ipAttempts := range t.ipAttempts {
		if ipAttempts.LockedUntil != nil && now.Before(*ipAttempts.LockedUntil) {
			lockedIPs++
		}
	}

	return map[string]interface{}{
		"total_users_tracked": len(t.attempts),
		"total_ips_tracked":   len(t.ipAttempts),
		"locked_users":        lockedUsers,
		"locked_ips":          lockedIPs,
		"max_attempts":        t.maxAttempts,
		"lockout_duration":    t.lockoutDuration.String(),
	}
}

// GetLockoutInfo 获取用户锁定信息.
func (t *LoginAttemptTracker) GetLockoutInfo(username string) map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	userAttempts, exists := t.attempts[username]
	if !exists {
		return map[string]interface{}{
			"username":           username,
			"locked":             false,
			"failed_attempts":    0,
			"remaining_attempts": t.maxAttempts,
		}
	}

	info := map[string]interface{}{
		"username":           username,
		"failed_attempts":    userAttempts.FailedCount,
		"remaining_attempts": t.maxAttempts - userAttempts.FailedCount,
	}

	if userAttempts.LockedUntil != nil && time.Now().Before(*userAttempts.LockedUntil) {
		info["locked"] = true
		info["locked_until"] = userAttempts.LockedUntil.Format(time.RFC3339)
		info["remaining_time"] = time.Until(*userAttempts.LockedUntil).Round(time.Second).String()
	} else {
		info["locked"] = false
	}

	return info
}
