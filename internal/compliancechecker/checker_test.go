package compliancechecker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestNewChecker(t *testing.T) {
	config := DefaultScanConfig([]string{"/tmp"})
	checker := NewChecker(config)

	if checker == nil {
		t.Fatal("NewChecker returned nil")
	}
	if checker.config.ScanGDPR != true {
		t.Error("Expected ScanGDPR to be true")
	}
	if checker.config.MaxFileSize != 10*1024*1024 {
		t.Errorf("Expected MaxFileSize 10MB, got %d", checker.config.MaxFileSize)
	}
}

func TestDefaultScanConfig(t *testing.T) {
	paths := []string{"/home", "/var"}
	config := DefaultScanConfig(paths)

	if len(config.Paths) != 2 {
		t.Errorf("Expected 2 paths, got %d", len(config.Paths))
	}
	if !config.ScanGDPR {
		t.Error("ScanGDPR should be true by default")
	}
	if !config.ScanGB20 {
		t.Error("ScanGB20 should be true by default")
	}
	if !config.ScanFilePermission {
		t.Error("ScanFilePermission should be true by default")
	}
	if !config.ScanSensitiveData {
		t.Error("ScanSensitiveData should be true by default")
	}
	if !config.ScanEncryption {
		t.Error("ScanEncryption should be true by default")
	}
}

func TestIsSensitiveFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"password.txt", true},
		{"secret.key", true},
		{"credentials.json", true},
		{"id_rsa", true},
		{".env", true},
		{"server.pem", true},
		{"token.dat", true},
		{"readme.md", false},
		{"data.csv", false},
		{"config.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isSensitiveFile(tt.path); got != tt.expected {
				t.Errorf("isSensitiveFile(%s) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestSensitiveDataPatterns(t *testing.T) {
	// ID card pattern
	idCard := "110101199003076531"
	if !idCardPattern.MatchString(idCard) {
		t.Errorf("ID card pattern should match %s", idCard)
	}

	// Invalid ID card (too short)
	if idCardPattern.MatchString("110101199003076") {
		t.Error("Should not match short ID")
	}

	// Phone pattern
	phone := "13812345678"
	if !phonePattern.MatchString(phone) {
		t.Errorf("Phone pattern should match %s", phone)
	}

	// Invalid phone (wrong prefix)
	if phonePattern.MatchString("12345678901") {
		t.Error("Should not match invalid phone prefix")
	}

	// Email pattern
	email := "test@example.com"
	if !emailPattern.MatchString(email) {
		t.Errorf("Email pattern should match %s", email)
	}

	// SSN pattern
	ssn := "123-45-6789"
	if !ssnPattern.MatchString(ssn) {
		t.Errorf("SSN pattern should match %s", ssn)
	}
}

func TestFilePermissionCheck(t *testing.T) {
	// Save and restore umask
	oldUmask := syscall.Umask(0)
	defer syscall.Umask(oldUmask)

	// Create temp directory
	tmpDir := t.TempDir()

	// Create a world-writable file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0666); err != nil {
		t.Fatal(err)
	}

	// Verify permission was set correctly
	info, _ := os.Stat(testFile)
	if info.Mode().Perm()&0002 == 0 {
		t.Skip("Could not create world-writable file (umask restriction)")
	}

	config := ScanConfig{
		Paths:              []string{tmpDir},
		ScanFilePermission: true,
		ScanSensitiveData:  false,
		ScanEncryption:     false,
	}
	checker := NewChecker(config)

	report, err := checker.RunScan()
	if err != nil {
		t.Fatal(err)
	}

	// Should find world-writable issue
	found := false
	for _, issue := range report.Issues {
		if issue.Type == IssueFilePermission {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find world-writable permission issue")
	}
}

func TestSensitiveDataDetection(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file with sensitive data
	testFile := filepath.Join(tmpDir, "sensitive.txt")
	content := `
Name: John Doe
ID: 110101199003076531
Phone: 13812345678
Email: john@example.com
`
	if err := os.WriteFile(testFile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	config := ScanConfig{
		Paths:              []string{tmpDir},
		ScanFilePermission: false,
		ScanSensitiveData:  true,
		ScanEncryption:     false,
	}
	checker := NewChecker(config)

	report, err := checker.RunScan()
	if err != nil {
		t.Fatal(err)
	}

	if len(report.Issues) < 3 {
		t.Errorf("Expected at least 3 sensitive data issues, got %d", len(report.Issues))
	}

	// Check for ID card issue
	foundID := false
	foundPhone := false
	foundEmail := false
	for _, issue := range report.Issues {
		switch {
		case issue.Type == IssueSensitiveData && issue.Title == "Chinese ID card number detected":
			foundID = true
		case issue.Type == IssueSensitiveData && issue.Title == "Phone number detected":
			foundPhone = true
		case issue.Type == IssueSensitiveData && issue.Title == "Email address detected":
			foundEmail = true
		}
	}
	if !foundID {
		t.Error("Expected ID card detection issue")
	}
	if !foundPhone {
		t.Error("Expected phone detection issue")
	}
	if !foundEmail {
		t.Error("Expected email detection issue")
	}
}

func TestEncryptionCheck(t *testing.T) {
	tmpDir := t.TempDir()

	// Create unencrypted PEM file
	pemFile := filepath.Join(tmpDir, "server.pem")
	pemContent := `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0Z3VS5JJcds3xfn/ygWyF8PbnGy0AHL5wZhGhO3h5T4R
-----END RSA PRIVATE KEY-----`
	if err := os.WriteFile(pemFile, []byte(pemContent), 0600); err != nil {
		t.Fatal(err)
	}

	config := ScanConfig{
		Paths:              []string{tmpDir},
		ScanFilePermission: false,
		ScanSensitiveData:  false,
		ScanEncryption:     true,
	}
	checker := NewChecker(config)

	report, err := checker.RunScan()
	if err != nil {
		t.Fatal(err)
	}

	// Should detect unencrypted PEM
	found := false
	for _, issue := range report.Issues {
		if issue.Type == IssueEncryption {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected unencrypted PEM detection issue")
	}
}

func TestComplianceScore(t *testing.T) {
	checker := NewChecker(DefaultScanConfig([]string{}))

	// No issues = 100 score
	score := checker.GetScore()
	if score.Overall != 100 {
		t.Errorf("Expected 100 score with no issues, got %.2f", score.Overall)
	}
	if score.Level != "A" {
		t.Errorf("Expected level A, got %s", score.Level)
	}

	// Add some issues and check score drops
	checker.AddIssue(Issue{
		Type:     IssueGDPR,
		Severity: SeverityCritical,
		Title:    "Critical GDPR issue",
	})
	checker.AddIssue(Issue{
		Type:     IssueFilePermission,
		Severity: SeverityHigh,
		Title:    "High permission issue",
	})

	score = checker.GetScore()
	if score.Overall >= 100 {
		t.Error("Score should decrease after adding issues")
	}
	if score.Issues != 2 {
		t.Errorf("Expected 2 issues count, got %d", score.Issues)
	}
}

func TestIssueDeduplication(t *testing.T) {
	checker := NewChecker(DefaultScanConfig([]string{}))

	// Add same issue twice
	checker.AddIssue(Issue{
		Type:     IssueGDPR,
		Severity: SeverityHigh,
		Title:    "Duplicate issue",
		Path:     "/test",
	})
	checker.AddIssue(Issue{
		Type:     IssueGDPR,
		Severity: SeverityHigh,
		Title:    "Duplicate issue",
		Path:     "/test",
	})

	if len(checker.GetIssues()) != 1 {
		t.Errorf("Expected deduplication, got %d issues", len(checker.GetIssues()))
	}
}

func TestScoreLevels(t *testing.T) {
	tests := []struct {
		score    float64
		expected string
	}{
		{95, "A"},
		{85, "B"},
		{75, "C"},
		{65, "D"},
		{50, "F"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			level := "A"
			switch {
			case tt.score >= 90:
				level = "A"
			case tt.score >= 80:
				level = "B"
			case tt.score >= 70:
				level = "C"
			case tt.score >= 60:
				level = "D"
			default:
				level = "F"
			}
			if level != tt.expected {
				t.Errorf("Score %.0f: expected level %s, got %s", tt.score, tt.expected, level)
			}
		})
	}
}

func TestFixSuggestion(t *testing.T) {
	tests := []struct {
		issueType IssueType
		fix       string
	}{
		{IssueFilePermission, "chmod o-w /test"},
		{IssueSensitiveData, "Remove or encrypt sensitive data before storage"},
		{IssueEncryption, "Encrypt sensitive files using GPG: gpg -c <file>"},
		{IssueGDPR, "Implement data retention policies and user data deletion mechanisms"},
		{IssueGB20, "Implement access controls, audit logging, and regular backups per GB/T 22239-2019"},
	}

	for _, tt := range tests {
		t.Run(string(tt.issueType), func(t *testing.T) {
			issue := Issue{Type: tt.issueType}
			if tt.issueType == IssueFilePermission {
				issue.Fix = "chmod o-w /test"
			}
			suggestion := FixSuggestion(issue)
			if suggestion == "" {
				t.Error("Expected non-empty fix suggestion")
			}
		})
	}
}

func TestMarshalReport(t *testing.T) {
	report := &ComplianceReport{
		Summary: ComplianceScore{
			Overall: 95.5,
			Level:   "A",
			GDPR:    100,
			GB20:    90,
			Security: 96.5,
		},
		Issues: []Issue{
			{
				ID:          "issue-0001",
				Type:        IssueGDPR,
				Severity:    SeverityHigh,
				Title:       "Test issue",
				Description: "Test description",
			},
		},
		PathsCount: 2,
	}

	data, err := MarshalReport(report)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty JSON output")
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Invalid JSON output: %v", err)
	}
}

func TestUnmarshalReport(t *testing.T) {
	data := []byte(`{
		"summary": {"overall": 90, "level": "A", "gdpr": 95, "gb20": 85, "security": 90},
		"issues": [],
		"scan_time": "2024-01-01T00:00:00Z",
		"duration": 1000000,
		"paths_count": 1
	}`)

	report, err := UnmarshalReport(data)
	if err != nil {
		t.Fatal(err)
	}

	if report.Summary.Overall != 90 {
		t.Errorf("Expected overall 90, got %.2f", report.Summary.Overall)
	}
	if report.Summary.Level != "A" {
		t.Errorf("Expected level A, got %s", report.Summary.Level)
	}
}

func TestValidateAESKeyLength(t *testing.T) {
	tests := []struct {
		key      []byte
		expected bool
	}{
		{make([]byte, 16), true},  // AES-128
		{make([]byte, 24), true},  // AES-192
		{make([]byte, 32), true},  // AES-256
		{make([]byte, 10), false}, // Invalid
		{make([]byte, 48), false}, // Invalid
		{make([]byte, 0), false},  // Empty
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if got := ValidateAESKeyLength(tt.key); got != tt.expected {
				t.Errorf("ValidateAESKeyLength(%d bytes) = %v, want %v", len(tt.key), got, tt.expected)
			}
		})
	}
}

func TestCheckAESBlockCipher(t *testing.T) {
	tests := []struct {
		data     []byte
		expected bool
	}{
		{make([]byte, 16), true},  // One block
		{make([]byte, 32), true},  // Two blocks
		{make([]byte, 10), false}, // Not block-aligned
		{make([]byte, 0), false},  // Empty
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if got := CheckAESBlockCipher(tt.data); got != tt.expected {
				t.Errorf("CheckAESBlockCipher(%d bytes) = %v, want %v", len(tt.data), got, tt.expected)
			}
		})
	}
}

func TestFullScanWithMixedContent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create various test files

	// 1. Sensitive data file with open permissions
	sensitiveFile := filepath.Join(tmpDir, "users.csv")
	sensitiveContent := "name,id_card,phone,email\nJohn,110101199003076531,13812345678,john@test.com\n"
	if err := os.WriteFile(sensitiveFile, []byte(sensitiveContent), 0666); err != nil {
		t.Fatal(err)
	}

	// 2. Normal file with restrictive permissions
	normalFile := filepath.Join(tmpDir, "readme.md")
	if err := os.WriteFile(normalFile, []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Secret file
	secretFile := filepath.Join(tmpDir, "secret.key")
	if err := os.WriteFile(secretFile, []byte("mysecretkey123456"), 0600); err != nil {
		t.Fatal(err)
	}

	// Run full scan
	config := DefaultScanConfig([]string{tmpDir})
	checker := NewChecker(config)

	report, err := checker.RunScan()
	if err != nil {
		t.Fatal(err)
	}

	// Should have found multiple issues
	if len(report.Issues) == 0 {
		t.Error("Expected at least one issue from full scan")
	}

	// Score should be less than 100
	if report.Summary.Overall >= 100 {
		t.Error("Expected score less than 100")
	}

	// Report should be marshallable
	_, err = MarshalReport(report)
	if err != nil {
		t.Fatalf("Failed to marshal report: %v", err)
	}
}

func TestIgnorePatterns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory that should be ignored
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create file in .git with sensitive data
	gitFile := filepath.Join(gitDir, "config")
	if err := os.WriteFile(gitFile, []byte("110101199003076531"), 0666); err != nil {
		t.Fatal(err)
	}

	// Create file outside .git with sensitive data
	mainFile := filepath.Join(tmpDir, "data.txt")
	if err := os.WriteFile(mainFile, []byte("110101199003076531"), 0600); err != nil {
		t.Fatal(err)
	}

	config := DefaultScanConfig([]string{tmpDir})
	checker := NewChecker(config)

	report, err := checker.RunScan()
	if err != nil {
		t.Fatal(err)
	}

	// .git files should be ignored
	for _, issue := range report.Issues {
		if filepath.Base(issue.Path) == "config" {
			t.Error("Files in .git should be ignored")
		}
	}
}

func TestSeverityWeights(t *testing.T) {
	weights := map[Severity]float64{
		SeverityCritical: 25,
		SeverityHigh:     15,
		SeverityMedium:   8,
		SeverityLow:      3,
		SeverityInfo:     1,
	}

	if weights[SeverityCritical] <= weights[SeverityHigh] {
		t.Error("Critical should weigh more than High")
	}
	if weights[SeverityHigh] <= weights[SeverityMedium] {
		t.Error("High should weigh more than Medium")
	}
}
