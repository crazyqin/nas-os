// Package homedashboard 测试
package homedashboard

import (
	"testing"
	"time"
)

func TestCreateDashboard(t *testing.T) {
	m := NewManager()

	d := m.CreateDashboard(CreateDashboardRequest{
		UserID: "user1",
		Name:   "我的仪表盘",
	})

	if d == nil {
		t.Fatal("仪表盘不应为nil")
	}
	if d.UserID != "user1" {
		t.Errorf("用户ID不匹配: %s", d.UserID)
	}
	if d.Name != "我的仪表盘" {
		t.Errorf("名称不匹配: %s", d.Name)
	}
	if len(d.Layouts) != 1 {
		t.Errorf("应有1个默认布局: %d", len(d.Layouts))
	}
	if !d.Layouts[0].IsDefault {
		t.Error("默认布局应标记为默认")
	}
	if d.Layouts[0].Columns != 12 {
		t.Errorf("默认列数应为12: %d", d.Layouts[0].Columns)
	}
}

func TestGetDashboard(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})

	got, err := m.GetDashboard(d.ID)
	if err != nil {
		t.Fatalf("获取仪表盘失败: %v", err)
	}
	if got.ID != d.ID {
		t.Errorf("ID不匹配")
	}
}

func TestGetDashboardNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetDashboard("nonexistent")
	if err == nil {
		t.Error("不存在的仪表盘应返回错误")
	}
}

func TestListDashboards(t *testing.T) {
	m := NewManager()
	m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "d1"})
	m.CreateDashboard(CreateDashboardRequest{UserID: "u2", Name: "d2"})
	m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "d3"})

	all := m.ListDashboards("")
	if len(all) != 3 {
		t.Errorf("应有3个仪表盘: %d", len(all))
	}

	u1 := m.ListDashboards("u1")
	if len(u1) != 2 {
		t.Errorf("u1应有2个仪表盘: %d", len(u1))
	}
}

func TestUpdateDashboard(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "old"})

	updated, err := m.UpdateDashboard(d.ID, "new")
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Name != "new" {
		t.Errorf("名称不匹配: %s", updated.Name)
	}
}

func TestUpdateDashboardNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.UpdateDashboard("nonexistent", "new")
	if err == nil {
		t.Error("不存在的仪表盘应返回错误")
	}
}

func TestDeleteDashboard(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})

	err := m.DeleteDashboard(d.ID)
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}

	_, err = m.GetDashboard(d.ID)
	if err == nil {
		t.Error("已删除的仪表盘不应存在")
	}
}

func TestDeleteDashboardNotFound(t *testing.T) {
	m := NewManager()
	err := m.DeleteDashboard("nonexistent")
	if err == nil {
		t.Error("删除不存在的仪表盘应返回错误")
	}
}

func TestDashboardCount(t *testing.T) {
	m := NewManager()
	if m.DashboardCount() != 0 {
		t.Errorf("初始仪表盘数应为0: %d", m.DashboardCount())
	}

	m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "d1"})
	m.CreateDashboard(CreateDashboardRequest{UserID: "u2", Name: "d2"})
	if m.DashboardCount() != 2 {
		t.Errorf("仪表盘数应为2: %d", m.DashboardCount())
	}
}

func TestAddLayout(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})

	layout, err := m.AddLayout(d.ID, CreateLayoutRequest{
		Name:    "布局2",
		Columns: 8,
		Rows:    6,
	})
	if err != nil {
		t.Fatalf("添加布局失败: %v", err)
	}
	if layout.Name != "布局2" {
		t.Errorf("布局名称不匹配: %s", layout.Name)
	}
	if layout.Columns != 8 {
		t.Errorf("列数不匹配: %d", layout.Columns)
	}

	// 验证仪表盘更新
	got, _ := m.GetDashboard(d.ID)
	if len(got.Layouts) != 2 {
		t.Errorf("应有2个布局: %d", len(got.Layouts))
	}
}

func TestAddLayoutDefaults(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})

	layout, err := m.AddLayout(d.ID, CreateLayoutRequest{Name: "auto"})
	if err != nil {
		t.Fatalf("添加布局失败: %v", err)
	}
	if layout.Columns != 12 {
		t.Errorf("默认列数应为12: %d", layout.Columns)
	}
	if layout.Rows != 8 {
		t.Errorf("默认行数应为8: %d", layout.Rows)
	}
}

func TestAddLayoutDashboardNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.AddLayout("nonexistent", CreateLayoutRequest{Name: "test"})
	if err == nil {
		t.Error("不存在的仪表盘应返回错误")
	}
}

func TestSetActiveLayout(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})
	layout, _ := m.AddLayout(d.ID, CreateLayoutRequest{Name: "layout2"})

	err := m.SetActiveLayout(d.ID, layout.ID)
	if err != nil {
		t.Fatalf("设置活动布局失败: %v", err)
	}

	got, _ := m.GetDashboard(d.ID)
	if got.ActiveLayout != layout.ID {
		t.Errorf("活动布局不匹配: %s", got.ActiveLayout)
	}
}

func TestSetActiveLayoutNotFound(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})

	err := m.SetActiveLayout(d.ID, "nonexistent")
	if err == nil {
		t.Error("不存在的布局应返回错误")
	}
}

func TestDeleteLayout(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})
	layout, _ := m.AddLayout(d.ID, CreateLayoutRequest{Name: "extra"})

	err := m.DeleteLayout(d.ID, layout.ID)
	if err != nil {
		t.Fatalf("删除布局失败: %v", err)
	}

	got, _ := m.GetDashboard(d.ID)
	if len(got.Layouts) != 1 {
		t.Errorf("应有1个布局: %d", len(got.Layouts))
	}
}

func TestDeleteDefaultLayout(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})

	err := m.DeleteLayout(d.ID, d.Layouts[0].ID)
	if err == nil {
		t.Error("不应允许删除默认布局")
	}
}

func TestAddWidget(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})
	layout := d.Layouts[0]

	widget, err := m.AddWidget(d.ID, layout.ID, AddWidgetRequest{
		Type:     WidgetTypeNASStatus,
		Title:    "NAS状态",
		Size:     WidgetSize{Width: 6, Height: 4},
		Position: WidgetPosition{X: 0, Y: 0},
	})
	if err != nil {
		t.Fatalf("添加Widget失败: %v", err)
	}
	if widget.Type != WidgetTypeNASStatus {
		t.Errorf("Widget类型不匹配: %s", widget.Type)
	}
	if widget.Title != "NAS状态" {
		t.Errorf("Widget标题不匹配: %s", widget.Title)
	}
	if widget.Size.Width != 6 {
		t.Errorf("宽度不匹配: %d", widget.Size.Width)
	}
}

func TestAddWidgetDefaults(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})
	layout := d.Layouts[0]

	widget, _ := m.AddWidget(d.ID, layout.ID, AddWidgetRequest{
		Type: WidgetTypeWeather,
	})
	if widget.Size.Width != 4 {
		t.Errorf("默认宽度应为4: %d", widget.Size.Width)
	}
	if widget.Size.Height != 2 {
		t.Errorf("默认高度应为2: %d", widget.Size.Height)
	}
}

func TestAddWidgetDashboardNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.AddWidget("nonexistent", "layout", AddWidgetRequest{Type: WidgetTypeWeather})
	if err == nil {
		t.Error("不存在的仪表盘应返回错误")
	}
}

func TestAddWidgetLayoutNotFound(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})
	_, err := m.AddWidget(d.ID, "nonexistent", AddWidgetRequest{Type: WidgetTypeWeather})
	if err == nil {
		t.Error("不存在的布局应返回错误")
	}
}

func TestGetWidget(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})
	layout := d.Layouts[0]
	widget, _ := m.AddWidget(d.ID, layout.ID, AddWidgetRequest{Type: WidgetTypeWeather, Title: "天气"})

	got, err := m.GetWidget(d.ID, layout.ID, widget.ID)
	if err != nil {
		t.Fatalf("获取Widget失败: %v", err)
	}
	if got.Title != "天气" {
		t.Errorf("标题不匹配: %s", got.Title)
	}
}

func TestGetWidgetNotFound(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})
	layout := d.Layouts[0]

	_, err := m.GetWidget(d.ID, layout.ID, "nonexistent")
	if err == nil {
		t.Error("不存在的Widget应返回错误")
	}
}

func TestUpdateWidget(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})
	layout := d.Layouts[0]
	widget, _ := m.AddWidget(d.ID, layout.ID, AddWidgetRequest{Type: WidgetTypeWeather, Title: "old"})

	newTitle := "new"
	updated, err := m.UpdateWidget(d.ID, layout.ID, widget.ID, UpdateWidgetRequest{Title: &newTitle})
	if err != nil {
		t.Fatalf("更新Widget失败: %v", err)
	}
	if updated.Title != "new" {
		t.Errorf("标题不匹配: %s", updated.Title)
	}
}

func TestUpdateWidgetPosition(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})
	layout := d.Layouts[0]
	widget, _ := m.AddWidget(d.ID, layout.ID, AddWidgetRequest{
		Type:     WidgetTypeWeather,
		Position: WidgetPosition{X: 0, Y: 0},
	})

	newPos := WidgetPosition{X: 3, Y: 5}
	updated, _ := m.UpdateWidget(d.ID, layout.ID, widget.ID, UpdateWidgetRequest{Position: &newPos})
	if updated.Position.X != 3 || updated.Position.Y != 5 {
		t.Errorf("位置不匹配: %+v", updated.Position)
	}
}

func TestDeleteWidget(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})
	layout := d.Layouts[0]
	widget, _ := m.AddWidget(d.ID, layout.ID, AddWidgetRequest{Type: WidgetTypeWeather})

	err := m.DeleteWidget(d.ID, layout.ID, widget.ID)
	if err != nil {
		t.Fatalf("删除Widget失败: %v", err)
	}

	got, _ := m.GetDashboard(d.ID)
	if len(got.Layouts[0].Widgets) != 0 {
		t.Errorf("Widget应已被删除: %d", len(got.Layouts[0].Widgets))
	}
}

func TestDeleteWidgetNotFound(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})
	layout := d.Layouts[0]

	err := m.DeleteWidget(d.ID, layout.ID, "nonexistent")
	if err == nil {
		t.Error("不存在的Widget应返回错误")
	}
}

func TestWidgetTemplates(t *testing.T) {
	m := NewManager()

	tmpl := &WidgetTemplate{
		ID:          "tmpl-1",
		Name:        "天气Widget",
		Description: "显示天气信息",
		Type:        WidgetTypeWeather,
		Author:      "nas-os",
		Version:     "1.0.0",
		DefaultSize: WidgetSize{Width: 4, Height: 3},
		Downloads:   100,
		Rating:      4.5,
		CreatedAt:   time.Now(),
	}

	m.RegisterTemplate(tmpl)

	// 获取模板
	got, err := m.GetTemplate("tmpl-1")
	if err != nil {
		t.Fatalf("获取模板失败: %v", err)
	}
	if got.Name != "天气Widget" {
		t.Errorf("模板名称不匹配: %s", got.Name)
	}

	// 列出模板
	templates := m.ListTemplates("")
	if len(templates) != 1 {
		t.Errorf("应有1个模板: %d", len(templates))
	}

	// 按类型过滤
	wTemplates := m.ListTemplates(WidgetTypeWeather)
	if len(wTemplates) != 1 {
		t.Errorf("天气模板应有1个: %d", len(wTemplates))
	}

	otherTemplates := m.ListTemplates(WidgetTypeNASStatus)
	if len(otherTemplates) != 0 {
		t.Errorf("NAS状态模板应为0: %d", len(otherTemplates))
	}
}

func TestDownloadTemplate(t *testing.T) {
	m := NewManager()
	tmpl := &WidgetTemplate{ID: "tmpl-1", Name: "test", Downloads: 10}
	m.RegisterTemplate(tmpl)

	err := m.DownloadTemplate("tmpl-1")
	if err != nil {
		t.Fatalf("下载模板失败: %v", err)
	}

	got, _ := m.GetTemplate("tmpl-1")
	if got.Downloads != 11 {
		t.Errorf("下载次数应为11: %d", got.Downloads)
	}
}

func TestDownloadTemplateNotFound(t *testing.T) {
	m := NewManager()
	err := m.DownloadTemplate("nonexistent")
	if err == nil {
		t.Error("不存在的模板应返回错误")
	}
}

func TestRateTemplate(t *testing.T) {
	m := NewManager()
	tmpl := &WidgetTemplate{ID: "tmpl-1", Name: "test", Downloads: 10, Rating: 4.0}
	m.RegisterTemplate(tmpl)

	err := m.RateTemplate("tmpl-1", 5.0)
	if err != nil {
		t.Fatalf("评价模板失败: %v", err)
	}

	got, _ := m.GetTemplate("tmpl-1")
	if got.Rating < 4.0 || got.Rating > 5.0 {
		t.Errorf("评分应在4-5之间: %f", got.Rating)
	}
}

func TestRateTemplateInvalidRating(t *testing.T) {
	m := NewManager()
	tmpl := &WidgetTemplate{ID: "tmpl-1", Name: "test"}
	m.RegisterTemplate(tmpl)

	err := m.RateTemplate("tmpl-1", 6.0)
	if err == nil {
		t.Error("无效评分应返回错误")
	}

	err = m.RateTemplate("tmpl-1", -1.0)
	if err == nil {
		t.Error("负评分应返回错误")
	}
}

func TestRateTemplateNotFound(t *testing.T) {
	m := NewManager()
	err := m.RateTemplate("nonexistent", 3.0)
	if err == nil {
		t.Error("不存在的模板应返回错误")
	}
}

func TestTemplateCount(t *testing.T) {
	m := NewManager()
	if m.TemplateCount() != 0 {
		t.Errorf("初始模板数应为0: %d", m.TemplateCount())
	}

	m.RegisterTemplate(&WidgetTemplate{ID: "t1", Name: "t1"})
	m.RegisterTemplate(&WidgetTemplate{ID: "t2", Name: "t2"})
	if m.TemplateCount() != 2 {
		t.Errorf("模板数应为2: %d", m.TemplateCount())
	}
}

func TestWidgetDataCache(t *testing.T) {
	m := NewManager()

	data := NASStatusData{
		Hostname:  "nas-server",
		CPU:       CPUData{UsagePercent: 45.5, Cores: 8},
		Timestamp: time.Now(),
	}

	m.SetWidgetData("widget-1", data)
	got := m.GetWidgetData("widget-1")
	if got == nil {
		t.Fatal("缓存数据不应为nil")
	}

	cached := got.(NASStatusData)
	if cached.Hostname != "nas-server" {
		t.Errorf("主机名不匹配: %s", cached.Hostname)
	}
	if cached.CPU.UsagePercent != 45.5 {
		t.Errorf("CPU使用率不匹配: %f", cached.CPU.UsagePercent)
	}
}

func TestWidgetDataCacheNotFound(t *testing.T) {
	m := NewManager()
	got := m.GetWidgetData("nonexistent")
	if got != nil {
		t.Error("不存在的缓存应返回nil")
	}
}

func TestWebSocketSubscribe(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})

	ch := m.Subscribe(d.ID)

	msg := WSMessage{
		Type:    "widget_update",
		Payload: map[string]string{"widget_id": "w1"},
		Time:    time.Now(),
	}
	m.NotifySubscribers(d.ID, msg)

	select {
	case received := <-ch:
		if received.Type != "widget_update" {
			t.Errorf("消息类型不匹配: %s", received.Type)
		}
	default:
		t.Error("应收到消息")
	}

	m.Unsubscribe(d.ID, ch)
}

func TestWebSocketUnsubscribe(t *testing.T) {
	m := NewManager()
	d := m.CreateDashboard(CreateDashboardRequest{UserID: "u1", Name: "test"})

	ch := m.Subscribe(d.ID)
	m.Unsubscribe(d.ID, ch)

	// channel 应已关闭
	_, ok := <-ch
	if ok {
		t.Error("channel应已关闭")
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"a/b/c", []string{"a", "b", "c"}},
		{"single", []string{"single"}},
		{"a/b/", []string{"a", "b"}},
		{"/a/b", []string{"a", "b"}},
		{"", []string{}},
	}

	for _, tt := range tests {
		result := splitPath(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("splitPath(%q): 期望%d段，实际%d段", tt.input, len(tt.expected), len(result))
			continue
		}
		for i, s := range result {
			if s != tt.expected[i] {
				t.Errorf("splitPath(%q)[%d]: 期望%q，实际%q", tt.input, i, tt.expected[i], s)
			}
		}
	}
}
