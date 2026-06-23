package desktopwidgets

import (
	"testing"
)

func TestCreateWidget(t *testing.T) {
	mgr := NewWidgetManager()

	widget := &Widget{
		Type:   WidgetTypeClock,
		UserID: "user-1",
		Position: WidgetPosition{
			X:      10,
			Y:      10,
			Width:  200,
			Height: 100,
		},
		Config: WidgetConfig{
			Title: "时钟",
		},
	}

	err := mgr.CreateWidget(widget)
	if err != nil {
		t.Fatalf("创建小组件失败: %v", err)
	}

	if widget.ID == "" {
		t.Error("小组件 ID 不应为空")
	}
}

func TestGetWidgetData(t *testing.T) {
	mgr := NewWidgetManager()

	widget := &Widget{
		Type:   WidgetTypeCPU,
		UserID: "user-1",
	}
	mgr.CreateWidget(widget)

	data, err := mgr.GetWidgetData(widget.ID)
	if err != nil {
		t.Fatalf("获取小组件数据失败: %v", err)
	}

	if data.Data == nil {
		t.Error("数据不应为空")
	}
}

func TestDesktopLayout(t *testing.T) {
	mgr := NewWidgetManager()

	layout := &DesktopLayout{
		UserID:    "user-1",
		Wallpaper: "default.jpg",
		Theme:     "dark",
	}

	err := mgr.SaveLayout(layout)
	if err != nil {
		t.Fatalf("保存布局失败: %v", err)
	}

	saved, err := mgr.GetLayout("user-1")
	if err != nil {
		t.Fatalf("获取布局失败: %v", err)
	}

	if saved.Theme != "dark" {
		t.Errorf("主题应为 dark, got %s", saved.Theme)
	}
}

func TestDeleteWidget(t *testing.T) {
	mgr := NewWidgetManager()

	widget := &Widget{
		Type:   WidgetTypeNotes,
		UserID: "user-1",
	}
	mgr.CreateWidget(widget)

	err := mgr.DeleteWidget(widget.ID)
	if err != nil {
		t.Fatalf("删除小组件失败: %v", err)
	}

	_, err = mgr.GetWidget(widget.ID)
	if err == nil {
		t.Error("删除后不应能获取小组件")
	}
}
