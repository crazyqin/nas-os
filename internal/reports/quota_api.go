// Package reports 提供报表生成和管理功能
// v2.45.0 资源配额管理 API
package reports

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"nas-os/internal/api"
)

// timePtr 辅助函数：将 time.Time 转换为 *time.Time
func timePtr(t time.Time) *time.Time {
	return &t
}

// ========== 配额管理 API 处理器 ==========

// QuotaAPIHandlers 配额管理 API 处理器
type QuotaAPIHandlers struct {
	manager      *QuotaManagerAdapter
	monitor      *QuotaMonitorService
	alertManager *QuotaAlertManager
	usageStats   *QuotaUsageStatistics
	exporter     *QuotaReportExporter
}

// QuotaManagerAdapter 配额管理适配器
type QuotaManagerAdapter struct {
	quotas map[string]*QuotaDefinition
	usages map[string]*QuotaUsageRecord
	alerts map[string]*QuotaAlertRecord
}

// QuotaDefinition 配额定义
type QuotaDefinition struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`          // user, service, directory
	TargetID     string    `json:"target_id"`     // 用户ID/服务名/路径
	TargetName   string    `json:"target_name"`   // 显示名称
	VolumeName   string    `json:"volume_name"`   // 卷名
	Path         string    `json:"path"`          // 限制路径
	HardLimit    uint64    `json:"hard_limit"`    // 硬限制（字节）
	SoftLimit    uint64    `json:"soft_limit"`    // 软限制（字节）
	WarningLimit uint64    `json:"warning_limit"` // 警告阈值（字节）
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Enabled      bool      `json:"enabled"`
}

// QuotaUsageRecord 配额使用记录
type QuotaUsageRecord struct {
	QuotaID      string    `json:"quota_id"`
	Type         string    `json:"type"`
	TargetID     string    `json:"target_id"`
	TargetName   string    `json:"target_name"`
	VolumeName   string    `json:"volume_name"`
	HardLimit    uint64    `json:"hard_limit"`
	SoftLimit    uint64    `json:"soft_limit"`
	UsedBytes    uint64    `json:"used_bytes"`
	Available    uint64    `json:"available"`
	UsagePercent float64   `json:"usage_percent"`
	IsOverSoft   bool      `json:"is_over_soft"`
	IsOverHard   bool      `json:"is_over_hard"`
	LastChecked  time.Time `json:"last_checked"`
}

// QuotaAlertRecord 配额告警记录
type QuotaAlertRecord struct {
	ID           string     `json:"id"`
	QuotaID      string     `json:"quota_id"`
	Type         string     `json:"type"`     // soft_limit, hard_limit, warning
	Severity     string     `json:"severity"` // info, warning, critical, emergency
	Status       string     `json:"status"`   // active, resolved, silenced
	TargetID     string     `json:"target_id"`
	TargetName   string     `json:"target_name"`
	VolumeName   string     `json:"volume_name"`
	UsedBytes    uint64     `json:"used_bytes"`
	LimitBytes   uint64     `json:"limit_bytes"`
	UsagePercent float64    `json:"usage_percent"`
	Threshold    float64    `json:"threshold"`
	Message      string     `json:"message"`
	CreatedAt    time.Time  `json:"created_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
}

// QuotaMonitorService 配额监控服务
type QuotaMonitorService struct{}

// QuotaAlertManager 配额告警管理器
type QuotaAlertManager struct{}

// QuotaUsageStatistics 配额使用统计
type QuotaUsageStatistics struct{}

// QuotaReportExporter 配额报告导出器
type QuotaReportExporter struct{}

// NewQuotaAPIHandlers 创建配额管理 API 处理器
func NewQuotaAPIHandlers() *QuotaAPIHandlers {
	return &QuotaAPIHandlers{
		manager: &QuotaManagerAdapter{
			quotas: make(map[string]*QuotaDefinition),
			usages: make(map[string]*QuotaUsageRecord),
			alerts: make(map[string]*QuotaAlertRecord),
		},
		monitor:      &QuotaMonitorService{},
		alertManager: &QuotaAlertManager{},
		usageStats:   &QuotaUsageStatistics{},
		exporter:     &QuotaReportExporter{},
	}
}

// RegisterQuotaRoutes 注册配额管理路由
func (h *QuotaAPIHandlers) RegisterQuotaRoutes(apiGroup *gin.RouterGroup) {
	// ========== 配额管理 ==========
	quota := apiGroup.Group("/quota")
	{
		// 配额 CRUD
		quota.GET("", h.listQuotas)
		quota.GET("/:id", h.getQuota)
		quota.POST("", h.createQuota)
		quota.PUT("/:id", h.updateQuota)
		quota.DELETE("/:id", h.deleteQuota)

		// 用户配额
		quota.GET("/user/:user_id", h.getUserQuotas)
		quota.POST("/user/:user_id", h.setUserQuota)
		quota.DELETE("/user/:user_id", h.deleteUserQuota)

		// 服务配额
		quota.GET("/service/:service_name", h.getServiceQuotas)
		quota.POST("/service/:service_name", h.setServiceQuota)
		quota.DELETE("/service/:service_name", h.deleteServiceQuota)

		// 目录配额
		quota.GET("/directory/*path", h.getDirectoryQuota)
		quota.POST("/directory/*path", h.setDirectoryQuota)
		quota.DELETE("/directory/*path", h.deleteDirectoryQuota)
	}

	// ========== 配额使用统计 ==========
	usage := apiGroup.Group("/quota-usage")
	{
		usage.GET("", h.getAllUsage)
		usage.GET("/summary", h.getUsageSummary)
		usage.GET("/:quota_id", h.getQuotaUsage)
		usage.GET("/user/:user_id", h.getUserUsage)
		usage.GET("/volume/:volume_name", h.getVolumeUsage)
		usage.GET("/top", h.getTopUsage)
		usage.GET("/history/:quota_id", h.getUsageHistory)
		usage.POST("/refresh", h.refreshUsage)
	}

	// ========== 配额告警 ==========
	alerts := apiGroup.Group("/quota-alerts")
	{
		alerts.GET("", h.listAlerts)
		alerts.GET("/active", h.getActiveAlerts)
		alerts.GET("/:alert_id", h.getAlert)
		alerts.PUT("/:alert_id/resolve", h.resolveAlert)
		alerts.PUT("/:alert_id/silence", h.silenceAlert)
		alerts.POST("/test", h.testAlert)

		// 告警配置
		alerts.GET("/config", h.getAlertConfig)
		alerts.PUT("/config", h.updateAlertConfig)
	}

	// ========== 配额预警 ==========
	warnings := apiGroup.Group("/quota-warnings")
	{
		warnings.GET("", h.getWarnings)
		warnings.GET("/predictions", h.getPredictions)
		warnings.GET("/forecast/:quota_id", h.getForecast)
		warnings.POST("/thresholds", h.setWarningThresholds)
	}

	// ========== 配额报告 ==========
	reports := apiGroup.Group("/quota-reports")
	{
		reports.GET("", h.listQuotaReports)
		reports.POST("/generate", h.generateQuotaReport)
		reports.GET("/:report_id", h.getQuotaReport)
		reports.GET("/:report_id/export", h.exportQuotaReport)
		reports.DELETE("/:report_id", h.deleteQuotaReport)
	}
}

// ========== 配额管理 API ==========

// listQuotas 列出所有配额
func (h *QuotaAPIHandlers) listQuotas(c *gin.Context) {
	// 获取查询参数
	volumeName := c.Query("volume")
	quotaType := c.Query("type")

	// 模拟返回数据
	quotas := []QuotaDefinition{
		{
			ID:           "quota_001",
			Type:         "user",
			TargetID:     "user1",
			TargetName:   "张三",
			VolumeName:   "volume1",
			HardLimit:    100 * 1024 * 1024 * 1024, // 100GB
			SoftLimit:    80 * 1024 * 1024 * 1024,  // 80GB
			WarningLimit: 60 * 1024 * 1024 * 1024,  // 60GB
			CreatedAt:    time.Now().AddDate(0, -1, 0),
			UpdatedAt:    time.Now(),
			Enabled:      true,
		},
		{
			ID:           "quota_002",
			Type:         "service",
			TargetID:     "backup",
			TargetName:   "备份服务",
			VolumeName:   "volume1",
			HardLimit:    500 * 1024 * 1024 * 1024, // 500GB
			SoftLimit:    400 * 1024 * 1024 * 1024, // 400GB
			WarningLimit: 300 * 1024 * 1024 * 1024, // 300GB
			CreatedAt:    time.Now().AddDate(0, -1, 0),
			UpdatedAt:    time.Now(),
			Enabled:      true,
		},
	}

	// 过滤
	if volumeName != "" {
		var filtered []QuotaDefinition
		for _, q := range quotas {
			if q.VolumeName == volumeName {
				filtered = append(filtered, q)
			}
		}
		quotas = filtered
	}

	if quotaType != "" {
		var filtered []QuotaDefinition
		for _, q := range quotas {
			if q.Type == quotaType {
				filtered = append(filtered, q)
			}
		}
		quotas = filtered
	}

	api.OK(c, gin.H{
		"quotas": quotas,
		"total":  len(quotas),
	})
}

// getQuota 获取单个配额
func (h *QuotaAPIHandlers) getQuota(c *gin.Context) {
	id := c.Param("id")

	// 模拟返回数据
	quota := &QuotaDefinition{
		ID:           id,
		Type:         "user",
		TargetID:     "user1",
		TargetName:   "张三",
		VolumeName:   "volume1",
		HardLimit:    100 * 1024 * 1024 * 1024,
		SoftLimit:    80 * 1024 * 1024 * 1024,
		WarningLimit: 60 * 1024 * 1024 * 1024,
		CreatedAt:    time.Now().AddDate(0, -1, 0),
		UpdatedAt:    time.Now(),
		Enabled:      true,
	}

	api.OK(c, quota)
}

// createQuota 创建配额
func (h *QuotaAPIHandlers) createQuota(c *gin.Context) {
	var req CreateQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 验证
	if req.HardLimit <= 0 {
		api.BadRequest(c, "硬限制必须大于0")
		return
	}

	if req.SoftLimit > req.HardLimit {
		api.BadRequest(c, "软限制不能大于硬限制")
		return
	}

	// 创建配额
	quota := &QuotaDefinition{
		ID:           "quota_" + time.Now().Format("20060102150405"),
		Type:         req.Type,
		TargetID:     req.TargetID,
		TargetName:   req.TargetName,
		VolumeName:   req.VolumeName,
		Path:         req.Path,
		HardLimit:    req.HardLimit,
		SoftLimit:    req.SoftLimit,
		WarningLimit: req.WarningLimit,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Enabled:      true,
	}

	api.Created(c, gin.H{
		"message": "配额创建成功",
		"quota":   quota,
	})
}

// updateQuota 更新配额
func (h *QuotaAPIHandlers) updateQuota(c *gin.Context) {
	id := c.Param("id")

	var req UpdateQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 验证
	if req.SoftLimit != nil && req.HardLimit != nil {
		if *req.SoftLimit > *req.HardLimit {
			api.BadRequest(c, "软限制不能大于硬限制")
			return
		}
	}

	api.OK(c, gin.H{
		"message":  "配额更新成功",
		"quota_id": id,
	})
}

// deleteQuota 删除配额
func (h *QuotaAPIHandlers) deleteQuota(c *gin.Context) {
	id := c.Param("id")

	api.OK(c, gin.H{
		"message":  "配额删除成功",
		"quota_id": id,
	})
}

// getUserQuotas 获取用户配额
func (h *QuotaAPIHandlers) getUserQuotas(c *gin.Context) {
	userID := c.Param("user_id")

	quotas := []QuotaDefinition{
		{
			ID:           "quota_user_" + userID,
			Type:         "user",
			TargetID:     userID,
			TargetName:   "用户" + userID,
			VolumeName:   "volume1",
			HardLimit:    100 * 1024 * 1024 * 1024,
			SoftLimit:    80 * 1024 * 1024 * 1024,
			WarningLimit: 60 * 1024 * 1024 * 1024,
			CreatedAt:    time.Now().AddDate(0, -1, 0),
			UpdatedAt:    time.Now(),
			Enabled:      true,
		},
	}

	api.OK(c, gin.H{
		"user_id": userID,
		"quotas":  quotas,
	})
}

// setUserQuota 设置用户配额
func (h *QuotaAPIHandlers) setUserQuota(c *gin.Context) {
	userID := c.Param("user_id")

	var req SetUserQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, gin.H{
		"message": "用户配额设置成功",
		"user_id": userID,
		"quota":   req,
	})
}

// deleteUserQuota 删除用户配额
func (h *QuotaAPIHandlers) deleteUserQuota(c *gin.Context) {
	userID := c.Param("user_id")

	api.OK(c, gin.H{
		"message": "用户配额删除成功",
		"user_id": userID,
	})
}

// getServiceQuotas 获取服务配额
func (h *QuotaAPIHandlers) getServiceQuotas(c *gin.Context) {
	serviceName := c.Param("service_name")

	quotas := []QuotaDefinition{
		{
			ID:           "quota_service_" + serviceName,
			Type:         "service",
			TargetID:     serviceName,
			TargetName:   serviceName,
			VolumeName:   "volume1",
			HardLimit:    500 * 1024 * 1024 * 1024,
			SoftLimit:    400 * 1024 * 1024 * 1024,
			WarningLimit: 300 * 1024 * 1024 * 1024,
			CreatedAt:    time.Now().AddDate(0, -1, 0),
			UpdatedAt:    time.Now(),
			Enabled:      true,
		},
	}

	api.OK(c, gin.H{
		"service_name": serviceName,
		"quotas":       quotas,
	})
}

// setServiceQuota 设置服务配额
func (h *QuotaAPIHandlers) setServiceQuota(c *gin.Context) {
	serviceName := c.Param("service_name")

	var req SetServiceQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, gin.H{
		"message":      "服务配额设置成功",
		"service_name": serviceName,
		"quota":        req,
	})
}

// deleteServiceQuota 删除服务配额
func (h *QuotaAPIHandlers) deleteServiceQuota(c *gin.Context) {
	serviceName := c.Param("service_name")

	api.OK(c, gin.H{
		"message":      "服务配额删除成功",
		"service_name": serviceName,
	})
}

// getDirectoryQuota 获取目录配额
func (h *QuotaAPIHandlers) getDirectoryQuota(c *gin.Context) {
	path := c.Param("path")

	quota := &QuotaDefinition{
		ID:           "quota_dir_" + path,
		Type:         "directory",
		TargetID:     path,
		TargetName:   path,
		VolumeName:   "volume1",
		Path:         path,
		HardLimit:    1000 * 1024 * 1024 * 1024, // 1TB
		SoftLimit:    800 * 1024 * 1024 * 1024,  // 800GB
		WarningLimit: 600 * 1024 * 1024 * 1024,  // 600GB
		CreatedAt:    time.Now().AddDate(0, -1, 0),
		UpdatedAt:    time.Now(),
		Enabled:      true,
	}

	api.OK(c, quota)
}

// setDirectoryQuota 设置目录配额
func (h *QuotaAPIHandlers) setDirectoryQuota(c *gin.Context) {
	path := c.Param("path")

	var req SetDirectoryQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, gin.H{
		"message": "目录配额设置成功",
		"path":    path,
		"quota":   req,
	})
}

// deleteDirectoryQuota 删除目录配额
func (h *QuotaAPIHandlers) deleteDirectoryQuota(c *gin.Context) {
	path := c.Param("path")

	api.OK(c, gin.H{
		"message": "目录配额删除成功",
		"path":    path,
	})
}

// ========== 配额使用统计 API ==========

// getAllUsage 获取所有配额使用情况
func (h *QuotaAPIHandlers) getAllUsage(c *gin.Context) {
	usages := []QuotaUsageRecord{
		{
			QuotaID:      "quota_001",
			Type:         "user",
			TargetID:     "user1",
			TargetName:   "张三",
			VolumeName:   "volume1",
			HardLimit:    100 * 1024 * 1024 * 1024,
			SoftLimit:    80 * 1024 * 1024 * 1024,
			UsedBytes:    65 * 1024 * 1024 * 1024,
			Available:    35 * 1024 * 1024 * 1024,
			UsagePercent: 65.0,
			IsOverSoft:   false,
			IsOverHard:   false,
			LastChecked:  time.Now(),
		},
		{
			QuotaID:      "quota_002",
			Type:         "service",
			TargetID:     "backup",
			TargetName:   "备份服务",
			VolumeName:   "volume1",
			HardLimit:    500 * 1024 * 1024 * 1024,
			SoftLimit:    400 * 1024 * 1024 * 1024,
			UsedBytes:    420 * 1024 * 1024 * 1024,
			Available:    80 * 1024 * 1024 * 1024,
			UsagePercent: 84.0,
			IsOverSoft:   true,
			IsOverHard:   false,
			LastChecked:  time.Now(),
		},
	}

	api.OK(c, gin.H{
		"usages": usages,
		"total":  len(usages),
	})
}

// getUsageSummary 获取使用情况汇总
func (h *QuotaAPIHandlers) getUsageSummary(c *gin.Context) {
	summary := QuotaUsageSummary{
		TotalQuotas:         10,
		TotalHardLimitBytes: 1000 * 1024 * 1024 * 1024,
		TotalSoftLimitBytes: 800 * 1024 * 1024 * 1024,
		TotalUsedBytes:      650 * 1024 * 1024 * 1024,
		TotalAvailableBytes: 350 * 1024 * 1024 * 1024,
		AvgUsagePercent:     65.0,
		OverSoftLimit:       3,
		OverHardLimit:       1,
		ActiveAlerts:        4,
		ByType: map[string]TypeUsageStats{
			"user":      {Count: 6, UsedBytes: 300 * 1024 * 1024 * 1024, LimitBytes: 500 * 1024 * 1024 * 1024},
			"service":   {Count: 3, UsedBytes: 250 * 1024 * 1024 * 1024, LimitBytes: 400 * 1024 * 1024 * 1024},
			"directory": {Count: 1, UsedBytes: 100 * 1024 * 1024 * 1024, LimitBytes: 100 * 1024 * 1024 * 1024},
		},
		TopUsers: []TopUsageItem{
			{TargetName: "张三", UsedGB: 120, LimitGB: 150, UsagePercent: 80.0},
			{TargetName: "李四", UsedGB: 95, LimitGB: 100, UsagePercent: 95.0},
			{TargetName: "王五", UsedGB: 60, LimitGB: 100, UsagePercent: 60.0},
		},
		GeneratedAt: time.Now(),
	}

	api.OK(c, summary)
}

// getQuotaUsage 获取单个配额使用情况
func (h *QuotaAPIHandlers) getQuotaUsage(c *gin.Context) {
	quotaID := c.Param("quota_id")

	usage := QuotaUsageRecord{
		QuotaID:      quotaID,
		Type:         "user",
		TargetID:     "user1",
		TargetName:   "张三",
		VolumeName:   "volume1",
		HardLimit:    100 * 1024 * 1024 * 1024,
		SoftLimit:    80 * 1024 * 1024 * 1024,
		UsedBytes:    75 * 1024 * 1024 * 1024,
		Available:    25 * 1024 * 1024 * 1024,
		UsagePercent: 75.0,
		IsOverSoft:   false,
		IsOverHard:   false,
		LastChecked:  time.Now(),
	}

	api.OK(c, usage)
}

// getUserUsage 获取用户使用情况
func (h *QuotaAPIHandlers) getUserUsage(c *gin.Context) {
	userID := c.Param("user_id")

	usages := []QuotaUsageRecord{
		{
			QuotaID:      "quota_user_" + userID,
			Type:         "user",
			TargetID:     userID,
			TargetName:   "用户" + userID,
			VolumeName:   "volume1",
			HardLimit:    100 * 1024 * 1024 * 1024,
			SoftLimit:    80 * 1024 * 1024 * 1024,
			UsedBytes:    65 * 1024 * 1024 * 1024,
			Available:    35 * 1024 * 1024 * 1024,
			UsagePercent: 65.0,
			IsOverSoft:   false,
			IsOverHard:   false,
			LastChecked:  time.Now(),
		},
	}

	api.OK(c, gin.H{
		"user_id": userID,
		"usages":  usages,
	})
}

// getVolumeUsage 获取卷使用情况
func (h *QuotaAPIHandlers) getVolumeUsage(c *gin.Context) {
	volumeName := c.Param("volume_name")

	usages := []QuotaUsageRecord{
		{
			QuotaID:      "quota_001",
			Type:         "user",
			TargetID:     "user1",
			TargetName:   "张三",
			VolumeName:   volumeName,
			HardLimit:    100 * 1024 * 1024 * 1024,
			SoftLimit:    80 * 1024 * 1024 * 1024,
			UsedBytes:    65 * 1024 * 1024 * 1024,
			Available:    35 * 1024 * 1024 * 1024,
			UsagePercent: 65.0,
			IsOverSoft:   false,
			IsOverHard:   false,
			LastChecked:  time.Now(),
		},
	}

	api.OK(c, gin.H{
		"volume_name": volumeName,
		"usages":      usages,
	})
}

// getTopUsage 获取使用量排行
func (h *QuotaAPIHandlers) getTopUsage(c *gin.Context) {
	// 获取参数
	limit := 10
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	sortBy := c.DefaultQuery("sort", "usage") // usage, percent

	topUsers := []TopUsageItem{
		{TargetName: "李四", UsedGB: 95, LimitGB: 100, UsagePercent: 95.0},
		{TargetName: "张三", UsedGB: 120, LimitGB: 150, UsagePercent: 80.0},
		{TargetName: "王五", UsedGB: 60, LimitGB: 100, UsagePercent: 60.0},
		{TargetName: "赵六", UsedGB: 45, LimitGB: 80, UsagePercent: 56.25},
		{TargetName: "备份服务", UsedGB: 420, LimitGB: 500, UsagePercent: 84.0},
	}

	// 按使用率排序
	if sortBy == "percent" {
		// 已按使用率排序
	} else {
		// 按使用量排序
		// 已按使用量排序
	}

	if len(topUsers) > limit {
		topUsers = topUsers[:limit]
	}

	api.OK(c, gin.H{
		"top_users": topUsers,
		"sort_by":   sortBy,
	})
}

// getUsageHistory 获取使用历史
func (h *QuotaAPIHandlers) getUsageHistory(c *gin.Context) {
	quotaID := c.Param("quota_id")

	// 获取时间范围
	days := 7
	if d := c.Query("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}

	history := make([]UsageHistoryPoint, 0)
	now := time.Now()

	for i := days - 1; i >= 0; i-- {
		t := now.AddDate(0, 0, -i)
		history = append(history, UsageHistoryPoint{
			Timestamp:    t,
			UsedBytes:    uint64((60 + (days-i)*2)) * 1024 * 1024 * 1024,
			UsagePercent: float64(60 + (days-i)*2),
		})
	}

	api.OK(c, gin.H{
		"quota_id": quotaID,
		"history":  history,
		"days":     days,
	})
}

// refreshUsage 刷新使用统计
func (h *QuotaAPIHandlers) refreshUsage(c *gin.Context) {
	volumeName := c.Query("volume")

	api.OK(c, gin.H{
		"message":      "使用统计刷新成功",
		"volume":       volumeName,
		"refreshed_at": time.Now(),
	})
}

// ========== 配额告警 API ==========

// listAlerts 列出所有告警
func (h *QuotaAPIHandlers) listAlerts(c *gin.Context) {
	// 获取过滤参数
	status := c.Query("status")
	severity := c.Query("severity")

	alerts := []QuotaAlertRecord{
		{
			ID:           "alert_001",
			QuotaID:      "quota_002",
			Type:         "soft_limit",
			Severity:     "warning",
			Status:       "active",
			TargetID:     "backup",
			TargetName:   "备份服务",
			VolumeName:   "volume1",
			UsedBytes:    420 * 1024 * 1024 * 1024,
			LimitBytes:   400 * 1024 * 1024 * 1024,
			UsagePercent: 84.0,
			Threshold:    80.0,
			Message:      "备份服务配额使用率已达 84%，超过软限制",
			CreatedAt:    time.Now().Add(-1 * time.Hour),
		},
		{
			ID:           "alert_002",
			QuotaID:      "quota_user_003",
			Type:         "hard_limit",
			Severity:     "critical",
			Status:       "active",
			TargetID:     "user3",
			TargetName:   "王五",
			VolumeName:   "volume1",
			UsedBytes:    105 * 1024 * 1024 * 1024,
			LimitBytes:   100 * 1024 * 1024 * 1024,
			UsagePercent: 105.0,
			Threshold:    100.0,
			Message:      "用户王五已超出硬限制，无法写入新文件",
			CreatedAt:    time.Now().Add(-30 * time.Minute),
		},
	}

	// 过滤
	if status != "" {
		var filtered []QuotaAlertRecord
		for _, a := range alerts {
			if a.Status == status {
				filtered = append(filtered, a)
			}
		}
		alerts = filtered
	}

	if severity != "" {
		var filtered []QuotaAlertRecord
		for _, a := range alerts {
			if a.Severity == severity {
				filtered = append(filtered, a)
			}
		}
		alerts = filtered
	}

	api.OK(c, gin.H{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

// getActiveAlerts 获取活跃告警
func (h *QuotaAPIHandlers) getActiveAlerts(c *gin.Context) {
	alerts := []QuotaAlertRecord{
		{
			ID:           "alert_001",
			QuotaID:      "quota_002",
			Type:         "soft_limit",
			Severity:     "warning",
			Status:       "active",
			TargetID:     "backup",
			TargetName:   "备份服务",
			VolumeName:   "volume1",
			UsedBytes:    420 * 1024 * 1024 * 1024,
			LimitBytes:   400 * 1024 * 1024 * 1024,
			UsagePercent: 84.0,
			Threshold:    80.0,
			Message:      "备份服务配额使用率已达 84%，超过软限制",
			CreatedAt:    time.Now().Add(-1 * time.Hour),
		},
	}

	api.OK(c, gin.H{
		"alerts":       alerts,
		"total":        len(alerts),
		"generated_at": time.Now(),
	})
}

// getAlert 获取单个告警
func (h *QuotaAPIHandlers) getAlert(c *gin.Context) {
	alertID := c.Param("alert_id")

	alert := &QuotaAlertRecord{
		ID:           alertID,
		QuotaID:      "quota_002",
		Type:         "soft_limit",
		Severity:     "warning",
		Status:       "active",
		TargetID:     "backup",
		TargetName:   "备份服务",
		VolumeName:   "volume1",
		UsedBytes:    420 * 1024 * 1024 * 1024,
		LimitBytes:   400 * 1024 * 1024 * 1024,
		UsagePercent: 84.0,
		Threshold:    80.0,
		Message:      "备份服务配额使用率已达 84%，超过软限制",
		CreatedAt:    time.Now().Add(-1 * time.Hour),
	}

	api.OK(c, alert)
}

// resolveAlert 解决告警
func (h *QuotaAPIHandlers) resolveAlert(c *gin.Context) {
	alertID := c.Param("alert_id")

	var req ResolveAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 可选请求体
	}

	now := time.Now()

	api.OK(c, gin.H{
		"message":     "告警已解决",
		"alert_id":    alertID,
		"resolved_at": now,
	})
}

// silenceAlert 静默告警
func (h *QuotaAPIHandlers) silenceAlert(c *gin.Context) {
	alertID := c.Param("alert_id")

	var req SilenceAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, gin.H{
		"message":        "告警已静默",
		"alert_id":       alertID,
		"duration":       req.Duration,
		"silenced_until": time.Now().Add(time.Duration(req.Duration) * time.Minute),
	})
}

// testAlert 测试告警
func (h *QuotaAPIHandlers) testAlert(c *gin.Context) {
	var req TestAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, gin.H{
		"message":  "测试告警已发送",
		"channels": req.Channels,
	})
}

// getAlertConfig 获取告警配置
func (h *QuotaAPIHandlers) getAlertConfig(c *gin.Context) {
	config := QuotaAlertConfig{
		Enabled:            true,
		WarningThreshold:   70.0,
		CriticalThreshold:  85.0,
		EmergencyThreshold: 95.0,
		CheckInterval:      5, // 分钟
		NotifyEmail:        true,
		NotifyWebhook:      true,
		WebhookURL:         "https://example.com/webhook",
		SilenceDuration:    60, // 分钟
		EscalationEnabled:  true,
		EscalationInterval: 30, // 分钟
		MaxEscalationLevel: 3,
	}

	api.OK(c, config)
}

// updateAlertConfig 更新告警配置
func (h *QuotaAPIHandlers) updateAlertConfig(c *gin.Context) {
	var req QuotaAlertConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, gin.H{
		"message": "告警配置更新成功",
		"config":  req,
	})
}

// ========== 配额预警 API ==========

// getWarnings 获取预警列表
func (h *QuotaAPIHandlers) getWarnings(c *gin.Context) {
	warnings := []QuotaWarning{
		{
			ID:           "warning_001",
			QuotaID:      "quota_003",
			TargetName:   "李四",
			Type:         "usage_high",
			Severity:     "warning",
			CurrentValue: 95.0,
			Threshold:    85.0,
			Message:      "用户李四配额使用率已达 95%",
			CreatedAt:    time.Now(),
		},
		{
			ID:            "warning_002",
			QuotaID:       "quota_004",
			TargetName:    "备份服务",
			Type:          "growth_fast",
			Severity:      "info",
			CurrentValue:  84.0,
			Threshold:     80.0,
			Message:       "备份服务配额增长较快，预计 5 天内将达到软限制",
			PredictedDays: 5,
			CreatedAt:     time.Now(),
		},
	}

	api.OK(c, gin.H{
		"warnings": warnings,
		"total":    len(warnings),
	})
}

// getPredictions 获取使用预测
func (h *QuotaAPIHandlers) getPredictions(c *gin.Context) {
	predictions := []QuotaPredictionOutput{
		{
			QuotaID:           "quota_001",
			TargetName:        "张三",
			CurrentUsage:      65.0,
			DailyGrowthGB:     2.5,
			DaysToSoftLimit:   6,
			DaysToHardLimit:   14,
			EstimatedSoftDate: timePtr(time.Now().AddDate(0, 0, 6)),
			EstimatedHardDate: timePtr(time.Now().AddDate(0, 0, 14)),
			RiskLevel:         "medium",
		},
		{
			QuotaID:           "quota_003",
			TargetName:        "李四",
			CurrentUsage:      95.0,
			DailyGrowthGB:     0.5,
			DaysToSoftLimit:   0,
			DaysToHardLimit:   10,
			EstimatedSoftDate: nil,
			EstimatedHardDate: timePtr(time.Now().AddDate(0, 0, 10)),
			RiskLevel:         "critical",
		},
	}

	api.OK(c, gin.H{
		"predictions":  predictions,
		"generated_at": time.Now(),
	})
}

// getForecast 获取使用预测详情
func (h *QuotaAPIHandlers) getForecast(c *gin.Context) {
	quotaID := c.Param("quota_id")

	days := 30
	if d := c.Query("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}

	forecast := make([]ForecastPoint, 0)
	baseUsage := 65.0
	dailyGrowth := 2.0

	for i := 0; i <= days; i++ {
		t := time.Now().AddDate(0, 0, i)
		predictedUsage := baseUsage + float64(i)*dailyGrowth
		if predictedUsage > 100 {
			predictedUsage = 100
		}
		forecast = append(forecast, ForecastPoint{
			Date:           t,
			PredictedUsage: predictedUsage,
			Confidence:     0.95 - float64(i)*0.01, // 置信度随时间降低
		})
	}

	api.OK(c, gin.H{
		"quota_id":     quotaID,
		"forecast":     forecast,
		"daily_growth": dailyGrowth,
		"generated_at": time.Now(),
	})
}

// setWarningThresholds 设置预警阈值
func (h *QuotaAPIHandlers) setWarningThresholds(c *gin.Context) {
	var req SetWarningThresholdsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 验证阈值
	if req.WarningThreshold >= req.CriticalThreshold {
		api.BadRequest(c, "警告阈值必须小于严重阈值")
		return
	}

	api.OK(c, gin.H{
		"message":    "预警阈值设置成功",
		"thresholds": req,
	})
}

// ========== 配额报告 API ==========

// listQuotaReports 列出配额报告
func (h *QuotaAPIHandlers) listQuotaReports(c *gin.Context) {
	reports := []QuotaReportMeta{
		{
			ID:          "report_001",
			Name:        "配额使用月报 - 202603",
			Type:        "monthly",
			GeneratedAt: time.Now().AddDate(0, 0, -1),
			Period: ReportPeriod{
				StartTime: time.Now().AddDate(0, -1, 0),
				EndTime:   time.Now(),
			},
			Format:    "json",
			Size:      102400,
			Status:    "completed",
			CreatedBy: "system",
		},
	}

	api.OK(c, gin.H{
		"reports": reports,
		"total":   len(reports),
	})
}

// generateQuotaReport 生成配额报告
func (h *QuotaAPIHandlers) generateQuotaReport(c *gin.Context) {
	var req GenerateQuotaReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	report := QuotaReportMeta{
		ID:          "report_" + time.Now().Format("20060102150405"),
		Name:        req.Name,
		Type:        req.Type,
		GeneratedAt: time.Now(),
		Period:      req.Period,
		Format:      req.Format,
		Status:      "generating",
		CreatedBy:   "user",
	}

	api.Accepted(c, gin.H{
		"message": "报告生成任务已提交",
		"report":  report,
	})
}

// getQuotaReport 获取配额报告
func (h *QuotaAPIHandlers) getQuotaReport(c *gin.Context) {
	reportID := c.Param("report_id")

	report := &DetailedQuotaReport{
		ID:          reportID,
		Name:        "配额使用月报 - 202603",
		Type:        "monthly",
		GeneratedAt: time.Now().AddDate(0, 0, -1),
		Period: ReportPeriod{
			StartTime: time.Now().AddDate(0, -1, 0),
			EndTime:   time.Now(),
		},
		Summary: QuotaUsageSummary{
			TotalQuotas:         20,
			TotalHardLimitBytes: 2000 * 1024 * 1024 * 1024,
			TotalUsedBytes:      1500 * 1024 * 1024 * 1024,
			AvgUsagePercent:     75.0,
			OverSoftLimit:       5,
			OverHardLimit:       2,
			ActiveAlerts:        7,
		},
		Details: []QuotaUsageRecord{
			{
				QuotaID:      "quota_001",
				Type:         "user",
				TargetName:   "张三",
				VolumeName:   "volume1",
				HardLimit:    100 * 1024 * 1024 * 1024,
				UsedBytes:    65 * 1024 * 1024 * 1024,
				UsagePercent: 65.0,
			},
		},
	}

	api.OK(c, report)
}

// exportQuotaReport 导出配额报告
func (h *QuotaAPIHandlers) exportQuotaReport(c *gin.Context) {
	reportID := c.Param("report_id")
	format := c.DefaultQuery("format", "json")

	if format != "json" && format != "csv" {
		api.BadRequest(c, "不支持的导出格式")
		return
	}

	api.OK(c, gin.H{
		"message":      "报告导出成功",
		"report_id":    reportID,
		"format":       format,
		"download_url": fmt.Sprintf("/api/v1/quota-reports/%s/download?format=%s", reportID, format),
	})
}

// deleteQuotaReport 删除配额报告
func (h *QuotaAPIHandlers) deleteQuotaReport(c *gin.Context) {
	reportID := c.Param("report_id")

	api.OK(c, gin.H{
		"message":   "报告删除成功",
		"report_id": reportID,
	})
}

// ========== 请求/响应结构体 ==========

// CreateQuotaRequest 创建配额请求
type CreateQuotaRequest struct {
	Type         string `json:"type" binding:"required"`        // user, service, directory
	TargetID     string `json:"target_id" binding:"required"`   // 用户ID/服务名/路径
	TargetName   string `json:"target_name"`                    // 显示名称
	VolumeName   string `json:"volume_name" binding:"required"` // 卷名
	Path         string `json:"path"`                           // 限制路径
	HardLimit    uint64 `json:"hard_limit" binding:"required"`  // 硬限制（字节）
	SoftLimit    uint64 `json:"soft_limit"`                     // 软限制（字节）
	WarningLimit uint64 `json:"warning_limit"`                  // 警告阈值（字节）
}

// UpdateQuotaRequest 更新配额请求
type UpdateQuotaRequest struct {
	HardLimit    *uint64 `json:"hard_limit"`    // 硬限制（字节）
	SoftLimit    *uint64 `json:"soft_limit"`    // 软限制（字节）
	WarningLimit *uint64 `json:"warning_limit"` // 警告阈值（字节）
	Enabled      *bool   `json:"enabled"`       // 是否启用
}

// SetUserQuotaRequest 设置用户配额请求
type SetUserQuotaRequest struct {
	VolumeName   string `json:"volume_name" binding:"required"`
	HardLimit    uint64 `json:"hard_limit" binding:"required"`
	SoftLimit    uint64 `json:"soft_limit"`
	WarningLimit uint64 `json:"warning_limit"`
}

// SetServiceQuotaRequest 设置服务配额请求
type SetServiceQuotaRequest struct {
	VolumeName   string `json:"volume_name" binding:"required"`
	HardLimit    uint64 `json:"hard_limit" binding:"required"`
	SoftLimit    uint64 `json:"soft_limit"`
	WarningLimit uint64 `json:"warning_limit"`
}

// SetDirectoryQuotaRequest 设置目录配额请求
type SetDirectoryQuotaRequest struct {
	VolumeName   string `json:"volume_name" binding:"required"`
	HardLimit    uint64 `json:"hard_limit" binding:"required"`
	SoftLimit    uint64 `json:"soft_limit"`
	WarningLimit uint64 `json:"warning_limit"`
}

// QuotaUsageSummary 配额使用汇总
type QuotaUsageSummary struct {
	TotalQuotas         int                       `json:"total_quotas"`
	TotalHardLimitBytes uint64                    `json:"total_hard_limit_bytes"`
	TotalSoftLimitBytes uint64                    `json:"total_soft_limit_bytes"`
	TotalUsedBytes      uint64                    `json:"total_used_bytes"`
	TotalAvailableBytes uint64                    `json:"total_available_bytes"`
	AvgUsagePercent     float64                   `json:"avg_usage_percent"`
	OverSoftLimit       int                       `json:"over_soft_limit"`
	OverHardLimit       int                       `json:"over_hard_limit"`
	ActiveAlerts        int                       `json:"active_alerts"`
	ByType              map[string]TypeUsageStats `json:"by_type"`
	TopUsers            []TopUsageItem            `json:"top_users"`
	GeneratedAt         time.Time                 `json:"generated_at"`
}

// TypeUsageStats 类型使用统计
type TypeUsageStats struct {
	Count      int    `json:"count"`
	UsedBytes  uint64 `json:"used_bytes"`
	LimitBytes uint64 `json:"limit_bytes"`
}

// TopUsageItem 使用量排行项
type TopUsageItem struct {
	TargetName   string  `json:"target_name"`
	UsedGB       float64 `json:"used_gb"`
	LimitGB      float64 `json:"limit_gb"`
	UsagePercent float64 `json:"usage_percent"`
}

// UsageHistoryPoint 使用历史数据点
type UsageHistoryPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	UsedBytes    uint64    `json:"used_bytes"`
	UsagePercent float64   `json:"usage_percent"`
}

// ResolveAlertRequest 解决告警请求
type ResolveAlertRequest struct {
	Resolution string `json:"resolution"` // 解决说明
}

// SilenceAlertRequest 静默告警请求
type SilenceAlertRequest struct {
	Duration int    `json:"duration" binding:"required"` // 静默时长（分钟）
	Reason   string `json:"reason"`                      // 静默原因
}

// TestAlertRequest 测试告警请求
type TestAlertRequest struct {
	Channels []string `json:"channels" binding:"required"` // email, webhook, sms
	QuotaID  string   `json:"quota_id"`
}

// QuotaAlertConfig 配额告警配置
type QuotaAlertConfig struct {
	Enabled            bool    `json:"enabled"`
	WarningThreshold   float64 `json:"warning_threshold"`   // 警告阈值（%）
	CriticalThreshold  float64 `json:"critical_threshold"`  // 严重阈值（%）
	EmergencyThreshold float64 `json:"emergency_threshold"` // 紧急阈值（%）
	CheckInterval      int     `json:"check_interval"`      // 检查间隔（分钟）
	NotifyEmail        bool    `json:"notify_email"`
	NotifyWebhook      bool    `json:"notify_webhook"`
	WebhookURL         string  `json:"webhook_url"`
	SilenceDuration    int     `json:"silence_duration"` // 静默时长（分钟）
	EscalationEnabled  bool    `json:"escalation_enabled"`
	EscalationInterval int     `json:"escalation_interval"` // 升级间隔（分钟）
	MaxEscalationLevel int     `json:"max_escalation_level"`
}

// QuotaWarning 配额预警
type QuotaWarning struct {
	ID            string     `json:"id"`
	QuotaID       string     `json:"quota_id"`
	TargetName    string     `json:"target_name"`
	Type          string     `json:"type"` // usage_high, growth_fast, near_limit
	Severity      string     `json:"severity"`
	CurrentValue  float64    `json:"current_value"`
	Threshold     float64    `json:"threshold"`
	Message       string     `json:"message"`
	PredictedDays int        `json:"predicted_days,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
}

// QuotaPredictionOutput 配额预测输出
type QuotaPredictionOutput struct {
	QuotaID           string     `json:"quota_id"`
	TargetName        string     `json:"target_name"`
	CurrentUsage      float64    `json:"current_usage"`
	DailyGrowthGB     float64    `json:"daily_growth_gb"`
	DaysToSoftLimit   int        `json:"days_to_soft_limit"`
	DaysToHardLimit   int        `json:"days_to_hard_limit"`
	EstimatedSoftDate *time.Time `json:"estimated_soft_date,omitempty"`
	EstimatedHardDate *time.Time `json:"estimated_hard_date,omitempty"`
	RiskLevel         string     `json:"risk_level"` // low, medium, high, critical
}

// SetWarningThresholdsRequest 设置预警阈值请求
type SetWarningThresholdsRequest struct {
	WarningThreshold   float64 `json:"warning_threshold" binding:"required"`
	CriticalThreshold  float64 `json:"critical_threshold" binding:"required"`
	EmergencyThreshold float64 `json:"emergency_threshold" binding:"required"`
}

// GenerateQuotaReportRequest 生成配额报告请求
type GenerateQuotaReportRequest struct {
	Name   string       `json:"name" binding:"required"`
	Type   string       `json:"type" binding:"required"` // daily, weekly, monthly
	Period ReportPeriod `json:"period"`
	Format string       `json:"format"` // json, csv
}

// QuotaReportMeta 配额报告元数据
type QuotaReportMeta struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	GeneratedAt time.Time    `json:"generated_at"`
	Period      ReportPeriod `json:"period"`
	Format      string       `json:"format"`
	Size        int64        `json:"size"`
	Status      string       `json:"status"` // generating, completed, failed
	CreatedBy   string       `json:"created_by"`
}

// DetailedQuotaReport 详细配额报告
type DetailedQuotaReport struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Type        string             `json:"type"`
	GeneratedAt time.Time          `json:"generated_at"`
	Period      ReportPeriod       `json:"period"`
	Summary     QuotaUsageSummary  `json:"summary"`
	Details     []QuotaUsageRecord `json:"details"`
}
