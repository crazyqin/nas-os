package themeengine

import (
	"fmt"
	"sync"
	"time"
)

// DarkModeType represents the dark mode switching strategy.
type DarkModeType string

const (
	DarkModeManual DarkModeType = "manual"
	DarkModeAuto   DarkModeType = "auto"
)

// DarkModeConfig holds dark mode configuration.
type DarkModeConfig struct {
	Type      DarkModeType `json:"type"`
	Enabled   bool         `json:"enabled"`
	StartTime string       `json:"start_time,omitempty"` // HH:MM format for auto mode
	EndTime   string       `json:"end_time,omitempty"`   // HH:MM format for auto mode
}

// DarkModeManager handles dark mode switching.
type DarkModeManager struct {
	mu     sync.RWMutex
	config DarkModeConfig
	themes *ThemeManager
}

// NewDarkModeManager creates a new dark mode manager.
func NewDarkModeManager(themes *ThemeManager) *DarkModeManager {
	return &DarkModeManager{
		config: DarkModeConfig{
			Type:    DarkModeManual,
			Enabled: false,
		},
		themes: themes,
	}
}

// Enable enables dark mode.
func (dm *DarkModeManager) Enable() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.config.Enabled = true
	return dm.applyDarkTheme()
}

// Disable disables dark mode.
func (dm *DarkModeManager) Disable() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.config.Enabled = false
	return dm.applyLightTheme()
}

// Toggle toggles dark mode on/off.
func (dm *DarkModeManager) Toggle() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.config.Enabled = !dm.config.Enabled

	if dm.config.Enabled {
		return dm.applyDarkTheme()
	}
	return dm.applyLightTheme()
}

// SetMode sets the dark mode switching strategy.
func (dm *DarkModeManager) SetMode(mode DarkModeType) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if mode != DarkModeManual && mode != DarkModeAuto {
		return fmt.Errorf("invalid dark mode type: %s", mode)
	}

	dm.config.Type = mode
	return nil
}

// SetAutoSchedule sets the automatic dark mode schedule.
func (dm *DarkModeManager) SetAutoSchedule(startTime, endTime string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if _, err := time.Parse("15:04", startTime); err != nil {
		return fmt.Errorf("invalid start time format: %s", startTime)
	}
	if _, err := time.Parse("15:04", endTime); err != nil {
		return fmt.Errorf("invalid end time format: %s", endTime)
	}

	dm.config.Type = DarkModeAuto
	dm.config.StartTime = startTime
	dm.config.EndTime = endTime
	return nil
}

// GetConfig returns the current dark mode configuration.
func (dm *DarkModeManager) GetConfig() DarkModeConfig {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.config
}

// IsEnabled returns whether dark mode is currently enabled.
func (dm *DarkModeManager) IsEnabled() bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.config.Enabled
}

// ShouldUseDarkMode checks if dark mode should be active based on the current time.
func (dm *DarkModeManager) ShouldUseDarkMode() bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if dm.config.Type == DarkModeManual {
		return dm.config.Enabled
	}

	// Auto mode: check time range
	now := time.Now()
	currentTime := now.Format("15:04")

	start := dm.config.StartTime
	end := dm.config.EndTime

	if start <= end {
		return currentTime >= start && currentTime < end
	}
	// Overnight range (e.g., 22:00 - 06:00)
	return currentTime >= start || currentTime < end
}

// UpdateFromSystem checks system time and applies appropriate theme.
func (dm *DarkModeManager) UpdateFromSystem() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.config.Type != DarkModeAuto {
		return nil
	}

	shouldDark := dm.ShouldUseDarkMode()
	if shouldDark && !dm.config.Enabled {
		dm.config.Enabled = true
		return dm.applyDarkTheme()
	} else if !shouldDark && dm.config.Enabled {
		dm.config.Enabled = false
		return dm.applyLightTheme()
	}

	return nil
}

// applyDarkTheme applies the dark theme (caller must hold lock).
func (dm *DarkModeManager) applyDarkTheme() error {
	return dm.themes.Switch("default-dark")
}

// applyLightTheme applies the light theme (caller must hold lock).
func (dm *DarkModeManager) applyLightTheme() error {
	return dm.themes.Switch("default-light")
}
