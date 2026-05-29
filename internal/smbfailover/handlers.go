package smbfailover

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers provides HTTP handlers for SMBFailover
type Handlers struct {
	manager *FailoverManager
	logger  *zap.Logger
}

// NewHandlers creates new SMBFailover handlers
func NewHandlers(manager *FailoverManager, logger *zap.Logger) *Handlers {
	return &Handlers{manager: manager, logger: logger}
}

// RegisterRoutes registers SMBFailover API routes
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	fo := rg.Group("/failover")
	{
		fo.GET("/status", h.GetStatus)
		fo.GET("/nodes", h.ListNodes)
		fo.POST("/nodes", h.RegisterNode)
		fo.POST("/initiate", h.InitiateFailover)
		fo.GET("/events", h.GetEvents)
		fo.GET("/sessions", h.ListSessions)
	}
}

// GetStatus returns cluster status
func (h *Handlers) GetStatus(c *gin.Context) {
	status := h.manager.GetClusterStatus()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": status})
}

// ListNodes lists all cluster nodes
func (h *Handlers) ListNodes(c *gin.Context) {
	h.manager.mu.RLock()
	defer h.manager.mu.RUnlock()
	nodes := make([]*ClusterNode, 0, len(h.manager.nodes))
	for _, n := range h.manager.nodes {
		nodes = append(nodes, n)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": nodes})
}

// RegisterNode registers a new cluster node
func (h *Handlers) RegisterNode(c *gin.Context) {
	var node ClusterNode
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	h.manager.RegisterNode(&node)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "node registered"})
}

// InitiateFailover manually triggers failover
func (h *Handlers) InitiateFailover(c *gin.Context) {
	var req struct {
		TargetNode string `json:"target_node"`
		Reason     string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.manager.InitiateFailover(req.TargetNode, req.Reason); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "failover initiated"})
}

// GetEvents returns failover events
func (h *Handlers) GetEvents(c *gin.Context) {
	limit := 20
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	events := h.manager.GetEvents(limit)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": events})
}

// ListSessions lists tracked SMB sessions
func (h *Handlers) ListSessions(c *gin.Context) {
	h.manager.mu.RLock()
	defer h.manager.mu.RUnlock()
	sessions := make([]*SMBSession, 0, len(h.manager.sessions))
	for _, s := range h.manager.sessions {
		sessions = append(sessions, s)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": sessions, "total": len(sessions)})
}
