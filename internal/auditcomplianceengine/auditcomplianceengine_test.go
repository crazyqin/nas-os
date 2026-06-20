package auditcomplianceengine

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

func setupTestEngine(t *testing.T) *ComplianceEngine {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	config := &EngineConfig{
		AuditRetention:     30 * 24 * time.Hour,
		AutoAssess:         true,
		AlertOnViolation:   true,
		DataRegion:         "us-east-1",
		EncryptionRequired: true,
	}
	engine := NewComplianceEngine(config, logger)
	t.Cleanup(func() {
		engine.Shutdown()
	})
	return engine
}

func TestNewComplianceEngine(t *testing.T) {
	t.Run("with config and logger", func(t *testing.T) {
		engine := setupTestEngine(t)
		if engine == nil {
			t.Fatal("expected engine to be non-nil")
		}
		if engine.frameworks == nil {
			t.Fatal("expected frameworks map to be initialized")
		}
		if engine.controls == nil {
			t.Fatal("expected controls map to be initialized")
		}
		if engine.findings == nil {
			t.Fatal("expected findings map to be initialized")
		}
		if engine.reports == nil {
			t.Fatal("expected reports map to be initialized")
		}
	})

	t.Run("with nil config", func(t *testing.T) {
		logger := slog.Default()
		engine := NewComplianceEngine(nil, logger)
		if engine == nil {
			t.Fatal("expected engine to be non-nil")
		}
		if engine.config == nil {
			t.Fatal("expected default config to be set")
		}
		if engine.config.AuditRetention != 365*24*time.Hour {
			t.Errorf("expected default audit retention of 365 days, got %v", engine.config.AuditRetention)
		}
		engine.Shutdown()
	})

	t.Run("with nil logger", func(t *testing.T) {
		engine := NewComplianceEngine(nil, nil)
		if engine == nil {
			t.Fatal("expected engine to be non-nil")
		}
		if engine.logger == nil {
			t.Fatal("expected default logger to be set")
		}
		engine.Shutdown()
	})
}

func TestRegisterFramework(t *testing.T) {
	engine := setupTestEngine(t)

	t.Run("register valid framework", func(t *testing.T) {
		framework := &ComplianceFramework{
			ID:      "soc2",
			Name:    "SOC 2",
			Type:    FrameworkSOC2,
			Version: "2017",
		}
		err := engine.RegisterFramework(framework)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if framework.Status != ComplianceUnknown {
			t.Errorf("expected status ComplianceUnknown, got %v", framework.Status)
		}
	})

	t.Run("register duplicate framework", func(t *testing.T) {
		framework := &ComplianceFramework{
			ID:      "soc2",
			Name:    "SOC 2 Duplicate",
			Type:    FrameworkSOC2,
			Version: "2017",
		}
		err := engine.RegisterFramework(framework)
		if err != ErrFrameworkAlreadyExists {
			t.Fatalf("expected ErrFrameworkAlreadyExists, got %v", err)
		}
	})

	t.Run("register nil framework", func(t *testing.T) {
		err := engine.RegisterFramework(nil)
		if err != ErrNilFramework {
			t.Fatalf("expected ErrNilFramework, got %v", err)
		}
	})

	t.Run("register framework with empty ID", func(t *testing.T) {
		framework := &ComplianceFramework{
			Name: "Invalid Framework",
		}
		err := engine.RegisterFramework(framework)
		if err != ErrInvalidFrameworkID {
			t.Fatalf("expected ErrInvalidFrameworkID, got %v", err)
		}
	})
}

func TestAddControl(t *testing.T) {
	engine := setupTestEngine(t)

	framework := &ComplianceFramework{
		ID:      "iso27001",
		Name:    "ISO 27001",
		Type:    FrameworkISO27001,
		Version: "2022",
	}
	engine.RegisterFramework(framework)

	t.Run("add valid control", func(t *testing.T) {
		control := &Control{
			ID:          "ctrl-001",
			FrameworkID: "iso27001",
			Name:        "Access Control",
			Description: "Implement access control policies",
			Category:    "Security",
		}
		err := engine.AddControl(control)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if control.Status != ControlNotImplemented {
			t.Errorf("expected status ControlNotImplemented, got %v", control.Status)
		}
		if len(framework.Controls) != 1 || framework.Controls[0] != "ctrl-001" {
			t.Errorf("expected framework to have control ctrl-001")
		}
	})

	t.Run("add duplicate control", func(t *testing.T) {
		control := &Control{
			ID:          "ctrl-001",
			FrameworkID: "iso27001",
			Name:        "Access Control Duplicate",
		}
		err := engine.AddControl(control)
		if err != ErrControlAlreadyExists {
			t.Fatalf("expected ErrControlAlreadyExists, got %v", err)
		}
	})

	t.Run("add nil control", func(t *testing.T) {
		err := engine.AddControl(nil)
		if err != ErrNilControl {
			t.Fatalf("expected ErrNilControl, got %v", err)
		}
	})

	t.Run("add control with empty ID", func(t *testing.T) {
		control := &Control{
			FrameworkID: "iso27001",
			Name:        "Invalid Control",
		}
		err := engine.AddControl(control)
		if err != ErrInvalidControlID {
			t.Fatalf("expected ErrInvalidControlID, got %v", err)
		}
	})

	t.Run("add control to non-existent framework", func(t *testing.T) {
		control := &Control{
			ID:          "ctrl-002",
			FrameworkID: "non-existent",
			Name:        "Orphan Control",
		}
		err := engine.AddControl(control)
		if err != ErrFrameworkNotFound {
			t.Fatalf("expected ErrFrameworkNotFound, got %v", err)
		}
	})
}

func TestRecordAuditEvent(t *testing.T) {
	engine := setupTestEngine(t)

	t.Run("record valid audit event", func(t *testing.T) {
		entry := &AuditEntry{
			EventType:    EventAccess,
			Actor:        "user123",
			ActorType:    ActorUser,
			Resource:     "/api/data",
			ResourceType: "endpoint",
			Action:       "GET",
			Result:       ActionSuccess,
			IPAddress:    "192.168.1.1",
			UserAgent:    "Mozilla/5.0",
			SessionID:    "sess-001",
			Details: map[string]interface{}{
				"query": "select * from users",
			},
		}
		err := engine.RecordAuditEvent(entry)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.ID == "" {
			t.Fatal("expected entry ID to be generated")
		}
		if entry.Timestamp.IsZero() {
			t.Fatal("expected timestamp to be set")
		}
	})

	t.Run("record nil audit entry", func(t *testing.T) {
		err := engine.RecordAuditEvent(nil)
		if err != ErrNilAuditEntry {
			t.Fatalf("expected ErrNilAuditEntry, got %v", err)
		}
	})

	t.Run("multiple audit events", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			entry := &AuditEntry{
				EventType: EventAccess,
				Actor:     "user123",
				ActorType: ActorUser,
				Resource:  "/api/data",
				Action:    "GET",
				Result:    ActionSuccess,
			}
			engine.RecordAuditEvent(entry)
		}
		metrics := engine.GetMetrics()
		if metrics.AuditTrailSize < 6 {
			t.Errorf("expected at least 6 audit entries, got %d", metrics.AuditTrailSize)
		}
	})
}

func TestRunAssessment(t *testing.T) {
	engine := setupTestEngine(t)

	framework := &ComplianceFramework{
		ID:      "soc2",
		Name:    "SOC 2",
		Type:    FrameworkSOC2,
		Version: "2017",
	}
	engine.RegisterFramework(framework)

	control1 := &Control{
		ID:          "ctrl-001",
		FrameworkID: "soc2",
		Name:        "Access Control",
		Category:    "Security",
	}
	engine.AddControl(control1)
	control1.Status = ControlImplemented

	control2 := &Control{
		ID:          "ctrl-002",
		FrameworkID: "soc2",
		Name:        "Encryption",
		Category:    "Security",
	}
	engine.AddControl(control2)
	control2.Status = ControlNotImplemented

	control3 := &Control{
		ID:          "ctrl-003",
		FrameworkID: "soc2",
		Name:        "Logging",
		Category:    "Monitoring",
	}
	engine.AddControl(control3)
	control3.Status = ControlPartial

	t.Run("run assessment on existing framework", func(t *testing.T) {
		report, err := engine.RunAssessment("soc2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report == nil {
			t.Fatal("expected report to be non-nil")
		}
		if report.Score < 0 || report.Score > 100 {
			t.Errorf("expected score between 0 and 100, got %f", report.Score)
		}
		if report.Summary.TotalControls != 3 {
			t.Errorf("expected 3 total controls, got %d", report.Summary.TotalControls)
		}
		if report.Summary.Passed != 1 {
			t.Errorf("expected 1 passed control, got %d", report.Summary.Passed)
		}
		if report.Summary.Failed != 1 {
			t.Errorf("expected 1 failed control, got %d", report.Summary.Failed)
		}
	})

	t.Run("run assessment on non-existent framework", func(t *testing.T) {
		_, err := engine.RunAssessment("non-existent")
		if err != ErrFrameworkNotFound {
			t.Fatalf("expected ErrFrameworkNotFound, got %v", err)
		}
	})
}

func TestCreateFinding(t *testing.T) {
	engine := setupTestEngine(t)

	framework := &ComplianceFramework{
		ID:   "iso27001",
		Name: "ISO 27001",
	}
	engine.RegisterFramework(framework)

	control := &Control{
		ID:          "ctrl-001",
		FrameworkID: "iso27001",
		Name:        "Access Control",
	}
	engine.AddControl(control)

	t.Run("create valid finding", func(t *testing.T) {
		finding := &Finding{
			ControlID:   "ctrl-001",
			Severity:    FindingHigh,
			Title:       "Missing MFA",
			Description: "Multi-factor authentication is not enabled",
			Remediation: "Enable MFA for all users",
			AssignedTo:  "security-team",
		}
		err := engine.CreateFinding(finding)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if finding.ID == "" {
			t.Fatal("expected finding ID to be generated")
		}
		if finding.Status != FindingOpen {
			t.Errorf("expected status FindingOpen, got %v", finding.Status)
		}
		if finding.FoundAt.IsZero() {
			t.Fatal("expected FoundAt to be set")
		}
		if finding.DueDate.IsZero() {
			t.Fatal("expected DueDate to be set")
		}
	})

	t.Run("create finding with nil", func(t *testing.T) {
		err := engine.CreateFinding(nil)
		if err != ErrNilFinding {
			t.Fatalf("expected ErrNilFinding, got %v", err)
		}
	})

	t.Run("create finding with empty control ID", func(t *testing.T) {
		finding := &Finding{
			Title: "Invalid Finding",
		}
		err := engine.CreateFinding(finding)
		if err != ErrInvalidControlID {
			t.Fatalf("expected ErrInvalidControlID, got %v", err)
		}
	})

	t.Run("create finding for non-existent control", func(t *testing.T) {
		finding := &Finding{
			ControlID: "non-existent",
			Title:     "Orphan Finding",
		}
		err := engine.CreateFinding(finding)
		if err != ErrControlNotFound {
			t.Fatalf("expected ErrControlNotFound, got %v", err)
		}
	})
}

func TestResolveFinding(t *testing.T) {
	engine := setupTestEngine(t)

	framework := &ComplianceFramework{
		ID:   "soc2",
		Name: "SOC 2",
	}
	engine.RegisterFramework(framework)

	control := &Control{
		ID:          "ctrl-001",
		FrameworkID: "soc2",
		Name:        "Access Control",
	}
	engine.AddControl(control)

	finding := &Finding{
		ControlID:   "ctrl-001",
		Severity:    FindingHigh,
		Title:       "Missing MFA",
		Description: "Multi-factor authentication is not enabled",
	}
	engine.CreateFinding(finding)

	t.Run("resolve existing finding", func(t *testing.T) {
		err := engine.ResolveFinding(finding.ID, "MFA enabled for all users")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if finding.Status != FindingResolved {
			t.Errorf("expected status FindingResolved, got %v", finding.Status)
		}
		if finding.ResolvedAt.IsZero() {
			t.Fatal("expected ResolvedAt to be set")
		}
	})

	t.Run("resolve non-existent finding", func(t *testing.T) {
		err := engine.ResolveFinding("non-existent", "resolution")
		if err != ErrFindingNotFound {
			t.Fatalf("expected ErrFindingNotFound, got %v", err)
		}
	})

	t.Run("resolve already resolved finding", func(t *testing.T) {
		err := engine.ResolveFinding(finding.ID, "duplicate resolution")
		if err != ErrFindingAlreadyResolved {
			t.Fatalf("expected ErrFindingAlreadyResolved, got %v", err)
		}
	})

	t.Run("resolve with empty finding ID", func(t *testing.T) {
		err := engine.ResolveFinding("", "resolution")
		if err != ErrInvalidFindingID {
			t.Fatalf("expected ErrInvalidFindingID, got %v", err)
		}
	})
}

func TestGenerateReport(t *testing.T) {
	engine := setupTestEngine(t)

	framework := &ComplianceFramework{
		ID:      "gdpr",
		Name:    "GDPR",
		Type:    FrameworkGDPR,
		Version: "2018",
	}
	engine.RegisterFramework(framework)

	control := &Control{
		ID:          "ctrl-001",
		FrameworkID: "gdpr",
		Name:        "Data Protection",
	}
	control.Status = ControlImplemented
	engine.AddControl(control)

	t.Run("generate compliance report", func(t *testing.T) {
		report, err := engine.GenerateReport("gdpr", ReportTypeCompliance)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report == nil {
			t.Fatal("expected report to be non-nil")
		}
		if report.Type != ReportTypeCompliance {
			t.Errorf("expected report type Compliance, got %v", report.Type)
		}
		if report.PublishedAt.IsZero() {
			t.Fatal("expected PublishedAt to be set")
		}
	})

	t.Run("generate executive report", func(t *testing.T) {
		report, err := engine.GenerateReport("gdpr", ReportTypeExecutive)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Type != ReportTypeExecutive {
			t.Errorf("expected report type Executive, got %v", report.Type)
		}
	})

	t.Run("generate report for non-existent framework", func(t *testing.T) {
		_, err := engine.GenerateReport("non-existent", ReportTypeCompliance)
		if err != ErrFrameworkNotFound {
			t.Fatalf("expected ErrFrameworkNotFound, got %v", err)
		}
	})
}

func TestSearchAuditLogs(t *testing.T) {
	engine := setupTestEngine(t)

	for i := 0; i < 10; i++ {
		entry := &AuditEntry{
			EventType:    EventAccess,
			Actor:        "user123",
			ActorType:    ActorUser,
			Resource:     "/api/data",
			ResourceType: "endpoint",
			Action:       "GET",
			Result:       ActionSuccess,
			IPAddress:    "192.168.1.1",
		}
		engine.RecordAuditEvent(entry)
	}

	for i := 0; i < 5; i++ {
		entry := &AuditEntry{
			EventType:    EventModify,
			Actor:        "admin456",
			ActorType:    ActorUser,
			Resource:     "/api/config",
			ResourceType: "endpoint",
			Action:       "PUT",
			Result:       ActionSuccess,
			IPAddress:    "192.168.1.2",
		}
		engine.RecordAuditEvent(entry)
	}

	t.Run("search with nil filter", func(t *testing.T) {
		_, err := engine.SearchAuditLogs(nil)
		if err != ErrNilFilter {
			t.Fatalf("expected ErrNilFilter, got %v", err)
		}
	})

	t.Run("search by actor", func(t *testing.T) {
		filter := &AuditLogFilter{
			Actor:     "user123",
			EventType: EventTypeUnset,
		}
		results, err := engine.SearchAuditLogs(filter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 10 {
			t.Errorf("expected 10 results, got %d", len(results))
		}
	})

	t.Run("search by event type", func(t *testing.T) {
		filter := &AuditLogFilter{
			EventType: EventModify,
		}
		results, err := engine.SearchAuditLogs(filter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 5 {
			t.Errorf("expected 5 results, got %d", len(results))
		}
	})

	t.Run("search with limit", func(t *testing.T) {
		filter := &AuditLogFilter{
			EventType: EventTypeUnset,
			Limit:     3,
		}
		results, err := engine.SearchAuditLogs(filter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 3 {
			t.Errorf("expected 3 results, got %d", len(results))
		}
	})

	t.Run("search by IP address", func(t *testing.T) {
		filter := &AuditLogFilter{
			EventType: EventTypeUnset,
			IPAddress: "192.168.1.2",
		}
		results, err := engine.SearchAuditLogs(filter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 5 {
			t.Errorf("expected 5 results, got %d", len(results))
		}
	})

	t.Run("search by time range", func(t *testing.T) {
		now := time.Now()
		filter := &AuditLogFilter{
			EventType: EventTypeUnset,
			StartTime: now.Add(-1 * time.Hour),
			EndTime:   now.Add(1 * time.Hour),
		}
		results, err := engine.SearchAuditLogs(filter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 15 {
			t.Errorf("expected 15 results, got %d", len(results))
		}
	})
}

func TestGetMetrics(t *testing.T) {
	engine := setupTestEngine(t)

	framework := &ComplianceFramework{
		ID:   "soc2",
		Name: "SOC 2",
	}
	engine.RegisterFramework(framework)

	control := &Control{
		ID:          "ctrl-001",
		FrameworkID: "soc2",
		Name:        "Access Control",
	}
	engine.AddControl(control)

	finding := &Finding{
		ControlID: "ctrl-001",
		Severity:  FindingHigh,
		Title:     "Missing MFA",
	}
	engine.CreateFinding(finding)

	entry := &AuditEntry{
		EventType: EventAccess,
		Actor:     "user123",
		ActorType: ActorUser,
		Resource:  "/api/data",
		Action:    "GET",
		Result:    ActionSuccess,
	}
	engine.RecordAuditEvent(entry)

	t.Run("get metrics", func(t *testing.T) {
		metrics := engine.GetMetrics()
		if metrics == nil {
			t.Fatal("expected metrics to be non-nil")
		}
		if metrics.AuditTrailSize < 1 {
			t.Errorf("expected at least 1 audit entry, got %d", metrics.AuditTrailSize)
		}
		if metrics.OpenFindings != 1 {
			t.Errorf("expected 1 open finding, got %d", metrics.OpenFindings)
		}
	})
}

func TestComplianceAssessor(t *testing.T) {
	t.Run("initial state", func(t *testing.T) {
		assessor := &ComplianceAssessor{
			rules:    make(map[string]*AssessmentRule),
			accuracy: 1.0,
		}
		if assessor.accuracy != 1.0 {
			t.Errorf("expected accuracy 1.0, got %f", assessor.accuracy)
		}
		if len(assessor.rules) != 0 {
			t.Errorf("expected 0 rules, got %d", len(assessor.rules))
		}
	})
}

func TestEnumTypes(t *testing.T) {
	t.Run("FrameworkType values", func(t *testing.T) {
		if FrameworkSOC2 != 0 {
			t.Errorf("expected FrameworkSOC2 to be 0, got %d", FrameworkSOC2)
		}
		if FrameworkISO27001 != 1 {
			t.Errorf("expected FrameworkISO27001 to be 1, got %d", FrameworkISO27001)
		}
		if FrameworkGDPR != 2 {
			t.Errorf("expected FrameworkGDPR to be 2, got %d", FrameworkGDPR)
		}
		if FrameworkHIPAA != 3 {
			t.Errorf("expected FrameworkHIPAA to be 3, got %d", FrameworkHIPAA)
		}
		if FrameworkPCIDSS != 4 {
			t.Errorf("expected FrameworkPCIDSS to be 4, got %d", FrameworkPCIDSS)
		}
		if FrameworkCustom != 5 {
			t.Errorf("expected FrameworkCustom to be 5, got %d", FrameworkCustom)
		}
	})

	t.Run("ComplianceStatus values", func(t *testing.T) {
		if ComplianceCompliant != 0 {
			t.Errorf("expected ComplianceCompliant to be 0, got %d", ComplianceCompliant)
		}
		if CompliancePartial != 1 {
			t.Errorf("expected CompliancePartial to be 1, got %d", CompliancePartial)
		}
		if ComplianceNonCompliant != 2 {
			t.Errorf("expected ComplianceNonCompliant to be 2, got %d", ComplianceNonCompliant)
		}
		if ComplianceUnknown != 3 {
			t.Errorf("expected ComplianceUnknown to be 3, got %d", ComplianceUnknown)
		}
	})

	t.Run("FindingSeverity values", func(t *testing.T) {
		if FindingLow != 0 {
			t.Errorf("expected FindingLow to be 0, got %d", FindingLow)
		}
		if FindingMedium != 1 {
			t.Errorf("expected FindingMedium to be 1, got %d", FindingMedium)
		}
		if FindingHigh != 2 {
			t.Errorf("expected FindingHigh to be 2, got %d", FindingHigh)
		}
		if FindingCritical != 3 {
			t.Errorf("expected FindingCritical to be 3, got %d", FindingCritical)
		}
	})

	t.Run("EventType values", func(t *testing.T) {
		if EventAccess != 0 {
			t.Errorf("expected EventAccess to be 0, got %d", EventAccess)
		}
		if EventModify != 1 {
			t.Errorf("expected EventModify to be 1, got %d", EventModify)
		}
		if EventDelete != 2 {
			t.Errorf("expected EventDelete to be 2, got %d", EventDelete)
		}
	})

	t.Run("ActionResult values", func(t *testing.T) {
		if ActionSuccess != 0 {
			t.Errorf("expected ActionSuccess to be 0, got %d", ActionSuccess)
		}
		if ActionFailure != 1 {
			t.Errorf("expected ActionFailure to be 1, got %d", ActionFailure)
		}
		if ActionDenied != 2 {
			t.Errorf("expected ActionDenied to be 2, got %d", ActionDenied)
		}
	})
}

func TestEndToEnd(t *testing.T) {
	engine := setupTestEngine(t)

	// 1. 注册框架
	framework := &ComplianceFramework{
		ID:      "soc2",
		Name:    "SOC 2 Type II",
		Type:    FrameworkSOC2,
		Version: "2017",
	}
	if err := engine.RegisterFramework(framework); err != nil {
		t.Fatalf("failed to register framework: %v", err)
	}

	// 2. 添加控制项
	controls := []*Control{
		{ID: "ctrl-001", FrameworkID: "soc2", Name: "Access Control", Category: "Security"},
		{ID: "ctrl-002", FrameworkID: "soc2", Name: "Encryption", Category: "Security"},
		{ID: "ctrl-003", FrameworkID: "soc2", Name: "Logging", Category: "Monitoring"},
	}

	for _, control := range controls {
		if err := engine.AddControl(control); err != nil {
			t.Fatalf("failed to add control %s: %v", control.ID, err)
		}
	}

	// 3. 记录审计事件
	entry := &AuditEntry{
		EventType: EventAuth,
		Actor:     "admin",
		ActorType: ActorUser,
		Resource:  "/login",
		Action:    "POST",
		Result:    ActionSuccess,
	}
	if err := engine.RecordAuditEvent(entry); err != nil {
		t.Fatalf("failed to record audit event: %v", err)
	}

	// 4. 创建发现
	finding := &Finding{
		ControlID:   "ctrl-001",
		Severity:    FindingHigh,
		Title:       "Missing MFA",
		Description: "MFA not enabled for admin users",
		Remediation: "Enable MFA for all admin accounts",
	}
	if err := engine.CreateFinding(finding); err != nil {
		t.Fatalf("failed to create finding: %v", err)
	}

	// 5. 运行评估
	report, err := engine.RunAssessment("soc2")
	if err != nil {
		t.Fatalf("failed to run assessment: %v", err)
	}
	if report.Score < 0 || report.Score > 100 {
		t.Errorf("invalid score: %f", report.Score)
	}

	// 6. 解决发现
	if err := engine.ResolveFinding(finding.ID, "MFA enabled"); err != nil {
		t.Fatalf("failed to resolve finding: %v", err)
	}

	// 7. 生成报告
	finalReport, err := engine.GenerateReport("soc2", ReportTypeExecutive)
	if err != nil {
		t.Fatalf("failed to generate report: %v", err)
	}
	if finalReport.Score < 0 || finalReport.Score > 100 {
		t.Errorf("invalid final score: %f", finalReport.Score)
	}

	// 8. 搜索审计日志
	filter := &AuditLogFilter{
		Actor:     "admin",
		EventType: EventTypeUnset,
	}
	logs, err := engine.SearchAuditLogs(filter)
	if err != nil {
		t.Fatalf("failed to search audit logs: %v", err)
	}
	if len(logs) == 0 {
		t.Error("expected to find audit logs")
	}

	// 9. 获取指标
	metrics := engine.GetMetrics()
	if metrics == nil {
		t.Fatal("expected metrics to be non-nil")
	}
	if metrics.OpenFindings != 0 {
		t.Errorf("expected 0 open findings after resolution, got %d", metrics.OpenFindings)
	}
	if metrics.ResolvedFindings != 1 {
		t.Errorf("expected 1 resolved finding, got %d", metrics.ResolvedFindings)
	}
}
