package resourceoptimizer

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
	"go.uber.org/zap"
)

// Manager 资源优化管理器.
type Manager struct {
	logger         *zap.Logger
	mu             sync.RWMutex
	history        []ResourceSnapshot
	maxHistory     int
	lastAnalysis   *AnalysisResult
	analyzing      bool
}

// NewManager 创建管理器.
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		logger:     logger,
		maxHistory: 720, // 保留720个快照（1小时一次，保留30天）
	}
}

// CollectSnapshot 采集当前资源快照.
func (m *Manager) CollectSnapshot(ctx context.Context) (*ResourceSnapshot, error) {
	snapshot := &ResourceSnapshot{
		Timestamp: time.Now(),
	}

	// 采集 CPU
	cpuMetrics, err := m.collectCPU(ctx)
	if err != nil {
		m.logger.Warn("采集CPU指标失败", zap.Error(err))
	} else {
		snapshot.CPU = cpuMetrics
	}

	// 采集内存
	memMetrics, err := m.collectMemory(ctx)
	if err != nil {
		m.logger.Warn("采集内存指标失败", zap.Error(err))
	} else {
		snapshot.Memory = memMetrics
	}

	// 采集磁盘
	diskMetrics, err := m.collectDisk(ctx)
	if err != nil {
		m.logger.Warn("采集磁盘指标失败", zap.Error(err))
	} else {
		snapshot.Disk = diskMetrics
	}

	// 采集网络
	netMetrics, err := m.collectNetwork(ctx)
	if err != nil {
		m.logger.Warn("采集网络指标失败", zap.Error(err))
	} else {
		snapshot.Network = netMetrics
	}

	// 采集进程
	procMetrics, err := m.collectProcesses(ctx)
	if err != nil {
		m.logger.Warn("采集进程指标失败", zap.Error(err))
	} else {
		snapshot.Processes = procMetrics
	}

	// 保存历史
	m.mu.Lock()
	m.history = append(m.history, *snapshot)
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}
	m.mu.Unlock()

	return snapshot, nil
}

// Analyze 执行综合分析.
func (m *Manager) Analyze(ctx context.Context, req *AnalyzeRequest) (*AnalysisResult, error) {
	if req == nil {
		req = DefaultAnalyzeRequest()
	}

	m.mu.Lock()
	if m.analyzing {
		m.mu.Unlock()
		return nil, ErrAnalysisInProgress
	}
	m.analyzing = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.analyzing = false
		m.mu.Unlock()
	}()

	startTime := time.Now()
	analysisID := uuid.New().String()

	m.logger.Info("开始资源分析", zap.String("analysis_id", analysisID))

	// 采集快照
	snapshot, err := m.CollectSnapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("采集快照失败: %w", err)
	}

	result := &AnalysisResult{
		AnalysisID: analysisID,
		StartedAt:  startTime,
		Snapshot:   snapshot,
	}

	// 生成建议
	result.Recommendations = m.generateRecommendations(snapshot, req.ResourceTypes)

	// 趋势预测
	if req.IncludeTrend {
		result.Trends = m.predictTrends(req.ResourceTypes)
	}

	// 成本优化
	if req.IncludeCost {
		result.CostOptimizations = m.analyzeCostOptimizations(snapshot, req.ResourceTypes)
	}

	// 计算健康评分
	result.OverallScore = m.calculateHealthScore(snapshot)

	result.FinishedAt = time.Now()
	result.DurationSeconds = result.FinishedAt.Sub(result.StartedAt).Seconds()

	m.mu.Lock()
	m.lastAnalysis = result
	m.mu.Unlock()

	m.logger.Info("资源分析完成",
		zap.String("analysis_id", analysisID),
		zap.Float64("duration_seconds", result.DurationSeconds),
		zap.Float64("health_score", result.OverallScore),
	)

	return result, nil
}

// GetLastAnalysis 获取最后一次分析结果.
func (m *Manager) GetLastAnalysis() *AnalysisResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastAnalysis
}

// GetHistory 获取历史快照.
func (m *Manager) GetHistory(limit int) []ResourceSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	start := len(m.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]ResourceSnapshot, limit)
	copy(result, m.history[start:])
	return result
}

// GetRecommendations 获取优化建议.
func (m *Manager) GetRecommendations(resourceTypes []ResourceType) []Recommendation {
	if len(resourceTypes) == 0 {
		resourceTypes = []ResourceType{ResourceCPU, ResourceMemory, ResourceDisk, ResourceNetwork}
	}

	// 先采集最新快照
	ctx := context.Background()
	snapshot, err := m.CollectSnapshot(ctx)
	if err != nil {
		m.logger.Error("采集快照失败", zap.Error(err))
		return nil
	}

	return m.generateRecommendations(snapshot, resourceTypes)
}

// GetTrends 获取趋势预测.
func (m *Manager) GetTrends(resourceTypes []ResourceType) []TrendPrediction {
	if len(resourceTypes) == 0 {
		resourceTypes = []ResourceType{ResourceCPU, ResourceMemory, ResourceDisk, ResourceNetwork}
	}
	return m.predictTrends(resourceTypes)
}

// ========== 私有方法 ==========

func (m *Manager) collectCPU(ctx context.Context) (*CPUMetrics, error) {
	percent, err := cpu.PercentWithContext(ctx, time.Second, false)
	if err != nil {
		return nil, err
	}

	perCPU, err := cpu.PercentWithContext(ctx, time.Second, true)
	if err != nil {
		return nil, err
	}

	metrics := &CPUMetrics{
		CoreUsage: perCPU,
	}
	if len(percent) > 0 {
		metrics.UsagePercent = percent[0]
	}

	// 获取负载平均值 (Linux only)
	load, err := cpu.CountsWithContext(ctx, true)
	if err == nil && load > 0 {
		metrics.LoadAvg1 = float64(load) * metrics.UsagePercent / 100
	}

	return metrics, nil
}

func (m *Manager) collectMemory(ctx context.Context) (*MemoryMetrics, error) {
	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}

	swap, _ := mem.SwapMemoryWithContext(ctx)

	metrics := &MemoryMetrics{
		TotalMB:      float64(vm.Total) / 1024 / 1024,
		UsedMB:       float64(vm.Used) / 1024 / 1024,
		FreeMB:       float64(vm.Free) / 1024 / 1024,
		CachedMB:     float64(vm.Cached) / 1024 / 1024,
		BuffersMB:    float64(vm.Buffers) / 1024 / 1024,
		UsagePercent: vm.UsedPercent,
	}

	if swap != nil {
		metrics.SwapTotalMB = float64(swap.Total) / 1024 / 1024
		metrics.SwapUsedMB = float64(swap.Used) / 1024 / 1024
	}

	return metrics, nil
}

func (m *Manager) collectDisk(ctx context.Context) (*DiskMetrics, error) {
	usage, err := disk.UsageWithContext(ctx, "/")
	if err != nil {
		return nil, err
	}

	metrics := &DiskMetrics{
		MountPoint:   "/",
		TotalGB:      float64(usage.Total) / 1024 / 1024 / 1024,
		UsedGB:       float64(usage.Used) / 1024 / 1024 / 1024,
		FreeGB:       float64(usage.Free) / 1024 / 1024 / 1024,
		UsagePercent: usage.UsedPercent,
	}

	return metrics, nil
}

func (m *Manager) collectNetwork(ctx context.Context) (*NetworkMetrics, error) {
	counters, err := net.IOCountersWithContext(ctx, false)
	if err != nil {
		return nil, err
	}

	if len(counters) == 0 {
		return nil, fmt.Errorf("no network counters available")
	}

	c := counters[0]
	return &NetworkMetrics{
		Interface: "total",
		TotalRxMB: float64(c.BytesRecv) / 1024 / 1024,
		TotalTxMB: float64(c.BytesSent) / 1024 / 1024,
		Errors:    int(c.Errin + c.Errout),
		Dropped:   int(c.Dropin + c.Dropout),
	}, nil
}

func (m *Manager) collectProcesses(ctx context.Context) ([]ProcessMetrics, error) {
	pids, err := process.PidsWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var processes []ProcessMetrics
	for _, pid := range pids {
		p, err := process.NewProcessWithContext(ctx, pid)
		if err != nil {
			continue
		}

		name, _ := p.NameWithContext(ctx)
		cpuPercent, _ := p.CPUPercentWithContext(ctx)
		memInfo, _ := p.MemoryInfoWithContext(ctx)

		proc := ProcessMetrics{
			PID:        int(pid),
			Name:       name,
			CPUPercent: cpuPercent,
		}

		if memInfo != nil {
			proc.MemoryMB = float64(memInfo.RSS) / 1024 / 1024
		}

		processes = append(processes, proc)

		// 限制返回前20个进程
		if len(processes) >= 20 {
			break
		}
	}

	return processes, nil
}

func (m *Manager) generateRecommendations(snapshot *ResourceSnapshot, resourceTypes []ResourceType) []Recommendation {
	var recommendations []Recommendation

	for _, rt := range resourceTypes {
		switch rt {
		case ResourceCPU:
			recommendations = append(recommendations, m.cpuRecommendations(snapshot)...)
		case ResourceMemory:
			recommendations = append(recommendations, m.memoryRecommendations(snapshot)...)
		case ResourceDisk:
			recommendations = append(recommendations, m.diskRecommendations(snapshot)...)
		case ResourceNetwork:
			recommendations = append(recommendations, m.networkRecommendations(snapshot)...)
		}
	}

	return recommendations
}

func (m *Manager) cpuRecommendations(snapshot *ResourceSnapshot) []Recommendation {
	var recs []Recommendation
	if snapshot.CPU == nil {
		return recs
	}

	cpu := snapshot.CPU
	if cpu.UsagePercent > 90 {
		recs = append(recs, Recommendation{
			ID:           uuid.New().String(),
			ResourceType: ResourceCPU,
			Title:        "CPU使用率过高",
			Description:  fmt.Sprintf("当前CPU使用率 %.1f%%，建议检查高CPU进程并优化", cpu.UsagePercent),
			CurrentValue: fmt.Sprintf("%.1f%%", cpu.UsagePercent),
			ExpectedValue: "< 80%",
			Priority:     PriorityCritical,
			Action:       "检查高CPU进程，考虑优化代码或增加CPU资源",
			Category:     "performance",
			CreatedAt:    time.Now(),
		})
	} else if cpu.UsagePercent > 70 {
		recs = append(recs, Recommendation{
			ID:           uuid.New().String(),
			ResourceType: ResourceCPU,
			Title:        "CPU使用率偏高",
			Description:  fmt.Sprintf("当前CPU使用率 %.1f%%，接近阈值", cpu.UsagePercent),
			CurrentValue: fmt.Sprintf("%.1f%%", cpu.UsagePercent),
			ExpectedValue: "< 70%",
			Priority:     PriorityHigh,
			Action:       "监控CPU使用趋势，识别优化机会",
			Category:     "monitoring",
			CreatedAt:    time.Now(),
		})
	}

	// 检查CPU使用不均衡
	if len(cpu.CoreUsage) > 1 {
		maxUsage := 0.0
		minUsage := 100.0
		for _, u := range cpu.CoreUsage {
			if u > maxUsage {
				maxUsage = u
			}
			if u < minUsage {
				minUsage = u
			}
		}
		if maxUsage-minUsage > 50 {
			recs = append(recs, Recommendation{
				ID:           uuid.New().String(),
				ResourceType: ResourceCPU,
				Title:        "CPU核心使用不均衡",
				Description:  fmt.Sprintf("CPU核心使用差异 %.1f%%，可能存在线程绑定问题", maxUsage-minUsage),
				CurrentValue: fmt.Sprintf("最大 %.1f%% / 最小 %.1f%%", maxUsage, minUsage),
				ExpectedValue: "差异 < 30%",
				Priority:     PriorityMedium,
				Action:       "检查进程的CPU亲和性设置",
				Category:     "performance",
				CreatedAt:    time.Now(),
			})
		}
	}

	return recs
}

func (m *Manager) memoryRecommendations(snapshot *ResourceSnapshot) []Recommendation {
	var recs []Recommendation
	if snapshot.Memory == nil {
		return recs
	}

	mem := snapshot.Memory
	if mem.UsagePercent > 90 {
		recs = append(recs, Recommendation{
			ID:           uuid.New().String(),
			ResourceType: ResourceMemory,
			Title:        "内存使用率过高",
			Description:  fmt.Sprintf("当前内存使用率 %.1f%%，可能导致OOM", mem.UsagePercent),
			CurrentValue: fmt.Sprintf("%.1f%%", mem.UsagePercent),
			ExpectedValue: "< 80%",
			Priority:     PriorityCritical,
			Action:       "检查内存泄漏进程，考虑增加内存或优化使用",
			Category:     "stability",
			CreatedAt:    time.Now(),
		})
	}

	// 检查swap使用
	if mem.SwapTotalMB > 0 && mem.SwapUsedMB > 0 {
		swapPercent := (mem.SwapUsedMB / mem.SwapTotalMB) * 100
		if swapPercent > 50 {
			recs = append(recs, Recommendation{
				ID:           uuid.New().String(),
				ResourceType: ResourceMemory,
				Title:        "Swap使用率过高",
				Description:  fmt.Sprintf("Swap使用率 %.1f%%，系统性能可能受到影响", swapPercent),
				CurrentValue: fmt.Sprintf("%.1f%%", swapPercent),
				ExpectedValue: "< 20%",
				Priority:     PriorityHigh,
				EstimatedSaving: "性能提升",
				Action:       "增加物理内存或优化内存使用",
				Category:     "performance",
				CreatedAt:    time.Now(),
			})
		}
	}

	// 检查内存泄漏迹象
	if len(m.history) >= 5 {
		recent := m.history[len(m.history)-5:]
		increasing := true
		for i := 1; i < len(recent); i++ {
			if recent[i].Memory == nil || recent[i-1].Memory == nil {
				increasing = false
				break
			}
			if recent[i].Memory.UsedMB <= recent[i-1].Memory.UsedMB {
				increasing = false
				break
			}
		}
		if increasing {
			recs = append(recs, Recommendation{
				ID:           uuid.New().String(),
				ResourceType: ResourceMemory,
				Title:        "可能存在内存泄漏",
				Description:  "最近5次采样内存使用持续增长",
			CurrentValue: fmt.Sprintf("%.1f MB", recent[len(recent)-1].Memory.UsedMB),
				Priority:     PriorityHigh,
				Action:       "检查进程内存增长趋势，使用pprof分析",
				Category:     "stability",
				CreatedAt:    time.Now(),
			})
		}
	}

	return recs
}

func (m *Manager) diskRecommendations(snapshot *ResourceSnapshot) []Recommendation {
	var recs []Recommendation
	if snapshot.Disk == nil {
		return recs
	}

	d := snapshot.Disk
	if d.UsagePercent > 90 {
		recs = append(recs, Recommendation{
			ID:           uuid.New().String(),
			ResourceType: ResourceDisk,
			Title:        "磁盘空间严重不足",
			Description:  fmt.Sprintf("磁盘使用率 %.1f%%，可能导致写入失败", d.UsagePercent),
			CurrentValue: fmt.Sprintf("%.1f%%", d.UsagePercent),
			ExpectedValue: "< 80%",
			Priority:     PriorityCritical,
			EstimatedSaving: fmt.Sprintf("%.1f GB", d.FreeGB),
			Action:       "清理临时文件、日志，或扩展存储",
			Category:     "capacity",
			CreatedAt:    time.Now(),
		})
	} else if d.UsagePercent > 80 {
		recs = append(recs, Recommendation{
			ID:           uuid.New().String(),
			ResourceType: ResourceDisk,
			Title:        "磁盘空间不足",
			Description:  fmt.Sprintf("磁盘使用率 %.1f%%，建议提前规划", d.UsagePercent),
			CurrentValue: fmt.Sprintf("%.1f%%", d.UsagePercent),
			ExpectedValue: "< 80%",
			Priority:     PriorityHigh,
			Action:       "清理不必要文件，规划存储扩展",
			Category:     "capacity",
			CreatedAt:    time.Now(),
		})
	}

	return recs
}

func (m *Manager) networkRecommendations(snapshot *ResourceSnapshot) []Recommendation {
	var recs []Recommendation
	if snapshot.Network == nil {
		return recs
	}

	n := snapshot.Network
	if n.Errors > 0 || n.Dropped > 0 {
		recs = append(recs, Recommendation{
			ID:           uuid.New().String(),
			ResourceType: ResourceNetwork,
			Title:        "网络错误或丢包",
			Description:  fmt.Sprintf("检测到 %d 个错误和 %d 个丢包", n.Errors, n.Dropped),
			CurrentValue: fmt.Sprintf("错误: %d, 丢包: %d", n.Errors, n.Dropped),
			ExpectedValue: "错误: 0, 丢包: 0",
			Priority:     PriorityHigh,
			Action:       "检查网络连接、网卡配置和驱动",
			Category:     "reliability",
			CreatedAt:    time.Now(),
		})
	}

	return recs
}

func (m *Manager) predictTrends(resourceTypes []ResourceType) []TrendPrediction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var predictions []TrendPrediction

	for _, rt := range resourceTypes {
		values := m.extractTrendData(rt)
		if len(values) < 3 {
			continue
		}

		prediction := TrendPrediction{
			ResourceType: rt,
			CurrentUsage: values[len(values)-1],
			DataPoints:   make([]TrendPoint, len(values)),
		}

		// 简单线性回归预测
		slope, intercept := linearRegression(values)
		prediction.PredictedUsage7d = slope*float64(len(values)+7) + intercept
		prediction.PredictedUsage30d = slope*float64(len(values)+30) + intercept

		// 趋势方向
		if slope > 0.1 {
			prediction.TrendDirection = "rising"
		} else if slope < -0.1 {
			prediction.TrendDirection = "falling"
		} else {
			prediction.TrendDirection = "stable"
		}

		// 置信度（基于数据点数量）
		prediction.Confidence = math.Min(float64(len(values))/50, 0.95)

		// 填充数据点
		for i, v := range values {
			prediction.DataPoints[i] = TrendPoint{
				Value: v,
			}
		}

		// 警告
		if prediction.PredictedUsage30d > 90 {
			prediction.Warnings = append(prediction.Warnings,
				fmt.Sprintf("按当前趋势，30天后%s使用率可能超过90%%", rt))
		}

		predictions = append(predictions, prediction)
	}

	return predictions
}

func (m *Manager) extractTrendData(rt ResourceType) []float64 {
	var values []float64
	for _, snap := range m.history {
		var val float64
		switch rt {
		case ResourceCPU:
			if snap.CPU != nil {
				val = snap.CPU.UsagePercent
			}
		case ResourceMemory:
			if snap.Memory != nil {
				val = snap.Memory.UsagePercent
			}
		case ResourceDisk:
			if snap.Disk != nil {
				val = snap.Disk.UsagePercent
			}
		case ResourceNetwork:
			if snap.Network != nil {
				val = snap.Network.TotalRxMB + snap.Network.TotalTxMB
			}
		}
		values = append(values, val)
	}
	return values
}

func (m *Manager) analyzeCostOptimizations(snapshot *ResourceSnapshot, resourceTypes []ResourceType) []CostOptimization {
	var optimizations []CostOptimization

	// 简化计算：基于使用率估算成本优化空间
	if snapshot.Memory != nil && contains(resourceTypes, ResourceMemory) {
		unusedPercent := 100 - snapshot.Memory.UsagePercent
		if unusedPercent > 40 {
			savingPercent := unusedPercent * 0.3 // 假设可以节省30%的未使用资源
			optimizations = append(optimizations, CostOptimization{
				ResourceType:  ResourceMemory,
				CurrentCost:   snapshot.Memory.TotalMB * 0.01, // 简化估算
				OptimizedCost: snapshot.Memory.TotalMB * (1 - savingPercent/100) * 0.01,
				SavingAmount:  snapshot.Memory.TotalMB * savingPercent / 100 * 0.01,
				SavingPercent: savingPercent,
				Recommendations: []string{
					"评估是否可以缩减内存规格",
					"优化应用内存使用",
				},
			})
		}
	}

	if snapshot.Disk != nil && contains(resourceTypes, ResourceDisk) {
		unusedPercent := 100 - snapshot.Disk.UsagePercent
		if unusedPercent > 50 {
			savingPercent := unusedPercent * 0.2
			optimizations = append(optimizations, CostOptimization{
				ResourceType:  ResourceDisk,
				CurrentCost:   snapshot.Disk.TotalGB * 0.5, // 简化估算
				OptimizedCost: snapshot.Disk.TotalGB * (1 - savingPercent/100) * 0.5,
				SavingAmount:  snapshot.Disk.TotalGB * savingPercent / 100 * 0.5,
				SavingPercent: savingPercent,
				Recommendations: []string{
					"清理不必要的存储",
					"考虑使用压缩或归档策略",
				},
			})
		}
	}

	return optimizations
}

func (m *Manager) calculateHealthScore(snapshot *ResourceSnapshot) float64 {
	score := 100.0

	if snapshot.CPU != nil {
		if snapshot.CPU.UsagePercent > 90 {
			score -= 20
		} else if snapshot.CPU.UsagePercent > 70 {
			score -= 10
		}
	}

	if snapshot.Memory != nil {
		if snapshot.Memory.UsagePercent > 90 {
			score -= 25
		} else if snapshot.Memory.UsagePercent > 70 {
			score -= 10
		}
	}

	if snapshot.Disk != nil {
		if snapshot.Disk.UsagePercent > 90 {
			score -= 25
		} else if snapshot.Disk.UsagePercent > 80 {
			score -= 10
		}
	}

	if snapshot.Network != nil {
		if snapshot.Network.Errors > 10 || snapshot.Network.Dropped > 10 {
			score -= 15
		}
	}

	return math.Max(0, score)
}

// linearRegression 简单线性回归.
func linearRegression(values []float64) (slope, intercept float64) {
	n := float64(len(values))
	if n == 0 {
		return 0, 0
	}

	var sumX, sumY, sumXY, sumX2 float64
	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0, sumY / n
	}

	slope = (n*sumXY - sumX*sumY) / denominator
	intercept = (sumY - slope*sumX) / n

	return slope, intercept
}

func contains(types []ResourceType, target ResourceType) bool {
	for _, t := range types {
		if t == target {
			return true
		}
	}
	return false
}
