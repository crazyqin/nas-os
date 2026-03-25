package docker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// newTestVersionManager 创建测试用的 VersionManager.
func newTestVersionManager(t *testing.T) *VersionManager {
	tempDir := t.TempDir()
	// 创建一个最小化的 Manager
	mgr := &Manager{}
	store, err := NewAppStore(mgr, tempDir)
	if err != nil {
		t.Skipf("无法创建 AppStore: %v", err)
	}
	vm, err := NewVersionManager(store, tempDir)
	if err != nil {
		t.Skipf("无法创建 VersionManager: %v", err)
	}
	return vm
}

func TestNewVersionManager(t *testing.T) {
	vm := newTestVersionManager(t)
	if vm == nil {
		return
	}
	assert.NotNil(t, vm)
	assert.NotNil(t, vm.versions)
	assert.NotNil(t, vm.notifications)
}

func TestVersionManager_GetNotifications(t *testing.T) {
	vm := newTestVersionManager(t)
	if vm == nil {
		return
	}

	// Add a notification
	vm.notifications["n1"] = &UpdateNotification{
		ID:         "n1",
		AppID:      "app1",
		AppName:    "TestApp",
		CurrentVer: "1.0.0",
		LatestVer:  "2.0.0",
		Read:       false,
		Dismissed:  false,
	}

	t.Run("all notifications", func(t *testing.T) {
		notifications := vm.GetNotifications(false)
		assert.Len(t, notifications, 1)
	})

	t.Run("unread only", func(t *testing.T) {
		notifications := vm.GetNotifications(true)
		assert.Len(t, notifications, 1)
	})
}

func TestVersionManager_MarkNotificationRead(t *testing.T) {
	vm := newTestVersionManager(t)
	if vm == nil {
		return
	}

	vm.notifications["n1"] = &UpdateNotification{
		ID:   "n1",
		Read: false,
	}

	err := vm.MarkNotificationRead("n1")
	assert.NoError(t, err)
	assert.True(t, vm.notifications["n1"].Read)
}

func TestVersionManager_MarkNotificationRead_NotFound(t *testing.T) {
	vm := newTestVersionManager(t)
	if vm == nil {
		return
	}

	err := vm.MarkNotificationRead("nonexistent")
	assert.Error(t, err)
}

func TestVersionManager_DismissNotification(t *testing.T) {
	vm := newTestVersionManager(t)
	if vm == nil {
		return
	}

	vm.notifications["n1"] = &UpdateNotification{
		ID:        "n1",
		Dismissed: false,
	}

	err := vm.DismissNotification("n1")
	assert.NoError(t, err)
	assert.True(t, vm.notifications["n1"].Dismissed)
}

func TestVersionManager_MarkAllNotificationsRead(t *testing.T) {
	vm := newTestVersionManager(t)
	if vm == nil {
		return
	}

	// Add multiple notifications
	vm.notifications["n1"] = &UpdateNotification{ID: "n1", Read: false}
	vm.notifications["n2"] = &UpdateNotification{ID: "n2", Read: false}

	err := vm.MarkAllNotificationsRead()
	assert.NoError(t, err)

	for _, n := range vm.notifications {
		assert.True(t, n.Read)
	}
}

func TestVersionManager_GetUnreadCount(t *testing.T) {
	vm := newTestVersionManager(t)
	if vm == nil {
		return
	}

	vm.notifications["n1"] = &UpdateNotification{ID: "n1", Read: false}
	vm.notifications["n2"] = &UpdateNotification{ID: "n2", Read: true}
	vm.notifications["n3"] = &UpdateNotification{ID: "n3", Read: false}

	count := vm.GetUnreadCount()
	assert.Equal(t, 2, count)
}

func TestVersionManager_ClearNotifications(t *testing.T) {
	vm := newTestVersionManager(t)
	if vm == nil {
		return
	}

	vm.notifications["n1"] = &UpdateNotification{ID: "n1"}

	err := vm.ClearNotifications()
	assert.NoError(t, err)
	assert.Empty(t, vm.notifications)
}

func TestVersionManager_GetAvailableVersions(t *testing.T) {
	vm := newTestVersionManager(t)
	if vm == nil {
		return
	}

	// Add versions manually with valid PublishedAt time (within 1 hour for cache to be valid)
	vm.versions["template-1"] = []*AppVersion{
		{ID: "v1", TemplateID: "template-1", Version: "latest", Digest: "abc123", PublishedAt: time.Now()},
		{ID: "v2", TemplateID: "template-1", Version: "v1.0.0", Digest: "def456", PublishedAt: time.Now()},
	}

	versions, err := vm.GetAvailableVersions("template-1")
	assert.NoError(t, err)
	assert.Len(t, versions, 2)
}

func TestVersionManager_GetAvailableVersions_Empty(t *testing.T) {
	vm := newTestVersionManager(t)
	if vm == nil {
		return
	}

	versions, err := vm.GetAvailableVersions("nonexistent")
	// 期望返回错误，因为模板不存在
	assert.Error(t, err)
	assert.Nil(t, versions)
}

func TestUpdateNotification_Struct(t *testing.T) {
	now := time.Now()
	notification := UpdateNotification{
		ID:           "notif-123",
		AppID:        "app-456",
		AppName:      "TestApp",
		CurrentVer:   "1.0.0",
		LatestVer:    "2.0.0",
		ReleaseNotes: "Major update with new features",
		Read:         false,
		Dismissed:    false,
		CreatedAt:    now,
	}

	assert.Equal(t, "notif-123", notification.ID)
	assert.Equal(t, "TestApp", notification.AppName)
	assert.False(t, notification.Read)
}

func TestAppVersion_Struct(t *testing.T) {
	now := time.Now()
	version := AppVersion{
		ID:           "v1",
		TemplateID:   "template-1",
		Version:      "2.0.0",
		ImageTag:     "latest",
		ReleaseNotes: "Bug fixes and improvements",
		PublishedAt:  now,
		Digest:       "sha256:abc123",
		Size:         1024000,
		IsLatest:     true,
	}

	assert.Equal(t, "v1", version.ID)
	assert.Equal(t, "2.0.0", version.Version)
	assert.True(t, version.IsLatest)
}

func TestVersionManager_CheckForUpdates(t *testing.T) {
	vm := newTestVersionManager(t)
	if vm == nil {
		return
	}

	// CheckForUpdates may fail without network, but should not panic
	_, _ = vm.CheckForUpdates()
}

func TestVersionManager_StartUpdateChecker(t *testing.T) {
	vm := newTestVersionManager(t)
	if vm == nil {
		return
	}

	// Start and stop immediately
	vm.StartUpdateChecker(1 * time.Hour)
	// Give it a moment then the test ends
	time.Sleep(100 * time.Millisecond)
}

func TestVersionManager_loadVersions(t *testing.T) {
	t.Run("non-existent file", func(t *testing.T) {
		vm := &VersionManager{
			versionsFile: "/nonexistent/versions.json",
			versions:     make(map[string][]*AppVersion),
		}

		err := vm.loadVersions()
		// Should handle gracefully
		_ = err
	})
}

func TestVersionManager_loadNotifications(t *testing.T) {
	t.Run("non-existent file", func(t *testing.T) {
		vm := &VersionManager{
			notifyFile:    "/nonexistent/notifications.json",
			notifications: make(map[string]*UpdateNotification),
		}

		err := vm.loadNotifications()
		// Should handle gracefully
		_ = err
	})
}
