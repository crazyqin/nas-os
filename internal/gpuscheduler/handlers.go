// Package gpuscheduler 提供 REST API 处理器
package gpuscheduler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers GPU 调度器 API 处理器
type Handlers struct {
	scheduler *Scheduler
	logger    *zap.Logger
}

// NewHandlers 创建处理器
func NewHandlers(scheduler *Scheduler, logger *zap.Logger) *Handlers {
	return &Handlers{
		scheduler: scheduler,
		logger:    logger,
	}
}

// RegisterRoutes 注册路由到 /api/gpuscheduler 路由组
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	gs := r.Group("/gpuscheduler")
	{
		// GPU 设备管理
		gs.GET("/devices", h.listDevices)
		gs.GET("/devices/:id", h.getDevice)

		// 资源分配
		gs.POST("/allocate", h.allocate)
		gs.DELETE("/allocate/:id", h.release)

		// 统计信息
		gs.GET("/stats", h.getStats)

		// 调度策略
		gs.GET("/policy", h.getPolicy)
		gs.PUT("/policy", h.updatePolicy)
	}
}

// ========== GPU 设备 Handlers ==========

// listDevices 列出所有 GPU 设备
func (h *Handlers) listDevices(c *gin.Context) {
	devices := h.scheduler.ListDevices()
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    devices,
	})
}

// getDevice 获取指定 GPU 设备信息
func (h *Handlers) getDevice(c *gin.Context) {
	id := c.Param("id")
	device, err := h.scheduler.GetDevice(id)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, Response{
				Code:    1,
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    device,
	})
}

// ========== 资源分配 Handlers ==========

// allocate 分配 GPU 资源
func (h *Handlers) allocate(c *gin.Context) {
	var req AllocateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    1,
			Message: "请求参数无效: " + err.Error(),
		})
		return
	}

	allocation, err := h.scheduler.Allocate(req)
	if err != nil {
		// 根据错误类型返回不同状态码
		switch err.(type) {
		case *NotFoundError:
			c.JSON(http.StatusNotFound, Response{Code: 1, Message: err.Error()})
		case *InsufficientResourceError:
			c.JSON(http.StatusConflict, Response{Code: 1, Message: err.Error()})
		case *ValidationError:
			c.JSON(http.StatusBadRequest, Response{Code: 1, Message: err.Error()})
		case *PolicyViolationError:
			c.JSON(http.StatusForbidden, Response{Code: 1, Message: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, Response{Code: 1, Message: err.Error()})
		}
		return
	}

	h.logger.Info("GPU 资源分配成功",
		zap.String("allocation_id", allocation.ID),
		zap.String("container_id", req.ContainerID))

	c.JSON(http.StatusCreated, Response{
		Code:    0,
		Message: "分配成功",
		Data:    allocation,
	})
}

// release 释放 GPU 资源
func (h *Handlers) release(c *gin.Context) {
	id := c.Param("id")
	if err := h.scheduler.Release(id); err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, Response{
				Code:    1,
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	h.logger.Info("GPU 资源释放成功", zap.String("allocation_id", id))

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "释放成功",
	})
}

// ========== 统计信息 Handlers ==========

// getStats 获取 GPU 使用统计
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.scheduler.GetStats()
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// ========== 调度策略 Handlers ==========

// getPolicy 获取当前调度策略
func (h *Handlers) getPolicy(c *gin.Context) {
	policy := h.scheduler.GetPolicy()
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    policy,
	})
}

// updatePolicy 更新调度策略
func (h *Handlers) updatePolicy(c *gin.Context) {
	var req UpdatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    1,
			Message: "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := h.scheduler.UpdatePolicy(req); err != nil {
		if _, ok := err.(*ValidationError); ok {
			c.JSON(http.StatusBadRequest, Response{
				Code:    1,
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	h.logger.Info("调度策略更新成功")

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "策略更新成功",
		Data:    h.scheduler.GetPolicy(),
	})
}
