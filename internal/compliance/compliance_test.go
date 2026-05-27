package compliance

import (
	"testing"
)

func TestNewComplianceEngine(t *testing.T) {
	engine := NewComplianceEngine()
	if engine == nil {
		t.Fatal("NewComplianceEngine 返回 nil")
	}
}

func TestGetStandards(t *testing.T) {
	engine := NewComplianceEngine()

	standards := engine.GetStandards()
	if len(standards) != 3 {
		t.Errorf("期望3个标准，得到 %d", len(standards))
	}

	// 检查GDPR
	found := false
	for _, s := range standards {
		if s.ID == StandardGDPR {
			found = true
			break
		}
	}
	if !found {
		t.Error("未找到GDPR标准")
	}
}

func TestGetStandardInfo(t *testing.T) {
	engine := NewComplianceEngine()

	// 存在的标准
	info, err := engine.GetStandardInfo(StandardGDPR)
	if err != nil {
		t.Fatalf("获取GDPR标准失败: %v", err)
	}
	if info.Name != "GDPR 通用数据保护条例" {
		t.Errorf("期望名称 'GDPR 通用数据保护条例'，得到 '%s'", info.Name)
	}

	// 不存在的标准
	_, err = engine.GetStandardInfo("nonexistent")
	if err != ErrStandardNotFound {
		t.Errorf("期望 ErrStandardNotFound，得到 %v", err)
	}
}

func TestGetCheckItems(t *testing.T) {
	engine := NewComplianceEngine()

	checks, err := engine.GetCheckItems(StandardGDPR)
	if err != nil {
		t.Fatalf("获取GDPR检查项失败: %v", err)
	}

	if len(checks) == 0 {
		t.Error("GDPR检查项为空")
	}

	// 检查项结构
	for _, check := range checks {
		if check.ID == "" {
			t.Error("检查项ID为空")
		}
		if check.Standard != StandardGDPR {
			t.Errorf("检查项标准不匹配: %s", check.Standard)
		}
	}
}

func TestRunComplianceCheck(t *testing.T) {
	engine := NewComplianceEngine()

	report, err := engine.RunComplianceCheck(StandardGDPR)
	if err != nil {
		t.Fatalf("执行GDPR合规检查失败: %v", err)
	}

	if report == nil {
		t.Fatal("报告为空")
	}

	if report.ID == "" {
		t.Error("报告ID为空")
	}

	if report.Standard != StandardGDPR {
		t.Errorf("报告标准不匹配: %s", report.Standard)
	}

	if report.TotalChecks == 0 {
		t.Error("检查总数为0")
	}

	if report.Results == nil {
		t.Error("检查结果为空")
	}

	// 检查合规等级
	validLevels := map[string]bool{"A": true, "B": true, "C": true, "D": true}
	if !validLevels[report.ComplianceLevel] {
		t.Errorf("无效的合规等级: %s", report.ComplianceLevel)
	}
}

func TestRunComplianceCheckInvalidStandard(t *testing.T) {
	engine := NewComplianceEngine()

	_, err := engine.RunComplianceCheck("nonexistent")
	if err != ErrStandardNotFound {
		t.Errorf("期望 ErrStandardNotFound，得到 %v", err)
	}
}

func TestRegisterCheckFunc(t *testing.T) {
	engine := NewComplianceEngine()

	// 注册自定义检查函数
	engine.RegisterCheckFunc("gdpr-001", func() CheckStatus {
		return StatusPass
	})

	// 执行检查
	report, err := engine.RunComplianceCheck(StandardGDPR)
	if err != nil {
		t.Fatalf("执行检查失败: %v", err)
	}

	// 验证自定义函数被调用
	for _, result := range report.Results {
		if result.CheckID == "gdpr-001" {
			if result.Status != StatusPass {
				t.Errorf("期望状态 pass，得到 %s", result.Status)
			}
			break
		}
	}
}

func TestGetReports(t *testing.T) {
	engine := NewComplianceEngine()

	// 初始无报告
	reports := engine.GetReports()
	if len(reports) != 0 {
		t.Error("初始应无报告")
	}

	// 执行检查生成报告
	_, err := engine.RunComplianceCheck(StandardGDPR)
	if err != nil {
		t.Fatalf("执行检查失败: %v", err)
	}

	reports = engine.GetReports()
	if len(reports) != 1 {
		t.Errorf("期望1份报告，得到 %d", len(reports))
	}
}

func TestGetLatestReport(t *testing.T) {
	engine := NewComplianceEngine()

	// 无报告
	_, err := engine.GetLatestReport(StandardGDPR)
	if err == nil {
		t.Error("应返回错误")
	}

	// 执行检查
	_, err = engine.RunComplianceCheck(StandardGDPR)
	if err != nil {
		t.Fatalf("执行检查失败: %v", err)
	}

	report, err := engine.GetLatestReport(StandardGDPR)
	if err != nil {
		t.Fatalf("获取最新报告失败: %v", err)
	}

	if report.Standard != StandardGDPR {
		t.Errorf("报告标准不匹配: %s", report.Standard)
	}
}

func TestGetDashboard(t *testing.T) {
	engine := NewComplianceEngine()

	// 执行检查
	_, err := engine.RunComplianceCheck(StandardGDPR)
	if err != nil {
		t.Fatalf("执行检查失败: %v", err)
	}

	dashboard := engine.GetDashboard()
	if dashboard == nil {
		t.Fatal("仪表盘为空")
	}

	if len(dashboard.Standards) == 0 {
		t.Error("标准列表为空")
	}

	if len(dashboard.RecentReports) == 0 {
		t.Error("最近报告为空")
	}
}

func TestComplianceLevelCalculation(t *testing.T) {
	engine := NewComplianceEngine()

	tests := []struct {
		score    float64
		expected string
	}{
		{95, "A"},
		{80, "B"},
		{65, "C"},
		{50, "D"},
	}

	for _, tt := range tests {
		level := engine.calculateComplianceLevel(tt.score)
		if level != tt.expected {
			t.Errorf("分数 %f: 期望等级 %s，得到 %s", tt.score, tt.expected, level)
		}
	}
}

func TestStandardInfoStructure(t *testing.T) {
	engine := NewComplianceEngine()

	standards := engine.GetStandards()
	for _, std := range standards {
		if std.ID == "" {
			t.Error("标准ID为空")
		}
		if std.Name == "" {
			t.Error("标准名称为空")
		}
		if std.Version == "" {
			t.Error("标准版本为空")
		}
	}
}

func TestCheckItemStructure(t *testing.T) {
	engine := NewComplianceEngine()

	checks, err := engine.GetCheckItems(StandardISO27001)
	if err != nil {
		t.Fatalf("获取ISO27001检查项失败: %v", err)
	}

	for _, check := range checks {
		if check.ID == "" {
			t.Error("检查项ID为空")
		}
		if check.Standard != StandardISO27001 {
			t.Errorf("标准不匹配: %s", check.Standard)
		}
		if check.Category == "" {
			t.Error("检查项分类为空")
		}
		if check.Severity == "" {
			t.Error("严重程度为空")
		}
	}
}

func TestRunAllStandards(t *testing.T) {
	engine := NewComplianceEngine()

	standards := []ComplianceStandard{StandardGDPR, StandardMLPS2, StandardISO27001}

	for _, std := range standards {
		report, err := engine.RunComplianceCheck(std)
		if err != nil {
			t.Errorf("执行 %s 检查失败: %v", std, err)
			continue
		}

		if report.TotalChecks == 0 {
			t.Errorf("%s 检查总数为0", std)
		}

		if report.ComplianceLevel == "" {
			t.Errorf("%s 合规等级为空", std)
		}
	}

	// 验证所有报告都已生成
	reports := engine.GetReports()
	if len(reports) != 3 {
		t.Errorf("期望3份报告，得到 %d", len(reports))
	}
}
