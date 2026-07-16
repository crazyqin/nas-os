package nids

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

// DefaultRules 返回内置检测规则集.
func DefaultRules() []*Rule {
	return []*Rule{
		// SQL 注入检测
		{
			ID:       "rule-sqli-001",
			Name:     "SQL Injection - UNION SELECT",
			Enabled:  true,
			Priority: 1,
			Severity: SeverityCritical,
			Action:   ActionBlock,
			Type:     DetectionSignature,
			Protocol: ProtoAny,
			Pattern:  `(?i)(union\s+select|union\s+all\s+select)`,
			Tags:     []string{"sqli", "web", "injection"},
		},
		{
			ID:       "rule-sqli-002",
			Name:     "SQL Injection - OR 1=1",
			Enabled:  true,
			Priority: 1,
			Severity: SeverityHigh,
			Action:   ActionAlert,
			Type:     DetectionSignature,
			Protocol: ProtoAny,
			Pattern:  `(?i)(or\s+1\s*=\s*1|'\s+or\s+')`,
			Tags:     []string{"sqli", "web", "injection"},
		},
		// XSS 检测
		{
			ID:       "rule-xss-001",
			Name:     "XSS - Script Tag",
			Enabled:  true,
			Priority: 2,
			Severity: SeverityHigh,
			Action:   ActionAlert,
			Type:     DetectionSignature,
			Protocol: ProtoAny,
			Pattern:  `(?i)<\s*script[^>]*>`,
			Tags:     []string{"xss", "web"},
		},
		// 命令注入检测
		{
			ID:       "rule-cmdi-001",
			Name:     "Command Injection - Shell Commands",
			Enabled:  true,
			Priority: 1,
			Severity: SeverityCritical,
			Action:   ActionBlock,
			Type:     DetectionSignature,
			Protocol: ProtoAny,
			Pattern:  `(?i)(;\s*(ls|cat|wget|curl|bash|sh|nc|ncat)\b|` + "`" + `[^` + "`" + `]+` + "`" + `)`,
			Tags:     []string{"cmdi", "injection"},
		},
		// 目录遍历检测
		{
			ID:       "rule-traversal-001",
			Name:     "Directory Traversal",
			Enabled:  true,
			Priority: 2,
			Severity: SeverityHigh,
			Action:   ActionAlert,
			Type:     DetectionSignature,
			Protocol: ProtoAny,
			Pattern:  `\.\./\.\./|\.\.\\|\.\.%2[fF]|\.\.%5[cC]`,
			Tags:     []string{"traversal", "web"},
		},
		// 端口扫描检测
		{
			ID:       "rule-scan-001",
			Name:     "Port Scan Detection",
			Enabled:  true,
			Priority: 3,
			Severity: SeverityMedium,
			Action:   ActionAlert,
			Type:     DetectionAnomaly,
			Protocol: ProtoTCP,
			Threshold: &ThresholdConfig{
				Count:   20,
				Seconds: 10,
				TrackBy: "src",
			},
			Tags: []string{"scan", "recon"},
		},
		// SYN Flood 检测
		{
			ID:       "rule-dos-001",
			Name:     "SYN Flood Detection",
			Enabled:  true,
			Priority: 1,
			Severity: SeverityCritical,
			Action:   ActionBlock,
			Type:     DetectionAnomaly,
			Protocol: ProtoTCP,
			Threshold: &ThresholdConfig{
				Count:   100,
				Seconds: 5,
				TrackBy: "src",
			},
			Tags: []string{"dos", "flood"},
		},
		// SSH 暴力破解
		{
			ID:       "rule-bruteforce-001",
			Name:     "SSH Brute Force",
			Enabled:  true,
			Priority: 2,
			Severity: SeverityHigh,
			Action:   ActionBlock,
			Type:     DetectionAnomaly,
			Protocol: ProtoTCP,
			DstPort:  "22",
			Threshold: &ThresholdConfig{
				Count:   5,
				Seconds: 60,
				TrackBy: "src",
			},
			Tags: []string{"bruteforce", "ssh"},
		},
		// DNS 隧道检测
		{
			ID:       "rule-dns-001",
			Name:     "DNS Tunnel - Long Query",
			Enabled:  true,
			Priority: 3,
			Severity: SeverityMedium,
			Action:   ActionAlert,
			Type:     DetectionSignature,
			Protocol: ProtoUDP,
			DstPort:  "53",
			Content:  "dns_query_len:>100",
			Tags:     []string{"dns", "tunnel", "exfiltration"},
		},
		// ICMP 隧道检测
		{
			ID:       "rule-icmp-001",
			Name:     "ICMP Tunnel - Oversized Payload",
			Enabled:  true,
			Priority: 3,
			Severity: SeverityMedium,
			Action:   ActionAlert,
			Type:     DetectionAnomaly,
			Protocol: ProtoICMP,
			Threshold: &ThresholdConfig{
				Count:   50,
				Seconds: 60,
				TrackBy: "src",
			},
			Tags: []string{"icmp", "tunnel", "exfiltration"},
		},
	}
}

// MatchRule 检查数据包是否匹配规则.
func MatchRule(rule *Rule, pkt *PacketInfo) bool {
	if !rule.Enabled {
		return false
	}

	// 协议匹配
	if rule.Protocol != ProtoAny && rule.Protocol != pkt.Protocol {
		return false
	}

	// 源 IP 匹配
	if rule.SrcIP != "" && !matchIP(rule.SrcIP, pkt.SrcIP) {
		return false
	}

	// 目标 IP 匹配
	if rule.DstIP != "" && !matchIP(rule.DstIP, pkt.DstIP) {
		return false
	}

	// 源端口匹配
	if rule.SrcPort != "" && !matchPort(rule.SrcPort, pkt.SrcPort) {
		return false
	}

	// 目标端口匹配
	if rule.DstPort != "" && !matchPort(rule.DstPort, pkt.DstPort) {
		return false
	}

	// 签名模式匹配
	if rule.Pattern != "" && rule.Type == DetectionSignature {
		if len(pkt.Payload) == 0 {
			return false
		}
		matched, err := regexp.Match(rule.Pattern, pkt.Payload)
		if err != nil || !matched {
			return false
		}
	}

	// 内容匹配
	if rule.Content != "" && rule.Type == DetectionSignature {
		if !matchContent(rule.Content, pkt) {
			return false
		}
	}

	return true
}

// matchIP 匹配 IP（支持单 IP 和 CIDR）.
func matchIP(pattern string, ip net.IP) bool {
	// 尝试 CIDR
	if strings.Contains(pattern, "/") {
		_, cidr, err := net.ParseCIDR(pattern)
		if err != nil {
			return false
		}
		return cidr.Contains(ip)
	}
	// 单 IP
	target := net.ParseIP(pattern)
	if target == nil {
		return false
	}
	return target.Equal(ip)
}

// matchPort 匹配端口（支持单端口和范围）.
func matchPort(pattern string, port int) bool {
	if strings.Contains(pattern, "-") {
		parts := strings.SplitN(pattern, "-", 2)
		if len(parts) != 2 {
			return false
		}
		var low, high int
		if _, err := fmt.Sscanf(parts[0], "%d", &low); err != nil {
			return false
		}
		if _, err := fmt.Sscanf(parts[1], "%d", &high); err != nil {
			return false
		}
		return port >= low && port <= high
	}
	var target int
	if _, err := fmt.Sscanf(pattern, "%d", &target); err != nil {
		return false
	}
	return port == target
}

// matchContent 匹配内容规则.
func matchContent(content string, pkt *PacketInfo) bool {
	// DNS 查询长度检测
	if content == "dns_query_len:>100" {
		if pkt.Protocol == ProtoUDP && pkt.DstPort == 53 {
			return len(pkt.Payload) > 100
		}
		return false
	}
	// 关键字匹配
	return strings.Contains(strings.ToLower(string(pkt.Payload)), strings.ToLower(content))
}

// ValidateRule 验证规则有效性.
func ValidateRule(rule *Rule) error {
	if rule.ID == "" {
		return ErrInvalidRule
	}
	if rule.Name == "" {
		return ErrInvalidRule
	}
	if rule.Threshold != nil {
		if rule.Threshold.Count <= 0 || rule.Threshold.Seconds <= 0 {
			return ErrInvalidThreshold
		}
		if rule.Threshold.TrackBy != "src" && rule.Threshold.TrackBy != "dst" && rule.Threshold.TrackBy != "both" {
			return ErrInvalidThreshold
		}
	}
	// 验证 CIDR
	if rule.SrcIP != "" {
		if strings.Contains(rule.SrcIP, "/") {
			if _, _, err := net.ParseCIDR(rule.SrcIP); err != nil {
				return ErrInvalidCIDR
			}
		} else if net.ParseIP(rule.SrcIP) == nil {
			return ErrInvalidCIDR
		}
	}
	if rule.DstIP != "" {
		if strings.Contains(rule.DstIP, "/") {
			if _, _, err := net.ParseCIDR(rule.DstIP); err != nil {
				return ErrInvalidCIDR
			}
		} else if net.ParseIP(rule.DstIP) == nil {
			return ErrInvalidCIDR
		}
	}
	return nil
}

// LoadDefaultRules 加载默认规则到管理器.
func (m *Manager) LoadDefaultRules() {
	for _, rule := range DefaultRules() {
		rule.CreatedAt = time.Now()
		rule.UpdatedAt = time.Now()
		m.rules[rule.ID] = rule
	}
}
