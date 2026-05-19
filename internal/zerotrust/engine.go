// Package zerotrust 提供零信任网络架构实现
package zerotrust

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Engine 零信任策略引擎.
type Engine struct {
	mu sync.RWMutex

	// 策略存储
	policies  map[string]*TrustPolicy      // id -> policy
	segments  map[string]*NetworkSegment   // id -> segment
	rules     map[string]*AccessRule       // id -> rule
	identities map[string]*Identity        // id -> identity
	sessions  map[string]*AccessSession    // id -> session

	// 审计
	auditLog  *AuditLogger
	stats     ZeroTrustStats

	// WireGuard 集成
	wgManager *WireGuardManager

	// 配置
	config    *Config
	startTime time.Time
}

// Config 零信任配置.
type Config struct {
	Enabled              bool          `json:"enabled"`
	DefaultTrustLevel    TrustLevel    `json:"defaultTrustLevel"`
	SessionTimeout       time.Duration `json:"sessionTimeout"`
	MaxConcurrentSessions int          `json:"maxConcurrentSessions"`
	RequireMFA           bool          `json:"requireMFA"`
	RequireDeviceAuth    bool          `json:"requireDeviceAuth"`
	AuditLogPath         string        `json:"auditLogPath"`
	WireGuardEnabled     bool          `json:"wireGuardEnabled"`
	WireGuardInterface   string        `json:"wireGuardInterface"`
	WireGuardPort        int           `json:"wireGuardPort"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		Enabled:              true,
		DefaultTrustLevel:    TrustLevelLow,
		SessionTimeout:       30 * time.Minute,
		MaxConcurrentSessions: 1000,
		RequireMFA:           false,
		RequireDeviceAuth:    false,
		AuditLogPath:         "/var/log/nas-os/zerotrust-audit.log",
		WireGuardEnabled:     false,
		WireGuardInterface:   "wg0",
		WireGuardPort:        51820,
	}
}

// NewEngine 创建零信任引擎.
func NewEngine(cfg *Config) (*Engine, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	e := &Engine{
		policies:   make(map[string]*TrustPolicy),
		segments:   make(map[string]*NetworkSegment),
		rules:      make(map[string]*AccessRule),
		identities: make(map[string]*Identity),
		sessions:   make(map[string]*AccessSession),
		config:     cfg,
		startTime:  time.Now(),
	}

	// 初始化审计日志
	auditLogger, err := NewAuditLogger(cfg.AuditLogPath)
	if err != nil {
		return nil, fmt.Errorf("failed to init audit logger: %w", err)
	}
	e.auditLog = auditLogger

	// 初始化 WireGuard 管理器
	if cfg.WireGuardEnabled {
		wgMgr, err := NewWireGuardManager(cfg.WireGuardInterface, cfg.WireGuardPort)
		if err != nil {
			return nil, fmt.Errorf("failed to init wireguard manager: %w", err)
		}
		e.wgManager = wgMgr
	}

	return e, nil
}

// EvaluateAccess 评估访问请求（核心零信任逻辑）.
func (e *Engine) EvaluateAccess(ctx context.Context, req *AccessRequest) (*AccessDecision, error) {
	if !e.config.Enabled {
		return &AccessDecision{
			Allowed: true,
			Reason:  "zero trust engine disabled",
		}, nil
	}

	// 1. 身份验证
	identity, err := e.verifyIdentity(req.SubjectID)
	if err != nil {
		e.auditLog.LogAccessDenied(req, "identity verification failed: "+err.Error())
		return &AccessDecision{
			Allowed: false,
			Reason:  "identity verification failed",
		}, nil
	}

	// 2. 设备合规检查
	if e.config.RequireDeviceAuth && req.DeviceID != "" {
		if !e.isDeviceCompliant(req.DeviceID) {
			e.auditLog.LogAccessDenied(req, "device not compliant")
			return &AccessDecision{
				Allowed: false,
				Reason:  "device not compliant",
			}, nil
		}
	}

	// 3. 检查访问规则
	rule, err := e.findMatchingRule(req)
	if err != nil {
		e.auditLog.LogAccessDenied(req, "no matching rule: "+err.Error())
		return &AccessDecision{
			Allowed: false,
			Reason:  "no matching access rule",
		}, nil
	}

	// 4. 检查规则状态
	if rule.Status != StatusApproved {
		e.auditLog.LogAccessDenied(req, "rule not approved: "+string(rule.Status))
		return &AccessDecision{
			Allowed: false,
			Reason:  "access rule not approved",
		}, nil
	}

	// 5. 检查过期时间
	if rule.ExpiresAt != nil && rule.ExpiresAt.Before(time.Now()) {
		e.auditLog.LogAccessDenied(req, "rule expired")
		return &AccessDecision{
			Allowed: false,
			Reason:  "access rule expired",
		}, nil
	}

	// 6. MFA 检查
	if e.config.RequireMFA || rule.RequireMFA {
		if !req.MFAVerified {
			e.auditLog.LogAccessDenied(req, "mfa required")
			return &AccessDecision{
				Allowed:    false,
				Reason:     "multi-factor authentication required",
				RequireMFA: true,
			}, nil
		}
	}

	// 7. 策略评估
	policyDecision, err := e.evaluatePolicies(req, identity)
	if err != nil {
		e.auditLog.LogAccessDenied(req, "policy evaluation failed: "+err.Error())
		return &AccessDecision{
			Allowed: false,
			Reason:  "policy evaluation failed",
		}, nil
	}

	if !policyDecision.Allowed {
		e.auditLog.LogPolicyViolation(req, policyDecision.PolicyID, policyDecision.Reason)
		return policyDecision, nil
	}

	// 8. 检查并发会话限制
	if e.config.MaxConcurrentSessions > 0 {
		activeSessions := e.countActiveSessions(req.SubjectID)
		if activeSessions >= e.config.MaxConcurrentSessions {
			e.auditLog.LogAccessDenied(req, "max concurrent sessions reached")
			return &AccessDecision{
				Allowed: false,
				Reason:  "max concurrent sessions reached",
			}, nil
		}
	}

	// 9. 创建访问会话
	session := &AccessSession{
		ID:         uuid.New().String(),
		RuleID:     rule.ID,
		SubjectID:  req.SubjectID,
		ResourceID: req.ResourceID,
		Status:     StatusApproved,
		StartTime:  time.Now(),
		ExpiresAt:  time.Now().Add(e.config.SessionTimeout),
		VerifiedBy: req.VerificationMethods,
		TrustScore: identity.TrustScore,
		DeviceID:   req.DeviceID,
		SourceIP:   req.SourceIP,
	}

	e.mu.Lock()
	e.sessions[session.ID] = session
	e.stats.mu.Lock()
	e.stats.TotalRequests++
	e.stats.AllowedRequests++
	e.stats.ActiveSessions++
	e.stats.mu.Unlock()
	e.mu.Unlock()

	// 记录审计日志
	e.auditLog.LogAccessGranted(req, rule.ID, session.ID)

	return &AccessDecision{
		Allowed:   true,
		Reason:    "access granted",
		SessionID: session.ID,
		ExpiresAt: &session.ExpiresAt,
		RuleID:    rule.ID,
	}, nil
}

// AccessRequest 访问请求.
type AccessRequest struct {
	SubjectID          string               `json:"subjectId"`
	SubjectType        string               `json:"subjectType"`
	ResourceID         string               `json:"resourceId"`
	ResourceType       string               `json:"resourceType"`
	Action             string               `json:"action"`
	SourceIP           string               `json:"sourceIP"`
	DestIP             string               `json:"destIP"`
	DestPort           int                  `json:"destPort"`
	Protocol           string               `json:"protocol"`
	DeviceID           string               `json:"deviceId,omitempty"`
	MFAVerified        bool                 `json:"mfaVerified"`
	VerificationMethods []VerificationMethod `json:"verificationMethods,omitempty"`
	RequestHeaders     map[string]string    `json:"requestHeaders,omitempty"`
}

// AccessDecision 访问决策.
type AccessDecision struct {
	Allowed    bool       `json:"allowed"`
	Reason     string     `json:"reason"`
	SessionID  string     `json:"sessionId,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	RuleID     string     `json:"ruleId,omitempty"`
	PolicyID   string     `json:"policyId,omitempty"`
	RequireMFA bool       `json:"requireMFA,omitempty"`
}

// verifyIdentity 验证身份.
func (e *Engine) verifyIdentity(subjectID string) (*Identity, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	identity, ok := e.identities[subjectID]
	if !ok {
		return nil, fmt.Errorf("identity not found: %s", subjectID)
	}

	if !identity.Enabled {
		return nil, fmt.Errorf("identity disabled: %s", subjectID)
	}

	// 检查验证是否过期（默认 24 小时）
	if time.Since(identity.LastVerified) > 24*time.Hour {
		return nil, fmt.Errorf("identity verification expired: %s", subjectID)
	}

	return identity, nil
}

// isDeviceCompliant 检查设备合规性.
func (e *Engine) isDeviceCompliant(deviceID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, identity := range e.identities {
		if identity.DeviceID == deviceID && identity.Type == "device" {
			return identity.IsCompliant && identity.Enabled
		}
	}
	return false
}

// findMatchingRule 查找匹配的访问规则.
func (e *Engine) findMatchingRule(req *AccessRequest) (*AccessRule, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}

		// 检查主体匹配
		if rule.SubjectType != "" && rule.SubjectType != req.SubjectType {
			continue
		}
		if rule.SubjectID != "" && rule.SubjectID != req.SubjectID {
			continue
		}

		// 检查资源匹配
		if rule.ResourceType != "" && rule.ResourceType != req.ResourceType {
			continue
		}
		if rule.ResourceID != "" && rule.ResourceID != req.ResourceID {
			continue
		}

		// 检查动作匹配
		if len(rule.AllowedActions) > 0 {
			actionAllowed := false
			for _, a := range rule.AllowedActions {
				if a == req.Action {
					actionAllowed = true
					break
				}
			}
			if !actionAllowed {
				continue
			}
		}

		return rule, nil
	}

	return nil, fmt.Errorf("no matching rule found")
}

// evaluatePolicies 评估策略.
func (e *Engine) evaluatePolicies(req *AccessRequest, identity *Identity) (*AccessDecision, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 按优先级排序策略
	sortedPolicies := e.getSortedPolicies()

	for _, policy := range sortedPolicies {
		if !policy.Enabled {
			continue
		}

		// 检查时间有效性
		if policy.ValidFrom != nil && policy.ValidFrom.After(time.Now()) {
			continue
		}
		if policy.ValidUntil != nil && policy.ValidUntil.Before(time.Now()) {
			continue
		}

		// 检查源分段匹配
		if !e.matchesSegment(policy.SourceSegments, req.SourceIP) {
			continue
		}

		// 检查目标分段匹配
		if !e.matchesSegment(policy.DestSegments, req.DestIP) {
			continue
		}

		// 检查身份匹配
		if !e.matchesIdentity(policy.SourceIdentities, req.SubjectID) {
			continue
		}

		// 检查端口匹配
		if len(policy.AllowedPorts) > 0 {
			portAllowed := false
			for _, p := range policy.AllowedPorts {
				if p == req.DestPort {
					portAllowed = true
					break
				}
			}
			if !portAllowed {
				continue
			}
		}

		// 检查协议匹配
		if len(policy.AllowedProtocols) > 0 {
			protoAllowed := false
			for _, p := range policy.AllowedProtocols {
				if p == req.Protocol {
					protoAllowed = true
					break
				}
			}
			if !protoAllowed {
				continue
			}
		}

		// 检查信任等级
		if identity.TrustLevel < policy.RequiredTrust {
			return &AccessDecision{
				Allowed:  false,
				Reason:   fmt.Sprintf("insufficient trust level: have %s, need %s", identity.TrustLevel, policy.RequiredTrust),
				PolicyID: policy.ID,
			}, nil
		}

		// 根据策略动作决策
		switch policy.Action {
		case ActionAllow:
			return &AccessDecision{
				Allowed:  true,
				Reason:   "allowed by policy: " + policy.Name,
				PolicyID: policy.ID,
			}, nil
		case ActionDeny:
			return &AccessDecision{
				Allowed:  false,
				Reason:   "denied by policy: " + policy.Name,
				PolicyID: policy.ID,
			}, nil
		case ActionMFA:
			if req.MFAVerified {
				return &AccessDecision{
					Allowed:  true,
					Reason:   "allowed by policy with MFA: " + policy.Name,
					PolicyID: policy.ID,
				}, nil
			}
			return &AccessDecision{
				Allowed:    false,
				Reason:     "MFA required by policy: " + policy.Name,
				PolicyID:   policy.ID,
				RequireMFA: true,
			}, nil
		case ActionAudit:
			// 审计模式：允许但记录
			e.auditLog.LogAuditEvent(req, policy.ID, "audit policy triggered")
			return &AccessDecision{
				Allowed:  true,
				Reason:   "allowed with audit by policy: " + policy.Name,
				PolicyID: policy.ID,
			}, nil
		case ActionAlert:
			// 告警模式：允许但告警
			e.stats.mu.Lock()
			e.stats.SecurityAlerts++
			e.stats.mu.Unlock()
			return &AccessDecision{
				Allowed:  true,
				Reason:   "allowed with alert by policy: " + policy.Name,
				PolicyID: policy.ID,
			}, nil
		}
	}

	// 默认拒绝（零信任原则）
	return &AccessDecision{
		Allowed: false,
		Reason:  "default deny: no matching policy",
	}, nil
}

// getSortedPolicies 获取排序后的策略列表.
func (e *Engine) getSortedPolicies() []*TrustPolicy {
	policies := make([]*TrustPolicy, 0, len(e.policies))
	for _, p := range e.policies {
		policies = append(policies, p)
	}

	// 按优先级排序（简单冒泡，策略数量通常不多）
	for i := 0; i < len(policies); i++ {
		for j := i + 1; j < len(policies); j++ {
			if policies[i].Priority > policies[j].Priority {
				policies[i], policies[j] = policies[j], policies[i]
			}
		}
	}

	return policies
}

// matchesSegment 检查 IP 是否匹配分段.
func (e *Engine) matchesSegment(segmentIDs []string, ip string) bool {
	if len(segmentIDs) == 0 {
		return true // 空列表表示匹配所有
	}

	if ip == "" {
		return false
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	for _, segID := range segmentIDs {
		segment, ok := e.segments[segID]
		if !ok {
			continue
		}

		// 检查子网
		if segment.Subnet != "" {
			_, cidr, err := net.ParseCIDR(segment.Subnet)
			if err == nil && cidr.Contains(parsedIP) {
				return true
			}
		}

		// 检查 IP 范围
		if segment.IPRange != "" {
			if e.ipInRange(parsedIP, segment.IPRange) {
				return true
			}
		}

		// 检查允许的 IP 列表
		for _, allowedIP := range segment.AllowedIPs {
			if allowedIP == ip {
				return true
			}
		}
	}

	return false
}

// ipInRange 检查 IP 是否在范围内.
func (e *Engine) ipInRange(ip net.IP, ipRange string) bool {
	// 支持格式: "start-end"
	// 这里简化实现
	return false
}

// matchesIdentity 检查身份是否匹配.
func (e *Engine) matchesIdentity(identityIDs []string, subjectID string) bool {
	if len(identityIDs) == 0 {
		return true // 空列表表示匹配所有
	}

	for _, id := range identityIDs {
		if id == subjectID {
			return true
		}
		// 检查组匹配
		identity, ok := e.identities[subjectID]
		if ok {
			// 这里可以扩展组匹配逻辑
			_ = identity
		}
	}

	return false
}

// countActiveSessions 统计活跃会话数.
func (e *Engine) countActiveSessions(subjectID string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	count := 0
	now := time.Now()
	for _, session := range e.sessions {
		if session.SubjectID == subjectID &&
			session.Status == StatusApproved &&
			session.ExpiresAt.After(now) {
			count++
		}
	}
	return count
}

// RevokeSession 撤销会话.
func (e *Engine) RevokeSession(sessionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	session, ok := e.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.Status = StatusRevoked
	now := time.Now()
	session.EndTime = &now

	e.stats.mu.Lock()
	e.stats.ActiveSessions--
	e.stats.mu.Unlock()

	e.auditLog.LogSessionRevoked(sessionID, session.SubjectID)
	return nil
}

// GetStats 获取统计信息.
func (e *Engine) GetStats() *ZeroTrustStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := e.stats.GetSnapshot()
	stats.Uptime = int64(time.Since(e.startTime).Seconds())
	stats.TotalPolicies = len(e.policies)
	stats.TotalSegments = len(e.segments)
	stats.TotalAccessRules = len(e.rules)
	stats.TotalIdentities = len(e.identities)

	// 统计活跃数
	now := time.Now()
	for _, p := range e.policies {
		if p.Enabled {
			stats.ActivePolicies++
		} else {
			stats.DisabledPolicies++
		}
	}

	for _, s := range e.segments {
		if s.Enabled {
			stats.ActiveSegments++
		}
		if s.IsIsolated {
			stats.IsolatedSegments++
		}
	}

	for _, session := range e.sessions {
		if session.Status == StatusApproved && session.ExpiresAt.After(now) {
			stats.ActiveSessions++
		}
	}

	// WireGuard 统计
	if e.wgManager != nil {
		wgStats := e.wgManager.GetStats()
		stats.WGTunnels = wgStats.Tunnels
		stats.WGPeers = wgStats.Peers
		stats.WGActivePeers = wgStats.ActivePeers
		stats.WGBytesSent = wgStats.BytesSent
		stats.WGBytesRecv = wgStats.BytesRecv
	}

	return stats
}

// AddPolicy 添加策略.
func (e *Engine) AddPolicy(policy *TrustPolicy) error {
	if policy.ID == "" {
		policy.ID = uuid.New().String()
	}
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	e.mu.Lock()
	defer e.mu.Unlock()

	e.policies[policy.ID] = policy
	e.stats.mu.Lock()
	e.stats.LastPolicyUpdate = time.Now()
	e.stats.mu.Unlock()

	e.auditLog.LogPolicyChange("add", policy.ID, policy.Name)
	return nil
}

// UpdatePolicy 更新策略.
func (e *Engine) UpdatePolicy(policy *TrustPolicy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.policies[policy.ID]; !ok {
		return fmt.Errorf("policy not found: %s", policy.ID)
	}

	policy.UpdatedAt = time.Now()
	e.policies[policy.ID] = policy

	e.stats.mu.Lock()
	e.stats.LastPolicyUpdate = time.Now()
	e.stats.mu.Unlock()

	e.auditLog.LogPolicyChange("update", policy.ID, policy.Name)
	return nil
}

// DeletePolicy 删除策略.
func (e *Engine) DeletePolicy(policyID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	policy, ok := e.policies[policyID]
	if !ok {
		return fmt.Errorf("policy not found: %s", policyID)
	}

	delete(e.policies, policyID)

	e.stats.mu.Lock()
	e.stats.LastPolicyUpdate = time.Now()
	e.stats.mu.Unlock()

	e.auditLog.LogPolicyChange("delete", policyID, policy.Name)
	return nil
}

// GetPolicy 获取策略.
func (e *Engine) GetPolicy(policyID string) (*TrustPolicy, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	policy, ok := e.policies[policyID]
	return policy, ok
}

// ListPolicies 列出所有策略.
func (e *Engine) ListPolicies() []*TrustPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()
	policies := make([]*TrustPolicy, 0, len(e.policies))
	for _, p := range e.policies {
		policies = append(policies, p)
	}
	return policies
}

// AddSegment 添加网络分段.
func (e *Engine) AddSegment(segment *NetworkSegment) error {
	if segment.ID == "" {
		segment.ID = uuid.New().String()
	}
	segment.CreatedAt = time.Now()
	segment.UpdatedAt = time.Now()

	e.mu.Lock()
	defer e.mu.Unlock()

	e.segments[segment.ID] = segment
	return nil
}

// UpdateSegment 更新网络分段.
func (e *Engine) UpdateSegment(segment *NetworkSegment) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.segments[segment.ID]; !ok {
		return fmt.Errorf("segment not found: %s", segment.ID)
	}

	segment.UpdatedAt = time.Now()
	e.segments[segment.ID] = segment
	return nil
}

// DeleteSegment 删除网络分段.
func (e *Engine) DeleteSegment(segmentID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.segments[segmentID]; !ok {
		return fmt.Errorf("segment not found: %s", segmentID)
	}

	delete(e.segments, segmentID)
	return nil
}

// GetSegment 获取网络分段.
func (e *Engine) GetSegment(segmentID string) (*NetworkSegment, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	segment, ok := e.segments[segmentID]
	return segment, ok
}

// ListSegments 列出所有网络分段.
func (e *Engine) ListSegments() []*NetworkSegment {
	e.mu.RLock()
	defer e.mu.RUnlock()
	segments := make([]*NetworkSegment, 0, len(e.segments))
	for _, s := range e.segments {
		segments = append(segments, s)
	}
	return segments
}

// AddRule 添加访问规则.
func (e *Engine) AddRule(rule *AccessRule) error {
	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	e.mu.Lock()
	defer e.mu.Unlock()

	e.rules[rule.ID] = rule
	return nil
}

// UpdateRule 更新访问规则.
func (e *Engine) UpdateRule(rule *AccessRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.rules[rule.ID]; !ok {
		return fmt.Errorf("rule not found: %s", rule.ID)
	}

	rule.UpdatedAt = time.Now()
	e.rules[rule.ID] = rule
	return nil
}

// DeleteRule 删除访问规则.
func (e *Engine) DeleteRule(ruleID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.rules[ruleID]; !ok {
		return fmt.Errorf("rule not found: %s", ruleID)
	}

	delete(e.rules, ruleID)
	return nil
}

// GetRule 获取访问规则.
func (e *Engine) GetRule(ruleID string) (*AccessRule, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	rule, ok := e.rules[ruleID]
	return rule, ok
}

// ListRules 列出所有访问规则.
func (e *Engine) ListRules() []*AccessRule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	rules := make([]*AccessRule, 0, len(e.rules))
	for _, r := range e.rules {
		rules = append(rules, r)
	}
	return rules
}

// AddIdentity 添加身份.
func (e *Engine) AddIdentity(identity *Identity) error {
	if identity.ID == "" {
		identity.ID = uuid.New().String()
	}
	identity.CreatedAt = time.Now()
	identity.UpdatedAt = time.Now()

	e.mu.Lock()
	defer e.mu.Unlock()

	e.identities[identity.ID] = identity
	return nil
}

// UpdateIdentity 更新身份.
func (e *Engine) UpdateIdentity(identity *Identity) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.identities[identity.ID]; !ok {
		return fmt.Errorf("identity not found: %s", identity.ID)
	}

	identity.UpdatedAt = time.Now()
	e.identities[identity.ID] = identity
	return nil
}

// DeleteIdentity 删除身份.
func (e *Engine) DeleteIdentity(identityID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.identities[identityID]; !ok {
		return fmt.Errorf("identity not found: %s", identityID)
	}

	delete(e.identities, identityID)
	return nil
}

// GetIdentity 获取身份.
func (e *Engine) GetIdentity(identityID string) (*Identity, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	identity, ok := e.identities[identityID]
	return identity, ok
}

// ListIdentities 列出所有身份.
func (e *Engine) ListIdentities() []*Identity {
	e.mu.RLock()
	defer e.mu.RUnlock()
	identities := make([]*Identity, 0, len(e.identities))
	for _, i := range e.identities {
		identities = append(identities, i)
	}
	return identities
}

// GetSession 获取会话.
func (e *Engine) GetSession(sessionID string) (*AccessSession, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	session, ok := e.sessions[sessionID]
	return session, ok
}

// ListSessions 列出所有会话.
func (e *Engine) ListSessions() []*AccessSession {
	e.mu.RLock()
	defer e.mu.RUnlock()
	sessions := make([]*AccessSession, 0, len(e.sessions))
	for _, s := range e.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// GetAuditLog 获取审计日志.
func (e *Engine) GetAuditLog() *AuditLogger {
	return e.auditLog
}

// GetWireGuardManager 获取 WireGuard 管理器.
func (e *Engine) GetWireGuardManager() *WireGuardManager {
	return e.wgManager
}

// Close 关闭引擎.
func (e *Engine) Close() error {
	log.Println("[ZeroTrust] Shutting down zero trust engine...")

	if e.auditLog != nil {
		if err := e.auditLog.Close(); err != nil {
			log.Printf("[ZeroTrust] Failed to close audit logger: %v", err)
		}
	}

	if e.wgManager != nil {
		if err := e.wgManager.Close(); err != nil {
			log.Printf("[ZeroTrust] Failed to close wireguard manager: %v", err)
		}
	}

	return nil
}
