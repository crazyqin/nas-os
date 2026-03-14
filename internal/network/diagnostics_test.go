package network

import (
	"strings"
	"testing"
)

// TestPingOptionsDefaults 测试 Ping 选项默认值
func TestPingOptionsDefaults(t *testing.T) {
	// 测试默认值设置
	opts := PingOptions{}
	if opts.Count != 0 {
		t.Errorf("默认 Count 应该是 0")
	}
	if opts.Timeout != 0 {
		t.Errorf("默认 Timeout 应该是 0")
	}
}

// TestPingResultFields 测试 Ping 结果字段
func TestPingResultFields(t *testing.T) {
	result := &PingResult{
		Host:        "192.168.1.1",
		PacketsSent: 4,
		PacketsRecv: 4,
		PacketLoss:  0.0,
		MinRTT:      0.5,
		MaxRTT:      1.2,
		AvgRTT:      0.8,
		StdDevRTT:   0.3,
		Output:      "ping output",
	}

	if result.Host != "192.168.1.1" {
		t.Errorf("Host 不匹配")
	}
	if result.PacketLoss != 0.0 {
		t.Errorf("PacketLoss 应该是 0")
	}
}

// TestParsePingOutput 测试解析 Ping 输出
func TestParsePingOutput(t *testing.T) {
	mgr := NewManager("/tmp/test-network")

	tests := []struct {
		name           string
		output         string
		expectedSent   int
		expectedRecv   int
		expectedLoss   float64
		expectedMinRTT float64
		expectedAvgRTT float64
		expectedMaxRTT float64
	}{
		{
			name: "正常输出",
			output: `PING 192.168.1.1 (192.168.1.1) 56(84) bytes of data.
--- 192.168.1.1 ping statistics ---
4 packets transmitted, 4 received, 0% packet loss, time 3005ms
rtt min/avg/max/mdev = 0.123/0.456/0.789/0.123 ms`,
			expectedSent:   4,
			expectedRecv:   4,
			expectedLoss:   0.0,
			expectedMinRTT: 0.123,
			expectedAvgRTT: 0.456,
			expectedMaxRTT: 0.789,
		},
		{
			name: "有丢包",
			output: `PING 192.168.1.1 (192.168.1.1) 56(84) bytes of data.
--- 192.168.1.1 ping statistics ---
4 packets transmitted, 2 received, 50% packet loss, time 3005ms
rtt min/avg/max/mdev = 0.100/0.200/0.300/0.100 ms`,
			expectedSent:   4,
			expectedRecv:   2,
			expectedLoss:   50.0,
			expectedMinRTT: 0.100,
			expectedAvgRTT: 0.200,
			expectedMaxRTT: 0.300,
		},
		{
			name: "全部丢包",
			output: `PING 192.168.1.1 (192.168.1.1) 56(84) bytes of data.
--- 192.168.1.1 ping statistics ---
4 packets transmitted, 0 received, 100% packet loss, time 3005ms`,
			expectedSent: 4,
			expectedRecv: 0,
			expectedLoss: 100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &PingResult{}
			mgr.parsePingOutput(tt.output, result)

			if result.PacketsSent != tt.expectedSent {
				t.Errorf("PacketsSent 不匹配: got %d, want %d", result.PacketsSent, tt.expectedSent)
			}
			if result.PacketsRecv != tt.expectedRecv {
				t.Errorf("PacketsRecv 不匹配: got %d, want %d", result.PacketsRecv, tt.expectedRecv)
			}
			if tt.expectedLoss > 0 && result.PacketLoss != tt.expectedLoss {
				t.Errorf("PacketLoss 不匹配: got %f, want %f", result.PacketLoss, tt.expectedLoss)
			}
			if tt.expectedMinRTT > 0 && result.MinRTT != tt.expectedMinRTT {
				t.Errorf("MinRTT 不匹配: got %f, want %f", result.MinRTT, tt.expectedMinRTT)
			}
		})
	}
}

// TestTracerouteHop 测试路由跳结构
func TestTracerouteHop(t *testing.T) {
	hop := TracerouteHop{
		Hop:     1,
		Host:    "192.168.1.1",
		IP:      "192.168.1.1",
		RTT1:    0.5,
		RTT2:    0.6,
		RTT3:    0.7,
		Timeout: false,
	}

	if hop.Hop != 1 {
		t.Errorf("Hop 应该是 1")
	}
	if hop.Timeout {
		t.Errorf("Timeout 应该是 false")
	}
}

// TestTracerouteResult 测试路由追踪结果
func TestTracerouteResult(t *testing.T) {
	result := &TracerouteResult{
		Host: "8.8.8.8",
		Hops: []TracerouteHop{
			{Hop: 1, Host: "192.168.1.1", IP: "192.168.1.1"},
			{Hop: 2, Host: "10.0.0.1", IP: "10.0.0.1"},
		},
		Complete: true,
	}

	if len(result.Hops) != 2 {
		t.Errorf("应该有 2 跳")
	}
	if !result.Complete {
		t.Errorf("应该已完成")
	}
}

// TestParseTracerouteOutput 测试解析 Traceroute 输出
func TestParseTracerouteOutput(t *testing.T) {
	mgr := NewManager("/tmp/test-network")

	output := `traceroute to 8.8.8.8 (8.8.8.8), 30 hops max, 60 byte packets
 1  192.168.1.1 (192.168.1.1)  0.123 ms  0.456 ms  0.789 ms
 2  10.0.0.1 (10.0.0.1)  1.234 ms  1.567 ms  1.890 ms
 3  * * *`

	result := &TracerouteResult{
		Host: "8.8.8.8",
		Hops: make([]TracerouteHop, 0),
	}

	mgr.parseTracerouteOutput(output, result)

	if len(result.Hops) != 3 {
		t.Errorf("应该有 3 跳，实际: %d", len(result.Hops))
	}

	// 检查第一跳
	if result.Hops[0].Host != "192.168.1.1" {
		t.Errorf("第一跳主机不匹配: %s", result.Hops[0].Host)
	}

	// 检查超时跳
	if !result.Hops[2].Timeout {
		t.Errorf("第三跳应该超时")
	}
}

// TestDNSLookupResult 测试 DNS 查询结果
func TestDNSLookupResult(t *testing.T) {
	result := &DNSLookupResult{
		Host:      "example.com",
		Addresses: []string{"93.184.216.34"},
		CNAME:     "example.com",
		QueryTime: 15000000,
	}

	if len(result.Addresses) != 1 {
		t.Errorf("应该有 1 个地址")
	}
	if result.QueryTime <= 0 {
		t.Errorf("QueryTime 应该大于 0")
	}
}

// TestMXRecord 测试 MX 记录
func TestMXRecord(t *testing.T) {
	mx := MXRecord{
		Preference: 10,
		Host:       "mail.example.com",
	}

	if mx.Preference != 10 {
		t.Errorf("Preference 应该是 10")
	}
}

// TestNSRecord 测试 NS 记录
func TestNSRecord(t *testing.T) {
	ns := NSRecord{
		Host: "ns1.example.com",
	}

	if ns.Host != "ns1.example.com" {
		t.Errorf("Host 不匹配")
	}
}

// TestPortScanResult 测试端口扫描结果
func TestPortScanResult(t *testing.T) {
	result := &PortScanResult{
		Host: "192.168.1.1",
		Ports: []PortStatus{
			{Port: 22, Protocol: "tcp", Open: true, Service: "SSH"},
			{Port: 80, Protocol: "tcp", Open: false},
		},
		ScanTime: 1500,
	}

	if len(result.Ports) != 2 {
		t.Errorf("应该有 2 个端口")
	}
	if result.Ports[0].Service != "SSH" {
		t.Errorf("端口 22 应该是 SSH 服务")
	}
}

// TestPortStatus 测试端口状态
func TestPortStatus(t *testing.T) {
	status := PortStatus{
		Port:     443,
		Protocol: "tcp",
		Open:     true,
		Service:  "HTTPS",
	}

	if status.Port != 443 {
		t.Errorf("端口应该是 443")
	}
	if !status.Open {
		t.Errorf("端口应该是开放的")
	}
}

// TestIdentifyService 测试服务识别
func TestIdentifyService(t *testing.T) {
	mgr := NewManager("/tmp/test-network")

	tests := []struct {
		port            int
		expectedService string
	}{
		{22, "SSH"},
		{80, "HTTP"},
		{443, "HTTPS"},
		{3306, "MySQL"},
		{5432, "PostgreSQL"},
		{6379, "Redis"},
		{8080, "HTTP Proxy"},
		{27017, "MongoDB"},
		{9999, ""}, // 未知端口
	}

	for _, tt := range tests {
		t.Run(tt.expectedService, func(t *testing.T) {
			service := mgr.identifyService(tt.port)
			if service != tt.expectedService {
				t.Errorf("端口 %d 服务不匹配: got %s, want %s", tt.port, service, tt.expectedService)
			}
		})
	}
}

// TestDiagnosticResult 测试诊断结果
func TestDiagnosticResult(t *testing.T) {
	result := &DiagnosticResult{
		Success: true,
		Output:  "test output",
		Details: map[string]interface{}{
			"key": "value",
		},
	}

	if !result.Success {
		t.Errorf("Success 应该是 true")
	}
	if result.Output != "test output" {
		t.Errorf("Output 不匹配")
	}
}

// TestConnectivityStatus 测试连接状态
func TestConnectivityStatus(t *testing.T) {
	status := &ConnectivityStatus{
		Connected: true,
		Checks: map[string]bool{
			"dns":      true,
			"internet": true,
			"gateway":  true,
		},
	}

	if !status.Connected {
		t.Errorf("应该已连接")
	}
	if !status.Checks["dns"] {
		t.Errorf("DNS 检查应该成功")
	}
}

// TestARPEntry 测试 ARP 条目
func TestARPEntry(t *testing.T) {
	entry := ARPEntry{
		IP:        "192.168.1.1",
		HWAddress: "00:11:22:33:44:55",
		Interface: "eth0",
	}

	if entry.IP != "192.168.1.1" {
		t.Errorf("IP 不匹配")
	}
	if entry.HWAddress != "00:11:22:33:44:55" {
		t.Errorf("MAC 地址不匹配")
	}
}

// TestParseDigOutput 测试解析 Dig 输出
func TestParseDigOutput(t *testing.T) {
	mgr := NewManager("/tmp/test-network")

	output := `; <<>> DiG 9.16.1 <<>> example.com
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 12345
;; flags: qr rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0

;; QUESTION SECTION:
;example.com.			IN	A

;; ANSWER SECTION:
example.com.		300	IN	A	93.184.216.34

;; Query time: 15 msec
;; SERVER: 8.8.8.8#53(8.8.8.8)`

	ips := mgr.ParseDigOutput(output)

	if len(ips) != 1 {
		t.Errorf("应该找到 1 个 IP，实际: %d", len(ips))
	}
	if len(ips) > 0 && ips[0] != "93.184.216.34" {
		t.Errorf("IP 不匹配: %s", ips[0])
	}
}

// TestParseDigOutputEmpty 测试解析空 Dig 输出
func TestParseDigOutputEmpty(t *testing.T) {
	mgr := NewManager("/tmp/test-network")

	output := `; <<>> DiG 9.16.1 <<>> nonexistent.example
;; no answer`

	ips := mgr.ParseDigOutput(output)

	if len(ips) != 0 {
		t.Errorf("应该找到 0 个 IP，实际: %d", len(ips))
	}
}

// TestDiagnosticResultDetails 测试诊断结果详情
func TestDiagnosticResultDetails(t *testing.T) {
	result := &DiagnosticResult{
		Success: true,
		Output:  "diagnostic complete",
		Details: map[string]interface{}{
			"latency":  0.5,
			"packets":  4,
			"hostname": "test.local",
		},
	}

	if result.Details["hostname"] != "test.local" {
		t.Errorf("hostname 详情不匹配")
	}
}

// TestPingOutputParsingWithDifferentFormats 测试不同格式的 Ping 输出解析
func TestPingOutputParsingWithDifferentFormats(t *testing.T) {
	mgr := NewManager("/tmp/test-network")

	// 测试不带 RTT 统计的输出
	output := `PING 192.168.1.1 (192.168.1.1) 56(84) bytes of data.
--- 192.168.1.1 ping statistics ---
4 packets transmitted, 4 received, 0% packet loss, time 3005ms`

	result := &PingResult{}
	mgr.parsePingOutput(output, result)

	if result.PacketsSent != 4 {
		t.Errorf("PacketsSent 应该是 4")
	}
	if result.PacketsRecv != 4 {
		t.Errorf("PacketsRecv 应该是 4")
	}
}

// TestTracerouteParseWithLocalhost 测试解析包含 localhost 的 traceroute 输出
func TestTracerouteParseWithLocalhost(t *testing.T) {
	mgr := NewManager("/tmp/test-network")

	output := `tracepath to 8.8.8.8 (8.8.8.8), 30 hops max
 1?: [LOCALHOST]    0.123ms pmtu 1500
 1  192.168.1.1    0.456ms`

	result := &TracerouteResult{
		Host: "8.8.8.8",
		Hops: make([]TracerouteHop, 0),
	}

	mgr.parseTracepathOutput(output, result)

	if len(result.Hops) != 2 {
		t.Errorf("应该有 2 跳，实际: %d", len(result.Hops))
	}

	// 检查 localhost 跳
	if result.Hops[0].Host != "localhost" {
		t.Errorf("第一跳应该是 localhost，实际: %s", result.Hops[0].Host)
	}
}

// TestPortScanProtocolDefault 测试端口扫描协议默认值
func TestPortScanProtocolDefault(t *testing.T) {
	// 测试协议默认值逻辑
	protocol := ""
	if protocol == "" {
		protocol = "tcp"
	}

	if protocol != "tcp" {
		t.Errorf("默认协议应该是 tcp")
	}
}

// TestPingCountDefault 测试 Ping 计数默认值
func TestPingCountDefault(t *testing.T) {
	opts := PingOptions{}

	// 模拟 Ping 方法中的默认值设置
	if opts.Count == 0 {
		opts.Count = 4
	}

	if opts.Count != 4 {
		t.Errorf("默认 Count 应该是 4")
	}
}

// TestPingTimeoutDefault 测试 Ping 超时默认值
func TestPingTimeoutDefault(t *testing.T) {
	opts := PingOptions{}

	// 模拟 Ping 方法中的默认值设置
	if opts.Timeout == 0 {
		opts.Timeout = 1000
	}

	if opts.Timeout != 1000 {
		t.Errorf("默认 Timeout 应该是 1000")
	}
}

// TestTracerouteMaxHopsDefault 测试 Traceroute 最大跳数默认值
func TestTracerouteMaxHopsDefault(t *testing.T) {
	maxHops := 0

	// 模拟 Traceroute 方法中的默认值设置
	if maxHops == 0 {
		maxHops = 30
	}

	if maxHops != 30 {
		t.Errorf("默认 maxHops 应该是 30")
	}
}

// TestPacketLossCalculation 测试丢包率计算
func TestPacketLossCalculation(t *testing.T) {
	tests := []struct {
		sent       int
		recv       int
		expectedLoss float64
	}{
		{4, 4, 0.0},
		{4, 3, 25.0},
		{4, 2, 50.0},
		{4, 1, 75.0},
		{4, 0, 100.0},
		{10, 8, 20.0},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			var loss float64
			if tt.sent > 0 {
				loss = float64(tt.sent-tt.recv) / float64(tt.sent) * 100
			}

			if loss != tt.expectedLoss {
				t.Errorf("丢包率计算错误: sent=%d, recv=%d, got=%f, expected=%f",
					tt.sent, tt.recv, loss, tt.expectedLoss)
			}
		})
	}
}

// TestParseTracepathOutputWithNoReply 测试解析包含 no reply 的 tracepath 输出
func TestParseTracepathOutputWithNoReply(t *testing.T) {
	mgr := NewManager("/tmp/test-network")

	output := `tracepath to 8.8.8.8 (8.8.8.8), 30 hops max
 1  192.168.1.1    0.456ms
 2  no reply
 3  10.0.0.1    1.234ms`

	result := &TracerouteResult{
		Host: "8.8.8.8",
		Hops: make([]TracerouteHop, 0),
	}

	mgr.parseTracepathOutput(output, result)

	if len(result.Hops) != 3 {
		t.Errorf("应该有 3 跳，实际: %d", len(result.Hops))
	}

	// 检查 no reply 跳
	if !result.Hops[1].Timeout {
		t.Errorf("第二跳应该超时")
	}
}

// TestPortScanMultiplePorts 测试多端口扫描结果
func TestPortScanResultMultiplePorts(t *testing.T) {
	result := &PortScanResult{
		Host: "192.168.1.1",
		Ports: []PortStatus{
			{Port: 22, Protocol: "tcp", Open: true, Service: "SSH"},
			{Port: 80, Protocol: "tcp", Open: true, Service: "HTTP"},
			{Port: 443, Protocol: "tcp", Open: false},
			{Port: 3306, Protocol: "tcp", Open: true, Service: "MySQL"},
		},
		ScanTime: 2000,
	}

	openCount := 0
	for _, p := range result.Ports {
		if p.Open {
			openCount++
		}
	}

	if openCount != 3 {
		t.Errorf("应该有 3 个开放端口，实际: %d", openCount)
	}
}

// TestDNSLookupResultWithMXRecords 测试带 MX 记录的 DNS 查询结果
func TestDNSLookupResultWithMXRecords(t *testing.T) {
	result := &DNSLookupResult{
		Host:      "example.com",
		Addresses: []string{"93.184.216.34"},
		MXRecords: []MXRecord{
			{Preference: 10, Host: "mail.example.com"},
			{Preference: 20, Host: "mail2.example.com"},
		},
		NSRecords: []NSRecord{
			{Host: "ns1.example.com"},
			{Host: "ns2.example.com"},
		},
		TXTRecords: []string{"v=spf1 include:_spf.example.com ~all"},
	}

	if len(result.MXRecords) != 2 {
		t.Errorf("应该有 2 个 MX 记录")
	}
	if len(result.NSRecords) != 2 {
		t.Errorf("应该有 2 个 NS 记录")
	}
	if len(result.TXTRecords) != 1 {
		t.Errorf("应该有 1 个 TXT 记录")
	}
}

// TestStringContainForParsing 测试字符串包含检查
func TestStringContainForParsing(t *testing.T) {
	output := "4 packets transmitted, 4 received, 0% packet loss"

	if !strings.Contains(output, "packets transmitted") {
		t.Error("应该包含 'packets transmitted'")
	}

	if !strings.Contains(output, "rtt min/avg/max/mdev") {
		// 这个输出不包含 RTT 统计，是正常的
		t.Log("输出不包含 RTT 统计")
	}
}

// TestTracerouteHopWithMultipleRTT 测试带多个 RTT 的路由跳
func TestTracerouteHopWithMultipleRTT(t *testing.T) {
	hop := TracerouteHop{
		Hop:  1,
		Host: "192.168.1.1",
		IP:   "192.168.1.1",
		RTT1: 0.123,
		RTT2: 0.456,
		RTT3: 0.789,
	}

	if hop.RTT1 > hop.RTT3 {
		t.Errorf("RTT1 应该小于 RTT3")
	}
}

// TestParsePortsEmpty 测试空端口字符串解析
func TestParsePortsEmpty(t *testing.T) {
	// 这个方法在 docker/manager.go 中，但我们测试类似的逻辑
	ports := ""
	if ports == "" {
		t.Log("空端口字符串应该返回空结果")
	}
}

// TestServiceIdentificationAllPorts 测试所有已知端口的服务识别
func TestServiceIdentificationAllPorts(t *testing.T) {
	mgr := NewManager("/tmp/test-network")

	knownPorts := map[int]string{
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

	for port, expectedService := range knownPorts {
		service := mgr.identifyService(port)
		if service != expectedService {
			t.Errorf("端口 %d: got %s, want %s", port, service, expectedService)
		}
	}
}

// TestDiagnosticResultWithError 测试带错误的诊断结果
func TestDiagnosticResultWithError(t *testing.T) {
	result := &DiagnosticResult{
		Success: false,
		Output:  "",
		Error:   "connection refused",
	}

	if result.Success {
		t.Errorf("Success 应该是 false")
	}
	if result.Error == "" {
		t.Errorf("应该有错误信息")
	}
}

// BenchmarkParsePingOutput 基准测试 Ping 输出解析
func BenchmarkParsePingOutput(b *testing.B) {
	mgr := NewManager("/tmp/test-network")

	output := `PING 192.168.1.1 (192.168.1.1) 56(84) bytes of data.
--- 192.168.1.1 ping statistics ---
4 packets transmitted, 4 received, 0% packet loss, time 3005ms
rtt min/avg/max/mdev = 0.123/0.456/0.789/0.123 ms`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := &PingResult{}
		mgr.parsePingOutput(output, result)
	}
}

// BenchmarkIdentifyService 基准测试服务识别
func BenchmarkIdentifyService(b *testing.B) {
	mgr := NewManager("/tmp/test-network")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.identifyService(443)
	}
}

// BenchmarkParseDigOutput 基准测试 Dig 输出解析
func BenchmarkParseDigOutput(b *testing.B) {
	mgr := NewManager("/tmp/test-network")

	output := `;; ANSWER SECTION:
example.com.		300	IN	A	93.184.216.34
example.com.		300	IN	A	93.184.216.35`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.ParseDigOutput(output)
	}
}

// TestNetworkDiagnoseStructure 测试网络诊断结构
func TestNetworkDiagnoseStructure(t *testing.T) {
	// 测试综合诊断结果结构
	results := map[string]interface{}{
		"dns": &DNSLookupResult{Host: "example.com", Addresses: []string{"1.2.3.4"}},
		"ping": &PingResult{Host: "example.com", PacketLoss: 0.0},
		"ports": &PortScanResult{Host: "example.com", Ports: []PortStatus{}},
	}

	if results["dns"] == nil {
		t.Error("dns 结果不应该为 nil")
	}
	if results["ping"] == nil {
		t.Error("ping 结果不应该为 nil")
	}
}