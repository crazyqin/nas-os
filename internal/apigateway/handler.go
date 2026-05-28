// Package apigateway 提供 REST API 处理器
package apigateway

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers API 网关 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	gw := r.Group("/api-gateway")
	{
		// 网关状态
		gw.GET("/status", h.getStatus)
		gw.GET("/stats", h.getStats)
		gw.GET("/config", h.getConfig)
		gw.PUT("/config", h.updateConfig)

		// 网关控制
		gw.POST("/start", h.start)
		gw.POST("/stop", h.stop)

		// 路由管理
		routes := gw.Group("/routes")
		{
			routes.GET("", h.listRoutes)
			routes.POST("", h.addRoute)
			routes.GET("/:id", h.getRoute)
			routes.PUT("/:id", h.updateRoute)
			routes.DELETE("/:id", h.deleteRoute)
		}

		// 上游服务管理
		upstreams := gw.Group("/upstreams")
		{
			upstreams.GET("", h.listUpstreams)
			upstreams.POST("", h.addUpstream)
			upstreams.GET("/:id", h.getUpstream)
			upstreams.PUT("/:id", h.updateUpstream)
			upstreams.DELETE("/:id", h.deleteUpstream)

			// 目标管理
			upstreams.GET("/:id/targets", h.listTargets)
			upstreams.POST("/:id/targets", h.addTarget)
			upstreams.DELETE("/:id/targets/:targetId", h.removeTarget)
		}

		// 消费者管理
		consumers := gw.Group("/consumers")
		{
			consumers.GET("", h.listConsumers)
			consumers.POST("", h.addConsumer)
			consumers.GET("/:id", h.getConsumer)
			consumers.DELETE("/:id", h.deleteConsumer)
		}

		// API Key 管理
		apiKeys := gw.Group("/api-keys")
		{
			apiKeys.GET("", h.listAPIKeys)
			apiKeys.POST("", h.addAPIKey)
			apiKeys.DELETE("/:key", h.deleteAPIKey)
		}

		// API 版本管理
		versions := gw.Group("/versions")
		{
			versions.GET("", h.listAPIVersions)
			versions.POST("", h.addAPIVersion)
			versions.GET("/:version", h.getAPIVersion)
		}

		// 熔断器状态
		gw.GET("/circuit-breakers/:upstreamId", h.getCircuitBreakerState)

		// 请求日志
		gw.GET("/logs", h.getRequestLogs)
		gw.DELETE("/logs", h.clearRequestLogs)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ==================== 网关状态 ====================

// getStatus 获取网关状态
func (h *Handlers) getStatus(c *gin.Context) {
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"running":   h.manager.IsRunning(),
			"uptime":    time.Since(h.manager.startTime).String(),
			"routes":    len(h.manager.routes),
			"upstreams": len(h.manager.upstreams),
		},
	})
}

// getStats 获取网关统计
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
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
	var cfg GatewayConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}
	h.manager.UpdateConfig(&cfg)
	c.JSON(http.StatusOK, response{Code: 0, Message: "config updated"})
}

// start 启动网关
func (h *Handlers) start(c *gin.Context) {
	if err := h.manager.Start(); err != nil {
		c.JSON(http.StatusConflict, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "gateway started"})
}

// stop 停止网关
func (h *Handlers) stop(c *gin.Context) {
	if err := h.manager.Stop(); err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "gateway stopped"})
}

// ==================== 路由管理 ====================

// listRoutes 列出路由
func (h *Handlers) listRoutes(c *gin.Context) {
	routes := h.manager.ListRoutes()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: routes})
}

// addRoute 添加路由
func (h *Handlers) addRoute(c *gin.Context) {
	var route Route
	if err := c.ShouldBindJSON(&route); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}
	if err := h.manager.AddRoute(&route); err != nil {
		c.JSON(http.StatusConflict, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, response{Code: 0, Message: "route created", Data: route})
}

// getRoute 获取路由
func (h *Handlers) getRoute(c *gin.Context) {
	id := c.Param("id")
	route, err := h.manager.GetRoute(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: route})
}

// updateRoute 更新路由
func (h *Handlers) updateRoute(c *gin.Context) {
	id := c.Param("id")
	var route Route
	if err := c.ShouldBindJSON(&route); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}
	route.ID = id
	if err := h.manager.UpdateRoute(&route); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "route updated", Data: route})
}

// deleteRoute 删除路由
func (h *Handlers) deleteRoute(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRoute(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "route deleted"})
}

// ==================== 上游服务管理 ====================

// listUpstreams 列出上游服务
func (h *Handlers) listUpstreams(c *gin.Context) {
	upstreams := h.manager.ListUpstreams()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: upstreams})
}

// addUpstream 添加上游服务
func (h *Handlers) addUpstream(c *gin.Context) {
	var upstream Upstream
	if err := c.ShouldBindJSON(&upstream); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}
	if err := h.manager.AddUpstream(&upstream); err != nil {
		c.JSON(http.StatusConflict, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, response{Code: 0, Message: "upstream created", Data: upstream})
}

// getUpstream 获取上游服务
func (h *Handlers) getUpstream(c *gin.Context) {
	id := c.Param("id")
	upstream, err := h.manager.GetUpstream(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: upstream})
}

// updateUpstream 更新上游服务
func (h *Handlers) updateUpstream(c *gin.Context) {
	id := c.Param("id")
	var upstream Upstream
	if err := c.ShouldBindJSON(&upstream); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}
	upstream.ID = id
	if err := h.manager.UpdateUpstream(&upstream); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "upstream updated", Data: upstream})
}

// deleteUpstream 删除上游服务
func (h *Handlers) deleteUpstream(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteUpstream(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "upstream deleted"})
}

// listTargets 列出目标
func (h *Handlers) listTargets(c *gin.Context) {
	upstreamID := c.Param("id")
	upstream, err := h.manager.GetUpstream(upstreamID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: upstream.Targets})
}

// addTarget 添加目标
func (h *Handlers) addTarget(c *gin.Context) {
	upstreamID := c.Param("id")
	var target Target
	if err := c.ShouldBindJSON(&target); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}
	if err := h.manager.AddTarget(upstreamID, &target); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, response{Code: 0, Message: "target added", Data: target})
}

// removeTarget 移除目标
func (h *Handlers) removeTarget(c *gin.Context) {
	upstreamID := c.Param("id")
	targetID := c.Param("targetId")
	if err := h.manager.RemoveTarget(upstreamID, targetID); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "target removed"})
}

// ==================== 消费者管理 ====================

// listConsumers 列出消费者
func (h *Handlers) listConsumers(c *gin.Context) {
	consumers := h.manager.ListConsumers()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: consumers})
}

// addConsumer 添加消费者
func (h *Handlers) addConsumer(c *gin.Context) {
	var consumer Consumer
	if err := c.ShouldBindJSON(&consumer); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}
	if err := h.manager.AddConsumer(&consumer); err != nil {
		c.JSON(http.StatusConflict, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, response{Code: 0, Message: "consumer created", Data: consumer})
}

// getConsumer 获取消费者
func (h *Handlers) getConsumer(c *gin.Context) {
	id := c.Param("id")
	consumer, err := h.manager.GetConsumer(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: consumer})
}

// deleteConsumer 删除消费者
func (h *Handlers) deleteConsumer(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteConsumer(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "consumer deleted"})
}

// ==================== API Key 管理 ====================

// listAPIKeys 列出 API Key
func (h *Handlers) listAPIKeys(c *gin.Context) {
	consumerID := c.Query("consumer_id")
	keys := h.manager.ListAPIKeys(consumerID)
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: keys})
}

// addAPIKey 添加 API Key
func (h *Handlers) addAPIKey(c *gin.Context) {
	var keyInfo APIKeyInfo
	if err := c.ShouldBindJSON(&keyInfo); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}
	if err := h.manager.AddAPIKey(&keyInfo); err != nil {
		c.JSON(http.StatusConflict, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, response{Code: 0, Message: "api key created", Data: keyInfo})
}

// deleteAPIKey 删除 API Key
func (h *Handlers) deleteAPIKey(c *gin.Context) {
	key := c.Param("key")
	if err := h.manager.DeleteAPIKey(key); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "api key deleted"})
}

// ==================== API 版本管理 ====================

// listAPIVersions 列出 API 版本
func (h *Handlers) listAPIVersions(c *gin.Context) {
	versions := h.manager.ListAPIVersions()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: versions})
}

// addAPIVersion 添加 API 版本
func (h *Handlers) addAPIVersion(c *gin.Context) {
	var version APIVersion
	if err := c.ShouldBindJSON(&version); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}
	h.manager.AddAPIVersion(&version)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "api version added", Data: version})
}

// getAPIVersion 获取 API 版本
func (h *Handlers) getAPIVersion(c *gin.Context) {
	version := c.Param("version")
	v, err := h.manager.GetAPIVersion(version)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: v})
}

// ==================== 熔断器状态 ====================

// getCircuitBreakerState 获取熔断器状态
func (h *Handlers) getCircuitBreakerState(c *gin.Context) {
	upstreamID := c.Param("upstreamId")
	state, err := h.manager.GetCircuitBreakerState(upstreamID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"upstream_id": upstreamID,
			"state":       state,
		},
	})
}

// ==================== 请求日志 ====================

// getRequestLogs 获取请求日志
func (h *Handlers) getRequestLogs(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	logs := h.manager.GetRequestLogs(limit)
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: logs})
}

// clearRequestLogs 清除请求日志
func (h *Handlers) clearRequestLogs(c *gin.Context) {
	h.manager.ClearRequestLogs()
	c.JSON(http.StatusOK, response{Code: 0, Message: "logs cleared"})
}

// ==================== 代理处理器 ====================

// ProxyHandler 创建代理处理器
func (h *Handlers) ProxyHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 匹配路由
		route := h.manager.MatchRoute(c.Request.Method, c.Request.URL.Path)
		if route == nil {
			c.JSON(http.StatusNotFound, response{
				Code:    1,
				Message: fmt.Sprintf("no route found for %s %s", c.Request.Method, c.Request.URL.Path),
			})
			return
		}

		// 获取上游服务
		upstream, err := h.manager.GetUpstream(route.UpstreamID)
		if err != nil {
			c.JSON(http.StatusBadGateway, response{Code: 1, Message: "upstream not found"})
			return
		}

		// 选择目标
		target, err := h.manager.SelectTarget(upstream.ID)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, response{Code: 1, Message: err.Error()})
			return
		}

		// 限流检查
		if h.manager.config.RateLimit.Enabled && h.manager.rateLimiter != nil {
			clientIP := c.ClientIP()
			if !h.manager.rateLimiter.Allow(clientIP) {
				c.JSON(http.StatusTooManyRequests, response{Code: 1, Message: "rate limit exceeded"})
				return
			}
		}

		// 认证检查
		if h.manager.config.Auth.Enabled {
			if !h.authenticate(c) {
				c.JSON(http.StatusUnauthorized, response{Code: 1, Message: "unauthorized"})
				return
			}
		}

		// 创建反向代理
		proxy := h.manager.CreateReverseProxy(target, route)

		// 记录请求日志
		log := &RequestLog{
			Method:       c.Request.Method,
			Path:         c.Request.URL.Path,
			ClientIP:     c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			UpstreamHost: fmt.Sprintf("%s:%d", target.Host, target.Port),
			RequestSize:  c.Request.ContentLength,
		}

		// 执行代理
		proxy.ServeHTTP(c.Writer, c.Request)

		// 记录响应
		log.StatusCode = c.Writer.Status()
		log.Duration = time.Since(start)
		log.ResponseSize = int64(c.Writer.Size())
		h.manager.LogRequest(log)
	}
}

// authenticate 认证请求
func (h *Handlers) authenticate(c *gin.Context) bool {
	switch h.manager.config.Auth.Type {
	case AuthTypeAPIKey:
		// 从 header 或 query 获取 API Key
		key := c.GetHeader(h.manager.config.Auth.HeaderName)
		if key == "" && h.manager.config.Auth.QueryParam != "" {
			key = c.Query(h.manager.config.Auth.QueryParam)
		}
		if key == "" {
			return false
		}
		keyInfo, err := h.manager.ValidateAPIKey(key)
		if err != nil {
			return false
		}
		c.Set("consumer_id", keyInfo.ConsumerID)
		return true

	case AuthTypeJWT, AuthTypeOAuth2:
		// JWT 和 OAuth2 需要额外实现
		return true

	default:
		return true
	}
}
