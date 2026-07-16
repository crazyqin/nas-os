package netscan

import (
	"context"
	"testing"
	"time"
)

func TestNewDiscoverer(t *testing.T) {
	config := DiscoveryConfig{
		Network:    "192.168.1.0/24",
		Timeout:    5 * time.Second,
		Concurrent: 100,
		UseARP:     true,
		UseICMP:    true,
	}

	d := NewDiscoverer(config)
	if d == nil {
		t.Fatal("NewDiscoverer 返回 nil")
	}

	if d.config.Timeout != 5*time.Second {
		t.Errorf("期望超时 5s，实际 %v", d.config.Timeout)
	}

	if d.config.Concurrent != 100 {
		t.Errorf("期望并发 100，实际 %d", d.config.Concurrent)
	}
}

func TestNewDiscovererDefaults(t *testing.T) {
	config := DiscoveryConfig{
		Network: "192.168.1.0/24",
	}

	d := NewDiscoverer(config)
	if d.config.Timeout != 3*time.Second {
		t.Errorf("默认超时应为 3s，实际 %v", d.config.Timeout)
	}

	if d.config.Concurrent != 50 {
		t.Errorf("默认并发应为 50，实际 %d", d.config.Concurrent)
	}
}

func TestExpandNetwork(t *testing.T) {
	d := NewDiscoverer(DiscoveryConfig{
		Network: "192.168.1.0/24",
	})

	ips, err := d.expandNetwork("192.168.1.0/24")
	if err != nil {
		t.Fatalf("展开网段失败：%v", err)
	}

	// /24 应该有 254 个可用 IP (去掉网络和广播地址)
	if len(ips) != 254 {
		t.Errorf("期望 254 个 IP，实际 %d", len(ips))
	}

	if ips[0] != "192.168.1.1" {
		t.Errorf("第一个 IP 应为 192.168.1.1，实际 %s", ips[0])
	}

	if ips[253] != "192.168.1.254" {
		t.Errorf("最后一个 IP 应为 192.168.1.254，实际 %s", ips[253])
	}
}

func TestExpandNetworkInvalid(t *testing.T) {
	d := NewDiscoverer(DiscoveryConfig{})

	_, err := d.expandNetwork("invalid")
	if err == nil {
		t.Error("应该返回错误")
	}
}

func TestParseMACFromArping(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "正常输出",
			input:  "ARPING 192.168.1.1 from 192.168.1.100 eth0\nUnicast reply from 192.168.1.1 [AA:BB:CC:DD:EE:FF]  1.234ms",
			expect: "aa:bb:cc:dd:ee:ff",
		},
		{
			name:   "无 MAC",
			input:  "ARPING 192.168.1.1\nTimeout",
			expect: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMACFromArping(tt.input)
			if result != tt.expect {
				t.Errorf("期望 %s，实际 %s", tt.expect, result)
			}
		})
	}
}

func TestGuessService(t *testing.T) {
	tests := []struct {
		port     int
		protocol string
		expect   string
	}{
		{22, "tcp", "ssh"},
		{80, "tcp", "http"},
		{443, "tcp", "https"},
		{3306, "tcp", "mysql"},
		{53, "udp", "dns"},
		{123, "udp", "ntp"},
		{99999, "tcp", ""},
	}

	for _, tt := range tests {
		result := GuessService(tt.port, tt.protocol)
		if result != tt.expect {
			t.Errorf("端口 %d/%s: 期望 %s，实际 %s", tt.port, tt.protocol, tt.expect, result)
		}
	}
}

func TestNewPortScanner(t *testing.T) {
	config := PortScanConfig{
		Target:     "127.0.0.1",
		Ports:      []int{22, 80, 443},
		Protocol:   ProtocolTCP,
		Timeout:    5 * time.Second,
		Concurrent: 100,
	}

	scanner := NewPortScanner(config)
	if scanner == nil {
		t.Fatal("NewPortScanner 返回 nil")
	}

	if scanner.config.Target != "127.0.0.1" {
		t.Errorf("目标错误：%s", scanner.config.Target)
	}
}

func TestPortScannerDefaults(t *testing.T) {
	config := PortScanConfig{
		Target: "127.0.0.1",
	}

	scanner := NewPortScanner(config)
	if scanner.config.Timeout != 2*time.Second {
		t.Errorf("默认超时应为 2s，实际 %v", scanner.config.Timeout)
	}

	if scanner.config.Concurrent != 100 {
		t.Errorf("默认并发应为 100，实际 %d", scanner.config.Concurrent)
	}

	if scanner.config.Protocol != ProtocolTCP {
		t.Errorf("默认协议应为 tcp，实际 %s", scanner.config.Protocol)
	}
}

func TestGetTargetPorts(t *testing.T) {
	// 自定义端口
	scanner1 := NewPortScanner(PortScanConfig{
		Ports: []int{80, 443},
	})
	ports1 := scanner1.getTargetPorts()
	if len(ports1) != 2 {
		t.Errorf("期望 2 个端口，实际 %d", len(ports1))
	}

	// Top 10
	scanner2 := NewPortScanner(PortScanConfig{
		TopPorts: 10,
	})
	ports2 := scanner2.getTargetPorts()
	if len(ports2) != 10 {
		t.Errorf("期望 10 个端口，实际 %d", len(ports2))
	}

	// 默认常用端口
	scanner3 := NewPortScanner(PortScanConfig{})
	ports3 := scanner3.getTargetPorts()
	if len(ports3) != len(CommonPorts) {
		t.Errorf("期望 %d 个常用端口，实际 %d", len(CommonPorts), len(ports3))
	}
}

func TestNewServiceDetector(t *testing.T) {
	config := ServiceDetectConfig{
		Target:     "127.0.0.1",
		Ports:      []int{22, 80},
		Timeout:    5 * time.Second,
		BannerGrab: true,
	}

	detector := NewServiceDetector(config)
	if detector == nil {
		t.Fatal("NewServiceDetector 返回 nil")
	}
}

func TestParseSSHVersion(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"SSH-2.0-OpenSSH_8.9p1 Ubuntu-3ubuntu0.1", "OpenSSH_8.9p1"},
		{"SSH-2.0-dropbear_2022.83", "dropbear_2022.83"},
		{"invalid", "invalid"},
	}

	for _, tt := range tests {
		result := parseSSHVersion(tt.input)
		if result != tt.expect {
			t.Errorf("输入 %s: 期望 %s，实际 %s", tt.input, tt.expect, result)
		}
	}
}

func TestParseHTTPServer(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"HTTP/1.1 200 OK\r\nServer: nginx/1.18.0\r\n", "nginx/1.18.0"},
		{"HTTP/1.1 200 OK\r\nContent-Type: text/html\r\n", ""},
	}

	for _, tt := range tests {
		result := parseHTTPServer(tt.input)
		if result != tt.expect {
			t.Errorf("期望 %s，实际 %s", tt.expect, result)
		}
	}
}

func TestParseFTPVersion(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"220 ProFTPD 1.3.7e Server", "ProFTPD 1.3.7e Server"},
		{"220-Welcome to Pure-FTPd", "Welcome to Pure-FTPd"},
	}

	for _, tt := range tests {
		result := parseFTPVersion(tt.input)
		if result != tt.expect {
			t.Errorf("期望 %s，实际 %s", tt.expect, result)
		}
	}
}

func TestNewTopologyBuilder(t *testing.T) {
	builder := NewTopologyBuilder()
	if builder == nil {
		t.Fatal("NewTopologyBuilder 返回 nil")
	}
}

func TestTopologyBuilderBuild(t *testing.T) {
	builder := NewTopologyBuilder()

	builder.AddDevice(Device{
		IP:       "192.168.1.1",
		MAC:      "aa:bb:cc:dd:ee:01",
		Hostname: "router",
		State:    DeviceStateOnline,
		OpenPorts: []Port{
			{Number: 80, Protocol: ProtocolTCP, State: PortStateOpen, Service: "http"},
			{Number: 53, Protocol: ProtocolUDP, State: PortStateOpen, Service: "dns"},
		},
	})

	builder.AddDevice(Device{
		IP:       "192.168.1.100",
		Hostname: "my-pc",
		State:    DeviceStateOnline,
		OpenPorts: []Port{
			{Number: 22, Protocol: ProtocolTCP, State: PortStateOpen, Service: "ssh"},
		},
	})

	topo := builder.Build()

	if len(topo.Nodes) != 2 {
		t.Errorf("期望 2 个节点，实际 %d", len(topo.Nodes))
	}

	if len(topo.Edges) == 0 {
		t.Error("应该有边连接")
	}

	// 验证设备类型
	for _, node := range topo.Nodes {
		if node.IP == "192.168.1.1" && node.Type != "router" {
			t.Errorf("路由器类型错误：%s", node.Type)
		}
	}
}

func TestSameSubnet(t *testing.T) {
	tests := []struct {
		ip1    string
		ip2    string
		expect bool
	}{
		{"192.168.1.1", "192.168.1.100", true},
		{"192.168.1.1", "192.168.2.1", false},
		{"10.0.0.1", "10.0.0.254", true},
		{"invalid", "192.168.1.1", false},
	}

	for _, tt := range tests {
		result := sameSubnet(tt.ip1, tt.ip2)
		if result != tt.expect {
			t.Errorf("%s 和 %s: 期望 %v，实际 %v", tt.ip1, tt.ip2, tt.expect, result)
		}
	}
}

func TestNewScanner(t *testing.T) {
	s := NewScanner(10)
	if s == nil {
		t.Fatal("NewScanner 返回 nil")
	}

	if s.taskMgr.maxWorkers != 10 {
		t.Errorf("期望 maxWorkers 10，实际 %d", s.taskMgr.maxWorkers)
	}
}

func TestNewTaskManager(t *testing.T) {
	tm := NewTaskManager(5)
	if tm == nil {
		t.Fatal("NewTaskManager 返回 nil")
	}

	if tm.maxWorkers != 5 {
		t.Errorf("期望 maxWorkers 5，实际 %d", tm.maxWorkers)
	}
}

func TestTaskManagerOperations(t *testing.T) {
	tm := NewTaskManager(10)

	task := &ScanTask{
		ID:        "test-1",
		Type:      "discovery",
		Target:    "192.168.1.0/24",
		Status:    "running",
		StartTime: time.Now(),
	}

	// 添加任务
	tm.AddTask(task)

	// 获取任务
	got, ok := tm.GetTask("test-1")
	if !ok || got.ID != "test-1" {
		t.Error("获取任务失败")
	}

	// 列出任务
	tasks := tm.ListTasks()
	if len(tasks) != 1 {
		t.Errorf("期望 1 个任务，实际 %d", len(tasks))
	}

	// 取消任务
	err := tm.CancelTask("test-1")
	if err != nil {
		t.Errorf("取消任务失败：%v", err)
	}

	got, _ = tm.GetTask("test-1")
	if got.Status != "cancelled" {
		t.Errorf("任务状态应为 cancelled，实际 %s", got.Status)
	}

	// 取消不存在的任务
	err = tm.CancelTask("non-exist")
	if err == nil {
		t.Error("应该返回错误")
	}
}

func TestTaskManagerCleanFinished(t *testing.T) {
	tm := NewTaskManager(10)

	tm.AddTask(&ScanTask{ID: "1", Status: "running"})
	tm.AddTask(&ScanTask{ID: "2", Status: "completed"})
	tm.AddTask(&ScanTask{ID: "3", Status: "failed"})
	tm.AddTask(&ScanTask{ID: "4", Status: "cancelled"})
	tm.AddTask(&ScanTask{ID: "5", Status: "running"})

	cleaned := tm.CleanFinished()
	if cleaned != 3 {
		t.Errorf("期望清理 3 个任务，实际 %d", cleaned)
	}

	tasks := tm.ListTasks()
	if len(tasks) != 2 {
		t.Errorf("期望剩余 2 个任务，实际 %d", len(tasks))
	}
}

func TestNewTaskManagerDefaults(t *testing.T) {
	tm := NewTaskManager(0)
	if tm.maxWorkers != 10 {
		t.Errorf("默认 maxWorkers 应为 10，实际 %d", tm.maxWorkers)
	}
}

func TestScannerListTasks(t *testing.T) {
	s := NewScanner(10)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 启动一个快速完成的任务
	config := PortScanConfig{
		Target:     "127.0.0.1",
		Ports:      []int{1}, // 不太可能开放的端口
		Protocol:   ProtocolTCP,
		Timeout:    100 * time.Millisecond,
		Concurrent: 1,
	}

	task, err := s.StartPortScan(ctx, config)
	if err != nil {
		t.Fatalf("启动任务失败：%v", err)
	}

	if task == nil {
		t.Fatal("任务为 nil")
	}

	// 等待任务完成
	time.Sleep(500 * time.Millisecond)

	tasks := s.ListTasks()
	if len(tasks) == 0 {
		t.Error("应该有任务")
	}
}
