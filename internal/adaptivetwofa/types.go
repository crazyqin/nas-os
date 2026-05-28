// Package adaptivetwofa 实现自适应双因素认证模块
// 根据登录风险评分动态调整认证要求，平衡安全性和用户体验
package adaptivetwofa

import (
	"sync"
	"time"
)

// RiskLevel 风险等级
type RiskLevel int

const (
	// RiskLow 低风险 - 信任设备，可跳过2FA
	RiskLow RiskLevel = iota
	// RiskMedium 中风险 - 需要简单验证
	RiskMedium
	// RiskHigh 高风险 - 需要完整2FA
	RiskHigh
	// RiskCritical 极高风险 - 需要额外验证或阻止
	RiskCritical
)

// String 返回风险等级的字符串表示
func (r RiskLevel) String() string {
	switch r {
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	case RiskCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// RiskScore 风险评分
type RiskScore struct {
	// Score 总分 (0-100, 越高越危险)
	Score int `json:"score"`
	// Level 风险等级
	Level RiskLevel `json:"level"`
	// Factors 风险因子详情
	Factors []RiskFactor `json:"factors"`
	// EvaluatedAt 评估时间
	EvaluatedAt time.Time `json:"evaluated_at"`
}

// RiskFactor 风险因子
type RiskFactor struct {
	// Name 因子名称
	Name string `json:"name"`
	// Description 描述
	Description string `json:"description"`
	// Score 该因子的得分
	Score int `json:"score"`
	// Weight 权重 (0-1)
	Weight float64 `json:"weight"`
}

// LoginContext 登录上下文
type LoginContext struct {
	// UserID 用户ID
	UserID string `json:"user_id"`
	// Username 用户名
	Username string `json:"username"`
	// IP 登录IP
	IP string `json:"ip"`
	// UserAgent 用户代理
	UserAgent string `json:"user_agent"`
	// DeviceFingerprint 设备指纹
	DeviceFingerprint string `json:"device_fingerprint"`
	// Timestamp 登录时间
	Timestamp time.Time `json:"timestamp"`
	// GeoLocation 地理位置 (可选，由外部服务提供)
	GeoLocation *GeoLocation `json:"geo_location,omitempty"`
}

// GeoLocation 地理位置信息
type GeoLocation struct {
	// Country 国家
	Country string `json:"country"`
	// Region 地区
	Region string `json:"region"`
	// City 城市
	City string `json:"city"`
	// Latitude 纬度
	Latitude float64 `json:"latitude"`
	// Longitude 经度
	Longitude float64 `json:"longitude"`
}

// TrustedDevice 信任设备
type TrustedDevice struct {
	// DeviceID 设备唯一标识
	DeviceID string `json:"device_id"`
	// UserID 用户ID
	UserID string `json:"user_id"`
	// Fingerprint 设备指纹
	Fingerprint string `json:"fingerprint"`
	// IP 首次登录IP
	IP string `json:"ip"`
	// UserAgent 用户代理
	UserAgent string `json:"user_agent"`
	// GeoLocation 首次登录地理位置
	GeoLocation *GeoLocation `json:"geo_location,omitempty"`
	// CreatedAt 首次信任时间
	CreatedAt time.Time `json:"created_at"`
	// LastSeenAt 最后使用时间
	LastSeenAt time.Time `json:"last_seen_at"`
	// ExpiresAt 过期时间 (默认30天)
	ExpiresAt time.Time `json:"expires_at"`
	// TrustLevel 信任级别 (1-10)
	TrustLevel int `json:"trust_level"`
}

// IsExpired 检查设备是否过期
func (td *TrustedDevice) IsExpired() bool {
	return time.Now().After(td.ExpiresAt)
}

// AuthChallenge 认证挑战
type AuthChallenge struct {
	// ChallengeID 挑战ID
	ChallengeID string `json:"challenge_id"`
	// UserID 用户ID
	UserID string `json:"user_id"`
	// Type 挑战类型 (totp, sms, email, backup_code)
	Type string `json:"type"`
	// Required 是否必须完成
	Required bool `json:"required"`
	// RiskScore 触发该挑战的风险评分
	RiskScore *RiskScore `json:"risk_score"`
	// ExpiresAt 过期时间
	ExpiresAt time.Time `json:"expires_at"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
}

// IsExpired 检查挑战是否过期
func (ac *AuthChallenge) IsExpired() bool {
	return time.Now().After(ac.ExpiresAt)
}

// AdaptiveAuthResult 自适应认证结果
type AdaptiveAuthResult struct {
	// Allowed 是否允许登录
	Allowed bool `json:"allowed"`
	// RiskScore 风险评分
	RiskScore *RiskScore `json:"risk_score"`
	// Challenges 需要完成的挑战列表
	Challenges []AuthChallenge `json:"challenges,omitempty"`
	// TrustDevice 是否需要信任设备提示
	TrustDevicePrompt bool `json:"trust_device_prompt"`
	// Message 消息
	Message string `json:"message,omitempty"`
}

// LoginHistory 登录历史
type LoginHistory struct {
	// UserID 用户ID
	UserID string `json:"user_id"`
	// IP 登录IP
	IP string `json:"ip"`
	// UserAgent 用户代理
	UserAgent string `json:"user_agent"`
	// DeviceFingerprint 设备指纹
	DeviceFingerprint string `json:"device_fingerprint"`
	// GeoLocation 地理位置
	GeoLocation *GeoLocation `json:"geo_location,omitempty"`
	// Success 是否成功
	Success bool `json:"success"`
	// RiskScore 风险评分
	RiskScore int `json:"risk_score"`
	// Timestamp 登录时间
	Timestamp time.Time `json:"timestamp"`
}

// AdaptiveConfig 自适应2FA配置
type AdaptiveConfig struct {
	// LowRiskThreshold 低风险阈值 (0-100)
	LowRiskThreshold int `json:"low_risk_threshold"`
	// MediumRiskThreshold 中风险阈值 (0-100)
	MediumRiskThreshold int `json:"medium_risk_threshold"`
	// HighRiskThreshold 高风险阈值 (0-100)
	HighRiskThreshold int `json:"high_risk_threshold"`
	// TrustedDeviceTTL 信任设备有效期 (默认30天)
	TrustedDeviceTTL time.Duration `json:"trusted_device_ttl"`
	// MaxTrustedDevices 每用户最大信任设备数
	MaxTrustedDevices int `json:"max_trusted_devices"`
	// ChallengeTTL 挑战有效期 (默认5分钟)
	ChallengeTTL time.Duration `json:"challenge_ttl"`
	// NewIPWeight 新IP风险权重
	NewIPWeight float64 `json:"new_ip_weight"`
	// NewDeviceWeight 新设备风险权重
	NewDeviceWeight float64 `json:"new_device_weight"`
	// UnusualTimeWeight 异常时间风险权重
	UnusualTimeWeight float64 `json:"unusual_time_weight"`
	// GeoChangeWeight 地理位置变化风险权重
	GeoChangeWeight float64 `json:"geo_change_weight"`
	// RapidLoginWeight 短时间多次登录风险权重
	RapidLoginWeight float64 `json:"rapid_login_weight"`
	// RapidLoginWindow 短时间登录窗口
	RapidLoginWindow time.Duration `json:"rapid_login_window"`
	// RapidLoginThreshold 短时间登录次数阈值
	RapidLoginThreshold int `json:"rapid_login_threshold"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *AdaptiveConfig {
	return &AdaptiveConfig{
		LowRiskThreshold:    25,
		MediumRiskThreshold: 50,
		HighRiskThreshold:   75,
		TrustedDeviceTTL:    30 * 24 * time.Hour, // 30天
		MaxTrustedDevices:   5,
		ChallengeTTL:        5 * time.Minute,
		NewIPWeight:         0.3,
		NewDeviceWeight:     0.35,
		UnusualTimeWeight:   0.1,
		GeoChangeWeight:     0.15,
		RapidLoginWeight:    0.1,
		RapidLoginWindow:    10 * time.Minute,
		RapidLoginThreshold: 5,
	}
}

// UserLoginStats 用户登录统计
type UserLoginStats struct {
	mu             sync.RWMutex
	UserID         string
	LastIPs        []string
	LastDevices    []string
	LastLocations  []GeoLocation
	LoginTimes     []time.Time
	NormalHours    map[int]int // 小时 -> 登录次数
	LastLoginTime  time.Time
	TotalLogins    int
	FailedAttempts int
}
