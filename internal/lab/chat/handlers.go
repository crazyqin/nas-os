// Package chat 提供 REST API 处理器
package chat

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

// Handlers 即时通讯模块 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/chat 路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	chat := r.Group("/chat")
	{
		// 频道 CRUD
		chat.POST("/channels", h.createChannel)
		chat.GET("/channels", h.listChannels)
		chat.GET("/channels/:id", h.getChannel)
		chat.PUT("/channels/:id", h.updateChannel)
		chat.DELETE("/channels/:id", h.deleteChannel)

		// 消息管理
		chat.POST("/channels/:id/messages", h.sendMessage)
		chat.GET("/channels/:id/messages", h.getMessages)
		chat.PUT("/messages/:id", h.editMessage)
		chat.DELETE("/messages/:id", h.deleteMessage)

		// 成员管理
		chat.POST("/channels/:id/members", h.addMember)
		chat.DELETE("/channels/:id/members/:uid", h.removeMember)
		chat.GET("/channels/:id/members", h.listMembers)
		chat.PUT("/channels/:id/members/:uid", h.updateMemberRole)

		// 反应
		chat.POST("/messages/:id/reactions", h.addReaction)
		chat.DELETE("/messages/:id/reactions", h.removeReaction)

		// 搜索
		chat.GET("/search", h.searchMessages)

		// 未读计数
		chat.GET("/unread", h.getUnreadCount)

		// 已读标记
		chat.POST("/channels/:id/read", h.markAsRead)
	}
}

// ========== Channel Handlers ==========

func (h *Handlers) createChannel(c *gin.Context) {
	var req CreateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	ch := h.manager.CreateChannel(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: ch})
}

func (h *Handlers) getChannel(c *gin.Context) {
	id := c.Param("id")
	ch, err := h.manager.GetChannel(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: ch})
}

func (h *Handlers) listChannels(c *gin.Context) {
	userID := c.Query("user")
	var chs []*Channel
	if userID != "" {
		chs = h.manager.ListChannelsByUser(userID)
	} else {
		chs = h.manager.ListChannels()
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(chs),
			"channels": chs,
		},
	})
}

func (h *Handlers) updateChannel(c *gin.Context) {
	id := c.Param("id")
	var req UpdateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	ch, err := h.manager.UpdateChannel(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: ch})
}

func (h *Handlers) deleteChannel(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteChannel(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== Message Handlers ==========

func (h *Handlers) sendMessage(c *gin.Context) {
	channelID := c.Param("id")
	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	msg, err := h.manager.SendMessage(channelID, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "sent", Data: msg})
}

func (h *Handlers) getMessages(c *gin.Context) {
	channelID := c.Param("id")

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	offsetStr := c.DefaultQuery("offset", "0")
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	msgs, total, err := h.manager.GetMessages(channelID, limit, offset)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    total,
			"limit":    limit,
			"offset":   offset,
			"messages": msgs,
		},
	})
}

func (h *Handlers) editMessage(c *gin.Context) {
	msgID := c.Param("id")
	var req UpdateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	msg, err := h.manager.EditMessage(msgID, *req.Content)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: msg})
}

func (h *Handlers) deleteMessage(c *gin.Context) {
	msgID := c.Param("id")
	if err := h.manager.DeleteMessage(msgID); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== Member Handlers ==========

func (h *Handlers) addMember(c *gin.Context) {
	channelID := c.Param("id")
	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	member, err := h.manager.AddMember(channelID, req)
	if err != nil {
		c.JSON(http.StatusConflict, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "added", Data: member})
}

func (h *Handlers) removeMember(c *gin.Context) {
	channelID := c.Param("id")
	userID := c.Param("uid")

	if err := h.manager.RemoveMember(channelID, userID); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "removed"})
}

func (h *Handlers) listMembers(c *gin.Context) {
	channelID := c.Param("id")
	members, err := h.manager.ListMembers(channelID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(members),
			"members": members,
		},
	})
}

func (h *Handlers) updateMemberRole(c *gin.Context) {
	channelID := c.Param("id")
	userID := c.Param("uid")
	var req UpdateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	member, err := h.manager.UpdateMemberRole(channelID, userID, req.Role)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: member})
}

// ========== Reaction Handlers ==========

func (h *Handlers) addReaction(c *gin.Context) {
	msgID := c.Param("id")
	var req AddReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.manager.AddReaction(msgID, req); err != nil {
		c.JSON(http.StatusConflict, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "added"})
}

func (h *Handlers) removeReaction(c *gin.Context) {
	msgID := c.Param("id")
	var req RemoveReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.manager.RemoveReaction(msgID, req); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "removed"})
}

// ========== Search ==========

func (h *Handlers) searchMessages(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "query parameter 'q' is required"})
		return
	}

	channelID := c.Query("channel")
	msgs := h.manager.SearchMessages(query, channelID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(msgs),
			"query":    query,
			"channel":  channelID,
			"messages": msgs,
		},
	})
}

// ========== Unread / Read ==========

func (h *Handlers) getUnreadCount(c *gin.Context) {
	userID := c.Query("user")
	if userID == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "query parameter 'user' is required"})
		return
	}

	counts := h.manager.GetUnreadCount(userID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"user":   userID,
			"unread": counts,
		},
	})
}

func (h *Handlers) markAsRead(c *gin.Context) {
	channelID := c.Param("id")

	var req struct {
		UserID string `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.manager.MarkAsRead(channelID, req.UserID); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "marked as read"})
}
