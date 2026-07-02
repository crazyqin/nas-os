package nfspkaudit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// --- Enums ---

// AuthResult represents the outcome of a Kerberos authentication attempt.
type AuthResult string

const (
	AuthSuccess  AuthResult = "success"
	AuthFailure  AuthResult = "failure"
	AuthExpired  AuthResult = "expired"
	AuthRejected AuthResult = "rejected"
)

// EncryptionType represents the Kerberos encryption type used in a service ticket.
type EncryptionType string

const (
	EncAES256  EncryptionType = "aes256-cts-hmac-sha1-96"
	EncAES128  EncryptionType = "aes128-cts-hmac-sha1-96"
	EncDES3    EncryptionType = "des3-cbc-sha1"
	EncRC4     EncryptionType = "rc4-hmac"
	EncDES     EncryptionType = "des-cbc-crc"
	EncUnknown EncryptionType = "unknown"
)

// AlertSeverity represents the severity level of an alert.
type AlertSeverity string

const (
	SeverityLow      AlertSeverity = "low"
	SeverityMedium   AlertSeverity = "medium"
	SeverityHigh     AlertSeverity = "high"
	SeverityCritical AlertSeverity = "critical"
)

// ComplianceFramework represents a compliance standard.
type ComplianceFramework string

const (
	FrameworkSOC2     ComplianceFramework = "SOC2"
	FrameworkGDPR     ComplianceFramework = "GDPR"
	FrameworkISO27001 ComplianceFramework = "ISO27001"
)

// --- Data Structures ---

// AuthEvent represents a single NFS Kerberos authentication event.
type AuthEvent struct {
	ID              string            `json:"id"`
	Timestamp       time.Time         `json:"timestamp"`
	ClientPrincipal string            `json:"client_principal"`
	ClientIP        string            `json:"client_ip"`
	ServerPrincipal string            `json:"server_principal"`
	ServiceTicket   string            `json:"service_ticket"`
	EncryptionType  EncryptionType    `json:"encryption_type"`
	Result          AuthResult        `json:"result"`
	Reason          string            `json:"reason,omitempty"`
	SessionID       string            `json:"session_id,omitempty"`
	NFSExport       string            `json:"nfs_export,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// AuditTrailEntry represents a single link in the audit trail chain.
type AuditTrailEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // "auth", "access", "operation"
	Detail    string    `json:"detail"`
	UserID    string    `json:"user_id"`
}

// AuditTrail represents the complete authentication → access → operation chain.
type AuditTrail struct {
	SessionID string            `json:"session_id"`
	Entries   []AuditTrailEntry `json:"entries"`
	StartTime time.Time         `json:"start_time"`
	EndTime   time.Time         `json:"end_time"`
}

// SecurityAssessment represents the result of security analysis.
type SecurityAssessment struct {
	ID              string         `json:"id"`
	Timestamp       time.Time      `json:"timestamp"`
	WeakTickets     []WeakTicket   `json:"weak_tickets"`
	ReplayAttacks   []ReplayAttack `json:"replay_attacks"`
	RiskScore       float64        `json:"risk_score"` // 0-100
	Recommendations []string       `json:"recommendations"`
}

// WeakTicket represents a weak or insecure service ticket.
type WeakTicket struct {
	TicketID        string         `json:"ticket_id"`
	ClientPrincipal string         `json:"client_principal"`
	EncryptionType  EncryptionType `json:"encryption_type"`
	Reason          string         `json:"reason"`
}

// ReplayAttack represents a detected replay attack indicator.
type ReplayAttack struct {
	ID              string    `json:"id"`
	Timestamp       time.Time `json:"timestamp"`
	ClientPrincipal string    `json:"client_principal"`
	ClientIP        string    `json:"client_ip"`
	ServiceTicket   string    `json:"service_ticket"`
	Description     string    `json:"description"`
}

// Alert represents a triggered audit alert.
type Alert struct {
	ID              string        `json:"id"`
	Timestamp       time.Time     `json:"timestamp"`
	RuleName        string        `json:"rule_name"`
	Severity        AlertSeverity `json:"severity"`
	Description     string        `json:"description"`
	ClientPrincipal string        `json:"client_principal,omitempty"`
	ClientIP        string        `json:"client_ip,omitempty"`
	EventIDs        []string      `json:"event_ids"`
}

// ComplianceReport represents a compliance audit report.
type ComplianceReport struct {
	ID              string              `json:"id"`
	Framework       ComplianceFramework `json:"framework"`
	Period          string              `json:"period"`
	StartTime       time.Time           `json:"start_time"`
	EndTime         time.Time           `json:"end_time"`
	TotalAuthEvents int                 `json:"total_auth_events"`
	SuccessCount    int                 `json:"success_count"`
	FailureCount    int                 `json:"failure_count"`
	ExpiredCount    int                 `json:"expired_count"`
	RejectedCount   int                 `json:"rejected_count"`
	UniqueClients   int                 `json:"unique_clients"`
	UniqueExports   int                 `json:"unique_exports"`
	AlertCount      int                 `json:"alert_count"`
	SecurityScore   float64             `json:"security_score"`
	Findings        []ComplianceFinding `json:"findings"`
	GeneratedAt     time.Time           `json:"generated_at"`
}

// ComplianceFinding represents a compliance finding or observation.
type ComplianceFinding struct {
	ID          string        `json:"id"`
	Category    string        `json:"category"`
	Severity    AlertSeverity `json:"severity"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Controls    []string      `json:"controls"`
}

// AuditFilter represents filter criteria for querying audit logs.
type AuditFilter struct {
	ClientPrincipal string
	ClientIP        string
	Result          AuthResult
	NFSExport       string
	StartTime       time.Time
	EndTime         time.Time
	Limit           int
}

// --- Core Components ---

// AuditLogStore stores and queries audit log entries.
type AuditLogStore struct {
	mu     sync.RWMutex
	events []AuthEvent
}

// NewAuditLogStore creates a new AuditLogStore.
func NewAuditLogStore() *AuditLogStore {
	return &AuditLogStore{
		events: make([]AuthEvent, 0),
	}
}

// Store stores an authentication event.
func (s *AuditLogStore) Store(event AuthEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
}

// Query queries audit events matching the filter.
func (s *AuditLogStore) Query(filter AuditFilter) []AuthEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []AuthEvent
	for _, e := range s.events {
		if filter.ClientPrincipal != "" && e.ClientPrincipal != filter.ClientPrincipal {
			continue
		}
		if filter.ClientIP != "" && e.ClientIP != filter.ClientIP {
			continue
		}
		if filter.Result != "" && e.Result != filter.Result {
			continue
		}
		if filter.NFSExport != "" && e.NFSExport != filter.NFSExport {
			continue
		}
		if !filter.StartTime.IsZero() && e.Timestamp.Before(filter.StartTime) {
			continue
		}
		if !filter.EndTime.IsZero() && e.Timestamp.After(filter.EndTime) {
			continue
		}
		results = append(results, e)
	}
	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}
	return results
}

// GetAll returns all events.
func (s *AuditLogStore) GetAll() []AuthEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AuthEvent, len(s.events))
	copy(out, s.events)
	return out
}

// Count returns the total event count.
func (s *AuditLogStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events)
}

// Export exports all events as JSON.
func (s *AuditLogStore) Export() ([]byte, error) {
	return json.MarshalIndent(s.GetAll(), "", "  ")
}

// --- SecurityAssessor ---

// SecurityAssessor evaluates Kerberos authentication security.
type SecurityAssessor struct {
	store           *AuditLogStore
	weakEncryptions map[EncryptionType]bool
}

// NewSecurityAssessor creates a new SecurityAssessor.
func NewSecurityAssessor(store *AuditLogStore) *SecurityAssessor {
	return &SecurityAssessor{
		store: store,
		weakEncryptions: map[EncryptionType]bool{
			EncDES:  true,
			EncRC4:  true,
			EncDES3: true,
		},
	}
}

// Assess performs security analysis on stored events.
func (sa *SecurityAssessor) Assess(ctx context.Context) SecurityAssessment {
	events := sa.store.GetAll()

	assessment := SecurityAssessment{
		ID:        fmt.Sprintf("assess-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
	}

	seenTickets := make(map[string]AuthEvent)

	for _, e := range events {
		// weak ticket detection
		if sa.weakEncryptions[e.EncryptionType] {
			assessment.WeakTickets = append(assessment.WeakTickets, WeakTicket{
				TicketID:        e.ServiceTicket,
				ClientPrincipal: e.ClientPrincipal,
				EncryptionType:  e.EncryptionType,
				Reason:          fmt.Sprintf("weak encryption type: %s", e.EncryptionType),
			})
		}

		// replay attack detection: same ticket from different IPs or timestamps
		if prev, exists := seenTickets[e.ServiceTicket]; exists {
			if prev.ClientIP != e.ClientIP {
				assessment.ReplayAttacks = append(assessment.ReplayAttacks, ReplayAttack{
					ID:              fmt.Sprintf("replay-%d", len(assessment.ReplayAttacks)),
					Timestamp:       e.Timestamp,
					ClientPrincipal: e.ClientPrincipal,
					ClientIP:        e.ClientIP,
					ServiceTicket:   e.ServiceTicket,
					Description:     fmt.Sprintf("ticket %s previously used from %s, now from %s", e.ServiceTicket, prev.ClientIP, e.ClientIP),
				})
			}
		} else {
			seenTickets[e.ServiceTicket] = e
		}
	}

	assessment.RiskScore = sa.calculateRiskScore(&assessment)

	if len(assessment.WeakTickets) > 0 {
		assessment.Recommendations = append(assessment.Recommendations,
			"Replace weak encryption tickets with AES256 (aes256-cts-hmac-sha1-96)")
	}
	if len(assessment.ReplayAttacks) > 0 {
		assessment.Recommendations = append(assessment.Recommendations,
			"Investigate replay attack indicators and rotate affected service tickets")
	}
	if assessment.RiskScore > 70 {
		assessment.Recommendations = append(assessment.Recommendations,
			"Review Kerberos configuration and enforce stronger security policies")
	}

	return assessment
}

func (sa *SecurityAssessor) calculateRiskScore(a *SecurityAssessment) float64 {
	score := 0.0
	score += float64(len(a.WeakTickets)) * 5
	score += float64(len(a.ReplayAttacks)) * 20
	if score > 100 {
		score = 100
	}
	return score
}

// --- AlertRules ---

// AlertRule represents a rule for triggering alerts.
type AlertRule struct {
	Name        string
	Severity    AlertSeverity
	Description string
	Evaluate    func(events []AuthEvent) Alert
}

// AlertRules manages alert rules and evaluation.
type AlertRules struct {
	rules []AlertRule
}

// NewAlertRules creates a new AlertRules with default rules.
func NewAlertRules() *AlertRules {
	ar := &AlertRules{}
	ar.RegisterDefaults()
	return ar
}

// RegisterDefaults registers the default alert rules.
func (ar *AlertRules) RegisterDefaults() {
	ar.rules = append(ar.rules, AlertRule{
		Name:        "consecutive_auth_failures",
		Severity:    SeverityHigh,
		Description: "Consecutive authentication failures from the same client",
		Evaluate: func(events []AuthEvent) Alert {
			failuresByClient := make(map[string][]AuthEvent)
			for _, e := range events {
				if e.Result == AuthFailure || e.Result == AuthRejected {
					failuresByClient[e.ClientPrincipal] = append(failuresByClient[e.ClientPrincipal], e)
				}
			}
			for principal, fails := range failuresByClient {
				if len(fails) >= 3 {
					ids := make([]string, 0, len(fails))
					for _, f := range fails {
						ids = append(ids, f.ID)
					}
					return Alert{
						ID:              fmt.Sprintf("alert-fail-%d", time.Now().UnixNano()),
						Timestamp:       time.Now(),
						RuleName:        "consecutive_auth_failures",
						Severity:        SeverityHigh,
						Description:     fmt.Sprintf("Client %s has %d authentication failures", principal, len(fails)),
						ClientPrincipal: principal,
						ClientIP:        fails[len(fails)-1].ClientIP,
						EventIDs:        ids,
					}
				}
			}
			return Alert{}
		},
	})

	ar.rules = append(ar.rules, AlertRule{
		Name:        "abnormal_access_pattern",
		Severity:    SeverityMedium,
		Description: "Abnormal access patterns detected",
		Evaluate: func(events []AuthEvent) Alert {
			exportClientMap := make(map[string]map[string]bool)
			for _, e := range events {
				if e.Result != AuthSuccess {
					continue
				}
				if e.NFSExport == "" {
					continue
				}
				key := e.NFSExport
				if exportClientMap[key] == nil {
					exportClientMap[key] = make(map[string]bool)
				}
				exportClientMap[key][e.ClientPrincipal] = true
			}
			for export, clients := range exportClientMap {
				if len(clients) > 10 {
					ids := make([]string, 0)
					for _, e := range events {
						if e.NFSExport == export {
							ids = append(ids, e.ID)
						}
					}
					return Alert{
						ID:          fmt.Sprintf("alert-abnormal-%d", time.Now().UnixNano()),
						Timestamp:   time.Now(),
						RuleName:    "abnormal_access_pattern",
						Severity:    SeverityMedium,
						Description: fmt.Sprintf("Export %s accessed by %d unique clients", export, len(clients)),
						EventIDs:    ids,
					}
				}
			}
			return Alert{}
		},
	})

	ar.rules = append(ar.rules, AlertRule{
		Name:        "expired_ticket_usage",
		Severity:    SeverityCritical,
		Description: "Expired service ticket used for authentication",
		Evaluate: func(events []AuthEvent) Alert {
			var expiredEvents []AuthEvent
			for _, e := range events {
				if e.Result == AuthExpired {
					expiredEvents = append(expiredEvents, e)
				}
			}
			if len(expiredEvents) == 0 {
				return Alert{}
			}
			ids := make([]string, 0, len(expiredEvents))
			for _, e := range expiredEvents {
				ids = append(ids, e.ID)
			}
			return Alert{
				ID:          fmt.Sprintf("alert-expired-%d", time.Now().UnixNano()),
				Timestamp:   time.Now(),
				RuleName:    "expired_ticket_usage",
				Severity:    SeverityCritical,
				Description: fmt.Sprintf("%d expired ticket usage attempts detected", len(expiredEvents)),
				EventIDs:    ids,
			}
		},
	})
}

// Evaluate runs all rules against the provided events and returns triggered alerts.
func (ar *AlertRules) Evaluate(events []AuthEvent) []Alert {
	var alerts []Alert
	for _, rule := range ar.rules {
		alert := rule.Evaluate(events)
		if alert.ID != "" {
			alerts = append(alerts, alert)
		}
	}
	return alerts
}

// --- ComplianceReporter ---

// ComplianceReporter generates compliance audit reports.
type ComplianceReporter struct {
	store *AuditLogStore
	rules *AlertRules
}

// NewComplianceReporter creates a new ComplianceReporter.
func NewComplianceReporter(store *AuditLogStore, rules *AlertRules) *ComplianceReporter {
	return &ComplianceReporter{
		store: store,
		rules: rules,
	}
}

// GenerateReport generates a compliance report for the given framework and period.
func (cr *ComplianceReporter) GenerateReport(ctx context.Context, framework ComplianceFramework, period time.Duration) ComplianceReport {
	end := time.Now()
	start := end.Add(-period)

	events := cr.store.Query(AuditFilter{
		StartTime: start,
		EndTime:   end,
	})

	report := ComplianceReport{
		ID:          fmt.Sprintf("report-%d", end.UnixNano()),
		Framework:   framework,
		Period:      period.String(),
		StartTime:   start,
		EndTime:     end,
		GeneratedAt: time.Now(),
	}

	clients := make(map[string]bool)
	exports := make(map[string]bool)

	for _, e := range events {
		report.TotalAuthEvents++
		clients[e.ClientPrincipal] = true
		exports[e.NFSExport] = true

		switch e.Result {
		case AuthSuccess:
			report.SuccessCount++
		case AuthFailure:
			report.FailureCount++
		case AuthExpired:
			report.ExpiredCount++
		case AuthRejected:
			report.RejectedCount++
		}
	}

	report.UniqueClients = len(clients)
	report.UniqueExports = len(exports)

	alerts := cr.rules.Evaluate(events)
	report.AlertCount = len(alerts)

	report.SecurityScore = cr.calculateSecurityScore(&report, events)
	report.Findings = cr.generateFindings(&report, &alerts)

	return report
}

func (cr *ComplianceReporter) calculateSecurityScore(report *ComplianceReport, events []AuthEvent) float64 {
	if report.TotalAuthEvents == 0 {
		return 100.0
	}
	successRate := float64(report.SuccessCount) / float64(report.TotalAuthEvents) * 100
	penalty := float64(report.AlertCount) * 5
	score := successRate - penalty
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}

func (cr *ComplianceReporter) generateFindings(report *ComplianceReport, alerts *[]Alert) []ComplianceFinding {
	var findings []ComplianceFinding

	if report.FailureCount > 0 {
		findings = append(findings, ComplianceFinding{
			ID:          fmt.Sprintf("finding-%d-fail", len(findings)+1),
			Category:    "Authentication",
			Severity:    SeverityMedium,
			Title:       "Authentication Failures Detected",
			Description: fmt.Sprintf("%d authentication failures during the reporting period", report.FailureCount),
			Controls:    []string{"CC6.1", "CC6.2"},
		})
	}

	if report.ExpiredCount > 0 {
		findings = append(findings, ComplianceFinding{
			ID:          fmt.Sprintf("finding-%d-exp", len(findings)+1),
			Category:    "Ticket Management",
			Severity:    SeverityHigh,
			Title:       "Expired Ticket Usage",
			Description: fmt.Sprintf("%d expired ticket usage attempts during the reporting period", report.ExpiredCount),
			Controls:    []string{"CC6.1", "CC6.7"},
		})
	}

	if report.AlertCount > 0 {
		findings = append(findings, ComplianceFinding{
			ID:          fmt.Sprintf("finding-%d-alert", len(findings)+1),
			Category:    "Alerting",
			Severity:    SeverityHigh,
			Title:       "Security Alerts Triggered",
			Description: fmt.Sprintf("%d security alerts were triggered during the reporting period", report.AlertCount),
			Controls:    []string{"CC7.1", "CC7.2"},
		})
	}

	return findings
}

// --- NFSKerberosAuditor ---

// NFSKerberosAuditor is the main auditor orchestrating all NFS Kerberos audit operations.
type NFSKerberosAuditor struct {
	mu       sync.RWMutex
	store    *AuditLogStore
	assessor *SecurityAssessor
	rules    *AlertRules
	reporter *ComplianceReporter
	alerts   []Alert
}

// NewNFSKerberosAuditor creates a new NFSKerberosAuditor with default components.
func NewNFSKerberosAuditor() *NFSKerberosAuditor {
	store := NewAuditLogStore()
	rules := NewAlertRules()
	return &NFSKerberosAuditor{
		store:    store,
		assessor: NewSecurityAssessor(store),
		rules:    rules,
		reporter: NewComplianceReporter(store, rules),
		alerts:   make([]Alert, 0),
	}
}

// RecordEvent records an authentication event and evaluates alerts.
func (a *NFSKerberosAuditor) RecordEvent(event AuthEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.store.Store(event)

	newAlerts := a.rules.Evaluate(a.store.GetAll())
	if len(newAlerts) > 0 {
		a.alerts = append(a.alerts, newAlerts...)
	}
}

// GetAuditTrail builds an audit trail for a given session ID.
func (a *NFSKerberosAuditor) GetAuditTrail(sessionID string) AuditTrail {
	events := a.store.Query(AuditFilter{})

	var trailEntries []AuditTrailEntry
	var start, end time.Time

	for _, e := range events {
		if e.SessionID != sessionID {
			continue
		}
		if start.IsZero() || e.Timestamp.Before(start) {
			start = e.Timestamp
		}
		if end.IsZero() || e.Timestamp.After(end) {
			end = e.Timestamp
		}
		trailEntries = append(trailEntries, AuditTrailEntry{
			ID:        e.ID,
			Timestamp: e.Timestamp,
			Type:      "auth",
			Detail:    fmt.Sprintf("Kerberos auth %s for %s on %s", e.Result, e.ClientPrincipal, e.NFSExport),
			UserID:    e.ClientPrincipal,
		})
	}

	return AuditTrail{
		SessionID: sessionID,
		Entries:   trailEntries,
		StartTime: start,
		EndTime:   end,
	}
}

// GetComplianceReport generates a compliance report for the given framework.
func (a *NFSKerberosAuditor) GetComplianceReport(ctx context.Context, framework ComplianceFramework, period time.Duration) ComplianceReport {
	return a.reporter.GenerateReport(ctx, framework, period)
}

// GetAlerts returns all triggered alerts since the given time.
func (a *NFSKerberosAuditor) GetAlerts(since time.Time) []Alert {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var results []Alert
	for _, alert := range a.alerts {
		if alert.Timestamp.After(since) || alert.Timestamp.Equal(since) {
			results = append(results, alert)
		}
	}
	return results
}

// GetSecurityAssessment performs security assessment.
func (a *NFSKerberosAuditor) GetSecurityAssessment(ctx context.Context) SecurityAssessment {
	return a.assessor.Assess(ctx)
}

// QueryEvents queries audit events with a filter.
func (a *NFSKerberosAuditor) QueryEvents(filter AuditFilter) []AuthEvent {
	return a.store.Query(filter)
}

// GetStore returns the underlying audit log store.
func (a *NFSKerberosAuditor) GetStore() *AuditLogStore {
	return a.store
}

// --- RESTful API Handlers ---

// APIHandler provides RESTful API endpoints for the audit module.
type APIHandler struct {
	auditor *NFSKerberosAuditor
}

// NewAPIHandler creates a new APIHandler.
func NewAPIHandler(auditor *NFSKerberosAuditor) *APIHandler {
	return &APIHandler{auditor: auditor}
}

// GetAuditTrail handler: GET /api/nfspkaudit/trail?session_id=xxx.
func (h *APIHandler) GetAuditTrail(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, `{"error":"session_id required"}`, http.StatusBadRequest)
		return
	}

	trail := h.auditor.GetAuditTrail(sessionID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trail)
}

// GetComplianceReportHandler: GET /api/nfspkaudit/report?framework=SOC2&period=24h.
func (h *APIHandler) GetComplianceReportHandler(w http.ResponseWriter, r *http.Request) {
	frameworkStr := r.URL.Query().Get("framework")
	if frameworkStr == "" {
		frameworkStr = "SOC2"
	}
	framework := ComplianceFramework(frameworkStr)

	periodStr := r.URL.Query().Get("period")
	if periodStr == "" {
		periodStr = "24h"
	}

	period, err := time.ParseDuration(periodStr)
	if err != nil {
		http.Error(w, `{"error":"invalid period format"}`, http.StatusBadRequest)
		return
	}

	report := h.auditor.GetComplianceReport(r.Context(), framework, period)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

// GetAlertsHandler: GET /api/nfspkaudit/alerts?since=2024-01-01T00:00:00Z.
func (h *APIHandler) GetAlertsHandler(w http.ResponseWriter, r *http.Request) {
	sinceStr := r.URL.Query().Get("since")
	var since time.Time
	if sinceStr != "" {
		var err error
		since, err = time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			http.Error(w, `{"error":"invalid time format, use RFC3339"}`, http.StatusBadRequest)
			return
		}
	} else {
		since = time.Time{}
	}

	alerts := h.auditor.GetAlerts(since)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(alerts)
}

// GetSecurityAssessmentHandler: GET /api/nfspkaudit/assessment.
func (h *APIHandler) GetSecurityAssessmentHandler(w http.ResponseWriter, r *http.Request) {
	assessment := h.auditor.GetSecurityAssessment(r.Context())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(assessment)
}

// RegisterRoutes registers the API routes on the given mux.
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/nfspkaudit/trail", h.GetAuditTrail)
	mux.HandleFunc("/api/nfspkaudit/report", h.GetComplianceReportHandler)
	mux.HandleFunc("/api/nfspkaudit/alerts", h.GetAlertsHandler)
	mux.HandleFunc("/api/nfspkaudit/assessment", h.GetSecurityAssessmentHandler)
}
