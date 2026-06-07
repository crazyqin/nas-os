// Package zerotrustaccess 提供动态权限调整和风险管理
package zerotrustaccess

import (
	"fmt"
	"sync"
	"time"
)

// ========== 风险评分系统 ==========

// RiskEngine 风险评估引擎
type RiskEngine struct {
	mu            sync.RWMutex
	riskProfiles  map[string]*RiskProfile
	riskFactors   map[string]RiskFactorConfig
	thresholds    RiskThresholds
	alertHandlers []AlertHandler
}

// RiskProfile 风险档案
type RiskProfile struct {
	UserID       string             `json:"user_id"`
	DeviceID     string             `json:"device_id"`
	CurrentScore float64            `json:"current_score"` // 0-100
	History      []RiskScoreRecord  `json:"history"`
	Factors      map[string]float64 `json:"factors"`
	LastUpdated  time.Time          `json:"last_updated"`
	Level        RiskLevel          `json:"level"`
}

// RiskScoreRecord 风险分数记录
type RiskScoreRecord struct {
	Score     float64   `json:"score"`
	Timestamp time.Time `json:"timestamp"`
	Trigger   string    `json:"trigger"`
}

// RiskLevel 风险级别
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

// RiskFactorConfig 风险因素配置
type RiskFactorConfig struct {
	Name        string  `json:"name"`
	Weight      float64 `json:"weight"`
	BaseScore   float64 `json:"base_score"`
	DecayRate   float64 `json:"decay_rate"`
	Description string  `json:"description"`
}

// RiskThresholds 风险阈值
type RiskThresholds struct {
	Low      float64 `json:"low"`      // < 30
	Medium   float64 `json:"medium"`   // 30-60
	High     float64 `json:"high"`     // 60-85
	Critical float64 `json:"critical"` // >= 85
}

// AlertHandler 告警处理器
type AlertHandler func(alert RiskAlert)

// RiskAlert 风险告警
type RiskAlert struct {
	Level     RiskLevel `json:"level"`
	UserID    string    `json:"user_id"`
	DeviceID  string    `json:"device_id"`
	Score     float64   `json:"score"`
	Message   string    `json:"message"`
	Trigger   string    `json:"trigger"`
	Timestamp time.Time `json:"timestamp"`
}

// ========== 动态权限 ==========

// DynamicPermission 动态权限
type DynamicPermission struct {
	ID           string                 `json:"id"`
	UserID       string                 `json:"user_id"`
	DeviceID     string                 `json:"device_id"`
	Resource     string                 `json:"resource"`
	BaseLevel    AccessLevel            `json:"base_level"`
	CurrentLevel AccessLevel            `json:"current_level"`
	Constraints  []PermissionConstraint `json:"constraints"`
	ExpiresAt    time.Time              `json:"expires_at"`
	GrantedAt    time.Time              `json:"granted_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	Reason       string                 `json:"reason"`
}

// AccessLevel 访问级别
type AccessLevel int

const (
	AccessLevelNone  AccessLevel = 0 // 无访问
	AccessLevelRead  AccessLevel = 1 // 只读
	AccessLevelWrite AccessLevel = 2 // 读写
	AccessLevelAdmin AccessLevel = 3 // 管理
	AccessLevelFull  AccessLevel = 4 // 完全控制
)

func (l AccessLevel) String() string {
	switch l {
	case AccessLevelNone:
		return "none"
	case AccessLevelRead:
		return "read"
	case AccessLevelWrite:
		return "write"
	case AccessLevelAdmin:
		return "admin"
	case AccessLevelFull:
		return "full"
	default:
		return "unknown"
	}
}

// PermissionConstraint 权限约束
type PermissionConstraint struct {
	Type     string `json:"type"`     // "time", "location", "risk_level", "mfa_required"
	Operator string `json:"operator"` // "eq", "gt", "lt", "in"
	Value    string `json:"value"`
}

// ========== 动态权限管理器 ==========

// PermissionManager 权限管理器
type PermissionManager struct {
	mu                sync.RWMutex
	permissions       map[string]*DynamicPermission
	riskEngine        *RiskEngine
	downgradePolicies []DowngradePolicy
}

// DowngradePolicy 降级策略
type DowngradePolicy struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	RiskLevel  RiskLevel   `json:"risk_level"`
	MaxLevel   AccessLevel `json:"max_level"`
	Conditions []string    `json:"conditions"`
	Enabled    bool        `json:"enabled"`
}

// NewPermissionManager 创建权限管理器
func NewPermissionManager(riskEngine *RiskEngine) *PermissionManager {
	return &PermissionManager{
		permissions:       make(map[string]*DynamicPermission),
		riskEngine:        riskEngine,
		downgradePolicies: getDefaultDowngradePolicies(),
	}
}

// getDefaultDowngradePolicies 获取默认降级策略
func getDefaultDowngradePolicies() []DowngradePolicy {
	return []DowngradePolicy{
		{
			ID:        "high-risk-downgrade",
			Name:      "高风险降级",
			RiskLevel: RiskLevelHigh,
			MaxLevel:  AccessLevelRead,
			Enabled:   true,
		},
		{
			ID:        "critical-risk-block",
			Name:      "极高风险阻断",
			RiskLevel: RiskLevelCritical,
			MaxLevel:  AccessLevelNone,
			Enabled:   true,
		},
	}
}

// GrantPermission 授予权限
func (m *PermissionManager) GrantPermission(perm *DynamicPermission) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.permissionKey(perm.UserID, perm.DeviceID, perm.Resource)

	perm.GrantedAt = time.Now()
	perm.UpdatedAt = time.Now()
	perm.CurrentLevel = perm.BaseLevel

	// 检查风险并调整
	if m.riskEngine != nil {
		profile := m.riskEngine.GetProfile(perm.UserID, perm.DeviceID)
		if profile != nil {
			adjustedLevel := m.adjustLevelByRisk(perm.BaseLevel, profile.Level)
			if adjustedLevel != perm.BaseLevel {
				perm.CurrentLevel = adjustedLevel
				perm.Reason = fmt.Sprintf("风险等级 %s，权限从 %s 降为 %s", profile.Level, perm.BaseLevel, adjustedLevel)
			}
		}
	}

	m.permissions[key] = perm
	return nil
}

// AdjustPermission 根据风险调整权限
func (m *PermissionManager) AdjustPermission(userID, deviceID, resource string) *DynamicPermission {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.permissionKey(userID, deviceID, resource)
	perm, exists := m.permissions[key]
	if !exists {
		return nil
	}

	if m.riskEngine == nil {
		return perm
	}

	profile := m.riskEngine.GetProfile(userID, deviceID)
	if profile == nil {
		return perm
	}

	newLevel := m.adjustLevelByRisk(perm.BaseLevel, profile.Level)

	if newLevel != perm.CurrentLevel {
		perm.CurrentLevel = newLevel
		perm.UpdatedAt = time.Now()
		perm.Reason = fmt.Sprintf("动态调整: 风险等级 %s (分数: %.1f)", profile.Level, profile.CurrentScore)
	}

	return perm
}

// adjustLevelByRisk 根据风险调整级别
func (m *PermissionManager) adjustLevelByRisk(baseLevel AccessLevel, riskLevel RiskLevel) AccessLevel {
	for _, policy := range m.downgradePolicies {
		if !policy.Enabled {
			continue
		}

		if riskLevel == policy.RiskLevel {
			if baseLevel > policy.MaxLevel {
				return policy.MaxLevel
			}
		}
	}

	// 风险递增逻辑
	switch riskLevel {
	case RiskLevelCritical:
		return AccessLevelNone
	case RiskLevelHigh:
		if baseLevel > AccessLevelRead {
			return AccessLevelRead
		}
	case RiskLevelMedium:
		if baseLevel > AccessLevelWrite {
			return AccessLevelWrite
		}
	}

	return baseLevel
}

// RevokePermission 撤销权限
func (m *PermissionManager) RevokePermission(userID, deviceID, resource string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.permissionKey(userID, deviceID, resource)
	delete(m.permissions, key)
	return nil
}

// GetPermission 获取权限
func (m *PermissionManager) GetPermission(userID, deviceID, resource string) *DynamicPermission {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := m.permissionKey(userID, deviceID, resource)
	return m.permissions[key]
}

// GetUserPermissions 获取用户所有权限
func (m *PermissionManager) GetUserPermissions(userID string) []*DynamicPermission {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var perms []*DynamicPermission
	for _, p := range m.permissions {
		if p.UserID == userID {
			perms = append(perms, p)
		}
	}
	return perms
}

// permissionKey 生成权限key
func (m *PermissionManager) permissionKey(userID, deviceID, resource string) string {
	return fmt.Sprintf("%s:%s:%s", userID, deviceID, resource)
}

// ========== 风险引擎实现 ==========

// NewRiskEngine 创建风险引擎
func NewRiskEngine(thresholds RiskThresholds) *RiskEngine {
	engine := &RiskEngine{
		riskProfiles:  make(map[string]*RiskProfile),
		riskFactors:   make(map[string]RiskFactorConfig),
		thresholds:    thresholds,
		alertHandlers: make([]AlertHandler, 0),
	}

	// 初始化默认风险因素
	engine.initDefaultFactors()

	return engine
}

// initDefaultFactors 初始化默认风险因素
func (e *RiskEngine) initDefaultFactors() {
	e.riskFactors = map[string]RiskFactorConfig{
		"failed_login": {
			Name:        "登录失败",
			Weight:      0.2,
			BaseScore:   20,
			DecayRate:   0.5,
			Description: "登录失败次数",
		},
		"unusual_time": {
			Name:        "异常时间访问",
			Weight:      0.1,
			BaseScore:   10,
			DecayRate:   1.0,
			Description: "非工作时间访问",
		},
		"new_device": {
			Name:        "新设备",
			Weight:      0.15,
			BaseScore:   15,
			DecayRate:   0.3,
			Description: "新注册设备",
		},
		"location_change": {
			Name:        "位置变化",
			Weight:      0.15,
			BaseScore:   25,
			DecayRate:   0.4,
			Description: "地理位置快速变化",
		},
		"privilege_escalation": {
			Name:        "权限提升",
			Weight:      0.25,
			BaseScore:   40,
			DecayRate:   0.2,
			Description: "尝试访问高权限资源",
		},
		"anomaly_detected": {
			Name:        "行为异常",
			Weight:      0.15,
			BaseScore:   30,
			DecayRate:   0.3,
			Description: "检测到异常行为模式",
		},
	}
}

// GetProfile 获取风险档案
func (e *RiskEngine) GetProfile(userID, deviceID string) *RiskProfile {
	e.mu.RLock()
	defer e.mu.RUnlock()

	key := e.profileKey(userID, deviceID)
	return e.riskProfiles[key]
}

// UpdateRisk 更新风险分数
func (e *RiskEngine) UpdateRisk(userID, deviceID, factor string, score float64) *RiskProfile {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := e.profileKey(userID, deviceID)
	profile, exists := e.riskProfiles[key]

	if !exists {
		profile = &RiskProfile{
			UserID:   userID,
			DeviceID: deviceID,
			Factors:  make(map[string]float64),
		}
		e.riskProfiles[key] = profile
	}

	// 更新因素分数
	profile.Factors[factor] = score

	// 计算总分
	totalScore := 0.0
	totalWeight := 0.0

	for factorName, factorScore := range profile.Factors {
		config, ok := e.riskFactors[factorName]
		if ok {
			totalScore += factorScore * config.Weight
			totalWeight += config.Weight
		} else {
			totalScore += factorScore * 0.1
			totalWeight += 0.1
		}
	}

	if totalWeight > 0 {
		profile.CurrentScore = totalScore / totalWeight
	}

	// 确定风险级别
	profile.Level = e.scoreToRiskLevel(profile.CurrentScore)

	// 记录历史
	profile.History = append(profile.History, RiskScoreRecord{
		Score:     profile.CurrentScore,
		Timestamp: time.Now(),
		Trigger:   factor,
	})

	profile.LastUpdated = time.Now()

	// 检查是否需要告警
	e.checkAlerts(profile)

	return profile
}

// scoreToRiskLevel 分数转风险级别
func (e *RiskEngine) scoreToRiskLevel(score float64) RiskLevel {
	switch {
	case score >= e.thresholds.Critical:
		return RiskLevelCritical
	case score >= e.thresholds.High:
		return RiskLevelHigh
	case score >= e.thresholds.Medium:
		return RiskLevelMedium
	default:
		return RiskLevelLow
	}
}

// checkAlerts 检查告警
func (e *RiskEngine) checkAlerts(profile *RiskProfile) {
	if profile.Level >= RiskLevelHigh {
		alert := RiskAlert{
			Level:     profile.Level,
			UserID:    profile.UserID,
			DeviceID:  profile.DeviceID,
			Score:     profile.CurrentScore,
			Message:   fmt.Sprintf("用户 %s 设备 %s 风险分数达到 %.1f (级别: %s)", profile.UserID, profile.DeviceID, profile.CurrentScore, profile.Level),
			Timestamp: time.Now(),
		}

		for _, handler := range e.alertHandlers {
			handler(alert)
		}
	}
}

// AddAlertHandler 添加告警处理器
func (e *RiskEngine) AddAlertHandler(handler AlertHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.alertHandlers = append(e.alertHandlers, handler)
}

// profileKey 生成档案key
func (e *RiskEngine) profileKey(userID, deviceID string) string {
	return fmt.Sprintf("%s:%s", userID, deviceID)
}

// GetRiskFactors 获取风险因素配置
func (e *RiskEngine) GetRiskFactors() map[string]RiskFactorConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()

	factors := make(map[string]RiskFactorConfig)
	for k, v := range e.riskFactors {
		factors[k] = v
	}
	return factors
}

// CalculateTimeRisk 计算时间风险
func CalculateTimeRisk(t time.Time) float64 {
	hour := t.Hour()
	// 非工作时间（22:00 - 06:00）
	if hour >= 22 || hour < 6 {
		return 30.0
	}
	// 早高峰（6:00 - 8:00）
	if hour >= 6 && hour < 8 {
		return 10.0
	}
	return 0.0
}

// CalculateLocationRisk 计算位置风险
func CalculateLocationRisk(lastLocation, currentLocation string, timeDiff time.Duration) float64 {
	if lastLocation == "" || currentLocation == "" {
		return 0.0
	}

	if lastLocation != currentLocation {
		// 短时间内位置变化大
		if timeDiff < 2*time.Hour {
			return 50.0
		}
		return 20.0
	}

	return 0.0
}

// DecayRiskScore 衰减风险分数
func DecayRiskScore(score float64, decayRate float64, elapsed time.Duration) float64 {
	hours := elapsed.Hours()
	decay := score * (1 - decayRate*hours/24)
	if decay < 0 {
		return 0
	}
	return decay
}
