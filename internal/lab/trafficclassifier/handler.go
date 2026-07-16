// Package trafficclassifier 提供 REST API 处理器
package trafficclassifier

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 流量分类 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	tc := r.Group("/traffic-classifier")
	{
		// 流量分析
		tc.POST("/analyze", h.analyze)
		tc.GET("/stats", h.getStats)

		// 分类规则
		tc.POST("/rules", h.addRule)
		tc.GET("/rules", h.listRules)
		tc.GET("/rules/:id", h.getRule)
		tc.PUT("/rules/:id", h.updateRule)
		tc.DELETE("/rules/:id", h.deleteRule)

		// 带宽策略
		tc.POST("/bandwidth-policies", h.addBandwidthPolicy)
		tc.GET("/bandwidth-policies", h.listBandwidthPolicies)
		tc.GET("/bandwidth-policies/:id", h.getBandwidthPolicy)
		tc.DELETE("/bandwidth-policies/:id", h.deleteBandwidthPolicy)

		// 镜像配置
		tc.POST("/mirrors", h.addMirrorConfig)
		tc.GET("/mirrors", h.listMirrorConfigs)
		tc.DELETE("/mirrors/:id", h.deleteMirrorConfig)

		// QoS 规则
		tc.POST("/qos-rules", h.addQoSRule)
		tc.GET("/qos-rules", h.listQoSRules)
		tc.DELETE("/qos-rules/:id", h.deleteQoSRule)

		// 异常告警
		tc.GET("/alerts", h.listAlerts)
		tc.POST("/alerts/:id/resolve", h.resolveAlert)

		// 报告
		tc.POST("/reports", h.generateReport)
		tc.GET("/reports", h.listReports)
		tc.GET("/reports/:id", h.getReport)

		// 配置
		tc.GET("/config", h.getConfig)
		tc.PUT("/config", h.updateConfig)

		// 服务控制
		tc.POST("/start", h.start)
		tc.POST("/stop", h.stop)
		tc.GET("/status", h.status)
	}
}

// response 标准响应.
type tcResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// analyze 流量分析.
func (h *Handlers) analyze(c *gin.Context) {
	var req AnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, tcResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	resp, err := h.manager.AnalyzeFlows(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, tcResponse{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "success", Data: resp})
}

// getStats 获取流量统计.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "success", Data: stats})
}

// addRule 添加分类规则.
func (h *Handlers) addRule(c *gin.Context) {
	var rule ClassificationRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, tcResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	h.manager.AddRule(&rule)
	c.JSON(http.StatusCreated, tcResponse{Code: 0, Message: "rule created", Data: rule})
}

// listRules 列出分类规则.
func (h *Handlers) listRules(c *gin.Context) {
	rules := h.manager.ListRules()
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "success", Data: rules})
}

// getRule 获取分类规则.
func (h *Handlers) getRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.manager.GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, tcResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "success", Data: rule})
}

// updateRule 更新分类规则.
func (h *Handlers) updateRule(c *gin.Context) {
	id := c.Param("id")
	var rule ClassificationRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, tcResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	rule.ID = id
	if err := h.manager.UpdateRule(&rule); err != nil {
		c.JSON(http.StatusNotFound, tcResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "rule updated", Data: rule})
}

// deleteRule 删除分类规则.
func (h *Handlers) deleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRule(id); err != nil {
		c.JSON(http.StatusNotFound, tcResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "rule deleted"})
}

// addBandwidthPolicy 添加带宽策略.
func (h *Handlers) addBandwidthPolicy(c *gin.Context) {
	var policy BandwidthPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, tcResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	h.manager.AddBandwidthPolicy(&policy)
	c.JSON(http.StatusCreated, tcResponse{Code: 0, Message: "policy created", Data: policy})
}

// listBandwidthPolicies 列出带宽策略.
func (h *Handlers) listBandwidthPolicies(c *gin.Context) {
	policies := h.manager.ListBandwidthPolicies()
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "success", Data: policies})
}

// getBandwidthPolicy 获取带宽策略.
func (h *Handlers) getBandwidthPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetBandwidthPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, tcResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "success", Data: policy})
}

// deleteBandwidthPolicy 删除带宽策略.
func (h *Handlers) deleteBandwidthPolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteBandwidthPolicy(id); err != nil {
		c.JSON(http.StatusNotFound, tcResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "policy deleted"})
}

// addMirrorConfig 添加镜像配置.
func (h *Handlers) addMirrorConfig(c *gin.Context) {
	var cfg MirrorConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, tcResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	h.manager.AddMirrorConfig(&cfg)
	c.JSON(http.StatusCreated, tcResponse{Code: 0, Message: "mirror config created", Data: cfg})
}

// listMirrorConfigs 列出镜像配置.
func (h *Handlers) listMirrorConfigs(c *gin.Context) {
	configs := h.manager.ListMirrorConfigs()
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "success", Data: configs})
}

// deleteMirrorConfig 删除镜像配置.
func (h *Handlers) deleteMirrorConfig(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteMirrorConfig(id); err != nil {
		c.JSON(http.StatusNotFound, tcResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "mirror config deleted"})
}

// addQoSRule 添加 QoS 规则.
func (h *Handlers) addQoSRule(c *gin.Context) {
	var rule QoSRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, tcResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	h.manager.AddQoSRule(&rule)
	c.JSON(http.StatusCreated, tcResponse{Code: 0, Message: "qos rule created", Data: rule})
}

// listQoSRules 列出 QoS 规则.
func (h *Handlers) listQoSRules(c *gin.Context) {
	rules := h.manager.ListQoSRules()
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "success", Data: rules})
}

// deleteQoSRule 删除 QoS 规则.
func (h *Handlers) deleteQoSRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteQoSRule(id); err != nil {
		c.JSON(http.StatusNotFound, tcResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "qos rule deleted"})
}

// listAlerts 列出告警.
func (h *Handlers) listAlerts(c *gin.Context) {
	resolvedStr := c.DefaultQuery("resolved", "false")
	resolved := resolvedStr == "true"
	alerts := h.manager.ListAlerts(resolved)
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "success", Data: alerts})
}

// resolveAlert 解决告警.
func (h *Handlers) resolveAlert(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ResolveAlert(id); err != nil {
		c.JSON(http.StatusNotFound, tcResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "alert resolved"})
}

// generateReport 生成报告.
func (h *Handlers) generateReport(c *gin.Context) {
	var req ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, tcResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	report := h.manager.GenerateReport(&req)
	c.JSON(http.StatusCreated, tcResponse{Code: 0, Message: "report generated", Data: report})
}

// listReports 列出报告.
func (h *Handlers) listReports(c *gin.Context) {
	reports := h.manager.ListReports()
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "success", Data: reports})
}

// getReport 获取报告.
func (h *Handlers) getReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.manager.GetReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, tcResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "success", Data: report})
}

// getConfig 获取配置.
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "success", Data: cfg})
}

// updateConfig 更新配置.
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg ClassifierConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, tcResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	h.manager.UpdateConfig(&cfg)
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "config updated"})
}

// start 启动服务.
func (h *Handlers) start(c *gin.Context) {
	if err := h.manager.Start(); err != nil {
		c.JSON(http.StatusConflict, tcResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "service started"})
}

// stop 停止服务.
func (h *Handlers) stop(c *gin.Context) {
	h.manager.Stop()
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "service stopped"})
}

// status 获取服务状态.
func (h *Handlers) status(c *gin.Context) {
	running := h.manager.IsRunning()
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	c.JSON(http.StatusOK, tcResponse{Code: 0, Message: "success", Data: map[string]interface{}{
		"running": running,
		"limit":   limit,
	}})
}
