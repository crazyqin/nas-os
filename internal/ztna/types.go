// Package ztna 提供零信任网络访问（Zero Trust Network Access）功能，
// 实现基于身份的访问控制、设备信任评估和动态策略管理。
package ztna

import "time"

// ========== 策略类型 ==========

// Policy 零信任访问策略.
type Policy struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Enabled     bool         `json:"enabled"`
	Priority    int          `json:"priority"`             // 优先级，数字越小优先级越高
	Rules       []AccessRule `json:"rules"`                // 访问规则列表
	Conditions  []Condition  `json:"conditions,omitempty"` // 额外条件
	MinTrust    int          `json:"min_trust"`            // 最低信任分（0-100）
	Action      PolicyAction `json:"action"`               // 策略动作
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// AccessRule 访问规则，定义谁可以在什么条件下访问什么资源.
type AccessRule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Enabled     bool     `json:"enabled"`
	Identity    string   `json:"identity"`               // 用户/组/角色标识
	Resource    string   `json:"resource"`               // 资源标识（服务、端点等）
	Actions     []string `json:"actions"`                // 允许的操作（read, write, admin 等）
	DeviceTypes []string `json:"device_types,omitempty"` // 允许的设备类型
	MinTrust    int      `json:"min_trust"`              // 最低设备信任分
}

// PolicyAction 策略动作类型.
type PolicyAction string

const (
	// ActionAllow 允许访问.
	ActionAllow PolicyAction = "allow"
	// ActionDeny 拒绝访问.
	ActionDeny PolicyAction = "deny"
	// ActionStepUp 需要额外验证（step-up authentication）.
	ActionStepUp PolicyAction = "step_up"
)

// Condition 策略条件，用于动态策略评估.
type Condition struct {
	Type     ConditionType `json:"type"`
	Operator string        `json:"operator"` // eq, ne, gt, lt, gte, lte, in, not_in
	Value    string        `json:"value"`
}

// ConditionType 条件类型.
type ConditionType string

const (
	// ConditionTime 时间条件（工作时间等）.
	ConditionTime ConditionType = "time"
	// ConditionLocation 地理位置条件.
	ConditionLocation ConditionType = "location"
	// ConditionNetwork 网络条件（IP段、VPN等）.
	ConditionNetwork ConditionType = "network"
	// ConditionDeviceOS 设备操作系统条件.
	ConditionDeviceOS ConditionType = "device_os"
)

// ========== 设备信任类型 ==========

// DeviceTrust 设备信任信息.
type DeviceTrust struct {
	DeviceID      string        `json:"device_id"`
	UserID        string        `json:"user_id"`
	DeviceName    string        `json:"device_name"`
	DeviceType    string        `json:"device_type"`    // desktop, mobile, tablet
	OS            string        `json:"os"`             // 操作系统
	OSVersion     string        `json:"os_version"`     // 操作系统版本
	Compliant     bool          `json:"compliant"`      // 是否符合安全策略
	ManagedDevice bool          `json:"managed_device"` // 是否受管理设备
	PatchLevel    string        `json:"patch_level"`    // 补丁级别
	TrustScore    int           `json:"trust_score"`    // 信任分（0-100）
	TrustFactors  []TrustFactor `json:"trust_factors"`  // 信任因素明细
	LastVerified  time.Time     `json:"last_verified"`  // 最后验证时间
	Status        DeviceStatus  `json:"status"`         // 设备状态
}

// TrustFactor 信任评分因素.
type TrustFactor struct {
	Name   string `json:"name"`   // 因素名称
	Weight int    `json:"weight"` // 权重
	Score  int    `json:"score"`  // 得分
	Detail string `json:"detail"` // 详细说明
}

// DeviceStatus 设备状态.
type DeviceStatus string

const (
	// DeviceStatusTrusted 可信设备.
	DeviceStatusTrusted DeviceStatus = "trusted"
	// DeviceStatusUnknown 未知设备.
	DeviceStatusUnknown DeviceStatus = "unknown"
	// DeviceStatusCompromised 已被入侵的设备.
	DeviceStatusCompromised DeviceStatus = "compromised"
	// DeviceStatusBlocked 已封禁设备.
	DeviceStatusBlocked DeviceStatus = "blocked"
)

// ========== 会话类型 ==========

// Session 零信任访问会话.
type Session struct {
	ID           string        `json:"id"`
	UserID       string        `json:"user_id"`
	DeviceID     string        `json:"device_id"`
	Resource     string        `json:"resource"`
	Actions      []string      `json:"actions"`
	TrustScore   int           `json:"trust_score"`
	PolicyID     string        `json:"policy_id"`
	IPAddress    string        `json:"ip_address"`
	UserAgent    string        `json:"user_agent"`
	StartedAt    time.Time     `json:"started_at"`
	ExpiresAt    time.Time     `json:"expires_at"`
	LastActivity time.Time     `json:"last_activity"`
	Status       SessionStatus `json:"status"`
}

// SessionStatus 会话状态.
type SessionStatus string

const (
	// SessionStatusActive 活跃会话.
	SessionStatusActive SessionStatus = "active"
	// SessionStatusExpired 已过期会话.
	SessionStatusExpired SessionStatus = "expired"
	// SessionStatusRevoked 已撤销会话.
	SessionStatusRevoked SessionStatus = "revoked"
)

// ========== 身份类型 ==========

// Identity 身份信息，用于零信任身份验证.
type Identity struct {
	ID       string   `json:"id"`
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
	Groups   []string `json:"groups"`
	Verified bool     `json:"verified"` // 身份是否已验证
}

// ========== 请求/响应类型 ==========

// VerifyRequest 设备验证请求.
type VerifyRequest struct {
	DeviceID   string `json:"device_id" binding:"required"`
	UserID     string `json:"user_id" binding:"required"`
	DeviceName string `json:"device_name"`
	DeviceType string `json:"device_type"`
	OS         string `json:"os"`
	OSVersion  string `json:"os_version"`
	PatchLevel string `json:"patch_level"`
}

// VerifyResponse 设备验证响应.
type VerifyResponse struct {
	DeviceID   string        `json:"device_id"`
	TrustScore int           `json:"trust_score"`
	Status     DeviceStatus  `json:"status"`
	Compliant  bool          `json:"compliant"`
	Factors    []TrustFactor `json:"factors"`
}

// AccessCheckRequest 访问检查请求.
type AccessCheckRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	DeviceID string `json:"device_id" binding:"required"`
	Resource string `json:"resource" binding:"required"`
	Action   string `json:"action" binding:"required"`
}

// AccessCheckResponse 访问检查响应.
type AccessCheckResponse struct {
	Allowed   bool         `json:"allowed"`
	Action    PolicyAction `json:"action"`
	SessionID string       `json:"session_id,omitempty"`
	Reason    string       `json:"reason"`
}

// CreatePolicyRequest 创建策略请求.
type CreatePolicyRequest struct {
	Name        string       `json:"name" binding:"required"`
	Description string       `json:"description"`
	Priority    int          `json:"priority"`
	Rules       []AccessRule `json:"rules"`
	Conditions  []Condition  `json:"conditions"`
	MinTrust    int          `json:"min_trust"`
	Action      PolicyAction `json:"action" binding:"required"`
}
