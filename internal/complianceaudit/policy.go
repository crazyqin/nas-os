// Package complianceaudit 提供合规策略管理
package complianceaudit

import (
	"fmt"
	"time"

	"go.uber.org/zap"
)

// PolicyEngine 合规策略引擎
type PolicyEngine struct {
	policies map[ComplianceStandard]*CompliancePolicy
	logger   *zap.Logger
}

// CompliancePolicy 合规策略
type CompliancePolicy struct {
	Standard    ComplianceStandard `json:"standard"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Version     string             `json:"version"`
	Controls    []PolicyControl    `json:"controls"`
	Enabled     bool               `json:"enabled"`
}

// PolicyControl 策略控制项
type PolicyControl struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Category    CheckCategory `json:"category"`
	Required    bool          `json:"required"`
	Weight      float64       `json:"weight"` // 权重 0-1
}

// PolicyResult 策略评估结果
type PolicyResult struct {
	Standard   ComplianceStandard `json:"standard"`
	Score      float64            `json:"score"`
	PassRate   float64            `json:"pass_rate"`
	Controls   []ControlResult    `json:"controls"`
	Compliant  bool               `json:"compliant"`
	AssessedAt time.Time          `json:"assessed_at"`
}

// ControlResult 控制项评估结果
type ControlResult struct {
	ControlID string      `json:"control_id"`
	Name      string      `json:"name"`
	Status    CheckStatus `json:"status"`
	Score     float64     `json:"score"`
	Findings  int         `json:"findings"`
}

// NewPolicyEngine 创建策略引擎
func NewPolicyEngine(logger *zap.Logger) *PolicyEngine {
	if logger == nil {
		logger = zap.NewNop()
	}

	pe := &PolicyEngine{
		policies: make(map[ComplianceStandard]*CompliancePolicy),
		logger:   logger,
	}

	// 注册内置策略
	pe.registerBuiltinPolicies()

	return pe
}

// registerBuiltinPolicies 注册内置合规策略
func (pe *PolicyEngine) registerBuiltinPolicies() {
	// GDPR 策略
	pe.policies[StandardGDPR] = &CompliancePolicy{
		Standard:    StandardGDPR,
		Name:        "欧盟通用数据保护条例 (GDPR)",
		Description: "GDPR 是欧盟制定的数据保护和隐私法规",
		Version:     "2018",
		Enabled:     true,
		Controls: []PolicyControl{
			{ID: "GDPR-01", Name: "数据处理合法性", Description: "确保数据处理有合法基础", Category: CategoryDataProtection, Required: true, Weight: 0.15},
			{ID: "GDPR-02", Name: "数据主体权利", Description: "支持数据主体的访问、更正、删除权", Category: CategoryDataProtection, Required: true, Weight: 0.15},
			{ID: "GDPR-03", Name: "数据加密", Description: "个人数据传输和存储必须加密", Category: CategoryEncryption, Required: true, Weight: 0.20},
			{ID: "GDPR-04", Name: "访问控制", Description: "实施适当的访问控制措施", Category: CategoryAccessControl, Required: true, Weight: 0.15},
			{ID: "GDPR-05", Name: "数据泄露通知", Description: "72小时内报告数据泄露事件", Category: CategoryIncidentResponse, Required: true, Weight: 0.10},
			{ID: "GDPR-06", Name: "密码策略", Description: "实施强密码策略", Category: CategoryPasswordPolicy, Required: true, Weight: 0.10},
			{ID: "GDPR-07", Name: "审计日志", Description: "记录数据处理活动", Category: CategoryAuditLog, Required: false, Weight: 0.10},
			{ID: "GDPR-08", Name: "网络安全", Description: "保护网络通信安全", Category: CategoryNetworkSecurity, Required: false, Weight: 0.05},
		},
	}

	// 等保2.0 策略
	pe.policies[StandardMLPS2] = &CompliancePolicy{
		Standard:    StandardMLPS2,
		Name:        "网络安全等级保护2.0",
		Description: "中国网络安全等级保护制度 (GB/T 22239-2019)",
		Version:     "2.0",
		Enabled:     true,
		Controls: []PolicyControl{
			{ID: "MLPS2-01", Name: "安全物理环境", Description: "机房物理安全", Category: CategoryAccessControl, Required: true, Weight: 0.10},
			{ID: "MLPS2-02", Name: "安全通信网络", Description: "网络架构安全", Category: CategoryNetworkSecurity, Required: true, Weight: 0.15},
			{ID: "MLPS2-03", Name: "安全区域边界", Description: "边界防护和访问控制", Category: CategoryNetworkSecurity, Required: true, Weight: 0.15},
			{ID: "MLPS2-04", Name: "安全计算环境", Description: "主机和应用安全", Category: CategoryAccessControl, Required: true, Weight: 0.15},
			{ID: "MLPS2-05", Name: "安全管理中心", Description: "集中安全管理和审计", Category: CategoryAuditLog, Required: true, Weight: 0.15},
			{ID: "MLPS2-06", Name: "安全管理制度", Description: "安全策略和管理制度", Category: CategoryDataProtection, Required: true, Weight: 0.10},
			{ID: "MLPS2-07", Name: "安全管理人员", Description: "人员安全管理", Category: CategoryPasswordPolicy, Required: true, Weight: 0.10},
			{ID: "MLPS2-08", Name: "安全建设管理", Description: "系统建设安全管理", Category: CategoryEncryption, Required: false, Weight: 0.10},
		},
	}

	// ISO 27001 策略
	pe.policies[StandardISO27001] = &CompliancePolicy{
		Standard:    StandardISO27001,
		Name:        "ISO/IEC 27001 信息安全管理体系",
		Description: "国际信息安全管理体系标准",
		Version:     "2022",
		Enabled:     true,
		Controls: []PolicyControl{
			{ID: "ISO-01", Name: "信息安全策略", Description: "建立信息安全策略", Category: CategoryDataProtection, Required: true, Weight: 0.10},
			{ID: "ISO-02", Name: "访问控制", Description: "实施访问控制策略", Category: CategoryAccessControl, Required: true, Weight: 0.15},
			{ID: "ISO-03", Name: "密码学", Description: "正确使用密码学控制", Category: CategoryEncryption, Required: true, Weight: 0.15},
			{ID: "ISO-04", Name: "物理安全", Description: "物理和环境安全", Category: CategoryAccessControl, Required: true, Weight: 0.10},
			{ID: "ISO-05", Name: "运营安全", Description: "运营安全管理", Category: CategoryAuditLog, Required: true, Weight: 0.15},
			{ID: "ISO-06", Name: "通信安全", Description: "网络通信安全管理", Category: CategoryNetworkSecurity, Required: true, Weight: 0.15},
			{ID: "ISO-07", Name: "系统获取开发", Description: "系统开发和维护安全", Category: CategoryDataProtection, Required: false, Weight: 0.10},
			{ID: "ISO-08", Name: "供应商关系", Description: "供应商信息安全", Category: CategoryPasswordPolicy, Required: false, Weight: 0.10},
		},
	}

	// SOC 2 策略
	pe.policies[StandardSOC2] = &CompliancePolicy{
		Standard:    StandardSOC2,
		Name:        "SOC 2 服务组织控制",
		Description: "AICPA SOC 2 信任服务标准",
		Version:     "2017",
		Enabled:     true,
		Controls: []PolicyControl{
			{ID: "SOC2-01", Name: "安全性", Description: "系统受保护免受未授权访问", Category: CategoryAccessControl, Required: true, Weight: 0.25},
			{ID: "SOC2-02", Name: "可用性", Description: "系统可用性承诺", Category: CategoryNetworkSecurity, Required: true, Weight: 0.20},
			{ID: "SOC2-03", Name: "处理完整性", Description: "系统处理完整准确", Category: CategoryAuditLog, Required: true, Weight: 0.20},
			{ID: "SOC2-04", Name: "保密性", Description: "信息保密性保护", Category: CategoryEncryption, Required: true, Weight: 0.20},
			{ID: "SOC2-05", Name: "隐私", Description: "个人信息收集使用保护", Category: CategoryDataProtection, Required: true, Weight: 0.15},
		},
	}
}

// GetPolicy 获取合规策略
func (pe *PolicyEngine) GetPolicy(standard ComplianceStandard) (*CompliancePolicy, bool) {
	p, ok := pe.policies[standard]
	return p, ok
}

// ListPolicies 列出所有策略
func (pe *PolicyEngine) ListPolicies() []*CompliancePolicy {
	policies := make([]*CompliancePolicy, 0, len(pe.policies))
	for _, p := range pe.policies {
		policies = append(policies, p)
	}
	return policies
}

// EvaluateAll 评估所有策略
func (pe *PolicyEngine) EvaluateAll(report *ComplianceReport) []*PolicyResult {
	results := make([]*PolicyResult, 0)

	for _, policy := range pe.policies {
		if !policy.Enabled {
			continue
		}
		result := pe.Evaluate(report, policy.Standard)
		results = append(results, result)
	}

	return results
}

// Evaluate 评估指定标准的合规性
func (pe *PolicyEngine) Evaluate(report *ComplianceReport, standard ComplianceStandard) *PolicyResult {
	policy, ok := pe.policies[standard]
	if !ok {
		return &PolicyResult{
			Standard:   standard,
			Score:      0,
			Compliant:  false,
			AssessedAt: time.Now(),
		}
	}

	result := &PolicyResult{
		Standard:   standard,
		Controls:   make([]ControlResult, 0),
		AssessedAt: time.Now(),
	}

	// 按类别统计检查结果
	categoryResults := make(map[CheckCategory][]*CheckResult)
	for _, sr := range report.Standards {
		if sr.Standard == standard {
			for _, check := range sr.Checks {
				categoryResults[check.Category] = append(categoryResults[check.Category], check)
			}
		}
	}

	// 也检查全局的 findings
	findingCategories := make(map[CheckCategory]int)
	for _, f := range report.Findings {
		if f.Standard == standard {
			findingCategories[f.Category]++
		}
	}

	// 评估每个控制项
	totalScore := 0.0
	totalWeight := 0.0
	passedControls := 0

	for _, control := range policy.Controls {
		cr := ControlResult{
			ControlID: control.ID,
			Name:      control.Name,
			Status:    StatusPass,
		}

		checks := categoryResults[control.Category]
		if len(checks) > 0 {
			passCount := 0
			for _, c := range checks {
				if c.Status == StatusPass {
					passCount++
				}
			}
			cr.Score = float64(passCount) / float64(len(checks)) * 100
			cr.Findings = len(checks) - passCount

			if passCount < len(checks) {
				if float64(passCount)/float64(len(checks)) < 0.5 {
					cr.Status = StatusFail
				} else {
					cr.Status = StatusWarn
				}
			}
		} else {
			// 没有相关检查项，给 50 分
			cr.Score = 50
			cr.Status = StatusWarn
		}

		// 检查 findings
		if n, ok := findingCategories[control.Category]; ok && n > 0 {
			cr.Findings += n
			if cr.Status == StatusPass {
				cr.Status = StatusWarn
			}
		}

		totalScore += cr.Score * control.Weight
		totalWeight += control.Weight

		if cr.Status == StatusPass {
			passedControls++
		}

		result.Controls = append(result.Controls, cr)
	}

	// 计算总分
	if totalWeight > 0 {
		result.Score = totalScore / totalWeight
	}
	result.PassRate = float64(passedControls) / float64(len(policy.Controls)) * 100

	// 判断是否合规：加权分数 >= 70 且没有必需控制项失败
	result.Compliant = result.Score >= 70
	if result.Compliant {
		for _, control := range policy.Controls {
			if control.Required {
				for _, cr := range result.Controls {
					if cr.ControlID == control.ID && cr.Status == StatusFail {
						result.Compliant = false
						break
					}
				}
			}
		}
	}

	return result
}

// AddPolicy 添加自定义策略
func (pe *PolicyEngine) AddPolicy(policy *CompliancePolicy) error {
	if policy == nil {
		return fmt.Errorf("policy cannot be nil")
	}
	if policy.Standard == "" {
		return fmt.Errorf("policy standard cannot be empty")
	}
	pe.policies[policy.Standard] = policy
	pe.logger.Info("added compliance policy",
		zap.String("standard", string(policy.Standard)),
		zap.String("name", policy.Name),
	)
	return nil
}

// RemovePolicy 移除策略
func (pe *PolicyEngine) RemovePolicy(standard ComplianceStandard) {
	delete(pe.policies, standard)
	pe.logger.Info("removed compliance policy", zap.String("standard", string(standard)))
}

// EnablePolicy 启用策略
func (pe *PolicyEngine) EnablePolicy(standard ComplianceStandard) error {
	p, ok := pe.policies[standard]
	if !ok {
		return fmt.Errorf("policy %q not found", standard)
	}
	p.Enabled = true
	return nil
}

// DisablePolicy 禁用策略
func (pe *PolicyEngine) DisablePolicy(standard ComplianceStandard) error {
	p, ok := pe.policies[standard]
	if !ok {
		return fmt.Errorf("policy %q not found", standard)
	}
	p.Enabled = false
	return nil
}

// GetComplianceStatus 获取合规状态摘要
func (pe *PolicyEngine) GetComplianceStatus(report *ComplianceReport) map[ComplianceStandard]bool {
	status := make(map[ComplianceStandard]bool)
	results := pe.EvaluateAll(report)
	for _, r := range results {
		status[r.Standard] = r.Compliant
	}
	return status
}
