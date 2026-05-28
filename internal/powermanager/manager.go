// Package powermanager 提供电源管理功能
package powermanager

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// Manager 电源管理器.
type Manager struct {
	mu              sync.RWMutex
	currentPlan     *PowerPlanConfig
	schedules       map[string]*PowerSchedule
	upsInfo         *UPSInfo
	consumptionLog  []*ConsumptionRecord
	maxLogSize      int
	stopChan        chan struct{}
	running         bool
}

// NewManager 创建电源管理器.
func NewManager() *Manager {
	return &Manager{
		currentPlan: &PowerPlanConfig{
			Plan:          PowerPlanBalanced,
			CPUGovernor:   "ondemand",
			HDDStandby:    30,
			LEDBrightness: 50,
			FanProfile:    "auto",
			WoLEnabled:    true,
			UpdatedAt:     time.Now(),
		},
		schedules:      make(map[string]*PowerSchedule),
		upsInfo:        &UPSInfo{Status: UPSStatusUnknown},
		consumptionLog: make([]*ConsumptionRecord, 0),
		maxLogSize:     1440, // 24小时，每分钟一条
		stopChan:       make(chan struct{}),
	}
}

// Start 启动电源管理器.
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopChan = make(chan struct{})
	m.mu.Unlock()

	go m.monitorLoop()
	log.Println("power manager started")
}

// Stop 停止电源管理器.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopChan)
	log.Println("power manager stopped")
}

// monitorLoop 监控循环.
func (m *Manager) monitorLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.collectConsumption()
			m.checkUPS()
			m.checkSchedules()
		case <-m.stopChan:
			return
		}
	}
}

// GetPlans 获取所有电源计划.
func (m *Manager) GetPlans() []*PowerPlanConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plans := []*PowerPlanConfig{
		{
			Plan:          PowerPlanHighPerf,
			CPUGovernor:   "performance",
			HDDStandby:    0,
			LEDBrightness: 100,
			FanProfile:    "performance",
			WoLEnabled:    true,
		},
		{
			Plan:          PowerPlanBalanced,
			CPUGovernor:   "ondemand",
			HDDStandby:    30,
			LEDBrightness: 50,
			FanProfile:    "auto",
			WoLEnabled:    true,
		},
		{
			Plan:          PowerPlanPowerSave,
			CPUGovernor:   "powersave",
			HDDStandby:    10,
			LEDBrightness: 20,
			FanProfile:    "quiet",
			WoLEnabled:    false,
		},
	}
	return plans
}

// GetCurrentPlan 获取当前电源计划.
func (m *Manager) GetCurrentPlan() *PowerPlanConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentPlan
}

// SetPlan 设置电源计划.
func (m *Manager) SetPlan(plan PowerPlan) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.currentPlan = &PowerPlanConfig{
		Plan:          plan,
		UpdatedAt:     time.Now(),
	}

	switch plan {
	case PowerPlanHighPerf:
		m.currentPlan.CPUGovernor = "performance"
		m.currentPlan.HDDStandby = 0
		m.currentPlan.LEDBrightness = 100
		m.currentPlan.FanProfile = "performance"
		m.currentPlan.WoLEnabled = true
	case PowerPlanBalanced:
		m.currentPlan.CPUGovernor = "ondemand"
		m.currentPlan.HDDStandby = 30
		m.currentPlan.LEDBrightness = 50
		m.currentPlan.FanProfile = "auto"
		m.currentPlan.WoLEnabled = true
	case PowerPlanPowerSave:
		m.currentPlan.CPUGovernor = "powersave"
		m.currentPlan.HDDStandby = 10
		m.currentPlan.LEDBrightness = 20
		m.currentPlan.FanProfile = "quiet"
		m.currentPlan.WoLEnabled = false
	default:
		return fmt.Errorf("unknown power plan: %s", plan)
	}

	log.Printf("power plan changed to %s", plan)
	return nil
}

// AddSchedule 添加定时任务.
func (m *Manager) AddSchedule(schedule *PowerSchedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if schedule.ID == "" {
		schedule.ID = fmt.Sprintf("schedule_%d", time.Now().UnixNano())
	}
	schedule.CreatedAt = time.Now()

	m.schedules[schedule.ID] = schedule
	log.Printf("added power schedule: %s (%s at %s)", schedule.ID, schedule.Action, schedule.Time)
	return nil
}

// RemoveSchedule 删除定时任务.
func (m *Manager) RemoveSchedule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.schedules[id]; !ok {
		return fmt.Errorf("schedule %s not found", id)
	}

	delete(m.schedules, id)
	log.Printf("removed power schedule: %s", id)
	return nil
}

// GetSchedules 获取所有定时任务.
func (m *Manager) GetSchedules() []*PowerSchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	schedules := make([]*PowerSchedule, 0, len(m.schedules))
	for _, s := range m.schedules {
		schedules = append(schedules, s)
	}
	return schedules
}

// GetUPSStatus 获取 UPS 状态.
func (m *Manager) GetUPSStatus() *UPSInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.upsInfo
}

// GetConsumptionStats 获取功耗统计.
func (m *Manager) GetConsumptionStats() *ConsumptionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &ConsumptionStats{}

	if len(m.consumptionLog) > 0 {
		stats.Current = m.consumptionLog[len(m.consumptionLog)-1]

		var total float64
		var peak float64
		for _, r := range m.consumptionLog {
			total += r.PowerWatts
			if r.PowerWatts > peak {
				peak = r.PowerWatts
			}
		}
		stats.Average24h = total / float64(len(m.consumptionLog))
		stats.Peak24h = peak
		stats.TotalKWh = total / 1000.0 / 60.0 // 瓦特*分钟 -> 千瓦时
	}

	return stats
}

// SendWakeOnLAN 发送网络唤醒包.
func (m *Manager) SendWakeOnLAN(req *WoLRequest) error {
	if req.MACAddress == "" {
		return fmt.Errorf("MAC address is required")
	}

	mac, err := net.ParseMAC(req.MACAddress)
	if err != nil {
		return fmt.Errorf("invalid MAC address: %w", err)
	}

	broadcast := req.Broadcast
	if broadcast == "" {
		broadcast = "255.255.255.255"
	}

	port := req.Port
	if port == 0 {
		port = 9
	}

	// 构建 Magic Packet
	packet := make([]byte, 6+16*6)
	// 6 字节 0xFF
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}
	// 16 次重复 MAC 地址
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:], mac)
	}

	// 发送 UDP 包
	addr := net.JoinHostPort(broadcast, fmt.Sprintf("%d", port))
	conn, err := net.Dial("udp4", addr)
	if err != nil {
		return fmt.Errorf("failed to dial UDP: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write(packet)
	if err != nil {
		return fmt.Errorf("failed to send WoL packet: %w", err)
	}

	log.Printf("sent WoL packet to %s", req.MACAddress)
	return nil
}

// collectConsumption 收集功耗数据.
func (m *Manager) collectConsumption() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟功耗数据采集
	record := &ConsumptionRecord{
		Timestamp:  time.Now(),
		PowerWatts: 45.0 + float64(time.Now().Second()%20),
		CPUUsage:   15.0 + float64(time.Now().Second()%30),
		DiskIO:     1024.0 * float64(time.Now().Second()%10),
		NetworkIO:  512.0 * float64(time.Now().Second()%5),
	}

	m.consumptionLog = append(m.consumptionLog, record)

	// 限制日志大小
	if len(m.consumptionLog) > m.maxLogSize {
		m.consumptionLog = m.consumptionLog[len(m.consumptionLog)-m.maxLogSize:]
	}
}

// checkUPS 检查 UPS 状态.
func (m *Manager) checkUPS() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟 UPS 状态检查
	m.upsInfo = &UPSInfo{
		Status:        UPSStatusOnline,
		BatteryLevel:  95,
		LoadPercent:   30,
		InputVoltage:  220.5,
		OutputVoltage: 220.0,
		Temperature:   35.0,
		RuntimeMins:   45,
		LastUpdated:   time.Now(),
	}
}

// checkSchedules 检查定时任务.
func (m *Manager) checkSchedules() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	currentTime := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())
	currentDay := now.Weekday().String()[:3]

	for _, schedule := range m.schedules {
		if !schedule.Enabled {
			continue
		}

		if schedule.Time != currentTime {
			continue
		}

		// 检查是否是今天
		dayMatch := false
		for _, day := range schedule.Days {
			if day == currentDay {
				dayMatch = true
				break
			}
		}

		if dayMatch {
			log.Printf("executing scheduled power action: %s", schedule.Action)
			// 实际执行关机/开机命令
		}
	}
}
