// Package mobileapi 提供移动端远程管理API服务
package mobileapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	config := &ManagerConfig{
		AuthConfig: &AuthConfig{
			JWTSecret:          "test-secret-key",
			AccessTokenExpiry:  1 * time.Hour,
			RefreshTokenExpiry: 30 * 24 * time.Hour,
			Issuer:             "test",
			MaxSessions:        10,
		},
		PushConfig: DefaultPushConfig(),
		SyncConfig: DefaultSyncConfig(),
		MaxHistory: 100,
	}
	return NewManager(config)
}

func setupTestHandlers(t *testing.T) (*Handlers, *Manager) {
	t.Helper()
	manager := setupTestManager(t)
	handlers := NewHandlers(manager)
	return handlers, manager
}

func setupTestRouter(t *testing.T) (*gin.Engine, *Handlers, *Manager) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handlers, manager := setupTestHandlers(t)
	router := gin.New()
	handlers.RegisterRoutes(router.Group("/api"))
	return router, handlers, manager
}

// ========== 设备注册测试 ==========

func TestRegisterDevice(t *testing.T) {
	manager := setupTestManager(t)

	device := &MobileDevice{
		UserID:       "user-001",
		DeviceName:   "iPhone 15",
		Platform:     PlatformIOS,
		OSVersion:    "17.0",
		AppVersion:   "1.0.0",
		PushToken:    "test-push-token",
		PushProvider: ProviderAPNs,
	}

	token, err := manager.RegisterDevice(device)
	require.NoError(t, err)
	require.NotNil(t, token)
	assert.NotEmpty(t, token.AccessToken)
	assert.NotEmpty(t, token.RefreshToken)
	assert.Equal(t, "Bearer", token.TokenType)
	assert.Equal(t, device.ID, token.DeviceID)
}

func TestRegisterDeviceAndroid(t *testing.T) {
	manager := setupTestManager(t)

	device := &MobileDevice{
		UserID:       "user-001",
		DeviceName:   "Pixel 8",
		Platform:     PlatformAndroid,
		OSVersion:    "14",
		AppVersion:   "1.0.0",
		PushToken:    "fcm-token-123",
		PushProvider: ProviderFCM,
	}

	token, err := manager.RegisterDevice(device)
	require.NoError(t, err)
	assert.NotEmpty(t, token.AccessToken)
}

func TestRegisterDeviceDuplicate(t *testing.T) {
	manager := setupTestManager(t)

	device := &MobileDevice{
		ID:       "device-001",
		UserID:   "user-001",
		Platform: PlatformIOS,
	}

	_, err := manager.RegisterDevice(device)
	require.NoError(t, err)

	// 再次注册应该更新设备
	token, err := manager.RegisterDevice(device)
	require.NoError(t, err)
	assert.NotEmpty(t, token.AccessToken)
}

// ========== 认证测试 ==========

func TestAuthenticate(t *testing.T) {
	manager := setupTestManager(t)

	// 先注册设备
	device := &MobileDevice{
		ID:       "device-001",
		UserID:   "user-001",
		Platform: PlatformIOS,
	}
	_, err := manager.RegisterDevice(device)
	require.NoError(t, err)

	// 认证
	token, err := manager.authService.Authenticate("device-001", "user-001")
	require.NoError(t, err)
	assert.NotEmpty(t, token.AccessToken)
}

func TestAuthenticateBlockedDevice(t *testing.T) {
	manager := setupTestManager(t)

	// 注册设备
	device := &MobileDevice{
		ID:       "device-001",
		UserID:   "user-001",
		Platform: PlatformIOS,
	}
	_, err := manager.RegisterDevice(device)
	require.NoError(t, err)

	// 封禁设备
	err = manager.authService.BlockDevice("device-001")
	require.NoError(t, err)

	// 认证应该失败
	_, err = manager.authService.Authenticate("device-001", "user-001")
	assert.ErrorIs(t, err, ErrDeviceBlocked)
}

func TestValidateToken(t *testing.T) {
	manager := setupTestManager(t)

	// 注册设备获取token
	device := &MobileDevice{
		ID:       "device-001",
		UserID:   "user-001",
		Platform: PlatformIOS,
	}
	token, err := manager.RegisterDevice(device)
	require.NoError(t, err)

	// 验证token
	claims, err := manager.authService.ValidateToken(token.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "user-001", claims.UserID)
	assert.Equal(t, "device-001", claims.DeviceID)
}

func TestValidateTokenExpired(t *testing.T) {
	config := &AuthConfig{
		JWTSecret:          "test-secret",
		AccessTokenExpiry:  -1 * time.Hour, // 已过期
		RefreshTokenExpiry: 30 * 24 * time.Hour,
		Issuer:             "test",
	}
	auth := NewAuthService(config)

	device := &MobileDevice{
		ID:     "device-001",
		UserID: "user-001",
	}
	token, err := auth.RegisterDevice(device)
	require.NoError(t, err)

	_, err = auth.ValidateToken(token.AccessToken)
	assert.ErrorIs(t, err, ErrTokenExpired)
}

func TestRefreshToken(t *testing.T) {
	manager := setupTestManager(t)

	// 注册设备获取token
	device := &MobileDevice{
		ID:       "device-001",
		UserID:   "user-001",
		Platform: PlatformIOS,
	}
	token, err := manager.RegisterDevice(device)
	require.NoError(t, err)

	// 刷新token
	newToken, err := manager.authService.RefreshAccessToken(token.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, newToken.AccessToken)
	assert.NotEmpty(t, newToken.RefreshToken)
	// 刷新后应该有新的refresh token
	assert.NotEqual(t, token.RefreshToken, newToken.RefreshToken)
}

func TestRefreshTokenRevoked(t *testing.T) {
	manager := setupTestManager(t)

	// 注册设备获取token
	device := &MobileDevice{
		ID:       "device-001",
		UserID:   "user-001",
		Platform: PlatformIOS,
	}
	token, err := manager.RegisterDevice(device)
	require.NoError(t, err)

	// 撤销token
	err = manager.authService.RevokeToken(token.RefreshToken)
	require.NoError(t, err)

	// 刷新应该失败
	_, err = manager.authService.RefreshAccessToken(token.RefreshToken)
	assert.ErrorIs(t, err, ErrTokenRevoked)
}

// ========== 通知偏好测试 ==========

func TestNotificationPreference(t *testing.T) {
	manager := setupTestManager(t)

	// 设置偏好
	pref := &NotificationPreference{
		UserID:   "user-001",
		DeviceID: "device-001",
		Category: CategoryBackup,
		Enabled:  true,
		Sound:    true,
		Vibrate:  false,
		Badge:    true,
	}
	manager.SetNotificationPreference(pref)

	// 获取偏好
	got := manager.GetNotificationPreference("user-001", "device-001", CategoryBackup)
	require.NotNil(t, got)
	assert.True(t, got.Enabled)
	assert.True(t, got.Sound)
	assert.False(t, got.Vibrate)
}

func TestNotificationPreferenceDisabled(t *testing.T) {
	manager := setupTestManager(t)

	// 默认应该是启用的
	assert.True(t, manager.IsNotificationEnabled("user-001", "device-001", CategoryBackup))

	// 禁用通知
	pref := &NotificationPreference{
		UserID:   "user-001",
		DeviceID: "device-001",
		Category: CategoryBackup,
		Enabled:  false,
	}
	manager.SetNotificationPreference(pref)

	assert.False(t, manager.IsNotificationEnabled("user-001", "device-001", CategoryBackup))
}

func TestListNotificationPreferences(t *testing.T) {
	manager := setupTestManager(t)

	// 设置多个偏好
	categories := []NotificationCategory{CategoryBackup, CategorySecurity, CategorySystem}
	for _, cat := range categories {
		manager.SetNotificationPreference(&NotificationPreference{
			UserID:   "user-001",
			DeviceID: "device-001",
			Category: cat,
			Enabled:  true,
		})
	}

	prefs := manager.ListNotificationPreferences("user-001", "device-001")
	assert.Len(t, prefs, 3)
}

// ========== 通知历史测试 ==========

func TestNotificationHistory(t *testing.T) {
	manager := setupTestManager(t)

	// 添加通知
	for i := 0; i < 5; i++ {
		manager.AddNotificationHistory(&NotificationHistoryItem{
			UserID:   "user-001",
			DeviceID: "device-001",
			Category: CategorySystem,
			Title:    "Test",
			Body:     "Test notification",
		})
	}

	// 获取历史
	history := manager.GetNotificationHistory("user-001", 10, 0)
	assert.Len(t, history, 5)
}

func TestNotificationHistoryPagination(t *testing.T) {
	manager := setupTestManager(t)

	// 添加通知
	for i := 0; i < 20; i++ {
		manager.AddNotificationHistory(&NotificationHistoryItem{
			UserID: "user-001",
			Title:  "Test",
		})
	}

	// 第一页
	page1 := manager.GetNotificationHistory("user-001", 10, 0)
	assert.Len(t, page1, 10)

	// 第二页
	page2 := manager.GetNotificationHistory("user-001", 10, 10)
	assert.Len(t, page2, 10)
}

func TestMarkNotificationRead(t *testing.T) {
	manager := setupTestManager(t)

	// 添加通知
	ids := make([]string, 5)
	for i := 0; i < 5; i++ {
		item := &NotificationHistoryItem{
			UserID: "user-001",
			Title:  "Test",
		}
		manager.AddNotificationHistory(item)
		ids[i] = item.ID
	}

	// 标记前3个为已读
	count := manager.MarkNotificationRead("user-001", ids[:3])
	assert.Equal(t, 3, count)

	// 未读数量应该是2
	unread := manager.GetUnreadCount("user-001")
	assert.Equal(t, 2, unread)
}

func TestMarkAllNotificationsRead(t *testing.T) {
	manager := setupTestManager(t)

	// 添加通知
	for i := 0; i < 10; i++ {
		manager.AddNotificationHistory(&NotificationHistoryItem{
			UserID: "user-001",
			Title:  "Test",
		})
	}

	assert.Equal(t, 10, manager.GetUnreadCount("user-001"))

	// 标记所有为已读
	count := manager.MarkAllNotificationsRead("user-001")
	assert.Equal(t, 10, count)
	assert.Equal(t, 0, manager.GetUnreadCount("user-001"))
}

func TestNotificationHistoryMaxLimit(t *testing.T) {
	config := &ManagerConfig{
		AuthConfig: DefaultAuthConfig(),
		PushConfig: DefaultPushConfig(),
		SyncConfig: DefaultSyncConfig(),
		MaxHistory: 5, // 最多保留5条
	}
	manager := NewManager(config)

	// 添加10条
	for i := 0; i < 10; i++ {
		manager.AddNotificationHistory(&NotificationHistoryItem{
			UserID: "user-001",
			Title:  "Test",
		})
	}

	// 应该只保留最近5条
	history := manager.GetNotificationHistory("user-001", 100, 0)
	assert.Len(t, history, 5)
}

// ========== 冲突管理测试 ==========

func TestSyncConflictResolution(t *testing.T) {
	manager := setupTestManager(t)

	// 创建冲突
	conflict := &ConflictRecord{
		ItemID:        "item-001",
		ConflictType:  ConflictModify,
		ServerVersion: "v1",
		ClientVersion: "v2",
	}
	manager.CreateConflict(conflict)
	assert.NotEmpty(t, conflict.ID)

	// 列出冲突
	conflicts := manager.ListConflicts("user-001")
	assert.Len(t, conflicts, 1)

	// 解决冲突
	err := manager.ResolveConflict(conflict.ID, ResolutionServer)
	require.NoError(t, err)

	// 冲突应该已解决
	conflicts = manager.ListConflicts("user-001")
	assert.Len(t, conflicts, 0)
}

func TestResolveConflictNotFound(t *testing.T) {
	manager := setupTestManager(t)

	err := manager.ResolveConflict("non-existent", ResolutionServer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "conflict not found")
}

// ========== 设备绑定测试 ==========

func TestDeviceBinding(t *testing.T) {
	manager := setupTestManager(t)

	// 注册设备（自动创建绑定）
	device := &MobileDevice{
		ID:       "device-001",
		UserID:   "user-001",
		Platform: PlatformIOS,
	}
	_, err := manager.RegisterDevice(device)
	require.NoError(t, err)

	// 列出绑定
	bindings := manager.ListBindings("user-001")
	assert.Len(t, bindings, 1)
	assert.Equal(t, "device-001", bindings[0].DeviceID)
	assert.True(t, bindings[0].Active)
}

func TestDeviceUnbinding(t *testing.T) {
	manager := setupTestManager(t)

	// 注册设备
	device := &MobileDevice{
		ID:       "device-001",
		UserID:   "user-001",
		Platform: PlatformIOS,
	}
	_, err := manager.RegisterDevice(device)
	require.NoError(t, err)

	// 移除设备（自动解绑）
	err = manager.RemoveDevice("device-001")
	require.NoError(t, err)

	// 绑定应该已解绑
	bindings := manager.ListBindings("user-001")
	assert.Len(t, bindings, 0)
}

// ========== 同步服务测试 ==========

func TestSyncService(t *testing.T) {
	syncSvc := NewSyncService(nil)

	// 添加同步项
	item := &SyncItem{
		UserID:   "user-001",
		DeviceID: "device-001",
		Type:     SyncPhoto,
		FileName: "test.jpg",
		FileSize: 1024,
	}
	err := syncSvc.AddSyncItem(item)
	require.NoError(t, err)

	// 列出同步项
	items := syncSvc.ListItems("user-001")
	assert.Len(t, items, 1)
	assert.Equal(t, SyncPending, items[0].Status)
}

func TestSyncServiceConfig(t *testing.T) {
	syncSvc := NewSyncService(nil)

	config := syncSvc.GetConfig()
	assert.True(t, config.Enabled)
	assert.True(t, config.AutoSyncPhotos)

	// 更新配置
	newConfig := &SyncConfig{
		Enabled:        false,
		AutoSyncPhotos: false,
		SyncOnWifiOnly: false,
	}
	syncSvc.UpdateConfig(newConfig)

	config = syncSvc.GetConfig()
	assert.False(t, config.Enabled)
}

func TestSyncStats(t *testing.T) {
	syncSvc := NewSyncService(nil)

	stats := syncSvc.GetStats()
	assert.False(t, stats.IsSyncing)
	assert.Equal(t, int64(0), stats.TotalItems)
}

// ========== 推送服务测试 ==========

func TestPushService(t *testing.T) {
	pushSvc := NewPushService(nil, 2)
	defer pushSvc.Stop()

	// 发送通知
	notification := &PushNotification{
		DeviceID: "device-001",
		Title:    "Test",
		Body:     "Test notification",
	}
	err := pushSvc.Send(notification)
	require.NoError(t, err)

	// 等待处理
	time.Sleep(200 * time.Millisecond)

	// 检查历史
	history := pushSvc.GetHistory()
	assert.True(t, len(history) > 0)
}

func TestPushServiceClearHistory(t *testing.T) {
	pushSvc := NewPushService(nil, 1)
	defer pushSvc.Stop()

	// 发送通知
	for i := 0; i < 5; i++ {
		pushSvc.Send(&PushNotification{
			DeviceID: "device-001",
			Title:    "Test",
		})
	}

	time.Sleep(300 * time.Millisecond)

	// 清空历史
	pushSvc.ClearHistory()
	history := pushSvc.GetHistory()
	assert.Len(t, history, 0)
}

// ========== HTTP API 测试 ==========

func TestAPIRegisterDevice(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	body := DeviceRegisterRequest{
		UserID:     "user-001",
		DeviceName: "Test Device",
		Platform:   PlatformIOS,
		OSVersion:  "17.0",
		AppVersion: "1.0.0",
		PushToken:  "test-token",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/mobile-api/auth/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "device registered", resp.Message)
}

func TestAPIRegisterDeviceInvalidRequest(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	// 缺少必填字段
	body := map[string]string{
		"userId": "user-001",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/mobile-api/auth/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAPIAuthMiddleware(t *testing.T) {
	router, _, manager := setupTestRouter(t)

	// 先注册设备
	device := &MobileDevice{
		ID:       "device-001",
		UserID:   "user-001",
		Platform: PlatformIOS,
	}
	_, _ = manager.RegisterDevice(device)

	// 无token访问
	req := httptest.NewRequest(http.MethodGet, "/api/mobile-api/devices", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// 获取token
	token, _ := manager.authService.Authenticate("device-001", "user-001")

	// 使用token访问
	req = httptest.NewRequest(http.MethodGet, "/api/mobile-api/devices", nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPIDeviceCRUD(t *testing.T) {
	router, _, manager := setupTestRouter(t)

	// 注册设备
	device := &MobileDevice{
		ID:       "device-001",
		UserID:   "user-001",
		Platform: PlatformIOS,
	}
	token, _ := manager.RegisterDevice(device)

	// 获取设备
	req := httptest.NewRequest(http.MethodGet, "/api/mobile-api/devices/device-001", nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 更新设备
	updateBody := UpdateDeviceRequest{
		DeviceName: "Updated Device",
		AppVersion: "2.0.0",
	}
	jsonBody, _ := json.Marshal(updateBody)
	req = httptest.NewRequest(http.MethodPut, "/api/mobile-api/devices/device-001", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPISendPush(t *testing.T) {
	router, _, manager := setupTestRouter(t)

	// 注册设备
	device := &MobileDevice{
		ID:       "device-001",
		UserID:   "user-001",
		Platform: PlatformIOS,
	}
	token, _ := manager.RegisterDevice(device)

	// 发送推送
	pushBody := SendPushRequest{
		DeviceID: "device-001",
		Title:    "Test",
		Body:     "Test push",
	}
	jsonBody, _ := json.Marshal(pushBody)
	req := httptest.NewRequest(http.MethodPost, "/api/mobile-api/push/send", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPINotificationPreferences(t *testing.T) {
	router, _, manager := setupTestRouter(t)

	// 注册设备
	device := &MobileDevice{
		ID:       "device-001",
		UserID:   "user-001",
		Platform: PlatformIOS,
	}
	token, _ := manager.RegisterDevice(device)

	// 更新偏好
	prefBody := NotificationPreference{
		Category: CategoryBackup,
		Enabled:  true,
		Sound:    true,
	}
	jsonBody, _ := json.Marshal(prefBody)
	req := httptest.NewRequest(http.MethodPut, "/api/mobile-api/notifications/preferences", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 获取偏好
	req = httptest.NewRequest(http.MethodGet, "/api/mobile-api/notifications/preferences", nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPISyncConfig(t *testing.T) {
	router, _, manager := setupTestRouter(t)

	// 注册设备
	device := &MobileDevice{
		ID:       "device-001",
		UserID:   "user-001",
		Platform: PlatformIOS,
	}
	token, _ := manager.RegisterDevice(device)

	// 更新同步配置
	configBody := SyncConfig{
		Enabled:        true,
		AutoSyncPhotos: false,
		SyncOnWifiOnly: true,
		MaxFileSize:    500 * 1024 * 1024,
	}
	jsonBody, _ := json.Marshal(configBody)
	req := httptest.NewRequest(http.MethodPost, "/api/mobile-api/sync/config", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 获取同步配置
	req = httptest.NewRequest(http.MethodGet, "/api/mobile-api/sync/config", nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPISystemStatus(t *testing.T) {
	router, _, manager := setupTestRouter(t)

	// 注册设备
	device := &MobileDevice{
		ID:       "device-001",
		UserID:   "user-001",
		Platform: PlatformIOS,
	}
	token, _ := manager.RegisterDevice(device)

	req := httptest.NewRequest(http.MethodGet, "/api/mobile-api/control/status", nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ========== Manager生命周期测试 ==========

func TestManagerStop(t *testing.T) {
	manager := setupTestManager(t)

	// 不应该panic
	assert.NotPanics(t, func() {
		manager.Stop()
	})
}

func TestManagerGetServices(t *testing.T) {
	manager := setupTestManager(t)

	assert.NotNil(t, manager.GetAuthService())
	assert.NotNil(t, manager.GetPushService())
	assert.NotNil(t, manager.GetSyncService())
}

// ========== 增量同步测试 ==========

func TestGetSyncDelta(t *testing.T) {
	manager := setupTestManager(t)

	delta := manager.GetSyncDelta("user-001", "device-001", time.Now().Add(-1*time.Hour))
	assert.NotNil(t, delta)
	assert.NotNil(t, delta.Changes)
}

// ========== 边界情况测试 ==========

func TestRemoveDeviceNotFound(t *testing.T) {
	manager := setupTestManager(t)

	err := manager.RemoveDevice("non-existent")
	assert.ErrorIs(t, err, ErrDeviceNotFound)
}

func TestGetNotificationHistoryEmpty(t *testing.T) {
	manager := setupTestManager(t)

	history := manager.GetNotificationHistory("user-001", 10, 0)
	assert.Nil(t, history)
}

func TestGetUnreadCountEmpty(t *testing.T) {
	manager := setupTestManager(t)

	count := manager.GetUnreadCount("user-001")
	assert.Equal(t, 0, count)
}
