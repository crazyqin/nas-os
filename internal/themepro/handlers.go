// Package themepro 提供 REST API 处理器
package themepro

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 主题 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	theme := r.Group("/theme")
	{
		// 主题管理
		theme.GET("", h.listThemes)
		theme.GET("/active", h.getActiveTheme)
		theme.GET("/:id", h.getTheme)
		theme.POST("", h.createTheme)
		theme.PUT("/:id", h.updateTheme)
		theme.DELETE("/:id", h.deleteTheme)
		theme.POST("/:id/apply", h.applyTheme)
		theme.POST("/:id/duplicate", h.duplicateTheme)

		// 分类查询
		theme.GET("/builtin", h.getBuiltinThemes)
		theme.GET("/custom", h.getCustomThemes)
		theme.GET("/defaults", h.getDefaultThemes)

		// 导入导出
		theme.POST("/export", h.exportTheme)
		theme.POST("/import", h.importTheme)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// listThemes 列出所有主题
func (h *Handlers) listThemes(c *gin.Context) {
	themes := h.manager.ListThemes()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    themes,
	})
}

// getActiveTheme 获取当前活跃主题
func (h *Handlers) getActiveTheme(c *gin.Context) {
	theme, err := h.manager.GetActiveTheme()
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    theme,
	})
}

// getTheme 获取主题
func (h *Handlers) getTheme(c *gin.Context) {
	id := c.Param("id")
	theme, err := h.manager.GetTheme(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    theme,
	})
}

// createTheme 创建主题
func (h *Handlers) createTheme(c *gin.Context) {
	var req CreateThemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	theme, err := h.manager.CreateCustomTheme(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "theme created",
		Data:    theme,
	})
}

// updateTheme 更新主题
func (h *Handlers) updateTheme(c *gin.Context) {
	id := c.Param("id")
	var req UpdateThemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	theme, err := h.manager.UpdateTheme(id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "theme updated",
		Data:    theme,
	})
}

// deleteTheme 删除主题
func (h *Handlers) deleteTheme(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteTheme(id); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "theme deleted",
	})
}

// applyTheme 应用主题
func (h *Handlers) applyTheme(c *gin.Context) {
	id := c.Param("id")
	theme, err := h.manager.ApplyTheme(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "theme applied",
		Data:    theme,
	})
}

// duplicateTheme 复制主题
func (h *Handlers) duplicateTheme(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	theme, err := h.manager.DuplicateTheme(id, req.Name)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "theme duplicated",
		Data:    theme,
	})
}

// getBuiltinThemes 获取内置主题
func (h *Handlers) getBuiltinThemes(c *gin.Context) {
	themes := h.manager.GetBuiltinThemes()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    themes,
	})
}

// getCustomThemes 获取自定义主题
func (h *Handlers) getCustomThemes(c *gin.Context) {
	themes := h.manager.GetCustomThemes()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    themes,
	})
}

// getDefaultThemes 获取默认主题
func (h *Handlers) getDefaultThemes(c *gin.Context) {
	themes := h.manager.GetDefaultThemes()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    themes,
	})
}

// exportTheme 导出主题
func (h *Handlers) exportTheme(c *gin.Context) {
	var req struct {
		ThemeIDs []string `json:"theme_ids" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	pack, err := h.manager.ExportTheme(req.ThemeIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "theme exported",
		Data:    pack,
	})
}

// importTheme 导入主题
func (h *Handlers) importTheme(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "failed to read request body",
		})
		return
	}

	themes, err := h.manager.ImportTheme(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "themes imported",
		Data:    themes,
	})
}
