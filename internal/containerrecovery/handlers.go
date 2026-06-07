// Package containerrecovery 提供 REST API 处理器
package containerrecovery

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 容器恢复模块 API 处理器.
type Handlers struct {
	engine *Engine
}

// NewHandlers 创建处理器.
func NewHandlers(engine *Engine) *Handlers {
	return &Handlers{engine: engine}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	cr := r.Group("/container-recovery")
	{
		cr.GET("/status", h.getStatus)
		cr.GET("/containers", h.listContainers)
		cr.POST("/containers", h.registerContainer)
		cr.DELETE("/containers/:name", h.unregisterContainer)
		cr.GET("/containers/:name", h.getContainer)
		cr.POST("/containers/:name/recover", h.triggerRecovery)
		cr.GET("/containers/:name/records", h.getContainerRecords)
		cr.GET("/records", h.getRecords)
		cr.GET("/stats", h.getStats)
		cr.GET("/config", h.getConfig)
		cr.PUT("/config", h.updateConfig)
		cr.POST("/start", h.startEngine)
		cr.POST("/stop", h.stopEngine)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// getStatus 获取引擎状态.
func (h *Handlers) getStatus(c *gin.Context) {
	cfg := h.engine.GetConfig()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"running": h.engine.IsRunning(),
			"enabled": cfg.Enabled,
		},
	})
}

// listContainers 列出所有已注册容器.
func (h *Handlers) listContainers(c *gin.Context) {
	containers := h.engine.ListContainers()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":      len(containers),
			"containers": containers,
		},
	})
}

// RegisterContainerRequest 注册容器请求.
type RegisterContainerRequest struct {
	ContainerName string            `json:"container_name" binding:"required"`
	Enabled       bool              `json:"enabled"`
	HealthCheck   HealthCheckConfig `json:"health_check" binding:"required"`
	Strategy      RecoveryStrategy  `json:"strategy"`
	Dependencies  []string          `json:"dependencies,omitempty"`
	Priority      int               `json:"priority"`
	Hooks         []RecoveryHook    `json:"hooks,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
}

// registerContainer 注册容器.
func (h *Handlers) registerContainer(c *gin.Context) {
	var req RegisterContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// 应用默认值
	strategy := req.Strategy
	if strategy.MaxRetries == 0 {
		strategy = DefaultRecoveryStrategy()
	}

	cfg := &ContainerConfig{
		ContainerName: req.ContainerName,
		Enabled:       req.Enabled,
		HealthCheck:   req.HealthCheck,
		Strategy:      strategy,
		Dependencies:  req.Dependencies,
		Priority:      req.Priority,
		Hooks:         req.Hooks,
		Labels:        req.Labels,
	}

	h.engine.RegisterContainer(cfg)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "container registered",
		Data:    cfg,
	})
}

// unregisterContainer 注销容器.
func (h *Handlers) unregisterContainer(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "container name is required",
		})
		return
	}

	h.engine.UnregisterContainer(name)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "container unregistered",
	})
}

// getContainer 获取容器配置.
func (h *Handlers) getContainer(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "container name is required",
		})
		return
	}

	cfg, ok := h.engine.GetContainer(name)
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: "container not found",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cfg,
	})
}

// triggerRecovery 手动触发恢复.
func (h *Handlers) triggerRecovery(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "container name is required",
		})
		return
	}

	record, err := h.engine.TriggerRecovery(name)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "recovery triggered",
		Data:    record,
	})
}

// getContainerRecords 获取容器恢复记录.
func (h *Handlers) getContainerRecords(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "container name is required",
		})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	records, err := h.engine.GetRecoveryRecords(name, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"container": name,
			"total":     len(records),
			"records":   records,
		},
	})
}

// getRecords 获取所有恢复记录.
func (h *Handlers) getRecords(c *gin.Context) {
	container := c.Query("container")
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	records, err := h.engine.GetRecoveryRecords(container, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(records),
			"records": records,
		},
	})
}

// getStats 获取恢复统计.
func (h *Handlers) getStats(c *gin.Context) {
	stats, err := h.engine.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
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

// getConfig 获取引擎配置.
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.engine.GetConfig()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cfg,
	})
}

// updateConfig 更新引擎配置.
func (h *Handlers) updateConfig(c *gin.Context) {
	var config EngineConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid config: " + err.Error(),
		})
		return
	}

	h.engine.UpdateConfig(config)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "config updated",
		Data:    h.engine.GetConfig(),
	})
}

// startEngine 启动引擎.
func (h *Handlers) startEngine(c *gin.Context) {
	if h.engine.IsRunning() {
		c.JSON(http.StatusOK, response{
			Code:    0,
			Message: "engine already running",
		})
		return
	}

	h.engine.Start()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "engine started",
	})
}

// stopEngine 停止引擎.
func (h *Handlers) stopEngine(c *gin.Context) {
	if !h.engine.IsRunning() {
		c.JSON(http.StatusOK, response{
			Code:    0,
			Message: "engine already stopped",
		})
		return
	}

	h.engine.Stop()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "engine stopped",
	})
}

// ========== 高级查询 API ==========

// GetRecoveryOrderRequest 获取恢复顺序请求.
type GetRecoveryOrderRequest struct {
	Containers []string `json:"containers" binding:"required"`
}

// getRecoveryOrder 获取恢复顺序.
func (h *Handlers) getRecoveryOrder(c *gin.Context) {
	var req GetRecoveryOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	order := h.engine.depGraph.GetRecoveryOrder(req.Containers)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"containers": req.Containers,
			"order":      order,
		},
	})
}

// getDependents 获取依赖指定容器的容器.
func (h *Handlers) getDependents(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "container name is required",
		})
		return
	}

	dependents := h.engine.depGraph.GetDependents(name)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"container":  name,
			"dependents": dependents,
		},
	})
}

// getDependencies 获取容器的依赖.
func (h *Handlers) getDependencies(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "container name is required",
		})
		return
	}

	deps := h.engine.depGraph.GetDependencies(name)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"container":    name,
			"dependencies": deps,
		},
	})
}

// ========== 批量操作 API ==========

// BatchRecoveryRequest 批量恢复请求.
type BatchRecoveryRequest struct {
	Containers []string `json:"containers" binding:"required"`
}

// batchRecovery 批量恢复.
func (h *Handlers) batchRecovery(c *gin.Context) {
	var req BatchRecoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// 按依赖顺序恢复
	order := h.engine.depGraph.GetRecoveryOrder(req.Containers)

	results := make(map[string]*RecoveryRecord)
	var lastErr error

	for _, container := range order {
		record, err := h.engine.TriggerRecovery(container)
		if err != nil {
			lastErr = err
			results[container] = &RecoveryRecord{
				Container:    container,
				Status:       RecoveryStatusFailed,
				ErrorMessage: err.Error(),
			}
		} else {
			results[container] = record
		}
	}

	httpStatus := http.StatusOK
	if lastErr != nil {
		httpStatus = http.StatusMultiStatus
	}

	c.JSON(httpStatus, response{
		Code:    0,
		Message: "batch recovery completed",
		Data: gin.H{
			"order":   order,
			"results": results,
		},
	})
}

// ========== 钩子管理 API ==========

// AddHookRequest 添加钩子请求.
type AddHookRequest struct {
	Container string       `json:"container" binding:"required"`
	Hook      RecoveryHook `json:"hook" binding:"required"`
}

// addHook 添加钩子.
func (h *Handlers) addHook(c *gin.Context) {
	var req AddHookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	h.engine.mu.Lock()
	defer h.engine.mu.Unlock()

	if _, ok := h.engine.containers[req.Container]; !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: "container not found",
		})
		return
	}

	h.engine.hooks[req.Container] = append(h.engine.hooks[req.Container], req.Hook)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "hook added",
	})
}

// removeHook 移除钩子.
func (h *Handlers) removeHook(c *gin.Context) {
	container := c.Query("container")
	hookName := c.Query("hook_name")

	if container == "" || hookName == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "container and hook_name are required",
		})
		return
	}

	h.engine.mu.Lock()
	defer h.engine.mu.Unlock()

	hooks := h.engine.hooks[container]
	for i, hook := range hooks {
		if hook.Name == hookName {
			h.engine.hooks[container] = append(hooks[:i], hooks[i+1:]...)
			c.JSON(http.StatusOK, response{
				Code:    0,
				Message: "hook removed",
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, response{
		Code:    1,
		Message: "hook not found",
	})
}

// ========== 工具函数 ==========

// parseDuration 解析持续时间字符串.
func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}
