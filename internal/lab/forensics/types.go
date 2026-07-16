// Package forensics provides digital forensics toolkit for NAS-OS.
// Includes incident timeline analysis, evidence collection, file system analysis,
// network traffic analysis, log correlation, and forensic reporting.
//
// Key features:
//   - Incident timeline reconstruction
//   - Evidence collection with chain of custody
//   - File system metadata analysis
//   - Network connection forensics
//   - Log correlation and anomaly detection
//   - Forensic report generation
//
// References:
//   - NIST SP 800-86: Guide to Integrating Forensic Techniques
//   - RFC 3227: Guidelines for Evidence Collection and Archiving
//   - SANS Digital Forensics methodology
package forensics

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ========== Core Types ==========

// Manager is the central forensics manager.
type Manager struct {
	mu        sync.RWMutex
	cases     map[string]*Case
	timelines map[string]*Timeline
	evidence  map[string]*Evidence
	config    Config
	logger    Logger
}

// Logger interface for forensics logging.
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
}

// Config holds forensics configuration.
type Config struct {
	StoragePath       string        `json:"storagePath"`       // Base path for forensic data
	MaxCaseAge        time.Duration `json:"maxCaseAge"`        // Max age before archival
	EvidenceHashAlgo  string        `json:"evidenceHashAlgo"`  // Hash algorithm (sha256, sha512)
	TimelineMaxSize   int           `json:"timelineMaxSize"`   // Max events per timeline
	EnableAutoCollect bool          `json:"enableAutoCollect"` // Auto-collect evidence on incidents
	EncryptionKey     string        `json:"encryptionKey"`     // Key for evidence encryption
}

// Case represents a forensic investigation case.
type Case struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Status       CaseStatus `json:"status"`
	Priority     Priority   `json:"priority"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	ClosedAt     *time.Time `json:"closedAt,omitempty"`
	Investigator string     `json:"investigator"`
	Tags         []string   `json:"tags"`
	EvidenceIDs  []string   `json:"evidenceIds"`
	TimelineID   string     `json:"timelineId"`
	Notes        []Note     `json:"notes"`
	Findings     []Finding  `json:"findings"`
}

// CaseStatus represents the status of a forensic case.
type CaseStatus string

const (
	CaseStatusOpen       CaseStatus = "open"
	CaseStatusInProgress CaseStatus = "in_progress"
	CaseStatusClosed     CaseStatus = "closed"
	CaseStatusArchived   CaseStatus = "archived"
)

// Priority represents the priority level.
type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// Note represents a case note.
type Note struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

// Finding represents a forensic finding.
type Finding struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Severity    Priority  `json:"severity"`
	Category    string    `json:"category"`
	EvidenceIDs []string  `json:"evidenceIds"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Evidence represents collected forensic evidence.
type Evidence struct {
	ID             string                 `json:"id"`
	CaseID         string                 `json:"caseId"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Type           EvidenceType           `json:"type"`
	Source         string                 `json:"source"`
	CollectedAt    time.Time              `json:"collectedAt"`
	CollectedBy    string                 `json:"collectedBy"`
	Hash           string                 `json:"hash"`
	HashAlgorithm  string                 `json:"hashAlgorithm"`
	Size           int64                  `json:"size"`
	FilePath       string                 `json:"filePath"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	ChainOfCustody []CustodyEntry         `json:"chainOfCustody"`
	Tags           []string               `json:"tags"`
}

// EvidenceType represents the type of evidence.
type EvidenceType string

const (
	EvidenceTypeFile       EvidenceType = "file"
	EvidenceTypeLog        EvidenceType = "log"
	EvidenceTypeNetwork    EvidenceType = "network"
	EvidenceTypeMemory     EvidenceType = "memory"
	EvidenceTypeDisk       EvidenceType = "disk"
	EvidenceTypeConfig     EvidenceType = "config"
	EvidenceTypeScreenshot EvidenceType = "screenshot"
	EvidenceTypeDatabase   EvidenceType = "database"
)

// CustodyEntry represents a chain of custody entry.
type CustodyEntry struct {
	Action    string    `json:"action"`
	Officer   string    `json:"officer"`
	Timestamp time.Time `json:"timestamp"`
	Location  string    `json:"location"`
	Notes     string    `json:"notes"`
}

// Timeline represents an incident timeline.
type Timeline struct {
	ID        string    `json:"id"`
	CaseID    string    `json:"caseId"`
	Name      string    `json:"name"`
	Events    []Event   `json:"events"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Event represents a timeline event.
type Event struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Type        EventType              `json:"type"`
	Category    string                 `json:"category"`
	Description string                 `json:"description"`
	Source      string                 `json:"source"`
	Severity    Priority               `json:"severity"`
	Actor       string                 `json:"actor,omitempty"`
	Target      string                 `json:"target,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	EvidenceIDs []string               `json:"evidenceIds,omitempty"`
}

// EventType represents the type of timeline event.
type EventType string

const (
	EventFileAccess   EventType = "file_access"
	EventFileModify   EventType = "file_modify"
	EventFileDelete   EventType = "file_delete"
	EventFileCreate   EventType = "file_create"
	EventLogin        EventType = "login"
	EventLogout       EventType = "logout"
	EventLoginFail    EventType = "login_fail"
	EventNetworkConn  EventType = "network_connection"
	EventProcessStart EventType = "process_start"
	EventProcessStop  EventType = "process_stop"
	EventConfigChange EventType = "config_change"
	EventPrivilegeEsc EventType = "privilege_escalation"
	EventDataExfil    EventType = "data_exfiltration"
	EventMalware      EventType = "malware"
	EventAnomaly      EventType = "anomaly"
)

// ForensicReport represents a generated forensic report.
type ForensicReport struct {
	ID          string      `json:"id"`
	CaseID      string      `json:"caseId"`
	GeneratedAt time.Time   `json:"generatedAt"`
	GeneratedBy string      `json:"generatedBy"`
	Summary     string      `json:"summary"`
	Timeline    []Event     `json:"timeline"`
	Evidence    []*Evidence `json:"evidence"`
	Findings    []Finding   `json:"findings"`
	Conclusion  string      `json:"conclusion"`
	Format      string      `json:"format"` // json, html, pdf
	FilePath    string      `json:"filePath"`
}

// FileMetadata represents file system metadata for forensics.
type FileMetadata struct {
	Path          string    `json:"path"`
	Name          string    `json:"name"`
	Size          int64     `json:"size"`
	IsDir         bool      `json:"isDir"`
	ModTime       time.Time `json:"modTime"`
	AccessTime    time.Time `json:"accessTime,omitempty"`
	ChangeTime    time.Time `json:"changeTime,omitempty"`
	Mode          string    `json:"mode"`
	Owner         string    `json:"owner,omitempty"`
	Group         string    `json:"group,omitempty"`
	Inode         uint64    `json:"inode,omitempty"`
	Hash          string    `json:"hash,omitempty"`
	ContentType   string    `json:"contentType,omitempty"`
	IsHidden      bool      `json:"isHidden"`
	IsSymlink     bool      `json:"isSymlink"`
	SymlinkTarget string    `json:"symlinkTarget,omitempty"`
}

// NetworkConnection represents a network connection for forensics.
type NetworkConnection struct {
	Timestamp   time.Time `json:"timestamp"`
	Protocol    string    `json:"protocol"`
	LocalAddr   string    `json:"localAddr"`
	LocalPort   int       `json:"localPort"`
	RemoteAddr  string    `json:"remoteAddr"`
	RemotePort  int       `json:"remotePort"`
	State       string    `json:"state"`
	ProcessID   int       `json:"processId,omitempty"`
	ProcessName string    `json:"processName,omitempty"`
	BytesSent   int64     `json:"bytesSent,omitempty"`
	BytesRecv   int64     `json:"bytesRecv,omitempty"`
	IsEncrypted bool      `json:"isEncrypted"`
	ThreatLevel Priority  `json:"threatLevel,omitempty"`
	GeoLocation string    `json:"geoLocation,omitempty"`
}

// ========== Manager Implementation ==========

// NewManager creates a new forensics manager.
func NewManager(config Config, logger Logger) (*Manager, error) {
	if config.StoragePath == "" {
		config.StoragePath = "/var/lib/nas-os/forensics"
	}
	if config.EvidenceHashAlgo == "" {
		config.EvidenceHashAlgo = "sha256"
	}
	if config.TimelineMaxSize == 0 {
		config.TimelineMaxSize = 100000
	}
	if config.MaxCaseAge == 0 {
		config.MaxCaseAge = 365 * 24 * time.Hour // 1 year
	}

	// Create storage directories
	dirs := []string{
		filepath.Join(config.StoragePath, "cases"),
		filepath.Join(config.StoragePath, "evidence"),
		filepath.Join(config.StoragePath, "timelines"),
		filepath.Join(config.StoragePath, "reports"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("create forensics directory %s: %w", dir, err)
		}
	}

	m := &Manager{
		cases:     make(map[string]*Case),
		timelines: make(map[string]*Timeline),
		evidence:  make(map[string]*Evidence),
		config:    config,
		logger:    logger,
	}

	// Load existing cases
	if err := m.loadCases(); err != nil {
		logger.Warn("failed to load existing cases", "error", err)
	}

	return m, nil
}

// ========== Case Management ==========

// CreateCase creates a new forensic case.
func (m *Manager) CreateCase(name, description, investigator string, priority Priority, tags []string) (*Case, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generateID("case")
	now := time.Now()

	case_ := &Case{
		ID:           id,
		Name:         name,
		Description:  description,
		Status:       CaseStatusOpen,
		Priority:     priority,
		CreatedAt:    now,
		UpdatedAt:    now,
		Investigator: investigator,
		Tags:         tags,
		EvidenceIDs:  []string{},
		Notes:        []Note{},
		Findings:     []Finding{},
	}

	// Create associated timeline
	timelineID := generateID("tl")
	timeline := &Timeline{
		ID:        timelineID,
		CaseID:    id,
		Name:      fmt.Sprintf("Timeline for %s", name),
		Events:    []Event{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.timelines[timelineID] = timeline
	case_.TimelineID = timelineID

	m.cases[id] = case_

	if err := m.saveCase(case_); err != nil {
		return nil, fmt.Errorf("save case: %w", err)
	}

	m.logger.Info("forensic case created", "caseId", id, "name", name)
	return case_, nil
}

// GetCase returns a case by ID.
func (m *Manager) GetCase(id string) (*Case, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	case_, exists := m.cases[id]
	if !exists {
		return nil, fmt.Errorf("case %s not found", id)
	}
	return case_, nil
}

// ListCases returns all cases.
func (m *Manager) ListCases(status CaseStatus) []*Case {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cases := make([]*Case, 0, len(m.cases))
	for _, c := range m.cases {
		if status == "" || c.Status == status {
			cases = append(cases, c)
		}
	}

	sort.Slice(cases, func(i, j int) bool {
		return cases[i].CreatedAt.After(cases[j].CreatedAt)
	})

	return cases
}

// UpdateCaseStatus updates the status of a case.
func (m *Manager) UpdateCaseStatus(id string, status CaseStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	case_, exists := m.cases[id]
	if !exists {
		return fmt.Errorf("case %s not found", id)
	}

	case_.Status = status
	case_.UpdatedAt = time.Now()

	if status == CaseStatusClosed {
		now := time.Now()
		case_.ClosedAt = &now
	}

	return m.saveCase(case_)
}

// AddNote adds a note to a case.
func (m *Manager) AddNote(caseID, author, content string) (*Note, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	case_, exists := m.cases[caseID]
	if !exists {
		return nil, fmt.Errorf("case %s not found", caseID)
	}

	note := &Note{
		ID:        generateID("note"),
		Author:    author,
		Content:   content,
		CreatedAt: time.Now(),
	}

	case_.Notes = append(case_.Notes, *note)
	case_.UpdatedAt = time.Now()

	if err := m.saveCase(case_); err != nil {
		return nil, fmt.Errorf("save case: %w", err)
	}

	return note, nil
}

// AddFinding adds a finding to a case.
func (m *Manager) AddFinding(caseID, title, description, category string, severity Priority, evidenceIDs []string) (*Finding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	case_, exists := m.cases[caseID]
	if !exists {
		return nil, fmt.Errorf("case %s not found", caseID)
	}

	finding := &Finding{
		ID:          generateID("find"),
		Title:       title,
		Description: description,
		Severity:    severity,
		Category:    category,
		EvidenceIDs: evidenceIDs,
		CreatedAt:   time.Now(),
	}

	case_.Findings = append(case_.Findings, *finding)
	case_.UpdatedAt = time.Now()

	if err := m.saveCase(case_); err != nil {
		return nil, fmt.Errorf("save case: %w", err)
	}

	m.logger.Info("finding added", "caseId", caseID, "findingId", finding.ID)
	return finding, nil
}

// ========== Evidence Management ==========

// CollectFileEvidence collects evidence from a file.
func (m *Manager) CollectFileEvidence(caseID, filePath, description, collector string) (*Evidence, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	case_, exists := m.cases[caseID]
	if !exists {
		return nil, fmt.Errorf("case %s not found", caseID)
	}

	// Get file info
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	// Calculate hash
	hash, err := hashFile(filePath, m.config.EvidenceHashAlgo)
	if err != nil {
		return nil, fmt.Errorf("hash file: %w", err)
	}

	// Copy evidence to storage
	evidenceID := generateID("ev")
	evidencePath := filepath.Join(m.config.StoragePath, "evidence", evidenceID, filepath.Base(filePath))
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0750); err != nil {
		return nil, fmt.Errorf("create evidence directory: %w", err)
	}

	if err := copyFile(filePath, evidencePath); err != nil {
		return nil, fmt.Errorf("copy evidence: %w", err)
	}

	now := time.Now()
	evidence := &Evidence{
		ID:            evidenceID,
		CaseID:        caseID,
		Name:          filepath.Base(filePath),
		Description:   description,
		Type:          EvidenceTypeFile,
		Source:        filePath,
		CollectedAt:   now,
		CollectedBy:   collector,
		Hash:          hash,
		HashAlgorithm: m.config.EvidenceHashAlgo,
		Size:          info.Size(),
		FilePath:      evidencePath,
		ChainOfCustody: []CustodyEntry{
			{
				Action:    "collected",
				Officer:   collector,
				Timestamp: now,
				Location:  filePath,
				Notes:     "Initial collection",
			},
		},
		Tags: []string{},
	}

	m.evidence[evidenceID] = evidence
	case_.EvidenceIDs = append(case_.EvidenceIDs, evidenceID)
	case_.UpdatedAt = now

	if err := m.saveCase(case_); err != nil {
		return nil, fmt.Errorf("save case: %w", err)
	}
	if err := m.saveEvidence(evidence); err != nil {
		return nil, fmt.Errorf("save evidence: %w", err)
	}

	m.logger.Info("evidence collected", "evidenceId", evidenceID, "source", filePath)
	return evidence, nil
}

// CollectLogEvidence collects evidence from log entries.
func (m *Manager) CollectLogEvidence(caseID string, logEntries []map[string]string, description, collector string) (*Evidence, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	case_, exists := m.cases[caseID]
	if !exists {
		return nil, fmt.Errorf("case %s not found", caseID)
	}

	evidenceID := generateID("ev")
	evidencePath := filepath.Join(m.config.StoragePath, "evidence", evidenceID, "logs.json")

	// Marshal log entries
	data, err := json.MarshalIndent(logEntries, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal logs: %w", err)
	}

	// Write to file
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0750); err != nil {
		return nil, fmt.Errorf("create evidence directory: %w", err)
	}
	if err := os.WriteFile(evidencePath, data, 0640); err != nil {
		return nil, fmt.Errorf("write logs: %w", err)
	}

	// Calculate hash
	hash := sha256.Sum256(data)

	now := time.Now()
	evidence := &Evidence{
		ID:            evidenceID,
		CaseID:        caseID,
		Name:          "Log Evidence",
		Description:   description,
		Type:          EvidenceTypeLog,
		Source:        "system_logs",
		CollectedAt:   now,
		CollectedBy:   collector,
		Hash:          hex.EncodeToString(hash[:]),
		HashAlgorithm: "sha256",
		Size:          int64(len(data)),
		FilePath:      evidencePath,
		ChainOfCustody: []CustodyEntry{
			{
				Action:    "collected",
				Officer:   collector,
				Timestamp: now,
				Location:  "system_logs",
				Notes:     fmt.Sprintf("Collected %d log entries", len(logEntries)),
			},
		},
		Tags: []string{},
	}

	m.evidence[evidenceID] = evidence
	case_.EvidenceIDs = append(case_.EvidenceIDs, evidenceID)
	case_.UpdatedAt = now

	if err := m.saveCase(case_); err != nil {
		return nil, fmt.Errorf("save case: %w", err)
	}
	if err := m.saveEvidence(evidence); err != nil {
		return nil, fmt.Errorf("save evidence: %w", err)
	}

	m.logger.Info("log evidence collected", "evidenceId", evidenceID, "entries", len(logEntries))
	return evidence, nil
}

// GetEvidence returns evidence by ID.
func (m *Manager) GetEvidence(id string) (*Evidence, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	evidence, exists := m.evidence[id]
	if !exists {
		return nil, fmt.Errorf("evidence %s not found", id)
	}
	return evidence, nil
}

// ListEvidence returns all evidence for a case.
func (m *Manager) ListEvidence(caseID string) []*Evidence {
	m.mu.RLock()
	defer m.mu.RUnlock()

	evidence := make([]*Evidence, 0)
	for _, e := range m.evidence {
		if e.CaseID == caseID {
			evidence = append(evidence, e)
		}
	}

	sort.Slice(evidence, func(i, j int) bool {
		return evidence[i].CollectedAt.After(evidence[j].CollectedAt)
	})

	return evidence
}

// VerifyEvidence verifies evidence integrity.
func (m *Manager) VerifyEvidence(id string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	evidence, exists := m.evidence[id]
	if !exists {
		return false, fmt.Errorf("evidence %s not found", id)
	}

	// Calculate current hash
	currentHash, err := hashFile(evidence.FilePath, evidence.HashAlgorithm)
	if err != nil {
		return false, fmt.Errorf("hash file: %w", err)
	}

	return currentHash == evidence.Hash, nil
}

// TransferCustody transfers custody of evidence.
func (m *Manager) TransferCustody(evidenceID, fromOfficer, toOfficer, location, notes string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	evidence, exists := m.evidence[evidenceID]
	if !exists {
		return fmt.Errorf("evidence %s not found", evidenceID)
	}

	entry := CustodyEntry{
		Action:    "transferred",
		Officer:   toOfficer,
		Timestamp: time.Now(),
		Location:  location,
		Notes:     fmt.Sprintf("Transferred from %s. %s", fromOfficer, notes),
	}

	evidence.ChainOfCustody = append(evidence.ChainOfCustody, entry)

	return m.saveEvidence(evidence)
}

// ========== Timeline Management ==========

// AddTimelineEvent adds an event to a timeline.
func (m *Manager) AddTimelineEvent(timelineID string, event Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	timeline, exists := m.timelines[timelineID]
	if !exists {
		return fmt.Errorf("timeline %s not found", timelineID)
	}

	if event.ID == "" {
		event.ID = generateID("evt")
	}

	timeline.Events = append(timeline.Events, event)
	timeline.UpdatedAt = time.Now()

	// Sort events by timestamp
	sort.Slice(timeline.Events, func(i, j int) bool {
		return timeline.Events[i].Timestamp.Before(timeline.Events[j].Timestamp)
	})

	// Trim if exceeds max size
	if len(timeline.Events) > m.config.TimelineMaxSize {
		timeline.Events = timeline.Events[len(timeline.Events)-m.config.TimelineMaxSize:]
	}

	return m.saveTimeline(timeline)
}

// GetTimeline returns a timeline by ID.
func (m *Manager) GetTimeline(id string) (*Timeline, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	timeline, exists := m.timelines[id]
	if !exists {
		return nil, fmt.Errorf("timeline %s not found", id)
	}
	return timeline, nil
}

// GetCaseTimeline returns the timeline for a case.
func (m *Manager) GetCaseTimeline(caseID string) (*Timeline, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, t := range m.timelines {
		if t.CaseID == caseID {
			return t, nil
		}
	}
	return nil, fmt.Errorf("timeline for case %s not found", caseID)
}

// ========== File System Analysis ==========

// AnalyzeDirectory analyzes a directory and returns file metadata.
func (m *Manager) AnalyzeDirectory(dirPath string, recursive bool) ([]*FileMetadata, error) {
	files := make([]*FileMetadata, 0)

	walkFunc := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		meta := &FileMetadata{
			Path:      path,
			Name:      info.Name(),
			Size:      info.Size(),
			IsDir:     info.IsDir(),
			ModTime:   info.ModTime(),
			Mode:      info.Mode().String(),
			IsHidden:  strings.HasPrefix(info.Name(), "."),
			IsSymlink: info.Mode()&os.ModeSymlink != 0,
		}

		// Get owner/group (Unix only)
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			meta.Inode = stat.Ino
		}

		files = append(files, meta)

		if !recursive && info.IsDir() && path != dirPath {
			return filepath.SkipDir
		}

		return nil
	}

	if err := filepath.WalkDir(dirPath, walkFunc); err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}

	return files, nil
}

// FindModifiedFiles finds files modified within a time range.
func (m *Manager) FindModifiedFiles(dirPath string, start, end time.Time) ([]*FileMetadata, error) {
	files, err := m.AnalyzeDirectory(dirPath, true)
	if err != nil {
		return nil, err
	}

	modified := make([]*FileMetadata, 0)
	for _, f := range files {
		if !f.IsDir && f.ModTime.After(start) && f.ModTime.Before(end) {
			modified = append(modified, f)
		}
	}

	return modified, nil
}

// FindDeletedFiles finds recently deleted files (by comparing snapshots).
func (m *Manager) FindDeletedFiles(before, after []string) []string {
	beforeSet := make(map[string]bool)
	for _, f := range before {
		beforeSet[f] = true
	}

	deleted := make([]string, 0)
	for _, f := range before {
		found := false
		for _, a := range after {
			if a == f {
				found = true
				break
			}
		}
		if !found {
			deleted = append(deleted, f)
		}
	}

	return deleted
}

// ========== Network Forensics ==========

// AnalyzeNetworkConnections analyzes network connections for suspicious activity.
func (m *Manager) AnalyzeNetworkConnections(connections []NetworkConnection) []NetworkConnection {
	suspicious := make([]NetworkConnection, 0)

	for _, conn := range connections {
		isSuspicious := false

		// Check for unusual ports
		if isUnusualPort(conn.RemotePort) {
			conn.ThreatLevel = PriorityMedium
			isSuspicious = true
		}

		// Check for known malicious IPs (simplified)
		if isKnownMalicious(conn.RemoteAddr) {
			conn.ThreatLevel = PriorityHigh
			isSuspicious = true
		}

		// Check for data exfiltration patterns
		if conn.BytesSent > 100*1024*1024 { // > 100MB sent
			conn.ThreatLevel = PriorityHigh
			isSuspicious = true
		}

		if isSuspicious {
			suspicious = append(suspicious, conn)
		}
	}

	return suspicious
}

// ScanSecurity performs a quick security scan on a directory path.
func (m *Manager) ScanSecurity(path string, includeNetwork bool) (*SecurityScanResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := &SecurityScanResult{
		ScanID:    generateID("scan"),
		Timestamp: time.Now(),
		Path:      path,
		Issues:    make([]SecurityIssue, 0),
	}

	// Analyze directory
	files, err := m.AnalyzeDirectory(path, true)
	if err != nil {
		return nil, fmt.Errorf("analyze directory: %w", err)
	}

	result.FilesScanned = len(files)

	// Check for suspicious files
	for _, f := range files {
		if f.IsDir {
			continue
		}

		// Check for hidden files
		if f.IsHidden {
			result.Issues = append(result.Issues, SecurityIssue{
				Type:        "hidden_file",
				Severity:    "medium",
				Path:        f.Path,
				Description: "Hidden file detected",
			})
		}

		// Check for symlinks
		if f.IsSymlink {
			result.Issues = append(result.Issues, SecurityIssue{
				Type:        "symlink",
				Severity:    "low",
				Path:        f.Path,
				Description: fmt.Sprintf("Symlink to: %s", f.SymlinkTarget),
			})
		}

		// Check for large files (>100MB)
		if f.Size > 100*1024*1024 {
			result.Issues = append(result.Issues, SecurityIssue{
				Type:        "large_file",
				Severity:    "low",
				Path:        f.Path,
				Description: fmt.Sprintf("Large file: %d bytes", f.Size),
			})
		}
	}

	result.IssuesFound = len(result.Issues)

	// Generate summary
	if result.IssuesFound == 0 {
		result.Summary = "No security issues found"
	} else {
		result.Summary = fmt.Sprintf("Found %d security issues in %d files", result.IssuesFound, result.FilesScanned)
	}

	m.logger.Info("security scan completed", "path", path, "issues", result.IssuesFound)
	return result, nil
}

// ========== Report Generation ==========

// GenerateReport generates a forensic report for a case.
func (m *Manager) GenerateReport(caseID, investigator, format string) (*ForensicReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	case_, exists := m.cases[caseID]
	if !exists {
		return nil, fmt.Errorf("case %s not found", caseID)
	}

	// Get timeline
	timeline, _ := m.GetCaseTimeline(caseID)
	var events []Event
	if timeline != nil {
		events = timeline.Events
	}

	// Get evidence
	evidenceList := m.ListEvidence(caseID)

	report := &ForensicReport{
		ID:          generateID("rpt"),
		CaseID:      caseID,
		GeneratedAt: time.Now(),
		GeneratedBy: investigator,
		Summary:     case_.Description,
		Timeline:    events,
		Evidence:    evidenceList,
		Findings:    case_.Findings,
		Format:      format,
	}

	// Generate report file
	reportPath := filepath.Join(m.config.StoragePath, "reports", fmt.Sprintf("%s.%s", report.ID, format))
	if err := m.saveReport(report, reportPath); err != nil {
		return nil, fmt.Errorf("save report: %w", err)
	}
	report.FilePath = reportPath

	return report, nil
}

// ========== Helper Functions ==========

func generateID(prefix string) string {
	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UnixNano(), randomHex(4))
}

func randomHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(time.Now().UnixNano() >> (uint(i) * 8))
	}
	return hex.EncodeToString(b)
}

func hashFile(path, algo string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := fmt.Fprint(h, ""); err != nil {
		return "", err
	}

	// Read file and hash
	buf := make([]byte, 64*1024)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = fmt.Fprint(out, "")
	if err != nil {
		return err
	}

	_, err = out.ReadFrom(in)
	return err
}

func isUnusualPort(port int) bool {
	unusualPorts := map[int]bool{
		4444: true, 5555: true, 6666: true, 7777: true,
		8888: true, 9999: true, 1234: true, 31337: true,
	}
	return unusualPorts[port]
}

func isKnownMalicious(ip string) bool {
	// Simplified check - in production, use threat intelligence feeds
	maliciousPrefixes := []string{
		"10.0.0.", "192.168.", "172.16.",
	}
	for _, prefix := range maliciousPrefixes {
		if strings.HasPrefix(ip, prefix) {
			return false // Internal IPs are not malicious
		}
	}
	return false
}

// ========== Persistence ==========

func (m *Manager) saveCase(c *Case) error {
	path := filepath.Join(m.config.StoragePath, "cases", c.ID+".json")
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0640)
}

func (m *Manager) loadCases() error {
	casesDir := filepath.Join(m.config.StoragePath, "cases")
	entries, err := os.ReadDir(casesDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(casesDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var c Case
		if err := json.Unmarshal(data, &c); err != nil {
			continue
		}

		m.cases[c.ID] = &c
	}

	return nil
}

func (m *Manager) saveEvidence(e *Evidence) error {
	path := filepath.Join(m.config.StoragePath, "evidence", e.ID+".json")
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0640)
}

func (m *Manager) saveTimeline(t *Timeline) error {
	path := filepath.Join(m.config.StoragePath, "timelines", t.ID+".json")
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0640)
}

func (m *Manager) saveReport(r *ForensicReport, path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0640)
}
