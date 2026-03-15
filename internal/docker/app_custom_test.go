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
}

func TestCustomTemplateManager_ListTemplates(t *testing.T) {
	tempDir := t.TempDir()
	ctm, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	// Initially empty
	templates := ctm.ListTemplates()
	assert.Empty(t, templates)

	// Create a template
	compose := `version: '3'`
	_, err = ctm.CreateFromCompose("test", "Test", "Test app", "test", compose)
	require.NoError(t, err)

	templates = ctm.ListTemplates()
	assert.Len(t, templates, 1)
}

func TestCustomTemplateManager_GetTemplate(t *testing.T) {
	tempDir := t.TempDir()
	ctm, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	compose := `version: '3'`
	template, err := ctm.CreateFromCompose("test", "Test", "Test app", "test", compose)
	require.NoError(t, err)

	// Get existing template
	retrieved := ctm.GetTemplate(template.ID)
	require.NotNil(t, retrieved)
	assert.Equal(t, template.ID, retrieved.ID)

	// Get non-existing template
	retrieved = ctm.GetTemplate("nonexistent")
	assert.Nil(t, retrieved)
}

func TestCustomTemplateManager_DeleteTemplate(t *testing.T) {
	tempDir := t.TempDir()
	ctm, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	compose := `version: '3'`
	template, err := ctm.CreateFromCompose("test", "Test", "Test app", "test", compose)
	require.NoError(t, err)

	// Delete existing template
	err = ctm.DeleteTemplate(template.ID)
	require.NoError(t, err)

	// Verify deletion
	retrieved := ctm.GetTemplate(template.ID)
	assert.Nil(t, retrieved)

	// Delete non-existing template
	err = ctm.DeleteTemplate("nonexistent")
	assert.Error(t, err)
}

func TestCustomTemplateManager_UpdateTemplate(t *testing.T) {
	tempDir := t.TempDir()
	ctm, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	compose := `version: '3'`
	template, err := ctm.CreateFromCompose("test", "Test", "Test app", "test", compose)
	require.NoError(t, err)

	// Update template
	updates := map[string]interface{}{
		"display_name": "Updated Name",
		"description":  "Updated description",
	}

	err = ctm.UpdateTemplate(template.ID, updates)
	require.NoError(t, err)

	retrieved := ctm.GetTemplate(template.ID)
	require.NotNil(t, retrieved)
	assert.Equal(t, "Updated Name", retrieved.DisplayName)
	assert.Equal(t, "Updated description", retrieved.Description)
}

func TestCustomTemplateManager_IncrementDownloads(t *testing.T) {
	tempDir := t.TempDir()
	ctm, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	compose := `version: '3'`
	template, err := ctm.CreateFromCompose("test", "Test", "Test app", "test", compose)
	require.NoError(t, err)

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
	id1 := generateTemplateID()
	id2 := generateTemplateID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}

func TestCustomTemplateManager_SaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()

	// Create manager and template
	ctm1, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	compose := `version: '3'`
	template, err := ctm1.CreateFromCompose("test", "Test", "Test app", "test", compose)
	require.NoError(t, err)

	// Create new manager to load templates
	ctm2, err := NewCustomTemplateManager(tempDir)
	require.NoError(t, err)

	// Verify template was loaded
	retrieved := ctm2.GetTemplate(template.ID)
	require.NotNil(t, retrieved)
	assert.Equal(t, template.ID, retrieved.ID)
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
	err = ctm.UpdateTemplate("nonexistent", updates)
	assert.Error(t, err)
}