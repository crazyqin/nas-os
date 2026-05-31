package truecommand

import "time"

// TrueCommandConfig TrueCommand 配置.
type TrueCommandConfig struct {
	APIEndpoint   string        `json:"api_endpoint"`
	APIKey        string        `json:"-"` // 不序列化
	PollInterval  time.Duration `json:"poll_interval"`
	AlertLimit    int           `json:"alert_limit"`
	MaxSystems    int           `json:"max_systems"`
	EnableMetrics bool          `json:"enable_metrics"`
	EnableAlerts  bool          `json:"enable_alerts"`
	RetentionDays int           `json:"retention_days"`
}

// NASSystem NAS 系统.
type NASSystem struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Host          string            `json:"host"`
	Port          int               `json:"port"`
	APIKey        string            `json:"-"` // 不序列化
	Version       string            `json:"version"`
	Status        SystemStatus      `json:"status"`
	ClusterID     string            `json:"cluster_id"`
	CPUUsage      float64           `json:"cpu_usage"`
	CPUCores      int               `json:"cpu_cores"`
	MemoryUsed    int64             `json:"memory_used"`
	MemoryTotal   int64             `json:"memory_total"`
	StorageUsed   int64             `json:"storage_used"`
	StorageTotal  int64             `json:"storage_total"`
	NetworkIn     int64             `json:"network_in"`
	NetworkOut    int64             `json:"network_out"`
	Uptime        time.Duration     `json:"uptime"`
	Hostname      string            `json:"hostname"`
	OS            string            `json:"os"`
	Platform      string            `json:"platform"`
	Attributes    map[string]string `json:"attributes"`
	LastSeen      time.Time         `json:"last_seen"`
	RegisteredAt  time.Time         `json:"registered_at"`
}

// SystemStatus 系统状态.
type SystemStatus string

const (
	SystemStatusOnline  SystemStatus = "online"
	SystemStatusOffline SystemStatus = "offline"
	SystemStatusDegraded SystemStatus = "degraded"
	SystemStatusMaintenance SystemStatus = "maintenance"
)

// Cluster 集群.
type Cluster struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Type        ClusterType   `json:"type"`
	Status      ClusterStatus `json:"status"`
	Members     []string      `json:"members"`
	VirtualIP   string        `json:"virtual_ip"`
	SharedStorage string      `json:"shared_storage"`
	CreatedAt   time.Time     `json:"created_at"`
}

// ClusterType 集群类型.
type ClusterType string

const (
	ClusterTypeHA      ClusterType = "ha"
	ClusterTypeScale   ClusterType = "scale"
	ClusterTypeFederated ClusterType = "federated"
)

// ClusterStatus 集群状态.
type ClusterStatus string

const (
	ClusterStatusActive    ClusterStatus = "active"
	ClusterStatusInactive  ClusterStatus = "inactive"
	ClusterStatusDegraded  ClusterStatus = "degraded"
	ClusterStatusFailed    ClusterStatus = "failed"
)

// Alert 告警.
type Alert struct {
	ID           string    `json:"id"`
	SystemID     string    `json:"system_id"`
	Type         string    `json:"type"`
	Message      string    `json:"message"`
	Severity     string    `json:"severity"`
	Acknowledged bool      `json:"acknowledged"`
	AckedAt      time.Time `json:"acked_at"`
	Timestamp    time.Time `json:"timestamp"`
}

// Dashboard 仪表板.
type Dashboard struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Widgets   []Widget     `json:"widgets"`
	Layout    string       `json:"layout"`
	Default   bool         `json:"default"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// Widget 仪表板组件.
type Widget struct {
	ID       string            `json:"id"`
	Type     WidgetType        `json:"type"`
	Title    string            `json:"title"`
	SystemID string            `json:"system_id"`
	Config   map[string]string `json:"config"`
	Position Position          `json:"position"`
}

// WidgetType 组件类型.
type WidgetType string

const (
	WidgetTypeCPU     WidgetType = "cpu"
	WidgetTypeMemory  WidgetType = "memory"
	WidgetTypeStorage WidgetType = "storage"
	WidgetTypeNetwork WidgetType = "network"
	WidgetTypeAlerts  WidgetType = "alerts"
	WidgetTypeStatus  WidgetType = "status"
	WidgetTypeChart   WidgetType = "chart"
)

// Position 组件位置.
type Position struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// TrueCommandStats 统计信息.
type TrueCommandStats struct {
	TotalSystems    int     `json:"total_systems"`
	OnlineSystems   int     `json:"online_systems"`
	TotalClusters   int     `json:"total_clusters"`
	TotalAlerts     int     `json:"total_alerts"`
	UnackedAlerts   int     `json:"unacked_alerts"`
	TotalDashboards int     `json:"total_dashboards"`
	AvgCPUUsage     float64 `json:"avg_cpu_usage"`
	TotalMemory     int64   `json:"total_memory"`
	TotalStorage    int64   `json:"total_storage"`
}

// ReplicationJob 复制任务.
type ReplicationJob struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"source_id"`
	TargetID    string    `json:"target_id"`
	Dataset     string    `json:"dataset"`
	Status      string    `json:"status"`
	Progress    float64   `json:"progress"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}
