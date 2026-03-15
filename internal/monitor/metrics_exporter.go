// Package monitor 提供监控系统功能
// metrics_exporter.go - Prometheus 指标导出器
//
// v2.58.0 工部创建
//
// 功能:
//   - Prometheus 指标导出器
//   - 自定义指标支持
//   - 指标聚合
//   - 多命名空间支持
//   - 动态标签管理

package monitor

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsExporter Prometheus 指标导出器
type MetricsExporter struct {
	mu sync.RWMutex

	// 配置
	namespace string
	subsystem string
	port      int
	path      string

	// Prometheus 注册表
	registry *prometheus.Registry

	// 内置指标
	systemMetrics   *SystemMetrics
	storageMetrics  *StorageMetrics
	backupMetrics   *BackupMetrics
	serviceMetrics  *ServiceMetrics
	customMetrics   *CustomMetrics
	aggregatedStats *AggregatedStats

	// HTTP 服务器
	server *http.Server

	// 状态
	running bool
	stopCh  chan struct{}
}

// MetricsExporterConfig 导出器配置
type MetricsExporterConfig struct {
	Namespace string // 命名空间，默认 "nas_os"
	Subsystem string // 子系统名
	Port      int    // 监听端口，默认 9090
	Path      string // 指标路径，默认 "/metrics"
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	CPUUsage       prometheus.Gauge
	CPUIdle        prometheus.Gauge
	CPUUser        prometheus.Gauge
	CPUSystem      prometheus.Gauge
	CPUIOWait      prometheus.Gauge

	MemoryTotal    prometheus.Gauge
	MemoryUsed     prometheus.Gauge
	MemoryFree     prometheus.Gauge
	MemoryCached   prometheus.Gauge
	MemoryBuffers  prometheus.Gauge
	MemoryUsage    prometheus.Gauge

	SwapTotal      prometheus.Gauge
	SwapUsed       prometheus.Gauge
	SwapFree       prometheus.Gauge
	SwapUsage      prometheus.Gauge

	Load1          prometheus.Gauge
	Load5          prometheus.Gauge
	Load15         prometheus.Gauge

	Uptime         prometheus.Gauge
	ProcessCount   prometheus.Gauge
	ThreadCount    prometheus.Gauge
	FileDescriptor prometheus.Gauge
}

// StorageMetrics 存储指标
type StorageMetrics struct {
	DiskTotal          *prometheus.GaugeVec
	DiskUsed           *prometheus.GaugeVec
	DiskFree           *prometheus.GaugeVec
	DiskUsage          *prometheus.GaugeVec
	DiskReadBytes      *prometheus.GaugeVec
	DiskWriteBytes     *prometheus.GaugeVec
	DiskReadOps        *prometheus.GaugeVec
	DiskWriteOps       *prometheus.GaugeVec
	DiskReadLatency    *prometheus.GaugeVec
	DiskWriteLatency   *prometheus.GaugeVec

	VolumeTotal        *prometheus.GaugeVec
	VolumeUsed         *prometheus.GaugeVec
	VolumeFree         *prometheus.GaugeVec
	VolumeUsage        *prometheus.GaugeVec
	VolumeHealth       *prometheus.GaugeVec

	StoragePoolTotal   *prometheus.GaugeVec
	StoragePoolUsed    *prometheus.GaugeVec
	StoragePoolFree    *prometheus.GaugeVec
	StoragePoolHealth  *prometheus.GaugeVec

	QuotaUsed          *prometheus.GaugeVec
	QuotaLimit         *prometheus.GaugeVec
	QuotaUsage         *prometheus.GaugeVec
}

// BackupMetrics 备份指标
type BackupMetrics struct {
	BackupTotal       *prometheus.GaugeVec
	BackupSize        *prometheus.GaugeVec
	BackupDuration    *prometheus.GaugeVec
	BackupStatus      *prometheus.GaugeVec
	BackupLastRun     *prometheus.GaugeVec
	BackupLastError   *prometheus.GaugeVec
	BackupFiles       *prometheus.GaugeVec
	BackupSpeed       *prometheus.GaugeVec

	RestoreTotal      *prometheus.GaugeVec
	RestoreDuration   *prometheus.GaugeVec
	RestoreStatus     *prometheus.GaugeVec
}

// ServiceMetrics 服务指标
type ServiceMetrics struct {
	ServiceStatus     *prometheus.GaugeVec
	ServiceUptime     *prometheus.GaugeVec
	ServiceRestarts   *prometheus.GaugeVec
	ServiceConnections *prometheus.GaugeVec
	ServiceLatency    *prometheus.GaugeVec
	ServiceErrors     *prometheus.CounterVec

	APIRequests       *prometheus.CounterVec
	APILatency        *prometheus.HistogramVec
	APIInFlight       prometheus.Gauge
	APIErrors         *prometheus.CounterVec

	UserSessions      *prometheus.GaugeVec
	UserTotal         prometheus.Gauge
	UserActive        prometheus.Gauge
}

// CustomMetrics 自定义指标容器
type CustomMetrics struct {
	mu      sync.RWMutex
	gauges  map[string]*prometheus.GaugeVec
	counters map[string]*prometheus.CounterVec
	histograms map[string]*prometheus.HistogramVec
	summaries  map[string]*prometheus.SummaryVec
}

// AggregatedStats 聚合统计
type AggregatedStats struct {
	mu           sync.RWMutex
	lastUpdate   time.Time
	updatePeriod time.Duration

	// 聚合值
	avgCPUUsage    float64
	maxCPUUsage    float64
	avgMemoryUsage float64
	maxMemoryUsage float64
	avgDiskUsage   float64
	maxDiskUsage   float64
	totalRequests  int64
	totalErrors    int64
	dataPoints     int
}

// NewMetricsExporter 创建指标导出器
func NewMetricsExporter(config MetricsExporterConfig) *MetricsExporter {
	if config.Namespace == "" {
		config.Namespace = "nas_os"
	}
	if config.Port == 0 {
		config.Port = 9090
	}
	if config.Path == "" {
		config.Path = "/metrics"
	}

	registry := prometheus.NewRegistry()

	exporter := &MetricsExporter{
		namespace: config.Namespace,
		subsystem: config.Subsystem,
		port:      config.Port,
		path:      config.Path,
		registry:  registry,
		stopCh:    make(chan struct{}),
		customMetrics: &CustomMetrics{
			gauges:     make(map[string]*prometheus.GaugeVec),
			counters:   make(map[string]*prometheus.CounterVec),
			histograms: make(map[string]*prometheus.HistogramVec),
			summaries:  make(map[string]*prometheus.SummaryVec),
		},
		aggregatedStats: &AggregatedStats{
			updatePeriod: time.Minute,
		},
	}

	// 初始化所有指标
	exporter.initSystemMetrics()
	exporter.initStorageMetrics()
	exporter.initBackupMetrics()
	exporter.initServiceMetrics()

	// 注册到自定义注册表
	exporter.registerMetrics()

	return exporter
}

// initSystemMetrics 初始化系统指标
func (e *MetricsExporter) initSystemMetrics() {
	ns := e.namespace
	sub := e.subsystem

	e.systemMetrics = &SystemMetrics{
		CPUUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "cpu_usage_percent",
			Help: "Current CPU usage percentage",
		}),
		CPUIdle: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "cpu_idle_percent",
			Help: "CPU idle percentage",
		}),
		CPUUser: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "cpu_user_percent",
			Help: "CPU user mode percentage",
		}),
		CPUSystem: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "cpu_system_percent",
			Help: "CPU system mode percentage",
		}),
		CPUIOWait: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "cpu_iowait_percent",
			Help: "CPU I/O wait percentage",
		}),

		MemoryTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "memory_total_bytes",
			Help: "Total system memory in bytes",
		}),
		MemoryUsed: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "memory_used_bytes",
			Help: "Used system memory in bytes",
		}),
		MemoryFree: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "memory_free_bytes",
			Help: "Free system memory in bytes",
		}),
		MemoryCached: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "memory_cached_bytes",
			Help: "Cached memory in bytes",
		}),
		MemoryBuffers: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "memory_buffers_bytes",
			Help: "Buffer memory in bytes",
		}),
		MemoryUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "memory_usage_percent",
			Help: "Memory usage percentage",
		}),

		SwapTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "swap_total_bytes",
			Help: "Total swap space in bytes",
		}),
		SwapUsed: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "swap_used_bytes",
			Help: "Used swap space in bytes",
		}),
		SwapFree: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "swap_free_bytes",
			Help: "Free swap space in bytes",
		}),
		SwapUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "swap_usage_percent",
			Help: "Swap usage percentage",
		}),

		Load1: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "load_1m",
			Help: "System load average 1 minute",
		}),
		Load5: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "load_5m",
			Help: "System load average 5 minutes",
		}),
		Load15: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "load_15m",
			Help: "System load average 15 minutes",
		}),

		Uptime: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "uptime_seconds",
			Help: "System uptime in seconds",
		}),
		ProcessCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "process_count",
			Help: "Total number of processes",
		}),
		ThreadCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "thread_count",
			Help: "Total number of threads",
		}),
		FileDescriptor: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "file_descriptors",
			Help: "Open file descriptors count",
		}),
	}
}

// initStorageMetrics 初始化存储指标
func (e *MetricsExporter) initStorageMetrics() {
	ns := e.namespace
	sub := e.subsystem

	e.storageMetrics = &StorageMetrics{
		DiskTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "disk_total_bytes",
			Help: "Total disk space in bytes",
		}, []string{"device", "mountpoint"}),

		DiskUsed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "disk_used_bytes",
			Help: "Used disk space in bytes",
		}, []string{"device", "mountpoint"}),

		DiskFree: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "disk_free_bytes",
			Help: "Free disk space in bytes",
		}, []string{"device", "mountpoint"}),

		DiskUsage: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "disk_usage_percent",
			Help: "Disk usage percentage",
		}, []string{"device", "mountpoint"}),

		DiskReadBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "disk_read_bytes_total",
			Help: "Total bytes read from disk",
		}, []string{"device"}),

		DiskWriteBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "disk_write_bytes_total",
			Help: "Total bytes written to disk",
		}, []string{"device"}),

		DiskReadOps: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "disk_read_ops_total",
			Help: "Total disk read operations",
		}, []string{"device"}),

		DiskWriteOps: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "disk_write_ops_total",
			Help: "Total disk write operations",
		}, []string{"device"}),

		DiskReadLatency: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "disk_read_latency_ms",
			Help: "Disk read latency in milliseconds",
		}, []string{"device"}),

		DiskWriteLatency: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "disk_write_latency_ms",
			Help: "Disk write latency in milliseconds",
		}, []string{"device"}),

		VolumeTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "volume_total_bytes",
			Help: "Total volume space in bytes",
		}, []string{"volume", "type"}),

		VolumeUsed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "volume_used_bytes",
			Help: "Used volume space in bytes",
		}, []string{"volume", "type"}),

		VolumeFree: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "volume_free_bytes",
			Help: "Free volume space in bytes",
		}, []string{"volume", "type"}),

		VolumeUsage: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "volume_usage_percent",
			Help: "Volume usage percentage",
		}, []string{"volume", "type"}),

		VolumeHealth: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "volume_health",
			Help: "Volume health status (1=healthy, 0=unhealthy)",
		}, []string{"volume", "type"}),

		StoragePoolTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "storage_pool_total_bytes",
			Help: "Total storage pool space in bytes",
		}, []string{"pool"}),

		StoragePoolUsed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "storage_pool_used_bytes",
			Help: "Used storage pool space in bytes",
		}, []string{"pool"}),

		StoragePoolFree: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "storage_pool_free_bytes",
			Help: "Free storage pool space in bytes",
		}, []string{"pool"}),

		StoragePoolHealth: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "storage_pool_health",
			Help: "Storage pool health status (1=healthy, 0=unhealthy)",
		}, []string{"pool"}),

		QuotaUsed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "quota_used_bytes",
			Help: "Quota used in bytes",
		}, []string{"user", "volume"}),

		QuotaLimit: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "quota_limit_bytes",
			Help: "Quota limit in bytes",
		}, []string{"user", "volume"}),

		QuotaUsage: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "quota_usage_percent",
			Help: "Quota usage percentage",
		}, []string{"user", "volume"}),
	}
}

// initBackupMetrics 初始化备份指标
func (e *MetricsExporter) initBackupMetrics() {
	ns := e.namespace
	sub := e.subsystem

	e.backupMetrics = &BackupMetrics{
		BackupTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "backup_total",
			Help: "Total number of backups",
		}, []string{"type", "status"}),

		BackupSize: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "backup_size_bytes",
			Help: "Backup size in bytes",
		}, []string{"type", "id"}),

		BackupDuration: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "backup_duration_seconds",
			Help: "Backup duration in seconds",
		}, []string{"type", "id"}),

		BackupStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "backup_status",
			Help: "Backup status (1=success, 0=failed)",
		}, []string{"type", "id"}),

		BackupLastRun: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "backup_last_run_timestamp",
			Help: "Timestamp of last backup run",
		}, []string{"type"}),

		BackupLastError: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "backup_last_error_timestamp",
			Help: "Timestamp of last backup error",
		}, []string{"type"}),

		BackupFiles: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "backup_files_total",
			Help: "Total number of files in backup",
		}, []string{"type", "id"}),

		BackupSpeed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "backup_speed_bytes_per_second",
			Help: "Backup speed in bytes per second",
		}, []string{"type", "id"}),

		RestoreTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "restore_total",
			Help: "Total number of restores",
		}, []string{"status"}),

		RestoreDuration: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "restore_duration_seconds",
			Help: "Restore duration in seconds",
		}, []string{"id"}),

		RestoreStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "restore_status",
			Help: "Restore status (1=success, 0=failed)",
		}, []string{"id"}),
	}
}

// initServiceMetrics 初始化服务指标
func (e *MetricsExporter) initServiceMetrics() {
	ns := e.namespace
	sub := e.subsystem

	e.serviceMetrics = &ServiceMetrics{
		ServiceStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "service_status",
			Help: "Service status (1=running, 0=stopped)",
		}, []string{"service"}),

		ServiceUptime: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "service_uptime_seconds",
			Help: "Service uptime in seconds",
		}, []string{"service"}),

		ServiceRestarts: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "service_restarts_total",
			Help: "Total service restarts",
		}, []string{"service"}),

		ServiceConnections: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "service_connections",
			Help: "Active service connections",
		}, []string{"service"}),

		ServiceLatency: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "service_latency_ms",
			Help: "Service latency in milliseconds",
		}, []string{"service"}),

		ServiceErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub, Name: "service_errors_total",
			Help: "Total service errors",
		}, []string{"service", "type"}),

		APIRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub, Name: "api_requests_total",
			Help: "Total API requests",
		}, []string{"method", "endpoint", "status"}),

		APILatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns, Subsystem: sub, Name: "api_latency_seconds",
			Help:    "API request latency in seconds",
			Buckets: []float64{0.001, 0.01, 0.05, 0.1, 0.5, 1, 2, 5, 10},
		}, []string{"method", "endpoint"}),

		APIInFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "api_requests_in_flight",
			Help: "Current number of API requests being processed",
		}),

		APIErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns, Subsystem: sub, Name: "api_errors_total",
			Help: "Total API errors",
		}, []string{"method", "endpoint", "type"}),

		UserSessions: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "user_sessions",
			Help: "Active user sessions",
		}, []string{"user"}),

		UserTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "users_total",
			Help: "Total number of users",
		}),

		UserActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Subsystem: sub, Name: "users_active",
			Help: "Number of active users",
		}),
	}
}

// registerMetrics 注册所有指标到注册表
func (e *MetricsExporter) registerMetrics() {
	// 注册系统指标
	e.registry.MustRegister(
		e.systemMetrics.CPUUsage,
		e.systemMetrics.CPUIdle,
		e.systemMetrics.CPUUser,
		e.systemMetrics.CPUSystem,
		e.systemMetrics.CPUIOWait,
		e.systemMetrics.MemoryTotal,
		e.systemMetrics.MemoryUsed,
		e.systemMetrics.MemoryFree,
		e.systemMetrics.MemoryCached,
		e.systemMetrics.MemoryBuffers,
		e.systemMetrics.MemoryUsage,
		e.systemMetrics.SwapTotal,
		e.systemMetrics.SwapUsed,
		e.systemMetrics.SwapFree,
		e.systemMetrics.SwapUsage,
		e.systemMetrics.Load1,
		e.systemMetrics.Load5,
		e.systemMetrics.Load15,
		e.systemMetrics.Uptime,
		e.systemMetrics.ProcessCount,
		e.systemMetrics.ThreadCount,
		e.systemMetrics.FileDescriptor,
	)

	// 注册存储指标
	e.registry.MustRegister(
		e.storageMetrics.DiskTotal,
		e.storageMetrics.DiskUsed,
		e.storageMetrics.DiskFree,
		e.storageMetrics.DiskUsage,
		e.storageMetrics.DiskReadBytes,
		e.storageMetrics.DiskWriteBytes,
		e.storageMetrics.DiskReadOps,
		e.storageMetrics.DiskWriteOps,
		e.storageMetrics.DiskReadLatency,
		e.storageMetrics.DiskWriteLatency,
		e.storageMetrics.VolumeTotal,
		e.storageMetrics.VolumeUsed,
		e.storageMetrics.VolumeFree,
		e.storageMetrics.VolumeUsage,
		e.storageMetrics.VolumeHealth,
		e.storageMetrics.StoragePoolTotal,
		e.storageMetrics.StoragePoolUsed,
		e.storageMetrics.StoragePoolFree,
		e.storageMetrics.StoragePoolHealth,
		e.storageMetrics.QuotaUsed,
		e.storageMetrics.QuotaLimit,
		e.storageMetrics.QuotaUsage,
	)

	// 注册备份指标
	e.registry.MustRegister(
		e.backupMetrics.BackupTotal,
		e.backupMetrics.BackupSize,
		e.backupMetrics.BackupDuration,
		e.backupMetrics.BackupStatus,
		e.backupMetrics.BackupLastRun,
		e.backupMetrics.BackupLastError,
		e.backupMetrics.BackupFiles,
		e.backupMetrics.BackupSpeed,
		e.backupMetrics.RestoreTotal,
		e.backupMetrics.RestoreDuration,
		e.backupMetrics.RestoreStatus,
	)

	// 注册服务指标
	e.registry.MustRegister(
		e.serviceMetrics.ServiceStatus,
		e.serviceMetrics.ServiceUptime,
		e.serviceMetrics.ServiceRestarts,
		e.serviceMetrics.ServiceConnections,
		e.serviceMetrics.ServiceLatency,
		e.serviceMetrics.ServiceErrors,
		e.serviceMetrics.APIRequests,
		e.serviceMetrics.APILatency,
		e.serviceMetrics.APIInFlight,
		e.serviceMetrics.APIErrors,
		e.serviceMetrics.UserSessions,
		e.serviceMetrics.UserTotal,
		e.serviceMetrics.UserActive,
	)
}

// Start 启动导出器
func (e *MetricsExporter) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return nil
	}

	// 创建 HTTP 处理器
	handler := promhttp.HandlerFor(e.registry, promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
	})

	// 创建路由
	mux := http.NewServeMux()
	mux.Handle(e.path, handler)

	// 健康检查端点
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// 创建服务器
	addr := fmt.Sprintf(":%d", e.port)
	e.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	e.running = true

	// 启动服务器
	go func() {
		if err := e.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// 服务器错误
		}
	}()

	return nil
}

// Stop 停止导出器
func (e *MetricsExporter) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil
	}

	close(e.stopCh)

	if e.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := e.server.Shutdown(ctx); err != nil {
			return err
		}
	}

	e.running = false
	return nil
}

// ========== 更新系统指标 ==========

// UpdateCPU 更新 CPU 指标
func (e *MetricsExporter) UpdateCPU(usage, idle, user, system, iowait float64) {
	e.systemMetrics.CPUUsage.Set(usage)
	e.systemMetrics.CPUIdle.Set(idle)
	e.systemMetrics.CPUUser.Set(user)
	e.systemMetrics.CPUSystem.Set(system)
	e.systemMetrics.CPUIOWait.Set(iowait)

	e.updateAggregatedCPU(usage)
}

// UpdateMemory 更新内存指标
func (e *MetricsExporter) UpdateMemory(total, used, free, cached, buffers uint64, usage float64) {
	e.systemMetrics.MemoryTotal.Set(float64(total))
	e.systemMetrics.MemoryUsed.Set(float64(used))
	e.systemMetrics.MemoryFree.Set(float64(free))
	e.systemMetrics.MemoryCached.Set(float64(cached))
	e.systemMetrics.MemoryBuffers.Set(float64(buffers))
	e.systemMetrics.MemoryUsage.Set(usage)

	e.updateAggregatedMemory(usage)
}

// UpdateSwap 更新 Swap 指标
func (e *MetricsExporter) UpdateSwap(total, used, free uint64, usage float64) {
	e.systemMetrics.SwapTotal.Set(float64(total))
	e.systemMetrics.SwapUsed.Set(float64(used))
	e.systemMetrics.SwapFree.Set(float64(free))
	e.systemMetrics.SwapUsage.Set(usage)
}

// UpdateLoad 更新系统负载
func (e *MetricsExporter) UpdateLoad(load1, load5, load15 float64) {
	e.systemMetrics.Load1.Set(load1)
	e.systemMetrics.Load5.Set(load5)
	e.systemMetrics.Load15.Set(load15)
}

// UpdateSystem 更新系统状态
func (e *MetricsExporter) UpdateSystem(uptime uint64, processes, threads, fds int) {
	e.systemMetrics.Uptime.Set(float64(uptime))
	e.systemMetrics.ProcessCount.Set(float64(processes))
	e.systemMetrics.ThreadCount.Set(float64(threads))
	e.systemMetrics.FileDescriptor.Set(float64(fds))
}

// ========== 更新存储指标 ==========

// UpdateDisk 更新磁盘指标
func (e *MetricsExporter) UpdateDisk(device, mountpoint string, total, used, free uint64, usage float64) {
	labels := prometheus.Labels{"device": device, "mountpoint": mountpoint}
	e.storageMetrics.DiskTotal.With(labels).Set(float64(total))
	e.storageMetrics.DiskUsed.With(labels).Set(float64(used))
	e.storageMetrics.DiskFree.With(labels).Set(float64(free))
	e.storageMetrics.DiskUsage.With(labels).Set(usage)

	e.updateAggregatedDisk(usage)
}

// UpdateDiskIO 更新磁盘 I/O 指标
func (e *MetricsExporter) UpdateDiskIO(device string, readBytes, writeBytes, readOps, writeOps uint64, readLatency, writeLatency float64) {
	labels := prometheus.Labels{"device": device}
	e.storageMetrics.DiskReadBytes.With(labels).Set(float64(readBytes))
	e.storageMetrics.DiskWriteBytes.With(labels).Set(float64(writeBytes))
	e.storageMetrics.DiskReadOps.With(labels).Set(float64(readOps))
	e.storageMetrics.DiskWriteOps.With(labels).Set(float64(writeOps))
	e.storageMetrics.DiskReadLatency.With(labels).Set(readLatency)
	e.storageMetrics.DiskWriteLatency.With(labels).Set(writeLatency)
}

// UpdateVolume 更新卷指标
func (e *MetricsExporter) UpdateVolume(volume, volumeType string, total, used, free uint64, usage float64, healthy bool) {
	labels := prometheus.Labels{"volume": volume, "type": volumeType}
	e.storageMetrics.VolumeTotal.With(labels).Set(float64(total))
	e.storageMetrics.VolumeUsed.With(labels).Set(float64(used))
	e.storageMetrics.VolumeFree.With(labels).Set(float64(free))
	e.storageMetrics.VolumeUsage.With(labels).Set(usage)

	healthValue := float64(0)
	if healthy {
		healthValue = 1
	}
	e.storageMetrics.VolumeHealth.With(labels).Set(healthValue)
}

// UpdateStoragePool 更新存储池指标
func (e *MetricsExporter) UpdateStoragePool(pool string, total, used, free uint64, healthy bool) {
	labels := prometheus.Labels{"pool": pool}
	e.storageMetrics.StoragePoolTotal.With(labels).Set(float64(total))
	e.storageMetrics.StoragePoolUsed.With(labels).Set(float64(used))
	e.storageMetrics.StoragePoolFree.With(labels).Set(float64(free))

	healthValue := float64(0)
	if healthy {
		healthValue = 1
	}
	e.storageMetrics.StoragePoolHealth.With(labels).Set(healthValue)
}

// UpdateQuota 更新配额指标
func (e *MetricsExporter) UpdateQuota(user, volume string, used, limit uint64) {
	labels := prometheus.Labels{"user": user, "volume": volume}
	e.storageMetrics.QuotaUsed.With(labels).Set(float64(used))
	e.storageMetrics.QuotaLimit.With(labels).Set(float64(limit))
	if limit > 0 {
		e.storageMetrics.QuotaUsage.With(labels).Set(float64(used) / float64(limit) * 100)
	}
}

// ========== 更新备份指标 ==========

// UpdateBackup 更新备份指标
func (e *MetricsExporter) UpdateBackup(backupType, id string, size uint64, duration float64, status bool, files int, speed float64) {
	idLabels := prometheus.Labels{"type": backupType, "id": id}
	e.backupMetrics.BackupSize.With(idLabels).Set(float64(size))
	e.backupMetrics.BackupDuration.With(idLabels).Set(duration)

	statusValue := float64(0)
	if status {
		statusValue = 1
	}
	e.backupMetrics.BackupStatus.With(idLabels).Set(statusValue)
	e.backupMetrics.BackupFiles.With(idLabels).Set(float64(files))
	e.backupMetrics.BackupSpeed.With(idLabels).Set(speed)
}

// RecordBackupRun 记录备份运行
func (e *MetricsExporter) RecordBackupRun(backupType string, timestamp time.Time) {
	e.backupMetrics.BackupLastRun.With(prometheus.Labels{"type": backupType}).Set(float64(timestamp.Unix()))
}

// RecordBackupError 记录备份错误
func (e *MetricsExporter) RecordBackupError(backupType string, timestamp time.Time) {
	e.backupMetrics.BackupLastError.With(prometheus.Labels{"type": backupType}).Set(float64(timestamp.Unix()))
}

// ========== 更新服务指标 ==========

// UpdateServiceStatus 更新服务状态
func (e *MetricsExporter) UpdateServiceStatus(service string, running bool, uptime uint64, restarts int) {
	labels := prometheus.Labels{"service": service}

	statusValue := float64(0)
	if running {
		statusValue = 1
	}
	e.serviceMetrics.ServiceStatus.With(labels).Set(statusValue)
	e.serviceMetrics.ServiceUptime.With(labels).Set(float64(uptime))
	e.serviceMetrics.ServiceRestarts.With(labels).Set(float64(restarts))
}

// UpdateServiceMetrics 更新服务指标
func (e *MetricsExporter) UpdateServiceMetrics(service string, connections int, latency float64) {
	labels := prometheus.Labels{"service": service}
	e.serviceMetrics.ServiceConnections.With(labels).Set(float64(connections))
	e.serviceMetrics.ServiceLatency.With(labels).Set(latency)
}

// RecordServiceError 记录服务错误
func (e *MetricsExporter) RecordServiceError(service, errorType string) {
	e.serviceMetrics.ServiceErrors.WithLabelValues(service, errorType).Inc()
}

// RecordAPIRequest 记录 API 请求
func (e *MetricsExporter) RecordAPIRequest(method, endpoint, status string, duration float64) {
	e.serviceMetrics.APIRequests.WithLabelValues(method, endpoint, status).Inc()
	e.serviceMetrics.APILatency.WithLabelValues(method, endpoint).Observe(duration)
}

// IncAPIInFlight 增加正在处理的请求数
func (e *MetricsExporter) IncAPIInFlight() {
	e.serviceMetrics.APIInFlight.Inc()
}

// DecAPIInFlight 减少正在处理的请求数
func (e *MetricsExporter) DecAPIInFlight() {
	e.serviceMetrics.APIInFlight.Dec()
}

// RecordAPIError 记录 API 错误
func (e *MetricsExporter) RecordAPIError(method, endpoint, errorType string) {
	e.serviceMetrics.APIErrors.WithLabelValues(method, endpoint, errorType).Inc()
}

// UpdateUserMetrics 更新用户指标
func (e *MetricsExporter) UpdateUserMetrics(total, active int, sessions map[string]int) {
	e.serviceMetrics.UserTotal.Set(float64(total))
	e.serviceMetrics.UserActive.Set(float64(active))

	for user, count := range sessions {
		e.serviceMetrics.UserSessions.WithLabelValues(user).Set(float64(count))
	}
}

// ========== 自定义指标 ==========

// RegisterCustomGauge 注册自定义 Gauge
func (e *MetricsExporter) RegisterCustomGauge(name, help string, labels []string) (*prometheus.GaugeVec, error) {
	e.customMetrics.mu.Lock()
	defer e.customMetrics.mu.Unlock()

	if _, exists := e.customMetrics.gauges[name]; exists {
		return nil, fmt.Errorf("gauge %s already registered", name)
	}

	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: e.namespace,
		Name:      name,
		Help:      help,
	}, labels)

	e.registry.MustRegister(gauge)
	e.customMetrics.gauges[name] = gauge

	return gauge, nil
}

// RegisterCustomCounter 注册自定义 Counter
func (e *MetricsExporter) RegisterCustomCounter(name, help string, labels []string) (*prometheus.CounterVec, error) {
	e.customMetrics.mu.Lock()
	defer e.customMetrics.mu.Unlock()

	if _, exists := e.customMetrics.counters[name]; exists {
		return nil, fmt.Errorf("counter %s already registered", name)
	}

	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: e.namespace,
		Name:      name,
		Help:      help,
	}, labels)

	e.registry.MustRegister(counter)
	e.customMetrics.counters[name] = counter

	return counter, nil
}

// RegisterCustomHistogram 注册自定义 Histogram
func (e *MetricsExporter) RegisterCustomHistogram(name, help string, labels []string, buckets []float64) (*prometheus.HistogramVec, error) {
	e.customMetrics.mu.Lock()
	defer e.customMetrics.mu.Unlock()

	if _, exists := e.customMetrics.histograms[name]; exists {
		return nil, fmt.Errorf("histogram %s already registered", name)
	}

	histogram := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: e.namespace,
		Name:      name,
		Help:      help,
		Buckets:   buckets,
	}, labels)

	e.registry.MustRegister(histogram)
	e.customMetrics.histograms[name] = histogram

	return histogram, nil
}

// GetCustomGauge 获取自定义 Gauge
func (e *MetricsExporter) GetCustomGauge(name string) (*prometheus.GaugeVec, bool) {
	e.customMetrics.mu.RLock()
	defer e.customMetrics.mu.RUnlock()
	gauge, exists := e.customMetrics.gauges[name]
	return gauge, exists
}

// GetCustomCounter 获取自定义 Counter
func (e *MetricsExporter) GetCustomCounter(name string) (*prometheus.CounterVec, bool) {
	e.customMetrics.mu.RLock()
	defer e.customMetrics.mu.RUnlock()
	counter, exists := e.customMetrics.counters[name]
	return counter, exists
}

// ========== 指标聚合 ==========

func (e *MetricsExporter) updateAggregatedCPU(usage float64) {
	e.aggregatedStats.mu.Lock()
	defer e.aggregatedStats.mu.Unlock()

	e.aggregatedStats.dataPoints++
	e.aggregatedStats.avgCPUUsage = (e.aggregatedStats.avgCPUUsage*float64(e.aggregatedStats.dataPoints-1) + usage) / float64(e.aggregatedStats.dataPoints)
	if usage > e.aggregatedStats.maxCPUUsage {
		e.aggregatedStats.maxCPUUsage = usage
	}
	e.aggregatedStats.lastUpdate = time.Now()
}

func (e *MetricsExporter) updateAggregatedMemory(usage float64) {
	e.aggregatedStats.mu.Lock()
	defer e.aggregatedStats.mu.Unlock()

	e.aggregatedStats.dataPoints++
	e.aggregatedStats.avgMemoryUsage = (e.aggregatedStats.avgMemoryUsage*float64(e.aggregatedStats.dataPoints-1) + usage) / float64(e.aggregatedStats.dataPoints)
	if usage > e.aggregatedStats.maxMemoryUsage {
		e.aggregatedStats.maxMemoryUsage = usage
	}
	e.aggregatedStats.lastUpdate = time.Now()
}

func (e *MetricsExporter) updateAggregatedDisk(usage float64) {
	e.aggregatedStats.mu.Lock()
	defer e.aggregatedStats.mu.Unlock()

	e.aggregatedStats.dataPoints++
	e.aggregatedStats.avgDiskUsage = (e.aggregatedStats.avgDiskUsage*float64(e.aggregatedStats.dataPoints-1) + usage) / float64(e.aggregatedStats.dataPoints)
	if usage > e.aggregatedStats.maxDiskUsage {
		e.aggregatedStats.maxDiskUsage = usage
	}
	e.aggregatedStats.lastUpdate = time.Now()
}

// GetAggregatedStats 获取聚合统计
func (e *MetricsExporter) GetAggregatedStats() AggregatedStats {
	e.aggregatedStats.mu.RLock()
	defer e.aggregatedStats.mu.RUnlock()
	return *e.aggregatedStats
}

// ResetAggregatedStats 重置聚合统计
func (e *MetricsExporter) ResetAggregatedStats() {
	e.aggregatedStats.mu.Lock()
	defer e.aggregatedStats.mu.Unlock()
	e.aggregatedStats.avgCPUUsage = 0
	e.aggregatedStats.maxCPUUsage = 0
	e.aggregatedStats.avgMemoryUsage = 0
	e.aggregatedStats.maxMemoryUsage = 0
	e.aggregatedStats.avgDiskUsage = 0
	e.aggregatedStats.maxDiskUsage = 0
	e.aggregatedStats.totalRequests = 0
	e.aggregatedStats.totalErrors = 0
	e.aggregatedStats.dataPoints = 0
	e.aggregatedStats.lastUpdate = time.Now()
}

// IsRunning 检查是否正在运行
func (e *MetricsExporter) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// GetEndpoint 获取指标端点
func (e *MetricsExporter) GetEndpoint() string {
	return fmt.Sprintf("http://localhost:%d%s", e.port, e.path)
}