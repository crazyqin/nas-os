package smbaudit

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 提供 SMB 审计的 HTTP 处理器
type Handlers struct {
	logger *SMBAuditLogger
}

// NewHandlers 创建新的 SMB 审计处理器
func NewHandlers(logger *SMBAuditLogger) *Handlers {
	return &Handlers{logger: logger}
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

	if user != "" {
		events := h.logger.GetByUser(user, limit+offset)
		total := len(events)
		if offset >= total {
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": []*AuditEntry{}, "total": 0})
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
	events := h.logger.Query(&AuditFilter{}, limit, offset)
	total := h.logger.GetStats()["total_events"].(int)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": events, "total": total})
}

// getEvent 获取单个审计事件详情
func (h *Handlers) getEvent(c *gin.Context) {
	id := c.Param("id")
	events := h.logger.Query(&AuditFilter{}, 10000, 0)
	for _, e := range events {
		if e.ID == id {
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
	result := AuditResult("failure")
	events := h.logger.Query(&AuditFilter{Result: &result}, limit, 0)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": events, "total": len(events)})
}

// getStats 获取审计统计
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.logger.GetStats()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats})
}

// exportEvents 导出审计事件
func (h *Handlers) exportEvents(c *gin.Context) {
	data, err := h.logger.ExportJSON()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=smb-audit-export.json")
	c.Data(http.StatusOK, "application/json", data)
}

// clearEvents 清理旧事件
func (h *Handlers) clearEvents(c *gin.Context) {
	h.logger.Clear()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "清理完成"})
}

// getConfig 获取审计配置
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.logger.GetConfig()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": cfg})
}

// updateConfig 更新审计配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg AuditConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.logger.UpdateConfig(&cfg)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "配置已更新"})
}
