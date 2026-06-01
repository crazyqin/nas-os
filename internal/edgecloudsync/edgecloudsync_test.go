package edgecloudsync

import (
	"testing"
)

func TestNew(t *testing.T) {
	cfg := Config{
		ConflictRes:  ConflictNewest,
		MaxQueueSize: 100,
	}
	sync := New(cfg)
	if sync == nil {
		t.Fatal("New 返回 nil")
	}
	if sync.conflictRes != ConflictNewest {
		t.Fatalf("期望冲突解决策略=%s, got %s", ConflictNewest, sync.conflictRes)
	}
}

func TestRegisterNode(t *testing.T) {
	sync := New(Config{})
	node := &EdgeNode{
		ID:   "edge1",
		Name: "边缘节点1",
	}

	err := sync.RegisterNode(node)
	if err != nil {
		t.Fatalf("注册节点失败: %v", err)
	}

	if node.Status != "online" {
		t.Fatalf("期望状态=online, got %s", node.Status)
	}
}

func TestRegisterNodeEmptyID(t *testing.T) {
	sync := New(Config{})
	node := &EdgeNode{Name: "test"}

	err := sync.RegisterNode(node)
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

func TestUnregisterNode(t *testing.T) {
	sync := New(Config{})
	sync.RegisterNode(&EdgeNode{ID: "edge1", Name: "test"})

	err := sync.UnregisterNode("edge1")
	if err != nil {
		t.Fatalf("注销节点失败: %v", err)
	}
}

func TestUnregisterNodeNotFound(t *testing.T) {
	sync := New(Config{})
	err := sync.UnregisterNode("nonexistent")
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

func TestCreateSyncTask(t *testing.T) {
	sync := New(Config{})
	sync.RegisterNode(&EdgeNode{ID: "edge1", Name: "源"})
	sync.RegisterNode(&EdgeNode{ID: "cloud1", Name: "云"})

	task, err := sync.CreateSyncTask("edge1", "cloud1", ModeEdgeToCloud)
	if err != nil {
		t.Fatalf("创建同步任务失败: %v", err)
	}
	if task.Status != "pending" {
		t.Fatalf("期望状态=pending, got %s", task.Status)
	}
}

func TestCreateSyncTaskNodeNotFound(t *testing.T) {
	sync := New(Config{})
	_, err := sync.CreateSyncTask("edge1", "cloud1", ModeEdgeToCloud)
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

func TestQueueOfflineItem(t *testing.T) {
	sync := New(Config{MaxQueueSize: 10})
	item := &SyncItem{
		ID:   "item1",
		Path: "/test/file.txt",
	}

	err := sync.QueueOfflineItem(item)
	if err != nil {
		t.Fatalf("添加离线项失败: %v", err)
	}
	if item.SyncStatus != "queued" {
		t.Fatalf("期望状态=queued, got %s", item.SyncStatus)
	}
}

func TestQueueOfflineItemFull(t *testing.T) {
	sync := New(Config{MaxQueueSize: 1})
	sync.QueueOfflineItem(&SyncItem{ID: "item1"})

	err := sync.QueueOfflineItem(&SyncItem{ID: "item2"})
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

func TestProcessOfflineQueue(t *testing.T) {
	sync := New(Config{})
	sync.QueueOfflineItem(&SyncItem{ID: "item1"})
	sync.QueueOfflineItem(&SyncItem{ID: "item2"})

	processed := sync.ProcessOfflineQueue()
	if processed != 2 {
		t.Fatalf("期望处理 2 项, got %d", processed)
	}
}

func TestResolveConflict(t *testing.T) {
	sync := New(Config{})
	sync.items["item1"] = &SyncItem{
		ID:           "item1",
		ConflictWith: "item2",
	}

	err := sync.ResolveConflict("item1", "local")
	if err != nil {
		t.Fatalf("解决冲突失败: %v", err)
	}
}

func TestGetNodeStatus(t *testing.T) {
	sync := New(Config{})
	sync.RegisterNode(&EdgeNode{ID: "edge1", Name: "test"})

	node, err := sync.GetNodeStatus("edge1")
	if err != nil {
		t.Fatalf("获取节点状态失败: %v", err)
	}
	if node.ID != "edge1" {
		t.Fatalf("期望ID=edge1, got %s", node.ID)
	}
}

func TestGetSyncStats(t *testing.T) {
	sync := New(Config{})
	sync.RegisterNode(&EdgeNode{ID: "edge1", Name: "test"})

	stats := sync.GetSyncStats()
	if stats["total_nodes"] != 1 {
		t.Fatalf("期望 total_nodes=1, got %v", stats["total_nodes"])
	}
}
