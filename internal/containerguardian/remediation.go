package containerguardian

import (
	"fmt"
)

// generateRemediations produces actionable remediation suggestions based on scan results.
func (g *Guardian) generateRemediations(result *ScanResult) []Remediation {
	remediations := make([]Remediation, 0)
	idCounter := 0

	// Vulnerability-based remediations
	for _, v := range result.Vulnerabilities {
		if v.FixedIn != "" {
			idCounter++
			remediations = append(remediations, Remediation{
				ID:          fmt.Sprintf("REM-%03d", idCounter),
				Category:    "vulnerability",
				Severity:    v.Severity,
				Title:       fmt.Sprintf("Upgrade %s to fix %s", v.Package, v.CVE),
				Description: fmt.Sprintf("Package %s version %s has %s vulnerability (%s). Upgrade to %s.", v.Package, v.Version, v.Severity, v.CVE, v.FixedIn),
				Action:      fmt.Sprintf("Update %s from %s to %s", v.Package, v.Version, v.FixedIn),
				AutoFixable: true,
			})
		} else {
			idCounter++
			remediations = append(remediations, Remediation{
				ID:          fmt.Sprintf("REM-%03d", idCounter),
				Category:    "vulnerability",
				Severity:    v.Severity,
				Title:       fmt.Sprintf("Monitor %s for fix (%s)", v.Package, v.CVE),
				Description: fmt.Sprintf("Package %s has %s vulnerability (%s) but no fix is available yet. Monitor upstream for patches.", v.Package, v.Severity, v.CVE),
				Action:      "Monitor upstream and apply patch when available; consider alternatives or workarounds",
				AutoFixable: false,
			})
		}
	}

	// Compliance-based remediations
	for _, c := range result.Compliance {
		if c.Status == ComplianceFail {
			idCounter++
			remediations = append(remediations, Remediation{
				ID:          fmt.Sprintf("REM-%03d", idCounter),
				Category:    "compliance",
				Severity:    c.Severity,
				Title:       fmt.Sprintf("Fix compliance: %s", c.Name),
				Description: c.Description,
				Action:      g.complianceAction(c.ID),
				AutoFixable: g.isComplianceAutoFixable(c.ID),
			})
		}
	}

	// Signature-based remediations
	if result.Signature != nil && result.Signature.Status == SignatureMissing {
		idCounter++
		remediations = append(remediations, Remediation{
			ID:          fmt.Sprintf("REM-%03d", idCounter),
			Category:    "signature",
			Severity:    SeverityHigh,
			Title:       "Sign container image",
			Description: "Container image is not signed. Unsigned images can be tampered with in transit.",
			Action:      "Use Cosign or Notary to sign images: `cosign sign --key cosign.key <image>`",
			AutoFixable: false,
		})
	}

	// Sensitive data remediations
	for _, s := range result.Sensitive {
		idCounter++
		remediations = append(remediations, Remediation{
			ID:          fmt.Sprintf("REM-%03d", idCounter),
			Category:    "sensitive",
			Severity:    sensitivityToSeverity(s.Sensitivity),
			Title:       fmt.Sprintf("Remove sensitive data: %s", s.Type),
			Description: s.Description,
			Action:      s.Remediation,
			AutoFixable: false,
		})
	}

	return remediations
}

// complianceAction returns the recommended action for a compliance rule
func (g *Guardian) complianceAction(ruleID string) string {
	actions := map[string]string{
		"CIS-4.1":  "Add USER directive in Dockerfile: `USER 1000:1000`",
		"CIS-4.2":  "Use official images from trusted registries with verified publishers",
		"CIS-5.3":  "Add --cap-drop=ALL and only add required capabilities with --cap-add",
		"CIS-5.4":  "Remove --privileged flag; use specific capabilities instead",
		"CIS-5.5":  "Remove sensitive host mounts; use Docker volumes with named volumes",
		"CIS-5.10": "Use bridge or custom network: `docker network create`",
		"CIS-5.12": "Set memory limit: `docker run --memory=512m <image>`",
		"CIS-5.14": "Set CPU limit: `docker run --cpus=1.0 <image>`",
		"CIS-5.15": "Mount root filesystem read-only: `docker run --read-only <image>`",
		"CIS-5.25": "Add --security-opt=no-new-privileges",
		"CIS-5.28": "Set PID limit: `docker run --pids-limit=100 <image>`",
		"CIS-5.31": "Remove -v /var/run/docker.sock mount from container",
	}
	if action, ok := actions[ruleID]; ok {
		return action
	}
	return "Review compliance rule and apply recommended configuration"
}

// isComplianceAutoFixable returns whether a compliance issue can be auto-fixed
func (g *Guardian) isComplianceAutoFixable(ruleID string) bool {
	autoFixable := map[string]bool{
		"CIS-4.1":  true,
		"CIS-5.12": true,
		"CIS-5.14": true,
		"CIS-5.28": true,
	}
	return autoFixable[ruleID]
}

// sensitivityToSeverity converts sensitivity level to severity for remediation
func sensitivityToSeverity(level SensitivityLevel) string {
	switch level {
	case SensitivityCritical:
		return SeverityCritical
	case SensitivityHigh:
		return SeverityHigh
	case SensitivityMedium:
		return SeverityMedium
	case SensitivityLow:
		return SeverityLow
	default:
		return SeverityInfo
	}
}
