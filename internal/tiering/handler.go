// Package tiering API 处理器
package tiering

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler API 处理器.
type Handler struct {
	manager   *Manager
	metrics   *Metrics
	generator *EfficiencyReportGenerator
}

// NewHandler 创建 API 处理器.
func NewHandler(manager *Manager) *Handler {
	metrics := NewMetrics()
	return &Handler{
		manager:   manager,
		metrics:   metrics,
		generator: NewEfficiencyReportGenerator(manager, metrics, nil),
	}
}

// NewHandlerWithGenerator 创建带自定义生成器的 API 处理器.
func NewHandlerWithGenerator(manager *Manager, metrics *Metrics, generator *EfficiencyReportGenerator) *Handler {
	return &Handler{
		manager:   manager,
		metrics:   metrics,
		generator: generator,
	}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	tiering := r.Group("/tiering")
	{
		// 存储层配置
		tiering.POST("/tiers", h.CreateTier)
		tiering.GET("/tiers", h.ListTiers)
		tiering.GET("/tiers/:type", h.GetTier)
		tiering.PUT("/tiers/:type", h.UpdateTier)
		tiering.DELETE("/tiers/:type", h.DeleteTier)

		// 分层策略
		tiering.GET("/policies", h.ListPolicies)
		tiering.POST("/policies", h.CreatePolicy)
		tiering.GET("/policies/:id", h.GetPolicy)
		tiering.PUT("/policies/:id", h.UpdatePolicy)
		tiering.DELETE("/policies/:id", h.DeletePolicy)
		tiering.POST("/policies/:id/execute", h.ExecutePolicy)

		// 迁移操作
		tiering.POST("/migrate", h.Migrate)

		// 任务管理
		tiering.GET("/tasks", h.ListTasks)
		tiering.GET("/tasks/:id", h.GetTask)
		tiering.DELETE("/tasks/:id", h.CancelTask)

		// 状态查询
		tiering.GET("/status", h.GetStatus)
		tiering.GET("/stats", h.GetStats)
		tiering.GET("/stats/:type", h.GetTierStats)

		// 效率报告 API (v2.4.0 新增)
		tiering.GET("/reports/efficiency", h.GetEfficiencyReport)
		tiering.GET("/reports/distribution", h.GetDataDistribution)
		tiering.GET("/reports/migration-efficiency", h.GetMigrationEfficiency)
		tiering.GET("/reports/cost-analysis", h.GetCostAnalysis)
		tiering.GET("/reports/capacity-forecast", h.GetCapacityForecast)
		tiering.GET("/reports/health-score", h.GetHealthScore)
		tiering.GET("/reports/recommendations", h.GetRecommendations)

		// 优化操作 API
		tiering.POST("/optimize/ssd-cache", h.OptimizeSSDCache)
		tiering.POST("/optimize/auto-migrate", h.AutoMigrate)

		// 指标导出
		tiering.GET("/metrics", h.GetMetrics)
		tiering.GET("/metrics/prometheus", h.GetPrometheusMetrics)
	}
}

// ListTiers 列出所有存储层.
func (h *Handler) ListTiers(c *gin.Context) {
	tiers := h.manager.ListTiers()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    tiers,
	})
}

// CreateTier 创建存储层.
func (h *Handler) CreateTier(c *gin.Context) {
	var config TierConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	if config.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "存储层类型不能为空",
		})
		return
	}

	if err := h.manager.CreateTier(config.Type, config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "存储层已创建",
		"data":    config,
	})
}

// GetTier 获取存储层配置.
func (h *Handler) GetTier(c *gin.Context) {
	tierType := TierType(c.Param("type"))
	tier, err := h.manager.GetTier(tierType)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    tier,
	})
}

// UpdateTier 更新存储层配置.
func (h *Handler) UpdateTier(c *gin.Context) {
	tierType := TierType(c.Param("type"))
	var config TierConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	config.Type = tierType
	if err := h.manager.UpdateTier(tierType, config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "存储层配置已更新",
	})
}

// DeleteTier 删除存储层.
func (h *Handler) DeleteTier(c *gin.Context) {
	tierType := TierType(c.Param("type"))
	if err := h.manager.DeleteTier(tierType); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "存储层已删除",
	})
}

// ListPolicies 列出所有策略.
func (h *Handler) ListPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    policies,
	})
}

// CreatePolicy 创建策略.
func (h *Handler) CreatePolicy(c *gin.Context) {
	var policy Policy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	created, err := h.manager.CreatePolicy(policy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "策略已创建",
		"data":    created,
	})
}

// GetPolicy 获取策略.
func (h *Handler) GetPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    policy,
	})
}

// UpdatePolicy 更新策略.
func (h *Handler) UpdatePolicy(c *gin.Context) {
	id := c.Param("id")
	var policy Policy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	if err := h.manager.UpdatePolicy(id, policy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "策略已更新",
	})
}

// DeletePolicy 删除策略.
func (h *Handler) DeletePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePolicy(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "策略已删除",
	})
}

// ExecutePolicy 执行策略.
func (h *Handler) ExecutePolicy(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.ExecutePolicy(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "策略执行中",
		"data":    task,
	})
}

// Migrate 手动迁移.
func (h *Handler) Migrate(c *gin.Context) {
	var req MigrateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	task, err := h.manager.Migrate(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "迁移任务已创建",
		"data":    task,
	})
}

// ListTasks 列出迁移任务.
func (h *Handler) ListTasks(c *gin.Context) {
	tasks := h.manager.ListTasks(100)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    tasks,
	})
}

// GetTask 获取任务详情.
func (h *Handler) GetTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    task,
	})
}

// CancelTask 取消任务.
func (h *Handler) CancelTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.CancelTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "任务已取消",
	})
}

// GetStatus 获取分层状态.
func (h *Handler) GetStatus(c *gin.Context) {
	status := h.manager.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    status,
	})
}

// GetStats 获取分层统计.
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetAccessStats()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// GetTierStats 获取存储层统计.
func (h *Handler) GetTierStats(c *gin.Context) {
	tierType := TierType(c.Param("type"))

	tier, err := h.manager.GetTier(tierType)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	stats, err := h.manager.GetTierStats(tierType)
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
		"data": gin.H{
			"tier":  tier,
			"stats": stats,
		},
	})
}

// ==================== 效率报告 API ====================

// GetEfficiencyReport 获取完整的分层效率报告.
func (h *Handler) GetEfficiencyReport(c *gin.Context) {
	period := c.DefaultQuery("period", "daily")

	report, err := h.generator.GenerateReport(period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成报告失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    report,
	})
}

// GetDataDistribution 获取冷热数据分布报告.
func (h *Handler) GetDataDistribution(c *gin.Context) {
	report := h.generator.generateDataDistribution()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    report,
	})
}

// GetMigrationEfficiency 获取迁移效率统计报告.
func (h *Handler) GetMigrationEfficiency(c *gin.Context) {
	period := c.DefaultQuery("period", "daily")
	report := h.generator.generateMigrationEfficiency(period)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    report,
	})
}

// GetCostAnalysis 获取成本分析报告.
func (h *Handler) GetCostAnalysis(c *gin.Context) {
	report := h.generator.generateCostAnalysis()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    report,
	})
}

// GetCapacityForecast 获取容量趋势预测报告.
func (h *Handler) GetCapacityForecast(c *gin.Context) {
	days := 90
	if d := c.Query("days"); d != "" {
		if parsed, err := parseIntParam(d); err == nil {
			days = parsed
		}
	}

	report := h.generator.generateCapacityForecast(days)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    report,
	})
}

// GetHealthScore 获取分层健康评分.
func (h *Handler) GetHealthScore(c *gin.Context) {
	score := h.generator.calculateHealthScore()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    score,
	})
}

// GetRecommendations 获取优化建议.
func (h *Handler) GetRecommendations(c *gin.Context) {
	recommendations := h.generator.generateRecommendations()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    recommendations,
	})
}

// OptimizeSSDCache 优化 SSD 缓存层.
func (h *Handler) OptimizeSSDCache(c *gin.Context) {
	result, err := h.manager.OptimizeSSDCache()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "优化失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "SSD缓存优化已完成",
		"data":    result,
	})
}

// AutoMigrate 执行自动数据迁移.
func (h *Handler) AutoMigrate(c *gin.Context) {
	result, err := h.manager.AutoMigrate()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "自动迁移失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "自动迁移已完成",
		"data":    result,
	})
}

// GetMetrics 获取监控指标.
func (h *Handler) GetMetrics(c *gin.Context) {
	summary := h.metrics.GetSummary()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    summary,
	})
}

// GetPrometheusMetrics 获取 Prometheus 格式指标.
func (h *Handler) GetPrometheusMetrics(c *gin.Context) {
	output := h.metrics.ExportPrometheus()

	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(output))
}

// parseIntParam 解析整数参数.
func parseIntParam(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}
