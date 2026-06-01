// Package zerotrust 提供零信任安全功能，包括设备信任评估、持续认证、最小权限策略等。
package zerotrust

import (
	"time"
)

// ==================== 信任评分相关 ====================

// TrustScore 信任评分
type TrustScore struct {
	Overall     float64           `json:"overall"`      // 0-100
	Device      float64           `json:"device"`       // 设备信任分
	Identity    float64           `json:"identity"`     // 身份信任分
	Network     float64           `json:"network"`      // 网络信任分
	Behavior    float64           `json:"behavior"`     // 行为信任分
	Compliance  float64           `json:"compliance"`   // 合规信任分
	Factors     []TrustFactor     `json:"factors"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// TrustFactor 信任因子
type TrustFactor struct {
	Name      string  `json:"name"`
	Score     float64 `json:"score"`
	Weight    float64 `json:"weight"`
	Details   string  `json:"details"`
}

// ==================== 设备信任相关 ====================

// DeviceTrust 设备信任信息
type DeviceTrust struct {
	ID              string            `json:"id"`
	DeviceID        string            `json:"device_id"`
	DeviceName      string            `json:"device_name"`
	DeviceType      string            `json:"device_type"`      // desktop, mobile, server, iot
	OS              string            `json:"os"`
	OSVersion       string            `json:"os_version"`
	TrustScore      TrustScore        `json:"trust_score"`
	Status          string            `json:"status"`           // trusted, untrusted, compromised, quarantined`
	Owner           string            `json:"owner"`
	Location        string            `json:"location"`
	IPAddress       string            `json:"ip_address"`
	MACAddress      string            `json:"mac_address"`
	Certificates    []Certificate     `json:"certificates,omitempty"`
	ComplianceState ComplianceState   `json:"compliance_state"`
	LastSeen        time.Time         `json:"last_seen"`
	RegisteredAt    time.Time         `json:"registered_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// Certificate 证书信息
type Certificate struct {
	ID        string    `json:"id"`
	Subject   string    `json:"subject"`
	Issuer    string    `json:"issuer"`
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
	Status    string    `json:"status"` // valid, expired, revoked
}

// ComplianceState 合规状态
type ComplianceState struct {
	Compliant    bool     `json:"compliant"`
	LastChecked  time.Time `json:"last_checked"`
	Issues       []string `json:"issues,omitempty"`
	Antivirus    string   `json:"antivirus"`
	Firewall     string   `json:"firewall"`
	Encryption   string   `json:"encryption"`
}

// DeviceFilter 设备过滤器
type DeviceFilter struct {
	Status    string `json:"status,omitempty"`
	DeviceType string `json:"device_type,omitempty"`
	Owner     string `json:"owner,omitempty"`
	MinScore  *float64 `json:"min_score,omitempty"`
}

// ==================== 访问策略相关 ====================

// AccessPolicy 访问策略
type AccessPolicy struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Priority    int             `json:"priority"`
	Enabled     bool            `json:"enabled"`
	Subject     PolicySubject   `json:"subject"`
	Resource    PolicyResource  `json:"resource"`
	Action      string          `json:"action"`           // allow, deny, require-mfa
	Conditions  []Condition     `json:"conditions"`
	Constraints []Constraint    `json:"constraints"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	CreatedBy   string          `json:"created_by"`
}

// PolicySubject 策略主体
type PolicySubject struct {
	Type       string   `json:"type"`       // user, group, device, service
	IDs        []string `json:"ids"`
	Roles      []string `json:"roles,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// PolicyResource 策略资源
type PolicyResource struct {
	Type       string   `json:"type"`       // file, api, database, service
	IDs        []string `json:"ids"`
	Paths      []string `json:"paths,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// Condition 策略条件
type Condition struct {
	Type     string `json:"type"`     // time, location, network, device-trust
	Operator string `json:"operator"` // equals, not-equals, in, not-in, greater-than, less-than
	Value    string `json:"value"`
}

// Constraint 策略约束
type Constraint struct {
	Type  string `json:"type"`  // max-session, mfa-required, encryption-required
	Value string `json:"value"`
}

// ==================== 认证会话相关 ====================

// AuthSession 认证会话
type AuthSession struct {
	ID              string            `json:"id"`
	UserID          string            `json:"user_id"`
	DeviceID        string            `json:"device_id"`
	StartTime       time.Time         `json:"start_time"`
	LastActivity    time.Time         `json:"last_activity"`
	ExpiresAt       time.Time         `json:"expires_at"`
	Status          string            `json:"status"`         // active, expired, revoked
	TrustScore      float64           `json:"trust_score"`
	AuthFactors     []string          `json:"auth_factors"`   // password, mfa, biometric, certificate
	Location        string            `json:"location"`
	IPAddress       string            `json:"ip_address"`
	UserAgent       string            `json:"user_agent"`
	Activities      []SessionActivity `json:"activities,omitempty"`
	RiskEvents      []RiskEvent       `json:"risk_events,omitempty"`
}

// SessionActivity 会话活动
type SessionActivity struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Result    string    `json:"result"`
}

// RiskEvent 风险事件
type RiskEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	RiskScore   float64   `json:"risk_score"`
}

// SessionFilter 会话过滤器
type SessionFilter struct {
	UserID   string `json:"user_id,omitempty"`
	DeviceID string `json:"device_id,omitempty"`
	Status   string `json:"status,omitempty"`
}

// ==================== 威胁事件相关 ====================

// ThreatEvent 威胁事件
type ThreatEvent struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`        // unauthorized-access, anomaly, malware, data-leak
	Severity    string    `json:"severity"`    // critical, high, medium, low
	Source      string    `json:"source"`
	Target      string    `json:"target"`
	Description string    `json:"description"`
	Indicators  []string  `json:"indicators"`
	Status      string    `json:"status"`      // detected, investigating, mitigated, resolved
	Actions     []string  `json:"actions"`     // 已采取的措施
	AssignedTo  string    `json:"assigned_to,omitempty"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	Notes       string    `json:"notes,omitempty"`
}

// ThreatFilter 威胁过滤器
type ThreatFilter struct {
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Types     []string   `json:"types,omitempty"`
	Severities []string  `json:"severities,omitempty"`
	Statuses  []string   `json:"statuses,omitempty"`
}

// ==================== 统计相关 ====================

// TrustStats 信任统计
type TrustStats struct {
	TotalDevices     int     `json:"total_devices"`
	TrustedDevices   int     `json:"trusted_devices"`
	UntrustedDevices int     `json:"untrusted_devices"`
	CompromisedDevices int   `json:"compromised_devices"`
	AverageScore     float64 `json:"average_score"`
	ActiveSessions   int     `json:"active_sessions"`
	ThreatsDetected  int     `json:"threats_detected"`
	ThreatsMitigated int     `json:"threats_mitigated"`
}
