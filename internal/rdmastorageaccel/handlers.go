package rdmastorageaccel

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler HTTP API 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建新的处理器实例
func NewHandler(manager *Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	rdmaGroup := router.Group("/rdmastorage")
	{
		// 设备管理
		rdmaGroup.GET("/devices", h.ListDevices)
		rdmaGroup.GET("/devices/:id", h.GetDevice)

		// 配置管理
		rdmaGroup.GET("/config", h.GetConfig)
		rdmaGroup.PUT("/config", h.UpdateConfig)

		// 存储目标管理
		rdmaGroup.POST("/targets", h.CreateTarget)
		rdmaGroup.GET("/targets", h.ListTargets)
		rdmaGroup.DELETE("/targets/:id", h.DeleteTarget)

		// 性能监控
		rdmaGroup.GET("/metrics", h.GetMetrics)

		// 基准测试
		rdmaGroup.POST("/benchmark", h.RunBenchmark)

		// 调优管理
		rdmaGroup.GET("/tuning", h.GetTuningProfiles)
		rdmaGroup.POST("/tuning/apply", h.ApplyTuningProfile)

		// 健康检查
		rdmaGroup.GET("/health", h.HealthCheck)
	}
}

// ListDevices 列出 RDMA 设备
func (h *Handler) ListDevices(c *gin.Context) {
	devices := h.manager.GetDevices()

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    devices,
	})
}

// GetDevice 获取设备详情
func (h *Handler) GetDevice(c *gin.Context) {
	id := c.Param("id")

	device, err := h.manager.GetDevice(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    device,
	})
}

// GetConfig 获取 RDMA 配置
func (h *Handler) GetConfig(c *gin.Context) {
	config := h.manager.GetConfig()

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    config,
	})
}

// UpdateConfig 更新 RDMA 配置
func (h *Handler) UpdateConfig(c *gin.Context) {
	var config RDMAConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "无效的请求参数: " + err.Error(),
		})
		return
	}

	if err := h.manager.UpdateConfig(&config); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "配置已更新",
		Data:    config,
	})
}

// CreateTarget 创建存储目标
func (h *Handler) CreateTarget(c *gin.Context) {
	var req StorageTarget
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "无效的请求参数: " + err.Error(),
		})
		return
	}

	target, err := h.manager.CreateTarget(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Code:    0,
		Message: "存储目标已创建",
		Data:    target,
	})
}

// ListTargets 列出存储目标
func (h *Handler) ListTargets(c *gin.Context) {
	targets := h.manager.GetTargets()

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    targets,
	})
}

// DeleteTarget 删除存储目标
func (h *Handler) DeleteTarget(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteTarget(id); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "存储目标已删除",
	})
}

// GetMetrics 获取性能指标
func (h *Handler) GetMetrics(c *gin.Context) {
	deviceID := c.Query("device_id")
	limitStr := c.Query("limit")

	if limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, APIResponse{
				Code:    400,
				Message: "无效的 limit 参数",
			})
			return
		}

		history := h.manager.GetMetricsHistory(limit)
		c.JSON(http.StatusOK, APIResponse{
			Code:    0,
			Message: "ok",
			Data:    history,
		})
		return
	}

	metrics := h.manager.GetMetrics(deviceID)
	if metrics == nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: "未找到性能指标",
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    metrics,
	})
}

// RunBenchmark 运行基准测试
func (h *Handler) RunBenchmark(c *gin.Context) {
	var config BenchmarkConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "无效的请求参数: " + err.Error(),
		})
		return
	}

	result, err := h.manager.RunBenchmark(&config)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "基准测试完成",
		Data:    result,
	})
}

// GetTuningProfiles 获取调优预设
func (h *Handler) GetTuningProfiles(c *gin.Context) {
	profiles := h.manager.GetTuningProfiles()

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    profiles,
	})
}

// ApplyTuningProfile 应用调优预设
func (h *Handler) ApplyTuningProfile(c *gin.Context) {
	var req struct {
		ProfileID string `json:"profile_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "无效的请求参数: " + err.Error(),
		})
		return
	}

	if err := h.manager.ApplyTuningProfile(req.ProfileID); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "调优预设已应用",
	})
}

// HealthCheck 执行健康检查
func (h *Handler) HealthCheck(c *gin.Context) {
	deviceID := c.Query("device_id")
	targetID := c.Query("target_id")

	if deviceID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "device_id 参数必填",
		})
		return
	}

	result := h.manager.HealthCheck(deviceID, targetID)

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    result,
	})
}