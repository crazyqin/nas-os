package onboarding

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 引导系统HTTP处理器
type Handler struct {
	ob *Onboarding
}

// NewHandler 创建处理器
func NewHandler(ob *Onboarding) *Handler {
	return &Handler{ob: ob}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	onboarding := rg.Group("/onboarding")
	{
		onboarding.GET("/status", h.GetStatus)
		onboarding.POST("/start", h.Start)
		onboarding.POST("/complete/:step", h.CompleteStep)
		onboarding.POST("/skip", h.Skip)
		onboarding.POST("/reset", h.Reset)
		onboarding.GET("/steps", h.GetSteps)
		onboarding.GET("/quickstart", h.GetQuickStartCards)
		onboarding.GET("/tutorials", h.GetTutorials)
		onboarding.GET("/tutorials/:id", h.GetTutorial)
	}
}

// GetStatus 获取引导状态
func (h *Handler) GetStatus(c *gin.Context) {
	state := h.ob.GetState()
	total, completed, inProgress, notStarted := h.ob.GetProgress()
	stats := h.ob.GetCompletionStats()
	c.JSON(http.StatusOK, gin.H{
		"state":       state,
		"totalSteps":  total,
		"completed":   completed,
		"inProgress":  inProgress,
		"notStarted":  notStarted,
		"completionRate": func() float64 {
			if total == 0 {
				return 0
			}
			return float64(completed) / float64(total) * 100
		}(),
		"stats": stats,
	})
}

// Start 开始引导
func (h *Handler) Start(c *gin.Context) {
	if err := h.ob.Start(); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "onboarding started"})
}

// CompleteStep 完成某步骤
func (h *Handler) CompleteStep(c *gin.Context) {
	stepID := c.Param("step")
	if stepID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "step parameter is required"})
		return
	}
	if err := h.ob.CompleteStep(stepID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "step " + stepID + " completed"})
}

// Skip 跳过引导
func (h *Handler) Skip(c *gin.Context) {
	if err := h.ob.Skip(); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "onboarding skipped"})
}

// Reset 重置引导
func (h *Handler) Reset(c *gin.Context) {
	h.ob.Reset()
	c.JSON(http.StatusOK, gin.H{"message": "onboarding reset"})
}

// GetSteps 获取步骤列表
func (h *Handler) GetSteps(c *gin.Context) {
	steps := h.ob.GetSteps()
	c.JSON(http.StatusOK, steps)
}

// GetQuickStartCards 获取快速入门卡片
func (h *Handler) GetQuickStartCards(c *gin.Context) {
	cards := h.ob.GetQuickStartCards()
	c.JSON(http.StatusOK, cards)
}

// GetTutorials 获取教程列表
func (h *Handler) GetTutorials(c *gin.Context) {
	tutorials := h.ob.GetTutorials()
	c.JSON(http.StatusOK, tutorials)
}

// GetTutorial 获取教程详情
func (h *Handler) GetTutorial(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tutorial id is required"})
		return
	}
	tutorial, err := h.ob.GetTutorial(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tutorial)
}
