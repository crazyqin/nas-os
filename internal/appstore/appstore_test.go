// Package appstore 应用商店测试
package appstore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *AppStore {
	t.Helper()
	store := NewAppStore(AppStoreConfig{UpdateInterval: time.Hour})
	err := store.Start()
	require.NoError(t, err)
	t.Cleanup(func() { store.Stop() })
	return store
}

func TestNewAppStore(t *testing.T) {
	store := NewAppStore(AppStoreConfig{})
	assert.NotNil(t, store)
}

func TestListApps_All(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	apps := store.ListApps(ctx, nil)
	assert.NotEmpty(t, apps)
}

func TestListApps_Filter(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	mediaCat := CategoryMedia
	mediaApps := store.ListApps(ctx, &mediaCat)
	assert.NotEmpty(t, mediaApps)
	for _, app := range mediaApps {
		assert.Equal(t, CategoryMedia, app.Category)
	}
}

func TestListApps_Empty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	apps := store.ListApps(ctx, nil)
	assert.NotEmpty(t, apps) // Should return all apps
}

func TestGetApp(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	app, err := store.GetApp(ctx, "jellyfin")
	require.NoError(t, err)
	assert.Equal(t, "jellyfin", app.ID)
	assert.Equal(t, "Jellyfin", app.Title)
}

func TestGetApp_NotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	_, err := store.GetApp(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestInstallApp(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.InstallApp(ctx, "jellyfin")
	require.NoError(t, err)

	// Wait for async installation to complete (simulated ~4s)
	time.Sleep(5 * time.Second)

	installed := store.GetInstalledApps()
	assert.NotEmpty(t, installed)
}

func TestInstallApp_NotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.InstallApp(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestUninstallApp(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	store.InstallApp(ctx, "jellyfin")
	time.Sleep(5 * time.Second)

	installed := store.GetInstalledApps()
	require.NotEmpty(t, installed)

	err := store.UninstallApp(ctx, installed[0].ID)
	require.NoError(t, err)

	// After uninstall, app should still exist in catalog
	_, err = store.GetApp(ctx, "jellyfin")
	assert.NoError(t, err)

	installed = store.GetInstalledApps()
	assert.Empty(t, installed)
}

func TestUninstallApp_NotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.UninstallApp(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestStartStopApp(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.InstallApp(ctx, "jellyfin")
	require.NoError(t, err)
	time.Sleep(5 * time.Second)

	installed := store.GetInstalledApps()
	require.NotEmpty(t, installed)

	err = store.StopApp(installed[0].ID)
	assert.NoError(t, err)

	err = store.StartApp(installed[0].ID)
	assert.NoError(t, err)
}

func TestSearchApps(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	apps := store.SearchApps(ctx, "Jellyfin")
	assert.NotEmpty(t, apps)
	assert.Equal(t, "jellyfin", apps[0].ID)
}

func TestSearchApps_Empty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	apps := store.SearchApps(ctx, "zzz_nonexistent_app_zzz")
	assert.Empty(t, apps)
}

// ========== Health Check Tests ==========

func TestBuiltinAppHealthCheck(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	app, err := store.GetApp(ctx, "jellyfin")
	require.NoError(t, err)
	require.NotNil(t, app.HealthCheck)

	assert.Equal(t, "http", app.HealthCheck.Type)
	assert.Equal(t, "/health", app.HealthCheck.URL)
	assert.Equal(t, 8096, app.HealthCheck.Port)
	assert.Equal(t, 30*time.Second, app.HealthCheck.Interval)
	assert.Equal(t, 3, app.HealthCheck.Retries)
}

func TestBuiltinAppNextcloudHealthCheck(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	app, err := store.GetApp(ctx, "nextcloud")
	require.NoError(t, err)
	require.NotNil(t, app.HealthCheck)

	assert.Equal(t, "http", app.HealthCheck.Type)
	assert.Equal(t, "/status.php", app.HealthCheck.URL)
}

func TestRunHealthCheck_HTTP(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Install jellyfin (has HTTP health check)
	err := store.InstallApp(ctx, "jellyfin")
	require.NoError(t, err)
	time.Sleep(5 * time.Second)

	installed := store.GetInstalledApps()
	require.NotEmpty(t, installed)

	result, err := store.RunHealthCheck(installed[0].ID)
	require.NoError(t, err)
	assert.Equal(t, HealthHealthy, result.Status)
	assert.Contains(t, result.Message, "HTTP check")
	assert.Equal(t, installed[0].ID, result.InstalledID)
}

func TestRunHealthCheck_NoConfig(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Install portainer (no health check configured)
	err := store.InstallApp(ctx, "portainer")
	require.NoError(t, err)
	time.Sleep(5 * time.Second)

	installed := store.GetInstalledApps()
	require.NotEmpty(t, installed)

	result, err := store.RunHealthCheck(installed[0].ID)
	require.NoError(t, err)
	assert.Equal(t, HealthUnknown, result.Status)
	assert.Contains(t, result.Message, "no health check configured")
}

func TestRunHealthCheck_NotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.RunHealthCheck("nonexistent")
	assert.Error(t, err)
}

func TestRunHealthCheck_UpdateHealthStatus(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.InstallApp(ctx, "jellyfin")
	require.NoError(t, err)
	time.Sleep(5 * time.Second)

	installed := store.GetInstalledApps()
	require.NotEmpty(t, installed)

	// Before health check, status may be empty
	store.RunHealthCheck(installed[0].ID)

	// After health check, status should be set
	updated := store.GetInstalledApps()
	require.NotEmpty(t, updated)
	assert.Equal(t, HealthHealthy, updated[0].HealthStatus)
	assert.NotNil(t, updated[0].HealthSince)
}

// ========== Compose Template Tests ==========

func TestBuiltinAppComposeTemplate(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	app, err := store.GetApp(ctx, "jellyfin")
	require.NoError(t, err)
	require.NotNil(t, app.ComposeTemplate)

	assert.Equal(t, "3.8", app.ComposeTemplate.Version)
	require.Len(t, app.ComposeTemplate.Services, 1)

	svc := app.ComposeTemplate.Services[0]
	assert.Equal(t, "jellyfin", svc.Name)
	assert.Equal(t, "jellyfin/jellyfin:10.9.0", svc.Image)
	assert.Equal(t, "unless-stopped", svc.RestartPolicy)
	require.Len(t, svc.Ports, 1)
	assert.Equal(t, 8096, svc.Ports[0].Container)
	assert.Len(t, svc.Volumes, 2)
}

func TestGetComposeTemplate(t *testing.T) {
	store := newTestStore(t)

	tmpl, err := store.GetComposeTemplate("jellyfin")
	require.NoError(t, err)
	require.NotNil(t, tmpl)
	assert.NotEmpty(t, tmpl.Services)
}

func TestGetComposeTemplate_NotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.GetComposeTemplate("nonexistent")
	assert.Error(t, err)
}

func TestNextcloudComposeTemplateMultiService(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	app, err := store.GetApp(ctx, "nextcloud")
	require.NoError(t, err)
	require.NotNil(t, app.ComposeTemplate)

	// Nextcloud should have 3 services: nextcloud, postgres, redis
	require.Len(t, app.ComposeTemplate.Services, 3)

	serviceNames := make(map[string]bool)
	for _, svc := range app.ComposeTemplate.Services {
		serviceNames[svc.Name] = true
	}
	assert.True(t, serviceNames["nextcloud"])
	assert.True(t, serviceNames["postgres"])
	assert.True(t, serviceNames["redis"])

	// Nextcloud service should depend on postgres and redis
	nextcloudSvc := app.ComposeTemplate.Services[0]
	assert.Equal(t, "nextcloud", nextcloudSvc.Name)
	assert.Contains(t, nextcloudSvc.DependsOn, "postgres")
	assert.Contains(t, nextcloudSvc.DependsOn, "redis")
}

// ========== Dependency Tests ==========

func TestBuiltinAppDependencies(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	app, err := store.GetApp(ctx, "nextcloud")
	require.NoError(t, err)
	require.NotEmpty(t, app.Dependencies)

	assert.Len(t, app.Dependencies, 2)

	deps := make(map[string]AppDependency)
	for _, dep := range app.Dependencies {
		deps[dep.AppID] = dep
	}

	postgresDep, ok := deps["postgres"]
	require.True(t, ok)
	assert.True(t, postgresDep.Required)
	assert.Equal(t, "Database backend", postgresDep.Reason)

	redisDep, ok := deps["redis"]
	require.True(t, ok)
	assert.False(t, redisDep.Required)
	assert.Equal(t, "Caching layer for performance", redisDep.Reason)
}

func TestGetAppDependencies(t *testing.T) {
	store := newTestStore(t)

	deps, err := store.GetAppDependencies("nextcloud")
	require.NoError(t, err)
	assert.Len(t, deps, 2)
}

func TestGetAppDependencies_NoDeps(t *testing.T) {
	store := newTestStore(t)

	deps, err := store.GetAppDependencies("jellyfin")
	require.NoError(t, err)
	assert.Empty(t, deps)
}

func TestGetAppDependencies_NotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.GetAppDependencies("nonexistent")
	assert.Error(t, err)
}
