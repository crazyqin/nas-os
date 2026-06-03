package lxccontainer

import (
	"fmt"
	"sync"
)

// TemplateManager 模板管理器.
type TemplateManager struct {
	mu        sync.RWMutex
	templates map[string]*Template
}

// NewTemplateManager 创建模板管理器.
func NewTemplateManager() *TemplateManager {
	tm := &TemplateManager{
		templates: make(map[string]*Template),
	}
	tm.loadDefaults()
	return tm
}

// loadDefaults 加载默认模板.
func (tm *TemplateManager) loadDefaults() {
	defaults := []*Template{
		{
			Name:        "ubuntu-22.04",
			Distro:      "ubuntu",
			Version:     "22.04",
			Description: "Ubuntu 22.04 LTS 基础镜像",
			SizeMB:      256,
			Packages:    []string{"apt", "systemd", "iproute2"},
			Metadata:    map[string]string{"init": "systemd"},
		},
		{
			Name:        "ubuntu-24.04",
			Distro:      "ubuntu",
			Version:     "24.04",
			Description: "Ubuntu 24.04 LTS 基础镜像",
			SizeMB:      280,
			Packages:    []string{"apt", "systemd", "iproute2"},
			Metadata:    map[string]string{"init": "systemd"},
		},
		{
			Name:        "debian-12",
			Distro:      "debian",
			Version:     "12",
			Description: "Debian 12 (Bookworm) 基础镜像",
			SizeMB:      220,
			Packages:    []string{"apt", "systemd", "iproute2"},
			Metadata:    map[string]string{"init": "systemd"},
		},
		{
			Name:        "alpine-3.19",
			Distro:      "alpine",
			Version:     "3.19",
			Description: "Alpine Linux 3.19 轻量镜像",
			SizeMB:      64,
			Packages:    []string{"apk", "openrc"},
			Metadata:    map[string]string{"init": "openrc"},
		},
		{
			Name:        "centos-stream-9",
			Distro:      "centos",
			Version:     "stream-9",
			Description: "CentOS Stream 9 基础镜像",
			SizeMB:      300,
			Packages:    []string{"dnf", "systemd"},
			Metadata:    map[string]string{"init": "systemd"},
		},
	}

	for _, t := range defaults {
		tm.templates[t.Name] = t
	}
}

// Register 注册新模板.
func (tm *TemplateManager) Register(t *Template) error {
	if t.Name == "" {
		return fmt.Errorf("模板名称不能为空")
	}
	if t.Distro == "" {
		return fmt.Errorf("发行版不能为空")
	}
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if _, exists := tm.templates[t.Name]; exists {
		return fmt.Errorf("模板 %s 已存在", t.Name)
	}
	tm.templates[t.Name] = t
	return nil
}

// Get 获取模板.
func (tm *TemplateManager) Get(name string) (*Template, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	t, ok := tm.templates[name]
	if !ok {
		return nil, fmt.Errorf("模板 %s 不存在", name)
	}
	return t, nil
}

// List 列出所有模板.
func (tm *TemplateManager) List() []*Template {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	result := make([]*Template, 0, len(tm.templates))
	for _, t := range tm.templates {
		result = append(result, t)
	}
	return result
}

// ListByDistro 按发行版列出模板.
func (tm *TemplateManager) ListByDistro(distro string) []*Template {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	var result []*Template
	for _, t := range tm.templates {
		if t.Distro == distro {
			result = append(result, t)
		}
	}
	return result
}

// Delete 删除模板.
func (tm *TemplateManager) Delete(name string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if _, exists := tm.templates[name]; !exists {
		return fmt.Errorf("模板 %s 不存在", name)
	}
	delete(tm.templates, name)
	return nil
}

// Exists 检查模板是否存在.
func (tm *TemplateManager) Exists(name string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	_, ok := tm.templates[name]
	return ok
}

// Count 返回模板数量.
func (tm *TemplateManager) Count() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.templates)
}
