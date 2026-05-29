package incidentmgr

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.incidents == nil {
		t.Fatal("incidents map not initialized")
	}
	if m.timeline == nil {
		t.Fatal("timeline map not initialized")
	}
	if m.rca == nil {
		t.Fatal("rca map not initialized")
	}
}

func TestCreateIncident(t *testing.T) {
	m := NewManager()

	// 测试创建成功
	inc := Incident{
		Title:       "测试事件",
		Description: "测试描述",
		Severity:    Sev2High,
	}

	created, err := m.CreateIncident(inc)
	if err != nil {
		t.Fatalf("CreateIncident failed: %v", err)
	}

	if created.ID == "" {
		t.Fatal("incident ID not generated")
	}
	if created.Title != inc.Title {
		t.Errorf("expected title %s, got %s", inc.Title, created.Title)
	}
	if created.Status != StatusOpen {
		t.Errorf("expected status %s, got %s", StatusOpen, created.Status)
	}
	if created.Severity != Sev2High {
		t.Errorf("expected severity %s, got %s", Sev2High, created.Severity)
	}

	// 测试标题为空
	_, err = m.CreateIncident(Incident{})
	if err == nil {
		t.Fatal("expected error for empty title")
	}

	// 测试默认严重程度
	inc2 := Incident{Title: "默认严重程度"}
	created2, err := m.CreateIncident(inc2)
	if err != nil {
		t.Fatalf("CreateIncident failed: %v", err)
	}
	if created2.Severity != Sev3Medium {
		t.Errorf("expected default severity %s, got %s", Sev3Medium, created2.Severity)
	}
}

func TestGetIncident(t *testing.T) {
	m := NewManager()

	// 创建事件
	inc := Incident{Title: "测试事件", Severity: Sev3Medium}
	created, _ := m.CreateIncident(inc)

	// 测试获取成功
	found, err := m.GetIncident(created.ID)
	if err != nil {
		t.Fatalf("GetIncident failed: %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, found.ID)
	}

	// 测试获取不存在的事件
	_, err = m.GetIncident("INC-20260101-9999")
	if err == nil {
		t.Fatal("expected error for non-existent incident")
	}
}

func TestListIncidents(t *testing.T) {
	m := NewManager()

	// 创建不同状态的事件
	m.CreateIncident(Incident{Title: "事件1", Severity: Sev1Critical})
	m.CreateIncident(Incident{Title: "事件2", Severity: Sev2High})
	m.CreateIncident(Incident{Title: "事件3", Severity: Sev3Medium})

	// 列出所有事件
	all, err := m.ListIncidents("")
	if err != nil {
		t.Fatalf("ListIncidents failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 incidents, got %d", len(all))
	}

	// 按状态过滤
	open, err := m.ListIncidents(StatusOpen)
	if err != nil {
		t.Fatalf("ListIncidents failed: %v", err)
	}
	if len(open) != 3 {
		t.Errorf("expected 3 open incidents, got %d", len(open))
	}

	closed, _ := m.ListIncidents(StatusClosed)
	if len(closed) != 0 {
		t.Errorf("expected 0 closed incidents, got %d", len(closed))
	}
}

func TestUpdateIncidentStatus(t *testing.T) {
	m := NewManager()

	inc := Incident{Title: "测试事件", Severity: Sev3Medium}
	created, _ := m.CreateIncident(inc)

	// 测试状态流转 Open -> Acknowledged
	err := m.UpdateIncidentStatus(created.ID, StatusAcknowledged)
	if err != nil {
		t.Fatalf("UpdateIncidentStatus failed: %v", err)
	}

	found, _ := m.GetIncident(created.ID)
	if found.Status != StatusAcknowledged {
		t.Errorf("expected status %s, got %s", StatusAcknowledged, found.Status)
	}

	// 测试无效状态流转 Open -> Resolved（跳过中间状态）
	err = m.UpdateIncidentStatus(created.ID, StatusResolved)
	if err == nil {
		t.Fatal("expected error for invalid status transition")
	}

	// 测试不存在的事件
	err = m.UpdateIncidentStatus("INC-20260101-9999", StatusClosed)
	if err == nil {
		t.Fatal("expected error for non-existent incident")
	}
}

func TestAssignIncident(t *testing.T) {
	m := NewManager()

	inc := Incident{Title: "测试事件", Severity: Sev3Medium}
	created, _ := m.CreateIncident(inc)

	// 测试分配成功
	err := m.AssignIncident(created.ID, "张三")
	if err != nil {
		t.Fatalf("AssignIncident failed: %v", err)
	}

	found, _ := m.GetIncident(created.ID)
	if found.Assignee != "张三" {
		t.Errorf("expected assignee 张三, got %s", found.Assignee)
	}

	// 测试重新分配
	err = m.AssignIncident(created.ID, "李四")
	if err != nil {
		t.Fatalf("AssignIncident failed: %v", err)
	}

	found, _ = m.GetIncident(created.ID)
	if found.Assignee != "李四" {
		t.Errorf("expected assignee 李四, got %s", found.Assignee)
	}

	// 测试不存在的事件
	err = m.AssignIncident("INC-20260101-9999", "王五")
	if err == nil {
		t.Fatal("expected error for non-existent incident")
	}
}

func TestTimeline(t *testing.T) {
	m := NewManager()

	inc := Incident{Title: "测试事件", Severity: Sev3Medium}
	created, _ := m.CreateIncident(inc)

	// 获取时间线
	events, err := m.GetTimeline(created.ID)
	if err != nil {
		t.Fatalf("GetTimeline failed: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 timeline event, got %d", len(events))
	}

	// 添加自定义事件
	err = m.AddTimelineEvent(created.ID, TimelineEvent{
		Type:        TimelineEventComment,
		Description: "这是一条评论",
		Operator:    "操作员A",
	})
	if err != nil {
		t.Fatalf("AddTimelineEvent failed: %v", err)
	}

	// 验证时间线
	events, _ = m.GetTimeline(created.ID)
	if len(events) != 2 {
		t.Errorf("expected 2 timeline events, got %d", len(events))
	}

	// 测试不存在的事件
	_, err = m.GetTimeline("INC-20260101-9999")
	if err == nil {
		t.Fatal("expected error for non-existent incident")
	}

	err = m.AddTimelineEvent("INC-20260101-9999", TimelineEvent{
		Type:        TimelineEventComment,
		Description: "测试",
	})
	if err == nil {
		t.Fatal("expected error for non-existent incident")
	}
}

func TestRCA(t *testing.T) {
	m := NewManager()

	inc := Incident{Title: "测试事件", Severity: Sev3Medium}
	created, _ := m.CreateIncident(inc)

	// 测试创建 RCA
	rca := RCAReport{
		RootCause:      "根因分析",
		ImpactScope:    "影响范围",
		FixActions:     []string{"修复措施1", "修复措施2"},
		PreventActions: []string{"预防措施1", "预防措施2"},
	}

	err := m.CreateRCA(created.ID, rca)
	if err != nil {
		t.Fatalf("CreateRCA failed: %v", err)
	}

	// 测试获取 RCA
	found, err := m.GetRCA(created.ID)
	if err != nil {
		t.Fatalf("GetRCA failed: %v", err)
	}
	if found.RootCause != rca.RootCause {
		t.Errorf("expected root cause %s, got %s", rca.RootCause, found.RootCause)
	}
	if len(found.FixActions) != 2 {
		t.Errorf("expected 2 fix actions, got %d", len(found.FixActions))
	}

	// 测试根因为空
	err = m.CreateRCA(created.ID, RCAReport{})
	if err == nil {
		t.Fatal("expected error for empty root cause")
	}

	// 测试不存在的事件
	err = m.CreateRCA("INC-20260101-9999", rca)
	if err == nil {
		t.Fatal("expected error for non-existent incident")
	}

	_, err = m.GetRCA("INC-20260101-9999")
	if err == nil {
		t.Fatal("expected error for non-existent incident")
	}
}

func TestGetIncidentStats(t *testing.T) {
	m := NewManager()

	// 创建不同状态和严重程度的事件
	m.CreateIncident(Incident{Title: "事件1", Severity: Sev1Critical})
	m.CreateIncident(Incident{Title: "事件2", Severity: Sev2High})
	m.CreateIncident(Incident{Title: "事件3", Severity: Sev3Medium})
	m.CreateIncident(Incident{Title: "事件4", Severity: Sev4Low})

	stats, err := m.GetIncidentStats()
	if err != nil {
		t.Fatalf("GetIncidentStats failed: %v", err)
	}

	if stats["total"] != 4 {
		t.Errorf("expected total 4, got %d", stats["total"])
	}
	if stats["open"] != 4 {
		t.Errorf("expected open 4, got %d", stats["open"])
	}
	if stats["critical"] != 1 {
		t.Errorf("expected critical 1, got %d", stats["critical"])
	}
	if stats["high"] != 1 {
		t.Errorf("expected high 1, got %d", stats["high"])
	}
	if stats["medium"] != 1 {
		t.Errorf("expected medium 1, got %d", stats["medium"])
	}
	if stats["low"] != 1 {
		t.Errorf("expected low 1, got %d", stats["low"])
	}
}

func TestEscalateIncident(t *testing.T) {
	m := NewManager()

	// 测试从 Low 升级到 Medium
	inc := Incident{Title: "测试事件", Severity: Sev4Low}
	created, _ := m.CreateIncident(inc)

	err := m.EscalateIncident(created.ID)
	if err != nil {
		t.Fatalf("EscalateIncident failed: %v", err)
	}

	found, _ := m.GetIncident(created.ID)
	if found.Severity != Sev3Medium {
		t.Errorf("expected severity %s, got %s", Sev3Medium, found.Severity)
	}

	// 测试从 Medium 升级到 High
	err = m.EscalateIncident(created.ID)
	if err != nil {
		t.Fatalf("EscalateIncident failed: %v", err)
	}

	found, _ = m.GetIncident(created.ID)
	if found.Severity != Sev2High {
		t.Errorf("expected severity %s, got %s", Sev2High, found.Severity)
	}

	// 测试从 High 升级到 Critical
	err = m.EscalateIncident(created.ID)
	if err != nil {
		t.Fatalf("EscalateIncident failed: %v", err)
	}

	found, _ = m.GetIncident(created.ID)
	if found.Severity != Sev1Critical {
		t.Errorf("expected severity %s, got %s", Sev1Critical, found.Severity)
	}

	// 测试已经是 Critical 无法升级
	err = m.EscalateIncident(created.ID)
	if err == nil {
		t.Fatal("expected error for already critical incident")
	}

	// 测试不存在的事件
	err = m.EscalateIncident("INC-20260101-9999")
	if err == nil {
		t.Fatal("expected error for non-existent incident")
	}
}

func TestConcurrency(t *testing.T) {
	m := NewManager()

	// 并发创建事件
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			_, err := m.CreateIncident(Incident{
				Title:    "并发事件",
				Severity: Sev3Medium,
			})
			if err != nil {
				t.Errorf("concurrent CreateIncident failed: %v", err)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	stats, _ := m.GetIncidentStats()
	if stats["total"] != 10 {
		t.Errorf("expected total 10, got %d", stats["total"])
	}
}

func TestIncidentIDFormat(t *testing.T) {
	m := NewManager()

	inc, _ := m.CreateIncident(Incident{
		Title:    "测试ID格式",
		Severity: Sev3Medium,
	})

	// 验证ID格式: INC-YYYYMMDD-XXXX
	if len(inc.ID) != 17 {
		t.Errorf("expected ID length 17, got %d: %s", len(inc.ID), inc.ID)
	}
	if inc.ID[:4] != "INC-" {
		t.Errorf("expected ID prefix INC-, got %s", inc.ID[:4])
	}
}

func TestIsValidStatusTransition(t *testing.T) {
	tests := []struct {
		from IncidentStatus
		to   IncidentStatus
		want bool
	}{
		{StatusOpen, StatusAcknowledged, true},
		{StatusOpen, StatusInvestigating, true},
		{StatusOpen, StatusClosed, true},
		{StatusOpen, StatusResolved, false},
		{StatusAcknowledged, StatusInvestigating, true},
		{StatusAcknowledged, StatusClosed, true},
		{StatusAcknowledged, StatusResolved, false},
		{StatusInvestigating, StatusResolved, true},
		{StatusInvestigating, StatusClosed, true},
		{StatusResolved, StatusClosed, true},
		{StatusResolved, StatusOpen, true},
		{StatusClosed, StatusOpen, true},
		{StatusClosed, StatusAcknowledged, false},
	}

	for _, tt := range tests {
		got := isValidStatusTransition(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("isValidStatusTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}
