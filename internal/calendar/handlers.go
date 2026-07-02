// Package calendar 提供日历与事件管理功能.
package calendar

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 日历 HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/calendar 路由组.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	calendarGroup := api.Group("/calendar")
	{
		// 日历管理
		calendarGroup.GET("", h.listCalendars)
		calendarGroup.POST("", h.createCalendar)
		calendarGroup.GET("/:id", h.getCalendar)
		calendarGroup.PUT("/:id", h.updateCalendar)
		calendarGroup.DELETE("/:id", h.deleteCalendar)

		// 事件管理
		calendarGroup.GET("/:id/events", h.listEvents)
		calendarGroup.POST("/:id/events", h.createEvent)
		calendarGroup.GET("/events/:eventId", h.getEvent)
		calendarGroup.PUT("/events/:eventId", h.updateEvent)
		calendarGroup.DELETE("/events/:eventId", h.deleteEvent)

		// 事件查询
		calendarGroup.GET("/events", h.queryEvents)

		// ICS 导入/导出
		calendarGroup.GET("/:id/export", h.exportCalendar)
		calendarGroup.POST("/:id/import", h.importICS)
		calendarGroup.GET("/export-all", h.exportAll)
	}
}

// ========== 日历 Handlers ==========

// listCalendars 列出所有日历.
func (h *Handlers) listCalendars(c *gin.Context) {
	cals := h.manager.ListCalendars()
	c.JSON(http.StatusOK, gin.H{
		"calendars": cals,
		"total":     len(cals),
	})
}

// createCalendar 创建日历.
func (h *Handlers) createCalendar(c *gin.Context) {
	var cal Calendar
	if err := c.ShouldBindJSON(&cal); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.CreateCalendar(&cal); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "日历创建成功",
		"calendar": cal,
	})
}

// getCalendar 获取日历.
func (h *Handlers) getCalendar(c *gin.Context) {
	id := c.Param("id")
	cal, err := h.manager.GetCalendar(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cal)
}

// updateCalendar 更新日历.
func (h *Handlers) updateCalendar(c *gin.Context) {
	id := c.Param("id")
	var cal Calendar
	if err := c.ShouldBindJSON(&cal); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	cal.ID = id

	if err := h.manager.UpdateCalendar(&cal); err != nil {
		code := http.StatusInternalServerError
		switch err {
		case ErrCalendarNotFound:
			code = http.StatusNotFound
		case ErrInvalidInput:
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "日历更新成功"})
}

// deleteCalendar 删除日历.
func (h *Handlers) deleteCalendar(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteCalendar(id); err != nil {
		code := http.StatusInternalServerError
		switch err {
		case ErrCalendarNotFound:
			code = http.StatusNotFound
		case ErrCalendarHasEvents:
			code = http.StatusConflict
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "日历删除成功", "id": id})
}

// ========== 事件 Handlers ==========

// listEvents 列出日历下的所有事件.
func (h *Handlers) listEvents(c *gin.Context) {
	calendarID := c.Param("id")
	if _, err := h.manager.GetCalendar(calendarID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	events := h.manager.QueryEvents(EventQuery{CalendarID: calendarID})
	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  len(events),
	})
}

// createEvent 创建事件.
func (h *Handlers) createEvent(c *gin.Context) {
	calendarID := c.Param("id")

	var evt Event
	if err := c.ShouldBindJSON(&evt); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	evt.CalendarID = calendarID

	if err := h.manager.CreateEvent(&evt); err != nil {
		code := http.StatusBadRequest
		if err == ErrCalendarNotFound {
			code = http.StatusNotFound
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "事件创建成功",
		"event":   evt,
	})
}

// getEvent 获取事件.
func (h *Handlers) getEvent(c *gin.Context) {
	eventID := c.Param("eventId")
	evt, err := h.manager.GetEvent(eventID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, evt)
}

// updateEvent 更新事件.
func (h *Handlers) updateEvent(c *gin.Context) {
	eventID := c.Param("eventId")
	var evt Event
	if err := c.ShouldBindJSON(&evt); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	evt.ID = eventID

	if err := h.manager.UpdateEvent(&evt); err != nil {
		code := http.StatusInternalServerError
		switch err {
		case ErrEventNotFound:
			code = http.StatusNotFound
		case ErrCalendarNotFound:
			code = http.StatusNotFound
		case ErrInvalidInput:
			code = http.StatusBadRequest
		case ErrEndTimeBeforeStart:
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "事件更新成功"})
}

// deleteEvent 删除事件.
func (h *Handlers) deleteEvent(c *gin.Context) {
	eventID := c.Param("eventId")
	if err := h.manager.DeleteEvent(eventID); err != nil {
		code := http.StatusInternalServerError
		if err == ErrEventNotFound {
			code = http.StatusNotFound
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "事件删除成功", "id": eventID})
}

// ========== 查询 ==========

// queryEvents 按条件查询事件.
func (h *Handlers) queryEvents(c *gin.Context) {
	var q EventQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "查询参数无效: " + err.Error()})
		return
	}

	// 解析日期参数
	if startStr := c.Query("start"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			q.Start = t
		} else if t, err := time.Parse("2006-01-02", startStr); err == nil {
			q.Start = t
		}
	}
	if endStr := c.Query("end"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			q.End = t
		} else if t, err := time.Parse("2006-01-02", endStr); err == nil {
			q.End = t
		}
	}

	events := h.manager.QueryEvents(q)
	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  len(events),
	})
}

// ========== ICS 导入/导出 ==========

// exportCalendar 导出单个日历.
func (h *Handlers) exportCalendar(c *gin.Context) {
	calendarID := c.Param("id")
	icsContent, err := h.manager.ExportCalendar(calendarID)
	if err != nil {
		code := http.StatusInternalServerError
		if err == ErrCalendarNotFound {
			code = http.StatusNotFound
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "text/calendar; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=calendar.ics")
	c.String(http.StatusOK, icsContent)
}

// exportAll 导出所有日历.
func (h *Handlers) exportAll(c *gin.Context) {
	icsContent, err := h.manager.ExportAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "text/calendar; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=all-calendars.ics")
	c.String(http.StatusOK, icsContent)
}

// importICS 导入 ICS 文件.
func (h *Handlers) importICS(c *gin.Context) {
	calendarID := c.Param("id")

	var input struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	count, err := h.manager.ImportICS(calendarID, input.Content)
	if err != nil {
		code := http.StatusInternalServerError
		if err == ErrCalendarNotFound {
			code = http.StatusNotFound
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "ICS 导入成功",
		"imported":    count,
		"calendar_id": calendarID,
	})
}
