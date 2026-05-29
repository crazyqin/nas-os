// Package auditreport 提供合规报告模板
package auditreport

import (
	"fmt"
	"time"
)

// ComplianceStandard 合规标准.
type ComplianceStandard string

const (
	StandardGDPR   ComplianceStandard = "GDPR"
	StandardFIPS140 ComplianceStandard = "FIPS140"
	StandardDJB20  ComplianceStandard = "等保2.0"
	StandardSOC2   ComplianceStandard = "SOC2"
	StandardHIPAA  ComplianceStandard = "HIPAA"
	StandardISO27001 ComplianceStandard = "ISO27001"
)

// ReportTemplate 报告模板.
type ReportTemplate struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Standard    ComplianceStandard `json:"standard"`
	Description string             `json:"description"`
	Sections    []ReportSection    `json:"sections"`
	CreatedAt   time.Time          `json:"created_at"`
}

// ReportSection 报告章节.
type ReportSection struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Items       []SectionItem `json:"items"`
}

// SectionItem 章节项目.
type SectionItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	CheckFunc   string `json:"check_func"` // 检查函数标识
	Required    bool   `json:"required"`
}

// ComplianceReport 合规报告.
type ComplianceReport struct {
	ID           string             `json:"id"`
	Standard     ComplianceStandard `json:"standard"`
	GeneratedAt  time.Time          `json:"generated_at"`
	Score        float64            `json:"score"`
	Passed       int                `json:"passed"`
	Failed       int                `json:"failed"`
	Total        int                `json:"total"`
	Sections     []ComplianceSection `json:"sections"`
	Summary      string             `json:"summary"`
	Template     string             `json:"template"`
}

// ComplianceSection 合规章节结果.
type ComplianceSection struct {
	ID       string               `json:"id"`
	Title    string               `json:"title"`
	Score    float64              `json:"score"`
	Items    []ComplianceItemResult `json:"items"`
}

// ComplianceItemResult 合规项结果.
type ComplianceItemResult struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Status      string `json:"status"` // passed, failed, not_applicable
	Description string `json:"description"`
	Detail      string `json:"detail"`
}

// TemplateManager 模板管理器.
type TemplateManager struct {
	templates map[string]*ReportTemplate
}

// NewTemplateManager 创建模板管理器.
func NewTemplateManager() *TemplateManager {
	tm := &TemplateManager{
		templates: make(map[string]*ReportTemplate),
	}
	tm.loadDefaultTemplates()
	return tm
}

// GetTemplate 获取模板.
func (tm *TemplateManager) GetTemplate(standard ComplianceStandard) (*ReportTemplate, error) {
	key := string(standard)
	if tmpl, ok := tm.templates[key]; ok {
		return tmpl, nil
	}
	return nil, fmt.Errorf("template for standard %q not found", standard)
}

// ListTemplates 列出所有模板.
func (tm *TemplateManager) ListTemplates() []*ReportTemplate {
	var templates []*ReportTemplate
	for _, t := range tm.templates {
		templates = append(templates, t)
	}
	return templates
}

// RegisterTemplate 注册模板.
func (tm *TemplateManager) RegisterTemplate(tmpl *ReportTemplate) {
	tm.templates[string(tmpl.Standard)] = tmpl
}

// loadDefaultTemplates 加载默认模板.
func (tm *TemplateManager) loadDefaultTemplates() {
	// GDPR 模板
	tm.templates["GDPR"] = &ReportTemplate{
		ID:          "gdpr-v1",
		Name:        "GDPR 合规报告",
		Standard:    StandardGDPR,
		Description: "欧盟通用数据保护条例合规检查",
		Sections: []ReportSection{
			{
				ID:          "gdpr-dp",
				Title:       "数据处理",
				Description: "数据处理合规性检查",
				Items: []SectionItem{
					{ID: "GDPR-DP-001", Title: "数据处理同意机制", Description: "检查是否建立了有效的同意收集和管理机制", CheckFunc: "check_consent_mechanism", Required: true},
					{ID: "GDPR-DP-002", Title: "数据最小化原则", Description: "检查是否只收集必要的个人数据", CheckFunc: "check_data_minimization", Required: true},
					{ID: "GDPR-DP-003", Title: "目的限制", Description: "检查数据处理是否符合声明的目的", CheckFunc: "check_purpose_limitation", Required: true},
				},
			},
			{
				ID:          "gdpr-rights",
				Title:       "数据主体权利",
				Description: "数据主体权利保障检查",
				Items: []SectionItem{
					{ID: "GDPR-RT-001", Title: "访问权", Description: "检查是否支持数据主体访问其个人数据", CheckFunc: "check_access_right", Required: true},
					{ID: "GDPR-RT-002", Title: "删除权", Description: "检查是否支持数据主体请求删除其数据", CheckFunc: "check_erasure_right", Required: true},
					{ID: "GDPR-RT-003", Title: "数据可携权", Description: "检查是否支持数据导出和转移", CheckFunc: "check_data_portability", Required: true},
					{ID: "GDPR-RT-004", Title: "反对权", Description: "检查是否支持数据主体反对数据处理", CheckFunc: "check_objection_right", Required: false},
				},
			},
			{
				ID:          "gdpr-security",
				Title:       "安全措施",
				Description: "数据安全措施检查",
				Items: []SectionItem{
					{ID: "GDPR-SE-001", Title: "数据加密", Description: "检查数据传输和存储是否加密", CheckFunc: "check_encryption", Required: true},
					{ID: "GDPR-SE-002", Title: "访问控制", Description: "检查是否实施了适当的访问控制", CheckFunc: "check_access_control", Required: true},
					{ID: "GDPR-SE-003", Title: "数据泄露通知", Description: "检查是否建立了数据泄露通知流程", CheckFunc: "check_breach_notification", Required: true},
				},
			},
			{
				ID:          "gdpr-dpo",
				Title:       "数据保护官",
				Description: "DPO 相关检查",
				Items: []SectionItem{
					{ID: "GDPR-DPO-001", Title: "DPO 指定", Description: "检查是否指定了数据保护官", CheckFunc: "check_dpo_designated", Required: true},
					{ID: "GDPR-DPO-002", Title: "DPIA 执行", Description: "检查是否完成了数据保护影响评估", CheckFunc: "check_dpia", Required: true},
				},
			},
		},
		CreatedAt: time.Now(),
	}

	// FIPS 140 模板
	tm.templates["FIPS140"] = &ReportTemplate{
		ID:          "fips140-v1",
		Name:        "FIPS 140-2/140-3 合规报告",
		Standard:    StandardFIPS140,
		Description: "联邦信息处理标准密码模块合规检查",
		Sections: []ReportSection{
			{
				ID:          "fips-crypto",
				Title:       "密码模块",
				Description: "密码模块实现检查",
				Items: []SectionItem{
					{ID: "FIPS-CM-001", Title: "密码算法", Description: "检查是否使用 FIPS 批准的密码算法", CheckFunc: "check_approved_algorithms", Required: true},
					{ID: "FIPS-CM-002", Title: "密钥管理", Description: "检查密钥生成、存储和销毁", CheckFunc: "check_key_management", Required: true},
					{ID: "FIPS-CM-003", Title: "随机数生成", Description: "检查是否使用经批准的 RNG", CheckFunc: "check_rng", Required: true},
				},
			},
			{
				ID:          "fips-auth",
				Title:       "身份认证",
				Description: "身份认证机制检查",
				Items: []SectionItem{
					{ID: "FIPS-AU-001", Title: "认证机制", Description: "检查身份认证机制强度", CheckFunc: "check_auth_mechanism", Required: true},
					{ID: "FIPS-AU-002", Title: "多因素认证", Description: "检查是否支持多因素认证", CheckFunc: "check_mfa", Required: true},
					{ID: "FIPS-AU-003", Title: "会话管理", Description: "检查会话安全配置", CheckFunc: "check_session_mgmt", Required: true},
				},
			},
			{
				ID:          "fips-integrity",
				Title:       "完整性保护",
				Description: "数据完整性保护检查",
				Items: []SectionItem{
					{ID: "FIPS-IN-001", Title: "完整性校验", Description: "检查是否实施完整性校验机制", CheckFunc: "check_integrity_check", Required: true},
					{ID: "FIPS-IN-002", Title: "自检功能", Description: "检查密码模块自检功能", CheckFunc: "check_self_test", Required: true},
					{ID: "FIPS-IN-003", Title: "审计日志", Description: "检查审计日志完整性保护", CheckFunc: "check_audit_log_integrity", Required: true},
				},
			},
			{
				ID:          "fips-env",
				Title:       "物理安全",
				Description: "物理环境保护检查",
				Items: []SectionItem{
					{ID: "FIPS-PS-001", Title: "物理防护", Description: "检查物理安全边界保护", CheckFunc: "check_physical_security", Required: true},
					{ID: "FIPS-PS-002", Title: "环境控制", Description: "检查环境控制措施", CheckFunc: "check_env_control", Required: false},
				},
			},
		},
		CreatedAt: time.Now(),
	}

	// 等保 2.0 模板
	tm.templates["等保2.0"] = &ReportTemplate{
		ID:          "djb20-v1",
		Name:        "网络安全等级保护 2.0 合规报告",
		Standard:    StandardDJB20,
		Description: "中国网络安全等级保护 2.0 标准合规检查",
		Sections: []ReportSection{
			{
				ID:          "djb-network",
				Title:       "网络安全",
				Description: "网络安全检查",
				Items: []SectionItem{
					{ID: "DJB-NW-001", Title: "网络架构", Description: "检查网络架构安全性", CheckFunc: "check_network_architecture", Required: true},
					{ID: "DJB-NW-002", Title: "通信传输", Description: "检查通信传输加密", CheckFunc: "check_communication_encryption", Required: true},
					{ID: "DJB-NW-003", Title: "边界防护", Description: "检查网络边界防护措施", CheckFunc: "check_border_protection", Required: true},
					{ID: "DJB-NW-004", Title: "访问控制", Description: "检查网络访问控制策略", CheckFunc: "check_network_access_control", Required: true},
				},
			},
			{
				ID:          "djb-host",
				Title:       "主机安全",
				Description: "主机安全检查",
				Items: []SectionItem{
					{ID: "DJB-HS-001", Title: "身份鉴别", Description: "检查主机身份鉴别机制", CheckFunc: "check_host_authentication", Required: true},
					{ID: "DJB-HS-002", Title: "访问控制", Description: "检查主机访问控制策略", CheckFunc: "check_host_access_control", Required: true},
					{ID: "DJB-HS-003", Title: "安全审计", Description: "检查主机审计功能", CheckFunc: "check_host_audit", Required: true},
					{ID: "DJB-HS-004", Title: "入侵防范", Description: "检查入侵检测和防范措施", CheckFunc: "check_intrusion_prevention", Required: true},
					{ID: "DJB-HS-005", Title: "恶意代码防范", Description: "检查恶意代码防范措施", CheckFunc: "check_malware_prevention", Required: true},
				},
			},
			{
				ID:          "djb-app",
				Title:       "应用安全",
				Description: "应用安全检查",
				Items: []SectionItem{
					{ID: "DJB-AP-001", Title: "身份鉴别", Description: "检查应用身份鉴别机制", CheckFunc: "check_app_authentication", Required: true},
					{ID: "DJB-AP-002", Title: "访问控制", Description: "检查应用访问控制", CheckFunc: "check_app_access_control", Required: true},
					{ID: "DJB-AP-003", Title: "安全审计", Description: "检查应用审计功能", CheckFunc: "check_app_audit", Required: true},
					{ID: "DJB-AP-004", Title: "通信完整性", Description: "检查通信完整性保护", CheckFunc: "check_communication_integrity", Required: true},
					{ID: "DJB-AP-005", Title: "软件容错", Description: "检查软件容错能力", CheckFunc: "check_software_fault_tolerance", Required: true},
				},
			},
			{
				ID:          "djb-data",
				Title:       "数据安全",
				Description: "数据安全检查",
				Items: []SectionItem{
					{ID: "DJB-DA-001", Title: "数据完整性", Description: "检查数据完整性保护", CheckFunc: "check_data_integrity", Required: true},
					{ID: "DJB-DA-002", Title: "数据保密性", Description: "检查数据保密性保护", CheckFunc: "check_data_confidentiality", Required: true},
					{ID: "DJB-DA-003", Title: "数据备份恢复", Description: "检查数据备份和恢复机制", CheckFunc: "check_data_backup", Required: true},
					{ID: "DJB-DA-004", Title: "剩余信息保护", Description: "检查剩余信息保护措施", CheckFunc: "check_residual_info", Required: true},
				},
			},
			{
				ID:          "djb-mgmt",
				Title:       "安全管理",
				Description: "安全管理检查",
				Items: []SectionItem{
					{ID: "DJB-MG-001", Title: "安全管理制度", Description: "检查安全管理制度", CheckFunc: "check_security_policy", Required: true},
					{ID: "DJB-MG-002", Title: "安全管理机构", Description: "检查安全管理机构设置", CheckFunc: "check_security_org", Required: true},
					{ID: "DJB-MG-003", Title: "安全管理人员", Description: "检查安全管理人员配置", CheckFunc: "check_security_personnel", Required: true},
					{ID: "DJB-MG-004", Title: "安全建设管理", Description: "检查安全建设管理流程", CheckFunc: "check_security_construction", Required: true},
					{ID: "DJB-MG-005", Title: "安全运维管理", Description: "检查安全运维管理措施", CheckFunc: "check_security_operations", Required: true},
				},
			},
		},
		CreatedAt: time.Now(),
	}

	// SOC2 模板
	tm.templates["SOC2"] = &ReportTemplate{
		ID:          "soc2-v1",
		Name:        "SOC 2 合规报告",
		Standard:    StandardSOC2,
		Description: "SOC 2 服务组织控制报告",
		Sections: []ReportSection{
			{
				ID:          "soc2-cc",
				Title:       "通用标准",
				Description: "通用标准准则",
				Items: []SectionItem{
					{ID: "SOC2-CC-001", Title: "控制环境", Description: "检查控制环境", CheckFunc: "check_control_environment", Required: true},
					{ID: "SOC2-CC-002", Title: "沟通与信息", Description: "检查沟通和信息机制", CheckFunc: "check_communication", Required: true},
					{ID: "SOC2-CC-003", Title: "风险评估", Description: "检查风险评估流程", CheckFunc: "check_risk_assessment", Required: true},
					{ID: "SOC2-CC-004", Title: "监控活动", Description: "检查监控活动", CheckFunc: "check_monitoring", Required: true},
					{ID: "SOC2-CC-005", Title: "控制活动", Description: "检查控制活动", CheckFunc: "check_control_activities", Required: true},
				},
			},
			{
				ID:          "soc2-a1",
				Title:       "可用性",
				Description: "可用性准则",
				Items: []SectionItem{
					{ID: "SOC2-A1-001", Title: "性能监控", Description: "检查系统性能监控", CheckFunc: "check_performance_monitoring", Required: true},
					{ID: "SOC2-A1-002", Title: "灾难恢复", Description: "检查灾难恢复计划", CheckFunc: "check_disaster_recovery", Required: true},
				},
			},
		},
		CreatedAt: time.Now(),
	}
}
