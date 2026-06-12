// Package nfsoverrdma 提供NFS over RDMA的HTTP处理器
package nfsoverrdma

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers NFS over RDMA HTTP处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	nfsGroup := api.Group("/nfs-rdma")
	{
		// RDMA配置
		nfsGroup.GET("/rdma/config", h.getRDMAConfig)
		nfsGroup.PUT("/rdma/config", h.configureRDMA)
		nfsGroup.GET("/rdma/stats", h.getRDMAStats)
		nfsGroup.POST("/rdma/test", h.testRDMAConnection)

		// NFS导出管理
		nfsGroup.POST("/exports", h.createExport)
		nfsGroup.GET("/exports", h.listExports)
		nfsGroup.GET("/exports/:id", h.getExport)
		nfsGroup.PUT("/exports/:id", h.updateExport)
		nfsGroup.DELETE("/exports/:id", h.deleteExport)
		nfsGroup.GET("/exports/:id/stats", h.getExportStats)
		nfsGroup.POST("/exports/:id/rdma/enable", h.enableRDMA)
		nfsGroup.POST("/exports/:id/rdma/disable", h.disableRDMA)

		// 客户端管理
		nfsGroup.POST("/clients", h.addClient)
		nfsGroup.GET("/clients", h.listClients)
		nfsGroup.GET("/clients/:id", h.getClient)
		nfsGroup.DELETE("/clients/:id", h.removeClient)

		// 性能监控
		nfsGroup.GET("/performance/:exportId", h.getPerformanceMetrics)
		nfsGroup.GET("/stats", h.getServerStats)
	}
}

// getRDMAConfig 获取RDMA配置
func (h *Handlers) getRDMAConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "RDMA配置查询"})
}

// configureRDMA 配置RDMA
func (h *Handlers) configureRDMA(c *gin.Context) {
	var config RDMAConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.ConfigureRDMA(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "RDMA配置成功"})
}

// getRDMAStats 获取RDMA统计
func (h *Handlers) getRDMAStats(c *gin.Context) {
	stats := h.manager.GetRDMAStats()
	c.JSON(http.StatusOK, stats)
}

// testRDMAConnection 测试RDMA连接
func (h *Handlers) testRDMAConnection(c *gin.Context) {
	var req struct {
		TargetIP string `json:"target_ip" binding:"required"`
		Port     int    `json:"port"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if req.Port == 0 {
		req.Port = 2049
	}

	result, err := h.manager.SimulateRDMAConnection(req.TargetIP, req.Port)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// createExport 创建NFS导出
func (h *Handlers) createExport(c *gin.Context) {
	var export NFSExport
	if err := c.ShouldBindJSON(&export); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.CreateExport(&export); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "NFS导出创建成功",
		"export":  export,
	})
}

// listExports 列出导出
func (h *Handlers) listExports(c *gin.Context) {
	exports := h.manager.ListExports()
	c.JSON(http.StatusOK, gin.H{
		"exports": exports,
		"total":   len(exports),
	})
}

// getExport 获取导出
func (h *Handlers) getExport(c *gin.Context) {
	id := c.Param("id")
	export, err := h.manager.GetExport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, export)
}

// updateExport 更新导出
func (h *Handlers) updateExport(c *gin.Context) {
	id := c.Param("id")
	var export NFSExport
	if err := c.ShouldBindJSON(&export); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	export.ID = id
	if err := h.manager.UpdateExport(&export); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "NFS导出更新成功"})
}

// deleteExport 删除导出
func (h *Handlers) deleteExport(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteExport(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "NFS导出删除成功"})
}

// getExportStats 获取导出统计
func (h *Handlers) getExportStats(c *gin.Context) {
	id := c.Param("id")
	stats := h.manager.GetExportStats(id)
	if stats == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "导出不存在"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// enableRDMA 启用RDMA
func (h *Handlers) enableRDMA(c *gin.Context) {
	id := c.Param("id")
	var config RDMAConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.EnableRDMAOnExport(id, &config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "RDMA启用成功"})
}

// disableRDMA 禁用RDMA
func (h *Handlers) disableRDMA(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DisableRDMAOnExport(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "RDMA禁用成功"})
}

// addClient 添加客户端
func (h *Handlers) addClient(c *gin.Context) {
	var client NFSClient
	if err := c.ShouldBindJSON(&client); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.AddClient(&client); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "客户端添加成功",
		"client":  client,
	})
}

// listClients 列出客户端
func (h *Handlers) listClients(c *gin.Context) {
	exportID := c.Query("export_id")
	clients := h.manager.ListClients(exportID)
	c.JSON(http.StatusOK, gin.H{
		"clients": clients,
		"total":   len(clients),
	})
}

// getClient 获取客户端
func (h *Handlers) getClient(c *gin.Context) {
	id := c.Param("id")
	client, err := h.manager.GetClient(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, client)
}

// removeClient 移除客户端
func (h *Handlers) removeClient(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RemoveClient(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "客户端移除成功"})
}

// getPerformanceMetrics 获取性能指标
func (h *Handlers) getPerformanceMetrics(c *gin.Context) {
	exportID := c.Param("exportId")
	metrics := h.manager.GetPerformanceMetrics(exportID)
	if metrics == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "导出不存在"})
		return
	}
	c.JSON(http.StatusOK, metrics)
}

// getServerStats 获取服务器统计
func (h *Handlers) getServerStats(c *gin.Context) {
	stats := h.manager.GetServerStats()
	c.JSON(http.StatusOK, stats)
}
