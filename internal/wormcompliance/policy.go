package wormcompliance

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// PolicyManager 策略管理器
type PolicyManager struct {
	mu       sync.RWMutex
	policies map[string]*Policy
}

// NewPolicyManager 创建策略管理器
func NewPolicyManager() *PolicyManager {
	return &PolicyManager{
		policies: make(map[string]*Policy),
	}
}

// CreatePolicy 创建策略
func (pm *PolicyManager) CreatePolicy(name, description string, mode ComplianceMode, retention RetentionPeriod, applyToPaths []string, regulations []RegulationType) (*Policy, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 验证保留期
	if err := validateRetentionPeriod(retention); err != nil {
		return nil, err
	}

	// 验证合规模式
	if err := validateComplianceMode(mode); err != nil {
		return nil, err
	}

	// 检查路径冲突
	for _, existing := range pm.policies {
		if existing.Enabled {
			for _, newPath := range applyToPaths {
				for _, existPath := range existing.ApplyToPaths {
					if isPathOverlap(newPath, existPath) {
						return nil, fmt.Errorf("路径 %s 与策略 %s 的路径 %s 冲突", newPath, existing.ID, existPath)
					}
				}
			}
		}
	}

	policy := &Policy{
		ID:              uuid.New().String(),
		Name:            name,
		Description:     description,
		Mode:            mode,
		RetentionPeriod: retention,
		Enabled:         true,
		ApplyToPaths:    applyToPaths,
		Regulations:     regulations,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	pm.policies[policy.ID] = policy
	return policy, nil
}

// GetPolicy 获取策略
func (pm *PolicyManager) GetPolicy(id string) (*Policy, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	policy, exists := pm.policies[id]
	if !exists {
		return nil, fmt.Errorf("策略 %s 不存在", id)
	}
	return policy, nil
}

// UpdatePolicy 更新策略
func (pm *PolicyManager) UpdatePolicy(id string, updates map[string]interface{}) (*Policy, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	policy, exists := pm.policies[id]
	if !exists {
		return nil, fmt.Errorf("策略 %s 不存在", id)
	}

	// 只允许更新部分字段
	if name, ok := updates["name"].(string); ok {
		policy.Name = name
	}
	if desc, ok := updates["description"].(string); ok {
		policy.Description = desc
	}
	if enabled, ok := updates["enabled"].(bool); ok {
		policy.Enabled = enabled
	}
	if paths, ok := updates["apply_to_paths"].([]string); ok {
		policy.ApplyToPaths = paths
	}
	if regs, ok := updates["regulations"].([]RegulationType); ok {
		policy.Regulations = regs
	}

	policy.UpdatedAt = time.Now()
	return policy, nil
}

// DeletePolicy 删除策略
func (pm *PolicyManager) DeletePolicy(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.policies[id]; !exists {
		return fmt.Errorf("策略 %s 不存在", id)
	}

	delete(pm.policies, id)
	return nil
}

// ListPolicies 列出所有策略
func (pm *PolicyManager) ListPolicies() []*Policy {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	policies := make([]*Policy, 0, len(pm.policies))
	for _, p := range pm.policies {
		policies = append(policies, p)
	}
	return policies
}

// GetPoliciesForPath 获取适用于指定路径的策略
func (pm *PolicyManager) GetPoliciesForPath(path string) []*Policy {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var matched []*Policy
	for _, p := range pm.policies {
		if !p.Enabled {
			continue
		}
		for _, policyPath := range p.ApplyToPaths {
			if isPathMatch(path, policyPath) {
				matched = append(matched, p)
				break
			}
		}
	}
	return matched
}

// GetPoliciesByRegulation 按法规类型获取策略
func (pm *PolicyManager) GetPoliciesByRegulation(reg RegulationType) []*Policy {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var matched []*Policy
	for _, p := range pm.policies {
		for _, r := range p.Regulations {
			if r == reg {
				matched = append(matched, p)
				break
			}
		}
	}
	return matched
}

// validateRetentionPeriod 验证保留期
func validateRetentionPeriod(rp RetentionPeriod) error {
	if rp.Unit == RetentionForever {
		return nil
	}
	if rp.Value <= 0 {
		return fmt.Errorf("保留期必须大于0")
	}
	switch rp.Unit {
	case RetentionDays, RetentionMonths, RetentionYears:
		return nil
	default:
		return fmt.Errorf("无效的保留期单位: %s", rp.Unit)
	}
}

// validateComplianceMode 验证合规模式
func validateComplianceMode(mode ComplianceMode) error {
	switch mode {
	case ModeGovernance, ModeEnterprise, ModeRegulatory:
		return nil
	default:
		return fmt.Errorf("无效的合规模式: %s", mode)
	}
}

// isPathOverlap 检查路径是否重叠
func isPathOverlap(path1, path2 string) bool {
	// 简单路径重叠检查
	if path1 == path2 {
		return true
	}
	// 检查是否一个路径是另一个的子目录
	if len(path1) > len(path2) && path1[:len(path2)] == path2 && path1[len(path2)] == '/' {
		return true
	}
	if len(path2) > len(path1) && path2[:len(path1)] == path1 && path2[len(path1)] == '/' {
		return true
	}
	return false
}

// isPathMatch 检查路径是否匹配策略路径
func isPathMatch(path, policyPath string) bool {
	if path == policyPath {
		return true
	}
	// 检查路径是否在策略路径下
	if len(path) > len(policyPath) && path[:len(policyPath)] == policyPath && path[len(policyPath)] == '/' {
		return true
	}
	// 支持通配符 *
	if policyPath == "*" {
		return true
	}
	return false
}
