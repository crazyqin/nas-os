package forensics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogger implements Logger interface for testing.
type mockLogger struct{}

func (l *mockLogger) Info(msg string, keysAndValues ...interface{})  {}
func (l *mockLogger) Error(msg string, keysAndValues ...interface{}) {}
func (l *mockLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (l *mockLogger) Debug(msg string, keysAndValues ...interface{}) {}

func setupTestManager(t *testing.T) (*Manager, string) {
	tmpDir := t.TempDir()
	config := Config{
		StoragePath:      filepath.Join(tmpDir, "forensics"),
		EvidenceHashAlgo: "sha256",
		TimelineMaxSize:  1000,
		MaxCaseAge:       365 * 24 * time.Hour,
	}

	manager, err := NewManager(config, &mockLogger{})
	require.NoError(t, err)

	return manager, tmpDir
}

func TestCreateCase(t *testing.T) {
	manager, _ := setupTestManager(t)

	case_, err := manager.CreateCase(
		"Test Case",
		"Test investigation",
		"investigator1",
		PriorityHigh,
		[]string{"malware", "incident"},
	)

	require.NoError(t, err)
	assert.NotEmpty(t, case_.ID)
	assert.Equal(t, "Test Case", case_.Name)
	assert.Equal(t, CaseStatusOpen, case_.Status)
	assert.Equal(t, PriorityHigh, case_.Priority)
	assert.NotEmpty(t, case_.TimelineID)
}

func TestGetCase(t *testing.T) {
	manager, _ := setupTestManager(t)

	// Create a case
	created, err := manager.CreateCase(
		"Get Test",
		"Testing get",
		"inv1",
		PriorityMedium,
		nil,
	)
	require.NoError(t, err)

	// Get the case
	retrieved, err := manager.GetCase(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, created.Name, retrieved.Name)

	// Try to get non-existent case
	_, err = manager.GetCase("nonexistent")
	assert.Error(t, err)
}

func TestListCases(t *testing.T) {
	manager, _ := setupTestManager(t)

	// Create multiple cases
	_, err := manager.CreateCase("Case 1", "Desc 1", "inv1", PriorityLow, nil)
	require.NoError(t, err)
	_, err = manager.CreateCase("Case 2", "Desc 2", "inv2", PriorityHigh, nil)
	require.NoError(t, err)
	_, err = manager.CreateCase("Case 3", "Desc 3", "inv3", PriorityCritical, nil)
	require.NoError(t, err)

	// List all cases
	cases := manager.ListCases("")
	assert.Len(t, cases, 3)

	// List by priority (status filter)
	cases = manager.ListCases(CaseStatusOpen)
	assert.Len(t, cases, 3)
}

func TestUpdateCaseStatus(t *testing.T) {
	manager, _ := setupTestManager(t)

	case_, err := manager.CreateCase(
		"Status Test",
		"Testing status update",
		"inv1",
		PriorityMedium,
		nil,
	)
	require.NoError(t, err)

	// Update status
	err = manager.UpdateCaseStatus(case_.ID, CaseStatusInProgress)
	require.NoError(t, err)

	// Verify update
	updated, err := manager.GetCase(case_.ID)
	require.NoError(t, err)
	assert.Equal(t, CaseStatusInProgress, updated.Status)

	// Close the case
	err = manager.UpdateCaseStatus(case_.ID, CaseStatusClosed)
	require.NoError(t, err)

	updated, err = manager.GetCase(case_.ID)
	require.NoError(t, err)
	assert.Equal(t, CaseStatusClosed, updated.Status)
	assert.NotNil(t, updated.ClosedAt)
}

func TestAddNote(t *testing.T) {
	manager, _ := setupTestManager(t)

	case_, err := manager.CreateCase(
		"Note Test",
		"Testing notes",
		"inv1",
		PriorityMedium,
		nil,
	)
	require.NoError(t, err)

	// Add note
	note, err := manager.AddNote(case_.ID, "investigator", "Found suspicious activity")
	require.NoError(t, err)
	assert.NotEmpty(t, note.ID)
	assert.Equal(t, "investigator", note.Author)
	assert.Equal(t, "Found suspicious activity", note.Content)

	// Verify note was added
	updated, err := manager.GetCase(case_.ID)
	require.NoError(t, err)
	assert.Len(t, updated.Notes, 1)
}

func TestAddFinding(t *testing.T) {
	manager, _ := setupTestManager(t)

	case_, err := manager.CreateCase(
		"Finding Test",
		"Testing findings",
		"inv1",
		PriorityMedium,
		nil,
	)
	require.NoError(t, err)

	// Add finding
	finding, err := manager.AddFinding(
		case_.ID,
		"Malware Detected",
		"Found trojan in system directory",
		"malware",
		PriorityCritical,
		[]string{"ev-1", "ev-2"},
	)
	require.NoError(t, err)
	assert.NotEmpty(t, finding.ID)
	assert.Equal(t, "Malware Detected", finding.Title)
	assert.Equal(t, PriorityCritical, finding.Severity)

	// Verify finding was added
	updated, err := manager.GetCase(case_.ID)
	require.NoError(t, err)
	assert.Len(t, updated.Findings, 1)
}

func TestCollectFileEvidence(t *testing.T) {
	manager, tmpDir := setupTestManager(t)

	// Create a test file
	testFile := filepath.Join(tmpDir, "testfile.txt")
	err := os.WriteFile(testFile, []byte("test content for forensics"), 0644)
	require.NoError(t, err)

	// Create a case
	case_, err := manager.CreateCase(
		"Evidence Test",
		"Testing evidence collection",
		"inv1",
		PriorityMedium,
		nil,
	)
	require.NoError(t, err)

	// Collect evidence
	evidence, err := manager.CollectFileEvidence(
		case_.ID,
		testFile,
		"Test file evidence",
		"investigator",
	)
	require.NoError(t, err)
	assert.NotEmpty(t, evidence.ID)
	assert.Equal(t, EvidenceTypeFile, evidence.Type)
	assert.NotEmpty(t, evidence.Hash)
	assert.Len(t, evidence.ChainOfCustody, 1)

	// Verify evidence integrity
	valid, err := manager.VerifyEvidence(evidence.ID)
	require.NoError(t, err)
	assert.True(t, valid)

	// List evidence for case
	evidenceList := manager.ListEvidence(case_.ID)
	assert.Len(t, evidenceList, 1)
}

func TestCollectLogEvidence(t *testing.T) {
	manager, _ := setupTestManager(t)

	// Create a case
	case_, err := manager.CreateCase(
		"Log Test",
		"Testing log evidence",
		"inv1",
		PriorityMedium,
		nil,
	)
	require.NoError(t, err)

	// Create log entries
	logEntries := []map[string]string{
		{"timestamp": "2026-05-30T10:00:00Z", "level": "ERROR", "message": "Failed login attempt"},
		{"timestamp": "2026-05-30T10:01:00Z", "level": "WARN", "message": "Suspicious activity"},
	}

	// Collect log evidence
	evidence, err := manager.CollectLogEvidence(
		case_.ID,
		logEntries,
		"Security logs",
		"investigator",
	)
	require.NoError(t, err)
	assert.NotEmpty(t, evidence.ID)
	assert.Equal(t, EvidenceTypeLog, evidence.Type)
	assert.Greater(t, evidence.Size, int64(0))
}

func TestTimeline(t *testing.T) {
	manager, _ := setupTestManager(t)

	// Create a case
	case_, err := manager.CreateCase(
		"Timeline Test",
		"Testing timeline",
		"inv1",
		PriorityMedium,
		nil,
	)
	require.NoError(t, err)

	// Get timeline
	timeline, err := manager.GetCaseTimeline(case_.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, timeline.ID)

	// Add events
	event1 := Event{
		Timestamp:   time.Now().Add(-2 * time.Hour),
		Type:        EventLogin,
		Category:    "authentication",
		Description: "User login",
		Source:      "auth.log",
		Severity:    PriorityLow,
		Actor:       "admin",
	}

	event2 := Event{
		Timestamp:   time.Now().Add(-1 * time.Hour),
		Type:        EventFileAccess,
		Category:    "filesystem",
		Description: "Sensitive file accessed",
		Source:      "audit.log",
		Severity:    PriorityHigh,
		Actor:       "admin",
		Target:      "/etc/passwd",
	}

	err = manager.AddTimelineEvent(timeline.ID, event1)
	require.NoError(t, err)

	err = manager.AddTimelineEvent(timeline.ID, event2)
	require.NoError(t, err)

	// Verify events
	updated, err := manager.GetCaseTimeline(case_.ID)
	require.NoError(t, err)
	assert.Len(t, updated.Events, 2)
	assert.True(t, updated.Events[0].Timestamp.Before(updated.Events[1].Timestamp))
}

func TestAnalyzeDirectory(t *testing.T) {
	manager, tmpDir := setupTestManager(t)

	// Create test files
	testDir := filepath.Join(tmpDir, "testdir")
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(testDir, "file1.txt"), []byte("content1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(testDir, "file2.txt"), []byte("content2"), 0644)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(testDir, "subdir"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(testDir, "subdir", "file3.txt"), []byte("content3"), 0644)
	require.NoError(t, err)

	// Analyze non-recursive
	files, err := manager.AnalyzeDirectory(testDir, false)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(files), 3) // at least 2 files + 1 dir

	// Analyze recursive
	files, err = manager.AnalyzeDirectory(testDir, true)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(files), 4) // at least 3 files + 1 subdir
}

func TestFindModifiedFiles(t *testing.T) {
	manager, tmpDir := setupTestManager(t)

	// Create test file
	testFile := filepath.Join(tmpDir, "modified.txt")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	// Find files modified in the last hour
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now().Add(1 * time.Hour)

	files, err := manager.FindModifiedFiles(tmpDir, start, end)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, "modified.txt", files[0].Name)
}

func TestTransferCustody(t *testing.T) {
	manager, tmpDir := setupTestManager(t)

	// Create test file
	testFile := filepath.Join(tmpDir, "evidence.txt")
	err := os.WriteFile(testFile, []byte("evidence content"), 0644)
	require.NoError(t, err)

	// Create case and collect evidence
	case_, err := manager.CreateCase(
		"Custody Test",
		"Testing custody transfer",
		"inv1",
		PriorityMedium,
		nil,
	)
	require.NoError(t, err)

	evidence, err := manager.CollectFileEvidence(
		case_.ID,
		testFile,
		"Test evidence",
		"officer1",
	)
	require.NoError(t, err)

	// Transfer custody
	err = manager.TransferCustody(
		evidence.ID,
		"officer1",
		"officer2",
		"lab",
		"Transfer for analysis",
	)
	require.NoError(t, err)

	// Verify custody chain
	updated, err := manager.GetEvidence(evidence.ID)
	require.NoError(t, err)
	assert.Len(t, updated.ChainOfCustody, 2)
	assert.Equal(t, "transferred", updated.ChainOfCustody[1].Action)
	assert.Equal(t, "officer2", updated.ChainOfCustody[1].Officer)
}

func TestGenerateReport(t *testing.T) {
	manager, _ := setupTestManager(t)

	// Create case with evidence
	case_, err := manager.CreateCase(
		"Report Test",
		"Testing report generation",
		"inv1",
		PriorityHigh,
		[]string{"malware"},
	)
	require.NoError(t, err)

	// Add a finding
	_, err = manager.AddFinding(
		case_.ID,
		"Security Breach",
		"Unauthorized access detected",
		"security",
		PriorityCritical,
		nil,
	)
	require.NoError(t, err)

	// Generate report
	report, err := manager.GenerateReport(case_.ID, "investigator", "json")
	require.NoError(t, err)
	assert.NotEmpty(t, report.ID)
	assert.Equal(t, case_.ID, report.CaseID)
	assert.NotEmpty(t, report.FilePath)

	// Verify report file exists
	_, err = os.Stat(report.FilePath)
	assert.NoError(t, err)
}

func TestStats(t *testing.T) {
	manager, _ := setupTestManager(t)

	// Create some cases
	_, err := manager.CreateCase("Case 1", "Desc", "inv1", PriorityLow, nil)
	require.NoError(t, err)
	_, err = manager.CreateCase("Case 2", "Desc", "inv2", PriorityHigh, nil)
	require.NoError(t, err)

	// Get stats
	stats := manager.Stats()
	assert.Equal(t, 2, stats["totalCases"])
	assert.Equal(t, 0, stats["totalEvidence"])
}

func TestDeleteCase(t *testing.T) {
	manager, _ := setupTestManager(t)

	// Create case
	case_, err := manager.CreateCase(
		"Delete Test",
		"Testing deletion",
		"inv1",
		PriorityMedium,
		nil,
	)
	require.NoError(t, err)

	// Delete case
	err = manager.DeleteCase(case_.ID)
	require.NoError(t, err)

	// Verify deletion
	_, err = manager.GetCase(case_.ID)
	assert.Error(t, err)
}

func TestJSONSerialization(t *testing.T) {
	case_ := &Case{
		ID:          "test-123",
		Name:        "Test Case",
		Description: "Test description",
		Status:      CaseStatusOpen,
		Priority:    PriorityHigh,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	data, err := json.Marshal(case_)
	require.NoError(t, err)

	var decoded Case
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, case_.ID, decoded.ID)
	assert.Equal(t, case_.Name, decoded.Name)
	assert.Equal(t, case_.Status, decoded.Status)
}
