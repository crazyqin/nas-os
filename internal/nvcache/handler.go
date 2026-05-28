// Package nvcache 提供 NVMe 缓存 REST API 处理器
package nvcache

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers NVMe 缓存 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	nvcache := r.Group("/nvcache")
	{
		// 设备管理
		nvcache.GET("/devices", h.listDevices)
		nvcache.POST("/devices", h.registerDevice)
		nvcache.GET("/devices/:id", h.getDevice)
		nvcache.DELETE("/devices/:id", h.unregisterDevice)
		nvcache.GET("/devices/search", h.searchDevices)

		// 缓存池管理
		nvcache.GET("/pools", h.listPools)
		nvcache.POST("/pools", h.createPool)
		nvcache.GET("/pools/:id", h.getPool)
		nvcache.DELETE("/pools/:id", h.deletePool)
		nvcache.PUT("/pools/:id/policy", h.updatePoolPolicy)

		// 缓存映射管理
		nvcache.GET("/mappings", h.listMappings)
		nvcache.POST("/mappings", h.createMapping)
		nvcache.GET("/mappings/:id", h.getMapping)
		nvcache.DELETE("/mappings/:id", h.deleteMapping)

		// 分层规则管理
		nvcache.GET("/tier-rules", h.listTierRules)
		nvcache.POST("/tier-rules", h.createTierRule)
		nvcache.GET("/tier-rules/:id", h.getTierRule)
		nvcache.PUT("/tier-rules/:id", h.updateTierRule)
		nvcache.DELETE("/tier-rules/:id", h.deleteTierRule)

		// 预热任务管理
		nvcache.GET("/warmup", h.listWarmupTasks)
		nvcache.POST("/warmup", h.createWarmupTask)
		nvcache.GET("/warmup/:id", h.getWarmupTask)
		nvcache.POST("/warmup/:id/cancel", h.cancelWarmupTask)

		// 一致性检查
		nvcache.GET("/consistency", h.listConsistencyChecks)
		nvcache.POST("/consistency", h.startConsistencyCheck)
		nvcache.GET("/consistency/:id", h.getConsistencyCheck)

		// 统计信息
		nvcache.GET("/stats/:pool_id", h.getStats)
		nvcache.GET("/stats/:pool_id/history", h.getStatsHistory)

		// 缓存操作
		nvcache.POST("/flush", h.flushCache)
		nvcache.POST("/invalidate", h.invalidateCache)

		// 配置
		nvcache.GET("/config", h.getConfig)
		nvcache.PUT("/config", h.updateConfig)

		// 系统概览
		nvcache.GET("/overview", h.getOverview)

		// 支持列表
		nvcache.GET("/supported/policies", h.getSupportedPolicies)
		nvcache.GET("/supported/evictions", h.getSupportedEvictions)
		nvcache.GET("/supported/raid-levels", h.getSupportedRAIDLevels)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// listDevices 列出设备
func (h *Handlers) listDevices(c *gin.Context) {
	role := DeviceRole(c.Query("role"))
	devices := h.manager.ListDevices(role)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    devices,
	})
}

// registerDevice 注册设备
func (h *Handlers) registerDevice(c *gin.Context) {
	var req RegisterDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	device, err := h.manager.RegisterDevice(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "device registered",
		Data:    device,
	})
}

// getDevice 获取设备
func (h *Handlers) getDevice(c *gin.Context) {
	id := c.Param("id")
	device, err := h.manager.GetDevice(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    device,
	})
}

// unregisterDevice 注销设备
func (h *Handlers) unregisterDevice(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.UnregisterDevice(id); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "device unregistered",
	})
}

// searchDevices 搜索设备
func (h *Handlers) searchDevices(c *gin.Context) {
	keyword := c.Query("q")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "search keyword is required",
		})
		return
	}

	devices := h.manager.SearchDevices(keyword)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    devices,
	})
}

// listPools 列出缓存池
func (h *Handlers) listPools(c *gin.Context) {
	pools := h.manager.ListPools()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    pools,
	})
}

// createPool 创建缓存池
func (h *Handlers) createPool(c *gin.Context) {
	var req CreatePoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	pool, err := h.manager.CreatePool(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "cache pool created",
		Data:    pool,
	})
}

// getPool 获取缓存池
func (h *Handlers) getPool(c *gin.Context) {
	id := c.Param("id")
	pool, err := h.manager.GetPool(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    pool,
	})
}

// deletePool 删除缓存池
func (h *Handlers) deletePool(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePool(id); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "cache pool deleted",
	})
}

// updatePoolPolicy 更新缓存池策略
func (h *Handlers) updatePoolPolicy(c *gin.Context) {
	id := c.Param("id")
	var req UpdatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	pool, err := h.manager.UpdatePoolPolicy(id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "pool policy updated",
		Data:    pool,
	})
}

// listMappings 列出缓存映射
func (h *Handlers) listMappings(c *gin.Context) {
	poolID := c.Query("pool_id")
	mappings := h.manager.ListMappings(poolID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    mappings,
	})
}

// createMapping 创建缓存映射
func (h *Handlers) createMapping(c *gin.Context) {
	var req CreateMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	mapping, err := h.manager.CreateMapping(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "cache mapping created",
		Data:    mapping,
	})
}

// getMapping 获取缓存映射
func (h *Handlers) getMapping(c *gin.Context) {
	id := c.Param("id")
	mapping, err := h.manager.GetMapping(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    mapping,
	})
}

// deleteMapping 删除缓存映射
func (h *Handlers) deleteMapping(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteMapping(id); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "cache mapping deleted",
	})
}

// listTierRules 列出分层规则
func (h *Handlers) listTierRules(c *gin.Context) {
	rules := h.manager.ListTierRules()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    rules,
	})
}

// createTierRule 创建分层规则
func (h *Handlers) createTierRule(c *gin.Context) {
	var req CreateTierRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	rule, err := h.manager.CreateTierRule(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "tier rule created",
		Data:    rule,
	})
}

// getTierRule 获取分层规则
func (h *Handlers) getTierRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.manager.GetTierRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    rule,
	})
}

// updateTierRule 更新分层规则
func (h *Handlers) updateTierRule(c *gin.Context) {
	id := c.Param("id")
	var req CreateTierRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	rule, err := h.manager.UpdateTierRule(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "tier rule updated",
		Data:    rule,
	})
}

// deleteTierRule 删除分层规则
func (h *Handlers) deleteTierRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteTierRule(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "tier rule deleted",
	})
}

// listWarmupTasks 列出预热任务
func (h *Handlers) listWarmupTasks(c *gin.Context) {
	poolID := c.Query("pool_id")
	tasks := h.manager.ListWarmupTasks(poolID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    tasks,
	})
}

// createWarmupTask 创建预热任务
func (h *Handlers) createWarmupTask(c *gin.Context) {
	var req CreateWarmupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	task, err := h.manager.CreateWarmupTask(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "warmup task created",
		Data:    task,
	})
}

// getWarmupTask 获取预热任务
func (h *Handlers) getWarmupTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.GetWarmupTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    task,
	})
}

// cancelWarmupTask 取消预热任务
func (h *Handlers) cancelWarmupTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.CancelWarmupTask(id); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "warmup task cancelled",
	})
}

// listConsistencyChecks 列出一致性检查
func (h *Handlers) listConsistencyChecks(c *gin.Context) {
	poolID := c.Query("pool_id")
	checks := h.manager.ListConsistencyChecks(poolID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    checks,
	})
}

// startConsistencyCheck 启动一致性检查
func (h *Handlers) startConsistencyCheck(c *gin.Context) {
	poolID := c.Query("pool_id")
	if poolID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "pool_id is required",
		})
		return
	}

	check, err := h.manager.StartConsistencyCheck(poolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "consistency check started",
		Data:    check,
	})
}

// getConsistencyCheck 获取一致性检查结果
func (h *Handlers) getConsistencyCheck(c *gin.Context) {
	id := c.Param("id")
	check, err := h.manager.GetConsistencyCheck(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    check,
	})
}

// getStats 获取缓存统计
func (h *Handlers) getStats(c *gin.Context) {
	poolID := c.Param("pool_id")
	stats, err := h.manager.GetStats(poolID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// getStatsHistory 获取统计历史
func (h *Handlers) getStatsHistory(c *gin.Context) {
	poolID := c.Param("pool_id")
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	history := h.manager.GetStatsHistory(poolID, limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    history,
	})
}

// flushCache 刷回缓存
func (h *Handlers) flushCache(c *gin.Context) {
	var req FlushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.FlushCache(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "cache flush completed",
	})
}

// invalidateCache 失效缓存
func (h *Handlers) invalidateCache(c *gin.Context) {
	var req struct {
		PoolID string   `json:"pool_id" binding:"required"`
		Paths  []string `json:"paths" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.InvalidateCache(req.PoolID, req.Paths); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "cache invalidated",
	})
}

// getConfig 获取配置
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cfg,
	})
}

// updateConfig 更新配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg CacheGlobalConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	h.manager.UpdateConfig(&cfg)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "config updated",
	})
}

// getOverview 获取系统概览
func (h *Handlers) getOverview(c *gin.Context) {
	overview := h.manager.GetSystemOverview()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    overview,
	})
}

// getSupportedPolicies 获取支持的缓存策略
func (h *Handlers) getSupportedPolicies(c *gin.Context) {
	policies := h.manager.GetSupportedPolicies()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    policies,
	})
}

// getSupportedEvictions 获取支持的淘汰策略
func (h *Handlers) getSupportedEvictions(c *gin.Context) {
	evictions := h.manager.GetSupportedEvictions()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    evictions,
	})
}

// getSupportedRAIDLevels 获取支持的 RAID 级别
func (h *Handlers) getSupportedRAIDLevels(c *gin.Context) {
	levels := h.manager.GetSupportedRAIDLevels()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    levels,
	})
}
