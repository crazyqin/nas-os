package smartdatalifecycle

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// RetentionManager 保留策略管理器
// 管理数据保留策略、合规要求、法律冻结
type RetentionManager struct {
	config  RetentionConfig
	manager *Manager
	logger  *zap.Logger
}

// NewRetentionManager 创建保留策略管理器
func NewRetentionManager(config RetentionConfig, manager *Manager, logger *zap.Logger) *RetentionManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RetentionManager{
		config:  config,
		manager: manager,
		logger:  logger,
	}
}

// Run 执行保留策略检查
func (rm *RetentionManager) Run(ctx context.Context) error {
	rm.logger.Info("retention check started")

	// 检查过期数据
	expiredCount, err := rm.checkExpirations(ctx)
	if err != nil {
		rm.logger.Error("expiration check failed", zap.Error(err))
	}

	// 检查宽限期
	graceCount, err := rm.checkGracePeriod(ctx)
	if err != nil {
		rm.logger.Error("grace period check failed", zap.Error(err))
	}

	rm.logger.Info("retention check completed",
		zap.Int("expired", expiredCount),
		zap.Int("in_grace_period", graceCount))

	return nil
}

// checkExpirations 检查数据过期
func (rm *RetentionManager) checkExpirations(ctx context.Context) (int, error) {
	expiredCount := 0

	rm.manager.mu.RLock()
	items := make([]*DataItem, 0)
	for _, item := range rm.manager.dataItems {
		items = append(items, item)
	}
	rm.manager.mu.RUnlock()

	for _, item := range items {
		if ctx.Err() != nil {
			return expiredCount, ctx.Err()
		}

		if item.ExpiresAt == nil {
			continue
		}

		if time.Now().Before(*item.ExpiresAt) {
			continue
		}

		// 已过期，检查保留策略
		policy, _ := rm.manager.GetRetentionPolicy(item.RetentionPolicyID)
		if policy == nil {
			// 使用默认动作
			if rm.config.DefaultRetentionDays > 0 {
				rm.handleExpiredItem(item, ActionArchive)
			}
			continue
		}

		// 法律冻结检查
		if item.LegalHold {
			rm.logger.Info("expired item under legal hold, skipping",
				zap.String("item_id", item.ID),
				zap.String("path", item.Path))
			continue
		}

		// 合规策略检查
		if policy.CompliancePolicy {
			// 合规策略下，过期后必须执行到期动作
			rm.handleExpiredItem(item, policy.ExpirationAction)
		} else {
			rm.handleExpiredItem(item, policy.ExpirationAction)
		}

		expiredCount++
	}

	return expiredCount, nil
}

// handleExpiredItem 处理过期数据项
func (rm *RetentionManager) handleExpiredItem(item *DataItem, action RetentionAction) {
	switch action {
	case ActionArchive:
		if err := rm.manager.UpdateItemStage(item.ID, StageArchive, "retention"); err != nil {
			rm.logger.Error("failed to archive expired item",
				zap.String("item_id", item.ID),
				zap.Error(err))
		} else {
			rm.logger.Info("expired item archived",
				zap.String("item_id", item.ID),
				zap.String("path", item.Path))
		}

	case ActionDelete:
		if err := rm.manager.UpdateItemStage(item.ID, StageExpired, "retention"); err != nil {
			rm.logger.Error("failed to delete expired item",
				zap.String("item_id", item.ID),
				zap.Error(err))
		} else {
			rm.logger.Info("expired item deleted",
				zap.String("item_id", item.ID),
				zap.String("path", item.Path))
		}

	case ActionMigrate:
		// 迁移到归档层
		if err := rm.manager.UpdateItemStage(item.ID, StageArchive, "retention"); err != nil {
			rm.logger.Error("failed to migrate expired item",
				zap.String("item_id", item.ID),
				zap.Error(err))
		}

	case ActionNotify:
		// 仅通知，不执行动作
		rm.logger.Info("expired item notification",
			zap.String("item_id", item.ID),
			zap.String("path", item.Path))

	case ActionFreeze:
		// 冻结（设置法律冻结）
		rm.manager.mu.Lock()
		if i, ok := rm.manager.dataItems[item.ID]; ok {
			i.LegalHold = true
		}
		rm.manager.mu.Unlock()
		rm.logger.Info("expired item frozen",
			zap.String("item_id", item.ID),
			zap.String("path", item.Path))
	}
}

// checkGracePeriod 检查宽限期
func (rm *RetentionManager) checkGracePeriod(ctx context.Context) (int, error) {
	graceCount := 0

	if rm.config.GracePeriodDays <= 0 {
		return 0, nil
	}

	rm.manager.mu.RLock()
	items := make([]*DataItem, 0)
	for _, item := range rm.manager.dataItems {
		items = append(items, item)
	}
	rm.manager.mu.RUnlock()

	for _, item := range items {
		if ctx.Err() != nil {
			return graceCount, ctx.Err()
		}

		if item.ExpiresAt == nil {
			continue
		}

		// 检查是否在宽限期内
		graceEnd := item.ExpiresAt.AddDate(0, 0, rm.config.GracePeriodDays)
		if time.Now().After(*item.ExpiresAt) && time.Now().Before(graceEnd) {
			graceCount++
			rm.logger.Debug("item in grace period",
				zap.String("item_id", item.ID),
				zap.Time("expires_at", *item.ExpiresAt),
				zap.Time("grace_end", graceEnd))
		}
	}

	return graceCount, nil
}

// SetRetentionPolicy 设置数据项的保留策略
func (rm *RetentionManager) SetRetentionPolicy(itemID string, policyID string) error {
	rm.manager.mu.Lock()
	defer rm.manager.mu.Unlock()

	item, ok := rm.manager.dataItems[itemID]
	if !ok {
		return fmt.Errorf("item not found: %s", itemID)
	}

	policy, ok := rm.manager.policies[policyID]
	if !ok {
		return fmt.Errorf("policy not found: %s", policyID)
	}

	item.RetentionPolicyID = policyID

	// 计算过期时间
	if policy.RetentionDays > 0 {
		expiry := item.CreatedAt.AddDate(0, 0, policy.RetentionDays)
		item.ExpiresAt = &expiry
	}

	// 记录事件
	event := &LifecycleEvent{
		ID:          fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		EventType:   EventRetentionSet,
		ItemID:      itemID,
		ItemPath:    item.Path,
		Details:     fmt.Sprintf("retention policy set: %s", policyID),
		TriggeredBy: "manual",
		CreatedAt:   time.Now(),
	}
	rm.manager.events = append(rm.manager.events, event)

	rm.logger.Info("retention policy set",
		zap.String("item_id", itemID),
		zap.String("policy_id", policyID))

	return nil
}

// SetLegalHold 设置法律冻结
func (rm *RetentionManager) SetLegalHold(itemID string, hold bool) error {
	rm.manager.mu.Lock()
	defer rm.manager.mu.Unlock()

	item, ok := rm.manager.dataItems[itemID]
	if !ok {
		return fmt.Errorf("item not found: %s", itemID)
	}

	item.LegalHold = hold

	// 记录事件
	event := &LifecycleEvent{
		ID:          fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		EventType:   EventLegalHold,
		ItemID:      itemID,
		ItemPath:    item.Path,
		Details:     fmt.Sprintf("legal hold: %v", hold),
		TriggeredBy: "manual",
		CreatedAt:   time.Now(),
	}
	rm.manager.events = append(rm.manager.events, event)

	rm.logger.Info("legal hold updated",
		zap.String("item_id", itemID),
		zap.Bool("hold", hold))

	return nil
}

// GetExpiringItems 获取即将过期的数据项
func (rm *RetentionManager) GetExpiringItems(days int) []*DataItem {
	rm.manager.mu.RLock()
	defer rm.manager.mu.RUnlock()

	result := make([]*DataItem, 0)
	cutoff := time.Now().AddDate(0, 0, days)

	for _, item := range rm.manager.dataItems {
		if item.ExpiresAt != nil && item.ExpiresAt.Before(cutoff) {
			result = append(result, item)
		}
	}

	return result
}

// GetRetainedItems 获取受保留策略保护的数据项
func (rm *RetentionManager) GetRetainedItems(policyID string) []*DataItem {
	rm.manager.mu.RLock()
	defer rm.manager.mu.RUnlock()

	result := make([]*DataItem, 0)
	for _, item := range rm.manager.dataItems {
		if item.RetentionPolicyID == policyID {
			result = append(result, item)
		}
	}

	return result
}

// ValidateRetentionPolicy 验证保留策略
func (rm *RetentionManager) ValidateRetentionPolicy(policy *RetentionPolicy) []string {
	var warnings []string

	if policy.RetentionDays < 0 {
		warnings = append(warnings, "retention_days cannot be negative")
	}

	if policy.RetentionDays > 3650 { // 10年
		warnings = append(warnings, "retention_days is very long (>10 years)")
	}

	if policy.CompliancePolicy && policy.RetentionDays <= 0 {
		warnings = append(warnings, "compliance policy should have a defined retention period")
	}

	if policy.ExpirationAction == ActionDelete && policy.CompliancePolicy {
		warnings = append(warnings, "compliance policy with delete action may violate regulations")
	}

	return warnings
}

// GetRetentionStats 获取保留策略统计
func (rm *RetentionManager) GetRetentionStats() map[string]interface{} {
	rm.manager.mu.RLock()
	defer rm.manager.mu.RUnlock()

	stats := map[string]interface{}{
		"total_policies":    len(rm.manager.policies),
		"items_with_policy": 0,
		"legal_holds":       0,
		"expiring_soon":     0,
		"expired":           0,
	}

	now := time.Now()
	for _, item := range rm.manager.dataItems {
		if item.RetentionPolicyID != "" {
			stats["items_with_policy"] = stats["items_with_policy"].(int) + 1
		}
		if item.LegalHold {
			stats["legal_holds"] = stats["legal_holds"].(int) + 1
		}
		if item.ExpiresAt != nil {
			if item.ExpiresAt.Before(now) {
				stats["expired"] = stats["expired"].(int) + 1
			} else if item.ExpiresAt.Before(now.AddDate(0, 0, 30)) {
				stats["expiring_soon"] = stats["expiring_soon"].(int) + 1
			}
		}
	}

	return stats
}
