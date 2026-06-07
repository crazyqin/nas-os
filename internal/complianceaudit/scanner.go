// Package complianceaudit 提供安全扫描功能
package complianceaudit

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Scanner 安全扫描器
type Scanner struct {
	logger *zap.Logger
}

// NewScanner 创建安全扫描器
func NewScanner(logger *zap.Logger) *Scanner {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Scanner{logger: logger}
}

// ScanSystemConfig 系统安全配置审计
func (s *Scanner) ScanSystemConfig(ctx *CheckContext) *CheckResult {
	result := &CheckResult{
		Name:      "config_audit",
		Standard:  StandardMLPS2,
		Category:  CategoryAccessControl,
		Status:    StatusPass,
		RiskLevel: RiskLow,
		Details:   make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	issues := make([]string, 0)

	// 检查 SSH 配置
	if sshIssues := s.checkSSHConfig(); len(sshIssues) > 0 {
		issues = append(issues, sshIssues...)
		result.Details["ssh_issues"] = sshIssues
	}

	// 检查内核安全参数
	if kernelIssues := s.checkKernelSecurity(); len(kernelIssues) > 0 {
		issues = append(issues, kernelIssues...)
		result.Details["kernel_issues"] = kernelIssues
	}

	// 检查系统更新状态
	if updateIssues := s.checkSystemUpdates(); len(updateIssues) > 0 {
		issues = append(issues, updateIssues...)
		result.Details["update_issues"] = updateIssues
	}

	if len(issues) > 0 {
		result.Status = StatusFail
		result.RiskLevel = RiskHigh
		result.Message = fmt.Sprintf("发现 %d 个安全配置问题", len(issues))
	} else {
		result.Message = "系统安全配置正常"
	}

	return result
}

// checkSSHConfig 检查 SSH 配置安全性
func (s *Scanner) checkSSHConfig() []string {
	issues := make([]string, 0)
	configPath := "/etc/ssh/sshd_config"

	data, err := os.ReadFile(configPath)
	if err != nil {
		// 非 Linux 环境或无权限，跳过
		return issues
	}

	content := string(data)

	// 检查是否允许 root 登录
	if strings.Contains(content, "PermitRootLogin yes") {
		issues = append(issues, "SSH 允许 root 直接登录")
	}

	// 检查是否允许空密码
	if strings.Contains(content, "PermitEmptyPasswords yes") {
		issues = append(issues, "SSH 允许空密码登录")
	}

	// 检查协议版本
	if strings.Contains(content, "Protocol 1") {
		issues = append(issues, "SSH 使用不安全的协议版本1")
	}

	return issues
}

// checkKernelSecurity 检查内核安全参数
func (s *Scanner) checkKernelSecurity() []string {
	issues := make([]string, 0)

	if runtime.GOOS != "linux" {
		return issues
	}

	params := map[string]string{
		"/proc/sys/net/ipv4/ip_forward":                   "0",
		"/proc/sys/net/ipv4/conf/all/accept_redirects":    "0",
		"/proc/sys/net/ipv4/conf/all/send_redirects":      "0",
		"/proc/sys/net/ipv4/conf/all/accept_source_route": "0",
		"/proc/sys/net/ipv4/conf/all/log_martians":        "1",
	}

	for path, expected := range params {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		actual := strings.TrimSpace(string(data))
		if actual != expected {
			issues = append(issues, fmt.Sprintf("内核参数 %s 当前值 %s, 期望 %s", filepath.Base(path), actual, expected))
		}
	}

	return issues
}

// checkSystemUpdates 检查系统更新状态
func (s *Scanner) checkSystemUpdates() []string {
	issues := make([]string, 0)

	if runtime.GOOS != "linux" {
		return issues
	}

	// 尝试检查可用更新（不执行实际更新）
	cmd := exec.Command("apt", "list", "--upgradable")
	output, err := cmd.Output()
	if err != nil {
		return issues
	}

	lines := strings.Split(string(output), "\n")
	upgradable := 0
	for _, line := range lines[1:] { // 跳过标题行
		if strings.TrimSpace(line) != "" {
			upgradable++
		}
	}

	if upgradable > 10 {
		issues = append(issues, fmt.Sprintf("有 %d 个软件包可更新", upgradable))
	}

	return issues
}

// ScanFilePermissions 文件权限合规检查
func (s *Scanner) ScanFilePermissions(ctx *CheckContext) *CheckResult {
	result := &CheckResult{
		Name:      "file_permissions",
		Standard:  StandardMLPS2,
		Category:  CategoryAccessControl,
		Status:    StatusPass,
		RiskLevel: RiskLow,
		Details:   make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	issues := make([]string, 0)

	// 敏感目录及其期望权限
	sensitiveDirs := map[string]os.FileMode{
		"/etc":         0755,
		"/etc/ssh":     0700,
		"/etc/ssl":     0755,
		"/var/log":     0755,
		"/root":        0700,
		"/home":        0755,
		"/etc/crontab": 0600,
		"/etc/shadow":  0640,
		"/etc/passwd":  0644,
		"/etc/gshadow": 0640,
		"/etc/group":   0644,
	}

	for path, expected := range sensitiveDirs {
		info, err := os.Stat(path)
		if err != nil {
			continue // 跳过不存在的路径
		}

		actual := info.Mode().Perm()
		// 对于目录，检查权限是否过于宽松
		if info.IsDir() && actual > expected {
			issues = append(issues, fmt.Sprintf("%s 权限 %o 过于宽松 (期望 <= %o)", path, actual, expected))
		} else if !info.IsDir() && actual > expected {
			issues = append(issues, fmt.Sprintf("%s 权限 %o 过于宽松 (期望 <= %o)", path, actual, expected))
		}
	}

	// 检查 SUID/SGID 文件
	if suidIssues := s.checkSUIDFiles(); len(suidIssues) > 0 {
		issues = append(issues, suidIssues...)
		result.Details["suid_issues"] = suidIssues
	}

	if len(issues) > 0 {
		result.Status = StatusFail
		result.RiskLevel = RiskMedium
		result.Message = fmt.Sprintf("发现 %d 个文件权限问题", len(issues))
		result.Details["issues"] = issues
	} else {
		result.Message = "文件权限检查通过"
	}

	return result
}

// checkSUIDFiles 检查异常 SUID 文件
func (s *Scanner) checkSUIDFiles() []string {
	issues := make([]string, 0)

	if runtime.GOOS != "linux" {
		return issues
	}

	cmd := exec.Command("find", "/", "-perm", "-4000", "-type", "f", "-maxdepth", "4")
	output, err := cmd.Output()
	if err != nil {
		return issues
	}

	// 已知安全的 SUID 文件
	knownSafe := map[string]bool{
		"/usr/bin/passwd":              true,
		"/usr/bin/sudo":                true,
		"/usr/bin/su":                  true,
		"/usr/bin/newgrp":              true,
		"/usr/bin/chsh":                true,
		"/usr/bin/chfn":                true,
		"/usr/bin/gpasswd":             true,
		"/usr/bin/mount":               true,
		"/usr/bin/umount":              true,
		"/usr/bin/pkexec":              true,
		"/usr/lib/openssh/ssh-keysign": true,
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		path := strings.TrimSpace(scanner.Text())
		if path == "" {
			continue
		}
		if !knownSafe[path] {
			issues = append(issues, fmt.Sprintf("发现非标准 SUID 文件: %s", path))
		}
	}

	return issues
}

// ScanPasswordPolicy 用户密码策略审计
func (s *Scanner) ScanPasswordPolicy(ctx *CheckContext) *CheckResult {
	result := &CheckResult{
		Name:      "password_policy",
		Standard:  StandardGDPR,
		Category:  CategoryPasswordPolicy,
		Status:    StatusPass,
		RiskLevel: RiskLow,
		Details:   make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	issues := make([]string, 0)

	if runtime.GOOS != "linux" {
		result.Message = "密码策略检查仅支持 Linux"
		return result
	}

	// 检查 PAM 密码策略
	if pamIssues := s.checkPAMPolicy(); len(pamIssues) > 0 {
		issues = append(issues, pamIssues...)
		result.Details["pam_issues"] = pamIssues
	}

	// 检查 /etc/login.defs
	if loginIssues := s.checkLoginDefs(); len(loginIssues) > 0 {
		issues = append(issues, loginIssues...)
		result.Details["login_defs_issues"] = loginIssues
	}

	// 检查用户密码状态
	if pwdIssues := s.checkPasswordStatus(); len(pwdIssues) > 0 {
		issues = append(issues, pwdIssues...)
		result.Details["password_status_issues"] = pwdIssues
	}

	if len(issues) > 0 {
		result.Status = StatusFail
		result.RiskLevel = RiskHigh
		result.Message = fmt.Sprintf("发现 %d 个密码策略问题", len(issues))
	} else {
		result.Message = "密码策略符合要求"
	}

	return result
}

// checkPAMPolicy 检查 PAM 密码策略
func (s *Scanner) checkPAMPolicy() []string {
	issues := make([]string, 0)

	pamFiles := []string{
		"/etc/pam.d/common-password",
		"/etc/pam.d/system-auth",
		"/etc/pam.d/password-auth",
	}

	hasMinLen := false
	hasUpperLower := false
	hasDigit := false

	for _, f := range pamFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		content := string(data)

		if strings.Contains(content, "minlen=") {
			hasMinLen = true
			// 检查最小长度是否 >= 8
			for _, line := range strings.Split(content, "\n") {
				if strings.Contains(line, "minlen=") {
					parts := strings.Split(line, "minlen=")
					if len(parts) > 1 {
						val := strings.Fields(parts[1])[0]
						if n, err := strconv.Atoi(val); err == nil && n < 8 {
							issues = append(issues, fmt.Sprintf("密码最小长度 %d 不足 (建议 >= 8)", n))
						}
					}
				}
			}
		}

		if strings.Contains(content, "ucredit=") || strings.Contains(content, "lcredit=") {
			hasUpperLower = true
		}
		if strings.Contains(content, "dcredit=") {
			hasDigit = true
		}
	}

	if !hasMinLen {
		issues = append(issues, "未配置密码最小长度要求")
	}
	if !hasUpperLower {
		issues = append(issues, "未要求密码包含大小写字母")
	}
	if !hasDigit {
		issues = append(issues, "未要求密码包含数字")
	}

	return issues
}

// checkLoginDefs 检查 /etc/login.defs 密码策略
func (s *Scanner) checkLoginDefs() []string {
	issues := make([]string, 0)

	data, err := os.ReadFile("/etc/login.defs")
	if err != nil {
		return issues
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	config := make(map[string]string)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			config[parts[0]] = parts[1]
		}
	}

	// 检查密码有效期
	if maxDays, ok := config["PASS_MAX_DAYS"]; ok {
		if n, err := strconv.Atoi(maxDays); err == nil && n > 90 {
			issues = append(issues, fmt.Sprintf("密码最大有效期 %d 天过长 (建议 <= 90)", n))
		}
	}

	// 检查密码最小修改间隔
	if minDays, ok := config["PASS_MIN_DAYS"]; ok {
		if n, err := strconv.Atoi(minDays); err == nil && n < 1 {
			issues = append(issues, "密码最小修改间隔不足")
		}
	}

	// 检查密码过期警告
	if warnDays, ok := config["PASS_WARN_AGE"]; ok {
		if n, err := strconv.Atoi(warnDays); err == nil && n < 7 {
			issues = append(issues, "密码过期警告天数不足 (建议 >= 7)")
		}
	}

	return issues
}

// checkPasswordStatus 检查用户密码状态
func (s *Scanner) checkPasswordStatus() []string {
	issues := make([]string, 0)

	cmd := exec.Command("awk", "-F:", "{if ($2 == \"\" || $2 == \"!\") print $1}", "/etc/shadow")
	output, err := cmd.Output()
	if err != nil {
		return issues
	}

	users := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, u := range users {
		u = strings.TrimSpace(u)
		if u != "" {
			issues = append(issues, fmt.Sprintf("用户 %s 没有设置密码或密码已锁定", u))
		}
	}

	// 检查是否有空密码用户
	cmd2 := exec.Command("awk", "-F:", "{if ($2 == \"\") print $1}", "/etc/shadow")
	output2, err := cmd2.Output()
	if err != nil {
		return issues
	}

	emptyPwdUsers := strings.Split(strings.TrimSpace(string(output2)), "\n")
	for _, u := range emptyPwdUsers {
		u = strings.TrimSpace(u)
		if u != "" {
			issues = append(issues, fmt.Sprintf("用户 %s 使用空密码 (高风险)", u))
		}
	}

	return issues
}

// ScanNetworkExposure 网络服务暴露检测
func (s *Scanner) ScanNetworkExposure(ctx *CheckContext) *CheckResult {
	result := &CheckResult{
		Name:      "network_exposure",
		Standard:  StandardISO27001,
		Category:  CategoryNetworkSecurity,
		Status:    StatusPass,
		RiskLevel: RiskLow,
		Details:   make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	issues := make([]string, 0)

	if runtime.GOOS != "linux" {
		result.Message = "网络暴露检查仅支持 Linux"
		return result
	}

	// 高风险端口
	highRiskPorts := map[int]string{
		21:    "FTP",
		23:    "Telnet",
		25:    "SMTP (未加密)",
		135:   "RPC",
		139:   "NetBIOS",
		445:   "SMB",
		1433:  "MSSQL",
		3306:  "MySQL",
		3389:  "RDP",
		5432:  "PostgreSQL",
		5900:  "VNC",
		6379:  "Redis",
		27017: "MongoDB",
	}

	// 检查监听端口
	cmd := exec.Command("ss", "-tlnp")
	output, err := cmd.Output()
	if err != nil {
		// 备用方案
		cmd = exec.Command("netstat", "-tlnp")
		output, err = cmd.Output()
		if err != nil {
			result.Message = "无法获取网络监听信息"
			return result
		}
	}

	exposedServices := make(map[int]string)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		for port, service := range highRiskPorts {
			if strings.Contains(line, fmt.Sprintf(":%d ", port)) || strings.Contains(line, fmt.Sprintf(":%d\t", port)) {
				// 检查是否只监听本地
				if !strings.Contains(line, "127.0.0.1") && !strings.Contains(line, "::1") {
					exposedServices[port] = service
				}
			}
		}
	}

	for port, service := range exposedServices {
		issues = append(issues, fmt.Sprintf("端口 %d (%s) 对外暴露", port, service))
	}

	result.Details["exposed_services"] = exposedServices

	// 检查防火墙状态
	if fwIssues := s.checkFirewall(); len(fwIssues) > 0 {
		issues = append(issues, fwIssues...)
		result.Details["firewall_issues"] = fwIssues
	}

	if len(issues) > 0 {
		result.Status = StatusFail
		result.RiskLevel = RiskHigh
		result.Message = fmt.Sprintf("发现 %d 个网络安全问题", len(issues))
	} else {
		result.Message = "网络暴露检查通过"
	}

	return result
}

// checkFirewall 检查防火墙状态
func (s *Scanner) checkFirewall() []string {
	issues := make([]string, 0)

	// 检查 iptables 是否有规则
	cmd := exec.Command("iptables", "-L", "-n")
	output, err := cmd.Output()
	if err != nil {
		return issues
	}

	content := string(output)
	if strings.Contains(content, "Chain INPUT (policy ACCEPT)") {
		// 检查是否有自定义规则
		lines := strings.Split(content, "\n")
		ruleCount := 0
		inInput := false
		for _, line := range lines {
			if strings.HasPrefix(line, "Chain INPUT") {
				inInput = true
				continue
			}
			if strings.HasPrefix(line, "Chain ") {
				inInput = false
				continue
			}
			if inInput && strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "target") {
				ruleCount++
			}
		}
		if ruleCount == 0 {
			issues = append(issues, "防火墙 INPUT 链策略为 ACCEPT 且无规则")
		}
	}

	return issues
}

// ScanEncryptionStatus 数据加密状态检查
func (s *Scanner) ScanEncryptionStatus(ctx *CheckContext) *CheckResult {
	result := &CheckResult{
		Name:      "encryption_status",
		Standard:  StandardGDPR,
		Category:  CategoryEncryption,
		Status:    StatusPass,
		RiskLevel: RiskLow,
		Details:   make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	issues := make([]string, 0)

	if runtime.GOOS != "linux" {
		result.Message = "加密状态检查仅支持 Linux"
		return result
	}

	// 检查磁盘加密
	if diskIssues := s.checkDiskEncryption(); len(diskIssues) > 0 {
		issues = append(issues, diskIssues...)
		result.Details["disk_encryption_issues"] = diskIssues
	}

	// 检查 TLS/SSL 配置
	if tlsIssues := s.checkTLSConfig(); len(tlsIssues) > 0 {
		issues = append(issues, tlsIssues...)
		result.Details["tls_issues"] = tlsIssues
	}

	// 检查 SSH 加密算法
	if sshIssues := s.checkSSHEncryption(); len(sshIssues) > 0 {
		issues = append(issues, sshIssues...)
		result.Details["ssh_encryption_issues"] = sshIssues
	}

	if len(issues) > 0 {
		result.Status = StatusFail
		result.RiskLevel = RiskMedium
		result.Message = fmt.Sprintf("发现 %d 个加密相关问题", len(issues))
	} else {
		result.Message = "加密状态检查通过"
	}

	return result
}

// checkDiskEncryption 检查磁盘加密状态
func (s *Scanner) checkDiskEncryption() []string {
	issues := make([]string, 0)

	// 检查是否有 LUKS 加密分区
	cmd := exec.Command("lsblk", "-o", "NAME,TYPE,FSTYPE,MOUNTPOINT")
	output, err := cmd.Output()
	if err != nil {
		return issues
	}

	hasCrypto := strings.Contains(string(output), "crypto_LUKS")
	if !hasCrypto {
		issues = append(issues, "未检测到 LUKS 磁盘加密")
	}

	return issues
}

// checkTLSConfig 检查 TLS 配置
func (s *Scanner) checkTLSConfig() []string {
	issues := make([]string, 0)

	// 检查是否禁用了不安全的 SSL/TLS 版本
	cmd := exec.Command("openssl", "ciphers", "-v", "ALL")
	output, err := cmd.Output()
	if err != nil {
		return issues
	}

	content := string(output)
	if strings.Contains(content, "SSLv3") || strings.Contains(content, "TLSv1.0") {
		issues = append(issues, "系统支持不安全的 SSL/TLS 版本")
	}

	return issues
}

// checkSSHEncryption 检查 SSH 加密算法
func (s *Scanner) checkSSHEncryption() []string {
	issues := make([]string, 0)

	data, err := os.ReadFile("/etc/ssh/sshd_config")
	if err != nil {
		return issues
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}

		// 检查是否使用了弱加密算法
		if strings.HasPrefix(line, "Ciphers") {
			if strings.Contains(line, "arcfour") || strings.Contains(line, "3des") {
				issues = append(issues, "SSH 配置中包含弱加密算法")
			}
		}

		if strings.HasPrefix(line, "MACs") {
			if strings.Contains(line, "hmac-md5") || strings.Contains(line, "hmac-sha1") {
				issues = append(issues, "SSH 配置中包含弱 MAC 算法")
			}
		}
	}

	return issues
}

// ScanDataProtection GDPR 数据保护合规检查
func (s *Scanner) ScanDataProtection(ctx *CheckContext) *CheckResult {
	result := &CheckResult{
		Name:      "gdpr_data_protection",
		Standard:  StandardGDPR,
		Category:  CategoryDataProtection,
		Status:    StatusPass,
		RiskLevel: RiskLow,
		Details:   make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	issues := make([]string, 0)

	// 检查数据保留策略
	if retentionIssues := s.checkDataRetention(); len(retentionIssues) > 0 {
		issues = append(issues, retentionIssues...)
		result.Details["retention_issues"] = retentionIssues
	}

	// 检查日志记录
	if logIssues := s.checkAuditLogging(); len(logIssues) > 0 {
		issues = append(issues, logIssues...)
		result.Details["logging_issues"] = logIssues
	}

	// 检查访问控制
	if accessIssues := s.checkAccessControl(); len(accessIssues) > 0 {
		issues = append(issues, accessIssues...)
		result.Details["access_issues"] = accessIssues
	}

	if len(issues) > 0 {
		result.Status = StatusFail
		result.RiskLevel = RiskMedium
		result.Message = fmt.Sprintf("发现 %d 个数据保护问题", len(issues))
	} else {
		result.Message = "数据保护检查通过"
	}

	return result
}

// checkDataRetention 检查数据保留策略
func (s *Scanner) checkDataRetention() []string {
	issues := make([]string, 0)

	// 检查日志轮转配置
	logrotateConf := "/etc/logrotate.conf"
	data, err := os.ReadFile(logrotateConf)
	if err != nil {
		return issues
	}

	content := string(data)
	if !strings.Contains(content, "rotate") {
		issues = append(issues, "日志轮转未配置保留策略")
	}

	return issues
}

// checkAuditLogging 检查审计日志配置
func (s *Scanner) checkAuditLogging() []string {
	issues := make([]string, 0)

	if runtime.GOOS != "linux" {
		return issues
	}

	// 检查 auditd 是否运行
	cmd := exec.Command("systemctl", "is-active", "auditd")
	output, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) != "active" {
		issues = append(issues, "审计服务 (auditd) 未运行")
	}

	// 检查 rsyslog 是否运行
	cmd = exec.Command("systemctl", "is-active", "rsyslog")
	output, err = cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) != "active" {
		// 也检查 syslog-ng
		cmd = exec.Command("systemctl", "is-active", "syslog-ng")
		output, err = cmd.Output()
		if err != nil || strings.TrimSpace(string(output)) != "active" {
			issues = append(issues, "系统日志服务未运行")
		}
	}

	return issues
}

// checkAccessControl 检查访问控制
func (s *Scanner) checkAccessControl() []string {
	issues := make([]string, 0)

	if runtime.GOOS != "linux" {
		return issues
	}

	// 检查是否有 uid=0 的非 root 用户
	cmd := exec.Command("awk", "-F:", "{if ($3 == 0 && $1 != \"root\") print $1}", "/etc/passwd")
	output, err := cmd.Output()
	if err != nil {
		return issues
	}

	users := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, u := range users {
		u = strings.TrimSpace(u)
		if u != "" {
			issues = append(issues, fmt.Sprintf("用户 %s 拥有 root 权限 (uid=0)", u))
		}
	}

	return issues
}

// ScanAuditLog 等保2.0 审计日志合规检查
func (s *Scanner) ScanAuditLog(ctx *CheckContext) *CheckResult {
	result := &CheckResult{
		Name:      "mlps2_audit_log",
		Standard:  StandardMLPS2,
		Category:  CategoryAuditLog,
		Status:    StatusPass,
		RiskLevel: RiskLow,
		Details:   make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	issues := make([]string, 0)

	if runtime.GOOS != "linux" {
		result.Message = "审计日志检查仅支持 Linux"
		return result
	}

	// 检查日志文件权限
	logFiles := []string{
		"/var/log/syslog",
		"/var/log/auth.log",
		"/var/log/audit/audit.log",
		"/var/log/messages",
	}

	for _, f := range logFiles {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		perm := info.Mode().Perm()
		if perm > 0640 {
			issues = append(issues, fmt.Sprintf("日志文件 %s 权限 %o 过于宽松", f, perm))
		}
	}

	// 检查日志完整性保护
	if integrityIssues := s.checkLogIntegrity(); len(integrityIssues) > 0 {
		issues = append(issues, integrityIssues...)
		result.Details["integrity_issues"] = integrityIssues
	}

	// 检查日志保留期
	if retentionIssues := s.checkLogRetention(); len(retentionIssues) > 0 {
		issues = append(issues, retentionIssues...)
		result.Details["retention_issues"] = retentionIssues
	}

	if len(issues) > 0 {
		result.Status = StatusFail
		result.RiskLevel = RiskMedium
		result.Message = fmt.Sprintf("发现 %d 个审计日志问题", len(issues))
	} else {
		result.Message = "审计日志检查通过"
	}

	return result
}

// checkLogIntegrity 检查日志完整性保护
func (s *Scanner) checkLogIntegrity() []string {
	issues := make([]string, 0)

	// 检查是否有远程日志服务器配置
	rsyslogConf := "/etc/rsyslog.conf"
	data, err := os.ReadFile(rsyslogConf)
	if err != nil {
		return issues
	}

	content := string(data)
	hasRemote := false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		// 检查远程日志转发
		if strings.Contains(line, "@@") || strings.Contains(line, "@") {
			if strings.Contains(line, "action") || strings.Contains(line, "*.*") {
				hasRemote = true
				break
			}
		}
	}

	if !hasRemote {
		issues = append(issues, "未配置远程日志服务器 (建议配置以防本地篡改)")
	}

	return issues
}

// checkLogRetention 检查日志保留配置
func (s *Scanner) checkLogRetention() []string {
	issues := make([]string, 0)

	logrotateConf := "/etc/logrotate.conf"
	data, err := os.ReadFile(logrotateConf)
	if err != nil {
		return issues
	}

	content := string(data)
	// 等保2.0 要求日志保留至少 6 个月
	if strings.Contains(content, "rotate") {
		// 简单检查
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			if strings.Contains(line, "rotate") && !strings.HasPrefix(line, "#") {
				parts := strings.Fields(line)
				for i, p := range parts {
					if p == "rotate" && i+1 < len(parts) {
						n, err := strconv.Atoi(parts[i+1])
						if err == nil && n < 26 { // 周轮转26次约6个月
							issues = append(issues, fmt.Sprintf("日志保留周期可能不足6个月 (rotate %d)", n))
						}
					}
				}
			}
		}
	}

	return issues
}
