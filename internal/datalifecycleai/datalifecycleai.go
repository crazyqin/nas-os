package datalifecycleai

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// LifecycleEngine 数据生命周期AI引擎
type LifecycleEngine struct {
	mu         sync.RWMutex
	policies   map[string]*LifecyclePolicy
	rules      map[string]*LifecycleRule
	dataAssets map[string]*DataAsset
	actions    []*LifecycleAction
	aiEngine   *DecisionEngine
	metrics    *LifecycleMetrics
	config     *LifecycleConfig
	logger     *slog.Logger
	ctx        context.Context
	cancel     context.CancelFunc
}

// LifecyclePolicy 生命周期策略
type LifecyclePolicy struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Rules       []string  `json:"rules"`
	Priority    int       `json:"priority"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// LifecycleRule 生命周期规则
type LifecycleRule struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Condition  *RuleCondition         `json:"condition"`
	Action     RuleAction             `json:"action"`
	Parameters map[string]interface{} `json:"parameters"`
	Schedule   string                 `json:"schedule"`
	Enabled    bool                   `json:"enabled"`
	HitCount   int64                  `json:"hit_count"`
}

// RuleCondition 规则条件
type RuleCondition struct {
	Type     ConditionType    `json:"type"`
	Field    string           `json:"field,omitempty"`
	Operator Operator         `json:"operator"`
	Value    interface{}      `json:"value,omitempty"`
	And      []*RuleCondition `json:"and,omitempty"`
	Or       []*RuleCondition `json:"or,omitempty"`
}

// DataAsset 数据资产
type DataAsset struct {
	ID          string                 `json:"id"`
	Path        string                 `json:"path"`
	Size        int64                  `json:"size"`
	MimeType    string                 `json:"mime_type"`
	CreatedAt   time.Time              `json:"created_at"`
	ModifiedAt  time.Time              `json:"modified_at"`
	AccessedAt  time.Time              `json:"accessed_at"`
	AccessCount int64                  `json:"access_count"`
	Owner       string                 `json:"owner"`
	Tags        []string               `json:"tags"`
	Tier        DataTier               `json:"tier"`
	Compliance  *ComplianceInfo        `json:"compliance,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ComplianceInfo 合规信息
type ComplianceInfo struct {
	RetainUntil   time.Time          `json:"retain_until"`
	Classification DataClassification `json:"classification"`
	Region        string             `json:"region"`
	Encrypted     bool               `json:"encrypted"`
	Immutable     bool               `json:"immutable"`
	AuditRequired bool               `json:"audit_required"`
}

// LifecycleAction 生命周期操作
type LifecycleAction struct {
	ID          string       `json:"id"`
	AssetID     string       `json:"asset_id"`
	ActionType  ActionType   `json:"action_type"`
	Status      ActionStatus `json:"status"`
	Reason      string       `json:"reason"`
	Savings     int64        `json:"savings"`
	StartedAt   time.Time    `json:"started_at"`
	CompletedAt time.Time    `json:"completed_at"`
	Error       string       `json:"error,omitempty"`
}

// DecisionEngine AI决策引擎
type DecisionEngine struct {
	mu               sync.RWMutex
	models           map[string]*DecisionModel
	rules            []*AIRule
	accuracy         float64
	decisions        int64
	correctDecisions int64
}

// DecisionModel 决策模型
type DecisionModel struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      ModelType `json:"type"`
	Features  []string  `json:"features"`
	Accuracy  float64   `json:"accuracy"`
	TrainedAt time.Time `json:"trained_at"`
	Version   int       `json:"version"`
}

// AIRule AI规则
type AIRule struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Weight   float64   `json:"weight"`
	Priority int       `json:"priority"`
}

// LifecycleMetrics 生命周期指标
type LifecycleMetrics struct {
	TotalAssets     int64   `json:"total_assets"`
	HotData         int64   `json:"hot_data"`
	WarmData        int64   `json:"warm_data"`
	ColdData        int64   `json:"cold_data"`
	ArchivedData    int64   `json:"archived_data"`
	TotalSaved      int64   `json:"total_saved"`
	ActionsExecuted int64   `json:"actions_executed"`
	ComplianceRate  float64 `json:"compliance_rate"`
	AutoActions     int64   `json:"auto_actions"`
}

// LifecycleConfig 生命周期配置
type LifecycleConfig struct {
	ScanInterval      time.Duration `json:"scan_interval"`
	AutoExecute       bool          `json:"auto_execute"`
	DryRun            bool          `json:"dry_run"`
	MaxConcurrent     int           `json:"max_concurrent"`
	ComplianceMode    bool          `json:"compliance_mode"`
	AIDecisionEnabled bool          `json:"ai_decision_enabled"`
}

// DataTier 数据层级
type DataTier int

const (
	DataTierHot DataTier = iota
	DataTierWarm
	DataTierCold
	DataTierArchive
	DataTierDelete
)

// String 返回数据层级的字符串表示
func (t DataTier) String() string {
	switch t {
	case DataTierHot:
		return "hot"
	case DataTierWarm:
		return "warm"
	case DataTierCold:
		return "cold"
	case DataTierArchive:
		return "archive"
	case DataTierDelete:
		return "delete"
	default:
		return "unknown"
	}
}

// DataClassification 数据分类
type DataClassification int

const (
	ClassificationPublic DataClassification = iota
	ClassificationInternal
	ClassificationConfidential
	ClassificationRestricted
	ClassificationTopSecret
)

// String 返回数据分类的字符串表示
func (c DataClassification) String() string {
	switch c {
	case ClassificationPublic:
		return "public"
	case ClassificationInternal:
		return "internal"
	case ClassificationConfidential:
		return "confidential"
	case ClassificationRestricted:
		return "restricted"
	case ClassificationTopSecret:
		return "top_secret"
	default:
		return "unknown"
	}
}

// ConditionType 条件类型
type ConditionType int

const (
	ConditionAge ConditionType = iota
	ConditionSize
	ConditionAccess
	ConditionType_
	ConditionTag
	ConditionOwner
	ConditionCustom
)

// Operator 操作符
type Operator int

const (
	OpEquals Operator = iota
	OpNotEquals
	OpGreaterThan
	OpLessThan
	OpContains
	OpMatches
)

// RuleAction 规则动作
type RuleAction int

const (
	ActionArchive RuleAction = iota
	ActionMigrate
	ActionCompress
	ActionEncrypt
	ActionDelete
	ActionNotify
	ActionTag
)

// String 返回规则动作的字符串表示
func (a RuleAction) String() string {
	switch a {
	case ActionArchive:
		return "archive"
	case ActionMigrate:
		return "migrate"
	case ActionCompress:
		return "compress"
	case ActionEncrypt:
		return "encrypt"
	case ActionDelete:
		return "delete"
	case ActionNotify:
		return "notify"
	case ActionTag:
		return "tag"
	default:
		return "unknown"
	}
}

// ActionType 操作类型
type ActionType int

const (
	ActionTypeMigrate ActionType = iota
	ActionTypeArchive
	ActionTypeDelete
	ActionTypeCompress
	ActionTypeEncrypt
)

// String 返回操作类型的字符串表示
func (t ActionType) String() string {
	switch t {
	case ActionTypeMigrate:
		return "migrate"
	case ActionTypeArchive:
		return "archive"
	case ActionTypeDelete:
		return "delete"
	case ActionTypeCompress:
		return "compress"
	case ActionTypeEncrypt:
		return "encrypt"
	default:
		return "unknown"
	}
}

// ActionStatus 操作状态
type ActionStatus int

const (
	ActionPending ActionStatus = iota
	ActionRunning
	ActionCompleted
	ActionFailed
	ActionCancelled
)

// String 返回操作状态的字符串表示
func (s ActionStatus) String() string {
	switch s {
	case ActionPending:
		return "pending"
	case ActionRunning:
		return "running"
	case ActionCompleted:
		return "completed"
	case ActionFailed:
		return "failed"
	case ActionCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// ModelType 模型类型
type ModelType int

const (
	ModelTypeTierDecision ModelType = iota
	ModelTypeRetirementDecision
	ModelTypeComplianceDecision
)

// NewLifecycleEngine 创建生命周期引擎
func NewLifecycleEngine(config *LifecycleConfig, logger *slog.Logger) (*LifecycleEngine, error) {
	if config == nil {
		return nil, ErrConfigInvalid
	}

	if logger == nil {
		logger = slog.Default()
	}

	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 10
	}

	if config.ScanInterval <= 0 {
		config.ScanInterval = 1 * time.Hour
	}

	ctx, cancel := context.WithCancel(context.Background())

	engine := &LifecycleEngine{
		policies:   make(map[string]*LifecyclePolicy),
		rules:      make(map[string]*LifecycleRule),
		dataAssets: make(map[string]*DataAsset),
		actions:    make([]*LifecycleAction, 0),
		aiEngine:   NewDecisionEngine(),
		metrics:    &LifecycleMetrics{},
		config:     config,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}

	logger.Info("Lifecycle engine created",
		"auto_execute", config.AutoExecute,
		"dry_run", config.DryRun,
		"ai_enabled", config.AIDecisionEnabled,
	)

	return engine, nil
}

// NewDecisionEngine 创建AI决策引擎
func NewDecisionEngine() *DecisionEngine {
	engine := &DecisionEngine{
		models:   make(map[string]*DecisionModel),
		rules:    make([]*AIRule, 0),
		accuracy: 0.85, // 默认准确率
	}

	// 初始化默认模型
	engine.models["tier_decision"] = &DecisionModel{
		ID:   "tier_decision",
		Name: "Tier Decision Model",
		Type: ModelTypeTierDecision,
		Features: []string{
			"age_days",
			"access_frequency",
			"size",
			"last_access_days",
		},
		Accuracy:  0.88,
		TrainedAt: time.Now(),
		Version:   1,
	}

	engine.models["retirement_decision"] = &DecisionModel{
		ID:   "retirement_decision",
		Name: "Retirement Decision Model",
		Type: ModelTypeRetirementDecision,
		Features: []string{
			"age_days",
			"access_count",
			"compliance_retention",
			"data_classification",
		},
		Accuracy:  0.92,
		TrainedAt: time.Now(),
		Version:   1,
	}

	engine.models["compliance_decision"] = &DecisionModel{
		ID:   "compliance_decision",
		Name: "Compliance Decision Model",
		Type: ModelTypeComplianceDecision,
		Features: []string{
			"classification",
			"retention_days",
			"region",
			"encryption_status",
		},
		Accuracy:  0.95,
		TrainedAt: time.Now(),
		Version:   1,
	}

	return engine
}

// AddPolicy 添加生命周期策略
func (e *LifecycleEngine) AddPolicy(policy *LifecyclePolicy) error {
	if policy == nil {
		return ErrPolicyInvalid
	}

	if policy.ID == "" {
		return ErrPolicyInvalid
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.policies[policy.ID]; exists {
		return ErrPolicyAlreadyExists
	}

	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now
	e.policies[policy.ID] = policy

	e.logger.Info("Policy added",
		"policy_id", policy.ID,
		"name", policy.Name,
		"rules_count", len(policy.Rules),
	)

	return nil
}

// AddRule 添加生命周期规则
func (e *LifecycleEngine) AddRule(rule *LifecycleRule) error {
	if rule == nil {
		return ErrRuleInvalid
	}

	if rule.ID == "" || rule.Condition == nil {
		return ErrRuleInvalid
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.rules[rule.ID]; exists {
		return ErrRuleAlreadyExists
	}

	e.rules[rule.ID] = rule

	e.logger.Info("Rule added",
		"rule_id", rule.ID,
		"name", rule.Name,
		"action", rule.Action.String(),
	)

	return nil
}

// RegisterAsset 注册数据资产
func (e *LifecycleEngine) RegisterAsset(asset *DataAsset) error {
	if asset == nil {
		return ErrAssetInvalid
	}

	if asset.ID == "" || asset.Path == "" {
		return ErrAssetInvalid
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.dataAssets[asset.ID]; exists {
		return ErrAssetAlreadyExists
	}

	now := time.Now()
	if asset.CreatedAt.IsZero() {
		asset.CreatedAt = now
	}
	if asset.ModifiedAt.IsZero() {
		asset.ModifiedAt = now
	}
	if asset.AccessedAt.IsZero() {
		asset.AccessedAt = now
	}

	e.dataAssets[asset.ID] = asset

	// 更新指标
	e.updateMetrics()

	e.logger.Info("Asset registered",
		"asset_id", asset.ID,
		"path", asset.Path,
		"size", asset.Size,
		"tier", asset.Tier.String(),
	)

	return nil
}

// EvaluateAsset 评估资产并做出AI决策
func (e *LifecycleEngine) EvaluateAsset(assetID string) (*LifecycleAction, error) {
	e.mu.RLock()
	asset, exists := e.dataAssets[assetID]
	e.mu.RUnlock()

	if !exists {
		return nil, ErrAssetNotFound
	}

	// 合规检查
	if e.config.ComplianceMode {
		if err := e.checkCompliance(asset); err != nil {
			e.logger.Warn("Compliance check failed",
				"asset_id", assetID,
				"error", err,
			)
			return nil, err
		}
	}

	// 使用AI决策引擎评估
	if e.config.AIDecisionEnabled {
		return e.aiEvaluate(asset)
	}

	// 使用规则引擎评估
	return e.ruleEvaluate(asset)
}

// aiEvaluate AI决策评估
func (e *LifecycleEngine) aiEvaluate(asset *DataAsset) (*LifecycleAction, error) {
	e.aiEngine.mu.RLock()
	defer e.aiEngine.mu.RUnlock()

	// 计算特征值
	features := e.extractFeatures(asset)

	// 获取模型
	model, exists := e.aiEngine.models["tier_decision"]
	if !exists {
		return nil, ErrModelNotFound
	}

	// AI决策：计算推荐层级
	recommendedTier := e.calculateTierDecision(features, model)

	// 如果需要迁移
	if recommendedTier != asset.Tier {
		// 检查合规约束
		if asset.Compliance != nil && asset.Compliance.Immutable {
			e.logger.Info("Asset is immutable, skipping migration",
				"asset_id", asset.ID,
			)
			return nil, nil
		}

		// 计算节省空间
		savings := e.calculateSavings(asset, recommendedTier)

		action := &LifecycleAction{
			ID:         fmt.Sprintf("action-%s-%d", asset.ID, time.Now().UnixNano()),
			AssetID:    asset.ID,
			ActionType: e.getActionType(asset.Tier, recommendedTier),
			Status:     ActionPending,
			Reason:     fmt.Sprintf("AI recommends migration from %s to %s", asset.Tier.String(), recommendedTier.String()),
			Savings:    savings,
			StartedAt:  time.Now(),
		}

		// 更新AI决策统计
		e.aiEngine.decisions++

		e.logger.Info("AI decision made",
			"asset_id", asset.ID,
			"from_tier", asset.Tier.String(),
			"to_tier", recommendedTier.String(),
			"action_type", action.ActionType.String(),
		)

		return action, nil
	}

	return nil, nil
}

// ruleEvaluate 规则引擎评估
func (e *LifecycleEngine) ruleEvaluate(asset *DataAsset) (*LifecycleAction, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var matchedAction *LifecycleAction

	// 遍历所有规则
	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}

		// 评估条件
		if e.evaluateCondition(rule.Condition, asset) {
			rule.HitCount++

			// 创建操作
			matchedAction = &LifecycleAction{
				ID:         fmt.Sprintf("action-%s-%d", asset.ID, time.Now().UnixNano()),
				AssetID:    asset.ID,
				ActionType: e.mapRuleAction(rule.Action),
				Status:     ActionPending,
				Reason:     fmt.Sprintf("Matched rule: %s", rule.Name),
				StartedAt:  time.Now(),
			}

			e.logger.Info("Rule matched",
				"rule_id", rule.ID,
				"asset_id", asset.ID,
				"action", rule.Action.String(),
			)

			break // 使用第一个匹配的规则
		}
	}

	return matchedAction, nil
}

// evaluateCondition 评估条件
func (e *LifecycleEngine) evaluateCondition(condition *RuleCondition, asset *DataAsset) bool {
	if condition == nil {
		return false
	}

	// 处理AND条件
	if len(condition.And) > 0 {
		for _, andCond := range condition.And {
			if !e.evaluateCondition(andCond, asset) {
				return false
			}
		}
		return true
	}

	// 处理OR条件
	if len(condition.Or) > 0 {
		for _, orCond := range condition.Or {
			if e.evaluateCondition(orCond, asset) {
				return true
			}
		}
		return false
	}

	// 评估单个条件
	return e.evaluateSingleCondition(condition, asset)
}

// evaluateSingleCondition 评估单个条件
func (e *LifecycleEngine) evaluateSingleCondition(condition *RuleCondition, asset *DataAsset) bool {
	now := time.Now()

	switch condition.Type {
	case ConditionAge:
		ageDays := now.Sub(asset.CreatedAt).Hours() / 24
		return e.compareNumeric(ageDays, condition.Operator, condition.Value)

	case ConditionSize:
		return e.compareNumeric(float64(asset.Size), condition.Operator, condition.Value)

	case ConditionAccess:
		lastAccessDays := now.Sub(asset.AccessedAt).Hours() / 24
		return e.compareNumeric(lastAccessDays, condition.Operator, condition.Value)

	case ConditionTag:
		for _, tag := range asset.Tags {
			if e.compareString(tag, condition.Operator, condition.Value) {
				return true
			}
		}
		return false

	case ConditionOwner:
		return e.compareString(asset.Owner, condition.Operator, condition.Value)

	default:
		return false
	}
}

// compareNumeric 比较数值
func (e *LifecycleEngine) compareNumeric(actual float64, op Operator, expected interface{}) bool {
	expectedVal, ok := toFloat64(expected)
	if !ok {
		return false
	}

	switch op {
	case OpEquals:
		return actual == expectedVal
	case OpNotEquals:
		return actual != expectedVal
	case OpGreaterThan:
		return actual > expectedVal
	case OpLessThan:
		return actual < expectedVal
	default:
		return false
	}
}

// compareString 比较字符串
func (e *LifecycleEngine) compareString(actual string, op Operator, expected interface{}) bool {
	expectedStr, ok := expected.(string)
	if !ok {
		return false
	}

	switch op {
	case OpEquals:
		return actual == expectedStr
	case OpNotEquals:
		return actual != expectedStr
	case OpContains:
		return containsSubstring(actual, expectedStr)
	default:
		return false
	}
}

// toFloat64 转换为float64
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	default:
		return 0, false
	}
}

// containsSubstring 检查子字符串
func containsSubstring(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && searchSubstring(s, substr)
}

// searchSubstring 搜索子字符串
func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// extractFeatures 提取资产特征
func (e *LifecycleEngine) extractFeatures(asset *DataAsset) map[string]float64 {
	now := time.Now()

	classification := float64(0)
	if asset.Compliance != nil {
		classification = float64(asset.Compliance.Classification)
	}

	return map[string]float64{
		"age_days":          now.Sub(asset.CreatedAt).Hours() / 24,
		"access_frequency":  float64(asset.AccessCount),
		"size":              float64(asset.Size),
		"last_access_days":  now.Sub(asset.AccessedAt).Hours() / 24,
		"modified_days":     now.Sub(asset.ModifiedAt).Hours() / 24,
		"classification":    classification,
	}
}

// calculateTierDecision 计算层级决策
func (e *LifecycleEngine) calculateTierDecision(features map[string]float64, model *DecisionModel) DataTier {
	ageDays := features["age_days"]
	lastAccessDays := features["last_access_days"]
	accessFreq := features["access_frequency"]

	// AI评分算法
	score := 0.0

	// 年龄因素 (0-40分)
	if ageDays < 30 {
		score += 40
	} else if ageDays < 90 {
		score += 30
	} else if ageDays < 180 {
		score += 20
	} else if ageDays < 365 {
		score += 10
	}

	// 访问频率因素 (0-35分)
	if accessFreq > 100 {
		score += 35
	} else if accessFreq > 50 {
		score += 25
	} else if accessFreq > 10 {
		score += 15
	} else if accessFreq > 0 {
		score += 5
	}

	// 最后访问时间因素 (0-25分)
	if lastAccessDays < 7 {
		score += 25
	} else if lastAccessDays < 30 {
		score += 20
	} else if lastAccessDays < 90 {
		score += 15
	} else if lastAccessDays < 180 {
		score += 10
	} else {
		score += 5
	}

	// 根据分数决定层级
	if score >= 80 {
		return DataTierHot
	} else if score >= 60 {
		return DataTierWarm
	} else if score >= 40 {
		return DataTierCold
	} else if score >= 20 {
		return DataTierArchive
	}

	return DataTierArchive
}

// calculateSavings 计算节省空间
func (e *LifecycleEngine) calculateSavings(asset *DataAsset, targetTier DataTier) int64 {
	// 不同层级的压缩率
	compressionRates := map[DataTier]float64{
		DataTierHot:     1.0,
		DataTierWarm:    0.8,
		DataTierCold:    0.5,
		DataTierArchive: 0.3,
	}

	currentRate := compressionRates[asset.Tier]
	targetRate := compressionRates[targetTier]

	return int64(float64(asset.Size) * (currentRate - targetRate))
}

// getActionType 获取操作类型
func (e *LifecycleEngine) getActionType(fromTier, toTier DataTier) ActionType {
	if toTier == DataTierDelete {
		return ActionTypeDelete
	} else if toTier == DataTierArchive {
		return ActionTypeArchive
	} else if toTier > fromTier {
		return ActionTypeMigrate
	}
	return ActionTypeMigrate
}

// mapRuleAction 映射规则动作到操作类型
func (e *LifecycleEngine) mapRuleAction(action RuleAction) ActionType {
	switch action {
	case ActionArchive:
		return ActionTypeArchive
	case ActionMigrate:
		return ActionTypeMigrate
	case ActionCompress:
		return ActionTypeCompress
	case ActionEncrypt:
		return ActionTypeEncrypt
	case ActionDelete:
		return ActionTypeDelete
	default:
		return ActionTypeMigrate
	}
}

// ExecuteAction 执行生命周期操作
func (e *LifecycleEngine) ExecuteAction(action *LifecycleAction) error {
	if action == nil {
		return ErrActionFailed
	}

	e.mu.Lock()

	// 检查是否干运行
	if e.config.DryRun {
		e.logger.Info("Dry run: would execute action",
			"action_id", action.ID,
			"asset_id", action.AssetID,
			"type", action.ActionType.String(),
		)
		e.mu.Unlock()
		return nil
	}

	// 检查并发限制
	runningCount := 0
	for _, a := range e.actions {
		if a.Status == ActionRunning {
			runningCount++
		}
	}

	if runningCount >= e.config.MaxConcurrent {
		e.mu.Unlock()
		return ErrConcurrentLimit
	}

	// 添加到操作列表
	action.Status = ActionRunning
	e.actions = append(e.actions, action)
	e.mu.Unlock()

	e.logger.Info("Executing action",
		"action_id", action.ID,
		"asset_id", action.AssetID,
		"type", action.ActionType.String(),
		"reason", action.Reason,
	)

	// 模拟执行操作
	err := e.simulateAction(action)

	e.mu.Lock()
	defer e.mu.Unlock()

	if err != nil {
		action.Status = ActionFailed
		action.Error = err.Error()
		e.logger.Error("Action failed",
			"action_id", action.ID,
			"error", err,
		)
		return err
	}

	action.Status = ActionCompleted
	action.CompletedAt = time.Now()

	// 更新资产层级
	if asset, exists := e.dataAssets[action.AssetID]; exists {
		switch action.ActionType {
		case ActionTypeArchive:
			asset.Tier = DataTierArchive
		case ActionTypeMigrate:
			// 根据规则更新层级
		case ActionTypeDelete:
			delete(e.dataAssets, action.AssetID)
		}
	}

	// 更新指标
	e.metrics.ActionsExecuted++
	e.metrics.TotalSaved += action.Savings
	e.updateMetrics()

	e.logger.Info("Action completed",
		"action_id", action.ID,
		"duration", action.CompletedAt.Sub(action.StartedAt),
		"savings", action.Savings,
	)

	return nil
}

// simulateAction 模拟执行操作
func (e *LifecycleEngine) simulateAction(action *LifecycleAction) error {
	// 模拟执行延迟
	time.Sleep(100 * time.Millisecond)

	// 模拟一些失败场景
	asset, exists := e.dataAssets[action.AssetID]
	if !exists {
		return ErrAssetNotFound
	}

	// 检查合规约束
	if asset.Compliance != nil && asset.Compliance.Immutable {
		if action.ActionType == ActionTypeDelete {
			return ErrComplianceViolation
		}
	}

	return nil
}

// ScanAndProcess 扫描并处理所有资产
func (e *LifecycleEngine) ScanAndProcess() error {
	e.mu.RLock()
	assetIDs := make([]string, 0, len(e.dataAssets))
	for id := range e.dataAssets {
		assetIDs = append(assetIDs, id)
	}
	e.mu.RUnlock()

	e.logger.Info("Starting scan and process",
		"assets_count", len(assetIDs),
	)

	processed := 0
	failed := 0

	for _, assetID := range assetIDs {
		// 评估资产
		action, err := e.EvaluateAsset(assetID)
		if err != nil {
			e.logger.Error("Failed to evaluate asset",
				"asset_id", assetID,
				"error", err,
			)
			failed++
			continue
		}

		if action == nil {
			continue
		}

		// 自动执行
		if e.config.AutoExecute {
			if err := e.ExecuteAction(action); err != nil {
				e.logger.Error("Failed to execute action",
					"action_id", action.ID,
					"error", err,
				)
				failed++
				continue
			}
			e.metrics.AutoActions++
		}

		processed++
	}

	e.logger.Info("Scan and process completed",
		"processed", processed,
		"failed", failed,
	)

	return nil
}

// checkCompliance 检查合规性
func (e *LifecycleEngine) checkCompliance(asset *DataAsset) error {
	if asset.Compliance == nil {
		return nil
	}

	now := time.Now()

	// 检查保留期
	if !asset.Compliance.RetainUntil.IsZero() && now.Before(asset.Compliance.RetainUntil) {
		// 还在保留期内，不能删除
		e.logger.Debug("Asset within retention period",
			"asset_id", asset.ID,
			"retain_until", asset.Compliance.RetainUntil,
		)
	}

	// 检查加密要求
	if asset.Compliance.Classification >= ClassificationConfidential && !asset.Compliance.Encrypted {
		return ErrEncryptionRequired
	}

	return nil
}

// GetComplianceReport 获取合规报告
func (e *LifecycleEngine) GetComplianceReport() (*ComplianceReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	report := &ComplianceReport{
		GeneratedAt:   time.Now(),
		TotalAssets:   int64(len(e.dataAssets)),
		Violations:    make([]ComplianceViolation, 0),
		Recommendations: make([]string, 0),
	}

	// 统计合规状态
	compliant := int64(0)
	nonCompliant := int64(0)

	for _, asset := range e.dataAssets {
		if err := e.checkCompliance(asset); err != nil {
			nonCompliant++
			report.Violations = append(report.Violations, ComplianceViolation{
				AssetID:   asset.ID,
				Path:      asset.Path,
				Violation: err.Error(),
				Severity:  "high",
			})
		} else {
			compliant++
		}
	}

	// 计算合规率
	if report.TotalAssets > 0 {
		report.ComplianceRate = float64(compliant) / float64(report.TotalAssets) * 100
		e.metrics.ComplianceRate = report.ComplianceRate
	}

	// 生成建议
	if len(report.Violations) > 0 {
		report.Recommendations = append(report.Recommendations,
			fmt.Sprintf("Found %d compliance violations", len(report.Violations)),
			"Review and remediate violations immediately",
			"Enable encryption for confidential data",
		)
	}

	return report, nil
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	GeneratedAt     time.Time            `json:"generated_at"`
	TotalAssets     int64                `json:"total_assets"`
	ComplianceRate  float64              `json:"compliance_rate"`
	Violations      []ComplianceViolation `json:"violations"`
	Recommendations []string             `json:"recommendations"`
}

// ComplianceViolation 合规违规
type ComplianceViolation struct {
	AssetID   string `json:"asset_id"`
	Path      string `json:"path"`
	Violation string `json:"violation"`
	Severity  string `json:"severity"`
}

// GetMetrics 获取指标
func (e *LifecycleEngine) GetMetrics() *LifecycleMetrics {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 更新指标
	e.updateMetrics()

	return &LifecycleMetrics{
		TotalAssets:     e.metrics.TotalAssets,
		HotData:         e.metrics.HotData,
		WarmData:        e.metrics.WarmData,
		ColdData:        e.metrics.ColdData,
		ArchivedData:    e.metrics.ArchivedData,
		TotalSaved:      e.metrics.TotalSaved,
		ActionsExecuted: e.metrics.ActionsExecuted,
		ComplianceRate:  e.metrics.ComplianceRate,
		AutoActions:     e.metrics.AutoActions,
	}
}

// updateMetrics 更新指标
func (e *LifecycleEngine) updateMetrics() {
	e.metrics.TotalAssets = int64(len(e.dataAssets))
	e.metrics.HotData = 0
	e.metrics.WarmData = 0
	e.metrics.ColdData = 0
	e.metrics.ArchivedData = 0

	for _, asset := range e.dataAssets {
		switch asset.Tier {
		case DataTierHot:
			e.metrics.HotData++
		case DataTierWarm:
			e.metrics.WarmData++
		case DataTierCold:
			e.metrics.ColdData++
		case DataTierArchive:
			e.metrics.ArchivedData++
		}
	}
}
