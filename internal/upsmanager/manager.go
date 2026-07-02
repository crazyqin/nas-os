package upsmanager

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrUPSNotFound UPS 设备未找到.
	ErrUPSNotFound = errors.New("UPS 设备未找到")
	// ErrUPSAlreadyConnected UPS 已连接.
	ErrUPSAlreadyConnected = errors.New("UPS 设备已连接")
	// ErrNoPrimaryUPS 没有主 UPS.
	ErrNoPrimaryUPS = errors.New("没有主 UPS 设备")
	// ErrProtocolNotSupported 协议不支持.
	ErrProtocolNotSupported = errors.New("协议不支持")
	// ErrConnectionFailed 连接失败.
	ErrConnectionFailed = errors.New("UPS 连接失败")
	// ErrShutdownPolicyNotFound 关机策略未找到.
	ErrShutdownPolicyNotFound = errors.New("关机策略未找到")
	// ErrEventNotFound 事件未找到.
	ErrEventNotFound = errors.New("事件未找到")
)

// ========== 管理器 ==========

// Manager UPS 电源管理核心.
type Manager struct {
	mu       sync.RWMutex
	config   Config
	devices  map[string]*UPSDevice      // upsID -> device
	power    map[string]*PowerStatus    // upsID -> power status
	health   map[string]*HardwareHealth // upsID -> hardware health
	policies map[string]*ShutdownPolicy // policyID -> shutdown policy
	events   []PowerEvent               // 电源事件历史
	stats    map[string]*PowerStats     // upsID -> stats

	// 运行状态
	running    bool
	stopCh     chan struct{}
	pollTicker *time.Ticker

	// 告警回调
	alertCallback func(event PowerEvent)
}

// NewManager 创建 UPS 管理器.
func NewManager(cfg Config) *Manager {
	return &Manager{
		config:   cfg,
		devices:  make(map[string]*UPSDevice),
		power:    make(map[string]*PowerStatus),
		health:   make(map[string]*HardwareHealth),
		policies: make(map[string]*ShutdownPolicy),
		stats:    make(map[string]*PowerStats),
		stopCh:   make(chan struct{}),
	}
}

// SetAlertCallback 设置告警回调函数.
func (m *Manager) SetAlertCallback(cb func(event PowerEvent)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertCallback = cb
}

// ========== UPS 设备发现与连接 ==========

// Discover 发现 UPS 设备.
func (m *Manager) Discover(req DiscoverRequest) ([]*UPSDevice, error) {
	switch req.Protocol {
	case ProtocolUSBHID:
		return m.discoverUSB()
	case ProtocolSNMP:
		return m.discoverSNMP(req.Address, req.Port)
	case ProtocolNUT:
		return m.discoverNUT(req.Address, req.Port)
	default:
		return nil, ErrProtocolNotSupported
	}
}

// Connect 连接 UPS 设备.
func (m *Manager) Connect(req ConnectRequest) (*UPSDevice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已连接
	for _, dev := range m.devices {
		if dev.Address == req.Address && dev.Protocol == req.Protocol {
			return nil, ErrUPSAlreadyConnected
		}
	}

	// 生成设备 ID
	upsID := fmt.Sprintf("ups-%s-%d", req.Protocol, len(m.devices)+1)

	// 如果设为主 UPS，先将其他设备设为非主
	if req.IsPrimary {
		for _, dev := range m.devices {
			dev.IsPrimary = false
		}
	}

	device := &UPSDevice{
		ID:          upsID,
		Name:        req.Name,
		Model:       fmt.Sprintf("UPS-%s", req.Protocol),
		Protocol:    req.Protocol,
		Address:     req.Address,
		Port:        req.Port,
		Status:      UPSStatusOnline,
		IsPrimary:   req.IsPrimary,
		ConnectedAt: time.Now(),
		LastSeen:    time.Now(),
		CreatedAt:   time.Now(),
	}

	m.devices[upsID] = device

	// 初始化电源状态
	m.power[upsID] = &PowerStatus{
		UPSID:         upsID,
		Status:        UPSStatusOnline,
		InputVoltage:  220.0,
		OutputVoltage: 220.0,
		InputFreq:     50.0,
		Load:          30.0,
		Battery: BatteryInfo{
			Charge:      100.0,
			Voltage:     24.0,
			Current:     0.5,
			Temperature: 25.0,
			Health:      "good",
			Capacity:    100.0,
			UpdatedAt:   time.Now(),
		},
		Temperature: 30.0,
		RuntimeLeft: 120,
		UpdatedAt:   time.Now(),
	}

	// 初始化统计
	m.stats[upsID] = &PowerStats{
		UPSID:         upsID,
		UptimePercent: 100.0,
		UpdatedAt:     time.Now(),
	}

	// 初始化硬件健康
	m.health[upsID] = &HardwareHealth{
		UPSID: upsID,
		DiskTemps: []DiskTemp{
			{Device: "/dev/sda", Temp: 35.0},
		},
		FanSpeeds: []FanSpeed{
			{Name: "CPU Fan", RPM: 1200},
		},
		Voltages: []VoltageReading{
			{Name: "Vcore", Value: 1.2},
		},
		UpdatedAt: time.Now(),
	}

	// 记录事件
	m.recordEvent(upsID, EventUPSSwitch2, "UPS 已连接: "+req.Name, "info")

	log.Printf("[UPS管理] UPS 已连接: %s (%s)", upsID, req.Address)

	return device, nil
}

// Disconnect 断开 UPS 设备.
func (m *Manager) Disconnect(upsID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[upsID]
	if !ok {
		return ErrUPSNotFound
	}

	// 记录断开事件
	m.recordEvent(upsID, EventUPSDisconnected, "UPS 已断开: "+device.Name, "info")

	delete(m.devices, upsID)
	delete(m.power, upsID)
	delete(m.health, upsID)
	delete(m.stats, upsID)

	log.Printf("[UPS管理] UPS 已断开: %s", upsID)
	return nil
}

// ListDevices 列出所有 UPS 设备.
func (m *Manager) ListDevices() []*UPSDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*UPSDevice, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	return devices
}

// GetDevice 获取 UPS 设备信息.
func (m *Manager) GetDevice(upsID string) (*UPSDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[upsID]
	if !ok {
		return nil, ErrUPSNotFound
	}
	return device, nil
}

// ========== 电源状态监控 ==========

// GetPowerStatus 获取电源状态.
func (m *Manager) GetPowerStatus(upsID string) (*PowerStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.devices[upsID]; !ok {
		return nil, ErrUPSNotFound
	}

	status, ok := m.power[upsID]
	if !ok {
		return nil, fmt.Errorf("无电源状态数据")
	}
	return status, nil
}

// GetAllPowerStatus 获取所有 UPS 电源状态.
func (m *Manager) GetAllPowerStatus() map[string]*PowerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*PowerStatus, len(m.power))
	for id, status := range m.power {
		result[id] = status
	}
	return result
}

// GetPrimaryPowerStatus 获取主 UPS 电源状态.
func (m *Manager) GetPrimaryPowerStatus() (*PowerStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, dev := range m.devices {
		if dev.IsPrimary {
			status, ok := m.power[dev.ID]
			if !ok {
				return nil, fmt.Errorf("主 UPS 无电源状态数据")
			}
			return status, nil
		}
	}
	return nil, ErrNoPrimaryUPS
}

// ========== 硬件健康监控 ==========

// GetHardwareHealth 获取硬件健康信息.
func (m *Manager) GetHardwareHealth(upsID string) (*HardwareHealth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.devices[upsID]; !ok {
		return nil, ErrUPSNotFound
	}

	health, ok := m.health[upsID]
	if !ok {
		return nil, fmt.Errorf("无硬件健康数据")
	}
	return health, nil
}

// ========== 关机策略 ==========

// CreateShutdownPolicy 创建关机策略.
func (m *Manager) CreateShutdownPolicy(req SetShutdownPolicyRequest) (*ShutdownPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policyID := fmt.Sprintf("policy-%d", len(m.policies)+1)
	policy := &ShutdownPolicy{
		ID:               policyID,
		Name:             req.Name,
		Enabled:          req.Enabled,
		BatteryThreshold: req.BatteryThreshold,
		DelaySeconds:     req.DelaySeconds,
		RuntimeThreshold: req.RuntimeThreshold,
		NotifyBefore:     req.NotifyBefore,
		Command:          req.Command,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	m.policies[policyID] = policy
	return policy, nil
}

// GetShutdownPolicy 获取关机策略.
func (m *Manager) GetShutdownPolicy(policyID string) (*ShutdownPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[policyID]
	if !ok {
		return nil, ErrShutdownPolicyNotFound
	}
	return policy, nil
}

// ListShutdownPolicies 列出所有关机策略.
func (m *Manager) ListShutdownPolicies() []*ShutdownPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*ShutdownPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// UpdateShutdownPolicy 更新关机策略.
func (m *Manager) UpdateShutdownPolicy(policyID string, req SetShutdownPolicyRequest) (*ShutdownPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, ok := m.policies[policyID]
	if !ok {
		return nil, ErrShutdownPolicyNotFound
	}

	policy.Name = req.Name
	policy.Enabled = req.Enabled
	policy.BatteryThreshold = req.BatteryThreshold
	policy.DelaySeconds = req.DelaySeconds
	policy.RuntimeThreshold = req.RuntimeThreshold
	policy.NotifyBefore = req.NotifyBefore
	policy.Command = req.Command
	policy.UpdatedAt = time.Now()

	return policy, nil
}

// DeleteShutdownPolicy 删除关机策略.
func (m *Manager) DeleteShutdownPolicy(policyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[policyID]; !ok {
		return ErrShutdownPolicyNotFound
	}

	delete(m.policies, policyID)
	return nil
}

// ========== 电源事件 ==========

// GetEvents 获取电源事件.
func (m *Manager) GetEvents(params EventQueryParams) []PowerEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []PowerEvent
	for _, e := range m.events {
		if params.UPSID != "" && e.UPSID != params.UPSID {
			continue
		}
		if params.Type != "" && string(e.Type) != params.Type {
			continue
		}
		if params.Severity != "" && e.Severity != params.Severity {
			continue
		}
		result = append(result, e)
	}

	// 分页
	total := len(result)
	start := params.Offset
	if start > total {
		start = total
	}
	end := start + params.Limit
	if params.Limit == 0 {
		end = total
	}
	if end > total {
		end = total
	}

	return result[start:end]
}

// GetEventCount 获取事件总数.
func (m *Manager) GetEventCount(upsID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if upsID == "" {
		return len(m.events)
	}
	count := 0
	for _, e := range m.events {
		if e.UPSID == upsID {
			count++
		}
	}
	return count
}

// ========== 电源统计 ==========

// GetPowerStats 获取电源统计.
func (m *Manager) GetPowerStats(upsID string) (*PowerStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.devices[upsID]; !ok {
		return nil, ErrUPSNotFound
	}

	stats, ok := m.stats[upsID]
	if !ok {
		return nil, fmt.Errorf("无统计数据")
	}
	return stats, nil
}

// ========== 配置管理 ==========

// GetConfig 获取配置.
func (m *Manager) GetConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(req UpdateConfigRequest) Config {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.PollInterval > 0 {
		m.config.PollInterval = req.PollInterval
	}
	if req.AlertThreshold > 0 {
		m.config.AlertThreshold = req.AlertThreshold
	}
	m.config.AutoSwitch = req.AutoSwitch
	if req.HistoryMax > 0 {
		m.config.HistoryMax = req.HistoryMax
	}

	return m.config
}

// ========== 运行控制 ==========

// Start 启动定时轮询.
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopCh = make(chan struct{})
	m.mu.Unlock()

	interval := time.Duration(m.config.PollInterval) * time.Second
	m.pollTicker = time.NewTicker(interval)

	go m.pollLoop()

	log.Printf("[UPS管理] 启动定时轮询，间隔 %v", interval)
}

// Stop 停止定时轮询.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	if m.pollTicker != nil {
		m.pollTicker.Stop()
	}
	close(m.stopCh)
	log.Println("[UPS管理] 停止定时轮询")
}

// IsRunning 是否运行中.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// ========== 内部方法 ==========

// pollLoop 轮询循环.
func (m *Manager) pollLoop() {
	// 立即采集一次
	m.poll()

	for {
		select {
		case <-m.pollTicker.C:
			m.poll()
		case <-m.stopCh:
			return
		}
	}
}

// poll 执行一次轮询.
func (m *Manager) poll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for upsID, device := range m.devices {
		// 更新最后通信时间
		device.LastSeen = time.Now()

		// 模拟电源状态变化
		m.simulatePowerStatus(upsID)

		// 更新硬件健康
		m.updateHardwareHealth(upsID)

		// 检查关机策略
		m.checkShutdownPolicies(upsID)

		// 检查主备切换
		if m.config.AutoSwitch {
			m.checkFailover(upsID)
		}
	}
}

// simulatePowerStatus 模拟电源状态变化（实际应从 UPS 读取）.
func (m *Manager) simulatePowerStatus(upsID string) {
	status, ok := m.power[upsID]
	if !ok {
		return
	}

	// 模拟数据波动
	status.InputVoltage = 220.0 + float64(time.Now().Second()%10-5)
	status.OutputVoltage = status.InputVoltage * 0.99
	status.Load = 30.0 + float64(time.Now().Second()%20)
	status.Temperature = 30.0 + float64(time.Now().Second()%5)

	// 模拟电池电量缓慢下降
	if status.Status == UPSStatusOnBattery {
		status.Battery.Charge -= 0.1
		if status.Battery.Charge < 0 {
			status.Battery.Charge = 0
		}
		status.RuntimeLeft = int(status.Battery.Charge * 1.2)
	} else {
		if status.Battery.Charge < 100 {
			status.Battery.Charge += 0.5
			if status.Battery.Charge > 100 {
				status.Battery.Charge = 100
			}
		}
		status.RuntimeLeft = 120
	}

	status.Battery.Temperature = 25.0 + float64(time.Now().Second()%5)
	status.Battery.Voltage = 24.0 - status.Battery.Charge*0.01
	status.Battery.UpdatedAt = time.Now()
	status.UpdatedAt = time.Now()
}

// updateHardwareHealth 更新硬件健康信息.
func (m *Manager) updateHardwareHealth(upsID string) {
	health, ok := m.health[upsID]
	if !ok {
		return
	}

	now := time.Now()
	health.CPUTemp = 45.0 + float64(now.Second()%10)

	for i := range health.DiskTemps {
		health.DiskTemps[i].Temp = 35.0 + float64(now.Second()%8)
	}

	for i := range health.FanSpeeds {
		health.FanSpeeds[i].RPM = 1200 + now.Second()*10
	}

	health.UpdatedAt = now

	// 检查硬件告警
	if health.CPUTemp > 70 {
		m.recordEvent(upsID, EventHardwareAlert,
			fmt.Sprintf("CPU 温度过高: %.1f°C", health.CPUTemp), "warning")
	}

	for _, disk := range health.DiskTemps {
		if disk.Temp > 55 {
			m.recordEvent(upsID, EventHardwareAlert,
				fmt.Sprintf("磁盘 %s 温度过高: %.1f°C", disk.Device, disk.Temp), "warning")
		}
	}
}

// checkShutdownPolicies 检查关机策略.
func (m *Manager) checkShutdownPolicies(upsID string) {
	status, ok := m.power[upsID]
	if !ok || status.Status != UPSStatusOnBattery {
		return
	}

	for _, policy := range m.policies {
		if !policy.Enabled {
			continue
		}

		shouldShutdown := false
		reason := ""

		// 检查电池电量阈值
		if policy.BatteryThreshold > 0 && status.Battery.Charge <= policy.BatteryThreshold {
			shouldShutdown = true
			reason = fmt.Sprintf("电池电量 %.1f%% 低于阈值 %.1f%%", status.Battery.Charge, policy.BatteryThreshold)
		}

		// 检查运行时间阈值
		if policy.RuntimeThreshold > 0 && status.RuntimeLeft <= policy.RuntimeThreshold {
			shouldShutdown = true
			reason = fmt.Sprintf("剩余运行时间 %d 分钟低于阈值 %d 分钟", status.RuntimeLeft, policy.RuntimeThreshold)
		}

		if shouldShutdown {
			// 记录事件
			m.recordEvent(upsID, EventBatteryLow, reason, "critical")

			// 更新电池状态
			status.Status = UPSStatusLowBattery
			m.power[upsID] = status

			log.Printf("[UPS管理] 关机策略触发: %s - %s", policy.Name, reason)

			// 实际关机逻辑应在这里实现
			// m.executeShutdown(policy, reason)
		}
	}
}

// checkFailover 检查主备切换.
func (m *Manager) checkFailover(failedUPS string) {
	failedDevice, ok := m.devices[failedUPS]
	if !ok || !failedDevice.IsPrimary {
		return
	}

	failedStatus, ok := m.power[failedUPS]
	if !ok {
		return
	}

	// 主 UPS 故障或低电量，切换到备用
	if failedStatus.Status == UPSStatusFault || failedStatus.Status == UPSStatusLowBattery {
		// 找到备用 UPS
		for backupID, backupDevice := range m.devices {
			if backupID == failedUPS {
				continue
			}

			backupStatus, ok := m.power[backupID]
			if !ok || backupStatus.Status == UPSStatusFault {
				continue
			}

			// 执行切换
			failedDevice.IsPrimary = false
			backupDevice.IsPrimary = true

			m.recordEvent(backupID, EventUPSSwitch,
				fmt.Sprintf("主备切换: %s -> %s", failedDevice.Name, backupDevice.Name), "warning")

			log.Printf("[UPS管理] 主备切换: %s -> %s", failedDevice.Name, backupDevice.Name)
			break
		}
	}
}

// recordEvent 记录电源事件.
func (m *Manager) recordEvent(upsID string, eventType PowerEventType, message, severity string) PowerEvent {
	event := PowerEvent{
		ID:        fmt.Sprintf("evt-%d", len(m.events)+1),
		UPSID:     upsID,
		Type:      eventType,
		Message:   message,
		Severity:  severity,
		Timestamp: time.Now(),
	}

	m.events = append(m.events, event)

	// 限制历史记录数
	if len(m.events) > m.config.HistoryMax {
		m.events = m.events[len(m.events)-m.config.HistoryMax:]
	}

	// 更新统计
	m.updateStats(upsID, event)

	// 触发告警回调
	if severity == "warning" || severity == "critical" {
		if m.alertCallback != nil {
			go m.alertCallback(event)
		}
	}

	return event
}

// updateStats 更新统计数据.
func (m *Manager) updateStats(upsID string, event PowerEvent) {
	stats, ok := m.stats[upsID]
	if !ok {
		stats = &PowerStats{UPSID: upsID}
		m.stats[upsID] = stats
	}

	stats.TotalEvents++
	stats.UpdatedAt = time.Now()

	switch event.Type {
	case EventPowerOut:
		stats.PowerOutCount++
		now := time.Now()
		stats.LastPowerOut = &now
	case EventPowerRestore:
		now := time.Now()
		stats.LastPowerRestore = &now
	case EventBatteryLow, EventBatteryCritical:
		stats.BatteryDrainCount++
	}
}

// discoverUSB 发现 USB HID UPS 设备.
func (m *Manager) discoverUSB() ([]*UPSDevice, error) {
	// 模拟 USB HID 设备发现
	// 实际实现应扫描 /dev/usb/hiddev* 或使用 libusb
	log.Println("[UPS管理] 扫描 USB HID UPS 设备...")

	// 返回模拟的发现结果
	devices := []*UPSDevice{
		{
			ID:           "ups-usb-1",
			Name:         "USB UPS",
			Model:        "APC Back-UPS BX650LI",
			Manufacturer: "APC",
			Protocol:     ProtocolUSBHID,
			Address:      "/dev/usb/hiddev0",
			Status:       UPSStatusOnline,
		},
	}
	return devices, nil
}

// discoverSNMP 发现 SNMP UPS 设备.
func (m *Manager) discoverSNMP(address string, port int) ([]*UPSDevice, error) {
	if address == "" {
		address = "192.168.1.0/24"
	}
	if port == 0 {
		port = 161
	}

	log.Printf("[UPS管理] 扫描 SNMP UPS 设备: %s:%d", address, port)

	// 返回模拟的发现结果
	devices := []*UPSDevice{
		{
			ID:       "ups-snmp-1",
			Name:     "SNMP UPS",
			Model:    "Eaton 5P 1500",
			Protocol: ProtocolSNMP,
			Address:  "192.168.1.100",
			Port:     port,
			Status:   UPSStatusOnline,
		},
	}
	return devices, nil
}

// discoverNUT 发现 NUT UPS 设备.
func (m *Manager) discoverNUT(address string, port int) ([]*UPSDevice, error) {
	if address == "" {
		address = "localhost"
	}
	if port == 0 {
		port = 3493
	}

	log.Printf("[UPS管理] 扫描 NUT UPS 设备: %s:%d", address, port)

	// 返回模拟的发现结果
	devices := []*UPSDevice{
		{
			ID:       "ups-nut-1",
			Name:     "NUT UPS",
			Model:    "CyberPower CP1500AVRLCD",
			Protocol: ProtocolNUT,
			Address:  address,
			Port:     port,
			Status:   UPSStatusOnline,
		},
	}
	return devices, nil
}

// SetUPSStatus 手动设置 UPS 状态（用于测试）.
func (m *Manager) SetUPSStatus(upsID string, status UPSStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[upsID]
	if !ok {
		return ErrUPSNotFound
	}

	oldStatus := device.Status
	device.Status = status

	// 同步更新电源状态
	if power, ok := m.power[upsID]; ok {
		power.Status = status
	}

	// 记录状态变化事件
	if oldStatus != status {
		switch status {
		case UPSStatusOnBattery:
			m.recordEvent(upsID, EventPowerOut, "市电断电，切换到电池供电", "warning")
		case UPSStatusOnline:
			if oldStatus == UPSStatusOnBattery || oldStatus == UPSStatusLowBattery {
				m.recordEvent(upsID, EventPowerRestore, "市电已恢复", "info")
			}
		case UPSStatusLowBattery:
			m.recordEvent(upsID, EventBatteryLow, "电池电量低", "critical")
		case UPSStatusFault:
			m.recordEvent(upsID, EventHardwareAlert, "UPS 故障", "critical")
		}
	}

	return nil
}

// GetStatusSummary 获取所有 UPS 状态摘要.
func (m *Manager) GetStatusSummary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := map[string]interface{}{
		"totalDevices":  len(m.devices),
		"totalPolicies": len(m.policies),
		"totalEvents":   len(m.events),
		"running":       m.running,
	}

	// 统计各状态设备数
	statusCount := make(map[UPSStatus]int)
	for _, dev := range m.devices {
		statusCount[dev.Status]++
	}
	summary["statusCount"] = statusCount

	// 找到主 UPS
	for _, dev := range m.devices {
		if dev.IsPrimary {
			summary["primaryUPS"] = dev.ID
			if status, ok := m.power[dev.ID]; ok {
				summary["primaryBattery"] = status.Battery.Charge
			}
			break
		}
	}

	return summary
}
