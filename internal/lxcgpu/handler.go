package lxcgpu

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler LXC GPU管理API处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建API处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	api := r.Group("/api/v1/lxcgpu")
	{
		// GPU设备管理
		api.GET("/devices", h.listDevices)
		api.GET("/devices/:pci", h.getDevice)
		api.POST("/devices/refresh", h.refreshDevices)

		// GPU分配管理
		api.POST("/assign", h.assignGPU)
		api.DELETE("/assign", h.unassignGPU)
		api.PUT("/quota", h.updateQuota)
		api.GET("/assignments", h.listAssignments)
		api.GET("/assignments/:id", h.getAssignment)
		api.GET("/containers/:id/gpus", h.getContainerGPUs)

		// 热插拔
		api.POST("/hotplug", h.hotplugGPU)

		// 统计与仪表盘
		api.GET("/dashboard", h.getDashboard)
		api.GET("/containers/:id/stats", h.getContainerStats)

		// 批量操作
		api.POST("/bulk/assign", h.bulkAssign)
	}
}

// ========== GPU设备管理 ==========

// listDevices 列出所有GPU设备
func (h *Handler) listDevices(c *gin.Context) {
	devices := h.manager.GetDeviceManager().ListDevices()
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    devices,
	})
}

// getDevice 获取GPU设备详情
func (h *Handler) getDevice(c *gin.Context) {
	pciAddr := c.Param("pci")

	device, err := h.manager.GetDeviceManager().GetDevice(pciAddr)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    device,
	})
}

// refreshDevices 刷新GPU设备列表
func (h *Handler) refreshDevices(c *gin.Context) {
	devices, err := h.manager.GetDeviceManager().DiscoverDevices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "刷新GPU设备失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "GPU设备列表已刷新",
		Data:    devices,
	})
}

// ========== GPU分配管理 ==========

// assignGPU 分配GPU给容器
func (h *Handler) assignGPU(c *gin.Context) {
	var req AssignGPURequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	assignment, err := h.manager.AssignGPU(&req)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Message: "GPU分配成功",
		Data:    assignment,
	})
}

// unassignGPU 取消GPU分配
func (h *Handler) unassignGPU(c *gin.Context) {
	var req UnassignGPURequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := h.manager.UnassignGPU(&req); err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "GPU分配已取消",
	})
}

// updateQuota 更新GPU资源配额
func (h *Handler) updateQuota(c *gin.Context) {
	var req UpdateQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := h.manager.UpdateQuota(&req); err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "GPU配额已更新",
	})
}

// listAssignments 列出所有分配记录
func (h *Handler) listAssignments(c *gin.Context) {
	// 可按容器ID过滤
	containerID := c.Query("containerId")

	var assignments []*LXCGPUAssignment
	if containerID != "" {
		assignments = h.manager.GetContainerAssignments(containerID)
	} else {
		assignments = h.manager.ListAllAssignments()
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    assignments,
	})
}

// getAssignment 获取分配详情
func (h *Handler) getAssignment(c *gin.Context) {
	id := c.Param("id")

	assignment, err := h.manager.GetAssignment(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    assignment,
	})
}

// getContainerGPUs 获取容器的GPU列表
func (h *Handler) getContainerGPUs(c *gin.Context) {
	containerID := c.Param("id")

	assignments := h.manager.GetContainerAssignments(containerID)
	gpuDevices := h.manager.GetDeviceManager().GetDeviceForContainer(containerID)

	result := gin.H{
		"containerId": containerID,
		"assignments": assignments,
		"gpus":        gpuDevices,
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    result,
	})
}

// ========== 热插拔 ==========

// hotplugGPU GPU热插拔操作
func (h *Handler) hotplugGPU(c *gin.Context) {
	var req HotplugRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	// 验证action参数
	if req.Action != "attach" && req.Action != "detach" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "action参数无效，仅支持 'attach' 或 'detach'",
		})
		return
	}

	assignment, err := h.manager.HotplugGPU(&req)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	message := "GPU附加成功"
	if req.Action == "detach" {
		message = "GPU分离成功"
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: message,
		Data:    assignment,
	})
}

// ========== 统计与仪表盘 ==========

// getDashboard 获取GPU分配仪表盘
func (h *Handler) getDashboard(c *gin.Context) {
	dashboard := h.manager.GetDashboard()

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    dashboard,
	})
}

// getContainerStats 获取容器GPU统计
func (h *Handler) getContainerStats(c *gin.Context) {
	containerID := c.Param("id")

	stats, err := h.manager.GetContainerGPUStats(containerID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    stats,
	})
}

// ========== 批量操作 ==========

// bulkAssign 批量分配GPU给多个容器
func (h *Handler) bulkAssign(c *gin.Context) {
	var req BulkAssignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	// 批量分配需要MPS或时间片共享模式
	if req.ShareMode == ShareModeExclusive && len(req.ContainerIDs) > 1 {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "独占模式下无法批量分配给多个容器",
		})
		return
	}

	results := make([]gin.H, 0, len(req.ContainerIDs))
	var succeeded, failed int

	for _, containerID := range req.ContainerIDs {
		assignReq := &AssignGPURequest{
			ContainerID: containerID,
			GPUPCIAddr:  req.GPUPCIAddr,
			ShareMode:   req.ShareMode,
			GPUQuota:    req.GPUQuota,
		}

		assignment, err := h.manager.AssignGPU(assignReq)
		if err != nil {
			failed++
			results = append(results, gin.H{
				"containerId": containerID,
				"success":     false,
				"error":       err.Error(),
			})
		} else {
			succeeded++
			results = append(results, gin.H{
				"containerId": containerID,
				"success":     true,
				"assignment":  assignment,
			})
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "批量分配完成",
		Data: gin.H{
			"succeeded": succeeded,
			"failed":    failed,
			"details":   results,
		},
	})
}
