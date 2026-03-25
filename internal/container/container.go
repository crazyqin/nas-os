package container

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// 常量定义.
const (
	// DefaultStopTimeout 默认停止超时时间（秒）.
	DefaultStopTimeout = 10
	// DefaultLogTail 默认日志行数.
	DefaultLogTail = 100
	// DefaultStatsTimeout 默认统计超时时间.
	DefaultStatsTimeout = 5 * time.Second
	// DefaultRestartPolicy 默认重启策略.
	DefaultRestartPolicy = "unless-stopped"
)

// Manager 容器管理器.
type Manager struct {
	socketPath string
}

// Container 容器信息.
type Container struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Command       string            `json:"command"`
	Created       time.Time         `json:"created"`
	Status        string            `json:"status"`
	State         string            `json:"state"`
	Running       bool              `json:"running"`
	Ports         []PortMapping     `json:"ports"`
	Labels        map[string]string `json:"labels"`
	Networks      []string          `json:"networks"`
	Volumes       []VolumeMount     `json:"volumes"`
	CPUUsage      float64           `json:"cpuUsage"`
	MemUsage      uint64            `json:"memUsage"`
	MemLimit      uint64            `json:"memLimit"`
	RestartPolicy string            `json:"restartPolicy"`
}

// PortMapping 端口映射.
type PortMapping struct {
	HostIP        string `json:"hostIp"`
	HostPort      string `json:"hostPort"`
	ContainerPort string `json:"containerPort"`
	Protocol      string `json:"protocol"`
}

// String 返回端口映射的字符串表示.
func (p *PortMapping) String() string {
	if p.HostIP != "" {
		return p.HostIP + ":" + p.HostPort + ":" + p.ContainerPort + "/" + p.Protocol
	}
	return p.HostPort + ":" + p.ContainerPort + "/" + p.Protocol
}

// VolumeMount 卷挂载.
type VolumeMount struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Mode        string `json:"mode"`
	RW          bool   `json:"rw"`
}

// String 返回卷挂载的字符串表示.
func (v *VolumeMount) String() string {
	if v.Mode != "" {
		return v.Source + ":" + v.Destination + ":" + v.Mode
	}
	return v.Source + ":" + v.Destination
}

// Stats 容器实时统计.
type Stats struct {
	CPUUsage   float64   `json:"cpuUsage"`
	MemUsage   uint64    `json:"memUsage"`
	MemLimit   uint64    `json:"memLimit"`
	MemPercent float64   `json:"memPercent"`
	NetRX      uint64    `json:"netRx"`
	NetTX      uint64    `json:"netTx"`
	BlockRead  uint64    `json:"blockRead"`
	BlockWrite uint64    `json:"blockWrite"`
	PIDs       uint64    `json:"pids"`
	Timestamp  time.Time `json:"timestamp"`
}

// MemoryPercent 计算内存使用百分比.
func (s *Stats) MemoryPercent() float64 {
	if s.MemLimit == 0 {
		return 0.0
	}
	return float64(s.MemUsage) / float64(s.MemLimit) * 100
}

// Config 容器创建配置.
type Config struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Command     []string          `json:"command,omitempty"`
	Ports       []string          `json:"ports,omitempty"`   // "8080:80"
	Volumes     []string          `json:"volumes,omitempty"` // "/host/path:/container/path"
	Environment map[string]string `json:"environment,omitempty"`
	Network     string            `json:"network,omitempty"`
	Restart     string            `json:"restart,omitempty"`  // "no", "always", "on-failure", "unless-stopped"
	CPULimit    string            `json:"cpuLimit,omitempty"` // "0.5" = 50%
	MemLimit    string            `json:"memLimit,omitempty"` // "512m"
	Labels      map[string]string `json:"labels,omitempty"`
	Detach      bool              `json:"detach,omitempty"`
	Interactive bool              `json:"interactive,omitempty"`
	TTY         bool              `json:"tty,omitempty"`
}

// Log 容器日志.
type Log struct {
	Timestamp time.Time `json:"timestamp"`
	Line      string    `json:"line"`
	Source    string    `json:"source"` // "stdout" or "stderr"
}

// NewManager 创建容器管理器.
func NewManager() (*Manager, error) {
	socketPath := os.Getenv("DOCKER_HOST")
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}

	return &Manager{
		socketPath: socketPath,
	}, nil
}

// IsRunning 检查 Docker 是否运行.
func (m *Manager) IsRunning() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "info")
	return cmd.Run() == nil
}

// GetVersion 获取 Docker 版本信息.
func (m *Manager) GetVersion() (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "version", "--format", "{{json .}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取 Docker 版本失败：%w", err)
	}

	var raw struct {
		Client struct {
			Version string `json:"Version"`
			API     string `json:"ApiVersion"`
			Go      string `json:"GoVersion"`
			OS      string `json:"Os"`
			Arch    string `json:"Arch"`
		} `json:"Client"`
		Server struct {
			Version string `json:"Version"`
			API     string `json:"ApiVersion"`
		} `json:"Server"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, err
	}

	return map[string]string{
		"clientVersion": raw.Client.Version,
		"clientAPI":     raw.Client.API,
		"serverVersion": raw.Server.Version,
		"serverAPI":     raw.Server.API,
		"goVersion":     raw.Client.Go,
		"os":            raw.Client.OS,
		"arch":          raw.Client.Arch,
	}, nil
}

// ListContainers 列出容器.
func (m *Manager) ListContainers(all bool) ([]*Container, error) {
	args := []string{"ps", "--format", "{{json .}}"}
	if all {
		args = []string{"ps", "-a", "--format", "{{json .}}"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法列出容器：%w", err)
	}

	var containers []*Container
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		var raw struct {
			ID      string `json:"ID"`
			Names   string `json:"Names"`
			Image   string `json:"Image"`
			Command string `json:"Command"`
			Status  string `json:"Status"`
			State   string `json:"State"`
			Ports   string `json:"Ports"`
		}

		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		// 获取详细信息
		container, err := m.GetContainer(raw.ID)
		if err != nil {
			continue
		}

		containers = append(containers, container)
	}

	return containers, nil
}

// GetContainer 获取容器详情.
func (m *Manager) GetContainer(id string) (*Container, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{json .}}", id)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法获取容器信息：%w", err)
	}

	var raw []struct {
		ID    string   `json:"Id"`
		Name  string   `json:"Name"`
		Image string   `json:"Image"`
		Path  string   `json:"Path"`
		Args  []string `json:"Args"`
		State struct {
			Status    string    `json:"Status"`
			Running   bool      `json:"Running"`
			StartedAt time.Time `json:"StartedAt"`
		} `json:"State"`
		NetworkSettings struct {
			Networks map[string]struct{} `json:"Networks"`
			Ports    map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"Ports"`
		} `json:"NetworkSettings"`
		Mounts []struct {
			Source      string `json:"Source"`
			Destination string `json:"Destination"`
			Mode        string `json:"Mode"`
			RW          bool   `json:"RW"`
		} `json:"Mounts"`
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		HostConfig struct {
			RestartPolicy struct {
				Name string `json:"Name"`
			} `json:"RestartPolicy"`
		} `json:"HostConfig"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, err
	}

	if len(raw) == 0 {
		return nil, fmt.Errorf("容器不存在")
	}

	c := raw[0]
	container := &Container{
		ID:            c.ID[:12],
		Name:          strings.TrimPrefix(c.Name, "/"),
		Image:         c.Image,
		Command:       c.Path,
		State:         c.State.Status,
		Running:       c.State.Running,
		Status:        c.State.Status,
		Created:       c.State.StartedAt,
		Labels:        c.Config.Labels,
		Volumes:       make([]VolumeMount, 0),
		Ports:         make([]PortMapping, 0),
		RestartPolicy: c.HostConfig.RestartPolicy.Name,
	}

	// 解析命令参数
	if len(c.Args) > 0 {
		container.Command += " " + strings.Join(c.Args, " ")
	}

	// 解析网络
	for network := range c.NetworkSettings.Networks {
		container.Networks = append(container.Networks, network)
	}

	// 解析端口映射
	for containerPort, bindings := range c.NetworkSettings.Ports {
		for _, binding := range bindings {
			parts := strings.Split(containerPort, "/")
			protocol := "tcp"
			if len(parts) > 1 {
				protocol = parts[1]
			}
			container.Ports = append(container.Ports, PortMapping{
				HostIP:        binding.HostIP,
				HostPort:      binding.HostPort,
				ContainerPort: parts[0],
				Protocol:      protocol,
			})
		}
	}

	// 解析卷挂载
	for _, mount := range c.Mounts {
		container.Volumes = append(container.Volumes, VolumeMount{
			Source:      mount.Source,
			Destination: mount.Destination,
			Mode:        mount.Mode,
			RW:          mount.RW,
		})
	}

	return container, nil
}

// CreateContainer 创建容器.
func (m *Manager) CreateContainer(config *Config) (*Container, error) {
	args := []string{"run", "-d"}

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

	// 网络模式
	if config.Network != "" {
		args = append(args, "--network", config.Network)
	}

	// 重启策略
	if config.Restart != "" {
		args = append(args, "--restart", config.Restart)
	} else {
		args = append(args, "--restart", DefaultRestartPolicy)
	}

	// CPU 限制
	if config.CPULimit != "" {
		args = append(args, "--cpus", config.CPULimit)
	}

	// 内存限制
	if config.MemLimit != "" {
		args = append(args, "-m", config.MemLimit)
	}

	// 标签
	for k, v := range config.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}

	// 交互式/TTY
	if config.Interactive {
		args = append(args, "-i")
	}
	if config.TTY {
		args = append(args, "-t")
	}

	// 镜像和命令
	args = append(args, config.Image)
	if len(config.Command) > 0 {
		args = append(args, config.Command...)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("创建容器失败：%w, %s", err, string(output))
	}

	containerID := strings.TrimSpace(string(output))
	return m.GetContainer(containerID)
}

// StartContainer 启动容器.
func (m *Manager) StartContainer(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "start", id)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("启动容器失败：%w, %s", err, string(output))
	}
	return nil
}

// StopContainer 停止容器.
func (m *Manager) StopContainer(id string, timeout int) error {
	args := []string{"stop"}
	if timeout > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", timeout))
	}
	args = append(args, id)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout+10)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("停止容器失败：%w, %s", err, string(output))
	}
	return nil
}

// RestartContainer 重启容器.
func (m *Manager) RestartContainer(id string, timeout int) error {
	args := []string{"restart"}
	if timeout > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", timeout))
	}
	args = append(args, id)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout+10)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("重启容器失败：%w, %s", err, string(output))
	}
	return nil
}

// RemoveContainer 删除容器.
func (m *Manager) RemoveContainer(id string, force bool, removeVolumes bool) error {
	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	if removeVolumes {
		args = append(args, "-v")
	}
	args = append(args, id)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("删除容器失败：%w, %s", err, string(output))
	}
	return nil
}

// GetContainerStats 获取容器实时统计.
func (m *Manager) GetContainerStats(id string) (*Stats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultStatsTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "stats", "--no-stream", "--format", "{{json .}}", id)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取容器统计失败：%w", err)
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

	stats := &Stats{
		Timestamp: time.Now(),
	}

	// 解析 CPU 百分比
	cpuStr := strings.TrimSuffix(raw.CPUPerc, "%")
	if _, err := fmt.Sscanf(cpuStr, "%f", &stats.CPUUsage); err != nil {
		stats.CPUUsage = 0
	}

	// 解析内存使用
	memParts := strings.Split(raw.MemUsage, " / ")
	if len(memParts) >= 2 {
		stats.MemUsage = parseSize(memParts[0])
		stats.MemLimit = parseSize(memParts[1])
	}

	// 解析内存百分比
	memPercStr := strings.TrimSuffix(raw.MemPerc, "%")
	if _, err := fmt.Sscanf(memPercStr, "%f", &stats.MemPercent); err != nil {
		stats.MemPercent = 0
	}

	// 解析网络 I/O
	netParts := strings.Split(raw.NetIO, " / ")
	if len(netParts) >= 2 {
		stats.NetRX = parseSize(netParts[0])
		stats.NetTX = parseSize(netParts[1])
	}

	// 解析磁盘 I/O
	blockParts := strings.Split(raw.BlockIO, " / ")
	if len(blockParts) >= 2 {
		stats.BlockRead = parseSize(blockParts[0])
		stats.BlockWrite = parseSize(blockParts[1])
	}

	// 解析进程数
	if _, err := fmt.Sscanf(raw.PIDs, "%d", &stats.PIDs); err != nil {
		stats.PIDs = 0
	}

	return stats, nil
}

// GetContainerLogs 获取容器日志.
func (m *Manager) GetContainerLogs(id string, tail int, follow bool) ([]Log, error) {
	args := []string{"logs"}
	if tail <= 0 {
		tail = DefaultLogTail
	}
	args = append(args, "--tail", fmt.Sprintf("%d", tail))
	if follow {
		args = append(args, "-f")
	}
	args = append(args, id)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil && len(output) == 0 {
		return nil, fmt.Errorf("获取日志失败：%w", err)
	}

	var logs []Log
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		logs = append(logs, Log{
			Timestamp: time.Now(),
			Line:      scanner.Text(),
			Source:    "stdout",
		})
	}

	return logs, nil
}

// parseSize 解析大小字符串.
func parseSize(s string) uint64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")

	var size float64
	var unit string

	n, err := fmt.Sscanf(s, "%f%s", &size, &unit)
	if err != nil && n == 0 {
		return 0
	}

	// 纯数字（没有单位）
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

// Validate 验证容器配置.
func (c *Config) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if c.Image == "" {
		return fmt.Errorf("image is required")
	}
	return nil
}
