// Package tiering 智能分层规则引擎 - 实现基于文件年龄、访问频率、文件大小的组合条件规则，
// 自动分层迁移引擎，预设策略模板，以及迁移成本预估。
package tiering

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ==================== 智能规则类型定义 ====================

// SmartConditionType 智能规则条件类型.
type SmartConditionType string

const (
	// SmartConditionAge 基于文件年龄（最后访问时间）.
	SmartConditionAge SmartConditionType = "age"
	// SmartConditionFrequency 基于访问频率.
	SmartConditionFrequency SmartConditionType = "frequency"
	// SmartConditionSize 基于文件大小.
	SmartConditionSize SmartConditionType = "size"
)

// LogicalOp 逻辑运算符.
type LogicalOp string

const (
	// LogicalOpAND 所有条件都满足.
	LogicalOpAND LogicalOp = "and"
	// LogicalOpOR 任一条件满足.
	LogicalOpOR LogicalOp = "or"
)

// SmartCondition 智能规则的单个条件.
type SmartCondition struct {
	Type      SmartConditionType `json:"type"`      // 条件类型
	Operator  string             `json:"operator"`  // 比较运算符: >=, <=, ==, >, <
	Threshold int64              `json:"threshold"` // 阈值
	Unit      string             `json:"unit"`      // 单位: days(天), count(次), bytes(字节), mb, gb
}

// SmartRule 智能分层规则 - 支持基于 age/frequency/size 的组合条件.
type SmartRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`

	// 组合条件
	Conditions   []SmartCondition `json:"conditions"`   // 条件列表
	LogicalOp    LogicalOp        `json:"logicalOp"`    // 条件间的逻辑关系
	FilePatterns []string         `json:"filePatterns"` // 文件匹配模式（如 ".mp4,.mkv"）
	ExcludePaths []string         `json:"excludePaths"` // 排除路径

	// 目标分层
	SourceTier TierType     `json:"sourceTier"` // 源存储层（空表示不限）
	TargetTier TierType     `json:"targetTier"` // 目标存储层
	Action     PolicyAction `json:"action"`     // 动作: move/copy/archive

	// 执行配置
	Priority int  `json:"priority"` // 优先级（数字越大越优先执行）
	DryRun   bool `json:"dryRun"`   // 试运行模式
	MaxFiles int  `json:"maxFiles"` // 单次最大迁移文件数（0=不限）

	// 调度
	ScheduleType ScheduleType `json:"scheduleType"` // manual/interval/cron
	ScheduleExpr string       `json:"scheduleExpr"` // cron 表达式或间隔时间

	// 元数据
	LastRun   time.Time `json:"lastRun,omitempty"`
	NextRun   time.Time `json:"nextRun,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// SmartEvaluateResult 智能规则评估结果.
type SmartEvaluateResult struct {
	RuleID       string   `json:"ruleId"`
	RuleName     string   `json:"ruleName"`
	Matched      bool     `json:"matched"`
	MatchedFiles []string `json:"matchedFiles,omitempty"`
	TotalSize    int64    `json:"totalSize"`
}

// SmartExecuteResult 智能规则执行结果.
type SmartExecuteResult struct {
	StartTime       time.Time             `json:"startTime"`
	EndTime         time.Time             `json:"endTime"`
	Duration        time.Duration         `json:"duration"`
	RulesEvaluated  int                   `json:"rulesEvaluated"`
	RulesTriggered  int                   `json:"rulesTriggered"`
	TotalFilesMoved int                   `json:"totalFilesMoved"`
	TotalBytesMoved int64                 `json:"totalBytesMoved"`
	TaskIDs         []string              `json:"taskIds,omitempty"`
	Results         []SmartEvaluateResult `json:"results"`
	DryRun          bool                  `json:"dryRun"`
}

// ==================== 预设策略模板 ====================

// PresetTemplate 分层策略预设模板.
type PresetTemplate struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Category    string      `json:"category"` // performance/balanced/capacity/archive
	Rules       []SmartRule `json:"rules"`
}

// ==================== 迁移计划 ====================

// MigrationPlan 迁移计划.
type MigrationPlan struct {
	ID            string              `json:"id"`
	GeneratedAt   time.Time           `json:"generatedAt"`
	Rules         []SmartRule         `json:"rules"`
	Estimates     []MigrationEstimate `json:"estimates"`
	TotalFiles    int                 `json:"totalFiles"`
	TotalBytes    int64               `json:"totalBytes"`
	EstimatedTime time.Duration       `json:"estimatedTime"`
	EstimatedCost *MigrationCost      `json:"estimatedCost"`
	Summary       string              `json:"summary"`
}

// MigrationEstimate 单条规则的迁移预估.
type MigrationEstimate struct {
	RuleID           string        `json:"ruleId"`
	RuleName         string        `json:"ruleName"`
	MatchedFiles     int           `json:"matchedFiles"`
	MatchedBytes     int64         `json:"matchedBytes"`
	SourceTier       TierType      `json:"sourceTier"`
	TargetTier       TierType      `json:"targetTier"`
	EstimatedTime    time.Duration `json:"estimatedTime"`
	EstimatedSavings float64       `json:"estimatedSavings"` // 预估每月节省费用
}

// MigrationCost 迁移成本预估.
type MigrationCost struct {
	// 迁移成本
	TransferTimeHours float64 `json:"transferTimeHours"` // 迁移耗时（小时）
	TransferBandwidth float64 `json:"transferBandwidth"` // 使用带宽 MB/s

	// 存储成本变化
	CurrentMonthlyCost   float64 `json:"currentMonthlyCost"`   // 当前月存储成本
	ProjectedMonthlyCost float64 `json:"projectedMonthlyCost"` // 迁移后月存储成本
	MonthlySavings       float64 `json:"monthlySavings"`       // 每月节省
	AnnualSavings        float64 `json:"annualSavings"`        // 年度节省
	SavingsPercent       float64 `json:"savingsPercent"`       // 节省比例(%)

	// 性能影响
	EstimatedIOPSChange float64 `json:"estimatedIopsChange"` // 预估IOPS变化(%)
	EstimatedLatencyMs  float64 `json:"estimatedLatencyMs"`  // 预估延迟变化(ms)
}

// ==================== AutoTierEngine ====================

// AutoTierEngine 自动分层迁移引擎.
type AutoTierEngine struct {
	mu      sync.RWMutex
	manager *Manager
	engine  *RulesEngine
	rules   map[string]*SmartRule
	dataDir string
}

// NewAutoTierEngine 创建自动分层引擎.
func NewAutoTierEngine(manager *Manager, engine *RulesEngine, dataDir string) *AutoTierEngine {
	return &AutoTierEngine{
		manager: manager,
		engine:  engine,
		rules:   make(map[string]*SmartRule),
		dataDir: dataDir,
	}
}

// Initialize 初始化自动分层引擎.
func (e *AutoTierEngine) Initialize() error {
	return e.loadSmartRules()
}

// AddSmartRule 创建智能分层规则.
func (e *AutoTierEngine) AddSmartRule(rule SmartRule) (*SmartRule, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if rule.Name == "" {
		return nil, fmt.Errorf("规则名称不能为空")
	}
	if rule.TargetTier == "" {
		return nil, fmt.Errorf("目标存储层不能为空")
	}
	if len(rule.Conditions) == 0 {
		return nil, fmt.Errorf("至少需要一个条件")
	}

	// 验证条件
	for i, cond := range rule.Conditions {
		if err := validateSmartCondition(cond); err != nil {
			return nil, fmt.Errorf("条件 #%d 无效: %w", i+1, err)
		}
	}

	// 默认值
	if rule.LogicalOp == "" {
		rule.LogicalOp = LogicalOpAND
	}
	if rule.Action == "" {
		rule.Action = PolicyActionMove
	}

	if rule.ID == "" {
		rule.ID = "sr_" + uuid.New().String()[:8]
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	e.rules[rule.ID] = &rule

	if err := e.saveSmartRulesLocked(); err != nil {
		delete(e.rules, rule.ID)
		return nil, err
	}

	return &rule, nil
}

// GetSmartRule 获取智能规则.
func (e *AutoTierEngine) GetSmartRule(id string) (*SmartRule, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rule, ok := e.rules[id]
	if !ok {
		return nil, fmt.Errorf("智能规则不存在: %s", id)
	}
	return rule, nil
}

// UpdateSmartRule 更新智能规则.
func (e *AutoTierEngine) UpdateSmartRule(id string, rule SmartRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, ok := e.rules[id]
	if !ok {
		return fmt.Errorf("智能规则不存在: %s", id)
	}

	rule.ID = id
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()

	e.rules[id] = &rule
	return e.saveSmartRulesLocked()
}

// DeleteSmartRule 删除智能规则.
func (e *AutoTierEngine) DeleteSmartRule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.rules[id]; !ok {
		return fmt.Errorf("智能规则不存在: %s", id)
	}

	delete(e.rules, id)
	return e.saveSmartRulesLocked()
}

// ListSmartRules 列出所有智能规则.
func (e *AutoTierEngine) ListSmartRules() []*SmartRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	list := make([]*SmartRule, 0, len(e.rules))
	for _, rule := range e.rules {
		list = append(list, rule)
	}

	// 按优先级排序
	sort.Slice(list, func(i, j int) bool {
		return list[i].Priority > list[j].Priority
	})

	return list
}

// EvaluateSmartRule 评估单个文件是否匹配智能规则.
func (e *AutoTierEngine) EvaluateSmartRule(record *FileAccessRecord, rule *SmartRule) bool {
	if !rule.Enabled {
		return false
	}

	// 检查源层限制
	if rule.SourceTier != "" && record.CurrentTier != rule.SourceTier {
		return false
	}

	// 检查文件模式
	if len(rule.FilePatterns) > 0 && !matchFilePatterns(record.Path, rule.FilePatterns) {
		return false
	}

	// 检查排除路径
	if len(rule.ExcludePaths) > 0 && matchExcludePaths(record.Path, rule.ExcludePaths) {
		return false
	}

	// 评估条件
	if len(rule.Conditions) == 0 {
		return false
	}

	if rule.LogicalOp == LogicalOpOR {
		// OR: 任一条件满足即匹配
		for _, cond := range rule.Conditions {
			if evaluateSmartCondition(record, cond) {
				return true
			}
		}
		return false
	}

	// AND: 所有条件都满足
	for _, cond := range rule.Conditions {
		if !evaluateSmartCondition(record, cond) {
			return false
		}
	}
	return true
}

// ExecuteSmartRules 执行所有启用的智能规则.
func (e *AutoTierEngine) ExecuteSmartRules() (*SmartExecuteResult, error) {
	e.mu.RLock()
	var enabledRules []*SmartRule
	for _, rule := range e.rules {
		if rule.Enabled {
			enabledRules = append(enabledRules, rule)
		}
	}
	e.mu.RUnlock()

	if len(enabledRules) == 0 {
		return nil, fmt.Errorf("没有启用的智能规则")
	}

	// 按优先级排序
	sort.Slice(enabledRules, func(i, j int) bool {
		return enabledRules[i].Priority > enabledRules[j].Priority
	})

	result := &SmartExecuteResult{
		StartTime: time.Now(),
		Results:   make([]SmartEvaluateResult, 0),
	}

	// 已迁移的文件（避免重复迁移）
	migrated := make(map[string]bool)

	// 收集所有文件记录
	allRecords := e.collectAllRecords()

	for _, rule := range enabledRules {
		result.RulesEvaluated++

		evalResult := SmartEvaluateResult{
			RuleID:   rule.ID,
			RuleName: rule.Name,
		}

		var matchedRecords []*FileAccessRecord

		for _, record := range allRecords {
			if migrated[record.Path] {
				continue
			}
			if e.EvaluateSmartRule(record, rule) {
				matchedRecords = append(matchedRecords, record)
				evalResult.MatchedFiles = append(evalResult.MatchedFiles, record.Path)
				evalResult.TotalSize += record.Size
			}
		}

		// 限制单次迁移文件数
		if rule.MaxFiles > 0 && len(matchedRecords) > rule.MaxFiles {
			matchedRecords = matchedRecords[:rule.MaxFiles]
			evalResult.MatchedFiles = evalResult.MatchedFiles[:rule.MaxFiles]
		}

		if len(matchedRecords) > 0 {
			evalResult.Matched = true
			result.RulesTriggered++
			result.TotalFilesMoved += len(matchedRecords)
			result.TotalBytesMoved += evalResult.TotalSize

			if !rule.DryRun {
				taskIDs := e.executeSmartMigration(rule, matchedRecords)
				result.TaskIDs = append(result.TaskIDs, taskIDs...)
			}

			// 标记已迁移
			for _, r := range matchedRecords {
				migrated[r.Path] = true
			}
		}

		// 更新最后执行时间
		e.mu.Lock()
		if r, ok := e.rules[rule.ID]; ok {
			r.LastRun = time.Now()
		}
		e.mu.Unlock()

		result.Results = append(result.Results, evalResult)
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// GenerateMigrationPlan 生成迁移计划（不执行迁移）.
func (e *AutoTierEngine) GenerateMigrationPlan() (*MigrationPlan, error) {
	e.mu.RLock()
	var enabledRules []*SmartRule
	for _, rule := range e.rules {
		if rule.Enabled {
			enabledRules = append(enabledRules, rule)
		}
	}
	e.mu.RUnlock()

	if len(enabledRules) == 0 {
		return nil, fmt.Errorf("没有启用的智能规则")
	}

	sort.Slice(enabledRules, func(i, j int) bool {
		return enabledRules[i].Priority > enabledRules[j].Priority
	})

	allRecords := e.collectAllRecords()
	migrated := make(map[string]bool)

	plan := &MigrationPlan{
		ID:          "plan_" + uuid.New().String()[:8],
		GeneratedAt: time.Now(),
		Estimates:   make([]MigrationEstimate, 0),
	}

	analyzer := NewCostAnalyzer(e.manager)

	for _, rule := range enabledRules {
		est := MigrationEstimate{
			RuleID:     rule.ID,
			RuleName:   rule.Name,
			SourceTier: rule.SourceTier,
			TargetTier: rule.TargetTier,
		}

		var totalBytes int64
		var matchedCount int

		for _, record := range allRecords {
			if migrated[record.Path] {
				continue
			}
			if e.EvaluateSmartRule(record, rule) {
				matchedCount++
				totalBytes += record.Size
				migrated[record.Path] = true
			}
		}

		if rule.MaxFiles > 0 && matchedCount > rule.MaxFiles {
			// 按比例估算大小
			ratio := float64(rule.MaxFiles) / float64(matchedCount)
			totalBytes = int64(float64(totalBytes) * ratio)
			matchedCount = rule.MaxFiles
		}

		est.MatchedFiles = matchedCount
		est.MatchedBytes = totalBytes
		est.EstimatedTime = estimateTransferTime(totalBytes, rule.SourceTier, rule.TargetTier)
		est.EstimatedSavings = analyzer.EstimateTierCostDifference(totalBytes, rule.SourceTier, rule.TargetTier)

		plan.Estimates = append(plan.Estimates, est)
		plan.TotalFiles += matchedCount
		plan.TotalBytes += totalBytes
		plan.EstimatedTime += est.EstimatedTime
	}

	// 汇总成本
	plan.EstimatedCost = analyzer.EstimateMigrationCost(plan.TotalBytes, plan.EstimatedTime)
	plan.Summary = fmt.Sprintf("计划迁移 %d 个文件（%s），预计耗时 %s，预计月节省 $%.2f",
		plan.TotalFiles, formatBytes(plan.TotalBytes), formatDuration(plan.EstimatedTime),
		plan.EstimatedCost.MonthlySavings)

	return plan, nil
}

// EstimateMigrationCost 预估迁移成本.
func EstimateMigrationCost(manager *Manager, totalBytes int64, sourceTier, targetTier TierType) *MigrationCost {
	analyzer := NewCostAnalyzer(manager)
	estTime := estimateTransferTime(totalBytes, sourceTier, targetTier)
	return analyzer.EstimateMigrationCost(totalBytes, estTime)
}

// GetPresetTemplates 获取所有预设策略模板.
func GetPresetTemplates() []PresetTemplate {
	return []PresetTemplate{
		{
			ID:          "tpl_high_performance",
			Name:        "高性能模式",
			Description: "所有数据保留在SSD层，最大化IOPS和低延迟，适合数据库和虚拟机工作负载",
			Category:    "performance",
			Rules: []SmartRule{
				{
					Name:        "SSD热数据保护",
					Description: "频繁访问的文件保持在SSD",
					Enabled:     true,
					Conditions: []SmartCondition{
						{Type: SmartConditionFrequency, Operator: ">=", Threshold: 10, Unit: "count"},
					},
					LogicalOp:  LogicalOpAND,
					TargetTier: TierTypeSSD,
					Action:     PolicyActionMove,
					Priority:   100,
				},
				{
					Name:        "小文件保留在SSD",
					Description: "小文件始终留在SSD以加速访问",
					Enabled:     true,
					Conditions: []SmartCondition{
						{Type: SmartConditionSize, Operator: "<=", Threshold: 100 * 1024 * 1024, Unit: "bytes"},
					},
					LogicalOp:  LogicalOpAND,
					TargetTier: TierTypeSSD,
					Action:     PolicyActionMove,
					Priority:   90,
				},
			},
		},
		{
			ID:          "tpl_balanced",
			Name:        "均衡模式",
			Description: "根据访问频率智能分层，热数据留SSD，冷数据下移HDD，适合通用NAS场景",
			Category:    "balanced",
			Rules: []SmartRule{
				{
					Name:        "热数据提升到SSD",
					Description: "频繁访问的文件提升到SSD层",
					Enabled:     true,
					Conditions: []SmartCondition{
						{Type: SmartConditionFrequency, Operator: ">=", Threshold: 5, Unit: "count"},
					},
					LogicalOp:  LogicalOpAND,
					SourceTier: TierTypeHDD,
					TargetTier: TierTypeSSD,
					Action:     PolicyActionMove,
					Priority:   100,
				},
				{
					Name:        "冷数据下沉到HDD",
					Description: "超过30天未访问的文件迁移到HDD",
					Enabled:     true,
					Conditions: []SmartCondition{
						{Type: SmartConditionAge, Operator: ">=", Threshold: 30, Unit: "days"},
					},
					LogicalOp:  LogicalOpAND,
					SourceTier: TierTypeSSD,
					TargetTier: TierTypeHDD,
					Action:     PolicyActionMove,
					Priority:   80,
				},
			},
		},
		{
			ID:          "tpl_capacity",
			Name:        "大容量模式",
			Description: "最大化存储容量利用，仅保留最热数据在SSD，大量数据下沉HDD/归档",
			Category:    "capacity",
			Rules: []SmartRule{
				{
					Name:        "极热数据保SSD",
					Description: "只有极高频访问的文件保留在SSD",
					Enabled:     true,
					Conditions: []SmartCondition{
						{Type: SmartConditionFrequency, Operator: ">=", Threshold: 20, Unit: "count"},
					},
					LogicalOp:  LogicalOpAND,
					TargetTier: TierTypeSSD,
					Action:     PolicyActionMove,
					Priority:   100,
					MaxFiles:   100,
				},
				{
					Name:        "大文件下沉HDD",
					Description: "大于1GB的文件迁移到HDD",
					Enabled:     true,
					Conditions: []SmartCondition{
						{Type: SmartConditionSize, Operator: ">=", Threshold: 1024 * 1024 * 1024, Unit: "bytes"},
					},
					LogicalOp:  LogicalOpAND,
					SourceTier: TierTypeSSD,
					TargetTier: TierTypeHDD,
					Action:     PolicyActionMove,
					Priority:   90,
				},
				{
					Name:        "冷数据归档",
					Description: "超过14天未访问的文件迁移到HDD",
					Enabled:     true,
					Conditions: []SmartCondition{
						{Type: SmartConditionAge, Operator: ">=", Threshold: 14, Unit: "days"},
					},
					LogicalOp:  LogicalOpAND,
					SourceTier: TierTypeSSD,
					TargetTier: TierTypeHDD,
					Action:     PolicyActionMove,
					Priority:   80,
				},
			},
		},
		{
			ID:          "tpl_archive",
			Name:        "归档模式",
			Description: "最小化在线存储成本，长时间不访问的数据自动归档到云存储",
			Category:    "archive",
			Rules: []SmartRule{
				{
					Name:        "温数据下沉HDD",
					Description: "超过7天未访问的文件迁移到HDD",
					Enabled:     true,
					Conditions: []SmartCondition{
						{Type: SmartConditionAge, Operator: ">=", Threshold: 7, Unit: "days"},
					},
					LogicalOp:  LogicalOpAND,
					SourceTier: TierTypeSSD,
					TargetTier: TierTypeHDD,
					Action:     PolicyActionMove,
					Priority:   100,
				},
				{
					Name:        "冷数据归档到云",
					Description: "超过60天未访问的文件归档到云存储",
					Enabled:     true,
					Conditions: []SmartCondition{
						{Type: SmartConditionAge, Operator: ">=", Threshold: 60, Unit: "days"},
					},
					LogicalOp:  LogicalOpAND,
					SourceTier: TierTypeHDD,
					TargetTier: TierTypeCloud,
					Action:     PolicyActionArchive,
					Priority:   90,
				},
				{
					Name:        "视频文件优先归档",
					Description: "大视频文件超过14天未访问即归档",
					Enabled:     true,
					Conditions: []SmartCondition{
						{Type: SmartConditionAge, Operator: ">=", Threshold: 14, Unit: "days"},
						{Type: SmartConditionSize, Operator: ">=", Threshold: 500 * 1024 * 1024, Unit: "bytes"},
					},
					LogicalOp:    LogicalOpAND,
					FilePatterns: []string{".mp4", ".mkv", ".avi", ".mov", ".wmv"},
					TargetTier:   TierTypeCloud,
					Action:       PolicyActionArchive,
					Priority:     95,
				},
			},
		},
	}
}

// ==================== 内部辅助函数 ====================

// collectAllRecords 收集所有存储层的文件记录.
func (e *AutoTierEngine) collectAllRecords() []*FileAccessRecord {
	var allRecords []*FileAccessRecord
	for _, tier := range []TierType{TierTypeSSD, TierTypeHDD, TierTypeCloud} {
		records := e.manager.tracker.GetRecordsByTier(tier)
		allRecords = append(allRecords, records...)
	}
	return allRecords
}

// executeSmartMigration 对匹配的文件执行迁移.
func (e *AutoTierEngine) executeSmartMigration(rule *SmartRule, records []*FileAccessRecord) []string {
	var paths []string
	for _, record := range records {
		if record.CurrentTier == rule.TargetTier {
			continue
		}
		paths = append(paths, record.Path)
	}

	if len(paths) == 0 {
		return nil
	}

	var taskIDs []string

	// 按源层分组
	sourceTiers := make(map[TierType][]string)
	for _, record := range records {
		if record.CurrentTier == rule.TargetTier {
			continue
		}
		sourceTiers[record.CurrentTier] = append(sourceTiers[record.CurrentTier], record.Path)
	}

	for sourceTier, tierPaths := range sourceTiers {
		task, err := e.manager.Migrate(MigrateRequest{
			Paths:      tierPaths,
			SourceTier: sourceTier,
			TargetTier: rule.TargetTier,
			Action:     rule.Action,
		})
		if err == nil {
			taskIDs = append(taskIDs, task.ID)
		}
	}

	return taskIDs
}

// validateSmartCondition 验证智能条件.
func validateSmartCondition(cond SmartCondition) error {
	switch cond.Type {
	case SmartConditionAge, SmartConditionFrequency, SmartConditionSize:
		// 有效
	default:
		return fmt.Errorf("不支持的条件类型: %s", cond.Type)
	}

	switch cond.Operator {
	case ">=", "<=", "==", ">", "<":
		// 有效
	default:
		return fmt.Errorf("不支持的运算符: %s", cond.Operator)
	}

	if cond.Threshold < 0 {
		return fmt.Errorf("阈值不能为负数")
	}

	return nil
}

// evaluateSmartCondition 评估单个条件.
func evaluateSmartCondition(record *FileAccessRecord, cond SmartCondition) bool {
	var value int64

	switch cond.Type {
	case SmartConditionAge:
		ageDays := int64(time.Since(record.AccessTime).Hours() / 24)
		if ageDays < 0 {
			ageDays = 0
		}
		value = ageDays

	case SmartConditionFrequency:
		value = record.AccessCount

	case SmartConditionSize:
		value = record.Size

	default:
		return false
	}

	threshold := cond.Threshold

	// 根据单位转换阈值
	switch cond.Unit {
	case "mb":
		threshold *= 1024 * 1024
	case "gb":
		threshold *= 1024 * 1024 * 1024
	case "days":
		// 已经是天数
	case "count":
		// 已经是次数
	case "bytes":
		// 已经是字节
	}

	switch cond.Operator {
	case ">=":
		return value >= threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	case ">":
		return value > threshold
	case "<":
		return value < threshold
	default:
		return false
	}
}

// matchFilePatterns 检查文件路径是否匹配模式列表.
func matchFilePatterns(path string, patterns []string) bool {
	ext := filepath.Ext(path)
	for _, pattern := range patterns {
		pattern = normalizePattern(pattern)
		if ext == pattern {
			return true
		}
	}
	return false
}

// matchExcludePaths 检查文件路径是否在排除列表中.
func matchExcludePaths(path string, excludes []string) bool {
	for _, exclude := range excludes {
		if len(path) >= len(exclude) && path[:len(exclude)] == exclude {
			return true
		}
	}
	return false
}

// normalizePattern 规范化文件模式.
func normalizePattern(pattern string) string {
	pattern = patternTrimSpace(pattern)
	if pattern != "" && pattern[0] != '.' {
		pattern = "." + pattern
	}
	return pattern
}

func patternTrimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[len(s)-1] == ' ') {
		if s[0] == ' ' {
			s = s[1:]
		}
		if len(s) > 0 && s[len(s)-1] == ' ' {
			s = s[:len(s)-1]
		}
	}
	return s
}

// estimateTransferTime 预估迁移时间.
func estimateTransferTime(bytes int64, sourceTier, targetTier TierType) time.Duration {
	// 各层间的典型传输速度 (MB/s)
	speeds := map[TierType]float64{
		TierTypeSSD:   500, // SSD: ~500 MB/s
		TierTypeHDD:   150, // HDD: ~150 MB/s
		TierTypeCloud: 50,  // 云: ~50 MB/s (网络受限)
	}

	sourceSpeed := speeds[sourceTier]
	if sourceSpeed == 0 {
		sourceSpeed = 150
	}
	targetSpeed := speeds[targetTier]
	if targetSpeed == 0 {
		targetSpeed = 150
	}

	// 取较慢的一方
	effectiveSpeed := sourceSpeed
	if targetSpeed < effectiveSpeed {
		effectiveSpeed = targetSpeed
	}

	mbTotal := float64(bytes) / (1024 * 1024)
	seconds := mbTotal / effectiveSpeed

	return time.Duration(seconds * float64(time.Second))
}

// formatBytes 格式化字节数.
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// formatDuration 格式化时长.
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// ==================== 持久化 ====================

type smartRulesConfig struct {
	Rules []*SmartRule `json:"rules"`
}

func (e *AutoTierEngine) smartRulesFilePath() string {
	return filepath.Join(e.dataDir, "smart_tiering_rules.json")
}

func (e *AutoTierEngine) loadSmartRules() error {
	path := e.smartRulesFilePath()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var cfg smartRulesConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	e.mu.Lock()
	for _, rule := range cfg.Rules {
		e.rules[rule.ID] = rule
	}
	e.mu.Unlock()

	return nil
}

func (e *AutoTierEngine) saveSmartRulesLocked() error {
	list := make([]*SmartRule, 0, len(e.rules))
	for _, rule := range e.rules {
		list = append(list, rule)
	}

	cfg := smartRulesConfig{Rules: list}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(e.dataDir, 0750); err != nil {
		return err
	}

	return os.WriteFile(e.smartRulesFilePath(), data, 0640)
}
