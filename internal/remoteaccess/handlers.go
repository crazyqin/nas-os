package remoteaccess

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for remote access management.
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler creates a new remote access HTTP handler.
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{manager: manager, logger: logger}
}

// RegisterRoutes registers remote access API routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	ra := rg.Group("/remoteaccess")
	{
		// Configuration
		ra.GET("/config", h.GetConfig)
		ra.PUT("/config", h.UpdateConfig)

		// NAT Detection
		ra.GET("/nat/detect", h.DetectNAT)
		ra.GET("/nat/status", h.GetNATStatus)

		// P2P Sessions
		ra.POST("/p2p/sessions", h.CreateP2PSession)
		ra.GET("/p2p/sessions", h.ListP2PSessions)
		ra.GET("/p2p/sessions/:id", h.GetP2PSession)
		ra.DELETE("/p2p/sessions/:id", h.CloseP2PSession)

		// Relay Sessions
		ra.POST("/relay/sessions", h.CreateRelaySession)
		ra.GET("/relay/sessions", h.ListRelaySessions)
		ra.GET("/relay/sessions/:id", h.GetRelaySession)
		ra.DELETE("/relay/sessions/:id", h.CloseRelaySession)

		// DDNS
		ra.GET("/ddns/status", h.GetDDNSStatus)
		ra.POST("/ddns/update", h.UpdateDDNS)

		// Port Mappings
		ra.POST("/portmappings", h.CreatePortMapping)
		ra.GET("/portmappings", h.ListPortMappings)
		ra.GET("/portmappings/:id", h.GetPortMapping)
		ra.DELETE("/portmappings/:id", h.DeletePortMapping)
		ra.POST("/portmappings/refresh", h.RefreshPortMappings)

		// Connections
		ra.GET("/connections", h.ListConnections)
		ra.GET("/connections/stats", h.GetConnectionStats)
		ra.GET("/connections/:id", h.GetConnection)
		ra.DELETE("/connections/:id", h.CloseConnection)

		// Sessions
		ra.POST("/sessions", h.CreateSession)
		ra.GET("/sessions", h.ListSessions)
		ra.GET("/sessions/:id", h.GetSession)
		ra.POST("/sessions/:id/refresh", h.RefreshSession)
		ra.DELETE("/sessions/:id", h.InvalidateSession)
		ra.DELETE("/sessions/user/:user_id", h.InvalidateAllUserSessions)

		// Authentication
		ra.POST("/auth/login", h.Login)
		ra.POST("/auth/validate", h.ValidateToken)
		ra.POST("/auth/refresh", h.RefreshToken)
		ra.POST("/auth/revoke", h.RevokeToken)

		// Health
		ra.GET("/health", h.HealthCheck)
	}
}

// ============================================================
// Configuration Handlers
// ============================================================

// GetConfig handles GET /api/v1/remoteaccess/config.
func (h *Handler) GetConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, config)
}

// UpdateConfigRequest is the request body for updating configuration.
type UpdateConfigRequest struct {
	STUN       *STUNConfig       `json:"stun,omitempty"`
	TURN       *TURNConfig       `json:"turn,omitempty"`
	Relay      *RelayConfig      `json:"relay,omitempty"`
	DDNS       *DDNSConfig       `json:"ddns,omitempty"`
	UPnP       *UPnPConfig       `json:"upnp,omitempty"`
	NATPMP     *NATPMPConfig     `json:"natpmp,omitempty"`
	Auth       *AuthConfig       `json:"auth,omitempty"`
	Encryption *EncryptionConfig `json:"encryption,omitempty"`
	DebugMode  *bool             `json:"debug_mode,omitempty"`
}

// UpdateConfig handles PUT /api/v1/remoteaccess/config.
func (h *Handler) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := h.manager.GetConfig()

	if req.STUN != nil {
		config.STUN = *req.STUN
	}
	if req.TURN != nil {
		config.TURN = *req.TURN
	}
	if req.Relay != nil {
		config.Relay = *req.Relay
	}
	if req.DDNS != nil {
		config.DDNS = *req.DDNS
	}
	if req.UPnP != nil {
		config.UPnP = *req.UPnP
	}
	if req.NATPMP != nil {
		config.NATPMP = *req.NATPMP
	}
	if req.Auth != nil {
		config.Auth = *req.Auth
	}
	if req.Encryption != nil {
		config.Encryption = *req.Encryption
	}
	if req.DebugMode != nil {
		config.DebugMode = *req.DebugMode
	}

	if err := h.manager.UpdateConfig(config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Configuration updated successfully"})
}

// ============================================================
// NAT Detection Handlers
// ============================================================

// DetectNAT handles GET /api/v1/remoteaccess/nat/detect.
func (h *Handler) DetectNAT(c *gin.Context) {
	result, err := h.manager.DetectNAT(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetNATStatus handles GET /api/v1/remoteaccess/nat/status.
func (h *Handler) GetNATStatus(c *gin.Context) {
	status := map[string]interface{}{
		"nat_type":  h.manager.state.natType,
		"public_ip": h.manager.state.publicIP,
	}

	c.JSON(http.StatusOK, status)
}

// ============================================================
// P2P Session Handlers
// ============================================================

// CreateP2PSessionRequest is the request body for creating a P2P session.
type CreateP2PSessionRequest struct {
	PeerID string `json:"peer_id" binding:"required"`
}

// CreateP2PSession handles POST /api/v1/remoteaccess/p2p/sessions.
func (h *Handler) CreateP2PSession(c *gin.Context) {
	var req CreateP2PSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.manager.CreateP2PSession(c.Request.Context(), req.PeerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, session)
}

// ListP2PSessions handles GET /api/v1/remoteaccess/p2p/sessions.
func (h *Handler) ListP2PSessions(c *gin.Context) {
	sessions := h.manager.ListP2PSessions()
	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"total":    len(sessions),
	})
}

// GetP2PSession handles GET /api/v1/remoteaccess/p2p/sessions/:id.
func (h *Handler) GetP2PSession(c *gin.Context) {
	id := c.Param("id")

	session, exists := h.manager.GetP2PSession(id)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "P2P session not found"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// CloseP2PSession handles DELETE /api/v1/remoteaccess/p2p/sessions/:id.
func (h *Handler) CloseP2PSession(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.CloseP2PSession(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "P2P session closed"})
}

// ============================================================
// Relay Session Handlers
// ============================================================

// CreateRelaySessionRequest is the request body for creating a relay session.
type CreateRelaySessionRequest struct {
	ClientID string `json:"client_id" binding:"required"`
}

// CreateRelaySession handles POST /api/v1/remoteaccess/relay/sessions.
func (h *Handler) CreateRelaySession(c *gin.Context) {
	var req CreateRelaySessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.manager.CreateRelaySession(c.Request.Context(), req.ClientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, session)
}

// ListRelaySessions handles GET /api/v1/remoteaccess/relay/sessions.
func (h *Handler) ListRelaySessions(c *gin.Context) {
	sessions := h.manager.ListRelaySessions()
	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"total":    len(sessions),
	})
}

// GetRelaySession handles GET /api/v1/remoteaccess/relay/sessions/:id.
func (h *Handler) GetRelaySession(c *gin.Context) {
	id := c.Param("id")

	session, exists := h.manager.GetRelaySession(id)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Relay session not found"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// CloseRelaySession handles DELETE /api/v1/remoteaccess/relay/sessions/:id.
func (h *Handler) CloseRelaySession(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.CloseRelaySession(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Relay session closed"})
}

// ============================================================
// DDNS Handlers
// ============================================================

// GetDDNSStatus handles GET /api/v1/remoteaccess/ddns/status.
func (h *Handler) GetDDNSStatus(c *gin.Context) {
	status := h.manager.GetDDNSStatus()
	if status == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "DDNS status not available"})
		return
	}

	c.JSON(http.StatusOK, status)
}

// UpdateDDNS handles POST /api/v1/remoteaccess/ddns/update.
func (h *Handler) UpdateDDNS(c *gin.Context) {
	if err := h.manager.UpdateDDNS(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "DDNS update initiated"})
}

// ============================================================
// Port Mapping Handlers
// ============================================================

// CreatePortMappingRequest is the request body for creating a port mapping.
type CreatePortMappingRequest struct {
	Protocol     string `json:"protocol" binding:"required,oneof=tcp udp"`
	ExternalPort int    `json:"external_port" binding:"required,min=1,max=65535"`
	InternalPort int    `json:"internal_port" binding:"required,min=1,max=65535"`
	InternalIP   string `json:"internal_ip"`
	Description  string `json:"description"`
	Enabled      *bool  `json:"enabled"`
	Method       string `json:"method" binding:"omitempty,oneof=upnp natpmp"`
	LeaseTime    int    `json:"lease_time"`
}

// CreatePortMapping handles POST /api/v1/remoteaccess/portmappings.
func (h *Handler) CreatePortMapping(c *gin.Context) {
	var req CreatePortMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	method := PortMappingUPnP
	if req.Method == "natpmp" {
		method = PortMappingNATPMP
	}

	mapping := &PortMapping{
		Protocol:     PortMappingProtocol(req.Protocol),
		ExternalPort: req.ExternalPort,
		InternalPort: req.InternalPort,
		InternalIP:   req.InternalIP,
		Description:  req.Description,
		Enabled:      enabled,
		Method:       method,
		LeaseTime:    req.LeaseTime,
	}

	if err := h.manager.CreatePortMapping(c.Request.Context(), mapping); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, mapping)
}

// ListPortMappings handles GET /api/v1/remoteaccess/portmappings.
func (h *Handler) ListPortMappings(c *gin.Context) {
	mappings := h.manager.ListPortMappings()
	c.JSON(http.StatusOK, gin.H{
		"port_mappings": mappings,
		"total":         len(mappings),
	})
}

// GetPortMapping handles GET /api/v1/remoteaccess/portmappings/:id.
func (h *Handler) GetPortMapping(c *gin.Context) {
	id := c.Param("id")

	mapping, exists := h.manager.GetPortMapping(id)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Port mapping not found"})
		return
	}

	c.JSON(http.StatusOK, mapping)
}

// DeletePortMapping handles DELETE /api/v1/remoteaccess/portmappings/:id.
func (h *Handler) DeletePortMapping(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeletePortMapping(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Port mapping deleted"})
}

// RefreshPortMappings handles POST /api/v1/remoteaccess/portmappings/refresh.
func (h *Handler) RefreshPortMappings(c *gin.Context) {
	if err := h.manager.RefreshPortMappings(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Port mappings refreshed"})
}

// ============================================================
// Connection Handlers
// ============================================================

// ListConnections handles GET /api/v1/remoteaccess/connections.
func (h *Handler) ListConnections(c *gin.Context) {
	connections := h.manager.ListConnections()
	c.JSON(http.StatusOK, gin.H{
		"connections": connections,
		"total":       len(connections),
	})
}

// GetConnectionStats handles GET /api/v1/remoteaccess/connections/stats.
func (h *Handler) GetConnectionStats(c *gin.Context) {
	stats := h.manager.GetConnectionStats()
	c.JSON(http.StatusOK, stats)
}

// GetConnection handles GET /api/v1/remoteaccess/connections/:id.
func (h *Handler) GetConnection(c *gin.Context) {
	id := c.Param("id")

	conn, exists := h.manager.GetConnection(id)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Connection not found"})
		return
	}

	c.JSON(http.StatusOK, conn)
}

// CloseConnection handles DELETE /api/v1/remoteaccess/connections/:id.
func (h *Handler) CloseConnection(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.CloseConnection(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Connection closed"})
}

// ============================================================
// Session Handlers
// ============================================================

// CreateSessionRequest is the request body for creating a session.
type CreateSessionRequest struct {
	UserID     string `json:"user_id" binding:"required"`
	DeviceID   string `json:"device_id" binding:"required"`
	DeviceName string `json:"device_name"`
	IP         string `json:"ip"`
	UserAgent  string `json:"user_agent"`
}

// CreateSession handles POST /api/v1/remoteaccess/sessions.
func (h *Handler) CreateSession(c *gin.Context) {
	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get IP from request if not provided
	if req.IP == "" {
		req.IP = c.ClientIP()
	}

	session, err := h.manager.CreateSession(req.UserID, req.DeviceID, req.DeviceName, req.IP, req.UserAgent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, session)
}

// ListSessions handles GET /api/v1/remoteaccess/sessions.
func (h *Handler) ListSessions(c *gin.Context) {
	sessions := h.manager.ListSessions()
	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"total":    len(sessions),
	})
}

// GetSession handles GET /api/v1/remoteaccess/sessions/:id.
func (h *Handler) GetSession(c *gin.Context) {
	id := c.Param("id")

	session, exists := h.manager.GetSession(id)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// RefreshSession handles POST /api/v1/remoteaccess/sessions/:id/refresh.
func (h *Handler) RefreshSession(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.RefreshSession(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session refreshed"})
}

// InvalidateSession handles DELETE /api/v1/remoteaccess/sessions/:id.
func (h *Handler) InvalidateSession(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.InvalidateSession(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session invalidated"})
}

// InvalidateAllUserSessions handles DELETE /api/v1/remoteaccess/sessions/user/:user_id.
func (h *Handler) InvalidateAllUserSessions(c *gin.Context) {
	userID := c.Param("user_id")

	count := h.manager.InvalidateAllUserSessions(userID)
	c.JSON(http.StatusOK, gin.H{
		"message": "All user sessions invalidated",
		"count":   count,
	})
}

// ============================================================
// Authentication Handlers
// ============================================================

// LoginRequest is the request body for login.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login handles POST /api/v1/remoteaccess/auth/login.
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := h.manager.Authenticate(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, token)
}

// ValidateTokenRequest is the request body for token validation.
type ValidateTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

// ValidateToken handles POST /api/v1/remoteaccess/auth/validate.
func (h *Handler) ValidateToken(c *gin.Context) {
	var req ValidateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	claims, err := h.manager.ValidateToken(req.Token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, claims)
}

// RefreshTokenRequest is the request body for token refresh.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshToken handles POST /api/v1/remoteaccess/auth/refresh.
func (h *Handler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := h.manager.RefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, token)
}

// RevokeTokenRequest is the request body for token revocation.
type RevokeTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

// RevokeToken handles POST /api/v1/remoteaccess/auth/revoke.
func (h *Handler) RevokeToken(c *gin.Context) {
	var req RevokeTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.RevokeToken(req.Token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Token revoked"})
}

// ============================================================
// Health Handler
// ============================================================

// HealthCheck handles GET /api/v1/remoteaccess/health.
func (h *Handler) HealthCheck(c *gin.Context) {
	stats := h.manager.GetConnectionStats()

	health := map[string]interface{}{
		"status":            "healthy",
		"uptime":            stats.Uptime.String(),
		"active_connections": stats.ActiveConnections,
		"p2p_sessions":      len(h.manager.ListP2PSessions()),
		"relay_sessions":    len(h.manager.ListRelaySessions()),
		"sessions":          len(h.manager.ListSessions()),
		"port_mappings":     len(h.manager.ListPortMappings()),
	}

	c.JSON(http.StatusOK, health)
}

// ============================================================
// Helper functions
// ============================================================

// parseIntParam parses an integer query parameter.
func parseIntParam(c *gin.Context, name string, defaultValue int) int {
	str := c.Query(name)
	if str == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(str)
	if err != nil {
		return defaultValue
	}
	return val
}

// parseBoolParam parses a boolean query parameter.
func parseBoolParam(c *gin.Context, name string, defaultValue bool) bool {
	str := c.Query(name)
	if str == "" {
		return defaultValue
	}
	val, err := strconv.ParseBool(str)
	if err != nil {
		return defaultValue
	}
	return val
}
