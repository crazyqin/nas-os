package sysmonitor

import (
	"sync"
)

// 系统监控模块 - CPU/内存/磁盘实时监控
type Manager struct {
	mu      sync.RWMutex
	config  *Config
	running bool
}

// Config 配置
type Config struct {
	Enabled  bool `json:"enabled"`
	Interval int  `json:"interval"`
}

// NewManager 创建管理器
func NewManager(cfg *Config) *Manager {
	return &Manager{
		config: cfg,
	}
}

// Start 启动
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	m.running = true
	return nil
}

// Stop 停止
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	return nil
}

// IsRunning 运行状态
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}
