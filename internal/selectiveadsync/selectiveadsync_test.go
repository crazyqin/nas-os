package selectiveadsync

import (
	"testing"
)

func TestNewSelectiveADSyncManager(t *testing.T) {
	manager := NewSelectiveADSyncManager()
	if manager == nil {
		t.Fatal("NewSelectiveADSyncManager 返回 nil")
	}

	if manager.ous == nil {
		t.Fatal("ous map 未初始化")
	}

	if manager.rules == nil {
		t.Fatal("rules map 未初始化")
	}

	if manager.syncHistory == nil {
		t.Fatal("syncHistory 未初始化")
	}
}

func TestDiscoverOUs(t *testing.T) {
	manager := NewSelectiveADSyncManager()

	ous, err := manager.DiscoverOUs()
	if err != nil {
		t.Fatalf("发现OU失败: %v", err)
	}

	if len(ous) == 0 {
		t.Fatal("应该发现至少一个OU")
	}

	// 验证OU信息
	found := false
	for _, ou := range ous {
		if ou.Name == "Users" {
			found = true
			if ou.DN != "OU=Users,DC=example,DC=com" {
				t.Errorf("期望DN 'OU=Users,DC=example,DC=com', 实际 '%s'", ou.DN)
			}
			break
		}
	}

	if !found {
		t.Error("应该找到Users OU")
	}
}

func TestSelectOUs(t *testing.T) {
	manager := NewSelectiveADSyncManager()

	// 先发现OU
	manager.DiscoverOUs()

	// 测试选择OU
	err := manager.SelectOUs([]string{"OU=Users,DC=example,DC=com"}, false)
	if err != nil {
		t.Fatalf("选择OU失败: %v", err)
	}

	// 验证选择状态
	ou, exists := manager.ous["OU=Users,DC=example,DC=com"]
	if !exists {
		t.Fatal("OU不存在")
	}

	if !ou.IsSelected {
		t.Error("OU应该被选中")
	}

	// 测试选择不存在的OU
	err = manager.SelectOUs([]string{"OU=NonExistent,DC=example,DC=com"}, false)
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}
}

func TestDeselectOUs(t *testing.T) {
	manager := NewSelectiveADSyncManager()

	// 先发现OU并选择
	manager.DiscoverOUs()
	manager.SelectOUs([]string{"OU=Users,DC=example,DC=com"}, false)

	// 测试取消选择
	err := manager.DeselectOUs([]string{"OU=Users,DC=example,DC=com"})
	if err != nil {
		t.Fatalf("取消选择OU失败: %v", err)
	}

	// 验证取消选择状态
	ou := manager.ous["OU=Users,DC=example,DC=com"]
	if ou.IsSelected {
		t.Error("OU不应该被选中")
	}
}

func TestGetSelectedOUs(t *testing.T) {
	manager := NewSelectiveADSyncManager()

	// 先发现OU
	manager.DiscoverOUs()

	// 选择多个OU
	manager.SelectOUs([]string{
		"OU=Users,DC=example,DC=com",
		"OU=Groups,DC=example,DC=com",
	}, false)

	// 获取已选择的OU
	selected := manager.GetSelectedOUs()

	if len(selected) != 2 {
		t.Errorf("期望2个已选择的OU, 实际 %d", len(selected))
	}

	// 验证名称
	names := make(map[string]bool)
	for _, ou := range selected {
		names[ou.Name] = true
	}

	if !names["Users"] || !names["Groups"] {
		t.Error("应该选择Users和Groups")
	}
}

func TestCreateRule(t *testing.T) {
	manager := NewSelectiveADSyncManager()

	rule := SyncRule{
		Name:        "Test Rule",
		Description: "测试规则",
		Filter: OUFilter{
			IncludeOUs: []string{"OU=Users,DC=example,DC=com"},
		},
		SyncUsers:  true,
		SyncGroups: true,
		Enabled:    true,
	}

	created, err := manager.CreateRule(rule)
	if err != nil {
		t.Fatalf("创建规则失败: %v", err)
	}

	if created.ID == "" {
		t.Error("规则ID不能为空")
	}

	if created.Name != "Test Rule" {
		t.Errorf("期望规则名称 'Test Rule', 实际 '%s'", created.Name)
	}

	// 测试创建无名称规则
	invalidRule := SyncRule{
		Description: "无名称规则",
	}

	_, err = manager.CreateRule(invalidRule)
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}
}

func TestUpdateRule(t *testing.T) {
	manager := NewSelectiveADSyncManager()

	// 先创建规则
	rule := SyncRule{
		Name:     "Original Rule",
		Enabled:  true,
	}
	created, _ := manager.CreateRule(rule)

	// 更新规则
	created.Name = "Updated Rule"
	created.Description = "更新后的规则"

	err := manager.UpdateRule(*created)
	if err != nil {
		t.Fatalf("更新规则失败: %v", err)
	}

	// 验证更新
	updated, _ := manager.GetRule(created.ID)
	if updated.Name != "Updated Rule" {
		t.Errorf("期望规则名称 'Updated Rule', 实际 '%s'", updated.Name)
	}

	// 测试更新不存在的规则
	nonExistent := SyncRule{
		ID:   "non-existent",
		Name: "Test",
	}
	err = manager.UpdateRule(nonExistent)
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}
}

func TestDeleteRule(t *testing.T) {
	manager := NewSelectiveADSyncManager()

	// 先创建规则
	rule := SyncRule{
		Name:    "To Delete",
		Enabled: true,
	}
	created, _ := manager.CreateRule(rule)

	// 删除规则
	err := manager.DeleteRule(created.ID)
	if err != nil {
		t.Fatalf("删除规则失败: %v", err)
	}

	// 验证删除
	_, err = manager.GetRule(created.ID)
	if err == nil {
		t.Fatal("规则应该已被删除")
	}

	// 测试删除不存在的规则
	err = manager.DeleteRule("non-existent")
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}
}

func TestListRules(t *testing.T) {
	manager := NewSelectiveADSyncManager()

	// 创建多个规则
	for i := 0; i < 3; i++ {
		rule := SyncRule{
			Name:    "Rule " + string(rune('A'+i)),
			Enabled: true,
		}
		manager.CreateRule(rule)
	}

	// 列出规则
	rules := manager.ListRules()

	if len(rules) != 3 {
		t.Errorf("期望3个规则, 实际 %d", len(rules))
	}
}

func TestSync(t *testing.T) {
	manager := NewSelectiveADSyncManager()

	// 发现OU
	manager.DiscoverOUs()

	// 选择OU
	manager.SelectOUs([]string{
		"OU=Users,DC=example,DC=com",
		"OU=Groups,DC=example,DC=com",
	}, false)

	// 创建规则
	rule := SyncRule{
		Name:       "Sync Users",
		SyncUsers:  true,
		SyncGroups: true,
		Enabled:    true,
		Filter: OUFilter{
			IncludeOUs: []string{"Users", "Groups"},
		},
	}
	manager.CreateRule(rule)

	// 执行同步
	req := SyncRequest{
		DryRun: false,
	}

	result, err := manager.Sync(req)
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}

	if result == nil {
		t.Fatal("同步结果为 nil")
	}

	if result.Status != SyncStatusSuccess {
		t.Errorf("期望状态 'success', 实际 '%s'", result.Status)
	}

	if result.TotalOUs != 2 {
		t.Errorf("期望2个OU, 实际 %d", result.TotalOUs)
	}

	if result.SyncedOUs != 2 {
		t.Errorf("期望同步2个OU, 实际 %d", result.SyncedOUs)
	}
}

func TestSyncDryRun(t *testing.T) {
	manager := NewSelectiveADSyncManager()

	// 发现OU并选择
	manager.DiscoverOUs()
	manager.SelectOUs([]string{"OU=Users,DC=example,DC=com"}, false)

	// 模拟运行
	req := SyncRequest{
		DryRun: true,
	}

	result, err := manager.Sync(req)
	if err != nil {
		t.Fatalf("模拟同步失败: %v", err)
	}

	if result.Status != SyncStatusSuccess {
		t.Errorf("期望状态 'success', 实际 '%s'", result.Status)
	}
}

func TestGetLastSyncResult(t *testing.T) {
	manager := NewSelectiveADSyncManager()

	// 初始应该为nil
	result := manager.GetLastSyncResult()
	if result != nil {
		t.Fatal("初始结果应该为nil")
	}

	// 执行同步
	manager.DiscoverOUs()
	manager.SelectOUs([]string{"OU=Users,DC=example,DC=com"}, false)
	manager.Sync(SyncRequest{})

	// 获取最后结果
	result = manager.GetLastSyncResult()
	if result == nil {
		t.Fatal("应该有同步结果")
	}
}

func TestGetSyncHistory(t *testing.T) {
	manager := NewSelectiveADSyncManager()

	// 执行多次同步
	manager.DiscoverOUs()
	manager.SelectOUs([]string{"OU=Users,DC=example,DC=com"}, false)

	manager.Sync(SyncRequest{})
	manager.Sync(SyncRequest{})

	// 获取历史
	history := manager.GetSyncHistory()

	if len(history) != 2 {
		t.Errorf("期望2条历史记录, 实际 %d", len(history))
	}
}

func TestGetStats(t *testing.T) {
	manager := NewSelectiveADSyncManager()

	// 执行同步
	manager.DiscoverOUs()
	manager.SelectOUs([]string{"OU=Users,DC=example,DC=com"}, false)
	manager.Sync(SyncRequest{})

	// 获取统计
	stats := manager.GetStats()

	if stats.TotalSyncs != 1 {
		t.Errorf("期望1次同步, 实际 %d", stats.TotalSyncs)
	}

	if stats.SuccessSyncs != 1 {
		t.Errorf("期望1次成功, 实际 %d", stats.SuccessSyncs)
	}

	if stats.TotalOUs == 0 {
		t.Error("总OU数应该大于0")
	}
}

func TestApplyFilters(t *testing.T) {
	manager := NewSelectiveADSyncManager()

	// 创建测试OU
	ous := []OUInfo{
		{DN: "OU=Users,DC=example,DC=com", Name: "Users"},
		{DN: "OU=Groups,DC=example,DC=com", Name: "Groups"},
		{DN: "OU=Computers,DC=example,DC=com", Name: "Computers"},
	}

	// 测试包含过滤
	rules := []SyncRule{
		{
			Enabled: true,
			Filter: OUFilter{
				IncludeOUs: []string{"Users"},
			},
		},
	}

	filtered := manager.applyFilters(ous, rules)
	if len(filtered) != 1 {
		t.Errorf("期望1个OU, 实际 %d", len(filtered))
	}

	// 测试排除过滤
	rules = []SyncRule{
		{
			Enabled: true,
			Filter: OUFilter{
				ExcludeOUs: []string{"Computers"},
			},
		},
	}

	filtered = manager.applyFilters(ous, rules)
	if len(filtered) != 2 {
		t.Errorf("期望2个OU, 实际 %d", len(filtered))
	}
}

func TestMatchesRule(t *testing.T) {
	manager := NewSelectiveADSyncManager()

	ou := OUInfo{
		DN:   "OU=Users,DC=example,DC=com",
		Name: "Users",
	}

	// 测试包含匹配
	rule := SyncRule{
		Enabled: true,
		Filter: OUFilter{
			IncludeOUs: []string{"Users"},
		},
	}

	if !manager.matchesRule(ou, rule) {
		t.Error("应该匹配包含规则")
	}

	// 测试排除匹配
	rule = SyncRule{
		Enabled: true,
		Filter: OUFilter{
			ExcludeOUs: []string{"Users"},
		},
	}

	if manager.matchesRule(ou, rule) {
		t.Error("不应该匹配排除规则")
	}

	// 测试模式匹配
	rule = SyncRule{
		Enabled: true,
		Filter: OUFilter{
			Patterns: []string{".*Users.*"},
		},
	}

	if !manager.matchesRule(ou, rule) {
		t.Error("应该匹配模式规则")
	}
}

func TestConfig(t *testing.T) {
	manager := NewSelectiveADSyncManager()

	// 获取默认配置
	config := manager.GetConfig()
	if config.SyncInterval == 0 {
		t.Error("默认同步间隔应该大于0")
	}

	// 设置新配置
	newConfig := OUSyncConfig{
		DomainController: "dc.example.com",
		BaseDN:           "DC=example,DC=com",
		SyncInterval:     12 * 3600, // 12小时
		Enabled:          true,
	}

	manager.SetConfig(newConfig)

	// 验证配置
	config = manager.GetConfig()
	if config.DomainController != "dc.example.com" {
		t.Errorf("期望域控制器 'dc.example.com', 实际 '%s'", config.DomainController)
	}

	if !config.Enabled {
		t.Error("配置应该启用")
	}
}

func TestNewModule(t *testing.T) {
	module := NewModule()
	if module == nil {
		t.Fatal("NewModule 返回 nil")
	}

	if module.Name() != "selectiveadsync" {
		t.Errorf("期望模块名称 'selectiveadsync', 实际 '%s'", module.Name())
	}

	if !module.IsEnabled() {
		t.Error("模块应该默认启用")
	}

	// 测试禁用
	module.Disable()
	if module.IsEnabled() {
		t.Error("模块应该被禁用")
	}

	// 测试启用
	module.Enable()
	if !module.IsEnabled() {
		t.Error("模块应该被启用")
	}
}

func TestReplaceSelection(t *testing.T) {
	manager := NewSelectiveADSyncManager()

	// 发现OU
	manager.DiscoverOUs()

	// 初始选择
	manager.SelectOUs([]string{
		"OU=Users,DC=example,DC=com",
		"OU=Groups,DC=example,DC=com",
	}, false)

	selected := manager.GetSelectedOUs()
	if len(selected) != 2 {
		t.Errorf("期望2个已选择OU, 实际 %d", len(selected))
	}

	// 替换选择
	manager.SelectOUs([]string{
		"OU=Computers,DC=example,DC=com",
	}, true)

	selected = manager.GetSelectedOUs()
	if len(selected) != 1 {
		t.Errorf("期望1个已选择OU, 实际 %d", len(selected))
	}

	if selected[0].Name != "Computers" {
		t.Errorf("期望选择Computers, 实际 %s", selected[0].Name)
	}
}
