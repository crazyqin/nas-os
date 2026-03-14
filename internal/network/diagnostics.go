package network

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

// DiagnosticResult 诊断结果
type DiagnosticResult struct {
	Success bool                   `json:"success"`
	Output  string                 `json:"output"`
	Error   string                 `json:"error,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// PingOptions Ping 选项
type PingOptions struct {
	Count    int    `json:"count"`    // 发送次数
	Interval int    `json:"interval"` // 间隔（毫秒）
	Timeout  int    `json:"timeout"`  // 超时（毫秒")
	Size     int    `json:"size"`     // 包大小
}

// PingResult Ping 结果
type PingResult struct {
	Host         string  `json:"host"`
	PacketsSent  int     `json:"packetsSent"`
	PacketsRecv  int     `json:"packetsRecv"`
	PacketLoss   float64 `json:"packetLoss"`
	MinRTT       float64 `json:"minRtt"`   // 毫秒
	MaxRTT       float64 `json:"maxRtt"`   // 毫秒
	AvgRTT       float64 `json:"avgRtt"`   // 毫秒
	StdDevRTT    float64 `json:"stdDevRtt"` // 毫秒
	Output       string  `json:"output"`
}

// TracerouteHop 路由跳
type TracerouteHop struct {
	Hop     int      `json:"hop"`
	Host    string   `json:"host"`
	IP      string   `json:"ip"`
	RTT1    float64  `json:"rtt1"`    // 毫秒
	RTT2    float64  `json:"rtt2"`    // 毫秒
	RTT3    float64  `json:"rtt3"`    // 毫秒
	Timeout bool     `json:"timeout"` // 是否超时
}

// TracerouteResult Traceroute 结果
type TracerouteResult struct {
	Host     string          `json:"host"`
	Hops     []TracerouteHop `json:"hops"`
	Output   string          `json:"output"`
	Complete bool            `json:"complete"`
}

// DNSLookupResult DNS 查询结果
type DNSLookupResult struct {
	Host       string   `json:"host"`
	Addresses  []string `json:"addresses"`
	CNAME      string   `json:"cname,omitempty"`
	MXRecords  []MXRecord `json:"mxRecords,omitempty"`
	NSRecords  []NSRecord `json:"nsRecords,omitempty"`
	TXTRecords []string  `json:"txtRecords,omitempty"`
	QueryTime  int64     `json:"queryTime"` // 纳秒
}

// MXRecord MX 记录
type MXRecord struct {
	Preference int    `json:"preference"`
	Host       string `json:"host"`
}

// NSRecord NS 记录
type NSRecord struct {
	Host string `json:"host"`
}

// PortScanResult 端口扫描结果
type PortScanResult struct {
	Host    string        `json:"host"`
	Ports   []PortStatus  `json:"ports"`
	ScanTime int64        `json:"scanTime"` // 毫秒
}

// PortStatus 端口状态
type PortStatus struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // tcp, udp
	Open     bool   `json:"open"`
	Service  string `json:"service,omitempty"`
}

// Ping 执行 Ping 测试
func (m *Manager) Ping(host string, opts PingOptions) (*PingResult, error) {
	if opts.Count == 0 {
		opts.Count = 4
	}
	if opts.Timeout == 0 {
		opts.Timeout = 1000
	}

	args := []string{"-c", fmt.Sprintf("%d", opts.Count)}

	if opts.Interval > 0 {
		args = append(args, "-i", fmt.Sprintf("%.1f", float64(opts.Interval)/1000))
	}

	if opts.Timeout > 0 {
		args = append(args, "-W", fmt.Sprintf("%.1f", float64(opts.Timeout)/1000))
	}

	if opts.Size > 0 {
		args = append(args, "-s", fmt.Sprintf("%d", opts.Size))
	}

	args = append(args, host)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(opts.Count*opts.Timeout+5000)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ping", args...)
	output, err := cmd.CombinedOutput()

	result := &PingResult{
		Host:   host,
		Output: string(output),
	}

	if err != nil {
		return result, fmt.Errorf("ping 执行失败: %w", err)
	}

	// 解析输出
	m.parsePingOutput(string(output), result)

	return result, nil
}

// parsePingOutput 解析 ping 输出
func (m *Manager) parsePingOutput(output string, result *PingResult) {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		// 解析统计行: "4 packets transmitted, 4 received, 0% packet loss"
		if strings.Contains(line, "packets transmitted") {
			fmt.Sscanf(line, "%d packets transmitted, %d received",
				&result.PacketsSent, &result.PacketsRecv)
			if result.PacketsSent > 0 {
				result.PacketLoss = float64(result.PacketsSent-result.PacketsRecv) / float64(result.PacketsSent) * 100
			}
		}

		// 解析 RTT 行: "rtt min/avg/max/mdev = 0.123/0.456/0.789/0.123 ms"
		if strings.Contains(line, "rtt min/avg/max/mdev") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				values := strings.TrimSpace(parts[1])
				values = strings.TrimSuffix(values, " ms")
				rttParts := strings.Split(values, "/")
				if len(rttParts) >= 4 {
					fmt.Sscanf(rttParts[0], "%f", &result.MinRTT)
					fmt.Sscanf(rttParts[1], "%f", &result.AvgRTT)
					fmt.Sscanf(rttParts[2], "%f", &result.MaxRTT)
					fmt.Sscanf(rttParts[3], "%f", &result.StdDevRTT)
				}
			}
		}
	}
}

// Traceroute 执行路由追踪
func (m *Manager) Traceroute(host string, maxHops int) (*TracerouteResult, error) {
	if maxHops == 0 {
		maxHops = 30
	}

	args := []string{"-m", fmt.Sprintf("%d", maxHops), host}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "traceroute", args...)
	output, err := cmd.CombinedOutput()

	result := &TracerouteResult{
		Host:   host,
		Output: string(output),
		Hops:   make([]TracerouteHop, 0),
	}

	if err != nil {
		// 尝试使用 tracepath 作为备选
		cmd = exec.CommandContext(ctx, "tracepath", host)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return result, fmt.Errorf("traceroute 执行失败: %w", err)
		}
		result.Output = string(output)
		m.parseTracepathOutput(string(output), result)
		return result, nil
	}

	m.parseTracerouteOutput(string(output), result)
	return result, nil
}

// parseTracerouteOutput 解析 traceroute 输出
func (m *Manager) parseTracerouteOutput(output string, result *TracerouteResult) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	hopNum := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过标题行
		if strings.Contains(line, "traceroute to") || line == "" {
			continue
		}

		hopNum++
		hop := TracerouteHop{Hop: hopNum}

		// 解析行: " 1  192.168.1.1 (192.168.1.1)  0.123 ms  0.456 ms  0.789 ms"
		// 或: " 1  * * *"
		if strings.Contains(line, "* * *") {
			hop.Timeout = true
		} else {
			parts := strings.Fields(line)
			if len(parts) >= 5 {
				// 跳过序号
				i := 0
				for i < len(parts) && parts[i] != fmt.Sprintf("%d", hopNum) {
					i++
				}
				if i < len(parts) {
					i++ // 跳过序号
				}

				if i < len(parts) {
					hop.Host = parts[i]
					// 检查是否有括号内的 IP
					if i+1 < len(parts) && strings.HasPrefix(parts[i+1], "(") {
						hop.IP = strings.Trim(parts[i+1], "()")
						i += 2
					} else {
						hop.IP = parts[i]
						i++
					}
				}

				// 解析 RTT 值
				var rtts []float64
				for i < len(parts) {
					if parts[i] == "*" {
						rtts = append(rtts, 0)
						i++
						if i < len(parts) && parts[i] == "ms" {
							i++
						}
					} else {
						var rtt float64
						fmt.Sscanf(parts[i], "%f", &rtt)
						rtts = append(rtts, rtt)
						i++
						if i < len(parts) && parts[i] == "ms" {
							i++
						}
					}
				}

				if len(rtts) >= 1 {
					hop.RTT1 = rtts[0]
				}
				if len(rtts) >= 2 {
					hop.RTT2 = rtts[1]
				}
				if len(rtts) >= 3 {
					hop.RTT3 = rtts[2]
				}
			}
		}

		result.Hops = append(result.Hops, hop)

		// 检查是否到达目标
		if hop.Host == result.Host || hop.IP == result.Host {
			result.Complete = true
		}
	}
}

// parseTracepathOutput 解析 tracepath 输出
func (m *Manager) parseTracepathOutput(output string, result *TracerouteResult) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	hopNum := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.Contains(line, "tracepath to") || line == "" {
			continue
		}

		hopNum++
		hop := TracerouteHop{Hop: hopNum}

		if strings.Contains(line, "no reply") {
			hop.Timeout = true
		} else {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// 格式: " 1?: [LOCALHOST]    0.123ms pmtu 1500"
				// 或: " 1  192.168.1.1    0.456ms"
				hostPart := parts[1]
				if hostPart == "[LOCALHOST]" {
					hop.Host = "localhost"
					hop.IP = "127.0.0.1"
				} else {
					hop.Host = hostPart
					hop.IP = hostPart
				}

				// 查找 RTT
				for i := 2; i < len(parts); i++ {
					if strings.HasSuffix(parts[i], "ms") {
						var rtt float64
						fmt.Sscanf(parts[i], "%fms", &rtt)
						hop.RTT1 = rtt
						break
					}
				}
			}
		}

		result.Hops = append(result.Hops, hop)
	}
}

// DNSLookup 执行 DNS 查询
func (m *Manager) DNSLookup(host string, recordType string) (*DNSLookupResult, error) {
	start := time.Now()

	result := &DNSLookupResult{
		Host:      host,
		Addresses: make([]string, 0),
	}

	// A/AAAA 记录
	addrs, err := net.LookupHost(host)
	if err == nil {
		result.Addresses = addrs
	}

	// CNAME 记录
	cname, err := net.LookupCNAME(host)
	if err == nil && cname != host && cname != "" {
		result.CNAME = cname
	}

	// MX 记录
	if recordType == "" || recordType == "MX" {
		mxRecords, err := net.LookupMX(host)
		if err == nil {
			for _, mx := range mxRecords {
				result.MXRecords = append(result.MXRecords, MXRecord{
					Preference: int(mx.Pref),
					Host:       mx.Host,
				})
			}
		}
	}

	// NS 记录
	if recordType == "" || recordType == "NS" {
		nsRecords, err := net.LookupNS(host)
		if err == nil {
			for _, ns := range nsRecords {
				result.NSRecords = append(result.NSRecords, NSRecord{
					Host: ns.Host,
				})
			}
		}
	}

	// TXT 记录
	if recordType == "" || recordType == "TXT" {
		txtRecords, err := net.LookupTXT(host)
		if err == nil {
			result.TXTRecords = txtRecords
		}
	}

	result.QueryTime = time.Since(start).Nanoseconds()

	return result, nil
}

// PortScan 端口扫描
func (m *Manager) PortScan(host string, ports []int, protocol string) (*PortScanResult, error) {
	start := time.Now()

	if protocol == "" {
		protocol = "tcp"
	}

	result := &PortScanResult{
		Host:  host,
		Ports: make([]PortStatus, 0),
	}

	timeout := 1 * time.Second

	for _, port := range ports {
		status := PortStatus{
			Port:     port,
			Protocol: protocol,
		}

		address := fmt.Sprintf("%s:%d", host, port)

		if protocol == "tcp" {
			conn, err := net.DialTimeout("tcp", address, timeout)
			if err == nil {
				status.Open = true
				conn.Close()
				// 尝试识别服务
				status.Service = m.identifyService(port)
			}
		} else {
			// UDP 扫描
			conn, err := net.DialTimeout("udp", address, timeout)
			if err == nil {
				// 发送空数据包
				conn.Write([]byte{})
				conn.SetReadDeadline(time.Now().Add(timeout))
				buf := make([]byte, 1024)
				_, err := conn.Read(buf)
				if err == nil {
					status.Open = true
				}
				conn.Close()
			}
		}

		result.Ports = append(result.Ports, status)
	}

	result.ScanTime = time.Since(start).Milliseconds()

	return result, nil
}

// identifyService 识别端口对应的服务
func (m *Manager) identifyService(port int) string {
	services := map[int]string{
		20:   "FTP Data",
		21:   "FTP",
		22:   "SSH",
		23:   "Telnet",
		25:   "SMTP",
		53:   "DNS",
		80:   "HTTP",
		110:  "POP3",
		143:  "IMAP",
		443:  "HTTPS",
		465:  "SMTPS",
		587:  "SMTP (TLS)",
		993:  "IMAPS",
		995:  "POP3S",
		3306: "MySQL",
		5432: "PostgreSQL",
		6379: "Redis",
		8080: "HTTP Proxy",
		8443: "HTTPS (Alt)",
		9000: "PHP-FPM",
		27017: "MongoDB",
	}

	if service, ok := services[port]; ok {
		return service
	}
	return ""
}

// NetworkDiagnose 综合网络诊断
func (m *Manager) NetworkDiagnose(host string) (map[string]interface{}, error) {
	results := make(map[string]interface{})

	// DNS 解析
	dnsResult, err := m.DNSLookup(host, "")
	if err == nil {
		results["dns"] = dnsResult
	} else {
		results["dnsError"] = err.Error()
	}

	// Ping 测试
	pingResult, err := m.Ping(host, PingOptions{Count: 4})
	if err == nil {
		results["ping"] = pingResult
	} else {
		results["pingError"] = err.Error()
	}

	// 常用端口扫描
	commonPorts := []int{22, 80, 443, 3306, 5432, 6379}
	portResult, err := m.PortScan(host, commonPorts, "tcp")
	if err == nil {
		results["ports"] = portResult
	} else {
		results["portsError"] = err.Error()
	}

	return results, nil
}

// Whois 查询域名 Whois 信息
func (m *Manager) Whois(domain string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "whois", domain)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("whois 查询失败: %w", err)
	}

	return string(output), nil
}

// Nslookup 执行 nslookup 命令
func (m *Manager) Nslookup(host string, server string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := []string{host}
	if server != "" {
		args = append(args, server)
	}

	cmd := exec.CommandContext(ctx, "nslookup", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("nslookup 执行失败: %w", err)
	}

	return string(output), nil
}

// Dig 执行 DNS 查询（使用 dig 命令）
func (m *Manager) Dig(host string, recordType string, server string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := []string{}
	if server != "" {
		args = append(args, "@"+server)
	}
	args = append(args, host)
	if recordType != "" {
		args = append(args, recordType)
	}

	cmd := exec.CommandContext(ctx, "dig", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("dig 执行失败: %w", err)
	}

	return string(output), nil
}

// ParseDigOutput 解析 dig 输出获取 IP 列表
func (m *Manager) ParseDigOutput(output string) []string {
	var ips []string
	scanner := bufio.NewScanner(strings.NewReader(output))

	inAnswer := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, ";; ANSWER SECTION:") {
			inAnswer = true
			continue
		}

		if strings.HasPrefix(line, ";;") {
			inAnswer = false
			continue
		}

		if inAnswer && line != "" {
			// 格式: "example.com.  300  IN  A  93.184.216.34"
			parts := strings.Fields(line)
			if len(parts) >= 5 {
				ip := parts[len(parts)-1]
				// 验证是否为有效 IP
				if net.ParseIP(ip) != nil {
					ips = append(ips, ip)
				}
			}
		}
	}

	return ips
}

// CheckConnectivity 检查网络连接状态
func (m *Manager) CheckConnectivity() (*ConnectivityStatus, error) {
	status := &ConnectivityStatus{
		Checks: make(map[string]bool),
	}

	// 检查 DNS 解析
	if _, err := net.LookupHost("google.com"); err == nil {
		status.Checks["dns"] = true
	} else {
		status.Checks["dns"] = false
	}

	// 检查外网连接
	testHosts := []string{
		"8.8.8.8:53",     // Google DNS
		"1.1.1.1:53",     // Cloudflare DNS
		"208.67.222.222:53", // OpenDNS
	}

	for _, host := range testHosts {
		conn, err := net.DialTimeout("tcp", host, 2*time.Second)
		if err == nil {
			conn.Close()
			status.Checks["internet"] = true
			break
		}
	}

	// 检查网关
	if gateway := m.getDefaultGateway(""); gateway != "" {
		// 尝试 ping 网关
		_, err := m.Ping(gateway, PingOptions{Count: 1, Timeout: 2000})
		status.Checks["gateway"] = err == nil
	}

	// 计算总体状态
	status.Connected = status.Checks["internet"]

	return status, nil
}

// ConnectivityStatus 连接状态
type ConnectivityStatus struct {
	Connected bool              `json:"connected"`
	Checks    map[string]bool   `json:"checks"`
}

// Netstat 获取网络连接状态
func (m *Manager) Netstat() (string, error) {
	cmd := exec.Command("netstat", "-tulpn")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 尝试 ss 命令
		cmd = exec.Command("ss", "-tulpn")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("获取网络状态失败: %w", err)
		}
	}
	return string(output), nil
}

// ARPTable 获取 ARP 表
func (m *Manager) ARPTable() ([]ARPEntry, error) {
	cmd := exec.Command("arp", "-n")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取 ARP 表失败: %w", err)
	}

	var entries []ARPEntry
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "Address") || strings.HasPrefix(line, "IP") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 3 {
			entry := ARPEntry{
				IP:        parts[0],
				HWAddress: parts[2],
			}
			if len(parts) >= 5 {
				entry.Interface = parts[4]
			}
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

// ARPEntry ARP 表条目
type ARPEntry struct {
	IP        string `json:"ip"`
	HWAddress string `json:"hwAddress"`
	Interface string `json:"interface"`
}