package sysbackup

import (
\t"sync"
)

// 系统备份模块 - 配置备份与恢复
type Manager struct {
\tmu      sync.RWMutex
\tconfig  *Config
\trunning bool
}

// Config 配置
type Config struct {
\tEnabled     bool   `json:"enabled"`
\tInterval    int    `json:"interval"`
}

// NewManager 创建管理器
func NewManager(cfg *Config) *Manager {
\treturn &Manager{
\t\tconfig: cfg,
\t}
}

// Start 启动
func (m *Manager) Start() error {
\tm.mu.Lock()
\tdefer m.mu.Unlock()
\tif m.running {
\t\treturn nil
\t}
\tm.running = true
\treturn nil
}

// Stop 停止
func (m *Manager) Stop() error {
\tm.mu.Lock()
\tdefer m.mu.Unlock()
\tm.running = false
\treturn nil
}

// IsRunning 运行状态
func (m *Manager) IsRunning() bool {
\tm.mu.RLock()
\tdefer m.mu.RUnlock()
\treturn m.running
}
