package smb

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// SMBSecurityConfig SMB安全配置.
type SMBSecurityConfig struct {
	// IP访问控制
	IPWhitelist []string `json:"ip_whitelist"`
	IPBlacklist []string `json:"ip_blacklist"`

	// 限流配置
	RateLimit RateLimitConfig `json:"rate_limit"`

	// SMB审计
	AuditEnabled bool   `json:"audit_enabled"`
	AuditLogPath string `json:"audit_log_path"`

	// 加密配置
	MinProtocol string `json:"min_protocol"` // SMB2, SMB3
	EncryptData bool   `json:"encrypt_data"`

	// 自动封禁配置
	AutoBanEnabled      bool `json:"auto_ban_enabled"`
	AutoBanThreshold    int  `json:"auto_ban_threshold"`     // 失败次数阈值
	AutoBanWindowMins   int  `json:"auto_ban_window_mins"`   // 检测窗口（分钟）
	AutoBanDurationMins int  `json:"auto_ban_duration_mins"` // 封禁时长（分钟）
}

// RateLimitConfig 限流配置.
type RateLimitConfig struct {
	Enabled       bool `json:"enabled"`
	MaxConnPerIP  int  `json:"max_conn_per_ip"`
	MaxConnTotal  int  `json:"max_conn_total"`
	WindowSeconds int  `json:"window_seconds"`
}

// IPBanEntry IP封禁记录.
type IPBanEntry struct {
	IP        string    `json:"ip"`
	Reason    string    `json:"reason"`
	BannedAt  time.Time `json:"banned_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// FailedAttempt 失败尝试记录.
type FailedAttempt struct {
	IP       string    `json:"ip"`
	Username string    `json:"username"`
	Time     time.Time `json:"time"`
	Reason   string    `json:"reason"`
}

// AuditLogEntry 审计日志条目.
type AuditLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
	IP        string    `json:"ip"`
	Username  string    `json:"username,omitempty"`
	ShareName string    `json:"share_name,omitempty"`
	Action    string    `json:"action"`
	Result    string    `json:"result"` // success, denied, error
	Details   string    `json:"details,omitempty"`
}

// SecurityManager SMB安全管理器.
type SecurityManager struct {
	mu             sync.RWMutex
	config         *SMBSecurityConfig
	bannedIPs      map[string]*IPBanEntry
	failedAttempts map[string][]FailedAttempt // IP -> attempts
	auditLog       *AuditLogger
	configPath     string
	logger         *zap.SugaredLogger

	// 连接计数（用于限流）
	connCounts map[string]int // IP -> count
	totalConns int
}

// AuditLogger 审计日志记录器.
type AuditLogger struct {
	mu      sync.Mutex
	file    *os.File
	logPath string
	logger  *zap.SugaredLogger
}

// NewSecurityManager 创建安全管理器.
func NewSecurityManager(configPath string, logger *zap.SugaredLogger) *SecurityManager {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	sm := &SecurityManager{
		config:         newDefaultSecurityConfig(),
		bannedIPs:      make(map[string]*IPBanEntry),
		failedAttempts: make(map[string][]FailedAttempt),
		configPath:     configPath,
		logger:         logger,
		connCounts:     make(map[string]int),
	}

	// 加载配置
	if err := sm.loadConfig(); err != nil {
		logger.Warnw("加载安全配置失败，使用默认配置", "error", err)
	}

	// 初始化审计日志
	if sm.config.AuditEnabled && sm.config.AuditLogPath != "" {
		auditLogger, err := NewAuditLogger(sm.config.AuditLogPath, logger)
		if err != nil {
			logger.Warnw("初始化审计日志失败", "error", err)
		} else {
			sm.auditLog = auditLogger
		}
	}

	return sm
}

// newDefaultSecurityConfig 创建默认安全配置.
func newDefaultSecurityConfig() *SMBSecurityConfig {
	return &SMBSecurityConfig{
		IPWhitelist: []string{},
		IPBlacklist: []string{},
		RateLimit: RateLimitConfig{
			Enabled:       true,
			MaxConnPerIP:  10,
			MaxConnTotal:  1000,
			WindowSeconds: 60,
		},
		AuditEnabled:        true,
		AuditLogPath:        "/var/log/samba/audit.json",
		MinProtocol:         "SMB2",
		EncryptData:         false,
		AutoBanEnabled:      true,
		AutoBanThreshold:    5,
		AutoBanWindowMins:   5,
		AutoBanDurationMins: 30,
	}
}

// loadConfig 加载配置.
func (sm *SecurityManager) loadConfig() error {
	data, err := os.ReadFile(sm.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 配置文件不存在，使用默认配置
		}
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg SMBSecurityConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	sm.mu.Lock()
	sm.config = &cfg
	sm.mu.Unlock()
	return nil
}

// saveConfig 保存配置.
func (sm *SecurityManager) saveConfig() error {
	sm.mu.RLock()
	cfg := sm.config
	sm.mu.RUnlock()
	return sm.saveConfigFrom(cfg)
}

// saveConfigFrom 从指定配置保存（已持有锁的外部调用）.
func (sm *SecurityManager) saveConfigFrom(cfg *SMBSecurityConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(sm.configPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	tmpPath := sm.configPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0640); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	if err := os.Rename(tmpPath, sm.configPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("重命名配置文件失败: %w", err)
	}

	return nil
}

// GetConfig 获取安全配置.
func (sm *SecurityManager) GetConfig() *SMBSecurityConfig {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.config
}

// UpdateConfig 更新安全配置.
func (sm *SecurityManager) UpdateConfig(cfg *SMBSecurityConfig) error {
	sm.mu.Lock()
	sm.config = cfg

	// 如果审计配置变更，重新初始化审计日志
	if cfg.AuditEnabled && cfg.AuditLogPath != "" {
		auditLogger, err := NewAuditLogger(cfg.AuditLogPath, sm.logger)
		if err != nil {
			sm.mu.Unlock()
			return fmt.Errorf("初始化审计日志失败: %w", err)
		}
		if sm.auditLog != nil {
			_ = sm.auditLog.Close()
		}
		sm.auditLog = auditLogger
	}
	sm.mu.Unlock()

	return sm.saveConfig()
}

// CheckIPAllowed 检查IP是否允许访问.
func (sm *SecurityManager) CheckIPAllowed(ip string) (bool, string) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 检查封禁列表
	if ban, exists := sm.bannedIPs[ip]; exists {
		if time.Now().Before(ban.ExpiresAt) {
			return false, fmt.Sprintf("IP已封禁: %s (原因: %s, 解封时间: %s)",
				ip, ban.Reason, ban.ExpiresAt.Format("2006-01-02 15:04:05"))
		}
		// 封禁已过期，移除
		delete(sm.bannedIPs, ip)
	}

	// 检查黑名单
	for _, blackIP := range sm.config.IPBlacklist {
		if matchIP(ip, blackIP) {
			return false, fmt.Sprintf("IP在黑名单中: %s", ip)
		}
	}

	// 如果有白名单，只允许白名单中的IP
	if len(sm.config.IPWhitelist) > 0 {
		allowed := false
		for _, whiteIP := range sm.config.IPWhitelist {
			if matchIP(ip, whiteIP) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false, fmt.Sprintf("IP不在白名单中: %s", ip)
		}
	}

	return true, ""
}

// CheckRateLimit 检查是否超过限流阈值.
func (sm *SecurityManager) CheckRateLimit(ip string) (bool, string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.config.RateLimit.Enabled {
		return true, ""
	}

	// 检查单IP连接数
	if sm.config.RateLimit.MaxConnPerIP > 0 {
		if sm.connCounts[ip] >= sm.config.RateLimit.MaxConnPerIP {
			return false, fmt.Sprintf("IP连接数超限: %s (当前: %d, 最大: %d)",
				ip, sm.connCounts[ip], sm.config.RateLimit.MaxConnPerIP)
		}
	}

	// 检查总连接数
	if sm.config.RateLimit.MaxConnTotal > 0 {
		if sm.totalConns >= sm.config.RateLimit.MaxConnTotal {
			return false, fmt.Sprintf("总连接数超限 (当前: %d, 最大: %d)",
				sm.totalConns, sm.config.RateLimit.MaxConnTotal)
		}
	}

	return true, ""
}

// IncrementConnection 增加连接计数.
func (sm *SecurityManager) IncrementConnection(ip string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.connCounts[ip]++
	sm.totalConns++
}

// DecrementConnection 减少连接计数.
func (sm *SecurityManager) DecrementConnection(ip string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.connCounts[ip] > 0 {
		sm.connCounts[ip]--
	}
	if sm.totalConns > 0 {
		sm.totalConns--
	}
}

// RecordFailedAttempt 记录失败尝试.
func (sm *SecurityManager) RecordFailedAttempt(ip, username, reason string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	attempt := FailedAttempt{
		IP:       ip,
		Username: username,
		Time:     time.Now(),
		Reason:   reason,
	}

	sm.failedAttempts[ip] = append(sm.failedAttempts[ip], attempt)

	// 清理过期记录
	sm.cleanupFailedAttemptsLocked(ip)

	// 记录审计日志
	if sm.auditLog != nil {
		sm.auditLog.Log(AuditLogEntry{
			Timestamp: time.Now(),
			EventType: "auth_failed",
			IP:        ip,
			Username:  username,
			Action:    "authenticate",
			Result:    "denied",
			Details:   reason,
		})
	}

	// 检查是否需要自动封禁
	if sm.config.AutoBanEnabled {
		if len(sm.failedAttempts[ip]) >= sm.config.AutoBanThreshold {
			sm.autoBanIPLocked(ip, "暴力破解检测")
		}
	}
}

// cleanupFailedAttemptsLocked 清理过期的失败尝试记录（调用时已持有锁）.
func (sm *SecurityManager) cleanupFailedAttemptsLocked(ip string) {
	window := time.Duration(sm.config.AutoBanWindowMins) * time.Minute
	cutoff := time.Now().Add(-window)

	valid := make([]FailedAttempt, 0)
	for _, attempt := range sm.failedAttempts[ip] {
		if attempt.Time.After(cutoff) {
			valid = append(valid, attempt)
		}
	}
	sm.failedAttempts[ip] = valid
}

// autoBanIPLocked 自动封禁IP（调用时已持有锁）.
func (sm *SecurityManager) autoBanIPLocked(ip, reason string) {
	// 检查是否已在白名单
	for _, whiteIP := range sm.config.IPWhitelist {
		if matchIP(ip, whiteIP) {
			sm.logger.Infow("白名单IP不会被自动封禁", "ip", ip)
			return
		}
	}

	ban := &IPBanEntry{
		IP:        ip,
		Reason:    reason,
		BannedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Duration(sm.config.AutoBanDurationMins) * time.Minute),
	}
	sm.bannedIPs[ip] = ban

	sm.logger.Warnw("IP已被自动封禁", "ip", ip, "reason", reason,
		"expires_at", ban.ExpiresAt)

	// 记录审计日志
	if sm.auditLog != nil {
		sm.auditLog.Log(AuditLogEntry{
			Timestamp: time.Now(),
			EventType: "ip_banned",
			IP:        ip,
			Action:    "ban",
			Result:    "success",
			Details:   reason,
		})
	}
}

// BanIP 手动封禁IP.
func (sm *SecurityManager) BanIP(ip, reason string, durationMins int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 检查是否在白名单
	for _, whiteIP := range sm.config.IPWhitelist {
		if matchIP(ip, whiteIP) {
			return fmt.Errorf("白名单IP不能被封禁: %s", ip)
		}
	}

	ban := &IPBanEntry{
		IP:        ip,
		Reason:    reason,
		BannedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Duration(durationMins) * time.Minute),
	}

	// 如果永久封禁
	if durationMins <= 0 {
		ban.ExpiresAt = time.Now().AddDate(100, 0, 0) // 100年后
	}

	sm.bannedIPs[ip] = ban

	sm.logger.Infow("IP已封禁", "ip", ip, "reason", reason, "duration_mins", durationMins)

	// 记录审计日志
	if sm.auditLog != nil {
		sm.auditLog.Log(AuditLogEntry{
			Timestamp: time.Now(),
			EventType: "ip_banned",
			IP:        ip,
			Action:    "manual_ban",
			Result:    "success",
			Details:   fmt.Sprintf("%s (duration: %d mins)", reason, durationMins),
		})
	}

	return nil
}

// UnbanIP 解封IP.
func (sm *SecurityManager) UnbanIP(ip string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.bannedIPs[ip]; !exists {
		return fmt.Errorf("IP未被封禁: %s", ip)
	}

	delete(sm.bannedIPs, ip)

	sm.logger.Infow("IP已解封", "ip", ip)

	// 记录审计日志
	if sm.auditLog != nil {
		sm.auditLog.Log(AuditLogEntry{
			Timestamp: time.Now(),
			EventType: "ip_unbanned",
			IP:        ip,
			Action:    "unban",
			Result:    "success",
		})
	}

	return nil
}

// GetBannedIPs 获取封禁IP列表.
func (sm *SecurityManager) GetBannedIPs() []*IPBanEntry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*IPBanEntry, 0, len(sm.bannedIPs))
	for _, ban := range sm.bannedIPs {
		// 只返回未过期的封禁
		if time.Now().Before(ban.ExpiresAt) {
			result = append(result, ban)
		}
	}
	return result
}

// AddToWhitelist 添加IP到白名单.
func (sm *SecurityManager) AddToWhitelist(ip string) error {
	sm.mu.Lock()

	// 检查是否已在白名单
	for _, existing := range sm.config.IPWhitelist {
		if existing == ip {
			sm.mu.Unlock()
			return nil // 已存在
		}
	}

	sm.config.IPWhitelist = append(sm.config.IPWhitelist, ip)

	// 从封禁列表中移除
	delete(sm.bannedIPs, ip)

	// 复制配置后释放锁，再保存（避免死锁）
	cfg := sm.config
	sm.mu.Unlock()

	sm.logger.Infow("IP已添加到白名单", "ip", ip)
	return sm.saveConfigFrom(cfg)
}

// RemoveFromWhitelist 从白名单移除IP.
func (sm *SecurityManager) RemoveFromWhitelist(ip string) error {
	sm.mu.Lock()

	for i, existing := range sm.config.IPWhitelist {
		if existing == ip {
			sm.config.IPWhitelist = append(
				sm.config.IPWhitelist[:i],
				sm.config.IPWhitelist[i+1:]...)
			cfg := sm.config
			sm.mu.Unlock()
			sm.logger.Infow("IP已从白名单移除", "ip", ip)
			return sm.saveConfigFrom(cfg)
		}
	}

	sm.mu.Unlock()
	return fmt.Errorf("IP不在白名单中: %s", ip)
}

// AddToBlacklist 添加IP到黑名单.
func (sm *SecurityManager) AddToBlacklist(ip string) error {
	sm.mu.Lock()

	// 检查是否在白名单
	for _, whiteIP := range sm.config.IPWhitelist {
		if whiteIP == ip {
			sm.mu.Unlock()
			return fmt.Errorf("IP在白名单中，不能添加到黑名单: %s", ip)
		}
	}

	// 检查是否已在黑名单
	for _, existing := range sm.config.IPBlacklist {
		if existing == ip {
			sm.mu.Unlock()
			return nil // 已存在
		}
	}

	sm.config.IPBlacklist = append(sm.config.IPBlacklist, ip)
	cfg := sm.config
	sm.mu.Unlock()

	sm.logger.Infow("IP已添加到黑名单", "ip", ip)
	return sm.saveConfigFrom(cfg)
}

// RemoveFromBlacklist 从黑名单移除IP.
func (sm *SecurityManager) RemoveFromBlacklist(ip string) error {
	sm.mu.Lock()

	for i, existing := range sm.config.IPBlacklist {
		if existing == ip {
			sm.config.IPBlacklist = append(
				sm.config.IPBlacklist[:i],
				sm.config.IPBlacklist[i+1:]...)
			cfg := sm.config
			sm.mu.Unlock()
			sm.logger.Infow("IP已从黑名单移除", "ip", ip)
			return sm.saveConfigFrom(cfg)
		}
	}

	sm.mu.Unlock()
	return fmt.Errorf("IP不在黑名单中: %s", ip)
}

// LogAccess 记录访问日志.
func (sm *SecurityManager) LogAccess(ip, username, shareName, action, result, details string) {
	sm.mu.RLock()
	auditLog := sm.auditLog
	sm.mu.RUnlock()

	if auditLog == nil {
		return
	}

	auditLog.Log(AuditLogEntry{
		Timestamp: time.Now(),
		EventType: "access",
		IP:        ip,
		Username:  username,
		ShareName: shareName,
		Action:    action,
		Result:    result,
		Details:   details,
	})
}

// GetAuditLogs 获取审计日志.
func (sm *SecurityManager) GetAuditLogs(limit int, offset int) ([]AuditLogEntry, error) {
	sm.mu.RLock()
	auditLog := sm.auditLog
	sm.mu.RUnlock()

	if auditLog == nil {
		return nil, fmt.Errorf("审计日志未启用")
	}

	return auditLog.ReadLogs(limit, offset)
}

// CleanupExpiredBans 清理过期的封禁记录.
func (sm *SecurityManager) CleanupExpiredBans() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	count := 0
	for ip, ban := range sm.bannedIPs {
		if now.After(ban.ExpiresAt) {
			delete(sm.bannedIPs, ip)
			count++
		}
	}

	if count > 0 {
		sm.logger.Infow("清理过期封禁记录", "count", count)
	}

	return count
}

// matchIP 检查IP是否匹配（支持CIDR）.
func matchIP(ip, pattern string) bool {
	// 直接匹配
	if ip == pattern {
		return true
	}

	// CIDR匹配
	if _, cidr, err := net.ParseCIDR(pattern); err == nil {
		peerIP := net.ParseIP(ip)
		if peerIP != nil {
			return cidr.Contains(peerIP)
		}
	}

	return false
}

// NewAuditLogger 创建审计日志记录器.
func NewAuditLogger(logPath string, logger *zap.SugaredLogger) (*AuditLogger, error) {
	// 确保目录存在
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return nil, fmt.Errorf("打开日志文件失败: %w", err)
	}

	return &AuditLogger{
		file:    file,
		logPath: logPath,
		logger:  logger,
	}, nil
}

// Log 记录审计日志.
func (al *AuditLogger) Log(entry AuditLogEntry) {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.file == nil {
		return
	}

	data, err := json.Marshal(entry)
	if err != nil {
		al.logger.Warnw("序列化审计日志失败", "error", err)
		return
	}

	_, err = al.file.WriteString(string(data) + "\n")
	if err != nil {
		al.logger.Warnw("写入审计日志失败", "error", err)
	}
}

// ReadLogs 读取审计日志.
func (al *AuditLogger) ReadLogs(limit int, offset int) ([]AuditLogEntry, error) {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.file == nil {
		return nil, fmt.Errorf("日志文件未打开")
	}

	// 关闭当前文件句柄以便读取
	_ = al.file.Close()

	file, err := os.Open(al.logPath)
	if err != nil {
		// 重新打开文件用于写入
		al.file, _ = os.OpenFile(al.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
		return nil, fmt.Errorf("打开日志文件失败: %w", err)
	}
	defer func() { _ = file.Close() }()

	// 重新打开文件用于写入
	al.file, _ = os.OpenFile(al.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)

	// 读取所有行
	entries := make([]AuditLogEntry, 0)
	scanner := NewReverseScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if offset > 0 && lineNum <= offset {
			continue
		}
		if limit > 0 && len(entries) >= limit {
			break
		}

		var entry AuditLogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

// Close 关闭日志文件.
func (al *AuditLogger) Close() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.file != nil {
		err := al.file.Close()
		al.file = nil
		return err
	}
	return nil
}

// ReverseScanner 反向扫描器.
type ReverseScanner struct {
	file   *os.File
	offset int64
	buffer []byte
	eof    bool
}

// NewReverseScanner 创建反向扫描器.
func NewReverseScanner(file *os.File) *ReverseScanner {
	info, _ := file.Stat()
	return &ReverseScanner{
		file:   file,
		offset: info.Size(),
		buffer: make([]byte, 1024),
	}
}

// Scan 扫描下一行.
func (rs *ReverseScanner) Scan() bool {
	if rs.eof || rs.offset <= 0 {
		rs.eof = true
		return false
	}

	// 向后读取直到找到换行符
	var line []byte
	for rs.offset > 0 {
		readSize := int64(len(rs.buffer))
		if readSize > rs.offset {
			readSize = rs.offset
		}
		rs.offset -= readSize

		_, _ = rs.file.ReadAt(rs.buffer[:readSize], rs.offset)

		for i := int(readSize) - 1; i >= 0; i-- {
			if rs.buffer[i] == '\n' {
				if len(line) > 0 {
					rs.offset += int64(i + 1)
					rs.buffer = append(rs.buffer[:0], line...)
					return true
				}
			} else {
				line = append([]byte{rs.buffer[i]}, line...)
			}
		}
	}

	if len(line) > 0 {
		rs.buffer = append(rs.buffer[:0], line...)
		return true
	}

	rs.eof = true
	return false
}

// Bytes 返回当前行的字节.
func (rs *ReverseScanner) Bytes() []byte {
	return rs.buffer
}

// Err 返回错误.
func (rs *ReverseScanner) Err() error {
	return nil
}
