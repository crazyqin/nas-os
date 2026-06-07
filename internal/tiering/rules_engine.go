// Package tiering Smart Tiering Rules Engine
package tiering

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ConditionType 条件类型.
type ConditionType string

const (
	// ConditionTypeAge 文件年龄条件.
	ConditionTypeAge ConditionType = "age"
	// ConditionTypeAccessFrequency 访问频率条件.
	ConditionTypeAccessFrequency ConditionType = "access_frequency"
	// ConditionTypeFileType 文件类型条件.
	ConditionTypeFileType ConditionType = "file_type"
)

// TieringRule 分层规则.
type TieringRule struct {
	ID            string        `json:"id"`
	RuleName      string        `json:"ruleName"`
	TierType      TierType      `json:"tierType"`      // 目标存储层
	ConditionType ConditionType `json:"conditionType"` // 条件类型
	Threshold     int64         `json:"threshold"`     // 阈值（天数/次数）
	FilePattern   string        `json:"filePattern"`   // 文件扩展名模式（如 ".mp4,.mkv"）
	Enabled       bool          `json:"enabled"`       // 是否启用
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
}

// EvaluateResult 规则评估结果.
type EvaluateResult struct {
	RuleID   string   `json:"ruleId"`
	RuleName string   `json:"ruleName"`
	Matched  bool     `json:"matched"`
	Files    []string `json:"files,omitempty"`
}

// ExecuteResult 规则执行结果.
type ExecuteResult struct {
	StartTime    time.Time        `json:"startTime"`
	EndTime      time.Time        `json:"endTime"`
	Duration     time.Duration    `json:"duration"`
	RulesApplied int              `json:"rulesApplied"`
	FilesMatched int              `json:"filesMatched"`
	Tasks        []string         `json:"tasks"`
	Results      []EvaluateResult `json:"results"`
}

// RulesEngine 分层规则引擎.
type RulesEngine struct {
	mu      sync.RWMutex
	manager *Manager
	rules   map[string]*TieringRule
	dataDir string
}

// NewRulesEngine 创建规则引擎.
func NewRulesEngine(manager *Manager, dataDir string) *RulesEngine {
	return &RulesEngine{
		manager: manager,
		rules:   make(map[string]*TieringRule),
		dataDir: dataDir,
	}
}

// Initialize 初始化规则引擎.
func (e *RulesEngine) Initialize() error {
	return e.loadRules()
}

// Evaluate 评估单个文件是否匹配规则.
func (e *RulesEngine) Evaluate(file *FileAccessRecord, rule *TieringRule) bool {
	if !rule.Enabled {
		return false
	}

	switch rule.ConditionType {
	case ConditionTypeAge:
		return e.evaluateAge(file, rule)
	case ConditionTypeAccessFrequency:
		return e.evaluateAccessFrequency(file, rule)
	case ConditionTypeFileType:
		return e.evaluateFileType(file, rule)
	default:
		return false
	}
}

// evaluateAge 评估文件年龄条件.
func (e *RulesEngine) evaluateAge(file *FileAccessRecord, rule *TieringRule) bool {
	if rule.Threshold <= 0 {
		return false
	}
	ageDays := int64(time.Since(file.ModTime).Hours() / 24)
	return ageDays >= rule.Threshold
}

// evaluateAccessFrequency 评估访问频率条件.
func (e *RulesEngine) evaluateAccessFrequency(file *FileAccessRecord, rule *TieringRule) bool {
	if rule.Threshold <= 0 {
		return false
	}
	// 计算每天访问次数
	daysSinceFirstAccess := int64(time.Since(file.AccessTime).Hours() / 24)
	if daysSinceFirstAccess <= 0 {
		daysSinceFirstAccess = 1
	}
	freqPerDay := file.AccessCount / daysSinceFirstAccess
	return freqPerDay >= rule.Threshold
}

// evaluateFileType 评估文件类型条件.
func (e *RulesEngine) evaluateFileType(file *FileAccessRecord, rule *TieringRule) bool {
	if rule.FilePattern == "" {
		return false
	}

	ext := strings.ToLower(filepath.Ext(file.Path))
	patterns := strings.Split(rule.FilePattern, ",")

	for _, pattern := range patterns {
		pattern = strings.TrimSpace(strings.ToLower(pattern))
		if !strings.HasPrefix(pattern, ".") {
			pattern = "." + pattern
		}
		if ext == pattern {
			return true
		}
	}
	return false
}

// AddRule 添加规则.
func (e *RulesEngine) AddRule(rule TieringRule) (*TieringRule, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if rule.RuleName == "" {
		return nil, fmt.Errorf("规则名称不能为空")
	}

	if rule.ConditionType == "" {
		return nil, fmt.Errorf("条件类型不能为空")
	}

	// 验证条件类型
	switch rule.ConditionType {
	case ConditionTypeAge, ConditionTypeAccessFrequency, ConditionTypeFileType:
		// 有效
	default:
		return nil, fmt.Errorf("无效的条件类型: %s", rule.ConditionType)
	}

	// 生成 ID
	if rule.ID == "" {
		rule.ID = "rule_" + uuid.New().String()[:8]
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	e.rules[rule.ID] = &rule

	if err := e.saveRulesLocked(); err != nil {
		delete(e.rules, rule.ID)
		return nil, err
	}

	return &rule, nil
}

// GetRule 获取规则.
func (e *RulesEngine) GetRule(id string) (*TieringRule, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rule, ok := e.rules[id]
	if !ok {
		return nil, fmt.Errorf("规则不存在: %s", id)
	}

	return rule, nil
}

// UpdateRule 更新规则.
func (e *RulesEngine) UpdateRule(id string, rule TieringRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, ok := e.rules[id]
	if !ok {
		return fmt.Errorf("规则不存在: %s", id)
	}

	rule.ID = id
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()

	e.rules[id] = &rule

	return e.saveRulesLocked()
}

// RemoveRule 删除规则.
func (e *RulesEngine) RemoveRule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.rules[id]; !ok {
		return fmt.Errorf("规则不存在: %s", id)
	}

	delete(e.rules, id)

	return e.saveRulesLocked()
}

// ListRules 列出所有规则.
func (e *RulesEngine) ListRules() []*TieringRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	list := make([]*TieringRule, 0, len(e.rules))
	for _, rule := range e.rules {
		list = append(list, rule)
	}
	return list
}

// Execute 执行所有启用的规则，扫描存储池并自动迁移文件.
func (e *RulesEngine) Execute() (*ExecuteResult, error) {
	e.mu.RLock()
	var enabledRules []*TieringRule
	for _, rule := range e.rules {
		if rule.Enabled {
			enabledRules = append(enabledRules, rule)
		}
	}
	e.mu.RUnlock()

	if len(enabledRules) == 0 {
		return nil, fmt.Errorf("没有启用的规则")
	}

	result := &ExecuteResult{
		StartTime: time.Now(),
		Results:   make([]EvaluateResult, 0),
	}

	// 收集所有文件记录
	allRecords := e.collectAllRecords()

	for _, rule := range enabledRules {
		evalResult := EvaluateResult{
			RuleID:   rule.ID,
			RuleName: rule.RuleName,
			Matched:  false,
		}

		var matchedFiles []string
		var matchedRecords []*FileAccessRecord

		for _, record := range allRecords {
			if e.Evaluate(record, rule) {
				matchedFiles = append(matchedFiles, record.Path)
				matchedRecords = append(matchedRecords, record)
			}
		}

		if len(matchedFiles) > 0 {
			evalResult.Matched = true
			evalResult.Files = matchedFiles
			result.FilesMatched += len(matchedFiles)
			result.RulesApplied++

			// 执行迁移
			taskIDs := e.executeMigration(rule, matchedRecords)
			result.Tasks = append(result.Tasks, taskIDs...)
		}

		result.Results = append(result.Results, evalResult)
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// collectAllRecords 收集所有存储层的文件记录.
func (e *RulesEngine) collectAllRecords() []*FileAccessRecord {
	var allRecords []*FileAccessRecord
	for _, tier := range []TierType{TierTypeSSD, TierTypeHDD, TierTypeCloud} {
		records := e.manager.tracker.GetRecordsByTier(tier)
		allRecords = append(allRecords, records...)
	}
	return allRecords
}

// executeMigration 对匹配的文件执行迁移.
func (e *RulesEngine) executeMigration(rule *TieringRule, records []*FileAccessRecord) []string {
	var paths []string
	for _, record := range records {
		// 跳过已经在目标层的文件
		if record.CurrentTier == rule.TierType {
			continue
		}
		paths = append(paths, record.Path)
	}

	if len(paths) == 0 {
		return nil
	}

	// 查找合适的源层（从当前层迁移到规则指定的目标层）
	// 按优先级排列
	tierOrder := []TierType{TierTypeSSD, TierTypeHDD, TierTypeCloud}

	var taskIDs []string

	for _, sourceTier := range tierOrder {
		if sourceTier == rule.TierType {
			continue
		}

		var sourcePaths []string
		for _, record := range records {
			if record.CurrentTier == sourceTier {
				sourcePaths = append(sourcePaths, record.Path)
			}
		}

		if len(sourcePaths) == 0 {
			continue
		}

		task, err := e.manager.Migrate(MigrateRequest{
			Paths:      sourcePaths,
			SourceTier: sourceTier,
			TargetTier: rule.TierType,
			Action:     PolicyActionMove,
		})

		if err == nil {
			taskIDs = append(taskIDs, task.ID)
		}
	}

	return taskIDs
}

// ==================== 持久化 ====================

type rulesConfig struct {
	Rules []*TieringRule `json:"rules"`
}

func (e *RulesEngine) rulesFilePath() string {
	return filepath.Join(e.dataDir, "tiering_rules.json")
}

func (e *RulesEngine) loadRules() error {
	path := e.rulesFilePath()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // 文件不存在，使用空规则
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var cfg rulesConfig
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

func (e *RulesEngine) saveRulesLocked() error {
	list := make([]*TieringRule, 0, len(e.rules))
	for _, rule := range e.rules {
		list = append(list, rule)
	}

	cfg := rulesConfig{Rules: list}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(e.dataDir, 0750); err != nil {
		return err
	}

	return os.WriteFile(e.rulesFilePath(), data, 0640)
}
