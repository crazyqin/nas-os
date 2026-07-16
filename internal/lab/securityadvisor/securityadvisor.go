package securityadvisor

import (
	"sort"
	"strings"
	"time"
)

// Severity describes the operational risk level of a finding.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// CheckInput is a compact, dependency-free snapshot for security posture checks.
type CheckInput struct {
	SSHPasswordLogin       bool      `json:"sshPasswordLogin"`
	MFAEnabled             bool      `json:"mfaEnabled"`
	FirewallEnabled        bool      `json:"firewallEnabled"`
	AuditLogEnabled        bool      `json:"auditLogEnabled"`
	BackupAgeHours         int       `json:"backupAgeHours"`
	PublicShareCount       int       `json:"publicShareCount"`
	AdminWithoutMFA        int       `json:"adminWithoutMfa"`
	PendingSecurityPatches int       `json:"pendingSecurityPatches"`
	LastScan               time.Time `json:"lastScan,omitempty"`
}

// Finding is one actionable recommendation.
type Finding struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Severity       Severity `json:"severity"`
	ScorePenalty   int      `json:"scorePenalty"`
	Recommendation string   `json:"recommendation"`
}

// Report summarizes NAS security posture similar to Synology Security Advisor.
type Report struct {
	Score     int       `json:"score"`
	Grade     string    `json:"grade"`
	Findings  []Finding `json:"findings"`
	ScannedAt time.Time `json:"scannedAt"`
}

// Analyze evaluates common NAS hardening controls and returns a deterministic report.
func Analyze(in CheckInput) Report {
	findings := make([]Finding, 0, 8)
	add := func(id, title string, sev Severity, penalty int, rec string) {
		findings = append(findings, Finding{ID: id, Title: title, Severity: sev, ScorePenalty: penalty, Recommendation: rec})
	}

	if in.SSHPasswordLogin {
		add("ssh-password-login", "SSH password login is enabled", SeverityCritical, 18, "Disable SSH password authentication and require keys or FIDO2-backed access.")
	}
	if !in.MFAEnabled {
		add("mfa-disabled", "Multi-factor authentication is disabled", SeverityCritical, 18, "Enable MFA for all privileged users.")
	}
	if in.AdminWithoutMFA > 0 {
		add("admin-without-mfa", "Privileged accounts without MFA detected", SeverityCritical, min(20, 8+in.AdminWithoutMFA*3), "Require MFA enrollment before granting administrator privileges.")
	}
	if !in.FirewallEnabled {
		add("firewall-disabled", "Host firewall is disabled", SeverityWarning, 12, "Enable the NAS firewall and allow only required management and sharing ports.")
	}
	if !in.AuditLogEnabled {
		add("audit-log-disabled", "Audit logging is disabled", SeverityWarning, 10, "Enable immutable audit logging for auth, share, and admin events.")
	}
	if in.BackupAgeHours > 72 {
		add("stale-backup", "Latest backup is stale", SeverityWarning, min(18, 6+in.BackupAgeHours/24), "Run a fresh backup and verify restore integrity.")
	}
	if in.PublicShareCount > 0 {
		add("public-shares", "Public shares are exposed", SeverityWarning, min(15, 5+in.PublicShareCount*2), "Review public links, add expiry times, and require passwords for sensitive shares.")
	}
	if in.PendingSecurityPatches > 0 {
		sev := SeverityWarning
		if in.PendingSecurityPatches >= 5 {
			sev = SeverityCritical
		}
		add("pending-security-patches", "Security patches are pending", sev, min(20, 5+in.PendingSecurityPatches*2), "Apply security updates during the next maintenance window.")
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return severityRank(findings[i].Severity) > severityRank(findings[j].Severity)
		}
		return findings[i].ID < findings[j].ID
	})

	score := 100
	for _, f := range findings {
		score -= f.ScorePenalty
	}
	if score < 0 {
		score = 0
	}
	return Report{Score: score, Grade: grade(score), Findings: findings, ScannedAt: time.Now().UTC()}
}

// Summary returns a concise human-readable one-line report.
func (r Report) Summary() string {
	parts := make([]string, 0, len(r.Findings))
	for _, f := range r.Findings {
		parts = append(parts, f.ID)
	}
	if len(parts) == 0 {
		return "Security score 100 (A): no findings"
	}
	return "Security score " + itoa(r.Score) + " (" + r.Grade + "): " + strings.Join(parts, ", ")
}

func severityRank(s Severity) int {
	switch s {
	case SeverityCritical:
		return 3
	case SeverityWarning:
		return 2
	default:
		return 1
	}
}

func grade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [3]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
