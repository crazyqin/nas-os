package monitor

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// === AlertRuleEngine 基础测试 ===

func TestNewAlertRuleEngine(t *testing.T) {
	alertMgr := NewAlertingManager()
	engine := NewAlertRuleEngine("", alertMgr)

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.rules)
	assert.NotNil(t, engine.alertManager)
}

func TestNewAlertRuleEngine_WithConfigPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "alert-engine-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "rules.json")
	alertMgr := NewAlertingManager()
	engine := NewAlertRuleEngine(configPath, alertMgr)

	assert.NotNil(t, engine)
	assert.Equal(t, configPath, engine.configPath)
}

func TestAlertRuleEngine_AddRule(t *testing.T) {
	// 创建没有默认规则的引擎
	engine := &AlertRuleEngine{
		rules:        make(map[string]*AlertRuleConfig),
		alertManager: NewAlertingManager(),
	}

	rule := &AlertRuleConfig{
		ID:      "test-rule-1",
		Name:    "Test Rule",
		Enabled: true,
		Type:    RuleTypeCPU,
		Conditions: []RuleCondition{
			{Field: "cpu_usage", Operator: OpGreaterThan, Value: 80},
		},
		Logic: LogicAnd,
		Level: "warning",
	}

	err := engine.AddRule(rule)
	assert.NoError(t, err)

	rules := engine.GetRules()
	assert.Len(t, rules, 1)
}

func TestAlertRuleEngine_AddRule_Duplicate(t *testing.T) {
	// 创建没有默认规则的引擎
	engine := &AlertRuleEngine{
		rules:        make(map[string]*AlertRuleConfig),
		alertManager: NewAlertingManager(),
	}

	rule := &AlertRuleConfig{
		ID:      "test-rule-1",
		Name:    "Test Rule",
		Enabled: true,
		Type:    RuleTypeCPU,
	}

	_ = engine.AddRule(rule)
	// 添加相同 ID 的规则会覆盖之前的
	err := engine.AddRule(rule)
	assert.NoError(t, err) // AddRule 允许覆盖
}

func TestAlertRuleEngine_UpdateRule(t *testing.T) {
	engine := NewAlertRuleEngine("", NewAlertingManager())

	rule := &AlertRuleConfig{
		ID:      "test-rule-1",
		Name:    "Test Rule",
		Enabled: true,
		Type:    RuleTypeCPU,
		Level:   "warning",
	}
	_ = engine.AddRule(rule)

	// 更新规则
	updated := &AlertRuleConfig{
		ID:      "test-rule-1",
		Name:    "Updated Rule",
		Enabled: false,
		Type:    RuleTypeMemory,
		Level:   "critical",
	}

	err := engine.UpdateRule(updated)
	assert.NoError(t, err)

	retrieved, err := engine.GetRule("test-rule-1")
	require.NoError(t, err)
	assert.Equal(t, "Updated Rule", retrieved.Name)
	assert.False(t, retrieved.Enabled)
	assert.Equal(t, "critical", retrieved.Level)
}

func TestAlertRuleEngine_UpdateRule_NotFound(t *testing.T) {
	engine := NewAlertRuleEngine("", NewAlertingManager())

	err := engine.UpdateRule(&AlertRuleConfig{ID: "nonexistent"})
	assert.Error(t, err)
}

func TestAlertRuleEngine_DeleteRule(t *testing.T) {
	engine := NewAlertRuleEngine("", NewAlertingManager())

	rule := &AlertRuleConfig{
		ID:      "test-rule-1",
		Name:    "Test Rule",
		Enabled: true,
		Type:    RuleTypeCPU,
	}
	_ = engine.AddRule(rule)

	err := engine.DeleteRule("test-rule-1")
	assert.NoError(t, err)

	_, err = engine.GetRule("test-rule-1")
	assert.Error(t, err)
}

func TestAlertRuleEngine_DeleteRule_NotFound(t *testing.T) {
	engine := NewAlertRuleEngine("", NewAlertingManager())

	err := engine.DeleteRule("nonexistent")
	assert.Error(t, err)
}

func TestAlertRuleEngine_GetRule(t *testing.T) {
	engine := NewAlertRuleEngine("", NewAlertingManager())

	rule := &AlertRuleConfig{
		ID:      "test-rule-1",
		Name:    "Test Rule",
		Enabled: true,
		Type:    RuleTypeCPU,
	}
	_ = engine.AddRule(rule)

	retrieved, err := engine.GetRule("test-rule-1")
	require.NoError(t, err)
	assert.Equal(t, "test-rule-1", retrieved.ID)
	assert.Equal(t, "Test Rule", retrieved.Name)
}

func TestAlertRuleEngine_GetRule_NotFound(t *testing.T) {
	engine := NewAlertRuleEngine("", NewAlertingManager())

	_, err := engine.GetRule("nonexistent")
	assert.Error(t, err)
}

func TestAlertRuleEngine_GetRules(t *testing.T) {
	engine := NewAlertRuleEngine("", NewAlertingManager())

	// 初始规则（包含默认规则）
	initialCount := len(engine.GetRules())

	// 添加规则
	_ = engine.AddRule(&AlertRuleConfig{ID: "rule-1", Type: RuleTypeCPU})
	_ = engine.AddRule(&AlertRuleConfig{ID: "rule-2", Type: RuleTypeMemory})

	rules := engine.GetRules()
	assert.Equal(t, initialCount+2, len(rules))
}

func TestAlertRuleEngine_GetRulesByType(t *testing.T) {
	engine := NewAlertRuleEngine("", NewAlertingManager())

	_ = engine.AddRule(&AlertRuleConfig{ID: "cpu-1", Type: RuleTypeCPU})
	_ = engine.AddRule(&AlertRuleConfig{ID: "cpu-2", Type: RuleTypeCPU})
	_ = engine.AddRule(&AlertRuleConfig{ID: "mem-1", Type: RuleTypeMemory})

	cpuRules := engine.GetRulesByType(RuleTypeCPU)
	assert.GreaterOrEqual(t, len(cpuRules), 2)

	memRules := engine.GetRulesByType(RuleTypeMemory)
	assert.GreaterOrEqual(t, len(memRules), 1)
}

func TestAlertRuleEngine_EnableRule(t *testing.T) {
	engine := NewAlertRuleEngine("", NewAlertingManager())

	rule := &AlertRuleConfig{
		ID:      "test-rule-1",
		Name:    "Test Rule",
		Enabled: false,
		Type:    RuleTypeCPU,
	}
	_ = engine.AddRule(rule)

	err := engine.EnableRule("test-rule-1")
	assert.NoError(t, err)

	retrieved, _ := engine.GetRule("test-rule-1")
	assert.True(t, retrieved.Enabled)
}

func TestAlertRuleEngine_DisableRule(t *testing.T) {
	engine := NewAlertRuleEngine("", NewAlertingManager())

	rule := &AlertRuleConfig{
		ID:      "test-rule-1",
		Name:    "Test Rule",
		Enabled: true,
		Type:    RuleTypeCPU,
	}
	_ = engine.AddRule(rule)

	err := engine.DisableRule("test-rule-1")
	assert.NoError(t, err)

	retrieved, _ := engine.GetRule("test-rule-1")
	assert.False(t, retrieved.Enabled)
}

// === 规则评估测试 ===

func TestAlertRuleEngine_Evaluate(t *testing.T) {
	alertMgr := NewAlertingManager()
	engine := NewAlertRuleEngine("", alertMgr)

	// 添加测试规则
	rule := &AlertRuleConfig{
		ID:      "cpu-high",
		Name:    "CPU High",
		Enabled: true,
		Type:    RuleTypeCPU,
		Conditions: []RuleCondition{
			{Field: "cpu_usage", Operator: OpGreaterThan, Value: 80},
		},
		Logic: LogicAnd,
		Level: "warning",
	}
	_ = engine.AddRule(rule)

	// 评估指标
	metrics := map[string]float64{
		"cpu_usage": 90,
	}

	alerts := engine.Evaluate(metrics, nil)
	assert.GreaterOrEqual(t, len(alerts), 1)
}

func TestAlertRuleEngine_Evaluate_DisabledRule(t *testing.T) {
	alertMgr := NewAlertingManager()
	// 创建没有默认规则的引擎
	engine := &AlertRuleEngine{
		rules:        make(map[string]*AlertRuleConfig),
		alertManager: alertMgr,
	}

	// 添加禁用的规则
	rule := &AlertRuleConfig{
		ID:      "cpu-high",
		Name:    "CPU High",
		Enabled: false,
		Type:    RuleTypeCPU,
		Conditions: []RuleCondition{
			{Field: "cpu_usage", Operator: OpGreaterThan, Value: 80},
		},
	}
	_ = engine.AddRule(rule)

	metrics := map[string]float64{
		"cpu_usage": 90,
	}

	alerts := engine.Evaluate(metrics, nil)
	assert.Len(t, alerts, 0)
}

func TestAlertRuleEngine_Evaluate_MultipleConditions(t *testing.T) {
	alertMgr := NewAlertingManager()
	// 创建没有默认规则的引擎
	engine := &AlertRuleEngine{
		rules:        make(map[string]*AlertRuleConfig),
		alertManager: alertMgr,
	}

	// AND 逻辑：所有条件都要满足
	rule := &AlertRuleConfig{
		ID:      "cpu-and-mem",
		Name:    "CPU and Memory High",
		Enabled: true,
		Type:    RuleTypeCPU,
		Conditions: []RuleCondition{
			{Field: "cpu_usage", Operator: OpGreaterThan, Value: 80},
			{Field: "memory_usage", Operator: OpGreaterThan, Value: 80},
		},
		Logic: LogicAnd,
		Level: "critical",
	}
	_ = engine.AddRule(rule)

	// 只有 CPU 高
	metrics1 := map[string]float64{
		"cpu_usage":    90,
		"memory_usage": 50,
	}
	alerts := engine.Evaluate(metrics1, nil)
	assert.Len(t, alerts, 0) // AND 逻辑，不满足

	// 两者都高
	metrics2 := map[string]float64{
		"cpu_usage":    90,
		"memory_usage": 85,
	}
	alerts = engine.Evaluate(metrics2, nil)
	assert.GreaterOrEqual(t, len(alerts), 1)
}

func TestAlertRuleEngine_Evaluate_OrLogic(t *testing.T) {
	alertMgr := NewAlertingManager()
	engine := NewAlertRuleEngine("", alertMgr)

	// OR 逻辑：任一条件满足
	rule := &AlertRuleConfig{
		ID:      "cpu-or-mem",
		Name:    "CPU or Memory High",
		Enabled: true,
		Type:    RuleTypeCPU,
		Conditions: []RuleCondition{
			{Field: "cpu_usage", Operator: OpGreaterThan, Value: 80},
			{Field: "memory_usage", Operator: OpGreaterThan, Value: 80},
		},
		Logic: LogicOr,
		Level: "warning",
	}
	_ = engine.AddRule(rule)

	// 只有 CPU 高
	metrics := map[string]float64{
		"cpu_usage":    90,
		"memory_usage": 50,
	}
	alerts := engine.Evaluate(metrics, nil)
	assert.GreaterOrEqual(t, len(alerts), 1) // OR 逻辑，满足
}

// === 持久化测试 ===

func TestAlertRuleEngine_SaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "alert-engine-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "rules.json")

	// 创建引擎并添加规则
	engine1 := NewAlertRuleEngine(configPath, NewAlertingManager())
	rule := &AlertRuleConfig{
		ID:      "test-rule-1",
		Name:    "Test Rule",
		Enabled: true,
		Type:    RuleTypeCPU,
		Conditions: []RuleCondition{
			{Field: "cpu_usage", Operator: OpGreaterThan, Value: 80},
		},
	}
	_ = engine1.AddRule(rule)

	// 保存
	err = engine1.saveRules()
	require.NoError(t, err)

	// 加载到新引擎
	engine2 := NewAlertRuleEngine(configPath, NewAlertingManager())
	rules := engine2.GetRules()

	found := false
	for _, r := range rules {
		if r.ID == "test-rule-1" {
			found = true
			assert.Equal(t, "Test Rule", r.Name)
			break
		}
	}
	assert.True(t, found)
}

func TestAlertRuleEngine_LoadRules_Nonexistent(t *testing.T) {
	engine := NewAlertRuleEngine("/nonexistent/path/rules.json", NewAlertingManager())
	// 不应该报错
	assert.NotNil(t, engine)
}

// === 统计测试 ===

func TestAlertRuleEngine_GetRuleStats(t *testing.T) {
	engine := NewAlertRuleEngine("", NewAlertingManager())

	_ = engine.AddRule(&AlertRuleConfig{ID: "rule-1", Enabled: true, Type: RuleTypeCPU})
	_ = engine.AddRule(&AlertRuleConfig{ID: "rule-2", Enabled: false, Type: RuleTypeMemory})

	stats := engine.GetRuleStats()

	total, ok := stats["total_rules"].(int)
	assert.True(t, ok)
	assert.GreaterOrEqual(t, total, 2)

	enabled, ok := stats["enabled_rules"].(int)
	assert.True(t, ok)
	assert.GreaterOrEqual(t, enabled, 1)

	disabled, ok := stats["disabled_rules"].(int)
	assert.True(t, ok)
	assert.GreaterOrEqual(t, disabled, 1)
}

// === 冷却时间测试 ===

func TestAlertRuleEngine_Cooldown(t *testing.T) {
	alertMgr := NewAlertingManager()
	engine := NewAlertRuleEngine("", alertMgr)

	rule := &AlertRuleConfig{

		ID:       "cpu-high",
		Name:     "CPU High",
		Enabled:  true,
		Type:     RuleTypeCPU,
		Cooldown: time.Minute, // 1 分钟冷却

		ID:        "cpu-high",
		Name:      "CPU High",
		Enabled:   true,
		Type:      RuleTypeCPU,
		Cooldown:  time.Minute, // 1 分钟冷却

		Conditions: []RuleCondition{
			{Field: "cpu_usage", Operator: OpGreaterThan, Value: 80},
		},
		Level: "warning",
	}
	_ = engine.AddRule(rule)

	metrics := map[string]float64{
		"cpu_usage": 90,
	}

	// 第一次评估应该触发
	alerts := engine.Evaluate(metrics, nil)
	assert.GreaterOrEqual(t, len(alerts), 1)

	// 立即再次评估，应该被冷却
	alerts = engine.Evaluate(metrics, nil)
	assert.Len(t, alerts, 0) // 冷却期内不触发
}

// === 默认规则测试 ===

func TestAlertRuleEngine_DefaultRules(t *testing.T) {
	engine := NewAlertRuleEngine("", NewAlertingManager())

	rules := engine.GetRules()

	// 应该有默认规则
	assert.NotEmpty(t, rules)

	// 检查是否包含 CPU 相关规则
	hasCPURule := false
	for _, r := range rules {
		if r.Type == RuleTypeCPU {
			hasCPURule = true
			break
		}
	}
	assert.True(t, hasCPURule, "应该包含 CPU 默认规则")
}

// === 比较运算符测试 ===

func TestCompareOperators(t *testing.T) {
	// 测试比较运算符常量
	assert.Equal(t, CompareOp("eq"), OpEqual)
	assert.Equal(t, CompareOp("ne"), OpNotEqual)
	assert.Equal(t, CompareOp("gt"), OpGreaterThan)
	assert.Equal(t, CompareOp("gte"), OpGreaterEqual)
	assert.Equal(t, CompareOp("lt"), OpLessThan)
	assert.Equal(t, CompareOp("lte"), OpLessEqual)
	assert.Equal(t, CompareOp("contains"), OpContains)
	assert.Equal(t, CompareOp("matches"), OpMatches)
	assert.Equal(t, CompareOp("exists"), OpExists)
	assert.Equal(t, CompareOp("rate"), OpChangeRate)
}

// === 逻辑运算符测试 ===

func TestLogicOperators(t *testing.T) {
	assert.Equal(t, LogicOperator("and"), LogicAnd)
	assert.Equal(t, LogicOperator("or"), LogicOr)
}

// === 规则类型测试 ===

func TestAlertRuleTypes(t *testing.T) {
	types := []AlertRuleType{
		RuleTypeCPU,
		RuleTypeMemory,
		RuleTypeDisk,
		RuleTypeDiskHealth,
		RuleTypeNetwork,
		RuleTypeTemperature,
		RuleTypeService,
		RuleTypeBackup,
		RuleTypeCustom,
	}

	for _, rt := range types {
		assert.NotEmpty(t, string(rt))
	}
}

// === 聚合类型测试 ===

func TestAggregationTypes(t *testing.T) {
	types := []AggregationType{
		AggAvg,
		AggMax,
		AggMin,
		AggSum,
		AggCount,
	}

	for _, at := range types {
		assert.NotEmpty(t, string(at))
	}
}

// === MetricValue 测试 ===

func TestMetricValue(t *testing.T) {
	now := time.Now()
	mv := MetricValue{
		Name:      "cpu_usage",
		Value:     75.5,
		Labels:    map[string]string{"host": "server1"},
		Timestamp: now,
		Metadata:  map[string]interface{}{"unit": "percent"},
	}

	assert.Equal(t, "cpu_usage", mv.Name)
	assert.Equal(t, 75.5, mv.Value)
	assert.Equal(t, "server1", mv.Labels["host"])
	assert.Equal(t, now, mv.Timestamp)
}

// === AlertRuleConfig 测试 ===

func TestAlertRuleConfig(t *testing.T) {
	now := time.Now()
	rule := &AlertRuleConfig{
		ID:          "test-rule",
		Name:        "Test Rule",
		Description: "Test Description",
		Enabled:     true,
		Type:        RuleTypeCPU,
		Conditions: []RuleCondition{
			{Field: "cpu_usage", Operator: OpGreaterThan, Value: 80},
		},
		Logic:        LogicAnd,
		Level:        "warning",
		Duration:     5 * time.Minute,
		Cooldown:     1 * time.Minute,
		Channels:     []string{"email", "webhook"},
		Labels:       map[string]string{"severity": "high"},
		Annotations:  map[string]string{"summary": "CPU high"},
		CreatedAt:    now,
		UpdatedAt:    now,
		TriggerCount: 0,
	}

	assert.Equal(t, "test-rule", rule.ID)
	assert.True(t, rule.Enabled)
	assert.Equal(t, 5*time.Minute, rule.Duration)
	assert.Equal(t, 1*time.Minute, rule.Cooldown)
	assert.Len(t, rule.Channels, 2)
}

// === RuleCondition 测试 ===

func TestRuleCondition(t *testing.T) {
	cond := RuleCondition{
		Field:    "cpu_usage",
		Operator: OpGreaterThan,
		Value:    80,
		Duration: 5 * time.Minute,
		Aggregation: &Aggregation{
			Type:   AggAvg,
			Window: 10 * time.Minute,
		},
	}

	assert.Equal(t, "cpu_usage", cond.Field)
	assert.Equal(t, OpGreaterThan, cond.Operator)
	assert.Equal(t, 80, cond.Value)
	assert.Equal(t, 5*time.Minute, cond.Duration)
	assert.NotNil(t, cond.Aggregation)
	assert.Equal(t, AggAvg, cond.Aggregation.Type)
}

// === 基准测试 ===

func BenchmarkAlertRuleEngine_Evaluate(b *testing.B) {
	engine := NewAlertRuleEngine("", NewAlertingManager())
	_ = engine.AddRule(&AlertRuleConfig{
		ID:      "cpu-high",
		Enabled: true,
		Type:    RuleTypeCPU,
		Conditions: []RuleCondition{
			{Field: "cpu_usage", Operator: OpGreaterThan, Value: 80},
		},
		Logic: LogicAnd,
	})

	metrics := map[string]float64{"cpu_usage": 90}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Evaluate(metrics, nil)
	}
}

func BenchmarkAlertRuleEngine_GetRules(b *testing.B) {
	engine := NewAlertRuleEngine("", NewAlertingManager())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.GetRules()
	}
}

