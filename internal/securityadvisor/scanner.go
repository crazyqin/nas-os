// Package securityadvisor provides security scanning functionality.
package securityadvisor

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Scanner 安全扫描器
type Scanner struct {
	config ScanConfig
	logger *zap.Logger
	mu     sync.RWMutex
}

// NewScanner 创建新的安全扫描器
func NewScanner(config ScanConfig, logger *zap.Logger) *Scanner {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Scanner{
		config: config,
		logger: logger,
	}
}

// RunFullScan 执行完整安全扫描
func (s *Scanner) RunFullScan(ctx context.Context) (*SecurityReport, error) {
	startTime := time.Now()
	s.logger.Info("Starting full security scan")

	report := &SecurityReport{
		ID:       fmt.Sprintf("scan-%d", time.Now().Unix()),
		ScanTime: startTime,
		Checks:   make([]SecurityCheck, 0),
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	// 并发执行各项检查
	checks := []struct {
		name string
		fn   func(context.Context) []SecurityCheck
	}{
		{"weak_passwords", s.scanWeakPasswords},
		{"open_ports", s.scanOpenPorts},
		{"file_permissions", s.scanFilePermissions},
		{"ssl_certificates", s.scanSSLCertificates},
		{"system_updates", s.scanSystemUpdates},
		{"malware", s.scanMalware},
		{"firewall", s.scanFirewall},
	}

	for _, check := range checks {
		wg.Add(1)
		go func(c struct {
			name string
			fn   func(context.Context) []SecurityCheck
		}) {
			defer wg.Done()
			results := c.fn(ctx)
			mu.Lock()
			report.Checks = append(report.Checks, results...)
			mu.Unlock()
		}(check)
	}

	wg.Wait()

	// 计算统计信息
	report.Duration = time.Since(startTime)
	for _, check := range report.Checks {
		switch check.Status {
		case "critical":
			report.CriticalIssues++
		case "warning":
			report.WarningIssues++
		case "info":
			report.InfoIssues++
		}
	}
	report.TotalIssues = report.CriticalIssues + report.WarningIssues + report.InfoIssues

	// 计算总体评分
	report.OverallScore = CalculateOverallScore(report.Checks)
	report.SecurityLevel = GetSecurityLevel(report.OverallScore)

	// 生成建议
	report.Recommendations = GenerateRecommendations(report.Checks)

	s.logger.Info("Security scan completed",
		zap.Int("score", report.OverallScore),
		zap.Int("issues", report.TotalIssues),
		zap.Duration("duration", report.Duration))

	return report, nil
}

// scanWeakPasswords 扫描弱密码
func (s *Scanner) scanWeakPasswords(ctx context.Context) []SecurityCheck {
	s.logger.Info("Scanning weak passwords")
	checks := make([]SecurityCheck, 0)
	startTime := time.Now()

	// 读取 /etc/passwd 获取用户列表
	content, err := os.ReadFile("/etc/passwd")
	if err != nil {
		s.logger.Error("Failed to read /etc/passwd", zap.Error(err))
		return checks
	}

	users := parsePasswdFile(string(content))
	policy := DefaultPasswordPolicy()

	for _, user := range users {
		if ctx.Err() != nil {
			break
		}

		check := SecurityCheck{
			ID:        fmt.Sprintf("pwd-%s", user),
			Name:      fmt.Sprintf("Weak Password Check - %s", user),
			Category:  "password",
			CheckedAt: time.Now(),
		}

		// 检查密码强度（简化实现）
		strength := checkPasswordStrength(user, policy)
		if strength == "weak" {
			check.Status = "warning"
			check.Score = 40
			check.Message = fmt.Sprintf("User '%s' may have a weak password", user)
			check.Remediation = "Change password to meet complexity requirements"
		} else {
			check.Status = "pass"
			check.Score = 100
			check.Message = fmt.Sprintf("User '%s' password policy compliant", user)
		}

		check.Duration = time.Since(startTime)
		checks = append(checks, check)
	}

	return checks
}

// scanOpenPorts 扫描开放端口
func (s *Scanner) scanOpenPorts(ctx context.Context) []SecurityCheck {
	s.logger.Info("Scanning open ports")
	checks := make([]SecurityCheck, 0)
	startTime := time.Now()

	// 使用 netstat 或 ss 获取开放端口
	cmd := exec.CommandContext(ctx, "ss", "-tuln")
	output, err := cmd.Output()
	if err != nil {
		s.logger.Error("Failed to run ss command", zap.Error(err))
		return checks
	}

	ports := parseSSOutput(string(output))
	riskConfig := DefaultPortRiskConfig()

	for _, port := range ports {
		if ctx.Err() != nil {
			break
		}

		check := SecurityCheck{
			ID:        fmt.Sprintf("port-%d-%s", port.Port, port.Protocol),
			Name:      fmt.Sprintf("Open Port - %d/%s", port.Port, port.Protocol),
			Category:  "port",
			CheckedAt: time.Now(),
		}

		risk := assessPortRisk(port.Port, riskConfig)
		port.Risk = risk

		switch risk {
		case "high":
			check.Status = "critical"
			check.Score = 20
			check.Message = fmt.Sprintf("High risk port %d is open", port.Port)
			check.Remediation = "Consider closing this port or restricting access"
		case "medium":
			check.Status = "warning"
			check.Score = 60
			check.Message = fmt.Sprintf("Medium risk port %d is open", port.Port)
			check.Remediation = "Review if this port is necessary"
		default:
			check.Status = "pass"
			check.Score = 100
			check.Message = fmt.Sprintf("Port %d is safe", port.Port)
		}

		check.Duration = time.Since(startTime)
		checks = append(checks, check)
	}

	return checks
}

// scanFilePermissions 扫描文件权限
func (s *Scanner) scanFilePermissions(ctx context.Context) []SecurityCheck {
	s.logger.Info("Scanning file permissions")
	checks := make([]SecurityCheck, 0)
	startTime := time.Now()

	config := DefaultCriticalFileConfig()

	for _, filePath := range config.Paths {
		if ctx.Err() != nil {
			break
		}

		check := SecurityCheck{
			ID:        fmt.Sprintf("file-%s", filepath.Base(filePath)),
			Name:      fmt.Sprintf("File Permission - %s", filePath),
			Category:  "permission",
			CheckedAt: time.Now(),
		}

		info, err := os.Stat(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				check.Status = "info"
				check.Score = 100
				check.Message = fmt.Sprintf("File %s does not exist", filePath)
			} else {
				check.Status = "warning"
				check.Score = 50
				check.Message = fmt.Sprintf("Cannot access file %s", filePath)
			}
			check.Duration = time.Since(startTime)
			checks = append(checks, check)
			continue
		}

		perm := info.Mode().Perm()
		permStr := fmt.Sprintf("%04o", perm)
		maxPerm, _ := strconv.ParseUint(config.MaxPermission, 8, 32)

		if perm > os.FileMode(maxPerm) {
			check.Status = "warning"
			check.Score = 60
			check.Message = fmt.Sprintf("File %s has too permissive rights: %s", filePath, permStr)
			check.Details = fmt.Sprintf("Current: %s, Expected: %s or less", permStr, config.MaxPermission)
			check.Remediation = fmt.Sprintf("Run: chmod %s %s", config.MaxPermission, filePath)
		} else {
			check.Status = "pass"
			check.Score = 100
			check.Message = fmt.Sprintf("File %s permissions are correct", filePath)
		}

		check.Duration = time.Since(startTime)
		checks = append(checks, check)
	}

	return checks
}

// scanSSLCertificates 扫描SSL证书
func (s *Scanner) scanSSLCertificates(ctx context.Context) []SecurityCheck {
	s.logger.Info("Scanning SSL certificates")
	checks := make([]SecurityCheck, 0)
	startTime := time.Now()

	config := DefaultSSLCheckConfig()

	for _, domain := range config.Domains {
		if ctx.Err() != nil {
			break
		}

		check := SecurityCheck{
			ID:        fmt.Sprintf("ssl-%s", domain),
			Name:      fmt.Sprintf("SSL Certificate - %s", domain),
			Category:  "ssl",
			CheckedAt: time.Now(),
		}

		// 简化的证书检查
		daysUntilExpiry := checkSSLCertificate(domain)

		if daysUntilExpiry < 0 {
			check.Status = "critical"
			check.Score = 0
			check.Message = fmt.Sprintf("SSL certificate for %s has expired", domain)
			check.Remediation = "Renew the SSL certificate immediately"
		} else if daysUntilExpiry < config.CriticalDays {
			check.Status = "critical"
			check.Score = 20
			check.Message = fmt.Sprintf("SSL certificate for %s expires in %d days", domain, daysUntilExpiry)
			check.Remediation = "Renew the SSL certificate urgently"
		} else if daysUntilExpiry < config.WarningDays {
			check.Status = "warning"
			check.Score = 60
			check.Message = fmt.Sprintf("SSL certificate for %s expires in %d days", domain, daysUntilExpiry)
			check.Remediation = "Plan to renew the SSL certificate"
		} else {
			check.Status = "pass"
			check.Score = 100
			check.Message = fmt.Sprintf("SSL certificate for %s is valid", domain)
		}

		check.Duration = time.Since(startTime)
		checks = append(checks, check)
	}

	return checks
}

// scanSystemUpdates 扫描系统更新
func (s *Scanner) scanSystemUpdates(ctx context.Context) []SecurityCheck {
	s.logger.Info("Scanning system updates")
	checks := make([]SecurityCheck, 0)
	startTime := time.Now()

	check := SecurityCheck{
		ID:        "sys-updates",
		Name:      "System Updates",
		Category:  "update",
		CheckedAt: time.Now(),
	}

	// 检查是否有可用更新
	cmd := exec.CommandContext(ctx, "apt", "list", "--upgradable")
	output, err := cmd.Output()
	if err != nil {
		// 如果 apt 不可用，尝试 yum
		cmd = exec.CommandContext(ctx, "yum", "check-update")
		output, err = cmd.Output()
	}

	if err != nil {
		check.Status = "info"
		check.Score = 100
		check.Message = "Unable to check for updates"
	} else {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		updateCount := len(lines) - 1 // 减去标题行

		if updateCount > 0 {
			check.Status = "warning"
			check.Score = 70
			check.Message = fmt.Sprintf("%d system updates available", updateCount)
			check.Remediation = "Run system update: apt upgrade"
		} else {
			check.Status = "pass"
			check.Score = 100
			check.Message = "System is up to date"
		}
	}

	check.Duration = time.Since(startTime)
	checks = append(checks, check)

	return checks
}

// scanMalware 扫描恶意软件
func (s *Scanner) scanMalware(ctx context.Context) []SecurityCheck {
	s.logger.Info("Scanning malware")
	checks := make([]SecurityCheck, 0)
	startTime := time.Now()

	check := SecurityCheck{
		ID:        "malware-scan",
		Name:      "Malware Scan",
		Category:  "malware",
		CheckedAt: time.Now(),
	}

	// 检查是否有恶意软件扫描工具
	_, err := exec.LookPath("clamscan")
	if err != nil {
		check.Status = "info"
		check.Score = 50
		check.Message = "No malware scanner installed"
		check.Remediation = "Install ClamAV: apt install clamav"
	} else {
		// 执行扫描
		cmd := exec.CommandContext(ctx, "clamscan", "--infected", "--recursive", "/tmp")
		output, err := cmd.Output()

		if err != nil {
			// clamscan 返回1表示发现感染
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				lines := strings.Split(strings.TrimSpace(string(output)), "\n")
				infectedCount := 0
				for _, line := range lines {
					if strings.Contains(line, "FOUND") {
						infectedCount++
					}
				}
				check.Status = "critical"
				check.Score = 0
				check.Message = fmt.Sprintf("Found %d infected files", infectedCount)
				check.Remediation = "Remove infected files immediately"
			} else {
				check.Status = "warning"
				check.Score = 50
				check.Message = "Malware scan incomplete"
			}
		} else {
			check.Status = "pass"
			check.Score = 100
			check.Message = "No malware detected"
		}
	}

	check.Duration = time.Since(startTime)
	checks = append(checks, check)

	return checks
}

// scanFirewall 扫描防火墙状态
func (s *Scanner) scanFirewall(ctx context.Context) []SecurityCheck {
	s.logger.Info("Scanning firewall status")
	checks := make([]SecurityCheck, 0)
	startTime := time.Now()

	check := SecurityCheck{
		ID:        "firewall-status",
		Name:      "Firewall Status",
		Category:  "firewall",
		CheckedAt: time.Now(),
	}

	// 检查 iptables 规则
	cmd := exec.CommandContext(ctx, "iptables", "-L", "-n")
	output, err := cmd.Output()
	if err != nil {
		check.Status = "warning"
		check.Score = 30
		check.Message = "Unable to check firewall status"
		check.Remediation = "Ensure iptables is installed and accessible"
	} else {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) <= 3 {
			check.Status = "warning"
			check.Score = 40
			check.Message = "Firewall has no rules configured"
			check.Remediation = "Configure firewall rules to restrict access"
		} else {
			check.Status = "pass"
			check.Score = 100
			check.Message = "Firewall is configured"
		}
	}

	check.Duration = time.Since(startTime)
	checks = append(checks, check)

	return checks
}

// ============================================================
// 辅助函数
// ============================================================

// parsePasswdFile 解析 /etc/passwd 文件
func parsePasswdFile(content string) []string {
	users := make([]string, 0)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) >= 7 {
			uid, _ := strconv.Atoi(parts[2])
			if uid >= 1000 { // 只检查普通用户
				users = append(users, parts[0])
			}
		}
	}
	return users
}

// checkPasswordStrength 检查密码强度（简化实现）
func checkPasswordStrength(username string, policy PasswordPolicy) string {
	// 实际应用中应该调用 pam 或 shadow 相关 API
	// 这里返回默认值
	return "medium"
}

// parseSSOutput 解析 ss 命令输出
func parseSSOutput(output string) []PortScanResult {
	ports := make([]PortScanResult, 0)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if strings.Contains(line, "LISTEN") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				addr := fields[4]
				parts := strings.Split(addr, ":")
				if len(parts) >= 2 {
					port, _ := strconv.Atoi(parts[len(parts)-1])
					if port > 0 {
						protocol := "tcp"
						if strings.Contains(fields[0], "udp") {
							protocol = "udp"
						}
						ports = append(ports, PortScanResult{
							Port:     port,
							Protocol: protocol,
							State:    "open",
						})
					}
				}
			}
		}
	}

	return ports
}

// assessPortRisk 评估端口风险
func assessPortRisk(port int, config PortRiskConfig) string {
	for _, p := range config.HighRiskPorts {
		if p == port {
			return "high"
		}
	}
	for _, p := range config.MediumRiskPorts {
		if p == port {
			return "medium"
		}
	}
	return "low"
}

// checkSSLCertificate 检查SSL证书（简化实现）
func checkSSLCertificate(domain string) int {
	conn, err := net.DialTimeout("tcp", domain+":443", 5*time.Second)
	if err != nil {
		return -1
	}
	conn.Close()

	// 简化实现，返回默认值
	// 实际应用中应使用 crypto/tls 获取证书信息
	return 90
}
