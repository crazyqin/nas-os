package nfspkaudit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func makeEventPast(id, principal, ip, ticket string, enc EncryptionType, result AuthResult, sessionID, nfsExport string, offset time.Duration) AuthEvent {
	return AuthEvent{
		ID:              id,
		Timestamp:       time.Now().Add(-offset),
		ClientPrincipal: principal,
		ClientIP:        ip,
		ServerPrincipal: "nfs/server@REALM",
		ServiceTicket:   ticket,
		EncryptionType:  enc,
		Result:          result,
		SessionID:       sessionID,
		NFSExport:       nfsExport,
	}
}

func TestAuditLogStore_StoreAndQuery(t *testing.T) {
	store := NewAuditLogStore()

	e1 := makeEventPast("e1", "user1@REALM", "10.0.0.1", "tgt-001", EncAES256, AuthSuccess, "s1", "/export1", 0)
	e2 := makeEventPast("e2", "user2@REALM", "10.0.0.2", "tgt-002", EncRC4, AuthFailure, "s2", "/export2", time.Minute)

	store.Store(e1)
	store.Store(e2)

	if store.Count() != 2 {
		t.Fatalf("expected count 2, got %d", store.Count())
	}

	// Query by client principal
	results := store.Query(AuditFilter{ClientPrincipal: "user1@REALM"})
	if len(results) != 1 || results[0].ID != "e1" {
		t.Fatalf("expected e1 for user1, got %v", results)
	}

	// Query by result
	results = store.Query(AuditFilter{Result: AuthFailure})
	if len(results) != 1 || results[0].ID != "e2" {
		t.Fatalf("expected e2 for failure, got %v", results)
	}

	// Query by time range (only e1 which is at now, e2 is 1 min ago)
	results = store.Query(AuditFilter{StartTime: time.Now().Add(-30 * time.Second)})
	if len(results) != 1 || results[0].ID != "e1" {
		t.Fatalf("expected e1 within recent range, got %v", results)
	}

	// Query with limit
	results = store.Query(AuditFilter{Limit: 1})
	if len(results) != 1 {
		t.Fatalf("expected 1 result with limit, got %d", len(results))
	}

	// Query all
	results = store.GetAll()
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestAuditLogStore_Export(t *testing.T) {
	store := NewAuditLogStore()
	store.Store(makeEventPast("e1", "user1@REALM", "10.0.0.1", "tgt-001", EncAES256, AuthSuccess, "s1", "/export1", 0))

	data, err := store.Export()
	if err != nil {
		t.Fatalf("export error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("exported data is empty")
	}
}

func TestSecurityAssessor_WeakTicketDetection(t *testing.T) {
	store := NewAuditLogStore()

	store.Store(makeEventPast("e1", "user1@REALM", "10.0.0.1", "weak-001", EncRC4, AuthSuccess, "s1", "/export1", 0))
	store.Store(makeEventPast("e2", "user2@REALM", "10.0.0.2", "strong-001", EncAES256, AuthSuccess, "s2", "/export2", 0))

	assessor := NewSecurityAssessor(store)
	assessment := assessor.Assess(context.Background())

	if len(assessment.WeakTickets) != 1 {
		t.Fatalf("expected 1 weak ticket, got %d", len(assessment.WeakTickets))
	}
	if assessment.WeakTickets[0].EncryptionType != EncRC4 {
		t.Fatalf("expected RC4 weak ticket, got %s", assessment.WeakTickets[0].EncryptionType)
	}
	if assessment.RiskScore <= 0 {
		t.Fatal("expected non-zero risk score")
	}
}

func TestSecurityAssessor_ReplayAttackDetection(t *testing.T) {
	store := NewAuditLogStore()

	// Same ticket from different IPs
	store.Store(makeEventPast("e1", "user1@REALM", "10.0.0.1", "dup-001", EncAES256, AuthSuccess, "s1", "/export1", 2*time.Minute))
	store.Store(makeEventPast("e2", "user1@REALM", "10.0.0.2", "dup-001", EncAES256, AuthSuccess, "s2", "/export1", time.Minute))

	assessor := NewSecurityAssessor(store)
	assessment := assessor.Assess(context.Background())

	if len(assessment.ReplayAttacks) != 1 {
		t.Fatalf("expected 1 replay attack, got %d", len(assessment.ReplayAttacks))
	}
	if assessment.ReplayAttacks[0].ServiceTicket != "dup-001" {
		t.Fatalf("expected dup-001, got %s", assessment.ReplayAttacks[0].ServiceTicket)
	}
}

func TestSecurityAssessor_Recommendations(t *testing.T) {
	store := NewAuditLogStore()

	store.Store(makeEventPast("e1", "user1@REALM", "10.0.0.1", "weak-001", EncDES, AuthSuccess, "s1", "/export1", 0))

	assessor := NewSecurityAssessor(store)
	assessment := assessor.Assess(context.Background())

	if len(assessment.Recommendations) == 0 {
		t.Fatal("expected recommendations for weak tickets")
	}
}

func TestAlertRules_ConsecutiveAuthFailures(t *testing.T) {
	store := NewAuditLogStore()

	store.Store(makeEventPast("e1", "user1@REALM", "10.0.0.1", "t1", EncAES256, AuthFailure, "s1", "/export1", 2*time.Minute))
	store.Store(makeEventPast("e2", "user1@REALM", "10.0.0.1", "t2", EncAES256, AuthFailure, "s2", "/export1", time.Minute))
	store.Store(makeEventPast("e3", "user1@REALM", "10.0.0.1", "t3", EncAES256, AuthFailure, "s3", "/export1", 0))

	rules := NewAlertRules()
	alerts := rules.Evaluate(store.GetAll())

	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].RuleName != "consecutive_auth_failures" {
		t.Fatalf("expected consecutive_auth_failures, got %s", alerts[0].RuleName)
	}
	if alerts[0].Severity != SeverityHigh {
		t.Fatalf("expected high severity, got %s", alerts[0].Severity)
	}
}

func TestAlertRules_ExpiredTicketUsage(t *testing.T) {
	store := NewAuditLogStore()

	store.Store(makeEventPast("e1", "user1@REALM", "10.0.0.1", "exp-001", EncAES256, AuthExpired, "s1", "/export1", 0))

	rules := NewAlertRules()
	alerts := rules.Evaluate(store.GetAll())

	found := false
	for _, a := range alerts {
		if a.RuleName == "expired_ticket_usage" {
			found = true
			if a.Severity != SeverityCritical {
				t.Fatalf("expected critical severity, got %s", a.Severity)
			}
		}
	}
	if !found {
		t.Fatal("expected expired_ticket_usage alert")
	}
}

func TestAlertRules_AbnormalAccessPattern(t *testing.T) {
	store := NewAuditLogStore()

	for i := 0; i < 15; i++ {
		store.Store(makeEventPast(
			"e-abn-"+string(rune(i)),
			"client"+string(rune(i))+"@REALM",
			"10.0.0."+string(rune(i)),
			"ticket-"+string(rune(i)),
			EncAES256, AuthSuccess, "s", "/export-shared", 0,
		))
	}

	rules := NewAlertRules()
	alerts := rules.Evaluate(store.GetAll())

	found := false
	for _, a := range alerts {
		if a.RuleName == "abnormal_access_pattern" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected abnormal_access_pattern alert for >10 unique clients")
	}
}

func TestComplianceReporter_GenerateReport(t *testing.T) {
	store := NewAuditLogStore()

	store.Store(makeEventPast("e1", "user1@REALM", "10.0.0.1", "t1", EncAES256, AuthSuccess, "s1", "/export1", 0))
	store.Store(makeEventPast("e2", "user2@REALM", "10.0.0.2", "t2", EncAES256, AuthFailure, "s2", "/export1", time.Minute))
	store.Store(makeEventPast("e3", "user3@REALM", "10.0.0.3", "t3", EncRC4, AuthExpired, "s3", "/export2", 2*time.Minute))

	rules := NewAlertRules()
	reporter := NewComplianceReporter(store, rules)

	report := reporter.GenerateReport(context.Background(), FrameworkSOC2, 24*time.Hour)

	if report.Framework != FrameworkSOC2 {
		t.Fatalf("expected SOC2, got %s", report.Framework)
	}
	if report.TotalAuthEvents != 3 {
		t.Fatalf("expected 3 events, got %d", report.TotalAuthEvents)
	}
	if report.SuccessCount != 1 {
		t.Fatalf("expected 1 success, got %d", report.SuccessCount)
	}
	if report.FailureCount != 1 {
		t.Fatalf("expected 1 failure, got %d", report.FailureCount)
	}
	if report.ExpiredCount != 1 {
		t.Fatalf("expected 1 expired, got %d", report.ExpiredCount)
	}
	if report.UniqueClients != 3 {
		t.Fatalf("expected 3 unique clients, got %d", report.UniqueClients)
	}
	if report.UniqueExports != 2 {
		t.Fatalf("expected 2 unique exports, got %d", report.UniqueExports)
	}
	if report.SecurityScore < 0 || report.SecurityScore > 100 {
		t.Fatalf("security score out of range: %f", report.SecurityScore)
	}
}

func TestComplianceReporter_Findings(t *testing.T) {
	store := NewAuditLogStore()

	store.Store(makeEventPast("e1", "user1@REALM", "10.0.0.1", "t1", EncAES256, AuthFailure, "s1", "/export1", 0))
	store.Store(makeEventPast("e2", "user1@REALM", "10.0.0.1", "t2", EncAES256, AuthExpired, "s2", "/export1", time.Minute))

	rules := NewAlertRules()
	reporter := NewComplianceReporter(store, rules)

	report := reporter.GenerateReport(context.Background(), FrameworkGDPR, 24*time.Hour)

	if len(report.Findings) == 0 {
		t.Fatal("expected findings for failures and expired tickets")
	}

	foundFailure := false
	foundExpired := false
	for _, f := range report.Findings {
		if f.Category == "Authentication" {
			foundFailure = true
		}
		if f.Category == "Ticket Management" {
			foundExpired = true
		}
	}
	if !foundFailure {
		t.Fatal("expected authentication failure finding")
	}
	if !foundExpired {
		t.Fatal("expected expired ticket finding")
	}
}

func TestNFSKerberosAuditor_RecordAndGetAlerts(t *testing.T) {
	auditor := NewNFSKerberosAuditor()

	auditor.RecordEvent(makeEventPast("e1", "user1@REALM", "10.0.0.1", "t1", EncAES256, AuthFailure, "s1", "/export1", 2*time.Minute))
	auditor.RecordEvent(makeEventPast("e2", "user1@REALM", "10.0.0.1", "t2", EncAES256, AuthFailure, "s2", "/export1", time.Minute))
	auditor.RecordEvent(makeEventPast("e3", "user1@REALM", "10.0.0.1", "t3", EncAES256, AuthFailure, "s3", "/export1", 0))

	alerts := auditor.GetAlerts(time.Time{})
	if len(alerts) == 0 {
		t.Fatal("expected alerts after 3 consecutive failures")
	}

	// Verify we can filter by time - alerts from future
	since := time.Now().Add(time.Hour)
	alerts = auditor.GetAlerts(since)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts after now+1h, got %d", len(alerts))
	}
}

func TestNFSKerberosAuditor_GetAuditTrail(t *testing.T) {
	auditor := NewNFSKerberosAuditor()

	auditor.RecordEvent(makeEventPast("e1", "user1@REALM", "10.0.0.1", "t1", EncAES256, AuthSuccess, "session-1", "/export1", 2*time.Minute))
	auditor.RecordEvent(makeEventPast("e2", "user2@REALM", "10.0.0.2", "t2", EncAES256, AuthSuccess, "session-2", "/export2", time.Minute))
	auditor.RecordEvent(makeEventPast("e3", "user1@REALM", "10.0.0.1", "t3", EncAES256, AuthSuccess, "session-1", "/export1", 0))

	trail := auditor.GetAuditTrail("session-1")
	if len(trail.Entries) != 2 {
		t.Fatalf("expected 2 trail entries for session-1, got %d", len(trail.Entries))
	}
	if trail.SessionID != "session-1" {
		t.Fatalf("expected session-1, got %s", trail.SessionID)
	}
}

func TestNFSKerberosAuditor_GetComplianceReport(t *testing.T) {
	auditor := NewNFSKerberosAuditor()

	auditor.RecordEvent(makeEventPast("e1", "user1@REALM", "10.0.0.1", "t1", EncAES256, AuthSuccess, "s1", "/export1", 0))
	auditor.RecordEvent(makeEventPast("e2", "user2@REALM", "10.0.0.2", "t2", EncAES256, AuthFailure, "s2", "/export1", time.Minute))

	report := auditor.GetComplianceReport(context.Background(), FrameworkSOC2, 24*time.Hour)
	if report.TotalAuthEvents != 2 {
		t.Fatalf("expected 2 events, got %d", report.TotalAuthEvents)
	}
	if report.Framework != FrameworkSOC2 {
		t.Fatalf("expected SOC2, got %s", report.Framework)
	}
}

func TestNFSKerberosAuditor_GetSecurityAssessment(t *testing.T) {
	auditor := NewNFSKerberosAuditor()

	auditor.RecordEvent(makeEventPast("e1", "user1@REALM", "10.0.0.1", "weak-001", EncDES, AuthSuccess, "s1", "/export1", 0))

	assessment := auditor.GetSecurityAssessment(context.Background())
	if len(assessment.WeakTickets) == 0 {
		t.Fatal("expected weak tickets in assessment")
	}
}

func TestNFSKerberosAuditor_QueryEvents(t *testing.T) {
	auditor := NewNFSKerberosAuditor()

	auditor.RecordEvent(makeEventPast("e1", "user1@REALM", "10.0.0.1", "t1", EncAES256, AuthSuccess, "s1", "/export1", 0))
	auditor.RecordEvent(makeEventPast("e2", "user2@REALM", "10.0.0.2", "t2", EncAES256, AuthFailure, "s2", "/export2", 0))

	results := auditor.QueryEvents(AuditFilter{ClientIP: "10.0.0.1"})
	if len(results) != 1 || results[0].ID != "e1" {
		t.Fatalf("expected e1 for 10.0.0.1, got %v", results)
	}
}

// --- API Handler Tests ---

func TestAPIHandler_GetAuditTrail(t *testing.T) {
	auditor := NewNFSKerberosAuditor()
	auditor.RecordEvent(makeEventPast("e1", "user1@REALM", "10.0.0.1", "t1", EncAES256, AuthSuccess, "session-1", "/export1", 0))

	handler := NewAPIHandler(auditor)

	req := httptest.NewRequest(http.MethodGet, "/api/nfspkaudit/trail?session_id=session-1", nil)
	rec := httptest.NewRecorder()
	handler.GetAuditTrail(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAPIHandler_GetAuditTrail_MissingSessionID(t *testing.T) {
	auditor := NewNFSKerberosAuditor()
	handler := NewAPIHandler(auditor)

	req := httptest.NewRequest(http.MethodGet, "/api/nfspkaudit/trail", nil)
	rec := httptest.NewRecorder()
	handler.GetAuditTrail(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAPIHandler_GetComplianceReport(t *testing.T) {
	auditor := NewNFSKerberosAuditor()
	auditor.RecordEvent(makeEventPast("e1", "user1@REALM", "10.0.0.1", "t1", EncAES256, AuthSuccess, "s1", "/export1", 0))

	handler := NewAPIHandler(auditor)

	req := httptest.NewRequest(http.MethodGet, "/api/nfspkaudit/report?framework=SOC2&period=24h", nil)
	rec := httptest.NewRecorder()
	handler.GetComplianceReportHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAPIHandler_GetComplianceReport_InvalidPeriod(t *testing.T) {
	auditor := NewNFSKerberosAuditor()
	handler := NewAPIHandler(auditor)

	req := httptest.NewRequest(http.MethodGet, "/api/nfspkaudit/report?period=invalid", nil)
	rec := httptest.NewRecorder()
	handler.GetComplianceReportHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAPIHandler_GetAlerts(t *testing.T) {
	auditor := NewNFSKerberosAuditor()

	auditor.RecordEvent(makeEventPast("e1", "user1@REALM", "10.0.0.1", "t1", EncAES256, AuthFailure, "s1", "/export1", 2*time.Minute))
	auditor.RecordEvent(makeEventPast("e2", "user1@REALM", "10.0.0.1", "t2", EncAES256, AuthFailure, "s2", "/export1", time.Minute))
	auditor.RecordEvent(makeEventPast("e3", "user1@REALM", "10.0.0.1", "t3", EncAES256, AuthFailure, "s3", "/export1", 0))

	handler := NewAPIHandler(auditor)

	req := httptest.NewRequest(http.MethodGet, "/api/nfspkaudit/alerts", nil)
	rec := httptest.NewRecorder()
	handler.GetAlertsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAPIHandler_GetAlerts_InvalidTime(t *testing.T) {
	auditor := NewNFSKerberosAuditor()
	handler := NewAPIHandler(auditor)

	req := httptest.NewRequest(http.MethodGet, "/api/nfspkaudit/alerts?since=invalid", nil)
	rec := httptest.NewRecorder()
	handler.GetAlertsHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAPIHandler_GetSecurityAssessment(t *testing.T) {
	auditor := NewNFSKerberosAuditor()
	auditor.RecordEvent(makeEventPast("e1", "user1@REALM", "10.0.0.1", "t1", EncRC4, AuthSuccess, "s1", "/export1", 0))

	handler := NewAPIHandler(auditor)

	req := httptest.NewRequest(http.MethodGet, "/api/nfspkaudit/assessment", nil)
	rec := httptest.NewRecorder()
	handler.GetSecurityAssessmentHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAPIHandler_RegisterRoutes(t *testing.T) {
	auditor := NewNFSKerberosAuditor()
	handler := NewAPIHandler(auditor)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Verify routes are registered by making a request
	req := httptest.NewRequest(http.MethodGet, "/api/nfspkaudit/report?framework=SOC2&period=1h", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for registered route, got %d", rec.Code)
	}
}

func TestNFSKerberosAuditor_GetStore(t *testing.T) {
	auditor := NewNFSKerberosAuditor()
	store := auditor.GetStore()
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if store.Count() != 0 {
		t.Fatalf("expected empty store, got %d", store.Count())
	}
}
