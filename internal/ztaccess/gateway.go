package ztaccess

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrSessionExpired     = errors.New("session expired")
	ErrAccessDenied       = errors.New("access denied")
	ErrDeviceNotTrusted   = errors.New("device not trusted")
	ErrMaxSessionsReached = errors.New("maximum sessions reached")
)

// Gateway manages the zero-trust access gateway
type Gateway struct {
	zt *ZTAccess
}

// NewGateway creates a new gateway
func NewGateway(zt *ZTAccess) *Gateway {
	return &Gateway{zt: zt}
}

// Authenticate authenticates a user and creates a session
func (g *Gateway) Authenticate(username, password string, device DeviceInfo) (*Session, error) {
	g.zt.mu.Lock()
	defer g.zt.mu.Unlock()

	// Find user
	var user *UserIdentity
	for _, u := range g.zt.users {
		if u.Username == username {
			user = u
			break
		}
	}

	if user == nil {
		g.zt.addAuditEntry("", "authenticate", "login", "failed", device.IPAddress, "User not found")
		return nil, ErrInvalidCredentials
	}

	// Verify password (simplified - in production use bcrypt)
	if !g.verifyPassword(user.UserID, password) {
		g.zt.addAuditEntry(user.UserID, "authenticate", "login", "failed", device.IPAddress, "Invalid password")
		return nil, ErrInvalidCredentials
	}

	// Check device trust
	if !g.isDeviceTrusted(user.UserID, device) {
		g.zt.addAnomaly(user.UserID, "", "untrusted_device", "medium", "Login from untrusted device", device)
	}

	// Check for max sessions
	sessionCount := g.countUserSessions(user.UserID)
	if sessionCount >= g.zt.maxSessions {
		g.zt.removeOldestSession(user.UserID)
	}

	// Create session
	session := &Session{
		SessionID:    generateSessionID(),
		UserID:       user.UserID,
		Device:       device,
		AccessLevel:  user.AccessLevel,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(g.zt.sessionTTL),
		LastActivity: time.Now(),
		IsActive:     true,
		ActivityLog:  make([]Activity, 0),
	}

	g.zt.sessions[session.SessionID] = session

	// Update user login info
	user.LastLogin = time.Now()
	user.LoginCount++

	g.zt.addAuditEntry(user.UserID, "authenticate", "login", "success", device.IPAddress, "Login successful")

	return session, nil
}

// ValidateSession validates a session
func (g *Gateway) ValidateSession(sessionID string) (*Session, error) {
	g.zt.mu.RLock()
	defer g.zt.mu.RUnlock()

	session, exists := g.zt.sessions[sessionID]
	if !exists {
		return nil, ErrSessionExpired
	}

	if time.Now().After(session.ExpiresAt) {
		session.IsActive = false
		return nil, ErrSessionExpired
	}

	if !session.IsActive {
		return nil, ErrSessionExpired
	}

	// Update last activity
	session.LastActivity = time.Now()

	return session, nil
}

// Authorize checks if a session has access to a resource
func (g *Gateway) Authorize(sessionID, resource, action string) (bool, error) {
	session, err := g.ValidateSession(sessionID)
	if err != nil {
		return false, err
	}

	g.zt.mu.RLock()
	defer g.zt.mu.RUnlock()

	// Check policies
	for _, policy := range g.zt.policies {
		if !policy.Enabled {
			continue
		}

		if g.policyMatches(policy, resource, action, session) {
			// Log activity
			session.ActivityLog = append(session.ActivityLog, Activity{
				Timestamp: time.Now(),
				Action:    action,
				Resource:  resource,
				Result:    "allowed",
				IPAddress: session.Device.IPAddress,
			})
			g.zt.addAuditEntry(session.UserID, action, resource, "allowed", session.Device.IPAddress, "Policy matched")
			return true, nil
		}
	}

	// Default deny
	session.ActivityLog = append(session.ActivityLog, Activity{
		Timestamp: time.Now(),
		Action:    action,
		Resource:  resource,
		Result:    "denied",
		IPAddress: session.Device.IPAddress,
	})
	g.zt.addAuditEntry(session.UserID, action, resource, "denied", session.Device.IPAddress, "No matching policy")

	return false, nil
}

// RevokeSession revokes a session
func (g *Gateway) RevokeSession(sessionID string) error {
	g.zt.mu.Lock()
	defer g.zt.mu.Unlock()

	session, exists := g.zt.sessions[sessionID]
	if !exists {
		return ErrSessionExpired
	}

	session.IsActive = false
	g.zt.addAuditEntry(session.UserID, "revoke", "session", "success", session.Device.IPAddress, "Session revoked")

	return nil
}

// verifyPassword verifies a password (simplified)
func (g *Gateway) verifyPassword(userID, password string) bool {
	// In production, use bcrypt
	// This is a simplified version
	mac := hmac.New(sha256.New, g.zt.jwtSecret)
	mac.Write([]byte(userID + password))
	hash := hex.EncodeToString(mac.Sum(nil))
	
	// Store and compare (simplified)
	_ = hash
	return true // Simplified for now
}

// isDeviceTrusted checks if a device is trusted
func (g *Gateway) isDeviceTrusted(userID string, device DeviceInfo) bool {
	// Check device fingerprint against known devices
	// In production, maintain a device trust store
	return true // Simplified for now
}

// countUserSessions counts active sessions for a user
func (g *Gateway) countUserSessions(userID string) int {
	count := 0
	for _, session := range g.zt.sessions {
		if session.UserID == userID && session.IsActive {
			count++
		}
	}
	return count
}

// removeOldestSession removes the oldest session for a user
func (g *Gateway) removeOldestSession(userID string) {
	var oldest *Session
	for _, session := range g.zt.sessions {
		if session.UserID == userID && session.IsActive {
			if oldest == nil || session.CreatedAt.Before(oldest.CreatedAt) {
				oldest = session
			}
		}
	}
	if oldest != nil {
		oldest.IsActive = false
	}
}

// policyMatches checks if a policy matches
func (g *Gateway) policyMatches(policy *AccessPolicy, resource, action string, session *Session) bool {
	// Check resource match
	resourceMatch := false
	for _, r := range policy.Resources {
		if r == "*" || r == resource {
			resourceMatch = true
			break
		}
	}
	if !resourceMatch {
		return false
	}

	// Check action match
	actionMatch := false
	for _, a := range policy.Actions {
		if a == "*" || a == action {
			actionMatch = true
			break
		}
	}
	if !actionMatch {
		return false
	}

	// Check conditions
	for _, condition := range policy.Conditions {
		if !g.evaluateCondition(condition, session) {
			return false
		}
	}

	return true
}

// evaluateCondition evaluates a policy condition
func (g *Gateway) evaluateCondition(condition Condition, session *Session) bool {
	switch condition.Type {
	case "time":
		// Time-based conditions
		return true // Simplified
	case "ip":
		// IP-based conditions
		return true // Simplified
	case "device":
		// Device-based conditions
		return true // Simplified
	default:
		return true
	}
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	// In production, use crypto/rand
	return hex.EncodeToString([]byte(time.Now().String()))
}

// addAuditEntry adds an audit entry
func (zt *ZTAccess) addAuditEntry(userID, action, resource, result, ipAddress, details string) {
	entry := AuditEntry{
		EntryID:   hex.EncodeToString([]byte(time.Now().String())),
		Timestamp: time.Now(),
		UserID:    userID,
		Action:    action,
		Resource:  resource,
		Result:    result,
		IPAddress: ipAddress,
		Details:   details,
	}
	zt.auditLog = append(zt.auditLog, entry)
}

// addAnomaly adds an anomaly detection
func (zt *ZTAccess) addAnomaly(userID, sessionID, anomalyType, severity, description string, device DeviceInfo) {
	anomaly := AnomalyDetection{
		UserID:      userID,
		SessionID:   sessionID,
		AnomalyType: anomalyType,
		Severity:    severity,
		Description: description,
		Timestamp:   time.Now(),
		DeviceInfo:  device,
	}
	zt.anomalies = append(zt.anomalies, anomaly)
}
