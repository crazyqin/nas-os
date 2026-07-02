package wanguard

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// ThreatType 威胁类型.
type ThreatType string

const (
	ThreatTypeSYNFlood   ThreatType = "syn_flood"
	ThreatTypeUDPFlood   ThreatType = "udp_flood"
	ThreatTypeICMPFlood  ThreatType = "icmp_flood"
	ThreatTypePortScan   ThreatType = "port_scan"
	ThreatTypeBruteForce ThreatType = "brute_force"
)

// IPStatus IP状态.
type IPStatus string

const (
	IPStatusWhitelisted IPStatus = "whitelisted"
	IPStatusBlacklisted IPStatus = "blacklisted"
	IPStatusNormal      IPStatus = "normal"
	IPStatusBanned      IPStatus = "banned"
)

// Config 配置.
type Config struct {
	// 检测阈值
	SYNFloodThreshold   int `json:"syn_flood_threshold"`  // SYN包阈值/秒
	UDPFloodThreshold   int `json:"udp_flood_threshold"`  // UDP包阈值/秒
	ICMPFloodThreshold  int `json:"icmp_flood_threshold"` // ICMP包阈值/秒
	ConnectionThreshold int `json:"connection_threshold"` // 连接数阈值

	// 自动封禁配置
	AutoBanEnabled bool          `json:"auto_ban_enabled"`
	BanDuration    time.Duration `json:"ban_duration"`
	MaxThreatScore int           `json:"max_threat_score"` // 自动封禁阈值

	// 速率限制
	RateLimitEnabled    bool `json:"rate_limit_enabled"`
	MaxConnectionsPerIP int  `json:"max_connections_per_ip"`

	// 窗口大小
	WindowSize time.Duration `json:"window_size"`
}

// DefaultConfig 默认配置.
func DefaultConfig() Config {
	return Config{
		SYNFloodThreshold:   1000,
		UDPFloodThreshold:   5000,
		ICMPFloodThreshold:  500,
		ConnectionThreshold: 100,
		AutoBanEnabled:      true,
		BanDuration:         30 * time.Minute,
		MaxThreatScore:      100,
		RateLimitEnabled:    true,
		MaxConnectionsPerIP: 50,
		WindowSize:          1 * time.Minute,
	}
}

// IPEntry IP条目.
type IPEntry struct {
	IP       net.IP     `json:"ip"`
	Status   IPStatus   `json:"status"`
	Reason   string     `json:"reason,omitempty"`
	BannedAt *time.Time `json:"banned_at,omitempty"`
	BanUntil *time.Time `json:"ban_until,omitempty"`
	HitCount int64      `json:"hit_count"`
	LastSeen time.Time  `json:"last_seen"`
}

// ThreatRecord 威胁记录.
type ThreatRecord struct {
	IP        net.IP     `json:"ip"`
	Type      ThreatType `json:"type"`
	Score     int        `json:"score"`
	Count     int64      `json:"count"`
	FirstSeen time.Time  `json:"first_seen"`
	LastSeen  time.Time  `json:"last_seen"`
	Details   string     `json:"details,omitempty"`
}

// TrafficStats 流量统计.
type TrafficStats struct {
	TotalPackets   int64            `json:"total_packets"`
	TotalBytes     int64            `json:"total_bytes"`
	SYNPackets     int64            `json:"syn_packets"`
	UDPPackets     int64            `json:"udp_packets"`
	ICMPPackets    int64            `json:"icmp_packets"`
	TCPPackets     int64            `json:"tcp_packets"`
	BlockedPackets int64            `json:"blocked_packets"`
	UniqueIPs      int              `json:"unique_ips"`
	TopSources     map[string]int64 `json:"top_sources"`
	WindowStart    time.Time        `json:"window_start"`
	WindowEnd      time.Time        `json:"window_end"`
}

// RateLimitRule 速率限制规则.
type RateLimitRule struct {
	Name       string        `json:"name"`
	Protocol   string        `json:"protocol"`
	MaxPackets int           `json:"max_packets"`
	MaxBytes   int64         `json:"max_bytes"`
	Window     time.Duration `json:"window"`
	Enabled    bool          `json:"enabled"`
}

// PacketInfo 数据包信息.
type PacketInfo struct {
	SrcIP     net.IP
	DstIP     net.IP
	SrcPort   uint16
	DstPort   uint16
	Protocol  string // tcp, udp, icmp
	Size      int
	IsSYN     bool
	IsACK     bool
	IsFIN     bool
	Timestamp time.Time
}

// Manager 防护管理器.
type Manager struct {
	mu              sync.RWMutex
	config          Config
	blacklist       map[string]*IPEntry
	whitelist       map[string]*IPEntry
	threatRecords   map[string][]*ThreatRecord
	trafficStats    *TrafficStats
	packetWindow    map[string][]time.Time // IP -> 时间戳列表
	connectionCount map[string]int         // IP -> 当前连接数
	bannedIPs       map[string]*IPEntry
	startTime       time.Time
	stopChan        chan struct{}
}

// NewManager 创建防护管理器.
func NewManager(config Config) *Manager {
	m := &Manager{
		config:        config,
		blacklist:     make(map[string]*IPEntry),
		whitelist:     make(map[string]*IPEntry),
		threatRecords: make(map[string][]*ThreatRecord),
		trafficStats: &TrafficStats{
			TopSources:  make(map[string]int64),
			WindowStart: time.Now(),
		},
		packetWindow:    make(map[string][]time.Time),
		connectionCount: make(map[string]int),
		bannedIPs:       make(map[string]*IPEntry),
		startTime:       time.Now(),
		stopChan:        make(chan struct{}),
	}

	// 启动后台清理任务
	go m.cleanupRoutine()

	return m
}

// CheckPacket 检查数据包.
func (m *Manager) CheckPacket(pkt PacketInfo) (allowed bool, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 更新流量统计
	m.updateStats(pkt)

	srcIP := pkt.SrcIP.String()
	now := pkt.Timestamp

	// 检查白名单
	if entry, exists := m.whitelist[srcIP]; exists {
		entry.HitCount++
		entry.LastSeen = now
		return true, "whitelisted"
	}

	// 检查黑名单
	if entry, exists := m.blacklist[srcIP]; exists {
		entry.HitCount++
		entry.LastSeen = now
		m.trafficStats.BlockedPackets++
		return false, "blacklisted"
	}

	// 检查自动封禁
	if entry, exists := m.bannedIPs[srcIP]; exists {
		if entry.BanUntil != nil && now.After(*entry.BanUntil) {
			// 封禁已过期，移除
			delete(m.bannedIPs, srcIP)
		} else {
			entry.HitCount++
			entry.LastSeen = now
			m.trafficStats.BlockedPackets++
			return false, "auto_banned"
		}
	}

	// 速率限制检查
	if m.config.RateLimitEnabled {
		if !m.checkRateLimit(srcIP, now) {
			m.addThreatRecord(pkt.SrcIP, ThreatTypeBruteForce, 10, "rate_limit_exceeded")
			return false, "rate_limited"
		}
	}

	// DDoS检测
	threat := m.detectThreat(pkt)
	if threat != nil {
		m.addThreatRecord(pkt.SrcIP, threat.Type, threat.Score, threat.Details)

		// 自动封禁
		if m.config.AutoBanEnabled {
			m.autoBanIfNeeded(srcIP, now)
		}

		m.trafficStats.BlockedPackets++
		return false, fmt.Sprintf("threat_detected:%s", threat.Type)
	}

	// 更新连接计数
	m.connectionCount[srcIP]++

	return true, "allowed"
}

// updateStats 更新流量统计.
func (m *Manager) updateStats(pkt PacketInfo) {
	m.trafficStats.TotalPackets++
	m.trafficStats.TotalBytes += int64(pkt.Size)

	srcIP := pkt.SrcIP.String()
	m.trafficStats.TopSources[srcIP]++

	switch pkt.Protocol {
	case "tcp":
		m.trafficStats.TCPPackets++
		if pkt.IsSYN {
			m.trafficStats.SYNPackets++
		}
	case "udp":
		m.trafficStats.UDPPackets++
	case "icmp":
		m.trafficStats.ICMPPackets++
	}

	// 更新窗口时间
	m.trafficStats.WindowEnd = pkt.Timestamp
}

// checkRateLimit 检查速率限制.
func (m *Manager) checkRateLimit(ip string, now time.Time) bool {
	windowStart := now.Add(-m.config.WindowSize)

	// 清理过期记录
	timestamps := m.packetWindow[ip]
	validTimestamps := make([]time.Time, 0, len(timestamps))
	for _, ts := range timestamps {
		if ts.After(windowStart) {
			validTimestamps = append(validTimestamps, ts)
		}
	}

	// 检查限制
	if len(validTimestamps) >= m.config.MaxConnectionsPerIP {
		return false
	}

	// 添加当前时间戳
	m.packetWindow[ip] = append(validTimestamps, now)
	return true
}

// detectThreat 检测威胁.
func (m *Manager) detectThreat(pkt PacketInfo) *ThreatRecord {
	srcIP := pkt.SrcIP.String()
	now := pkt.Timestamp
	windowStart := now.Add(-m.config.WindowSize)

	// SYN Flood检测
	if pkt.Protocol == "tcp" && pkt.IsSYN {
		synCount := m.countPacketsInWindow(srcIP, "syn", windowStart, now)
		if synCount >= int64(m.config.SYNFloodThreshold) {
			return &ThreatRecord{
				IP:        pkt.SrcIP,
				Type:      ThreatTypeSYNFlood,
				Score:     30,
				Count:     synCount,
				FirstSeen: windowStart,
				LastSeen:  now,
				Details:   fmt.Sprintf("SYN count: %d", synCount),
			}
		}
	}

	// UDP Flood检测
	if pkt.Protocol == "udp" {
		udpCount := m.countPacketsInWindow(srcIP, "udp", windowStart, now)
		if udpCount >= int64(m.config.UDPFloodThreshold) {
			return &ThreatRecord{
				IP:        pkt.SrcIP,
				Type:      ThreatTypeUDPFlood,
				Score:     40,
				Count:     udpCount,
				FirstSeen: windowStart,
				LastSeen:  now,
				Details:   fmt.Sprintf("UDP count: %d", udpCount),
			}
		}
	}

	// ICMP Flood检测
	if pkt.Protocol == "icmp" {
		icmpCount := m.countPacketsInWindow(srcIP, "icmp", windowStart, now)
		if icmpCount >= int64(m.config.ICMPFloodThreshold) {
			return &ThreatRecord{
				IP:        pkt.SrcIP,
				Type:      ThreatTypeICMPFlood,
				Score:     25,
				Count:     icmpCount,
				FirstSeen: windowStart,
				LastSeen:  now,
				Details:   fmt.Sprintf("ICMP count: %d", icmpCount),
			}
		}
	}

	// 连接数检测
	if m.connectionCount[srcIP] >= m.config.ConnectionThreshold {
		return &ThreatRecord{
			IP:        pkt.SrcIP,
			Type:      ThreatTypeBruteForce,
			Score:     20,
			Count:     int64(m.connectionCount[srcIP]),
			FirstSeen: windowStart,
			LastSeen:  now,
			Details:   fmt.Sprintf("Connection count: %d", m.connectionCount[srcIP]),
		}
	}

	return nil
}

// countPacketsInWindow 统计窗口内特定类型的包.
func (m *Manager) countPacketsInWindow(ip, packetType string, start, end time.Time) int64 {
	// 简化实现：使用威胁记录来统计
	key := fmt.Sprintf("%s:%s", ip, packetType)
	if records, exists := m.threatRecords[key]; exists {
		var count int64
		for _, r := range records {
			if r.LastSeen.After(start) && r.LastSeen.Before(end) {
				count += r.Count
			}
		}
		return count
	}
	return 0
}

// addThreatRecord 添加威胁记录.
func (m *Manager) addThreatRecord(ip net.IP, threatType ThreatType, score int, details string) {
	key := fmt.Sprintf("%s:%s", ip.String(), threatType)
	now := time.Now()

	record := &ThreatRecord{
		IP:        ip,
		Type:      threatType,
		Score:     score,
		Count:     1,
		FirstSeen: now,
		LastSeen:  now,
		Details:   details,
	}

	if records, exists := m.threatRecords[key]; exists {
		// 更新现有记录
		if len(records) > 0 {
			last := records[len(records)-1]
			if last.Type == threatType {
				last.Count++
				last.LastSeen = now
				last.Score += score
				return
			}
		}
		m.threatRecords[key] = append(records, record)
	} else {
		m.threatRecords[key] = []*ThreatRecord{record}
	}
}

// autoBanIfNeeded 自动封禁.
func (m *Manager) autoBanIfNeeded(ip string, now time.Time) {
	// 计算总威胁分数
	totalScore := m.getTotalThreatScore(ip)

	if totalScore >= m.config.MaxThreatScore {
		banUntil := now.Add(m.config.BanDuration)
		m.bannedIPs[ip] = &IPEntry{
			IP:       net.ParseIP(ip),
			Status:   IPStatusBanned,
			Reason:   fmt.Sprintf("auto_banned: threat score %d", totalScore),
			BannedAt: &now,
			BanUntil: &banUntil,
			LastSeen: now,
		}
	}
}

// getTotalThreatScore 获取总威胁分数.
func (m *Manager) getTotalThreatScore(ip string) int {
	totalScore := 0
	for key, records := range m.threatRecords {
		if len(key) > len(ip) && key[:len(ip)] == ip {
			for _, r := range records {
				totalScore += r.Score
			}
		}
	}
	return totalScore
}

// AddToBlacklist 添加到黑名单.
func (m *Manager) AddToBlacklist(ip net.IP, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ipStr := ip.String()

	// 检查是否在白名单中
	if _, exists := m.whitelist[ipStr]; exists {
		return fmt.Errorf("IP %s is whitelisted, remove from whitelist first", ipStr)
	}

	m.blacklist[ipStr] = &IPEntry{
		IP:       ip,
		Status:   IPStatusBlacklisted,
		Reason:   reason,
		LastSeen: time.Now(),
	}

	// 从封禁列表中移除（如果有）
	delete(m.bannedIPs, ipStr)

	return nil
}

// RemoveFromBlacklist 从黑名单移除.
func (m *Manager) RemoveFromBlacklist(ip net.IP) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ipStr := ip.String()
	if _, exists := m.blacklist[ipStr]; !exists {
		return fmt.Errorf("IP %s is not in blacklist", ipStr)
	}

	delete(m.blacklist, ipStr)
	return nil
}

// AddToWhitelist 添加到白名单.
func (m *Manager) AddToWhitelist(ip net.IP, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ipStr := ip.String()

	// 检查是否在黑名单中
	if _, exists := m.blacklist[ipStr]; exists {
		return fmt.Errorf("IP %s is blacklisted, remove from blacklist first", ipStr)
	}

	m.whitelist[ipStr] = &IPEntry{
		IP:       ip,
		Status:   IPStatusWhitelisted,
		Reason:   reason,
		LastSeen: time.Now(),
	}

	// 从封禁列表中移除（如果有）
	delete(m.bannedIPs, ipStr)

	return nil
}

// RemoveFromWhitelist 从白名单移除.
func (m *Manager) RemoveFromWhitelist(ip net.IP) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ipStr := ip.String()
	if _, exists := m.whitelist[ipStr]; !exists {
		return fmt.Errorf("IP %s is not in whitelist", ipStr)
	}

	delete(m.whitelist, ipStr)
	return nil
}

// GetTrafficStats 获取流量统计.
func (m *Manager) GetTrafficStats() TrafficStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 创建副本
	stats := TrafficStats{
		TotalPackets:   m.trafficStats.TotalPackets,
		TotalBytes:     m.trafficStats.TotalBytes,
		SYNPackets:     m.trafficStats.SYNPackets,
		UDPPackets:     m.trafficStats.UDPPackets,
		ICMPPackets:    m.trafficStats.ICMPPackets,
		TCPPackets:     m.trafficStats.TCPPackets,
		BlockedPackets: m.trafficStats.BlockedPackets,
		UniqueIPs:      len(m.trafficStats.TopSources),
		TopSources:     make(map[string]int64),
		WindowStart:    m.trafficStats.WindowStart,
		WindowEnd:      m.trafficStats.WindowEnd,
	}

	// 复制TopSources
	for ip, count := range m.trafficStats.TopSources {
		stats.TopSources[ip] = count
	}

	return stats
}

// DetectAnomaly 异常检测.
func (m *Manager) DetectAnomaly() []ThreatRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	anomalies := make([]ThreatRecord, 0)

	// 检查流量突增
	stats := m.trafficStats
	if stats.TotalPackets > 0 {
		duration := stats.WindowEnd.Sub(stats.WindowStart).Seconds()
		if duration > 0 {
			pps := float64(stats.TotalPackets) / duration
			if pps > float64(m.config.SYNFloodThreshold) {
				anomalies = append(anomalies, ThreatRecord{
					Type:    ThreatTypeSYNFlood,
					Score:   50,
					Details: fmt.Sprintf("High packet rate: %.2f pps", pps),
				})
			}
		}
	}

	// 检查异常IP
	for ip, count := range m.connectionCount {
		if count > m.config.ConnectionThreshold {
			anomalies = append(anomalies, ThreatRecord{
				IP:      net.ParseIP(ip),
				Type:    ThreatTypeBruteForce,
				Score:   20,
				Count:   int64(count),
				Details: fmt.Sprintf("High connection count: %d", count),
			})
		}
	}

	// 检查威胁记录中的异常
	for _, records := range m.threatRecords {
		for _, record := range records {
			if record.Score >= m.config.MaxThreatScore {
				anomalies = append(anomalies, *record)
			}
		}
	}

	return anomalies
}

// AutoBan 自动封禁检查并执行.
func (m *Manager) AutoBan() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	bannedIPs := make([]string, 0)
	now := time.Now()

	// 检查所有IP的威胁分数
	checkedIPs := make(map[string]bool)

	for key, records := range m.threatRecords {
		// 提取IP地址
		parts := splitKey(key)
		if len(parts) < 1 {
			continue
		}
		ip := parts[0]

		if checkedIPs[ip] {
			continue
		}
		checkedIPs[ip] = true

		// 跳过白名单
		if _, exists := m.whitelist[ip]; exists {
			continue
		}

		// 跳过已封禁
		if _, exists := m.bannedIPs[ip]; exists {
			continue
		}

		// 计算威胁分数
		totalScore := 0
		for _, r := range records {
			totalScore += r.Score
		}

		if totalScore >= m.config.MaxThreatScore {
			banUntil := now.Add(m.config.BanDuration)
			m.bannedIPs[ip] = &IPEntry{
				IP:       net.ParseIP(ip),
				Status:   IPStatusBanned,
				Reason:   fmt.Sprintf("auto_banned: threat score %d", totalScore),
				BannedAt: &now,
				BanUntil: &banUntil,
				LastSeen: now,
			}
			bannedIPs = append(bannedIPs, ip)
		}
	}

	return bannedIPs
}

// splitKey 分割威胁记录的key.
func splitKey(key string) []string {
	// key格式: "ip:threatType"
	for i, c := range key {
		if c == ':' {
			return []string{key[:i], key[i+1:]}
		}
	}
	return []string{key}
}

// GetBlacklist 获取黑名单.
func (m *Manager) GetBlacklist() []IPEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]IPEntry, 0, len(m.blacklist))
	for _, entry := range m.blacklist {
		entries = append(entries, *entry)
	}
	return entries
}

// GetWhitelist 获取白名单.
func (m *Manager) GetWhitelist() []IPEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]IPEntry, 0, len(m.whitelist))
	for _, entry := range m.whitelist {
		entries = append(entries, *entry)
	}
	return entries
}

// GetBannedIPs 获取封禁IP列表.
func (m *Manager) GetBannedIPs() []IPEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]IPEntry, 0, len(m.bannedIPs))
	for _, entry := range m.bannedIPs {
		entries = append(entries, *entry)
	}
	return entries
}

// GetThreatRecords 获取威胁记录.
func (m *Manager) GetThreatRecords() []*ThreatRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := make([]*ThreatRecord, 0)
	for _, recordList := range m.threatRecords {
		records = append(records, recordList...)
	}
	return records
}

// GetIPStatus 获取IP状态.
func (m *Manager) GetIPStatus(ip net.IP) IPStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ipStr := ip.String()

	if _, exists := m.whitelist[ipStr]; exists {
		return IPStatusWhitelisted
	}
	if _, exists := m.blacklist[ipStr]; exists {
		return IPStatusBlacklisted
	}
	if _, exists := m.bannedIPs[ipStr]; exists {
		return IPStatusBanned
	}

	return IPStatusNormal
}

// IsBlocked 检查IP是否被阻断.
func (m *Manager) IsBlocked(ip net.IP) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ipStr := ip.String()

	// 白名单不阻断
	if _, exists := m.whitelist[ipStr]; exists {
		return false
	}

	// 黑名单阻断
	if _, exists := m.blacklist[ipStr]; exists {
		return true
	}

	// 封禁IP阻断
	if entry, exists := m.bannedIPs[ipStr]; exists {
		if entry.BanUntil != nil && time.Now().After(*entry.BanUntil) {
			return false
		}
		return true
	}

	return false
}

// cleanupRoutine 清理过期数据.
func (m *Manager) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup()
		case <-m.stopChan:
			return
		}
	}
}

// cleanup 清理过期数据.
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 清理过期的封禁IP
	for ip, entry := range m.bannedIPs {
		if entry.BanUntil != nil && now.After(*entry.BanUntil) {
			delete(m.bannedIPs, ip)
		}
	}

	// 清理过期的速率限制记录
	windowStart := now.Add(-m.config.WindowSize)
	for ip, timestamps := range m.packetWindow {
		validTimestamps := make([]time.Time, 0)
		for _, ts := range timestamps {
			if ts.After(windowStart) {
				validTimestamps = append(validTimestamps, ts)
			}
		}
		if len(validTimestamps) == 0 {
			delete(m.packetWindow, ip)
		} else {
			m.packetWindow[ip] = validTimestamps
		}
	}

	// 重置连接计数
	m.connectionCount = make(map[string]int)
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	close(m.stopChan)
}

// Reset 重置统计.
func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.trafficStats = &TrafficStats{
		TopSources:  make(map[string]int64),
		WindowStart: time.Now(),
	}
	m.packetWindow = make(map[string][]time.Time)
	m.connectionCount = make(map[string]int)
	m.threatRecords = make(map[string][]*ThreatRecord)
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(config Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// AddRateLimitRule 添加速率限制规则.
func (m *Manager) AddRateLimitRule(rule RateLimitRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// 这里可以扩展为存储规则列表
	// 目前简化实现
}

// GetConnectionCount 获取连接数.
func (m *Manager) GetConnectionCount(ip net.IP) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connectionCount[ip.String()]
}

// GetActiveThreats 获取活跃威胁.
func (m *Manager) GetActiveThreats() []ThreatRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	active := make([]ThreatRecord, 0)
	now := time.Now()
	windowStart := now.Add(-m.config.WindowSize)

	for _, records := range m.threatRecords {
		for _, record := range records {
			if record.LastSeen.After(windowStart) {
				active = append(active, *record)
			}
		}
	}

	return active
}

// String 字符串表示.
func (m *Manager) String() string {
	stats := m.GetTrafficStats()
	return fmt.Sprintf("WanGuard[packets=%d, blocked=%d, blacklist=%d, whitelist=%d, banned=%d]",
		stats.TotalPackets,
		stats.BlockedPackets,
		len(m.blacklist),
		len(m.whitelist),
		len(m.bannedIPs),
	)
}
