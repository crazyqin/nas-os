// Package smbguard 提供 SMB 暴力破解防护引擎，
// 支持 SMB 连接监控、自动封锁策略、IP 白/黑名单和告警通知。
// 对标群晖自动封锁功能，专为 SMB 协议暴力破解防护设计。
package smbguard

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// BlockAction 封锁动作类型.
type BlockAction string

const (
	BlockActionTemp BlockAction = "temp" // 临时封锁
	BlockActionPerm BlockAction = "perm" // 永久封锁
	BlockActionNone BlockAction = "none" // 仅告警
)

// AlertSeverity 告警严重程度.
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// ConnectionState 连接状态.
type ConnectionState string

const (
	StateActive  ConnectionState = "active"
	StateIdle    ConnectionState = "idle"
	StateBlocked ConnectionState = "blocked"
	StateExpired ConnectionState = "expired"
)

// SMBConnection SMB 连接记录.
type SMBConnection struct {
	ID             string          `json:"id"`
	ClientIP       net.IP          `json:"client_ip"`
	ClientPort     int             `json:"client_port"`
	Username       string          `json:"username,omitempty"`
	ShareName      string          `json:"share_name,omitempty"`
	State          ConnectionState `json:"state"`
	AuthAttempts   int             `json:"auth_attempts"`
	FailedAttempts int             `json:"failed_attempts"`
	FirstSeen      time.Time       `json:"first_seen"`
	LastSeen       time.Time       `json:"last_seen"`
	BytesIn        int64           `json:"bytes_in"`
	BytesOut       int64           `json:"bytes_out"`
}

// BlockPolicy 自动封锁策略.
type BlockPolicy struct {
	ID                string      `json:"id"`
	Name              string      `json:"name"`
	Enabled           bool        `json:"enabled"`
	MaxFailedAttempts int         `json:"max_failed_attempts"`
	WindowSeconds     int         `json:"window_seconds"`
	BlockDuration     int         `json:"block_duration"`
	BlockAction       BlockAction `json:"block_action"`
	ApplyToUsers      []string    `json:"apply_to_users"`
	ApplyToShares     []string    `json:"apply_to_shares"`
	Priority          int         `json:"priority"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

// IPListType IP 列表类型.
type IPListType string

const (
	ListTypeWhitelist IPListType = "whitelist"
	ListTypeBlacklist IPListType = "blacklist"
)

// IPListEntry IP 列表条目.
type IPListEntry struct {
	ID        string     `json:"id"`
	IP        string     `json:"ip"`
	ListType  IPListType `json:"list_type"`
	Reason    string     `json:"reason"`
	AddedBy   string     `json:"added_by"`
	AddedAt   time.Time  `json:"added_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// BlockedIP 被封锁的 IP.
type BlockedIP struct {
	IP             string      `json:"ip"`
	Reason         string      `json:"reason"`
	PolicyID       string      `json:"policy_id"`
	PolicyName     string      `json:"policy_name"`
	BlockAction    BlockAction `json:"block_action"`
	BlockedAt      time.Time   `json:"blocked_at"`
	ExpiresAt      *time.Time  `json:"expires_at,omitempty"`
	FailedAttempts int         `json:"failed_attempts"`
	AutoBlocked    bool        `json:"auto_blocked"`
}

// Alert 告警.
type Alert struct {
	ID           string        `json:"id"`
	ClientIP     string        `json:"client_ip"`
	Severity     AlertSeverity `json:"severity"`
	Type         string        `json:"type"`
	Message      string        `json:"message"`
	PolicyID     string        `json:"policy_id,omitempty"`
	Username     string        `json:"username,omitempty"`
	Timestamp    time.Time     `json:"timestamp"`
	Acknowledged bool          `json:"acknowledged"`
}

// GuardConfig 防护引擎配置.
type GuardConfig struct {
	MaxConnections    int  `json:"max_connections"`
	MaxFailedAttempts int  `json:"max_failed_attempts"`
	WindowSeconds     int  `json:"window_seconds"`
	DefaultBlockDur   int  `json:"default_block_duration"`
	MaxAlerts         int  `json:"max_alerts"`
	EnableAutoBlock   bool `json:"enable_auto_block"`
	LogAllConnections bool `json:"log_all_connections"`
}

// DefaultGuardConfig 默认配置.
func DefaultGuardConfig() GuardConfig {
	return GuardConfig{
		MaxConnections:    1000,
		MaxFailedAttempts: 5,
		WindowSeconds:     300,
		DefaultBlockDur:   1800,
		MaxAlerts:         10000,
		EnableAutoBlock:   true,
		LogAllConnections: false,
	}
}

// 预定义错误.
var (
	ErrPolicyNotFound  = errors.New("block policy not found")
	ErrPolicyExists    = errors.New("block policy already exists")
	ErrIPAlreadyInList = errors.New("IP already in the specified list")
	ErrIPNotFound      = errors.New("IP not found in list")
	ErrInvalidIP       = errors.New("invalid IP address")
	ErrConnectionLimit = errors.New("connection limit exceeded")
	ErrIPBlocked       = errors.New("IP is blocked")
)

// Engine SMB 暴力破解防护引擎.
type Engine struct {
	mu            sync.RWMutex
	config        GuardConfig
	connections   map[string]*SMBConnection
	policies      map[string]*BlockPolicy
	blockedIPs    map[string]*BlockedIP
	whitelist     map[string]bool
	blacklist     map[string]bool
	ipListEntries map[string]*IPListEntry
	alerts        []Alert
	connCounter   int64
	alertCounter  int64
}

// NewEngine 创建 SMB 防护引擎.
func NewEngine(config GuardConfig) *Engine {
	if config.MaxFailedAttempts <= 0 {
		config = DefaultGuardConfig()
	}
	return &Engine{
		config:        config,
		connections:   make(map[string]*SMBConnection),
		policies:      make(map[string]*BlockPolicy),
		blockedIPs:    make(map[string]*BlockedIP),
		whitelist:     make(map[string]bool),
		blacklist:     make(map[string]bool),
		ipListEntries: make(map[string]*IPListEntry),
		alerts:        make([]Alert, 0, 1024),
	}
}

// --- 连接监控 ---

// OnConnect 处理 SMB 连接事件.
func (e *Engine) OnConnect(clientIP net.IP, port int, username string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	ipStr := clientIP.String()

	// 检查连接数限制
	if e.config.MaxConnections > 0 && len(e.connections) >= e.config.MaxConnections {
		return ErrConnectionLimit
	}

	// 检查黑名单
	if e.blacklist[ipStr] {
		return ErrIPBlocked
	}

	// 检查是否被封锁
	if blocked, ok := e.blockedIPs[ipStr]; ok {
		if blocked.ExpiresAt == nil || blocked.ExpiresAt.After(time.Now()) {
			return ErrIPBlocked
		}
		// 封锁已过期，移除
		delete(e.blockedIPs, ipStr)
	}

	// 记录连接
	e.connCounter++
	conn := &SMBConnection{
		ID:         fmt.Sprintf("conn-%d", e.connCounter),
		ClientIP:   clientIP,
		ClientPort: port,
		Username:   username,
		State:      StateActive,
		FirstSeen:  time.Now(),
		LastSeen:   time.Now(),
	}
	e.connections[ipStr] = conn

	return nil
}

// OnAuthFailure 处理认证失败事件.
func (e *Engine) OnAuthFailure(clientIP net.IP, username string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	ipStr := clientIP.String()

	conn, ok := e.connections[ipStr]
	if !ok {
		e.connCounter++
		conn = &SMBConnection{
			ID:        fmt.Sprintf("conn-%d", e.connCounter),
			ClientIP:  clientIP,
			Username:  username,
			State:     StateActive,
			FirstSeen: time.Now(),
		}
		e.connections[ipStr] = conn
	}

	conn.AuthAttempts++
	conn.FailedAttempts++
	conn.LastSeen = time.Now()

	// 白名单 IP 不触发自动封锁
	if e.whitelist[ipStr] {
		return
	}

	// 检查封锁策略
	if e.config.EnableAutoBlock {
		e.evaluateBlockPolicies(ipStr, conn)
	}
}

// OnAuthSuccess 处理认证成功事件.
func (e *Engine) OnAuthSuccess(clientIP net.IP, username string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	ipStr := clientIP.String()

	conn, ok := e.connections[ipStr]
	if ok {
		conn.AuthAttempts++
		conn.LastSeen = time.Now()
	}
}

// OnDisconnect 处理断开连接事件.
func (e *Engine) OnDisconnect(clientIP net.IP) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.connections, clientIP.String())
}

// GetConnection 获取连接信息.
func (e *Engine) GetConnection(clientIP net.IP) *SMBConnection {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.connections[clientIP.String()]
}

// ListConnections 列出所有活动连接.
func (e *Engine) ListConnections() []*SMBConnection {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*SMBConnection, 0, len(e.connections))
	for _, conn := range e.connections {
		result = append(result, conn)
	}
	return result
}

// --- 自动封锁策略 ---

// CreatePolicy 创建封锁策略.
func (e *Engine) CreatePolicy(p *BlockPolicy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if p.ID == "" {
		return ErrPolicyNotFound
	}
	if _, exists := e.policies[p.ID]; exists {
		return ErrPolicyExists
	}

	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	e.policies[p.ID] = p
	return nil
}

// UpdatePolicy 更新封锁策略.
func (e *Engine) UpdatePolicy(policyID string, updated *BlockPolicy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.policies[policyID]; !ok {
		return ErrPolicyNotFound
	}

	updated.ID = policyID
	updated.UpdatedAt = time.Now()
	e.policies[policyID] = updated
	return nil
}

// DeletePolicy 删除封锁策略.
func (e *Engine) DeletePolicy(policyID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.policies[policyID]; !ok {
		return ErrPolicyNotFound
	}
	delete(e.policies, policyID)
	return nil
}

// GetPolicy 获取策略.
func (e *Engine) GetPolicy(policyID string) (*BlockPolicy, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	p, ok := e.policies[policyID]
	if !ok {
		return nil, ErrPolicyNotFound
	}
	return p, nil
}

// ListPolicies 列出所有策略.
func (e *Engine) ListPolicies() []*BlockPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*BlockPolicy, 0, len(e.policies))
	for _, p := range e.policies {
		result = append(result, p)
	}
	return result
}

// evaluateBlockPolicies 评估封锁策略（需持有写锁）.
func (e *Engine) evaluateBlockPolicies(ipStr string, conn *SMBConnection) {
	// 先用默认策略
	maxFails := e.config.MaxFailedAttempts
	window := time.Duration(e.config.WindowSeconds) * time.Second
	blockDur := e.config.DefaultBlockDur
	action := BlockActionTemp
	policyID := "default"
	policyName := "Default Auto-Block"

	// 找最高优先级的匹配策略
	for _, p := range e.policies {
		if !p.Enabled {
			continue
		}
		if p.MaxFailedAttempts <= 0 {
			continue
		}
		if len(p.ApplyToUsers) > 0 && !stringInSlice(conn.Username, p.ApplyToUsers) {
			continue
		}
		if len(p.ApplyToShares) > 0 && !stringInSlice(conn.ShareName, p.ApplyToShares) {
			continue
		}
		maxFails = p.MaxFailedAttempts
		window = time.Duration(p.WindowSeconds) * time.Second
		blockDur = p.BlockDuration
		action = p.BlockAction
		policyID = p.ID
		policyName = p.Name
		break
	}

	// 检查窗口内的失败次数
	if conn.FailedAttempts >= maxFails {
		if window <= 0 || conn.LastSeen.Sub(conn.FirstSeen) <= window {
			e.doBlockIP(ipStr, &BlockedIP{
				IP:             ipStr,
				Reason:         fmt.Sprintf("SMB brute force: %d failed attempts in %v window", conn.FailedAttempts, window),
				PolicyID:       policyID,
				PolicyName:     policyName,
				BlockAction:    action,
				BlockedAt:      time.Now(),
				FailedAttempts: conn.FailedAttempts,
				AutoBlocked:    true,
			}, blockDur)

			e.addAlert(&Alert{
				ClientIP:  ipStr,
				Severity:  AlertSeverityCritical,
				Type:      "brute_force",
				Message:   fmt.Sprintf("IP %s auto-blocked: %d failed SMB auth attempts", ipStr, conn.FailedAttempts),
				PolicyID:  policyID,
				Username:  conn.Username,
				Timestamp: time.Now(),
			})
		}
	}
}

// --- IP 白/黑名单 ---

// AddToWhitelist 添加 IP 到白名单.
func (e *Engine) AddToWhitelist(ipStr string, reason string, addedBy string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ErrInvalidIP
	}

	e.whitelist[ipStr] = true
	e.ipListEntries[fmt.Sprintf("wl-%s", ipStr)] = &IPListEntry{
		ID:       fmt.Sprintf("wl-%s", ipStr),
		IP:       ipStr,
		ListType: ListTypeWhitelist,
		Reason:   reason,
		AddedBy:  addedBy,
		AddedAt:  time.Now(),
	}

	// 如果 IP 之前被封锁，解除封锁
	delete(e.blockedIPs, ipStr)

	return nil
}

// RemoveFromWhitelist 从白名单移除 IP.
func (e *Engine) RemoveFromWhitelist(ipStr string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.whitelist[ipStr] {
		return ErrIPNotFound
	}

	delete(e.whitelist, ipStr)
	delete(e.ipListEntries, fmt.Sprintf("wl-%s", ipStr))
	return nil
}

// AddToBlacklist 添加 IP 到黑名单.
func (e *Engine) AddToBlacklist(ipStr string, reason string, addedBy string, durationSeconds int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ErrInvalidIP
	}

	e.blacklist[ipStr] = true
	entry := &IPListEntry{
		ID:       fmt.Sprintf("bl-%s", ipStr),
		IP:       ipStr,
		ListType: ListTypeBlacklist,
		Reason:   reason,
		AddedBy:  addedBy,
		AddedAt:  time.Now(),
	}
	if durationSeconds > 0 {
		exp := time.Now().Add(time.Duration(durationSeconds) * time.Second)
		entry.ExpiresAt = &exp
	}
	e.ipListEntries[entry.ID] = entry

	// 同时添加到封锁列表
	e.doBlockIP(ipStr, &BlockedIP{
		IP:          ipStr,
		Reason:      reason,
		BlockAction: BlockActionPerm,
		BlockedAt:   time.Now(),
		ExpiresAt:   entry.ExpiresAt,
		AutoBlocked: false,
	}, durationSeconds)

	return nil
}

// RemoveFromBlacklist 从黑名单移除 IP.
func (e *Engine) RemoveFromBlacklist(ipStr string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.blacklist[ipStr] {
		return ErrIPNotFound
	}

	delete(e.blacklist, ipStr)
	delete(e.ipListEntries, fmt.Sprintf("bl-%s", ipStr))
	delete(e.blockedIPs, ipStr)
	return nil
}

// IsWhitelisted 检查 IP 是否在白名单.
func (e *Engine) IsWhitelisted(ipStr string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.whitelist[ipStr]
}

// IsBlacklisted 检查 IP 是否在黑名单.
func (e *Engine) IsBlacklisted(ipStr string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.blacklist[ipStr]
}

// ListWhitelist 列出白名单.
func (e *Engine) ListWhitelist() []*IPListEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*IPListEntry, 0)
	for _, entry := range e.ipListEntries {
		if entry.ListType == ListTypeWhitelist {
			result = append(result, entry)
		}
	}
	return result
}

// ListBlacklist 列出黑名单.
func (e *Engine) ListBlacklist() []*IPListEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*IPListEntry, 0)
	for _, entry := range e.ipListEntries {
		if entry.ListType == ListTypeBlacklist {
			result = append(result, entry)
		}
	}
	return result
}

// --- 封锁管理 ---

// doBlockIP 封锁 IP（需持有写锁）.
func (e *Engine) doBlockIP(ipStr string, blocked *BlockedIP, durationSeconds int) {
	if durationSeconds > 0 {
		exp := blocked.BlockedAt.Add(time.Duration(durationSeconds) * time.Second)
		blocked.ExpiresAt = &exp
	}
	e.blockedIPs[ipStr] = blocked
}

// UnblockIP 手动解除封锁.
func (e *Engine) UnblockIP(ipStr string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.blockedIPs[ipStr]; !ok {
		return ErrIPNotFound
	}

	delete(e.blockedIPs, ipStr)

	e.addAlert(&Alert{
		ClientIP:  ipStr,
		Severity:  AlertSeverityInfo,
		Type:      "unblock",
		Message:   fmt.Sprintf("IP %s manually unblocked", ipStr),
		Timestamp: time.Now(),
	})

	return nil
}

// GetBlockedIPs 获取所有被封锁的 IP.
func (e *Engine) GetBlockedIPs() []*BlockedIP {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*BlockedIP, 0, len(e.blockedIPs))
	for _, blocked := range e.blockedIPs {
		result = append(result, blocked)
	}
	return result
}

// IsIPBlocked 检查 IP 是否被封锁.
func (e *Engine) IsIPBlocked(ipStr string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	blocked, ok := e.blockedIPs[ipStr]
	if !ok {
		return false
	}
	if blocked.ExpiresAt != nil && blocked.ExpiresAt.Before(time.Now()) {
		return false
	}
	return true
}

// --- 告警通知 ---

// GetAlerts 获取告警列表.
func (e *Engine) GetAlerts(limit int, unacknowledgedOnly bool) []Alert {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.alerts) {
		limit = len(e.alerts)
	}

	result := make([]Alert, 0, limit)
	for i := len(e.alerts) - 1; i >= 0 && len(result) < limit; i-- {
		if unacknowledgedOnly && e.alerts[i].Acknowledged {
			continue
		}
		result = append(result, e.alerts[i])
	}
	return result
}

// AcknowledgeAlert 确认告警.
func (e *Engine) AcknowledgeAlert(alertID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.alerts {
		if e.alerts[i].ID == alertID {
			e.alerts[i].Acknowledged = true
			return nil
		}
	}
	return errors.New("alert not found")
}

// addAlert 添加告警（需持有写锁）.
func (e *Engine) addAlert(alert *Alert) {
	e.alertCounter++
	if alert.ID == "" {
		alert.ID = fmt.Sprintf("alert-%d", e.alertCounter)
	}
	e.alerts = append(e.alerts, *alert)
	if len(e.alerts) > e.config.MaxAlerts {
		e.alerts = e.alerts[len(e.alerts)-e.config.MaxAlerts:]
	}
}

// CleanupExpired 清理过期的封锁和连接，可由定时任务定期调用。
func (e *Engine) CleanupExpired() int {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	count := 0

	// 清理过期封锁
	for ipStr, blocked := range e.blockedIPs {
		if blocked.ExpiresAt != nil && blocked.ExpiresAt.Before(now) {
			delete(e.blockedIPs, ipStr)
			count++
		}
	}

	// 清理过期黑名单条目
	for id, entry := range e.ipListEntries {
		if entry.ExpiresAt != nil && entry.ExpiresAt.Before(now) {
			delete(e.ipListEntries, id)
			delete(e.blacklist, entry.IP)
		}
	}

	return count
}

func stringInSlice(value string, items []string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
