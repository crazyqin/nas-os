package smartdatalifecycle

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// Cleaner 智能清理器
// 清理过期数据、临时文件、重复数据等
type Cleaner struct {
	config  CleanupConfig
	manager *Manager
	logger  *zap.Logger
}

// NewCleaner 创建清理器
func NewCleaner(config CleanupConfig, manager *Manager, logger *zap.Logger) *Cleaner {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Cleaner{
		config:  config,
		manager: manager,
		logger:  logger,
	}
}

// Run 执行清理检查
func (c *Cleaner) Run(ctx context.Context) error {
	c.logger.Info("cleanup check started")

	rules := c.manager.ListCleanupRules()
	if len(rules) == 0 {
		c.logger.Debug("no cleanup rules configured")
		return nil
	}

	totalCleaned := 0
	totalFreed := int64(0)

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		result, err := c.runRule(ctx, rule)
		if err != nil {
			c.logger.Error("cleanup rule failed",
				zap.String("rule_id", rule.ID),
				zap.Error(err))
			continue
		}

		totalCleaned += result.ItemsDeleted
		totalFreed += result.SpaceFreed
	}

	c.logger.Info("cleanup check completed",
		zap.Int("total_cleaned", totalCleaned),
		zap.Int64("total_freed", totalFreed))

	return nil
}

// runRule 执行单个清理规则
func (c *Cleaner) runRule(ctx context.Context, rule *CleanupRule) (*CleanupResult, error) {
	startTime := time.Now()

	result := &CleanupResult{
		RuleID:    rule.ID,
		StartedAt: startTime,
		Errors:    make([]string, 0),
	}

	switch rule.RuleType {
	case RuleTypeExpired:
		cleaned, freed, errors := c.cleanExpired(ctx, rule)
		result.ItemsDeleted = cleaned
		result.SpaceFreed = freed
		result.Errors = append(result.Errors, errors...)

	case RuleTypeTempFiles:
		cleaned, freed, errors := c.cleanTempFiles(ctx, rule)
		result.ItemsDeleted = cleaned
		result.SpaceFreed = freed
		result.Errors = append(result.Errors, errors...)

	case RuleTypeDuplicates:
		// 重复数据清理由 Deduplicator 处理
		c.logger.Debug("duplicate cleanup handled by deduplicator")

	case RuleTypeEmptyDirs:
		// 空目录清理
		c.logger.Debug("empty directory cleanup not yet implemented")

	case RuleTypeOldLogs:
		cleaned, freed, errors := c.cleanOldLogs(ctx, rule)
		result.ItemsDeleted = cleaned
		result.SpaceFreed = freed
		result.Errors = append(result.Errors, errors...)

	case RuleTypeTrash:
		cleaned, freed, errors := c.cleanTrash(ctx, rule)
		result.ItemsDeleted = cleaned
		result.SpaceFreed = freed
		result.Errors = append(result.Errors, errors...)

	default:
		return nil, fmt.Errorf("unknown cleanup rule type: %s", rule.RuleType)
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)

	// 更新规则统计
	c.manager.mu.Lock()
	if r, ok := c.manager.cleanupRules[rule.ID]; ok {
		now := time.Now()
		r.LastRunAt = &now
		r.TotalCleaned += int64(result.ItemsDeleted)
		r.TotalFreed += result.SpaceFreed
	}
	c.manager.mu.Unlock()

	return result, nil
}

// cleanExpired 清理过期数据
func (c *Cleaner) cleanExpired(ctx context.Context, rule *CleanupRule) (int, int64, []string) {
	cleaned := 0
	freed := int64(0)
	errors := make([]string, 0)

	// 获取已过期的数据项
	c.manager.mu.RLock()
	expiredItems := make([]*DataItem, 0)
	for _, item := range c.manager.dataItems {
		if ctx.Err() != nil {
			c.manager.mu.RUnlock()
			return cleaned, freed, errors
		}

		if item.ExpiresAt != nil && time.Now().After(*item.ExpiresAt) {
			if !item.LegalHold {
				expiredItems = append(expiredItems, item)
			}
		}
	}
	c.manager.mu.RUnlock()

	// 执行清理
	for _, item := range expiredItems {
		if ctx.Err() != nil {
			break
		}

		if c.manager.config.DryRun {
			c.logger.Info("dry run: would delete expired item",
				zap.String("item_id", item.ID),
				zap.String("path", item.Path))
			cleaned++
			freed += item.Size
			continue
		}

		if err := c.manager.UpdateItemStage(item.ID, StageDeleted, "cleanup:"+rule.ID); err != nil {
			errors = append(errors, fmt.Sprintf("failed to delete item %s: %v", item.ID, err))
			continue
		}

		cleaned++
		freed += item.Size
	}

	return cleaned, freed, errors
}

// cleanTempFiles 清理临时文件
func (c *Cleaner) cleanTempFiles(ctx context.Context, rule *CleanupRule) (int, int64, []string) {
	cleaned := 0
	freed := int64(0)
	errors := make([]string, 0)

	// 获取临时文件模式
	patterns := rule.TempFilePatterns
	if len(patterns) == 0 {
		patterns = []string{"*.tmp", "*.temp", "~*", "*.swp", "*.bak"}
	}

	// 获取活跃数据项
	items := c.manager.ListItems(StageActive, 0, 0)

	for _, item := range items {
		if ctx.Err() != nil {
			break
		}

		// 检查是否匹配临时文件模式
		if !matchesAnyPattern(item.Path, patterns) {
			continue
		}

		// 检查年龄
		if rule.MaxFileAgeDays > 0 {
			fileAge := time.Since(item.ModifiedAt).Hours() / 24
			if fileAge < float64(rule.MaxFileAgeDays) {
				continue
			}
		}

		// 跳过法律冻结
		if item.LegalHold {
			continue
		}

		if c.manager.config.DryRun {
			c.logger.Info("dry run: would delete temp file",
				zap.String("item_id", item.ID),
				zap.String("path", item.Path))
			cleaned++
			freed += item.Size
			continue
		}

		if err := c.manager.UpdateItemStage(item.ID, StageDeleted, "cleanup:"+rule.ID); err != nil {
			errors = append(errors, fmt.Sprintf("failed to delete temp file %s: %v", item.ID, err))
			continue
		}

		cleaned++
		freed += item.Size
	}

	return cleaned, freed, errors
}

// cleanOldLogs 清理旧日志
func (c *Cleaner) cleanOldLogs(ctx context.Context, rule *CleanupRule) (int, int64, []string) {
	cleaned := 0
	freed := int64(0)
	errors := make([]string, 0)

	retentionDays := rule.LogRetentionDays
	if retentionDays <= 0 {
		retentionDays = 30 // 默认30天
	}

	// 获取日志文件
	logPatterns := []string{"*.log", "*.log.*", "*.gz"}
	items := c.manager.ListItems(StageActive, 0, 0)

	for _, item := range items {
		if ctx.Err() != nil {
			break
		}

		if !matchesAnyPattern(item.Path, logPatterns) {
			continue
		}

		// 检查年龄
		fileAge := time.Since(item.ModifiedAt).Hours() / 24
		if fileAge < float64(retentionDays) {
			continue
		}

		if c.manager.config.DryRun {
			c.logger.Info("dry run: would delete old log",
				zap.String("item_id", item.ID),
				zap.String("path", item.Path))
			cleaned++
			freed += item.Size
			continue
		}

		if err := c.manager.UpdateItemStage(item.ID, StageDeleted, "cleanup:"+rule.ID); err != nil {
			errors = append(errors, fmt.Sprintf("failed to delete old log %s: %v", item.ID, err))
			continue
		}

		cleaned++
		freed += item.Size
	}

	return cleaned, freed, errors
}

// cleanTrash 清理回收站
func (c *Cleaner) cleanTrash(ctx context.Context, rule *CleanupRule) (int, int64, []string) {
	cleaned := 0
	freed := int64(0)
	errors := make([]string, 0)

	retentionDays := c.config.TrashRetentionDays
	if retentionDays <= 0 {
		retentionDays = 30
	}

	// 获取已删除的数据项
	deletedItems := c.manager.ListItems(StageDeleted, 0, 0)

	for _, item := range deletedItems {
		if ctx.Err() != nil {
			break
		}

		if item.DeletedAt == nil {
			continue
		}

		// 检查是否超过保留期
		daysSinceDelete := time.Since(*item.DeletedAt).Hours() / 24
		if daysSinceDelete < float64(retentionDays) {
			continue
		}

		// 法律冻结检查
		if item.LegalHold {
			continue
		}

		if c.manager.config.DryRun {
			c.logger.Info("dry run: would permanently delete trash item",
				zap.String("item_id", item.ID),
				zap.String("path", item.Path))
			cleaned++
			freed += item.Size
			continue
		}

		// 从存储中删除（这里只是标记，实际删除需要外部存储支持）
		c.manager.mu.Lock()
		delete(c.manager.dataItems, item.ID)
		c.manager.mu.Unlock()

		cleaned++
		freed += item.Size
	}

	return cleaned, freed, errors
}

// ExecuteCleanup 手动执行清理
func (c *Cleaner) ExecuteCleanup(ctx context.Context, ruleID string) (*CleanupResult, error) {
	rule, ok := c.manager.GetCleanupRule(ruleID)
	if !ok {
		return nil, fmt.Errorf("cleanup rule not found: %s", ruleID)
	}

	return c.runRule(ctx, rule)
}

// GetCleanupPreview 获取清理预览
func (c *Cleaner) GetCleanupPreview(ctx context.Context, ruleID string) (*DryRunResult, error) {
	rule, ok := c.manager.GetCleanupRule(ruleID)
	if !ok {
		return nil, fmt.Errorf("cleanup rule not found: %s", ruleID)
	}

	// 临时启用试运行
	originalDryRun := c.manager.config.DryRun
	c.manager.config.DryRun = true
	defer func() {
		c.manager.config.DryRun = originalDryRun
	}()

	result, err := c.runRule(ctx, rule)
	if err != nil {
		return nil, err
	}

	return &DryRunResult{
		ItemsAffected: result.ItemsDeleted,
		TotalSize:     result.SpaceFreed,
		Actions:       []string{fmt.Sprintf("Would delete %d items", result.ItemsDeleted)},
	}, nil
}
