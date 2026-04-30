// Package nvmeof provides NVMe-oF access audit logging.
package nvmeof

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// AuditEvent represents an NVMe-oF audit event.
type AuditEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	EventType    string    `json:"event_type"`    // connect, disconnect, auth_fail, subsystem_create, etc.
	HostNQN      string    `json:"host_nqn"`
	SubsystemNQN string    `json:"subsystem_nqn"`
	SourceIP     string    `json:"source_ip"`
	PortID       string    `json:"port_id"`
	Success      bool      `json:"success"`
	Details      string    `json:"details,omitempty"`
}

// AuditLogger provides audit logging for NVMe-oF operations.
type AuditLogger struct {
	mu       sync.Mutex
	logger   *zap.Logger
	logFile  *os.File
	logPath  string
	encoder  *json.Encoder
	events   []AuditEvent
	maxEvents int
}

// NewAuditLogger creates a new NVMe-oF audit logger.
func NewAuditLogger(dataDir string, logger *zap.Logger) (*AuditLogger, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	logDir := filepath.Join(dataDir, "audit")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	logPath := filepath.Join(logDir, "nvmeof-audit.jsonl")
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log: %w", err)
	}

	al := &AuditLogger{
		logger:    logger,
		logFile:   logFile,
		logPath:   logPath,
		encoder:   json.NewEncoder(logFile),
		events:    make([]AuditEvent, 0, 1000),
		maxEvents: 10000,
	}

	return al, nil
}

// LogEvent logs an audit event.
func (al *AuditLogger) LogEvent(event AuditEvent) {
	al.mu.Lock()
	defer al.mu.Unlock()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Write to file
	if err := al.encoder.Encode(event); err != nil {
		al.logger.Error("Failed to write audit event", zap.Error(err))
	}

	// Keep in memory for recent queries
	al.events = append(al.events, event)
	if len(al.events) > al.maxEvents {
		al.events = al.events[len(al.events)-al.maxEvents:]
	}

	al.logger.Info("NVMe-oF audit event",
		zap.String("type", event.EventType),
		zap.String("host", event.HostNQN),
		zap.String("subsystem", event.SubsystemNQN),
		zap.String("source_ip", event.SourceIP),
		zap.Bool("success", event.Success))
}

// LogConnection logs a connection event.
func (al *AuditLogger) LogConnection(hostNQN, subsystemNQN, sourceIP, portID string, success bool, details string) {
	al.LogEvent(AuditEvent{
		EventType:    "connection",
		HostNQN:      hostNQN,
		SubsystemNQN: subsystemNQN,
		SourceIP:     sourceIP,
		PortID:       portID,
		Success:      success,
		Details:      details,
	})
}

// LogAuthFailure logs an authentication failure.
func (al *AuditLogger) LogAuthFailure(hostNQN, subsystemNQN, sourceIP, reason string) {
	al.LogEvent(AuditEvent{
		EventType:    "auth_failure",
		HostNQN:      hostNQN,
		SubsystemNQN: subsystemNQN,
		SourceIP:     sourceIP,
		Success:      false,
		Details:      reason,
	})
}

// LogSubsystemEvent logs a subsystem management event.
func (al *AuditLogger) LogSubsystemEvent(eventType, subsystemNQN, details string) {
	al.LogEvent(AuditEvent{
		EventType:    eventType,
		SubsystemNQN: subsystemNQN,
		Success:      true,
		Details:      details,
	})
}

// GetRecentEvents returns recent audit events.
func (al *AuditLogger) GetRecentEvents(limit int) []AuditEvent {
	al.mu.Lock()
	defer al.mu.Unlock()

	if limit <= 0 || limit > len(al.events) {
		limit = len(al.events)
	}

	start := len(al.events) - limit
	result := make([]AuditEvent, limit)
	copy(result, al.events[start:])
	return result
}

// Close closes the audit logger.
func (al *AuditLogger) Close() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.logFile != nil {
		return al.logFile.Close()
	}
	return nil
}
