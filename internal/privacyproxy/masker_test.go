package privacyproxy

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// =========================================================================
// 脱敏引擎测试
// =========================================================================

func TestNewMasker(t *testing.T) {
	m := NewMasker(nil)
	if m == nil {
		t.Fatal("NewMasker 返回 nil")
	}
	rules := m.ListRules()
	if len(rules) == 0 {
		t.Fatal("内置规则数不应为 0")
	}
	expectedIDs := []string{
		"builtin-id-card", "builtin-phone", "builtin-email",
		"builtin-bank-card", "builtin-ipv4", "builtin-api-key",
		"builtin-passport", "builtin-license-plate",
	}
	for _, id := range expectedIDs {
		found := false
		for _, r := range rules {
			if r.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("缺少内置规则: %s", id)
		}
	}
}

func TestMaskIDCard(t *testing.T) {
	m := NewMasker(nil)
	text := "我的身份证号是110101199001011234，请保密。"
	out := m.Mask(text)
	if out.TotalHits == 0 {
		t.Fatal("身份证号未匹配")
	}
	if strings.Contains(out.MaskedText, "110101199001011234") {
		t.Error("身份证号未被脱敏")
	}
	if !strings.HasPrefix(out.MaskedText, "我的身份证号是110101") {
		t.Error("身份证号前6位应保留")
	}
}

func TestMaskPhone(t *testing.T) {
	m := NewMasker(nil)
	text := "联系电话：13812345678，请打这个号码。"
	out := m.Mask(text)
	if out.TotalHits == 0 {
		t.Fatal("手机号未匹配")
	}
	if strings.Contains(out.MaskedText, "13812345678") {
		t.Error("手机号未被脱敏")
	}
	if !strings.Contains(out.MaskedText, "138") {
		t.Error("手机号前3位应保留")
	}
}

func TestMaskEmail(t *testing.T) {
	m := NewMasker(nil)
	text := "请发邮件到 user@example.com 谢谢"
	out := m.Mask(text)
	if out.TotalHits == 0 {
		t.Fatal("邮箱未匹配")
	}
	if strings.Contains(out.MaskedText, "user@example.com") {
		t.Error("邮箱未被脱敏")
	}
}

func TestMaskBankCard(t *testing.T) {
	m := NewMasker(nil)
	text := "银行卡号：6222021234567890123"
	out := m.Mask(text)
	if out.TotalHits == 0 {
		t.Fatal("银行卡号未匹配")
	}
	if strings.Contains(out.MaskedText, "6222021234567890123") {
		t.Error("银行卡号未被脱敏")
	}
}

func TestMaskIPv4(t *testing.T) {
	m := NewMasker(nil)
	text := "服务器地址是 192.168.1.100 请连接"
	out := m.Mask(text)
	if out.TotalHits == 0 {
		t.Fatal("IPv4 地址未匹配")
	}
	if strings.Contains(out.MaskedText, "192.168.1.100") {
		t.Error("IPv4 地址未被脱敏")
	}
}

func TestMaskAPIKey(t *testing.T) {
	m := NewMasker(nil)
	tests := []string{
		"sk-abcdefghijklmnopqrstuvwxyz1234567890",
		"AKIAABCDEFGHIJKLMNOP",
		"AIzaSyA1234567890_abcdefghijklmnopqrstu",
		"Bearer abcdefghijklmnop1234567890",
	}
	for _, key := range tests {
		out := m.Mask(key)
		if out.TotalHits == 0 {
			t.Errorf("API Key 未匹配: %s", key)
		}
		if strings.Contains(out.MaskedText, key) {
			t.Errorf("API Key 未被脱敏: %s", key)
		}
	}
}

func TestMaskMultipleTypes(t *testing.T) {
	m := NewMasker(nil)
	text := "用户张三，身份证110101199001011234，电话13812345678，邮箱 zhangsan@test.com，IP 10.0.0.1，API Key sk-abcdef1234567890abcdef1234567890"
	out := m.Mask(text)
	if out.TotalHits < 5 {
		t.Errorf("应命中至少 5 种敏感数据，实际命中 %d", out.TotalHits)
	}
	sensitiveItems := []string{
		"110101199001011234",
		"13812345678",
		"zhangsan@test.com",
		"10.0.0.1",
		"sk-abcdef1234567890abcdef1234567890",
	}
	for _, s := range sensitiveItems {
		if strings.Contains(out.MaskedText, s) {
			t.Errorf("敏感数据未被脱敏: %s", s)
		}
	}
}

func TestMaskEmptyString(t *testing.T) {
	m := NewMasker(nil)
	out := m.Mask("")
	if out.TotalHits != 0 {
		t.Error("空字符串不应有匹配")
	}
	if out.MaskedText != "" {
		t.Error("空字符串脱敏后应为空")
	}
}

func TestMaskNoSensitiveData(t *testing.T) {
	m := NewMasker(nil)
	text := "这是一段普通文本，没有敏感信息。"
	out := m.Mask(text)
	if out.TotalHits != 0 {
		t.Errorf("不应有匹配，实际命中 %d", out.TotalHits)
	}
	if out.MaskedText != text {
		t.Error("无敏感数据时文本不应改变")
	}
}

// =========================================================================
// 自定义规则测试
// =========================================================================

func TestAddCustomRule(t *testing.T) {
	m := NewMasker(nil)
	rule := &MaskRule{
		ID:          "custom-employee-id",
		Name:        "工号",
		Pattern:     `EMP\d{6}`,
		Action:      ActionReplace,
		Replacement: "[工号已隐藏]",
		Enabled:     true,
	}
	if err := m.AddRule(rule); err != nil {
		t.Fatalf("添加自定义规则失败: %v", err)
	}
	text := "我的工号是EMP123456"
	out := m.Mask(text)
	if out.TotalHits == 0 {
		t.Fatal("自定义规则未匹配")
	}
	if !strings.Contains(out.MaskedText, "[工号已隐藏]") {
		t.Error("自定义规则替换文本不正确")
	}
}

func TestAddCustomRuleInvalidRegex(t *testing.T) {
	m := NewMasker(nil)
	rule := &MaskRule{
		ID:      "custom-bad",
		Name:    "错误规则",
		Pattern: `[invalid`,
		Action:  ActionReplace,
	}
	if err := m.AddRule(rule); err == nil {
		t.Error("无效正则应返回错误")
	}
}

func TestAddCustomRuleEmptyID(t *testing.T) {
	m := NewMasker(nil)
	rule := &MaskRule{
		Pattern: `\d+`,
		Action:  ActionMask,
	}
	if err := m.AddRule(rule); err == nil {
		t.Error("空 ID 应返回错误")
	}
}

func TestRemoveCustomRule(t *testing.T) {
	m := NewMasker(nil)
	rule := &MaskRule{
		ID:      "custom-temp",
		Name:    "临时规则",
		Pattern: `TEMP\d+`,
		Action:  ActionRedact,
	}
	m.AddRule(rule)
	if err := m.RemoveRule("custom-temp"); err != nil {
		t.Fatalf("删除自定义规则失败: %v", err)
	}
	_, found := m.GetRule("custom-temp")
	if found {
		t.Error("规则删除后不应能获取")
	}
}

func TestRemoveBuiltinRuleFails(t *testing.T) {
	m := NewMasker(nil)
	if err := m.RemoveRule("builtin-phone"); err == nil {
		t.Error("删除内置规则应失败")
	}
}

func TestEnableDisableRule(t *testing.T) {
	m := NewMasker(nil)
	if err := m.EnableRule("builtin-phone", false); err != nil {
		t.Fatalf("禁用规则失败: %v", err)
	}
	text := "电话13812345678"
	out := m.Mask(text)
	phoneMatched := false
	for _, match := range out.Matches {
		if match.RuleID == "builtin-phone" {
			phoneMatched = true
			break
		}
	}
	if phoneMatched {
		t.Error("禁用的规则不应匹配")
	}
	if err := m.EnableRule("builtin-phone", true); err != nil {
		t.Fatalf("启用规则失败: %v", err)
	}
	out = m.Mask(text)
	phoneMatched = false
	for _, match := range out.Matches {
		if match.RuleID == "builtin-phone" {
			phoneMatched = true
			break
		}
	}
	if !phoneMatched {
		t.Error("启用后规则应匹配")
	}
}

func TestGetRule(t *testing.T) {
	m := NewMasker(nil)
	rule, ok := m.GetRule("builtin-email")
	if !ok {
		t.Fatal("应能获取邮箱规则")
	}
	if rule.Name != "邮箱" {
		t.Errorf("规则名称不正确: %s", rule.Name)
	}
}

// =========================================================================
// 脱敏动作测试
// =========================================================================

func TestActionMask(t *testing.T) {
	m := NewMasker(nil)
	rule := &MaskRule{
		ID:         "test-mask",
		Name:       "测试掩码",
		Pattern:    `TEST\d{8}`,
		Action:     ActionMask,
		KeepPrefix: 2,
		KeepSuffix: 2,
		Enabled:    true,
	}
	m.AddRule(rule)
	text := "代码TEST12345678结束"
	out := m.Mask(text)
	if out.TotalHits == 0 {
		t.Fatal("未匹配")
	}
	if strings.Contains(out.MaskedText, "TEST12345678") {
		t.Error("原始数据不应出现")
	}
}

func TestActionReplace(t *testing.T) {
	m := NewMasker(nil)
	rule := &MaskRule{
		ID:          "test-replace",
		Name:        "测试替换",
		Pattern:     `SECRET\d+`,
		Action:      ActionReplace,
		Replacement: "[已替换]",
		Enabled:     true,
	}
	m.AddRule(rule)
	text := "密码是SECRET12345"
	out := m.Mask(text)
	if !strings.Contains(out.MaskedText, "[已替换]") {
		t.Error("替换动作未生效")
	}
	if strings.Contains(out.MaskedText, "SECRET12345") {
		t.Error("原始数据不应出现")
	}
}

func TestActionHash(t *testing.T) {
	m := NewMasker(nil)
	rule := &MaskRule{
		ID:      "test-hash",
		Name:    "测试哈希",
		Pattern: `TOKEN\d+`,
		Action:  ActionHash,
		Enabled: true,
	}
	m.AddRule(rule)
	text := "令牌TOKEN12345"
	out := m.Mask(text)
	if out.TotalHits == 0 {
		t.Fatal("未匹配")
	}
	if strings.Contains(out.MaskedText, "TOKEN12345") {
		t.Error("原始数据不应出现")
	}
	if !strings.Contains(out.MaskedText, "hash:") {
		t.Error("哈希脱敏应包含 hash: 前缀")
	}
}

func TestActionRedact(t *testing.T) {
	m := NewMasker(nil)
	rule := &MaskRule{
		ID:      "test-redact",
		Name:    "测试删除",
		Pattern: `PASSWORD\d+`,
		Action:  ActionRedact,
		Enabled: true,
	}
	m.AddRule(rule)
	text := "密码PASSWORD12345"
	out := m.Mask(text)
	if !strings.Contains(out.MaskedText, "[REDACTED]") {
		t.Error("删除动作应替换为 [REDACTED]")
	}
}

// =========================================================================
// 审计日志测试
// =========================================================================

func TestAuditorLog(t *testing.T) {
	a := NewAuditor(100)
	entry := AuditEntry{
		ID:         "test-1",
		Timestamp:  time.Now(),
		RuleID:     "builtin-phone",
		RuleName:   "手机号",
		Original:   "13812345678",
		Masked:     "138****78",
		TargetAPI:  "api.openai.com",
		MatchCount: 1,
		Success:    true,
	}
	a.Log(entry)
	total, _ := a.Stats()
	if total != 1 {
		t.Errorf("审计日志条数应为 1，实际 %d", total)
	}
}

func TestAuditorQuery(t *testing.T) {
	a := NewAuditor(100)
	for i := 0; i < 10; i++ {
		a.Log(AuditEntry{
			ID:         "entry-" + string(rune('A'+i)),
			Timestamp:  time.Now(),
			RuleID:     "builtin-phone",
			RuleName:   "手机号",
			Original:   "13812345678",
			Masked:     "138****78",
			TargetAPI:  "api.openai.com",
			MatchCount: 1,
			Success:    true,
		})
	}
	for i := 0; i < 5; i++ {
		a.Log(AuditEntry{
			ID:         "email-" + string(rune('A'+i)),
			Timestamp:  time.Now(),
			RuleID:     "builtin-email",
			RuleName:   "邮箱",
			Original:   "test@example.com",
			Masked:     "t*****.com",
			TargetAPI:  "api.anthropic.com",
			MatchCount: 1,
			Success:    true,
		})
	}
	_, total := a.Query(AuditQuery{})
	if total != 15 {
		t.Errorf("总条数应为 15，实际 %d", total)
	}
	entries, total := a.Query(AuditQuery{RuleID: "builtin-email"})
	if total != 5 {
		t.Errorf("邮箱规则条数应为 5，实际 %d", total)
	}
	if len(entries) != 5 {
		t.Errorf("返回条数应为 5，实际 %d", len(entries))
	}
	_, total = a.Query(AuditQuery{TargetAPI: "anthropic"})
	if total != 5 {
		t.Errorf("anthropic 条数应为 5，实际 %d", total)
	}
	entries, _ = a.Query(AuditQuery{Limit: 3, Offset: 0})
	if len(entries) != 3 {
		t.Errorf("分页返回 3 条，实际 %d", len(entries))
	}
}

func TestAuditorGetByID(t *testing.T) {
	a := NewAuditor(100)
	a.Log(AuditEntry{
		ID:        "find-me",
		RuleID:    "builtin-phone",
		RuleName:  "手机号",
		Original:  "13812345678",
		Masked:    "138****78",
		TargetAPI: "api.openai.com",
		Success:   true,
	})
	entry, found := a.GetByID("find-me")
	if !found {
		t.Fatal("未找到指定 ID 的日志")
	}
	if entry.RuleID != "builtin-phone" {
		t.Error("规则 ID 不匹配")
	}
}

func TestAuditorExportJSON(t *testing.T) {
	a := NewAuditor(100)
	a.Log(AuditEntry{
		ID:        "export-1",
		Timestamp: time.Now(),
		RuleID:    "builtin-phone",
		RuleName:  "手机号",
		Original:  "13812345678",
		Masked:    "138****78",
		TargetAPI: "api.openai.com",
		Success:   true,
	})
	var buf bytes.Buffer
	if err := a.ExportJSON(&buf, AuditQuery{}); err != nil {
		t.Fatalf("JSON 导出失败: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("导出的 JSON 不可解析: %v", err)
	}
	if result["total"] == nil {
		t.Error("JSON 应包含 total 字段")
	}
}

func TestAuditorExportCSV(t *testing.T) {
	a := NewAuditor(100)
	a.Log(AuditEntry{
		ID:        "csv-1",
		Timestamp: time.Now(),
		RuleID:    "builtin-phone",
		RuleName:  "手机号",
		Original:  "13812345678",
		Masked:    "138****78",
		TargetAPI: "api.openai.com",
		Success:   true,
	})
	var buf bytes.Buffer
	if err := a.ExportCSV(&buf, AuditQuery{}); err != nil {
		t.Fatalf("CSV 导出失败: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("CSV 导出不应为空")
	}
	if !strings.Contains(buf.String(), "规则ID") {
		t.Error("CSV 应包含表头'规则ID'")
	}
}

func TestAuditorGenerateReport(t *testing.T) {
	a := NewAuditor(100)
	now := time.Now()
	for i := 0; i < 20; i++ {
		a.Log(AuditEntry{
			ID:         "report-" + string(rune('A'+i)),
			Timestamp:  now,
			RuleID:     "builtin-phone",
			RuleName:   "手机号",
			Original:   "13812345678",
			Masked:     "138****78",
			TargetAPI:  "api.openai.com",
			MatchCount: 2,
			Success:    true,
		})
	}
	report := a.GenerateReport(now.Add(-time.Hour), now.Add(time.Hour))
	if report.TotalRequests != 20 {
		t.Errorf("总请求数应为 20，实际 %d", report.TotalRequests)
	}
	if report.TotalMasked != 40 { // 每条 MatchCount=2
		t.Errorf("总脱敏次数应为 40，实际 %d", report.TotalMasked)
	}
	if report.UniqueRulesUsed != 1 {
		t.Errorf("使用规则数应为 1，实际 %d", report.UniqueRulesUsed)
	}
	if len(report.TopRules) != 1 {
		t.Errorf("TopRules 长度应为 1，实际 %d", len(report.TopRules))
	}
	if len(report.TopTargetAPIs) != 1 {
		t.Errorf("TopTargetAPIs 长度应为 1，实际 %d", len(report.TopTargetAPIs))
	}
}

func TestAuditorClear(t *testing.T) {
	a := NewAuditor(100)
	a.Log(AuditEntry{ID: "1", RuleID: "r1", Success: true})
	a.Log(AuditEntry{ID: "2", RuleID: "r2", Success: true})
	total, _ := a.Stats()
	if total != 2 {
		t.Fatalf("应为 2 条，实际 %d", total)
	}
	a.Clear()
	total, _ = a.Stats()
	if total != 0 {
		t.Errorf("清空后应为 0 条，实际 %d", total)
	}
}

func TestAuditorRingBuffer(t *testing.T) {
	a := NewAuditor(5) // 小容量测试环形缓冲
	for i := 0; i < 10; i++ {
		a.Log(AuditEntry{
			ID:       "ring-" + string(rune('A'+i)),
			RuleID:   "builtin-phone",
			RuleName: "手机号",
			Success:  true,
		})
	}
	total, capacity := a.Stats()
	if total != 5 {
		t.Errorf("环形缓冲应保留 5 条，实际 %d", total)
	}
	if capacity != 5 {
		t.Errorf("容量应为 5，实际 %d", capacity)
	}
}

// =========================================================================
// 代理服务器测试
// =========================================================================

func TestProxyServerCreation(t *testing.T) {
	cfg := DefaultMaskConfig()
	cfg.ListenAddr = "127.0.0.1:0" // 使用端口 0 避免冲突
	masker := NewMasker(cfg)
	auditor := NewAuditor(100)
	proxy := NewProxyServer(cfg, masker, auditor)
	if proxy == nil {
		t.Fatal("NewProxyServer 返回 nil")
	}
	if proxy.IsRunning() {
		t.Error("新创建的代理不应在运行")
	}
}

func TestProxyServerStartStop(t *testing.T) {
	cfg := DefaultMaskConfig()
	cfg.ListenAddr = "127.0.0.1:18421"
	masker := NewMasker(cfg)
	auditor := NewAuditor(100)
	proxy := NewProxyServer(cfg, masker, auditor)

	if err := proxy.Start(); err != nil {
		t.Fatalf("启动代理失败: %v", err)
	}
	if !proxy.IsRunning() {
		t.Error("代理应处于运行状态")
	}
	// 重复启动应报错
	if err := proxy.Start(); err == nil {
		t.Error("重复启动应报错")
	}
	if err := proxy.Stop(); err != nil {
		t.Fatalf("停止代理失败: %v", err)
	}
	if proxy.IsRunning() {
		t.Error("停止后代理不应运行")
	}
}

func TestProxyDomainAllow(t *testing.T) {
	cfg := DefaultMaskConfig()
	cfg.AllowedDomains = []string{"api.openai.com"}
	cfg.BlockedDomains = []string{"evil.com"}
	masker := NewMasker(cfg)
	auditor := NewAuditor(100)
	proxy := NewProxyServer(cfg, masker, auditor)

	if !proxy.isDomainAllowed("api.openai.com") {
		t.Error("api.openai.com 应被允许")
	}
	if proxy.isDomainAllowed("evil.com") {
		t.Error("evil.com 应被阻止")
	}
	if proxy.isDomainAllowed("unknown.com") {
		t.Error("unknown.com 不在白名单中应被阻止")
	}
}

func TestProxyProviderToHost(t *testing.T) {
	cfg := DefaultMaskConfig()
	proxy := NewProxyServer(cfg, nil, nil)
	tests := map[string]string{
		"openai":      "api.openai.com",
		"anthropic":   "api.anthropic.com",
		"google":      "generativelanguage.googleapis.com",
		"dashscope":   "dashscope.aliyuncs.com",
		"deepseek":    "api.deepseek.com",
		"siliconflow": "api.siliconflow.cn",
		"unknown":     "",
	}
	for provider, expected := range tests {
		got := proxy.providerToHost(provider)
		if got != expected {
			t.Errorf("providerToHost(%s) = %s, 期望 %s", provider, got, expected)
		}
	}
}

// =========================================================================
// 配置测试
// =========================================================================

func TestDefaultMaskConfig(t *testing.T) {
	cfg := DefaultMaskConfig()
	if !cfg.Enabled {
		t.Error("默认配置应启用")
	}
	if cfg.ListenAddr == "" {
		t.Error("监听地址不应为空")
	}
	if cfg.MaxBodyBytes <= 0 {
		t.Error("最大请求体应大于 0")
	}
	if cfg.StreamTimeout <= 0 {
		t.Error("流式超时应大于 0")
	}
	if len(cfg.AllowedDomains) == 0 {
		t.Error("默认白名单不应为空")
	}
}

func TestMaskConfigJSONRoundTrip(t *testing.T) {
	cfg := DefaultMaskConfig()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var restored MaskConfig
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	if restored.ListenAddr != cfg.ListenAddr {
		t.Error("JSON 往返后监听地址不一致")
	}
}
