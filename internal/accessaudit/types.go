// Package accessaudit 提供零信任访问审计功能
package accessaudit

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrRecordNotFound 审计记录不存在.
	ErrRecordNotFound = errors.New("审计记录不存在")
	// ErrInvalidTimeRange 无效的时间范围.
	ErrInvalidTimeRange = errors.New("无效的时间范围")
	// ErrInvalidQuery 无效的查询参数.
	ErrInvalidQuery = errors.New("无效的查询参数")
)

// ========== 风险等级 ==========

// RiskLevel 风险等级.
type RiskLevel string

const (
	// RiskLow 低风险.
	RiskLow RiskLevel = "low"
	// RiskMedium 中风险.
	RiskMedium RiskLevel = "medium"
	// RiskHigh 高风险.
	RiskHigh RiskLevel = "high"
	// RiskCritical 严重风险.
	RiskCritical RiskLevel = "critical"
)

// ========== 访问状态 ==========

// AccessStatus 访问状态.
type AccessStatus string

const (
	// StatusSuccess 访问成功.
	StatusSuccess AccessStatus = "success"
	// StatusDenied 访问拒绝.
	StatusDenied AccessStatus = "denied"
	// StatusFailed 访问失败.
	StatusFailed AccessStatus = "failed"
	// StatusError 系统错误.
	StatusError AccessStatus = "error"
)

// ========== 核心数据结构 ==========

// AccessRecord 访问审计记录.
type AccessRecord struct {
	ID           string            `json:"id"`
	Timestamp    time.Time         `json:"timestamp"`
	UserID       string            `json:"user_id"`
	UserName     string            `json:"user_name,omitempty"`
	SourceIP     string            `json:"source_ip"`
	UserAgent    string            `json:"user_agent,omitempty"`
	Resource     string            `json:"resource"`
	ResourceType string            `json:"resource_type,omitempty"`
	Action       string            `json:"action"`
	Status       AccessStatus      `json:"status"`
	StatusCode   int               `json:"status_code,omitempty"`
	Duration     int64             `json:"duration_ms,omitempty"` // 毫秒
	RequestSize  int64             `json:"request_size,omitempty"`
	RiskScore    float64           `json:"risk_score"`
	RiskLevel    RiskLevel         `json:"risk_level"`
	Reason       string            `json:"reason,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// AccessQuery 访问记录查询条件.
type AccessQuery struct {
	StartTime    *time.Time   `json:"start_time,omitempty"`
	EndTime      *time.Time   `json:"end_time,omitempty"`
	UserID       string       `json:"user_id,omitempty"`
	SourceIP     string       `json:"source_ip,omitempty"`
	Resource     string       `json:"resource,omitempty"`
	ResourceType string       `json:"resource_type,omitempty"`
	Action       string       `json:"action,omitempty"`
	Status       AccessStatus `json:"status,omitempty"`
	RiskLevel    RiskLevel    `json:"risk_level,omitempty"`
	MinRiskScore *float64     `json:"min_risk_score,omitempty"`
	Limit        int          `json:"limit,omitempty"`
	Offset       int          `json:"offset,omitempty"`
}

// AnomalyDetection 异常检测结果.
type AnomalyDetection struct {
	ID             string    `json:"id"`
	Timestamp      time.Time `json:"timestamp"`
	AnomalyType    string    `json:"anomaly_type"`
	Description    string    `json:"description"`
	Severity       RiskLevel `json:"severity"`
	UserID         string    `json:"user_id,omitempty"`
	SourceIP       string    `json:"source_ip,omitempty"`
	RelatedRecords []string  `json:"related_records,omitempty"`
	RiskScore      float64   `json:"risk_score"`
	IsResolved     bool      `json:"is_resolved"`
}

// AuditReport 审计报告.
type AuditReport struct {
	ID               string             `json:"id"`
	GeneratedAt      time.Time          `json:"generated_at"`
	StartTime        time.Time          `json:"start_time"`
	EndTime          time.Time          `json:"end_time"`
	TotalRecords     int                `json:"total_records"`
	SuccessCount     int                `json:"success_count"`
	DeniedCount      int                `json:"denied_count"`
	FailedCount      int                `json:"failed_count"`
	ErrorCount       int                `json:"error_count"`
	AvgRiskScore     float64            `json:"avg_risk_score"`
	HighRiskCount    int                `json:"high_risk_count"`
	TopUsers         []UserStats        `json:"top_users"`
	TopResources     []ResourceStats    `json:"top_resources"`
	TopSourceIPs     []IPStats          `json:"top_source_ips"`
	Anomalies        []AnomalyDetection `json:"anomalies"`
	RiskDistribution map[RiskLevel]int  `json:"risk_distribution"`
	HourlyActivity   []HourlyActivity   `json:"hourly_activity"`
}

// UserStats 用户统计.
type UserStats struct {
	UserID       string  `json:"user_id"`
	UserName     string  `json:"user_name,omitempty"`
	AccessCount  int     `json:"access_count"`
	SuccessCount int     `json:"success_count"`
	DeniedCount  int     `json:"denied_count"`
	AvgRiskScore float64 `json:"avg_risk_score"`
}

// ResourceStats 资源统计.
type ResourceStats struct {
	Resource     string  `json:"resource"`
	AccessCount  int     `json:"access_count"`
	SuccessCount int     `json:"success_count"`
	DeniedCount  int     `json:"denied_count"`
	AvgRiskScore float64 `json:"avg_risk_score"`
}

// IPStats IP统计.
type IPStats struct {
	SourceIP     string  `json:"source_ip"`
	AccessCount  int     `json:"access_count"`
	SuccessCount int     `json:"success_count"`
	DeniedCount  int     `json:"denied_count"`
	UniqueUsers  int     `json:"unique_users"`
	AvgRiskScore float64 `json:"avg_risk_score"`
}

// HourlyActivity 每小时活动统计.
type HourlyActivity struct {
	Hour         int `json:"hour"`
	AccessCount  int `json:"access_count"`
	SuccessCount int `json:"success_count"`
	DeniedCount  int `json:"denied_count"`
}

// RiskScoreConfig 风险评分配置.
type RiskScoreConfig struct {
	// FailedAttemptWeight 失败尝试权重.
	FailedAttemptWeight float64 `json:"failed_attempt_weight"`
	// UnusualTimeWeight 非常规时间权重.
	UnusualTimeWeight float64 `json:"unusual_time_weight"`
	// UnusualIPWeight 异常IP权重.
	UnusualIPWeight float64 `json:"unusual_ip_weight"`
	// HighFreqWeight 高频访问权重.
	HighFreqWeight float64 `json:"high_freq_weight"`
	// SensitiveResourceWeight 敏感资源权重.
	SensitiveResourceWeight float64 `json:"sensitive_resource_weight"`
	// UnusualTimeStart 非常规时间开始（小时）.
	UnusualTimeStart int `json:"unusual_time_start"`
	// UnusualTimeEnd 非常规时间结束（小时）.
	UnusualTimeEnd int `json:"unusual_time_end"`
	// HighFreqThreshold 高频访问阈值（次/分钟）.
	HighFreqThreshold int `json:"high_freq_threshold"`
	// SensitiveResources 敏感资源列表.
	SensitiveResources []string `json:"sensitive_resources"`
}

// DefaultRiskScoreConfig 返回默认风险评分配置.
func DefaultRiskScoreConfig() *RiskScoreConfig {
	return &RiskScoreConfig{
		FailedAttemptWeight:     0.3,
		UnusualTimeWeight:       0.2,
		UnusualIPWeight:         0.15,
		HighFreqWeight:          0.2,
		SensitiveResourceWeight: 0.15,
		UnusualTimeStart:        0,
		UnusualTimeEnd:          6,
		HighFreqThreshold:       100,
		SensitiveResources: []string{
			"/api/admin", "/api/system", "/api/security",
			"/api/users/password", "/api/backup", "/api/restore",
		},
	}
}
