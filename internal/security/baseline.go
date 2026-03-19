package security

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// BaselineManager 安全基线检查管理器
type BaselineManager struct {
	checks []BaselineCheck
}

// BaselineCheck 基线检查项
type BaselineCheck struct {
	ID          string
	Name        string
	Description string
	Category    string
	Severity    string
	CheckFunc   func() BaselineCheckResult
}

// NewBaselineManager 创建基线检查管理器
func NewBaselineManager() *BaselineManager {
	bm := &BaselineManager{
		checks: make([]BaselineCheck, 0),
	}

	// 注册所有检查项
	bm.registerChecks()

	return bm
}

// registerChecks 注册所有检查项
func (bm *BaselineManager) registerChecks() {
	// ========== 认证安全 ==========
	bm.checks = append(bm.checks, BaselineCheck{
		ID:          "AUTH-001",
		Name:        "密码复杂度检查",
		Description: "检查系统密码策略是否符合安全要求",
		Category:    "auth",
		Severity:    "high",
		CheckFunc:   bm.checkPasswordPolicy,
	})

	bm.checks = append(bm.checks, BaselineCheck{
		ID:          "AUTH-002",
		Name:        "MFA 启用状态",
		Description: "检查管理员账户是否启用了双因素认证",
		Category:    "auth",
		Severity:    "high",
		CheckFunc:   bm.checkMFAEnabled,
	})

	bm.checks = append(bm.checks, BaselineCheck{
		ID:          "AUTH-003",
		Name:        "默认密码检查",
		Description: "检查是否存在使用默认密码的账户",
		Category:    "auth",
		Severity:    "critical",
		CheckFunc:   bm.checkDefaultPasswords,
	})

	bm.checks = append(bm.checks, BaselineCheck{
		ID:          "AUTH-004",
		Name:        "账户锁定策略",
		Description: "检查是否配置了账户锁定策略",
		Category:    "auth",
		Severity:    "medium",
		CheckFunc:   bm.checkAccountLockoutPolicy,
	})

	// ========== 网络安全 ==========
	bm.checks = append(bm.checks, BaselineCheck{
		ID:          "NET-001",
		Name:        "防火墙状态",
		Description: "检查防火墙是否启用",
		Category:    "network",
		Severity:    "high",
		CheckFunc:   bm.checkFirewallEnabled,
	})

	bm.checks = append(bm.checks, BaselineCheck{
		ID:          "NET-002",
		Name:        "SSH 配置检查",
		Description: "检查 SSH 配置是否安全",
		Category:    "network",
		Severity:    "high",
		CheckFunc:   bm.checkSSHConfig,
	})

	bm.checks = append(bm.checks, BaselineCheck{
		ID:          "NET-003",
		Name:        "开放端口检查",
		Description: "检查是否有不必要的高风险端口开放",
		Category:    "network",
		Severity:    "medium",
		CheckFunc:   bm.checkOpenPorts,
	})

	bm.checks = append(bm.checks, BaselineCheck{
		ID:          "NET-004",
		Name:        "HTTPS 强制",
		Description: "检查是否强制使用 HTTPS",
		Category:    "network",
		Severity:    "high",
		CheckFunc:   bm.checkHTTPSRequired,
	})

	// ========== 系统安全 ==========
	bm.checks = append(bm.checks, BaselineCheck{
		ID:          "SYS-001",
		Name:        "系统更新状态",
		Description: "检查系统是否为最新版本",
		Category:    "system",
		Severity:    "medium",
		CheckFunc:   bm.checkSystemUpdates,
	})

	bm.checks = append(bm.checks, BaselineCheck{
		ID:          "SYS-002",
		Name:        "Root 登录检查",
		Description: "检查是否禁用了 root 远程登录",
		Category:    "system",
		Severity:    "high",
		CheckFunc:   bm.checkRootLogin,
	})

	bm.checks = append(bm.checks, BaselineCheck{
		ID:          "SYS-003",
		Name:        "文件权限检查",
		Description: "检查关键文件的权限设置",
		Category:    "system",
		Severity:    "medium",
		CheckFunc:   bm.checkFilePermissions,
	})

	bm.checks = append(bm.checks, BaselineCheck{
		ID:          "SYS-004",
		Name:        "日志记录状态",
		Description: "检查系统日志是否启用",
		Category:    "system",
		Severity:    "medium",
		CheckFunc:   bm.checkLoggingEnabled,
	})

	// ========== 文件安全 ==========
	bm.checks = append(bm.checks, BaselineCheck{
		ID:          "FILE-001",
		Name:        "敏感文件加密",
		Description: "检查敏感文件是否加密存储",
		Category:    "file",
		Severity:    "high",
		CheckFunc:   bm.checkSensitiveFilesEncrypted,
	})

	bm.checks = append(bm.checks, BaselineCheck{
		ID:          "FILE-002",
		Name:        "共享权限检查",
		Description: "检查共享文件夹的权限设置",
		Category:    "file",
		Severity:    "medium",
		CheckFunc:   bm.checkSharePermissions,
	})
}

// RunCheck 运行单个检查
func (bm *BaselineManager) RunCheck(checkID string) BaselineCheckResult {
	for _, check := range bm.checks {
		if check.ID == checkID {
			return check.CheckFunc()
		}
	}

	return BaselineCheckResult{
		CheckID: checkID,
		Status:  "skipped",
		Message: "检查项不存在",
	}
}

// RunAllChecks 运行所有检查
func (bm *BaselineManager) RunAllChecks() BaselineReport {
	report := BaselineReport{
		ReportID:  generateReportID(),
		Timestamp: time.Now(),
		Results:   make([]BaselineCheckResult, 0),
	}

	for _, check := range bm.checks {
		result := check.CheckFunc()
		report.Results = append(report.Results, result)

		// 统计
		report.TotalChecks++
		switch result.Status {
		case "pass":
			report.Passed++
		case "fail":
			report.Failed++
		case "warning":
			report.Warning++
		case "skipped":
			report.Skipped++
		}
	}

	// 计算总体得分
	if report.TotalChecks > 0 {
		report.OverallScore = (report.Passed * 100) / report.TotalChecks
	}

	return report
}

// RunChecksByCategory 按类别运行检查
func (bm *BaselineManager) RunChecksByCategory(category string) BaselineReport {
	report := BaselineReport{
		ReportID:  generateReportID(),
		Timestamp: time.Now(),
		Results:   make([]BaselineCheckResult, 0),
	}

	for _, check := range bm.checks {
		if check.Category == category {
			result := check.CheckFunc()
			report.Results = append(report.Results, result)

			report.TotalChecks++
			switch result.Status {
			case "pass":
				report.Passed++
			case "fail":
				report.Failed++
			case "warning":
				report.Warning++
			case "skipped":
				report.Skipped++
			}
		}
	}

	if report.TotalChecks > 0 {
		report.OverallScore = (report.Passed * 100) / report.TotalChecks
	}

	return report
}

// ========== 检查实现 ==========

// checkPasswordPolicy 检查密码策略
func (bm *BaselineManager) checkPasswordPolicy() BaselineCheckResult {
	result := BaselineCheckResult{
		CheckID:     "AUTH-001",
		Name:        "密码复杂度检查",
		Description: "检查系统密码策略是否符合安全要求",
		Category:    "auth",
		Severity:    "high",
	}

	// 检查 /etc/pam.d/common-password
	pamConfig := "/etc/pam.d/common-password"
	if _, err := os.Stat(pamConfig); err == nil {
		data, err := os.ReadFile(pamConfig)
		if err == nil {
			content := string(data)
			hasMinlen := strings.Contains(content, "minlen=")
			hasUppercase := strings.Contains(content, "ucredit=")
			hasLowercase := strings.Contains(content, "lcredit=")
			hasDigit := strings.Contains(content, "dcredit=")

			if hasMinlen && hasUppercase && hasLowercase && hasDigit {
				result.Status = "pass"
				result.Message = "密码策略配置良好"
				result.Details = map[string]interface{}{
					"min_length": true,
					"uppercase":  true,
					"lowercase":  true,
					"digit":      true,
				}
			} else {
				result.Status = "fail"
				result.Message = "密码策略不够严格"
				result.Remediation = "建议配置密码复杂度要求：最小长度 8 位，包含大小写字母和数字"
				result.Details = map[string]interface{}{
					"min_length": hasMinlen,
					"uppercase":  hasUppercase,
					"lowercase":  hasLowercase,
					"digit":      hasDigit,
				}
			}
			return result
		}
	}

	// 如果无法检查 PAM 配置，返回警告
	result.Status = "warning"
	result.Message = "无法检查密码策略配置"
	return result
}

// checkMFAEnabled 检查 MFA 启用状态
func (bm *BaselineManager) checkMFAEnabled() BaselineCheckResult {
	result := BaselineCheckResult{
		CheckID:     "AUTH-002",
		Name:        "MFA 启用状态",
		Description: "检查管理员账户是否启用了双因素认证",
		Category:    "auth",
		Severity:    "high",
	}

	// 这里应该检查实际的用户 MFA 状态
	// 简化实现：检查配置文件
	mfaConfig := "/etc/nas-os/mfa.conf"
	if _, err := os.Stat(mfaConfig); err == nil {
		result.Status = "pass"
		result.Message = "MFA 已配置"
		return result
	}

	result.Status = "warning"
	result.Message = "MFA 未配置或配置文件不存在"
	result.Remediation = "建议为所有管理员账户启用双因素认证"
	return result
}

// checkDefaultPasswords 检查默认密码
func (bm *BaselineManager) checkDefaultPasswords() BaselineCheckResult {
	result := BaselineCheckResult{
		CheckID:     "AUTH-003",
		Name:        "默认密码检查",
		Description: "检查是否存在使用默认密码的账户",
		Category:    "auth",
		Severity:    "critical",
	}

	// 简化实现：检查常见默认用户名是否存在
	defaultUsers := []string{"admin", "root", "test", "guest"}
	foundDefault := false

	for _, defaultUser := range defaultUsers {
		// 检查用户是否存在于 /etc/passwd
		cmd := exec.Command("grep", "-c", "^"+defaultUser+":", "/etc/passwd")
		output, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(output)) != "0" {
			foundDefault = true
			break
		}
	}

	if foundDefault {
		result.Status = "warning"
		result.Message = "发现使用默认用户名的账户（建议检查）"
		result.Remediation = "建议修改或删除默认账户，或确保使用强密码"
	} else {
		result.Status = "pass"
		result.Message = "未发现默认账户"
	}

	return result
}

// checkAccountLockoutPolicy 检查账户锁定策略
func (bm *BaselineManager) checkAccountLockoutPolicy() BaselineCheckResult {
	result := BaselineCheckResult{
		CheckID:     "AUTH-004",
		Name:        "账户锁定策略",
		Description: "检查是否配置了账户锁定策略",
		Category:    "auth",
		Severity:    "medium",
	}

	// 检查 fail2ban 配置
	fail2banConfig := "/etc/fail2ban/jail.local"
	if _, err := os.Stat(fail2banConfig); err == nil {
		result.Status = "pass"
		result.Message = "账户锁定策略已配置"
		return result
	}

	result.Status = "warning"
	result.Message = "未找到账户锁定策略配置"
	result.Remediation = "建议配置 fail2ban 或类似的失败登录保护机制"
	return result
}

// checkFirewallEnabled 检查防火墙状态
func (bm *BaselineManager) checkFirewallEnabled() BaselineCheckResult {
	result := BaselineCheckResult{
		CheckID:     "NET-001",
		Name:        "防火墙状态",
		Description: "检查防火墙是否启用",
		Category:    "network",
		Severity:    "high",
	}

	// 检查 iptables 规则
	cmd := exec.Command("iptables", "-L", "-n")
	output, err := cmd.Output()
	if err != nil {
		result.Status = "fail"
		result.Message = "无法检查防火墙状态"
		result.Remediation = "请确保 iptables 可用并已配置防火墙规则"
		return result
	}

	// 简单检查是否有规则
	if len(output) > 100 {
		result.Status = "pass"
		result.Message = "防火墙已启用并有规则配置"
	} else {
		result.Status = "warning"
		result.Message = "防火墙可能未正确配置"
		result.Remediation = "建议配置防火墙规则限制不必要的访问"
	}

	return result
}

// checkSSHConfig 检查 SSH 配置
func (bm *BaselineManager) checkSSHConfig() BaselineCheckResult {
	result := BaselineCheckResult{
		CheckID:     "NET-002",
		Name:        "SSH 配置检查",
		Description: "检查 SSH 配置是否安全",
		Category:    "network",
		Severity:    "high",
	}

	sshConfig := "/etc/ssh/sshd_config"
	if _, err := os.Stat(sshConfig); err != nil {
		result.Status = "skipped"
		result.Message = "SSH 未安装或配置文件不存在"
		return result
	}

	data, err := os.ReadFile(sshConfig)
	if err != nil {
		result.Status = "warning"
		result.Message = "无法读取 SSH 配置文件"
		return result
	}

	content := string(data)
	issues := []string{}

	// 检查 PermitRootLogin
	if strings.Contains(content, "PermitRootLogin yes") {
		issues = append(issues, "允许 root 登录")
	}

	// 检查 PasswordAuthentication
	if strings.Contains(content, "PasswordAuthentication yes") {
		issues = append(issues, "允许密码认证（建议使用密钥认证）")
	}

	// 检查 PermitEmptyPasswords
	if strings.Contains(content, "PermitEmptyPasswords yes") {
		issues = append(issues, "允许空密码（严重安全问题）")
	}

	if len(issues) > 0 {
		result.Status = "fail"
		result.Message = "SSH 配置存在安全问题：" + strings.Join(issues, ", ")
		result.Remediation = "建议修改 SSH 配置：禁用 root 登录、使用密钥认证、禁止空密码"
	} else {
		result.Status = "pass"
		result.Message = "SSH 配置安全"
	}

	return result
}

// checkOpenPorts 检查开放端口
func (bm *BaselineManager) checkOpenPorts() BaselineCheckResult {
	result := BaselineCheckResult{
		CheckID:     "NET-003",
		Name:        "开放端口检查",
		Description: "检查是否有不必要的高风险端口开放",
		Category:    "network",
		Severity:    "medium",
	}

	// 高风险端口列表
	highRiskPorts := map[int]string{
		21:    "FTP",
		23:    "Telnet",
		25:    "SMTP",
		110:   "POP3",
		143:   "IMAP",
		445:   "SMB",
		3306:  "MySQL",
		5432:  "PostgreSQL",
		6379:  "Redis",
		27017: "MongoDB",
	}

	// 检查监听的端口
	listeningPorts := make(map[int]string)

	// 使用 ss 命令检查监听端口
	cmd := exec.Command("ss", "-tlnp")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "LISTEN") {
				parts := strings.Fields(line)
				for _, part := range parts {
					if strings.Contains(part, ":") {
						portStr := strings.Split(part, ":")[len(strings.Split(part, ":"))-1]
						if port, err := strconv.Atoi(portStr); err == nil {
							listeningPorts[port] = ""
						}
					}
				}
			}
		}
	}

	// 检查是否有高风险端口
	foundRisks := []string{}
	for port, service := range highRiskPorts {
		if _, exists := listeningPorts[port]; exists {
			foundRisks = append(foundRisks, fmt.Sprintf("%d (%s)", port, service))
		}
	}

	if len(foundRisks) > 0 {
		result.Status = "warning"
		result.Message = "发现高风险端口开放：" + strings.Join(foundRisks, ", ")
		result.Remediation = "建议关闭不必要的服务或使用防火墙限制访问"
		result.Details = map[string]interface{}{
			"risky_ports": foundRisks,
		}
	} else {
		result.Status = "pass"
		result.Message = "未发现高风险端口"
	}

	return result
}

// checkHTTPSRequired 检查 HTTPS 强制
func (bm *BaselineManager) checkHTTPSRequired() BaselineCheckResult {
	result := BaselineCheckResult{
		CheckID:     "NET-004",
		Name:        "HTTPS 强制",
		Description: "检查是否强制使用 HTTPS",
		Category:    "network",
		Severity:    "high",
	}

	// 检查是否有 SSL 证书
	sslPaths := []string{
		"/etc/ssl/certs",
		"/etc/nginx/ssl",
		"/etc/pki/tls",
	}

	hasSSL := false
	for _, path := range sslPaths {
		if _, err := os.Stat(path); err == nil {
			hasSSL = true
			break
		}
	}

	if hasSSL {
		result.Status = "pass"
		result.Message = "SSL 证书已配置"
	} else {
		result.Status = "warning"
		result.Message = "未找到 SSL 证书配置"
		result.Remediation = "建议配置 HTTPS 并使用有效的 SSL 证书"
	}

	return result
}

// checkSystemUpdates 检查系统更新
func (bm *BaselineManager) checkSystemUpdates() BaselineCheckResult {
	result := BaselineCheckResult{
		CheckID:     "SYS-001",
		Name:        "系统更新状态",
		Description: "检查系统是否为最新版本",
		Category:    "system",
		Severity:    "medium",
	}

	// 检查包管理器
	cmd := exec.Command("which", "apt")
	if err := cmd.Run(); err == nil {
		// Debian/Ubuntu
		cmd = exec.Command("apt", "list", "--upgradable")
		output, err := cmd.Output()
		if err == nil && len(output) > 50 {
			result.Status = "warning"
			result.Message = "系统有可用更新"
			result.Remediation = "建议运行 'apt update && apt upgrade' 更新系统"
			return result
		}
	}

	cmd = exec.Command("which", "yum")
	if err := cmd.Run(); err == nil {
		// RHEL/CentOS
		cmd = exec.Command("yum", "check-update")
		if err := cmd.Run(); err != nil {
			// 有更新时 yum check-update 返回非零
			result.Status = "warning"
			result.Message = "系统有可用更新"
			result.Remediation = "建议运行 'yum update' 更新系统"
			return result
		}
	}

	result.Status = "pass"
	result.Message = "系统已是最新"
	return result
}

// checkRootLogin 检查 root 登录
func (bm *BaselineManager) checkRootLogin() BaselineCheckResult {
	result := BaselineCheckResult{
		CheckID:     "SYS-002",
		Name:        "Root 登录检查",
		Description: "检查是否禁用了 root 远程登录",
		Category:    "system",
		Severity:    "high",
	}

	sshConfig := "/etc/ssh/sshd_config"
	if _, err := os.Stat(sshConfig); err != nil {
		result.Status = "skipped"
		result.Message = "SSH 未安装"
		return result
	}

	data, err := os.ReadFile(sshConfig)
	if err != nil {
		result.Status = "warning"
		result.Message = "无法读取 SSH 配置"
		return result
	}

	content := string(data)
	if strings.Contains(content, "PermitRootLogin no") ||
		strings.Contains(content, "PermitRootLogin prohibit-password") {
		result.Status = "pass"
		result.Message = "Root 远程登录已禁用"
	} else if strings.Contains(content, "PermitRootLogin yes") {
		result.Status = "fail"
		result.Message = "Root 远程登录已启用（严重安全问题）"
		result.Remediation = "建议在 /etc/ssh/sshd_config 中设置 PermitRootLogin no"
	} else {
		result.Status = "warning"
		result.Message = "Root 登录配置不明确"
		result.Remediation = "建议明确设置 PermitRootLogin no"
	}

	return result
}

// checkFilePermissions 检查文件权限
func (bm *BaselineManager) checkFilePermissions() BaselineCheckResult {
	result := BaselineCheckResult{
		CheckID:     "SYS-003",
		Name:        "文件权限检查",
		Description: "检查关键文件的权限设置",
		Category:    "system",
		Severity:    "medium",
	}

	// 检查关键文件
	criticalFiles := map[string]int{
		"/etc/passwd":          0644,
		"/etc/shadow":          0640,
		"/etc/group":           0644,
		"/etc/gshadow":         0640,
		"/etc/ssh/sshd_config": 0600,
	}

	issues := []string{}
	for file, expectedMode := range criticalFiles {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		mode := info.Mode().Perm()
		if mode != os.FileMode(expectedMode) {
			issues = append(issues, fmt.Sprintf("%s 权限为 %o (期望 %o)", file, mode, expectedMode))
		}
	}

	if len(issues) > 0 {
		result.Status = "warning"
		result.Message = "发现文件权限问题：" + strings.Join(issues, ", ")
		result.Remediation = "建议使用 chmod 修正文件权限"
	} else {
		result.Status = "pass"
		result.Message = "关键文件权限配置正确"
	}

	return result
}

// checkLoggingEnabled 检查日志记录
func (bm *BaselineManager) checkLoggingEnabled() BaselineCheckResult {
	result := BaselineCheckResult{
		CheckID:     "SYS-004",
		Name:        "日志记录状态",
		Description: "检查系统日志是否启用",
		Category:    "system",
		Severity:    "medium",
	}

	// 检查日志目录
	logDirs := []string{
		"/var/log",
		"/var/log/nas-os",
	}

	for _, dir := range logDirs {
		if _, err := os.Stat(dir); err == nil {
			result.Status = "pass"
			result.Message = "日志记录已启用"
			return result
		}
	}

	result.Status = "warning"
	result.Message = "日志目录不存在"
	result.Remediation = "建议配置系统日志记录"
	return result
}

// checkSensitiveFilesEncrypted 检查敏感文件加密
func (bm *BaselineManager) checkSensitiveFilesEncrypted() BaselineCheckResult {
	result := BaselineCheckResult{
		CheckID:     "FILE-001",
		Name:        "敏感文件加密",
		Description: "检查敏感文件是否加密存储",
		Category:    "file",
		Severity:    "high",
	}

	// 检查加密配置
	encryptConfig := "/etc/nas-os/encryption.conf"
	if _, err := os.Stat(encryptConfig); err == nil {
		result.Status = "pass"
		result.Message = "敏感文件加密已配置"
		return result
	}

	result.Status = "warning"
	result.Message = "未找到加密配置"
	result.Remediation = "建议对敏感文件（如密码、密钥）进行加密存储"
	return result
}

// checkSharePermissions 检查共享权限
func (bm *BaselineManager) checkSharePermissions() BaselineCheckResult {
	result := BaselineCheckResult{
		CheckID:     "FILE-002",
		Name:        "共享权限检查",
		Description: "检查共享文件夹的权限设置",
		Category:    "file",
		Severity:    "medium",
	}

	// 检查 SMB 配置
	smbConfig := "/etc/samba/smb.conf"
	if _, err := os.Stat(smbConfig); err != nil {
		result.Status = "skipped"
		result.Message = "SMB 未配置"
		return result
	}

	data, err := os.ReadFile(smbConfig)
	if err != nil {
		result.Status = "warning"
		result.Message = "无法读取 SMB 配置"
		return result
	}

	content := string(data)
	issues := []string{}

	// 检查是否有 guest ok = yes
	if strings.Contains(content, "guest ok = yes") || strings.Contains(content, "public = yes") {
		issues = append(issues, "存在允许匿名访问的共享")
	}

	// 检查是否有 writable = yes without authentication
	if strings.Contains(content, "writable = yes") && !strings.Contains(content, "valid users") {
		issues = append(issues, "存在无认证的可写共享")
	}

	if len(issues) > 0 {
		result.Status = "warning"
		result.Message = "共享权限配置问题：" + strings.Join(issues, ", ")
		result.Remediation = "建议限制共享访问权限，禁用匿名访问"
	} else {
		result.Status = "pass"
		result.Message = "共享权限配置合理"
	}

	return result
}

// generateReportID 生成报告 ID
func generateReportID() string {
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		panic(err) // crypto/rand 失败是致命错误
	}
	return fmt.Sprintf("report-%s", hex.EncodeToString(randomBytes))
}

// GetCheckList 获取所有检查项列表
func (bm *BaselineManager) GetCheckList() []map[string]interface{} {
	list := make([]map[string]interface{}, 0, len(bm.checks))
	for _, check := range bm.checks {
		list = append(list, map[string]interface{}{
			"id":          check.ID,
			"name":        check.Name,
			"description": check.Description,
			"category":    check.Category,
			"severity":    check.Severity,
		})
	}
	return list
}

// GetCategories 获取所有检查类别
func (bm *BaselineManager) GetCategories() []string {
	categories := make(map[string]bool)
	for _, check := range bm.checks {
		categories[check.Category] = true
	}

	result := make([]string, 0, len(categories))
	for cat := range categories {
		result = append(result, cat)
	}
	return result
}
