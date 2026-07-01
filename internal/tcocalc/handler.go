// Package tcocalc 提供TCO总拥有成本分析HTTP接口
package tcocalc

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler TCO计算HTTP处理器
type Handler struct {
	service *Service
}

// NewHandler 创建HTTP处理器
func NewHandler(svc *Service) *Handler {
	return &Handler{service: svc}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/tcocalc")
	{
		g.POST("/calculate", h.Calculate)
		g.GET("/cloud-pricing", h.CloudPricing)
	}
}

// Calculate 计算TCO分析报告
// POST /api/v1/tcocalc/calculate
func (h *Handler) Calculate(c *gin.Context) {
	var req TCORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	report, err := h.service.Calculate(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": report})
}

// CloudPricing 获取云服务商定价列表
// GET /api/v1/tcocalc/cloud-pricing
func (h *Handler) CloudPricing(c *gin.Context) {
	pricing := h.service.GetCloudPricing()
	c.JSON(http.StatusOK, gin.H{"data": pricing})
}