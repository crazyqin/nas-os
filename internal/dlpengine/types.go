// Package dlpengine 提供数据防泄漏引擎功能，检测和阻止敏感数据外泄。
// 支持多种敏感数据模式识别、内容扫描和传输阻断。
package dlpengine

import "time"

// SensitivityLevel 敏感级别
type SensitivityLevel string

const (
	SensitivityCritical SensitivityLevel = "critical"
	SensitivityHigh     SensitivityLevel = "high"
	SensitivityMedium   SensitivityLevel = "medium"
	SensitivityLow      SensitivityLevel = "low"
	SensitivityNone     SensitivityLevel = "none"
)

// PatternType 模式类型
type PatternType string

const (
	PatternRegex    PatternType = "regex"
	PatternKeyword  PatternType = "keyword"
	PatternNLP      PatternType = "nlp"
	PatternFingerprint PatternType = "fingerprint"
	PatternML       PatternType = "ml"
)

// ViolationStatus 违规状态
type ViolationStatus string

const (
	ViolationStatusNew        ViolationStatus = "new"
	ViolationStatusReviewing  ViolationStatus = "reviewing"
	ViolationStatusConfirmed  ViolationStatus = "confirmed"
	ViolationStatusFalsePositive ViolationStatus = "false_positive"
	ViolationStatusResolved   ViolationStatus = "resolved"
	ViolationStatusDismissed  ViolationStatus = "dismissed"
)

// TransferProtocol 传输协议
type TransferProtocol string

const (
	ProtocolHTTP   TransferProtocol = "http"
	ProtocolHTTPS  TransferProtocol = "https"
	ProtocolFTP    TransferProtocol = "ftp"
	ProtocolSMTP   TransferProtocol = "smtp"
	ProtocolSMB    TransferProtocol = "smb"
	ProtocolUSB    TransferProtocol = "usb"
	ProtocolCloud  TransferProtocol = "cloud"
	ProtocolAPI    TransferProtocol = "api"
)

// PolicyAction 策略动作
type PolicyAction string

const (
	ActionBlock    PolicyAction = "block"
	ActionWarn     PolicyAction = "warn"
	ActionQuarantine PolicyAction = "quarantine"
	ActionEncrypt  PolicyAction = "encrypt"
	ActionRedact   PolicyAction = "redact"
	ActionLog      PolicyAction = "log"
	ActionNotify   PolicyAction = "notify"
)

// DLPPolicy DLP策略
type DLPPolicy struct {
	ID          string           `json:"id"`
	Name        string           `json:"name" binding:"required"`
	Description string           `json:"description,omitempty"`
	Enabled     bool             `json:"enabled"`
	Priority    int              `json:"priority"`
	Action      PolicyAction     `json:"action" binding:"required"`
	Level       SensitivityLevel `json:"level" binding:"required"`
	Patterns    []string         `json:"patterns" binding:"required,min=1"`
	Channels    []TransferProtocol `json:"channels,omitempty"`
	Users       []string         `json:"users,omitempty"`
	Groups      []string         `json:"groups,omitempty"`
	Exceptions  []string         `json:"exceptions,omitempty"`
	NotifyEmail string           `json:"notify_email,omitempty"`
	MaxMatches  int              `json:"max_matches,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// SensitivePattern 敏感数据模式
type SensitivePattern struct {
	ID          string           `json:"id"`
	Name        string           `json:"name" binding:"required"`
	Description string           `json:"description,omitempty"`
	Type        PatternType      `json:"type" binding:"required"`
	Level       SensitivityLevel `json:"level" binding:"required"`
	Pattern     string           `json:"pattern" binding:"required"`
	IsEnabled   bool             `json:"is_enabled"`
	Category    string           `json:"category,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
	Examples    []string         `json:"examples,omitempty"`
	Confidence  float64          `json:"confidence"`
	LastUpdated time.Time        `json:"last_updated"`
	CreatedAt   time.Time        `json:"created_at"`
}

// Violation 违规记录
type Violation struct {
	ID          string           `json:"id"`
	PolicyID    string           `json:"policy_id"`
	PolicyName  string           `json:"policy_name"`
	PatternID   string           `json:"pattern_id"`
	Level       SensitivityLevel `json:"level"`
	Status      ViolationStatus  `json:"status"`
	UserID      string           `json:"user_id"`
	UserName    string           `json:"user_name,omitempty"`
	Resource    string           `json:"resource"`
	Channel     TransferProtocol `json:"channel"`
	SourceIP    string           `json:"source_ip,omitempty"`
	Destination string           `json:"destination,omitempty"`
	MatchCount  int              `json:"match_count"`
	MatchedData []MatchedContent `json:"matched_data,omitempty"`
	Context     string           `json:"context,omitempty"`
	Action      PolicyAction     `json:"action"`
	Blocked     bool             `json:"blocked"`
	Timestamp   time.Time        `json:"timestamp"`
	ReviewedBy  string           `json:"reviewed_by,omitempty"`
	ReviewedAt  *time.Time       `json:"reviewed_at,omitempty"`
	Notes       string           `json:"notes,omitempty"`
}

// MatchedContent 匹配内容
type MatchedContent struct {
	PatternName string `json:"pattern_name"`
	Match       string `json:"match"`
	StartPos    int    `json:"start_pos"`
	EndPos      int    `json:"end_pos"`
	Confidence  float64 `json:"confidence"`
}

// ScanResult 扫描结果
type ScanResult struct {
	ID              string           `json:"id"`
	ScanID          string           `json:"scan_id"`
	Resource        string           `json:"resource"`
	ContentType     string           `json:"content_type,omitempty"`
	Size            int64            `json:"size"`
	HasViolation    bool             `json:"has_violation"`
	ViolationCount  int              `json:"violation_count"`
	MaxLevel        SensitivityLevel `json:"max_level"`
	Violations      []*Violation     `json:"violations,omitempty"`
	PatternsMatched int              `json:"patterns_matched"`
	ScanDuration    time.Duration    `json:"scan_duration"`
	Timestamp       time.Time        `json:"timestamp"`
	Blocked         bool             `json:"blocked"`
	Action          PolicyAction     `json:"action"`
}

// ScanRequest 扫描请求
type ScanRequest struct {
	ID          string `json:"id"`
	Content     []byte `json:"content" binding:"required"`
	Resource    string `json:"resource" binding:"required"`
	ContentType string `json:"content_type,omitempty"`
	UserID      string `json:"user_id,omitempty"`
	Channel     TransferProtocol `json:"channel,omitempty"`
	SourceIP    string `json:"source_ip,omitempty"`
	Destination string `json:"destination,omitempty"`
}

// ContentInspection 内容检查结果
type ContentInspection struct {
	ContentType string            `json:"content_type"`
	Encoding    string            `json:"encoding,omitempty"`
	Language    string            `json:"language,omitempty"`
	HasPII      bool              `json:"has_pii"`
	HasPHI      bool              `json:"has_phi"`
	HasPCI      bool              `json:"has_pci"`
	Categories  map[string]int    `json:"categories"`
	Entities    []DetectedEntity  `json:"entities,omitempty"`
}

// DetectedEntity 检测到的实体
type DetectedEntity struct {
	Type       string  `json:"type"`
	Value      string  `json:"value"`
	StartPos   int     `json:"start_pos"`
	EndPos     int     `json:"end_pos"`
	Confidence float64 `json:"confidence"`
}

// ScanStats 扫描统计
type ScanStats struct {
	TotalScans      int64            `json:"total_scans"`
	ViolationsFound int64            `json:"violations_found"`
	BlockedTransfers int64           `json:"blocked_transfers"`
	ByLevel         map[SensitivityLevel]int64 `json:"by_level"`
	ByChannel       map[TransferProtocol]int64 `json:"by_channel"`
	ByAction        map[PolicyAction]int64     `json:"by_action"`
	TopPatterns     []PatternStat    `json:"top_patterns"`
	TopUsers        []UserStat       `json:"top_users"`
}

// PatternStat 模式统计
type PatternStat struct {
	PatternID   string `json:"pattern_id"`
	PatternName string `json:"pattern_name"`
	MatchCount  int64  `json:"match_count"`
}

// UserStat 用户统计
type UserStat struct {
	UserID       string `json:"user_id"`
	UserName     string `json:"user_name"`
	ViolationCount int64 `json:"violation_count"`
}

// DLPConfig DLP引擎配置
type DLPConfig struct {
	Enabled             bool    `json:"enabled"`
	ScanTimeout         int     `json:"scan_timeout_seconds"`
	MaxContentSize      int64   `json:"max_content_size_bytes"`
	MinConfidence       float64 `json:"min_confidence"`
	AutoBlock           bool    `json:"auto_block"`
	QuarantineEnabled   bool    `json:"quarantine_enabled"`
	QuarantinePath      string  `json:"quarantine_path"`
	NotificationEnabled bool    `json:"notification_enabled"`
	AuditEnabled        bool    `json:"audit_enabled"`
	RetentionDays       int     `json:"retention_days"`
	MaxViolationsPerUser int    `json:"max_violations_per_user"`
	AlertThreshold      int     `json:"alert_threshold"`
	ContentInspection   bool    `json:"content_inspection"`
}

// DefaultDLPConfig 默认配置
func DefaultDLPConfig() *DLPConfig {
	return &DLPConfig{
		Enabled:             true,
		ScanTimeout:         30,
		MaxContentSize:      100 * 1024 * 1024, // 100MB
		MinConfidence:       0.7,
		AutoBlock:           true,
		QuarantineEnabled:   true,
		QuarantinePath:      "/var/quarantine/dlp",
		NotificationEnabled: true,
		AuditEnabled:        true,
		RetentionDays:       90,
		MaxViolationsPerUser: 100,
		AlertThreshold:      10,
		ContentInspection:   true,
	}
}
