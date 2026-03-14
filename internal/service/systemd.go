// Package service 提供系统服务管理功能
package service

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// SystemdBackend systemd 服务后端
type SystemdBackend struct {
	systemctlPath string
}

// NewSystemdBackend 创建 systemd 后端
func NewSystemdBackend() (*SystemdBackend, error) {
	// 检查 systemctl 是否可用
	systemctlPath, err := exec.LookPath("systemctl")
	if err != nil {
		return nil, fmt.Errorf("systemctl 不可用: %w", err)
	}

	return &SystemdBackend{
		systemctlPath: systemctlPath,
	}, nil
}

// Start 启动服务
func (b *SystemdBackend) Start(name string) error {
	// 尝试标准服务名，如果失败则尝试添加 .service 后缀
	serviceName := b.normalizeServiceName(name)

	cmd := exec.Command(b.systemctlPath, "start", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl start 失败: %s: %w", string(output), err)
	}
	return nil
}

// Stop 停止服务
func (b *SystemdBackend) Stop(name string) error {
	serviceName := b.normalizeServiceName(name)

	cmd := exec.Command(b.systemctlPath, "stop", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl stop 失败: %s: %w", string(output), err)
	}
	return nil
}

// Restart 重启服务
func (b *SystemdBackend) Restart(name string) error {
	serviceName := b.normalizeServiceName(name)

	cmd := exec.Command(b.systemctlPath, "restart", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl restart 失败: %s: %w", string(output), err)
	}
	return nil
}

// Status 获取服务状态
func (b *SystemdBackend) Status(name string) (*ServiceStatus, error) {
	serviceName := b.normalizeServiceName(name)

	status := &ServiceStatus{}

	// 获取服务状态
	cmd := exec.Command(b.systemctlPath, "show", serviceName,
		"--property=ActiveState,SubState,MainPID,ExecMainStartTimestamp,MemoryCurrent,CPUUsageNSec,Result")
	output, err := cmd.Output()
	if err != nil {
		status.LastError = err.Error()
		return status, nil // 返回空状态而不是错误
	}

	// 解析输出
	props := b.parseProperties(string(output))

	// 运行状态
	status.Running = props["ActiveState"] == "active" && props["SubState"] == "running"

	// PID
	if pid, err := strconv.Atoi(props["MainPID"]); err == nil && pid > 0 {
		status.PID = pid
	}

	// 启动时间
	if startTime := props["ExecMainStartTimestamp"]; startTime != "" && startTime != "n/a" {
		if t, err := time.Parse(time.RFC3339Nano, startTime); err == nil {
			status.StartedAt = t
			status.Uptime = time.Since(t)
		}
	}

	// 内存使用
	if mem := props["MemoryCurrent"]; mem != "" && mem != "[not set]" && mem != "infinity" {
		if memBytes, err := strconv.ParseUint(mem, 10, 64); err == nil {
			status.Memory = memBytes
		}
	}

	// CPU 使用
	if cpu := props["CPUUsageNSec"]; cpu != "" && cpu != "[not set]" {
		if cpuNsec, err := strconv.ParseUint(cpu, 10, 64); err == nil {
			// 转换为百分比（近似值）
			// 注意：这是累计 CPU 时间，不是实时使用率
			// 实时 CPU 使用率需要更复杂的计算
			status.CPU = float64(cpuNsec) / 1e9 // 转换为秒
		}
	}

	// 最后错误
	if result := props["Result"]; result != "" && result != "success" {
		status.LastError = result
	}

	// 如果服务正在运行，获取更详细的信息
	if status.Running && status.PID > 0 {
		b.enrichStatus(status)
	}

	return status, nil
}

// Enable 启用服务开机自启
func (b *SystemdBackend) Enable(name string) error {
	serviceName := b.normalizeServiceName(name)

	cmd := exec.Command(b.systemctlPath, "enable", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl enable 失败: %s: %w", string(output), err)
	}
	return nil
}

// Disable 禁用服务开机自启
func (b *SystemdBackend) Disable(name string) error {
	serviceName := b.normalizeServiceName(name)

	cmd := exec.Command(b.systemctlPath, "disable", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl disable 失败: %s: %w", string(output), err)
	}
	return nil
}

// IsEnabled 检查服务是否开机自启
func (b *SystemdBackend) IsEnabled(name string) (bool, error) {
	serviceName := b.normalizeServiceName(name)

	cmd := exec.Command(b.systemctlPath, "is-enabled", serviceName)
	output, err := cmd.Output()
	if err != nil {
		// is-enabled 返回非零表示未启用
		return false, nil
	}

	result := strings.TrimSpace(string(output))
	return result == "enabled" || result == "enabled-runtime" || result == "static", nil
}

// IsRunning 检查服务是否运行中
func (b *SystemdBackend) IsRunning(name string) (bool, error) {
	serviceName := b.normalizeServiceName(name)

	cmd := exec.Command(b.systemctlPath, "is-active", serviceName)
	output, err := cmd.Output()
	if err != nil {
		return false, nil
	}

	return strings.TrimSpace(string(output)) == "active", nil
}

// List 列出所有服务
func (b *SystemdBackend) List() ([]*Service, error) {
	// 获取所有服务单元
	cmd := exec.Command(b.systemctlPath, "list-units", "--type=service", "--all",
		"--no-pager", "--no-legend")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("systemctl list-units 失败: %w", err)
	}

	services := make([]*Service, 0)
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		unitName := fields[0]
		// 只处理 .service 单元
		if !strings.HasSuffix(unitName, ".service") {
			continue
		}

		name := strings.TrimSuffix(unitName, ".service")
		loadState := fields[1]
		activeState := fields[2]
		subState := fields[3]

		// 跳过未加载的服务
		if loadState == "not-found" || loadState == "masked" {
			continue
		}

		// 描述可能包含空格，从第4个字段开始
		description := ""
		if len(fields) > 4 {
			description = strings.Join(fields[4:], " ")
		}

		svc := &Service{
			Name:        name,
			Description: description,
			Type:        "systemd",
			UnitFile:    unitName,
			Status: ServiceStatus{
				Running: activeState == "active" && subState == "running",
			},
		}

		// 检查是否启用
		enabled, _ := b.IsEnabled(name)
		svc.Enabled = enabled

		services = append(services, svc)
	}

	return services, nil
}

// Get 获取单个服务信息
func (b *SystemdBackend) Get(name string) (*Service, error) {
	serviceName := b.normalizeServiceName(name)

	// 获取服务详细信息
	cmd := exec.Command(b.systemctlPath, "show", serviceName,
		"--property=Description,LoadState,ActiveState,SubState,UnitFileState")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取服务 %s 信息失败: %w", name, err)
	}

	props := b.parseProperties(string(output))

	svc := &Service{
		Name:     name,
		Type:     "systemd",
		UnitFile: serviceName,
	}

	svc.Description = props["Description"]
	svc.Status.Running = props["ActiveState"] == "active" && props["SubState"] == "running"
	svc.Enabled = props["UnitFileState"] == "enabled" || props["UnitFileState"] == "enabled-runtime"

	// 获取更详细的状态
	status, err := b.Status(name)
	if err == nil {
		svc.Status = *status
	}

	return svc, nil
}

// normalizeServiceName 标准化服务名称
func (b *SystemdBackend) normalizeServiceName(name string) string {
	// 如果已经包含后缀，直接返回
	if strings.Contains(name, ".") {
		return name
	}
	// 默认添加 .service 后缀
	return name + ".service"
}

// parseProperties 解析 systemctl show 输出的属性
func (b *SystemdBackend) parseProperties(output string) map[string]string {
	props := make(map[string]string)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			props[parts[0]] = parts[1]
		}
	}

	return props
}

// enrichStatus 使用其他命令丰富状态信息
func (b *SystemdBackend) enrichStatus(status *ServiceStatus) {
	if status.PID <= 0 {
		return
	}

	// 使用 /proc 获取进程信息
	// 读取 /proc/[pid]/stat 获取 CPU 和内存信息
	statFile := fmt.Sprintf("/proc/%d/stat", status.PID)
	cmd := exec.Command("cat", statFile)
	output, err := cmd.Output()
	if err != nil {
		return
	}

	// 解析 /proc/[pid]/stat
	// 格式: pid (comm) state ppid pgrp session tty_nr tpgid flags ...
	// 我们需要第 14 和 15 个字段 (utime, stime)
	fields := strings.Fields(string(output))
	if len(fields) < 24 {
		return
	}

	// 获取内存使用 (RSS，第24个字段，单位是页)
	if rss, err := strconv.ParseUint(fields[23], 10, 64); err == nil {
		// 获取系统页大小
		pageSize := uint64(4096) // 默认 4KB
		if ps := exec.Command("getconf", "PAGESIZE"); ps.Run() == nil {
			if out, err := ps.Output(); err == nil {
				if ps, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64); err == nil {
					pageSize = ps
				}
			}
		}
		status.Memory = rss * pageSize
	}
}

// GetServiceLogs 获取服务日志
func (b *SystemdBackend) GetServiceLogs(name string, lines int, follow bool) (string, error) {
	serviceName := b.normalizeServiceName(name)

	args := []string{"journalctl", "-u", serviceName}
	if lines > 0 {
		args = append(args, "-n", strconv.Itoa(lines))
	}
	if follow {
		args = append(args, "-f")
	}

	cmd := exec.Command(args[0], args[1:]...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取服务日志失败: %w", err)
	}

	return string(output), nil
}

// GetServiceUnitFile 获取服务单元文件内容
func (b *SystemdBackend) GetServiceUnitFile(name string) (string, error) {
	serviceName := b.normalizeServiceName(name)

	cmd := exec.Command(b.systemctlPath, "cat", serviceName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取服务单元文件失败: %w", err)
	}

	return string(output), nil
}

// GetFailedServices 获取失败的服务列表
func (b *SystemdBackend) GetFailedServices() ([]*Service, error) {
	cmd := exec.Command(b.systemctlPath, "list-units", "--state=failed", "--type=service",
		"--no-pager", "--no-legend")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取失败服务列表失败: %w", err)
	}

	services := make([]*Service, 0)
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		unitName := fields[0]
		if !strings.HasSuffix(unitName, ".service") {
			continue
		}

		name := strings.TrimSuffix(unitName, ".service")
		description := ""
		if len(fields) > 4 {
			description = strings.Join(fields[4:], " ")
		}

		svc := &Service{
			Name:        name,
			Description: description,
			Type:        "systemd",
			UnitFile:    unitName,
			Status: ServiceStatus{
				Running:   false,
				LastError: "failed",
			},
		}

		services = append(services, svc)
	}

	return services, nil
}

// DaemonReload 重新加载 systemd 配置
func (b *SystemdBackend) DaemonReload() error {
	cmd := exec.Command(b.systemctlPath, "daemon-reload")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl daemon-reload 失败: %s: %w", string(output), err)
	}
	return nil
}

// ResetFailed 重置失败状态
func (b *SystemdBackend) ResetFailed(name string) error {
	serviceName := b.normalizeServiceName(name)

	cmd := exec.Command(b.systemctlPath, "reset-failed", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl reset-failed 失败: %s: %w", string(output), err)
	}
	return nil
}

// Mask 屏蔽服务
func (b *SystemdBackend) Mask(name string) error {
	serviceName := b.normalizeServiceName(name)

	cmd := exec.Command(b.systemctlPath, "mask", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl mask 失败: %s: %w", string(output), err)
	}
	return nil
}

// Unmask 取消屏蔽服务
func (b *SystemdBackend) Unmask(name string) error {
	serviceName := b.normalizeServiceName(name)

	cmd := exec.Command(b.systemctlPath, "unmask", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl unmask 失败: %s: %w", string(output), err)
	}
	return nil
}

// GetServiceDependencies 获取服务依赖
func (b *SystemdBackend) GetServiceDependencies(name string) ([]string, error) {
	serviceName := b.normalizeServiceName(name)

	cmd := exec.Command(b.systemctlPath, "list-dependencies", serviceName, "--no-pager", "--plain")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取服务依赖失败: %w", err)
	}

	deps := make([]string, 0)
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line == serviceName {
			continue
		}
		// 移除前导符号
		line = strings.TrimLeft(line, "│├└─ ")
		if line != "" {
			deps = append(deps, line)
		}
	}

	return deps, nil
}
