// Package zerotrust 提供零信任网络架构实现
// 支持最小权限、持续验证、微隔离等核心安全能力
package zerotrust

import (
	"sync"
	"time"
)

// TrustLevel 信任等级.
type TrustLevel string

const (
	TrustLevelNone     TrustLevel = "none"     // 无信任
	TrustLevelLow      TrustLevel = "low"      // 低信任
	TrustLevelMedium   TrustLevel = "medium"   // 中等信任
	TrustLevelHigh     TrustLevel = "high"     // 高信任
	TrustLevelCritical TrustLevel = "critical" // 关键信任
)

// PolicyAction 策略动作.
type PolicyAction string

const (
	ActionAllow  PolicyAction = "allow"  // 允许
	ActionDeny   PolicyAction = "deny"   // 拒绝
	ActionAudit  PolicyAction = "audit"  // 审计记录
	ActionMFA    PolicyAction = "mfa"    // 需要多因素认证
	ActionAlert  PolicyAction = "alert"  // 告警
)

// SegmentType 网络分段类型.
type SegmentType string

const (
	SegmentTypeVLAN    SegmentType = "vlan"    // VLAN 分段
	SegmentTypeSubnet  SegmentType = "subnet"  // 子网分段
	SegmentTypeWireGuard SegmentType = "wireguard" // WireGuard VPN 分段
	SegmentTypeContainer SegmentType = "container" // 容器分段
)

// AccessStatus 访问状态.
type AccessStatus string

const (
	StatusPending  AccessStatus = "pending"  // 待审批
	StatusApproved AccessStatus = "approved" // 已批准
	StatusDenied   AccessStatus = "denied"   // 已拒绝
	StatusExpired  AccessStatus = "expired"  // 已过期
	StatusRevoked  AccessStatus = "revoked"  // 已撤销
)

// VerificationMethod 验证方式.
type VerificationMethod string

const (
	VerifyCertificate VerificationMethod = "certificate" // 证书验证
	VerifyMFA         VerificationMethod = "mfa"         // 多因素认证
	VerifyBiometric   VerificationMethod = "biometric"   // 生物识别
	VerifyToken       VerificationMethod = "token"       // 令牌验证
	VerifyDevice      VerificationMethod = "device"      // 设备验证
)

// TrustPolicy 信任策略.
type TrustPolicy struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Enabled     bool         `json:"enabled"`
	Priority    int          `json:"priority"`    // 优先级，数字越小优先级越高

	// 策略条件
	SourceSegments   []string `json:"sourceSegments"`   // 源网络分段
	DestSegments     []string `json:"destSegments"`     // 目标网络分段
	SourceIdentities []string `json:"sourceIdentities"` // 源身份
	DestIdentities   []string `json:"destIdentities"`   // 目标身份
	AllowedPorts     []int    `json:"allowedPorts"`     // 允许的端口
	AllowedProtocols []string `json:"allowedProtocols"` // 允许的协议

	// 策略动作
	Action           PolicyAction       `json:"action"`
	RequiredTrust    TrustLevel         `json:"requiredTrust"`
	RequiredVerifies []VerificationMethod `json:"requiredVerifies"` // 需要的验证方式

	// 时间限制
	ValidFrom    *time.Time `json:"validFrom,omitempty"`
	ValidUntil   *time.Time `json:"validUntil,omitempty"`
	ScheduleCron string     `json:"scheduleCron,omitempty"` // 生效时间表

	// 元数据
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	CreatedBy string    `json:"createdBy"`
}

// NetworkSegment 网络分段.
type NetworkSegment struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Type        SegmentType `json:"type"`
	Enabled     bool        `json:"enabled"`

	// 网络配置
	Subnet     string   `json:"subnet,omitempty"`     // CIDR 格式子网
	VLANID     int      `json:"vlanId,omitempty"`     // VLAN ID
	IPRange    string   `json:"ipRange,omitempty"`    // IP 范围
	AllowedIPs []string `json:"allowedIPs,omitempty"` // 允许的 IP 列表

	// WireGuard 配置
	WGPublicKey  string `json:"wgPublicKey,omitempty"`  // WireGuard 公钥
	WGEndpoint   string `json:"wgEndpoint,omitempty"`   // WireGuard 端点
	WGAllowedIPs string `json:"wgAllowedIPs,omitempty"` // WireGuard 允许的 IP

	// 安全属性
	TrustLevel  TrustLevel `json:"trustLevel"`
	IsEncrypted bool       `json:"isEncrypted"` // 是否加密传输
	IsIsolated  bool       `json:"isIsolated"`  // 是否隔离

	// 元数据
	Labels    map[string]string `json:"labels,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

// AccessRule 访问规则.
type AccessRule struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Enabled     bool         `json:"enabled"`

	// 访问主体
	SubjectType  string `json:"subjectType"`  // user, device, service, group
	SubjectID    string `json:"subjectId"`     // 主体 ID
	SubjectGroup string `json:"subjectGroup,omitempty"` // 主体组

	// 访问客体
	ResourceType string `json:"resourceType"` // segment, service, endpoint
	ResourceID   string `json:"resourceId"`   // 资源 ID

	// 访问权限
	AllowedActions []string `json:"allowedActions"` // 允许的操作
	MaxDuration    int      `json:"maxDuration"`    // 最大访问时长（秒）
	MaxConnections int      `json:"maxConnections"` // 最大并发连接数

	// 验证要求
	RequiredVerifies []VerificationMethod `json:"requiredVerifies"`
	RequireMFA       bool                `json:"requireMFA"`
	RequireDeviceAuth bool               `json:"requireDeviceAuth"`

	// 状态
	Status    AccessStatus `json:"status"`
	ExpiresAt *time.Time   `json:"expiresAt,omitempty"`

	// 元数据
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	CreatedBy string    `json:"createdBy"`
}

// Identity 身份标识.
type Identity struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"` // user, device, service
	Enabled     bool   `json:"enabled"`

	// 认证信息
	CertificateFingerprint string `json:"certificateFingerprint,omitempty"`
	PublicKey              string `json:"publicKey,omitempty"`
	MFAEnabled             bool   `json:"mfaEnabled"`

	// 设备信息
	DeviceID     string `json:"deviceId,omitempty"`
	DeviceType   string `json:"deviceType,omitempty"`
	OSVersion    string `json:"osVersion,omitempty"`
	IsCompliant  bool   `json:"isCompliant"` // 是否合规

	// 信任属性
	TrustLevel   TrustLevel `json:"trustLevel"`
	TrustScore   float64    `json:"trustScore"`   // 信任评分 0-100
	LastVerified time.Time  `json:"lastVerified"` // 最后验证时间

	// 元数据
	Labels    map[string]string `json:"labels,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

// AccessSession 访问会话.
type AccessSession struct {
	ID         string       `json:"id"`
	RuleID     string       `json:"ruleId"`
	SubjectID  string       `json:"subjectId"`
	ResourceID string       `json:"resourceId"`

	// 会话信息
	Status     AccessStatus `json:"status"`
	StartTime  time.Time    `json:"startTime"`
	EndTime    *time.Time   `json:"endTime,omitempty"`
	ExpiresAt  time.Time    `json:"expiresAt"`

	// 验证信息
	VerifiedBy    []VerificationMethod `json:"verifiedBy"`
	TrustScore    float64              `json:"trustScore"`
	DeviceID      string               `json:"deviceId,omitempty"`
	SourceIP      string               `json:"sourceIP"`

	// 流量统计
	BytesSent     int64 `json:"bytesSent"`
	BytesReceived int64 `json:"bytesReceived"`
	Requests      int64 `json:"requests"`
}

// AccessAuditEntry 访问审计条目.
type AccessAuditEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`

	// 事件信息
	EventType string `json:"eventType"` // access_request, access_granted, access_denied, access_revoked, policy_violation
	Severity  string `json:"severity"`  // info, warning, error, critical

	// 主体信息
	SubjectID   string `json:"subjectId"`
	SubjectType string `json:"subjectType"`
	SourceIP    string `json:"sourceIP"`
	UserAgent   string `json:"userAgent,omitempty"`
	DeviceID    string `json:"deviceId,omitempty"`

	// 资源信息
	ResourceID   string `json:"resourceId"`
	ResourceType string `json:"resourceType"`
	DestIP       string `json:"destIP,omitempty"`
	DestPort     int    `json:"destPort,omitempty"`

	// 策略信息
	PolicyID   string       `json:"policyId,omitempty"`
	RuleID     string       `json:"ruleId,omitempty"`
	Action     PolicyAction `json:"action"`
	TrustLevel TrustLevel   `json:"trustLevel"`

	// 结果
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`

	// 元数据
	RequestHeaders map[string]string `json:"requestHeaders,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
}

// ZeroTrustStats 零信任统计.
type ZeroTrustStats struct {
	mu sync.RWMutex `json:"-"`

	// 策略统计
	TotalPolicies    int `json:"totalPolicies"`
	ActivePolicies   int `json:"activePolicies"`
	DisabledPolicies int `json:"disabledPolicies"`

	// 分段统计
	TotalSegments    int `json:"totalSegments"`
	ActiveSegments   int `json:"activeSegments"`
	IsolatedSegments int `json:"isolatedSegments"`

	// 访问统计
	TotalAccessRules int `json:"totalAccessRules"`
	ActiveSessions   int `json:"activeSessions"`
	TotalRequests    int64 `json:"totalRequests"`
	AllowedRequests  int64 `json:"allowedRequests"`
	DeniedRequests   int64 `json:"deniedRequests"`

	// 身份统计
	TotalIdentities int `json:"totalIdentities"`
	VerifiedDevices int `json:"verifiedDevices"`
	MFAEnabledUsers int `json:"mfaEnabledUsers"`

	// 审计统计
	TotalAuditEntries  int64 `json:"totalAuditEntries"`
	PolicyViolations   int64 `json:"policyViolations"`
	SecurityAlerts     int64 `json:"securityAlerts"`

	// WireGuard 统计
	WGTunnels     int   `json:"wgTunnels"`
	WGPeers       int   `json:"wgPeers"`
	WGActivePeers int   `json:"wgActivePeers"`
	WGBytesSent   int64 `json:"wgBytesSent"`
	WGBytesRecv   int64 `json:"wgBytesRecv"`

	// 时间统计
	LastPolicyUpdate time.Time `json:"lastPolicyUpdate"`
	LastAuditCheck   time.Time `json:"lastAuditCheck"`
	Uptime           int64     `json:"uptime"` // 运行时间（秒）
}

// GetSnapshot 获取统计快照（线程安全）.
func (s *ZeroTrustStats) GetSnapshot() *ZeroTrustStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &ZeroTrustStats{
		TotalPolicies:      s.TotalPolicies,
		ActivePolicies:     s.ActivePolicies,
		DisabledPolicies:   s.DisabledPolicies,
		TotalSegments:      s.TotalSegments,
		ActiveSegments:     s.ActiveSegments,
		IsolatedSegments:   s.IsolatedSegments,
		TotalAccessRules:   s.TotalAccessRules,
		ActiveSessions:     s.ActiveSessions,
		TotalRequests:      s.TotalRequests,
		AllowedRequests:    s.AllowedRequests,
		DeniedRequests:     s.DeniedRequests,
		TotalIdentities:    s.TotalIdentities,
		VerifiedDevices:    s.VerifiedDevices,
		MFAEnabledUsers:    s.MFAEnabledUsers,
		TotalAuditEntries:  s.TotalAuditEntries,
		PolicyViolations:   s.PolicyViolations,
		SecurityAlerts:     s.SecurityAlerts,
		WGTunnels:          s.WGTunnels,
		WGPeers:            s.WGPeers,
		WGActivePeers:      s.WGActivePeers,
		WGBytesSent:        s.WGBytesSent,
		WGBytesRecv:        s.WGBytesRecv,
		LastPolicyUpdate:   s.LastPolicyUpdate,
		LastAuditCheck:     s.LastAuditCheck,
		Uptime:             s.Uptime,
	}
}
