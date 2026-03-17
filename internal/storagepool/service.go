// Package storagepool 提供存储池管理功能
package storagepool

import (
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Service 存储池服务
type Service struct {
	Manager  *Manager
	Handlers *Handlers
	monitor  *Monitor
}

// NewService 创建存储池服务
func NewService(dataPath, mountBase string) (*Service, error) {
	manager, err := NewManager(dataPath, mountBase)
	if err != nil {
		return nil, err
	}

	handlers := NewHandlers(manager)
	monitor := NewMonitor(manager)

	return &Service{
		Manager:  manager,
		Handlers: handlers,
		monitor:  monitor,
	}, nil
}

// Initialize 初始化服务
func (s *Service) Initialize() error {
	// 启动监控
	s.monitor.Start()

	log.Println("✅ 存储池服务就绪")
	return nil
}

// Close 关闭服务
func (s *Service) Close() {
	s.monitor.Stop()
}

// RegisterRoutes 注册路由
func (s *Service) RegisterRoutes(r *gin.RouterGroup) {
	s.Handlers.RegisterRoutes(r)
}

// DefaultDataPath 默认数据路径
const DefaultDataPath = "/var/lib/nas-os/storage-pools"

// DefaultMountBase 默认挂载基础目录
const DefaultMountBase = "/mnt/pools"

// InitializeService 初始化存储池服务（便捷函数）
func InitializeService() (*Service, error) {
	svc, err := NewService(DefaultDataPath, DefaultMountBase)
	if err != nil {
		return nil, err
	}

	if err := svc.Initialize(); err != nil {
		return nil, err
	}

	return svc, nil
}

// Monitor 存储池监控器
type Monitor struct {
	manager  *Manager
	interval time.Duration
	stopChan chan struct{}
	running  bool
	mu       sync.Mutex
}

// NewMonitor 创建监控器
func NewMonitor(manager *Manager) *Monitor {
	return &Monitor{
		manager:  manager,
		interval: 30 * time.Second,
		stopChan: make(chan struct{}),
	}
}

// Start 启动监控
func (m *Monitor) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return
	}

	m.running = true
	go m.run()
}

// Stop 停止监控
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	close(m.stopChan)
	m.running = false
}

// SetInterval 设置监控间隔
func (m *Monitor) SetInterval(interval time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.interval = interval
}

// run 运行监控循环
func (m *Monitor) run() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkAllPools()
		}
	}
}

// checkAllPools 检查所有存储池状态
func (m *Monitor) checkAllPools() {
	pools := m.manager.ListPools()

	for _, pool := range pools {
		m.checkPoolHealth(pool)
	}
}

// checkPoolHealth 检查存储池健康状态
func (m *Monitor) checkPoolHealth(pool *Pool) {
	// 计算健康分数
	healthScore := m.calculateHealthScore(pool)

	// 更新状态
	status := PoolStatusHealthy
	if healthScore < 60 {
		status = PoolStatusFaulted
	} else if healthScore < 80 {
		status = PoolStatusDegraded
	}

	// 检查设备状态
	faultedDevices := 0
	for _, d := range pool.Devices {
		if d.Status == DeviceStatusFaulted {
			faultedDevices++
		}
	}

	config := RAIDConfigs[pool.RAIDLevel]
	if faultedDevices > config.FaultTolerance {
		status = PoolStatusFaulted
	} else if faultedDevices > 0 {
		status = PoolStatusDegraded
	}

	// 更新统计
	update := &PoolStatsUpdate{
		HealthScore: &healthScore,
		Status:      status,
	}

	if err := m.manager.UpdatePoolStats(pool.ID, update); err != nil {
		log.Printf("⚠️ 更新存储池 %s 状态失败: %v", pool.Name, err)
	}
}

// calculateHealthScore 计算健康分数
func (m *Monitor) calculateHealthScore(pool *Pool) int {
	score := 100

	// 检查设备健康
	for _, d := range pool.Devices {
		switch d.Status {
		case DeviceStatusFaulted:
			score -= 30
		case DeviceStatusOffline:
			score -= 20
		}

		// SMART 状态
		if d.Health == "FAIL" || d.Health == "PREDICT_FAILURE" {
			score -= 25
		}

		// 温度警告
		if d.Temperature > 60 {
			score -= 5
		}
		if d.Temperature > 70 {
			score -= 10
		}
	}

	// 使用率
	usagePercent := float64(pool.Used) / float64(pool.Size) * 100
	if usagePercent > 90 {
		score -= 10
	} else if usagePercent > 80 {
		score -= 5
	}

	if score < 0 {
		score = 0
	}

	return score
}
