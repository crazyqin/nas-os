package desktopmanager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// DesktopIcon represents a desktop icon configuration.
type DesktopIcon struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	Path      string    `json:"path"`
	Type      string    `json:"type"` // app, file, folder, widget, shortcut
	X         int       `json:"x"`
	Y         int       `json:"y"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	GroupID   string    `json:"group_id,omitempty"`
	Visible   bool      `json:"visible"`
	Locked    bool      `json:"locked"`
	Order     int       `json:"order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DesktopGroup represents a group of desktop icons.
type DesktopGroup struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	X         int           `json:"x"`
	Y         int           `json:"y"`
	Width     int           `json:"width"`
	Height    int           `json:"height"`
	Collapsed bool          `json:"collapsed"`
	Icons     []DesktopIcon `json:"icons"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// Wallpaper 壁纸设置.
type Wallpaper struct {
	Path    string `json:"path,omitempty"`
	Mode    string `json:"mode"` // fill, fit, stretch, tile, center
	Color   string `json:"color,omitempty"`
	Opacity int    `json:"opacity"`
	Blur    int    `json:"blur"`
}

// DesktopLayout represents the complete desktop layout.
type DesktopLayout struct {
	Icons     []DesktopIcon  `json:"icons"`
	Groups    []DesktopGroup `json:"groups"`
	Wallpaper *Wallpaper     `json:"wallpaper,omitempty"`
	GridSize  int            `json:"grid_size"`
	Columns   int            `json:"columns"`
	Rows      int            `json:"rows"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// Manager handles desktop layout management.
type Manager struct {
	mu       sync.RWMutex
	layout   DesktopLayout
	gridSize int
}

// NewManager creates a new desktop manager.
func NewManager(gridSize int) *Manager {
	if gridSize <= 0 {
		gridSize = 80
	}
	return &Manager{
		layout: DesktopLayout{
			Icons:     make([]DesktopIcon, 0),
			Groups:    make([]DesktopGroup, 0),
			GridSize:  gridSize,
			UpdatedAt: time.Now(),
		},
		gridSize: gridSize,
	}
}

// GetLayout returns the current desktop layout.
func (m *Manager) GetLayout() DesktopLayout {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.layout
}

// UpdateIconPosition updates the position of an icon.
func (m *Manager) UpdateIconPosition(id string, x, y int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, icon := range m.layout.Icons {
		if icon.ID == id {
			// Snap to grid
			m.layout.Icons[i].X = (x / m.gridSize) * m.gridSize
			m.layout.Icons[i].Y = (y / m.gridSize) * m.gridSize
			m.layout.Icons[i].UpdatedAt = time.Now()
			m.layout.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("icon not found: %s", id)
}

// AddIcon adds a new icon to the desktop.
func (m *Manager) AddIcon(icon DesktopIcon) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	icon.CreatedAt = time.Now()
	icon.UpdatedAt = time.Now()
	icon.Visible = true
	m.layout.Icons = append(m.layout.Icons, icon)
	m.layout.UpdatedAt = time.Now()
	return nil
}

// RemoveIcon removes an icon from the desktop.
func (m *Manager) RemoveIcon(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, icon := range m.layout.Icons {
		if icon.ID == id {
			m.layout.Icons = append(m.layout.Icons[:i], m.layout.Icons[i+1:]...)
			m.layout.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("icon not found: %s", id)
}

// CreateGroup creates a new group with the specified icons.
func (m *Manager) CreateGroup(name string, iconIDs []string) (*DesktopGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	group := &DesktopGroup{
		ID:        fmt.Sprintf("group_%d", time.Now().UnixNano()),
		Name:      name,
		Collapsed: false,
		Icons:     make([]DesktopIcon, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	for _, id := range iconIDs {
		for i, icon := range m.layout.Icons {
			if icon.ID == id {
				m.layout.Icons[i].GroupID = group.ID
				group.Icons = append(group.Icons, m.layout.Icons[i])
				break
			}
		}
	}

	if len(group.Icons) > 0 {
		// Position group at average position of icons
		totalX, totalY := 0, 0
		for _, icon := range group.Icons {
			totalX += icon.X
			totalY += icon.Y
		}
		group.X = totalX / len(group.Icons)
		group.Y = totalY / len(group.Icons)
		group.Width = 200
		group.Height = 150

		m.layout.Groups = append(m.layout.Groups, *group)
		m.layout.UpdatedAt = time.Now()
	}

	return group, nil
}

// ToggleGroup toggles the collapsed state of a group.
func (m *Manager) ToggleGroup(groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, group := range m.layout.Groups {
		if group.ID == groupID {
			m.layout.Groups[i].Collapsed = !m.layout.Groups[i].Collapsed
			m.layout.Groups[i].UpdatedAt = time.Now()
			m.layout.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("group not found: %s", groupID)
}

// SetWallpaper sets the desktop wallpaper.
func (m *Manager) SetWallpaper(wp Wallpaper) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.layout.Wallpaper = &wp
	m.layout.UpdatedAt = time.Now()
	return nil
}

// LockIcon locks an icon in place.
func (m *Manager) LockIcon(id string, locked bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, icon := range m.layout.Icons {
		if icon.ID == id {
			m.layout.Icons[i].Locked = locked
			m.layout.Icons[i].UpdatedAt = time.Now()
			m.layout.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("icon not found: %s", id)
}

// RegisterRoutes registers desktop manager HTTP routes.
func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/desktop/layout", m.handleGetLayout)
	mux.HandleFunc("/api/desktop/icon/move", m.handleMoveIcon)
	mux.HandleFunc("/api/desktop/icon/add", m.handleAddIcon)
	mux.HandleFunc("/api/desktop/icon/remove", m.handleRemoveIcon)
	mux.HandleFunc("/api/desktop/icon/lock", m.handleLockIcon)
	mux.HandleFunc("/api/desktop/group/create", m.handleCreateGroup)
	mux.HandleFunc("/api/desktop/group/toggle", m.handleToggleGroup)
	mux.HandleFunc("/api/desktop/wallpaper", m.handleSetWallpaper)
}

func (m *Manager) handleGetLayout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	layout := m.GetLayout()
	json.NewEncoder(w).Encode(layout)
}

func (m *Manager) handleMoveIcon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
		X  int    `json:"x"`
		Y  int    `json:"y"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := m.UpdateIconPosition(req.ID, req.X, req.Y); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (m *Manager) handleAddIcon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var icon DesktopIcon
	if err := json.NewDecoder(r.Body).Decode(&icon); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := m.AddIcon(icon); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (m *Manager) handleRemoveIcon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := m.RemoveIcon(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (m *Manager) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name    string   `json:"name"`
		IconIDs []string `json:"icon_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	group, err := m.CreateGroup(req.Name, req.IconIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(group)
}

func (m *Manager) handleToggleGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		GroupID string `json:"group_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := m.ToggleGroup(req.GroupID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (m *Manager) handleLockIcon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID     string `json:"id"`
		Locked bool   `json:"locked"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := m.LockIcon(req.ID, req.Locked); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (m *Manager) handleSetWallpaper(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var wp Wallpaper
	if err := json.NewDecoder(r.Body).Decode(&wp); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := m.SetWallpaper(wp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
