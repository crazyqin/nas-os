package ransomai

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler AI勒索检测HTTP处理器
type Handler struct {
	ra *RansomAI
}

// NewHandler 创建处理器
func NewHandler(ra *RansomAI) *Handler {
	return &Handler{ra: ra}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	group := rg.Group("/ransomai")
	{
		group.GET("/alerts", h.GetAlerts)
		group.GET("/honeypots", h.GetHoneypots)
		group.GET("/config", h.GetConfig)
		group.POST("/event", h.RecordEvent)
		group.POST("/honeypot", h.AddHoneypot)
		group.POST("/start", h.Start)
		group.POST("/stop", h.Stop)
	}
}

// GetAlerts 获取告警
func (h *Handler) GetAlerts(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": h.ra.GetAlerts()})
}

// GetHoneypots 获取蜜罐
func (h *Handler) GetHoneypots(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": h.ra.GetHoneypots()})
}

// GetConfig 获取配置
func (h *Handler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": h.ra.GetConfig()})
}

// RecordEvent 记录文件事件
func (h *Handler) RecordEvent(c *gin.Context) {
	var event FileEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	alert := h.ra.RecordEvent(event)
	if alert != nil {
		c.JSON(http.StatusOK, gin.H{"status": "alert", "data": alert})
	} else {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

// AddHoneypot 添加蜜罐
func (h *Handler) AddHoneypot(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
		Ext  string `json:"ext"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.ra.AddHoneypot(req.Path, req.Ext)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Start 启动
func (h *Handler) Start(c *gin.Context) {
	h.ra.Start()
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Stop 停止
func (h *Handler) Stop(c *gin.Context) {
	h.ra.Stop()
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
