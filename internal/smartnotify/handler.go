// Package smartnotify 提供 REST API 处理器
package smartnotify

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 智能通知 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	notify := r.Group("/smartnotify")
	{
		// 通知发送
		notify.POST("/send", h.sendNotification)
		notify.POST("/send/batch", h.sendBatchNotifications)

		// 通知查询
		notify.GET("/notifications", h.listNotifications)
		notify.GET("/notifications/:id", h.getNotification)

		// 规则管理
		notify.GET("/rules", h.listRules)
		notify.POST("/rules", h.createRule)
		notify.GET("/rules/:id", h.getRule)
		notify.PUT("/rules/:id", h.updateRule)
		notify.DELETE("/rules/:id", h.deleteRule)
		notify.POST("/rules/:id/toggle", h.toggleRule)

		// 模板管理
		notify.GET("/templates", h.listTemplates)
		notify.POST("/templates", h.createTemplate)
		notify.GET("/templates/:id", h.getTemplate)
		notify.PUT("/templates/:id", h.updateTemplate)
		notify.DELETE("/templates/:id", h.deleteTemplate)
		notify.POST("/templates/:id/render", h.renderTemplate)

		// 历史和统计
		notify.GET("/history", h.getHistory)
		notify.GET("/stats", h.getStats)

		// 配置
		notify.GET("/config", h.getConfig)
		notify.PUT("/config", h.updateConfig)

		// 渠道
		notify.GET("/channels", h.getChannels)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// sendNotification 发送通知.
func (h *Handlers) sendNotification(c *gin.Context) {
	var notify Notification
	if err := c.ShouldBindJSON(&notify); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.SendNotification(&notify); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "notification sent",
		Data:    notify,
	})
}

// sendBatchNotifications 批量发送通知.
func (h *Handlers) sendBatchNotifications(c *gin.Context) {
	var notifications []Notification
	if err := c.ShouldBindJSON(&notifications); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if len(notifications) > 50 {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "batch size exceeds maximum (50)",
		})
		return
	}

	results := make([]Notification, 0, len(notifications))
	for _, notify := range notifications {
		n := notify
		if err := h.manager.SendNotification(&n); err != nil {
			n.Status = StatusFailed
		}
		results = append(results, n)
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "batch sent",
		Data:    results,
	})
}

// listNotifications 列出通知.
func (h *Handlers) listNotifications(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	notifications := h.manager.ListNotifications(limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    notifications,
	})
}

// getNotification 获取通知详情.
func (h *Handlers) getNotification(c *gin.Context) {
	id := c.Param("id")
	notify, err := h.manager.GetNotification(id)
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
		Data:    notify,
	})
}

// listRules 列出规则.
func (h *Handlers) listRules(c *gin.Context) {
	rules := h.manager.ListRules()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    rules,
	})
}

// createRule 创建规则.
func (h *Handlers) createRule(c *gin.Context) {
	var rule NotifyRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.CreateRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, response{
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

// getRule 获取规则.
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

// updateRule 更新规则.
func (h *Handlers) updateRule(c *gin.Context) {
	id := c.Param("id")
	var rule NotifyRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.UpdateRule(id, &rule); err != nil {
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

// deleteRule 删除规则.
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

// toggleRule 切换规则状态.
func (h *Handlers) toggleRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ToggleRule(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "rule toggled",
	})
}

// listTemplates 列出模板.
func (h *Handlers) listTemplates(c *gin.Context) {
	templates := h.manager.ListTemplates()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    templates,
	})
}

// createTemplate 创建模板.
func (h *Handlers) createTemplate(c *gin.Context) {
	var tpl NotifyTemplate
	if err := c.ShouldBindJSON(&tpl); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.CreateTemplate(&tpl); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "template created",
		Data:    tpl,
	})
}

// getTemplate 获取模板.
func (h *Handlers) getTemplate(c *gin.Context) {
	id := c.Param("id")
	tpl, err := h.manager.GetTemplate(id)
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
		Data:    tpl,
	})
}

// updateTemplate 更新模板.
func (h *Handlers) updateTemplate(c *gin.Context) {
	id := c.Param("id")
	var tpl NotifyTemplate
	if err := c.ShouldBindJSON(&tpl); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.UpdateTemplate(id, &tpl); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "template updated",
		Data:    tpl,
	})
}

// deleteTemplate 删除模板.
func (h *Handlers) deleteTemplate(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteTemplate(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "template deleted",
	})
}

// renderTemplate 渲染模板.
func (h *Handlers) renderTemplate(c *gin.Context) {
	id := c.Param("id")
	var vars map[string]string
	if err := c.ShouldBindJSON(&vars); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	title, content, err := h.manager.RenderTemplate(id, vars)
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
		Data: map[string]string{
			"title":   title,
			"content": content,
		},
	})
}

// getHistory 获取历史.
func (h *Handlers) getHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	history := h.manager.GetHistory(limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    history,
	})
}

// getStats 获取统计.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// getConfig 获取配置.
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cfg,
	})
}

// updateConfig 更新配置.
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg SmartNotifyConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	h.manager.UpdateConfig(&cfg)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "config updated",
	})
}

// getChannels 获取支持的渠道.
func (h *Handlers) getChannels(c *gin.Context) {
	channels := ValidChannels()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    channels,
	})
}
