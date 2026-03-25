// Package monitor 提供监控仪表板 API
// 包括实时数据、历史数据、告警统计等接口
package monitor

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// DashboardAPI 监控仪表板 API.
type DashboardAPI struct {
	manager           *Manager
	alertManager      *AlertingManager
	diskHealthMonitor *DiskHealthMonitor
	ruleEngine        *AlertRuleEngine
	logCollector      *LogCollector
	promMetrics       *PrometheusMetrics
}

// NewDashboardAPI 创建监控仪表板 API.
func NewDashboardAPI(
	manager *Manager,
	alertManager *AlertingManager,
	diskHealthMonitor *DiskHealthMonitor,
	ruleEngine *AlertRuleEngine,
	logCollector *LogCollector,
	promMetrics *PrometheusMetrics,
) *DashboardAPI {
	return &DashboardAPI{
		manager:           manager,
		alertManager:      alertManager,
		diskHealthMonitor: diskHealthMonitor,
		ruleEngine:        ruleEngine,
		logCollector:      logCollector,
		promMetrics:       promMetrics,
	}
}

// RegisterRoutes 注册路由.
func (api *DashboardAPI) RegisterRoutes(r *gin.RouterGroup) {
	dashboard := r.Group("/dashboard")
	{
		// 概览
		dashboard.GET("/overview", api.GetOverview)
		dashboard.GET("/health", api.GetHealthStatus)

		// 实时数据
		dashboard.GET("/realtime", api.GetRealtimeData)
		dashboard.GET("/realtime/stream", api.StreamRealtimeData)

		// 系统监控
		dashboard.GET("/system", api.GetSystemMetrics)
		dashboard.GET("/system/history", api.GetSystemHistory)

		// 磁盘监控
		dashboard.GET("/disks", api.GetDiskMetrics)
		dashboard.GET("/disks/:device", api.GetDiskDetail)
		dashboard.GET("/disks/:device/smart", api.GetDiskSMART)
		dashboard.GET("/disks/:device/history", api.GetDiskHistory)

		// 网络监控
		dashboard.GET("/network", api.GetNetworkMetrics)
		dashboard.GET("/network/history", api.GetNetworkHistory)
		dashboard.GET("/network/interfaces", api.GetNetworkInterfaces)

		// 告警
		dashboard.GET("/alerts", api.GetAlerts)
		dashboard.GET("/alerts/stats", api.GetAlertStats)
		dashboard.GET("/alerts/history", api.GetAlertHistory)
		dashboard.POST("/alerts/:id/acknowledge", api.AcknowledgeAlert)
		dashboard.POST("/alerts/:id/resolve", api.ResolveAlert)

		// 告警规则
		dashboard.GET("/rules", api.GetAlertRules)
		dashboard.POST("/rules", api.CreateAlertRule)
		dashboard.PUT("/rules/:id", api.UpdateAlertRule)
		dashboard.DELETE("/rules/:id", api.DeleteAlertRule)
		dashboard.POST("/rules/:id/enable", api.EnableAlertRule)
		dashboard.POST("/rules/:id/disable", api.DisableAlertRule)

		// 日志
		dashboard.GET("/logs", api.QueryLogs)
		dashboard.GET("/logs/sources", api.GetLogSources)
		dashboard.GET("/logs/stats", api.GetLogStats)
	}
}

// OverviewResponse 概览响应.
type OverviewResponse struct {
	System    *SystemOverview  `json:"system"`
	Storage   *StorageOverview `json:"storage"`
	Alerts    *AlertOverview   `json:"alerts"`
	Health    *HealthOverview  `json:"health"`
	Network   *NetworkOverview `json:"network"`
	Timestamp time.Time        `json:"timestamp"`
}

// SystemOverview 系统概览.
type SystemOverview struct {
	Hostname    string    `json:"hostname"`
	Uptime      string    `json:"uptime"`
	CPUUsage    float64   `json:"cpuUsage"`
	MemoryUsage float64   `json:"memoryUsage"`
	LoadAvg     []float64 `json:"loadAvg"`
	Processes   int       `json:"processes"`
}

// StorageOverview 存储概览.
type StorageOverview struct {
	TotalDisks   int     `json:"totalDisks"`
	HealthyDisks int     `json:"healthyDisks"`
	TotalSpace   uint64  `json:"totalSpace"`
	UsedSpace    uint64  `json:"usedSpace"`
	UsagePercent float64 `json:"usagePercent"`
}

// AlertOverview 告警概览.
type AlertOverview struct {
	Total        int `json:"total"`
	Critical     int `json:"critical"`
	Warning      int `json:"warning"`
	Acknowledged int `json:"acknowledged"`
}

// HealthOverview 健康概览.
type HealthOverview struct {
	OverallScore int      `json:"overallScore"`
	Status       string   `json:"status"`
	Issues       []string `json:"issues,omitempty"`
}

// NetworkOverview 网络概览.
type NetworkOverview struct {
	Interfaces   int    `json:"interfaces"`
	TotalRXBytes uint64 `json:"totalRxBytes"`
	TotalTXBytes uint64 `json:"totalTxBytes"`
}

// GetOverview 获取概览
// @Summary 获取系统概览
// @Description 获取系统整体状态概览，包括系统资源、存储、告警、健康状态等
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /dashboard/overview [get]
// @Security BearerAuth.
func (api *DashboardAPI) GetOverview(c *gin.Context) {
	overview := &OverviewResponse{
		Timestamp: time.Now(),
	}

	// 系统概览
	if stats, err := api.manager.GetSystemStats(); err == nil {
		overview.System = &SystemOverview{
			Hostname:    api.manager.GetHostname(),
			Uptime:      stats.Uptime,
			CPUUsage:    stats.CPUUsage,
			MemoryUsage: stats.MemoryUsage,
			LoadAvg:     stats.LoadAvg,
			Processes:   stats.Processes,
		}
	}

	// 存储概览
	if disks, err := api.manager.GetDiskStats(); err == nil {
		var totalSpace, usedSpace uint64
		for _, d := range disks {
			if d.FSType != "tmpfs" && d.FSType != "devtmpfs" {
				totalSpace += d.Total
				usedSpace += d.Used
			}
		}

		var usagePercent float64
		if totalSpace > 0 {
			usagePercent = float64(usedSpace) / float64(totalSpace) * 100
		}

		diskSummary := api.diskHealthMonitor.GetDiskSummary()

		overview.Storage = &StorageOverview{
			TotalDisks:   diskSummary.TotalDisks,
			HealthyDisks: diskSummary.Healthy,
			TotalSpace:   totalSpace,
			UsedSpace:    usedSpace,
			UsagePercent: usagePercent,
		}
	}

	// 告警概览
	if stats := api.alertManager.GetAlertStats(); stats != nil {
		total, _ := stats["total"].(int)
		critical, _ := stats["critical"].(int)
		warning, _ := stats["warning"].(int)
		acknowledged, _ := stats["acknowledged"].(int)
		overview.Alerts = &AlertOverview{
			Total:        total,
			Critical:     critical,
			Warning:      warning,
			Acknowledged: acknowledged,
		}
	}

	// 健康概览
	healthOverview := &HealthOverview{
		OverallScore: 100,
		Status:       "healthy",
		Issues:       make([]string, 0),
	}

	if api.diskHealthMonitor != nil {
		summary := api.diskHealthMonitor.GetDiskSummary()
		if summary.Failed > 0 {
			healthOverview.OverallScore -= 30
			healthOverview.Status = "critical"
			healthOverview.Issues = append(healthOverview.Issues, "存在故障磁盘")
		} else if summary.Degraded > 0 {
			healthOverview.OverallScore -= 15
			healthOverview.Status = "warning"
			healthOverview.Issues = append(healthOverview.Issues, "存在性能下降的磁盘")
		}
	}

	overview.Health = healthOverview

	// 网络概览
	if netStats, err := api.manager.GetNetworkStats(); err == nil {
		var totalRX, totalTX uint64
		for _, n := range netStats {
			totalRX += n.RXBytes
			totalTX += n.TXBytes
		}

		overview.Network = &NetworkOverview{
			Interfaces:   len(netStats),
			TotalRXBytes: totalRX,
			TotalTXBytes: totalTX,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    overview,
	})
}

// GetHealthStatus 获取健康状态
// @Summary 获取健康状态
// @Description 获取系统整体健康状态
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/health [get]
// @Security BearerAuth.
func (api *DashboardAPI) GetHealthStatus(c *gin.Context) {
	status := map[string]interface{}{
		"overall":    "healthy",
		"score":      100,
		"components": make(map[string]interface{}),
		"timestamp":  time.Now(),
	}

	components, _ := status["components"].(map[string]interface{})

	// 系统健康
	if stats, err := api.manager.GetSystemStats(); err == nil {
		systemScore := 100
		if stats.CPUUsage > 90 {
			systemScore -= 20
		} else if stats.CPUUsage > 80 {
			systemScore -= 10
		}
		if stats.MemoryUsage > 95 {
			systemScore -= 25
		} else if stats.MemoryUsage > 85 {
			systemScore -= 15
		}

		components["system"] = map[string]interface{}{
			"status":      getHealthStatusFromScore(systemScore),
			"score":       systemScore,
			"cpuUsage":    stats.CPUUsage,
			"memoryUsage": stats.MemoryUsage,
		}
	}

	// 存储健康
	if api.diskHealthMonitor != nil {
		summary := api.diskHealthMonitor.GetDiskSummary()
		storageScore := 100
		if summary.Failed > 0 {
			storageScore -= 40
		}
		if summary.Degraded > 0 {
			storageScore -= 20
		}
		if summary.Warning > 0 {
			storageScore -= 10
		}

		components["storage"] = map[string]interface{}{
			"status":     getHealthStatusFromScore(storageScore),
			"score":      storageScore,
			"totalDisks": summary.TotalDisks,
			"healthy":    summary.Healthy,
			"degraded":   summary.Degraded,
			"failed":     summary.Failed,
		}
	}

	// 计算总分
	totalScore := 0
	count := 0
	for _, comp := range components {
		if m, ok := comp.(map[string]interface{}); ok {
			if score, ok := m["score"].(int); ok {
				totalScore += score
				count++
			}
		}
	}

	if count > 0 {
		status["score"] = totalScore / count
		status["overall"] = getHealthStatusFromScore(totalScore / count)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    status,
	})
}

// GetRealtimeData 获取实时数据
// @Summary 获取实时监控数据
// @Description 获取实时系统资源使用情况
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/realtime [get]
// @Security BearerAuth.
func (api *DashboardAPI) GetRealtimeData(c *gin.Context) {
	data := map[string]interface{}{
		"timestamp": time.Now(),
	}

	// 系统数据
	if stats, err := api.manager.GetSystemStats(); err == nil {
		data["system"] = stats
	}

	// 磁盘数据
	if disks, err := api.manager.GetDiskStats(); err == nil {
		data["disks"] = disks
	}

	// 网络数据
	if net, err := api.manager.GetNetworkStats(); err == nil {
		data["network"] = net
	}

	// 磁盘健康
	if api.diskHealthMonitor != nil {
		data["diskHealth"] = api.diskHealthMonitor.GetAllDisksHealth()
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    data,
	})
}

// StreamRealtimeData 实时数据流 (SSE)
// @Summary 实时数据流
// @Description 通过 Server-Sent Events 推送实时监控数据
// @Tags dashboard
// @Produce text/event-stream
// @Success 200 "SSE 流"
// @Router /dashboard/realtime/stream [get]
// @Security BearerAuth.
func (api *DashboardAPI) StreamRealtimeData(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			// 获取实时数据
			data := map[string]interface{}{
				"timestamp": time.Now(),
			}

			if stats, err := api.manager.GetSystemStats(); err == nil {
				data["cpu"] = stats.CPUUsage
				data["memory"] = stats.MemoryUsage
				data["load"] = stats.LoadAvg
			}

			// 发送 SSE 事件
			c.SSEvent("data", data)
			c.Writer.Flush()
		}
	}
}

// GetSystemMetrics 获取系统指标
// @Summary 获取系统指标
// @Description 获取 CPU、内存、负载等系统指标
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/system [get]
// @Security BearerAuth.
func (api *DashboardAPI) GetSystemMetrics(c *gin.Context) {
	stats, err := api.manager.GetSystemStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// GetSystemHistory 获取系统历史
// @Summary 获取系统历史数据
// @Description 获取系统资源使用历史数据
// @Tags dashboard
// @Produce json
// @Param period query string false "时间周期 (1h, 6h, 24h, 7d, 30d)"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/system/history [get]
// @Security BearerAuth.
func (api *DashboardAPI) GetSystemHistory(c *gin.Context) {
	period := c.DefaultQuery("period", "1h")

	// 解析时间范围
	var duration time.Duration
	switch period {
	case "1h":
		duration = time.Hour
	case "6h":
		duration = 6 * time.Hour
	case "24h":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	case "30d":
		duration = 30 * 24 * time.Hour
	default:
		duration = time.Hour
	}

	// 模拟历史数据（实际应从时序数据库获取）
	now := time.Now()
	points := make([]map[string]interface{}, 0)

	interval := duration / 60 // 60 个数据点
	for i := 0; i < 60; i++ {
		t := now.Add(-interval * time.Duration(60-i))
		points = append(points, map[string]interface{}{
			"timestamp":   t,
			"cpuUsage":    30 + float64(i%20), // 模拟数据
			"memoryUsage": 40 + float64(i%15),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"period":    period,
			"startTime": now.Add(-duration),
			"endTime":   now,
			"points":    points,
		},
	})
}

// GetDiskMetrics 获取磁盘指标
// @Summary 获取磁盘指标
// @Description 获取所有磁盘的使用情况
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/disks [get]
// @Security BearerAuth.
func (api *DashboardAPI) GetDiskMetrics(c *gin.Context) {
	stats, err := api.manager.GetDiskStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	// 获取磁盘健康信息
	healthMap := make(map[string]*DiskHealthInfo)
	if api.diskHealthMonitor != nil {
		for _, h := range api.diskHealthMonitor.GetAllDisksHealth() {
			healthMap[h.Device] = h
		}
	}

	// 合合信息
	result := make([]map[string]interface{}, 0)
	for _, d := range stats {
		item := map[string]interface{}{
			"device":       d.Device,
			"mountPoint":   d.MountPoint,
			"total":        d.Total,
			"used":         d.Used,
			"free":         d.Free,
			"usagePercent": d.UsagePercent,
			"fsType":       d.FSType,
		}

		if health, ok := healthMap[d.Device]; ok {
			item["health"] = map[string]interface{}{
				"status":      health.HealthStatus,
				"score":       health.HealthScore,
				"temperature": health.Temperature,
				"isSSD":       health.IsSSD,
			}
		}

		result = append(result, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// GetDiskDetail 获取磁盘详情
// @Summary 获取磁盘详情
// @Description 获取指定磁盘的详细信息
// @Tags dashboard
// @Produce json
// @Param device path string true "磁盘设备名"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/disks/{device} [get]
// @Security BearerAuth.
func (api *DashboardAPI) GetDiskDetail(c *gin.Context) {
	device := c.Param("device")

	// 获取使用情况
	var diskStats *DiskStats
	if stats, err := api.manager.GetDiskStats(); err == nil {
		for _, d := range stats {
			if d.Device == device {
				diskStats = d
				break
			}
		}
	}

	// 获取健康信息
	var healthInfo *DiskHealthInfo
	if api.diskHealthMonitor != nil {
		healthInfo, _ = api.diskHealthMonitor.GetDiskHealth(device)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"usage":  diskStats,
			"health": healthInfo,
		},
	})
}

// GetDiskSMART 获取磁盘 SMART 信息
// @Summary 获取磁盘 SMART 信息
// @Description 获取指定磁盘的 SMART 健康数据
// @Tags dashboard
// @Produce json
// @Param device path string true "磁盘设备名"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/disks/{device}/smart [get]
// @Security BearerAuth.
func (api *DashboardAPI) GetDiskSMART(c *gin.Context) {
	device := c.Param("device")

	if api.diskHealthMonitor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "磁盘健康监控未启用",
		})
		return
	}

	info, err := api.diskHealthMonitor.GetDiskHealth(device)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    info,
	})
}

// GetDiskHistory 获取磁盘历史
// @Summary 获取磁盘历史数据
// @Description 获取磁盘使用率和 I/O 历史数据
// @Tags dashboard
// @Produce json
// @Param device path string true "磁盘设备名"
// @Param period query string false "时间周期"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/disks/{device}/history [get]
// @Security BearerAuth.
func (api *DashboardAPI) GetDiskHistory(c *gin.Context) {
	device := c.Param("device")
	period := c.DefaultQuery("period", "24h")

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"device":  device,
			"period":  period,
			"message": "历史数据需要时序数据库支持",
		},
	})
}

// GetNetworkMetrics 获取网络指标
// @Summary 获取网络指标
// @Description 获取网络接口流量统计
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/network [get]
// @Security BearerAuth.
func (api *DashboardAPI) GetNetworkMetrics(c *gin.Context) {
	stats, err := api.manager.GetNetworkStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// GetNetworkHistory 获取网络历史
// @Summary 获取网络历史数据
// @Description 获取网络流量历史数据
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/network/history [get]
// @Security BearerAuth.
func (api *DashboardAPI) GetNetworkHistory(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"message": "历史数据需要时序数据库支持",
		},
	})
}

// GetNetworkInterfaces 获取网络接口列表
// @Summary 获取网络接口列表
// @Description 获取所有网络接口信息
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/network/interfaces [get]
// @Security BearerAuth.
func (api *DashboardAPI) GetNetworkInterfaces(c *gin.Context) {
	stats, _ := api.manager.GetNetworkStats()

	interfaces := make([]map[string]interface{}, 0, len(stats))
	for _, s := range stats {
		interfaces = append(interfaces, map[string]interface{}{
			"name":      s.Interface,
			"rxBytes":   s.RXBytes,
			"txBytes":   s.TXBytes,
			"rxPackets": s.RXPackets,
			"txPackets": s.TXPackets,
			"rxErrors":  s.RXErrors,
			"txErrors":  s.TXErrors,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    interfaces,
	})
}

// GetAlerts 获取告警列表
// @Summary 获取告警列表
// @Description 获取活动告警列表
// @Tags dashboard
// @Produce json
// @Param level query string false "告警级别"
// @Param type query string false "告警类型"
// @Param limit query int false "数量限制" default(20)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/alerts [get]
// @Security BearerAuth.
func (api *DashboardAPI) GetAlerts(c *gin.Context) {
	level := c.Query("level")
	alertType := c.Query("type")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filters := make(map[string]string)
	if level != "" {
		filters["level"] = level
	}
	if alertType != "" {
		filters["type"] = alertType
	}

	alerts := api.alertManager.GetAlerts(limit, offset, filters)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    alerts,
	})
}

// GetAlertStats 获取告警统计
// @Summary 获取告警统计
// @Description 获取告警统计数据
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/alerts/stats [get]
// @Security BearerAuth.
func (api *DashboardAPI) GetAlertStats(c *gin.Context) {
	stats := api.alertManager.GetAlertStats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// GetAlertHistory 获取告警历史
// @Summary 获取告警历史
// @Description 获取告警历史记录
// @Tags dashboard
// @Produce json
// @Param limit query int false "数量限制" default(50)
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/alerts/history [get]
// @Security BearerAuth.
func (api *DashboardAPI) GetAlertHistory(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	history := api.alertManager.GetAlertHistory(limit, offset)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    history,
	})
}

// AcknowledgeAlert 确认告警
// @Summary 确认告警
// @Description 确认指定告警
// @Tags dashboard
// @Param id path string true "告警 ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/alerts/{id}/acknowledge [post]
// @Security BearerAuth.
func (api *DashboardAPI) AcknowledgeAlert(c *gin.Context) {
	id := c.Param("id")

	// 获取用户信息（从上下文）
	user := "system"
	if u, exists := c.Get("username"); exists {
		if username, ok := u.(string); ok {
			user = username
		}
	}

	if err := api.alertManager.AcknowledgeAlert(id, user); err != nil {
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

// ResolveAlert 解决告警
// @Summary 解决告警
// @Description 标记告警为已解决
// @Tags dashboard
// @Param id path string true "告警 ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/alerts/{id}/resolve [post]
// @Security BearerAuth.
func (api *DashboardAPI) ResolveAlert(c *gin.Context) {
	id := c.Param("id")

	if err := api.alertManager.ResolveAlert(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "告警已解决",
	})
}

// GetAlertRules 获取告警规则
// @Summary 获取告警规则
// @Description 获取所有告警规则
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/rules [get]
// @Security BearerAuth.
func (api *DashboardAPI) GetAlertRules(c *gin.Context) {
	rules := api.ruleEngine.GetRules()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rules,
	})
}

// CreateAlertRule 创建告警规则
// @Summary 创建告警规则
// @Description 创建新的告警规则
// @Tags dashboard
// @Accept json
// @Produce json
// @Param rule body AlertRuleConfig true "告警规则"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/rules [post]
// @Security BearerAuth.
func (api *DashboardAPI) CreateAlertRule(c *gin.Context) {
	var rule AlertRuleConfig
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := api.ruleEngine.AddRule(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则创建成功",
		"data":    rule,
	})
}

// UpdateAlertRule 更新告警规则
// @Summary 更新告警规则
// @Description 更新指定告警规则
// @Tags dashboard
// @Accept json
// @Produce json
// @Param id path string true "规则 ID"
// @Param rule body AlertRuleConfig true "告警规则"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/rules/{id} [put]
// @Security BearerAuth.
func (api *DashboardAPI) UpdateAlertRule(c *gin.Context) {
	id := c.Param("id")

	var rule AlertRuleConfig
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	rule.ID = id

	if err := api.ruleEngine.UpdateRule(&rule); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则更新成功",
	})
}

// DeleteAlertRule 删除告警规则
// @Summary 删除告警规则
// @Description 删除指定告警规则
// @Tags dashboard
// @Param id path string true "规则 ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/rules/{id} [delete]
// @Security BearerAuth.
func (api *DashboardAPI) DeleteAlertRule(c *gin.Context) {
	id := c.Param("id")

	if err := api.ruleEngine.DeleteRule(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则删除成功",
	})
}

// EnableAlertRule 启用告警规则
// @Summary 启用告警规则
// @Description 启用指定告警规则
// @Tags dashboard
// @Param id path string true "规则 ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/rules/{id}/enable [post]
// @Security BearerAuth.
func (api *DashboardAPI) EnableAlertRule(c *gin.Context) {
	id := c.Param("id")

	if err := api.ruleEngine.EnableRule(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则已启用",
	})
}

// DisableAlertRule 禁用告警规则
// @Summary 禁用告警规则
// @Description 禁用指定告警规则
// @Tags dashboard
// @Param id path string true "规则 ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/rules/{id}/disable [post]
// @Security BearerAuth.
func (api *DashboardAPI) DisableAlertRule(c *gin.Context) {
	id := c.Param("id")

	if err := api.ruleEngine.DisableRule(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则已禁用",
	})
}

// QueryLogs 查询日志
// @Summary 查询日志
// @Description 查询系统日志
// @Tags dashboard
// @Produce json
// @Param level query string false "日志级别"
// @Param source query string false "日志来源"
// @Param message query string false "消息关键词"
// @Param startTime query string false "开始时间 (RFC3339)"
// @Param endTime query string false "结束时间 (RFC3339)"
// @Param limit query int false "数量限制" default(100)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/logs [get]
// @Security BearerAuth.
func (api *DashboardAPI) QueryLogs(c *gin.Context) {
	if api.logCollector == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "日志收集未启用",
		})
		return
	}

	filter := &LogQueryFilter{
		Level:   c.Query("level"),
		Source:  c.Query("source"),
		Message: c.Query("message"),
	}

	if startTime := c.Query("startTime"); startTime != "" {
		t, err := time.Parse(time.RFC3339, startTime)
		if err == nil {
			filter.StartTime = &t
		}
	}

	if endTime := c.Query("endTime"); endTime != "" {
		t, err := time.Parse(time.RFC3339, endTime)
		if err == nil {
			filter.EndTime = &t
		}
	}

	filter.Limit, _ = strconv.Atoi(c.DefaultQuery("limit", "100"))
	filter.Offset, _ = strconv.Atoi(c.DefaultQuery("offset", "0"))
	filter.Order = "desc"

	entries, total, err := api.logCollector.Query(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"entries": entries,
			"total":   total,
			"limit":   filter.Limit,
			"offset":  filter.Offset,
		},
	})
}

// GetLogSources 获取日志源
// @Summary 获取日志源列表
// @Description 获取所有日志源
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/logs/sources [get]
// @Security BearerAuth.
func (api *DashboardAPI) GetLogSources(c *gin.Context) {
	if api.logCollector == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "日志收集未启用",
		})
		return
	}

	sources := api.logCollector.GetSources()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    sources,
	})
}

// GetLogStats 获取日志统计
// @Summary 获取日志统计
// @Description 获取日志收集统计信息
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /dashboard/logs/stats [get]
// @Security BearerAuth.
func (api *DashboardAPI) GetLogStats(c *gin.Context) {
	if api.logCollector == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "日志收集未启用",
		})
		return
	}

	stats := api.logCollector.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// getHealthStatusFromScore 根据分数获取健康状态.
func getHealthStatusFromScore(score int) string {
	if score >= 90 {
		return "healthy"
	} else if score >= 70 {
		return "warning"
	} else if score >= 40 {
		return "degraded"
	}
	return "critical"
}
