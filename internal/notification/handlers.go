// Package notification 提供通知中心 API
// Version: v2.49.0
package notification

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers API 处理器
type Handlers struct {
	service *Service
}

// NewHandlers 创建处理器
func NewHandlers(service *Service) *Handlers {
	return &Handlers{
		service: service,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	notify := r.Group("/notification")
	{
		// 通知发送
		notify.POST("/send", h.sendNotification)
		notify.POST("/send-async", h.sendAsyncNotification)
		notify.GET("/stats", h.getStats)

		// 渠道管理
		channels := notify.Group("/channels")
		{
			channels.GET("", h.listChannels)
			channels.POST("", h.createChannel)
			channels.GET("/:id", h.getChannel)
			channels.PUT("/:id", h.updateChannel)
			channels.DELETE("/:id", h.deleteChannel)
			channels.POST("/:id/test", h.testChannel)
		}

		// 模板管理
		templates := notify.Group("/templates")
		{
			templates.GET("", h.listTemplates)
			templates.POST("", h.createTemplate)
			templates.GET("/:id", h.getTemplate)
			templates.PUT("/:id", h.updateTemplate)
			templates.DELETE("/:id", h.deleteTemplate)
			templates.POST("/:id/render", h.renderTemplate)
		}

		// 规则管理
		rules := notify.Group("/rules")
		{
			rules.GET("", h.listRules)
			rules.POST("", h.createRule)
			rules.GET("/:id", h.getRule)
			rules.PUT("/:id", h.updateRule)
			rules.DELETE("/:id", h.deleteRule)
			rules.POST("/:id/enable", h.enableRule)
			rules.POST("/:id/disable", h.disableRule)
			rules.POST("/:id/test", h.testRule)
		}

		// 历史记录
		history := notify.Group("/history")
		{
			history.GET("", h.listHistory)
			history.GET("/:id", h.getHistoryRecord)
			history.DELETE("/:id", h.deleteHistoryRecord)
			history.POST("/:id/retry", h.retryRecord)
			history.DELETE("", h.clearHistory)
			history.GET("/stats", h.getHistoryStats)
		}
	}
}

// ========== 通知发送 ==========

// sendNotification 发送通知
// @Summary 发送通知
// @Description 发送通知到指定或匹配的渠道
// @Tags notification
// @Accept json
// @Produce json
// @Param request body SendRequest true "发送请求"
// @Success 200 {object} SendResponse "成功"
// @Router /notification/send [post]
// @Security BearerAuth
func (h *Handlers) sendNotification(c *gin.Context) {
	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	resp, err := h.service.Send(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    resp,
	})
}

// sendAsyncNotification 异步发送通知
// @Summary 异步发送通知
// @Description 异步发送通知，不等待结果
// @Tags notification
// @Accept json
// @Produce json
// @Param request body SendRequest true "发送请求"
// @Success 200 {object} map[string]string "成功"
// @Router /notification/send-async [post]
// @Security BearerAuth
func (h *Handlers) sendAsyncNotification(c *gin.Context) {
	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	notificationID, err := h.service.SendAsync(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "通知已提交",
		"data": gin.H{
			"notificationId": notificationID,
		},
	})
}

// getStats 获取统计
// @Summary 获取统计
// @Description 获取通知发送统计信息
// @Tags notification
// @Produce json
// @Param start query string false "开始时间"
// @Param end query string false "结束时间"
// @Success 200 {object} HistoryStats "成功"
// @Router /notification/stats [get]
// @Security BearerAuth
func (h *Handlers) getStats(c *gin.Context) {
	var startTime, endTime *time.Time

	if start := c.Query("start"); start != "" {
		t, err := time.Parse(time.RFC3339, start)
		if err == nil {
			startTime = &t
		}
	}

	if end := c.Query("end"); end != "" {
		t, err := time.Parse(time.RFC3339, end)
		if err == nil {
			endTime = &t
		}
	}

	stats := h.service.GetStats(startTime, endTime)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// ========== 渠道管理 ==========

// listChannels 列出渠道
// @Summary 列出渠道
// @Description 列出所有通知渠道
// @Tags notification/channels
// @Produce json
// @Param type query string false "渠道类型"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/channels [get]
// @Security BearerAuth
func (h *Handlers) listChannels(c *gin.Context) {
	channelType := ChannelType(c.Query("type"))
	channels := h.service.GetChannelManager().ListChannels(channelType)

	// 隐藏敏感信息
	result := make([]gin.H, 0, len(channels))
	for _, ch := range channels {
		safeConfig := make(map[string]interface{})
		for k, v := range ch.Config {
			if k == "password" || k == "secret" || k == "botToken" {
				safeConfig[k] = "******"
			} else {
				safeConfig[k] = v
			}
		}
		result = append(result, gin.H{
			"id":          ch.ID,
			"name":        ch.Name,
			"type":        ch.Type,
			"enabled":     ch.Enabled,
			"config":      safeConfig,
			"description": ch.Description,
			"createdAt":   ch.CreatedAt,
			"updatedAt":   ch.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// createChannel 创建渠道
// @Summary 创建渠道
// @Description 创建新的通知渠道
// @Tags notification/channels
// @Accept json
// @Produce json
// @Param request body ChannelConfig true "渠道配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/channels [post]
// @Security BearerAuth
func (h *Handlers) createChannel(c *gin.Context) {
	var config ChannelConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.service.AddChannel(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "渠道已创建",
		"data":    config,
	})
}

// getChannel 获取渠道
// @Summary 获取渠道
// @Description 获取指定渠道的详细信息
// @Tags notification/channels
// @Produce json
// @Param id path string true "渠道ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/channels/{id} [get]
// @Security BearerAuth
func (h *Handlers) getChannel(c *gin.Context) {
	id := c.Param("id")

	channel, err := h.service.GetChannelManager().GetChannel(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    channel,
	})
}

// updateChannel 更新渠道
// @Summary 更新渠道
// @Description 更新指定渠道的配置
// @Tags notification/channels
// @Accept json
// @Produce json
// @Param id path string true "渠道ID"
// @Param request body ChannelConfig true "渠道配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/channels/{id} [put]
// @Security BearerAuth
func (h *Handlers) updateChannel(c *gin.Context) {
	id := c.Param("id")

	var config ChannelConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	config.ID = id
	if err := h.service.UpdateChannel(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "渠道已更新",
		"data":    config,
	})
}

// deleteChannel 删除渠道
// @Summary 删除渠道
// @Description 删除指定渠道
// @Tags notification/channels
// @Produce json
// @Param id path string true "渠道ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/channels/{id} [delete]
// @Security BearerAuth
func (h *Handlers) deleteChannel(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.RemoveChannel(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "渠道已删除",
	})
}

// testChannel 测试渠道
// @Summary 测试渠道
// @Description 发送测试通知到指定渠道
// @Tags notification/channels
// @Produce json
// @Param id path string true "渠道ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/channels/{id}/test [post]
// @Security BearerAuth
func (h *Handlers) testChannel(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.TestChannel(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "测试失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "测试通知已发送",
	})
}

// ========== 模板管理 ==========

// listTemplates 列出模板
// @Summary 列出模板
// @Description 列出所有通知模板
// @Tags notification/templates
// @Produce json
// @Param category query string false "模板类别"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/templates [get]
// @Security BearerAuth
func (h *Handlers) listTemplates(c *gin.Context) {
	category := c.Query("category")
	templates := h.service.GetTemplateManager().List(category)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    templates,
	})
}

// createTemplate 创建模板
// @Summary 创建模板
// @Description 创建新的通知模板
// @Tags notification/templates
// @Accept json
// @Produce json
// @Param request body Template true "模板配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/templates [post]
// @Security BearerAuth
func (h *Handlers) createTemplate(c *gin.Context) {
	var template Template
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.service.GetTemplateManager().Create(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "模板已创建",
		"data":    template,
	})
}

// getTemplate 获取模板
// @Summary 获取模板
// @Description 获取指定模板的详细信息
// @Tags notification/templates
// @Produce json
// @Param id path string true "模板ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/templates/{id} [get]
// @Security BearerAuth
func (h *Handlers) getTemplate(c *gin.Context) {
	id := c.Param("id")

	template, err := h.service.GetTemplateManager().Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    template,
	})
}

// updateTemplate 更新模板
// @Summary 更新模板
// @Description 更新指定模板的配置
// @Tags notification/templates
// @Accept json
// @Produce json
// @Param id path string true "模板ID"
// @Param request body Template true "模板配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/templates/{id} [put]
// @Security BearerAuth
func (h *Handlers) updateTemplate(c *gin.Context) {
	id := c.Param("id")

	var template Template
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	template.ID = id
	if err := h.service.GetTemplateManager().Update(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "模板已更新",
		"data":    template,
	})
}

// deleteTemplate 删除模板
// @Summary 删除模板
// @Description 删除指定模板
// @Tags notification/templates
// @Produce json
// @Param id path string true "模板ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/templates/{id} [delete]
// @Security BearerAuth
func (h *Handlers) deleteTemplate(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.GetTemplateManager().Delete(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "模板已删除",
	})
}

// renderTemplate 渲染模板
// @Summary 渲染模板
// @Description 使用指定变量渲染模板
// @Tags notification/templates
// @Accept json
// @Produce json
// @Param id path string true "模板ID"
// @Param request body map[string]interface{} true "变量"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/templates/{id}/render [post]
// @Security BearerAuth
func (h *Handlers) renderTemplate(c *gin.Context) {
	id := c.Param("id")

	var variables map[string]interface{}
	if err := c.ShouldBindJSON(&variables); err != nil {
		variables = make(map[string]interface{})
	}

	rendered, err := h.service.GetTemplateManager().Render(id, variables)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"subject": rendered.Subject,
			"body":    rendered.Body,
		},
	})
}

// ========== 规则管理 ==========

// listRules 列出规则
// @Summary 列出规则
// @Description 列出所有通知规则
// @Tags notification/rules
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/rules [get]
// @Security BearerAuth
func (h *Handlers) listRules(c *gin.Context) {
	rules := h.service.GetRuleEngine().ListRules()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rules,
	})
}

// createRule 创建规则
// @Summary 创建规则
// @Description 创建新的通知规则
// @Tags notification/rules
// @Accept json
// @Produce json
// @Param request body Rule true "规则配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/rules [post]
// @Security BearerAuth
func (h *Handlers) createRule(c *gin.Context) {
	var rule Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.service.GetRuleEngine().CreateRule(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则已创建",
		"data":    rule,
	})
}

// getRule 获取规则
// @Summary 获取规则
// @Description 获取指定规则的详细信息
// @Tags notification/rules
// @Produce json
// @Param id path string true "规则ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/rules/{id} [get]
// @Security BearerAuth
func (h *Handlers) getRule(c *gin.Context) {
	id := c.Param("id")

	rule, err := h.service.GetRuleEngine().GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rule,
	})
}

// updateRule 更新规则
// @Summary 更新规则
// @Description 更新指定规则的配置
// @Tags notification/rules
// @Accept json
// @Produce json
// @Param id path string true "规则ID"
// @Param request body Rule true "规则配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/rules/{id} [put]
// @Security BearerAuth
func (h *Handlers) updateRule(c *gin.Context) {
	id := c.Param("id")

	var rule Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	rule.ID = id
	if err := h.service.GetRuleEngine().UpdateRule(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则已更新",
		"data":    rule,
	})
}

// deleteRule 删除规则
// @Summary 删除规则
// @Description 删除指定规则
// @Tags notification/rules
// @Produce json
// @Param id path string true "规则ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/rules/{id} [delete]
// @Security BearerAuth
func (h *Handlers) deleteRule(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.GetRuleEngine().DeleteRule(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则已删除",
	})
}

// enableRule 启用规则
// @Summary 启用规则
// @Description 启用指定规则
// @Tags notification/rules
// @Produce json
// @Param id path string true "规则ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/rules/{id}/enable [post]
// @Security BearerAuth
func (h *Handlers) enableRule(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.GetRuleEngine().EnableRule(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则已启用",
	})
}

// disableRule 禁用规则
// @Summary 禁用规则
// @Description 禁用指定规则
// @Tags notification/rules
// @Produce json
// @Param id path string true "规则ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/rules/{id}/disable [post]
// @Security BearerAuth
func (h *Handlers) disableRule(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.GetRuleEngine().DisableRule(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则已禁用",
	})
}

// testRule 测试规则
// @Summary 测试规则
// @Description 测试规则匹配
// @Tags notification/rules
// @Accept json
// @Produce json
// @Param id path string true "规则ID"
// @Param request body Notification true "测试通知"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/rules/{id}/test [post]
// @Security BearerAuth
func (h *Handlers) testRule(c *gin.Context) {
	id := c.Param("id")

	rule, err := h.service.GetRuleEngine().GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	var notification Notification
	if err := c.ShouldBindJSON(&notification); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	result := h.service.GetRuleEngine().TestRule(rule, &notification)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// ========== 历史记录 ==========

// listHistory 列出历史记录
// @Summary 列出历史记录
// @Description 列出通知发送历史记录
// @Tags notification/history
// @Produce json
// @Param status query string false "状态过滤"
// @Param channel query string false "渠道过滤"
// @Param level query string false "级别过滤"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/history [get]
// @Security BearerAuth
func (h *Handlers) listHistory(c *gin.Context) {
	filter := &HistoryFilter{
		Status:   NotificationStatus(c.Query("status")),
		Channel:  ChannelType(c.Query("channel")),
		Level:    NotificationLevel(c.Query("level")),
		Category: c.Query("category"),
		Source:   c.Query("source"),
		Search:   c.Query("search"),
	}

	if page := c.Query("page"); page != "" {
		var p int
		if _, err := fmt.Sscanf(page, "%d", &p); err == nil {
			filter.Page = p
		}
	}

	if pageSize := c.Query("pageSize"); pageSize != "" {
		var ps int
		if _, err := fmt.Sscanf(pageSize, "%d", &ps); err == nil {
			filter.PageSize = ps
		}
	}

	records := h.service.GetHistoryManager().Query(filter)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"records": records,
			"total":   h.service.GetHistoryManager().Count(),
		},
	})
}

// getHistoryRecord 获取历史记录
// @Summary 获取历史记录
// @Description 获取指定历史记录的详细信息
// @Tags notification/history
// @Produce json
// @Param id path string true "记录ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/history/{id} [get]
// @Security BearerAuth
func (h *Handlers) getHistoryRecord(c *gin.Context) {
	id := c.Param("id")

	record, err := h.service.GetHistoryManager().GetRecord(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    record,
	})
}

// deleteHistoryRecord 删除历史记录
// @Summary 删除历史记录
// @Description 删除指定历史记录
// @Tags notification/history
// @Produce json
// @Param id path string true "记录ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/history/{id} [delete]
// @Security BearerAuth
func (h *Handlers) deleteHistoryRecord(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.GetHistoryManager().DeleteRecord(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "记录已删除",
	})
}

// retryRecord 重试记录
// @Summary 重试记录
// @Description 重试发送失败的通知
// @Tags notification/history
// @Produce json
// @Param id path string true "记录ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/history/{id}/retry [post]
// @Security BearerAuth
func (h *Handlers) retryRecord(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.RetryFailed(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "重试成功",
	})
}

// clearHistory 清空历史记录
// @Summary 清空历史记录
// @Description 清空所有通知发送历史记录
// @Tags notification/history
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/history [delete]
// @Security BearerAuth
func (h *Handlers) clearHistory(c *gin.Context) {
	if err := h.service.GetHistoryManager().Clear(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "历史记录已清空",
	})
}

// getHistoryStats 获取历史统计
// @Summary 获取历史统计
// @Description 获取通知发送历史统计
// @Tags notification/history
// @Produce json
// @Param start query string false "开始时间"
// @Param end query string false "结束时间"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /notification/history/stats [get]
// @Security BearerAuth
func (h *Handlers) getHistoryStats(c *gin.Context) {
	var startTime, endTime *time.Time

	if start := c.Query("start"); start != "" {
		t, err := time.Parse(time.RFC3339, start)
		if err == nil {
			startTime = &t
		}
	}

	if end := c.Query("end"); end != "" {
		t, err := time.Parse(time.RFC3339, end)
		if err == nil {
			endTime = &t
		}
	}

	stats := h.service.GetHistoryManager().GetStats(startTime, endTime)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}