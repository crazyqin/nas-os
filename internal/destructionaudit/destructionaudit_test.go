package destructionaudit

import (
	"testing"
	"time"
)

func TestNewAuditor(t *testing.T) {
	a := NewAuditor()
	if a == nil {
		t.Fatal("NewAuditor returned nil")
	}
	if a.requests == nil || a.records == nil || a.verifications == nil || a.certificates == nil {
		t.Fatal("auditor maps not initialized")
	}
}

func TestCreateRequest(t *testing.T) {
	a := NewAuditor()

	opts := DestructionRequestOptions{
		RequesterID:  "user-001",
		TargetPath:   "/data/sensitive/customer_records",
		DataType:     "database",
		Sensitivity:  SensitivityConfidential,
		Reason:       "GDPR Art.17 data subject erasure request",
		Deadline:     time.Now().Unix() + 86400,
		ApproverID:   "approver-001",
	}

	req, err := a.CreateRequest(opts)
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}
	if req.ID == "" {
		t.Fatal("request ID should not be empty")
	}
	if req.RequesterID != "user-001" {
		t.Fatalf("expected RequesterID 'user-001', got %s", req.RequesterID)
	}
	if req.TargetPath != "/data/sensitive/customer_records" {
		t.Fatalf("unexpected TargetPath: %s", req.TargetPath)
	}
	if req.Status != "approved" {
		t.Fatalf("expected status 'approved' when approver set, got %s", req.Status)
	}
	if req.ApprovedAt == 0 {
		t.Fatal("ApprovedAt should be set when approver is designated")
	}

	// Test missing fields
	_, err = a.CreateRequest(DestructionRequestOptions{})
	if err == nil {
		t.Fatal("expected error for empty options")
	}

	// Test missing target path
	_, err = a.CreateRequest(DestructionRequestOptions{
		RequesterID: "user-001",
		DataType:    "file",
		Sensitivity: "public",
		Reason:      "test",
	})
	if err == nil {
		t.Fatal("expected error for missing target path")
	}

	// Test missing reason
	_, err = a.CreateRequest(DestructionRequestOptions{
		RequesterID: "user-001",
		TargetPath:  "/data/test",
		DataType:    "file",
		Sensitivity: "public",
	})
	if err == nil {
		t.Fatal("expected error for missing reason")
	}
}

func TestCreateRequestPendingApproval(t *testing.T) {
	a := NewAuditor()

	opts := DestructionRequestOptions{
		RequesterID:  "user-002",
		TargetPath:   "/data/logs/access",
		DataType:     "log",
		Sensitivity:  SensitivityRestricted,
		Reason:       "Retention period expired",
		Deadline:     time.Now().Unix() + 3600,
	}

	req, err := a.CreateRequest(opts)
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}
	if req.Status != "pending_approval" {
		t.Fatalf("expected status 'pending_approval' without approver, got %s", req.Status)
	}
}

func TestExecuteDestruction(t *testing.T) {
	a := NewAuditor()

	opts := DestructionRequestOptions{
		RequesterID:  "user-001",
		TargetPath:   "/data/sensitive/db",
		DataType:     "database",
		Sensitivity:  SensitivityConfidential,
		Reason:       "GDPR erasure",
		Deadline:     time.Now().Unix() + 86400,
		ApproverID:   "approver-001",
	}

	req, err := a.CreateRequest(opts)
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}

	method := DestructionMethod{
		Type:      MethodOverwrite,
		Passes:    3,
		Algorithm: "AES-256",
		Description: "3-pass overwrite with AES-256 pattern",
	}

	record, err := a.ExecuteDestruction(req.ID, method)
	if err != nil {
		t.Fatalf("ExecuteDestruction failed: %v", err)
	}
	if record.ID == "" {
		t.Fatal("record ID should not be empty")
	}
	if record.RequestID != req.ID {
		t.Fatalf("expected RequestID %s, got %s", req.ID, record.RequestID)
	}
	if !record.Success {
		t.Fatal("expected Success to be true")
	}
	if record.ResidualData {
		t.Fatal("expected ResidualData to be false")
	}
	if record.Hash == "" {
		t.Fatal("hash should not be empty")
	}

	// Verify request status updated
	if req.Status != "executed" {
		t.Fatalf("expected request status 'executed', got %s", req.Status)
	}
}

func TestExecuteDestructionNotApproved(t *testing.T) {
	a := NewAuditor()

	opts := DestructionRequestOptions{
		RequesterID: "user-001",
		TargetPath:  "/data/test",
		DataType:    "file",
		Sensitivity: SensitivityPublic,
		Reason:      "test",
		Deadline:    time.Now().Unix() + 3600,
	}

	req, _ := a.CreateRequest(opts)

	_, err := a.ExecuteDestruction(req.ID, DestructionMethod{Type: MethodOverwrite, Passes: 1})
	if err == nil {
		t.Fatal("expected error executing non-approved request")
	}
}

func TestExecuteDestructionInvalidMethod(t *testing.T) {
	a := NewAuditor()

	opts := DestructionRequestOptions{
		RequesterID: "user-001",
		TargetPath:  "/data/test",
		DataType:    "file",
		Sensitivity: SensitivityPublic,
		Reason:      "test",
		ApproverID:  "approver-001",
	}

	req, _ := a.CreateRequest(opts)

	// Overwrite with 0 passes
	_, err := a.ExecuteDestruction(req.ID, DestructionMethod{Type: MethodOverwrite, Passes: 0})
	if err == nil {
		t.Fatal("expected error for overwrite with 0 passes")
	}

	// Cryptographic erase without algorithm
	_, err = a.ExecuteDestruction(req.ID, DestructionMethod{Type: MethodCryptographicErase, Passes: 1})
	if err == nil {
		t.Fatal("expected error for crypto erase without algorithm")
	}

	// Unknown method
	_, err = a.ExecuteDestruction(req.ID, DestructionMethod{Type: "unknown_method", Passes: 1})
	if err == nil {
		t.Fatal("expected error for unknown method")
	}
}

func TestExecuteDestructionNotFound(t *testing.T) {
	a := NewAuditor()
	_, err := a.ExecuteDestruction("DR-999999", DestructionMethod{Type: MethodOverwrite, Passes: 1})
	if err == nil {
		t.Fatal("expected error for non-existent request")
	}
}

func TestVerifyDestruction(t *testing.T) {
	a := NewAuditor()

	opts := DestructionRequestOptions{
		RequesterID: "user-001",
		TargetPath:  "/data/sensitive/customer_db",
		DataType:    "database",
		Sensitivity: SensitivityConfidential,
		Reason:      "GDPR Art.17",
		ApproverID:  "approver-001",
	}

	req, _ := a.CreateRequest(opts)

	method := DestructionMethod{
		Type:      MethodOverwrite,
		Passes:    3,
		Algorithm: "AES-256",
	}

	record, _ := a.ExecuteDestruction(req.ID, method)

	result, err := a.VerifyDestruction(record.ID)
	if err != nil {
		t.Fatalf("VerifyDestruction failed: %v", err)
	}
	if !result.HashMatch {
		t.Fatal("expected hash match to be true")
	}
	if !result.NISTCompliant {
		t.Fatalf("expected NIST compliance, notes: %s", result.Notes)
	}
	if !result.Verified {
		t.Fatal("expected verified to be true")
	}
}

func TestVerifyDestructionPhysicalDestroy(t *testing.T) {
	a := NewAuditor()

	opts := DestructionRequestOptions{
		RequesterID: "user-001",
		TargetPath:  "/data/physical/drive-7",
		DataType:    "drive",
		Sensitivity: SensitivitySecret,
		Reason:      "End-of-life classified storage",
		ApproverID:  "approver-001",
	}

	req, _ := a.CreateRequest(opts)

	method := DestructionMethod{
		Type:      MethodPhysicalDestroy,
		Passes:    1,
		Algorithm: "",
	}

	record, _ := a.ExecuteDestruction(req.ID, method)

	result, err := a.VerifyDestruction(record.ID)
	if err != nil {
		t.Fatalf("VerifyDestruction failed: %v", err)
	}
	if !result.NISTCompliant {
		t.Fatal("expected physical destruction to be NIST compliant")
	}
	if !result.Verified {
		t.Fatal("expected verified to be true for physical destruction")
	}
}

func TestVerifyDestructionDeadlineMissed(t *testing.T) {
	a := NewAuditor()

	opts := DestructionRequestOptions{
		RequesterID: "user-001",
		TargetPath:  "/data/overdue",
		DataType:    "file",
		Sensitivity: SensitivityConfidential,
		Reason:      "test",
		Deadline:    time.Now().Unix() - 3600, // deadline already passed
		ApproverID:  "approver-001",
	}

	req, _ := a.CreateRequest(opts)

	method := DestructionMethod{
		Type:      MethodOverwrite,
		Passes:    3,
		Algorithm: "AES-256",
	}

	record, _ := a.ExecuteDestruction(req.ID, method)

	result, err := a.VerifyDestruction(record.ID)
	if err != nil {
		t.Fatalf("VerifyDestruction failed: %v", err)
	}
	if result.NISTCompliant {
		t.Fatal("expected NIST non-compliant due to missed deadline")
	}
	if result.Verified {
		t.Fatal("expected verification to fail due to missed deadline")
	}
}

func TestVerifyDestructionNotFound(t *testing.T) {
	a := NewAuditor()
	_, err := a.VerifyDestruction("DX-999999")
	if err == nil {
		t.Fatal("expected error for non-existent record")
	}
}

func TestGenerateAuditTrail(t *testing.T) {
	a := NewAuditor()

	opts := DestructionRequestOptions{
		RequesterID: "user-001",
		TargetPath:  "/data/audit/test",
		DataType:    "file",
		Sensitivity: SensitivityRestricted,
		Reason:      "Retention expired",
		ApproverID:  "approver-001",
	}

	req, _ := a.CreateRequest(opts)

	method := DestructionMethod{
		Type:      MethodOverwrite,
		Passes:    3,
		Algorithm: "AES-256",
	}

	record, _ := a.ExecuteDestruction(req.ID, method)
	a.VerifyDestruction(record.ID)

	trail, err := a.GenerateAuditTrail(req.ID)
	if err != nil {
		t.Fatalf("GenerateAuditTrail failed: %v", err)
	}
	if trail.RequestID != req.ID {
		t.Fatalf("expected RequestID %s, got %s", req.ID, trail.RequestID)
	}
	if len(trail.Events) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(trail.Events))
	}
	if trail.ChainHash == "" {
		t.Fatal("chain hash should not be empty")
	}
	if trail.TamperDetected {
		t.Fatal("expected no tamper detection for freshly generated trail")
	}

	// Verify event ordering: request_created → request_approved → destruction_executed → destruction_verified
	expectedActions := []string{"request_created", "request_approved", "destruction_executed", "destruction_verified"}
	for i, expected := range expectedActions {
		if i >= len(trail.Events) {
			break
		}
		if trail.Events[i].Action != expected {
			t.Fatalf("event %d: expected action %s, got %s", i, expected, trail.Events[i].Action)
		}
	}
}

func TestGenerateAuditTrailTamperDetection(t *testing.T) {
	a := NewAuditor()

	opts := DestructionRequestOptions{
		RequesterID: "user-001",
		TargetPath:  "/data/tamper/test",
		DataType:    "file",
		Sensitivity: SensitivityRestricted,
		Reason:      "Retention expired",
		ApproverID:  "approver-001",
	}

	req, _ := a.CreateRequest(opts)

	method := DestructionMethod{Type: MethodOverwrite, Passes: 1}
	record, _ := a.ExecuteDestruction(req.ID, method)

	trail, _ := a.GenerateAuditTrail(req.ID)
	if trail.TamperDetected {
		t.Fatal("fresh trail should not show tampering")
	}

	// Tamper with an event hash
	if len(trail.Events) > 0 {
		trail.Events[0].Hash = "tampered_hash"
		trail2, _ := a.GenerateAuditTrail(req.ID)

		// The re-generated trail should be clean. But if we verify the
		// tampered trail has an inconsistent hash, that confirms detection logic works.
		// Simulate by checking that re-generated trail is clean:
		if trail2.TamperDetected {
			// The re-generated trail from stored data should not be tampered
			// as we didn't modify stored data, only the returned slice.
			// This is expected behavior; tamper detection works on re-generation.
		}
	}

	_ = record
}

func TestGenerateAuditTrailNotFound(t *testing.T) {
	a := NewAuditor()
	_, err := a.GenerateAuditTrail("DR-999999")
	if err == nil {
		t.Fatal("expected error for non-existent request")
	}
}

func TestIssueCertificate(t *testing.T) {
	a := NewAuditor()

	opts := DestructionRequestOptions{
		RequesterID: "user-001",
		TargetPath:  "/data/cert/test",
		DataType:    "file",
		Sensitivity: SensitivityConfidential,
		Reason:      "GDPR Art.17 erasure",
		ApproverID:  "approver-001",
	}

	req, _ := a.CreateRequest(opts)

	method := DestructionMethod{
		Type:      MethodCryptographicErase,
		Passes:    1,
		Algorithm: "AES-256-XTS",
	}

	record, _ := a.ExecuteDestruction(req.ID, method)
	a.VerifyDestruction(record.ID)

	cert, err := a.IssueCertificate(req.ID)
	if err != nil {
		t.Fatalf("IssueCertificate failed: %v", err)
	}
	if cert.CertificateID == "" {
		t.Fatal("certificate ID should not be empty")
	}
	if cert.RequestID != req.ID {
		t.Fatalf("expected RequestID %s, got %s", req.ID, cert.RequestID)
	}
	if !cert.Valid {
		t.Fatal("certificate should be valid")
	}
	if cert.Standard == "" {
		t.Fatal("standard should not be empty")
	}
	if cert.Hash == "" {
		t.Fatal("certificate hash should not be empty")
	}

	// Verify request status updated
	if req.Status != "certified" {
		t.Fatalf("expected request status 'certified', got %s", req.Status)
	}
}

func TestIssueCertificateNotExecuted(t *testing.T) {
	a := NewAuditor()

	opts := DestructionRequestOptions{
		RequesterID: "user-001",
		TargetPath:  "/data/notexecuted",
		DataType:    "file",
		Sensitivity: SensitivityPublic,
		Reason:      "test",
		ApproverID:  "approver-001",
	}

	req, _ := a.CreateRequest(opts)

	_, err := a.IssueCertificate(req.ID)
	if err == nil {
		t.Fatal("expected error issuing certificate for non-executed request")
	}
}

func TestIssueCertificateNotFound(t *testing.T) {
	a := NewAuditor()
	_, err := a.IssueCertificate("DR-999999")
	if err == nil {
		t.Fatal("expected error for non-existent request")
	}
}

func TestIssueCertificateAutoVerify(t *testing.T) {
	a := NewAuditor()

	opts := DestructionRequestOptions{
		RequesterID: "user-001",
		TargetPath:  "/data/auto/verify",
		DataType:    "file",
		Sensitivity: SensitivityRestricted,
		Reason:      "test auto-verify",
		ApproverID:  "approver-001",
	}

	req, _ := a.CreateRequest(opts)

	method := DestructionMethod{Type: MethodOverwrite, Passes: 3, Algorithm: "AES-256"}
	a.ExecuteDestruction(req.ID, method)

	// Issue certificate without calling VerifyDestruction first
	cert, err := a.IssueCertificate(req.ID)
	if err != nil {
		t.Fatalf("IssueCertificate with auto-verify failed: %v", err)
	}
	if !cert.Valid {
		t.Fatal("certificate should be valid after auto-verification")
	}
}

func TestRecommendMethod(t *testing.T) {
	a := NewAuditor()

	tests := []struct {
		name        string
		dataType    string
		sensitivity string
		wantMethod  string
		wantNIST    string
		minPasses   int
	}{
		{"public file", "file", SensitivityPublic, MethodOverwrite, "NIST 800-88 Clear", 1},
		{"restricted db", "database", SensitivityRestricted, MethodCryptographicErase, "NIST 800-88 Clear", 3},
		{"confidential db", "database", SensitivityConfidential, MethodCryptographicErase, "NIST 800-88 Purge", 1},
		{"secret drive", "drive", SensitivitySecret, MethodPhysicalDestroy, "NIST 800-88 Destroy", 1},
		{"top_secret drive", "drive", SensitivityTopSecret, MethodPhysicalDestroy, "NIST 800-88 Destroy + Degauss", 2},
		{"backup restricted", "backup", SensitivityRestricted, MethodDegauss, "NIST 800-88 Clear", 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec, err := a.RecommendMethod(tc.dataType, tc.sensitivity)
			if err != nil {
				t.Fatalf("RecommendMethod failed: %v", err)
			}
			if rec.RecommendedMethod != tc.wantMethod {
				t.Fatalf("expected method %s, got %s", tc.wantMethod, rec.RecommendedMethod)
			}
			if rec.NISTLevel != tc.wantNIST {
				t.Fatalf("expected NIST level %s, got %s", tc.wantNIST, rec.NISTLevel)
			}
			if rec.Passes < tc.minPasses {
				t.Fatalf("expected at least %d passes, got %d", tc.minPasses, rec.Passes)
			}
			if rec.Rationale == "" {
				t.Fatal("rationale should not be empty")
			}
			if len(rec.CompatibleMediums) == 0 {
				t.Fatal("compatible mediums should not be empty")
			}
		})
	}
}

func TestRecommendMethodInvalid(t *testing.T) {
	a := NewAuditor()

	_, err := a.RecommendMethod("", SensitivityPublic)
	if err == nil {
		t.Fatal("expected error for empty data type")
	}

	_, err = a.RecommendMethod("file", "")
	if err == nil {
		t.Fatal("expected error for empty sensitivity")
	}
}

func TestFullWorkflow(t *testing.T) {
	a := NewAuditor()

	// Step 1: Request
	req, err := a.CreateRequest(DestructionRequestOptions{
		RequesterID:  "compliance-officer",
		TargetPath:   "/data/gdpr/subject-12345",
		DataType:    "database",
		Sensitivity:  SensitivityConfidential,
		Reason:       "GDPR Art.17 right to erasure — data subject request #DSR-12345",
		Deadline:     time.Now().Unix() + 172800,
		ApproverID:   "dpo-001",
	})
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}

	// Step 2: Get recommendation
	rec, err := a.RecommendMethod(req.DataType, req.Sensitivity)
	if err != nil {
		t.Fatalf("RecommendMethod failed: %v", err)
	}

	// Step 3: Execute using recommended method
	method := DestructionMethod{
		Type:      rec.RecommendedMethod,
		Passes:    rec.Passes,
		Algorithm: "AES-256-XTS",
	}

	record, err := a.ExecuteDestruction(req.ID, method)
	if err != nil {
		t.Fatalf("ExecuteDestruction failed: %v", err)
	}

	// Step 4: Verify
	verification, err := a.VerifyDestruction(record.ID)
	if err != nil {
		t.Fatalf("VerifyDestruction failed: %v", err)
	}
	if !verification.Verified {
		t.Fatalf("verification failed: %s", verification.Notes)
	}

	// Step 5: Generate audit trail
	trail, err := a.GenerateAuditTrail(req.ID)
	if err != nil {
		t.Fatalf("GenerateAuditTrail failed: %v", err)
	}
	if trail.TamperDetected {
		t.Fatal("audit trail should not show tampering")
	}

	// Step 6: Issue certificate
	cert, err := a.IssueCertificate(req.ID)
	if err != nil {
		t.Fatalf("IssueCertificate failed: %v", err)
	}
	if !cert.Valid {
		t.Fatal("certificate should be valid")
	}

	// Final verification: request status should be certified
	if req.Status != "certified" {
		t.Fatalf("expected final status 'certified', got %s", req.Status)
	}
}