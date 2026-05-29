// Package containerautoupdate 提供 REST API 处理器
package containerautoupdate

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 容器自动更新 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	cu := r.Group("/container/autoupdate")
	{
		// 容器管理
		cu.GET("/containers", h.ListContainers)
		cu.POST("/containers", h.AddContainer)
		cu.GET("/containers/:id", h.GetContainer)
		cu.PUT("/containers/:id", h.UpdateContainer)
		cu.DELETE("/containers/:id", h.RemoveContainer)

		// 更新操作
		cu.POST("/check", h.CheckUpdates)
		cu.POST("/check/:id", h.CheckContainerUpdate)
		cu.POST("/apply/:id", h.ApplyUpdate)
		cu.POST("/rollback/:id", h.Rollback)

		// 健康检查
		cu.GET("/health/:id", h.HealthCheck)

		// 历史和统计
		cu.GET("/history", h.GetHistory)
		cu.GET("/history/:id", h.GetContainerHistory)
		cu.GET("/stats", h.GetStats)
		cu.DELETE("/history", h.ClearHistory)
	}
}

// ========== 容器管理接口 ==========

// ListContainers 列出所有容器.
func (h *Handlers) ListContainers(c *gin.Context) {
	containers := h.manager.ListContainers()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":      len(containers),
			"containers": containers,
		},
	})
}

// AddContainer 添加容器.
func (h *Handlers) AddContainer(c *gin.Context) {
	var req AddContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	container := h.manager.AddContainer(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: container})
}

// GetContainer 获取容器详情.
func (h *Handlers) GetContainer(c *gin.Context) {
	id := c.Param("id")
	container, err := h.manager.GetContainer(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: container})
}

// UpdateContainer 更新容器配置.
func (h *Handlers) UpdateContainer(c *gin.Context) {
	id := c.Param("id")
	var req AddContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	container, err := h.manager.UpdateContainer(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: container})
}

// RemoveContainer 移除容器.
func (h *Handlers) RemoveContainer(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RemoveContainer(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "removed"})
}

// ========== 更新操作接口 ==========

// CheckUpdates 检查所有容器更新.
func (h *Handlers) CheckUpdates(c *gin.Context) {
	updates, err := h.manager.CheckUpdates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(updates),
			"updates": updates,
		},
	})
}

// CheckContainerUpdate 检查单个容器更新.
func (h *Handlers) CheckContainerUpdate(c *gin.Context) {
	id := c.Param("id")
	update, err := h.manager.CheckContainerUpdate(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	if update == nil {
		c.JSON(http.StatusOK, response{Code: 0, Message: "no update available", Data: nil})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "update available", Data: update})
}

// ApplyUpdate 应用更新.
func (h *Handlers) ApplyUpdate(c *gin.Context) {
	id := c.Param("id")
	var req ApplyUpdateRequest
	// 请求体是可选的
	_ = c.ShouldBindJSON(&req)

	update, err := h.manager.ApplyUpdate(id, req.NewTag)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "update applied", Data: update})
}

// Rollback 回滚.
func (h *Handlers) Rollback(c *gin.Context) {
	id := c.Param("id")
	var req RollbackRequest
	// 请求体是可选的
	_ = c.ShouldBindJSON(&req)

	update, err := h.manager.Rollback(id, req.UpdateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "rollback completed", Data: update})
}

// ========== 健康检查接口 ==========

// HealthCheck 执行健康检查.
func (h *Handlers) HealthCheck(c *gin.Context) {
	id := c.Param("id")
	healthy, err := h.manager.HealthCheck(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"healthy": healthy,
		},
	})
}

// ========== 历史和统计接口 ==========

// GetHistory 获取更新历史.
func (h *Handlers) GetHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	history := h.manager.GetUpdateHistory(limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(history),
			"history": history,
		},
	})
}

// GetContainerHistory 获取容器更新历史.
func (h *Handlers) GetContainerHistory(c *gin.Context) {
	id := c.Param("id")
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	history := h.manager.GetContainerHistory(id, limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(history),
			"history": history,
		},
	})
}

// GetStats 获取更新统计.
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}

// ClearHistory 清除历史记录.
func (h *Handlers) ClearHistory(c *gin.Context) {
	h.manager.ClearHistory()
	c.JSON(http.StatusOK, response{Code: 0, Message: "history cleared"})
}
