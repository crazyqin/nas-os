package smbfailover

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestAuditLogger_New(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)
	assert.NotNil(t, auditLogger)

	// Cleanup
	auditLogger.Stop()
}

func TestAuditLogger_StartStop(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	// Start
	err = auditLogger.Start()
	require.NoError(t, err)

	// Try to start again
	err = auditLogger.Start()
	assert.Error(t, err)

	// Stop
	auditLogger.Stop()
}

func TestAuditLogger_LogEvent(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	err = auditLogger.Start()
	require.NoError(t, err)
	defer auditLogger.Stop()

	// Log an event
	event := AuditEvent{
		EventType: AuditEventFailoverStart,
		Source:    "node-1",
		Target:    "node-2",
		Message:   "failover started",
		Success:   true,
	}

	auditLogger.LogEvent(event)

	// Verify event was logged
	events := auditLogger.GetEvents(10)
	assert.Len(t, events, 1)
	assert.Equal(t, AuditEventFailoverStart, events[0].EventType)
	assert.Equal(t, "node-1", events[0].Source)
	assert.Equal(t, "node-2", events[0].Target)
}

func TestAuditLogger_LogFailoverStart(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	err = auditLogger.Start()
	require.NoError(t, err)
	defer auditLogger.Stop()

	event := &FailoverEvent{
		ID:       "failover-1",
		FromNode: "node-1",
		ToNode:   "node-2",
		Reason:   "test",
	}

	auditLogger.LogFailoverStart(event)

	events := auditLogger.GetEvents(10)
	assert.Len(t, events, 1)
	assert.Equal(t, AuditEventFailoverStart, events[0].EventType)
	assert.Contains(t, events[0].Message, "node-1")
	assert.Contains(t, events[0].Message, "node-2")
}

func TestAuditLogger_LogFailoverEnd(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	err = auditLogger.Start()
	require.NoError(t, err)
	defer auditLogger.Stop()

	// Success case
	event := &FailoverEvent{
		ID:        "failover-1",
		FromNode:  "node-1",
		ToNode:    "node-2",
		Reason:    "test",
		Success:   true,
		Sessions:  5,
		Duration:  100 * time.Millisecond,
	}

	auditLogger.LogFailoverEnd(event)

	events := auditLogger.GetEvents(10)
	assert.Len(t, events, 1)
	assert.Equal(t, AuditEventFailoverComplete, events[0].EventType)
	assert.True(t, events[0].Success)

	// Failure case
	event = &FailoverEvent{
		ID:       "failover-2",
		FromNode: "node-1",
		ToNode:   "node-2",
		Reason:   "test",
		Success:  false,
	}

	auditLogger.LogFailoverEnd(event)

	events = auditLogger.GetEvents(10)
	assert.Len(t, events, 2)
	assert.Equal(t, AuditEventFailoverFailed, events[1].EventType)
	assert.False(t, events[1].Success)
}

func TestAuditLogger_LogNodeFailure(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	err = auditLogger.Start()
	require.NoError(t, err)
	defer auditLogger.Stop()

	auditLogger.LogNodeFailure("node-1", "heartbeat timeout")

	events := auditLogger.GetEvents(10)
	assert.Len(t, events, 1)
	assert.Equal(t, AuditEventNodeFailure, events[0].EventType)
	assert.Equal(t, "node-1", events[0].NodeID)
	assert.Contains(t, events[0].Message, "heartbeat timeout")
}

func TestAuditLogger_LogNodeRecovery(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	err = auditLogger.Start()
	require.NoError(t, err)
	defer auditLogger.Stop()

	auditLogger.LogNodeRecovery("node-1")

	events := auditLogger.GetEvents(10)
	assert.Len(t, events, 1)
	assert.Equal(t, AuditEventNodeRecovery, events[0].EventType)
	assert.Equal(t, "node-1", events[0].NodeID)
	assert.True(t, events[0].Success)
}

func TestAuditLogger_LogSessionTransfer(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	err = auditLogger.Start()
	require.NoError(t, err)
	defer auditLogger.Stop()

	auditLogger.LogSessionTransfer("session-1", "node-1", "node-2")

	events := auditLogger.GetEvents(10)
	assert.Len(t, events, 1)
	assert.Equal(t, AuditEventSessionTransfer, events[0].EventType)
	assert.Equal(t, "session-1", events[0].SessionID)
	assert.Equal(t, "node-1", events[0].Source)
	assert.Equal(t, "node-2", events[0].Target)
}

func TestAuditLogger_LogVIPChange(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	err = auditLogger.Start()
	require.NoError(t, err)
	defer auditLogger.Stop()

	auditLogger.LogVIPChange("192.168.1.100", "node-1", "node-2")

	events := auditLogger.GetEvents(10)
	assert.Len(t, events, 1)
	assert.Equal(t, AuditEventVIPChange, events[0].EventType)
	assert.Equal(t, "node-1", events[0].Source)
	assert.Equal(t, "node-2", events[0].Target)
}

func TestAuditLogger_LogSyncComplete(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	err = auditLogger.Start()
	require.NoError(t, err)
	defer auditLogger.Stop()

	auditLogger.LogSyncComplete("node-1", "node-2", 10)

	events := auditLogger.GetEvents(10)
	assert.Len(t, events, 1)
	assert.Equal(t, AuditEventSyncComplete, events[0].EventType)
	assert.True(t, events[0].Success)
}

func TestAuditLogger_LogSyncFailed(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	err = auditLogger.Start()
	require.NoError(t, err)
	defer auditLogger.Stop()

	auditLogger.LogSyncFailed("node-1", "node-2", assert.AnError)

	events := auditLogger.GetEvents(10)
	assert.Len(t, events, 1)
	assert.Equal(t, AuditEventSyncFailed, events[0].EventType)
	assert.False(t, events[0].Success)
}

func TestAuditLogger_LogQuorumLost(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	err = auditLogger.Start()
	require.NoError(t, err)
	defer auditLogger.Stop()

	auditLogger.LogQuorumLost(1, 3)

	events := auditLogger.GetEvents(10)
	assert.Len(t, events, 1)
	assert.Equal(t, AuditEventQuorumLost, events[0].EventType)
	assert.False(t, events[0].Success)
}

func TestAuditLogger_LogQuorumRestored(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	err = auditLogger.Start()
	require.NoError(t, err)
	defer auditLogger.Stop()

	auditLogger.LogQuorumRestored(3, 3)

	events := auditLogger.GetEvents(10)
	assert.Len(t, events, 1)
	assert.Equal(t, AuditEventQuorumRestored, events[0].EventType)
	assert.True(t, events[0].Success)
}

func TestAuditLogger_GetEventsByType(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	err = auditLogger.Start()
	require.NoError(t, err)
	defer auditLogger.Stop()

	// Log different event types
	auditLogger.LogEvent(AuditEvent{EventType: AuditEventFailoverStart, Message: "start"})
	auditLogger.LogEvent(AuditEvent{EventType: AuditEventNodeFailure, Message: "failure"})
	auditLogger.LogEvent(AuditEvent{EventType: AuditEventFailoverComplete, Message: "complete"})
	auditLogger.LogEvent(AuditEvent{EventType: AuditEventNodeFailure, Message: "failure2"})

	// Get by type
	failoverEvents := auditLogger.GetEventsByType(AuditEventFailoverStart, 10)
	assert.Len(t, failoverEvents, 1)

	nodeFailures := auditLogger.GetEventsByType(AuditEventNodeFailure, 10)
	assert.Len(t, nodeFailures, 2)
}

func TestAuditLogger_GetEventsByTimeRange(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	err = auditLogger.Start()
	require.NoError(t, err)
	defer auditLogger.Stop()

	// Log events at different times
	now := time.Now()
	auditLogger.LogEvent(AuditEvent{
		EventType: AuditEventFailoverStart,
		Timestamp: now.Add(-1 * time.Hour),
		Message:   "old event",
	})
	auditLogger.LogEvent(AuditEvent{
		EventType: AuditEventFailoverComplete,
		Timestamp: now,
		Message:   "new event",
	})

	// Get by time range
	events := auditLogger.GetEventsByTimeRange(now.Add(-30*time.Minute), now.Add(30*time.Minute))
	assert.Len(t, events, 1)
	assert.Equal(t, "new event", events[0].Message)
}

func TestAuditLogger_SearchEvents(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	err = auditLogger.Start()
	require.NoError(t, err)
	defer auditLogger.Stop()

	// Log events
	auditLogger.LogEvent(AuditEvent{
		EventType: AuditEventFailoverStart,
		Source:    "node-1",
		Target:    "node-2",
		Message:   "failover from node-1 to node-2",
	})
	auditLogger.LogEvent(AuditEvent{
		EventType: AuditEventNodeFailure,
		NodeID:    "node-3",
		Message:   "node-3 failed",
	})

	// Search by source
	events := auditLogger.SearchEvents("node-1", 10)
	assert.Len(t, events, 1)
	assert.Equal(t, "node-1", events[0].Source)

	// Search by node ID
	events = auditLogger.SearchEvents("node-3", 10)
	assert.Len(t, events, 1)
	assert.Equal(t, "node-3", events[0].NodeID)
}

func TestAuditLogger_GetMetrics(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	err = auditLogger.Start()
	require.NoError(t, err)
	defer auditLogger.Stop()

	// Log some events
	auditLogger.LogEvent(AuditEvent{EventType: AuditEventFailoverStart})
	auditLogger.LogEvent(AuditEvent{EventType: AuditEventNodeFailure})
	auditLogger.LogEvent(AuditEvent{EventType: AuditEventFailoverStart})

	metrics := auditLogger.GetMetrics()
	assert.Equal(t, int64(3), metrics.TotalEvents)
	assert.Equal(t, int64(2), metrics.EventsByType[AuditEventFailoverStart])
	assert.Equal(t, int64(1), metrics.EventsByType[AuditEventNodeFailure])
}

func TestAuditLogger_ExportEvents(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	err = auditLogger.Start()
	require.NoError(t, err)
	defer auditLogger.Stop()

	// Log events
	now := time.Now()
	auditLogger.LogEvent(AuditEvent{
		EventType: AuditEventFailoverStart,
		Timestamp: now,
		Message:   "test",
	})

	// Export
	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)
	data, err := auditLogger.ExportEvents(start, end)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), "test")
}

func TestAuditLogger_Flush(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()
	config.FlushInterval = 50 * time.Millisecond

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	err = auditLogger.Start()
	require.NoError(t, err)
	defer auditLogger.Stop()

	// Log non-important event (won't be written immediately)
	auditLogger.LogEvent(AuditEvent{
		EventType: AuditEventHealthCheck,
		Message:   "health check",
	})

	// Wait for flush
	time.Sleep(100 * time.Millisecond)

	// Verify file was written
	logPath := filepath.Join(config.LogDir, config.LogFile)
	_, err = os.Stat(logPath)
	assert.NoError(t, err)
}

func TestAuditLogger_ImportantEvents(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	err = auditLogger.Start()
	require.NoError(t, err)
	defer auditLogger.Stop()

	// Log important events
	auditLogger.LogEvent(AuditEvent{
		EventType: AuditEventFailoverStart,
		Message:   "failover start",
	})
	auditLogger.LogEvent(AuditEvent{
		EventType: AuditEventNodeFailure,
		Message:   "node failure",
	})
	auditLogger.LogEvent(AuditEvent{
		EventType: AuditEventQuorumLost,
		Message:   "quorum lost",
	})

	// All should be written immediately
	logPath := filepath.Join(config.LogDir, config.LogFile)
	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "failover start")
	assert.Contains(t, string(data), "node failure")
	assert.Contains(t, string(data), "quorum lost")
}

func TestAuditConfig_Defaults(t *testing.T) {
	config := DefaultAuditConfig()

	assert.Equal(t, "/var/log/nas-os/smb-failover", config.LogDir)
	assert.Equal(t, "audit.log", config.LogFile)
	assert.Equal(t, 100, config.MaxSizeMB)
	assert.Equal(t, 10, config.MaxBackups)
	assert.Equal(t, 30, config.MaxAge)
	assert.True(t, config.Compress)
	assert.True(t, config.EnableJSON)
	assert.False(t, config.EnableConsole)
	assert.Equal(t, 90*24*time.Hour, config.RetentionPeriod)
	assert.Equal(t, 5*time.Second, config.FlushInterval)
	assert.Equal(t, 1000, config.BufferSize)
	assert.True(t, config.EnableMetrics)
}

func TestAuditEvent_Fields(t *testing.T) {
	event := AuditEvent{
		ID:        "audit-1",
		EventType: AuditEventFailoverStart,
		Timestamp: time.Now(),
		Source:    "node-1",
		Target:    "node-2",
		NodeID:    "node-1",
		SessionID: "session-1",
		Message:   "test event",
		Details: map[string]interface{}{
			"key": "value",
		},
		Success:  true,
		Error:    "",
		Duration: 100 * time.Millisecond,
		UserID:   "user-1",
		ClientIP: "192.168.1.100",
	}

	assert.Equal(t, "audit-1", event.ID)
	assert.Equal(t, AuditEventFailoverStart, event.EventType)
	assert.Equal(t, "node-1", event.Source)
	assert.Equal(t, "node-2", event.Target)
	assert.Equal(t, "node-1", event.NodeID)
	assert.Equal(t, "session-1", event.SessionID)
	assert.Equal(t, "test event", event.Message)
	assert.True(t, event.Success)
	assert.Equal(t, 100*time.Millisecond, event.Duration)
	assert.Equal(t, "user-1", event.UserID)
	assert.Equal(t, "192.168.1.100", event.ClientIP)
}

func TestAuditEventType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		event    AuditEventType
		expected string
	}{
		{"FailoverStart", AuditEventFailoverStart, "failover_start"},
		{"FailoverComplete", AuditEventFailoverComplete, "failover_complete"},
		{"FailoverFailed", AuditEventFailoverFailed, "failover_failed"},
		{"SessionTransfer", AuditEventSessionTransfer, "session_transfer"},
		{"NodeFailure", AuditEventNodeFailure, "node_failure"},
		{"NodeRecovery", AuditEventNodeRecovery, "node_recovery"},
		{"HealthCheck", AuditEventHealthCheck, "health_check"},
		{"VIPChange", AuditEventVIPChange, "vip_change"},
		{"SyncComplete", AuditEventSyncComplete, "sync_complete"},
		{"SyncFailed", AuditEventSyncFailed, "sync_failed"},
		{"ConfigChange", AuditEventConfigChange, "config_change"},
		{"ManualFailover", AuditEventManualFailover, "manual_failover"},
		{"QuorumLost", AuditEventQuorumLost, "quorum_lost"},
		{"QuorumRestored", AuditEventQuorumRestored, "quorum_restored"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.event))
		})
	}
}

func TestAuditMetrics_Fields(t *testing.T) {
	metrics := AuditMetrics{
		TotalEvents:   100,
		EventsByType:  map[AuditEventType]int64{AuditEventFailoverStart: 50},
		LastEventTime: time.Now(),
		FailedEvents:  5,
	}

	assert.Equal(t, int64(100), metrics.TotalEvents)
	assert.Equal(t, int64(50), metrics.EventsByType[AuditEventFailoverStart])
	assert.Equal(t, int64(5), metrics.FailedEvents)
}

func TestContainsString(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"Exact match", "hello", "hello", true},
		{"Contains", "hello world", "world", true},
		{"Empty substring", "hello", "", true},
		{"Not found", "hello", "xyz", false},
		{"Empty string", "", "hello", false},
		{"Both empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsString(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsQuery(t *testing.T) {
	event := AuditEvent{
		Message:  "failover started",
		Source:   "node-1",
		Target:   "node-2",
		NodeID:   "node-1",
		SessionID: "session-1",
		Details: map[string]interface{}{
			"key": "value",
		},
	}

	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{"Empty query", "", true},
		{"Match message", "started", true},
		{"Match source", "node-1", true},
		{"Match target", "node-2", true},
		{"Match node ID", "node-1", true},
		{"Match session ID", "session-1", true},
		{"No match", "xyz", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsQuery(event, tt.query)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsImportantEvent(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAuditConfig()
	config.LogDir = t.TempDir()

	auditLogger, err := NewAuditLogger(config, logger)
	require.NoError(t, err)

	tests := []struct {
		name     string
		event    AuditEventType
		expected bool
	}{
		{"FailoverStart", AuditEventFailoverStart, true},
		{"FailoverComplete", AuditEventFailoverComplete, true},
		{"FailoverFailed", AuditEventFailoverFailed, true},
		{"NodeFailure", AuditEventNodeFailure, true},
		{"QuorumLost", AuditEventQuorumLost, true},
		{"HealthCheck", AuditEventHealthCheck, false},
		{"VIPChange", AuditEventVIPChange, false},
		{"SyncComplete", AuditEventSyncComplete, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := auditLogger.isImportantEvent(tt.event)
			assert.Equal(t, tt.expected, result)
		})
	}
}
