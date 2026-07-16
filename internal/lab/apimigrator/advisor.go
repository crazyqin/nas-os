package apimigrator

import (
	"sort"
	"time"
)

// Signal represents the current state of API configuration and usage.
type Signal struct {
	LegacyAPIEnabled         bool
	WebSocketAPIEnabled      bool
	APIKeyAuthMethod         string
	SCRAMEnabled             bool
	APIDeprecatedEndpoints   int
	APIVersion               string
	ClientsUsingLegacyAPI   int
	TokenRotationAge        time.Duration
	RateLimitEnabled         bool
	APIDocumentationUpdated  bool
	OpenAPIEnabled           bool
	WebhookSupport           bool
	AuditLogEnabled          bool
}

// Recommendation is a single actionable suggestion produced by the advisor.
type Recommendation struct {
	ID       string
	Title    string
	Priority string
	Action   string
	Reason   string
}

// priorityRank maps priority labels to a numeric rank for sorting.
// Lower rank means higher urgency.
func priorityRank(priority string) int {
	switch priority {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}

// Analyze inspects the given Signal and returns a sorted list of
// Recommendations for API modernization.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	// 1. Legacy API enabled with deprecated endpoints
	if s.LegacyAPIEnabled && s.APIDeprecatedEndpoints > 0 {
		recs = append(recs, Recommendation{
			ID:       "migrate-deprecated-endpoints",
			Title:    "Migrate Deprecated API Endpoints",
			Priority: "critical",
			Action:   "Identify all deprecated endpoints in use and migrate them to the current WebSocket JSON-RPC 2.0 API surface.",
			Reason:   "Legacy API is enabled with deprecated endpoints still present, posing security and maintainability risks.",
		})
	}

	// 2. WebSocket API not enabled
	if !s.WebSocketAPIEnabled {
		recs = append(recs, Recommendation{
			ID:       "enable-websocket-api",
			Title:    "Enable WebSocket API",
			Priority: "high",
			Action:   "Enable the WebSocket JSON-RPC 2.0 API for real-time, bidirectional communication.",
			Reason:   "WebSocket API is not enabled; modern clients benefit from persistent connections and reduced overhead.",
		})
	}

	// 3. Basic auth without SCRAM
	if !s.SCRAMEnabled && s.APIKeyAuthMethod == "basic" {
		recs = append(recs, Recommendation{
			ID:       "upgrade-to-scram",
			Title:    "Upgrade to SCRAM-SHA-512 Authentication",
			Priority: "high",
			Action:   "Replace basic API key authentication with SCRAM-SHA-512 for secure, challenge-response authentication.",
			Reason:   "Basic authentication is insecure compared to SCRAM-SHA-512, which provides mutual authentication and prevents replay attacks.",
		})
	}

	// 4. Clients still using legacy API
	if s.ClientsUsingLegacyAPI > 0 {
		recs = append(recs, Recommendation{
			ID:       "migrate-legacy-clients",
			Title:    "Migrate Legacy API Clients",
			Priority: "high",
			Action:   "Update or replace all clients still using the legacy REST/XML API to use the WebSocket JSON-RPC 2.0 API.",
			Reason:   "Clients are still using the legacy API, which limits performance and increases maintenance burden.",
		})
	}

	// 5. Token rotation overdue
	if s.TokenRotationAge > 90*24*time.Hour {
		recs = append(recs, Recommendation{
			ID:       "rotate-api-keys",
			Title:    "Rotate API Keys",
			Priority: "medium",
			Action:   "Rotate all API keys and tokens that have exceeded the 90-day rotation threshold.",
			Reason:   "API keys have not been rotated in over 90 days, increasing the risk of credential compromise.",
		})
	}

	// 6. Rate limiting not enabled
	if !s.RateLimitEnabled {
		recs = append(recs, Recommendation{
			ID:       "enable-rate-limiting",
			Title:    "Enable Rate Limiting",
			Priority: "medium",
			Action:   "Enable rate limiting on all API endpoints to protect against abuse and DoS attacks.",
			Reason:   "Rate limiting is not enabled; the API is vulnerable to abuse and denial-of-service attacks.",
		})
	}

	// 7. OpenAPI documentation not enabled
	if !s.OpenAPIEnabled {
		recs = append(recs, Recommendation{
			ID:       "enable-openapi-docs",
			Title:    "Enable OpenAPI Documentation",
			Priority: "medium",
			Action:   "Generate and publish OpenAPI/Swagger documentation for all API endpoints.",
			Reason:   "OpenAPI documentation is not enabled; machine-readable specs improve discoverability and client generation.",
		})
	}

	// 8. Webhook support not enabled
	if !s.WebhookSupport {
		recs = append(recs, Recommendation{
			ID:       "enable-webhooks",
			Title:    "Enable Webhook Support",
			Priority: "low",
			Action:   "Implement webhook support so clients can subscribe to system events via HTTP callbacks.",
			Reason:   "Webhook support is not enabled; clients must poll for updates, increasing latency and server load.",
		})
	}

	// 9. Audit logging not enabled
	if !s.AuditLogEnabled {
		recs = append(recs, Recommendation{
			ID:       "enable-audit-logging",
			Title:    "Enable API Audit Logging",
			Priority: "medium",
			Action:   "Enable audit logging for all API calls to record access, mutations, and administrative actions.",
			Reason:   "Audit logging is not enabled; security incidents and compliance gaps cannot be traced.",
		})
	}

	// 10. API documentation not updated
	if !s.APIDocumentationUpdated {
		recs = append(recs, Recommendation{
			ID:       "update-api-docs",
			Title:    "Update API Documentation",
			Priority: "low",
			Action:   "Review and update all API documentation to reflect the current API surface and deprecation schedule.",
			Reason:   "API documentation is out of date; developers may rely on incorrect or missing information.",
		})
	}

	// Sort by priority rank (critical first)
	sort.SliceStable(recs, func(i, j int) bool {
		return priorityRank(recs[i].Priority) < priorityRank(recs[j].Priority)
	})

	return recs
}