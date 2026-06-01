// Package zerotrust 提供零信任安全核心逻辑
package zerotrust

import (
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 零信任安全管理器
type Manager struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	devices     map[string]*DeviceTrust
	policies    map[string]*AccessPolicy
	sessions    map[string]*AuthSession
	threats     map[string]*ThreatEvent
	trustScores map[string]*TrustScore
}

// NewManager 创建零信任安全管理器
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &Manager{
		logger:      logger,
		devices:     make(map[string]*DeviceTrust),
		policies:    make(map[string]*AccessPolicy),
		sessions:    make(map[string]*AuthSession),
		threats:     make(map[string]*ThreatEvent),
		trustScores: make(map[string]*TrustScore),
	}

	// 初始化默认策略
	m.initDefaultPolicies()

	return m
}

// initDefaultPolicies 初始化默认访问策略
func (m *Manager) initDefaultPolicies() {
	defaultPolicies := []*AccessPolicy{
		{
			ID: "policy-001", Name: "默认拒绝策略", Description: "未明确允许的访问将被拒绝",
			Priority: 1, Enabled: true,
			Subject: PolicySubject{Type: "*", IDs: []string{"*"}},
			Resource: PolicyResource{Type: "*", IDs: []string{"*"}},
			Action: "deny",
		},
		{
			ID: "policy-002", Name: "管理员完全访问", Description: "管理员拥有完全访问权限",
			Priority: 100, Enabled: true,
			Subject: PolicySubject{Type: "user", Roles: []string{"admin"}},
			Resource: PolicyResource{Type: "*", IDs: []string{"*"}},
			Action: "allow",
			Conditions: []Condition{
				{Type: "device-trust", Operator: "greater-than", Value: "80"},
			},
		},
		{
			ID: "policy-003", Name: "MFA 要求策略", Description: "敏感资源需要多因素认证",
			Priority: 50, Enabled: true,
			Subject: PolicySubject{Type: "*", IDs: []string{"*"}},
			Resource: PolicyResource{Type: "database", IDs: []string{"*"}},
			Action: "require-mfa",
			Constraints: []Constraint{
				{Type: "mfa-required", Value: "true"},
			},
		},
	}

	now := time.Now()
	for _, policy := range defaultPolicies {
		policy.CreatedAt = now
		policy.UpdatedAt = now
		policy.CreatedBy = "system"
		m.policies[policy.ID] = policy
	}
}

// RegisterDevice 注册设备
func (m *Manager) RegisterDevice(device *DeviceTrust) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device.DeviceID == "" {
		return fmt.Errorf("device_id is required")
	}

	device.ID = fmt.Sprintf("dev-%d", time.Now().UnixNano())
	device.Status = "untrusted"
	device.RegisteredAt = time.Now()
	device.UpdatedAt = time.Now()
	device.LastSeen = time.Now()

	// 初始化信任评分
	device.TrustScore = m.calculateInitialTrustScore(device)

	m.devices[device.ID] = device
	m.logger.Info("Registered device",
		zap.String("id", device.ID),
		zap.String("device_id", device.DeviceID),
		zap.String("name", device.DeviceName),
	)
	return nil
}

// calculateInitialTrustScore 计算初始信任评分
func (m *Manager) calculateInitialTrustScore(device *DeviceTrust) TrustScore {
	// 基础信任分
	deviceScore := 50.0
	identityScore := 50.0
	networkScore := 50.0
	behaviorScore := 50.0
	complianceScore := 50.0

	// 根据设备类型调整
	switch device.DeviceType {
	case "server":
		deviceScore = 70.0
	case "desktop":
		deviceScore = 60.0
	case "mobile":
		deviceScore = 50.0
	case "iot":
		deviceScore = 40.0
	}

	// 计算综合分
	overall := (deviceScore*0.3 + identityScore*0.2 + networkScore*0.2 + behaviorScore*0.15 + complianceScore*0.15)

	return TrustScore{
		Overall:    math.Round(overall*100) / 100,
		Device:     deviceScore,
		Identity:   identityScore,
		Network:    networkScore,
		Behavior:   behaviorScore,
		Compliance: complianceScore,
		Factors: []TrustFactor{
			{Name: "device-type", Score: deviceScore, Weight: 0.3, Details: "设备类型评估"},
			{Name: "identity", Score: identityScore, Weight: 0.2, Details: "身份验证状态"},
			{Name: "network", Score: networkScore, Weight: 0.2, Details: "网络安全评估"},
			{Name: "behavior", Score: behaviorScore, Weight: 0.15, Details: "行为模式分析"},
			{Name: "compliance", Score: complianceScore, Weight: 0.15, Details: "合规状态"},
		},
		UpdatedAt: time.Now(),
	}
}

// EvaluateTrust 评估信任
func (m *Manager) EvaluateTrust(deviceID string) (*TrustScore, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 查找设备
	var device *DeviceTrust
	for _, d := range m.devices {
		if d.DeviceID == deviceID || d.ID == deviceID {
			device = d
			break
		}
	}

	if device == nil {
		return nil, fmt.Errorf("device %s not found", deviceID)
	}

	// 重新计算信任评分
	score := m.calculateTrustScore(device)
	device.TrustScore = score
	device.UpdatedAt = time.Now()

	// 根据评分更新状态
	if score.Overall >= 80 {
		device.Status = "trusted"
	} else if score.Overall >= 50 {
		device.Status = "untrusted"
	} else {
		device.Status = "quarantined"
	}

	m.trustScores[deviceID] = &score
	m.logger.Info("Evaluated trust",
		zap.String("device", deviceID),
		zap.Float64("score", score.Overall),
		zap.String("status", device.Status),
	)

	return &score, nil
}

// calculateTrustScore 计算信任评分
func (m *Manager) calculateTrustScore(device *DeviceTrust) TrustScore {
	deviceScore := 50.0
	identityScore := 50.0
	networkScore := 50.0
	behaviorScore := 50.0
	complianceScore := 50.0

	// 根据设备类型
	switch device.DeviceType {
	case "server":
		deviceScore = 80.0
	case "desktop":
		deviceScore = 70.0
	case "mobile":
		deviceScore = 60.0
	}

	// 根据合规状态
	if device.ComplianceState.Compliant {
		complianceScore = 90.0
	}

	// 根据最后活跃时间
	hoursSinceLastSeen := time.Since(device.LastSeen).Hours()
	if hoursSinceLastSeen < 24 {
		behaviorScore = 80.0
	} else if hoursSinceLastSeen < 168 {
		behaviorScore = 60.0
	} else {
		behaviorScore = 40.0
	}

	overall := deviceScore*0.3 + identityScore*0.2 + networkScore*0.2 + behaviorScore*0.15 + complianceScore*0.15

	return TrustScore{
		Overall:    math.Round(overall*100) / 100,
		Device:     deviceScore,
		Identity:   identityScore,
		Network:    networkScore,
		Behavior:   behaviorScore,
		Compliance: complianceScore,
		Factors: []TrustFactor{
			{Name: "device-type", Score: deviceScore, Weight: 0.3},
			{Name: "identity", Score: identityScore, Weight: 0.2},
			{Name: "network", Score: networkScore, Weight: 0.2},
			{Name: "behavior", Score: behaviorScore, Weight: 0.15},
			{Name: "compliance", Score: complianceScore, Weight: 0.15},
		},
		UpdatedAt: time.Now(),
	}
}

// GetDeviceTrust 获取设备信任信息
func (m *Manager) GetDeviceTrust(id string) (*DeviceTrust, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[id]
	if !ok {
		return nil, fmt.Errorf("device %s not found", id)
	}
	return device, nil
}

// ListDevices 获取设备列表
func (m *Manager) ListDevices(filter *DeviceFilter) []*DeviceTrust {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var devices []*DeviceTrust
	for _, device := range m.devices {
		if m.matchesDeviceFilter(device, filter) {
			devices = append(devices, device)
		}
	}
	return devices
}

// matchesDeviceFilter 检查设备是否匹配过滤器
func (m *Manager) matchesDeviceFilter(device *DeviceTrust, filter *DeviceFilter) bool {
	if filter == nil {
		return true
	}
	if filter.Status != "" && device.Status != filter.Status {
		return false
	}
	if filter.DeviceType != "" && device.DeviceType != filter.DeviceType {
		return false
	}
	if filter.Owner != "" && device.Owner != filter.Owner {
		return false
	}
	if filter.MinScore != nil && device.TrustScore.Overall < *filter.MinScore {
		return false
	}
	return true
}

// SetPolicy 设置访问策略
func (m *Manager) SetPolicy(policy *AccessPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = fmt.Sprintf("policy-%d", time.Now().UnixNano())
	}

	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now

	m.policies[policy.ID] = policy
	m.logger.Info("Set access policy",
		zap.String("id", policy.ID),
		zap.String("name", policy.Name),
		zap.String("action", policy.Action),
	)
	return nil
}

// GetPolicy 获取访问策略
func (m *Manager) GetPolicy(id string) (*AccessPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", id)
	}
	return policy, nil
}

// ListPolicies 获取策略列表
func (m *Manager) ListPolicies() []*AccessPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var policies []*AccessPolicy
	for _, policy := range m.policies {
		policies = append(policies, policy)
	}
	return policies
}

// DeletePolicy 删除策略
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[id]; !ok {
		return fmt.Errorf("policy %s not found", id)
	}

	delete(m.policies, id)
	m.logger.Info("Deleted access policy", zap.String("id", id))
	return nil
}

// CheckAccess 检查访问权限
func (m *Manager) CheckAccess(subjectType, subjectID, resourceType, resourceID string) (bool, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 按优先级排序检查策略
	var matchedPolicy *AccessPolicy
	for _, policy := range m.policies {
		if !policy.Enabled {
			continue
		}
		if m.matchesPolicy(policy, subjectType, subjectID, resourceType, resourceID) {
			if matchedPolicy == nil || policy.Priority > matchedPolicy.Priority {
				matchedPolicy = policy
			}
		}
	}

	if matchedPolicy == nil {
		return false, "no-matching-policy"
	}

	return matchedPolicy.Action == "allow" || matchedPolicy.Action == "require-mfa", matchedPolicy.Action
}

// matchesPolicy 检查策略是否匹配
func (m *Manager) matchesPolicy(policy *AccessPolicy, subjectType, subjectID, resourceType, resourceID string) bool {
	// 检查主体
	if policy.Subject.Type != "*" && policy.Subject.Type != subjectType {
		return false
	}
	if len(policy.Subject.IDs) > 0 && policy.Subject.IDs[0] != "*" {
		found := false
		for _, id := range policy.Subject.IDs {
			if id == subjectID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查资源
	if policy.Resource.Type != "*" && policy.Resource.Type != resourceType {
		return false
	}
	if len(policy.Resource.IDs) > 0 && policy.Resource.IDs[0] != "*" {
		found := false
		for _, id := range policy.Resource.IDs {
			if id == resourceID {
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

// CreateSession 创建认证会话
func (m *Manager) CreateSession(session *AuthSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session.ID == "" {
		session.ID = fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	session.StartTime = time.Now()
	session.LastActivity = time.Now()
	session.Status = "active"

	m.sessions[session.ID] = session
	m.logger.Info("Created auth session",
		zap.String("id", session.ID),
		zap.String("user", session.UserID),
	)
	return nil
}

// GetSession 获取会话
func (m *Manager) GetSession(id string) (*AuthSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %s not found", id)
	}
	return session, nil
}

// ListSessions 获取会话列表
func (m *Manager) ListSessions(filter *SessionFilter) []*AuthSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []*AuthSession
	for _, session := range m.sessions {
		if m.matchesSessionFilter(session, filter) {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// matchesSessionFilter 检查会话是否匹配过滤器
func (m *Manager) matchesSessionFilter(session *AuthSession, filter *SessionFilter) bool {
	if filter == nil {
		return true
	}
	if filter.UserID != "" && session.UserID != filter.UserID {
		return false
	}
	if filter.DeviceID != "" && session.DeviceID != filter.DeviceID {
		return false
	}
	if filter.Status != "" && session.Status != filter.Status {
		return false
	}
	return true
}

// RevokeSession 撤销会话
func (m *Manager) RevokeSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session %s not found", id)
	}

	session.Status = "revoked"
	m.logger.Info("Revoked auth session", zap.String("id", id))
	return nil
}

// BlockThreat 阻断威胁
func (m *Manager) BlockThreat(threat *ThreatEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if threat.ID == "" {
		threat.ID = fmt.Sprintf("threat-%d", time.Now().UnixNano())
	}
	threat.Timestamp = time.Now()
	threat.Status = "detected"

	m.threats[threat.ID] = threat
	m.logger.Warn("Threat detected and blocked",
		zap.String("id", threat.ID),
		zap.String("type", threat.Type),
		zap.String("severity", threat.Severity),
		zap.String("source", threat.Source),
	)

	// 自动采取措施
	go m.mitigateThreat(threat)

	return nil
}

// mitigateThreat 缓解威胁
func (m *Manager) mitigateThreat(threat *ThreatEvent) {
	// 根据威胁类型采取不同措施
	switch threat.Type {
	case "unauthorized-access":
		// 撤销相关会话
		for _, session := range m.sessions {
			if session.IPAddress == threat.Source {
				m.mu.Lock()
				session.Status = "revoked"
				m.mu.Unlock()
				threat.Actions = append(threat.Actions, fmt.Sprintf("revoked-session-%s", session.ID))
			}
		}
	case "anomaly":
		// 标记设备为不可信
		for _, device := range m.devices {
			if device.IPAddress == threat.Source {
				m.mu.Lock()
				device.Status = "quarantined"
				m.mu.Unlock()
				threat.Actions = append(threat.Actions, fmt.Sprintf("quarantined-device-%s", device.ID))
			}
		}
	}

	threat.Status = "mitigated"
	m.logger.Info("Threat mitigated",
		zap.String("id", threat.ID),
		zap.Strings("actions", threat.Actions),
	)
}

// GetThreat 获取威胁事件
func (m *Manager) GetThreat(id string) (*ThreatEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	threat, ok := m.threats[id]
	if !ok {
		return nil, fmt.Errorf("threat %s not found", id)
	}
	return threat, nil
}

// ListThreats 获取威胁列表
func (m *Manager) ListThreats(filter *ThreatFilter) []*ThreatEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var threats []*ThreatEvent
	for _, threat := range m.threats {
		if m.matchesThreatFilter(threat, filter) {
			threats = append(threats, threat)
		}
	}
	return threats
}

// matchesThreatFilter 检查威胁是否匹配过滤器
func (m *Manager) matchesThreatFilter(threat *ThreatEvent, filter *ThreatFilter) bool {
	if filter == nil {
		return true
	}
	if filter.StartTime != nil && threat.Timestamp.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && threat.Timestamp.After(*filter.EndTime) {
		return false
	}
	if len(filter.Types) > 0 {
		found := false
		for _, t := range filter.Types {
			if threat.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(filter.Severities) > 0 {
		found := false
		for _, s := range filter.Severities {
			if threat.Severity == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(filter.Statuses) > 0 {
		found := false
		for _, s := range filter.Statuses {
			if threat.Status == s {
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

// ResolveThreat 解决威胁
func (m *Manager) ResolveThreat(id, notes string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	threat, ok := m.threats[id]
	if !ok {
		return fmt.Errorf("threat %s not found", id)
	}

	now := time.Now()
	threat.Status = "resolved"
	threat.ResolvedAt = &now
	threat.Notes = notes

	return nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats() *TrustStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &TrustStats{
		TotalDevices: len(m.devices),
	}

	totalScore := 0.0
	for _, device := range m.devices {
		totalScore += device.TrustScore.Overall
		switch device.Status {
		case "trusted":
			stats.TrustedDevices++
		case "untrusted":
			stats.UntrustedDevices++
		case "compromised":
			stats.CompromisedDevices++
		}
	}

	if stats.TotalDevices > 0 {
		stats.AverageScore = totalScore / float64(stats.TotalDevices)
	}

	for _, session := range m.sessions {
		if session.Status == "active" {
			stats.ActiveSessions++
		}
	}

	for _, threat := range m.threats {
		stats.ThreatsDetected++
		if threat.Status == "mitigated" || threat.Status == "resolved" {
			stats.ThreatsMitigated++
		}
	}

	return stats
}
