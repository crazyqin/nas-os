// Package fctarget implements Fibre Channel target management for NAS-OS.
package fctarget

import (
	"time"
)

// FCTarget represents a Fibre Channel target.
type FCTarget struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`                    // Friendly name
	Alias         string      `json:"alias,omitempty"`         // Target alias
	WWPN          string      `json:"wwpn"`                    // World Wide Port Name
	WWNN          string      `json:"wwnn"`                    // World Wide Node Name
	Mode          TargetMode  `json:"mode"`                    // Target mode
	LUNs          []*LUN      `json:"luns"`                    // Logical Unit Numbers
	Ports         []*Port     `json:"ports"`                   // FC ports
	Zones         []*Zone     `json:"zones"`                   // FC zones
	MaxSessions   int         `json:"maxSessions"`             // Maximum concurrent sessions
	Enabled       bool        `json:"enabled"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
}

// TargetMode defines the FC target operating mode.
type TargetMode string

const (
	// TargetModeInitiator represents initiator mode.
	TargetModeInitiator TargetMode = "initiator"
	// TargetModeTarget represents target mode.
	TargetModeTarget TargetMode = "target"
	// TargetModeDual represents dual (initiator + target) mode.
	TargetModeDual TargetMode = "dual"
)

// TargetInput for creating/updating FC targets.
type TargetInput struct {
	Name        string     `json:"name" binding:"required"`
	Alias       string     `json:"alias"`
	Mode        TargetMode `json:"mode"`
	MaxSessions int        `json:"maxSessions"`
}

// LUN represents a Fibre Channel Logical Unit Number.
type LUN struct {
	ID        string         `json:"id"`
	Number    int            `json:"number"`    // LUN number (0-255)
	Name      string         `json:"name"`      // Friendly name
	Type      LUNType        `json:"type"`      // file or block
	Path      string         `json:"path"`      // File path or block device path
	Size      int64          `json:"size"`      // Size in bytes
	BlockSize int            `json:"blockSize"` // Block size (512, 4096, etc.)
	ReadOnly  bool           `json:"readOnly"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

// LUNType defines the type of LUN backing.
type LUNType string

const (
	// LUNTypeFile represents file-backed LUN type.
	LUNTypeFile LUNType = "file"
	// LUNTypeBlock represents block device LUN type.
	LUNTypeBlock LUNType = "block"
)

// LUNInput for creating/updating LUNs.
type LUNInput struct {
	Name      string  `json:"name" binding:"required"`
	Type      LUNType `json:"type" binding:"required"`
	Path      string  `json:"path"`      // Auto-generated for file type
	Size      int64   `json:"size"`      // Required for file type
	BlockSize int     `json:"blockSize"` // Default 512
	ReadOnly  bool    `json:"readOnly"`
}

// LUNMapping represents a LUN mapping to an initiator.
type LUNMapping struct {
	LUNID        string `json:"lunId"`
	InitiatorWWPN string `json:"initiatorWwpn"`
	LUNNumber    int    `json:"lunNumber"`
}

// LUNMappingInput for creating LUN mappings.
type LUNMappingInput struct {
	InitiatorWWPN string `json:"initiatorWwpn" binding:"required"`
	LUNNumber     int    `json:"lunNumber"`
}

// Port represents a Fibre Channel port.
type Port struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`      // Port name (e.g., fc0, fc1)
	WWPN      string    `json:"wwpn"`      // World Wide Port Name
	WWNN      string    `json:"wwnn"`      // World Wide Node Name
	Speed     string    `json:"speed"`     // Port speed (8G, 16G, 32G, etc.)
	State     PortState `json:"state"`     // Port state
	Type      PortType  `json:"type"`      // Port type
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// PortState defines the FC port state.
type PortState string

const (
	// PortStateOnline represents port is online.
	PortStateOnline PortState = "online"
	// PortStateOffline represents port is offline.
	PortStateOffline PortState = "offline"
	// PortStateLinkDown represents port link is down.
	PortStateLinkDown PortState = "link_down"
	// PortStateError represents port is in error state.
	PortStateError PortState = "error"
)

// PortType defines the FC port type.
type PortType string

const (
	// PortTypeNPort represents N_Port (Node Port).
	PortTypeNPort PortType = "n_port"
	// PortTypeNLPort represents NL_Port (Node Loop Port).
	PortTypeNLPort PortType = "nl_port"
	// PortTypeFLPort represents FL_Port (Fabric Loop Port).
	PortTypeFLPort PortType = "fl_port"
	// PortTypeEPort represents E_Port (Expansion Port).
	PortTypeEPort PortType = "e_port"
	// PortTypeTarget represents Target port.
	PortTypeTarget PortType = "target"
	// PortTypeInitiator represents Initiator port.
	PortTypeInitiator PortType = "initiator"
)

// PortInput for creating/updating ports.
type PortInput struct {
	Name    string `json:"name" binding:"required"`
	Speed   string `json:"speed"`
	Enabled bool   `json:"enabled"`
}

// Session represents an active FC session.
type Session struct {
	ID           string    `json:"id"`
	TargetWWPN   string    `json:"targetWwpn"`
	InitiatorWWPN string   `json:"initiatorWwpn"`
	PortID       string    `json:"portId"`
	State        string    `json:"state"`
	ConnectedAt  time.Time `json:"connectedAt"`
}

// Zone represents an FC zone.
type Zone struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`      // Zone name
	Members   []ZoneMember `json:"members"`   // Zone members
	Enabled   bool        `json:"enabled"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

// ZoneMember represents a member of an FC zone.
type ZoneMember struct {
	WWPN string `json:"wwpn"` // World Wide Port Name
	Type string `json:"type"` // "initiator" or "target"
}

// ZoneInput for creating/updating zones.
type ZoneInput struct {
	Name    string       `json:"name" binding:"required"`
	Members []ZoneMember `json:"members" binding:"required"`
}

// TargetStatus represents target operational status.
type TargetStatus struct {
	ID            string     `json:"id"`
	WWPN          string     `json:"wwpn"`
	WWNN          string     `json:"wwnn"`
	Running       bool       `json:"running"`
	Sessions      []*Session `json:"sessions"`
	SessionCount  int        `json:"sessionCount"`
	MaxSessions   int        `json:"maxSessions"`
	LUNCount      int        `json:"lunCount"`
	PortCount     int        `json:"portCount"`
}

// PerformanceStats represents FC performance statistics.
type PerformanceStats struct {
	TargetID      string    `json:"targetId"`
	IOPS          IOPSStats `json:"iops"`
	Throughput    ThroughputStats `json:"throughput"`
	Latency       LatencyStats `json:"latency"`
	CollectedAt   time.Time `json:"collectedAt"`
}

// IOPSStats represents IOPS statistics.
type IOPSStats struct {
	Read  int64 `json:"read"`
	Write int64 `json:"write"`
	Total int64 `json:"total"`
}

// ThroughputStats represents throughput statistics.
type ThroughputStats struct {
	ReadMBps  float64 `json:"readMBps"`
	WriteMBps float64 `json:"writeMBps"`
	TotalMBps float64 `json:"totalMBps"`
}

// LatencyStats represents latency statistics.
type LatencyStats struct {
	AvgMs float64 `json:"avgMs"`
	MaxMs float64 `json:"maxMs"`
	MinMs float64 `json:"minMs"`
	P95Ms float64 `json:"p95Ms"`
	P99Ms float64 `json:"p99Ms"`
}

// Config for FC target service.
type Config struct {
	Enabled    bool   `json:"enabled"`
	ListenAddr string `json:"listenAddr"`
	ListenPort int    `json:"listenPort"`
	MaxLUNs    int    `json:"maxLuns"`
	MaxPorts   int    `json:"maxPorts"`
}

// DefaultConfig returns default FC target configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:    true,
		ListenAddr: "0.0.0.0",
		ListenPort: 3260,
		MaxLUNs:    256,
		MaxPorts:   16,
	}
}

// Errors.
var (
	ErrTargetNotFound     = &FCError{Code: 404, Message: "target not found"}
	ErrTargetExists       = &FCError{Code: 409, Message: "target already exists"}
	ErrLUNNotFound        = &FCError{Code: 404, Message: "LUN not found"}
	ErrLUNExists          = &FCError{Code: 409, Message: "LUN already exists"}
	ErrPortNotFound       = &FCError{Code: 404, Message: "port not found"}
	ErrPortExists         = &FCError{Code: 409, Message: "port already exists"}
	ErrZoneNotFound       = &FCError{Code: 404, Message: "zone not found"}
	ErrZoneExists         = &FCError{Code: 409, Message: "zone already exists"}
	ErrInvalidWWPN        = &FCError{Code: 400, Message: "invalid WWPN format"}
	ErrInvalidWWNN        = &FCError{Code: 400, Message: "invalid WWNN format"}
	ErrInvalidSize        = &FCError{Code: 400, Message: "invalid size"}
	ErrMaxLUNsReached     = &FCError{Code: 503, Message: "maximum LUNs reached"}
	ErrMaxPortsReached    = &FCError{Code: 503, Message: "maximum ports reached"}
	ErrMaxSessionsReached = &FCError{Code: 503, Message: "maximum sessions reached"}
	ErrInvalidMode        = &FCError{Code: 400, Message: "invalid target mode"}
)

// FCError represents an FC target error.
type FCError struct {
	Code    interface{}
	Message string
}

// Error returns the error message.
func (e *FCError) Error() string {
	return e.Message
}
