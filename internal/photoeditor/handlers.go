// Package photoeditor 提供REST API处理器
package photoeditor

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type Handlers struct {
	manager *Manager
}

func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	editor := r.Group("/photoeditor")
	{
		editor.POST("/edit", h.Edit)
		editor.GET("/presets", h.GetPresets)
		editor.GET("/presets/:id", h.GetPreset)
		editor.POST("/presets", h.CreatePreset)
		editor.DELETE("/presets/:id", h.DeletePreset)
	}
}

// Edit 编辑照片.
func (h *Handlers) Edit(c *gin.Context) {
	var req EditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	result, err := h.manager.ApplyEdit(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 500, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 200, Data: result})
}

// GetPresets 获取所有预设.
func (h *Handlers) GetPresets(c *gin.Context) {
	presets := h.manager.GetPresets()
	c.JSON(http.StatusOK, response{Code: 200, Data: presets})
}

// GetPreset 获取预设.
func (h *Handlers) GetPreset(c *gin.Context) {
	id := c.Param("id")
	preset, err := h.manager.GetPreset(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 404, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 200, Data: preset})
}

// CreatePreset 创建预设.
func (h *Handlers) CreatePreset(c *gin.Context) {
	var preset Preset
	if err := c.ShouldBindJSON(&preset); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	created, err := h.manager.CreatePreset(preset)
	if err != nil {
		c.JSON(http.StatusConflict, response{Code: 409, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 201, Data: created})
}

// DeletePreset 删除预设.
func (h *Handlers) DeletePreset(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePreset(id); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 200, Message: "deleted"})
}
