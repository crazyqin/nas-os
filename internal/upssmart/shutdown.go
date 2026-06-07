// Package upssmart 提供 UPS 智能管理功能
package upssmart

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// ShutdownTrigger 关机触发条件
type ShutdownTrigger string

const (
	TriggerBatteryLow ShutdownTrigger = "battery_low" // 电量低于阈值
	TriggerRuntimeLow ShutdownTrigger = "runtime_low" // 剩余时间低于阈值
	TriggerOverload   ShutdownTrigger = "overload"    // 负载超过阈值
	TriggerManual     ShutdownTrigger = "manual"      // 手动触发
)

// ShutdownPhase 关机阶段
type ShutdownPhase string

const (
	PhaseNotify       ShutdownPhase = "notify"        // 通知阶段
	PhaseSaveData     ShutdownPhase = "save_data"     // 保存数据阶段
	PhaseStopServices ShutdownPhase = "stop_services" // 停止服务阶段
	PhaseShutdown     ShutdownPhase = "shutdown"      // 关机阶段
)

// ShutdownPolicy 关机策略配置
type ShutdownPolicy struct {
	BatteryThreshold  int           `json:"battery_threshold"`   // 电量阈值百分比（默认20%）
	RuntimeThreshold  time.Duration `json:"runtime_threshold"`   // 剩余运行时间阈值（默认5分钟）
	LoadThreshold     int           `json:"load_threshold"`      // 负载阈值百分比（默认90%）
	DelayBeforeAction time.Duration `json:"delay_before_action"` // 触发后延迟执行时间（默认30秒）
	EnableSnapshot    bool          `json:"enable_snapshot"`     // 是否启用关机前快照
	SnapshotPath      string        `json:"snapshot_path"`       // 快照保存路径
	NotifyCommands    []string      `json:"notify_commands"`     // 通知命令列表
	StopServices      []string      `json:"stop_services"`       // 需要停止的服务列表
	GracefulTimeout   time.Duration `json:"graceful_timeout"`    // 优雅关闭超时（默认60秒）
}

// DefaultShutdownPolicy 返回默认关机策略
func DefaultShutdownPolicy() ShutdownPolicy {
	return ShutdownPolicy{
		BatteryThreshold:  20,
		RuntimeThreshold:  5 * time.Minute,
		LoadThreshold:     90,
		DelayBeforeAction: 30 * time.Second,
		EnableSnapshot:    true,
		SnapshotPath:      "/var/log/nas-os/snapshots",
		NotifyCommands:    []string{},
		StopServices:      []string{"docker", "smbd", "nfs-kernel-server"},
		GracefulTimeout:   60 * time.Second,
	}
}

// SystemSnapshot 系统状态快照
type SystemSnapshot struct {
	Timestamp  time.Time              `json:"timestamp"`
	Hostname   string                 `json:"hostname"`
	Uptime     string                 `json:"uptime"`
	Trigger    ShutdownTrigger        `json:"trigger"`
	UPSStatus  map[string]UPSStatus   `json:"ups_status"`
	Services   []ServiceStatus        `json:"services"`
	DiskUsage  map[string]int         `json:"disk_usage"` // 挂载点 -> 使用率%
	Processes  int                    `json:"processes"`  // 进程数
	LoadAvg    [3]float64             `json:"load_avg"`   // 1/5/15分钟负载
	CustomData map[string]interface{} `json:"custom_data"`
}

// ServiceStatus 服务状态
type ServiceStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"` // running, stopped, error
	PID    int    `json:"pid"`
}

// ShutdownManager 关机管理器
type ShutdownManager struct {
	mu           sync.RWMutex
	policy       ShutdownPolicy
	upsManager   *UPSManager
	shutdownCh   chan ShutdownTrigger
	snapshotCh   chan struct{}
	stopCh       chan struct{}
	running      bool
	shutdownOnce sync.Once
}

// NewShutdownManager 创建关机管理器
func NewShutdownManager(upsManager *UPSManager, policy ShutdownPolicy) *ShutdownManager {
	return &ShutdownManager{
		policy:     policy,
		upsManager: upsManager,
		shutdownCh: make(chan ShutdownTrigger, 10),
		snapshotCh: make(chan struct{}, 1),
		stopCh:     make(chan struct{}),
	}
}

// Start 启动关机管理器
func (sm *ShutdownManager) Start() {
	sm.mu.Lock()
	if sm.running {
		sm.mu.Unlock()
		return
	}
	sm.running = true
	sm.mu.Unlock()

	// 注册 UPS 事件回调
	sm.upsManager.RegisterEventCallback(sm.handleUPSEvent)

	// 启动监控协程
	go sm.monitorLoop()
	go sm.shutdownLoop()

	log.Println("✅ 关机管理器已启动")
}

// Stop 停止关机管理器
func (sm *ShutdownManager) Stop() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.running {
		return
	}

	close(sm.stopCh)
	sm.running = false
	log.Println("关机管理器已停止")
}

// GetPolicy 获取当前关机策略
func (sm *ShutdownManager) GetPolicy() ShutdownPolicy {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.policy
}

// UpdatePolicy 更新关机策略
func (sm *ShutdownManager) UpdatePolicy(policy ShutdownPolicy) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.policy = policy
	log.Printf("✅ 关机策略已更新: 电量阈值=%d%%, 运行时间阈值=%v, 负载阈值=%d%%",
		policy.BatteryThreshold, policy.RuntimeThreshold, policy.LoadThreshold)
}

// TriggerShutdown 手动触发关机
func (sm *ShutdownManager) TriggerShutdown() {
	sm.shutdownCh <- TriggerManual
}

// handleUPSEvent 处理 UPS 事件
func (sm *ShutdownManager) handleUPSEvent(event PowerEventRecord) {
	sm.mu.RLock()
	policy := sm.policy
	sm.mu.RUnlock()

	// 获取设备状态
	device, err := sm.upsManager.GetDevice(event.UPSID)
	if err != nil {
		return
	}

	status := device.Status

	// 检查是否需要触发关机
	if status.OnBattery {
		// 检查电量阈值
		if status.BatteryLevel <= policy.BatteryThreshold {
			log.Printf("⚠️ UPS %s 电量低于阈值 (%d%% <= %d%%), 触发关机",
				event.UPSID, status.BatteryLevel, policy.BatteryThreshold)
			sm.shutdownCh <- TriggerBatteryLow
			return
		}

		// 检查运行时间阈值
		if status.RuntimeLeft <= policy.RuntimeThreshold {
			log.Printf("⚠️ UPS %s 剩余运行时间低于阈值 (%v <= %v), 触发关机",
				event.UPSID, status.RuntimeLeft, policy.RuntimeThreshold)
			sm.shutdownCh <- TriggerRuntimeLow
			return
		}
	}

	// 检查负载阈值
	if status.LoadPercent >= policy.LoadThreshold {
		log.Printf("⚠️ UPS %s 负载超过阈值 (%d%% >= %d%%), 触发关机",
			event.UPSID, status.LoadPercent, policy.LoadThreshold)
		sm.shutdownCh <- TriggerOverload
		return
	}
}

// monitorLoop 持续监控 UPS 状态
func (sm *ShutdownManager) monitorLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sm.stopCh:
			return
		case <-ticker.C:
			sm.checkThresholds()
		}
	}
}

// checkThresholds 检查所有阈值
func (sm *ShutdownManager) checkThresholds() {
	sm.mu.RLock()
	policy := sm.policy
	sm.mu.RUnlock()

	// 获取主 UPS
	primary, err := sm.upsManager.GetPrimaryUPS()
	if err != nil {
		return
	}

	status := primary.Status

	// 检查电量阈值
	if status.OnBattery && status.BatteryLevel <= policy.BatteryThreshold {
		sm.shutdownCh <- TriggerBatteryLow
	}

	// 检查运行时间阈值
	if status.OnBattery && status.RuntimeLeft <= policy.RuntimeThreshold {
		sm.shutdownCh <- TriggerRuntimeLow
	}

	// 检查负载阈值
	if status.LoadPercent >= policy.LoadThreshold {
		sm.shutdownCh <- TriggerOverload
	}
}

// shutdownLoop 处理关机请求
func (sm *ShutdownManager) shutdownLoop() {
	for {
		select {
		case <-sm.stopCh:
			return
		case trigger := <-sm.shutdownCh:
			sm.executeShutdown(trigger)
		}
	}
}

// executeShutdown 执行优雅关机流程
func (sm *ShutdownManager) executeShutdown(trigger ShutdownTrigger) {
	sm.shutdownOnce.Do(func() {
		log.Printf("🔌 开始优雅关机流程 (触发原因: %s)", trigger)

		sm.mu.RLock()
		policy := sm.policy
		sm.mu.RUnlock()

		// 等待延迟时间
		if policy.DelayBeforeAction > 0 {
			log.Printf("⏳ 等待 %v 后执行关机...", policy.DelayBeforeAction)
			time.Sleep(policy.DelayBeforeAction)
		}

		// 阶段1: 通知用户
		sm.phaseNotify(trigger, policy)

		// 阶段2: 创建系统快照
		if policy.EnableSnapshot {
			sm.phaseSnapshot(trigger, policy)
		}

		// 阶段3: 停止服务
		sm.phaseStopServices(policy)

		// 阶段4: 执行关机
		sm.phaseShutdown()
	})
}

// phaseNotify 通知阶段
func (sm *ShutdownManager) phaseNotify(trigger ShutdownTrigger, policy ShutdownPolicy) {
	log.Printf("📢 阶段1: 通知用户 (触发原因: %s)", trigger)

	message := fmt.Sprintf("⚠️ 系统即将关机 (原因: %s)", trigger)

	// 执行通知命令
	for _, cmd := range policy.NotifyCommands {
		go func(command string) {
			if err := runCommand(command, message); err != nil {
				log.Printf("通知命令执行失败: %v", err)
			}
		}(cmd)
	}

	// 等待一下让用户收到通知
	time.Sleep(5 * time.Second)
}

// phaseSnapshot 系统快照阶段
func (sm *ShutdownManager) phaseSnapshot(trigger ShutdownTrigger, policy ShutdownPolicy) {
	log.Println("📸 阶段2: 创建系统快照")

	snapshot := sm.createSnapshot(trigger)

	// 确保目录存在
	if err := os.MkdirAll(policy.SnapshotPath, 0755); err != nil {
		log.Printf("创建快照目录失败: %v", err)
		return
	}

	// 保存快照
	filename := fmt.Sprintf("snapshot_%s.json", time.Now().Format("20060102_150405"))
	filepath := filepath.Join(policy.SnapshotPath, filename)

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		log.Printf("序列化快照失败: %v", err)
		return
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		log.Printf("保存快照失败: %v", err)
		return
	}

	log.Printf("✅ 系统快照已保存: %s", filepath)
}

// createSnapshot 创建系统快照
func (sm *ShutdownManager) createSnapshot(trigger ShutdownTrigger) SystemSnapshot {
	// 获取主机名
	hostname, _ := os.Hostname()

	// 获取所有 UPS 状态
	upsStatus := make(map[string]UPSStatus)
	for _, device := range sm.upsManager.GetAllDevices() {
		upsStatus[device.ID] = device.Status
	}

	// 获取服务状态
	services := sm.getServiceStatus()

	// 获取磁盘使用率
	diskUsage := sm.getDiskUsage()

	// 获取负载平均值
	loadAvg := sm.getLoadAvg()

	return SystemSnapshot{
		Timestamp: time.Now(),
		Hostname:  hostname,
		Trigger:   trigger,
		UPSStatus: upsStatus,
		Services:  services,
		DiskUsage: diskUsage,
		LoadAvg:   loadAvg,
	}
}

// getServiceStatus 获取服务状态（模拟实现）
func (sm *ShutdownManager) getServiceStatus() []ServiceStatus {
	sm.mu.RLock()
	services := sm.policy.StopServices
	sm.mu.RUnlock()

	result := make([]ServiceStatus, 0)
	for _, svc := range services {
		result = append(result, ServiceStatus{
			Name:   svc,
			Status: "running",
		})
	}
	return result
}

// getDiskUsage 获取磁盘使用率（模拟实现）
func (sm *ShutdownManager) getDiskUsage() map[string]int {
	return map[string]int{
		"/":     45,
		"/data": 60,
	}
}

// getLoadAvg 获取负载平均值（模拟实现）
func (sm *ShutdownManager) getLoadAvg() [3]float64 {
	return [3]float64{1.2, 1.5, 1.8}
}

// phaseStopServices 停止服务阶段
func (sm *ShutdownManager) phaseStopServices(policy ShutdownPolicy) {
	log.Println("⏹️ 阶段3: 停止服务")

	for _, svc := range policy.StopServices {
		log.Printf("停止服务: %s", svc)

		// 实际应执行: systemctl stop <service>
		if err := runCommand("systemctl", "stop", svc); err != nil {
			log.Printf("停止服务 %s 失败: %v", svc, err)
		}
	}

	// 等待服务完全停止
	time.Sleep(policy.GracefulTimeout)
}

// phaseShutdown 执行关机
func (sm *ShutdownManager) phaseShutdown() {
	log.Println("🔌 阶段4: 执行系统关机")

	// 实际应执行: shutdown -h now
	if err := runCommand("shutdown", "-h", "now"); err != nil {
		log.Printf("关机命令执行失败: %v", err)
	}
}

// runCommand 执行系统命令
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// String 返回关机管理器摘要
func (sm *ShutdownManager) String() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return fmt.Sprintf("ShutdownManager[running=%v, battery_threshold=%d%%]",
		sm.running, sm.policy.BatteryThreshold)
}
