// Package apps 应用生命周期管理
// 提供应用的启动、停止、重启、状态查询等核心操作
package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"nas-os/pkg/app"
)

// Manager 应用生命周期管理器（Docker Compose 实现）
type Manager struct {
	// 可注入其他依赖（如日志、监控等）
}

// NewManager 创建应用生命周期管理器
func NewManager() *Manager {
	return &Manager{}
}

// ========== 容器操作实现 ==========

// CreateContainer 创建容器
func (m *Manager) CreateContainer(ctx context.Context, config *app.ContainerConfig) (string, error) {
	args := []string{"create"}

	// 容器名称
	if config.Name != "" {
		args = append(args, "--name", config.Name)
	}

	// 端口映射
	for _, port := range config.Ports {
		args = append(args, "-p", port)
	}

	// 卷挂载
	for _, vol := range config.Volumes {
		args = append(args, "-v", vol)
	}

	// 环境变量
	for k, v := range config.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// 网络
	if config.Network != "" {
		args = append(args, "--network", config.Network)
	}

	// 重启策略
	if config.Restart != "" {
		args = append(args, "--restart", config.Restart)
	}

	// CPU限制
	if config.CPULimit != "" {
		args = append(args, "--cpus", config.CPULimit)
	}

	// 内存限制
	if config.MemLimit != "" {
		args = append(args, "-m", config.MemLimit)
	}

	// 特权模式
	if config.Privileged {
		args = append(args, "--privileged")
	}

	// 标签
	for k, v := range config.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}

	// 镜像
	args = append(args, config.Image)

	// 命令
	if len(config.Command) > 0 {
		args = append(args, config.Command...)
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("创建容器失败: %w, %s", err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}

// StartContainer 启动容器
func (m *Manager) StartContainer(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, "docker", "start", id)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("启动容器失败: %w, %s", err, string(output))
	}
	return nil
}

// StopContainer 停止容器
func (m *Manager) StopContainer(ctx context.Context, id string, timeout int) error {
	args := []string{"stop"}
	if timeout > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", timeout))
	}
	args = append(args, id)

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("停止容器失败: %w, %s", err, string(output))
	}
	return nil
}

// RemoveContainer 移除容器
func (m *Manager) RemoveContainer(ctx context.Context, id string, force bool) error {
	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, id)

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("移除容器失败: %w, %s", err, string(output))
	}
	return nil
}

// GetContainerStatus 获取容器状态
func (m *Manager) GetContainerStatus(ctx context.Context, id string) (*app.ContainerStatus, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{json .}}", id)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取容器状态失败: %w", err)
	}

	var raw []struct {
		ID    string `json:"Id"`
		Name  string `json:"Name"`
		Image string `json:"Image"`
		State struct {
			Status    string    `json:"Status"`
			Running   bool      `json:"Running"`
			StartedAt time.Time `json:"StartedAt"`
		} `json:"State"`
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		NetworkSettings struct {
			Ports map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"Ports"`
		} `json:"NetworkSettings"`
		Mounts []struct {
			Source      string `json:"Source"`
			Destination string `json:"Destination"`
		} `json:"Mounts"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("解析容器状态失败: %w", err)
	}

	if len(raw) == 0 {
		return nil, fmt.Errorf("容器不存在")
	}

	c := raw[0]
	status := &app.ContainerStatus{
		ID:      c.ID[:12],
		Name:    strings.TrimPrefix(c.Name, "/"),
		Image:   c.Image,
		State:   c.State.Status,
		Status:  c.State.Status,
		Running: c.State.Running,
		Created: c.State.StartedAt,
		Labels:  c.Config.Labels,
		Ports:   []string{},
		Volumes: []string{},
	}

	// 解析端口
	for containerPort, bindings := range c.NetworkSettings.Ports {
		for _, binding := range bindings {
			status.Ports = append(status.Ports, fmt.Sprintf("%s:%s", binding.HostPort, strings.Split(containerPort, "/")[0]))
		}
	}

	// 解析卷
	for _, mount := range c.Mounts {
		status.Volumes = append(status.Volumes, fmt.Sprintf("%s:%s", mount.Source, mount.Destination))
	}

	return status, nil
}

// ========== Compose 操作实现 ==========

// ComposeUp 启动Compose项目
func (m *Manager) ComposeUp(ctx context.Context, composePath string) error {
	// 使用 compose v2 语法
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composePath, "up", "-d", "--remove-orphans")
	cmd.Dir = filepath.Dir(composePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Compose启动失败: %w, %s", err, string(output))
	}
	return nil
}

// ComposeDown 停止Compose项目
func (m *Manager) ComposeDown(ctx context.Context, composePath string) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composePath, "down")
	cmd.Dir = filepath.Dir(composePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Compose停止失败: %w, %s", err, string(output))
	}
	return nil
}

// ComposePS 获取Compose服务状态
func (m *Manager) ComposePS(ctx context.Context, composePath string) ([]app.ComposeService, error) {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composePath, "ps", "--format", "json")
	cmd.Dir = filepath.Dir(composePath)
	output, err := cmd.Output()
	if err != nil {
		// 可能是项目未启动
		return []app.ComposeService{}, nil
	}

	var services []app.ComposeService
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		var raw struct {
			Name   string `json:"Name"`
			State  string `json:"State"`
			Status string `json:"Status"`
			Image  string `json:"Image"`
			Ports  string `json:"Publishers"` // v2 格式
		}

		// 尝试两种格式解析
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		svc := app.ComposeService{
			Name:   raw.Name,
			State:  raw.State,
			Status: raw.Status,
			Image:  raw.Image,
			Ports:  raw.Ports,
			Running: raw.State == "running",
		}
		services = append(services, svc)
	}

	return services, nil
}

// ComposeLogs 获取Compose日志
func (m *Manager) ComposeLogs(ctx context.Context, composePath string, tail int) (map[string][]string, error) {
	args := []string{"compose", "-f", composePath, "logs"}
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", tail))
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = filepath.Dir(composePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("获取Compose日志失败: %w", err)
	}

	// 解析日志（按服务分组）
	logs := make(map[string][]string)
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		// 解析服务名（格式: service_name|log_content）
		parts := strings.SplitN(line, "|", 2)
		if len(parts) >= 2 {
			serviceName := strings.TrimSpace(parts[0])
			logContent := strings.TrimSpace(parts[1])
			logs[serviceName] = append(logs[serviceName], logContent)
		}
	}

	return logs, nil
}

// ComposeRestart 重启Compose项目
func (m *Manager) ComposeRestart(ctx context.Context, composePath string, timeout int) error {
	args := []string{"compose", "-f", composePath, "restart"}
	if timeout > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", timeout))
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = filepath.Dir(composePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Compose重启失败: %w, %s", err, string(output))
	}
	return nil
}

// ComposePull 拉取Compose镜像
func (m *Manager) ComposePull(ctx context.Context, composePath string) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composePath, "pull")
	cmd.Dir = filepath.Dir(composePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Compose镜像拉取失败: %w, %s", err, string(output))
	}
	return nil
}

// ========== 应用状态监控 ==========

// GetAppStats 获取应用资源使用统计
func (m *Manager) GetAppStats(ctx context.Context, composePath string) (map[string]*ServiceStats, error) {
	// 获取服务列表
	services, err := m.ComposePS(ctx, composePath)
	if err != nil {
		return nil, err
	}

	stats := make(map[string]*ServiceStats)

	for _, svc := range services {
		if svc.State != "running" {
			stats[svc.Name] = &ServiceStats{
				CPUUsage:    0,
				MemUsage:    0,
				MemLimit:    0,
				NetRx:       0,
				NetTx:       0,
				BlockRead:   0,
				BlockWrite:  0,
			}
			continue
		}

		// 获取容器统计
		containerStats, err := m.getContainerStats(ctx, svc.Name)
		if err != nil {
			fmt.Printf("获取容器 %s 统计失败: %v\n", svc.Name, err)
			continue
		}

		stats[svc.Name] = containerStats
	}

	return stats, nil
}

// ServiceStats 服务资源统计
type ServiceStats struct {
	CPUUsage    float64 `json:"cpuUsage"`
	MemUsage    uint64  `json:"memUsage"`
	MemLimit    uint64  `json:"memLimit"`
	MemPercent  float64 `json:"memPercent"`
	NetRx       uint64  `json:"netRx"`
	NetTx       uint64  `json:"netTx"`
	BlockRead   uint64  `json:"blockRead"`
	BlockWrite  uint64  `json:"blockWrite"`
	PIDs        uint64  `json:"pids"`
}

// getContainerStats 获取单个容器统计
func (m *Manager) getContainerStats(ctx context.Context, containerName string) (*ServiceStats, error) {
	cmd := exec.CommandContext(ctx, "docker", "stats", "--no-stream", "--format", "{{json .}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取容器统计失败: %w", err)
	}

	var raw struct {
		CPUPerc  string `json:"CPUPerc"`
		MemUsage string `json:"MemUsage"`
		MemPerc  string `json:"MemPerc"`
		NetIO    string `json:"NetIO"`
		BlockIO  string `json:"BlockIO"`
		PIDs     string `json:"PIDs"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, err
	}

	stats := &ServiceStats{}

	// 解析CPU百分比
	cpuStr := strings.TrimSuffix(raw.CPUPerc, "%")
	fmt.Sscanf(cpuStr, "%f", &stats.CPUUsage)

	// 解析内存
	memParts := strings.Split(raw.MemUsage, " / ")
	if len(memParts) >= 2 {
		stats.MemUsage = parseSize(memParts[0])
		stats.MemLimit = parseSize(memParts[1])
	}

	// 解析内存百分比
	memPercStr := strings.TrimSuffix(raw.MemPerc, "%")
	fmt.Sscanf(memPercStr, "%f", &stats.MemPercent)

	// 解析网络I/O
	netParts := strings.Split(raw.NetIO, " / ")
	if len(netParts) >= 2 {
		stats.NetRx = parseSize(netParts[0])
		stats.NetTx = parseSize(netParts[1])
	}

	// 解析磁盘I/O
	blockParts := strings.Split(raw.BlockIO, " / ")
	if len(blockParts) >= 2 {
		stats.BlockRead = parseSize(blockParts[0])
		stats.BlockWrite = parseSize(blockParts[1])
	}

	// 解析进程数
	fmt.Sscanf(raw.PIDs, "%d", &stats.PIDs)

	return stats, nil
}

// parseSize 解析大小字符串
func parseSize(s string) uint64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")

	var size float64
	var unit string

	n, _ := fmt.Sscanf(s, "%f%s", &size, &unit)

	// 纯数字
	if n == 1 {
		return uint64(size)
	}

	switch strings.ToUpper(unit) {
	case "KB", "KIB", "K":
		return uint64(size * 1024)
	case "MB", "MIB", "M":
		return uint64(size * 1024 * 1024)
	case "GB", "GIB", "G":
		return uint64(size * 1024 * 1024 * 1024)
	case "TB", "TIB", "T":
		return uint64(size * 1024 * 1024 * 1024 * 1024)
	default:
		return uint64(size)
	}
}

// ========== 健康检查 ==========

// CheckAppHealth 检查应用健康状态
func (m *Manager) CheckAppHealth(ctx context.Context, composePath string) (*HealthReport, error) {
	services, err := m.ComposePS(ctx, composePath)
	if err != nil {
		return nil, err
	}

	report := &HealthReport{
		Timestamp: time.Now(),
		Services:  make(map[string]ServiceHealth),
	}

	allHealthy := true
	for _, svc := range services {
		healthy := svc.State == "running" && svc.Health != "unhealthy"
		report.Services[svc.Name] = ServiceHealth{
			Healthy:   healthy,
			State:     svc.State,
			Status:    svc.Status,
			Health:    svc.Health,
		}
		if !healthy {
			allHealthy = false
		}
	}

	if len(services) == 0 {
		report.Overall = "stopped"
		report.Message = "应用已停止"
	} else if allHealthy {
		report.Overall = "healthy"
		report.Message = "所有服务健康"
	} else {
		report.Overall = "unhealthy"
		report.Message = "部分服务异常"
	}

	return report, nil
}

// HealthReport 健康检查报告
type HealthReport struct {
	Timestamp time.Time              `json:"timestamp"`
	Overall   string                 `json:"overall"`  // healthy/unhealthy/stopped
	Message   string                 `json:"message"`
	Services  map[string]ServiceHealth `json:"services"`
}

// ServiceHealth 服务健康状态
type ServiceHealth struct {
	Healthy bool   `json:"healthy"`
	State   string `json:"state"`
	Status  string `json:"status"`
	Health  string `json:"health"`
}