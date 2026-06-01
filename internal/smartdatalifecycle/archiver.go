package smartdatalifecycle

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// Archiver 自动归档器
// 基于访问频率、时间等条件自动归档数据
type Archiver struct {
	config  ArchiveConfig
	manager *Manager
	logger  *zap.Logger
}

// NewArchiver 创建归档器
func NewArchiver(config ArchiveConfig, manager *Manager, logger *zap.Logger) *Archiver {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Archiver{
		config:  config,
		manager: manager,
		logger:  logger,
	}
}

// Run 执行归档检查
func (a *Archiver) Run(ctx context.Context) error {
	a.logger.Info("archive check started")

	policies := a.manager.ListArchivePolicies()
	if len(policies) == 0 {
		a.logger.Debug("no archive policies configured")
		return nil
	}

	totalArchived := 0
	totalBytes := int64(0)

	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		archived, bytes, err := a.runPolicy(ctx, policy)
		if err != nil {
			a.logger.Error("archive policy failed",
				zap.String("policy_id", policy.ID),
				zap.Error(err))
			continue
		}

		totalArchived += archived
		totalBytes += bytes
	}

	a.logger.Info("archive check completed",
		zap.Int("total_archived", totalArchived),
		zap.Int64("total_bytes", totalBytes))

	return nil
}

// runPolicy 执行单个归档策略
func (a *Archiver) runPolicy(ctx context.Context, policy *ArchivePolicy) (int, int64, error) {
	// 获取活跃数据项
	items := a.manager.ListItems(StageActive, 0, 0)

	archived := 0
	totalBytes := int64(0)
	batchCount := 0

	for _, item := range items {
		if ctx.Err() != nil {
			return archived, totalBytes, ctx.Err()
		}

		// 批量限制
		if a.config.BatchSize > 0 && batchCount >= a.config.BatchSize {
			break
		}

		// 法律冻结检查
		if item.LegalHold {
			continue
		}

		// 检查是否匹配策略条件
		if !a.matchesPolicy(item, policy) {
			continue
		}

		// 试运行模式
		if a.manager.config.DryRun {
			a.logger.Info("dry run: would archive",
				zap.String("item_id", item.ID),
				zap.String("path", item.Path))
			archived++
			totalBytes += item.Size
			batchCount++
			continue
		}

		// 执行归档
		if err := a.manager.UpdateItemStage(item.ID, policy.TargetStage, "policy:"+policy.ID); err != nil {
			a.logger.Error("failed to archive item",
				zap.String("item_id", item.ID),
				zap.Error(err))
			continue
		}

		archived++
		totalBytes += item.Size
		batchCount++
	}

	// 更新策略统计
	if archived > 0 {
		a.manager.mu.Lock()
		if p, ok := a.manager.archivePolicies[policy.ID]; ok {
			now := time.Now()
			p.LastRunAt = &now
			p.TotalArchived += int64(archived)
			p.TotalBytes += totalBytes
		}
		a.manager.mu.Unlock()
	}

	return archived, totalBytes, nil
}

// matchesPolicy 检查数据项是否匹配归档策略
func (a *Archiver) matchesPolicy(item *DataItem, policy *ArchivePolicy) bool {
	// 检查路径匹配
	if !matchesPattern(item.Path, policy.PathPrefixes, policy.FilePatterns) {
		return false
	}

	// 检查排除模式
	if matchesAnyPattern(item.Path, policy.ExcludePatterns) {
		return false
	}

	// 检查文件大小
	if policy.MinFileSizeBytes > 0 && item.Size < policy.MinFileSizeBytes {
		return false
	}
	if policy.MaxFileSizeBytes > 0 && item.Size > policy.MaxFileSizeBytes {
		return false
	}

	// 根据触发条件检查
	switch policy.Trigger {
	case TriggerAccessFrequency:
		if policy.MaxAccessCount > 0 && item.AccessCount > int64(policy.MaxAccessCount) {
			return false
		}
		// 还需要检查时间
		if policy.DaysSinceAccess > 0 {
			daysSinceAccess := time.Since(item.AccessedAt).Hours() / 24
			if daysSinceAccess < float64(policy.DaysSinceAccess) {
				return false
			}
		}

	case TriggerLastAccessTime:
		if policy.DaysSinceAccess > 0 {
			daysSinceAccess := time.Since(item.AccessedAt).Hours() / 24
			if daysSinceAccess < float64(policy.DaysSinceAccess) {
				return false
			}
		}

	case TriggerAge:
		if policy.FileAgeDays > 0 {
			fileAge := time.Since(item.CreatedAt).Hours() / 24
			if fileAge < float64(policy.FileAgeDays) {
				return false
			}
		}

	case TriggerSize:
		// 仅基于大小，上面已经检查过了

	case TriggerCombined:
		// 组合条件：需要满足所有配置的条件
		if policy.DaysSinceAccess > 0 {
			daysSinceAccess := time.Since(item.AccessedAt).Hours() / 24
			if daysSinceAccess < float64(policy.DaysSinceAccess) {
				return false
			}
		}
		if policy.FileAgeDays > 0 {
			fileAge := time.Since(item.CreatedAt).Hours() / 24
			if fileAge < float64(policy.FileAgeDays) {
				return false
			}
		}
		if policy.MaxAccessCount > 0 && item.AccessCount > int64(policy.MaxAccessCount) {
			return false
		}
	}

	return true
}

// EvaluateItem 评估单个数据项的归档建议
func (a *Archiver) EvaluateItem(item *DataItem) (shouldArchive bool, reason string, targetStage LifecycleStage) {
	// 默认空闲天数检查
	if a.config.MinIdleDays > 0 {
		idleDays := time.Since(item.AccessedAt).Hours() / 24
		if idleDays < float64(a.config.MinIdleDays) {
			return false, "", ""
		}
	}

	// 检查策略
	policies := a.manager.ListArchivePolicies()
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		if a.matchesPolicy(item, policy) {
			reason = fmt.Sprintf("matches policy '%s' (%s)", policy.Name, policy.Trigger)
			return true, reason, policy.TargetStage
		}
	}

	// 默认规则：基于空闲时间
	idleDays := time.Since(item.AccessedAt).Hours() / 24
	if idleDays > 90 {
		return true, "idle for more than 90 days", StageArchive
	}
	if idleDays > 30 {
		return true, "idle for more than 30 days", StageCold
	}

	return false, "", ""
}

// GetArchiveCandidates 获取归档候选列表
func (a *Archiver) GetArchiveCandidates(limit int) []*DataItem {
	items := a.manager.ListItems(StageActive, 0, 0)

	candidates := make([]*DataItem, 0)
	for _, item := range items {
		if item.LegalHold {
			continue
		}

		shouldArchive, _, _ := a.EvaluateItem(item)
		if shouldArchive {
			candidates = append(candidates, item)
			if limit > 0 && len(candidates) >= limit {
				break
			}
		}
	}

	return candidates
}

// matchesPattern 检查路径是否匹配模式
func matchesPattern(path string, prefixes, patterns []string) bool {
	// 如果没有指定前缀和模式，则匹配所有
	if len(prefixes) == 0 && len(patterns) == 0 {
		return true
	}

	// 检查前缀
	for _, prefix := range prefixes {
		if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			return true
		}
	}

	// 简单的模式匹配（支持 * 通配符）
	for _, pattern := range patterns {
		if simpleMatch(path, pattern) {
			return true
		}
	}

	return false
}

// matchesAnyPattern 检查路径是否匹配任一模式
func matchesAnyPattern(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if simpleMatch(path, pattern) {
			return true
		}
	}
	return false
}

// simpleMatch 简单模式匹配
func simpleMatch(path, pattern string) bool {
	if pattern == "*" {
		return true
	}
	// 前缀匹配
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(path) >= len(prefix) && path[:len(prefix)] == prefix
	}
	// 后缀匹配
	if len(pattern) > 0 && pattern[0] == '*' {
		suffix := pattern[1:]
		return len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix
	}
	// 精确匹配
	return path == pattern
}
