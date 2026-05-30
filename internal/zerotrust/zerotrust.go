// Package zerotrust 提供零信任安全模块实现
// 对标 TrueNAS Ransomware Defense + 现代零信任架构
package zerotrust

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// 类型定义
// ============================================================================

// TrustLevel 信任等级
type TrustLevel int

const (
	TrustLevelUntrusted TrustLevel = iota
	TrustLevelLow
	TrustLevelMedium
	TrustLevelHigh
	TrustLevelFull
)

func (t TrustLevel) String() string {
	switch t {
	case TrustLevelUntrusted:
		return "untrusted"
	case TrustLevelLow:
		return "low"
	case TrustLevelMedium:
		return "medium"
	case TrustLevelHigh:
		return "high"
	case TrustLevelFull:
		return "full"
	default:
		return "unknown"
	}
}

// Severity 事件严重等级
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

// ResponseAction 响应动作类型
type ResponseAction int

const (
	ActionLog       ResponseAction = iota
	ActionAlert
	ActionThrottle
	ActionBlock
	ActionQuarantine
)

func (r ResponseAction) String() string {
	switch r {
	case ActionLog:
		return "log"
	case ActionAlert:
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

// PolicyEffect 策略效果
type PolicyEffect int

const (
	PolicyAllow    PolicyEffect = iota
	PolicyDeny
	PolicyChallenge
)

func (p PolicyEffect) String() string {
	switch p {
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

// ============================================================================
// 1. 安全策略引擎
// ============================================================================

// SecurityPolicy 安全策略定义
type SecurityPolicy struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Enabled     bool             `json:"enabled"`
	Priority    int              `json:"priority"`
	Effect      PolicyEffect     `json:"effect"`
	Conditions  PolicyCondition  `json:"conditions"`
	Actions     []ResponseAction `json:"actions"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// PolicyCondition 策略条件
type PolicyCondition struct {
	Users       []string   `json:"users,omitempty"`
	Groups      []string   `json:"groups,omitempty"`
	Devices     []string   `json:"devices,omitempty"`
	DeviceTypes []string   `json:"device_types,omitempty"`
	Locations   []string   `json:"locations,omitempty"`
	Networks    []string   `json:"networks,omitempty"`
	TimeStart   string     `json:"time_start,omitempty"`
	TimeEnd     string     `json:"time_end,omitempty"`
	Weekdays    []string   `json:"weekdays,omitempty"`
	MinTrust    TrustLevel `json:"min_trust"`
	Resources   []string   `json:"resources,omitempty"`
	Actions     []string   `json:"actions,omitempty"`
}

// AccessRequest 访问请求
type AccessRequest struct {
	UserID     string    `json:"user_id"`
	Groups     []string  `json:"groups"`
	DeviceID   string    `json:"device_id"`
	DeviceType string    `json:"device_type"`
	IP         string    `json:"ip"`
	Location   string    `json:"location"`
	Resource   string    `json:"resource"`
	Action     string    `json:"action"`
	Timestamp  time.Time `json:"timestamp"`
}

// AccessDecision 访问决策
type AccessDecision struct {
	Allowed    bool             `json:"allowed"`
	Effect     PolicyEffect     `json:"effect"`
	PolicyID   string           `json:"policy_id"`
	Reason     string           `json:"reason"`
	Challenges []string         `json:"challenges,omitempty"`
	Actions    []ResponseAction `json:"actions,omitempty"`
	DecidedAt  time.Time        `json:"decided_at"`
}

// PolicyEngine 安全策略引擎
type PolicyEngine struct {
	policies map[string]*SecurityPolicy
	mu       sync.RWMutex
}

// NewPolicyEngine 创建安全策略引擎
func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{policies: make(map[string]*SecurityPolicy)}
}

// AddPolicy 添加安全策略
func (pe *PolicyEngine) AddPolicy(policy *SecurityPolicy) error {
	if policy.ID == "" {
		return errors.New("策略ID不能为空")
	}
	if policy.Name == "" {
		return errors.New("策略名称不能为空")
	}
	pe.mu.Lock()
	defer pe.mu.Unlock()
	now := time.Now()
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = now
	}
	policy.UpdatedAt = now
	pe.policies[policy.ID] = policy
	return nil
}

// RemovePolicy 移除安全策略
func (pe *PolicyEngine) RemovePolicy(policyID string) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	if _, exists := pe.policies[policyID]; !exists {
		return fmt.Errorf("策略 %s 不存在", policyID)
	}
	delete(pe.policies, policyID)
	return nil
}

// GetPolicy 获取安全策略
func (pe *PolicyEngine) GetPolicy(policyID string) (*SecurityPolicy, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	policy, exists := pe.policies[policyID]
	if !exists {
		return nil, fmt.Errorf("策略 %s 不存在", policyID)
	}
	return policy, nil
}

// ListPolicies 列出所有安全策略（按优先级排序）
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

// Evaluate 评估访问请求
func (pe *PolicyEngine) Evaluate(req AccessRequest) AccessDecision {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	policies := make([]*SecurityPolicy, 0)
	for _, p := range pe.policies {
		if p.Enabled {
			policies = append(policies, p)
		}
	}
	sort.Slice(policies, func(i, j int) bool { return policies[i].Priority < policies[j].Priority })

	for _, policy := range policies {
		if matchCondition(policy.Conditions, req) {
			return AccessDecision{
				Allowed:   policy.Effect == PolicyAllow,
				Effect:    policy.Effect,
				PolicyID:  policy.ID,
				Reason:    fmt.Sprintf("匹配策略: %s", policy.Name),
				Actions:   policy.Actions,
				DecidedAt: time.Now(),
			}
		}
	}

	return AccessDecision{
		Allowed:   false,
		Effect:    PolicyDeny,
		PolicyID:  "default",
		Reason:    "没有匹配的策略，默认拒绝",
		DecidedAt: time.Now(),
	}
}

// matchCondition 检查是否匹配策略条件
func matchCondition(cond PolicyCondition, req AccessRequest) bool {
	if len(cond.Users) > 0 && !containsString(cond.Users, req.UserID) {
		return false
	}
	if len(cond.Groups) > 0 {
		matched := false
		for _, group := range cond.Groups {
			if containsString(req.Groups, group) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(cond.Devices) > 0 && !containsString(cond.Devices, req.DeviceID) {
		return false
	}
	if len(cond.DeviceTypes) > 0 && !containsString(cond.DeviceTypes, req.DeviceType) {
		return false
	}
	if len(cond.Networks) > 0 {
		matched := false
		for _, network := range cond.Networks {
			if matchNetwork(network, req.IP) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(cond.Locations) > 0 && !containsString(cond.Locations, req.Location) {
		return false
	}
	if cond.TimeStart != "" && cond.TimeEnd != "" {
		if !matchTimeRange(cond.TimeStart, cond.TimeEnd, req.Timestamp) {
			return false
		}
	}
	if len(cond.Weekdays) > 0 {
		weekday := req.Timestamp.Weekday().String()
		if !containsString(cond.Weekdays, weekday) {
			return false
		}
	}
	if len(cond.Resources) > 0 && !matchResourcePattern(cond.Resources, req.Resource) {
		return false
	}
	if len(cond.Actions) > 0 && !containsString(cond.Actions, req.Action) {
		return false
	}
	return true
}

// ============================================================================
// 2. 设备信任评估
// ============================================================================

// DeviceInfo 设备信息
type DeviceInfo struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	OS           string            `json:"os"`
	OSVersion    string            `json:"os_version"`
	Browser      string            `json:"browser,omitempty"`
	Fingerprint  string            `json:"fingerprint"`
	IP           string            `json:"ip"`
	MACAddress   string            `json:"mac_address,omitempty"`
	LastSeen     time.Time         `json:"last_seen"`
	RegisteredAt time.Time         `json:"registered_at"`
	Compliance   DeviceCompliance  `json:"compliance"`
	TrustLevel   TrustLevel        `json:"trust_level"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// DeviceCompliance 设备合规状态
type DeviceCompliance struct {
	OSUpdated       bool      `json:"os_updated"`
	FirewallEnabled bool      `json:"firewall_enabled"`
	AntivirusActive bool      `json:"antivirus_active"`
	DiskEncrypted   bool      `json:"disk_encrypted"`
	PasswordStrong  bool      `json:"password_strong"`
	LastChecked     time.Time `json:"last_checked"`
	ComplianceScore float64   `json:"compliance_score"`
}

// DeviceTrustManager 设备信任管理器
type DeviceTrustManager struct {
	devices      map[string]*DeviceInfo
	fingerprints map[string]string
	mu           sync.RWMutex
}

// NewDeviceTrustManager 创建设备信任管理器
func NewDeviceTrustManager() *DeviceTrustManager {
	return &DeviceTrustManager{
		devices:      make(map[string]*DeviceInfo),
		fingerprints: make(map[string]string),
	}
}

// RegisterDevice 注册设备
func (dtm *DeviceTrustManager) RegisterDevice(device *DeviceInfo) error {
	if device.ID == "" {
		return errors.New("设备ID不能为空")
	}
	if device.Fingerprint == "" {
		return errors.New("设备指纹不能为空")
	}
	dtm.mu.Lock()
	defer dtm.mu.Unlock()
	if existingID, exists := dtm.fingerprints[device.Fingerprint]; exists && existingID != device.ID {
		return errors.New("设备指纹已被其他设备使用")
	}
	now := time.Now()
	if device.RegisteredAt.IsZero() {
		device.RegisteredAt = now
	}
	device.LastSeen = now
	if device.TrustLevel == 0 {
		device.TrustLevel = TrustLevelLow
	}
	dtm.devices[device.ID] = device
	dtm.fingerprints[device.Fingerprint] = device.ID
	return nil
}

// UnregisterDevice 注销设备
func (dtm *DeviceTrustManager) UnregisterDevice(deviceID string) error {
	dtm.mu.Lock()
	defer dtm.mu.Unlock()
	device, exists := dtm.devices[deviceID]
	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}
	delete(dtm.fingerprints, device.Fingerprint)
	delete(dtm.devices, deviceID)
	return nil
}

// GetDevice 获取设备信息
func (dtm *DeviceTrustManager) GetDevice(deviceID string) (*DeviceInfo, error) {
	dtm.mu.RLock()
	defer dtm.mu.RUnlock()
	device, exists := dtm.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("设备 %s 不存在", deviceID)
	}
	return device, nil
}

// UpdateLastSeen 更新设备最后在线时间
func (dtm *DeviceTrustManager) UpdateLastSeen(deviceID string) error {
	dtm.mu.Lock()
	defer dtm.mu.Unlock()
	device, exists := dtm.devices[deviceID]
	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}
	device.LastSeen = time.Now()
	return nil
}

// CheckCompliance 检查设备合规性并更新信任等级
func (dtm *DeviceTrustManager) CheckCompliance(deviceID string) (*DeviceCompliance, error) {
	dtm.mu.Lock()
	defer dtm.mu.Unlock()
	device, exists := dtm.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("设备 %s 不存在", deviceID)
	}
	score := 0.0
	if device.Compliance.OSUpdated {
		score += 20
	}
	if device.Compliance.FirewallEnabled {
		score += 20
	}
	if device.Compliance.AntivirusActive {
		score += 20
	}
	if device.Compliance.DiskEncrypted {
		score += 20
	}
	if device.Compliance.PasswordStrong {
		score += 20
	}
	device.Compliance.ComplianceScore = score
	device.Compliance.LastChecked = time.Now()
	switch {
	case score >= 80:
		device.TrustLevel = TrustLevelHigh
	case score >= 60:
		device.TrustLevel = TrustLevelMedium
	case score >= 40:
		device.TrustLevel = TrustLevelLow
	default:
		device.TrustLevel = TrustLevelUntrusted
	}
	cpy := device.Compliance
	return &cpy, nil
}

// GenerateFingerprint 生成设备指纹（SHA256）
func GenerateFingerprint(components map[string]string) string {
	keys := make([]string, 0, len(components))
	for k := range components {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s;", k, components[k])
	}
	h := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(h[:])
}

// EvaluateDeviceTrust 评估设备信任度（综合在线时间与合规分数）
func (dtm *DeviceTrustManager) EvaluateDeviceTrust(deviceID string) (TrustLevel, error) {
	dtm.mu.RLock()
	defer dtm.mu.RUnlock()
	device, exists := dtm.devices[deviceID]
	if !exists {
		return TrustLevelUntrusted, fmt.Errorf("设备 %s 不存在", deviceID)
	}
	trust := device.TrustLevel
	elapsed := time.Since(device.LastSeen)
	switch {
	case elapsed > 30*24*time.Hour:
		trust = TrustLevelUntrusted
	case elapsed > 7*24*time.Hour:
		if trust > TrustLevelLow {
			trust = TrustLevelLow
		}
	case elapsed > 24*time.Hour:
		if trust > TrustLevelMedium {
			trust = TrustLevelMedium
		}
	}
	if device.Compliance.ComplianceScore < 40 && trust > TrustLevelLow {
		trust = TrustLevelLow
	}
	return trust, nil
}

// ListDevices 列出所有设备
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

// ============================================================================
// 3. 持续认证
// ============================================================================

// Session 会话信息
type Session struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	DeviceID     string     `json:"device_id"`
	IP           string     `json:"ip"`
	Location     string     `json:"location"`
	StartedAt    time.Time  `json:"started_at"`
	LastActivity time.Time  `json:"last_activity"`
	ExpiresAt    time.Time  `json:"expires_at"`
	RiskScore    float64    `json:"risk_score"`
	TrustLevel   TrustLevel `json:"trust_level"`
	Active       bool       `json:"active"`
	Activities   []Activity `json:"activities"`
}

// Activity 用户活动
type Activity struct {
	Type      string    `json:"type"`
	Resource  string    `json:"resource"`
	Action    string    `json:"action"`
	IP        string    `json:"ip"`
	Location  string    `json:"location"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
	RiskScore float64   `json:"risk_score"`
}

// ContinuousAuth 持续认证管理器
type ContinuousAuth struct {
	sessions       map[string]*Session
	userSessions   map[string][]string
	failedAttempts map[string][]time.Time
	mu             sync.RWMutex
	sessionTimeout time.Duration
	maxRiskScore   float64
}

// NewContinuousAuth 创建持续认证管理器
func NewContinuousAuth() *ContinuousAuth {
	return &ContinuousAuth{
		sessions:       make(map[string]*Session),
		userSessions:   make(map[string][]string),
		failedAttempts: make(map[string][]time.Time),
		sessionTimeout: 24 * time.Hour,
		maxRiskScore:   80.0,
	}
}

// CreateSession 创建会话
func (ca *ContinuousAuth) CreateSession(userID, deviceID, ip, location string) (*Session, error) {
	if userID == "" {
		return nil, errors.New("用户ID不能为空")
	}
	ca.mu.Lock()
	defer ca.mu.Unlock()
	if ca.isIPBlocked(ip) {
		return nil, errors.New("IP已被临时封禁，失败尝试过多")
	}
	now := time.Now()
	session := &Session{
		ID:           generateSessionID(userID, deviceID, now),
		UserID:       userID,
		DeviceID:     deviceID,
		IP:           ip,
		Location:     location,
		StartedAt:    now,
		LastActivity: now,
		ExpiresAt:    now.Add(ca.sessionTimeout),
		RiskScore:    0,
		TrustLevel:   TrustLevelMedium,
		Active:       true,
		Activities:   make([]Activity, 0),
	}
	ca.sessions[session.ID] = session
	ca.userSessions[userID] = append(ca.userSessions[userID], session.ID)
	return session, nil
}

// GetSession 获取会话
func (ca *ContinuousAuth) GetSession(sessionID string) (*Session, error) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	s, ok := ca.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("会话 %s 不存在", sessionID)
	}
	return s, nil
}

// EndSession 结束会话
func (ca *ContinuousAuth) EndSession(sessionID string) error {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	s, ok := ca.sessions[sessionID]
	if !ok {
		return fmt.Errorf("会话 %s 不存在", sessionID)
	}
	s.Active = false
	return nil
}

// RecordActivity 记录活动并重新评估风险
func (ca *ContinuousAuth) RecordActivity(sessionID string, activity Activity) error {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	session, ok := ca.sessions[sessionID]
	if !ok {
		return fmt.Errorf("会话 %s 不存在", sessionID)
	}
	if !session.Active {
		return errors.New("会话已结束")
	}
	activity.Timestamp = time.Now()
	session.Activities = append(session.Activities, activity)
	session.LastActivity = time.Now()
	session.RiskScore = ca.calculateRiskScore(session)
	switch {
	case session.RiskScore >= 80:
		session.TrustLevel = TrustLevelUntrusted
	case session.RiskScore >= 60:
		session.TrustLevel = TrustLevelLow
	case session.RiskScore >= 40:
		session.TrustLevel = TrustLevelMedium
	case session.RiskScore >= 20:
		session.TrustLevel = TrustLevelHigh
	default:
		session.TrustLevel = TrustLevelFull
	}
	if session.RiskScore >= ca.maxRiskScore {
		session.Active = false
	}
	return nil
}

// RecordFailedAttempt 记录失败尝试
func (ca *ContinuousAuth) RecordFailedAttempt(ip string) {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	now := time.Now()
	ca.failedAttempts[ip] = append(ca.failedAttempts[ip], now)
	cutoff := now.Add(-1 * time.Hour)
	valid := make([]time.Time, 0)
	for _, t := range ca.failedAttempts[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	ca.failedAttempts[ip] = valid
}

func (ca *ContinuousAuth) isIPBlocked(ip string) bool {
	attempts, ok := ca.failedAttempts[ip]
	if !ok {
		return false
	}
	cutoff := time.Now().Add(-5 * time.Minute)
	count := 0
	for _, t := range attempts {
		if t.After(cutoff) {
			count++
		}
	}
	return count >= 5
}

func (ca *ContinuousAuth) calculateRiskScore(s *Session) float64 {
	risk := 0.0
	cutoff := time.Now().Add(-5 * time.Minute)
	recent := 0
	for _, a := range s.Activities {
		if a.Timestamp.After(cutoff) {
			recent++
		}
	}
	if recent > 20 {
		risk += 30
	} else if recent > 10 {
		risk += 15
	}
	failures := 0
	for _, a := range s.Activities {
		if !a.Success {
			failures++
		}
	}
	if failures > 5 {
		risk += 25
	} else if failures > 2 {
		risk += 10
	}
	locs := make(map[string]bool)
	for _, a := range s.Activities {
		if a.Location != "" {
			locs[a.Location] = true
		}
	}
	if len(locs) > 3 {
		risk += 20
	}
	for _, a := range s.Activities {
		if a.Type == "admin" || a.Type == "download" {
			risk += 5
		}
	}
	if risk > 100 {
		risk = 100
	}
	return risk
}

// GetUserSessions 获取用户的所有活跃会话
func (ca *ContinuousAuth) GetUserSessions(userID string) []*Session {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	ids, ok := ca.userSessions[userID]
	if !ok {
		return nil
	}
	sessions := make([]*Session, 0)
	for _, id := range ids {
		if s, ok := ca.sessions[id]; ok && s.Active {
			sessions = append(sessions, s)
		}
	}
	return sessions
}

// CleanupExpiredSessions 清理过期会话
func (ca *ContinuousAuth) CleanupExpiredSessions() int {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	now := time.Now()
	cleaned := 0
	for id, s := range ca.sessions {
		if s.ExpiresAt.Before(now) || (!s.Active && now.Sub(s.LastActivity) > time.Hour) {
			delete(ca.sessions, id)
			cleaned++
		}
	}
	return cleaned
}

// ============================================================================
// 4. 微分段
// ============================================================================

// NetworkSegment 网络分段
type NetworkSegment struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Subnets      []string `json:"subnets"`
	Services     []string `json:"services"`
	SecurityZone string   `json:"security_zone"`
	Isolation    bool     `json:"isolation"`
}

// AccessRule 网络访问规则
type AccessRule struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	SourceSeg   string       `json:"source_segment"`
	DestSeg     string       `json:"dest_segment"`
	Ports       []int        `json:"ports"`
	Protocol    string       `json:"protocol"`
	Effect      PolicyEffect `json:"effect"`
	Enabled     bool         `json:"enabled"`
	Description string       `json:"description"`
}

// MicroSegmentManager 微分段管理器
type MicroSegmentManager struct {
	segments map[string]*NetworkSegment
	rules    map[string]*AccessRule
	mu       sync.RWMutex
}

// NewMicroSegmentManager 创建微分段管理器
func NewMicroSegmentManager() *MicroSegmentManager {
	return &MicroSegmentManager{
		segments: make(map[string]*NetworkSegment),
		rules:    make(map[string]*AccessRule),
	}
}

// AddSegment 添加网络分段
func (msm *MicroSegmentManager) AddSegment(segment *NetworkSegment) error {
	if segment.ID == "" {
		return errors.New("分段ID不能为空")
	}
	for _, subnet := range segment.Subnets {
		if _, _, err := net.ParseCIDR(subnet); err != nil {
			return fmt.Errorf("无效的子网格式 %s: %v", subnet, err)
		}
	}
	msm.mu.Lock()
	defer msm.mu.Unlock()
	msm.segments[segment.ID] = segment
	return nil
}

// RemoveSegment 移除网络分段及其相关规则
func (msm *MicroSegmentManager) RemoveSegment(segmentID string) error {
	msm.mu.Lock()
	defer msm.mu.Unlock()
	if _, exists := msm.segments[segmentID]; !exists {
		return fmt.Errorf("分段 %s 不存在", segmentID)
	}
	for ruleID, rule := range msm.rules {
		if rule.SourceSeg == segmentID || rule.DestSeg == segmentID {
			delete(msm.rules, ruleID)
		}
	}
	delete(msm.segments, segmentID)
	return nil
}

// AddAccessRule 添加访问规则
func (msm *MicroSegmentManager) AddAccessRule(rule *AccessRule) error {
	if rule.ID == "" {
		return errors.New("规则ID不能为空")
	}
	msm.mu.Lock()
	defer msm.mu.Unlock()
	if _, ok := msm.segments[rule.SourceSeg]; !ok {
		return fmt.Errorf("源分段 %s 不存在", rule.SourceSeg)
	}
	if _, ok := msm.segments[rule.DestSeg]; !ok {
		return fmt.Errorf("目标分段 %s 不存在", rule.DestSeg)
	}
	msm.rules[rule.ID] = rule
	return nil
}

// RemoveAccessRule 移除访问规则
func (msm *MicroSegmentManager) RemoveAccessRule(ruleID string) error {
	msm.mu.Lock()
	defer msm.mu.Unlock()
	if _, exists := msm.rules[ruleID]; !exists {
		return fmt.Errorf("规则 %s 不存在", ruleID)
	}
	delete(msm.rules, ruleID)
	return nil
}

// CheckAccess 检查网络访问权限
func (msm *MicroSegmentManager) CheckAccess(sourceIP, destIP string, port int, protocol string) (PolicyEffect, string) {
	msm.mu.RLock()
	defer msm.mu.RUnlock()
	srcSeg := msm.findSegmentForIP(sourceIP)
	dstSeg := msm.findSegmentForIP(destIP)
	if srcSeg == "" || dstSeg == "" {
		return PolicyDeny, "无法确定网络分段"
	}
	if srcSeg == dstSeg {
		return PolicyAllow, "同一网络分段内通信"
	}
	for _, rule := range msm.rules {
		if !rule.Enabled {
			continue
		}
		if rule.SourceSeg == srcSeg && rule.DestSeg == dstSeg {
			if rule.Protocol != "any" && rule.Protocol != protocol {
				continue
			}
			if len(rule.Ports) > 0 && !containsInt(rule.Ports, port) {
				continue
			}
			return rule.Effect, rule.Name
		}
	}
	return PolicyDeny, "无匹配的访问规则"
}

func (msm *MicroSegmentManager) findSegmentForIP(ip string) string {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return ""
	}
	for _, seg := range msm.segments {
		for _, subnet := range seg.Subnets {
			_, network, err := net.ParseCIDR(subnet)
			if err != nil {
				continue
			}
			if network.Contains(parsedIP) {
				return seg.ID
			}
		}
	}
	return ""
}

// ListSegments 列出所有分段
func (msm *MicroSegmentManager) ListSegments() []*NetworkSegment {
	msm.mu.RLock()
	defer msm.mu.RUnlock()
	segments := make([]*NetworkSegment, 0, len(msm.segments))
	for _, s := range msm.segments {
		segments = append(segments, s)
	}
	sort.Slice(segments, func(i, j int) bool { return segments[i].ID < segments[j].ID })
	return segments
}

// ListAccessRules 列出所有访问规则
func (msm *MicroSegmentManager) ListAccessRules() []*AccessRule {
	msm.mu.RLock()
	defer msm.mu.RUnlock()
	rules := make([]*AccessRule, 0, len(msm.rules))
	for _, r := range msm.rules {
		rules = append(rules, r)
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	return rules
}

// ============================================================================
// 5. 威胁检测
// ============================================================================

// ThreatEvent 威胁事件
type ThreatEvent struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Source      string         `json:"source"`
	Target      string         `json:"target"`
	IP          string         `json:"ip"`
	Description string         `json:"description"`
	Severity    Severity       `json:"severity"`
	Timestamp   time.Time      `json:"timestamp"`
	Evidence    string         `json:"evidence"`
	Action      ResponseAction `json:"action"`
}

// ThreatDetector 威胁检测器
type ThreatDetector struct {
	events        []*ThreatEvent
	bruteForceMap map[string][]time.Time
	abnormalMap   map[string][]Activity
	mu            sync.RWMutex
	maxEvents     int
}

// NewThreatDetector 创建威胁检测器
func NewThreatDetector() *ThreatDetector {
	return &ThreatDetector{
		events:        make([]*ThreatEvent, 0),
		bruteForceMap: make(map[string][]time.Time),
		abnormalMap:   make(map[string][]Activity),
		maxEvents:     10000,
	}
}

// DetectBruteForce 检测暴力破解（5分钟内失败≥5次触发）
func (td *ThreatDetector) DetectBruteForce(ip string, success bool) *ThreatEvent {
	td.mu.Lock()
	defer td.mu.Unlock()
	now := time.Now()
	if !success {
		td.bruteForceMap[ip] = append(td.bruteForceMap[ip], now)
	}
	cutoff := now.Add(-30 * time.Minute)
	valid := make([]time.Time, 0)
	for _, t := range td.bruteForceMap[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	td.bruteForceMap[ip] = valid

	fiveMinCutoff := now.Add(-5 * time.Minute)
	recentFailures := 0
	for _, t := range valid {
		if t.After(fiveMinCutoff) {
			recentFailures++
		}
	}
	if recentFailures >= 5 {
		event := &ThreatEvent{
			ID:          generateEventID("brute_force", ip, now),
			Type:        "brute_force",
			Source:      ip,
			IP:          ip,
			Description: fmt.Sprintf("检测到暴力破解尝试，5分钟内失败%d次", recentFailures),
			Severity:    SeverityHigh,
			Timestamp:   now,
			Evidence:    fmt.Sprintf("IP: %s, 失败次数: %d", ip, recentFailures),
			Action:      ActionBlock,
		}
		td.addEvent(event)
		return event
	}
	return nil
}

// DetectAbnormalLogin 检测异常登录（10分钟内多位置登录）
func (td *ThreatDetector) DetectAbnormalLogin(userID, ip, location string) *ThreatEvent {
	td.mu.Lock()
	defer td.mu.Unlock()
	now := time.Now()
	activities := td.abnormalMap[userID]
	cutoff := now.Add(-10 * time.Minute)
	recentLocations := make(map[string]bool)
	for _, a := range activities {
		if a.Timestamp.After(cutoff) && a.Location != "" {
			recentLocations[a.Location] = true
		}
	}
	recentLocations[location] = true

	if len(recentLocations) > 2 {
		event := &ThreatEvent{
			ID:          generateEventID("abnormal_login", userID, now),
			Type:        "abnormal_login",
			Source:      userID,
			IP:          ip,
			Description: fmt.Sprintf("检测到异常登录，10分钟内从%d个不同位置登录", len(recentLocations)),
			Severity:    SeverityHigh,
			Timestamp:   now,
			Evidence:    fmt.Sprintf("用户: %s, 位置数: %d, IP: %s", userID, len(recentLocations), ip),
			Action:      ActionAlert,
		}
		td.addEvent(event)
		return event
	}

	td.abnormalMap[userID] = append(td.abnormalMap[userID], Activity{
		Type: "login", IP: ip, Location: location, Timestamp: now, Success: true,
	})
	hourCutoff := now.Add(-1 * time.Hour)
	valid := make([]Activity, 0)
	for _, a := range td.abnormalMap[userID] {
		if a.Timestamp.After(hourCutoff) {
			valid = append(valid, a)
		}
	}
	td.abnormalMap[userID] = valid
	return nil
}

// DetectSQLInjection 检测SQL注入攻击
func (td *ThreatDetector) DetectSQLInjection(input, source string) *ThreatEvent {
	td.mu.RLock()
	defer td.mu.RUnlock()
	patterns := []string{
		"' OR '1'='1", "' OR 1=1", "; DROP TABLE", "; DELETE FROM",
		"UNION SELECT", "' UNION", "--", "/*", "*/", "xp_", "EXEC(", "EXECUTE(",
	}
	upper := strings.ToUpper(input)
	for _, pat := range patterns {
		if strings.Contains(upper, strings.ToUpper(pat)) {
			event := &ThreatEvent{
				ID:          generateEventID("sql_injection", source, time.Now()),
				Type:        "sql_injection",
				Source:      source,
				Description: fmt.Sprintf("检测到SQL注入尝试，匹配模式: %s", pat),
				Severity:    SeverityCritical,
				Timestamp:   time.Now(),
				Evidence:    fmt.Sprintf("输入: %s, 模式: %s", input, pat),
				Action:      ActionBlock,
			}
			td.addEvent(event)
			return event
		}
	}
	return nil
}

// DetectXSS 检测XSS攻击
func (td *ThreatDetector) DetectXSS(input, source string) *ThreatEvent {
	td.mu.RLock()
	defer td.mu.RUnlock()
	patterns := []string{
		"<script",
		"javascript:",
		"onerror=",
		"onload=",
		"onclick=",
		"onfocus=",
		"onblur=",
		"onmouseover=",
		"eval(",
		"document.cookie",
		"document.write",
		"innerHTML",
	}
	lower := strings.ToLower(input)
	for _, pat := range patterns {
		if strings.Contains(lower, pat) {
			event := &ThreatEvent{
				ID:          generateEventID("xss", source, time.Now()),
				Type:        "xss",
				Source:      source,
				Description: fmt.Sprintf("检测到XSS攻击尝试，匹配模式: %s", pat),
				Severity:    SeverityHigh,
				Timestamp:   time.Now(),
				Evidence:    fmt.Sprintf("输入: %s, 模式: %s", input, pat),
				Action:      ActionBlock,
			}
			td.addEvent(event)
			return event
		}
	}
	return nil
}

func (td *ThreatDetector) addEvent(event *ThreatEvent) {
	td.events = append(td.events, event)
	if len(td.events) > td.maxEvents {
		td.events = td.events[len(td.events)-td.maxEvents:]
	}
}

// GetEvents 获取最新的威胁事件
func (td *ThreatDetector) GetEvents(limit int) []*ThreatEvent {
	td.mu.RLock()
	defer td.mu.RUnlock()
	if limit <= 0 || limit > len(td.events) {
		limit = len(td.events)
	}
	start := len(td.events) - limit
	if start < 0 {
		start = 0
	}
	events := make([]*ThreatEvent, limit)
	copy(events, td.events[start:])
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	return events
}

// GetEventsByType 按类型获取威胁事件
func (td *ThreatDetector) GetEventsByType(threatType string, limit int) []*ThreatEvent {
	td.mu.RLock()
	defer td.mu.RUnlock()
	events := make([]*ThreatEvent, 0)
	for i := len(td.events) - 1; i >= 0 && len(events) < limit; i-- {
		if td.events[i].Type == threatType {
			events = append(events, td.events[i])
		}
	}
	return events
}

// ============================================================================
// 6. 安全事件管理
// ============================================================================

// SecurityEvent 安全事件
type SecurityEvent struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Category    string            `json:"category"`
	Source      string            `json:"source"`
	Target      string            `json:"target"`
	UserID      string            `json:"user_id"`
	IP          string            `json:"ip"`
	Description string            `json:"description"`
	Severity    Severity          `json:"severity"`
	Actions     []ResponseAction  `json:"actions"`
	Timestamp   time.Time         `json:"timestamp"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Resolved    bool              `json:"resolved"`
	ResolvedAt  *time.Time        `json:"resolved_at,omitempty"`
	ResolvedBy  string            `json:"resolved_by,omitempty"`
}

// SecurityEventManager 安全事件管理器
type SecurityEventManager struct {
	events    []*SecurityEvent
	alerts    []*SecurityEvent
	mu        sync.RWMutex
	maxEvents int
}

// NewSecurityEventManager 创建安全事件管理器
func NewSecurityEventManager() *SecurityEventManager {
	return &SecurityEventManager{
		events:    make([]*SecurityEvent, 0),
		alerts:    make([]*SecurityEvent, 0),
		maxEvents: 50000,
	}
}

// RecordEvent 记录安全事件
func (sem *SecurityEventManager) RecordEvent(event *SecurityEvent) {
	sem.mu.Lock()
	defer sem.mu.Unlock()
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.ID == "" {
		event.ID = generateEventID(event.Type, event.Source, event.Timestamp)
	}
	sem.events = append(sem.events, event)
	if event.Severity >= SeverityHigh {
		sem.alerts = append(sem.alerts, event)
	}
	if len(sem.events) > sem.maxEvents {
		sem.events = sem.events[len(sem.events)-sem.maxEvents:]
	}
	if len(sem.alerts) > sem.maxEvents/10 {
		sem.alerts = sem.alerts[len(sem.alerts)-sem.maxEvents/10:]
	}
}

// GetEvents 获取事件列表（倒序，可按严重等级过滤）
func (sem *SecurityEventManager) GetEvents(limit int, severity *Severity) []*SecurityEvent {
	sem.mu.RLock()
	defer sem.mu.RUnlock()
	events := make([]*SecurityEvent, 0)
	for i := len(sem.events) - 1; i >= 0 && len(events) < limit; i-- {
		if severity == nil || sem.events[i].Severity >= *severity {
			events = append(events, sem.events[i])
		}
	}
	return events
}

// GetAlerts 获取告警列表
func (sem *SecurityEventManager) GetAlerts(limit int, resolved *bool) []*SecurityEvent {
	sem.mu.RLock()
	defer sem.mu.RUnlock()
	alerts := make([]*SecurityEvent, 0)
	for i := len(sem.alerts) - 1; i >= 0 && len(alerts) < limit; i-- {
		if resolved == nil || sem.alerts[i].Resolved == *resolved {
			alerts = append(alerts, sem.alerts[i])
		}
	}
	return alerts
}

// ResolveEvent 解决事件
func (sem *SecurityEventManager) ResolveEvent(eventID, resolvedBy string) error {
	sem.mu.Lock()
	defer sem.mu.Unlock()
	for _, event := range sem.events {
		if event.ID == eventID {
			now := time.Now()
			event.Resolved = true
			event.ResolvedAt = &now
			event.ResolvedBy = resolvedBy
			return nil
		}
	}
	return fmt.Errorf("事件 %s 不存在", eventID)
}

// GetEventStats 获取事件统计
func (sem *SecurityEventManager) GetEventStats() map[string]int {
	sem.mu.RLock()
	defer sem.mu.RUnlock()
	stats := map[string]int{
		"total": len(sem.events), "alerts": len(sem.alerts),
		"info": 0, "low": 0, "medium": 0, "high": 0, "critical": 0,
		"resolved": 0, "unresolved": 0,
	}
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
		if e.Resolved {
			stats["resolved"]++
		} else {
			stats["unresolved"]++
		}
	}
	return stats
}

// ============================================================================
// 7. 合规报告
// ============================================================================

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID              string          `json:"id"`
	Title           string          `json:"title"`
	GeneratedAt     time.Time       `json:"generated_at"`
	Period          ReportPeriod    `json:"period"`
	Summary         ReportSummary   `json:"summary"`
	Sections        []ReportSection `json:"sections"`
	Recommendations []string        `json:"recommendations"`
	RiskLevel       Severity        `json:"risk_level"`
}

// ReportPeriod 报告周期
type ReportPeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ReportSummary 报告摘要
type ReportSummary struct {
	TotalEvents      int     `json:"total_events"`
	CriticalEvents   int     `json:"critical_events"`
	HighEvents       int     `json:"high_events"`
	MediumEvents     int     `json:"medium_events"`
	LowEvents        int     `json:"low_events"`
	InfoEvents       int     `json:"info_events"`
	ResolvedEvents   int     `json:"resolved_events"`
	ComplianceScore  float64 `json:"compliance_score"`
	DeviceCompliance float64 `json:"device_compliance"`
	ActiveSessions   int     `json:"active_sessions"`
	ThreatsDetected  int     `json:"threats_detected"`
	ThreatsBlocked   int     `json:"threats_blocked"`
}

// ReportSection 报告章节
type ReportSection struct {
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Items       []ReportItem `json:"items"`
	Score       float64      `json:"score"`
}

// ReportItem 报告条目
type ReportItem struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Details     string `json:"details,omitempty"`
}

// ComplianceReporter 合规报告生成器
type ComplianceReporter struct {
	policyEngine   *PolicyEngine
	deviceManager  *DeviceTrustManager
	sessionManager *ContinuousAuth
	segmentManager *MicroSegmentManager
	threatDetector *ThreatDetector
	eventManager   *SecurityEventManager
}

// NewComplianceReporter 创建合规报告生成器
func NewComplianceReporter(pe *PolicyEngine, dtm *DeviceTrustManager, ca *ContinuousAuth, msm *MicroSegmentManager, td *ThreatDetector, sem *SecurityEventManager) *ComplianceReporter {
	return &ComplianceReporter{
		policyEngine: pe, deviceManager: dtm, sessionManager: ca,
		segmentManager: msm, threatDetector: td, eventManager: sem,
	}
}

// GenerateReport 生成合规报告
func (cr *ComplianceReporter) GenerateReport(title string, start, end time.Time) *ComplianceReport {
	report := &ComplianceReport{
		ID:              generateEventID("report", title, time.Now()),
		Title:           title,
		GeneratedAt:     time.Now(),
		Period:          ReportPeriod{Start: start, End: end},
		Sections:        make([]ReportSection, 0),
		Recommendations: make([]string, 0),
	}
	cr.collectSummary(report)
	report.Sections = append(report.Sections, cr.generatePolicySection())
	report.Sections = append(report.Sections, cr.generateDeviceSection())
	report.Sections = append(report.Sections, cr.generateSessionSection())
	report.Sections = append(report.Sections, cr.generateNetworkSection())
	report.Sections = append(report.Sections, cr.generateThreatSection())
	cr.generateRecommendations(report)
	cr.calculateRiskLevel(report)
	return report
}

func (cr *ComplianceReporter) collectSummary(report *ComplianceReport) {
	stats := cr.eventManager.GetEventStats()
	report.Summary.TotalEvents = stats["total"]
	report.Summary.CriticalEvents = stats["critical"]
	report.Summary.HighEvents = stats["high"]
	report.Summary.MediumEvents = stats["medium"]
	report.Summary.LowEvents = stats["low"]
	report.Summary.InfoEvents = stats["info"]
	report.Summary.ResolvedEvents = stats["resolved"]

	devices := cr.deviceManager.ListDevices()
	if len(devices) > 0 {
		total := 0.0
		for _, d := range devices {
			total += d.Compliance.ComplianceScore
		}
		report.Summary.DeviceCompliance = total / float64(len(devices))
	}

	threats := cr.threatDetector.GetEvents(1000)
	report.Summary.ThreatsDetected = len(threats)
	blocked := 0
	for _, e := range threats {
		if e.Action == ActionBlock {
			blocked++
		}
	}
	report.Summary.ThreatsBlocked = blocked
	report.Summary.ComplianceScore = cr.calcComplianceScore(report)
}

func (cr *ComplianceReporter) calcComplianceScore(report *ComplianceReport) float64 {
	score := 100.0
	score -= float64(report.Summary.CriticalEvents) * 10
	score -= float64(report.Summary.HighEvents) * 5
	score -= float64(report.Summary.MediumEvents) * 2
	if report.Summary.DeviceCompliance > 0 {
		score = (score + report.Summary.DeviceCompliance) / 2
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}

func (cr *ComplianceReporter) generatePolicySection() ReportSection {
	sec := ReportSection{Title: "安全策略", Description: "安全策略配置和执行情况", Items: make([]ReportItem, 0)}
	policies := cr.policyEngine.ListPolicies()
	enabled := 0
	for _, p := range policies {
		if p.Enabled {
			enabled++
		}
	}
	sec.Items = append(sec.Items, ReportItem{Name: "策略总数", Status: "pass", Description: fmt.Sprintf("已配置 %d 个安全策略", len(policies))})
	if len(policies) == 0 {
		sec.Items = append(sec.Items, ReportItem{Name: "策略配置", Status: "fail", Description: "未配置任何安全策略"})
		sec.Score = 0
	} else {
		sec.Score = float64(enabled) / float64(len(policies)) * 100
	}
	return sec
}

func (cr *ComplianceReporter) generateDeviceSection() ReportSection {
	sec := ReportSection{Title: "设备信任", Description: "设备注册和合规情况", Items: make([]ReportItem, 0)}
	devices := cr.deviceManager.ListDevices()
	compliant := 0
	for _, d := range devices {
		if d.Compliance.ComplianceScore >= 60 {
			compliant++
		}
	}
	sec.Items = append(sec.Items, ReportItem{Name: "设备总数", Status: "pass", Description: fmt.Sprintf("已注册 %d 个设备", len(devices))})
	if len(devices) > 0 {
		rate := float64(compliant) / float64(len(devices)) * 100
		sec.Items = append(sec.Items, ReportItem{Name: "设备合规率", Status: complianceStatus(rate), Description: fmt.Sprintf("%.1f%% 设备达到合规标准", rate)})
		sec.Score = rate
	} else {
		sec.Score = 100
	}
	return sec
}

func (cr *ComplianceReporter) generateSessionSection() ReportSection {
	sec := ReportSection{Title: "会话管理", Description: "用户会话和认证情况", Items: make([]ReportItem, 0)}
	sec.Items = append(sec.Items, ReportItem{Name: "持续认证", Status: "pass", Description: "持续认证机制已启用"})
	sec.Score = 100
	return sec
}

func (cr *ComplianceReporter) generateNetworkSection() ReportSection {
	sec := ReportSection{Title: "网络隔离", Description: "微分段和网络访问控制", Items: make([]ReportItem, 0)}
	segs := cr.segmentManager.ListSegments()
	rules := cr.segmentManager.ListAccessRules()
	sec.Items = append(sec.Items, ReportItem{Name: "网络分段", Status: complianceStatus(float64(len(segs)) * 20), Description: fmt.Sprintf("已配置 %d 个网络分段", len(segs))})
	sec.Items = append(sec.Items, ReportItem{Name: "访问规则", Status: "pass", Description: fmt.Sprintf("已配置 %d 条访问规则", len(rules))})
	if len(segs) > 0 {
		sec.Score = 100
	} else {
		sec.Score = 50
	}
	return sec
}

func (cr *ComplianceReporter) generateThreatSection() ReportSection {
	sec := ReportSection{Title: "威胁检测", Description: "威胁检测和响应情况", Items: make([]ReportItem, 0)}
	threats := cr.threatDetector.GetEvents(100)
	blocked := 0
	for _, e := range threats {
		if e.Action == ActionBlock {
			blocked++
		}
	}
	sec.Items = append(sec.Items, ReportItem{Name: "检测到的威胁", Status: threatStatus(len(threats)), Description: fmt.Sprintf("检测到 %d 个威胁事件", len(threats))})
	sec.Items = append(sec.Items, ReportItem{Name: "已阻断的威胁", Status: "pass", Description: fmt.Sprintf("成功阻断 %d 个威胁", blocked)})
	if len(threats) == 0 {
		sec.Score = 100
	} else {
		sec.Score = float64(blocked) / float64(len(threats)) * 100
	}
	return sec
}

func (cr *ComplianceReporter) generateRecommendations(report *ComplianceReport) {
	if len(cr.policyEngine.ListPolicies()) == 0 {
		report.Recommendations = append(report.Recommendations, "建议配置安全策略以加强访问控制")
	}
	for _, d := range cr.deviceManager.ListDevices() {
		if d.Compliance.ComplianceScore < 60 {
			report.Recommendations = append(report.Recommendations, fmt.Sprintf("设备 %s 合规分数不足，建议检查安全配置", d.Name))
		}
	}
	if len(cr.threatDetector.GetEvents(10)) > 0 {
		report.Recommendations = append(report.Recommendations, "检测到威胁活动，建议加强监控和防护")
	}
	if report.Summary.CriticalEvents > 0 {
		report.Recommendations = append(report.Recommendations, "存在未解决的严重事件，建议立即处理")
	}
	if len(cr.segmentManager.ListSegments()) == 0 {
		report.Recommendations = append(report.Recommendations, "建议配置网络分段以实现微隔离")
	}
}

func (cr *ComplianceReporter) calculateRiskLevel(report *ComplianceReport) {
	switch {
	case report.Summary.CriticalEvents > 0:
		report.RiskLevel = SeverityCritical
	case report.Summary.HighEvents > 5:
		report.RiskLevel = SeverityHigh
	case report.Summary.HighEvents > 0 || report.Summary.MediumEvents > 10:
		report.RiskLevel = SeverityMedium
	case report.Summary.MediumEvents > 0:
		report.RiskLevel = SeverityLow
	default:
		report.RiskLevel = SeverityInfo
	}
}

// ExportReportJSON 导出报告为JSON
func (cr *ComplianceReporter) ExportReportJSON(report *ComplianceReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// ============================================================================
// ZeroTrustManager — 零信任安全管理器（整合所有组件）
// ============================================================================

// ZeroTrustManager 零信任安全管理器
type ZeroTrustManager struct {
	PolicyEngine   *PolicyEngine
	DeviceManager  *DeviceTrustManager
	SessionManager *ContinuousAuth
	SegmentManager *MicroSegmentManager
	ThreatDetector *ThreatDetector
	EventManager   *SecurityEventManager
	Reporter       *ComplianceReporter
	mu             sync.RWMutex
}

// NewZeroTrustManager 创建零信任安全管理器
func NewZeroTrustManager() *ZeroTrustManager {
	pe := NewPolicyEngine()
	dtm := NewDeviceTrustManager()
	ca := NewContinuousAuth()
	msm := NewMicroSegmentManager()
	td := NewThreatDetector()
	sem := NewSecurityEventManager()
	rep := NewComplianceReporter(pe, dtm, ca, msm, td, sem)
	return &ZeroTrustManager{
		PolicyEngine: pe, DeviceManager: dtm, SessionManager: ca,
		SegmentManager: msm, ThreatDetector: td, EventManager: sem, Reporter: rep,
	}
}

// ProcessAccessRequest 处理访问请求（完整零信任流程）
func (ztm *ZeroTrustManager) ProcessAccessRequest(req AccessRequest) AccessDecision {
	ztm.mu.RLock()
	defer ztm.mu.RUnlock()

	// 1. 检查设备信任
	if req.DeviceID != "" {
		trust, err := ztm.DeviceManager.EvaluateDeviceTrust(req.DeviceID)
		if err == nil && trust < TrustLevelLow {
			return AccessDecision{
				Allowed: false, Effect: PolicyDeny, PolicyID: "device_trust",
				Reason: fmt.Sprintf("设备信任等级不足: %s", trust), DecidedAt: time.Now(),
			}
		}
	}

	// 2. 检查会话风险
	if req.UserID != "" {
		for _, s := range ztm.SessionManager.GetUserSessions(req.UserID) {
			if s.RiskScore >= 80 {
				return AccessDecision{
					Allowed: false, Effect: PolicyDeny, PolicyID: "session_risk",
					Reason: fmt.Sprintf("会话风险过高: %.1f", s.RiskScore), DecidedAt: time.Now(),
				}
			}
		}
	}

	// 3. 策略评估
	decision := ztm.PolicyEngine.Evaluate(req)

	// 4. 记录安全事件
	ztm.EventManager.RecordEvent(&SecurityEvent{
		Type: "access_request", Category: "authorization",
		Source: req.UserID, Target: req.Resource, UserID: req.UserID, IP: req.IP,
		Description: fmt.Sprintf("访问请求: %s -> %s (%s)", req.UserID, req.Resource, decision.Effect),
		Severity: severityFromDecision(decision), Actions: decision.Actions,
	})

	return decision
}

// ============================================================================
// 辅助函数
// ============================================================================

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func containsInt(slice []int, item int) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func matchNetwork(network, ip string) bool {
	if strings.Contains(network, "/") {
		_, ipNet, err := net.ParseCIDR(network)
		if err != nil {
			return false
		}
		return ipNet.Contains(net.ParseIP(ip))
	}
	return network == ip
}

func matchTimeRange(start, end string, t time.Time) bool {
	sp := strings.Split(start, ":")
	ep := strings.Split(end, ":")
	if len(sp) != 2 || len(ep) != 2 {
		return false
	}
	sh, sm := parseInt(sp[0]), parseInt(sp[1])
	eh, em := parseInt(ep[0]), parseInt(ep[1])
	cur := t.Hour()*60 + t.Minute()
	rs := sh*60 + sm
	re := eh*60 + em
	if rs <= re {
		return cur >= rs && cur <= re
	}
	return cur >= rs || cur <= re
}

func matchResourcePattern(patterns []string, resource string) bool {
	for _, p := range patterns {
		if p == "*" {
			return true
		}
		if strings.HasSuffix(p, "*") {
			prefix := strings.TrimSuffix(p, "*")
			if strings.HasPrefix(resource, prefix) {
				return true
			}
		}
		if p == resource {
			return true
		}
	}
	return false
}

func parseInt(s string) int {
	r := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			r = r*10 + int(c-'0')
		}
	}
	return r
}

func generateSessionID(userID, deviceID string, t time.Time) string {
	data := fmt.Sprintf("%s:%s:%d", userID, deviceID, t.UnixNano())
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:16])
}

func generateEventID(eventType, source string, t time.Time) string {
	data := fmt.Sprintf("%s:%s:%d", eventType, source, t.UnixNano())
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:16])
}

func severityFromDecision(d AccessDecision) Severity {
	if !d.Allowed {
		return SeverityMedium
	}
	return SeverityInfo
}

func complianceStatus(score float64) string {
	switch {
	case score >= 80:
		return "pass"
	case score >= 60:
		return "warning"
	default:
		return "fail"
	}
}

func threatStatus(count int) string {
	switch {
	case count == 0:
		return "pass"
	case count <= 5:
		return "warning"
	default:
		return "fail"
	}
}
