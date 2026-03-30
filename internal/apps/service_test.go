package apps

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"nas-os/pkg/app"
)

// MockContainerManager 模拟容器管理器
type MockContainerManager struct {
	containers map[string]string
}

func NewMockContainerManager() *MockContainerManager {
	return &MockContainerManager{
		containers: make(map[string]string),
	}
}

func (m *MockContainerManager) CreateContainer(ctx context.Context, config *app.ContainerConfig) (string, error) {
	id := "container-" + config.Name
	m.containers[config.Name] = id
	return id, nil
}

func (m *MockContainerManager) StartContainer(ctx context.Context, id string) error {
	return nil
}

func (m *MockContainerManager) StopContainer(ctx context.Context, id string, timeout int) error {
	return nil
}

func (m *MockContainerManager) RemoveContainer(ctx context.Context, id string, force bool) error {
	return nil
}

func (m *MockContainerManager) GetContainerStatus(ctx context.Context, id string) (*app.ContainerStatus, error) {
	return nil, nil
}

func (m *MockContainerManager) ComposeUp(ctx context.Context, composePath string) error {
	return nil
}

func (m *MockContainerManager) ComposeDown(ctx context.Context, composePath string) error {
	return nil
}

func (m *MockContainerManager) ComposePS(ctx context.Context, composePath string) ([]app.ComposeService, error) {
	return nil, nil
}

func TestNewService(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	// 创建测试服务
	cfg := &ServiceConfig{
		DataDir: tmpDir,
		Manager: NewMockContainerManager(),
	}

	service, err := NewService(cfg)
	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}

	// 验证目录创建
	if _, err := os.Stat(filepath.Join(tmpDir, "templates")); os.IsNotExist(err) {
		t.Error("模板目录未创建")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "compose")); os.IsNotExist(err) {
		t.Error("安装目录未创建")
	}

	// 关闭服务
	if err := service.Close(); err != nil {
		t.Errorf("关闭服务失败: %v", err)
	}
}

func TestServiceListTemplates(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &ServiceConfig{
		DataDir: tmpDir,
		Manager: NewMockContainerManager(),
	}

	service, err := NewService(cfg)
	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}
	defer service.Close()

	// 内置模板应被加载
	templates, err := service.ListTemplates("")
	if err != nil {
		t.Errorf("列出模板失败: %v", err)
	}
	if len(templates) < 1 {
		t.Errorf("应至少有1个内置模板，实际: %d", len(templates))
	}
}

func TestServiceCategories(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &ServiceConfig{
		DataDir: tmpDir,
		Manager: NewMockContainerManager(),
	}

	service, err := NewService(cfg)
	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}
	defer service.Close()

	categories := service.GetCategories()
	// 初始应返回预定义分类
	if len(categories) == 0 {
		t.Error("应有预定义分类")
	}
}

func TestServiceListInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &ServiceConfig{
		DataDir: tmpDir,
		Manager: NewMockContainerManager(),
	}

	service, err := NewService(cfg)
	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}
	defer service.Close()

	// 初始应返回空列表
	installed := service.ListInstalled()
	if len(installed) != 0 {
		t.Errorf("初始应无已安装应用，实际: %d", len(installed))
	}
}