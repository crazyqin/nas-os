package container

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Network Docker 网络
type Network struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Scope      string            `json:"scope"`
	Subnet     string            `json:"subnet"`
	Gateway    string            `json:"gateway"`
	IPRange    string            `json:"ipRange"`
	Internal   bool              `json:"internal"`
	Attachable bool              `json:"attachable"`
	Labels     map[string]string `json:"labels"`
	Containers []string          `json:"containers"` // 容器 ID 列表
	Created    time.Time         `json:"created"`
}

// NetworkConfig 网络创建配置
type NetworkConfig struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`     // "bridge", "overlay", "macvlan", "ipvlan", "none"
	Subnet     string            `json:"subnet"`     // "172.20.0.0/16"
	Gateway    string            `json:"gateway"`    // "172.20.0.1"
	IPRange    string            `json:"ipRange"`    // "172.20.0.0/24"
	Internal   bool              `json:"internal"`   // 禁止访问外部网络
	Attachable bool              `json:"attachable"` // 允许独立容器连接
	Labels     map[string]string `json:"labels"`
	Options    map[string]string `json:"options"`
}

// NetworkManager 网络管理器
type NetworkManager struct {
	manager *Manager
}

// NewNetworkManager 创建网络管理器
func NewNetworkManager(mgr *Manager) *NetworkManager {
	return &NetworkManager{
		manager: mgr,
	}
}

// ListNetworks 列出所有网络
func (nm *NetworkManager) ListNetworks() ([]*Network, error) {
	cmd := exec.Command("docker", "network", "ls", "--format", "{{json .}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法列出网络：%w", err)
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

		// 获取详细信息
		network, err := nm.GetNetwork(raw.ID)
		if err != nil {
			network = &Network{
				ID:     raw.ID,
				Name:   raw.Name,
				Driver: raw.Driver,
				Scope:  raw.Scope,
			}
		}

		networks = append(networks, network)
	}

	return networks, nil
}

// GetNetwork 获取网络详情
func (nm *NetworkManager) GetNetwork(id string) (*Network, error) {
	cmd := exec.Command("docker", "network", "inspect", "--format", "{{json .}}", id)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法获取网络信息：%w", err)
	}

	var raw struct {
		ID         string `json:"Id"`
		Name       string `json:"Name"`
		Driver     string `json:"Driver"`
		Scope      string `json:"Scope"`
		Internal   bool   `json:"Internal"`
		Attachable bool   `json:"Attachable"`
		IPAM       struct {
			Config []struct {
				Subnet  string `json:"Subnet"`
				Gateway string `json:"Gateway"`
				IPRange string `json:"IPRange"`
			} `json:"Config"`
		} `json:"IPAM"`
		Containers map[string]struct {
			Name string `json:"Name"`
		} `json:"Containers"`
		Labels  map[string]string `json:"Labels"`
		Created time.Time         `json:"Created"`
		Options map[string]string `json:"Options"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, err
	}

	network := &Network{
		ID:         raw.ID,
		Name:       raw.Name,
		Driver:     raw.Driver,
		Scope:      raw.Scope,
		Internal:   raw.Internal,
		Attachable: raw.Attachable,
		Labels:     raw.Labels,
		Created:    raw.Created,
		Containers: make([]string, 0),
	}

	// 解析 IPAM 配置
	if len(raw.IPAM.Config) > 0 {
		network.Subnet = raw.IPAM.Config[0].Subnet
		network.Gateway = raw.IPAM.Config[0].Gateway
		network.IPRange = raw.IPAM.Config[0].IPRange
	}

	// 获取连接的容器
	for _, container := range raw.Containers {
		network.Containers = append(network.Containers, container.Name)
	}

	return network, nil
}

// CreateNetwork 创建网络
func (nm *NetworkManager) CreateNetwork(config *NetworkConfig) (*Network, error) {
	args := []string{"network", "create"}

	// 驱动
	if config.Driver != "" {
		args = append(args, "--driver", config.Driver)
	} else {
		config.Driver = "bridge"
	}

	// 子网
	if config.Subnet != "" {
		args = append(args, "--subnet", config.Subnet)
	}

	// 网关
	if config.Gateway != "" {
		args = append(args, "--gateway", config.Gateway)
	}

	// IP 范围
	if config.IPRange != "" {
		args = append(args, "--ip-range", config.IPRange)
	}

	// 内部网络
	if config.Internal {
		args = append(args, "--internal")
	}

	// 可连接
	if config.Attachable {
		args = append(args, "--attachable")
	}

	// 标签
	for k, v := range config.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}

	// 驱动选项
	for k, v := range config.Options {
		args = append(args, "--opt", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, config.Name)

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("创建网络失败：%w, %s", err, string(output))
	}

	networkID := strings.TrimSpace(string(output))
	return nm.GetNetwork(networkID)
}

// RemoveNetwork 删除网络
func (nm *NetworkManager) RemoveNetwork(id string) error {
	cmd := exec.Command("docker", "network", "rm", id)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("删除网络失败：%w, %s", err, string(output))
	}
	return nil
}

// ConnectNetwork 连接容器到网络
func (nm *NetworkManager) ConnectNetwork(networkID, containerID string, aliases []string) error {
	args := []string{"network", "connect"}

	// 网络别名
	for _, alias := range aliases {
		args = append(args, "--alias", alias)
	}

	args = append(args, networkID, containerID)

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("连接网络失败：%w, %s", err, string(output))
	}
	return nil
}

// DisconnectNetwork 断开容器与网络的连接
func (nm *NetworkManager) DisconnectNetwork(networkID, containerID string, force bool) error {
	args := []string{"network", "disconnect"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, networkID, containerID)

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("断开网络失败：%w, %s", err, string(output))
	}
	return nil
}

// PruneNetworks 清理未使用的网络
func (nm *NetworkManager) PruneNetworks() (uint64, error) {
	cmd := exec.Command("docker", "network", "prune", "-f")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("清理网络失败：%w", err)
	}

	// 解析删除的网络数量
	var deleted uint64
	outputStr := string(output)
	if strings.Contains(outputStr, "Total reclaimed space:") {
		parts := strings.Split(outputStr, "Total reclaimed space:")
		if len(parts) > 1 {
			sizeStr := strings.TrimSpace(parts[1])
			deleted = parseSize(sizeStr)
		}
	}

	return deleted, nil
}

// GetNetworkTypes 返回支持的网络类型
func (nm *NetworkManager) GetNetworkTypes() []map[string]string {
	return []map[string]string{
		{
			"name":        "bridge",
			"description": "桥接网络（默认）",
			"use_case":    "单机容器间通信",
		},
		{
			"name":        "host",
			"description": "主机网络",
			"use_case":    "容器直接使用主机网络栈",
		},
		{
			"name":        "overlay",
			"description": "覆盖网络",
			"use_case":    "多主机容器通信（Swarm）",
		},
		{
			"name":        "macvlan",
			"description": "MAC VLAN 网络",
			"use_case":    "容器直接连接物理网络",
		},
		{
			"name":        "none",
			"description": "无网络",
			"use_case":    "完全隔离的容器",
		},
	}
}
