package aideidentification

import (
	"testing"
)

func TestNewDeidentificationManager(t *testing.T) {
	manager := NewDeidentificationManager(nil)
	if manager == nil {
		t.Fatal("NewDeidentificationManager 返回 nil")
	}

	// 检查内置规则是否初始化
	rules := manager.ListRules()
	if len(rules) == 0 {
		t.Fatal("内置规则未初始化")
	}

	// 检查是否包含手机号规则
	found := false
	for _, rule := range rules {
		if rule.PIIType == PIITypePhone {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("缺少手机号检测规则")
	}
}

func TestDeidentifyPhone(t *testing.T) {
	manager := NewDeidentificationManager(nil)

	tests := []struct {
		input    string
		expected string
	}{
		{"我的手机号是13812345678", "我的手机号是138****5678"},
		{"联系方式：15900001111，请联系", "联系方式：159****1111，请联系"},
		{"没有手机号的文本", "没有手机号的文本"},
	}

	for _, test := range tests {
		result, err := manager.Deidentify(test.input, "")
		if err != nil {
			t.Errorf("脱敏失败: %v", err)
			continue
		}

		if result.RedactedText != test.expected {
			t.Errorf("输入: %s\n期望: %s\n实际: %s", test.input, test.expected, result.RedactedText)
		}
	}
}

func TestDeidentifyEmail(t *testing.T) {
	manager := NewDeidentificationManager(nil)

	input := "邮箱：test@example.com，备用：user123@gmail.com"
	result, err := manager.Deidentify(input, "")
	if err != nil {
		t.Fatalf("脱敏失败: %v", err)
	}

	// 邮箱应该被部分脱敏
	if result.TotalRedacted < 2 {
		t.Errorf("期望至少2个脱敏项，实际 %d", result.TotalRedacted)
	}
}

func TestDeidentifyIDCard(t *testing.T) {
	manager := NewDeidentificationManager(nil)

	input := "身份证号：110101199003076515"
	result, err := manager.Deidentify(input, "")
	if err != nil {
		t.Fatalf("脱敏失败: %v", err)
	}

	if result.TotalRedacted == 0 {
		t.Error("身份证号未被脱敏")
	}
}

func TestDeidentifyIP(t *testing.T) {
	manager := NewDeidentificationManager(nil)

	input := "服务器IP：192.168.1.100"
	result, err := manager.Deidentify(input, "")
	if err != nil {
		t.Fatalf("脱敏失败: %v", err)
	}

	if result.TotalRedacted == 0 {
		t.Error("IP地址未被脱敏")
	}

	// IP应该被掩码脱敏
	if result.RedactedText == input {
		t.Error("IP地址未被替换")
	}
}

func TestCreateRule(t *testing.T) {
	manager := NewDeidentificationManager(nil)

	req := &CreateRuleRequest{
		Name:        "测试规则",
		Description: "测试用规则",
		PIIType:     PIITypePhone,
		Policy:      PolicyMask,
		Pattern:     `1[3-9]\d{9}`,
		Placeholder: "[手机号]",
		Priority:    200,
	}

	rule, err := manager.CreateRule(req)
	if err != nil {
		t.Fatalf("创建规则失败: %v", err)
	}

	if rule.ID == "" {
		t.Error("规则ID为空")
	}

	if rule.Name != "测试规则" {
		t.Errorf("规则名称不匹配: %s", rule.Name)
	}
}

func TestUpdateRule(t *testing.T) {
	manager := NewDeidentificationManager(nil)

	// 先创建规则
	createReq := &CreateRuleRequest{
		Name:    "原始规则",
		PIIType: PIITypePhone,
		Policy:  PolicyMask,
		Pattern: `1[3-9]\d{9}`,
	}

	rule, _ := manager.CreateRule(createReq)

	// 更新规则
	enabled := true
	updateReq := &UpdateRuleRequest{
		ID:      rule.ID,
		Name:    "更新后的规则",
		Enabled: &enabled,
	}

	updated, err := manager.UpdateRule(updateReq)
	if err != nil {
		t.Fatalf("更新规则失败: %v", err)
	}

	if updated.Name != "更新后的规则" {
		t.Errorf("规则名称未更新: %s", updated.Name)
	}
}

func TestDeleteRule(t *testing.T) {
	manager := NewDeidentificationManager(nil)

	// 创建规则
	createReq := &CreateRuleRequest{
		Name:    "待删除规则",
		PIIType: PIITypePhone,
		Policy:  PolicyMask,
		Pattern: `1[3-9]\d{9}`,
	}

	rule, _ := manager.CreateRule(createReq)

	// 删除规则
	err := manager.DeleteRule(rule.ID)
	if err != nil {
		t.Fatalf("删除规则失败: %v", err)
	}

	// 验证已删除
	_, err = manager.GetRule(rule.ID)
	if err == nil {
		t.Error("规则应该已被删除")
	}
}

func TestDeleteBuiltinRule(t *testing.T) {
	manager := NewDeidentificationManager(nil)

	// 尝试删除内置规则
	rules := manager.ListRules()
	for _, rule := range rules {
		if rule.ID[:7] == "builtin" {
			err := manager.DeleteRule(rule.ID)
			if err == nil {
				t.Error("应该禁止删除内置规则")
			}
			break
		}
	}
}

func TestDeidentifyBatch(t *testing.T) {
	manager := NewDeidentificationManager(nil)

	req := &BatchDeidentificationRequest{
		Texts: []string{
			"张三的手机号是13812345678",
			"李四的邮箱是lisi@example.com",
			"王五的身份证号是110101199003076515",
		},
	}

	result, err := manager.DeidentifyBatch(req)
	if err != nil {
		t.Fatalf("批量脱敏失败: %v", err)
	}

	if len(result.Results) != 3 {
		t.Errorf("期望3个结果，实际 %d", len(result.Results))
	}

	if result.Summary.TotalTexts != 3 {
		t.Errorf("汇总文本数不正确: %d", result.Summary.TotalTexts)
	}
}

func TestGetStats(t *testing.T) {
	manager := NewDeidentificationManager(nil)

	// 执行一些脱敏操作
	manager.Deidentify("13812345678", "")
	manager.Deidentify("test@example.com", "")

	stats := manager.GetStats()

	if stats.TotalProcessed != 2 {
		t.Errorf("总处理次数不正确: %d", stats.TotalProcessed)
	}

	if stats.TotalRedacted == 0 {
		t.Error("总脱敏次数为0")
	}
}

func TestGetAuditLog(t *testing.T) {
	manager := NewDeidentificationManager(nil)

	// 执行脱敏操作生成日志
	manager.Deidentify("13812345678", "")

	logs := manager.GetAuditLog(10)
	if len(logs) == 0 {
		t.Error("审计日志为空")
	}
}

func TestPartialMask(t *testing.T) {
	manager := NewDeidentificationManager(nil)

	tests := []struct {
		input    string
		expected string
	}{
		{"13812345678", "138****5678"},
		{"abcdefghij", "abc***ghij"},
		{"ab", "**"},
	}

	for _, test := range tests {
		result := manager.partialMask(test.input)
		if result != test.expected {
			t.Errorf("输入: %s\n期望: %s\n实际: %s", test.input, test.expected, result)
		}
	}
}

func TestApplyPolicy(t *testing.T) {
	manager := NewDeidentificationManager(nil)

	rule := &DeidentificationRule{
		PIIType:     PIITypePhone,
		Placeholder: "[手机号]",
	}

	// 测试 Mask 策略
	rule.Policy = PolicyMask
	result := manager.applyPolicy("13812345678", rule)
	if result != "[手机号]" {
		t.Errorf("Mask策略结果不正确: %s", result)
	}

	// 测试 Hash 策略
	rule.Policy = PolicyHash
	result = manager.applyPolicy("13812345678", rule)
	if len(result) < 10 {
		t.Errorf("Hash策略结果太短: %s", result)
	}

	// 测试 Remove 策略
	rule.Policy = PolicyRemove
	result = manager.applyPolicy("13812345678", rule)
	if result != "" {
		t.Errorf("Remove策略结果不为空: %s", result)
	}

	// 测试 Replace 策略
	rule.Policy = PolicyReplace
	result = manager.applyPolicy("13812345678", rule)
	if result == "" {
		t.Error("Replace策略结果为空")
	}
}

func TestDeidentifyWithCustomRule(t *testing.T) {
	manager := NewDeidentificationManager(nil)

	// 创建自定义规则
	req := &CreateRuleRequest{
		Name:        "自定义手机号规则",
		PIIType:     PIITypePhone,
		Policy:      PolicyMask,
		Pattern:     `1[3-9]\d{9}`,
		Placeholder: "[手机]",
		Priority:    200,
	}

	rule, _ := manager.CreateRule(req)

	// 使用自定义规则脱敏
	result, err := manager.Deidentify("手机号13812345678", rule.ID)
	if err != nil {
		t.Fatalf("脱敏失败: %v", err)
	}

	if result.TotalRedacted == 0 {
		t.Error("自定义规则未生效")
	}
}
