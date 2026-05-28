package filewatcher

import (
	"testing"
)

func TestManager_CreateWatcher(t *testing.T) {
	m := NewManager()

	req := CreateWatcherRequest{
		Name:      "文档监控",
		Paths:     []string{"/data/docs"},
		Events:    []EventType{EventCreate, EventModify},
		Recursive: true,
	}

	watcher, err := m.CreateWatcher(req)
	if err != nil {
		t.Fatalf("创建监控器失败: %v", err)
	}
	if watcher.Name != "文档监控" {
		t.Errorf("期望名称 '文档监控', 得到 '%s'", watcher.Name)
	}
	if !watcher.Recursive {
		t.Error("应该递归监控")
	}
}

func TestManager_CreateWatcher_DefaultEvents(t *testing.T) {
	m := NewManager()

	req := CreateWatcherRequest{
		Name:  "默认事件",
		Paths: []string{"/data"},
	}

	watcher, err := m.CreateWatcher(req)
	if err != nil {
		t.Fatalf("创建监控器失败: %v", err)
	}
	if len(watcher.Events) != 3 {
		t.Errorf("期望3个默认事件, 得到 %d", len(watcher.Events))
	}
}

func TestManager_GetWatcher_NotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetWatcher("nonexistent")
	if err != ErrWatcherNotFound {
		t.Errorf("期望 ErrWatcherNotFound, 得到 %v", err)
	}
}

func TestManager_ListWatchers(t *testing.T) {
	m := NewManager()

	m.CreateWatcher(CreateWatcherRequest{Name: "w1", Paths: []string{"/a"}})
	m.CreateWatcher(CreateWatcherRequest{Name: "w2", Paths: []string{"/b"}})

	watchers := m.ListWatchers()
	if len(watchers) != 2 {
		t.Errorf("期望2个监控器, 得到 %d", len(watchers))
	}
}

func TestManager_DeleteWatcher(t *testing.T) {
	m := NewManager()

	watcher, _ := m.CreateWatcher(CreateWatcherRequest{Name: "删除测试", Paths: []string{"/data"}})

	if err := m.DeleteWatcher(watcher.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
}

func TestManager_RecordEvent(t *testing.T) {
	m := NewManager()

	watcher, _ := m.CreateWatcher(CreateWatcherRequest{Name: "事件测试", Paths: []string{"/data"}})

	event := m.RecordEvent(watcher.ID, EventCreate, "/data/test.txt")
	if event.Type != EventCreate {
		t.Errorf("期望 create, 得到 %s", event.Type)
	}
	if event.Path != "/data/test.txt" {
		t.Errorf("路径不匹配: %s", event.Path)
	}
}

func TestManager_GetEvents(t *testing.T) {
	m := NewManager()

	watcher, _ := m.CreateWatcher(CreateWatcherRequest{Name: "获取事件", Paths: []string{"/data"}})
	m.RecordEvent(watcher.ID, EventCreate, "/a.txt")
	m.RecordEvent(watcher.ID, EventModify, "/b.txt")

	events := m.GetEvents(watcher.ID, 0)
	if len(events) != 2 {
		t.Errorf("期望2个事件, 得到 %d", len(events))
	}

	events = m.GetEvents(watcher.ID, 1)
	if len(events) != 1 {
		t.Errorf("期望1个事件(limit), 得到 %d", len(events))
	}
}

func TestManager_GetStats(t *testing.T) {
	m := NewManager()

	m.CreateWatcher(CreateWatcherRequest{Name: "统计测试", Paths: []string{"/data"}})

	stats := m.GetStats()
	if stats.TotalWatchers != 1 {
		t.Errorf("期望1个监控器, 得到 %d", stats.TotalWatchers)
	}
}
