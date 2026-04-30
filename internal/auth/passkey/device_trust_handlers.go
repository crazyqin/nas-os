// Package passkey HTTP handlers for device trust endpoints.
// Routes: /api/v1/auth/passkey/trust/*
package passkey

import (
	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// ========== Device Trust Handlers ==========

// TrustHandlers provides HTTP handlers for device trust management.
type TrustHandlers struct {
	manager *DeviceTrustManager
}

// NewTrustHandlers creates a new device trust handlers instance.
func NewTrustHandlers(mgr *DeviceTrustManager) *TrustHandlers {
	return &TrustHandlers{manager: mgr}
}

// RegisterTrustRoutes registers device trust routes under the passkey group.
func (h *TrustHandlers) RegisterTrustRoutes(authGroup *gin.RouterGroup) {
	g := authGroup.Group("/trust")
	{
		// Trust a device (after TOTP verification)
		g.POST("", h.TrustDevice)

		// Check if current device is trusted
		g.POST("/verify", h.VerifyTrust)

		// List trusted devices for current user
		g.GET("/devices", h.ListDevices)

		// Revoke a specific device trust
		g.DELETE("/devices/:id", h.RevokeDevice)

		// Revoke all trusted devices
		g.POST("/revoke-all", h.RevokeAll)

		// Trust stats
		g.GET("/stats", h.GetStats)

		// Config
		g.GET("/config", h.GetConfig)
	}
}

// TrustDeviceRequest is the HTTP request for trusting a device.
type TrustDeviceRequest struct {
	DeviceName  string `json:"deviceName"`
	DeviceType  string `json:"deviceType"`
	BrowserName string `json:"browserName"`
	BrowserVer  string `json:"browserVersion"`
	OSName      string `json:"osName"`
	OSVersion   string `json:"osVersion"`
	Fingerprint string `json:"fingerprint" binding:"required"`
	TrustDays   int    `json:"trustDays"`
	TOTPCode    string `json:"totpCode"`
}

// TrustDevice handles POST /trust - trust a device.
func (h *TrustHandlers) TrustDevice(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "authentication required")
		return
	}

	var req TrustDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	// NOTE: In production, TOTPCode should be verified against the user's TOTP secret
	// before allowing device trust. This is delegated to the caller/middleware.
	if req.TOTPCode == "" {
		api.BadRequest(c, "totpCode is required to trust a device")
		return
	}

	trustReq := TrustRequest{
		DeviceInfo: DeviceInfo{
			DeviceName:  req.DeviceName,
			DeviceType:  req.DeviceType,
			BrowserName: req.BrowserName,
			BrowserVer:  req.BrowserVer,
			OSName:      req.OSName,
			OSVersion:   req.OSVersion,
			Fingerprint: req.Fingerprint,
			IPAddress:   c.ClientIP(),
		},
		TrustDays: req.TrustDays,
		TOTPCode:  req.TOTPCode,
	}

	device, err := h.manager.TrustDevice(userID, trustReq)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// Return sanitized device info
	api.Created(c, gin.H{
		"deviceId":   device.ID,
		"deviceName": device.DeviceName,
		"expiresAt":  device.ExpiresAt.Format("2006-01-02T15:04:05Z"),
		"message":    "device trusted successfully",
	})
}

// VerifyTrustRequest is the request for verifying device trust.
type VerifyTrustRequest struct {
	Fingerprint string `json:"fingerprint" binding:"required"`
}

// VerifyTrust handles POST /trust/verify - check if a device is trusted.
func (h *TrustHandlers) VerifyTrust(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "authentication required")
		return
	}

	var req VerifyTrustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	result := h.manager.VerifyDeviceTrust(userID, req.Fingerprint)
	api.OK(c, result)
}

// ListDevices handles GET /trust/devices - list trusted devices.
func (h *TrustHandlers) ListDevices(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "authentication required")
		return
	}

	devices := h.manager.GetTrustedDevices(userID)
	api.OK(c, gin.H{
		"devices": devices,
		"total":   len(devices),
	})
}

// RevokeDevice handles DELETE /trust/devices/:id - revoke a device.
func (h *TrustHandlers) RevokeDevice(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "authentication required")
		return
	}

	deviceID := c.Param("id")
	if err := h.manager.RevokeDevice(userID, deviceID); err != nil {
		api.NotFound(c, "device not found")
		return
	}

	api.OK(c, gin.H{"message": "device trust revoked"})
}

// RevokeAllRequest is the request for revoking all devices.
type RevokeAllRequest struct {
	Reason string `json:"reason"`
}

// RevokeAll handles POST /trust/revoke-all - revoke all trusted devices.
func (h *TrustHandlers) RevokeAll(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "authentication required")
		return
	}

	var req RevokeAllRequest
	_ = c.ShouldBindJSON(&req)

	count := h.manager.RevokeAllDevices(userID, req.Reason)
	api.OK(c, gin.H{
		"message": "all device trusts revoked",
		"count":   count,
	})
}

// GetStats handles GET /trust/stats - get device trust statistics.
func (h *TrustHandlers) GetStats(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "authentication required")
		return
	}

	stats := h.manager.Stats(userID)
	api.OK(c, stats)
}

// GetConfig handles GET /trust/config - get trust configuration.
func (h *TrustHandlers) GetConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	api.OK(c, gin.H{
		"trustDurationHours": cfg.TrustDuration.Hours(),
		"maxDevices":         cfg.MaxDevices,
		"requireName":        cfg.RequireName,
		"revokeOnPassword":   cfg.RevokeOnPassword,
	})
}
