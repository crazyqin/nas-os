// Package powermanager 提供智能功耗管理功能，包括硬盘休眠、CPU调频、定时开关机、功耗统计和节能策略。
package powermanager

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

// ========== 类型定义 ==========

// PowerPlan 节能策略等级.
type PowerPlan string

const (
	PowerPlanHighPerf  PowerPlan = "high_performance" // 高性能
	PowerPlanBalanced  PowerPlan = "balanced"         // 均衡
	PowerPlanPowerSave PowerPlan = "power_save"       // 节能
)

// UPSStatus UPS 状态.
type UPSStatus string

const (
	UPSStatusOnline   UPSStatus = "online"   // 在线（市电供电）
	UPSStatusBattery  UPSStatus = "battery"  // 电池供电
	UPSStatusLow      UPSStatus = "low"      // 电池低电量
	UPSStatusCritical UPSStatus = "critical" // 电量危急
	UPSStatusUnknown  UPSStatus = "unknown"  // 未知
)

// DiskState 硬盘状态.
type DiskState string

const (
	DiskStateActive DiskState = "active" // 活跃
	DiskStateIdle   DiskState = "idle"   // 空闲
	DiskStateStandby DiskState = "standby" // 待机（休眠）
)

// PowerSchedule 电源定时任务.
type PowerSchedule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Action    string    `json:"action"`    // "power_on" 或 "power_off"
	Time      string    `json:"time"`      // HH:MM 格式
	CronExpr  string    `json:"cron_expr"` // cron 表达式（优先于 Time）
	Days      []string  `json:"days"`      // 星期几 ["mon","tue",...]
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// PowerPlanConfig 电源计划配置.
type PowerPlanConfig struct {
	Plan          PowerPlan `json:"plan"`
	CPUGovernor   string    `json:"cpu_governor"`   // CPU 调频策略
	HDDStandby    int       `json:"hdd_standby"`    // 硬盘休眠时间（分钟）
	LEDBrightness int       `json:"led_brightness"` // LED 亮度 0-100
	FanProfile    string    `json:"fan_profile"`    // 风扇策略
	WoLEnabled    bool      `json:"wol_enabled"`    // 网络唤醒
	UpdatedAt     time.Time `json:"updated_at"`
}

// UPSInfo UPS 详细信息.
type UPSInfo struct {
	Status        UPSStatus `json:"status"`
	BatteryLevel  int       `json:"battery_level"`  // 电池电量百分比
	LoadPercent   int       `json:"load_percent"`   // 负载百分比
	InputVoltage  float64   `json:"input_voltage"`  // 输入电压
	OutputVoltage float64   `json:"output_voltage"` // 输出电压
	Temperature   float64   `json:"temperature"`    // 温度
	RuntimeMins   int       `json:"runtime_mins"`   // 剩余运行时间（分钟）
	LastUpdated   time.Time `json:"last_updated"`
}

// ConsumptionRecord 功耗记录.
type ConsumptionRecord struct {
	Timestamp  time.Time `json:"timestamp"`
	PowerWatts float64   `json:"power_watts"` // 功耗（瓦特）
	CPUUsage   float64   `json:"cpu_usage"`   // CPU 使用率
	DiskIO     float64   `json:"disk_io"`     // 磁盘 IO
	NetworkIO  float64   `json:"network_io"`  // 网络 IO
}

// ConsumptionStats 功耗统计.
type ConsumptionStats struct {
	Current    *ConsumptionRecord   `json:"current"`
	Average24h float64              `json:"average_24h"` // 24小时平均功耗
	Peak24h    float64              `json:"peak_24h"`    // 24小时峰值功耗
	TotalKWh   float64              `json:"total_kwh"`   // 总用电量（千瓦时）
	History    []*ConsumptionRecord `json:"history,omitempty"`
}

// WoLRequest 网络唤醒请求.
type WoLRequest struct {
	MACAddress string `json:"mac_address" binding:"required"` // 目标 MAC 地址
	Broadcast  string `json:"broadcast"`                      // 广播地址，默认 255.255.255.255
	Port       int    `json:"port"`                           // 端口，默认 9
}

// DiskInfo 硬盘信息.
type DiskInfo struct {
	Device      string    `json:"device"`       // 设备名，如 /dev/sda
	Model       string    `json:"model"`        // 型号
	State       DiskState `json:"state"`        // 当前状态
	IdleSince   time.Time `json:"idle_since"`   // 空闲起始时间
	IdleTimeout int       `json:"idle_timeout"` // 空闲超时（分钟）
	ReadBytes   int64     `json:"read_bytes"`   // 读取字节数
	WriteBytes  int64     `json:"write_bytes"`  // 写入字节数
	Temperature float64   `json:"temperature"`  // 温度
}

// CPUInfo CPU 信息.
type CPUInfo struct {
	Governor      string  `json:"governor"`       // 当前调频策略
	CurrentFreq   int     `json:"current_freq"`   // 当前频率 (MHz)
	MinFreq       int     `json:"min_freq"`       // 最小频率 (MHz)
	MaxFreq       int     `json:"max_freq"`       // 最大频率 (MHz)
	Usage         float64 `json:"usage"`          // CPU 使用率 (%)
	Temperature   float64 `json:"temperature"`    // 温度
	CoreCount     int     `json:"core_count"`     // 核心数
}

// ========== Manager ==========

// Manager 智能功耗管理器.
type Manager struct {
	mu             sync.RWMutex
	currentPlan    *PowerPlanConfig
	schedules      map[string]*PowerSchedule
	upsInfo        *UPSInfo
	consumptionLog []*ConsumptionRecord
	maxLogSize     int
	stopChan       chan struct{}
	running        bool
	disks          map[string]*DiskInfo
	cpuInfo        *CPUInfo
	diskIdleMin    int // 硬盘空闲超时（分钟）
}

// NewManager 创建功耗管理器.
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
		disks:          make(map[string]*DiskInfo),
		cpuInfo: &CPUInfo{
			Governor:    "ondemand",
			MinFreq:     800,
			MaxFreq:     3000,
			CurrentFreq: 1500,
			CoreCount:   4,
		},
		diskIdleMin: 30,
	}
}

// Start 启动功耗管理器.
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

// Stop 停止功耗管理器.
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

// IsRunning 返回管理器是否运行中.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// ========== 电源计划 ==========

// GetPlans 获取所有电源计划.
func (m *Manager) GetPlans() []*PowerPlanConfig {
	return []*PowerPlanConfig{
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

	config := &PowerPlanConfig{
		Plan:      plan,
		UpdatedAt: time.Now(),
	}

	switch plan {
	case PowerPlanHighPerf:
		config.CPUGovernor = "performance"
		config.HDDStandby = 0
		config.LEDBrightness = 100
		config.FanProfile = "performance"
		config.WoLEnabled = true
	case PowerPlanBalanced:
		config.CPUGovernor = "ondemand"
		config.HDDStandby = 30
		config.LEDBrightness = 50
		config.FanProfile = "auto"
		config.WoLEnabled = true
	case PowerPlanPowerSave:
		config.CPUGovernor = "powersave"
		config.HDDStandby = 10
		config.LEDBrightness = 20
		config.FanProfile = "quiet"
		config.WoLEnabled = false
	default:
		return fmt.Errorf("unknown power plan: %s", plan)
	}

	m.currentPlan = config
	m.applyCPUFreq(config.CPUGovernor)
	m.diskIdleMin = config.HDDStandby
	log.Printf("power plan changed to %s", plan)
	return nil
}

// ========== 定时任务（含cron支持） ==========

// AddSchedule 添加定时任务.
func (m *Manager) AddSchedule(schedule *PowerSchedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if schedule.ID == "" {
		schedule.ID = fmt.Sprintf("schedule_%d", time.Now().UnixNano())
	}
	schedule.CreatedAt = time.Now()

	// 验证 cron 表达式
	if schedule.CronExpr != "" {
		if _, err := ParseCronExpr(schedule.CronExpr); err != nil {
			return fmt.Errorf("invalid cron expression: %w", err)
		}
	}

	m.schedules[schedule.ID] = schedule
	log.Printf("added power schedule: %s (%s)", schedule.ID, schedule.Action)
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

// ========== 硬盘休眠管理 ==========

// RegisterDisk 注册硬盘.
func (m *Manager) RegisterDisk(device, model string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.disks[device] = &DiskInfo{
		Device:      device,
		Model:       model,
		State:       DiskStateActive,
		IdleTimeout: m.diskIdleMin,
	}
	log.Printf("registered disk: %s (%s)", device, model)
}

// GetDiskStatus 获取所有硬盘状态.
func (m *Manager) GetDiskStatus() []*DiskInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disks := make([]*DiskInfo, 0, len(m.disks))
	for _, d := range m.disks {
		disks = append(disks, d)
	}
	return disks
}

// HibernateDisk 将指定硬盘休眠.
func (m *Manager) HibernateDisk(device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	disk, ok := m.disks[device]
	if !ok {
		return fmt.Errorf("disk %s not found", device)
	}

	disk.State = DiskStateStandby
	disk.IdleSince = time.Now()
	log.Printf("disk %s hibernated", device)
	return nil
}

// WakeDisk 唤醒指定硬盘.
func (m *Manager) WakeDisk(device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	disk, ok := m.disks[device]
	if !ok {
		return fmt.Errorf("disk %s not found", device)
	}

	disk.State = DiskStateActive
	disk.IdleSince = time.Time{}
	log.Printf("disk %s woken up", device)
	return nil
}

// ========== CPU 动态调频 ==========

// GetCPUInfo 获取 CPU 信息.
func (m *Manager) GetCPUInfo() *CPUInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cpuInfo
}

// SetCPUGovernor 设置 CPU 调频策略.
func (m *Manager) SetCPUGovernor(governor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	validGovernors := map[string]bool{
		"performance": true,
		"powersave":   true,
		"ondemand":    true,
		"conservative": true,
		"schedutil":   true,
	}

	if !validGovernors[governor] {
		return fmt.Errorf("invalid CPU governor: %s", governor)
	}

	m.cpuInfo.Governor = governor
	m.applyCPUFreq(governor)
	log.Printf("CPU governor changed to %s", governor)
	return nil
}

// SetCPUFrequency 设置 CPU 固定频率（MHz）.
func (m *Manager) SetCPUFrequency(freqMHz int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if freqMHz < m.cpuInfo.MinFreq || freqMHz > m.cpuInfo.MaxFreq {
		return fmt.Errorf("frequency %d MHz out of range [%d, %d]", freqMHz, m.cpuInfo.MinFreq, m.cpuInfo.MaxFreq)
	}

	m.cpuInfo.CurrentFreq = freqMHz
	m.cpuInfo.Governor = "userspace"
	log.Printf("CPU frequency set to %d MHz", freqMHz)
	return nil
}

// ========== UPS ==========

// GetUPSStatus 获取 UPS 状态.
func (m *Manager) GetUPSStatus() *UPSInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.upsInfo
}

// ========== 功耗统计 ==========

// GetConsumptionStats 获取功耗统计.
func (m *Manager) GetConsumptionStats() *ConsumptionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &ConsumptionStats{}

	if len(m.consumptionLog) > 0 {
		stats.Current = m.consumptionLog[len(m.consumptionLog)-1]

		var total, peak float64
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

// GetConsumptionHistory 获取功耗历史记录.
func (m *Manager) GetConsumptionHistory() []*ConsumptionRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := make([]*ConsumptionRecord, len(m.consumptionLog))
	copy(history, m.consumptionLog)
	return history
}

// ========== Wake on LAN ==========

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
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:], mac)
	}

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

// ========== 内部方法 ==========

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
			m.checkDiskIdle()
			m.adjustCPUFreq()
		case <-m.stopChan:
			return
		}
	}
}

// collectConsumption 收集功耗数据.
func (m *Manager) collectConsumption() {
	m.mu.Lock()
	defer m.mu.Unlock()

	record := &ConsumptionRecord{
		Timestamp:  time.Now(),
		PowerWatts: 45.0 + rand.Float64()*20,
		CPUUsage:   m.cpuInfo.Usage,
		DiskIO:     rand.Float64() * 10240,
		NetworkIO:  rand.Float64() * 5120,
	}

	m.consumptionLog = append(m.consumptionLog, record)

	if len(m.consumptionLog) > m.maxLogSize {
		m.consumptionLog = m.consumptionLog[len(m.consumptionLog)-m.maxLogSize:]
	}
}

// checkUPS 检查 UPS 状态.
func (m *Manager) checkUPS() {
	m.mu.Lock()
	defer m.mu.Unlock()

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

		// 优先使用 cron 表达式
		if schedule.CronExpr != "" {
			if MatchCron(schedule.CronExpr, now) {
				log.Printf("executing cron scheduled action: %s", schedule.Action)
			}
			continue
		}

		// 回退到简单时间匹配
		if schedule.Time != currentTime {
			continue
		}

		dayMatch := false
		for _, day := range schedule.Days {
			if day == currentDay {
				dayMatch = true
				break
			}
		}

		if dayMatch || len(schedule.Days) == 0 {
			log.Printf("executing scheduled power action: %s", schedule.Action)
		}
	}
}

// checkDiskIdle 检查硬盘空闲状态，自动休眠.
func (m *Manager) checkDiskIdle() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, disk := range m.disks {
		if disk.State == DiskStateStandby {
			continue // 已休眠
		}

		// 模拟IO检查
		ioActive := rand.Float64() > 0.7

		if ioActive {
			disk.State = DiskStateActive
			disk.IdleSince = time.Time{}
			disk.ReadBytes += int64(rand.Intn(1024 * 1024))
			disk.WriteBytes += int64(rand.Intn(512 * 1024))
		} else {
			if disk.IdleSince.IsZero() {
				disk.IdleSince = time.Now()
				disk.State = DiskStateIdle
			}

			idleDuration := time.Since(disk.IdleSince)
			if int(idleDuration.Minutes()) >= disk.IdleTimeout && disk.IdleTimeout > 0 {
				disk.State = DiskStateStandby
				log.Printf("disk %s auto-hibernated after %d min idle", disk.Device, int(idleDuration.Minutes()))
			}
		}
	}
}

// applyCPUFreq 根据策略设置CPU频率.
func (m *Manager) applyCPUFreq(governor string) {
	switch governor {
	case "performance":
		m.cpuInfo.CurrentFreq = m.cpuInfo.MaxFreq
	case "powersave":
		m.cpuInfo.CurrentFreq = m.cpuInfo.MinFreq
	case "ondemand", "conservative", "schedutil":
		m.cpuInfo.CurrentFreq = (m.cpuInfo.MinFreq + m.cpuInfo.MaxFreq) / 2
	}
}

// adjustCPUFreq 根据负载动态调频.
func (m *Manager) adjustCPUFreq() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cpuInfo.Governor != "ondemand" && m.cpuInfo.Governor != "schedutil" {
		return
	}

	// 模拟负载变化
	m.cpuInfo.Usage = 10.0 + rand.Float64()*80.0
	m.cpuInfo.Temperature = 35.0 + rand.Float64()*25.0

	// 根据负载调频
	if m.cpuInfo.Usage > 70 {
		m.cpuInfo.CurrentFreq = m.cpuInfo.MaxFreq
	} else if m.cpuInfo.Usage > 40 {
		m.cpuInfo.CurrentFreq = (m.cpuInfo.MinFreq + m.cpuInfo.MaxFreq) / 2
	} else {
		m.cpuInfo.CurrentFreq = m.cpuInfo.MinFreq
	}
}

// ========== Cron 工具函数 ==========

// ParseCronExpr 解析 cron 表达式（简化版，支持 5 位格式：分 时 日 月 周）.
func ParseCronExpr(expr string) ([]string, error) {
	parts := splitFields(expr)
	if len(parts) != 5 {
		return nil, fmt.Errorf("cron expression must have 5 fields, got %d", len(parts))
	}
	return parts, nil
}

// MatchCron 检查时间是否匹配 cron 表达式.
func MatchCron(expr string, t time.Time) bool {
	parts, err := ParseCronExpr(expr)
	if err != nil {
		return false
	}

	min := t.Minute()
	hour := t.Hour()
	day := t.Day()
	month := int(t.Month())
	weekday := int(t.Weekday())

	return matchField(parts[0], min, 0, 59) &&
		matchField(parts[1], hour, 0, 23) &&
		matchField(parts[2], day, 1, 31) &&
		matchField(parts[3], month, 1, 12) &&
		matchField(parts[4], weekday, 0, 6)
}

// splitFields 按空格分割字段.
func splitFields(expr string) []string {
	var fields []string
	var current []byte

	for i := 0; i < len(expr); i++ {
		if expr[i] == ' ' || expr[i] == '\t' {
			if len(current) > 0 {
				fields = append(fields, string(current))
				current = current[:0]
			}
		} else {
			current = append(current, expr[i])
		}
	}
	if len(current) > 0 {
		fields = append(fields, string(current))
	}
	return fields
}

// matchField 匹配单个 cron 字段.
func matchField(field string, value, minVal, maxVal int) bool {
	if field == "*" {
		return true
	}

	// 处理 */N 步进
	if len(field) > 2 && field[0] == '*' && field[1] == '/' {
		step := 0
		for i := 2; i < len(field); i++ {
			if field[i] < '0' || field[i] > '9' {
				return false
			}
			step = step*10 + int(field[i]-'0')
		}
		if step <= 0 {
			return false
		}
		return (value-minVal)%step == 0
	}

	// 处理逗号分隔的列表
	for _, part := range splitComma(field) {
		if matchSingle(part, value, minVal, maxVal) {
			return true
		}
	}

	return false
}

// matchSingle 匹配单个值或范围.
func matchSingle(part string, value, minVal, maxVal int) bool {
	// 范围 a-b
	for i := 0; i < len(part); i++ {
		if part[i] == '-' {
			start := parseInt(part[:i])
			end := parseInt(part[i+1:])
			if start >= minVal && end <= maxVal && value >= start && value <= end {
				return true
			}
			return false
		}
	}

	// 精确值
	return parseInt(part) == value
}

// splitComma 按逗号分割.
func splitComma(s string) []string {
	var parts []string
	var current []byte
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			if len(current) > 0 {
				parts = append(parts, string(current))
				current = current[:0]
			}
		} else {
			current = append(current, s[i])
		}
	}
	if len(current) > 0 {
		parts = append(parts, string(current))
	}
	return parts
}

// parseInt 解析整数.
func parseInt(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return -1
		}
		n = n*10 + int(s[i]-'0')
	}
	return n
}
