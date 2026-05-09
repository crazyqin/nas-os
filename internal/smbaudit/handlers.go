package smbaudit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 提供 SMB 审计的 HTTP 处理器
type Handlers struct {
	auditor *Auditor
}

// NewHandlers 创建新的 SMB 审计处理器
func NewHandlers(auditor *Auditor) *Handlers {
	return &Handlers{auditor: auditor}
}

// RegisterRoutes 注册 SMB 审计 API 路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	smb := rg.Group("/smb-audit")
	{
		smb.GET("/events", h.listEvents)
		smb.GET("/events/:id", h.getEvent)
		smb.GET("/failed", h.getFailedEvents)
		smb.GET("/stats", h.getStats)
		smb.POST("/export", h.exportEvents)
		smb.DELETE("/events", h.clearEvents)
		smb.GET("/config", h.getConfig)
		smb.PUT("/config", h.updateConfig)
	}
}

// listEvents 获取审计事件列表
func (h *Handlers) listEvents(c *gin.Context) {
	limit := 100
	offset := 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	// 支持按条件过滤
	user := c.Query("user")
	share := c.Query("share")
	action := c.Query("action")
	startStr := c.Query("start")
	endStr := c.Query("end")

	// 如果有时间范围参数
	if startStr != "" && endStr != "" {
		start, err1 := time.Parse(time.RFC3339, startStr)
		end, err2 := time.Parse(time.RFC3339, endStr)
		if err1 != nil || err2 != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "时间格式无效，请使用 RFC3339 格式"})
			return
		}
		events := h.auditor.GetEventsByTimeRange(start, end)
		// 应用偏移和限制
		total := len(events)
		if offset >= total {
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": []AuditEvent{}, "total": 0})
			return
		}
		endIdx := offset + limit
		if endIdx > total {
			endIdx = total
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": events[offset:endIdx], "total": total})
		return
	}

	if user != "" {
		events := h.auditor.GetEventsByUser(user, limit+offset)
		total := len(events)
		if offset >= total {
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": []AuditEvent{}, "total": 0})
			return
		}
		endIdx := offset + limit
		if endIdx > total {
			endIdx = total
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": events[offset:endIdx], "total": total})
		return
	}

	if share != "" {
		events := h.auditor.GetEventsByShare(share, limit+offset)
		total := len(events)
		if offset >= total {
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": []AuditEvent{}, "total": 0})
			return
		}
		endIdx := offset + limit
		if endIdx > total {
			endIdx = total
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": events[offset:endIdx], "total": total})
		return
	}

	if action != "" {
		events := h.auditor.GetEventsByAction(action, limit+offset)
		total := len(events)
		if offset >= total {
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": []AuditEvent{}, "total": 0})
			return
		}
		endIdx := offset + limit
		if endIdx > total {
			endIdx = total
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": events[offset:endIdx], "total": total})
		return
	}

	// 默认分页查询
	events, total := h.auditor.GetEvents(limit, offset)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": events, "total": total})
}

// getEvent 获取单个审计事件详情
func (h *Handlers) getEvent(c *gin.Context) {
	id := c.Param("id")
	events, _ := h.auditor.GetEvents(h.auditor.GetAuditStats()["total_events"].(int), 0)
	for _, e := range events {
		if e.EventID == id {
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": e})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "事件未找到"})
}

// getFailedEvents 获取失败事件
func (h *Handlers) getFailedEvents(c *gin.Context) {
	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	events := h.auditor.GetFailedEvents(limit)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": events, "total": len(events)})
}

// getStats 获取审计统计
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.auditor.GetAuditStats()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats})
}

// exportEvents 导出审计事件
func (h *Handlers) exportEvents(c *gin.Context) {
	var req struct {
		Start  string `json:"start"`
		End    string `json:"end"`
		Format string `json:"format"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	start, err := time.Parse(time.RFC3339, req.Start)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "start 时间格式无效"})
		return
	}
	end, err := time.Parse(time.RFC3339, req.End)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "end 时间格式无效"})
		return
	}

	data, err := h.auditor.ExportEvents(start, end, req.Format)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	switch req.Format {
	case "csv":
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=smb-audit-export.csv")
	default:
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", "attachment; filename=smb-audit-export.json")
	}
	c.Data(http.StatusOK, c.Writer.Header().Get("Content-Type"), data)
}

// clearEvents 清理旧事件
func (h *Handlers) clearEvents(c *gin.Context) {
	days := 90
	if v := c.Query("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = n
		}
	}
	before := time.Now().AddDate(0, 0, -days)
	removed := h.auditor.ClearEvents(before)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "清理完成", "removed": removed})
}

// getConfig 获取审计配置
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.auditor.GetConfig()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": cfg})
}

// updateConfig 更新审计配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg AuditConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.auditor.UpdateConfig(cfg)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "配置已更新"})
}
