// Package nvmeof provides REST API handlers for NVMe over Fabric management.
package nvmeof

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for NVMe-oF management.
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler creates a new NVMe-oF HTTP handler.
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{manager: manager, logger: logger}
}

// RegisterRoutes registers NVMe-oF API routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	nvme := rg.Group("/nvmeof")
	{
		// Subsystem management
		nvme.POST("/subsystems", h.CreateSubsystem)
		nvme.GET("/subsystems", h.ListSubsystems)
		nvme.GET("/subsystems/:nqn", h.GetSubsystem)
		nvme.DELETE("/subsystems/:nqn", h.DeleteSubsystem)
		nvme.PUT("/subsystems/:nqn/allow-host", h.AllowHost)
		nvme.PUT("/subsystems/:nqn/revoke-host", h.RevokeHost)

		// Namespace management
		nvme.POST("/subsystems/:nqn/namespaces", h.AddNamespace)
		nvme.DELETE("/subsystems/:nqn/namespaces/:nsid", h.RemoveNamespace)

		// Port management
		nvme.POST("/ports", h.CreatePort)
		nvme.GET("/ports", h.ListPorts)
		nvme.DELETE("/ports/:portid", h.StopPort)

		// Stats
		nvme.GET("/stats", h.GetStats)
	}
}

// createSubsystemReq is the request body for creating a subsystem.
type createSubsystemReq struct {
	NQN           string `json:"nqn" binding:"required"`
	SerialNumber  string `json:"serial_number"`
	ModelNumber   string `json:"model_number"`
	MaxNamespaces int    `json:"max_namespaces"`
}

// CreateSubsystem handles POST /api/v1/nvmeof/subsystems.
func (h *Handler) CreateSubsystem(c *gin.Context) {
	var req createSubsystemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.MaxNamespaces <= 0 {
		req.MaxNamespaces = 256
	}
	if req.SerialNumber == "" {
		req.SerialNumber = "NAS-OS-001"
	}
	if req.ModelNumber == "" {
		req.ModelNumber = "NAS-OS NVMe Controller"
	}

	subsys, err := h.manager.CreateSubsystem(c.Request.Context(),
		req.NQN, req.SerialNumber, req.ModelNumber, req.MaxNamespaces)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, subsys)
}

// ListSubsystems handles GET /api/v1/nvmeof/subsystems.
func (h *Handler) ListSubsystems(c *gin.Context) {
	subsystems := h.manager.ListSubsystems()
	c.JSON(http.StatusOK, gin.H{
		"subsystems": subsystems,
		"total":      len(subsystems),
	})
}

// GetSubsystem handles GET /api/v1/nvmeof/subsystems/:nqn.
func (h *Handler) GetSubsystem(c *gin.Context) {
	nqn := c.Param("nqn")
	subsys, err := h.manager.GetSubsystem(nqn)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, subsys)
}

// DeleteSubsystem handles DELETE /api/v1/nvmeof/subsystems/:nqn.
func (h *Handler) DeleteSubsystem(c *gin.Context) {
	nqn := c.Param("nqn")
	if err := h.manager.DeleteSubsystem(c.Request.Context(), nqn); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "subsystem deleted"})
}

// allowHostReq is the request body for allowing a host.
type allowHostReq struct {
	HostNQN string `json:"host_nqn" binding:"required"`
}

// AllowHost handles PUT /api/v1/nvmeof/subsystems/:nqn/allow-host.
func (h *Handler) AllowHost(c *gin.Context) {
	nqn := c.Param("nqn")
	var req allowHostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.AllowHost(c.Request.Context(), nqn, req.HostNQN); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "host allowed"})
}

// RevokeHost handles PUT /api/v1/nvmeof/subsystems/:nqn/revoke-host.
func (h *Handler) RevokeHost(c *gin.Context) {
	nqn := c.Param("nqn")
	var req allowHostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.RevokeHost(c.Request.Context(), nqn, req.HostNQN); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "host revoked"})
}

// addNamespaceReq is the request body for adding a namespace.
type addNamespaceReq struct {
	Path      string `json:"path" binding:"required"`
	Size      int64  `json:"size" binding:"required"`
	BlockSize int    `json:"block_size"`
}

// AddNamespace handles POST /api/v1/nvmeof/subsystems/:nqn/namespaces.
func (h *Handler) AddNamespace(c *gin.Context) {
	nqn := c.Param("nqn")
	var req addNamespaceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.BlockSize <= 0 {
		req.BlockSize = 4096
	}

	ns, err := h.manager.AddNamespace(c.Request.Context(), nqn, req.Path, req.Size, req.BlockSize)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, ns)
}

// RemoveNamespace handles DELETE /api/v1/nvmeof/subsystems/:nqn/namespaces/:nsid.
func (h *Handler) RemoveNamespace(c *gin.Context) {
	nqn := c.Param("nqn")
	nsIDStr := c.Param("nsid")
	nsID, err := strconv.Atoi(nsIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid namespace ID"})
		return
	}

	if err := h.manager.RemoveNamespace(c.Request.Context(), nqn, nsID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "namespace removed"})
}

// createPortReq is the request body for creating a port.
type createPortReq struct {
	Transport string `json:"transport"`
	Address   string `json:"address" binding:"required"`
	Port      int    `json:"port" binding:"required"`
}

// CreatePort handles POST /api/v1/nvmeof/ports.
func (h *Handler) CreatePort(c *gin.Context) {
	var req createPortReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Transport == "" {
		req.Transport = "tcp"
	}

	port, err := h.manager.CreatePort(c.Request.Context(), req.Transport, req.Address, req.Port)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, port)
}

// ListPorts handles GET /api/v1/nvmeof/ports.
func (h *Handler) ListPorts(c *gin.Context) {
	ports := h.manager.ListPorts()
	c.JSON(http.StatusOK, gin.H{
		"ports": ports,
		"total": len(ports),
	})
}

// StopPort handles DELETE /api/v1/nvmeof/ports/:portid.
func (h *Handler) StopPort(c *gin.Context) {
	portID := c.Param("portid")
	if err := h.manager.StopPort(c.Request.Context(), portID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "port stopped"})
}

// GetStats handles GET /api/v1/nvmeof/stats.
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}
