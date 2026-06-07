package storageqos

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TargetManager 目标管理器
type TargetManager struct {
	mu      sync.RWMutex
	targets map[string]*QoSTarget
}

// NewTargetManager 创建目标管理器
func NewTargetManager() *TargetManager {
	return &TargetManager{
		targets: make(map[string]*QoSTarget),
	}
}

// RegisterTarget 注册目标
func (tm *TargetManager) RegisterTarget(target *QoSTarget) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if target.ID == "" {
		return fmt.Errorf("目标ID不能为空")
	}
	if target.Type == "" {
		return fmt.Errorf("目标类型不能为空")
	}
	if target.Type != "volume" && target.Type != "share" && target.Type != "container" {
		return fmt.Errorf("无效的目标类型: %s", target.Type)
	}

	tm.targets[target.ID] = target
	return nil
}

// UnregisterTarget 注销目标
func (tm *TargetManager) UnregisterTarget(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.targets[id]; !exists {
		return fmt.Errorf("目标不存在: %s", id)
	}

	delete(tm.targets, id)
	return nil
}

// GetTarget 获取目标
func (tm *TargetManager) GetTarget(id string) (*QoSTarget, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	target, exists := tm.targets[id]
	if !exists {
		return nil, fmt.Errorf("目标不存在: %s", id)
	}

	return target, nil
}

// ListTargets 列出所有目标
func (tm *TargetManager) ListTargets() []*QoSTarget {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	targets := make([]*QoSTarget, 0, len(tm.targets))
	for _, target := range tm.targets {
		targets = append(targets, target)
	}
	return targets
}

// GetTargetsByType 按类型获取目标
func (tm *TargetManager) GetTargetsByType(targetType string) []*QoSTarget {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var targets []*QoSTarget
	for _, target := range tm.targets {
		if target.Type == targetType {
			targets = append(targets, target)
		}
	}
	return targets
}

// MetricsCollector 指标采集器
type MetricsCollector struct {
	mu        sync.RWMutex
	metrics   map[string]*QoSMetrics
	lastStats map[string]*DiskStats
	stopCh    chan struct{}
	interval  time.Duration
}

// DiskStats 磁盘统计信息
type DiskStats struct {
	ReadsCompleted  int64
	ReadsMerged     int64
	ReadSectors     int64
	ReadTime        int64
	WritesCompleted int64
	WritesMerged    int64
	WriteSectors    int64
	WriteTime       int64
	IOInProgress    int64
	IOTime          int64
	WeightedIOTime  int64
	Timestamp       time.Time
}

// NewMetricsCollector 创建指标采集器
func NewMetricsCollector(interval time.Duration) *MetricsCollector {
	return &MetricsCollector{
		metrics:   make(map[string]*QoSMetrics),
		lastStats: make(map[string]*DiskStats),
		stopCh:    make(chan struct{}),
		interval:  interval,
	}
}

// Start 启动指标采集
func (mc *MetricsCollector) Start() {
	go mc.collectLoop()
}

// Stop 停止指标采集
func (mc *MetricsCollector) Stop() {
	close(mc.stopCh)
}

// collectLoop 采集循环
func (mc *MetricsCollector) collectLoop() {
	ticker := time.NewTicker(mc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-mc.stopCh:
			return
		case <-ticker.C:
			mc.collectAll()
		}
	}
}

// collectAll 采集所有指标
func (mc *MetricsCollector) collectAll() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// 从 /proc/diskstats 采集
	stats, err := readDiskStats()
	if err != nil {
		return
	}

	for device, stat := range stats {
		// 计算IOPS
		lastStat, exists := mc.lastStats[device]
		if !exists {
			mc.lastStats[device] = stat
			continue
		}

		timeDiff := stat.Timestamp.Sub(lastStat.Timestamp).Seconds()
		if timeDiff <= 0 {
			continue
		}

		readIOPS := float64(stat.ReadsCompleted-lastStat.ReadsCompleted) / timeDiff
		writeIOPS := float64(stat.WritesCompleted-lastStat.WritesCompleted) / timeDiff

		// 计算带宽 (转换为MB/s)
		readBW := float64(stat.ReadSectors-lastStat.ReadSectors) * 512 / 1024 / 1024 / timeDiff
		writeBW := float64(stat.WriteSectors-lastStat.WriteSectors) * 512 / 1024 / 1024 / timeDiff

		// 计算延迟
		readLatency := int64(0)
		if stat.ReadsCompleted > lastStat.ReadsCompleted {
			readLatency = (stat.ReadTime - lastStat.ReadTime) / (stat.ReadsCompleted - lastStat.ReadsCompleted)
		}
		writeLatency := int64(0)
		if stat.WritesCompleted > lastStat.WritesCompleted {
			writeLatency = (stat.WriteTime - lastStat.WriteTime) / (stat.WritesCompleted - lastStat.WritesCompleted)
		}
		latency := (readLatency + writeLatency) / 2

		// 计算利用率
		utilization := float64(stat.IOTime-lastStat.IOTime) / (timeDiff * 1000) * 100
		if utilization > 100 {
			utilization = 100
		}

		mc.metrics[device] = &QoSMetrics{
			TargetID:    device,
			IOPS:        int64(readIOPS + writeIOPS),
			ReadIOPS:    int64(readIOPS),
			WriteIOPS:   int64(writeIOPS),
			Bandwidth:   int64(readBW + writeBW),
			ReadBW:      int64(readBW),
			WriteBW:     int64(writeBW),
			Latency:     latency,
			QueueDepth:  stat.IOInProgress,
			Utilization: utilization,
			Timestamp:   time.Now(),
		}

		mc.lastStats[device] = stat
	}
}

// GetMetrics 获取指标
func (mc *MetricsCollector) GetMetrics(targetID string) (*QoSMetrics, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	metrics, exists := mc.metrics[targetID]
	if !exists {
		return nil, fmt.Errorf("指标不存在: %s", targetID)
	}

	return metrics, nil
}

// GetAllMetrics 获取所有指标
func (mc *MetricsCollector) GetAllMetrics() map[string]*QoSMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make(map[string]*QoSMetrics)
	for k, v := range mc.metrics {
		result[k] = v
	}
	return result
}

// readDiskStats 读取 /proc/diskstats
func readDiskStats() (map[string]*DiskStats, error) {
	file, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, fmt.Errorf("无法打开 /proc/diskstats: %w", err)
	}
	defer file.Close()

	stats := make(map[string]*DiskStats)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 14 {
			continue
		}

		device := fields[2]
		// 跳过分区，只统计磁盘
		if strings.Contains(device, "p") {
			continue
		}

		readCompleted, _ := strconv.ParseInt(fields[3], 10, 64)
		readMerged, _ := strconv.ParseInt(fields[4], 10, 64)
		readSectors, _ := strconv.ParseInt(fields[5], 10, 64)
		readTime, _ := strconv.ParseInt(fields[6], 10, 64)
		writeCompleted, _ := strconv.ParseInt(fields[7], 10, 64)
		writeMerged, _ := strconv.ParseInt(fields[8], 10, 64)
		writeSectors, _ := strconv.ParseInt(fields[9], 10, 64)
		writeTime, _ := strconv.ParseInt(fields[10], 10, 64)
		ioInProgress, _ := strconv.ParseInt(fields[11], 10, 64)
		ioTime, _ := strconv.ParseInt(fields[12], 10, 64)
		weightedIOTime, _ := strconv.ParseInt(fields[13], 10, 64)

		stats[device] = &DiskStats{
			ReadsCompleted:  readCompleted,
			ReadsMerged:     readMerged,
			ReadSectors:     readSectors,
			ReadTime:        readTime,
			WritesCompleted: writeCompleted,
			WritesMerged:    writeMerged,
			WriteSectors:    writeSectors,
			WriteTime:       writeTime,
			IOInProgress:    ioInProgress,
			IOTime:          ioTime,
			WeightedIOTime:  weightedIOTime,
			Timestamp:       time.Now(),
		}
	}

	return stats, nil
}

// ViolationDetector 违规检测器
type ViolationDetector struct {
	mu         sync.RWMutex
	violations []*QoSViolation
	manager    *QoSManager
	collector  *MetricsCollector
	alertFunc  func(violation *QoSViolation)
	stopCh     chan struct{}
}

// NewViolationDetector 创建违规检测器
func NewViolationDetector(manager *QoSManager, collector *MetricsCollector, alertFunc func(violation *QoSViolation)) *ViolationDetector {
	return &ViolationDetector{
		violations: make([]*QoSViolation, 0),
		manager:    manager,
		collector:  collector,
		alertFunc:  alertFunc,
		stopCh:     make(chan struct{}),
	}
}

// Start 启动违规检测
func (vd *ViolationDetector) Start() {
	go vd.detectLoop()
}

// Stop 停止违规检测
func (vd *ViolationDetector) Stop() {
	close(vd.stopCh)
}

// detectLoop 检测循环
func (vd *ViolationDetector) detectLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-vd.stopCh:
			return
		case <-ticker.C:
			vd.detect()
		}
	}
}

// detect 检测违规
func (vd *ViolationDetector) detect() {
	vd.mu.Lock()
	defer vd.mu.Unlock()

	policies := vd.manager.GetEnabledPolicies()
	for _, policy := range policies {
		// 获取目标指标
		metrics, err := vd.collector.GetMetrics(policy.TargetID)
		if err != nil {
			continue
		}

		// 检查IOPS违规
		if policy.MaxIOPS > 0 && metrics.IOPS > policy.MaxIOPS {
			violation := &QoSViolation{
				ID:         fmt.Sprintf("v_%d", time.Now().UnixNano()),
				PolicyID:   policy.ID,
				PolicyName: policy.Name,
				TargetID:   policy.TargetID,
				Type:       "iops_exceeded",
				Threshold:  policy.MaxIOPS,
				Actual:     metrics.IOPS,
				Message:    fmt.Sprintf("IOPS超过限制: 当前 %d, 上限 %d", metrics.IOPS, policy.MaxIOPS),
				Severity:   vd.getSeverity(metrics.IOPS, policy.MaxIOPS),
				Timestamp:  time.Now(),
			}
			vd.addViolation(violation)
		}

		// 检查带宽违规
		if policy.MaxBandwidth > 0 && metrics.Bandwidth > policy.MaxBandwidth {
			violation := &QoSViolation{
				ID:         fmt.Sprintf("v_%d", time.Now().UnixNano()),
				PolicyID:   policy.ID,
				PolicyName: policy.Name,
				TargetID:   policy.TargetID,
				Type:       "bandwidth_exceeded",
				Threshold:  policy.MaxBandwidth,
				Actual:     metrics.Bandwidth,
				Message:    fmt.Sprintf("带宽超过限制: 当前 %d MB/s, 上限 %d MB/s", metrics.Bandwidth, policy.MaxBandwidth),
				Severity:   vd.getSeverity(metrics.Bandwidth, policy.MaxBandwidth),
				Timestamp:  time.Now(),
			}
			vd.addViolation(violation)
		}

		// 检查延迟违规
		if policy.LatencyMax > 0 && metrics.Latency > policy.LatencyMax {
			violation := &QoSViolation{
				ID:         fmt.Sprintf("v_%d", time.Now().UnixNano()),
				PolicyID:   policy.ID,
				PolicyName: policy.Name,
				TargetID:   policy.TargetID,
				Type:       "latency_exceeded",
				Threshold:  policy.LatencyMax,
				Actual:     metrics.Latency,
				Message:    fmt.Sprintf("延迟超过阈值: 当前 %d ms, 阈值 %d ms", metrics.Latency, policy.LatencyMax),
				Severity:   vd.getSeverity(metrics.Latency, policy.LatencyMax),
				Timestamp:  time.Now(),
			}
			vd.addViolation(violation)
		}
	}
}

// getSeverity 获取严重程度
func (vd *ViolationDetector) getSeverity(actual, threshold int64) string {
	ratio := float64(actual) / float64(threshold)
	if ratio > 1.5 {
		return "critical"
	}
	return "warning"
}

// addViolation 添加违规记录
func (vd *ViolationDetector) addViolation(violation *QoSViolation) {
	vd.violations = append(vd.violations, violation)

	// 限制历史记录数量
	if len(vd.violations) > vd.manager.config.ViolationHistory {
		vd.violations = vd.violations[1:]
	}

	// 触发告警
	if vd.alertFunc != nil {
		vd.alertFunc(violation)
	}
}

// GetViolations 获取违规记录
func (vd *ViolationDetector) GetViolations() []*QoSViolation {
	vd.mu.RLock()
	defer vd.mu.RUnlock()

	result := make([]*QoSViolation, len(vd.violations))
	copy(result, vd.violations)
	return result
}

// GetViolationsByPolicy 按策略获取违规记录
func (vd *ViolationDetector) GetViolationsByPolicy(policyID string) []*QoSViolation {
	vd.mu.RLock()
	defer vd.mu.RUnlock()

	var result []*QoSViolation
	for _, v := range vd.violations {
		if v.PolicyID == policyID {
			result = append(result, v)
		}
	}
	return result
}

// GetUnresolvedViolations 获取未解决的违规
func (vd *ViolationDetector) GetUnresolvedViolations() []*QoSViolation {
	vd.mu.RLock()
	defer vd.mu.RUnlock()

	var result []*QoSViolation
	for _, v := range vd.violations {
		if !v.Resolved {
			result = append(result, v)
		}
	}
	return result
}

// ResolveViolation 解决违规
func (vd *ViolationDetector) ResolveViolation(id string) error {
	vd.mu.Lock()
	defer vd.mu.Unlock()

	for _, v := range vd.violations {
		if v.ID == id {
			v.Resolved = true
			now := time.Now()
			v.ResolvedAt = &now
			return nil
		}
	}
	return fmt.Errorf("违规记录不存在: %s", id)
}

// IOController IO控制器
type IOController struct {
	mu     sync.RWMutex
	limits map[string]*IOLimit
}

// IOLimit IO限制
type IOLimit struct {
	TargetID     string
	DevicePath   string
	CGroupPath   string
	MaxIOPS      int64
	MaxBandwidth int64
}

// NewIOController 创建IO控制器
func NewIOController() *IOController {
	return &IOController{
		limits: make(map[string]*IOLimit),
	}
}

// SetIOPSLimit 设置IOPS限制
func (ic *IOController) SetIOPSLimit(targetID, devicePath string, maxIOPS int64) error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	// 尝试通过 cgroup blkio 设置限制
	cgroupPath := fmt.Sprintf("/sys/fs/cgroup/blkio/storageqos/%s", targetID)
	if err := os.MkdirAll(cgroupPath, 0755); err != nil {
		return fmt.Errorf("创建cgroup目录失败: %w", err)
	}

	// 设置 blkio.throttle.read_iops_device
	readLimitPath := filepath.Join(cgroupPath, "blkio.throttle.read_iops_device")
	writeLimitPath := filepath.Join(cgroupPath, "blkio.throttle.write_iops_device")

	// 获取设备号
	deviceNum, err := getDeviceNumber(devicePath)
	if err != nil {
		return fmt.Errorf("获取设备号失败: %w", err)
	}

	limitStr := fmt.Sprintf("%s %d", deviceNum, maxIOPS)

	if err := os.WriteFile(readLimitPath, []byte(limitStr), 0644); err != nil {
		return fmt.Errorf("设置读IOPS限制失败: %w", err)
	}
	if err := os.WriteFile(writeLimitPath, []byte(limitStr), 0644); err != nil {
		return fmt.Errorf("设置写IOPS限制失败: %w", err)
	}

	ic.limits[targetID] = &IOLimit{
		TargetID:   targetID,
		DevicePath: devicePath,
		CGroupPath: cgroupPath,
		MaxIOPS:    maxIOPS,
	}

	return nil
}

// SetBandwidthLimit 设置带宽限制
func (ic *IOController) SetBandwidthLimit(targetID, devicePath string, maxBandwidth int64) error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	// 尝试通过 cgroup blkio 设置限制
	cgroupPath := fmt.Sprintf("/sys/fs/cgroup/blkio/storageqos/%s", targetID)
	if err := os.MkdirAll(cgroupPath, 0755); err != nil {
		return fmt.Errorf("创建cgroup目录失败: %w", err)
	}

	// 设置 blkio.throttle.read_bps_device
	readLimitPath := filepath.Join(cgroupPath, "blkio.throttle.read_bps_device")
	writeLimitPath := filepath.Join(cgroupPath, "blkio.throttle.write_bps_device")

	// 获取设备号
	deviceNum, err := getDeviceNumber(devicePath)
	if err != nil {
		return fmt.Errorf("获取设备号失败: %w", err)
	}

	// 转换为字节/秒
	maxBandwidthBytes := maxBandwidth * 1024 * 1024
	limitStr := fmt.Sprintf("%s %d", deviceNum, maxBandwidthBytes)

	if err := os.WriteFile(readLimitPath, []byte(limitStr), 0644); err != nil {
		return fmt.Errorf("设置读带宽限制失败: %w", err)
	}
	if err := os.WriteFile(writeLimitPath, []byte(limitStr), 0644); err != nil {
		return fmt.Errorf("设置写带宽限制失败: %w", err)
	}

	if limit, exists := ic.limits[targetID]; exists {
		limit.MaxBandwidth = maxBandwidth
	} else {
		ic.limits[targetID] = &IOLimit{
			TargetID:     targetID,
			DevicePath:   devicePath,
			CGroupPath:   cgroupPath,
			MaxBandwidth: maxBandwidth,
		}
	}

	return nil
}

// RemoveIOLimit 移除IO限制
func (ic *IOController) RemoveIOLimit(targetID string) error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	limit, exists := ic.limits[targetID]
	if !exists {
		return fmt.Errorf("IO限制不存在: %s", targetID)
	}

	// 清理cgroup目录
	if err := os.RemoveAll(limit.CGroupPath); err != nil {
		return fmt.Errorf("清理cgroup目录失败: %w", err)
	}

	delete(ic.limits, targetID)
	return nil
}

// GetIOLimit 获取IO限制
func (ic *IOController) GetIOLimit(targetID string) (*IOLimit, error) {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	limit, exists := ic.limits[targetID]
	if !exists {
		return nil, fmt.Errorf("IO限制不存在: %s", targetID)
	}

	return limit, nil
}

// ListIOLimits 列出所有IO限制
func (ic *IOController) ListIOLimits() []*IOLimit {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	limits := make([]*IOLimit, 0, len(ic.limits))
	for _, limit := range ic.limits {
		limits = append(limits, limit)
	}
	return limits
}

// getDeviceNumber 获取设备号
func getDeviceNumber(devicePath string) (string, error) {
	// 读取 /sys/block/ 下的设备信息
	deviceName := filepath.Base(devicePath)
	devPath := fmt.Sprintf("/sys/block/%s/dev", deviceName)

	data, err := os.ReadFile(devPath)
	if err != nil {
		return "", fmt.Errorf("读取设备号失败: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}

// AdaptiveQoS 自适应QoS
type AdaptiveQoS struct {
	mu        sync.RWMutex
	manager   *QoSManager
	collector *MetricsCollector
	stopCh    chan struct{}
}

// NewAdaptiveQoS 创建自适应QoS
func NewAdaptiveQoS(manager *QoSManager, collector *MetricsCollector) *AdaptiveQoS {
	return &AdaptiveQoS{
		manager:   manager,
		collector: collector,
		stopCh:    make(chan struct{}),
	}
}

// Start 启动自适应QoS
func (aq *AdaptiveQoS) Start() {
	go aq.adaptLoop()
}

// Stop 停止自适应QoS
func (aq *AdaptiveQoS) Stop() {
	close(aq.stopCh)
}

// adaptLoop 自适应循环
func (aq *AdaptiveQoS) adaptLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-aq.stopCh:
			return
		case <-ticker.C:
			aq.adapt()
		}
	}
}

// adapt 执行自适应调整
func (aq *AdaptiveQoS) adapt() {
	aq.mu.Lock()
	defer aq.mu.Unlock()

	policies := aq.manager.GetEnabledPolicies()
	for _, policy := range policies {
		if !policy.Adaptive {
			continue
		}

		metrics, err := aq.collector.GetMetrics(policy.TargetID)
		if err != nil {
			continue
		}

		// 根据利用率调整限制
		if metrics.Utilization > 80 {
			// 高负载 - 降低限制
			aq.adjustLimits(policy, 0.8)
		} else if metrics.Utilization < 30 {
			// 低负载 - 提高限制
			aq.adjustLimits(policy, 1.2)
		}
	}
}

// adjustLimits 调整限制
func (aq *AdaptiveQoS) adjustLimits(policy *QoSPolicy, factor float64) {
	if policy.MaxIOPS > 0 {
		newMaxIOPS := int64(float64(policy.MaxIOPS) * factor)
		if newMaxIOPS < 100 {
			newMaxIOPS = 100
		}
		policy.MaxIOPS = newMaxIOPS
	}

	if policy.MaxBandwidth > 0 {
		newMaxBW := int64(float64(policy.MaxBandwidth) * factor)
		if newMaxBW < 10 {
			newMaxBW = 10
		}
		policy.MaxBandwidth = newMaxBW
	}

	policy.UpdatedAt = time.Now()
}

// PriorityQueue 优先级队列
type PriorityQueue struct {
	mu       sync.RWMutex
	queues   map[QoSLevel][]*IORequest
	capacity int
}

// IORequest IO请求
type IORequest struct {
	TargetID  string
	Level     QoSLevel
	Operation string // read, write
	Size      int64
	Priority  int
	Timestamp time.Time
}

// NewPriorityQueue 创建优先级队列
func NewPriorityQueue(capacity int) *PriorityQueue {
	return &PriorityQueue{
		queues: map[QoSLevel][]*IORequest{
			QoSLevelPlatinum: make([]*IORequest, 0),
			QoSLevelGold:     make([]*IORequest, 0),
			QoSLevelSilver:   make([]*IORequest, 0),
			QoSLevelBronze:   make([]*IORequest, 0),
		},
		capacity: capacity,
	}
}

// Enqueue 入队
func (pq *PriorityQueue) Enqueue(req *IORequest) error {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	queue, exists := pq.queues[req.Level]
	if !exists {
		return fmt.Errorf("无效的优先级级别: %s", req.Level)
	}

	if len(queue) >= pq.capacity {
		return fmt.Errorf("队列已满")
	}

	pq.queues[req.Level] = append(queue, req)
	return nil
}

// Dequeue 出队
func (pq *PriorityQueue) Dequeue() (*IORequest, error) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	// 按优先级顺序出队
	levels := []QoSLevel{QoSLevelPlatinum, QoSLevelGold, QoSLevelSilver, QoSLevelBronze}
	for _, level := range levels {
		queue := pq.queues[level]
		if len(queue) > 0 {
			req := queue[0]
			pq.queues[level] = queue[1:]
			return req, nil
		}
	}

	return nil, fmt.Errorf("队列为空")
}

// GetQueueSize 获取队列大小
func (pq *PriorityQueue) GetQueueSize(level QoSLevel) int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	queue, exists := pq.queues[level]
	if !exists {
		return 0
	}

	return len(queue)
}

// GetTotalSize 获取总大小
func (pq *PriorityQueue) GetTotalSize() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	total := 0
	for _, queue := range pq.queues {
		total += len(queue)
	}
	return total
}

// Clear 清空队列
func (pq *PriorityQueue) Clear() {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	for level := range pq.queues {
		pq.queues[level] = make([]*IORequest, 0)
	}
}
