package smbfailover

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// SessionState represents the state of an SMB session for failover
type SessionState struct {
	mu              sync.RWMutex
	SessionID       string            `json:"session_id"`
	ClientIP        string            `json:"client_ip"`
	Username        string            `json:"username"`
	AuthContext     *AuthContext      `json:"auth_context"`
	TreeConnections []TreeConnState   `json:"tree_connections"`
	OpenFiles       []FileHandleState `json:"open_files"`
	Locks           []LockState       `json:"locks"`
	NotifyWatches   []NotifyWatch     `json:"notify_watches"`
	CreatedAt       time.Time         `json:"created_at"`
	LastActivity    time.Time         `json:"last_activity"`
	SequenceNum     uint64            `json:"sequence_num"`
	ClientCaps      uint32            `json:"client_caps"`
	MaxReadSize     uint32            `json:"max_read_size"`
	MaxWriteSize    uint32            `json:"max_write_size"`
	Encrypted       bool              `json:"encrypted"`
	SigningRequired bool              `json:"signing_required"`
	SigningKey      []byte            `json:"signing_key,omitempty"`
	EncryptionKey   []byte            `json:"encryption_key,omitempty"`
}

// AuthContext contains authentication state
type AuthContext struct {
	AuthType     string    `json:"auth_type"` // NTLM, Kerberos, etc.
	NTLMSession  []byte    `json:"ntlm_session,omitempty"`
	KerberosTGT  []byte    `json:"kerberos_tgt,omitempty"`
	TokenGroups  []uint32  `json:"token_groups"`
	UserSID      string    `json:"user_sid"`
	GroupSIDs    []string  `json:"group_sids"`
	Privileges   uint32    `json:"privileges"`
	ExpiryTime   time.Time `json:"expiry_time"`
}

// TreeConnState represents the state of a tree connection
type TreeConnState struct {
	ID          string   `json:"id"`
	ShareName   string   `json:"share_name"`
	SharePath   string   `json:"share_path"`
	AccessMask  uint32   `json:"access_mask"`
	ConnectedAt time.Time `json:"connected_at"`
	IsDfs       bool     `json:"is_dfs"`
	IsDFSN      bool     `json:"is_dfsn"`
	IsPinned    bool     `json:"is_pinned"`
}

// FileHandleState represents an open file handle
type FileHandleState struct {
	FileID       string    `json:"file_id"`
	Handle       uint64    `json:"handle"`
	TreeConnID   string    `json:"tree_conn_id"`
	RelativePath string    `json:"relative_path"`
	FullPath     string    `json:"full_path"`
	AccessMask   uint32    `json:"access_mask"`
	ShareMode    uint32    `json:"share_mode"`
	CreateOptions uint32   `json:"create_options"`
	IsDirectory  bool      `json:"is_directory"`
	StreamName   string    `json:"stream_name,omitempty"`
	OpenedAt     time.Time `json:"opened_at"`
	LastOp       time.Time `json:"last_op"`
}

// LockState represents a file lock
type LockState struct {
	LockID     string `json:"lock_id"`
	FileID     string `json:"file_id"`
	Offset     int64  `json:"offset"`
	Length     int64  `json:"length"`
	LockType   uint32 `json:"lock_type"` // SMB2_LOCKFLAG_SHARED_LOCK, etc.
	Owner      []byte `json:"owner"`
	GrantedAt  time.Time `json:"granted_at"`
}

// NotifyWatch represents a change notification watch
type NotifyWatch struct {
	WatchID    string `json:"watch_id"`
	TreeConnID string `json:"tree_conn_id"`
	Path       string `json:"path"`
	Filter     uint32 `json:"filter"`
	Recursive  bool   `json:"recursive"`
	Subdir     string `json:"subdir,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// SessionManager manages session state for stateful failover
type SessionManager struct {
	mu             sync.RWMutex
	sessions       map[string]*SessionState
	logger         *zap.Logger
	config         SessionConfig
	stateStore     StateStore
	eventListeners []SessionEventListener
	running        bool
	stopCh         chan struct{}
}

// SessionConfig configures session management behavior
type SessionConfig struct {
	MaxSessions          int           `json:"max_sessions"`
	SessionTimeout       time.Duration `json:"session_timeout"`
	IdleTimeout          time.Duration `json:"idle_timeout"`
	CleanupInterval      time.Duration `json:"cleanup_interval"`
	MaxFilesPerSession   int           `json:"max_files_per_session"`
	MaxLocksPerSession   int           `json:"max_locks_per_session"`
	PersistState         bool          `json:"persist_state"`
	CompressState        bool          `json:"compress_state"`
	ValidateOnRestore    bool          `json:"validate_on_restore"`
}

// DefaultSessionConfig returns sensible defaults
func DefaultSessionConfig() SessionConfig {
	return SessionConfig{
		MaxSessions:        10000,
		SessionTimeout:     8 * time.Hour,
		IdleTimeout:        30 * time.Minute,
		CleanupInterval:    5 * time.Minute,
		MaxFilesPerSession: 1000,
		MaxLocksPerSession: 5000,
		PersistState:       true,
		CompressState:      true,
		ValidateOnRestore:  true,
	}
}

// StateStore is the interface for persistent state storage
type StateStore interface {
	SaveSession(session *SessionState) error
	LoadSession(sessionID string) (*SessionState, error)
	DeleteSession(sessionID string) error
	ListSessions() ([]string, error)
	SaveBulk(sessions []*SessionState) error
	Close() error
}

// SessionEvent represents a session lifecycle event
type SessionEvent struct {
	Type      string         `json:"type"`
	SessionID string         `json:"session_id"`
	Timestamp time.Time      `json:"timestamp"`
	State     *SessionState  `json:"state,omitempty"`
	Error     error          `json:"error,omitempty"`
}

// SessionEventListener is called on session events
type SessionEventListener func(event SessionEvent)

// NewSessionManager creates a new session manager
func NewSessionManager(config SessionConfig, stateStore StateStore, logger *zap.Logger) *SessionManager {
	return &SessionManager{
		sessions:   make(map[string]*SessionState),
		logger:     logger,
		config:     config,
		stateStore: stateStore,
		stopCh:     make(chan struct{}),
	}
}

// Start starts the session manager
func (sm *SessionManager) Start() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.running {
		return fmt.Errorf("session manager already running")
	}

	// Load persisted sessions
	if sm.stateStore != nil && sm.config.PersistState {
		if err := sm.loadPersistedSessions(); err != nil {
			sm.logger.Error("failed to load persisted sessions", zap.Error(err))
		}
	}

	go sm.cleanupLoop()
	sm.running = true

	sm.logger.Info("session manager started",
		zap.Int("loaded_sessions", len(sm.sessions)),
		zap.Int("max_sessions", sm.config.MaxSessions))

	return nil
}

// Stop stops the session manager
func (sm *SessionManager) Stop() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.running {
		return
	}

	close(sm.stopCh)
	sm.running = false
	sm.logger.Info("session manager stopped")
}

// CreateSession creates a new session state
func (sm *SessionManager) CreateSession(sessionID, clientIP, username string) (*SessionState, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(sm.sessions) >= sm.config.MaxSessions {
		return nil, fmt.Errorf("maximum sessions (%d) reached", sm.config.MaxSessions)
	}

	if _, exists := sm.sessions[sessionID]; exists {
		return nil, fmt.Errorf("session %s already exists", sessionID)
	}

	now := time.Now()
	session := &SessionState{
		SessionID:       sessionID,
		ClientIP:        clientIP,
		Username:        username,
		TreeConnections: make([]TreeConnState, 0),
		OpenFiles:       make([]FileHandleState, 0),
		Locks:           make([]LockState, 0),
		NotifyWatches:   make([]NotifyWatch, 0),
		CreatedAt:       now,
		LastActivity:    now,
		SequenceNum:     1,
	}

	sm.sessions[sessionID] = session

	sm.emitEvent(SessionEvent{
		Type:      "created",
		SessionID: sessionID,
		Timestamp: now,
		State:     session,
	})

	sm.logger.Info("session created",
		zap.String("session_id", sessionID),
		zap.String("client_ip", clientIP),
		zap.String("username", username))

	return session, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*SessionState, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	session, ok := sm.sessions[sessionID]
	return session, ok
}

// GetSessionsByClient returns all sessions for a client IP
func (sm *SessionManager) GetSessionsByClient(clientIP string) []*SessionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var result []*SessionState
	for _, session := range sm.sessions {
		if session.ClientIP == clientIP {
			result = append(result, session)
		}
	}
	return result
}

// UpdateSessionActivity updates the last activity time
func (sm *SessionManager) UpdateSessionActivity(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	session.mu.Lock()
	session.LastActivity = time.Now()
	session.SequenceNum++
	session.mu.Unlock()

	return nil
}

// AddTreeConnection adds a tree connection to a session
func (sm *SessionManager) AddTreeConnection(sessionID string, tc TreeConnState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	session.TreeConnections = append(session.TreeConnections, tc)
	session.LastActivity = time.Now()

	sm.logger.Debug("tree connection added",
		zap.String("session_id", sessionID),
		zap.String("share", tc.ShareName))

	return nil
}

// AddFileHandle adds an open file handle to a session
func (sm *SessionManager) AddFileHandle(sessionID string, fh FileHandleState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if len(session.OpenFiles) >= sm.config.MaxFilesPerSession {
		return fmt.Errorf("maximum open files (%d) reached for session", sm.config.MaxFilesPerSession)
	}

	session.OpenFiles = append(session.OpenFiles, fh)
	session.LastActivity = time.Now()

	sm.logger.Debug("file handle added",
		zap.String("session_id", sessionID),
		zap.String("file_id", fh.FileID),
		zap.String("path", fh.FullPath))

	return nil
}

// RemoveFileHandle removes an open file handle
func (sm *SessionManager) RemoveFileHandle(sessionID, fileID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	for i, fh := range session.OpenFiles {
		if fh.FileID == fileID {
			session.OpenFiles = append(session.OpenFiles[:i], session.OpenFiles[i+1:]...)
			session.LastActivity = time.Now()
			sm.logger.Debug("file handle removed",
				zap.String("session_id", sessionID),
				zap.String("file_id", fileID))
			return nil
		}
	}

	return fmt.Errorf("file handle %s not found in session %s", fileID, sessionID)
}

// AddLock adds a lock to a session
func (sm *SessionManager) AddLock(sessionID string, lock LockState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if len(session.Locks) >= sm.config.MaxLocksPerSession {
		return fmt.Errorf("maximum locks (%d) reached for session", sm.config.MaxLocksPerSession)
	}

	session.Locks = append(session.Locks, lock)
	session.LastActivity = time.Now()

	sm.logger.Debug("lock added",
		zap.String("session_id", sessionID),
		zap.String("lock_id", lock.LockID))

	return nil
}

// RemoveLock removes a lock
func (sm *SessionManager) RemoveLock(sessionID, lockID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	for i, lock := range session.Locks {
		if lock.LockID == lockID {
			session.Locks = append(session.Locks[:i], session.Locks[i+1:]...)
			session.LastActivity = time.Now()
			sm.logger.Debug("lock removed",
				zap.String("session_id", sessionID),
				zap.String("lock_id", lockID))
			return nil
		}
	}

	return fmt.Errorf("lock %s not found in session %s", lockID, sessionID)
}

// CloseSession closes a session and releases all resources
func (sm *SessionManager) CloseSession(sessionID string) error {
	sm.mu.Lock()
	session, ok := sm.sessions[sessionID]
	if !ok {
		sm.mu.Unlock()
		return fmt.Errorf("session %s not found", sessionID)
	}
	delete(sm.sessions, sessionID)
	sm.mu.Unlock()

	sm.emitEvent(SessionEvent{
		Type:      "closed",
		SessionID: sessionID,
		Timestamp: time.Now(),
		State:     session,
	})

	// Persist removal
	if sm.stateStore != nil && sm.config.PersistState {
		if err := sm.stateStore.DeleteSession(sessionID); err != nil {
			sm.logger.Error("failed to delete session from store", zap.Error(err))
		}
	}

	sm.logger.Info("session closed",
		zap.String("session_id", sessionID),
		zap.String("username", session.Username))

	return nil
}

// SerializeSession serializes a session state to bytes
func SerializeSession(session *SessionState) ([]byte, error) {
	session.mu.RLock()
	defer session.mu.RUnlock()

	data, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session: %w", err)
	}
	return data, nil
}

// DeserializeSession deserializes a session state from bytes
func DeserializeSession(data []byte) (*SessionState, error) {
	var session SessionState
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}
	return &session, nil
}

// GetSerializedSessions returns all sessions serialized for sync
func (sm *SessionManager) GetSerializedSessions() (map[string][]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string][]byte, len(sm.sessions))
	for id, session := range sm.sessions {
		data, err := SerializeSession(session)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize session %s: %w", id, err)
		}
		result[id] = data
	}
	return result, nil
}

// RestoreSessions restores sessions from serialized data
func (sm *SessionManager) RestoreSessions(data map[string][]byte) (int, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	restored := 0
	for id, sessionData := range data {
		session, err := DeserializeSession(sessionData)
		if err != nil {
			sm.logger.Error("failed to deserialize session",
				zap.String("session_id", id),
				zap.Error(err))
			continue
		}

		if sm.config.ValidateOnRestore {
			if err := sm.validateSession(session); err != nil {
				sm.logger.Warn("invalid session skipped",
					zap.String("session_id", id),
					zap.Error(err))
				continue
			}
		}

		sm.sessions[id] = session
		restored++
	}

	sm.logger.Info("sessions restored", zap.Int("count", restored))
	return restored, nil
}

// validateSession validates session state
func (sm *SessionManager) validateSession(session *SessionState) error {
	if session.SessionID == "" {
		return fmt.Errorf("missing session ID")
	}
	if session.ClientIP == "" {
		return fmt.Errorf("missing client IP")
	}
	if session.Username == "" {
		return fmt.Errorf("missing username")
	}
	// Check if auth context is not expired
	if session.AuthContext != nil && !session.AuthContext.ExpiryTime.IsZero() {
		if time.Now().After(session.AuthContext.ExpiryTime) {
			return fmt.Errorf("authentication context expired")
		}
	}
	return nil
}

// GetSessionStats returns session statistics
func (sm *SessionManager) GetSessionStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	totalFiles := 0
	totalLocks := 0
	totalTreeConns := 0

	for _, session := range sm.sessions {
		session.mu.RLock()
		totalFiles += len(session.OpenFiles)
		totalLocks += len(session.Locks)
		totalTreeConns += len(session.TreeConnections)
		session.mu.RUnlock()
	}

	return map[string]interface{}{
		"total_sessions":    len(sm.sessions),
		"total_open_files":  totalFiles,
		"total_locks":       totalLocks,
		"total_tree_conns":  totalTreeConns,
		"max_sessions":      sm.config.MaxSessions,
	}
}

// AddEventListener adds a session event listener
func (sm *SessionManager) AddEventListener(listener SessionEventListener) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.eventListeners = append(sm.eventListeners, listener)
}

// emitEvent emits a session event
func (sm *SessionManager) emitEvent(event SessionEvent) {
	for _, listener := range sm.eventListeners {
		go listener(event)
	}
}

// loadPersistedSessions loads sessions from the state store
func (sm *SessionManager) loadPersistedSessions() error {
	if sm.stateStore == nil {
		return nil
	}

	sessionIDs, err := sm.stateStore.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	for _, id := range sessionIDs {
		session, err := sm.stateStore.LoadSession(id)
		if err != nil {
			sm.logger.Warn("failed to load session",
				zap.String("session_id", id),
				zap.Error(err))
			continue
		}
		sm.sessions[id] = session
	}

	return nil
}

// cleanupLoop periodically cleans up idle sessions
func (sm *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(sm.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sm.stopCh:
			return
		case <-ticker.C:
			sm.cleanupIdleSessions()
		}
	}
}

// cleanupIdleSessions removes idle sessions
func (sm *SessionManager) cleanupIdleSessions() {
	sm.mu.Lock()
	var toClose []string

	for id, session := range sm.sessions {
		session.mu.RLock()
		idle := time.Since(session.LastActivity)
		session.mu.RUnlock()

		if idle > sm.config.IdleTimeout {
			toClose = append(toClose, id)
		}
	}
	sm.mu.Unlock()

	for _, id := range toClose {
		if err := sm.CloseSession(id); err != nil {
			sm.logger.Error("failed to close idle session",
				zap.String("session_id", id),
				zap.Error(err))
		} else {
			sm.logger.Info("idle session cleaned up",
				zap.String("session_id", id))
		}
	}
}

// PersistSessions persists all sessions to the state store
func (sm *SessionManager) PersistSessions() error {
	if sm.stateStore == nil || !sm.config.PersistState {
		return nil
	}

	sm.mu.RLock()
	sessions := make([]*SessionState, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}
	sm.mu.RUnlock()

	if err := sm.stateStore.SaveBulk(sessions); err != nil {
		return fmt.Errorf("failed to persist sessions: %w", err)
	}

	sm.logger.Debug("sessions persisted", zap.Int("count", len(sessions)))
	return nil
}

// GetSessionCount returns the current session count
func (sm *SessionManager) GetSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}
