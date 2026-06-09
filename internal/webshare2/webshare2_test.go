package webshare2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWebShareService(t *testing.T) {
	service := NewWebShareService(WebShareConfig{})
	assert.NotNil(t, service)
}

func TestCreateShareLink(t *testing.T) {
	service := NewWebShareService(WebShareConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	req := CreateShareRequest{
		Name:       "test-file.txt",
		Path:       "/data/test-file.txt",
		Permission: PermissionView,
		CreatedBy:  "user1",
		Size:       1024,
		MimeType:   "text/plain",
	}

	link, err := service.CreateShareLink(req)
	require.NoError(t, err)
	assert.NotEmpty(t, link.ID)
	assert.NotEmpty(t, link.Token)
	assert.Equal(t, "test-file.txt", link.Name)
	assert.Equal(t, PermissionView, link.Permission)
	assert.True(t, link.IsActive)
}

func TestGetShareLink(t *testing.T) {
	service := NewWebShareService(WebShareConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	req := CreateShareRequest{
		Name:       "test-file.txt",
		Path:       "/data/test-file.txt",
		Permission: PermissionView,
		CreatedBy:  "user1",
		Size:       1024,
		MimeType:   "text/plain",
	}

	link, err := service.CreateShareLink(req)
	require.NoError(t, err)

	// Retrieve by token
	retrieved, err := service.GetShareLink(link.Token)
	require.NoError(t, err)
	assert.Equal(t, link.ID, retrieved.ID)
}

func TestGetShareLink_NotFound(t *testing.T) {
	service := NewWebShareService(WebShareConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	_, err = service.GetShareLink("nonexistent")
	assert.Error(t, err)
}

func TestDeleteShareLink(t *testing.T) {
	service := NewWebShareService(WebShareConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	req := CreateShareRequest{
		Name:       "test-file.txt",
		Path:       "/data/test-file.txt",
		Permission: PermissionView,
		CreatedBy:  "user1",
		Size:       1024,
		MimeType:   "text/plain",
	}

	link, err := service.CreateShareLink(req)
	require.NoError(t, err)

	err = service.DeleteShareLink(link.ID)
	require.NoError(t, err)

	// Should not be retrievable after deletion
	_, err = service.GetShareLink(link.Token)
	assert.Error(t, err)
}

func TestListShareLinks(t *testing.T) {
	service := NewWebShareService(WebShareConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	// Create multiple links
	for i := 0; i < 3; i++ {
		req := CreateShareRequest{
			Name:       "test-file.txt",
			Path:       "/data/test-file.txt",
			Permission: PermissionView,
			CreatedBy:  "user1",
			Size:       1024,
			MimeType:   "text/plain",
		}
		_, err := service.CreateShareLink(req)
		require.NoError(t, err)
	}

	links := service.ListShareLinks("user1")
	assert.Len(t, links, 3)
}

func TestRecordAccess(t *testing.T) {
	service := NewWebShareService(WebShareConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	req := CreateShareRequest{
		Name:       "test-file.txt",
		Path:       "/data/test-file.txt",
		Permission: PermissionDownload,
		CreatedBy:  "user1",
		Size:       1024,
		MimeType:   "text/plain",
	}

	link, err := service.CreateShareLink(req)
	require.NoError(t, err)

	service.RecordAccess(link.ID, "download", "192.168.1.1", "Mozilla/5.0")

	logs := service.GetAccessLogs(link.ID)
	assert.Len(t, logs, 1)
	assert.Equal(t, "download", logs[0].Action)
}

func TestGetStats(t *testing.T) {
	service := NewWebShareService(WebShareConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	req := CreateShareRequest{
		Name:       "test-file.txt",
		Path:       "/data/test-file.txt",
		Permission: PermissionView,
		CreatedBy:  "user1",
		Size:       1024,
		MimeType:   "text/plain",
	}

	_, err = service.CreateShareLink(req)
	require.NoError(t, err)

	stats := service.GetStats()
	assert.Equal(t, 1, stats.TotalLinks)
	assert.Equal(t, 1, stats.ActiveLinks)
}

func TestMaxActiveLinks(t *testing.T) {
	service := NewWebShareService(WebShareConfig{MaxActiveLinks: 2})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	for i := 0; i < 2; i++ {
		req := CreateShareRequest{
			Name:       "test-file.txt",
			Path:       "/data/test-file.txt",
			Permission: PermissionView,
			CreatedBy:  "user1",
			Size:       1024,
			MimeType:   "text/plain",
		}
		_, err := service.CreateShareLink(req)
		require.NoError(t, err)
	}

	// Third should fail
	req := CreateShareRequest{
		Name:       "test-file.txt",
		Path:       "/data/test-file.txt",
		Permission: PermissionView,
		CreatedBy:  "user1",
		Size:       1024,
		MimeType:   "text/plain",
	}
	_, err = service.CreateShareLink(req)
	assert.Error(t, err)
}

func TestExpiredLink(t *testing.T) {
	service := NewWebShareService(WebShareConfig{DefaultExpiry: 0})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	expiry := time.Now().Add(-1 * time.Hour) // Already expired
	req := CreateShareRequest{
		Name:       "test-file.txt",
		Path:       "/data/test-file.txt",
		Permission: PermissionView,
		CreatedBy:  "user1",
		Size:       1024,
		MimeType:   "text/plain",
	}

	link, err := service.CreateShareLink(req)
	require.NoError(t, err)

	// Manually set expiry to past
	service.mu.Lock()
	link.ExpiresAt = &expiry
	service.mu.Unlock()

	_, err = service.GetShareLink(link.Token)
	assert.Error(t, err)
}
