package smartnotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestPriorityString(t *testing.T) {
	tests := []struct {
		p    Priority
		want string
	}{
		{PriorityLow, "low"},
		{PriorityMedium, "medium"},
		{PriorityHigh, "high"},
		{PriorityCritical, "critical"},
		{Priority(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.p.String(); got != tt.want {
			t.Errorf("Priority(%d).String() = %q, want %q", tt.p, got, tt.want)
		}
	}
}

func TestParsePriority(t *testing.T) {
	tests := []struct {
		s    string
		want Priority
	}{
		{"low", PriorityLow},
		{"medium", PriorityMedium},
		{"high", PriorityHigh},
		{"critical", PriorityCritical},
		{"unknown", PriorityMedium},
		{"", PriorityMedium},
	}
	for _, tt := range tests {
		if got := ParsePriority(tt.s); got != tt.want {
			t.Errorf("ParsePriority(%q) = %d, want %d", tt.s, got, tt.want)
		}
	}
}

func TestMatchSource(t *testing.T) {
	tests := []struct {
		source  string
		pattern string
		want    bool
	}{
		{"disk-monitor", "disk-monitor", true},
		{"disk-monitor", "cpu-*", false},
		{"disk-monitor", "*-monitor", true},
		{"disk-monitor", "disk*", true},
		{"disk-monitor", "*", true},
		{"disk-monitor", "*isk*", true},
		{"app-server", "*-server", true},
		{"app-server", "app-*", true},
		{"app-server", "web-*", false},
	}
	for _, tt := range tests {
		got := matchSource(tt.source, tt.pattern)
		if got != tt.want {
			t.Errorf("matchSource(%q, %q) = %v, want %v", tt.source, tt.pattern, got, tt.want)
		}
	}
}

func TestRouterAddRemoveGetRule(t *testing.T) {
	router := NewRouter()

	rule := &RoutingRule{
		ID:       "test-rule-1",
		Name:     "Test Rule",
		Priority: []Priority{PriorityHigh, PriorityCritical},
		Channels: []Channel{ChannelEmail},
		Enabled:  true,
	}

	router.AddRule(rule)

	got, ok := router.GetRule("test-rule-1")
	if !ok {
		t.Fatal("expected rule to exist")
	}
	if got.Name != "Test Rule" {
		t.Errorf("rule name = %q, want %q", got.Name, "Test Rule")
	}

	rules := router.ListRules()
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}

	router.RemoveRule("test-rule-1")
	_, ok = router.GetRule("test-rule-1")
	if ok {
		t.Error("expected rule to be removed")
	}
}

func TestRouterUserPreference(t *testing.T) {
	router := NewRouter()

	pref := &UserPreference{
		UserID:      "user1",
		Channels:    []Channel{ChannelEmail, ChannelTelegram},
		MinPriority: PriorityMedium,
		QuietHours: &QuietHours{
			Enabled:  true,
			Start:    "22:00",
			End:      "08:00",
			Timezone: "UTC",
		},
	}

	router.SetUserPreference(pref)

	got, ok := router.GetUserPreference("user1")
	if !ok {
		t.Fatal("expected preference to exist")
	}
	if got.UserID != "user1" {
		t.Errorf("user id = %q, want %q", got.UserID, "user1")
	}
	if len(got.Channels) != 2 {
		t.Errorf("channels count = %d, want 2", len(got.Channels))
	}

	router.DeleteUserPreference("user1")
	_, ok = router.GetUserPreference("user1")
	if ok {
		t.Error("expected preference to be deleted")
	}
}

func TestRouterSendAndDeliver(t *testing.T) {
	var mu sync.Mutex
	var delivered []*Notification

	router := NewRouter(
		WithDeliveryFunc(func(ch Channel, recipient string, notif *Notification) error {
			mu.Lock()
			defer mu.Unlock()
			delivered = append(delivered, notif)
			return nil
		}),
		WithAggregationConfig(&AggregationConfig{Enabled: false}),
	)

	router.AddRule(&RoutingRule{
		ID:         "default",
		Name:       "Default Rule",
		Priority:   []Priority{PriorityLow, PriorityMedium, PriorityHigh, PriorityCritical},
		SourceMatch: []string{"*"},
		Channels:   []Channel{ChannelEmail},
		Recipients: []string{"admin@example.com"},
		Enabled:    true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	router.Start(ctx)

	notif := &Notification{
		ID:       "test-notif-1",
		Title:    "Test Alert",
		Content:  "Something happened",
		Priority: PriorityHigh,
		Source:   "test-module",
	}

	if err := router.Send(notif); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Wait for delivery
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := len(delivered)
	mu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 delivery, got %d", count)
	}

	// Check history
	history := router.History(10)
	if len(history) < 1 {
		t.Errorf("expected at least 1 history entry, got %d", len(history))
	}
}

func TestRouterRetryOnFailure(t *testing.T) {
	var mu sync.Mutex
	attempts := 0

	router := NewRouter(
		WithDeliveryFunc(func(ch Channel, recipient string, notif *Notification) error {
			mu.Lock()
			attempts++
			current := attempts
			mu.Unlock()
			if current < 3 {
				return fmt.Errorf("delivery failed (attempt %d)", current)
			}
			return nil
		}),
		WithRetryConfig(&RetryConfig{
			MaxRetries:  3,
			InitialWait: 10 * time.Millisecond,
			MaxWait:     50 * time.Millisecond,
			Multiplier:  2.0,
		}),
		WithAggregationConfig(&AggregationConfig{Enabled: false}),
	)

	router.AddRule(&RoutingRule{
		ID:         "default",
		Name:       "Default",
		Priority:   []Priority{PriorityCritical},
		SourceMatch: []string{"*"},
		Channels:   []Channel{ChannelWebhook},
		Recipients: []string{"http://example.com/hook"},
		Enabled:    true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	router.Start(ctx)

	notif := &Notification{
		ID:       "retry-test",
		Title:    "Retry Test",
		Content:  "Should retry",
		Priority: PriorityCritical,
		Source:   "test",
	}

	_ = router.Send(notif)

	// Wait for retries to complete
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	finalAttempts := attempts
	mu.Unlock()

	if finalAttempts < 3 {
		t.Errorf("expected at least 3 attempts, got %d", finalAttempts)
	}
}

func TestRouterAggregation(t *testing.T) {
	var mu sync.Mutex
	var deliveredCount int

	router := NewRouter(
		WithDeliveryFunc(func(ch Channel, recipient string, notif *Notification) error {
			mu.Lock()
			defer mu.Unlock()
			deliveredCount++
			return nil
		}),
		WithAggregationConfig(&AggregationConfig{
			Enabled:  true,
			Window:   200 * time.Millisecond,
			MaxCount: 5,
			GroupBy:  []string{"source"},
		}),
	)

	router.AddRule(&RoutingRule{
		ID:         "default",
		Name:       "Default",
		Priority:   []Priority{PriorityLow, PriorityMedium, PriorityHigh},
		SourceMatch: []string{"*"},
		Channels:   []Channel{ChannelEmail},
		Recipients: []string{"admin@test.com"},
		Enabled:    true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	router.Start(ctx)

	// Send 3 notifications from same source (below MaxCount, so they aggregate)
	for i := 0; i < 3; i++ {
		notif := &Notification{
			ID:       fmt.Sprintf("agg-test-%d", i),
			Title:    fmt.Sprintf("Alert %d", i),
			Content:  fmt.Sprintf("Content %d", i),
			Priority: PriorityMedium,
			Source:   "disk-monitor",
		}
		_ = router.Send(notif)
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for aggregation window to flush
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	count := deliveredCount
	mu.Unlock()

	// Should deliver 1 aggregated notification (not 3 individual)
	if count != 1 {
		t.Errorf("expected 1 aggregated delivery, got %d", count)
	}
}

func TestRouterAggregationMaxCountFlush(t *testing.T) {
	var mu sync.Mutex
	var deliveredCount int

	router := NewRouter(
		WithDeliveryFunc(func(ch Channel, recipient string, notif *Notification) error {
			mu.Lock()
			defer mu.Unlock()
			deliveredCount++
			return nil
		}),
		WithAggregationConfig(&AggregationConfig{
			Enabled:  true,
			Window:   10 * time.Second, // long window so it only flushes by count
			MaxCount: 3,
			GroupBy:  []string{},
		}),
	)

	router.AddRule(&RoutingRule{
		ID:         "default",
		Name:       "Default",
		Priority:   []Priority{PriorityMedium},
		SourceMatch: []string{"*"},
		Channels:   []Channel{ChannelDiscord},
		Recipients: []string{"#alerts"},
		Enabled:    true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	router.Start(ctx)

	// Send exactly MaxCount notifications to trigger immediate flush
	for i := 0; i < 3; i++ {
		_ = router.Send(&Notification{
			ID:       fmt.Sprintf("max-test-%d", i),
			Title:    "Alert",
			Content:  "Content",
			Priority: PriorityMedium,
			Source:   "cpu-monitor",
		})
		time.Sleep(20 * time.Millisecond)
	}

	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	count := deliveredCount
	mu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 aggregated delivery on max count flush, got %d", count)
	}
}

func TestRouterNoMatchingRule(t *testing.T) {
	router := NewRouter(
		WithAggregationConfig(&AggregationConfig{Enabled: false}),
	)

	router.AddRule(&RoutingRule{
		ID:         "high-only",
		Name:       "High Only",
		Priority:   []Priority{PriorityCritical},
		SourceMatch: []string{"*"},
		Channels:   []Channel{ChannelEmail},
		Recipients: []string{"admin@test.com"},
		Enabled:    true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	router.Start(ctx)

	_ = router.Send(&Notification{
		ID:       "no-match",
		Title:    "Low Priority",
		Content:  "Should not route",
		Priority: PriorityLow,
		Source:   "test",
	})

	time.Sleep(100 * time.Millisecond)

	history := router.History(10)
	// No delivery should be recorded
	if len(history) != 0 {
		t.Errorf("expected no history entries, got %d", len(history))
	}
}

func TestRouterDisabledRule(t *testing.T) {
	router := NewRouter(
		WithAggregationConfig(&AggregationConfig{Enabled: false}),
	)

	router.AddRule(&RoutingRule{
		ID:         "disabled",
		Name:       "Disabled",
		Priority:   []Priority{PriorityHigh},
		SourceMatch: []string{"*"},
		Channels:   []Channel{ChannelEmail},
		Recipients: []string{"admin@test.com"},
		Enabled:    false, // disabled
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	router.Start(ctx)

	_ = router.Send(&Notification{
		ID:       "disabled-test",
		Title:    "Test",
		Content:  "Content",
		Priority: PriorityHigh,
		Source:   "test",
	})

	time.Sleep(100 * time.Millisecond)

	history := router.History(10)
	if len(history) != 0 {
		t.Errorf("expected no deliveries for disabled rule, got %d", len(history))
	}
}

func TestParseHHMM(t *testing.T) {
	tests := []struct {
		input string
		want  int
		err   bool
	}{
		{"00:00", 0, false},
		{"12:30", 750, false},
		{"23:59", 1439, false},
		{"25:00", 0, true},
		{"12:60", 0, true},
		{"abc", 0, true},
	}
	for _, tt := range tests {
		got, err := parseHHMM(tt.input)
		if (err != nil) != tt.err {
			t.Errorf("parseHHMM(%q) error = %v, wantErr %v", tt.input, err, tt.err)
			continue
		}
		if !tt.err && got != tt.want {
			t.Errorf("parseHHMM(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// --- HTTP Handler Tests ---

func TestHandleSend(t *testing.T) {
	router := NewRouter(
		WithDeliveryFunc(func(ch Channel, recipient string, notif *Notification) error {
			return nil
		}),
		WithAggregationConfig(&AggregationConfig{Enabled: false}),
	)
	router.AddRule(&RoutingRule{
		ID:         "default",
		Name:       "Default",
		Priority:   []Priority{PriorityHigh},
		SourceMatch: []string{"*"},
		Channels:   []Channel{ChannelEmail},
		Recipients: []string{"admin@test.com"},
		Enabled:    true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	router.Start(ctx)

	handler := NewHandler(router)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body := SendRequest{
		Title:    "Test Alert",
		Content:  "Something happened",
		Priority: "high",
		Source:   "test-module",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/notifications/send", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success=true, got error: %s", resp.Error)
	}
}

func TestHandleSendMissingFields(t *testing.T) {
	router := NewRouter()
	handler := NewHandler(router)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body := SendRequest{Title: "", Content: ""}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/notifications/send", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleRulesCRUD(t *testing.T) {
	router := NewRouter()
	handler := NewHandler(router)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create rule
	rule := RoutingRule{
		ID:       "rule-1",
		Name:     "Test Rule",
		Priority: []Priority{PriorityHigh},
		Channels: []Channel{ChannelTelegram},
		Enabled:  true,
	}
	bodyBytes, _ := json.Marshal(rule)

	req := httptest.NewRequest("POST", "/api/rules", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("create: expected 201, got %d", w.Code)
	}

	// List rules
	req = httptest.NewRequest("GET", "/api/rules", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("list: expected 200, got %d", w.Code)
	}

	var listResp APIResponse
	_ = json.NewDecoder(w.Body).Decode(&listResp)

	// Get rule
	req = httptest.NewRequest("GET", "/api/rules/rule-1", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("get: expected 200, got %d", w.Code)
	}

	// Delete rule
	req = httptest.NewRequest("DELETE", "/api/rules/rule-1", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("delete: expected 200, got %d", w.Code)
	}

	// Get deleted rule
	req = httptest.NewRequest("GET", "/api/rules/rule-1", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("get deleted: expected 404, got %d", w.Code)
	}
}

func TestHandlePreferencesCRUD(t *testing.T) {
	router := NewRouter()
	handler := NewHandler(router)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Set preference
	pref := UserPreference{
		Channels:    []Channel{ChannelEmail, ChannelTelegram},
		MinPriority: PriorityHigh,
		QuietHours: &QuietHours{
			Enabled:  true,
			Start:    "22:00",
			End:      "08:00",
			Timezone: "UTC",
		},
	}
	bodyBytes, _ := json.Marshal(pref)

	req := httptest.NewRequest("PUT", "/api/preferences/user1", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("set: expected 200, got %d", w.Code)
	}

	// Get preference
	req = httptest.NewRequest("GET", "/api/preferences/user1", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("get: expected 200, got %d", w.Code)
	}

	var getResp APIResponse
	_ = json.NewDecoder(w.Body).Decode(&getResp)

	// Delete preference
	req = httptest.NewRequest("DELETE", "/api/preferences/user1", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("delete: expected 200, got %d", w.Code)
	}

	// Get deleted preference
	req = httptest.NewRequest("GET", "/api/preferences/user1", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("get deleted: expected 404, got %d", w.Code)
	}
}

func TestHandleHistory(t *testing.T) {
	router := NewRouter(
		WithDeliveryFunc(func(ch Channel, recipient string, notif *Notification) error {
			return nil
		}),
		WithAggregationConfig(&AggregationConfig{Enabled: false}),
	)
	router.AddRule(&RoutingRule{
		ID:         "default",
		Name:       "Default",
		Priority:   []Priority{PriorityHigh},
		SourceMatch: []string{"*"},
		Channels:   []Channel{ChannelEmail},
		Recipients: []string{"admin@test.com"},
		Enabled:    true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	router.Start(ctx)

	handler := NewHandler(router)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Send a notification first
	body := SendRequest{
		Title:    "History Test",
		Content:  "Content",
		Priority: "high",
		Source:   "test",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/notifications/send", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	time.Sleep(100 * time.Millisecond)

	// Get history
	req = httptest.NewRequest("GET", "/api/notifications/history?limit=10", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp APIResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Errorf("expected success=true")
	}
}

func TestTimeWindowMatch(t *testing.T) {
	router := NewRouter()

	// Test with current time window using UTC
	now := time.Now().UTC()
	tw := &TimeWindow{
		Start:    fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute()),
		End:      fmt.Sprintf("%02d:%02d", (now.Hour()+1)%24, now.Minute()),
		Timezone: "UTC",
	}

	if !router.inTimeWindow(tw) {
		t.Error("expected current time to be within window")
	}

	// Test with past time window
	pastTw := &TimeWindow{
		Start:    "01:00",
		End:      "02:00",
		Timezone: "UTC",
	}
	// This may or may not match depending on current time, just ensure no panic
	_ = router.inTimeWindow(pastTw)
}

func TestDisabledRuleSkipped(t *testing.T) {
	var mu sync.Mutex
	var deliveredCount int

	router := NewRouter(
		WithDeliveryFunc(func(ch Channel, recipient string, notif *Notification) error {
			mu.Lock()
			defer mu.Unlock()
			deliveredCount++
			return nil
		}),
		WithAggregationConfig(&AggregationConfig{Enabled: false}),
	)

	// Add disabled rule
	router.AddRule(&RoutingRule{
		ID:         "disabled",
		Name:       "Disabled",
		Priority:   []Priority{PriorityHigh},
		SourceMatch: []string{"*"},
		Channels:   []Channel{ChannelEmail},
		Recipients: []string{"admin@test.com"},
		Enabled:    false,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	router.Start(ctx)

	_ = router.Send(&Notification{
		ID:       "disabled-test",
		Title:    "Test",
		Content:  "Content",
		Priority: PriorityHigh,
		Source:   "test",
	})

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := deliveredCount
	mu.Unlock()

	if count != 0 {
		t.Errorf("expected 0 deliveries for disabled rule, got %d", count)
	}
}

func TestSourcePatternMatching(t *testing.T) {
	tests := []struct {
		name    string
		ruleSrc []string
		notifSrc string
		want    bool
	}{
		{"exact match", []string{"disk-monitor"}, "disk-monitor", true},
		{"wildcard all", []string{"*"}, "anything", true},
		{"suffix match", []string{"*-monitor"}, "cpu-monitor", true},
		{"prefix match", []string{"disk*"}, "disk-alert", true},
		{"contains match", []string{"*isk*"}, "disk-check", true},
		{"no match", []string{"cpu-*"}, "disk-monitor", false},
		{"multiple patterns", []string{"cpu-*", "disk-*"}, "disk-alert", true},
		{"multiple no match", []string{"cpu-*", "mem-*"}, "disk-alert", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &RoutingRule{
				ID:         "test",
				SourceMatch: tt.ruleSrc,
				Priority:   []Priority{PriorityHigh},
				Enabled:    true,
			}
			notif := &Notification{
				Source:   tt.notifSrc,
				Priority: PriorityHigh,
			}
			router := NewRouter()
			got := router.matchRule(rule, notif)
			if got != tt.want {
				t.Errorf("matchRule() = %v, want %v", got, tt.want)
			}
		})
	}
}
