// Package enhanced provides SMB audit integration hooks
package enhanced

import (
	"sync"
	"time"
)

// SMAuditHook SMB审计钩子 - 用于集成到SMB管理器
type SMAuditHook struct {
	manager   *SMAuditManager
	sessions  map[string]*SMBSessionInfo
	mu        sync.RWMutex
}

// OpenFileInfoExt 扩展的打开文件信息（包含字节统计）
type OpenFileInfoExt struct {
	OpenFileInfo
	BytesRead    int64
	BytesWritten int64
}

// SMBSessionInfo SMB会话信息缓存
type SMBSessionInfo struct {
	SessionID     string
	Username      string
	ClientIP      string
	ShareName     string
	ConnectedAt   time.Time
	LastActivity  time.Time
	BytesRead     int64
	BytesWritten  int64
	OpenFiles     map[string]*OpenFileInfoExt
}

// OpenFileInfo 使用 session_audit.go 中已定义的类型，这里添加本地扩展字段
// 本地 OpenFileInfoExt 扩展了会话审计中的 OpenFileInfo

// NewSMAuditHook 创建SMB审计钩子
func NewSMAuditHook(manager *SMAuditManager) *SMAuditHook {
	return &SMAuditHook{
		manager:  manager,
		sessions: make(map[string]*SMBSessionInfo),
	}
}

// OnSessionConnect 会话连接钩子
func (h *SMAuditHook) OnSessionConnect(sessionID, username, clientIP, protocolVersion string, computerName string) {
	if h.manager == nil {
		return
	}

	h.mu.Lock()
	h.sessions[sessionID] = &SMBSessionInfo{
		SessionID:    sessionID,
		Username:     username,
		ClientIP:     clientIP,
		ConnectedAt:  time.Now(),
		LastActivity: time.Now(),
		OpenFiles:    make(map[string]*OpenFileInfoExt),
	}
	h.mu.Unlock()

	// 记录连接事件
	h.manager.LogConnect(&SMBSession{
		SessionID:       sessionID,
		Username:        username,
		ClientIP:        clientIP,
		ComputerName:    computerName,
		ProtocolVersion: protocolVersion,
		ConnectedAt:     time.Now(),
		State:           SessionStateActive,
	})
}

// OnSessionDisconnect 会话断开钩子
func (h *SMAuditHook) OnSessionDisconnect(sessionID string) {
	if h.manager == nil {
		return
	}

	h.mu.Lock()
	info, exists := h.sessions[sessionID]
	if exists {
		delete(h.sessions, sessionID)
	}
	h.mu.Unlock()

	if exists {
		h.manager.LogDisconnect(sessionID, info.Username, info.ClientIP, info.BytesRead, info.BytesWritten)
	}
}

// OnTreeConnect 共享连接钩子
func (h *SMAuditHook) OnTreeConnect(sessionID, shareName, permissions string) {
	if h.manager == nil {
		return
	}

	h.mu.Lock()
	if info, exists := h.sessions[sessionID]; exists {
		info.ShareName = shareName
		info.LastActivity = time.Now()
	}
	h.mu.Unlock()

	var username, clientIP string
	h.mu.RLock()
	if info, exists := h.sessions[sessionID]; exists {
		username = info.Username
		clientIP = info.ClientIP
	}
	h.mu.RUnlock()

	h.manager.LogTreeConnect(sessionID, shareName, username, clientIP, permissions)
}

// OnTreeDisconnect 共享断开钩子
func (h *SMAuditHook) OnTreeDisconnect(sessionID, shareName string) {
	if h.manager == nil {
		return
	}

	var username, clientIP string
	h.mu.RLock()
	if info, exists := h.sessions[sessionID]; exists {
		username = info.Username
		clientIP = info.ClientIP
		info.LastActivity = time.Now()
	}
	h.mu.RUnlock()

	h.manager.LogTreeDisconnect(sessionID, shareName, username, clientIP)
}

// OnFileOpen 文件打开钩子
func (h *SMAuditHook) OnFileOpen(sessionID, shareName, filePath, accessMode string, isDirectory bool) {
	if h.manager == nil {
		return
	}

	h.mu.Lock()
	if info, exists := h.sessions[sessionID]; exists {
		info.OpenFiles[filePath] = &OpenFileInfoExt{
			OpenFileInfo: OpenFileInfo{
				Path:       filePath,
				OpenTime:   time.Now(),
				AccessMode: accessMode,
			},
		}
		info.LastActivity = time.Now()
	}
	h.mu.Unlock()

	var username, clientIP string
	h.mu.RLock()
	if info, exists := h.sessions[sessionID]; exists {
		username = info.Username
		clientIP = info.ClientIP
	}
	h.mu.RUnlock()

	h.manager.LogFileOpen(sessionID, shareName, username, clientIP, filePath, accessMode, isDirectory)
}

// OnFileClose 文件关闭钩子
func (h *SMAuditHook) OnFileClose(sessionID, shareName, filePath string, bytesRead, bytesWritten int64) {
	if h.manager == nil {
		return
	}

	h.mu.Lock()
	if info, exists := h.sessions[sessionID]; exists {
		// 累计字节统计
		info.BytesRead += bytesRead
		info.BytesWritten += bytesWritten
		info.LastActivity = time.Now()

		// 移除打开文件记录
		delete(info.OpenFiles, filePath)
	}
	h.mu.Unlock()

	var username, clientIP string
	h.mu.RLock()
	if info, exists := h.sessions[sessionID]; exists {
		username = info.Username
		clientIP = info.ClientIP
	}
	h.mu.RUnlock()

	h.manager.LogFileClose(sessionID, shareName, username, clientIP, filePath, bytesRead, bytesWritten)
}

// OnFileRead 文件读取钩子
func (h *SMAuditHook) OnFileRead(sessionID, shareName, filePath string, offset, length int64) {
	if h.manager == nil {
		return
	}

	h.mu.Lock()
	if info, exists := h.sessions[sessionID]; exists {
		info.LastActivity = time.Now()
		if f, ok := info.OpenFiles[filePath]; ok {
			f.BytesRead += length
		}
	}
	h.mu.Unlock()

	var username, clientIP string
	h.mu.RLock()
	if info, exists := h.sessions[sessionID]; exists {
		username = info.Username
		clientIP = info.ClientIP
	}
	h.mu.RUnlock()

	h.manager.LogFileRead(sessionID, shareName, username, clientIP, filePath, offset, length)
}

// OnFileWrite 文件写入钩子
func (h *SMAuditHook) OnFileWrite(sessionID, shareName, filePath string, offset, length int64, contentDigest string) {
	if h.manager == nil {
		return
	}

	h.mu.Lock()
	if info, exists := h.sessions[sessionID]; exists {
		info.LastActivity = time.Now()
		if f, ok := info.OpenFiles[filePath]; ok {
			f.BytesWritten += length
		}
	}
	h.mu.Unlock()

	var username, clientIP string
	h.mu.RLock()
	if info, exists := h.sessions[sessionID]; exists {
		username = info.Username
		clientIP = info.ClientIP
	}
	h.mu.RUnlock()

	h.manager.LogFileWrite(sessionID, shareName, username, clientIP, filePath, offset, length, contentDigest)
}

// OnFileDelete 文件删除钩子
func (h *SMAuditHook) OnFileDelete(sessionID, shareName, filePath string, isDirectory bool) {
	if h.manager == nil {
		return
	}

	h.mu.Lock()
	if info, exists := h.sessions[sessionID]; exists {
		info.LastActivity = time.Now()
	}
	h.mu.Unlock()

	var username, clientIP string
	h.mu.RLock()
	if info, exists := h.sessions[sessionID]; exists {
		username = info.Username
		clientIP = info.ClientIP
	}
	h.mu.RUnlock()

	h.manager.LogFileDelete(sessionID, shareName, username, clientIP, filePath, isDirectory)
}

// OnFileRename 文件重命名钩子
func (h *SMAuditHook) OnFileRename(sessionID, shareName, oldPath, newPath string, isDirectory bool) {
	if h.manager == nil {
		return
	}

	h.mu.Lock()
	if info, exists := h.sessions[sessionID]; exists {
		info.LastActivity = time.Now()
		// 更新打开文件记录
		if f, ok := info.OpenFiles[oldPath]; ok {
			delete(info.OpenFiles, oldPath)
			info.OpenFiles[newPath] = f
			f.Path = newPath
		}
	}
	h.mu.Unlock()

	var username, clientIP string
	h.mu.RLock()
	if info, exists := h.sessions[sessionID]; exists {
		username = info.Username
		clientIP = info.ClientIP
	}
	h.mu.RUnlock()

	h.manager.LogFileRename(sessionID, shareName, username, clientIP, oldPath, newPath, isDirectory)
}

// OnFileCreate 文件创建钩子
func (h *SMAuditHook) OnFileCreate(sessionID, shareName, filePath string, isDirectory bool, permissions string) {
	if h.manager == nil {
		return
	}

	h.mu.Lock()
	if info, exists := h.sessions[sessionID]; exists {
		info.LastActivity = time.Now()
	}
	h.mu.Unlock()

	var username, clientIP string
	h.mu.RLock()
	if info, exists := h.sessions[sessionID]; exists {
		username = info.Username
		clientIP = info.ClientIP
	}
	h.mu.RUnlock()

	h.manager.LogFileCreate(sessionID, shareName, username, clientIP, filePath, isDirectory, permissions)
}

// OnPermissionChange 权限变更钩子
func (h *SMAuditHook) OnPermissionChange(sessionID, shareName, filePath, oldPerms, newPerms string) {
	if h.manager == nil {
		return
	}

	h.mu.Lock()
	if info, exists := h.sessions[sessionID]; exists {
		info.LastActivity = time.Now()
	}
	h.mu.Unlock()

	var username, clientIP string
	h.mu.RLock()
	if info, exists := h.sessions[sessionID]; exists {
		username = info.Username
		clientIP = info.ClientIP
	}
	h.mu.RUnlock()

	h.manager.LogPermissionChange(sessionID, shareName, username, clientIP, filePath, oldPerms, newPerms)
}

// OnOwnershipChange 所有者变更钩子
func (h *SMAuditHook) OnOwnershipChange(sessionID, shareName, filePath, oldOwner, newOwner string) {
	if h.manager == nil {
		return
	}

	h.mu.Lock()
	if info, exists := h.sessions[sessionID]; exists {
		info.LastActivity = time.Now()
	}
	h.mu.Unlock()

	var username, clientIP string
	h.mu.RLock()
	if info, exists := h.sessions[sessionID]; exists {
		username = info.Username
		clientIP = info.ClientIP
	}
	h.mu.RUnlock()

	h.manager.LogOwnershipChange(sessionID, shareName, username, clientIP, filePath, oldOwner, newOwner)
}

// OnFileLock 文件锁定钩子
func (h *SMAuditHook) OnFileLock(sessionID, shareName, filePath, lockType, lockRange string) {
	if h.manager == nil {
		return
	}

	h.mu.Lock()
	if info, exists := h.sessions[sessionID]; exists {
		info.LastActivity = time.Now()
	}
	h.mu.Unlock()

	var username, clientIP string
	h.mu.RLock()
	if info, exists := h.sessions[sessionID]; exists {
		username = info.Username
		clientIP = info.ClientIP
	}
	h.mu.RUnlock()

	h.manager.LogFileLock(sessionID, shareName, username, clientIP, filePath, lockType, lockRange)
}

// OnFileUnlock 文件解锁钩子
func (h *SMAuditHook) OnFileUnlock(sessionID, shareName, filePath string) {
	if h.manager == nil {
		return
	}

	h.mu.Lock()
	if info, exists := h.sessions[sessionID]; exists {
		info.LastActivity = time.Now()
	}
	h.mu.Unlock()

	var username, clientIP string
	h.mu.RLock()
	if info, exists := h.sessions[sessionID]; exists {
		username = info.Username
		clientIP = info.ClientIP
	}
	h.mu.RUnlock()

	h.manager.LogFileUnlock(sessionID, shareName, username, clientIP, filePath)
}

// OnOperationFailure 操作失败钩子
func (h *SMAuditHook) OnOperationFailure(sessionID, shareName, filePath string, operation SMBFileOperation, errorCode int, errorMessage string) {
	if h.manager == nil {
		return
	}

	var username, clientIP string
	h.mu.RLock()
	if info, exists := h.sessions[sessionID]; exists {
		username = info.Username
		clientIP = info.ClientIP
	}
	h.mu.RUnlock()

	h.manager.LogOperationFailure(sessionID, shareName, username, clientIP, filePath, operation, errorCode, errorMessage)
}

// OnOperationDenied 操作被拒绝钩子
func (h *SMAuditHook) OnOperationDenied(sessionID, shareName, filePath string, operation SMBFileOperation, reason string) {
	if h.manager == nil {
		return
	}

	var username, clientIP string
	h.mu.RLock()
	if info, exists := h.sessions[sessionID]; exists {
		username = info.Username
		clientIP = info.ClientIP
	}
	h.mu.RUnlock()

	h.manager.LogOperationDenied(sessionID, shareName, username, clientIP, filePath, operation, reason)
}

// GetSessionInfo 获取会话信息
func (h *SMAuditHook) GetSessionInfo(sessionID string) *SMBSessionInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sessions[sessionID]
}

// GetOpenFiles 获取会话打开的文件
func (h *SMAuditHook) GetOpenFiles(sessionID string) map[string]*OpenFileInfoExt {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if info, exists := h.sessions[sessionID]; exists {
		result := make(map[string]*OpenFileInfoExt)
		for k, v := range info.OpenFiles {
			result[k] = v
		}
		return result
	}
	return nil
}

// GetActiveSessions 获取活跃会话列表
func (h *SMAuditHook) GetActiveSessions() []*SMBSessionInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]*SMBSessionInfo, 0, len(h.sessions))
	for _, info := range h.sessions {
		result = append(result, info)
	}
	return result
}

// CleanupIdleSessions 清理空闲会话
func (h *SMAuditHook) CleanupIdleSessions(idleTimeout time.Duration) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	count := 0

	for sessionID, info := range h.sessions {
		if now.Sub(info.LastActivity) > idleTimeout {
			h.manager.LogDisconnect(sessionID, info.Username, info.ClientIP, info.BytesRead, info.BytesWritten)
			delete(h.sessions, sessionID)
			count++
		}
	}

	return count
}