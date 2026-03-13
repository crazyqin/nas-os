// Package iscsi implements iSCSI target management for NAS-OS
package iscsi

import (
	"time"
)

// Target represents an iSCSI target
type Target struct {
	ID               string            `json:"id"`
	IQN              string            `json:"iqn"`               // iSCSI Qualified Name
	Name             string            `json:"name"`              // Friendly name
	Alias            string            `json:"alias,omitempty"`   // Target alias
	LUNs             []*LUN            `json:"luns"`              // Logical Unit Numbers
	CHAP             *CHAPConfig       `json:"chap,omitempty"`    // CHAP authentication
	MaxSessions      int               `json:"maxSessions"`       // Maximum concurrent sessions
	CurrentSessions  int               `json:"currentSessions"`   // Current active sessions
	AllowedInitiators []string         `json:"allowedInitiators,omitempty"` // Allowed initiator IQNs
	Enabled          bool              `json:"enabled"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
}

// TargetInput for creating/updating targets
type TargetInput struct {
	Name             string   `json:"name" binding:"required"`
	Alias            string   `json:"alias"`
	IQN              string   `json:"iqn"` // Auto-generated if empty
	MaxSessions      int      `json:"maxSessions"`
	AllowedInitiators []string `json:"allowedInitiators"`
	CHAP             *CHAPInput `json:"chap"`
}

// LUN represents a Logical Unit Number
type LUN struct {
	ID          string       `json:"id"`
	Number      int          `json:"number"`           // LUN number (0-255)
	Name        string       `json:"name"`             // Friendly name
	Type        LUNType      `json:"type"`             // file or block
	Path        string       `json:"path"`             // File path or block device path
	Size        int64        `json:"size"`             // Size in bytes
	BlockSize   int          `json:"blockSize"`        // Block size (512, 4096, etc.)
	ReadOnly    bool         `json:"readOnly"`
	Snapshots   []*LUNSnapshot `json:"snapshots,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// LUNType defines the type of LUN backing
type LUNType string

const (
	LUNTypeFile  LUNType = "file"
	LUNTypeBlock LUNType = "block"
)

// LUNInput for creating/updating LUNs
type LUNInput struct {
	Name      string  `json:"name" binding:"required"`
	Type      LUNType `json:"type" binding:"required"`
	Path      string  `json:"path"`        // Auto-generated for file type
	Size      int64   `json:"size"`        // Required for file type
	BlockSize int     `json:"blockSize"`   // Default 512
	ReadOnly  bool    `json:"readOnly"`
}

// LUNExpandInput for expanding LUN
type LUNExpandInput struct {
	Size int64 `json:"size" binding:"required"` // New size (must be larger)
}

// LUNSnapshot represents a LUN snapshot
type LUNSnapshot struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	LUNNumber int       `json:"lunNumber"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}

// LUNSnapshotInput for creating snapshots
type LUNSnapshotInput struct {
	Name string `json:"name" binding:"required"`
}

// CHAPConfig holds CHAP authentication settings
type CHAPConfig struct {
	Enabled     bool   `json:"enabled"`
	Username    string `json:"username"`
	Secret      string `json:"secret,omitempty"`      // Hidden in responses
	Mutual      bool   `json:"mutual"`               // Mutual CHAP enabled
	MutualUser  string `json:"mutualUser,omitempty"`
	MutualSecret string `json:"mutualSecret,omitempty"` // Hidden in responses
}

// CHAPInput for configuring CHAP
type CHAPInput struct {
	Enabled      bool   `json:"enabled"`
	Username     string `json:"username" binding:"required_if=Enabled true"`
	Secret       string `json:"secret" binding:"required_if=Enabled true"`
	Mutual       bool   `json:"mutual"`
	MutualUser   string `json:"mutualUser"`
	MutualSecret string `json:"mutualSecret"`
}

// Session represents an active iSCSI session
type Session struct {
	ID           string    `json:"id"`
	TargetIQN    string    `json:"targetIqn"`
	InitiatorIQN string    `json:"initiatorIqn"`
	InitiatorIP  string    `json:"initiatorIp"`
	ConnectedAt  time.Time `json:"connectedAt"`
	State        string    `json:"state"`
}

// TargetStatus represents target operational status
type TargetStatus struct {
	IQN             string     `json:"iqn"`
	Running         bool       `json:"running"`
	Sessions        []*Session `json:"sessions"`
	SessionCount    int        `json:"sessionCount"`
	MaxSessions     int        `json:"maxSessions"`
	LUNCount        int        `json:"lunCount"`
}

// Config for iSCSI service
type Config struct {
	Enabled       bool   `json:"enabled"`
	PortalIP      string `json:"portalIp"`
	PortalPort    int    `json:"portalPort"`
	DiscoveryAuth bool   `json:"discoveryAuth"`
}

// Errors
var (
	ErrTargetNotFound     = &ISError{Code: 404, Message: "target not found"}
	ErrTargetExists       = &ISError{Code: 409, Message: "target already exists"}
	ErrLUNNotFound        = &ISError{Code: 404, Message: "LUN not found"}
	ErrLUNExists          = &ISError{Code: 409, Message: "LUN already exists"}
	ErrInvalidIQN         = &ISError{Code: 400, Message: "invalid IQN format"}
	ErrInvalidSize        = &ISError{Code: 400, Message: "invalid size"}
	ErrShrinkNotSupported = &ISError{Code: 400, Message: "shrinking LUN not supported"}
	ErrCHAPRequired       = &ISError{Code: 400, Message: "CHAP username/secret required"}
	ErrMaxSessionsReached = &ISError{Code: 503, Message: "maximum sessions reached"}
)

// ISError represents an iSCSI error
type ISError struct {
	Code    int
	Message string
}

func (e *ISError) Error() string {
	return e.Message
}