package domainsync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Manager 域同步管理器.
type Manager struct {
	mu         sync.RWMutex
	config     SyncConfig
	configPath string
	engine     *SyncEngine
}

// NewManager 创建域同步管理器.
func NewManager() *Manager {
	return &Manager{
		config: DefaultSyncConfig(),
	}
}

// NewManagerWithConfig 创建带配置文件的管理器.
func NewManagerWithConfig(configPath string) (*Manager, error) {
	m := NewManager()
	m.configPath = configPath

	if err := m.loadConfig(); err != nil {
		return nil, fmt.Errorf("加载域同步配置失败: %w", err)
	}

	return m, nil
}

// loadConfig 从文件加载配置.
func (m *Manager) loadConfig() error {
	if m.configPath == "" {
		return nil
	}

	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config SyncConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	m.config = config
	return nil
}

// saveConfig 保存配置到文件.
func (m *Manager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0750); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0600)
}

// ========== 配置管理 ==========

// GetConfig 获取当前配置.
func (m *Manager) GetConfig() SyncConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新同步配置.
func (m *Manager) UpdateConfig(config SyncConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 基本验证
	if config.DCConfig.Host == "" {
		return fmt.Errorf("%w: 域控制器地址不能为空", ErrInvalidConfig)
	}
	if config.DCConfig.Domain == "" && config.DCConfig.BaseDN == "" {
		return fmt.Errorf("%w: 域名和基础 DN 不能同时为空", ErrInvalidConfig)
	}

	m.config = config

	// 如果引擎正在运行，更新引擎配置
	if m.engine != nil {
		m.engine.UpdateConfig(config)
	}

	return m.saveConfig()
}

// ========== OU 管理 ==========

// ListOUs 列出所有 OU.
func (m *Manager) ListOUs() ([]*OU, error) {
	m.mu.RLock()
	config := m.config.DCConfig
	m.mu.RUnlock()

	discoverer := NewOUDiscoverer(config)
	return discoverer.Discover()
}

// ========== 同步操作 ==========

// StartSync 手动触发同步.
func (m *Manager) StartSync(ctx context.Context) (*SyncResult, error) {
	m.mu.Lock()
	engine := m.engine
	if engine == nil {
		engine = NewSyncEngine(m.config)
		m.engine = engine
	}
	m.mu.Unlock()

	return engine.SyncOnce(ctx)
}

// StartScheduled 启动定时同步.
func (m *Manager) StartScheduled(ctx context.Context) error {
	m.mu.Lock()
	engine := m.engine
	if engine == nil {
		engine = NewSyncEngine(m.config)
		m.engine = engine
	}
	m.mu.Unlock()

	return engine.StartScheduled(ctx)
}

// Stop 停止同步.
func (m *Manager) Stop() {
	m.mu.RLock()
	engine := m.engine
	m.mu.RUnlock()

	if engine != nil {
		engine.Stop()
	}
}

// ========== 状态查询 ==========

// GetStatus 获取同步状态.
func (m *Manager) GetStatus() *DomainSyncStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &DomainSyncStatus{
		Enabled:       m.config.DCConfig.Host != "",
		Strategy:      m.config.Strategy,
		OUCount:       len(m.config.SelectedOUs),
		SelectedCount: len(m.config.SelectedOUs),
		ScheduleCron:  m.config.ScheduleCron,
	}

	if m.engine != nil {
		status.Status = m.engine.GetStatus()
		lastResult := m.engine.GetLastResult()
		if lastResult != nil {
			status.LastResult = lastResult
			status.LastSyncID = lastResult.ID
			status.LastSyncTime = &lastResult.StartTime
		}
	} else {
		status.Status = SyncStatusIdle
	}

	// 测试 DC 连接状态
	dc := m.config.DCConfig
	if dc.Host != "" {
		discoverer := NewOUDiscoverer(dc)
		_, err := discoverer.Discover()
		status.DCConnected = err == nil
	}

	return status
}

// Close 关闭管理器.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.engine != nil {
		m.engine.Stop()
		m.engine = nil
	}
	return nil
}
