package themeengine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ThemePackage represents an importable/exportable theme package.
type ThemePackage struct {
	Version     string       `json:"version"`
	Theme       *Theme       `json:"theme"`
	ColorScheme *ColorScheme `json:"color_scheme,omitempty"`
	Layout      *Layout      `json:"layout,omitempty"`
	ExportedAt  string       `json:"exported_at"`
}

// ImportExportManager handles theme import and export operations.
type ImportExportManager struct {
	mu        sync.RWMutex
	exportDir string
	themes    *ThemeManager
	colors    *ColorSchemeManager
	layouts   *LayoutManager
}

// NewImportExportManager creates a new import/export manager.
func NewImportExportManager(exportDir string, themes *ThemeManager, colors *ColorSchemeManager, layouts *LayoutManager) *ImportExportManager {
	return &ImportExportManager{
		exportDir: exportDir,
		themes:    themes,
		colors:    colors,
		layouts:   layouts,
	}
}

// Export exports a theme to a JSON file.
func (ie *ImportExportManager) Export(themeID, outputPath string) error {
	ie.mu.RLock()
	defer ie.mu.RUnlock()

	theme, err := ie.themes.Get(themeID)
	if err != nil {
		return fmt.Errorf("failed to get theme: %w", err)
	}

	pkg := &ThemePackage{
		Version:     "1.0",
		Theme:       theme,
		ColorScheme: theme.Colors,
		Layout:      theme.Layout,
		ExportedAt:  timeNow().Format("2006-01-02T15:04:05Z"),
	}

	data, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal theme: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Import imports a theme from a JSON file.
func (ie *ImportExportManager) Import(inputPath string) (*Theme, error) {
	ie.mu.Lock()
	defer ie.mu.Unlock()

	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var pkg ThemePackage
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal theme: %w", err)
	}

	if pkg.Version == "" {
		return nil, fmt.Errorf("invalid theme package: missing version")
	}

	if pkg.Theme == nil {
		return nil, fmt.Errorf("invalid theme package: missing theme data")
	}

	// Check if theme already exists
	if _, exists := ie.themes.themes[pkg.Theme.ID]; exists {
		return nil, fmt.Errorf("theme %s already exists", pkg.Theme.ID)
	}

	// Import color scheme if present
	if pkg.ColorScheme != nil {
		pkg.Theme.Colors = pkg.ColorScheme
	}

	// Import layout if present
	if pkg.Layout != nil {
		pkg.Theme.Layout = pkg.Layout
	}

	// Register the theme
	ie.themes.themes[pkg.Theme.ID] = pkg.Theme
	return pkg.Theme, nil
}

// ExportToFile exports a theme to the default export directory.
func (ie *ImportExportManager) ExportToFile(themeID, filename string) error {
	outputPath := filepath.Join(ie.exportDir, filename)
	return ie.Export(themeID, outputPath)
}

// ImportFromFile imports a theme from the default export directory.
func (ie *ImportExportManager) ImportFromFile(filename string) (*Theme, error) {
	inputPath := filepath.Join(ie.exportDir, filename)
	return ie.Import(inputPath)
}

// ListExports lists all exported theme files in the export directory.
func (ie *ImportExportManager) ListExports() ([]string, error) {
	ie.mu.RLock()
	defer ie.mu.RUnlock()

	entries, err := os.ReadDir(ie.exportDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read export directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

// ValidatePackage validates a theme package file without importing it.
func (ie *ImportExportManager) ValidatePackage(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var pkg ThemePackage
	if err := json.Unmarshal(data, &pkg); err != nil {
		return fmt.Errorf("invalid JSON format: %w", err)
	}

	if pkg.Version == "" {
		return fmt.Errorf("missing version field")
	}

	if pkg.Theme == nil {
		return fmt.Errorf("missing theme data")
	}

	if pkg.Theme.ID == "" {
		return fmt.Errorf("theme missing ID")
	}

	if pkg.Theme.Name == "" {
		return fmt.Errorf("theme missing name")
	}

	return nil
}

// timeNow returns the current time (allows for testing).
var timeNow = func() time.Time {
	return time.Now()
}
