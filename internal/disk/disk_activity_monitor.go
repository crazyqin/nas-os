// Package disk provides disk activity monitoring for power management.
package disk

import (
	"sync"
	"time"
)

// ActivityEvent represents disk activity event.
type ActivityEvent struct {
	DiskID    string    `json:"disk_id"`
	Type      string    `json:"type"` // read, write, seek
	Timestamp time.Time `json:"timestamp"`
	Size      int64     `json:"size"` // bytes transferred
}

// ActivityMonitor monitors disk I/O activity.
type ActivityMonitor struct {
	mu      sync.RWMutex
	events  map[string][]ActivityEvent // diskID -> recent events
	maxSize int                        // max events per disk
}

// NewActivityMonitor creates a new activity monitor.
func NewActivityMonitor() *ActivityMonitor {
	return &ActivityMonitor{
		events:  make(map[string][]ActivityEvent),
		maxSize: 100,
	}
}

// RecordEvent records a disk activity event.
func (am *ActivityMonitor) RecordEvent(diskID string, eventType string, size int64) {
	am.mu.Lock()
	defer am.mu.Unlock()

	event := ActivityEvent{
		DiskID:    diskID,
		Type:      eventType,
		Timestamp: time.Now(),
		Size:      size,
	}

	events := am.events[diskID]
	events = append(events, event)

	// Trim to max size
	if len(events) > am.maxSize {
		events = events[len(events)-am.maxSize:]
	}

	am.events[diskID] = events
}

// GetRecentActivity returns recent activity for a disk.
func (am *ActivityMonitor) GetRecentActivity(diskID string, since time.Time) []ActivityEvent {
	am.mu.RLock()
	defer am.mu.RUnlock()

	events := am.events[diskID]
	result := make([]ActivityEvent, 0)

	for _, event := range events {
		if event.Timestamp.After(since) {
			result = append(result, event)
		}
	}

	return result
}

// GetActivityStats returns activity statistics for a disk.
func (am *ActivityMonitor) GetActivityStats(diskID string, since time.Time) *ActivityStats {
	am.mu.RLock()
	defer am.mu.RUnlock()

	events := am.events[diskID]
	stats := &ActivityStats{
		DiskID:       diskID,
		ReadCount:    0,
		WriteCount:   0,
		ReadBytes:    0,
		WriteBytes:   0,
		LastActivity: time.Time{},
	}

	for _, event := range events {
		if event.Timestamp.Before(since) {
			continue
		}

		if event.Timestamp.After(stats.LastActivity) {
			stats.LastActivity = event.Timestamp
		}

		switch event.Type {
		case "read":
			stats.ReadCount++
			stats.ReadBytes += event.Size
		case "write":
			stats.WriteCount++
			stats.WriteBytes += event.Size
		}
	}

	return stats
}

// HasRecentActivity checks if disk has recent activity.
func (am *ActivityMonitor) HasRecentActivity(diskID string, threshold time.Duration) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()

	events := am.events[diskID]
	if len(events) == 0 {
		return false
	}

	lastEvent := events[len(events)-1]
	return time.Since(lastEvent.Timestamp) < threshold
}

// ClearEvents clears all events for a disk.
func (am *ActivityMonitor) ClearEvents(diskID string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.events[diskID] = nil
}

// ActivityStats represents disk activity statistics.
type ActivityStats struct {
	DiskID       string    `json:"disk_id"`
	ReadCount    int       `json:"read_count"`
	WriteCount   int       `json:"write_count"`
	ReadBytes    int64     `json:"read_bytes"`
	WriteBytes   int64     `json:"write_bytes"`
	LastActivity time.Time `json:"last_activity"`
}
