// Package syslogserver 提供 REST API 处理器
package syslogserver

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 日志管理 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/syslog 路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	sl := r.Group("/syslog")
	{
		// 日志查询
		sl.GET("/logs", h.searchLogs)
		sl.GET("/logs/:id", h.getLogByID)

		// 日志导出
		sl.POST("/logs/export", h.exportLogs)

		// 转发目标管理
		sl.GET("/forward-targets", h.listForwardTargets)
		sl.POST("/forward-targets", h.createForwardTarget)
		sl.GET("/forward-targets/:id", h.getForwardTarget)
		sl.PUT("/forward-targets/:id", h.updateForwardTarget)
		sl.DELETE("/forward-targets/:id", h.deleteForwardTarget)

		// 告警规则管理
		sl.GET("/alert-rules", h.listAlertRules)
		sl.POST("/alert-rules", h.createAlertRule)
		sl.GET("/alert-rules/:id", h.getAlertRule)
		sl.PUT("/alert-rules/:id", h.updateAlertRule)
		sl.DELETE("/alert-rules/:id", h.deleteAlertRule)

		// 告警事件
		sl.GET("/alert-events", h.listAlertEvents)

		// 仪表板统计
		sl.GET("/stats/dashboard", h.getDashboardStats)

		// WebSocket 实时日志流
		sl.GET("/ws", h.handleWebSocket)

		// 服务器状态
		sl.GET("/status", h.getStatus)
	}
}

// ========== 日志查询处理 ==========

// searchLogs 搜索日志.
func (h *Handlers) searchLogs(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	result := h.manager.SearchLogs(req)
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: result})
}

// getLogByID 根据 ID 获取日志.
func (h *Handlers) getLogByID(c *gin.Context) {
	id := c.Param("id")
	entry, err := h.manager.GetLogByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: entry})
}

// ========== 日志导出处理 ==========

// exportLogs 导出日志.
func (h *Handlers) exportLogs(c *gin.Context) {
	var req ExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	data, err := h.manager.ExportLogs(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: "export failed: " + err.Error()})
		return
	}

	switch req.Format {
	case "csv":
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=syslog-export.csv")
	case "json":
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", "attachment; filename=syslog-export.json")
	}

	c.Data(http.StatusOK, c.GetHeader("Content-Type"), data)
}

// ========== 转发目标处理 ==========

// listForwardTargets 列出所有转发目标.
func (h *Handlers) listForwardTargets(c *gin.Context) {
	targets := h.manager.ListForwardTargets()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(targets),
			"targets": targets,
		},
	})
}

// createForwardTarget 创建转发目标.
func (h *Handlers) createForwardTarget(c *gin.Context) {
	var req CreateForwardTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	target := h.manager.CreateForwardTarget(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: target})
}

// getForwardTarget 获取转发目标.
func (h *Handlers) getForwardTarget(c *gin.Context) {
	id := c.Param("id")
	target, err := h.manager.GetForwardTarget(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: target})
}

// updateForwardTarget 更新转发目标.
func (h *Handlers) updateForwardTarget(c *gin.Context) {
	id := c.Param("id")
	var req UpdateForwardTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	target, err := h.manager.UpdateForwardTarget(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: target})
}

// deleteForwardTarget 删除转发目标.
func (h *Handlers) deleteForwardTarget(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteForwardTarget(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== 告警规则处理 ==========

// listAlertRules 列出所有告警规则.
func (h *Handlers) listAlertRules(c *gin.Context) {
	rules := h.manager.ListAlertRules()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(rules),
			"rules": rules,
		},
	})
}

// createAlertRule 创建告警规则.
func (h *Handlers) createAlertRule(c *gin.Context) {
	var req CreateAlertRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	rule := h.manager.CreateAlertRule(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: rule})
}

// getAlertRule 获取告警规则.
func (h *Handlers) getAlertRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.manager.GetAlertRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: rule})
}

// updateAlertRule 更新告警规则.
func (h *Handlers) updateAlertRule(c *gin.Context) {
	id := c.Param("id")
	var req UpdateAlertRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	rule, err := h.manager.UpdateAlertRule(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: rule})
}

// deleteAlertRule 删除告警规则.
func (h *Handlers) deleteAlertRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteAlertRule(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== 告警事件处理 ==========

// listAlertEvents 列出告警事件.
func (h *Handlers) listAlertEvents(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 100
	}

	events := h.manager.ListAlertEvents(limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(events),
			"events": events,
		},
	})
}

// ========== 仪表板统计处理 ==========

// getDashboardStats 获取仪表板统计.
func (h *Handlers) getDashboardStats(c *gin.Context) {
	stats := h.manager.GetDashboardStats()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}

// ========== WebSocket 处理 ==========

// handleWebSocket 处理 WebSocket 连接 (SSE 模式).
func (h *Handlers) handleWebSocket(c *gin.Context) {
	// 获取过滤参数
	hostname := c.Query("hostname")
	appName := c.Query("app_name")
	facility := c.Query("facility")
	severity := c.Query("severity")

	var filter *SearchRequest
	if hostname != "" || appName != "" || facility != "" || severity != "" {
		filter = &SearchRequest{
			Hostname: hostname,
			AppName:  appName,
			Facility: facility,
			Severity: severity,
		}
	}

	// 升级为 SSE (Server-Sent Events)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	clientID := uuid.New().String()
	client := &WSClient{
		ID:     clientID,
		Conn:   nil, // SSE 模式
		Filter: filter,
		Send:   make(chan []byte, 100),
	}

	h.manager.RegisterWSClient(client)
	defer h.manager.UnregisterWSClient(clientID)

	// 使用 SSE 流式输出
	c.Stream(func(w io.Writer) bool {
		select {
		case data, ok := <-client.Send:
			if !ok {
				return false
			}
			w.Write(data)
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// ========== 状态查询 ==========

// getStatus 获取服务器状态.
func (h *Handlers) getStatus(c *gin.Context) {
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"status":          "running",
			"ws_clients":      h.manager.GetWSClientCount(),
			"total_entries":   len(h.manager.entries),
			"forward_targets": len(h.manager.forwardTargets),
			"alert_rules":     len(h.manager.alertRules),
		},
	})
}
