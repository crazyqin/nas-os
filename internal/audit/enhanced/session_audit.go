// Package enhanced provides enhanced audit logging capabilities
// SMB/NFS Session Audit - 会话审计 (参考TrueNAS Scale 24.04)
package enhanced

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionProtocol 会话协议类型.
type SessionProtocol string

// SMB/NFS Session Protocol Types.
const (
	// ProtocolSMB indicates SMB protocol.
	ProtocolSMB SessionProtocol = "smb"
	// ProtocolNFS indicates NFS protocol.
	ProtocolNFS SessionProtocol = "nfs"
)

// SessionState 会话状态.
type SessionState string

// SessionState constants.
const (
	SessionStateActive       SessionState = "active"
	SessionStateIdle         SessionState = "idle"
	SessionStateDisconnected SessionState = "disconnected"
	SessionStateClosed       SessionState = "closed"
)

// SMBSession SMB会话信息.
type SMBSession struct {
	SessionID       string                 `json:"session_id"`
	ClientIP        string                 `json:"client_ip"`
	ClientPort      int                    `json:"client_port"`
	Username        string                 `json:"username"`
	Domain          string                 `json:"domain,omitempty"`
	ComputerName    string                 `json:"computer_name,omitempty"`
	ProtocolVersion string                 `json:"protocol_version"` // SMB1/SMB2/SMB3
	ConnectedAt     time.Time              `json:"connected_at"`
	LastActivity    time.Time              `json:"last_activity"`
	State           SessionState           `json:"state"`
	TreeConnects    []TreeConnect          `json:"tree_connects,omitempty"`
	OpenFiles       []OpenFileInfo         `json:"open_files,omitempty"`
	BytesRead       int64                  `json:"bytes_read"`
	BytesWritten    int64                  `json:"bytes_written"`
	LockedFiles     []string               `json:"locked_files,omitempty"`
	Extra           map[string]interface{} `json:"extra,omitempty"`
}

// TreeConnect SMB树连接信息.
type TreeConnect struct {
	ShareName   string    `json:"share_name"`
	ConnectTime time.Time `json:"connect_time"`
	Permissions string    `json:"permissions"` // R/W/RW
}

// OpenFileInfo 打开的文件信息.
type OpenFileInfo struct {
	Path       string    `json:"path"`
	ShareName  string    `json:"share_name"`
	OpenTime   time.Time `json:"open_time"`
	AccessMode string    `json:"access_mode"` // read/write/read-write
	LockType   string    `json:"lock_type,omitempty"`
}

// NFSSession NFS会话信息.
type NFSSession struct {
	SessionID    string                 `json:"session_id"`
	ClientIP     string                 `json:"client_ip"`
	ClientPort   int                    `json:"client_port"`
	Protocol     string                 `json:"protocol"` // NFS3/NFS4
	ConnectedAt  time.Time              `json:"connected_at"`
	LastActivity time.Time              `json:"last_activity"`
	State        SessionState           `json:"state"`
	Exports      []ExportMount          `json:"exports,omitempty"`
	OpenFiles    []OpenFileInfo         `json:"open_files,omitempty"`
	BytesRead    int64                  `json:"bytes_read"`
	BytesWritten int64                  `json:"bytes_written"`
	Extra        map[string]interface{} `json:"extra,omitempty"`
}

// ExportMount NFS导出挂载信息.
type ExportMount struct {
	ExportPath  string    `json:"export_path"`
	MountTime   time.Time `json:"mount_time"`
	Permissions string    `json:"permissions"` // ro/rw
}

// SessionAuditEvent 会话审计事件.
type SessionAuditEvent struct {
	EventID   string                 `json:"event_id"`
	Timestamp time.Time              `json:"timestamp"`
	Protocol  SessionProtocol        `json:"protocol"`
	EventType string                 `json:"event_type"` // connect/disconnect/file_open/file_close/file_lock/file_unlock
	SessionID string                 `json:"session_id"`
	ClientIP  string                 `json:"client_ip"`
	Username  string                 `json:"username,omitempty"`
	Resource  string                 `json:"resource,omitempty"`
	Action    string                 `json:"action,omitempty"`
	Status    string                 `json:"status"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// SessionAuditConfig 会话审计配置.
type SessionAuditConfig struct {
	Enabled         bool          `json:"enabled"`
	LogPath         string        `json:"log_path"`
	MaxLogAgeDays   int           `json:"max_log_age_days"`
	MaxLogSizeMB    int           `json:"max_log_size_mb"`
	LogFileOps      bool          `json:"log_file_ops"`     // 记录文件操作
	LogLockOps      bool          `json:"log_lock_ops"`     // 记录锁定操作
	SessionTimeout  time.Duration `json:"session_timeout"`  // 会话超时
	CleanupInterval time.Duration `json:"cleanup_interval"` // 清理间隔
}

// SessionAuditManager 会话审计管理器.
type SessionAuditManager struct {
	config      SessionAuditConfig
	smbSessions map[string]*SMBSession
	nfsSessions map[string]*NFSSession
	mu          sync.RWMutex
	eventChan   chan SessionAuditEvent
	stopChan    chan struct{}
}

// NewSessionAuditManager 创建会话审计管理器.
func NewSessionAuditManager(config SessionAuditConfig) *SessionAuditManager {
	m := &SessionAuditManager{
		config:      config,
		smbSessions: make(map[string]*SMBSession),
		nfsSessions: make(map[string]*NFSSession),
		eventChan:   make(chan SessionAuditEvent, 1000),
		stopChan:    make(chan struct{}),
	}

	if config.Enabled {
		go m.processEvents()
		go m.cleanupSessions()
	}

	return m
}

// LogSMBConnect 记录SMB连接.
func (m *SessionAuditManager) LogSMBConnect(session *SMBSession) {
	if !m.config.Enabled {
		return
	}

	m.mu.Lock()
	m.smbSessions[session.SessionID] = session
	m.mu.Unlock()

	m.eventChan <- SessionAuditEvent{
		EventID:   generateEventID(),
		Timestamp: time.Now(),
		Protocol:  ProtocolSMB,
		EventType: "connect",
		SessionID: session.SessionID,
		ClientIP:  session.ClientIP,
		Username:  session.Username,
		Status:    "success",
	}
}

// LogSMBDisconnect 记录SMB断开.
func (m *SessionAuditManager) LogSMBDisconnect(sessionID string) {
	if !m.config.Enabled {
		return
	}

	m.mu.Lock()
	session, exists := m.smbSessions[sessionID]
	if exists {
		session.State = SessionStateDisconnected
		delete(m.smbSessions, sessionID)
	}
	m.mu.Unlock()

	if exists {
		m.eventChan <- SessionAuditEvent{
			EventID:   generateEventID(),
			Timestamp: time.Now(),
			Protocol:  ProtocolSMB,
			EventType: "disconnect",
			SessionID: sessionID,
			ClientIP:  session.ClientIP,
			Username:  session.Username,
			Status:    "success",
		}
	}
}

// LogNFSConnect 记录NFS连接.
func (m *SessionAuditManager) LogNFSConnect(session *NFSSession) {
	if !m.config.Enabled {
		return
	}

	m.mu.Lock()
	m.nfsSessions[session.SessionID] = session
	m.mu.Unlock()

	m.eventChan <- SessionAuditEvent{
		EventID:   generateEventID(),
		Timestamp: time.Now(),
		Protocol:  ProtocolNFS,
		EventType: "connect",
		SessionID: session.SessionID,
		ClientIP:  session.ClientIP,
		Status:    "success",
	}
}

// LogNFDisconnect 记录NFS断开.
func (m *SessionAuditManager) LogNFDisconnect(sessionID string) {
	if !m.config.Enabled {
		return
	}

	m.mu.Lock()
	session, exists := m.nfsSessions[sessionID]
	if exists {
		session.State = SessionStateDisconnected
		delete(m.nfsSessions, sessionID)
	}
	m.mu.Unlock()

	if exists {
		m.eventChan <- SessionAuditEvent{
			EventID:   generateEventID(),
			Timestamp: time.Now(),
			Protocol:  ProtocolNFS,
			EventType: "disconnect",
			SessionID: sessionID,
			ClientIP:  session.ClientIP,
			Status:    "success",
		}
	}
}

// LogFileOpen 记录文件打开.
func (m *SessionAuditManager) LogFileOpen(protocol SessionProtocol, sessionID, filePath, username string) {
	if !m.config.Enabled || !m.config.LogFileOps {
		return
	}

	var clientIP string
	m.mu.RLock()
	if protocol == ProtocolSMB {
		if s, ok := m.smbSessions[sessionID]; ok {
			clientIP = s.ClientIP
		}
	} else {
		if s, ok := m.nfsSessions[sessionID]; ok {
			clientIP = s.ClientIP
		}
	}
	m.mu.RUnlock()

	m.eventChan <- SessionAuditEvent{
		EventID:   generateEventID(),
		Timestamp: time.Now(),
		Protocol:  protocol,
		EventType: "file_open",
		SessionID: sessionID,
		ClientIP:  clientIP,
		Username:  username,
		Resource:  filePath,
		Action:    "open",
		Status:    "success",
	}
}

// LogFileClose 记录文件关闭.
func (m *SessionAuditManager) LogFileClose(protocol SessionProtocol, sessionID, filePath string) {
	if !m.config.Enabled || !m.config.LogFileOps {
		return
	}

	var clientIP string
	m.mu.RLock()
	if protocol == ProtocolSMB {
		if s, ok := m.smbSessions[sessionID]; ok {
			clientIP = s.ClientIP
		}
	} else {
		if s, ok := m.nfsSessions[sessionID]; ok {
			clientIP = s.ClientIP
		}
	}
	m.mu.RUnlock()

	m.eventChan <- SessionAuditEvent{
		EventID:   generateEventID(),
		Timestamp: time.Now(),
		Protocol:  protocol,
		EventType: "file_close",
		SessionID: sessionID,
		ClientIP:  clientIP,
		Resource:  filePath,
		Action:    "close",
		Status:    "success",
	}
}

// LogFileLock 记录文件锁定.
func (m *SessionAuditManager) LogFileLock(protocol SessionProtocol, sessionID, filePath, lockType, username string) {
	if !m.config.Enabled || !m.config.LogLockOps {
		return
	}

	var clientIP string
	m.mu.RLock()
	if protocol == ProtocolSMB {
		if s, ok := m.smbSessions[sessionID]; ok {
			clientIP = s.ClientIP
			s.LockedFiles = append(s.LockedFiles, filePath)
		}
	} else {
		if s, ok := m.nfsSessions[sessionID]; ok {
			clientIP = s.ClientIP
		}
	}
	m.mu.RUnlock()

	m.eventChan <- SessionAuditEvent{
		EventID:   generateEventID(),
		Timestamp: time.Now(),
		Protocol:  protocol,
		EventType: "file_lock",
		SessionID: sessionID,
		ClientIP:  clientIP,
		Username:  username,
		Resource:  filePath,
		Action:    lockType,
		Status:    "success",
		Details:   map[string]interface{}{"lock_type": lockType},
	}
}

// LogFileUnlock 记录文件解锁.
func (m *SessionAuditManager) LogFileUnlock(protocol SessionProtocol, sessionID, filePath string) {
	if !m.config.Enabled || !m.config.LogLockOps {
		return
	}

	var clientIP string
	m.mu.RLock()
	if protocol == ProtocolSMB {
		if s, ok := m.smbSessions[sessionID]; ok {
			clientIP = s.ClientIP
			// Remove from locked files
			for i, f := range s.LockedFiles {
				if f == filePath {
					s.LockedFiles = append(s.LockedFiles[:i], s.LockedFiles[i+1:]...)
					break
				}
			}
		}
	} else {
		if s, ok := m.nfsSessions[sessionID]; ok {
			clientIP = s.ClientIP
		}
	}
	m.mu.RUnlock()

	m.eventChan <- SessionAuditEvent{
		EventID:   generateEventID(),
		Timestamp: time.Now(),
		Protocol:  protocol,
		EventType: "file_unlock",
		SessionID: sessionID,
		ClientIP:  clientIP,
		Resource:  filePath,
		Action:    "unlock",
		Status:    "success",
	}
}

// GetSMBSessions 获取所有SMB会话.
func (m *SessionAuditManager) GetSMBSessions() []*SMBSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*SMBSession, 0, len(m.smbSessions))
	for _, s := range m.smbSessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// GetNFSSessions 获取所有NFS会话.
func (m *SessionAuditManager) GetNFSSessions() []*NFSSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*NFSSession, 0, len(m.nfsSessions))
	for _, s := range m.nfsSessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// GetSessionStats 获取会话统计.
func (m *SessionAuditManager) GetSessionStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	smbActive := 0
	smbIdle := 0
	for _, s := range m.smbSessions {
		switch s.State {
		case SessionStateActive:
			smbActive++
		case SessionStateIdle:
			smbIdle++
		}
	}

	nfsActive := 0
	nfsIdle := 0
	for _, s := range m.nfsSessions {
		switch s.State {
		case SessionStateActive:
			nfsActive++
		case SessionStateIdle:
			nfsIdle++
		}
	}

	return map[string]interface{}{
		"smb": map[string]int{
			"total":  len(m.smbSessions),
			"active": smbActive,
			"idle":   smbIdle,
		},
		"nfs": map[string]int{
			"total":  len(m.nfsSessions),
			"active": nfsActive,
			"idle":   nfsIdle,
		},
	}
}

// processEvents 处理事件日志.
func (m *SessionAuditManager) processEvents() {
	for {
		select {
		case event := <-m.eventChan:
			m.writeEvent(event)
		case <-m.stopChan:
			return
		}
	}
}

// writeEvent 写入事件日志.
func (m *SessionAuditManager) writeEvent(event SessionAuditEvent) {
	if m.config.LogPath == "" {
		return
	}

	// 确保目录存在
	if err := os.MkdirAll(m.config.LogPath, 0755); err != nil {
		return
	}

	// 按日期分文件
	filename := fmt.Sprintf("session-audit-%s.log", time.Now().Format("2006-01-02"))
	filepath := filepath.Join(m.config.LogPath, filename)

	// 追加写入
	f, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer func() {
		_ = f.Close()
	}()

	data, _ := json.Marshal(event)
	_, _ = f.WriteString(string(data) + "\n")
}

// cleanupSessions 清理过期会话.
func (m *SessionAuditManager) cleanupSessions() {
	if m.config.CleanupInterval == 0 {
		m.config.CleanupInterval = 5 * time.Minute
	}

	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupExpiredSessions()
		case <-m.stopChan:
			return
		}
	}
}

// cleanupExpiredSessions 清理过期会话.
func (m *SessionAuditManager) cleanupExpiredSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	timeout := m.config.SessionTimeout
	if timeout == 0 {
		timeout = 30 * time.Minute
	}

	now := time.Now()

	// 清理SMB会话
	for id, session := range m.smbSessions {
		if now.Sub(session.LastActivity) > timeout {
			delete(m.smbSessions, id)
		}
	}

	// 清理NFS会话
	for id, session := range m.nfsSessions {
		if now.Sub(session.LastActivity) > timeout {
			delete(m.nfsSessions, id)
		}
	}
}

// Stop 停止审计管理器.
func (m *SessionAuditManager) Stop() {
	close(m.stopChan)
}

// generateEventID 生成事件ID.
func generateEventID() string {
	return fmt.Sprintf("evt-%d", time.Now().UnixNano())
}
