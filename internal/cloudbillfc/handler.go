// Package cloudbillfc 提供云存储成本预测HTTP接口
package cloudbillfc

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 云存储成本预测HTTP处理器
type Handler struct {
	service *Service
}

// NewHandler 创建HTTP处理器
func NewHandler(svc *Service) *Handler {
	return &Handler{service: svc}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/cloudbillfc")
	{
		g.POST("/forecast", h.Forecast)
		g.GET("/providers", h.Providers)
		g.POST("/compare", h.Compare)
	}
}

// Forecast 预测云存储成本
// POST /api/v1/cloudbillfc/forecast
func (h *Handler) Forecast(c *gin.Context) {
	var config ForecastConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	report, err := h.service.Forecast(config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": report})
}

// Providers 获取云服务商定价列表
// GET /api/v1/cloudbillfc/providers
func (h *Handler) Providers(c *gin.Context) {
	providers := h.service.GetProviders()
	c.JSON(http.StatusOK, gin.H{"data": providers})
}

// CompareRequest 对比请求
type CompareRequest struct {
	StorageGB       float64 `json:"storage_gb"`
	MonthlyEgressGB float64 `json:"monthly_egress_gb"`
	MonthlyAPI10K   float64 `json:"monthly_api_10k"`
}

// Compare 对比多个云服务商
// POST /api/v1/cloudbillfc/compare
func (h *Handler) Compare(c *gin.Context) {
	var req CompareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if req.StorageGB < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "存储用量不能为负"})
		return
	}

	comparisons := h.service.CompareProviders(req.StorageGB, req.MonthlyEgressGB, req.MonthlyAPI10K)
	c.JSON(http.StatusOK, gin.H{"data": comparisons})
}