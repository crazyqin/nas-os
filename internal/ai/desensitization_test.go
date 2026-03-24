// Package ai provides data desensitization tests
package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== Desensitizer Core Tests ====================

func TestNewDesensitizer(t *testing.T) {
	d := NewDesensitizer()
	require.NotNil(t, d)
	require.NotNil(t, d.tokenStore)
	assert.NotEmpty(t, d.rules)
}

func TestNewDesensitizerWithRules(t *testing.T) {
	customRules := []DesensitizationRule{
		{
			ID:       "custom_1",
			Name:     "Custom Rule",
			Type:     PIICustom,
			Pattern:  `\bCUSTOM\d+\b`,
			Strategy: StrategyMask,
			Enabled:  true,
			Priority: 50,
		},
	}

	d := NewDesensitizerWithRules(customRules)
	require.NotNil(t, d)
	assert.Len(t, d.rules, 1)
	assert.Equal(t, "custom_1", d.rules[0].ID)
}

// ==================== Default Rules Tests ====================

func TestDefaultDesensitizationRules(t *testing.T) {
	rules := DefaultDesensitizationRules()

	assert.NotEmpty(t, rules)

	// Verify priority ordering (descending)
	for i := 1; i < len(rules); i++ {
		assert.GreaterOrEqual(t, rules[i-1].Priority, rules[i].Priority,
			"Rules should be sorted by priority descending")
	}

	// Verify essential rules exist
	ruleMap := make(map[string]bool)
	for _, r := range rules {
		ruleMap[r.ID] = true
	}

	expectedRules := []string{"id_card_cn", "credit_card", "phone_cn", "email", "ip_address"}
	for _, id := range expectedRules {
		assert.True(t, ruleMap[id], "Missing expected rule: %s", id)
	}
}

// ==================== Strategy Tests ====================

func TestStrategyMask(t *testing.T) {
	d := NewDesensitizer()

	rule := DesensitizationRule{
		ID:       "test_mask",
		Type:     PIICustom,
		Pattern:  `\bSECRET\b`,
		Strategy: StrategyMask,
		MaskChar: "*",
		Enabled:  true,
		Priority: 100,
	}
	d.AddRule(rule)

	result := d.Process("This is SECRET data")
	assert.Contains(t, result.Processed, "******")
	assert.NotContains(t, result.Processed, "SECRET")
	assert.Equal(t, 1, result.RedactionCount)
}

func TestStrategyPartial(t *testing.T) {
	d := NewDesensitizer()

	// Test with phone number (uses partial by default)
	result := d.Process("我的手机号是13812345678")

	assert.Contains(t, result.Processed, "138****5678")
	assert.NotContains(t, result.Processed, "13812345678")
	assert.Equal(t, 1, result.RedactionCount)

	// Verify redaction details
	require.Len(t, result.Redactions, 1)
	assert.Equal(t, PIIPhone, result.Redactions[0].Type)
	assert.Equal(t, StrategyPartial, result.Redactions[0].Strategy)
}

func TestStrategyHash(t *testing.T) {
	d := NewDesensitizerWithRules([]DesensitizationRule{
		{
			ID:       "test_hash",
			Type:     PIICustom,
			Pattern:  `\bHASHME\b`,
			Strategy: StrategyHash,
			Enabled:  true,
			Priority: 100,
		},
	})

	result := d.Process("Please hash HASHME value")
	assert.Contains(t, result.Processed, "[HASH:")
	assert.NotContains(t, result.Processed, "HASHME")
}

func TestStrategyRedact(t *testing.T) {
	d := NewDesensitizerWithRules([]DesensitizationRule{
		{
			ID:       "test_redact",
			Type:     PIICustom,
			Pattern:  `\bREDACTME\b`,
			Strategy: StrategyRedact,
			Enabled:  true,
			Priority: 100,
		},
	})

	result := d.Process("Please redact REDACTME value")
	assert.Contains(t, result.Processed, "[REDACTED]")
	assert.NotContains(t, result.Processed, "REDACTME")
}

func TestStrategyTokenize(t *testing.T) {
	d := NewDesensitizerWithRules([]DesensitizationRule{
		{
			ID:       "test_tokenize",
			Type:     PIICustom,
			Pattern:  `\bTOKENIZE\b`,
			Strategy: StrategyTokenize,
			Enabled:  true,
			Priority: 100,
		},
	})

	result := d.Process("Please tokenize TOKENIZE value")
	assert.Contains(t, result.Processed, "[CUSTOM_TOKEN_")
	assert.NotContains(t, result.Processed, "TOKENIZE")
}

// ==================== PII Type Tests ====================

func TestPII_IDCard(t *testing.T) {
	d := NewDesensitizer()

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard ID card",
			input:    "身份证号：110101199001011234",
			expected: "1**************4", // showFirst=1, showLast=1
		},
		{
			name:     "ID card with X suffix",
			input:    "身份证：44030419951212345X",
			expected: "4**************X",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := d.Process(tc.input)
			assert.Contains(t, result.Processed, tc.expected)
			assert.NotContains(t, result.Processed, tc.input[strings.Index(tc.input, "1"):])
		})
	}
}

func TestPII_Phone(t *testing.T) {
	d := NewDesensitizer()

	result := d.Process("联系电话：13812345678")

	// Should partially mask: showFirst=3, showLast=4
	assert.Contains(t, result.Processed, "138****5678")
	assert.Equal(t, 1, result.RedactionCount)
	assert.Equal(t, PIIPhone, result.Redactions[0].Type)
}

func TestPII_Email(t *testing.T) {
	d := NewDesensitizer()

	result := d.Process("邮箱地址：test@example.com")

	// Should partially mask: showFirst=2
	assert.Contains(t, result.Processed, "te***")
	assert.NotContains(t, result.Processed, "test@example.com")
}

func TestPII_CreditCard(t *testing.T) {
	d := NewDesensitizer()

	result := d.Process("信用卡：6222021234567890")

	// Should partially mask: showFirst=4, showLast=4
	assert.Contains(t, result.Processed, "6222********7890")
	assert.Equal(t, 1, result.RedactionCount)
}

func TestPII_IPAddress(t *testing.T) {
	d := NewDesensitizer()

	result := d.Process("服务器IP：192.168.1.100")

	// Should partially mask: showFirst=2
	assert.Contains(t, result.Processed, "19*****")
	assert.NotContains(t, result.Processed, "192.168.1.100")
}

func TestPII_Multiple(t *testing.T) {
	d := NewDesensitizer()

	input := "姓名：张三，电话：13812345678，邮箱：test@example.com，身份证：110101199001011234"
	result := d.Process(input)

	assert.Equal(t, 4, result.RedactionCount)

	// Verify no original PII in output
	assert.NotContains(t, result.Processed, "13812345678")
	assert.NotContains(t, result.Processed, "test@example.com")
	assert.NotContains(t, result.Processed, "110101199001011234")
}

// ==================== ProcessWithContext Tests ====================

func TestProcessWithContext(t *testing.T) {
	d := NewDesensitizer()
	ctx := context.Background()

	result := d.ProcessWithContext(ctx, "手机号13812345678", "session123")

	assert.Equal(t, 1, result.RedactionCount)
	assert.NotContains(t, result.Processed, "13812345678")

	// Token should be stored
	assert.Equal(t, 1, d.tokenStore.GetTokenCount())
}

func TestRestore(t *testing.T) {
	d := NewDesensitizer()
	sessionID := "test-session"

	original := "我的手机号是13812345678"
	desensitized := d.ProcessWithContext(context.Background(), original, sessionID)

	// Restore should bring back original
	restored := d.Restore(desensitized.Processed, sessionID)
	assert.Contains(t, restored, "13812345678")
}

// ==================== Rule Management Tests ====================

func TestAddRule(t *testing.T) {
	d := NewDesensitizer()
	initialCount := len(d.GetRules())

	customRule := DesensitizationRule{
		ID:       "custom_pattern",
		Name:     "Custom Pattern",
		Type:     PIICustom,
		Pattern:  `\bCUSTOM\d+\b`,
		Strategy: StrategyMask,
		Enabled:  true,
		Priority: 200, // High priority
	}

	d.AddRule(customRule)

	rules := d.GetRules()
	assert.Len(t, rules, initialCount+1)

	// Verify rule is inserted at correct position
	assert.Equal(t, "custom_pattern", rules[0].ID, "High priority rule should be first")

	// Test the new rule works
	result := d.Process("Found CUSTOM123 in text")
	assert.Contains(t, result.Processed, "********")
	assert.NotContains(t, result.Processed, "CUSTOM123")
}

func TestRemoveRule(t *testing.T) {
	d := NewDesensitizer()

	// Remove phone rule
	d.RemoveRule("phone_cn")

	result := d.Process("手机号13812345678")
	// Phone should not be masked anymore
	assert.Contains(t, result.Processed, "13812345678")
}

func TestSetRuleEnabled(t *testing.T) {
	d := NewDesensitizer()

	// Disable phone rule
	d.SetRuleEnabled("phone_cn", false)

	result := d.Process("手机号13812345678")
	assert.Contains(t, result.Processed, "13812345678")

	// Re-enable
	d.SetRuleEnabled("phone_cn", true)
	result = d.Process("手机号13812345678")
	assert.NotContains(t, result.Processed, "13812345678")
}

// ==================== Token Store Tests ====================

func TestTokenStore(t *testing.T) {
	ts := NewTokenStore()

	token := "[TOKEN_123]"
	original := "secret_value"

	ts.Store(token, original, PIICustom, "session1")

	// Retrieve
	val, exists := ts.Get(token)
	assert.True(t, exists)
	assert.Equal(t, original, val)

	// Restore
	text := "Some text with [TOKEN_123] embedded"
	restored := ts.Restore(text, "session1")
	assert.Contains(t, restored, original)
}

func TestTokenStore_SessionIsolation(t *testing.T) {
	ts := NewTokenStore()

	ts.Store("[T1]", "value1", PIICustom, "session1")
	ts.Store("[T2]", "value2", PIICustom, "session2")

	// Restore with session1 should only restore session1 tokens
	text := "[T1] and [T2]"
	restored := ts.Restore(text, "session1")
	assert.Contains(t, restored, "value1")
	assert.NotContains(t, restored, "value2")

	// Empty sessionID should restore all
	restored = ts.Restore(text, "")
	assert.Contains(t, restored, "value1")
	assert.Contains(t, restored, "value2")
}

func TestTokenStore_ClearSession(t *testing.T) {
	ts := NewTokenStore()

	ts.Store("[T1]", "v1", PIICustom, "session1")
	ts.Store("[T2]", "v2", PIICustom, "session2")

	ts.ClearSession("session1")

	_, exists := ts.Get("[T1]")
	assert.False(t, exists)

	_, exists = ts.Get("[T2]")
	assert.True(t, exists)
}

func TestTokenStore_Clear(t *testing.T) {
	ts := NewTokenStore()

	ts.Store("[T1]", "v1", PIICustom, "s1")
	ts.Store("[T2]", "v2", PIICustom, "s2")

	ts.Clear()

	assert.Equal(t, 0, ts.GetTokenCount())
}

// ==================== API Tests ====================

func TestDesensitizationAPI_Desensitize(t *testing.T) {
	api := NewDesensitizationAPI()

	req := &DesensitizeRequest{
		Text: "电话：13812345678",
	}

	resp := api.Desensitize(req)
	assert.True(t, resp.Success)
	assert.Equal(t, 1, resp.RedactionCount)
	assert.NotContains(t, resp.Processed, "13812345678")
}

func TestDesensitizationAPI_DesensitizeWithStrategy(t *testing.T) {
	api := NewDesensitizationAPI()

	req := &DesensitizeRequest{
		Text:     "电话：13812345678",
		Strategy: StrategyMask,
	}

	resp := api.Desensitize(req)
	assert.True(t, resp.Success)
	assert.Contains(t, resp.Processed, "***********") // Full mask
}

func TestDesensitizationAPI_Restore(t *testing.T) {
	api := NewDesensitizationAPI()

	// First desensitize with session
	req := &DesensitizeRequest{
		Text:      "电话：13812345678",
		SessionID: "test-session",
	}
	desensitized := api.Desensitize(req)

	// Then restore
	restoreReq := &RestoreRequest{
		Text:      desensitized.Processed,
		SessionID: "test-session",
	}
	restoreResp := api.Restore(restoreReq)

	assert.True(t, restoreResp.Success)
	assert.Contains(t, restoreResp.Restored, "13812345678")
}

func TestDesensitizationAPI_GetRules(t *testing.T) {
	api := NewDesensitizationAPI()

	rules := api.GetRules()
	assert.NotEmpty(t, rules)
}

func TestDesensitizationAPI_AddRule(t *testing.T) {
	api := NewDesensitizationAPI()

	customRule := DesensitizationRule{
		ID:       "custom_test",
		Type:     PIICustom,
		Pattern:  `\bSECRET\d+\b`,
		Strategy: StrategyMask,
		Enabled:  true,
		Priority: 200,
	}

	api.AddRule(customRule)

	result := api.Desensitize(&DesensitizeRequest{Text: "Found SECRET123"})
	assert.Contains(t, result.Processed, "********")
}

// ==================== Config Export/Import Tests ====================

func TestExportImportConfig(t *testing.T) {
	d := NewDesensitizer()

	// Export
	config, err := d.ExportConfig()
	require.NoError(t, err)
	assert.NotEmpty(t, config)

	// Import to new desensitizer
	d2 := NewDesensitizerWithRules(nil)
	err = d2.ImportConfig(config)
	require.NoError(t, err)

	// Both should behave the same
	result1 := d.Process("手机：13812345678")
	result2 := d2.Process("手机：13812345678")

	assert.Equal(t, result1.Processed, result2.Processed)
}

// ==================== Edge Cases Tests ====================

func TestEmptyInput(t *testing.T) {
	d := NewDesensitizer()

	result := d.Process("")
	assert.Equal(t, "", result.Processed)
	assert.Equal(t, 0, result.RedactionCount)
}

func TestNoMatch(t *testing.T) {
	d := NewDesensitizer()

	result := d.Process("这是一段普通文本，没有敏感信息")
	assert.Equal(t, "这是一段普通文本，没有敏感信息", result.Processed)
	assert.Equal(t, 0, result.RedactionCount)
}

func TestDisabledRule(t *testing.T) {
	d := NewDesensitizerWithRules([]DesensitizationRule{
		{
			ID:       "disabled_rule",
			Type:     PIICustom,
			Pattern:  `\bPATTERN\b`,
			Strategy: StrategyMask,
			Enabled:  false,
			Priority: 100,
		},
	})

	result := d.Process("Found PATTERN here")
	assert.Contains(t, result.Processed, "PATTERN")
}

func TestConcurrency(t *testing.T) {
	d := NewDesensitizer()

	done := make(chan bool)

	// Run multiple goroutines
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				d.Process("手机：13812345678")
			}
			done <- true
		}()
	}

	// Wait for all to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestOverlappingPatterns(t *testing.T) {
	d := NewDesensitizer()

	// Both phone and ID card patterns might overlap with numeric patterns
	// Higher priority should be processed first
	result := d.Process("身份证：110101199001011234 和 手机：13812345678")

	assert.Equal(t, 2, result.RedactionCount)
}

func TestPartialMaskShortValue(t *testing.T) {
	d := NewDesensitizer()

	// Test with a value shorter than showFirst + showLast
	rule := DesensitizationRule{
		ID:        "short_test",
		Type:      PIICustom,
		Pattern:   `\bABC\b`,
		Strategy:  StrategyPartial,
		ShowFirst: 5,
		ShowLast:  5,
		Enabled:   true,
		Priority:  200,
	}
	d.AddRule(rule)

	result := d.Process("Value ABC here")
	// Should handle gracefully without panic
	assert.NotNil(t, result.Processed)
}

// ==================== Performance Tests ====================

func BenchmarkDesensitize(b *testing.B) {
	d := NewDesensitizer()
	text := "手机：13812345678，邮箱：test@example.com，身份证：110101199001011234"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Process(text)
	}
}

func BenchmarkDesensitizeWithTokenize(b *testing.B) {
	d := NewDesensitizer()
	text := "手机：13812345678"
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.ProcessWithContext(ctx, text, "session")
	}
}

// ==================== Integration Tests ====================

func TestFullWorkflow(t *testing.T) {
	api := NewDesensitizationAPI()
	sessionID := "test-session-" + time.Now().Format("20060102150405")

	// 1. Desensitize
	req := &DesensitizeRequest{
		Text:      "用户张三，电话13812345678，邮箱test@example.com，身份证110101199001011234",
		SessionID: sessionID,
	}
	result := api.Desensitize(req)

	assert.True(t, result.Success)
	assert.Equal(t, 3, result.RedactionCount) // phone, email, id_card
	assert.NotContains(t, result.Processed, "13812345678")
	assert.NotContains(t, result.Processed, "test@example.com")
	assert.NotContains(t, result.Processed, "110101199001011234")

	// 2. Restore
	restoreReq := &RestoreRequest{
		Text:      result.Processed,
		SessionID: sessionID,
	}
	restored := api.Restore(restoreReq)

	assert.Contains(t, restored.Restored, "13812345678")
	assert.Contains(t, restored.Restored, "test@example.com")
	assert.Contains(t, restored.Restored, "110101199001011234")
}
