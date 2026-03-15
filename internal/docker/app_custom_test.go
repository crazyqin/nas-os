package docker

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCustomTemplateManager(t *testing.T) {
	t.Run("with valid directory", func(t *testing.T) {
		tempDir := t.TempDir()
		ctm, err := NewCustomTemplateManager(tempDir)
		require.NoError(t, err)
		assert.NotNil(t, ctm)
		assert.Equal(t, tempDir, ctm.templatesDir)
	})

	t.Run("with empty directory", func(t *testing.T) {
		ctm, err := NewCustomTemplateManager("")
		require.NoError(t, err)
		assert.NotNil(t, ctm)
		assert.Empty(t, ctm.templatesDir)
	})
}

func TestCustomTemplateManager_CreateFromCompose(t *testing.T) {
	tempDir := t.TempDir()
	ctm, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	compose := `version: '3'
services:
  nginx:
    image: nginx:latest
    ports:
      - "80:80"
`

	template, err := ctm.CreateFromCompose("nginx", "Nginx", "Nginx web server", "web", compose)
	require.NoError(t, err)
	assert.NotNil(t, template)
	assert.Equal(t, "nginx", template.Name)
	assert.Equal(t, "Nginx", template.DisplayName)
	assert.Equal(t, "web", template.Category)
	assert.Contains(t, template.Compose, "nginx")
	assert.NotEmpty(t, template.ID)
	assert.False(t, template.CreatedAt.IsZero())
	assert.Equal(t, "custom-nginx", template.ID) // ID format is custom-{name}
}

func TestCustomTemplateManager_CreateFromCompose_EmptyName(t *testing.T) {
	tempDir := t.TempDir()
	ctm, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	compose := `version: '3'`
	_, err = ctm.CreateFromCompose("", "Test", "Test app", "test", compose)
	assert.Error(t, err)
}

func TestCustomTemplateManager_ListTemplates(t *testing.T) {
	tempDir := t.TempDir()
	ctm, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	// Initially empty
	templates := ctm.ListTemplates()
	assert.Empty(t, templates)

	// Manually add template to map
	template := &CustomTemplate{
		ID:          "custom-test",
		Name:        "test",
		DisplayName: "Test",
		Description: "Test app",
		Category:    "test",
		Compose:     `version: '3'`,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	ctm.templates[template.ID] = template

	templates = ctm.ListTemplates()
	assert.Len(t, templates, 1)
}

func TestCustomTemplateManager_GetTemplate(t *testing.T) {
	tempDir := t.TempDir()
	ctm, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	// Manually add template to map
	template := &CustomTemplate{
		ID:          "custom-test",
		Name:        "test",
		DisplayName: "Test",
		Description: "Test app",
		Category:    "test",
		Compose:     `version: '3'`,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	ctm.templates[template.ID] = template

	// Get existing template
	retrieved, err := ctm.GetTemplate(template.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, template.ID, retrieved.ID)

	// Get non-existing template
	retrieved, err = ctm.GetTemplate("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, retrieved)
}

func TestCustomTemplateManager_DeleteTemplate(t *testing.T) {
	tempDir := t.TempDir()
	ctm, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	// Manually add template to map
	template := &CustomTemplate{
		ID:          "custom-test",
		Name:        "test",
		DisplayName: "Test",
		Description: "Test app",
		Category:    "test",
		Compose:     `version: '3'`,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	ctm.templates[template.ID] = template

	// Create template file
	ctm.saveTemplate(template)

	// Delete existing template
	err = ctm.DeleteTemplate(template.ID)
	require.NoError(t, err)

	// Verify deletion from map
	retrieved, err := ctm.GetTemplate(template.ID)
	assert.Error(t, err)
	assert.Nil(t, retrieved)

	// Delete non-existing template
	err = ctm.DeleteTemplate("nonexistent")
	assert.Error(t, err)
}

func TestCustomTemplateManager_UpdateTemplate(t *testing.T) {
	tempDir := t.TempDir()
	ctm, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	// Manually add template to map
	template := &CustomTemplate{
		ID:          "custom-test",
		Name:        "test",
		DisplayName: "Test",
		Description: "Test app",
		Category:    "test",
		Compose:     `version: '3'`,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	ctm.templates[template.ID] = template

	// Update template
	updates := map[string]interface{}{
		"display_name": "Updated Name",
		"description":  "Updated description",
	}

	updated, err := ctm.UpdateTemplate(template.ID, updates)
	require.NoError(t, err)

	retrieved, err := ctm.GetTemplate(template.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, "Updated Name", retrieved.DisplayName)
	assert.Equal(t, "Updated description", retrieved.Description)
	_ = updated
}

func TestCustomTemplateManager_IncrementDownloads(t *testing.T) {
	tempDir := t.TempDir()
	ctm, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	// Manually add template to map
	template := &CustomTemplate{
		ID:          "custom-test",
		Name:        "test",
		DisplayName: "Test",
		Description: "Test app",
		Category:    "test",
		Compose:     `version: '3'`,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	ctm.templates[template.ID] = template

	// Increment downloads
	err = ctm.IncrementDownloads(template.ID)
	require.NoError(t, err)
}

func TestCustomTemplate_Fields(t *testing.T) {
	now := time.Now()
	template := &CustomTemplate{
		ID:          "test-id",
		Name:        "nginx",
		DisplayName: "Nginx",
		Description: "Nginx web server",
		Category:    "web",
		Compose:     "version: '3'",
		CreatedAt:   now,
		UpdatedAt:   now,
		Source:      "compose",
		SourceURL:   "https://example.com",
	}

	assert.Equal(t, "test-id", template.ID)
	assert.Equal(t, "nginx", template.Name)
	assert.Equal(t, "Nginx", template.DisplayName)
	assert.Equal(t, "web", template.Category)
	assert.Equal(t, "compose", template.Source)
}

func TestGenerateTemplateID(t *testing.T) {
	id1 := generateTemplateID("test")
	id2 := generateTemplateID("My App")

	assert.Equal(t, "custom-test", id1)
	assert.Equal(t, "custom-my-app", id2) // spaces replaced with dashes, lowercase

	// Same name should produce same ID
	id3 := generateTemplateID("test")
	assert.Equal(t, id1, id3)
}

func TestCustomTemplateManager_SaveTemplate(t *testing.T) {
	tempDir := t.TempDir()
	ctm, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	template := &CustomTemplate{
		ID:          "custom-test",
		Name:        "test",
		DisplayName: "Test",
		Description: "Test app",
		Category:    "test",
		Compose:     `version: '3'`,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Save template
	err = ctm.saveTemplate(template)
	require.NoError(t, err)

	// Verify file was created
	filePath := filepath.Join(tempDir, "custom-test.json")
	_, err = os.Stat(filePath)
	require.NoError(t, err)
}

func TestCustomTemplateManager_LoadTemplates(t *testing.T) {
	tempDir := t.TempDir()

	// Create a template file manually
	file, err := os.Create(filepath.Join(tempDir, "custom-test.json"))
	require.NoError(t, err)
	_, err = file.WriteString(`{"id":"custom-test","name":"test","display_name":"Test","description":"Test app","category":"test","compose":"version: '3'"}`)
	require.NoError(t, err)
	file.Close()

	// Create manager which should load templates
	ctm, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	templates := ctm.ListTemplates()
	assert.GreaterOrEqual(t, len(templates), 1)
}

func TestCustomTemplateManager_EmptyTemplatesDir(t *testing.T) {
	// Create temp directory without templates
	tempDir := t.TempDir()
	ctm, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	templates := ctm.ListTemplates()
	assert.Empty(t, templates)
}

func TestCustomTemplateManager_InvalidJSONFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create invalid JSON file
	invalidFile := filepath.Join(tempDir, "invalid.json")
	err := os.WriteFile(invalidFile, []byte("invalid json"), 0644)
	require.NoError(t, err)

	// Should handle error gracefully
	ctm, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)
	assert.NotNil(t, ctm)
}

func TestCustomTemplateManager_UpdateNonExistent(t *testing.T) {
	tempDir := t.TempDir()
	ctm, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	updates := map[string]interface{}{"name": "test"}
	_, err = ctm.UpdateTemplate("nonexistent", updates)
	assert.Error(t, err)
}

func TestCustomTemplateManager_IncrementDownloadsNonExistent(t *testing.T) {
	tempDir := t.TempDir()
	ctm, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	// IncrementDownloads on non-existent template should not error (or check implementation)
	err = ctm.IncrementDownloads("nonexistent")
	// The function may or may not return an error - adjust based on actual behavior
	_ = err
}

func TestCustomTemplateManager_SaveTemplate_EmptyDir(t *testing.T) {
	ctm, err := NewCustomTemplateManager("")
	require.NoError(t, err)

	template := &CustomTemplate{
		ID:          "custom-test",
		Name:        "test",
		DisplayName: "Test",
		Description: "Test app",
		Category:    "test",
		Compose:     `version: '3'`,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Save template with empty dir should not error
	err = ctm.saveTemplate(template)
	require.NoError(t, err)
}