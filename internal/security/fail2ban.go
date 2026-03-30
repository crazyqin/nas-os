// Package security 提供失败登录保护功能 (刑部)
package security

import (
	"sync"
	"time"
)

// Fail2BanManager 失败登录保护管理器.
type Fail2BanManager struct {
	config     Fail2BanConfig
	attempts   map[string][]FailedLoginAttempt // IP -> attempts
	bannedIPs  map[string]*BannedIP            // IP -> ban info
	lockouts   map[string]*AccountLockout      // username -> lockout
	mu         sync.RWMutex
	notifyFunc func(alert Alert)
}

// NewFail2BanManager 创建失败登录保护管理器.
func NewFail2BanManager() *Fail2BanManager {
	return &Fail2BanManager{
		config: Fail2BanConfig{
			Enabled:            true,
			MaxAttempts:        5,
			WindowMinutes:      10,
			BanDurationMinutes: 30,
			AutoUnban:          true,
			NotifyOnBan:        true,
			ProtectedServices:  []string{"ssh", "web", "ftp"},
		},
		attempts:  make(map[string][]FailedLoginAttempt),
		bannedIPs: make(map[string]*BannedIP),
		lockouts:  make(map[string]*AccountLockout),
	}
}

// RecordFailedAttempt 记录失败登录尝试.
func (m *Fail2BanManager) RecordFailedAttempt(ip, username, userAgent, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	attempt := FailedLoginAttempt{
		IP:        ip,
		Username:  username,
		Timestamp: time.Now(),
		UserAgent: userAgent,
		Reason:    reason,
	}

	m.attempts[ip] = append(m.attempts[ip], attempt)

	// 检查是否需要封禁
	if len(m.attempts[ip]) >= m.config.MaxAttempts {
		m.banIP(ip)
	}
}

// banIP 封禁IP.
func (m *Fail2BanManager) banIP(ip string) {
	now := time.Now()
	expiresAt := now.Add(time.Duration(m.config.BanDurationMinutes) * time.Minute)

	m.bannedIPs[ip] = &BannedIP{
		IP:        ip,
		Reason:    "exceeded_max_attempts",
		BannedAt:  now,
		ExpiresAt: expiresAt,
		Attempts:  len(m.attempts[ip]),
	}

	// 清除尝试记录
	delete(m.attempts, ip)

	// 发送通知
	if m.notifyFunc != nil && m.config.NotifyOnBan {
		m.notifyFunc(Alert{
			ID:          generateFail2BanAlertID(),
			Timestamp:   now,
			Severity:    "high",
			Type:        "ip_banned",
			Title:       "IP被封禁",
			Description: "IP " + ip + " 因多次失败登录被封禁",
			SourceIP:    ip,
		})
	}
}

// IsBanned 检查IP是否被封禁.
func (m *Fail2BanManager) IsBanned(ip string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ban, exists := m.bannedIPs[ip]
	if !exists {
		return false
	}

	// 检查是否过期
	if m.config.AutoUnban && time.Now().After(ban.ExpiresAt) {
		return false
	}

	return true
}

// UnbanIP 解封IP.
func (m *Fail2BanManager) UnbanIP(ip string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.bannedIPs[ip]; exists {
		delete(m.bannedIPs, ip)
		return true
	}
	return false
}

// RecordFailedLogin 记录失败登录.
func (m *Fail2BanManager) RecordFailedLogin(ip, username, userAgent, reason string) {
	m.RecordFailedAttempt(ip, username, userAgent, reason)
}

// RecordSuccessfulLogin 记录成功登录.
func (m *Fail2BanManager) RecordSuccessfulLogin(ip, username string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// 清除该IP的失败尝试记录
	delete(m.attempts, ip)
}

// GetBannedIPs 获取所有被封禁的IP.
func (m *Fail2BanManager) GetBannedIPs() []*BannedIP {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*BannedIP, 0)
	for _, ban := range m.bannedIPs {
		result = append(result, ban)
	}
	return result
}

// SetNotifyFunc 设置通知函数.
func (m *Fail2BanManager) SetNotifyFunc(fn func(alert Alert)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyFunc = fn
}

// generateFail2BanAlertID 生成告警ID.
func generateFail2BanAlertID() string {
	return "alert_" + time.Now().Format("20060102150405")
}

// GetStatus 获取Fail2Ban状态.
func (m *Fail2BanManager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"enabled":        m.config.Enabled,
		"max_attempts":   m.config.MaxAttempts,
		"banned_count":   len(m.bannedIPs),
		"total_attempts": len(m.attempts),
	}
}

// UpdateConfig 更新配置.
func (m *Fail2BanManager) UpdateConfig(config Fail2BanConfig) Fail2BanConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	old := m.config
	m.config = config
	return old
}

// GetFailedAttempts 获取失败尝试记录.
func (m *Fail2BanManager) GetFailedAttempts(ip string) []FailedLoginAttempt {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.attempts[ip]
}

// StartCleanupRoutine 启动清理routine.
func (m *Fail2BanManager) StartCleanupRoutine() {
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			m.cleanupExpired()
		}
	}()
}

// cleanupExpired 清理过期记录.
func (m *Fail2BanManager) cleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	window := time.Duration(m.config.WindowMinutes) * time.Minute

	// 清理过期尝试记录
	for ip, attempts := range m.attempts {
		var valid []FailedLoginAttempt
		for _, a := range attempts {
			if now.Sub(a.Timestamp) <= window {
				valid = append(valid, a)
			}
		}
		if len(valid) == 0 {
			delete(m.attempts, ip)
		} else {
			m.attempts[ip] = valid
		}
	}

	// 清理过期封禁
	for ip, ban := range m.bannedIPs {
		if m.config.AutoUnban && now.After(ban.ExpiresAt) {
			delete(m.bannedIPs, ip)
		}
	}
}
