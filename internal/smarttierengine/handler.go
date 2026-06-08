package smarttierengine

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 统一智能分层引擎HTTP处理器
type Handler struct {
	engine *SmartTierEngine
}

// NewHandler 创建处理器
func NewHandler(engine *SmartTierEngine) *Handler {
	return &Handler{engine: engine}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	group := rg.Group("/smarttierengine")
	{
		group.GET("/stats", h.GetStats)
		group.GET("/files", h.GetFiles)
		group.GET("/migrations", h.GetMigrations)
		group.POST("/record", h.RecordAccess)
		group.POST("/start", h.Start)
		group.POST("/stop", h.Stop)
	}
}

// GetStats 获取引擎统计
func (h *Handler) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": h.engine.GetStats()})
}

// GetFiles 获取文件热度列表
func (h *Handler) GetFiles(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": h.engine.GetFiles()})
}

// GetMigrations 获取迁移任务
func (h *Handler) GetMigrations(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": h.engine.GetMigrations()})
}

// RecordAccess 记录文件访问
func (h *Handler) RecordAccess(c *gin.Context) {
	var req struct {
		Path   string `json:"path" binding:"required"`
		Size   int64  `json:"size"`
		IsRead bool   `json:"isRead"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.engine.RecordAccess(req.Path, req.Size, req.IsRead)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Start 启动引擎
func (h *Handler) Start(c *gin.Context) {
	h.engine.Start()
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "引擎已启动"})
}

// Stop 停止引擎
func (h *Handler) Stop(c *gin.Context) {
	h.engine.Stop()
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "引擎已停止"})
}
