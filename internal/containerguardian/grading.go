package containerguardian

// CalculateScore computes the overall security score and grade for a scan result.
// Score is 0-100, with 100 being perfect security.
func (g *Guardian) CalculateScore(result *ScanResult) *SecurityScore {
	score := &SecurityScore{
		Overall: 100.0,
		Breakdown: ScoreBreakdown{},
	}

	// Deductions from vulnerabilities
	vulnScore := 100.0
	for _, v := range result.Vulnerabilities {
		switch v.Severity {
		case SeverityCritical:
			vulnScore -= 20.0
		case SeverityHigh:
			vulnScore -= 12.0
		case SeverityMedium:
			vulnScore -= 6.0
		case SeverityLow:
			vulnScore -= 2.0
		case SeverityInfo:
			vulnScore -= 0.5
		}
	}
	if vulnScore < 0 {
		vulnScore = 0
	}
	score.VulnScore = vulnScore
	score.Breakdown.VulnDeductions = 100.0 - vulnScore

	// Deductions from compliance failures
	compScore := 100.0
	for _, c := range result.Compliance {
		switch c.Status {
		case ComplianceFail:
			switch c.Severity {
			case SeverityCritical:
				compScore -= 15.0
			case SeverityHigh:
				compScore -= 10.0
			case SeverityMedium:
				compScore -= 5.0
			default:
				compScore -= 2.0
			}
		case ComplianceWarn:
			compScore -= 3.0
		}
	}
	if compScore < 0 {
		compScore = 0
	}
	score.CompScore = compScore
	score.Breakdown.ComplianceDeductions = 100.0 - compScore

	// Deductions from runtime anomalies
	runtimeScore := 100.0
	if result.Runtime != nil {
		for _, a := range result.Runtime.Anomalies {
			switch a.Severity {
			case SeverityCritical:
				runtimeScore -= 20.0
			case SeverityHigh:
				runtimeScore -= 12.0
			case SeverityMedium:
				runtimeScore -= 6.0
			case SeverityLow:
				runtimeScore -= 2.0
			}
		}
	}
	if runtimeScore < 0 {
		runtimeScore = 0
	}
	score.RuntimeScore = runtimeScore
	score.Breakdown.RuntimeDeductions = 100.0 - runtimeScore

	// Deductions from sensitive findings
	sensitiveScore := 100.0
	for _, s := range result.Sensitive {
		switch s.Sensitivity {
		case SensitivityCritical:
			sensitiveScore -= 15.0
		case SensitivityHigh:
			sensitiveScore -= 10.0
		case SensitivityMedium:
			sensitiveScore -= 5.0
		case SensitivityLow:
			sensitiveScore -= 2.0
		}
	}
	if sensitiveScore < 0 {
		sensitiveScore = 0
	}
	score.SensitiveScore = sensitiveScore
	score.Breakdown.SensitiveDeductions = 100.0 - sensitiveScore

	// Signature bonus
	if result.Signature != nil && result.Signature.Status == SignatureValid {
		score.Breakdown.SignatureBonus = 5.0
	}

	// Remediation bonus
	if len(result.Remediations) > 0 {
		autoFixCount := 0
		for _, r := range result.Remediations {
			if r.AutoFixable {
				autoFixCount++
			}
		}
		if autoFixCount > 0 {
			score.Breakdown.RemediationBonus = float64(autoFixCount) * 2.0
			if score.Breakdown.RemediationBonus > 10.0 {
				score.Breakdown.RemediationBonus = 10.0
			}
		}
	}

	// Weighted overall score
	// Vulnerability: 40%, Compliance: 25%, Runtime: 20%, Sensitive: 15%
	score.Overall = (vulnScore * 0.40) + (compScore * 0.25) + (runtimeScore * 0.20) + (sensitiveScore * 0.15)

	// Apply bonuses
	score.Overall += score.Breakdown.SignatureBonus
	score.Overall += score.Breakdown.RemediationBonus

	// Clamp
	if score.Overall > 100 {
		score.Overall = 100
	}
	if score.Overall < 0 {
		score.Overall = 0
	}

	score.Grade = GradeFromScore(score.Overall)

	return score
}

// calculateSummary builds a VulnSummary from a scan result
func (g *Guardian) calculateSummary(result *ScanResult) VulnSummary {
	summary := VulnSummary{
		Total: len(result.Vulnerabilities),
	}

	for _, v := range result.Vulnerabilities {
		switch v.Severity {
		case SeverityCritical:
			summary.Critical++
		case SeverityHigh:
			summary.High++
		case SeverityMedium:
			summary.Medium++
		case SeverityLow:
			summary.Low++
		default:
			summary.Info++
		}
		if v.FixedIn != "" {
			summary.Fixed++
		} else {
			summary.Unfixed++
		}
	}

	for _, c := range result.Compliance {
		if c.Status == CompliancePass {
			summary.CompliancePass++
		} else if c.Status == ComplianceFail {
			summary.ComplianceFail++
		}
	}

	return summary
}
