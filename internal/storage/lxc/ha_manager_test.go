// Package lxcstorage 测试 LXC HA 管理
package lxcstorage

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockLXCManager 模拟 LXC 管理器
type MockLXCManager struct {
	containers map[string]string // containerID -> status
	migrations []MigrationRecord
}

type MigrationRecord struct {
	ContainerID string
	TargetNode  string
}

func NewMockLXCManager() *MockLXCManager {
	return &MockLXCManager{
		containers: make(map[string]string),
		migrations: make([]MigrationRecord, 0),
	}
}

func (m *MockLXCManager) MigrateContainer(ctx context.Context, containerID, targetNode string) error {
	m.migrations = append(m.migrations, MigrationRecord{
		ContainerID: containerID,
		TargetNode:  targetNode,
	})
	m.containers[containerID] = "Running"
	return nil
}

func (m *MockLXCManager) StartContainer(ctx context.Context, containerID string) error {
	m.containers[containerID] = "Running"
	return nil
}

func (m *MockLXCManager) StopContainer(ctx context.Context, containerID string, force bool) error {
	m.containers[containerID] = "Stopped"
	return nil
}

func (m *MockLXCManager) GetContainerStatus(ctx context.Context, containerID string) (string, error) {
	status, exists := m.containers[containerID]
	if !exists {
		return "", ErrContainerNotFound
	}
	return status, nil
}

func (m *MockLXCManager) ListContainers(ctx context.Context) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0)
	for id, status := range m.containers {
		result = append(result, map[string]interface{}{
			"id":     id,
			"status": status,
		})
	}
	return result, nil
}

// ========== HA 管理器测试 ==========

func TestNewHAManager(t *testing.T) {
	tempDir := t.TempDir()
	mockLXC := NewMockLXCManager()

	manager, err := NewHAManager(tempDir, mockLXC)
	require.NoError(t, err)
	require.NotNil(t, manager)

	assert.NotNil(t, manager.containers)
	assert.NotNil(t, manager.nodes)
	assert.NotNil(t, manager.clusterConfig)
	assert.Equal(t, "nas-os-cluster", manager.clusterConfig.Name)
}

func TestHAManager_RegisterNode(t *testing.T) {
	tempDir := t.TempDir()
	mockLXC := NewMockLXCManager()
	manager, err := NewHAManager(tempDir, mockLXC)
	require.NoError(t, err)

	node := &HANode{
		ID:          "node-1",
		Name:        "Primary Node",
		Address:     "192.168.1.100",
		Port:        8443,
		State:       HAStateActive,
		Priority:    100,
		StoragePools: []string{"zfs-pool", "btrfs-pool"},
	}

	err = manager.RegisterNode(node)
	require.NoError(t, err)

	// 验证节点已注册
	registeredNode, err := manager.GetNode("node-1")
	require.NoError(t, err)
	assert.Equal(t, "node-1", registeredNode.ID)
	assert.True(t, registeredNode.Online)

	// 列出节点
	nodes := manager.ListNodes()
	assert.Len(t, nodes, 1)
}

func TestHAManager_EnableHA(t *testing.T) {
	tempDir := t.TempDir()
	mockLXC := NewMockLXCManager()
	manager, err := NewHAManager(tempDir, mockLXC)
	require.NoError(t, err)

	// 先注册节点
	node1 := &HANode{
		ID:           "node-1",
		Name:         "Primary",
		Address:      "192.168.1.100",
		Port:         8443,
		Priority:     100,
		StoragePools: []string{"zfs-pool"},
	}
	node2 := &HANode{
		ID:           "node-2",
		Name:         "Standby",
		Address:      "192.168.1.101",
		Port:         8443,
		Priority:     50,
		StoragePools: []string{"zfs-pool"},
	}

	require.NoError(t, manager.RegisterNode(node1))
	require.NoError(t, manager.RegisterNode(node2))

	// 启用 HA
	config := &HAContainerConfig{
		Name:         "test-container",
		PrimaryNode:  "node-1",
		StandbyNodes: []string{"node-2"},
		Mode:         HAModeActivePassive,
		Policy:       FailoverPolicyAuto,
		Priority:     100,
		HealthCheck: HealthCheckConfig{
			Enabled:  true,
			Interval: 10 * time.Second,
			Timeout:  5 * time.Second,
		},
		StorageVolumes: []StorageVolumeRef{
			{
				PoolName:   "zfs-pool",
				VolumeName: "vol1",
				MountPath:  "/data",
			},
		},
	}

	err = manager.EnableHA(context.Background(), "container-1", config)
	require.NoError(t, err)

	// 验证 HA 已启用
	haContainer, err := manager.GetHAContainer("container-1")
	require.NoError(t, err)
	assert.Equal(t, "container-1", haContainer.ID)
	assert.Equal(t, HAStateActive, haContainer.State)
	assert.True(t, haContainer.Enabled)

	// 列出 HA 容器
	containers := manager.ListHAContainers()
	assert.Len(t, containers, 1)
}

func TestHAManager_EnableHA_InvalidNode(t *testing.T) {
	tempDir := t.TempDir()
	mockLXC := NewMockLXCManager()
	manager, err := NewHAManager(tempDir, mockLXC)
	require.NoError(t, err)

	config := &HAContainerConfig{
		Name:         "test-container",
		PrimaryNode:  "node-1",
		StandbyNodes: []string{"node-nonexistent"},
	}

	err = manager.EnableHA(context.Background(), "container-1", config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node not found")
}

func TestHAManager_EnableHA_MissingStoragePool(t *testing.T) {
	tempDir := t.TempDir()
	mockLXC := NewMockLXCManager()
	manager, err := NewHAManager(tempDir, mockLXC)
	require.NoError(t, err)

	node1 := &HANode{
		ID:           "node-1",
		Name:         "Primary",
		StoragePools: []string{"zfs-pool"},
	}
	node2 := &HANode{
		ID:           "node-2",
		Name:         "Standby",
		StoragePools: []string{"other-pool"}, // 没有 zfs-pool
	}

	require.NoError(t, manager.RegisterNode(node1))
	require.NoError(t, manager.RegisterNode(node2))

	config := &HAContainerConfig{
		Name:         "test-container",
		PrimaryNode:  "node-1",
		StandbyNodes: []string{"node-2"},
		StorageVolumes: []StorageVolumeRef{
			{
				PoolName:   "zfs-pool",
				VolumeName: "vol1",
				Shared:     false, // 需要所有节点都有这个存储池
			},
		},
	}

	err = manager.EnableHA(context.Background(), "container-1", config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not have storage pool")
}

func TestHAManager_Failover(t *testing.T) {
	tempDir := t.TempDir()
	mockLXC := NewMockLXCManager()
	manager, err := NewHAManager(tempDir, mockLXC)
	require.NoError(t, err)

	// 注册节点
	node1 := &HANode{
		ID:           "node-1",
		Name:         "Primary",
		Address:      "192.168.1.100",
		StoragePools: []string{"zfs-pool"},
		Priority:     100,
	}
	node2 := &HANode{
		ID:           "node-2",
		Name:         "Standby",
		Address:      "192.168.1.101",
		StoragePools: []string{"zfs-pool"},
		Priority:     50,
	}

	require.NoError(t, manager.RegisterNode(node1))
	require.NoError(t, manager.RegisterNode(node2))

	// 启用 HA
	config := &HAContainerConfig{
		Name:         "test-container",
		PrimaryNode:  "node-1",
		StandbyNodes: []string{"node-2"},
		Mode:         HAModeActivePassive,
		Policy:       FailoverPolicyManual,
		StorageVolumes: []StorageVolumeRef{
			{
				PoolName: "zfs-pool",
				Shared:   true,
			},
		},
	}
	require.NoError(t, manager.EnableHA(context.Background(), "container-1", config))

	// 执行故障转移
	result, err := manager.Failover(context.Background(), "container-1", "node-2", false)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "node-1", result.OldPrimary)
	assert.Equal(t, "node-2", result.NewPrimary)

	// 验证状态已更新
	haContainer, err := manager.GetHAContainer("container-1")
	require.NoError(t, err)
	assert.Equal(t, "node-2", haContainer.PrimaryNode)
	assert.Contains(t, haContainer.StandbyNodes, "node-1")
	assert.Equal(t, 1, haContainer.FailoverCount)

	// 验证迁移已执行
	assert.Len(t, mockLXC.migrations, 1)
	assert.Equal(t, "container-1", mockLXC.migrations[0].ContainerID)
	assert.Equal(t, "node-2", mockLXC.migrations[0].TargetNode)
}

func TestHAManager_Failover_ContainerNotFound(t *testing.T) {
	tempDir := t.TempDir()
	mockLXC := NewMockLXCManager()
	manager, err := NewHAManager(tempDir, mockLXC)
	require.NoError(t, err)

	result, err := manager.Failover(context.Background(), "nonexistent", "node-2", false)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, ErrContainerNotFound, err)
}

func TestHAManager_Failover_TargetOffline(t *testing.T) {
	tempDir := t.TempDir()
	mockLXC := NewMockLXCManager()
	manager, err := NewHAManager(tempDir, mockLXC)
	require.NoError(t, err)

	node1 := &HANode{ID: "node-1", Name: "Primary", StoragePools: []string{"zfs-pool"}}
	node2 := &HANode{ID: "node-2", Name: "Standby", StoragePools: []string{"zfs-pool"}, Online: false}

	require.NoError(t, manager.RegisterNode(node1))
	manager.RegisterNode(node2)
	manager.nodes["node-2"].Online = false // 强制设置为离线

	config := &HAContainerConfig{
		Name:         "test",
		PrimaryNode:  "node-1",
		StandbyNodes: []string{"node-2"},
	}
	require.NoError(t, manager.EnableHA(context.Background(), "container-1", config))

	result, err := manager.Failover(context.Background(), "container-1", "node-2", false)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "offline")
}

func TestHAManager_AutoFailover(t *testing.T) {
	tempDir := t.TempDir()
	mockLXC := NewMockLXCManager()
	manager, err := NewHAManager(tempDir, mockLXC)
	require.NoError(t, err)

	// 注册多个节点
	node1 := &HANode{ID: "node-1", Name: "Primary", StoragePools: []string{"zfs-pool"}, Priority: 100}
	node2 := &HANode{ID: "node-2", Name: "Standby1", StoragePools: []string{"zfs-pool"}, Priority: 50}
	node3 := &HANode{ID: "node-3", Name: "Standby2", StoragePools: []string{"zfs-pool"}, Priority: 80}

	require.NoError(t, manager.RegisterNode(node1))
	require.NoError(t, manager.RegisterNode(node2))
	require.NoError(t, manager.RegisterNode(node3))

	// 启用多个容器的 HA
	config1 := &HAContainerConfig{
		Name:         "container-1",
		PrimaryNode:  "node-1",
		StandbyNodes: []string{"node-2", "node-3"},
		Policy:       FailoverPolicyAuto,
	}
	config2 := &HAContainerConfig{
		Name:         "container-2",
		PrimaryNode:  "node-1",
		StandbyNodes: []string{"node-2", "node-3"},
		Policy:       FailoverPolicyAuto,
	}

	require.NoError(t, manager.EnableHA(context.Background(), "container-1", config1))
	require.NoError(t, manager.EnableHA(context.Background(), "container-2", config2))

	// 执行自动故障转移（node-1 失败）
	results, err := manager.AutoFailover(context.Background(), "node-1")
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// 验证都转移到优先级最高的可用节点（node-3）
	for _, result := range results {
		assert.True(t, result.Success)
		assert.Equal(t, "node-3", result.NewPrimary) // node-3 优先级更高
	}
}

func TestHAManager_DisableHA(t *testing.T) {
	tempDir := t.TempDir()
	mockLXC := NewMockLXCManager()
	manager, err := NewHAManager(tempDir, mockLXC)
	require.NoError(t, err)

	node1 := &HANode{ID: "node-1", Name: "Primary", StoragePools: []string{"zfs-pool"}}
	node2 := &HANode{ID: "node-2", Name: "Standby", StoragePools: []string{"zfs-pool"}}

	require.NoError(t, manager.RegisterNode(node1))
	require.NoError(t, manager.RegisterNode(node2))

	config := &HAContainerConfig{
		Name:         "test",
		PrimaryNode:  "node-1",
		StandbyNodes: []string{"node-2"},
	}
	require.NoError(t, manager.EnableHA(context.Background(), "container-1", config))

	// 禁用 HA
	err = manager.DisableHA(context.Background(), "container-1")
	require.NoError(t, err)

	haContainer, err := manager.GetHAContainer("container-1")
	require.NoError(t, err)
	assert.False(t, haContainer.Enabled)
}

func TestHAManager_CheckNodeHealth(t *testing.T) {
	tempDir := t.TempDir()
	mockLXC := NewMockLXCManager()
	manager, err := NewHAManager(tempDir, mockLXC)
	require.NoError(t, err)

	node1 := &HANode{
		ID:           "node-1",
		Name:         "Primary",
		StoragePools: []string{"zfs-pool"},
	}
	node2 := &HANode{
		ID:           "node-2",
		Name:         "Standby",
		StoragePools: []string{"zfs-pool"},
	}

	require.NoError(t, manager.RegisterNode(node1))
	require.NoError(t, manager.RegisterNode(node2))

	// 初始状态都健康
	health := manager.CheckNodeHealth()
	assert.True(t, health["node-1"])
	assert.True(t, health["node-2"])

	// 模拟 node-2 心跳超时
	manager.nodes["node-2"].LastHeartbeat = time.Now().Add(-30 * time.Second)
	health = manager.CheckNodeHealth()
	assert.True(t, health["node-1"])
	assert.False(t, health["node-2"])
}

func TestHAContainerConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  HAContainerConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: HAContainerConfig{
				PrimaryNode:  "node-1",
				StandbyNodes: []string{"node-2"},
			},
			wantErr: false,
		},
		{
			name: "missing primary",
			config: HAContainerConfig{
				StandbyNodes: []string{"node-2"},
			},
			wantErr: true,
		},
		{
			name: "missing standby",
			config: HAContainerConfig{
				PrimaryNode: "node-1",
			},
			wantErr: true,
		},
		{
			name: "primary in standby list",
			config: HAContainerConfig{
				PrimaryNode:  "node-1",
				StandbyNodes: []string{"node-1", "node-2"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHAManager_StatePersistence(t *testing.T) {
	tempDir := t.TempDir()
	mockLXC := NewMockLXCManager()

	// 创建第一个管理器并添加数据
	manager1, err := NewHAManager(tempDir, mockLXC)
	require.NoError(t, err)

	node := &HANode{ID: "node-1", Name: "Primary", StoragePools: []string{"zfs-pool"}}
	require.NoError(t, manager1.RegisterNode(node))

	config := &HAContainerConfig{
		Name:         "test",
		PrimaryNode:  "node-1",
		StandbyNodes: []string{"node-2"},
	}
	require.NoError(t, manager1.EnableHA(context.Background(), "container-1", config))

	// 创建第二个管理器，验证数据已持久化
	manager2, err := NewHAManager(tempDir, mockLXC)
	require.NoError(t, err)

	// 验证节点已加载
	loadedNode, err := manager2.GetNode("node-1")
	require.NoError(t, err)
	assert.Equal(t, "node-1", loadedNode.ID)

	// 验证容器已加载
	loadedContainer, err := manager2.GetHAContainer("container-1")
	require.NoError(t, err)
	assert.Equal(t, "container-1", loadedContainer.ID)
}

func TestHAManager_Events(t *testing.T) {
	tempDir := t.TempDir()
	mockLXC := NewMockLXCManager()
	manager, err := NewHAManager(tempDir, mockLXC)
	require.NoError(t, err)

	// 订阅事件
	eventChan := manager.SubscribeEvents()

	node := &HANode{ID: "node-1", Name: "Primary", StoragePools: []string{"zfs-pool"}}
	require.NoError(t, manager.RegisterNode(node))

	config := &HAContainerConfig{
		Name:         "test",
		PrimaryNode:  "node-1",
		StandbyNodes: []string{"node-2"},
	}
	require.NoError(t, manager.EnableHA(context.Background(), "container-1", config))

	// 验证事件已发送
	select {
	case event := <-eventChan:
		assert.NotEmpty(t, event.Type)
		assert.NotEmpty(t, event.Message)
	case <-time.After(1 * time.Second):
		t.Fatal("Expected event not received")
	}
}