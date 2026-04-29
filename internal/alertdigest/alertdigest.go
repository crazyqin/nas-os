// Package alertdigest provides batched notification digest delivery.
// Instead of sending individual alerts, it collects alerts over a configurable
// interval and sends a consolidated digest. Inspired by enterprise notification
// best practices from Synology Active Insight and TrueNAS alert systems.
package alertdigest

import (
	"fmt"
	"sync"
	"time"
)

// Severity levels for alerts.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

// Alert represents a single alert event.
type Alert struct {
	ID        string    `json:"id"`
	Source    string    `json:"source"`    // e.g., "disk", "network", "backup"
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Severity  Severity  `json:"severity"`
	CreatedAt time.Time `json:"created_at"`
	Acked     bool      `json:"acked"`
}

// Digest represents a batch of alerts ready for delivery.
type Digest struct {
	ID         string    `json:"id"`
	Period     string    `json:"period"` // e.g., "hourly", "daily"
	Alerts     []*Alert  `json:"alerts"`
	Critical   int       `json:"critical_count"`
	Warning    int       `json:"warning_count"`
	Info       int       `json:"info_count"`
	CreatedAt  time.Time `json:"created_at"`
	Delivered  bool      `json:"delivered"`
}

// DigestConfig defines digest delivery preferences per channel.
type DigestConfig struct {
	Channel   string        `json:"channel"`   // e.g., "email", "telegram", "webhook"
	Interval  time.Duration `json:"interval"`  // How often to send digests
	MinCount  int           `json:"min_count"` // Minimum alerts before sending (0 = always)
	Severities []Severity   `json:"severities"` // Only include these severities
}

// Collector gathers alerts and produces digests.
type Collector struct {
	mu         sync.RWMutex
	pending    []*Alert
	digests    []*Digest
	configs    map[string]*DigestConfig
	nextID     int
	digestID   int
}

// NewCollector creates a new alert digest collector.
func NewCollector() *Collector {
	return &Collector{
		pending: make([]*Alert, 0),
		digests: make([]*Digest, 0),
		configs: make(map[string]*DigestConfig),
	}
}

// AddAlert adds an alert to the pending queue.
func (c *Collector) AddAlert(source, title, message string, severity Severity) *Alert {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nextID++
	alert := &Alert{
		ID:        fmt.Sprintf("alert_%d", c.nextID),
		Source:    source,
		Title:     title,
		Message:   message,
		Severity:  severity,
		CreatedAt: time.Now(),
	}
	c.pending = append(c.pending, alert)
	return alert
}

// AckAlert acknowledges an alert by ID.
func (c *Collector) AckAlert(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, a := range c.pending {
		if a.ID == id {
			a.Acked = true
			return true
		}
	}
	return false
}

// ConfigureDigest sets delivery preferences for a channel.
func (c *Collector) ConfigureDigest(channel string, config DigestConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.configs[channel] = &config
}

// FlushPending collects all pending alerts into a digest.
func (c *Collector) FlushPending(period string) *Digest {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.pending) == 0 {
		return nil
	}

	c.digestID++
	digest := &Digest{
		ID:        fmt.Sprintf("digest_%d", c.digestID),
		Period:    period,
		Alerts:    c.pending,
		CreatedAt: time.Now(),
	}

	for _, a := range c.pending {
		switch a.Severity {
		case SeverityCritical:
			digest.Critical++
		case SeverityWarning:
			digest.Warning++
		case SeverityInfo:
			digest.Info++
		}
	}

	c.digests = append(c.digests, digest)
	c.pending = make([]*Alert, 0) // Reset pending

	return digest
}

// GetPending returns the current pending alerts.
func (c *Collector) GetPending() []*Alert {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*Alert, len(c.pending))
	copy(result, c.pending)
	return result
}

// GetDigests returns all produced digests.
func (c *Collector) GetDigests() []*Digest {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*Digest, len(c.digests))
	copy(result, c.digests)
	return result
}

// MarkDelivered marks a digest as delivered.
func (c *Collector) MarkDelivered(digestID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, d := range c.digests {
		if d.ID == digestID {
			d.Delivered = true
			return true
		}
	}
	return false
}

// PendingCount returns the number of pending alerts.
func (c *Collector) PendingCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.pending)
}

// Summary returns a human-readable summary of pending alerts.
func (c *Collector) Summary() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.pending) == 0 {
		return "No pending alerts"
	}

	var crit, warn, info int
	for _, a := range c.pending {
		switch a.Severity {
		case SeverityCritical:
			crit++
		case SeverityWarning:
			warn++
		case SeverityInfo:
			info++
		}
	}

	return fmt.Sprintf("%d alerts: %d critical, %d warning, %d info",
		len(c.pending), crit, warn, info)
}
