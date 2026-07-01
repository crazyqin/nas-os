// Package lxcgpupass HTTP 处理器
// 提供 GPU 直通管理的 REST API
package lxcgpupass

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler GPU 直通管理 API 处理器
type Handler struct {
	service *Service
}

// NewHandler 创建 API 处理器
func NewHandler(svc *Service) *Handler {
	return &Handler{service: svc}
}

// RegisterRoutes 注册路由到指定路由组
// 路由前缀: /api/v1/lxcgpupass
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/api/v1/lxcgpupass")
	{
		g.GET("/devices", h.listDevices)       // 列出所有 GPU 设备
		g.POST("/assign", h.assignGPU)         // 分配 GPU 到容器
		g.POST("/remove", h.removeGPU)         // 从容器移除 GPU
		g.GET("/status", h.getStatus)          // 查看分配状态
	}
}

// listDevices 列出所有 GPU 设备
// GET /api/v1/lxcgpupass/devices
func (h *Handler) listDevices(c *gin.Context) {
	devices := h.service.GetDevices()
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    devices,
	})
}

// assignGPU 将 GPU 设备分配给 LXC 容器
// POST /api/v1/lxcgpupass/assign
// Body: {"containerId":"lxc-100","gpuPciAddr":"0000:01:00.0"}
func (h *Handler) assignGPU(c *gin.Context) {
	var req AssignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	assignment, err := h.service.AssignGPU(&req)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Message: "GPU 分配成功",
		Data:    assignment,
	})
}

// removeGPU 从容器移除 GPU 分配
// POST /api/v1/lxcgpupass/remove
// Body: {"containerId":"lxc-100","gpuPciAddr":"0000:01:00.0","force":false}
func (h *Handler) removeGPU(c *gin.Context) {
	var req RemoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := h.service.RemoveGPU(&req); err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "GPU 分配已移除",
	})
}

// getStatus 获取 GPU 分配状态总览
// GET /api/v1/lxcgpupass/status
func (h *Handler) getStatus(c *gin.Context) {
	status := h.service.GetStatus()
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    status,
	})
}
