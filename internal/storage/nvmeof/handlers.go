package nvmeof

import (
	"errors"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers NVMe-oF API.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/nvmeof")
	{
		group.GET("/status", h.getStatus)
		group.POST("/start", h.startService)
		group.POST("/stop", h.stopService)

		group.GET("/targets", h.listTargets)
		group.POST("/targets", h.createTarget)
		group.GET("/targets/:id", h.getTarget)
		group.DELETE("/targets/:id", h.deleteTarget)

		group.GET("/initiators", h.listInitiators)
		group.POST("/initiators", h.createInitiator)
		group.POST("/initiators/:id/connect", h.connectInitiator)
		group.POST("/initiators/:id/disconnect", h.disconnectInitiator)
		group.DELETE("/initiators/:id", h.deleteInitiator)
	}
}

// CreateTargetRequest 创建 Target 请求.
type CreateTargetRequest struct {
	Name        string      `json:"name" binding:"required"`
	NQN         string      `json:"nqn,omitempty"`
	Transport   Transport   `json:"transport,omitempty"`
	Address     string      `json:"address" binding:"required"`
	Port        int         `json:"port" binding:"required,min=1,max=65535"`
	Namespaces  []Namespace `json:"namespaces,omitempty"`
	Enabled     bool        `json:"enabled"`
	Description string      `json:"description,omitempty"`
}

// CreateInitiatorRequest 创建 Initiator 请求.
type CreateInitiatorRequest struct {
	Name      string    `json:"name" binding:"required"`
	Transport Transport `json:"transport,omitempty"`
	TargetNQN string    `json:"targetNqn" binding:"required"`
	Address   string    `json:"address" binding:"required"`
	Port      int       `json:"port" binding:"required,min=1,max=65535"`
	HostNQN   string    `json:"hostNqn,omitempty"`
}

func (h *Handlers) getStatus(c *gin.Context) {
	api.OK(c, gin.H{
		"status":     h.manager.GetStatus(),
		"targets":    len(h.manager.ListTargets()),
		"initiators": len(h.manager.ListInitiators()),
	})
}

func (h *Handlers) startService(c *gin.Context) {
	if err := h.manager.Start(); err != nil {
		api.InternalError(c, err.Error())
		return
	}
	api.OKWithMessage(c, "NVMe-oF 服务已启动", gin.H{"status": h.manager.GetStatus()})
}

func (h *Handlers) stopService(c *gin.Context) {
	if err := h.manager.Stop(); err != nil {
		api.InternalError(c, err.Error())
		return
	}
	api.OKWithMessage(c, "NVMe-oF 服务已停止", gin.H{"status": h.manager.GetStatus()})
}

func (h *Handlers) listTargets(c *gin.Context) {
	api.OK(c, h.manager.ListTargets())
}

func (h *Handlers) createTarget(c *gin.Context) {
	var req CreateTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "无效的请求参数: "+err.Error())
		return
	}
	if req.Transport == "" {
		req.Transport = TransportTCP
	}

	item, err := h.manager.CreateTarget(Target{
		Name:        req.Name,
		NQN:         req.NQN,
		Transport:   req.Transport,
		Address:     req.Address,
		Port:        req.Port,
		Namespaces:  req.Namespaces,
		Enabled:     req.Enabled,
		Description: req.Description,
	})
	if err != nil {
		if errors.Is(err, ErrTargetExists) {
			api.Conflict(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}
	api.Created(c, item)
}

func (h *Handlers) getTarget(c *gin.Context) {
	item, err := h.manager.GetTarget(c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrTargetNotFound) {
			api.NotFound(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}
	api.OK(c, item)
}

func (h *Handlers) deleteTarget(c *gin.Context) {
	if err := h.manager.DeleteTarget(c.Param("id")); err != nil {
		if errors.Is(err, ErrTargetNotFound) {
			api.NotFound(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}
	api.OKWithMessage(c, "删除成功", nil)
}

func (h *Handlers) listInitiators(c *gin.Context) {
	api.OK(c, h.manager.ListInitiators())
}

func (h *Handlers) createInitiator(c *gin.Context) {
	var req CreateInitiatorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "无效的请求参数: "+err.Error())
		return
	}
	if req.Transport == "" {
		req.Transport = TransportTCP
	}

	item, err := h.manager.CreateInitiator(Initiator{
		Name:      req.Name,
		Transport: req.Transport,
		TargetNQN: req.TargetNQN,
		Address:   req.Address,
		Port:      req.Port,
		HostNQN:   req.HostNQN,
	})
	if err != nil {
		if errors.Is(err, ErrInitiatorExists) {
			api.Conflict(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}
	api.Created(c, item)
}

func (h *Handlers) connectInitiator(c *gin.Context) {
	item, err := h.manager.ConnectInitiator(c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrInitiatorNotFound) {
			api.NotFound(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}
	api.OKWithMessage(c, "连接成功", item)
}

func (h *Handlers) disconnectInitiator(c *gin.Context) {
	item, err := h.manager.DisconnectInitiator(c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrInitiatorNotFound) {
			api.NotFound(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}
	api.OKWithMessage(c, "断开成功", item)
}

func (h *Handlers) deleteInitiator(c *gin.Context) {
	if err := h.manager.DeleteInitiator(c.Param("id")); err != nil {
		if errors.Is(err, ErrInitiatorNotFound) {
			api.NotFound(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}
	api.OKWithMessage(c, "删除成功", nil)
}
