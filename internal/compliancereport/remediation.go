// Package compliancereport 提供整改建议生成功能
package compliancereport

import "fmt"

// RemediationGenerator 整改建议生成器.
type RemediationGenerator struct{}

// NewRemediationGenerator 创建整改建议生成器.
func NewRemediationGenerator() *RemediationGenerator {
	return &RemediationGenerator{}
}

// Generate 为失败和警告的扫描结果生成整改建议.
func (g *RemediationGenerator) Generate(results []ScanResult) []Remediation {
	var remediations []Remediation

	for _, r := range results {
		if r.Status == CheckItemFail || r.Status == CheckItemWarning {
			remediation := g.generateForResult(r)
			remediations = append(remediations, remediation)
		}
	}

	return remediations
}

// generateForResult 为单个扫描结果生成整改建议.
func (g *RemediationGenerator) generateForResult(r ScanResult) Remediation {
	// 根据 checkID 匹配具体建议
	switch r.CheckID {
	case "ac_mfa":
		return Remediation{
			CheckID:     r.CheckID,
			Title:       "启用全面多因素认证",
			Description: "部分用户账户未启用多因素认证，存在账户被暴力破解的风险",
			Steps: []string{
				"1. 管理后台 → 安全设置 → 多因素认证",
				"2. 强制所有管理员账户启用 MFA",
				"3. 配置默认 MFA 策略为 TOTP",
				"4. 设置 MFA 启用宽限期为 7 天",
				"5. 通过邮件通知未启用 MFA 的用户",
			},
			Priority: SeverityCritical,
		}
	case "de_key_mgmt":
		return Remediation{
			CheckID:     r.CheckID,
			Title:       "更新密钥轮换策略",
			Description: "加密密钥轮换周期超过推荐的安全期限",
			Steps: []string{
				"1. 安全设置 → 密钥管理 → 轮换策略",
				"2. 将主密钥轮换周期设置为 90 天",
				"3. 将数据加密密钥轮换周期设置为 30 天",
				"4. 立即执行一次手动密钥轮换",
				"5. 验证轮换后数据可正常访问",
			},
			Priority: SeverityHigh,
		}
	case "bk_policy":
		return Remediation{
			CheckID:     r.CheckID,
			Title:       "配置或更新备份策略",
			Description: "备份策略未配置或已过期，数据存在丢失风险",
			Steps: []string{
				"1. 备份管理 → 策略配置",
				"2. 创建自动备份计划：每日 02:00 增量备份",
				"3. 配置每周日 03:00 全量备份",
				"4. 设置备份保留策略：增量 30 天，全量 90 天",
				"5. 配置异地备份目标（如云存储）",
				"6. 执行一次手动备份验证",
			},
			Priority: SeverityCritical,
		}
	case "bk_restore_test":
		return Remediation{
			CheckID:     r.CheckID,
			Title:       "执行备份恢复测试",
			Description: "备份恢复测试已超过推荐间隔，无法确保备份可恢复",
			Steps: []string{
				"1. 选择最近的备份点",
				"2. 在测试环境执行恢复操作",
				"3. 验证恢复数据的完整性和一致性",
				"4. 记录恢复测试结果",
				"5. 设置自动恢复测试计划（每月一次）",
			},
			Priority: SeverityMedium,
		}
	default:
		return g.generateGenericRemediation(r)
	}
}

// generateGenericRemediation 生成通用整改建议.
func (g *RemediationGenerator) generateGenericRemediation(r ScanResult) Remediation {
	prefix := "修复"
	if r.Status == CheckItemWarning {
		prefix = "改善"
	}

	return Remediation{
		CheckID:     r.CheckID,
		Title:       fmt.Sprintf("%s: %s", prefix, r.Name),
		Description: fmt.Sprintf("检查项 %s 未通过，需要关注: %s", r.Name, r.Message),
		Steps: []string{
			fmt.Sprintf("1. 检查 %s 相关配置", r.Name),
			"2. 根据合规要求调整配置",
			"3. 验证配置更改生效",
			"4. 重新运行合规扫描确认修复",
		},
		Priority: r.Severity,
	}
}
