package containerguardian

import (
	"context"
	"strings"
	"time"
)

// scanVulnerabilities scans an image for known CVE vulnerabilities from the built-in database
func (g *Guardian) scanVulnerabilities(ctx context.Context, image string) ([]Vulnerability, error) {
	g.vulnMu.RLock()
	defer g.vulnMu.RUnlock()

	vulns := make([]Vulnerability, 0)

	// Exact match
	if found, ok := g.vulnDB[image]; ok {
		vulns = append(vulns, found...)
	}

	// Prefix match (e.g., "nginx:1.20" matches "nginx:1.20.x")
	for key, dbVulns := range g.vulnDB {
		if key != image && strings.HasPrefix(image, key) {
			vulns = append(vulns, dbVulns...)
		}
	}

	return vulns, nil
}

// initVulnDB initializes the built-in CVE vulnerability database
func (g *Guardian) initVulnDB() {
	now := time.Now()

	g.vulnDB["nginx:1.20"] = []Vulnerability{
		{
			ID: "VULN-001", CVE: "CVE-2021-23017", Severity: SeverityHigh,
			Title: "DNS Resolver Off-By-One Heap Write", Description: "A security issue in nginx resolver was identified which might allow an attacker who is able to forge UDP packets from the DNS server to cause 1-byte memory overwrite.",
			Package: "nginx", Version: "1.20.0", FixedIn: "1.20.1",
			URL: "https://nvd.nist.gov/vuln/detail/CVE-2021-23017", PublishedAt: now.Add(-720 * 24 * time.Hour),
		},
		{
			ID: "VULN-002", CVE: "CVE-2021-23018", Severity: SeverityMedium,
			Title: "HTTP/2 Memory Leak", Description: "Memory disclosure via HTTP/2 connection handling.",
			Package: "nginx", Version: "1.20.0", FixedIn: "1.20.1",
			URL: "https://nvd.nist.gov/vuln/detail/CVE-2021-23018", PublishedAt: now.Add(-720 * 24 * time.Hour),
		},
		{
			ID: "VULN-003", CVE: "CVE-2022-41741", Severity: SeverityCritical,
			Title: "mp4 Module Memory Corruption", Description: "Memory corruption in the ngx_http_mp4_module.",
			Package: "nginx", Version: "1.20.0", FixedIn: "1.20.2",
			URL: "https://nvd.nist.gov/vuln/detail/CVE-2022-41741", PublishedAt: now.Add(-600 * 24 * time.Hour),
		},
	}

	g.vulnDB["redis:6.0"] = []Vulnerability{
		{
			ID: "VULN-010", CVE: "CVE-2021-32625", Severity: SeverityCritical,
			Title: "Integer Overflow Heap Overflow", Description: "An integer overflow bug in Redis 6.0 and newer can be exploited using the STRALGO LCS command to corrupt the heap.",
			Package: "redis", Version: "6.0.0", FixedIn: "6.0.13",
			URL: "https://nvd.nist.gov/vuln/detail/CVE-2021-32625", PublishedAt: now.Add(-700 * 24 * time.Hour),
		},
		{
			ID: "VULN-011", CVE: "CVE-2022-24735", Severity: SeverityHigh,
			Title: "Lua Script Sandbox Escape", Description: "Authenticated users can use Lua scripts to manipulate non-local heap memory.",
			Package: "redis", Version: "6.0.0", FixedIn: "6.0.16",
			URL: "https://nvd.nist.gov/vuln/detail/CVE-2022-24735", PublishedAt: now.Add(-500 * 24 * time.Hour),
		},
	}

	g.vulnDB["mysql:5.7"] = []Vulnerability{
		{
			ID: "VULN-020", CVE: "CVE-2021-2154", Severity: SeverityHigh,
			Title: "Server DML Privilege Escalation", Description: "Vulnerability in MySQL Server allowing high privileged attacker to compromise the server.",
			Package: "mysql-server", Version: "5.7.0", FixedIn: "5.7.34",
			URL: "https://nvd.nist.gov/vuln/detail/CVE-2021-2154", PublishedAt: now.Add(-730 * 24 * time.Hour),
		},
		{
			ID: "VULN-021", CVE: "CVE-2021-2160", Severity: SeverityMedium,
			Title: "Server Optimizer DoS", Description: "Vulnerability in MySQL Server allowing denial of service via optimizer component.",
			Package: "mysql-server", Version: "5.7.0", FixedIn: "5.7.34",
			URL: "https://nvd.nist.gov/vuln/detail/CVE-2021-2160", PublishedAt: now.Add(-730 * 24 * time.Hour),
		},
	}

	g.vulnDB["postgres:13"] = []Vulnerability{
		{
			ID: "VULN-030", CVE: "CVE-2021-32027", Severity: SeverityHigh,
			Title: "Buffer Overflow in array aggregation", Description: "Buffer overrun from integer underflow in array manipulation.",
			Package: "postgresql-13", Version: "13.0", FixedIn: "13.3",
			URL: "https://nvd.nist.gov/vuln/detail/CVE-2021-32027", PublishedAt: now.Add(-710 * 24 * time.Hour),
		},
	}

	g.vulnDB["ubuntu:20.04"] = []Vulnerability{
		{
			ID: "VULN-040", CVE: "CVE-2021-3493", Severity: SeverityHigh,
			Title: "OverlayFS Privilege Escalation", Description: "The overlayfs implementation did not properly handle user namespaces allowing local privilege escalation.",
			Package: "linux-aws", Version: "5.4.0", FixedIn: "5.4.0-1045.47",
			URL: "https://nvd.nist.gov/vuln/detail/CVE-2021-3493", PublishedAt: now.Add(-700 * 24 * time.Hour),
		},
	}

	g.vulnDB["node:14"] = []Vulnerability{
		{
			ID: "VULN-050", CVE: "CVE-2021-22930", Severity: SeverityCritical,
			Title: "HTTP Request Smuggling", Description: "Node.js before 16.6.0 is vulnerable to HTTP request smuggling.",
			Package: "nodejs", Version: "14.0.0", FixedIn: "14.17.4",
			URL: "https://nvd.nist.gov/vuln/detail/CVE-2021-22930", PublishedAt: now.Add(-700 * 24 * time.Hour),
		},
		{
			ID: "VULN-051", CVE: "CVE-2021-22931", Severity: SeverityHigh,
			Title: "DNS rebinding in --inspect", Description: "DNS rebinding and denial of service with debugger enabled.",
			Package: "nodejs", Version: "14.0.0", FixedIn: "14.17.4",
			URL: "https://nvd.nist.gov/vuln/detail/CVE-2021-22931", PublishedAt: now.Add(-700 * 24 * time.Hour),
		},
	}

	g.vulnDB["python:3.9"] = []Vulnerability{
		{
			ID: "VULN-060", CVE: "CVE-2021-3733", Severity: SeverityMedium,
			Title: "urllib Regular Expression DoS", Description: "The urllib library is vulnerable to ReDoS via a HTTP header.",
			Package: "python3.9", Version: "3.9.0", FixedIn: "3.9.7",
			URL: "https://nvd.nist.gov/vuln/detail/CVE-2021-3733", PublishedAt: now.Add(-680 * 24 * time.Hour),
		},
	}
}
