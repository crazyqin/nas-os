package netscan

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Discoverer 设备发现器.
type Discoverer struct {
	config DiscoveryConfig
}

// NewDiscoverer 创建设备发现器.
func NewDiscoverer(config DiscoveryConfig) *Discoverer {
	if config.Timeout == 0 {
		config.Timeout = 3 * time.Second
	}
	if config.Concurrent == 0 {
		config.Concurrent = 50
	}
	return &Discoverer{config: config}
}

// Discover 发现网络设备.
func (d *Discoverer) Discover(ctx context.Context) ([]Device, error) {
	ips, err := d.expandNetwork(d.config.Network)
	if err != nil {
		return nil, fmt.Errorf("解析网段失败：%w", err)
	}

	var devices []Device
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 并发控制
	sem := make(chan struct{}, d.config.Concurrent)

	for _, ip := range ips {
		select {
		case <-ctx.Done():
			return devices, ctx.Err()
		default:
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(ip string) {
			defer wg.Done()
			defer func() { <-sem }()

			var device *Device

			if d.config.UseARP {
				device = d.arpProbe(ip)
			}

			if device == nil && d.config.UseICMP {
				device = d.icmpPing(ip)
			}

			if device != nil {
				mu.Lock()
				devices = append(devices, *device)
				mu.Unlock()
			}
		}(ip)
	}

	wg.Wait()
	return devices, nil
}

// arpProbe ARP 探测.
func (d *Discoverer) arpProbe(ip string) *Device {
	ctx, cancel := context.WithTimeout(context.Background(), d.config.Timeout)
	defer cancel()

	// 使用 arping 或 arp 命令
	cmd := exec.CommandContext(ctx, "arping", "-c", "1", "-w", "1", ip)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	mac := parseMACFromArping(string(output))
	if mac == "" {
		return nil
	}

	hostname := resolveHostname(ip)

	return &Device{
		IP:       ip,
		MAC:      mac,
		Hostname: hostname,
		State:    DeviceStateOnline,
		LastSeen: time.Now(),
	}
}

// icmpPing ICMP Ping.
func (d *Discoverer) icmpPing(ip string) *Device {
	ctx, cancel := context.WithTimeout(context.Background(), d.config.Timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", "1", ip)
	err := cmd.Run()
	rtt := time.Since(start)

	if err != nil {
		return nil
	}

	hostname := resolveHostname(ip)

	return &Device{
		IP:       ip,
		Hostname: hostname,
		State:    DeviceStateOnline,
		RTT:      rtt,
		LastSeen: time.Now(),
	}
}

// expandNetwork 展开 CIDR 网段.
func (d *Discoverer) expandNetwork(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}

	// 去掉网络地址和广播地址
	if len(ips) > 2 {
		ips = ips[1 : len(ips)-1]
	}

	return ips, nil
}

// inc IP 地址递增.
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// parseMACFromArping 从 arping 输出解析 MAC 地址.
func parseMACFromArping(output string) string {
	re := regexp.MustCompile(`(?i)([0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2})`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		return strings.ToLower(matches[1])
	}
	return ""
}

// resolveHostname 解析主机名.
func resolveHostname(ip string) string {
	names, err := net.LookupAddr(ip)
	if err != nil || len(names) == 0 {
		return ""
	}
	return strings.TrimSuffix(names[0], ".")
}

// ARPScan ARP 扫描.
func ARPScan(ctx context.Context, network string) ([]Device, error) {
	config := DiscoveryConfig{
		Network:    network,
		Methods:    []string{"arp"},
		Timeout:    3 * time.Second,
		Concurrent: 50,
		UseARP:     true,
	}
	d := NewDiscoverer(config)
	return d.Discover(ctx)
}

// ICMPPingScan ICMP 扫描.
func ICMPPingScan(ctx context.Context, network string) ([]Device, error) {
	config := DiscoveryConfig{
		Network:    network,
		Methods:    []string{"ping"},
		Timeout:    3 * time.Second,
		Concurrent: 50,
		UseICMP:    true,
	}
	d := NewDiscoverer(config)
	return d.Discover(ctx)
}

// GetARPTable 获取系统 ARP 表.
func GetARPTable() (map[string]string, error) {
	cmd := exec.Command("arp", "-an")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	table := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	re := regexp.MustCompile(`\((\d+\.\d+\.\d+\.\d+)\)\s+at\s+([0-9a-fA-F:]+)`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if len(matches) >= 3 {
			table[matches[1]] = strings.ToLower(matches[2])
		}
	}

	return table, nil
}
