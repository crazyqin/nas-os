package lock

import (
	"fmt"
	"time"
)

// ========== SMB 锁适配器 ==========

// SMBLockAdapter SMB 锁适配器.
type SMBLockAdapter struct {
	manager *Manager
}

// NewSMBLockAdapter 创建 SMB 锁适配器.
func NewSMBLockAdapter(manager *Manager) *SMBLockAdapter {
	return &SMBLockAdapter{manager: manager}
}

// Lock 锁定文件..
func (a *SMBLockAdapter) Lock(filePath string, owner string, exclusive bool) error {
	lockType := LockTypeShared
	if exclusive {
		lockType = LockTypeExclusive
	}

	req := &LockRequest{
		FilePath: filePath,
		LockType: lockType,
		LockMode: LockModeAuto, // SMB 通常使用自动锁
		Owner:    owner,
		Protocol: "SMB",
		Timeout:  int(a.manager.config.DefaultTimeout.Seconds()),
	}

	_, _, err := a.manager.Lock(req)
	return err
}

// Unlock 解锁文件.
func (a *SMBLockAdapter) Unlock(filePath string, owner string) error {
	return a.manager.UnlockByPath(filePath, owner)
}

// IsLocked 检查文件是否被锁定.
func (a *SMBLockAdapter) IsLocked(filePath string) bool {
	return a.manager.IsLocked(filePath)
}

// GetLockOwner 获取锁持有者.
func (a *SMBLockAdapter) GetLockOwner(filePath string) (string, error) {
	info, err := a.manager.GetLockByPath(filePath)
	if err != nil {
		return "", err
	}
	return info.Owner, nil
}

// GetLockInfo 获取锁详情.
func (a *SMBLockAdapter) GetLockInfo(filePath string) (*LockInfo, error) {
	return a.manager.GetLockByPath(filePath)
}

// ========== NFS 锁适配器 ==========

// NFSLockAdapter NFS 锁适配器.
type NFSLockAdapter struct {
	manager *Manager
}

// NewNFSLockAdapter 创建 NFS 锁适配器.
func NewNFSLockAdapter(manager *Manager) *NFSLockAdapter {
	return &NFSLockAdapter{manager: manager}
}

// Lock 锁定文件..
func (a *NFSLockAdapter) Lock(filePath string, owner string, exclusive bool) error {
	lockType := LockTypeShared
	if exclusive {
		lockType = LockTypeExclusive
	}

	req := &LockRequest{
		FilePath: filePath,
		LockType: lockType,
		LockMode: LockModeAdvisory, // NFS 通常使用建议锁
		Owner:    owner,
		Protocol: "NFS",
		Timeout:  int(a.manager.config.DefaultTimeout.Seconds()),
	}

	_, _, err := a.manager.Lock(req)
	return err
}

// Unlock 解锁文件.
func (a *NFSLockAdapter) Unlock(filePath string, owner string) error {
	return a.manager.UnlockByPath(filePath, owner)
}

// IsLocked 检查文件是否被锁定.
func (a *NFSLockAdapter) IsLocked(filePath string) bool {
	return a.manager.IsLocked(filePath)
}

// GetLockOwner 获取锁持有者.
func (a *NFSLockAdapter) GetLockOwner(filePath string) (string, error) {
	info, err := a.manager.GetLockByPath(filePath)
	if err != nil {
		return "", err
	}
	return info.Owner, nil
}

// GetLockInfo 获取锁详情.
func (a *NFSLockAdapter) GetLockInfo(filePath string) (*LockInfo, error) {
	return a.manager.GetLockByPath(filePath)
}

// ========== WebDAV 锁适配器 ==========

// WebDAVLockAdapter WebDAV 锁适配器.
type WebDAVLockAdapter struct {
	manager *Manager
}

// NewWebDAVLockAdapter 创建 WebDAV 锁适配器.
func NewWebDAVLockAdapter(manager *Manager) *WebDAVLockAdapter {
	return &WebDAVLockAdapter{manager: manager}
}

// Lock 锁定文件..
func (a *WebDAVLockAdapter) Lock(filePath string, owner string, exclusive bool) error {
	lockType := LockTypeShared
	if exclusive {
		lockType = LockTypeExclusive
	}

	req := &LockRequest{
		FilePath: filePath,
		LockType: lockType,
		LockMode: LockModeManual, // WebDAV 通常使用手动锁
		Owner:    owner,
		Protocol: "WebDAV",
		Timeout:  int(a.manager.config.DefaultTimeout.Seconds()),
	}

	_, _, err := a.manager.Lock(req)
	return err
}

// Unlock 解锁文件.
func (a *WebDAVLockAdapter) Unlock(filePath string, owner string) error {
	return a.manager.UnlockByPath(filePath, owner)
}

// IsLocked 检查文件是否被锁定.
func (a *WebDAVLockAdapter) IsLocked(filePath string) bool {
	return a.manager.IsLocked(filePath)
}

// GetLockOwner 获取锁持有者.
func (a *WebDAVLockAdapter) GetLockOwner(filePath string) (string, error) {
	info, err := a.manager.GetLockByPath(filePath)
	if err != nil {
		return "", err
	}
	return info.Owner, nil
}

// GetLockInfo 获取锁详情.
func (a *WebDAVLockAdapter) GetLockInfo(filePath string) (*LockInfo, error) {
	return a.manager.GetLockByPath(filePath)
}

// ========== Drive 客户端锁适配器（参考群晖 Drive）==========

// DriveLockAdapter Drive 客户端锁适配器.
type DriveLockAdapter struct {
	manager *Manager
}

// NewDriveLockAdapter 创建 Drive 锁适配器.
func NewDriveLockAdapter(manager *Manager) *DriveLockAdapter {
	return &DriveLockAdapter{manager: manager}
}

// LockWithNotification 锁定文件并发送通知.
func (a *DriveLockAdapter) LockWithNotification(filePath, owner, ownerName, clientID string, exclusive bool, timeout int, notifyOthers bool) (*LockInfo, *LockConflict, error) {
	lockType := LockTypeShared
	if exclusive {
		lockType = LockTypeExclusive
	}

	strategy := ConflictStrategyReject
	if notifyOthers {
		strategy = ConflictStrategyNotify
	}

	req := &LockRequest{
		FilePath:         filePath,
		LockType:         lockType,
		LockMode:         LockModeAuto, // Drive 客户端使用自动锁
		Owner:            owner,
		OwnerName:        ownerName,
		ClientID:         clientID,
		Protocol:         "Drive",
		Timeout:          timeout,
		ConflictStrategy: strategy,
	}

	lock, conflict, err := a.manager.Lock(req)
	if err != nil {
		return nil, conflict, err
	}

	return lock.ToInfo(), conflict, nil
}

// RequestExclusiveLock 请求独占锁（带等待）.
func (a *DriveLockAdapter) RequestExclusiveLock(filePath, owner, ownerName, clientID string, timeout, waitTimeout int) (*LockInfo, error) {
	req := &LockRequest{
		FilePath:         filePath,
		LockType:         LockTypeExclusive,
		LockMode:         LockModeManual,
		Owner:            owner,
		OwnerName:        ownerName,
		ClientID:         clientID,
		Protocol:         "Drive",
		Timeout:          timeout,
		WaitTimeout:      waitTimeout,
		ConflictStrategy: ConflictStrategyWait,
	}

	lock, _, err := a.manager.Lock(req)
	if err != nil {
		return nil, err
	}

	return lock.ToInfo(), nil
}

// Unlock 解锁文件.
func (a *DriveLockAdapter) Unlock(filePath string, owner string) error {
	return a.manager.UnlockByPath(filePath, owner)
}

// IsLocked 检查文件是否被锁定.
func (a *DriveLockAdapter) IsLocked(filePath string) bool {
	return a.manager.IsLocked(filePath)
}

// GetLockOwner 获取锁持有者.
func (a *DriveLockAdapter) GetLockOwner(filePath string) (string, error) {
	info, err := a.manager.GetLockByPath(filePath)
	if err != nil {
		return "", err
	}
	return info.Owner, nil
}

// GetLockInfo 获取锁详情.
func (a *DriveLockAdapter) GetLockInfo(filePath string) (*LockInfo, error) {
	return a.manager.GetLockByPath(filePath)
}

// ========== 协作锁管理器 ==========

// CollaborativeLockManager 协作锁管理器（用于多用户协作）.
type CollaborativeLockManager struct {
	manager *Manager
}

// NewCollaborativeLockManager 创建协作锁管理器.
func NewCollaborativeLockManager(manager *Manager) *CollaborativeLockManager {
	return &CollaborativeLockManager{manager: manager}
}

// JoinSharedLock 加入共享锁（多人协作）.
func (c *CollaborativeLockManager) JoinSharedLock(filePath, owner, ownerName, clientID string) (*LockInfo, error) {
	req := &LockRequest{
		FilePath:  filePath,
		LockType:  LockTypeShared,
		LockMode:  LockModeAuto,
		Owner:     owner,
		OwnerName: ownerName,
		ClientID:  clientID,
		Protocol:  "Collab",
	}

	lock, _, err := c.manager.Lock(req)
	if err != nil {
		return nil, err
	}

	return lock.ToInfo(), nil
}

// RequestEditLock 请求编辑锁（从共享升级到独占）.
func (c *CollaborativeLockManager) RequestEditLock(lockID, owner string) error {
	return c.manager.UpgradeLock(lockID, owner)
}

// ReleaseEditLock 释放编辑锁（降级回共享）.
func (c *CollaborativeLockManager) ReleaseEditLock(lockID, owner string) error {
	return c.manager.DowngradeLock(lockID, owner)
}

// GetCollaborators 获取文件的所有协作者.
func (c *CollaborativeLockManager) GetCollaborators(filePath string) ([]*SharedOwner, error) {
	info, err := c.manager.GetLockByPath(filePath)
	if err != nil {
		return nil, err
	}

	if info.LockType == "shared" {
		return info.SharedOwners, nil
	}

	// 独占锁只有一个持有者
	return []*SharedOwner{
		{
			Owner:     info.Owner,
			OwnerName: info.OwnerName,
			ClientID:  info.ClientID,
			Protocol:  info.Protocol,
		},
	}, nil
}

// ========== 锁通知服务 ==========

// LockNotification 锁通知.
type LockNotification struct {
	Type        NotificationType `json:"type"`
	FilePath    string           `json:"filePath"`
	FileName    string           `json:"fileName"`
	LockID      string           `json:"lockId,omitempty"`
	Owner       string           `json:"owner"`
	OwnerName   string           `json:"ownerName"`
	Message     string           `json:"message"`
	Timestamp   time.Time        `json:"timestamp"`
	RequestedBy string           `json:"requestedBy,omitempty"`
}

// NotificationType 通知类型.
type NotificationType string

const (
	// NotificationTypeLockRequested 有人请求锁.
	NotificationTypeLockRequested NotificationType = "lock_requested"
	// NotificationTypeLockAcquired 锁已获取.
	NotificationTypeLockAcquired NotificationType = "lock_acquired"
	// NotificationTypeLockReleased 锁已释放.
	NotificationTypeLockReleased NotificationType = "lock_released"
	// NotificationTypeLockPreempted 锁被抢占.
	NotificationTypeLockPreempted NotificationType = "lock_preempted"
	// NotificationTypeLockExpired 锁已过期.
	NotificationTypeLockExpired NotificationType = "lock_expired"
	// NotificationTypeCollaboratorJoined 协作者加入.
	NotificationTypeCollaboratorJoined NotificationType = "collaborator_joined"
	// NotificationTypeCollaboratorLeft 协作者离开.
	NotificationTypeCollaboratorLeft NotificationType = "collaborator_left"
)

// NotificationHandler 通知处理器.
type NotificationHandler func(notification *LockNotification)

// LockNotificationService 锁通知服务.
type LockNotificationService struct {
	manager  *Manager
	handlers []NotificationHandler
}

// NewLockNotificationService 创建锁通知服务.
func NewLockNotificationService(manager *Manager) *LockNotificationService {
	return &LockNotificationService{
		manager:  manager,
		handlers: make([]NotificationHandler, 0),
	}
}

// RegisterHandler 注册通知处理器.
func (s *LockNotificationService) RegisterHandler(handler NotificationHandler) {
	s.handlers = append(s.handlers, handler)
}

// Notify 发送通知.
func (s *LockNotificationService) Notify(notification *LockNotification) {
	notification.Timestamp = time.Now()
	for _, handler := range s.handlers {
		go handler(notification)
	}
}

// NotifyLockRequested 通知锁请求.
func (s *LockNotificationService) NotifyLockRequested(filePath, fileName, owner, ownerName, requestedBy string) {
	s.Notify(&LockNotification{
		Type:        NotificationTypeLockRequested,
		FilePath:    filePath,
		FileName:    fileName,
		Owner:       owner,
		OwnerName:   ownerName,
		RequestedBy: requestedBy,
		Message:     fmt.Sprintf("%s is requesting access to %s", requestedBy, fileName),
	})
}

// NotifyLockPreempted 通知锁被抢占.
func (s *LockNotificationService) NotifyLockPreempted(lockID, filePath, fileName, owner, ownerName, preemptor string) {
	s.Notify(&LockNotification{
		Type:      NotificationTypeLockPreempted,
		FilePath:  filePath,
		FileName:  fileName,
		LockID:    lockID,
		Owner:     owner,
		OwnerName: ownerName,
		Message:   fmt.Sprintf("Your lock on %s has been released by %s", fileName, preemptor),
	})
}

// NotifyCollaboratorJoined 通知协作者加入.
func (s *LockNotificationService) NotifyCollaboratorJoined(filePath, fileName, collaborator string) {
	s.Notify(&LockNotification{
		Type:     NotificationTypeCollaboratorJoined,
		FilePath: filePath,
		FileName: fileName,
		Message:  fmt.Sprintf("%s has joined the collaboration", collaborator),
	})
}
