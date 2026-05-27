// Package assetmgr 提供IT资产管理功能
package assetmgr

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// Scanner 网络扫描器.
type Scanner struct {
	mu       sync.Mutex
	scanning bool
	results  []*ScanResult
}

// NewScanner 创建网络扫描器.
func NewScanner() *Scanner {
	return &Scanner{
		results: make([]*ScanResult, 0),
	}
}

// Scan 执行网络扫描.
func (s *Scanner) Scan(scanRange string) (*ScanResult, error) {
	s.mu.Lock()
	if s.scanning {
		s.mu.Unlock()
		return nil, ErrScanInProgress
	}
	s.scanning = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.scanning = false
		s.mu.Unlock()
	}()

	result := &ScanResult{
		ID:        fmt.Sprintf("scan-%d", time.Now().Unix()),
		ScanRange: scanRange,
		StartTime: time.Now(),
		Status:    "running",
	}

	// 解析扫描范围
	ips, err := parseIPRange(scanRange)
	if err != nil {
		result.Status = "failed"
		result.EndTime = time.Now()
		return result, err
	}

	assets := make([]*Asset, 0)
	for _, ip := range ips {
		asset := probeHost(ip)
		if asset != nil {
			assets = append(assets, asset)
		}
	}

	result.Assets = assets
	result.TotalFound = len(assets)
	result.Status = "completed"
	result.EndTime = time.Now()

	s.results = append(s.results, result)
	return result, nil
}

// GetScanResults 获取历史扫描结果.
func (s *Scanner) GetScanResults() []*ScanResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.results
}

// probeHost 探测主机（简化实现，实际应使用SNMP/WMI）.
func probeHost(ip string) *Asset {
	// 尝试TCP连接常用端口
	ports := []int{22, 80, 443, 161, 445}
	openPorts := make([]int, 0)
	for _, port := range ports {
		addr := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			openPorts = append(openPorts, port)
		}
	}

	if len(openPorts) == 0 {
		return nil
	}

	// 根据开放端口推测设备类型
	assetType := guessAssetType(openPorts)

	return &Asset{
		ID:         fmt.Sprintf("asset-%s", ip),
		Name:       fmt.Sprintf("设备-%s", ip),
		Type:       assetType,
		Status:     StatusOnline,
		IPAddress:  ip,
		Tags:       make(map[string]string),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// guessAssetType 根据端口推测设备类型.
func guessAssetType(ports []int) AssetType {
	portMap := make(map[int]bool)
	for _, p := range ports {
		portMap[p] = true
	}

	// SNMP (161) 通常是网络设备
	if portMap[161] && !portMap[22] {
		return TypeSwitch
	}
	// SMB (445) + SSH 通常是服务器
	if portMap[445] && portMap[22] {
		return TypeServer
	}
	// 只有SSH通常是服务器
	if portMap[22] {
		return TypeServer
	}
	// 只有HTTP/S通常是路由器或防火墙
	if portMap[80] || portMap[443] {
		return TypeRouter
	}
	return TypeOther
}

// parseIPRange 解析IP范围，支持 CIDR 和起止格式.
func parseIPRange(r string) ([]string, error) {
	// CIDR: 192.168.1.0/24
	if _, ipNet, err := net.ParseCIDR(r); err == nil {
		ips := make([]string, 0)
		for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); inc(ip) {
			ips = append(ips, ip.String())
		}
		// 去掉网络地址和广播地址
		if len(ips) > 2 {
			ips = ips[1 : len(ips)-1]
		}
		return ips, nil
	}

	// 单个IP
	if net.ParseIP(r) != nil {
		return []string{r}, nil
	}

	// 范围: 192.168.1.1-192.168.1.10
	if len(r) > 0 {
		// 尝试简单范围解析
		for i := 0; i < len(r); i++ {
			if r[i] == '-' {
				start := net.ParseIP(r[:i])
				end := net.ParseIP(r[i+1:])
				if start != nil && end != nil {
					return expandIPRange(start, end), nil
				}
			}
		}
	}

	return nil, ErrInvalidInput
}

// inc 递增IP地址.
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// expandIPRange 展开IP范围.
func expandIPRange(start, end net.IP) []string {
	ips := make([]string, 0)
	ip := make(net.IP, len(start))
	copy(ip, start.To4())
	endIP := end.To4()

	for {
		ips = append(ips, ip.String())
		if ip.Equal(endIP) {
			break
		}
		inc(ip)
		// 安全限制：最多1024个IP
		if len(ips) > 1024 {
			break
		}
	}
	return ips
}

// SNMPProbe SNMP探测（预留接口）.
func SNMPProbe(ip, community string) (*Asset, error) {
	// 实际实现应使用 gosnmp 库
	// 此处为框架代码
	return &Asset{
		ID:        fmt.Sprintf("snmp-%s", ip),
		Name:      fmt.Sprintf("SNMP设备-%s", ip),
		Type:      TypeSwitch,
		Status:    StatusOnline,
		IPAddress: ip,
		Tags:      make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// WMIProbe WMI探测（预留接口）.
func WMIProbe(ip, user, pass string) (*Asset, error) {
	// 实际实现应使用 WMI 客户端库
	// 此处为框架代码
	return &Asset{
		ID:        fmt.Sprintf("wmi-%s", ip),
		Name:      fmt.Sprintf("WMI设备-%s", ip),
		Type:      TypeServer,
		Status:    StatusOnline,
		IPAddress: ip,
		Hardware: &HardwareInfo{
			CPUModel: "Unknown",
			CPUCores: 4,
			RAMGB:    16,
		},
		Tags:      make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}
