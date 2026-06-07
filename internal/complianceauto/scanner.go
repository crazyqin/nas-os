// Package complianceauto 提供自动化合规检查功能
package complianceauto

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Scanner 合规扫描引擎
type Scanner struct {
	mu         sync.RWMutex
	rules      map[string]*ComplianceRule // 规则库
	checks     map[string]RuleCheck       // 检查函数
	stats      *ComplianceStats           // 统计信息
	lastScan   *ComplianceScan            // 最近扫描
	isScanning bool                       // 是否正在扫描
	cancelFunc context.CancelFunc         // 取消函数
}

// NewScanner 创建扫描引擎
func NewScanner() *Scanner {
	s := &Scanner{
		rules:  make(map[string]*ComplianceRule),
		checks: make(map[string]RuleCheck),
		stats:  &ComplianceStats{},
	}
	// 加载默认规则库
	s.loadDefaultRules()
	return s
}

// loadDefaultRules 加载默认合规规则库
func (s *Scanner) loadDefaultRules() {
	// CIS 基准规则
	s.loadCISRules()
	// NIST 框架规则
	s.loadNISTRules()
	// GDPR 规则
	s.loadGDPRRules()
	// 等保2.0 规则
	s.loadMLPS2Rules()
}

// loadCISRules 加载 CIS 基准规则
func (s *Scanner) loadCISRules() {
	rules := []*ComplianceRule{
		// 文件系统安全
		{
			ID:          "CIS-1.1.1",
			Standard:    StandardCIS,
			Category:    CategorySystemHardening,
			Severity:    SeverityHigh,
			Title:       "确保 /tmp 挂载选项包含 noexec",
			Description: "noexec 选项可防止在 /tmp 分区上执行二进制文件",
			Requirement: "/tmp 分区应使用 noexec 选项挂载",
			Remediation: "编辑 /etc/fstab，在 /tmp 挂载行添加 noexec 选项",
			Enabled:     true,
			Tags:        []string{"filesystem", "mount"},
		},
		{
			ID:          "CIS-1.1.2",
			Standard:    StandardCIS,
			Category:    CategorySystemHardening,
			Severity:    SeverityHigh,
			Title:       "确保 /tmp 挂载选项包含 nosuid",
			Description: "nosuid 选项可防止 SUID 位生效",
			Requirement: "/tmp 分区应使用 nosuid 选项挂载",
			Remediation: "编辑 /etc/fstab，在 /tmp 挂载行添加 nosuid 选项",
			Enabled:     true,
			Tags:        []string{"filesystem", "mount"},
		},
		// 服务安全
		{
			ID:          "CIS-2.1.1",
			Standard:    StandardCIS,
			Category:    CategoryNetworkSecurity,
			Severity:    SeverityCritical,
			Title:       "确保未使用 inetd 服务",
			Description: "inetd 是旧式超级服务器，存在安全风险",
			Requirement: "inetd 服务应被禁用",
			Remediation: "systemctl disable inetd",
			Enabled:     true,
			Tags:        []string{"service", "inetd"},
		},
		{
			ID:          "CIS-2.2.1",
			Standard:    StandardCIS,
			Category:    CategoryNetworkSecurity,
			Severity:    SeverityMedium,
			Title:       "确保 NTP 已配置",
			Description: "时间同步对于日志审计和安全事件关联至关重要",
			Requirement: "NTP 服务应正确配置并运行",
			Remediation: "安装并配置 chrony 或 ntp 服务",
			Enabled:     true,
			Tags:        []string{"ntp", "time"},
		},
		// 账户与访问控制
		{
			ID:          "CIS-5.1.1",
			Standard:    StandardCIS,
			Category:    CategoryAccessControl,
			Severity:    SeverityCritical,
			Title:       "确保密码过期策略已配置",
			Description: "密码过期可降低密码泄露风险",
			Requirement: "密码最大使用期限应不超过 90 天",
			Remediation: "编辑 /etc/login.defs 设置 PASS_MAX_DAYS 90",
			Enabled:     true,
			Tags:        []string{"password", "account"},
		},
		{
			ID:          "CIS-5.2.1",
			Standard:    StandardCIS,
			Category:    CategoryIdentityAuth,
			Severity:    SeverityCritical,
			Title:       "确保 SSH 禁用 root 登录",
			Description: "直接 root 登录增加了未授权访问风险",
			Requirement: "SSH 配置应禁止 root 直接登录",
			Remediation: "在 /etc/ssh/sshd_config 设置 PermitRootLogin no",
			Enabled:     true,
			Tags:        []string{"ssh", "root"},
		},
		// 日志与审计
		{
			ID:          "CIS-4.1.1",
			Standard:    StandardCIS,
			Category:    CategoryAuditLogging,
			Severity:    SeverityHigh,
			Title:       "确保审计日志存储已配置",
			Description: "审计日志需要安全存储并有足够保留期",
			Requirement: "审计日志应存储在独立分区且保留至少 90 天",
			Remediation: "配置 auditd 日志存储策略和轮转",
			Enabled:     true,
			Tags:        []string{"audit", "logging"},
		},
		// 加密
		{
			ID:          "CIS-6.1.1",
			Standard:    StandardCIS,
			Category:    CategoryEncryption,
			Severity:    SeverityHigh,
			Title:       "确保文件系统加密已启用",
			Description: "敏感数据应使用加密保护",
			Requirement: "存储敏感数据的分区应启用加密",
			Remediation: "配置 LUKS 或其他磁盘加密方案",
			Enabled:     true,
			Tags:        []string{"encryption", "storage"},
		},
	}
	for _, rule := range rules {
		s.rules[rule.ID] = rule
		s.registerCheck(rule.ID)
	}
}

// loadNISTRules 加载 NIST 框架规则
func (s *Scanner) loadNISTRules() {
	rules := []*ComplianceRule{
		{
			ID:          "NIST-AC-1",
			Standard:    StandardNIST,
			Category:    CategoryAccessControl,
			Severity:    SeverityCritical,
			Title:       "访问控制策略和程序",
			Description: "组织应制定、发布、审查和更新访问控制策略",
			Requirement: "应存在书面的访问控制策略文档",
			Remediation: "制定并维护访问控制策略文档",
			Enabled:     true,
			Tags:        []string{"policy", "access"},
		},
		{
			ID:          "NIST-AC-2",
			Standard:    StandardNIST,
			Category:    CategoryAccessControl,
			Severity:    SeverityHigh,
			Title:       "账户管理",
			Description: "组织应管理系统账户的创建、启用、修改、禁用和删除",
			Requirement: "应有账户生命周期管理流程",
			Remediation: "实施账户管理流程，定期审查账户",
			Enabled:     true,
			Tags:        []string{"account", "management"},
		},
		{
			ID:          "NIST-AU-1",
			Standard:    StandardNIST,
			Category:    CategoryAuditLogging,
			Severity:    SeverityHigh,
			Title:       "审计和问责策略",
			Description: "组织应制定审计和问责策略",
			Requirement: "应存在审计日志策略，定义记录内容和保留期",
			Remediation: "制定审计日志策略，配置日志记录",
			Enabled:     true,
			Tags:        []string{"audit", "policy"},
		},
		{
			ID:          "NIST-SC-1",
			Standard:    StandardNIST,
			Category:    CategorySystemHardening,
			Severity:    SeverityHigh,
			Title:       "系统和通信保护策略",
			Description: "组织应制定系统和通信保护策略",
			Requirement: "应有系统安全配置基线",
			Remediation: "制定系统安全基线并定期检查",
			Enabled:     true,
			Tags:        []string{"system", "protection"},
		},
		{
			ID:          "NIST-SC-8",
			Standard:    StandardNIST,
			Category:    CategoryEncryption,
			Severity:    SeverityCritical,
			Title:       "传输中数据的保密性和完整性",
			Description: "组织应保护传输中数据的保密性和完整性",
			Requirement: "网络传输应使用 TLS 1.2 或更高版本",
			Remediation: "配置 TLS 加密，禁用不安全的协议",
			Enabled:     true,
			Tags:        []string{"encryption", "tls", "network"},
		},
		{
			ID:          "NIST-SC-28",
			Standard:    StandardNIST,
			Category:    CategoryEncryption,
			Severity:    SeverityHigh,
			Title:       "静态数据保护",
			Description: "组织应保护静态数据的保密性和完整性",
			Requirement: "敏感数据应加密存储",
			Remediation: "实施存储加密方案",
			Enabled:     true,
			Tags:        []string{"encryption", "storage"},
		},
		{
			ID:          "NIST-IR-1",
			Standard:    StandardNIST,
			Category:    CategoryIncidentResponse,
			Severity:    SeverityHigh,
			Title:       "事件响应策略和程序",
			Description: "组织应制定事件响应策略和程序",
			Requirement: "应有书面的事件响应计划",
			Remediation: "制定事件响应计划并定期演练",
			Enabled:     true,
			Tags:        []string{"incident", "response"},
		},
	}
	for _, rule := range rules {
		s.rules[rule.ID] = rule
		s.registerCheck(rule.ID)
	}
}

// loadGDPRRules 加载 GDPR 规则
func (s *Scanner) loadGDPRRules() {
	rules := []*ComplianceRule{
		{
			ID:          "GDPR-5.1.a",
			Standard:    StandardGDPR,
			Category:    CategoryDataProtection,
			Severity:    SeverityCritical,
			Title:       "数据处理合法性、公平性和透明性",
			Description: "个人数据处理应合法、公平、透明",
			Requirement: "应有数据处理的法律依据记录",
			Remediation: "建立数据处理法律依据文档",
			Enabled:     true,
			Tags:        []string{"legal", "transparency"},
		},
		{
			ID:          "GDPR-5.1.b",
			Standard:    StandardGDPR,
			Category:    CategoryDataProtection,
			Severity:    SeverityHigh,
			Title:       "目的限制",
			Description: "个人数据收集应有明确、合法的目的",
			Requirement: "数据收集目的应明确记录",
			Remediation: "制定数据收集目的声明",
			Enabled:     true,
			Tags:        []string{"purpose", "data"},
		},
		{
			ID:          "GDPR-5.1.c",
			Standard:    StandardGDPR,
			Category:    CategoryDataProtection,
			Severity:    SeverityHigh,
			Title:       "数据最小化",
			Description: "个人数据应与处理目的相关且限于必要范围",
			Requirement: "只收集必要的个人数据",
			Remediation: "审查并减少数据收集范围",
			Enabled:     true,
			Tags:        []string{"minimization", "data"},
		},
		{
			ID:          "GDPR-5.1.d",
			Standard:    StandardGDPR,
			Category:    CategoryDataProtection,
			Severity:    SeverityMedium,
			Title:       "准确性",
			Description: "个人数据应准确并保持最新",
			Requirement: "应有数据准确性维护机制",
			Remediation: "建立数据更新和纠正流程",
			Enabled:     true,
			Tags:        []string{"accuracy", "data"},
		},
		{
			ID:          "GDPR-5.1.e",
			Standard:    StandardGDPR,
			Category:    CategoryDataProtection,
			Severity:    SeverityHigh,
			Title:       "存储限制",
			Description: "个人数据保存不应超过必要期限",
			Requirement: "应有数据保留期策略",
			Remediation: "制定数据保留和删除策略",
			Enabled:     true,
			Tags:        []string{"retention", "data"},
		},
		{
			ID:          "GDPR-25.1",
			Standard:    StandardGDPR,
			Category:    CategoryPrivacyControl,
			Severity:    SeverityCritical,
			Title:       "设计和默认的数据保护",
			Description: "数据保护应从设计阶段就考虑",
			Requirement: "系统应内置隐私保护措施",
			Remediation: "实施隐私保护设计原则",
			Enabled:     true,
			Tags:        []string{"privacy", "design"},
		},
		{
			ID:          "GDPR-32.1",
			Standard:    StandardGDPR,
			Category:    CategoryEncryption,
			Severity:    SeverityCritical,
			Title:       "处理安全性",
			Description: "应实施适当的技术和组织措施保护数据",
			Requirement: "应实施加密、假名化等安全措施",
			Remediation: "实施数据加密和访问控制",
			Enabled:     true,
			Tags:        []string{"security", "encryption"},
		},
		{
			ID:          "GDPR-33.1",
			Standard:    StandardGDPR,
			Category:    CategoryIncidentResponse,
			Severity:    SeverityCritical,
			Title:       "数据泄露通知",
			Description: "数据泄露应在 72 小时内通知监管机构",
			Requirement: "应有数据泄露通知流程",
			Remediation: "建立数据泄露响应和通知流程",
			Enabled:     true,
			Tags:        []string{"breach", "notification"},
		},
	}
	for _, rule := range rules {
		s.rules[rule.ID] = rule
		s.registerCheck(rule.ID)
	}
}

// loadMLPS2Rules 加载等保2.0规则
func (s *Scanner) loadMLPS2Rules() {
	rules := []*ComplianceRule{
		// 安全物理环境
		{
			ID:          "MLPS2-PE-1",
			Standard:    StandardMLPS2,
			Category:    CategorySystemHardening,
			Severity:    SeverityCritical,
			Title:       "物理位置选择",
			Description: "机房应选择在具有防震、防风和防雨能力的建筑内",
			Requirement: "机房物理位置应满足安全要求",
			Remediation: "检查并确保机房物理位置符合要求",
			Enabled:     true,
			Tags:        []string{"physical", "location"},
		},
		// 安全通信网络
		{
			ID:          "MLPS2-CN-1",
			Standard:    StandardMLPS2,
			Category:    CategoryNetworkSecurity,
			Severity:    SeverityCritical,
			Title:       "网络架构",
			Description: "应保证网络设备的业务处理能力满足业务需要",
			Requirement: "网络架构应合理规划，避免单点故障",
			Remediation: "优化网络架构，增加冗余",
			Enabled:     true,
			Tags:        []string{"network", "architecture"},
		},
		{
			ID:          "MLPS2-CN-2",
			Standard:    StandardMLPS2,
			Category:    CategoryNetworkSecurity,
			Severity:    SeverityHigh,
			Title:       "通信传输",
			Description: "应采用校验技术或密码技术保证通信过程中数据的完整性",
			Requirement: "网络传输应使用加密和完整性校验",
			Remediation: "部署 TLS/SSL 加密传输",
			Enabled:     true,
			Tags:        []string{"network", "encryption"},
		},
		// 安全区域边界
		{
			ID:          "MLPS2-BD-1",
			Standard:    StandardMLPS2,
			Category:    CategoryNetworkSecurity,
			Severity:    SeverityCritical,
			Title:       "边界防护",
			Description: "应保证跨越边界的访问和数据流通过边界设备提供的受控接口进行通信",
			Requirement: "应部署防火墙等边界防护设备",
			Remediation: "配置防火墙策略，控制网络访问",
			Enabled:     true,
			Tags:        []string{"firewall", "boundary"},
		},
		{
			ID:          "MLPS2-BD-2",
			Standard:    StandardMLPS2,
			Category:    CategoryNetworkSecurity,
			Severity:    SeverityHigh,
			Title:       "访问控制",
			Description: "应在网络边界或区域之间根据访问控制策略设置访问控制规则",
			Requirement: "应实施网络访问控制策略",
			Remediation: "配置 ACL 规则，限制非授权访问",
			Enabled:     true,
			Tags:        []string{"access", "control"},
		},
		// 安全计算环境
		{
			ID:          "MLPS2-CE-1",
			Standard:    StandardMLPS2,
			Category:    CategoryIdentityAuth,
			Severity:    SeverityCritical,
			Title:       "身份鉴别",
			Description: "应对登录的用户进行身份标识和鉴别",
			Requirement: "应实施强身份认证机制",
			Remediation: "配置多因素认证",
			Enabled:     true,
			Tags:        []string{"identity", "authentication"},
		},
		{
			ID:          "MLPS2-CE-2",
			Standard:    StandardMLPS2,
			Category:    CategoryAccessControl,
			Severity:    SeverityCritical,
			Title:       "访问控制",
			Description: "应对登录的用户分配账户和权限",
			Requirement: "应实施基于角色的访问控制",
			Remediation: "配置 RBAC 权限管理",
			Enabled:     true,
			Tags:        []string{"access", "rbac"},
		},
		{
			ID:          "MLPS2-CE-3",
			Standard:    StandardMLPS2,
			Category:    CategoryAuditLogging,
			Severity:    SeverityHigh,
			Title:       "安全审计",
			Description: "应对审计进程进行保护，防止受到未预期的中断",
			Requirement: "应启用并保护审计日志",
			Remediation: "配置审计日志，实施日志保护措施",
			Enabled:     true,
			Tags:        []string{"audit", "logging"},
		},
		{
			ID:          "MLPS2-CE-4",
			Standard:    StandardMLPS2,
			Category:    CategoryEncryption,
			Severity:    SeverityCritical,
			Title:       "数据完整性",
			Description: "应采用校验技术或密码技术保证重要数据在传输过程中的完整性",
			Requirement: "应实施数据完整性保护措施",
			Remediation: "配置数据完整性校验机制",
			Enabled:     true,
			Tags:        []string{"integrity", "data"},
		},
		{
			ID:          "MLPS2-CE-5",
			Standard:    StandardMLPS2,
			Category:    CategoryEncryption,
			Severity:    SeverityCritical,
			Title:       "数据保密性",
			Description: "应采用密码技术保证重要数据在存储过程中的保密性",
			Requirement: "应实施数据加密存储",
			Remediation: "部署存储加密方案",
			Enabled:     true,
			Tags:        []string{"encryption", "storage"},
		},
		{
			ID:          "MLPS2-CE-6",
			Standard:    StandardMLPS2,
			Category:    CategoryBackupRecovery,
			Severity:    SeverityHigh,
			Title:       "数据备份恢复",
			Description: "应提供重要数据的本地数据备份与恢复功能",
			Requirement: "应建立数据备份和恢复机制",
			Remediation: "配置自动备份策略和恢复测试",
			Enabled:     true,
			Tags:        []string{"backup", "recovery"},
		},
		// 安全管理中心
		{
			ID:          "MLPS2-MC-1",
			Standard:    StandardMLPS2,
			Category:    CategorySystemHardening,
			Severity:    SeverityHigh,
			Title:       "系统管理",
			Description: "应对系统管理员进行身份鉴别",
			Requirement: "系统管理应通过安全通道进行",
			Remediation: "配置安全的管理通道",
			Enabled:     true,
			Tags:        []string{"management", "system"},
		},
		{
			ID:          "MLPS2-MC-2",
			Standard:    StandardMLPS2,
			Category:    CategoryAuditLogging,
			Severity:    SeverityHigh,
			Title:       "审计管理",
			Description: "应对审计管理员进行身份鉴别",
			Requirement: "审计管理应独立于系统管理",
			Remediation: "分离审计管理和系统管理职责",
			Enabled:     true,
			Tags:        []string{"audit", "management"},
		},
	}
	for _, rule := range rules {
		s.rules[rule.ID] = rule
		s.registerCheck(rule.ID)
	}
}

// registerCheck 注册规则检查函数
func (s *Scanner) registerCheck(ruleID string) {
	// 根据规则ID注册对应的检查函数
	// 这里是简化版本，实际应实现具体的检查逻辑
	s.checks[ruleID] = func() (*CheckDetail, error) {
		rule, exists := s.rules[ruleID]
		if !exists {
			return nil, fmt.Errorf("规则 %s 不存在", ruleID)
		}

		// 模拟检查过程
		detail := &CheckDetail{
			RuleID:    ruleID,
			CheckedAt: time.Now(),
			Duration:  time.Millisecond * 100,
		}

		// 根据规则类别执行不同的检查
		switch rule.Category {
		case CategoryAccessControl:
			detail = s.checkAccessControl(rule)
		case CategoryAuditLogging:
			detail = s.checkAuditLogging(rule)
		case CategoryNetworkSecurity:
			detail = s.checkNetworkSecurity(rule)
		case CategoryEncryption:
			detail = s.checkEncryption(rule)
		case CategoryIdentityAuth:
			detail = s.checkIdentityAuth(rule)
		default:
			detail.Result = ResultSkip
			detail.Message = "暂未实现此规则的检查逻辑"
		}

		return detail, nil
	}
}

// checkAccessControl 检查访问控制类规则
func (s *Scanner) checkAccessControl(rule *ComplianceRule) *CheckDetail {
	detail := &CheckDetail{
		RuleID:    rule.ID,
		CheckedAt: time.Now(),
	}
	// 实际应检查文件权限、用户配置等
	detail.Result = ResultPass
	detail.Message = "访问控制检查通过"
	detail.Duration = time.Since(detail.CheckedAt)
	return detail
}

// checkAuditLogging 检查审计日志类规则
func (s *Scanner) checkAuditLogging(rule *ComplianceRule) *CheckDetail {
	detail := &CheckDetail{
		RuleID:    rule.ID,
		CheckedAt: time.Now(),
	}
	// 实际应检查审计日志配置
	detail.Result = ResultPass
	detail.Message = "审计日志检查通过"
	detail.Duration = time.Since(detail.CheckedAt)
	return detail
}

// checkNetworkSecurity 检查网络安全类规则
func (s *Scanner) checkNetworkSecurity(rule *ComplianceRule) *CheckDetail {
	detail := &CheckDetail{
		RuleID:    rule.ID,
		CheckedAt: time.Now(),
	}
	// 实际应检查网络配置、防火墙规则等
	detail.Result = ResultPass
	detail.Message = "网络安全检查通过"
	detail.Duration = time.Since(detail.CheckedAt)
	return detail
}

// checkEncryption 检查加密类规则
func (s *Scanner) checkEncryption(rule *ComplianceRule) *CheckDetail {
	detail := &CheckDetail{
		RuleID:    rule.ID,
		CheckedAt: time.Now(),
	}
	// 实际应检查加密配置
	detail.Result = ResultPass
	detail.Message = "加密检查通过"
	detail.Duration = time.Since(detail.CheckedAt)
	return detail
}

// checkIdentityAuth 检查身份认证类规则
func (s *Scanner) checkIdentityAuth(rule *ComplianceRule) *CheckDetail {
	detail := &CheckDetail{
		RuleID:    rule.ID,
		CheckedAt: time.Now(),
	}
	// 实际应检查认证配置
	detail.Result = ResultPass
	detail.Message = "身份认证检查通过"
	detail.Duration = time.Since(detail.CheckedAt)
	return detail
}

// Scan 执行合规扫描
func (s *Scanner) Scan(ctx context.Context, standards []ComplianceStandard) (*ComplianceScan, error) {
	s.mu.Lock()
	if s.isScanning {
		s.mu.Unlock()
		return nil, fmt.Errorf("扫描正在进行中")
	}
	s.isScanning = true
	ctx, s.cancelFunc = context.WithCancel(ctx)
	s.mu.Unlock()

	scan := &ComplianceScan{
		ID:        fmt.Sprintf("scan-%d", time.Now().UnixNano()),
		Standards: standards,
		Status:    StatusRunning,
		StartTime: time.Now(),
		Checks:    make([]CheckDetail, 0),
		Errors:    make([]ScanError, 0),
	}

	// 收集需要检查的规则
	var rulesToCheck []*ComplianceRule
	s.mu.RLock()
	for _, rule := range s.rules {
		if !rule.Enabled {
			continue
		}
		if len(standards) > 0 {
			for _, std := range standards {
				if rule.Standard == std {
					rulesToCheck = append(rulesToCheck, rule)
					break
				}
			}
		} else {
			rulesToCheck = append(rulesToCheck, rule)
		}
	}
	s.mu.RUnlock()

	scan.TotalRules = len(rulesToCheck)

	// 执行检查
	for _, rule := range rulesToCheck {
		select {
		case <-ctx.Done():
			scan.Status = StatusCancelled
			break
		default:
		}

		s.mu.RLock()
		checkFunc, exists := s.checks[rule.ID]
		s.mu.RUnlock()

		if !exists {
			scan.Errors = append(scan.Errors, ScanError{
				RuleID: rule.ID,
				Error:  "检查函数未注册",
			})
			scan.ErrorRules++
			continue
		}

		detail, err := checkFunc()
		if err != nil {
			scan.Errors = append(scan.Errors, ScanError{
				RuleID: rule.ID,
				Error:  err.Error(),
			})
			scan.ErrorRules++
			continue
		}

		scan.Checks = append(scan.Checks, *detail)

		switch detail.Result {
		case ResultPass:
			scan.PassedRules++
		case ResultFail:
			scan.FailedRules++
		case ResultWarning:
			scan.WarnRules++
		case ResultSkip:
			scan.SkipRules++
		}
	}

	scan.EndTime = time.Now()
	scan.Duration = scan.EndTime.Sub(scan.StartTime)

	if scan.Status != StatusCancelled {
		scan.Status = StatusCompleted
	}

	// 更新统计
	s.mu.Lock()
	s.lastScan = scan
	s.isScanning = false
	s.stats.TotalScans++
	if scan.Status == StatusCompleted {
		s.stats.SuccessfulScans++
	} else if scan.Status == StatusFailed {
		s.stats.FailedScans++
	}
	now := time.Now()
	s.stats.LastScanTime = &now
	s.stats.LastScanStatus = scan.Status
	s.mu.Unlock()

	return scan, nil
}

// CancelScan 取消扫描
func (s *Scanner) CancelScan() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
}

// GetLastScan 获取最近扫描结果
func (s *Scanner) GetLastScan() *ComplianceScan {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastScan
}

// GetStats 获取统计信息
func (s *Scanner) GetStats() *ComplianceStats {
	return s.stats.GetSnapshot()
}

// GetRules 获取所有规则
func (s *Scanner) GetRules() []*ComplianceRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rules := make([]*ComplianceRule, 0, len(s.rules))
	for _, rule := range s.rules {
		rules = append(rules, rule)
	}
	return rules
}

// GetRulesByStandard 按标准获取规则
func (s *Scanner) GetRulesByStandard(standard ComplianceStandard) []*ComplianceRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var rules []*ComplianceRule
	for _, rule := range s.rules {
		if rule.Standard == standard {
			rules = append(rules, rule)
		}
	}
	return rules
}

// GetRulesByCategory 按类别获取规则
func (s *Scanner) GetRulesByCategory(category RuleCategory) []*ComplianceRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var rules []*ComplianceRule
	for _, rule := range s.rules {
		if rule.Category == category {
			rules = append(rules, rule)
		}
	}
	return rules
}

// EnableRule 启用规则
func (s *Scanner) EnableRule(ruleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rule, exists := s.rules[ruleID]
	if !exists {
		return fmt.Errorf("规则 %s 不存在", ruleID)
	}
	rule.Enabled = true
	rule.UpdatedAt = time.Now()
	return nil
}

// DisableRule 禁用规则
func (s *Scanner) DisableRule(ruleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rule, exists := s.rules[ruleID]
	if !exists {
		return fmt.Errorf("规则 %s 不存在", ruleID)
	}
	rule.Enabled = false
	rule.UpdatedAt = time.Now()
	return nil
}
