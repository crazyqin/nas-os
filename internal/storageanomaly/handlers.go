// Package storageanomaly 提供 REST API 处理器
package storageanomaly

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 存储异常检测 API 处理器.
type Handlers struct {
	manager *AnomalyManager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *AnomalyManager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	sa := r.Group("/storage/anomaly")
	{
		// 基线管理
		sa.POST("/baseline", h.BuildBaseline)
		sa.GET("/baseline", h.ListBaselines)
		sa.GET("/baseline/*path", h.GetBaseline)

		// 异常检测
		sa.POST("/detect", h.DetectAnomaly)
		sa.POST("/ingest", h.IngestSample)

		// 事件管理
		sa.GET("/events", h.ListEvents)
		sa.GET("/events/:id", h.GetEvent)
		sa.POST("/events/:id/resolve", h.ResolveEvent)

		// 规则管理
		sa.GET("/rules", h.ListRules)
		sa.POST("/rules", h.AddRule)
		sa.PUT("/rules/:id/toggle", h.ToggleRule)
		sa.DELETE("/rules/:id", h.RemoveRule)

		// 配置
		sa.GET("/config", h.GetConfig)
		sa.PUT("/config", h.UpdateConfig)

		// 统计
		sa.GET("/stats", h.GetStats)
	}
}

// ========== 基线接口 ==========

// BuildBaseline 构建基线.
func (h *Handlers) BuildBaseline(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	baseline, err := h.manager.BuildBaseline(req.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "baseline built", Data: baseline})
}

// ListBaselines 列出基线.
func (h *Handlers) ListBaselines(c *gin.Context) {
	baselines := h.manager.ListBaselines()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":     len(baselines),
			"baselines": baselines,
		},
	})
}

// GetBaseline 获取基线.
func (h *Handlers) GetBaseline(c *gin.Context) {
	path := c.Param("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "path is required"})
		return
	}

	baseline, err := h.manager.GetBaseline(path)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: baseline})
}

// ========== 检测接口 ==========

// DetectAnomaly 检测异常.
func (h *Handlers) DetectAnomaly(c *gin.Context) {
	var req struct {
		Path       string         `json:"path" binding:"required"`
		SampleData SampleDataPoint `json:"sample_data"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	events, err := h.manager.DetectAnomaly(req.Path, req.SampleData)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "detection completed",
		Data: gin.H{
			"anomaly_count": len(events),
			"events":        events,
		},
	})
}

// IngestSample 导入采样数据.
func (h *Handlers) IngestSample(c *gin.Context) {
	var req IngestSampleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	h.manager.IngestSample(req)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "sample ingested",
		Data: gin.H{
			"path":         req.Path,
			"sample_count": h.manager.GetSampleCount(req.Path),
		},
	})
}

// ========== 事件接口 ==========

// ListEvents 列出事件.
func (h *Handlers) ListEvents(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	severity := c.Query("severity")
	eventType := c.Query("type")

	events := h.manager.ListEvents(limit, severity, eventType)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(events),
			"events": events,
		},
	})
}

// GetEvent 获取事件详情.
func (h *Handlers) GetEvent(c *gin.Context) {
	id := c.Param("id")
	evt, err := h.manager.GetEvent(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: evt})
}

// ResolveEvent 标记事件已解决.
func (h *Handlers) ResolveEvent(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ResolveEvent(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "event resolved"})
}

// ========== 规则接口 ==========

// ListRules 列出规则.
func (h *Handlers) ListRules(c *gin.Context) {
	rules := h.manager.ListRules()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(rules),
			"rules":  rules,
		},
	})
}

// AddRule 添加规则.
func (h *Handlers) AddRule(c *gin.Context) {
	var req AddRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	rule := h.manager.AddRule(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "rule created", Data: rule})
}

// ToggleRule 启用/禁用规则.
func (h *Handlers) ToggleRule(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.manager.ToggleRule(id, req.Enabled); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "rule updated"})
}

// RemoveRule 移除规则.
func (h *Handlers) RemoveRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RemoveRule(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "rule removed"})
}

// ========== 配置接口 ==========

// GetConfig 获取配置.
func (h *Handlers) GetConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: config})
}

// UpdateConfig 更新配置.
func (h *Handlers) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	h.manager.UpdateConfig(req)
	c.JSON(http.StatusOK, response{Code: 0, Message: "config updated", Data: h.manager.GetConfig()})
}

// ========== 统计接口 ==========

// GetStats 获取统计.
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}
