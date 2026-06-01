// Package onboarding 提供新手引导 REST API 处理器
package onboarding

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 新手引导 API 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	onboarding := r.Group("/onboarding")
	{
		// 向导管理
		onboarding.GET("/wizard", h.listWizards)
		onboarding.GET("/wizard/:id", h.getWizard)

		// 步骤管理
		onboarding.POST("/step", h.completeStep)

		// 功能引导
		onboarding.GET("/guides", h.getGuides)
		onboarding.GET("/guides/:id", h.getGuide)

		// 最佳实践推荐
		onboarding.GET("/practices", h.recommendPractice)

		// 进度追踪
		onboarding.GET("/progress", h.getProgress)
		onboarding.DELETE("/progress", h.resetProgress)
	}
}

type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (h *Handler) listWizards(c *gin.Context) {
	wizards := h.manager.ListWizards()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: wizards})
}

func (h *Handler) getWizard(c *gin.Context) {
	id := c.Param("id")
	wizard, err := h.manager.GetWizard(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: wizard})
}

func (h *Handler) completeStep(c *gin.Context) {
	var req CompleteStepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	progress, err := h.manager.CompleteStep(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "step completed", Data: progress})
}

func (h *Handler) getGuides(c *gin.Context) {
	category := c.Query("category")
	guides := h.manager.GetGuides(category)
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: guides})
}

func (h *Handler) getGuide(c *gin.Context) {
	id := c.Param("id")
	guide, err := h.manager.GetGuide(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: guide})
}

func (h *Handler) recommendPractice(c *gin.Context) {
	category := c.Query("category")
	practices := h.manager.RecommendPractice(category)
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: practices})
}

func (h *Handler) getProgress(c *gin.Context) {
	userID := c.Query("user_id")
	wizardID := c.DefaultQuery("wizard_id", "wizard-default")

	if userID == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "user_id is required"})
		return
	}

	progress, err := h.manager.GetProgress(userID, wizardID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: progress})
}

func (h *Handler) resetProgress(c *gin.Context) {
	userID := c.Query("user_id")
	wizardID := c.DefaultQuery("wizard_id", "wizard-default")

	if userID == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "user_id is required"})
		return
	}

	if err := h.manager.ResetProgress(userID, wizardID); err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "progress reset"})
}
