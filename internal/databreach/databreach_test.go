package databreach

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
	if m.incidents == nil {
		t.Error("incidents 未初始化")
	}
	if m.notifications == nil {
		t.Error("notifications 未初始化")
	}
	if m.reports == nil {
		t.Error("reports 未初始化")
	}
	if m.timers == nil {
		t.Error("timers 未初始化")
	}
}

func TestReportBreach(t *testing.T) {
	m := NewManager()

	// 有效事件
	incident := BreachIncident{
		BreachType:      BreachTypeDataLeak,
		ImpactScope:     "用户数据库",
		Classification:  ClassificationPII,
		AffectedRecords: 5000,
		Description:     "测试泄露事件",
	}

	result, err := m.ReportBreach(incident)
	if err != nil {
		t.Fatalf("ReportBreach 失败: %v", err)
	}

	if result.ID == "" {
		t.Error("事件ID为空")
	}
	if result.Status != StatusReported {
		t.Errorf("期望状态 %s, 得到 %s", StatusReported, result.Status)
	}
	if result.ReportedAt.IsZero() {
		t.Error("ReportedAt 为零值")
	}
	if result.UpdatedAt.IsZero() {
		t.Error("UpdatedAt 为零值")
	}

	// 测试自定义发现时间
	incident2 := BreachIncident{
		BreachType:      BreachTypeRansomware,
		ImpactScope:     "文件服务器",
		Classification:  ClassificationConfidential,
		AffectedRecords: 100,
		DiscoveredAt:    time.Now().Add(-24 * time.Hour),
	}

	result2, err := m.ReportBreach(incident2)
	if err != nil {
		t.Fatalf("ReportBreach 失败: %v", err)
	}
	if result2.DiscoveredAt.IsZero() {
		t.Error("自定义 DiscoveredAt 未保留")
	}
}

func TestReportBreachInvalidType(t *testing.T) {
	m := NewManager()

	incident := BreachIncident{
		BreachType:      "InvalidType",
		Classification:  ClassificationPII,
		AffectedRecords: 100,
	}

	_, err := m.ReportBreach(incident)
	if err != ErrInvalidBreachType {
		t.Errorf("期望 ErrInvalidBreachType, 得到 %v", err)
	}
}

func TestReportBreachInvalidClassification(t *testing.T) {
	m := NewManager()

	incident := BreachIncident{
		BreachType:      BreachTypeDataLeak,
		Classification:  "InvalidClass",
		AffectedRecords: 100,
	}

	_, err := m.ReportBreach(incident)
	if err != ErrInvalidClassification {
		t.Errorf("期望 ErrInvalidClassification, 得到 %v", err)
	}
}

func TestGetBreach(t *testing.T) {
	m := NewManager()

	// 创建事件
	incident := BreachIncident{
		BreachType:      BreachTypeUnauthorizedAccess,
		ImpactScope:     "内部系统",
		Classification:  ClassificationInternal,
		AffectedRecords: 50,
	}

	created, err := m.ReportBreach(incident)
	if err != nil {
		t.Fatalf("ReportBreach 失败: %v", err)
	}

	// 获取事件
	got, err := m.GetBreach(created.ID)
	if err != nil {
		t.Fatalf("GetBreach 失败: %v", err)
	}

	if got.ID != created.ID {
		t.Errorf("ID 不匹配: 期望 %s, 得到 %s", created.ID, got.ID)
	}
	if got.BreachType != BreachTypeUnauthorizedAccess {
		t.Errorf("BreachType 不匹配: 期望 %s, 得到 %s", BreachTypeUnauthorizedAccess, got.BreachType)
	}

	// 获取不存在的事件
	_, err = m.GetBreach("BREACH-999999")
	if err != ErrBreachNotFound {
		t.Errorf("期望 ErrBreachNotFound, 得到 %v", err)
	}
}

func TestListBreaches(t *testing.T) {
	m := NewManager()

	// 创建多个事件
	types := []BreachType{BreachTypeDataLeak, BreachTypeRansomware, BreachTypeMisconfig}
	for _, bt := range types {
		_, err := m.ReportBreach(BreachIncident{
			BreachType:      bt,
			ImpactScope:     "测试",
			Classification:  ClassificationInternal,
			AffectedRecords: 100,
		})
		if err != nil {
			t.Fatalf("ReportBreach 失败: %v", err)
		}
	}

	// 列出全部
	all, err := m.ListBreaches("")
	if err != nil {
		t.Fatalf("ListBreaches 失败: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("期望 3 个事件, 得到 %d", len(all))
	}

	// 按状态筛选
	reported, err := m.ListBreaches("Reported")
	if err != nil {
		t.Fatalf("ListBreaches 失败: %v", err)
	}
	if len(reported) != 3 {
		t.Errorf("期望 3 个 Reported 事件, 得到 %d", len(reported))
	}

	// 无效状态
	_, err = m.ListBreaches("InvalidStatus")
	if err != ErrInvalidStatus {
		t.Errorf("期望 ErrInvalidStatus, 得到 %v", err)
	}

	// 更新一个事件状态后再筛选
	m.UpdateBreachStatus(all[0].ID, "Investigating")
	investigating, _ := m.ListBreaches("Investigating")
	if len(investigating) != 1 {
		t.Errorf("期望 1 个 Investigating 事件, 得到 %d", len(investigating))
	}
}

func TestUpdateBreachStatus(t *testing.T) {
	m := NewManager()

	incident, _ := m.ReportBreach(BreachIncident{
		BreachType:      BreachTypeDataLeak,
		ImpactScope:     "测试",
		Classification:  ClassificationInternal,
		AffectedRecords: 100,
	})

	// 更新为调查中
	err := m.UpdateBreachStatus(incident.ID, "Investigating")
	if err != nil {
		t.Fatalf("UpdateBreachStatus 失败: %v", err)
	}

	got, _ := m.GetBreach(incident.ID)
	if got.Status != StatusInvestigating {
		t.Errorf("期望状态 %s, 得到 %s", StatusInvestigating, got.Status)
	}

	// 更新为已遏制
	err = m.UpdateBreachStatus(incident.ID, "Contained")
	if err != nil {
		t.Fatalf("UpdateBreachStatus 失败: %v", err)
	}

	// 更新不存在的事件
	err = m.UpdateBreachStatus("BREACH-999999", "Resolved")
	if err != ErrBreachNotFound {
		t.Errorf("期望 ErrBreachNotFound, 得到 %v", err)
	}

	// 无效状态
	err = m.UpdateBreachStatus(incident.ID, "InvalidStatus")
	if err != ErrInvalidStatus {
		t.Errorf("期望 ErrInvalidStatus, 得到 %v", err)
	}
}

func TestNotificationTimer(t *testing.T) {
	m := NewManager()

	incident, _ := m.ReportBreach(BreachIncident{
		BreachType:      BreachTypeDataLeak,
		ImpactScope:     "测试",
		Classification:  ClassificationPII,
		AffectedRecords: 1000,
	})

	// 启动计时器
	err := m.StartNotificationTimer(incident.ID)
	if err != nil {
		t.Fatalf("StartNotificationTimer 失败: %v", err)
	}

	// 获取截止时间
	deadline, err := m.GetNotificationDeadline(incident.ID)
	if err != nil {
		t.Fatalf("GetNotificationDeadline 失败: %v", err)
	}

	// 验证截止时间约为72小时后
	now := time.Now()
	expectedDeadline := now.Add(72 * time.Hour)
	if deadline.Before(expectedDeadline.Add(-1*time.Minute)) || deadline.After(expectedDeadline.Add(1*time.Minute)) {
		t.Errorf("截止时间不正确: 期望约 %v, 得到 %v", expectedDeadline, deadline)
	}

	// 重复启动应失败
	err = m.StartNotificationTimer(incident.ID)
	if err != ErrNotificationTimerAlreadyStarted {
		t.Errorf("期望 ErrNotificationTimerAlreadyStarted, 得到 %v", err)
	}

	// 不存在的事件
	err = m.StartNotificationTimer("BREACH-999999")
	if err != ErrBreachNotFound {
		t.Errorf("期望 ErrBreachNotFound, 得到 %v", err)
	}
}

func TestGetNotificationDeadlineNotSet(t *testing.T) {
	m := NewManager()

	incident, _ := m.ReportBreach(BreachIncident{
		BreachType:      BreachTypeDataLeak,
		ImpactScope:     "测试",
		Classification:  ClassificationInternal,
		AffectedRecords: 100,
	})

	// 未启动计时器
	_, err := m.GetNotificationDeadline(incident.ID)
	if err != ErrNotificationDeadlineNotSet {
		t.Errorf("期望 ErrNotificationDeadlineNotSet, 得到 %v", err)
	}
}

func TestAddAndGetNotifications(t *testing.T) {
	m := NewManager()

	incident, _ := m.ReportBreach(BreachIncident{
		BreachType:      BreachTypeDataLeak,
		ImpactScope:     "测试",
		Classification:  ClassificationPII,
		AffectedRecords: 5000,
	})

	// 添加通知
	record := NotificationRecord{
		Recipient: "数据保护局",
		Method:    MethodEmail,
		Content:   "数据泄露通知",
	}

	err := m.AddNotification(incident.ID, record)
	if err != nil {
		t.Fatalf("AddNotification 失败: %v", err)
	}

	// 获取通知
	records, err := m.GetNotifications(incident.ID)
	if err != nil {
		t.Fatalf("GetNotifications 失败: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("期望 1 条通知, 得到 %d", len(records))
	}

	if records[0].Recipient != "数据保护局" {
		t.Errorf("接收方不匹配: 期望 '数据保护局', 得到 '%s'", records[0].Recipient)
	}
	if records[0].Method != MethodEmail {
		t.Errorf("通知方式不匹配: 期望 %s, 得到 %s", MethodEmail, records[0].Method)
	}
	if records[0].Status != NotifyStatusPending {
		t.Errorf("通知状态不匹配: 期望 %s, 得到 %s", NotifyStatusPending, records[0].Status)
	}
	if records[0].ID == "" {
		t.Error("通知ID为空")
	}
	if records[0].BreachID != incident.ID {
		t.Errorf("BreachID 不匹配: 期望 %s, 得到 %s", incident.ID, records[0].BreachID)
	}

	// 添加多条通知
	m.AddNotification(incident.ID, NotificationRecord{
		Recipient: "受影响用户",
		Method:    MethodLetter,
		Status:    NotifyStatusSent,
	})

	records, _ = m.GetNotifications(incident.ID)
	if len(records) != 2 {
		t.Errorf("期望 2 条通知, 得到 %d", len(records))
	}

	// 不存在的事件
	_, err = m.GetNotifications("BREACH-999999")
	if err != ErrBreachNotFound {
		t.Errorf("期望 ErrBreachNotFound, 得到 %v", err)
	}

	err = m.AddNotification("BREACH-999999", record)
	if err != ErrBreachNotFound {
		t.Errorf("期望 ErrBreachNotFound, 得到 %v", err)
	}
}

func TestGetNotificationsEmpty(t *testing.T) {
	m := NewManager()

	incident, _ := m.ReportBreach(BreachIncident{
		BreachType:      BreachTypeMisconfig,
		ImpactScope:     "测试",
		Classification:  ClassificationInternal,
		AffectedRecords: 10,
	})

	records, err := m.GetNotifications(incident.ID)
	if err != nil {
		t.Fatalf("GetNotifications 失败: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("期望 0 条通知, 得到 %d", len(records))
	}
}

func TestGenerateComplianceReport(t *testing.T) {
	m := NewManager()

	// 创建PII泄露事件
	incident, _ := m.ReportBreach(BreachIncident{
		BreachType:      BreachTypeDataLeak,
		ImpactScope:     "用户数据库",
		Classification:  ClassificationPII,
		AffectedRecords: 10000,
	})

	// 启动通知计时器
	m.StartNotificationTimer(incident.ID)

	// 生成报告
	report, err := m.GenerateComplianceReport(incident.ID)
	if err != nil {
		t.Fatalf("GenerateComplianceReport 失败: %v", err)
	}

	if report.ID == "" {
		t.Error("报告ID为空")
	}
	if report.BreachID != incident.ID {
		t.Errorf("BreachID 不匹配: 期望 %s, 得到 %s", incident.ID, report.BreachID)
	}
	if !report.GDPR33 {
		t.Error("GDPR33 应为 true")
	}
	if !report.GDPR34 {
		t.Error("PII 数据应触发 GDPR34")
	}
	if report.Completed {
		t.Error("未完成所有通知时 Completed 应为 false")
	}

	// 内部数据不应触发 GDPR34
	incident2, _ := m.ReportBreach(BreachIncident{
		BreachType:      BreachTypeMisconfig,
		ImpactScope:     "内部系统",
		Classification:  ClassificationInternal,
		AffectedRecords: 100,
	})

	report2, _ := m.GenerateComplianceReport(incident2.ID)
	if report2.GDPR34 {
		t.Error("内部数据不应触发 GDPR34")
	}

	// 不存在的事件
	_, err = m.GenerateComplianceReport("BREACH-999999")
	if err != ErrBreachNotFound {
		t.Errorf("期望 ErrBreachNotFound, 得到 %v", err)
	}
}

func TestCalculateRiskScore(t *testing.T) {
	m := NewManager()

	tests := []struct {
		name     string
		incident BreachIncident
		minScore int
		maxScore int
	}{
		{
			name: "高风险 - 勒索软件+PHI+大量记录",
			incident: BreachIncident{
				BreachType:      BreachTypeRansomware,
				Classification:  ClassificationPHI,
				AffectedRecords: 2000000,
				ImpactScope:     "医院系统",
			},
			minScore: 90,
			maxScore: 100,
		},
		{
			name: "中风险 - 数据泄漏+内部数据",
			incident: BreachIncident{
				BreachType:      BreachTypeDataLeak,
				Classification:  ClassificationInternal,
				AffectedRecords: 500,
				ImpactScope:     "内部文档",
			},
			minScore: 30,
			maxScore: 50,
		},
		{
			name: "低风险 - 配置错误+公开数据",
			incident: BreachIncident{
				BreachType:      BreachTypeMisconfig,
				Classification:  ClassificationPublic,
				AffectedRecords: 50,
				ImpactScope:     "公开网站",
			},
			minScore: 1,
			maxScore: 25,
		},
		{
			name: "PII + 未授权访问",
			incident: BreachIncident{
				BreachType:      BreachTypeUnauthorizedAccess,
				Classification:  ClassificationPII,
				AffectedRecords: 50000,
				ImpactScope:     "用户账户",
			},
			minScore: 60,
			maxScore: 80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, err := m.ReportBreach(tt.incident)
			if err != nil {
				t.Fatalf("ReportBreach 失败: %v", err)
			}

			score, err := m.CalculateRiskScore(created.ID)
			if err != nil {
				t.Fatalf("CalculateRiskScore 失败: %v", err)
			}

			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("风险评分 %d 不在期望范围 [%d, %d]", score, tt.minScore, tt.maxScore)
			}
		})
	}

	// 不存在的事件
	_, err := m.CalculateRiskScore("BREACH-999999")
	if err != ErrBreachNotFound {
		t.Errorf("期望 ErrBreachNotFound, 得到 %v", err)
	}
}

func TestGetBreachStatistics(t *testing.T) {
	m := NewManager()

	// 创建不同类型的事件
	breachTypes := []BreachType{
		BreachTypeDataLeak,
		BreachTypeDataLeak,
		BreachTypeRansomware,
		BreachTypeMisconfig,
	}

	for _, bt := range breachTypes {
		_, err := m.ReportBreach(BreachIncident{
			BreachType:      bt,
			ImpactScope:     "测试",
			Classification:  ClassificationInternal,
			AffectedRecords: 100,
		})
		if err != nil {
			t.Fatalf("ReportBreach 失败: %v", err)
		}
	}

	stats, err := m.GetBreachStatistics()
	if err != nil {
		t.Fatalf("GetBreachStatistics 失败: %v", err)
	}

	if stats["total"] != 4 {
		t.Errorf("期望 total=4, 得到 %d", stats["total"])
	}
	if stats["Reported"] != 4 {
		t.Errorf("期望 Reported=4, 得到 %d", stats["Reported"])
	}
	if stats["DataLeak"] != 2 {
		t.Errorf("期望 DataLeak=2, 得到 %d", stats["DataLeak"])
	}
	if stats["Ransomware"] != 1 {
		t.Errorf("期望 Ransomware=1, 得到 %d", stats["Ransomware"])
	}
	if stats["Misconfig"] != 1 {
		t.Errorf("期望 Misconfig=1, 得到 %d", stats["Misconfig"])
	}
}

func TestExportGDPRReport(t *testing.T) {
	m := NewManager()

	// 创建PII泄露事件
	incident, _ := m.ReportBreach(BreachIncident{
		BreachType:      BreachTypeDataLeak,
		ImpactScope:     "用户数据库",
		Classification:  ClassificationPII,
		AffectedRecords: 10000,
		Description:     "SQL注入导致数据泄露",
	})

	// 启动通知计时器
	m.StartNotificationTimer(incident.ID)

	// 添加通知记录
	m.AddNotification(incident.ID, NotificationRecord{
		Recipient: "数据保护局",
		Method:    MethodEmail,
		Status:    NotifyStatusSent,
	})

	// 导出报告
	data, err := m.ExportGDPRReport(incident.ID)
	if err != nil {
		t.Fatalf("ExportGDPRReport 失败: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("导出报告为空")
	}

	// 验证是有效JSON
	var report map[string]interface{}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("导出报告不是有效JSON: %v", err)
	}

	// 验证关键字段
	if report["report_type"] == nil {
		t.Error("report_type 字段缺失")
	}
	if report["incident_id"] != incident.ID {
		t.Errorf("incident_id 不匹配: 期望 %s, 得到 %v", incident.ID, report["incident_id"])
	}
	if report["classification"] != string(ClassificationPII) {
		t.Errorf("classification 不匹配: 期望 %s, 得到 %v", ClassificationPII, report["classification"])
	}

	// 验证包含 Article 33/34
	reportType, ok := report["report_type"].(string)
	if !ok {
		t.Error("report_type 不是字符串")
	}
	if reportType != "GDPR Article 33/34 - 监管机构及数据主体通知" {
		t.Errorf("report_type 不正确: %s", reportType)
	}

	// 内部数据导出（仅Article 33）
	incident2, _ := m.ReportBreach(BreachIncident{
		BreachType:      BreachTypeMisconfig,
		ImpactScope:     "内部系统",
		Classification:  ClassificationInternal,
		AffectedRecords: 100,
	})

	data2, _ := m.ExportGDPRReport(incident2.ID)
	var report2 map[string]interface{}
	json.Unmarshal(data2, &report2)

	reportType2 := report2["report_type"].(string)
	if reportType2 != "GDPR Article 33 - 监管机构通知" {
		t.Errorf("内部数据 report_type 不正确: %s", reportType2)
	}

	// 不存在的事件
	_, err = m.ExportGDPRReport("BREACH-999999")
	if err != ErrBreachNotFound {
		t.Errorf("期望 ErrBreachNotFound, 得到 %v", err)
	}
}

func TestBreachTypes(t *testing.T) {
	types := []BreachType{
		BreachTypeUnauthorizedAccess,
		BreachTypeDataLeak,
		BreachTypeRansomware,
		BreachTypeInsiderThreat,
		BreachTypeMisconfig,
		BreachTypeThirdParty,
	}

	m := NewManager()
	for _, bt := range types {
		_, err := m.ReportBreach(BreachIncident{
			BreachType:      bt,
			ImpactScope:     "测试",
			Classification:  ClassificationInternal,
			AffectedRecords: 100,
		})
		if err != nil {
			t.Errorf("泄露类型 %s 验证失败: %v", bt, err)
		}
	}
}

func TestDataClassifications(t *testing.T) {
	classifications := []DataClassification{
		ClassificationPublic,
		ClassificationInternal,
		ClassificationConfidential,
		ClassificationRestricted,
		ClassificationPII,
		ClassificationPHI,
	}

	m := NewManager()
	for _, dc := range classifications {
		_, err := m.ReportBreach(BreachIncident{
			BreachType:      BreachTypeDataLeak,
			ImpactScope:     "测试",
			Classification:  dc,
			AffectedRecords: 100,
		})
		if err != nil {
			t.Errorf("数据分类 %s 验证失败: %v", dc, err)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager()

	// 并发创建事件
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := m.ReportBreach(BreachIncident{
				BreachType:      BreachTypeDataLeak,
				ImpactScope:     "并发测试",
				Classification:  ClassificationInternal,
				AffectedRecords: 100,
			})
			if err != nil {
				t.Errorf("并发 ReportBreach 失败: %v", err)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	stats, _ := m.GetBreachStatistics()
	if stats["total"] != 10 {
		t.Errorf("期望 10 个事件, 得到 %d", stats["total"])
	}
}

func TestFullWorkflow(t *testing.T) {
	m := NewManager()

	// 1. 报告泄露
	incident, err := m.ReportBreach(BreachIncident{
		BreachType:      BreachTypeRansomware,
		ImpactScope:     "生产数据库",
		Classification:  ClassificationPII,
		AffectedRecords: 50000,
		Description:     "勒索软件攻击导致用户数据泄露",
	})
	if err != nil {
		t.Fatalf("报告泄露失败: %v", err)
	}

	// 2. 启动通知计时器
	err = m.StartNotificationTimer(incident.ID)
	if err != nil {
		t.Fatalf("启动计时器失败: %v", err)
	}

	// 3. 更新状态
	m.UpdateBreachStatus(incident.ID, "Investigating")

	// 4. 添加通知
	m.AddNotification(incident.ID, NotificationRecord{
		Recipient: "数据保护局",
		Method:    MethodEmail,
		Status:    NotifyStatusSent,
	})
	m.AddNotification(incident.ID, NotificationRecord{
		Recipient: "受影响用户",
		Method:    MethodLetter,
		Status:    NotifyStatusSent,
	})

	// 5. 更新状态为已解决
	m.UpdateBreachStatus(incident.ID, "Resolved")

	// 6. 计算风险评分
	score, _ := m.CalculateRiskScore(incident.ID)
	if score < 70 {
		t.Errorf("高风险事件评分应 >= 70, 得到 %d", score)
	}

	// 7. 生成合规报告
	report, _ := m.GenerateComplianceReport(incident.ID)
	if !report.GDPR33 {
		t.Error("GDPR33 应为 true")
	}
	if !report.GDPR34 {
		t.Error("PII 数据应触发 GDPR34")
	}
	if !report.Completed {
		t.Error("所有通知已发送且状态为 Resolved 时 Completed 应为 true")
	}

	// 8. 导出GDPR报告
	gdprData, _ := m.ExportGDPRReport(incident.ID)
	if len(gdprData) == 0 {
		t.Error("GDPR报告为空")
	}

	// 9. 验证统计
	stats, _ := m.GetBreachStatistics()
	if stats["total"] != 1 {
		t.Errorf("期望 total=1, 得到 %d", stats["total"])
	}
	if stats["Ransomware"] != 1 {
		t.Errorf("期望 Ransomware=1, 得到 %d", stats["Ransomware"])
	}
}
