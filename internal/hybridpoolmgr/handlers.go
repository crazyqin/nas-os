// Package hybridpoolmgr 提供混合存储池 REST API 处理器
package hybridpoolmgr

import (
	"strconv"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 混合池 API 处理器.
type Handlers struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager, logger *zap.Logger) *Handlers {
	return &Handlers{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	pools := r.Group("/hybrid-pools")
	{
		// 池管理
		pools.GET("", h.listPools)
		pools.POST("", h.createPool)
		pools.GET("/:name", h.getPool)
		pools.DELETE("/:name", h.deletePool)

		// 设备管理
		pools.POST("/:name/devices", h.addDevice)
		pools.DELETE("/:name/devices", h.removeDevice)

		// IO 统计与热度分析
		pools.GET("/:name/io-stats", h.getIOStats)
		pools.GET("/:name/heat-analysis", h.analyzeHeat)
		pools.GET("/:name/blocks/:blockId/heat", h.getBlockHeat)
		pools.POST("/:name/io", h.recordIO)

		// 自动分层
		pools.POST("/:name/tiering/run", h.runTiering)
		pools.PUT("/:name/tiering", h.updateTieringConfig)

		// 重平衡
		pools.POST("/:name/rebalance/run", h.runRebalance)
		pools.PUT("/:name/rebalance", h.updateRebalancePolicy)

		// 健康监控
		pools.GET("/:name/health", h.checkHealth)
		pools.GET("/:name/alerts", h.getAlerts)
		pools.POST("/:name/alerts", h.addAlert)
		pools.POST("/:name/alerts/:alertId/resolve", h.resolveAlert)
	}
}

// ========== 池管理 ==========

// listPools 列出所有混合池.
func (h *Handlers) listPools(c *gin.Context) {
	pools := h.manager.ListPools()
	api.OK(c, pools)
}

// createPool 创建混合池.
func (h *Handlers) createPool(c *gin.Context) {
	var req CreatePoolRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	pool, err := h.manager.CreatePool(&req)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, pool)
}

// getPool 获取混合池.
func (h *Handlers) getPool(c *gin.Context) {
	name := c.Param("name")
	pool, err := h.manager.GetPool(name)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, pool)
}

// deletePool 删除混合池.
func (h *Handlers) deletePool(c *gin.Context) {
	name := c.Param("name")
	force := c.Query("force") == "true"

	if err := h.manager.DeletePool(name, force); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "混合池已删除", nil)
}

// ========== 设备管理 ==========

// addDevice 添加设备.
func (h *Handlers) addDevice(c *gin.Context) {
	poolName := c.Param("name")
	var req AddDeviceRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.AddDevice(poolName, &req); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "设备已添加", nil)
}

// removeDevice 移除设备.
func (h *Handlers) removeDevice(c *gin.Context) {
	poolName := c.Param("name")
	devicePath := c.Query("device")
	if devicePath == "" {
		api.BadRequest(c, "设备路径不能为空")
		return
	}

	if err := h.manager.RemoveDevice(poolName, devicePath); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "设备已移除", nil)
}

// ========== IO 统计与热度分析 ==========

// getIOStats 获取 IO 统计.
func (h *Handlers) getIOStats(c *gin.Context) {
	poolName := c.Param("name")
	stats, err := h.manager.GetPoolIOStats(poolName)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, stats)
}

// analyzeHeat 热度分析.
func (h *Handlers) analyzeHeat(c *gin.Context) {
	poolName := c.Param("name")
	topN := 10
	if n, err := strconv.Atoi(c.Query("top")); err == nil && n > 0 {
		topN = n
	}

	result, err := h.manager.AnalyzeHeat(poolName, topN)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, result)
}

// getBlockHeat 获取块热度.
func (h *Handlers) getBlockHeat(c *gin.Context) {
	poolName := c.Param("name")
	blockID := c.Param("blockId")

	heat, err := h.manager.GetBlockHeat(poolName, blockID)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, heat)
}

// recordIO 记录 IO.
func (h *Handlers) recordIO(c *gin.Context) {
	poolName := c.Param("name")

	var req struct {
		BlockID        string     `json:"blockId" binding:"required"`
		Path           string     `json:"path" binding:"required"`
		Tier           DeviceTier `json:"tier" binding:"required"`
		Size           uint64     `json:"size"`
		IsRead         bool       `json:"isRead"`
		LatencyMicros  float64    `json:"latencyMicros"`
	}
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	h.manager.RecordIO(poolName, req.BlockID, req.Path, req.Tier, req.Size, req.IsRead, req.LatencyMicros)
	api.OKWithMessage(c, "IO 已记录", nil)
}

// ========== 自动分层 ==========

// runTiering 执行自动分层.
func (h *Handlers) runTiering(c *gin.Context) {
	poolName := c.Param("name")

	result, err := h.manager.RunTiering(poolName)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, result)
}

// updateTieringConfig 更新分层配置.
func (h *Handlers) updateTieringConfig(c *gin.Context) {
	poolName := c.Param("name")

	var config TieringConfig
	if err := api.BindAndValidate(c, &config); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.UpdateTieringConfig(poolName, config); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "分层配置已更新", nil)
}

// ========== 重平衡 ==========

// runRebalance 执行重平衡.
func (h *Handlers) runRebalance(c *gin.Context) {
	poolName := c.Param("name")

	result, err := h.manager.RunRebalance(poolName)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, result)
}

// updateRebalancePolicy 更新重平衡策略.
func (h *Handlers) updateRebalancePolicy(c *gin.Context) {
	poolName := c.Param("name")

	var policy RebalancePolicy
	if err := api.BindAndValidate(c, &policy); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.UpdateRebalancePolicy(poolName, policy); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "重平衡策略已更新", nil)
}

// ========== 健康监控 ==========

// checkHealth 检查池健康.
func (h *Handlers) checkHealth(c *gin.Context) {
	poolName := c.Param("name")

	health, err := h.manager.CheckHealth(poolName)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, health)
}

// getAlerts 获取告警.
func (h *Handlers) getAlerts(c *gin.Context) {
	poolName := c.Param("name")
	resolved := c.Query("resolved") == "true"

	alerts := h.manager.GetAlerts(poolName, resolved)
	api.OK(c, alerts)
}

// addAlert 添加告警.
func (h *Handlers) addAlert(c *gin.Context) {
	poolName := c.Param("name")

	var req struct {
		Device  string     `json:"device" binding:"required"`
		Message string     `json:"message" binding:"required"`
		Level   AlertLevel `json:"level" binding:"required"`
	}
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	h.manager.AddAlert(poolName, req.Device, req.Message, req.Level)
	api.OKWithMessage(c, "告警已添加", nil)
}

// resolveAlert 解决告警.
func (h *Handlers) resolveAlert(c *gin.Context) {
	poolName := c.Param("name")
	alertID := c.Param("alertId")

	if err := h.manager.ResolveAlert(poolName, alertID); err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OKWithMessage(c, "告警已解决", nil)
}
