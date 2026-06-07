package smartarchive

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PolicyEngine 归档策略引擎.
type PolicyEngine struct {
	mu sync.RWMutex

	// 策略列表
	policies map[string]*ArchivePolicy

	// 规则列表
	rules map[string]*RetentionRule

	// 文件元数据缓存
	fileCache map[string]*FileMetadata

	// 统计
	stats *PolicyStats
}

// FileMetadata 文件元数据.
type FileMetadata struct {
	Path        string      `json:"path"`
	Size        int64       `json:"size"`
	Extension   string      `json:"extension"`
	MimeType    string      `json:"mimeType"`
	CreatedAt   time.Time   `json:"createdAt"`
	ModifiedAt  time.Time   `json:"modifiedAt"`
	AccessedAt  time.Time   `json:"accessedAt"`
	AccessCount int64       `json:"accessCount"`
	IsDir       bool        `json:"isDir"`
	Owner       string      `json:"owner"`
	Tags        []string    `json:"tags,omitempty"`
	CurrentTier StorageTier `json:"currentTier"`
	Checksum    string      `json:"checksum,omitempty"`
}

// PolicyStats 策略统计.
type PolicyStats struct {
	TotalEvaluations int64                       `json:"totalEvaluations"`
	TotalMatches     int64                       `json:"totalMatches"`
	ByPolicy         map[string]*PolicyExecStats `json:"byPolicy"`
	LastEvaluation   time.Time                   `json:"lastEvaluation"`
}

// PolicyExecStats 策略执行统计.
type PolicyExecStats struct {
	PolicyID     string        `json:"policyId"`
	PolicyName   string        `json:"policyName"`
	Executions   int64         `json:"executions"`
	Matches      int64         `json:"matches"`
	Successes    int64         `json:"successes"`
	Failures     int64         `json:"failures"`
	TotalFiles   int64         `json:"totalFiles"`
	TotalBytes   int64         `json:"totalBytes"`
	LastExecTime time.Time     `json:"lastExecTime"`
	AvgExecTime  time.Duration `json:"avgExecTime"`
}

// PolicyMatch 策略匹配结果.
type PolicyMatch struct {
	PolicyID   string        `json:"policyId"`
	PolicyName string        `json:"policyName"`
	FilePath   string        `json:"filePath"`
	MatchedAt  time.Time     `json:"matchedAt"`
	Conditions []string      `json:"conditions"` // 匹配的条件
	Action     ArchiveAction `json:"action"`
	TargetTier StorageTier   `json:"targetTier"`
	Confidence float64       `json:"confidence"`
}

// NewPolicyEngine 创建策略引擎.
func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{
		policies:  make(map[string]*ArchivePolicy),
		rules:     make(map[string]*RetentionRule),
		fileCache: make(map[string]*FileMetadata),
		stats: &PolicyStats{
			ByPolicy: make(map[string]*PolicyExecStats),
		},
	}
}

// AddPolicy 添加归档策略.
func (pe *PolicyEngine) AddPolicy(policy *ArchivePolicy) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if policy.ID == "" {
		return fmt.Errorf("策略 ID 不能为空")
	}

	if _, exists := pe.policies[policy.ID]; exists {
		return fmt.Errorf("策略 %s 已存在", policy.ID)
	}

	// 验证策略
	if err := pe.validatePolicy(policy); err != nil {
		return fmt.Errorf("策略验证失败: %w", err)
	}

	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now

	pe.policies[policy.ID] = policy
	pe.stats.ByPolicy[policy.ID] = &PolicyExecStats{
		PolicyID:   policy.ID,
		PolicyName: policy.Name,
	}

	log.Printf("[PolicyEngine] 添加策略: %s (%s)", policy.Name, policy.ID)
	return nil
}

// UpdatePolicy 更新归档策略.
func (pe *PolicyEngine) UpdatePolicy(policy *ArchivePolicy) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if _, exists := pe.policies[policy.ID]; !exists {
		return fmt.Errorf("策略 %s 不存在", policy.ID)
	}

	if err := pe.validatePolicy(policy); err != nil {
		return fmt.Errorf("策略验证失败: %w", err)
	}

	policy.UpdatedAt = time.Now()
	pe.policies[policy.ID] = policy

	return nil
}

// RemovePolicy 删除归档策略.
func (pe *PolicyEngine) RemovePolicy(policyID string) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if _, exists := pe.policies[policyID]; !exists {
		return fmt.Errorf("策略 %s 不存在", policyID)
	}

	delete(pe.policies, policyID)
	delete(pe.stats.ByPolicy, policyID)

	return nil
}

// GetPolicy 获取归档策略.
func (pe *PolicyEngine) GetPolicy(policyID string) (*ArchivePolicy, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	policy, exists := pe.policies[policyID]
	if !exists {
		return nil, fmt.Errorf("策略 %s 不存在", policyID)
	}

	return policy, nil
}

// ListPolicies 列出所有归档策略.
func (pe *PolicyEngine) ListPolicies() []*ArchivePolicy {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	policies := make([]*ArchivePolicy, 0, len(pe.policies))
	for _, p := range pe.policies {
		policies = append(policies, p)
	}

	return policies
}

// AddRetentionRule 添加保留规则.
func (pe *PolicyEngine) AddRetentionRule(rule *RetentionRule) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("规则 ID 不能为空")
	}

	if _, exists := pe.rules[rule.ID]; exists {
		return fmt.Errorf("规则 %s 已存在", rule.ID)
	}

	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	pe.rules[rule.ID] = rule
	log.Printf("[PolicyEngine] 添加保留规则: %s (%s)", rule.Name, rule.ID)
	return nil
}

// EvaluateFile 评估文件是否匹配任何策略.
func (pe *PolicyEngine) EvaluateFile(metadata *FileMetadata) []PolicyMatch {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	matches := make([]PolicyMatch, 0)
	pe.stats.TotalEvaluations++

	for _, policy := range pe.policies {
		if !policy.Enabled {
			continue
		}

		match, ok := pe.evaluatePolicy(policy, metadata)
		if ok {
			matches = append(matches, match)
			pe.stats.TotalMatches++

			// 更新策略统计
			if stats, exists := pe.stats.ByPolicy[policy.ID]; exists {
				stats.Matches++
			}
		}
	}

	pe.stats.LastEvaluation = time.Now()
	return matches
}

// evaluatePolicy 评估单个策略.
func (pe *PolicyEngine) evaluatePolicy(policy *ArchivePolicy, metadata *FileMetadata) (PolicyMatch, bool) {
	match := PolicyMatch{
		PolicyID:   policy.ID,
		PolicyName: policy.Name,
		FilePath:   metadata.Path,
		MatchedAt:  time.Now(),
		Conditions: make([]string, 0),
		Action:     policy.Action,
		TargetTier: policy.TargetTier,
	}

	conditions := &policy.Conditions
	matched := true

	// 检查访问频率条件
	if conditions.MinAccessCount > 0 && metadata.AccessCount < conditions.MinAccessCount {
		matched = false
	}
	if conditions.MaxAccessCount > 0 && metadata.AccessCount > conditions.MaxAccessCount {
		matched = false
	}
	if matched && (conditions.MinAccessCount > 0 || conditions.MaxAccessCount > 0) {
		match.Conditions = append(match.Conditions, "access_count")
	}

	// 检查闲置天数
	if conditions.AccessIdleDays > 0 {
		idleDays := int(time.Since(metadata.AccessedAt).Hours() / 24)
		if idleDays < conditions.AccessIdleDays {
			matched = false
		} else {
			match.Conditions = append(match.Conditions, "idle_days")
		}
	}

	// 检查文件年龄
	if conditions.FileAgeDays > 0 {
		ageDays := int(time.Since(metadata.CreatedAt).Hours() / 24)
		if ageDays < conditions.FileAgeDays {
			matched = false
		} else {
			match.Conditions = append(match.Conditions, "file_age")
		}
	}

	// 检查创建时间
	if !conditions.CreatedBefore.IsZero() && metadata.CreatedAt.After(conditions.CreatedBefore) {
		matched = false
	} else if !conditions.CreatedBefore.IsZero() {
		match.Conditions = append(match.Conditions, "created_before")
	}

	// 检查修改时间
	if !conditions.ModifiedBefore.IsZero() && metadata.ModifiedAt.After(conditions.ModifiedBefore) {
		matched = false
	} else if !conditions.ModifiedBefore.IsZero() {
		match.Conditions = append(match.Conditions, "modified_before")
	}

	// 检查最后访问时间
	if !conditions.LastAccessBefore.IsZero() && metadata.AccessedAt.After(conditions.LastAccessBefore) {
		matched = false
	} else if !conditions.LastAccessBefore.IsZero() {
		match.Conditions = append(match.Conditions, "last_access_before")
	}

	// 检查文件大小
	if conditions.MinFileSize > 0 && metadata.Size < conditions.MinFileSize {
		matched = false
	}
	if conditions.MaxFileSize > 0 && metadata.Size > conditions.MaxFileSize {
		matched = false
	}
	if matched && (conditions.MinFileSize > 0 || conditions.MaxFileSize > 0) {
		match.Conditions = append(match.Conditions, "file_size")
	}

	// 检查文件扩展名
	if len(conditions.FileExtensions) > 0 {
		ext := strings.ToLower(filepath.Ext(metadata.Path))
		found := false
		for _, allowedExt := range conditions.FileExtensions {
			if ext == allowedExt || ext == "."+allowedExt {
				found = true
				break
			}
		}
		if !found {
			matched = false
		} else {
			match.Conditions = append(match.Conditions, "file_extension")
		}
	}

	// 检查 MIME 类型
	if len(conditions.MimeTypes) > 0 {
		found := false
		for _, allowedMime := range conditions.MimeTypes {
			if metadata.MimeType == allowedMime {
				found = true
				break
			}
		}
		if !found {
			matched = false
		} else {
			match.Conditions = append(match.Conditions, "mime_type")
		}
	}

	// 检查路径模式
	if len(conditions.PathPatterns) > 0 {
		matched_pattern := false
		for _, pattern := range conditions.PathPatterns {
			if matchPathPattern(metadata.Path, pattern) {
				matched_pattern = true
				break
			}
		}
		if !matched_pattern {
			matched = false
		} else {
			match.Conditions = append(match.Conditions, "path_pattern")
		}
	}

	// 检查排除模式
	if len(conditions.ExcludePatterns) > 0 {
		for _, pattern := range conditions.ExcludePatterns {
			if matchPathPattern(metadata.Path, pattern) {
				matched = false
				break
			}
		}
		if matched {
			match.Conditions = append(match.Conditions, "exclude_pattern")
		}
	}

	// 检查源存储层
	if len(conditions.SourceTiers) > 0 {
		found := false
		for _, tier := range conditions.SourceTiers {
			if metadata.CurrentTier == tier {
				found = true
				break
			}
		}
		if !found {
			matched = false
		} else {
			match.Conditions = append(match.Conditions, "source_tier")
		}
	}

	// 检查标签条件
	if len(conditions.RequiredTags) > 0 {
		if !hasAllTags(metadata.Tags, conditions.RequiredTags) {
			matched = false
		} else {
			match.Conditions = append(match.Conditions, "required_tags")
		}
	}

	if len(conditions.ExcludeTags) > 0 {
		if hasAnyTag(metadata.Tags, conditions.ExcludeTags) {
			matched = false
		}
	}

	// 计算置信度
	if matched {
		match.Confidence = calculateMatchConfidence(match.Conditions)
	}

	return match, matched
}

// validatePolicy 验证策略.
func (pe *PolicyEngine) validatePolicy(policy *ArchivePolicy) error {
	if policy.Name == "" {
		return fmt.Errorf("策略名称不能为空")
	}

	// 验证目标层级
	switch policy.TargetTier {
	case TierHot, TierWarm, TierCold, TierIce:
		// 有效
	default:
		return fmt.Errorf("无效的目标层级: %s", policy.TargetTier)
	}

	// 验证动作
	switch policy.Action {
	case ArchiveActionMove, ArchiveActionCompress, ArchiveActionDeduplicate,
		ArchiveActionDelete, ArchiveActionSnapshot:
		// 有效
	default:
		return fmt.Errorf("无效的归档动作: %s", policy.Action)
	}

	// 验证压缩算法
	if policy.Compression != "" {
		switch policy.Compression {
		case CompressionNone, CompressionGzip, CompressionZstd,
			CompressionLZ4, CompressionBrotli, CompressionXZ:
			// 有效
		default:
			return fmt.Errorf("无效的压缩算法: %s", policy.Compression)
		}
	}

	// 验证调度表达式（简化验证）
	if policy.Schedule != "" {
		// 实际应该验证 cron 表达式
		if len(policy.Schedule) < 6 {
			return fmt.Errorf("无效的调度表达式: %s", policy.Schedule)
		}
	}

	return nil
}

// matchPathPattern 匹配路径模式.
func matchPathPattern(path, pattern string) bool {
	// 简化的模式匹配
	// 支持 * 通配符
	if pattern == "*" {
		return true
	}

	// 前缀匹配
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(path, prefix)
	}

	// 后缀匹配
	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(path, suffix)
	}

	// 包含匹配
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		substr := strings.Trim(pattern, "*")
		return strings.Contains(path, substr)
	}

	// 精确匹配
	return path == pattern
}

// hasAllTags 检查是否包含所有必需标签.
func hasAllTags(tags, required []string) bool {
	tagSet := make(map[string]bool)
	for _, tag := range tags {
		tagSet[tag] = true
	}

	for _, req := range required {
		if !tagSet[req] {
			return false
		}
	}

	return true
}

// hasAnyTag 检查是否包含任何排除标签.
func hasAnyTag(tags, exclude []string) bool {
	tagSet := make(map[string]bool)
	for _, tag := range tags {
		tagSet[tag] = true
	}

	for _, exc := range exclude {
		if tagSet[exc] {
			return true
		}
	}

	return false
}

// calculateMatchConfidence 计算匹配置信度.
func calculateMatchConfidence(conditions []string) float64 {
	if len(conditions) == 0 {
		return 0.5
	}

	// 匹配的条件越多，置信度越高
	baseConfidence := 0.6
	conditionBonus := float64(len(conditions)) * 0.1

	confidence := baseConfidence + conditionBonus
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// GetStats 获取策略统计.
func (pe *PolicyEngine) GetStats() *PolicyStats {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	return pe.stats
}

// UpdateFileCache 更新文件缓存.
func (pe *PolicyEngine) UpdateFileCache(metadata *FileMetadata) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	pe.fileCache[metadata.Path] = metadata
}

// GetFileMetadata 获取文件元数据.
func (pe *PolicyEngine) GetFileMetadata(path string) (*FileMetadata, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	metadata, exists := pe.fileCache[path]
	if !exists {
		return nil, fmt.Errorf("文件 %s 的元数据不存在", path)
	}

	return metadata, nil
}

// ScanAndEvaluate 扫描并评估文件.
func (pe *PolicyEngine) ScanAndEvaluate(paths []string) []PolicyMatch {
	allMatches := make([]PolicyMatch, 0)

	for _, path := range paths {
		metadata := pe.getFileMetadata(path)
		if metadata == nil {
			continue
		}

		matches := pe.EvaluateFile(metadata)
		allMatches = append(allMatches, matches...)
	}

	return allMatches
}

// getFileMetadata 获取文件元数据（从缓存或文件系统）.
func (pe *PolicyEngine) getFileMetadata(path string) *FileMetadata {
	pe.mu.RLock()
	metadata, exists := pe.fileCache[path]
	pe.mu.RUnlock()

	if exists {
		return metadata
	}

	// 实际实现应该从文件系统读取
	// 这里返回一个模拟的元数据
	return &FileMetadata{
		Path:        path,
		Size:        0,
		Extension:   filepath.Ext(path),
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
		AccessedAt:  time.Now(),
		AccessCount: 0,
		CurrentTier: TierHot,
		Tags:        make([]string, 0),
	}
}

// GetMatchingFiles 获取匹配指定策略的文件.
func (pe *PolicyEngine) GetMatchingFiles(policyID string, paths []string) ([]*FileMetadata, error) {
	pe.mu.RLock()
	policy, exists := pe.policies[policyID]
	pe.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("策略 %s 不存在", policyID)
	}

	matchingFiles := make([]*FileMetadata, 0)

	for _, path := range paths {
		metadata := pe.getFileMetadata(path)
		if metadata == nil {
			continue
		}

		_, matched := pe.evaluatePolicy(policy, metadata)
		if matched {
			matchingFiles = append(matchingFiles, metadata)
		}
	}

	return matchingFiles, nil
}

// ExportPolicies 导出策略配置.
func (pe *PolicyEngine) ExportPolicies() map[string]*ArchivePolicy {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	result := make(map[string]*ArchivePolicy, len(pe.policies))
	for id, policy := range pe.policies {
		copy := *policy
		result[id] = &copy
	}

	return result
}

// ImportPolicies 导入策略配置.
func (pe *PolicyEngine) ImportPolicies(policies map[string]*ArchivePolicy) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	for _, policy := range policies {
		if err := pe.validatePolicy(policy); err != nil {
			return fmt.Errorf("导入策略 %s 失败: %w", policy.ID, err)
		}

		now := time.Now()
		policy.CreatedAt = now
		policy.UpdatedAt = now

		pe.policies[policy.ID] = policy
		pe.stats.ByPolicy[policy.ID] = &PolicyExecStats{
			PolicyID:   policy.ID,
			PolicyName: policy.Name,
		}
	}

	log.Printf("[PolicyEngine] 导入 %d 个策略", len(policies))
	return nil
}
