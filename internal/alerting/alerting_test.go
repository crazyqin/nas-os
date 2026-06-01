package alerting

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// ========== 告警分组管理器测试 ==========

func TestNewAlertGroupManager(t *testing.T) {
	mgr := NewAlertGroupManager()

	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.groups)
	assert.NotNil(t, mgr.stats)

	// 验证默认分组已初始化
	assert.NotEmpty(t, mgr.groups)

	// 验证统计初始化
	assert.Len(t, mgr.stats, 4)
	assert.NotNil(t, mgr.stats[CategoryStorage])
	assert.NotNil(t, mgr.stats[CategoryNetwork])
	assert.NotNil(t, mgr.stats[CategorySystem])
	assert.NotNil(t, mgr.stats[CategorySecurity])
}

func TestAlertGroupManager_CreateGroup(t *testing.T) {
	mgr := NewAlertGroupManager()

	group := AlertGroup{
		ID:          "test-group-1",
		Name:        "测试分组",
		Description: "测试描述",
		Category:    CategoryStorage,
		Priority:    5,
		Enabled:     true,
		Rules:       []string{"rule-1", "rule-2"},
		Labels:      map[string]string{"env": "test"},
	}

	created, err := mgr.CreateGroup(group)
	assert.NoError(t, err)
	assert.NotNil(t, created)
	assert.Equal(t, "test-group-1", created.ID)
	assert.Equal(t, "测试分组", created.Name)
	assert.False(t, created.CreatedAt.IsZero())
	assert.False(t, created.UpdatedAt.IsZero())
}

func TestAlertGroupManager_CreateGroupDuplicate(t *testing.T) {
	mgr := NewAlertGroupManager()

	group := AlertGroup{
		ID:       "duplicate-id",
		Name:     "分组1",
		Category: CategoryStorage,
	}

	_, err := mgr.CreateGroup(group)
	assert.NoError(t, err)

	// 重复创建应返回错误
	group.Name = "分组2"
	_, err = mgr.CreateGroup(group)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "分组已存在")
}

func TestAlertGroupManager_CreateGroupAutoID(t *testing.T) {
	mgr := NewAlertGroupManager()

	group := AlertGroup{
		Name:     "自动ID分组",
		Category: CategoryNetwork,
	}

	created, err := mgr.CreateGroup(group)
	assert.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Contains(t, created.ID, "group-network-")
}

func TestAlertGroupManager_GetGroup(t *testing.T) {
	mgr := NewAlertGroupManager()

	// 获取存在的分组
	group, err := mgr.GetGroup("storage-disk-space")
	assert.NoError(t, err)
	assert.NotNil(t, group)
	assert.Equal(t, "磁盘空间", group.Name)
	assert.Equal(t, CategoryStorage, group.Category)

	// 获取不存在的分组
	_, err = mgr.GetGroup("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "分组不存在")
}

func TestAlertGroupManager_ListGroups(t *testing.T) {
	mgr := NewAlertGroupManager()

	groups := mgr.ListGroups()
	assert.NotEmpty(t, groups)

	// 验证按优先级排序
	for i := 1; i < len(groups); i++ {
		assert.GreaterOrEqual(t, groups[i].Priority, groups[i-1].Priority)
	}
}

func TestAlertGroupManager_ListGroupsByCategory(t *testing.T) {
	mgr := NewAlertGroupManager()

	storageGroups := mgr.ListGroupsByCategory(CategoryStorage)
	assert.NotEmpty(t, storageGroups)

	for _, g := range storageGroups {
		assert.Equal(t, CategoryStorage, g.Category)
	}

	networkGroups := mgr.ListGroupsByCategory(CategoryNetwork)
	assert.NotEmpty(t, networkGroups)

	for _, g := range networkGroups {
		assert.Equal(t, CategoryNetwork, g.Category)
	}
}

func TestAlertGroupManager_UpdateGroup(t *testing.T) {
	mgr := NewAlertGroupManager()

	updated := AlertGroup{
		Name:        "更新后的分组",
		Description: "更新后的描述",
		Category:    CategorySystem,
		Priority:    10,
		Enabled:     false,
	}

	result, err := mgr.UpdateGroup("system-cpu", updated)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "更新后的分组", result.Name)
	assert.Equal(t, "system-cpu", result.ID) // ID应保持不变

	// 更新不存在的分组
	_, err = mgr.UpdateGroup("non-existent", updated)
	assert.Error(t, err)
}

func TestAlertGroupManager_DeleteGroup(t *testing.T) {
	mgr := NewAlertGroupManager()

	// 创建一个测试分组
	group := AlertGroup{
		ID:       "delete-test",
		Name:     "待删除",
		Category: CategorySecurity,
	}
	_, err := mgr.CreateGroup(group)
	assert.NoError(t, err)

	// 删除分组
	err = mgr.DeleteGroup("delete-test")
	assert.NoError(t, err)

	// 验证已删除
	_, err = mgr.GetGroup("delete-test")
	assert.Error(t, err)

	// 删除不存在的分组
	err = mgr.DeleteGroup("non-existent")
	assert.Error(t, err)
}

func TestAlertGroupManager_EnableDisableGroup(t *testing.T) {
	mgr := NewAlertGroupManager()

	// 禁用分组
	err := mgr.DisableGroup("system-cpu")
	assert.NoError(t, err)

	group, _ := mgr.GetGroup("system-cpu")
	assert.False(t, group.Enabled)

	// 启用分组
	err = mgr.EnableGroup("system-cpu")
	assert.NoError(t, err)

	group, _ = mgr.GetGroup("system-cpu")
	assert.True(t, group.Enabled)

	// 操作不存在的分组
	err = mgr.EnableGroup("non-existent")
	assert.Error(t, err)

	err = mgr.DisableGroup("non-existent")
	assert.Error(t, err)
}

func TestAlertGroupManager_RuleManagement(t *testing.T) {
	mgr := NewAlertGroupManager()

	// 添加规则
	err := mgr.AddRuleToGroup("storage-disk-space", "new-rule-1")
	assert.NoError(t, err)

	group, _ := mgr.GetGroup("storage-disk-space")
	assert.Contains(t, group.Rules, "new-rule-1")

	// 重复添加规则（应忽略）
	err = mgr.AddRuleToGroup("storage-disk-space", "new-rule-1")
	assert.NoError(t, err)

	// 移除规则
	err = mgr.RemoveRuleFromGroup("storage-disk-space", "new-rule-1")
	assert.NoError(t, err)

	group, _ = mgr.GetGroup("storage-disk-space")
	assert.NotContains(t, group.Rules, "new-rule-1")

	// 移除不存在的规则
	err = mgr.RemoveRuleFromGroup("storage-disk-space", "non-existent-rule")
	assert.Error(t, err)

	// 操作不存在的分组
	err = mgr.AddRuleToGroup("non-existent", "rule")
	assert.Error(t, err)
}

func TestAlertGroupManager_GetGroupsByRuleID(t *testing.T) {
	mgr := NewAlertGroupManager()

	groups := mgr.GetGroupsByRuleID("disk-space-warning")
	assert.NotEmpty(t, groups)

	for _, g := range groups {
		assert.Contains(t, g.Rules, "disk-space-warning")
	}
}

func TestAlertGroupManager_UpdateStats(t *testing.T) {
	mgr := NewAlertGroupManager()

	now := time.Now()
	alerts := []GroupAlertInfo{
		{
			ID:           "alert-1",
			Level:        AlertLevelCritical,
			Resolved:     false,
			Acknowledged: false,
			Timestamp:    &now,
		},
		{
			ID:           "alert-2",
			Level:        AlertLevelWarning,
			Resolved:     false,
			Acknowledged: true,
			Timestamp:    &now,
		},
		{
			ID:           "alert-3",
			Level:        AlertLevelInfo,
			Resolved:     true,
			Acknowledged: true,
			Timestamp:    &now,
		},
	}

	mgr.UpdateStats(CategoryStorage, alerts)

	stats := mgr.GetStats(CategoryStorage)
	assert.NotNil(t, stats)
	assert.Equal(t, 3, stats.TotalAlerts)
	assert.Equal(t, 2, stats.ActiveAlerts) // 2个未解决
	assert.Equal(t, 1, stats.CriticalCount)
	assert.Equal(t, 1, stats.WarningCount)
	assert.Equal(t, 1, stats.InfoCount)
	assert.InDelta(t, 66.67, stats.AcknowledgedPct, 0.01) // 2/3 = 66.67%
	assert.NotNil(t, stats.LastAlertTime)
}

func TestAlertGroupManager_GetAllStats(t *testing.T) {
	mgr := NewAlertGroupManager()

	allStats := mgr.GetAllStats()
	assert.Len(t, allStats, 4)
	assert.Contains(t, allStats, CategoryStorage)
	assert.Contains(t, allStats, CategoryNetwork)
	assert.Contains(t, allStats, CategorySystem)
	assert.Contains(t, allStats, CategorySecurity)
}

func TestAlertGroupManager_GetCategorySummary(t *testing.T) {
	mgr := NewAlertGroupManager()

	// 添加一些告警数据
	now := time.Now()
	alerts := []GroupAlertInfo{
		{Level: AlertLevelCritical, Resolved: false, Timestamp: &now},
		{Level: AlertLevelWarning, Resolved: false, Timestamp: &now},
	}
	mgr.UpdateStats(CategoryStorage, alerts)

	summary := mgr.GetCategorySummary()
	assert.NotNil(t, summary)
	assert.Contains(t, summary, "categories")
	assert.Contains(t, summary, "total_active")
	assert.Contains(t, summary, "total_critical")
	assert.Contains(t, summary, "health_score")
}

func TestAlertGroupManager_SaveLoadGroups(t *testing.T) {
	mgr := NewAlertGroupManager()

	// 创建临时文件
	tmpFile := t.TempDir() + "/groups.json"

	// 保存分组
	err := mgr.SaveGroups(tmpFile)
	assert.NoError(t, err)

	// 创建新的管理器并加载
	mgr2 := NewAlertGroupManager()
	err = mgr2.LoadGroups(tmpFile)
	assert.NoError(t, err)

	// 验证加载的分组包含原始分组（加上新加载的）
	originalGroups := mgr.ListGroups()
	loadedGroups := mgr2.ListGroups()
	assert.GreaterOrEqual(t, len(loadedGroups), len(originalGroups))
}

func TestAlertGroupManager_LoadGroupsNonExistent(t *testing.T) {
	mgr := NewAlertGroupManager()

	// 加载不存在的文件应返回nil
	err := mgr.LoadGroups("/non/existent/path.json")
	assert.NoError(t, err)
}

// ========== 告警规则测试 ==========

func TestNewCustomAlertRule(t *testing.T) {
	rule := NewCustomAlertRule("CPU过高", GroupSystem, MetricCPUUsage)

	assert.NotNil(t, rule)
	assert.NotEmpty(t, rule.ID)
	assert.Equal(t, "CPU过高", rule.Name)
	assert.Equal(t, GroupSystem, rule.Group)
	assert.Equal(t, MetricCPUUsage, rule.Metric)
	assert.True(t, rule.Enabled)
	assert.Equal(t, AlertLevelWarning, rule.Level)
	assert.Equal(t, OpGreaterThan, rule.Operator)
	assert.Equal(t, 60, rule.Duration)
}

func TestCustomAlertRule_EvaluateImmediate(t *testing.T) {
	rule := NewCustomAlertRule("磁盘空间", GroupStorage, MetricDiskUsage)
	rule.Threshold = 90.0
	rule.Operator = OpGreaterThan
	rule.Duration = 0 // 立即触发

	// 值超过阈值
	assert.True(t, rule.Evaluate(95.0))

	// 值等于阈值（> 不能相等）
	assert.False(t, rule.Evaluate(90.0))

	// 值低于阈值
	assert.False(t, rule.Evaluate(85.0))
}

func TestCustomAlertRule_EvaluateWithDuration(t *testing.T) {
	rule := NewCustomAlertRule("CPU持续高", GroupSystem, MetricCPUUsage)
	rule.Threshold = 80.0
	rule.Operator = OpGreaterThan
	rule.Duration = 2 // 2秒持续时间

	// 第一次评估，应开始pending
	assert.False(t, rule.Evaluate(90.0))

	// 等待一段时间但未达到持续时间
	time.Sleep(500 * time.Millisecond)
	assert.False(t, rule.Evaluate(90.0))

	// 等待达到持续时间
	time.Sleep(2 * time.Second)
	assert.True(t, rule.Evaluate(90.0))
}

func TestCustomAlertRule_EvaluateOperators(t *testing.T) {
	tests := []struct {
		name      string
		operator  Operator
		threshold float64
		value     float64
		expected  bool
	}{
		{"大于-触发", OpGreaterThan, 80, 90, true},
		{"大于-不触发", OpGreaterThan, 80, 80, false},
		{"大于等于-触发", OpGreaterEqual, 80, 80, true},
		{"大于等于-不触发", OpGreaterEqual, 80, 79, false},
		{"小于-触发", OpLessThan, 80, 70, true},
		{"小于-不触发", OpLessThan, 80, 80, false},
		{"小于等于-触发", OpLessEqual, 80, 80, true},
		{"小于等于-不触发", OpLessEqual, 80, 81, false},
		{"等于-触发", OpEqual, 80, 80, true},
		{"等于-不触发", OpEqual, 80, 81, false},
		{"不等于-触发", OpNotEqual, 80, 81, true},
		{"不等于-不触发", OpNotEqual, 80, 80, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewCustomAlertRule("test", GroupSystem, MetricCPUUsage)
			rule.Threshold = tt.threshold
			rule.Operator = tt.operator
			rule.Duration = 0

			assert.Equal(t, tt.expected, rule.Evaluate(tt.value))
		})
	}
}

func TestCustomAlertRule_ResetPending(t *testing.T) {
	rule := NewCustomAlertRule("test", GroupSystem, MetricCPUUsage)
	rule.Threshold = 80
	rule.Duration = 100

	// 触发pending
	rule.Evaluate(90)
	state := rule.GetState()
	assert.True(t, state.IsPending)

	// 重置pending
	rule.ResetPending()
	state = rule.GetState()
	assert.False(t, state.IsPending)
}

func TestCustomAlertRule_Update(t *testing.T) {
	rule := NewCustomAlertRule("test", GroupSystem, MetricCPUUsage)

	newName := "新名称"
	newThreshold := 90.0
	newLevel := AlertLevelCritical

	err := rule.Update(RuleUpdate{
		Name:      &newName,
		Threshold: &newThreshold,
		Level:     &newLevel,
	})

	assert.NoError(t, err)
	assert.Equal(t, "新名称", rule.Name)
	assert.Equal(t, 90.0, rule.Threshold)
	assert.Equal(t, AlertLevelCritical, rule.Level)
}

func TestCustomAlertRule_UpdateInvalidThreshold(t *testing.T) {
	rule := NewCustomAlertRule("test", GroupSystem, MetricCPUUsage)

	invalidThreshold := -10.0
	err := rule.Update(RuleUpdate{
		Threshold: &invalidThreshold,
	})

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidThreshold, err)
}

func TestCustomAlertRule_UpdateInvalidDuration(t *testing.T) {
	rule := NewCustomAlertRule("test", GroupSystem, MetricCPUUsage)

	invalidDuration := -5
	err := rule.Update(RuleUpdate{
		Duration: &invalidDuration,
	})

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidDuration, err)
}

// ========== 规则管理器测试 ==========

func TestNewCustomRuleManager(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewCustomRuleManager(logger, nil)

	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.rules)
	assert.NotNil(t, mgr.ruleList)
	assert.NotNil(t, mgr.groups)
	assert.NotNil(t, mgr.collectors)
}

func TestCustomRuleManager_AddRule(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewCustomRuleManager(logger, nil)

	rule := NewCustomAlertRule("测试规则", GroupSystem, MetricCPUUsage)
	rule.Threshold = 80

	err := mgr.AddRule(rule)
	assert.NoError(t, err)

	// 验证规则已添加
	fetched, err := mgr.GetRule(rule.ID)
	assert.NoError(t, err)
	assert.Equal(t, "测试规则", fetched.Name)
}

func TestCustomRuleManager_AddRuleDuplicateName(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewCustomRuleManager(logger, nil)

	rule1 := NewCustomAlertRule("重复名称", GroupSystem, MetricCPUUsage)
	rule2 := NewCustomAlertRule("重复名称", GroupStorage, MetricDiskUsage)

	err := mgr.AddRule(rule1)
	assert.NoError(t, err)

	err = mgr.AddRule(rule2)
	assert.Error(t, err)
	assert.Equal(t, ErrRuleNameDuplicate, err)
}

func TestCustomRuleManager_GetRule(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewCustomRuleManager(logger, nil)

	rule := NewCustomAlertRule("获取测试", GroupSystem, MetricCPUUsage)
	mgr.AddRule(rule)

	fetched, err := mgr.GetRule(rule.ID)
	assert.NoError(t, err)
	assert.Equal(t, rule.ID, fetched.ID)

	// 获取不存在的规则
	_, err = mgr.GetRule("non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrRuleNotFound, err)
}

func TestCustomRuleManager_GetRules(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewCustomRuleManager(logger, nil)

	rules := mgr.GetRules()
	assert.NotEmpty(t, rules) // 应包含默认模板

	// 添加自定义规则
	rule := NewCustomAlertRule("自定义规则", GroupSystem, MetricCPUUsage)
	mgr.AddRule(rule)

	rules = mgr.GetRules()
	assert.Greater(t, len(rules), 1)
}

func TestCustomRuleManager_GetRulesByGroup(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewCustomRuleManager(logger, nil)

	systemRules := mgr.GetRulesByGroup(GroupSystem)
	assert.NotEmpty(t, systemRules)

	for _, r := range systemRules {
		assert.Equal(t, GroupSystem, r.Group)
	}
}

func TestCustomRuleManager_GetEnabledRules(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewCustomRuleManager(logger, nil)

	// 添加一个启用的规则
	rule := NewCustomAlertRule("启用规则", GroupSystem, MetricCPUUsage)
	rule.Enabled = true
	mgr.AddRule(rule)

	enabledRules := mgr.GetEnabledRules()
	assert.NotEmpty(t, enabledRules)

	for _, r := range enabledRules {
		assert.True(t, r.Enabled)
	}
}

func TestCustomRuleManager_UpdateRule(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewCustomRuleManager(logger, nil)

	rule := NewCustomAlertRule("待更新", GroupSystem, MetricCPUUsage)
	mgr.AddRule(rule)

	newName := "已更新"
	err := mgr.UpdateRule(rule.ID, RuleUpdate{
		Name: &newName,
	})
	assert.NoError(t, err)

	updated, _ := mgr.GetRule(rule.ID)
	assert.Equal(t, "已更新", updated.Name)

	// 更新不存在的规则
	err = mgr.UpdateRule("non-existent", RuleUpdate{})
	assert.Error(t, err)
	assert.Equal(t, ErrRuleNotFound, err)
}

func TestCustomRuleManager_DeleteRule(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewCustomRuleManager(logger, nil)

	rule := NewCustomAlertRule("待删除", GroupSystem, MetricCPUUsage)
	mgr.AddRule(rule)

	err := mgr.DeleteRule(rule.ID)
	assert.NoError(t, err)

	_, err = mgr.GetRule(rule.ID)
	assert.Error(t, err)
	assert.Equal(t, ErrRuleNotFound, err)

	// 删除不存在的规则
	err = mgr.DeleteRule("non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrRuleNotFound, err)
}

func TestCustomRuleManager_GetTemplates(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewCustomRuleManager(logger, nil)

	templates := mgr.GetTemplates()
	assert.NotEmpty(t, templates)

	for _, tmpl := range templates {
		assert.Equal(t, "true", tmpl.Labels["template"])
		assert.False(t, tmpl.Enabled) // 模板默认禁用
	}
}

func TestCustomRuleManager_GetStats(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewCustomRuleManager(logger, nil)

	stats := mgr.GetStats()
	assert.Greater(t, stats.TotalRules, 0)
	assert.Greater(t, stats.Templates, 0)
	assert.NotEmpty(t, stats.ByGroup)
	assert.NotEmpty(t, stats.ByMetric)
	assert.NotEmpty(t, stats.ByLevel)
}

func TestCustomRuleManager_RegisterCollector(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewCustomRuleManager(logger, nil)

	collector := &mockCollector{
		values: map[string]float64{"cpu1": 90.0},
	}

	mgr.RegisterCollector(MetricCPUUsage, collector)
	assert.Contains(t, mgr.collectors, MetricCPUUsage)
}

func TestCustomRuleManager_SetOnAlert(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewCustomRuleManager(logger, nil)

	mgr.SetOnAlert(func(alert *CustomAlert) {
		// 回调已设置
	})

	assert.NotNil(t, mgr.onAlert)
}

func TestAlertLevelPriority(t *testing.T) {
	assert.Equal(t, 4, AlertLevelPriority[AlertLevelEmergency])
	assert.Equal(t, 3, AlertLevelPriority[AlertLevelCritical])
	assert.Equal(t, 2, AlertLevelPriority[AlertLevelWarning])
	assert.Equal(t, 1, AlertLevelPriority[AlertLevelInfo])
}

func TestDefaultRuleManagerConfig(t *testing.T) {
	config := DefaultRuleManagerConfig()

	assert.Equal(t, 100, config.MaxRules)
	assert.Equal(t, 30*time.Second, config.CheckInterval)
	assert.Equal(t, 60, config.DefaultDuration)
}

// ========== 辅助函数测试 ==========

func TestGenerateGroupID(t *testing.T) {
	id1 := generateGroupID(CategoryStorage)
	id2 := generateGroupID(CategoryStorage)

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.Contains(t, id1, "group-storage-")
	assert.NotEqual(t, id1, id2) // 应生成唯一ID
}

func TestSortGroupsByPriority(t *testing.T) {
	groups := []*AlertGroup{
		{ID: "1", Priority: 5},
		{ID: "2", Priority: 1},
		{ID: "3", Priority: 3},
	}

	sortGroupsByPriority(groups)

	assert.Equal(t, 1, groups[0].Priority)
	assert.Equal(t, 3, groups[1].Priority)
	assert.Equal(t, 5, groups[2].Priority)
}

func TestGetCategoryName(t *testing.T) {
	assert.Equal(t, "存储", getCategoryName(CategoryStorage))
	assert.Equal(t, "网络", getCategoryName(CategoryNetwork))
	assert.Equal(t, "系统", getCategoryName(CategorySystem))
	assert.Equal(t, "安全", getCategoryName(CategorySecurity))
}

func TestGetCategoryIcon(t *testing.T) {
	assert.Equal(t, "storage", getCategoryIcon(CategoryStorage))
	assert.Equal(t, "network", getCategoryIcon(CategoryNetwork))
	assert.Equal(t, "system", getCategoryIcon(CategorySystem))
	assert.Equal(t, "security", getCategoryIcon(CategorySecurity))
}

func TestGetHealthStatus(t *testing.T) {
	tests := []struct {
		name     string
		stats    *GroupStats
		expected string
	}{
		{"有Critical", &GroupStats{CriticalCount: 1}, "critical"},
		{"有Warning", &GroupStats{WarningCount: 1}, "warning"},
		{"有Info", &GroupStats{InfoCount: 1}, "info"},
		{"健康", &GroupStats{}, "healthy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, getHealthStatus(tt.stats))
		})
	}
}

func TestContains(t *testing.T) {
	list := []string{"a", "b", "c"}

	assert.True(t, contains(list, "a"))
	assert.True(t, contains(list, "b"))
	assert.False(t, contains(list, "d"))
	assert.False(t, contains([]string{}, "a"))
}

// ========== Mock ==========

type mockCollector struct {
	values map[string]float64
	err    error
}

func (m *mockCollector) Collect(ctx context.Context) (map[string]float64, error) {
	return m.values, m.err
}
