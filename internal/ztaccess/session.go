package ztaccess

import (
	"time"
)

// SessionManager manages user sessions
type SessionManager struct {
	zt *ZTAccess
}

// NewSessionManager creates a new session manager
func NewSessionManager(zt *ZTAccess) *SessionManager {
	return &SessionManager{zt: zt}
}

// CreateSession creates a new session
func (sm *SessionManager) CreateSession(userID string, device DeviceInfo, level AccessLevel) *Session {
	sm.zt.mu.Lock()
	defer sm.zt.mu.Unlock()

	session := &Session{
		SessionID:    generateSessionID(),
		UserID:       userID,
		Device:       device,
		AccessLevel:  level,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(sm.zt.sessionTTL),
		LastActivity: time.Now(),
		IsActive:     true,
		ActivityLog:  make([]Activity, 0),
	}

	sm.zt.sessions[session.SessionID] = session
	return session
}

// GetSession returns a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*Session, bool) {
	sm.zt.mu.RLock()
	defer sm.zt.mu.RUnlock()

	session, exists := sm.zt.sessions[sessionID]
	return session, exists
}

// UpdateSessionActivity updates session activity
func (sm *SessionManager) UpdateSessionActivity(sessionID, action, resource string) {
	sm.zt.mu.Lock()
	defer sm.zt.mu.Unlock()

	session, exists := sm.zt.sessions[sessionID]
	if !exists {
		return
	}

	session.LastActivity = time.Now()
	session.ActivityLog = append(session.ActivityLog, Activity{
		Timestamp: time.Now(),
		Action:    action,
		Resource:  resource,
		Result:    "pending",
		IPAddress: session.Device.IPAddress,
	})
}

// TerminateSession terminates a session
func (sm *SessionManager) TerminateSession(sessionID string) {
	sm.zt.mu.Lock()
	defer sm.zt.mu.Unlock()

	if session, exists := sm.zt.sessions[sessionID]; exists {
		session.IsActive = false
	}
}

// TerminateUserSessions terminates all sessions for a user
func (sm *SessionManager) TerminateUserSessions(userID string) {
	sm.zt.mu.Lock()
	defer sm.zt.mu.Unlock()

	for _, session := range sm.zt.sessions {
		if session.UserID == userID {
			session.IsActive = false
		}
	}
}

// GetActiveSessions returns all active sessions
func (sm *SessionManager) GetActiveSessions() []*Session {
	sm.zt.mu.RLock()
	defer sm.zt.mu.RUnlock()

	var active []*Session
	for _, session := range sm.zt.sessions {
		if session.IsActive && time.Now().Before(session.ExpiresAt) {
			active = append(active, session)
		}
	}
	return active
}

// GetUserSessions returns all sessions for a user
func (sm *SessionManager) GetUserSessions(userID string) []*Session {
	sm.zt.mu.RLock()
	defer sm.zt.mu.RUnlock()

	var sessions []*Session
	for _, session := range sm.zt.sessions {
		if session.UserID == userID {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// CleanupExpiredSessions removes expired sessions
func (sm *SessionManager) CleanupExpiredSessions() int {
	sm.zt.mu.Lock()
	defer sm.zt.mu.Unlock()

	count := 0
	for id, session := range sm.zt.sessions {
		if time.Now().After(session.ExpiresAt) || !session.IsActive {
			delete(sm.zt.sessions, id)
			count++
		}
	}
	return count
}

// GetSessionStats returns session statistics
func (sm *SessionManager) GetSessionStats() map[string]interface{} {
	sm.zt.mu.RLock()
	defer sm.zt.mu.RUnlock()

	total := len(sm.zt.sessions)
	active := 0
	expired := 0

	for _, session := range sm.zt.sessions {
		if session.IsActive && time.Now().Before(session.ExpiresAt) {
			active++
		} else {
			expired++
		}
	}

	return map[string]interface{}{
		"total":   total,
		"active":  active,
		"expired": expired,
	}
}

// StartCleanupRoutine starts periodic session cleanup
func (sm *SessionManager) StartCleanupRoutine(interval time.Duration, stopCh <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				sm.CleanupExpiredSessions()
			}
		}
	}()
}
