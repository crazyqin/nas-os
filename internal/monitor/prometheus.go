package monitor

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusMetrics Prometheus 指标收集器
// v2.29.0 增强版 - 支持完整的 NAS-OS 监控指标.
type PrometheusMetrics struct {
	// ========== 系统指标 ==========

	// CPU 使用率
	CPUUsageGauge prometheus.Gauge

	// 内存使用
	MemoryUsageGauge     prometheus.Gauge
	MemoryTotalGauge     prometheus.Gauge
	MemoryUsedGauge      prometheus.Gauge
	MemoryAvailableGauge prometheus.Gauge

	// Swap 使用
	SwapUsageGauge prometheus.Gauge
	SwapTotalGauge prometheus.Gauge
	SwapUsedGauge  prometheus.Gauge

	// 系统负载
	LoadAvg1Gauge  prometheus.Gauge
	LoadAvg5Gauge  prometheus.Gauge
	LoadAvg15Gauge prometheus.Gauge

	// 运行时间
	UptimeGauge prometheus.Gauge

	// ========== 磁盘指标 ==========

	// 磁盘使用率
	DiskUsageGauge *prometheus.GaugeVec

	// 磁盘空间
	DiskTotalGauge *prometheus.GaugeVec
	DiskUsedGauge  *prometheus.GaugeVec
	DiskFreeGauge  *prometheus.GaugeVec

	// 磁盘 I/O
	DiskReadBytesGauge  *prometheus.GaugeVec
	DiskWriteBytesGauge *prometheus.GaugeVec
	DiskReadOpsGauge    *prometheus.GaugeVec
	DiskWriteOpsGauge   *prometheus.GaugeVec

	// ========== 网络指标 ==========

	// 网络流量
	NetworkRXBytesGauge   *prometheus.GaugeVec
	NetworkTXBytesGauge   *prometheus.GaugeVec
	NetworkRXPacketsGauge *prometheus.GaugeVec
	NetworkTXPacketsGauge *prometheus.GaugeVec
	NetworkRXErrorsGauge  *prometheus.GaugeVec
	NetworkTXErrorsGauge  *prometheus.GaugeVec

	// ========== NAS-OS 业务指标 ==========

	// 备份指标
	BackupTotalGauge    *prometheus.GaugeVec
	BackupSizeGauge     *prometheus.GaugeVec
	BackupDurationGauge *prometheus.GaugeVec
	BackupStatusGauge   *prometheus.GaugeVec
	BackupLastRunGauge  *prometheus.GaugeVec

	// 快照指标
	SnapshotTotalGauge     *prometheus.GaugeVec
	SnapshotSizeGauge      *prometheus.GaugeVec
	SnapshotCreationTime   *prometheus.GaugeVec
	SnapshotRetentionGauge *prometheus.GaugeVec

	// 存储池指标
	StoragePoolUsageGauge  *prometheus.GaugeVec
	StoragePoolTotalGauge  *prometheus.GaugeVec
	StoragePoolUsedGauge   *prometheus.GaugeVec
	StoragePoolHealthGauge *prometheus.GaugeVec

	// 共享指标
	ShareTotalGauge       *prometheus.GaugeVec
	ShareConnectionsGauge *prometheus.GaugeVec
	ShareBytesGauge       *prometheus.GaugeVec

	// 用户指标
	UserTotalGauge    prometheus.Gauge
	UserActiveGauge   prometheus.Gauge
	UserSessionsGauge *prometheus.GaugeVec

	// API 指标
	APIRequestsTotal    *prometheus.CounterVec
	APIRequestDuration  *prometheus.HistogramVec
	APIRequestsInFlight prometheus.Gauge

	// ========== 服务健康指标 ==========

	// 服务状态
	ServiceHealthGauge *prometheus.GaugeVec
	ServiceUptimeGauge *prometheus.GaugeVec

	// 告警
	AlertsTotalGauge  *prometheus.GaugeVec
	AlertsActiveGauge prometheus.Gauge

	// 健康评分
	HealthScoreGauge prometheus.Gauge
}

// NewPrometheusMetrics 创建 Prometheus 指标收集器.
func NewPrometheusMetrics(namespace string) *PrometheusMetrics {
	if namespace == "" {
		namespace = "nas_os"
	}

	return &PrometheusMetrics{
		// 系统指标
		CPUUsageGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "cpu_usage_percent",
			Help:      "Current CPU usage percentage",
		}),

		MemoryUsageGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "memory_usage_percent",
			Help:      "Current memory usage percentage",
		}),

		MemoryTotalGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "memory_total_bytes",
			Help:      "Total system memory in bytes",
		}),

		MemoryUsedGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "memory_used_bytes",
			Help:      "Used system memory in bytes",
		}),

		MemoryAvailableGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "memory_available_bytes",
			Help:      "Available system memory in bytes",
		}),

		SwapUsageGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "swap_usage_percent",
			Help:      "Current swap usage percentage",
		}),

		SwapTotalGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "swap_total_bytes",
			Help:      "Total swap space in bytes",
		}),

		SwapUsedGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "swap_used_bytes",
			Help:      "Used swap space in bytes",
		}),

		LoadAvg1Gauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "load_avg_1",
			Help:      "1 minute load average",
		}),

		LoadAvg5Gauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "load_avg_5",
			Help:      "5 minute load average",
		}),

		LoadAvg15Gauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "load_avg_15",
			Help:      "15 minute load average",
		}),

		UptimeGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "uptime_seconds",
			Help:      "System uptime in seconds",
		}),

		// 磁盘指标
		DiskUsageGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "disk_usage_percent",
			Help:      "Disk usage percentage by mount point",
		}, []string{"device", "mountpoint", "fstype"}),

		DiskTotalGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "disk_total_bytes",
			Help:      "Total disk space in bytes",
		}, []string{"device", "mountpoint"}),

		DiskUsedGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "disk_used_bytes",
			Help:      "Used disk space in bytes",
		}, []string{"device", "mountpoint"}),

		DiskFreeGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "disk_free_bytes",
			Help:      "Free disk space in bytes",
		}, []string{"device", "mountpoint"}),

		DiskReadBytesGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "disk_read_bytes_total",
			Help:      "Total bytes read from disk",
		}, []string{"device"}),

		DiskWriteBytesGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "disk_write_bytes_total",
			Help:      "Total bytes written to disk",
		}, []string{"device"}),

		DiskReadOpsGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "disk_read_ops_total",
			Help:      "Total disk read operations",
		}, []string{"device"}),

		DiskWriteOpsGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "disk_write_ops_total",
			Help:      "Total disk write operations",
		}, []string{"device"}),

		// 网络指标
		NetworkRXBytesGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "network_rx_bytes_total",
			Help:      "Total bytes received",
		}, []string{"interface"}),

		NetworkTXBytesGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "network_tx_bytes_total",
			Help:      "Total bytes transmitted",
		}, []string{"interface"}),

		NetworkRXPacketsGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "network_rx_packets_total",
			Help:      "Total packets received",
		}, []string{"interface"}),

		NetworkTXPacketsGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "network_tx_packets_total",
			Help:      "Total packets transmitted",
		}, []string{"interface"}),

		NetworkRXErrorsGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "network_rx_errors_total",
			Help:      "Total receive errors",
		}, []string{"interface"}),

		NetworkTXErrorsGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "network_tx_errors_total",
			Help:      "Total transmit errors",
		}, []string{"interface"}),

		// 备份指标
		BackupTotalGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "backup_total",
			Help:      "Total number of backup jobs",
		}, []string{"type", "status"}),

		BackupSizeGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "backup_size_bytes",
			Help:      "Backup size in bytes",
		}, []string{"job_id", "job_name"}),

		BackupDurationGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "backup_duration_seconds",
			Help:      "Backup duration in seconds",
		}, []string{"job_id", "job_name"}),

		BackupStatusGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "backup_status",
			Help:      "Backup job status (0=pending, 1=running, 2=completed, 3=failed, 4=cancelled)",
		}, []string{"job_id"}),

		BackupLastRunGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "backup_last_run_timestamp",
			Help:      "Unix timestamp of last backup run",
		}, []string{"job_id"}),

		// 快照指标
		SnapshotTotalGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "snapshot_total",
			Help:      "Total number of snapshots",
		}, []string{"pool", "type"}),

		SnapshotSizeGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "snapshot_size_bytes",
			Help:      "Snapshot size in bytes",
		}, []string{"pool", "snapshot_id"}),

		SnapshotCreationTime: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "snapshot_creation_timestamp",
			Help:      "Snapshot creation timestamp",
		}, []string{"pool", "snapshot_id"}),

		SnapshotRetentionGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "snapshot_retention_days",
			Help:      "Snapshot retention period in days",
		}, []string{"policy_id"}),

		// 存储池指标
		StoragePoolUsageGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "storage_pool_usage_percent",
			Help:      "Storage pool usage percentage",
		}, []string{"pool_name", "pool_type"}),

		StoragePoolTotalGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "storage_pool_total_bytes",
			Help:      "Storage pool total size in bytes",
		}, []string{"pool_name", "pool_type"}),

		StoragePoolUsedGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "storage_pool_used_bytes",
			Help:      "Storage pool used size in bytes",
		}, []string{"pool_name", "pool_type"}),

		StoragePoolHealthGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "storage_pool_health_status",
			Help:      "Storage pool health status (0=unknown, 1=healthy, 2=degraded, 3=failed)",
		}, []string{"pool_name"}),

		// 共享指标
		ShareTotalGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "share_total",
			Help:      "Total number of shares",
		}, []string{"type"}),

		ShareConnectionsGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "share_connections",
			Help:      "Active connections to share",
		}, []string{"share_name", "type"}),

		ShareBytesGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "share_bytes_transferred_total",
			Help:      "Total bytes transferred via share",
		}, []string{"share_name", "direction"}),

		// 用户指标
		UserTotalGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "users_total",
			Help:      "Total number of users",
		}),

		UserActiveGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "users_active",
			Help:      "Number of active users",
		}),

		UserSessionsGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "user_sessions",
			Help:      "Active user sessions",
		}, []string{"username"}),

		// API 指标
		APIRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "api_requests_total",
			Help:      "Total number of API requests",
		}, []string{"method", "path", "status"}),

		APIRequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "api_request_duration_seconds",
			Help:      "API request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "path"}),

		APIRequestsInFlight: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "api_requests_in_flight",
			Help:      "Number of API requests currently being processed",
		}),

		// 服务健康指标
		ServiceHealthGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "service_health_status",
			Help:      "Service health status (0=unknown, 1=healthy, 2=degraded, 3=failed)",
		}, []string{"service"}),

		ServiceUptimeGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "service_uptime_seconds",
			Help:      "Service uptime in seconds",
		}, []string{"service"}),

		// 告警指标
		AlertsTotalGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "alerts_total",
			Help:      "Total number of alerts by type",
		}, []string{"type", "level"}),

		AlertsActiveGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "alerts_active",
			Help:      "Number of active alerts",
		}),

		// 健康评分
		HealthScoreGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "health_score",
			Help:      "Overall system health score (0-100)",
		}),
	}
}

// UpdateSystemMetrics 更新系统指标.
func (pm *PrometheusMetrics) UpdateSystemMetrics(stats *SystemStats) {
	if stats == nil {
		return
	}

	pm.CPUUsageGauge.Set(stats.CPUUsage)
	pm.MemoryUsageGauge.Set(stats.MemoryUsage)
	pm.MemoryTotalGauge.Set(float64(stats.MemoryTotal))
	pm.MemoryUsedGauge.Set(float64(stats.MemoryUsed))
	pm.MemoryAvailableGauge.Set(float64(stats.MemoryFree))
	pm.SwapUsageGauge.Set(stats.SwapUsage)
	pm.SwapTotalGauge.Set(float64(stats.SwapTotal))
	pm.SwapUsedGauge.Set(float64(stats.SwapUsed))
	pm.UptimeGauge.Set(float64(stats.UptimeSeconds))

	if len(stats.LoadAvg) >= 3 {
		pm.LoadAvg1Gauge.Set(stats.LoadAvg[0])
		pm.LoadAvg5Gauge.Set(stats.LoadAvg[1])
		pm.LoadAvg15Gauge.Set(stats.LoadAvg[2])
	}
}

// UpdateDiskMetrics 更新磁盘指标.
func (pm *PrometheusMetrics) UpdateDiskMetrics(diskStats []*DiskStats) {
	for _, d := range diskStats {
		if d.FSType == "tmpfs" || d.FSType == "devtmpfs" {
			continue
		}

		labels := prometheus.Labels{
			"device":     d.Device,
			"mountpoint": d.MountPoint,
		}

		pm.DiskUsageGauge.With(labels).Set(d.UsagePercent)
		pm.DiskTotalGauge.With(labels).Set(float64(d.Total))
		pm.DiskUsedGauge.With(labels).Set(float64(d.Used))
		pm.DiskFreeGauge.With(labels).Set(float64(d.Free))
	}
}

// UpdateNetworkMetrics 更新网络指标.
func (pm *PrometheusMetrics) UpdateNetworkMetrics(netStats []*NetworkStats) {
	for _, n := range netStats {
		labels := prometheus.Labels{
			"interface": n.Interface,
		}

		pm.NetworkRXBytesGauge.With(labels).Set(float64(n.RXBytes))
		pm.NetworkTXBytesGauge.With(labels).Set(float64(n.TXBytes))
		pm.NetworkRXPacketsGauge.With(labels).Set(float64(n.RXPackets))
		pm.NetworkTXPacketsGauge.With(labels).Set(float64(n.TXPackets))
		pm.NetworkRXErrorsGauge.With(labels).Set(float64(n.RXErrors))
		pm.NetworkTXErrorsGauge.With(labels).Set(float64(n.TXErrors))
	}
}

// UpdateHealthScore 更新健康评分.
func (pm *PrometheusMetrics) UpdateHealthScore(score float64) {
	pm.HealthScoreGauge.Set(score)
}

// RecordAPIRequest 记录 API 请求.
func (pm *PrometheusMetrics) RecordAPIRequest(method, path, status string, duration float64) {
	pm.APIRequestsTotal.WithLabelValues(method, path, status).Inc()
	pm.APIRequestDuration.WithLabelValues(method, path).Observe(duration)
}

// IncrementAPIRequestsInFlight 增加 API 请求计数.
func (pm *PrometheusMetrics) IncrementAPIRequestsInFlight() {
	pm.APIRequestsInFlight.Inc()
}

// DecrementAPIRequestsInFlight 减少 API 请求计数.
func (pm *PrometheusMetrics) DecrementAPIRequestsInFlight() {
	pm.APIRequestsInFlight.Dec()
}
