package resmonpro

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 资源监控处理器.
type Handlers struct {
	collector *Collector
}

// NewHandlers 创建处理器.
func NewHandlers() *Handlers {
	return &Handlers{
		collector: NewCollector(),
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	resmonpro := r.Group("/resmonpro")
	{
		resmonpro.GET("/processes", h.getProcesses)
		resmonpro.GET("/gpu", h.getGPU)
		resmonpro.GET("/network", h.getNetwork)
		resmonpro.GET("/diskio", h.getDiskIO)
		resmonpro.GET("/bottleneck", h.getBottleneck)
	}
}

// getProcesses 获取进程列表
// @Summary 获取进程资源使用情况
// @Description 获取所有进程的 CPU、内存、IO、网络等资源使用信息
// @Tags resmonpro
// @Accept json
// @Produce json
// @Success 200 {object} ResmonProResponse
// @Failure 500 {object} ResmonProResponse
// @Router /resmonpro/processes [get].
func (h *Handlers) getProcesses(c *gin.Context) {
	processes, err := h.collector.CollectProcesses()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ResmonProResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ResmonProResponse{
		Code:    0,
		Message: "success",
		Data:    processes,
	})
}

// getGPU 获取 GPU 状态
// @Summary 获取 GPU 信息
// @Description 获取 NVIDIA/AMD GPU 的温度、利用率、显存等信息
// @Tags resmonpro
// @Accept json
// @Produce json
// @Success 200 {object} ResmonProResponse
// @Failure 500 {object} ResmonProResponse
// @Router /resmonpro/gpu [get].
func (h *Handlers) getGPU(c *gin.Context) {
	gpus, err := h.collector.CollectGPU()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ResmonProResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ResmonProResponse{
		Code:    0,
		Message: "success",
		Data:    gpus,
	})
}

// getNetwork 获取网络流量
// @Summary 获取网络流量信息
// @Description 获取各网络接口的流量统计信息
// @Tags resmonpro
// @Accept json
// @Produce json
// @Success 200 {object} ResmonProResponse
// @Failure 500 {object} ResmonProResponse
// @Router /resmonpro/network [get].
func (h *Handlers) getNetwork(c *gin.Context) {
	flows, err := h.collector.CollectNetworkFlow()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ResmonProResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ResmonProResponse{
		Code:    0,
		Message: "success",
		Data:    flows,
	})
}

// getDiskIO 获取磁盘 I/O
// @Summary 获取磁盘 I/O 信息
// @Description 获取磁盘的 IOPS、延迟、吞吐量等性能指标
// @Tags resmonpro
// @Accept json
// @Produce json
// @Success 200 {object} ResmonProResponse
// @Failure 500 {object} ResmonProResponse
// @Router /resmonpro/diskio [get].
func (h *Handlers) getDiskIO(c *gin.Context) {
	disks, err := h.collector.CollectDiskIO()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ResmonProResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ResmonProResponse{
		Code:    0,
		Message: "success",
		Data:    disks,
	})
}

// getBottleneck 获取瓶颈诊断
// @Summary 获取系统瓶颈诊断
// @Description 自动诊断系统资源瓶颈并提供优化建议
// @Tags resmonpro
// @Accept json
// @Produce json
// @Success 200 {object} ResmonProResponse
// @Failure 500 {object} ResmonProResponse
// @Router /resmonpro/bottleneck [get].
func (h *Handlers) getBottleneck(c *gin.Context) {
	diagnoses, err := h.collector.DiagnoseBottlenecks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ResmonProResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ResmonProResponse{
		Code:    0,
		Message: "success",
		Data:    diagnoses,
	})
}
