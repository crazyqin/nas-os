package themeengine

import (
	"fmt"
	"sync"
	"time"
)

// Theme represents a UI theme configuration.
type Theme struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	IsDark      bool         `json:"is_dark"`
	Colors      *ColorScheme `json:"colors,omitempty"`
	Layout      *Layout      `json:"layout,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// ThemeManager manages theme creation, deletion, and switching.
type ThemeManager struct {
	mu          sync.RWMutex
	themes      map[string]*Theme
	activeTheme string
}

// NewThemeManager creates a new theme manager with default themes.
func NewThemeManager() *ThemeManager {
	m := &ThemeManager{
		themes: make(map[string]*Theme),
	}

	// Register built-in themes
	m.registerBuiltinThemes()
	return m
}

// registerBuiltinThemes registers the default light and dark themes.
func (m *ThemeManager) registerBuiltinThemes() {
	light := &Theme{
		ID:          "default-light",
		Name:        "默认浅色",
		Description: "系统默认浅色主题",
		IsDark:      false,
		Colors:      DefaultLightColors(),
		Layout:      DefaultLayout(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	dark := &Theme{
		ID:          "default-dark",
		Name:        "默认深色",
		Description: "系统默认深色主题",
		IsDark:      true,
		Colors:      DefaultDarkColors(),
		Layout:      DefaultLayout(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.themes[light.ID] = light
	m.themes[dark.ID] = dark
	m.activeTheme = light.ID
}

// Create creates a new theme.
func (m *ThemeManager) Create(theme *Theme) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if theme.ID == "" {
		return fmt.Errorf("theme ID cannot be empty")
	}

	if _, exists := m.themes[theme.ID]; exists {
		return fmt.Errorf("theme %s already exists", theme.ID)
	}

	theme.CreatedAt = time.Now()
	theme.UpdatedAt = time.Now()

	// Set defaults if not provided
	if theme.Colors == nil {
		if theme.IsDark {
			theme.Colors = DefaultDarkColors()
		} else {
			theme.Colors = DefaultLightColors()
		}
	}
	if theme.Layout == nil {
		theme.Layout = DefaultLayout()
	}

	m.themes[theme.ID] = theme
	return nil
}

// Delete removes a theme. Cannot delete built-in or active themes.
func (m *ThemeManager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	theme, exists := m.themes[id]
	if !exists {
		return fmt.Errorf("theme %s not found", id)
	}

	if id == "default-light" || id == "default-dark" {
		return fmt.Errorf("cannot delete built-in theme %s", id)
	}

	if id == m.activeTheme {
		return fmt.Errorf("cannot delete active theme %s", id)
	}

	_ = theme // suppress unused warning
	delete(m.themes, id)
	return nil
}

// Switch switches the active theme.
func (m *ThemeManager) Switch(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.themes[id]; !exists {
		return fmt.Errorf("theme %s not found", id)
	}

	m.activeTheme = id
	return nil
}

// GetActive returns the currently active theme.
func (m *ThemeManager) GetActive() *Theme {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.themes[m.activeTheme]
}

// Get returns a theme by ID.
func (m *ThemeManager) Get(id string) (*Theme, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	theme, exists := m.themes[id]
	if !exists {
		return nil, fmt.Errorf("theme %s not found", id)
	}

	return theme, nil
}

// List returns all registered themes.
func (m *ThemeManager) List() []*Theme {
	m.mu.RLock()
	defer m.mu.RUnlock()

	themes := make([]*Theme, 0, len(m.themes))
	for _, t := range m.themes {
		themes = append(themes, t)
	}
	return themes
}

// Update updates an existing theme's properties.
func (m *ThemeManager) Update(id string, updateFn func(*Theme)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	theme, exists := m.themes[id]
	if !exists {
		return fmt.Errorf("theme %s not found", id)
	}

	if id == "default-light" || id == "default-dark" {
		return fmt.Errorf("cannot modify built-in theme %s", id)
	}

	updateFn(theme)
	theme.UpdatedAt = time.Now()
	return nil
}
