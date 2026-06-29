package driveinsight

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// TieringEngine 分层规则引擎。
// 基于文件年龄和访问模式自动识别冷热数据，根据可配置规则生成迁移建议。
// 参考群晖 DSM 7.3 Smarter Tiering：自动将非活跃数据迁移至高性价比层。
type TieringEngine struct {
	mu     sync.RWMutex
	logger *zap.Logger
	plans  map[string]*TieringPlan
	rules  map[string]*TieringRule
}

// NewTieringEngine 创建分层规则引擎。
func NewTieringEngine(logger *zap.Logger) *TieringEngine {
	if logger == nil {
		logger = zap.NewNop()
	}
	engine := &TieringEngine{
		logger: logger,
		plans:  make(map[string]*TieringPlan),
		rules:  make(map[string]*TieringRule),
	}
	// 注册默认规则
	engine.registerDefaultRules()
	return engine
}

// registerDefaultRules 注册默认分层规则。
func (e *TieringEngine) registerDefaultRules() {
	defaults := []TieringRule{
		{
			ID:         "default-hot",
			Name:       "热数据保持规则",
			Enabled:    true,
			Priority:   1,
			Conditions: []RuleCondition{
				{Field: RuleFieldLastAccess, Operator: OpLessEqual, Value: "7"},
			},
			TargetTier:  TierIDHot,
			Action:      ActionPin,
			Description: "7天内被访问的数据保持在热数据层",
		},
		{
			ID:         "default-warm",
			Name:       "温数据降级规则",
			Enabled:    true,
			Priority:   2,
			Conditions: []RuleCondition{
				{Field: RuleFieldLastAccess, Operator: OpGreaterThan, Value: "7"},
				{Field: RuleFieldLastAccess, Operator: OpLessEqual, Value: "30"},
			},
			TargetTier:  TierIDWarm,
			Action:      ActionMigrate,
			Description: "7-30天未访问的数据迁移到温数据层",
		},
		{
			ID:         "default-cold",
			Name:       "冷数据降级规则",
			Enabled:    true,
			Priority:   3,
			Conditions: []RuleCondition{
				{Field: RuleFieldLastAccess, Operator: OpGreaterThan, Value: "30"},
				{Field: RuleFieldLastAccess, Operator: OpLessEqual, Value: "90"},
			},
			TargetTier:  TierIDCold,
			Action:      ActionMigrate,
			Description: "30-90天未访问的数据迁移到冷数据层",
		},
		{
			ID:         "default-archive",
			Name:       "归档规则",
			Enabled:    true,
			Priority:   4,
			Conditions: []RuleCondition{
				{Field: RuleFieldLastAccess, Operator: OpGreaterThan, Value: "90"},
				{Field: RuleFieldAge, Operator: OpGreaterThan, Value: "180"},
			},
			TargetTier:  TierIDArchive,
			Action:      ActionArchive,
			Description: "90天以上未访问且存在超180天的数据归档",
		},
	}

	for _, rule := range defaults {
		e.rules[rule.ID] = &rule
	}
}

// AddRule 添加自定义分层规则。
func (e *TieringEngine) AddRule(rule TieringRule) error {
	if rule.ID == "" {
		return fmt.Errorf("规则ID不能为空")
	}
	if rule.Name == "" {
		return fmt.Errorf("规则名称不能为空")
	}
	if len(rule.Conditions) == 0 {
		return fmt.Errorf("规则必须包含至少一个条件")
	}

	// 验证条件
	for i, cond := range rule.Conditions {
		if err := validateCondition(cond); err != nil {
			return fmt.Errorf("条件%d无效: %w", i+1, err)
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.rules[rule.ID]; exists {
		return fmt.Errorf("规则已存在: %s", rule.ID)
	}
	e.rules[rule.ID] = &rule
	e.logger.Info("添加分层规则", zap.String("id", rule.ID), zap.String("name", rule.Name))
	return nil
}

// RemoveRule 删除分层规则。
func (e *TieringEngine) RemoveRule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.rules[id]; !exists {
		return fmt.Errorf("规则不存在: %s", id)
	}
	delete(e.rules, id)
	e.logger.Info("删除分层规则", zap.String("id", id))
	return nil
}

// UpdateRule 更新分层规则。
func (e *TieringEngine) UpdateRule(rule TieringRule) error {
	if rule.ID == "" {
		return fmt.Errorf("规则ID不能为空")
	}
	for i, cond := range rule.Conditions {
		if err := validateCondition(cond); err != nil {
			return fmt.Errorf("条件%d无效: %w", i+1, err)
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.rules[rule.ID]; !exists {
		return fmt.Errorf("规则不存在: %s", rule.ID)
	}
	e.rules[rule.ID] = &rule
	return nil
}

// GetRule 获取规则。
func (e *TieringEngine) GetRule(id string) (*TieringRule, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rule, exists := e.rules[id]
	if !exists {
		return nil, fmt.Errorf("规则不存在: %s", id)
	}
	return rule, nil
}

// ListRules 列出所有规则（按优先级排序）。
func (e *TieringEngine) ListRules() []TieringRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]TieringRule, 0, len(e.rules))
	for _, rule := range e.rules {
		if rule.Enabled {
			result = append(result, *rule)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority < result[j].Priority
	})
	return result
}

// CreatePlan 创建分层计划。
func (e *TieringEngine) CreatePlan(plan TieringPlan) error {
	if plan.ID == "" {
		return fmt.Errorf("计划ID不能为空")
	}
	if plan.Name == "" {
		return fmt.Errorf("计划名称不能为空")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.plans[plan.ID]; exists {
		return fmt.Errorf("计划已存在: %s", plan.ID)
	}
	e.plans[plan.ID] = &plan
	e.logger.Info("创建分层计划", zap.String("id", plan.ID), zap.String("name", plan.Name))
	return nil
}

// GetPlan 获取分层计划。
func (e *TieringEngine) GetPlan(id string) (*TieringPlan, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	plan, exists := e.plans[id]
	if !exists {
		return nil, fmt.Errorf("计划不存在: %s", id)
	}
	return plan, nil
}

// ListPlans 列出所有分层计划。
func (e *TieringEngine) ListPlans() []TieringPlan {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]TieringPlan, 0, len(e.plans))
	for _, plan := range e.plans {
		result = append(result, *plan)
	}
	return result
}

// Evaluate 评估文件访问模式，返回分层建议。
// 按规则优先级依次匹配，返回第一个匹配规则的建议。
func (e *TieringEngine) Evaluate(pattern FileAccessPattern) (*TieringSuggestion, error) {
	rules := e.ListRules()
	now := time.Now()

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		matched, err := e.matchConditions(rule.Conditions, pattern, now)
		if err != nil {
			e.logger.Debug("规则匹配出错",
				zap.String("rule_id", rule.ID),
				zap.Error(err),
			)
			continue
		}
		if matched {
			return &TieringSuggestion{
				File:        pattern.Path,
				RuleID:      rule.ID,
				RuleName:    rule.Name,
				TargetTier:  rule.TargetTier,
				Action:      rule.Action,
				CurrentTier: pattern.DataTier,
				Reason:      rule.Description,
			}, nil
		}
	}

	// 无匹配规则，保持当前层
	return &TieringSuggestion{
		File:        pattern.Path,
		RuleID:      "no-match",
		RuleName:    "无匹配规则",
		TargetTier:  pattern.DataTier,
		Action:      ActionPin,
		CurrentTier: pattern.DataTier,
		Reason:      "未匹配任何分层规则，保持当前层级",
	}, nil
}

// EvaluateBatch 批量评估文件访问模式。
func (e *TieringEngine) EvaluateBatch(patterns []FileAccessPattern) []TieringSuggestion {
	suggestions := make([]TieringSuggestion, 0, len(patterns))
	for _, p := range patterns {
		s, err := e.Evaluate(p)
		if err != nil {
			e.logger.Debug("评估失败", zap.String("path", p.Path), zap.Error(err))
			continue
		}
		suggestions = append(suggestions, *s)
	}
	return suggestions
}

// matchConditions 检查文件是否满足所有规则条件（AND 逻辑）。
func (e *TieringEngine) matchConditions(conditions []RuleCondition, pattern FileAccessPattern, now time.Time) (bool, error) {
	for _, cond := range conditions {
		matched, err := e.matchCondition(cond, pattern, now)
		if err != nil {
			return false, err
		}
		if !matched {
			return false, nil
		}
	}
	return true, nil
}

// matchCondition 检查单个条件。
func (e *TieringEngine) matchCondition(cond RuleCondition, pattern FileAccessPattern, now time.Time) (bool, error) {
	switch cond.Field {
	case RuleFieldAge:
		days := now.Sub(pattern.ModTime).Hours() / 24
		return compareNumeric(days, cond.Operator, cond.Value)

	case RuleFieldLastAccess:
		days := now.Sub(pattern.AccessTime).Hours() / 24
		return compareNumeric(days, cond.Operator, cond.Value)

	case RuleFieldAccessCount:
		return compareNumeric(float64(pattern.AccessCount), cond.Operator, cond.Value)

	case RuleFieldSize:
		sizeMB := float64(pattern.Size) / float64(1024*1024)
		return compareNumeric(sizeMB, cond.Operator, cond.Value)

	case RuleFileType:
		return compareString(pattern.Path, cond.Operator, cond.Value)

	default:
		return false, fmt.Errorf("未知条件字段: %s", cond.Field)
	}
}

// compareNumeric 数值比较。
func compareNumeric(actual float64, op RuleOperator, valueStr string) (bool, error) {
	expected, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return false, fmt.Errorf("条件值不是有效数字: %s", valueStr)
	}

	switch op {
	case OpGreaterThan:
		return actual > expected, nil
	case OpLessThan:
		return actual < expected, nil
	case OpEqual:
		return actual == expected, nil
	case OpGreaterEqual:
		return actual >= expected, nil
	case OpLessEqual:
		return actual <= expected, nil
	default:
		return false, fmt.Errorf("数值比较不支持操作符: %s", op)
	}
}

// compareString 字符串比较。
func compareString(actual string, op RuleOperator, value string) (bool, error) {
	switch op {
	case OpContains:
		return strings.Contains(strings.ToLower(actual), strings.ToLower(value)), nil
	case OpEqual:
		return strings.EqualFold(actual, value), nil
	default:
		return false, fmt.Errorf("字符串比较不支持操作符: %s", op)
	}
}

// validateCondition 验证条件有效性。
func validateCondition(cond RuleCondition) error {
	switch cond.Field {
	case RuleFieldAge, RuleFieldLastAccess, RuleFieldAccessCount, RuleFieldSize:
		if cond.Operator == OpContains {
			return fmt.Errorf("数值字段不支持 contains 操作符")
		}
		if cond.Value == "" {
			return fmt.Errorf("条件值不能为空")
		}
		if _, err := strconv.ParseFloat(cond.Value, 64); err != nil {
			return fmt.Errorf("数值字段值无效: %s", cond.Value)
		}
	case RuleFileType:
		if cond.Operator != OpContains && cond.Operator != OpEqual {
			return fmt.Errorf("文件类型字段仅支持 contains 和 = 操作符")
		}
	default:
		return fmt.Errorf("未知条件字段: %s", cond.Field)
	}
	return nil
}

// TieringSuggestion 分层建议。
type TieringSuggestion struct {
	File        string     `json:"file"`         // 文件路径
	RuleID      string     `json:"rule_id"`      // 匹配规则ID
	RuleName    string     `json:"rule_name"`    // 规则名称
	TargetTier  TierID     `json:"target_tier"`  // 建议目标层
	CurrentTier TierID     `json:"current_tier"` // 当前层
	Action      RuleAction `json:"action"`       // 建议动作
	Reason      string     `json:"reason"`       // 原因说明
}

// GenerateMigrationPlan 生成迁移计划。
// 对一组文件访问模式进行评估，生成需要迁移的文件列表。
func (e *TieringEngine) GenerateMigrationPlan(patterns []FileAccessPattern) *MigrationPlan {
	plan := &MigrationPlan{
		GeneratedAt: time.Now(),
		Total:       len(patterns),
	}

	for _, p := range patterns {
		s, err := e.Evaluate(p)
		if err != nil {
			plan.Skipped++
			continue
		}

		if s.Action == ActionMigrate || s.Action == ActionArchive {
			// 检查是否确实需要迁移（当前层 != 目标层）
			if s.CurrentTier != s.TargetTier {
				plan.Migrations = append(plan.Migrations, MigrationItem{
					File:        p.Path,
					Size:        p.Size,
					CurrentTier: s.CurrentTier,
					TargetTier:  s.TargetTier,
					RuleID:      s.RuleID,
					Action:      s.Action,
					Reason:      s.Reason,
				})
				plan.MigrateSize += p.Size
			} else {
				plan.NoAction++
			}
		} else {
			plan.NoAction++
		}
	}

	plan.Pending = len(plan.Migrations)

	e.logger.Info("生成迁移计划",
		zap.Int("total", plan.Total),
		zap.Int("pending", plan.Pending),
		zap.Int64("migrate_size_bytes", plan.MigrateSize),
	)

	return plan
}

// MigrationPlan 迁移计划。
type MigrationPlan struct {
	GeneratedAt  time.Time       `json:"generated_at"`
	Total        int             `json:"total"`          // 文件总数
	Pending      int             `json:"pending"`        // 待迁移数
	NoAction     int             `json:"no_action"`      // 无需迁移
	Skipped      int             `json:"skipped"`        // 跳过（评估失败）
	MigrateSize  int64           `json:"migrate_size"`   // 迁移总大小（字节）
	Migrations   []MigrationItem `json:"migrations"`     // 迁移项列表
}

// MigrationItem 单个迁移项。
type MigrationItem struct {
	File        string     `json:"file"`
	Size        int64      `json:"size"`
	CurrentTier TierID     `json:"current_tier"`
	TargetTier  TierID     `json:"target_tier"`
	RuleID      string     `json:"rule_id"`
	Action      RuleAction `json:"action"`
	Reason      string     `json:"reason"`
}

// IdentifyColdData 识别冷数据。
// 返回所有被分类为 Cold 或 Frozen 的文件模式。
func (e *TieringEngine) IdentifyColdData(patterns []FileAccessPattern) []FileAccessPattern {
	var cold []FileAccessPattern
	for _, p := range patterns {
		if p.AccessFreq == AccessFreqCold || p.AccessFreq == AccessFreqFrozen {
			cold = append(cold, p)
		}
	}
	return cold
}

// IdentifyHotData 识别热数据。
// 返回所有被分类为 Hot 的文件模式。
func (e *TieringEngine) IdentifyHotData(patterns []FileAccessPattern) []FileAccessPattern {
	var hot []FileAccessPattern
	for _, p := range patterns {
		if p.AccessFreq == AccessFreqHot {
			hot = append(hot, p)
		}
	}
	return hot
}

// GetDataDistribution 获取数据分布统计。
func (e *TieringEngine) GetDataDistribution(patterns []FileAccessPattern) map[AccessFreq]int {
	dist := map[AccessFreq]int{
		AccessFreqHot:    0,
		AccessFreqWarm:   0,
		AccessFreqCold:   0,
		AccessFreqFrozen: 0,
	}
	for _, p := range patterns {
		dist[p.AccessFreq]++
	}
	return dist
}
