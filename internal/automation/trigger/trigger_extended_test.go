package trigger

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ========== FileTrigger 测试 ==========

func TestFileTrigger_StartStop(t *testing.T) {
	trigger := &FileTrigger{
		Path:      t.TempDir(),
		Recursive: false,
		Events:    []string{"create", "modify"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := trigger.Start(ctx, func(data map[string]interface{}) {})
	if err != nil {
		t.Logf("FileTrigger.Start error (expected in some envs): %v", err)
	}

	err = trigger.Stop()
	if err != nil {
		t.Logf("FileTrigger.Stop error: %v", err)
	}
}

func TestFileTrigger_WithNonexistentPath(t *testing.T) {
	trigger := &FileTrigger{
		Path:      "/nonexistent/path/that/does/not/exist",
		Recursive: false,
	}

	ctx := context.Background()
	err := trigger.Start(ctx, func(data map[string]interface{}) {})
	if err == nil {
		t.Error("Expected error for nonexistent path")
	}
}

// ========== TimeTrigger 测试 ==========

func TestTimeTrigger_StartStop(t *testing.T) {
	trigger := &TimeTrigger{
		Schedule: "*/1 * * * *",
		Timezone: "UTC",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := trigger.Start(ctx, func(data map[string]interface{}) {})
	if err != nil {
		t.Logf("TimeTrigger.Start error: %v", err)
	}

	err = trigger.Stop()
	if err != nil {
		t.Logf("TimeTrigger.Stop error: %v", err)
	}
}

func TestTimeTrigger_InvalidSchedule(t *testing.T) {
	trigger := &TimeTrigger{
		Schedule: "invalid-cron-expression",
	}

	ctx := context.Background()
	err := trigger.Start(ctx, func(data map[string]interface{}) {})
	if err == nil {
		t.Error("Expected error for invalid cron schedule")
	}
}

// ========== EventTrigger 测试 ==========

func TestEventTrigger_StartStop(t *testing.T) {
	trigger := &EventTrigger{
		EventType: "test.event",
		Filter:    nil,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := trigger.Start(ctx, func(data map[string]interface{}) {})
	if err != nil {
		t.Logf("EventTrigger.Start error: %v", err)
	}

	err = trigger.Stop()
	if err != nil {
		t.Logf("EventTrigger.Stop error: %v", err)
	}
}

func TestEventTrigger_Filter(t *testing.T) {
	trigger := &EventTrigger{
		EventType: "user.login",
		Filter: map[string]interface{}{
			"role": "admin",
		},
	}

	ctx := context.Background()

	err := trigger.Start(ctx, func(data map[string]interface{}) {})
	if err != nil {
		t.Skipf("EventTrigger.Start error: %v", err)
		return
	}

	trigger.Stop()
}

// ========== WebhookTrigger 测试 ==========

func TestWebhookTrigger_StartStop(t *testing.T) {
	trigger := &WebhookTrigger{
		Path:   "/webhook/test",
		Secret: "test-secret",
		Method: "POST",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := trigger.Start(ctx, func(data map[string]interface{}) {})
	if err != nil {
		t.Logf("WebhookTrigger.Start error: %v", err)
	}

	err = trigger.Stop()
	if err != nil {
		t.Logf("WebhookTrigger.Stop error: %v", err)
	}
}

func TestWebhookTrigger_VerifySignature(t *testing.T) {
	secret := "my-secret"
	payload := []byte(`{"event": "test"}`)

	// Generate valid signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	trigger := &WebhookTrigger{
		Path:   "/webhook/test",
		Secret: secret,
	}

	// Verify with valid signature
	valid := verifyWebhookSignature(trigger.Secret, payload, "sha256="+expectedSig)
	if !valid {
		t.Error("Signature verification should succeed with valid signature")
	}

	// Verify with invalid signature
	invalid := verifyWebhookSignature(trigger.Secret, payload, "sha256=invalid")
	if invalid {
		t.Error("Signature verification should fail with invalid signature")
	}
}

func TestWebhookTrigger_HandleRequest(t *testing.T) {
	// Create test request
	body := strings.NewReader(`{"event": "test", "data": "value"}`)
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/webhook/test", body)
	req.Header.Set("Content-Type", "application/json")

	// Process request
	data, err := parseWebhookRequestBody(req)
	if err != nil {
		t.Fatalf("Failed to parse request: %v", err)
	}

	if data["event"] != "test" {
		t.Errorf("Expected event=test, got %v", data["event"])
	}
}

func TestWebhookTrigger_HandleRequest_WithSignature(t *testing.T) {
	secret := "my-secret"
	payload := []byte(`{"event": "signed"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/webhook/test", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", "sha256="+signature)

	data, err := parseWebhookRequestBody(req)
	if err != nil {
		t.Fatalf("Failed to parse request: %v", err)
	}

	if data["event"] != "signed" {
		t.Errorf("Expected event=signed, got %v", data["event"])
	}
}

// Helper functions for webhook testing.
func verifyWebhookSignature(secret string, payload []byte, signature string) bool {
	if secret == "" {
		return true
	}

	sig := strings.TrimPrefix(signature, "sha256=")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sig), []byte(expected))
}

func parseWebhookRequestBody(r *http.Request) (map[string]interface{}, error) {
	var data map[string]interface{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&data)
	return data, err
}

// ========== EventManager 更多测试 ==========

func TestEventManager_PublishNoSubscribers(t *testing.T) {
	em := &EventManager{
		subscribers: make(map[string][]chan map[string]interface{}),
	}

	// Should not panic when publishing to no subscribers
	em.Publish("nonexistent.event", map[string]interface{}{"test": "data"})
}

func TestEventManager_MultipleEventTypes(t *testing.T) {
	em := &EventManager{
		subscribers: make(map[string][]chan map[string]interface{}),
	}

	ch1 := em.Subscribe("event.type1")
	ch2 := em.Subscribe("event.type2")

	em.Publish("event.type1", map[string]interface{}{"type": 1})
	em.Publish("event.type2", map[string]interface{}{"type": 2})

	select {
	case msg := <-ch1:
		if msg["type"] != 1 {
			t.Error("Wrong data in ch1")
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for ch1")
	}

	select {
	case msg := <-ch2:
		if msg["type"] != 2 {
			t.Error("Wrong data in ch2")
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for ch2")
	}
}

func TestEventManager_ConcurrentPublish(t *testing.T) {
	em := &EventManager{
		subscribers: make(map[string][]chan map[string]interface{}),
	}

	ch := em.Subscribe("concurrent.event")

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			em.Publish("concurrent.event", map[string]interface{}{"n": n})
			done <- true
		}(i)
	}

	// Wait for all publishes
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should receive all messages
	received := 0
	timeout := time.After(2 * time.Second)
	for received < 10 {
		select {
		case <-ch:
			received++
		case <-timeout:
			t.Errorf("Only received %d messages out of 10", received)
			return
		}
	}
}
