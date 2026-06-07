// Package ailoganalyzer 提供 REST API 处理器
package ailoganalyzer

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers AI 日志分析器 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/ai-log-analyzer 路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	ala := r.Group("/ai-log-analyzer")
	{
		// 日志管理
		ala.POST("/logs", h.addLog)
		ala.GET("/logs", h.queryLogs)
		ala.GET("/logs/:id", h.getLog)
		ala.DELETE("/logs", h.deleteLogs)

		// 模式识别
		ala.POST("/patterns", h.createPattern)
		ala.GET("/patterns", h.listPatterns)
		ala.GET("/patterns/:id", h.getPattern)
		ala.PUT("/patterns/:id", h.updatePattern)
		ala.DELETE("/patterns/:id", h.deletePattern)

		// 异常检测规则
		ala.POST("/rules", h.createRule)
		ala.GET("/rules", h.listRules)
		ala.GET("/rules/:id", h.getRule)
		ala.PUT("/rules/:id", h.updateRule)
		ala.DELETE("/rules/:id", h.deleteRule)

		// 告警管理
		ala.GET("/alerts", h.listAlerts)
		ala.GET("/alerts/:id", h.getAlert)
		ala.PUT("/alerts/:id", h.updateAlert)
		ala.DELETE("/alerts/:id", h.deleteAlert)

		// 日志聚类
		ala.GET("/clusters", h.listClusters)
		ala.GET("/clusters/:id", h.getCluster)

		// 根因分析
		ala.POST("/analysis/:alertId", h.analyzeRootCause)
		ala.GET("/analysis", h.listAnalyses)
		ala.GET("/analysis/:id", h.getAnalysis)

		// 日志流
		ala.POST("/streams", h.createStream)
		ala.GET("/streams", h.listStreams)
		ala.GET("/streams/:id", h.getStream)
		ala.POST("/streams/:id/start", h.startStream)
		ala.POST("/streams/:id/stop", h.stopStream)
		ala.DELETE("/streams/:id", h.deleteStream)

		// 保留策略
		ala.POST("/policies", h.createRetentionPolicy)
		ala.GET("/policies", h.listRetentionPolicies)
		ala.DELETE("/policies/:id", h.deleteRetentionPolicy)
		ala.POST("/policies/apply", h.applyRetentionPolicies)

		// 统计
		ala.GET("/stats", h.getStats)
		ala.POST("/analysis/run", h.runAnalysis)
	}
}

// ========== 日志管理 ==========

func (h *Handlers) addLog(c *gin.Context) {
	var entry LogEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	h.manager.AddLog(&entry)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: entry})
}

func (h *Handlers) queryLogs(c *gin.Context) {
	var req QueryLogsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid query: " + err.Error()})
		return
	}

	// 解析时间参数
	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			req.StartTime = &t
		}
	}
	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			req.EndTime = &t
		}
	}

	// 解析分页参数
	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil {
			req.Page = page
		}
	}
	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if pageSize, err := strconv.Atoi(pageSizeStr); err == nil {
			req.PageSize = pageSize
		}
	}

	logs, total := h.manager.QueryLogs(req)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":     total,
			"page":      req.Page,
			"page_size": req.PageSize,
			"logs":      logs,
		},
	})
}

func (h *Handlers) getLog(c *gin.Context) {
	id := c.Param("id")
	log, err := h.manager.GetLog(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: log})
}

func (h *Handlers) deleteLogs(c *gin.Context) {
	beforeStr := c.Query("before")
	if beforeStr == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "parameter 'before' is required"})
		return
	}

	before, err := time.Parse(time.RFC3339, beforeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid time format"})
		return
	}

	count := h.manager.DeleteLogs(before)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "deleted",
		Data: gin.H{
			"count": count,
		},
	})
}

// ========== 模式识别 ==========

func (h *Handlers) createPattern(c *gin.Context) {
	var req CreatePatternRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	pattern := h.manager.CreatePattern(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: pattern})
}

func (h *Handlers) listPatterns(c *gin.Context) {
	patterns := h.manager.ListPatterns()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(patterns),
			"patterns": patterns,
		},
	})
}

func (h *Handlers) getPattern(c *gin.Context) {
	id := c.Param("id")
	pattern, err := h.manager.GetPattern(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: pattern})
}

func (h *Handlers) updatePattern(c *gin.Context) {
	id := c.Param("id")
	var req UpdatePatternRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	pattern, err := h.manager.UpdatePattern(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: pattern})
}

func (h *Handlers) deletePattern(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePattern(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== 异常检测规则 ==========

func (h *Handlers) createRule(c *gin.Context) {
	var req CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	rule := h.manager.CreateRule(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: rule})
}

func (h *Handlers) listRules(c *gin.Context) {
	rules := h.manager.ListRules()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(rules),
			"rules": rules,
		},
	})
}

func (h *Handlers) getRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.manager.GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: rule})
}

func (h *Handlers) updateRule(c *gin.Context) {
	id := c.Param("id")
	var req UpdateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	rule, err := h.manager.UpdateRule(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: rule})
}

func (h *Handlers) deleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRule(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== 告警管理 ==========

func (h *Handlers) listAlerts(c *gin.Context) {
	status := c.Query("status")
	alerts := h.manager.ListAlerts(status)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(alerts),
			"alerts": alerts,
		},
	})
}

func (h *Handlers) getAlert(c *gin.Context) {
	id := c.Param("id")
	alert, err := h.manager.GetAlert(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: alert})
}

func (h *Handlers) updateAlert(c *gin.Context) {
	id := c.Param("id")
	var req UpdateAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	alert, err := h.manager.UpdateAlert(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: alert})
}

func (h *Handlers) deleteAlert(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteAlert(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== 日志聚类 ==========

func (h *Handlers) listClusters(c *gin.Context) {
	clusters := h.manager.ListClusters()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(clusters),
			"clusters": clusters,
		},
	})
}

func (h *Handlers) getCluster(c *gin.Context) {
	id := c.Param("id")
	cluster, err := h.manager.GetCluster(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: cluster})
}

// ========== 根因分析 ==========

func (h *Handlers) analyzeRootCause(c *gin.Context) {
	alertID := c.Param("alertId")
	analysis, err := h.manager.AnalyzeRootCause(alertID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "analysis completed", Data: analysis})
}

func (h *Handlers) listAnalyses(c *gin.Context) {
	analyses := h.manager.ListAnalyses()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(analyses),
			"analyses": analyses,
		},
	})
}

func (h *Handlers) getAnalysis(c *gin.Context) {
	id := c.Param("id")
	analysis, err := h.manager.GetAnalysis(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: analysis})
}

// ========== 日志流 ==========

func (h *Handlers) createStream(c *gin.Context) {
	var req CreateStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	stream := h.manager.CreateStream(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: stream})
}

func (h *Handlers) listStreams(c *gin.Context) {
	streams := h.manager.ListStreams()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(streams),
			"streams": streams,
		},
	})
}

func (h *Handlers) getStream(c *gin.Context) {
	id := c.Param("id")
	stream, err := h.manager.GetStream(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stream})
}

func (h *Handlers) startStream(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StartStream(id); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "stream started"})
}

func (h *Handlers) stopStream(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StopStream(id); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "stream stopped"})
}

func (h *Handlers) deleteStream(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteStream(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== 保留策略 ==========

func (h *Handlers) createRetentionPolicy(c *gin.Context) {
	var req CreateRetentionPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	policy := h.manager.CreateRetentionPolicy(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: policy})
}

func (h *Handlers) listRetentionPolicies(c *gin.Context) {
	policies := h.manager.ListRetentionPolicies()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(policies),
			"policies": policies,
		},
	})
}

func (h *Handlers) deleteRetentionPolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRetentionPolicy(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

func (h *Handlers) applyRetentionPolicies(c *gin.Context) {
	count := h.manager.ApplyRetentionPolicies()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "policies applied",
		Data: gin.H{
			"deleted_count": count,
		},
	})
}

// ========== 统计 ==========

func (h *Handlers) getStats(c *gin.Context) {
	var req StatsQueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid query: " + err.Error()})
		return
	}

	// 解析时间参数
	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			req.StartTime = &t
		}
	}
	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			req.EndTime = &t
		}
	}

	stats := h.manager.GetStats(req)
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}

func (h *Handlers) runAnalysis(c *gin.Context) {
	var req StatsQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	result := h.manager.RunAnalysis(req)
	c.JSON(http.StatusOK, response{Code: 0, Message: "analysis completed", Data: result})
}
