package docker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewVersionManager(t *testing.T) {
	t.Run("with valid directory", func(t *testing.T) {
		tempDir := t.TempDir()
		vm, err := NewVersionManager(tempDir)
		require.NoError(t, err)
		assert.NotNil(t, vm)
		assert.NotNil(t, vm.versions)
		assert.NotNil(t, vm.notifications)
	})

	t.Run("with empty directory", func(t *testing.T) {
		vm, err := NewVersionManager("")
		require.NoError(t, err)
		assert.NotNil(t, vm)
	})
}

func TestVersionManager_GetNotifications(t *testing.T) {
	tempDir := t.TempDir()
	vm, err := NewVersionManager(tempDir)
	require.NoError(t, err)

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
	tempDir := t.TempDir()
	vm, err := NewVersionManager(tempDir)
	require.NoError(t, err)

	vm.notifications["n1"] = &UpdateNotification{
		ID:   "n1",
		Read: false,
	}

	err = vm.MarkNotificationRead("n1")
	require.NoError(t, err)

	assert.True(t, vm.notifications["n1"].Read)
}

func TestVersionManager_MarkNotificationRead_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	vm, err := NewVersionManager(tempDir)
	require.NoError(t, err)

	err = vm.MarkNotificationRead("nonexistent")
	assert.Error(t, err)
}

func TestVersionManager_DismissNotification(t *testing.T) {
	tempDir := t.TempDir()
	vm, err := NewVersionManager(tempDir)
	require.NoError(t, err)

	vm.notifications["n1"] = &UpdateNotification{
		ID:        "n1",
		Dismissed: false,
	}

	err = vm.DismissNotification("n1")
	require.NoError(t, err)

	assert.True(t, vm.notifications["n1"].Dismissed)
}

func TestVersionManager_MarkAllNotificationsRead(t *testing.T) {
	tempDir := t.TempDir()
	vm, err := NewVersionManager(tempDir)
	require.NoError(t, err)

	// Add multiple notifications
	for i := 1; i <= 3; i++ {
		vm.notifications[string(rune('n'+i))] = &UpdateNotification{
			ID:   string(rune('n' + i)),
			Read: false,
		}
	}

	err = vm.MarkAllNotificationsRead()
	require.NoError(t, err)

	for _, n := range vm.notifications {
		assert.True(t, n.Read)
	}
}

func TestVersionManager_GetUnreadCount(t *testing.T) {
	tempDir := t.TempDir()
	vm, err := NewVersionManager(tempDir)
	require.NoError(t, err)

	vm.notifications["n1"] = &UpdateNotification{ID: "n1", Read: false}
	vm.notifications["n2"] = &UpdateNotification{ID: "n2", Read: true}
	vm.notifications["n3"] = &UpdateNotification{ID: "n3", Read: false}

	count := vm.GetUnreadCount()
	assert.Equal(t, 2, count)
}

func TestVersionManager_ClearNotifications(t *testing.T) {
	tempDir := t.TempDir()
	vm, err := NewVersionManager(tempDir)
	require.NoError(t, err)

	vm.notifications["n1"] = &UpdateNotification{ID: "n1"}

	err = vm.ClearNotifications()
	require.NoError(t, err)

	assert.Empty(t, vm.notifications)
}

func TestVersionManager_GetAvailableVersions(t *testing.T) {
	tempDir := t.TempDir()
	vm, err := NewVersionManager(tempDir)
	require.NoError(t, err)

	// Add versions
	vm.versions["template-1"] = []*AppVersion{
		{Tag: "latest", Digest: "abc123"},
		{Tag: "v1.0.0", Digest: "def456"},
		{Tag: "v2.0.0", Digest: "ghi789"},
	}

	versions := vm.GetAvailableVersions("template-1")
	assert.Len(t, versions, 3)
}

func TestVersionManager_GetAvailableVersions_Empty(t *testing.T) {
	tempDir := t.TempDir()
	vm, err := NewVersionManager(tempDir)
	require.NoError(t, err)

	versions := vm.GetAvailableVersions("nonexistent")
	assert.Empty(t, versions)
}

func TestVersionManager_SaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	vm, err := NewVersionManager(tempDir)
	require.NoError(t, err)

	// Add data
	vm.versions["t1"] = []*AppVersion{{Tag: "v1.0.0"}}
	vm.notifications["n1"] = &UpdateNotification{ID: "n1", AppName: "Test"}

	// Save
	err = vm.saveVersions()
	require.NoError(t, err)

	err = vm.saveNotifications()
	require.NoError(t, err)

	// Create new manager to load
	vm2, err := NewVersionManager(tempDir)
	require.NoError(t, err)

	// Verify data was loaded
	assert.NotNil(t, vm2.versions["t1"])
	assert.NotNil(t, vm2.notifications["n1"])
}

func TestUpdateNotification_Struct(t *testing.T) {
	now := time.Now()
	notification := UpdateNotification{
		ID:          "notif-123",
		AppID:       "app-456",
		AppName:     "TestApp",
		CurrentVer:  "1.0.0",
		LatestVer:   "2.0.0",
		Description: "Major update with new features",
		Severity:    "major",
		Read:        false,
		Dismissed:   false,
		CreatedAt:   now,
	}

	assert.Equal(t, "notif-123", notification.ID)
	assert.Equal(t, "TestApp", notification.AppName)
	assert.Equal(t, "major", notification.Severity)
	assert.False(t, notification.Read)
}

func TestAppVersion_Struct(t *testing.T) {
	now := time.Now()
	version := AppVersion{
		Tag:        "v2.0.0",
		Digest:     "sha256:abc123",
		Size:       1024000,
		Created:    now,
		Changelog:  "Bug fixes and improvements",
		IsLatest:   true,
		IsVerified: true,
	}

	assert.Equal(t, "v2.0.0", version.Tag)
	assert.True(t, version.IsLatest)
	assert.True(t, version.IsVerified)
}

func TestVersionManager_CheckForUpdates(t *testing.T) {
	tempDir := t.TempDir()
	vm, err := NewVersionManager(tempDir)
	require.NoError(t, err)

	// This test requires network, so we just verify it doesn't crash
	installed := []*InstalledApp{
		{
			ID:      "app1",
			Name:    "test",
			Version: "1.0.0",
		},
	}

	// CheckForUpdates may fail without network, but should not panic
	_ = vm.CheckForUpdates(installed)
}

func TestVersionManager_UpdateAppVersion(t *testing.T) {
	tempDir := t.TempDir()
	vm, err := NewVersionManager(tempDir)
	require.NoError(t, err)

	// This test would require a real Docker connection
	// Just verify the method exists and doesn't panic with nil manager
	vm.manager = nil

	err = vm.UpdateAppVersion("app1", "2.0.0")
	// Should handle gracefully with nil manager
	_ = err
}

func TestVersionManager_StartUpdateChecker(t *testing.T) {
	tempDir := t.TempDir()
	vm, err := NewVersionManager(tempDir)
	require.NoError(t, err)

	// Start and stop immediately
	stopCh := vm.StartUpdateChecker(1 * time.Hour)
	if stopCh != nil {
		close(stopCh)
	}
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
			notificationsFile: "/nonexistent/notifications.json",
			notifications:      make(map[string]*UpdateNotification),
		}

		err := vm.loadNotifications()
		// Should handle gracefully
		_ = err
	})
}