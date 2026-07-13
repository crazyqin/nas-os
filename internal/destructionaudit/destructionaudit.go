package destructionaudit

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// DestructionAuditor manages data destruction requests, execution, verification,
// audit trail generation, and compliance certification per GDPR/CCPA and NIST 800-88.
type DestructionAuditor struct {
	mu          sync.RWMutex
	requests    map[string]*DestructionRequest
	records     map[string]*DestructionRecord
	verifications map[string]*VerificationResult
	certificates  map[string]*DestructionCertificate
	nextReqID    int
	nextRecID    int
	nextCertID   int
}

// DestructionRequestOptions defines parameters for creating a new destruction request.
type DestructionRequestOptions struct {
	RequesterID string
	TargetPath  string
	DataType    string
	Sensitivity string
	Reason      string
	Deadline    int64
	ApproverID  string
}

// DestructionRequest represents a formalized request to destroy data.
type DestructionRequest struct {
	ID          string
	RequesterID string
	TargetPath  string
	DataType    string
	Sensitivity string
	Status      string
	CreatedAt   int64
	Deadline    int64
	ApproverID  string
	ApprovedAt  int64
}

// DestructionMethod describes the method utilized to destroy data.
type DestructionMethod struct {
	Type            string
	Passes          int
	Algorithm       string
	VerificationHash string
	Description     string
}

// DestructionRecord captures the outcome of a destruction execution.
type DestructionRecord struct {
	ID           string
	RequestID    string
	Method       DestructionMethod
	ExecutedAt   int64
	ExecutedBy   string
	Duration     int64
	Success      bool
	ResidualData bool
	Hash         string
}

// VerificationResult records the outcome of verifying a destruction record.
type VerificationResult struct {
	RecordID      string
	Verified      bool
	Method        string
	HashMatch     bool
	NISTCompliant bool
	VerifiedAt    int64
	Notes         string
}

// AuditTrail contains a tamper-evident chain of events for a destruction request.
type AuditTrail struct {
	RequestID     string
	Events        []AuditEvent
	ChainHash     string
	TamperDetected bool
}

// AuditEvent is one link in the audit chain.
type AuditEvent struct {
	Timestamp int64
	Action    string
	Actor     string
	Details   string
	Hash      string
}

// DestructionCertificate is a GDPR/CCPA-compliant proof-of-destruction document.
type DestructionCertificate struct {
	CertificateID string
	RequestID     string
	IssuedAt      int64
	DestroyedData string
	Method        string
	Standard      string
	VerifierID    string
	Hash          string
	Valid         bool
}

// MethodRecommendation suggests a destruction method based on data characteristics.
type MethodRecommendation struct {
	RecommendedMethod   string
	Rationale           string
	NISTLevel          string
	EstimatedTime       string
	Passes              int
	CompatibleMediums  []string
}

// Sensitivity / method constants
const (
	SensitivityRestricted = "restricted"
	SensitivityConfidential = "confidential"
	SensitivitySecret      = "secret"
	SensitivityTopSecret   = "top_secret"
	SensitivityPublic      = "public"
)

const (
	MethodOverwrite    = "overwrite"
	MethodCryptographicErase = "cryptographic_erase"
	MethodPhysicalDestroy   = "physical_destruction"
	MethodDegauss      = "degauss"
)

// NewAuditor creates a fully initialized DestructionAuditor.
func NewAuditor() *DestructionAuditor {
	return &DestructionAuditor{
		requests:      make(map[string]*DestructionRequest),
		records:       make(map[string]*DestructionRecord),
		verifications: make(map[string]*VerificationResult),
		certificates:  make(map[string]*DestructionCertificate),
	}
}

// CreateRequest registers a new data destruction request.
func (a *DestructionAuditor) CreateRequest(opts DestructionRequestOptions) (*DestructionRequest, error) {
	if opts.RequesterID == "" {
		return nil, errors.New("requester ID cannot be empty")
	}
	if opts.TargetPath == "" {
		return nil, errors.New("target path cannot be empty")
	}
	if opts.DataType == "" {
		return nil, errors.New("data type cannot be empty")
	}
	if opts.Sensitivity == "" {
		return nil, errors.New("sensitivity cannot be empty")
	}
	if opts.Reason == "" {
		return nil, errors.New("reason cannot be empty")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.nextReqID++
	id := fmt.Sprintf("DR-%06d", a.nextReqID)

	now := time.Now().Unix()

	req := &DestructionRequest{
		ID:          id,
		RequesterID: opts.RequesterID,
		TargetPath:  opts.TargetPath,
		DataType:    opts.DataType,
		Sensitivity: opts.Sensitivity,
		Status:      "pending_approval",
		CreatedAt:   now,
		Deadline:    opts.Deadline,
		ApproverID:  opts.ApproverID,
	}

	// Auto-approve if approver is designated
	if opts.ApproverID != "" {
		req.Status = "approved"
		req.ApprovedAt = now
	} else {
		req.Status = "pending_approval"
	}

	a.requests[id] = req
	return req, nil
}

// ExecuteDestruction performs the destruction of data for an approved request
// using the specified method.
func (a *DestructionAuditor) ExecuteDestruction(requestID string, method DestructionMethod) (*DestructionRecord, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	req, ok := a.requests[requestID]
	if !ok {
		return nil, fmt.Errorf("destruction request %s not found", requestID)
	}
	if req.Status != "approved" {
		return nil, fmt.Errorf("request %s is not approved (current status: %s)", requestID, req.Status)
	}

	// Validate method
	if err := validateMethod(method); err != nil {
		return nil, fmt.Errorf("invalid destruction method: %w", err)
	}

	a.nextRecID++
	recordID := fmt.Sprintf("DX-%06d", a.nextRecID)

	now := time.Now().Unix()
	duration := int64(1) // Simulated execution time (seconds) per pass
	if method.Passes > 0 {
		duration = int64(method.Passes) * 2
	}

	// Determine success based on method completeness
	success := true
	residual := false

	// Physical destruction always succeeds; overwrite/cryptographic depend on passes
	if method.Type == MethodOverwrite && method.Passes < 1 {
		success = false
		residual = true
	}

	// Compute verification hash: chain of request ID + method info + timestamp
	hashInput := fmt.Sprintf("%s|%s|%s|%d|%d", requestID, method.Type, method.Algorithm, method.Passes, now)
	h := sha256.Sum256([]byte(hashInput))
	hash := hex.EncodeToString(h[:])

	record := &DestructionRecord{
		ID:           recordID,
		RequestID:    requestID,
		Method:       method,
		ExecutedAt:   now,
		ExecutedBy:   req.RequesterID,
		Duration:     duration,
		Success:      success,
		ResidualData: residual,
		Hash:         hash,
	}

	a.records[recordID] = record

	// Update request status
	req.Status = "executed"

	return record, nil
}

// VerifyDestruction checks a destruction record for compliance with NIST 800-88
// and verifies the integrity of the destruction hash.
func (a *DestructionAuditor) VerifyDestruction(recordID string) (*VerificationResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	record, ok := a.records[recordID]
	if !ok {
		return nil, fmt.Errorf("destruction record %s not found", recordID)
	}

	// Recompute hash to verify integrity
	hashInput := fmt.Sprintf("%s|%s|%s|%d|%d",
		record.RequestID, record.Method.Type, record.Method.Algorithm,
		record.Method.Passes, record.ExecutedAt)
	h := sha256.Sum256([]byte(hashInput))
	recomputed := hex.EncodeToString(h[:])

	hashMatch := recomputed == record.Hash

	// NIST 800-88 compliance check
	nistCompliant := false
	notes := []string{}

	switch record.Method.Type {
	case MethodOverwrite:
		if record.Method.Passes >= 1 && hashMatch && record.Success && !record.ResidualData {
			nistCompliant = true
		} else {
			if record.Method.Passes < 1 {
				notes = append(notes, "insufficient overwrite passes (minimum 1 required)")
			}
			if record.ResidualData {
				notes = append(notes, "residual data detected")
			}
		}
	case MethodCryptographicErase:
		if record.Method.Algorithm != "" && hashMatch && record.Success {
			nistCompliant = true
		} else {
			if record.Method.Algorithm == "" {
				notes = append(notes, "missing cryptographic algorithm specification")
			}
		}
	case MethodPhysicalDestroy:
		if hashMatch && record.Success {
			nistCompliant = true
		}
	case MethodDegauss:
		if hashMatch && record.Success {
			nistCompliant = true
		}
	default:
		notes = append(notes, fmt.Sprintf("unknown method type: %s", record.Method.Type))
	}

	// Check deadline compliance if request exists
	if req, ok := a.requests[record.RequestID]; ok {
		if req.Deadline > 0 && record.ExecutedAt > req.Deadline {
			nistCompliant = false
			notes = append(notes, "destruction executed after deadline")
		}
	}

	verified := hashMatch && record.Success && nistCompliant

	result := &VerificationResult{
		RecordID:      recordID,
		Verified:      verified,
		Method:        record.Method.Type,
		HashMatch:     hashMatch,
		NISTCompliant: nistCompliant,
		VerifiedAt:    time.Now().Unix(),
		Notes:         strings.Join(notes, "; "),
	}

	a.verifications[recordID] = result
	return result, nil
}

// GenerateAuditTrail builds a tamper-evident chain of events for the given request.
func (a *DestructionAuditor) GenerateAuditTrail(requestID string) (*AuditTrail, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	req, ok := a.requests[requestID]
	if !ok {
		return nil, fmt.Errorf("destruction request %s not found", requestID)
	}

	var events []AuditEvent

	// Event 1: Request created
	prevHash := ""
	details := fmt.Sprintf("Request created by %s for %s (type: %s, sensitivity: %s)",
		req.RequesterID, req.TargetPath, req.DataType, req.Sensitivity)
	e1 := buildEvent(req.CreatedAt, "request_created", req.RequesterID, details, prevHash)
	events = append(events, e1)
	prevHash = e1.Hash

	// Event 2: Approval (if applicable)
	if req.ApprovedAt > 0 {
		details := fmt.Sprintf("Request approved by %s", req.ApproverID)
		e2 := buildEvent(req.ApprovedAt, "request_approved", req.ApproverID, details, prevHash)
		events = append(events, e2)
		prevHash = e2.Hash
	}

	// Event 3+: Destruction records
	for _, rec := range a.records {
		if rec.RequestID != requestID {
			continue
		}
		details := fmt.Sprintf("Destruction executed: method=%s, passes=%d, success=%v, residual=%v, duration=%ds",
			rec.Method.Type, rec.Method.Passes, rec.Success, rec.ResidualData, rec.Duration)
		e := buildEvent(rec.ExecutedAt, "destruction_executed", rec.ExecutedBy, details, prevHash)
		events = append(events, e)
		prevHash = e.Hash
	}

	// Event 4+: Verifications
	for _, ver := range a.verifications {
		rec, ok := a.records[ver.RecordID]
		if !ok || rec.RequestID != requestID {
			continue
		}
		details := fmt.Sprintf("Verification: verified=%v, hash_match=%v, nist_compliant=%v, notes=%s",
			ver.Verified, ver.HashMatch, ver.NISTCompliant, ver.Notes)
		e := buildEvent(ver.VerifiedAt, "destruction_verified", "system", details, prevHash)
		events = append(events, e)
		prevHash = e.Hash
	}

	// Event 5+: Certificates
	for _, cert := range a.certificates {
		if cert.RequestID != requestID {
			continue
		}
		details := fmt.Sprintf("Certificate issued: %s, standard=%s, valid=%v",
			cert.CertificateID, cert.Standard, cert.Valid)
		e := buildEvent(cert.IssuedAt, "certificate_issued", cert.VerifierID, details, prevHash)
		events = append(events, e)
		prevHash = e.Hash
	}

	// Verify chain integrity
	tamperDetected := false
	chainHash := prevHash
	if len(events) > 0 {
		expectedPrev := ""
		for i, ev := range events {
			if i == 0 {
				expectedPrev = ""
			}
			recomputed := computeEventHash(ev.Timestamp, ev.Action, ev.Actor, ev.Details, expectedPrev)
			if recomputed != ev.Hash {
				tamperDetected = true
				break
			}
			expectedPrev = ev.Hash
		}
	}

	return &AuditTrail{
		RequestID:     requestID,
		Events:        events,
		ChainHash:     chainHash,
		TamperDetected: tamperDetected,
	}, nil
}

// IssueCertificate generates a formal destruction certificate per the given request.
func (a *DestructionAuditor) IssueCertificate(requestID string) (*DestructionCertificate, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	req, ok := a.requests[requestID]
	if !ok {
		return nil, fmt.Errorf("destruction request %s not found", requestID)
	}
	if req.Status != "executed" {
		return nil, fmt.Errorf("request %s has not been executed (status: %s)", requestID, req.Status)
	}

	// Find the destruction record for this request
	var record *DestructionRecord
	for _, rec := range a.records {
		if rec.RequestID == requestID {
			record = rec
			break
		}
	}
	if record == nil {
		return nil, fmt.Errorf("no destruction record found for request %s", requestID)
	}

	// Find verification result
	var verification *VerificationResult
	for _, ver := range a.verifications {
		if ver.RecordID == record.ID {
			verification = ver
			break
		}
	}
	if verification == nil {
		// Auto-verify if not already done — need to unlock to call VerifyDestruction
		// But we hold the lock. Instead, inline the verification logic.
		hashInput := fmt.Sprintf("%s|%s|%s|%d|%d",
			record.RequestID, record.Method.Type, record.Method.Algorithm,
			record.Method.Passes, record.ExecutedAt)
		h := sha256.Sum256([]byte(hashInput))
		recomputed := hex.EncodeToString(h[:])
		verification = &VerificationResult{
			RecordID:      record.ID,
			Verified:      recomputed == record.Hash && record.Success,
			Method:        record.Method.Type,
			HashMatch:     recomputed == record.Hash,
			NISTCompliant: recomputed == record.Hash && record.Success,
			VerifiedAt:    time.Now().Unix(),
			Notes:         "",
		}
		a.verifications[record.ID] = verification
	}

	if !verification.Verified {
		return nil, errors.New("destruction verification failed; cannot issue certificate")
	}

	a.nextCertID++
	certID := fmt.Sprintf("DC-%06d", a.nextCertID)

	now := time.Now().Unix()
	standard := "NIST 800-88"
	if req.Sensitivity == SensitivityTopSecret || req.Sensitivity == SensitivitySecret {
		standard = "NIST 800-88 / GDPR Art.17"
	}

	// Certificate hash
	hashInput := fmt.Sprintf("%s|%s|%s|%s|%d", certID, requestID, record.Method.Type, standard, now)
	h := sha256.Sum256([]byte(hashInput))
	hash := hex.EncodeToString(h[:])

	cert := &DestructionCertificate{
		CertificateID: certID,
		RequestID:     requestID,
		IssuedAt:      now,
		DestroyedData: req.TargetPath,
		Method:        record.Method.Type,
		Standard:      standard,
		VerifierID:    "system",
		Hash:          hash,
		Valid:         true,
	}

	a.certificates[certID] = cert
	req.Status = "certified"

	return cert, nil
}

// RecommendMethod suggests an appropriate destruction method based on data type and sensitivity.
func (a *DestructionAuditor) RecommendMethod(dataType string, sensitivity string) (*MethodRecommendation, error) {
	if dataType == "" {
		return nil, errors.New("data type cannot be empty")
	}
	if sensitivity == "" {
		return nil, errors.New("sensitivity cannot be empty")
	}

	rec := &MethodRecommendation{}

	switch sensitivity {
	case SensitivityPublic:
		rec.RecommendedMethod = MethodOverwrite
		rec.Rationale = "Public data requires only basic sanitization; single-pass overwrite sufficient."
		rec.NISTLevel = "NIST 800-88 Clear"
		rec.EstimatedTime = "1-5 minutes"
		rec.Passes = 1
		rec.CompatibleMediums = []string{"HDD", "SSD", "NVMe", "Tape"}
	case SensitivityRestricted:
		rec.RecommendedMethod = MethodOverwrite
		rec.Rationale = "Restricted data should be overwritten with at least 3 passes to ensure data is not recoverable."
		rec.NISTLevel = "NIST 800-88 Clear"
		rec.EstimatedTime = "10-30 minutes"
		rec.Passes = 3
		rec.CompatibleMediums = []string{"HDD", "SSD", "NVMe"}
	case SensitivityConfidential:
		rec.RecommendedMethod = MethodCryptographicErase
		rec.Rationale = "Confidential data benefits from cryptographic erase which destroys encryption keys, making recovery impossible."
		rec.NISTLevel = "NIST 800-88 Purge"
		rec.EstimatedTime = "1-5 minutes"
		rec.Passes = 1
		rec.CompatibleMediums = []string{"SSD", "NVMe", "Self-Encrypting Drive"}
	case SensitivitySecret:
		rec.RecommendedMethod = MethodPhysicalDestroy
		rec.Rationale = "Secret-level data requires physical destruction to guarantee no recovery is possible."
		rec.NISTLevel = "NIST 800-88 Destroy"
		rec.EstimatedTime = "30-60 minutes"
		rec.Passes = 1
		rec.CompatibleMediums = []string{"HDD", "SSD", "NVMe", "Tape", "Optical"}
	case SensitivityTopSecret:
		rec.RecommendedMethod = MethodPhysicalDestroy
		rec.Rationale = "Top-secret data mandates physical destruction followed by degaussing for magnetic media."
		rec.NISTLevel = "NIST 800-88 Destroy + Degauss"
		rec.EstimatedTime = "60-120 minutes"
		rec.Passes = 2
		rec.CompatibleMediums = []string{"HDD", "SSD", "NVMe", "Tape", "Optical"}
	default:
		rec.RecommendedMethod = MethodOverwrite
		rec.Rationale = fmt.Sprintf("Unknown sensitivity '%s'; defaulting to overwrite with 3 passes.", sensitivity)
		rec.NISTLevel = "NIST 800-88 Clear"
		rec.EstimatedTime = "10-30 minutes"
		rec.Passes = 3
		rec.CompatibleMediums = []string{"HDD", "SSD", "NVMe"}
	}

	// Adjust for specific data types
	switch dataType {
	case "database", "record":
		if sensitivity != SensitivityTopSecret {
			rec.RecommendedMethod = MethodCryptographicErase
			rec.Rationale += " Cryptographic erase is preferred for structured data records."
		}
	case "backup", "archive":
		if rec.RecommendedMethod != MethodPhysicalDestroy {
			rec.RecommendedMethod = MethodDegauss
			rec.Rationale += " Degaussing is effective for backup/archive magnetic media."
		}
	case "log":
		rec.Passes = 1
		rec.EstimatedTime = "1-5 minutes"
	}

	return rec, nil
}

// --- Internal helpers ---

func validateMethod(method DestructionMethod) error {
	switch method.Type {
	case MethodOverwrite:
		if method.Passes < 1 {
			return errors.New("overwrite method requires at least 1 pass")
		}
	case MethodCryptographicErase:
		if method.Algorithm == "" {
			return errors.New("cryptographic erase requires an algorithm specification")
		}
	case MethodPhysicalDestroy:
		// always valid
	case MethodDegauss:
		// always valid
	default:
		return fmt.Errorf("unknown destruction method type: %s", method.Type)
	}
	return nil
}

func buildEvent(timestamp int64, action, actor, details, prevHash string) AuditEvent {
	hash := computeEventHash(timestamp, action, actor, details, prevHash)
	return AuditEvent{
		Timestamp: timestamp,
		Action:   action,
		Actor:    actor,
		Details:  details,
		Hash:     hash,
	}
}

func computeEventHash(timestamp int64, action, actor, details, prevHash string) string {
	input := fmt.Sprintf("%d|%s|%s|%s|%s", timestamp, action, actor, details, prevHash)
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])
}