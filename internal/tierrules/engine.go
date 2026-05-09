package tierrules

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// MigrateFunc 迁移执行函数（由外部注入，便于测试）.
type MigrateFunc func(ctx context.Context, file FileInfo, target StorageTier) error

// FileListFunc 文件列表获取函数（由外部注入，便于测试）.
type FileListFunc func(ctx context.Context) ([]FileInfo, error)

// Engine 分层规则引擎.
type Engine struct {
	mu          sync.RWMutex
	rules       []TierRule
	stats       TierStats
	logger      *zap.Logger
	migrateFunc MigrateFunc
	listFunc    FileListFunc
}

// NewEngine 创建分层规则引擎.
func NewEngine(logger *zap.Logger) *Engine {
	return &Engine{
		rules:  make([]TierRule, 0),
		logger: logger,
	}
}

// SetMigrateFunc 设置迁移执行函数.
func (e *Engine) SetMigrateFunc(fn MigrateFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.migrateFunc = fn
}

// SetFileListFunc 设置文件列表获取函数.
func (e *Engine) SetFileListFunc(fn FileListFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.listFunc = fn
}

// AddRule 添加分层规则.
func (e *Engine) AddRule(rule TierRule) error {
	if rule.Name == "" {
		return ErrRuleNameEmpty
	}
	if !ValidTiers[rule.SourceTier] {
		return fmt.Errorf("%w: %s", ErrInvalidTier, rule.SourceTier)
	}
	if !ValidTiers[rule.TargetTier] {
		return fmt.Errorf("%w: %s", ErrInvalidTier, rule.TargetTier)
	}
	if rule.SourceTier == rule.TargetTier {
		return ErrSameTier
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// 检查名称是否重复
	for _, r := range e.rules {
		if r.Name == rule.Name {
			return ErrRuleNameDuplicate
		}
	}

	now := time.Now()
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = now
	}
	rule.UpdatedAt = now
	if !rule.Enabled {
		rule.Enabled = true // 默认启用
	}

	e.rules = append(e.rules, rule)
	e.logger.Info("添加分层规则", zap.String("name", rule.Name), zap.String("source", string(rule.SourceTier)), zap.String("target", string(rule.TargetTier)))
	return nil
}

// RemoveRule 删除分层规则.
func (e *Engine) RemoveRule(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, r := range e.rules {
		if r.Name == name {
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			e.logger.Info("删除分层规则", zap.String("name", name))
			return nil
		}
	}
	return ErrRuleNotFound
}

// ListRules 列出所有规则（按优先级降序）.
func (e *Engine) ListRules() []TierRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	out := make([]TierRule, len(e.rules))
	copy(out, e.rules)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Priority > out[j].Priority
	})
	return out
}

// Evaluate 评估文件应迁移到哪个层级.
// 按优先级降序遍历规则，返回第一个匹配的目标层级.
func (e *Engine) Evaluate(file FileInfo) (string, error) {
	if file.CurrentTier == "" {
		return "", fmt.Errorf("%w: 文件当前层级为空", ErrInvalidTier)
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	// 按优先级降序排列
	rules := make([]TierRule, len(e.rules))
	copy(rules, e.rules)
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if rule.SourceTier != file.CurrentTier {
			continue
		}
		if matchConditions(file, rule.Conditions) {
			e.logger.Debug("文件匹配规则",
				zap.String("file", file.Path),
				zap.String("rule", rule.Name),
				zap.String("target", string(rule.TargetTier)),
			)
			return string(rule.TargetTier), nil
		}
	}

	return "", ErrNoMatchingRule
}

// RunBatch 批量执行分层迁移.
func (e *Engine) RunBatch(ctx context.Context) (*TierStats, error) {
	return e.runBatchInternal(ctx, false)
}

// RunBatchDryRun 试运行批量迁移（不实际执行迁移）.
func (e *Engine) RunBatchDryRun(ctx context.Context) (*TierStats, error) {
	return e.runBatchInternal(ctx, true)
}

// runBatchInternal 内部批量执行.
func (e *Engine) runBatchInternal(ctx context.Context, dryRun bool) (*TierStats, error) {
	e.mu.RLock()
	listFn := e.listFunc
	migFn := e.migrateFunc
	e.mu.RUnlock()

	if listFn == nil {
		return nil, fmt.Errorf("未设置文件列表函数")
	}
	if migFn == nil && !dryRun {
		return nil, fmt.Errorf("未设置迁移执行函数")
	}

	// 获取文件列表
	files, err := listFn(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取文件列表失败: %w", err)
	}

	stats := &TierStats{}
	now := time.Now()

	for _, file := range files {
		// 检查 context 是否取消
		select {
		case <-ctx.Done():
			e.logger.Warn("批量迁移被取消")
			return stats, ctx.Err()
		default:
		}

		tier, err := e.Evaluate(file)
		if err != nil {
			if err == ErrNoMatchingRule {
				continue
			}
			stats.ErrorCount++
			e.logger.Error("评估文件失败", zap.String("file", file.Path), zap.Error(err))
			continue
		}

		if dryRun {
			stats.TotalMoved++
			stats.TotalBytes += file.Size
			e.logger.Info("[试运行] 将迁移文件",
				zap.String("file", file.Path),
				zap.String("from", string(file.CurrentTier)),
				zap.String("to", tier),
			)
			continue
		}

		// 执行迁移
		if err := migFn(ctx, file, StorageTier(tier)); err != nil {
			stats.ErrorCount++
			e.logger.Error("迁移文件失败", zap.String("file", file.Path), zap.Error(err))
			continue
		}

		stats.TotalMoved++
		stats.TotalBytes += file.Size
	}

	stats.LastRunTime = now

	// 更新全局统计
	e.mu.Lock()
	e.stats.TotalMoved += stats.TotalMoved
	e.stats.TotalBytes += stats.TotalBytes
	e.stats.LastRunTime = now
	e.stats.ErrorCount += stats.ErrorCount
	e.mu.Unlock()

	return stats, nil
}

// GetStats 获取分层迁移统计.
func (e *Engine) GetStats() TierStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// ========== 条件匹配 ==========

// matchConditions 检查文件是否满足所有条件.
func matchConditions(file FileInfo, cond TierConditions) bool {
	// 检查未访问天数
	if cond.MaxAccessDays > 0 {
		days := int(time.Since(file.AccessTime).Hours() / 24)
		if days < cond.MaxAccessDays {
			return false
		}
	}

	// 检查文件年龄
	if cond.MinAgeDays > 0 {
		days := int(time.Since(file.ModTime).Hours() / 24)
		if days < cond.MinAgeDays {
			return false
		}
	}

	// 检查文件大小下限
	if cond.MinSizeBytes > 0 && file.Size < cond.MinSizeBytes {
		return false
	}

	// 检查文件大小上限
	if cond.MaxSizeBytes > 0 && file.Size > cond.MaxSizeBytes {
		return false
	}

	// 检查文件名模式
	if len(cond.FilePatterns) > 0 {
		matched := false
		for _, pattern := range cond.FilePatterns {
			if matchFilePattern(file.Name, pattern) {
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

// matchFilePattern 文件名模式匹配（支持通配符 * 和 ?）.
func matchFilePattern(name, pattern string) bool {
	// 支持扩展名简写（如 ".log" 匹配 "*.log"）
	if strings.HasPrefix(pattern, ".") {
		pattern = "*" + pattern
	}
	matched, _ := filepath.Match(pattern, name)
	return matched
}
