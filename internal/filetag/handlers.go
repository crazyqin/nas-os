package filetag

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler HTTP API处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建Handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	ft := rg.Group("/filetag")
	{
		// 标签管理
		ft.POST("/tags", h.CreateTag)
		ft.GET("/tags", h.ListTags)
		ft.GET("/tags/:id", h.GetTag)
		ft.PUT("/tags/:id", h.UpdateTag)
		ft.DELETE("/tags/:id", h.DeleteTag)

		// 文件标签关联
		ft.POST("/files/:file/tag", h.TagFile)
		ft.DELETE("/files/:file/tag/:tagId", h.UntagFile)
		ft.GET("/files/:file/tags", h.GetFileTags)

		// 标签文件查询
		ft.GET("/tags/:id/files", h.GetTagFiles)

		// 搜索
		ft.POST("/search", h.SearchFiles)

		// 批量操作
		ft.POST("/batch/tag", h.BatchTag)
		ft.POST("/batch/untag", h.BatchUntag)

		// 统计
		ft.GET("/stats", h.GetAllStats)
		ft.GET("/stats/:id", h.GetTagStats)
		ft.GET("/categories", h.GetCategories)
	}
}

// CreateTag 创建标签
func (h *Handler) CreateTag(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Color       string `json:"color"`
		Description string `json:"description"`
		Category    string `json:"category"`
		CreatedBy   string `json:"created_by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "参数错误: " + err.Error(),
		})
		return
	}

	tag, err := h.manager.CreateTag(req.Name, req.Color, req.Description, req.Category, req.CreatedBy)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data:    tag,
	})
}

// GetTag 获取标签
func (h *Handler) GetTag(c *gin.Context) {
	tagID := c.Param("id")
	tag, err := h.manager.GetTag(tagID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    tag,
	})
}

// ListTags 列出标签
func (h *Handler) ListTags(c *gin.Context) {
	category := c.Query("category")
	tags := h.manager.ListTags(category)

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    tags,
	})
}

// UpdateTag 更新标签
func (h *Handler) UpdateTag(c *gin.Context) {
	tagID := c.Param("id")
	var req struct {
		Name        string `json:"name"`
		Color       string `json:"color"`
		Description string `json:"description"`
		Category    string `json:"category"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "参数错误: " + err.Error(),
		})
		return
	}

	tag, err := h.manager.UpdateTag(tagID, req.Name, req.Color, req.Description, req.Category)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    tag,
	})
}

// DeleteTag 删除标签
func (h *Handler) DeleteTag(c *gin.Context) {
	tagID := c.Param("id")
	if err := h.manager.DeleteTag(tagID); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "标签已删除",
	})
}

// TagFile 为文件添加标签
func (h *Handler) TagFile(c *gin.Context) {
	filePath := c.Param("file")
	var req struct {
		TagID    string `json:"tag_id" binding:"required"`
		TaggedBy string `json:"tagged_by"`
		Note     string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "参数错误: " + err.Error(),
		})
		return
	}

	fileTag, err := h.manager.TagFile(filePath, req.TagID, req.TaggedBy, req.Note)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data:    fileTag,
	})
}

// UntagFile 移除文件标签
func (h *Handler) UntagFile(c *gin.Context) {
	filePath := c.Param("file")
	tagID := c.Param("tagId")

	if err := h.manager.UntagFile(filePath, tagID); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "标签已移除",
	})
}

// GetFileTags 获取文件的所有标签
func (h *Handler) GetFileTags(c *gin.Context) {
	filePath := c.Param("file")
	tags := h.manager.GetFileTags(filePath)

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    tags,
	})
}

// GetTagFiles 获取标签关联的所有文件
func (h *Handler) GetTagFiles(c *gin.Context) {
	tagID := c.Param("id")
	files := h.manager.GetTagFiles(tagID)

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    files,
	})
}

// SearchFiles 搜索文件
func (h *Handler) SearchFiles(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "参数错误: " + err.Error(),
		})
		return
	}

	// 设置分页默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	result := h.manager.SearchFiles(&req)

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    result,
	})
}

// BatchTag 批量打标签
func (h *Handler) BatchTag(c *gin.Context) {
	var req BatchTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "参数错误: " + err.Error(),
		})
		return
	}

	results, err := h.manager.BatchTag(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    results,
		Message: "批量打标签完成",
	})
}

// BatchUntag 批量移除标签
func (h *Handler) BatchUntag(c *gin.Context) {
	var req BatchUntagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "参数错误: " + err.Error(),
		})
		return
	}

	if err := h.manager.BatchUntag(&req); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "批量移除标签完成",
	})
}

// GetAllStats 获取所有标签统计
func (h *Handler) GetAllStats(c *gin.Context) {
	stats := h.manager.GetAllStats()

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    stats,
	})
}

// GetTagStats 获取标签统计
func (h *Handler) GetTagStats(c *gin.Context) {
	tagID := c.Param("id")
	stat, err := h.manager.GetTagStats(tagID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    stat,
	})
}

// GetCategories 获取所有分类
func (h *Handler) GetCategories(c *gin.Context) {
	categories := h.manager.GetCategories()

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    categories,
	})
}

// ParsePagination 解析分页参数
func ParsePagination(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return
}