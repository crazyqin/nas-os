package unifiedhealthhub

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

func TestNewHealthHub(t *testing.T) {
	logger := newTestLogger()

	// Test with nil config
	hub := NewHealthHub(nil, logger)
	if hub == nil {
		t.Fatal("Expected non-nil hub")
	}
	if hub.config == nil {
		t.Fatal("Expected default config")
	}
	if hub.config.CheckInterval != 30*time.Second {
		t.Errorf("Expected check interval 30s, got %v", hub.config.CheckInterval)
	}
	hub.Stop()

	// Test with custom config
	config := &HubConfig{
		CheckInterval:     10 * time.Second,
		AlertCooldown:     2 * time.Minute,
		AutoIncident:      false,
		PredictionEnabled: false,
		RetentionDays:     7,
	}
	hub = NewHealthHub(config, logger)
	if hub.config.CheckInterval != 10*time.Second {
		t.Errorf("Expected check interval 10s, got %v", hub.config.CheckInterval)
	}
	hub.Stop()

	// Test with nil logger
	hub = NewHealthHub(nil, nil)
	if hub == nil {
		t.Fatal("Expected non-nil hub with nil logger")
	}
	hub.Stop()
}

func TestRegisterSubsystem(t *testing.T) {
	hub := NewHealthHub(nil, newTestLogger())
	defer hub.Stop()

	// Test successful registration
	sub := &Subsystem{
		ID:   "storage-1",
		Name: "Main Storage",
		Type: SubsystemStorage,
	}
	err := hub.RegisterSubsystem(sub)
	if err != nil {
		t.Fatalf("Failed to register subsystem: %v", err)
	}

	// Verify registration
	hub.mu.RLock()
	registered, exists := hub.subsystems["storage-1"]
	hub.mu.RUnlock()
	if !exists {
		t.Fatal("Subsystem not found after registration")
	}
	if registered.Name != "Main Storage" {
		t.Errorf("Expected name 'Main Storage', got '%s'", registered.Name)
	}
	if registered.Status != HealthUnknown {
		t.Errorf("Expected initial status Unknown, got %v", registered.Status)
	}

	// Test duplicate registration
	err = hub.RegisterSubsystem(sub)
	if err != ErrSubsystemExists {
		t.Errorf("Expected ErrSubsystemExists, got %v", err)
	}

	// Test nil subsystem
	err = hub.RegisterSubsystem(nil)
	if err == nil {
		t.Error("Expected error for nil subsystem")
	}

	// Test auto-generated ID
	sub2 := &Subsystem{
		Name: "Network",
		Type: SubsystemNetwork,
	}
	err = hub.RegisterSubsystem(sub2)
	if err != nil {
		t.Fatalf("Failed to register subsystem with auto ID: %v", err)
	}
	if sub2.ID == "" {
		t.Error("Expected auto-generated ID")
	}
}

func TestRunHealthCheck(t *testing.T) {
	hub := NewHealthHub(nil, newTestLogger())
	defer hub.Stop()

	// Register a subsystem first
	sub := &Subsystem{
		ID:   "test-sub",
		Name: "Test Subsystem",
		Type: SubsystemService,
	}
	hub.RegisterSubsystem(sub)

	// Test successful health check
	check := &HealthCheck{
		ID:      "check-1",
		Name:    "CPU Check",
		Type:    CheckTypeMetric,
		Status:  CheckPass,
		Message: "CPU usage normal",
		Value:   45.0,
	}
	result, err := hub.RunHealthCheck("test-sub", check)
	if err != nil {
		t.Fatalf("Failed to run health check: %v", err)
	}
	if result.ID != "check-1" {
		t.Errorf("Expected check ID 'check-1', got '%s'", result.ID)
	}
	if result.LastRun.IsZero() {
		t.Error("Expected LastRun to be set")
	}

	// Test with non-existent subsystem
	_, err = hub.RunHealthCheck("non-existent", check)
	if err != ErrSubsystemNotFound {
		t.Errorf("Expected ErrSubsystemNotFound, got %v", err)
	}

	// Test with nil check
	_, err = hub.RunHealthCheck("test-sub", nil)
	if err == nil {
		t.Error("Expected error for nil check")
	}

	// Test multiple checks affecting score
	check2 := &HealthCheck{
		ID:     "check-2",
		Name:   "Memory Check",
		Type:   CheckTypeMetric,
		Status: CheckFail,
	}
	hub.RunHealthCheck("test-sub", check2)

	hub.mu.RLock()
	updatedSub := hub.subsystems["test-sub"]
	hub.mu.RUnlock()

	if updatedSub.Score >= 100 {
		t.Error("Expected score to decrease after failed check")
	}
	if updatedSub.Status == HealthHealthy {
		t.Error("Expected status to not be healthy after failed check")
	}
}

func TestRaiseAlert(t *testing.T) {
	config := &HubConfig{
		AutoIncident:      true,
		PredictionEnabled: true,
	}
	hub := NewHealthHub(config, newTestLogger())
	defer hub.Stop()

	// Register subsystem
	sub := &Subsystem{ID: "sub-1", Name: "Test", Type: SubsystemService}
	hub.RegisterSubsystem(sub)

	// Test raising info alert (no auto-incident)
	alert := &HealthAlert{
		Subsystem: "sub-1",
		Level:     AlertInfo,
		Title:     "Test Alert",
		Message:   "This is a test",
		Source:    "test",
	}
	result, err := hub.RaiseAlert(alert)
	if err != nil {
		t.Fatalf("Failed to raise alert: %v", err)
	}
	if result.ID == "" {
		t.Error("Expected alert ID to be generated")
	}

	// Verify no auto-incident for info alert
	hub.mu.RLock()
	incidentCount := len(hub.incidents)
	hub.mu.RUnlock()
	if incidentCount != 0 {
		t.Errorf("Expected 0 incidents for info alert, got %d", incidentCount)
	}

	// Test raising critical alert (should auto-create incident)
	criticalAlert := &HealthAlert{
		Subsystem: "sub-1",
		Level:     AlertCritical,
		Title:     "Critical Alert",
		Message:   "System failing",
		Source:    "test",
	}
	_, err = hub.RaiseAlert(criticalAlert)
	if err != nil {
		t.Fatalf("Failed to raise critical alert: %v", err)
	}

	hub.mu.RLock()
	incidentCount = len(hub.incidents)
	hub.mu.RUnlock()
	if incidentCount != 1 {
		t.Errorf("Expected 1 auto-incident for critical alert, got %d", incidentCount)
	}

	// Test with non-existent subsystem
	badAlert := &HealthAlert{
		Subsystem: "non-existent",
		Level:     AlertWarning,
		Title:     "Bad",
	}
	_, err = hub.RaiseAlert(badAlert)
	if err != ErrSubsystemNotFound {
		t.Errorf("Expected ErrSubsystemNotFound, got %v", err)
	}

	// Test nil alert
	_, err = hub.RaiseAlert(nil)
	if err == nil {
		t.Error("Expected error for nil alert")
	}
}

func TestCreateIncident(t *testing.T) {
	hub := NewHealthHub(nil, newTestLogger())
	defer hub.Stop()

	// Register subsystem and alert
	sub := &Subsystem{ID: "sub-1", Name: "Test", Type: SubsystemService}
	hub.RegisterSubsystem(sub)

	alert := &HealthAlert{
		ID:        "alert-1",
		Subsystem: "sub-1",
		Level:     AlertWarning,
		Title:     "Test Alert",
	}
	hub.mu.Lock()
	hub.alerts[alert.ID] = alert
	hub.mu.Unlock()

	// Test successful incident creation
	incident := &Incident{
		Title:      "Test Incident",
		Severity:   SeverityMedium,
		Subsystems: []string{"sub-1"},
		Alerts:     []string{"alert-1"},
	}
	result, err := hub.CreateIncident(incident)
	if err != nil {
		t.Fatalf("Failed to create incident: %v", err)
	}
	if result.ID == "" {
		t.Error("Expected incident ID to be generated")
	}
	if result.Status != IncidentOpen {
		t.Errorf("Expected status Open, got %v", result.Status)
	}
	if len(result.Timeline) != 1 {
		t.Errorf("Expected 1 timeline event, got %d", len(result.Timeline))
	}

	// Test with non-existent subsystem
	badIncident := &Incident{
		Title:      "Bad Incident",
		Subsystems: []string{"non-existent"},
	}
	_, err = hub.CreateIncident(badIncident)
	if err != ErrSubsystemNotFound {
		t.Errorf("Expected ErrSubsystemNotFound, got %v", err)
	}

	// Test with non-existent alert
	badIncident2 := &Incident{
		Title:  "Bad Incident 2",
		Alerts: []string{"non-existent"},
	}
	_, err = hub.CreateIncident(badIncident2)
	if err != ErrAlertNotFound {
		t.Errorf("Expected ErrAlertNotFound, got %v", err)
	}

	// Test nil incident
	_, err = hub.CreateIncident(nil)
	if err == nil {
		t.Error("Expected error for nil incident")
	}
}

func TestAcknowledgeAlert(t *testing.T) {
	hub := NewHealthHub(nil, newTestLogger())
	defer hub.Stop()

	// Create an alert
	alert := &HealthAlert{
		ID:    "alert-1",
		Level: AlertWarning,
		Title: "Test Alert",
	}
	hub.mu.Lock()
	hub.alerts[alert.ID] = alert
	hub.mu.Unlock()

	// Test successful acknowledgment
	err := hub.AcknowledgeAlert("alert-1", "admin")
	if err != nil {
		t.Fatalf("Failed to acknowledge alert: %v", err)
	}

	hub.mu.RLock()
	ackedAlert := hub.alerts["alert-1"]
	hub.mu.RUnlock()

	if !ackedAlert.Acked {
		t.Error("Expected alert to be acknowledged")
	}
	if ackedAlert.AckedBy != "admin" {
		t.Errorf("Expected AckedBy 'admin', got '%s'", ackedAlert.AckedBy)
	}
	if ackedAlert.AckedAt.IsZero() {
		t.Error("Expected AckedAt to be set")
	}

	// Test double acknowledgment
	err = hub.AcknowledgeAlert("alert-1", "admin2")
	if err != ErrAlertAlreadyAcked {
		t.Errorf("Expected ErrAlertAlreadyAcked, got %v", err)
	}

	// Test non-existent alert
	err = hub.AcknowledgeAlert("non-existent", "admin")
	if err != ErrAlertNotFound {
		t.Errorf("Expected ErrAlertNotFound, got %v", err)
	}
}

func TestResolveIncident(t *testing.T) {
	hub := NewHealthHub(nil, newTestLogger())
	defer hub.Stop()

	// Create an incident
	incident := &Incident{
		ID:       "inc-1",
		Title:    "Test Incident",
		Severity: SeverityHigh,
		Status:   IncidentOpen,
		Timeline: []*IncidentEvent{},
	}
	hub.mu.Lock()
	hub.incidents[incident.ID] = incident
	hub.mu.Unlock()

	// Test successful resolution
	err := hub.ResolveIncident("inc-1", "Fixed the issue")
	if err != nil {
		t.Fatalf("Failed to resolve incident: %v", err)
	}

	hub.mu.RLock()
	resolved := hub.incidents["inc-1"]
	hub.mu.RUnlock()

	if resolved.Status != IncidentResolved {
		t.Errorf("Expected status Resolved, got %v", resolved.Status)
	}
	if resolved.Resolution != "Fixed the issue" {
		t.Errorf("Expected resolution 'Fixed the issue', got '%s'", resolved.Resolution)
	}
	if resolved.ResolvedAt.IsZero() {
		t.Error("Expected ResolvedAt to be set")
	}

	// Test resolving already resolved (should still work if not closed)
	err = hub.ResolveIncident("inc-1", "Double check")
	if err != nil {
		t.Fatalf("Expected no error re-resolving, got %v", err)
	}

	// Test with closed incident
	incident2 := &Incident{
		ID:       "inc-2",
		Title:    "Closed Incident",
		Status:   IncidentClosed,
		Timeline: []*IncidentEvent{},
	}
	hub.mu.Lock()
	hub.incidents[incident2.ID] = incident2
	hub.mu.Unlock()

	err = hub.ResolveIncident("inc-2", "test")
	if err != ErrIncidentAlreadyClosed {
		t.Errorf("Expected ErrIncidentAlreadyClosed, got %v", err)
	}

	// Test non-existent incident
	err = hub.ResolveIncident("non-existent", "test")
	if err != ErrIncidentNotFound {
		t.Errorf("Expected ErrIncidentNotFound, got %v", err)
	}
}

func TestGetOverallHealth(t *testing.T) {
	hub := NewHealthHub(nil, newTestLogger())
	defer hub.Stop()

	// Test with no subsystems
	snapshot, err := hub.GetOverallHealth()
	if err != nil {
		t.Fatalf("Failed to get overall health: %v", err)
	}
	if snapshot.Overall != 100.0 {
		t.Errorf("Expected 100 for empty hub, got %f", snapshot.Overall)
	}

	// Add subsystems with different scores
	sub1 := &Subsystem{ID: "sub-1", Name: "Sub1", Score: 90.0, Status: HealthHealthy}
	sub2 := &Subsystem{ID: "sub-2", Name: "Sub2", Score: 70.0, Status: HealthDegraded}
	hub.RegisterSubsystem(sub1)
	hub.RegisterSubsystem(sub2)

	// Add an alert
	hub.mu.Lock()
	hub.alerts["alert-1"] = &HealthAlert{ID: "alert-1", Resolved: false}
	hub.mu.Unlock()

	snapshot, err = hub.GetOverallHealth()
	if err != nil {
		t.Fatalf("Failed to get overall health: %v", err)
	}

	expectedScore := (90.0 + 70.0) / 2.0
	if snapshot.Overall != expectedScore {
		t.Errorf("Expected overall score %f, got %f", expectedScore, snapshot.Overall)
	}
	if snapshot.AlertCount != 1 {
		t.Errorf("Expected 1 active alert, got %d", snapshot.AlertCount)
	}
	if len(snapshot.Subsystems) != 2 {
		t.Errorf("Expected 2 subsystem scores, got %d", len(snapshot.Subsystems))
	}

	// Verify history is recorded
	hub.mu.RLock()
	historyLen := len(hub.history)
	hub.mu.RUnlock()
	if historyLen != 1 {
		t.Errorf("Expected 1 history entry, got %d", historyLen)
	}
}

func TestPredictHealth(t *testing.T) {
	config := &HubConfig{PredictionEnabled: true}
	hub := NewHealthHub(config, newTestLogger())
	defer hub.Stop()

	// Register subsystem with some checks
	sub := &Subsystem{
		ID:     "sub-1",
		Name:   "Test",
		Status: HealthHealthy,
		Score:  95.0,
		Checks: []*HealthCheck{
			{ID: "c1", Status: CheckPass},
			{ID: "c2", Status: CheckPass},
			{ID: "c3", Status: CheckFail},
		},
	}
	hub.RegisterSubsystem(sub)

	// Test prediction
	prediction, err := hub.PredictHealth("sub-1", 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to predict health: %v", err)
	}

	if prediction.Subsystem != "sub-1" {
		t.Errorf("Expected subsystem 'sub-1', got '%s'", prediction.Subsystem)
	}
	if prediction.TimeWindow != 1*time.Hour {
		t.Errorf("Expected time window 1h, got %v", prediction.TimeWindow)
	}
	if prediction.Probability <= 0 || prediction.Probability > 1 {
		t.Errorf("Invalid probability: %f", prediction.Probability)
	}

	// Test with non-existent subsystem
	_, err = hub.PredictHealth("non-existent", 1*time.Hour)
	if err != ErrSubsystemNotFound {
		t.Errorf("Expected ErrSubsystemNotFound, got %v", err)
	}

	// Test with prediction disabled
	hub.config.PredictionEnabled = false
	_, err = hub.PredictHealth("sub-1", 1*time.Hour)
	if err != ErrPredictionDisabled {
		t.Errorf("Expected ErrPredictionDisabled, got %v", err)
	}
}

func TestGetMetrics(t *testing.T) {
	hub := NewHealthHub(nil, newTestLogger())
	defer hub.Stop()

	// Add subsystems with checks
	sub1 := &Subsystem{
		ID:   "sub-1",
		Name: "Sub1",
		Checks: []*HealthCheck{
			{Status: CheckPass},
			{Status: CheckPass},
			{Status: CheckWarn},
		},
	}
	sub2 := &Subsystem{
		ID:   "sub-2",
		Name: "Sub2",
		Checks: []*HealthCheck{
			{Status: CheckPass},
			{Status: CheckFail},
		},
	}
	hub.RegisterSubsystem(sub1)
	hub.RegisterSubsystem(sub2)

	// Add some incidents
	hub.mu.Lock()
	hub.incidents["inc-1"] = &Incident{Status: IncidentResolved}
	hub.incidents["inc-2"] = &Incident{Status: IncidentOpen}
	hub.mu.Unlock()

	metrics := hub.GetMetrics()

	// 3 pass out of 5 total checks = 60%
	expectedAvailability := 60.0
	if metrics.Availability != expectedAvailability {
		t.Errorf("Expected availability %f, got %f", expectedAvailability, metrics.Availability)
	}

	if metrics.ResolvedCount != 1 {
		t.Errorf("Expected 1 resolved incident, got %d", metrics.ResolvedCount)
	}

	if metrics.IncidentCount != 1 { // Active incidents
		t.Errorf("Expected 1 active incident, got %d", metrics.IncidentCount)
	}
}

func TestAddHealthRule(t *testing.T) {
	hub := NewHealthHub(nil, newTestLogger())
	defer hub.Stop()

	rule := &HealthRule{
		Name: "High CPU Rule",
		Condition: &RuleCondition{
			Metric:    "cpu_usage",
			Operator:  ">",
			Threshold: 90.0,
			Duration:  5 * time.Minute,
		},
		Actions: []*RuleAction{
			{
				Type: "alert",
				Parameters: map[string]interface{}{
					"level": "critical",
				},
			},
		},
		Priority: 1,
		Enabled:  true,
	}

	err := hub.AddHealthRule(rule)
	if err != nil {
		t.Fatalf("Failed to add health rule: %v", err)
	}

	if rule.ID == "" {
		t.Error("Expected rule ID to be generated")
	}

	hub.mu.RLock()
	_, exists := hub.rules[rule.ID]
	hub.mu.RUnlock()
	if !exists {
		t.Error("Rule not found after adding")
	}

	// Test nil rule
	err = hub.AddHealthRule(nil)
	if err == nil {
		t.Error("Expected error for nil rule")
	}
}

func TestSubsystemHealthCalculation(t *testing.T) {
	hub := NewHealthHub(nil, newTestLogger())
	defer hub.Stop()

	tests := []struct {
		name           string
		checks         []*HealthCheck
		expectedStatus HealthStatus
		minScore       float64
		maxScore       float64
	}{
		{
			name:           "All passing",
			checks:         []*HealthCheck{{Status: CheckPass}, {Status: CheckPass}, {Status: CheckPass}},
			expectedStatus: HealthHealthy,
			minScore:       90,
			maxScore:       100,
		},
		{
			name:           "Mixed with warnings",
			checks:         []*HealthCheck{{Status: CheckPass}, {Status: CheckWarn}, {Status: CheckPass}},
			expectedStatus: HealthDegraded,
			minScore:       70,
			maxScore:       89,
		},
		{
			name:           "Some failures",
			checks:         []*HealthCheck{{Status: CheckPass}, {Status: CheckFail}, {Status: CheckFail}},
			expectedStatus: HealthUnhealthy,
			minScore:       40,
			maxScore:       69,
		},
		{
			name:           "All failing",
			checks:         []*HealthCheck{{Status: CheckFail}, {Status: CheckFail}, {Status: CheckFail}},
			expectedStatus: HealthCritical,
			minScore:       0,
			maxScore:       39,
		},
		{
			name:           "No checks",
			checks:         []*HealthCheck{},
			expectedStatus: HealthUnknown,
			minScore:       100,
			maxScore:       100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &Subsystem{
				ID:     "test-" + tt.name,
				Name:   "Test",
				Checks: tt.checks,
			}

			score, status := hub.calculateSubsystemHealth(sub)

			if status != tt.expectedStatus {
				t.Errorf("Expected status %v, got %v", tt.expectedStatus, status)
			}
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("Expected score between %f and %f, got %f", tt.minScore, tt.maxScore, score)
			}
		})
	}
}

func TestAlertLevelToIncidentSeverity(t *testing.T) {
	hub := NewHealthHub(nil, newTestLogger())
	defer hub.Stop()

	tests := []struct {
		level    AlertLevel
		expected IncidentSeverity
	}{
		{AlertInfo, SeverityLow},
		{AlertWarning, SeverityMedium},
		{AlertCritical, SeverityHigh},
		{AlertEmergency, SeverityCritical},
	}

	for _, tt := range tests {
		result := hub.alertLevelToIncidentSeverity(tt.level)
		if result != tt.expected {
			t.Errorf("For alert level %v, expected severity %v, got %v", tt.level, tt.expected, result)
		}
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID("test")
	id2 := generateID("test")

	if id1 == "" {
		t.Error("Expected non-empty ID")
	}
	if id1 == id2 {
		t.Error("Expected unique IDs")
	}
	if id1[:4] != "test" {
		t.Errorf("Expected ID to start with 'test', got '%s'", id1[:4])
	}
}
