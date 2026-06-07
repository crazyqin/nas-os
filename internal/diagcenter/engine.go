// Package diagcenter 诊断引擎实现。
package diagcenter

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"go.uber.org/zap"
)

// Engine 诊断引擎.
type Engine struct {
	mu      sync.RWMutex
	config  *Config
	logger  *zap.Logger
	history []DiagResult
	diagID  int32 // 原子标志，防止并发诊断
}

// NewEngine 创建诊断引擎.
func NewEngine(logger *zap.Logger, config *Config) *Engine {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultConfig()
	}

	return &Engine{
		config:  config,
		logger:  logger,
		history: make([]DiagResult, 0, 100),
	}
}

// RunDiagnostic 执行完整诊断.
func (e *Engine) RunDiagnostic(ctx context.Context, categories []CheckCategory) (*DiagResult, error) {
	start := time.Now()
	resultID := uuid.New().String()

	e.logger.Info("开始系统诊断", zap.String("id", resultID))

	// 确定检查类别
	if len(categories) == 0 {
		categories = []CheckCategory{
			CategoryDisk,
			CategoryMemory,
			CategoryCPU,
			CategoryService,
			CategoryNetwork,
			CategoryRAID,
		}
	}

	// 执行各项检查
	checks := make([]CheckItem, 0, len(categories)*3)
	alerts := make([]Alert, 0)

	for _, cat := range categories {
		switch cat {
		case CategoryDisk:
			diskChecks, diskAlerts := e.checkDisks(ctx)
			checks = append(checks, diskChecks...)
			alerts = append(alerts, diskAlerts...)
		case CategoryMemory:
			memChecks, memAlerts := e.checkMemory(ctx)
			checks = append(checks, memChecks...)
			alerts = append(alerts, memAlerts...)
		case CategoryCPU:
			cpuChecks, cpuAlerts := e.checkCPU(ctx)
			checks = append(checks, cpuChecks...)
			alerts = append(alerts, cpuAlerts...)
		case CategoryService:
			svcChecks, svcAlerts := e.checkServices(ctx)
			checks = append(checks, svcChecks...)
			alerts = append(alerts, svcAlerts...)
		case CategoryNetwork:
			netChecks, netAlerts := e.checkNetwork(ctx)
			checks = append(checks, netChecks...)
			alerts = append(alerts, netAlerts...)
		case CategoryRAID:
			raidChecks, raidAlerts := e.checkRAID(ctx)
			checks = append(checks, raidChecks...)
			alerts = append(alerts, raidAlerts...)
		}
	}

	// 确定整体状态
	overallStatus := StatusHealthy
	for _, check := range checks {
		if check.Status == StatusFatal {
			overallStatus = StatusFatal
			break
		}
		if check.Status == StatusCritical {
			overallStatus = StatusCritical
		}
		if check.Status == StatusDegraded && overallStatus == StatusHealthy {
			overallStatus = StatusDegraded
		}
	}

	// 生成摘要
	summary := e.generateSummary(checks, overallStatus)

	result := &DiagResult{
		ID:        resultID,
		Timestamp: time.Now(),
		Status:    overallStatus,
		Checks:    checks,
		Alerts:    alerts,
		Summary:   summary,
		Duration:  time.Since(start),
	}

	// 保存到历史
	e.mu.Lock()
	e.history = append([]DiagResult{*result}, e.history...)
	if len(e.history) > 100 {
		e.history = e.history[:100]
	}
	e.mu.Unlock()

	e.logger.Info("诊断完成",
		zap.String("id", resultID),
		zap.String("status", string(overallStatus)),
		zap.Int("checks", len(checks)),
		zap.Int("alerts", len(alerts)),
		zap.Duration("duration", result.Duration),
	)

	return result, nil
}

// GetLatest 获取最近一次诊断结果.
func (e *Engine) GetLatest() *DiagResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.history) == 0 {
		return nil
	}
	return &e.history[0]
}

// GetHistory 获取诊断历史.
func (e *Engine) GetHistory(query HistoryQuery) *HistoryResponse {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if query.Limit <= 0 {
		query.Limit = 100
	}
	if query.Days <= 0 {
		query.Days = 30
	}

	cutoff := time.Now().AddDate(0, 0, -query.Days)
	results := make([]DiagResult, 0)

	for _, r := range e.history {
		if r.Timestamp.After(cutoff) {
			results = append(results, r)
			if len(results) >= query.Limit {
				break
			}
		}
	}

	return &HistoryResponse{
		Results:    results,
		TotalCount: len(results),
	}
}

// ========== 磁盘检查 ==========

func (e *Engine) checkDisks(ctx context.Context) ([]CheckItem, []Alert) {
	checks := make([]CheckItem, 0)
	alerts := make([]Alert, 0)

	// 获取磁盘分区信息
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		e.logger.Error("获取磁盘分区失败", zap.Error(err))
		return checks, alerts
	}

	// 检查磁盘使用率
	for _, p := range partitions {
		usage, err := disk.UsageWithContext(ctx, p.Mountpoint)
		if err != nil {
			continue
		}

		usedPercent := usage.UsedPercent
		status := StatusHealthy
		severity := SeverityInfo
		var remediation *Remediation

		if usedPercent > 95 {
			status = StatusCritical
			severity = SeverityCritical
			remediation = &Remediation{
				Title:       "磁盘空间严重不足",
				Description: fmt.Sprintf("%s 使用率已达 %.1f%%，需要立即清理", p.Mountpoint, usedPercent),
				Steps: []string{
					"1. 检查大文件: du -sh /* | sort -hr | head -20",
					"2. 清理日志文件: journalctl --vacuum-time=7d",
					"3. 清理 Docker: docker system prune -a",
					"4. 考虑扩展存储或迁移数据",
				},
				QuickFix: fmt.Sprintf("docker system prune -a && journalctl --vacuum-time=7d"),
			}
		} else if usedPercent > 85 {
			status = StatusDegraded
			severity = SeverityWarning
			remediation = &Remediation{
				Title:       "磁盘空间不足",
				Description: fmt.Sprintf("%s 使用率已达 %.1f%%，建议清理", p.Mountpoint, usedPercent),
				Steps: []string{
					"1. 检查大文件: du -sh /* | sort -hr | head -20",
					"2. 清理临时文件: rm -rf /tmp/*",
					"3. 清理包缓存: apt clean / yum clean all",
				},
				QuickFix: "apt clean && rm -rf /tmp/*",
			}
		}

		check := CheckItem{
			Category:    CategoryDisk,
			Name:        fmt.Sprintf("磁盘使用率 - %s", p.Mountpoint),
			Status:      status,
			Severity:    severity,
			Message:     fmt.Sprintf("使用率: %.1f%%", usedPercent),
			Value:       usedPercent,
			Threshold:   85.0,
			Remediation: remediation,
		}
		checks = append(checks, check)

		if severity >= SeverityWarning {
			alerts = append(alerts, Alert{
				ID:          uuid.New().String(),
				Category:    string(CategoryDisk),
				Severity:    severity,
				Title:       check.Name,
				Description: check.Message,
				Timestamp:   time.Now(),
				Remediation: remediation,
			})
		}
	}

	// 检查 SMART 状态（如果可用）
	smartChecks, smartAlerts := e.checkSMART(ctx)
	checks = append(checks, smartChecks...)
	alerts = append(alerts, smartAlerts...)

	return checks, alerts
}

// checkSMART 检查磁盘 SMART 状态.
func (e *Engine) checkSMART(ctx context.Context) ([]CheckItem, []Alert) {
	checks := make([]CheckItem, 0)
	alerts := make([]Alert, 0)

	// 尝试运行 smartctl
	cmd := exec.CommandContext(ctx, "smartctl", "--scan")
	output, err := cmd.Output()
	if err != nil {
		// smartctl 不可用，跳过
		e.logger.Debug("smartctl 不可用，跳过 SMART 检查")
		return checks, alerts
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析设备名
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		device := parts[0]

		// 获取 SMART 状态
		smartCmd := exec.CommandContext(ctx, "smartctl", "-H", device)
		smartOutput, err := smartCmd.CombinedOutput()
		if err != nil {
			continue
		}

		smartStr := string(smartOutput)
		status := StatusHealthy
		severity := SeverityInfo
		message := "SMART 状态正常"
		var remediation *Remediation

		if strings.Contains(smartStr, "FAILED") {
			status = StatusFatal
			severity = SeverityFatal
			message = "SMART 检测到磁盘故障"
			remediation = &Remediation{
				Title:       "磁盘即将故障",
				Description: fmt.Sprintf("%s SMART 检测到故障，建议立即更换", device),
				Steps: []string{
					"1. 备份该磁盘上的所有数据",
					"2. 准备替换磁盘",
					"3. 如果是 RAID，替换后重建阵列",
					"4. 联系硬件供应商更换",
				},
				QuickFix: fmt.Sprintf("smartctl -a %s", device),
			}
		} else if strings.Contains(smartStr, "PREFAIL") {
			status = StatusDegraded
			severity = SeverityWarning
			message = "SMART 检测到预故障状态"
			remediation = &Remediation{
				Title:       "磁盘健康状况下降",
				Description: fmt.Sprintf("%s 检测到预故障指标，建议关注", device),
				Steps: []string{
					"1. 定期检查 SMART 数据: smartctl -a " + device,
					"2. 备份重要数据",
					"3. 考虑更换磁盘",
				},
				QuickFix: fmt.Sprintf("smartctl -a %s", device),
			}
		}

		check := CheckItem{
			Category:    CategoryDisk,
			Name:        fmt.Sprintf("SMART 状态 - %s", device),
			Status:      status,
			Severity:    severity,
			Message:     message,
			Value:       string(status),
			Threshold:   "healthy",
			Remediation: remediation,
		}
		checks = append(checks, check)

		if severity >= SeverityWarning {
			alerts = append(alerts, Alert{
				ID:          uuid.New().String(),
				Category:    string(CategoryDisk),
				Severity:    severity,
				Title:       check.Name,
				Description: message,
				Timestamp:   time.Now(),
				Remediation: remediation,
			})
		}
	}

	return checks, alerts
}

// ========== 内存检查 ==========

func (e *Engine) checkMemory(ctx context.Context) ([]CheckItem, []Alert) {
	checks := make([]CheckItem, 0)
	alerts := make([]Alert, 0)

	vmStat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		e.logger.Error("获取内存信息失败", zap.Error(err))
		return checks, alerts
	}

	usedPercent := vmStat.UsedPercent
	status := StatusHealthy
	severity := SeverityInfo
	var remediation *Remediation

	if usedPercent > e.config.MemCritPercent {
		status = StatusCritical
		severity = SeverityCritical
		remediation = &Remediation{
			Title:       "内存使用率过高",
			Description: fmt.Sprintf("内存使用率已达 %.1f%%，可能导致系统不稳定", usedPercent),
			Steps: []string{
				"1. 查看内存占用最高的进程: ps aux --sort=-%mem | head -20",
				"2. 检查内存泄漏: valgrind --tool=memcheck <程序>",
				"3. 增加 swap 空间",
				"4. 考虑增加物理内存",
			},
			QuickFix: "sync && echo 3 > /proc/sys/vm/drop_caches",
		}
	} else if usedPercent > e.config.MemWarnPercent {
		status = StatusDegraded
		severity = SeverityWarning
		remediation = &Remediation{
			Title:       "内存使用率偏高",
			Description: fmt.Sprintf("内存使用率已达 %.1f%%，建议关注", usedPercent),
			Steps: []string{
				"1. 查看内存占用: free -h",
				"2. 清理缓存: sync && echo 3 > /proc/sys/vm/drop_caches",
				"3. 检查是否有内存泄漏的进程",
			},
			QuickFix: "sync && echo 3 > /proc/sys/vm/drop_caches",
		}
	}

	check := CheckItem{
		Category:    CategoryMemory,
		Name:        "内存使用率",
		Status:      status,
		Severity:    severity,
		Message:     fmt.Sprintf("使用率: %.1f%% (已用: %s / 总计: %s)", usedPercent, formatBytes(vmStat.Used), formatBytes(vmStat.Total)),
		Value:       usedPercent,
		Threshold:   e.config.MemWarnPercent,
		Remediation: remediation,
	}
	checks = append(checks, check)

	if severity >= SeverityWarning {
		alerts = append(alerts, Alert{
			ID:          uuid.New().String(),
			Category:    string(CategoryMemory),
			Severity:    severity,
			Title:       "内存使用率过高",
			Description: check.Message,
			Timestamp:   time.Now(),
			Remediation: remediation,
		})
	}

	return checks, alerts
}

// ========== CPU 检查 ==========

func (e *Engine) checkCPU(ctx context.Context) ([]CheckItem, []Alert) {
	checks := make([]CheckItem, 0)
	alerts := make([]Alert, 0)

	// 获取 CPU 使用率
	percent, err := cpu.PercentWithContext(ctx, 5*time.Second, false)
	if err != nil {
		e.logger.Error("获取 CPU 使用率失败", zap.Error(err))
		return checks, alerts
	}

	if len(percent) == 0 {
		return checks, alerts
	}

	usedPercent := percent[0]
	status := StatusHealthy
	severity := SeverityInfo
	var remediation *Remediation

	if usedPercent > e.config.CPUCritPercent {
		status = StatusCritical
		severity = SeverityCritical
		remediation = &Remediation{
			Title:       "CPU 使用率过高",
			Description: fmt.Sprintf("CPU 使用率已达 %.1f%%，可能导致系统响应缓慢", usedPercent),
			Steps: []string{
				"1. 查看 CPU 占用最高的进程: top -bn1 | head -20",
				"2. 检查是否有异常进程: ps aux --sort=-%cpu | head -20",
				"3. 检查是否有挖矿程序",
				"4. 考虑升级 CPU 或优化应用",
			},
			QuickFix: "top -bn1 | head -20",
		}
	} else if usedPercent > e.config.CPUWarnPercent {
		status = StatusDegraded
		severity = SeverityWarning
		remediation = &Remediation{
			Title:       "CPU 使用率偏高",
			Description: fmt.Sprintf("CPU 使用率已达 %.1f%%，建议关注", usedPercent),
			Steps: []string{
				"1. 查看 CPU 占用: top -bn1 | head -20",
				"2. 检查是否有异常进程",
			},
			QuickFix: "top -bn1 | head -20",
		}
	}

	// 获取负载平均值
	loadAvg, _ := loadAvgWithContext(ctx)
	loadMsg := ""
	if len(loadAvg) >= 3 {
		loadMsg = fmt.Sprintf(" 负载: %.2f %.2f %.2f", loadAvg[0], loadAvg[1], loadAvg[2])
	}

	check := CheckItem{
		Category:    CategoryCPU,
		Name:        "CPU 使用率",
		Status:      status,
		Severity:    severity,
		Message:     fmt.Sprintf("使用率: %.1f%%%s", usedPercent, loadMsg),
		Value:       usedPercent,
		Threshold:   e.config.CPUWarnPercent,
		Remediation: remediation,
	}
	checks = append(checks, check)

	if severity >= SeverityWarning {
		alerts = append(alerts, Alert{
			ID:          uuid.New().String(),
			Category:    string(CategoryCPU),
			Severity:    severity,
			Title:       "CPU 使用率过高",
			Description: check.Message,
			Timestamp:   time.Now(),
			Remediation: remediation,
		})
	}

	return checks, alerts
}

// loadAvgWithContext 获取系统负载平均值.
func loadAvgWithContext(ctx context.Context) ([]float64, error) {
	// 从 /proc/loadavg 读取
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return nil, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return nil, fmt.Errorf("invalid loadavg format")
	}

	var loadAvg []float64
	for i := 0; i < 3; i++ {
		var v float64
		_, err := fmt.Sscanf(fields[i], "%f", &v)
		if err != nil {
			return nil, err
		}
		loadAvg = append(loadAvg, v)
	}
	return loadAvg, nil
}

// ========== 服务检查 ==========

func (e *Engine) checkServices(ctx context.Context) ([]CheckItem, []Alert) {
	checks := make([]CheckItem, 0)
	alerts := make([]Alert, 0)

	for _, svc := range e.config.RequiredServices {
		status := StatusHealthy
		severity := SeverityInfo
		message := fmt.Sprintf("服务 %s 运行中", svc)
		var remediation *Remediation

		// 检查服务状态
		running := isServiceRunning(ctx, svc)
		if !running {
			status = StatusCritical
			severity = SeverityCritical
			message = fmt.Sprintf("服务 %s 未运行", svc)
			remediation = &Remediation{
				Title:       fmt.Sprintf("服务 %s 已停止", svc),
				Description: fmt.Sprintf("关键服务 %s 未运行，可能影响系统功能", svc),
				Steps: []string{
					fmt.Sprintf("1. 检查服务状态: systemctl status %s", svc),
					fmt.Sprintf("2. 查看服务日志: journalctl -u %s -n 50", svc),
					fmt.Sprintf("3. 尝试重启服务: systemctl restart %s", svc),
					fmt.Sprintf("4. 如果重启失败，检查配置文件"),
				},
				QuickFix: fmt.Sprintf("systemctl restart %s", svc),
			}
		}

		check := CheckItem{
			Category:    CategoryService,
			Name:        fmt.Sprintf("服务状态 - %s", svc),
			Status:      status,
			Severity:    severity,
			Message:     message,
			Value:       running,
			Threshold:   true,
			Remediation: remediation,
		}
		checks = append(checks, check)

		if severity >= SeverityWarning {
			alerts = append(alerts, Alert{
				ID:          uuid.New().String(),
				Category:    string(CategoryService),
				Severity:    severity,
				Title:       check.Name,
				Description: message,
				Timestamp:   time.Now(),
				Remediation: remediation,
			})
		}
	}

	return checks, alerts
}

// isServiceRunning 检查服务是否运行.
func isServiceRunning(ctx context.Context, service string) bool {
	cmd := exec.CommandContext(ctx, "systemctl", "is-active", service)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "active"
}

// ========== 网络检查 ==========

func (e *Engine) checkNetwork(ctx context.Context) ([]CheckItem, []Alert) {
	checks := make([]CheckItem, 0)
	alerts := make([]Alert, 0)

	for _, target := range e.config.NetworkTargets {
		status := StatusHealthy
		severity := SeverityInfo
		message := fmt.Sprintf("网络连通性正常 - %s", target)
		var remediation *Remediation

		// 使用 TCP 检测连通性
		conn, err := net.DialTimeout("tcp", target+":53", 5*time.Second)
		if err != nil {
			status = StatusCritical
			severity = SeverityCritical
			message = fmt.Sprintf("网络连通性失败 - %s: %v", target, err)
			remediation = &Remediation{
				Title:       "网络连通性异常",
				Description: fmt.Sprintf("无法连接到 %s，可能存在网络问题", target),
				Steps: []string{
					"1. 检查网络接口: ip addr show",
					"2. 检查路由表: ip route show",
					"3. 测试 DNS: nslookup " + target,
					"4. 检查防火墙: iptables -L -n",
				},
				QuickFix: fmt.Sprintf("ping -c 3 %s", target),
			}
		} else {
			conn.Close()
		}

		check := CheckItem{
			Category:    CategoryNetwork,
			Name:        fmt.Sprintf("网络连通性 - %s", target),
			Status:      status,
			Severity:    severity,
			Message:     message,
			Value:       target,
			Threshold:   "reachable",
			Remediation: remediation,
		}
		checks = append(checks, check)

		if severity >= SeverityWarning {
			alerts = append(alerts, Alert{
				ID:          uuid.New().String(),
				Category:    string(CategoryNetwork),
				Severity:    severity,
				Title:       check.Name,
				Description: message,
				Timestamp:   time.Now(),
				Remediation: remediation,
			})
		}
	}

	return checks, alerts
}

// ========== RAID 检查 ==========

func (e *Engine) checkRAID(ctx context.Context) ([]CheckItem, []Alert) {
	checks := make([]CheckItem, 0)
	alerts := make([]Alert, 0)

	// 读取 /proc/mdstat
	data, err := os.ReadFile("/proc/mdstat")
	if err != nil {
		// 没有 RAID 配置，跳过
		e.logger.Debug("未检测到 RAID 配置")
		return checks, alerts
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "md") {
			continue
		}

		// 解析 RAID 设备
		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}

		device := parts[0]
		status := StatusHealthy
		severity := SeverityInfo
		message := fmt.Sprintf("RAID 设备 %s 状态正常", device)
		var remediation *Remediation

		// 检查是否有降级标志
		if strings.Contains(line, "_") || strings.Contains(line, "F") {
			status = StatusDegraded
			severity = SeverityWarning
			message = fmt.Sprintf("RAID 设备 %s 处于降级状态", device)
			remediation = &Remediation{
				Title:       "RAID 阵列降级",
				Description: fmt.Sprintf("RAID 设备 %s 有磁盘故障，需要更换", device),
				Steps: []string{
					fmt.Sprintf("1. 查看 RAID 详情: cat /proc/mdstat"),
					fmt.Sprintf("2. 检查磁盘状态: mdadm --detail /dev/%s", device),
					fmt.Sprintf("3. 替换故障磁盘"),
					fmt.Sprintf("4. 重建阵列: mdadm --manage /dev/%s --add /dev/sdX", device),
				},
				QuickFix: fmt.Sprintf("mdadm --detail /dev/%s", device),
			}
		}

		check := CheckItem{
			Category:    CategoryRAID,
			Name:        fmt.Sprintf("RAID 状态 - %s", device),
			Status:      status,
			Severity:    severity,
			Message:     message,
			Value:       line,
			Threshold:   "clean",
			Remediation: remediation,
		}
		checks = append(checks, check)

		if severity >= SeverityWarning {
			alerts = append(alerts, Alert{
				ID:          uuid.New().String(),
				Category:    string(CategoryRAID),
				Severity:    severity,
				Title:       check.Name,
				Description: message,
				Timestamp:   time.Now(),
				Remediation: remediation,
			})
		}
	}

	return checks, alerts
}

// ========== 辅助函数 ==========

// generateSummary 生成诊断摘要.
func (e *Engine) generateSummary(checks []CheckItem, status DiagStatus) string {
	total := len(checks)
	healthy := 0
	warning := 0
	critical := 0
	fatal := 0

	for _, check := range checks {
		switch check.Status {
		case StatusHealthy:
			healthy++
		case StatusDegraded:
			warning++
		case StatusCritical:
			critical++
		case StatusFatal:
			fatal++
		}
	}

	var statusDesc string
	switch status {
	case StatusHealthy:
		statusDesc = "系统健康"
	case StatusDegraded:
		statusDesc = "系统存在警告"
	case StatusCritical:
		statusDesc = "系统存在严重问题"
	case StatusFatal:
		statusDesc = "系统存在致命故障"
	}

	return fmt.Sprintf("%s - 共 %d 项检查: %d 正常, %d 警告, %d 严重, %d 致命",
		statusDesc, total, healthy, warning, critical, fatal)
}

// formatBytes 格式化字节数.
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
