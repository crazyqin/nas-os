// Package syslogserver 提供日志集中管理服务
package syslogserver

import (
	"net"
	"time"
)

// SyslogSeverity syslog 严重级别 (RFC 5424).
type SyslogSeverity int

const (
	SeverityEmergency     SyslogSeverity = 0 // Emergency: system is unusable
	SeverityAlert         SyslogSeverity = 1 // Alert: action must be taken immediately
	SeverityCritical      SyslogSeverity = 2 // Critical: critical conditions
	SeverityError         SyslogSeverity = 3 // Error: error conditions
	SeverityWarning       SyslogSeverity = 4 // Warning: warning conditions
	SeverityNotice        SyslogSeverity = 5 // Notice: normal but significant condition
	SeverityInformational SyslogSeverity = 6 // Informational: informational messages
	SeverityDebug         SyslogSeverity = 7 // Debug: debug-level messages
)

// SeverityNames 严重级别名称映射.
var SeverityNames = map[SyslogSeverity]string{
	SeverityEmergency:     "emergency",
	SeverityAlert:         "alert",
	SeverityCritical:      "critical",
	SeverityError:         "error",
	SeverityWarning:       "warning",
	SeverityNotice:        "notice",
	SeverityInformational: "informational",
	SeverityDebug:         "debug",
}

// SyslogFacility syslog 设施 (RFC 5424).
type SyslogFacility int

const (
	FacilityKern     SyslogFacility = 0  // kernel messages
	FacilityUser     SyslogFacility = 1  // user-level messages
	FacilityMail     SyslogFacility = 2  // mail system
	FacilityDaemon   SyslogFacility = 3  // system daemons
	FacilityAuth     SyslogFacility = 4  // security/authorization messages
	FacilitySyslog   SyslogFacility = 5  // messages generated internally by syslogd
	FacilityLPR      SyslogFacility = 6  // line printer subsystem
	FacilityNews     SyslogFacility = 7  // network news subsystem
	FacilityUUCP     SyslogFacility = 8  // UUCP subsystem
	FacilityCron     SyslogFacility = 9  // clock daemon
	FacilityAuthPriv SyslogFacility = 10 // security/authorization messages
	FacilityFTP      SyslogFacility = 11 // FTP daemon
	FacilityNTP      SyslogFacility = 12 // NTP subsystem
	FacilityLogAudit SyslogFacility = 13 // log audit
	FacilityLogAlert SyslogFacility = 14 // log alert
	FacilityClock    SyslogFacility = 15 // clock daemon (note 2)
	FacilityLocal0   SyslogFacility = 16 // local use 0
	FacilityLocal1   SyslogFacility = 17 // local use 1
	FacilityLocal2   SyslogFacility = 18 // local use 2
	FacilityLocal3   SyslogFacility = 19 // local use 3
	FacilityLocal4   SyslogFacility = 20 // local use 4
	FacilityLocal5   SyslogFacility = 21 // local use 5
	FacilityLocal6   SyslogFacility = 22 // local use 6
	FacilityLocal7   SyslogFacility = 23 // local use 7
)

// FacilityNames 设施名称映射.
var FacilityNames = map[SyslogFacility]string{
	FacilityKern:     "kern",
	FacilityUser:     "user",
	FacilityMail:     "mail",
	FacilityDaemon:   "daemon",
	FacilityAuth:     "auth",
	FacilitySyslog:   "syslog",
	FacilityLPR:      "lpr",
	FacilityNews:     "news",
	FacilityUUCP:     "uucp",
	FacilityCron:     "cron",
	FacilityAuthPriv: "authpriv",
	FacilityFTP:      "ftp",
	FacilityNTP:      "ntp",
	FacilityLogAudit: "logaudit",
	FacilityLogAlert: "logalert",
	FacilityClock:    "clock",
	FacilityLocal0:   "local0",
	FacilityLocal1:   "local1",
	FacilityLocal2:   "local2",
	FacilityLocal3:   "local3",
	FacilityLocal4:   "local4",
	FacilityLocal5:   "local5",
	FacilityLocal6:   "local6",
	FacilityLocal7:   "local7",
}

// SyslogEntry syslog 日志条目.
type SyslogEntry struct {
	ID        string         `json:"id"`
	Priority  int            `json:"priority"`           // priority = facility * 8 + severity
	Facility  SyslogFacility `json:"facility"`
	Severity  SyslogSeverity `json:"severity"`
	Timestamp time.Time      `json:"timestamp"`
	Hostname  string         `json:"hostname"`
	AppName   string         `json:"app_name"`
	ProcID   string         `json:"proc_id,omitempty"`
	MsgID     string         `json:"msg_id,omitempty"`
	Message   string         `json:"message"`
	StructuredData string    `json:"structured_data,omitempty"`
	Raw       string         `json:"raw"`                // 原始日志行
	SourceIP  string         `json:"source_ip"`          // 来源 IP
	Protocol  string         `json:"protocol"`           // tcp/udp
	Tags      []string       `json:"tags,omitempty"`     // 自定义标签
	ReceivedAt time.Time     `json:"received_at"`        // 接收时间
}

// ForwardTarget 日志转发目标.
type ForwardTarget struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Protocol  string    `json:"protocol"` // tcp/udp
	Enabled   bool      `json:"enabled"`
	Filter    string    `json:"filter,omitempty"`    // 过滤条件（facility:severity）
	CreatedAt time.Time `json:"created_at"`
}

// AlertRule 告警规则.
type AlertRule struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Enabled     bool      `json:"enabled"`
	Type        string    `json:"type"`        // keyword / frequency
	Keyword     string    `json:"keyword"`     // 关键词（keyword 类型）
	Facility    string    `json:"facility"`    // 设施过滤
	Severity    string    `json:"severity"`    // 最低严重级别
	Frequency   int       `json:"frequency"`   // 频率阈值（次/分钟）
	WindowSec   int       `json:"window_sec"`  // 时间窗口（秒）
	NotifyType  string    `json:"notify_type"` // log / webhook
	WebhookURL  string    `json:"webhook_url,omitempty"`
	LastTrigger *time.Time `json:"last_trigger,omitempty"`
	TriggerCount int64     `json:"trigger_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// ArchivePolicy 日志归档策略.
type ArchivePolicy struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Enabled      bool      `json:"enabled"`
	MaxAgeDays   int       `json:"max_age_days"`   // 最大保留天数
	MaxSizeMB    int       `json:"max_size_mb"`    // 最大存储大小（MB）
	CompressOld  bool      `json:"compress_old"`   // 是否压缩旧日志
	CreatedAt    time.Time `json:"created_at"`
}

// ========== 请求/响应结构 ==========

// SearchRequest 日志搜索请求.
type SearchRequest struct {
	Query     string    `form:"q"`
	Hostname  string    `form:"hostname"`
	AppName   string    `form:"app_name"`
	Facility  string    `form:"facility"`
	Severity  string    `form:"severity"`
	SourceIP  string    `form:"source_ip"`
	StartTime *time.Time `form:"start_time"`
	EndTime   *time.Time `form:"end_time"`
	Tags      []string  `form:"tags"`
	Page      int       `form:"page,default=1"`
	PageSize  int       `form:"page_size,default=100"`
	SortBy    string    `form:"sort_by,default=timestamp"`
	SortOrder string    `form:"sort_order,default=desc"`
}

// SearchResponse 日志搜索响应.
type SearchResponse struct {
	Total   int           `json:"total"`
	Page    int           `json:"page"`
	Size    int           `json:"page_size"`
	Entries []*SyslogEntry `json:"entries"`
}

// CreateForwardTargetRequest 创建转发目标请求.
type CreateForwardTargetRequest struct {
	Name     string `json:"name" binding:"required"`
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port" binding:"required,min=1,max=65535"`
	Protocol string `json:"protocol" binding:"required,oneof=tcp udp"`
	Enabled  bool   `json:"enabled"`
	Filter   string `json:"filter,omitempty"`
}

// UpdateForwardTargetRequest 更新转发目标请求.
type UpdateForwardTargetRequest struct {
	Name     *string `json:"name,omitempty"`
	Host     *string `json:"host,omitempty"`
	Port     *int    `json:"port,omitempty"`
	Protocol *string `json:"protocol,omitempty"`
	Enabled  *bool   `json:"enabled,omitempty"`
	Filter   *string `json:"filter,omitempty"`
}

// CreateAlertRuleRequest 创建告警规则请求.
type CreateAlertRuleRequest struct {
	Name       string `json:"name" binding:"required"`
	Enabled    bool   `json:"enabled"`
	Type       string `json:"type" binding:"required,oneof=keyword frequency"`
	Keyword    string `json:"keyword,omitempty"`
	Facility   string `json:"facility,omitempty"`
	Severity   string `json:"severity,omitempty"`
	Frequency  int    `json:"frequency,omitempty"`
	WindowSec  int    `json:"window_sec,omitempty"`
	NotifyType string `json:"notify_type" binding:"required,oneof=log webhook"`
	WebhookURL string `json:"webhook_url,omitempty"`
}

// UpdateAlertRuleRequest 更新告警规则请求.
type UpdateAlertRuleRequest struct {
	Name       *string `json:"name,omitempty"`
	Enabled    *bool   `json:"enabled,omitempty"`
	Keyword    *string `json:"keyword,omitempty"`
	Facility   *string `json:"facility,omitempty"`
	Severity   *string `json:"severity,omitempty"`
	Frequency  *int    `json:"frequency,omitempty"`
	WindowSec  *int    `json:"window_sec,omitempty"`
	NotifyType *string `json:"notify_type,omitempty"`
	WebhookURL *string `json:"webhook_url,omitempty"`
}

// ExportRequest 日志导出请求.
type ExportRequest struct {
	Format    string    `json:"format" binding:"required,oneof=csv json"`
	Query     string    `json:"query,omitempty"`
	Hostname  string    `json:"hostname,omitempty"`
	Facility  string    `json:"facility,omitempty"`
	Severity  string    `json:"severity,omitempty"`
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Limit     int       `json:"limit,omitempty"`
}

// DashboardStats 仪表板统计数据.
type DashboardStats struct {
	TotalEntries   int64                  `json:"total_entries"`
	EntriesToday   int64                  `json:"entries_today"`
	EntriesPerHour []HourlyCount          `json:"entries_per_hour"`
	BySeverity     map[string]int64       `json:"by_severity"`
	ByFacility     map[string]int64       `json:"by_facility"`
	ByHost         map[string]int64       `json:"by_host"`
	ByApp          map[string]int64       `json:"by_app"`
	TopSources     []SourceCount          `json:"top_sources"`
	RecentAlerts   []*AlertEvent          `json:"recent_alerts"`
}

// HourlyCount 每小时计数.
type HourlyCount struct {
	Hour  string `json:"hour"`
	Count int64  `json:"count"`
}

// SourceCount 来源计数.
type SourceCount struct {
	Source string `json:"source"`
	Count  int64  `json:"count"`
}

// AlertEvent 告警事件.
type AlertEvent struct {
	ID        string    `json:"id"`
	RuleID    string    `json:"rule_id"`
	RuleName  string    `json:"rule_name"`
	Message   string    `json:"message"`
	Entry     *SyslogEntry `json:"entry,omitempty"`
	TriggeredAt time.Time `json:"triggered_at"`
}

// WSClient WebSocket 客户端.
type WSClient struct {
	ID      string
	Conn    net.Conn
	Filter  *SearchRequest
	Send    chan []byte
}
