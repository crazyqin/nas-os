package smartarchive

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// RetentionManager 数据保留管理器.
type RetentionManager struct {
	mu sync.RWMutex

	// 保留规则
	rules map[string]*RetentionRule

	// 文件保留状态
	retentionStatus map[string]*FileRetentionStatus

	// 合规检查结果
	complianceResults map[string]*ComplianceResult

	// 统计
	stats *RetentionStats
}

// FileRetentionStatus 文件保留状态.
type FileRetentionStatus struct {
	FilePath      string           `json:"filePath"`
	RuleID        string           `json:"ruleId"`
	RuleName      string           `json:"ruleName"`
	Status        ComplianceStatus `json:"status"`
	RetainedUntil time.Time        `json:"retainedUntil"`
	IsLocked      bool             `json:"isLocked"`
	IsLegalHold   bool             `json:"isLegalHold"`
	LastChecked   time.Time        `json:"lastChecked"`
	Exemptions    []string         `json:"exemptions,omitempty"`
	Notes         string           `json:"notes,omitempty"`
}

// ComplianceResult 合规检查结果.
type ComplianceResult struct {
	ID             string            `json:"id"`
	RuleID         string            `json:"ruleId"`
	RuleName       string            `json:"ruleName"`
	CheckedAt      time.Time         `json:"checkedAt"`
	Status         ComplianceStatus  `json:"status"`
	TotalFiles     int64             `json:"totalFiles"`
	Compliant      int64             `json:"compliant"`
	NonCompliant   int64             `json:"nonCompliant"`
	Warnings       int64             `json:"warnings"`
	Exempted       int64             `json:"exempted"`
	Expired        int64             `json:"expired"`
	ComplianceRate float64           `json:"complianceRate"`
	Issues         []ComplianceIssue `json:"issues,omitempty"`
	NextCheck      time.Time         `json:"nextCheck"`
}

// ComplianceIssue 合规问题.
type ComplianceIssue struct {
	FilePath    string    `json:"filePath"`
	IssueType   string    `json:"issueType"` // expired/missing_tag/invalid_state
	Severity    string    `json:"severity"`  // high/medium/low
	Description string    `json:"description"`
	Suggestion  string    `json:"suggestion"`
	DetectedAt  time.Time `json:"detectedAt"`
}

// RetentionStats 保留统计.
type RetentionStats struct {
	TotalRules          int64                      `json:"totalRules"`
	ActiveRules         int64                      `json:"activeRules"`
	TotalFiles          int64                      `json:"totalFiles"`
	RetainedFiles       int64                      `json:"retainedFiles"`
	ExpiredFiles        int64                      `json:"expiredFiles"`
	LockedFiles         int64                      `json:"lockedFiles"`
	LegalHoldFiles      int64                      `json:"legalHoldFiles"`
	LastComplianceCheck time.Time                  `json:"lastComplianceCheck"`
	ComplianceRate      float64                    `json:"complianceRate"`
	ByStatus            map[ComplianceStatus]int64 `json:"byStatus"`
}

// RetentionAction 执行保留动作.
type RetentionActionResult struct {
	RuleID         string          `json:"ruleId"`
	RuleName       string          `json:"ruleName"`
	Action         RetentionAction `json:"action"`
	ExecutedAt     time.Time       `json:"executedAt"`
	FilesProcessed int64           `json:"filesProcessed"`
	FilesAffected  int64           `json:"filesAffected"`
	Success        bool            `json:"success"`
	Error          string          `json:"error,omitempty"`
	Details        string          `json:"details,omitempty"`
}

// NewRetentionManager 创建保留管理器.
func NewRetentionManager() *RetentionManager {
	return &RetentionManager{
		rules:             make(map[string]*RetentionRule),
		retentionStatus:   make(map[string]*FileRetentionStatus),
		complianceResults: make(map[string]*ComplianceResult),
		stats: &RetentionStats{
			ByStatus: make(map[ComplianceStatus]int64),
		},
	}
}

// AddRule 添加保留规则.
func (rm *RetentionManager) AddRule(rule *RetentionRule) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("规则 ID 不能为空")
	}

	if _, exists := rm.rules[rule.ID]; exists {
		return fmt.Errorf("规则 %s 已存在", rule.ID)
	}

	if err := rm.validateRule(rule); err != nil {
		return fmt.Errorf("规则验证失败: %w", err)
	}

	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	rm.rules[rule.ID] = rule
	rm.stats.TotalRules++
	if rule.Enabled {
		rm.stats.ActiveRules++
	}

	log.Printf("[Retention] 添加保留规则: %s (%s)", rule.Name, rule.ID)
	return nil
}

// UpdateRule 更新保留规则.
func (rm *RetentionManager) UpdateRule(rule *RetentionRule) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.rules[rule.ID]; !exists {
		return fmt.Errorf("规则 %s 不存在", rule.ID)
	}

	if err := rm.validateRule(rule); err != nil {
		return fmt.Errorf("规则验证失败: %w", err)
	}

	rule.UpdatedAt = time.Now()
	rm.rules[rule.ID] = rule

	return nil
}

// RemoveRule 删除保留规则.
func (rm *RetentionManager) RemoveRule(ruleID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rule, exists := rm.rules[ruleID]
	if !exists {
		return fmt.Errorf("规则 %s 不存在", ruleID)
	}

	// 检查是否有文件关联
	for _, status := range rm.retentionStatus {
		if status.RuleID == ruleID {
			return fmt.Errorf("规则 %s 仍有文件关联，无法删除", ruleID)
		}
	}

	delete(rm.rules, ruleID)
	if rule.Enabled {
		rm.stats.ActiveRules--
	}
	rm.stats.TotalRules--

	return nil
}

// GetRule 获取保留规则.
func (rm *RetentionManager) GetRule(ruleID string) (*RetentionRule, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	rule, exists := rm.rules[ruleID]
	if !exists {
		return nil, fmt.Errorf("规则 %s 不存在", ruleID)
	}

	return rule, nil
}

// ListRules 列出所有保留规则.
func (rm *RetentionManager) ListRules() []*RetentionRule {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	rules := make([]*RetentionRule, 0, len(rm.rules))
	for _, r := range rm.rules {
		rules = append(rules, r)
	}

	return rules
}

// CheckRetention 检查文件保留状态.
func (rm *RetentionManager) CheckRetention(filePath string, metadata *FileMetadata) *RetentionCheckResult {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := &RetentionCheckResult{
		FilePath:  filePath,
		CheckedAt: time.Now(),
		Rules:     make([]RetentionRuleMatch, 0),
	}

	// 检查是否被锁定
	if status, exists := rm.retentionStatus[filePath]; exists {
		result.IsLocked = status.IsLocked
		result.IsLegalHold = status.IsLegalHold
		result.RetainedUntil = status.RetainedUntil
	}

	// 检查所有规则
	for _, rule := range rm.rules {
		if !rule.Enabled {
			continue
		}

		match, ok := rm.evaluateRule(rule, metadata)
		if ok {
			result.Rules = append(result.Rules, match)
			result.HasRetention = true

			// 更新保留时间
			if match.RetentionDuration > 0 {
				retainUntil := metadata.CreatedAt.Add(match.RetentionDuration)
				if retainUntil.After(result.RetainedUntil) {
					result.RetainedUntil = retainUntil
				}
			}
		}
	}

	// 判断是否可以删除
	result.CanDelete = rm.canDelete(result)

	return result
}

// RetentionCheckResult 保留检查结果.
type RetentionCheckResult struct {
	FilePath      string               `json:"filePath"`
	CheckedAt     time.Time            `json:"checkedAt"`
	HasRetention  bool                 `json:"hasRetention"`
	IsLocked      bool                 `json:"isLocked"`
	IsLegalHold   bool                 `json:"isLegalHold"`
	RetainedUntil time.Time            `json:"retainedUntil"`
	CanDelete     bool                 `json:"canDelete"`
	Rules         []RetentionRuleMatch `json:"rules"`
}

// RetentionRuleMatch 保留规则匹配.
type RetentionRuleMatch struct {
	RuleID             string          `json:"ruleId"`
	RuleName           string          `json:"ruleName"`
	Action             RetentionAction `json:"action"`
	RetentionDuration  time.Duration   `json:"retentionDuration"`
	ComplianceRequired bool            `json:"complianceRequired"`
	IsLegalHold        bool            `json:"isLegalHold"`
	MatchedConditions  []string        `json:"matchedConditions"`
}

// evaluateRule 评估规则.
func (rm *RetentionManager) evaluateRule(rule *RetentionRule, metadata *FileMetadata) (RetentionRuleMatch, bool) {
	match := RetentionRuleMatch{
		RuleID:             rule.ID,
		RuleName:           rule.Name,
		Action:             rule.Action,
		ComplianceRequired: rule.ComplianceRequired,
		IsLegalHold:        rule.LegalHold,
		MatchedConditions:  make([]string, 0),
	}

	conditions := &rule.Conditions
	matched := true

	// 检查最大保留时间
	if conditions.MaxAge > 0 {
		match.RetentionDuration = conditions.MaxAge
		age := time.Since(metadata.CreatedAt)
		if age > conditions.MaxAge {
			matched = false
		} else {
			match.MatchedConditions = append(match.MatchedConditions, "max_age")
		}
	}

	// 检查最小保留时间
	if conditions.MinAge > 0 {
		age := time.Since(metadata.CreatedAt)
		if age < conditions.MinAge {
			matched = false
		} else {
			match.MatchedConditions = append(match.MatchedConditions, "min_age")
		}
	}

	// 检查过期时间
	if !conditions.ExpiresBefore.IsZero() {
		if metadata.CreatedAt.After(conditions.ExpiresBefore) {
			matched = false
		} else {
			match.MatchedConditions = append(match.MatchedConditions, "expires_before")
		}
	}

	// 检查路径模式
	if len(conditions.PathPatterns) > 0 {
		pathMatched := false
		for _, pattern := range conditions.PathPatterns {
			if matchPathPattern(metadata.Path, pattern) {
				pathMatched = true
				break
			}
		}
		if !pathMatched {
			matched = false
		} else {
			match.MatchedConditions = append(match.MatchedConditions, "path_pattern")
		}
	}

	// 检查排除路径
	if len(conditions.ExcludePaths) > 0 {
		for _, pattern := range conditions.ExcludePaths {
			if matchPathPattern(metadata.Path, pattern) {
				matched = false
				break
			}
		}
	}

	// 检查大小条件
	if conditions.MinSize > 0 && metadata.Size < conditions.MinSize {
		matched = false
	}
	if conditions.MaxSize > 0 && metadata.Size > conditions.MaxSize {
		matched = false
	}
	if matched && (conditions.MinSize > 0 || conditions.MaxSize > 0) {
		match.MatchedConditions = append(match.MatchedConditions, "size")
	}

	// 检查标签条件
	if len(conditions.RequiredTags) > 0 {
		if !hasAllTags(metadata.Tags, conditions.RequiredTags) {
			matched = false
		} else {
			match.MatchedConditions = append(match.MatchedConditions, "required_tags")
		}
	}

	if len(conditions.ExcludeTags) > 0 {
		if hasAnyTag(metadata.Tags, conditions.ExcludeTags) {
			matched = false
		}
	}

	// 检查文件扩展名
	if len(conditions.FileExtensions) > 0 {
		ext := getFileExtension(metadata.Path)
		extMatched := false
		for _, allowedExt := range conditions.FileExtensions {
			if ext == allowedExt || ext == "."+allowedExt {
				extMatched = true
				break
			}
		}
		if !extMatched {
			matched = false
		} else {
			match.MatchedConditions = append(match.MatchedConditions, "file_extension")
		}
	}

	return match, matched
}

// canDelete 判断是否可以删除.
func (rm *RetentionManager) canDelete(result *RetentionCheckResult) bool {
	// 法律保留的文件不能删除
	if result.IsLegalHold {
		return false
	}

	// 锁定的文件不能删除
	if result.IsLocked {
		return false
	}

	// 检查保留期限
	if !result.RetainedUntil.IsZero() && time.Now().Before(result.RetainedUntil) {
		return false
	}

	return true
}

// SetRetention 设置文件保留状态.
func (rm *RetentionManager) SetRetention(filePath, ruleID string, duration time.Duration) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rule, exists := rm.rules[ruleID]
	if !exists {
		return fmt.Errorf("规则 %s 不存在", ruleID)
	}

	status := &FileRetentionStatus{
		FilePath:      filePath,
		RuleID:        ruleID,
		RuleName:      rule.Name,
		Status:        ComplianceStatusCompliant,
		RetainedUntil: time.Now().Add(duration),
		IsLocked:      true,
		LastChecked:   time.Now(),
	}

	rm.retentionStatus[filePath] = status
	rm.stats.RetainedFiles++
	rm.stats.LockedFiles++
	rm.stats.TotalFiles++

	return nil
}

// ReleaseRetention 释放文件保留.
func (rm *RetentionManager) ReleaseRetention(filePath string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	status, exists := rm.retentionStatus[filePath]
	if !exists {
		return fmt.Errorf("文件 %s 没有保留状态", filePath)
	}

	if status.IsLegalHold {
		return fmt.Errorf("文件 %s 处于法律保留状态，无法释放", filePath)
	}

	if status.IsLocked && time.Now().Before(status.RetainedUntil) {
		return fmt.Errorf("文件 %s 仍在保留期内，无法释放", filePath)
	}

	delete(rm.retentionStatus, filePath)
	rm.stats.RetainedFiles--
	rm.stats.LockedFiles--

	return nil
}

// SetLegalHold 设置法律保留.
func (rm *RetentionManager) SetLegalHold(filePath string, enabled bool) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	status, exists := rm.retentionStatus[filePath]
	if !exists {
		status = &FileRetentionStatus{
			FilePath: filePath,
			Status:   ComplianceStatusCompliant,
		}
		rm.retentionStatus[filePath] = status
	}

	status.IsLegalHold = enabled
	status.IsLocked = enabled

	if enabled {
		rm.stats.LegalHoldFiles++
	} else {
		rm.stats.LegalHoldFiles--
	}

	return nil
}

// RunComplianceCheck 运行合规检查.
func (rm *RetentionManager) RunComplianceCheck() *ComplianceResult {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	result := &ComplianceResult{
		ID:        generateID(),
		CheckedAt: time.Now(),
		Issues:    make([]ComplianceIssue, 0),
	}

	// 检查所有保留状态
	for filePath, status := range rm.retentionStatus {
		result.TotalFiles++

		// 检查是否过期
		if !status.RetainedUntil.IsZero() && time.Now().After(status.RetainedUntil) {
			if status.IsLegalHold {
				result.Exempted++
				status.Status = ComplianceStatusExempt
			} else {
				result.Expired++
				status.Status = ComplianceStatusWarning
				result.Issues = append(result.Issues, ComplianceIssue{
					FilePath:    filePath,
					IssueType:   "expired",
					Severity:    "medium",
					Description: fmt.Sprintf("文件保留期已过期: %s", status.RetainedUntil),
					Suggestion:  "检查是否需要续期或释放保留",
					DetectedAt:  time.Now(),
				})
			}
		} else {
			result.Compliant++
			status.Status = ComplianceStatusCompliant
		}

		status.LastChecked = time.Now()
	}

	// 计算合规率
	if result.TotalFiles > 0 {
		result.ComplianceRate = float64(result.Compliant) / float64(result.TotalFiles) * 100
	}

	// 更新统计
	rm.stats.LastComplianceCheck = time.Now()
	rm.stats.ComplianceRate = result.ComplianceRate

	// 保存结果
	rm.complianceResults[result.ID] = result

	return result
}

// ExecuteRetentionAction 执行保留动作.
func (rm *RetentionManager) ExecuteRetentionAction(ruleID string) *RetentionActionResult {
	rm.mu.RLock()
	rule, exists := rm.rules[ruleID]
	rm.mu.RUnlock()

	if !exists {
		return &RetentionActionResult{
			RuleID:  ruleID,
			Success: false,
			Error:   fmt.Sprintf("规则 %s 不存在", ruleID),
		}
	}

	result := &RetentionActionResult{
		RuleID:     ruleID,
		RuleName:   rule.Name,
		Action:     rule.Action,
		ExecutedAt: time.Now(),
	}

	// 执行动作
	switch rule.Action {
	case RetentionActionArchive:
		result.Details = "归档到冷存储"
		result.Success = true
	case RetentionActionDelete:
		result.Details = "删除过期文件"
		result.Success = true
	case RetentionActionNotify:
		result.Details = "发送通知"
		result.Success = true
	case RetentionActionMoveToIce:
		result.Details = "移动到冰冻层"
		result.Success = true
	default:
		result.Error = fmt.Sprintf("未知动作: %s", rule.Action)
	}

	return result
}

// GetStats 获取保留统计.
func (rm *RetentionManager) GetStats() *RetentionStats {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return rm.stats
}

// GetRetentionStatus 获取文件保留状态.
func (rm *RetentionManager) GetRetentionStatus(filePath string) (*FileRetentionStatus, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	status, exists := rm.retentionStatus[filePath]
	if !exists {
		return nil, fmt.Errorf("文件 %s 没有保留状态", filePath)
	}

	return status, nil
}

// ListRetentionStatus 列出所有保留状态.
func (rm *RetentionManager) ListRetentionStatus() []*FileRetentionStatus {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	statuses := make([]*FileRetentionStatus, 0, len(rm.retentionStatus))
	for _, s := range rm.retentionStatus {
		statuses = append(statuses, s)
	}

	return statuses
}

// validateRule 验证规则.
func (rm *RetentionManager) validateRule(rule *RetentionRule) error {
	if rule.Name == "" {
		return fmt.Errorf("规则名称不能为空")
	}

	// 验证动作
	switch rule.Action {
	case RetentionActionArchive, RetentionActionDelete, RetentionActionNotify, RetentionActionMoveToIce:
		// 有效
	default:
		return fmt.Errorf("无效的保留动作: %s", rule.Action)
	}

	// 验证宽限期
	if rule.GracePeriod < 0 {
		return fmt.Errorf("宽限期不能为负数")
	}

	return nil
}

// GetExpiredFiles 获取过期文件列表.
func (rm *RetentionManager) GetExpiredFiles() []string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	expired := make([]string, 0)
	now := time.Now()

	for filePath, status := range rm.retentionStatus {
		if !status.RetainedUntil.IsZero() && now.After(status.RetainedUntil) {
			if !status.IsLegalHold {
				expired = append(expired, filePath)
			}
		}
	}

	return expired
}

// GetLegalHoldFiles 获取法律保留文件列表.
func (rm *RetentionManager) GetLegalHoldFiles() []string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	files := make([]string, 0)

	for filePath, status := range rm.retentionStatus {
		if status.IsLegalHold {
			files = append(files, filePath)
		}
	}

	return files
}
