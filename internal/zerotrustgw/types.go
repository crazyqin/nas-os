// Package zerotrustgw 提供零信任网关功能，实现最小权限访问控制和持续身份验证。
// 基于零信任安全模型，对每次访问请求进行动态信任评估和策略执行。
package zerotrustgw

import "time"

// TrustLevel 信任等级
type TrustLevel string

const (
	TrustLevelHigh   TrustLevel = "high"
	TrustLevelMedium TrustLevel = "medium"
	TrustLevelLow    TrustLevel = "low"
	TrustLevelNone   TrustLevel = "none"
)

// AccessDecision 访问决策
type AccessDecision string

const (
	DecisionAllow AccessDecision = "allow"
	DecisionDeny  AccessDecision = "deny"
	DecisionMFA   AccessDecision = "require_mfa"
	DecisionStepUp AccessDecision = "step_up_auth"
)

// PolicyAction 策略动作
type PolicyAction string

const (
	ActionAllow    PolicyAction = "allow"
	ActionDeny     PolicyAction = "deny"
	ActionAudit    PolicyAction = "audit"
	ActionAlert    PolicyAction = "alert"
	ActionQuarantine PolicyAction = "quarantine"
)

// IdentityProvider 身份提供商
type IdentityProvider string

const (
	ProviderLocal  IdentityProvider = "local"
	ProviderLDAP   IdentityProvider = "ldap"
	ProviderOAuth  IdentityProvider = "oauth"
	ProviderSAML   IdentityProvider = "saml"
	ProviderOIDC   IdentityProvider = "oidc"
)

// TrustPolicy 信任策略
type TrustPolicy struct {
	ID          string         `json:"id"`
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description,omitempty"`
	Priority    int            `json:"priority"`
	Enabled     bool           `json:"enabled"`
	Conditions  []Condition    `json:"conditions" binding:"required,min=1"`
	Action      PolicyAction   `json:"action" binding:"required"`
	MinTrust    TrustLevel     `json:"min_trust"`
	Resources   []string       `json:"resources,omitempty"`
	Users       []string       `json:"users,omitempty"`
	Groups      []string       `json:"groups,omitempty"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// Condition 策略条件
type Condition struct {
	Type     string   `json:"type" binding:"required"`
	Operator string   `json:"operator" binding:"required"`
	Values   []string `json:"values" binding:"required,min=1"`
}

// AccessRequest 访问请求
type AccessRequest struct {
	ID          string            `json:"id"`
	UserID      string            `json:"user_id" binding:"required"`
	Resource    string            `json:"resource" binding:"required"`
	Action      string            `json:"action" binding:"required"`
	SourceIP    string            `json:"source_ip,omitempty"`
	UserAgent   string            `json:"user_agent,omitempty"`
	DeviceID    string            `json:"device_id,omitempty"`
	DeviceName  string            `json:"device_name,omitempty"`
	Location    *GeoLocation      `json:"location,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	SessionID   string            `json:"session_id,omitempty"`
}

// GeoLocation 地理位置
type GeoLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Country   string  `json:"country,omitempty"`
	City      string  `json:"city,omitempty"`
	Region    string  `json:"region,omitempty"`
}

// VerificationResult 验证结果
type VerificationResult struct {
	ID             string          `json:"id"`
	RequestID      string          `json:"request_id"`
	Decision       AccessDecision  `json:"decision"`
	TrustScore     float64         `json:"trust_score"`
	TrustLevel     TrustLevel      `json:"trust_level"`
	MatchedPolicy  string          `json:"matched_policy,omitempty"`
	Reasons        []string        `json:"reasons,omitempty"`
	RequiresMFA    bool            `json:"requires_mfa"`
	AllowedActions []string        `json:"allowed_actions,omitempty"`
	ExpiresAt      time.Time       `json:"expires_at"`
	Timestamp      time.Time       `json:"timestamp"`
	ProcessingTime time.Duration   `json:"processing_time"`
}

// TrustScore 信任分数
type TrustScore struct {
	UserID        string             `json:"user_id"`
	Overall       float64            `json:"overall"`
	DeviceScore   float64            `json:"device_score"`
	NetworkScore  float64            `json:"network_score"`
	BehaviorScore float64            `json:"behavior_score"`
	LocationScore float64            `json:"location_score"`
	TimeScore     float64            `json:"time_score"`
	Level         TrustLevel         `json:"level"`
	Factors       []ScoreFactor      `json:"factors,omitempty"`
	LastUpdated   time.Time          `json:"last_updated"`
	History       []TrustScoreRecord `json:"history,omitempty"`
}

// ScoreFactor 分数因子
type ScoreFactor struct {
	Name    string  `json:"name"`
	Score   float64 `json:"score"`
	Weight  float64 `json:"weight"`
	Detail  string  `json:"detail,omitempty"`
}

// TrustScoreRecord 信任分数历史记录
type TrustScoreRecord struct {
	Score     float64   `json:"score"`
	Level     TrustLevel `json:"level"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason,omitempty"`
}

// DeviceProfile 设备画像
type DeviceProfile struct {
	DeviceID     string    `json:"device_id"`
	DeviceName   string    `json:"device_name"`
	DeviceType   string    `json:"device_type"`
	OS           string    `json:"os,omitempty"`
	OSVersion    string    `json:"os_version,omitempty"`
	Browser      string    `json:"browser,omitempty"`
	IsManaged    bool      `json:"is_managed"`
	IsCompliant  bool      `json:"is_compliant"`
	LastSeen     time.Time `json:"last_seen"`
	TrustScore   float64   `json:"trust_score"`
	Fingerprint  string    `json:"fingerprint,omitempty"`
	CertExpiry   *time.Time `json:"cert_expiry,omitempty"`
}

// NetworkContext 网络上下文
type NetworkContext struct {
	SourceIP    string `json:"source_ip"`
	IsVPN       bool   `json:"is_vpn"`
	IsTor       bool   `json:"is_tor"`
	IsProxy     bool   `json:"is_proxy"`
	ASN         int    `json:"asn,omitempty"`
	ISP         string `json:"isp,omitempty"`
	ThreatLevel string `json:"threat_level,omitempty"`
	GeoLocation *GeoLocation `json:"geo_location,omitempty"`
}

// AuditEntry 审计日志条目
type AuditEntry struct {
	ID          string          `json:"id"`
	Timestamp   time.Time       `json:"timestamp"`
	UserID      string          `json:"user_id"`
	Resource    string          `json:"resource"`
	Action      string          `json:"action"`
	Decision    AccessDecision  `json:"decision"`
	TrustScore  float64         `json:"trust_score"`
	SourceIP    string          `json:"source_ip"`
	DeviceID    string          `json:"device_id,omitempty"`
	PolicyID    string          `json:"policy_id,omitempty"`
	Reasons     []string        `json:"reasons,omitempty"`
	RiskLevel   string          `json:"risk_level,omitempty"`
	SessionID   string          `json:"session_id,omitempty"`
}

// SessionInfo 会话信息
type SessionInfo struct {
	SessionID    string    `json:"session_id"`
	UserID       string    `json:"user_id"`
	DeviceID     string    `json:"device_id"`
	SourceIP     string    `json:"source_ip"`
	StartedAt    time.Time `json:"started_at"`
	LastActivity time.Time `json:"last_activity"`
	TrustScore   float64   `json:"trust_score"`
	IsActive     bool      `json:"is_active"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// ZeroTrustConfig 零信任网关配置
type ZeroTrustConfig struct {
	Enabled              bool    `json:"enabled"`
	DefaultTrustLevel    TrustLevel `json:"default_trust_level"`
	MinTrustScore        float64 `json:"min_trust_score"`
	MFAEnabled           bool    `json:"mfa_enabled"`
	ContinuousAuth       bool    `json:"continuous_auth"`
	ContinuousAuthInterval int   `json:"continuous_auth_interval_minutes"`
	SessionTimeout       int     `json:"session_timeout_minutes"`
	MaxFailedAttempts    int     `json:"max_failed_attempts"`
	LockoutDuration      int     `json:"lockout_duration_minutes"`
	AuditEnabled         bool    `json:"audit_enabled"`
	AlertOnLowTrust      bool    `json:"alert_on_low_trust"`
	GeoFencingEnabled    bool    `json:"geo_fencing_enabled"`
	AllowedCountries     []string `json:"allowed_countries,omitempty"`
	BlockedCountries     []string `json:"blocked_countries,omitempty"`
	DeviceTrustRequired  bool    `json:"device_trust_required"`
}

// DefaultZeroTrustConfig 默认配置
func DefaultZeroTrustConfig() *ZeroTrustConfig {
	return &ZeroTrustConfig{
		Enabled:              true,
		DefaultTrustLevel:    TrustLevelMedium,
		MinTrustScore:        0.5,
		MFAEnabled:           true,
		ContinuousAuth:       true,
		ContinuousAuthInterval: 30,
		SessionTimeout:       480,
		MaxFailedAttempts:    5,
		LockoutDuration:      30,
		AuditEnabled:         true,
		AlertOnLowTrust:      true,
		GeoFencingEnabled:    false,
		DeviceTrustRequired:  true,
	}
}
