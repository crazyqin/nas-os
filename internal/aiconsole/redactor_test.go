package aiconsole

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== Redactor 单元测试 ====================

func TestRedactor_SetDefaultRules(t *testing.T) {
	r := NewRedactor()
	r.SetDefaultRules()

	rules := r.GetRules()
	assert.NotEmpty(t, rules, "默认规则不应为空")
	assert.True(t, len(rules) >= 6, "至少应有 6 条默认规则")

	// 验证规则按优先级排序
	for i := 1; i < len(rules); i++ {
		assert.GreaterOrEqual(t, rules[i-1].Priority, rules[i].Priority,
			"规则应按优先级降序排列")
	}
}

func TestRedactor_Process_IDCard(t *testing.T) {
	r := NewRedactor()
	r.SetDefaultRules()

	tests := []struct {
		name     string
		input    string
		wantMask bool
	}{
		{
			name:     "有效身份证号",
			input:    "我的身份证号是110101199003071234",
			wantMask: true,
		},
		{
			name:     "身份证号末位X",
			input:    "身份证：11010119900307123X",
			wantMask: true,
		},
		{
			name:     "无效身份证号-位数不对",
			input:    "数字 12345678901234567",
			wantMask: false, // 17位，不符合
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.Process(tt.input)
			assert.True(t, result.HasRedaction == tt.wantMask,
				"HasRedaction 期望 %v，实际 %v", tt.wantMask, result.HasRedaction)
			if tt.wantMask {
				assert.NotEqual(t, tt.input, result.Processed, "处理后文本应有变化")
				assert.Equal(t, 1, result.RedactCount)
			}
		})
	}
}

func TestRedactor_Process_Phone(t *testing.T) {
	r := NewRedactor()
	r.SetDefaultRules()

	input := "电话 13812345678 请拨打"
	result := r.Process(input)

	assert.True(t, result.HasRedaction)
	assert.Equal(t, 1, result.RedactCount)
	assert.Contains(t, result.Processed, "138")
	assert.Contains(t, result.Processed, "5678")
	assert.NotContains(t, result.Processed, "13812345678")
}

func TestRedactor_Process_Email(t *testing.T) {
	r := NewRedactor()
	r.SetDefaultRules()

	input := "邮箱: zhangsan@example.com"
	result := r.Process(input)

	assert.True(t, result.HasRedaction)
	assert.Equal(t, 1, result.RedactCount)
	assert.NotContains(t, result.Processed, "zhangsan@example.com")
	// partial 策略，显示前2字符
	assert.Contains(t, result.Processed, "zh")
}

func TestRedactor_Process_IPAddress(t *testing.T) {
	r := NewRedactor()
	r.SetDefaultRules()

	input := "服务器地址 192.168.1.100"
	result := r.Process(input)

	assert.True(t, result.HasRedaction)
	assert.Equal(t, 1, result.RedactCount)
	assert.NotContains(t, result.Processed, "192.168.1.100")
}

func TestRedactor_Process_BankCard(t *testing.T) {
	r := NewRedactor()
	r.SetDefaultRules()

	input := "卡号 6222021234567890"
	result := r.Process(input)

	assert.True(t, result.HasRedaction)
	// 银行卡 partial 策略：显示前4后4
	assert.Contains(t, result.Processed, "6222")
	assert.Contains(t, result.Processed, "7890")
}

func TestRedactor_Process_MultiplePII(t *testing.T) {
	r := NewRedactor()
	r.SetDefaultRules()

	input := "姓名：张三，手机：13912345678，邮箱：zhang@test.com，身份证：110101199003071234"
	result := r.Process(input)

	assert.True(t, result.HasRedaction)
	assert.GreaterOrEqual(t, result.RedactCount, 3, "至少应检测到手机、邮箱、身份证")
	// 手机号和邮箱都不应原文出现
	assert.NotContains(t, result.Processed, "13912345678")
	assert.NotContains(t, result.Processed, "zhang@test.com")
	assert.NotContains(t, result.Processed, "110101199003071234")
}

func TestRedactor_Process_NoPII(t *testing.T) {
	r := NewRedactor()
	r.SetDefaultRules()

	input := "这是一段普通文本，不包含任何敏感信息。"
	result := r.Process(input)

	assert.False(t, result.HasRedaction)
	assert.Equal(t, 0, result.RedactCount)
	assert.Equal(t, input, result.Processed)
}

func TestRedactor_Process_EmptyText(t *testing.T) {
	r := NewRedactor()
	r.SetDefaultRules()

	result := r.Process("")
	assert.False(t, result.HasRedaction)
	assert.Equal(t, 0, result.RedactCount)
	assert.Equal(t, "", result.Processed)
}

func TestRedactor_HasPII(t *testing.T) {
	r := NewRedactor()
	r.SetDefaultRules()

	assert.True(t, r.HasPII("邮箱 test@example.com"))
	assert.True(t, r.HasPII("电话 13800000000"))
	assert.False(t, r.HasPII("没有敏感数据"))
	assert.False(t, r.HasPII(""))
}

func TestRedactor_LoadCustomRules(t *testing.T) {
	r := NewRedactor()
	customRules := []*RedactRule{
		{
			ID:       "custom_order",
			Name:     "订单号",
			PIIType:  PIICustom,
			Pattern:  `\bORD\d{10}\b`,
			Strategy: StrategyMask,
			MaskChar: "#",
			Enabled:  true,
			Priority: 50,
		},
	}
	err := r.LoadRules(customRules)
	require.NoError(t, err)

	result := r.Process("订单号 ORD1234567890 已发货")
	assert.True(t, result.HasRedaction)
	assert.NotContains(t, result.Processed, "ORD1234567890")
	assert.Contains(t, result.Processed, "##########")
}

func TestRedactor_DisabledRule(t *testing.T) {
	r := NewRedactor()
	rules := []*RedactRule{
		{
			ID:       "disabled_rule",
			Name:     "已禁用规则",
			PIIType:  PIIEmail,
			Pattern:  `[\w.\-]+@[\w.\-]+\.\w+`,
			Strategy: StrategyMask,
			Enabled:  false,
			Priority: 100,
		},
	}
	err := r.LoadRules(rules)
	require.NoError(t, err)

	result := r.Process("邮箱 test@example.com")
	assert.False(t, result.HasRedaction)
	assert.Contains(t, result.Processed, "test@example.com")
}

func TestRedactor_StrategyHash(t *testing.T) {
	r := NewRedactor()
	rules := []*RedactRule{
		{
			ID:       "hash_rule",
			Name:     "哈希规则",
			PIIType:  PIICustom,
			Pattern:  `SECRET_\w+`,
			Strategy: StrategyHash,
			Enabled:  true,
			Priority: 100,
		},
	}
	err := r.LoadRules(rules)
	require.NoError(t, err)

	result := r.Process("密码 SECRET_abc123")
	assert.True(t, result.HasRedaction)
	assert.Contains(t, result.Processed, "[HASH:")
	assert.NotContains(t, result.Processed, "SECRET_abc123")
}

func TestRedactor_StrategyRemove(t *testing.T) {
	r := NewRedactor()
	rules := []*RedactRule{
		{
			ID:          "remove_rule",
			Name:        "移除规则",
			PIIType:     PIICustom,
			Pattern:     `TODO:`,
			Strategy:    StrategyRemove,
			Replacement: "[已移除]",
			Enabled:     true,
			Priority:    100,
		},
	}
	err := r.LoadRules(rules)
	require.NoError(t, err)

	result := r.Process("请忽略 TODO: 这部分")
	assert.True(t, result.HasRedaction)
	assert.Contains(t, result.Processed, "[已移除]")
	assert.NotContains(t, result.Processed, "TODO:")
}

func TestRedactor_PartialStrategy_EdgeCases(t *testing.T) {
	r := NewRedactor()
	rules := []*RedactRule{
		{
			ID:        "short_partial",
			Name:      "短文本部分显示",
			PIIType:   PIICustom,
			Pattern:   `\bAB\b`,
			Strategy:  StrategyPartial,
			ShowFirst: 5,
			ShowLast:  5,
			MaskChar:  "*",
			Enabled:   true,
			Priority:  100,
		},
	}
	err := r.LoadRules(rules)
	require.NoError(t, err)

	// 当 showFirst + showLast >= 文本长度时，全部显示
	result := r.Process("AB")
	assert.True(t, result.HasRedaction)
	// "AB" 长度 2，showFirst(5) + showLast(5) >= 2，所以不掩码
	assert.Equal(t, "AB", result.Processed)
}

func TestRedactor_RedactResult_Details(t *testing.T) {
	r := NewRedactor()
	r.SetDefaultRules()

	input := "联系 13800000000"
	result := r.Process(input)

	require.Len(t, result.Redactions, 1)
	detail := result.Redactions[0]
	assert.Equal(t, PIIPhone, detail.PIIType)
	assert.Equal(t, StrategyPartial, detail.Strategy)
	assert.NotEmpty(t, detail.Replaced)
	assert.NotEmpty(t, detail.RuleID)
	assert.NotEmpty(t, detail.RuleName)
}

func TestRedactor_InvalidRegex(t *testing.T) {
	r := NewRedactor()
	rules := []*RedactRule{
		{
			ID:       "bad_regex",
			Name:     "无效正则",
			PIIType:  PIICustom,
			Pattern:  `[invalid`,
			Strategy: StrategyMask,
			Enabled:  true,
			Priority: 100,
		},
	}
	err := r.LoadRules(rules)
	assert.Error(t, err, "无效正则应返回错误")
}

func TestRedactor_ConcurrentProcess(t *testing.T) {
	r := NewRedactor()
	r.SetDefaultRules()

	// 并发安全测试
	done := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		go func(idx int) {
			result := r.Process("邮箱 test@test.com 手机 13800000000")
			assert.True(t, result.HasRedaction)
			done <- true
		}(i)
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestRedactor_ChineseName(t *testing.T) {
	r := NewRedactor()
	// 加载中文姓名规则（使用简单匹配方式，Go 不支持前瞻断言）
	rules := []*RedactRule{
		{
			ID:       "chinese_name",
			Name:     "中文姓名",
			PIIType:  PIIName,
			Pattern:  `[\x{4e00}-\x{9fff}]{2}先生`,
			Strategy: StrategyPartial,
			ShowFirst: 1, ShowLast: 0,
			MaskChar:  "*",
			Enabled:   true,
			Priority:  70,
		},
	}
	err := r.LoadRules(rules)
	require.NoError(t, err)

	input := "请转告张三先生"
	result := r.Process(input)
	assert.True(t, result.HasRedaction)
	assert.Contains(t, result.Processed, "张")
	assert.NotContains(t, result.Processed, "张三先")
}

func TestRedactor_Passport(t *testing.T) {
	r := NewRedactor()
	r.SetDefaultRules()

	input := "护照号码 E12345678"
	result := r.Process(input)

	assert.True(t, result.HasRedaction)
	assert.NotContains(t, result.Processed, "E12345678")
}

// ==================== types 测试 ====================

func TestPIITypeConstants(t *testing.T) {
	assert.Equal(t, PIIType("email"), PIIEmail)
	assert.Equal(t, PIIType("phone"), PIIPhone)
	assert.Equal(t, PIIType("id_card"), PIIIDCard)
	assert.Equal(t, PIIType("bank_card"), PIIBankCard)
	assert.Equal(t, PIIType("name"), PIIName)
	assert.Equal(t, PIIType("passport"), PIIPassport)
	assert.Equal(t, PIIType("ip_address"), PIIIPAddress)
	assert.Equal(t, PIIType("custom"), PIICustom)
}

func TestRedactStrategyConstants(t *testing.T) {
	assert.Equal(t, RedactStrategy("mask"), StrategyMask)
	assert.Equal(t, RedactStrategy("partial"), StrategyPartial)
	assert.Equal(t, RedactStrategy("hash"), StrategyHash)
	assert.Equal(t, RedactStrategy("remove"), StrategyRemove)
}

func TestModelProviderConstants(t *testing.T) {
	assert.Equal(t, ModelProvider("openai_compat"), ProviderOpenAI)
	assert.Equal(t, ModelProvider("local"), ProviderLocal)
	assert.Equal(t, ModelProvider("custom"), ProviderCustom)
}

// ==================== store 工具函数测试 ====================

func TestBoolToInt(t *testing.T) {
	assert.Equal(t, 1, boolToInt(true))
	assert.Equal(t, 0, boolToInt(false))
}

// ==================== handlers 工具函数测试 ====================

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{"长 key", "sk-1234567890abcdef", "sk-1****cdef"},
		{"短 key", "abc", "****"},
		{"刚好 8 位", "12345678", "****"},
		{"9 位", "123456789", "1234****6789"},
		{"空 key", "", "****"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskAPIKey(tt.key)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ==================== BuildAuditWhere 测试 ====================

func TestBuildAuditWhere_EmptyFilter(t *testing.T) {
	where, args := buildAuditWhere(AuditQueryFilter{})
	assert.Empty(t, where)
	assert.Empty(t, args)
}

func TestBuildAuditWhere_UserFilter(t *testing.T) {
	where, args := buildAuditWhere(AuditQueryFilter{UserID: "user123"})
	assert.Contains(t, where, "user_id = ?")
	assert.Equal(t, []interface{}{"user123"}, args)
}

func TestBuildAuditWhere_MultipleFilters(t *testing.T) {
	success := true
	filter := AuditQueryFilter{
		UserID:  "user1",
		Action:  "chat",
		Success: &success,
	}
	where, args := buildAuditWhere(filter)
	assert.Contains(t, where, "user_id = ?")
	assert.Contains(t, where, "action = ?")
	assert.Contains(t, where, "success = ?")
	assert.Len(t, args, 3)
	assert.Equal(t, "user1", args[0])
	assert.Equal(t, "chat", args[1])
	assert.Equal(t, 1, args[2]) // true -> 1
}

func TestBuildAuditWhere_SuccessFalse(t *testing.T) {
	success := false
	filter := AuditQueryFilter{Success: &success}
	where, args := buildAuditWhere(filter)
	assert.Contains(t, where, "success = ?")
	assert.Equal(t, 0, args[0]) // false -> 0
}

// ==================== validatePattern 测试 ====================

func TestValidatePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{"合法正则", `\d+`, false},
		{"合法复杂正则", `\b[1-9]\d{5}\b`, false},
		{"无效正则", `[invalid`, true},
		{"空正则", "", false}, // 空正则合法，匹配空串
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validatePattern(tt.pattern)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ==================== Redactor 集成测试 ====================

func TestRedactor_Integration_RealWorldText(t *testing.T) {
	r := NewRedactor()
	r.SetDefaultRules()

	input := `用户信息：
姓名：李四
手机：13912345678
邮箱：lisi@company.com
身份证：110101199501011234
银行卡：6222021234567890
IP：10.0.0.1
护照：E12345678
这是一段普通的说明文字，不包含敏感信息。`

	result := r.Process(input)

	// 验证所有敏感信息被脱敏
	assert.NotContains(t, result.Processed, "13912345678")
	assert.NotContains(t, result.Processed, "lisi@company.com")
	assert.NotContains(t, result.Processed, "110101199501011234")
	assert.NotContains(t, result.Processed, "6222021234567890")
	assert.NotContains(t, result.Processed, "10.0.0.1")
	assert.NotContains(t, result.Processed, "E12345678")

	// 普通文本应保留
	assert.Contains(t, result.Processed, "用户信息")
	assert.Contains(t, result.Processed, "普通的说明文字")

	// 应有多条脱敏记录
	assert.GreaterOrEqual(t, result.RedactCount, 6)
	assert.True(t, result.HasRedaction)
}

func TestRedactor_Integration_MaskCharVariants(t *testing.T) {
	r := NewRedactor()
	rules := []*RedactRule{
		{
			ID:        "custom_mask",
			Name:      "自定义掩码",
			PIIType:   PIICustom,
			Pattern:   `\bTEST\w+\b`,
			Strategy:  StrategyMask,
			MaskChar:  "#",
			Enabled:   true,
			Priority:  100,
		},
	}
	err := r.LoadRules(rules)
	require.NoError(t, err)

	result := r.Process("替换 TEST123456")
	assert.Contains(t, result.Processed, "##########")
}

func TestRedactor_Integration_OrderingPriority(t *testing.T) {
	// 高优先级规则应先处理
	r := NewRedactor()
	rules := []*RedactRule{
		{
			ID:       "low_priority",
			Name:     "低优先级",
			PIIType:  PIICustom,
			Pattern:  `\d+`,
			Strategy: StrategyMask,
			MaskChar: "L",
			Enabled:  true,
			Priority: 10,
		},
		{
			ID:       "high_priority",
			Name:     "高优先级",
			PIIType:  PIICustom,
			Pattern:  `\d{3,}`,
			Strategy: StrategyMask,
			MaskChar: "H",
			Enabled:  true,
			Priority: 100,
		},
	}
	err := r.LoadRules(rules)
	require.NoError(t, err)

	// "12345" 应该先被高优先级规则（\d{3,}）处理，全部替换为 H
	result := r.Process("数字 12345")
	// 高优先级先匹配 \d{3,}，替换成 HHHHH
	// 然后低优先级 \d+ 在剩余文本中无匹配（已被替换为字母）
	assert.NotContains(t, result.Processed, "12345")
}

// 确保 strings 包被使用（某些 linter 需要）
var _ = strings.TrimSpace
