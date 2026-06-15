package eventbus

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewEventBus(t *testing.T) {
	bus := NewEventBus(nil, nil)
	if bus == nil {
		t.Fatal("NewEventBus returned nil")
	}
	defer bus.Stop()
}

func TestPublishSubscribe(t *testing.T) {
	bus := NewEventBus(nil, nil)
	defer bus.Stop()

	var received int32
	bus.Subscribe("test.topic", func(ctx context.Context, event *Event) error {
		atomic.AddInt32(&received, 1)
		return nil
	})

	event := &Event{
		Topic:  "test.topic",
		Source: "test",
		Type:   "test.event",
		Payload: map[string]interface{}{
			"key": "value",
		},
	}

	bus.Publish(context.Background(), event)
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&received) != 1 {
		t.Errorf("expected 1 delivery, got %d", received)
	}

	metrics := bus.GetMetrics()
	if metrics.EventsPublished != 1 {
		t.Errorf("expected 1 published, got %d", metrics.EventsPublished)
	}
}

func TestWildcardSubscribe(t *testing.T) {
	bus := NewEventBus(nil, nil)
	defer bus.Stop()

	var received int32
	bus.Subscribe("*", func(ctx context.Context, event *Event) error {
		atomic.AddInt32(&received, 1)
		return nil
	})

	bus.Publish(context.Background(), &Event{Topic: "any.topic", Source: "test"})
	bus.Publish(context.Background(), &Event{Topic: "other.topic", Source: "test"})
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&received) != 2 {
		t.Errorf("expected 2 deliveries, got %d", received)
	}
}

func TestEventFilter(t *testing.T) {
	bus := NewEventBus(nil, nil)
	defer bus.Stop()

	var received int32
	bus.SubscribeWithFilter("filtered.topic", func(ctx context.Context, event *Event) error {
		atomic.AddInt32(&received, 1)
		return nil
	}, &EventFilter{
		MinPriority: PriorityHigh,
	})

	bus.Publish(context.Background(), &Event{
		Topic:    "filtered.topic",
		Source:   "test",
		Priority: PriorityLow,
	})
	bus.Publish(context.Background(), &Event{
		Topic:    "filtered.topic",
		Source:   "test",
		Priority: PriorityCritical,
	})
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&received) != 1 {
		t.Errorf("expected 1 delivery (filtered), got %d", received)
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := NewEventBus(nil, nil)
	defer bus.Stop()

	var received int32
	sub := bus.Subscribe("unsub.topic", func(ctx context.Context, event *Event) error {
		atomic.AddInt32(&received, 1)
		return nil
	})

	bus.Publish(context.Background(), &Event{Topic: "unsub.topic", Source: "test"})
	time.Sleep(50 * time.Millisecond)

	bus.Unsubscribe(sub)

	bus.Publish(context.Background(), &Event{Topic: "unsub.topic", Source: "test"})
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&received) != 1 {
		t.Errorf("expected 1 delivery after unsubscribe, got %d", received)
	}
}

func TestEventLog(t *testing.T) {
	bus := NewEventBus(&BusConfig{MaxLogSize: 5, MaxDeadLetter: 5}, nil)
	defer bus.Stop()

	for i := 0; i < 10; i++ {
		bus.Publish(context.Background(), &Event{
			Topic:  "log.test",
			Source: "test",
		})
	}

	log := bus.GetEventLog(10)
	if len(log) != 5 {
		t.Errorf("expected 5 log entries (max size), got %d", len(log))
	}
}

func TestDeadLetter(t *testing.T) {
	bus := NewEventBus(nil, nil)
	defer bus.Stop()

	bus.Subscribe("fail.topic", func(ctx context.Context, event *Event) error {
		return fmt.Errorf("delivery failed")
	})

	bus.Publish(context.Background(), &Event{Topic: "fail.topic", Source: "test"})
	time.Sleep(100 * time.Millisecond)

	dl := bus.GetDeadLetter()
	if len(dl) != 1 {
		t.Errorf("expected 1 dead letter, got %d", len(dl))
	}
}

func TestPriorityFiltering(t *testing.T) {
	bus := NewEventBus(nil, nil)
	defer bus.Stop()

	var received int32
	bus.SubscribeWithFilter("priority.test", func(ctx context.Context, event *Event) error {
		atomic.AddInt32(&received, 1)
		return nil
	}, &EventFilter{
		Sources: []string{"trusted-source"},
	})

	bus.Publish(context.Background(), &Event{
		Topic:  "priority.test",
		Source: "untrusted",
	})
	bus.Publish(context.Background(), &Event{
		Topic:  "priority.test",
		Source: "trusted-source",
	})
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&received) != 1 {
		t.Errorf("expected 1 delivery (source filtered), got %d", received)
	}
}

func TestWebhookConfig(t *testing.T) {
	bus := NewEventBus(nil, nil)
	defer bus.Stop()

	bus.AddWebhook(&WebhookConfig{
		Name:    "test-webhook",
		URL:     "https://example.com/webhook",
		Topics:  []string{"alert.*"},
		Method:  "POST",
		Enabled: true,
		Retries: 3,
	})

	// Verify webhook was added (no direct way to check, just ensure no panic)
	bus.Publish(context.Background(), &Event{
		Topic:  "alert.critical",
		Source: "test",
	})
}
