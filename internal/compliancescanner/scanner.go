// Package compliancescanner 提供安全合规扫描功能
package compliancescanner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Scanner 扫描器.
type Scanner struct {
	mu         sync.RWMutex
	logger     *zap.Logger
	checkFuncs map[string]CheckFunction
	fixFuncs   map[string]FixFunction
	timeout    time.Duration
}

// CheckFunction 检查函数类型.
type CheckFunction func(ctx context.Context) (*ScanResult, error)

// FixFunction 修复函数类型.
type FixFunction func(ctx context.Context) error

// NewScanner 创建扫描器.
func NewScanner(logger *zap.Logger, timeout time.Duration) *Scanner {
	if logger == nil {
		logger = zap.NewNop()
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	s := &Scanner{
		logger:     logger,
		checkFuncs: make(map[string]CheckFunction),
		fixFuncs:   make(map[string]FixFunction),
		timeout:    timeout,
	}

	s.registerBuiltinChecks()
	return s
}

// registerBuiltinChecks 注册内置检查函数.
func (s *Scanner) registerBuiltinChecks() {
	s.checkFuncs["checkTmpMount"] = s.checkTmpMount
	s.checkFuncs["checkTmpPermissions"] = s.checkTmpPermissions
	s.checkFuncs["checkBootloaderConfig"] = s.checkBootloaderConfig
	s.checkFuncs["checkIdentityAuth"] = s.checkIdentityAuth
	s.checkFuncs["checkLoginFailure"] = s.checkLoginFailure
	s.checkFuncs["checkAccessControl"] = s.checkAccessControl
	s.checkFuncs["checkAuditPolicy"] = s.checkAuditPolicy
	s.checkFuncs["checkPasswdPermissions"] = s.checkPasswdPermissions
	s.checkFuncs["checkShadowPermissions"] = s.checkShadowPermissions
	s.checkFuncs["checkSUIDFiles"] = s.checkSUIDFiles
	s.checkFuncs["checkSGIDFiles"] = s.checkSGIDFiles
	s.checkFuncs["checkImportantFiles"] = s.checkImportantFiles
	s.checkFuncs["checkUserDataProtection"] = s.checkUserDataProtection
	s.checkFuncs["checkIPForward"] = s.checkIPForward
	s.checkFuncs["checkFirewall"] = s.checkFirewall
	s.checkFuncs["checkOpenPorts"] = s.checkOpenPorts
	s.checkFuncs["checkNetworkArchitecture"] = s.checkNetworkArchitecture
	s.checkFuncs["checkBoundaryProtection"] = s.checkBoundaryProtection
	s.checkFuncs["checkUnnecessaryServices"] = s.checkUnnecessaryServices
	s.checkFuncs["checkSSHConfig"] = s.checkSSHConfig
	s.checkFuncs["checkMalwareProtection"] = s.checkMalwareProtection
	s.checkFuncs["checkDataIntegrity"] = s.checkDataIntegrity
	s.checkFuncs["checkTLSVersion"] = s.checkTLSVersion
	s.checkFuncs["checkCipherSuites"] = s.checkCipherSuites
	s.checkFuncs["checkSSHKeyAlgorithms"] = s.checkSSHKeyAlgorithms
	s.checkFuncs["checkCryptoCompliance"] = s.checkCryptoCompliance

	s.fixFuncs["fixPasswdPermissions"] = s.fixPasswdPermissions
	s.fixFuncs["fixShadowPermissions"] = s.fixShadowPermissions
	s.fixFuncs["disableIPForward"] = s.disableIPForward
}

// RegisterCheckFunction 注册检查函数.
func (s *Scanner) RegisterCheckFunction(name string, fn CheckFunction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkFuncs[name] = fn
}

// RegisterFixFunction 注册修复函数.
func (s *Scanner) RegisterFixFunction(name string, fn FixFunction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fixFuncs[name] = fn
}

// ExecuteCheck 执行检查.
func (s *Scanner) ExecuteCheck(ctx context.Context, funcName string) (*ScanResult, error) {
	s.mu.RLock()
	fn, exists := s.checkFuncs[funcName]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("检查函数不存在: %s", funcName)
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return fn(ctx)
}

// ExecuteFix 执行修复.
func (s *Scanner) ExecuteFix(ctx context.Context, funcName string) error {
	s.mu.RLock()
	fn, exists := s.fixFuncs[funcName]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("修复函数不存在: %s", funcName)
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return fn(ctx)
}

// ========== 系统配置检查 ==========

func (s *Scanner) checkTmpMount(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategorySystemConfig, Severity: SeverityHigh, CheckedAt: time.Now()}
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		result.Result = ResultError
		result.Details = fmt.Sprintf("读取挂载信息失败: %v", err)
		return result, nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, " /tmp ") {
			result.Result = ResultPass
			result.Evidence = line
			result.Details = "/tmp 已独立挂载"
			return result, nil
		}
	}
	result.Result = ResultFail
	result.Details = "/tmp 未独立挂载"
	result.Remediation = "配置 /tmp 为独立分区或tmpfs"
	return result, nil
}

func (s *Scanner) checkTmpPermissions(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategorySystemConfig, Severity: SeverityMedium, CheckedAt: time.Now()}
	info, err := os.Stat("/tmp")
	if err != nil {
		result.Result = ResultError
		result.Details = fmt.Sprintf("获取 /tmp 信息失败: %v", err)
		return result, nil
	}
	perm := info.Mode().Perm()
	if perm == 0o1777 || perm == 0o777 {
		result.Result = ResultPass
		result.Evidence = fmt.Sprintf("权限: %04o", perm)
		result.Details = "/tmp 权限正确"
	} else {
		result.Result = ResultFail
		result.Evidence = fmt.Sprintf("当前权限: %04o，期望: 1777", perm)
		result.Details = "/tmp 权限不正确"
		result.Remediation = "设置 /tmp 权限为 1777"
	}
	return result, nil
}

func (s *Scanner) checkBootloaderConfig(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategorySystemConfig, Severity: SeverityCritical, CheckedAt: time.Now()}
	for _, grubFile := range []string{"/boot/grub/grub.cfg", "/boot/grub2/grub.cfg"} {
		info, err := os.Stat(grubFile)
		if err != nil {
			continue
		}
		perm := info.Mode().Perm()
		if perm <= 0o600 {
			result.Result = ResultPass
			result.Evidence = fmt.Sprintf("%s 权限: %04o", grubFile, perm)
			result.Details = "引导加载器配置文件权限正确"
			return result, nil
		}
		result.Result = ResultFail
		result.Evidence = fmt.Sprintf("%s 权限: %04o，期望: 600 或更严格", grubFile, perm)
		result.Details = "引导加载器配置文件权限过于宽松"
		result.Remediation = fmt.Sprintf("chmod 600 %s", grubFile)
		return result, nil
	}
	result.Result = ResultSkip
	result.Details = "未找到 GRUB 配置文件"
	return result, nil
}

func (s *Scanner) checkIdentityAuth(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategorySystemConfig, Severity: SeverityCritical, CheckedAt: time.Now()}
	for _, pamFile := range []string{"/etc/pam.d/common-auth", "/etc/pam.d/system-auth"} {
		data, err := os.ReadFile(pamFile)
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, "pam_unix.so") || strings.Contains(content, "pam_sss.so") {
			result.Result = ResultPass
			result.Details = "已配置身份鉴别机制"
			result.Evidence = pamFile
			return result, nil
		}
	}
	result.Result = ResultWarning
	result.Details = "建议配置多因素身份鉴别"
	result.Remediation = "配置 PAM 模块实现多因素认证"
	return result, nil
}

func (s *Scanner) checkLoginFailure(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategorySystemConfig, Severity: SeverityHigh, CheckedAt: time.Now()}
	for _, pamFile := range []string{"/etc/pam.d/common-auth", "/etc/pam.d/system-auth"} {
		data, err := os.ReadFile(pamFile)
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, "pam_tally2") || strings.Contains(content, "pam_faillock") {
			result.Result = ResultPass
			result.Details = "已配置登录失败处理"
			result.Evidence = pamFile
			return result, nil
		}
	}
	result.Result = ResultFail
	result.Details = "未配置登录失败处理策略"
	result.Remediation = "配置 pam_tally2 或 pam_faillock 模块"
	return result, nil
}

func (s *Scanner) checkAccessControl(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategorySystemConfig, Severity: SeverityCritical, CheckedAt: time.Now()}
	data, err := os.ReadFile("/etc/login.defs")
	if err != nil {
		result.Result = ResultError
		result.Details = fmt.Sprintf("读取 login.defs 失败: %v", err)
		return result, nil
	}
	content := string(data)
	if strings.Contains(content, "UMASK") && strings.Contains(content, "PASS_MAX_DAYS") {
		result.Result = ResultPass
		result.Details = "已配置基本访问控制策略"
	} else {
		result.Result = ResultWarning
		result.Details = "访问控制策略不完善"
		result.Remediation = "配置完善的访问控制策略"
	}
	return result, nil
}

func (s *Scanner) checkAuditPolicy(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategorySystemConfig, Severity: SeverityCritical, CheckedAt: time.Now()}
	if _, err := exec.LookPath("auditctl"); err == nil {
		result.Result = ResultPass
		result.Details = "审计服务可用"
	} else {
		result.Result = ResultWarning
		result.Details = "建议启用审计服务"
		result.Remediation = "安装并配置 auditd"
	}
	return result, nil
}

// ========== 文件权限检查 ==========

func (s *Scanner) checkPasswdPermissions(ctx context.Context) (*ScanResult, error) {
	return s.checkFilePermission("/etc/passwd", 0o644, CategoryFilePermission, SeverityCritical)
}

func (s *Scanner) checkShadowPermissions(ctx context.Context) (*ScanResult, error) {
	return s.checkFilePermission("/etc/shadow", 0o640, CategoryFilePermission, SeverityCritical)
}

func (s *Scanner) checkFilePermission(path string, expected os.FileMode, category ScanCategory, severity SeverityLevel) (*ScanResult, error) {
	result := &ScanResult{Category: category, Severity: severity, CheckedAt: time.Now()}
	info, err := os.Stat(path)
	if err != nil {
		result.Result = ResultError
		result.Details = fmt.Sprintf("获取文件信息失败: %v", err)
		return result, nil
	}
	perm := info.Mode().Perm()
	if perm <= expected {
		result.Result = ResultPass
		result.Evidence = fmt.Sprintf("%s 权限: %04o", path, perm)
		result.Details = "文件权限正确"
	} else {
		result.Result = ResultFail
		result.Evidence = fmt.Sprintf("%s 权限: %04o，期望: %04o 或更严格", path, perm, expected)
		result.Details = "文件权限过于宽松"
		result.Remediation = fmt.Sprintf("chmod %04o %s", expected, path)
	}
	return result, nil
}

func (s *Scanner) checkSUIDFiles(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategoryFilePermission, Severity: SeverityHigh, CheckedAt: time.Now()}
	knownSUID := []string{
		"/usr/bin/passwd", "/usr/bin/sudo", "/usr/bin/su", "/usr/bin/pkexec",
		"/usr/bin/chsh", "/usr/bin/chfn", "/usr/bin/newgrp", "/usr/bin/gpasswd",
		"/usr/bin/mount", "/usr/bin/umount", "/usr/bin/fusermount",
	}
	suidFiles := make([]string, 0)
	for _, path := range knownSUID {
		info, err := os.Stat(path)
		if err == nil && info.Mode()&os.ModeSetuid != 0 {
			suidFiles = append(suidFiles, path)
		}
	}
	if len(suidFiles) > 0 {
		result.Result = ResultWarning
		result.Evidence = fmt.Sprintf("发现 %d 个SUID文件", len(suidFiles))
		result.Details = "系统中存在SUID文件，请检查是否必要"
		result.Remediation = "移除不必要的SUID权限: chmod u-s <file>"
	} else {
		result.Result = ResultPass
		result.Details = "未发现异常SUID文件"
	}
	return result, nil
}

func (s *Scanner) checkSGIDFiles(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategoryFilePermission, Severity: SeverityHigh, CheckedAt: time.Now()}
	cmd := exec.CommandContext(ctx, "find", "/usr", "-type", "f", "-perm", "-2000", "-ls")
	output, err := cmd.Output()
	if err != nil {
		result.Result = ResultError
		result.Details = fmt.Sprintf("查找SGID文件失败: %v", err)
		return result, nil
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > 0 && lines[0] != "" {
		result.Result = ResultWarning
		result.Evidence = fmt.Sprintf("发现 %d 个SGID文件", len(lines))
		result.Details = "系统中存在SGID文件，请检查是否必要"
		result.Remediation = "移除不必要的SGID权限: chmod g-s <file>"
	} else {
		result.Result = ResultPass
		result.Details = "未发现异常SGID文件"
	}
	return result, nil
}

func (s *Scanner) checkImportantFiles(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategoryFilePermission, Severity: SeverityCritical, CheckedAt: time.Now()}
	importantFiles := map[string]os.FileMode{
		"/etc/passwd":          0o644,
		"/etc/shadow":          0o640,
		"/etc/group":           0o644,
		"/etc/gshadow":         0o640,
		"/etc/ssh/sshd_config": 0o600,
	}
	failedFiles := make([]string, 0)
	for path, expected := range importantFiles {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Mode().Perm() > expected {
			failedFiles = append(failedFiles, path)
		}
	}
	if len(failedFiles) > 0 {
		result.Result = ResultFail
		result.Details = fmt.Sprintf("%d 个重要文件权限不正确", len(failedFiles))
		result.Evidence = strings.Join(failedFiles, ", ")
		result.Remediation = "检查并修复重要文件权限"
	} else {
		result.Result = ResultPass
		result.Details = "重要文件权限正确"
	}
	return result, nil
}

func (s *Scanner) checkUserDataProtection(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategoryFilePermission, Severity: SeverityHigh, CheckedAt: time.Now()}
	entries, err := os.ReadDir("/home")
	if err != nil {
		result.Result = ResultError
		result.Details = fmt.Sprintf("读取主目录失败: %v", err)
		return result, nil
	}
	insecureDirs := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := os.Stat(filepath.Join("/home", entry.Name()))
		if err != nil {
			continue
		}
		if info.Mode().Perm() > 0o750 {
			insecureDirs = append(insecureDirs, entry.Name())
		}
	}
	if len(insecureDirs) > 0 {
		result.Result = ResultWarning
		result.Details = fmt.Sprintf("%d 个用户主目录权限过于宽松", len(insecureDirs))
		result.Evidence = strings.Join(insecureDirs, ", ")
		result.Remediation = "设置用户主目录权限为 750 或更严格"
	} else {
		result.Result = ResultPass
		result.Details = "用户数据保护措施到位"
	}
	return result, nil
}

// ========== 网络安全检查 ==========

func (s *Scanner) checkIPForward(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategoryNetworkSecurity, Severity: SeverityHigh, CheckedAt: time.Now()}
	data, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if err != nil {
		result.Result = ResultError
		result.Details = fmt.Sprintf("读取IP转发配置失败: %v", err)
		return result, nil
	}
	value := strings.TrimSpace(string(data))
	if value == "0" {
		result.Result = ResultPass
		result.Evidence = "ip_forward = 0"
		result.Details = "IP转发已禁用"
	} else {
		result.Result = ResultFail
		result.Evidence = fmt.Sprintf("ip_forward = %s", value)
		result.Details = "IP转发已启用"
		result.Remediation = "在 /etc/sysctl.conf 中设置 net.ipv4.ip_forward = 0"
	}
	return result, nil
}

func (s *Scanner) checkFirewall(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategoryNetworkSecurity, Severity: SeverityCritical, CheckedAt: time.Now()}
	if _, err := exec.LookPath("iptables"); err == nil {
		cmd := exec.CommandContext(ctx, "iptables", "-L", "-n")
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			result.Result = ResultPass
			result.Details = "防火墙已配置"
			result.Evidence = "iptables"
			return result, nil
		}
	}
	if _, err := exec.LookPath("nft"); err == nil {
		cmd := exec.CommandContext(ctx, "nft", "list", "ruleset")
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			result.Result = ResultPass
			result.Details = "防火墙已配置"
			result.Evidence = "nftables"
			return result, nil
		}
	}
	result.Result = ResultFail
	result.Details = "未检测到活跃的防火墙规则"
	result.Remediation = "启用并配置防火墙"
	return result, nil
}

func (s *Scanner) checkOpenPorts(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategoryNetworkSecurity, Severity: SeverityMedium, CheckedAt: time.Now()}
	cmd := exec.CommandContext(ctx, "ss", "-tlnp")
	output, err := cmd.Output()
	if err != nil {
		result.Result = ResultError
		result.Details = fmt.Sprintf("检查端口失败: %v", err)
		return result, nil
	}
	lines := strings.Split(string(output), "\n")
	if len(lines) > 1 {
		lines = lines[1:]
	}
	safePorts := map[string]bool{"22": true, "80": true, "443": true, "8080": true}
	openPorts := make([]string, 0)
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 4 {
			addr := parts[3]
			portParts := strings.Split(addr, ":")
			if len(portParts) >= 2 {
				port := portParts[len(portParts)-1]
				if !safePorts[port] {
					openPorts = append(openPorts, port)
				}
			}
		}
	}
	if len(openPorts) > 0 {
		result.Result = ResultWarning
		result.Details = fmt.Sprintf("发现 %d 个非常规开放端口", len(openPorts))
		result.Evidence = strings.Join(openPorts, ", ")
		result.Remediation = "关闭不必要的网络端口"
	} else {
		result.Result = ResultPass
		result.Details = "未发现异常开放端口"
	}
	return result, nil
}

func (s *Scanner) checkNetworkArchitecture(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategoryNetworkSecurity, Severity: SeverityCritical, CheckedAt: time.Now()}
	cmd := exec.CommandContext(ctx, "ip", "addr", "show")
	output, err := cmd.Output()
	if err != nil {
		result.Result = ResultError
		result.Details = fmt.Sprintf("检查网络架构失败: %v", err)
		return result, nil
	}
	if strings.Count(string(output), "state UP") > 1 {
		result.Result = ResultPass
		result.Details = "已配置多个网络接口，建议进行安全域划分"
	} else {
		result.Result = ResultWarning
		result.Details = "建议实施网络安全域划分"
		result.Remediation = "配置网络安全域，实施访问控制"
	}
	return result, nil
}

func (s *Scanner) checkBoundaryProtection(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategoryNetworkSecurity, Severity: SeverityCritical, CheckedAt: time.Now()}
	hasVPN := false
	for _, dir := range []string{"/etc/openvpn", "/etc/wireguard", "/etc/v2ray"} {
		if _, err := os.Stat(dir); err == nil {
			hasVPN = true
			break
		}
	}
	if hasVPN {
		result.Result = ResultPass
		result.Details = "已配置边界防护措施"
	} else {
		result.Result = ResultWarning
		result.Details = "建议配置网络边界防护"
		result.Remediation = "配置VPN或防火墙进行边界防护"
	}
	return result, nil
}

// ========== 服务安全检查 ==========

func (s *Scanner) checkUnnecessaryServices(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategoryServiceSecurity, Severity: SeverityMedium, CheckedAt: time.Now()}
	unnecessaryServices := []string{"telnet.socket", "rsh.socket", "rlogin.socket", "rexec.socket", "tftp.socket"}
	if _, err := exec.LookPath("systemctl"); err == nil {
		runningServices := make([]string, 0)
		for _, svc := range unnecessaryServices {
			cmd := exec.CommandContext(ctx, "systemctl", "is-active", svc)
			output, err := cmd.Output()
			if err == nil && strings.TrimSpace(string(output)) == "active" {
				runningServices = append(runningServices, svc)
			}
		}
		if len(runningServices) > 0 {
			result.Result = ResultFail
			result.Details = fmt.Sprintf("发现 %d 个不必要的服务运行中", len(runningServices))
			result.Evidence = strings.Join(runningServices, ", ")
			result.Remediation = "禁用不必要的服务"
		} else {
			result.Result = ResultPass
			result.Details = "未发现不必要的服务"
		}
	} else {
		result.Result = ResultSkip
		result.Details = "系统无 systemctl，跳过检查"
	}
	return result, nil
}

func (s *Scanner) checkSSHConfig(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategoryServiceSecurity, Severity: SeverityCritical, CheckedAt: time.Now()}
	data, err := os.ReadFile("/etc/ssh/sshd_config")
	if err != nil {
		result.Result = ResultError
		result.Details = fmt.Sprintf("读取SSH配置失败: %v", err)
		return result, nil
	}
	content := string(data)
	issues := make([]string, 0)
	if strings.Contains(content, "PermitRootLogin yes") {
		issues = append(issues, "允许root登录")
	}
	if strings.Contains(content, "PermitEmptyPasswords yes") {
		issues = append(issues, "允许空密码")
	}
	if strings.Contains(content, "Protocol 1") {
		issues = append(issues, "使用不安全的SSH协议1")
	}
	if len(issues) > 0 {
		result.Result = ResultFail
		result.Details = fmt.Sprintf("SSH配置存在 %d 个安全问题", len(issues))
		result.Evidence = strings.Join(issues, "; ")
		result.Remediation = "修复SSH安全配置"
	} else {
		result.Result = ResultPass
		result.Details = "SSH配置安全"
	}
	return result, nil
}

func (s *Scanner) checkMalwareProtection(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategoryServiceSecurity, Severity: SeverityCritical, CheckedAt: time.Now()}
	hasAV := false
	for _, tool := range []string{"clamscan", "rkhunter", "chkrootkit"} {
		if _, err := exec.LookPath(tool); err == nil {
			hasAV = true
			break
		}
	}
	if hasAV {
		result.Result = ResultPass
		result.Details = "已安装恶意代码防范工具"
	} else {
		result.Result = ResultWarning
		result.Details = "建议安装恶意代码防范工具"
		result.Remediation = "安装 clamav 或其他杀毒软件"
	}
	return result, nil
}

func (s *Scanner) checkDataIntegrity(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategoryServiceSecurity, Severity: SeverityHigh, CheckedAt: time.Now()}
	hasTool := false
	for _, tool := range []string{"aide", "tripwire", "ossec"} {
		if _, err := exec.LookPath(tool); err == nil {
			hasTool = true
			break
		}
	}
	if hasTool {
		result.Result = ResultPass
		result.Details = "已安装数据完整性检查工具"
	} else {
		result.Result = ResultWarning
		result.Details = "建议安装数据完整性检查工具"
		result.Remediation = "安装 aide 或其他完整性检查工具"
	}
	return result, nil
}

// ========== 加密合规检查 ==========

func (s *Scanner) checkTLSVersion(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategoryCryptoCompliance, Severity: SeverityCritical, CheckedAt: time.Now()}
	cmd := exec.CommandContext(ctx, "openssl", "version")
	output, err := cmd.Output()
	if err != nil {
		result.Result = ResultError
		result.Details = fmt.Sprintf("检查OpenSSL版本失败: %v", err)
		return result, nil
	}
	version := string(output)
	if strings.Contains(version, "1.1.1") || strings.Contains(version, "3.") {
		result.Result = ResultPass
		result.Details = "支持 TLS 1.2+"
		result.Evidence = version
	} else {
		result.Result = ResultWarning
		result.Details = "建议升级 OpenSSL 以支持 TLS 1.2+"
		result.Evidence = version
	}
	return result, nil
}

func (s *Scanner) checkCipherSuites(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategoryCryptoCompliance, Severity: SeverityHigh, CheckedAt: time.Now()}
	data, err := os.ReadFile("/etc/ssh/sshd_config")
	if err != nil {
		result.Result = ResultError
		result.Details = fmt.Sprintf("读取SSH配置失败: %v", err)
		return result, nil
	}
	content := string(data)
	weakCiphers := []string{"3des-cbc", "aes128-cbc", "aes192-cbc", "aes256-cbc", "blowfish-cbc"}
	foundWeak := make([]string, 0)
	for _, cipher := range weakCiphers {
		if strings.Contains(content, cipher) {
			foundWeak = append(foundWeak, cipher)
		}
	}
	if len(foundWeak) > 0 {
		result.Result = ResultFail
		result.Details = fmt.Sprintf("发现 %d 个弱密码套件", len(foundWeak))
		result.Evidence = strings.Join(foundWeak, ", ")
		result.Remediation = "禁用弱密码套件，使用强密码算法"
	} else {
		result.Result = ResultPass
		result.Details = "未发现弱密码套件"
	}
	return result, nil
}

func (s *Scanner) checkSSHKeyAlgorithms(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategoryCryptoCompliance, Severity: SeverityHigh, CheckedAt: time.Now()}
	data, err := os.ReadFile("/etc/ssh/sshd_config")
	if err != nil {
		result.Result = ResultError
		result.Details = fmt.Sprintf("读取SSH配置失败: %v", err)
		return result, nil
	}
	content := string(data)
	weakAlgorithms := []string{"ssh-dss", "ssh-rsa"}
	foundWeak := make([]string, 0)
	for _, algo := range weakAlgorithms {
		if strings.Contains(content, algo) {
			foundWeak = append(foundWeak, algo)
		}
	}
	if len(foundWeak) > 0 {
		result.Result = ResultWarning
		result.Details = fmt.Sprintf("发现 %d 个不推荐的密钥算法", len(foundWeak))
		result.Evidence = strings.Join(foundWeak, ", ")
		result.Remediation = "使用更安全的密钥算法如 ed25519 或 ecdsa"
	} else {
		result.Result = ResultPass
		result.Details = "SSH密钥算法配置安全"
	}
	return result, nil
}

func (s *Scanner) checkCryptoCompliance(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{Category: CategoryCryptoCompliance, Severity: SeverityCritical, CheckedAt: time.Now()}
	cmd := exec.CommandContext(ctx, "openssl", "list", "-public-key-algorithms")
	output, err := cmd.Output()
	if err != nil {
		result.Result = ResultError
		result.Details = fmt.Sprintf("检查密码算法失败: %v", err)
		return result, nil
	}
	algorithms := string(output)
	if strings.Contains(algorithms, "RSA") || strings.Contains(algorithms, "EC") {
		result.Result = ResultPass
		result.Details = "支持主流密码算法"
		result.Evidence = "RSA/EC 可用"
	} else {
		result.Result = ResultWarning
		result.Details = "密码算法支持有限"
		result.Remediation = "确保系统支持 RSA 和 EC 算法"
	}
	return result, nil
}

// ========== 修复函数 ==========

func (s *Scanner) fixPasswdPermissions(ctx context.Context) error {
	return os.Chmod("/etc/passwd", 0o644)
}

func (s *Scanner) fixShadowPermissions(ctx context.Context) error {
	return os.Chmod("/etc/shadow", 0o640)
}

func (s *Scanner) disableIPForward(ctx context.Context) error {
	return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("0"), 0o644)
}
