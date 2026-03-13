package versioning

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 版本控制 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{
		manager: manager,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	versions := r.Group("/versions")
	{
		// 文件版本管理
		versions.GET("/files/*path", h.listFileVersions)
		versions.GET("/:id", h.getVersion)
		versions.POST("/:id/restore", h.restoreVersion)
		versions.DELETE("/:id", h.deleteVersion)
		versions.GET("/:id/diff", h.getVersionDiff)

		// 手动创建版本
		versions.POST("/files/*path", h.createVersion)

		// 统计信息
		versions.GET("/stats", h.getStats)

		// 配置管理
		versions.GET("/config", h.getConfig)
		versions.PUT("/config", h.updateConfig)
	}
}

// ========== API 处理函数 ==========

// listFileVersions 获取文件的所有版本
// @Summary 获取文件版本列表
// @Description 获取指定文件的所有历史版本
// @Tags versioning
// @Accept json
// @Produce json
// @Param path path string true "文件路径"
// @Success 200 {object} GenericResponse "成功"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /api/v1/versions/files/{path} [get]
func (h *Handlers) listFileVersions(c *gin.Context) {
	filePath := c.Param("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "文件路径不能为空",
		})
		return
	}

	versions, err := h.manager.GetVersions(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    versions,
	})
}

// getVersion 获取指定版本详情
// @Summary 获取版本详情
// @Description 获取指定版本的详细信息
// @Tags versioning
// @Accept json
// @Produce json
// @Param id path string true "版本 ID"
// @Success 200 {object} GenericResponse "成功"
// @Failure 404 {object} GenericResponse "版本不存在"
// @Router /api/v1/versions/{id} [get]
func (h *Handlers) getVersion(c *gin.Context) {
	versionID := c.Param("id")

	version, err := h.manager.GetVersion(versionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    version,
	})
}

// createVersion 创建新版本
// @Summary 创建文件版本
// @Description 为指定文件创建新版本快照
// @Tags versioning
// @Accept json
// @Produce json
// @Param path path string true "文件路径"
// @Param request body CreateVersionRequest true "版本创建参数"
// @Success 200 {object} GenericResponse "创建成功"
// @Failure 400 {object} GenericResponse "请求参数错误"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /api/v1/versions/files/{path} [post]
func (h *Handlers) createVersion(c *gin.Context) {
	filePath := c.Param("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "文件路径不能为空",
		})
		return
	}

	var req CreateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 使用默认值
		req.Description = "手动创建版本"
		req.TriggerType = "manual"
	}

	if req.TriggerType == "" {
		req.TriggerType = "manual"
	}

	version, err := h.manager.CreateVersion(filePath, req.UserID, req.Description, req.TriggerType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "版本创建成功",
		"data":    version,
	})
}

// restoreVersion 恢复到指定版本
// @Summary 恢复版本
// @Description 将文件恢复到指定版本
// @Tags versioning
// @Accept json
// @Produce json
// @Param id path string true "版本 ID"
// @Param request body RestoreVersionRequest true "恢复参数"
// @Success 200 {object} GenericResponse "恢复成功"
// @Failure 400 {object} GenericResponse "请求参数错误"
// @Failure 404 {object} GenericResponse "版本不存在"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /api/v1/versions/{id}/restore [post]
func (h *Handlers) restoreVersion(c *gin.Context) {
	versionID := c.Param("id")

	var req RestoreVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.TargetPath = "" // 恢复到原始路径
	}

	if err := h.manager.RestoreVersion(versionID, req.TargetPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "版本恢复成功",
	})
}

// deleteVersion 删除指定版本
// @Summary 删除版本
// @Description 删除指定的历史版本
// @Tags versioning
// @Accept json
// @Produce json
// @Param id path string true "版本 ID"
// @Success 200 {object} GenericResponse "删除成功"
// @Failure 404 {object} GenericResponse "版本不存在"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /api/v1/versions/{id} [delete]
func (h *Handlers) deleteVersion(c *gin.Context) {
	versionID := c.Param("id")

	if err := h.manager.DeleteVersion(versionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "版本已删除",
	})
}

// getVersionDiff 获取版本差异
// @Summary 获取版本差异
// @Description 获取指定版本与当前文件的差异
// @Tags versioning
// @Accept json
// @Produce json
// @Param id path string true "版本 ID"
// @Success 200 {object} GenericResponse "成功"
// @Failure 404 {object} GenericResponse "版本不存在"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /api/v1/versions/{id}/diff [get]
func (h *Handlers) getVersionDiff(c *gin.Context) {
	versionID := c.Param("id")

	diff, err := h.manager.GetDiff(versionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    diff,
	})
}

// getStats 获取统计信息
// @Summary 获取版本控制统计信息
// @Description 获取版本控制模块的统计信息
// @Tags versioning
// @Accept json
// @Produce json
// @Success 200 {object} GenericResponse "成功"
// @Router /api/v1/versions/stats [get]
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// getConfig 获取配置
// @Summary 获取版本控制配置
// @Description 获取版本控制模块的配置信息
// @Tags versioning
// @Accept json
// @Produce json
// @Success 200 {object} GenericResponse "成功"
// @Router /api/v1/versions/config [get]
func (h *Handlers) getConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    h.manager.config,
	})
}

// updateConfig 更新配置
// @Summary 更新版本控制配置
// @Description 更新版本控制模块的配置
// @Tags versioning
// @Accept json
// @Produce json
// @Param request body Config true "配置参数"
// @Success 200 {object} GenericResponse "更新成功"
// @Failure 400 {object} GenericResponse "请求参数错误"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /api/v1/versions/config [put]
func (h *Handlers) updateConfig(c *gin.Context) {
	var config Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.manager.UpdateConfig(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "配置更新成功",
	})
}

// ========== 请求/响应类型 ==========

// CreateVersionRequest 创建版本请求
type CreateVersionRequest struct {
	UserID      string `json:"userId"`
	Description string `json:"description"`
	TriggerType string `json:"triggerType"`
}

// RestoreVersionRequest 恢复版本请求
type RestoreVersionRequest struct {
	TargetPath string `json:"targetPath"` // 目标路径，为空则恢复到原始位置
}

// GenericResponse 通用响应
type GenericResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
