// Package stocatcalc 提供存储总拥有成本（TCO）计算HTTP接口
package stocatcalc

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 存储TCO计算HTTP处理器
type Handler struct {
	service *Service
}

// NewHandler 创建HTTP处理器
func NewHandler(svc *Service) *Handler {
	return &Handler{service: svc}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/stocatcalc")
	{
		g.POST("/calculate", h.Calculate)
		g.GET("/templates", h.Templates)
		g.POST("/compare", h.Compare)
	}
}

// Calculate 计算存储TCO
// POST /api/v1/stocatcalc/calculate
func (h *Handler) Calculate(c *gin.Context) {
	var req CalcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	result, err := h.service.Calculate(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// Templates 获取预置模板列表
// GET /api/v1/stocatcalc/templates
func (h *Handler) Templates(c *gin.Context) {
	templates := h.service.GetTemplates()
	c.JSON(http.StatusOK, gin.H{"data": templates})
}

// Compare 对比多个存储方案
// POST /api/v1/stocatcalc/compare
func (h *Handler) Compare(c *gin.Context) {
	var reqs []CalcRequest
	if err := c.ShouldBindJSON(&reqs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	result, err := h.service.Compare(reqs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}
