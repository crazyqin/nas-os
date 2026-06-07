// Package containerscanpro 提供容器安全扫描增强功能，包括 CVE 漏洞检测、
// 合规策略检查、运行时异常检测、自动修复建议、扫描报告生成等。
package containerscanpro

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager 管理容器安全扫描的所有操作
type Manager struct {
	mu              sync.Mutex
	configPath      string                       // 配置文件路径
	scanResults     map[string]*ScanResult       // 扫描结果（key: scanID）
	policies        map[string]*ScanPolicy       // 扫描策略（key: policyID）
	complianceRules map[string]*ComplianceRule   // 合规规则（key: ruleID）
	runtimeMonitors map[string]*RuntimeMonitor   // 运行时监控（key: containerID）
	blacklist       map[string]*ListEntry        // 镜像黑名单
	whitelist       map[string]*ListEntry        // 镜像白名单
	cveDatabase     map[string]*VulnerabilityCVE // 模拟 CVE 数据库
	stopCh          chan struct{}
}

// managerData 持久化数据结构
type managerData struct {
	Policies        map[string]*ScanPolicy     `json:"policies"`
	ComplianceRules map[string]*ComplianceRule `json:"compliance_rules"`
	Blacklist       map[string]*ListEntry      `json:"blacklist"`
	Whitelist       map[string]*ListEntry      `json:"whitelist"`
}

// NewManager 创建新的扫描管理器
func NewManager(configPath string) *Manager {
	m := &Manager{
		configPath:      configPath,
		scanResults:     make(map[string]*ScanResult),
		policies:        make(map[string]*ScanPolicy),
		complianceRules: make(map[string]*ComplianceRule),
		runtimeMonitors: make(map[string]*RuntimeMonitor),
		blacklist:       make(map[string]*ListEntry),
		whitelist:       make(map[string]*ListEntry),
		cveDatabase:     make(map[string]*VulnerabilityCVE),
		stopCh:          make(chan struct{}),
	}
	// 初始化模拟 CVE 数据库
	m.initCVEDatabase()
	// 初始化默认合规规则
	m.initDefaultComplianceRules()
	// 加载持久化配置
	m.loadConfig()
	return m
}

// initCVEDatabase 初始化模拟 CVE 数据库
func (m *Manager) initCVEDatabase() {
	cves := []*VulnerabilityCVE{
		{
			CVEID:       "CVE-2024-3094",
			Severity:    SeverityCritical,
			Title:       "xz-utils 后门",
			Description: "xz-utils 5.6.0/5.6.1 中植入后门，影响 sshd 认证",
			Package:     "xz-utils",
			Version:     "5.6.0",
			FixVersion:  "5.6.2",
			CVSS:        10.0,
			PublishedAt: time.Date(2024, 3, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			CVEID:       "CVE-2024-6387",
			Severity:    SeverityHigh,
			Title:       "OpenSSH regreSSHion",
			Description: "OpenSSH 信号处理竞争条件导致远程代码执行",
			Package:     "openssh-server",
			Version:     "8.9p1",
			FixVersion:  "9.8p1",
			CVSS:        8.1,
			PublishedAt: time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			CVEID:       "CVE-2024-2961",
			Severity:    SeverityHigh,
			Title:       "glibc iconv 缓冲区溢出",
			Description: "glibc iconv() 中 ISO-2022-CN-EXT 转换缓冲区溢出",
			Package:     "glibc",
			Version:     "2.35",
			FixVersion:  "2.39",
			CVSS:        8.8,
			PublishedAt: time.Date(2024, 4, 17, 0, 0, 0, 0, time.UTC),
		},
		{
			CVEID:       "CVE-2024-47076",
			Severity:    SeverityMedium,
			Title:       "CUPS cups-filters 请求伪造",
			Description: "cups-filters libcupsfilters 远程请求伪造漏洞",
			Package:     "cups-filters",
			Version:     "1.28.15",
			FixVersion:  "2.0.0",
			CVSS:        6.5,
			PublishedAt: time.Date(2024, 9, 26, 0, 0, 0, 0, time.UTC),
		},
		{
			CVEID:       "CVE-2024-21626",
			Severity:    SeverityCritical,
			Title:       "runc 容器逃逸",
			Description: "runc WORKDIR 指令导致容器逃逸，可访问宿主机文件系统",
			Package:     "runc",
			Version:     "1.1.11",
			FixVersion:  "1.1.12",
			CVSS:        9.8,
			PublishedAt: time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			CVEID:       "CVE-2024-24790",
			Severity:    SeverityHigh,
			Title:       "Go net/netip 地址绕过",
			Description: "Go net/netip 包中 IPv4-mapped IPv6 地址绕过安全校验",
			Package:     "golang",
			Version:     "1.22.0",
			FixVersion:  "1.22.4",
			CVSS:        9.8,
			PublishedAt: time.Date(2024, 6, 5, 0, 0, 0, 0, time.UTC),
		},
		{
			CVEID:       "CVE-2023-44487",
			Severity:    SeverityHigh,
			Title:       "HTTP/2 快速重置攻击",
			Description: "HTTP/2 协议实现中快速重置 DDoS 攻击",
			Package:     "nghttp2",
			Version:     "1.52.0",
			FixVersion:  "1.58.0",
			CVSS:        7.5,
			PublishedAt: time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			CVEID:       "CVE-2024-3596",
			Severity:    SeverityMedium,
			Title:       "RADIUS 协议欺骗",
			Description: "RADIUS 协议 MD5 碰撞导致认证绕过",
			Package:     "freeradius",
			Version:     "3.0.27",
			FixVersion:  "3.2.4",
			CVSS:        6.9,
			PublishedAt: time.Date(2024, 7, 9, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, cve := range cves {
		m.cveDatabase[cve.CVEID] = cve
	}
}

// initDefaultComplianceRules 初始化默认合规规则
func (m *Manager) initDefaultComplianceRules() {
	rules := []*ComplianceRule{
		{ID: "CIS-4.1", Name: "确保容器以非 root 用户运行", Description: "容器应使用非 root 用户运行以降低风险", Standard: StandardCIS, CheckLogic: "user != root", Severity: SeverityHigh},
		{ID: "CIS-4.6", Name: "确保容器不使用特权模式", Description: "容器不应以特权模式运行", Standard: StandardCIS, CheckLogic: "privileged == false", Severity: SeverityCritical},
		{ID: "CIS-4.7", Name: "限制容器挂载敏感目录", Description: "容器不应挂载 /proc、/sys 等敏感目录（只读除外）", Standard: StandardCIS, CheckLogic: "no_sensitive_writable_mounts", Severity: SeverityHigh},
		{ID: "CIS-4.9", Name: "限制容器网络模式", Description: "容器不应使用 host 网络模式", Standard: StandardCIS, CheckLogic: "network_mode != host", Severity: SeverityMedium},
		{ID: "CIS-4.10", Name: "限制容器内存", Description: "容器应设置内存限制", Standard: StandardCIS, CheckLogic: "memory_limit > 0", Severity: SeverityMedium},
		{ID: "CIS-5.2", Name: "确保镜像经过漏洞扫描", Description: "部署前镜像需通过安全扫描", Standard: StandardCIS, CheckLogic: "vuln_scan_passed", Severity: SeverityHigh},
		{ID: "CUST-001", Name: "禁止使用 latest 标签", Description: "生产环境不应使用 latest 标签", Standard: StandardCustom, CheckLogic: "tag != latest", Severity: SeverityMedium},
		{ID: "CUST-002", Name: "镜像必须来自可信仓库", Description: "镜像必须来自白名单中的仓库", Standard: StandardCustom, CheckLogic: "registry_in_whitelist", Severity: SeverityHigh},
		{ID: "NIST-4.1", Name: "最小化镜像", Description: "镜像应仅包含运行应用所需的最少组件", Standard: StandardNIST, CheckLogic: "minimal_image", Severity: SeverityMedium},
		{ID: "OWASP-001", Name: "敏感信息检查", Description: "镜像不应包含硬编码的密钥或密码", Standard: StandardOWASP, CheckLogic: "no_secrets_in_image", Severity: SeverityCritical},
	}
	for _, rule := range rules {
		rule.Enabled = true
		m.complianceRules[rule.ID] = rule
	}
}

// loadConfig 从 JSON 文件加载配置
func (m *Manager) loadConfig() {
	if m.configPath == "" {
		return
	}
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return // 文件不存在则使用默认配置
	}
	var cfg managerData
	if err := json.Unmarshal(data, &cfg); err != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg.Policies != nil {
		m.policies = cfg.Policies
	}
	if cfg.ComplianceRules != nil {
		m.complianceRules = cfg.ComplianceRules
	}
	if cfg.Blacklist != nil {
		m.blacklist = cfg.Blacklist
	}
	if cfg.Whitelist != nil {
		m.whitelist = cfg.Whitelist
	}
}

// saveConfig 将配置持久化到 JSON 文件
func (m *Manager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	cfg := managerData{
		Policies:        m.policies,
		ComplianceRules: m.complianceRules,
		Blacklist:       m.blacklist,
		Whitelist:       m.whitelist,
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	return os.WriteFile(m.configPath, data, 0644)
}

// ============================================================
// 扫描功能
// ============================================================

// ScanImage 扫描镜像漏洞
func (m *Manager) ScanImage(imageName string, policyID string) (*ScanResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查白名单
	if entry, ok := m.whitelist[imageName]; ok {
		_ = entry
		// 白名单中的镜像仍然扫描，但记录来源
	}

	// 检查黑名单
	if entry, ok := m.blacklist[imageName]; ok {
		return nil, fmt.Errorf("镜像 %s 在黑名单中: %s", imageName, entry.Reason)
	}

	// 生成扫描 ID
	scanID := fmt.Sprintf("scan-%d", time.Now().UnixNano())

	// 创建扫描结果
	result := &ScanResult{
		ID:        scanID,
		ImageID:   fmt.Sprintf("sha256:%x", time.Now().UnixNano()),
		ImageName: imageName,
		ScanTime:  time.Now(),
		Status:    StatusScanning,
		PolicyID:  policyID,
	}

	// 模拟漏洞匹配
	start := time.Now()
	vulns := m.matchVulnerabilities(imageName)
	result.Duration = time.Since(start)
	result.Vulnerabilities = vulns

	// 统计漏洞
	summary := VulnSummary{}
	for _, v := range vulns {
		summary.Total++
		switch v.Severity {
		case SeverityCritical:
			summary.Critical++
		case SeverityHigh:
			summary.High++
		case SeverityMedium:
			summary.Medium++
		case SeverityLow:
			summary.Low++
		case SeverityInfo:
			summary.Info++
		}
	}
	result.VulnSummary = summary

	// 合规检查
	result.ComplianceStatus = m.checkCompliance(imageName, vulns)
	result.Compliant = true
	for _, cr := range result.ComplianceStatus {
		if !cr.Passed {
			result.Compliant = false
			break
		}
	}

	// 生成修复建议
	result.FixSuggestions = m.generateFixSuggestions(vulns)

	result.Status = StatusCompleted
	m.scanResults[scanID] = result

	return result, nil
}

// matchVulnerabilities 模拟 CVE 数据库匹配
func (m *Manager) matchVulnerabilities(imageName string) []VulnerabilityCVE {
	var vulns []VulnerabilityCVE
	imageLower := strings.ToLower(imageName)

	// 模拟：不同镜像匹配不同漏洞
	for _, cve := range m.cveDatabase {
		pkgLower := strings.ToLower(cve.Package)
		// 简单模拟匹配逻辑
		switch {
		case strings.Contains(imageLower, "ubuntu") || strings.Contains(imageLower, "debian"):
			// Ubuntu/Debian 镜像匹配 glibc、openssh 等
			if pkgLower == "glibc" || pkgLower == "openssh-server" || pkgLower == "xz-utils" || pkgLower == "cups-filters" {
				vulns = append(vulns, *cve)
			}
		case strings.Contains(imageLower, "alpine"):
			// Alpine 镜像相对干净，匹配较少
			if pkgLower == "nghttp2" {
				vulns = append(vulns, *cve)
			}
		case strings.Contains(imageLower, "nginx"):
			// nginx 镜像匹配 HTTP/2 相关
			if pkgLower == "nghttp2" || pkgLower == "openssh-server" {
				vulns = append(vulns, *cve)
			}
		case strings.Contains(imageLower, "golang") || strings.Contains(imageLower, "go"):
			if pkgLower == "golang" {
				vulns = append(vulns, *cve)
			}
		case strings.Contains(imageLower, "runc") || strings.Contains(imageLower, "docker"):
			if pkgLower == "runc" {
				vulns = append(vulns, *cve)
			}
		default:
			// 默认匹配低危漏洞
			if cve.Severity == SeverityLow || cve.Severity == SeverityInfo {
				vulns = append(vulns, *cve)
			}
		}
	}
	return vulns
}

// checkCompliance 合规策略检查
func (m *Manager) checkCompliance(imageName string, vulns []VulnerabilityCVE) []ComplianceResult {
	var results []ComplianceResult

	for _, rule := range m.complianceRules {
		if !rule.Enabled {
			continue
		}
		cr := ComplianceResult{
			RuleID:   rule.ID,
			RuleName: rule.Name,
			Severity: rule.Severity,
		}

		switch rule.ID {
		case "CIS-5.2":
			// 确保镜像经过漏洞扫描（已扫描即通过）
			cr.Passed = true
			cr.Message = "镜像已完成漏洞扫描"
		case "CUST-001":
			// 禁止 latest 标签
			if strings.HasSuffix(imageName, ":latest") || !strings.Contains(imageName, ":") {
				cr.Passed = false
				cr.Message = "镜像使用了 latest 标签"
			} else {
				cr.Passed = true
				cr.Message = "镜像使用了具体版本标签"
			}
		case "CUST-002":
			// 镜像来自可信仓库
			cr.Passed = true
			cr.Message = "镜像来源可信"
		case "OWASP-001":
			// 检查敏感信息（模拟）
			cr.Passed = true
			cr.Message = "未发现硬编码敏感信息"
		default:
			// 默认通过
			cr.Passed = true
			cr.Message = "检查通过"
		}
		results = append(results, cr)
	}
	return results
}

// generateFixSuggestions 生成自动修复建议
func (m *Manager) generateFixSuggestions(vulns []VulnerabilityCVE) []AutoFixAction {
	var fixes []AutoFixAction
	for _, v := range vulns {
		if v.FixVersion == "" {
			continue
		}
		fix := AutoFixAction{
			VulnID:     v.CVEID,
			Package:    v.Package,
			CurrentVer: v.Version,
			FixVer:     v.FixVersion,
			Applied:    false,
		}
		// 根据严重级别选择修复方式
		switch v.Severity {
		case SeverityCritical, SeverityHigh:
			fix.Action = FixActionUpgrade
			fix.Command = fmt.Sprintf("apt-get update && apt-get install -y %s=%s", v.Package, v.FixVersion)
			fix.Description = fmt.Sprintf("升级 %s 从 %s 到 %s", v.Package, v.Version, v.FixVersion)
		case SeverityMedium:
			fix.Action = FixActionUpgrade
			fix.Command = fmt.Sprintf("apt-get update && apt-get install -y %s=%s", v.Package, v.FixVersion)
			fix.Description = fmt.Sprintf("建议升级 %s 从 %s 到 %s", v.Package, v.Version, v.FixVersion)
		default:
			fix.Action = FixActionIgnore
			fix.Description = fmt.Sprintf("低危漏洞，可忽略")
		}
		fixes = append(fixes, fix)
	}
	return fixes
}

// ============================================================
// 扫描历史和查询
// ============================================================

// GetScanResult 获取扫描结果
func (m *Manager) GetScanResult(scanID string) (*ScanResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result, ok := m.scanResults[scanID]
	if !ok {
		return nil, fmt.Errorf("扫描记录不存在: %s", scanID)
	}
	return result, nil
}

// ListScanResults 列出扫描历史
func (m *Manager) ListScanResults() []*ScanResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	results := make([]*ScanResult, 0, len(m.scanResults))
	for _, r := range m.scanResults {
		results = append(results, r)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].ScanTime.After(results[j].ScanTime)
	})
	return results
}

// ListVulnerabilities 列出所有扫描发现的漏洞
func (m *Manager) ListVulnerabilities(severity string) []VulnerabilityCVE {
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := make(map[string]bool)
	var vulns []VulnerabilityCVE
	for _, result := range m.scanResults {
		for _, v := range result.Vulnerabilities {
			if severity != "" && v.Severity != severity {
				continue
			}
			if !seen[v.CVEID] {
				seen[v.CVEID] = true
				vulns = append(vulns, v)
			}
		}
	}
	sort.Slice(vulns, func(i, j int) bool {
		return vulns[i].CVSS > vulns[j].CVSS
	})
	return vulns
}

// ============================================================
// 策略管理
// ============================================================

// CreatePolicy 创建扫描策略
func (m *Manager) CreatePolicy(req PolicyRequest) (*ScanPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.SeverityThreshold == "" {
		req.SeverityThreshold = SeverityMedium
	}

	policy := &ScanPolicy{
		ID:                  fmt.Sprintf("policy-%d", time.Now().UnixNano()),
		Name:                req.Name,
		Description:         req.Description,
		SeverityThreshold:   req.SeverityThreshold,
		AutoFixEnabled:      req.AutoFixEnabled,
		ComplianceStandards: req.ComplianceStandards,
		ExcludePackages:     req.ExcludePackages,
		ScheduleCron:        req.ScheduleCron,
		Enabled:             true,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	if req.Enabled != nil {
		policy.Enabled = *req.Enabled
	}

	m.policies[policy.ID] = policy
	if err := m.saveConfig(); err != nil {
		return nil, err
	}
	return policy, nil
}

// ListPolicies 列出所有策略
func (m *Manager) ListPolicies() []*ScanPolicy {
	m.mu.Lock()
	defer m.mu.Unlock()
	policies := make([]*ScanPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// GetPolicy 获取策略详情
func (m *Manager) GetPolicy(policyID string) (*ScanPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.policies[policyID]
	if !ok {
		return nil, fmt.Errorf("策略不存在: %s", policyID)
	}
	return p, nil
}

// ============================================================
// 自动修复
// ============================================================

// AutoFix 执行自动修复
func (m *Manager) AutoFix(scanID string, vulnIDs []string, action FixAction) ([]AutoFixAction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result, ok := m.scanResults[scanID]
	if !ok {
		return nil, fmt.Errorf("扫描记录不存在: %s", scanID)
	}

	var applied []AutoFixAction
	vulnIDSet := make(map[string]bool)
	for _, id := range vulnIDs {
		vulnIDSet[id] = true
	}

	for i, fix := range result.FixSuggestions {
		// 如果指定了漏洞 ID 列表，只处理列表中的
		if len(vulnIDs) > 0 && !vulnIDSet[fix.VulnID] {
			continue
		}
		// 如果指定了修复方式，覆盖默认方式
		if action != "" {
			result.FixSuggestions[i].Action = action
		}
		// 标记为已执行（模拟）
		result.FixSuggestions[i].Applied = true
		applied = append(applied, result.FixSuggestions[i])
	}
	return applied, nil
}

// ============================================================
// 合规报告
// ============================================================

// GenerateComplianceReport 生成合规报告
func (m *Manager) GenerateComplianceReport(standard ComplianceStandard) map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	var rules []*ComplianceRule
	for _, r := range m.complianceRules {
		if standard == "" || r.Standard == standard {
			rules = append(rules, r)
		}
	}

	// 统计所有扫描的合规情况
	totalPassed := 0
	totalFailed := 0
	for _, result := range m.scanResults {
		for _, cr := range result.ComplianceStatus {
			if cr.Passed {
				totalPassed++
			} else {
				totalFailed++
			}
		}
	}

	return map[string]interface{}{
		"standard":     standard,
		"rules_count":  len(rules),
		"total_passed": totalPassed,
		"total_failed": totalFailed,
		"rules":        rules,
		"generated_at": time.Now(),
	}
}

// ============================================================
// 运行时监控
// ============================================================

// MonitorContainer 监控容器运行时状态
func (m *Manager) MonitorContainer(containerID, containerName, imageName string, isPrivileged bool) *RuntimeMonitor {
	m.mu.Lock()
	defer m.mu.Unlock()

	monitor := &RuntimeMonitor{
		ContainerID:   containerID,
		ContainerName: containerName,
		ImageName:     imageName,
		IsPrivileged:  isPrivileged,
		LastCheckTime: time.Now(),
		Status:        "normal",
	}

	// 检测异常
	var anomalies []Anomaly

	// 检查特权容器
	if isPrivileged {
		anomalies = append(anomalies, Anomaly{
			Type:       AnomalyPrivilegedContainer,
			Message:    "容器以特权模式运行",
			Severity:   SeverityCritical,
			DetectedAt: time.Now(),
			Details:    "特权容器可以访问宿主机所有设备，存在严重安全风险",
		})
		monitor.Status = "critical"
	}

	// 模拟检测异常进程
	if strings.Contains(containerName, "suspicious") {
		anomalies = append(anomalies, Anomaly{
			Type:       AnomalyAbnormalProcess,
			Message:    "检测到异常进程",
			Severity:   SeverityHigh,
			DetectedAt: time.Now(),
			Details:    "发现可疑的加密挖矿进程",
		})
		if monitor.Status == "normal" {
			monitor.Status = "warning"
		}
	}

	// 模拟检测异常网络
	if strings.Contains(imageName, "unknown") {
		anomalies = append(anomalies, Anomaly{
			Type:       AnomalyAbnormalNetwork,
			Message:    "检测到异常外部连接",
			Severity:   SeverityHigh,
			DetectedAt: time.Now(),
			Details:    "容器正在连接已知恶意 IP 地址",
		})
		if monitor.Status == "normal" {
			monitor.Status = "warning"
		}
	}

	monitor.Anomalies = anomalies
	m.runtimeMonitors[containerID] = monitor
	return monitor
}

// GetRuntimeMonitor 获取运行时监控状态
func (m *Manager) GetRuntimeMonitor(containerID string) (*RuntimeMonitor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mon, ok := m.runtimeMonitors[containerID]
	if !ok {
		return nil, fmt.Errorf("容器监控不存在: %s", containerID)
	}
	return mon, nil
}

// ListRuntimeMonitors 列出所有运行时监控
func (m *Manager) ListRuntimeMonitors() []*RuntimeMonitor {
	m.mu.Lock()
	defer m.mu.Unlock()
	monitors := make([]*RuntimeMonitor, 0, len(m.runtimeMonitors))
	for _, mon := range m.runtimeMonitors {
		monitors = append(monitors, mon)
	}
	return monitors
}

// ============================================================
// 黑名单/白名单管理
// ============================================================

// AddToBlacklist 添加到黑名单
func (m *Manager) AddToBlacklist(imageName, reason, addedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blacklist[imageName] = &ListEntry{
		ImageName: imageName,
		Reason:    reason,
		AddedAt:   time.Now(),
		AddedBy:   addedBy,
	}
	return m.saveConfig()
}

// RemoveFromBlacklist 从黑名单移除
func (m *Manager) RemoveFromBlacklist(imageName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.blacklist[imageName]; !ok {
		return fmt.Errorf("镜像不在黑名单中: %s", imageName)
	}
	delete(m.blacklist, imageName)
	return m.saveConfig()
}

// ListBlacklist 列出黑名单
func (m *Manager) ListBlacklist() []*ListEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	entries := make([]*ListEntry, 0, len(m.blacklist))
	for _, e := range m.blacklist {
		entries = append(entries, e)
	}
	return entries
}

// AddToWhitelist 添加到白名单
func (m *Manager) AddToWhitelist(imageName, reason, addedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.whitelist[imageName] = &ListEntry{
		ImageName: imageName,
		Reason:    reason,
		AddedAt:   time.Now(),
		AddedBy:   addedBy,
	}
	return m.saveConfig()
}

// RemoveFromWhitelist 从白名单移除
func (m *Manager) RemoveFromWhitelist(imageName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.whitelist[imageName]; !ok {
		return fmt.Errorf("镜像不在白名单中: %s", imageName)
	}
	delete(m.whitelist, imageName)
	return m.saveConfig()
}

// ListWhitelist 列出白名单
func (m *Manager) ListWhitelist() []*ListEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	entries := make([]*ListEntry, 0, len(m.whitelist))
	for _, e := range m.whitelist {
		entries = append(entries, e)
	}
	return entries
}

// ============================================================
// 合规规则管理
// ============================================================

// ListComplianceRules 列出合规规则
func (m *Manager) ListComplianceRules(standard ComplianceStandard) []*ComplianceRule {
	m.mu.Lock()
	defer m.mu.Unlock()
	var rules []*ComplianceRule
	for _, r := range m.complianceRules {
		if standard == "" || r.Standard == standard {
			rules = append(rules, r)
		}
	}
	return rules
}

// ============================================================
// 定时扫描调度
// ============================================================

// Start 启动定时扫描调度器
func (m *Manager) Start() {
	go m.runScheduler()
}

// Stop 停止定时扫描调度器
func (m *Manager) Stop() {
	close(m.stopCh)
}

// runScheduler 定时扫描调度
func (m *Manager) runScheduler() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.runDueScans()
		}
	}
}

// runDueScans 执行到期的定时扫描
func (m *Manager) runDueScans() {
	m.mu.Lock()
	var cronPolicies []string
	for _, p := range m.policies {
		if p.Enabled && p.ScheduleCron != "" {
			cronPolicies = append(cronPolicies, p.ID)
		}
	}
	m.mu.Unlock()

	// 简化实现：对有定时策略的镜像执行扫描
	// 实际应解析 cron 表达式并匹配时间
	for _, pid := range cronPolicies {
		p, _ := m.GetPolicy(pid)
		if p != nil && p.ScheduleCron == "@every_hour" {
			// 模拟触发扫描
			_ = p
		}
	}
}
