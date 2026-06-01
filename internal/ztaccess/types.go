package ztaccess

import (
	"sync"
	"time"
)

// AccessLevel defines the level of access
type AccessLevel string

const (
	LevelAdmin  AccessLevel = "admin"
	LevelUser   AccessLevel = "user"
	LevelGuest  AccessLevel = "guest"
	LevelCustom AccessLevel = "custom"
)

// AuthMethod defines the authentication method
type AuthMethod string

const (
	AuthPassword  AuthMethod = "password"
	AuthToken     AuthMethod = "token"
	AuthBiometric AuthMethod = "biometric"
	AuthMFA       AuthMethod = "mfa"
)

// DeviceInfo represents device information
type DeviceInfo struct {
	DeviceID    string `json:"device_id"`
	DeviceName  string `json:"device_name"`
	DeviceType  string `json:"device_type"`
	OS          string `json:"os"`
	Browser     string `json:"browser"`
	Fingerprint string `json:"fingerprint"`
	IPAddress   string `json:"ip_address"`
	Location    string `json:"location"`
}

// UserIdentity represents a user identity
type UserIdentity struct {
	UserID       string      `json:"user_id"`
	Username     string      `json:"username"`
	Email        string      `json:"email"`
	AccessLevel  AccessLevel `json:"access_level"`
	Groups       []string    `json:"groups"`
	Permissions  []string    `json:"permissions"`
	MFAEnabled   bool        `json:"mfa_enabled"`
	LastLogin    time.Time   `json:"last_login"`
	LoginCount   int         `json:"login_count"`
}

// Session represents an active session
type Session struct {
	SessionID    string     `json:"session_id"`
	UserID       string     `json:"user_id"`
	Device       DeviceInfo `json:"device"`
	AccessLevel  AccessLevel `json:"access_level"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    time.Time  `json:"expires_at"`
	LastActivity time.Time  `json:"last_activity"`
	IsActive     bool       `json:"is_active"`
	ActivityLog  []Activity `json:"activity_log"`
}

// Activity represents a session activity
type Activity struct {
	Timestamp   time.Time `json:"timestamp"`
	Action      string    `json:"action"`
	Resource    string    `json:"resource"`
	Result      string    `json:"result"`
	IPAddress   string    `json:"ip_address"`
}

// AccessPolicy defines an access policy
type AccessPolicy struct {
	PolicyID    string      `json:"policy_id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Resources   []string    `json:"resources"`
	Actions     []string    `json:"actions"`
	Conditions  []Condition `json:"conditions"`
	Priority    int         `json:"priority"`
	Enabled     bool        `json:"enabled"`
}

// Condition represents a policy condition
type Condition struct {
	Type     string      `json:"type"` // "time", "ip", "device", "location"
	Operator string      `json:"operator"` // "in", "not_in", "equals", "contains"
	Value    interface{} `json:"value"`
}

// AnomalyDetection represents an anomaly detection result
type AnomalyDetection struct {
	UserID      string    `json:"user_id"`
	SessionID   string    `json:"session_id"`
	AnomalyType string    `json:"anomaly_type"`
	Severity    string    `json:"severity"` // "low", "medium", "high", "critical"
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	DeviceInfo  DeviceInfo `json:"device_info"`
}

// AuditEntry represents an audit log entry
type AuditEntry struct {
	EntryID     string    `json:"entry_id"`
	Timestamp   time.Time `json:"timestamp"`
	UserID      string    `json:"user_id"`
	Action      string    `json:"action"`
	Resource    string    `json:"resource"`
	Result      string    `json:"result"`
	IPAddress   string    `json:"ip_address"`
	UserAgent   string    `json:"user_agent"`
	Details     string    `json:"details"`
}

// ZTAccess manages the zero-trust access system
type ZTAccess struct {
	mu            sync.RWMutex
	users         map[string]*UserIdentity
	sessions      map[string]*Session
	policies      map[string]*AccessPolicy
	anomalies     []AnomalyDetection
	auditLog      []AuditEntry
	jwtSecret     []byte
	sessionTTL    time.Duration
	maxSessions   int
}

// NewZTAccess creates a new ZTAccess instance
func NewZTAccess() *ZTAccess {
	return &ZTAccess{
		users:       make(map[string]*UserIdentity),
		sessions:    make(map[string]*Session),
		policies:    make(map[string]*AccessPolicy),
		anomalies:   make([]AnomalyDetection, 0),
		auditLog:    make([]AuditEntry, 0),
		jwtSecret:   []byte("default-secret-change-in-production"),
		sessionTTL:  24 * time.Hour,
		maxSessions: 100,
	}
}

// removeOldestSession removes the oldest session for a user
func (zt *ZTAccess) removeOldestSession(userID string) {
	var oldest *Session
	for _, session := range zt.sessions {
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
