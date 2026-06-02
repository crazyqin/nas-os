package ipprotection

import (
	"fmt"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ==================== IP 防护管理器 ====================

// Manager IP 防护管理器
type Manager struct {
	mu           sync.RWMutex
	logger       *zap.Logger
	config       *IPProtectionConfig
	detector     *Detector
	rateLimiter  *RateLimiterManager
	records      map[string]*IPRecord    // IP -> 记录
	allowList    map[string]*AllowListEntry
	denyList     map[string]*DenyListEntry
	banLog       []*BanRecord
	stopChan     chan struct{}
	running      bool
}

// NewManager 创建 IP 防护管理器
func NewManager(logger *zap.Logger, config *IPProtectionConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultIPProtectionConfig()
	}

	m := &Manager{
		logger:      logger,
		config:      config,
		detector:    NewDetector(logger, config),
		rateLimiter: NewRateLimiterManager(config),
		records:     make(map[string]*IPRecord),
		allowList:   make(map[string]*AllowListEntry),
		denyList:    make(map[string]*DenyListEntry),
		banLog:      make([]*BanRecord, 0),
		stopChan:    make(chan struct{}),
	}

	// 初始化黑白名单
	m.initLists()

	return m
}

// initLists 初始化黑白名单
func (m *Manager) initLists() {
	now := time.Now()
	for _, ip := range m.config.WhitelistedIPs {
		m.allowList[ip] = &AllowListEntry{
			IP:       ip,
			IsIPv6:   isIPv6(ip),
			Comment:  "默认白名单",
			AddedAt:  now,
			IsActive: true,
		}
	}
	for _, ip := range m.config.BlacklistedIPs {
		m.denyList[ip] = &DenyListEntry{
			IP:       ip,
			IsIPv6:   isIPv6(ip),
			Reason:   BanReasonManual,
			Comment:  "默认黑名单",
			AddedAt:  now,
			IsActive: true,
		}
	}
}

// Start 启动管理器
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	// 启动信誉恢复协程
	go m.reputationRecoveryLoop()

	m.logger.Info("IP 防护管理器已启动")
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}
	m.running = false
	close(m.stopChan)
	m.rateLimiter.Stop()

	m.logger.Info("IP 防护管理器已停止")
}

// ==================== 核心检查方法 ====================

// CheckRequest 检查请求是否允许
// 返回：是否允许、拒绝原因
func (m *Manager) CheckRequest(ip string) (bool, BanReason) {
	// 1. 检查白名单
	if m.isWhitelisted(ip) {
		return true, ""
	}

	// 2. 检查黑名单
	if m.isBlacklisted(ip) {
		return false, BanReasonManual
	}

	// 3. 检查自动封禁状态
	m.mu.RLock()
	record, exists := m.records[ip]
	if exists && record.IsBanned {
		if time.Now().Before(record.BanExpiry) {
			m.mu.RUnlock()
			return false, record.BanReason
		}
		// 封禁已过期，解除
		m.mu.RUnlock()
		m.unbanIP(ip)
	} else {
		m.mu.RUnlock()
	}

	// 4. 检查频率限制
	if !m.rateLimiter.Allow(ip) {
		m.onRateLimitExceeded(ip)
		return false, BanReasonRateLimit
	}

	// 5. 更新访问记录
	m.touchRecord(ip)

	return true, ""
}

// ProcessLoginAttempt 处理登录尝试
func (m *Manager) ProcessLoginAttempt(attempt *LoginAttempt) {
	// 白名单 IP 不处理
	if m.isWhitelisted(attempt.IP) {
		return
	}

	// 记录到检测器
	m.detector.RecordLoginAttempt(attempt)

	if attempt.Success {
		m.onLoginSuccess(attempt.IP)
		return
	}

	// 登录失败
	m.onLoginFailure(attempt.IP, attempt)
}

// RecordPortAccess 记录端口访问
func (m *Manager) RecordPortAccess(ip string, port int) {
	if m.isWhitelisted(ip) {
		return
	}

	m.detector.RecordAccess(&AccessRecord{
		IP:        ip,
		Port:      port,
		Timestamp: time.Now(),
	})

	// 检测端口扫描
	result := m.detector.DetectPortScan(ip)
	if result.Detected {
		m.onPortScanDetected(ip, result)
	}
}

// RecordHTTPAccess 记录 HTTP 访问
func (m *Manager) RecordHTTPAccess(ip, path, userAgent string) {
	if m.isWhitelisted(ip) {
		return
	}

	m.detector.RecordAccess(&AccessRecord{
		IP:        ip,
		Path:      path,
		UserAgent: userAgent,
		Timestamp: time.Now(),
	})

	// 检测可疑 User-Agent
	uaResult := m.detector.DetectSuspiciousUserAgent(ip, userAgent)
	if uaResult.Detected && uaResult.Confidence > 0.7 {
		m.onSuspiciousActivity(ip, uaResult)
	}

	// 检测暴力破解
	bfResult := m.detector.DetectBruteForce(ip)
	if bfResult.Detected {
		m.onBruteForceDetected(ip, bfResult)
	}
}

// ==================== 黑白名单管理 ====================

// AddToAllowList 添加到白名单
func (m *Manager) AddToAllowList(ip, comment string, duration time.Duration) error {
	if err := validateIP(ip); err != nil {
		return fmt.Errorf("无效的 IP 地址: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	entry := &AllowListEntry{
		IP:       ip,
		IsIPv6:   isIPv6(ip),
		Comment:  comment,
		AddedAt:  time.Now(),
		IsActive: true,
	}
	if duration > 0 {
		entry.ExpiresAt = time.Now().Add(duration)
	}

	m.allowList[ip] = entry

	// 从黑名单移除
	delete(m.denyList, ip)

	// 解除封禁
	if record, exists := m.records[ip]; exists {
		record.IsBanned = false
		record.BanReason = ""
		record.BanExpiry = time.Time{}
	}

	m.logger.Info("IP 已添加到白名单", zap.String("ip", ip))
	return nil
}

// RemoveFromAllowList 从白名单移除
func (m *Manager) RemoveFromAllowList(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.allowList, ip)
	m.logger.Info("IP 已从白名单移除", zap.String("ip", ip))
}

// AddToDenyList 添加到黑名单
func (m *Manager) AddToDenyList(ip string, reason BanReason, comment string, duration time.Duration) error {
	if err := validateIP(ip); err != nil {
		return fmt.Errorf("无效的 IP 地址: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 不允许封禁白名单 IP
	if _, exists := m.allowList[ip]; exists {
		return fmt.Errorf("IP %s 在白名单中，无法加入黑名单", ip)
	}

	entry := &DenyListEntry{
		IP:       ip,
		IsIPv6:   isIPv6(ip),
		Reason:   reason,
		Comment:  comment,
		AddedAt:  time.Now(),
		IsActive: true,
	}
	if duration > 0 {
		entry.ExpiresAt = time.Now().Add(duration)
	}

	m.denyList[ip] = entry

	// 更新记录
	m.getOrCreateRecord(ip).IsBanned = true
	m.getOrCreateRecord(ip).BanReason = reason
	if duration > 0 {
		m.getOrCreateRecord(ip).BanExpiry = time.Now().Add(duration)
	}

	m.logger.Warn("IP 已添加到黑名单",
		zap.String("ip", ip),
		zap.String("reason", string(reason)),
	)
	return nil
}

// RemoveFromDenyList 从黑名单移除
func (m *Manager) RemoveFromDenyList(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.denyList, ip)
	m.unbanIPUnsafe(ip)

	m.logger.Info("IP 已从黑名单移除", zap.String("ip", ip))
}

// GetAllowList 获取白名单
func (m *Manager) GetAllowList() []*AllowListEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AllowListEntry, 0, len(m.allowList))
	for _, entry := range m.allowList {
		result = append(result, entry)
	}
	return result
}

// GetDenyList 获取黑名单
func (m *Manager) GetDenyList() []*DenyListEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*DenyListEntry, 0, len(m.denyList))
	for _, entry := range m.denyList {
		result = append(result, entry)
	}
	return result
}

// ==================== IP 统计与查询 ====================

// GetIPStats 获取 IP 统计信息
func (m *Manager) GetIPStats(ip string) *IPStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, exists := m.records[ip]
	if !exists {
		return &IPStats{
			IP:              ip,
			ReputationScore: m.config.InitialReputationScore,
			ThreatLevel:     ThreatLevelLow,
		}
	}

	return &IPStats{
		IP:              record.IPString,
		ReputationScore: record.ReputationScore,
		ThreatLevel:     m.calcThreatLevel(record.ReputationScore),
		IsBanned:        record.IsBanned,
		BanReason:       record.BanReason,
		BanExpiry:       record.BanExpiry,
		BanCount:        record.BanCount,
		TotalRequests:   record.TotalRequests,
		FirstSeen:       record.FirstSeen,
		LastSeen:        record.LastSeen,
		RecentFailures:  m.detector.GetRecentLoginFailures(ip),
		RecentPorts:     m.detector.GetRecentPortCount(ip),
	}
}

// GetGlobalStats 获取全局统计
func (m *Manager) GetGlobalStats() *GlobalStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &GlobalStats{
		TotalIPsTracked: len(m.records),
		WhitelistedIPs:  len(m.allowList),
		BlacklistedIPs:  len(m.denyList),
		TotalBans:       len(m.banLog),
		LastUpdated:     time.Now(),
	}

	totalScore := 0
	for _, record := range m.records {
		totalScore += record.ReputationScore
		if record.IsBanned && time.Now().Before(record.BanExpiry) {
			stats.ActiveBans++
		}
		if record.ReputationScore < m.config.MinReputationScore {
			stats.LowReputationIPs++
		}
	}

	if stats.TotalIPsTracked > 0 {
		stats.AvgReputation = float64(totalScore) / float64(stats.TotalIPsTracked)
	}

	return stats
}

// GetBanLog 获取封禁日志
func (m *Manager) GetBanLog(limit int) []*BanRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.banLog) {
		limit = len(m.banLog)
	}

	// 返回最新的记录
	start := len(m.banLog) - limit
	result := make([]*BanRecord, limit)
	copy(result, m.banLog[start:])
	return result
}

// ==================== 内部方法 ====================

// onLoginFailure 处理登录失败
func (m *Manager) onLoginFailure(ip string, attempt *LoginAttempt) {
	// 扣除信誉分
	m.adjustReputation(ip, -m.config.LoginFailurePenalty)

	// 检测是否达到封禁阈值
	triggered, count, threshold := m.detector.DetectLoginFailure(ip)
	if triggered && m.config.EnableAutoBan {
		detail := fmt.Sprintf("登录失败 %d 次，阈值 %d", count, threshold)
		m.banIP(ip, BanReasonLoginFailure, m.config.AutoBanDuration, detail)
	}
}

// onLoginSuccess 处理登录成功
func (m *Manager) onLoginSuccess(ip string) {
	// 恢复少量信誉分
	m.adjustReputation(ip, 1)
}

// onRateLimitExceeded 处理频率超限
func (m *Manager) onRateLimitExceeded(ip string) {
	m.adjustReputation(ip, -m.config.RateLimitPenalty)

	record := m.getOrCreateRecord(ip)
	record.TotalRequests++
}

// onPortScanDetected 处理端口扫描检测
func (m *Manager) onPortScanDetected(ip string, result *DetectionResult) {
	m.adjustReputation(ip, -m.config.ScanPenalty)

	// 信誉分过低直接封禁
	record := m.getOrCreateRecord(ip)
	if record.ReputationScore < m.config.MinReputationScore {
		m.banIP(ip, BanReasonPortScan, m.config.AutoBanDuration*2, result.Details)
	} else {
		m.banIP(ip, BanReasonPortScan, m.config.AutoBanDuration, result.Details)
	}
}

// onBruteForceDetected 处理暴力破解检测
func (m *Manager) onBruteForceDetected(ip string, result *DetectionResult) {
	m.banIP(ip, BanReasonBruteForce, m.config.AutoBanDuration*3, result.Details)
}

// onSuspiciousActivity 处理可疑活动
func (m *Manager) onSuspiciousActivity(ip string, result *DetectionResult) {
	m.adjustReputation(ip, -20)

	record := m.getOrCreateRecord(ip)
	if record.ReputationScore < m.config.MinReputationScore {
		m.banIP(ip, BanReasonSuspicious, m.config.AutoBanDuration, result.Details)
	}
}

// banIP 封禁 IP
func (m *Manager) banIP(ip string, reason BanReason, duration time.Duration, details string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 白名单不封禁
	if _, exists := m.allowList[ip]; exists {
		return
	}

	record := m.getOrCreateRecord(ip)
	record.IsBanned = true
	record.BanReason = reason
	record.BanExpiry = time.Now().Add(duration)
	record.BanCount++

	// 记录封禁日志
	banRecord := &BanRecord{
		ID:        generateBanID(),
		IP:        ip,
		IsIPv6:    isIPv6(ip),
		Reason:    reason,
		Duration:  duration,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(duration),
		IsActive:  true,
		Details:   details,
	}
	m.banLog = append(m.banLog, banRecord)

	m.logger.Warn("IP 已被封禁",
		zap.String("ip", ip),
		zap.String("reason", string(reason)),
		zap.Duration("duration", duration),
		zap.String("details", details),
	)
}

// unbanIP 解除封禁（需要持锁）
func (m *Manager) unbanIP(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unbanIPUnsafe(ip)
}

// unbanIPUnsafe 解除封禁（不加锁，调用方需持锁）
func (m *Manager) unbanIPUnsafe(ip string) {
	if record, exists := m.records[ip]; exists {
		record.IsBanned = false
		record.BanReason = ""
		record.BanExpiry = time.Time{}
	}

	// 更新封禁日志
	for _, ban := range m.banLog {
		if ban.IP == ip && ban.IsActive {
			ban.IsActive = false
		}
	}
}

// adjustReputation 调整信誉分
func (m *Manager) adjustReputation(ip string, delta int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record := m.getOrCreateRecord(ip)
	record.ReputationScore += delta

	// 限制范围
	if record.ReputationScore > m.config.InitialReputationScore {
		record.ReputationScore = m.config.InitialReputationScore
	}
	if record.ReputationScore < 0 {
		record.ReputationScore = 0
	}
}

// touchRecord 更新 IP 记录
func (m *Manager) touchRecord(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record := m.getOrCreateRecord(ip)
	record.LastSeen = time.Now()
	record.TotalRequests++
}

// getOrCreateRecord 获取或创建 IP 记录（调用方需持锁）
func (m *Manager) getOrCreateRecord(ip string) *IPRecord {
	record, exists := m.records[ip]
	if !exists {
		now := time.Now()
		record = &IPRecord{
			IP:              net.ParseIP(ip),
			IPString:        ip,
			IsIPv6:          isIPv6(ip),
			ReputationScore: m.config.InitialReputationScore,
			FirstSeen:       now,
			LastSeen:        now,
		}
		m.records[ip] = record
	}
	return record
}

// isWhitelisted 检查是否在白名单
func (m *Manager) isWhitelisted(ip string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 精确匹配
	if entry, exists := m.allowList[ip]; exists && entry.IsActive {
		if entry.ExpiresAt.IsZero() || time.Now().Before(entry.ExpiresAt) {
			return true
		}
	}

	// CIDR 匹配
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}
	for _, entry := range m.allowList {
		if entry.Subnet != "" && entry.IsActive {
			_, cidr, err := net.ParseCIDR(entry.Subnet)
			if err == nil && cidr.Contains(parsedIP) {
				return true
			}
		}
	}

	return false
}

// isBlacklisted 检查是否在黑名单
func (m *Manager) isBlacklisted(ip string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 精确匹配
	if entry, exists := m.denyList[ip]; exists && entry.IsActive {
		if entry.ExpiresAt.IsZero() || time.Now().Before(entry.ExpiresAt) {
			return true
		}
	}

	// CIDR 匹配
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}
	for _, entry := range m.denyList {
		if entry.Subnet != "" && entry.IsActive {
			_, cidr, err := net.ParseCIDR(entry.Subnet)
			if err == nil && cidr.Contains(parsedIP) {
				return true
			}
		}
	}

	return false
}

// calcThreatLevel 计算威胁等级
func (m *Manager) calcThreatLevel(score int) ThreatLevel {
	if score >= 80 {
		return ThreatLevelLow
	} else if score >= 60 {
		return ThreatLevelMedium
	} else if score >= 40 {
		return ThreatLevelHigh
	}
	return ThreatLevelCritical
}

// reputationRecoveryLoop 信誉恢复循环
func (m *Manager) reputationRecoveryLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.recoverReputation()
		case <-m.stopChan:
			return
		}
	}
}

// recoverReputation 恢复信誉分
func (m *Manager) recoverReputation() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, record := range m.records {
		if record.ReputationScore < m.config.InitialReputationScore {
			record.ReputationScore += m.config.ReputationRecoverRate
			if record.ReputationScore > m.config.InitialReputationScore {
				record.ReputationScore = m.config.InitialReputationScore
			}
		}
	}
}

// ==================== 工具函数 ====================

// isIPv6 判断是否为 IPv6
func isIPv6(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.To4() == nil
}

// validateIP 验证 IP 地址
func validateIP(ip string) error {
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}
	return nil
}

// generateBanID 生成封禁 ID
func generateBanID() string {
	return fmt.Sprintf("ban-%d", time.Now().UnixNano())
}
