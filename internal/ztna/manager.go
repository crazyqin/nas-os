package ztna

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 错误定义 ==========

var (
	// ErrDeviceNotFound 设备未找到.
	ErrDeviceNotFound = errors.New("设备未找到")
	// ErrPolicyNotFound 策略未找到.
	ErrPolicyNotFound = errors.New("策略未找到")
	// ErrSessionNotFound 会话未找到.
	ErrSessionNotFound = errors.New("会话未找到")
	// ErrAccessDenied 访问被拒绝.
	ErrAccessDenied = errors.New("访问被拒绝")
	// ErrTrustScoreTooLow 信任分过低.
	ErrTrustScoreTooLow = errors.New("设备信任分过低")
	// ErrSessionExpired 会话已过期.
	ErrSessionExpired = errors.New("会话已过期")
)

// ========== 管理器 ==========

// Manager ZTNA 核心管理器，管理策略、设备信任和会话.
type Manager struct {
	mu         sync.RWMutex
	policies   map[string]*Policy      // policyID -> Policy
	devices    map[string]*DeviceTrust // deviceID -> DeviceTrust
	sessions   map[string]*Session     // sessionID -> Session
	identities map[string]*Identity    // userID -> Identity
}

// NewManager 创建 ZTNA 管理器.
func NewManager() *Manager {
	return &Manager{
		policies:   make(map[string]*Policy),
		devices:    make(map[string]*DeviceTrust),
		sessions:   make(map[string]*Session),
		identities: make(map[string]*Identity),
	}
}

// ========== 策略管理 ==========

// CreatePolicy 创建访问策略.
func (m *Manager) CreatePolicy(req CreatePolicyRequest) (*Policy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy := &Policy{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Enabled:     true,
		Priority:    req.Priority,
		Rules:       req.Rules,
		Conditions:  req.Conditions,
		MinTrust:    req.MinTrust,
		Action:      req.Action,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 为规则生成 ID
	for i := range policy.Rules {
		if policy.Rules[i].ID == "" {
			policy.Rules[i].ID = uuid.New().String()
		}
	}

	m.policies[policy.ID] = policy
	return policy, nil
}

// GetPolicy 获取策略.
func (m *Manager) GetPolicy(policyID string) (*Policy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[policyID]
	if !ok {
		return nil, ErrPolicyNotFound
	}
	return policy, nil
}

// ListPolicies 列出所有策略.
func (m *Manager) ListPolicies() []*Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*Policy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// DeletePolicy 删除策略.
func (m *Manager) DeletePolicy(policyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[policyID]; !ok {
		return ErrPolicyNotFound
	}
	delete(m.policies, policyID)
	return nil
}

// ========== 设备信任管理 ==========

// VerifyDevice 验证设备并计算信任分.
func (m *Manager) VerifyDevice(req VerifyRequest) (*DeviceTrust, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 计算信任分
	trustFactors := m.calculateTrustFactors(req)
	trustScore := m.calculateTrustScore(trustFactors)

	// 确定设备状态
	status := m.determineDeviceStatus(trustScore, true)
	compliant := trustScore >= 50

	// 构建设备信任信息
	device := &DeviceTrust{
		DeviceID:      req.DeviceID,
		UserID:        req.UserID,
		DeviceName:    req.DeviceName,
		DeviceType:    req.DeviceType,
		OS:            req.OS,
		OSVersion:     req.OSVersion,
		Compliant:     compliant,
		ManagedDevice: true, // 简化实现，实际应查询 MDM
		PatchLevel:    req.PatchLevel,
		TrustScore:    trustScore,
		TrustFactors:  trustFactors,
		LastVerified:  time.Now(),
		Status:        status,
	}

	m.devices[req.DeviceID] = device
	return device, nil
}

// GetDeviceTrust 获取设备信任信息.
func (m *Manager) GetDeviceTrust(deviceID string) (*DeviceTrust, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return nil, ErrDeviceNotFound
	}
	return device, nil
}

// CheckTrustScore 检查设备信任分是否满足要求.
func (m *Manager) CheckTrustScore(deviceID string, minTrust int) (bool, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return false, 0, ErrDeviceNotFound
	}

	return device.TrustScore >= minTrust, device.TrustScore, nil
}

// ========== 策略评估 ==========

// EvaluatePolicy 评估访问请求是否满足策略.
func (m *Manager) EvaluatePolicy(userID, deviceID, resource, action string) (*Policy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 获取设备信任信息
	device, deviceOK := m.devices[deviceID]

	// 收集所有匹配的策略
	var matchedPolicies []*Policy
	for _, policy := range m.policies {
		if !policy.Enabled {
			continue
		}

		// 检查规则匹配
		if m.matchesPolicy(policy, userID, resource, action) {
			// 检查信任分
			if deviceOK && device.TrustScore < policy.MinTrust {
				continue
			}

			// 检查条件
			if m.evaluateConditions(policy.Conditions) {
				matchedPolicies = append(matchedPolicies, policy)
			}
		}
	}

	if len(matchedPolicies) == 0 {
		return nil, ErrAccessDenied
	}

	// 返回优先级最高的策略
	best := matchedPolicies[0]
	for _, p := range matchedPolicies[1:] {
		if p.Priority < best.Priority {
			best = p
		}
	}

	return best, nil
}

// ========== 会话管理 ==========

// CreateSession 创建访问会话.
func (m *Manager) CreateSession(userID, deviceID, resource string, actions []string, policyID string, trustScore int) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session := &Session{
		ID:           uuid.New().String(),
		UserID:       userID,
		DeviceID:     deviceID,
		Resource:     resource,
		Actions:      actions,
		TrustScore:   trustScore,
		PolicyID:     policyID,
		StartedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(8 * time.Hour), // 默认8小时
		LastActivity: time.Now(),
		Status:       SessionStatusActive,
	}

	m.sessions[session.ID] = session
	return session, nil
}

// GetSession 获取会话.
func (m *Manager) GetSession(sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return session, nil
}

// ListSessions 列出所有活跃会话.
func (m *Manager) ListSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// RevokeAccess 撤销会话（立即终止访问）.
func (m *Manager) RevokeAccess(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}

	session.Status = SessionStatusRevoked
	return nil
}

// RevokeAllUserSessions 撤销用户的所有会话.
func (m *Manager) RevokeAllUserSessions(userID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, s := range m.sessions {
		if s.UserID == userID && s.Status == SessionStatusActive {
			s.Status = SessionStatusRevoked
			count++
		}
	}
	return count
}

// ========== 内部辅助方法 ==========

// calculateTrustFactors 计算信任因素.
func (m *Manager) calculateTrustFactors(req VerifyRequest) []TrustFactor {
	var factors []TrustFactor

	// 操作系统信任
	osScore := 80
	if req.OS != "" {
		switch strings.ToLower(req.OS) {
		case "windows", "macos", "linux":
			osScore = 90
		case "android", "ios":
			osScore = 85
		default:
			osScore = 60
		}
	}
	factors = append(factors, TrustFactor{
		Name:   "操作系统",
		Weight: 20,
		Score:  osScore,
		Detail: fmt.Sprintf("%s %s", req.OS, req.OSVersion),
	})

	// 补丁级别信任
	patchScore := 70
	if req.PatchLevel != "" {
		switch strings.ToLower(req.PatchLevel) {
		case "latest", "current":
			patchScore = 100
		case "recent":
			patchScore = 80
		case "outdated":
			patchScore = 40
		}
	}
	factors = append(factors, TrustFactor{
		Name:   "补丁级别",
		Weight: 30,
		Score:  patchScore,
		Detail: fmt.Sprintf("补丁级别: %s", req.PatchLevel),
	})

	// 设备类型信任
	deviceScore := 70
	switch strings.ToLower(req.DeviceType) {
	case "desktop", "laptop":
		deviceScore = 90
	case "mobile", "tablet":
		deviceScore = 80
	}
	factors = append(factors, TrustFactor{
		Name:   "设备类型",
		Weight: 20,
		Score:  deviceScore,
		Detail: fmt.Sprintf("设备类型: %s", req.DeviceType),
	})

	// 管理状态信任
	factors = append(factors, TrustFactor{
		Name:   "设备管理",
		Weight: 30,
		Score:  80, // 简化实现，实际应查询 MDM
		Detail: "受管理设备",
	})

	return factors
}

// calculateTrustScore 计算加权信任分.
func (m *Manager) calculateTrustScore(factors []TrustFactor) int {
	if len(factors) == 0 {
		return 0
	}

	totalWeight := 0
	weightedSum := 0
	for _, f := range factors {
		weightedSum += f.Score * f.Weight
		totalWeight += f.Weight
	}

	if totalWeight == 0 {
		return 0
	}

	score := weightedSum / totalWeight
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return score
}

// determineDeviceStatus 根据信任分确定设备状态.
func (m *Manager) determineDeviceStatus(score int, compliant bool) DeviceStatus {
	if !compliant {
		return DeviceStatusCompromised
	}
	switch {
	case score >= 80:
		return DeviceStatusTrusted
	case score >= 50:
		return DeviceStatusUnknown
	default:
		return DeviceStatusBlocked
	}
}

// matchesPolicy 检查请求是否匹配策略规则.
func (m *Manager) matchesPolicy(policy *Policy, userID, resource, action string) bool {
	if len(policy.Rules) == 0 {
		return true // 无规则则允许
	}

	for _, rule := range policy.Rules {
		if !rule.Enabled {
			continue
		}

		// 检查身份匹配
		if rule.Identity != "" && rule.Identity != userID {
			continue
		}

		// 检查资源匹配（支持通配符）
		if rule.Resource != "" && !m.matchesResource(rule.Resource, resource) {
			continue
		}

		// 检查操作匹配
		if len(rule.Actions) > 0 && !contains(rule.Actions, action) {
			continue
		}

		return true
	}

	return false
}

// matchesResource 资源匹配（支持简单通配符）.
func (m *Manager) matchesResource(pattern, resource string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == resource {
		return true
	}
	// 支持前缀通配符: "api/*" 匹配 "api/users"
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(resource, prefix+"/")
	}
	return false
}

// evaluateConditions 评估策略条件.
func (m *Manager) evaluateConditions(conditions []Condition) bool {
	if len(conditions) == 0 {
		return true
	}

	now := time.Now()
	for _, cond := range conditions {
		switch cond.Type {
		case ConditionTime:
			if !evaluateTimeCondition(cond, now) {
				return false
			}
		case ConditionLocation, ConditionNetwork, ConditionDeviceOS:
			// 简化实现：实际应接入相应服务
			continue
		}
	}

	return true
}

// evaluateTimeCondition 评估时间条件.
func evaluateTimeCondition(cond Condition, now time.Time) bool {
	hour := now.Hour()
	switch cond.Operator {
	case "gte":
		var h int
		if _, err := fmt.Sscanf(cond.Value, "%d", &h); err == nil {
			return hour >= h
		}
	case "lte":
		var h int
		if _, err := fmt.Sscanf(cond.Value, "%d", &h); err == nil {
			return hour <= h
		}
	case "between":
		parts := strings.Split(cond.Value, "-")
		if len(parts) == 2 {
			var start, end int
			if _, err := fmt.Sscanf(parts[0], "%d", &start); err == nil {
				if _, err := fmt.Sscanf(parts[1], "%d", &end); err == nil {
					return hour >= start && hour <= end
				}
			}
		}
	}
	return true
}

// contains 检查切片是否包含指定元素.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
