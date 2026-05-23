// Package diskhealth 提供 SMART 磁盘健康监测和故障预测功能
package diskhealth

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// Handlers 磁盘健康 API 处理器
type Handlers struct {
	monitor *DiskHealthMonitor
	mu      sync.RWMutex
}

// NewHandlers 创建磁盘健康处理器
func NewHandlers(monitor *DiskHealthMonitor) *Handlers {
	return &Handlers{
		monitor: monitor,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	api := r.Group("/api/v1/disk")
	{
		api.GET("/health", h.getAllDiskHealth)
		api.GET("/health/:device", h.getDiskHealth)
		api.GET("/health/:device/history", h.getDiskHistory)
		api.POST("/health/scan", h.triggerScan)
	}
}

// getAllDiskHealth 获取所有磁盘健康状态
// @Summary 获取所有磁盘健康状态
// @Description 返回系统中所有磁盘的 SMART 健康状态和评分
// @Tags disk-health
// @Produce json
// @Success 200 {object} DiskHealthResponse
// @Router /api/v1/disk/health [get]
func (h *Handlers) getAllDiskHealth(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	disks := h.monitor.GetAllDiskHealth()

	// 按健康评分排序（最差的排前面）
	sortDiskHealthByScore(disks)

	c.JSON(http.StatusOK, DiskHealthResponse{
		Code:    0,
		Message: "success",
		Data:    disks,
	})
}

// getDiskHealth 获取单个磁盘健康状态
// @Summary 获取单个磁盘健康详情
// @Description 返回指定磁盘的详细 SMART 健康状态
// @Tags disk-health
// @Produce json
// @Param device path string true "设备名称"
// @Success 200 {object} DiskHealthResponse
// @Failure 404 {object} DiskHealthResponse
// @Router /api/v1/disk/health/{device} [get]
func (h *Handlers) getDiskHealth(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, DiskHealthResponse{
			Code:    1,
			Message: "设备名称不能为空",
		})
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	status, exists := h.monitor.GetDiskHealth(device)
	if !exists {
		c.JSON(http.StatusNotFound, DiskHealthResponse{
			Code:    2,
			Message: "未找到设备: " + device,
		})
		return
	}

	c.JSON(http.StatusOK, DiskHealthResponse{
		Code:    0,
		Message: "success",
		Data:    status,
	})
}

// getDiskHistory 获取磁盘健康历史趋势
// @Summary 获取磁盘健康历史
// @Description 返回指定磁盘的健康评分历史趋势
// @Tags disk-health
// @Produce json
// @Param device path string true "设备名称"
// @Success 200 {object} DiskHealthResponse
// @Failure 404 {object} DiskHealthResponse
// @Router /api/v1/disk/health/{device}/history [get]
func (h *Handlers) getDiskHistory(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, DiskHealthResponse{
			Code:    1,
			Message: "设备名称不能为空",
		})
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	history, exists := h.monitor.GetDiskHistory(device)
	if !exists {
		c.JSON(http.StatusNotFound, DiskHealthResponse{
			Code:    2,
			Message: "未找到设备历史记录: " + device,
		})
		return
	}

	c.JSON(http.StatusOK, DiskHealthResponse{
		Code:    0,
		Message: "success",
		Data:    history,
	})
}

// triggerScan 触发手动扫描
// @Summary 触发磁盘健康扫描
// @Description 触发指定或所有磁盘的 SMART 扫描
// @Tags disk-health
// @Accept json
// @Produce json
// @Param request body ScanRequest true "扫描请求"
// @Success 200 {object} DiskHealthResponse
// @Failure 400 {object} DiskHealthResponse
// @Router /api/v1/disk/health/scan [post]
func (h *Handlers) triggerScan(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 如果解析失败，使用默认配置扫描所有磁盘
		req = ScanRequest{
			Force: true,
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	response, err := h.monitor.TriggerScan(req.Devices, req.Force)
	if err != nil {
		c.JSON(http.StatusInternalServerError, DiskHealthResponse{
			Code:    3,
			Message: "扫描失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, DiskHealthResponse{
		Code:    0,
		Message: response.Message,
		Data:    response,
	})
}

// sortDiskHealthByScore 按健康评分排序磁盘（最差的排前面）
func sortDiskHealthByScore(disks []*DiskHealthStatus) {
	// 简单的插入排序
	for i := 1; i < len(disks); i++ {
		key := disks[i]
		j := i - 1
		for j >= 0 && disks[j].HealthScore > key.HealthScore {
			disks[j+1] = disks[j]
			j--
		}
		disks[j+1] = key
	}
}
