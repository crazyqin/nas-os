package themeengine

import (
	"fmt"
	"sync"
	"time"
)

// SidebarPosition represents sidebar position.
type SidebarPosition string

const (
	SidebarLeft   SidebarPosition = "left"
	SidebarRight  SidebarPosition = "right"
	SidebarHidden SidebarPosition = "hidden"
)

// ContentWidth represents the content area width style.
type ContentWidth string

const (
	ContentWidthFull  ContentWidth = "full"
	ContentWidthBoxed ContentWidth = "boxed"
	ContentWidthWide  ContentWidth = "wide"
)

// Layout represents a page layout configuration.
type Layout struct {
	SidebarPosition SidebarPosition `json:"sidebar_position"`
	SidebarWidth    int             `json:"sidebar_width"`
	ContentWidth    ContentWidth    `json:"content_width"`
	HeaderHeight    int             `json:"header_height"`
	FooterVisible   bool            `json:"footer_visible"`
	CompactMode     bool            `json:"compact_mode"`
	FontScale       float64         `json:"font_scale"`
}

// LayoutManager manages layout configurations.
type LayoutManager struct {
	mu       sync.RWMutex
	layouts  map[string]*Layout
	activeID string
}

// NewLayoutManager creates a new layout manager with default layout.
func NewLayoutManager() *LayoutManager {
	m := &LayoutManager{
		layouts:  make(map[string]*Layout),
		activeID: "default",
	}

	m.layouts["default"] = DefaultLayout()
	return m
}

// DefaultLayout returns the default layout configuration.
func DefaultLayout() *Layout {
	return &Layout{
		SidebarPosition: SidebarLeft,
		SidebarWidth:    260,
		ContentWidth:    ContentWidthWide,
		HeaderHeight:    64,
		FooterVisible:   true,
		CompactMode:     false,
		FontScale:       1.0,
	}
}

// Create creates a new layout configuration.
func (lm *LayoutManager) Create(id string, layout *Layout) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if id == "" {
		return fmt.Errorf("layout ID cannot be empty")
	}

	if _, exists := lm.layouts[id]; exists {
		return fmt.Errorf("layout %s already exists", id)
	}

	if err := validateLayout(layout); err != nil {
		return err
	}

	lm.layouts[id] = layout
	return nil
}

// Get returns a layout by ID.
func (lm *LayoutManager) Get(id string) (*Layout, error) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	layout, exists := lm.layouts[id]
	if !exists {
		return nil, fmt.Errorf("layout %s not found", id)
	}

	return layout, nil
}

// Update updates an existing layout.
func (lm *LayoutManager) Update(id string, updateFn func(*Layout)) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	layout, exists := lm.layouts[id]
	if !exists {
		return fmt.Errorf("layout %s not found", id)
	}

	if id == "default" {
		return fmt.Errorf("cannot modify default layout")
	}

	updateFn(layout)
	return validateLayout(layout)
}

// Delete removes a layout configuration.
func (lm *LayoutManager) Delete(id string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if id == "default" {
		return fmt.Errorf("cannot delete default layout")
	}

	if id == lm.activeID {
		return fmt.Errorf("cannot delete active layout %s", id)
	}

	if _, exists := lm.layouts[id]; !exists {
		return fmt.Errorf("layout %s not found", id)
	}

	delete(lm.layouts, id)
	return nil
}

// SetActive sets the active layout.
func (lm *LayoutManager) SetActive(id string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if _, exists := lm.layouts[id]; !exists {
		return fmt.Errorf("layout %s not found", id)
	}

	lm.activeID = id
	return nil
}

// GetActive returns the active layout.
func (lm *LayoutManager) GetActive() *Layout {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	return lm.layouts[lm.activeID]
}

// List returns all layout IDs.
func (lm *LayoutManager) List() []string {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	ids := make([]string, 0, len(lm.layouts))
	for id := range lm.layouts {
		ids = append(ids, id)
	}
	return ids
}

// ApplyToTheme applies a layout to a theme.
func (lm *LayoutManager) ApplyToTheme(layoutID string, theme *Theme) error {
	layout, err := lm.Get(layoutID)
	if err != nil {
		return err
	}

	theme.Layout = layout
	theme.UpdatedAt = time.Now()
	return nil
}

// validateLayout validates layout configuration values.
func validateLayout(layout *Layout) error {
	if layout.SidebarWidth < 0 || layout.SidebarWidth > 600 {
		return fmt.Errorf("sidebar width must be between 0 and 600, got %d", layout.SidebarWidth)
	}

	if layout.HeaderHeight < 0 || layout.HeaderHeight > 200 {
		return fmt.Errorf("header height must be between 0 and 200, got %d", layout.HeaderHeight)
	}

	if layout.FontScale < 0.5 || layout.FontScale > 2.0 {
		return fmt.Errorf("font scale must be between 0.5 and 2.0, got %f", layout.FontScale)
	}

	validPositions := map[SidebarPosition]bool{
		SidebarLeft:   true,
		SidebarRight:  true,
		SidebarHidden: true,
	}
	if !validPositions[layout.SidebarPosition] {
		return fmt.Errorf("invalid sidebar position: %s", layout.SidebarPosition)
	}

	validWidths := map[ContentWidth]bool{
		ContentWidthFull:  true,
		ContentWidthBoxed: true,
		ContentWidthWide:  true,
	}
	if !validWidths[layout.ContentWidth] {
		return fmt.Errorf("invalid content width: %s", layout.ContentWidth)
	}

	return nil
}
