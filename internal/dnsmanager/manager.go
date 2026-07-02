// Package dnsmanager 提供 DNS 管理服务器核心业务逻辑
package dnsmanager

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager DNS 管理器.
type Manager struct {
	mu        sync.RWMutex
	records   map[string]*DNSRecord
	zones     map[string]*DNSZone
	rules     map[string]*DNSRule
	upstreams map[string]*UpstreamServer
	queryLog  []*DNSQuery
	nextID    int64
}

// NewManager 创建新的 DNS 管理器.
func NewManager() *Manager {
	m := &Manager{
		records:   make(map[string]*DNSRecord),
		zones:     make(map[string]*DNSZone),
		rules:     make(map[string]*DNSRule),
		upstreams: make(map[string]*UpstreamServer),
		queryLog:  make([]*DNSQuery, 0),
	}

	// 预置公共 DNS 上游服务器
	m.addDefaultUpstreams()

	// 预置常用广告拦截规则
	m.addDefaultBlockRules()

	return m
}

// addDefaultUpstreams 添加默认上游 DNS 服务器.
func (m *Manager) addDefaultUpstreams() {
	defaults := []UpstreamServer{
		{ID: uuid.New().String(), Address: "8.8.8.8", Port: 53, Protocol: ProtocolUDP, Enabled: true},
		{ID: uuid.New().String(), Address: "1.1.1.1", Port: 53, Protocol: ProtocolUDP, Enabled: true},
		{ID: uuid.New().String(), Address: "223.5.5.5", Port: 53, Protocol: ProtocolUDP, Enabled: true},
	}

	for _, u := range defaults {
		m.upstreams[u.ID] = &u
	}
}

// addDefaultBlockRules 添加默认广告拦截规则.
func (m *Manager) addDefaultBlockRules() {
	defaultRules := []DNSRule{
		{ID: uuid.New().String(), Pattern: "ads.google.com", Action: ActionBlock, Enabled: true, Category: "ads"},
		{ID: uuid.New().String(), Pattern: "doubleclick.net", Action: ActionBlock, Enabled: true, Category: "ads"},
		{ID: uuid.New().String(), Pattern: "googleadservices.com", Action: ActionBlock, Enabled: true, Category: "ads"},
		{ID: uuid.New().String(), Pattern: "ads.facebook.com", Action: ActionBlock, Enabled: true, Category: "ads"},
		{ID: uuid.New().String(), Pattern: "analytics.google.com", Action: ActionBlock, Enabled: true, Category: "tracking"},
		{ID: uuid.New().String(), Pattern: "tracking.example.com", Action: ActionBlock, Enabled: true, Category: "tracking"},
		{ID: uuid.New().String(), Pattern: "malware.example.com", Action: ActionBlock, Enabled: true, Category: "malware"},
		{ID: uuid.New().String(), Pattern: "phishing.example.com", Action: ActionBlock, Enabled: true, Category: "malware"},
		{ID: uuid.New().String(), Pattern: "popup.example.com", Action: ActionBlock, Enabled: true, Category: "ads"},
		{ID: uuid.New().String(), Pattern: "banner.example.com", Action: ActionBlock, Enabled: true, Category: "ads"},
		{ID: uuid.New().String(), Pattern: "*.ad.doubleclick.net", Action: ActionBlock, Enabled: true, Category: "ads"},
		{ID: uuid.New().String(), Pattern: "tracker.example.com", Action: ActionBlock, Enabled: true, Category: "tracking"},
	}

	for _, rule := range defaultRules {
		rule.CreatedAt = time.Now()
		m.rules[rule.ID] = &rule
	}
}

// ========== DNS 记录管理 ==========

// AddRecord 添加 DNS 记录.
func (m *Manager) AddRecord(zone string, record DNSRecord) (*DNSRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if record.Name == "" {
		return nil, fmt.Errorf("记录名称不能为空")
	}
	if record.Value == "" {
		return nil, fmt.Errorf("记录值不能为空")
	}

	// 验证记录类型
	switch record.Type {
	case RecordTypeA, RecordTypeAAAA, RecordTypeCNAME, RecordTypeMX, RecordTypeTXT, RecordTypeNS, RecordTypeSRV:
	default:
		return nil, fmt.Errorf("无效的记录类型: %s", record.Type)
	}

	// 确保区域存在
	zoneName := zone
	if zoneName == "" {
		zoneName = "default"
	}
	if _, exists := m.zones[zoneName]; !exists {
		m.zones[zoneName] = &DNSZone{
			ID:      uuid.New().String(),
			Name:    zoneName,
			Records: make(map[string][]DNSRecord),
			Serial:  1,
			Refresh: 3600,
			Retry:   600,
			Expire:  86400,
			Minimum: 300,
		}
	}

	now := time.Now()
	newRecord := &DNSRecord{
		ID:        uuid.New().String(),
		Name:      strings.ToLower(record.Name),
		Type:      record.Type,
		Value:     record.Value,
		TTL:       record.TTL,
		Priority:  record.Priority,
		CreatedAt: now,
		Enabled:   true,
	}

	if newRecord.TTL == 0 {
		newRecord.TTL = 300
	}

	m.records[newRecord.ID] = newRecord

	// 添加到区域
	z := m.zones[zoneName]
	key := newRecord.Name
	z.Records[key] = append(z.Records[key], *newRecord)
	z.Serial++

	log.Printf("[dnsmanager] 添加记录: %s -> %s (%s) 到区域 %s", newRecord.Name, newRecord.Value, newRecord.Type, zoneName)
	return newRecord, nil
}

// UpdateRecord 更新 DNS 记录.
func (m *Manager) UpdateRecord(id string, req UpdateRecordRequest) (*DNSRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, exists := m.records[id]
	if !exists {
		return nil, fmt.Errorf("记录不存在: %s", id)
	}

	if req.Name != nil {
		record.Name = strings.ToLower(*req.Name)
	}
	if req.Type != nil {
		record.Type = *req.Type
	}
	if req.Value != nil {
		record.Value = *req.Value
	}
	if req.TTL != nil {
		record.TTL = *req.TTL
	}
	if req.Priority != nil {
		record.Priority = *req.Priority
	}
	if req.Enabled != nil {
		record.Enabled = *req.Enabled
	}

	log.Printf("[dnsmanager] 更新记录: %s", record.Name)
	return record, nil
}

// DeleteRecord 删除 DNS 记录.
func (m *Manager) DeleteRecord(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, exists := m.records[id]
	if !exists {
		return fmt.Errorf("记录不存在: %s", id)
	}

	// 从区域中删除
	for _, zone := range m.zones {
		if records, ok := zone.Records[record.Name]; ok {
			for i, r := range records {
				if r.ID == id {
					zone.Records[record.Name] = append(records[:i], records[i+1:]...)
					zone.Serial++
					break
				}
			}
		}
	}

	delete(m.records, id)
	log.Printf("[dnsmanager] 删除记录: %s", record.Name)
	return nil
}

// ListRecords 列出指定区域的所有记录.
func (m *Manager) ListRecords(zone string) ([]DNSRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	zoneName := zone
	if zoneName == "" {
		zoneName = "default"
	}

	z, exists := m.zones[zoneName]
	if !exists {
		// 返回空列表而不是错误
		return []DNSRecord{}, nil
	}

	var result []DNSRecord
	for _, records := range z.Records {
		result = append(result, records...)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// ========== DNS 规则管理 ==========

// AddRule 添加过滤规则.
func (m *Manager) AddRule(rule DNSRule) (*DNSRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.Pattern == "" {
		return nil, fmt.Errorf("规则模式不能为空")
	}

	// 验证规则动作
	switch rule.Action {
	case ActionBlock, ActionAllow, ActionRedirect:
	default:
		return nil, fmt.Errorf("无效的规则动作: %s", rule.Action)
	}

	if rule.Action == ActionRedirect && rule.Target == "" {
		return nil, fmt.Errorf("重定向规则必须指定目标地址")
	}

	now := time.Now()
	newRule := &DNSRule{
		ID:        uuid.New().String(),
		Pattern:   strings.ToLower(rule.Pattern),
		Action:    rule.Action,
		Target:    rule.Target,
		Enabled:   true,
		Category:  rule.Category,
		HitCount:  0,
		CreatedAt: now,
	}

	m.rules[newRule.ID] = newRule
	log.Printf("[dnsmanager] 添加规则: %s (%s)", newRule.Pattern, newRule.Action)
	return newRule, nil
}

// UpdateRule 更新过滤规则.
func (m *Manager) UpdateRule(id string, req UpdateRuleRequest) (*DNSRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[id]
	if !exists {
		return nil, fmt.Errorf("规则不存在: %s", id)
	}

	if req.Pattern != nil {
		rule.Pattern = strings.ToLower(*req.Pattern)
	}
	if req.Action != nil {
		rule.Action = *req.Action
	}
	if req.Target != nil {
		rule.Target = *req.Target
	}
	if req.Category != nil {
		rule.Category = *req.Category
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}

	log.Printf("[dnsmanager] 更新规则: %s", rule.Pattern)
	return rule, nil
}

// DeleteRule 删除过滤规则.
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[id]
	if !exists {
		return fmt.Errorf("规则不存在: %s", id)
	}

	delete(m.rules, id)
	log.Printf("[dnsmanager] 删除规则: %s", rule.Pattern)
	return nil
}

// ListRules 列出所有过滤规则.
func (m *Manager) ListRules() ([]DNSRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]DNSRule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, *r)
	}

	sort.Slice(rules, func(i, j int) bool {
		return rules[i].CreatedAt.After(rules[j].CreatedAt)
	})

	return rules, nil
}

// ToggleRule 切换规则启用状态.
func (m *Manager) ToggleRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[id]
	if !exists {
		return fmt.Errorf("规则不存在: %s", id)
	}

	rule.Enabled = !rule.Enabled
	log.Printf("[dnsmanager] 切换规则状态: %s -> %v", rule.Pattern, rule.Enabled)
	return nil
}

// ========== DNS 解析 ==========

// Resolve 解析 DNS 请求.
func (m *Manager) Resolve(domain, queryType string) (*DNSRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	domain = strings.ToLower(strings.TrimSuffix(domain, "."))

	// 查找自定义记录
	for _, record := range m.records {
		if record.Enabled && record.Name == domain && string(record.Type) == queryType {
			return record, nil
		}
	}

	return nil, fmt.Errorf("未找到记录: %s (%s)", domain, queryType)
}

// ShouldBlock 检查域名是否应该被拦截.
func (m *Manager) ShouldBlock(domain string) (bool, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	domain = strings.ToLower(strings.TrimSuffix(domain, "."))

	// 按规则顺序检查（按创建时间倒序，最新规则优先）
	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}

		if matchDomain(domain, rule.Pattern) {
			rule.HitCount++
			switch rule.Action {
			case ActionBlock:
				return true, rule.Pattern, nil
			case ActionAllow:
				return false, rule.Pattern, nil
			case ActionRedirect:
				return false, rule.Pattern, nil
			}
		}
	}

	return false, "", nil
}

// matchDomain 匹配域名（支持通配符）.
func matchDomain(domain, pattern string) bool {
	if domain == pattern {
		return true
	}

	// 子域名匹配
	if strings.HasSuffix(domain, "."+pattern) {
		return true
	}

	// 通配符匹配
	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*") // ".example.com"
		if strings.HasSuffix(domain, suffix) || domain == strings.TrimPrefix(suffix, ".") {
			return true
		}
	}

	// 正则匹配
	if matched, _ := regexp.MatchString(pattern, domain); matched {
		return true
	}

	return false
}

// ========== 查询日志 ==========

// LogQuery 记录 DNS 查询.
func (m *Manager) LogQuery(query DNSQuery) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if query.ID == "" {
		query.ID = uuid.New().String()
	}
	if query.Timestamp.IsZero() {
		query.Timestamp = time.Now()
	}

	m.queryLog = append(m.queryLog, &query)

	// 限制日志数量
	if len(m.queryLog) > 10000 {
		m.queryLog = m.queryLog[len(m.queryLog)-10000:]
	}

	return nil
}

// GetStats 获取统计信息.
func (m *Manager) GetStats(period string) (*DNSStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &DNSStats{
		TopDomains: make([]DomainStat, 0),
		TopClients: make([]ClientStat, 0),
		TopBlocked: make([]DomainStat, 0),
	}

	// 根据时间段过滤日志
	var cutoff time.Time
	switch period {
	case "hour":
		cutoff = time.Now().Add(-1 * time.Hour)
	case "day":
		cutoff = time.Now().Add(-24 * time.Hour)
	case "week":
		cutoff = time.Now().Add(-7 * 24 * time.Hour)
	case "month":
		cutoff = time.Now().Add(-30 * 24 * time.Hour)
	default:
		cutoff = time.Time{} // 不过滤
	}

	domainCounts := make(map[string]int64)
	clientCounts := make(map[string]int64)
	blockedDomainCounts := make(map[string]int64)

	for _, q := range m.queryLog {
		if !cutoff.IsZero() && q.Timestamp.Before(cutoff) {
			continue
		}

		stats.TotalQueries++
		domainCounts[q.Domain]++
		clientCounts[q.Client]++

		if q.Blocked {
			stats.BlockedQueries++
			blockedDomainCounts[q.Domain]++
		} else {
			stats.AllowedQueries++
		}
	}

	// 统计 TopDomains
	for domain, count := range domainCounts {
		stats.TopDomains = append(stats.TopDomains, DomainStat{Domain: domain, Count: count})
	}
	sort.Slice(stats.TopDomains, func(i, j int) bool {
		return stats.TopDomains[i].Count > stats.TopDomains[j].Count
	})
	if len(stats.TopDomains) > 10 {
		stats.TopDomains = stats.TopDomains[:10]
	}

	// 统计 TopClients
	for client, count := range clientCounts {
		stats.TopClients = append(stats.TopClients, ClientStat{Client: client, Count: count})
	}
	sort.Slice(stats.TopClients, func(i, j int) bool {
		return stats.TopClients[i].Count > stats.TopClients[j].Count
	})
	if len(stats.TopClients) > 10 {
		stats.TopClients = stats.TopClients[:10]
	}

	// 统计 TopBlocked
	for domain, count := range blockedDomainCounts {
		stats.TopBlocked = append(stats.TopBlocked, DomainStat{Domain: domain, Count: count})
	}
	sort.Slice(stats.TopBlocked, func(i, j int) bool {
		return stats.TopBlocked[i].Count > stats.TopBlocked[j].Count
	})
	if len(stats.TopBlocked) > 10 {
		stats.TopBlocked = stats.TopBlocked[:10]
	}

	return stats, nil
}

// GetQueryLog 获取查询日志（分页）.
func (m *Manager) GetQueryLog(limit, offset int) ([]DNSQuery, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := len(m.queryLog)

	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	// 按时间倒序
	sorted := make([]*DNSQuery, len(m.queryLog))
	copy(sorted, m.queryLog)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.After(sorted[j].Timestamp)
	})

	if offset >= total {
		return []DNSQuery{}, total, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	result := make([]DNSQuery, 0, end-offset)
	for _, q := range sorted[offset:end] {
		result = append(result, *q)
	}

	return result, total, nil
}

// ========== 上游服务器管理 ==========

// AddUpstream 添加上游 DNS 服务器.
func (m *Manager) AddUpstream(server UpstreamServer) (*UpstreamServer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if server.Address == "" {
		return nil, fmt.Errorf("服务器地址不能为空")
	}
	if server.Port <= 0 || server.Port > 65535 {
		return nil, fmt.Errorf("无效的端口号: %d", server.Port)
	}

	// 验证协议
	switch server.Protocol {
	case ProtocolUDP, ProtocolTCP, ProtocolDoH, ProtocolDoT:
	default:
		return nil, fmt.Errorf("无效的协议: %s", server.Protocol)
	}

	newServer := &UpstreamServer{
		ID:       uuid.New().String(),
		Address:  server.Address,
		Port:     server.Port,
		Protocol: server.Protocol,
		Enabled:  true,
		Latency:  0,
	}

	m.upstreams[newServer.ID] = newServer
	log.Printf("[dnsmanager] 添加上游服务器: %s:%d (%s)", newServer.Address, newServer.Port, newServer.Protocol)
	return newServer, nil
}

// RemoveUpstream 删除上游 DNS 服务器.
func (m *Manager) RemoveUpstream(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	server, exists := m.upstreams[id]
	if !exists {
		return fmt.Errorf("上游服务器不存在: %s", id)
	}

	delete(m.upstreams, id)
	log.Printf("[dnsmanager] 删除上游服务器: %s:%d", server.Address, server.Port)
	return nil
}

// TestUpstream 测试上游 DNS 服务器延迟.
func (m *Manager) TestUpstream(id string) (time.Duration, error) {
	m.mu.RLock()
	server, exists := m.upstreams[id]
	if !exists {
		m.mu.RUnlock()
		return 0, fmt.Errorf("上游服务器不存在: %s", id)
	}
	m.mu.RUnlock()

	// 模拟测试（实际应发送 DNS 查询）
	start := time.Now()
	// 这里应该实际测试 DNS 服务器
	// 为简化实现，使用模拟延迟
	time.Sleep(10 * time.Millisecond)
	latency := time.Since(start)

	m.mu.Lock()
	server.Latency = latency.Milliseconds()
	m.mu.Unlock()

	return latency, nil
}

// ========== 拦截列表导入 ==========

// ImportBlockList 从 URL 导入拦截列表.
func (m *Manager) ImportBlockList(url string) (int, error) {
	if url == "" {
		return 0, fmt.Errorf("URL 不能为空")
	}

	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("下载拦截列表失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	var imported int
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}

		// 解析 AdGuard 格式 (||domain.com^)
		if strings.HasPrefix(line, "||") && strings.HasSuffix(line, "^") {
			domain := strings.TrimPrefix(line, "||")
			domain = strings.TrimSuffix(domain, "^")
			domain = strings.TrimSpace(domain)
			if domain != "" {
				m.mu.Lock()
				rule := &DNSRule{
					ID:        uuid.New().String(),
					Pattern:   strings.ToLower(domain),
					Action:    ActionBlock,
					Enabled:   true,
					Category:  "imported",
					HitCount:  0,
					CreatedAt: time.Now(),
				}
				m.rules[rule.ID] = rule
				m.mu.Unlock()
				imported++
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return imported, fmt.Errorf("读取拦截列表出错: %w", err)
	}

	log.Printf("[dnsmanager] 导入拦截列表: %d 条规则", imported)
	return imported, nil
}

// ========== 配置导出 ==========

// ExportConfig 导出配置为 JSON.
func (m *Manager) ExportConfig() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config := map[string]interface{}{
		"records":   m.records,
		"rules":     m.rules,
		"upstreams": m.upstreams,
		"zones":     m.zones,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("导出配置失败: %w", err)
	}

	return data, nil
}
