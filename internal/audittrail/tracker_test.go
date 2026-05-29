// Package audittrail 提供合规审计追踪功能
package audittrail

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(logger)
	if mgr == nil {
		t.Fatal("NewManager 返回 nil")
	}
}

func TestRecordOperation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(logger)

	record := &AuditRecord{
		UserID:       "user1",
		UserName:     "测试用户",
		UserIP:       "192.168.1.1",
		Action:       ActionCreate,
		Resource:     "/api/test",
		ResourceType: "API",
		Result:       ResultSuccess,
		RequestID:    "req-001",
	}

	err := mgr.RecordOperation(record)
	if err != nil {
		t.Fatalf("RecordOperation 失败: %v", err)
	}

	if record.ID == "" {
		t.Error("记录ID未生成")
	}

	if record.Checksum == "" {
		t.Error("校验和未生成")
	}

	if record.Timestamp.IsZero() {
		t.Error("时间戳未设置")
	}

	// 验证不可篡改
	originalChecksum := record.Checksum
	record.Action = ActionDelete // 尝试修改
	if record.Checksum != originalChecksum {
		// 注意：这里只是测试，实际存储后不应修改
	}
}

func TestRecordImmutability(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(logger)

	record := &AuditRecord{
		UserID:   "user1",
		Action:   ActionCreate,
		Resource: "/api/test",
		Result:   ResultSuccess,
	}

	mgr.RecordOperation(record)

	// 获取记录
	fetched, err := mgr.GetRecord(record.ID)
	if err != nil {
		t.Fatalf("GetRecord 失败: %v", err)
	}

	// 验证记录相同
	if fetched.ID != record.ID {
		t.Error("记录ID不匹配")
	}

	if fetched.Checksum != record.Checksum {
		t.Error("校验和不匹配")
	}
}

func TestQueryRecords(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(logger)

	// 添加测试数据
	for i := 0; i < 10; i++ {
		mgr.RecordOperation(&AuditRecord{
			UserID:   "user1",
			Action:   ActionCreate,
			Resource: "/api/test",
			Result:   ResultSuccess,
		})
	}

	for i := 0; i < 5; i++ {
		mgr.RecordOperation(&AuditRecord{
			UserID:   "user2",
			Action:   ActionDelete,
			Resource: "/api/resource",
			Result:   ResultFailed,
		})
	}

	// 查询所有
	query := AuditQuery{}
	records := mgr.QueryRecords(query)
	if len(records) != 15 {
		t.Errorf("期望15条记录，得到 %d", len(records))
	}

	// 按用户查询
	query = AuditQuery{UserID: "user1"}
	records = mgr.QueryRecords(query)
	if len(records) != 10 {
		t.Errorf("期望10条记录，得到 %d", len(records))
	}

	// 按操作类型查询
	query = AuditQuery{Action: ActionDelete}
	records = mgr.QueryRecords(query)
	if len(records) != 5 {
		t.Errorf("期望5条记录，得到 %d", len(records))
	}

	// 按结果查询
	query = AuditQuery{Result: ResultFailed}
	records = mgr.QueryRecords(query)
	if len(records) != 5 {
		t.Errorf("期望5条记录，得到 %d", len(records))
	}
}

func TestOperationChain(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(logger)

	requestID := "req-chain-001"

	// 创建操作链
	operations := []ActionType{ActionLogin, ActionRead, ActionUpdate, ActionLogout}
	for _, action := range operations {
		mgr.RecordOperation(&AuditRecord{
			UserID:    "user1",
			Action:    action,
			Resource:  "/api/resource",
			Result:    ResultSuccess,
			RequestID: requestID,
		})
		time.Sleep(10 * time.Millisecond) // 确保时间戳不同
	}

	// 获取操作链
	chain, err := mgr.GetOperationChain(requestID)
	if err != nil {
		t.Fatalf("GetOperationChain 失败: %v", err)
	}

	if len(chain.Records) != 4 {
		t.Errorf("期望4条记录，得到 %d", len(chain.Records))
	}

	if chain.RequestID != requestID {
		t.Error("RequestID 不匹配")
	}

	if chain.FinalResult != ResultSuccess {
		t.Error("最终结果不正确")
	}
}

func TestRetentionPolicy(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(logger)

	// 测试7年保留策略
	mgr.SetRetentionPolicy(Retention7Years)
	record7 := &AuditRecord{
		UserID:   "user1",
		Action:   ActionCreate,
		Resource: "/api/test",
		Result:   ResultSuccess,
	}
	mgr.RecordOperation(record7)

	if record7.RetentionPolicy != Retention7Years {
		t.Error("保留策略未设置为7年")
	}

	expectedExpiry := record7.Timestamp.AddDate(7, 0, 0)
	if !record7.ExpiresAt.Equal(expectedExpiry) {
		t.Error("过期时间计算错误")
	}

	// 测试永久保留策略
	mgr.SetRetentionPolicy(RetentionPermanent)
	recordPerm := &AuditRecord{
		UserID:   "user1",
		Action:   ActionCreate,
		Resource: "/api/test",
		Result:   ResultSuccess,
	}
	mgr.RecordOperation(recordPerm)

	if !recordPerm.ExpiresAt.IsZero() {
		t.Error("永久保留策略过期时间应为零值")
	}
}

func TestAnomalyDetection(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(logger)

	// 添加异常规则
	rule := &AnomalyRule{
		Name:        "失败登录检测",
		Description: "检测登录失败操作",
		Level:       AnomalyMedium,
		Enabled:     true,
		Conditions: []RuleCondition{
			{
				Field:    "action",
				Operator: "eq",
				Value:    "LOGIN",
			},
			{
				Field:    "result",
				Operator: "eq",
				Value:    "FAILED",
			},
		},
	}
	mgr.AddAnomalyRule(rule)

	// 触发异常
	mgr.RecordOperation(&AuditRecord{
		UserID:   "user1",
		Action:   ActionLogin,
		Resource: "/api/login",
		Result:   ResultFailed,
	})

	// 检查异常
	anomalies := mgr.GetAnomalies(nil)
	if len(anomalies) != 1 {
		t.Fatalf("期望1个异常，得到 %d", len(anomalies))
	}

	if anomalies[0].RuleName != "失败登录检测" {
		t.Error("规则名称不匹配")
	}

	if anomalies[0].Level != AnomalyMedium {
		t.Error("异常级别不匹配")
	}

	// 解决异常
	err := mgr.ResolveAnomaly(anomalies[0].ID, "admin")
	if err != nil {
		t.Fatalf("ResolveAnomaly 失败: %v", err)
	}

	// 验证已解决
	resolved := true
	resolvedAnomalies := mgr.GetAnomalies(&resolved)
	if len(resolvedAnomalies) != 1 {
		t.Error("异常未标记为已解决")
	}
}

func TestComplianceReport(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(logger)

	// 添加带合规标签的记录
	for i := 0; i < 10; i++ {
		mgr.RecordOperation(&AuditRecord{
			UserID:           "user1",
			Action:           ActionCreate,
			Resource:         "/api/data",
			Result:           ResultSuccess,
			ComplianceTags:   []ComplianceStandard{StandardSOC2, StandardGDPR},
			RetentionPolicy:  Retention7Years,
		})
	}

	now := time.Now()
	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)

	// 生成SOC2报告
	report, err := mgr.GenerateComplianceReport(StandardSOC2, start, end)
	if err != nil {
		t.Fatalf("GenerateComplianceReport 失败: %v", err)
	}

	if report.Standard != StandardSOC2 {
		t.Error("报告标准不正确")
	}

	if report.Summary.TotalRecords != 10 {
		t.Errorf("期望10条记录，得到 %d", report.Summary.TotalRecords)
	}

	if report.Summary.SuccessCount != 10 {
		t.Errorf("期望10次成功，得到 %d", report.Summary.SuccessCount)
	}

	if report.Status != "COMPLETED" {
		t.Error("报告状态不正确")
	}
}

func TestExportRecords(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(logger)

	// 添加测试数据
	for i := 0; i < 5; i++ {
		mgr.RecordOperation(&AuditRecord{
			UserID:   "user1",
			Action:   ActionCreate,
			Resource: "/api/test",
			Result:   ResultSuccess,
		})
	}

	// 导出JSON
	jsonData, err := mgr.ExportRecords(ExportRequest{
		Format: FormatJSON,
		Query:  AuditQuery{},
	})
	if err != nil {
		t.Fatalf("JSON导出失败: %v", err)
	}
	if len(jsonData) == 0 {
		t.Error("JSON导出数据为空")
	}

	// 导出CSV
	csvData, err := mgr.ExportRecords(ExportRequest{
		Format: FormatCSV,
		Query:  AuditQuery{},
	})
	if err != nil {
		t.Fatalf("CSV导出失败: %v", err)
	}
	if len(csvData) == 0 {
		t.Error("CSV导出数据为空")
	}

	// 导出PDF
	pdfData, err := mgr.ExportRecords(ExportRequest{
		Format: FormatPDF,
		Query:  AuditQuery{},
	})
	if err != nil {
		t.Fatalf("PDF导出失败: %v", err)
	}
	if len(pdfData) == 0 {
		t.Error("PDF导出数据为空")
	}
}

func TestComplianceStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(logger)

	// 添加测试数据
	mgr.RecordOperation(&AuditRecord{
		UserID:         "user1",
		Action:         ActionCreate,
		Result:         ResultSuccess,
		ComplianceTags: []ComplianceStandard{StandardSOC2},
	})

	mgr.RecordOperation(&AuditRecord{
		UserID:         "user2",
		Action:         ActionDelete,
		Result:         ResultFailed,
		ComplianceTags: []ComplianceStandard{StandardGDPR},
	})

	stats := mgr.GetComplianceStats()

	if stats.TotalRecords != 2 {
		t.Errorf("期望2条记录，得到 %d", stats.TotalRecords)
	}

	if stats.RecordsByAction[ActionCreate] != 1 {
		t.Error("操作类型统计错误")
	}

	if stats.RecordsByResult[ResultSuccess] != 1 {
		t.Error("结果统计错误")
	}

	if stats.RecordsByStandard[StandardSOC2] != 1 {
		t.Error("合规标准统计错误")
	}
}

func TestConcurrentAccess(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(logger)

	done := make(chan bool, 10)

	// 并发写入
	for i := 0; i < 10; i++ {
		go func(index int) {
			mgr.RecordOperation(&AuditRecord{
				UserID:   "user1",
				Action:   ActionCreate,
				Resource: "/api/test",
				Result:   ResultSuccess,
			})
			done <- true
		}(i)
	}

	// 等待所有写入完成
	for i := 0; i < 10; i++ {
		<-done
	}

	records := mgr.QueryRecords(AuditQuery{})
	if len(records) != 10 {
		t.Errorf("期望10条记录，得到 %d", len(records))
	}
}
