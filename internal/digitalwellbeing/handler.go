// Package digitalwellbeing 提供 REST API 处理器
package digitalwellbeing

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
	wellbeing := r.Group("/wellbeing")
	{
		// 屏幕时间
		wellbeing.GET("/screen-time", h.getScreenTime)
		wellbeing.GET("/screen-time/range", h.getScreenTimeRange)

		// 使用模式分析
		wellbeing.GET("/patterns", h.getPatterns)

		// 专注模式
		wellbeing.POST("/focus", h.startFocus)
		wellbeing.GET("/focus", h.listFocusSessions)
		wellbeing.GET("/focus/:id", h.getFocusSession)
		wellbeing.PUT("/focus/:id/stop", h.stopFocus)

		// 家庭成员
		wellbeing.GET("/family", h.listMembers)
		wellbeing.POST("/family", h.addMember)
		wellbeing.PUT("/family/:id", h.updateMember)
		wellbeing.DELETE("/family/:id", h.removeMember)

		// 健康报告
		wellbeing.GET("/report", h.getReport)

		// 停机时间
		wellbeing.GET("/downtime", h.getDowntime)
		wellbeing.POST("/downtime", h.setDowntime)

		// 应用限制
		wellbeing.GET("/limits", h.getAppLimits)
		wellbeing.POST("/limits", h.setAppLimit)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
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

	st, err := h.manager.GetScreenTime(userID, date)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: st})
}

// getScreenTimeRange 获取时间段内的屏幕时间.
func (h *Handlers) getScreenTimeRange(c *gin.Context) {
	userID := c.Query("user_id")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if userID == "" || startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "user_id, start_date and end_date are required",
		})
		return
	}

	times, err := h.manager.GetScreenTimeRange(userID, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: times})
}

// getPatterns 获取使用模式分析.
func (h *Handlers) getPatterns(c *gin.Context) {
	userID := c.Query("user_id")
	period := c.DefaultQuery("period", "weekly")

	if userID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "user_id is required",
		})
		return
	}

	pattern, err := h.manager.AnalyzePatterns(userID, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: pattern})
}

// startFocus 开始专注会话.
func (h *Handlers) startFocus(c *gin.Context) {
	var req struct {
		UserID      string   `json:"user_id" binding:"required"`
		Name        string   `json:"name" binding:"required"`
		DurationMin int      `json:"duration_min" binding:"required"`
		BlockedApps []string `json:"blocked_apps,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	session, err := h.manager.StartFocus(req.UserID, req.Name, req.DurationMin, req.BlockedApps)
	if err != nil {
		c.JSON(http.StatusConflict, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "focus session started", Data: session})
}

// listFocusSessions 获取专注会话列表.
func (h *Handlers) listFocusSessions(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "user_id is required",
		})
		return
	}

	sessions := h.manager.ListFocusSessions(userID)
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: sessions})
}

// getFocusSession 获取专注会话详情.
func (h *Handlers) getFocusSession(c *gin.Context) {
	id := c.Param("id")
	session, err := h.manager.GetFocusSession(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: session})
}

// stopFocus 停止专注会话.
func (h *Handlers) stopFocus(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StopFocus(id); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "focus session stopped"})
}

// listMembers 获取家庭成员列表.
func (h *Handlers) listMembers(c *gin.Context) {
	members := h.manager.ListMembers()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: members})
}

// addMember 添加家庭成员.
func (h *Handlers) addMember(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Role     string `json:"role" binding:"required"`
		AgeGroup string `json:"age_group,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	member := h.manager.AddMember(req.Name, req.Role, req.AgeGroup)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "member added", Data: member})
}

// updateMember 更新家庭成员.
func (h *Handlers) updateMember(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name string `json:"name,omitempty"`
		Role string `json:"role,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	if err := h.manager.UpdateMember(id, req.Name, req.Role); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "member updated"})
}

// removeMember 移除家庭成员.
func (h *Handlers) removeMember(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RemoveMember(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "member removed"})
}

// getReport 获取健康报告.
func (h *Handlers) getReport(c *gin.Context) {
	userID := c.Query("user_id")
	period := c.DefaultQuery("period", "weekly")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if userID == "" || startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "user_id, start_date and end_date are required",
		})
		return
	}

	report, err := h.manager.GetReport(userID, period, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: report})
}

// getDowntime 获取停机时间计划.
func (h *Handlers) getDowntime(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "user_id is required",
		})
		return
	}

	schedule := h.manager.GetDowntimeSchedule(userID)
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: schedule})
}

// setDowntime 设置停机时间计划.
func (h *Handlers) setDowntime(c *gin.Context) {
	var req struct {
		UserID    string   `json:"user_id" binding:"required"`
		Enabled   bool     `json:"enabled"`
		StartHour int      `json:"start_hour"`
		EndHour   int      `json:"end_hour"`
		Days      []string `json:"days"`
		AllowApps []string `json:"allow_apps,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	schedule := &DowntimeSchedule{
		Enabled:   req.Enabled,
		StartHour: req.StartHour,
		EndHour:   req.EndHour,
		Days:      req.Days,
		AllowApps: req.AllowApps,
	}

	h.manager.SetDowntimeSchedule(req.UserID, schedule)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "downtime schedule set", Data: schedule})
}

// getAppLimits 获取应用限制.
func (h *Handlers) getAppLimits(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "user_id is required",
		})
		return
	}

	limits := h.manager.GetAppLimits(userID)
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: limits})
}

// setAppLimit 设置应用限制.
func (h *Handlers) setAppLimit(c *gin.Context) {
	var req struct {
		UserID   string `json:"user_id" binding:"required"`
		AppName  string `json:"app_name" binding:"required"`
		AppID    string `json:"app_id" binding:"required"`
		DailyMin int    `json:"daily_min" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	limit := h.manager.SetAppLimit(req.UserID, req.AppName, req.AppID, req.DailyMin)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "app limit set", Data: limit})
}
