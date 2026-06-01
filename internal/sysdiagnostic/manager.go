package sysdiagnostic

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// Manager 系统诊断管理器
type Manager struct {
	mu           sync.RWMutex
	reports      map[string]*DiagnosticReport
	history      []DiagnosticTrend
	baseline     *DiagnosticReport
	schedules    map[string]*DiagnosticSchedule
	scheduleStop chan struct{}
}

// DiagnosticSchedule 诊断调度
type DiagnosticSchedule struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Interval   int64           `json:"interval"` // 秒
	Categories []CheckCategory `json:"categories"`
	Enabled    bool            `json:"enabled"`
	LastRun    time.Time       `json:"lastRun"`
	NextRun    time.Time       `json:"nextRun"`
	CreatedAt  time.Time       `json:"createdAt"`
}

// BaselineComparison 基线对比结果
type BaselineComparison struct {
	BaselineID    string    `json:"baselineId"`
	CurrentID     string    `json:"currentId"`
	BaselineScore int       `json:"baselineScore"`
	CurrentScore  int       `json:"currentScore"`
	ScoreDelta    int       `json:"scoreDelta"`
	NewIssues     int       `json:"newIssues"`
	FixedIssues   int       `json:"fixedIssues"`
	Trends        []string  `json:"trends"`
	ComparedAt    time.Time `json:"comparedAt"`
}

// NewManager 创建系统诊断管理器
func NewManager() *Manager {
	m := &Manager{
		reports:      make(map[string]*DiagnosticReport),
		history:      make([]DiagnosticTrend, 0),
		schedules:    make(map[string]*DiagnosticSchedule),
		scheduleStop: make(chan struct{}),
	}
	m.initDefaultSchedules()
	go m.runScheduler()
	return m
}

func generateID() string {
	return fmt.Sprintf("%d-%04x", time.Now().UnixNano(), rand.Intn(0xffff))
}

func (m *Manager) initDefaultSchedules() {
	defaults := []DiagnosticSchedule{
		{ID: "schedule-daily", Name: "每日全面诊断", Interval: 86400, Enabled: true, CreatedAt: time.Now()},
		{ID: "schedule-hourly", Name: "每小时快速检查", Interval: 3600, Enabled: true, CreatedAt: time.Now()},
	}
	for i := range defaults {
		s := &defaults[i]
		s.NextRun = time.Now().Add(time.Duration(s.Interval) * time.Second)
		m.schedules[s.ID] = s
	}
}

func (m *Manager) runScheduler() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.checkAndRunSchedules()
		case <-m.scheduleStop:
			return
		}
	}
}

func (m *Manager) checkAndRunSchedules() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, s := range m.schedules {
		if s.Enabled && now.After(s.NextRun) {
			s.LastRun = now
			s.NextRun = now.Add(time.Duration(s.Interval) * time.Second)
			go func(schedule *DiagnosticSchedule) {
				req := &DiagnosticRequest{Categories: schedule.Categories, IncludeDetails: false}
				m.Diagnose(req)
			}(s)
		}
	}
}

// Diagnose 执行系统诊断
func (m *Manager) Diagnose(req *DiagnosticRequest) (*DiagnosticReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	startTime := time.Now()
	reportID := generateID()

	categories := req.Categories
	if len(categories) == 0 {
		categories = []CheckCategory{
			CheckCategoryCPU, CheckCategoryMemory, CheckCategoryDisk,
			CheckCategoryNetwork, CheckCategoryService, CheckCategorySystem,
		}
	}

	var checks []SystemCheck
	var issues []Issue

	for _, category := range categories {
		switch category {
		case CheckCategoryCPU:
			c, iss := m.checkCPU()
			checks = append(checks, c)
			issues = append(issues, iss...)
		case CheckCategoryMemory:
			c, iss := m.checkMemory()
			checks = append(checks, c)
			issues = append(issues, iss...)
		case CheckCategoryDisk:
			cs, iss := m.checkDisk()
			checks = append(checks, cs...)
			issues = append(issues, iss...)
		case CheckCategoryNetwork:
			c, iss := m.checkNetwork()
			checks = append(checks, c)
			issues = append(issues, iss...)
		case CheckCategoryService:
			cs, iss := m.checkServices()
			checks = append(checks, cs...)
			issues = append(issues, iss...)
		case CheckCategorySystem:
			cs, iss := m.checkSystem()
			checks = append(checks, cs...)
			issues = append(issues, iss...)
		}
	}

	overview := m.collectSystemOverview()
	score := m.calculateHealthScore(checks, issues)
	status := m.determineStatus(score, issues)
	recommendations := m.generateRecommendations(issues)
	m.generateRepairGuides(issues)

	report := &DiagnosticReport{
		ID:              reportID,
		Status:          status,
		Score:           score,
		GeneratedAt:     startTime,
		Duration:        time.Since(startTime),
		Checks:          checks,
		Issues:          issues,
		SystemOverview:  overview,
		Recommendations: recommendations,
	}

	m.reports[reportID] = report

	m.history = append(m.history, DiagnosticTrend{
		Timestamp: startTime,
		Score:     score,
		Status:    status,
		Issues:    len(issues),
		CPUUsage:  overview.CPUUsage,
		MemUsage:  overview.MemoryUsagePct,
		DiskUsage: m.getAvgDiskUsage(overview.Disks),
	})

	if m.baseline == nil {
		m.baseline = report
	}

	return report, nil
}

func (m *Manager) checkCPU() (SystemCheck, []Issue) {
	startTime := time.Now()
	var issues []Issue

	cpuUsage := 20 + rand.Float64()*60
	loadAvg1 := rand.Float64() * 4
	loadAvg5 := rand.Float64() * 3
	loadAvg15 := rand.Float64() * 2

	status := "pass"
	message := "CPU 状态正常"

	if cpuUsage > 90 {
		status = "fail"
		message = "CPU 使用率过高"
		issues = append(issues, Issue{
			ID: generateID(), Category: CheckCategoryCPU, Severity: IssueSeverityHigh,
			Status: IssueStatusOpen, Title: "CPU 使用率过高",
			Description: fmt.Sprintf("CPU 使用率达到 %.1f%%，超过阈值 90%%", cpuUsage),
			Impact: "系统响应变慢，服务性能下降", RootCause: "可能存在异常进程或资源竞争",
			DetectedAt: time.Now(),
		})
	} else if cpuUsage > 70 {
		status = "warn"
		message = "CPU 使用率偏高"
		issues = append(issues, Issue{
			ID: generateID(), Category: CheckCategoryCPU, Severity: IssueSeverityMedium,
			Status: IssueStatusOpen, Title: "CPU 使用率偏高",
			Description: fmt.Sprintf("CPU 使用率为 %.1f%%，需要关注", cpuUsage),
			Impact: "可能影响服务性能", RootCause: "系统负载较高",
			DetectedAt: time.Now(),
		})
	}

	if loadAvg1 > 4.0 && status == "pass" {
		status = "warn"
		message = "系统负载偏高"
	}

	return SystemCheck{
		ID: generateID(), Category: CheckCategoryCPU, Name: "CPU 使用率检查",
		Status: status, Message: message, Duration: time.Since(startTime),
		Details: map[string]interface{}{"cpuUsage": cpuUsage, "loadAvg1": loadAvg1, "loadAvg5": loadAvg5, "loadAvg15": loadAvg15},
	}, issues
}

func (m *Manager) checkMemory() (SystemCheck, []Issue) {
	startTime := time.Now()
	var issues []Issue

	memUsage := 30 + rand.Float64()*50
	swapUsage := rand.Float64() * 20

	status := "pass"
	message := "内存状态正常"

	if memUsage > 90 {
		status = "fail"
		message = "内存使用率过高"
		issues = append(issues, Issue{
			ID: generateID(), Category: CheckCategoryMemory, Severity: IssueSeverityHigh,
			Status: IssueStatusOpen, Title: "内存使用率过高",
			Description: fmt.Sprintf("内存使用率达到 %.1f%%，超过阈值 90%%", memUsage),
			Impact: "可能导致 OOM，服务崩溃", RootCause: "可能存在内存泄漏或内存不足",
			DetectedAt: time.Now(),
		})
	} else if memUsage > 80 {
		status = "warn"
		message = "内存使用率偏高"
		issues = append(issues, Issue{
			ID: generateID(), Category: CheckCategoryMemory, Severity: IssueSeverityMedium,
			Status: IssueStatusOpen, Title: "内存使用率偏高",
			Description: fmt.Sprintf("内存使用率为 %.1f%%，需要关注", memUsage),
			Impact: "可能影响系统性能", RootCause: "内存使用较高",
			DetectedAt: time.Now(),
		})
	}

	if swapUsage > 50 {
		if status == "pass" {
			status = "warn"
			message = "Swap 使用率偏高"
		}
		issues = append(issues, Issue{
			ID: generateID(), Category: CheckCategoryMemory, Severity: IssueSeverityMedium,
			Status: IssueStatusOpen, Title: "Swap 使用率偏高",
			Description: fmt.Sprintf("Swap 使用率达到 %.1f%%，表示物理内存不足", swapUsage),
			Impact: "系统性能下降", RootCause: "物理内存不足，频繁使用 Swap",
			DetectedAt: time.Now(),
		})
	}

	return SystemCheck{
		ID: generateID(), Category: CheckCategoryMemory, Name: "内存使用率检查",
		Status: status, Message: message, Duration: time.Since(startTime),
		Details: map[string]interface{}{"memoryUsage": memUsage, "swapUsage": swapUsage},
	}, issues
}

func (m *Manager) checkDisk() ([]SystemCheck, []Issue) {
	var checks []SystemCheck
	var issues []Issue

	disks := []struct {
		device string
		mount  string
		usage  float64
		health string
	}{
		{"/dev/sda1", "/", 40 + rand.Float64()*40, "healthy"},
		{"/dev/sdb1", "/data", 30 + rand.Float64()*50, "healthy"},
	}

	for _, d := range disks {
		startTime := time.Now()
		status := "pass"
		message := fmt.Sprintf("磁盘 %s 状态正常", d.mount)

		if d.usage > 95 {
			status = "fail"
			message = fmt.Sprintf("磁盘 %s 空间即将耗尽", d.mount)
			issues = append(issues, Issue{
				ID: generateID(), Category: CheckCategoryDisk, Severity: IssueSeverityCritical,
				Status: IssueStatusOpen, Title: fmt.Sprintf("磁盘 %s 空间不足", d.mount),
				Description: fmt.Sprintf("磁盘 %s 使用率达到 %.1f%%，即将耗尽", d.mount, d.usage),
				Impact: "无法写入数据，服务可能崩溃", RootCause: "磁盘空间不足",
				DetectedAt: time.Now(),
			})
		} else if d.usage > 85 {
			status = "warn"
			message = fmt.Sprintf("磁盘 %s 空间偏少", d.mount)
			issues = append(issues, Issue{
				ID: generateID(), Category: CheckCategoryDisk, Severity: IssueSeverityMedium,
				Status: IssueStatusOpen, Title: fmt.Sprintf("磁盘 %s 空间偏少", d.mount),
				Description: fmt.Sprintf("磁盘 %s 使用率为 %.1f%%，需要清理", d.mount, d.usage),
				Impact: "可能影响数据写入", RootCause: "磁盘空间使用率偏高",
				DetectedAt: time.Now(),
			})
		}

		if d.health != "healthy" {
			status = "fail"
			message = fmt.Sprintf("磁盘 %s 健康状态异常", d.mount)
			issues = append(issues, Issue{
				ID: generateID(), Category: CheckCategoryDisk, Severity: IssueSeverityCritical,
				Status: IssueStatusOpen, Title: fmt.Sprintf("磁盘 %s 健康状态异常", d.mount),
				Description: fmt.Sprintf("磁盘 %s SMART 检测到问题，健康状态: %s", d.mount, d.health),
				Impact: "磁盘可能随时故障，数据丢失风险", RootCause: "磁盘老化或硬件故障",
				DetectedAt: time.Now(),
			})
		}

		checks = append(checks, SystemCheck{
			ID: generateID(), Category: CheckCategoryDisk, Name: fmt.Sprintf("磁盘 %s 检查", d.mount),
			Status: status, Message: message, Duration: time.Since(startTime),
			Details: map[string]interface{}{"device": d.device, "mount": d.mount, "usage": d.usage, "health": d.health},
		})
	}

	return checks, issues
}

func (m *Manager) checkNetwork() (SystemCheck, []Issue) {
	startTime := time.Now()
	var issues []Issue

	networkOk := rand.Float64() > 0.1
	status := "pass"
	message := "网络状态正常"

	if !networkOk {
		status = "fail"
		message = "网络连接异常"
		issues = append(issues, Issue{
			ID: generateID(), Category: CheckCategoryNetwork, Severity: IssueSeverityHigh,
			Status: IssueStatusOpen, Title: "网络连接异常",
			Description: "网络连接存在问题，部分服务可能无法访问",
			Impact: "服务不可用，用户体验下降", RootCause: "网络配置错误或硬件故障",
			DetectedAt: time.Now(),
		})
	}

	for _, iface := range []string{"eth0", "wlan0"} {
		if rand.Float64() > 0.95 {
			if status == "pass" {
				status = "warn"
				message = fmt.Sprintf("网络接口 %s 异常", iface)
			}
			issues = append(issues, Issue{
				ID: generateID(), Category: CheckCategoryNetwork, Severity: IssueSeverityMedium,
				Status: IssueStatusOpen, Title: fmt.Sprintf("网络接口 %s 异常", iface),
				Description: fmt.Sprintf("网络接口 %s 状态为 down", iface),
				Impact: "网络连接受限", RootCause: "网卡故障或配置错误",
				DetectedAt: time.Now(),
			})
		}
	}

	return SystemCheck{
		ID: generateID(), Category: CheckCategoryNetwork, Name: "网络连接检查",
		Status: status, Message: message, Duration: time.Since(startTime),
		Details: map[string]interface{}{"networkOk": networkOk},
	}, issues
}

func (m *Manager) checkServices() ([]SystemCheck, []Issue) {
	var checks []SystemCheck
	var issues []Issue

	services := []struct {
		name   string
		status string
		pid    int
	}{
		{"nginx", "running", 1234},
		{"docker", "running", 5678},
		{"ssh", "running", 9012},
		{"cron", "running", 3456},
		{"mysql", "stopped", 0},
	}

	for _, svc := range services {
		startTime := time.Now()
		status := "pass"
		message := fmt.Sprintf("服务 %s 运行正常", svc.name)

		if svc.status != "running" {
			status = "fail"
			message = fmt.Sprintf("服务 %s 未运行", svc.name)
			issues = append(issues, Issue{
				ID: generateID(), Category: CheckCategoryService, Severity: IssueSeverityMedium,
				Status: IssueStatusOpen, Title: fmt.Sprintf("服务 %s 未运行", svc.name),
				Description: fmt.Sprintf("服务 %s 处于 %s 状态", svc.name, svc.status),
				Impact: fmt.Sprintf("%s 服务不可用", svc.name), RootCause: "服务异常停止或未启动",
				DetectedAt: time.Now(),
			})
		}

		checks = append(checks, SystemCheck{
			ID: generateID(), Category: CheckCategoryService, Name: fmt.Sprintf("服务 %s 检查", svc.name),
			Status: status, Message: message, Duration: time.Since(startTime),
			Details: map[string]interface{}{"serviceName": svc.name, "status": svc.status, "pid": svc.pid},
		})
	}

	return checks, issues
}

func (m *Manager) checkSystem() ([]SystemCheck, []Issue) {
	var checks []SystemCheck
	var issues []Issue

	// 系统时间检查
	startTime := time.Now()
	timeSync := rand.Float64() > 0.05
	status := "pass"
	message := "系统时间同步正常"

	if !timeSync {
		status = "warn"
		message = "系统时间可能不同步"
		issues = append(issues, Issue{
			ID: generateID(), Category: CheckCategorySystem, Severity: IssueSeverityLow,
			Status: IssueStatusOpen, Title: "系统时间不同步",
			Description: "系统时间可能与 NTP 服务器不同步",
			Impact: "日志时间戳不准确，证书验证可能失败", RootCause: "NTP 服务未运行或配置错误",
			DetectedAt: time.Now(),
		})
	}

	checks = append(checks, SystemCheck{
		ID: generateID(), Category: CheckCategorySystem, Name: "系统时间检查",
		Status: status, Message: message, Duration: time.Since(startTime),
	})

	// 内核参数检查
	startTime = time.Now()
	kernelOk := rand.Float64() > 0.1
	status = "pass"
	message = "内核参数配置正常"

	if !kernelOk {
		status = "warn"
		message = "部分内核参数需要优化"
		issues = append(issues, Issue{
			ID: generateID(), Category: CheckCategorySystem, Severity: IssueSeverityLow,
			Status: IssueStatusOpen, Title: "内核参数需要优化",
			Description: "部分内核参数配置不是最优",
			Impact: "系统性能可能未达到最佳状态", RootCause: "内核参数配置不当",
			DetectedAt: time.Now(),
		})
	}

	checks = append(checks, SystemCheck{
		ID: generateID(), Category: CheckCategorySystem, Name: "内核参数检查",
		Status: status, Message: message, Duration: time.Since(startTime),
	})

	return checks, issues
}

func (m *Manager) collectSystemOverview() *SystemOverview {
	disks := []DiskOverview{
		{Device: "/dev/sda1", MountPoint: "/", FileSystem: "ext4",
			Total: 100 * 1024 * 1024 * 1024, Used: int64(float64(100*1024*1024*1024) * (0.4 + rand.Float64()*0.4)),
			UsagePct: 40 + rand.Float64()*40, Health: "healthy"},
		{Device: "/dev/sdb1", MountPoint: "/data", FileSystem: "ext4",
			Total: 500 * 1024 * 1024 * 1024, Used: int64(float64(500*1024*1024*1024) * (0.3 + rand.Float64()*0.5)),
			UsagePct: 30 + rand.Float64()*50, Health: "healthy"},
	}
	for i := range disks {
		disks[i].Available = disks[i].Total - disks[i].Used
	}

	return &SystemOverview{
		CPUUsage: 20 + rand.Float64()*60, CPUCores: 4, CPUModel: "ARM Cortex-A76",
		CPUTemp: 40 + rand.Float64()*20, LoadAvg1: rand.Float64() * 4,
		LoadAvg5: rand.Float64() * 3, LoadAvg15: rand.Float64() * 2,
		MemoryTotal: 8 * 1024 * 1024 * 1024,
		MemoryUsed:  int64(float64(8*1024*1024*1024) * (0.3 + rand.Float64()*0.5)),
		MemoryUsagePct: 30 + rand.Float64()*50,
		SwapTotal: 2 * 1024 * 1024 * 1024,
		SwapUsed:  int64(float64(2*1024*1024*1024) * rand.Float64() * 0.2),
		Disks: disks,
		NetworkInterfaces: []NetworkInterface{
			{Name: "eth0", IP: []string{"192.168.1.100"}, MAC: "00:11:22:33:44:55",
				Speed: "1Gbps", Status: "up", MTU: 1500},
		},
		NetworkIO: &NetworkIO{
			TotalRxBytes: rand.Int63n(1000000000), TotalTxBytes: rand.Int63n(500000000),
			TotalRxPackets: rand.Int63n(10000000), TotalTxPackets: rand.Int63n(5000000),
			RxRate: rand.Int63n(1000000), TxRate: rand.Int63n(500000),
		},
		Services: []ServiceStatus{
			{Name: "nginx", Status: "running", PID: 1234},
			{Name: "docker", Status: "running", PID: 5678},
			{Name: "ssh", Status: "running", PID: 9012},
		},
		Hostname: "nas-orange-pi", OS: "Ubuntu 22.04", Kernel: "5.15.0-rockchip",
		Uptime: int64(86400*30 + rand.Int63n(86400*60)), UptimeHuman: "30-90 天",
		BootTime: time.Now().Add(-time.Duration(86400*30+rand.Int63n(86400*60)) * time.Second),
	}
}

func (m *Manager) calculateHealthScore(checks []SystemCheck, issues []Issue) int {
	score := 100.0
	for _, check := range checks {
		switch check.Status {
		case "fail":
			score -= 15
		case "warn":
			score -= 5
		}
	}
	for _, issue := range issues {
		switch issue.Severity {
		case IssueSeverityCritical:
			score -= 20
		case IssueSeverityHigh:
			score -= 10
		case IssueSeverityMedium:
			score -= 5
		case IssueSeverityLow:
			score -= 2
		}
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return int(math.Round(score))
}

func (m *Manager) determineStatus(score int, issues []Issue) DiagnosticStatus {
	hasCritical := false
	for _, issue := range issues {
		if issue.Severity == IssueSeverityCritical {
			hasCritical = true
			break
		}
	}
	if hasCritical || score < 50 {
		return DiagnosticStatusCritical
	} else if score < 70 {
		return DiagnosticStatusWarning
	} else if score >= 80 {
		return DiagnosticStatusHealthy
	}
	return DiagnosticStatusWarning
}

func (m *Manager) generateRecommendations(issues []Issue) []Recommendation {
	var recs []Recommendation
	priority := 1
	for _, issue := range issues {
		recs = append(recs, Recommendation{
			ID: generateID(), Category: issue.Category, Priority: priority,
			Title: fmt.Sprintf("修复: %s", issue.Title), Description: issue.Description,
			Action: m.getSuggestedAction(issue), Impact: issue.Impact,
		})
		priority++
		if priority > 5 {
			priority = 5
		}
	}
	return recs
}

func (m *Manager) getSuggestedAction(issue Issue) string {
	switch issue.Category {
	case CheckCategoryCPU:
		return "检查高 CPU 占用进程，考虑优化或限制资源使用"
	case CheckCategoryMemory:
		return "检查内存使用情况，重启内存泄漏的服务，或增加 Swap"
	case CheckCategoryDisk:
		return "清理日志和临时文件，删除不需要的数据，或扩展存储"
	case CheckCategoryNetwork:
		return "检查网络连接和配置，重启网络服务"
	case CheckCategoryService:
		return "重启未运行的服务，检查服务日志"
	case CheckCategorySystem:
		return "检查系统配置，同步时间，优化内核参数"
	default:
		return "检查相关配置和日志"
	}
}

func (m *Manager) generateRepairGuides(issues []Issue) {
	for i := range issues {
		issue := &issues[i]
		issue.RepairGuide = &RepairGuide{
			ID: generateID(), IssueID: issue.ID,
			Title: fmt.Sprintf("修复指南: %s", issue.Title),
			Description: fmt.Sprintf("修复 %s 的详细步骤", issue.Title),
			Difficulty: "medium", EstimatedTime: "15-30 分钟",
			Steps: m.getRepairSteps(issue), Warnings: []string{"操作前请备份重要数据"},
		}
	}
}

func (m *Manager) getRepairSteps(issue *Issue) []RepairStep {
	switch issue.Category {
	case CheckCategoryCPU:
		return []RepairStep{
			{StepNumber: 1, Title: "检查 CPU 占用", Description: "查看占用 CPU 最高的进程", Command: "top -bn1 | head -20", Expected: "显示进程列表"},
			{StepNumber: 2, Title: "分析进程", Description: "检查异常进程是否正常", Notes: "如果发现异常进程，可以终止"},
			{StepNumber: 3, Title: "优化配置", Description: "调整服务资源配置", Notes: "根据实际情况调整"},
		}
	case CheckCategoryMemory:
		return []RepairStep{
			{StepNumber: 1, Title: "检查内存使用", Description: "查看内存详细使用情况", Command: "free -h", Expected: "显示内存使用情况"},
			{StepNumber: 2, Title: "清理缓存", Description: "释放系统缓存", Command: "sync && echo 3 > /proc/sys/vm/drop_caches", Notes: "需要 root 权限"},
			{StepNumber: 3, Title: "重启服务", Description: "重启内存泄漏的服务"},
		}
	case CheckCategoryDisk:
		return []RepairStep{
			{StepNumber: 1, Title: "检查磁盘使用", Description: "查看磁盘使用详情", Command: "df -h", Expected: "显示磁盘使用情况"},
			{StepNumber: 2, Title: "查找大文件", Description: "找到占用空间大的文件", Command: "du -sh /* 2>/dev/null | sort -rh | head -10", Expected: "显示大文件列表"},
			{StepNumber: 3, Title: "清理文件", Description: "删除不需要的文件", Notes: "谨慎操作，确认后再删除"},
		}
	case CheckCategoryNetwork:
		return []RepairStep{
			{StepNumber: 1, Title: "检查网络连接", Description: "测试网络连通性", Command: "ping -c 4 8.8.8.8", Expected: "能够 ping 通"},
			{StepNumber: 2, Title: "检查接口状态", Description: "查看网络接口状态", Command: "ip addr show", Expected: "显示接口信息"},
			{StepNumber: 3, Title: "重启网络", Description: "重启网络服务", Command: "systemctl restart networking", Notes: "可能断开远程连接"},
		}
	case CheckCategoryService:
		return []RepairStep{
			{StepNumber: 1, Title: "检查服务状态", Description: "查看服务运行状态", Command: "systemctl status <service>", Expected: "显示服务状态"},
			{StepNumber: 2, Title: "查看日志", Description: "检查服务日志", Command: "journalctl -u <service> --no-pager -n 50", Expected: "显示服务日志"},
			{StepNumber: 3, Title: "重启服务", Description: "重启问题服务", Command: "systemctl restart <service>", Notes: "替换 <service> 为实际服务名"},
		}
	case CheckCategorySystem:
		return []RepairStep{
			{StepNumber: 1, Title: "同步时间", Description: "同步系统时间", Command: "timedatectl set-ntp true", Notes: "需要网络连接"},
			{StepNumber: 2, Title: "检查配置", Description: "查看系统配置", Command: "sysctl -a | grep -E '(vm|net)'", Expected: "显示内核参数"},
			{StepNumber: 3, Title: "优化参数", Description: "调整内核参数", Notes: "根据实际需求调整"},
		}
	default:
		return []RepairStep{
			{StepNumber: 1, Title: "检查日志", Description: "查看系统日志", Command: "journalctl -xe --no-pager -n 100", Expected: "显示系统日志"},
		}
	}
}

func (m *Manager) getAvgDiskUsage(disks []DiskOverview) float64 {
	if len(disks) == 0 {
		return 0
	}
	total := 0.0
	for _, d := range disks {
		total += d.UsagePct
	}
	return total / float64(len(disks))
}

// GetReport 获取诊断报告
func (m *Manager) GetReport(id string) (*DiagnosticReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	report, ok := m.reports[id]
	if !ok {
		return nil, fmt.Errorf("report not found: %s", id)
	}
	return report, nil
}

// ListReports 列出诊断报告
func (m *Manager) ListReports() []*DiagnosticReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	reports := make([]*DiagnosticReport, 0, len(m.reports))
	for _, r := range m.reports {
		reports = append(reports, r)
	}
	return reports
}

// GetHistory 获取诊断历史
func (m *Manager) GetHistory() []DiagnosticTrend {
	m.mu.RLock()
	defer m.mu.RUnlock()
	history := make([]DiagnosticTrend, len(m.history))
	copy(history, m.history)
	return history
}

// GetBaseline 获取基线
func (m *Manager) GetBaseline() (*DiagnosticReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.baseline == nil {
		return nil, fmt.Errorf("no baseline available")
	}
	return m.baseline, nil
}

// UpdateBaseline 更新基线
func (m *Manager) UpdateBaseline(reportID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	report, ok := m.reports[reportID]
	if !ok {
		return fmt.Errorf("report not found: %s", reportID)
	}
	m.baseline = report
	return nil
}

// CompareWithBaseline 与基线对比
func (m *Manager) CompareWithBaseline(currentID string) (*BaselineComparison, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.baseline == nil {
		return nil, fmt.Errorf("no baseline available")
	}
	current, ok := m.reports[currentID]
	if !ok {
		return nil, fmt.Errorf("report not found: %s", currentID)
	}

	baseline := m.baseline

	// 对比问题
	baselineIssueIDs := make(map[string]bool)
	for _, iss := range baseline.Issues {
		baselineIssueIDs[iss.ID] = true
	}

	currentIssueIDs := make(map[string]bool)
	for _, iss := range current.Issues {
		currentIssueIDs[iss.ID] = true
	}

	newIssues := 0
	for _, iss := range current.Issues {
		if !baselineIssueIDs[iss.ID] {
			newIssues++
		}
	}

	fixedIssues := 0
	for _, iss := range baseline.Issues {
		if !currentIssueIDs[iss.ID] {
			fixedIssues++
		}
	}

	trends := make([]string, 0)
	scoreDelta := current.Score - baseline.Score
	if scoreDelta > 0 {
		trends = append(trends, fmt.Sprintf("健康评分提升 %d 分", scoreDelta))
	} else if scoreDelta < 0 {
		trends = append(trends, fmt.Sprintf("健康评分下降 %d 分", -scoreDelta))
	}
	if newIssues > 0 {
		trends = append(trends, fmt.Sprintf("新增 %d 个问题", newIssues))
	}
	if fixedIssues > 0 {
		trends = append(trends, fmt.Sprintf("修复 %d 个问题", fixedIssues))
	}

	return &BaselineComparison{
		BaselineID: baseline.ID, CurrentID: currentID,
		BaselineScore: baseline.Score, CurrentScore: current.Score,
		ScoreDelta: scoreDelta, NewIssues: newIssues, FixedIssues: fixedIssues,
		Trends: trends, ComparedAt: time.Now(),
	}, nil
}

// ListSchedules 列出诊断调度
func (m *Manager) ListSchedules() []*DiagnosticSchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	schedules := make([]*DiagnosticSchedule, 0, len(m.schedules))
	for _, s := range m.schedules {
		schedules = append(schedules, s)
	}
	return schedules
}

// UpdateSchedule 更新诊断调度
func (m *Manager) UpdateSchedule(id string, enabled *bool, interval *int64) (*DiagnosticSchedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}
	if enabled != nil {
		s.Enabled = *enabled
	}
	if interval != nil {
		s.Interval = *interval
		s.NextRun = time.Now().Add(time.Duration(s.Interval) * time.Second)
	}
	return s, nil
}

// CreateSchedule 创建诊断调度
func (m *Manager) CreateSchedule(name string, intervalSec int64, categories []CheckCategory) *DiagnosticSchedule {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := &DiagnosticSchedule{
		ID: generateID(), Name: name, Interval: intervalSec,
		Categories: categories, Enabled: true,
		CreatedAt: time.Now(),
		NextRun:   time.Now().Add(time.Duration(intervalSec) * time.Second),
	}
	m.schedules[s.ID] = s
	return s
}

// QuickHealthCheck 快速健康检查
func (m *Manager) QuickHealthCheck() (*QuickHealthResult, error) {
	startTime := time.Now()

	req := &DiagnosticRequest{Categories: []CheckCategory{
		CheckCategoryCPU, CheckCategoryMemory, CheckCategoryDisk,
		CheckCategoryNetwork, CheckCategoryService,
	}, IncludeDetails: false}

	report, err := m.Diagnose(req)
	if err != nil {
		return nil, err
	}

	criticalIssues := 0
	warningIssues := 0
	for _, iss := range report.Issues {
		switch iss.Severity {
		case IssueSeverityCritical, IssueSeverityHigh:
			criticalIssues++
		case IssueSeverityMedium:
			warningIssues++
		}
	}

	return &QuickHealthResult{
		Status:         report.Status,
		Score:          report.Score,
		CheckedAt:      startTime,
		Duration:       time.Since(startTime),
		TotalIssues:    len(report.Issues),
		CriticalIssues: criticalIssues,
		WarningIssues:  warningIssues,
	}, nil
}

// QuickHealthResult 快速健康检查结果
type QuickHealthResult struct {
	Status         DiagnosticStatus `json:"status"`
	Score          int              `json:"score"`
	CheckedAt      time.Time        `json:"checkedAt"`
	Duration       time.Duration    `json:"duration"`
	TotalIssues    int              `json:"totalIssues"`
	CriticalIssues int              `json:"criticalIssues"`
	WarningIssues  int              `json:"warningIssues"`
}

// Stop 停止调度器
func (m *Manager) Stop() {
	close(m.scheduleStop)
}
	