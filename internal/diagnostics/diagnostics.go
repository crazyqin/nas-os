// Package diagnostics 提供系统诊断中心功能
// 一键诊断、健康评分、问题检测、诊断报告、历史记录、建议生成
package diagnostics

import (
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// DiagnosticReport 诊断报告
type DiagnosticReport struct {
	ID          string        `json:"id"`
	Timestamp   time.Time     `json:"timestamp"`
	Duration    time.Duration `json:"duration"`
	Score       int           `json:"score"`  // 健康评分 0-100
	Status      string        `json:"status"` // excellent/good/fair/poor/critical
	CPU         *CPUDiag      `json:"cpu"`
	Memory      *MemoryDiag   `json:"memory"`
	Disk        *DiskDiag     `json:"disk"`
	Network     *NetworkDiag  `json:"network"`
	Problems    []Problem     `json:"problems"`
	Suggestions []Suggestion  `json:"suggestions"`
	Summary     string        `json:"summary"`
}

// CPUDiag CPU诊断信息
type CPUDiag struct {
	Usage       float64 `json:"usage"`       // 使用率 (%)
	LoadAvg1    float64 `json:"loadAvg1"`    // 1分钟负载
	LoadAvg5    float64 `json:"loadAvg5"`    // 5分钟负载
	LoadAvg15   float64 `json:"loadAvg15"`   // 15分钟负载
	Cores       int     `json:"cores"`       // 核心数
	Temperature float64 `json:"temperature"` // 温度 (°C)
	Score       int     `json:"score"`       // 单项评分
	Status      string  `json:"status"`
}

// MemoryDiag 内存诊断信息
type MemoryDiag struct {
	Total       uint64  `json:"total"`       // 总量 (字节)
	Used        uint64  `json:"used"`        // 已用
	Available   uint64  `json:"available"`   // 可用
	UsedPercent float64 `json:"usedPercent"` // 使用率 (%)
	SwapTotal   uint64  `json:"swapTotal"`   // Swap总量
	SwapUsed    uint64  `json:"swapUsed"`    // Swap已用
	Score       int     `json:"score"`
	Status      string  `json:"status"`
}

// DiskDiag 磁盘诊断信息
type DiskDiag struct {
	Partitions  []PartitionInfo `json:"partitions"`
	TotalSpace  uint64          `json:"totalSpace"`  // 总空间
	UsedSpace   uint64          `json:"usedSpace"`   // 已用空间
	UsedPercent float64         `json:"usedPercent"` // 使用率
	Score       int             `json:"score"`
	Status      string          `json:"status"`
}

// PartitionInfo 分区信息
type PartitionInfo struct {
	MountPoint  string  `json:"mountPoint"`
	Device      string  `json:"device"`
	FSType      string  `json:"fstype"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"usedPercent"`
}

// NetworkDiag 网络诊断信息
type NetworkDiag struct {
	Interfaces   []InterfaceInfo `json:"interfaces"`
	Connectivity bool            `json:"connectivity"`
	Latency      float64         `json:"latency"` // ms
	Score        int             `json:"score"`
	Status       string          `json:"status"`
}

// InterfaceInfo 网络接口信息
type InterfaceInfo struct {
	Name     string `json:"name"`
	IP       string `json:"ip"`
	Status   string `json:"status"` // up/down
	RxBytes  uint64 `json:"rxBytes"`
	TxBytes  uint64 `json:"txBytes"`
	RxErrors uint64 `json:"rxErrors"`
	TxErrors uint64 `json:"txErrors"`
}

// Problem 检测到的问题
type Problem struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"` // critical/warning/info
	Category    string `json:"category"` // cpu/memory/disk/network
	Title       string `json:"title"`
	Description string `json:"description"`
	Value       string `json:"value,omitempty"`
	Threshold   string `json:"threshold,omitempty"`
}

// Suggestion 优化建议
type Suggestion struct {
	ID          string `json:"id"`
	Priority    string `json:"priority"` // high/medium/low
	Category    string `json:"category"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action,omitempty"`
}

// DiagConfig 诊断配置
type DiagConfig struct {
	MaxHistory      int           `json:"maxHistory"`      // 最大历史记录数
	HistoryInterval time.Duration `json:"historyInterval"` // 历史记录间隔
	CPUThreshold    float64       `json:"cpuThreshold"`    // CPU告警阈值
	MemThreshold    float64       `json:"memThreshold"`    // 内存告警阈值
	DiskThreshold   float64       `json:"diskThreshold"`   // 磁盘告警阈值
}

// DefaultConfig 默认配置
func DefaultConfig() DiagConfig {
	return DiagConfig{
		MaxHistory:      100,
		HistoryInterval: time.Hour,
		CPUThreshold:    80.0,
		MemThreshold:    85.0,
		DiskThreshold:   90.0,
	}
}

// ========== Manager ==========

// Manager 诊断管理器
type Manager struct {
	config  DiagConfig
	history []DiagnosticReport
	mu      sync.RWMutex
	stopCh  chan struct{}
}

// NewManager 创建诊断管理器
func NewManager(cfg DiagConfig) *Manager {
	return &Manager{
		config:  cfg,
		history: make([]DiagnosticReport, 0, cfg.MaxHistory),
		stopCh:  make(chan struct{}),
	}
}

// Start 启动诊断管理器
func (m *Manager) Start() {
	log.Println("[diagnostics] 管理器启动")
	// 启动后台历史清理
	go m.cleanupLoop()
}

// Stop 停止诊断管理器
func (m *Manager) Stop() {
	close(m.stopCh)
	log.Println("[diagnostics] 管理器停止")
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.cleanupHistory()
		}
	}
}

func (m *Manager) cleanupHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.history) > m.config.MaxHistory {
		m.history = m.history[len(m.history)-m.config.MaxHistory:]
	}
}

// ========== 核心诊断方法 ==========

// RunDiagnostic 执行一键诊断
func (m *Manager) RunDiagnostic() (*DiagnosticReport, error) {
	start := time.Now()

	report := &DiagnosticReport{
		ID:        fmt.Sprintf("diag-%d", start.UnixNano()),
		Timestamp: start,
	}

	// 执行各项检测
	report.CPU = m.diagnoseCPU()
	report.Memory = m.diagnoseMemory()
	report.Disk = m.diagnoseDisk()
	report.Network = m.diagnoseNetwork()

	// 检测问题
	report.Problems = m.detectProblems(report)
	if report.Problems == nil {
		report.Problems = make([]Problem, 0)
	}

	// 计算健康评分
	report.Score = m.calculateScore(report)
	report.Status = scoreToStatus(report.Score)

	// 生成建议
	report.Suggestions = m.generateSuggestions(report)
	if report.Suggestions == nil {
		report.Suggestions = make([]Suggestion, 0)
	}

	// 生成摘要
	report.Summary = m.generateSummary(report)

	report.Duration = time.Since(start)

	// 保存历史
	m.addToHistory(*report)

	return report, nil
}

// diagnoseCPU 诊断CPU
func (m *Manager) diagnoseCPU() *CPUDiag {
	diag := &CPUDiag{
		Cores: runtime.NumCPU(),
	}

	// 读取负载平均值
	if loadAvg, err := readLoadAvg(); err == nil {
		diag.LoadAvg1 = loadAvg[0]
		diag.LoadAvg5 = loadAvg[1]
		diag.LoadAvg15 = loadAvg[2]
		// 使用负载估算CPU使用率
		diag.Usage = (loadAvg[0] / float64(diag.Cores)) * 100
		if diag.Usage > 100 {
			diag.Usage = 100
		}
	}

	// 读取CPU温度
	if temp, err := readCPUTemp(); err == nil {
		diag.Temperature = temp
	}

	// 评分
	diag.Score, diag.Status = scoreCPU(diag)
	return diag
}

// diagnoseMemory 诊断内存
func (m *Manager) diagnoseMemory() *MemoryDiag {
	diag := &MemoryDiag{}

	if memInfo, err := readMemInfo(); err == nil {
		diag.Total = memInfo["MemTotal"]
		diag.Available = memInfo["MemAvailable"]
		diag.Used = diag.Total - diag.Available
		if diag.Total > 0 {
			diag.UsedPercent = float64(diag.Used) / float64(diag.Total) * 100
		}
		diag.SwapTotal = memInfo["SwapTotal"]
		diag.SwapUsed = diag.SwapTotal - memInfo["SwapFree"]
	}

	diag.Score, diag.Status = scoreMemory(diag)
	return diag
}

// diagnoseDisk 诊断磁盘
func (m *Manager) diagnoseDisk() *DiskDiag {
	diag := &DiskDiag{
		Partitions: make([]PartitionInfo, 0),
	}

	partitions, err := readDiskPartitions()
	if err != nil {
		return diag
	}

	for _, p := range partitions {
		usage, err := readDiskUsage(p.Mountpoint)
		if err != nil {
			continue
		}
		info := PartitionInfo{
			MountPoint:  p.Mountpoint,
			Device:      p.Device,
			FSType:      p.FSType,
			Total:       usage.Total,
			Used:        usage.Used,
			Free:        usage.Free,
			UsedPercent: usage.UsedPercent,
		}
		diag.Partitions = append(diag.Partitions, info)
		diag.TotalSpace += usage.Total
		diag.UsedSpace += usage.Used
	}

	if diag.TotalSpace > 0 {
		diag.UsedPercent = float64(diag.UsedSpace) / float64(diag.TotalSpace) * 100
	}

	diag.Score, diag.Status = scoreDisk(diag)
	return diag
}

// diagnoseNetwork 诊断网络
func (m *Manager) diagnoseNetwork() *NetworkDiag {
	diag := &NetworkDiag{
		Interfaces: make([]InterfaceInfo, 0),
	}

	interfaces, err := readNetworkInterfaces()
	if err == nil {
		diag.Interfaces = interfaces
	}

	// 检测连通性
	diag.Connectivity = checkConnectivity()
	if diag.Connectivity {
		diag.Latency = measureLatency()
	}

	diag.Score, diag.Status = scoreNetwork(diag)
	return diag
}

// ========== 评分系统 ==========

func (m *Manager) calculateScore(report *DiagnosticReport) int {
	scores := []struct {
		weight int
		score  int
	}{
		{30, report.CPU.Score},
		{30, report.Memory.Score},
		{25, report.Disk.Score},
		{15, report.Network.Score},
	}

	totalWeight := 0
	totalScore := 0
	for _, s := range scores {
		totalWeight += s.weight
		totalScore += s.weight * s.score
	}

	if totalWeight == 0 {
		return 0
	}
	return totalScore / totalWeight
}

func scoreCPU(diag *CPUDiag) (int, string) {
	score := 100

	// 使用率评分
	switch {
	case diag.Usage > 95:
		score -= 50
	case diag.Usage > 80:
		score -= 30
	case diag.Usage > 60:
		score -= 15
	case diag.Usage > 40:
		score -= 5
	}

	// 负载评分
	loadPerCore := diag.LoadAvg1 / float64(diag.Cores)
	switch {
	case loadPerCore > 2.0:
		score -= 30
	case loadPerCore > 1.0:
		score -= 15
	case loadPerCore > 0.7:
		score -= 5
	}

	// 温度评分
	switch {
	case diag.Temperature > 90:
		score -= 30
	case diag.Temperature > 80:
		score -= 20
	case diag.Temperature > 70:
		score -= 10
	case diag.Temperature > 60:
		score -= 5
	}

	score = int(math.Max(0, float64(score)))
	return score, scoreToStatus(score)
}

func scoreMemory(diag *MemoryDiag) (int, string) {
	score := 100

	switch {
	case diag.UsedPercent > 95:
		score -= 50
	case diag.UsedPercent > 85:
		score -= 30
	case diag.UsedPercent > 75:
		score -= 15
	case diag.UsedPercent > 60:
		score -= 5
	}

	// Swap使用评分
	if diag.SwapTotal > 0 {
		swapPercent := float64(diag.SwapUsed) / float64(diag.SwapTotal) * 100
		switch {
		case swapPercent > 80:
			score -= 20
		case swapPercent > 50:
			score -= 10
		case swapPercent > 20:
			score -= 5
		}
	}

	score = int(math.Max(0, float64(score)))
	return score, scoreToStatus(score)
}

func scoreDisk(diag *DiskDiag) (int, string) {
	score := 100

	// 整体使用率评分
	switch {
	case diag.UsedPercent > 95:
		score -= 40
	case diag.UsedPercent > 90:
		score -= 25
	case diag.UsedPercent > 80:
		score -= 10
	}

	// 检查单个分区
	for _, p := range diag.Partitions {
		switch {
		case p.UsedPercent > 98:
			score -= 20
		case p.UsedPercent > 95:
			score -= 10
		case p.UsedPercent > 90:
			score -= 5
		}
	}

	score = int(math.Max(0, float64(score)))
	return score, scoreToStatus(score)
}

func scoreNetwork(diag *NetworkDiag) (int, string) {
	score := 100

	if !diag.Connectivity {
		score -= 50
	}

	// 延迟评分
	switch {
	case diag.Latency > 500:
		score -= 30
	case diag.Latency > 100:
		score -= 15
	case diag.Latency > 50:
		score -= 5
	}

	// 检查接口错误
	for _, iface := range diag.Interfaces {
		if iface.Status == "down" {
			score -= 10
		}
		if iface.RxErrors > 0 || iface.TxErrors > 0 {
			score -= 5
		}
	}

	score = int(math.Max(0, float64(score)))
	return score, scoreToStatus(score)
}

func scoreToStatus(score int) string {
	switch {
	case score >= 90:
		return "excellent"
	case score >= 75:
		return "good"
	case score >= 60:
		return "fair"
	case score >= 40:
		return "poor"
	default:
		return "critical"
	}
}

// ========== 问题检测 ==========

func (m *Manager) detectProblems(report *DiagnosticReport) []Problem {
	var problems []Problem

	// CPU问题
	if report.CPU.Usage > m.config.CPUThreshold {
		problems = append(problems, Problem{
			ID:          "high-cpu-usage",
			Severity:    severity(report.CPU.Usage, m.config.CPUThreshold, 95),
			Category:    "cpu",
			Title:       "CPU使用率过高",
			Description: fmt.Sprintf("当前CPU使用率 %.1f%%，超过阈值 %.1f%%", report.CPU.Usage, m.config.CPUThreshold),
			Value:       fmt.Sprintf("%.1f%%", report.CPU.Usage),
			Threshold:   fmt.Sprintf("%.1f%%", m.config.CPUThreshold),
		})
	}

	if report.CPU.Temperature > 80 {
		problems = append(problems, Problem{
			ID:          "high-cpu-temp",
			Severity:    severity(report.CPU.Temperature, 80, 95),
			Category:    "cpu",
			Title:       "CPU温度过高",
			Description: fmt.Sprintf("CPU温度 %.1f°C，建议检查散热", report.CPU.Temperature),
			Value:       fmt.Sprintf("%.1f°C", report.CPU.Temperature),
			Threshold:   "80°C",
		})
	}

	// 内存问题
	if report.Memory.UsedPercent > m.config.MemThreshold {
		problems = append(problems, Problem{
			ID:          "high-memory-usage",
			Severity:    severity(report.Memory.UsedPercent, m.config.MemThreshold, 95),
			Category:    "memory",
			Title:       "内存使用率过高",
			Description: fmt.Sprintf("当前内存使用率 %.1f%%，可用内存不足", report.Memory.UsedPercent),
			Value:       fmt.Sprintf("%.1f%%", report.Memory.UsedPercent),
			Threshold:   fmt.Sprintf("%.1f%%", m.config.MemThreshold),
		})
	}

	// 磁盘问题
	for _, p := range report.Disk.Partitions {
		if p.UsedPercent > m.config.DiskThreshold {
			problems = append(problems, Problem{
				ID:          "high-disk-usage-" + sanitizeID(p.MountPoint),
				Severity:    severity(p.UsedPercent, m.config.DiskThreshold, 98),
				Category:    "disk",
				Title:       fmt.Sprintf("磁盘空间不足 (%s)", p.MountPoint),
				Description: fmt.Sprintf("分区 %s 使用率 %.1f%%，剩余空间 %s", p.MountPoint, p.UsedPercent, formatBytes(p.Free)),
				Value:       fmt.Sprintf("%.1f%%", p.UsedPercent),
				Threshold:   fmt.Sprintf("%.1f%%", m.config.DiskThreshold),
			})
		}
	}

	// 网络问题
	if !report.Network.Connectivity {
		problems = append(problems, Problem{
			ID:          "no-network",
			Severity:    "critical",
			Category:    "network",
			Title:       "网络连接断开",
			Description: "无法检测到外部网络连接",
		})
	}

	if report.Network.Latency > 100 {
		problems = append(problems, Problem{
			ID:          "high-latency",
			Severity:    severity(report.Network.Latency, 100, 500),
			Category:    "network",
			Title:       "网络延迟过高",
			Description: fmt.Sprintf("当前延迟 %.1fms，网络响应缓慢", report.Network.Latency),
			Value:       fmt.Sprintf("%.1fms", report.Network.Latency),
			Threshold:   "100ms",
		})
	}

	return problems
}

func severity(value, warnThreshold, criticalThreshold float64) string {
	switch {
	case value >= criticalThreshold:
		return "critical"
	case value >= warnThreshold:
		return "warning"
	default:
		return "info"
	}
}

// ========== 建议生成 ==========

func (m *Manager) generateSuggestions(report *DiagnosticReport) []Suggestion {
	var suggestions []Suggestion

	// CPU建议
	if report.CPU.Usage > 80 {
		suggestions = append(suggestions, Suggestion{
			ID:          "reduce-cpu-usage",
			Priority:    "high",
			Category:    "cpu",
			Title:       "降低CPU使用率",
			Description: "当前CPU负载较高，建议检查并关闭不必要的进程",
			Action:      "使用 top 或 htop 查看占用CPU的进程",
		})
	}

	if report.CPU.Temperature > 70 {
		suggestions = append(suggestions, Suggestion{
			ID:          "check-cooling",
			Priority:    "medium",
			Category:    "cpu",
			Title:       "检查散热系统",
			Description: "CPU温度偏高，建议检查风扇和散热膏",
			Action:      "清理风扇灰尘，检查散热膏是否需要更换",
		})
	}

	// 内存建议
	if report.Memory.UsedPercent > 75 {
		suggestions = append(suggestions, Suggestion{
			ID:          "free-memory",
			Priority:    "high",
			Category:    "memory",
			Title:       "释放内存",
			Description: "内存使用率较高，建议关闭不必要的应用或增加内存",
			Action:      "检查内存占用最高的进程，考虑重启内存泄漏的服务",
		})
	}

	// 磁盘建议
	for _, p := range report.Disk.Partitions {
		if p.UsedPercent > 85 {
			suggestions = append(suggestions, Suggestion{
				ID:          "cleanup-disk-" + sanitizeID(p.MountPoint),
				Priority:    "high",
				Category:    "disk",
				Title:       fmt.Sprintf("清理磁盘空间 (%s)", p.MountPoint),
				Description: fmt.Sprintf("分区 %s 空间不足，建议清理日志和临时文件", p.MountPoint),
				Action:      "清理 /var/log、/tmp，删除不需要的Docker镜像",
			})
			break // 只添加一次磁盘建议
		}
	}

	// 网络建议
	if !report.Network.Connectivity {
		suggestions = append(suggestions, Suggestion{
			ID:          "check-network",
			Priority:    "high",
			Category:    "network",
			Title:       "检查网络连接",
			Description: "无法连接外部网络，请检查网络配置和物理连接",
			Action:      "检查网线、路由器，验证DNS配置",
		})
	}

	return suggestions
}

// ========== 摘要生成 ==========

func (m *Manager) generateSummary(report *DiagnosticReport) string {
	var parts []string

	// 总体状态
	parts = append(parts, fmt.Sprintf("系统健康评分: %d/100 (%s)", report.Score, report.Status))

	// 问题统计
	if len(report.Problems) > 0 {
		critical := 0
		warnings := 0
		for _, p := range report.Problems {
			switch p.Severity {
			case "critical":
				critical++
			case "warning":
				warnings++
			}
		}
		parts = append(parts, fmt.Sprintf("发现问题: %d个 (严重: %d, 警告: %d)", len(report.Problems), critical, warnings))
	} else {
		parts = append(parts, "未发现问题")
	}

	// 建议统计
	if len(report.Suggestions) > 0 {
		parts = append(parts, fmt.Sprintf("优化建议: %d条", len(report.Suggestions)))
	}

	return strings.Join(parts, "; ")
}

// ========== 历史记录 ==========

func (m *Manager) addToHistory(report DiagnosticReport) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.history = append(m.history, report)

	if len(m.history) > m.config.MaxHistory {
		m.history = m.history[1:]
	}
}

// GetHistory 获取历史记录
func (m *Manager) GetHistory(limit int) []DiagnosticReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	start := len(m.history) - limit
	result := make([]DiagnosticReport, limit)
	copy(result, m.history[start:])
	return result
}

// GetLatestReport 获取最新报告
func (m *Manager) GetLatestReport() *DiagnosticReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.history) == 0 {
		return nil
	}

	report := m.history[len(m.history)-1]
	return &report
}

// GetTrend 获取趋势数据
func (m *Manager) GetTrend(hours int) []TrendPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	var points []TrendPoint

	for _, r := range m.history {
		if r.Timestamp.After(cutoff) {
			points = append(points, TrendPoint{
				Timestamp: r.Timestamp,
				Score:     r.Score,
				CPU:       r.CPU.Usage,
				Memory:    r.Memory.UsedPercent,
				Disk:      r.Disk.UsedPercent,
			})
		}
	}

	return points
}

// TrendPoint 趋势数据点
type TrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Score     int       `json:"score"`
	CPU       float64   `json:"cpu"`
	Memory    float64   `json:"memory"`
	Disk      float64   `json:"disk"`
}

// ========== 辅助函数 ==========

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

func sanitizeID(s string) string {
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, " ", "-")
	return strings.ToLower(s)
}

// ========== 系统信息读取 (可测试的接口) ==========

type diskPartition struct {
	Device     string
	Mountpoint string
	FSType     string
}

type diskUsage struct {
	Total       uint64
	Used        uint64
	Free        uint64
	UsedPercent float64
}

// 以下函数可以被测试替身覆盖
var (
	readLoadAvg = func() ([3]float64, error) {
		data, err := os.ReadFile("/proc/loadavg")
		if err != nil {
			return [3]float64{}, err
		}
		var avg [3]float64
		_, err = fmt.Sscanf(string(data), "%f %f %f", &avg[0], &avg[1], &avg[2])
		return avg, err
	}

	readCPUTemp = func() (float64, error) {
		// 尝试读取thermal zone
		data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
		if err != nil {
			return 0, err
		}
		var temp float64
		_, err = fmt.Sscanf(string(data), "%f", &temp)
		return temp / 1000.0, err
	}

	readMemInfo = func() (map[string]uint64, error) {
		data, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return nil, err
		}
		info := make(map[string]uint64)
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				key := strings.TrimSuffix(parts[0], ":")
				var val uint64
				_, err := fmt.Sscanf(parts[1], "%d", &val)
				if err == nil {
					info[key] = val * 1024 // 转换为字节
				}
			}
		}
		return info, nil
	}

	readDiskPartitions = func() ([]diskPartition, error) {
		data, err := os.ReadFile("/proc/mounts")
		if err != nil {
			return nil, err
		}
		var partitions []diskPartition
		lines := strings.Split(string(data), "\n")
		seen := make(map[string]bool)
		for _, line := range lines {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				device := parts[0]
				mountpoint := parts[1]
				fstype := parts[2]
				// 只统计真实文件系统
				if strings.HasPrefix(device, "/dev/") && !seen[mountpoint] {
					partitions = append(partitions, diskPartition{
						Device:     device,
						Mountpoint: mountpoint,
						FSType:     fstype,
					})
					seen[mountpoint] = true
				}
			}
		}
		return partitions, nil
	}

	readDiskUsage = func(mountpoint string) (diskUsage, error) {
		var stat syscallStatfs
		err := statfs(mountpoint, &stat)
		if err != nil {
			return diskUsage{}, err
		}
		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bavail * uint64(stat.Bsize)
		used := total - free
		var usedPercent float64
		if total > 0 {
			usedPercent = float64(used) / float64(total) * 100
		}
		return diskUsage{
			Total:       total,
			Used:        used,
			Free:        free,
			UsedPercent: usedPercent,
		}, nil
	}

	readNetworkInterfaces = func() ([]InterfaceInfo, error) {
		data, err := os.ReadFile("/proc/net/dev")
		if err != nil {
			return nil, err
		}
		var interfaces []InterfaceInfo
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if i < 2 { // 跳过标题行
				continue
			}
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				name := strings.TrimSpace(parts[0])
				if name == "lo" {
					continue
				}
				fields := strings.Fields(parts[1])
				if len(fields) >= 10 {
					info := InterfaceInfo{
						Name:   name,
						Status: "up",
					}
					fmt.Sscanf(fields[0], "%d", &info.RxBytes)
					fmt.Sscanf(fields[2], "%d", &info.RxErrors)
					fmt.Sscanf(fields[8], "%d", &info.TxBytes)
					fmt.Sscanf(fields[10], "%d", &info.TxErrors)
					interfaces = append(interfaces, info)
				}
			}
		}
		return interfaces, nil
	}

	checkConnectivity = func() bool {
		// 简单检查：尝试读取DNS配置
		_, err := os.ReadFile("/etc/resolv.conf")
		if err != nil {
			return false
		}
		// 实际场景中可以ping外部地址
		return true
	}

	measureLatency = func() float64 {
		// 简化实现：返回模拟值
		// 实际场景中可以ping DNS服务器测量延迟
		return 10.0
	}
)

// syscallStatfs 系统调用结构
type syscallStatfs struct {
	Type    int64
	Bsize   int64
	Blocks  uint64
	Bfree   uint64
	Bavail  uint64
	Files   uint64
	Ffree   uint64
	Fsid    [2]int32
	Namelen int64
	Frsize  int64
	Flags   int64
	Spare   [4]int64
}

var statfs = func(path string, stat *syscallStatfs) error {
	// 使用系统调用获取磁盘信息
	// 这里使用简化的实现
	return fmt.Errorf("not implemented")
}
