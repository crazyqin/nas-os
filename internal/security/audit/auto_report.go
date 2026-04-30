// Package audit provides automated security posture reporting for NAS-OS.
// Generates comprehensive security reports including vulnerability scanning,
// configuration auditing, and access anomaly statistics.
//
// Features:
//   - Automated weekly/monthly security reports
//   - Vulnerability scan integration
//   - Configuration compliance checks
//   - Access anomaly detection and statistics
//   - Trend analysis across reporting periods
package audit

import (
	"fmt"
	"sync"
	"time"
)

// ========== Report Types ==========

// ReportType defines the frequency/type of a security report.
type ReportType string

const (
	ReportTypeWeekly  ReportType = "weekly"
	ReportTypeMonthly ReportType = "monthly"
	ReportTypeDaily   ReportType = "daily"
)

// SecurityReport represents a complete security posture report.
type SecurityReport struct {
	ID          string           `json:"id"`
	Type        ReportType       `json:"type"`
	GeneratedAt time.Time        `json:"generatedAt"`
	PeriodStart time.Time        `json:"periodStart"`
	PeriodEnd   time.Time        `json:"periodEnd"`
	Summary     ReportSummary     `json:"summary"`
	VulnSection VulnSection       `json:"vulnerability"`
	ConfigSection ConfigSection    `json:"configAudit"`
	AccessSection AccessSection    `json:"accessAnomaly"`
	Trends      TrendSection     `json:"trends"`
	Recommendations []string     `json:"recommendations"`
}

// ReportSummary provides high-level security posture overview.
type ReportSummary struct {
	OverallScore      int    `json:"overallScore"`      // 0-100
	ScoreDelta        int    `json:"scoreDelta"`        // Change from last report
	TotalEvents       int    `json:"totalEvents"`       // Events in period
	CriticalIssues    int    `json:"criticalIssues"`
	HighIssues        int    `json:"highIssues"`
	MediumIssues      int    `json:"mediumIssues"`
	LowIssues         int    `json:"lowIssues"`
	ResolvedIssues    int    `json:"resolvedIssues"`
	NewIssues         int    `json:"newIssues"`
	Status            string `json:"status"`            // healthy, warning, critical
}

// VulnSection contains vulnerability scan results.
type VulnSection struct {
	TotalScanned   int                  `json:"totalScanned"`
	VulnFound      int                  `json:"vulnerabilitiesFound"`
	Critical       int                  `json:"critical"`
	High           int                  `json:"high"`
	Medium         int                  `json:"medium"`
	Low            int                  `json:"low"`
	Items          []VulnItem           `json:"items"`
	LastScanTime   time.Time            `json:"lastScanTime"`
}

// VulnItem represents a single vulnerability finding.
type VulnItem struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	CVE         string `json:"cve,omitempty"`
	CVSS        float64 `json:"cvss,omitempty"`
	Affected    string `json:"affected"`
	Remediation string `json:"remediation"`
	Status      string `json:"status"` // open, patched, mitigated
}

// ConfigSection contains configuration compliance audit results.
type ConfigSection struct {
	TotalChecks   int              `json:"totalChecks"`
	Passed        int              `json:"passed"`
	Failed        int              `json:"failed"`
	Warning       int              `json:"warning"`
	CompliancePct float64          `json:"compliancePercentage"`
	Items         []ConfigCheckItem `json:"items"`
}

// ConfigCheckItem represents a single configuration check result.
type ConfigCheckItem struct {
	Category    string `json:"category"`
	Check       string `json:"check"`
	Status      string `json:"status"` // pass, fail, warning
	Expected    string `json:"expected"`
	Actual      string `json:"actual"`
	Remediation string `json:"remediation"`
}

// AccessSection contains access anomaly statistics.
type AccessSection struct {
	TotalLogins      int               `json:"totalLogins"`
	SuccessfulLogins int               `json:"successfulLogins"`
	FailedLogins     int               `json:"failedLogins"`
	UniqueIPs        int               `json:"uniqueIPs"`
	Anomalies        []AccessAnomaly   `json:"anomalies"`
	TopIPs           []IPStat          `json:"topIPs"`
	TopUsers         []UserStat        `json:"topUsers"`
	OffHourAccess    int               `json:"offHourAccess"`
	GeoAnomalies     int               `json:"geoAnomalies"`
}

// AccessAnomaly represents a detected access anomaly.
type AccessAnomaly struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"` // brute_force, new_location, off_hours, unusual_volume
	UserID      string    `json:"userId"`
	Username    string    `json:"username"`
	IP          string    `json:"ip"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`
}

// IPStat represents IP address statistics.
type IPStat struct {
	IP       string `json:"ip"`
	Count    int    `json:"count"`
	Country  string `json:"country,omitempty"`
}

// UserStat represents user activity statistics.
type UserStat struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Logins   int    `json:"logins"`
	Failures int    `json:"failures"`
}

// TrendSection contains trend data for comparison.
type TrendSection struct {
	ScoreHistory       []TrendPoint `json:"scoreHistory"`
	VulnHistory        []TrendPoint `json:"vulnHistory"`
	LoginFailureTrend  []TrendPoint `json:"loginFailureTrend"`
}

// TrendPoint is a single data point in a trend line.
type TrendPoint struct {
	Date  string `json:"date"`
	Value int    `json:"value"`
}

// ========== Data Sources ==========

// VulnScanner is an interface for vulnerability scanning data.
type VulnScanner interface {
	Scan() ([]VulnItem, error)
	LastScanTime() time.Time
}

// ConfigAuditor is an interface for configuration compliance checking.
type ConfigAuditor interface {
	AuditChecks() ([]ConfigCheckItem, error)
}

// LoginDataSource is an interface for login event data.
type LoginDataSource interface {
	GetLoginEvents(start, end time.Time) ([]LoginEvent, error)
}

// LoginEvent represents a login event for report generation.
type LoginEvent struct {
	Timestamp time.Time
	UserID    string
	Username  string
	IP        string
	Success   bool
	UserAgent string
	Country   string
}

// AuditLogDataSource is an interface for audit log events.
type AuditLogDataSource interface {
	GetEvents(start, end time.Time, category string) ([]AuditEvent, error)
}

// AuditEvent represents a generic audit event.
type AuditEvent struct {
	Timestamp time.Time
	Category  string
	Event     string
	UserID    string
	Username  string
	IP        string
	Level     string
	Details   map[string]interface{}
}

// ========== Report Generator ==========

// ReportGenerator generates automated security reports.
type ReportGenerator struct {
	mu             sync.RWMutex
	vulnScanner    VulnScanner
	configAuditor  ConfigAuditor
	loginSource    LoginDataSource
	auditSource    AuditLogDataSource
	reports        []*SecurityReport
	maxReports     int
	config         ReportGeneratorConfig
	scoreCalculator ScoreCalculator
}

// ReportGeneratorConfig configuration for report generation.
type ReportGeneratorConfig struct {
	Enabled         bool     `json:"enabled"`
	WeeklyEnabled   bool     `json:"weeklyEnabled"`
	MonthlyEnabled  bool     `json:"monthlyEnabled"`
	ReportDay       string   `json:"reportDay"`       // Day of week for weekly (monday, tuesday, etc.)
	ReportHour      int      `json:"reportHour"`      // Hour to generate (0-23)
	AnomalyThreshold int     `json:"anomalyThreshold"` // Failed logins to trigger anomaly
	OffHourStart    int      `json:"offHourStart"`    // Off-hours start (22 = 10 PM)
	OffHourEnd      int      `json:"offHourEnd"`      // Off-hours end (6 = 6 AM)
}

// DefaultReportGeneratorConfig returns production defaults.
func DefaultReportGeneratorConfig() ReportGeneratorConfig {
	return ReportGeneratorConfig{
		Enabled:         true,
		WeeklyEnabled:   true,
		MonthlyEnabled:  true,
		ReportDay:       "monday",
		ReportHour:      8,
		AnomalyThreshold: 5,
		OffHourStart:    22,
		OffHourEnd:      6,
	}
}

// ScoreCalculator calculates the overall security score.
type ScoreCalculator interface {
	CalculateScore(vulns []VulnItem, configChecks []ConfigCheckItem, accessAnomalies []AccessAnomaly) int
}

// DefaultScoreCalculator implements a weighted scoring algorithm.
type DefaultScoreCalculator struct{}

// CalculateScore calculates a security score (0-100).
func (d *DefaultScoreCalculator) CalculateScore(vulns []VulnItem, configChecks []ConfigCheckItem, accessAnomalies []AccessAnomaly) int {
	score := 100

	// Deduct for vulnerabilities
	for _, v := range vulns {
		switch v.Severity {
		case "critical":
			score -= 15
		case "high":
			score -= 8
		case "medium":
			score -= 3
		case "low":
			score -= 1
		}
	}

	// Deduct for config failures
	for _, c := range configChecks {
		if c.Status == "fail" {
			score -= 5
		} else if c.Status == "warning" {
			score -= 2
		}
	}

	// Deduct for access anomalies
	score -= len(accessAnomalies) * 3

	if score < 0 {
		score = 0
	}
	return score
}

// ========== Constructor ==========

// NewReportGenerator creates a new security report generator.
func NewReportGenerator(cfg ReportGeneratorConfig, opts ...ReportGeneratorOption) *ReportGenerator {
	if cfg.ReportHour < 0 || cfg.ReportHour > 23 {
		cfg.ReportHour = 8
	}
	if cfg.AnomalyThreshold <= 0 {
		cfg.AnomalyThreshold = 5
	}
	if cfg.OffHourStart == 0 {
		cfg.OffHourStart = 22
	}
	if cfg.OffHourEnd == 0 {
		cfg.OffHourEnd = 6
	}

	rg := &ReportGenerator{
		reports:         make([]*SecurityReport, 0),
		maxReports:      100,
		config:          cfg,
		scoreCalculator: &DefaultScoreCalculator{},
	}

	for _, opt := range opts {
		opt(rg)
	}

	return rg
}

// ReportGeneratorOption is a functional option for ReportGenerator.
type ReportGeneratorOption func(*ReportGenerator)

// WithVulnScanner sets the vulnerability scanner data source.
func WithVulnScanner(vs VulnScanner) ReportGeneratorOption {
	return func(rg *ReportGenerator) { rg.vulnScanner = vs }
}

// WithConfigAuditor sets the configuration auditor data source.
func WithConfigAuditor(ca ConfigAuditor) ReportGeneratorOption {
	return func(rg *ReportGenerator) { rg.configAuditor = ca }
}

// WithLoginDataSource sets the login event data source.
func WithLoginDataSource(ls LoginDataSource) ReportGeneratorOption {
	return func(rg *ReportGenerator) { rg.loginSource = ls }
}

// WithAuditLogDataSource sets the audit log data source.
func WithAuditLogDataSource(al AuditLogDataSource) ReportGeneratorOption {
	return func(rg *ReportGenerator) { rg.auditSource = al }
}

// WithScoreCalculator sets a custom score calculator.
func WithScoreCalculator(sc ScoreCalculator) ReportGeneratorOption {
	return func(rg *ReportGenerator) { rg.scoreCalculator = sc }
}

// ========== Report Generation ==========

// GenerateReport generates a security report for the given period.
func (rg *ReportGenerator) GenerateReport(reportType ReportType, periodStart, periodEnd time.Time) (*SecurityReport, error) {
	if !rg.config.Enabled {
		return nil, fmt.Errorf("report generation is disabled")
	}

	report := &SecurityReport{
		ID:          generateReportID(),
		Type:        reportType,
		GeneratedAt: time.Now(),
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
	}

	// 1. Vulnerability Section
	vulns := rg.collectVulnerabilities()
	report.VulnSection = vulns

	// 2. Configuration Audit Section
	configChecks := rg.collectConfigChecks()
	report.ConfigSection = configChecks

	// 3. Access Anomaly Section
	accessSection := rg.collectAccessData(periodStart, periodEnd)
	report.AccessSection = accessSection

	// 4. Calculate overall score
	score := rg.scoreCalculator.CalculateScore(
		vulns.Items,
		configChecks.Items,
		accessSection.Anomalies,
	)

	// 5. Build summary
	report.Summary = rg.buildSummary(score, vulns, configChecks, accessSection)

	// 6. Build trends (include this report)
	report.Trends = rg.buildTrendsIncludingCurrent(report)

	// 7. Generate recommendations
	report.Recommendations = rg.generateRecommendations(report)

	// Store report
	rg.mu.Lock()
	rg.reports = append(rg.reports, report)
	if len(rg.reports) > rg.maxReports {
		rg.reports = rg.reports[len(rg.reports)-rg.maxReports:]
	}
	rg.mu.Unlock()

	return report, nil
}

// GetReport returns a report by ID.
func (rg *ReportGenerator) GetReport(id string) *SecurityReport {
	rg.mu.RLock()
	defer rg.mu.RUnlock()
	for _, r := range rg.reports {
		if r.ID == id {
			return r
		}
	}
	return nil
}

// ListReports returns all stored reports.
func (rg *ReportGenerator) ListReports() []*SecurityReport {
	rg.mu.RLock()
	defer rg.mu.RUnlock()
	result := make([]*SecurityReport, len(rg.reports))
	copy(result, rg.reports)
	return result
}

// GetLatestReport returns the most recent report.
func (rg *ReportGenerator) GetLatestReport() *SecurityReport {
	rg.mu.RLock()
	defer rg.mu.RUnlock()
	if len(rg.reports) == 0 {
		return nil
	}
	return rg.reports[len(rg.reports)-1]
}

// ========== Period Calculation ==========

// CalculateWeeklyPeriod returns the start and end times for a weekly report.
func CalculateWeeklyPeriod(now time.Time) (start, end time.Time) {
	// Previous Monday to previous Sunday
	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	end = now.Truncate(24 * time.Hour)
	start = end.AddDate(0, 0, -int(weekday)-6)
	return start, end
}

// CalculateMonthlyPeriod returns the start and end times for a monthly report.
func CalculateMonthlyPeriod(now time.Time) (start, end time.Time) {
	// First day of previous month to last day of previous month
	firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end = firstOfThisMonth.Add(-time.Second)
	start = time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, now.Location())
	return start, end
}

// ShouldGenerateReport checks if a report should be generated now.
func (rg *ReportGenerator) ShouldGenerateReport(now time.Time) (shouldWeekly, shouldMonthly bool) {
	// Weekly: on configured day at configured hour
	if rg.config.WeeklyEnabled {
		if rg.isConfiguredDay(now) && now.Hour() == rg.config.ReportHour {
			shouldWeekly = true
		}
	}

	// Monthly: on the 1st of each month at configured hour
	if rg.config.MonthlyEnabled {
		if now.Day() == 1 && now.Hour() == rg.config.ReportHour {
			shouldMonthly = true
		}
	}

	return shouldWeekly, shouldMonthly
}

// isConfiguredDay checks if today is the configured report day.
func (rg *ReportGenerator) isConfiguredDay(now time.Time) bool {
	switch rg.config.ReportDay {
	case "monday":
		return now.Weekday() == time.Monday
	case "tuesday":
		return now.Weekday() == time.Tuesday
	case "wednesday":
		return now.Weekday() == time.Wednesday
	case "thursday":
		return now.Weekday() == time.Thursday
	case "friday":
		return now.Weekday() == time.Friday
	case "saturday":
		return now.Weekday() == time.Saturday
	case "sunday":
		return now.Weekday() == time.Sunday
	default:
		return now.Weekday() == time.Monday
	}
}

// ========== Internal Data Collection ==========

func (rg *ReportGenerator) collectVulnerabilities() VulnSection {
	section := VulnSection{
		LastScanTime: time.Now(),
	}

	if rg.vulnScanner == nil {
		return section
	}

	items, err := rg.vulnScanner.Scan()
	if err != nil {
		return section
	}

	section.Items = items
	section.TotalScanned = len(items)
	section.LastScanTime = rg.vulnScanner.LastScanTime()

	for _, item := range items {
		section.VulnFound++
		switch item.Severity {
		case "critical":
			section.Critical++
		case "high":
			section.High++
		case "medium":
			section.Medium++
		case "low":
			section.Low++
		}
	}

	return section
}

func (rg *ReportGenerator) collectConfigChecks() ConfigSection {
	section := ConfigSection{}

	if rg.configAuditor == nil {
		return section
	}

	items, err := rg.configAuditor.AuditChecks()
	if err != nil {
		return section
	}

	section.Items = items
	section.TotalChecks = len(items)

	for _, item := range items {
		switch item.Status {
		case "pass":
			section.Passed++
		case "fail":
			section.Failed++
		case "warning":
			section.Warning++
		}
	}

	if section.TotalChecks > 0 {
		section.CompliancePct = float64(section.Passed) / float64(section.TotalChecks) * 100
	}

	return section
}

func (rg *ReportGenerator) collectAccessData(start, end time.Time) AccessSection {
	section := AccessSection{}

	if rg.loginSource == nil {
		return section
	}

	events, err := rg.loginSource.GetLoginEvents(start, end)
	if err != nil {
		return section
	}

	ipCounts := make(map[string]int)
	ipCountries := make(map[string]string)
	userSuccess := make(map[string]int)
	userFailure := make(map[string]int)
	userNames := make(map[string]string)
	offHourCount := 0

	for _, e := range events {
		section.TotalLogins++
		if e.Success {
			section.SuccessfulLogins++
			userSuccess[e.UserID]++
		} else {
			section.FailedLogins++
			userFailure[e.UserID]++
		}
		ipCounts[e.IP]++
		if e.Country != "" {
			ipCountries[e.IP] = e.Country
		}
		userNames[e.UserID] = e.Username

		// Off-hours detection
		hour := e.Timestamp.Hour()
		if rg.config.OffHourStart > rg.config.OffHourEnd {
			// Wraps midnight (e.g., 22-6)
			if hour >= rg.config.OffHourStart || hour < rg.config.OffHourEnd {
				offHourCount++
			}
		} else {
			if hour >= rg.config.OffHourStart && hour < rg.config.OffHourEnd {
				offHourCount++
			}
		}
	}

	section.UniqueIPs = len(ipCounts)
	section.OffHourAccess = offHourCount

	// Build top IPs
	section.TopIPs = buildTopIPs(ipCounts, ipCountries, 10)

	// Build top users
	section.TopUsers = buildTopUsers(userSuccess, userFailure, userNames, 10)

	// Detect anomalies
	section.Anomalies = rg.detectAnomalies(events)

	return section
}

func buildTopIPs(counts map[string]int, countries map[string]string, limit int) []IPStat {
	type kv struct {
		ip    string
		count int
		co    string
	}
	var sorted []kv
	for ip, count := range counts {
		sorted = append(sorted, kv{ip, count, countries[ip]})
	}
	// Simple insertion sort for small N
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].count > sorted[j-1].count; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}
	result := make([]IPStat, len(sorted))
	for i, s := range sorted {
		result[i] = IPStat{IP: s.ip, Count: s.count, Country: s.co}
	}
	return result
}

func buildTopUsers(success, failure map[string]int, names map[string]string, limit int) []UserStat {
	type kv struct {
		userID string
		s      int
		f      int
	}
	merged := make(map[string]kv)
	for uid, s := range success {
		merged[uid] = kv{uid, s, 0}
	}
	for uid, f := range failure {
		if v, ok := merged[uid]; ok {
			v.f = f
			merged[uid] = v
		} else {
			merged[uid] = kv{uid, 0, f}
		}
	}
	var sorted []kv
	for _, v := range merged {
		sorted = append(sorted, v)
	}
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && (sorted[j].s+sorted[j].f) > (sorted[j-1].s+sorted[j-1].f); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}
	result := make([]UserStat, len(sorted))
	for i, s := range sorted {
		result[i] = UserStat{
			UserID:   s.userID,
			Username: names[s.userID],
			Logins:   s.s,
			Failures: s.f,
		}
	}
	return result
}

// detectAnomalies identifies access anomalies from login events.
func (rg *ReportGenerator) detectAnomalies(events []LoginEvent) []AccessAnomaly {
	var anomalies []AccessAnomaly

	// Group failures by IP in a 5-minute window
	type failWindow struct {
		count   int
		users   map[string]bool
		lastTime time.Time
	}
	ipWindows := make(map[string]*failWindow)

	for _, e := range events {
		if !e.Success {
			w, ok := ipWindows[e.IP]
			if !ok || e.Timestamp.Sub(w.lastTime) > 5*time.Minute {
				// New window
				w = &failWindow{
					users:   map[string]bool{e.Username: true},
					count:   1,
					lastTime: e.Timestamp,
				}
				ipWindows[e.IP] = w
			} else {
				w.count++
				w.users[e.Username] = true
				w.lastTime = e.Timestamp
			}

			if w.count >= rg.config.AnomalyThreshold {
				anomalies = append(anomalies, AccessAnomaly{
					Timestamp:   e.Timestamp,
					Type:        "brute_force",
					UserID:      e.UserID,
					Username:    e.Username,
					IP:          e.IP,
					Description: fmt.Sprintf("brute force detected: %d failed attempts from %s targeting %d user(s)", w.count, e.IP, len(w.users)),
					Severity:    "high",
				})
			}
		}

		// Off-hours access
		hour := e.Timestamp.Hour()
		isOffHours := false
		if rg.config.OffHourStart > rg.config.OffHourEnd {
			isOffHours = hour >= rg.config.OffHourStart || hour < rg.config.OffHourEnd
		} else {
			isOffHours = hour >= rg.config.OffHourStart && hour < rg.config.OffHourEnd
		}

		if isOffHours && e.Success {
			anomalies = append(anomalies, AccessAnomaly{
				Timestamp:   e.Timestamp,
				Type:        "off_hours",
				UserID:      e.UserID,
				Username:    e.Username,
				IP:          e.IP,
				Description: fmt.Sprintf("off-hours login at %02d:%02d from %s", hour, e.Timestamp.Minute(), e.IP),
				Severity:    "low",
			})
		}
	}

	return anomalies
}

func (rg *ReportGenerator) buildSummary(score int, vulns VulnSection, config ConfigSection, access AccessSection) ReportSummary {
	critical := vulns.Critical
	high := vulns.High
	medium := vulns.Medium
	low := vulns.Low

	totalIssues := critical + high + medium + low

	status := "healthy"
	if critical > 0 {
		status = "critical"
	} else if high > 0 || config.Failed > 3 {
		status = "warning"
	}

	// Calculate delta from previous report
	scoreDelta := 0
	rg.mu.RLock()
	if len(rg.reports) > 0 {
		lastReport := rg.reports[len(rg.reports)-1]
		scoreDelta = score - lastReport.Summary.OverallScore
	}
	rg.mu.RUnlock()

	return ReportSummary{
		OverallScore:   score,
		ScoreDelta:     scoreDelta,
		TotalEvents:    access.TotalLogins,
		CriticalIssues: critical,
		HighIssues:     high,
		MediumIssues:   medium,
		LowIssues:      low,
		ResolvedIssues: 0, // Would need historical data to compute
		NewIssues:      totalIssues,
		Status:         status,
	}
}

func (rg *ReportGenerator) buildTrends(reportType ReportType) TrendSection {
	rg.mu.RLock()
	defer rg.mu.RUnlock()

	section := TrendSection{}
	for _, r := range rg.reports {
		dateStr := r.GeneratedAt.Format("2006-01-02")
		section.ScoreHistory = append(section.ScoreHistory, TrendPoint{
			Date:  dateStr,
			Value: r.Summary.OverallScore,
		})
		section.VulnHistory = append(section.VulnHistory, TrendPoint{
			Date:  dateStr,
			Value: r.VulnSection.VulnFound,
		})
		section.LoginFailureTrend = append(section.LoginFailureTrend, TrendPoint{
			Date:  dateStr,
			Value: r.AccessSection.FailedLogins,
		})
	}
	return section
}

// buildTrendsIncludingCurrent builds trends including the current report.
func (rg *ReportGenerator) buildTrendsIncludingCurrent(current *SecurityReport) TrendSection {
	rg.mu.RLock()
	defer rg.mu.RUnlock()

	section := TrendSection{}
	all := make([]*SecurityReport, len(rg.reports))
	copy(all, rg.reports)
	all = append(all, current)

	for _, r := range all {
		dateStr := r.GeneratedAt.Format("2006-01-02")
		section.ScoreHistory = append(section.ScoreHistory, TrendPoint{
			Date:  dateStr,
			Value: r.Summary.OverallScore,
		})
		section.VulnHistory = append(section.VulnHistory, TrendPoint{
			Date:  dateStr,
			Value: r.VulnSection.VulnFound,
		})
		section.LoginFailureTrend = append(section.LoginFailureTrend, TrendPoint{
			Date:  dateStr,
			Value: r.AccessSection.FailedLogins,
		})
	}
	return section
}

func (rg *ReportGenerator) generateRecommendations(report *SecurityReport) []string {
	var recs []string

	// Vulnerability recommendations
	if report.VulnSection.Critical > 0 {
		recs = append(recs, fmt.Sprintf("URGENT: %d critical vulnerabilities found. Patch immediately.", report.VulnSection.Critical))
	}
	if report.VulnSection.High > 0 {
		recs = append(recs, fmt.Sprintf("HIGH: %d high-severity vulnerabilities require attention within 7 days.", report.VulnSection.High))
	}

	// Config recommendations
	if report.ConfigSection.Failed > 0 {
		recs = append(recs, fmt.Sprintf("CONFIG: %d configuration checks failed. Review and remediate.", report.ConfigSection.Failed))
	}
	if report.ConfigSection.CompliancePct < 80 {
		recs = append(recs, fmt.Sprintf("Compliance at %.1f%% - target is 80%%+. Review failed checks.", report.ConfigSection.CompliancePct))
	}

	// Access recommendations
	if report.AccessSection.FailedLogins > 100 {
		recs = append(recs, fmt.Sprintf("HIGH: %d failed logins in period. Consider enabling IP-based rate limiting.", report.AccessSection.FailedLogins))
	}
	if len(report.AccessSection.Anomalies) > 5 {
		recs = append(recs, fmt.Sprintf("ALERT: %d access anomalies detected. Review for potential security incidents.", len(report.AccessSection.Anomalies)))
	}
	if report.AccessSection.OffHourAccess > 10 {
		recs = append(recs, fmt.Sprintf("INFO: %d off-hours access events. Verify these are legitimate.", report.AccessSection.OffHourAccess))
	}

	if len(recs) == 0 {
		recs = append(recs, "No critical issues found. Continue monitoring.")
	}

	return recs
}

// ========== Helper ==========

func generateReportID() string {
	b, _ := generateRandomBytes(8)
	return fmt.Sprintf("RPT-%s-%s", time.Now().Format("20060102"), fmt.Sprintf("%x", b))
}

func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(time.Now().UnixNano() >> (8 * (i % 8)))
	}
	return b, nil
}

// Stats returns report generator statistics.
func (rg *ReportGenerator) Stats() map[string]interface{} {
	rg.mu.RLock()
	defer rg.mu.RUnlock()
	return map[string]interface{}{
		"totalReports": len(rg.reports),
		"enabled":      rg.config.Enabled,
		"weeklyEnabled": rg.config.WeeklyEnabled,
		"monthlyEnabled": rg.config.MonthlyEnabled,
		"reportDay":    rg.config.ReportDay,
		"reportHour":   rg.config.ReportHour,
	}
}

// GetConfig returns the generator configuration.
func (rg *ReportGenerator) GetConfig() ReportGeneratorConfig {
	rg.mu.RLock()
	defer rg.mu.RUnlock()
	return rg.config
}

// UpdateConfig updates the generator configuration.
func (rg *ReportGenerator) UpdateConfig(cfg ReportGeneratorConfig) {
	rg.mu.Lock()
	defer rg.mu.Unlock()
	rg.config = cfg
}
