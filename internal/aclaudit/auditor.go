package aclaudit

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Permission represents a permission type
type Permission string

const (
	PermRead      Permission = "read"
	PermWrite     Permission = "write"
	PermExecute   Permission = "execute"
	PermDelete    Permission = "delete"
	PermCreate    Permission = "create"
	PermModify    Permission = "modify"
	PermShare     Permission = "share"
	PermAdmin     Permission = "admin"
	PermAudit     Permission = "audit"
	PermBackup    Permission = "backup"
	PermRestore   Permission = "restore"
	PermQuota     Permission = "quota"
	PermSecurity  Permission = "security"
)

// AccessDecision represents the outcome of an access check
type AccessDecision string

const (
	DecisionAllow AccessDecision = "allow"
	DecisionDeny  AccessDecision = "deny"
	DecisionError AccessDecision = "error"
)

// AuditEntry represents a single audit log entry
type AuditEntry struct {
	ID          string         `json:"id"`
	Timestamp   time.Time      `json:"timestamp"`
	UserID      string         `json:"user_id"`
	UserName    string         `json:"user_name"`
	ResourceID  string         `json:"resource_id"`
	ResourceType string        `json:"resource_type"`
	ResourcePath string        `json:"resource_path"`
	Permission  Permission     `json:"permission"`
	Decision    AccessDecision `json:"decision"`
	Reason      string         `json:"reason"`
	SourceIP    string         `json:"source_ip"`
	UserAgent   string         `json:"user_agent"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// AccessPattern represents detected access patterns
type AccessPattern struct {
	UserID       string    `json:"user_id"`
	ResourceType string    `json:"resource_type"`
	Permission   Permission `json:"permission"`
	Frequency    int       `json:"frequency"`
	LastAccess   time.Time `json:"last_access"`
	AnomalyScore float64   `json:"anomaly_score"` // 0-1, higher = more anomalous
}

// Anomaly represents a detected security anomaly
type Anomaly struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`
	Severity    string    `json:"severity"` // low, medium, high, critical
	UserID      string    `json:"user_id"`
	Description string    `json:"description"`
	Entries     []string  `json:"entry_ids"`
}

// Auditor provides ACL audit functionality
type Auditor struct {
	mu       sync.RWMutex
	entries  []AuditEntry
	patterns map[string]*AccessPattern
	anomalies []Anomaly
	config   AuditorConfig
}

// AuditorConfig contains auditor configuration
type AuditorConfig struct {
	MaxEntries       int    `json:"max_entries"`
	RetentionDays    int    `json:"retention_days"`
	AnomalyThreshold float64 `json:"anomaly_threshold"` // 0-1
}

// DefaultAuditorConfig returns default configuration
func DefaultAuditorConfig() AuditorConfig {
	return AuditorConfig{
		MaxEntries:       100000,
		RetentionDays:    90,
		AnomalyThreshold: 0.7,
	}
}

// NewAuditor creates a new ACL auditor
func NewAuditor(config AuditorConfig) *Auditor {
	return &Auditor{
		patterns: make(map[string]*AccessPattern),
		config:   config,
	}
}

// LogAccess logs an access attempt
func (a *Auditor) LogAccess(entry AuditEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("%d-%s", time.Now().UnixNano(), entry.UserID)
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	
	a.entries = append(a.entries, entry)
	
	// Update patterns
	a.updatePattern(entry)
	
	// Check for anomalies
	a.checkAnomaly(entry)
	
	// Cleanup old entries
	a.cleanup()
}

// updatePattern updates access pattern tracking
func (a *Auditor) updatePattern(entry AuditEntry) {
	key := fmt.Sprintf("%s:%s:%s", entry.UserID, entry.ResourceType, entry.Permission)
	
	pattern, exists := a.patterns[key]
	if !exists {
		pattern = &AccessPattern{
			UserID:       entry.UserID,
			ResourceType: entry.ResourceType,
			Permission:   entry.Permission,
		}
		a.patterns[key] = pattern
	}
	
	pattern.Frequency++
	pattern.LastAccess = entry.Timestamp
}

// checkAnomaly checks for anomalous access patterns
func (a *Auditor) checkAnomaly(entry AuditEntry) {
	// Check for off-hours access
	hour := entry.Timestamp.Hour()
	if hour < 6 || hour > 22 {
		a.addAnomaly(Anomaly{
			Type:        "off_hours_access",
			Severity:    "medium",
			UserID:      entry.UserID,
			Description: fmt.Sprintf("非工作时间访问: %s", entry.Timestamp.Format("2006-01-02 15:04")),
		})
	}
	
	// Check for rapid access
	key := fmt.Sprintf("%s:%s:%s", entry.UserID, entry.ResourceType, entry.Permission)
	if pattern, exists := a.patterns[key]; exists {
		if pattern.Frequency > 100 { // threshold
			a.addAnomaly(Anomaly{
				Type:        "high_frequency_access",
				Severity:    "high",
				UserID:      entry.UserID,
				Description: fmt.Sprintf("高频访问检测: %d次", pattern.Frequency),
			})
		}
	}
	
	// Check for denied access attempts
	if entry.Decision == DecisionDeny {
		a.addAnomaly(Anomaly{
			Type:        "access_denied",
			Severity:    "low",
			UserID:      entry.UserID,
			Description: fmt.Sprintf("访问被拒绝: %s on %s", entry.Permission, entry.ResourcePath),
		})
	}
}

// addAnomaly adds an anomaly to the list
func (a *Auditor) addAnomaly(anomaly Anomaly) {
	anomaly.ID = fmt.Sprintf("anomaly-%d", time.Now().UnixNano())
	anomaly.Timestamp = time.Now()
	a.anomalies = append(a.anomalies, anomaly)
}

// cleanup removes old entries based on retention policy
func (a *Auditor) cleanup() {
	cutoff := time.Now().AddDate(0, 0, -a.config.RetentionDays)
	
	var filtered []AuditEntry
	for _, entry := range a.entries {
		if entry.Timestamp.After(cutoff) {
			filtered = append(filtered, entry)
		}
	}
	a.entries = filtered
	
	// Limit total entries
	if len(a.entries) > a.config.MaxEntries {
		a.entries = a.entries[len(a.entries)-a.config.MaxEntries:]
	}
}

// QueryEntries queries audit entries with filters
func (a *Auditor) QueryEntries(filter AuditFilter) []AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	var results []AuditEntry
	
	for _, entry := range a.entries {
		if a.matchesFilter(entry, filter) {
			results = append(results, entry)
		}
	}
	
	// Apply limit
	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}
	
	return results
}

// AuditFilter represents query filters for audit entries
type AuditFilter struct {
	UserID       string         `json:"user_id,omitempty"`
	ResourceType string         `json:"resource_type,omitempty"`
	Permission   Permission     `json:"permission,omitempty"`
	Decision     AccessDecision `json:"decision,omitempty"`
	StartTime    *time.Time     `json:"start_time,omitempty"`
	EndTime      *time.Time     `json:"end_time,omitempty"`
	Limit        int            `json:"limit,omitempty"`
}

// matchesFilter checks if an entry matches the filter
func (a *Auditor) matchesFilter(entry AuditEntry, filter AuditFilter) bool {
	if filter.UserID != "" && entry.UserID != filter.UserID {
		return false
	}
	if filter.ResourceType != "" && entry.ResourceType != filter.ResourceType {
		return false
	}
	if filter.Permission != "" && entry.Permission != filter.Permission {
		return false
	}
	if filter.Decision != "" && entry.Decision != filter.Decision {
		return false
	}
	if filter.StartTime != nil && entry.Timestamp.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && entry.Timestamp.After(*filter.EndTime) {
		return false
	}
	return true
}

// GetAnomalies returns detected anomalies
func (a *Auditor) GetAnomalies(since time.Time) []Anomaly {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	var results []Anomaly
	for _, anomaly := range a.anomalies {
		if anomaly.Timestamp.After(since) {
			results = append(results, anomaly)
		}
	}
	return results
}

// GetPatterns returns access patterns
func (a *Auditor) GetPatterns() []AccessPattern {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	patterns := make([]AccessPattern, 0, len(a.patterns))
	for _, p := range a.patterns {
		patterns = append(patterns, *p)
	}
	return patterns
}

// GenerateReport generates an audit report
func (a *Auditor) GenerateReport(ctx context.Context, period time.Duration) AuditReport {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	start := time.Now().Add(-period)
	
	report := AuditReport{
		Period:    period.String(),
		StartTime: start,
		EndTime:   time.Now(),
	}
	
	// Count by decision
	decisionCounts := make(map[AccessDecision]int)
	permCounts := make(map[Permission]int)
	userActivity := make(map[string]int)
	
	for _, entry := range a.entries {
		if entry.Timestamp.Before(start) {
			continue
		}
		
		report.TotalEntries++
		decisionCounts[entry.Decision]++
		permCounts[entry.Permission]++
		userActivity[entry.UserID]++
	}
	
	report.AllowCount = decisionCounts[DecisionAllow]
	report.DenyCount = decisionCounts[DecisionDeny]
	report.ErrorCount = decisionCounts[DecisionError]
	
	// Find most active users
	var activeUsers []UserActivity
	for userID, count := range userActivity {
		activeUsers = append(activeUsers, UserActivity{
			UserID:  userID,
			Count:   count,
		})
	}
	report.ActiveUsers = activeUsers
	
	// Count anomalies
	var anomalyCount int
	for _, anomaly := range a.anomalies {
		if anomaly.Timestamp.After(start) {
			anomalyCount++
		}
	}
	report.AnomalyCount = anomalyCount
	
	return report
}

// AuditReport contains audit summary
type AuditReport struct {
	Period       string         `json:"period"`
	StartTime    time.Time      `json:"start_time"`
	EndTime      time.Time      `json:"end_time"`
	TotalEntries int            `json:"total_entries"`
	AllowCount   int            `json:"allow_count"`
	DenyCount    int            `json:"deny_count"`
	ErrorCount   int            `json:"error_count"`
	AnomalyCount int            `json:"anomaly_count"`
	ActiveUsers  []UserActivity `json:"active_users"`
}

// UserActivity represents user activity summary
type UserActivity struct {
	UserID string `json:"user_id"`
	Count  int    `json:"count"`
}

// Export exports audit entries as JSON
func (a *Auditor) Export(ctx context.Context, filter AuditFilter) ([]byte, error) {
	entries := a.QueryEntries(filter)
	return json.MarshalIndent(entries, "", "  ")
}
