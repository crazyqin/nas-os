// Package threatintel - scanner.go 实现漏洞扫描器，包括端口扫描、服务指纹识别
// 和 CVE 匹配功能。
package threatintel

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// ============================================================
// 扫描配置
// ============================================================

// ScanConfig 扫描配置
type ScanConfig struct {
	// Target 扫描目标（IP 或 CIDR）
	Target string `json:"target"`
	// Ports 要扫描的端口列表（空表示常用端口）
	Ports []int `json:"ports,omitempty"`
	// ScanType 扫描类型
	ScanType string `json:"scan_type"` // "quick", "full", "stealth"
	// Timeout 连接超时时间
	Timeout time.Duration `json:"timeout"`
	// MaxConcurrent 最大并发数
	MaxConcurrent int `json:"max_concurrent"`
	// ServiceDetection 是否进行服务指纹识别
	ServiceDetection bool `json:"service_detection"`
	// VulnScan 是否进行漏洞扫描
	VulnScan bool `json:"vuln_scan"`
}

// DefaultScanConfig 默认扫描配置
func DefaultScanConfig() *ScanConfig {
	return &ScanConfig{
		ScanType:         "quick",
		Timeout:          3 * time.Second,
		MaxConcurrent:    100,
		ServiceDetection: true,
		VulnScan:         true,
	}
}

// CommonPorts 常用端口列表
var CommonPorts = []int{
	21, 22, 23, 25, 53, 80, 110, 143, 443, 445,
	993, 995, 1433, 1521, 3306, 3389, 5432, 5900, 6379, 8080,
	8443, 8888, 9090, 27017,
}

// TopPortsTop100 前 100 常用端口
var TopPortsTop100 = []int{
	1, 3, 7, 9, 13, 17, 19, 21, 22, 23, 25, 26, 37, 53, 79, 80, 81, 88,
	106, 110, 111, 113, 119, 135, 139, 143, 144, 179, 199, 389, 427, 443,
	444, 445, 465, 513, 514, 515, 543, 544, 548, 554, 587, 631, 646, 873,
	990, 993, 995, 1025, 1026, 1027, 1028, 1029, 1110, 1433, 1720, 1723,
	1755, 1900, 2000, 2001, 2049, 2121, 2717, 3000, 3128, 3306, 3389,
	3986, 4899, 5000, 5009, 5051, 5060, 5101, 5190, 5357, 5432, 5631,
	5666, 5800, 5900, 6000, 6001, 6646, 7070, 8000, 8008, 8009, 8080,
	8081, 8443, 8888, 9100, 9999, 10000, 27017, 32768, 49152, 49153,
	49154, 49155, 49156, 49157,
}

// ============================================================
// 服务指纹
// ============================================================

// ServiceFingerprint 服务指纹
type ServiceFingerprint struct {
	Port     int    `json:"port"`
	Banner   string `json:"banner"`
	Service  string `json:"service"`
	Version  string `json:"version"`
	Protocol string `json:"protocol"`
}

// wellKnownServices 常见端口-服务映射
var wellKnownServices = map[int]ServiceFingerprint{
	21:    {Port: 21, Service: "ftp", Protocol: "tcp"},
	22:    {Port: 22, Service: "ssh", Protocol: "tcp"},
	23:    {Port: 23, Service: "telnet", Protocol: "tcp"},
	25:    {Port: 25, Service: "smtp", Protocol: "tcp"},
	53:    {Port: 53, Service: "dns", Protocol: "tcp/udp"},
	80:    {Port: 80, Service: "http", Protocol: "tcp"},
	110:   {Port: 110, Service: "pop3", Protocol: "tcp"},
	143:   {Port: 143, Service: "imap", Protocol: "tcp"},
	443:   {Port: 443, Service: "https", Protocol: "tcp"},
	445:   {Port: 445, Service: "smb", Protocol: "tcp"},
	993:   {Port: 993, Service: "imaps", Protocol: "tcp"},
	995:   {Port: 995, Service: "pop3s", Protocol: "tcp"},
	1433:  {Port: 1433, Service: "mssql", Protocol: "tcp"},
	3306:  {Port: 3306, Service: "mysql", Protocol: "tcp"},
	3389:  {Port: 3389, Service: "rdp", Protocol: "tcp"},
	5432:  {Port: 5432, Service: "postgresql", Protocol: "tcp"},
	5900:  {Port: 5900, Service: "vnc", Protocol: "tcp"},
	6379:  {Port: 6379, Service: "redis", Protocol: "tcp"},
	8080:  {Port: 8080, Service: "http-proxy", Protocol: "tcp"},
	8443:  {Port: 8443, Service: "https-alt", Protocol: "tcp"},
	27017: {Port: 27017, Service: "mongodb", Protocol: "tcp"},
}

// Scanner 端口扫描器
type Scanner struct {
	config  *ScanConfig
	engine  *Engine
	mu      sync.Mutex
	results []PortScanResult
}

// PortScanResult 端口扫描结果
type PortScanResult struct {
	Port    int    `json:"port"`
	State   string `json:"state"` // "open", "closed", "filtered"
	Service string `json:"service"`
	Version string `json:"version"`
	Banner  string `json:"banner"`
}

// NewScanner 创建端口扫描器
func NewScanner(config *ScanConfig, engine *Engine) *Scanner {
	if config == nil {
		config = DefaultScanConfig()
	}
	return &Scanner{
		config:  config,
		engine:  engine,
		results: make([]PortScanResult, 0),
	}
}

// ScanPorts 扫描端口
func (s *Scanner) ScanPorts(target string, ports []int) (*ScanResult, error) {
	if !s.engine.scanMgr.TryStartScan() {
		return nil, ErrScanInProgress
	}
	defer s.engine.scanMgr.FinishScan()

	scanID := fmt.Sprintf("scan-%d", time.Now().UnixNano())
	startTime := time.Now()

	result := &ScanResult{
		ID:        scanID,
		ScanType:  "port",
		Status:    ScanStatusRunning,
		Target:    target,
		StartTime: startTime,
		Services:  make([]ServiceInfo, 0),
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, s.config.MaxConcurrent)

	for _, port := range ports {
		wg.Add(1)
		sem <- struct{}{}

		go func(p int) {
			defer wg.Done()
			defer func() { <-sem }()

			portResult := s.scanPort(target, p)

			mu.Lock()
			if portResult.State == "open" {
				result.OpenPorts++
				result.Services = append(result.Services, ServiceInfo{
					Port:        p,
					Protocol:    "tcp",
					ServiceName: portResult.Service,
					Version:     portResult.Version,
					Banner:      portResult.Banner,
					State:       portResult.State,
				})
			}
			result.TotalPorts++
			mu.Unlock()
		}(port)
	}

	wg.Wait()

	result.Status = ScanStatusComplete
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.RiskScore = s.calculateRiskScore(result)
	result.Summary = fmt.Sprintf("扫描完成: %d/%d 端口开放", result.OpenPorts, result.TotalPorts)

	s.engine.SaveScanResult(result)
	return result, nil
}

// ScanCommonPorts 扫描常用端口
func (s *Scanner) ScanCommonPorts(target string) (*ScanResult, error) {
	return s.ScanPorts(target, CommonPorts)
}

// scanPort 扫描单个端口
func (s *Scanner) scanPort(target string, port int) PortScanResult {
	// 扫描端口
	addr := net.JoinHostPort(target, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, s.config.Timeout)

	result := PortScanResult{
		Port:  port,
		State: "closed",
	}

	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			result.State = "filtered"
		}
		return result
	}
	defer conn.Close()

	result.State = "open"

	// 服务指纹识别
	if s.config.ServiceDetection {
		if fp, exists := wellKnownServices[port]; exists {
			result.Service = fp.Service
		}

		// 尝试读取 Banner
		banner := s.readBanner(conn)
		if banner != "" {
			result.Banner = banner
			result.Version = s.detectVersion(banner, result.Service)
		}
	}

	return result
}

// readBanner 读取服务 Banner
func (s *Scanner) readBanner(conn net.Conn) string {
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(buf[:n]))
}

// detectVersion 从 Banner 检测服务版本
func (s *Scanner) detectVersion(banner, service string) string {
	banner = strings.ToLower(banner)

	switch {
	case service == "ssh" && strings.HasPrefix(banner, "ssh-"):
		return banner
	case service == "ftp" && strings.Contains(banner, "ftp"):
		return banner
	case service == "smtp" && strings.Contains(banner, "smtp"):
		return banner
	case service == "http" && strings.Contains(banner, "server:"):
		parts := strings.SplitN(banner, "server:", 2)
		if len(parts) > 1 {
			return strings.TrimSpace(parts[1])
		}
	}

	return ""
}

// calculateRiskScore 计算扫描风险评分
func (s *Scanner) calculateRiskScore(result *ScanResult) int {
	riskScore := 0

	// 开放端口数量风险
	switch {
	case result.OpenPorts > 20:
		riskScore += 40
	case result.OpenPorts > 10:
		riskScore += 30
	case result.OpenPorts > 5:
		riskScore += 20
	default:
		riskScore += 10
	}

	// 高风险服务风险
	highRiskServices := map[string]bool{
		"telnet": true, "ftp": true, "vnc": true, "rdp": true,
		"redis": true, "mongodb": true, "mssql": true,
	}

	for _, svc := range result.Services {
		if highRiskServices[svc.ServiceName] {
			riskScore += 15
		}
	}

	// 漏洞风险
	for _, vuln := range result.Vulnerabilities {
		switch vuln.Severity {
		case SeverityCritical:
			riskScore += 25
		case SeverityHigh:
			riskScore += 15
		case SeverityMedium:
			riskScore += 10
		case SeverityLow:
			riskScore += 5
		}
	}

	if riskScore > 100 {
		riskScore = 100
	}

	return riskScore
}

// MatchVulnerabilities 匹配已知漏洞
func (s *Scanner) MatchVulnerabilities(services []ServiceInfo) []Vulnerability {
	var vulns []Vulnerability

	for _, svc := range services {
		// 检查常见漏洞
		for _, knownVuln := range CommonVulns {
			if strings.Contains(strings.ToLower(svc.ServiceName), knownVuln.Service) ||
				strings.Contains(strings.ToLower(svc.Banner), knownVuln.Service) {
				vuln := Vulnerability{
					ID:              fmt.Sprintf("vuln-%s-%d", knownVuln.CVE, svc.Port),
					CVE:             knownVuln.CVE,
					Title:           knownVuln.Title,
					Description:     knownVuln.Description,
					Severity:        knownVuln.Severity,
					CVSS:            knownVuln.CVSS,
					AffectedService: svc.ServiceName,
					AffectedPort:    svc.Port,
					Solution:        knownVuln.Solution,
					PublishedAt:     time.Now(),
				}
				vulns = append(vulns, vuln)
			}
		}

		// 检查不安全配置
		if svc.ServiceName == "telnet" {
			vulns = append(vulns, Vulnerability{
				ID:              fmt.Sprintf("vuln-insecure-telnet-%d", svc.Port),
				Title:           "不安全的 Telnet 服务",
				Description:     "Telnet 使用明文传输，存在凭据泄露风险",
				Severity:        SeverityHigh,
				CVSS:            7.5,
				AffectedService: "telnet",
				AffectedPort:    svc.Port,
				Solution:        "禁用 Telnet，使用 SSH 替代",
			})
		}

		if svc.ServiceName == "ftp" {
			vulns = append(vulns, Vulnerability{
				ID:              fmt.Sprintf("vuln-insecure-ftp-%d", svc.Port),
				Title:           "不安全的 FTP 服务",
				Description:     "FTP 使用明文传输凭据",
				Severity:        SeverityMedium,
				CVSS:            5.0,
				AffectedService: "ftp",
				AffectedPort:    svc.Port,
				Solution:        "使用 SFTP 或 FTPS 替代",
			})
		}
	}

	return vulns
}

// QuickScan 快速扫描（仅常用端口，无漏洞检测）
func (s *Scanner) QuickScan(target string) (*ScanResult, error) {
	origDetection := s.config.ServiceDetection
	origVuln := s.config.VulnScan
	s.config.ServiceDetection = false
	s.config.VulnScan = false
	defer func() {
		s.config.ServiceDetection = origDetection
		s.config.VulnScan = origVuln
	}()

	return s.ScanPorts(target, CommonPorts)
}

// FullScan 全面扫描（Top 100 端口 + 服务识别 + 漏洞匹配）
func (s *Scanner) FullScan(target string) (*ScanResult, error) {
	origDetection := s.config.ServiceDetection
	origVuln := s.config.VulnScan
	s.config.ServiceDetection = true
	s.config.VulnScan = true
	defer func() {
		s.config.ServiceDetection = origDetection
		s.config.VulnScan = origVuln
	}()

	result, err := s.ScanPorts(target, TopPortsTop100)
	if err != nil {
		return nil, err
	}

	// 匹配漏洞
	if s.config.VulnScan {
		vulns := s.MatchVulnerabilities(result.Services)
		result.Vulnerabilities = vulns
		result.RiskScore = s.calculateRiskScore(result)
		result.ScanType = "full"
	}

	return result, nil
}
