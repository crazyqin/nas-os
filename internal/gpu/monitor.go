// Package gpu GPU监控模块
package gpu

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Monitor GPU监控器
type Monitor struct {
	manager  *Manager
	interval int // 秒
	logger   *zap.Logger
	mu       sync.RWMutex
	stopped  bool
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewMonitor 创建GPU监控器
func NewMonitor(manager *Manager, interval int, logger *zap.Logger) *Monitor {
	if logger == nil {
		logger = zap.NewNop()
	}
	if interval <= 0 {
		interval = 5
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Monitor{
		manager:  manager,
		interval: interval,
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start 启动监控
func (m *Monitor) Start(ctx context.Context) {
	m.mu.Lock()
	m.stopped = false
	m.mu.Unlock()

	ticker := time.NewTicker(time.Duration(m.interval) * time.Second)
	defer ticker.Stop()

	m.logger.Info("GPU监控器已启动", zap.Int("interval", m.interval))

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("GPU监控器已停止")
			return
		case <-m.ctx.Done():
			m.logger.Info("GPU监控器已停止")
			return
		case <-ticker.C:
			m.collect()
		}
	}
}

// collect 收集GPU指标
func (m *Monitor) collect() {
	m.mu.RLock()
	if m.stopped {
		m.mu.RUnlock()
		return
	}
	m.mu.RUnlock()

	// 通过健康检查更新GPU状态
	m.manager.checkHealth()
}

// SetInterval 设置监控间隔
func (m *Monitor) SetInterval(interval int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if interval <= 0 {
		interval = 5
	}
	m.interval = interval
}

// Stop 停止监控
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopped = true
	m.cancel()
}

// IsRunning 检查监控器是否运行
func (m *Monitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return !m.stopped
}
