// Package remotesupport 提供远程支持隧道功能
package remotesupport

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 远程支持 HTTP 处理器.
type Handlers struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager, logger *zap.Logger) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handlers{
		manager: mgr,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	rs := api.Group("/remotesupport")
	{
		// 会话管理
		rs.POST("/sessions", h.createSession)
		rs.GET("/sessions", h.listSessions)
		rs.GET("/sessions/:id", h.getSession)
		rs.PUT("/sessions/:id", h.updateSession)
		rs.POST("/sessions/:id/close", h.closeSession)

		// 令牌验证
		rs.POST("/tokens/validate", h.validateToken)

		// 隧道管理
		rs.POST("/sessions/:id/tunnel", h.establishTunnel)
		rs.GET("/sessions/:id/tunnel", h.getTunnel)
		rs.DELETE("/sessions/:id/tunnel", h.closeTunnel)

		// 传输记录
		rs.POST("/sessions/:id/transfer", h.recordTransfer)

		// 审计日志
		rs.GET("/sessions/:id/audit", h.getAuditLog)
		rs.POST("/sessions/:id/audit", h.addAuditEntry)

		// 统计
		rs.GET("/stats", h.getStats)
	}
}

// ========== 通用响应 ==========

// Response 通用 API 响应结构.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 返回成功响应.
func Success(data interface{}) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

// Error 返回错误响应.
func Error(code int, message string) Response {
	return Response{Code: code, Message: message}
}

// ========== 会话 API ==========

// createSession 创建会话.
func (h *Handlers) createSession(c *gin.Context) {
	var req SessionCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	session, token, err := h.manager.CreateSession(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, Success(gin.H{
		"session": session,
		"token":   token.Token,
	}))
}

// listSessions 列出会话.
func (h *Handlers) listSessions(c *gin.Context) {
	sessions := h.manager.ListSessions()
	c.JSON(http.StatusOK, Success(sessions))
}

// getSession 获取会话.
func (h *Handlers) getSession(c *gin.Context) {
	id := c.Param("id")

	session, err := h.manager.GetSession(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(session))
}

// updateSession 更新会话.
func (h *Handlers) updateSession(c *gin.Context) {
	id := c.Param("id")

	var req SessionUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	session, err := h.manager.UpdateSession(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(session))
}

// closeSession 关闭会话.
func (h *Handlers) closeSession(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.CloseSession(id); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

// ========== 令牌 API ==========

// validateToken 验证令牌.
func (h *Handlers) validateToken(c *gin.Context) {
	var req TokenValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	// 获取客户端 IP
	if req.ClientIP == "" {
		req.ClientIP = c.ClientIP()
	}

	session, err := h.manager.ValidateToken(req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, Error(401, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(session))
}

// ========== 隧道 API ==========

// establishTunnel 建立隧道.
func (h *Handlers) establishTunnel(c *gin.Context) {
	id := c.Param("id")

	tunnel, err := h.manager.EstablishTunnel(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, Success(tunnel))
}

// getTunnel 获取隧道.
func (h *Handlers) getTunnel(c *gin.Context) {
	id := c.Param("id")

	tunnel, err := h.manager.GetTunnel(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(tunnel))
}

// closeTunnel 关闭隧道.
func (h *Handlers) closeTunnel(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.CloseTunnel(id); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

// ========== 传输 API ==========

// recordTransfer 记录传输.
func (h *Handlers) recordTransfer(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		BytesUp   int64 `json:"bytes_up"`
		BytesDown int64 `json:"bytes_down"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.manager.RecordTransfer(id, req.BytesUp, req.BytesDown); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

// ========== 审计 API ==========

// getAuditLog 获取审计日志.
func (h *Handlers) getAuditLog(c *gin.Context) {
	id := c.Param("id")

	log, err := h.manager.GetAuditLog(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(log))
}

// addAuditEntry 添加审计日志.
func (h *Handlers) addAuditEntry(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Action string `json:"action" binding:"required"`
		Detail string `json:"detail"`
		Source string `json:"source"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.manager.AddAuditEntry(id, req.Action, req.Detail, req.Source); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, Success(nil))
}

// ========== 统计 API ==========

// getStats 获取统计.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, Success(stats))
}
