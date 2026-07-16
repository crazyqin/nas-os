package aiwriter

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers HTTP处理器.
type Handlers struct{ mgr *Manager }

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager) *Handlers { return &Handlers{mgr: mgr} }

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/ai-writer")
	{
		g.POST("/generate", h.Generate)
		g.POST("/template/fill", h.FillTemplate)
		g.GET("/templates", h.ListTemplates)
		g.GET("/templates/:id", h.GetTemplate)
		g.GET("/history", h.GetHistory)
		g.GET("/stats", h.GetStats)
	}
}

// Generate 生成文本.
func (h *Handlers) Generate(c *gin.Context) {
	var req WriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	result, err := h.mgr.GenerateText(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

// FillTemplate 填充模板.
func (h *Handlers) FillTemplate(c *gin.Context) {
	var req TemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	result, err := h.mgr.FillTemplate(&req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

// ListTemplates 列出模板.
func (h *Handlers) ListTemplates(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.ListTemplates()})
}

// GetTemplate 获取模板.
func (h *Handlers) GetTemplate(c *gin.Context) {
	tmpl, err := h.mgr.GetTemplate(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": tmpl})
}

// GetHistory 获取历史记录.
func (h *Handlers) GetHistory(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.GetHistory()})
}

// GetStats 获取统计信息.
func (h *Handlers) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.GetStats()})
}
