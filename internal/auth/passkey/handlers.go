// Package passkey provides HTTP handlers for the Passkey/WebAuthn API.
// Routes are mounted under /api/v1/auth/passkey/
package passkey

import (
	"nas-os/internal/api"
	"nas-os/internal/auth"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ========== Request/Response Types ==========

// RegisterStartRequest starts a registration ceremony.
type RegisterStartRequest struct {
	Username    string `json:"username" binding:"required"`
	DisplayName string `json:"displayName"`
	// UserID is optional; if not provided, derived from username lookup.
	UserID string `json:"userId"`
}

// RegisterStartResponse is returned by the register-start endpoint.
type RegisterStartResponse struct {
	SessionID string                 `json:"sessionId"`
	Options   map[string]interface{} `json:"options"`
	ExpiresIn int                    `json:"expiresIn"` // seconds
}

// RegisterFinishRequest completes a registration ceremony.
type RegisterFinishRequest struct {
	SessionID string                 `json:"sessionId" binding:"required"`
	Response  map[string]interface{} `json:"response" binding:"required"`
}

// RegisterFinishResponse is returned after successful registration.
type RegisterFinishResponse struct {
	CredentialID string `json:"credentialId"`
	Name         string `json:"name"`
	IsPasskey    bool   `json:"isPasskey"`
	CreatedAt    string `json:"createdAt"`
}

// AuthStartRequest starts an authentication ceremony.
type AuthStartRequest struct {
	Username string `json:"username"` // required for non-discoverable; omit for auto-fill
	// UserID may be used instead of Username
	UserID string `json:"userId"`
}

// AuthStartResponse is returned by the auth-start endpoint.
type AuthStartResponse struct {
	SessionID string                 `json:"sessionId"`
	Options   map[string]interface{} `json:"options"`
	ExpiresIn int                    `json:"expiresIn"`
	AutoFill  bool                   `json:"autoFill"` // true if browser will auto-select credential
}

// AuthFinishRequest completes an authentication ceremony.
type AuthFinishRequest struct {
	SessionID string                 `json:"sessionId" binding:"required"`
	Response  map[string]interface{} `json:"response" binding:"required"`
}

// AuthFinishResponse is returned after successful authentication.
type AuthFinishResponse struct {
	UserID     string `json:"userId"`
	Username   string `json:"username"`
	NewAuthTime string `json:"newAuthTime"`
	DeviceType  string `json:"deviceType"`
}

// CredentialInfo represents a stored passkey for the UI.
type CredentialInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	DeviceType  string   `json:"deviceType"`
	Transport   []string `json:"transport"`
	IsPasskey   bool     `json:"isPasskey"`
	BackupState string   `json:"backupState"`
	CreatedAt   string   `json:"createdAt"`
	LastUsedAt  string   `json:"lastUsedAt,omitempty"`
}

// ========== Handlers ==========

// Handlers holds HTTP handlers for Passkey endpoints.
type Handlers struct {
	manager     *Manager
	userService UserService
}

// UserService abstracts user lookup (to avoid circular deps).
type UserService interface {
	GetUserByUsername(username string) (*auth.User, error)
	GetUserByID(id string) (*auth.User, error)
}

// NewHandlers creates a new Passkey handlers instance.
func NewHandlers(mgr *Manager, userSvc UserService) *Handlers {
	return &Handlers{manager: mgr, userService: userSvc}
}

// RegisterRoutes registers passkey routes under the given auth group.
func (h *Handlers) RegisterRoutes(authGroup *gin.RouterGroup) {
	g := authGroup.Group("/passkey")
	{
		// Public endpoints (no auth required, for login flow)
		g.POST("/register-start", h.RegisterStart)
		g.POST("/register-finish", h.RegisterFinish)
		g.POST("/auth-start", h.AuthStart)
		g.POST("/auth-finish", h.AuthFinish)

		// Authenticated endpoints (require existing session)
		g.GET("/credentials", h.RequireAuth(h.ListCredentials))
		g.DELETE("/credentials/:id", h.RequireAuth(h.DeleteCredential))
		g.PATCH("/credentials/:id", h.RequireAuth(h.RenameCredential))
		g.GET("/stats", h.RequireAuth(h.GetStats))
	}
}

// ========== Registration Handlers ==========

// RegisterStart begins the registration ceremony.
// POST /api/v1/auth/passkey/register-start
func (h *Handlers) RegisterStart(c *gin.Context) {
	var req RegisterStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	// Look up user to verify they exist
	user, err := h.userService.GetUserByUsername(req.Username)
	if err != nil {
		api.Unauthorized(c, "user not found")
		return
	}
	userID := user.ID

	displayName := req.DisplayName
	if displayName == "" {
		displayName = user.Username
	}

	sessionID, options, err := h.manager.RegistrationOptions(userID, user.Username, displayName)
	if err != nil {
		api.InternalError(c, "failed to generate registration options: "+err.Error())
		return
	}

	api.OK(c, RegisterStartResponse{
		SessionID: sessionID,
		Options:   options,
		ExpiresIn: 300, // 5 minutes
	})
}

// RegisterFinish completes the registration ceremony.
// POST /api/v1/auth/passkey/register-finish
func (h *Handlers) RegisterFinish(c *gin.Context) {
	var req RegisterFinishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	cred, err := h.manager.VerifyRegistration(req.SessionID, req.Response)
	if err != nil {
		api.Unauthorized(c, "registration verification failed: "+err.Error())
		return
	}

	api.OK(c, RegisterFinishResponse{
		CredentialID: cred.ID,
		Name:         cred.Name,
		IsPasskey:    cred.IsPasskey,
		CreatedAt:    cred.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// ========== Authentication Handlers ==========

// AuthStart begins the authentication ceremony.
// POST /api/v1/auth/passkey/auth-start
func (h *Handlers) AuthStart(c *gin.Context) {
	var req AuthStartRequest
	// Body is optional; if omitted, auto-fill mode is used
	_ = c.ShouldBindJSON(&req)

	var sessionID string
	var options map[string]interface{}
	var autoFill bool
	var err error

	userID := req.UserID
	if userID == "" && req.Username != "" {
		// Look up user by username
		user, lookupErr := h.userService.GetUserByUsername(req.Username)
		if lookupErr != nil {
			api.Unauthorized(c, "user not found")
			return
		}
		userID = user.ID
	}

	if userID == "" {
		// Auto-fill / discoverable credential mode
		sessionID, options, err = h.manager.AuthenticationOptionsAuto()
		autoFill = true
	} else {
		sessionID, options, err = h.manager.AuthenticationOptions(userID)
	}
	if err != nil {
		api.InternalError(c, "failed to generate auth options: "+err.Error())
		return
	}

	api.OK(c, AuthStartResponse{
		SessionID: sessionID,
		Options:   options,
		ExpiresIn: 60,
		AutoFill:  autoFill,
	})
}

// AuthFinish completes the authentication ceremony.
// POST /api/v1/auth/passkey/auth-finish
func (h *Handlers) AuthFinish(c *gin.Context) {
	var req AuthFinishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	userID, authInfo, err := h.manager.VerifyAuthentication(req.SessionID, req.Response)
	if err != nil {
		api.Unauthorized(c, "authentication failed: "+err.Error())
		return
	}

	// Look up user info
	var username string
	if h.userService != nil {
		if user, lookupErr := h.userService.GetUserByID(userID); lookupErr == nil {
			username = user.Username
		}
	}

	deviceType := ""
	if authInfo != nil {
		deviceType = authInfo.DeviceType
	}

	api.OK(c, AuthFinishResponse{
		UserID:      userID,
		Username:    username,
		NewAuthTime: "now",
		DeviceType:  deviceType,
	})
}

// ========== Credential Management Handlers ==========

// ListCredentials lists all passkeys for the current user.
// GET /api/v1/auth/passkey/credentials
func (h *Handlers) ListCredentials(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "unauthorized")
		return
	}

	creds := h.manager.GetCredentials(userID)
	result := make([]CredentialInfo, len(creds))
	for i, cred := range creds {
		lastUsed := ""
		if cred.LastUsedAt != nil {
			lastUsed = cred.LastUsedAt.Format("2006-01-02T15:04:05Z")
		}
		result[i] = CredentialInfo{
			ID:          cred.ID,
			Name:        cred.Name,
			DeviceType:  cred.DeviceType,
			Transport:   cred.Transport,
			IsPasskey:   cred.IsPasskey,
			BackupState: cred.BackupState,
			CreatedAt:   cred.CreatedAt.Format("2006-01-02T15:04:05Z"),
			LastUsedAt:  lastUsed,
		}
	}

	api.OK(c, result)
}

// DeleteCredential removes a passkey.
// DELETE /api/v1/auth/passkey/credentials/:id
func (h *Handlers) DeleteCredential(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "unauthorized")
		return
	}

	credID := c.Param("id")
	if err := h.manager.RemoveCredential(userID, credID); err != nil {
		api.NotFound(c, "credential not found")
		return
	}

	api.OK(c, gin.H{"message": "passkey deleted"})
}

// RenameCredential updates the friendly name of a passkey.
// PATCH /api/v1/auth/passkey/credentials/:id
func (h *Handlers) RenameCredential(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "unauthorized")
		return
	}

	credID := c.Param("id")
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "name required")
		return
	}

	if err := h.manager.RenameCredential(userID, credID, req.Name); err != nil {
		api.NotFound(c, "credential not found")
		return
	}

	api.OK(c, gin.H{"message": "passkey renamed"})
}

// GetStats returns passkey statistics for the current user.
// GET /api/v1/auth/passkey/stats
func (h *Handlers) GetStats(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "unauthorized")
		return
	}

	stats := h.manager.Stats(userID)
	api.OK(c, stats)
}

// ========== Middleware ==========

// RequireAuth is a middleware that ensures the request has a valid session.
func (h *Handlers) RequireAuth(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			api.Unauthorized(c, "authentication required")
			c.Abort()
			return
		}
		handler(c)
	}
}

// Ensure uuid import is used
var _ = uuid.New
