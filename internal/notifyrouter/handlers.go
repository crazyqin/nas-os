// Package notifyrouter 提供 REST API 处理器
package notifyrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 通知路由 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	notify := r.Group("/notifyrouter")
	{
		// 通知路由
		notify.POST("/route", h.routeNotification)

		// 规则管理
		notify.GET("/rules", h.listRules)
		notify.POST("/rules", h.setRule)
		notify.GET("/rules/:id", h.getRule)
		notify.PUT("/rules/:id", h.updateRule)
		notify.DELETE("/rules/:id", h.deleteRule)

		// 投递状态
		notify.GET("/deliveries", h.getDeliveryStatus)
		notify.GET("/deliveries/:id", h.getDeliveryByID)
		notify.PUT("/deliveries/:id/status", h.updateDeliveryStatus)

		// 用户偏好
		notify.GET("/preferences/:user_id", h.getUserPreference)
		notify.PUT("/preferences/:user_id", h.setUserPreference)

		// 渠道管理
		notify.GET("/channels/stats", h.getChannelStats)
		notify.POST("/channels/optimize", h.optimizeChannels)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// routeNotification 路由通知
func (h *Handlers) routeNotification(c *gin.Context) {
	var req RouteNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.RouteNotification(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "notification routed",
		Data:    result,
	})
}

// listRules 列出规则
func (h *Handlers) listRules(c *gin.Context) {
	rules := h.manager.GetRules()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    rules,
	})
}

// setRule 创建规则
func (h *Handlers) setRule(c *gin.Context) {
	var req SetRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	rule, err := h.manager.SetRules(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "rule created",
		Data:    rule,
	})
}

// getRule 获取规则
func (h *Handlers) getRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.manager.GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    rule,
	})
}

// updateRule 更新规则
func (h *Handlers) updateRule(c *gin.Context) {
	id := c.Param("id")
	var req SetRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	rule, err := h.manager.UpdateRule(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "rule updated",
		Data:    rule,
	})
}

// deleteRule 删除规则
func (h *Handlers) deleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRule(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "rule deleted",
	})
}

// getDeliveryStatus 获取投递状态
func (h *Handlers) getDeliveryStatus(c *gin.Context) {
	notifyID := c.Query("notify_id")
	deliveryID := c.Query("delivery_id")

	deliveries, err := h.manager.GetDeliveryStatus(notifyID, deliveryID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    deliveries,
	})
}

// getDeliveryByID 获取投递记录
func (h *Handlers) getDeliveryByID(c *gin.Context) {
	id := c.Param("id")
	deliveries, err := h.manager.GetDeliveryStatus("", id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	if len(deliveries) == 0 {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: "delivery not found",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    deliveries[0],
	})
}

// updateDeliveryStatus 更新投递状态
func (h *Handlers) updateDeliveryStatus(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status    DeliveryStatus `json:"status" binding:"required"`
		Error     string         `json:"error,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.UpdateDeliveryStatus(id, req.Status, req.Error); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "delivery status updated",
	})
}

// getUserPreference 获取用户偏好
func (h *Handlers) getUserPreference(c *gin.Context) {
	userID := c.Param("user_id")
	pref, err := h.manager.GetUserPreference(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    pref,
	})
}

// setUserPreference 设置用户偏好
func (h *Handlers) setUserPreference(c *gin.Context) {
	userID := c.Param("user_id")
	var pref UserPreference
	if err := c.ShouldBindJSON(&pref); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	pref.UserID = userID
	if err := h.manager.SetUserPreference(&pref); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "preference updated",
		Data:    pref,
	})
}

// getChannelStats 获取渠道统计
func (h *Handlers) getChannelStats(c *gin.Context) {
	stats := h.manager.GetChannelStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// optimizeChannels 优化渠道
func (h *Handlers) optimizeChannels(c *gin.Context) {
	result := h.manager.OptimizeChannels()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}
