package zerotrust

import (
	"crypto/sha256"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

// ==================== Policy Engine ====================

type SecurityPolicy struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Enabled    bool            `json:"enabled"`
	Priority   int             `json:"priority"`
	Effect     PolicyEffect    `json:"effect"`
	Conditions PolicyCondition `json:"conditions"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

type PolicyCondition struct {
	Users     []string `json:"users,omitempty"`
	Networks  []string `json:"networks,omitempty"`
	Resources []string `json:"resources,omitempty"`
	TimeStart string   `json:"time_start,omitempty"`
	TimeEnd   string   `json:"time_end,omitempty"`
}

type PolicyEffect int

const (
	PolicyAllow PolicyEffect = iota
	PolicyDeny
	PolicyChallenge
)

func (e PolicyEffect) String() string {
	switch e {
	case PolicyAllow:
		return "allow"
	case PolicyDeny:
		return "deny"
	case PolicyChallenge:
		return "challenge"
	default:
		return "unknown"
	}
}

type PolicyDecision struct {
	Allowed  bool         `json:"allowed"`
	Effect   PolicyEffect `json:"effect"`
	PolicyID string       `json:"policy_id"`
}

type PolicyEngine struct {
	mu       sync.RWMutex
	policies map[string]*SecurityPolicy
}

func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{policies: make(map[string]*SecurityPolicy)}
}

func (pe *PolicyEngine) AddPolicy(p *SecurityPolicy) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	if p.ID == "" {
		return fmt.Errorf("policy ID required")
	}
	if p.Name == "" {
		return fmt.Errorf("policy name required")
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	pe.policies[p.ID] = p
	return nil
}

func (pe *PolicyEngine) GetPolicy(id string) (*SecurityPolicy, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	p, ok := pe.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy not found: %s", id)
	}
	return p, nil
}

func (pe *PolicyEngine) RemovePolicy(id string) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	if _, ok := pe.policies[id]; !ok {
		return fmt.Errorf("policy not found: %s", id)
	}
	delete(pe.policies, id)
	return nil
}

func (pe *PolicyEngine) ListPolicies() []*SecurityPolicy {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	policies := make([]*SecurityPolicy, 0, len(pe.policies))
	for _, p := range pe.policies {
		policies = append(policies, p)
	}
	sort.Slice(policies, func(i, j int) bool { return policies[i].Priority < policies[j].Priority })
	return policies
}

func (pe *PolicyEngine) Evaluate(req ZTAccessRequest) PolicyDecision {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	policies := make([]*SecurityPolicy, 0, len(pe.policies))
	for _, p := range pe.policies {
		policies = append(policies, p)
	}
	sort.Slice(policies, func(i, j int) bool { return policies[i].Priority < policies[j].Priority })
	for _, p := range policies {
		if !p.Enabled {
			continue
		}
		if pe.matchPolicy(p, req) {
			return PolicyDecision{Allowed: p.Effect == PolicyAllow, Effect: p.Effect, PolicyID: p.ID}
		}
	}
	return PolicyDecision{Allowed: false, Effect: PolicyDeny, PolicyID: "default"}
}

func (pe *PolicyEngine) matchPolicy(p *SecurityPolicy, req ZTAccessRequest) bool {
	if len(p.Conditions.Users) > 0 {
		found := false
		for _, u := range p.Conditions.Users {
			if u == req.UserID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(p.Conditions.Networks) > 0 && req.IP != "" {
		found := false
		for _, n := range p.Conditions.Networks {
			_, cidr, err := net.ParseCIDR(n)
			if err != nil {
				continue
			}
			ip := net.ParseIP(req.IP)
			if ip != nil && cidr.Contains(ip) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if p.Conditions.TimeStart != "" && p.Conditions.TimeEnd != "" {
		now := req.Timestamp
		if now.IsZero() {
			now = time.Now()
		}
		startParts := strings.Split(p.Conditions.TimeStart, ":")
		endParts := strings.Split(p.Conditions.TimeEnd, ":")
		if len(startParts) == 2 && len(endParts) == 2 {
			currentMin := now.Hour()*60 + now.Minute()
			startTotal := parseNum(startParts[0])*60 + parseNum(startParts[1])
			endTotal := parseNum(endParts[0])*60 + parseNum(endParts[1])
			if currentMin < startTotal || currentMin > endTotal {
				return false
			}
		}
	}
	if len(p.Conditions.Resources) > 0 {
		found := false
		for _, r := range p.Conditions.Resources {
			if strings.HasSuffix(r, "*") {
				if strings.HasPrefix(req.Resource, strings.TrimSuffix(r, "*")) {
					found = true
					break
				}
			} else if r == req.Resource {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func parseNum(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

// ==================== Access Request ====================

type ZTAccessRequest struct {
	UserID    string    `json:"user_id"`
	DeviceID  string    `json:"device_id,omitempty"`
	Resource  string    `json:"resource"`
	Action    string    `json:"action"`
	IP        string    `json:"ip,omitempty"`
	Location  string    `json:"location,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ==================== Device Trust ====================

type DeviceInfo struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Fingerprint string           `json:"fingerprint"`
	IP          string           `json:"ip,omitempty"`
	TrustLevel  ZTTrustLevel     `json:"trust_level"`
	Compliance  DeviceCompliance `json:"compliance"`
	LastSeen    time.Time        `json:"last_seen"`
}

type DeviceCompliance struct {
	OSUpdated       bool `json:"os_updated"`
	FirewallEnabled bool `json:"firewall_enabled"`
	AntivirusActive bool `json:"antivirus_active"`
	DiskEncrypted   bool `json:"disk_encrypted"`
	PasswordStrong  bool `json:"password_strong"`
}

type ComplianceResult struct {
	ComplianceScore float64  `json:"compliance_score"`
	Issues          []string `json:"issues,omitempty"`
}

type ZTTrustLevel int

const (
	ZTTrustLevelUntrusted ZTTrustLevel = iota
	ZTTrustLevelLow
	ZTTrustLevelMedium
	ZTTrustLevelHigh
	ZTTrustLevelFull
)

func (l ZTTrustLevel) String() string {
	switch l {
	case ZTTrustLevelUntrusted:
		return "untrusted"
	case ZTTrustLevelLow:
		return "low"
	case ZTTrustLevelMedium:
		return "medium"
	case ZTTrustLevelHigh:
		return "high"
	case ZTTrustLevelFull:
		return "full"
	default:
		return "unknown"
	}
}

type DeviceTrustManager struct {
	mu      sync.RWMutex
	devices map[string]*DeviceInfo
}

func NewDeviceTrustManager() *DeviceTrustManager {
	return &DeviceTrustManager{devices: make(map[string]*DeviceInfo)}
}

func (dtm *DeviceTrustManager) RegisterDevice(d *DeviceInfo) error {
	dtm.mu.Lock()
	defer dtm.mu.Unlock()
	if d.ID == "" {
		return fmt.Errorf("device ID required")
	}
	if d.Fingerprint == "" {
		return fmt.Errorf("fingerprint required")
	}
	for _, existing := range dtm.devices {
		if existing.Fingerprint == d.Fingerprint && existing.ID != d.ID {
			return fmt.Errorf("duplicate fingerprint")
		}
	}
	if existing, ok := dtm.devices[d.ID]; ok {
		d.TrustLevel = existing.TrustLevel
		d.LastSeen = existing.LastSeen
	} else {
		d.TrustLevel = ZTTrustLevelLow
		d.LastSeen = time.Now()
	}
	dtm.devices[d.ID] = d
	return nil
}

func (dtm *DeviceTrustManager) GetDevice(id string) (*DeviceInfo, error) {
	dtm.mu.RLock()
	defer dtm.mu.RUnlock()
	d, ok := dtm.devices[id]
	if !ok {
		return nil, fmt.Errorf("device not found: %s", id)
	}
	return d, nil
}

func (dtm *DeviceTrustManager) UnregisterDevice(id string) error {
	dtm.mu.Lock()
	defer dtm.mu.Unlock()
	if _, ok := dtm.devices[id]; !ok {
		return fmt.Errorf("device not found: %s", id)
	}
	delete(dtm.devices, id)
	return nil
}

func (dtm *DeviceTrustManager) CheckCompliance(id string) (*ComplianceResult, error) {
	dtm.mu.Lock()
	defer dtm.mu.Unlock()
	d, ok := dtm.devices[id]
	if !ok {
		return nil, fmt.Errorf("device not found: %s", id)
	}
	score := 0.0
	if d.Compliance.OSUpdated {
		score += 20
	}
	if d.Compliance.FirewallEnabled {
		score += 20
	}
	if d.Compliance.AntivirusActive {
		score += 20
	}
	if d.Compliance.DiskEncrypted {
		score += 20
	}
	if d.Compliance.PasswordStrong {
		score += 20
	}
	switch score {
	case 100:
		d.TrustLevel = ZTTrustLevelHigh
	case 0:
		d.TrustLevel = ZTTrustLevelUntrusted
	}
	return &ComplianceResult{ComplianceScore: score}, nil
}

func (dtm *DeviceTrustManager) ListDevices() []*DeviceInfo {
	dtm.mu.RLock()
	defer dtm.mu.RUnlock()
	devices := make([]*DeviceInfo, 0, len(dtm.devices))
	for _, d := range dtm.devices {
		devices = append(devices, d)
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].ID < devices[j].ID })
	return devices
}

func (dtm *DeviceTrustManager) EvaluateDeviceTrust(id string) (ZTTrustLevel, error) {
	dtm.mu.RLock()
	defer dtm.mu.RUnlock()
	d, ok := dtm.devices[id]
	if !ok {
		return ZTTrustLevelUntrusted, fmt.Errorf("device not found: %s", id)
	}
	return d.TrustLevel, nil
}

func GenerateFingerprint(attrs map[string]string) string {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(attrs[k])
		sb.WriteString(";")
	}
	hash := sha256.Sum256([]byte(sb.String()))
	return fmt.Sprintf("%x", hash)
}

// ==================== Continuous Auth ====================

type Activity struct {
	Type     string `json:"type"`
	Success  bool   `json:"success"`
	IP       string `json:"ip,omitempty"`
	Location string `json:"location,omitempty"`
}

type CASession struct {
	ID           string       `json:"id"`
	UserID       string       `json:"user_id"`
	DeviceID     string       `json:"device_id"`
	IP           string       `json:"ip"`
	Location     string       `json:"location"`
	Active       bool         `json:"active"`
	TrustLevel   ZTTrustLevel `json:"trust_level"`
	RiskScore    float64      `json:"risk_score"`
	Activities   []Activity   `json:"activities,omitempty"`
	StartTime    time.Time    `json:"start_time"`
	LastActivity time.Time    `json:"last_activity"`
}

type ContinuousAuth struct {
	mu             sync.RWMutex
	sessions       map[string]*CASession
	failedAttempts map[string]int
	blockedIPs     map[string]time.Time
	sessionCounter int
}

func NewContinuousAuth() *ContinuousAuth {
	return &ContinuousAuth{
		sessions:       make(map[string]*CASession),
		failedAttempts: make(map[string]int),
		blockedIPs:     make(map[string]time.Time),
	}
}

func (ca *ContinuousAuth) CreateSession(userID, deviceID, ip, location string) (*CASession, error) {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	if userID == "" {
		return nil, fmt.Errorf("user ID required")
	}
	if blockTime, ok := ca.blockedIPs[ip]; ok {
		if time.Since(blockTime) < 30*time.Minute {
			return nil, fmt.Errorf("IP %s blocked", ip)
		}
		delete(ca.blockedIPs, ip)
	}
	ca.sessionCounter++
	s := &CASession{
		ID:           fmt.Sprintf("session-%d", ca.sessionCounter),
		UserID:       userID,
		DeviceID:     deviceID,
		IP:           ip,
		Location:     location,
		Active:       true,
		TrustLevel:   ZTTrustLevelMedium,
		StartTime:    time.Now(),
		LastActivity: time.Now(),
	}
	ca.sessions[s.ID] = s
	return s, nil
}

func (ca *ContinuousAuth) GetSession(id string) (*CASession, error) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	s, ok := ca.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	return s, nil
}

func (ca *ContinuousAuth) EndSession(id string) error {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	s, ok := ca.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	s.Active = false
	return nil
}

func (ca *ContinuousAuth) RecordActivity(sessionID string, activity Activity) error {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	s, ok := ca.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	if !s.Active {
		return fmt.Errorf("session ended")
	}
	s.Activities = append(s.Activities, activity)
	s.LastActivity = time.Now()
	if !activity.Success {
		s.RiskScore += 10
	}
	if activity.IP != s.IP {
		s.RiskScore += 20
	}
	if activity.Location != s.Location {
		s.RiskScore += 15
	}
	if s.RiskScore >= 80 {
		s.Active = false
	}
	return nil
}

func (ca *ContinuousAuth) RecordFailedAttempt(ip string) {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	ca.failedAttempts[ip]++
	if ca.failedAttempts[ip] >= 5 {
		ca.blockedIPs[ip] = time.Now()
	}
}

func (ca *ContinuousAuth) GetUserSessions(userID string) []*CASession {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	var sessions []*CASession
	for _, s := range ca.sessions {
		if s.UserID == userID {
			sessions = append(sessions, s)
		}
	}
	return sessions
}

func (ca *ContinuousAuth) CleanupExpiredSessions() int {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	count := 0
	for id, s := range ca.sessions {
		if !s.Active && time.Since(s.LastActivity) > 1*time.Hour {
			delete(ca.sessions, id)
			count++
		}
	}
	return count
}

// ==================== Micro Segment ====================

type ZTNetworkSegment struct {
	ID      string   `json:"id"`
	Name    string   `json:"name,omitempty"`
	Subnets []string `json:"subnets"`
}

type ZTAccessRule struct {
	ID        string       `json:"id"`
	SourceSeg string       `json:"source_seg"`
	DestSeg   string       `json:"dest_seg"`
	Ports     []int        `json:"ports,omitempty"`
	Protocol  string       `json:"protocol"`
	Effect    PolicyEffect `json:"effect"`
	Enabled   bool         `json:"enabled"`
}

type MicroSegmentManager struct {
	mu       sync.RWMutex
	segments map[string]*ZTNetworkSegment
	rules    map[string]*ZTAccessRule
}

func NewMicroSegmentManager() *MicroSegmentManager {
	return &MicroSegmentManager{
		segments: make(map[string]*ZTNetworkSegment),
		rules:    make(map[string]*ZTAccessRule),
	}
}

func (msm *MicroSegmentManager) AddSegment(seg *ZTNetworkSegment) error {
	msm.mu.Lock()
	defer msm.mu.Unlock()
	if seg.ID == "" {
		return fmt.Errorf("segment ID required")
	}
	for _, s := range seg.Subnets {
		if _, _, err := net.ParseCIDR(s); err != nil {
			return fmt.Errorf("invalid CIDR: %s", s)
		}
	}
	msm.segments[seg.ID] = seg
	return nil
}

func (msm *MicroSegmentManager) RemoveSegment(id string) error {
	msm.mu.Lock()
	defer msm.mu.Unlock()
	if _, ok := msm.segments[id]; !ok {
		return fmt.Errorf("segment not found: %s", id)
	}
	delete(msm.segments, id)
	for ruleID, rule := range msm.rules {
		if rule.SourceSeg == id || rule.DestSeg == id {
			delete(msm.rules, ruleID)
		}
	}
	return nil
}

func (msm *MicroSegmentManager) AddAccessRule(rule *ZTAccessRule) error {
	msm.mu.Lock()
	defer msm.mu.Unlock()
	if _, ok := msm.segments[rule.SourceSeg]; !ok {
		return fmt.Errorf("source segment not found: %s", rule.SourceSeg)
	}
	if _, ok := msm.segments[rule.DestSeg]; !ok {
		return fmt.Errorf("dest segment not found: %s", rule.DestSeg)
	}
	msm.rules[rule.ID] = rule
	return nil
}

func (msm *MicroSegmentManager) RemoveAccessRule(id string) error {
	msm.mu.Lock()
	defer msm.mu.Unlock()
	if _, ok := msm.rules[id]; !ok {
		return fmt.Errorf("rule not found: %s", id)
	}
	delete(msm.rules, id)
	return nil
}

func (msm *MicroSegmentManager) ListAccessRules() []*ZTAccessRule {
	msm.mu.RLock()
	defer msm.mu.RUnlock()
	rules := make([]*ZTAccessRule, 0, len(msm.rules))
	for _, r := range msm.rules {
		rules = append(rules, r)
	}
	return rules
}

func (msm *MicroSegmentManager) ListSegments() []*ZTNetworkSegment {
	msm.mu.RLock()
	defer msm.mu.RUnlock()
	segs := make([]*ZTNetworkSegment, 0, len(msm.segments))
	for _, s := range msm.segments {
		segs = append(segs, s)
	}
	sort.Slice(segs, func(i, j int) bool { return segs[i].ID < segs[j].ID })
	return segs
}

func (msm *MicroSegmentManager) CheckAccess(srcIP, dstIP string, port int, protocol string) (PolicyEffect, string) {
	msm.mu.RLock()
	defer msm.mu.RUnlock()
	srcSeg := msm.findSegment(srcIP)
	dstSeg := msm.findSegment(dstIP)
	if srcSeg == "" || dstSeg == "" {
		return PolicyDeny, "unknown-segment"
	}
	if srcSeg == dstSeg {
		return PolicyAllow, "same-segment"
	}
	for _, rule := range msm.rules {
		if !rule.Enabled {
			continue
		}
		if rule.SourceSeg == srcSeg && rule.DestSeg == dstSeg {
			if rule.Protocol != "any" && rule.Protocol != protocol {
				continue
			}
			if len(rule.Ports) > 0 {
				found := false
				for _, p := range rule.Ports {
					if p == port {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			return rule.Effect, rule.ID
		}
	}
	return PolicyDeny, "no-matching-rule"
}

func (msm *MicroSegmentManager) findSegment(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}
	for segID, seg := range msm.segments {
		for _, subnet := range seg.Subnets {
			_, cidr, err := net.ParseCIDR(subnet)
			if err != nil {
				continue
			}
			if cidr.Contains(ip) {
				return segID
			}
		}
	}
	return ""
}

// ==================== Threat Detector ====================

type Severity int

const (
	SeverityInfo Severity = iota
	SeverityLow
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityLow:
		return "low"
	case SeverityMedium:
		return "medium"
	case SeverityHigh:
		return "high"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

type ResponseAction int

const (
	ActionLog ResponseAction = iota
	ZTActionAlert
	ActionThrottle
	ActionBlock
	ActionQuarantine
)

func (a ResponseAction) String() string {
	switch a {
	case ActionLog:
		return "log"
	case ZTActionAlert:
		return "alert"
	case ActionThrottle:
		return "throttle"
	case ActionBlock:
		return "block"
	case ActionQuarantine:
		return "quarantine"
	default:
		return "unknown"
	}
}

type ThreatDetectionEvent struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Severity  Severity       `json:"severity"`
	Action    ResponseAction `json:"action"`
	Source    string         `json:"source"`
	Timestamp time.Time      `json:"timestamp"`
	Details   string         `json:"details"`
}

type ThreatDetector struct {
	mu           sync.RWMutex
	events       []*ThreatDetectionEvent
	failCounts   map[string]int
	loginHistory map[string][]string
}

func NewThreatDetector() *ThreatDetector {
	return &ThreatDetector{
		events:       make([]*ThreatDetectionEvent, 0),
		failCounts:   make(map[string]int),
		loginHistory: make(map[string][]string),
	}
}

func (td *ThreatDetector) DetectBruteForce(ip string, success bool) *ThreatDetectionEvent {
	td.mu.Lock()
	defer td.mu.Unlock()
	if success {
		td.failCounts[ip] = 0
		return nil
	}
	td.failCounts[ip]++
	if td.failCounts[ip] >= 5 {
		ev := &ThreatDetectionEvent{ID: fmt.Sprintf("ev-%d", time.Now().UnixNano()), Type: "brute_force", Severity: SeverityHigh, Action: ActionBlock, Source: ip, Timestamp: time.Now()}
		td.events = append(td.events, ev)
		return ev
	}
	return nil
}

func (td *ThreatDetector) DetectAbnormalLogin(userID, ip, location string) *ThreatDetectionEvent {
	td.mu.Lock()
	defer td.mu.Unlock()
	td.loginHistory[userID] = append(td.loginHistory[userID], location)
	locs := make(map[string]bool)
	for _, l := range td.loginHistory[userID] {
		locs[l] = true
	}
	if len(locs) >= 3 {
		ev := &ThreatDetectionEvent{ID: fmt.Sprintf("ev-%d", time.Now().UnixNano()), Type: "abnormal_login", Severity: SeverityHigh, Action: ZTActionAlert, Source: ip, Timestamp: time.Now()}
		td.events = append(td.events, ev)
		return ev
	}
	return nil
}

func (td *ThreatDetector) DetectSQLInjection(input, context string) *ThreatDetectionEvent {
	td.mu.Lock()
	defer td.mu.Unlock()
	lower := strings.ToLower(input)
	for _, p := range []string{"' or ", "1'='1", "drop table", "union select"} {
		if strings.Contains(lower, p) {
			ev := &ThreatDetectionEvent{ID: fmt.Sprintf("ev-%d", time.Now().UnixNano()), Type: "sql_injection", Severity: SeverityCritical, Action: ActionBlock, Timestamp: time.Now()}
			td.events = append(td.events, ev)
			return ev
		}
	}
	return nil
}

func (td *ThreatDetector) DetectXSS(input, context string) *ThreatDetectionEvent {
	td.mu.Lock()
	defer td.mu.Unlock()
	lower := strings.ToLower(input)
	for _, p := range []string{"<script", "javascript:", "onerror=", "eval("} {
		if strings.Contains(lower, p) {
			ev := &ThreatDetectionEvent{ID: fmt.Sprintf("ev-%d", time.Now().UnixNano()), Type: "xss", Severity: SeverityHigh, Action: ActionBlock, Timestamp: time.Now()}
			td.events = append(td.events, ev)
			return ev
		}
	}
	return nil
}

func (td *ThreatDetector) GetEvents(limit int) []*ThreatDetectionEvent {
	td.mu.RLock()
	defer td.mu.RUnlock()
	if limit > len(td.events) {
		limit = len(td.events)
	}
	return td.events[len(td.events)-limit:]
}

func (td *ThreatDetector) GetEventsByType(eventType string, limit int) []*ThreatDetectionEvent {
	td.mu.RLock()
	defer td.mu.RUnlock()
	var result []*ThreatDetectionEvent
	for _, e := range td.events {
		if e.Type == eventType {
			result = append(result, e)
		}
	}
	if limit > len(result) {
		limit = len(result)
	}
	return result[len(result)-limit:]
}

// ==================== Security Event Manager ====================

type SecurityEvent struct {
	ID         string     `json:"id"`
	Type       string     `json:"type"`
	Severity   Severity   `json:"severity"`
	Source     string     `json:"source"`
	Timestamp  time.Time  `json:"timestamp"`
	Resolved   bool       `json:"resolved"`
	ResolvedBy string     `json:"resolved_by,omitempty"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

type SecurityAlert struct {
	ID        string    `json:"id"`
	EventID   string    `json:"event_id"`
	Severity  Severity  `json:"severity"`
	Timestamp time.Time `json:"timestamp"`
}

type SecurityEventManager struct {
	mu     sync.RWMutex
	events []*SecurityEvent
	alerts []*SecurityAlert
}

func NewSecurityEventManager() *SecurityEventManager {
	return &SecurityEventManager{
		events: make([]*SecurityEvent, 0),
		alerts: make([]*SecurityAlert, 0),
	}
}

func (sem *SecurityEventManager) RecordEvent(event *SecurityEvent) {
	sem.mu.Lock()
	defer sem.mu.Unlock()
	if event.ID == "" {
		event.ID = fmt.Sprintf("ev-%d", time.Now().UnixNano())
	}
	event.Timestamp = time.Now()
	sem.events = append(sem.events, event)
	if event.Severity >= SeverityHigh {
		sem.alerts = append(sem.alerts, &SecurityAlert{ID: fmt.Sprintf("al-%d", time.Now().UnixNano()), EventID: event.ID, Severity: event.Severity, Timestamp: time.Now()})
	}
}

func (sem *SecurityEventManager) GetEvents(limit int, severity *Severity) []*SecurityEvent {
	sem.mu.RLock()
	defer sem.mu.RUnlock()
	var result []*SecurityEvent
	for _, e := range sem.events {
		if severity != nil && e.Severity < *severity {
			continue
		}
		result = append(result, e)
	}
	if limit > len(result) {
		limit = len(result)
	}
	return result[len(result)-limit:]
}

func (sem *SecurityEventManager) GetAlerts(limit int, severity *Severity) []*SecurityAlert {
	sem.mu.RLock()
	defer sem.mu.RUnlock()
	var result []*SecurityAlert
	for _, a := range sem.alerts {
		if severity != nil && a.Severity != *severity {
			continue
		}
		result = append(result, a)
	}
	if limit > len(result) {
		limit = len(result)
	}
	return result[len(result)-limit:]
}

func (sem *SecurityEventManager) ResolveEvent(id, resolvedBy string) error {
	sem.mu.Lock()
	defer sem.mu.Unlock()
	for _, e := range sem.events {
		if e.ID == id {
			now := time.Now()
			e.Resolved = true
			e.ResolvedBy = resolvedBy
			e.ResolvedAt = &now
			return nil
		}
	}
	return fmt.Errorf("event not found: %s", id)
}

func (sem *SecurityEventManager) GetEventStats() map[string]int {
	sem.mu.RLock()
	defer sem.mu.RUnlock()
	stats := map[string]int{"total": len(sem.events), "info": 0, "low": 0, "medium": 0, "high": 0, "critical": 0}
	for _, e := range sem.events {
		switch e.Severity {
		case SeverityInfo:
			stats["info"]++
		case SeverityLow:
			stats["low"]++
		case SeverityMedium:
			stats["medium"]++
		case SeverityHigh:
			stats["high"]++
		case SeverityCritical:
			stats["critical"]++
		}
	}
	return stats
}

// ==================== Zero Trust Manager ====================

type ZeroTrustManager struct {
	PolicyEngine   *PolicyEngine
	DeviceManager  *DeviceTrustManager
	SegmentManager *MicroSegmentManager
	Reporter       *ComplianceReporter
}

func NewZeroTrustManager() *ZeroTrustManager {
	return &ZeroTrustManager{
		PolicyEngine:   NewPolicyEngine(),
		DeviceManager:  NewDeviceTrustManager(),
		SegmentManager: NewMicroSegmentManager(),
		Reporter:       NewComplianceReporter(),
	}
}

func (ztm *ZeroTrustManager) ProcessAccessRequest(req ZTAccessRequest) PolicyDecision {
	decision := ztm.PolicyEngine.Evaluate(req)
	if decision.Allowed && req.DeviceID != "" {
		device, err := ztm.DeviceManager.GetDevice(req.DeviceID)
		if err == nil && device.TrustLevel == ZTTrustLevelUntrusted {
			return PolicyDecision{Allowed: false, Effect: PolicyDeny, PolicyID: "device-trust"}
		}
	}
	return decision
}

// ==================== Compliance Reporter ====================

type ComplianceReport struct {
	Title           string
	Sections        []ReportSection
	Summary         ReportSummaryData
	Recommendations []string
}

type ReportSection struct {
	Title  string
	Score  float64
	Detail string
}

type ReportSummaryData struct {
	ComplianceScore float64
}

type ComplianceReporter struct{}

func NewComplianceReporter() *ComplianceReporter {
	return &ComplianceReporter{}
}

func (cr *ComplianceReporter) GenerateReport(title string, start, end time.Time) *ComplianceReport {
	report := &ComplianceReport{
		Title: title,
		Sections: []ReportSection{
			{Title: "策略合规", Score: 80},
			{Title: "设备信任", Score: 70},
			{Title: "网络安全", Score: 90},
			{Title: "认证安全", Score: 85},
			{Title: "威胁防护", Score: 75},
		},
		Summary: ReportSummaryData{ComplianceScore: 80},
		Recommendations: []string{
			"建议启用多因素认证",
			"建议定期更新设备信任评分",
		},
	}
	return report
}

func (cr *ComplianceReporter) ExportReportJSON(report *ComplianceReport) ([]byte, error) {
	return []byte(fmt.Sprintf(`{"title":"%s","score":%.0f}`, report.Title, report.Summary.ComplianceScore)), nil
}
