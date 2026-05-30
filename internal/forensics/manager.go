package forensics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// API provides REST API handlers for forensics.
type API struct {
	manager *Manager
}

// NewAPI creates a new forensics API.
func NewAPI(manager *Manager) *API {
	return &API{manager: manager}
}

// RegisterRoutes registers forensics API routes.
func (a *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/forensics/cases", a.handleCases)
	mux.HandleFunc("/api/forensics/cases/", a.handleCaseByID)
	mux.HandleFunc("/api/forensics/cases/evidence", a.handleCaseEvidence)
	mux.HandleFunc("/api/forensics/cases/timeline", a.handleCaseTimeline)
	mux.HandleFunc("/api/forensics/cases/report", a.handleCaseReport)
	mux.HandleFunc("/api/forensics/evidence", a.handleEvidence)
	mux.HandleFunc("/api/forensics/evidence/", a.handleEvidenceByID)
	mux.HandleFunc("/api/forensics/timeline", a.handleTimeline)
	mux.HandleFunc("/api/forensics/reports", a.handleReports)
	mux.HandleFunc("/api/forensics/scan", a.handleSecurityScan)
	mux.HandleFunc("/api/forensics/analyze/filesystem", a.handleFileSystemAnalysis)
	mux.HandleFunc("/api/forensics/analyze/network", a.handleNetworkAnalysis)
}

// handleCases handles case CRUD operations.
func (a *API) handleCases(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listCases(w, r)
	case http.MethodPost:
		a.createCase(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCaseByID handles operations on a specific case.
func (a *API) handleCaseByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/forensics/cases/"):]
	if id == "" {
		http.Error(w, "case ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		a.getCase(w, id)
	case http.MethodPut:
		a.updateCase(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCaseEvidence handles evidence collection for a case.
func (a *API) handleCaseEvidence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract case ID from path: /api/v1/forensics/cases/evidence?caseId=xxx
	caseID := r.URL.Query().Get("caseId")
	if caseID == "" {
		http.Error(w, "caseId is required", http.StatusBadRequest)
		return
	}

	var req CollectEvidenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	req.CaseID = caseID

	var evidence *Evidence
	var err error

	switch EvidenceType(req.Type) {
	case EvidenceTypeLog:
		var entries []map[string]string
		if err := json.Unmarshal([]byte(req.Source), &entries); err != nil {
			http.Error(w, "invalid log entries format", http.StatusBadRequest)
			return
		}
		evidence, err = a.manager.CollectLogEvidence(req.CaseID, entries, req.Description, req.Collector)
	default:
		evidence, err = a.manager.CollectFileEvidence(req.CaseID, req.Source, req.Description, req.Collector)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, evidence)
}

// handleCaseTimeline handles timeline operations for a case.
func (a *API) handleCaseTimeline(w http.ResponseWriter, r *http.Request) {
	caseID := r.URL.Query().Get("caseId")
	if caseID == "" {
		http.Error(w, "caseId is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		timeline, err := a.manager.GetCaseTimeline(caseID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, timeline)
	case http.MethodPost:
		var req AddEventRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		// Get timeline for case
		timeline, err := a.manager.GetCaseTimeline(caseID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		event := Event{
			Timestamp:   req.Timestamp,
			Type:        EventType(req.Type),
			Category:    req.Category,
			Description: req.Description,
			Source:      req.Source,
			Severity:    Priority(req.Severity),
			Actor:       req.Actor,
			Target:      req.Target,
			Details:     req.Details,
		}

		if err := a.manager.AddTimelineEvent(timeline.ID, event); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusCreated, event)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCaseReport handles report generation for a case.
func (a *API) handleCaseReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	caseID := r.URL.Query().Get("caseId")
	if caseID == "" {
		http.Error(w, "caseId is required", http.StatusBadRequest)
		return
	}

	investigator := r.URL.Query().Get("investigator")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	report, err := a.manager.GenerateReport(caseID, investigator, format)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, report)
}

// handleSecurityScan handles quick security scan requests.
func (a *API) handleSecurityScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SecurityScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	result, err := a.manager.ScanSecurity(req.Path, req.IncludeNetwork)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleEvidence handles evidence operations.
func (a *API) handleEvidence(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listEvidence(w, r)
	case http.MethodPost:
		a.collectEvidence(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleEvidenceByID handles operations on specific evidence.
func (a *API) handleEvidenceByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/forensics/evidence/"):]
	if id == "" {
		http.Error(w, "evidence ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		a.getEvidence(w, id)
	case http.MethodPost:
		if r.URL.Query().Get("action") == "verify" {
			a.verifyEvidence(w, id)
		} else if r.URL.Query().Get("action") == "transfer" {
			a.transferCustody(w, r, id)
		} else {
			http.Error(w, "invalid action", http.StatusBadRequest)
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTimeline handles timeline operations.
func (a *API) handleTimeline(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.getTimeline(w, r)
	case http.MethodPost:
		a.addTimelineEvent(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleReports handles report generation.
func (a *API) handleReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.generateReport(w, r)
}

// handleFileSystemAnalysis handles file system analysis requests.
func (a *API) handleFileSystemAnalysis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.analyzeFileSystem(w, r)
}

// handleNetworkAnalysis handles network analysis requests.
func (a *API) handleNetworkAnalysis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.analyzeNetwork(w, r)
}

// ========== Case Operations ==========

type CreateCaseRequest struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Investigator string   `json:"investigator"`
	Priority     string   `json:"priority"`
	Tags         []string `json:"tags"`
}

func (a *API) createCase(w http.ResponseWriter, r *http.Request) {
	var req CreateCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	priority := Priority(req.Priority)
	if priority == "" {
		priority = PriorityMedium
	}

	case_, err := a.manager.CreateCase(req.Name, req.Description, req.Investigator, priority, req.Tags)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, case_)
}

func (a *API) listCases(w http.ResponseWriter, r *http.Request) {
	status := CaseStatus(r.URL.Query().Get("status"))
	cases := a.manager.ListCases(status)
	writeJSON(w, http.StatusOK, cases)
}

func (a *API) getCase(w http.ResponseWriter, id string) {
	case_, err := a.manager.GetCase(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, case_)
}

type UpdateCaseRequest struct {
	Status string `json:"status"`
}

func (a *API) updateCase(w http.ResponseWriter, r *http.Request, id string) {
	var req UpdateCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Status != "" {
		if err := a.manager.UpdateCaseStatus(id, CaseStatus(req.Status)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	case_, _ := a.manager.GetCase(id)
	writeJSON(w, http.StatusOK, case_)
}

// ========== Evidence Operations ==========

type CollectEvidenceRequest struct {
	CaseID      string `json:"caseId"`
	Source      string `json:"source"`
	Description string `json:"description"`
	Collector   string `json:"collector"`
	Type        string `json:"type"`
}

func (a *API) collectEvidence(w http.ResponseWriter, r *http.Request) {
	var req CollectEvidenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.CaseID == "" || req.Source == "" {
		http.Error(w, "caseId and source are required", http.StatusBadRequest)
		return
	}

	var evidence *Evidence
	var err error

	switch EvidenceType(req.Type) {
	case EvidenceTypeLog:
		// For log evidence, source should be JSON array of log entries
		var entries []map[string]string
		if err := json.Unmarshal([]byte(req.Source), &entries); err != nil {
			http.Error(w, "invalid log entries format", http.StatusBadRequest)
			return
		}
		evidence, err = a.manager.CollectLogEvidence(req.CaseID, entries, req.Description, req.Collector)
	default:
		evidence, err = a.manager.CollectFileEvidence(req.CaseID, req.Source, req.Description, req.Collector)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, evidence)
}

func (a *API) listEvidence(w http.ResponseWriter, r *http.Request) {
	caseID := r.URL.Query().Get("caseId")
	if caseID == "" {
		http.Error(w, "caseId is required", http.StatusBadRequest)
		return
	}

	evidence := a.manager.ListEvidence(caseID)
	writeJSON(w, http.StatusOK, evidence)
}

func (a *API) getEvidence(w http.ResponseWriter, id string) {
	evidence, err := a.manager.GetEvidence(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, evidence)
}

func (a *API) verifyEvidence(w http.ResponseWriter, id string) {
	valid, err := a.manager.VerifyEvidence(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"evidenceId": id,
		"valid":      valid,
		"verifiedAt": time.Now(),
	})
}

type TransferCustodyRequest struct {
	FromOfficer string `json:"fromOfficer"`
	ToOfficer   string `json:"toOfficer"`
	Location    string `json:"location"`
	Notes       string `json:"notes"`
}

func (a *API) transferCustody(w http.ResponseWriter, r *http.Request, evidenceID string) {
	var req TransferCustodyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := a.manager.TransferCustody(evidenceID, req.FromOfficer, req.ToOfficer, req.Location, req.Notes); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "transferred"})
}

// ========== Timeline Operations ==========

type AddEventRequest struct {
	TimelineID  string                 `json:"timelineId"`
	Timestamp   time.Time              `json:"timestamp"`
	Type        string                 `json:"type"`
	Category    string                 `json:"category"`
	Description string                 `json:"description"`
	Source      string                 `json:"source"`
	Severity    string                 `json:"severity"`
	Actor       string                 `json:"actor"`
	Target      string                 `json:"target"`
	Details     map[string]interface{} `json:"details"`
}

func (a *API) addTimelineEvent(w http.ResponseWriter, r *http.Request) {
	var req AddEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.TimelineID == "" {
		http.Error(w, "timelineId is required", http.StatusBadRequest)
		return
	}

	event := Event{
		Timestamp:   req.Timestamp,
		Type:        EventType(req.Type),
		Category:    req.Category,
		Description: req.Description,
		Source:      req.Source,
		Severity:    Priority(req.Severity),
		Actor:       req.Actor,
		Target:      req.Target,
		Details:     req.Details,
	}

	if err := a.manager.AddTimelineEvent(req.TimelineID, event); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, event)
}

func (a *API) getTimeline(w http.ResponseWriter, r *http.Request) {
	caseID := r.URL.Query().Get("caseId")
	timelineID := r.URL.Query().Get("timelineId")

	var timeline *Timeline
	var err error

	if timelineID != "" {
		timeline, err = a.manager.GetTimeline(timelineID)
	} else if caseID != "" {
		timeline, err = a.manager.GetCaseTimeline(caseID)
	} else {
		http.Error(w, "caseId or timelineId is required", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, timeline)
}

// ========== Report Operations ==========

type GenerateReportRequest struct {
	CaseID      string `json:"caseId"`
	Investigator string `json:"investigator"`
	Format      string `json:"format"`
}

func (a *API) generateReport(w http.ResponseWriter, r *http.Request) {
	var req GenerateReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.CaseID == "" {
		http.Error(w, "caseId is required", http.StatusBadRequest)
		return
	}

	if req.Format == "" {
		req.Format = "json"
	}

	report, err := a.manager.GenerateReport(req.CaseID, req.Investigator, req.Format)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, report)
}

// ========== Analysis Operations ==========

type FileSystemAnalysisRequest struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

type SecurityScanRequest struct {
	Path           string `json:"path"`
	IncludeNetwork bool   `json:"includeNetwork"`
}

type SecurityScanResult struct {
	ScanID         string            `json:"scanId"`
	Timestamp      time.Time         `json:"timestamp"`
	Path           string            `json:"path"`
	FilesScanned   int               `json:"filesScanned"`
	IssuesFound    int               `json:"issuesFound"`
	Issues         []SecurityIssue   `json:"issues"`
	NetworkThreats int               `json:"networkThreats,omitempty"`
	Summary        string            `json:"summary"`
}

type SecurityIssue struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

func (a *API) analyzeFileSystem(w http.ResponseWriter, r *http.Request) {
	var req FileSystemAnalysisRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	// If time range specified, find modified files
	if req.StartTime != "" && req.EndTime != "" {
		start, err := time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			http.Error(w, "invalid startTime format", http.StatusBadRequest)
			return
		}
		end, err := time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			http.Error(w, "invalid endTime format", http.StatusBadRequest)
			return
		}

		files, err := a.manager.FindModifiedFiles(req.Path, start, end)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"path":  req.Path,
			"files": files,
			"count": len(files),
		})
		return
	}

	// General directory analysis
	files, err := a.manager.AnalyzeDirectory(req.Path, req.Recursive)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":  req.Path,
		"files": files,
		"count": len(files),
	})
}

type NetworkAnalysisRequest struct {
	Connections []NetworkConnection `json:"connections"`
}

func (a *API) analyzeNetwork(w http.ResponseWriter, r *http.Request) {
	var req NetworkAnalysisRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	suspicious := a.manager.AnalyzeNetworkConnections(req.Connections)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total":      len(req.Connections),
		"suspicious": len(suspicious),
		"connections": suspicious,
	})
}

// ========== Helpers ==========

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// String returns string representation of case status.
func (s CaseStatus) String() string {
	return string(s)
}

// String returns string representation of priority.
func (p Priority) String() string {
	return string(p)
}

// String returns string representation of evidence type.
func (e EvidenceType) String() string {
	return string(e)
}

// String returns string representation of event type.
func (e EventType) String() string {
	return string(e)
}

// MarshalJSON implements custom JSON marshaling for Case.
func (c *Case) MarshalJSON() ([]byte, error) {
	type Alias Case
	return json.Marshal(&struct {
		*Alias
		CreatedAt string `json:"createdAt"`
		UpdatedAt string `json:"updatedAt"`
		ClosedAt  string `json:"closedAt,omitempty"`
	}{
		Alias:     (*Alias)(c),
		CreatedAt: c.CreatedAt.Format(time.RFC3339),
		UpdatedAt: c.UpdatedAt.Format(time.RFC3339),
		ClosedAt:  formatTimePtr(c.ClosedAt),
	})
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// Stats returns forensics statistics.
func (m *Manager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"totalCases":    len(m.cases),
		"totalEvidence": len(m.evidence),
		"totalTimelines": len(m.timelines),
	}

	// Count by status
	statusCounts := make(map[CaseStatus]int)
	for _, c := range m.cases {
		statusCounts[c.Status]++
	}
	stats["casesByStatus"] = statusCounts

	// Count by priority
	priorityCounts := make(map[Priority]int)
	for _, c := range m.cases {
		priorityCounts[c.Priority]++
	}
	stats["casesByPriority"] = priorityCounts

	// Count evidence by type
	evidenceCounts := make(map[EvidenceType]int)
	for _, e := range m.evidence {
		evidenceCounts[e.Type]++
	}
	stats["evidenceByType"] = evidenceCounts

	return stats
}

// GetStats returns forensics statistics via API.
func (a *API) GetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := a.manager.Stats()
	writeJSON(w, http.StatusOK, stats)
}

// DeleteCase deletes a forensic case.
func (m *Manager) DeleteCase(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	case_, exists := m.cases[id]
	if !exists {
		return fmt.Errorf("case %s not found", id)
	}

	// Delete associated timeline
	if case_.TimelineID != "" {
		delete(m.timelines, case_.TimelineID)
		tlPath := filepath.Join(m.config.StoragePath, "timelines", case_.TimelineID+".json")
		os.Remove(tlPath)
	}

	// Delete associated evidence
	for _, eid := range case_.EvidenceIDs {
		delete(m.evidence, eid)
		evPath := filepath.Join(m.config.StoragePath, "evidence", eid+".json")
		os.Remove(evPath)
	}

	// Delete case
	delete(m.cases, id)
	casePath := filepath.Join(m.config.StoragePath, "cases", id+".json")
	os.Remove(casePath)

	m.logger.Info("forensic case deleted", "caseId", id)
	return nil
}
