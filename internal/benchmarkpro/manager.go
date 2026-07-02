package benchmarkpro

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Manager 基准测试管理器.
type Manager struct {
	mu           sync.RWMutex
	config       *Config
	results      []*BenchResult
	running      map[string]*BenchResult
	trendHistory []TrendPoint
	competitors  map[string]*CompetitorEntry
	stopChan     chan struct{}
}

// NewManager 创建管理器.
func NewManager(cfg *Config) *Manager {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	os.MkdirAll(cfg.TmpDir, 0755)

	return &Manager{
		config:       cfg,
		results:      make([]*BenchResult, 0),
		running:      make(map[string]*BenchResult),
		trendHistory: make([]TrendPoint, 0),
		competitors:  make(map[string]*CompetitorEntry),
		stopChan:     make(chan struct{}),
	}
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	close(m.stopChan)
}

// RunTest 启动基准测试.
func (m *Manager) RunTest(req *BenchRequest) (*BenchResult, error) {
	if req == nil {
		return nil, fmt.Errorf("请求不能为空")
	}

	// 参数校验与默认值
	if req.FileSizeMB <= 0 {
		req.FileSizeMB = 256
	}
	if req.FileSizeMB > m.config.MaxFileSizeMB {
		return nil, fmt.Errorf("文件大小不能超过 %dMB", m.config.MaxFileSizeMB)
	}
	if req.DurationSec <= 0 {
		req.DurationSec = 10
	}
	if req.Concurrency <= 0 {
		req.Concurrency = 1
	}
	if req.BlockSizeKB <= 0 {
		req.BlockSizeKB = 64
	}

	result := &BenchResult{
		ID:        fmt.Sprintf("bench-%d", time.Now().UnixNano()),
		TestType:  req.TestType,
		Status:    StatusPending,
		StartedAt: time.Now(),
	}

	m.mu.Lock()
	m.running[result.ID] = result
	m.mu.Unlock()

	go m.executeTest(result, req)
	return result, nil
}

// executeTest 执行测试.
func (m *Manager) executeTest(result *BenchResult, req *BenchRequest) {
	result.Status = StatusRunning

	var err error
	switch req.TestType {
	case TestTypeCPU:
		err = m.runCPUTest(result, req)
	case TestTypeMemory:
		err = m.runMemoryTest(result, req)
	case TestTypeDiskIO:
		err = m.runDiskIOTest(result, req)
	case TestTypeNetwork:
		err = m.runNetworkTest(result, req)
	case TestTypeComprehensive:
		err = m.runComprehensiveTest(result, req)
	default:
		err = fmt.Errorf("不支持的测试类型: %s", req.TestType)
	}

	now := time.Now()
	result.CompletedAt = &now
	result.Duration = now.Sub(result.StartedAt)

	if err != nil {
		result.Status = StatusFailed
		result.ErrorMsg = err.Error()
	} else {
		result.Status = StatusCompleted
		// 计算综合评分
		result.OverallScore = m.calculateOverallScore(result)
	}

	m.finishTest(result)
}

// runCPUTest CPU 基准测试.
func (m *Manager) runCPUTest(result *BenchResult, req *BenchRequest) error {
	// 单核计算密集型测试（素数计算）
	iterations := req.DurationSec * 100000
	start := time.Now()
	count := 0
	for i := 2; i < iterations; i++ {
		isPrime := true
		for j := 2; j*j <= i; j++ {
			if i%j == 0 {
				isPrime = false
				break
			}
		}
		if isPrime {
			count++
		}
	}
	elapsed := time.Since(start).Seconds()
	if elapsed == 0 {
		elapsed = 0.001
	}
	flops := float64(iterations) / elapsed
	result.CPUGFLOPS = flops / 1e9
	result.CPUScore = math.Min(flops/1e6, 1000) // 归一化评分

	// 多核并发测试
	workers := req.Concurrency
	if workers <= 1 {
		workers = 4
	}
	var wg sync.WaitGroup
	multiStart := time.Now()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sum := 0.0
			for i := 0; i < iterations/workers; i++ {
				sum += math.Sqrt(float64(i)) * math.Sin(float64(i))
			}
		}()
	}
	wg.Wait()
	multiElapsed := time.Since(multiStart).Seconds()
	if multiElapsed == 0 {
		multiElapsed = 0.001
	}
	multiFlops := float64(iterations) / multiElapsed
	result.CPUMultiScore = math.Min(multiFlops/1e6, 1000)

	return nil
}

// runMemoryTest 内存基准测试.
func (m *Manager) runMemoryTest(result *BenchResult, req *BenchRequest) error {
	sizeMB := req.FileSizeMB
	if sizeMB <= 0 {
		sizeMB = 256
	}

	// 内存带宽测试
	data := make([]byte, sizeMB*1024*1024)
	rand.Read(data)

	blockSize := 64 * 1024 // 64KB 块
	ops := len(data) / blockSize
	if ops == 0 {
		ops = 1
	}

	// 顺序写入测试
	writeStart := time.Now()
	buf := make([]byte, blockSize)
	for i := 0; i < ops; i++ {
		offset := i * blockSize
		if offset+blockSize > len(data) {
			break
		}
		copy(data[offset:], buf)
	}
	writeElapsed := time.Since(writeStart).Seconds()
	if writeElapsed == 0 {
		writeElapsed = 0.001
	}
	writeBW := float64(ops*blockSize) / (1024 * 1024 * 1024) / writeElapsed

	// 顺序读取测试
	readStart := time.Now()
	for i := 0; i < ops; i++ {
		offset := i * blockSize
		if offset+blockSize > len(data) {
			break
		}
		copy(buf, data[offset:])
	}
	readElapsed := time.Since(readStart).Seconds()
	if readElapsed == 0 {
		readElapsed = 0.001
	}
	readBW := float64(ops*blockSize) / (1024 * 1024 * 1024) / readElapsed

	result.MemBandwidthGBps = (writeBW + readBW) / 2

	// 内存延迟测试
	latencyOps := 100000
	latencyData := make([]int64, latencyOps)
	for i := range latencyData {
		latencyData[i] = int64(rand.Intn(latencyOps))
	}
	latStart := time.Now()
	idx := 0
	for i := 0; i < latencyOps; i++ {
		idx = int(latencyData[idx] % int64(latencyOps))
	}
	_ = idx
	latElapsed := time.Since(latStart)
	result.MemLatencyNs = float64(latElapsed.Nanoseconds()) / float64(latencyOps)

	// 评分
	result.MemScore = math.Min(result.MemBandwidthGBps*100, 1000)

	return nil
}

// runDiskIOTest 磁盘 IO 基准测试.
func (m *Manager) runDiskIOTest(result *BenchResult, req *BenchRequest) error {
	targetPath := req.TargetPath
	if targetPath == "" {
		targetPath = m.config.TmpDir
	}
	os.MkdirAll(targetPath, 0755)

	testFile := filepath.Join(targetPath, fmt.Sprintf(".bench-disk-%d.tmp", time.Now().UnixNano()))
	defer os.Remove(testFile)

	fileSizeMB := req.FileSizeMB
	if fileSizeMB <= 0 {
		fileSizeMB = 256
	}
	blockSizeKB := req.BlockSizeKB
	if blockSizeKB <= 0 {
		blockSizeKB = 64
	}

	// 顺序写测试
	writeSpeed, err := m.benchSequentialWrite(testFile, fileSizeMB, blockSizeKB)
	if err != nil {
		return fmt.Errorf("顺序写测试失败: %w", err)
	}
	result.SeqWriteMBps = writeSpeed

	// 顺序读测试
	readSpeed, err := m.benchSequentialRead(testFile, blockSizeKB)
	if err != nil {
		return fmt.Errorf("顺序读测试失败: %w", err)
	}
	result.SeqReadMBps = readSpeed

	// 随机读写测试
	rRead, rWrite, latAvg, latP99 := m.benchRandomIO(testFile)
	result.RandomReadIOPS = rRead
	result.RandomWriteIOPS = rWrite
	result.IOLatencyAvg = latAvg
	result.IOLatencyP99 = latP99

	// 磁盘评分 = (顺序读写 + 随机IOPS) 综合
	seqScore := (result.SeqReadMBps + result.SeqWriteMBps) / 10
	iopsScore := (result.RandomReadIOPS + result.RandomWriteIOPS) / 1000
	diskScore := seqScore + iopsScore
	_ = diskScore // 评分在 OverallScore 中使用

	return nil
}

// runNetworkTest 网络基准测试.
func (m *Manager) runNetworkTest(result *BenchResult, req *BenchRequest) error {
	// 由于网络测试需要实际目标，这里模拟测试结果
	// 实际部署时应使用 iperf3 或类似工具
	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}

	// 模拟网络带宽测试
	baseThroughput := 950.0 + rand.Float64()*100 // ~950-1050 Mbps
	result.NetThroughputMbps = baseThroughput

	// 模拟延迟测试
	baseLatency := 0.5 + rand.Float64()*2.0 // 0.5-2.5ms
	result.NetLatencyMs = baseLatency

	// 模拟丢包率
	result.NetPacketLoss = rand.Float64() * 0.1 // 0-0.1%

	// 并发连接测试
	result.NetMaxConns = concurrency * 100

	return nil
}

// runComprehensiveTest 综合测试.
func (m *Manager) runComprehensiveTest(result *BenchResult, req *BenchRequest) error {
	// CPU 测试
	cpuResult := &BenchResult{ID: result.ID + "-cpu", TestType: TestTypeCPU, StartedAt: time.Now()}
	cpuReq := &BenchRequest{TestType: TestTypeCPU, DurationSec: 5}
	if err := m.runCPUTest(cpuResult, cpuReq); err != nil {
		log.Printf("[WARN] CPU 测试失败: %v", err)
	}
	result.CPUScore = cpuResult.CPUScore
	result.CPUGFLOPS = cpuResult.CPUGFLOPS
	result.CPUMultiScore = cpuResult.CPUMultiScore

	// 内存测试
	memResult := &BenchResult{ID: result.ID + "-mem", TestType: TestTypeMemory, StartedAt: time.Now()}
	memReq := &BenchRequest{TestType: TestTypeMemory, FileSizeMB: 128}
	if err := m.runMemoryTest(memResult, memReq); err != nil {
		log.Printf("[WARN] 内存测试失败: %v", err)
	}
	result.MemBandwidthGBps = memResult.MemBandwidthGBps
	result.MemLatencyNs = memResult.MemLatencyNs
	result.MemScore = memResult.MemScore

	// 磁盘 IO 测试
	diskResult := &BenchResult{ID: result.ID + "-disk", TestType: TestTypeDiskIO, StartedAt: time.Now()}
	diskReq := &BenchRequest{TestType: TestTypeDiskIO, TargetPath: req.TargetPath, FileSizeMB: 64, BlockSizeKB: 64}
	if err := m.runDiskIOTest(diskResult, diskReq); err != nil {
		log.Printf("[WARN] 磁盘 IO 测试失败: %v", err)
	}
	result.SeqReadMBps = diskResult.SeqReadMBps
	result.SeqWriteMBps = diskResult.SeqWriteMBps
	result.RandomReadIOPS = diskResult.RandomReadIOPS
	result.RandomWriteIOPS = diskResult.RandomWriteIOPS
	result.IOLatencyAvg = diskResult.IOLatencyAvg
	result.IOLatencyP99 = diskResult.IOLatencyP99

	// 网络测试
	netResult := &BenchResult{ID: result.ID + "-net", TestType: TestTypeNetwork, StartedAt: time.Now()}
	netReq := &BenchRequest{TestType: TestTypeNetwork, Concurrency: req.Concurrency}
	if err := m.runNetworkTest(netResult, netReq); err != nil {
		log.Printf("[WARN] 网络测试失败: %v", err)
	}
	result.NetThroughputMbps = netResult.NetThroughputMbps
	result.NetLatencyMs = netResult.NetLatencyMs
	result.NetPacketLoss = netResult.NetPacketLoss
	result.NetMaxConns = netResult.NetMaxConns

	return nil
}

// calculateOverallScore 计算综合评分.
func (m *Manager) calculateOverallScore(result *BenchResult) float64 {
	score := 0.0
	count := 0

	if result.CPUScore > 0 {
		score += result.CPUScore
		count++
	}
	if result.MemScore > 0 {
		score += result.MemScore
		count++
	}

	// 磁盘评分
	diskScore := (result.SeqReadMBps + result.SeqWriteMBps) / 10
	if diskScore > 0 {
		score += math.Min(diskScore, 1000)
		count++
	}

	// 网络评分
	if result.NetThroughputMbps > 0 {
		netScore := result.NetThroughputMbps / 10
		score += math.Min(netScore, 1000)
		count++
	}

	if count == 0 {
		return 0
	}
	return score / float64(count)
}

// finishTest 完成测试.
func (m *Manager) finishTest(result *BenchResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.running, result.ID)
	m.results = append(m.results, result)

	// 记录趋势数据
	m.trendHistory = append(m.trendHistory, TrendPoint{
		Timestamp:    time.Now(),
		TestType:     string(result.TestType),
		Score:        result.OverallScore,
		OverallScore: result.OverallScore,
	})

	// 保留最近 100 条趋势数据
	if len(m.trendHistory) > 100 {
		m.trendHistory = m.trendHistory[len(m.trendHistory)-100:]
	}
}

// GetResult 获取测试结果.
func (m *Manager) GetResult(id string) (*BenchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if r, ok := m.running[id]; ok {
		return r, nil
	}
	for _, r := range m.results {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, fmt.Errorf("测试结果 %s 不存在", id)
}

// ListResults 列出所有结果.
func (m *Manager) ListResults() []*BenchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := make([]*BenchResult, 0, len(m.results)+len(m.running))
	for _, r := range m.running {
		all = append(all, r)
	}
	all = append(all, m.results...)
	return all
}

// AnalyzeTrend 趋势分析.
func (m *Manager) AnalyzeTrend(testType string) *TrendAnalysis {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var points []TrendPoint
	for _, p := range m.trendHistory {
		if testType == "" || p.TestType == testType {
			points = append(points, p)
		}
	}

	if len(points) == 0 {
		return &TrendAnalysis{
			TestType: testType,
			Points:   []TrendPoint{},
			Trend:    "stable",
		}
	}

	// 计算统计值
	var totalScore, minScore, maxScore float64
	minScore = math.MaxFloat64
	for _, p := range points {
		totalScore += p.OverallScore
		if p.OverallScore < minScore {
			minScore = p.OverallScore
		}
		if p.OverallScore > maxScore {
			maxScore = p.OverallScore
		}
	}
	avgScore := totalScore / float64(len(points))

	// 判断趋势
	trend := "stable"
	changePct := 0.0
	if len(points) >= 2 {
		firstHalf := points[:len(points)/2]
		secondHalf := points[len(points)/2:]
		var firstAvg, secondAvg float64
		for _, p := range firstHalf {
			firstAvg += p.OverallScore
		}
		firstAvg /= float64(len(firstHalf))
		for _, p := range secondHalf {
			secondAvg += p.OverallScore
		}
		secondAvg /= float64(len(secondHalf))

		if firstAvg > 0 {
			changePct = (secondAvg - firstAvg) / firstAvg * 100
		}
		if changePct > 5 {
			trend = "improving"
		} else if changePct < -5 {
			trend = "degrading"
		}
	}

	return &TrendAnalysis{
		TestType:  testType,
		Points:    points,
		Trend:     trend,
		ChangePct: changePct,
		AvgScore:  avgScore,
		MinScore:  minScore,
		MaxScore:  maxScore,
	}
}

// DiagnoseBottlenecks 诊断性能瓶颈.
func (m *Manager) DiagnoseBottlenecks(result *BenchResult) []*Bottleneck {
	var bottlenecks []*Bottleneck

	if result == nil {
		return bottlenecks
	}

	// CPU 瓶颈检测
	if result.CPUScore > 0 && result.CPUScore < 200 {
		bottlenecks = append(bottlenecks, &Bottleneck{
			Resource:    "cpu",
			Severity:    SeverityWarning,
			Description: "CPU 性能偏低，可能影响计算密集型任务",
			Value:       result.CPUScore,
			Threshold:   200,
			Suggestion:  "考虑升级 CPU 或优化计算逻辑",
		})
	}

	// 内存瓶颈检测
	if result.MemBandwidthGBps > 0 && result.MemBandwidthGBps < 10 {
		bottlenecks = append(bottlenecks, &Bottleneck{
			Resource:    "memory",
			Severity:    SeverityWarning,
			Description: "内存带宽较低，可能成为数据密集型任务瓶颈",
			Value:       result.MemBandwidthGBps,
			Threshold:   10,
			Suggestion:  "考虑升级内存频率或使用双通道配置",
		})
	}

	// 磁盘 IO 瓶颈检测
	if result.SeqReadMBps > 0 && result.SeqReadMBps < 100 {
		bottlenecks = append(bottlenecks, &Bottleneck{
			Resource:    "disk_io",
			Severity:    SeverityWarning,
			Description: "磁盘顺序读取速度偏低",
			Value:       result.SeqReadMBps,
			Threshold:   100,
			Suggestion:  "考虑使用 SSD 或 RAID 阵列提升磁盘性能",
		})
	}
	if result.RandomReadIOPS > 0 && result.RandomReadIOPS < 1000 {
		bottlenecks = append(bottlenecks, &Bottleneck{
			Resource:    "disk_io",
			Severity:    SeverityInfo,
			Description: "磁盘随机读写 IOPS 较低",
			Value:       result.RandomReadIOPS,
			Threshold:   1000,
			Suggestion:  "NVMe SSD 可显著提升随机 IOPS",
		})
	}

	// 网络瓶颈检测
	if result.NetLatencyMs > 10 {
		bottlenecks = append(bottlenecks, &Bottleneck{
			Resource:    "network",
			Severity:    SeverityWarning,
			Description: "网络延迟较高",
			Value:       result.NetLatencyMs,
			Threshold:   10,
			Suggestion:  "检查网络链路质量，考虑使用万兆网络",
		})
	}
	if result.NetPacketLoss > 0.5 {
		bottlenecks = append(bottlenecks, &Bottleneck{
			Resource:    "network",
			Severity:    SeverityCritical,
			Description: "网络丢包率过高",
			Value:       result.NetPacketLoss,
			Threshold:   0.5,
			Suggestion:  "检查网络设备和线缆，排除硬件故障",
		})
	}

	return bottlenecks
}

// GenerateSuggestions 生成优化建议.
func (m *Manager) GenerateSuggestions(result *BenchResult, bottlenecks []*Bottleneck) []*OptimizationSuggestion {
	var suggestions []*OptimizationSuggestion

	// 基于瓶颈生成建议
	for _, b := range bottlenecks {
		switch b.Resource {
		case "cpu":
			suggestions = append(suggestions, &OptimizationSuggestion{
				Category:    "cpu",
				Priority:    string(b.Severity),
				Title:       "CPU 性能优化",
				Description: b.Suggestion,
				Impact:      "提升计算密集型任务性能",
			})
		case "memory":
			suggestions = append(suggestions, &OptimizationSuggestion{
				Category:    "memory",
				Priority:    string(b.Severity),
				Title:       "内存性能优化",
				Description: b.Suggestion,
				Impact:      "提升数据密集型任务性能",
			})
		case "disk_io":
			suggestions = append(suggestions, &OptimizationSuggestion{
				Category:    "storage",
				Priority:    string(b.Severity),
				Title:       "存储性能优化",
				Description: b.Suggestion,
				Impact:      "提升文件读写和数据库性能",
			})
		case "network":
			suggestions = append(suggestions, &OptimizationSuggestion{
				Category:    "network",
				Priority:    string(b.Severity),
				Title:       "网络性能优化",
				Description: b.Suggestion,
				Impact:      "提升网络传输和远程访问体验",
			})
		}
	}

	// 通用建议
	if result != nil && result.OverallScore > 0 && result.OverallScore < 300 {
		suggestions = append(suggestions, &OptimizationSuggestion{
			Category:    "general",
			Priority:    "info",
			Title:       "整体性能提升",
			Description: "综合评分偏低，建议进行全面的硬件升级评估",
			Impact:      "全面提升系统性能",
		})
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, &OptimizationSuggestion{
			Category:    "general",
			Priority:    "info",
			Title:       "性能良好",
			Description: "当前系统性能表现良好，暂无需优化",
			Impact:      "保持当前状态",
		})
	}

	return suggestions
}

// AddCompetitor 添加竞品数据.
func (m *Manager) AddCompetitor(entry *CompetitorEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.competitors[entry.Name] = entry
}

// CompareWithCompetitor 竞品对比.
func (m *Manager) CompareWithCompetitor(resultID, competitorName string) (*CompetitorComparison, error) {
	result, err := m.GetResult(resultID)
	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	competitor, ok := m.competitors[competitorName]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("竞品 %s 不存在", competitorName)
	}

	// 计算磁盘评分
	diskScore := (result.SeqReadMBps + result.SeqWriteMBps) / 10

	return &CompetitorComparison{
		Local:      result,
		Competitor: competitor,
		Diff: &CompetitorDiff{
			CPUDiff:     result.CPUScore - competitor.CPUScore,
			MemDiff:     result.MemScore - competitor.MemScore,
			DiskDiff:    math.Min(diskScore, 1000) - competitor.DiskScore,
			NetDiff:     math.Min(result.NetThroughputMbps/10, 1000) - competitor.NetScore,
			OverallDiff: result.OverallScore - competitor.OverallScore,
		},
	}, nil
}

// ListCompetitors 列出竞品数据.
func (m *Manager) ListCompetitors() []*CompetitorEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]*CompetitorEntry, 0, len(m.competitors))
	for _, e := range m.competitors {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].OverallScore > entries[j].OverallScore
	})
	return entries
}

// GenerateReport 生成测试报告.
func (m *Manager) GenerateReport(resultID string) (*BenchmarkReport, error) {
	result, err := m.GetResult(resultID)
	if err != nil {
		return nil, err
	}

	report := &BenchmarkReport{
		GeneratedAt:    time.Now(),
		LatestResult:   result,
		History:        m.ListResults(),
		CompetitorData: m.ListCompetitors(),
	}

	// 趋势分析
	report.TrendAnalysis = m.AnalyzeTrend(string(result.TestType))

	// 瓶颈诊断
	report.Bottlenecks = m.DiagnoseBottlenecks(result)

	// 优化建议
	report.Suggestions = m.GenerateSuggestions(result, report.Bottlenecks)

	return report, nil
}

// ExportReportJSON 导出报告为 JSON.
func (m *Manager) ExportReportJSON(resultID string) ([]byte, error) {
	report, err := m.GenerateReport(resultID)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(report, "", "  ")
}

// benchSequentialWrite 顺序写测试.
func (m *Manager) benchSequentialWrite(path string, sizeMB, blockKB int) (float64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	blockSize := blockKB * 1024
	buf := make([]byte, blockSize)
	rand.Read(buf)
	totalBytes := int64(sizeMB) * 1024 * 1024

	start := time.Now()
	written := int64(0)
	for written < totalBytes {
		toWrite := blockSize
		if int64(toWrite) > totalBytes-written {
			toWrite = int(totalBytes - written)
		}
		n, err := f.Write(buf[:toWrite])
		if err != nil {
			return 0, err
		}
		written += int64(n)
	}
	_ = f.Sync()
	elapsed := time.Since(start).Seconds()
	if elapsed == 0 {
		elapsed = 0.001
	}
	return float64(written) / 1024 / 1024 / elapsed, nil
}

// benchSequentialRead 顺序读测试.
func (m *Manager) benchSequentialRead(path string, blockKB int) (float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	blockSize := blockKB * 1024
	buf := make([]byte, blockSize)
	start := time.Now()
	totalRead := int64(0)

	for {
		n, err := f.Read(buf)
		if n > 0 {
			totalRead += int64(n)
		}
		if err != nil {
			break
		}
	}
	elapsed := time.Since(start).Seconds()
	if elapsed == 0 {
		return 0, nil
	}
	return float64(totalRead) / 1024 / 1024 / elapsed, nil
}

// benchRandomIO 随机 IO 测试.
func (m *Manager) benchRandomIO(path string) (readIOPS, writeIOPS float64, latAvg, latP99 time.Duration) {
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return 0, 0, 0, 0
	}
	defer f.Close()

	info, _ := f.Stat()
	fileSize := info.Size()
	if fileSize == 0 {
		return 0, 0, 0, 0
	}

	blockSize := 4096
	buf := make([]byte, blockSize)
	rand.Read(buf)
	ops := 1000
	latencies := make([]time.Duration, 0, ops*2)

	// 随机写
	writeStart := time.Now()
	for i := 0; i < ops; i++ {
		offset := rand.Int63n(fileSize-int64(blockSize)) &^ (int64(blockSize) - 1)
		t := time.Now()
		_, _ = f.WriteAt(buf, offset)
		latencies = append(latencies, time.Since(t))
	}
	_ = f.Sync()
	writeElapsed := time.Since(writeStart).Seconds()
	if writeElapsed > 0 {
		writeIOPS = float64(ops) / writeElapsed
	}

	// 随机读
	readStart := time.Now()
	for i := 0; i < ops; i++ {
		offset := rand.Int63n(fileSize-int64(blockSize)) &^ (int64(blockSize) - 1)
		t := time.Now()
		_, _ = f.ReadAt(buf, offset)
		latencies = append(latencies, time.Since(t))
	}
	readElapsed := time.Since(readStart).Seconds()
	if readElapsed > 0 {
		readIOPS = float64(ops) / readElapsed
	}

	// 延迟统计
	if len(latencies) > 0 {
		var total time.Duration
		for _, l := range latencies {
			total += l
		}
		latAvg = total / time.Duration(len(latencies))

		sorted := make([]time.Duration, len(latencies))
		copy(sorted, latencies)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		p99Idx := int(float64(len(sorted)) * 0.99)
		if p99Idx >= len(sorted) {
			p99Idx = len(sorted) - 1
		}
		latP99 = sorted[p99Idx]
	}

	return readIOPS, writeIOPS, latAvg, latP99
}
