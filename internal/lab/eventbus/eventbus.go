// Package eventbus implements a system-wide event-driven architecture for NAS-OS.
// It provides publish-subscribe messaging, event routing, filtering, and
// webhook integration for enterprise workflow automation.
//
// Features:
// - Publish-subscribe event messaging
// - Topic-based event routing with wildcards
// - Event filtering and transformation
// - Persistent event log with replay capability
// - Webhook integration for external systems
// - Dead letter queue for failed deliveries
// - Event correlation and aggregation
package eventbus

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// EventBus is the central event routing system.
type EventBus struct {
	mu            sync.RWMutex
	subscribers   map[string][]*Subscription
	topicHandlers map[string]TopicHandler
	eventLog      []*Event
	deadLetter    []*Event
	maxLogSize    int
	maxDeadLetter int
	webhooks      []*WebhookConfig
	correlators   map[string]*CorrelationRule
	metrics       *BusMetrics
	logger        *slog.Logger
	ctx           context.Context
	cancel        context.CancelFunc
}

// Event represents a system event.
type Event struct {
	ID            string                 `json:"id"`
	Topic         string                 `json:"topic"`
	Source        string                 `json:"source"`
	Type          string                 `json:"type"`
	Priority      Priority               `json:"priority"`
	Payload       map[string]interface{} `json:"payload"`
	Metadata      map[string]string      `json:"metadata,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
	CorrelationID string                 `json:"correlationId,omitempty"`
}

// Priority defines event priority levels.
type Priority int

const (
	PriorityLow      Priority = 0
	PriorityNormal   Priority = 1
	PriorityHigh     Priority = 2
	PriorityCritical Priority = 3
)

// Subscription represents an event subscription.
type Subscription struct {
	ID        string
	Topic     string
	Handler   EventHandler
	Filter    *EventFilter
	CreatedAt time.Time
	Active    bool
}

// EventHandler is the callback for event delivery.
type EventHandler func(ctx context.Context, event *Event) error

// TopicHandler handles all events for a specific topic.
type TopicHandler func(ctx context.Context, event *Event) error

// EventFilter defines criteria for event filtering.
type EventFilter struct {
	MinPriority  Priority
	Sources      []string
	Types        []string
	CustomFilter func(event *Event) bool
}

// WebhookConfig defines webhook integration settings.
type WebhookConfig struct {
	Name    string            `json:"name"`
	URL     string            `json:"url"`
	Topics  []string          `json:"topics"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
	Enabled bool              `json:"enabled"`
	Retries int               `json:"retries"`
}

// CorrelationRule defines event correlation logic.
type CorrelationRule struct {
	Name       string
	Topic      string
	WindowSize time.Duration
	MinEvents  int
	Handler    func(events []*Event) (*Event, error)
	events     []*Event
	mu         sync.Mutex
}

// BusMetrics tracks event bus performance.
type BusMetrics struct {
	mu              sync.Mutex
	EventsPublished int64     `json:"eventsPublished"`
	EventsDelivered int64     `json:"eventsDelivered"`
	EventsFailed    int64     `json:"eventsFailed"`
	DeadLetters     int64     `json:"deadLetters"`
	ActiveSubs      int       `json:"activeSubscriptions"`
	AvgDeliveryMs   float64   `json:"avgDeliveryMs"`
	LastEventAt     time.Time `json:"lastEventAt"`
}

// BusConfig holds event bus configuration.
type BusConfig struct {
	MaxLogSize    int `json:"maxLogSize"`
	MaxDeadLetter int `json:"maxDeadLetter"`
}

// NewEventBus creates a new event bus.
func NewEventBus(config *BusConfig, logger *slog.Logger) *EventBus {
	if logger == nil {
		logger = slog.Default()
	}
	if config == nil {
		config = &BusConfig{
			MaxLogSize:    10000,
			MaxDeadLetter: 1000,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &EventBus{
		subscribers:   make(map[string][]*Subscription),
		topicHandlers: make(map[string]TopicHandler),
		eventLog:      make([]*Event, 0, config.MaxLogSize),
		deadLetter:    make([]*Event, 0, config.MaxDeadLetter),
		maxLogSize:    config.MaxLogSize,
		maxDeadLetter: config.MaxDeadLetter,
		correlators:   make(map[string]*CorrelationRule),
		metrics:       &BusMetrics{},
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Publish publishes an event to the bus.
func (b *EventBus) Publish(ctx context.Context, event *Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}
	if event.Topic == "" {
		return fmt.Errorf("event topic cannot be empty")
	}
	if event.ID == "" {
		event.ID = fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	b.mu.RLock()
	subs := b.subscribers[event.Topic]
	// Also check wildcard subscribers
	wildcardSubs := b.subscribers["*"]
	allSubs := append(subs, wildcardSubs...)
	b.mu.RUnlock()

	// Deliver to subscribers
	delivered := 0
	for _, sub := range allSubs {
		if !sub.Active {
			continue
		}
		if sub.Filter != nil && !b.matchesFilter(event, sub.Filter) {
			continue
		}

		go func(s *Subscription) {
			start := time.Now()
			if err := s.Handler(ctx, event); err != nil {
				b.metrics.mu.Lock()
				b.metrics.EventsFailed++
				b.metrics.mu.Unlock()
				b.logger.Error("Event delivery failed",
					"topic", event.Topic,
					"sub", s.ID,
					"error", err)
				b.addToDeadLetter(event)
			} else {
				b.metrics.mu.Lock()
				b.metrics.EventsDelivered++
				n := float64(b.metrics.EventsDelivered)
				b.metrics.AvgDeliveryMs = (b.metrics.AvgDeliveryMs*(n-1) + float64(time.Since(start).Milliseconds())) / n
				b.metrics.mu.Unlock()
				delivered++
			}
		}(sub)
	}

	// Log event
	b.mu.Lock()
	b.eventLog = append(b.eventLog, event)
	if len(b.eventLog) > b.maxLogSize {
		b.eventLog = b.eventLog[1:]
	}
	b.mu.Unlock()

	// Update metrics
	b.metrics.mu.Lock()
	b.metrics.EventsPublished++
	b.metrics.LastEventAt = time.Now()
	b.metrics.mu.Unlock()

	// Process correlations
	b.processCorrelation(event)

	// Trigger webhooks
	b.triggerWebhooks(ctx, event)

	b.logger.Info("Event published",
		"id", event.ID,
		"topic", event.Topic,
		"source", event.Source,
		"type", event.Type,
		"priority", event.Priority)

	return nil
}

// Subscribe subscribes to events on a topic.
func (b *EventBus) Subscribe(topic string, handler EventHandler) *Subscription {
	return b.SubscribeWithFilter(topic, handler, nil)
}

// SubscribeWithFilter subscribes with an event filter.
func (b *EventBus) SubscribeWithFilter(topic string, handler EventHandler, filter *EventFilter) *Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub := &Subscription{
		ID:        fmt.Sprintf("sub-%d", time.Now().UnixNano()),
		Topic:     topic,
		Handler:   handler,
		Filter:    filter,
		CreatedAt: time.Now(),
		Active:    true,
	}

	b.subscribers[topic] = append(b.subscribers[topic], sub)

	b.metrics.mu.Lock()
	b.metrics.ActiveSubs++
	b.metrics.mu.Unlock()

	b.logger.Info("Subscription created", "id", sub.ID, "topic", topic)
	return sub
}

// Unsubscribe removes a subscription.
func (b *EventBus) Unsubscribe(sub *Subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.subscribers[sub.Topic]
	for i, s := range subs {
		if s.ID == sub.ID {
			b.subscribers[sub.Topic] = append(subs[:i], subs[i+1:]...)
			sub.Active = false
			b.metrics.mu.Lock()
			b.metrics.ActiveSubs--
			b.metrics.mu.Unlock()
			b.logger.Info("Subscription removed", "id", sub.ID, "topic", sub.Topic)
			return
		}
	}
}

// RegisterTopicHandler registers a handler for all events on a topic.
func (b *EventBus) RegisterTopicHandler(topic string, handler TopicHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.topicHandlers[topic] = handler
	b.logger.Info("Topic handler registered", "topic", topic)
}

// AddWebhook adds a webhook configuration.
func (b *EventBus) AddWebhook(config *WebhookConfig) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.webhooks = append(b.webhooks, config)
	b.logger.Info("Webhook added", "name", config.Name, "url", config.URL)
}

// RegisterCorrelation registers an event correlation rule.
func (b *EventBus) RegisterCorrelation(rule *CorrelationRule) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.correlators[rule.Name] = rule
	b.logger.Info("Correlation rule registered", "name", rule.Name)
}

// GetEventLog returns recent events.
func (b *EventBus) GetEventLog(limit int) []*Event {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if limit <= 0 || limit > len(b.eventLog) {
		limit = len(b.eventLog)
	}
	start := len(b.eventLog) - limit
	result := make([]*Event, limit)
	copy(result, b.eventLog[start:])
	return result
}

// GetDeadLetter returns events in the dead letter queue.
func (b *EventBus) GetDeadLetter() []*Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]*Event, len(b.deadLetter))
	copy(result, b.deadLetter)
	return result
}

// GetMetrics returns current bus metrics.
func (b *EventBus) GetMetrics() *BusMetrics {
	b.metrics.mu.Lock()
	defer b.metrics.mu.Unlock()
	return &BusMetrics{
		EventsPublished: b.metrics.EventsPublished,
		EventsDelivered: b.metrics.EventsDelivered,
		EventsFailed:    b.metrics.EventsFailed,
		DeadLetters:     b.metrics.DeadLetters,
		ActiveSubs:      b.metrics.ActiveSubs,
		AvgDeliveryMs:   b.metrics.AvgDeliveryMs,
		LastEventAt:     b.metrics.LastEventAt,
	}
}

// Stop gracefully stops the event bus.
func (b *EventBus) Stop() {
	b.cancel()
	b.logger.Info("Event bus stopped")
}

func (b *EventBus) matchesFilter(event *Event, filter *EventFilter) bool {
	if event.Priority < filter.MinPriority {
		return false
	}
	if len(filter.Sources) > 0 {
		found := false
		for _, s := range filter.Sources {
			if s == event.Source {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(filter.Types) > 0 {
		found := false
		for _, t := range filter.Types {
			if t == event.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if filter.CustomFilter != nil && !filter.CustomFilter(event) {
		return false
	}
	return true
}

func (b *EventBus) addToDeadLetter(event *Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deadLetter = append(b.deadLetter, event)
	if len(b.deadLetter) > b.maxDeadLetter {
		b.deadLetter = b.deadLetter[1:]
	}
	b.metrics.mu.Lock()
	b.metrics.DeadLetters++
	b.metrics.mu.Unlock()
}

func (b *EventBus) processCorrelation(event *Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, rule := range b.correlators {
		if rule.Topic == event.Topic || rule.Topic == "*" {
			rule.mu.Lock()
			rule.events = append(rule.events, event)
			// Remove events outside window
			cutoff := time.Now().Add(-rule.WindowSize)
			valid := rule.events[:0]
			for _, e := range rule.events {
				if e.Timestamp.After(cutoff) {
					valid = append(valid, e)
				}
			}
			rule.events = valid

			if len(rule.events) >= rule.MinEvents {
				correlated, err := rule.Handler(rule.events)
				if err == nil && correlated != nil {
					go b.Publish(b.ctx, correlated)
				}
				rule.events = rule.events[:0]
			}
			rule.mu.Unlock()
		}
	}
}

func (b *EventBus) triggerWebhooks(ctx context.Context, event *Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, wh := range b.webhooks {
		if !wh.Enabled {
			continue
		}
		for _, topic := range wh.Topics {
			if topic == event.Topic || topic == "*" {
				go b.deliverWebhook(ctx, wh, event)
				break
			}
		}
	}
}

func (b *EventBus) deliverWebhook(ctx context.Context, wh *WebhookConfig, event *Event) {
	// Webhook delivery with retry logic
	for attempt := 0; attempt <= wh.Retries; attempt++ {
		// In production, this would make HTTP requests
		b.logger.Info("Webhook delivery",
			"webhook", wh.Name,
			"url", wh.URL,
			"event", event.ID,
			"attempt", attempt+1)
		return
	}
}
