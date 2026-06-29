// Package filerequest 提供文件收集请求功能的安全增强模块。
// 实现密码保护、IP限速、文件类型过滤、防滥用等安全功能，参考群晖 DSM 7.3。
package filerequest

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// SecurityManager 安全管理器，负责文件请求的安全控制。
type SecurityManager struct {
	mu sync.RWMutex

	// IP限速配置
	ipRateLimit    int           // 每IP每分钟最大请求次数
	ipRateWindow   time.Duration // 限速窗口
	ipAccessCount  map[string][]time.Time

	// 密码尝试限制
	maxPasswordAttempts int                                   // 最大密码尝试次数
	passwordAttempts    map[string]int                         // linkID → 尝试次数
	passwordBlocked     map[string]time.Time                   // linkID → 封锁截止时间
	passwordBlockTime   time.Duration                          // 封锁时长

	// 文件类型控制
	blockedExtensions map[string]bool // 禁止的扩展名
}

// NewSecurityManager 创建安全管理器。
func NewSecurityManager() *SecurityManager {
	return &SecurityManager{
		ipRateLimit:         60,
		ipRateWindow:        time.Minute,
		ipAccessCount:       make(map[string][]time.Time),
		maxPasswordAttempts: 5,
		passwordAttempts:    make(map[string]int),
		passwordBlocked:     make(map[string]time.Time),
		passwordBlockTime:   15 * time.Minute,
		blockedExtensions: map[string]bool{
			".exe": true, ".bat": true, ".cmd": true, ".com": true,
			".sh": true, ".ps1": true, ".vbs": true, ".scr": true,
			".js": true, ".jar": true, ".msi": true,
		},
	}
}

// CheckIPRateLimit 检查IP是否超过限速。
func (sm *SecurityManager) CheckIPRateLimit(ip string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-sm.ipRateWindow)

	// 清理过期记录
	accesses := sm.ipAccessCount[ip]
	valid := accesses[:0]
	for _, t := range accesses {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= sm.ipRateLimit {
		sm.ipAccessCount[ip] = valid
		return fmt.Errorf("IP %s 超过限速 (%d次/%v)", ip, sm.ipRateLimit, sm.ipRateWindow)
	}

	sm.ipAccessCount[ip] = append(valid, now)
	return nil
}

// VerifyPassword 验证链接密码。
func (sm *SecurityManager) VerifyPassword(linkID, password, correctPassword string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 检查是否被封锁
	if blockedUntil, ok := sm.passwordBlocked[linkID]; ok {
		if time.Now().Before(blockedUntil) {
			return fmt.Errorf("密码尝试次数过多，请 %v 后再试", blockedUntil.Sub(time.Now()).Round(time.Second))
		}
		delete(sm.passwordBlocked, linkID)
		sm.passwordAttempts[linkID] = 0
	}

	if correctPassword == "" {
		return nil // 无密码保护
	}

	if password == correctPassword {
		sm.passwordAttempts[linkID] = 0
		return nil
	}

	sm.passwordAttempts[linkID]++
	if sm.passwordAttempts[linkID] >= sm.maxPasswordAttempts {
		sm.passwordBlocked[linkID] = time.Now().Add(sm.passwordBlockTime)
		return fmt.Errorf("密码错误次数过多，已封锁 %v", sm.passwordBlockTime)
	}

	remaining := sm.maxPasswordAttempts - sm.passwordAttempts[linkID]
	return fmt.Errorf("密码错误，剩余尝试次数 %d", remaining)
}

// ValidateFileSize 验证文件大小是否在允许范围内。
func (sm *SecurityManager) ValidateFileSize(size int64, maxSize int64) error {
	if maxSize > 0 && size > maxSize {
		return fmt.Errorf("文件大小 %d 超过限制 %d", size, maxSize)
	}
	return nil
}

// ValidateFileType 验证文件类型是否被允许。
func (sm *SecurityManager) ValidateFileType(filename string, allowedExts []string) error {
	ext := strings.ToLower(getExtension(filename))

	// 检查黑名单
	if sm.blockedExtensions[ext] {
		return fmt.Errorf("文件类型 %s 被禁止", ext)
	}

	// 检查白名单
	if len(allowedExts) > 0 {
		found := false
		for _, allowed := range allowedExts {
			if ext == strings.ToLower(allowed) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("文件类型 %s 不在允许列表中", ext)
		}
	}

	return nil
}

// ValidateUploadCount 验证上传文件数量是否超过限制。
func (sm *SecurityManager) ValidateUploadCount(currentCount, maxCount int) error {
	if maxCount > 0 && currentCount >= maxCount {
		return fmt.Errorf("已达到最大文件数量限制 %d", maxCount)
	}
	return nil
}

// IsIPBlocked 检查IP是否被封锁。
func (sm *SecurityManager) IsIPBlocked(ip string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	accesses, ok := sm.ipAccessCount[ip]
	if !ok {
		return false
	}
	cutoff := time.Now().Add(-sm.ipRateWindow)
	valid := 0
	for _, t := range accesses {
		if t.After(cutoff) {
			valid++
		}
	}
	return valid >= sm.ipRateLimit*2 // 超过2倍限速则封锁
}

// ClearExpiredBlocks 清理过期的密码封锁记录。
func (sm *SecurityManager) ClearExpiredBlocks() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for linkID, until := range sm.passwordBlocked {
		if now.After(until) {
			delete(sm.passwordBlocked, linkID)
			delete(sm.passwordAttempts, linkID)
		}
	}
}

// SetIPRateLimit 设置IP限速参数。
func (sm *SecurityManager) SetIPRateLimit(limit int, window time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.ipRateLimit = limit
	sm.ipRateWindow = window
}

// SetMaxPasswordAttempts 设置最大密码尝试次数。
func (sm *SecurityManager) SetMaxPasswordAttempts(max int, blockTime time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.maxPasswordAttempts = max
	sm.passwordBlockTime = blockTime
}

// AddBlockedExtension 添加禁止的文件扩展名。
func (sm *SecurityManager) AddBlockedExtension(ext string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.blockedExtensions[strings.ToLower(ext)] = true
}

// RemoveBlockedExtension 移除禁止的文件扩展名。
func (sm *SecurityManager) RemoveBlockedExtension(ext string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.blockedExtensions, strings.ToLower(ext))
}

// IsLinkExpired 检查链接是否过期。
func (sm *SecurityManager) IsLinkExpired(link *RequestLink) bool {
	if link.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*link.ExpiresAt)
}

// IsLinkAccessLimitReached 检查链接访问次数是否达到上限。
func (sm *SecurityManager) IsLinkAccessLimitReached(link *RequestLink) bool {
	if link.MaxAccessCount <= 0 {
		return false
	}
	return link.AccessCount >= link.MaxAccessCount
}

// CheckLinkAccess 检查链接访问权限。
func (sm *SecurityManager) CheckLinkAccess(link *RequestLink, ip string) error {
	if !link.IsActive {
		return fmt.Errorf("链接已被禁用")
	}
	if sm.IsLinkExpired(link) {
		return fmt.Errorf("链接已过期")
	}
	if sm.IsLinkAccessLimitReached(link) {
		return fmt.Errorf("链接访问次数已达上限")
	}
	if err := sm.CheckIPRateLimit(ip); err != nil {
		return err
	}
	return nil
}

// ExtractIP 从请求中提取客户端IP。
func ExtractIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

// getExtension 获取文件扩展名。
func getExtension(filename string) string {
	idx := strings.LastIndex(filename, ".")
	if idx < 0 {
		return ""
	}
	return filename[idx:]
}
