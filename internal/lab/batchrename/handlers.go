// Package batchrename 提供REST API处理器
package batchrename

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
	br := r.Group("/batchrename")
	{
		br.POST("/preview", h.Preview)
		br.POST("/rename", h.Rename)
	}
}

// Preview 预览重命名结果.
func (h *Handlers) Preview(c *gin.Context) {
	var req BatchRenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	previews := h.manager.Preview(req.Files, req.Rule)
	c.JSON(http.StatusOK, response{Code: 200, Data: previews})
}

// Rename 执行重命名.
func (h *Handlers) Rename(c *gin.Context) {
	var req BatchRenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	if req.DryRun {
		previews := h.manager.Preview(req.Files, req.Rule)
		c.JSON(http.StatusOK, response{Code: 200, Message: "dry run", Data: previews})
		return
	}

	result := h.manager.Rename(req.Files, req.Rule)
	c.JSON(http.StatusOK, response{Code: 200, Data: result})
}
