package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// HotReloader manages hot reloading of plugins
type HotReloader struct {
	pluginDir  string
	manager    *Manager
	watchers   map[string]*PluginWatcher
	checksums  map[string]string
	interval   time.Duration
	stopCh     chan struct{}
	mu         sync.RWMutex
	notifyFunc func(pluginID string, event HotReloadEvent)
}

// PluginWatcher tracks plugin file changes
type PluginWatcher struct {
	PluginID   string
	Path       string
	LastMod    time.Time
	Checksum   string
	CheckCount int
}

// HotReloadEvent represents a hot reload event
type HotReloadEvent struct {
	Type      HotReloadEventType `json:"type"`
	PluginID  string             `json:"pluginId"`
	Timestamp int64              `json:"timestamp"`
	Message   string             `json:"message"`
	Error     string             `json:"error,omitempty"`
}

// HotReloadEventType defines event types
type HotReloadEventType string

const (
	EventPluginLoaded     HotReloadEventType = "loaded"
	EventPluginUnloaded   HotReloadEventType = "unloaded"
	EventPluginReloaded   HotReloadEventType = "reloaded"
	EventPluginError      HotReloadEventType = "error"
	EventPluginDiscovered HotReloadEventType = "discovered"
)

// HotReloadConfig holds hot reloader configuration
type HotReloadConfig struct {
	PluginDir     string
	CheckInterval time.Duration
	NotifyFunc    func(pluginID string, event HotReloadEvent)
}

// NewHotReloader creates a new hot reloader
func NewHotReloader(cfg HotReloadConfig, manager *Manager) *HotReloader {
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = 5 * time.Second
	}

	return &HotReloader{
		pluginDir:  cfg.PluginDir,
		manager:    manager,
		watchers:   make(map[string]*PluginWatcher),
		checksums:  make(map[string]string),
		interval:   cfg.CheckInterval,
		stopCh:     make(chan struct{}),
		notifyFunc: cfg.NotifyFunc,
	}
}

// Start begins watching for plugin changes
func (hr *HotReloader) Start() {
	go hr.watch()
	log.Printf("Plugin hot reloader started, watching: %s", hr.pluginDir)
}

// Stop stops the hot reloader
func (hr *HotReloader) Stop() {
	close(hr.stopCh)
	log.Println("Plugin hot reloader stopped")
}

// watch continuously checks for plugin changes
func (hr *HotReloader) watch() {
	ticker := time.NewTicker(hr.interval)
	defer ticker.Stop()

	// Initial scan
	hr.scanPlugins()

	for {
		select {
		case <-hr.stopCh:
			return
		case <-ticker.C:
			hr.checkForChanges()
		}
	}
}

// scanPlugins performs initial scan of plugin directory
func (hr *HotReloader) scanPlugins() {
	entries, err := os.ReadDir(hr.pluginDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Error scanning plugin directory: %v", err)
		}
		return
	}

	for _, entry := range entries {
		path := filepath.Join(hr.pluginDir, entry.Name())
		hr.registerPlugin(path, entry)
	}
}

// registerPlugin registers a plugin for watching
func (hr *HotReloader) registerPlugin(path string, entry os.DirEntry) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	var pluginID string
	var isPlugin bool

	if entry.IsDir() {
		// Directory plugin
		manifestPath := filepath.Join(path, "manifest.json")
		if _, err := os.Stat(manifestPath); err == nil {
			pluginID = entry.Name()
			isPlugin = true
		}
	} else if strings.HasSuffix(entry.Name(), ".so") {
		// .so file plugin
		pluginID = strings.TrimSuffix(entry.Name(), ".so")
		isPlugin = true
	}

	if !isPlugin {
		return
	}

	info, err := entry.Info()
	if err != nil {
		return
	}

	checksum := hr.calculateChecksum(path)

	hr.watchers[pluginID] = &PluginWatcher{
		PluginID: pluginID,
		Path:     path,
		LastMod:  info.ModTime(),
		Checksum: checksum,
	}

	hr.checksums[pluginID] = checksum

	// Notify discovery
	hr.notify(pluginID, HotReloadEvent{
		Type:      EventPluginDiscovered,
		PluginID:  pluginID,
		Timestamp: time.Now().Unix(),
		Message:   fmt.Sprintf("Plugin discovered: %s", pluginID),
	})
}

// checkForChanges checks for plugin modifications
func (hr *HotReloader) checkForChanges() {
	entries, err := os.ReadDir(hr.pluginDir)
	if err != nil {
		return
	}

	hr.mu.Lock()
	defer hr.mu.Unlock()

	// Check for new or modified plugins
	foundPlugins := make(map[string]bool)

	for _, entry := range entries {
		path := filepath.Join(hr.pluginDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		var pluginID string
		var isPlugin bool

		if entry.IsDir() {
			manifestPath := filepath.Join(path, "manifest.json")
			if _, err := os.Stat(manifestPath); err == nil {
				pluginID = entry.Name()
				isPlugin = true
			}
		} else if strings.HasSuffix(entry.Name(), ".so") {
			pluginID = strings.TrimSuffix(entry.Name(), ".so")
			isPlugin = true
		}

		if !isPlugin {
			continue
		}

		foundPlugins[pluginID] = true

		watcher, exists := hr.watchers[pluginID]
		if !exists {
			// New plugin
			hr.registerPlugin(path, entry)
			continue
		}

		// Check for modifications
		if info.ModTime().After(watcher.LastMod) {
			newChecksum := hr.calculateChecksum(path)
			if newChecksum != watcher.Checksum {
				// Plugin changed, trigger reload
				go hr.reloadPlugin(pluginID, path)
				watcher.LastMod = info.ModTime()
				watcher.Checksum = newChecksum
				hr.checksums[pluginID] = newChecksum
			}
		}
	}

	// Check for removed plugins
	for pluginID := range hr.watchers {
		if !foundPlugins[pluginID] {
			// Plugin removed
			go hr.unloadPlugin(pluginID)
			delete(hr.watchers, pluginID)
			delete(hr.checksums, pluginID)
		}
	}
}

// reloadPlugin reloads a plugin
func (hr *HotReloader) reloadPlugin(pluginID, path string) {
	log.Printf("Reloading plugin: %s", pluginID)

	// Notify reload start
	hr.notify(pluginID, HotReloadEvent{
		Type:      EventPluginReloaded,
		PluginID:  pluginID,
		Timestamp: time.Now().Unix(),
		Message:   fmt.Sprintf("Reloading plugin: %s", pluginID),
	})

	// Check if plugin is currently loaded
	if state, err := hr.manager.Get(pluginID); err == nil && state.Enabled {
		// Disable first
		if err := hr.manager.Disable(pluginID); err != nil {
			hr.notify(pluginID, HotReloadEvent{
				Type:      EventPluginError,
				PluginID:  pluginID,
				Timestamp: time.Now().Unix(),
				Message:   fmt.Sprintf("Failed to disable plugin for reload: %s", pluginID),
				Error:     err.Error(),
			})
			return
		}
	}

	// Unload from loader
	if hr.manager.loader != nil {
		hr.manager.loader.Unload(pluginID)
	}

	// Re-install/reload
	state, err := hr.manager.Install(path)
	if err != nil {
		hr.notify(pluginID, HotReloadEvent{
			Type:      EventPluginError,
			PluginID:  pluginID,
			Timestamp: time.Now().Unix(),
			Message:   fmt.Sprintf("Failed to reload plugin: %s", pluginID),
			Error:     err.Error(),
		})
		return
	}

	// Re-enable if it was enabled before
	if state != nil {
		if err := hr.manager.Enable(pluginID); err != nil {
			hr.notify(pluginID, HotReloadEvent{
				Type:      EventPluginError,
				PluginID:  pluginID,
				Timestamp: time.Now().Unix(),
				Message:   fmt.Sprintf("Failed to re-enable plugin: %s", pluginID),
				Error:     err.Error(),
			})
			return
		}
	}

	hr.notify(pluginID, HotReloadEvent{
		Type:      EventPluginLoaded,
		PluginID:  pluginID,
		Timestamp: time.Now().Unix(),
		Message:   fmt.Sprintf("Plugin reloaded successfully: %s", pluginID),
	})
}

// unloadPlugin unloads a removed plugin
func (hr *HotReloader) unloadPlugin(pluginID string) {
	log.Printf("Unloading removed plugin: %s", pluginID)

	// Uninstall
	if err := hr.manager.Uninstall(pluginID); err != nil {
		log.Printf("Error unloading plugin %s: %v", pluginID, err)
	}

	hr.notify(pluginID, HotReloadEvent{
		Type:      EventPluginUnloaded,
		PluginID:  pluginID,
		Timestamp: time.Now().Unix(),
		Message:   fmt.Sprintf("Plugin unloaded: %s", pluginID),
	})
}

// calculateChecksum calculates checksum for a plugin
func (hr *HotReloader) calculateChecksum(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}

	if info.IsDir() {
		// Calculate checksum for directory contents
		hash := sha256.New()
		filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				f, err := os.Open(p)
				if err != nil {
					return nil
				}
				defer f.Close()
				io.Copy(hash, f)
			}
			return nil
		})
		return hex.EncodeToString(hash.Sum(nil))[:16]
	}

	// Calculate checksum for single file
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	hash := sha256.New()
	io.Copy(hash, f)
	return hex.EncodeToString(hash.Sum(nil))[:16]
}

// notify sends a notification
func (hr *HotReloader) notify(pluginID string, event HotReloadEvent) {
	if hr.notifyFunc != nil {
		hr.notifyFunc(pluginID, event)
	}
}

// GetWatchers returns all plugin watchers
func (hr *HotReloader) GetWatchers() map[string]*PluginWatcher {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	result := make(map[string]*PluginWatcher)
	for k, v := range hr.watchers {
		result[k] = v
	}
	return result
}

// ForceReload forces a reload of a specific plugin
func (hr *HotReloader) ForceReload(pluginID string) error {
	hr.mu.RLock()
	watcher, exists := hr.watchers[pluginID]
	hr.mu.RUnlock()

	if !exists {
		return fmt.Errorf("plugin %s not found in watchers", pluginID)
	}

	hr.reloadPlugin(pluginID, watcher.Path)
	return nil
}

// HotReloadStatus represents the status of hot reloading
type HotReloadStatus struct {
	Running       bool                     `json:"running"`
	CheckInterval string                   `json:"checkInterval"`
	Watchers      map[string]WatcherStatus `json:"watchers"`
}

// WatcherStatus represents status of a plugin watcher
type WatcherStatus struct {
	PluginID    string `json:"pluginId"`
	Path        string `json:"path"`
	LastMod     int64  `json:"lastMod"`
	Checksum    string `json:"checksum"`
	ReloadCount int    `json:"reloadCount"`
}

// GetStatus returns hot reloader status
func (hr *HotReloader) GetStatus() HotReloadStatus {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	watchers := make(map[string]WatcherStatus)
	for id, w := range hr.watchers {
		watchers[id] = WatcherStatus{
			PluginID: w.PluginID,
			Path:     w.Path,
			LastMod:  w.LastMod.Unix(),
			Checksum: w.Checksum,
		}
	}

	return HotReloadStatus{
		Running:       true,
		CheckInterval: hr.interval.String(),
		Watchers:      watchers,
	}
}

// MarshalJSON implements json.Marshaler for HotReloadEvent
func (e HotReloadEvent) MarshalJSON() ([]byte, error) {
	type Alias HotReloadEvent
	return json.Marshal(&struct {
		Alias
	}{
		Alias: Alias(e),
	})
}
