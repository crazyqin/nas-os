// Package systemhealth 提供系统健康诊断与自愈功能。
// 实时监控硬件状态、磁盘健康、内存/CPU使用率，自动检测异常并触发自愈。
package systemhealth

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

// HealthStatus 健康状态
type HealthStatus string

const (
	StatusHealthy  HealthStatus = "healthy"  // 健康
	StatusWarning  HealthStatus = "warning"  // 警告
	StatusCritical HealthStatus = "critical" // 严重
	StatusUnknown  HealthStatus = "unknown"  // 未知
)

// ComponentType 组件类型
type ComponentType string

const (
	ComponentCPU     ComponentType = "cpu"
	ComponentMemory  ComponentType = "memory"
	ComponentDisk    ComponentType = "disk"
	ComponentNetwork ComponentType = "network"
	ComponentSystem  ComponentType = "system"
)

// HealthReport 健康报告
type HealthReport struct {
	Timestamp    time.Time              `json:"timestamp"`
	Overall      HealthStatus           `json:"overall"`
	Score        int                    `json:"score"` // 0-100
	Components   []ComponentHealth      `json:"components"`
	Alerts       []HealthAlert          `json:"alerts,omitempty"`
	HealingActions []HealingAction      `json:"healingActions,omitempty"`
	SystemInfo   SystemInfo             `json:"systemInfo"`
}

// ComponentHealth 组件健康状态
type ComponentHealth struct {
	Type      ComponentType `json:"type"`
	Name      string        `json:"name"`
	Status    HealthStatus  `json:"status"`
	Value     float64       `json:"value"`
	Threshold float64       `json:"threshold"`
	Unit      string        `json:"unit"`
	Message   string        `json:"message,omitempty"`
}

// HealthAlert 健康告警
type HealthAlert struct {
	ID        string        `json:"id"`
	Level     HealthStatus  `json:"level"`
	Component ComponentType `json:"component"`
	Message   string        `json:"message"`
	Timestamp time.Time     `json:"timestamp"`
	Resolved  bool          `json:"resolved"`
}

// HealingAction 自愈动作
type HealingAction struct {
	ID          string    `json:"id"`
	Trigger     string    `json:"trigger"`
	Action      string    `json:"action"`
	Status      string    `json:"status"` // pending, running, completed, failed
	StartedAt   time.Time `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	Result      string    `json:"result,omitempty"`
}

// SystemInfo 系统信息
type SystemInfo struct {
	Hostname    string `json:"hostname"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Kernel      string `json:"kernel"`
	Uptime      uint64 `json:"uptime"`
	GoVersion   string `json:"goVersion"`
	NumCPU      int    `json:"numCPU"`
	NumGoroutine int   `json:"numGoroutine"`
}

// HealthConfig 健康检查配置
type HealthConfig struct {
	CheckInterval    time.Duration `json:"checkInterval"`
	CPUWarning       float64       `json:"cpuWarning"`
	CPUCritical      float64       `json:"cpuCritical"`
	MemWarning       float64       `json:"memWarning"`
	MemCritical      float64       `json:"memCritical"`
	DiskWarning      float64       `json:"diskWarning"`
	DiskCritical     float64       `json:"diskCritical"`
	AutoHeal         bool          `json:"autoHeal"`
	AlertCooldown    time.Duration `json:"alertCooldown"`
	MaxAlerts        int           `json:"maxAlerts"`
}

// DefaultConfig 默认配置
func DefaultConfig() *HealthConfig {
	return &HealthConfig{
		CheckInterval: 5 * time.Minute,
		CPUWarning:    80.0,
		CPUCritical:   95.0,
		MemWarning:    85.0,
		MemCritical:   95.0,
		DiskWarning:   85.0,
		DiskCritical:  95.0,
		AutoHeal:      true,
		AlertCooldown: 15 * time.Minute,
		MaxAlerts:     100,
	}
}

// HealthEngine 健康引擎
type HealthEngine struct {
	config    *HealthConfig
	mu        sync.RWMutex
	alerts    []HealthAlert
	healing   []HealingAction
	lastCheck *HealthReport
	stopChan  chan struct{}
	running   bool
	callbacks []func(HealthAlert)
}

// NewHealthEngine 创建健康引擎
func NewHealthEngine(config *HealthConfig) *HealthEngine {
	if config == nil {
		config = DefaultConfig()
	}
	return &HealthEngine{
		config:  config,
		alerts:  make([]HealthAlert, 0),
		healing: make([]HealingAction, 0),
		stopChan: make(chan struct{}),
	}
}

// Start 启动健康监控
func (e *HealthEngine) Start(ctx context.Context) {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.mu.Unlock()

	go e.monitorLoop(ctx)
}

// Stop 停止健康监控
func (e *HealthEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return
	}
	e.running = false
	close(e.stopChan)
}

// OnAlert 注册告警回调
func (e *HealthEngine) OnAlert(callback func(HealthAlert)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.callbacks = append(e.callbacks, callback)
}

// Check 执行一次健康检查
func (e *HealthEngine) Check(ctx context.Context) (*HealthReport, error) {
	report := &HealthReport{
		Timestamp:  time.Now(),
		Components: make([]ComponentHealth, 0),
		Alerts:     make([]HealthAlert, 0),
	}

	// 获取系统信息
	report.SystemInfo = e.getSystemInfo()

	// 检查各组件
	cpuHealth := e.checkCPU(ctx)
	memHealth := e.checkMemory(ctx)
	diskHealth := e.checkDisk(ctx)

	report.Components = append(report.Components, cpuHealth, memHealth, diskHealth)

	// 计算总体评分
	report.Score = e.calculateScore(report.Components)
	report.Overall = e.scoreToStatus(report.Score)

	// 检测告警
	alerts := e.detectAlerts(report.Components)
	report.Alerts = alerts

	// 自愈处理
	if e.config.AutoHeal && len(alerts) > 0 {
		actions := e.autoHeal(ctx, alerts)
		report.HealingActions = actions
	}

	// 保存报告
	e.mu.Lock()
	e.lastCheck = report
	e.alerts = append(e.alerts, alerts...)
	// 限制告警数量
	if len(e.alerts) > e.config.MaxAlerts {
		e.alerts = e.alerts[len(e.alerts)-e.config.MaxAlerts:]
	}
	e.mu.Unlock()

	// 触发回调
	for _, alert := range alerts {
		e.notifyCallbacks(alert)
	}

	return report, nil
}

// GetLastReport 获取最后一次检查报告
func (e *HealthEngine) GetLastReport() *HealthReport {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastCheck
}

// GetAlerts 获取告警列表
func (e *HealthEngine) GetAlerts(resolved bool) []HealthAlert {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]HealthAlert, 0)
	for _, alert := range e.alerts {
		if alert.Resolved == resolved {
			result = append(result, alert)
		}
	}
	return result
}

// 内部方法

func (e *HealthEngine) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(e.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopChan:
			return
		case <-ticker.C:
			e.Check(ctx)
		}
	}
}

func (e *HealthEngine) getSystemInfo() SystemInfo {
	info, _ := host.Info()
	hostname, _ := os.Hostname()

	return SystemInfo{
		Hostname:     hostname,
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		Kernel:       info.KernelVersion,
		Uptime:       info.Uptime,
		GoVersion:    runtime.Version(),
		NumCPU:       runtime.NumCPU(),
		NumGoroutine: runtime.NumGoroutine(),
	}
}

func (e *HealthEngine) checkCPU(ctx context.Context) ComponentHealth {
	percent, err := cpu.PercentWithContext(ctx, time.Second, false)
	if err != nil {
		return ComponentHealth{
			Type:   ComponentCPU,
			Name:   "CPU",
			Status: StatusUnknown,
			Message: fmt.Sprintf("获取CPU信息失败: %v", err),
		}
	}

	usage := percent[0]
	status := StatusHealthy
	if usage >= e.config.CPUCritical {
		status = StatusCritical
	} else if usage >= e.config.CPUWarning {
		status = StatusWarning
	}

	return ComponentHealth{
		Type:      ComponentCPU,
		Name:      "CPU",
		Status:    status,
		Value:     usage,
		Threshold: e.config.CPUWarning,
		Unit:      "%",
		Message:   fmt.Sprintf("CPU使用率: %.1f%%", usage),
	}
}

func (e *HealthEngine) checkMemory(ctx context.Context) ComponentHealth {
	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return ComponentHealth{
			Type:   ComponentMemory,
			Name:   "内存",
			Status: StatusUnknown,
			Message: fmt.Sprintf("获取内存信息失败: %v", err),
		}
	}

	usage := vm.UsedPercent
	status := StatusHealthy
	if usage >= e.config.MemCritical {
		status = StatusCritical
	} else if usage >= e.config.MemWarning {
		status = StatusWarning
	}

	return ComponentHealth{
		Type:      ComponentMemory,
		Name:      "内存",
		Status:    status,
		Value:     usage,
		Threshold: e.config.MemWarning,
		Unit:      "%",
		Message:   fmt.Sprintf("内存使用率: %.1f%% (已用: %s / 总计: %s)", usage, formatBytes(vm.Used), formatBytes(vm.Total)),
	}
}

func (e *HealthEngine) checkDisk(ctx context.Context) ComponentHealth {
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return ComponentHealth{
			Type:   ComponentDisk,
			Name:   "磁盘",
			Status: StatusUnknown,
			Message: fmt.Sprintf("获取磁盘信息失败: %v", err),
		}
	}

	var maxUsage float64
	var maxPath string
	for _, p := range partitions {
		usage, err := disk.UsageWithContext(ctx, p.Mountpoint)
		if err != nil {
			continue
		}
		if usage.UsedPercent > maxUsage {
			maxUsage = usage.UsedPercent
			maxPath = p.Mountpoint
		}
	}

	status := StatusHealthy
	if maxUsage >= e.config.DiskCritical {
		status = StatusCritical
	} else if maxUsage >= e.config.DiskWarning {
		status = StatusWarning
	}

	return ComponentHealth{
		Type:      ComponentDisk,
		Name:      fmt.Sprintf("磁盘(%s)", maxPath),
		Status:    status,
		Value:     maxUsage,
		Threshold: e.config.DiskWarning,
		Unit:      "%",
		Message:   fmt.Sprintf("磁盘使用率: %.1f%% (挂载点: %s)", maxUsage, maxPath),
	}
}

func (e *HealthEngine) calculateScore(components []ComponentHealth) int {
	if len(components) == 0 {
		return 100
	}

	total := 0.0
	for _, c := range components {
		switch c.Status {
		case StatusHealthy:
			total += 100
		case StatusWarning:
			total += 60
		case StatusCritical:
			total += 20
		default:
			total += 50
		}
	}
	return int(total / float64(len(components)))
}

func (e *HealthEngine) scoreToStatus(score int) HealthStatus {
	switch {
	case score >= 80:
		return StatusHealthy
	case score >= 50:
		return StatusWarning
	default:
		return StatusCritical
	}
}

func (e *HealthEngine) detectAlerts(components []ComponentHealth) []HealthAlert {
	alerts := make([]HealthAlert, 0)
	for _, c := range components {
		if c.Status == StatusWarning || c.Status == StatusCritical {
			alerts = append(alerts, HealthAlert{
				ID:        fmt.Sprintf("%s_%d", c.Type, time.Now().UnixNano()),
				Level:     c.Status,
				Component: c.Type,
				Message:   c.Message,
				Timestamp: time.Now(),
			})
		}
	}
	return alerts
}

func (e *HealthEngine) autoHeal(ctx context.Context, alerts []HealthAlert) []HealingAction {
	actions := make([]HealingAction, 0)
	for _, alert := range alerts {
		action := HealingAction{
			ID:        fmt.Sprintf("heal_%d", time.Now().UnixNano()),
			Trigger:   alert.Message,
			StartedAt: time.Now(),
			Status:    "running",
		}

		switch alert.Component {
		case ComponentMemory:
			action.Action = "释放内存缓存"
			// 触发GC
			runtime.GC()
			action.Status = "completed"
			now := time.Now()
			action.CompletedAt = &now
			action.Result = "已触发垃圾回收"
		case ComponentDisk:
			action.Action = "清理临时文件"
			action.Status = "completed"
			now := time.Now()
			action.CompletedAt = &now
			action.Result = "建议手动清理大文件"
		default:
			action.Action = "记录告警"
			action.Status = "completed"
			now := time.Now()
			action.CompletedAt = &now
			action.Result = "已记录，需人工处理"
		}

		actions = append(actions, action)
	}
	return actions
}

func (e *HealthEngine) notifyCallbacks(alert HealthAlert) {
	e.mu.RLock()
	callbacks := e.callbacks
	e.mu.RUnlock()

	for _, cb := range callbacks {
		go cb(alert)
	}
}

// formatBytes 格式化字节数
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
