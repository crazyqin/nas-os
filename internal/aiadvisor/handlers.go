// Package aiadvisor HTTP API handlers
package aiadvisor

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers AI 智能顾问 HTTP 处理器.
type Handlers struct {
	advisor *Advisor
	logger  *zap.Logger
}

// NewHandlers 创建 HTTP 处理器.
func NewHandlers(advisor *Advisor, logger *zap.Logger) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handlers{
		advisor: advisor,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由到 /api/v1 组.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	aiGroup := api.Group("/ai/advisor")
	{
		aiGroup.POST("/chat", h.handleChat)
		aiGroup.GET("/suggestions", h.handleSuggestions)
		aiGroup.POST("/diagnose", h.handleDiagnose)
		aiGroup.GET("/system-context", h.handleSystemContext)
		aiGroup.DELETE("/session/:session_id", h.handleClearSession)
		aiGroup.GET("/health", h.handleHealth)
	}
}

// handleChat 聊天接口.
// POST /api/v1/ai/advisor/chat
func (h *Handlers) handleChat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	resp, err := h.advisor.Chat(c.Request.Context(), &req)
	if err != nil {
		code := http.StatusInternalServerError
		if err == ErrEmptyMessage {
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleSuggestions 获取推荐接口.
// GET /api/v1/ai/advisor/suggestions
func (h *Handlers) handleSuggestions(c *gin.Context) {
	suggestions, err := h.advisor.GetSuggestions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"suggestions": suggestions,
		"total":       len(suggestions),
	})
}

// handleDiagnose 故障诊断接口.
// POST /api/v1/ai/advisor/diagnose
func (h *Handlers) handleDiagnose(c *gin.Context) {
	var req DiagnoseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	resp, err := h.advisor.Diagnose(c.Request.Context(), &req)
	if err != nil {
		code := http.StatusInternalServerError
		if err == ErrEmptyDiagnosis {
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleSystemContext 获取系统上下文接口.
// GET /api/v1/ai/advisor/system-context
func (h *Handlers) handleSystemContext(c *gin.Context) {
	sysCtx := h.advisor.GetSystemContext()
	c.JSON(http.StatusOK, sysCtx)
}

// handleClearSession 清除会话历史.
// DELETE /api/v1/ai/advisor/session/:session_id
func (h *Handlers) handleClearSession(c *gin.Context) {
	sessionID := c.Param("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id 不能为空"})
		return
	}
	h.advisor.ClearSession(sessionID)
	c.JSON(http.StatusOK, gin.H{"message": "会话已清除", "session_id": sessionID})
}

// handleHealth 健康检查接口.
// GET /api/v1/ai/advisor/health
func (h *Handlers) handleHealth(c *gin.Context) {
	err := h.advisor.HealthCheck(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unavailable",
			"error":  err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
