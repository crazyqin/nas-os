package privacyshield

import (
	"testing"
)

func TestNewShield(t *testing.T) {
	shield := NewShield()
	if shield == nil {
		t.Fatal("NewShield() 返回 nil")
	}

	patterns := shield.GetPatterns()
	if len(patterns) == 0 {
		t.Error("默认模式列表为空")
	}

	// 检查是否包含预定义的模式
	expectedCategories := map[string]bool{
		"phone":          false,
		"id_card":        false,
		"email":          false,
		"bank_card":      false,
		"ip_address":     false,
		"passport":       false,
		"social_security": false,
	}

	for _, p := range patterns {
		if _, exists := expectedCategories[p.Category]; exists {
			expectedCategories[p.Category] = true
		}
	}

	for category, found := range expectedCategories {
		if !found {
			t.Errorf("缺少预定义模式: %s", category)
		}
	}
}

func TestPhonePattern(t *testing.T) {
	shield := NewShield()

	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "单个手机号",
			content:  "我的手机号是13812345678",
			expected: 1,
		},
		{
			name:     "多个手机号",
			content:  "联系人：13812345678，15987654321",
			expected: 2,
		},
		{
			name:     "无效手机号",
			content:  "这是数字12345678",
			expected: 0,
		},
		{
			name:     "17开头手机号",
			content:  "电话：17600001111",
			expected: 1,
		},
		{
			name:     "19开头手机号",
			content:  "电话：19900001111",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := shield.ScanContent(tt.content)
			if err != nil {
				t.Fatalf("ScanContent 失败: %v", err)
			}

			phoneCount := 0
			for _, match := range result.Matches {
				if match.Pattern.Category == "phone" {
					phoneCount++
				}
			}

			if phoneCount != tt.expected {
				t.Errorf("期望找到 %d 个手机号，实际找到 %d 个", tt.expected, phoneCount)
			}
		})
	}
}

func TestIDCardPattern(t *testing.T) {
	shield := NewShield()

	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "有效身份证号",
			content:  "身份证：110101199003071234",
			expected: 1,
		},
		{
			name:     "带X的身份证号",
			content:  "身份证：11010119900307123X",
			expected: 1,
		},
		{
			name:     "无效身份证号（15位）",
			content:  "身份证：110101900307123",
			expected: 0,
		},
		{
			name:     "无效身份证号（错误日期）",
			content:  "身份证：110101199013071234",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := shield.ScanContent(tt.content)
			if err != nil {
				t.Fatalf("ScanContent 失败: %v", err)
			}

			idCardCount := 0
			for _, match := range result.Matches {
				if match.Pattern.Category == "id_card" {
					idCardCount++
				}
			}

			if idCardCount != tt.expected {
				t.Errorf("期望找到 %d 个身份证号，实际找到 %d 个", tt.expected, idCardCount)
			}
		})
	}
}

func TestEmailPattern(t *testing.T) {
	shield := NewShield()

	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "标准邮箱",
			content:  "邮箱：test@example.com",
			expected: 1,
		},
		{
			name:     "带点的邮箱",
			content:  "邮箱：first.last@company.co.jp",
			expected: 1,
		},
		{
			name:     "无效邮箱",
			content:  "这是@无效的",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := shield.ScanContent(tt.content)
			if err != nil {
				t.Fatalf("ScanContent 失败: %v", err)
			}

			emailCount := 0
			for _, match := range result.Matches {
				if match.Pattern.Category == "email" {
					emailCount++
				}
			}

			if emailCount != tt.expected {
				t.Errorf("期望找到 %d 个邮箱，实际找到 %d 个", tt.expected, emailCount)
			}
		})
	}
}

func TestBankCardPattern(t *testing.T) {
	shield := NewShield()

	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "16位银行卡",
			content:  "卡号：6222021234567890123",
			expected: 1,
		},
		{
			name:     "19位银行卡",
			content:  "卡号：622202123456789012",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := shield.ScanContent(tt.content)
			if err != nil {
				t.Fatalf("ScanContent 失败: %v", err)
			}

			bankCardCount := 0
			for _, match := range result.Matches {
				if match.Pattern.Category == "bank_card" {
					bankCardCount++
				}
			}

			if bankCardCount != tt.expected {
				t.Errorf("期望找到 %d 个银行卡号，实际找到 %d 个", tt.expected, bankCardCount)
			}
		})
	}
}

func TestIPAddressPattern(t *testing.T) {
	shield := NewShield()

	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "有效IP地址",
			content:  "服务器IP：192.168.1.100",
			expected: 1,
		},
		{
			name:     "多个IP地址",
			content:  "源：10.0.0.1 目标：172.16.0.1",
			expected: 2,
		},
		{
			name:     "无效IP地址",
			content:  "IP：256.1.1.1",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := shield.ScanContent(tt.content)
			if err != nil {
				t.Fatalf("ScanContent 失败: %v", err)
			}

			ipCount := 0
			for _, match := range result.Matches {
				if match.Pattern.Category == "ip_address" {
					ipCount++
				}
			}

			if ipCount != tt.expected {
				t.Errorf("期望找到 %d 个IP地址，实际找到 %d 个", tt.expected, ipCount)
			}
		})
	}
}

func TestMaskContent(t *testing.T) {
	shield := NewShield()

	tests := []struct {
		name     string
		content  string
		strategy string
		checkFn  func(t *testing.T, masked string)
	}{
		{
			name:     "手机号 partial 脱敏",
			content:  "手机号：13812345678",
			strategy: "partial",
			checkFn: func(t *testing.T, masked string) {
				if masked == "手机号：13812345678" {
					t.Error("手机号未被脱敏")
				}
				if len(masked) == 0 {
					t.Error("脱敏结果为空")
				}
			},
		},
		{
			name:     "邮箱 partial 脱敏",
			content:  "邮箱：test@example.com",
			strategy: "partial",
			checkFn: func(t *testing.T, masked string) {
				if masked == "邮箱：test@example.com" {
					t.Error("邮箱未被脱敏")
				}
			},
		},
		{
			name:     "完全掩码",
			content:  "手机号：13812345678",
			strategy: "mask",
			checkFn: func(t *testing.T, masked string) {
				if masked == "手机号：13812345678" {
					t.Error("手机号未被脱敏")
				}
			},
		},
		{
			name:     "哈希脱敏",
			content:  "手机号：13812345678",
			strategy: "hash",
			checkFn: func(t *testing.T, masked string) {
				if masked == "手机号：13812345678" {
					t.Error("手机号未被脱敏")
				}
			},
		},
		{
			name:     "删除脱敏",
			content:  "手机号：13812345678",
			strategy: "redact",
			checkFn: func(t *testing.T, masked string) {
				if masked == "手机号：13812345678" {
					t.Error("手机号未被脱敏")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := shield.MaskContent(tt.content, tt.strategy, nil)
			if err != nil {
				t.Fatalf("MaskContent 失败: %v", err)
			}

			tt.checkFn(t, result.Masked)
		})
	}
}

func TestMaskContentWithOptions(t *testing.T) {
	shield := NewShield()

	content := "手机号：13812345678"
	options := &MaskOptions{
		PrefixKeep: 2,
		SuffixKeep: 2,
		MaskChar:   "#",
	}

	result, err := shield.MaskContent(content, "partial", options)
	if err != nil {
		t.Fatalf("MaskContent 失败: %v", err)
	}

	if result.Masked == content {
		t.Error("手机号未被脱敏")
	}

	if result.MatchCount == 0 {
		t.Error("未匹配到敏感数据")
	}
}

func TestScanWithCategories(t *testing.T) {
	shield := NewShield()

	content := "手机号：13812345678，邮箱：test@example.com，IP：192.168.1.1"

	// 只扫描手机号
	result, err := shield.ScanContent(content, "phone")
	if err != nil {
		t.Fatalf("ScanContent 失败: %v", err)
	}

	for _, match := range result.Matches {
		if match.Pattern.Category != "phone" {
			t.Errorf("不应该匹配到非手机号类型: %s", match.Pattern.Category)
		}
	}

	// 扫描多个类别
	result, err = shield.ScanContent(content, "phone", "email")
	if err != nil {
		t.Fatalf("ScanContent 失败: %v", err)
	}

	if len(result.Matches) < 2 {
		t.Errorf("应该匹配到至少2个结果，实际匹配到 %d 个", len(result.Matches))
	}
}

func TestComplianceReport(t *testing.T) {
	shield := NewShield()

	content := "用户信息：姓名张三，手机13812345678，身份证110101199003071234"

	report, err := shield.GenerateComplianceReport(content, "GDPR")
	if err != nil {
		t.Fatalf("GenerateComplianceReport 失败: %v", err)
	}

	if report == nil {
		t.Fatal("报告为空")
	}

	if report.Framework != "GDPR" {
		t.Errorf("框架错误，期望 GDPR，实际 %s", report.Framework)
	}

	if report.TotalItems == 0 {
		t.Error("应该检测到敏感数据")
	}

	if report.Score < 0 || report.Score > 100 {
		t.Errorf("合规分数超出范围: %f", report.Score)
	}
}

func TestPIPLCompliance(t *testing.T) {
	shield := NewShield()

	content := "用户信息：手机13812345678，身份证110101199003071234"

	report, err := shield.GenerateComplianceReport(content, "PIPL")
	if err != nil {
		t.Fatalf("GenerateComplianceReport 失败: %v", err)
	}

	if report.Framework != "PIPL" {
		t.Errorf("框架错误，期望 PIPL，实际 %s", report.Framework)
	}

	// PIPL应该对身份证有更严格的要求
	hasIDCardIssue := false
	for _, issue := range report.Issues {
		if issue.Type == "identity_data" {
			hasIDCardIssue = true
			break
		}
	}

	if !hasIDCardIssue {
		t.Error("PIPL应该报告身份证数据问题")
	}
}

func TestRiskAssessment(t *testing.T) {
	shield := NewShield()

	tests := []struct {
		name        string
		content     string
		encrypted   bool
		accessLevel string
		expectLevel string
	}{
		{
			name:        "高风险场景",
			content:     "身份证110101199003071234，手机13812345678，卡号6222021234567890123",
			encrypted:   false,
			accessLevel: "public",
			expectLevel: "high",
		},
		{
			name:        "低风险场景",
			content:     "IP地址：192.168.1.1",
			encrypted:   true,
			accessLevel: "restricted",
			expectLevel: "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			risk, err := shield.AssessRisk(tt.content, tt.encrypted, tt.accessLevel)
			if err != nil {
				t.Fatalf("AssessRisk 失败: %v", err)
			}

			if risk.Overall < 0 || risk.Overall > 100 {
				t.Errorf("风险分数超出范围: %f", risk.Overall)
			}

			if risk.RiskLevel == "" {
				t.Error("风险等级为空")
			}

			if risk.Density < 0 || risk.Density > 1 {
				t.Errorf("密度分数超出范围: %f", risk.Density)
			}
		})
	}
}

func TestAddAndRemovePattern(t *testing.T) {
	shield := NewShield()

	// 添加新模式
	newPattern := SensitivePattern{
		Name:        "测试模式",
		Pattern:     `TEST-\d{6}`,
		Category:    "test",
		Severity:    5,
		Description: "测试用模式",
	}

	shield.AddPattern(newPattern)

	patterns := shield.GetPatterns()
	found := false
	for _, p := range patterns {
		if p.Name == "测试模式" {
			found = true
			break
		}
	}

	if !found {
		t.Error("新模式未被添加")
	}

	// 删除模式
	removed := shield.RemovePattern("测试模式")
	if !removed {
		t.Error("删除模式失败")
	}

	patterns = shield.GetPatterns()
	for _, p := range patterns {
		if p.Name == "测试模式" {
			t.Error("模式未被删除")
		}
	}

	// 删除不存在的模式
	removed = shield.RemovePattern("不存在的模式")
	if removed {
		t.Error("不应该删除成功")
	}
}

func TestScanResultRiskScore(t *testing.T) {
	shield := NewShield()

	content := "手机号：13812345678，邮箱：test@example.com，IP：192.168.1.1"

	result, err := shield.ScanContent(content)
	if err != nil {
		t.Fatalf("ScanContent 失败: %v", err)
	}

	if result.RiskScore < 0 || result.RiskScore > 100 {
		t.Errorf("风险分数超出范围: %f", result.RiskScore)
	}

	if result.Categories == nil {
		t.Error("分类统计为空")
	}

	if result.TotalMatches == 0 {
		t.Error("应该匹配到敏感数据")
	}
}

func TestConcurrentAccess(t *testing.T) {
	shield := NewShield()

	// 测试并发读取
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			patterns := shield.GetPatterns()
			if len(patterns) == 0 {
				t.Error("并发读取失败")
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// 测试并发写入和读取
	for i := 0; i < 5; i++ {
		go func() {
			shield.AddPattern(SensitivePattern{
				Name:    "并发测试",
				Pattern: `TEST-\d+`,
				Category: "concurrent",
			})
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		go func() {
			_ = shield.GetPatterns()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestEmptyContent(t *testing.T) {
	shield := NewShield()

	// 空内容扫描
	result, err := shield.ScanContent("")
	if err != nil {
		t.Fatalf("ScanContent 失败: %v", err)
	}

	if len(result.Matches) != 0 {
		t.Error("空内容不应该匹配到任何结果")
	}

	// 空内容脱敏
	maskResult, err := shield.MaskContent("", "mask", nil)
	if err != nil {
		t.Fatalf("MaskContent 失败: %v", err)
	}

	if maskResult.Masked != "" {
		t.Error("空内容脱敏应该返回空字符串")
	}

	if maskResult.MatchCount != 0 {
		t.Error("空内容不应该匹配到任何结果")
	}
}

func TestLineNumbers(t *testing.T) {
	shield := NewShield()

	content := "第一行正常\n第二行手机13812345678\n第三行正常\n第四行邮箱test@example.com"

	result, err := shield.ScanContent(content)
	if err != nil {
		t.Fatalf("ScanContent 失败: %v", err)
	}

	for _, match := range result.Matches {
		if match.LineNum == 0 {
			t.Error("行号应该大于0")
		}
		if match.LineNum > 4 {
			t.Errorf("行号超出范围: %d", match.LineNum)
		}
	}
}
