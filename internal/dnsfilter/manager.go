// Package dnsfilter 提供 DNS 广告过滤核心业务逻辑
package dnsfilter

import (
	"bufio"
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

// Manager DNS 过滤管理器.
type Manager struct {
	records     map[string]*DNSRecord
	filterLists map[string]*FilterList
	rules       map[string]*FilterRule
	upstreams   map[string]*UpstreamDNS
	policies    map[string]*FilterPolicy
	cache       map[cacheKey]*DNSCacheEntry
	queryLogs   []*QueryLog
	logStream   []chan LogStreamEvent // SSE 订阅者

	running   bool
	startTime time.Time
	mu        sync.RWMutex
	cacheMu   sync.RWMutex
	logMu     sync.RWMutex
}

// NewManager 创建 DNS 过滤管理器.
func NewManager() *Manager {
	m := &Manager{
		records:     make(map[string]*DNSRecord),
		filterLists: make(map[string]*FilterList),
		rules:       make(map[string]*FilterRule),
		upstreams:   make(map[string]*UpstreamDNS),
		policies:    make(map[string]*FilterPolicy),
		cache:       make(map[cacheKey]*DNSCacheEntry),
		queryLogs:   make([]*QueryLog, 0),
		logStream:   make([]chan LogStreamEvent, 0),
	}

	// 添加默认上游 DNS
	m.addDefaultUpstreams()

	return m
}

// addDefaultUpstreams 添加默认上游 DNS 服务器.
func (m *Manager) addDefaultUpstreams() {
	defaults := []UpstreamDNS{
		{ID: uuid.New().String(), Name: "Google DNS", Address: "8.8.8.8:53", Protocol: "udp", Enabled: true, Weight: 1, IsDefault: true},
		{ID: uuid.New().String(), Name: "Cloudflare DNS", Address: "1.1.1.1:53", Protocol: "udp", Enabled: true, Weight: 1, IsDefault: false},
		{ID: uuid.New().String(), Name: "阿里 DNS", Address: "223.5.5.5:53", Protocol: "udp", Enabled: true, Weight: 1, IsDefault: false},
	}

	for _, u := range defaults {
		u := u
		m.upstreams[u.ID] = &u
	}
}

// ========== DNS 服务器控制 ==========

// Start 启动 DNS 服务.
func (m *Manager) Start(listenAddr string, udpPort, tcpPort int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("dns server already running")
	}

	m.running = true
	m.startTime = time.Now()

	log.Printf("[dnsfilter] DNS 服务启动: %s (UDP:%d, TCP:%d)", listenAddr, udpPort, tcpPort)
	return nil
}

// Stop 停止 DNS 服务.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return fmt.Errorf("dns server not running")
	}

	m.running = false
	log.Printf("[dnsfilter] DNS 服务已停止")
	return nil
}

// GetStatus 获取 DNS 服务状态.
func (m *Manager) GetStatus() *DNSStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalBlocked := int64(0)
	for _, logEntry := range m.queryLogs {
		if logEntry.IsFiltered {
			totalBlocked++
		}
	}

	uptime := ""
	if m.running {
		uptime = time.Since(m.startTime).Round(time.Second).String()
	}

	return &DNSStatus{
		Running:        m.running,
		ListenAddr:     "0.0.0.0",
		UDPPort:        53,
		TCPPort:        53,
		TotalRules:     len(m.rules),
		CacheSize:      len(m.cache),
		Uptime:         uptime,
		StartTime:      m.startTime,
		QueriesServed:  int64(len(m.queryLogs)),
		QueriesBlocked: totalBlocked,
	}
}

// ========== 自定义 DNS 记录 CRUD ==========

// CreateDNSRecord 创建自定义 DNS 记录.
func (m *Manager) CreateDNSRecord(req CreateDNSRecordRequest) *DNSRecord {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	record := &DNSRecord{
		ID:        uuid.New().String(),
		Name:      strings.ToLower(req.Name),
		Type:      req.Type,
		Value:     req.Value,
		TTL:       req.TTL,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if record.TTL == 0 {
		record.TTL = 300 // 默认 TTL 5 分钟
	}

	m.records[record.ID] = record
	log.Printf("[dnsfilter] 创建 DNS 记录: %s -> %s (%s)", record.Name, record.Value, record.Type)
	return record
}

// GetDNSRecord 获取 DNS 记录.
func (m *Manager) GetDNSRecord(id string) (*DNSRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.records[id]
	if !ok {
		return nil, fmt.Errorf("dns record %q not found", id)
	}
	return record, nil
}

// ListDNSRecords 列出所有 DNS 记录.
func (m *Manager) ListDNSRecords() []*DNSRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := make([]*DNSRecord, 0, len(m.records))
	for _, r := range m.records {
		records = append(records, r)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Name < records[j].Name
	})

	return records
}

// UpdateDNSRecord 更新 DNS 记录.
func (m *Manager) UpdateDNSRecord(id string, req UpdateDNSRecordRequest) (*DNSRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.records[id]
	if !ok {
		return nil, fmt.Errorf("dns record %q not found", id)
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
	if req.Enabled != nil {
		record.Enabled = *req.Enabled
	}

	record.UpdatedAt = time.Now()
	m.invalidateCache(record.Name, string(record.Type))

	log.Printf("[dnsfilter] 更新 DNS 记录: %s", record.Name)
	return record, nil
}

// DeleteDNSRecord 删除 DNS 记录.
func (m *Manager) DeleteDNSRecord(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.records[id]
	if !ok {
		return fmt.Errorf("dns record %q not found", id)
	}

	m.invalidateCache(record.Name, string(record.Type))
	delete(m.records, id)

	log.Printf("[dnsfilter] 删除 DNS 记录: %s", record.Name)
	return nil
}

// ========== 过滤规则列表 CRUD ==========

// CreateFilterList 创建过滤规则列表.
func (m *Manager) CreateFilterList(req CreateFilterListRequest) *FilterList {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	fl := &FilterList{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		URL:         req.URL,
		Enabled:     true,
		RuleCount:   0,
		CreatedAt:   now,
	}

	m.filterLists[fl.ID] = fl
	log.Printf("[dnsfilter] 创建过滤规则列表: %s (%s)", fl.Name, fl.Type)
	return fl
}

// GetFilterList 获取过滤规则列表.
func (m *Manager) GetFilterList(id string) (*FilterList, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fl, ok := m.filterLists[id]
	if !ok {
		return nil, fmt.Errorf("filter list %q not found", id)
	}
	return fl, nil
}

// ListFilterLists 列出所有过滤规则列表.
func (m *Manager) ListFilterLists() []*FilterList {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lists := make([]*FilterList, 0, len(m.filterLists))
	for _, fl := range m.filterLists {
		lists = append(lists, fl)
	}

	sort.Slice(lists, func(i, j int) bool {
		return lists[i].Name < lists[j].Name
	})

	return lists
}

// UpdateFilterList 更新过滤规则列表.
func (m *Manager) UpdateFilterList(id string, req UpdateFilterListRequest) (*FilterList, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fl, ok := m.filterLists[id]
	if !ok {
		return nil, fmt.Errorf("filter list %q not found", id)
	}

	if req.Name != nil {
		fl.Name = *req.Name
	}
	if req.Description != nil {
		fl.Description = *req.Description
	}
	if req.URL != nil {
		fl.URL = *req.URL
	}
	if req.Enabled != nil {
		fl.Enabled = *req.Enabled
	}

	log.Printf("[dnsfilter] 更新过滤规则列表: %s", fl.Name)
	return fl, nil
}

// DeleteFilterList 删除过滤规则列表.
func (m *Manager) DeleteFilterList(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fl, ok := m.filterLists[id]
	if !ok {
		return fmt.Errorf("filter list %q not found", id)
	}

	for ruleID, rule := range m.rules {
		if rule.ListID == id {
			delete(m.rules, ruleID)
		}
	}

	delete(m.filterLists, id)
	log.Printf("[dnsfilter] 删除过滤规则列表: %s", fl.Name)
	return nil
}

// ========== 过滤规则 CRUD ==========

// CreateFilterRule 创建过滤规则.
func (m *Manager) CreateFilterRule(req CreateFilterRuleRequest) *FilterRule {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	rule := &FilterRule{
		ID:        uuid.New().String(),
		Pattern:   strings.ToLower(req.Pattern),
		Action:    req.Action,
		ListID:    req.ListID,
		Enabled:   true,
		HitCount:  0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.rules[rule.ID] = rule

	if rule.ListID != "" {
		if fl, ok := m.filterLists[rule.ListID]; ok {
			fl.RuleCount++
		}
	}

	log.Printf("[dnsfilter] 创建过滤规则: %s (%s)", rule.Pattern, rule.Action)
	return rule
}

// ListFilterRules 列出过滤规则.
func (m *Manager) ListFilterRules(listID string) []*FilterRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var rules []*FilterRule
	for _, r := range m.rules {
		if listID == "" || r.ListID == listID {
			rules = append(rules, r)
		}
	}

	sort.Slice(rules, func(i, j int) bool {
		return rules[i].CreatedAt.After(rules[j].CreatedAt)
	})

	return rules
}

// DeleteFilterRule 删除过滤规则.
func (m *Manager) DeleteFilterRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.rules[id]
	if !ok {
		return fmt.Errorf("filter rule %q not found", id)
	}

	if rule.ListID != "" {
		if fl, ok := m.filterLists[rule.ListID]; ok {
			fl.RuleCount--
		}
	}

	delete(m.rules, id)
	log.Printf("[dnsfilter] 删除过滤规则: %s", rule.Pattern)
	return nil
}

// SubscribeFilterList 订阅过滤规则列表.
func (m *Manager) SubscribeFilterList(listID string) error {
	m.mu.Lock()
	fl, ok := m.filterLists[listID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("filter list %q not found", listID)
	}

	if fl.URL == "" {
		m.mu.Unlock()
		return fmt.Errorf("filter list %q has no subscription URL", listID)
	}

	url := fl.URL
	m.mu.Unlock()

	rules, err := m.downloadRules(url)
	if err != nil {
		return fmt.Errorf("failed to download rules: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for ruleID, rule := range m.rules {
		if rule.ListID == listID {
			delete(m.rules, ruleID)
		}
	}

	action := ActionBlock
	if fl.Type == FilterListAllow {
		action = ActionAllow
	}

	for _, pattern := range rules {
		rule := &FilterRule{
			ID:        uuid.New().String(),
			Pattern:   strings.ToLower(pattern),
			Action:    action,
			ListID:    listID,
			Enabled:   true,
			HitCount:  0,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.rules[rule.ID] = rule
	}

	fl.RuleCount = len(rules)
	now := time.Now()
	fl.LastUpdated = &now

	log.Printf("[dnsfilter] 订阅规则列表 %s: 加载 %d 条规则", fl.Name, len(rules))
	return nil
}

// downloadRules 下载规则列表.
func (m *Manager) downloadRules(url string) ([]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var rules []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "!") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}

		// 解析 AdGuard 格式 (||domain.com^)
		if strings.HasPrefix(line, "||") && strings.HasSuffix(line, "^") {
			domain := strings.TrimPrefix(line, "||")
			domain = strings.TrimSuffix(domain, "^")
			domain = strings.TrimSpace(domain)
			if domain != "" {
				rules = append(rules, domain)
			}
		}
	}

	return rules, scanner.Err()
}

// ========== 上游 DNS 管理 ==========

// CreateUpstreamDNS 创建上游 DNS.
func (m *Manager) CreateUpstreamDNS(req CreateUpstreamDNSRequest) *UpstreamDNS {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.IsDefault {
		for _, u := range m.upstreams {
			u.IsDefault = false
		}
	}

	upstream := &UpstreamDNS{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Address:   req.Address,
		Protocol:  req.Protocol,
		Enabled:   true,
		Weight:    req.Weight,
		IsDefault: req.IsDefault,
	}

	if upstream.Weight == 0 {
		upstream.Weight = 1
	}

	m.upstreams[upstream.ID] = upstream
	log.Printf("[dnsfilter] 创建上游 DNS: %s (%s)", upstream.Name, upstream.Address)
	return upstream
}

// ListUpstreamDNS 列出上游 DNS.
func (m *Manager) ListUpstreamDNS() []*UpstreamDNS {
	m.mu.RLock()
	defer m.mu.RUnlock()

	upstreams := make([]*UpstreamDNS, 0, len(m.upstreams))
	for _, u := range m.upstreams {
		upstreams = append(upstreams, u)
	}

	sort.Slice(upstreams, func(i, j int) bool {
		return upstreams[i].Name < upstreams[j].Name
	})

	return upstreams
}

// UpdateUpstreamDNS 更新上游 DNS.
func (m *Manager) UpdateUpstreamDNS(id string, req UpdateUpstreamDNSRequest) (*UpstreamDNS, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	upstream, ok := m.upstreams[id]
	if !ok {
		return nil, fmt.Errorf("upstream dns %q not found", id)
	}

	if req.Name != nil {
		upstream.Name = *req.Name
	}
	if req.Address != nil {
		upstream.Address = *req.Address
	}
	if req.Protocol != nil {
		upstream.Protocol = *req.Protocol
	}
	if req.Weight != nil {
		upstream.Weight = *req.Weight
	}
	if req.IsDefault != nil {
		if *req.IsDefault {
			for _, u := range m.upstreams {
				u.IsDefault = false
			}
		}
		upstream.IsDefault = *req.IsDefault
	}
	if req.Enabled != nil {
		upstream.Enabled = *req.Enabled
	}

	log.Printf("[dnsfilter] 更新上游 DNS: %s", upstream.Name)
	return upstream, nil
}

// DeleteUpstreamDNS 删除上游 DNS.
func (m *Manager) DeleteUpstreamDNS(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	upstream, ok := m.upstreams[id]
	if !ok {
		return fmt.Errorf("upstream dns %q not found", id)
	}

	if upstream.IsDefault {
		return fmt.Errorf("cannot delete default upstream dns")
	}

	delete(m.upstreams, id)
	log.Printf("[dnsfilter] 删除上游 DNS: %s", upstream.Name)
	return nil
}

// ========== 过滤策略管理 ==========

// CreateFilterPolicy 创建过滤策略.
func (m *Manager) CreateFilterPolicy(req CreateFilterPolicyRequest) *FilterPolicy {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	policy := &FilterPolicy{
		ID:           uuid.New().String(),
		Name:         req.Name,
		Description:  req.Description,
		ClientMAC:    req.ClientMAC,
		ClientIP:     req.ClientIP,
		StartTime:    req.StartTime,
		EndTime:      req.EndTime,
		Weekdays:     req.Weekdays,
		BlockListIDs: req.BlockListIDs,
		AllowListIDs: req.AllowListIDs,
		UpstreamIDs:  req.UpstreamIDs,
		Enabled:      true,
		Priority:     req.Priority,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	m.policies[policy.ID] = policy
	log.Printf("[dnsfilter] 创建过滤策略: %s", policy.Name)
	return policy
}

// ListFilterPolicies 列出过滤策略.
func (m *Manager) ListFilterPolicies() []*FilterPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*FilterPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}

	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Priority > policies[j].Priority
	})

	return policies
}

// UpdateFilterPolicy 更新过滤策略.
func (m *Manager) UpdateFilterPolicy(id string, req UpdateFilterPolicyRequest) (*FilterPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("filter policy %q not found", id)
	}

	if req.Name != nil {
		policy.Name = *req.Name
	}
	if req.Description != nil {
		policy.Description = *req.Description
	}
	if req.ClientMAC != nil {
		policy.ClientMAC = *req.ClientMAC
	}
	if req.ClientIP != nil {
		policy.ClientIP = *req.ClientIP
	}
	if req.StartTime != nil {
		policy.StartTime = *req.StartTime
	}
	if req.EndTime != nil {
		policy.EndTime = *req.EndTime
	}
	if req.Weekdays != nil {
		policy.Weekdays = req.Weekdays
	}
	if req.BlockListIDs != nil {
		policy.BlockListIDs = req.BlockListIDs
	}
	if req.AllowListIDs != nil {
		policy.AllowListIDs = req.AllowListIDs
	}
	if req.UpstreamIDs != nil {
		policy.UpstreamIDs = req.UpstreamIDs
	}
	if req.Enabled != nil {
		policy.Enabled = *req.Enabled
	}
	if req.Priority != nil {
		policy.Priority = *req.Priority
	}

	policy.UpdatedAt = time.Now()

	log.Printf("[dnsfilter] 更新过滤策略: %s", policy.Name)
	return policy, nil
}

// DeleteFilterPolicy 删除过滤策略.
func (m *Manager) DeleteFilterPolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, ok := m.policies[id]
	if !ok {
		return fmt.Errorf("filter policy %q not found", id)
	}

	delete(m.policies, id)
	log.Printf("[dnsfilter] 删除过滤策略: %s", policy.Name)
	return nil
}

// ========== DNS 解析逻辑 ==========

// ResolveDNS 解析 DNS 请求.
func (m *Manager) ResolveDNS(domain, queryType, clientIP, clientMAC string) *QueryLog {
	start := time.Now()
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))

	logEntry := &QueryLog{
		ID:        uuid.New().String(),
		Timestamp: start,
		ClientIP:  clientIP,
		ClientMAC: clientMAC,
		Domain:    domain,
		Type:      queryType,
		Action:    ActionAllow,
	}

	// 1. 检查自定义 DNS 记录
	if record := m.lookupCustomRecord(domain, queryType); record != nil {
		logEntry.Answer = record.Value
		logEntry.Action = ActionAllow
		logEntry.Duration = time.Since(start).Milliseconds()
		m.recordQuery(logEntry)
		return logEntry
	}

	// 2. 检查过滤规则
	match := m.checkFilterRules(domain, clientIP, clientMAC)
	if match.Matched {
		logEntry.IsFiltered = true
		logEntry.FilterRule = match.Rule
		logEntry.Action = match.Action
		logEntry.Duration = time.Since(start).Milliseconds()
		m.recordQuery(logEntry)
		return logEntry
	}

	// 3. 检查缓存
	if cached := m.lookupCache(domain, queryType); cached != nil {
		logEntry.Answer = strings.Join(cached.Answers, ",")
		logEntry.Action = ActionAllow
		logEntry.Duration = time.Since(start).Milliseconds()
		m.recordQuery(logEntry)
		return logEntry
	}

	// 4. 转发到上游 DNS
	answers, upstream := m.forwardToUpstream(domain, queryType, clientIP)
	logEntry.Answer = strings.Join(answers, ",")
	logEntry.Upstream = upstream
	logEntry.Action = ActionAllow

	if len(answers) > 0 {
		m.addToCache(domain, queryType, answers, 300)
	}

	logEntry.Duration = time.Since(start).Milliseconds()
	m.recordQuery(logEntry)
	return logEntry
}

// lookupCustomRecord 查找自定义 DNS 记录.
func (m *Manager) lookupCustomRecord(domain, queryType string) *DNSRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, record := range m.records {
		if record.Enabled && record.Name == domain && string(record.Type) == queryType {
			return record
		}
	}
	return nil
}

// checkFilterRules 检查过滤规则.
func (m *Manager) checkFilterRules(domain, clientIP, clientMAC string) filterMatch {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy := m.getPolicyForClient(clientIP, clientMAC)

	// 检查白名单（优先级最高）
	if policy != nil {
		for _, listID := range policy.AllowListIDs {
			if m.matchDomainInList(domain, listID) {
				return filterMatch{Matched: true, Action: ActionAllow, Rule: "policy allow", ListID: listID}
			}
		}
	}

	// 检查全局白名单
	for _, rule := range m.rules {
		if rule.Enabled && rule.Action == ActionAllow && matchDomain(domain, rule.Pattern) {
			rule.HitCount++
			return filterMatch{Matched: true, Action: ActionAllow, Rule: rule.Pattern, ListID: rule.ListID}
		}
	}

	// 检查黑名单
	if policy != nil {
		for _, listID := range policy.BlockListIDs {
			if m.matchDomainInList(domain, listID) {
				return filterMatch{Matched: true, Action: ActionBlock, Rule: "policy block", ListID: listID}
			}
		}
	}

	// 检查全局黑名单
	for _, rule := range m.rules {
		if rule.Enabled && rule.Action == ActionBlock && matchDomain(domain, rule.Pattern) {
			rule.HitCount++
			return filterMatch{Matched: true, Action: ActionBlock, Rule: rule.Pattern, ListID: rule.ListID}
		}
	}

	return filterMatch{Matched: false, Action: ActionAllow}
}

// matchDomainInList 检查域名是否匹配列表中的规则.
func (m *Manager) matchDomainInList(domain, listID string) bool {
	for _, rule := range m.rules {
		if rule.Enabled && rule.ListID == listID && matchDomain(domain, rule.Pattern) {
			rule.HitCount++
			return true
		}
	}
	return false
}

// getPolicyForClient 获取客户端匹配的策略.
func (m *Manager) getPolicyForClient(clientIP, clientMAC string) *FilterPolicy {
	var matchedPolicy *FilterPolicy
	highestPriority := -1

	for _, policy := range m.policies {
		if !policy.Enabled {
			continue
		}

		if policy.ClientIP != "" && policy.ClientIP != clientIP {
			continue
		}
		if policy.ClientMAC != "" && policy.ClientMAC != clientMAC {
			continue
		}

		if !m.isPolicyTimeActive(policy) {
			continue
		}

		if policy.Priority > highestPriority {
			highestPriority = policy.Priority
			matchedPolicy = policy
		}
	}

	return matchedPolicy
}

// isPolicyTimeActive 检查策略是否在生效时间段内.
func (m *Manager) isPolicyTimeActive(policy *FilterPolicy) bool {
	if policy.StartTime == "" && policy.EndTime == "" && len(policy.Weekdays) == 0 {
		return true
	}

	now := time.Now()

	if len(policy.Weekdays) > 0 {
		weekday := int(now.Weekday())
		found := false
		for _, d := range policy.Weekdays {
			if d == weekday {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if policy.StartTime != "" && policy.EndTime != "" {
		currentTime := now.Format("15:04")
		if currentTime < policy.StartTime || currentTime > policy.EndTime {
			return false
		}
	}

	return true
}

// matchDomain 匹配域名（支持通配符和正则）.
func matchDomain(domain, pattern string) bool {
	if domain == pattern {
		return true
	}

	if strings.HasSuffix(domain, "."+pattern) {
		return true
	}

	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*") // ".example.com"
		// 匹配 example.com 和 *.example.com
		if strings.HasSuffix(domain, suffix) || domain == strings.TrimPrefix(suffix, ".") {
			return true
		}
	}

	if matched, _ := regexp.MatchString(pattern, domain); matched {
		return true
	}

	return false
}

// forwardToUpstream 转发请求到上游 DNS.
func (m *Manager) forwardToUpstream(domain, queryType, clientIP string) ([]string, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var upstreams []*UpstreamDNS
	for _, u := range m.upstreams {
		if u.Enabled {
			upstreams = append(upstreams, u)
		}
	}

	if len(upstreams) == 0 {
		return nil, ""
	}

	upstream := upstreams[0]
	answers := []string{"0.0.0.0"}

	return answers, upstream.Name
}

// ========== DNS 缓存管理 ==========

// lookupCache 查找缓存.
func (m *Manager) lookupCache(domain, queryType string) *DNSCacheEntry {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()

	key := cacheKey{Domain: domain, Type: queryType}
	entry, ok := m.cache[key]
	if !ok {
		return nil
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil
	}

	return entry
}

// addToCache 添加到缓存.
func (m *Manager) addToCache(domain, queryType string, answers []string, ttl int) {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	key := cacheKey{Domain: domain, Type: queryType}
	m.cache[key] = &DNSCacheEntry{
		Domain:    domain,
		Type:      queryType,
		Answers:   answers,
		TTL:       ttl,
		ExpiresAt: time.Now().Add(time.Duration(ttl) * time.Second),
		CreatedAt: time.Now(),
	}
}

// invalidateCache 清除指定域名的缓存.
func (m *Manager) invalidateCache(domain, queryType string) {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	if queryType != "" {
		delete(m.cache, cacheKey{Domain: domain, Type: queryType})
	} else {
		for key := range m.cache {
			if key.Domain == domain {
				delete(m.cache, key)
			}
		}
	}
}

// ClearCache 清除所有缓存.
func (m *Manager) ClearCache() {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	m.cache = make(map[cacheKey]*DNSCacheEntry)
	log.Printf("[dnsfilter] 缓存已清除")
}

// GetCacheSize 获取缓存大小.
func (m *Manager) GetCacheSize() int {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()

	return len(m.cache)
}

// ========== 查询日志 ==========

// recordQuery 记录查询日志.
func (m *Manager) recordQuery(entry *QueryLog) {
	m.logMu.Lock()
	defer m.logMu.Unlock()

	m.queryLogs = append(m.queryLogs, entry)

	if len(m.queryLogs) > 10000 {
		m.queryLogs = m.queryLogs[len(m.queryLogs)-10000:]
	}

	event := LogStreamEvent{
		ID:         entry.ID,
		Timestamp:  entry.Timestamp,
		ClientIP:   entry.ClientIP,
		Domain:     entry.Domain,
		Type:       entry.Type,
		Answer:     entry.Answer,
		IsFiltered: entry.IsFiltered,
		Action:     entry.Action,
		Duration:   entry.Duration,
	}

	for _, ch := range m.logStream {
		select {
		case ch <- event:
		default:
		}
	}
}

// GetQueryLogs 获取查询日志.
func (m *Manager) GetQueryLogs(req QueryLogRequest) []*QueryLog {
	m.logMu.RLock()
	defer m.logMu.RUnlock()

	var logs []*QueryLog
	for _, entry := range m.queryLogs {
		if req.ClientIP != "" && entry.ClientIP != req.ClientIP {
			continue
		}

		if req.Domain != "" && !strings.Contains(entry.Domain, req.Domain) {
			continue
		}

		if req.Action != "" && string(entry.Action) != req.Action {
			continue
		}

		if req.Since != "" {
			since, err := time.Parse(time.RFC3339, req.Since)
			if err == nil && entry.Timestamp.Before(since) {
				continue
			}
		}

		logs = append(logs, entry)
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}

	if len(logs) > limit {
		logs = logs[len(logs)-limit:]
	}

	return logs
}

// SubscribeLogStream 订阅日志流.
func (m *Manager) SubscribeLogStream() chan LogStreamEvent {
	ch := make(chan LogStreamEvent, 100)

	m.logMu.Lock()
	m.logStream = append(m.logStream, ch)
	m.logMu.Unlock()

	return ch
}

// UnsubscribeLogStream 取消订阅日志流.
func (m *Manager) UnsubscribeLogStream(ch chan LogStreamEvent) {
	m.logMu.Lock()
	defer m.logMu.Unlock()

	for i, c := range m.logStream {
		if c == ch {
			m.logStream = append(m.logStream[:i], m.logStream[i+1:]...)
			close(ch)
			break
		}
	}
}

// ========== 统计信息 ==========

// GetStats 获取查询统计.
func (m *Manager) GetStats() *QueryStats {
	m.logMu.RLock()
	defer m.logMu.RUnlock()

	stats := &QueryStats{
		TopBlocked:  make([]DomainStat, 0),
		TopAllowed:  make([]DomainStat, 0),
		TopClients:  make([]ClientStat, 0),
		HourlyStats: make([]HourlyStat, 0),
	}

	blockedDomains := make(map[string]int64)
	clientStats := make(map[string]*ClientStat)
	hourlyStats := make(map[string]*HourlyStat)
	uniqueDomains := make(map[string]bool)
	uniqueClients := make(map[string]bool)

	for _, entry := range m.queryLogs {
		stats.TotalQueries++
		uniqueDomains[entry.Domain] = true
		uniqueClients[entry.ClientIP] = true

		if entry.IsFiltered {
			stats.BlockedQueries++
			blockedDomains[entry.Domain]++
		} else {
			stats.AllowedQueries++
		}

		if cs, ok := clientStats[entry.ClientIP]; ok {
			cs.Total++
			if entry.IsFiltered {
				cs.Blocked++
			} else {
				cs.Allowed++
			}
		} else {
			cs = &ClientStat{
				ClientIP:  entry.ClientIP,
				ClientMAC: entry.ClientMAC,
				Total:     1,
			}
			if entry.IsFiltered {
				cs.Blocked = 1
			} else {
				cs.Allowed = 1
			}
			clientStats[entry.ClientIP] = cs
		}

		hourKey := entry.Timestamp.Format("2006-01-02 15:00")
		if hs, ok := hourlyStats[hourKey]; ok {
			hs.Total++
			if entry.IsFiltered {
				hs.Blocked++
			} else {
				hs.Allowed++
			}
		} else {
			hs = &HourlyStat{
				Hour:  hourKey,
				Total: 1,
			}
			if entry.IsFiltered {
				hs.Blocked = 1
			} else {
				hs.Allowed = 1
			}
			hourlyStats[hourKey] = hs
		}
	}

	stats.UniqueDomains = len(uniqueDomains)
	stats.UniqueClients = len(uniqueClients)

	if stats.TotalQueries > 0 {
		stats.BlockRate = float64(stats.BlockedQueries) / float64(stats.TotalQueries) * 100
	}

	for domain, count := range blockedDomains {
		stats.TopBlocked = append(stats.TopBlocked, DomainStat{Domain: domain, Count: count})
	}
	sort.Slice(stats.TopBlocked, func(i, j int) bool {
		return stats.TopBlocked[i].Count > stats.TopBlocked[j].Count
	})
	if len(stats.TopBlocked) > 10 {
		stats.TopBlocked = stats.TopBlocked[:10]
	}

	for _, cs := range clientStats {
		stats.TopClients = append(stats.TopClients, *cs)
	}
	sort.Slice(stats.TopClients, func(i, j int) bool {
		return stats.TopClients[i].Total > stats.TopClients[j].Total
	})
	if len(stats.TopClients) > 10 {
		stats.TopClients = stats.TopClients[:10]
	}

	for _, hs := range hourlyStats {
		stats.HourlyStats = append(stats.HourlyStats, *hs)
	}
	sort.Slice(stats.HourlyStats, func(i, j int) bool {
		return stats.HourlyStats[i].Hour < stats.HourlyStats[j].Hour
	})

	return stats
}
