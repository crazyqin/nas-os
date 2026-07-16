// Package containerfailover 容器 HA 故障转移模块
package containerfailover

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// StateBackend 状态同步后端接口.
type StateBackend interface {
	// Save 保存状态数据
	Save(key string, value []byte) error
	// Load 读取状态数据
	Load(key string) ([]byte, error)
	// Delete 删除状态数据
	Delete(key string) error
	// List 列出指定前缀下所有 key
	List(prefix string) ([]string, error)
}

// ========== etcd 模拟后端 ==========

// EtcdSimBackend etcd 模拟后端
// 使用内存 map 模拟 etcd KV 存储，用于测试和单机部署场景。
// 生产环境可替换为真实 etcd 客户端实现。
type EtcdSimBackend struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// NewEtcdSimBackend 创建 etcd 模拟后端.
func NewEtcdSimBackend() *EtcdSimBackend {
	return &EtcdSimBackend{
		data: make(map[string][]byte),
	}
}

// Save 保存状态数据.
func (e *EtcdSimBackend) Save(key string, value []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	cp := make([]byte, len(value))
	copy(cp, value)
	e.data[key] = cp
	return nil
}

// Load 读取状态数据.
func (e *EtcdSimBackend) Load(key string) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	val, exists := e.data[key]
	if !exists {
		return nil, fmt.Errorf("key %s 不存在", key)
	}
	cp := make([]byte, len(val))
	copy(cp, val)
	return cp, nil
}

// Delete 删除状态数据.
func (e *EtcdSimBackend) Delete(key string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.data, key)
	return nil
}

// List 列出指定前缀下所有 key.
func (e *EtcdSimBackend) List(prefix string) ([]string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	keys := make([]string, 0)
	for k := range e.data {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

// ========== 状态同步管理器 ==========

// StateSync 状态同步管理器
// 在 HA 集群节点间同步容器状态、IP 分配和故障转移事件。
type StateSync struct {
	mu      sync.RWMutex
	backend StateBackend
	// localNode 本节点 ID
	localNode string
	// syncInterval 同步间隔
	syncInterval time.Duration
	// stopCh 停止通道
	stopCh chan struct{}
	// running 是否正在运行
	running bool
}

// NewStateSync 创建状态同步管理器.
func NewStateSync(backend StateBackend, localNode string) *StateSync {
	return &StateSync{
		backend:      backend,
		localNode:    localNode,
		syncInterval: 10 * time.Second,
		stopCh:       make(chan struct{}),
	}
}

// SetSyncInterval 设置同步间隔.
func (s *StateSync) SetSyncInterval(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncInterval = d
}

// Start 启动周期性状态同步.
func (s *StateSync) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return fmt.Errorf("状态同步已在运行")
	}
	s.running = true
	s.stopCh = make(chan struct{})
	go s.run()
	return nil
}

// Stop 停止状态同步.
func (s *StateSync) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return fmt.Errorf("状态同步未运行")
	}
	close(s.stopCh)
	s.running = false
	return nil
}

// IsRunning 是否正在同步.
func (s *StateSync) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// run 同步循环.
func (s *StateSync) run() {
	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			// 同步循环由 Manager 驱动，这里仅保持心跳
			_ = s.heartbeat()
		}
	}
}

// heartbeat 写入节点心跳到后端.
func (s *StateSync) heartbeat() error {
	key := fmt.Sprintf("/containerfailover/nodes/%s/heartbeat", s.localNode)
	val, _ := json.Marshal(map[string]interface{}{
		"node":      s.localNode,
		"timestamp": time.Now().Unix(),
	})
	return s.backend.Save(key, val)
}

// SyncContainer 同步容器状态到后端.
func (s *StateSync) SyncContainer(c *Container) error {
	key := fmt.Sprintf("/containerfailover/containers/%s", c.ID)
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("序列化容器状态失败: %w", err)
	}
	return s.backend.Save(key, data)
}

// LoadContainer 从后端加载容器状态.
func (s *StateSync) LoadContainer(containerID string) (*Container, error) {
	key := fmt.Sprintf("/containerfailover/containers/%s", containerID)
	data, err := s.backend.Load(key)
	if err != nil {
		return nil, err
	}
	var c Container
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("反序列化容器状态失败: %w", err)
	}
	return &c, nil
}

// DeleteContainer 从后端删除容器状态.
func (s *StateSync) DeleteContainer(containerID string) error {
	key := fmt.Sprintf("/containerfailover/containers/%s", containerID)
	return s.backend.Delete(key)
}

// ListContainers 列出后端中所有容器 ID.
func (s *StateSync) ListContainers() ([]string, error) {
	keys, err := s.backend.List("/containerfailover/containers/")
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(keys))
	prefix := "/containerfailover/containers/"
	for _, k := range keys {
		id := k[len(prefix):]
		ids = append(ids, id)
	}
	return ids, nil
}

// SyncNode 同步节点状态到后端.
func (s *StateSync) SyncNode(n *ClusterNode) error {
	key := fmt.Sprintf("/containerfailover/nodes/%s", n.ID)
	data, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("序列化节点状态失败: %w", err)
	}
	return s.backend.Save(key, data)
}

// LoadNode 从后端加载节点状态.
func (s *StateSync) LoadNode(nodeID string) (*ClusterNode, error) {
	key := fmt.Sprintf("/containerfailover/nodes/%s", nodeID)
	data, err := s.backend.Load(key)
	if err != nil {
		return nil, err
	}
	var n ClusterNode
	if err := json.Unmarshal(data, &n); err != nil {
		return nil, fmt.Errorf("反序列化节点状态失败: %w", err)
	}
	return &n, nil
}

// SyncIPAllocation 同步 IP 分配记录到后端.
func (s *StateSync) SyncIPAllocation(alloc *IPAllocation) error {
	key := fmt.Sprintf("/containerfailover/ip/%s", alloc.IP)
	data, err := json.Marshal(alloc)
	if err != nil {
		return fmt.Errorf("序列化 IP 分配记录失败: %w", err)
	}
	return s.backend.Save(key, data)
}

// LoadIPAllocation 从后端加载 IP 分配记录.
func (s *StateSync) LoadIPAllocation(ip string) (*IPAllocation, error) {
	key := fmt.Sprintf("/containerfailover/ip/%s", ip)
	data, err := s.backend.Load(key)
	if err != nil {
		return nil, err
	}
	var alloc IPAllocation
	if err := json.Unmarshal(data, &alloc); err != nil {
		return nil, fmt.Errorf("反序列化 IP 分配记录失败: %w", err)
	}
	return &alloc, nil
}

// SyncFailoverEvent 同步故障转移事件到后端.
func (s *StateSync) SyncFailoverEvent(event *FailoverEvent) error {
	key := fmt.Sprintf("/containerfailover/events/%s", event.ID)
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化故障转移事件失败: %w", err)
	}
	return s.backend.Save(key, data)
}

// ListFailoverEvents 列出后端中所有故障转移事件.
func (s *StateSync) ListFailoverEvents() ([]*FailoverEvent, error) {
	keys, err := s.backend.List("/containerfailover/events/")
	if err != nil {
		return nil, err
	}
	events := make([]*FailoverEvent, 0, len(keys))
	prefix := "/containerfailover/events/"
	for _, k := range keys {
		data, err := s.backend.Load(k)
		if err != nil {
			continue
		}
		var ev FailoverEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			continue
		}
		events = append(events, &ev)
	}
	_ = prefix
	return events, nil
}

// SyncAll 全量同步容器列表.
func (s *StateSync) SyncAll(containers []*Container) error {
	for _, c := range containers {
		if err := s.SyncContainer(c); err != nil {
			return err
		}
	}
	return nil
}
