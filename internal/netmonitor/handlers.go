// Package netmonitor 提供网络监控 HTTP API
package netmonitor

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 网络监控 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	net := r.Group("/netmonitor")
	{
		net.GET("/interfaces", h.getInterfaces)
		net.GET("/interfaces/:name", h.getInterface)
		net.GET("/traffic", h.getTraffic)
		net.GET("/connections", h.getConnections)
		net.POST("/alerts", h.addAlertRule)
		net.GET("/alerts", h.getAlertRules)
		net.DELETE("/alerts/:id", h.removeAlertRule)
		net.GET("/alerts/events", h.getAlertEvents)
		net.GET("/topology", h.getTopology)
		net.POST("/topology/discover", h.discoverTopology)
		net.GET("/ports", h.checkPorts)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// getInterfaces 获取所有网络接口.
func (h *Handlers) getInterfaces(c *gin.Context) {
	ifaces := h.manager.GetInterfaces()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":      len(ifaces),
			"interfaces": ifaces,
		},
	})
}

// getInterface 获取指定网络接口.
func (h *Handlers) getInterface(c *gin.Context) {
	name := c.Param("name")

	iface, err := h.manager.GetInterface(name)
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
		Data:    iface,
	})
}

// getTraffic 获取流量统计.
func (h *Handlers) getTraffic(c *gin.Context) {
	iface := c.Query("interface")

	stats := h.manager.GetTrafficStats(iface)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(stats),
			"stats":  stats,
		},
	})
}

// getConnections 获取连接状态.
func (h *Handlers) getConnections(c *gin.Context) {
	stats := h.manager.GetConnections()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// addAlertRule 添加告警规则.
func (h *Handlers) addAlertRule(c *gin.Context) {
	var rule AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.AddAlertRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "alert rule added",
		Data:    rule,
	})
}

// getAlertRules 获取所有告警规则.
func (h *Handlers) getAlertRules(c *gin.Context) {
	rules := h.manager.GetAlertRules()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(rules),
			"rules": rules,
		},
	})
}

// removeAlertRule 删除告警规则.
func (h *Handlers) removeAlertRule(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.RemoveAlertRule(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "alert rule removed",
	})
}

// getAlertEvents 获取告警事件.
func (h *Handlers) getAlertEvents(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	events := h.manager.GetAlertEvents(limit)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(events),
			"events": events,
		},
	})
}

// getTopology 获取网络拓扑.
func (h *Handlers) getTopology(c *gin.Context) {
	topology := h.manager.GetTopology()

	if topology == nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: "topology not discovered yet, run discovery first",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    topology,
	})
}

// discoverTopology 发现网络拓扑.
func (h *Handlers) discoverTopology(c *gin.Context) {
	topology := h.manager.DiscoverTopology()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "topology discovered",
		Data:    topology,
	})
}

// PortCheckRequest 端口检查请求.
type PortCheckRequest struct {
	Host  string `json:"host" binding:"required"`
	Ports []int  `json:"ports" binding:"required"`
}

// checkPorts 检查端口状态.
func (h *Handlers) checkPorts(c *gin.Context) {
	host := c.Query("host")
	portsStr := c.QueryArray("ports")

	if host == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "host parameter is required",
		})
		return
	}

	ports := make([]int, 0)
	for _, p := range portsStr {
		port, err := strconv.Atoi(p)
		if err != nil {
			continue
		}
		ports = append(ports, port)
	}

	if len(ports) == 0 {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "at least one port is required",
		})
		return
	}

	results := h.manager.CheckPorts(host, ports)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"host":    host,
			"checked": time.Now(),
			"results": results,
		},
	})
}
