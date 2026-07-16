// Package vmimport 提供虚拟机镜像导入导出功能
package vmimport

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers HTTP API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建HTTP API处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	vmimport := rg.Group("/vmimport")
	{
		// 导入相关.
		vmimport.POST("/import", h.handleImport)
		vmimport.GET("/import/:id/status", h.handleImportStatus)
		vmimport.POST("/import/:id/cancel", h.handleCancelImport)

		// 格式相关.
		vmimport.GET("/formats", h.handleGetFormats)
		vmimport.POST("/validate", h.handleValidate)
		vmimport.POST("/convert", h.handleConvert)

		// 镜像管理.
		vmimport.GET("/images", h.handleListImages)
		vmimport.GET("/images/:id", h.handleGetImage)
		vmimport.DELETE("/images/:id", h.handleDeleteImage)

		// 导出相关.
		vmimport.POST("/export", h.handleExport)
		vmimport.GET("/export/:id/status", h.handleExportStatus)

		// 存储信息.
		vmimport.GET("/storage", h.handleStorageUsage)
	}
}

// handleImport 处理导入请求.
func (h *Handlers) handleImport(c *gin.Context) {
	var req ImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	task, err := h.manager.StartImport(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "导入任务已创建",
		"task":    task,
	})
}

// handleImportStatus 处理导入状态查询.
func (h *Handlers) handleImportStatus(c *gin.Context) {
	id := c.Param("id")

	task, err := h.manager.GetImportStatus(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// handleCancelImport 处理取消导入.
func (h *Handlers) handleCancelImport(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.CancelImport(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "导入任务已取消"})
}

// handleGetFormats 处理获取支持格式列表.
func (h *Handlers) handleGetFormats(c *gin.Context) {
	formats := h.manager.GetSupportedFormats()
	c.JSON(http.StatusOK, gin.H{"formats": formats})
}

// handleValidate 处理镜像验证请求.
func (h *Handlers) handleValidate(c *gin.Context) {
	var req ValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	result, err := ValidateImage(req.FilePath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// handleConvert 处理格式转换请求.
func (h *Handlers) handleConvert(c *gin.Context) {
	var req ConvertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	// 获取源镜像.
	img, err := h.manager.GetImage(req.ImageID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 检查目标格式.
	if !isSupportedFormat(req.TargetFormat) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的目标格式"})
		return
	}

	// 创建导入任务来执行转换.
	importReq := ImportRequest{
		Source:       img.FilePath,
		SourceType:   "file",
		TargetName:   req.OutputName,
		TargetFormat: req.TargetFormat,
	}

	task, err := h.manager.StartImport(importReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建转换任务失败: %v", err)})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "格式转换任务已创建",
		"task":    task,
	})
}

// handleListImages 处理镜像列表请求.
func (h *Handlers) handleListImages(c *gin.Context) {
	images := h.manager.ListImages()
	c.JSON(http.StatusOK, gin.H{"images": images})
}

// handleGetImage 处理获取镜像详情.
func (h *Handlers) handleGetImage(c *gin.Context) {
	id := c.Param("id")

	img, err := h.manager.GetImage(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, img)
}

// handleDeleteImage 处理删除镜像.
func (h *Handlers) handleDeleteImage(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteImage(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "镜像已删除"})
}

// handleExport 处理导出请求.
func (h *Handlers) handleExport(c *gin.Context) {
	var req ExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	task, err := h.manager.StartExport(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "导出任务已创建",
		"task":    task,
	})
}

// handleExportStatus 处理导出状态查询.
func (h *Handlers) handleExportStatus(c *gin.Context) {
	id := c.Param("id")

	task, err := h.manager.GetExportStatus(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// handleStorageUsage 处理存储空间查询.
func (h *Handlers) handleStorageUsage(c *gin.Context) {
	usage, err := h.manager.GetStorageUsage()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, usage)
}
