package docker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAppStore(t *testing.T) {
	t.Run("with valid directory", func(t *testing.T) {
		tempDir := t.TempDir()
		store, err := NewAppStore(nil, tempDir)
		require.NoError(t, err)
		assert.NotNil(t, store)
		assert.NotNil(t, store.templates)
		assert.NotNil(t, store.installed)
	})

	t.Run("with nil manager", func(t *testing.T) {
		tempDir := t.TempDir()
		store, err := NewAppStore(nil, tempDir)
		require.NoError(t, err)
		assert.Nil(t, store.manager)
	})
}

func TestAppStore_ListTemplates(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewAppStore(nil, tempDir)
	require.NoError(t, err)

	templates := store.ListTemplates()
	// Should have built-in templates
	assert.NotEmpty(t, templates)
}

func TestAppStore_GetTemplate(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewAppStore(nil, tempDir)
	require.NoError(t, err)

	t.Run("existing template", func(t *testing.T) {
		// Get one of the built-in templates
		templates := store.ListTemplates()
		if len(templates) > 0 {
			template := store.GetTemplate(templates[0].ID)
			assert.NotNil(t, template)
		}
	})

	t.Run("non-existent template", func(t *testing.T) {
		template := store.GetTemplate("nonexistent")
		assert.Nil(t, template)
	})
}

func TestAppStore_ListInstalled(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewAppStore(nil, tempDir)
	require.NoError(t, err)

	installed := store.ListInstalled()
	// Should be empty initially
	assert.Empty(t, installed)
}

func TestAppStore_GetInstalled(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewAppStore(nil, tempDir)
	require.NoError(t, err)

	t.Run("non-existent", func(t *testing.T) {
		app, err := store.GetInstalled("nonexistent")
		assert.Error(t, err)
		assert.Nil(t, app)
	})
}

func TestAppStore_SaveAndLoadInstalled(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewAppStore(nil, tempDir)
	require.NoError(t, err)

	// Manually add an installed app
	store.installed["app-1"] = &InstalledApp{
		ID:          "app-1",
		Name:        "test-app",
		DisplayName: "Test App",
		TemplateID:  "template-1",
		Version:     "1.0.0",
		Status:      "running",
		InstallTime: time.Now(),
	}

	// Save
	err = store.saveInstalled()
	require.NoError(t, err)

	// Create new store to load
	store2, err := NewAppStore(nil, tempDir)
	require.NoError(t, err)

	// Verify loaded
	app, err := store2.GetInstalled("app-1")
	require.NoError(t, err)
	assert.Equal(t, "test-app", app.Name)
}

func TestAppTemplate_Struct(t *testing.T) {
	template := AppTemplate{
		ID:          "nextcloud",
		Name:        "nextcloud",
		DisplayName: "Nextcloud",
		Description: "Private cloud storage",
		Category:    "Productivity",
		Icon:        "☁️",
		Version:     "latest",
		Image:       "nextcloud:latest",
		Ports: []PortConfig{
			{Port: 80, Protocol: "tcp", Description: "Web UI", Default: 8080},
		},
		Volumes: []VolumeConfig{
			{ContainerPath: "/var/www/html", Description: "Data", Default: "/data/nextcloud"},
		},
		Environment: map[string]string{
			"MYSQL_HOST": "db",
		},
		Compose: "version: '3'\nservices:\n  nextcloud:\n    image: nextcloud:latest",
		Notes:   "Initial setup required",
		Website: "https://nextcloud.com",
		Source:  "https://github.com/nextcloud",
	}

	assert.Equal(t, "nextcloud", template.ID)
	assert.Equal(t, "Productivity", template.Category)
	assert.Len(t, template.Ports, 1)
	assert.Len(t, template.Volumes, 1)
}

func TestPortConfig_Struct(t *testing.T) {
	port := PortConfig{
		Port:        8080,
		Protocol:    "tcp",
		Description: "HTTP port",
		Default:     80,
	}

	assert.Equal(t, 8080, port.Port)
	assert.Equal(t, "tcp", port.Protocol)
}

func TestVolumeConfig_Struct(t *testing.T) {
	vol := VolumeConfig{
		ContainerPath: "/data",
		Description:   "Application data",
		Default:       "/opt/app/data",
	}

	assert.Equal(t, "/data", vol.ContainerPath)
	assert.Equal(t, "/opt/app/data", vol.Default)
}

func TestInstalledApp_Struct(t *testing.T) {
	now := time.Now()
	app := InstalledApp{
		ID:          "installed-1",
		Name:        "nginx",
		DisplayName: "Nginx",
		TemplateID:  "nginx-template",
		Version:     "1.25.0",
		Status:      "running",
		InstallTime: now,
		Ports:       map[int]int{80: 8080},
		Volumes:     map[string]string{"/data": "/opt/nginx/data"},
		Environment: map[string]string{"DEBUG": "false"},
		ContainerID: "container-123",
		ComposePath: "/opt/apps/nginx/docker-compose.yml",
	}

	assert.Equal(t, "installed-1", app.ID)
	assert.Equal(t, "running", app.Status)
	assert.Equal(t, 8080, app.Ports[80])
}

func TestAppStore_loadBuiltinTemplates(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewAppStore(nil, tempDir)
	require.NoError(t, err)

	// Verify built-in templates were loaded
	assert.NotEmpty(t, store.templates)

	// Check for known templates
	knownTemplates := []string{"nextcloud", "jellyfin"}
	for _, id := range knownTemplates {
		template := store.GetTemplate(id)
		if template != nil {
			assert.NotEmpty(t, template.Name)
			assert.NotEmpty(t, template.Image)
		}
	}
}

func TestAppStore_renderCompose(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewAppStore(nil, tempDir)
	require.NoError(t, err)

	template := &AppTemplate{
		Compose: `version: '3'
services:
  app:
    image: test:latest
    ports:
      - "{{.Port}}:80"`,
	}

	params := map[string]interface{}{
		"Port": 8080,
	}

	result := store.renderCompose(template, params)
	assert.Contains(t, result, "8080:80")
}

func TestAppStore_generateDefaultCompose(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewAppStore(nil, tempDir)
	require.NoError(t, err)

	template := &AppTemplate{
		Name:    "test-app",
		Image:   "test/app:latest",
		Ports:   []PortConfig{{Port: 80, Default: 8080}},
		Volumes: []VolumeConfig{{ContainerPath: "/data", Default: "/opt/test/data"}},
		Environment: map[string]string{
			"DEBUG": "true",
		},
	}

	params := map[string]interface{}{
		"ports":    []string{"8080:80"},
		"volumes":  []string{"/data:/data"},
		"env":      map[string]string{"DEBUG": "true"},
	}

	result := store.generateDefaultCompose(template, params)
	assert.Contains(t, result, "version:")
	assert.Contains(t, result, "test/app:latest")
}

func TestAppStore_loadInstalled(t *testing.T) {
	t.Run("non-existent file", func(t *testing.T) {
		store := &AppStore{
			dataFile: "/nonexistent/installed.json",
			installed: make(map[string]*InstalledApp),
		}

		err := store.loadInstalled()
		// Should handle gracefully
		_ = err
	})
}

func TestAppStore_InstallApp(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewAppStore(nil, tempDir)
	require.NoError(t, err)

	// InstallApp requires Docker manager, so test with nil
	template := &AppTemplate{
		ID:    "test",
		Name:  "test-app",
		Image: "test:latest",
	}

	// With nil manager, this should fail or handle gracefully
	_, err = store.InstallApp(template, nil)
	// Error expected without Docker connection
	_ = err
}

func TestAppStore_UninstallApp(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewAppStore(nil, tempDir)
	require.NoError(t, err)

	// Add an installed app manually
	store.installed["app-1"] = &InstalledApp{
		ID:     "app-1",
		Name:   "test",
		Status: "stopped",
	}

	// With nil manager, uninstall may fail
	err = store.UninstallApp("app-1")
	_ = err
}

func TestAppStore_StartApp(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewAppStore(nil, tempDir)
	require.NoError(t, err)

	// Add an installed app
	store.installed["app-1"] = &InstalledApp{
		ID:   "app-1",
		Name: "test",
	}

	// StartApp requires Docker manager
	err = store.StartApp("app-1")
	_ = err
}

func TestAppStore_StopApp(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewAppStore(nil, tempDir)
	require.NoError(t, err)

	store.installed["app-1"] = &InstalledApp{
		ID:   "app-1",
		Name: "test",
	}

	err = store.StopApp("app-1")
	_ = err
}

func TestAppStore_RestartApp(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewAppStore(nil, tempDir)
	require.NoError(t, err)

	store.installed["app-1"] = &InstalledApp{
		ID:   "app-1",
		Name: "test",
	}

	err = store.RestartApp("app-1")
	_ = err
}

func TestAppStore_UpdateApp(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewAppStore(nil, tempDir)
	require.NoError(t, err)

	store.installed["app-1"] = &InstalledApp{
		ID:      "app-1",
		Name:    "test",
		Version: "1.0.0",
	}

	err = store.UpdateApp("app-1")
	_ = err
}

func TestAppStore_GetAppStats(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewAppStore(nil, tempDir)
	require.NoError(t, err)

	store.installed["app-1"] = &InstalledApp{
		ID:   "app-1",
		Name: "test",
	}

	stats, err := store.GetAppStats("app-1")
	// May fail without Docker connection
	_ = stats
	_ = err
}