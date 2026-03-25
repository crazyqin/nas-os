package integration

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// ========== 快照复制集成测试 ==========

// SnapshotInfo 快照信息.
type SnapshotInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
	Size      int64     `json:"size"`
	Status    string    `json:"status"`
}

// MockSnapshotManager 快照管理器 Mock.
type MockSnapshotManager struct {
	mu        sync.RWMutex
	snapshots map[string]*SnapshotInfo
}

func NewMockSnapshotManager() *MockSnapshotManager {
	return &MockSnapshotManager{
		snapshots: make(map[string]*SnapshotInfo),
	}
}

func (m *MockSnapshotManager) CreateSnapshot(source, name string) (*SnapshotInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("snap-%d", time.Now().UnixNano())
	info := &SnapshotInfo{
		ID:        id,
		Name:      name,
		Source:    source,
		CreatedAt: time.Now(),
		Size:      1024 * 1024,
		Status:    "created",
	}
	m.snapshots[id] = info
	return info, nil
}

func (m *MockSnapshotManager) DeleteSnapshot(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.snapshots[id]; !exists {
		return fmt.Errorf("snapshot not found: %s", id)
	}
	delete(m.snapshots, id)
	return nil
}

func (m *MockSnapshotManager) ListSnapshots() []*SnapshotInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SnapshotInfo, 0, len(m.snapshots))
	for _, s := range m.snapshots {
		result = append(result, s)
	}
	return result
}

// ReplicationTarget 复制目标.
type ReplicationTarget struct {
	NodeID    string `json:"node_id"`
	Address   string `json:"address"`
	Available bool   `json:"available"`
}

// MockReplicationManager 复制管理器 Mock.
type MockReplicationManager struct {
	mu       sync.RWMutex
	targets  map[string]*ReplicationTarget
	replicas map[string][]string // snapshotID -> nodeIDs
}

func NewMockReplicationManager() *MockReplicationManager {
	return &MockReplicationManager{
		targets:  make(map[string]*ReplicationTarget),
		replicas: make(map[string][]string),
	}
}

func (m *MockReplicationManager) AddTarget(nodeID, address string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.targets[nodeID] = &ReplicationTarget{
		NodeID:    nodeID,
		Address:   address,
		Available: true,
	}
}

func (m *MockReplicationManager) ReplicateSnapshot(snapID, targetNodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, exists := m.targets[targetNodeID]
	if !exists || !target.Available {
		return fmt.Errorf("target not available: %s", targetNodeID)
	}

	m.replicas[snapID] = append(m.replicas[snapID], targetNodeID)
	return nil
}

func (m *MockReplicationManager) GetReplicas(snapID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.replicas[snapID]
}

// TestSnapshotCreation 测试快照创建.
func TestSnapshotCreation(t *testing.T) {
	mgr := NewMockSnapshotManager()

	snap, err := mgr.CreateSnapshot("/data/important", "daily-snapshot")
	if err != nil {
		t.Fatalf("failed to create snapshot: %v", err)
	}

	if snap.Status != "created" {
		t.Errorf("expected status 'created', got '%s'", snap.Status)
	}

	if snap.Name != "daily-snapshot" {
		t.Errorf("expected name 'daily-snapshot', got '%s'", snap.Name)
	}

	// 验证快照已存储
	snapshots := mgr.ListSnapshots()
	if len(snapshots) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(snapshots))
	}
}

// TestSnapshotDeletion 测试快照删除.
func TestSnapshotDeletion(t *testing.T) {
	mgr := NewMockSnapshotManager()

	snap, _ := mgr.CreateSnapshot("/data", "test-snap")

	err := mgr.DeleteSnapshot(snap.ID)
	if err != nil {
		t.Errorf("failed to delete snapshot: %v", err)
	}

	snapshots := mgr.ListSnapshots()
	if len(snapshots) != 0 {
		t.Errorf("expected 0 snapshots after deletion, got %d", len(snapshots))
	}

	// 删除不存在的快照应报错
	err = mgr.DeleteSnapshot("nonexistent")
	if err == nil {
		t.Error("expected error when deleting nonexistent snapshot")
	}
}

// TestSnapshotReplication 测试快照复制.
func TestSnapshotReplication(t *testing.T) {
	snapMgr := NewMockSnapshotManager()
	replMgr := NewMockReplicationManager()

	// 添加复制目标
	replMgr.AddTarget("node-1", "192.168.1.2:8080")
	replMgr.AddTarget("node-2", "192.168.1.3:8080")

	// 创建快照
	snap, err := snapMgr.CreateSnapshot("/data", "replicated-snap")
	if err != nil {
		t.Fatalf("failed to create snapshot: %v", err)
	}

	// 复制到目标节点
	err = replMgr.ReplicateSnapshot(snap.ID, "node-1")
	if err != nil {
		t.Errorf("failed to replicate to node-1: %v", err)
	}

	err = replMgr.ReplicateSnapshot(snap.ID, "node-2")
	if err != nil {
		t.Errorf("failed to replicate to node-2: %v", err)
	}

	// 验证复制状态
	replicas := replMgr.GetReplicas(snap.ID)
	if len(replicas) != 2 {
		t.Errorf("expected 2 replicas, got %d", len(replicas))
	}
}

// TestSnapshotReplicationFailure 测试复制失败.
func TestSnapshotReplicationFailure(t *testing.T) {
	replMgr := NewMockReplicationManager()

	// 复制到不存在的目标应失败
	err := replMgr.ReplicateSnapshot("snap-1", "nonexistent-node")
	if err == nil {
		t.Error("expected error when replicating to nonexistent target")
	}
}

// TestConcurrentSnapshotCreation 测试并发快照创建.
func TestConcurrentSnapshotCreation(t *testing.T) {
	mgr := NewMockSnapshotManager()

	var wg sync.WaitGroup
	errors := make([]error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errors[idx] = mgr.CreateSnapshot("/data", fmt.Sprintf("concurrent-snap-%d", idx))
		}(i)
	}

	wg.Wait()

	for i, err := range errors {
		if err != nil {
			t.Errorf("concurrent snapshot %d failed: %v", i, err)
		}
	}

	if len(mgr.ListSnapshots()) != 10 {
		t.Errorf("expected 10 snapshots, got %d", len(mgr.ListSnapshots()))
	}
}

// TestSnapshotRetention 测试快照保留策略.
func TestSnapshotRetention(t *testing.T) {
	mgr := NewMockSnapshotManager()

	// 创建多个快照
	for i := 0; i < 10; i++ {
		mgr.CreateSnapshot("/data", fmt.Sprintf("retention-snap-%d", i))
	}

	// 验证创建了 10 个
	if len(mgr.ListSnapshots()) != 10 {
		t.Errorf("expected 10 snapshots, got %d", len(mgr.ListSnapshots()))
	}

	// 模拟保留策略：删除旧的快照
	snapshots := mgr.ListSnapshots()
	for i := 0; i < len(snapshots)-5; i++ {
		mgr.DeleteSnapshot(snapshots[i].ID)
	}

	// 验证只剩 5 个
	if len(mgr.ListSnapshots()) != 5 {
		t.Errorf("expected 5 snapshots after retention, got %d", len(mgr.ListSnapshots()))
	}
}

// BenchmarkSnapshotCreation 快照创建基准测试.
func BenchmarkSnapshotCreation(b *testing.B) {
	mgr := NewMockSnapshotManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.CreateSnapshot("/data", fmt.Sprintf("bench-snap-%d", i))
	}
}

// BenchmarkSnapshotReplication 快照复制基准测试.
func BenchmarkSnapshotReplication(b *testing.B) {
	snapMgr := NewMockSnapshotManager()
	replMgr := NewMockReplicationManager()
	replMgr.AddTarget("node-1", "192.168.1.2:8080")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		snap, _ := snapMgr.CreateSnapshot("/data", fmt.Sprintf("bench-snap-%d", i))
		replMgr.ReplicateSnapshot(snap.ID, "node-1")
	}
}
