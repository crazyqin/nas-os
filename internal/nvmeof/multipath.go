package nvmeof

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"go.uber.org/zap"
)

// MultipathManager NVMe-oF多路径管理器
type MultipathManager struct {
	logger    *zap.Logger
	paths     map[string][]PathInfo
	mu        sync.RWMutex
	failover  *FailoverManager
}

// PathInfo 路径信息
type PathInfo struct {
	ID         string    `json:"id"`
	HostNQN    string    `json:"host_nqn"`
	Subsystem  string    `json:"subsystem"`
	TrAddr     string    `json:"traddr"`
	TrSvcID    string    `json:"trsvcid"`
	Transport  string    `json:"transport"` // rdma, tcp, fc
	State      string    `json:"state"`     // live, connecting, faulty, deleted
	Priority   int       `json:"priority"`
	Latency    time.Duration `json:"latency"`
	IOPS       int64     `json:"iops"`
	Bandwidth  int64     `json:"bandwidth"`
	Errors     int64     `json:"errors"`
	LastCheck  time.Time `json:"last_check"`
}

// FailoverManager 故障切换管理器
type FailoverManager struct {
	logger       *zap.Logger
	config       FailoverConfig
	activePaths  map[string]string // subsystem -> active path ID
	mu           sync.RWMutex
	failoverChan chan FailoverEvent
}

// FailoverConfig 故障切换配置
type FailoverConfig struct {
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	FailoverThreshold   int           `json:"failover_threshold"`
	AutoFailback        bool          `json:"auto_failback"`
	FailbackDelay       time.Duration `json:"failback_delay"`
	MaxFailovers        int           `json:"max_failovers"`
	NotifyOnFailover    bool          `json:"notify_on_failover"`
}

// FailoverEvent 故障切换事件
type FailoverEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	Subsystem    string    `json:"subsystem"`
	FromPath     string    `json:"from_path"`
	ToPath       string    `json:"to_path"`
	Reason       string    `json:"reason"`
	Success      bool      `json:"success"`
	Duration     time.Duration `json:"duration"`
}

// DefaultFailoverConfig 默认故障切换配置
func DefaultFailoverConfig() FailoverConfig {
	return FailoverConfig{
		HealthCheckInterval: 5 * time.Second,
		FailoverThreshold:   3,
		AutoFailback:        true,
		FailbackDelay:       30 * time.Second,
		MaxFailovers:        10,
		NotifyOnFailover:    true,
	}
}

// NewMultipathManager 创建多路径管理器
func NewMultipathManager(logger *zap.Logger, config FailoverConfig) *MultipathManager {
	mm := &MultipathManager{
		logger:   logger,
		paths:    make(map[string][]PathInfo),
		failover: NewFailoverManager(logger, config),
	}
	return mm
}

// NewFailoverManager 创建故障切换管理器
func NewFailoverManager(logger *zap.Logger, config FailoverConfig) *FailoverManager {
	return &FailoverManager{
		logger:       logger,
		config:       config,
		activePaths:  make(map[string]string),
		failoverChan: make(chan FailoverEvent, 100),
	}
}

// AddPath 添加路径
func (mm *MultipathManager) AddPath(ctx context.Context, subsystem string, path PathInfo) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	path.ID = fmt.Sprintf("%s-%s-%s", subsystem, path.TrAddr, path.Transport)
	path.LastCheck = time.Now()
	path.State = "connecting"

	mm.paths[subsystem] = append(mm.paths[subsystem], path)
	mm.logger.Info("Added NVMe-oF path",
		zap.String("subsystem", subsystem),
		zap.String("path", path.ID),
		zap.String("transport", path.Transport))

	return nil
}

// RemovePath 移除路径
func (mm *MultipathManager) RemovePath(ctx context.Context, subsystem, pathID string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	paths := mm.paths[subsystem]
	for i, p := range paths {
		if p.ID == pathID {
			mm.paths[subsystem] = append(paths[:i], paths[i+1:]...)
			mm.logger.Info("Removed NVMe-oF path", zap.String("path", pathID))
			return nil
		}
	}
	return fmt.Errorf("path %s not found", pathID)
}

// GetPaths 获取子系统的所有路径
func (mm *MultipathManager) GetPaths(subsystem string) []PathInfo {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.paths[subsystem]
}

// GetActivePath 获取活跃路径
func (mm *MultipathManager) GetActivePath(subsystem string) *PathInfo {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	activeID := mm.failover.activePaths[subsystem]
	paths := mm.paths[subsystem]

	for _, p := range paths {
		if p.ID == activeID && p.State == "live" {
			return &p
		}
	}

	// Fallback to first live path
	for _, p := range paths {
		if p.State == "live" {
			return &p
		}
	}
	return nil
}

// CheckPathHealth 检查路径健康状态
func (mm *MultipathManager) CheckPathHealth(ctx context.Context, subsystem, pathID string) (*PathInfo, error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	paths := mm.paths[subsystem]
	for i, p := range paths {
		if p.ID == pathID {
			// 实际检查路径状态
			state := mm.checkPathState(ctx, p)
			paths[i].State = state
			paths[i].LastCheck = time.Now()

			if state == "faulty" {
				mm.logger.Warn("NVMe-oF path faulty",
					zap.String("path", pathID),
					zap.String("subsystem", subsystem))
			}
			return &paths[i], nil
		}
	}
	return nil, fmt.Errorf("path %s not found", pathID)
}

func (mm *MultipathManager) checkPathState(ctx context.Context, path PathInfo) string {
	// 使用nvme-cli检查路径状态
	cmd := exec.CommandContext(ctx, "nvme", "connect", "-t", path.Transport,
		"-a", path.TrAddr, "-s", path.TrSvcID, "-n", path.Subsystem, "--dry-run")
	if err := cmd.Run(); err != nil {
		return "faulty"
	}
	return "live"
}

// StartHealthMonitor 启动健康监控
func (mm *MultipathManager) StartHealthMonitor(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				mm.checkAllPaths(ctx)
			}
		}
	}()
}

func (mm *MultipathManager) checkAllPaths(ctx context.Context) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	for subsystem, paths := range mm.paths {
		for i, p := range paths {
			state := mm.checkPathState(ctx, p)
			paths[i].State = state
			paths[i].LastCheck = time.Now()

			if state == "faulty" {
				mm.logger.Warn("Path faulty, triggering failover",
					zap.String("subsystem", subsystem),
					zap.String("path", p.ID))
				mm.failover.TriggerFailover(subsystem, p.ID, "health_check_failed")
			}
		}
	}
}

// TriggerFailover 触发故障切换
func (fm *FailoverManager) TriggerFailover(subsystem, fromPath, reason string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	event := FailoverEvent{
		Timestamp: time.Now(),
		Subsystem: subsystem,
		FromPath:  fromPath,
		Reason:    reason,
	}

	fm.logger.Warn("NVMe-oF failover triggered",
		zap.String("subsystem", subsystem),
		zap.String("from", fromPath),
		zap.String("reason", reason))

	select {
	case fm.failoverChan <- event:
	default:
		fm.logger.Error("Failover channel full, dropping event")
	}
}

// GetFailoverEvents 获取故障切换事件
func (fm *FailoverManager) GetFailoverEvents() []FailoverEvent {
	var events []FailoverEvent
	for {
		select {
		case event := <-fm.failoverChan:
			events = append(events, event)
		default:
			return events
		}
	}
}

// RDMAConfig RDMA配置
type RDMAConfig struct {
	GIDIndex    int    `json:"gid_index"`
	MTU         int    `json:"mtu"`
	QueueSize   int    `json:"queue_size"`
	MaxSegments int    `json:"max_segments"`
}

// ConnectRDMA 通过RDMA连接NVMe-oF
func (mm *MultipathManager) ConnectRDMA(ctx context.Context, subsystem, addr, port string, config RDMAConfig) error {
	args := []string{"connect",
		"-t", "rdma",
		"-a", addr,
		"-s", port,
		"-n", subsystem,
		"-q", fmt.Sprintf("%d", config.QueueSize),
	}

	cmd := exec.CommandContext(ctx, "nvme", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nvme connect rdma failed: %s: %w", string(output), err)
	}

	mm.logger.Info("NVMe-oF RDMA connected",
		zap.String("subsystem", subsystem),
		zap.String("addr", addr),
		zap.String("port", port))

	return nil
}

// ConnectTCP 通过TCP连接NVMe-oF
func (mm *MultipathManager) ConnectTCP(ctx context.Context, subsystem, addr, port string) error {
	args := []string{"connect",
		"-t", "tcp",
		"-a", addr,
		"-s", port,
		"-n", subsystem,
	}

	cmd := exec.CommandContext(ctx, "nvme", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nvme connect tcp failed: %s: %w", string(output), err)
	}

	mm.logger.Info("NVMe-oF TCP connected",
		zap.String("subsystem", subsystem),
		zap.String("addr", addr))

	return nil
}

// Disconnect 断开NVMe-oF连接
func (mm *MultipathManager) Disconnect(ctx context.Context, subsystemNQN string) error {
	cmd := exec.CommandContext(ctx, "nvme", "disconnect", "-n", subsystemNQN)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nvme disconnect failed: %s: %w", string(output), err)
	}
	return nil
}

// GetSubsystemIOStats 获取子系统IO统计
func (mm *MultipathManager) GetSubsystemIOStats(ctx context.Context, subsystem string) (map[string]interface{}, error) {
	cmd := exec.CommandContext(ctx, "nvme", "id-ctrl", subsystem, "-o", "json")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("nvme id-ctrl failed: %w", err)
	}
	return map[string]interface{}{
		"raw": string(out),
	}, nil
}
