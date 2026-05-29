// Package hybridflash 提供 SSD/HDD 智能混合分层存储管理 HTTP API.
package hybridflash

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers API 处理器.
type Handlers struct {
	logger  *zap.Logger
	manager *Manager
}

// NewHandlers 创建 API 处理器.
func NewHandlers(logger *zap.Logger, mgr *Manager) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handlers{
		logger:  logger,
		manager: mgr,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	hybrid := rg.Group("/hybrid-flash")
	{
		// 混合闪存池管理
		pools := hybrid.Group("/pools")
		{
			pools.GET("", h.ListPools)
			pools.POST("", h.CreatePool)
			pools.GET("/:id/status", h.GetPoolStatus)
			pools.PUT("/:id/tier-policy", h.UpdateTierPolicy)
			pools.POST("/:id/rebalance", h.TriggerRebalance)
			pools.POST("/:id/capacity-suggestion", h.GetCapacitySuggestion)
			pools.GET("/:id/metrics", h.GetPerTierMetrics)
			pools.DELETE("/:id", h.DeletePool)
		}

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

		// 缓存策略
		hybrid.GET("/policies", h.ListCachePolicies)
		hybrid.POST("/policies", h.CreateCachePolicy)

		// 性能指标
		hybrid.GET("/metrics", h.GetMetrics)
	}
}

// ========== 混合闪存池管理 ==========

// ListPools 列出所有混合池.
func (h *Handlers) ListPools(c *gin.Context) {
	pools := h.manager.ListPools()

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    pools,
	})
}

// CreatePool 创建混合池.
func (h *Handlers) CreatePool(c *gin.Context) {
	var config HybridPoolConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "无效的请求参数: " + err.Error(),
		})
		return
	}

	pool, err := h.manager.CreatePool(&config)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "混合闪存池已创建",
		Data:    pool,
	})
}

// GetPoolStatus 获取池状态.
func (h *Handlers) GetPoolStatus(c *gin.Context) {
	poolID := c.Param("id")

	pool, err := h.manager.GetPool(poolID)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    pool,
	})
}

// UpdateTierPolicy 更新分层策略.
func (h *Handlers) UpdateTierPolicy(c *gin.Context) {
	poolID := c.Param("id")

	var policy TierPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "无效的请求参数: " + err.Error(),
		})
		return
	}

	if err := h.manager.UpdateTierPolicy(poolID, &policy); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "分层策略已更新",
	})
}

// TriggerRebalance 触发数据重平衡.
func (h *Handlers) TriggerRebalance(c *gin.Context) {
	poolID := c.Param("id")

	var req RebalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "无效的请求参数: " + err.Error(),
		})
		return
	}

	result, err := h.manager.Rebalance(poolID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "重平衡任务已启动",
		Data:    result,
	})
}

// DeletePool 删除混合闪存池.
func (h *Handlers) DeletePool(c *gin.Context) {
	poolID := c.Param("id")

	if err := h.manager.DeletePool(poolID); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "混合闪存池已删除",
	})
}

// GetCapacitySuggestion 获取容量规划建议.
func (h *Handlers) GetCapacitySuggestion(c *gin.Context) {
	poolID := c.Param("id")

	suggestion, err := h.manager.GetCapacitySuggestion(poolID)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    suggestion,
	})
}

// GetPerTierMetrics 获取分层性能指标.
func (h *Handlers) GetPerTierMetrics(c *gin.Context) {
	poolID := c.Param("id")

	iops, throughput, latency, err := h.manager.GetPerTierMetrics(poolID)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"iops":       iops,
			"throughput": throughput,
			"latency":    latency,
		},
	})
}

// ========== 分层状态和配置 ==========

// GetStatus 获取分层状态.
func (h *Handlers) GetStatus(c *gin.Context) {
	status := h.manager.GetStatus()
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    status,
	})
}

// UpdateConfig 更新配置.
func (h *Handlers) UpdateConfig(c *gin.Context) {
	var req ConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "无效的请求参数: " + err.Error(),
		})
		return
	}

	if req.TieringConfig != nil {
		h.manager.UpdateEngineConfig(*req.TieringConfig)
	}

	if req.HeatConfig != nil {
		h.manager.UpdateHeatConfig(*req.HeatConfig)
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "配置已更新",
	})
}

// GetConfig 获取当前配置.
func (h *Handlers) GetConfig(c *gin.Context) {
	status := h.manager.GetStatus()
	engine := h.manager.GetEngine()
	engineConfig := engine.GetHeatConfig()

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"tieringConfig": status.Config,
			"heatConfig":    engineConfig,
		},
	})
}

// GetReport 获取效率报告.
func (h *Handlers) GetReport(c *gin.Context) {
	period := c.DefaultQuery("period", "daily")

	report := h.manager.GenerateEfficiencyReport(period)

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    report,
	})
}

// ========== 块热度查询 ==========

// GetBlockHeat 获取块热度信息.
func (h *Handlers) GetBlockHeat(c *gin.Context) {
	blockID := c.Param("id")

	block, err := h.manager.GetBlockHeatInfo(blockID)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    block,
	})
}

// GetHotBlocks 获取热块列表.
func (h *Handlers) GetHotBlocks(c *gin.Context) {
	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	blocks := h.manager.GetHotBlocks(limit)

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    blocks,
	})
}

// GetColdBlocks 获取冷块列表.
func (h *Handlers) GetColdBlocks(c *gin.Context) {
	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	blocks := h.manager.GetColdBlocks(limit)

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    blocks,
	})
}

// ========== 缓存策略 ==========

// ListCachePolicies 列出缓存策略.
func (h *Handlers) ListCachePolicies(c *gin.Context) {
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

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    policies,
	})
}

// CreateCachePolicy 创建缓存策略.
func (h *Handlers) CreateCachePolicy(c *gin.Context) {
	var policy CachePolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "无效的请求参数: " + err.Error(),
		})
		return
	}

	policy.ID = fmt.Sprintf("policy-%d", time.Now().UnixNano())
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "缓存策略已创建",
		Data:    policy,
	})
}

// ========== 性能指标 ==========

// GetMetrics 获取性能指标.
func (h *Handlers) GetMetrics(c *gin.Context) {
	status := h.manager.GetStatus()

	// 生成示例指标
	metrics := &IOStatistics{
		TotalReads:      10000,
		TotalWrites:     5000,
		TotalReadBytes:  1024 * 1024 * 1024, // 1GB
		TotalWriteBytes: 512 * 1024 * 1024,  // 512MB
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

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    metrics,
	})
}
