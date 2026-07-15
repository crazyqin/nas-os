package compliancescan

import (
	"testing"
)

func TestNewScanner(t *testing.T) {
	s := NewScanner()
	if s == nil {
		t.Fatal("NewScanner 返回 nil")
	}
	standards := s.ListStandards()
	if len(standards) == 0 {
		t.Error("应有默认标准")
	}
}

func TestSTIGScan(t *testing.T) {
	s := NewScanner()
	report := s.RunScan(StandardSTIG)
	if report == nil {
		t.Fatal("STIG 扫描不应为空")
	}
	if report.TotalRules == 0 {
		t.Error("STIG 应有规则")
	}
	if report.Passed == 0 {
		t.Error("默认配置应有通过的规则")
	}
}

func TestFIPSScan(t *testing.T) {
	s := NewScanner()
	report := s.RunScan(StandardFIPS)
	if report == nil {
		t.Fatal("FIPS 扫描不应为空")
	}
}

func TestGDPRScan(t *testing.T) {
	s := NewScanner()
	report := s.RunScan(StandardGDPR)
	if report == nil {
		t.Fatal("GDPR 扫描不应为空")
	}
}

func TestEncryptionFail(t *testing.T) {
	s := NewScanner()
	s.SetContext(&ScanContext{
		EncryptionEnabled: false,
		MFAEnabled:        true,
		AuditEnabled:      true,
		PasswordMinLen:    14,
		FirewallEnabled:   true,
		TLSEnabled:        true,
	})

	report := s.RunScan(StandardSTIG)
	if report.Score == 100 {
		t.Error("加密未启用时不应得满分")
	}
	foundFail := false
	for _, r := range report.Results {
		if r.Rule.ID == "STIG-001" && r.Result.Status == StatusFail {
			foundFail = true
		}
	}
	if !foundFail {
		t.Error("STIG-001 应检测到加密未启用")
	}
}

func TestPasswordPolicyFail(t *testing.T) {
	s := NewScanner()
	s.SetContext(&ScanContext{
		EncryptionEnabled: true, MFAEnabled: true, AuditEnabled: true,
		PasswordMinLen: 8, FirewallEnabled: true, TLSEnabled: true,
	})

	report := s.RunScan(StandardSTIG)
	for _, r := range report.Results {
		if r.Rule.ID == "STIG-004" && r.Result.Status != StatusFail {
			t.Error("密码长度 8 应判定为失败")
		}
	}
}

func TestRunAllScans(t *testing.T) {
	s := NewScanner()
	results := s.RunAllScans()
	if len(results) == 0 {
		t.Error("应返回扫描结果")
	}
	for std, report := range results {
		if report.TotalRules == 0 {
			t.Errorf("标准 %s 应有规则", std)
		}
	}
}

func TestFormatReport(t *testing.T) {
	s := NewScanner()
	report := s.RunScan(StandardSTIG)
	output := s.FormatReport(report)
	if output == "" {
		t.Error("格式化报告不应为空")
	}
}

func TestCategorySummary(t *testing.T) {
	s := NewScanner()
	report := s.RunScan(StandardSTIG)
	for cat, summary := range report.Categories {
		if summary.Total == 0 {
			t.Errorf("分类 %s 总数不应为 0", cat)
		}
		if summary.Passed+summary.Failed+summary.Warning > summary.Total {
			t.Errorf("分类 %s 汇总不一致", cat)
		}
	}
}
