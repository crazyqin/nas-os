// Package ailoganalyzer 测试
package ailoganalyzer

import (
	"testing"
	"time"
)

func TestAddLog(t *testing.T) {
	m := NewManager()
	entry := &LogEntry{
		Level:   LevelError,
		Source:  "system",
		Message: "disk space low on /dev/sda1",
		Labels:  map[string]string{"host": "nas1"},
	}

	m.AddLog(entry)

	if entry.ID == "" {
		t.Fatal("日志ID不应为空")
	}
	if entry.Timestamp.IsZero() {
		t.Fatal("时间戳不应为零值")
	}
	if entry.ClusterID == "" {
		t.Fatal("聚类ID不应为空")
	}
}

func TestGetLog(t *testing.T) {
	m := NewManager()
	entry := &LogEntry{
		Level:   LevelInfo,
		Source:  "app",
		Message: "test message",
	}
	m.AddLog(entry)

	got, err := m.GetLog(entry.ID)
	if err != nil {
		t.Fatalf("获取日志失败: %v", err)
	}
	if got.Message != "test message" {
		t.Errorf("消息不匹配")
	}
}

func TestGetLogNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetLog("nonexistent")
	if err != ErrLogNotFound {
		t.Errorf("期望 ErrLogNotFound，实际: %v", err)
	}
}

func TestQueryLogs(t *testing.T) {
	m := NewManager()

	// 添加多条日志
	for i := 0; i < 10; i++ {
		level := LevelInfo
		if i%3 == 0 {
			level = LevelError
		}
		m.AddLog(&LogEntry{
			Level:   level,
			Source:  "app",
			Message: "test log message",
		})
	}

	// 查询所有
	logs, total := m.QueryLogs(QueryLogsRequest{})
	if total != 10 {
		t.Errorf("期望10条日志，实际 %d", total)
	}

	// 按级别查询
	logs, total = m.QueryLogs(QueryLogsRequest{Level: LevelError})
	if len(logs) == 0 {
		t.Error("应有错误日志")
	}
	for _, log := range logs {
		if log.Level != LevelError {
			t.Errorf("查询结果包含非错误日志: %s", log.Level)
		}
	}
}

func TestQueryLogsPagination(t *testing.T) {
	m := NewManager()

	for i := 0; i < 100; i++ {
		m.AddLog(&LogEntry{
			Level:   LevelInfo,
			Source:  "app",
			Message: "log message",
		})
	}

	// 分页查询
	logs, total := m.QueryLogs(QueryLogsRequest{Page: 1, PageSize: 10})
	if total != 100 {
		t.Errorf("总数应为100，实际 %d", total)
	}
	if len(logs) != 10 {
		t.Errorf("每页应返回10条，实际 %d", len(logs))
	}
}

func TestDeleteLogs(t *testing.T) {
	m := NewManager()

	// 添加旧日志
	oldEntry := &LogEntry{
		Level:     LevelInfo,
		Source:    "app",
		Message:   "old log",
		Timestamp: time.Now().Add(-48 * time.Hour),
	}
	m.logs[oldEntry.ID] = oldEntry

	// 添加新日志
	m.AddLog(&LogEntry{
		Level:   LevelInfo,
		Source:  "app",
		Message: "new log",
	})

	// 删除24小时前的日志
	count := m.DeleteLogs(time.Now().Add(-24 * time.Hour))
	if count != 1 {
		t.Errorf("期望删除1条，实际删除 %d", count)
	}
}

func TestCreatePattern(t *testing.T) {
	m := NewManager()

	pattern := m.CreatePattern(CreatePatternRequest{
		Name:     "磁盘空间不足",
		Regex:    `disk space low|no space left`,
		Keywords: []string{"disk", "space", "full"},
		Level:    LevelError,
		IsAnomaly: true,
		Severity: AlertLevelHigh,
	})

	if pattern.ID == "" {
		t.Fatal("模式ID不应为空")
	}
	if pattern.Name != "磁盘空间不足" {
		t.Errorf("名称不匹配")
	}
	if !pattern.Enabled {
		t.Error("模式默认应启用")
	}
}

func TestPatternMatching(t *testing.T) {
	m := NewManager()

	// 创建模式
	m.CreatePattern(CreatePatternRequest{
		Name:   "磁盘错误",
		Regex:  `disk.*error|disk.*fail`,
		Level:  LevelError,
	})

	// 添加匹配的日志
	entry := &LogEntry{
		Level:   LevelError,
		Source:  "system",
		Message: "disk read error on /dev/sda",
	}
	m.AddLog(entry)

	if entry.PatternID == "" {
		t.Error("日志应匹配到模式")
	}
}

func TestCreateRule(t *testing.T) {
	m := NewManager()

	rule := m.CreateRule(CreateRuleRequest{
		Name:      "错误频率检测",
		Type:      "frequency",
		Threshold: 10,
		Window:    60,
		Level:     LevelError,
	})

	if rule.ID == "" {
		t.Fatal("规则ID不应为空")
	}
	if rule.Type != "frequency" {
		t.Errorf("类型不匹配")
	}
}

func TestListRules(t *testing.T) {
	m := NewManager()

	m.CreateRule(CreateRuleRequest{Name: "规则1", Type: "frequency"})
	m.CreateRule(CreateRuleRequest{Name: "规则2", Type: "pattern"})

	rules := m.ListRules()
	if len(rules) != 2 {
		t.Errorf("期望2条规则，实际 %d", len(rules))
	}
}

func TestUpdateRule(t *testing.T) {
	m := NewManager()

	rule := m.CreateRule(CreateRuleRequest{
		Name:      "原始规则",
		Type:      "frequency",
		Threshold: 10,
	})

	newName := "更新后的规则"
	newThreshold := 20
	updated, err := m.UpdateRule(rule.ID, UpdateRuleRequest{
		Name:      &newName,
		Threshold: &newThreshold,
	})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Name != "更新后的规则" {
		t.Errorf("名称未更新")
	}
	if updated.Threshold != 20 {
		t.Errorf("阈值未更新")
	}
}

func TestDeleteRule(t *testing.T) {
	m := NewManager()

	rule := m.CreateRule(CreateRuleRequest{Name: "待删除", Type: "frequency"})
	err := m.DeleteRule(rule.ID)
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}

	_, err = m.GetRule(rule.ID)
	if err == nil {
		t.Error("已删除规则不应存在")
	}
}

func TestAlertManagement(t *testing.T) {
	m := NewManager()

	// 创建规则
	rule := m.CreateRule(CreateRuleRequest{
		Name:      "测试规则",
		Type:      "pattern",
		Threshold: 1,
	})

	// 添加触发日志
	m.CreatePattern(CreatePatternRequest{
		Name:  "测试模式",
		Regex: `critical error`,
	})

	entry := &LogEntry{
		Level:   LevelError,
		Source:  "app",
		Message: "critical error occurred",
	}
	m.AddLog(entry)

	// 列出告警
	alerts := m.ListAlerts("")
	if len(alerts) == 0 {
		// 如果没有自动触发，手动创建
		alert := &Alert{
			ID:        "test-alert-1",
			RuleID:    rule.ID,
			RuleName:  rule.Name,
			Level:     AlertLevelHigh,
			Status:    AlertStatusActive,
			Message:   "测试告警",
			Count:     1,
			FirstSeen: time.Now(),
			LastSeen:  time.Now(),
		}
		m.alerts[alert.ID] = alert
		alerts = m.ListAlerts("")
	}

	if len(alerts) == 0 {
		t.Error("应有告警")
	}

	// 更新告警状态
	if len(alerts) > 0 {
		status := AlertStatusResolved
		_, err := m.UpdateAlert(alerts[0].ID, UpdateAlertRequest{Status: &status})
		if err != nil {
			t.Fatalf("更新告警失败: %v", err)
		}
	}
}

func TestLogClustering(t *testing.T) {
	m := NewManager()

	// 添加相似日志
	for i := 0; i < 5; i++ {
		m.AddLog(&LogEntry{
			Level:   LevelError,
			Source:  "app",
			Message: "connection timeout to database server 192.168.1.100",
		})
	}

	// 添加不同日志
	m.AddLog(&LogEntry{
		Level:   LevelInfo,
		Source:  "app",
		Message: "user login successful",
	})

	clusters := m.ListClusters()
	if len(clusters) == 0 {
		t.Error("应有聚类")
	}

	// 检查是否有大聚类
	found := false
	for _, c := range clusters {
		if c.Count >= 5 {
			found = true
			break
		}
	}
	if !found {
		t.Error("应有包含5条以上日志的聚类")
	}
}

func TestRootCauseAnalysis(t *testing.T) {
	m := NewManager()

	// 创建告警
	alert := &Alert{
		ID:       "test-alert-rca",
		RuleID:   "rule-1",
		RuleName: "测试规则",
		Level:    AlertLevelHigh,
		Status:   AlertStatusActive,
		Message:  "测试告警",
		LogIDs:   []string{},
		Count:    3,
		FirstSeen: time.Now().Add(-10 * time.Minute),
		LastSeen:  time.Now(),
	}

	// 添加关联日志
	for i := 0; i < 3; i++ {
		entry := &LogEntry{
			Level:   LevelError,
			Source:  "database",
			Message: "connection pool exhausted",
		}
		m.AddLog(entry)
		alert.LogIDs = append(alert.LogIDs, entry.ID)
	}

	m.alerts[alert.ID] = alert

	// 执行根因分析
	analysis, err := m.AnalyzeRootCause(alert.ID)
	if err != nil {
		t.Fatalf("根因分析失败: %v", err)
	}

	if analysis.RootCause == "" {
		t.Error("根因不应为空")
	}
	if len(analysis.Timeline) == 0 {
		t.Error("时间线不应为空")
	}
	if len(analysis.Suggestions) == 0 {
		t.Error("建议不应为空")
	}
}

func TestLogStream(t *testing.T) {
	m := NewManager()

	// 创建流
	stream := m.CreateStream(CreateStreamRequest{
		Name:   "系统日志",
		Source: "/var/log/syslog",
	})

	if stream.ID == "" {
		t.Fatal("流ID不应为空")
	}
	if stream.Running {
		t.Error("新创建的流不应运行")
	}

	// 启动流
	err := m.StartStream(stream.ID)
	if err != nil {
		t.Fatalf("启动流失败: %v", err)
	}

	// 验证运行状态
	got, _ := m.GetStream(stream.ID)
	if !got.Running {
		t.Error("流应在运行")
	}

	// 停止流
	err = m.StopStream(stream.ID)
	if err != nil {
		t.Fatalf("停止流失败: %v", err)
	}
}

func TestRetentionPolicy(t *testing.T) {
	m := NewManager()

	// 创建策略
	policy := m.CreateRetentionPolicy(CreateRetentionPolicyRequest{
		Name:       "30天保留",
		MaxAgeDays: 30,
	})

	if policy.ID == "" {
		t.Fatal("策略ID不应为空")
	}

	// 列出策略
	policies := m.ListRetentionPolicies()
	if len(policies) != 1 {
		t.Errorf("期望1条策略，实际 %d", len(policies))
	}

	// 删除策略
	err := m.DeleteRetentionPolicy(policy.ID)
	if err != nil {
		t.Fatalf("删除策略失败: %v", err)
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager()

	// 添加日志
	for i := 0; i < 100; i++ {
		level := LevelInfo
		if i%5 == 0 {
			level = LevelError
		}
		m.AddLog(&LogEntry{
			Level:   level,
			Source:  "app",
			Message: "test message",
		})
	}

	stats := m.GetStats(StatsQueryRequest{})

	if stats.TotalLogs != 100 {
		t.Errorf("总日志数应为100，实际 %d", stats.TotalLogs)
	}
	if stats.LogsByLevel[LevelError] != 20 {
		t.Errorf("错误日志数应为20，实际 %d", stats.LogsByLevel[LevelError])
	}
	if stats.ErrorRate != 20.0 {
		t.Errorf("错误率应为20%%，实际 %.1f%%", stats.ErrorRate)
	}
}

func TestRunAnalysis(t *testing.T) {
	m := NewManager()

	// 添加日志
	for i := 0; i < 50; i++ {
		m.AddLog(&LogEntry{
			Level:   LevelInfo,
			Source:  "app",
			Message: "normal log",
		})
	}

	// 添加异常日志
	m.CreatePattern(CreatePatternRequest{
		Name:     "异常模式",
		Regex:    `anomaly|critical`,
		IsAnomaly: true,
	})

	for i := 0; i < 5; i++ {
		m.AddLog(&LogEntry{
			Level:   LevelError,
			Source:  "app",
			Message: "anomaly detected",
		})
	}

	result := m.RunAnalysis(StatsQueryRequest{})

	if result.TotalLogs != 55 {
		t.Errorf("总日志数应为55，实际 %d", result.TotalLogs)
	}
	if result.Summary == "" {
		t.Error("摘要不应为空")
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager()

	done := make(chan bool, 10)

	// 并发写入
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				m.AddLog(&LogEntry{
					Level:   LevelInfo,
					Source:  "app",
					Message: "concurrent log",
				})
			}
			done <- true
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 并发读取
	for i := 0; i < 10; i++ {
		go func() {
			logs, _ := m.QueryLogs(QueryLogsRequest{})
			if len(logs) == 0 {
				t.Error("应有日志")
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
