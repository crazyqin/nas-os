package ztaccess

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// AuthManager manages authentication
type AuthManager struct {
	zt *ZTAccess
}

// NewAuthManager creates a new auth manager
func NewAuthManager(zt *ZTAccess) *AuthManager {
	return &AuthManager{zt: zt}
}

// TokenClaims represents JWT token claims
type TokenClaims struct {
	UserID      string      `json:"user_id"`
	Username    string      `json:"username"`
	AccessLevel AccessLevel `json:"access_level"`
	DeviceID    string      `json:"device_id"`
	SessionID   string      `json:"session_id"`
	IssuedAt    time.Time   `json:"issued_at"`
	ExpiresAt   time.Time   `json:"expires_at"`
}

// GenerateToken generates a JWT-like token
func (am *AuthManager) GenerateToken(session *Session) (string, error) {
	claims := TokenClaims{
		UserID:      session.UserID,
		AccessLevel: session.AccessLevel,
		DeviceID:    session.Device.DeviceID,
		SessionID:   session.SessionID,
		IssuedAt:    time.Now(),
		ExpiresAt:   session.ExpiresAt,
	}

	// Create signature
	mac := hmac.New(sha256.New, am.zt.jwtSecret)
	mac.Write([]byte(claims.UserID + claims.SessionID + claims.ExpiresAt.String()))
	signature := hex.EncodeToString(mac.Sum(nil))

	// Simplified token format (in production, use proper JWT)
	token := hex.EncodeToString([]byte(claims.UserID)) + "." + signature

	return token, nil
}

// ValidateToken validates a token
func (am *AuthManager) ValidateToken(token string) (*TokenClaims, error) {
	// Simplified validation
	// In production, parse JWT properly
	session, err := am.zt.sessions[token]
	if !err {
		return nil, ErrSessionExpired
	}

	if time.Now().After(session.ExpiresAt) {
		return nil, ErrSessionExpired
	}

	return &TokenClaims{
		UserID:      session.UserID,
		AccessLevel: session.AccessLevel,
		DeviceID:    session.Device.DeviceID,
		SessionID:   session.SessionID,
		IssuedAt:    session.CreatedAt,
		ExpiresAt:   session.ExpiresAt,
	}, nil
}

// RefreshToken refreshes a token
func (am *AuthManager) RefreshToken(sessionID string) (string, error) {
	am.zt.mu.Lock()
	defer am.zt.mu.Unlock()

	session, exists := am.zt.sessions[sessionID]
	if !exists {
		return "", ErrSessionExpired
	}

	if time.Now().After(session.ExpiresAt) {
		return "", ErrSessionExpired
	}

	// Extend session
	session.ExpiresAt = time.Now().Add(am.zt.sessionTTL)
	session.LastActivity = time.Now()

	return am.GenerateToken(session)
}

// ValidateDeviceFingerprint validates device fingerprint
func (am *AuthManager) ValidateDeviceFingerprint(device DeviceInfo) bool {
	// In production, check against known device fingerprints
	if device.Fingerprint == "" {
		return false
	}
	return true
}

// GenerateDeviceFingerprint generates a device fingerprint
func (am *AuthManager) GenerateDeviceFingerprint(device DeviceInfo) string {
	mac := hmac.New(sha256.New, am.zt.jwtSecret)
	mac.Write([]byte(device.DeviceID + device.DeviceType + device.OS + device.Browser))
	return hex.EncodeToString(mac.Sum(nil))
}
