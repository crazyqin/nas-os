package zerotrustnac

import "time"

// PolicyType 策略类型
type PolicyType string

const (
	PolicyAllow      PolicyType = "allow"
	PolicyDeny       PolicyType = "deny"
	PolicyQuarantine PolicyType = "quarantine"
	PolicyMFA        PolicyType = "mfa"
	PolicyLimit      PolicyType = "limit"
)

// DeviceType 设备类型
type DeviceType string

const (
	DeviceDesktop DeviceType = "desktop"
	DeviceLaptop  DeviceType = "laptop"
	DeviceMobile  DeviceType = "mobile"
	DeviceTablet  DeviceType = "tablet"
	DeviceServer  DeviceType = "server"
	DeviceIoT     DeviceType = "iot"
	DevicePrinter DeviceType = "printer"
	DeviceNetwork DeviceType = "network"
	DeviceOther   DeviceType = "other"
)

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceTrusted     DeviceStatus = "trusted"
	DeviceUntrusted   DeviceStatus = "untrusted"
	DeviceQuarantined DeviceStatus = "quarantined"
	DeviceBlocked     DeviceStatus = "blocked"
	DevicePending     DeviceStatus = "pending"
)

// ComplianceStatus 合规状态
type ComplianceStatus string

const (
	ComplianceCompliant    ComplianceStatus = "compliant"
	ComplianceNonCompliant ComplianceStatus = "non_compliant"
	ComplianceUnknown      ComplianceStatus = "unknown"
	ComplianceRemediating  ComplianceStatus = "remediating"
)

// AuthMethod 认证方式
type AuthMethod string

const (
	AuthPassword    AuthMethod = "password"
	AuthMFA         AuthMethod = "mfa"
	AuthCertificate AuthMethod = "certificate"
	AuthBiometric   AuthMethod = "biometric"
	AuthToken       AuthMethod = "token"
	AuthSSO         AuthMethod = "sso"
)

// Device 设备
type Device struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Type             DeviceType        `json:"type"`
	Status           DeviceStatus      `json:"status"`
	Compliance       ComplianceStatus  `json:"compliance"`
	OwnerID          string            `json:"owner_id"`
	OwnerName        string            `json:"owner_name"`
	Organization     string            `json:"organization"`
	Department       string            `json:"department"`
	IPAddress        string            `json:"ip_address"`
	MACAddress       string            `json:"mac_address"`
	Hostname         string            `json:"hostname"`
	OS               string            `json:"os"`
	OSVersion        string            `json:"os_version"`
	Firmware         string            `json:"firmware"`
	Antivirus        string            `json:"antivirus,omitempty"`
	AntivirusUpdated bool              `json:"antivirus_updated"`
	FirewallEnabled  bool              `json:"firewall_enabled"`
	DiskEncrypted    bool              `json:"disk_encrypted"`
	LastPatchDate    *time.Time        `json:"last_patch_date,omitempty"`
	Tags             []string          `json:"tags"`
	Location         string            `json:"location,omitempty"`
	TrustScore       float64           `json:"trust_score"`
	RiskScore        float64           `json:"risk_score"`
	LastSeen         time.Time         `json:"last_seen"`
	RegisteredAt     time.Time         `json:"registered_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
	Certificates     []Certificate     `json:"certificates,omitempty"`
	ComplianceChecks []ComplianceCheck `json:"compliance_checks,omitempty"`
}

// Certificate 证书
type Certificate struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Issuer      string    `json:"issuer"`
	Subject     string    `json:"subject"`
	NotBefore   time.Time `json:"not_before"`
	NotAfter    time.Time `json:"not_after"`
	Fingerprint string    `json:"fingerprint"`
	Status      string    `json:"status"`
}

// ComplianceCheck 合规检查
type ComplianceCheck struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Category    string           `json:"category"`
	Status      ComplianceStatus `json:"status"`
	Required    bool             `json:"required"`
	Message     string           `json:"message,omitempty"`
	LastChecked time.Time        `json:"last_checked"`
	Remediation string           `json:"remediation,omitempty"`
}

// AccessPolicy 访问策略
type AccessPolicy struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Type        PolicyType  `json:"type"`
	Priority    int         `json:"priority"`
	Enabled     bool        `json:"enabled"`
	Conditions  []Condition `json:"conditions"`
	Actions     []Action    `json:"actions"`
	Resources   []string    `json:"resources"`
	Schedule    *Schedule   `json:"schedule,omitempty"`
	MaxDuration int         `json:"max_duration,omitempty"`
	Logging     bool        `json:"logging"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	CreatedBy   string      `json:"created_by"`
}

// Condition 策略条件
type Condition struct {
	Type     string      `json:"type"`
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
	Logic    string      `json:"logic,omitempty"`
}

// Action 策略动作
type Action struct {
	Type       string                 `json:"type"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// Schedule 时间计划
type Schedule struct {
	Type      string   `json:"type"`
	Days      []string `json:"days"`
	StartTime string   `json:"start_time"`
	EndTime   string   `json:"end_time"`
	TimeZone  string   `json:"time_zone"`
}

// AccessSession 访问会话
type AccessSession struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	UserName     string     `json:"user_name"`
	DeviceID     string     `json:"device_id"`
	DeviceName   string     `json:"device_name"`
	ResourceID   string     `json:"resource_id"`
	ResourceName string     `json:"resource_name"`
	IPAddress    string     `json:"ip_address"`
	Location     string     `json:"location"`
	AuthMethod   AuthMethod `json:"auth_method"`
	TrustScore   float64    `json:"trust_score"`
	StartedAt    time.Time  `json:"started_at"`
	LastActivity time.Time  `json:"last_activity"`
	ExpiresAt    time.Time  `json:"expires_at"`
	Status       string     `json:"status"`
	Activities   []Activity `json:"activities,omitempty"`
}

// Activity 活动记录
type Activity struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Resource  string    `json:"resource"`
	Action    string    `json:"action"`
	Result    string    `json:"result"`
	Details   string    `json:"details,omitempty"`
}

// SecurityEvent 安全事件
type SecurityEvent struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Severity    string                 `json:"severity"`
	Source      string                 `json:"source"`
	DeviceID    string                 `json:"device_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	Description string                 `json:"description"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Resolved    bool                   `json:"resolved"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
	ResolvedBy  string                 `json:"resolved_by,omitempty"`
}

// ThreatIntelligence 威胁情报
type ThreatIntelligence struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Value       string     `json:"value"`
	Severity    string     `json:"severity"`
	Source      string     `json:"source"`
	Description string     `json:"description"`
	Tags        []string   `json:"tags"`
	FirstSeen   time.Time  `json:"first_seen"`
	LastSeen    time.Time  `json:"last_seen"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Active      bool       `json:"active"`
}

// NetworkSegment 网络分段
type NetworkSegment struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CIDR        string    `json:"cidr"`
	VLAN        int       `json:"vlan,omitempty"`
	Isolation   string    `json:"isolation"`
	Devices     []string  `json:"devices"`
	Policies    []string  `json:"policies"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ZeroTrustStats 零信任统计
type ZeroTrustStats struct {
	TotalDevices        int     `json:"total_devices"`
	TrustedDevices      int     `json:"trusted_devices"`
	QuarantinedDevices  int     `json:"quarantined_devices"`
	BlockedDevices      int     `json:"blocked_devices"`
	TotalPolicies       int     `json:"total_policies"`
	ActivePolicies      int     `json:"active_policies"`
	ActiveSessions      int     `json:"active_sessions"`
	SecurityEvents      int     `json:"security_events"`
	UnresolvedEvents    int     `json:"unresolved_events"`
	AvgTrustScore       float64 `json:"avg_trust_score"`
	CompliantDevices    int     `json:"compliant_devices"`
	NonCompliantDevices int     `json:"non_compliant_devices"`
	TotalSegments       int     `json:"total_segments"`
	TotalThreats        int     `json:"total_threats"`
	ActiveThreats       int     `json:"active_threats"`
}

// RiskAssessment 风险评估
type RiskAssessment struct {
	DeviceID        string       `json:"device_id"`
	OverallRisk     float64      `json:"overall_risk"`
	Factors         []RiskFactor `json:"factors"`
	Recommendations []string     `json:"recommendations"`
	AssessedAt      time.Time    `json:"assessed_at"`
	NextAssessment  time.Time    `json:"next_assessment"`
}

// RiskFactor 风险因素
type RiskFactor struct {
	Name    string  `json:"name"`
	Weight  float64 `json:"weight"`
	Score   float64 `json:"score"`
	Details string  `json:"details"`
}

// RemediationTask 修复任务
type RemediationTask struct {
	ID          string     `json:"id"`
	DeviceID    string     `json:"device_id"`
	CheckID     string     `json:"check_id"`
	Status      string     `json:"status"`
	Description string     `json:"description"`
	Steps       []string   `json:"steps"`
	AssignedTo  string     `json:"assigned_to,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Notes       string     `json:"notes,omitempty"`
}

// AuthPolicy 认证策略
type AuthPolicy struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	Methods         []AuthMethod   `json:"methods"`
	RequireMFA      bool           `json:"require_mfa"`
	MaxSessionTime  int            `json:"max_session_time"`
	LockoutAttempts int            `json:"lockout_attempts"`
	LockoutDuration int            `json:"lockout_duration"`
	PasswordPolicy  PasswordPolicy `json:"password_policy"`
	Enabled         bool           `json:"enabled"`
	CreatedAt       time.Time      `json:"created_at"`
}

// PasswordPolicy 密码策略
type PasswordPolicy struct {
	MinLength       int  `json:"min_length"`
	RequireUpper    bool `json:"require_upper"`
	RequireLower    bool `json:"require_lower"`
	RequireDigit    bool `json:"require_digit"`
	RequireSpecial  bool `json:"require_special"`
	MaxAge          int  `json:"max_age"`
	NoReuse         int  `json:"no_reuse"`
	ComplexityScore int  `json:"complexity_score"`
}
