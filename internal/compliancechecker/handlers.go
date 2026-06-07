package compliancechecker

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// ScanRequest represents the request body for starting a scan
type ScanRequest struct {
	Paths              []string `json:"paths"`
	ScanGDPR           *bool    `json:"scan_gdpr,omitempty"`
	ScanGB20           *bool    `json:"scan_gb20,omitempty"`
	ScanFilePermission *bool    `json:"scan_file_permission,omitempty"`
	ScanSensitiveData  *bool    `json:"scan_sensitive_data,omitempty"`
	ScanEncryption     *bool    `json:"scan_encryption,omitempty"`
}

// FixRequest represents the request body for fix suggestions
type FixRequest struct {
	IssueID string `json:"issue_id"`
}

// APIResponse is the standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Handler handles compliance check HTTP requests
type Handler struct {
	mu           sync.RWMutex
	lastReport   *ComplianceReport
	lastScanTime time.Time
}

// NewHandler creates a new compliance handler
func NewHandler() *Handler {
	return &Handler{}
}

// RegisterRoutes registers compliance API routes on the provided mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/compliance/scan", h.handleScan)
	mux.HandleFunc("/api/v1/compliance/report", h.handleReport)
	mux.HandleFunc("/api/v1/compliance/score", h.handleScore)
	mux.HandleFunc("/api/v1/compliance/issues", h.handleIssues)
	mux.HandleFunc("/api/v1/compliance/fix", h.handleFix)
}

// handleScan starts a new compliance scan
// POST /api/v1/compliance/scan
func (h *Handler) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	if len(req.Paths) == 0 {
		writeError(w, http.StatusBadRequest, "paths is required")
		return
	}

	// Build config
	config := ScanConfig{
		Paths:              req.Paths,
		ScanGDPR:           true,
		ScanGB20:           true,
		ScanFilePermission: true,
		ScanSensitiveData:  true,
		ScanEncryption:     true,
		MaxFileSize:        10 * 1024 * 1024,
		IgnorePatterns:     []string{".git", "node_modules", ".DS_Store"},
	}

	// Override with request settings
	if req.ScanGDPR != nil {
		config.ScanGDPR = *req.ScanGDPR
	}
	if req.ScanGB20 != nil {
		config.ScanGB20 = *req.ScanGB20
	}
	if req.ScanFilePermission != nil {
		config.ScanFilePermission = *req.ScanFilePermission
	}
	if req.ScanSensitiveData != nil {
		config.ScanSensitiveData = *req.ScanSensitiveData
	}
	if req.ScanEncryption != nil {
		config.ScanEncryption = *req.ScanEncryption
	}

	checker := NewChecker(config)
	report, err := checker.RunScan()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Scan failed: "+err.Error())
		return
	}

	// Store report
	h.mu.Lock()
	h.lastReport = report
	h.lastScanTime = time.Now()
	h.mu.Unlock()

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    report,
	})
}

// handleReport returns the last scan report
// GET /api/v1/compliance/report
func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	h.mu.RLock()
	report := h.lastReport
	h.mu.RUnlock()

	if report == nil {
		writeError(w, http.StatusNotFound, "No scan report available. Run a scan first.")
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    report,
	})
}

// handleScore returns compliance score
// GET /api/v1/compliance/score
func (h *Handler) handleScore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	h.mu.RLock()
	report := h.lastReport
	h.mu.RUnlock()

	if report == nil {
		writeError(w, http.StatusNotFound, "No scan report available. Run a scan first.")
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    report.Summary,
	})
}

// handleIssues returns list of issues
// GET /api/v1/compliance/issues
func (h *Handler) handleIssues(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	h.mu.RLock()
	report := h.lastReport
	h.mu.RUnlock()

	if report == nil {
		writeError(w, http.StatusNotFound, "No scan report available. Run a scan first.")
		return
	}

	// Support filtering by type
	issueType := r.URL.Query().Get("type")
	severity := r.URL.Query().Get("severity")

	issues := report.Issues
	if issueType != "" || severity != "" {
		filtered := make([]Issue, 0)
		for _, issue := range issues {
			if issueType != "" && string(issue.Type) != issueType {
				continue
			}
			if severity != "" && string(issue.Severity) != severity {
				continue
			}
			filtered = append(filtered, issue)
		}
		issues = filtered
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    issues,
	})
}

// handleFix returns fix suggestions for an issue
// POST /api/v1/compliance/fix
func (h *Handler) handleFix(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	h.mu.RLock()
	report := h.lastReport
	h.mu.RUnlock()

	if report == nil {
		writeError(w, http.StatusNotFound, "No scan report available. Run a scan first.")
		return
	}

	var req FixRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	if req.IssueID == "" {
		writeError(w, http.StatusBadRequest, "issue_id is required")
		return
	}

	// Find the issue
	var found *Issue
	for i, issue := range report.Issues {
		if issue.ID == req.IssueID {
			found = &report.Issues[i]
			break
		}
	}

	if found == nil {
		writeError(w, http.StatusNotFound, "Issue not found: "+req.IssueID)
		return
	}

	suggestion := FixSuggestion(*found)
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"issue_id": found.ID,
			"title":    found.Title,
			"fix":      suggestion,
			"path":     found.Path,
			"severity": found.Severity,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, APIResponse{
		Success: false,
		Error:   message,
	})
}
