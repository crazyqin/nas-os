package fileversion

import (
	"net/http"
	"strconv"

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
	versions := r.Group("/fileversions")
	{
		// 版本管理
		versions.POST("", h.createVersion)
		versions.GET("", h.listAllVersions)
		versions.GET("/stats", h.getStats)

		// 文件版本操作
		versions.GET("/file/*path", h.listVersions)
		versions.GET("/:id", h.getVersion)
		versions.DELETE("/:id", h.deleteVersion)

		// 版本恢复
		versions.POST("/:id/restore", h.restoreVersion)

		// 版本对比
		versions.POST("/compare", h.compareVersions)
	}
}

// ========== 请求/响应结构 ==========

// CreateVersionRequest 创建版本请求
type CreateVersionRequest struct {
	FilePath    string `json:"file_path" binding:"required"`
	Description string `json:"description"`
}

// CompareVersionsRequest 对比版本请求
type CompareVersionsRequest struct {
	VersionID1 string `json:"version_id1" binding:"required"`
	VersionID2 string `json:"version_id2" binding:"required"`
}

// APIResponse 通用API响应
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ========== 处理函数 ==========

// createVersion 创建文件版本
func (h *Handlers) createVersion(c *gin.Context) {
	var req CreateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	version, err := h.manager.CreateVersion(c.Request.Context(), req.FilePath, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "创建版本失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Code:    201,
		Message: "版本创建成功",
		Data:    version,
	})
}

// listAllVersions 列出所有文件的版本
func (h *Handlers) listAllVersions(c *gin.Context) {
	versions := h.manager.ListAllVersions()

	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "success",
		Data:    versions,
	})
}

// listVersions 列出指定文件的版本历史
func (h *Handlers) listVersions(c *gin.Context) {
	filePath := c.Param("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "文件路径不能为空",
		})
		return
	}

	versions, err := h.manager.ListVersions(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "获取版本列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "success",
		Data: map[string]interface{}{
			"file_path": filePath,
			"versions":  versions,
			"total":     len(versions),
		},
	})
}

// getVersion 获取指定版本信息
func (h *Handlers) getVersion(c *gin.Context) {
	versionID := c.Param("id")
	if versionID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "版本ID不能为空",
		})
		return
	}

	version, err := h.manager.GetVersion(versionID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: "版本不存在: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "success",
		Data:    version,
	})
}

// deleteVersion 删除指定版本
func (h *Handlers) deleteVersion(c *gin.Context) {
	versionID := c.Param("id")
	if versionID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "版本ID不能为空",
		})
		return
	}

	if err := h.manager.DeleteVersion(versionID); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "删除版本失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "版本删除成功",
	})
}

// restoreVersion 恢复到指定版本
func (h *Handlers) restoreVersion(c *gin.Context) {
	versionID := c.Param("id")
	if versionID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "版本ID不能为空",
		})
		return
	}

	if err := h.manager.RestoreVersion(c.Request.Context(), versionID); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "恢复版本失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "版本恢复成功",
	})
}

// compareVersions 对比两个版本
func (h *Handlers) compareVersions(c *gin.Context) {
	var req CompareVersionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	diff, err := h.manager.CompareVersions(req.VersionID1, req.VersionID2)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "版本对比失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "success",
		Data:    diff,
	})
}

// getStats 获取版本统计信息
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()

	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "success",
		Data:    stats,
	})
}

// GetVersionsCount 获取版本总数（用于健康检查）
func (h *Handlers) GetVersionsCount() int {
	stats := h.manager.GetStats()
	return stats.TotalVersions
}

// GetStorageSize 获取存储大小（用于健康检查）
func (h *Handlers) GetStorageSize() int64 {
	stats := h.manager.GetStats()
	return stats.TotalSize
}

// GetVersionsCountParam 获取带参数的版本数量
func (h *Handlers) GetVersionsCountParam(filePath string) int {
	versions, err := h.manager.ListVersions(filePath)
	if err != nil {
		return 0
	}
	return len(versions)
}

// parsePagination 解析分页参数
func parsePagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	return page, pageSize
}
