// Package enhanced provides enhanced audit logging capabilities
// SMB Audit - SMB审计日志增强 (参考TrueNAS Scale 24.04)
package enhanced

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ========== SMB审计级别 ==========

// SMAuditLevel SMB审计级别.
type SMAuditLevel string

const (
	// SMAuditLevelNone 不记录审计日志.
	SMAuditLevelNone SMAuditLevel = "none"
	// SMAuditLevelMinimal 最小审计：仅记录连接/断开.
	SMAuditLevelMinimal SMAuditLevel = "minimal"
	// SMAuditLevelStandard 标准审计：记录连接、文件操作摘要.
	SMAuditLevelStandard SMAuditLevel = "standard"
	// SMAuditLevelDetailed 详细审计：记录所有文件操作详情.
	SMAuditLevelDetailed SMAuditLevel = "detailed"
	// SMAuditLevelFull 完整审计：记录所有操作包括读写内容摘要.
	SMAuditLevelFull SMAuditLevel = "full"
)

// auditLevelValue 级别数值映射.
var auditLevelValue = map[SMAuditLevel]int{
	SMAuditLevelNone:     0,
	SMAuditLevelMinimal:  1,
	SMAuditLevelStandard: 2,
	SMAuditLevelDetailed: 3,
	SMAuditLevelFull:     4,
}

// levelAtLeast 检查当前级别是否至少为指定级别.
func (l SMAuditLevel) atLeast(required SMAuditLevel) bool {
	return auditLevelValue[l] >= auditLevelValue[required]
}

// SMAuditConfig SMB审计配置.
type SMAuditConfig struct {
	Enabled             bool          `json:"enabled"`
	Level               SMAuditLevel  `json:"level"`
	LogPath             string        `json:"log_path"`
	MaxLogAgeDays       int           `json:"max_log_age_days"`
	MaxLogSizeMB        int           `json:"max_log_size_mb"`
	MaxEntriesPerFile   int           `json:"max_entries_per_file"`
	RotateInterval      time.Duration `json:"rotate_interval"`
	CompressOldLogs     bool          `json:"compress_old_logs"`
	IncludeContent      bool          `json:"include_content"`       // 是否记录文件内容摘要（仅full级别）
	MaxContentSize      int           `json:"max_content_size"`      // 内容摘要最大字节数
	LogFileRead         bool          `json:"log_file_read"`         // 记录文件读取
	LogFileWrite        bool          `json:"log_file_write"`        // 记录文件写入
	LogFileDelete       bool          `json:"log_file_delete"`       // 记录文件删除
	LogFileRename       bool          `json:"log_file_rename"`       // 记录文件重命名
	LogDirCreate        bool          `json:"log_dir_create"`        // 记录目录创建
	LogDirDelete        bool          `json:"log_dir_delete"`        // 记录目录删除
	LogPermissionChange bool          `json:"log_permission_change"` // 记录权限变更
	LogOwnershipChange  bool          `json:"log_ownership_change"`  // 记录所有者变更
	ExcludeShares       []string      `json:"exclude_shares"`        // 排除审计的共享
	ExcludeUsers        []string      `json:"exclude_users"`         // 排除审计的用户
	ExcludePaths        []string      `json:"exclude_paths"`         // 排除审计的路径模式
}

// DefaultSMAuditConfig 默认SMB审计配置.
func DefaultSMAuditConfig() SMAuditConfig {
	return SMAuditConfig{
		Enabled:             true,
		Level:               SMAuditLevelStandard,
		LogPath:             "/var/log/nas-os/audit/smb",
		MaxLogAgeDays:       90,
		MaxLogSizeMB:        100,
		MaxEntriesPerFile:   100000,
		RotateInterval:      time.Hour * 24,
		CompressOldLogs:     true,
		IncludeContent:      false,
		MaxContentSize:      1024,
		LogFileRead:         true,
		LogFileWrite:        true,
		LogFileDelete:       true,
		LogFileRename:       true,
		LogDirCreate:        true,
		LogDirDelete:        true,
		LogPermissionChange: true,
		LogOwnershipChange:  true,
		ExcludeShares:       []string{"IPC$", "print$"},
		ExcludeUsers:        []string{},
		ExcludePaths:        []string{},
	}
}

// ShouldLog 根据审计级别和配置判断是否需要记录.
func (c SMAuditConfig) ShouldLog(opType string) bool {
	if !c.Enabled || c.Level == SMAuditLevelNone {
		return false
	}

	switch opType {
	case "connect", "disconnect":
		return c.Level.atLeast(SMAuditLevelMinimal)
	case "file_read":
		return c.Level.atLeast(SMAuditLevelStandard) && c.LogFileRead
	case "file_write":
		return c.Level.atLeast(SMAuditLevelStandard) && c.LogFileWrite
	case "file_delete":
		return c.Level.atLeast(SMAuditLevelStandard) && c.LogFileDelete
	case "file_rename":
		return c.Level.atLeast(SMAuditLevelStandard) && c.LogFileRename
	case "dir_create":
		return c.Level.atLeast(SMAuditLevelStandard) && c.LogDirCreate
	case "dir_delete":
		return c.Level.atLeast(SMAuditLevelStandard) && c.LogDirDelete
	case "permission_change":
		return c.Level.atLeast(SMAuditLevelDetailed) && c.LogPermissionChange
	case "ownership_change":
		return c.Level.atLeast(SMAuditLevelDetailed) && c.LogOwnershipChange
	default:
		return c.Level.atLeast(SMAuditLevelStandard)
	}
}

// ========== SMB文件操作审计事件 ==========

// SMBFileOperation SMB文件操作类型.
type SMBFileOperation string

// SMB文件操作类型常量.
const (
	SMBFileOpRead           SMBFileOperation = "read"
	SMBFileOpWrite          SMBFileOperation = "write"
	SMBFileOpDelete         SMBFileOperation = "delete"
	SMBFileOpRename         SMBFileOperation = "rename"
	SMBFileOpCopy           SMBFileOperation = "copy"
	SMBFileOpMove           SMBFileOperation = "move"
	SMBFileOpCreate         SMBFileOperation = "create"
	SMBFileOpOpen           SMBFileOperation = "open"
	SMBFileOpClose          SMBFileOperation = "close"
	SMBFileOpLock           SMBFileOperation = "lock"
	SMBFileOpUnlock         SMBFileOperation = "unlock"
	SMBFileOpSetAttrib      SMBFileOperation = "set_attrib"
	SMBFileOpGetAttrib      SMBFileOperation = "get_attrib"
	SMBFileOpMkdir          SMBFileOperation = "mkdir"
	SMBFileOpRmdir          SMBFileOperation = "rmdir"
	SMBFileOpChangePerms    SMBFileOperation = "change_perms"
	SMBFileOpChangeOwner    SMBFileOperation = "change_owner"
	SMBFileOpTreeConnect    SMBFileOperation = "tree_connect"
	SMBFileOpTreeDisconnect SMBFileOperation = "tree_disconnect"
)

// SMAuditEvent SMB审计事件.
type SMAuditEvent struct {
	EventID       string                 `json:"event_id"`
	Timestamp     time.Time              `json:"timestamp"`
	SessionID     string                 `json:"session_id"`
	ShareName     string                 `json:"share_name"`
	Username      string                 `json:"username,omitempty"`
	Domain        string                 `json:"domain,omitempty"`
	ClientIP      string                 `json:"client_ip"`
	ClientPort    int                    `json:"client_port,omitempty"`
	ComputerName  string                 `json:"computer_name,omitempty"`
	Operation     SMBFileOperation       `json:"operation"`
	FilePath      string                 `json:"file_path"`
	OldPath       string                 `json:"old_path,omitempty"` // 重命名/移动操作的原路径
	NewPath       string                 `json:"new_path,omitempty"` // 重命名/移动操作的新路径
	Status        string                 `json:"status"`             // success, failure, denied
	ErrorCode     int                    `json:"error_code,omitempty"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	BytesRead     int64                  `json:"bytes_read,omitempty"`
	BytesWritten  int64                  `json:"bytes_written,omitempty"`
	Offset        int64                  `json:"offset,omitempty"`
	FileSize      int64                  `json:"file_size,omitempty"`
	IsDirectory   bool                   `json:"is_directory,omitempty"`
	Permissions   string                 `json:"permissions,omitempty"` // 权限掩码
	OldPerms      string                 `json:"old_perms,omitempty"`   // 变更前权限
	NewPerms      string                 `json:"new_perms,omitempty"`   // 变更后权限
	Owner         string                 `json:"owner,omitempty"`
	OldOwner      string                 `json:"old_owner,omitempty"`
	NewOwner      string                 `json:"new_owner,omitempty"`
	LockType      string                 `json:"lock_type,omitempty"` // 共享锁/排他锁
	LockRange     string                 `json:"lock_range,omitempty"`
	Duration      int64                  `json:"duration,omitempty"`         // 操作耗时(ms)
	ContentDigest string                 `json:"content_digest,omitempty"`   // 内容摘要(SHA256前16字节)
	ContentSample string                 `json:"content_sample,omitempty"`   // 内容样本(full级别)
	ProtocolVer   string                 `json:"protocol_version,omitempty"` // SMB1/SMB2/SMB3
	Encryption    string                 `json:"encryption,omitempty"`       // 加密状态
	Details       map[string]interface{} `json:"details,omitempty"`
}

// SMAuditEntry SMB审计日志条目（持久化格式）.
type SMAuditEntry struct {
	ID        string       `json:"id"`
	Event     SMAuditEvent `json:"event"`
	Signature string       `json:"signature,omitempty"` // 防篡改签名
}

// SMAuditStatistics SMB审计统计.
type SMAuditStatistics struct {
	TotalEvents        int64               `json:"total_events"`
	EventsByType       map[string]int64    `json:"events_by_type"`
	EventsByShare      map[string]int64    `json:"events_by_share"`
	EventsByUser       map[string]int64    `json:"events_by_user"`
	EventsByClient     map[string]int64    `json:"events_by_client"`
	BytesRead          int64               `json:"bytes_read"`
	BytesWritten       int64               `json:"bytes_written"`
	FilesDeleted       int64               `json:"files_deleted"`
	FilesCreated       int64               `json:"files_created"`
	DirsCreated        int64               `json:"dirs_created"`
	DirsDeleted        int64               `json:"dirs_deleted"`
	FailedOperations   int64               `json:"failed_operations"`
	DeniedOperations   int64               `json:"denied_operations"`
	TopFiles           []FileAccessCount   `json:"top_files"`
	TopUsers           []UserAccessCount   `json:"top_users"`
	TopClients         []ClientAccessCount `json:"top_clients"`
	HourlyDistribution map[int]int64       `json:"hourly_distribution"`
}

// FileAccessCount 文件访问计数.
type FileAccessCount struct {
	FilePath string `json:"file_path"`
	Count    int64  `json:"count"`
}

// UserAccessCount 用户访问计数.
type UserAccessCount struct {
	Username string `json:"username"`
	Count    int64  `json:"count"`
}

// ClientAccessCount 客户端访问计数.
type ClientAccessCount struct {
	ClientIP string `json:"client_ip"`
	Count    int64  `json:"count"`
}

// SMAuditManager SMB审计管理器.
type SMAuditManager struct {
	config      SMAuditConfig
	events      chan SMAuditEvent
	entries     []SMAuditEntry
	stats       SMAuditStatistics
	mu          sync.RWMutex
	stopCh      chan struct{}
	wg          sync.WaitGroup
	rotateTimer *time.Ticker
	currentFile *os.File
	currentSize int64
}

// NewSMAuditManager 创建SMB审计管理器.
func NewSMAuditManager(config SMAuditConfig) *SMAuditManager {
	if config.LogPath == "" {
		config.LogPath = "/var/log/nas-os/audit/smb"
	}
	if config.MaxLogAgeDays == 0 {
		config.MaxLogAgeDays = 90
	}
	if config.MaxLogSizeMB == 0 {
		config.MaxLogSizeMB = 100
	}
	if config.MaxEntriesPerFile == 0 {
		config.MaxEntriesPerFile = 100000
	}

	m := &SMAuditManager{
		config:  config,
		events:  make(chan SMAuditEvent, 10000),
		entries: make([]SMAuditEntry, 0),
		stats: SMAuditStatistics{
			EventsByType:       make(map[string]int64),
			EventsByShare:      make(map[string]int64),
			EventsByUser:       make(map[string]int64),
			EventsByClient:     make(map[string]int64),
			HourlyDistribution: make(map[int]int64),
		},
		stopCh: make(chan struct{}),
	}

	if config.Enabled {
		// 确保日志目录存在
		if err := os.MkdirAll(config.LogPath, 0750); err == nil {
			// 启动事件处理协程
			m.wg.Add(1)
			go m.processEvents()

			// 启动日志轮转
			if config.RotateInterval > 0 {
				m.rotateTimer = time.NewTicker(config.RotateInterval)
				m.wg.Add(1)
				go m.rotateLogs()
			}

			// 启动旧日志清理
			m.wg.Add(1)
			go m.cleanupOldLogs()
		}
	}

	return m
}

// ========== 审计事件记录方法 ==========

// LogConnect 记录SMB连接.
func (m *SMAuditManager) LogConnect(session *SMBSession) {
	if !m.config.Enabled || m.config.Level == SMAuditLevelNone {
		return
	}

	m.events <- SMAuditEvent{
		EventID:      generateSMBEventID(),
		Timestamp:    time.Now(),
		SessionID:    session.SessionID,
		Username:     session.Username,
		Domain:       session.Domain,
		ClientIP:     session.ClientIP,
		ClientPort:   session.ClientPort,
		ComputerName: session.ComputerName,
		Operation:    "connect",
		Status:       "success",
		ProtocolVer:  session.ProtocolVersion,
		Details: map[string]interface{}{
			"encryption": session.Extra["encryption"],
		},
	}
}

// LogDisconnect 记录SMB断开.
func (m *SMAuditManager) LogDisconnect(sessionID, username, clientIP string, bytesRead, bytesWritten int64) {
	if !m.config.Enabled || m.config.Level == SMAuditLevelNone {
		return
	}

	m.events <- SMAuditEvent{
		EventID:      generateSMBEventID(),
		Timestamp:    time.Now(),
		SessionID:    sessionID,
		Username:     username,
		ClientIP:     clientIP,
		Operation:    "disconnect",
		Status:       "success",
		BytesRead:    bytesRead,
		BytesWritten: bytesWritten,
	}
}

// LogTreeConnect 记录共享连接.
func (m *SMAuditManager) LogTreeConnect(sessionID, shareName, username, clientIP string, permissions string) {
	if !m.config.ShouldLog("tree_connect") {
		return
	}

	// 检查排除列表
	if m.isExcluded(shareName, username, "") {
		return
	}

	m.events <- SMAuditEvent{
		EventID:     generateSMBEventID(),
		Timestamp:   time.Now(),
		SessionID:   sessionID,
		ShareName:   shareName,
		Username:    username,
		ClientIP:    clientIP,
		Operation:   SMBFileOpTreeConnect,
		Status:      "success",
		Permissions: permissions,
	}
}

// LogTreeDisconnect 记录共享断开.
func (m *SMAuditManager) LogTreeDisconnect(sessionID, shareName, username, clientIP string) {
	if !m.config.ShouldLog("tree_disconnect") {
		return
	}

	if m.isExcluded(shareName, username, "") {
		return
	}

	m.events <- SMAuditEvent{
		EventID:   generateSMBEventID(),
		Timestamp: time.Now(),
		SessionID: sessionID,
		ShareName: shareName,
		Username:  username,
		ClientIP:  clientIP,
		Operation: SMBFileOpTreeDisconnect,
		Status:    "success",
	}
}

// LogFileOpen 记录文件打开.
func (m *SMAuditManager) LogFileOpen(sessionID, shareName, username, clientIP, filePath string, accessMode string, isDirectory bool) {
	if !m.config.ShouldLog("file_open") {
		return
	}

	if m.isExcluded(shareName, username, filePath) {
		return
	}

	m.events <- SMAuditEvent{
		EventID:     generateSMBEventID(),
		Timestamp:   time.Now(),
		SessionID:   sessionID,
		ShareName:   shareName,
		Username:    username,
		ClientIP:    clientIP,
		Operation:   SMBFileOpOpen,
		FilePath:    filePath,
		IsDirectory: isDirectory,
		Status:      "success",
		Details: map[string]interface{}{
			"access_mode": accessMode,
		},
	}
}

// LogFileClose 记录文件关闭.
func (m *SMAuditManager) LogFileClose(sessionID, shareName, username, clientIP, filePath string, bytesRead, bytesWritten int64) {
	if !m.config.ShouldLog("file_close") {
		return
	}

	if m.isExcluded(shareName, username, filePath) {
		return
	}

	m.events <- SMAuditEvent{
		EventID:      generateSMBEventID(),
		Timestamp:    time.Now(),
		SessionID:    sessionID,
		ShareName:    shareName,
		Username:     username,
		ClientIP:     clientIP,
		Operation:    SMBFileOpClose,
		FilePath:     filePath,
		BytesRead:    bytesRead,
		BytesWritten: bytesWritten,
		Status:       "success",
	}
}

// LogFileRead 记录文件读取.
func (m *SMAuditManager) LogFileRead(sessionID, shareName, username, clientIP, filePath string, offset, length int64) {
	if !m.config.ShouldLog("file_read") {
		return
	}

	if m.isExcluded(shareName, username, filePath) {
		return
	}

	m.events <- SMAuditEvent{
		EventID:   generateSMBEventID(),
		Timestamp: time.Now(),
		SessionID: sessionID,
		ShareName: shareName,
		Username:  username,
		ClientIP:  clientIP,
		Operation: SMBFileOpRead,
		FilePath:  filePath,
		Offset:    offset,
		BytesRead: length,
		Status:    "success",
	}
}

// LogFileWrite 记录文件写入.
func (m *SMAuditManager) LogFileWrite(sessionID, shareName, username, clientIP, filePath string, offset, length int64, contentDigest string) {
	if !m.config.ShouldLog("file_write") {
		return
	}

	if m.isExcluded(shareName, username, filePath) {
		return
	}

	event := SMAuditEvent{
		EventID:       generateSMBEventID(),
		Timestamp:     time.Now(),
		SessionID:     sessionID,
		ShareName:     shareName,
		Username:      username,
		ClientIP:      clientIP,
		Operation:     SMBFileOpWrite,
		FilePath:      filePath,
		Offset:        offset,
		BytesWritten:  length,
		ContentDigest: contentDigest,
		Status:        "success",
	}

	// Full级别记录内容样本 (在外部设置event.ContentSample)
	_ = m.config.Level == SMAuditLevelFull && m.config.IncludeContent

	m.events <- event
}

// LogFileDelete 记录文件删除.
func (m *SMAuditManager) LogFileDelete(sessionID, shareName, username, clientIP, filePath string, isDirectory bool) {
	if !m.config.ShouldLog("file_delete") {
		return
	}

	if m.isExcluded(shareName, username, filePath) {
		return
	}

	m.events <- SMAuditEvent{
		EventID:     generateSMBEventID(),
		Timestamp:   time.Now(),
		SessionID:   sessionID,
		ShareName:   shareName,
		Username:    username,
		ClientIP:    clientIP,
		Operation:   SMBFileOpDelete,
		FilePath:    filePath,
		IsDirectory: isDirectory,
		Status:      "success",
	}
}

// LogFileRename 记录文件重命名.
func (m *SMAuditManager) LogFileRename(sessionID, shareName, username, clientIP, oldPath, newPath string, isDirectory bool) {
	if !m.config.ShouldLog("file_rename") {
		return
	}

	if m.isExcluded(shareName, username, oldPath) {
		return
	}

	m.events <- SMAuditEvent{
		EventID:     generateSMBEventID(),
		Timestamp:   time.Now(),
		SessionID:   sessionID,
		ShareName:   shareName,
		Username:    username,
		ClientIP:    clientIP,
		Operation:   SMBFileOpRename,
		OldPath:     oldPath,
		NewPath:     newPath,
		FilePath:    newPath,
		IsDirectory: isDirectory,
		Status:      "success",
	}
}

// LogFileCreate 记录文件创建.
func (m *SMAuditManager) LogFileCreate(sessionID, shareName, username, clientIP, filePath string, isDirectory bool, permissions string) {
	if !m.config.ShouldLog("file_create") {
		return
	}

	if m.isExcluded(shareName, username, filePath) {
		return
	}

	m.events <- SMAuditEvent{
		EventID:     generateSMBEventID(),
		Timestamp:   time.Now(),
		SessionID:   sessionID,
		ShareName:   shareName,
		Username:    username,
		ClientIP:    clientIP,
		Operation:   SMBFileOpCreate,
		FilePath:    filePath,
		IsDirectory: isDirectory,
		Permissions: permissions,
		Status:      "success",
	}
}

// LogPermissionChange 记录权限变更.
func (m *SMAuditManager) LogPermissionChange(sessionID, shareName, username, clientIP, filePath, oldPerms, newPerms string) {
	if !m.config.ShouldLog("permission_change") {
		return
	}

	if m.isExcluded(shareName, username, filePath) {
		return
	}

	m.events <- SMAuditEvent{
		EventID:   generateSMBEventID(),
		Timestamp: time.Now(),
		SessionID: sessionID,
		ShareName: shareName,
		Username:  username,
		ClientIP:  clientIP,
		Operation: SMBFileOpChangePerms,
		FilePath:  filePath,
		OldPerms:  oldPerms,
		NewPerms:  newPerms,
		Status:    "success",
	}
}

// LogOwnershipChange 记录所有者变更.
func (m *SMAuditManager) LogOwnershipChange(sessionID, shareName, username, clientIP, filePath, oldOwner, newOwner string) {
	if !m.config.ShouldLog("ownership_change") {
		return
	}

	if m.isExcluded(shareName, username, filePath) {
		return
	}

	m.events <- SMAuditEvent{
		EventID:   generateSMBEventID(),
		Timestamp: time.Now(),
		SessionID: sessionID,
		ShareName: shareName,
		Username:  username,
		ClientIP:  clientIP,
		Operation: SMBFileOpChangeOwner,
		FilePath:  filePath,
		OldOwner:  oldOwner,
		NewOwner:  newOwner,
		Status:    "success",
	}
}

// LogFileLock 记录文件锁定.
func (m *SMAuditManager) LogFileLock(sessionID, shareName, username, clientIP, filePath, lockType, lockRange string) {
	if !m.config.ShouldLog("file_lock") {
		return
	}

	if m.isExcluded(shareName, username, filePath) {
		return
	}

	m.events <- SMAuditEvent{
		EventID:   generateSMBEventID(),
		Timestamp: time.Now(),
		SessionID: sessionID,
		ShareName: shareName,
		Username:  username,
		ClientIP:  clientIP,
		Operation: SMBFileOpLock,
		FilePath:  filePath,
		LockType:  lockType,
		LockRange: lockRange,
		Status:    "success",
	}
}

// LogFileUnlock 记录文件解锁.
func (m *SMAuditManager) LogFileUnlock(sessionID, shareName, username, clientIP, filePath string) {
	if !m.config.ShouldLog("file_unlock") {
		return
	}

	if m.isExcluded(shareName, username, filePath) {
		return
	}

	m.events <- SMAuditEvent{
		EventID:   generateSMBEventID(),
		Timestamp: time.Now(),
		SessionID: sessionID,
		ShareName: shareName,
		Username:  username,
		ClientIP:  clientIP,
		Operation: SMBFileOpUnlock,
		FilePath:  filePath,
		Status:    "success",
	}
}

// LogOperationFailure 记录操作失败.
func (m *SMAuditManager) LogOperationFailure(sessionID, shareName, username, clientIP, filePath string, operation SMBFileOperation, errorCode int, errorMessage string) {
	if !m.config.Enabled || m.config.Level == SMAuditLevelNone {
		return
	}

	if m.isExcluded(shareName, username, filePath) {
		return
	}

	m.events <- SMAuditEvent{
		EventID:      generateSMBEventID(),
		Timestamp:    time.Now(),
		SessionID:    sessionID,
		ShareName:    shareName,
		Username:     username,
		ClientIP:     clientIP,
		Operation:    operation,
		FilePath:     filePath,
		Status:       "failure",
		ErrorCode:    errorCode,
		ErrorMessage: errorMessage,
	}
}

// LogOperationDenied 记录操作被拒绝.
func (m *SMAuditManager) LogOperationDenied(sessionID, shareName, username, clientIP, filePath string, operation SMBFileOperation, reason string) {
	if !m.config.Enabled || m.config.Level == SMAuditLevelNone {
		return
	}

	if m.isExcluded(shareName, username, filePath) {
		return
	}

	m.events <- SMAuditEvent{
		EventID:      generateSMBEventID(),
		Timestamp:    time.Now(),
		SessionID:    sessionID,
		ShareName:    shareName,
		Username:     username,
		ClientIP:     clientIP,
		Operation:    operation,
		FilePath:     filePath,
		Status:       "denied",
		ErrorMessage: reason,
	}
}

// ========== 查询和统计方法 ==========

// GetStatistics 获取审计统计.
func (m *SMAuditManager) GetStatistics() SMAuditStatistics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 复制统计数据
	stats := SMAuditStatistics{
		TotalEvents:        m.stats.TotalEvents,
		BytesRead:          m.stats.BytesRead,
		BytesWritten:       m.stats.BytesWritten,
		FilesDeleted:       m.stats.FilesDeleted,
		FilesCreated:       m.stats.FilesCreated,
		DirsCreated:        m.stats.DirsCreated,
		DirsDeleted:        m.stats.DirsDeleted,
		FailedOperations:   m.stats.FailedOperations,
		DeniedOperations:   m.stats.DeniedOperations,
		EventsByType:       make(map[string]int64),
		EventsByShare:      make(map[string]int64),
		EventsByUser:       make(map[string]int64),
		EventsByClient:     make(map[string]int64),
		HourlyDistribution: make(map[int]int64),
	}

	for k, v := range m.stats.EventsByType {
		stats.EventsByType[k] = v
	}
	for k, v := range m.stats.EventsByShare {
		stats.EventsByShare[k] = v
	}
	for k, v := range m.stats.EventsByUser {
		stats.EventsByUser[k] = v
	}
	for k, v := range m.stats.EventsByClient {
		stats.EventsByClient[k] = v
	}
	for k, v := range m.stats.HourlyDistribution {
		stats.HourlyDistribution[k] = v
	}

	return stats
}

// QueryEvents 查询审计事件.
func (m *SMAuditManager) QueryEvents(opts SMAuditQueryOptions) ([]SMAuditEvent, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []SMAuditEvent
	for _, entry := range m.entries {
		if m.matchesQuery(entry.Event, opts) {
			results = append(results, entry.Event)
		}
	}

	total := len(results)

	// 排序（按时间倒序）
	// sort.Slice(results, func(i, j int) bool {
	// 	return results[i].Timestamp.After(results[j].Timestamp)
	// })

	// 分页
	start := opts.Offset
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}

	end := start + opts.Limit
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	if end > total {
		end = total
	}

	return results[start:end], total
}

// SMAuditQueryOptions SMB审计查询选项.
type SMAuditQueryOptions struct {
	Limit     int              `json:"limit"`
	Offset    int              `json:"offset"`
	StartTime *time.Time       `json:"start_time,omitempty"`
	EndTime   *time.Time       `json:"end_time,omitempty"`
	SessionID string           `json:"session_id,omitempty"`
	ShareName string           `json:"share_name,omitempty"`
	Username  string           `json:"username,omitempty"`
	ClientIP  string           `json:"client_ip,omitempty"`
	Operation SMBFileOperation `json:"operation,omitempty"`
	FilePath  string           `json:"file_path,omitempty"`
	Status    string           `json:"status,omitempty"`
}

// matchesQuery 检查事件是否匹配查询条件.
func (m *SMAuditManager) matchesQuery(event SMAuditEvent, opts SMAuditQueryOptions) bool {
	if opts.StartTime != nil && event.Timestamp.Before(*opts.StartTime) {
		return false
	}
	if opts.EndTime != nil && event.Timestamp.After(*opts.EndTime) {
		return false
	}
	if opts.SessionID != "" && event.SessionID != opts.SessionID {
		return false
	}
	if opts.ShareName != "" && event.ShareName != opts.ShareName {
		return false
	}
	if opts.Username != "" && event.Username != opts.Username {
		return false
	}
	if opts.ClientIP != "" && event.ClientIP != opts.ClientIP {
		return false
	}
	if opts.Operation != "" && event.Operation != opts.Operation {
		return false
	}
	if opts.FilePath != "" && event.FilePath != opts.FilePath {
		return false
	}
	if opts.Status != "" && event.Status != opts.Status {
		return false
	}
	return true
}

// ========== 内部方法 ==========

// isExcluded 检查是否在排除列表中.
func (m *SMAuditManager) isExcluded(shareName, username, filePath string) bool {
	// 检查排除的共享
	for _, s := range m.config.ExcludeShares {
		if s == shareName {
			return true
		}
	}

	// 检查排除的用户
	for _, u := range m.config.ExcludeUsers {
		if u == username {
			return true
		}
	}

	// 检查排除的路径
	for _, p := range m.config.ExcludePaths {
		if filePath != "" && (filePath == p || len(filePath) >= len(p) && filePath[:len(p)] == p) {
			return true
		}
	}

	return false
}

// processEvents 处理事件队列.
func (m *SMAuditManager) processEvents() {
	defer m.wg.Done()

	for {
		select {
		case event := <-m.events:
			m.writeEvent(event)
		case <-m.stopCh:
			// 处理剩余事件
			for len(m.events) > 0 {
				event := <-m.events
				m.writeEvent(event)
			}
			return
		}
	}
}

// writeEvent 写入事件到内存和文件.
func (m *SMAuditManager) writeEvent(event SMAuditEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 创建审计条目
	entry := SMAuditEntry{
		ID:    generateSMBEventID(),
		Event: event,
	}

	// 添加到内存
	m.entries = append(m.entries, entry)

	// 限制内存条目数量
	maxMemory := 100000
	if len(m.entries) > maxMemory {
		m.entries = m.entries[len(m.entries)-maxMemory:]
	}

	// 更新统计
	m.updateStats(event)

	// 写入文件
	m.writeToFile(entry)
}

// updateStats 更新统计数据.
func (m *SMAuditManager) updateStats(event SMAuditEvent) {
	m.stats.TotalEvents++
	m.stats.EventsByType[string(event.Operation)]++
	m.stats.EventsByShare[event.ShareName]++
	m.stats.EventsByUser[event.Username]++
	m.stats.EventsByClient[event.ClientIP]++
	m.stats.HourlyDistribution[event.Timestamp.Hour()]++

	m.stats.BytesRead += event.BytesRead
	m.stats.BytesWritten += event.BytesWritten

	switch event.Operation {
	case SMBFileOpDelete:
		if event.IsDirectory {
			m.stats.DirsDeleted++
		} else {
			m.stats.FilesDeleted++
		}
	case SMBFileOpCreate:
		if event.IsDirectory {
			m.stats.DirsCreated++
		} else {
			m.stats.FilesCreated++
		}
	}

	switch event.Status {
	case "failure":
		m.stats.FailedOperations++
	case "denied":
		m.stats.DeniedOperations++
	}
}

// writeToFile 写入日志文件.
func (m *SMAuditManager) writeToFile(entry SMAuditEntry) {
	if m.config.LogPath == "" {
		return
	}

	// 检查是否需要轮转
	if m.currentFile == nil || m.currentSize > int64(m.config.MaxLogSizeMB)*1024*1024 {
		m.rotateFile()
	}

	if m.currentFile == nil {
		return
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	n, err := m.currentFile.Write(append(data, '\n'))
	if err != nil {
		m.currentFile = nil
		return
	}

	m.currentSize += int64(n)
}

// rotateFile 轮转日志文件.
func (m *SMAuditManager) rotateFile() {
	if m.currentFile != nil {
		_ = m.currentFile.Close()
		m.currentFile = nil
	}

	// 创建新日志文件
	filename := fmt.Sprintf("smb-audit-%s.log", time.Now().Format("2006-01-02"))
	filepath := filepath.Join(m.config.LogPath, filename)

	f, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}

	m.currentFile = f
	m.currentSize = 0
}

// rotateLogs 定期轮转日志.
func (m *SMAuditManager) rotateLogs() {
	defer m.wg.Done()

	for {
		select {
		case <-m.rotateTimer.C:
			m.mu.Lock()
			m.rotateFile()
			m.mu.Unlock()
		case <-m.stopCh:
			return
		}
	}
}

// cleanupOldLogs 清理旧日志.
func (m *SMAuditManager) cleanupOldLogs() {
	defer m.wg.Done()

	ticker := time.NewTicker(time.Hour * 24)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.doCleanup()
		case <-m.stopCh:
			return
		}
	}
}

// doCleanup 执行清理.
func (m *SMAuditManager) doCleanup() {
	if m.config.LogPath == "" || m.config.MaxLogAgeDays <= 0 {
		return
	}

	entries, err := os.ReadDir(m.config.LogPath)
	if err != nil {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -m.config.MaxLogAgeDays)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			filePath := filepath.Join(m.config.LogPath, entry.Name())
			if m.config.CompressOldLogs && filepath.Ext(entry.Name()) != ".gz" {
				// 压缩后删除原文件
				_ = compressFile(filePath)
				_ = os.Remove(filePath)
			} else {
				_ = os.Remove(filePath)
			}
		}
	}
}

// compressFile 压缩文件.
func compressFile(filePath string) error {
	// 简单实现，实际可使用 gzip
	return nil
}

// Stop 停止审计管理器.
func (m *SMAuditManager) Stop() {
	close(m.stopCh)
	m.wg.Wait()

	m.mu.Lock()
	if m.currentFile != nil {
		_ = m.currentFile.Close()
	}
	m.mu.Unlock()
}

// UpdateConfig 更新配置.
func (m *SMAuditManager) UpdateConfig(config SMAuditConfig) {
	m.mu.Lock()
	m.config = config
	m.mu.Unlock()
}

// GetConfig 获取配置.
func (m *SMAuditManager) GetConfig() SMAuditConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// generateSMBEventID 生成事件ID.
func generateSMBEventID() string {
	return fmt.Sprintf("smb-%d-%s", time.Now().UnixNano(), randomString(8))
}

// randomString 生成随机字符串.
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
