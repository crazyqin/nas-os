// Package selectiveadsync - 选择性AD同步管理器
// 实现 OU 发现、过滤、同步逻辑
package selectiveadsync

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SelectiveADSyncManager 选择性AD同步管理器
type SelectiveADSyncManager struct {
	mu          sync.RWMutex
	config      OUSyncConfig
	ous         map[string]*OUInfo      // DN -> OU信息
	rules       map[string]*SyncRule    // 规则ID -> 同步规则
	syncHistory []SyncHistory           // 同步历史
	lastResult  *SyncResult             // 最后一次同步结果
	stats       SyncStats               // 统计信息
	ldapConn    interface{}             // LDAP连接（实际使用 go-ldap）
}

// NewSelectiveADSyncManager 创建管理器
func NewSelectiveADSyncManager() *SelectiveADSyncManager {
	return &SelectiveADSyncManager{
		config:      DefaultOUSyncConfig(),
		ous:         make(map[string]*OUInfo),
		rules:       make(map[string]*SyncRule),
		syncHistory: make([]SyncHistory, 0),
		stats:       SyncStats{},
	}
}

// SetConfig 设置配置
func (m *SelectiveADSyncManager) SetConfig(config OUSyncConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// GetConfig 获取配置
func (m *SelectiveADSyncManager) GetConfig() OUSyncConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// ============================================================
// OU 管理
// ============================================================

// DiscoverOUs 发现AD中的OU
func (m *SelectiveADSyncManager) DiscoverOUs() ([]OUInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟从AD发现OU（实际应连接LDAP查询）
	// 这里返回示例数据用于测试
	mockOUs := []OUInfo{
		{
			DN:          "OU=Users,DC=example,DC=com",
			Name:        "Users",
			Description: "用户组织单元",
			ParentDN:    "DC=example,DC=com",
			CreatedAt:   time.Now().Add(-30 * 24 * time.Hour),
			UpdatedAt:   time.Now(),
			ObjectCount: 150,
		},
		{
			DN:          "OU=Groups,DC=example,DC=com",
			Name:        "Groups",
			Description: "组组织单元",
			ParentDN:    "DC=example,DC=com",
			CreatedAt:   time.Now().Add(-30 * 24 * time.Hour),
			UpdatedAt:   time.Now(),
			ObjectCount: 25,
		},
		{
			DN:          "OU=Computers,DC=example,DC=com",
			Name:        "Computers",
			Description: "计算机组织单元",
			ParentDN:    "DC=example,DC=com",
			CreatedAt:   time.Now().Add(-30 * 24 * time.Hour),
			UpdatedAt:   time.Now(),
			ObjectCount: 50,
		},
		{
			DN:          "OU=ServiceAccounts,OU=Users,DC=example,DC=com",
			Name:        "ServiceAccounts",
			Description: "服务账户",
			ParentDN:    "OU=Users,DC=example,DC=com",
			CreatedAt:   time.Now().Add(-15 * 24 * time.Hour),
			UpdatedAt:   time.Now(),
			ObjectCount: 10,
		},
		{
			DN:          "OU=Admins,OU=Users,DC=example,DC=com",
			Name:        "Admins",
			Description: "管理员账户",
			ParentDN:    "OU=Users,DC=example,DC=com",
			CreatedAt:   time.Now().Add(-30 * 24 * time.Hour),
			UpdatedAt:   time.Now(),
			ObjectCount: 5,
		},
	}

	// 更新本地OU缓存
	for _, ou := range mockOUs {
		existing, exists := m.ous[ou.DN]
		if exists {
			ou.IsSelected = existing.IsSelected
		}
		ouCopy := ou
		m.ous[ou.DN] = &ouCopy
	}

	// 更新统计
	m.stats.TotalOUs = len(m.ous)
	m.updateSelectedCount()

	return mockOUs, nil
}

// ListOUs 列出所有已发现的OU
func (m *SelectiveADSyncManager) ListOUs() []OUInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ous := make([]OUInfo, 0, len(m.ous))
	for _, ou := range m.ous {
		ous = append(ous, *ou)
	}
	return ous
}

// SelectOUs 选择要同步的OU
func (m *SelectiveADSyncManager) SelectOUs(ouDNs []string, replace bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果是替换模式，先清除所有选择
	if replace {
		for _, ou := range m.ous {
			ou.IsSelected = false
		}
	}

	// 选择指定的OU
	for _, dn := range ouDNs {
		ou, exists := m.ous[dn]
		if !exists {
			return fmt.Errorf("OU不存在: %s", dn)
		}
		ou.IsSelected = true
	}

	m.updateSelectedCount()
	return nil
}

// DeselectOUs 取消选择OU
func (m *SelectiveADSyncManager) DeselectOUs(ouDNs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, dn := range ouDNs {
		ou, exists := m.ous[dn]
		if !exists {
			return fmt.Errorf("OU不存在: %s", dn)
		}
		ou.IsSelected = false
	}

	m.updateSelectedCount()
	return nil
}

// GetSelectedOUs 获取已选择的OU列表
func (m *SelectiveADSyncManager) GetSelectedOUs() []OUInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	selected := make([]OUInfo, 0)
	for _, ou := range m.ous {
		if ou.IsSelected {
			selected = append(selected, *ou)
		}
	}
	return selected
}

// ============================================================
// 规则管理
// ============================================================

// CreateRule 创建同步规则
func (m *SelectiveADSyncManager) CreateRule(rule SyncRule) (*SyncRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证规则
	if rule.Name == "" {
		return nil, fmt.Errorf("规则名称不能为空")
	}

	// 生成ID
	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}

	// 设置时间
	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	// 存储规则
	ruleCopy := rule
	m.rules[rule.ID] = &ruleCopy

	return &ruleCopy, nil
}

// UpdateRule 更新同步规则
func (m *SelectiveADSyncManager) UpdateRule(rule SyncRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.rules[rule.ID]
	if !exists {
		return fmt.Errorf("规则不存在: %s", rule.ID)
	}

	// 保留创建时间
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()

	ruleCopy := rule
	m.rules[rule.ID] = &ruleCopy

	return nil
}

// DeleteRule 删除同步规则
func (m *SelectiveADSyncManager) DeleteRule(ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[ruleID]; !exists {
		return fmt.Errorf("规则不存在: %s", ruleID)
	}

	delete(m.rules, ruleID)
	return nil
}

// GetRule 获取同步规则
func (m *SelectiveADSyncManager) GetRule(ruleID string) (*SyncRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.rules[ruleID]
	if !exists {
		return nil, fmt.Errorf("规则不存在: %s", ruleID)
	}

	return rule, nil
}

// ListRules 列出所有规则
func (m *SelectiveADSyncManager) ListRules() []SyncRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]SyncRule, 0, len(m.rules))
	for _, rule := range m.rules {
		rules = append(rules, *rule)
	}
	return rules
}

// ============================================================
// 同步逻辑
// ============================================================

// Sync 执行同步
func (m *SelectiveADSyncManager) Sync(req SyncRequest) (*SyncResult, error) {
	m.mu.Lock()

	// 检查是否已在同步中
	if m.lastResult != nil && m.lastResult.Status == SyncStatusSyncing {
		m.mu.Unlock()
		return nil, fmt.Errorf("同步正在进行中")
	}

	// 确定要应用的规则
	var rulesToApply []SyncRule
	if len(req.RuleIDs) > 0 {
		for _, id := range req.RuleIDs {
			rule, exists := m.rules[id]
			if !exists {
				m.mu.Unlock()
				return nil, fmt.Errorf("规则不存在: %s", id)
			}
			rulesToApply = append(rulesToApply, *rule)
		}
	} else {
		for _, rule := range m.rules {
			if rule.Enabled {
				rulesToApply = append(rulesToApply, *rule)
			}
		}
	}

	// 创建同步结果
	result := &SyncResult{
		ID:        uuid.New().String(),
		Status:    SyncStatusSyncing,
		StartTime: time.Now(),
		Details:   make([]SyncDetail, 0),
	}
	m.lastResult = result
	m.mu.Unlock()

	// 执行同步逻辑
	if req.DryRun {
		result.Status = SyncStatusSuccess
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime).String()
		return result, nil
	}

	// 获取要同步的OU列表
	selectedOUs := m.GetSelectedOUs()
	result.TotalOUs = len(selectedOUs)

	// 应用过滤规则
	filteredOUs := m.applyFilters(selectedOUs, rulesToApply)

	// 模拟同步过程
	for _, ou := range filteredOUs {
		detail := SyncDetail{
			OUName: ou.Name,
			Status: "success",
		}

		// 根据规则决定同步的对象类型
		for _, rule := range rulesToApply {
			if m.matchesRule(ou, rule) {
				if rule.SyncUsers {
					detail.ObjectType = "user"
					detail.ObjectCount += ou.ObjectCount
					result.SyncedUsers += ou.ObjectCount
					result.TotalUsers += ou.ObjectCount
				}
				if rule.SyncGroups {
					detail.ObjectType = "group"
					result.TotalGroups += 5
					result.SyncedGroups += 5
				}
				if rule.SyncComputers {
					detail.ObjectType = "computer"
				}
			}
		}

		result.Details = append(result.Details, detail)
		result.SyncedOUs++
	}

	// 更新结果状态
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime).String()
	result.Status = SyncStatusSuccess

	// 更新历史记录
	m.mu.Lock()
	history := SyncHistory{
		ID:        result.ID,
		Status:    result.Status,
		StartTime: result.StartTime,
		EndTime:   result.EndTime,
		Duration:  result.Duration,
		Summary:   fmt.Sprintf("同步了 %d/%d 个OU", result.SyncedOUs, result.TotalOUs),
		RuleCount: len(rulesToApply),
	}
	m.syncHistory = append(m.syncHistory, history)

	// 更新统计
	m.stats.TotalSyncs++
	m.stats.SuccessSyncs++
	m.stats.LastSyncTime = result.EndTime
	m.lastResult = result
	m.mu.Unlock()

	return result, nil
}

// GetLastSyncResult 获取最后一次同步结果
func (m *SelectiveADSyncManager) GetLastSyncResult() *SyncResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastResult
}

// GetSyncHistory 获取同步历史
func (m *SelectiveADSyncManager) GetSyncHistory() []SyncHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := make([]SyncHistory, len(m.syncHistory))
	copy(history, m.syncHistory)
	return history
}

// GetStats 获取统计信息
func (m *SelectiveADSyncManager) GetStats() SyncStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// ============================================================
// 内部方法
// ============================================================

// applyFilters 应用过滤规则
func (m *SelectiveADSyncManager) applyFilters(ous []OUInfo, rules []SyncRule) []OUInfo {
	if len(rules) == 0 {
		return ous
	}

	filtered := make([]OUInfo, 0)
	for _, ou := range ous {
		for _, rule := range rules {
			if rule.Enabled && m.matchesRule(ou, rule) {
				filtered = append(filtered, ou)
				break
			}
		}
	}

	return filtered
}

// matchesRule 检查OU是否匹配规则
func (m *SelectiveADSyncManager) matchesRule(ou OUInfo, rule SyncRule) bool {
	// 检查包含列表
	if len(rule.Filter.IncludeOUs) > 0 {
		found := false
		for _, include := range rule.Filter.IncludeOUs {
			if ou.DN == include || ou.Name == include {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查排除列表
	for _, exclude := range rule.Filter.ExcludeOUs {
		if ou.DN == exclude || ou.Name == exclude {
			return false
		}
	}

	// 检查模式匹配
	if len(rule.Filter.Patterns) > 0 {
		matched := false
		for _, pattern := range rule.Filter.Patterns {
			re, err := regexp.Compile(pattern)
			if err == nil && (re.MatchString(ou.DN) || re.MatchString(ou.Name)) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// updateSelectedCount 更新已选择OU计数
func (m *SelectiveADSyncManager) updateSelectedCount() {
	count := 0
	for _, ou := range m.ous {
		if ou.IsSelected {
			count++
		}
	}
	m.stats.SelectedOUs = count
}
