package securityaudit

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// VulnerabilityScanner 漏洞扫描器.
type VulnerabilityScanner struct {
	vulnerabilities []Vulnerability
	reports         []VulnerabilityScanReport
	mu              sync.RWMutex
}

// NewVulnerabilityScanner 创建漏洞扫描器.
func NewVulnerabilityScanner() *VulnerabilityScanner {
	return &VulnerabilityScanner{
		vulnerabilities: make([]Vulnerability, 0),
		reports:         make([]VulnerabilityScanReport, 0),
	}
}

// Scan 运行漏洞扫描.
func (s *VulnerabilityScanner) Scan(config VulnerabilityScanConfig) VulnerabilityScanReport {
	startTime := time.Now()
	reportID := uuid.New().String()

	// 模拟扫描过程
	vulns := make([]Vulnerability, 0)

	// 模拟系统包漏洞
	if config.ScanPackages {
		vulns = append(vulns, s.scanPackages()...)
	}

	// 模拟服务漏洞
	if config.ScanServices {
		vulns = append(vulns, s.scanServices()...)
	}

	// 模拟配置漏洞
	if config.ScanConfig {
		vulns = append(vulns, s.scanConfig()...)
	}

	// 模拟网络漏洞
	if config.ScanNetwork {
		vulns = append(vulns, s.scanNetwork()...)
	}

	// 模拟端口扫描
	if config.ScanPorts {
		vulns = append(vulns, s.scanPorts()...)
	}

	// 过滤排除的 CVE
	filteredVulns := make([]Vulnerability, 0)
	for _, v := range vulns {
		excluded := false
		for _, cve := range config.ExcludeCVEs {
			if v.CVEID == cve {
				excluded = true
				break
			}
		}
		if !excluded {
			filteredVulns = append(filteredVulns, v)
		}
	}

	// 统计各严重程度数量
	criticalCount, highCount, mediumCount, lowCount := 0, 0, 0, 0
	for _, v := range filteredVulns {
		switch v.Severity {
		case VulnSeverityCritical:
			criticalCount++
		case VulnSeverityHigh:
			highCount++
		case VulnSeverityMedium:
			mediumCount++
		case VulnSeverityLow:
			lowCount++
		}
	}

	report := VulnerabilityScanReport{
		ReportID:        reportID,
		ScanTime:        startTime,
		Duration:        time.Since(startTime),
		TotalFound:      len(filteredVulns),
		CriticalCount:   criticalCount,
		HighCount:       highCount,
		MediumCount:     mediumCount,
		LowCount:        lowCount,
		Vulnerabilities: filteredVulns,
		Summary:         s.generateSummary(filteredVulns),
		Recommendations: s.generateRecommendations(filteredVulns),
	}

	// 保存报告
	s.mu.Lock()
	s.reports = append(s.reports, report)
	s.vulnerabilities = filteredVulns
	s.mu.Unlock()

	return report
}

// scanPackages 扫描系统包漏洞.
func (s *VulnerabilityScanner) scanPackages() []Vulnerability {
	// 模拟一些系统包漏洞
	return []Vulnerability{
		{
			ID:          uuid.New().String(),
			CVEID:       "CVE-2024-1234",
			Name:        "OpenSSL 远程代码执行漏洞",
			Description: "OpenSSL 存在缓冲区溢出漏洞，可能导致远程代码执行",
			Severity:    VulnSeverityCritical,
			Status:      VulnStatusOpen,
			Category:    "package",
			Affected:    "openssl",
			Version:     "1.1.1",
			FixedIn:     "1.1.2",
			CVSSScore:   9.8,
			Solution:    "升级 OpenSSL 到 1.1.2 或更高版本",
			References:  []string{"https://nvd.nist.gov/vuln/detail/CVE-2024-1234"},
			FoundAt:     time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          uuid.New().String(),
			CVEID:       "CVE-2024-5678",
			Name:        "Nginx 请求走私漏洞",
			Description: "Nginx 在特定配置下存在 HTTP 请求走私漏洞",
			Severity:    VulnSeverityHigh,
			Status:      VulnStatusOpen,
			Category:    "package",
			Affected:    "nginx",
			Version:     "1.20.0",
			FixedIn:     "1.22.0",
			CVSSScore:   7.5,
			Solution:    "升级 Nginx 到 1.22.0 或更高版本",
			References:  []string{"https://nvd.nist.gov/vuln/detail/CVE-2024-5678"},
			FoundAt:     time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
}

// scanServices 扫描服务漏洞.
func (s *VulnerabilityScanner) scanServices() []Vulnerability {
	return []Vulnerability{
		{
			ID:          uuid.New().String(),
			CVEID:       "CVE-2024-9012",
			Name:        "SMB 签名绕过漏洞",
			Description: "SMB 协议签名验证存在绕过漏洞",
			Severity:    VulnSeverityHigh,
			Status:      VulnStatusOpen,
			Category:    "service",
			Affected:    "samba",
			Version:     "4.15",
			FixedIn:     "4.18",
			CVSSScore:   7.2,
			Solution:    "升级 Samba 到 4.18 或更高版本，并强制要求 SMB 签名",
			References:  []string{"https://nvd.nist.gov/vuln/detail/CVE-2024-9012"},
			FoundAt:     time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
}

// scanConfig 扫描配置漏洞.
func (s *VulnerabilityScanner) scanConfig() []Vulnerability {
	return []Vulnerability{
		{
			ID:          uuid.New().String(),
			Name:        "SSH 弱加密算法",
			Description: "SSH 服务允许使用弱加密算法",
			Severity:    VulnSeverityMedium,
			Status:      VulnStatusOpen,
			Category:    "config",
			Affected:    "sshd",
			CVSSScore:   5.3,
			Solution:    "禁用 arcfour, 3des 等弱加密算法，只允许 aes256-ctr, chacha20-poly1305",
			References:  []string{},
			FoundAt:     time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
}

// scanNetwork 扫描网络漏洞.
func (s *VulnerabilityScanner) scanNetwork() []Vulnerability {
	return []Vulnerability{
		{
			ID:          uuid.New().String(),
			CVEID:       "CVE-2024-3456",
			Name:        "TCP 序列号预测漏洞",
			Description: "TCP 序列号生成可预测，可能被用于会话劫持",
			Severity:    VulnSeverityMedium,
			Status:      VulnStatusOpen,
			Category:    "network",
			Affected:    "kernel",
			CVSSScore:   5.9,
			Solution:    "启用 RFC 6528 规范的序列号生成",
			References:  []string{"https://nvd.nist.gov/vuln/detail/CVE-2024-3456"},
			FoundAt:     time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
}

// scanPorts 扫描开放端口.
func (s *VulnerabilityScanner) scanPorts() []Vulnerability {
	return []Vulnerability{
		{
			ID:          uuid.New().String(),
			Name:        "不必要的端口开放",
			Description: "检测到不必要的端口对外开放",
			Severity:    VulnSeverityLow,
			Status:      VulnStatusOpen,
			Category:    "network",
			Affected:    "system",
			CVSSScore:   3.1,
			Solution:    "关闭不必要的服务和端口",
			References:  []string{},
			FoundAt:     time.Now(),
			UpdatedAt:   time.Now(),
			Details: map[string]interface{}{
				"open_ports": []int{21, 23, 25},
			},
		},
	}
}

// generateSummary 生成扫描摘要.
func (s *VulnerabilityScanner) generateSummary(vulns []Vulnerability) string {
	if len(vulns) == 0 {
		return "未发现安全漏洞"
	}

	critical, high, medium, low := 0, 0, 0, 0
	for _, v := range vulns {
		switch v.Severity {
		case VulnSeverityCritical:
			critical++
		case VulnSeverityHigh:
			high++
		case VulnSeverityMedium:
			medium++
		case VulnSeverityLow:
			low++
		}
	}

	return fmt.Sprintf("发现 %d 个漏洞：严重 %d，高危 %d，中危 %d，低危 %d",
		len(vulns), critical, high, medium, low)
}

// generateRecommendations 生成修复建议.
func (s *VulnerabilityScanner) generateRecommendations(vulns []Vulnerability) []string {
	recommendations := make([]string, 0)

	hasCritical := false
	hasHigh := false
	hasPackage := false
	hasConfig := false

	for _, v := range vulns {
		if v.Severity == VulnSeverityCritical {
			hasCritical = true
		}
		if v.Severity == VulnSeverityHigh {
			hasHigh = true
		}
		if v.Category == "package" {
			hasPackage = true
		}
		if v.Category == "config" {
			hasConfig = true
		}
	}

	if hasCritical {
		recommendations = append(recommendations, "立即修复所有严重漏洞，这些漏洞可能导致远程代码执行或数据泄露")
	}
	if hasHigh {
		recommendations = append(recommendations, "尽快修复高危漏洞，建议在 7 天内完成")
	}
	if hasPackage {
		recommendations = append(recommendations, "更新受影响的软件包到最新版本")
	}
	if hasConfig {
		recommendations = append(recommendations, "检查并修复安全配置问题")
	}

	recommendations = append(recommendations, "定期运行漏洞扫描，建议每周至少一次")
	recommendations = append(recommendations, "建立漏洞响应流程，确保及时修复")

	return recommendations
}

// GetVulnerabilities 获取漏洞列表.
func (s *VulnerabilityScanner) GetVulnerabilities(severity VulnerabilitySeverity, status VulnerabilityStatus) []Vulnerability {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Vulnerability, 0)
	for _, v := range s.vulnerabilities {
		if (severity == "" || v.Severity == severity) &&
			(status == "" || v.Status == status) {
			result = append(result, v)
		}
	}
	return result
}

// GetVulnerability 获取漏洞详情.
func (s *VulnerabilityScanner) GetVulnerability(id string) (*Vulnerability, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, v := range s.vulnerabilities {
		if v.ID == id {
			return &v, nil
		}
	}
	return nil, fmt.Errorf("漏洞 %s 不存在", id)
}

// UpdateStatus 更新漏洞状态.
func (s *VulnerabilityScanner) UpdateStatus(id string, status VulnerabilityStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, v := range s.vulnerabilities {
		if v.ID == id {
			s.vulnerabilities[i].Status = status
			s.vulnerabilities[i].UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("漏洞 %s 不存在", id)
}

// FixVulnerability 修复漏洞.
func (s *VulnerabilityScanner) FixVulnerability(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, v := range s.vulnerabilities {
		if v.ID == id {
			s.vulnerabilities[i].Status = VulnStatusFixed
			s.vulnerabilities[i].UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("漏洞 %s 不存在", id)
}

// GetLatestReport 获取最新扫描报告.
func (s *VulnerabilityScanner) GetLatestReport() *VulnerabilityScanReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.reports) == 0 {
		return nil
	}
	return &s.reports[len(s.reports)-1]
}

// GetReport 获取指定报告.
func (s *VulnerabilityScanner) GetReport(id string) (*VulnerabilityScanReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.reports {
		if r.ReportID == id {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("报告 %s 不存在", id)
}
