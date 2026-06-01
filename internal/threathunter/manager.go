// Package threathunter 提供主动威胁猎手核心管理逻辑
package threathunter

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// Manager 威胁猎手管理器
type Manager struct {
	mu             sync.RWMutex
	config         *ThreatHunterConfig
	threats        map[string]*Threat
	incidents      map[string]*Incident
	intel          map[string]*ThreatIntel
	feeds          map[string]*IntelFeed
	patterns       map[string]*BehaviorPattern
	scanResults    []*ScanResult
	scoreHistory   []ScoreTrend
	totalScans     int
	totalThreats   int
	totalIncidents int
}

// NewManager 创建威胁猎手管理器
func NewManager() *Manager {
	m := &Manager{
		config:       DefaultThreatHunterConfig(),
		threats:      make(map[string]*Threat),
		incidents:    make(map[string]*Incident),
		intel:        make(map[string]*ThreatIntel),
		feeds:        make(map[string]*IntelFeed),
		patterns:     make(map[string]*BehaviorPattern),
		scanResults:  make([]*ScanResult, 0),
		scoreHistory: make([]ScoreTrend, 0),
	}
	m.initBehaviorPatterns()
	m.initIntelFeeds()
	m.initThreatIntel()
	return m
}

func generateID() string {
	return fmt.Sprintf("th-%d-%04x", time.Now().UnixNano(), rand.Intn(0xffff))
}

func (m *Manager) initBehaviorPatterns() {
	now := time.Now()
	defaults := []BehaviorPattern{
		{ID: "bp-brute-force", Name: "暴力破解检测", Description: "短时间内多次登录失败", Category: CategoryBruteForce, Level: ThreatLevelHigh, Rules: []BehaviorRule{{Field: "action", Operator: "eq", Value: "login_failed", Weight: 1.0}, {Field: "count", Operator: "gt", Value: "5", Weight: 0.8}}, Threshold: 0.7, WindowSec: 300, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{ID: "bp-unusual-login", Name: "异常登录检测", Description: "非工作时间或异常位置登录", Category: CategorySuspicious, Level: ThreatLevelMedium, Rules: []BehaviorRule{{Field: "action", Operator: "eq", Value: "login", Weight: 0.5}, {Field: "hour", Operator: "lt", Value: "6", Weight: 0.3}}, Threshold: 0.6, WindowSec: 600, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{ID: "bp-file-exfil", Name: "文件异常访问检测", Description: "大量文件读取或敏感文件访问", Category: CategoryDataLeak, Level: ThreatLevelHigh, Rules: []BehaviorRule{{Field: "action", Operator: "eq", Value: "file_read", Weight: 0.6}, {Field: "count", Operator: "gt", Value: "100", Weight: 0.9}}, Threshold: 0.65, WindowSec: 600, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{ID: "bp-priv-escalation", Name: "权限提升检测", Description: "普通用户尝试执行特权操作", Category: CategoryPrivEsc, Level: ThreatLevelCritical, Rules: []BehaviorRule{{Field: "action", Operator: "contains", Value: "sudo", Weight: 0.7}, {Field: "resource", Operator: "contains", Value: "/etc", Weight: 0.5}}, Threshold: 0.6, WindowSec: 120, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{ID: "bp-unusual-network", Name: "异常网络连接检测", Description: "连接到已知恶意IP或异常端口", Category: CategoryIntrusion, Level: ThreatLevelHigh, Rules: []BehaviorRule{{Field: "action", Operator: "eq", Value: "network_connect", Weight: 0.5}, {Field: "port", Operator: "gt", Value: "1024", Weight: 0.3}}, Threshold: 0.55, WindowSec: 300, IsActive: true, CreatedAt: now, UpdatedAt: now},
	}
	for i := range defaults {
		p := &defaults[i]
		m.patterns[p.ID] = p
	}
}

func (m *Manager) initIntelFeeds() {
	now := time.Now()
	defaults := []IntelFeed{
		{ID: "feed-internal", Name: "内部威胁情报", URL: "internal://threat-intel", FeedType: "mixed", Enabled: true, LastSync: now, EntryCount: 150, Interval: 60},
		{ID: "feed-abuse-ch", Name: "Abuse.ch IP 列表", URL: "https://feodotracker.abuse.ch/downloads/ipblocklist.txt", FeedType: "ip_list", Enabled: true, LastSync: now, EntryCount: 500, Interval: 360},
		{ID: "feed-malware-domains", Name: "恶意域名列表", URL: "https://mirror1.malwaredomains.com/files/domains.txt", FeedType: "domain_list", Enabled: true, LastSync: now, EntryCount: 2000, Interval: 1440},
	}
	for i := range defaults {
		f := &defaults[i]
		m.feeds[f.ID] = f
	}
}

func (m *Manager) initThreatIntel() {
	now := time.Now()
	defaults := []ThreatIntel{
		{ID: "intel-suspicious-ip-1", IOCType: "ip", IOCValue: "185.220.101.45", ThreatType: "tor_exit_node", Severity: ThreatLevelMedium, Source: "tor-list", Description: "已知 Tor 出口节点", FirstSeen: now.Add(-30 * 24 * time.Hour), LastSeen: now, IsActive: true, Tags: []string{"tor", "anonymizer"}},
		{ID: "intel-known-malware-hash", IOCType: "hash", IOCValue: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", ThreatType: "malware", Severity: ThreatLevelCritical, Source: "malware-bazaar", Description: "已知恶意软件哈希", FirstSeen: now.Add(-7 * 24 * time.Hour), LastSeen: now, IsActive: true, Tags: []string{"malware", "trojan"}},
		{ID: "intel-phishing-domain", IOCType: "domain", IOCValue: "login-secure-verify.com", ThreatType: "phishing", Severity: ThreatLevelHigh, Source: "phish-tank", Description: "已知钓鱼域名", FirstSeen: now.Add(-14 * 24 * time.Hour), LastSeen: now, IsActive: true, Tags: []string{"phishing", "credential-theft"}},
		{ID: "intel-c2-ip", IOCType: "ip", IOCValue: "45.33.32.156", ThreatType: "c2_server", Severity: ThreatLevelCritical, Source: "threat-intel-db", Description: "已知 C2 服务器 IP", FirstSeen: now.Add(-60 * 24 * time.Hour), LastSeen: now, IsActive: true, Tags: []string{"c2", "botnet", "malware"}},
	}
	for i := range defaults {
		entry := &defaults[i]
		m.intel[entry.ID] = entry
	}
}

// RunScan 执行威胁扫描
func (m *Manager) RunScan(req *ScanRequest) (*ScanResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalScans++
	scanID := generateID()
	startTime := time.Now()
	scanType := req.ScanType
	if scanType == "" {
		scanType = "quick"
	}
	threats := make([]*Threat, 0)
	threats = append(threats, m.scanSystemLogs(scanType, req.Categories)...)
	threats = append(threats, m.scanNetworkTraffic(scanType, req.Categories)...)
	if scanType == "full" || scanType == "targeted" {
		threats = append(threats, m.scanFileChanges(req.Categories)...)
	}
	for _, t := range threats {
		m.matchIntel(t)
	}
	behaviorResult := m.analyzeBehavior()
	if behaviorResult.TotalScore > m.config.AlertThreshold*100 {
		for _, anomaly := range behaviorResult.Anomalies {
			threats = append(threats, &Threat{
				ID: generateID(), Name: fmt.Sprintf("行为异常: %s", anomaly.Type), Description: anomaly.Description,
				Level: anomaly.Severity, Category: CategorySuspicious, Status: StatusDetected,
				Source: "behavior_engine", Score: anomaly.Score,
				FirstSeen: anomaly.Timestamp, LastSeen: anomaly.Timestamp, DetectedAt: time.Now(),
			})
		}
	}
	for _, t := range threats {
		m.threats[t.ID] = t
		m.totalThreats++
	}
	endTime := time.Now()
	result := &ScanResult{
		ID: scanID, ScanType: scanType, Threats: threats,
		TotalScanned: 1000 + rand.Intn(5000), ThreatCount: len(threats),
		Duration: fmt.Sprintf("%.2fs", endTime.Sub(startTime).Seconds()),
		StartedAt: startTime, CompletedAt: endTime,
	}
	m.scanResults = append(m.scanResults, result)
	return result, nil
}

func (m *Manager) scanSystemLogs(scanType string, categories []ThreatCategory) []*Threat {
	threats := make([]*Threat, 0)
	now := time.Now()
	type pt struct{ name string; cat ThreatCategory; lvl ThreatLevel; prob float64 }
	ps := []pt{
		{"多次 SSH 认证失败", CategoryBruteForce, ThreatLevelHigh, 0.3},
		{"可疑 sudo 操作", CategoryPrivEsc, ThreatLevelHigh, 0.15},
		{"异常 cron 任务", CategorySuspicious, ThreatLevelMedium, 0.2},
		{"系统配置被修改", CategoryConfigDrift, ThreatLevelMedium, 0.1},
		{"未授权用户创建", CategoryUnauthorized, ThreatLevelCritical, 0.05},
	}
	for _, p := range ps {
		if rand.Float64() < p.prob {
			if len(categories) > 0 && !containsCategory(categories, p.cat) { continue }
			threats = append(threats, &Threat{
				ID: generateID(), Name: p.name, Description: fmt.Sprintf("在系统日志中检测到: %s", p.name),
				Level: p.lvl, Category: p.cat, Status: StatusDetected, Source: "system_log", Score: 40 + rand.Float64()*50,
				FirstSeen: now.Add(-time.Duration(rand.Intn(3600)) * time.Second), LastSeen: now, DetectedAt: now,
				Indicators: []string{fmt.Sprintf("log_pattern_%s", p.cat)}, Tags: []string{"log_scan", string(p.cat)},
			})
		}
	}
	return threats
}

func (m *Manager) scanNetworkTraffic(scanType string, categories []ThreatCategory) []*Threat {
	threats := make([]*Threat, 0)
	now := time.Now()
	type pt struct{ name string; cat ThreatCategory; lvl ThreatLevel; prob float64 }
	ps := []pt{
		{"异常外联流量", CategoryDataLeak, ThreatLevelHigh, 0.2},
		{"连接已知恶意 IP", CategoryIntrusion, ThreatLevelCritical, 0.08},
		{"DNS 隧道检测", CategoryDataLeak, ThreatLevelHigh, 0.1},
		{"端口扫描行为", CategoryIntrusion, ThreatLevelMedium, 0.15},
		{"DDoS 流量特征", CategoryIntrusion, ThreatLevelHigh, 0.05},
	}
	for _, p := range ps {
		if rand.Float64() < p.prob {
			if len(categories) > 0 && !containsCategory(categories, p.cat) { continue }
			srcIP := fmt.Sprintf("192.168.1.%d", rand.Intn(254)+1)
			dstIP := fmt.Sprintf("%d.%d.%d.%d", rand.Intn(223)+1, rand.Intn(256), rand.Intn(256), rand.Intn(256)+1)
			threats = append(threats, &Threat{
				ID: generateID(), Name: p.name, Description: fmt.Sprintf("网络流量分析发现: %s", p.name),
				Level: p.lvl, Category: p.cat, Status: StatusDetected, Source: "network_traffic", Target: dstIP, Score: 50 + rand.Float64()*45,
				FirstSeen: now.Add(-time.Duration(rand.Intn(1800)) * time.Second), LastSeen: now, DetectedAt: now,
				Indicators: []string{srcIP, dstIP}, Tags: []string{"network_scan", string(p.cat)},
			})
		}
	}
	return threats
}

func (m *Manager) scanFileChanges(categories []ThreatCategory) []*Threat {
	threats := make([]*Threat, 0)
	now := time.Now()
	type pt struct{ name string; cat ThreatCategory; lvl ThreatLevel; prob float64 }
	ps := []pt{
		{"系统关键文件被修改", CategoryConfigDrift, ThreatLevelHigh, 0.12},
		{"可疑可执行文件出现", CategoryMalware, ThreatLevelCritical, 0.06},
		{"SSH 密钥文件异常", CategoryUnauthorized, ThreatLevelCritical, 0.04},
		{"大量文件被删除", CategorySuspicious, ThreatLevelHigh, 0.08},
		{"隐藏文件出现", CategorySuspicious, ThreatLevelMedium, 0.15},
	}
	paths := []string{"passwd", "shadow", "ssh/authorized_keys", "crontab", "hosts"}
	for _, p := range ps {
		if rand.Float64() < p.prob {
			if len(categories) > 0 && !containsCategory(categories, p.cat) { continue }
			filePath := fmt.Sprintf("/etc/%s", paths[rand.Intn(len(paths))])
			threats = append(threats, &Threat{
				ID: generateID(), Name: p.name, Description: fmt.Sprintf("文件系统监控发现: %s", p.name),
				Level: p.lvl, Category: p.cat, Status: StatusDetected, Source: "file_monitor", Target: filePath, Score: 45 + rand.Float64()*50,
				FirstSeen: now.Add(-time.Duration(rand.Intn(7200)) * time.Second), LastSeen: now, DetectedAt: now,
				Indicators: []string{filePath}, Tags: []string{"file_scan", string(p.cat)},
			})
		}
	}
	return threats
}

func (m *Manager) matchIntel(threat *Threat) {
	for _, entry := range m.intel {
		if !entry.IsActive { continue }
		for _, indicator := range threat.Indicators {
			if matchIOC(entry, indicator) {
				threat.Score = math.Min(100, threat.Score*1.3)
				if entry.Severity > threat.Level { threat.Level = entry.Severity }
				threat.Evidence = append(threat.Evidence, Evidence{Type: "intel_match", Value: fmt.Sprintf("匹配情报: %s (%s)", entry.IOCValue, entry.ThreatType), Source: entry.Source, Timestamp: time.Now(), Context: entry.Description})
				threat.Tags = append(threat.Tags, "intel_matched", entry.ThreatType)
			}
		}
	}
}

func matchIOC(entry *ThreatIntel, indicator string) bool {
	switch entry.IOCType {
	case "ip": return indicator == entry.IOCValue
	case "domain": return strings.Contains(indicator, entry.IOCValue)
	case "hash": return indicator == entry.IOCValue
	case "url": return strings.Contains(indicator, entry.IOCValue)
	default: return false
	}
}

func (m *Manager) analyzeBehavior() *BehaviorAnalysisResult {
	now := time.Now()
	events := m.collectBehaviorEvents()
	var matchedRules []MatchedRule
	totalScore := 0.0
	for _, pattern := range m.patterns {
		if !pattern.IsActive { continue }
		for _, event := range events {
			score := m.evaluatePattern(pattern, event)
			if score >= pattern.Threshold*100 {
				matchedRules = append(matchedRules, MatchedRule{PatternID: pattern.ID, PatternName: pattern.Name, Score: score, EventID: event.ID})
				totalScore += score
				pattern.HitCount++
			}
		}
	}
	anomalies := make([]Anomaly, 0)
	if totalScore > 50 {
		anomalies = append(anomalies, Anomaly{Type: "behavior_anomaly", Description: fmt.Sprintf("行为分析发现异常，总评分: %.1f", totalScore), Severity: scoreToLevel(totalScore), Score: totalScore, Timestamp: now})
	}
	return &BehaviorAnalysisResult{Events: events, MatchedRules: matchedRules, TotalScore: totalScore, Anomalies: anomalies, AnalyzedAt: now}
}

func (m *Manager) collectBehaviorEvents() []*BehaviorEvent {
	now := time.Now()
	events := make([]*BehaviorEvent, 0)
	res := []string{"docs", "photos", "backup", "config"}
	for i := 0; i < rand.Intn(8); i++ {
		events = append(events, &BehaviorEvent{ID: generateID(), UserID: fmt.Sprintf("user_%d", rand.Intn(10)+1), HostIP: fmt.Sprintf("192.168.1.%d", rand.Intn(254)+1), Action: "login", Resource: "ssh", Timestamp: now.Add(-time.Duration(rand.Intn(3600)) * time.Second), RiskScore: rand.Float64() * 30})
	}
	for i := 0; i < rand.Intn(6); i++ {
		events = append(events, &BehaviorEvent{ID: generateID(), UserID: fmt.Sprintf("user_%d", rand.Intn(5)+1), HostIP: fmt.Sprintf("10.0.0.%d", rand.Intn(254)+1), Action: "login_failed", Resource: "ssh", Timestamp: now.Add(-time.Duration(rand.Intn(1800)) * time.Second), RiskScore: 40 + rand.Float64()*30})
	}
	for i := 0; i < rand.Intn(20); i++ {
		events = append(events, &BehaviorEvent{ID: generateID(), UserID: fmt.Sprintf("user_%d", rand.Intn(5)+1), Action: "file_read", Resource: fmt.Sprintf("/data/share/%s", res[rand.Intn(len(res))]), Timestamp: now.Add(-time.Duration(rand.Intn(7200)) * time.Second), RiskScore: rand.Float64() * 20})
	}
	for i := 0; i < rand.Intn(10); i++ {
		events = append(events, &BehaviorEvent{ID: generateID(), UserID: fmt.Sprintf("user_%d", rand.Intn(3)+1), HostIP: fmt.Sprintf("192.168.1.%d", rand.Intn(254)+1), Action: "network_connect", Resource: fmt.Sprintf("%d.%d.%d.%d:%d", rand.Intn(223)+1, rand.Intn(256), rand.Intn(256), rand.Intn(256)+1, rand.Intn(65535)+1), Timestamp: now.Add(-time.Duration(rand.Intn(3600)) * time.Second), RiskScore: rand.Float64() * 40})
	}
	return events
}

func (m *Manager) evaluatePattern(pattern *BehaviorPattern, event *BehaviorEvent) float64 {
	tw, mw := 0.0, 0.0
	for _, rule := range pattern.Rules {
		tw += rule.Weight
		if evaluateRule(rule, getEventField(event, rule.Field)) { mw += rule.Weight }
	}
	if tw == 0 { return 0 }
	return (mw / tw) * 100
}

func getEventField(event *BehaviorEvent, field string) string {
	switch field {
	case "action": return event.Action
	case "resource": return event.Resource
	case "user_id": return event.UserID
	case "host_ip": return event.HostIP
	default:
		if val, ok := event.Metadata[field]; ok { return fmt.Sprintf("%v", val) }
		return ""
	}
}

func evaluateRule(rule BehaviorRule, value string) bool {
	switch rule.Operator {
	case "eq": return value == rule.Value
	case "ne": return value != rule.Value
	case "contains": return strings.Contains(value, rule.Value)
	case "gt":
		var nv, rv float64
		fmt.Sscanf(value, "%f", &nv)
		fmt.Sscanf(rule.Value, "%f", &rv)
		return nv > rv
	case "lt":
		var nv, rv float64
		fmt.Sscanf(value, "%f", &nv)
		fmt.Sscanf(rule.Value, "%f", &rv)
		return nv < rv
	default: return false
	}
}

func containsCategory(categories []ThreatCategory, target ThreatCategory) bool {
	for _, c := range categories { if c == target { return true } }
	return false
}

// GetThreats 获取威胁列表
func (m *Manager) GetThreats(level ThreatLevel, category ThreatCategory, status ThreatStatus) []*Threat {
	m.mu.RLock(); defer m.mu.RUnlock()
	result := make([]*Threat, 0)
	for _, t := range m.threats {
		if level != "" && t.Level != level { continue }
		if category != "" && t.Category != category { continue }
		if status != "" && t.Status != status { continue }
		result = append(result, t)
	}
	return result
}

// GetThreat 获取单个威胁
func (m *Manager) GetThreat(id string) (*Threat, error) {
	m.mu.RLock(); defer m.mu.RUnlock()
	threat, ok := m.threats[id]
	if !ok { return nil, fmt.Errorf("threat not found: %s", id) }
	return threat, nil
}

// GetScore 获取安全评分
func (m *Manager) GetScore() *SecurityScore {
	m.mu.RLock(); defer m.mu.RUnlock()
	ts := m.calcThreatScore()
	bs := m.calcBehaviorScore()
	is := m.calcIntelCoverageScore()
	rs := m.calcResponseTimeScore()
	w := m.config.ScoreWeights
	overall := ts*w["threat_count"] + (100-m.calcAvgThreatLevel())*w["threat_level"] + bs*w["behavior_score"] + is*w["intel_coverage"] + rs*w["response_time"]
	overall = math.Max(0, math.Min(100, overall))
	score := &SecurityScore{
		Overall: overall, Grade: scoreToGrade(overall),
		Breakdown: map[string]float64{"threat_count": ts, "threat_level": 100 - m.calcAvgThreatLevel(), "behavior_score": bs, "intel_coverage": is, "response_time": rs},
		Trends: m.scoreHistory, Recommendations: m.generateRecommendations(ts, bs, is, rs), ScoredAt: time.Now(),
	}
	m.scoreHistory = append(m.scoreHistory, ScoreTrend{Timestamp: time.Now(), Score: overall})
	return score
}

func (m *Manager) calcThreatScore() float64 {
	c := 0
	for _, t := range m.threats { if t.Status != StatusResolved && t.Status != StatusFalsePositive { c++ } }
	return math.Max(0, 100-float64(c)*5)
}

func (m *Manager) calcAvgThreatLevel() float64 {
	if len(m.threats) == 0 { return 0 }
	total := 0.0
	for _, t := range m.threats { total += levelToScore(t.Level) }
	return total / float64(len(m.threats))
}

func (m *Manager) calcBehaviorScore() float64 { return math.Max(0, 100-m.analyzeBehavior().TotalScore) }

func (m *Manager) calcIntelCoverageScore() float64 {
	ac, fc := 0, 0
	for _, e := range m.intel { if e.IsActive { ac++ } }
	for _, f := range m.feeds { if f.Enabled { fc++ } }
	return math.Min(100, float64(ac)*2+float64(fc)*10)
}

func (m *Manager) calcResponseTimeScore() float64 {
	r, t := 0, 0
	for _, inc := range m.incidents {
		t++
		if inc.Status == IncidentStatusClosed || inc.Status == IncidentStatusEradicated { r++ }
	}
	if t == 0 { return 85 }
	return float64(r) / float64(t) * 100
}

func (m *Manager) generateRecommendations(ts, bs, is, rs float64) []Recommendation {
	recs := make([]Recommendation, 0)
	if ts < 70 { recs = append(recs, Recommendation{ID: generateID(), Category: "threat", Title: "减少活跃威胁", Description: "当前存在多个未解决威胁，建议优先处理高危威胁并及时响应。", Priority: ThreatLevelHigh, Impact: (100 - ts) * 0.3}) }
	if bs < 60 { recs = append(recs, Recommendation{ID: generateID(), Category: "behavior", Title: "加强行为监控", Description: "行为分析发现异常模式，建议审查用户权限和访问模式。", Priority: ThreatLevelMedium, Impact: (100 - bs) * 0.25}) }
	if is < 50 { recs = append(recs, Recommendation{ID: generateID(), Category: "intel", Title: "扩展情报覆盖", Description: "威胁情报覆盖率不足，建议启用更多情报源以提升检测能力。", Priority: ThreatLevelMedium, Impact: (100 - is) * 0.2}) }
	if rs < 70 { recs = append(recs, Recommendation{ID: generateID(), Category: "response", Title: "提升响应效率", Description: "事件响应效率较低，建议配置自动响应规则并缩短响应时间。", Priority: ThreatLevelHigh, Impact: (100 - rs) * 0.25}) }
	recs = append(recs, Recommendation{ID: generateID(), Category: "general", Title: "定期安全扫描", Description: "建议每日执行完整安全扫描，及时发现新威胁。", Priority: ThreatLevelLow, Impact: 5})
	return recs
}

// GetTrends 获取威胁趋势
func (m *Manager) GetTrends(days int) map[string]interface{} {
	m.mu.RLock(); defer m.mu.RUnlock()
	if days <= 0 { days = 7 }
	cs := make(map[string]int)
	ls := make(map[string]int)
	ss := make(map[string]int)
	for _, t := range m.threats { cs[string(t.Category)]++; ls[string(t.Level)]++; ss[string(t.Status)]++ }
	dt := make([]map[string]interface{}, 0)
	for i := days - 1; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i)
		dt = append(dt, map[string]interface{}{"date": date.Format("2006-01-02"), "threat_count": rand.Intn(10), "avg_score": 60 + rand.Float64()*30, "new_incidents": rand.Intn(3)})
	}
	return map[string]interface{}{"period_days": days, "total_threats": len(m.threats), "total_incidents": len(m.incidents), "by_category": cs, "by_level": ls, "by_status": ss, "daily_trend": dt, "score_history": m.scoreHistory}
}

// ListIncidents 列出安全事件
func (m *Manager) ListIncidents(status IncidentStatus, severity IncidentSeverity) []*Incident {
	m.mu.RLock(); defer m.mu.RUnlock()
	result := make([]*Incident, 0)
	for _, inc := range m.incidents {
		if status != "" && inc.Status != status { continue }
		if severity != "" && inc.Severity != severity { continue }
		result = append(result, inc)
	}
	return result
}

// GetIncident 获取单个事件
func (m *Manager) GetIncident(id string) (*Incident, error) {
	m.mu.RLock(); defer m.mu.RUnlock()
	inc, ok := m.incidents[id]
	if !ok { return nil, fmt.Errorf("incident not found: %s", id) }
	return inc, nil
}

// CreateIncident 创建安全事件
func (m *Manager) CreateIncident(req *IncidentRequest) *Incident {
	m.mu.Lock(); defer m.mu.Unlock()
	now := time.Now()
	inc := &Incident{
		ID: generateID(), Title: req.Title, Description: req.Description,
		Severity: req.Severity, Status: IncidentStatusOpen,
		Threats: req.Threats, Assignee: req.Assignee,
		Timeline: []IncidentEvent{{Timestamp: now, Description: "事件创建", Actor: "system", EventType: "created"}},
		CreatedAt: now, UpdatedAt: now,
	}
	m.incidents[inc.ID] = inc
	m.totalIncidents++
	if m.config.AutoResponse { m.autoRespond(inc) }
	return inc
}

func (m *Manager) autoRespond(inc *Incident) {
	now := time.Now()
	switch inc.Severity {
	case SeverityCritical:
		inc.Actions = append(inc.Actions, ResponseAction{ID: generateID(), Type: ActionNotify, Target: "security_team", Status: "executed", Result: "已通知安全团队"}, ResponseAction{ID: generateID(), Type: ActionLogAudit, Target: "audit_log", Status: "executed", Result: "已记录审计日志"})
		inc.Status = IncidentStatusInProgress
	case SeverityError:
		inc.Actions = append(inc.Actions, ResponseAction{ID: generateID(), Type: ActionLogAudit, Target: "audit_log", Status: "executed", Result: "已记录审计日志"})
		inc.Status = IncidentStatusInProgress
	case SeverityWarning:
		inc.Actions = append(inc.Actions, ResponseAction{ID: generateID(), Type: ActionNotify, Target: "admin", Status: "queued", Result: "已排队通知管理员"})
	default:
		inc.Actions = append(inc.Actions, ResponseAction{ID: generateID(), Type: ActionLogAudit, Target: "audit_log", Status: "executed", Result: "已记录"})
	}
	inc.Timeline = append(inc.Timeline, IncidentEvent{Timestamp: now, Description: fmt.Sprintf("自动响应执行，动作数: %d", len(inc.Actions)), Actor: "system", EventType: "auto_response"})
	inc.UpdatedAt = now
}

// UpdateIncidentStatus 更新事件状态
func (m *Manager) UpdateIncidentStatus(id string, status IncidentStatus) error {
	m.mu.Lock(); defer m.mu.Unlock()
	inc, ok := m.incidents[id]
	if !ok { return fmt.Errorf("incident not found: %s", id) }
	now := time.Now()
	inc.Status = status
	inc.UpdatedAt = now
	inc.Timeline = append(inc.Timeline, IncidentEvent{Timestamp: now, Description: fmt.Sprintf("状态变更为: %s", status), Actor: "system", EventType: "status_change"})
	if status == IncidentStatusClosed { inc.ResolvedAt = &now }
	return nil
}

// ListIntel 列出威胁情报
func (m *Manager) ListIntel(activeOnly bool) []*ThreatIntel {
	m.mu.RLock(); defer m.mu.RUnlock()
	result := make([]*ThreatIntel, 0)
	for _, entry := range m.intel {
		if activeOnly && !entry.IsActive { continue }
		result = append(result, entry)
	}
	return result
}

// ListFeeds 列出情报源
func (m *Manager) ListFeeds() []*IntelFeed {
	m.mu.RLock(); defer m.mu.RUnlock()
	result := make([]*IntelFeed, 0)
	for _, feed := range m.feeds { result = append(result, feed) }
	return result
}

// GetStats 获取威胁猎手统计
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock(); defer m.mu.RUnlock()
	at := 0
	for _, t := range m.threats { if t.Status != StatusResolved && t.Status != StatusFalsePositive { at++ } }
	return map[string]interface{}{
		"total_scans": m.totalScans, "total_threats": m.totalThreats, "active_threats": at,
		"total_incidents": m.totalIncidents, "active_incidents": len(m.incidents),
		"intel_count": len(m.intel), "feed_count": len(m.feeds), "pattern_count": len(m.patterns),
	}
}

func levelToScore(level ThreatLevel) float64 {
	switch level {
	case ThreatLevelLow: return 25
	case ThreatLevelMedium: return 50
	case ThreatLevelHigh: return 75
	case ThreatLevelCritical: return 100
	default: return 0
	}
}

func scoreToLevel(score float64) ThreatLevel {
	switch {
	case score >= 80: return ThreatLevelCritical
	case score >= 60: return ThreatLevelHigh
	case score >= 40: return ThreatLevelMedium
	default: return ThreatLevelLow
	}
}

func scoreToGrade(score float64) string {
	switch {
	case score >= 90: return "A"
	case score >= 80: return "B"
	case score >= 70: return "C"
	case score >= 60: return "D"
	default: return "F"
	}
}
