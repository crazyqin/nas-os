package securityscore

import (
	"fmt"
	"sync"
	"time"
)

// RiskLevel represents vulnerability risk level.
type RiskLevel string

const (
	RiskCritical RiskLevel = "critical"
	RiskHigh     RiskLevel = "high"
	RiskMedium   RiskLevel = "medium"
	RiskLow      RiskLevel = "low"
)

// KEVEntry represents a CISA Known Exploited Vulnerability.
type KEVEntry struct {
	CVEID         string    `json:"cve_id"`
	VendorProject string    `json:"vendor_project"`
	Product       string    `json:"product"`
	Vulnerability string    `json:"vulnerability"`
	DateAdded     time.Time `json:"date_added"`
	DueDate       time.Time `json:"due_date"`
	Action        string    `json:"action"`
	RansomwareUse string    `json:"ransomware_use"` // "known" or ""
}

// EPSSScore represents Exploit Prediction Scoring System data.
type EPSSScore struct {
	CVEID       string    `json:"cve_id"`
	Score       float64   `json:"score"`       // 0.0 - 1.0 probability
	Percentile  float64   `json:"percentile"`  // 0.0 - 100.0
	ModelVersion string   `json:"model_version"`
	CreatedAt   time.Time `json:"created_at"`
}

// LEVEntry represents a Local Exploit Vulnerability assessment.
type LEVEntry struct {
	CVEID        string    `json:"cve_id"`
	Component    string    `json:"component"`
	Version      string    `json:"version"`
	Severity     RiskLevel `json:"severity"`
	CVSSScore    float64   `json:"cvss_score"`
	EPSSScore    float64   `json:"epss_score"`
	KEVListed    bool      `json:"kev_listed"`
	Remediation  string    `json:"remediation"`
	FixAvailable bool      `json:"fix_available"`
}

// RiskAssessment represents a combined risk assessment.
type RiskAssessment struct {
	ID            string      `json:"id"`
	Timestamp     time.Time   `json:"timestamp"`
	TotalCVEs     int         `json:"total_cves"`
	KEVCount      int         `json:"kev_count"`
	HighEPSSCount int         `json:"high_epss_count"` // EPSS > 0.7
	OverallRisk   RiskLevel   `json:"overall_risk"`
	RiskScore     float64     `json:"risk_score"` // 0-100
	Vulnerabilities []LEVEntry `json:"vulnerabilities"`
	TopRisks      []LEVEntry  `json:"top_risks"`
	Remediations  []string    `json:"remediations"`
}

// RiskAssessor provides KEV/EPSS/LEV risk assessment.
type RiskAssessor struct {
	mu           sync.RWMutex
	kevList      map[string]*KEVEntry
	epssScores   map[string]*EPSSScore
	assessments  []*RiskAssessment
}

// NewRiskAssessor creates a new risk assessor.
func NewRiskAssessor() *RiskAssessor {
	return &RiskAssessor{
		kevList:     make(map[string]*KEVEntry),
		epssScores:  make(map[string]*EPSSScore),
		assessments: make([]*RiskAssessment, 0),
	}
}

// UpdateKEVList updates the KEV vulnerability list.
func (ra *RiskAssessor) UpdateKEVList(entries []KEVEntry) {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	for i := range entries {
		ra.kevList[entries[i].CVEID] = &entries[i]
	}
}

// UpdateEPSSScores updates EPSS scores.
func (ra *RiskAssessor) UpdateEPSSScores(scores []EPSSScore) {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	for i := range scores {
		ra.epssScores[scores[i].CVEID] = &scores[i]
	}
}

// AssessRisk performs a combined KEV/EPSS/LEV risk assessment.
func (ra *RiskAssessor) AssessRisk(vulnerabilities []LEVEntry) *RiskAssessment {
	ra.mu.RLock()
	defer ra.mu.RUnlock()

	assessment := &RiskAssessment{
		ID:              fmt.Sprintf("risk-%d", time.Now().UnixNano()),
		Timestamp:       time.Now(),
		TotalCVEs:       len(vulnerabilities),
		Vulnerabilities: vulnerabilities,
	}

	riskScore := 0.0
	var topRisks []LEVEntry

	for i, vuln := range vulnerabilities {
		// Check KEV
		if kev, exists := ra.kevList[vuln.CVEID]; exists {
			vulnerabilities[i].KEVListed = true
			assessment.KEVCount++
			riskScore += 30.0 // KEV is highest priority
			if kev.RansomwareUse == "known" {
				riskScore += 10.0
			}
		}

		// Check EPSS
		if epss, exists := ra.epssScores[vuln.CVEID]; exists {
			vulnerabilities[i].EPSSScore = epss.Score
			if epss.Score > 0.7 {
				assessment.HighEPSSCount++
				riskScore += 20.0
			} else if epss.Score > 0.3 {
				riskScore += 10.0
			}
		}

		// CVSS contribution
		riskScore += vuln.CVSSScore * 2.0

		// Collect high-risk vulnerabilities
		if vuln.KEVListed || vuln.EPSSScore > 0.7 || vuln.CVSSScore >= 9.0 {
			topRisks = append(topRisks, vulnerabilities[i])
		}
	}

	assessment.TopRisks = topRisks

	// Normalize risk score to 0-100
	if len(vulnerabilities) > 0 {
		assessment.RiskScore = riskScore / float64(len(vulnerabilities))
		if assessment.RiskScore > 100 {
			assessment.RiskScore = 100
		}
	}

	// Determine overall risk level
	switch {
	case assessment.RiskScore >= 80 || assessment.KEVCount > 5:
		assessment.OverallRisk = RiskCritical
	case assessment.RiskScore >= 60 || assessment.KEVCount > 2:
		assessment.OverallRisk = RiskHigh
	case assessment.RiskScore >= 40 || assessment.KEVCount > 0:
		assessment.OverallRisk = RiskMedium
	default:
		assessment.OverallRisk = RiskLow
	}

	// Generate remediation recommendations
	assessment.Remediations = ra.generateRemediations(assessment)

	ra.assessments = append(ra.assessments, assessment)
	return assessment
}

// generateRemediations generates prioritized remediation recommendations.
func (ra *RiskAssessor) generateRemediations(assessment *RiskAssessment) []string {
	var remediations []string

	if assessment.KEVCount > 0 {
		remediations = append(remediations,
			fmt.Sprintf("紧急修复 %d 个已知被利用漏洞 (KEV)", assessment.KEVCount))
	}

	if assessment.HighEPSSCount > 0 {
		remediations = append(remediations,
			fmt.Sprintf("优先修复 %d 个高概率利用漏洞 (EPSS > 70%%)", assessment.HighEPSSCount))
	}

	for _, vuln := range assessment.TopRisks {
		if vuln.FixAvailable && vuln.Remediation != "" {
			remediations = append(remediations,
				fmt.Sprintf("[%s] %s: %s", vuln.Severity, vuln.CVEID, vuln.Remediation))
		}
	}

	return remediations
}

// GetAssessmentHistory returns recent assessments.
func (ra *RiskAssessor) GetAssessmentHistory(limit int) []*RiskAssessment {
	ra.mu.RLock()
	defer ra.mu.RUnlock()

	if limit <= 0 || limit > len(ra.assessments) {
		limit = len(ra.assessments)
	}
	start := len(ra.assessments) - limit
	result := make([]*RiskAssessment, limit)
	copy(result, ra.assessments[start:])
	return result
}

// IsKEVListed checks if a CVE is in the KEV list.
func (ra *RiskAssessor) IsKEVListed(cveID string) bool {
	ra.mu.RLock()
	defer ra.mu.RUnlock()
	_, exists := ra.kevList[cveID]
	return exists
}

// GetEPSSScore returns the EPSS score for a CVE.
func (ra *RiskAssessor) GetEPSSScore(cveID string) (*EPSSScore, bool) {
	ra.mu.RLock()
	defer ra.mu.RUnlock()
	score, exists := ra.epssScores[cveID]
	return score, exists
}

// KEVCount returns the number of entries in the KEV list.
func (ra *RiskAssessor) KEVCount() int {
	ra.mu.RLock()
	defer ra.mu.RUnlock()
	return len(ra.kevList)
}
