package snmpagent

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 提供 SNMP 管理的 HTTP 处理器
type Handlers struct {
	agent *Agent
}

// NewHandlers 创建新的 SNMP 处理器
func NewHandlers(agent *Agent) *Handlers {
	return &Handlers{agent: agent}
}

// RegisterRoutes 注册 SNMP API 路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	snmp := rg.Group("/snmp")
	{
		snmp.GET("/status", h.getStatus)
		snmp.POST("/start", h.startAgent)
		snmp.POST("/stop", h.stopAgent)
		snmp.GET("/metrics", h.listMetrics)
		snmp.POST("/metrics", h.registerMetric)
		snmp.PUT("/metrics/:oid", h.updateMetric)
		snmp.DELETE("/metrics/:oid", h.deleteMetric)
		snmp.GET("/config", h.getConfig)
		snmp.PUT("/config", h.updateConfig)
	}
}

// getStatus 返回 SNMP 代理状态
func (h *Handlers) getStatus(c *gin.Context) {
	status := h.agent.GetStatus()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": status})
}

// startAgent 启动 SNMP 代理
func (h *Handlers) startAgent(c *gin.Context) {
	if err := h.agent.Start(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "SNMP 代理已启动"})
}

// stopAgent 停止 SNMP 代理
func (h *Handlers) stopAgent(c *gin.Context) {
	if err := h.agent.Stop(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "SNMP 代理已停止"})
}

// listMetrics 列出所有 SNMP 指标
func (h *Handlers) listMetrics(c *gin.Context) {
	metrics := h.agent.ListMetrics()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": metrics})
}

// registerMetric 注册新的 SNMP 指标
func (h *Handlers) registerMetric(c *gin.Context) {
	var req struct {
		OID    string            `json:"oid"`
		Name   string            `json:"name"`
		Value  interface{}       `json:"value"`
		Type   string            `json:"type"`
		Labels map[string]string `json:"labels"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.agent.RegisterMetric(req.OID, req.Name, req.Value, req.Type, req.Labels); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "指标已注册"})
}

// updateMetric 更新 SNMP 指标值
func (h *Handlers) updateMetric(c *gin.Context) {
	oid := c.Param("oid")
	var req struct {
		Value interface{} `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.agent.UpdateMetric(oid, req.Value); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "指标已更新"})
}

// deleteMetric 删除 SNMP 指标
func (h *Handlers) deleteMetric(c *gin.Context) {
	oid := c.Param("oid")
	if err := h.agent.UnregisterMetric(oid); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "指标已删除"})
}

// getConfig 获取 SNMP 配置
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.agent.GetConfig()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": cfg})
}

// updateConfig 更新 SNMP 配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg SNMPConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.agent.UpdateConfig(cfg)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "配置已更新"})
}
