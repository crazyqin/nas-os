package performance

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// APIHandlers 性能监控 API 处理器
type APIHandlers struct {
	logger     *zap.Logger
	collector  *SystemCollector
	storage    *StorageCollector
	health     *HealthChecker
	alerts     *AlertManager
	prometheus *PrometheusExporter
	monitor    *PerformanceMonitor
}

// NewAPIHandlers 创建 API 处理器
func NewAPIHandlers(
	logger *zap.Logger,
	collector *SystemCollector,
	storage *StorageCollector,
	health *HealthChecker,
	alerts *AlertManager,
	prometheus *PrometheusExporter,
	monitor *PerformanceMonitor,
) *APIHandlers {
	return &APIHandlers{
		logger:     logger,
		collector:  collector,
		storage:    storage,
		health:     health,
		alerts:     alerts,
		prometheus: prometheus,
		monitor:    monitor,
	}
}

// RegisterRoutes 注册路由
func (h *APIHandlers) RegisterRoutes(r *gin.RouterGroup) {
	// 指标端点
	r.GET("/metrics", h.GetMetrics)
	r.GET("/metrics/prometheus", h.GetPrometheusMetrics)

	// 健康检查
	r.GET("/health", h.GetHealth)
	r.GET("/health/checks", h.GetHealthChecks)

	// 告警管理
	r.GET("/alerts", h.GetAlerts)
	r.GET("/alerts/history", h.GetAlertHistory)
	r.POST("/alerts/:id/acknowledge", h.AcknowledgeAlert)
	r.POST("/alerts/:id/silence", h.SilenceAlert)
	r.DELETE("/alerts/:id", h.ClearAlert)

	// 告警规则
	r.GET("/alerts/rules", h.GetAlertRules)
	r.PUT("/alerts/rules/:id", h.UpdateAlertRule)

	// 系统指标
	r.GET("/system", h.GetSystemMetrics)
	r.GET("/system/cpu", h.GetCPUMetrics)
	r.GET("/system/memory", h.GetMemoryMetrics)
	r.GET("/system/disks", h.GetDiskMetrics)
	r.GET("/system/network", h.GetNetworkMetrics)

	// 存储性能
	r.GET("/storage", h.GetStorageMetrics)
	r.GET("/storage/iops", h.GetIOPSMetrics)
	r.GET("/storage/latency", h.GetLatencyMetrics)
	r.GET("/storage/throughput", h.GetThroughputMetrics)

	// 历史数据
	r.GET("/history/cpu", h.GetCPUHistory)
	r.GET("/history/memory", h.GetMemoryHistory)
	r.GET("/history/storage", h.GetStorageHistory)
}

// GetMetrics 获取综合指标
// @Summary 获取综合指标
// @Description 获取系统、存储、健康等综合指标
// @Tags 性能监控
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/metrics [get]
func (h *APIHandlers) GetMetrics(c *gin.Context) {
	metrics := gin.H{
		"timestamp": time.Now(),
		"system": gin.H{
			"cpu":    h.collector.collectCPU(),
			"memory": h.collector.collectMemory(),
			"disks":  h.collector.collectDisks(),
			"uptime": h.collector.getUptime(),
		},
		"storage": gin.H{
			"disk_io": h.collector.collectDiskIO(),
			"network": h.collector.collectNetwork(),
		},
		"health": h.health.GetHealth(),
		"alerts": gin.H{
			"active": len(h.alerts.GetAlerts()),
			"stats":  h.alerts.GetAlertStats(),
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    metrics,
	})
}

// GetPrometheusMetrics 获取 Prometheus 格式指标
// @Summary 获取 Prometheus 格式指标
// @Description 返回 Prometheus 格式的指标数据
// @Tags 性能监控
// @Produce plain
// @Success 200 {string} string
// @Router /metrics [get]
func (h *APIHandlers) GetPrometheusMetrics(c *gin.Context) {
	if h.prometheus == nil {
		c.String(http.StatusServiceUnavailable, "Prometheus exporter not configured")
		return
	}
	h.prometheus.Handler(c.Writer, c.Request)
}

// GetHealth 获取健康状态
// @Summary 获取健康状态
// @Description 获取系统健康状态
// @Tags 性能监控
// @Produce json
// @Success 200 {object} SystemHealth
// @Router /api/v1/health [get]
func (h *APIHandlers) GetHealth(c *gin.Context) {
	health := h.health.GetHealth()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    health,
	})
}

// GetHealthChecks 获取详细健康检查
// @Summary 获取详细健康检查
// @Description 获取各项健康检查的详细结果
// @Tags 性能监控
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/health/checks [get]
func (h *APIHandlers) GetHealthChecks(c *gin.Context) {
	health := h.health.GetHealth()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"status":  health.Status,
			"score":   health.Score,
			"checks":  health.Checks,
			"issues":  health.Issues,
			"uptime":  health.Uptime,
		},
	})
}

// GetAlerts 获取活动告警
// @Summary 获取活动告警
// @Description 获取当前活动的告警列表
// @Tags 告警管理
// @Produce json
// @Param level query string false "告警级别过滤"
// @Param type query string false "告警类型过滤"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/alerts [get]
func (h *APIHandlers) GetAlerts(c *gin.Context) {
	alerts := h.alerts.GetAlerts()

	// 过滤
	level := c.Query("level")
	alertType := c.Query("type")

	if level != "" || alertType != "" {
		filtered := make([]*Alert, 0)
		for _, a := range alerts {
			if level != "" && string(a.Level) != level {
				continue
			}
			if alertType != "" && a.Type != alertType {
				continue
			}
			filtered = append(filtered, a)
		}
		alerts = filtered
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"alerts": alerts,
			"total":  len(alerts),
			"stats":  h.alerts.GetAlertStats(),
		},
	})
}

// GetAlertHistory 获取告警历史
// @Summary 获取告警历史
// @Description 获取历史告警记录
// @Tags 告警管理
// @Produce json
// @Param limit query int false "返回数量限制" default(50)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/alerts/history [get]
func (h *APIHandlers) GetAlertHistory(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 {
		limit = 50
	}

	history := h.alerts.GetAlertHistory(limit)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"history": history,
			"total":   len(history),
		},
	})
}

// AcknowledgeAlert 确认告警
// @Summary 确认告警
// @Description 确认一个告警
// @Tags 告警管理
// @Param id path string true "告警ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/alerts/{id}/acknowledge [post]
func (h *APIHandlers) AcknowledgeAlert(c *gin.Context) {
	id := c.Param("id")
	user := c.GetString("user") // 从上下文获取用户

	if err := h.alerts.AcknowledgeAlert(id, user); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "告警已确认",
	})
}

// SilenceAlert 静默告警
// @Summary 静默告警
// @Description 静默一个告警
// @Tags 告警管理
// @Param id path string true "告警ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/alerts/{id}/silence [post]
func (h *APIHandlers) SilenceAlert(c *gin.Context) {
	id := c.Param("id")

	if err := h.alerts.SilenceAlert(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "告警已静默",
	})
}

// ClearAlert 清除告警
// @Summary 清除告警
// @Description 清除一个告警
// @Tags 告警管理
// @Param id path string true "告警ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/alerts/{id} [delete]
func (h *APIHandlers) ClearAlert(c *gin.Context) {
	id := c.Param("id")

	if err := h.alerts.ClearAlert(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "告警已清除",
	})
}

// GetAlertRules 获取告警规则
// @Summary 获取告警规则
// @Description 获取所有告警规则
// @Tags 告警管理
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/alerts/rules [get]
func (h *APIHandlers) GetAlertRules(c *gin.Context) {
	rules := h.alerts.GetRules()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rules,
	})
}

// UpdateAlertRule 更新告警规则
// @Summary 更新告警规则
// @Description 更新告警规则配置
// @Tags 告警管理
// @Param id path string true "规则ID"
// @Param body body map[string]interface{} true "更新内容"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/alerts/rules/{id} [put]
func (h *APIHandlers) UpdateAlertRule(c *gin.Context) {
	id := c.Param("id")

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数",
		})
		return
	}

	if err := h.alerts.UpdateRule(id, updates); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则已更新",
	})
}

// GetSystemMetrics 获取系统指标
// @Summary 获取系统指标
// @Description 获取系统资源指标
// @Tags 系统指标
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/system [get]
func (h *APIHandlers) GetSystemMetrics(c *gin.Context) {
	summary := h.collector.Collect()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    summary,
	})
}

// GetCPUMetrics 获取 CPU 指标
// @Summary 获取 CPU 指标
// @Description 获取 CPU 使用率和负载指标
// @Tags 系统指标
// @Produce json
// @Success 200 {object} CPUMetric
// @Router /api/v1/system/cpu [get]
func (h *APIHandlers) GetCPUMetrics(c *gin.Context) {
	cpu := h.collector.collectCPU()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    cpu,
	})
}

// GetMemoryMetrics 获取内存指标
// @Summary 获取内存指标
// @Description 获取内存使用指标
// @Tags 系统指标
// @Produce json
// @Success 200 {object} MemoryMetric
// @Router /api/v1/system/memory [get]
func (h *APIHandlers) GetMemoryMetrics(c *gin.Context) {
	mem := h.collector.collectMemory()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    mem,
	})
}

// GetDiskMetrics 获取磁盘指标
// @Summary 获取磁盘指标
// @Description 获取磁盘使用和 I/O 指标
// @Tags 系统指标
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/system/disks [get]
func (h *APIHandlers) GetDiskMetrics(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"usage": h.collector.collectDisks(),
			"io":    h.collector.collectDiskIO(),
		},
	})
}

// GetNetworkMetrics 获取网络指标
// @Summary 获取网络指标
// @Description 获取网络接口指标
// @Tags 系统指标
// @Produce json
// @Success 200 {object} []NetworkMetric
// @Router /api/v1/system/network [get]
func (h *APIHandlers) GetNetworkMetrics(c *gin.Context) {
	network := h.collector.collectNetwork()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    network,
	})
}

// GetStorageMetrics 获取存储性能指标
// @Summary 获取存储性能指标
// @Description 获取 IOPS、延迟、吞吐量等存储性能指标
// @Tags 存储性能
// @Produce json
// @Success 200 {object} StorageMetrics
// @Router /api/v1/storage [get]
func (h *APIHandlers) GetStorageMetrics(c *gin.Context) {
	metrics := h.storage.Collect()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    metrics,
	})
}

// GetIOPSMetrics 获取 IOPS 指标
// @Summary 获取 IOPS 指标
// @Description 获取 IOPS 指标详情
// @Tags 存储性能
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/storage/iops [get]
func (h *APIHandlers) GetIOPSMetrics(c *gin.Context) {
	metrics := h.storage.Collect()

	readAvg, writeAvg := h.storage.GetAverageIOPS()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"current": gin.H{
				"read":  metrics.IOPS.Read,
				"write": metrics.IOPS.Write,
				"total": metrics.IOPS.Total,
			},
			"average": gin.H{
				"read":  readAvg,
				"write": writeAvg,
			},
			"devices": metrics.Devices,
		},
	})
}

// GetLatencyMetrics 获取延迟指标
// @Summary 获取延迟指标
// @Description 获取存储延迟指标
// @Tags 存储性能
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/storage/latency [get]
func (h *APIHandlers) GetLatencyMetrics(c *gin.Context) {
	metrics := h.storage.Collect()
	readAvg, writeAvg := h.storage.GetAverageLatency()
	p50, p95, p99 := h.storage.CalculatePercentiles()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"current": gin.H{
				"read_avg_ms":  metrics.Latency.ReadAvgMs,
				"write_avg_ms": metrics.Latency.WriteAvgMs,
			},
			"average": gin.H{
				"read_ms":  readAvg,
				"write_ms": writeAvg,
			},
			"percentiles": gin.H{
				"p50_ms": p50,
				"p95_ms": p95,
				"p99_ms": p99,
			},
		},
	})
}

// GetThroughputMetrics 获取吞吐量指标
// @Summary 获取吞吐量指标
// @Description 获取存储吞吐量指标
// @Tags 存储性能
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/storage/throughput [get]
func (h *APIHandlers) GetThroughputMetrics(c *gin.Context) {
	metrics := h.storage.Collect()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"read_bytes_per_sec":  metrics.Throughput.ReadBytesPerSec,
			"write_bytes_per_sec": metrics.Throughput.WriteBytesPerSec,
			"total_bytes_per_sec": metrics.Throughput.TotalBytesPerSec,
			"read_mb_per_sec":     metrics.Throughput.ReadMBPerSec,
			"write_mb_per_sec":    metrics.Throughput.WriteMBPerSec,
			"total_mb_per_sec":    metrics.Throughput.TotalMBPerSec,
		},
	})
}

// GetCPUHistory 获取 CPU 历史数据
// @Summary 获取 CPU 历史数据
// @Description 获取 CPU 使用率历史数据
// @Tags 历史数据
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/history/cpu [get]
func (h *APIHandlers) GetCPUHistory(c *gin.Context) {
	history := h.collector.GetCPUHistory()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    history,
	})
}

// GetMemoryHistory 获取内存历史数据
// @Summary 获取内存历史数据
// @Description 获取内存使用率历史数据
// @Tags 历史数据
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/history/memory [get]
func (h *APIHandlers) GetMemoryHistory(c *gin.Context) {
	history := h.collector.GetMemoryHistory()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    history,
	})
}

// GetStorageHistory 获取存储历史数据
// @Summary 获取存储历史数据
// @Description 获取存储性能历史数据
// @Tags 历史数据
// @Produce json
// @Success 200 {object} []StorageMetrics
// @Router /api/v1/history/storage [get]
func (h *APIHandlers) GetStorageHistory(c *gin.Context) {
	history := h.storage.GetHistory()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    history,
	})
}