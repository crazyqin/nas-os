package shares

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RecycleHandlers 回收站 API 处理器。
type RecycleHandlers struct {
	smbManager SMBManager
	recycleBins map[string]*RecycleBin
}

// NewRecycleHandlers 创建回收站处理器。
func NewRecycleHandlers(smbMgr SMBManager) *RecycleHandlers {
	return &RecycleHandlers{
		smbManager:  smbMgr,
		recycleBins: make(map[string]*RecycleBin),
	}
}

// getOrCreateRecycleBin 获取或创建指定共享的回收站实例。
func (h *RecycleHandlers) getOrCreateRecycleBin(shareName string) (*RecycleBin, error) {
	if rb, ok := h.recycleBins[shareName]; ok {
		return rb, nil
	}

	sharePath := h.smbManager.GetSharePath(shareName)
	if sharePath == "" {
		return nil, &shareNotFoundError{shareName: shareName}
	}

	rb := NewRecycleBin(shareName, sharePath, RecycleBinConfig{
		Enabled:       true,
		RetentionDays: 30,
	})
	h.recycleBins[shareName] = rb

	return rb, nil
}

// shareNotFoundError 共享未找到错误。
type shareNotFoundError struct {
	shareName string
}

func (e *shareNotFoundError) Error() string {
	return "共享不存在: " + e.shareName
}

// RegisterRoutes 注册回收站相关路由。
// 应在 shares 路由组下注册。
func (h *RecycleHandlers) RegisterRoutes(api *gin.RouterGroup) {
	recycleGroup := api.Group("/shares/:share/recycle")
	{
		recycleGroup.GET("", h.listRecycle)
		recycleGroup.GET("/stats", h.getRecycleStats)
		recycleGroup.PUT("/config", h.updateRecycleConfig)
		recycleGroup.POST("/empty", h.emptyRecycle)
		recycleGroup.POST("/:id/restore", h.restoreEntry)
		recycleGroup.DELETE("/:id", h.purgeEntry)
	}
}

// listRecycle 获取回收站内容
// @Summary 列出回收站内容
// @Description 获取指定共享回收站中的所有文件列表
// @Tags shares/recycle
// @Accept json
// @Produce json
// @Param share path string true "共享名称"
// @Success 200 {object} Response{data=[]RecycleEntry} "成功"
// @Failure 404 {object} Response "共享不存在"
// @Failure 500 {object} Response "服务器内部错误"
// @Router /shares/{share}/recycle [get]
// @Security BearerAuth.
func (h *RecycleHandlers) listRecycle(c *gin.Context) {
	shareName := c.Param("share")

	rb, err := h.getOrCreateRecycleBin(shareName)
	if err != nil {
		if _, ok := err.(*shareNotFoundError); ok {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	entries, err := rb.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(entries))
}

// getRecycleStats 获取回收站统计信息
// @Summary 获取回收站统计
// @Description 获取指定共享回收站的统计信息（条目数、大小等）
// @Tags shares/recycle
// @Accept json
// @Produce json
// @Param share path string true "共享名称"
// @Success 200 {object} Response{data=RecycleStats} "成功"
// @Failure 404 {object} Response "共享不存在"
// @Failure 500 {object} Response "服务器内部错误"
// @Router /shares/{share}/recycle/stats [get]
// @Security BearerAuth.
func (h *RecycleHandlers) getRecycleStats(c *gin.Context) {
	shareName := c.Param("share")

	rb, err := h.getOrCreateRecycleBin(shareName)
	if err != nil {
		if _, ok := err.(*shareNotFoundError); ok {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	stats, err := rb.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(stats))
}

// restoreEntry 恢复回收站中的文件
// @Summary 恢复文件
// @Description 从回收站恢复指定文件到原始位置
// @Tags shares/recycle
// @Accept json
// @Produce json
// @Param share path string true "共享名称"
// @Param id path string true "条目 ID"
// @Success 200 {object} Response "恢复成功"
// @Failure 404 {object} Response "条目不存在"
// @Failure 409 {object} Response "目标位置已存在同名文件"
// @Failure 500 {object} Response "服务器内部错误"
// @Router /shares/{share}/recycle/{id}/restore [post]
// @Security BearerAuth.
func (h *RecycleHandlers) restoreEntry(c *gin.Context) {
	shareName := c.Param("share")
	entryID := c.Param("id")

	rb, err := h.getOrCreateRecycleBin(shareName)
	if err != nil {
		if _, ok := err.(*shareNotFoundError); ok {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	if err := rb.Restore(entryID); err != nil {
		// 判断是否为冲突错误（目标位置已存在）
		if strings.Contains(err.Error(), "目标位置已存在同名文件") {
			c.JSON(http.StatusConflict, Error(409, err.Error()))
			return
		}
		if strings.Contains(err.Error(), "条目不存在") {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(gin.H{"message": "文件恢复成功"}))
}

// purgeEntry 永久删除回收站中的条目
// @Summary 永久删除
// @Description 永久删除回收站中的指定条目
// @Tags shares/recycle
// @Accept json
// @Produce json
// @Param share path string true "共享名称"
// @Param id path string true "条目 ID"
// @Success 200 {object} Response "删除成功"
// @Failure 404 {object} Response "条目不存在"
// @Failure 500 {object} Response "服务器内部错误"
// @Router /shares/{share}/recycle/{id} [delete]
// @Security BearerAuth.
func (h *RecycleHandlers) purgeEntry(c *gin.Context) {
	shareName := c.Param("share")
	entryID := c.Param("id")

	rb, err := h.getOrCreateRecycleBin(shareName)
	if err != nil {
		if _, ok := err.(*shareNotFoundError); ok {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	if err := rb.Purge(entryID); err != nil {
		if strings.Contains(err.Error(), "条目不存在") {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(gin.H{"message": "永久删除成功"}))
}

// emptyRecycle 清空回收站
// @Summary 清空回收站
// @Description 清空指定共享的回收站，永久删除所有文件
// @Tags shares/recycle
// @Accept json
// @Produce json
// @Param share path string true "共享名称"
// @Success 200 {object} Response "清空成功"
// @Failure 404 {object} Response "共享不存在"
// @Failure 500 {object} Response "服务器内部错误"
// @Router /shares/{share}/recycle/empty [post]
// @Security BearerAuth.
func (h *RecycleHandlers) emptyRecycle(c *gin.Context) {
	shareName := c.Param("share")

	rb, err := h.getOrCreateRecycleBin(shareName)
	if err != nil {
		if _, ok := err.(*shareNotFoundError); ok {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	if err := rb.PurgeAll(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(gin.H{"message": "回收站已清空"}))
}

// updateRecycleConfig 更新回收站配置
// @Summary 更新回收站配置
// @Description 更新指定共享的回收站配置（启用状态、保留天数、容量限制）
// @Tags shares/recycle
// @Accept json
// @Produce json
// @Param share path string true "共享名称"
// @Param request body RecycleBinConfig true "回收站配置"
// @Success 200 {object} Response{data=RecycleBinConfig} "更新成功"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 404 {object} Response "共享不存在"
// @Failure 500 {object} Response "服务器内部错误"
// @Router /shares/{share}/recycle/config [put]
// @Security BearerAuth.
func (h *RecycleHandlers) updateRecycleConfig(c *gin.Context) {
	shareName := c.Param("share")

	var config RecycleBinConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, "请求参数错误: "+err.Error()))
		return
	}

	// 参数校验
	if config.RetentionDays < 0 {
		c.JSON(http.StatusBadRequest, Error(400, "保留天数不能为负数"))
		return
	}
	if config.MaxSizeGB < 0 {
		c.JSON(http.StatusBadRequest, Error(400, "容量限制不能为负数"))
		return
	}

	rb, err := h.getOrCreateRecycleBin(shareName)
	if err != nil {
		if _, ok := err.(*shareNotFoundError); ok {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	rb.UpdateConfig(config)

	c.JSON(http.StatusOK, Success(rb.GetConfig()))
}

