// Package gpuds 提供 GPU Direct Storage 功能
package gpuds

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers GPU Direct Storage API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	v1 := r.Group("/v1")
	{
		// GPU 设备管理
		v1.GET("/gpus", h.listGPUs)
		v1.GET("/gpus/:id", h.getGPU)
		v1.POST("/gpus/register", h.registerGPU)

		// 缓冲区管理
		v1.POST("/buffers", h.createBuffer)
		v1.GET("/buffers", h.listBuffers)
		v1.GET("/buffers/:id", h.getBuffer)
		v1.DELETE("/buffers/:id", h.freeBuffer)

		// 传输管理
		v1.POST("/transfers", h.startTransfer)
		v1.GET("/transfers", h.listTransfers)
		v1.GET("/transfers/:id", h.getTransfer)
		v1.POST("/transfers/:id/complete", h.completeTransfer)
		v1.POST("/transfers/:id/fail", h.failTransfer)
		v1.POST("/transfers/:id/cancel", h.cancelTransfer)

		// 统计信息
		v1.GET("/stats/transfers", h.getTransferStats)
		v1.GET("/stats/bandwidth/:device_id", h.getBandwidthStats)

		// 配置管理
		v1.GET("/config", h.getConfig)
		v1.PUT("/config", h.updateConfig)
	}
}

// listGPUs 列出 GPU 设备
func (h *Handlers) listGPUs(c *gin.Context) {
	devices, err := h.manager.DetectGPU()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": devices})
}

// getGPU 获取 GPU 设备详情
func (h *Handlers) getGPU(c *gin.Context) {
	id := c.Param("id")

	device, err := h.manager.GetGPU(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": device})
}

// registerGPU 注册 GPU 设备
func (h *Handlers) registerGPU(c *gin.Context) {
	var device GPUDevice

	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.RegisterGPU(device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "GPU registered"})
}

// createBuffer 创建缓冲区
func (h *Handlers) createBuffer(c *gin.Context) {
	var req struct {
		GPUDeviceID string `json:"gpu_device_id" binding:"required"`
		Size        int64  `json:"size" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	buffer, err := h.manager.CreateBuffer(req.GPUDeviceID, req.Size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": buffer})
}

// listBuffers 列出缓冲区
func (h *Handlers) listBuffers(c *gin.Context) {
	gpuDeviceID := c.Query("gpu_device_id")

	buffers := h.manager.ListBuffers(gpuDeviceID)

	c.JSON(http.StatusOK, gin.H{"data": buffers})
}

// getBuffer 获取缓冲区详情
func (h *Handlers) getBuffer(c *gin.Context) {
	id := c.Param("id")

	buffer, err := h.manager.GetBuffer(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": buffer})
}

// freeBuffer 释放缓冲区
func (h *Handlers) freeBuffer(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.FreeBuffer(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "buffer freed"})
}

// startTransfer 发起传输
func (h *Handlers) startTransfer(c *gin.Context) {
	var req struct {
		Source      TransferEndpoint `json:"source" binding:"required"`
		Destination TransferEndpoint `json:"destination" binding:"required"`
		Size        int64            `json:"size" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	job, err := h.manager.Transfer(req.Source, req.Destination, req.Size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": job})
}

// listTransfers 列出传输任务
func (h *Handlers) listTransfers(c *gin.Context) {
	state := TransferState(c.Query("state"))

	jobs := h.manager.ListTransferJobs(state)

	c.JSON(http.StatusOK, gin.H{"data": jobs})
}

// getTransfer 获取传输任务详情
func (h *Handlers) getTransfer(c *gin.Context) {
	id := c.Param("id")

	job, err := h.manager.GetTransferJob(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": job})
}

// completeTransfer 完成传输
func (h *Handlers) completeTransfer(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Transferred int64   `json:"transferred" binding:"required"`
		Bandwidth   float64 `json:"bandwidth"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.CompleteTransfer(id, req.Transferred, req.Bandwidth); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "transfer completed"})
}

// failTransfer 标记传输失败
func (h *Handlers) failTransfer(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Error string `json:"error" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.FailTransfer(id, req.Error); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "transfer marked as failed"})
}

// cancelTransfer 取消传输
func (h *Handlers) cancelTransfer(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.CancelTransfer(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "transfer cancelled"})
}

// getTransferStats 获取传输统计
func (h *Handlers) getTransferStats(c *gin.Context) {
	stats := h.manager.GetTransferStats()

	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// getBandwidthStats 获取带宽统计
func (h *Handlers) getBandwidthStats(c *gin.Context) {
	deviceID := c.Param("device_id")

	stats, err := h.manager.GetBandwidthStats(deviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// getConfig 获取配置
func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()

	c.JSON(http.StatusOK, gin.H{"data": config})
}

// updateConfig 更新配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var config GPUDSConfig

	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.manager.UpdateConfig(config)

	c.JSON(http.StatusOK, gin.H{"message": "config updated"})
}
