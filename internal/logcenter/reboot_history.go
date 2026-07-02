package logcenter

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// RebootReason describes why a node rebooted.
type RebootReason string

const (
	// RebootReasonUnknown is used when the source cannot classify the event.
	RebootReasonUnknown RebootReason = "unknown"
	// RebootReasonUserInitiated indicates an administrator requested the reboot.
	RebootReasonUserInitiated RebootReason = "user_initiated"
	// RebootReasonKernelPanic indicates a crash or watchdog restart.
	RebootReasonKernelPanic RebootReason = "kernel_panic"
	// RebootReasonScheduledUpdate indicates a planned update reboot.
	RebootReasonScheduledUpdate RebootReason = "scheduled_update"
	// RebootReasonPowerLoss indicates power interruption or brownout.
	RebootReasonPowerLoss RebootReason = "power_loss"
)

// RebootEvent is an auditable reboot history record.
type RebootEvent struct {
	ID        string       `json:"id"`
	Node      string       `json:"node"`
	Reason    RebootReason `json:"reason"`
	Source    string       `json:"source"`
	Details   string       `json:"details,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
}

// RebootHistory stores reboot events in timestamp order.
type RebootHistory struct {
	mu     sync.RWMutex
	events []RebootEvent
	limit  int
}

// NewRebootHistory creates a reboot history store.
func NewRebootHistory(limit int) *RebootHistory {
	if limit <= 0 {
		limit = 256
	}
	return &RebootHistory{limit: limit}
}

// Add records a reboot event and keeps the newest events.
func (h *RebootHistory) Add(event RebootEvent) RebootEvent {
	h.mu.Lock()
	defer h.mu.Unlock()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.Reason == "" {
		event.Reason = ClassifyRebootReason(event.Details)
	}
	if event.ID == "" {
		event.ID = event.Timestamp.Format("20060102T150405.000000000") + "-" + event.Node
	}

	h.events = append(h.events, event)
	sort.Slice(h.events, func(i, j int) bool {
		return h.events[i].Timestamp.Before(h.events[j].Timestamp)
	})
	if len(h.events) > h.limit {
		h.events = h.events[len(h.events)-h.limit:]
	}
	return event
}

// List returns reboot history newest first.
func (h *RebootHistory) List(limit int) []RebootEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if limit <= 0 || limit > len(h.events) {
		limit = len(h.events)
	}
	out := make([]RebootEvent, 0, limit)
	for i := len(h.events) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, h.events[i])
	}
	return out
}

// ClassifyRebootReason maps raw journal/IPMI/update text to a stable reason.
func ClassifyRebootReason(details string) RebootReason {
	text := strings.ToLower(details)
	switch {
	case strings.Contains(text, "panic") || strings.Contains(text, "watchdog") || strings.Contains(text, "oops"):
		return RebootReasonKernelPanic
	case strings.Contains(text, "apt") || strings.Contains(text, "upgrade") || strings.Contains(text, "scheduled") || strings.Contains(text, "maintenance"):
		return RebootReasonScheduledUpdate
	case strings.Contains(text, "power") || strings.Contains(text, "ups") || strings.Contains(text, "brownout"):
		return RebootReasonPowerLoss
	case strings.Contains(text, "admin") || strings.Contains(text, "user") || strings.Contains(text, "reboot command"):
		return RebootReasonUserInitiated
	default:
		return RebootReasonUnknown
	}
}
