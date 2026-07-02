package dashboard

import (
	"encoding/json"
	"testing"
	"time"
)

// ===== 进程管理测试 =====

func TestProcessManagerUpdateAndGet(t *testing.T) {
	pm := NewProcessManager()
	now := time.Now()

	pm.Update([]*ProcessInfo{
		{PID: 1, Name: "init", User: "root", CPUPercent: 0.1, MemPercent: 0.5, Status: "running", CreateTime: now},
		{PID: 100, Name: "nginx", User: "www", CPUPercent: 5.0, MemPercent: 2.0, Status: "running", CreateTime: now},
		{PID: 200, Name: "postgres", User: "pg", CPUPercent: 15.0, MemPercent: 8.0, Status: "running", CreateTime: now},
	})

	if pm.Count() != 3 {
		t.Errorf("期望 3 个进程，实际 %d", pm.Count())
	}

	p, err := pm.Get(100)
	if err != nil {
		t.Fatalf("获取进程失败: %v", err)
	}
	if p.Name != "nginx" {
		t.Errorf("进程名应为 nginx，实际 %s", p.Name)
	}
}

func TestProcessManagerGetNotFound(t *testing.T) {
	pm := NewProcessManager()
	_, err := pm.Get(999)
	if err == nil {
		t.Error("不存在的进程应返回错误")
	}
}

func TestProcessManagerTopCPU(t *testing.T) {
	pm := NewProcessManager()
	pm.Update([]*ProcessInfo{
		{PID: 1, Name: "a", CPUPercent: 1.0},
		{PID: 2, Name: "b", CPUPercent: 30.0},
		{PID: 3, Name: "c", CPUPercent: 15.0},
		{PID: 4, Name: "d", CPUPercent: 50.0},
	})

	top := pm.TopCPU(2)
	if len(top) != 2 {
		t.Fatalf("期望 2 个，实际 %d", len(top))
	}
	if top[0].Name != "d" {
		t.Errorf("第一名应为 d，实际 %s", top[0].Name)
	}
	if top[1].Name != "b" {
		t.Errorf("第二名应为 b，实际 %s", top[1].Name)
	}
}

func TestProcessManagerTopMemory(t *testing.T) {
	pm := NewProcessManager()
	pm.Update([]*ProcessInfo{
		{PID: 1, Name: "low", MemPercent: 1.0},
		{PID: 2, Name: "high", MemPercent: 50.0},
		{PID: 3, Name: "mid", MemPercent: 20.0},
	})

	top := pm.TopMemory(1)
	if len(top) != 1 || top[0].Name != "high" {
		t.Errorf("最高内存应为 high")
	}
}

func TestProcessManagerListByStatus(t *testing.T) {
	pm := NewProcessManager()
	pm.Update([]*ProcessInfo{
		{PID: 1, Name: "r1", Status: "running"},
		{PID: 2, Name: "r2", Status: "running"},
		{PID: 3, Name: "s1", Status: "sleeping"},
	})

	running := pm.ListByStatus("running")
	if len(running) != 2 {
		t.Errorf("运行中应有 2 个，实际 %d", len(running))
	}
}

func TestProcessManagerSearch(t *testing.T) {
	pm := NewProcessManager()
	pm.Update([]*ProcessInfo{
		{PID: 1, Name: "nginx", CmdLine: "/usr/sbin/nginx"},
		{PID: 2, Name: "postgres", CmdLine: "/usr/bin/postgres"},
		{PID: 3, Name: "node", CmdLine: "node server.js"},
	})

	results := pm.Search("nginx")
	if len(results) != 1 {
		t.Errorf("搜索 nginx 应返回 1 个，实际 %d", len(results))
	}

	results = pm.Search("NGINX")
	if len(results) != 1 {
		t.Errorf("大小写不敏感搜索应返回 1 个，实际 %d", len(results))
	}
}

func TestProcessManagerTopNOverflow(t *testing.T) {
	pm := NewProcessManager()
	pm.Update([]*ProcessInfo{
		{PID: 1, Name: "a", CPUPercent: 1.0},
	})

	top := pm.TopCPU(10)
	if len(top) != 1 {
		t.Errorf("N 超过总数时应返回全部，实际 %d", len(top))
	}
}

// ===== 服务管理测试 =====

func TestServiceManagerRegisterAndGet(t *testing.T) {
	sm := NewServiceManager()

	sm.Register(&ServiceInfo{
		Name:        "nginx",
		State:       ServiceRunning,
		Description: "Web 服务器",
		PID:         1234,
		Enabled:     true,
	})

	svc, err := sm.Get("nginx")
	if err != nil {
		t.Fatalf("获取服务失败: %v", err)
	}
	if svc.State != ServiceRunning {
		t.Errorf("状态应为 running，实际 %s", svc.State)
	}
}

func TestServiceManagerGetNotFound(t *testing.T) {
	sm := NewServiceManager()
	_, err := sm.Get("nope")
	if err == nil {
		t.Error("未注册服务应返回错误")
	}
}

func TestServiceManagerUpdate(t *testing.T) {
	sm := NewServiceManager()
	sm.Register(&ServiceInfo{Name: "app", State: ServiceRunning})

	err := sm.Update("app", ServiceStopped)
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	svc, _ := sm.Get("app")
	if svc.State != ServiceStopped {
		t.Errorf("状态应为 stopped，实际 %s", svc.State)
	}
}

func TestServiceManagerUpdateNotFound(t *testing.T) {
	sm := NewServiceManager()
	err := sm.Update("nope", ServiceRunning)
	if err == nil {
		t.Error("更新未注册服务应返回错误")
	}
}

func TestServiceManagerListByState(t *testing.T) {
	sm := NewServiceManager()
	sm.Register(&ServiceInfo{Name: "a", State: ServiceRunning})
	sm.Register(&ServiceInfo{Name: "b", State: ServiceStopped})
	sm.Register(&ServiceInfo{Name: "c", State: ServiceRunning})
	sm.Register(&ServiceInfo{Name: "d", State: ServiceFailed})

	running := sm.ListByState(ServiceRunning)
	if len(running) != 2 {
		t.Errorf("运行中应有 2 个，实际 %d", len(running))
	}

	failed := sm.ListByState(ServiceFailed)
	if len(failed) != 1 {
		t.Errorf("失败应有 1 个，实际 %d", len(failed))
	}
}

func TestServiceManagerSummary(t *testing.T) {
	sm := NewServiceManager()
	sm.Register(&ServiceInfo{Name: "a", State: ServiceRunning})
	sm.Register(&ServiceInfo{Name: "b", State: ServiceRunning})
	sm.Register(&ServiceInfo{Name: "c", State: ServiceStopped})
	sm.Register(&ServiceInfo{Name: "d", State: ServiceFailed})

	summary := sm.Summary()
	if summary.Total != 4 {
		t.Errorf("总数应为 4，实际 %d", summary.Total)
	}
	if summary.Running != 2 {
		t.Errorf("运行中应为 2，实际 %d", summary.Running)
	}
	if summary.Stopped != 1 {
		t.Errorf("停止应为 1，实际 %d", summary.Stopped)
	}
	if summary.Failed != 1 {
		t.Errorf("失败应为 1，实际 %d", summary.Failed)
	}
}

func TestServiceManagerUnregister(t *testing.T) {
	sm := NewServiceManager()
	sm.Register(&ServiceInfo{Name: "tmp", State: ServiceRunning})
	sm.Unregister("tmp")

	if sm.Count() != 0 {
		t.Error("注销后应为空")
	}
}

// ===== 告警管理测试 =====

func TestAlertManagerAddRule(t *testing.T) {
	am := NewAlertManager(100)

	err := am.AddRule(&AlertRule{
		ID:        "cpu-high",
		Name:      "CPU 过高",
		Metric:    "cpu",
		Operator:  OpGT,
		Enabled:   true,
		Threshold: 80.0,
		Severity:  AlertWarning,
	})
	if err != nil {
		t.Fatalf("添加规则失败: %v", err)
	}

	if am.RuleCount() != 1 {
		t.Errorf("规则数应为 1，实际 %d", am.RuleCount())
	}
}

func TestAlertManagerAddDuplicateRule(t *testing.T) {
	am := NewAlertManager(100)
	am.AddRule(&AlertRule{ID: "r1", Name: "test", Metric: "cpu", Operator: OpGT, Threshold: 50, Enabled: true})

	err := am.AddRule(&AlertRule{ID: "r1", Name: "dup", Metric: "cpu", Operator: OpGT, Threshold: 60, Enabled: true})
	if err == nil {
		t.Error("重复 ID 应返回错误")
	}
}

func TestAlertManagerAddRuleValidation(t *testing.T) {
	am := NewAlertManager(100)

	err := am.AddRule(&AlertRule{})
	if err == nil {
		t.Error("空 ID 应返回错误")
	}

	err = am.AddRule(&AlertRule{ID: "r1"})
	if err == nil {
		t.Error("空名称应返回错误")
	}

	err = am.AddRule(&AlertRule{ID: "r2", Name: "n"})
	if err == nil {
		t.Error("空指标应返回错误")
	}
}

func TestAlertManagerEvaluateTrigger(t *testing.T) {
	am := NewAlertManager(100)
	am.AddRule(&AlertRule{
		ID: "cpu-high", Name: "CPU 过高", Metric: "cpu",
		Operator: OpGT, Threshold: 80.0, Severity: AlertCritical,
		Enabled: true,
	})

	events := am.Evaluate("cpu", 90.0)
	if len(events) != 1 {
		t.Errorf("应触发 1 条告警，实际 %d", len(events))
	}
	if events[0].Severity != AlertCritical {
		t.Errorf("应为 critical，实际 %s", events[0].Severity)
	}
}

func TestAlertManagerEvaluateNoTrigger(t *testing.T) {
	am := NewAlertManager(100)
	am.AddRule(&AlertRule{
		ID: "cpu-high", Name: "CPU 过高", Metric: "cpu",
		Operator: OpGT, Threshold: 80.0, Severity: AlertWarning,
		Enabled: true,
	})

	events := am.Evaluate("cpu", 50.0)
	if len(events) != 0 {
		t.Errorf("不应触发告警，实际 %d 条", len(events))
	}
}

func TestAlertManagerEvaluateOperators(t *testing.T) {
	am := NewAlertManager(100)
	am.AddRule(&AlertRule{ID: "gt", Name: "gt", Metric: "m", Operator: OpGT, Threshold: 10, Enabled: true})
	am.AddRule(&AlertRule{ID: "gte", Name: "gte", Metric: "m", Operator: OpGTE, Threshold: 10, Enabled: true})
	am.AddRule(&AlertRule{ID: "lt", Name: "lt", Metric: "m", Operator: OpLT, Threshold: 10, Enabled: true})
	am.AddRule(&AlertRule{ID: "lte", Name: "lte", Metric: "m", Operator: OpLTE, Threshold: 10, Enabled: true})
	am.AddRule(&AlertRule{ID: "eq", Name: "eq", Metric: "m", Operator: OpEQ, Threshold: 10, Enabled: true})

	events := am.Evaluate("m", 10)
	// gte, lte, eq should trigger (value == 10)
	triggered := make(map[string]bool)
	for _, e := range events {
		triggered[e.RuleID] = true
	}
	if !triggered["gte"] || !triggered["lte"] || !triggered["eq"] {
		t.Error(">=, <=, == 在 value=10 时应触发")
	}
	if triggered["gt"] || triggered["lt"] {
		t.Error(">, < 在 value=10 时不应触发")
	}
}

func TestAlertManagerDisabledRule(t *testing.T) {
	am := NewAlertManager(100)
	am.AddRule(&AlertRule{
		ID: "disabled", Name: "x", Metric: "cpu",
		Operator: OpGT, Threshold: 0, Enabled: false,
	})

	events := am.Evaluate("cpu", 100)
	if len(events) != 0 {
		t.Error("禁用规则不应触发")
	}
}

func TestAlertManagerDeleteRule(t *testing.T) {
	am := NewAlertManager(100)
	am.AddRule(&AlertRule{ID: "r1", Name: "n", Metric: "m", Operator: OpGT, Threshold: 1, Enabled: true})

	err := am.DeleteRule("r1")
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if am.RuleCount() != 0 {
		t.Error("删除后规则数应为 0")
	}

	err = am.DeleteRule("nonexistent")
	if err == nil {
		t.Error("删除不存在的规则应返回错误")
	}
}

func TestAlertManagerGetEvents(t *testing.T) {
	am := NewAlertManager(100)
	am.AddRule(&AlertRule{ID: "r1", Name: "n", Metric: "cpu", Operator: OpGT, Threshold: 0, Enabled: true})

	am.Evaluate("cpu", 10)
	am.Evaluate("cpu", 20)
	am.Evaluate("cpu", 30)

	events := am.GetEvents(2)
	if len(events) != 2 {
		t.Errorf("应返回 2 条事件，实际 %d", len(events))
	}
}

func TestAlertManagerResolveEvent(t *testing.T) {
	am := NewAlertManager(100)
	am.AddRule(&AlertRule{ID: "r1", Name: "n", Metric: "cpu", Operator: OpGT, Threshold: 0, Enabled: true})

	am.Evaluate("cpu", 50)
	events := am.GetEvents(1)
	if len(events) == 0 {
		t.Fatal("应有事件")
	}

	err := am.ResolveEvent(events[0].ID)
	if err != nil {
		t.Fatalf("解决事件失败: %v", err)
	}

	resolved := am.GetEvents(1)
	if !resolved[0].Resolved {
		t.Error("事件应已解决")
	}
}

func TestAlertManagerResolveEventNotFound(t *testing.T) {
	am := NewAlertManager(100)
	err := am.ResolveEvent("nonexistent")
	if err == nil {
		t.Error("解决不存在的事件应返回错误")
	}
}

func TestAlertManagerEventsBySeverity(t *testing.T) {
	am := NewAlertManager(100)
	am.AddRule(&AlertRule{ID: "w", Name: "w", Metric: "m", Operator: OpGT, Threshold: 0, Severity: AlertWarning, Enabled: true})
	am.AddRule(&AlertRule{ID: "c", Name: "c", Metric: "m", Operator: OpGT, Threshold: 0, Severity: AlertCritical, Enabled: true})

	am.Evaluate("m", 1)

	warnEvents := am.GetEventsBySeverity(AlertWarning, 0)
	if len(warnEvents) != 1 {
		t.Errorf("warning 事件应为 1，实际 %d", len(warnEvents))
	}
}

func TestAlertManagerUpdateRule(t *testing.T) {
	am := NewAlertManager(100)
	am.AddRule(&AlertRule{ID: "r1", Name: "old", Metric: "cpu", Operator: OpGT, Threshold: 80, Enabled: true})

	err := am.UpdateRule(&AlertRule{ID: "r1", Name: "new", Metric: "cpu", Operator: OpGTE, Threshold: 90})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	rule, _ := am.GetRule("r1")
	if rule.Name != "new" {
		t.Errorf("名称应为 new，实际 %s", rule.Name)
	}
	if rule.Threshold != 90 {
		t.Errorf("阈值应为 90，实际 %.0f", rule.Threshold)
	}
}

func TestAlertManagerUpdateRuleNotFound(t *testing.T) {
	am := NewAlertManager(100)
	err := am.UpdateRule(&AlertRule{ID: "nope", Name: "x", Metric: "m", Operator: OpGT})
	if err == nil {
		t.Error("更新不存在的规则应返回错误")
	}
}

// ===== 数据导出测试 =====

func TestMetricsExporterRecordAndExport(t *testing.T) {
	me := NewMetricsExporter(1000)
	now := time.Now()

	me.Record(MetricSample{
		Timestamp: now,
		Metric:    "cpu",
		Values:    map[string]interface{}{"usage": 45.5, "cores": 4},
	})
	me.Record(MetricSample{
		Timestamp: now.Add(time.Second),
		Metric:    "memory",
		Values:    map[string]interface{}{"usage": 60.0, "total": 8192},
	})

	if me.SampleCount() != 2 {
		t.Errorf("采样数应为 2，实际 %d", me.SampleCount())
	}

	result, err := me.Export(ExportRequest{
		Format: ExportJSON,
	})
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	if result.Count != 2 {
		t.Errorf("导出数应为 2，实际 %d", result.Count)
	}

	var data []MetricSample
	if err := json.Unmarshal(result.Data, &data); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}
	if len(data) != 2 {
		t.Errorf("JSON 数据应有 2 条，实际 %d", len(data))
	}
}

func TestMetricsExporterCSVExport(t *testing.T) {
	me := NewMetricsExporter(100)
	me.Record(MetricSample{
		Timestamp: time.Now(),
		Metric:    "cpu",
		Values:    map[string]interface{}{"usage": 50.0},
	})

	result, err := me.Export(ExportRequest{Format: ExportCSV})
	if err != nil {
		t.Fatalf("CSV 导出失败: %v", err)
	}
	if len(result.Data) == 0 {
		t.Error("CSV 数据不应为空")
	}
}

func TestMetricsExporterFilterByMetric(t *testing.T) {
	me := NewMetricsExporter(100)
	me.Record(MetricSample{Timestamp: time.Now(), Metric: "cpu", Values: map[string]interface{}{"v": 1}})
	me.Record(MetricSample{Timestamp: time.Now(), Metric: "memory", Values: map[string]interface{}{"v": 2}})
	me.Record(MetricSample{Timestamp: time.Now(), Metric: "cpu", Values: map[string]interface{}{"v": 3}})

	result, _ := me.Export(ExportRequest{
		Format:  ExportJSON,
		Metrics: []string{"cpu"},
	})
	if result.Count != 2 {
		t.Errorf("过滤 cpu 应返回 2 条，实际 %d", result.Count)
	}
}

func TestMetricsExporterFilterByTime(t *testing.T) {
	me := NewMetricsExporter(100)
	base := time.Now()
	me.Record(MetricSample{Timestamp: base.Add(-time.Hour), Metric: "cpu", Values: map[string]interface{}{"v": 1}})
	me.Record(MetricSample{Timestamp: base, Metric: "cpu", Values: map[string]interface{}{"v": 2}})
	me.Record(MetricSample{Timestamp: base.Add(time.Hour), Metric: "cpu", Values: map[string]interface{}{"v": 3}})

	result, _ := me.Export(ExportRequest{
		Format:    ExportJSON,
		StartTime: base.Add(-time.Minute),
		EndTime:   base.Add(time.Minute),
	})
	if result.Count != 1 {
		t.Errorf("时间过滤应返回 1 条，实际 %d", result.Count)
	}
}

func TestMetricsExporterLimit(t *testing.T) {
	me := NewMetricsExporter(100)
	for i := 0; i < 20; i++ {
		me.Record(MetricSample{Timestamp: time.Now(), Metric: "cpu", Values: map[string]interface{}{"i": i}})
	}

	result, _ := me.Export(ExportRequest{Format: ExportJSON, Limit: 5})
	if result.Count != 5 {
		t.Errorf("limit=5 应返回 5 条，实际 %d", result.Count)
	}
}

func TestMetricsExporterEmpty(t *testing.T) {
	me := NewMetricsExporter(100)
	result, _ := me.Export(ExportRequest{Format: ExportJSON})
	if result.Count != 0 {
		t.Errorf("无数据应返回 0 条，实际 %d", result.Count)
	}
}

func TestMetricsExporterInvalidFormat(t *testing.T) {
	me := NewMetricsExporter(100)
	me.Record(MetricSample{Timestamp: time.Now(), Metric: "cpu", Values: map[string]interface{}{}})
	_, err := me.Export(ExportRequest{Format: "xml"})
	if err == nil {
		t.Error("无效格式应返回错误")
	}
}

func TestMetricsExporterMaxSize(t *testing.T) {
	me := NewMetricsExporter(5)
	for i := 0; i < 10; i++ {
		me.Record(MetricSample{Timestamp: time.Now(), Metric: "cpu", Values: map[string]interface{}{}})
	}
	if me.SampleCount() != 5 {
		t.Errorf("超过上限应截断，期望 5，实际 %d", me.SampleCount())
	}
}

func TestMetricsExporterClear(t *testing.T) {
	me := NewMetricsExporter(100)
	me.Record(MetricSample{Timestamp: time.Now(), Metric: "cpu", Values: map[string]interface{}{}})
	me.Clear()
	if me.SampleCount() != 0 {
		t.Errorf("清除后应为 0，实际 %d", me.SampleCount())
	}
}
