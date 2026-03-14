// Package service 提供系统服务管理功能
package service

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// MockBackend 模拟服务后端
type MockBackend struct {
	mu       sync.Mutex
	services map[string]*mockServiceState
}

type mockServiceState struct {
	running bool
	enabled bool
	status  ServiceStatus
}

func NewMockBackend() *MockBackend {
	return &MockBackend{
		services: make(map[string]*mockServiceState),
	}
}

func (m *MockBackend) Start(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.services[name]; !ok {
		m.services[name] = &mockServiceState{}
	}
	m.services[name].running = true
	m.services[name].status.Running = true
	m.services[name].status.StartedAt = time.Now()
	return nil
}

func (m *MockBackend) Stop(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, ok := m.services[name]; ok {
		state.running = false
		state.status.Running = false
	}
	return nil
}

func (m *MockBackend) Restart(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.services[name]; !ok {
		m.services[name] = &mockServiceState{}
	}
	m.services[name].running = true
	m.services[name].status.Running = true
	m.services[name].status.StartedAt = time.Now()
	return nil
}

func (m *MockBackend) Status(name string) (*ServiceStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, ok := m.services[name]; ok {
		status := state.status
		return &status, nil
	}
	return &ServiceStatus{}, nil
}

func (m *MockBackend) Enable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.services[name]; !ok {
		m.services[name] = &mockServiceState{}
	}
	m.services[name].enabled = true
	return nil
}

func (m *MockBackend) Disable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, ok := m.services[name]; ok {
		state.enabled = false
	}
	return nil
}

func (m *MockBackend) IsEnabled(name string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, ok := m.services[name]; ok {
		return state.enabled, nil
	}
	return false, nil
}

func (m *MockBackend) IsRunning(name string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, ok := m.services[name]; ok {
		return state.running, nil
	}
	return false, nil
}

func (m *MockBackend) List() ([]*Service, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	services := make([]*Service, 0, len(m.services))
	for name, state := range m.services {
		services = append(services, &Service{
			Name:    name,
			Type:    "mock",
			Enabled: state.enabled,
			Status:  state.status,
		})
	}
	return services, nil
}

func (m *MockBackend) Get(name string) (*Service, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, ok := m.services[name]; ok {
		return &Service{
			Name:    name,
			Type:    "mock",
			Enabled: state.enabled,
			Status:  state.status,
		}, nil
	}
	return nil, errors.New("service not found")
}

// TestNewManager 测试创建管理器
func TestNewManager(t *testing.T) {
	backend := NewMockBackend()
	manager, err := NewManagerWithBackend(backend)
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	if manager == nil {
		t.Fatal("管理器不应为空")
	}

	// 验证默认服务已加载
	services, err := manager.List()
	if err != nil {
		t.Fatalf("获取服务列表失败: %v", err)
	}

	if len(services) == 0 {
		t.Error("默认服务列表不应为空")
	}

	// 检查是否包含预期的默认服务
	expectedServices := []string{"smbd", "nfs-server", "docker"}
	for _, expected := range expectedServices {
		found := false
		for _, svc := range services {
			if svc.Name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("缺少默认服务: %s", expected)
		}
	}
}

// TestManagerStartStop 测试启动和停止服务
func TestManagerStartStop(t *testing.T) {
	backend := NewMockBackend()
	manager, err := NewManagerWithBackend(backend)
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	// 启动服务
	err = manager.Start("docker")
	if err != nil {
		t.Fatalf("启动 docker 服务失败: %v", err)
	}

	// 检查运行状态
	running, err := manager.IsRunning("docker")
	if err != nil {
		t.Fatalf("检查运行状态失败: %v", err)
	}
	if !running {
		t.Error("docker 服务应该正在运行")
	}

	// 停止服务
	err = manager.Stop("docker")
	if err != nil {
		t.Fatalf("停止 docker 服务失败: %v", err)
	}

	// 再次检查运行状态
	running, err = manager.IsRunning("docker")
	if err != nil {
		t.Fatalf("检查运行状态失败: %v", err)
	}
	if running {
		t.Error("docker 服务应该已停止")
	}
}

// TestManagerEnableDisable 测试启用和禁用服务
func TestManagerEnableDisable(t *testing.T) {
	backend := NewMockBackend()
	manager, err := NewManagerWithBackend(backend)
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	// 启用服务
	err = manager.Enable("smbd")
	if err != nil {
		t.Fatalf("启用 smbd 服务失败: %v", err)
	}

	// 检查启用状态
	enabled, err := manager.IsEnabled("smbd")
	if err != nil {
		t.Fatalf("检查启用状态失败: %v", err)
	}
	if !enabled {
		t.Error("smbd 服务应该已启用")
	}

	// 禁用服务
	err = manager.Disable("smbd")
	if err != nil {
		t.Fatalf("禁用 smbd 服务失败: %v", err)
	}

	// 再次检查启用状态
	enabled, err = manager.IsEnabled("smbd")
	if err != nil {
		t.Fatalf("检查启用状态失败: %v", err)
	}
	if enabled {
		t.Error("smbd 服务应该已禁用")
	}
}

// TestManagerRestart 测试重启服务
func TestManagerRestart(t *testing.T) {
	backend := NewMockBackend()
	manager, err := NewManagerWithBackend(backend)
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	// 重启服务
	err = manager.Restart("nginx")
	if err != nil {
		t.Fatalf("重启 nginx 服务失败: %v", err)
	}

	// 检查运行状态
	running, err := manager.IsRunning("nginx")
	if err != nil {
		t.Fatalf("检查运行状态失败: %v", err)
	}
	if !running {
		t.Error("nginx 服务应该正在运行")
	}
}

// TestManagerRegisterUnregister 测试注册和注销服务
func TestManagerRegisterUnregister(t *testing.T) {
	backend := NewMockBackend()
	manager, err := NewManagerWithBackend(backend)
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	// 注册新服务
	newService := &Service{
		Name:        "test-service",
		Description: "测试服务",
		Type:        "systemd",
	}

	err = manager.Register(newService)
	if err != nil {
		t.Fatalf("注册服务失败: %v", err)
	}

	// 检查服务是否存在
	svc, err := manager.Get("test-service")
	if err != nil {
		t.Fatalf("获取服务失败: %v", err)
	}

	if svc.Name != "test-service" {
		t.Errorf("服务名称不匹配: 期望 test-service, 实际 %s", svc.Name)
	}

	// 尝试重复注册
	err = manager.Register(newService)
	if err == nil {
		t.Error("重复注册应该返回错误")
	}

	// 注销服务
	err = manager.Unregister("test-service")
	if err != nil {
		t.Fatalf("注销服务失败: %v", err)
	}

	// 检查服务是否已删除
	_, err = manager.Get("test-service")
	if err == nil {
		t.Error("服务应该已被删除")
	}
}

// TestManagerStatus 测试获取服务状态
func TestManagerStatus(t *testing.T) {
	backend := NewMockBackend()
	manager, err := NewManagerWithBackend(backend)
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	// 启动服务
	_ = manager.Start("redis")

	// 获取状态
	status, err := manager.Status("redis")
	if err != nil {
		t.Fatalf("获取状态失败: %v", err)
	}

	if !status.Running {
		t.Error("redis 服务应该正在运行")
	}
}

// TestManagerList 测试列出服务
func TestManagerList(t *testing.T) {
	backend := NewMockBackend()
	manager, err := NewManagerWithBackend(backend)
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	// 启用几个服务
	_ = manager.Enable("docker")
	_ = manager.Enable("smbd")
	_ = manager.Start("docker")

	// 获取服务列表
	services, err := manager.List()
	if err != nil {
		t.Fatalf("获取服务列表失败: %v", err)
	}

	if len(services) == 0 {
		t.Error("服务列表不应为空")
	}

	// 验证启用状态已更新
	enabledCount := manager.GetEnabledCount()
	if enabledCount < 2 {
		t.Errorf("至少应该有两个启用的服务, 实际: %d", enabledCount)
	}

	// 注意：GetRunningCount 依赖 Refresh 更新 Status 字段
	// 由于 mock backend 的服务状态是独立的，需要手动刷新
	_ = manager.Refresh("docker")
}

// TestServiceNotFound 测试服务不存在的错误处理
func TestServiceNotFound(t *testing.T) {
	backend := NewMockBackend()
	manager, err := NewManagerWithBackend(backend)
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	// 尝试启动不存在的服务
	err = manager.Start("nonexistent-service")
	if err == nil {
		t.Error("启动不存在的服务应该返回错误")
	}

	// 尝试停止不存在的服务
	err = manager.Stop("nonexistent-service")
	if err == nil {
		t.Error("停止不存在的服务应该返回错误")
	}

	// 尝试获取不存在的服务
	_, err = manager.Get("nonexistent-service")
	if err == nil {
		t.Error("获取不存在的服务应该返回错误")
	}
}

// TestManagerRefresh 测试刷新服务状态
func TestManagerRefresh(t *testing.T) {
	backend := NewMockBackend()
	manager, err := NewManagerWithBackend(backend)
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	// 刷新单个服务
	err = manager.Refresh("docker")
	if err != nil {
		t.Fatalf("刷新服务状态失败: %v", err)
	}

	// 刷新所有服务
	err = manager.RefreshAll()
	if err != nil {
		t.Fatalf("刷新所有服务状态失败: %v", err)
	}
}
