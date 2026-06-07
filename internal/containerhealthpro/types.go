package containerhealthpro

import (
	"sync"
	"time"
)

// HealthCheckType 健康检查类型
type HealthCheckType string

const (
	HealthCheckHTTP    HealthCheckType = "http"
	HealthCheckTCP     HealthCheckType = "tcp"
	HealthCheckCmd     HealthCheckType = "cmd"
	HealthCheckProcess HealthCheckType = "process"
)

// ContainerStatus 容器状态
type ContainerStatus string

const (
	StatusHealthy   ContainerStatus = "healthy"
	StatusUnhealthy ContainerStatus = "unhealthy"
	StatusStarting  ContainerStatus = "starting"
	StatusStopped   ContainerStatus = "stopped"
	StatusDegraded  ContainerStatus = "degraded"
)

// RecoveryPolicy 恢复策略
type RecoveryPolicy string

const (
	RecoveryRestart  RecoveryPolicy = "restart"
	RecoveryRedeploy RecoveryPolicy = "redeploy"
	RecoveryFailover RecoveryPolicy = "failover"
	RecoveryNone     RecoveryPolicy = "none"
)

// AlertSeverity 告警级别
type AlertSeverity string

const (
	AlertInfo     AlertSeverity = "info"
	AlertWarning  AlertSeverity = "warning"
	AlertCritical AlertSeverity = "critical"
)

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Type           HealthCheckType   `json:"type"`
	Endpoint       string            `json:"endpoint,omitempty"`
	Port           int               `json:"port,omitempty"`
	Command        string            `json:"command,omitempty"`
	ProcessName    string            `json:"process_name,omitempty"`
	Interval       int               `json:"interval"`        // 检查间隔（秒）
	Timeout        int               `json:"timeout"`         // 超时时间（秒）
	MaxRetries     int               `json:"max_retries"`     // 最大重试次数
	ExpectedStatus int               `json:"expected_status"` // HTTP期望状态码
	Headers        map[string]string `json:"headers,omitempty"`
}

// ResourceLimits 资源限制
type ResourceLimits struct {
	CPUPercent    float64 `json:"cpu_percent"`    // CPU使用率上限
	MemoryPercent float64 `json:"memory_percent"` // 内存使用率上限
	NetworkMbps   float64 `json:"network_mbps"`   // 网络带宽上限（Mbps）
	DiskIOMBps    float64 `json:"disk_io_mbps"`   // 磁盘IO上限（MB/s）
}

// ResourceUsage 资源使用情况
type ResourceUsage struct {
	CPUPercent     float64   `json:"cpu_percent"`
	MemoryUsed     int64     `json:"memory_used"`  // 字节
	MemoryTotal    int64     `json:"memory_total"` // 字节
	MemoryPercent  float64   `json:"memory_percent"`
	NetRxBytes     int64     `json:"net_rx_bytes"`
	NetTxBytes     int64     `json:"net_tx_bytes"`
	DiskReadBytes  int64     `json:"disk_read_bytes"`
	DiskWriteBytes int64     `json:"disk_write_bytes"`
	Timestamp      time.Time `json:"timestamp"`
}

// ContainerDependency 容器依赖关系
type ContainerDependency struct {
	ContainerID string   `json:"container_id"`
	DependsOn   []string `json:"depends_on"`  // 依赖的容器ID列表
	RequiredBy  []string `json:"required_by"` // 被依赖的容器ID列表
	StartOrder  int      `json:"start_order"` // 启动顺序
}

// LogEntry 日志条目
type LogEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Level       string    `json:"level"`
	Message     string    `json:"message"`
	ContainerID string    `json:"container_id"`
	Source      string    `json:"source"`
}

// LogPattern 日志异常模式
type LogPattern struct {
	Pattern     string        `json:"pattern"`
	Severity    AlertSeverity `json:"severity"`
	Count       int           `json:"count"`
	LastSeen    time.Time     `json:"last_seen"`
	Description string        `json:"description"`
}

// PerformanceBaseline 性能基线
type PerformanceBaseline struct {
	CPUPercentAvg    float64   `json:"cpu_percent_avg"`
	CPUPercentP95    float64   `json:"cpu_percent_p95"`
	MemoryPercentAvg float64   `json:"memory_percent_avg"`
	MemoryPercentP95 float64   `json:"memory_percent_p95"`
	NetworkMbpsAvg   float64   `json:"network_mbps_avg"`
	DiskIOMBpsAvg    float64   `json:"disk_io_mbps_avg"`
	SampleCount      int       `json:"sample_count"`
	LastUpdated      time.Time `json:"last_updated"`
}

// PerformanceDeviation 性能偏差
type PerformanceDeviation struct {
	Metric    string  `json:"metric"`
	Baseline  float64 `json:"baseline"`
	Current   float64 `json:"current"`
	Deviation float64 `json:"deviation"` // 偏差百分比
	Threshold float64 `json:"threshold"` // 阈值
	Alert     bool    `json:"alert"`
}

// SecurityScanResult 安全扫描结果
type SecurityScanResult struct {
	ContainerID     string          `json:"container_id"`
	ImageName       string          `json:"image_name"`
	ScanTime        time.Time       `json:"scan_time"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	Score           float64         `json:"score"`  // 0-100
	Status          string          `json:"status"` // pass/fail/warning
}

// Vulnerability 漏洞信息
type Vulnerability struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"` // critical/high/medium/low
	Package     string `json:"package"`
	Version     string `json:"version"`
	FixedIn     string `json:"fixed_in,omitempty"`
	Description string `json:"description"`
}

// HealthHistory 健康检查历史记录
type HealthHistory struct {
	Timestamp    time.Time       `json:"timestamp"`
	Status       ContainerStatus `json:"status"`
	ResponseTime time.Duration   `json:"response_time"`
	Error        string          `json:"error,omitempty"`
}

// HealthTrend 健康趋势
type HealthTrend struct {
	ContainerID     string    `json:"container_id"`
	Period          string    `json:"period"` // 1h/24h/7d/30d
	UptimePercent   float64   `json:"uptime_percent"`
	AvgResponseTime float64   `json:"avg_response_time_ms"`
	IncidentCount   int       `json:"incident_count"`
	LastIncident    time.Time `json:"last_incident,omitempty"`
}

// Alert 告警信息
type Alert struct {
	ID          string        `json:"id"`
	ContainerID string        `json:"container_id"`
	Severity    AlertSeverity `json:"severity"`
	Message     string        `json:"message"`
	Timestamp   time.Time     `json:"timestamp"`
	Resolved    bool          `json:"resolved"`
	ResolvedAt  *time.Time    `json:"resolved_at,omitempty"`
}

// ContainerHealthPro 增强版容器健康信息
type ContainerHealthPro struct {
	ContainerID    string                 `json:"container_id"`
	Name           string                 `json:"name"`
	Image          string                 `json:"image"`
	Status         ContainerStatus        `json:"status"`
	HealthCheck    HealthCheckConfig      `json:"health_check"`
	ResourceLimits ResourceLimits         `json:"resource_limits"`
	ResourceUsage  ResourceUsage          `json:"resource_usage"`
	Dependency     ContainerDependency    `json:"dependency"`
	Baseline       PerformanceBaseline    `json:"baseline"`
	Deviations     []PerformanceDeviation `json:"deviations,omitempty"`
	SecurityScan   *SecurityScanResult    `json:"security_scan,omitempty"`
	Alerts         []Alert                `json:"alerts,omitempty"`
	History        []HealthHistory        `json:"history,omitempty"`
	LastCheck      time.Time              `json:"last_check"`
	LastHealthy    time.Time              `json:"last_healthy"`
	FailCount      int                    `json:"fail_count"`
	RestartCount   int                    `json:"restart_count"`
	AutoRestart    bool                   `json:"auto_restart"`
	RecoveryPolicy RecoveryPolicy         `json:"recovery_policy"`
	Uptime         time.Duration          `json:"uptime"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
}

// Manager 增强版容器健康管理器
type Manager struct {
	mu           sync.RWMutex
	containers   map[string]*ContainerHealthPro
	dependencies map[string]*ContainerDependency
	logPatterns  []LogPattern
	alerts       map[string][]Alert
	history      map[string][]HealthHistory
	maxHistory   int
	stopCh       chan struct{}
}

// NewManager 创建增强版容器健康管理器
func NewManager() *Manager {
	return &Manager{
		containers:   make(map[string]*ContainerHealthPro),
		dependencies: make(map[string]*ContainerDependency),
		logPatterns:  make([]LogPattern, 0),
		alerts:       make(map[string][]Alert),
		history:      make(map[string][]HealthHistory),
		maxHistory:   1000,
		stopCh:       make(chan struct{}),
	}
}
