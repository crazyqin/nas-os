package network

import (
	"fmt"
	"os/exec"
	"strings"
)

// ListPortForwards 列出所有端口转发规则
func (m *Manager) ListPortForwards() []*PortForward {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var rules []*PortForward
	for _, rule := range m.portForwards {
		rules = append(rules, rule)
	}
	return rules
}

// GetPortForward 获取单个端口转发规则
func (m *Manager) GetPortForward(name string) (*PortForward, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, ok := m.portForwards[name]
	if !ok {
		return nil, fmt.Errorf("端口转发规则不存在: %s", name)
	}
	return rule, nil
}

// AddPortForward 添加端口转发规则
func (m *Manager) AddPortForward(rule PortForward) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.Name == "" {
		return fmt.Errorf("规则名称不能为空")
	}

	if rule.ExternalPort <= 0 || rule.ExternalPort > 65535 {
		return fmt.Errorf("外部端口无效: %d", rule.ExternalPort)
	}

	if rule.InternalPort <= 0 || rule.InternalPort > 65535 {
		return fmt.Errorf("内部端口无效: %d", rule.InternalPort)
	}

	if rule.InternalIP == "" {
		return fmt.Errorf("内部 IP 不能为空")
	}

	if rule.Protocol == "" {
		rule.Protocol = "tcp"
	}

	rule.Protocol = strings.ToLower(rule.Protocol)
	if rule.Protocol != "tcp" && rule.Protocol != "udp" {
		return fmt.Errorf("协议只能是 tcp 或 udp")
	}

	// 检查是否已存在
	if _, ok := m.portForwards[rule.Name]; ok {
		return fmt.Errorf("端口转发规则已存在: %s", rule.Name)
	}

	// 应用 iptables 规则
	if rule.Enabled {
		if err := m.applyPortForwardRule(&rule); err != nil {
			return fmt.Errorf("应用规则失败: %w", err)
		}
	}

	m.portForwards[rule.Name] = &rule
	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	return nil
}

// UpdatePortForward 更新端口转发规则
func (m *Manager) UpdatePortForward(name string, rule PortForward) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldRule, ok := m.portForwards[name]
	if !ok {
		return fmt.Errorf("端口转发规则不存在: %s", name)
	}

	// 先删除旧规则
	if oldRule.Enabled {
		m.removePortForwardRule(oldRule)
	}

	// 应用新规则
	if rule.Enabled {
		if err := m.applyPortForwardRule(&rule); err != nil {
			return fmt.Errorf("应用规则失败: %w", err)
		}
	}

	// 如果名称改变，需要更新 map
	if rule.Name != name {
		delete(m.portForwards, name)
	}

	m.portForwards[rule.Name] = &rule
	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	return nil
}

// DeletePortForward 删除端口转发规则
func (m *Manager) DeletePortForward(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.portForwards[name]
	if !ok {
		return fmt.Errorf("端口转发规则不存在: %s", name)
	}

	// 从 iptables 移除
	if rule.Enabled {
		m.removePortForwardRule(rule)
	}

	delete(m.portForwards, name)
	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	return nil
}

// EnablePortForward 启用/禁用端口转发规则
func (m *Manager) EnablePortForward(name string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.portForwards[name]
	if !ok {
		return fmt.Errorf("端口转发规则不存在: %s", name)
	}

	if rule.Enabled == enabled {
		return nil // 状态已经是目标状态
	}

	if enabled {
		if err := m.applyPortForwardRule(rule); err != nil {
			return fmt.Errorf("应用规则失败: %w", err)
		}
	} else {
		m.removePortForwardRule(rule)
	}

	rule.Enabled = enabled
	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	return nil
}

// applyPortForwardRule 应用端口转发规则到 iptables
func (m *Manager) applyPortForwardRule(rule *PortForward) error {
	// 1. DNAT 规则：将外部流量重定向到内部 IP
	// iptables -t nat -A PREROUTING -p tcp --dport 80 -j DNAT --to-destination 192.168.1.100:80
	dnatCmd := exec.Command("iptables", "-t", "nat", "-A", "PREROUTING",
		"-p", rule.Protocol,
		"--dport", fmt.Sprintf("%d", rule.ExternalPort),
		"-j", "DNAT",
		"--to-destination", fmt.Sprintf("%s:%d", rule.InternalIP, rule.InternalPort))

	if err := dnatCmd.Run(); err != nil {
		return fmt.Errorf("添加 DNAT 规则失败: %w", err)
	}

	// 2. SNAT 规则：修改源地址，确保返回流量经过网关
	// iptables -t nat -A POSTROUTING -d 192.168.1.100 -p tcp --dport 80 -j MASQUERADE
	snatCmd := exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING",
		"-d", rule.InternalIP,
		"-p", rule.Protocol,
		"--dport", fmt.Sprintf("%d", rule.InternalPort),
		"-j", "MASQUERADE")

	if err := snatCmd.Run(); err != nil {
		// 如果 SNAT 失败，回滚 DNAT
		_ = exec.Command("iptables", "-t", "nat", "-D", "PREROUTING",
			"-p", rule.Protocol,
			"--dport", fmt.Sprintf("%d", rule.ExternalPort),
			"-j", "DNAT",
			"--to-destination", fmt.Sprintf("%s:%d", rule.InternalIP, rule.InternalPort)).Run()
		return fmt.Errorf("添加 SNAT 规则失败: %w", err)
	}

	// 3. FORWARD 规则：允许转发
	// iptables -A FORWARD -p tcp -d 192.168.1.100 --dport 80 -j ACCEPT
	forwardCmd := exec.Command("iptables", "-A", "FORWARD",
		"-p", rule.Protocol,
		"-d", rule.InternalIP,
		"--dport", fmt.Sprintf("%d", rule.InternalPort),
		"-j", "ACCEPT")

	if err := forwardCmd.Run(); err != nil {
		// 如果 FORWARD 失败，回滚前面的规则
		m.removePortForwardRule(rule)
		return fmt.Errorf("添加 FORWARD 规则失败: %w", err)
	}

	// 确保 IP 转发已启用
	// 忽略错误，IP转发可能已经启用
	_ = exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()

	return nil
}

// removePortForwardRule 从 iptables 移除端口转发规则
func (m *Manager) removePortForwardRule(rule *PortForward) {
	// 删除 DNAT 规则
	_ = exec.Command("iptables", "-t", "nat", "-D", "PREROUTING",
		"-p", rule.Protocol,
		"--dport", fmt.Sprintf("%d", rule.ExternalPort),
		"-j", "DNAT",
		"--to-destination", fmt.Sprintf("%s:%d", rule.InternalIP, rule.InternalPort)).Run()

	// 删除 SNAT 规则
	_ = exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING",
		"-d", rule.InternalIP,
		"-p", rule.Protocol,
		"--dport", fmt.Sprintf("%d", rule.InternalPort),
		"-j", "MASQUERADE").Run()

	// 删除 FORWARD 规则
	_ = exec.Command("iptables", "-D", "FORWARD",
		"-p", rule.Protocol,
		"-d", rule.InternalIP,
		"--dport", fmt.Sprintf("%d", rule.InternalPort),
		"-j", "ACCEPT").Run()
}

// ListActivePortForwards 列出系统中活跃的端口转发规则
func (m *Manager) ListActivePortForwards() ([]string, error) {
	// 获取 NAT 表的 PREROUTING 链
	cmd := exec.Command("iptables", "-t", "nat", "-L", "PREROUTING", "-n")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var rules []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "DNAT") {
			rules = append(rules, line)
		}
	}

	return rules, nil
}

// GetPortForwardStatus 获取端口转发规则状态
func (m *Manager) GetPortForwardStatus(name string) (string, error) {
	rule, err := m.GetPortForward(name)
	if err != nil {
		return "", err
	}

	if !rule.Enabled {
		return "disabled", nil
	}

	// 检查 iptables 中是否存在该规则
	cmd := exec.Command("iptables", "-t", "nat", "-C", "PREROUTING",
		"-p", rule.Protocol,
		"--dport", fmt.Sprintf("%d", rule.ExternalPort),
		"-j", "DNAT",
		"--to-destination", fmt.Sprintf("%s:%d", rule.InternalIP, rule.InternalPort))

	if err := cmd.Run(); err != nil {
		return "inactive", nil // 规则不存在于 iptables
	}

	return "active", nil
}
