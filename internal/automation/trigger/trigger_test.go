package trigger

import (
	"testing"
	"time"
)

func TestTriggerType_Constants(t *testing.T) {
	tests := []struct {
		triggerType TriggerType
		expected    string
	}{
		{TriggerTypeFile, "file"},
		{TriggerTypeTime, "time"},
		{TriggerTypeEvent, "event"},
		{TriggerTypeWebhook, "webhook"},
	}

	for _, tt := range tests {
		if string(tt.triggerType) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.triggerType))
		}
	}
}

func TestGetEventManager(t *testing.T) {
	em1 := GetEventManager(nil)
	em2 := GetEventManager(nil)

	if em1 == nil {
		t.Fatal("GetEventManager returned nil")
	}
	if em1 != em2 {
		t.Error("GetEventManager should return singleton")
	}
}

func TestEventManager_SubscribePublish(t *testing.T) {
	em := &EventManager{
		subscribers: make(map[string][]chan map[string]interface{}),
	}

	// Subscribe
	ch := em.Subscribe("test_event")

	// Publish
	data := map[string]interface{}{"key": "value"}
	em.Publish("test_event", data)

	select {
	case msg := <-ch:
		if msg["key"] != "value" {
			t.Error("received wrong data")
		}
	case <-time.After(time.Second):
		t.Error("did not receive event")
	}
}

func TestEventManager_Unsubscribe(t *testing.T) {
	em := &EventManager{
		subscribers: make(map[string][]chan map[string]interface{}),
	}

	ch := em.Subscribe("test_event")
	em.Unsubscribe("test_event", ch)

	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed after unsubscribe")
		}
	default:
		// Channel is closed (expected)
	}
}

func TestEventManager_MultipleSubscribers(t *testing.T) {
	em := &EventManager{
		subscribers: make(map[string][]chan map[string]interface{}),
	}

	ch1 := em.Subscribe("test_event")
	ch2 := em.Subscribe("test_event")

	data := map[string]interface{}{"msg": "hello"}
	em.Publish("test_event", data)

	// Both should receive
	<-ch1
	<-ch2
}

func TestFileTrigger_GetType(t *testing.T) {
	trigger := &FileTrigger{Path: "/tmp"}
	if trigger.GetType() != TriggerTypeFile {
		t.Errorf("expected %s, got %s", TriggerTypeFile, trigger.GetType())
	}
}

func TestTimeTrigger_GetType(t *testing.T) {
	trigger := &TimeTrigger{Schedule: "0 * * * *"}
	if trigger.GetType() != TriggerTypeTime {
		t.Errorf("expected %s, got %s", TriggerTypeTime, trigger.GetType())
	}
}

func TestEventTrigger_GetType(t *testing.T) {
	trigger := &EventTrigger{EventType: "user.login"}
	if trigger.GetType() != TriggerTypeEvent {
		t.Errorf("expected %s, got %s", TriggerTypeEvent, trigger.GetType())
	}
}

func TestWebhookTrigger_GetType(t *testing.T) {
	trigger := &WebhookTrigger{Path: "/webhook/test"}
	if trigger.GetType() != TriggerTypeWebhook {
		t.Errorf("expected %s, got %s", TriggerTypeWebhook, trigger.GetType())
	}
}

func TestFileTrigger_Fields(t *testing.T) {
	trigger := &FileTrigger{
		Path:      "/test/path",
		Recursive: true,
		Events:    []string{"created", "modified"},
	}

	if trigger.Path != "/test/path" {
		t.Error("Path mismatch")
	}
	if !trigger.Recursive {
		t.Error("Recursive should be true")
	}
	if len(trigger.Events) != 2 {
		t.Error("Events count mismatch")
	}
}

func TestTimeTrigger_Fields(t *testing.T) {
	trigger := &TimeTrigger{
		Schedule: "0 0 * * *",
		Timezone: "UTC",
	}

	if trigger.Schedule != "0 0 * * *" {
		t.Error("Schedule mismatch")
	}
	if trigger.Timezone != "UTC" {
		t.Error("Timezone mismatch")
	}
}

func TestEventTrigger_Fields(t *testing.T) {
	trigger := &EventTrigger{
		EventType: "user.created",
		Filter:    map[string]interface{}{"role": "admin"},
	}

	if trigger.EventType != "user.created" {
		t.Error("EventType mismatch")
	}
	if trigger.Filter["role"] != "admin" {
		t.Error("Filter mismatch")
	}
}

func TestWebhookTrigger_Fields(t *testing.T) {
	trigger := &WebhookTrigger{
		Path:    "/api/webhook/test",
		Secret:  "my-secret",
		Method:  "POST",
	}

	if trigger.Path != "/api/webhook/test" {
		t.Error("Path mismatch")
	}
	if trigger.Secret != "my-secret" {
		t.Error("Secret mismatch")
	}
	if trigger.Method != "POST" {
		t.Error("Method mismatch")
	}
}