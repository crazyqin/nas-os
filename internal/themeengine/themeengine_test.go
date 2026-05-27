package themeengine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestThemeManagerCreate(t *testing.T) {
	m := NewThemeManager()

	theme := &Theme{
		ID:          "custom-light",
		Name:        "Custom Light",
		Description: "A custom light theme",
		IsDark:      false,
	}

	if err := m.Create(theme); err != nil {
		t.Errorf("Create() error: %v", err)
	}

	got, err := m.Get("custom-light")
	if err != nil {
		t.Errorf("Get() error: %v", err)
	}
	if got.Name != "Custom Light" {
		t.Errorf("expected name 'Custom Light', got %s", got.Name)
	}
}

func TestThemeManagerCreateDuplicate(t *testing.T) {
	m := NewThemeManager()

	theme := &Theme{
		ID:   "test-theme",
		Name: "Test Theme",
	}

	if err := m.Create(theme); err != nil {
		t.Errorf("first Create() error: %v", err)
	}

	if err := m.Create(theme); err == nil {
		t.Error("expected error for duplicate theme, got nil")
	}
}

func TestThemeManagerDelete(t *testing.T) {
	m := NewThemeManager()

	theme := &Theme{
		ID:   "deletable",
		Name: "Deletable Theme",
	}

	if err := m.Create(theme); err != nil {
		t.Errorf("Create() error: %v", err)
	}

	if err := m.Delete("deletable"); err != nil {
		t.Errorf("Delete() error: %v", err)
	}

	if _, err := m.Get("deletable"); err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestThemeManagerDeleteBuiltin(t *testing.T) {
	m := NewThemeManager()

	if err := m.Delete("default-light"); err == nil {
		t.Error("expected error deleting built-in theme, got nil")
	}
}

func TestThemeManagerSwitch(t *testing.T) {
	m := NewThemeManager()

	theme := &Theme{
		ID:   "switchable",
		Name: "Switchable Theme",
	}

	if err := m.Create(theme); err != nil {
		t.Errorf("Create() error: %v", err)
	}

	if err := m.Switch("switchable"); err != nil {
		t.Errorf("Switch() error: %v", err)
	}

	active := m.GetActive()
	if active.ID != "switchable" {
		t.Errorf("expected active theme 'switchable', got %s", active.ID)
	}
}

func TestThemeManagerList(t *testing.T) {
	m := NewThemeManager()

	themes := m.List()
	if len(themes) != 2 {
		t.Errorf("expected 2 built-in themes, got %d", len(themes))
	}
}

func TestDarkModeToggle(t *testing.T) {
	themes := NewThemeManager()
	dm := NewDarkModeManager(themes)

	if dm.IsEnabled() {
		t.Error("expected dark mode to be disabled initially")
	}

	if err := dm.Toggle(); err != nil {
		t.Errorf("Toggle() error: %v", err)
	}

	if !dm.IsEnabled() {
		t.Error("expected dark mode to be enabled after toggle")
	}

	active := themes.GetActive()
	if active.ID != "default-dark" {
		t.Errorf("expected dark theme active, got %s", active.ID)
	}
}

func TestDarkModeEnableDisable(t *testing.T) {
	themes := NewThemeManager()
	dm := NewDarkModeManager(themes)

	if err := dm.Enable(); err != nil {
		t.Errorf("Enable() error: %v", err)
	}
	if !dm.IsEnabled() {
		t.Error("expected dark mode enabled")
	}

	if err := dm.Disable(); err != nil {
		t.Errorf("Disable() error: %v", err)
	}
	if dm.IsEnabled() {
		t.Error("expected dark mode disabled")
	}
}

func TestDarkModeAutoSchedule(t *testing.T) {
	themes := NewThemeManager()
	dm := NewDarkModeManager(themes)

	if err := dm.SetAutoSchedule("22:00", "06:00"); err != nil {
		t.Errorf("SetAutoSchedule() error: %v", err)
	}

	config := dm.GetConfig()
	if config.Type != DarkModeAuto {
		t.Errorf("expected auto mode, got %s", config.Type)
	}
	if config.StartTime != "22:00" {
		t.Errorf("expected start time 22:00, got %s", config.StartTime)
	}
}

func TestDarkModeInvalidSchedule(t *testing.T) {
	themes := NewThemeManager()
	dm := NewDarkModeManager(themes)

	if err := dm.SetAutoSchedule("invalid", "06:00"); err == nil {
		t.Error("expected error for invalid start time")
	}

	if err := dm.SetAutoSchedule("22:00", "invalid"); err == nil {
		t.Error("expected error for invalid end time")
	}
}

func TestColorSchemeCreate(t *testing.T) {
	cm := NewColorSchemeManager()

	scheme := &ColorScheme{
		Primary:       "#FF0000",
		Secondary:     "#00FF00",
		Accent:        "#0000FF",
		Background:    "#FFFFFF",
		Surface:       "#F0F0F0",
		Error:         "#FF0000",
		Warning:       "#FFA500",
		Success:       "#00FF00",
		Info:          "#0000FF",
		TextPrimary:   "#000000",
		TextSecondary: "#666666",
		Border:        "#CCCCCC",
	}

	if err := cm.Create("custom", scheme); err != nil {
		t.Errorf("Create() error: %v", err)
	}

	got, err := cm.Get("custom")
	if err != nil {
		t.Errorf("Get() error: %v", err)
	}
	if got.Primary != "#FF0000" {
		t.Errorf("expected primary #FF0000, got %s", got.Primary)
	}
}

func TestColorSchemeInvalidColor(t *testing.T) {
	cm := NewColorSchemeManager()

	scheme := &ColorScheme{
		Primary:       "invalid",
		Secondary:     "#00FF00",
		Accent:        "#0000FF",
		Background:    "#FFFFFF",
		Surface:       "#F0F0F0",
		Error:         "#FF0000",
		Warning:       "#FFA500",
		Success:       "#00FF00",
		Info:          "#0000FF",
		TextPrimary:   "#000000",
		TextSecondary: "#666666",
		Border:        "#CCCCCC",
	}

	if err := cm.Create("invalid-scheme", scheme); err == nil {
		t.Error("expected error for invalid color")
	}
}

func TestColorSchemeList(t *testing.T) {
	cm := NewColorSchemeManager()

	names := cm.List()
	if len(names) != 2 {
		t.Errorf("expected 2 built-in schemes, got %d", len(names))
	}
}

func TestLayoutCreate(t *testing.T) {
	lm := NewLayoutManager()

	layout := &Layout{
		SidebarPosition: SidebarRight,
		SidebarWidth:    300,
		ContentWidth:    ContentWidthBoxed,
		HeaderHeight:    80,
		FooterVisible:   false,
		CompactMode:     true,
		FontScale:       1.2,
	}

	if err := lm.Create("custom", layout); err != nil {
		t.Errorf("Create() error: %v", err)
	}

	got, err := lm.Get("custom")
	if err != nil {
		t.Errorf("Get() error: %v", err)
	}
	if got.SidebarPosition != SidebarRight {
		t.Errorf("expected sidebar right, got %s", got.SidebarPosition)
	}
}

func TestLayoutInvalidWidth(t *testing.T) {
	lm := NewLayoutManager()

	layout := &Layout{
		SidebarPosition: SidebarLeft,
		SidebarWidth:    700, // Invalid: > 600
		ContentWidth:    ContentWidthFull,
		HeaderHeight:    64,
		FooterVisible:   true,
		CompactMode:     false,
		FontScale:       1.0,
	}

	if err := lm.Create("invalid", layout); err == nil {
		t.Error("expected error for invalid sidebar width")
	}
}

func TestLayoutSetActive(t *testing.T) {
	lm := NewLayoutManager()

	layout := &Layout{
		SidebarPosition: SidebarLeft,
		SidebarWidth:    260,
		ContentWidth:    ContentWidthWide,
		HeaderHeight:    64,
		FooterVisible:   true,
		CompactMode:     false,
		FontScale:       1.0,
	}

	if err := lm.Create("new-layout", layout); err != nil {
		t.Errorf("Create() error: %v", err)
	}

	if err := lm.SetActive("new-layout"); err != nil {
		t.Errorf("SetActive() error: %v", err)
	}

	active := lm.GetActive()
	if active.SidebarWidth != 260 {
		t.Errorf("expected sidebar width 260, got %d", active.SidebarWidth)
	}
}

func TestImportExport(t *testing.T) {
	tmpDir := t.TempDir()

	themes := NewThemeManager()
	colors := NewColorSchemeManager()
	layouts := NewLayoutManager()
	ie := NewImportExportManager(tmpDir, themes, colors, layouts)

	// Create a custom theme to export
	theme := &Theme{
		ID:          "export-test",
		Name:        "Export Test",
		Description: "Theme for export testing",
		IsDark:      false,
		Colors:      DefaultLightColors(),
		Layout:      DefaultLayout(),
	}

	if err := themes.Create(theme); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Export
	exportPath := filepath.Join(tmpDir, "export-test.json")
	if err := ie.Export("export-test", exportPath); err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(exportPath); os.IsNotExist(err) {
		t.Fatal("export file does not exist")
	}

	// Delete the theme so we can import it
	if err := themes.Delete("export-test"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// Import
	imported, err := ie.Import(exportPath)
	if err != nil {
		t.Fatalf("Import() error: %v", err)
	}

	if imported.ID != "export-test" {
		t.Errorf("expected ID 'export-test', got %s", imported.ID)
	}
	if imported.Name != "Export Test" {
		t.Errorf("expected name 'Export Test', got %s", imported.Name)
	}
}

func TestImportDuplicate(t *testing.T) {
	tmpDir := t.TempDir()

	themes := NewThemeManager()
	colors := NewColorSchemeManager()
	layouts := NewLayoutManager()
	ie := NewImportExportManager(tmpDir, themes, colors, layouts)

	// Create and export a theme
	theme := &Theme{
		ID:   "dup-test",
		Name: "Duplicate Test",
	}

	if err := themes.Create(theme); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	exportPath := filepath.Join(tmpDir, "dup-test.json")
	if err := ie.Export("dup-test", exportPath); err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	// Try to import the same theme again
	if _, err := ie.Import(exportPath); err == nil {
		t.Error("expected error for duplicate import")
	}
}

func TestValidatePackage(t *testing.T) {
	tmpDir := t.TempDir()

	themes := NewThemeManager()
	colors := NewColorSchemeManager()
	layouts := NewLayoutManager()
	ie := NewImportExportManager(tmpDir, themes, colors, layouts)

	// Create and export a theme
	theme := &Theme{
		ID:   "validate-test",
		Name: "Validate Test",
	}

	if err := themes.Create(theme); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	exportPath := filepath.Join(tmpDir, "validate-test.json")
	if err := ie.Export("validate-test", exportPath); err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	// Validate the package
	if err := ie.ValidatePackage(exportPath); err != nil {
		t.Errorf("ValidatePackage() error: %v", err)
	}
}

func TestDefaultLightColors(t *testing.T) {
	scheme := DefaultLightColors()
	if scheme.Primary != "#1976D2" {
		t.Errorf("expected primary #1976D2, got %s", scheme.Primary)
	}
	if scheme.Background != "#FFFFFF" {
		t.Errorf("expected background #FFFFFF, got %s", scheme.Background)
	}
}

func TestDefaultDarkColors(t *testing.T) {
	scheme := DefaultDarkColors()
	if scheme.Primary != "#90CAF9" {
		t.Errorf("expected primary #90CAF9, got %s", scheme.Primary)
	}
	if scheme.Background != "#121212" {
		t.Errorf("expected background #121212, got %s", scheme.Background)
	}
}

func TestDefaultLayout(t *testing.T) {
	layout := DefaultLayout()
	if layout.SidebarPosition != SidebarLeft {
		t.Errorf("expected sidebar left, got %s", layout.SidebarPosition)
	}
	if layout.SidebarWidth != 260 {
		t.Errorf("expected sidebar width 260, got %d", layout.SidebarWidth)
	}
	if layout.FontScale != 1.0 {
		t.Errorf("expected font scale 1.0, got %f", layout.FontScale)
	}
}
