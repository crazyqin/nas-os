// Package passkey provides device trust management for WebAuthn/Passkey authentication.
// Implements "remember this device" functionality similar to Synology Secure SignIn,
// allowing users to skip MFA on trusted devices for a configurable period.
//
// Security considerations:
//   - Device trust is bound to a combination of user ID + device fingerprint
//   - Trust tokens are HMAC-signed and time-limited
//   - Trust can be revoked individually or globally per user
//   - Maximum trust duration is configurable (default 30 days)
package passkey

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// ========== Constants ==========

const (
	// DefaultTrustDuration is the default "remember device" period.
	DefaultTrustDuration = 30 * 24 * time.Hour // 30 days
	// MinTrustDuration is the minimum trust duration.
	MinTrustDuration = 1 * time.Hour
	// MaxTrustDuration is the maximum trust duration.
	MaxTrustDuration = 90 * 24 * time.Hour // 90 days
	// MaxTrustedDevicesPerUser is the maximum number of trusted devices per user.
	MaxTrustedDevicesPerUser = 20
)

// ========== Types ==========

// DeviceTrustManager manages trusted device state.
type DeviceTrustManager struct {
	mu        sync.RWMutex
	devices   map[string][]*TrustedDevice // userID -> trusted devices
	hmacKey   []byte                       // HMAC signing key for trust tokens
	config    DeviceTrustConfig
}

// DeviceTrustConfig holds configuration for device trust.
type DeviceTrustConfig struct {
	TrustDuration    time.Duration `json:"trustDuration"`    // How long a device stays trusted
	MaxDevices       int           `json:"maxDevices"`       // Max trusted devices per user
	RequireName      bool          `json:"requireName"`      // Require device name on trust
	RevokeOnPassword bool          `json:"revokeOnPassword"` // Revoke all trusts when password changes
}

// TrustedDevice represents a trusted device for a user.
type TrustedDevice struct {
	ID            string    `json:"id"`            // Unique device trust ID
	UserID        string    `json:"userId"`        // Owner user ID
	DeviceName    string    `json:"deviceName"`    // User-assigned or auto-detected name
	DeviceType    string    `json:"deviceType"`    // desktop, mobile, tablet
	BrowserName   string    `json:"browserName"`   // Chrome, Firefox, Safari, etc.
	BrowserVer    string    `json:"browserVersion"` // Browser version
	OSName        string    `json:"osName"`        // Windows, macOS, Linux, iOS, Android
	OSVersion     string    `json:"osVersion"`     // OS version
	Fingerprint   string    `json:"-"`             // Device fingerprint (not exposed via API)
	TrustToken    string    `json:"-"`             // HMAC-signed trust token (not exposed)
	IPAddress     string    `json:"ipAddress"`     // IP address at time of trust
	TrustedAt     time.Time `json:"trustedAt"`     // When trust was established
	ExpiresAt     time.Time `json:"expiresAt"`     // When trust expires
	LastUsedAt    time.Time `json:"lastUsedAt"`    // Last time device was used
	Revoked       bool      `json:"revoked"`       // Whether trust has been revoked
	RevokedAt     *time.Time `json:"revokedAt,omitempty"`
	RevokedReason string    `json:"revokedReason,omitempty"`
}

// DeviceInfo holds device information sent by the client.
type DeviceInfo struct {
	DeviceName  string `json:"deviceName"`
	DeviceType  string `json:"deviceType"`  // desktop, mobile, tablet
	BrowserName string `json:"browserName"`
	BrowserVer  string `json:"browserVersion"`
	OSName      string `json:"osName"`
	OSVersion   string `json:"osVersion"`
	Fingerprint string `json:"fingerprint"` // Browser fingerprint hash
	IPAddress   string `json:"-"`           // Set server-side
}

// TrustRequest represents a request to trust a device.
type TrustRequest struct {
	DeviceInfo   DeviceInfo `json:"deviceInfo"`
	TrustDays    int        `json:"trustDays"`    // 1-90 days, 0 = default
	TOTPCode     string     `json:"totpCode"`     // Required: must verify TOTP to trust device
}

// TrustVerificationResult is the result of checking if a device is trusted.
type TrustVerificationResult struct {
	Trusted    bool   `json:"trusted"`
	DeviceID   string `json:"deviceId,omitempty"`
	DeviceName string `json:"deviceName,omitempty"`
	ExpiresAt  string `json:"expiresAt,omitempty"`
	Reason     string `json:"reason,omitempty"` // Why not trusted
}

// DefaultDeviceTrustConfig returns production defaults.
func DefaultDeviceTrustConfig() DeviceTrustConfig {
	return DeviceTrustConfig{
		TrustDuration:    DefaultTrustDuration,
		MaxDevices:       MaxTrustedDevicesPerUser,
		RequireName:      false,
		RevokeOnPassword: true,
	}
}

// ========== Manager ==========

// NewDeviceTrustManager creates a new device trust manager.
func NewDeviceTrustManager(cfg DeviceTrustConfig, hmacKey []byte) *DeviceTrustManager {
	if cfg.TrustDuration == 0 {
		cfg.TrustDuration = DefaultTrustDuration
	}
	if cfg.MaxDevices == 0 {
		cfg.MaxDevices = MaxTrustedDevicesPerUser
	}
	if cfg.TrustDuration < MinTrustDuration {
		cfg.TrustDuration = MinTrustDuration
	}
	if cfg.TrustDuration > MaxTrustDuration {
		cfg.TrustDuration = MaxTrustDuration
	}
	if len(hmacKey) == 0 {
		// Generate a random key (caller should provide one)
		key, _ := generateRandom(32)
		hmacKey = key
	}
	return &DeviceTrustManager{
		devices: make(map[string][]*TrustedDevice),
		hmacKey: hmacKey,
		config:  cfg,
	}
}

// TrustDevice establishes trust for a device.
func (m *DeviceTrustManager) TrustDevice(userID string, req TrustRequest) (*TrustedDevice, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID required")
	}

	fp := req.DeviceInfo.Fingerprint
	if fp == "" {
		return nil, fmt.Errorf("device fingerprint required")
	}

	// Determine trust duration
	duration := m.config.TrustDuration
	if req.TrustDays > 0 {
		d := time.Duration(req.TrustDays) * 24 * time.Hour
		if d < MinTrustDuration {
			d = MinTrustDuration
		}
		if d > MaxTrustDuration {
			d = MaxTrustDuration
		}
		duration = d
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check max devices
	devices := m.devices[userID]
	activeCount := 0
	for _, d := range devices {
		if !d.Revoked && time.Now().Before(d.ExpiresAt) {
			activeCount++
		}
	}

	if activeCount >= m.config.MaxDevices {
		return nil, fmt.Errorf("maximum trusted devices (%d) reached; revoke an existing device first", m.config.MaxDevices)
	}

	// Check if this fingerprint is already trusted
	for _, d := range devices {
		if d.Fingerprint == fp && !d.Revoked && time.Now().Before(d.ExpiresAt) {
			// Already trusted - refresh the trust period
			now := time.Now()
			d.ExpiresAt = now.Add(duration)
			d.LastUsedAt = now
			if req.DeviceInfo.IPAddress != "" {
				d.IPAddress = req.DeviceInfo.IPAddress
			}
			if req.DeviceInfo.DeviceName != "" {
				d.DeviceName = req.DeviceInfo.DeviceName
			}
			return d, nil
		}
	}

	// Generate trust token
	trustToken, err := m.generateTrustToken(userID, fp)
	if err != nil {
		return nil, fmt.Errorf("generate trust token: %w", err)
	}

	now := time.Now()
	device := &TrustedDevice{
		ID:          generateDeviceTrustID(),
		UserID:      userID,
		DeviceName:  req.DeviceInfo.DeviceName,
		DeviceType:  req.DeviceInfo.DeviceType,
		BrowserName: req.DeviceInfo.BrowserName,
		BrowserVer:  req.DeviceInfo.BrowserVer,
		OSName:      req.DeviceInfo.OSName,
		OSVersion:   req.DeviceInfo.OSVersion,
		Fingerprint: fp,
		TrustToken:  trustToken,
		IPAddress:   req.DeviceInfo.IPAddress,
		TrustedAt:   now,
		ExpiresAt:   now.Add(duration),
		LastUsedAt:  now,
	}

	if device.DeviceName == "" {
		device.DeviceName = generateAutoDeviceName(req.DeviceInfo)
	}

	m.devices[userID] = append(m.devices[userID], device)
	return device, nil
}

// VerifyDeviceTrust checks if a device is trusted.
func (m *DeviceTrustManager) VerifyDeviceTrust(userID, fingerprint string) *TrustVerificationResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := &TrustVerificationResult{
		Trusted: false,
		Reason:  "device not trusted",
	}

	if userID == "" || fingerprint == "" {
		result.Reason = "missing user ID or fingerprint"
		return result
	}

	devices := m.devices[userID]
	now := time.Now()

	for _, d := range devices {
		if d.Fingerprint == fingerprint {
			if d.Revoked {
				result.Reason = "device trust revoked"
				return result
			}
			if now.After(d.ExpiresAt) {
				result.Reason = "device trust expired"
				return result
			}
			// Trusted!
			result.Trusted = true
			result.DeviceID = d.ID
			result.DeviceName = d.DeviceName
			result.ExpiresAt = d.ExpiresAt.Format(time.RFC3339)
			return result
		}
	}

	return result
}

// UpdateLastUsed updates the last-used timestamp for a trusted device.
func (m *DeviceTrustManager) UpdateLastUsed(userID, fingerprint string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, d := range m.devices[userID] {
		if d.Fingerprint == fingerprint && !d.Revoked {
			d.LastUsedAt = time.Now()
			return
		}
	}
}

// RevokeDevice revokes trust for a specific device.
func (m *DeviceTrustManager) RevokeDevice(userID, deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, d := range m.devices[userID] {
		if d.ID == deviceID {
			now := time.Now()
			d.Revoked = true
			d.RevokedAt = &now
			d.RevokedReason = "user_revoked"
			return nil
		}
	}
	return fmt.Errorf("device not found")
}

// RevokeAllDevices revokes all trusted devices for a user.
func (m *DeviceTrustManager) RevokeAllDevices(userID string, reason string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if reason == "" {
		reason = "user_revoked_all"
	}

	count := 0
	now := time.Now()
	for _, d := range m.devices[userID] {
		if !d.Revoked {
			d.Revoked = true
			d.RevokedAt = &now
			d.RevokedReason = reason
			count++
		}
	}
	return count
}

// GetTrustedDevices returns all trusted (non-revoked, non-expired) devices for a user.
func (m *DeviceTrustManager) GetTrustedDevices(userID string) []*TrustedDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	var result []*TrustedDevice
	for _, d := range m.devices[userID] {
		if !d.Revoked && now.Before(d.ExpiresAt) {
			// Return a copy with sensitive fields hidden
			copy := *d
			copy.Fingerprint = ""
			copy.TrustToken = ""
			result = append(result, &copy)
		}
	}
	return result
}

// GetAllDevices returns all devices (including revoked/expired) for a user.
func (m *DeviceTrustManager) GetAllDevices(userID string) []*TrustedDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*TrustedDevice
	for _, d := range m.devices[userID] {
		copy := *d
		copy.Fingerprint = ""
		copy.TrustToken = ""
		result = append(result, &copy)
	}
	return result
}

// CleanupExpired removes expired and revoked devices.
func (m *DeviceTrustManager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	removed := 0
	now := time.Now()
	for userID, devices := range m.devices {
		var active []*TrustedDevice
		for _, d := range devices {
			// Keep if not expired and not revoked, or if revoked recently (for audit)
			if (!d.Revoked && now.Before(d.ExpiresAt)) ||
				(d.Revoked && d.RevokedAt != nil && now.Sub(*d.RevokedAt) < 7*24*time.Hour) {
				active = append(active, d)
			} else {
				removed++
			}
		}
		m.devices[userID] = active
	}
	return removed
}

// Stats returns device trust statistics.
func (m *DeviceTrustManager) Stats(userID string) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := m.devices[userID]
	now := time.Now()
	active := 0
	expired := 0
	revoked := 0
	for _, d := range devices {
		switch {
		case d.Revoked:
			revoked++
		case now.After(d.ExpiresAt):
			expired++
		default:
			active++
		}
	}

	return map[string]interface{}{
		"total":           len(devices),
		"active":          active,
		"expired":         expired,
		"revoked":         revoked,
		"maxDevices":      m.config.MaxDevices,
		"trustDurationH":  m.config.TrustDuration.Hours(),
	}
}

// ========== Internal ==========

// generateTrustToken creates an HMAC-SHA256 trust token.
func (m *DeviceTrustManager) generateTrustToken(userID, fingerprint string) (string, error) {
	message := userID + "|" + fingerprint
	mac := hmac.New(sha256.New, m.hmacKey)
	if _, err := mac.Write([]byte(message)); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// generateDeviceTrustID generates a unique ID for a device trust entry.
func generateDeviceTrustID() string {
	b, _ := generateRandom(16)
	return base64.RawURLEncoding.EncodeToString(b)
}

// generateAutoDeviceName creates an automatic device name from device info.
func generateAutoDeviceName(info DeviceInfo) string {
	name := info.BrowserName
	if name == "" {
		name = info.DeviceType
	}
	if name == "" {
		name = "Unknown Device"
	}
	if info.OSName != "" {
		name += " on " + info.OSName
	}
	return name
}

// GetConfig returns the device trust configuration.
func (m *DeviceTrustManager) GetConfig() DeviceTrustConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig updates the device trust configuration.
func (m *DeviceTrustManager) UpdateConfig(cfg DeviceTrustConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg.TrustDuration > 0 {
		if cfg.TrustDuration < MinTrustDuration {
			cfg.TrustDuration = MinTrustDuration
		}
		if cfg.TrustDuration > MaxTrustDuration {
			cfg.TrustDuration = MaxTrustDuration
		}
		m.config.TrustDuration = cfg.TrustDuration
	}
	if cfg.MaxDevices > 0 {
		m.config.MaxDevices = cfg.MaxDevices
	}
	m.config.RequireName = cfg.RequireName
	m.config.RevokeOnPassword = cfg.RevokeOnPassword
}
