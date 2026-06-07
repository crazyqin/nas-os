// Package aiops 提供 AIOps 智能运维中心功能，包括自动故障诊断、告警聚合、自动修复、SLA监控等。
package aiops

import "time"

// Severity 告警严重级别
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// IncidentStatus 事件状态
type IncidentStatus string

const (
	StatusOpen          IncidentStatus = "open"
	StatusInvestigating IncidentStatus = "investigating"
	StatusMitigated     IncidentStatus = "mitigated"
	StatusResolved      IncidentStatus = "resolved"
	StatusClosed        IncidentStatus = "closed"
)

// AlertStatus 告警状态
type AlertStatus string

const (
	AlertStatusFiring     AlertStatus = "firing"
	AlertStatusResolved   AlertStatus = "resolved"
	AlertStatusSuppressed AlertStatus = "suppressed"
)

// RemediationStatus 修复状态
type RemediationStatus string

const (
	RemediationStatusPending RemediationStatus = "pending"
	RemediationStatusRunning RemediationStatus = "running"
	RemediationStatusSuccess RemediationStatus = "success"
	RemediationStatusFailed  RemediationStatus = "failed"
	RemediationStatusSkipped RemediationStatus = "skipped"
)

// Alert 原始告警
type Alert struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Source      string            `json:"source"`
	Severity    Severity          `json:"severity"`
	Status      AlertStatus       `json:"status"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Value       float64           `json:"value,omitempty"`
	Threshold   float64           `json:"threshold,omitempty"`
	Message     string            `json:"message,omitempty"`
	StartsAt    time.Time         `json:"starts_at"`
	EndsAt      *time.Time        `json:"ends_at,omitempty"`
}

// AlertGroup 告警聚合组
type AlertGroup struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Severity   Severity          `json:"severity"`
	Status     AlertStatus       `json:"status"`
	Alerts     []Alert           `json:"alerts"`
	AlertCount int               `json:"alert_count"`
	RootCause  string            `json:"root_cause,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	FirstSeen  time.Time         `json:"first_seen"`
	LastSeen   time.Time         `json:"last_seen"`
	ResolvedAt *time.Time        `json:"resolved_at,omitempty"`
}

// Incident 事件（故障工单）
type Incident struct {
	ID              string              `json:"id"`
	Title           string              `json:"title"`
	Description     string              `json:"description,omitempty"`
	Severity        Severity            `json:"severity"`
	Status          IncidentStatus      `json:"status"`
	AffectedService string              `json:"affected_service,omitempty"`
	RootCause       string              `json:"root_cause,omitempty"`
	AlertGroupID    string              `json:"alert_group_id,omitempty"`
	Remediations    []RemediationAction `json:"remediations,omitempty"`
	Diagnosis       *DiagnosisResult    `json:"diagnosis,omitempty"`
	Tags            []string            `json:"tags,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
	ResolvedAt      *time.Time          `json:"resolved_at,omitempty"`
}

// DiagnosisResult 诊断结果
type DiagnosisResult struct {
	ID                 string             `json:"id"`
	IncidentID         string             `json:"incident_id"`
	RootCause          string             `json:"root_cause"`
	Confidence         float64            `json:"confidence"`
	AffectedComponents []string           `json:"affected_components,omitempty"`
	SuggestedActions   []string           `json:"suggested_actions,omitempty"`
	Metrics            map[string]float64 `json:"metrics,omitempty"`
	Timeline           []TimelineEvent    `json:"timeline,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
}

// TimelineEvent 时间线事件
type TimelineEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Description string    `json:"description"`
	Severity    Severity  `json:"severity"`
}

// RemediationAction 修复动作
type RemediationAction struct {
	ID          string            `json:"id"`
	IncidentID  string            `json:"incident_id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Type        string            `json:"type"`
	Target      string            `json:"target"`
	Parameters  map[string]string `json:"parameters,omitempty"`
	Status      RemediationStatus `json:"status"`
	Result      string            `json:"result,omitempty"`
	Error       string            `json:"error,omitempty"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// SLATarget SLA 目标
type SLATarget struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Service           string    `json:"service"`
	TargetUptime      float64   `json:"target_uptime"`      // 百分比, 99.9
	TargetLatency     float64   `json:"target_latency"`     // 毫秒
	MeasurementPeriod string    `json:"measurement_period"` // "daily", "weekly", "monthly"
	CurrentUptime     float64   `json:"current_uptime"`
	CurrentLatency    float64   `json:"current_latency"`
	Status            string    `json:"status"` // "healthy", "at_risk", "breached"
	Breaches          int       `json:"breaches"`
	LastChecked       time.Time `json:"last_checked"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// OpsStats 运维统计
type OpsStats struct {
	TotalIncidents     int         `json:"total_incidents"`
	OpenIncidents      int         `json:"open_incidents"`
	ResolvedIncidents  int         `json:"resolved_incidents"`
	TotalAlerts        int         `json:"total_alerts"`
	ActiveAlerts       int         `json:"active_alerts"`
	SuppressedAlerts   int         `json:"suppressed_alerts"`
	TotalRemediations  int         `json:"total_remediations"`
	AutoFixedCount     int         `json:"auto_fixed_count"`
	MTTR               float64     `json:"mttr"`                 // 平均修复时间（分钟）
	AlertReductionRate float64     `json:"alert_reduction_rate"` // 告警压缩率
	Availability       float64     `json:"availability"`         // 可用性百分比
	SLATargets         []SLATarget `json:"sla_targets,omitempty"`
	RecentIncidents    []Incident  `json:"recent_incidents,omitempty"`
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	CPUUsage    float64    `json:"cpu_usage"`
	MemoryUsage float64    `json:"memory_usage"`
	DiskUsage   float64    `json:"disk_usage"`
	NetworkIn   float64    `json:"network_in"`
	NetworkOut  float64    `json:"network_out"`
	DiskIOPS    float64    `json:"disk_iops"`
	LoadAverage [3]float64 `json:"load_average"`
	Timestamp   time.Time  `json:"timestamp"`
}

// Anomaly 异常检测结果
type Anomaly struct {
	Metric    string    `json:"metric"`
	Value     float64   `json:"value"`
	Expected  float64   `json:"expected"`
	Deviation float64   `json:"deviation"`
	Severity  Severity  `json:"severity"`
	Timestamp time.Time `json:"timestamp"`
}

// KnowledgeEntry 运维知识条目
type KnowledgeEntry struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	RootCause   string    `json:"root_cause"`
	Symptoms    []string  `json:"symptoms"`
	Solution    string    `json:"solution"`
	Tags        []string  `json:"tags,omitempty"`
	IncidentIDs []string  `json:"incident_ids,omitempty"`
	UsageCount  int       `json:"usage_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DiagnoseRequest 诊断请求
type DiagnoseRequest struct {
	Service  string         `json:"service,omitempty"`
	Metrics  *SystemMetrics `json:"metrics,omitempty"`
	AlertIDs []string       `json:"alert_ids,omitempty"`
}

// RemediateRequest 修复请求
type RemediateRequest struct {
	IncidentID string `json:"incident_id" binding:"required"`
	ActionType string `json:"action_type,omitempty"`
	AutoMode   bool   `json:"auto_mode"`
}

// AlertIngestRequest 告警接收请求
type AlertIngestRequest struct {
	Alerts []Alert `json:"alerts" binding:"required"`
}

// SLATargetRequest SLA 目标请求
type SLATargetRequest struct {
	Name              string  `json:"name" binding:"required"`
	Service           string  `json:"service" binding:"required"`
	TargetUptime      float64 `json:"target_uptime"`
	TargetLatency     float64 `json:"target_latency"`
	MeasurementPeriod string  `json:"measurement_period"`
}
