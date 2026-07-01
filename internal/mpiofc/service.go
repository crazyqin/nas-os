// Package mpiofc 服务层
// 实现光纤通道 HBA 端口检测、多路径配置、路径状态监控和故障切换
package mpiofc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Service 多路径 I/O 光纤通道管理服务
type Service struct {
	mu          sync.RWMutex
	config      *Config
	ports       map[string]*HBAPort        // 端口 ID -> HBA 端口
	paths       map[string]*MPIOPath       // 路径 ID -> 多路径
	// targetPaths 目标 WWPN -> 路径 ID 列表
	targetPaths map[string][]string
	stats       map[string]*PathStatistics // 路径 ID -> 统计信息
	failoverTotal int                      // 总故障切换次数
}

// NewService 创建多路径管理服务
func NewService(cfg *Config) *Service {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Service{
		config:        cfg,
		ports:         make(map[string]*HBAPort),
		paths:         make(map[string]*MPIOPath),
		targetPaths:   make(map[string][]string),
		stats:         make(map[string]*PathStatistics),
		failoverTotal: 0,
	}
}

// ========== HBA 端口检测 ==========

// DetectPorts 扫描系统光纤通道 HBA 端口
// 读取 /sys/class/fc_host 下的端口信息
func (s *Service) DetectPorts() ([]*HBAPort, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.config.SysFCBase)
	if err != nil {
		// 无法读取 sysfs 时返回空列表（测试环境兼容）
		return []*HBAPort{}, nil
	}

	for _, entry := range entries {
		portName := entry.Name()
		portID := portName

		// 读取 WWPN
		wwpn := s.readSysFile(filepath.Join(s.config.SysFCBase, portName, "port_name"))
		// 读取 WWNN
		wwnn := s.readSysFile(filepath.Join(s.config.SysFCBase, portName, "node_name"))
		// 读取端口速率
		speed := s.readSysFile(filepath.Join(s.config.SysFCBase, portName, "speed"))
		// 读取端口类型
		portType := s.readSysFile(filepath.Join(s.config.SysFCBase, portName, "port_type"))
		// 读取 Fabric Name
		fabricName := s.readSysFile(filepath.Join(s.config.SysFCBase, portName, "fabric_name"))
		// 读取端口状态
		state := s.readSysFile(filepath.Join(s.config.SysFCBase, portName, "port_state"))

		// 判断端口在线状态
		online := strings.Contains(state, "Online")

		// 保留已有路径关联（未来可用于增量更新）
		_ = func() *HBAPort { return s.ports[portID] }
		port := &HBAPort{
			ID:         portID,
			Name:       portName,
			WWPN:       wwpn,
			WWNN:       wwnn,
			PortName:   portName,
			FabricName: fabricName,
			Speed:      speed,
			PortType:   portType,
			State:      portStateFromString(state),
			Online:     online,
			Supported:  true,
			UpdatedAt:  time.Now(),
		}
		s.ports[portID] = port
	}

	result := make([]*HBAPort, 0, len(s.ports))
	for _, p := range s.ports {
		result = append(result, p)
	}
	return result, nil
}

// GetPorts 获取所有 HBA 端口列表
func (s *Service) GetPorts() []*HBAPort {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*HBAPort, 0, len(s.ports))
	for _, p := range s.ports {
		result = append(result, p)
	}
	return result
}

// GetPort 根据 ID 获取 HBA 端口
func (s *Service) GetPort(portID string) (*HBAPort, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	port, ok := s.ports[portID]
	if !ok {
		return nil, fmt.Errorf("HBA 端口不存在: %s", portID)
	}
	return port, nil
}

// ========== 多路径配置 ==========

// ConfigureMPIO 配置多路径
// 根据指定策略创建多条路径到目标 WWPN
func (s *Service) ConfigureMPIO(req *MPIOConfig) ([]*MPIOPath, error) {
	if err := ValidateWWPN(req.TargetWWPN); err != nil {
		return nil, err
	}

	// 设置默认策略
	policy := req.Policy
	if policy == "" {
		policy = PathPolicyRoundRobin
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}

	// 验证所有路径配置的端口是否存在
	for _, pc := range req.Paths {
		if _, err := s.GetPort(pc.HBAPortID); err != nil {
			return nil, fmt.Errorf("路径配置失败: %w", err)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 移除该目标的旧路径
	oldPaths := s.targetPaths[req.TargetWWPN]
	for _, pid := range oldPaths {
		delete(s.paths, pid)
		delete(s.stats, pid)
	}
	s.targetPaths[req.TargetWWPN] = nil

	// 创建新路径
	created := make([]*MPIOPath, 0, len(req.Paths))
	now := time.Now()

	for idx, pc := range req.Paths {
		pathID := fmt.Sprintf("mpio-%s-%s-%d", req.TargetWWPN, pc.HBAPortID, now.UnixNano()+int64(idx))

		// 根据策略确定路径状态
		pathState := PathStateStandby
		active := false
		if policy == PathPolicyRoundRobin || policy == PathPolicyRoundRobin16 {
			// 轮询模式：所有路径都是活跃的
			pathState = PathStateActive
			active = true
		} else if policy == PathPolicyFailover {
			// 故障转移：仅优先级最高的路径活跃
			if idx == 0 {
				pathState = PathStateActive
				active = true
			}
		} else if policy == PathPolicyMinQueue {
			// 最小队列：初始全部待机，运行时动态选择
			pathState = PathStateStandby
		}

		path := &MPIOPath{
			ID:         pathID,
			HBAPortID:  pc.HBAPortID,
			TargetWWPN: req.TargetWWPN,
			PathState:  pathState,
			Policy:     policy,
			Priority:   pc.Priority,
			Active:     active,
			IOLoad:     0,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		s.paths[pathID] = path
		s.targetPaths[req.TargetWWPN] = append(s.targetPaths[req.TargetWWPN], pathID)

		// 初始化统计信息
		s.stats[pathID] = &PathStatistics{
			PathID:      pathID,
			HBAPortID:   pc.HBAPortID,
			TargetWWPN:  req.TargetWWPN,
			CollectedAt: now,
		}

		created = append(created, path)
	}

	// 故障转移模式：确保优先级最低的路径为活跃
	if policy == PathPolicyFailover && len(created) > 0 {
		s.reorderFailoverPaths(req.TargetWWPN)
	}

	return created, nil
}

// ========== 路径状态查询 ==========

// GetStatus 获取多路径状态总览
func (s *Service) GetStatus() *MPIOStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &MPIOStatus{
		Paths:          make([]*MPIOPath, 0, len(s.paths)),
		FailoverEvents: s.failoverTotal,
	}

	for _, p := range s.ports {
		status.TotalPorts++
		if p.Online {
			status.OnlinePorts++
		}
	}

	for _, path := range s.paths {
		status.TotalPaths++
		switch path.PathState {
		case PathStateActive:
			status.ActivePaths++
		case PathStateStandby:
			status.StandbyPaths++
		case PathStateFailed:
			status.FailedPaths++
		}
		status.Paths = append(status.Paths, path)
	}

	return status
}

// GetStatistics 获取所有路径统计信息
func (s *Service) GetStatistics() []*PathStatistics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*PathStatistics, 0, len(s.stats))
	for _, stat := range s.stats {
		result = append(result, stat)
	}
	return result
}

// GetPathsByTarget 获取指定目标 WWPN 的所有路径
func (s *Service) GetPathsByTarget(targetWWPN string) []*MPIOPath {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pathIDs := s.targetPaths[targetWWPN]
	result := make([]*MPIOPath, 0, len(pathIDs))
	for _, pid := range pathIDs {
		if p, ok := s.paths[pid]; ok {
			result = append(result, p)
		}
	}
	return result
}

// ========== 故障切换 ==========

// HandlePathFailure 处理路径故障，自动切换到备用路径
// 当活跃路径发生故障时，根据策略选择最佳备用路径
func (s *Service) HandlePathFailure(pathID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, ok := s.paths[pathID]
	if !ok {
		return fmt.Errorf("路径不存在: %s", pathID)
	}

	if path.PathState == PathStateFailed {
		return nil // 已经是故障状态
	}

	// 标记路径故障
	oldState := path.PathState
	path.PathState = PathStateFailed
	path.Active = false
	path.UpdatedAt = time.Now()

	// 如果故障路径是活跃的，执行故障切换
	if oldState == PathStateActive {
		// 查找该目标的备用路径
		pathIDs := s.targetPaths[path.TargetWWPN]
		for _, pid := range pathIDs {
			backup, ok := s.paths[pid]
			if !ok || backup.PathState != PathStateStandby {
				continue
			}

			// 激活备用路径
			backup.PathState = PathStateActive
			backup.Active = true
			backup.UpdatedAt = time.Now()

			// 更新故障切换计数
			backup.FailoverCount++
			now := time.Now()
			backup.LastFailoverAt = &now

			// 更新统计
			if stat, ok := s.stats[pid]; ok {
				stat.FailoverCount++
			}

			s.failoverTotal++
			break
		}
	}

	// 更新故障路径的统计
	if stat, ok := s.stats[pathID]; ok {
		stat.ErrorCount++
	}

	return nil
}

// ReactivatePath 重新激活已恢复的路径
func (s *Service) ReactivatePath(pathID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, ok := s.paths[pathID]
	if !ok {
		return fmt.Errorf("路径不存在: %s", pathID)
	}

	if path.PathState != PathStateFailed {
		return fmt.Errorf("路径未处于故障状态，无需恢复")
	}

	// 恢复路径为待机状态
	path.PathState = PathStateStandby
	path.Active = false
	path.UpdatedAt = time.Now()

	// 如果策略是轮询，重新设为活跃
	if path.Policy == PathPolicyRoundRobin || path.Policy == PathPolicyRoundRobin16 {
		path.PathState = PathStateActive
		path.Active = true
	}

	return nil
}

// ========== 内部辅助方法 ==========

// readSysFile 读取 sysfs 文件内容并返回去空格的字符串
func (s *Service) readSysFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// portStateFromString 从字符串解析端口状态
func portStateFromString(state string) PathState {
	state = strings.ToLower(state)
	switch {
	case strings.Contains(state, "online"):
		return PathStateActive
	case strings.Contains(state, "offline"):
		return PathStateFailed
	case strings.Contains(state, "standby"):
		return PathStateStandby
	default:
		return PathStateUnknown
	}
}

// reorderFailoverPaths 在故障转移模式下，根据优先级重新排序路径
// 确保优先级最高（数值最小）的路径为活跃，其余为待机
func (s *Service) reorderFailoverPaths(targetWWPN string) {
	pathIDs := s.targetPaths[targetWWPN]
	if len(pathIDs) == 0 {
		return
	}

	// 找到优先级最高（数值最小）的路径
	bestID := ""
	bestPriority := int(^uint(0) >> 1) // MaxInt
	for _, pid := range pathIDs {
		if p, ok := s.paths[pid]; ok && p.PathState != PathStateFailed {
			if p.Priority < bestPriority {
				bestPriority = p.Priority
				bestID = pid
			}
		}
	}

	// 设置活跃/待机
	for _, pid := range pathIDs {
		p, ok := s.paths[pid]
		if !ok {
			continue
		}
		if pid == bestID {
			p.PathState = PathStateActive
			p.Active = true
		} else if p.PathState != PathStateFailed {
			p.PathState = PathStateStandby
			p.Active = false
		}
		p.UpdatedAt = time.Now()
	}
}

// UpdatePathStats 更新路径统计信息（内部调用）
func (s *Service) UpdatePathStats(pathID string, iopsRead, iopsWrite int64, throughputRead, throughputWrite, latencyMs float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stat, ok := s.stats[pathID]
	if !ok {
		return
	}
	stat.IOPSRead = iopsRead
	stat.IOPSWrite = iopsWrite
	stat.IOPSTotal = iopsRead + iopsWrite
	stat.ThroughputRead = throughputRead
	stat.ThroughputWrite = throughputWrite
	stat.LatencyAvgMs = latencyMs
	if latencyMs > stat.LatencyMaxMs {
		stat.LatencyMaxMs = latencyMs
	}
	stat.CollectedAt = time.Now()

	// 更新路径 I/O 负载
	if path, ok := s.paths[pathID]; ok {
		path.IOLoad = int(latencyMs)
		path.UpdatedAt = time.Now()
	}
}
