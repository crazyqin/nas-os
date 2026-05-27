package accessaudit

import (
	"testing"
	"time"
)

func TestNewAuditor(t *testing.T) {
	config := DefaultRiskScoreConfig()
	auditor := NewAuditor(config)
	if auditor == nil {
		t.Fatal("NewAuditor 返回 nil")
	}

	// 测试默认配置
	auditor2 := NewAuditor(nil)
	if auditor2 == nil {
		t.Fatal("NewAuditor(nil) 返回 nil")
	}
}

func TestRecordAccess(t *testing.T) {
	auditor := NewAuditor(nil)

	record := &AccessRecord{
		ID:       "test-001",
		UserID:   "user1",
		SourceIP: "192.168.1.1",
		Resource: "/api/test",
		Action:   "GET",
		Status:   StatusSuccess,
	}

	auditor.RecordAccess(record)

	if record.RiskScore < 0 || record.RiskScore > 100 {
		t.Errorf("风险评分超出范围: %f", record.RiskScore)
	}

	if record.RiskLevel == "" {
		t.Error("风险等级未设置")
	}
}

func TestQueryRecords(t *testing.T) {
	auditor := NewAuditor(nil)

	// 添加测试数据
	for i := 0; i < 10; i++ {
		auditor.RecordAccess(&AccessRecord{
			ID:       "test-" + string(rune('0'+i)),
			UserID:   "user1",
			SourceIP: "192.168.1.1",
			Resource: "/api/test",
			Action:   "GET",
			Status:   StatusSuccess,
		})
	}

	// 查询所有
	query := AccessQuery{}
	records := auditor.Query(query)
	if len(records) != 10 {
		t.Errorf("期望10条记录，得到 %d", len(records))
	}

	// 按用户查询
	query = AccessQuery{UserID: "user1"}
	records = auditor.Query(query)
	if len(records) != 10 {
		t.Errorf("期望10条记录，得到 %d", len(records))
	}

	// 按状态查询
	query = AccessQuery{Status: StatusSuccess}
	records = auditor.Query(query)
	if len(records) != 10 {
		t.Errorf("期望10条记录，得到 %d", len(records))
	}

	// 分页测试
	query = AccessQuery{Limit: 5, Offset: 0}
	records = auditor.Query(query)
	if len(records) != 5 {
		t.Errorf("期望5条记录，得到 %d", len(records))
	}
}

func TestGenerateReport(t *testing.T) {
	auditor := NewAuditor(nil)

	// 添加测试数据
	for i := 0; i < 20; i++ {
		status := StatusSuccess
		if i%3 == 0 {
			status = StatusFailed
		}
		auditor.RecordAccess(&AccessRecord{
			ID:       "test-" + string(rune('0'+i%10)),
			UserID:   "user" + string(rune('1'+i%3)),
			SourceIP: "192.168.1." + string(rune('1'+i%5)),
			Resource: "/api/resource" + string(rune('1'+i%3)),
			Action:   "GET",
			Status:   status,
		})
	}

	endTime := time.Now()
	startTime := endTime.Add(-1 * time.Hour)

	report, err := auditor.GenerateReport(startTime, endTime)
	if err != nil {
		t.Fatalf("生成报告失败: %v", err)
	}

	if report.TotalRecords != 20 {
		t.Errorf("期望20条记录，得到 %d", report.TotalRecords)
	}

	if report.ID == "" {
		t.Error("报告ID未生成")
	}

	if report.RiskDistribution == nil {
		t.Error("风险分布未初始化")
	}
}

func TestGenerateReportInvalidTimeRange(t *testing.T) {
	auditor := NewAuditor(nil)

	startTime := time.Now()
	endTime := startTime.Add(-1 * time.Hour)

	_, err := auditor.GenerateReport(startTime, endTime)
	if err != ErrInvalidTimeRange {
		t.Errorf("期望 ErrInvalidTimeRange，得到 %v", err)
	}
}

func TestAnomalyDetection(t *testing.T) {
	auditor := NewAuditor(nil)

	// 添加频繁失败记录
	for i := 0; i < 6; i++ {
		auditor.RecordAccess(&AccessRecord{
			ID:       "fail-" + string(rune('0'+i)),
			UserID:   "attacker",
			SourceIP: "10.0.0.1",
			Resource: "/api/login",
			Action:   "POST",
			Status:   StatusFailed,
		})
	}

	anomalies := auditor.GetAnomalies()
	if len(anomalies) == 0 {
		t.Error("未检测到异常")
	}

	// 检查是否包含频繁失败异常
	found := false
	for _, a := range anomalies {
		if a.AnomalyType == "频繁失败访问" {
			found = true
			break
		}
	}
	if !found {
		t.Error("未检测到频繁失败异常")
	}
}

func TestResolveAnomaly(t *testing.T) {
	auditor := NewAuditor(nil)

	// 添加异常记录
	auditor.RecordAccess(&AccessRecord{
		ID:       "test-001",
		UserID:   "user1",
		SourceIP: "192.168.1.1",
		Resource: "/api/test",
		Action:   "GET",
		Status:   StatusFailed,
	})

	anomalies := auditor.GetAnomalies()
	if len(anomalies) == 0 {
		t.Skip("没有异常记录可测试")
	}

	// 解决第一个异常
	anomalyID := anomalies[0].ID
	if !auditor.ResolveAnomaly(anomalyID) {
		t.Error("解决异常失败")
	}

	// 验证已解决
	anomalies = auditor.GetAnomalies()
	for _, a := range anomalies {
		if a.ID == anomalyID && !a.IsResolved {
			t.Error("异常未标记为已解决")
		}
	}

	// 测试不存在的异常
	if auditor.ResolveAnomaly("non-existent") {
		t.Error("不应该能解决不存在的异常")
	}
}

func TestRiskScoreConfig(t *testing.T) {
	config := DefaultRiskScoreConfig()

	if config.FailedAttemptWeight != 0.3 {
		t.Errorf("期望失败权重 0.3，得到 %f", config.FailedAttemptWeight)
	}

	if config.UnusualTimeWeight != 0.2 {
		t.Errorf("期望非常规时间权重 0.2，得到 %f", config.UnusualTimeWeight)
	}

	if len(config.SensitiveResources) == 0 {
		t.Error("敏感资源列表为空")
	}
}

func TestRiskLevelCalculation(t *testing.T) {
	auditor := NewAuditor(nil)

	tests := []struct {
		score    float64
		expected RiskLevel
	}{
		{10, RiskLow},
		{30, RiskLow},
		{50, RiskMedium},
		{70, RiskHigh},
		{90, RiskCritical},
	}

	for _, tt := range tests {
		level := auditor.getRiskLevel(tt.score)
		if level != tt.expected {
			t.Errorf("评分 %f: 期望 %s，得到 %s", tt.score, tt.expected, level)
		}
	}
}
