// Package aiintent 实现AI意图引擎 - 自然语言驱动的存储管理
// 对标: 群晖 DSM Agent 自动化 + TrueNAS CLI 智能化
// 用户可以用自然语言描述存储需求, AI自动解析意图并执行操作
package aiintent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// IntentType 意图类型
type IntentType string

const (
	IntentStorageCreate   IntentType = "storage_create"   // 创建存储
	IntentStorageExpand   IntentType = "storage_expand"   // 扩容存储
	IntentBackupSchedule  IntentType = "backup_schedule"   // 备份调度
	IntentDataMigration   IntentType = "data_migration"   // 数据迁移
	IntentAccessControl   IntentType = "access_control"   // 权限控制
	IntentPerformanceTune IntentType = "performance_tune" // 性能调优
	IntentCostOptimize    IntentType = "cost_optimize"    // 成本优化
	IntentSecurityHarden  IntentType = "security_harden"  // 安全加固
	IntentQuery           IntentType = "query"            // 查询
	IntentUnknown         IntentType = "unknown"          // 未知
)

// Intent 意图
type Intent struct {
	ID          string            `json:"id"`
	Type        IntentType        `json:"type"`
	RawText     string            `json:"raw_text"`
	Parameters  map[string]string `json:"parameters"`
	Confidence  float64           `json:"confidence"`
	Status      IntentStatus      `json:"status"`
	Result      string            `json:"result,omitempty"`
	Error       string            `json:"error,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Actions     []Action          `json:"actions"`
}

// IntentStatus 意图状态
type IntentStatus string

const (
	StatusPending    IntentStatus = "pending"
	StatusParsing    IntentStatus = "parsing"
	StatusExecuting  IntentStatus = "executing"
	StatusCompleted  IntentStatus = "completed"
	StatusFailed     IntentStatus = "failed"
	StatusCancelled  IntentStatus = "cancelled"
)

// Action 执行动作
type Action struct {
	Type       string            `json:"type"`
	Target     string            `json:"target"`
	Parameters map[string]string `json:"parameters"`
	Status     string            `json:"status"`
	Result     string            `json:"result,omitempty"`
	Error      string            `json:"error,omitempty"`
	StartedAt  time.Time         `json:"started_at"`
	EndedAt    *time.Time        `json:"ended_at,omitempty"`
}

// IntentPattern 意图模式
type IntentPattern struct {
	Type      IntentType
	Keywords  []string
	Patterns  []string
	Handler   IntentHandler
}

// IntentHandler 意图处理器
type IntentHandler func(ctx context.Context, intent *Intent) error

// Engine AI意图引擎
type Engine struct {
	mu          sync.RWMutex
	intents     map[string]*Intent
	patterns    []IntentPattern
	handlers    map[IntentType]IntentHandler
	maxHistory  int
	totalParsed int
	totalExec   int
	totalFailed int
}

// NewEngine 创建意图引擎
func NewEngine() *Engine {
	e := &Engine{
		intents:    make(map[string]*Intent),
		patterns:   make([]IntentPattern, 0),
		handlers:   make(map[IntentType]IntentHandler),
		maxHistory: 1000,
	}
	e.registerDefaultPatterns()
	return e
}

// registerDefaultPatterns 注册默认意图模式
func (e *Engine) registerDefaultPatterns() {
	defaults := []IntentPattern{
		{
			Type:     IntentStorageCreate,
			Keywords: []string{"创建", "新建", "create", "添加存储", "创建存储池", "创建卷"},
			Patterns: []string{"创建*存储*", "新建*池*", "create *volume*"},
		},
		{
			Type:     IntentStorageExpand,
			Keywords: []string{"扩容", "扩展", "expand", "增大", "增加容量"},
			Patterns: []string{"扩容*存储*", "扩展*容量*", "expand *storage*"},
		},
		{
			Type:     IntentBackupSchedule,
			Keywords: []string{"备份", "backup", "定时备份", "自动备份", "备份计划"},
			Patterns: []string{"设置*备份*", "创建*备份计划*", "schedule *backup*"},
		},
		{
			Type:     IntentDataMigration,
			Keywords: []string{"迁移", "移动", "migrate", "搬迁", "转移数据"},
			Patterns: []string{"迁移*数据*", "移动*到*", "migrate *to*"},
		},
		{
			Type:     IntentAccessControl,
			Keywords: []string{"权限", "访问控制", "permission", "授权", "共享"},
			Patterns: []string{"设置*权限*", "授权*用户*", "share *with*"},
		},
		{
			Type:     IntentPerformanceTune,
			Keywords: []string{"性能", "优化", "performance", "加速", "调优"},
			Patterns: []string{"优化*性能*", "加速*存储*", "tune *performance*"},
		},
		{
			Type:     IntentCostOptimize,
			Keywords: []string{"成本", "省钱", "cost", "降本", "费用"},
			Patterns: []string{"优化*成本*", "降低*费用*", "optimize *cost*"},
		},
		{
			Type:     IntentSecurityHarden,
			Keywords: []string{"安全", "加密", "security", "加固", "防护"},
			Patterns: []string{"加固*安全*", "启用*加密*", "enable *encryption*"},
		},
		{
			Type:     IntentQuery,
			Keywords: []string{"查看", "查询", "show", "查询状态", "多少"},
			Patterns: []string{"查看*状态*", "查询*信息*", "show *status*"},
		},
	}
	e.patterns = append(e.patterns, defaults...)
}

// RegisterHandler 注册意图处理器
func (e *Engine) RegisterHandler(intentType IntentType, handler IntentHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers[intentType] = handler
}

// ParseIntent 解析自然语言意图
func (e *Engine) ParseIntent(ctx context.Context, text string) (*Intent, error) {
	if text == "" {
		return nil, fmt.Errorf("意图文本不能为空")
	}

	intent := &Intent{
		ID:         fmt.Sprintf("intent-%d", time.Now().UnixNano()),
		RawText:    text,
		Parameters: make(map[string]string),
		Status:     StatusPending,
		CreatedAt:  time.Now(),
		Actions:    make([]Action, 0),
	}

	// 解析意图类型
	intent.Type, intent.Confidence = e.matchIntent(text)
	intent.Parameters = e.extractParameters(text)

	e.mu.Lock()
	e.intents[intent.ID] = intent
	e.totalParsed++
	e.cleanupOldIntents()
	e.mu.Unlock()

	return intent, nil
}

// matchIntent 匹配意图类型
func (e *Engine) matchIntent(text string) (IntentType, float64) {
	lower := strings.ToLower(text)
	bestType := IntentUnknown
	bestScore := 0.0

	for _, p := range e.patterns {
		score := 0.0
		matched := 0
		for _, kw := range p.Keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				matched++
			}
		}
		if matched > 0 {
			score = float64(matched) / float64(len(p.Keywords))
			if score > bestScore {
				bestScore = score
				bestType = p.Type
			}
		}
	}

	if bestScore < 0.1 {
		return IntentUnknown, 0.0
	}
	return bestType, bestScore
}

// extractParameters 提取参数
func (e *Engine) extractParameters(text string) map[string]string {
	params := make(map[string]string)
	
	// 提取容量信息
	capacityWords := []string{"TB", "GB", "MB", "PB"}
	for _, unit := range capacityWords {
		idx := strings.Index(strings.ToUpper(text), unit)
		if idx > 0 {
			// 向前查找数字
			start := idx - 1
			for start >= 0 && (text[start] >= '0' && text[start] <= '9' || text[start] == '.') {
				start--
			}
			if start < idx-1 {
				params["capacity"] = text[start+1:idx] + unit
			}
		}
	}

	// 提取路径信息
	if idx := strings.Index(text, "/"); idx >= 0 {
		end := idx
		for end < len(text) && text[end] != ' ' && text[end] != ',' && text[end] != '.' {
			end++
		}
		params["path"] = text[idx:end]
	}

	// 提取用户名
	userKeywords := []string{"用户", "user", "给"}
	for _, kw := range userKeywords {
		idx := strings.Index(strings.ToLower(text), strings.ToLower(kw))
		if idx >= 0 {
			after := text[idx+len(kw):]
			after = strings.TrimSpace(after)
			end := 0
			for end < len(after) && after[end] != ' ' && after[end] != ',' {
				end++
			}
			if end > 0 {
				params["user"] = after[:end]
			}
		}
	}

	// 提取频率
	freqKeywords := map[string]string{
		"每天": "daily", "每日": "daily", "daily": "daily",
		"每周": "weekly", "weekly": "weekly",
		"每月": "monthly", "monthly": "monthly",
		"每小时": "hourly", "hourly": "hourly",
	}
	for cn, en := range freqKeywords {
		if strings.Contains(strings.ToLower(text), strings.ToLower(cn)) {
			params["frequency"] = en
		}
	}

	return params
}

// ExecuteIntent 执行意图
func (e *Engine) ExecuteIntent(ctx context.Context, intentID string) (*Intent, error) {
	e.mu.Lock()
	intent, ok := e.intents[intentID]
	if !ok {
		e.mu.Unlock()
		return nil, fmt.Errorf("意图 %s 不存在", intentID)
	}
	intent.Status = StatusExecuting
	e.mu.Unlock()

	// 查找处理器
	e.mu.RLock()
	handler, hasHandler := e.handlers[intent.Type]
	e.mu.RUnlock()

	if !hasHandler {
		intent.Status = StatusFailed
		intent.Error = fmt.Sprintf("未找到意图类型 %s 的处理器", intent.Type)
		e.mu.Lock()
		e.totalFailed++
		e.mu.Unlock()
		return intent, fmt.Errorf("%s", intent.Error)
	}

	// 执行处理
	startTime := time.Now()
	action := Action{
		Type:       string(intent.Type),
		Target:     intent.Parameters["path"],
		Parameters: intent.Parameters,
		Status:     "executing",
		StartedAt:  startTime,
	}
	intent.Actions = append(intent.Actions, action)

	err := handler(ctx, intent)
	endTime := time.Now()
	intent.Actions[len(intent.Actions)-1].EndedAt = &endTime

	if err != nil {
		intent.Status = StatusFailed
		intent.Error = err.Error()
		intent.Actions[len(intent.Actions)-1].Status = "failed"
		intent.Actions[len(intent.Actions)-1].Error = err.Error()
		e.mu.Lock()
		e.totalFailed++
		e.mu.Unlock()
		return intent, err
	}

	intent.Status = StatusCompleted
	intent.Result = "意图执行成功"
	now := time.Now()
	intent.CompletedAt = &now
	intent.Actions[len(intent.Actions)-1].Status = "completed"
	intent.Actions[len(intent.Actions)-1].Result = "成功"

	e.mu.Lock()
	e.totalExec++
	e.mu.Unlock()

	return intent, nil
}

// GetIntent 获取意图
func (e *Engine) GetIntent(intentID string) (*Intent, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	intent, ok := e.intents[intentID]
	if !ok {
		return nil, fmt.Errorf("意图 %s 不存在", intentID)
	}
	return intent, nil
}

// ListIntents 列出意图
func (e *Engine) ListIntents(limit int) []*Intent {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	intents := make([]*Intent, 0)
	for _, intent := range e.intents {
		intents = append(intents, intent)
		if limit > 0 && len(intents) >= limit {
			break
		}
	}
	return intents
}

// GetStats 获取统计信息
func (e *Engine) GetStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return map[string]interface{}{
		"total_intents":  len(e.intents),
		"total_parsed":   e.totalParsed,
		"total_executed": e.totalExec,
		"total_failed":   e.totalFailed,
		"handlers":       len(e.handlers),
		"patterns":       len(e.patterns),
	}
}

// cleanupOldIntents 清理旧意图
func (e *Engine) cleanupOldIntents() {
	if len(e.intents) <= e.maxHistory {
		return
	}
	// 删除最旧的意图
	oldest := ""
	oldestTime := time.Now()
	for id, intent := range e.intents {
		if intent.CreatedAt.Before(oldestTime) {
			oldestTime = intent.CreatedAt
			oldest = id
		}
	}
	if oldest != "" {
		delete(e.intents, oldest)
	}
}

// CancelIntent 取消意图
func (e *Engine) CancelIntent(intentID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	intent, ok := e.intents[intentID]
	if !ok {
		return fmt.Errorf("意图 %s 不存在", intentID)
	}
	if intent.Status == StatusCompleted || intent.Status == StatusFailed {
		return fmt.Errorf("意图 %s 已完成，无法取消", intentID)
	}
	intent.Status = StatusCancelled
	return nil
}
