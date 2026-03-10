package docker

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

// Manager Docker 管理器
type Manager struct {
	socketPath string
}

// Container 容器信息
type Container struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Status    string            `json:"status"`
	State     string            `json:"state"`
	Created   time.Time         `json:"created"`
	Ports     []PortMapping     `json:"ports"`
	Labels    map[string]string `json:"labels"`
	CPUUsage  float64           `json:"cpuUsage"`
	MemUsage  uint64            `json:"memUsage"`
	MemLimit  uint64            `json:"memLimit"`
	Networks  []string          `json:"networks"`
	Volumes   []VolumeMount     `json:"volumes"`
}

// PortMapping 端口映射
type PortMapping struct {
	HostIP        string `json:"hostIp"`
	HostPort      string `json:"hostPort"`
	ContainerPort string `json:"containerPort"`
	Protocol      string `json:"protocol"`
}

// VolumeMount 卷挂载
type VolumeMount struct {
	Source   string `json:"source"`
	Destination string `json:"destination"`
	Mode     string `json:"mode"`
	RW       bool   `json:"rw"`
}

// Image 镜像信息
type Image struct {
	ID         string   `json:"id"`
	Repository string   `json:"repository"`
	Tag        string   `json:"tag"`
	Size       uint64   `json:"size"`
	Created    time.Time `json:"created"`
}

// Network 网络信息
type Network struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Driver    string   `json:"driver"`
	Scope     string   `json:"scope"`
	Subnet    string   `json:"subnet"`
	Gateway   string   `json:"gateway"`
	Containers []string `json:"containers"`
}

// Volume 卷信息
type Volume struct {
	Name      string    `json:"name"`
	Driver    string    `json:"driver"`
	MountPoint string   `json:"mountPoint"`
	Size      uint64    `json:"size"`
	Created   time.Time `json:"created"`
}

// ContainerStats 容器统计信息
type ContainerStats struct {
	CPUUsage float64 `json:"cpuUsage"`
	MemUsage uint64  `json:"memUsage"`
	MemLimit uint64  `json:"memLimit"`
	NetRX    uint64  `json:"netRx"`
	NetTX    uint64  `json:"netTx"`
	BlockRead  uint64 `json:"blockRead"`
	BlockWrite uint64 `json:"blockWrite"`
}

// AppCatalog 应用目录
type AppCatalog struct {
	Name        string   `json:"name"`
	Image       string   `json:"image"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Ports       []int    `json:"ports"`
	Volumes     []string `json:"volumes"`
	Environment map[string]string `json:"environment"`
}

// NewManager 创建 Docker 管理器
func NewManager() (*Manager, error) {
	socketPath := os.Getenv("DOCKER_HOST")
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}

	return &Manager{
		socketPath: socketPath,
	}, nil
}

// IsRunning 检查 Docker 是否运行
func (m *Manager) IsRunning() bool {
	cmd := exec.Command("docker", "info")
	return cmd.Run() == nil
}

// ListContainers 列出容器
func (m *Manager) ListContainers(all bool) ([]*Container, error) {
	args := []string{"ps", "--format", "{{json .}}"}
	if all {
		args = []string{"ps", "-a", "--format", "{{json .}}"}
	}

	cmd := exec.Command("docker", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法列出容器: %w", err)
	}

	var containers []*Container
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		var raw struct {
			ID      string `json:"ID"`
			Names   string `json:"Names"`
			Image   string `json:"Image"`
			Status  string `json:"Status"`
			State   string `json:"State"`
			Ports   string `json:"Ports"`
		}

		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		container := &Container{
			ID:      raw.ID[:12],
			Name:    strings.TrimPrefix(raw.Names, "/"),
			Image:   raw.Image,
			Status:  raw.Status,
			State:   raw.State,
			Ports:   m.parsePorts(raw.Ports),
			Labels:  make(map[string]string),
		}

		containers = append(containers, container)
	}

	return containers, nil
}

// GetContainer 获取容器详情
func (m *Manager) GetContainer(id string) (*Container, error) {
	cmd := exec.Command("docker", "inspect", "--format", "{{json .}}", id)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法获取容器信息: %w", err)
	}

	var raw []struct {
		ID      string `json:"Id"`
		Name    string `json:"Name"`
		Image   string `json:"Image"`
		State   struct {
			Status  string    `json:"Status"`
			Created time.Time `json:"StartedAt"`
		} `json:"State"`
		NetworkSettings struct {
			Ports map[string][]struct {
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
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, err
	}

	if len(raw) == 0 {
		return nil, fmt.Errorf("容器不存在")
	}

	c := raw[0]
	container := &Container{
		ID:      c.ID[:12],
		Name:    strings.TrimPrefix(c.Name, "/"),
		Image:   c.Image,
		State:   c.State.Status,
		Status:  c.State.Status,
		Created: c.State.Created,
		Labels:  c.Config.Labels,
		Volumes: make([]VolumeMount, 0),
		Ports:   make([]PortMapping, 0),
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

// CreateContainer 创建容器
func (m *Manager) CreateContainer(name, image string, opts map[string]interface{}) (*Container, error) {
	args := []string{"run", "-d", "--name", name}

	// 端口映射
	if ports, ok := opts["ports"].([]string); ok {
		for _, port := range ports {
			args = append(args, "-p", port)
		}
	}

	// 卷挂载
	if volumes, ok := opts["volumes"].([]string); ok {
		for _, vol := range volumes {
			args = append(args, "-v", vol)
		}
	}

	// 环境变量
	if env, ok := opts["env"].(map[string]string); ok {
		for k, v := range env {
			args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
		}
	}

	// 网络模式
	if network, ok := opts["network"].(string); ok {
		args = append(args, "--network", network)
	}

	// 重启策略
	if restart, ok := opts["restart"].(string); ok {
		args = append(args, "--restart", restart)
	} else {
		args = append(args, "--restart", "unless-stopped")
	}

	args = append(args, image)

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("创建容器失败: %w, %s", err, string(output))
	}

	return m.GetContainer(name)
}

// StartContainer 启动容器
func (m *Manager) StartContainer(id string) error {
	cmd := exec.Command("docker", "start", id)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("启动容器失败: %w, %s", err, string(output))
	}
	return nil
}

// StopContainer 停止容器
func (m *Manager) StopContainer(id string, timeout int) error {
	args := []string{"stop"}
	if timeout > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", timeout))
	}
	args = append(args, id)

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("停止容器失败: %w, %s", err, string(output))
	}
	return nil
}

// RestartContainer 重启容器
func (m *Manager) RestartContainer(id string, timeout int) error {
	args := []string{"restart"}
	if timeout > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", timeout))
	}
	args = append(args, id)

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("重启容器失败: %w, %s", err, string(output))
	}
	return nil
}

// RemoveContainer 删除容器
func (m *Manager) RemoveContainer(id string, force bool) error {
	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, id)

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("删除容器失败: %w, %s", err, string(output))
	}
	return nil
}

// GetContainerStats 获取容器统计
func (m *Manager) GetContainerStats(id string) (*ContainerStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "stats", "--no-stream", "--format", "{{json .}}", id)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取容器统计失败: %w", err)
	}

	var raw struct {
		CPUPerc string `json:"CPUPerc"`
		MemUsage string `json:"MemUsage"`
		NetIO   string `json:"NetIO"`
		BlockIO string `json:"BlockIO"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, err
	}

	stats := &ContainerStats{}

	// 解析 CPU 百分比
	cpuStr := strings.TrimSuffix(raw.CPUPerc, "%")
	fmt.Sscanf(cpuStr, "%f", &stats.CPUUsage)

	// 解析内存使用
	memParts := strings.Split(raw.MemUsage, " / ")
	if len(memParts) >= 2 {
		stats.MemUsage = parseSize(memParts[0])
		stats.MemLimit = parseSize(memParts[1])
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

	return stats, nil
}

// ListImages 列出镜像
func (m *Manager) ListImages() ([]*Image, error) {
	cmd := exec.Command("docker", "images", "--format", "{{json .}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法列出镜像: %w", err)
	}

	var images []*Image
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		var raw struct {
			ID         string `json:"ID"`
			Repository string `json:"Repository"`
			Tag        string `json:"Tag"`
			Size       string `json:"Size"`
		}

		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		images = append(images, &Image{
			ID:         raw.ID,
			Repository: raw.Repository,
			Tag:        raw.Tag,
			Size:       parseSize(raw.Size),
		})
	}

	return images, nil
}

// PullImage 拉取镜像
func (m *Manager) PullImage(image string) error {
	cmd := exec.Command("docker", "pull", image)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("拉取镜像失败: %w, %s", err, string(output))
	}
	return nil
}

// RemoveImage 删除镜像
func (m *Manager) RemoveImage(id string, force bool) error {
	args := []string{"rmi"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, id)

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("删除镜像失败: %w, %s", err, string(output))
	}
	return nil
}

// ListNetworks 列出网络
func (m *Manager) ListNetworks() ([]*Network, error) {
	cmd := exec.Command("docker", "network", "ls", "--format", "{{json .}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法列出网络: %w", err)
	}

	var networks []*Network
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		var raw struct {
			ID     string `json:"ID"`
			Name   string `json:"Name"`
			Driver string `json:"Driver"`
			Scope  string `json:"Scope"`
		}

		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		networks = append(networks, &Network{
			ID:     raw.ID,
			Name:   raw.Name,
			Driver: raw.Driver,
			Scope:  raw.Scope,
		})
	}

	return networks, nil
}

// GetAppCatalog 获取应用目录
func (m *Manager) GetAppCatalog() []*AppCatalog {
	return []*AppCatalog{
		{
			Name:        "Plex",
			Image:       "plexinc/pms-docker:latest",
			Description: "媒体服务器",
			Category:    "Media",
			Ports:       []int{32400},
			Volumes:     []string{"/config", "/media"},
			Environment: map[string]string{"PLEX_CLAIM": "claim-xxx"},
		},
		{
			Name:        "Jellyfin",
			Image:       "jellyfin/jellyfin:latest",
			Description: "开源媒体服务器",
			Category:    "Media",
			Ports:       []int{8096},
			Volumes:     []string{"/config", "/media"},
			Environment: map[string]string{},
		},
		{
			Name:        "Home Assistant",
			Image:       "homeassistant/home-assistant:stable",
			Description: "智能家居平台",
			Category:    "Home Automation",
			Ports:       []int{8123},
			Volumes:     []string{"/config"},
			Environment: map[string]string{},
		},
		{
			Name:        "Nextcloud",
			Image:       "nextcloud:latest",
			Description: "私有云存储",
			Category:    "Productivity",
			Ports:       []int{80},
			Volumes:     []string{"/var/www/html"},
			Environment: map[string]string{},
		},
		{
			Name:        "Pi-hole",
			Image:       "pihole/pihole:latest",
			Description: "网络广告拦截",
			Category:    "Network",
			Ports:       []int{53, 80},
			Volumes:     []string{"/etc/pihole"},
			Environment: map[string]string{"TZ": "Asia/Shanghai"},
		},
		{
			Name:        "Transmission",
			Image:       "linuxserver/transmission:latest",
			Description: "BitTorrent 客户端",
			Category:    "Download",
			Ports:       []int{9091},
			Volumes:     []string{"/config", "/downloads"},
			Environment: map[string]string{"PUID": "1000", "PGID": "1000"},
		},
	}
}

// parsePorts 解析端口映射
func (m *Manager) parsePorts(ports string) []PortMapping {
	var result []PortMapping
	if ports == "" {
		return result
	}

	// 示例: 0.0.0.0:8080->80/tcp
	parts := strings.Split(ports, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		mapping := PortMapping{Protocol: "tcp"}
		if strings.Contains(part, "->") {
			sides := strings.Split(part, "->")
			if len(sides) == 2 {
				// 解析主机端
				hostParts := strings.Split(sides[0], ":")
				if len(hostParts) >= 2 {
					mapping.HostIP = hostParts[0]
					mapping.HostPort = hostParts[len(hostParts)-1]
				}

				// 解析容器端
				containerParts := strings.Split(sides[1], "/")
				mapping.ContainerPort = containerParts[0]
				if len(containerParts) > 1 {
					mapping.Protocol = containerParts[1]
				}
			}
		}

		result = append(result, mapping)
	}

	return result
}

// parseSize 解析大小字符串
func parseSize(s string) uint64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")

	var size uint64
	var unit string

	fmt.Sscanf(s, "%d%s", &size, &unit)

	switch strings.ToUpper(unit) {
	case "KB", "KIB":
		return size * 1024
	case "MB", "MIB":
		return size * 1024 * 1024
	case "GB", "GIB":
		return size * 1024 * 1024 * 1024
	case "TB", "TIB":
		return size * 1024 * 1024 * 1024 * 1024
	default:
		return size
	}
}