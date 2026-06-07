// Package zerotrustgw 提供零信任网关核心管理逻辑
package zerotrustgw

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 零信任网关管理器
type Manager struct {
	mu             sync.RWMutex
	logger         *zap.Logger
	config         *ZeroTrustConfig
	policies       map[string]*TrustPolicy
	sessions       map[string]*SessionInfo
	auditLog       []*AuditEntry
	deviceProfiles map[string]*DeviceProfile
	trustScores    map[string]*TrustScore
	stopChan       chan struct{}
	running        bool
}

// NewManager 创建零信任网关管理器
func NewManager(logger *zap.Logger, config *ZeroTrustConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultZeroTrustConfig()
	}

	return &Manager{
		logger:         logger,
		config:         config,
		policies:       make(map[string]*TrustPolicy),
		sessions:       make(map[string]*SessionInfo),
		auditLog:       make([]*AuditEntry, 0),
		deviceProfiles: make(map[string]*DeviceProfile),
		trustScores:    make(map[string]*TrustScore),
		stopChan:       make(chan struct{}),
	}
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// EvaluateAccess 评估访问请求
func (m *Manager) EvaluateAccess(ctx context.Context, req *AccessRequest) (*VerificationResult, error) {
	if !m.config.Enabled {
		return &VerificationResult{
			ID:         generateID(),
			RequestID:  req.ID,
			Decision:   DecisionAllow,
			TrustLevel: TrustLevelHigh,
			Timestamp:  time.Now(),
		}, nil
	}

	start := time.Now()

	if req.ID == "" {
		req.ID = generateID()
	}
	if req.Timestamp.IsZero() {
		req.Timestamp = start
	}

	// 计算信任分数
	trustScore := m.CalculateTrustScore(req.UserID, req)

	// 匹配策略
	policy, decision := m.matchPolicies(req, trustScore)

	result := &VerificationResult{
		ID:            generateID(),
		RequestID:     req.ID,
		TrustScore:    trustScore.Overall,
		TrustLevel:    trustScore.Level,
		MatchedPolicy: "",
		Reasons:       make([]string, 0),
		ExpiresAt:     start.Add(time.Duration(m.config.SessionTimeout) * time.Minute),
		Timestamp:     start,
	}

	if policy != nil {
		result.MatchedPolicy = policy.ID
	}

	// 根据信任分数和策略决定访问决策
	switch decision {
	case ActionDeny:
		result.Decision = DecisionDeny
		result.Reasons = append(result.Reasons, "access denied by policy")
	case ActionAllow:
		if trustScore.Overall < m.config.MinTrustScore {
			result.Decision = DecisionMFA
			result.RequiresMFA = true
			result.Reasons = append(result.Reasons, fmt.Sprintf("trust score %.2f below threshold %.2f", trustScore.Overall, m.config.MinTrustScore))
		} else {
			result.Decision = DecisionAllow
		}
	default:
		if trustScore.Overall < m.config.MinTrustScore {
			result.Decision = DecisionMFA
			result.RequiresMFA = true
		} else {
			result.Decision = DecisionAllow
		}
	}

	result.ProcessingTime = time.Since(start)

	// 记录审计日志
	m.addAuditEntry(&AuditEntry{
		ID:         generateID(),
		Timestamp:  start,
		UserID:     req.UserID,
		Resource:   req.Resource,
		Action:     req.Action,
		Decision:   result.Decision,
		TrustScore: trustScore.Overall,
		SourceIP:   req.SourceIP,
		DeviceID:   req.DeviceID,
		PolicyID:   result.MatchedPolicy,
		Reasons:    result.Reasons,
		SessionID:  req.SessionID,
	})

	m.logger.Info("access evaluation completed",
		zap.String("request_id", req.ID),
		zap.String("user_id", req.UserID),
		zap.String("resource", req.Resource),
		zap.String("decision", string(result.Decision)),
		zap.Float64("trust_score", trustScore.Overall),
		zap.Duration("processing_time", result.ProcessingTime))

	return result, nil
}

// CalculateTrustScore 计算信任分数
func (m *Manager) CalculateTrustScore(userID string, req *AccessRequest) *TrustScore {
	m.mu.RLock()
	existing, exists := m.trustScores[userID]
	m.mu.RUnlock()

	score := &TrustScore{
		UserID:      userID,
		LastUpdated: time.Now(),
		Factors:     make([]ScoreFactor, 0),
	}

	// 设备信任分数
	deviceScore := 1.0
	if req.DeviceID != "" {
		m.mu.RLock()
		device, ok := m.deviceProfiles[req.DeviceID]
		m.mu.RUnlock()
		if ok {
			deviceScore = device.TrustScore
			if !device.IsCompliant {
				deviceScore *= 0.5
			}
			if !device.IsManaged {
				deviceScore *= 0.8
			}
		}
	}
	score.DeviceScore = deviceScore
	score.Factors = append(score.Factors, ScoreFactor{
		Name:   "device",
		Score:  deviceScore,
		Weight: 0.3,
		Detail: "设备信任评估",
	})

	// 网络信任分数
	networkScore := 1.0
	if req.SourceIP != "" {
		// 简化的网络信任评估
		networkScore = 0.8
	}
	score.NetworkScore = networkScore
	score.Factors = append(score.Factors, ScoreFactor{
		Name:   "network",
		Score:  networkScore,
		Weight: 0.2,
		Detail: "网络环境评估",
	})

	// 行为信任分数
	behaviorScore := 0.8
	if exists {
		behaviorScore = existing.BehaviorScore
	}
	score.BehaviorScore = behaviorScore
	score.Factors = append(score.Factors, ScoreFactor{
		Name:   "behavior",
		Score:  behaviorScore,
		Weight: 0.3,
		Detail: "用户行为分析",
	})

	// 位置信任分数
	locationScore := 1.0
	if req.Location != nil {
		if m.config.GeoFencingEnabled {
			locationScore = m.evaluateLocation(req.Location)
		}
	}
	score.LocationScore = locationScore
	score.Factors = append(score.Factors, ScoreFactor{
		Name:   "location",
		Score:  locationScore,
		Weight: 0.1,
		Detail: "地理位置评估",
	})

	// 时间信任分数
	timeScore := m.evaluateTimeAccess(time.Now())
	score.TimeScore = timeScore
	score.Factors = append(score.Factors, ScoreFactor{
		Name:   "time",
		Score:  timeScore,
		Weight: 0.1,
		Detail: "访问时间评估",
	})

	// 计算加权总分
	score.Overall = score.DeviceScore*0.3 +
		score.NetworkScore*0.2 +
		score.BehaviorScore*0.3 +
		score.LocationScore*0.1 +
		score.TimeScore*0.1

	// 确定信任等级
	score.Level = m.calculateTrustLevel(score.Overall)

	// 更新历史记录
	if exists {
		score.History = existing.History
	}
	score.History = append(score.History, TrustScoreRecord{
		Score:     score.Overall,
		Level:     score.Level,
		Timestamp: time.Now(),
	})

	// 限制历史大小
	if len(score.History) > 100 {
		score.History = score.History[len(score.History)-100:]
	}

	// 保存信任分数
	m.mu.Lock()
	m.trustScores[userID] = score
	m.mu.Unlock()

	return score
}

// EnforcePolicy 执行策略
func (m *Manager) EnforcePolicy(policy *TrustPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = generateID()
	}
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	m.policies[policy.ID] = policy

	m.logger.Info("policy enforced",
		zap.String("policy_id", policy.ID),
		zap.String("name", policy.Name),
		zap.String("action", string(policy.Action)))

	return nil
}

// GetAuditLog 获取审计日志
func (m *Manager) GetAuditLog(limit int, userID string) []*AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]*AuditEntry, 0)

	// 从最新的开始
	for i := len(m.auditLog) - 1; i >= 0; i-- {
		entry := m.auditLog[i]
		if userID != "" && entry.UserID != userID {
			continue
		}
		entries = append(entries, entry)
		if limit > 0 && len(entries) >= limit {
			break
		}
	}

	return entries
}

// matchPolicies 匹配策略
func (m *Manager) matchPolicies(req *AccessRequest, score *TrustScore) (*TrustPolicy, PolicyAction) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matchedPolicy *TrustPolicy
	var matchedAction PolicyAction = ActionAllow

	// 按优先级排序匹配
	for _, policy := range m.policies {
		if !policy.Enabled {
			continue
		}

		// 检查过期时间
		if policy.ExpiresAt != nil && policy.ExpiresAt.Before(time.Now()) {
			continue
		}

		// 检查用户/组匹配
		if len(policy.Users) > 0 && !contains(policy.Users, req.UserID) {
			continue
		}

		// 检查资源匹配
		if len(policy.Resources) > 0 && !matchesAny(policy.Resources, req.Resource) {
			continue
		}

		// 检查条件匹配
		if m.evaluateConditions(policy.Conditions, req, score) {
			if matchedPolicy == nil || policy.Priority > matchedPolicy.Priority {
				matchedPolicy = policy
				matchedAction = policy.Action
			}
		}
	}

	return matchedPolicy, matchedAction
}

// evaluateConditions 评估条件
func (m *Manager) evaluateConditions(conditions []Condition, req *AccessRequest, score *TrustScore) bool {
	for _, cond := range conditions {
		switch cond.Type {
		case "trust_level":
			if !m.evaluateTrustLevelCondition(cond, score) {
				return false
			}
		case "source_ip":
			if !m.evaluateIPCondition(cond, req.SourceIP) {
				return false
			}
		case "time_range":
			if !m.evaluateTimeCondition(cond) {
				return false
			}
		case "device_compliant":
			if !m.evaluateDeviceCondition(cond, req.DeviceID) {
				return false
			}
		}
	}
	return true
}

// evaluateTrustLevelCondition 评估信任等级条件
func (m *Manager) evaluateTrustLevelCondition(cond Condition, score *TrustScore) bool {
	for _, v := range cond.Values {
		switch cond.Operator {
		case "eq":
			if string(score.Level) == v {
				return true
			}
		case "gte":
			if m.trustLevelValue(score.Level) >= m.trustLevelValue(TrustLevel(v)) {
				return true
			}
		case "lte":
			if m.trustLevelValue(score.Level) <= m.trustLevelValue(TrustLevel(v)) {
				return true
			}
		}
	}
	return false
}

// evaluateIPCondition 评估 IP 条件
func (m *Manager) evaluateIPCondition(cond Condition, ip string) bool {
	switch cond.Operator {
	case "in":
		for _, v := range cond.Values {
			if v == ip {
				return true
			}
		}
	case "not_in":
		for _, v := range cond.Values {
			if v == ip {
				return false
			}
		}
		return true
	}
	return false
}

// evaluateTimeCondition 评估时间条件
func (m *Manager) evaluateTimeCondition(cond Condition) bool {
	now := time.Now()
	hour := now.Hour()

	switch cond.Operator {
	case "between":
		// 简化处理，假设值为 "start_hour-end_hour"
		if len(cond.Values) > 0 {
			var start, end int
			fmt.Sscanf(cond.Values[0], "%d-%d", &start, &end)
			return hour >= start && hour <= end
		}
	case "outside":
		if len(cond.Values) > 0 {
			var start, end int
			fmt.Sscanf(cond.Values[0], "%d-%d", &start, &end)
			return hour < start || hour > end
		}
	}
	return true
}

// evaluateDeviceCondition 评估设备条件
func (m *Manager) evaluateDeviceCondition(cond Condition, deviceID string) bool {
	m.mu.RLock()
	device, ok := m.deviceProfiles[deviceID]
	m.mu.RUnlock()

	if !ok {
		return false
	}

	switch cond.Operator {
	case "is_compliant":
		return device.IsCompliant
	case "is_managed":
		return device.IsManaged
	}
	return true
}

// evaluateLocation 评估位置信任
func (m *Manager) evaluateLocation(loc *GeoLocation) float64 {
	if loc == nil {
		return 1.0
	}

	// 检查禁止国家
	for _, c := range m.config.BlockedCountries {
		if loc.Country == c {
			return 0.0
		}
	}

	// 检查允许国家
	if len(m.config.AllowedCountries) > 0 {
		for _, c := range m.config.AllowedCountries {
			if loc.Country == c {
				return 1.0
			}
		}
		return 0.3
	}

	return 0.8
}

// evaluateTimeAccess 评估时间访问
func (m *Manager) evaluateTimeAccess(t time.Time) float64 {
	hour := t.Hour()

	// 工作时间（9-18）信任度高
	if hour >= 9 && hour <= 18 {
		return 1.0
	}
	// 非工作时间信任度降低
	if hour >= 6 && hour <= 22 {
		return 0.7
	}
	// 深夜信任度最低
	return 0.4
}

// calculateTrustLevel 计算信任等级
func (m *Manager) calculateTrustLevel(score float64) TrustLevel {
	if score >= 0.8 {
		return TrustLevelHigh
	}
	if score >= 0.6 {
		return TrustLevelMedium
	}
	if score >= 0.3 {
		return TrustLevelLow
	}
	return TrustLevelNone
}

// trustLevelValue 信任等级数值
func (m *Manager) trustLevelValue(level TrustLevel) int {
	switch level {
	case TrustLevelHigh:
		return 4
	case TrustLevelMedium:
		return 3
	case TrustLevelLow:
		return 2
	case TrustLevelNone:
		return 1
	default:
		return 0
	}
}

// addAuditEntry 添加审计日志
func (m *Manager) addAuditEntry(entry *AuditEntry) {
	if !m.config.AuditEnabled {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.auditLog = append(m.auditLog, entry)

	// 限制日志大小
	if len(m.auditLog) > 10000 {
		m.auditLog = m.auditLog[len(m.auditLog)-10000:]
	}
}

// contains 检查切片是否包含元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// matchesAny 检查是否匹配任何模式
func matchesAny(patterns []string, target string) bool {
	for _, p := range patterns {
		if p == "*" || p == target {
			return true
		}
	}
	return false
}

// CreatePolicy 创建策略
func (m *Manager) CreatePolicy(req *TrustPolicy) (*TrustPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.ID == "" {
		req.ID = generateID()
	}
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()

	m.policies[req.ID] = req
	return req, nil
}

// GetPolicy 获取策略
func (m *Manager) GetPolicy(id string) (*TrustPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy not found: %s", id)
	}
	return policy, nil
}

// ListPolicies 列出所有策略
func (m *Manager) ListPolicies() []*TrustPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*TrustPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// UpdatePolicy 更新策略
func (m *Manager) UpdatePolicy(id string, req *TrustPolicy) (*TrustPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy not found: %s", id)
	}

	policy.Name = req.Name
	policy.Description = req.Description
	policy.Priority = req.Priority
	policy.Enabled = req.Enabled
	policy.Conditions = req.Conditions
	policy.Action = req.Action
	policy.MinTrust = req.MinTrust
	policy.Resources = req.Resources
	policy.Users = req.Users
	policy.Groups = req.Groups
	policy.ExpiresAt = req.ExpiresAt
	policy.UpdatedAt = time.Now()

	return policy, nil
}

// DeletePolicy 删除策略
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[id]; !ok {
		return fmt.Errorf("policy not found: %s", id)
	}
	delete(m.policies, id)
	return nil
}

// RegisterDevice 注册设备
func (m *Manager) RegisterDevice(device *DeviceProfile) (*DeviceProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device.DeviceID == "" {
		device.DeviceID = generateID()
	}
	device.LastSeen = time.Now()

	m.deviceProfiles[device.DeviceID] = device
	return device, nil
}

// GetDevice 获取设备信息
func (m *Manager) GetDevice(deviceID string) (*DeviceProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.deviceProfiles[deviceID]
	if !ok {
		return nil, fmt.Errorf("device not found: %s", deviceID)
	}
	return device, nil
}

// ListDevices 列出所有设备
func (m *Manager) ListDevices() []*DeviceProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*DeviceProfile, 0, len(m.deviceProfiles))
	for _, d := range m.deviceProfiles {
		devices = append(devices, d)
	}
	return devices
}

// GetTrustScore 获取用户信任分数
func (m *Manager) GetTrustScore(userID string) (*TrustScore, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	score, ok := m.trustScores[userID]
	if !ok {
		return nil, fmt.Errorf("trust score not found for user: %s", userID)
	}
	return score, nil
}

// CreateSession 创建会话
func (m *Manager) CreateSession(userID, deviceID, sourceIP string) *SessionInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	session := &SessionInfo{
		SessionID:    generateID(),
		UserID:       userID,
		DeviceID:     deviceID,
		SourceIP:     sourceIP,
		StartedAt:    time.Now(),
		LastActivity: time.Now(),
		TrustScore:   0.5,
		IsActive:     true,
		ExpiresAt:    time.Now().Add(time.Duration(m.config.SessionTimeout) * time.Minute),
	}

	m.sessions[session.SessionID] = session
	return session
}

// GetSession 获取会话
func (m *Manager) GetSession(sessionID string) (*SessionInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return session, nil
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *ZeroTrustConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *ZeroTrustConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	allowed := 0
	denied := 0
	mfaRequired := 0

	for _, entry := range m.auditLog {
		switch entry.Decision {
		case DecisionAllow:
			allowed++
		case DecisionDeny:
			denied++
		case DecisionMFA, DecisionStepUp:
			mfaRequired++
		}
	}

	return map[string]interface{}{
		"total_policies":  len(m.policies),
		"total_devices":   len(m.deviceProfiles),
		"active_sessions": len(m.sessions),
		"audit_entries":   len(m.auditLog),
		"access_allowed":  allowed,
		"access_denied":   denied,
		"mfa_required":    mfaRequired,
		"tracked_users":   len(m.trustScores),
	}
}
