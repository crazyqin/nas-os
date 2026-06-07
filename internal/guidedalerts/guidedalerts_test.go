package guidedalerts

import (
	"testing"
	"time"
)

// 类型测试

func TestAlertSeverity(t *testing.T) {
	tests := []struct {
		severity AlertSeverity
		str      string
	}{
		{SeverityInfo, "info"},
		{SeverityWarning, "warning"},
		{SeverityCritical, "critical"},
		{AlertSeverity(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.severity.String(); got != tt.str {
			t.Errorf("AlertSeverity(%d).String() = %s, want %s", tt.severity, got, tt.str)
		}
	}
}

func TestParseSeverity(t *testing.T) {
	tests := []struct {
		input string
		want  AlertSeverity
	}{
		{"info", SeverityInfo},
		{"warning", SeverityWarning},
		{"critical", SeverityCritical},
		{"invalid", SeverityInfo},
	}

	for _, tt := range tests {
		if got := ParseSeverity(tt.input); got != tt.want {
			t.Errorf("ParseSeverity(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// 规则引擎测试

func TestNewRuleEngine(t *testing.T) {
	engine := NewRuleEngine()
	if engine == nil {
		t.Fatal("NewRuleEngine returned nil")
	}
	if len(engine.rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(engine.rules))
	}
}

func TestRuleEngineAddRule(t *testing.T) {
	engine := NewRuleEngine()

	rule := &AlertRule{
		ID:       "test-rule",
		Name:     "Test Rule",
		Enabled:  true,
		Severity: SeverityWarning,
		Category: CategoryCPU,
		Condition: RuleCondition{
			Type:      ConditionThreshold,
			Metric:    "cpu_usage",
			Operator:  "gt",
			Threshold: 80,
		},
	}

	err := engine.AddRule(rule)
	if err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	// 重复添加应失败
	err = engine.AddRule(rule)
	if err == nil {
		t.Error("expected error when adding duplicate rule")
	}

	// 空ID应失败
	err = engine.AddRule(&AlertRule{})
	if err == nil {
		t.Error("expected error when adding rule with empty ID")
	}
}

func TestRuleEngineGetRule(t *testing.T) {
	engine := NewRuleEngine()
	engine.AddRule(&AlertRule{ID: "r1", Name: "Rule 1"})

	rule, ok := engine.GetRule("r1")
	if !ok {
		t.Fatal("GetRule returned false")
	}
	if rule.Name != "Rule 1" {
		t.Errorf("expected name 'Rule 1', got '%s'", rule.Name)
	}

	_, ok = engine.GetRule("nonexistent")
	if ok {
		t.Error("expected GetRule to return false for nonexistent rule")
	}
}

func TestRuleEngineDeleteRule(t *testing.T) {
	engine := NewRuleEngine()
	engine.AddRule(&AlertRule{ID: "r1"})

	err := engine.DeleteRule("r1")
	if err != nil {
		t.Fatalf("DeleteRule failed: %v", err)
	}

	_, ok := engine.GetRule("r1")
	if ok {
		t.Error("expected rule to be deleted")
	}

	err = engine.DeleteRule("nonexistent")
	if err == nil {
		t.Error("expected error when deleting nonexistent rule")
	}
}

func TestRuleEngineListRules(t *testing.T) {
	engine := NewRuleEngine()
	engine.AddRule(&AlertRule{ID: "r1", Category: CategoryCPU, Enabled: true})
	engine.AddRule(&AlertRule{ID: "r2", Category: CategoryMemory, Enabled: true})
	engine.AddRule(&AlertRule{ID: "r3", Category: CategoryCPU, Enabled: false})

	// 列出所有
	all := engine.ListRules("", false)
	if len(all) != 3 {
		t.Errorf("expected 3 rules, got %d", len(all))
	}

	// 按分类
	cpuRules := engine.ListRules(CategoryCPU, false)
	if len(cpuRules) != 2 {
		t.Errorf("expected 2 CPU rules, got %d", len(cpuRules))
	}

	// 只看启用的
	enabled := engine.ListRules("", true)
	if len(enabled) != 2 {
		t.Errorf("expected 2 enabled rules, got %d", len(enabled))
	}
}

func TestRuleEngineEvaluate(t *testing.T) {
	engine := NewRuleEngine()
	engine.AddRule(&AlertRule{
		ID:       "high-cpu",
		Enabled:  true,
		Severity: SeverityWarning,
		Category: CategoryCPU,
		Condition: RuleCondition{
			Type:      ConditionThreshold,
			Metric:    "cpu_usage",
			Operator:  "gt",
			Threshold: 80,
		},
	})

	// CPU > 80 应触发
	metrics := map[string]float64{"cpu_usage": 85}
	result, err := engine.Evaluate("high-cpu", metrics)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !result {
		t.Error("expected rule to trigger for cpu_usage=85")
	}

	// CPU <= 80 不应触发
	metrics = map[string]float64{"cpu_usage": 70}
	result, _ = engine.Evaluate("high-cpu", metrics)
	if result {
		t.Error("expected rule not to trigger for cpu_usage=70")
	}
}

func TestRuleEngineEvaluateDisabled(t *testing.T) {
	engine := NewRuleEngine()
	engine.AddRule(&AlertRule{
		ID:      "disabled-rule",
		Enabled: false,
		Condition: RuleCondition{
			Type:      ConditionThreshold,
			Metric:    "test",
			Operator:  "gt",
			Threshold: 0,
		},
	})

	result, _ := engine.Evaluate("disabled-rule", map[string]float64{"test": 100})
	if result {
		t.Error("disabled rule should not trigger")
	}
}

func TestRuleEngineEvaluateAll(t *testing.T) {
	engine := NewRuleEngine()
	engine.AddRule(&AlertRule{
		ID: "r1", Enabled: true,
		Condition: RuleCondition{Type: ConditionThreshold, Metric: "a", Operator: "gt", Threshold: 10},
	})
	engine.AddRule(&AlertRule{
		ID: "r2", Enabled: true,
		Condition: RuleCondition{Type: ConditionThreshold, Metric: "b", Operator: "gt", Threshold: 20},
	})
	engine.AddRule(&AlertRule{
		ID: "r3", Enabled: false,
		Condition: RuleCondition{Type: ConditionThreshold, Metric: "a", Operator: "gt", Threshold: 5},
	})

	results := engine.EvaluateAll(map[string]float64{"a": 15, "b": 10})
	if !results["r1"] {
		t.Error("expected r1 to trigger")
	}
	if results["r2"] {
		t.Error("expected r2 not to trigger")
	}
	if results["r3"] {
		t.Error("disabled r3 should not trigger")
	}
}

func TestEvalThreshold(t *testing.T) {
	tests := []struct {
		op        string
		threshold float64
		value     float64
		expected  bool
	}{
		{"gt", 80, 85, true},
		{"gt", 80, 75, false},
		{"lt", 80, 75, true},
		{"lt", 80, 85, false},
		{"gte", 80, 80, true},
		{"gte", 80, 79, false},
		{"lte", 80, 80, true},
		{"lte", 80, 81, false},
		{"eq", 80, 80, true},
		{"eq", 80, 81, false},
		{"ne", 80, 81, true},
		{"ne", 80, 80, false},
	}

	for _, tt := range tests {
		cond := &RuleCondition{
			Type:      ConditionThreshold,
			Metric:    "test",
			Operator:  tt.op,
			Threshold: tt.threshold,
		}
		metrics := map[string]float64{"test": tt.value}
		result := evalThreshold(cond, metrics)
		if result != tt.expected {
			t.Errorf("evalThreshold(op=%s, threshold=%f, value=%f) = %v, want %v",
				tt.op, tt.threshold, tt.value, result, tt.expected)
		}
	}
}

func TestBuiltinRules(t *testing.T) {
	rules := GetBuiltinRules()
	if len(rules) == 0 {
		t.Fatal("expected builtin rules to be non-empty")
	}

	// 检查规则是否有效
	for _, rule := range rules {
		if rule.ID == "" {
			t.Error("builtin rule has empty ID")
		}
		if rule.Name == "" {
			t.Errorf("builtin rule %s has empty name", rule.ID)
		}
		if rule.Condition.Type == "" {
			t.Errorf("builtin rule %s has empty condition type", rule.ID)
		}
	}
}

// 路由器测试

func TestNewAlertRouter(t *testing.T) {
	router := NewAlertRouter()
	if router == nil {
		t.Fatal("NewAlertRouter returned nil")
	}
}

func TestRouterAddChannel(t *testing.T) {
	router := NewAlertRouter()

	ch := &RouteChannel{
		ID:      "test-channel",
		Name:    "Test Channel",
		Type:    "webhook",
		Enabled: true,
	}

	err := router.AddChannel(ch)
	if err != nil {
		t.Fatalf("AddChannel failed: %v", err)
	}

	// 重复添加应失败
	err = router.AddChannel(ch)
	if err == nil {
		t.Error("expected error when adding duplicate channel")
	}

	// 空ID应失败
	err = router.AddChannel(&RouteChannel{})
	if err == nil {
		t.Error("expected error when adding channel with empty ID")
	}
}

func TestRouterRemoveChannel(t *testing.T) {
	router := NewAlertRouter()
	router.AddChannel(&RouteChannel{ID: "ch1"})

	err := router.RemoveChannel("ch1")
	if err != nil {
		t.Fatalf("RemoveChannel failed: %v", err)
	}

	_, ok := router.GetChannel("ch1")
	if ok {
		t.Error("expected channel to be removed")
	}

	err = router.RemoveChannel("nonexistent")
	if err == nil {
		t.Error("expected error when removing nonexistent channel")
	}
}

func TestRouterAddRouteRule(t *testing.T) {
	router := NewAlertRouter()
	router.AddChannel(&RouteChannel{ID: "ch1", Enabled: true})

	rule := &RouteRule{
		ID:       "rule1",
		Name:     "Test Rule",
		Channels: []string{"ch1"},
		Enabled:  true,
	}

	err := router.AddRouteRule(rule)
	if err != nil {
		t.Fatalf("AddRouteRule failed: %v", err)
	}

	// 引用不存在的通道应失败
	err = router.AddRouteRule(&RouteRule{
		ID:       "rule2",
		Channels: []string{"nonexistent"},
	})
	if err == nil {
		t.Error("expected error when adding rule with nonexistent channel")
	}
}

func TestRouterRoute(t *testing.T) {
	router := NewAlertRouter()
	router.AddChannel(&RouteChannel{ID: "ch1", Name: "Channel 1", Enabled: true})
	router.AddChannel(&RouteChannel{ID: "ch2", Name: "Channel 2", Enabled: true})

	// 添加匹配 critical 的规则
	router.AddRouteRule(&RouteRule{
		ID:       "critical-rule",
		Matchers: []LabelMatcher{{Name: "severity", Value: "critical", IsEqual: true}},
		Channels: []string{"ch1"},
		Continue: false,
		Enabled:  true,
	})

	// 添加默认规则
	router.AddRouteRule(&RouteRule{
		ID:       "default-rule",
		Matchers: []LabelMatcher{},
		Channels: []string{"ch2"},
		Enabled:  true,
	})

	// 测试 critical 告警
	criticalAlert := &Alert{ID: "a1", Severity: SeverityCritical}
	channels := router.Route(criticalAlert)
	if len(channels) != 1 {
		t.Errorf("expected 1 channel for critical alert, got %d", len(channels))
	}
	if channels[0].ID != "ch1" {
		t.Errorf("expected channel ch1, got %s", channels[0].ID)
	}

	// 测试 warning 告警（应走默认规则）
	warningAlert := &Alert{ID: "a2", Severity: SeverityWarning}
	channels = router.Route(warningAlert)
	if len(channels) != 1 {
		t.Errorf("expected 1 channel for warning alert, got %d", len(channels))
	}
	if channels[0].ID != "ch2" {
		t.Errorf("expected channel ch2, got %s", channels[0].ID)
	}
}

// 告警管理器测试

func TestNewAlertManager(t *testing.T) {
	am := NewAlertManager()
	if am == nil {
		t.Fatal("NewAlertManager returned nil")
	}
	if len(am.alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(am.alerts))
	}
	am.Stop()
}

func TestCreateAlert(t *testing.T) {
	am := NewAlertManager()
	defer am.Stop()

	alert := &Alert{
		ID:       "alert-1",
		Title:    "磁盘空间不足",
		Severity: SeverityWarning,
		Category: CategoryDisk,
	}

	err := am.CreateAlert(alert)
	if err != nil {
		t.Fatalf("CreateAlert failed: %v", err)
	}

	got, ok := am.GetAlert("alert-1")
	if !ok {
		t.Fatal("GetAlert returned false")
	}
	if got.Title != "磁盘空间不足" {
		t.Errorf("expected title '磁盘空间不足', got '%s'", got.Title)
	}
	if got.Count != 1 {
		t.Errorf("expected count 1, got %d", got.Count)
	}
	if got.Status != StatusActive {
		t.Errorf("expected status active, got %s", got.Status)
	}
}

func TestCreateAlertAggregation(t *testing.T) {
	am := NewAlertManager()
	defer am.Stop()

	am.CreateAlert(&Alert{ID: "a1", Title: "test", Severity: SeverityWarning})
	am.CreateAlert(&Alert{ID: "a1", Title: "test", Severity: SeverityWarning})

	alert, _ := am.GetAlert("a1")
	if alert.Count != 2 {
		t.Errorf("expected count 2, got %d", alert.Count)
	}
}

func TestAcknowledgeAlert(t *testing.T) {
	am := NewAlertManager()
	defer am.Stop()

	am.CreateAlert(&Alert{ID: "a1", Title: "test", Severity: SeverityInfo})

	err := am.AcknowledgeAlert("a1", "admin")
	if err != nil {
		t.Fatalf("AcknowledgeAlert failed: %v", err)
	}

	alert, _ := am.GetAlert("a1")
	if alert.Status != StatusAcknowledged {
		t.Errorf("expected status acknowledged, got %s", alert.Status)
	}
	if alert.AckedAt == nil {
		t.Error("expected AckedAt to be set")
	}

	// 不存在的告警
	err = am.AcknowledgeAlert("nonexistent", "admin")
	if err == nil {
		t.Error("expected error for nonexistent alert")
	}
}

func TestResolveAlert(t *testing.T) {
	am := NewAlertManager()
	defer am.Stop()

	am.CreateAlert(&Alert{ID: "a1", Title: "test", Severity: SeverityInfo})

	err := am.ResolveAlert("a1", "admin")
	if err != nil {
		t.Fatalf("ResolveAlert failed: %v", err)
	}

	alert, _ := am.GetAlert("a1")
	if alert.Status != StatusResolved {
		t.Errorf("expected status resolved, got %s", alert.Status)
	}
	if alert.ResolvedAt == nil {
		t.Error("expected ResolvedAt to be set")
	}
}

func TestListAlerts(t *testing.T) {
	am := NewAlertManager()
	defer am.Stop()

	am.CreateAlert(&Alert{ID: "a1", Title: "t1", Severity: SeverityCritical, Category: CategoryDisk})
	am.CreateAlert(&Alert{ID: "a2", Title: "t2", Severity: SeverityWarning, Category: CategoryNetwork})
	am.CreateAlert(&Alert{ID: "a3", Title: "t3", Severity: SeverityCritical, Category: CategoryCPU})
	am.ResolveAlert("a3", "")

	// 列出所有
	all := am.ListAlerts(AlertFilter{Severity: -1})
	if len(all) != 3 {
		t.Errorf("expected 3 alerts, got %d", len(all))
	}

	// 只看 critical
	critical := am.ListAlerts(AlertFilter{Severity: SeverityCritical})
	if len(critical) != 2 {
		t.Errorf("expected 2 critical alerts, got %d", len(critical))
	}

	// 只看未解决
	unresolved := am.ListAlerts(AlertFilter{Severity: -1, UnresolvedOnly: true})
	if len(unresolved) != 2 {
		t.Errorf("expected 2 unresolved alerts, got %d", len(unresolved))
	}

	// 按分类
	diskAlerts := am.ListAlerts(AlertFilter{Severity: -1, Category: CategoryDisk})
	if len(diskAlerts) != 1 {
		t.Errorf("expected 1 disk alert, got %d", len(diskAlerts))
	}
}

func TestGetStats(t *testing.T) {
	am := NewAlertManager()
	defer am.Stop()

	am.CreateAlert(&Alert{ID: "a1", Severity: SeverityCritical, Category: CategoryDisk})
	am.CreateAlert(&Alert{ID: "a2", Severity: SeverityWarning, Category: CategoryNetwork})
	am.CreateAlert(&Alert{ID: "a3", Severity: SeverityCritical, Category: CategoryCPU})
	am.ResolveAlert("a3", "")

	stats := am.GetStats()
	if stats.Total != 3 {
		t.Errorf("expected total 3, got %d", stats.Total)
	}
	if stats.BySeverity["critical"] != 2 {
		t.Errorf("expected critical 2, got %d", stats.BySeverity["critical"])
	}
	if stats.ResolvedCount != 1 {
		t.Errorf("expected resolved 1, got %d", stats.ResolvedCount)
	}
	if stats.ActiveCount != 2 {
		t.Errorf("expected active 2, got %d", stats.ActiveCount)
	}
}

func TestGetAlertHistory(t *testing.T) {
	am := NewAlertManager()
	defer am.Stop()

	am.CreateAlert(&Alert{ID: "a1", Title: "test", Severity: SeverityInfo})
	am.AcknowledgeAlert("a1", "admin")
	am.ResolveAlert("a1", "admin")

	history := am.GetAlertHistory("a1")
	if len(history) < 3 {
		t.Errorf("expected at least 3 history entries, got %d", len(history))
	}

	// 检查历史顺序（只验证关键动作的相对顺序）
	createdIdx := -1
	ackedIdx := -1
	resolvedIdx := -1
	for i, h := range history {
		switch h.Action {
		case "created":
			createdIdx = i
		case "acknowledged":
			ackedIdx = i
		case "resolved":
			resolvedIdx = i
		}
	}
	if createdIdx >= ackedIdx || ackedIdx >= resolvedIdx {
		t.Errorf("expected order: created < acknowledged < resolved, got indices %d, %d, %d",
			createdIdx, ackedIdx, resolvedIdx)
	}
}

// 静默规则测试

func TestSilenceRule(t *testing.T) {
	am := NewAlertManager()
	defer am.Stop()

	// 添加静默规则
	rule := &SilenceRule{
		ID:   "silence-1",
		Name: "Silence disk alerts",
		Matchers: []LabelMatcher{
			{Name: "category", Value: "disk", IsEqual: true},
		},
		StartsAt: time.Now().Add(-1 * time.Hour),
		EndsAt:   time.Now().Add(1 * time.Hour),
		Enabled:  true,
	}

	err := am.AddSilenceRule(rule)
	if err != nil {
		t.Fatalf("AddSilenceRule failed: %v", err)
	}

	// 创建匹配的告警应被静默
	am.CreateAlert(&Alert{ID: "a1", Title: "disk alert", Category: CategoryDisk, Severity: SeverityWarning})
	alert, _ := am.GetAlert("a1")
	if alert.Status != StatusSilenced {
		t.Errorf("expected status silenced, got %s", alert.Status)
	}

	// 创建不匹配的告警不应被静默
	am.CreateAlert(&Alert{ID: "a2", Title: "cpu alert", Category: CategoryCPU, Severity: SeverityWarning})
	alert, _ = am.GetAlert("a2")
	if alert.Status != StatusActive {
		t.Errorf("expected status active, got %s", alert.Status)
	}
}

func TestSilenceRuleManagement(t *testing.T) {
	am := NewAlertManager()
	defer am.Stop()

	rule := &SilenceRule{
		ID:       "s1",
		Name:     "Test",
		Enabled:  true,
		StartsAt: time.Now().Add(-1 * time.Hour),
		EndsAt:   time.Now().Add(1 * time.Hour),
	}

	am.AddSilenceRule(rule)

	rules := am.ListSilenceRules()
	if len(rules) != 1 {
		t.Errorf("expected 1 silence rule, got %d", len(rules))
	}

	err := am.RemoveSilenceRule("s1")
	if err != nil {
		t.Fatalf("RemoveSilenceRule failed: %v", err)
	}

	rules = am.ListSilenceRules()
	if len(rules) != 0 {
		t.Errorf("expected 0 silence rules after removal, got %d", len(rules))
	}
}

// 抑制规则测试

func TestInhibitionRule(t *testing.T) {
	am := NewAlertManager()
	defer am.Stop()

	// 添加抑制规则：当有 critical 的 disk 告警时，抑制 warning 的 disk 告警
	rule := &InhibitionRule{
		ID:   "inhibit-1",
		Name: "Inhibit lower severity",
		SourceMatchers: []LabelMatcher{
			{Name: "severity", Value: "critical", IsEqual: true},
			{Name: "category", Value: "disk", IsEqual: true},
		},
		TargetMatchers: []LabelMatcher{
			{Name: "severity", Value: "warning", IsEqual: true},
			{Name: "category", Value: "disk", IsEqual: true},
		},
		Equal:   []string{"category"},
		Enabled: true,
	}

	am.AddInhibitionRule(rule)

	// 先创建 critical 告警
	am.CreateAlert(&Alert{ID: "a1", Title: "critical disk", Category: CategoryDisk, Severity: SeverityCritical})

	// 创建 warning disk 告警应被抑制
	am.CreateAlert(&Alert{ID: "a2", Title: "warning disk", Category: CategoryDisk, Severity: SeverityWarning})
	alert, _ := am.GetAlert("a2")
	if alert.Status != StatusSilenced {
		t.Errorf("expected status silenced, got %s", alert.Status)
	}

	// 创建 warning cpu 告警不应被抑制
	am.CreateAlert(&Alert{ID: "a3", Title: "warning cpu", Category: CategoryCPU, Severity: SeverityWarning})
	alert, _ = am.GetAlert("a3")
	if alert.Status != StatusActive {
		t.Errorf("expected status active, got %s", alert.Status)
	}
}

func TestInhibitionRuleManagement(t *testing.T) {
	am := NewAlertManager()
	defer am.Stop()

	rule := &InhibitionRule{
		ID:      "i1",
		Name:    "Test",
		Enabled: true,
	}

	am.AddInhibitionRule(rule)

	rules := am.ListInhibitionRules()
	if len(rules) != 1 {
		t.Errorf("expected 1 inhibition rule, got %d", len(rules))
	}

	err := am.RemoveInhibitionRule("i1")
	if err != nil {
		t.Fatalf("RemoveInhibitionRule failed: %v", err)
	}

	rules = am.ListInhibitionRules()
	if len(rules) != 0 {
		t.Errorf("expected 0 inhibition rules after removal, got %d", len(rules))
	}
}

// 内置路由配置测试

func TestGetDefaultChannels(t *testing.T) {
	channels := GetDefaultChannels()
	if len(channels) == 0 {
		t.Fatal("expected default channels to be non-empty")
	}

	for _, ch := range channels {
		if ch.ID == "" {
			t.Error("default channel has empty ID")
		}
		if ch.Name == "" {
			t.Errorf("default channel %s has empty name", ch.ID)
		}
	}
}

func TestGetDefaultRouteRules(t *testing.T) {
	rules := GetDefaultRouteRules()
	if len(rules) == 0 {
		t.Fatal("expected default route rules to be non-empty")
	}

	for _, rule := range rules {
		if rule.ID == "" {
			t.Error("default route rule has empty ID")
		}
		if len(rule.Channels) == 0 {
			t.Errorf("default route rule %s has no channels", rule.ID)
		}
	}
}

// 监听器测试

type testListener struct {
	alerts   []*Alert
	resolves []*Alert
}

func (l *testListener) OnAlert(alert *Alert) {
	l.alerts = append(l.alerts, alert)
}

func (l *testListener) OnResolve(alert *Alert) {
	l.resolves = append(l.resolves, alert)
}

func (l *testListener) OnEscalate(alert *Alert, level int) {}

func TestAlertListener(t *testing.T) {
	am := NewAlertManager()
	defer am.Stop()

	listener := &testListener{}
	am.AddListener(listener)

	am.CreateAlert(&Alert{ID: "a1", Title: "test", Severity: SeverityInfo})
	time.Sleep(100 * time.Millisecond) // 等待异步通知

	if len(listener.alerts) != 1 {
		t.Errorf("expected 1 alert notification, got %d", len(listener.alerts))
	}

	am.ResolveAlert("a1", "")
	time.Sleep(100 * time.Millisecond)

	if len(listener.resolves) != 1 {
		t.Errorf("expected 1 resolve notification, got %d", len(listener.resolves))
	}
}

// 集成测试

func TestIntegrationFullWorkflow(t *testing.T) {
	am := NewAlertManager()
	defer am.Stop()

	// 1. 创建告警
	am.CreateAlert(&Alert{
		ID:          "disk-alert-1",
		Title:       "磁盘空间不足",
		Description: "/dev/sda1 使用率 95%",
		Severity:    SeverityCritical,
		Category:    CategoryDisk,
		Source:      "storage-monitor",
		Guidance: &Guidance{
			Steps: []RepairStep{
				{Order: 1, Title: "检查磁盘", Description: "运行 df -h"},
			},
			Difficulty:   "easy",
			EstimatedMin: 5,
		},
	})

	// 2. 验证告警创建
	alert, ok := am.GetAlert("disk-alert-1")
	if !ok {
		t.Fatal("alert not found")
	}
	if alert.Status != StatusActive {
		t.Errorf("expected active status, got %s", alert.Status)
	}

	// 3. 确认告警
	am.AcknowledgeAlert("disk-alert-1", "admin")
	alert, _ = am.GetAlert("disk-alert-1")
	if alert.Status != StatusAcknowledged {
		t.Errorf("expected acknowledged status, got %s", alert.Status)
	}

	// 4. 解决告警
	am.ResolveAlert("disk-alert-1", "admin")
	alert, _ = am.GetAlert("disk-alert-1")
	if alert.Status != StatusResolved {
		t.Errorf("expected resolved status, got %s", alert.Status)
	}
	if alert.ResolvedAt == nil {
		t.Error("expected ResolvedAt to be set")
	}

	// 5. 检查历史记录
	history := am.GetAlertHistory("disk-alert-1")
	if len(history) < 3 {
		t.Errorf("expected at least 3 history entries, got %d", len(history))
	}

	// 6. 检查统计
	stats := am.GetStats()
	if stats.Total != 1 {
		t.Errorf("expected total 1, got %d", stats.Total)
	}
	if stats.ResolvedCount != 1 {
		t.Errorf("expected resolved 1, got %d", stats.ResolvedCount)
	}
}

func TestIntegrationSilenceAndInhibit(t *testing.T) {
	am := NewAlertManager()
	defer am.Stop()

	// 添加静默规则
	am.AddSilenceRule(&SilenceRule{
		ID:   "s1",
		Name: "Silence network",
		Matchers: []LabelMatcher{
			{Name: "category", Value: "network", IsEqual: true},
		},
		StartsAt: time.Now().Add(-1 * time.Hour),
		EndsAt:   time.Now().Add(1 * time.Hour),
		Enabled:  true,
	})

	// 添加抑制规则
	am.AddInhibitionRule(&InhibitionRule{
		ID:   "i1",
		Name: "Inhibit lower disk alerts",
		SourceMatchers: []LabelMatcher{
			{Name: "severity", Value: "critical", IsEqual: true},
			{Name: "category", Value: "disk", IsEqual: true},
		},
		TargetMatchers: []LabelMatcher{
			{Name: "severity", Value: "warning", IsEqual: true},
			{Name: "category", Value: "disk", IsEqual: true},
		},
		Equal:   []string{"category"},
		Enabled: true,
	})

	// 网络告警应被静默
	am.CreateAlert(&Alert{ID: "a1", Category: CategoryNetwork, Severity: SeverityWarning})
	a1, _ := am.GetAlert("a1")
	if a1.Status != StatusSilenced {
		t.Errorf("network alert should be silenced, got %s", a1.Status)
	}

	// CPU 告警不应受影响
	am.CreateAlert(&Alert{ID: "a2", Category: CategoryCPU, Severity: SeverityWarning})
	a2, _ := am.GetAlert("a2")
	if a2.Status != StatusActive {
		t.Errorf("cpu alert should be active, got %s", a2.Status)
	}

	// critical disk 告警
	am.CreateAlert(&Alert{ID: "a3", Category: CategoryDisk, Severity: SeverityCritical})
	a3, _ := am.GetAlert("a3")
	if a3.Status != StatusActive {
		t.Errorf("critical disk alert should be active, got %s", a3.Status)
	}

	// warning disk 告警应被抑制
	am.CreateAlert(&Alert{ID: "a4", Category: CategoryDisk, Severity: SeverityWarning})
	a4, _ := am.GetAlert("a4")
	if a4.Status != StatusSilenced {
		t.Errorf("warning disk alert should be inhibited, got %s", a4.Status)
	}
}
