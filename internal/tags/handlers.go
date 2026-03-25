// Package tags 标签管理模块
package tags

import (
	"net/http"
	"strings"

	apiresponse "nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers 标签管理 HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{manager: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	// ========== 标签管理 ==========
	tags := api.Group("/tags")
	{
		tags.GET("", h.listTags)
		tags.POST("", h.createTag)
		tags.GET("/search", h.searchTags)
		tags.GET("/groups", h.listGroups)
		tags.GET("/stats", h.getStats)
		tags.GET("/:id", h.getTag)
		tags.PUT("/:id", h.updateTag)
		tags.DELETE("/:id", h.deleteTag)
		tags.GET("/:id/files", h.getTagFiles)
		tags.GET("/:id/usage", h.getTagUsage)
	}

	// ========== 文件标签 ==========
	files := api.Group("/files")
	{
		// 单个文件的标签操作
		files.GET("/:id/tags", h.getFileTags)
		files.POST("/:id/tags", h.addFileTags)
		files.DELETE("/:id/tags/:tagId", h.removeFileTag)

		// 批量和搜索操作
		files.GET("/by-tag/:tagId", h.getFilesByTagID)
		files.POST("/batch-tags", h.batchTagFiles)
	}
}

// ========== 标签 API ==========

// listTags 列出所有标签
// @Summary 列出所有标签
// @Description 获取所有标签列表，支持按分组筛选
// @Tags tags
// @Accept json
// @Produce json
// @Param group query string false "分组名称"
// @Success 200 {object} Response "成功"
// @Router /tags [get].
func (h *Handlers) listTags(c *gin.Context) {
	group := c.Query("group")

	var tags []*Tag
	var err error

	if group != "" {
		tags, err = h.manager.ListTagsByGroup(group)
	} else {
		tags, err = h.manager.ListTags()
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "获取标签列表失败"))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(tags))
}

// getTag 获取单个标签
// @Summary 获取标签详情
// @Description 根据ID获取标签详细信息
// @Tags tags
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Success 200 {object} Response "成功"
// @Failure 404 {object} Response "标签不存在"
// @Router /tags/{id} [get].
func (h *Handlers) getTag(c *gin.Context) {
	id := c.Param("id")

	tag, err := h.manager.GetTag(id)
	if err != nil {
		if err == ErrTagNotFound {
			c.JSON(http.StatusNotFound, apiresponse.Error(404, "标签不存在"))
			return
		}
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "获取标签失败"))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(tag))
}

// createTag 创建标签
// @Summary 创建标签
// @Description 创建新的标签
// @Tags tags
// @Accept json
// @Produce json
// @Param request body TagInput true "标签参数"
// @Success 201 {object} Response "创建成功"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 409 {object} Response "标签名称已存在"
// @Router /tags [post].
func (h *Handlers) createTag(c *gin.Context) {
	var req TagInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, "无效的请求参数"))
		return
	}

	// 验证名称
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, "标签名称不能为空"))
		return
	}

	tag, err := h.manager.CreateTag(req)
	if err != nil {
		if err == ErrTagExists {
			c.JSON(http.StatusConflict, apiresponse.Error(409, "标签名称已存在"))
			return
		}
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "创建标签失败"))
		return
	}

	c.JSON(http.StatusCreated, apiresponse.Success(tag))
}

// updateTag 更新标签
// @Summary 更新标签
// @Description 更新标签信息
// @Tags tags
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Param request body TagInput true "标签参数"
// @Success 200 {object} Response "更新成功"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 404 {object} Response "标签不存在"
// @Failure 409 {object} Response "标签名称已存在"
// @Router /tags/{id} [put].
func (h *Handlers) updateTag(c *gin.Context) {
	id := c.Param("id")

	var req TagInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, "无效的请求参数"))
		return
	}

	// 清理名称
	req.Name = strings.TrimSpace(req.Name)

	tag, err := h.manager.UpdateTag(id, req)
	if err != nil {
		switch err {
		case ErrTagNotFound:
			c.JSON(http.StatusNotFound, apiresponse.Error(404, "标签不存在"))
		case ErrTagExists:
			c.JSON(http.StatusConflict, apiresponse.Error(409, "标签名称已存在"))
		default:
			c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "更新标签失败"))
		}
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(tag))
}

// deleteTag 删除标签
// @Summary 删除标签
// @Description 删除指定标签及其所有文件关联
// @Tags tags
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Success 200 {object} Response "删除成功"
// @Failure 404 {object} Response "标签不存在"
// @Router /tags/{id} [delete].
func (h *Handlers) deleteTag(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteTag(id); err != nil {
		if err == ErrTagNotFound {
			c.JSON(http.StatusNotFound, apiresponse.Error(404, "标签不存在"))
			return
		}
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "删除标签失败"))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(nil))
}

// searchTags 搜索标签
// @Summary 搜索标签
// @Description 按名称模糊搜索标签
// @Tags tags
// @Accept json
// @Produce json
// @Param q query string true "搜索关键词"
// @Success 200 {object} Response "成功"
// @Router /tags/search [get].
func (h *Handlers) searchTags(c *gin.Context) {
	keyword := c.Query("q")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, "请提供搜索关键词"))
		return
	}

	tags, err := h.manager.SearchTags(keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "搜索标签失败"))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(tags))
}

// listGroups 列出标签分组
// @Summary 列出标签分组
// @Description 获取所有标签分组及其标签数量
// @Tags tags
// @Accept json
// @Produce json
// @Success 200 {object} Response "成功"
// @Router /tags/groups [get].
func (h *Handlers) listGroups(c *gin.Context) {
	groups, err := h.manager.ListGroups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "获取分组列表失败"))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(groups))
}

// getStats 获取统计信息
// @Summary 获取统计信息
// @Description 获取标签系统的统计信息
// @Tags tags
// @Accept json
// @Produce json
// @Success 200 {object} Response "成功"
// @Router /tags/stats [get].
func (h *Handlers) getStats(c *gin.Context) {
	stats, err := h.manager.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "获取统计信息失败"))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(stats))
}

// getTagFiles 获取标签关联的文件
// @Summary 获取标签关联的文件
// @Description 获取拥有指定标签的所有文件
// @Tags tags
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Success 200 {object} Response "成功"
// @Router /tags/{id}/files [get].
func (h *Handlers) getTagFiles(c *gin.Context) {
	id := c.Param("id")

	// 检查标签是否存在
	_, err := h.manager.GetTag(id)
	if err != nil {
		if err == ErrTagNotFound {
			c.JSON(http.StatusNotFound, apiresponse.Error(404, "标签不存在"))
			return
		}
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "获取标签失败"))
		return
	}

	files, err := h.manager.GetFilesByTags([]string{id}, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "获取文件列表失败"))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(files))
}

// getTagUsage 获取标签使用次数
// @Summary 获取标签使用次数
// @Description 获取指定标签被多少文件使用
// @Tags tags
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Success 200 {object} Response "成功"
// @Router /tags/{id}/usage [get].
func (h *Handlers) getTagUsage(c *gin.Context) {
	id := c.Param("id")

	// 检查标签是否存在
	_, err := h.manager.GetTag(id)
	if err != nil {
		if err == ErrTagNotFound {
			c.JSON(http.StatusNotFound, apiresponse.Error(404, "标签不存在"))
			return
		}
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "获取标签失败"))
		return
	}

	count, err := h.manager.GetTagUsageCount(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "获取使用次数失败"))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(gin.H{"count": count}))
}

// ========== 文件标签 API ==========

// getFileTags 获取文件的标签
// @Summary 获取文件的标签
// @Description 获取指定文件的所有标签
// @Tags files
// @Accept json
// @Produce json
// @Param id path string true "文件路径（URL编码）"
// @Success 200 {object} Response "成功"
// @Router /files/{id}/tags [get].
func (h *Handlers) getFileTags(c *gin.Context) {
	filePath := c.Param("id")

	tags, err := h.manager.GetTagsForFile(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "获取文件标签失败"))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(tags))
}

// addFileTags 为文件添加标签
// @Summary 为文件添加标签
// @Description 为指定文件添加一个或多个标签
// @Tags files
// @Accept json
// @Produce json
// @Param id path string true "文件路径（URL编码）"
// @Param request body addFileTagsRequest true "标签ID列表"
// @Success 200 {object} Response "成功"
// @Failure 400 {object} Response "请求参数错误"
// @Router /files/{id}/tags [post].
func (h *Handlers) addFileTags(c *gin.Context) {
	filePath := c.Param("id")

	var req addFileTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, "无效的请求参数"))
		return
	}

	if len(req.TagIDs) == 0 {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, "请提供标签ID"))
		return
	}

	err := h.manager.AddTagsToFile(filePath, req.TagIDs)
	if err != nil {
		switch err {
		case ErrInvalidTagID:
			c.JSON(http.StatusBadRequest, apiresponse.Error(400, "无效的标签ID"))
		case ErrInvalidFilePath:
			c.JSON(http.StatusBadRequest, apiresponse.Error(400, "无效的文件路径"))
		default:
			c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "添加标签失败"))
		}
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(nil))
}

// removeFileTag 从文件移除单个标签
// @Summary 从文件移除标签
// @Description 从指定文件移除单个标签
// @Tags files
// @Accept json
// @Produce json
// @Param id path string true "文件路径（URL编码）"
// @Param tagId path string true "标签ID"
// @Success 200 {object} Response "成功"
// @Router /files/{id}/tags/{tagId} [delete].
func (h *Handlers) removeFileTag(c *gin.Context) {
	filePath := c.Param("id")
	tagID := c.Param("tagId")

	err := h.manager.RemoveTagsFromFile(filePath, []string{tagID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "移除标签失败"))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(nil))
}

// getFilesByTagID 按标签ID查询文件
// @Summary 按标签ID查询文件
// @Description 获取拥有指定标签的所有文件
// @Tags files
// @Accept json
// @Produce json
// @Param tagId path string true "标签ID"
// @Success 200 {object} Response "成功"
// @Router /files/by-tag/{tagId} [get].
func (h *Handlers) getFilesByTagID(c *gin.Context) {
	tagID := c.Param("tagId")

	// 检查标签是否存在
	_, err := h.manager.GetTag(tagID)
	if err != nil {
		if err == ErrTagNotFound {
			c.JSON(http.StatusNotFound, apiresponse.Error(404, "标签不存在"))
			return
		}
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "获取标签失败"))
		return
	}

	files, err := h.manager.GetFilesByTags([]string{tagID}, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "查询文件失败"))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(files))
}

// batchTagFiles 批量标签操作
// @Summary 批量标签操作
// @Description 为多个文件批量添加或设置标签
// @Tags files
// @Accept json
// @Produce json
// @Param request body BatchTagRequest true "批量标签请求"
// @Success 200 {object} Response "成功"
// @Failure 400 {object} Response "请求参数错误"
// @Router /files/batch-tags [post].
func (h *Handlers) batchTagFiles(c *gin.Context) {
	var req BatchTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, "无效的请求参数"))
		return
	}

	if len(req.FilePaths) == 0 {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, "请提供文件路径"))
		return
	}
	if len(req.TagIDs) == 0 {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, "请提供标签ID"))
		return
	}

	var err error
	switch req.Action {
	case "add":
		err = h.manager.BatchAddTagsToFile(req.FilePaths, req.TagIDs)
	case "set":
		// 设置标签：先清除现有标签，再添加新标签
		for _, filePath := range req.FilePaths {
			if e := h.manager.SetFileTags(filePath, req.TagIDs); e != nil {
				err = e
				break
			}
		}
	case "remove":
		for _, filePath := range req.FilePaths {
			if e := h.manager.RemoveTagsFromFile(filePath, req.TagIDs); e != nil {
				err = e
				break
			}
		}
	case "clear":
		for _, filePath := range req.FilePaths {
			if e := h.manager.ClearFileTags(filePath); e != nil {
				err = e
				break
			}
		}
	default:
		// 默认行为：添加
		err = h.manager.BatchAddTagsToFile(req.FilePaths, req.TagIDs)
	}

	if err != nil {
		switch err {
		case ErrInvalidTagID:
			c.JSON(http.StatusBadRequest, apiresponse.Error(400, "无效的标签ID"))
		case ErrInvalidFilePath:
			c.JSON(http.StatusBadRequest, apiresponse.Error(400, "无效的文件路径"))
		default:
			c.JSON(http.StatusInternalServerError, apiresponse.Error(500, "批量操作失败"))
		}
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(nil))
}

// addFileTagsRequest 添加文件标签请求.
type addFileTagsRequest struct {
	TagIDs []string `json:"tagIds" binding:"required"`
}

// BatchTagRequest 批量标签请求.
type BatchTagRequest struct {
	Action    string   `json:"action"` // add, set, remove, clear
	FilePaths []string `json:"filePaths" binding:"required"`
	TagIDs    []string `json:"tagIds"`
}

// ========== 兼容旧接口 ==========

// getFilesByTags 按标签查询文件（兼容旧接口）
// @Summary 按标签查询文件
// @Description 获取拥有指定标签的文件列表
// @Tags files
// @Accept json
// @Produce json
// @Param tags query string true "标签ID列表，逗号分隔"
// @Param match query string false "匹配模式：all（必须包含所有标签）或 any（包含任意标签），默认 any"
// @Param q query string false "文件路径关键词"
// @Success 200 {object} Response "成功"
// @Router /files/by-tags [get]
