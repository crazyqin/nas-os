package containerhealth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 容器健康监控 HTTP 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建容器健康监控处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册容器健康监控路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	ch := rg.Group("/container-health")
	{
		ch.GET("/containers", h.listContainers)
		ch.POST("/containers", h.registerContainer)
		ch.GET("/containers/:id", h.getContainer)
		ch.DELETE("/containers/:id", h.unregisterContainer)
		ch.POST("/containers/:id/check", h.checkHealth)
		ch.POST("/containers/:id/restart", h.restartContainer)
		ch.PUT("/containers/:id/auto-restart", h.setAutoRestart)
		ch.GET("/report", h.getReport)
		ch.POST("/check-all", h.checkAll)
	}
}

// listContainers 列出所有监控中的容器
func (h *Handlers) listContainers(c *gin.Context) {
	containers := h.manager.ListContainers()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": containers})
}

// registerContainer 注册容器到健康监控
func (h *Handlers) registerContainer(c *gin.Context) {
	var req struct {
		ContainerID string           `json:"container_id" binding:"required"`
		Name        string           `json:"name" binding:"required"`
		Config      HealthCheckConfig `json:"config" binding:"required"`
		AutoRestart bool             `json:"auto_restart"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.RegisterContainer(req.ContainerID, req.Name, req.Config, req.AutoRestart); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "容器已注册到健康监控"})
}

// getContainer 获取容器健康状态详情
func (h *Handlers) getContainer(c *gin.Context) {
	id := c.Param("id")
	container, err := h.manager.GetContainer(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": container})
}

// unregisterContainer 注销容器健康监控
func (h *Handlers) unregisterContainer(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.UnregisterContainer(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "容器已注销"})
}

// checkHealth 手动检查容器健康状态
func (h *Handlers) checkHealth(c *gin.Context) {
	id := c.Param("id")
	container, err := h.manager.CheckHealth(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": container})
}

// restartContainer 重启容器
func (h *Handlers) restartContainer(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RestartContainer(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "容器重启请求已发送"})
}

// setAutoRestart 设置容器自动重启开关
func (h *Handlers) setAutoRestart(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Enable bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.SetAutoRestart(id, req.Enable); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "自动重启设置已更新"})
}

// getReport 获取健康统计报告
func (h *Handlers) getReport(c *gin.Context) {
	report := h.manager.GetHealthReport()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": report})
}

// checkAll 检查所有容器健康状态
func (h *Handlers) checkAll(c *gin.Context) {
	results := h.manager.CheckAllHealth()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": results})
}
