package smbfailover

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// AuditLogger logs failover events for auditing
type AuditLogger struct {
	mu          sync.RWMutex
	config      AuditConfig
	logger      *zap.Logger
	auditLogger *zap.Logger
	events      []AuditEvent
	fileWriter  *os.File
	jsonEncoder *json.Encoder
	running     bool
	stopCh      chan struct{}
}

// AuditConfig configures audit logging
type AuditConfig struct {
	LogDir          string        `json:"log_dir"`
	LogFile         string        `json:"log_file"`
	MaxSizeMB       int           `json:"max_size_mb"`
	MaxBackups      int           `json:"max_backups"`
	MaxAge          int           `json:"max_age_days"`
	Compress        bool          `json:"compress"`
	EnableJSON      bool          `json:"enable_json"`
	EnableConsole   bool          `json:"enable_console"`
	RetentionPeriod time.Duration `json:"retention_period"`
	FlushInterval   time.Duration `json:"flush_interval"`
	BufferSize      int           `json:"buffer_size"`
	EnableMetrics   bool          `json:"enable_metrics"`
}

// DefaultAuditConfig returns sensible defaults
func DefaultAuditConfig() AuditConfig {
	return AuditConfig{
		LogDir:          "/var/log/nas-os/smb-failover",
		LogFile:         "audit.log",
		MaxSizeMB:       100,
		MaxBackups:      10,
		MaxAge:          30,
		Compress:        true,
		EnableJSON:      true,
		EnableConsole:   false,
		RetentionPeriod: 90 * 24 * time.Hour, // 90 days
		FlushInterval:   5 * time.Second,
		BufferSize:      1000,
		EnableMetrics:   true,
	}
}

// AuditEventType represents the type of audit event
type AuditEventType string

const (
	AuditEventFailoverStart    AuditEventType = "failover_start"
	AuditEventFailoverComplete AuditEventType = "failover_complete"
	AuditEventFailoverFailed   AuditEventType = "failover_failed"
	AuditEventSessionTransfer  AuditEventType = "session_transfer"
	AuditEventNodeFailure      AuditEventType = "node_failure"
	AuditEventNodeRecovery     AuditEventType = "node_recovery"
	AuditEventHealthCheck      AuditEventType = "health_check"
	AuditEventVIPChange        AuditEventType = "vip_change"
	AuditEventSyncComplete     AuditEventType = "sync_complete"
	AuditEventSyncFailed       AuditEventType = "sync_failed"
	AuditEventConfigChange     AuditEventType = "config_change"
	AuditEventManualFailover   AuditEventType = "manual_failover"
	AuditEventQuorumLost       AuditEventType = "quorum_lost"
	AuditEventQuorumRestored   AuditEventType = "quorum_restored"
)

// AuditEvent represents an auditable event
type AuditEvent struct {
	ID        string                 `json:"id"`
	EventType AuditEventType         `json:"event_type"`
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source"`
	Target    string                 `json:"target,omitempty"`
	NodeID    string                 `json:"node_id,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
	Duration  time.Duration          `json:"duration,omitempty"`
	UserID    string                 `json:"user_id,omitempty"`
	ClientIP  string                 `json:"client_ip,omitempty"`
}

// AuditMetrics tracks audit logging metrics
type AuditMetrics struct {
	mu             sync.RWMutex
	TotalEvents    int64                    `json:"total_events"`
	EventsByType   map[AuditEventType]int64 `json:"events_by_type"`
	LastEventTime  time.Time                `json:"last_event_time"`
	FailedEvents   int64                    `json:"failed_events"`
	BufferedEvents int                      `json:"buffered_events"`
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(config AuditConfig, logger *zap.Logger) (*AuditLogger, error) {
	al := &AuditLogger{
		config: config,
		logger: logger,
		events: make([]AuditEvent, 0, config.BufferSize),
		stopCh: make(chan struct{}),
	}

	// Create log directory
	if err := os.MkdirAll(config.LogDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	// Open log file
	logPath := filepath.Join(config.LogDir, config.LogFile)
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}
	al.fileWriter = file
	al.jsonEncoder = json.NewEncoder(file)

	// Configure audit logger
	auditCore := al.buildAuditCore()
	al.auditLogger = zap.New(auditCore)

	return al, nil
}

// buildAuditCore builds the audit logger core
func (al *AuditLogger) buildAuditCore() zapcore.Core {
	cores := []zapcore.Core{}

	// JSON file encoder
	if al.config.EnableJSON {
		jsonEncoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.MillisDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		})
		fileWriter := zapcore.AddSync(al.fileWriter)
		cores = append(cores, zapcore.NewCore(jsonEncoder, fileWriter, zapcore.InfoLevel))
	}

	// Console encoder
	if al.config.EnableConsole {
		consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
		consoleWriter := zapcore.AddSync(os.Stdout)
		cores = append(cores, zapcore.NewCore(consoleEncoder, consoleWriter, zapcore.InfoLevel))
	}

	return zapcore.NewTee(cores...)
}

// Start starts the audit logger
func (al *AuditLogger) Start() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.running {
		return fmt.Errorf("audit logger already running")
	}

	al.running = true
	go al.flushLoop()
	go al.cleanupLoop()

	al.logger.Info("audit logger started",
		zap.String("log_dir", al.config.LogDir),
		zap.Bool("json_enabled", al.config.EnableJSON),
		zap.Bool("console_enabled", al.config.EnableConsole))

	return nil
}

// Stop stops the audit logger
func (al *AuditLogger) Stop() {
	al.mu.Lock()
	defer al.mu.Unlock()

	if !al.running {
		return
	}

	close(al.stopCh)
	al.flush()
	al.fileWriter.Close()
	al.running = false

	al.logger.Info("audit logger stopped")
}

// LogEvent logs an audit event
func (al *AuditLogger) LogEvent(event AuditEvent) {
	al.mu.Lock()

	// Generate ID if empty
	if event.ID == "" {
		event.ID = fmt.Sprintf("audit-%d", time.Now().UnixNano())
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Add to buffer
	al.events = append(al.events, event)

	// Update metrics
	al.updateMetrics(event)

	// Write immediately for important events
	if al.isImportantEvent(event.EventType) {
		al.writeEvent(event)
	}

	al.mu.Unlock()

	// Log to main logger
	al.logger.Debug("audit event recorded",
		zap.String("event_type", string(event.EventType)),
		zap.String("id", event.ID))
}

// isImportantEvent checks if an event should be logged immediately
func (al *AuditLogger) isImportantEvent(eventType AuditEventType) bool {
	switch eventType {
	case AuditEventFailoverStart,
		AuditEventFailoverComplete,
		AuditEventFailoverFailed,
		AuditEventNodeFailure,
		AuditEventQuorumLost:
		return true
	default:
		return false
	}
}

// writeEvent writes an event to the log
func (al *AuditLogger) writeEvent(event AuditEvent) {
	fields := []zap.Field{
		zap.String("event_id", event.ID),
		zap.String("event_type", string(event.EventType)),
		zap.Time("timestamp", event.Timestamp),
		zap.String("source", event.Source),
		zap.Bool("success", event.Success),
	}

	if event.Target != "" {
		fields = append(fields, zap.String("target", event.Target))
	}
	if event.NodeID != "" {
		fields = append(fields, zap.String("node_id", event.NodeID))
	}
	if event.SessionID != "" {
		fields = append(fields, zap.String("session_id", event.SessionID))
	}
	if event.Error != "" {
		fields = append(fields, zap.String("error", event.Error))
	}
	if event.Duration > 0 {
		fields = append(fields, zap.Duration("duration", event.Duration))
	}
	if event.UserID != "" {
		fields = append(fields, zap.String("user_id", event.UserID))
	}
	if event.ClientIP != "" {
		fields = append(fields, zap.String("client_ip", event.ClientIP))
	}

	al.auditLogger.Info(event.Message, fields...)
}

// updateMetrics updates audit metrics
func (al *AuditLogger) updateMetrics(event AuditEvent) {
	// This would update metrics if metrics are enabled
}

// flushLoop periodically flushes buffered events
func (al *AuditLogger) flushLoop() {
	ticker := time.NewTicker(al.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-al.stopCh:
			return
		case <-ticker.C:
			al.flush()
		}
	}
}

// flush writes all buffered events
func (al *AuditLogger) flush() {
	al.mu.Lock()
	events := al.events
	al.events = make([]AuditEvent, 0, al.config.BufferSize)
	al.mu.Unlock()

	for _, event := range events {
		if !al.isImportantEvent(event.EventType) {
			al.writeEvent(event)
		}
	}
}

// cleanupLoop periodically cleans up old log files
func (al *AuditLogger) cleanupLoop() {
	ticker := time.NewTicker(24 * time.Hour) // Run daily
	defer ticker.Stop()

	for {
		select {
		case <-al.stopCh:
			return
		case <-ticker.C:
			al.cleanupOldLogs()
		}
	}
}

// cleanupOldLogs removes old log files
func (al *AuditLogger) cleanupOldLogs() {
	entries, err := os.ReadDir(al.config.LogDir)
	if err != nil {
		al.logger.Error("failed to read audit log directory", zap.Error(err))
		return
	}

	cutoff := time.Now().Add(-al.config.RetentionPeriod)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(al.config.LogDir, entry.Name())
			if err := os.Remove(path); err != nil {
				al.logger.Error("failed to remove old audit log",
					zap.String("file", path),
					zap.Error(err))
			} else {
				al.logger.Info("removed old audit log", zap.String("file", path))
			}
		}
	}
}

// LogFailoverStart logs failover start event
func (al *AuditLogger) LogFailoverStart(event *FailoverEvent) {
	al.LogEvent(AuditEvent{
		EventType: AuditEventFailoverStart,
		Source:    event.FromNode,
		Target:    event.ToNode,
		Message:   fmt.Sprintf("Failover started from %s to %s", event.FromNode, event.ToNode),
		Details: map[string]interface{}{
			"event_id": event.ID,
			"reason":   event.Reason,
		},
	})
}

// LogFailoverEnd logs failover end event
func (al *AuditLogger) LogFailoverEnd(event *FailoverEvent) {
	eventType := AuditEventFailoverComplete
	if !event.Success {
		eventType = AuditEventFailoverFailed
	}

	al.LogEvent(AuditEvent{
		EventType: eventType,
		Source:    event.FromNode,
		Target:    event.ToNode,
		Success:   event.Success,
		Duration:  event.Duration,
		Message: fmt.Sprintf("Failover %s: %s -> %s (%d sessions, %v)",
			eventType, event.FromNode, event.ToNode, event.Sessions, event.Duration),
		Details: map[string]interface{}{
			"event_id":             event.ID,
			"sessions_transferred": event.Sessions,
			"reason":               event.Reason,
		},
	})
}

// LogNodeFailure logs node failure event
func (al *AuditLogger) LogNodeFailure(nodeID, reason string) {
	al.LogEvent(AuditEvent{
		EventType: AuditEventNodeFailure,
		NodeID:    nodeID,
		Message:   fmt.Sprintf("Node %s failed: %s", nodeID, reason),
		Details: map[string]interface{}{
			"reason": reason,
		},
	})
}

// LogNodeRecovery logs node recovery event
func (al *AuditLogger) LogNodeRecovery(nodeID string) {
	al.LogEvent(AuditEvent{
		EventType: AuditEventNodeRecovery,
		NodeID:    nodeID,
		Success:   true,
		Message:   fmt.Sprintf("Node %s recovered", nodeID),
	})
}

// LogSessionTransfer logs session transfer event
func (al *AuditLogger) LogSessionTransfer(sessionID, fromNode, toNode string) {
	al.LogEvent(AuditEvent{
		EventType: AuditEventSessionTransfer,
		SessionID: sessionID,
		Source:    fromNode,
		Target:    toNode,
		Success:   true,
		Message:   fmt.Sprintf("Session %s transferred from %s to %s", sessionID, fromNode, toNode),
	})
}

// LogVIPChange logs VIP change event
func (al *AuditLogger) LogVIPChange(vip, fromNode, toNode string) {
	al.LogEvent(AuditEvent{
		EventType: AuditEventVIPChange,
		Source:    fromNode,
		Target:    toNode,
		Success:   true,
		Message:   fmt.Sprintf("VIP %s moved from %s to %s", vip, fromNode, toNode),
		Details: map[string]interface{}{
			"vip": vip,
		},
	})
}

// LogSyncComplete logs sync completion event
func (al *AuditLogger) LogSyncComplete(sourceNode, targetNode string, sessions int) {
	al.LogEvent(AuditEvent{
		EventType: AuditEventSyncComplete,
		Source:    sourceNode,
		Target:    targetNode,
		Success:   true,
		Message:   fmt.Sprintf("State synced from %s to %s (%d sessions)", sourceNode, targetNode, sessions),
		Details: map[string]interface{}{
			"sessions_synced": sessions,
		},
	})
}

// LogSyncFailed logs sync failure event
func (al *AuditLogger) LogSyncFailed(sourceNode, targetNode string, err error) {
	al.LogEvent(AuditEvent{
		EventType: AuditEventSyncFailed,
		Source:    sourceNode,
		Target:    targetNode,
		Success:   false,
		Error:     err.Error(),
		Message:   fmt.Sprintf("Sync failed from %s to %s: %v", sourceNode, targetNode, err),
	})
}

// LogQuorumLost logs quorum lost event
func (al *AuditLogger) LogQuorumLost(activeNodes, totalNodes int) {
	al.LogEvent(AuditEvent{
		EventType: AuditEventQuorumLost,
		Success:   false,
		Message:   fmt.Sprintf("Quorum lost: %d/%d nodes active", activeNodes, totalNodes),
		Details: map[string]interface{}{
			"active_nodes": activeNodes,
			"total_nodes":  totalNodes,
		},
	})
}

// LogQuorumRestored logs quorum restored event
func (al *AuditLogger) LogQuorumRestored(activeNodes, totalNodes int) {
	al.LogEvent(AuditEvent{
		EventType: AuditEventQuorumRestored,
		Success:   true,
		Message:   fmt.Sprintf("Quorum restored: %d/%d nodes active", activeNodes, totalNodes),
		Details: map[string]interface{}{
			"active_nodes": activeNodes,
			"total_nodes":  totalNodes,
		},
	})
}

// GetEvents returns recent audit events
func (al *AuditLogger) GetEvents(limit int) []AuditEvent {
	al.mu.RLock()
	defer al.mu.RUnlock()

	if limit <= 0 || limit > len(al.events) {
		limit = len(al.events)
	}

	start := len(al.events) - limit
	events := make([]AuditEvent, limit)
	copy(events, al.events[start:])
	return events
}

// GetEventsByType returns events filtered by type
func (al *AuditLogger) GetEventsByType(eventType AuditEventType, limit int) []AuditEvent {
	al.mu.RLock()
	defer al.mu.RUnlock()

	filtered := make([]AuditEvent, 0)
	for i := len(al.events) - 1; i >= 0 && len(filtered) < limit; i-- {
		if al.events[i].EventType == eventType {
			filtered = append(filtered, al.events[i])
		}
	}

	// Reverse to chronological order
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	return filtered
}

// GetEventsByTimeRange returns events in a time range
func (al *AuditLogger) GetEventsByTimeRange(start, end time.Time) []AuditEvent {
	al.mu.RLock()
	defer al.mu.RUnlock()

	filtered := make([]AuditEvent, 0)
	for _, event := range al.events {
		if event.Timestamp.After(start) && event.Timestamp.Before(end) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

// GetMetrics returns audit logging metrics
func (al *AuditLogger) GetMetrics() *AuditMetrics {
	al.mu.RLock()
	defer al.mu.RUnlock()

	metrics := &AuditMetrics{
		TotalEvents:    int64(len(al.events)),
		EventsByType:   make(map[AuditEventType]int64),
		BufferedEvents: len(al.events),
	}

	for _, event := range al.events {
		metrics.EventsByType[event.EventType]++
		if metrics.LastEventTime.Before(event.Timestamp) {
			metrics.LastEventTime = event.Timestamp
		}
	}

	return metrics
}

// SearchEvents searches events by query
func (al *AuditLogger) SearchEvents(query string, limit int) []AuditEvent {
	al.mu.RLock()
	defer al.mu.RUnlock()

	results := make([]AuditEvent, 0)
	for i := len(al.events) - 1; i >= 0 && len(results) < limit; i-- {
		event := al.events[i]
		if containsQuery(event, query) {
			results = append(results, event)
		}
	}

	// Reverse to chronological order
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}

	return results
}

// containsQuery checks if an event matches a search query
func containsQuery(event AuditEvent, query string) bool {
	if query == "" {
		return true
	}

	// Search in message
	if containsString(event.Message, query) {
		return true
	}

	// Search in source
	if containsString(event.Source, query) {
		return true
	}

	// Search in target
	if containsString(event.Target, query) {
		return true
	}

	// Search in node ID
	if containsString(event.NodeID, query) {
		return true
	}

	// Search in session ID
	if containsString(event.SessionID, query) {
		return true
	}

	// Search in details
	for _, v := range event.Details {
		if s, ok := v.(string); ok && containsString(s, query) {
			return true
		}
	}

	return false
}

// containsString checks if a string contains a substring (case-insensitive)
func containsString(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) == 0 {
		return false
	}
	// Simple substring search
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ExportEvents exports events to JSON
func (al *AuditLogger) ExportEvents(start, end time.Time) ([]byte, error) {
	events := al.GetEventsByTimeRange(start, end)
	return json.MarshalIndent(events, "", "  ")
}
