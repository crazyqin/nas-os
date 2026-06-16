package containerguardian

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// GenerateReport generates a security report in the specified format.
func (g *Guardian) GenerateReport(scanID string, format ReportFormat) ([]byte, error) {
	g.resultMu.RLock()
	result, exists := g.results[scanID]
	g.resultMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("scan %s not found", scanID)
	}

	var data []byte
	var err error

	switch format {
	case ReportFormatJSON:
		data, err = g.generateJSONReport(result)
	case ReportFormatHTML:
		data, err = g.generateHTMLReport(result)
	default:
		data, err = g.generateJSONReport(result)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to generate report: %w", err)
	}

	reportID := fmt.Sprintf("report-%s-%s", scanID, format)
	g.reportMu.Lock()
	g.reports[reportID] = data
	g.reportMu.Unlock()

	g.logger.Info("report generated",
		zap.String("report_id", reportID),
		zap.String("format", string(format)),
		zap.Int("size", len(data)))

	return data, nil
}

// generateJSONReport generates a JSON format report
func (g *Guardian) generateJSONReport(result *ScanResult) ([]byte, error) {
	report := map[string]interface{}{
		"report_id":    fmt.Sprintf("RPT-%s-%d", result.ID, time.Now().UnixNano()),
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"image":        result.Image,
		"scan_id":      result.ID,
		"grade":        result.Grade,
		"score":        result.Score,
		"summary":      result.Summary,
		"vulnerabilities": result.Vulnerabilities,
		"compliance":   result.Compliance,
		"signature":    result.Signature,
		"sensitive":    result.Sensitive,
		"runtime":      result.Runtime,
		"remediations": result.Remediations,
		"scanned_at":   result.ScannedAt,
		"duration":     result.Duration.String(),
	}

	return json.MarshalIndent(report, "", "  ")
}

// generateHTMLReport generates an HTML format report
func (g *Guardian) generateHTMLReport(result *ScanResult) ([]byte, error) {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Container Security Report</title>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; margin: 0; padding: 20px; background: #f5f5f5; }
  .container { max-width: 1200px; margin: 0 auto; }
  .card { background: white; border-radius: 8px; padding: 24px; margin-bottom: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
  .header { text-align: center; }
  .grade { font-size: 72px; font-weight: bold; }
  .grade-A { color: #22c55e; }
  .grade-B { color: #84cc16; }
  .grade-C { color: #eab308; }
  .grade-D { color: #f97316; }
  .grade-F { color: #ef4444; }
  .score { font-size: 24px; color: #666; }
  .meta { color: #888; font-size: 14px; margin-top: 8px; }
  .section-title { font-size: 18px; font-weight: 600; margin-bottom: 12px; color: #333; }
  table { width: 100%; border-collapse: collapse; }
  th, td { padding: 10px 12px; text-align: left; border-bottom: 1px solid #eee; font-size: 14px; }
  th { background: #f8f9fa; font-weight: 600; color: #555; }
  .severity-critical { color: #dc2626; font-weight: bold; }
  .severity-high { color: #ea580c; font-weight: bold; }
  .severity-medium { color: #ca8a04; }
  .severity-low { color: #16a34a; }
  .status-pass { color: #22c55e; } .status-fail { color: #ef4444; } .status-warn { color: #eab308; }
  .badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 12px; font-weight: 500; }
  .badge-pass { background: #dcfce7; color: #166534; }
  .badge-fail { background: #fee2e2; color: #991b1b; }
  .badge-warn { background: #fef9c3; color: #854d0e; }
  .remediation { background: #f0f9ff; border-left: 4px solid #3b82f6; padding: 12px; margin: 8px 0; border-radius: 0 4px 4px 0; }
</style>
</head>
<body>
<div class="container">
`)

	// Header card
	sb.WriteString(fmt.Sprintf(`<div class="card header">
<h1>Container Security Report</h1>
<div class="grade grade-%s">%s</div>
<div class="score">Score: %.1f / 100</div>
<div class="meta">Image: %s | Scanned: %s | Duration: %s</div>
</div>
`, string(result.Grade), string(result.Grade), result.Score, result.Image,
		result.ScannedAt.Format(time.RFC3339), result.Duration))

	// Summary card
	sb.WriteString(`<div class="card">
<div class="section-title">Summary</div>
<table>
<tr><th>Metric</th><th>Count</th></tr>`)
	sb.WriteString(fmt.Sprintf("<tr><td>Total Vulnerabilities</td><td>%d</td></tr>", result.Summary.Total))
	sb.WriteString(fmt.Sprintf("<tr><td>Critical</td><td class=\"severity-critical\">%d</td></tr>", result.Summary.Critical))
	sb.WriteString(fmt.Sprintf("<tr><td>High</td><td class=\"severity-high\">%d</td></tr>", result.Summary.High))
	sb.WriteString(fmt.Sprintf("<tr><td>Medium</td><td class=\"severity-medium\">%d</td></tr>", result.Summary.Medium))
	sb.WriteString(fmt.Sprintf("<tr><td>Low</td><td class=\"severity-low\">%d</td></tr>", result.Summary.Low))
	sb.WriteString(fmt.Sprintf("<tr><td>With Fix</td><td>%d</td></tr>", result.Summary.Fixed))
	sb.WriteString(fmt.Sprintf("<tr><td>Without Fix</td><td>%d</td></tr>", result.Summary.Unfixed))
	sb.WriteString(fmt.Sprintf("<tr><td>Compliance Pass</td><td>%d</td></tr>", result.Summary.CompliancePass))
	sb.WriteString(fmt.Sprintf("<tr><td>Compliance Fail</td><td>%d</td></tr>", result.Summary.ComplianceFail))
	sb.WriteString("</table></div>")

	// Vulnerabilities card
	if len(result.Vulnerabilities) > 0 {
		sb.WriteString(`<div class="card"><div class="section-title">Vulnerabilities</div><table>
<tr><th>CVE</th><th>Severity</th><th>Package</th><th>Installed</th><th>Fixed In</th><th>Title</th></tr>`)
		for _, v := range result.Vulnerabilities {
			sb.WriteString(fmt.Sprintf("<tr><td>%s</td><td class=\"severity-%s\">%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
				v.CVE, strings.ToLower(v.Severity), v.Severity, v.Package, v.Version, v.FixedIn, v.Title))
		}
		sb.WriteString("</table></div>")
	}

	// Compliance card
	if len(result.Compliance) > 0 {
		sb.WriteString(`<div class="card"><div class="section-title">CIS Docker Benchmark Compliance</div><table>
<tr><th>ID</th><th>Rule</th><th>Status</th><th>Severity</th><th>Message</th></tr>`)
		for _, c := range result.Compliance {
			statusClass := "pass"
			if c.Status == ComplianceFail {
				statusClass = "fail"
			} else if c.Status == ComplianceWarn {
				statusClass = "warn"
			}
			sb.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td><td><span class=\"badge badge-%s\">%s</span></td><td class=\"severity-%s\">%s</td><td>%s</td></tr>",
				c.ID, c.Name, statusClass, string(c.Status), strings.ToLower(c.Severity), c.Severity, c.Message))
		}
		sb.WriteString("</table></div>")
	}

	// Signature card
	if result.Signature != nil {
		sb.WriteString(`<div class="card"><div class="section-title">Image Signature</div><table>`)
		sb.WriteString(fmt.Sprintf("<tr><td>Status</td><td>%s</td></tr>", result.Signature.Status))
		if result.Signature.Signer != "" {
			sb.WriteString(fmt.Sprintf("<tr><td>Signer</td><td>%s</td></tr>", result.Signature.Signer))
		}
		if result.Signature.Algorithm != "" {
			sb.WriteString(fmt.Sprintf("<tr><td>Algorithm</td><td>%s</td></tr>", result.Signature.Algorithm))
		}
		sb.WriteString("</table></div>")
	}

	// Remediations card
	if len(result.Remediations) > 0 {
		sb.WriteString(`<div class="card"><div class="section-title">Remediations</div>`)
		for _, r := range result.Remediations {
			autoFix := ""
			if r.AutoFixable {
				autoFix = " [Auto-fixable]"
			}
			sb.WriteString(fmt.Sprintf(`<div class="remediation">
<strong class="severity-%s">%s</strong>%s<br>
<span>%s</span><br>
<em>Action: %s</em>
</div>`, strings.ToLower(r.Severity), r.Title, autoFix, r.Description, r.Action))
		}
		sb.WriteString("</div>")
	}

	sb.WriteString(`</div></body></html>`)

	return []byte(sb.String()), nil
}
