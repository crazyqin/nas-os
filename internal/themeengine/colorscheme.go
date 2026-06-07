package themeengine

import (
	"fmt"
	"regexp"
	"sync"
	"time"
)

// ColorScheme represents a complete color configuration for a theme.
type ColorScheme struct {
	Primary       string `json:"primary"`
	Secondary     string `json:"secondary"`
	Accent        string `json:"accent"`
	Background    string `json:"background"`
	Surface       string `json:"surface"`
	Error         string `json:"error"`
	Warning       string `json:"warning"`
	Success       string `json:"success"`
	Info          string `json:"info"`
	TextPrimary   string `json:"text_primary"`
	TextSecondary string `json:"text_secondary"`
	Border        string `json:"border"`
}

// ColorSchemeManager manages custom color schemes.
type ColorSchemeManager struct {
	mu      sync.RWMutex
	schemes map[string]*ColorScheme
}

// NewColorSchemeManager creates a new color scheme manager.
func NewColorSchemeManager() *ColorSchemeManager {
	m := &ColorSchemeManager{
		schemes: make(map[string]*ColorScheme),
	}

	// Register built-in schemes
	m.schemes["default-light"] = DefaultLightColors()
	m.schemes["default-dark"] = DefaultDarkColors()
	return m
}

// DefaultLightColors returns the default light color scheme.
func DefaultLightColors() *ColorScheme {
	return &ColorScheme{
		Primary:       "#1976D2",
		Secondary:     "#424242",
		Accent:        "#82B1FF",
		Background:    "#FFFFFF",
		Surface:       "#F5F5F5",
		Error:         "#D32F2F",
		Warning:       "#FFA000",
		Success:       "#388E3C",
		Info:          "#1976D2",
		TextPrimary:   "#212121",
		TextSecondary: "#757575",
		Border:        "#E0E0E0",
	}
}

// DefaultDarkColors returns the default dark color scheme.
func DefaultDarkColors() *ColorScheme {
	return &ColorScheme{
		Primary:       "#90CAF9",
		Secondary:     "#BDBDBD",
		Accent:        "#82B1FF",
		Background:    "#121212",
		Surface:       "#1E1E1E",
		Error:         "#EF5350",
		Warning:       "#FFB74D",
		Success:       "#66BB6A",
		Info:          "#42A5F5",
		TextPrimary:   "#FFFFFF",
		TextSecondary: "#B0B0B0",
		Border:        "#333333",
	}
}

// Create creates a new custom color scheme.
func (c *ColorSchemeManager) Create(name string, scheme *ColorScheme) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if name == "" {
		return fmt.Errorf("scheme name cannot be empty")
	}

	if _, exists := c.schemes[name]; exists {
		return fmt.Errorf("color scheme %s already exists", name)
	}

	if err := validateColorScheme(scheme); err != nil {
		return err
	}

	c.schemes[name] = scheme
	return nil
}

// Get returns a color scheme by name.
func (c *ColorSchemeManager) Get(name string) (*ColorScheme, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	scheme, exists := c.schemes[name]
	if !exists {
		return nil, fmt.Errorf("color scheme %s not found", name)
	}

	return scheme, nil
}

// Update updates an existing color scheme.
func (c *ColorSchemeManager) Update(name string, updateFn func(*ColorScheme)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	scheme, exists := c.schemes[name]
	if !exists {
		return fmt.Errorf("color scheme %s not found", name)
	}

	if name == "default-light" || name == "default-dark" {
		return fmt.Errorf("cannot modify built-in color scheme %s", name)
	}

	updateFn(scheme)
	return validateColorScheme(scheme)
}

// Delete removes a custom color scheme.
func (c *ColorSchemeManager) Delete(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if name == "default-light" || name == "default-dark" {
		return fmt.Errorf("cannot delete built-in color scheme %s", name)
	}

	if _, exists := c.schemes[name]; !exists {
		return fmt.Errorf("color scheme %s not found", name)
	}

	delete(c.schemes, name)
	return nil
}

// List returns all available color scheme names.
func (c *ColorSchemeManager) List() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.schemes))
	for name := range c.schemes {
		names = append(names, name)
	}
	return names
}

// Apply applies a color scheme to a theme.
func (c *ColorSchemeManager) Apply(schemeName string, theme *Theme) error {
	scheme, err := c.Get(schemeName)
	if err != nil {
		return err
	}

	theme.Colors = scheme
	theme.UpdatedAt = time.Now()
	return nil
}

// validateColorScheme validates all color values in a scheme.
func validateColorScheme(scheme *ColorScheme) error {
	colors := map[string]string{
		"primary":        scheme.Primary,
		"secondary":      scheme.Secondary,
		"accent":         scheme.Accent,
		"background":     scheme.Background,
		"surface":        scheme.Surface,
		"error":          scheme.Error,
		"warning":        scheme.Warning,
		"success":        scheme.Success,
		"info":           scheme.Info,
		"text_primary":   scheme.TextPrimary,
		"text_secondary": scheme.TextSecondary,
		"border":         scheme.Border,
	}

	hexPattern := regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	for name, color := range colors {
		if !hexPattern.MatchString(color) {
			return fmt.Errorf("invalid color value for %s: %s", name, color)
		}
	}

	return nil
}
