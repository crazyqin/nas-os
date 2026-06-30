// Package tieringrules 提供数据分层自定义规则功能。
package tieringrules

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// FileItem 文件项（用于规则评估）
type FileItem struct {
	Path       string
	Size       int64
	AccessFreq int       // 过去N天访问次数
	ModifyTime time.Time // 最后修改时间
}

// Engine 分层规则引擎
type Engine struct {
	mu       sync.RWMutex
	rules    map[string]*TieringRule
	history  []*MigrationRecord
}

// NewEngine 创建规则引擎
func NewEngine() *Engine {
	return &Engine{
		rules:   make(map[string]*TieringRule),
		history: make([]*MigrationRecord, 0),
	}
}

// CreateRule 创建规则
func (e *Engine) CreateRule(req *CreateRuleRequest) (*TieringRule, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("rule name is required")
	}
	if req.SourcePool == "" || req.TargetPool == "" {
		return nil, fmt.Errorf("source and target pools are required")
	}
	if req.SourcePool == req.TargetPool {
		return nil, fmt.Errorf("source and target pools must be different")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	rule := &TieringRule{
		ID:         generateID(),
		Name:       req.Name,
		Condition:  req.Condition,
		Threshold:  req.Threshold,
		SourcePool: req.SourcePool,
		TargetPool: req.TargetPool,
		Action:     req.Action,
		Enabled:    req.Enabled,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	e.rules[rule.ID] = rule
	return rule, nil
}

// GetRule 获取规则
func (e *Engine) GetRule(id string) (*TieringRule, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	r, ok := e.rules[id]
	if !ok {
		return nil, fmt.Errorf("rule not found: %s", id)
	}
	return r, nil
}

// ListRules 列出所有规则
func (e *Engine) ListRules() []*TieringRule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*TieringRule, 0, len(e.rules))
	for _, r := range e.rules {
		result = append(result, r)
	}
	return result
}

// UpdateRule 更新规则
func (e *Engine) UpdateRule(id string, req *CreateRuleRequest) (*TieringRule, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, ok := e.rules[id]
	if !ok {
		return nil, fmt.Errorf("rule not found: %s", id)
	}
	if req.Name != "" {
		r.Name = req.Name
	}
	if req.Condition != "" {
		r.Condition = req.Condition
	}
	if req.Threshold > 0 {
		r.Threshold = req.Threshold
	}
	if req.SourcePool != "" {
		r.SourcePool = req.SourcePool
	}
	if req.TargetPool != "" {
		r.TargetPool = req.TargetPool
	}
	if req.Action != "" {
		r.Action = req.Action
	}
	r.Enabled = req.Enabled
	r.UpdatedAt = time.Now()
	return r, nil
}

// DeleteRule 删除规则
func (e *Engine) DeleteRule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.rules[id]; !ok {
		return fmt.Errorf("rule not found: %s", id)
	}
	delete(e.rules, id)
	return nil
}

// EvaluateFile 评估文件是否匹配任一启用的规则
func (e *Engine) EvaluateFile(f *FileItem) []*TieringRule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var matched []*TieringRule
	for _, r := range e.rules {
		if !r.Enabled {
			continue
		}
		if e.matchCondition(r, f) {
			matched = append(matched, r)
		}
	}
	return matched
}

// matchCondition 检查文件是否匹配规则条件
func (e *Engine) matchCondition(r *TieringRule, f *FileItem) bool {
	switch r.Condition {
	case ConditionAccessFreq:
		return int64(f.AccessFreq) < r.Threshold
	case ConditionModifyTime:
		days := int64(time.Since(f.ModifyTime).Hours() / 24)
		return days > r.Threshold
	case ConditionSize:
		return f.Size > r.Threshold
	}
	return false
}

// ExecuteMigration 执行数据迁移
func (e *Engine) ExecuteMigration(rule *TieringRule, f *FileItem) (*MigrationRecord, error) {
	rec := &MigrationRecord{
		ID:         generateID(),
		RuleID:     rule.ID,
		RuleName:   rule.Name,
		FilePath:   f.Path,
		SourcePool: rule.SourcePool,
		TargetPool: rule.TargetPool,
		FileSize:   f.Size,
		Status:     MigrationStatusPending,
		MigratedAt: time.Now(),
	}
	// Simulate migration (in real impl, would move/copy data)
	rec.Status = MigrationStatusSuccess

	e.mu.Lock()
	e.history = append(e.history, rec)
	e.mu.Unlock()
	return rec, nil
}

// GetHistory 获取迁移历史
func (e *Engine) GetHistory() []*MigrationRecord {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*MigrationRecord, len(e.history))
	copy(result, e.history)
	return result
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}