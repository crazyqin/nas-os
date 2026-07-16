// Package emailserver 提供 REST API 处理器
package emailserver

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 邮件服务器模块 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/email 路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	email := r.Group("/email")
	{
		// 邮件账户管理
		email.GET("/accounts", h.listAccounts)
		email.POST("/accounts", h.createAccount)
		email.GET("/accounts/:id", h.getAccount)
		email.PUT("/accounts/:id", h.updateAccount)
		email.DELETE("/accounts/:id", h.deleteAccount)

		// 邮件操作
		email.GET("/messages", h.listMessages)
		email.POST("/send", h.sendEmail)
		email.PUT("/messages/:id/read", h.markAsRead)
		email.PUT("/messages/:id/star", h.toggleStar)
		email.PUT("/messages/:id/move", h.moveMessage)
		email.DELETE("/messages/:id", h.deleteMessage)

		// SMTP/IMAP 配置
		email.GET("/settings/smtp", h.getSMTPConfig)
		email.PUT("/settings/smtp", h.updateSMTPConfig)
		email.GET("/settings/imap", h.getIMAPConfig)
		email.PUT("/settings/imap", h.updateIMAPConfig)

		// 过滤规则
		email.GET("/rules", h.listFilterRules)
		email.POST("/rules", h.createFilterRule)
		email.DELETE("/rules/:id", h.deleteFilterRule)

		// 反垃圾邮件配置
		email.GET("/antispam", h.getAntispamConfig)
		email.PUT("/antispam", h.updateAntispamConfig)

		// 统计信息
		email.GET("/stats", h.getStats)
	}
}

// ========== 邮件账户处理 ==========

func (h *Handlers) listAccounts(c *gin.Context) {
	accounts := h.manager.ListAccounts()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(accounts),
			"accounts": accounts,
		},
	})
}

func (h *Handlers) createAccount(c *gin.Context) {
	var req CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	acct := h.manager.CreateAccount(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: acct})
}

func (h *Handlers) getAccount(c *gin.Context) {
	id := c.Param("id")
	acct, err := h.manager.GetAccount(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: acct})
}

func (h *Handlers) updateAccount(c *gin.Context) {
	id := c.Param("id")
	var req UpdateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	acct, err := h.manager.UpdateAccount(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: acct})
}

func (h *Handlers) deleteAccount(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteAccount(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== 邮件操作处理 ==========

func (h *Handlers) listMessages(c *gin.Context) {
	accountID := c.Query("account_id")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "account_id is required"})
		return
	}

	folder := c.Query("folder")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	messages, total := h.manager.ListMessages(accountID, folder, page, pageSize)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":     total,
			"page":      page,
			"page_size": pageSize,
			"messages":  messages,
		},
	})
}

func (h *Handlers) sendEmail(c *gin.Context) {
	var req SendEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	// 检查是否为垃圾邮件发送者
	if h.manager.IsSpam(req.From) {
		c.JSON(http.StatusForbidden, response{Code: 2, Message: "sender is blacklisted"})
		return
	}

	msg, err := h.manager.SendEmail(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "sent", Data: msg})
}

func (h *Handlers) markAsRead(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.MarkAsRead(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "marked as read"})
}

func (h *Handlers) toggleStar(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ToggleStar(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "star toggled"})
}

func (h *Handlers) moveMessage(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Folder string `json:"folder" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.manager.MoveMessage(id, req.Folder); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "moved"})
}

func (h *Handlers) deleteMessage(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteMessage(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== SMTP/IMAP 配置处理 ==========

func (h *Handlers) getSMTPConfig(c *gin.Context) {
	cfg := h.manager.GetSMTPConfig()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: cfg})
}

func (h *Handlers) updateSMTPConfig(c *gin.Context) {
	var req UpdateSMTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	h.manager.UpdateSMTPConfig(req)
	c.JSON(http.StatusOK, response{Code: 0, Message: "updated"})
}

func (h *Handlers) getIMAPConfig(c *gin.Context) {
	cfg := h.manager.GetIMAPConfig()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: cfg})
}

func (h *Handlers) updateIMAPConfig(c *gin.Context) {
	var req UpdateIMAPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	h.manager.UpdateIMAPConfig(req)
	c.JSON(http.StatusOK, response{Code: 0, Message: "updated"})
}

// ========== 过滤规则处理 ==========

func (h *Handlers) listFilterRules(c *gin.Context) {
	rules := h.manager.ListFilterRules()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(rules),
			"rules": rules,
		},
	})
}

func (h *Handlers) createFilterRule(c *gin.Context) {
	var req CreateFilterRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	rule := h.manager.CreateFilterRule(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: rule})
}

func (h *Handlers) deleteFilterRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteFilterRule(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== 反垃圾邮件配置处理 ==========

func (h *Handlers) getAntispamConfig(c *gin.Context) {
	cfg := h.manager.GetAntispamConfig()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: cfg})
}

func (h *Handlers) updateAntispamConfig(c *gin.Context) {
	var req UpdateAntispamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	h.manager.UpdateAntispamConfig(req)
	c.JSON(http.StatusOK, response{Code: 0, Message: "updated"})
}

// ========== 统计信息 ==========

func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}
