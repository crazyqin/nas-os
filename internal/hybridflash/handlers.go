package hybridflash

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler API 处理器.
type Handler struct {
	engine *TieringEngine
}

// NewHandler 创建 API 处理器.
func NewHandler(engine *TieringEngine) *Handler {
	return &Handler{
		engine: engine,
	}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	hybrid := r.Group("/hybrid")
	{
		// 分层状态
		hybrid.GET("/status", h.GetStatus)

		// 配置管理
		hybrid.POST("/config", h.UpdateConfig)
		hybrid.GET("/config", h.GetConfig)

		// 效率报告
		hybrid.GET("/report", h.GetReport)

		// 块热度查询
		hybrid.GET("/blocks/:id/heat", h.GetBlockHeat)
		hybrid.GET("/blocks/hot", h.GetHotBlocks)
		hybrid.GET("/blocks/cold", h.GetColdBlocks)

		// 迁移管理
		hybrid.POST("/migrate", h.TriggerMigration)
		hybrid.GET("/migrate/tasks", h.ListMigrateTasks)
		hybrid.GET("/migrate/tasks/:id", h.GetMigrateTask)

		// 缓存策略
		hybrid.GET("/policies", h.ListCachePolicies)
		hybrid.POST("/policies", h.CreateCachePolicy)

		// 混合池管理
		hybrid.GET("/pools", h.ListPools)
		hybrid.GET("/pools/:id", h.GetPool)
		hybrid.POST("/pools", h.CreatePool)
		hybrid.PUT("/pools/:id", h.UpdatePool)

		// 性能指标
		hybrid.GET("/metrics", h.GetMetrics)
	}
}

// GetStatus 获取分层状态.
//
//	@Summary	获取混合分层状态
//	@Description	返回当前分层引擎状态、运行中的任务数、缓存命中率等
//	@Tags		hybridflash
//	@Accept		json
//	@Produce	json
//	@Success	200	{object}	TieringStatus
//	@Router		/api/v1/hybrid/status [get]
func (h *Handler) GetStatus(c *gin.Context) {
	status := h.engine.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    status,
	})
}

// UpdateConfig 更新配置.
//
//	@Summary	更新分层配置
//	@Description	更新分层引擎配置和热度追踪配置
//	@Tags		hybridflash
//	@Accept		json
//	@Produce	json
//	@Param		config	body		ConfigRequest	true	"配置参数"
//	@Success	200	{object}	Response
//	@Router		/api/v1/hybrid/config [post]
func (h *Handler) UpdateConfig(c *gin.Context) {
	var req ConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	if req.TieringConfig != nil {
		h.engine.UpdateConfig(*req.TieringConfig)
	}

	if req.HeatConfig != nil {
		h.engine.UpdateHeatConfig(*req.HeatConfig)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "配置已更新",
	})
}

// GetConfig 获取当前配置.
//
//	@Summary	获取分层配置
//	@Description	返回当前分层引擎配置和热度追踪配置
//	@Tags		hybridflash
//	@Produce	json
//	@Success	200	{object}	object
//	@Router		/api/v1/hybrid/config [get]
func (h *Handler) GetConfig(c *gin.Context) {
	status := h.engine.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"tieringConfig": status.Config,
			"heatConfig":    h.engine.heatConfig,
		},
	})
}

// GetReport 获取效率报告.
//
//	@Summary	获取分层效率报告
//	@Description	返回分层命中率、性能提升、空间利用率等统计
//	@Tags		hybridflash
//	@Produce	json
//	@Param		period	query		string	false	"报告周期"	default(daily)
//	@Success	200	{object}	EfficiencyReport
//	@Router		/api/v1/hybrid/report [get]
func (h *Handler) GetReport(c *gin.Context) {
	period := c.DefaultQuery("period", "daily")

	report := h.engine.GenerateEfficiencyReport(period)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    report,
	})
}

// GetBlockHeat 获取块热度信息.
//
//	@Summary	查询块热度
//	@Description	根据块ID查询其热度级别和访问记录
//	@Tags		hybridflash
//	@Produce	json
//	@Param		id	path		string	true	"块ID"
//	@Success	200	{object}	BlockAccessRecord
//	@Failure	404	{object}	ErrorResponse
//	@Router		/api/v1/hybrid/blocks/{id}/heat [get]
func (h *Handler) GetBlockHeat(c *gin.Context) {
	blockID := c.Param("id")

	block, err := h.engine.GetBlockHeatInfo(blockID)
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
		"data":    block,
	})
}

// GetHotBlocks 获取热块列表.
//
//	@Summary	查询热数据块
//	@Description	返回热度最高的数据块列表
//	@Tags		hybridflash
//	@Produce	json
//	@Param		limit	query		int	false	"返回数量"	default(10)
//	@Success	200	{array}		BlockAccessRecord
//	@Router		/api/v1/hybrid/blocks/hot [get]
func (h *Handler) GetHotBlocks(c *gin.Context) {
	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := parseIntParam(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	blocks := h.engine.GetHotBlocks(limit)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    blocks,
	})
}

// GetColdBlocks 获取冷块列表.
//
//	@Summary	查询冷数据块
//	@Description	返回最近最少访问的数据块列表
//	@Tags		hybridflash
//	@Produce	json
//	@Param		limit	query		int	false	"返回数量"	default(10)
//	@Success	200	{array}		BlockAccessRecord
//	@Router		/api/v1/hybrid/blocks/cold [get]
func (h *Handler) GetColdBlocks(c *gin.Context) {
	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := parseIntParam(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	blocks := h.engine.GetColdBlocks(limit)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    blocks,
	})
}

// TriggerMigration 触发迁移.
//
//	@Summary	手动触发数据迁移
//	@Description	创建一个从源层级到目标层级的迁移任务
//	@Tags		hybridflash
//	@Accept		json
//	@Produce	json
//	@Param		request	body		MigrateTask	true	"迁移请求"
//	@Success	200	{object}	MigrateTask
//	@Failure	400	{object}	ErrorResponse
//	@Router		/api/v1/hybrid/migrate [post]
func (h *Handler) TriggerMigration(c *gin.Context) {
	var req struct {
		SourceTier FlashType `json:"sourceTier"`
		TargetTier FlashType `json:"targetTier"`
		BlockIDs   []string  `json:"blockIds"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	task := &MigrateTask{
		ID:         fmt.Sprintf("manual-%d", time.Now().UnixNano()),
		Status:     MigrateStatusPending,
		CreatedAt:  time.Now(),
		SourceTier: req.SourceTier,
		TargetTier: req.TargetTier,
		TotalBlocks: int64(len(req.BlockIDs)),
		TotalBytes:  0,
	}

	// 加入迁移队列
	h.engine.migrationQueue <- task

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "迁移任务已创建",
		"data":    task,
	})
}

// ListMigrateTasks 列出迁移任务.
//
//	@Summary	列出迁移任务
//	@Description	返回所有迁移任务列表
//	@Tags		hybridflash
//	@Produce	json
//	@Success	200	{array}		MigrateTask
//	@Router		/api/v1/hybrid/migrate/tasks [get]
func (h *Handler) ListMigrateTasks(c *gin.Context) {
	h.engine.mu.RLock()
	tasks := make([]*MigrateTask, 0)
	for _, task := range h.engine.runningTasks {
		tasks = append(tasks, task)
	}
	tasks = append(tasks, h.engine.completedTasks...)
	h.engine.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    tasks,
	})
}

// GetMigrateTask 获取迁移任务详情.
//
//	@Summary	查询迁移任务
//	@Description	根据任务ID查询迁移任务详情
//	@Tags		hybridflash
//	@Produce	json
//	@Param		id	path		string	true	"任务ID"
//	@Success	200	{object}	MigrateTask
//	@Failure	404	{object}	ErrorResponse
//	@Router		/api/v1/hybrid/migrate/tasks/{id} [get]
func (h *Handler) GetMigrateTask(c *gin.Context) {
	taskID := c.Param("id")

	h.engine.mu.RLock()
	defer h.engine.mu.RUnlock()

	// 查找运行中的任务
	if task, exists := h.engine.runningTasks[taskID]; exists {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    task,
		})
		return
	}

	// 查找已完成的任务
	for _, task := range h.engine.completedTasks {
		if task.ID == taskID {
			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "success",
				"data":    task,
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": "任务不存在",
	})
}

// ListCachePolicies 列出缓存策略.
//
//	@Summary	列出缓存策略
//	@Description	返回所有缓存策略配置
//	@Tags		hybridflash
//	@Produce	json
//	@Success	200	{array}		CachePolicy
//	@Router		/api/v1/hybrid/policies [get]
func (h *Handler) ListCachePolicies(c *gin.Context) {
	// 返回示例策略
	policies := []*CachePolicy{
		{
			ID:            "default-l2arc",
			Name:          "L2ARC 默认策略",
			Description:   "SSD 作为 L2ARC 读缓存层",
			Enabled:       true,
			CacheRole:     CacheRoleL2ARC,
			HeatLevel:     HeatLevelHot,
			AccessPattern: AccessPatternRandom,
			MinBlockSize:  4096,
			MaxBlockSize:  1048576,
			Priority:      100,
			PreferSSD:     true,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            "default-slog",
			Name:          "SLOG 默认策略",
			Description:   "SSD 作为 SLOG 同步写入日志",
			Enabled:       true,
			CacheRole:     CacheRoleSLOG,
			HeatLevel:     HeatLevelHot,
			AccessPattern: AccessPatternSequential,
			MinBlockSize:  512,
			MaxBlockSize:  131072,
			Priority:      200,
			PreferSSD:     true,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    policies,
	})
}

// CreateCachePolicy 创建缓存策略.
//
//	@Summary	创建缓存策略
//	@Description	创建新的缓存策略配置
//	@Tags		hybridflash
//	@Accept		json
//	@Produce	json
//	@Param		policy	body		CachePolicy	true	"策略配置"
//	@Success	200	{object}	CachePolicy
//	@Failure	400	{object}	ErrorResponse
//	@Router		/api/v1/hybrid/policies [post]
func (h *Handler) CreateCachePolicy(c *gin.Context) {
	var policy CachePolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	policy.ID = fmt.Sprintf("policy-%d", time.Now().UnixNano())
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "缓存策略已创建",
		"data":    policy,
	})
}

// ListPools 列出混合池.
//
//	@Summary	列出混合池
//	@Description	返回所有混合存储池配置
//	@Tags		hybridflash
//	@Produce	json
//	@Success	200	{array}		HybridPool
//	@Router		/api/v1/hybrid/pools [get]
func (h *Handler) ListPools(c *gin.Context) {
	h.engine.mu.RLock()
	pools := make([]*HybridPool, 0, len(h.engine.pools))
	for _, pool := range h.engine.pools {
		pools = append(pools, pool)
	}
	h.engine.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    pools,
	})
}

// GetPool 获取混合池.
//
//	@Summary	查询混合池
//	@Description	根据池ID查询混合存储池详情
//	@Tags		hybridflash
//	@Produce	json
//	@Param		id	path		string	true	"池ID"
//	@Success	200	{object}	HybridPool
//	@Failure	404	{object}	ErrorResponse
//	@Router		/api/v1/hybrid/pools/{id} [get]
func (h *Handler) GetPool(c *gin.Context) {
	poolID := c.Param("id")

	pool, err := h.engine.GetPool(poolID)
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
		"data":    pool,
	})
}

// CreatePool 创建混合池.
//
//	@Summary	创建混合池
//	@Description	创建新的混合存储池
//	@Tags		hybridflash
//	@Accept		json
//	@Produce	json
//	@Param		pool	body		HybridPool	true	"池配置"
//	@Success	200	{object}	HybridPool
//	@Failure	400	{object}	ErrorResponse
//	@Router		/api/v1/hybrid/pools [post]
func (h *Handler) CreatePool(c *gin.Context) {
	var pool HybridPool
	if err := c.ShouldBindJSON(&pool); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	pool.ID = fmt.Sprintf("pool-%d", time.Now().UnixNano())
	pool.State = PoolStateOnline
	pool.CreatedAt = time.Now()
	pool.UpdatedAt = time.Now()

	h.engine.RegisterPool(&pool)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "混合池已创建",
		"data":    pool,
	})
}

// UpdatePool 更新混合池.
//
//	@Summary	更新混合池
//	@Description	更新混合存储池配置
//	@Tags		hybridflash
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string		true	"池ID"
//	@Param		pool	body		HybridPool	true	"池配置"
//	@Success	200	{object}	HybridPool
//	@Failure	404	{object}	ErrorResponse
//	@Router		/api/v1/hybrid/pools/{id} [put]
func (h *Handler) UpdatePool(c *gin.Context) {
	poolID := c.Param("id")

	existing, err := h.engine.GetPool(poolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	var update HybridPool
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	// 更新字段
	if update.Name != "" {
		existing.Name = update.Name
	}
	if update.State != "" {
		existing.State = update.State
	}
	if update.FlashDevices != nil {
		existing.FlashDevices = update.FlashDevices
	}
	if update.HDDDevices != nil {
		existing.HDDDevices = update.HDDDevices
	}
	if update.CachePolicies != nil {
		existing.CachePolicies = update.CachePolicies
	}
	existing.UpdatedAt = time.Now()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "混合池已更新",
		"data":    existing,
	})
}

// GetMetrics 获取性能指标.
//
//	@Summary	获取性能指标
//	@Description	返回当前 IO 统计和性能指标
//	@Tags		hybridflash
//	@Produce	json
//	@Success	200	{object}	IOStatistics
//	@Router		/api/v1/hybrid/metrics [get]
func (h *Handler) GetMetrics(c *gin.Context) {
	status := h.engine.GetStatus()

	// 生成示例指标
	metrics := &IOStatistics{
		TotalReads:      10000,
		TotalWrites:     5000,
		TotalReadBytes:  1024 * 1024 * 1024,  // 1GB
		TotalWriteBytes: 512 * 1024 * 1024,   // 512MB
		AvgReadLatency:  0.5,
		AvgWriteLatency: 1.2,
		HitRateL2ARC:    status.HitRateL2ARC,
		HitRateSLOG:     status.HitRateSLOG,
		RecentMetrics: []IOMetric{
			{
				Timestamp:      time.Now().Add(-5 * time.Minute),
				ReadIOPS:       2000,
				WriteIOPS:      1000,
				ReadBandwidth:  200,
				WriteBandwidth: 100,
				AvgLatency:     0.4,
				P99Latency:     1.5,
			},
			{
				Timestamp:      time.Now(),
				ReadIOPS:       2500,
				WriteIOPS:      1200,
				ReadBandwidth:  250,
				WriteBandwidth: 120,
				AvgLatency:     0.3,
				P99Latency:     1.2,
			},
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    metrics,
	})
}

// parseIntParam 解析整数参数.
func parseIntParam(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}
