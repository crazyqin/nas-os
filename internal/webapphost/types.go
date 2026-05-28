// Package webapphost - Web 应用托管平台
// 参考群晖 Web Station 和飞牛应用中心，实现 NAS 上的 Web 应用一键部署
package webapphost

import (
	"fmt"
	"sync"
	"time"
)

// WebApp Web 应用
type WebApp struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Type        string            `json:"type"` // docker, static, proxy
	Status      string            `json:"status"` // stopped, starting, running, stopping, error
	Domain      string            `json:"domain,omitempty"`
	Path        string            `json:"path"`
	Port        int               `json:"port"`
	SSLEnabled  bool              `json:"ssl_enabled"`
	TemplateID  string            `json:"template_id,omitempty"`
	Image       string            `json:"image,omitempty"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	Volumes     []VolumeMount     `json:"volumes,omitempty"`
	Ports       []PortMapping     `json:"ports,omitempty"`
	Resources   ResourceLimit     `json:"resources"`
	Tags        []string          `json:"tags,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Config      map[string]string `json:"config,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
}

// VolumeMount 卷挂载
type VolumeMount struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only"`
}

// PortMapping 端口映射
type PortMapping struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"` // tcp, udp
}

// ResourceLimit 资源限制
type ResourceLimit struct {
	CPUShares  int64  `json:"cpu_shares"`  // CPU 份额
	MemoryMB   int64  `json:"memory_mb"`   // 内存限制(MB)
	MemorySwap int64  `json:"memory_swap"` // 交换内存限制(MB)
	CPUQuota   int64  `json:"cpu_quota"`   // CPU 配额(微秒)
	PidsLimit  int64  `json:"pids_limit"`  // 进程数限制
	IOWeight   int    `json:"io_weight"`   // IO 权重
}

// AppTemplate 应用模板
type AppTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name"`
	Description string            `json:"description"`
	Category    string            `json:"category"` // web, database, media, devops, productivity
	Icon        string            `json:"icon"`
	Version     string            `json:"version"`
	Author      string            `json:"author"`
	Type        string            `json:"type"` // docker, static, proxy
	Image       string            `json:"image,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	EnvVars     []EnvVarDef       `json:"env_vars,omitempty"`
	Volumes     []VolumeDef       `json:"volumes,omitempty"`
	Ports       []PortDef         `json:"ports,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	MinMemory   int64             `json:"min_memory_mb"`
	MinCPU      float64           `json:"min_cpu"`
	MinDisk     int64             `json:"min_disk_mb"`
	HealthCheck *HealthCheckDef   `json:"health_check,omitempty"`
	DependsOn   []string          `json:"depends_on,omitempty"`
	Rating      float64           `json:"rating"`
	Downloads   int64             `json:"downloads"`
	Official    bool              `json:"official"`
	Featured    bool              `json:"featured"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// EnvVarDef 环境变量定义
type EnvVarDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Default     string `json:"default,omitempty"`
	Required    bool   `json:"required"`
	Secret      bool   `json:"secret"`
	Type        string `json:"type"` // string, number, boolean, select
	Options     []string `json:"options,omitempty"`
}

// VolumeDef 卷定义
type VolumeDef struct {
	Name          string `json:"name"`
	ContainerPath string `json:"container_path"`
	Description   string `json:"description"`
	Required      bool   `json:"required"`
	DefaultHost   string `json:"default_host,omitempty"`
	ReadOnly      bool   `json:"read_only"`
}

// PortDef 端口定义
type PortDef struct {
	Name          string `json:"name"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
	Description   string `json:"description"`
	Required      bool   `json:"required"`
}

// HealthCheckDef 健康检查定义
type HealthCheckDef struct {
	Type        string        `json:"type"` // http, tcp, cmd
	Path        string        `json:"path,omitempty"`
	Port        int           `json:"port,omitempty"`
	Command     string        `json:"command,omitempty"`
	Interval    time.Duration `json:"interval"`
	Timeout     time.Duration `json:"timeout"`
	Retries     int           `json:"retries"`
	StartPeriod time.Duration `json:"start_period"`
}

// DeployConfig 部署配置
type DeployConfig struct {
	AppName     string            `json:"app_name"`
	TemplateID  string            `json:"template_id,omitempty"`
	Type        string            `json:"type"` // docker, static, proxy
	Image       string            `json:"image,omitempty"`
	Version     string            `json:"version"`
	Domain      string            `json:"domain,omitempty"`
	Path        string            `json:"path"`
	Port        int               `json:"port"`
	SSLEnabled  bool              `json:"ssl_enabled"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	Volumes     []VolumeMount     `json:"volumes,omitempty"`
	Ports       []PortMapping     `json:"ports,omitempty"`
	Resources   ResourceLimit     `json:"resources"`
	Labels      map[string]string `json:"labels,omitempty"`
	Config      map[string]string `json:"config,omitempty"`
	SourcePath  string            `json:"source_path,omitempty"` // 静态文件路径
	TargetURL   string            `json:"target_url,omitempty"` // 反向代理目标
	AutoStart   bool              `json:"auto_start"`
	RestartPolicy string          `json:"restart_policy"` // always, on-failure, unless-stopped, no
}

// AppMetrics 应用指标
type AppMetrics struct {
	AppID       string    `json:"app_id"`
	CPUUsage    float64   `json:"cpu_usage"`    // CPU 使用率(%)
	MemoryUsage int64     `json:"memory_usage"` // 内存使用(bytes)
	MemoryLimit int64     `json:"memory_limit"` // 内存限制(bytes)
	DiskUsage   int64     `json:"disk_usage"`   // 磁盘使用(bytes)
	NetworkRx   int64     `json:"network_rx"`   // 网络接收(bytes)
	NetworkTx   int64   `json:"network_tx"`   // 网络发送(bytes)
	Uptime      int64     `json:"uptime"`       // 运行时间(秒)
	RequestCount int64    `json:"request_count"` // 请求总数
	ErrorCount  int64     `json:"error_count"`  // 错误数
	AvgResponse float64   `json:"avg_response"` // 平均响应时间(ms)
	Timestamp   time.Time `json:"timestamp"`
}

// AlertRule 告警规则
type AlertRule struct {
	ID          string        `json:"id"`
	AppID       string        `json:"app_id"`
	Name        string        `json:"name"`
	Type        string        `json:"type"` // cpu, memory, disk, error_rate, response_time
	Threshold   float64       `json:"threshold"`
	Duration    time.Duration `json:"duration"`
	Enabled     bool          `json:"enabled"`
	Notify      []string      `json:"notify"` // email, webhook, etc
	LastTrigger *time.Time    `json:"last_trigger,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
}

// DomainConfig 域名配置
type DomainConfig struct {
	Domain      string    `json:"domain"`
	AppID       string    `json:"app_id"`
	SSLEnabled  bool      `json:"ssl_enabled"`
	CertID      string    `json:"cert_id,omitempty"`
	RedirectHTTPS bool    `json:"redirect_https"`
	Headers     map[string]string `json:"headers,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RouteRule 路由规则
type RouteRule struct {
	ID        string    `json:"id"`
	Domain    string    `json:"domain"`
	Path      string    `json:"path"`
	AppID     string    `json:"app_id"`
	Priority  int       `json:"priority"`
	StripPath bool      `json:"strip_path"`
	Headers   map[string]string `json:"headers,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// SSLEntry SSL 证书条目
type SSLEntry struct {
	ID          string    `json:"id"`
	Domain      string    `json:"domain"`
	CertPath    string    `json:"cert_path"`
	KeyPath     string    `json:"key_path"`
	Issuer      string    `json:"issuer"`
	NotBefore   time.Time `json:"not_before"`
	NotAfter    time.Time `json:"not_after"`
	AutoRenew   bool      `json:"auto_renew"`
	Provider    string    `json:"provider"` // letsencrypt, selfsigned, custom
	Status      string    `json:"status"` // active, expired, pending, error
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MarketApp 市场应用
type MarketApp struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Icon        string   `json:"icon"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Type        string   `json:"type"`
	Tags        []string `json:"tags,omitempty"`
	Rating      float64  `json:"rating"`
	Downloads   int64    `json:"downloads"`
	Official    bool     `json:"official"`
	Featured    bool     `json:"featured"`
	Screenshots []string `json:"screenshots,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	License     string   `json:"license,omitempty"`
	SizeMB      int64    `json:"size_mb"`
	Installed   bool     `json:"installed"`
	UpdateAvail bool     `json:"update_available"`
}

// MarketCategory 市场分类
type MarketCategory struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	AppCount    int    `json:"app_count"`
}

// WebAppManager Web 应用管理器
type WebAppManager struct {
	mu          sync.RWMutex
	apps        map[string]*WebApp
	templates   map[string]*AppTemplate
	domains     map[string]*DomainConfig
	routes      map[string]*RouteRule
	sslEntries  map[string]*SSLEntry
	alerts      map[string]*AlertRule
	metrics     map[string]*AppMetrics
	config      *ManagerConfig
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	DataDir         string        `json:"data_dir"`
	DockerSocket    string        `json:"docker_socket"`
	SSLCertDir      string        `json:"ssl_cert_dir"`
	MaxApps         int           `json:"max_apps"`
	DefaultDomain   string        `json:"default_domain"`
	EnableSSL       bool          `json:"enable_ssl"`
	EnableMonitor   bool          `json:"enable_monitor"`
	MonitorInterval time.Duration `json:"monitor_interval"`
	BackupDir       string        `json:"backup_dir"`
}

// NewWebAppManager 创建 Web 应用管理器
func NewWebAppManager(config *ManagerConfig) *WebAppManager {
	if config == nil {
		config = &ManagerConfig{
			DataDir:         "/var/lib/nas-os/webapphost",
			DockerSocket:    "/var/run/docker.sock",
			SSLCertDir:      "/etc/nas-os/ssl",
			MaxApps:         100,
			EnableSSL:       true,
			EnableMonitor:   true,
			MonitorInterval: 30 * time.Second,
		}
	}
	return &WebAppManager{
		apps:       make(map[string]*WebApp),
		templates:  make(map[string]*AppTemplate),
		domains:    make(map[string]*DomainConfig),
		routes:     make(map[string]*RouteRule),
		sslEntries: make(map[string]*SSLEntry),
		alerts:     make(map[string]*AlertRule),
		metrics:    make(map[string]*AppMetrics),
		config:     config,
	}
}

// GenerateID 生成唯一 ID
func GenerateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
