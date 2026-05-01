package healthscore

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 健康评分 HTTP 处理器
type Handlers struct {
	mgr *Manager
}

// NewHandlers 创建处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	health := api.Group("/healthscore")
	{
		health.GET("/report", h.GetReport)
		health.POST("/check", h.RunCheck)
	}
}

func (h *Handlers) GetReport(c *gin.Context) {
	report := h.mgr.GetLatest()
	if report == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "尚未执行健康检查，请先调用 POST /api/v1/healthscore/check"})
		return
	}
	c.JSON(http.StatusOK, report)
}

func (h *Handlers) RunCheck(c *gin.Context) {
	report := h.mgr.RunCheck()
	c.JSON(http.StatusOK, report)
}
