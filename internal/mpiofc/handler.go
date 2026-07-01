// Package mpiofc HTTP 处理器
// 提供多路径 I/O 光纤通道管理的 REST API
package mpiofc

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 多路径 I/O 管理 API 处理器
type Handler struct {
	service *Service
}

// NewHandler 创建 API 处理器
func NewHandler(svc *Service) *Handler {
	return &Handler{service: svc}
}

// RegisterRoutes 注册路由到指定路由组
// 路由前缀: /api/v1/mpiofc
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/api/v1/mpiofc")
	{
		g.GET("/ports", h.listPorts)         // 检测/列出 HBA 端口
		g.POST("/configure", h.configure)    // 配置多路径
		g.GET("/status", h.getStatus)        // 查看路径状态
		g.GET("/statistics", h.getStatistics) // 查看统计信息
	}
}

// listPorts 列出所有光纤通道 HBA 端口
// GET /api/v1/mpiofc/ports
func (h *Handler) listPorts(c *gin.Context) {
	ports := h.service.GetPorts()
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    ports,
	})
}

// configure 配置多路径
// POST /api/v1/mpiofc/configure
// Body: {"targetWwpn":"5001438023456789","policy":"round-robin","paths":[{"hbaPortId":"fc_host0","priority":1}]}
func (h *Handler) configure(c *gin.Context) {
	var req MPIOConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	paths, err := h.service.ConfigureMPIO(&req)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Message: "多路径配置成功",
		Data:    paths,
	})
}

// getStatus 获取多路径状态总览
// GET /api/v1/mpiofc/status
func (h *Handler) getStatus(c *gin.Context) {
	status := h.service.GetStatus()
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    status,
	})
}

// getStatistics 获取路径统计信息
// GET /api/v1/mpiofc/statistics
func (h *Handler) getStatistics(c *gin.Context) {
	stats := h.service.GetStatistics()
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    stats,
	})
}
