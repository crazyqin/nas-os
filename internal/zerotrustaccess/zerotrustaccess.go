// Package zerotrustaccess 提供零信任网络访问系统
// 身份驱动访问控制、微分段隔离、持续认证、设备态势评估
package zerotrustaccess

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ========== 常量 ==========

const (
	Version = "1.0.0"
)

// ========== 信任级别 ==========

// TrustLevel 信任级别
type TrustLevel int

const (
	TrustNone     TrustLevel = 0 // 无信任
	TrustLow      TrustLevel = 1 // 低信任
	TrustMedium   TrustLevel = 2 // 中信任
	TrustHigh     TrustLevel = 3 // 高信任
	TrustVerified TrustLevel = 4 // 已验证
)

func (t TrustLevel) String() string {
	switch t {
	case TrustNone:
		return "none"
	case TrustLow:
		return "low"
	case TrustMedium:
		return "medium"
	case TrustHigh:
		return "high"
	case TrustVerified:
		return "verified"
	default:
		return "unknown"
	}
}

// ========== 设备状态 ==========

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceStatusUnknown    DeviceStatus = "unknown"
	DeviceStatusCompliant  DeviceStatus = "compliant"
	DeviceStatusNonCompliant DeviceStatus = "non_compliant"
	DeviceStatusCompromised DeviceStatus = "compromised"
	DeviceStatusBlocked    DeviceStatus = "blocked"
)

// ========== 访问策略 ==========

// AccessAction 访问动作
type AccessAction string

const (
	ActionAllow  AccessAction = "allow"
	ActionDeny   AccessAction = "deny"
	ActionMFA    AccessAction = "mfa"
	ActionLimit  AccessAction = "limit"
	ActionAudit  AccessAction = "audit"
)

// ========== 数据结构 ==========

// Identity 身份信息
type Identity struct {
	ID           string            `json:"id"`
	Username     string            `json:"username"`
	Email        string            `json:"email"`
	Groups       []string          `json:"groups"`
	Department   string            `json:"department"`
	TrustLevel   TrustLevel        `json:"trust_level"`
	LastLogin    time.Time         `json:"last_login"`
	LoginCount   int               `json:"login_count"`
	MFAEnabled   bool              `json:"mfa_enabled"`
	MFAMethod    string            `json:"mfa_method"`
	Attributes   map[string]string `json:"attributes"`
	RiskScore    float64           `json:"risk_score"` // 0-100
	Active       bool              `json:"active"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// Device 设备信息
type Device struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Type          string       `json:"type"` // "desktop", "laptop", "mobile", "server", "iot"
	OS            string       `json:"os"`
	OSVersion     string       `json:"os_version"`
	IPAddress     string       `json:"ip_address"`
	MACAddress    string       `json:"mac_address"`
	UserID        string       `json:"user_id"`
	Status        DeviceStatus `json:"status"`
	TrustLevel    TrustLevel   `json:"trust_level"`
	Compliance    float64      `json:"compliance"` // 0-100
	LastSeen      time.Time    `json:"last_seen"`
	RegisteredAt  time.Time    `json:"registered_at"`
	Attributes    map[string]string `json:"attributes"`
	HealthChecks  []HealthCheck `json:"health_checks"`
}

// HealthCheck 健康检查
type HealthCheck struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"` // "pass", "fail", "warning"
	Message   string    `json:"message"`
	CheckedAt time.Time `json:"checked_at"`
}

// AccessPolicy 访问策略
type AccessPolicy struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Priority    int          `json:"priority"`
	Enabled     bool         `json:"enabled"`
	Subject     PolicySubject `json:"subject"`
	Resource    PolicyResource `json:"resource"`
	Action      AccessAction `json:"action"`
	Conditions  []Condition  `json:"conditions"`
	Constraints []Constraint `json:"constraints"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// PolicySubject 策略主体
type PolicySubject struct {
	Type     string   `json:"type"` // "user", "group", "device", "service"
	Values   []string `json:"values"`
}

// PolicyResource 策略资源
type PolicyResource struct {
	Type     string   `json:"type"` // "application", "data", "network", "api"
	Values   []string `json:"values"`
}

// Condition 策略条件
type Condition struct {
	Type     string `json:"type"` // "time", "location", "trust_level", "device_status", "risk_score"
	Operator string `json:"operator"` // "eq", "neq", "gt", "lt", "gte", "lte", "in", "not_in"
	Value    string `json:"value"`
}

// Constraint 策略约束
type Constraint struct {
	Type  string `json:"type"` // "mfa", "encryption", "logging", "session_timeout"
	Value string `json:"value"`
}

// AccessRequest 访问请求
type AccessRequest struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	DeviceID    string    `json:"device_id"`
	Resource    string    `json:"resource"`
	Action      string    `json:"action"`
	IPAddress   string    `json:"ip_address"`
	UserAgent   string    `json:"user_agent"`
	Timestamp   time.Time `json:"timestamp"`
	Context     map[string]string `json:"context"`
}

// AccessDecision 访问决策
type AccessDecision struct {
	RequestID   string       `json:"request_id"`
	Allowed     bool         `json:"allowed"`
	Action      AccessAction `json:"action"`
	TrustLevel  TrustLevel   `json:"trust_level"`
	RiskScore   float64      `json:"risk_score"`
	Reason      string       `json:"reason"`
	Constraints []Constraint `json:"constraints"`
	SessionID   string       `json:"session_id"`
	ExpiresAt   time.Time    `json:"expires_at"`
	DecidedAt   time.Time    `json:"decided_at"`
}

// Session 会话信息
type Session struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	DeviceID    string    `json:"device_id"`
	TrustLevel  TrustLevel `json:"trust_level"`
	StartedAt   time.Time `json:"started_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	LastActive  time.Time `json:"last_active"`
	IPAddress   string    `json:"ip_address"`
	Resources   []string  `json:"resources"`
	Active      bool      `json:"active"`
}

// MicroSegment 微分段
type MicroSegment struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Resources   []string `json:"resources"`
	TrustLevel  TrustLevel `json:"trust_level"`
	Isolation   string   `json:"isolation"` // "strict", "moderate", "permissive"
	AllowedIn   []string `json:"allowed_in"`
	AllowedOut  []string `json:"allowed_out"`
	Policies    []string `json:"policies"`
}

// RiskAssessment 风险评估
type RiskAssessment struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	DeviceID    string    `json:"device_id"`
	Score       float64   `json:"score"` // 0-100
	Factors     []RiskFactor `json:"factors"`
	Level       string    `json:"level"` // "low", "medium", "high", "critical"
	Timestamp   time.Time `json:"timestamp"`
	ValidUntil  time.Time `json:"valid_until"`
}

// RiskFactor 风险因素
type RiskFactor struct {
	Name    string  `json:"name"`
	Weight  float64 `json:"weight"`
	Score   float64 `json:"score"`
	Details string  `json:"details"`
}

// AuditLog 审计日志
type AuditLog struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
	UserID    string    `json:"user_id"`
	DeviceID  string    `json:"device_id"`
	Resource  string    `json:"resource"`
	Action    string    `json:"action"`
	Result    string    `json:"result"`
	Details   string    `json:"details"`
	RiskScore float64   `json:"risk_score"`
	IPAddress string    `json:"ip_address"`
}

// ========== 管理器 ==========

// ZeroTrustManager 零信任管理器
type ZeroTrustManager struct {
	mu         sync.RWMutex
	identities map[string]*Identity
	devices    map[string]*Device
	policies   map[string]*AccessPolicy
	sessions   map[string]*Session
	segments   map[string]*MicroSegment
	assessments map[string]*RiskAssessment
	auditLogs  []*AuditLog
}

// NewZeroTrustManager 创建零信任管理器
func NewZeroTrustManager() *ZeroTrustManager {
	return &ZeroTrustManager{
		identities:  make(map[string]*Identity),
		devices:     make(map[string]*Device),
		policies:    make(map[string]*AccessPolicy),
		sessions:    make(map[string]*Session),
		segments:    make(map[string]*MicroSegment),
		assessments: make(map[string]*RiskAssessment),
		auditLogs:   make([]*AuditLog, 0, 1000),
	}
}

// RegisterIdentity 注册身份
func (m *ZeroTrustManager) RegisterIdentity(identity *Identity) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if identity.ID == "" {
		identity.ID = fmt.Sprintf("user-%d", time.Now().UnixNano())
	}
	identity.CreatedAt = time.Now()
	identity.UpdatedAt = time.Now()
	identity.Active = true

	m.identities[identity.ID] = identity
	return nil
}

// RegisterDevice 注册设备
func (m *ZeroTrustManager) RegisterDevice(device *Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device.ID == "" {
		device.ID = fmt.Sprintf("device-%d", time.Now().UnixNano())
	}
	device.RegisteredAt = time.Now()
	device.LastSeen = time.Now()
	device.Status = DeviceStatusUnknown
	device.TrustLevel = TrustNone

	m.devices[device.ID] = device
	return nil
}

// AddPolicy 添加策略
func (m *ZeroTrustManager) AddPolicy(policy *AccessPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = fmt.Sprintf("policy-%d", time.Now().UnixNano())
	}
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	m.policies[policy.ID] = policy
	return nil
}

// EvaluateAccess 评估访问请求
func (m *ZeroTrustManager) EvaluateAccess(request *AccessRequest) *AccessDecision {
	m.mu.RLock()
	defer m.mu.RUnlock()

	decision := &AccessDecision{
		RequestID: request.ID,
		DecidedAt: time.Now(),
	}

	// 获取身份和设备信息
	identity, hasIdentity := m.identities[request.UserID]
	device, hasDevice := m.devices[request.DeviceID]

	if !hasIdentity {
		decision.Allowed = false
		decision.Action = ActionDeny
		decision.Reason = "身份未注册"
		decision.TrustLevel = TrustNone
		decision.RiskScore = 100
		m.logAccess(request, decision)
		return decision
	}

	// 计算信任级别
	trustLevel := identity.TrustLevel
	if hasDevice {
		trustLevel = minTrustLevel(trustLevel, device.TrustLevel)
	}
	decision.TrustLevel = trustLevel

	// 计算风险分数
	riskScore := m.calculateRiskScore(identity, device, request)
	decision.RiskScore = riskScore

	// 匹配策略
	for _, policy := range m.policies {
		if !policy.Enabled {
			continue
		}

		if m.matchPolicy(policy, identity, device, request) {
			decision.Action = policy.Action
			decision.Constraints = policy.Constraints

			switch policy.Action {
			case ActionAllow:
				if trustLevel >= TrustMedium && riskScore < 50 {
					decision.Allowed = true
					decision.Reason = "策略允许"
				} else {
					decision.Allowed = false
					decision.Action = ActionMFA
					decision.Reason = "需要MFA验证"
				}
			case ActionDeny:
				decision.Allowed = false
				decision.Reason = "策略拒绝"
			case ActionMFA:
				decision.Allowed = false
				decision.Action = ActionMFA
				decision.Reason = "需要MFA验证"
			case ActionLimit:
				decision.Allowed = true
				decision.Reason = "受限访问"
			case ActionAudit:
				decision.Allowed = true
				decision.Reason = "允许但需审计"
			}

			break
		}
	}

	// 默认策略
	if decision.Action == "" {
		if trustLevel >= TrustHigh && riskScore < 30 {
			decision.Allowed = true
			decision.Action = ActionAllow
			decision.Reason = "默认允许（高信任）"
		} else {
			decision.Allowed = false
			decision.Action = ActionDeny
			decision.Reason = "默认拒绝（信任不足）"
		}
	}

	// 创建会话
	if decision.Allowed {
		session := &Session{
			ID:         fmt.Sprintf("session-%d", time.Now().UnixNano()),
			UserID:     request.UserID,
			DeviceID:   request.DeviceID,
			TrustLevel: trustLevel,
			StartedAt:  time.Now(),
			ExpiresAt:  time.Now().Add(time.Hour),
			LastActive: time.Now(),
			IPAddress:  request.IPAddress,
			Resources:  []string{request.Resource},
			Active:     true,
		}
		decision.SessionID = session.ID
		decision.ExpiresAt = session.ExpiresAt
		m.sessions[session.ID] = session
	}

	m.logAccess(request, decision)
	return decision
}

// calculateRiskScore 计算风险分数
func (m *ZeroTrustManager) calculateRiskScore(identity *Identity, device *Device, request *AccessRequest) float64 {
	score := 0.0
	factors := 0

	// 身份风险
	if identity.RiskScore > 0 {
		score += identity.RiskScore
		factors++
	}

	// 设备风险
	if device != nil {
		if device.Status == DeviceStatusNonCompliant {
			score += 30
		} else if device.Status == DeviceStatusCompromised {
			score += 80
		}
		if device.Compliance < 50 {
			score += 20
		}
		factors++
	}

	// 时间风险（非工作时间）
	hour := time.Now().Hour()
	if hour < 6 || hour > 22 {
		score += 10
	}
	factors++

	// MFA状态
	if !identity.MFAEnabled {
		score += 15
	}
	factors++

	if factors == 0 {
		return 50
	}
	return score / float64(factors)
}

// matchPolicy 匹配策略
func (m *ZeroTrustManager) matchPolicy(policy *AccessPolicy, identity *Identity, device *Device, request *AccessRequest) bool {
	// 检查主体匹配
	subjectMatch := false
	for _, val := range policy.Subject.Values {
		switch policy.Subject.Type {
		case "user":
			if identity.ID == val || identity.Username == val {
				subjectMatch = true
			}
		case "group":
			for _, g := range identity.Groups {
				if g == val {
					subjectMatch = true
				}
			}
		}
	}
	if !subjectMatch {
		return false
	}

	// 检查资源匹配
	resourceMatch := false
	for _, val := range policy.Resource.Values {
		if request.Resource == val || val == "*" {
			resourceMatch = true
		}
	}
	return resourceMatch
}

// logAccess 记录访问日志
func (m *ZeroTrustManager) logAccess(request *AccessRequest, decision *AccessDecision) {
	log := &AuditLog{
		ID:        fmt.Sprintf("log-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		EventType: "access",
		UserID:    request.UserID,
		DeviceID:  request.DeviceID,
		Resource:  request.Resource,
		Action:    request.Action,
		Details:   decision.Reason,
		RiskScore: decision.RiskScore,
		IPAddress: request.IPAddress,
	}

	if decision.Allowed {
		log.Result = "allow"
	} else {
		log.Result = "deny"
	}

	m.auditLogs = append(m.auditLogs, log)
}

// GetIdentities 获取所有身份
func (m *ZeroTrustManager) GetIdentities() []*Identity {
	m.mu.RLock()
	defer m.mu.RUnlock()

	identities := make([]*Identity, 0, len(m.identities))
	for _, i := range m.identities {
		identities = append(identities, i)
	}
	return identities
}

// GetDevices 获取所有设备
func (m *ZeroTrustManager) GetDevices() []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*Device, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	return devices
}

// GetPolicies 获取所有策略
func (m *ZeroTrustManager) GetPolicies() []*AccessPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*AccessPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// GetSessions 获取所有会话
func (m *ZeroTrustManager) GetSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// GetAuditLogs 获取审计日志
func (m *ZeroTrustManager) GetAuditLogs(limit int) []*AuditLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.auditLogs) {
		limit = len(m.auditLogs)
	}

	start := len(m.auditLogs) - limit
	if start < 0 {
		start = 0
	}

	logs := make([]*AuditLog, limit)
	copy(logs, m.auditLogs[start:])
	return logs
}

// AddSegment 添加微分段
func (m *ZeroTrustManager) AddSegment(segment *MicroSegment) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if segment.ID == "" {
		segment.ID = fmt.Sprintf("segment-%d", time.Now().UnixNano())
	}
	m.segments[segment.ID] = segment
	return nil
}

// GetSegments 获取所有微分段
func (m *ZeroTrustManager) GetSegments() []*MicroSegment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	segments := make([]*MicroSegment, 0, len(m.segments))
	for _, s := range m.segments {
		segments = append(segments, s)
	}
	return segments
}

// 辅助函数
func minTrustLevel(a, b TrustLevel) TrustLevel {
	if a < b {
		return a
	}
	return b
}

// ========== HTTP API ==========

// Handler HTTP API处理器
type Handler struct {
	manager *ZeroTrustManager
}

// NewHandler 创建处理器
func NewHandler(manager *ZeroTrustManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/identities", h.handleIdentities)
	mux.HandleFunc(prefix+"/devices", h.handleDevices)
	mux.HandleFunc(prefix+"/policies", h.handlePolicies)
	mux.HandleFunc(prefix+"/evaluate", h.handleEvaluate)
	mux.HandleFunc(prefix+"/sessions", h.handleSessions)
	mux.HandleFunc(prefix+"/segments", h.handleSegments)
	mux.HandleFunc(prefix+"/audit", h.handleAudit)
}

func (h *Handler) handleIdentities(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(h.manager.GetIdentities())
	case http.MethodPost:
		var identity Identity
		if err := json.NewDecoder(r.Body).Decode(&identity); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.manager.RegisterIdentity(&identity)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(identity)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleDevices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(h.manager.GetDevices())
	case http.MethodPost:
		var device Device
		if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.manager.RegisterDevice(&device)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(device)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handlePolicies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(h.manager.GetPolicies())
	case http.MethodPost:
		var policy AccessPolicy
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.manager.AddPolicy(&policy)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(policy)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request AccessRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	decision := h.manager.EvaluateAccess(&request)
	json.NewEncoder(w).Encode(decision)
}

func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.manager.GetSessions())
}

func (h *Handler) handleSegments(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(h.manager.GetSegments())
	case http.MethodPost:
		var segment MicroSegment
		if err := json.NewDecoder(r.Body).Decode(&segment); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.manager.AddSegment(&segment)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(segment)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := 100
	json.NewEncoder(w).Encode(h.manager.GetAuditLogs(limit))
}
