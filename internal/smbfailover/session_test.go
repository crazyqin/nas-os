package smbfailover

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSessionManager_CreateSession(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSessionConfig()
	manager := NewSessionManager(config, nil, logger)

	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	// Test create session
	session, err := manager.CreateSession("session-1", "192.168.1.100", "testuser")
	require.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, "session-1", session.SessionID)
	assert.Equal(t, "192.168.1.100", session.ClientIP)
	assert.Equal(t, "testuser", session.Username)
	assert.Equal(t, uint64(1), session.SequenceNum)

	// Test duplicate session
	_, err = manager.CreateSession("session-1", "192.168.1.100", "testuser")
	assert.Error(t, err)
}

func TestSessionManager_MaxSessions(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSessionConfig()
	config.MaxSessions = 2
	manager := NewSessionManager(config, nil, logger)

	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	// Create max sessions
	_, err = manager.CreateSession("session-1", "192.168.1.100", "user1")
	require.NoError(t, err)

	_, err = manager.CreateSession("session-2", "192.168.1.101", "user2")
	require.NoError(t, err)

	// Try to exceed limit
	_, err = manager.CreateSession("session-3", "192.168.1.102", "user3")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum sessions")
}

func TestSessionManager_GetSession(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSessionConfig()
	manager := NewSessionManager(config, nil, logger)

	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	// Create session
	_, err = manager.CreateSession("session-1", "192.168.1.100", "testuser")
	require.NoError(t, err)

	// Get session
	session, ok := manager.GetSession("session-1")
	assert.True(t, ok)
	assert.NotNil(t, session)
	assert.Equal(t, "session-1", session.SessionID)

	// Get non-existent session
	_, ok = manager.GetSession("non-existent")
	assert.False(t, ok)
}

func TestSessionManager_GetSessionsByClient(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSessionConfig()
	manager := NewSessionManager(config, nil, logger)

	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	// Create sessions for same client
	_, err = manager.CreateSession("session-1", "192.168.1.100", "user1")
	require.NoError(t, err)

	_, err = manager.CreateSession("session-2", "192.168.1.100", "user2")
	require.NoError(t, err)

	_, err = manager.CreateSession("session-3", "192.168.1.101", "user3")
	require.NoError(t, err)

	// Get sessions by client
	sessions := manager.GetSessionsByClient("192.168.1.100")
	assert.Len(t, sessions, 2)

	sessions = manager.GetSessionsByClient("192.168.1.101")
	assert.Len(t, sessions, 1)
}

func TestSessionManager_UpdateSessionActivity(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSessionConfig()
	manager := NewSessionManager(config, nil, logger)

	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	// Create session
	session, err := manager.CreateSession("session-1", "192.168.1.100", "testuser")
	require.NoError(t, err)

	initialSeq := session.SequenceNum
	initialActivity := session.LastActivity

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Update activity
	err = manager.UpdateSessionActivity("session-1")
	require.NoError(t, err)

	// Verify update
	session, _ = manager.GetSession("session-1")
	assert.True(t, session.SequenceNum > initialSeq)
	assert.True(t, session.LastActivity.After(initialActivity))

	// Update non-existent session
	err = manager.UpdateSessionActivity("non-existent")
	assert.Error(t, err)
}

func TestSessionManager_TreeConnections(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSessionConfig()
	manager := NewSessionManager(config, nil, logger)

	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	// Create session
	_, err = manager.CreateSession("session-1", "192.168.1.100", "testuser")
	require.NoError(t, err)

	// Add tree connection
	tc := TreeConnState{
		ID:          "tc-1",
		ShareName:   "documents",
		SharePath:   "/srv/documents",
		AccessMask:  0x1F01FF,
		ConnectedAt: time.Now(),
	}

	err = manager.AddTreeConnection("session-1", tc)
	require.NoError(t, err)

	// Verify
	session, _ := manager.GetSession("session-1")
	assert.Len(t, session.TreeConnections, 1)
	assert.Equal(t, "documents", session.TreeConnections[0].ShareName)
}

func TestSessionManager_FileHandles(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSessionConfig()
	config.MaxFilesPerSession = 2
	manager := NewSessionManager(config, nil, logger)

	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	// Create session
	_, err = manager.CreateSession("session-1", "192.168.1.100", "testuser")
	require.NoError(t, err)

	// Add file handles
	fh1 := FileHandleState{
		FileID:   "file-1",
		Handle:   1,
		FullPath: "/documents/test1.txt",
	}

	fh2 := FileHandleState{
		FileID:   "file-2",
		Handle:   2,
		FullPath: "/documents/test2.txt",
	}

	err = manager.AddFileHandle("session-1", fh1)
	require.NoError(t, err)

	err = manager.AddFileHandle("session-1", fh2)
	require.NoError(t, err)

	// Verify
	session, _ := manager.GetSession("session-1")
	assert.Len(t, session.OpenFiles, 2)

	// Try to exceed limit
	fh3 := FileHandleState{
		FileID:   "file-3",
		Handle:   3,
		FullPath: "/documents/test3.txt",
	}

	err = manager.AddFileHandle("session-1", fh3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum open files")

	// Remove file handle
	err = manager.RemoveFileHandle("session-1", "file-1")
	require.NoError(t, err)

	session, _ = manager.GetSession("session-1")
	assert.Len(t, session.OpenFiles, 1)

	// Remove non-existent
	err = manager.RemoveFileHandle("session-1", "non-existent")
	assert.Error(t, err)
}

func TestSessionManager_Locks(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSessionConfig()
	manager := NewSessionManager(config, nil, logger)

	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	// Create session
	_, err = manager.CreateSession("session-1", "192.168.1.100", "testuser")
	require.NoError(t, err)

	// Add lock
	lock := LockState{
		LockID: "lock-1",
		FileID: "file-1",
		Offset: 0,
		Length: 1024,
	}

	err = manager.AddLock("session-1", lock)
	require.NoError(t, err)

	// Verify
	session, _ := manager.GetSession("session-1")
	assert.Len(t, session.Locks, 1)

	// Remove lock
	err = manager.RemoveLock("session-1", "lock-1")
	require.NoError(t, err)

	session, _ = manager.GetSession("session-1")
	assert.Len(t, session.Locks, 0)

	// Remove non-existent
	err = manager.RemoveLock("session-1", "non-existent")
	assert.Error(t, err)
}

func TestSessionManager_CloseSession(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSessionConfig()
	manager := NewSessionManager(config, nil, logger)

	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	// Create session
	_, err = manager.CreateSession("session-1", "192.168.1.100", "testuser")
	require.NoError(t, err)

	// Close session
	err = manager.CloseSession("session-1")
	require.NoError(t, err)

	// Verify removed
	_, ok := manager.GetSession("session-1")
	assert.False(t, ok)

	// Close non-existent
	err = manager.CloseSession("session-1")
	assert.Error(t, err)
}

func TestSessionManager_SerializeDeserialize(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSessionConfig()
	manager := NewSessionManager(config, nil, logger)

	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	// Create session with data
	_, err = manager.CreateSession("session-1", "192.168.1.100", "testuser")
	require.NoError(t, err)

	session, _ := manager.GetSession("session-1")
	session.AuthContext = &AuthContext{
		AuthType:   "NTLM",
		UserSID:    "S-1-5-21-1234567890-1234567890-1234567890-1001",
		ExpiryTime: time.Now().Add(8 * time.Hour),
	}

	// Serialize
	data, err := SerializeSession(session)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Deserialize
	restored, err := DeserializeSession(data)
	require.NoError(t, err)
	assert.Equal(t, session.SessionID, restored.SessionID)
	assert.Equal(t, session.ClientIP, restored.ClientIP)
	assert.Equal(t, session.Username, restored.Username)
	assert.NotNil(t, restored.AuthContext)
}

func TestSessionManager_GetSerializedSessions(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSessionConfig()
	manager := NewSessionManager(config, nil, logger)

	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	// Create sessions
	_, err = manager.CreateSession("session-1", "192.168.1.100", "user1")
	require.NoError(t, err)

	_, err = manager.CreateSession("session-2", "192.168.1.101", "user2")
	require.NoError(t, err)

	// Get serialized
	serialized, err := manager.GetSerializedSessions()
	require.NoError(t, err)
	assert.Len(t, serialized, 2)
	assert.Contains(t, serialized, "session-1")
	assert.Contains(t, serialized, "session-2")
}

func TestSessionManager_RestoreSessions(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSessionConfig()
	manager := NewSessionManager(config, nil, logger)

	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	// Prepare sessions data
	sessions := map[string][]byte{
		"session-1": []byte(`{"session_id":"session-1","client_ip":"192.168.1.100","username":"user1"}`),
		"session-2": []byte(`{"session_id":"session-2","client_ip":"192.168.1.101","username":"user2"}`),
	}

	// Restore
	count, err := manager.RestoreSessions(sessions)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify
	session, ok := manager.GetSession("session-1")
	assert.True(t, ok)
	assert.Equal(t, "user1", session.Username)
}

func TestSessionManager_GetSessionStats(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSessionConfig()
	manager := NewSessionManager(config, nil, logger)

	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	// Create sessions
	_, err = manager.CreateSession("session-1", "192.168.1.100", "user1")
	require.NoError(t, err)

	// Add some resources
	manager.AddTreeConnection("session-1", TreeConnState{ID: "tc-1", ShareName: "share1"})
	manager.AddFileHandle("session-1", FileHandleState{FileID: "file-1"})
	manager.AddLock("session-1", LockState{LockID: "lock-1"})

	stats := manager.GetSessionStats()
	assert.Equal(t, 1, stats["total_sessions"])
	assert.Equal(t, 1, stats["total_open_files"])
	assert.Equal(t, 1, stats["total_locks"])
	assert.Equal(t, 1, stats["total_tree_conns"])
}

func TestSessionManager_EventListener(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSessionConfig()
	manager := NewSessionManager(config, nil, logger)

	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	var events []SessionEvent
	var mu sync.Mutex

	// Add listener
	manager.AddEventListener(func(event SessionEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})

	// Create and close session
	_, err = manager.CreateSession("session-1", "192.168.1.100", "testuser")
	require.NoError(t, err)

	err = manager.CloseSession("session-1")
	require.NoError(t, err)

	// Wait for events
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, events, 2)
	// Events may arrive in any order due to goroutine scheduling
	eventTypes := make([]string, len(events))
	for i, e := range events {
		eventTypes[i] = e.Type
	}
	assert.Contains(t, eventTypes, "created")
	assert.Contains(t, eventTypes, "closed")
}

func TestSessionManager_GetSessionCount(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSessionConfig()
	manager := NewSessionManager(config, nil, logger)

	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	assert.Equal(t, 0, manager.GetSessionCount())

	_, err = manager.CreateSession("session-1", "192.168.1.100", "user1")
	require.NoError(t, err)

	assert.Equal(t, 1, manager.GetSessionCount())

	_, err = manager.CreateSession("session-2", "192.168.1.101", "user2")
	require.NoError(t, err)

	assert.Equal(t, 2, manager.GetSessionCount())
}
