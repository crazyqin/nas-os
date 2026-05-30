// Package nvmefabrics 提供 NVMe over Fabrics 功能
package nvmefabrics

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers NVMe over Fabrics API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	v1 := r.Group("/v1")
	{
		// 目标管理
		v1.POST("/targets", h.createTarget)
		v1.GET("/targets", h.listTargets)
		v1.GET("/targets/:id", h.getTarget)
		v1.DELETE("/targets/:id", h.deleteTarget)

		// 子系统管理
		v1.POST("/targets/:target_id/subsystems", h.createSubsystem)
		v1.GET("/subsystems", h.listSubsystems)
		v1.GET("/subsystems/:nqn", h.getSubsystem)
		v1.DELETE("/subsystems/:nqn", h.deleteSubsystem)

		// 命名空间管理
		v1.POST("/subsystems/:nqn/namespaces", h.addNamespace)

		// 主机管理
		v1.POST("/subsystems/:nqn/hosts", h.addHost)
		v1.DELETE("/subsystems/:nqn/hosts/:host_nqn", h.removeHost)

		// 控制器管理
		v1.POST("/subsystems/:nqn/controllers", h.connectController)
		v1.GET("/controllers", h.listControllers)
		v1.DELETE("/controllers/:id", h.disconnectController)
		v1.GET("/controllers/:id/stats", h.getControllerStats)

		// 统计信息
		v1.GET("/stats", h.getStats)
	}
}

// createTarget 创建目标
func (h *Handlers) createTarget(c *gin.Context) {
	var req CreateTargetRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	target, err := h.manager.CreateTarget(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": target})
}

// listTargets 列出目标
func (h *Handlers) listTargets(c *gin.Context) {
	transport := TransportType(c.Query("transport"))

	targets := h.manager.ListTargets(transport)

	c.JSON(http.StatusOK, gin.H{"data": targets})
}

// getTarget 获取目标详情
func (h *Handlers) getTarget(c *gin.Context) {
	id := c.Param("id")

	target, err := h.manager.GetTarget(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": target})
}

// deleteTarget 删除目标
func (h *Handlers) deleteTarget(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteTarget(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "target deleted"})
}

// createSubsystem 创建子系统
func (h *Handlers) createSubsystem(c *gin.Context) {
	targetID := c.Param("target_id")

	var req CreateSubsystemRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	subsystem, err := h.manager.CreateSubsystem(targetID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": subsystem})
}

// listSubsystems 列出子系统
func (h *Handlers) listSubsystems(c *gin.Context) {
	targetID := c.Query("target_id")

	subsystems := h.manager.ListSubsystems(targetID)

	c.JSON(http.StatusOK, gin.H{"data": subsystems})
}

// getSubsystem 获取子系统详情
func (h *Handlers) getSubsystem(c *gin.Context) {
	nqn := c.Param("nqn")

	subsystem, err := h.manager.GetSubsystem(nqn)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": subsystem})
}

// deleteSubsystem 删除子系统
func (h *Handlers) deleteSubsystem(c *gin.Context) {
	nqn := c.Param("nqn")

	if err := h.manager.DeleteSubsystem(nqn); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "subsystem deleted"})
}

// addNamespace 添加命名空间
func (h *Handlers) addNamespace(c *gin.Context) {
	nqn := c.Param("nqn")

	var req AddNamespaceRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ns, err := h.manager.AddSubsystemNamespace(nqn, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": ns})
}

// addHost 添加主机
func (h *Handlers) addHost(c *gin.Context) {
	nqn := c.Param("nqn")

	var req struct {
		HostNQN string `json:"host_nqn" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.AddHost(nqn, req.HostNQN); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "host added"})
}

// removeHost 移除主机
func (h *Handlers) removeHost(c *gin.Context) {
	nqn := c.Param("nqn")
	hostNQN := c.Param("host_nqn")

	if err := h.manager.RemoveHost(nqn, hostNQN); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "host removed"})
}

// connectController 连接控制器
func (h *Handlers) connectController(c *gin.Context) {
	nqn := c.Param("nqn")

	var req ConnectHostRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	controller, err := h.manager.ConnectController(nqn, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": controller})
}

// listControllers 列出控制器
func (h *Handlers) listControllers(c *gin.Context) {
	nqn := c.Query("nqn")

	controllers := h.manager.ListControllers(nqn)

	c.JSON(http.StatusOK, gin.H{"data": controllers})
}

// disconnectController 断开控制器
func (h *Handlers) disconnectController(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DisconnectController(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "controller disconnected"})
}

// getControllerStats 获取控制器统计
func (h *Handlers) getControllerStats(c *gin.Context) {
	id := c.Param("id")

	stats, err := h.manager.GetControllerStats(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// getStats 获取统计信息
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()

	c.JSON(http.StatusOK, gin.H{"data": stats})
}
