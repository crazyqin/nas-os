package scrubsmart

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler HTTP API 处理器.
type Handler struct {
	scheduler *Scheduler
	logger    *zap.Logger
}

// NewHandler 创建 HTTP 处理器.
func NewHandler(scheduler *Scheduler, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		scheduler: scheduler,
		logger:    logger,
	}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	scrubGroup := api.Group("/scrubsmart")
	{
		scrubGroup.GET("/status", h.status)
		scrubGroup.POST("/config", h.updateConfig)
		scrubGroup.POST("/pause", h.pause)
		scrubGroup.POST("/resume", h.resume)
	}
}

// status 获取 Scrub 智能调度状态.
func (h *Handler) status(c *gin.Context) {
	status := h.scheduler.GetStatus()
	c.JSON(http.StatusOK, status)
}

// updateConfig 更新配置.
func (h *Handler) updateConfig(c *gin.Context) {
	var cfg Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	// 验证每个避峰窗口
	for _, w := range cfg.AvoidanceWindows {
		if err := w.Validate(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":  "无效的避峰窗口配置",
				"window": w.Name,
				"detail": err.Error(),
			})
			return
		}
	}

	h.scheduler.SetConfig(&cfg)
	h.logger.Info("配置已更新", zap.Int("windows", len(cfg.AvoidanceWindows)))

	c.JSON(http.StatusOK, gin.H{
		"message": "配置已更新",
		"config":  cfg,
		"status":  h.scheduler.GetStatus(),
	})
}

// pause 手动暂停 Scrub.
func (h *Handler) pause(c *gin.Context) {
	var req PauseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许空 body
		req.Reason = "手动暂停"
	}

	if err := h.scheduler.Pause(req.Reason); err != nil {
		h.logger.Error("暂停 Scrub 失败", zap.Error(err))
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("手动暂停 Scrub", zap.String("reason", req.Reason))
	c.JSON(http.StatusOK, gin.H{
		"message": "Scrub 已暂停",
		"reason":  req.Reason,
		"status":  h.scheduler.GetStatus(),
	})
}

// resume 手动恢复 Scrub.
func (h *Handler) resume(c *gin.Context) {
	var req ResumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许空 body，默认非强制
		req.Force = false
	}

	if err := h.scheduler.Resume(req.Force); err != nil {
		h.logger.Error("恢复 Scrub 失败", zap.Error(err))
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("恢复 Scrub", zap.Bool("force", req.Force))
	c.JSON(http.StatusOK, gin.H{
		"message": "Scrub 已恢复",
		"force":   req.Force,
		"status":  h.scheduler.GetStatus(),
	})
}
