// Package forensics2 provides digital forensics capabilities for the NAS OS.
// It implements file system forensic snapshots, audit chain verification,
// evidence preservation management, and forensic report generation to support
// incident investigation and compliance auditing.
package forensics2

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Core Structs
// ---------------------------------------------------------------------------

// ForensicSnapshot captures a point-in-time forensic image of a file system
// path, including file metadata and cryptographic hashes.
type ForensicSnapshot struct {
	ID          string       // Unique snapshot identifier
	SnapshotPath string      // File system path that was snapshotted
	Timestamp   time.Time    // When the snapshot was taken
	Entries     []SnapshotEntry // Individual file entries captured
	TotalFiles  int          // Total number of files captured
	TotalBytes  int64        // Total size of captured files
	HashChain   string       // SHA-256 of the entire entry chain
	mu          sync.Mutex
}

// SnapshotEntry represents a single file within a forensic snapshot.
type SnapshotEntry struct {
	Path     string    // Full file path
	Size     int64     // File size in bytes
	Mode     os.FileMode // File permissions
	ModTime  time.Time // Last modification time
	SHA256   string    // SHA-256 hash of file content
	IsDir    bool      // Whether entry is a directory
}

// AuditChain maintains a tamper-evident chain of audited operations using
// linked SHA-256 hashes.
type AuditChain struct {
	ID       string        // Chain identifier
	Head     string        // Hash of the latest entry (chain head)
	Entries  []AuditEntry  // Ordered list of audit entries
	mu       sync.Mutex
}

// AuditEntry is a single record in the audit chain.
type AuditEntry struct {
	Index     int       // Sequential index in the chain
	Action    string    // What action was performed (e.g., "read", "write", "delete")
	Actor     string    // Who performed the action
	Resource  string    // Target resource (file path, object, etc.)
	Timestamp time.Time // When the action occurred
	PrevHash  string    // Hash of the previous entry
	ThisHash  string    // Hash of this entry
	Metadata  map[string]string // Additional context
}

// EvidencePack represents sealed, tamper-evident evidence collected during
// an investigation.
type EvidencePack struct {
	ID           string         // Evidence pack identifier
	CaseID       string         // Associated case/incident ID
	Collector    string         // Who collected the evidence
	CollectedAt  time.Time     // Collection timestamp
	SnapshotID   string         // Link to ForensicSnapshot
	AuditChainID string        // Link to AuditChain
	Sealed       bool           // Whether evidence has been sealed
	SealHash     string        // Hash that locks the evidence
	Description  string         // Human-readable description
	Items        []EvidenceItem // Individual evidence items
}

// EvidenceItem is a single piece of evidence within an EvidencePack.
type EvidenceItem struct {
	Name        string    // Item name/description
	Source      string    // Where the item was collected from
	Hash        string    // SHA-256 hash of the item content
	CollectedAt time.Time // When this item was collected
}

// ForensicReport is a generated report summarizing forensic findings.
type ForensicReport struct {
	ID           string    // Report identifier
	CaseID       string    // Associated case/incident ID
	GeneratedAt  time.Time // When the report was generated
	GeneratedBy  string    // Who or what generated the report
	Summary      string    // Executive summary
	SnapshotID   string    // Reference to the forensic snapshot used
	AuditChainID string   // Reference to the audit chain used
	Findings     []Finding // Individual findings
	Timeline     []TimelineEvent // Chronological events
	Status       string    // "draft", "final", "archived"
}

// Finding represents a single forensic finding in a report.
type Finding struct {
	Title       string // Short title
	Description string // Detailed description
	Severity    string // "info", "low", "medium", "high", "critical"
	Evidence    []string // References to evidence items
}

// TimelineEvent represents a chronologically ordered event in the report.
type TimelineEvent struct {
	Timestamp time.Time
	Event     string
	Actor     string
}

// ---------------------------------------------------------------------------
// ForensicSnapshot Methods
// ---------------------------------------------------------------------------

// TakeSnapshot captures a forensic snapshot of the specified path, recording
// file metadata and content hashes for all files found.
func (s *ForensicSnapshot) TakeSnapshot(rootPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.SnapshotPath = rootPath
	s.Timestamp = time.Now()
	s.Entries = s.Entries[:0] // reset
	s.TotalFiles = 0
	s.TotalBytes = 0

	walkErr := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Record the error as a special entry but continue
			s.Entries = append(s.Entries, SnapshotEntry{
				Path:    path,
				ModTime: time.Now(),
				IsDir:   false,
				SHA256:  "error:" + err.Error(),
			})
			return nil
		}

		entry := SnapshotEntry{
			Path:    path,
			Size:    info.Size(),
			Mode:    info.Mode(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
		}

		if !info.IsDir() {
			hash, hErr := hashFile(path)
			if hErr != nil {
				entry.SHA256 = "error:" + hErr.Error()
			} else {
				entry.SHA256 = hash
			}
			s.TotalFiles++
			s.TotalBytes += info.Size()
		}

		s.Entries = append(s.Entries, entry)
		return nil
	})

	if walkErr != nil {
		return fmt.Errorf("snapshot walk failed: %w", walkErr)
	}

	// Compute chain hash over all entries
	s.HashChain = s.computeChainHash()
	return nil
}

// computeChainHash produces a rolling SHA-256 over all entry hashes.
func (s *ForensicSnapshot) computeChainHash() string {
	h := sha256.New()
	for _, e := range s.Entries {
		h.Write([]byte(e.Path))
		h.Write([]byte(e.SHA256))
		h.Write([]byte(e.ModTime.String()))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ---------------------------------------------------------------------------
// AuditChain Methods
// ---------------------------------------------------------------------------

// AppendEntry adds a new audit entry to the chain, linking it to the
// previous entry via its hash to maintain tamper-evidence.
func (c *AuditChain) AppendEntry(action, actor, resource string, metadata map[string]string) AuditEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	index := len(c.Entries)
	prevHash := c.Head
	if prevHash == "" {
		prevHash = "genesis"
	}

	entry := AuditEntry{
		Index:     index,
		Action:    action,
		Actor:     actor,
		Resource:  resource,
		Timestamp: time.Now(),
		PrevHash:  prevHash,
		Metadata:  metadata,
	}

	// Compute this entry's hash
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%d|%s|%s|%s|%s|%s", index, action, actor, resource, entry.Timestamp.Format(time.RFC3339Nano), prevHash)))
	for k, v := range metadata {
		h.Write([]byte(k))
		h.Write([]byte(v))
	}
	entry.ThisHash = hex.EncodeToString(h.Sum(nil))

	c.Entries = append(c.Entries, entry)
	c.Head = entry.ThisHash
	return entry
}

// VerifyChain validates the integrity of the audit chain by recomputing
// each entry's hash and verifying the linked hashes match. Returns true if
// the chain is intact, false with an error describing the break otherwise.
func (c *AuditChain) VerifyChain() (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prev := "genesis"
	for i, entry := range c.Entries {
		if entry.PrevHash != prev {
			return false, fmt.Errorf("chain broken at index %d: expected prevHash %s, got %s", i, prev, entry.PrevHash)
		}
		// Recompute hash
		h := sha256.New()
		h.Write([]byte(fmt.Sprintf("%d|%s|%s|%s|%s|%s", entry.Index, entry.Action, entry.Actor, entry.Resource, entry.Timestamp.Format(time.RFC3339Nano), entry.PrevHash)))
		for k, v := range entry.Metadata {
			h.Write([]byte(k))
			h.Write([]byte(v))
		}
		expected := hex.EncodeToString(h.Sum(nil))
		if entry.ThisHash != expected {
			return false, fmt.Errorf("hash mismatch at index %d: expected %s, got %s", i, expected, entry.ThisHash)
		}
		prev = entry.ThisHash
	}
	return true, nil
}

// ---------------------------------------------------------------------------
// EvidencePack Methods
// ---------------------------------------------------------------------------

// SealEvidence locks the evidence pack so it can no longer be modified.
// It computes a final seal hash over all items and marks the pack as sealed.
func (e *EvidencePack) SealEvidence() (string, error) {
	h := sha256.New()
	h.Write([]byte(e.ID))
	h.Write([]byte(e.CaseID))
	h.Write([]byte(e.Collector))
	h.Write([]byte(e.CollectedAt.Format(time.RFC3339Nano)))
	h.Write([]byte(e.Description))
	for _, item := range e.Items {
		h.Write([]byte(item.Name))
		h.Write([]byte(item.Source))
		h.Write([]byte(item.Hash))
		h.Write([]byte(item.CollectedAt.Format(time.RFC3339Nano)))
	}

	sealHash := hex.EncodeToString(h.Sum(nil))
	e.SealHash = sealHash
	e.Sealed = true
	return sealHash, nil
}

// AddItem adds a new evidence item to the pack. Once sealed, no items can be
// added.
func (e *EvidencePack) AddItem(name, source, contentHash string) error {
	if e.Sealed {
		return fmt.Errorf("evidence pack %s is sealed and cannot be modified", e.ID)
	}
	e.Items = append(e.Items, EvidenceItem{
		Name:        name,
		Source:      source,
		Hash:        contentHash,
		CollectedAt: time.Now(),
	})
	return nil
}

// ---------------------------------------------------------------------------
// ForensicReport Methods
// ---------------------------------------------------------------------------

// GenerateReport compiles findings and timeline events into a structured
// forensic report from the given snapshot and audit chain.
func (r *ForensicReport) GenerateReport(snapshot *ForensicSnapshot, chain *AuditChain, generatedBy string) error {
	r.GeneratedAt = time.Now()
	r.GeneratedBy = generatedBy
	r.Status = "draft"

	if snapshot != nil {
		r.SnapshotID = snapshot.ID
		r.Summary = fmt.Sprintf("Snapshot covers %s with %d files (%d bytes), chain hash: %s",
			snapshot.SnapshotPath, snapshot.TotalFiles, snapshot.TotalBytes, snapshot.HashChain)
	} else {
		r.Summary = "No snapshot data available."
	}

	if chain != nil {
		r.AuditChainID = chain.ID
		ok, err := chain.VerifyChain()
		if ok {
			r.Summary += fmt.Sprintf(" Audit chain %s verified (%d entries).", chain.ID, len(chain.Entries))
		} else {
			r.Summary += fmt.Sprintf(" Audit chain %s verification FAILED: %v.", chain.ID, err)
			r.Findings = append(r.Findings, Finding{
				Title:       "Audit Chain Integrity Failure",
				Description: fmt.Sprintf("Verification failed: %v", err),
				Severity:    "critical",
			})
		}
		// Build timeline from audit entries
		for _, entry := range chain.Entries {
			r.Timeline = append(r.Timeline, TimelineEvent{
				Timestamp: entry.Timestamp,
				Event:     fmt.Sprintf("%s %s by %s", entry.Action, entry.Resource, entry.Actor),
				Actor:     entry.Actor,
			})
		}
	}

	// Detect anomalies from snapshot entries
	if snapshot != nil {
		for _, entry := range snapshot.Entries {
			if len(entry.SHA256) > 6 && entry.SHA256[:6] == "error:" {
				r.Findings = append(r.Findings, Finding{
					Title:       "File Hash Error",
					Description: fmt.Sprintf("Could not hash file %s: %s", entry.Path, entry.SHA256[6:]),
					Severity:    "medium",
				})
			}
		}
	}

	if len(r.Findings) == 0 {
		r.Findings = append(r.Findings, Finding{
			Title:       "No Anomalies Detected",
			Description: "The forensic analysis did not detect any anomalies or integrity issues.",
			Severity:    "info",
		})
	}

	return nil
}

// Finalize marks the report as final, preventing further modifications.
func (r *ForensicReport) Finalize() {
	r.Status = "final"
}

// Archive marks the report as archived.
func (r *ForensicReport) Archive() {
	r.Status = "archived"
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// hashFile reads a file and returns its SHA-256 hex digest.
func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}