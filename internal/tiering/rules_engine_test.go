package tiering

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestRulesEngine(t *testing.T) (*RulesEngine, string) {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	config := DefaultPolicyEngineConfig()
	manager := NewManager(configPath, config)
	_ = manager.Initialize()
	_ = manager.CreateTier(TierTypeSSD, TierConfig{
		Type:     TierTypeSSD,
		Name:     "SSD",
		Path:     "/mnt/ssd",
		Priority: 100,
		Enabled:  true,
	})
	_ = manager.CreateTier(TierTypeHDD, TierConfig{
		Type:     TierTypeHDD,
		Name:     "HDD",
		Path:     "/mnt/hdd",
		Priority: 50,
		Enabled:  true,
	})

	engine := NewRulesEngine(manager, tmpDir)
	if err := engine.Initialize(); err != nil {
		t.Fatalf("初始化规则引擎失败: %v", err)
	}

	return engine, tmpDir
}

func TestAddAndListRules(t *testing.T) {
	engine, _ := setupTestRulesEngine(t)

	// 添加年龄规则
	rule1, err := engine.AddRule(TieringRule{
		RuleName:      "旧文件归档",
		TierType:      TierTypeHDD,
		ConditionType: ConditionTypeAge,
		Threshold:     30,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("添加规则失败: %v", err)
	}

	// 添加文件类型规则
	rule2, err := engine.AddRule(TieringRule{
		RuleName:      "视频文件分层",
		TierType:      TierTypeCloud,
		ConditionType: ConditionTypeFileType,
		FilePattern:   ".mp4,.mkv,.avi",
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("添加规则失败: %v", err)
	}

	// 列出规则
	rules := engine.ListRules()
	if len(rules) != 2 {
		t.Fatalf("期望 2 条规则，实际 %d 条", len(rules))
	}

	// 验证规则内容
	found1, found2 := false, false
	for _, r := range rules {
		if r.ID == rule1.ID {
			found1 = true
			if r.RuleName != "旧文件归档" {
				t.Errorf("规则1名称错误: %s", r.RuleName)
			}
		}
		if r.ID == rule2.ID {
			found2 = true
			if r.ConditionType != ConditionTypeFileType {
				t.Errorf("规则2条件类型错误: %s", r.ConditionType)
			}
		}
	}
	if !found1 || !found2 {
		t.Error("未找到预期的规则")
	}
}

func TestEvaluateAgeCondition(t *testing.T) {
	engine, _ := setupTestRulesEngine(t)

	rule := &TieringRule{
		ID:            "test-age",
		RuleName:      "旧文件规则",
		TierType:      TierTypeHDD,
		ConditionType: ConditionTypeAge,
		Threshold:     30,
		Enabled:       true,
	}

	// 文件年龄 60 天 - 应匹配
	oldFile := &FileAccessRecord{
		Path:    "/data/old_file.txt",
		ModTime: time.Now().Add(-60 * 24 * time.Hour),
	}
	if !engine.Evaluate(oldFile, rule) {
		t.Error("60天文件应匹配30天阈值规则")
	}

	// 文件年龄 10 天 - 不应匹配
	newFile := &FileAccessRecord{
		Path:    "/data/new_file.txt",
		ModTime: time.Now().Add(-10 * 24 * time.Hour),
	}
	if engine.Evaluate(newFile, rule) {
		t.Error("10天文件不应匹配30天阈值规则")
	}
}

func TestEvaluateFileTypeCondition(t *testing.T) {
	engine, _ := setupTestRulesEngine(t)

	rule := &TieringRule{
		ID:            "test-type",
		RuleName:      "视频文件规则",
		TierType:      TierTypeCloud,
		ConditionType: ConditionTypeFileType,
		FilePattern:   ".mp4,.mkv,.avi",
		Enabled:       true,
	}

	// 匹配的文件类型
	mp4File := &FileAccessRecord{
		Path: "/media/movie.mp4",
	}
	if !engine.Evaluate(mp4File, rule) {
		t.Error(".mp4 文件应匹配视频规则")
	}

	// 不匹配的文件类型
	txtFile := &FileAccessRecord{
		Path: "/docs/readme.txt",
	}
	if engine.Evaluate(txtFile, rule) {
		t.Error(".txt 文件不应匹配视频规则")
	}
}

func TestEvaluateAccessFrequencyCondition(t *testing.T) {
	engine, _ := setupTestRulesEngine(t)

	rule := &TieringRule{
		ID:            "test-freq",
		RuleName:      "热数据提升",
		TierType:      TierTypeSSD,
		ConditionType: ConditionTypeAccessFrequency,
		Threshold:     5,
		Enabled:       true,
	}

	// 高频访问文件 - 应匹配
	hotFile := &FileAccessRecord{
		Path:        "/data/hot_data.db",
		AccessCount: 100,
		AccessTime:  time.Now().Add(-10 * 24 * time.Hour),
	}
	if !engine.Evaluate(hotFile, rule) {
		t.Error("高频访问文件应匹配频率规则")
	}

	// 低频访问文件 - 不应匹配
	coldFile := &FileAccessRecord{
		Path:        "/data/cold_data.bak",
		AccessCount: 1,
		AccessTime:  time.Now().Add(-60 * 24 * time.Hour),
	}
	if engine.Evaluate(coldFile, rule) {
		t.Error("低频访问文件不应匹配频率规则")
	}
}

func TestRemoveRule(t *testing.T) {
	engine, _ := setupTestRulesEngine(t)

	rule, err := engine.AddRule(TieringRule{
		RuleName:      "临时规则",
		TierType:      TierTypeHDD,
		ConditionType: ConditionTypeAge,
		Threshold:     7,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("添加规则失败: %v", err)
	}

	if len(engine.ListRules()) != 1 {
		t.Fatal("添加后应有1条规则")
	}

	if err := engine.RemoveRule(rule.ID); err != nil {
		t.Fatalf("删除规则失败: %v", err)
	}

	if len(engine.ListRules()) != 0 {
		t.Fatal("删除后应有0条规则")
	}

	// 删除不存在的规则应报错
	if err := engine.RemoveRule("nonexistent"); err == nil {
		t.Error("删除不存在的规则应报错")
	}
}

func TestUpdateRule(t *testing.T) {
	engine, _ := setupTestRulesEngine(t)

	rule, err := engine.AddRule(TieringRule{
		RuleName:      "原始规则",
		TierType:      TierTypeHDD,
		ConditionType: ConditionTypeAge,
		Threshold:     30,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("添加规则失败: %v", err)
	}

	// 更新规则
	err = engine.UpdateRule(rule.ID, TieringRule{
		RuleName:      "更新后的规则",
		TierType:      TierTypeCloud,
		ConditionType: ConditionTypeAge,
		Threshold:     60,
		Enabled:       false,
	})
	if err != nil {
		t.Fatalf("更新规则失败: %v", err)
	}

	updated, _ := engine.GetRule(rule.ID)
	if updated.RuleName != "更新后的规则" {
		t.Errorf("规则名称未更新: %s", updated.RuleName)
	}
	if updated.Threshold != 60 {
		t.Errorf("阈值未更新: %d", updated.Threshold)
	}
	if updated.Enabled {
		t.Error("启用状态未更新")
	}
}

func TestRulesPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	config := DefaultPolicyEngineConfig()
	manager := NewManager(configPath, config)
	_ = manager.Initialize()

	// 创建并保存规则
	engine1 := NewRulesEngine(manager, tmpDir)
	_ = engine1.Initialize()

	_, _ = engine1.AddRule(TieringRule{
		RuleName:      "持久化测试规则",
		TierType:      TierTypeHDD,
		ConditionType: ConditionTypeAge,
		Threshold:     15,
		Enabled:       true,
	})

	// 验证文件存在
	rulesFile := filepath.Join(tmpDir, "tiering_rules.json")
	if _, err := os.Stat(rulesFile); os.IsNotExist(err) {
		t.Fatal("规则文件未创建")
	}

	// 新实例加载规则
	engine2 := NewRulesEngine(manager, tmpDir)
	if err := engine2.Initialize(); err != nil {
		t.Fatalf("加载规则失败: %v", err)
	}

	rules := engine2.ListRules()
	if len(rules) != 1 {
		t.Fatalf("期望加载1条规则，实际 %d 条", len(rules))
	}
	if rules[0].RuleName != "持久化测试规则" {
		t.Errorf("规则名称不匹配: %s", rules[0].RuleName)
	}
}

func TestDisabledRuleNotEvaluated(t *testing.T) {
	engine, _ := setupTestRulesEngine(t)

	rule := &TieringRule{
		ID:            "disabled",
		RuleName:      "禁用规则",
		TierType:      TierTypeHDD,
		ConditionType: ConditionTypeAge,
		Threshold:     1,
		Enabled:       false,
	}

	file := &FileAccessRecord{
		Path:    "/data/old.txt",
		ModTime: time.Now().Add(-100 * 24 * time.Hour),
	}

	if engine.Evaluate(file, rule) {
		t.Error("禁用的规则不应匹配任何文件")
	}
}
