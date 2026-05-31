package trafficshaper

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 流量整形 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	traffic := r.Group("/traffic")
	{
		// 规则管理
		traffic.POST("/rules", h.createRule)
		traffic.GET("/rules", h.listRules)
		traffic.PUT("/rules/:id", h.updateRule)
		traffic.DELETE("/rules/:id", h.deleteRule)
		traffic.POST("/rules/:id/toggle", h.toggleRule)

		// 类别管理
		traffic.POST("/classes", h.createClass)
		traffic.GET("/classes", h.listClasses)
		traffic.PUT("/classes/:id", h.updateClass)
		traffic.DELETE("/classes/:id", h.deleteClass)

		// 统计
		traffic.GET("/stats", h.getGlobalStats)
		traffic.GET("/stats/rules/:id", h.getRuleStats)

		// 带宽分配
		traffic.GET("/allocation", h.getAllocation)
		traffic.POST("/allocation/rebalance", h.rebalance)

		// 事件日志
		traffic.GET("/events", h.getEvents)

		// 模拟
		traffic.POST("/simulate", h.simulateTraffic)

		// 配置
		traffic.GET("/config", h.getConfig)
		traffic.PUT("/config", h.updateConfig)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// createRule 创建流量规则
func (h *Handlers) createRule(c *gin.Context) {
	var rule TrafficRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.CreateRule(&rule)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "rule created",
		Data:    result,
	})
}

// listRules 列出流量规则
func (h *Handlers) listRules(c *gin.Context) {
	rules := h.manager.ListRules()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    rules,
	})
}

// updateRule 更新流量规则
func (h *Handlers) updateRule(c *gin.Context) {
	id := c.Param("id")
	var rule TrafficRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.UpdateRule(id, &rule)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "rule updated",
		Data:    result,
	})
}

// deleteRule 删除流量规则
func (h *Handlers) deleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRule(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "rule deleted",
	})
}

// toggleRule 启用/禁用流量规则
func (h *Handlers) toggleRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.manager.ToggleRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "rule toggled",
		Data:    rule,
	})
}

// createClass 创建流量类别
func (h *Handlers) createClass(c *gin.Context) {
	var class TrafficClass
	if err := c.ShouldBindJSON(&class); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.CreateClass(&class)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "class created",
		Data:    result,
	})
}

// listClasses 列出流量类别
func (h *Handlers) listClasses(c *gin.Context) {
	classes := h.manager.ListClasses()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    classes,
	})
}

// updateClass 更新流量类别
func (h *Handlers) updateClass(c *gin.Context) {
	id := c.Param("id")
	var class TrafficClass
	if err := c.ShouldBindJSON(&class); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.UpdateClass(id, &class)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "class updated",
		Data:    result,
	})
}

// deleteClass 删除流量类别
func (h *Handlers) deleteClass(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteClass(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "class deleted",
	})
}

// getGlobalStats 获取全局流量统计
func (h *Handlers) getGlobalStats(c *gin.Context) {
	stats := h.manager.GetGlobalStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// getRuleStats 获取指定规则的流量统计
func (h *Handlers) getRuleStats(c *gin.Context) {
	id := c.Param("id")
	stats, err := h.manager.GetRuleStats(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// getAllocation 获取带宽分配
func (h *Handlers) getAllocation(c *gin.Context) {
	allocation := h.manager.GetAllocation()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    allocation,
	})
}

// rebalance 重新平衡带宽分配
func (h *Handlers) rebalance(c *gin.Context) {
	allocation := h.manager.Rebalance()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "bandwidth rebalanced",
		Data:    allocation,
	})
}

// getEvents 获取事件日志
func (h *Handlers) getEvents(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	events := h.manager.GetEvents()
	if len(events) > limit {
		events = events[len(events)-limit:]
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    events,
	})
}

// simulateTraffic 模拟流量
func (h *Handlers) simulateTraffic(c *gin.Context) {
	h.manager.SimulateTraffic()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "traffic simulated",
	})
}

// getConfig 获取配置
func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    config,
	})
}

// updateConfig 更新配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var config TrafficShaperConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	h.manager.UpdateConfig(&config)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "config updated",
	})
}
