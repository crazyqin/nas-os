package network

import (
	"fmt"
	"os/exec"
	"strings"
)

// ListFirewallRules 列出所有防火墙规则
func (m *Manager) ListFirewallRules() []*FirewallRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var rules []*FirewallRule
	for _, rule := range m.firewallRules {
		rules = append(rules, rule)
	}
	return rules
}

// GetFirewallRule 获取单个防火墙规则
func (m *Manager) GetFirewallRule(name string) (*FirewallRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, ok := m.firewallRules[name]
	if !ok {
		return nil, fmt.Errorf("防火墙规则不存在: %s", name)
	}
	return rule, nil
}

// AddFirewallRule 添加防火墙规则
func (m *Manager) AddFirewallRule(rule FirewallRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.Name == "" {
		return fmt.Errorf("规则名称不能为空")
	}

	// 设置默认值
	if rule.Protocol == "" {
		rule.Protocol = "all"
	}
	if rule.Action == "" {
		rule.Action = "accept"
	}
	if rule.Direction == "" {
		rule.Direction = "in"
	}

	// 验证参数
	rule.Protocol = strings.ToLower(rule.Protocol)
	if rule.Protocol != "tcp" && rule.Protocol != "udp" && rule.Protocol != "icmp" && rule.Protocol != "all" {
		return fmt.Errorf("协议只能是 tcp, udp, icmp 或 all")
	}

	rule.Action = strings.ToLower(rule.Action)
	if rule.Action != "accept" && rule.Action != "drop" && rule.Action != "reject" {
		return fmt.Errorf("动作只能是 accept, drop 或 reject")
	}

	rule.Direction = strings.ToLower(rule.Direction)
	if rule.Direction != "in" && rule.Direction != "out" && rule.Direction != "forward" {
		return fmt.Errorf("方向只能是 in, out 或 forward")
	}

	// 检查是否已存在
	if _, ok := m.firewallRules[rule.Name]; ok {
		return fmt.Errorf("防火墙规则已存在: %s", rule.Name)
	}

	// 应用 iptables 规则
	if rule.Enabled {
		if err := m.applyFirewallRule(&rule); err != nil {
			return fmt.Errorf("应用规则失败: %w", err)
		}
	}

	m.firewallRules[rule.Name] = &rule
	m.saveConfig()

	return nil
}

// UpdateFirewallRule 更新防火墙规则
func (m *Manager) UpdateFirewallRule(name string, rule FirewallRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldRule, ok := m.firewallRules[name]
	if !ok {
		return fmt.Errorf("防火墙规则不存在: %s", name)
	}

	// 先删除旧规则
	if oldRule.Enabled {
		m.removeFirewallRule(oldRule)
	}

	// 应用新规则
	if rule.Enabled {
		if err := m.applyFirewallRule(&rule); err != nil {
			return fmt.Errorf("应用规则失败: %w", err)
		}
	}

	// 如果名称改变，需要更新 map
	if rule.Name != name {
		delete(m.firewallRules, name)
	}

	m.firewallRules[rule.Name] = &rule
	m.saveConfig()

	return nil
}

// DeleteFirewallRule 删除防火墙规则
func (m *Manager) DeleteFirewallRule(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.firewallRules[name]
	if !ok {
		return fmt.Errorf("防火墙规则不存在: %s", name)
	}

	// 从 iptables 移除
	if rule.Enabled {
		m.removeFirewallRule(rule)
	}

	delete(m.firewallRules, name)
	m.saveConfig()

	return nil
}

// EnableFirewallRule 启用/禁用防火墙规则
func (m *Manager) EnableFirewallRule(name string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.firewallRules[name]
	if !ok {
		return fmt.Errorf("防火墙规则不存在: %s", name)
	}

	if rule.Enabled == enabled {
		return nil // 状态已经是目标状态
	}

	if enabled {
		if err := m.applyFirewallRule(rule); err != nil {
			return fmt.Errorf("应用规则失败: %w", err)
		}
	} else {
		m.removeFirewallRule(rule)
	}

	rule.Enabled = enabled
	m.saveConfig()

	return nil
}

// applyFirewallRule 应用防火墙规则到 iptables
func (m *Manager) applyFirewallRule(rule *FirewallRule) error {
	// 构建基本命令
	var chain string
	switch rule.Direction {
	case "in":
		chain = "INPUT"
	case "out":
		chain = "OUTPUT"
	case "forward":
		chain = "FORWARD"
	}

	args := []string{"-A", chain}

	// 添加协议
	if rule.Protocol != "all" {
		args = append(args, "-p", rule.Protocol)
	}

	// 添加源 IP
	if rule.SourceIP != "" {
		args = append(args, "-s", rule.SourceIP)
	}

	// 添加目标 IP
	if rule.DestIP != "" {
		args = append(args, "-d", rule.DestIP)
	}

	// 添加目标端口
	if rule.DestPort != "" {
		args = append(args, "--dport", rule.DestPort)
	}

	// 添加动作
	var action string
	switch rule.Action {
	case "accept":
		action = "ACCEPT"
	case "drop":
		action = "DROP"
	case "reject":
		action = "REJECT"
	}
	args = append(args, "-j", action)

	// 执行 iptables 命令
	cmd := exec.Command("iptables", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("iptables 执行失败: %w, 输出: %s", err, string(output))
	}

	return nil
}

// removeFirewallRule 从 iptables 移除防火墙规则
func (m *Manager) removeFirewallRule(rule *FirewallRule) {
	var chain string
	switch rule.Direction {
	case "in":
		chain = "INPUT"
	case "out":
		chain = "OUTPUT"
	case "forward":
		chain = "FORWARD"
	}

	args := []string{"-D", chain}

	if rule.Protocol != "all" {
		args = append(args, "-p", rule.Protocol)
	}

	if rule.SourceIP != "" {
		args = append(args, "-s", rule.SourceIP)
	}

	if rule.DestIP != "" {
		args = append(args, "-d", rule.DestIP)
	}

	if rule.DestPort != "" {
		args = append(args, "--dport", rule.DestPort)
	}

	var action string
	switch rule.Action {
	case "accept":
		action = "ACCEPT"
	case "drop":
		action = "DROP"
	case "reject":
		action = "REJECT"
	}
	args = append(args, "-j", action)

	_ = exec.Command("iptables", args...).Run()
}

// ListActiveFirewallRules 列出系统中活跃的防火墙规则
func (m *Manager) ListActiveFirewallRules() (map[string][]string, error) {
	chains := []string{"INPUT", "OUTPUT", "FORWARD"}
	result := make(map[string][]string)

	for _, chain := range chains {
		cmd := exec.Command("iptables", "-L", chain, "-n", "--line-numbers")
		output, err := cmd.Output()
		if err != nil {
			return nil, err
		}

		lines := strings.Split(string(output), "\n")
		var rules []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "Chain") && !strings.HasPrefix(line, "num") {
				rules = append(rules, line)
			}
		}
		result[chain] = rules
	}

	return result, nil
}

// GetFirewallStatus 获取防火墙状态
func (m *Manager) GetFirewallStatus() (*FirewallStatus, error) {
	status := &FirewallStatus{
		Enabled: false,
	}

	// 检查是否有任何 INPUT 规则（除了默认规则）
	cmd := exec.Command("iptables", "-L", "INPUT", "-n")
	output, err := cmd.Output()
	if err != nil {
		return status, nil // iptables 可能未安装或未运行
	}

	lines := strings.Split(string(output), "\n")
	// 跳过前两行（Chain 和 header）
	for i, line := range lines {
		if i < 2 {
			continue
		}
		line = strings.TrimSpace(line)
		if line != "" && !strings.Contains(line, "state RELATED,ESTABLISHED") {
			status.Enabled = true
			break
		}
	}

	// 获取各链的默认策略
	for _, chain := range []string{"INPUT", "OUTPUT", "FORWARD"} {
		cmd := exec.Command("iptables", "-L", chain, "-n")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "Chain "+chain) {
					if strings.Contains(line, "DROP") {
						status.DefaultPolicy = chain + ": DROP"
					} else if strings.Contains(line, "ACCEPT") {
						status.DefaultPolicy = chain + ": ACCEPT"
					}
				}
			}
		}
	}

	return status, nil
}

// FirewallStatus 防火墙状态
type FirewallStatus struct {
	Enabled       bool   `json:"enabled"`
	DefaultPolicy string `json:"defaultPolicy"`
}

// SetDefaultPolicy 设置防火墙默认策略
func (m *Manager) SetDefaultPolicy(chain, policy string) error {
	chain = strings.ToUpper(chain)
	policy = strings.ToUpper(policy)

	if chain != "INPUT" && chain != "OUTPUT" && chain != "FORWARD" {
		return fmt.Errorf("无效的链: %s", chain)
	}

	if policy != "ACCEPT" && policy != "DROP" {
		return fmt.Errorf("策略只能是 ACCEPT 或 DROP")
	}

	cmd := exec.Command("iptables", "-P", chain, policy)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("设置默认策略失败: %w, 输出: %s", err, string(output))
	}

	return nil
}

// FlushRules 清空指定链的所有规则
func (m *Manager) FlushRules(chain string) error {
	chain = strings.ToUpper(chain)

	if chain != "INPUT" && chain != "OUTPUT" && chain != "FORWARD" && chain != "ALL" {
		return fmt.Errorf("无效的链: %s", chain)
	}

	if chain == "ALL" {
		for _, c := range []string{"INPUT", "OUTPUT", "FORWARD"} {
			cmd := exec.Command("iptables", "-F", c)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("清空链 %s 失败: %w", c, err)
			}
		}
	} else {
		cmd := exec.Command("iptables", "-F", chain)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("清空链失败: %w", err)
		}
	}

	return nil
}

// SaveFirewallRules 保存防火墙规则到文件
func (m *Manager) SaveFirewallRules(path string) error {
	cmd := exec.Command("iptables-save")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("获取规则失败: %w", err)
	}

	if path == "" {
		path = "/etc/iptables/rules.v4"
	}

	return exec.Command("sh", "-c", fmt.Sprintf("echo '%s' > %s", string(output), path)).Run()
}

// RestoreFirewallRules 从文件恢复防火墙规则
func (m *Manager) RestoreFirewallRules(path string) error {
	if path == "" {
		path = "/etc/iptables/rules.v4"
	}

	cmd := exec.Command("iptables-restore", "<", path)
	return cmd.Run()
}
