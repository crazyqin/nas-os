// Package auditlog 审计日志 - HTTP API
package auditlog

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/audit")
	{
		group.POST("/logs", h.AddEntry)
		group.GET("/logs", h.QueryLogs)
		group.GET("/anomalies", h.GetAnomalies)
		group.POST("/anomalies/:id/resolve", h.ResolveAnomaly)
		group.POST("/reports", h.GenerateReport)
		group.GET("/reports/:id", h.GetReport)
		group.GET("/stats", h.GetStats)
	}
}

// AddEntry 添加日志
func (h *Handler) AddEntry(c *gin.Context) {
	var entry AuditEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效参数"})
		return
	}
	h.manager.AddEntry(entry)
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "日志已记录"})
}

// QueryLogs 查询日志
func (h *Handler) QueryLogs(c *gin.Context) {
	filter := LogFilter{
		Level:  LogLevel(c.Query("level")),
		Source: LogSource(c.Query("source")),
		User:   c.Query("user"),
	}
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			filter.Limit = v
		}
	}
	if t := c.Query("start_time"); t != "" {
		if v, err := time.Parse(time.RFC3339, t); err == nil {
			filter.StartTime = v
		}
	}
	if t := c.Query("end_time"); t != "" {
		if v, err := time.Parse(time.RFC3339, t); err == nil {
			filter.EndTime = v
		}
	}
	entries := h.manager.Query(filter)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": entries})
}

// GetAnomalies 获取异常
func (h *Handler) GetAnomalies(c *gin.Context) {
	resolved := c.Query("resolved") == "true"
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": h.manager.GetAnomalies(resolved)})
}

// ResolveAnomaly 解决异常
func (h *Handler) ResolveAnomaly(c *gin.Context) {
	if err := h.manager.ResolveAnomaly(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "异常已解决"})
}

// GenerateReport 生成报告
func (h *Handler) GenerateReport(c *gin.Context) {
	var req struct {
		Period string `json:"period"`
	}
	c.ShouldBindJSON(&req)
	if req.Period == "" {
		req.Period = "last_30_days"
	}
	report := h.manager.GenerateReport(req.Period)
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "报告已生成", "data": report})
}

// GetReport 获取报告
func (h *Handler) GetReport(c *gin.Context) {
	report, err := h.manager.GetReport(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": report})
}

// GetStats 获取统计
func (h *Handler) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": h.manager.GetStats()})
}
