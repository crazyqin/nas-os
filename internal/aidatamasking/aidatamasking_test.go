package aidatamasking

import (
	"testing"

	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(zap.NewNop(), nil)
}

func TestNewManager(t *testing.T) {
	m := setupTestManager(t)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.engine == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestMaskText_IDCard(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "18位身份证号",
			input:    "我的身份证号是110101199001011234",
			expected: "我的身份证号是110101********1234",
		},
		{
			name:     "身份证号带X",
			input:    "身份证:11010119900101123X",
			expected: "身份证:110101********123X",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := m.MaskText(&MaskingRequest{
				Text: tt.input,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.MaskedText != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, resp.MaskedText)
			}
		})
	}
}

func TestMaskText_Phone(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "手机号",
			input:    "联系电话：13812345678",
			expected: "联系电话：138****5678",
		},
		{
			name:     "多个手机号",
			input:    "手机1:13812345678,手机2:15987654321",
			expected: "手机1:138****5678,手机2:159****4321",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := m.MaskText(&MaskingRequest{
				Text: tt.input,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.MaskedText != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, resp.MaskedText)
			}
		})
	}
}

func TestMaskText_BankCard(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "16位银行卡",
			input:    "卡号：6222021234567890",
			expected: "卡号：6222********7890",
		},
		{
			name:     "19位银行卡",
			input:    "6222021234567890123",
			expected: "6222***********0123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := m.MaskText(&MaskingRequest{
				Text: tt.input,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.MaskedText != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, resp.MaskedText)
			}
		})
	}
}

func TestMaskText_Email(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "邮箱地址",
			input:    "邮箱：test@example.com",
			expected: "邮箱：tes*************",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := m.MaskText(&MaskingRequest{
				Text: tt.input,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.MaskedText != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, resp.MaskedText)
			}
		})
	}
}

func TestMaskText_IPAddress(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "IPv4地址",
			input:    "服务器IP：192.168.1.100",
			expected: "服务器IP：*************",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := m.MaskText(&MaskingRequest{
				Text: tt.input,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.MaskedText != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, resp.MaskedText)
			}
		})
	}
}

func TestMaskText_LicensePlate(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "车牌号",
			input:    "车牌：京A12345",
			expected: "车牌：京A***45",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := m.MaskText(&MaskingRequest{
				Text: tt.input,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.MaskedText != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, resp.MaskedText)
			}
		})
	}
}

func TestMaskText_Mixed(t *testing.T) {
	m := setupTestManager(t)

	input := "用户信息：姓名张三，手机13812345678，邮箱zhangsan@example.com，身份证110101199001011234"
	resp, err := m.MaskText(&MaskingRequest{
		Text: input,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证敏感数据被脱敏
	if resp.MaskedText == input {
		t.Error("expected text to be masked")
	}

	// 验证摘要
	if resp.Summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if resp.Summary.TotalMatches < 3 {
		t.Errorf("expected at least 3 matches, got %d", resp.Summary.TotalMatches)
	}
}

func TestMaskText_TestMode(t *testing.T) {
	m := setupTestManager(t)

	input := "手机：13812345678"
	resp, err := m.MaskText(&MaskingRequest{
		Text:     input,
		TestMode: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Results) == 0 {
		t.Error("expected results in test mode")
	}

	for _, r := range resp.Results {
		if r.Original == "" {
			t.Error("expected non-empty original")
		}
		if r.Masked == "" {
			t.Error("expected non-empty masked")
		}
		if r.StartPos < 0 {
			t.Error("expected valid start position")
		}
		if r.EndPos <= r.StartPos {
			t.Error("expected valid end position")
		}
	}
}

func TestBatchMaskText(t *testing.T) {
	m := setupTestManager(t)

	req := &BatchMaskingRequest{
		Texts: []string{
			"手机：13812345678",
			"没有敏感数据的文本",
			"身份证：110101199001011234",
		},
	}

	resp, err := m.BatchMaskText(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.TotalTexts != 3 {
		t.Errorf("expected 3 total texts, got %d", resp.TotalTexts)
	}
	if resp.TotalMasked < 2 {
		t.Errorf("expected at least 2 masked texts, got %d", resp.TotalMasked)
	}
	if len(resp.Results) != 3 {
		t.Errorf("expected 3 results, got %d", len(resp.Results))
	}
}

func TestProcessAIPrompt(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		name           string
		prompt         string
		expectMasked   bool
	}{
		{
			name:         "包含敏感数据",
			prompt:       "请帮我查询用户13812345678的信息",
			expectMasked: true,
		},
		{
			name:         "不含敏感数据",
			prompt:       "今天天气怎么样？",
			expectMasked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := m.ProcessAIPrompt(&AIPromptRequest{
				Prompt: tt.prompt,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectMasked {
				if !resp.HasSensitiveData {
					t.Error("expected to detect sensitive data")
				}
				if !resp.MaskingApplied {
					t.Error("expected masking to be applied")
				}
				if resp.MaskedPrompt == tt.prompt {
					t.Error("expected prompt to be masked")
				}
			} else {
				if resp.HasSensitiveData {
					t.Error("unexpected sensitive data detected")
				}
				if resp.MaskingApplied {
					t.Error("unexpected masking applied")
				}
			}
		})
	}
}

func TestCustomRules(t *testing.T) {
	m := setupTestManager(t)

	// 添加自定义规则
	rule := &MaskingRule{
		Name:        "自定义规则",
		DataType:    DataTypePhone,
		Strategy:    StrategyReplace,
		Enabled:     true,
		Replacement: "[手机号已脱敏]",
	}

	if err := m.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	// 验证规则已添加
	rules := m.ListRules()
	found := false
	for _, r := range rules {
		if r.Name == "自定义规则" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find custom rule")
	}

	// 使用自定义规则脱敏
	resp, err := m.MaskText(&MaskingRequest{
		Text:  "手机：13812345678",
		Rules: []*MaskingRule{rule},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.MaskedText != "手机：[手机号已脱敏]" {
		t.Errorf("expected '手机：[手机号已脱敏]', got '%s'", resp.MaskedText)
	}

	// 更新规则
	rule.Replacement = "[已脱敏]"
	if err := m.UpdateRule(rule.ID, rule); err != nil {
		t.Fatalf("UpdateRule failed: %v", err)
	}

	// 删除规则
	if err := m.DeleteRule(rule.ID); err != nil {
		t.Fatalf("DeleteRule failed: %v", err)
	}

	// 验证规则已删除
	_, err = m.GetRule(rule.ID)
	if err == nil {
		t.Error("expected error for deleted rule")
	}
}

func TestHashStrategy(t *testing.T) {
	m := setupTestManager(t)

	rule := &MaskingRule{
		Name:     "哈希规则",
		DataType: DataTypePhone,
		Strategy: StrategyHash,
		Enabled:  true,
	}

	m.AddRule(rule)

	resp, err := m.MaskText(&MaskingRequest{
		Text:  "手机：13812345678",
		Rules: []*MaskingRule{rule},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 哈希值应该是固定的
	if resp.MaskedText == "手机：13812345678" {
		t.Error("expected text to be hashed")
	}
}

func TestTruncateStrategy(t *testing.T) {
	m := setupTestManager(t)

	rule := &MaskingRule{
		Name:       "截断规则",
		DataType:   DataTypePhone,
		Strategy:   StrategyTruncate,
		Enabled:    true,
		KeepPrefix: 3,
		KeepSuffix: 4,
	}

	m.AddRule(rule)

	resp, err := m.MaskText(&MaskingRequest{
		Text:  "手机：13812345678",
		Rules: []*MaskingRule{rule},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.MaskedText != "手机：138...5678" {
		t.Errorf("expected '手机：138...5678', got '%s'", resp.MaskedText)
	}
}

func TestRedactStrategy(t *testing.T) {
	m := setupTestManager(t)

	rule := &MaskingRule{
		Name:     "删除规则",
		DataType: DataTypePhone,
		Strategy: StrategyRedact,
		Enabled:  true,
	}

	m.AddRule(rule)

	resp, err := m.MaskText(&MaskingRequest{
		Text:  "手机：13812345678",
		Rules: []*MaskingRule{rule},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 完全删除手机号
	if resp.MaskedText != "手机：" {
		t.Errorf("expected '手机：', got '%s'", resp.MaskedText)
	}
}

func TestHasSensitiveData(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{
			name:     "包含手机号",
			text:     "联系13812345678",
			expected: true,
		},
		{
			name:     "不包含敏感数据",
			text:     "这是一段普通文本",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			has, _ := m.HasSensitiveData(tt.text)
			if has != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, has)
			}
		})
	}
}

func TestLogs(t *testing.T) {
	m := setupTestManager(t)

	// 执行几次脱敏操作
	for i := 0; i < 5; i++ {
		m.MaskText(&MaskingRequest{
			Text: "测试文本13812345678",
		})
	}

	// 获取脱敏日志
	logs := m.GetMaskingLogs(10)
	if len(logs) < 5 {
		t.Errorf("expected at least 5 logs, got %d", len(logs))
	}

	// 获取审计日志
	auditLogs := m.GetAuditLogs(10)
	if len(auditLogs) < 5 {
		t.Errorf("expected at least 5 audit logs, got %d", len(auditLogs))
	}
}

func TestDisabledEngine(t *testing.T) {
	cfg := DefaultMaskingEngineConfig()
	cfg.Enabled = false
	m := NewManager(zap.NewNop(), cfg)

	_, err := m.MaskText(&MaskingRequest{
		Text: "测试文本",
	})
	if err == nil {
		t.Error("expected error when disabled")
	}
}

func TestValidDataTypes(t *testing.T) {
	types := ValidDataTypes()
	if len(types) == 0 {
		t.Error("expected non-empty data types")
	}

	if !IsValidDataType(DataTypePhone) {
		t.Error("expected phone to be valid")
	}
	if IsValidDataType("invalid") {
		t.Error("expected 'invalid' to be invalid")
	}
}

func TestValidStrategies(t *testing.T) {
	strategies := ValidStrategies()
	if len(strategies) == 0 {
		t.Error("expected non-empty strategies")
	}

	if !IsValidStrategy(StrategyMask) {
		t.Error("expected mask to be valid")
	}
	if IsValidStrategy("invalid") {
		t.Error("expected 'invalid' to be invalid")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultMaskingEngineConfig()

	if !cfg.Enabled {
		t.Error("expected enabled to be true")
	}
	if cfg.DefaultStrategy != StrategyMask {
		t.Errorf("expected default strategy %s, got %s", StrategyMask, cfg.DefaultStrategy)
	}
	if cfg.MaxTextLength != 1024*1024 {
		t.Errorf("expected max text length 1048576, got %d", cfg.MaxTextLength)
	}
	if !cfg.LogEnabled {
		t.Error("expected log enabled to be true")
	}
	if !cfg.AuditEnabled {
		t.Error("expected audit enabled to be true")
	}
	if cfg.AIIntegration == nil {
		t.Fatal("expected non-nil AI integration config")
	}
	if !cfg.AIIntegration.Enabled {
		t.Error("expected AI integration enabled")
	}
}

func TestPatternStats(t *testing.T) {
	stats := GetPatternStats()
	if len(stats) == 0 {
		t.Error("expected non-empty pattern stats")
	}

	if stats[DataTypePhone] == 0 {
		t.Error("expected phone pattern")
	}
	if stats[DataTypeIDCard] == 0 {
		t.Error("expected ID card pattern")
	}
}

func TestEngineRuleManagement(t *testing.T) {
	engine := NewEngine(nil)

	// List default rules
	rules := engine.ListRules()
	if len(rules) == 0 {
		t.Error("expected default rules")
	}

	// Add custom rule
	rule := &MaskingRule{
		Name:        "测试规则",
		DataType:    DataTypeName,
		Strategy:    StrategyMask,
		Enabled:     true,
		KeepPrefix:  1,
		KeepSuffix:  0,
		MaskChar:    "*",
	}

	if err := engine.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	if rule.ID == "" {
		t.Error("expected non-empty rule ID")
	}

	// Get rule
	got, err := engine.GetRule(rule.ID)
	if err != nil {
		t.Fatalf("GetRule failed: %v", err)
	}
	if got.Name != "测试规则" {
		t.Errorf("expected name '测试规则', got '%s'", got.Name)
	}

	// Delete rule
	if err := engine.DeleteRule(rule.ID); err != nil {
		t.Fatalf("DeleteRule failed: %v", err)
	}

	_, err = engine.GetRule(rule.ID)
	if err == nil {
		t.Error("expected error for deleted rule")
	}
}

func TestAddCustomPattern(t *testing.T) {
	pattern, err := AddCustomPattern(DataTypePhone, `custom_pattern`, "自定义模式")
	if err != nil {
		t.Fatalf("AddCustomPattern failed: %v", err)
	}

	if pattern == nil {
		t.Fatal("expected non-nil pattern")
	}
	if pattern.Name != "自定义模式" {
		t.Errorf("expected name '自定义模式', got '%s'", pattern.Name)
	}
}

func TestUpdateConfig(t *testing.T) {
	m := setupTestManager(t)

	newCfg := DefaultMaskingEngineConfig()
	newCfg.MaxTextLength = 100

	m.UpdateConfig(newCfg)

	cfg := m.GetConfig()
	if cfg.MaxTextLength != 100 {
		t.Errorf("expected max text length 100, got %d", cfg.MaxTextLength)
	}
}
