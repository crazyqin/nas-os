// Package onboardingwizard 提供 HTTP API 处理器
package onboardingwizard

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 引导模块 HTTP 处理器
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建处理器
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	return &Handler{manager: manager, logger: logger}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	wizard := rg.Group("/onboarding")
	{
		// 会话管理
		wizard.POST("/sessions", h.CreateSession)
		wizard.GET("/sessions", h.ListSessions)
		wizard.GET("/sessions/:id", h.GetSession)
		wizard.GET("/sessions/:id/progress", h.GetProgress)

		// 步骤操作
		wizard.POST("/sessions/:id/steps/:stepId/complete", h.CompleteStep)
		wizard.POST("/sessions/:id/steps/:stepId/skip", h.SkipStep)
		wizard.POST("/sessions/:id/steps/:stepId/unskip", h.UnskipStep)

		// 模板
		wizard.GET("/templates", h.GetTemplates)

		// 推荐
		wizard.GET("/recommendations", h.GetRecommendations)
	}
}

// CreateSession 创建引导会话
func (h *Handler) CreateSession(c *gin.Context) {
	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	session, err := h.manager.CreateSession(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, session)
}

// ListSessions 列出所有会话
func (h *Handler) ListSessions(c *gin.Context) {
	sessions := h.manager.ListSessions()
	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"total":    len(sessions),
	})
}

// GetSession 获取会话详情
func (h *Handler) GetSession(c *gin.Context) {
	sessionID := c.Param("id")
	session, err := h.manager.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, session)
}

// GetProgress 获取引导进度
func (h *Handler) GetProgress(c *gin.Context) {
	sessionID := c.Param("id")
	progress, err := h.manager.GetProgress(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, progress)
}

// CompleteStep 完成步骤
func (h *Handler) CompleteStep(c *gin.Context) {
	sessionID := c.Param("id")
	stepID := c.Param("stepId")

	var req CompleteStepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Data = nil
	}

	session, err := h.manager.CompleteStep(sessionID, stepID, req.Data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
}

// SkipStep 跳过步骤
func (h *Handler) SkipStep(c *gin.Context) {
	sessionID := c.Param("id")
	stepID := c.Param("stepId")

	session, err := h.manager.SkipStep(sessionID, stepID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
}

// UnskipStep 取消跳过步骤
func (h *Handler) UnskipStep(c *gin.Context) {
	sessionID := c.Param("id")
	stepID := c.Param("stepId")

	session, err := h.manager.UnskipStep(sessionID, stepID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
}

// GetTemplates 获取配置模板
func (h *Handler) GetTemplates(c *gin.Context) {
	templates := h.manager.GetTemplates()
	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"total":     len(templates),
	})
}

// GetRecommendations 获取功能推荐
func (h *Handler) GetRecommendations(c *gin.Context) {
	scenario := c.Query("scenario")
	if scenario == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'scenario' is required"})
		return
	}

	apps := h.manager.GetRecommendations(scenario)
	c.JSON(http.StatusOK, gin.H{
		"apps":  apps,
		"total": len(apps),
	})
}
