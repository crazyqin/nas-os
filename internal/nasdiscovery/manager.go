// Package nasdiscovery 提供局域网 NAS 设备自动发现功能
// 支持 mDNS/SSDP/UDP 广播多种发现协议
// 类似群晖 Synology Finder / 飞牛发现协议
package nasdiscovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 设备发现管理器
type Manager struct {
	mu          sync.RWMutex
	storagePath string
	devices     map[string]*NASDevice
	config      *DiscoveryConfig
	scanCancel  context.CancelFunc
	// 发现到的设备通知通道
	DeviceChan chan *NASDevice
}

// NewManager 创建设备发现管理器
func NewManager(storagePath string) *Manager {
	m := &Manager{
		storagePath: storagePath,
		devices:     make(map[string]*NASDevice),
		config: &DiscoveryConfig{
			Enabled:         true,
			ScanInterval:    60, // 60秒
			UDPPort:         9999,
			MDNSEnabled:     true,
			SSDPEnabled:     true,
			BroadcastAddr:   "255.255.255.255",
			AutoAddDevices:  true,
			TrustedNetworks: []string{"192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12"},
		},
		DeviceChan: make(chan *NASDevice, 100),
	}

	// 加载已保存的设备
	m.loadDevices()

	// 创建存储目录
	os.MkdirAll(filepath.Join(storagePath, "discovery"), 0755)

	return m
}

// StartDiscovery 启动设备发现
func (m *Manager) StartDiscovery(ctx context.Context) error {
	m.mu.Lock()
	if m.scanCancel != nil {
		m.mu.Unlock()
		return ErrAlreadyRunning
	}

	scanCtx, cancel := context.WithCancel(ctx)
	m.scanCancel = cancel
	m.mu.Unlock()

	// 启动 UDP 广播发现
	if m.config.UDPPort > 0 {
		go m.udpBroadcastLoop(scanCtx)
		go m.udpListenLoop(scanCtx)
	}

	// 启动主动扫描
	go m.activeScanLoop(scanCtx)

	return nil
}

// StopDiscovery 停止设备发现
func (m *Manager) StopDiscovery() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.scanCancel != nil {
		m.scanCancel()
		m.scanCancel = nil
	}
}

// ScanNow 立即执行一次扫描
func (m *Manager) ScanNow(ctx context.Context) (*ScanResult, error) {
	startTime := time.Now()
	result := &ScanResult{
		ID:        uuid.New().String(),
		StartTime: startTime,
		Status:    ScanStatusRunning,
	}

	// 获取本地网络接口
	ifaces, err := net.Interfaces()
	if err != nil {
		result.Status = ScanStatusFailed
		result.Error = err.Error()
		return result, err
	}

	discovered := make(map[string]*NASDevice)

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}

			// 扫描子网
			devices := m.scanSubnet(ctx, ipNet, iface.HardwareAddr)
			for _, d := range devices {
				discovered[d.IP] = d
			}
		}
	}

	// 合并到已知设备
	m.mu.Lock()
	for _, device := range discovered {
		if existing, ok := m.devices[device.IP]; ok {
			// 更新已有设备
			existing.LastSeen = time.Now()
			existing.Status = DeviceStatusOnline
			existing.Hostname = device.Hostname
			existing.MAC = device.MAC
		} else {
			device.ID = uuid.New().String()
			device.FirstSeen = time.Now()
			device.LastSeen = time.Now()
			device.Status = DeviceStatusOnline
			m.devices[device.IP] = device

			// 通知新设备
			select {
			case m.DeviceChan <- device:
			default:
			}
		}
	}
	m.mu.Unlock()

	endTime := time.Now()
	result.EndTime = &endTime
	result.Duration = endTime.Sub(startTime).String()
	result.DevicesFound = len(discovered)
	result.TotalDevices = len(m.devices)
	result.Status = ScanStatusCompleted

	// 保存设备列表
	m.saveDevices()

	return result, nil
}

// scanSubnet 扫描子网中的设备
func (m *Manager) scanSubnet(ctx context.Context, ipNet *net.IPNet, localMAC net.HardwareAddr) []*NASDevice {
	var devices []*NASDevice

	ip := ipNet.IP.Mask(ipNet.Mask)
	mask := ipNet.Mask

	// 计算子网大小
	ones, bits := mask.Size()
	if bits != 32 || ones < 16 {
		// 限制扫描范围，避免过大的子网
		return devices
	}

	// 生成子网内所有 IP
	for i := 1; i < (1<<uint(32-ones))-1; i++ {
		select {
		case <-ctx.Done():
			return devices
		default:
		}

		targetIP := make(net.IP, 4)
		copy(targetIP, ip.To4())
		for j := 3; j >= 0; j-- {
			targetIP[j] |= byte(i >> ((3 - j) * 8))
		}

		// 快速 TCP 连接检测（常用端口）
		device := m.probeDevice(ctx, targetIP.String())
		if device != nil {
			devices = append(devices, device)
		}
	}

	return devices
}

// probeDevice 探测单个设备
func (m *Manager) probeDevice(ctx context.Context, ip string) *NASDevice {
	// 检测常见 NAS 端口
	ports := []int{80, 443, 5000, 5001, 8080, 8443, 9090, 22}

	openPorts := make([]int, 0)
	for _, port := range ports {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

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

	// 尝试获取主机名
	hostname, _ := net.LookupAddr(ip)

	device := &NASDevice{
		IP:        ip,
		OpenPorts: openPorts,
		IsNAS:     m.isLikelyNAS(openPorts),
	}

	if len(hostname) > 0 {
		device.Hostname = hostname[0]
	}

	// 如果是 HTTPS 端口开放，标记为支持 SSL
	for _, port := range openPorts {
		if port == 443 || port == 5001 || port == 8443 {
			device.SSLEnabled = true
			break
		}
	}

	return device
}

// isLikelyNAS 判断是否为 NAS 设备
func (m *Manager) isLikelyNAS(ports []int) bool {
	// NAS 常见端口组合
	nasPorts := map[int]bool{
		5000: true, // Synology DSM HTTP
		5001: true, // Synology DSM HTTPS
		8080: true, // 通用 Web 管理
		9090: true, // TrueNAS
	}

	matchCount := 0
	for _, port := range ports {
		if nasPorts[port] {
			matchCount++
		}
	}

	return matchCount > 0
}

// GetDevices 获取所有已知设备
func (m *Manager) GetDevices() []*NASDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*NASDevice, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	return devices
}

// GetDevice 获取单个设备
func (m *Manager) GetDevice(deviceID string) (*NASDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, d := range m.devices {
		if d.ID == deviceID {
			return d, nil
		}
	}
	return nil, ErrDeviceNotFound
}

// RemoveDevice 移除设备
func (m *Manager) RemoveDevice(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for ip, d := range m.devices {
		if d.ID == deviceID {
			delete(m.devices, ip)
			m.saveDevices()
			return nil
		}
	}
	return ErrDeviceNotFound
}

// UpdateDeviceConfig 更新设备发现配置
func (m *Manager) UpdateDeviceConfig(config DiscoveryConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.ScanInterval > 0 {
		m.config.ScanInterval = config.ScanInterval
	}
	if config.UDPPort > 0 {
		m.config.UDPPort = config.UDPPort
	}
	m.config.Enabled = config.Enabled
	m.config.MDNSEnabled = config.MDNSEnabled
	m.config.SSDPEnabled = config.SSDPEnabled
	m.config.AutoAddDevices = config.AutoAddDevices

	if len(config.TrustedNetworks) > 0 {
		m.config.TrustedNetworks = config.TrustedNetworks
	}
}

// GetConfig 获取当前配置
func (m *Manager) GetConfig() *DiscoveryConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// MarkTrusted 标记设备为受信任
func (m *Manager) MarkTrusted(deviceID string, trusted bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, d := range m.devices {
		if d.ID == deviceID {
			d.Trusted = trusted
			m.saveDevices()
			return nil
		}
	}
	return ErrDeviceNotFound
}

// AddManualDevice 手动添加设备
func (m *Manager) AddManualDevice(ctx context.Context, ip, name string) (*NASDevice, error) {
	device := &NASDevice{
		ID:        uuid.New().String(),
		IP:        ip,
		Hostname:  name,
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
		Status:    DeviceStatusOnline,
		ManualAdd: true,
		IsNAS:     true,
	}

	m.mu.Lock()
	m.devices[ip] = device
	m.mu.Unlock()

	m.saveDevices()
	return device, nil
}

// GetOnlineDevices 获取在线设备
func (m *Manager) GetOnlineDevices() []*NASDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var online []*NASDevice
	for _, d := range m.devices {
		if d.Status == DeviceStatusOnline {
			online = append(online, d)
		}
	}
	return online
}

// udpBroadcastLoop UDP 广播循环
func (m *Manager) udpBroadcastLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(m.config.ScanInterval) * time.Second)
	defer ticker.Stop()

	// 立即广播一次
	m.sendBroadcast()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.sendBroadcast()
		}
	}
}

// sendBroadcast 发送 UDP 广播
func (m *Manager) sendBroadcast() {
	addr := net.JoinHostPort(m.config.BroadcastAddr, fmt.Sprintf("%d", m.config.UDPPort))
	conn, err := net.DialTimeout("udp", addr, 2*time.Second)
	if err != nil {
		return
	}
	defer conn.Close()

	msg := DiscoveryMessage{
		Type:      "discovery",
		Hostname:  getHostname(),
		Timestamp: time.Now().Unix(),
		Version:   "1.0",
	}

	data, _ := json.Marshal(msg)
	conn.Write(data)
}

// udpListenLoop UDP 监听循环
func (m *Manager) udpListenLoop(ctx context.Context) {
	addr := fmt.Sprintf(":%d", m.config.UDPPort)
	listener, err := net.ListenPacket("udp4", addr)
	if err != nil {
		return
	}
	defer listener.Close()

	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		listener.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, remoteAddr, err := listener.ReadFrom(buf)
		if err != nil {
			continue
		}

		var msg DiscoveryMessage
		if err := json.Unmarshal(buf[:n], &msg); err != nil {
			continue
		}

		if msg.Type == "discovery" {
			// 收到其他设备的发现请求，发送响应
			m.sendResponse(remoteAddr.String())
		} else if msg.Type == "response" {
			// 收到响应，添加设备
			ip, _, _ := net.SplitHostPort(remoteAddr.String())
			m.handleDiscoveryResponse(ip, msg)
		}
	}
}

// sendResponse 发送发现响应
func (m *Manager) sendResponse(target string) {
	conn, err := net.DialTimeout("udp", target, 2*time.Second)
	if err != nil {
		return
	}
	defer conn.Close()

	msg := DiscoveryMessage{
		Type:      "response",
		Hostname:  getHostname(),
		Timestamp: time.Now().Unix(),
		Version:   "1.0",
	}

	data, _ := json.Marshal(msg)
	conn.Write(data)
}

// handleDiscoveryResponse 处理发现响应
func (m *Manager) handleDiscoveryResponse(ip string, msg DiscoveryMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.devices[ip]; ok {
		existing.LastSeen = time.Now()
		existing.Status = DeviceStatusOnline
		if msg.Hostname != "" {
			existing.Hostname = msg.Hostname
		}
	} else if m.config.AutoAddDevices {
		device := &NASDevice{
			ID:        uuid.New().String(),
			IP:        ip,
			Hostname:  msg.Hostname,
			FirstSeen: time.Now(),
			LastSeen:  time.Now(),
			Status:    DeviceStatusOnline,
			IsNAS:     true, // 通过发现协议找到的设备视为 NAS
		}
		m.devices[ip] = device

		select {
		case m.DeviceChan <- device:
		default:
		}
	}
}

// activeScanLoop 主动扫描循环
func (m *Manager) activeScanLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(m.config.ScanInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkDeviceStatus()
		}
	}
}

// checkDeviceStatus 检查设备状态
func (m *Manager) checkDeviceStatus() {
	m.mu.Lock()
	defer m.mu.Unlock()

	timeout := 5 * time.Second
	for _, device := range m.devices {
		if device.ManualAdd {
			// 手动添加的设备始终标记为在线
			continue
		}

		// 尝试连接设备的开放端口
		online := false
		for _, port := range device.OpenPorts {
			addr := net.JoinHostPort(device.IP, fmt.Sprintf("%d", port))
			conn, err := net.DialTimeout("tcp", addr, timeout)
			if err == nil {
				conn.Close()
				online = true
				break
			}
		}

		if online {
			device.Status = DeviceStatusOnline
			device.LastSeen = time.Now()
		} else {
			// 超过 5 分钟未响应标记为离线
			if time.Since(device.LastSeen) > 5*time.Minute {
				device.Status = DeviceStatusOffline
			}
		}
	}
}

// saveDevices 保存设备列表到文件
func (m *Manager) saveDevices() error {
	data, err := json.MarshalIndent(m.devices, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(m.storagePath, "discovery", "devices.json")
	return os.WriteFile(path, data, 0644)
}

// loadDevices 从文件加载设备列表
func (m *Manager) loadDevices() error {
	path := filepath.Join(m.storagePath, "discovery", "devices.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &m.devices)
}

// getHostname 获取本机主机名
func getHostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return name
}
