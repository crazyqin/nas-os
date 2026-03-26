package security

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FirewallManager 防火墙管理器.
type FirewallManager struct {
	config      FirewallConfig
	rules       map[string]*FirewallRule
	ipBlacklist map[string]*IPBlacklistEntry
	ipWhitelist map[string]*IPWhitelistEntry
	mu          sync.RWMutex
	geoDB       *GeoIPDB // 地理位置数据库
}

// GeoIPDB 地理位置数据库（简化版）.
type GeoIPDB struct {
	// 实际实现会使用 MaxMind GeoIP2 数据库
}

// NewFirewallManager 创建防火墙管理器.
func NewFirewallManager() *FirewallManager {
	return &FirewallManager{
		config: FirewallConfig{
			Enabled:       true,
			DefaultPolicy: "deny",
			IPv6Enabled:   true,
			LogDropped:    true,
		},
		rules:       make(map[string]*FirewallRule),
		ipBlacklist: make(map[string]*IPBlacklistEntry),
		ipWhitelist: make(map[string]*IPWhitelistEntry),
		geoDB:       &GeoIPDB{},
	}
}

// GetConfig 获取防火墙配置.
func (fm *FirewallManager) GetConfig() FirewallConfig {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.config
}

// UpdateConfig 更新防火墙配置.
func (fm *FirewallManager) UpdateConfig(config FirewallConfig) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm.config = config

	// 应用配置到系统防火墙
	if err := fm.applyConfig(); err != nil {
		return fmt.Errorf("应用防火墙配置失败：%w", err)
	}

	return nil
}

// applyConfig 应用配置到系统防火墙（使用 iptables/nftables）.
func (fm *FirewallManager) applyConfig() error {
	if !fm.config.Enabled {
		// 禁用防火墙
		return fm.disableFirewall()
	}

	// 设置默认策略
	if err := fm.setDefaultPolicy(fm.config.DefaultPolicy); err != nil {
		return err
	}

	// 启用 IPv6 支持
	if fm.config.IPv6Enabled {
		if err := fm.enableIPv6Firewall(); err != nil {
			return err
		}
	}

	return nil
}

// setDefaultPolicy 设置默认策略.
func (fm *FirewallManager) setDefaultPolicy(policy string) error {
	// 使用 iptables 设置默认策略
	chainPolicy := "ACCEPT"
	if policy == "deny" {
		chainPolicy = "DROP"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "iptables", "-P", "INPUT", chainPolicy)
	if err := cmd.Run(); err != nil {
		// 如果没有权限，记录但不返回错误（兼容非 root 环境）
		return nil
	}

	if fm.config.IPv6Enabled {
		cmd = exec.CommandContext(ctx, "ip6tables", "-P", "INPUT", chainPolicy)
		_ = cmd.Run()
	}

	return nil
}

// enableIPv6Firewall 启用 IPv6 防火墙.
func (fm *FirewallManager) enableIPv6Firewall() error {
	// 启用 IPv6 防火墙规则
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ip6tables", "-L")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("IPv6 防火墙不可用：%w", err)
	}
	return nil
}

// disableFirewall 禁用防火墙.
func (fm *FirewallManager) disableFirewall() error {
	// 设置默认接受所有流量
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "iptables", "-P", "INPUT", "ACCEPT")
	_ = cmd.Run()
	cmd = exec.CommandContext(ctx, "iptables", "-P", "FORWARD", "ACCEPT")
	_ = cmd.Run()
	cmd = exec.CommandContext(ctx, "iptables", "-P", "OUTPUT", "ACCEPT")
	_ = cmd.Run()
	return nil
}

// ListRules 获取所有防火墙规则.
func (fm *FirewallManager) ListRules() []*FirewallRule {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	rules := make([]*FirewallRule, 0, len(fm.rules))
	for _, rule := range fm.rules {
		rules = append(rules, rule)
	}

	// 按优先级排序
	sortRulesByPriority(rules)
	return rules
}

// sortRulesByPriority 按优先级排序规则.
func sortRulesByPriority(rules []*FirewallRule) {
	for i := 0; i < len(rules)-1; i++ {
		for j := i + 1; j < len(rules); j++ {
			if rules[i].Priority > rules[j].Priority {
				rules[i], rules[j] = rules[j], rules[i]
			}
		}
	}
}

// GetRule 获取单条规则.
func (fm *FirewallManager) GetRule(id string) (*FirewallRule, bool) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	rule, exists := fm.rules[id]
	if !exists {
		return nil, false
	}

	// 返回副本
	ruleCopy := *rule
	return &ruleCopy, true
}

// AddRule 添加防火墙规则.
func (fm *FirewallManager) AddRule(rule FirewallRule) (*FirewallRule, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// 验证规则
	if err := fm.validateRule(&rule); err != nil {
		return nil, err
	}

	// 生成 ID
	rule.ID = uuid.New().String()
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	// 如果未设置优先级，自动分配
	if rule.Priority == 0 {
		rule.Priority = len(fm.rules) + 1
	}

	// 保存规则
	fm.rules[rule.ID] = &rule

	// 应用到系统防火墙
	if err := fm.applyRuleToSystem(&rule); err != nil {
		delete(fm.rules, rule.ID)
		return nil, fmt.Errorf("应用规则到系统防火墙失败：%w", err)
	}

	ruleCopy := rule
	return &ruleCopy, nil
}

// validateRule 验证防火墙规则.
func (fm *FirewallManager) validateRule(rule *FirewallRule) error {
	// 验证动作
	validActions := map[string]bool{"allow": true, "deny": true, "drop": true}
	if !validActions[rule.Action] {
		return fmt.Errorf("无效的动作：%s", rule.Action)
	}

	// 验证协议
	validProtocols := map[string]bool{"tcp": true, "udp": true, "icmp": true, "all": true}
	if !validProtocols[rule.Protocol] {
		return fmt.Errorf("无效的协议：%s", rule.Protocol)
	}

	// 验证方向
	validDirections := map[string]bool{"inbound": true, "outbound": true}
	if !validDirections[rule.Direction] {
		return fmt.Errorf("无效的方向：%s", rule.Direction)
	}

	// 验证 IP 地址
	if rule.SourceIP != "" {
		if _, _, err := net.ParseCIDR(rule.SourceIP); err != nil {
			if net.ParseIP(rule.SourceIP) == nil {
				return fmt.Errorf("无效的源 IP 地址：%s", rule.SourceIP)
			}
		}
	}

	if rule.DestIP != "" {
		if _, _, err := net.ParseCIDR(rule.DestIP); err != nil {
			if net.ParseIP(rule.DestIP) == nil {
				return fmt.Errorf("无效的目标 IP 地址：%s", rule.DestIP)
			}
		}
	}

	// 验证端口
	if rule.SourcePort != "" {
		if err := fm.validatePort(rule.SourcePort); err != nil {
			return fmt.Errorf("无效的源端口：%w", err)
		}
	}

	if rule.DestPort != "" {
		if err := fm.validatePort(rule.DestPort); err != nil {
			return fmt.Errorf("无效的目标端口：%w", err)
		}
	}

	return nil
}

// validatePort 验证端口号.
func (fm *FirewallManager) validatePort(port string) error {
	// 支持单个端口、端口范围、端口列表
	if strings.Contains(port, "-") {
		// 端口范围
		parts := strings.Split(port, "-")
		if len(parts) != 2 {
			return fmt.Errorf("无效的端口范围")
		}
		var start, end int
		if _, err := fmt.Sscanf(parts[0], "%d", &start); err != nil {
			return fmt.Errorf("无效的起始端口：%s", parts[0])
		}
		if _, err := fmt.Sscanf(parts[1], "%d", &end); err != nil {
			return fmt.Errorf("无效的结束端口：%s", parts[1])
		}
		if start < 1 || start > 65535 || end < 1 || end > 65535 || start > end {
			return fmt.Errorf("端口范围无效")
		}
	} else if strings.Contains(port, ",") {
		// 端口列表
		parts := strings.Split(port, ",")
		for _, p := range parts {
			var portNum int
			if _, err := fmt.Sscanf(p, "%d", &portNum); err != nil || portNum < 1 || portNum > 65535 {
				return fmt.Errorf("无效的端口号：%s", p)
			}
		}
	} else {
		// 单个端口
		var portNum int
		if _, err := fmt.Sscanf(port, "%d", &portNum); err != nil || portNum < 1 || portNum > 65535 {
			return fmt.Errorf("无效的端口号")
		}
	}

	return nil
}

// applyRuleToSystem 应用规则到系统防火墙.
func (fm *FirewallManager) applyRuleToSystem(rule *FirewallRule) error {
	if !rule.Enabled {
		return nil
	}

	// 构建 iptables 命令
	chain := "INPUT"
	if rule.Direction == "outbound" {
		chain = "OUTPUT"
	}

	action := "-j ACCEPT"
	if rule.Action == "deny" || rule.Action == "drop" {
		action = "-j DROP"
	}

	args := []string{"-A", chain}

	// 协议
	if rule.Protocol != "all" {
		args = append(args, "-p", rule.Protocol)
	}

	// 源 IP
	if rule.SourceIP != "" {
		args = append(args, "-s", rule.SourceIP)
	}

	// 目标 IP
	if rule.DestIP != "" {
		args = append(args, "-d", rule.DestIP)
	}

	// 目标端口
	if rule.DestPort != "" && rule.Protocol != "icmp" {
		args = append(args, "--dport", rule.DestPort)
	}

	// 动作
	args = append(args, strings.Fields(action)...)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "iptables", args...)
	if err := cmd.Run(); err != nil {
		// 非 root 环境下可能失败，记录但不返回错误
		return nil
	}

	// IPv6 支持
	if fm.config.IPv6Enabled {
		cmd = exec.CommandContext(ctx, "ip6tables", args...)
		_ = cmd.Run()
	}

	return nil
}

// UpdateRule 更新防火墙规则.
func (fm *FirewallManager) UpdateRule(id string, rule FirewallRule) (*FirewallRule, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	existing, exists := fm.rules[id]
	if !exists {
		return nil, fmt.Errorf("规则不存在")
	}

	// 保留原始 ID 和创建时间
	rule.ID = id
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()

	// 验证规则
	if err := fm.validateRule(&rule); err != nil {
		return nil, err
	}

	// 从系统防火墙移除旧规则
	if existing.Enabled {
		_ = fm.removeRuleFromSystem(existing)
	}

	// 保存新规则
	fm.rules[id] = &rule

	// 应用新规则到系统防火墙
	if err := fm.applyRuleToSystem(&rule); err != nil {
		fm.rules[id] = existing // 恢复旧规则
		return nil, fmt.Errorf("应用规则失败：%w", err)
	}

	ruleCopy := rule
	return &ruleCopy, nil
}

// DeleteRule 删除防火墙规则.
func (fm *FirewallManager) DeleteRule(id string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	rule, exists := fm.rules[id]
	if !exists {
		return fmt.Errorf("规则不存在")
	}

	// 从系统防火墙移除
	if rule.Enabled {
		if err := fm.removeRuleFromSystem(rule); err != nil {
			return err
		}
	}

	delete(fm.rules, id)
	return nil
}

// removeRuleFromSystem 从系统防火墙移除规则.
func (fm *FirewallManager) removeRuleFromSystem(rule *FirewallRule) error {
	// 实际实现需要删除对应的 iptables 规则
	// 这里简化处理
	return nil
}

// ========== IP 黑名单管理 ==========

// GetBlacklist 获取 IP 黑名单.
func (fm *FirewallManager) GetBlacklist() []*IPBlacklistEntry {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	entries := make([]*IPBlacklistEntry, 0, len(fm.ipBlacklist))
	for _, entry := range fm.ipBlacklist {
		entries = append(entries, entry)
	}
	return entries
}

// AddToBlacklist 添加 IP 到黑名单.
func (fm *FirewallManager) AddToBlacklist(ip, reason string, durationMinutes int) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// 验证 IP 地址
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("无效的 IP 地址")
	}

	entry := &IPBlacklistEntry{
		IP:        ip,
		Reason:    reason,
		CreatedAt: time.Now(),
	}

	if durationMinutes > 0 {
		expiresAt := time.Now().Add(time.Duration(durationMinutes) * time.Minute)
		entry.ExpiresAt = &expiresAt
	}

	fm.ipBlacklist[ip] = entry

	// 应用到系统防火墙
	return fm.blockIP(ip)
}

// blockIP 阻止 IP 地址.
func (fm *FirewallManager) blockIP(ip string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "iptables", "-A", "INPUT", "-s", ip, "-j", "DROP")
	if err := cmd.Run(); err != nil {
		return nil // 非 root 环境
	}

	if fm.config.IPv6Enabled && net.ParseIP(ip).To4() == nil {
		cmd = exec.CommandContext(ctx, "ip6tables", "-A", "INPUT", "-s", ip, "-j", "DROP")
		_ = cmd.Run()
	}

	return nil
}

// RemoveFromBlacklist 从黑名单移除 IP.
func (fm *FirewallManager) RemoveFromBlacklist(ip string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, exists := fm.ipBlacklist[ip]; !exists {
		return fmt.Errorf("IP 不在黑名单中")
	}

	delete(fm.ipBlacklist, ip)

	// 从系统防火墙移除
	return fm.unblockIP(ip)
}

// unblockIP 解除 IP 阻止.
func (fm *FirewallManager) unblockIP(ip string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "iptables", "-D", "INPUT", "-s", ip, "-j", "DROP")
	_ = cmd.Run()

	if fm.config.IPv6Enabled {
		cmd = exec.CommandContext(ctx, "ip6tables", "-D", "INPUT", "-s", ip, "-j", "DROP")
		_ = cmd.Run()
	}

	return nil
}

// IsBlacklisted 检查 IP 是否在黑名单中.
func (fm *FirewallManager) IsBlacklisted(ip string) bool {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	entry, exists := fm.ipBlacklist[ip]
	if !exists {
		return false
	}

	// 检查是否过期
	if entry.ExpiresAt != nil && time.Now().After(*entry.ExpiresAt) {
		return false
	}

	return true
}

// ========== IP 白名单管理 ==========

// GetWhitelist 获取 IP 白名单.
func (fm *FirewallManager) GetWhitelist() []*IPWhitelistEntry {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	entries := make([]*IPWhitelistEntry, 0, len(fm.ipWhitelist))
	for _, entry := range fm.ipWhitelist {
		entries = append(entries, entry)
	}
	return entries
}

// AddToWhitelist 添加 IP 到白名单.
func (fm *FirewallManager) AddToWhitelist(ip, reason string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if net.ParseIP(ip) == nil {
		return fmt.Errorf("无效的 IP 地址")
	}

	fm.ipWhitelist[ip] = &IPWhitelistEntry{
		IP:        ip,
		Reason:    reason,
		CreatedAt: time.Now(),
	}

	return nil
}

// RemoveFromWhitelist 从白名单移除 IP.
func (fm *FirewallManager) RemoveFromWhitelist(ip string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, exists := fm.ipWhitelist[ip]; !exists {
		return fmt.Errorf("IP 不在白名单中")
	}

	delete(fm.ipWhitelist, ip)
	return nil
}

// IsWhitelisted 检查 IP 是否在白名单中.
func (fm *FirewallManager) IsWhitelisted(ip string) bool {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	_, exists := fm.ipWhitelist[ip]
	return exists
}

// CleanupExpired 清理过期的黑名单条目.
func (fm *FirewallManager) CleanupExpired() {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	now := time.Now()
	for ip, entry := range fm.ipBlacklist {
		if entry.ExpiresAt != nil && now.After(*entry.ExpiresAt) {
			delete(fm.ipBlacklist, ip)
			// 从系统防火墙移除
			_ = fm.unblockIP(ip)
		}
	}
}

// StartCleanupRoutine 启动定期清理例程.
func (fm *FirewallManager) StartCleanupRoutine(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			fm.CleanupExpired()
		}
	}()
}
