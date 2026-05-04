package vpnserver

import (
	"fmt"
	"sync"
	"time"
)

// Fail2Ban 实现 VPN 登录失败的自动封禁机制。
// 记录登录失败次数，达到阈值后自动封禁指定时长。

// 封禁规则默认值
const (
	// DefaultMaxAttempts 默认最大失败尝试次数
	DefaultMaxAttempts = 5
	// DefaultWindowSeconds 默认统计窗口（秒）
	DefaultWindowSeconds = 300 // 5分钟
	// DefaultBanDurationSeconds 默认封禁时长（秒）
	DefaultBanDurationSeconds = 1800 // 30分钟
	// DefaultCleanupIntervalSeconds 默认清理间隔（秒）
	DefaultCleanupIntervalSeconds = 60
	// MaxEventLogSize 最大事件日志条数
	MaxEventLogSize = 1000
)

// BanEntry 封禁记录
type BanEntry struct {
	IP        string    `json:"ip"`
	Username  string    `json:"username,omitempty"`
	BanCount  int       `json:"ban_count"` // 累计封禁次数
	Reason    string    `json:"reason"`
	BannedAt  time.Time `json:"banned_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Active    bool      `json:"active"`
}

// FailAttempt 登录失败记录
type FailAttempt struct {
	IP        string    `json:"ip"`
	Username  string    `json:"username"`
	Timestamp time.Time `json:"timestamp"`
}

// Fail2BanEvent 封禁事件日志
type Fail2BanEvent struct {
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"` // "fail_attempt", "banned", "unbanned", "expired", "whitelist_add", "whitelist_remove"
	IP        string    `json:"ip"`
	Username  string    `json:"username,omitempty"`
	Details   string    `json:"details,omitempty"`
}

// Fail2BanConfig 封禁规则配置
type Fail2BanConfig struct {
	Enabled               bool `json:"enabled"`
	MaxAttempts           int  `json:"max_attempts"`
	WindowSeconds         int  `json:"window_seconds"`
	BanDurationSeconds    int  `json:"ban_duration_seconds"`
	CleanupIntervalSeconds int `json:"cleanup_interval_seconds"`
}

// Fail2BanStatus 封禁状态（用于API响应）
type Fail2BanStatus struct {
	Config       Fail2BanConfig  `json:"config"`
	BannedIPs    []BanEntry      `json:"banned_ips"`
	WhiteList    []string        `json:"whitelist"`
	TotalBanned  int             `json:"total_banned"`
	TotalEvents  int             `json:"total_events"`
	RecentEvents []Fail2BanEvent `json:"recent_events"`
}

// UnblockRequest 手动解封请求
type UnblockRequest struct {
	IP string `json:"ip"`
}

// WhiteListRequest 白名单管理请求
type WhiteListRequest struct {
	IP     string `json:"ip"`
	Action string `json:"action"` // "add" 或 "remove"
}

// Fail2Ban 引擎
type Fail2Ban struct {
	mu sync.RWMutex

	config Fail2BanConfig

	// 失败记录（按IP分组）
	attempts map[string][]FailAttempt

	// 封禁列表（按IP索引）
	bans map[string]*BanEntry

	// 白名单
	whitelist map[string]bool

	// 事件日志
	events []Fail2BanEvent

	// 关闭信号
	stopCh chan struct{}
}

// NewFail2Ban 创建 Fail2Ban 引擎
func NewFail2Ban() *Fail2Ban {
	f := &Fail2Ban{
		config: Fail2BanConfig{
			Enabled:               true,
			MaxAttempts:           DefaultMaxAttempts,
			WindowSeconds:         DefaultWindowSeconds,
			BanDurationSeconds:    DefaultBanDurationSeconds,
			CleanupIntervalSeconds: DefaultCleanupIntervalSeconds,
		},
		attempts: make(map[string][]FailAttempt),
		bans:     make(map[string]*BanEntry),
		whitelist: make(map[string]bool),
		events:   make([]Fail2BanEvent, 0, 100),
		stopCh:   make(chan struct{}),
	}

	// 启动后台清理协程
	go f.cleanupLoop()
	return f
}

// Stop 停止 Fail2Ban 引擎
func (f *Fail2Ban) Stop() {
	select {
	case <-f.stopCh:
	default:
		close(f.stopCh)
	}
}

// RecordFailAttempt 记录登录失败事件。
// 如果IP在白名单中，只记录日志不封禁。
// 如果达到封禁阈值，自动封禁。
func (f *Fail2Ban) RecordFailAttempt(ip, username string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.config.Enabled {
		return
	}

	now := time.Now()

	// 记录失败事件
	f.addEvent(Fail2BanEvent{
		Timestamp: now,
		EventType: "fail_attempt",
		IP:        ip,
		Username:  username,
		Details:   fmt.Sprintf("登录失败: 用户=%s", username),
	})

	// 检查白名单
	if f.whitelist[ip] {
		return
	}

	// 检查是否已被封禁
	if ban, exists := f.bans[ip]; exists && ban.Active && now.Before(ban.ExpiresAt) {
		return // 已被封禁，无需重复处理
	}

	// 记录失败尝试
	f.attempts[ip] = append(f.attempts[ip], FailAttempt{
		IP:        ip,
		Username:  username,
		Timestamp: now,
	})

	// 清理窗口外的记录
	windowStart := now.Add(-time.Duration(f.config.WindowSeconds) * time.Second)
	valid := f.attempts[ip][:0]
	for _, a := range f.attempts[ip] {
		if a.Timestamp.After(windowStart) {
			valid = append(valid, a)
		}
	}
	f.attempts[ip] = valid

	// 检查是否达到封禁阈值
	if len(valid) >= f.config.MaxAttempts {
		f.banIP(ip, username, now)
	}
}

// banIP 执行封禁
func (f *Fail2Ban) banIP(ip, username string, now time.Time) {
	expiresAt := now.Add(time.Duration(f.config.BanDurationSeconds) * time.Second)

	banCount := 1
	if old, exists := f.bans[ip]; exists {
		banCount = old.BanCount + 1
	}

	f.bans[ip] = &BanEntry{
		IP:        ip,
		Username:  username,
		BanCount:  banCount,
		Reason:    fmt.Sprintf("5分钟内失败%d次", f.config.MaxAttempts),
		BannedAt:  now,
		ExpiresAt: expiresAt,
		Active:    true,
	}

	// 清除失败记录
	delete(f.attempts, ip)

	f.addEvent(Fail2BanEvent{
		Timestamp: now,
		EventType: "banned",
		IP:        ip,
		Username:  username,
		Details:   fmt.Sprintf("封禁至 %s（第%d次封禁）", expiresAt.Format(time.RFC3339), banCount),
	})
}

// IsBanned 检查IP是否被封禁
func (f *Fail2Ban) IsBanned(ip string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	ban, exists := f.bans[ip]
	if !exists || !ban.Active {
		return false
	}

	if time.Now().After(ban.ExpiresAt) {
		// 过期了，标记为非活跃
		ban.Active = false
		return false
	}

	return true
}

// Unblock 手动解封指定IP
func (f *Fail2Ban) Unblock(ip string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	ban, exists := f.bans[ip]
	if !exists {
		return fmt.Errorf("IP %s 不在封禁列表中", ip)
	}

	ban.Active = false

	// 清除失败记录
	delete(f.attempts, ip)

	f.addEvent(Fail2BanEvent{
		Timestamp: time.Now(),
		EventType: "unbanned",
		IP:        ip,
		Details:   "手动解封",
	})

	return nil
}

// AddToWhiteList 将IP加入白名单（加入白名单同时解除封禁）
func (f *Fail2Ban) AddToWhiteList(ip string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.whitelist[ip] = true

	// 如果该IP已被封禁，解除封禁
	if ban, exists := f.bans[ip]; exists && ban.Active {
		ban.Active = false
	}

	// 清除失败记录
	delete(f.attempts, ip)

	f.addEvent(Fail2BanEvent{
		Timestamp: time.Now(),
		EventType: "whitelist_add",
		IP:        ip,
		Details:   "加入白名单",
	})
}

// RemoveFromWhiteList 将IP从白名单移除
func (f *Fail2Ban) RemoveFromWhiteList(ip string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.whitelist[ip] {
		return fmt.Errorf("IP %s 不在白名单中", ip)
	}

	delete(f.whitelist, ip)

	f.addEvent(Fail2BanEvent{
		Timestamp: time.Now(),
		EventType: "whitelist_remove",
		IP:        ip,
		Details:   "移出白名单",
	})

	return nil
}

// IsWhiteListed 检查IP是否在白名单中
func (f *Fail2Ban) IsWhiteListed(ip string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.whitelist[ip]
}

// GetStatus 获取 Fail2Ban 状态
func (f *Fail2Ban) GetStatus() Fail2BanStatus {
	f.mu.RLock()
	defer f.mu.RUnlock()

	now := time.Now()

	// 收集活跃封禁
	bannedIPs := make([]BanEntry, 0)
	for _, ban := range f.bans {
		if ban.Active && now.Before(ban.ExpiresAt) {
			bannedIPs = append(bannedIPs, *ban)
		}
	}

	// 收集白名单
	whitelist := make([]string, 0, len(f.whitelist))
	for ip := range f.whitelist {
		whitelist = append(whitelist, ip)
	}

	// 最近事件（最多50条）
	recentCount := 50
	if len(f.events) < recentCount {
		recentCount = len(f.events)
	}
	recentEvents := make([]Fail2BanEvent, recentCount)
	copy(recentEvents, f.events[len(f.events)-recentCount:])

	return Fail2BanStatus{
		Config:       f.config,
		BannedIPs:    bannedIPs,
		WhiteList:    whitelist,
		TotalBanned:  len(bannedIPs),
		TotalEvents:  len(f.events),
		RecentEvents: recentEvents,
	}
}

// GetBanEntry 获取指定IP的封禁信息
func (f *Fail2Ban) GetBanEntry(ip string) (*BanEntry, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	ban, exists := f.bans[ip]
	if !exists {
		return nil, false
	}

	// 返回副本
	cp := *ban
	return &cp, true
}

// GetBannedIPs 获取所有被封禁的IP列表
func (f *Fail2Ban) GetBannedIPs() []BanEntry {
	f.mu.RLock()
	defer f.mu.RUnlock()

	now := time.Now()
	result := make([]BanEntry, 0)
	for _, ban := range f.bans {
		if ban.Active && now.Before(ban.ExpiresAt) {
			result = append(result, *ban)
		}
	}
	return result
}

// GetFailAttempts 获取指定IP的失败尝试记录
func (f *Fail2Ban) GetFailAttempts(ip string) []FailAttempt {
	f.mu.RLock()
	defer f.mu.RUnlock()

	attempts, exists := f.attempts[ip]
	if !exists {
		return nil
	}

	result := make([]FailAttempt, len(attempts))
	copy(result, attempts)
	return result
}

// UpdateConfig 更新配置
func (f *Fail2Ban) UpdateConfig(cfg Fail2BanConfig) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.config = cfg
}

// GetConfig 获取当前配置
func (f *Fail2Ban) GetConfig() Fail2BanConfig {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.config
}

// addEvent 添加事件日志（调用者需持有锁）
func (f *Fail2Ban) addEvent(event Fail2BanEvent) {
	f.events = append(f.events, event)
	// 限制日志大小
	if len(f.events) > MaxEventLogSize {
		f.events = f.events[len(f.events)-MaxEventLogSize:]
	}
}

// cleanupLoop 后台清理过期封禁
func (f *Fail2Ban) cleanupLoop() {
	ticker := time.NewTicker(time.Duration(f.config.CleanupIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-f.stopCh:
			return
		case <-ticker.C:
			f.cleanup()
		}
	}
}

// cleanup 清理过期的封禁和失败记录
func (f *Fail2Ban) cleanup() {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()

	// 清理过期封禁
	for ip, ban := range f.bans {
		if ban.Active && now.After(ban.ExpiresAt) {
			ban.Active = false
			f.addEvent(Fail2BanEvent{
				Timestamp: now,
				EventType: "expired",
				IP:        ip,
				Details:   "封禁已过期，自动解封",
			})
		}
	}

	// 清理过期失败记录
	windowStart := now.Add(-time.Duration(f.config.WindowSeconds) * time.Second)
	for ip, attempts := range f.attempts {
		valid := attempts[:0]
		for _, a := range attempts {
			if a.Timestamp.After(windowStart) {
				valid = append(valid, a)
			}
		}
		if len(valid) == 0 {
			delete(f.attempts, ip)
		} else {
			f.attempts[ip] = valid
		}
	}
}
