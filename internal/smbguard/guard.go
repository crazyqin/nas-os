// Package smbguard 提供 SMB 暴力破解自动拦截功能
// 对标群晖 DSM 7.2 Auto Block for SMB，超越其实现
// 特性：滑动窗口检测、渐进式封禁、白名单、攻击模式分析
package smbguard

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// BanLevel 封禁级别
type BanLevel int

const (
	BanLevelWarn   BanLevel = 1 // 警告（不封禁）
	BanLevelTemp   BanLevel = 2 // 临时封禁（30分钟）
	BanLevelMedium BanLevel = 3 // 中期封禁（24小时）
	BanLevelPerm   BanLevel = 4 // 永久封禁（需手动解除）
)

// String 返回封禁级别描述
func (l BanLevel) String() string {
	switch l {
	case BanLevelWarn:
		return "warn"
	case BanLevelTemp:
		return "temporary"
	case BanLevelMedium:
		return "medium"
	case BanLevelPerm:
		return "permanent"
	default:
		return "unknown"
	}
}

// AttackPattern 攻击模式
type AttackPattern string

const (
	PatternBruteForce  AttackPattern = "brute_force" // 暴力破解
	PatternDistributed AttackPattern = "distributed" // 分布式攻击
	PatternCredential  AttackPattern = "credential"  // 凭据填充
	PatternScan        AttackPattern = "port_scan"   // 端口扫描
)

// GuardConfig 配置
type GuardConfig struct {
	Enabled            bool          `json:"enabled"`
	MaxAttempts        int           `json:"max_attempts"`        // 最大失败尝试次数
	WindowDuration     time.Duration `json:"window_duration"`     // 检测窗口时长
	TempBanDuration    time.Duration `json:"temp_ban_duration"`   // 临时封禁时长
	MediumBanDuration  time.Duration `json:"medium_ban_duration"` // 中期封禁时长
	WhitelistCIDRs     []string      `json:"whitelist_cidrs"`     // 白名单 CIDR
	EnableAutoEscalate bool          `json:"auto_escalate"`       // 自动升级封禁级别
	LogAttempts        bool          `json:"log_attempts"`        // 记录所有尝试
}

// DefaultConfig 返回默认配置
func DefaultConfig() GuardConfig {
	return GuardConfig{
		Enabled:            true,
		MaxAttempts:        5,
		WindowDuration:     10 * time.Minute,
		TempBanDuration:    30 * time.Minute,
		MediumBanDuration:  24 * time.Hour,
		WhitelistCIDRs:     []string{"127.0.0.0/8", "::1/128"},
		EnableAutoEscalate: true,
		LogAttempts:        true,
	}
}

// FailedAttempt 失败尝试记录
type FailedAttempt struct {
	Timestamp time.Time `json:"timestamp"`
	Username  string    `json:"username"`
	ClientIP  string    `json:"client_ip"`
	Reason    string    `json:"reason"`
}

// BannedEntry 封禁条目
type BannedEntry struct {
	IP        string        `json:"ip"`
	Level     BanLevel      `json:"level"`
	Pattern   AttackPattern `json:"pattern"`
	BannedAt  time.Time     `json:"banned_at"`
	ExpiresAt *time.Time    `json:"expires_at,omitempty"`
	Attempts  int           `json:"attempts"`
	Reason    string        `json:"reason"`
	Released  bool          `json:"released"`
}

// AttackStats 攻击统计
type AttackStats struct {
	TotalAttempts    int            `json:"total_attempts"`
	BlockedAttempts  int            `json:"blocked_attempts"`
	BannedIPs        int            `json:"banned_ips"`
	AttacksByPattern map[string]int `json:"attacks_by_pattern"`
	TopAttackers     []IPCount      `json:"top_attackers"`
	LastAttackTime   *time.Time     `json:"last_attack_time,omitempty"`
}

// IPCount IP 计数
type IPCount struct {
	IP    string `json:"ip"`
	Count int    `json:"count"`
}

// Guard SMB 安全守卫
type Guard struct {
	mu        sync.RWMutex
	config    GuardConfig
	attempts  map[string][]FailedAttempt // IP -> 尝试列表
	banned    map[string]*BannedEntry    // IP -> 封禁条目
	whitelist map[string]*net.IPNet      // 白名单网络
	stats     AttackStats
	stopCh    chan struct{}
}

// NewGuard 创建新的 SMB 安全守卫
func NewGuard(config GuardConfig) *Guard {
	g := &Guard{
		config:    config,
		attempts:  make(map[string][]FailedAttempt),
		banned:    make(map[string]*BannedEntry),
		whitelist: make(map[string]*net.IPNet),
		stats: AttackStats{
			AttacksByPattern: make(map[string]int),
		},
		stopCh: make(chan struct{}),
	}

	// 解析白名单
	for _, cidr := range config.WhitelistCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			g.whitelist[cidr] = network
		}
	}

	return g
}

// Start 启动守卫
func (g *Guard) Start(ctx context.Context) error {
	if !g.config.Enabled {
		return nil
	}

	// 启动过期清理协程
	go g.cleanupLoop(ctx)

	return nil
}

// Stop 停止守卫
func (g *Guard) Stop() {
	close(g.stopCh)
}

// RecordFailedAttempt 记录失败尝试
func (g *Guard) RecordFailedAttempt(clientIP, username, reason string) *BannedEntry {
	g.mu.Lock()
	defer g.mu.Unlock()

	// 检查白名单
	if g.isWhitelisted(clientIP) {
		return nil
	}

	// 记录尝试
	attempt := FailedAttempt{
		Timestamp: time.Now(),
		Username:  username,
		ClientIP:  clientIP,
		Reason:    reason,
	}

	g.attempts[clientIP] = append(g.attempts[clientIP], attempt)
	g.stats.TotalAttempts++

	// 清理窗口外的记录
	g.cleanExpiredAttempts(clientIP)

	// 检查是否需要封禁
	attemptsInWindow := len(g.attempts[clientIP])
	if attemptsInWindow >= g.config.MaxAttempts {
		return g.banIP(clientIP, PatternBruteForce,
			fmt.Sprintf("在 %v 内失败 %d 次", g.config.WindowDuration, attemptsInWindow))
	}

	return nil
}

// IsBanned 检查 IP 是否被封禁
func (g *Guard) IsBanned(clientIP string) (bool, *BannedEntry) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// 检查白名单
	if g.isWhitelisted(clientIP) {
		return false, nil
	}

	entry, exists := g.banned[clientIP]
	if !exists {
		return false, nil
	}

	// 检查是否过期
	if entry.ExpiresAt != nil && time.Now().After(*entry.ExpiresAt) && !entry.Released {
		return false, entry
	}

	if entry.Released {
		return false, entry
	}

	return true, entry
}

// ReleaseBan 解除封禁
func (g *Guard) ReleaseBan(clientIP string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	entry, exists := g.banned[clientIP]
	if !exists {
		return fmt.Errorf("IP %s 未被封禁", clientIP)
	}

	entry.Released = true
	g.stats.BannedIPs--
	delete(g.attempts, clientIP)

	return nil
}

// GetStats 获取统计信息
func (g *Guard) GetStats() AttackStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	stats := g.stats
	stats.BannedIPs = len(g.banned)

	// 计算 top attackers
	counts := make(map[string]int)
	for ip, attempts := range g.attempts {
		counts[ip] = len(attempts)
	}

	for ip, entry := range g.banned {
		if !entry.Released {
			counts[ip] += entry.Attempts
		}
	}

	// 排序取 top 10
	type ipCount struct {
		ip    string
		count int
	}
	var sorted []ipCount
	for ip, count := range counts {
		sorted = append(sorted, ipCount{ip, count})
	}

	// 简单冒泡排序
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[i].count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	limit := 10
	if len(sorted) < limit {
		limit = len(sorted)
	}

	stats.TopAttackers = make([]IPCount, limit)
	for i := 0; i < limit; i++ {
		stats.TopAttackers[i] = IPCount{
			IP:    sorted[i].ip,
			Count: sorted[i].count,
		}
	}

	return stats
}

// GetBannedIPs 获取所有被封禁的 IP
func (g *Guard) GetBannedIPs() []BannedEntry {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var entries []BannedEntry
	for _, entry := range g.banned {
		if !entry.Released {
			entries = append(entries, *entry)
		}
	}
	return entries
}

// AddWhitelist 添加白名单
func (g *Guard) AddWhitelist(cidr string) error {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("无效的 CIDR: %w", err)
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.whitelist[cidr] = network
	g.config.WhitelistCIDRs = append(g.config.WhitelistCIDRs, cidr)

	// 解除白名单中 IP 的封禁
	for ip, entry := range g.banned {
		if !entry.Released && network.Contains(net.ParseIP(ip)) {
			entry.Released = true
		}
	}

	return nil
}

// RemoveWhitelist 移除白名单
func (g *Guard) RemoveWhitelist(cidr string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.whitelist[cidr]; !exists {
		return fmt.Errorf("CIDR %s 不在白名单中", cidr)
	}

	delete(g.whitelist, cidr)

	// 更新配置
	newList := make([]string, 0, len(g.config.WhitelistCIDRs)-1)
	for _, c := range g.config.WhitelistCIDRs {
		if c != cidr {
			newList = append(newList, c)
		}
	}
	g.config.WhitelistCIDRs = newList

	return nil
}

// UpdateConfig 更新配置
func (g *Guard) UpdateConfig(config GuardConfig) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.config = config

	// 重新加载白名单
	g.whitelist = make(map[string]*net.IPNet)
	for _, cidr := range config.WhitelistCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			g.whitelist[cidr] = network
		}
	}
}

// banIP 封禁 IP
func (g *Guard) banIP(clientIP string, pattern AttackPattern, reason string) *BannedEntry {
	now := time.Now()
	level := g.determineBanLevel(clientIP)

	var expiresAt *time.Time
	switch level {
	case BanLevelTemp:
		t := now.Add(g.config.TempBanDuration)
		expiresAt = &t
	case BanLevelMedium:
		t := now.Add(g.config.MediumBanDuration)
		expiresAt = &t
	case BanLevelPerm:
		expiresAt = nil
	}

	entry := &BannedEntry{
		IP:        clientIP,
		Level:     level,
		Pattern:   pattern,
		BannedAt:  now,
		ExpiresAt: expiresAt,
		Attempts:  len(g.attempts[clientIP]),
		Reason:    reason,
	}

	g.banned[clientIP] = entry
	g.stats.BlockedAttempts++
	g.stats.AttacksByPattern[string(pattern)]++

	// 清除尝试记录
	delete(g.attempts, clientIP)

	// 更新最后攻击时间
	g.stats.LastAttackTime = &now

	return entry
}

// determineBanLevel 确定封禁级别
func (g *Guard) determineBanLevel(clientIP string) BanLevel {
	entry, exists := g.banned[clientIP]
	if !exists {
		return BanLevelTemp
	}

	if !g.config.EnableAutoEscalate {
		return entry.Level
	}

	// 渐进式升级
	switch entry.Level {
	case BanLevelWarn:
		return BanLevelTemp
	case BanLevelTemp:
		return BanLevelMedium
	case BanLevelMedium:
		return BanLevelPerm
	default:
		return BanLevelPerm
	}
}

// isWhitelisted 检查 IP 是否在白名单中
func (g *Guard) isWhitelisted(clientIP string) bool {
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	for _, network := range g.whitelist {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// cleanExpiredAttempts 清理窗口外的尝试记录
func (g *Guard) cleanExpiredAttempts(clientIP string) {
	cutoff := time.Now().Add(-g.config.WindowDuration)
	attempts := g.attempts[clientIP]

	valid := make([]FailedAttempt, 0, len(attempts))
	for _, a := range attempts {
		if a.Timestamp.After(cutoff) {
			valid = append(valid, a)
		}
	}

	g.attempts[clientIP] = valid
}

// cleanupLoop 定期清理过期封禁
func (g *Guard) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-g.stopCh:
			return
		case <-ticker.C:
			g.cleanupExpired()
		}
	}
}

// cleanupExpired 清理过期封禁
func (g *Guard) cleanupExpired() {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	for ip, entry := range g.banned {
		if entry.Released {
			delete(g.banned, ip)
			continue
		}

		if entry.ExpiresAt != nil && now.After(*entry.ExpiresAt) {
			entry.Released = true
			g.stats.BannedIPs--
			delete(g.banned, ip)
		}
	}
}
