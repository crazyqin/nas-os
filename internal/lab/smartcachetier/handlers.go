// Package smartcachetier 提供多级缓存智能管理功能
package smartcachetier

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 多级缓存 HTTP 处理器.
type Handlers struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager, logger *zap.Logger) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handlers{
		manager: mgr,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	ct := api.Group("/smartcachetier")
	{
		// 层级管理
		ct.POST("/tiers", h.createTier)
		ct.GET("/tiers", h.listTiers)
		ct.GET("/tiers/:level", h.getTier)
		ct.DELETE("/tiers/:level", h.deleteTier)

		// 缓存操作
		ct.POST("/cache", h.setCache)
		ct.GET("/cache/:key", h.getCache)
		ct.DELETE("/cache/:key", h.deleteCache)

		// 分层操作
		ct.POST("/cache/:key/promote", h.promoteEntry)
		ct.POST("/cache/:key/demote", h.demoteEntry)
		ct.POST("/auto-tiering", h.runAutoTiering)

		// 统计和配置
		ct.GET("/stats", h.getStats)
		ct.GET("/config", h.getConfig)
		ct.PUT("/config", h.updateConfig)
	}
}

// ========== 通用响应 ==========

// Response 通用 API 响应结构.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 返回成功响应.
func Success(data interface{}) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

// Error 返回错误响应.
func Error(code int, message string) Response {
	return Response{Code: code, Message: message}
}

// ========== 层级 API ==========

// createTier 创建缓存层级.
func (h *Handlers) createTier(c *gin.Context) {
	var req TierCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	tier, err := h.manager.CreateTier(req)
	if err != nil {
		c.JSON(http.StatusConflict, Error(409, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, Success(tier))
}

// listTiers 列出所有缓存层级.
func (h *Handlers) listTiers(c *gin.Context) {
	tiers := h.manager.ListTiers()
	c.JSON(http.StatusOK, Success(tiers))
}

// getTier 获取缓存层级.
func (h *Handlers) getTier(c *gin.Context) {
	level, err := strconv.Atoi(c.Param("level"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Error(400, "无效的层级参数"))
		return
	}

	tier, err := h.manager.GetTier(TierLevel(level))
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(tier))
}

// deleteTier 删除缓存层级.
func (h *Handlers) deleteTier(c *gin.Context) {
	level, err := strconv.Atoi(c.Param("level"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Error(400, "无效的层级参数"))
		return
	}

	if err := h.manager.DeleteTier(TierLevel(level)); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

// ========== 缓存 API ==========

// setCache 设置缓存.
func (h *Handlers) setCache(c *gin.Context) {
	var req CacheSetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	entry, err := h.manager.Set(req)
	if err != nil {
		c.JSON(http.StatusConflict, Error(409, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, Success(entry))
}

// getCache 获取缓存.
func (h *Handlers) getCache(c *gin.Context) {
	key := c.Param("key")

	entry, err := h.manager.Get(key)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(entry))
}

// deleteCache 删除缓存.
func (h *Handlers) deleteCache(c *gin.Context) {
	key := c.Param("key")

	if err := h.manager.Delete(key); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

// promoteEntry 提升缓存条目.
func (h *Handlers) promoteEntry(c *gin.Context) {
	key := c.Param("key")

	if err := h.manager.PromoteEntry(key); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

// demoteEntry 降级缓存条目.
func (h *Handlers) demoteEntry(c *gin.Context) {
	key := c.Param("key")

	if err := h.manager.DemoteEntry(key); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

// runAutoTiering 执行自动分层.
func (h *Handlers) runAutoTiering(c *gin.Context) {
	promoted, demoted := h.manager.RunAutoTiering()
	c.JSON(http.StatusOK, Success(gin.H{
		"promoted": promoted,
		"demoted":  demoted,
	}))
}

// ========== 统计 API ==========

// getStats 获取缓存统计.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, Success(stats))
}

// getConfig 获取缓存配置.
func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, Success(config))
}

// updateConfig 更新缓存配置.
func (h *Handlers) updateConfig(c *gin.Context) {
	var config CacheConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.manager.UpdateConfig(&config); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}
