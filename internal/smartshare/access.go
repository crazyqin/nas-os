// Package smartshare 提供访问控制功能
package smartshare

import (
	"crypto/subtle"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// AccessController 访问控制器.
type AccessController struct {
	mu              sync.RWMutex
	logger          *zap.Logger
	rateLimiter     map[string]*RateLimitEntry // IP -> RateLimit
	maxAttempts     int
	lockoutDuration time.Duration
}

// RateLimitEntry 速率限制条目.
type RateLimitEntry struct {
	Attempts    int        `json:"attempts"`
	LastTry     time.Time  `json:"last_try"`
	LockedUntil *time.Time `json:"locked_until,omitempty"`
}

// NewAccessController 创建访问控制器.
func NewAccessController(logger *zap.Logger) *AccessController {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &AccessController{
		logger:          logger,
		rateLimiter:     make(map[string]*RateLimitEntry),
		maxAttempts:     10,
		lockoutDuration: 15 * time.Minute,
	}
}

// AccessRequest 访问请求.
type AccessRequest struct {
	ShareID   string `json:"share_id"`
	Token     string `json:"token"`
	Password  string `json:"password,omitempty"`
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`
	Action    string `json:"action"` // view, download, preview
}

// AccessResult 访问结果.
type AccessResult struct {
	Allowed    bool          `json:"allowed"`
	Reason     string        `json:"reason,omitempty"`
	Link       *ShareLink    `json:"link,omitempty"`
	Log        *AccessLog    `json:"-"`
	RetryAfter time.Duration `json:"retry_after,omitempty"`
}

// CheckAccess 检查访问权限.
func (ac *AccessController) CheckAccess(req *AccessRequest, link *ShareLink) *AccessResult {
	result := &AccessResult{
		Link: link,
	}

	// 检查链接状态
	if link.Status != ShareStatusActive {
		result.Allowed = false
		result.Reason = fmt.Sprintf("share link is %s", link.Status)
		return result
	}

	// 检查过期时间
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		result.Allowed = false
		result.Reason = "share link has expired"
		return result
	}

	// 检查下载次数
	if req.Action == "download" && link.MaxDownloads > 0 && link.DownloadCount >= link.MaxDownloads {
		result.Allowed = false
		result.Reason = "download limit reached"
		return result
	}

	// 检查 IP 白名单
	if !ac.checkIPWhitelist(req.IPAddress, link.IPWhitelist) {
		result.Allowed = false
		result.Reason = "IP address not in whitelist"
		ac.logger.Warn("IP not in whitelist",
			zap.String("ip", req.IPAddress),
			zap.String("share_id", link.ID))
		return result
	}

	// 检查速率限制
	if retryAfter := ac.checkRateLimit(req.IPAddress); retryAfter > 0 {
		result.Allowed = false
		result.Reason = "rate limit exceeded"
		result.RetryAfter = retryAfter
		return result
	}

	// 检查密码
	if link.Mode == ShareModePassword {
		if req.Password == "" {
			result.Allowed = false
			result.Reason = "password required"
			return result
		}
		if !ac.verifyPassword(req.Password, link.Password) {
			ac.recordFailedAttempt(req.IPAddress)
			result.Allowed = false
			result.Reason = "invalid password"
			return result
		}
	}

	// 检查用户权限
	if link.Mode == ShareModePrivate && len(link.AllowedUsers) > 0 {
		// 这里应该从请求中获取用户ID，简化处理
		// 实际实现需要集成认证系统
	}

	result.Allowed = true
	return result
}

// checkIPWhitelist 检查 IP 白名单.
func (ac *AccessController) checkIPWhitelist(ipAddress string, whitelist []string) bool {
	if len(whitelist) == 0 {
		return true // 没有白名单限制
	}

	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return false
	}

	for _, allowedIP := range whitelist {
		// 支持 CIDR 格式
		if strings.Contains(allowedIP, "/") {
			_, ipNet, err := net.ParseCIDR(allowedIP)
			if err != nil {
				continue
			}
			if ipNet.Contains(ip) {
				return true
			}
		} else {
			// 单个 IP
			if allowedIP == ipAddress {
				return true
			}
		}
	}

	return false
}

// checkRateLimit 检查速率限制.
func (ac *AccessController) checkRateLimit(ipAddress string) time.Duration {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	entry, ok := ac.rateLimiter[ipAddress]
	if !ok {
		return 0
	}

	// 检查是否被锁定
	if entry.LockedUntil != nil && time.Now().Before(*entry.LockedUntil) {
		return time.Until(*entry.LockedUntil)
	}

	// 重置计数器（每小时重置）
	if time.Since(entry.LastTry) > time.Hour {
		entry.Attempts = 0
		entry.LockedUntil = nil
		return 0
	}

	return 0
}

// recordFailedAttempt 记录失败尝试.
func (ac *AccessController) recordFailedAttempt(ipAddress string) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	entry, ok := ac.rateLimiter[ipAddress]
	if !ok {
		entry = &RateLimitEntry{}
		ac.rateLimiter[ipAddress] = entry
	}

	entry.Attempts++
	entry.LastTry = time.Now()

	// 超过最大尝试次数，锁定
	if entry.Attempts >= ac.maxAttempts {
		lockUntil := time.Now().Add(ac.lockoutDuration)
		entry.LockedUntil = &lockUntil
		ac.logger.Warn("IP locked due to too many failed attempts",
			zap.String("ip", ipAddress),
			zap.Int("attempts", entry.Attempts),
			zap.Time("locked_until", lockUntil))
	}
}

// verifyPassword 验证密码（常量时间比较，防止时序攻击）.
func (ac *AccessController) verifyPassword(input, expected string) bool {
	return subtle.ConstantTimeCompare([]byte(input), []byte(expected)) == 1
}

// CleanupRateLimits 清理过期的速率限制条目.
func (ac *AccessController) CleanupRateLimits() {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	now := time.Now()
	for ip, entry := range ac.rateLimiter {
		if entry.LockedUntil != nil && now.After(*entry.LockedUntil) {
			delete(ac.rateLimiter, ip)
		} else if time.Since(entry.LastTry) > 24*time.Hour {
			delete(ac.rateLimiter, ip)
		}
	}
}

// SetMaxAttempts 设置最大尝试次数.
func (ac *AccessController) SetMaxAttempts(attempts int) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.maxAttempts = attempts
}

// SetLockoutDuration 设置锁定时长.
func (ac *AccessController) SetLockoutDuration(duration time.Duration) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.lockoutDuration = duration
}
