package apimigrator

import (
	"testing"
	"time"
)

func TestAnalyzeMigrateDeprecatedEndpoints(t *testing.T) {
	s := Signal{
		LegacyAPIEnabled:       true,
		APIDeprecatedEndpoints: 5,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "migrate-deprecated-endpoints" {
			if r.Priority != "critical" {
				t.Errorf("expected priority critical, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected migrate-deprecated-endpoints recommendation")
	}
}

func TestAnalyzeEnableWebSocketAPI(t *testing.T) {
	s := Signal{
		WebSocketAPIEnabled: false,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "enable-websocket-api" {
			if r.Priority != "high" {
				t.Errorf("expected priority high, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected enable-websocket-api recommendation")
	}
}

func TestAnalyzeUpgradeToSCRAM(t *testing.T) {
	s := Signal{
		SCRAMEnabled:     false,
		APIKeyAuthMethod: "basic",
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "upgrade-to-scram" {
			if r.Priority != "high" {
				t.Errorf("expected priority high, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected upgrade-to-scram recommendation")
	}
}

func TestAnalyzeUpgradeToSCRAMNotTriggeredWithSCRAM(t *testing.T) {
	s := Signal{
		SCRAMEnabled:     true,
		APIKeyAuthMethod: "scram-sha-512",
	}
	recs := Analyze(s)
	for _, r := range recs {
		if r.ID == "upgrade-to-scram" {
			t.Error("upgrade-to-scram should not be triggered when SCRAM is already enabled")
		}
	}
}

func TestAnalyzeUpgradeToSCRAMNotTriggeredWithNonBasic(t *testing.T) {
	s := Signal{
		SCRAMEnabled:     false,
		APIKeyAuthMethod: "bearer",
	}
	recs := Analyze(s)
	for _, r := range recs {
		if r.ID == "upgrade-to-scram" {
			t.Error("upgrade-to-scram should not be triggered when auth method is not basic")
		}
	}
}

func TestAnalyzeMigrateLegacyClients(t *testing.T) {
	s := Signal{
		ClientsUsingLegacyAPI: 3,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "migrate-legacy-clients" {
			if r.Priority != "high" {
				t.Errorf("expected priority high, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected migrate-legacy-clients recommendation")
	}
}

func TestAnalyzeRotateAPIKeys(t *testing.T) {
	s := Signal{
		TokenRotationAge: 91 * 24 * time.Hour,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "rotate-api-keys" {
			if r.Priority != "medium" {
				t.Errorf("expected priority medium, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected rotate-api-keys recommendation")
	}
}

func TestAnalyzeRotateAPIKeysNotTriggered(t *testing.T) {
	s := Signal{
		TokenRotationAge: 89 * 24 * time.Hour,
	}
	recs := Analyze(s)
	for _, r := range recs {
		if r.ID == "rotate-api-keys" {
			t.Error("rotate-api-keys should not be triggered when rotation age is under 90 days")
		}
	}
}

func TestAnalyzeEnableRateLimiting(t *testing.T) {
	s := Signal{
		RateLimitEnabled: false,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "enable-rate-limiting" {
			if r.Priority != "medium" {
				t.Errorf("expected priority medium, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected enable-rate-limiting recommendation")
	}
}

func TestAnalyzeEnableOpenAPIDocs(t *testing.T) {
	s := Signal{
		OpenAPIEnabled: false,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "enable-openapi-docs" {
			if r.Priority != "medium" {
				t.Errorf("expected priority medium, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected enable-openapi-docs recommendation")
	}
}

func TestAnalyzeEnableWebhooks(t *testing.T) {
	s := Signal{
		WebhookSupport: false,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "enable-webhooks" {
			if r.Priority != "low" {
				t.Errorf("expected priority low, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected enable-webhooks recommendation")
	}
}

func TestAnalyzeEnableAuditLogging(t *testing.T) {
	s := Signal{
		AuditLogEnabled: false,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "enable-audit-logging" {
			if r.Priority != "medium" {
				t.Errorf("expected priority medium, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected enable-audit-logging recommendation")
	}
}

func TestAnalyzeUpdateAPIDocs(t *testing.T) {
	s := Signal{
		APIDocumentationUpdated: false,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "update-api-docs" {
			if r.Priority != "low" {
				t.Errorf("expected priority low, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected update-api-docs recommendation")
	}
}

func TestAnalyzeEmptySignal(t *testing.T) {
	s := Signal{}
	recs := Analyze(s)

	// An empty signal should trigger all the "false/zero" based
	// recommendations: WebSocket, rate limiting, OpenAPI, webhooks,
	// audit logging, and API documentation.
	expected := map[string]bool{
		"enable-websocket-api":    false,
		"enable-rate-limiting":     false,
		"enable-openapi-docs":      false,
		"enable-webhooks":          false,
		"enable-audit-logging":     false,
		"update-api-docs":          false,
	}

	for _, r := range recs {
		if _, ok := expected[r.ID]; ok {
			expected[r.ID] = true
		}
	}

	for id, found := range expected {
		if !found {
			t.Errorf("expected recommendation %s to be present for empty signal", id)
		}
	}

	// Verify that positive conditions are NOT triggered
	notExpected := []string{
		"migrate-deprecated-endpoints",
		"upgrade-to-scram",
		"migrate-legacy-clients",
		"rotate-api-keys",
	}

	for _, id := range notExpected {
		for _, r := range recs {
			if r.ID == id {
				t.Errorf("recommendation %s should not be present for empty signal", id)
			}
		}
	}
}

func TestAnalyzeFullyModernizedSignal(t *testing.T) {
	s := Signal{
		LegacyAPIEnabled:        false,
		WebSocketAPIEnabled:      true,
		APIKeyAuthMethod:        "scram-sha-512",
		SCRAMEnabled:            true,
		APIDeprecatedEndpoints:  0,
		APIVersion:              "2.0",
		ClientsUsingLegacyAPI:  0,
		TokenRotationAge:        30 * 24 * time.Hour,
		RateLimitEnabled:        true,
		APIDocumentationUpdated: true,
		OpenAPIEnabled:          true,
		WebhookSupport:          true,
		AuditLogEnabled:         true,
	}
	recs := Analyze(s)
	if len(recs) != 0 {
		t.Errorf("expected 0 recommendations for fully modernized signal, got %d", len(recs))
	}
}

func TestAnalyzePriorityOrdering(t *testing.T) {
	s := Signal{
		LegacyAPIEnabled:        true,
		APIDeprecatedEndpoints:  2,
		WebSocketAPIEnabled:     false,
		SCRAMEnabled:            false,
		APIKeyAuthMethod:        "basic",
		ClientsUsingLegacyAPI:  5,
		TokenRotationAge:        100 * 24 * time.Hour,
		RateLimitEnabled:        false,
		APIDocumentationUpdated: false,
		OpenAPIEnabled:          false,
		WebhookSupport:          false,
		AuditLogEnabled:         false,
	}
	recs := Analyze(s)

	// All recommendations should be present
	if len(recs) != 10 {
		t.Fatalf("expected 10 recommendations, got %d", len(recs))
	}

	// Verify sorted by priority rank
	for i := 1; i < len(recs); i++ {
		rankPrev := priorityRank(recs[i-1].Priority)
		rankCurr := priorityRank(recs[i].Priority)
		if rankPrev > rankCurr {
			t.Errorf("recommendations not sorted: %s (rank %d) before %s (rank %d)",
				recs[i-1].ID, rankPrev, recs[i].ID, rankCurr)
		}
	}
}

func TestPriorityRank(t *testing.T) {
	tests := []struct {
		priority string
		expected int
	}{
		{"critical", 0},
		{"high", 1},
		{"medium", 2},
		{"low", 3},
		{"unknown", 4},
	}
	for _, tt := range tests {
		if got := priorityRank(tt.priority); got != tt.expected {
			t.Errorf("priorityRank(%q) = %d, want %d", tt.priority, got, tt.expected)
		}
	}
}