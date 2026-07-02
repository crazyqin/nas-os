// Package wellbeing 提供 REST API 处理器
package wellbeing

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 数字健康 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	wb := r.Group("/wellbeing")
	{
		// 使用追踪
		wb.POST("/sessions", h.startTracking)
		wb.POST("/sessions/:id/pause", h.pauseTracking)
		wb.POST("/sessions/:id/resume", h.resumeTracking)
		wb.POST("/sessions/:id/end", h.endTracking)
		wb.GET("/sessions/active", h.getActiveSessions)
		wb.GET("/sessions/history", h.getSessionHistory)

		// 休息提醒
		wb.GET("/reminders", h.listReminders)
		wb.POST("/reminders", h.createReminder)
		wb.GET("/reminders/:id", h.getReminder)
		wb.PUT("/reminders/:id", h.updateReminder)
		wb.DELETE("/reminders/:id", h.deleteReminder)
		wb.POST("/reminders/:id/trigger", h.triggerReminder)
		wb.POST("/reminders/:id/snooze", h.snoozeReminder)
		wb.POST("/reminders/:id/dismiss", h.dismissReminder)

		// 使用限制
		wb.GET("/limits", h.getUsageLimits)
		wb.POST("/limits", h.setUsageLimit)
		wb.PUT("/limits/:id", h.updateUsageLimit)
		wb.DELETE("/limits/:id", h.deleteUsageLimit)
		wb.GET("/limits/check", h.checkUsageLimit)

		// 屏幕时间
		wb.GET("/screentime", h.getScreenTime)

		// 报告和洞察
		wb.POST("/reports", h.generateReport)
		wb.GET("/insights", h.getInsights)
		wb.POST("/insights/:id/read", h.markInsightRead)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// startTracking 开始追踪.
func (h *Handlers) startTracking(c *gin.Context) {
	var req struct {
		UserID      string      `json:"user_id" binding:"required"`
		SessionType SessionType `json:"type" binding:"required"`
		AppName     string      `json:"app_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	session, err := h.manager.TrackUsage(req.UserID, req.SessionType, req.AppName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "tracking started",
		Data:    session,
	})
}

// pauseTracking 暂停追踪.
func (h *Handlers) pauseTracking(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.PauseTracking(id); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "tracking paused",
	})
}

// resumeTracking 恢复追踪.
func (h *Handlers) resumeTracking(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ResumeTracking(id); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "tracking resumed",
	})
}

// endTracking 结束追踪.
func (h *Handlers) endTracking(c *gin.Context) {
	id := c.Param("id")
	session, err := h.manager.EndTracking(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "tracking ended",
		Data:    session,
	})
}

// getActiveSessions 获取活跃会话.
func (h *Handlers) getActiveSessions(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "user_id is required",
		})
		return
	}

	sessions := h.manager.GetActiveSessions(userID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    sessions,
	})
}

// getSessionHistory 获取会话历史.
func (h *Handlers) getSessionHistory(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "user_id is required",
		})
		return
	}

	sessions := h.manager.GetSessionHistory(userID, 50)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    sessions,
	})
}

// listReminders 列出提醒.
func (h *Handlers) listReminders(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "user_id is required",
		})
		return
	}

	reminders := h.manager.ListReminders(userID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    reminders,
	})
}

// createReminder 创建提醒.
func (h *Handlers) createReminder(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id" binding:"required"`
		CreateReminderRequest
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	reminder, err := h.manager.SetReminder(req.UserID, &req.CreateReminderRequest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "reminder created",
		Data:    reminder,
	})
}

// getReminder 获取提醒.
func (h *Handlers) getReminder(c *gin.Context) {
	id := c.Param("id")
	reminder, err := h.manager.GetReminder(id)
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
		Data:    reminder,
	})
}

// updateReminder 更新提醒.
func (h *Handlers) updateReminder(c *gin.Context) {
	id := c.Param("id")
	var req UpdateReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	reminder, err := h.manager.UpdateReminder(id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "reminder updated",
		Data:    reminder,
	})
}

// deleteReminder 删除提醒.
func (h *Handlers) deleteReminder(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteReminder(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "reminder deleted",
	})
}

// triggerReminder 触发提醒.
func (h *Handlers) triggerReminder(c *gin.Context) {
	id := c.Param("id")
	reminder, err := h.manager.TriggerReminder(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "reminder triggered",
		Data:    reminder,
	})
}

// snoozeReminder 贪睡提醒.
func (h *Handlers) snoozeReminder(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.SnoozeReminder(id); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "reminder snoozed",
	})
}

// dismissReminder 忽略提醒.
func (h *Handlers) dismissReminder(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DismissReminder(id); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "reminder dismissed",
	})
}

// getUsageLimits 获取使用限制.
func (h *Handlers) getUsageLimits(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "user_id is required",
		})
		return
	}

	limits := h.manager.GetUsageLimits(userID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    limits,
	})
}

// setUsageLimit 设置使用限制.
func (h *Handlers) setUsageLimit(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id" binding:"required"`
		CreateUsageLimitRequest
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	limit, err := h.manager.SetUsageLimit(req.UserID, &req.CreateUsageLimitRequest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "usage limit set",
		Data:    limit,
	})
}

// updateUsageLimit 更新使用限制.
func (h *Handlers) updateUsageLimit(c *gin.Context) {
	id := c.Param("id")
	var req UpdateUsageLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	limit, err := h.manager.UpdateUsageLimit(id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "usage limit updated",
		Data:    limit,
	})
}

// deleteUsageLimit 删除使用限制.
func (h *Handlers) deleteUsageLimit(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteUsageLimit(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "usage limit deleted",
	})
}

// checkUsageLimit 检查使用限制.
func (h *Handlers) checkUsageLimit(c *gin.Context) {
	userID := c.Query("user_id")
	appName := c.Query("app_name")

	if userID == "" || appName == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "user_id and app_name are required",
		})
		return
	}

	isLimited, status, err := h.manager.CheckUsageLimit(userID, appName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"is_limited": isLimited,
			"status":     status,
		},
	})
}

// getScreenTime 获取屏幕时间.
func (h *Handlers) getScreenTime(c *gin.Context) {
	userID := c.Query("user_id")
	date := c.Query("date")

	if userID == "" || date == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "user_id and date are required",
		})
		return
	}

	screenTime, err := h.manager.GetScreenTime(userID, date)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    screenTime,
	})
}

// generateReport 生成报告.
func (h *Handlers) generateReport(c *gin.Context) {
	var req ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	report, err := h.manager.GenerateReport(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "report generated",
		Data:    report,
	})
}

// getInsights 获取洞察.
func (h *Handlers) getInsights(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "user_id is required",
		})
		return
	}

	insights := h.manager.GetInsights(userID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    insights,
	})
}

// markInsightRead 标记洞察已读.
func (h *Handlers) markInsightRead(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.MarkInsightRead(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "insight marked as read",
	})
}
