package containerguardian

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// SeverityLevel 表示漏洞严重程度
type SeverityLevel int

const (
	// SeverityCritical 严重漏洞
	SeverityCritical SeverityLevel = iota
	// SeverityHigh 高危漏洞
	SeverityHigh
	// SeverityMedium 中危漏洞
	SeverityMedium
	// SeverityLow 低危漏洞
	SeverityLow
)

// String 返回严重程度的字符串表示
func (s SeverityLevel) String() string {
	switch s {
	case SeverityCritical:
		return "Critical"
	case SeverityHigh:
		return "High"
	case SeverityMedium:
		return "Medium"
	case SeverityLow:
		return "Low"
	default:
		return "Unknown"
	}
}

// Vulnerability 表示一个安全漏洞
type Vulnerability struct {
	ID          string        `json:"id"`
	CVE         string        `json:"cve"`
	Severity    SeverityLevel `json:"severity"`
	Package     string        `json:"package"`
	Version     string        `json:"version"`
	FixedIn     string        `json:"fixed_in"`
	Description string        `json:"description"`
}

// ScanResult 表示镜像扫描结果
type ScanResult struct {
	ImageName       string          `json:"image_name"`
	ScanTime        time.Time       `json:"scan_time"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	Score           float64         `json:"score"`
	IsClean         bool            `json:"is_clean"`
}

// ResourceLimits 表示容器资源限制
type ResourceLimits struct {
	CPUQuota    int64  `json:"cpu_quota"`    // CPU配额（微秒）
	MemoryLimit int64  `json:"memory_limit"` // 内存限制（字节）
	PidsLimit   int64  `json:"pids_limit"`   // 进程数限制
	IOReadBPS   int64  `json:"io_read_bps"`  // 磁盘读取速率限制
	IOWriteBPS  int64  `json:"io_write_bps"` // 磁盘写入速率限制
}

// NetworkPolicy 表示网络隔离策略
type NetworkPolicy struct {
	Name         string   `json:"name"`
	AllowIngress bool     `json:"allow_ingress"`
	AllowEgress  bool     `json:"allow_egress"`
	AllowedPorts []int    `json:"allowed_ports"`
	BlockedPorts []int    `json:"blocked_ports"`
	AllowedCIDRs []string `json:"allowed_cidrs"`
	IsActive     bool     `json:"is_active"`
}

// SecurityPolicy 表示安全策略
type SecurityPolicy struct {
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	MaxSeverity    SeverityLevel   `json:"max_severity"`    // 允许的最大漏洞等级
	EnforceLimits  bool            `json:"enforce_limits"`  // 是否强制执行资源限制
	RequireScan    bool            `json:"require_scan"`    // 是否要求镜像扫描
	AutoRemediate  bool            `json:"auto_remediate"`  // 是否自动修复
	ResourceLimits *ResourceLimits `json:"resource_limits"` // 默认资源限制
	IsActive       bool            `json:"is_active"`
}

// AuditEntry 表示审计日志条目
type AuditEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	ContainerID string    `json:"container_id"`
	Action      string    `json:"action"`
	Details     string    `json:"details"`
	Severity    string    `json:"severity"`
	Success     bool      `json:"success"`
}

// ContainerStatus 表示容器运行时状态
type ContainerStatus struct {
	ContainerID string        `json:"container_id"`
	ImageName   string        `json:"image_name"`
	Running     bool          `json:"running"`
	CPUUsage    float64       `json:"cpu_usage"`
	MemoryUsage int64         `json:"memory_usage"`
	NetworkIO   int64         `json:"network_io"`
	Anomalies   []string      `json:"anomalies"`
	Uptime      time.Duration `json:"uptime"`
}

// ContainerGuardian 容器安全卫士主结构体
type ContainerGuardian struct {
	mu              sync.RWMutex
	policies        map[string]*SecurityPolicy
	networkPolicies map[string]*NetworkPolicy
	resourceLimits  map[string]*ResourceLimits
	auditLog        []AuditEntry
	scanResults     map[string]*ScanResult
	containers      map[string]*ContainerStatus
	vulnDB          map[string][]Vulnerability // 内置漏洞数据库
}

// New 创建新的ContainerGuardian实例
func New() *ContainerGuardian {
	cg := &ContainerGuardian{
		policies:        make(map[string]*SecurityPolicy),
		networkPolicies: make(map[string]*NetworkPolicy),
		resourceLimits:  make(map[string]*ResourceLimits),
		auditLog:        make([]AuditEntry, 0),
		scanResults:     make(map[string]*ScanResult),
		containers:      make(map[string]*ContainerStatus),
		vulnDB:          make(map[string][]Vulnerability),
	}
	cg.initVulnDB()
	return cg
}

// initVulnDB 初始化内置漏洞数据库（模拟）
func (cg *ContainerGuardian) initVulnDB() {
	cg.vulnDB["nginx:1.20"] = []Vulnerability{
		{ID: "VULN-001", CVE: "CVE-2021-23017", Severity: SeverityHigh, Package: "nginx", Version: "1.20.0", FixedIn: "1.20.1", Description: "DNS解析器漏洞"},
		{ID: "VULN-002", CVE: "CVE-2021-23018", Severity: SeverityMedium, Package: "nginx", Version: "1.20.0", FixedIn: "1.20.1", Description: "HTTP/2内存泄漏"},
	}
	cg.vulnDB["redis:6.0"] = []Vulnerability{
		{ID: "VULN-003", CVE: "CVE-2021-32625", Severity: SeverityCritical, Package: "redis", Version: "6.0.0", FixedIn: "6.0.13", Description: "整数溢出导致堆溢出"},
	}
	cg.vulnDB["mysql:5.7"] = []Vulnerability{
		{ID: "VULN-004", CVE: "CVE-2021-2154", Severity: SeverityHigh, Package: "mysql", Version: "5.7.0", FixedIn: "5.7.34", Description: "权限提升漏洞"},
		{ID: "VULN-005", CVE: "CVE-2021-2160", Severity: SeverityMedium, Package: "mysql", Version: "5.7.0", FixedIn: "5.7.34", Description: "拒绝服务漏洞"},
	}
	cg.vulnDB["ubuntu:20.04"] = []Vulnerability{
		{ID: "VULN-006", CVE: "CVE-2021-3493", Severity: SeverityHigh, Package: "overlayfs", Version: "20.04", FixedIn: "20.04.2", Description: "OverlayFS权限提升"},
	}
}

// ScanImage 扫描容器镜像漏洞
func (cg *ContainerGuardian) ScanImage(imageName string) (*ScanResult, error) {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	if imageName == "" {
		return nil, fmt.Errorf("镜像名称不能为空")
	}

	result := &ScanResult{
		ImageName:       imageName,
		ScanTime:        time.Now(),
		Vulnerabilities: make([]Vulnerability, 0),
		IsClean:         true,
	}

	// 从内置漏洞数据库查询
	if vulns, ok := cg.vulnDB[imageName]; ok {
		result.Vulnerabilities = vulns
		result.IsClean = false
	}

	// 计算安全评分
	result.Score = cg.calculateScanScore(result.Vulnerabilities)

	// 保存扫描结果
	cg.scanResults[imageName] = result

	// 记录审计日志
	cg.addAuditEntry("", "ScanImage",
		fmt.Sprintf("扫描镜像: %s, 发现漏洞: %d, 评分: %.2f", imageName, len(result.Vulnerabilities), result.Score),
		"INFO", true)

	return result, nil
}

// calculateScanScore 计算扫描评分
func (cg *ContainerGuardian) calculateScanScore(vulns []Vulnerability) float64 {
	if len(vulns) == 0 {
		return 100.0
	}

	score := 100.0
	for _, v := range vulns {
		switch v.Severity {
		case SeverityCritical:
			score -= 25.0
		case SeverityHigh:
			score -= 15.0
		case SeverityMedium:
			score -= 8.0
		case SeverityLow:
			score -= 3.0
		}
	}

	if score < 0 {
		score = 0
	}
	return score
}

// MonitorContainer 监控容器运行时状态，检测异常行为
func (cg *ContainerGuardian) MonitorContainer(containerID string) (*ContainerStatus, error) {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	if containerID == "" {
		return nil, fmt.Errorf("容器ID不能为空")
	}

	// 模拟获取容器状态
	status := &ContainerStatus{
		ContainerID: containerID,
		ImageName:   "nginx:1.20",
		Running:     true,
		CPUUsage:    45.5,
		MemoryUsage: 512 * 1024 * 1024, // 512MB
		NetworkIO:   1024 * 1024,        // 1MB
		Anomalies:   make([]string, 0),
		Uptime:      2 * time.Hour,
	}

	// 检测异常行为
	anomalies := cg.detectAnomalies(status)
	status.Anomalies = anomalies

	// 保存状态
	cg.containers[containerID] = status

	// 记录审计日志
	severity := "INFO"
	if len(anomalies) > 0 {
		severity = "WARNING"
	}
	cg.addAuditEntry(containerID, "MonitorContainer",
		fmt.Sprintf("监控容器: %s, CPU: %.1f%%, 内存: %dMB, 异常: %d", containerID, status.CPUUsage, status.MemoryUsage/(1024*1024), len(anomalies)),
		severity, true)

	return status, nil
}

// detectAnomalies 检测容器异常行为
func (cg *ContainerGuardian) detectAnomalies(status *ContainerStatus) []string {
	anomalies := make([]string, 0)

	// CPU使用率过高
	if status.CPUUsage > 90.0 {
		anomalies = append(anomalies, "CPU使用率异常高")
	}

	// 内存使用过高（超过1GB）
	if status.MemoryUsage > 1024*1024*1024 {
		anomalies = append(anomalies, "内存使用异常高")
	}

	// 网络IO异常
	if status.NetworkIO > 100*1024*1024 { // 100MB
		anomalies = append(anomalies, "网络IO异常高")
	}

	return anomalies
}

// SetResourceLimits 设置容器资源限制
func (cg *ContainerGuardian) SetResourceLimits(containerID string, limits *ResourceLimits) error {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	if containerID == "" {
		return fmt.Errorf("容器ID不能为空")
	}

	if limits == nil {
		return fmt.Errorf("资源限制不能为空")
	}

	// 验证参数合理性
	if limits.CPUQuota < 0 || limits.MemoryLimit < 0 || limits.PidsLimit < 0 {
		return fmt.Errorf("资源限制值不能为负数")
	}

	cg.resourceLimits[containerID] = limits

	// 记录审计日志
	cg.addAuditEntry(containerID, "SetResourceLimits",
		fmt.Sprintf("设置资源限制: CPU=%dμs, 内存=%dMB, 进程=%d", limits.CPUQuota, limits.MemoryLimit/(1024*1024), limits.PidsLimit),
		"INFO", true)

	return nil
}

// GetResourceLimits 获取容器资源限制
func (cg *ContainerGuardian) GetResourceLimits(containerID string) (*ResourceLimits, error) {
	cg.mu.RLock()
	defer cg.mu.RUnlock()

	if containerID == "" {
		return nil, fmt.Errorf("容器ID不能为空")
	}

	limits, ok := cg.resourceLimits[containerID]
	if !ok {
		return nil, fmt.Errorf("未找到容器 %s 的资源限制", containerID)
	}

	return limits, nil
}

// AddNetworkPolicy 添加网络隔离策略
func (cg *ContainerGuardian) AddNetworkPolicy(containerID string, policy *NetworkPolicy) error {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	if containerID == "" {
		return fmt.Errorf("容器ID不能为空")
	}

	if policy == nil {
		return fmt.Errorf("网络策略不能为空")
	}

	if policy.Name == "" {
		return fmt.Errorf("策略名称不能为空")
	}

	policy.IsActive = true
	cg.networkPolicies[containerID] = policy

	// 记录审计日志
	cg.addAuditEntry(containerID, "AddNetworkPolicy",
		fmt.Sprintf("添加网络策略: %s, 入站=%v, 出站=%v", policy.Name, policy.AllowIngress, policy.AllowEgress),
		"INFO", true)

	return nil
}

// RemoveNetworkPolicy 移除网络隔离策略
func (cg *ContainerGuardian) RemoveNetworkPolicy(containerID string) error {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	if containerID == "" {
		return fmt.Errorf("容器ID不能为空")
	}

	if _, ok := cg.networkPolicies[containerID]; !ok {
		return fmt.Errorf("未找到容器 %s 的网络策略", containerID)
	}

	delete(cg.networkPolicies, containerID)

	// 记录审计日志
	cg.addAuditEntry(containerID, "RemoveNetworkPolicy",
		fmt.Sprintf("移除网络策略: %s", containerID),
		"INFO", true)

	return nil
}

// CreatePolicy 创建安全策略
func (cg *ContainerGuardian) CreatePolicy(policy *SecurityPolicy) error {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	if policy == nil {
		return fmt.Errorf("安全策略不能为空")
	}

	if policy.Name == "" {
		return fmt.Errorf("策略名称不能为空")
	}

	if _, ok := cg.policies[policy.Name]; ok {
		return fmt.Errorf("策略 %s 已存在", policy.Name)
	}

	policy.IsActive = true
	cg.policies[policy.Name] = policy

	// 记录审计日志
	cg.addAuditEntry("", "CreatePolicy",
		fmt.Sprintf("创建安全策略: %s, 最大漏洞等级: %s", policy.Name, policy.MaxSeverity),
		"INFO", true)

	return nil
}

// ApplyPolicy 将安全策略应用到容器
func (cg *ContainerGuardian) ApplyPolicy(containerID, policyName string) error {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	if containerID == "" {
		return fmt.Errorf("容器ID不能为空")
	}

	if policyName == "" {
		return fmt.Errorf("策略名称不能为空")
	}

	policy, ok := cg.policies[policyName]
	if !ok {
		return fmt.Errorf("未找到策略: %s", policyName)
	}

	if !policy.IsActive {
		return fmt.Errorf("策略 %s 未激活", policyName)
	}

	// 如果策略包含资源限制，应用到容器
	if policy.EnforceLimits && policy.ResourceLimits != nil {
		cg.resourceLimits[containerID] = policy.ResourceLimits
	}

	// 记录审计日志
	cg.addAuditEntry(containerID, "ApplyPolicy",
		fmt.Sprintf("应用安全策略: %s 到容器: %s", policyName, containerID),
		"INFO", true)

	return nil
}

// GetAuditLog 获取容器审计日志
func (cg *ContainerGuardian) GetAuditLog(containerID string) []AuditEntry {
	cg.mu.RLock()
	defer cg.mu.RUnlock()

	if containerID == "" {
		// 返回所有日志
		result := make([]AuditEntry, len(cg.auditLog))
		copy(result, cg.auditLog)
		return result
	}

	// 过滤特定容器的日志
	result := make([]AuditEntry, 0)
	for _, entry := range cg.auditLog {
		if entry.ContainerID == containerID {
			result = append(result, entry)
		}
	}
	return result
}

// GetSecurityScore 获取容器安全评分
func (cg *ContainerGuardian) GetSecurityScore(containerID string) (float64, error) {
	cg.mu.RLock()
	defer cg.mu.RUnlock()

	if containerID == "" {
		return 0, fmt.Errorf("容器ID不能为空")
	}

	score := 100.0

	// 检查是否有扫描结果
	if scanResult, ok := cg.scanResults[containerID]; ok {
		score = scanResult.Score
	}

	// 检查网络策略
	if policy, ok := cg.networkPolicies[containerID]; ok {
		if policy.AllowIngress && policy.AllowEgress {
			score -= 10.0 // 允许双向通信扣分
		}
		if len(policy.BlockedPorts) > 0 {
			score += 5.0 // 有端口封锁加分
		}
	}

	// 检查资源限制
	if limits, ok := cg.resourceLimits[containerID]; ok {
		if limits.MemoryLimit > 0 && limits.CPUQuota > 0 {
			score += 5.0 // 有资源限制加分
		}
	}

	// 检查运行时异常
	if status, ok := cg.containers[containerID]; ok {
		score -= float64(len(status.Anomalies)) * 10.0
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score, nil
}

// GetRemediation 获取自动修复建议
func (cg *ContainerGuardian) GetRemediation(containerID string) ([]string, error) {
	cg.mu.RLock()
	defer cg.mu.RUnlock()

	if containerID == "" {
		return nil, fmt.Errorf("容器ID不能为空")
	}

	suggestions := make([]string, 0)

	// 基于扫描结果的修复建议
	if scanResult, ok := cg.scanResults[containerID]; ok {
		for _, vuln := range scanResult.Vulnerabilities {
			if vuln.FixedIn != "" {
				suggestions = append(suggestions,
					fmt.Sprintf("升级 %s 从 %s 到 %s 以修复 %s", vuln.Package, vuln.Version, vuln.FixedIn, vuln.CVE))
			}
		}
	}

	// 基于网络策略的建议
	if _, ok := cg.networkPolicies[containerID]; !ok {
		suggestions = append(suggestions, "建议添加网络隔离策略以增强安全性")
	}

	// 基于资源限制的建议
	if _, ok := cg.resourceLimits[containerID]; !ok {
		suggestions = append(suggestions, "建议设置资源限制以防止资源耗尽攻击")
	}

	// 基于运行时状态的建议
	if status, ok := cg.containers[containerID]; ok {
		if len(status.Anomalies) > 0 {
			suggestions = append(suggestions, fmt.Sprintf("检测到 %d 个异常行为，建议进行详细排查", len(status.Anomalies)))
		}
	}

	return suggestions, nil
}

// addAuditEntry 添加审计日志条目（内部方法，需要持有锁）
func (cg *ContainerGuardian) addAuditEntry(containerID, action, details, severity string, success bool) {
	entry := AuditEntry{
		Timestamp:   time.Now(),
		ContainerID: containerID,
		Action:      action,
		Details:     details,
		Severity:    severity,
		Success:     success,
	}
	cg.auditLog = append(cg.auditLog, entry)
}

// GetScanResult 获取镜像扫描结果
func (cg *ContainerGuardian) GetScanResult(imageName string) (*ScanResult, error) {
	cg.mu.RLock()
	defer cg.mu.RUnlock()

	if imageName == "" {
		return nil, fmt.Errorf("镜像名称不能为空")
	}

	result, ok := cg.scanResults[imageName]
	if !ok {
		return nil, fmt.Errorf("未找到镜像 %s 的扫描结果", imageName)
	}

	return result, nil
}

// ListPolicies 列出所有安全策略
func (cg *ContainerGuardian) ListPolicies() []*SecurityPolicy {
	cg.mu.RLock()
	defer cg.mu.RUnlock()

	policies := make([]*SecurityPolicy, 0, len(cg.policies))
	for _, p := range cg.policies {
		policies = append(policies, p)
	}
	return policies
}

// DeletePolicy 删除安全策略
func (cg *ContainerGuardian) DeletePolicy(policyName string) error {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	if policyName == "" {
		return fmt.Errorf("策略名称不能为空")
	}

	if _, ok := cg.policies[policyName]; !ok {
		return fmt.Errorf("未找到策略: %s", policyName)
	}

	delete(cg.policies, policyName)

	// 记录审计日志
	cg.addAuditEntry("", "DeletePolicy",
		fmt.Sprintf("删除安全策略: %s", policyName),
		"INFO", true)

	return nil
}

// GetVulnerabilityStats 获取漏洞统计信息
func (cg *ContainerGuardian) GetVulnerabilityStats(imageName string) (map[SeverityLevel]int, error) {
	cg.mu.RLock()
	defer cg.mu.RUnlock()

	if imageName == "" {
		return nil, fmt.Errorf("镜像名称不能为空")
	}

	scanResult, ok := cg.scanResults[imageName]
	if !ok {
		return nil, fmt.Errorf("未找到镜像 %s 的扫描结果", imageName)
	}

	stats := make(map[SeverityLevel]int)
	for _, vuln := range scanResult.Vulnerabilities {
		stats[vuln.Severity]++
	}

	return stats, nil
}

// AddVulnerability 添加自定义漏洞到数据库（用于测试或更新）
func (cg *ContainerGuardian) AddVulnerability(imageName string, vuln Vulnerability) {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	cg.vulnDB[imageName] = append(cg.vulnDB[imageName], vuln)
}

// FormatReport 格式化容器安全报告
func (cg *ContainerGuardian) FormatReport(containerID string) string {
	cg.mu.RLock()
	defer cg.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== 容器安全报告: %s ===\n", containerID))

	// 安全评分
	score, _ := cg.GetSecurityScore(containerID)
	sb.WriteString(fmt.Sprintf("安全评分: %.2f/100\n", score))

	// 扫描结果
	if scanResult, ok := cg.scanResults[containerID]; ok {
		sb.WriteString(fmt.Sprintf("漏洞数量: %d\n", len(scanResult.Vulnerabilities)))
		sb.WriteString(fmt.Sprintf("扫描评分: %.2f\n", scanResult.Score))
	}

	// 资源限制
	if limits, ok := cg.resourceLimits[containerID]; ok {
		sb.WriteString(fmt.Sprintf("CPU限制: %dμs\n", limits.CPUQuota))
		sb.WriteString(fmt.Sprintf("内存限制: %dMB\n", limits.MemoryLimit/(1024*1024)))
	}

	// 网络策略
	if policy, ok := cg.networkPolicies[containerID]; ok {
		sb.WriteString(fmt.Sprintf("网络策略: %s (入站: %v, 出站: %v)\n", policy.Name, policy.AllowIngress, policy.AllowEgress))
	}

	return sb.String()
}
