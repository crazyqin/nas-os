package wormcompliance

import (
	"fmt"
	"sync"
	"time"
)

// WORMManager WORM 合规管理器
type WORMManager struct {
	mu              sync.RWMutex
	config          WORMConfig
	policyManager   *PolicyManager
	immutabilityMgr *ImmutabilityManager
	auditManager    *AuditManager
}

// NewWORMManager 创建 WORM 管理器
func NewWORMManager(config WORMConfig) *WORMManager {
	return &WORMManager{
		config:          config,
		policyManager:   NewPolicyManager(),
		immutabilityMgr: NewImmutabilityManager(config),
		auditManager:    NewAuditManager(config.MaxAuditRetentionDays),
	}
}

// CreatePolicy 创建策略
func (wm *WORMManager) CreatePolicy(name, description string, mode ComplianceMode, retention RetentionPeriod, applyToPaths []string, regulations []RegulationType) (*Policy, error) {
	policy, err := wm.policyManager.CreatePolicy(name, description, mode, retention, applyToPaths, regulations)
	if err != nil {
		wm.auditManager.LogAction("", "create_policy", "system", fmt.Sprintf("创建策略失败: %s", name), "", false, err.Error())
		return nil, err
	}

	wm.auditManager.LogAction(policy.ID, "create_policy", "system", fmt.Sprintf("创建策略: %s", name), "", true, "")
	return policy, nil
}

// GetPolicy 获取策略
func (wm *WORMManager) GetPolicy(id string) (*Policy, error) {
	return wm.policyManager.GetPolicy(id)
}

// UpdatePolicy 更新策略
func (wm *WORMManager) UpdatePolicy(id string, updates map[string]interface{}) (*Policy, error) {
	policy, err := wm.policyManager.UpdatePolicy(id, updates)
	if err != nil {
		wm.auditManager.LogAction(id, "update_policy", "system", "更新策略失败", "", false, err.Error())
		return nil, err
	}

	wm.auditManager.LogAction(id, "update_policy", "system", fmt.Sprintf("更新策略: %s", policy.Name), "", true, "")
	return policy, nil
}

// DeletePolicy 删除策略
func (wm *WORMManager) DeletePolicy(id string) error {
	// 检查是否有对象使用此策略
	objects := wm.immutabilityMgr.ListObjects()
	for _, obj := range objects {
		if obj.PolicyID == id {
			return fmt.Errorf("策略 %s 仍有对象使用，无法删除", id)
		}
	}

	err := wm.policyManager.DeletePolicy(id)
	if err != nil {
		wm.auditManager.LogAction(id, "delete_policy", "system", "删除策略失败", "", false, err.Error())
		return err
	}

	wm.auditManager.LogAction(id, "delete_policy", "system", "删除策略", "", true, "")
	return nil
}

// ListPolicies 列出所有策略
func (wm *WORMManager) ListPolicies() []*Policy {
	return wm.policyManager.ListPolicies()
}

// ProtectObject 保护对象
func (wm *WORMManager) ProtectObject(path string, size int64, policyID, actor string, metadata map[string]string) (*ProtectedObject, error) {
	// 验证策略存在
	policy, err := wm.policyManager.GetPolicy(policyID)
	if err != nil {
		wm.auditManager.LogAction("", "protect_object", actor, fmt.Sprintf("保护对象失败: %s", path), "", false, err.Error())
		return nil, err
	}

	// 检查路径是否在策略范围内
	if !isPathInPolicy(path, policy) {
		err := fmt.Errorf("路径 %s 不在策略 %s 的保护范围内", path, policyID)
		wm.auditManager.LogAction("", "protect_object", actor, fmt.Sprintf("保护对象失败: %s", path), "", false, err.Error())
		return nil, err
	}

	obj, err := wm.immutabilityMgr.ProtectObject(path, size, policyID, actor, metadata)
	if err != nil {
		wm.auditManager.LogAction("", "protect_object", actor, fmt.Sprintf("保护对象失败: %s", path), "", false, err.Error())
		return nil, err
	}

	wm.auditManager.LogAction(obj.ID, "protect_object", actor, fmt.Sprintf("保护对象: %s", path), "", true, "")
	return obj, nil
}

// LockObject 锁定对象
func (wm *WORMManager) LockObject(objectID, actor string) error {
	// 获取对象
	obj, err := wm.immutabilityMgr.GetObject(objectID)
	if err != nil {
		wm.auditManager.LogAction(objectID, "lock_object", actor, "锁定对象失败", "", false, err.Error())
		return err
	}

	// 获取策略
	policy, err := wm.policyManager.GetPolicy(obj.PolicyID)
	if err != nil {
		wm.auditManager.LogAction(objectID, "lock_object", actor, "锁定对象失败", "", false, err.Error())
		return err
	}

	err = wm.immutabilityMgr.LockObject(objectID, policy.RetentionPeriod)
	if err != nil {
		wm.auditManager.LogAction(objectID, "lock_object", actor, "锁定对象失败", "", false, err.Error())
		return err
	}

	wm.auditManager.LogAction(objectID, "lock_object", actor, fmt.Sprintf("锁定对象，保留期: %s", policy.RetentionPeriod.Unit), "", true, "")
	return nil
}

// VerifyObject 验证对象完整性
func (wm *WORMManager) VerifyObject(objectID string) (bool, error) {
	return wm.immutabilityMgr.VerifyObject(objectID)
}

// VerifyHashChain 验证哈希链完整性
func (wm *WORMManager) VerifyHashChain() (bool, error) {
	return wm.immutabilityMgr.VerifyHashChain()
}

// VerifyAuditChain 验证审计链完整性
func (wm *WORMManager) VerifyAuditChain() (bool, error) {
	return wm.auditManager.VerifyAuditChain()
}

// DeleteObject 删除对象（根据合规模式）
func (wm *WORMManager) DeleteObject(objectID, actor string) error {
	obj, err := wm.immutabilityMgr.GetObject(objectID)
	if err != nil {
		wm.auditManager.LogAction(objectID, "delete_object", actor, "删除对象失败", "", false, err.Error())
		return err
	}

	// 获取策略
	policy, err := wm.policyManager.GetPolicy(obj.PolicyID)
	if err != nil {
		wm.auditManager.LogAction(objectID, "delete_object", actor, "删除对象失败", "", false, err.Error())
		return err
	}

	// 检查是否可以删除
	canDelete, reason := wm.immutabilityMgr.CanDelete(objectID, policy.Mode, time.Now())
	if !canDelete {
		wm.auditManager.LogAction(objectID, "delete_object", actor, fmt.Sprintf("删除被拒绝: %s", reason), "", false, reason)
		return fmt.Errorf("无法删除对象 %s: %s", objectID, reason)
	}

	err = wm.immutabilityMgr.RemoveObject(objectID)
	if err != nil {
		wm.auditManager.LogAction(objectID, "delete_object", actor, "删除对象失败", "", false, err.Error())
		return err
	}

	wm.auditManager.LogAction(objectID, "delete_object", actor, fmt.Sprintf("删除对象，原因: %s", reason), "", true, "")
	return nil
}

// GetObject 获取对象
func (wm *WORMManager) GetObject(objectID string) (*ProtectedObject, error) {
	return wm.immutabilityMgr.GetObject(objectID)
}

// ListObjects 列出所有对象
func (wm *WORMManager) ListObjects() []*ProtectedObject {
	return wm.immutabilityMgr.ListObjects()
}

// ListExpiredObjects 列出过期对象
func (wm *WORMManager) ListExpiredObjects() []*ProtectedObject {
	return wm.immutabilityMgr.ListExpiredObjects(time.Now())
}

// GetAuditLog 获取审计日志
func (wm *WORMManager) GetAuditLog(limit int) []*AuditEntry {
	return wm.auditManager.GetEntries(limit)
}

// GetAuditLogForObject 获取指定对象的审计日志
func (wm *WORMManager) GetAuditLogForObject(objectID string) []*AuditEntry {
	return wm.auditManager.GetEntriesForObject(objectID)
}

// GetAuditStats 获取审计统计
func (wm *WORMManager) GetAuditStats() map[string]interface{} {
	return wm.auditManager.GetStats()
}

// GenerateComplianceReport 生成合规报告
func (wm *WORMManager) GenerateComplianceReport(regulationType RegulationType) (*ComplianceReport, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	// 获取相关策略
	policies := wm.policyManager.GetPoliciesByRegulation(regulationType)

	// 获取所有对象
	objects := wm.immutabilityMgr.ListObjects()

	// 统计
	totalObjects := len(objects)
	protectedObjects := 0
	expiredObjects := 0
	totalPolicies := len(policies)
	activePolicies := 0
	var storageUsed int64

	var violations []ComplianceViolation

	for _, obj := range objects {
		storageUsed += obj.Size
		if obj.Locked {
			protectedObjects++
		}
		if obj.ExpiresAt != nil && time.Now().After(*obj.ExpiresAt) {
			expiredObjects++
		}

		// 检查合规违规
		policy, err := wm.policyManager.GetPolicy(obj.PolicyID)
		if err == nil {
			for _, reg := range policy.Regulations {
				if reg == regulationType {
					// 检查对象是否符合法规要求
					if violation := wm.checkRegulationCompliance(obj, policy, regulationType); violation != nil {
						violations = append(violations, *violation)
					}
					break
				}
			}
		}
	}

	for _, p := range policies {
		if p.Enabled {
			activePolicies++
		}
	}

	// 确定合规状态
	status := StatusCompliant
	if len(violations) > 0 {
		status = StatusNonCompliant
	}

	// 生成建议
	recommendations := wm.generateRecommendations(regulationType, violations)

	report := &ComplianceReport{
		ID:             fmt.Sprintf("report-%s-%d", regulationType, time.Now().Unix()),
		GeneratedAt:    time.Now(),
		RegulationType: regulationType,
		Status:         status,
		Summary: ReportSummary{
			TotalObjects:     totalObjects,
			ProtectedObjects: protectedObjects,
			ExpiredObjects:   expiredObjects,
			TotalPolicies:    totalPolicies,
			ActivePolicies:   activePolicies,
			TotalAuditLogs:   wm.auditManager.GetEntryCount(),
			StorageUsedBytes: storageUsed,
		},
		Violations:      violations,
		Recommendations: recommendations,
	}

	wm.auditManager.LogAction(report.ID, "generate_report", "system", fmt.Sprintf("生成 %s 合规报告", regulationType), "", true, "")
	return report, nil
}

// GetConfig 获取配置
func (wm *WORMManager) GetConfig() WORMConfig {
	return wm.config
}

// UpdateConfig 更新配置
func (wm *WORMManager) UpdateConfig(config WORMConfig) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.config = config
}

// PurgeOldAuditLogs 清理旧审计日志
func (wm *WORMManager) PurgeOldAuditLogs() int {
	return wm.auditManager.PurgeOldEntries()
}

// isPathInPolicy 检查路径是否在策略范围内
func isPathInPolicy(path string, policy *Policy) bool {
	for _, policyPath := range policy.ApplyToPaths {
		if isPathMatch(path, policyPath) {
			return true
		}
	}
	return false
}

// checkRegulationCompliance 检查法规合规性
func (wm *WORMManager) checkRegulationCompliance(obj *ProtectedObject, policy *Policy, regulation RegulationType) *ComplianceViolation {
	switch regulation {
	case RegulationGDPR:
		return wm.checkGDPRCompliance(obj, policy)
	case RegulationSOX:
		return wm.checkSOXCompliance(obj, policy)
	case RegulationHIPAA:
		return wm.checkHIPAACompliance(obj, policy)
	default:
		return nil
	}
}

// checkGDPRCompliance 检查 GDPR 合规性
func (wm *WORMManager) checkGDPRCompliance(obj *ProtectedObject, policy *Policy) *ComplianceViolation {
	// GDPR 要求: 数据保留期必须合理，不能无限期保留
	if policy.RetentionPeriod.IsForever() {
		return &ComplianceViolation{
			ObjectID:      obj.ID,
			Path:          obj.Path,
			ViolationType: "retention_period",
			Severity:      "high",
			Description:   "GDPR 不允许无限期保留个人数据",
			DetectedAt:    time.Now(),
		}
	}
	return nil
}

// checkSOXCompliance 检查 SOX 合规性
func (wm *WORMManager) checkSOXCompliance(obj *ProtectedObject, policy *Policy) *ComplianceViolation {
	// SOX 要求: 财务记录必须保留至少7年
	if !policy.RetentionPeriod.IsForever() {
		duration := policy.RetentionPeriod.GetDuration()
		if duration < 7*365*24*time.Hour {
			return &ComplianceViolation{
				ObjectID:      obj.ID,
				Path:          obj.Path,
				ViolationType: "retention_period",
				Severity:      "high",
				Description:   "SOX 要求财务记录保留至少7年",
				DetectedAt:    time.Now(),
			}
		}
	}
	return nil
}

// checkHIPAACompliance 检查 HIPAA 合规性
func (wm *WORMManager) checkHIPAACompliance(obj *ProtectedObject, policy *Policy) *ComplianceViolation {
	// HIPAA 要求: 医疗记录必须保留至少6年
	if !policy.RetentionPeriod.IsForever() {
		duration := policy.RetentionPeriod.GetDuration()
		if duration < 6*365*24*time.Hour {
			return &ComplianceViolation{
				ObjectID:      obj.ID,
				Path:          obj.Path,
				ViolationType: "retention_period",
				Severity:      "high",
				Description:   "HIPAA 要求医疗记录保留至少6年",
				DetectedAt:    time.Now(),
			}
		}
	}
	return nil
}

// generateRecommendations 生成建议
func (wm *WORMManager) generateRecommendations(regulation RegulationType, violations []ComplianceViolation) []string {
	var recommendations []string

	switch regulation {
	case RegulationGDPR:
		recommendations = append(recommendations, "确保所有个人数据都有明确的保留期限")
		recommendations = append(recommendations, "实施数据最小化原则")
		recommendations = append(recommendations, "建立数据主体权利响应流程")
	case RegulationSOX:
		recommendations = append(recommendations, "财务记录保留期至少7年")
		recommendations = append(recommendations, "确保审计日志完整性")
		recommendations = append(recommendations, "实施严格的访问控制")
	case RegulationHIPAA:
		recommendations = append(recommendations, "医疗记录保留期至少6年")
		recommendations = append(recommendations, "实施数据加密")
		recommendations = append(recommendations, "建立访问审计机制")
	}

	if len(violations) > 0 {
		recommendations = append(recommendations, "立即修复发现的合规违规")
		recommendations = append(recommendations, "加强合规监控和审计")
	}

	return recommendations
}
