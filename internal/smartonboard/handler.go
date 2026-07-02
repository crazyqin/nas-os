package smartonboard

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 智能引导HTTP处理器.
type Handler struct {
	ob *SmartOnboard
}

// NewHandler 创建处理器.
func NewHandler(ob *SmartOnboard) *Handler {
	return &Handler{ob: ob}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	group := rg.Group("/smartonboard")
	{
		group.GET("/profiles", h.GetProfiles)
		group.GET("/health", h.GetHealth)
		group.GET("/issues", h.GetIssues)
		group.POST("/profile", h.CreateProfile)
		group.POST("/complete", h.CompleteStep)
		group.POST("/skip", h.SkipStep)
		group.POST("/check", h.CheckHealth)
	}
}

// GetProfiles 获取引导配置.
func (h *Handler) GetProfiles(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": h.ob.GetProfiles()})
}

// GetHealth 获取健康状态.
func (h *Handler) GetHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": h.ob.GetHealth()})
}

// GetIssues 获取问题列表.
func (h *Handler) GetIssues(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": h.ob.GetIssues()})
}

// CreateProfile 创建引导配置.
func (h *Handler) CreateProfile(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	profile := h.ob.CreateProfile(req.Name)
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": profile})
}

// CompleteStep 完成步骤.
func (h *Handler) CompleteStep(c *gin.Context) {
	var req struct {
		ProfileID string `json:"profileId" binding:"required"`
		StepID    string `json:"stepId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.ob.CompleteStep(req.ProfileID, req.StepID) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile or step not found"})
	}
}

// SkipStep 跳过步骤.
func (h *Handler) SkipStep(c *gin.Context) {
	var req struct {
		ProfileID string `json:"profileId" binding:"required"`
		StepID    string `json:"stepId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.ob.SkipStep(req.ProfileID, req.StepID) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile or step not found"})
	}
}

// CheckHealth 执行健康检查.
func (h *Handler) CheckHealth(c *gin.Context) {
	health := h.ob.CheckHealth()
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": health})
}
