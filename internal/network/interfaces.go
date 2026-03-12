package network

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// ListInterfaces 获取所有网络接口
func (m *Manager) ListInterfaces() ([]*Interface, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 使用 ip 命令获取接口信息
	cmd := exec.Command("ip", "-json", "addr", "show")
	output, err := cmd.Output()
	if err != nil {
		// 回退到 /sys/class/net 方式
		return m.listInterfacesFromSys()
	}

	// 解析 JSON 输出
	return m.parseIPJson(output)
}

// listInterfacesFromSys 从 /sys/class/net 获取接口信息
func (m *Manager) listInterfacesFromSys() ([]*Interface, error) {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return nil, fmt.Errorf("无法读取网络接口: %w", err)
	}

	var interfaces []*Interface

	for _, entry := range entries {
		name := entry.Name()
		iface, err := m.getInterfaceInfo(name)
		if err != nil {
			continue
		}
		interfaces = append(interfaces, iface)
	}

	return interfaces, nil
}

// getInterfaceInfo 获取单个接口的详细信息
func (m *Manager) getInterfaceInfo(name string) (*Interface, error) {
	iface := &Interface{
		Name:  name,
		State: "down",
		Type:  "ethernet",
	}

	// 读取 MAC 地址
	if mac, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/address", name)); err == nil {
		iface.MAC = strings.TrimSpace(string(mac))
	}

	// 读取状态
	if state, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/operstate", name)); err == nil {
		iface.State = strings.TrimSpace(string(state))
	}

	// 读取 MTU
	if mtu, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/mtu", name)); err == nil {
		_, _ = fmt.Sscanf(strings.TrimSpace(string(mtu)), "%d", &iface.Mtu)
	}

	// 读取速度（仅对物理接口有效）
	if speed, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/speed", name)); err == nil {
		speedStr := strings.TrimSpace(string(speed))
		if speedStr != "" && speedStr != "-1" {
			iface.Speed = speedStr + " Mbps"
		}
	}

	// 判断接口类型
	if strings.HasPrefix(name, "wlan") || strings.HasPrefix(name, "wlp") {
		iface.Type = "wifi"
	} else if strings.HasPrefix(name, "br") {
		iface.Type = "bridge"
	} else if strings.HasPrefix(name, "docker") || strings.HasPrefix(name, "veth") {
		iface.Type = "virtual"
	} else if name == "lo" {
		iface.Type = "loopback"
		iface.State = "up" // lo 总是 up
	}

	// 获取 IP 地址信息
	if iface.State == "up" {
		iface.IP, iface.Netmask = m.getInterfaceIP(name)
		iface.Gateway = m.getDefaultGateway(name)
		iface.DNS = m.getDNS(name)

		// 获取流量统计
		iface.RxBytes, iface.TxBytes = m.getInterfaceStats(name)
	}

	return iface, nil
}

// getInterfaceIP 获取接口的 IP 地址
func (m *Manager) getInterfaceIP(name string) (string, string) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return "", ""
	}

	addrs, err := iface.Addrs()
	if err != nil || len(addrs) == 0 {
		return "", ""
	}

	// 取第一个 IPv4 地址
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		if ipnet.IP.To4() != nil {
			ones, _ := ipnet.Mask.Size()
			return ipnet.IP.String(), fmt.Sprintf("%d.%d.%d.%d",
				255<<8>>8<<(8-ones%8)>>8,
				255<<(16-ones)>>8<<ones%8,
				255<<(24-ones)>>16<<(ones%16),
				255<<(32-ones)>>24<<(ones%24))
		}
	}

	return "", ""
}

// getDefaultGateway 获取默认网关
func (m *Manager) getDefaultGateway(iface string) string {
	// 读取路由表
	cmd := exec.Command("ip", "route", "show", "dev", iface)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// 解析默认路由
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "default") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "via" && i+1 < len(fields) {
					return fields[i+1]
				}
			}
		}
	}

	return ""
}

// getDNS 获取 DNS 配置
func (m *Manager) getDNS(iface string) string {
	// 读取 resolv.conf 或 systemd-resolved 状态
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return ""
	}

	var dnsServers []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "nameserver ") {
			dns := strings.TrimPrefix(line, "nameserver ")
			dnsServers = append(dnsServers, dns)
		}
	}

	return strings.Join(dnsServers, ", ")
}

// getInterfaceStats 获取接口流量统计
func (m *Manager) getInterfaceStats(name string) (rxBytes, txBytes int64) {
	rxData, err1 := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/statistics/rx_bytes", name))
	txData, err2 := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/statistics/tx_bytes", name))

	if err1 == nil {
		_, _ = fmt.Sscanf(strings.TrimSpace(string(rxData)), "%d", &rxBytes)
	}
	if err2 == nil {
		_, _ = fmt.Sscanf(strings.TrimSpace(string(txData)), "%d", &txBytes)
	}

	return
}

// parseIPJson 解析 ip -json 输出
func (m *Manager) parseIPJson(data []byte) ([]*Interface, error) {
	// 简化的解析，实际应使用 encoding/json
	// 这里先用正则提取关键信息
	var interfaces []*Interface

	// 提取接口名称
	nameRegex := regexp.MustCompile(`"ifname"\s*:\s*"([^"]+)"`)
	names := nameRegex.FindAllStringSubmatch(string(data), -1)

	for i, match := range names {
		if len(match) > 1 {
			name := match[1]
			iface := &Interface{
				Name: name,
			}

			// 获取详细信息
			info, err := m.getInterfaceInfo(name)
			if err == nil {
				iface = info
			}

			interfaces = append(interfaces, iface)

			// 避免无限循环
			if i > 100 {
				break
			}
		}
	}

	return interfaces, nil
}

// GetInterface 获取单个接口信息
func (m *Manager) GetInterface(name string) (*Interface, error) {
	return m.getInterfaceInfo(name)
}

// ConfigureInterface 配置网络接口
func (m *Manager) ConfigureInterface(name string, config InterfaceConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.DHCP {
		// 使用 DHCP
		cmd := exec.Command("dhclient", name)
		if err := cmd.Run(); err != nil {
			// 尝试使用 systemd-networkd 或 NetworkManager
			return m.configureDHCPNetworkd(name)
		}
		return nil
	}

	// 静态配置
	// 设置 IP 地址
	if config.IP != "" && config.Netmask != "" {
		addr := fmt.Sprintf("%s/%s", config.IP, config.Netmask)
		cmd := exec.Command("ip", "addr", "flush", "dev", name)
		_ = cmd.Run()

		cmd = exec.Command("ip", "addr", "add", addr, "dev", name)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("设置 IP 地址失败: %w", err)
		}
	}

	// 设置网关
	if config.Gateway != "" {
		// 先删除旧的默认路由
		_ = exec.Command("ip", "route", "del", "default", "dev", name).Run()

		cmd := exec.Command("ip", "route", "add", "default", "via", config.Gateway, "dev", name)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("设置网关失败: %w", err)
		}
	}

	// 设置 DNS
	if config.DNS != "" {
		if err := m.setDNS(config.DNS); err != nil {
			return fmt.Errorf("设置 DNS 失败: %w", err)
		}
	}

	return nil
}

// configureDHCPNetworkd 使用 systemd-networkd 配置 DHCP
func (m *Manager) configureDHCPNetworkd(name string) error {
	config := fmt.Sprintf(`[Match]
Name=%s

[Network]
DHCP=yes
`, name)

	configPath := fmt.Sprintf("/etc/systemd/network/10-%s.network", name)
	return os.WriteFile(configPath, []byte(config), 0644)
}

// setDNS 设置 DNS 服务器
func (m *Manager) setDNS(dns string) error {
	// 写入 resolv.conf
	content := "# Generated by NAS-OS\n"
	for _, server := range strings.Split(dns, ",") {
		server = strings.TrimSpace(server)
		if server != "" {
			content += fmt.Sprintf("nameserver %s\n", server)
		}
	}

	return os.WriteFile("/etc/resolv.conf", []byte(content), 0644)
}

// ToggleInterface 启用/禁用网络接口
func (m *Manager) ToggleInterface(name string, up bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var cmd *exec.Cmd
	if up {
		cmd = exec.Command("ip", "link", "set", name, "up")
	} else {
		cmd = exec.Command("ip", "link", "set", name, "down")
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("切换接口状态失败: %w", err)
	}

	return nil
}

// GetNetworkStats 获取网络统计信息
func (m *Manager) GetNetworkStats() (*NetworkStats, error) {
	ifaces, err := m.ListInterfaces()
	if err != nil {
		return nil, err
	}

	stats := &NetworkStats{}
	for _, iface := range ifaces {
		stats.Interfaces = append(stats.Interfaces, InterfaceStats{
			Name:      iface.Name,
			RxBytes:   iface.RxBytes,
			TxBytes:   iface.TxBytes,
			RxPackets: 0,
			TxPackets: 0,
		})
		stats.TotalRxBytes += iface.RxBytes
		stats.TotalTxBytes += iface.TxBytes
	}

	return stats, nil
}
