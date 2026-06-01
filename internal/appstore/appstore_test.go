// Package appstore 应用商店测试
package appstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAppStore(t *testing.T) {
	store := NewAppStore()
	assert.NotNil(t, store)
}

func TestListApps_All(t *testing.T) {
	store := NewAppStore()
	ctx := context.Background()
	apps := store.ListApps(ctx, "")
	assert.NotEmpty(t, apps)
}

func TestListApps_Filter(t *testing.T) {
	store := NewAppStore()
	ctx := context.Background()
	mediaApps := store.ListApps(ctx, CategoryMedia)
	assert.NotEmpty(t, mediaApps)
	for _, app := range mediaApps {
		assert.Equal(t, CategoryMedia, app.Category)
	}
}

func TestListApps_Empty(t *testing.T) {
	store := NewAppStore()
	ctx := context.Background()
	apps := store.ListApps(ctx, "nonexistent")
	assert.Empty(t, apps)
}

func TestGetApp(t *testing.T) {
	store := NewAppStore()
	ctx := context.Background()
	app, err := store.GetApp(ctx, "plex")
	require.NoError(t, err)
	assert.Equal(t, "plex", app.ID)
	assert.Equal(t, "Plex Media Server", app.Name)
}

func TestGetApp_NotFound(t *testing.T) {
	store := NewAppStore()
	ctx := context.Background()
	_, err := store.GetApp(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestInstallApp(t *testing.T) {
	store := NewAppStore()
	ctx := context.Background()

	err := store.InstallApp(ctx, "plex")
	require.NoError(t, err)

	app, _ := store.GetApp(ctx, "plex")
	assert.True(t, app.Installed)
	assert.Equal(t, AppStatusInstalled, app.Status)
	assert.NotNil(t, app.InstalledAt)
}

func TestInstallApp_NotFound(t *testing.T) {
	store := NewAppStore()
	ctx := context.Background()

	err := store.InstallApp(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestUninstallApp(t *testing.T) {
	store := NewAppStore()
	ctx := context.Background()

	store.InstallApp(ctx, "plex")
	err := store.UninstallApp(ctx, "plex")
	require.NoError(t, err)

	app, _ := store.GetApp(ctx, "plex")
	assert.False(t, app.Installed)
	assert.Equal(t, AppStatusAvailable, app.Status)
	assert.Nil(t, app.InstalledAt)
}

func TestUninstallApp_NotFound(t *testing.T) {
	store := NewAppStore()
	ctx := context.Background()

	err := store.UninstallApp(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestUpdateApp(t *testing.T) {
	store := NewAppStore()
	ctx := context.Background()

	err := store.UpdateApp(ctx, "plex", "2.0.0")
	require.NoError(t, err)

	app, _ := store.GetApp(ctx, "plex")
	assert.Equal(t, "2.0.0", app.Version)
}

func TestSearchApps(t *testing.T) {
	store := NewAppStore()
	ctx := context.Background()

	apps := store.SearchApps(ctx, "Plex")
	assert.NotEmpty(t, apps)
	assert.Equal(t, "plex", apps[0].ID)
}

func TestSearchApps_Empty(t *testing.T) {
	store := NewAppStore()
	ctx := context.Background()

	apps := store.SearchApps(ctx, "nonexistent")
	assert.Empty(t, apps)
}
