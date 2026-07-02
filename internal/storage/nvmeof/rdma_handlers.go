// Package nvmeof - NVMe-oF RDMA API Handlers
// 提供 RDMA Target 和 Initiator 的 REST API

package nvmeof

import (
	"errors"

	"nas-os/internal/api"

	pkgnvmeof "nas-os/pkg/storage/nvmeof"

	"github.com/gin-gonic/gin"
)

// RDMAHandlers RDMA API 处理器.
type RDMAHandlers struct {
	rdmaTargetManager    *RDMATargetSysManager
	rdmaInitiatorManager *RDMAInitiatorSysManager
}

// NewRDMAHandlers 创建 RDMA API 处理器.
func NewRDMAHandlers(rdmaTargetManager *RDMATargetSysManager, rdmaInitiatorManager *RDMAInitiatorSysManager) *RDMAHandlers {
	return &RDMAHandlers{
		rdmaTargetManager:    rdmaTargetManager,
		rdmaInitiatorManager: rdmaInitiatorManager,
	}
}

// RegisterRoutes 注册 RDMA API 路由.
func (h *RDMAHandlers) RegisterRoutes(r *gin.RouterGroup) {
	rdma := r.Group("/nvmeof/rdma")
	{
		// RDMA 设备管理
		rdma.GET("/devices", h.listRDMADevices)
		rdma.GET("/devices/:name", h.getRDMADevice)

		// RDMA Target 管理
		rdma.GET("/target/status", h.getRDMATargetStatus)
		rdma.GET("/target/stats", h.getRDMATargetStats)
		rdma.POST("/target/start", h.startRDMATarget)
		rdma.POST("/target/stop", h.stopRDMATarget)

		rdma.GET("/target/ports", h.listRDMAPorts)
		rdma.POST("/target/ports", h.createRDMAPort)
		rdma.GET("/target/ports/:id", h.getRDMAPort)
		rdma.DELETE("/target/ports/:id", h.deleteRDMAPort)

		rdma.POST("/target/ports/:id/link/:nqn", h.linkSubsystemToPort)
		rdma.DELETE("/target/ports/:id/link/:nqn", h.unlinkSubsystemFromPort)

		// RDMA Initiator 管理
		rdma.GET("/initiator/status", h.getRDMAInitiatorStatus)
		rdma.GET("/initiator/stats", h.getRDMAInitiatorStats)
		rdma.POST("/initiator/start", h.startRDMAInitiator)
		rdma.POST("/initiator/stop", h.stopRDMAInitiator)

		rdma.POST("/initiator/discover", h.discoverRDMATargets)
		rdma.GET("/initiator/controllers", h.listRDMAControllers)
		rdma.POST("/initiator/controllers", h.connectRDMATarget)
		rdma.GET("/initiator/controllers/:name", h.getRDMAController)
		rdma.DELETE("/initiator/controllers/:name", h.disconnectRDMATarget)
		rdma.DELETE("/initiator/controllers", h.disconnectAllRDMATargets)
	}
}

// ========== RDMA 设备管理 ==========

func (h *RDMAHandlers) listRDMADevices(c *gin.Context) {
	if h.rdmaTargetManager == nil {
		api.InternalError(c, "RDMA Target Manager not initialized")
		return
	}

	devices := h.rdmaTargetManager.GetRDMADevices()
	api.OK(c, devices)
}

func (h *RDMAHandlers) getRDMADevice(c *gin.Context) {
	if h.rdmaTargetManager == nil {
		api.InternalError(c, "RDMA Target Manager not initialized")
		return
	}

	name := c.Param("name")
	device, err := h.rdmaTargetManager.GetRDMADevice(name)
	if err != nil {
		if errors.Is(err, pkgnvmeof.ErrRDMADeviceNotFound) {
			api.NotFound(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, device)
}

// ========== RDMA Target 管理 ==========

func (h *RDMAHandlers) getRDMATargetStatus(c *gin.Context) {
	if h.rdmaTargetManager == nil {
		api.InternalError(c, "RDMA Target Manager not initialized")
		return
	}

	api.OK(c, gin.H{
		"running":   h.rdmaTargetManager.IsRunning(),
		"available": h.rdmaTargetManager.pkgRdmaManager.IsAvailable(),
	})
}

func (h *RDMAHandlers) getRDMATargetStats(c *gin.Context) {
	if h.rdmaTargetManager == nil {
		api.InternalError(c, "RDMA Target Manager not initialized")
		return
	}

	stats := h.rdmaTargetManager.GetRDMAStats()
	api.OK(c, stats)
}

func (h *RDMAHandlers) startRDMATarget(c *gin.Context) {
	if h.rdmaTargetManager == nil {
		api.InternalError(c, "RDMA Target Manager not initialized")
		return
	}

	if err := h.rdmaTargetManager.Start(c.Request.Context()); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "RDMA Target 服务已启动", gin.H{
		"running": h.rdmaTargetManager.IsRunning(),
	})
}

func (h *RDMAHandlers) stopRDMATarget(c *gin.Context) {
	if h.rdmaTargetManager == nil {
		api.InternalError(c, "RDMA Target Manager not initialized")
		return
	}

	if err := h.rdmaTargetManager.Stop(); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "RDMA Target 服务已停止", gin.H{
		"running": h.rdmaTargetManager.IsRunning(),
	})
}

func (h *RDMAHandlers) listRDMAPorts(c *gin.Context) {
	if h.rdmaTargetManager == nil {
		api.InternalError(c, "RDMA Target Manager not initialized")
		return
	}

	ports, err := h.rdmaTargetManager.ListRDMAPorts()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, ports)
}

func (h *RDMAHandlers) createRDMAPort(c *gin.Context) {
	if h.rdmaTargetManager == nil {
		api.InternalError(c, "RDMA Target Manager not initialized")
		return
	}

	var req CreateRDMAPortRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "无效的请求参数: "+err.Error())
		return
	}

	port, err := h.rdmaTargetManager.CreateRDMAPort(c.Request.Context(), &req)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	// 如果指定了子系统 NQN，自动链接
	if req.SubsystemNQN != "" {
		if err := h.rdmaTargetManager.LinkSubsystemToRDMAPort(c.Request.Context(), port.ID, req.SubsystemNQN); err != nil {
			// 链接失败不影响端口创建，但需要通知
			api.OKWithMessage(c, "RDMA 端口创建成功，但子系统链接失败: "+err.Error(), port)
			return
		}
	}

	api.Created(c, port)
}

func (h *RDMAHandlers) getRDMAPort(c *gin.Context) {
	if h.rdmaTargetManager == nil {
		api.InternalError(c, "RDMA Target Manager not initialized")
		return
	}

	portIDStr := c.Param("id")
	portID := 0
	for _, ch := range portIDStr {
		if ch >= '0' && ch <= '9' {
			portID = portID*10 + int(ch-'0')
		}
	}

	port, err := h.rdmaTargetManager.GetRDMAPort(portID)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, port)
}

func (h *RDMAHandlers) deleteRDMAPort(c *gin.Context) {
	if h.rdmaTargetManager == nil {
		api.InternalError(c, "RDMA Target Manager not initialized")
		return
	}

	portIDStr := c.Param("id")
	portID := 0
	for _, ch := range portIDStr {
		if ch >= '0' && ch <= '9' {
			portID = portID*10 + int(ch-'0')
		}
	}

	if err := h.rdmaTargetManager.DeleteRDMAPort(c.Request.Context(), portID); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "RDMA 端口已删除", nil)
}

func (h *RDMAHandlers) linkSubsystemToPort(c *gin.Context) {
	if h.rdmaTargetManager == nil {
		api.InternalError(c, "RDMA Target Manager not initialized")
		return
	}

	portIDStr := c.Param("id")
	portID := 0
	for _, ch := range portIDStr {
		if ch >= '0' && ch <= '9' {
			portID = portID*10 + int(ch-'0')
		}
	}

	nqn := c.Param("nqn")

	if err := h.rdmaTargetManager.LinkSubsystemToRDMAPort(c.Request.Context(), portID, nqn); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "子系统已链接到 RDMA 端口", gin.H{
		"portId": portID,
		"nqn":    nqn,
	})
}

func (h *RDMAHandlers) unlinkSubsystemFromPort(c *gin.Context) {
	if h.rdmaTargetManager == nil {
		api.InternalError(c, "RDMA Target Manager not initialized")
		return
	}

	portIDStr := c.Param("id")
	portID := 0
	for _, ch := range portIDStr {
		if ch >= '0' && ch <= '9' {
			portID = portID*10 + int(ch-'0')
		}
	}

	nqn := c.Param("nqn")

	if err := h.rdmaTargetManager.UnlinkSubsystemFromRDMAPort(c.Request.Context(), portID, nqn); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "子系统已从 RDMA 端口解链", gin.H{
		"portId": portID,
		"nqn":    nqn,
	})
}

// ========== RDMA Initiator 管理 ==========

func (h *RDMAHandlers) getRDMAInitiatorStatus(c *gin.Context) {
	if h.rdmaInitiatorManager == nil {
		api.InternalError(c, "RDMA Initiator Manager not initialized")
		return
	}

	api.OK(c, gin.H{
		"running":   h.rdmaInitiatorManager.IsRunning(),
		"available": h.rdmaInitiatorManager.pkgRdmaManager.IsAvailable(),
	})
}

func (h *RDMAHandlers) getRDMAInitiatorStats(c *gin.Context) {
	if h.rdmaInitiatorManager == nil {
		api.InternalError(c, "RDMA Initiator Manager not initialized")
		return
	}

	stats := h.rdmaInitiatorManager.GetRDMAInitiatorStats()
	api.OK(c, stats)
}

func (h *RDMAHandlers) startRDMAInitiator(c *gin.Context) {
	if h.rdmaInitiatorManager == nil {
		api.InternalError(c, "RDMA Initiator Manager not initialized")
		return
	}

	if err := h.rdmaInitiatorManager.Start(c.Request.Context()); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "RDMA Initiator 服务已启动", gin.H{
		"running": h.rdmaInitiatorManager.IsRunning(),
	})
}

func (h *RDMAHandlers) stopRDMAInitiator(c *gin.Context) {
	if h.rdmaInitiatorManager == nil {
		api.InternalError(c, "RDMA Initiator Manager not initialized")
		return
	}

	if err := h.rdmaInitiatorManager.Stop(); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "RDMA Initiator 服务已停止", gin.H{
		"running": h.rdmaInitiatorManager.IsRunning(),
	})
}

func (h *RDMAHandlers) discoverRDMATargets(c *gin.Context) {
	if h.rdmaInitiatorManager == nil {
		api.InternalError(c, "RDMA Initiator Manager not initialized")
		return
	}

	var req DiscoverRDMATargetsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "无效的请求参数: "+err.Error())
		return
	}

	entries, err := h.rdmaInitiatorManager.DiscoverRDMATargets(c.Request.Context(), &req)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, gin.H{
		"entries": entries,
		"address": req.Address,
		"port":    req.Port,
	})
}

func (h *RDMAHandlers) listRDMAControllers(c *gin.Context) {
	if h.rdmaInitiatorManager == nil {
		api.InternalError(c, "RDMA Initiator Manager not initialized")
		return
	}

	controllers := h.rdmaInitiatorManager.ListRDMAControllers()
	api.OK(c, controllers)
}

func (h *RDMAHandlers) connectRDMATarget(c *gin.Context) {
	if h.rdmaInitiatorManager == nil {
		api.InternalError(c, "RDMA Initiator Manager not initialized")
		return
	}

	var req ConnectRDMATargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "无效的请求参数: "+err.Error())
		return
	}

	controller, err := h.rdmaInitiatorManager.ConnectRDMATarget(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, pkgnvmeof.ErrControllerConnected) {
			api.Conflict(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, controller)
}

func (h *RDMAHandlers) getRDMAController(c *gin.Context) {
	if h.rdmaInitiatorManager == nil {
		api.InternalError(c, "RDMA Initiator Manager not initialized")
		return
	}

	name := c.Param("name")

	controller, err := h.rdmaInitiatorManager.GetRDMAController(name)
	if err != nil {
		if errors.Is(err, pkgnvmeof.ErrControllerDisconnected) {
			api.NotFound(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, controller)
}

func (h *RDMAHandlers) disconnectRDMATarget(c *gin.Context) {
	if h.rdmaInitiatorManager == nil {
		api.InternalError(c, "RDMA Initiator Manager not initialized")
		return
	}

	name := c.Param("name")

	if err := h.rdmaInitiatorManager.DisconnectRDMATarget(c.Request.Context(), name); err != nil {
		if errors.Is(err, pkgnvmeof.ErrControllerDisconnected) {
			api.NotFound(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "RDMA 连接已断开", nil)
}

func (h *RDMAHandlers) disconnectAllRDMATargets(c *gin.Context) {
	if h.rdmaInitiatorManager == nil {
		api.InternalError(c, "RDMA Initiator Manager not initialized")
		return
	}

	if err := h.rdmaInitiatorManager.DisconnectAllRDMATargets(c.Request.Context()); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "所有 RDMA 连接已断开", nil)
}
