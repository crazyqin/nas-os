// Package syslogserver 提供日志集中管理核心业务逻辑
package syslogserver

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 日志管理器.
type Manager struct {
	entries         []*SyslogEntry
	forwardTargets  map[string]*ForwardTarget
	alertRules      map[string]*AlertRule
	alertEvents     []*AlertEvent
	archivePolicies map[string]*ArchivePolicy
	wsClients       map[string]*WSClient

	// 统计计数器
	statsMu         sync.RWMutex
	statsBySeverity map[SyslogSeverity]int64
	statsByFacility map[SyslogFacility]int64
	statsByHost     map[string]int64
	statsByApp      map[string]int64
	entriesToday    int64
	todayDate       string

	mu sync.RWMutex

	// syslog 服务器
	udpListener *net.UDPConn
	tcpListener *net.TCPListener

	// 控制通道
	stopCh chan struct{}
}

// NewManager 创建日志管理器.
func NewManager() *Manager {
	now := time.Now()
	return &Manager{
		entries:         make([]*SyslogEntry, 0),
		forwardTargets:  make(map[string]*ForwardTarget),
		alertRules:      make(map[string]*AlertRule),
		alertEvents:     make([]*AlertEvent, 0),
		archivePolicies: make(map[string]*ArchivePolicy),
		wsClients:       make(map[string]*WSClient),
		statsBySeverity: make(map[SyslogSeverity]int64),
		statsByFacility: make(map[SyslogFacility]int64),
		statsByHost:     make(map[string]int64),
		statsByApp:      make(map[string]int64),
		todayDate:       now.Format("2006-01-02"),
		stopCh:          make(chan struct{}),
	}
}

// StartSyslogServer 启动 syslog 服务器 (UDP/TCP 514).
func (m *Manager) StartSyslogServer() error {
	// 启动 UDP 服务器
	udpAddr, err := net.ResolveUDPAddr("udp", ":514")
	if err != nil {
		return fmt.Errorf("resolve udp addr: %w", err)
	}
	m.udpListener, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("listen udp: %w", err)
	}
	go m.serveUDP()

	// 启动 TCP 服务器
	tcpAddr, err := net.ResolveTCPAddr("tcp", ":514")
	if err != nil {
		return fmt.Errorf("resolve tcp addr: %w", err)
	}
	m.tcpListener, err = net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return fmt.Errorf("listen tcp: %w", err)
	}
	go m.serveTCP()

	// 启动归档清理定时器
	go m.archiveCleanupLoop()

	// 启动告警检查定时器
	go m.alertCheckLoop()

	log.Println("[syslogserver] 已启动 syslog 服务器 (UDP/TCP :514)")
	return nil
}

// StopSyslogServer 停止 syslog 服务器.
func (m *Manager) StopSyslogServer() {
	close(m.stopCh)
	if m.udpListener != nil {
		m.udpListener.Close()
	}
	if m.tcpListener != nil {
		m.tcpListener.Close()
	}
	log.Println("[syslogserver] 已停止 syslog 服务器")
}

// serveUDP 处理 UDP 请求.
func (m *Manager) serveUDP() {
	buf := make([]byte, 65536)
	for {
		select {
		case <-m.stopCh:
			return
		default:
		}

		n, addr, err := m.udpListener.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-m.stopCh:
				return
			default:
				log.Printf("[syslogserver] UDP 读取错误: %v", err)
				continue
			}
		}

		raw := strings.TrimSpace(string(buf[:n]))
		if raw == "" {
			continue
		}

		entry := m.parseSyslogMessage(raw, addr.IP.String(), "udp")
		m.processEntry(entry)
	}
}

// serveTCP 处理 TCP 请求.
func (m *Manager) serveTCP() {
	for {
		select {
		case <-m.stopCh:
			return
		default:
		}

		conn, err := m.tcpListener.AcceptTCP()
		if err != nil {
			select {
			case <-m.stopCh:
				return
			default:
				log.Printf("[syslogserver] TCP 接受错误: %v", err)
				continue
			}
		}

		go m.handleTCPConn(conn)
	}
}

// handleTCPConn 处理单个 TCP 连接.
func (m *Manager) handleTCPConn(conn *net.TCPConn) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().(*net.TCPAddr).IP.String()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}

		entry := m.parseSyslogMessage(raw, remoteAddr, "tcp")
		m.processEntry(entry)
	}
}

// parseSyslogMessage 解析 syslog 消息 (RFC 3164 / RFC 5424).
func (m *Manager) parseSyslogMessage(raw, sourceIP, protocol string) *SyslogEntry {
	entry := &SyslogEntry{
		ID:         uuid.New().String(),
		Raw:        raw,
		SourceIP:   sourceIP,
		Protocol:   protocol,
		ReceivedAt: time.Now(),
		Tags:       []string{},
	}

	// RFC 5424 格式: <priority>version timestamp hostname app-name procid msgid structured-data msg
	rfc5424Regex := regexp.MustCompile(`^<(\d+)>(\d+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(.*)$`)
	if matches := rfc5424Regex.FindStringSubmatch(raw); len(matches) > 0 {
		priority, _ := strconv.Atoi(matches[1])
		entry.Priority = priority
		entry.Facility = SyslogFacility(priority / 8)
		entry.Severity = SyslogSeverity(priority % 8)
		entry.Timestamp = parseTimestamp(matches[3])
		entry.Hostname = matches[4]
		entry.AppName = matches[5]
		entry.ProcID = matches[6]
		entry.MsgID = matches[7]
		entry.StructuredData = matches[8]
		// 提取消息部分（在 structured-data 之后）
		parts := strings.SplitN(matches[8], " ", 2)
		if len(parts) > 1 {
			entry.Message = parts[1]
		}
		return entry
	}

	// RFC 3164 格式: <priority>timestamp hostname app[pid]: message
	rfc3164Regex := regexp.MustCompile(`^<(\d+)>(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+(\S+)\s+(\S+?)(?:\[(\d+)\])?:\s*(.*)$`)
	if matches := rfc3164Regex.FindStringSubmatch(raw); len(matches) > 0 {
		priority, _ := strconv.Atoi(matches[1])
		entry.Priority = priority
		entry.Facility = SyslogFacility(priority / 8)
		entry.Severity = SyslogSeverity(priority % 8)
		entry.Timestamp = parseTimestamp(matches[2])
		entry.Hostname = matches[3]
		entry.AppName = matches[4]
		entry.ProcID = matches[5]
		entry.Message = matches[6]
		return entry
	}

	// 简单格式: <priority>message
	simpleRegex := regexp.MustCompile(`^<(\d+)>(.*)$`)
	if matches := simpleRegex.FindStringSubmatch(raw); len(matches) > 0 {
		priority, _ := strconv.Atoi(matches[1])
		entry.Priority = priority
		entry.Facility = SyslogFacility(priority / 8)
		entry.Severity = SyslogSeverity(priority % 8)
		entry.Timestamp = time.Now()
		entry.Hostname = sourceIP
		entry.Message = matches[2]
		return entry
	}

	// 无法解析，作为普通消息
	entry.Priority = 134 // facility=16 (local0), severity=6 (informational)
	entry.Facility = FacilityLocal0
	entry.Severity = SeverityInformational
	entry.Timestamp = time.Now()
	entry.Hostname = sourceIP
	entry.Message = raw
	return entry
}

// parseTimestamp 解析时间戳.
func parseTimestamp(s string) time.Time {
	// RFC 5424 格式
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"Jan  2 15:04:05",
		"Jan 02 15:04:05",
		"2006-01-02T15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t
		}
	}

	return time.Now()
}

// processEntry 处理日志条目.
func (m *Manager) processEntry(entry *SyslogEntry) {
	// 存储条目
	m.mu.Lock()
	m.entries = append(m.entries, entry)
	m.mu.Unlock()

	// 更新统计
	m.updateStats(entry)

	// 广播到 WebSocket 客户端
	m.broadcastEntry(entry)

	// 转发到远程目标
	m.forwardEntry(entry)

	// 检查告警规则
	m.checkAlerts(entry)
}

// updateStats 更新统计计数器.
func (m *Manager) updateStats(entry *SyslogEntry) {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()

	m.statsBySeverity[entry.Severity]++
	m.statsByFacility[entry.Facility]++
	m.statsByHost[entry.Hostname]++
	m.statsByApp[entry.AppName]++

	today := time.Now().Format("2006-01-02")
	if today != m.todayDate {
		m.todayDate = today
		m.entriesToday = 0
	}
	m.entriesToday++
}

// broadcastEntry 广播日志到 WebSocket 客户端.
func (m *Manager) broadcastEntry(entry *SyslogEntry) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	for _, client := range m.wsClients {
		// 检查过滤条件
		if client.Filter != nil && !m.matchFilter(entry, client.Filter) {
			continue
		}

		select {
		case client.Send <- data:
		default:
			// 客户端缓冲区满，跳过
		}
	}
}

// matchFilter 检查条目是否匹配过滤条件.
func (m *Manager) matchFilter(entry *SyslogEntry, filter *SearchRequest) bool {
	if filter.Hostname != "" && entry.Hostname != filter.Hostname {
		return false
	}
	if filter.AppName != "" && entry.AppName != filter.AppName {
		return false
	}
	if filter.Facility != "" {
		facilityName := FacilityNames[entry.Facility]
		if !strings.EqualFold(facilityName, filter.Facility) {
			return false
		}
	}
	if filter.Severity != "" {
		severityName := SeverityNames[entry.Severity]
		if !strings.EqualFold(severityName, filter.Severity) {
			return false
		}
	}
	if filter.SourceIP != "" && entry.SourceIP != filter.SourceIP {
		return false
	}
	if filter.Query != "" {
		query := strings.ToLower(filter.Query)
		if !strings.Contains(strings.ToLower(entry.Message), query) &&
			!strings.Contains(strings.ToLower(entry.AppName), query) &&
			!strings.Contains(strings.ToLower(entry.Hostname), query) {
			return false
		}
	}
	return true
}

// forwardEntry 转发日志到远程目标.
func (m *Manager) forwardEntry(entry *SyslogEntry) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, target := range m.forwardTargets {
		if !target.Enabled {
			continue
		}

		// 检查过滤条件
		if target.Filter != "" && !m.matchForwardFilter(entry, target.Filter) {
			continue
		}

		go m.sendToTarget(target, entry)
	}
}

// matchForwardFilter 检查转发过滤条件.
func (m *Manager) matchForwardFilter(entry *SyslogEntry, filter string) bool {
	// 格式: facility:severity
	parts := strings.SplitN(filter, ":", 2)
	if len(parts) == 2 {
		facilityName := FacilityNames[entry.Facility]
		severityName := SeverityNames[entry.Severity]
		if parts[0] != "" && !strings.EqualFold(facilityName, parts[0]) {
			return false
		}
		if parts[1] != "" && !strings.EqualFold(severityName, parts[1]) {
			return false
		}
	}
	return true
}

// sendToTarget 发送日志到目标服务器.
func (m *Manager) sendToTarget(target *ForwardTarget, entry *SyslogEntry) {
	addr := net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.Port))
	message := entry.Raw + "\n"

	if target.Protocol == "udp" {
		conn, err := net.Dial("udp", addr)
		if err != nil {
			log.Printf("[syslogserver] 转发到 %s 失败: %v", target.Name, err)
			return
		}
		defer conn.Close()
		conn.Write([]byte(message))
	} else {
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			log.Printf("[syslogserver] 转发到 %s 失败: %v", target.Name, err)
			return
		}
		defer conn.Close()
		conn.Write([]byte(message))
	}
}

// checkAlerts 检查告警规则.
func (m *Manager) checkAlerts(entry *SyslogEntry) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, rule := range m.alertRules {
		if !rule.Enabled {
			continue
		}

		if rule.Type == "keyword" && rule.Keyword != "" {
			if strings.Contains(strings.ToLower(entry.Message), strings.ToLower(rule.Keyword)) {
				m.triggerAlert(rule, entry)
			}
		}
	}
}

// triggerAlert 触发告警.
func (m *Manager) triggerAlert(rule *AlertRule, entry *SyslogEntry) {
	now := time.Now()
	alertEvent := &AlertEvent{
		ID:          uuid.New().String(),
		RuleID:      rule.ID,
		RuleName:    rule.Name,
		Message:     fmt.Sprintf("告警规则 [%s] 触发: %s", rule.Name, entry.Message),
		Entry:       entry,
		TriggeredAt: now,
	}

	m.alertEvents = append(m.alertEvents, alertEvent)
	rule.LastTrigger = &now
	rule.TriggerCount++

	log.Printf("[syslogserver] 告警触发: %s", alertEvent.Message)

	if rule.NotifyType == "webhook" && rule.WebhookURL != "" {
		go func() {
			body, err := json.Marshal(alertEvent)
			if err != nil {
				return
			}
			req, err := http.NewRequest(http.MethodPost, rule.WebhookURL, bytes.NewReader(body))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err == nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
		}()
	}
}

// archiveCleanupLoop 归档清理定时器.
func (m *Manager) archiveCleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.runArchiveCleanup()
		}
	}
}

// runArchiveCleanup 执行归档清理.
func (m *Manager) runArchiveCleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, policy := range m.archivePolicies {
		if !policy.Enabled {
			continue
		}

		cutoff := time.Now().AddDate(0, 0, -policy.MaxAgeDays)
		before := len(m.entries)

		// 按时间清理
		if policy.MaxAgeDays > 0 {
			filtered := make([]*SyslogEntry, 0)
			for _, entry := range m.entries {
				if entry.Timestamp.After(cutoff) {
					filtered = append(filtered, entry)
				}
			}
			m.entries = filtered
		}

		// 按大小清理（保留最新的 MaxSizeMB 数据）
		if policy.MaxSizeMB > 0 {
			maxEntries := policy.MaxSizeMB * 1000 // 粗略估算
			if len(m.entries) > maxEntries {
				m.entries = m.entries[len(m.entries)-maxEntries:]
			}
		}

		after := len(m.entries)
		if before != after {
			log.Printf("[syslogserver] 归档清理完成: 策略=%s, 清理 %d 条日志", policy.Name, before-after)
		}
	}
}

// alertCheckLoop 告警频率检查定时器.
func (m *Manager) alertCheckLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkFrequencyAlerts()
		}
	}
}

// checkFrequencyAlerts 检查频率告警.
func (m *Manager) checkFrequencyAlerts() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for _, rule := range m.alertRules {
		if !rule.Enabled || rule.Type != "frequency" || rule.Frequency <= 0 {
			continue
		}
		window := time.Duration(rule.WindowSec) * time.Second
		if window <= 0 {
			window = time.Minute
		}
		if rule.LastTrigger != nil && now.Sub(*rule.LastTrigger) < window {
			continue
		}
		cutoff := now.Add(-window)
		count := 0
		var latest *SyslogEntry
		for i := len(m.entries) - 1; i >= 0; i-- {
			entry := m.entries[i]
			if entry.Timestamp.Before(cutoff) {
				break
			}
			if rule.Facility != "" && !strings.EqualFold(FacilityNames[entry.Facility], rule.Facility) {
				continue
			}
			if rule.Severity != "" && !strings.EqualFold(SeverityNames[entry.Severity], rule.Severity) {
				continue
			}
			count++
			latest = entry
		}
		if count >= rule.Frequency && latest != nil {
			m.triggerAlert(rule, latest)
		}
	}
}

// ========== 日志查询 API ==========

// SearchLogs 搜索日志.
func (m *Manager) SearchLogs(req SearchRequest) *SearchResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 100
	}

	var results []*SyslogEntry
	for _, entry := range m.entries {
		if !m.matchFilter(entry, &req) {
			continue
		}
		results = append(results, entry)
	}

	// 排序
	sort.Slice(results, func(i, j int) bool {
		if req.SortOrder == "asc" {
			return results[i].Timestamp.Before(results[j].Timestamp)
		}
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	total := len(results)
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return &SearchResponse{
		Total:   total,
		Page:    req.Page,
		Size:    req.PageSize,
		Entries: results[start:end],
	}
}

// GetLogByID 根据 ID 获取日志.
func (m *Manager) GetLogByID(id string) (*SyslogEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, entry := range m.entries {
		if entry.ID == id {
			return entry, nil
		}
	}
	return nil, fmt.Errorf("log entry %q not found", id)
}

// ========== 转发目标管理 ==========

// CreateForwardTarget 创建转发目标.
func (m *Manager) CreateForwardTarget(req CreateForwardTargetRequest) *ForwardTarget {
	m.mu.Lock()
	defer m.mu.Unlock()

	target := &ForwardTarget{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Host:      req.Host,
		Port:      req.Port,
		Protocol:  req.Protocol,
		Enabled:   req.Enabled,
		Filter:    req.Filter,
		CreatedAt: time.Now(),
	}

	m.forwardTargets[target.ID] = target
	log.Printf("[syslogserver] 创建转发目标: %s (%s:%d)", target.Name, target.Host, target.Port)
	return target
}

// GetForwardTarget 获取转发目标.
func (m *Manager) GetForwardTarget(id string) (*ForwardTarget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	target, ok := m.forwardTargets[id]
	if !ok {
		return nil, fmt.Errorf("forward target %q not found", id)
	}
	return target, nil
}

// ListForwardTargets 列出所有转发目标.
func (m *Manager) ListForwardTargets() []*ForwardTarget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	targets := make([]*ForwardTarget, 0, len(m.forwardTargets))
	for _, t := range m.forwardTargets {
		targets = append(targets, t)
	}
	return targets
}

// UpdateForwardTarget 更新转发目标.
func (m *Manager) UpdateForwardTarget(id string, req UpdateForwardTargetRequest) (*ForwardTarget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, ok := m.forwardTargets[id]
	if !ok {
		return nil, fmt.Errorf("forward target %q not found", id)
	}

	if req.Name != nil {
		target.Name = *req.Name
	}
	if req.Host != nil {
		target.Host = *req.Host
	}
	if req.Port != nil {
		target.Port = *req.Port
	}
	if req.Protocol != nil {
		target.Protocol = *req.Protocol
	}
	if req.Enabled != nil {
		target.Enabled = *req.Enabled
	}
	if req.Filter != nil {
		target.Filter = *req.Filter
	}

	log.Printf("[syslogserver] 更新转发目标: %s", target.Name)
	return target, nil
}

// DeleteForwardTarget 删除转发目标.
func (m *Manager) DeleteForwardTarget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.forwardTargets[id]; !ok {
		return fmt.Errorf("forward target %q not found", id)
	}

	delete(m.forwardTargets, id)
	log.Printf("[syslogserver] 删除转发目标: %s", id)
	return nil
}

// ========== 告警规则管理 ==========

// CreateAlertRule 创建告警规则.
func (m *Manager) CreateAlertRule(req CreateAlertRuleRequest) *AlertRule {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule := &AlertRule{
		ID:         uuid.New().String(),
		Name:       req.Name,
		Enabled:    req.Enabled,
		Type:       req.Type,
		Keyword:    req.Keyword,
		Facility:   req.Facility,
		Severity:   req.Severity,
		Frequency:  req.Frequency,
		WindowSec:  req.WindowSec,
		NotifyType: req.NotifyType,
		WebhookURL: req.WebhookURL,
		CreatedAt:  time.Now(),
	}

	m.alertRules[rule.ID] = rule
	log.Printf("[syslogserver] 创建告警规则: %s", rule.Name)
	return rule
}

// GetAlertRule 获取告警规则.
func (m *Manager) GetAlertRule(id string) (*AlertRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, ok := m.alertRules[id]
	if !ok {
		return nil, fmt.Errorf("alert rule %q not found", id)
	}
	return rule, nil
}

// ListAlertRules 列出所有告警规则.
func (m *Manager) ListAlertRules() []*AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*AlertRule, 0, len(m.alertRules))
	for _, r := range m.alertRules {
		rules = append(rules, r)
	}
	return rules
}

// UpdateAlertRule 更新告警规则.
func (m *Manager) UpdateAlertRule(id string, req UpdateAlertRuleRequest) (*AlertRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.alertRules[id]
	if !ok {
		return nil, fmt.Errorf("alert rule %q not found", id)
	}

	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.Keyword != nil {
		rule.Keyword = *req.Keyword
	}
	if req.Facility != nil {
		rule.Facility = *req.Facility
	}
	if req.Severity != nil {
		rule.Severity = *req.Severity
	}
	if req.Frequency != nil {
		rule.Frequency = *req.Frequency
	}
	if req.WindowSec != nil {
		rule.WindowSec = *req.WindowSec
	}
	if req.NotifyType != nil {
		rule.NotifyType = *req.NotifyType
	}
	if req.WebhookURL != nil {
		rule.WebhookURL = *req.WebhookURL
	}

	log.Printf("[syslogserver] 更新告警规则: %s", rule.Name)
	return rule, nil
}

// DeleteAlertRule 删除告警规则.
func (m *Manager) DeleteAlertRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.alertRules[id]; !ok {
		return fmt.Errorf("alert rule %q not found", id)
	}

	delete(m.alertRules, id)
	log.Printf("[syslogserver] 删除告警规则: %s", id)
	return nil
}

// ListAlertEvents 列出告警事件.
func (m *Manager) ListAlertEvents(limit int) []*AlertEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.alertEvents) {
		limit = len(m.alertEvents)
	}

	// 返回最近的事件
	start := len(m.alertEvents) - limit
	if start < 0 {
		start = 0
	}

	events := make([]*AlertEvent, limit)
	copy(events, m.alertEvents[start:])
	return events
}

// ========== 仪表板统计 ==========

// GetDashboardStats 获取仪表板统计.
func (m *Manager) GetDashboardStats() *DashboardStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &DashboardStats{
		TotalEntries:   int64(len(m.entries)),
		EntriesToday:   m.entriesToday,
		EntriesPerHour: make([]HourlyCount, 24),
		BySeverity:     make(map[string]int64),
		ByFacility:     make(map[string]int64),
		ByHost:         make(map[string]int64),
		ByApp:          make(map[string]int64),
		TopSources:     make([]SourceCount, 0),
		RecentAlerts:   m.ListAlertEvents(10),
	}

	// 初始化每小时统计
	now := time.Now()
	for i := 0; i < 24; i++ {
		stats.EntriesPerHour[i] = HourlyCount{
			Hour:  fmt.Sprintf("%02d:00", i),
			Count: 0,
		}
	}

	// 统计每小时日志数量
	for _, entry := range m.entries {
		if entry.Timestamp.Format("2006-01-02") == now.Format("2006-01-02") {
			hour := entry.Timestamp.Hour()
			stats.EntriesPerHour[hour].Count++
		}
	}

	// 复制统计计数器
	m.statsMu.RLock()
	for sev, count := range m.statsBySeverity {
		stats.BySeverity[SeverityNames[sev]] = count
	}
	for fac, count := range m.statsByFacility {
		stats.ByFacility[FacilityNames[fac]] = count
	}
	for host, count := range m.statsByHost {
		stats.ByHost[host] = count
	}
	for app, count := range m.statsByApp {
		stats.ByApp[app] = count
	}
	m.statsMu.RUnlock()

	// 计算 Top Sources
	hostCounts := make([]SourceCount, 0)
	for host, count := range stats.ByHost {
		hostCounts = append(hostCounts, SourceCount{Source: host, Count: count})
	}
	sort.Slice(hostCounts, func(i, j int) bool {
		return hostCounts[i].Count > hostCounts[j].Count
	})
	if len(hostCounts) > 10 {
		hostCounts = hostCounts[:10]
	}
	stats.TopSources = hostCounts

	return stats
}

// ========== 日志导出 ==========

// ExportLogs 导出日志.
func (m *Manager) ExportLogs(req ExportRequest) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*SyslogEntry
	for _, entry := range m.entries {
		if req.Hostname != "" && entry.Hostname != req.Hostname {
			continue
		}
		if req.Facility != "" && FacilityNames[entry.Facility] != req.Facility {
			continue
		}
		if req.Severity != "" && SeverityNames[entry.Severity] != req.Severity {
			continue
		}
		if req.StartTime != nil && entry.Timestamp.Before(*req.StartTime) {
			continue
		}
		if req.EndTime != nil && entry.Timestamp.After(*req.EndTime) {
			continue
		}
		results = append(results, entry)
	}

	// 限制数量
	if req.Limit > 0 && len(results) > req.Limit {
		results = results[:req.Limit]
	}

	switch req.Format {
	case "csv":
		return m.exportCSV(results)
	case "json":
		return m.exportJSON(results)
	default:
		return nil, fmt.Errorf("unsupported format: %s", req.Format)
	}
}

// exportCSV 导出 CSV 格式.
func (m *Manager) exportCSV(entries []*SyslogEntry) ([]byte, error) {
	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// 写入表头
	writer.Write([]string{
		"id", "timestamp", "hostname", "app_name", "facility", "severity", "message", "source_ip", "protocol",
	})

	// 写入数据
	for _, entry := range entries {
		writer.Write([]string{
			entry.ID,
			entry.Timestamp.Format(time.RFC3339),
			entry.Hostname,
			entry.AppName,
			FacilityNames[entry.Facility],
			SeverityNames[entry.Severity],
			entry.Message,
			entry.SourceIP,
			entry.Protocol,
		})
	}

	writer.Flush()
	return []byte(buf.String()), writer.Error()
}

// exportJSON 导出 JSON 格式.
func (m *Manager) exportJSON(entries []*SyslogEntry) ([]byte, error) {
	return json.MarshalIndent(entries, "", "  ")
}

// ========== WebSocket 管理 ==========

// RegisterWSClient 注册 WebSocket 客户端.
func (m *Manager) RegisterWSClient(client *WSClient) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.wsClients[client.ID] = client
	log.Printf("[syslogserver] WebSocket 客户端注册: %s", client.ID)
}

// UnregisterWSClient 注销 WebSocket 客户端.
func (m *Manager) UnregisterWSClient(clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client, ok := m.wsClients[clientID]; ok {
		close(client.Send)
		delete(m.wsClients, clientID)
		log.Printf("[syslogserver] WebSocket 客户端注销: %s", clientID)
	}
}

// GetWSClientCount 获取 WebSocket 客户端数量.
func (m *Manager) GetWSClientCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.wsClients)
}
