package onboarding

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 引导系统HTTP处理器
type Handler struct {
	ob     *Onboarding
	wizard *WizardEngine
}

// NewHandler 创建处理器
func NewHandler(ob *Onboarding) *Handler {
	return &Handler{ob: ob}
}

// NewHandlerWithWizard 创建带向导引擎的处理器
func NewHandlerWithWizard(ob *Onboarding, wizard *WizardEngine) *Handler {
	return &Handler{ob: ob, wizard: wizard}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	onboarding := rg.Group("/onboarding")
	{
		// ---- 原有路由 ----
		onboarding.GET("/status", h.GetStatus)
		onboarding.POST("/start", h.Start)
		onboarding.POST("/complete/:step", h.CompleteStep)
		onboarding.POST("/skip", h.Skip)
		onboarding.POST("/reset", h.Reset)
		onboarding.GET("/steps", h.GetSteps)
		onboarding.GET("/quickstart", h.GetQuickStartCards)
		onboarding.GET("/tutorials", h.GetTutorials)
		onboarding.GET("/tutorials/:id", h.GetTutorial)

		// ---- 向导 2.0 路由 ----
		onboarding.GET("/wizard/status", h.GetWizardStatus)
		onboarding.POST("/wizard/start", h.StartWizard)
		onboarding.POST("/wizard/complete-step", h.CompleteWizardStep)
		onboarding.POST("/wizard/skip-step", h.SkipWizardStep)
		onboarding.POST("/wizard/reset", h.ResetWizard)
		onboarding.POST("/wizard/hardware", h.SetHardware)
		onboarding.GET("/wizard/recommendation", h.GetRAIDRecommendation)
		onboarding.POST("/wizard/detect", h.DetectAutoSkip)
	}
}

// ============================================================
// 原有 handler（不变）
// ============================================================

// GetStatus 获取引导状态
func (h *Handler) GetStatus(c *gin.Context) {
	state := h.ob.GetState()
	total, completed, inProgress, notStarted := h.ob.GetProgress()
	stats := h.ob.GetCompletionStats()
	c.JSON(http.StatusOK, gin.H{
		"state":          state,
		"totalSteps":     total,
		"completed":      completed,
		"inProgress":     inProgress,
		"notStarted":     notStarted,
		"completionRate": safeRate(completed, total),
		"stats":          stats,
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

// ============================================================
// 向导 2.0 handler
// ============================================================

// GetWizardStatus 获取向导状态
func (h *Handler) GetWizardStatus(c *gin.Context) {
	if h.wizard == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "wizard not initialized"})
		return
	}
	state := h.wizard.GetState()
	c.JSON(http.StatusOK, state)
}

// StartWizard 启动向导
func (h *Handler) StartWizard(c *gin.Context) {
	if h.wizard == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "wizard not initialized"})
		return
	}
	if err := h.wizard.Start(); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "wizard started"})
}

// CompleteWizardStepReq 完成向导步骤请求体
type CompleteWizardStepReq struct {
	StepID string `json:"stepId" binding:"required"`
}

// CompleteWizardStep 完成向导步骤
func (h *Handler) CompleteWizardStep(c *gin.Context) {
	if h.wizard == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "wizard not initialized"})
		return
	}
	var req CompleteWizardStepReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "stepId is required"})
		return
	}
	if err := h.wizard.CompleteStep(req.StepID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "step " + req.StepID + " completed"})
}

// SkipWizardStepReq 跳过向导步骤请求体
type SkipWizardStepReq struct {
	StepID string `json:"stepId" binding:"required"`
}

// SkipWizardStep 跳过向导步骤
func (h *Handler) SkipWizardStep(c *gin.Context) {
	if h.wizard == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "wizard not initialized"})
		return
	}
	var req SkipWizardStepReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "stepId is required"})
		return
	}
	if err := h.wizard.SkipStep(req.StepID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "step " + req.StepID + " skipped"})
}

// ResetWizard 重置向导
func (h *Handler) ResetWizard(c *gin.Context) {
	if h.wizard == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "wizard not initialized"})
		return
	}
	h.wizard.Reset()
	c.JSON(http.StatusOK, gin.H{"message": "wizard reset"})
}

// SetHardware 设置硬件配置
func (h *Handler) SetHardware(c *gin.Context) {
	if h.wizard == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "wizard not initialized"})
		return
	}
	var hw HardwareConfig
	if err := c.ShouldBindJSON(&hw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.wizard.SetHardwareConfig(hw)
	c.JSON(http.StatusOK, gin.H{
		"message":      "hardware config set",
		"recommendation": h.wizard.GetRecommendation(),
	})
}

// GetRAIDRecommendation 获取 RAID 推荐
func (h *Handler) GetRAIDRecommendation(c *gin.Context) {
	if h.wizard == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "wizard not initialized"})
		return
	}
	rec := h.wizard.GetRecommendation()
	if rec == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no recommendation available, set hardware config first"})
		return
	}
	c.JSON(http.StatusOK, rec)
}

// DetectAutoSkip 执行自动检测并跳过已完成步骤
func (h *Handler) DetectAutoSkip(c *gin.Context) {
	if h.wizard == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "wizard not initialized"})
		return
	}
	h.wizard.DetectAndAutoSkip()
	state := h.wizard.GetState()
	c.JSON(http.StatusOK, gin.H{
		"message": "auto detection complete",
		"state":   state,
	})
}

// ============================================================
// 辅助函数
// ============================================================

func safeRate(completed, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(completed) / float64(total) * 100
}
