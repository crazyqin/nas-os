// Package filewatcher 提供实时文件变更监控
package filewatcher

import (
	"fmt"
	"sync"
	"time"
)

// Manager 文件监控管理器.
type Manager struct {
	mu       sync.RWMutex
	watchers map[string]*Watcher
	events   map[string][]*FileEvent
	stats    WatcherStats
}

// NewManager 创建文件监控管理器.
func NewManager() *Manager {
	return &Manager{
		watchers: make(map[string]*Watcher),
		events:   make(map[string][]*FileEvent),
	}
}

// CreateWatcher 创建监控器.
func (m *Manager) CreateWatcher(req CreateWatcherRequest) (*Watcher, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	events := req.Events
	if len(events) == 0 {
		events = []EventType{EventCreate, EventModify, EventDelete}
	}

	id := fmt.Sprintf("fw-%d", time.Now().UnixNano())
	watcher := &Watcher{
		ID:        id,
		Name:      req.Name,
		Paths:     req.Paths,
		Events:    events,
		Patterns:  req.Patterns,
		Recursive: req.Recursive,
		Status:    WatcherStatusActive,
		Webhook:   req.Webhook,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.watchers[id] = watcher
	m.stats.TotalWatchers++
	m.stats.ActiveWatchers++

	return watcher, nil
}

// GetWatcher 获取监控器.
func (m *Manager) GetWatcher(id string) (*Watcher, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	w, ok := m.watchers[id]
	if !ok {
		return nil, ErrWatcherNotFound
	}
	return w, nil
}

// ListWatchers 列出所有监控器.
func (m *Manager) ListWatchers() []*Watcher {
	m.mu.RLock()
	defer m.mu.RUnlock()

	watchers := make([]*Watcher, 0, len(m.watchers))
	for _, w := range m.watchers {
		watchers = append(watchers, w)
	}
	return watchers
}

// DeleteWatcher 删除监控器.
func (m *Manager) DeleteWatcher(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.watchers[id]; !ok {
		return ErrWatcherNotFound
	}

	delete(m.watchers, id)
	m.stats.TotalWatchers--
	m.stats.ActiveWatchers--
	return nil
}

// RecordEvent 记录文件事件.
func (m *Manager) RecordEvent(watcherID string, eventType EventType, path string) *FileEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	event := &FileEvent{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		WatcherID: watcherID,
		Type:      eventType,
		Path:      path,
		Timestamp: time.Now(),
	}

	m.events[watcherID] = append(m.events[watcherID], event)
	m.stats.TotalEvents++
	m.stats.EventsToday++

	return event
}

// GetEvents 获取监控器事件.
func (m *Manager) GetEvents(watcherID string, limit int) []*FileEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := m.events[watcherID]
	if limit > 0 && limit < len(events) {
		return events[len(events)-limit:]
	}
	return events
}

// GetStats 获取统计.
func (m *Manager) GetStats() WatcherStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}
