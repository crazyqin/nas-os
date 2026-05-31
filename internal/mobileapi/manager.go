// Package mobileapi 提供移动端远程管理API服务
package mobileapi

import (
	"fmt"
	"sync"
	"time"
)

// Manager 移动端API管理器.
type Manager struct {
	mu            sync.RWMutex
	authService   *AuthService
	pushService   *PushService
	syncService   *SyncService
	preferences   map[string]*NotificationPreference // key: userID:deviceID:category
	history       []*NotificationHistoryItem
	conflicts     map[string]*ConflictRecord
	bindings      map[string]*DeviceBinding // key: bindingID
	maxHistory    int
}

// ManagerConfig 管理器配置.
type ManagerConfig struct {
	AuthConfig *AuthConfig `json:"authConfig,omitempty"`
	PushConfig *PushConfig `json:"pushConfig,omitempty"`
	SyncConfig *SyncConfig `json:"syncConfig,omitempty"`
	MaxHistory int         `json:"maxHistory,omitempty"` // 最大历史记录数
}

// DefaultManagerConfig 返回默认管理器配置.
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		AuthConfig: DefaultAuthConfig(),
		PushConfig: DefaultPushConfig(),
		SyncConfig: DefaultSyncConfig(),
		MaxHistory: 5000,
	}
}

// NewManager 创建移动端API管理器.
func NewManager(config *ManagerConfig) *Manager {
	if config == nil {
		config = DefaultManagerConfig()
	}
	if config.MaxHistory <= 0 {
		config.MaxHistory = 5000
	}

	return &Manager{
		authService: NewAuthService(config.AuthConfig),
		pushService: NewPushService(config.PushConfig, 5),
		syncService: NewSyncService(config.SyncConfig),
		preferences: make(map[string]*NotificationPreference),
		history:     make([]*NotificationHistoryItem, 0),
		conflicts:   make(map[string]*ConflictRecord),
		bindings:    make(map[string]*DeviceBinding),
		maxHistory:  config.MaxHistory,
	}
}

// GetAuthService 获取认证服务.
func (m *Manager) GetAuthService() *AuthService {
	return m.authService
}

// GetPushService 获取推送服务.
func (m *Manager) GetPushService() *PushService {
	return m.pushService
}

// GetSyncService 获取同步服务.
func (m *Manager) GetSyncService() *SyncService {
	return m.syncService
}

// ========== 设备管理 ==========

// RegisterDevice 注册设备并创建绑定.
func (m *Manager) RegisterDevice(device *MobileDevice) (*AuthToken, error) {
	token, err := m.authService.RegisterDevice(device)
	if err != nil {
		return nil, err
	}

	// 创建设备绑定
	m.createBinding(device.UserID, device.ID)

	return token, nil
}

// RemoveDevice 移除设备并解绑.
func (m *Manager) RemoveDevice(deviceID string) error {
	device, ok := m.authService.GetDevice(deviceID)
	if !ok {
		return ErrDeviceNotFound
	}

	// 解绑设备
	m.unbindDevice(device.ID)

	return m.authService.RemoveDevice(deviceID)
}

// createBinding 创建设备绑定.
func (m *Manager) createBinding(userID, deviceID string) *DeviceBinding {
	m.mu.Lock()
	defer m.mu.Unlock()

	binding := &DeviceBinding{
		ID:       generateID(),
		UserID:   userID,
		DeviceID: deviceID,
		BoundAt:  time.Now(),
		Active:   true,
	}
	m.bindings[binding.ID] = binding
	return binding
}

// unbindDevice 解绑设备.
func (m *Manager) unbindDevice(deviceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, b := range m.bindings {
		if b.DeviceID == deviceID && b.Active {
			now := time.Now()
			b.Active = false
			b.UnboundAt = &now
		}
	}
}

// ListBindings 列出用户的设备绑定.
func (m *Manager) ListBindings(userID string) []*DeviceBinding {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var bindings []*DeviceBinding
	for _, b := range m.bindings {
		if b.UserID == userID && b.Active {
			bindings = append(bindings, b)
		}
	}
	return bindings
}

// ========== 通知偏好管理 ==========

// preferenceKey 生成偏好设置key.
func preferenceKey(userID, deviceID, category string) string {
	return fmt.Sprintf("%s:%s:%s", userID, deviceID, category)
}

// SetNotificationPreference 设置通知偏好.
func (m *Manager) SetNotificationPreference(pref *NotificationPreference) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := preferenceKey(pref.UserID, pref.DeviceID, string(pref.Category))
	pref.UpdatedAt = time.Now()
	m.preferences[key] = pref
}

// GetNotificationPreference 获取通知偏好.
func (m *Manager) GetNotificationPreference(userID, deviceID string, category NotificationCategory) *NotificationPreference {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := preferenceKey(userID, deviceID, string(category))
	return m.preferences[key]
}

// ListNotificationPreferences 列出用户所有通知偏好.
func (m *Manager) ListNotificationPreferences(userID, deviceID string) []*NotificationPreference {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var prefs []*NotificationPreference
	for _, pref := range m.preferences {
		if pref.UserID == userID && pref.DeviceID == deviceID {
			prefs = append(prefs, pref)
		}
	}
	return prefs
}

// IsNotificationEnabled 检查通知是否启用.
func (m *Manager) IsNotificationEnabled(userID, deviceID string, category NotificationCategory) bool {
	pref := m.GetNotificationPreference(userID, deviceID, category)
	if pref == nil {
		return true // 默认启用
	}
	return pref.Enabled
}

// ========== 通知历史管理 ==========

// AddNotificationHistory 添加通知历史.
func (m *Manager) AddNotificationHistory(item *NotificationHistoryItem) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if item.ID == "" {
		item.ID = generateID()
	}
	item.CreatedAt = time.Now()

	m.history = append(m.history, item)

	// 限制历史记录数量
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}
}

// GetNotificationHistory 获取通知历史.
func (m *Manager) GetNotificationHistory(userID string, limit, offset int) []*NotificationHistoryItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 过滤用户通知
	var userHistory []*NotificationHistoryItem
	for i := len(m.history) - 1; i >= 0; i-- {
		if m.history[i].UserID == userID {
			userHistory = append(userHistory, m.history[i])
		}
	}

	// 分页
	if offset >= len(userHistory) {
		return nil
	}
	end := offset + limit
	if end > len(userHistory) {
		end = len(userHistory)
	}
	return userHistory[offset:end]
}

// MarkNotificationRead 标记通知为已读.
func (m *Manager) MarkNotificationRead(userID string, ids []string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}

	count := 0
	now := time.Now()
	for _, item := range m.history {
		if item.UserID == userID && idSet[item.ID] && !item.Read {
			item.Read = true
			item.ReadAt = &now
			count++
		}
	}
	return count
}

// MarkAllNotificationsRead 标记所有通知为已读.
func (m *Manager) MarkAllNotificationsRead(userID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	now := time.Now()
	for _, item := range m.history {
		if item.UserID == userID && !item.Read {
			item.Read = true
			item.ReadAt = &now
			count++
		}
	}
	return count
}

// GetUnreadCount 获取未读通知数量.
func (m *Manager) GetUnreadCount(userID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, item := range m.history {
		if item.UserID == userID && !item.Read {
			count++
		}
	}
	return count
}

// ========== 离线同步管理 ==========

// GetSyncDelta 获取增量同步数据.
func (m *Manager) GetSyncDelta(userID, deviceID string, lastSyncTime time.Time) *SyncDelta {
	// TODO: 实现实际的增量同步逻辑
	return &SyncDelta{
		LastSyncTime: lastSyncTime,
		Changes:      []SyncChange{},
		HasMore:      false,
	}
}

// CreateConflict 创建冲突记录.
func (m *Manager) CreateConflict(conflict *ConflictRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conflict.ID == "" {
		conflict.ID = generateID()
	}
	conflict.CreatedAt = time.Now()

	m.conflicts[conflict.ID] = conflict
}

// ResolveConflict 解决冲突.
func (m *Manager) ResolveConflict(conflictID string, resolution ConflictResolution) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conflict, ok := m.conflicts[conflictID]
	if !ok {
		return fmt.Errorf("conflict not found: %s", conflictID)
	}

	conflict.Resolution = &resolution
	conflict.Resolved = true
	now := time.Now()
	conflict.ResolvedAt = &now

	return nil
}

// ListConflicts 列出未解决的冲突.
func (m *Manager) ListConflicts(userID string) []*ConflictRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var conflicts []*ConflictRecord
	for _, c := range m.conflicts {
		if !c.Resolved {
			conflicts = append(conflicts, c)
		}
	}
	return conflicts
}

// ========== 推送通知 ==========

// SendNotification 发送推送通知（检查偏好）.
func (m *Manager) SendNotification(notification *PushNotification, category NotificationCategory) error {
	// 检查通知是否启用
	if !m.IsNotificationEnabled("", notification.DeviceID, category) {
		return nil // 通知已禁用
	}

	// 记录历史
	m.AddNotificationHistory(&NotificationHistoryItem{
		UserID:   "",
		DeviceID: notification.DeviceID,
		Category: category,
		Title:    notification.Title,
		Body:     notification.Body,
		Data:     notification.Data,
	})

	return m.pushService.Send(notification)
}

// Stop 停止所有服务.
func (m *Manager) Stop() {
	m.pushService.Stop()
	m.syncService.Stop()
}
