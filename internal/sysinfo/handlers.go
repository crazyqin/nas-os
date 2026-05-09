package sysinfo

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 系统信息 API 处理器。
type Handlers struct {
	collector *Collector
	logger    *zap.Logger
}

// NewHandlers 创建系统信息处理器。
func NewHandlers(collector *Collector, logger *zap.Logger) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handlers{
		collector: collector,
		logger:    logger,
	}
}

// RegisterRoutes 注册路由到指定的路由组。
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	sysinfo := r.Group("/sysinfo")
	{
		sysinfo.GET("", h.getSysInfo)
		sysinfo.GET("/cpu", h.getCPU)
		sysinfo.GET("/memory", h.getMemory)
		sysinfo.GET("/disks", h.getDisks)
		sysinfo.GET("/network", h.getNetwork)
	}
}

// Response API 响应。
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// getSysInfo 获取完整系统信息。
// @Summary 获取完整系统信息
// @Description 返回 CPU、内存、磁盘、网络、负载等完整系统概览
// @Tags sysinfo
// @Produce json
// @Success 200 {object} Response{data=SystemInfo}
// @Router /api/v1/sysinfo [get]
func (h *Handlers) getSysInfo(c *gin.Context) {
	info := h.collector.Collect()
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    info,
	})
}

// getCPU 获取 CPU 信息。
// @Summary 获取 CPU 信息
// @Description 返回 CPU 型号、核心数、使用率、温度、频率
// @Tags sysinfo
// @Produce json
// @Success 200 {object} Response{data=CPUInfo}
// @Router /api/v1/sysinfo/cpu [get]
func (h *Handlers) getCPU(c *gin.Context) {
	info := h.collector.CollectCPU()
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    info,
	})
}

// getMemory 获取内存信息。
// @Summary 获取内存信息
// @Description 返回内存总量、已用、可用、缓存、交换分区
// @Tags sysinfo
// @Produce json
// @Success 200 {object} Response{data=MemInfo}
// @Router /api/v1/sysinfo/memory [get]
func (h *Handlers) getMemory(c *gin.Context) {
	info := h.collector.CollectMemory()
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    info,
	})
}

// getDisks 获取磁盘信息。
// @Summary 获取磁盘信息
// @Description 返回所有挂载磁盘的容量、使用率、健康状态
// @Tags sysinfo
// @Produce json
// @Success 200 {object} Response{data=[]DiskInfo}
// @Router /api/v1/sysinfo/disks [get]
func (h *Handlers) getDisks(c *gin.Context) {
	info := h.collector.CollectDisks()
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    info,
	})
}

// getNetwork 获取网络接口信息。
// @Summary 获取网络接口信息
// @Description 返回所有网络接口的 IP、MAC、流量、速度
// @Tags sysinfo
// @Produce json
// @Success 200 {object} Response{data=[]NetInfo}
// @Router /api/v1/sysinfo/network [get]
func (h *Handlers) getNetwork(c *gin.Context) {
	info := h.collector.CollectNetwork()
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    info,
	})
}
