// Package dashboardoverview 提供REST API处理器
package dashboardoverview

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type Handlers struct {
	manager *Manager
}

func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	dash := r.Group("/dashboard")
	{
		dash.GET("/overview", h.Overview)
		dash.GET("/cpu", h.CPU)
		dash.GET("/memory", h.Memory)
		dash.GET("/storage", h.Storage)
		dash.GET("/network", h.Network)
		dash.GET("/services", h.Services)
		dash.GET("/alerts", h.Alerts)
		dash.POST("/alerts/:id/ack", h.AckAlert)
		dash.GET("/activity", h.Activity)
		dash.GET("/widgets", h.Widgets)
	}
}

// Overview 系统概览.
func (h *Handlers) Overview(c *gin.Context) {
	overview := h.manager.GetOverview()
	c.JSON(http.StatusOK, response{Code: 200, Data: overview})
}

// CPU CPU信息.
func (h *Handlers) CPU(c *gin.Context) {
	cpu := h.manager.GetCPU()
	c.JSON(http.StatusOK, response{Code: 200, Data: cpu})
}

// Memory 内存信息.
func (h *Handlers) Memory(c *gin.Context) {
	mem := h.manager.GetMemory()
	c.JSON(http.StatusOK, response{Code: 200, Data: mem})
}

// Storage 存储信息.
func (h *Handlers) Storage(c *gin.Context) {
	storage := h.manager.GetStorage()
	c.JSON(http.StatusOK, response{Code: 200, Data: storage})
}

// Network 网络信息.
func (h *Handlers) Network(c *gin.Context) {
	network := h.manager.GetNetwork()
	c.JSON(http.StatusOK, response{Code: 200, Data: network})
}

// Services 服务信息.
func (h *Handlers) Services(c *gin.Context) {
	services := h.manager.GetServices()
	c.JSON(http.StatusOK, response{Code: 200, Data: services})
}

// Alerts 告警信息.
func (h *Handlers) Alerts(c *gin.Context) {
	includeAcked := c.Query("acked") == "true"
	alerts := h.manager.GetAlerts(includeAcked)
	c.JSON(http.StatusOK, response{Code: 200, Data: alerts})
}

// AckAlert 确认告警.
func (h *Handlers) AckAlert(c *gin.Context) {
	id := c.Param("id")
	h.manager.AckAlert(id)
	c.JSON(http.StatusOK, response{Code: 200, Message: "acked"})
}

// Activity 活动记录.
func (h *Handlers) Activity(c *gin.Context) {
	overview := h.manager.GetOverview()
	c.JSON(http.StatusOK, response{Code: 200, Data: overview.Recent})
}

// Widgets 组件列表.
func (h *Handlers) Widgets(c *gin.Context) {
	widgets := h.manager.GetWidgets()
	c.JSON(http.StatusOK, response{Code: 200, Data: widgets})
}
