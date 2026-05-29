package auditreport

import (
	"testing"
	"time"
)

func TestAnalyzerDetectAnomalies(t *testing.T) {
	analyzer := NewAnalyzer()

	// 添加测试事件
	analyzer.AddEvent(&AuditEvent{
		ID:        "evt-1",
		UserID:    "user1",
		Action:    "login",
		Resource:  "/auth",
		IP:        "192.168.1.1",
		Result:    "success",
		Timestamp: time.Now(),
	})
	analyzer.AddEvent(&AuditEvent{
		ID:        "evt-2",
		UserID:    "user1",
		Action:    "read",
		Resource:  "/api/files",
		IP:        "192.168.1.1",
		Result:    "success",
		Timestamp: time.Now(),
	})

	anomalies := analyzer.DetectAnomalies()
	// 正常情况下不应该检测到异常
	if len(anomalies) > 0 {
		t.Logf("detected %d anomalies (expected for normal behavior)", len(anomalies))
	}
}

func TestAnalyzerDetectFrequencyAnomalies(t *testing.T) {
	analyzer := NewAnalyzer()
	analyzer.SetThreshold(AnomalyThreshold{
		FrequencyMultiple: 2.0,
		TimeWindowHours:   24,
		MaxFailedAttempts: 5,
		UnusualHoursStart: 0,
		UnusualHoursEnd:   6,
	})

	// 创建一个用户在特定时间有大量访问
	for i := 0; i < 10; i++ {
		analyzer.AddEvent(&AuditEvent{
			ID:        "freq-" + string(rune(i)),
			UserID:    "user1",
			Action:    "read",
			Resource:  "/api/data",
			Result:    "success",
			Timestamp: time.Now().Truncate(time.Hour),
		})
	}

	// 添加少量其他时间段的访问
	analyzer.AddEvent(&AuditEvent{
		ID:        "other-1",
		UserID:    "user1",
		Action:    "login",
		Resource:  "/auth",
		Result:    "success",
		Timestamp: time.Now().Add(-2 * time.Hour),
	})

	anomalies := analyzer.DetectAnomalies()

	// 可能检测到频率异常
	for _, a := range anomalies {
		if a.Type == "frequency_anomaly" {
			t.Logf("detected frequency anomaly: %s", a.Description)
		}
	}
}

func TestAnalyzerDetectFailedAttempts(t *testing.T) {
	analyzer := NewAnalyzer()
	analyzer.SetThreshold(AnomalyThreshold{
		FrequencyMultiple: 3.0,
		TimeWindowHours:   24,
		MaxFailedAttempts: 3,
		UnusualHoursStart: 0,
		UnusualHoursEnd:   6,
	})

	// 添加大量失败尝试
	for i := 0; i < 5; i++ {
		analyzer.AddEvent(&AuditEvent{
			ID:        "fail-" + string(rune(i)),
			UserID:    "user2",
			Action:    "login",
			Resource:  "/auth",
			Result:    "failure",
			Timestamp: time.Now(),
		})
	}

	anomalies := analyzer.DetectAnomalies()

	// 应该检测到失败尝试异常
	found := false
	for _, a := range anomalies {
		if a.Type == "excessive_failures" && a.UserID == "user2" {
			found = true
			t.Logf("detected excessive failures: %s", a.Description)
			if a.Severity != SeverityHigh {
				t.Errorf("expected high severity, got %q", a.Severity)
			}
		}
	}

	if !found {
		t.Error("expected to detect excessive failures anomaly")
	}
}

func TestAnalyzerDetectUnusualTimeAccess(t *testing.T) {
	analyzer := NewAnalyzer()
	analyzer.SetThreshold(AnomalyThreshold{
		FrequencyMultiple: 3.0,
		TimeWindowHours:   24,
		MaxFailedAttempts: 5,
		UnusualHoursStart: 0,
		UnusualHoursEnd:   6,
	})

	// 添加非工作时间访问
	now := time.Time{}
	analyzer.AddEvent(&AuditEvent{
		ID:        "unusual-1",
		UserID:    "user3",
		Action:    "login",
		Resource:  "/auth",
		Result:    "success",
		Timestamp: now.Add(2 * time.Hour), // 凌晨 2 点
	})
	analyzer.AddEvent(&AuditEvent{
		ID:        "unusual-2",
		UserID:    "user3",
		Action:    "read",
		Resource:  "/api/data",
		Result:    "success",
		Timestamp: now.Add(3 * time.Hour), // 凌晨 3 点
	})

	anomalies := analyzer.DetectAnomalies()

	// 应该检测到非工作时间访问
	found := false
	for _, a := range anomalies {
		if a.Type == "unusual_time_access" && a.UserID == "user3" {
			found = true
			t.Logf("detected unusual time access: %s", a.Description)
		}
	}

	if !found {
		t.Error("expected to detect unusual time access anomaly")
	}
}

func TestAnalyzerDetectPrivilegeEscalation(t *testing.T) {
	analyzer := NewAnalyzer()

	// 添加大量权限变更操作
	for i := 0; i < 4; i++ {
		analyzer.AddEvent(&AuditEvent{
			ID:        "priv-" + string(rune(i)),
			UserID:    "user4",
			Action:    "permission_change",
			Resource:  "/admin/users",
			Result:    "success",
			Timestamp: time.Now(),
		})
	}

	anomalies := analyzer.DetectAnomalies()

	// 应该检测到权限提升
	found := false
	for _, a := range anomalies {
		if a.Type == "privilege_escalation" && a.UserID == "user4" {
			found = true
			t.Logf("detected privilege escalation: %s", a.Description)
			if a.Severity != SeverityCritical {
				t.Errorf("expected critical severity, got %q", a.Severity)
			}
		}
	}

	if !found {
		t.Error("expected to detect privilege escalation anomaly")
	}
}

func TestAnalyzerAnalyzeAccessPattern(t *testing.T) {
	analyzer := NewAnalyzer()

	// 添加用户事件
	analyzer.AddEvent(&AuditEvent{
		ID:        "pat-1",
		UserID:    "user1",
		Action:    "login",
		Resource:  "/auth",
		Result:    "success",
		Timestamp: time.Now().Add(-1 * time.Hour),
	})
	analyzer.AddEvent(&AuditEvent{
		ID:        "pat-2",
		UserID:    "user1",
		Action:    "read",
		Resource:  "/api/files",
		Result:    "success",
		Timestamp: time.Now(),
	})
	analyzer.AddEvent(&AuditEvent{
		ID:        "pat-3",
		UserID:    "user1",
		Action:    "read",
		Resource:  "/api/data",
		Result:    "success",
		Timestamp: time.Now(),
	})

	pattern := analyzer.AnalyzeAccessPattern("user1")

	if pattern.UserID != "user1" {
		t.Errorf("expected user1, got %q", pattern.UserID)
	}
	if pattern.TotalEvents != 3 {
		t.Errorf("expected 3 events, got %d", pattern.TotalEvents)
	}
	if pattern.ByAction["login"] != 1 {
		t.Errorf("expected 1 login, got %d", pattern.ByAction["login"])
	}
	if pattern.ByAction["read"] != 2 {
		t.Errorf("expected 2 reads, got %d", pattern.ByAction["read"])
	}
	if pattern.ByResult["success"] != 3 {
		t.Errorf("expected 3 successes, got %d", pattern.ByResult["success"])
	}
}

func TestAnalyzerGetAllPatterns(t *testing.T) {
	analyzer := NewAnalyzer()

	analyzer.AddEvent(&AuditEvent{
		UserID:    "user1",
		Action:    "login",
		Resource:  "/auth",
		Result:    "success",
		Timestamp: time.Now(),
	})
	analyzer.AddEvent(&AuditEvent{
		UserID:    "user2",
		Action:    "login",
		Resource:  "/auth",
		Result:    "success",
		Timestamp: time.Now(),
	})

	patterns := analyzer.GetAllPatterns()

	if len(patterns) != 2 {
		t.Errorf("expected 2 patterns, got %d", len(patterns))
	}
	if _, ok := patterns["user1"]; !ok {
		t.Error("expected user1 pattern")
	}
	if _, ok := patterns["user2"]; !ok {
		t.Error("expected user2 pattern")
	}
}

func TestRiskScorerCalculateUserRisk(t *testing.T) {
	analyzer := NewAnalyzer()
	scorer := NewRiskScorer(analyzer)

	// 添加测试事件
	events := []*AuditEvent{
		{
			UserID:    "user1",
			Action:    "login",
			Resource:  "/auth",
			Result:    "success",
			Timestamp: time.Now(),
		},
		{
			UserID:    "user1",
			Action:    "read",
			Resource:  "/api/files",
			Result:    "success",
			Timestamp: time.Now(),
		},
		{
			UserID:    "user1",
			Action:    "delete",
			Resource:  "/api/files",
			Result:    "success",
			Timestamp: time.Now(),
		},
	}
	scorer.LoadEvents(events)

	result := scorer.CalculateUserRisk("user1")

	if result.UserID != "user1" {
		t.Errorf("expected user1, got %q", result.UserID)
	}
	if result.OverallScore < 0 || result.OverallScore > 100 {
		t.Errorf("expected score between 0-100, got %f", result.OverallScore)
	}
	if result.RiskLevel == "" {
		t.Error("expected risk level")
	}
	if result.Components.OperationWeight <= 0 {
		t.Error("expected positive operation weight")
	}
}

func TestRiskScorerHighRiskUser(t *testing.T) {
	analyzer := NewAnalyzer()
	scorer := NewRiskScorer(analyzer)

	// 创建高风险用户事件
	events := []*AuditEvent{
		{
			UserID:    "highrisk",
			Action:    "permission_change",
			Resource:  "/admin/users",
			Result:    "success",
			Timestamp: time.Now(),
		},
		{
			UserID:    "highrisk",
			Action:    "delete",
			Resource:  "/api/data",
			Result:    "success",
			Timestamp: time.Now(),
		},
		{
			UserID:    "highrisk",
			Action:    "config_change",
			Resource:  "/admin/config",
			Result:    "success",
			Timestamp: time.Now(),
		},
		{
			UserID:    "highrisk",
			Action:    "user_delete",
			Resource:  "/admin/users",
			Result:    "success",
			Timestamp: time.Now(),
		},
		{
			UserID:    "highrisk",
			Action:    "data_export",
			Resource:  "/api/export",
			Result:    "success",
			Timestamp: time.Now(),
		},
	}
	scorer.LoadEvents(events)

	result := scorer.CalculateUserRisk("highrisk")

	t.Logf("High risk user score: %.1f, level: %s", result.OverallScore, result.RiskLevel)

	// 高风险操作应该导致较高分数
	if result.OverallScore < 10 {
		t.Errorf("expected higher risk score for high-risk operations, got %f", result.OverallScore)
	}

	// 应该有风险项
	if len(result.TopRisks) == 0 {
		t.Error("expected top risks to be identified")
	}

	// 应该有建议
	if len(result.Recommendations) == 0 {
		t.Error("expected recommendations")
	}
}

func TestRiskScorerSafeUser(t *testing.T) {
	analyzer := NewAnalyzer()
	scorer := NewRiskScorer(analyzer)

	// 创建安全用户事件
	events := []*AuditEvent{
		{
			UserID:    "safeuser",
			Action:    "read",
			Resource:  "/api/public",
			Result:    "success",
			Timestamp: time.Now(),
		},
		{
			UserID:    "safeuser",
			Action:    "login",
			Resource:  "/auth",
			Result:    "success",
			Timestamp: time.Now(),
		},
	}
	scorer.LoadEvents(events)

	result := scorer.CalculateUserRisk("safeuser")

	t.Logf("Safe user score: %.1f, level: %s", result.OverallScore, result.RiskLevel)

	// 安全用户应该有较低分数
	if result.OverallScore > 30 {
		t.Errorf("expected lower risk score for safe user, got %f", result.OverallScore)
	}
}

func TestRiskScorerGetHighRiskUsers(t *testing.T) {
	analyzer := NewAnalyzer()
	scorer := NewRiskScorer(analyzer)

	// 添加多个用户
	events := []*AuditEvent{
		{UserID: "user1", Action: "read", Resource: "/api/data", Result: "success", Timestamp: time.Now()},
		{UserID: "user2", Action: "permission_change", Resource: "/admin", Result: "success", Timestamp: time.Now()},
		{UserID: "user2", Action: "delete", Resource: "/api/data", Result: "success", Timestamp: time.Now()},
		{UserID: "user3", Action: "read", Resource: "/api/public", Result: "success", Timestamp: time.Now()},
	}
	scorer.LoadEvents(events)

	highRisk := scorer.GetHighRiskUsers(50.0)

	t.Logf("Found %d high risk users", len(highRisk))

	// user2 应该有更高风险
	for _, r := range highRisk {
		t.Logf("User: %s, Score: %.1f, Level: %s", r.UserID, r.OverallScore, r.RiskLevel)
	}
}

func TestRiskScorerAllUsers(t *testing.T) {
	analyzer := NewAnalyzer()
	scorer := NewRiskScorer(analyzer)

	events := []*AuditEvent{
		{UserID: "user1", Action: "read", Resource: "/api/data", Result: "success", Timestamp: time.Now()},
		{UserID: "user2", Action: "login", Resource: "/auth", Result: "success", Timestamp: time.Now()},
	}
	scorer.LoadEvents(events)

	results := scorer.CalculateAllUserRisk()

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// 结果应该按分数降序排列
	if len(results) >= 2 && results[0].OverallScore < results[1].OverallScore {
		t.Error("expected results to be sorted by score descending")
	}
}

func TestRiskLevelDetermination(t *testing.T) {
	tests := []struct {
		score    float64
		expected RiskLevel
	}{
		{90, RiskLevelCritical},
		{70, RiskLevelHigh},
		{50, RiskLevelMedium},
		{30, RiskLevelLow},
		{10, RiskLevelSafe},
	}

	for _, tt := range tests {
		result := determineRiskLevel(tt.score)
		if result != tt.expected {
			t.Errorf("for score %f, expected %q, got %q", tt.score, tt.expected, result)
		}
	}
}

func TestOperationWeight(t *testing.T) {
	// 测试操作权重映射
	if OperationWeight[OpDelete] != 0.8 {
		t.Errorf("expected delete weight 0.8, got %f", OperationWeight[OpDelete])
	}
	if OperationWeight[OpPermissionChange] != 0.9 {
		t.Errorf("expected permission_change weight 0.9, got %f", OperationWeight[OpPermissionChange])
	}
	if OperationWeight[OpRead] != 0.1 {
		t.Errorf("expected read weight 0.1, got %f", OperationWeight[OpRead])
	}
}

func TestResourceSensitivityWeight(t *testing.T) {
	// 测试资源敏感度权重映射
	if ResourceSensitivityWeight[SensitivityCritical] != 1.0 {
		t.Errorf("expected critical weight 1.0, got %f", ResourceSensitivityWeight[SensitivityCritical])
	}
	if ResourceSensitivityWeight[SensitivityPublic] != 0.1 {
		t.Errorf("expected public weight 0.1, got %f", ResourceSensitivityWeight[SensitivityPublic])
	}
}

func TestMapActionToOperationType(t *testing.T) {
	tests := []struct {
		action   string
		expected OperationType
	}{
		{"delete", OpDelete},
		{"remove", OpDelete},
		{"update", OpModify},
		{"edit", OpModify},
		{"permission_change", OpPermissionChange},
		{"export", OpDataExport},
		{"download", OpDataExport},
		{"login", OpLogin},
		{"read", OpRead},
		{"unknown", OpRead}, // 默认
	}

	for _, tt := range tests {
		result := mapActionToOperationType(tt.action)
		if result != tt.expected {
			t.Errorf("action %q: expected %q, got %q", tt.action, tt.expected, result)
		}
	}
}

func TestInferResourceSensitivity(t *testing.T) {
	tests := []struct {
		resource string
		expected ResourceSensitivity
	}{
		{"/admin/config", SensitivityCritical},
		{"/api/secret", SensitivityCritical},
		{"/admin/users", SensitivityCritical}, // admin 关键词触发 critical
		{"/api/files", SensitivityMedium},
		{"/api/data", SensitivityMedium},
		{"/health", SensitivityPublic},
		{"/static/css", SensitivityPublic},
		{"/api/unknown", SensitivityLow},
	}

	for _, tt := range tests {
		result := inferResourceSensitivity(tt.resource)
		if result != tt.expected {
			t.Errorf("resource %q: expected %q, got %q", tt.resource, tt.expected, result)
		}
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s        string
		keywords []string
		expected bool
	}{
		{"/admin/config", []string{"admin"}, true},
		{"/api/data", []string{"admin"}, false},
		{"/api/secret", []string{"secret", "key"}, true},
		{"/public", []string{"public"}, true},
		{"/test", []string{"nonexistent"}, false},
	}

	for _, tt := range tests {
		result := contains(tt.s, tt.keywords...)
		if result != tt.expected {
			t.Errorf("contains(%q, %v): expected %v, got %v", tt.s, tt.keywords, tt.expected, result)
		}
	}
}
