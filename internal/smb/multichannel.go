package smb

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// MultichannelConfig 多通道SMB配置
type MultichannelConfig struct {
	Enabled           bool     `json:"enabled"`
	MaxChannels       int      `json:"max_channels"`        // 最大通道数
	Interfaces        []string `json:"interfaces"`          // 绑定的网卡接口
	AutoDiscover      bool     `json:"auto_discover"`       // 自动发现网卡
	RoundRobin        bool     `json:"round_robin"`         // 负载均衡轮询
	FailoverEnabled   bool     `json:"failover_enabled"`    // 故障切换
	HealthCheckSec    int      `json:"health_check_sec"`    // 健康检查间隔(秒)
	MinBandwidthMbps  int      `json:"min_bandwidth_mbps"`  // 最低带宽要求
	RequireSameSubnet bool     `json:"require_same_subnet"` // 要求同一子网
}

// NetworkInterface 网络接口信息（用于多通道）
type NetworkInterface struct {
	Name        string   `json:"name"`
	IPAddresses []string `json:"ip_addresses"`
	MAC         string   `json:"mac"`
	SpeedMbps   int      `json:"speed_mbps"`
	Up          bool     `json:"up"`
	Type        string   `json:"type"` // ethernet, wifi, bridge, virtual, loopback
	MTU         int      `json:"mtu"`
	RxDropped   int64    `json:"rx_dropped"`
	TxDropped   int64    `json:"tx_dropped"`
	Priority    int      `json:"priority"` // 用于排序优先级
}

// SMBChannel SMB通道信息
type SMBChannel struct {
	ID              int       `json:"id"`
	InterfaceName   string    `json:"interface_name"`
	IPAddress       string    `json:"ip_address"`
	Port            int       `json:"port"`
	Connected       bool      `json:"connected"`
	Connections     int       `json:"connections"` // 当前连接数
	BandwidthMbps   int       `json:"bandwidth_mbps"`
	ActiveSince     time.Time `json:"active_since"`
	LastError       string    `json:"last_error,omitempty"`
	HealthScore     int       `json:"health_score"` // 0-100
	RoundRobinIndex int       `json:"round_robin_index"`
}

// ChannelStatus 多通道状态
type ChannelStatus struct {
	Enabled          bool               `json:"enabled"`
	TotalChannels    int                `json:"total_channels"`
	ActiveChannels   int                `json:"active_channels"`
	TotalBandwidth   int                `json:"total_bandwidth_mbps"` // 总带宽
	TotalConnections int                `json:"total_connections"`
	Channels         []SMBChannel       `json:"channels"`
	Interfaces       []NetworkInterface `json:"interfaces"`
	FailoverActive   bool               `json:"failover_active"`
	LastHealthCheck  time.Time          `json:"last_health_check"`
	Config           MultichannelConfig `json:"config"`
}

// MultichannelManager 多通道管理器
type MultichannelManager struct {
	mu              sync.RWMutex
	config          MultichannelConfig
	channels        []*SMBChannel
	interfaces      []*NetworkInterface
	healthyChan     chan int      // 健康通道ID通知
	stopHealthCheck chan struct{} // 停止健康检查
	running         bool
}

// DefaultMultichannelConfig 默认多通道配置
func DefaultMultichannelConfig() MultichannelConfig {
	return MultichannelConfig{
		Enabled:           false,
		MaxChannels:       4,
		Interfaces:        []string{},
		AutoDiscover:      true,
		RoundRobin:        true,
		FailoverEnabled:   true,
		HealthCheckSec:    30,
		MinBandwidthMbps:  100,
		RequireSameSubnet: false,
	}
}

// NewMultichannelManager 创建多通道管理器
func NewMultichannelManager(config MultichannelConfig) *MultichannelManager {
	m := &MultichannelManager{
		config:     config,
		channels:   make([]*SMBChannel, 0),
		interfaces: make([]*NetworkInterface, 0),
		running:    false,
	}

	if config.Enabled {
		m.Start()
	}

	return m
}

// Start 启动多通道管理
func (m *MultichannelManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	// 发现网络接口
	if err := m.discoverInterfaces(); err != nil {
		return fmt.Errorf("发现网络接口失败: %w", err)
	}

	// 创建通道
	if err := m.createChannels(); err != nil {
		return fmt.Errorf("创建通道失败: %w", err)
	}

	// 启动健康检查
	m.healthyChan = make(chan int, 10)
	m.stopHealthCheck = make(chan struct{})
	go m.healthCheckLoop()

	m.running = true
	logInfo("多通道SMB已启动", "channels", len(m.channels))
	return nil
}

// Stop 停止多通道管理
func (m *MultichannelManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	// 停止健康检查
	if m.stopHealthCheck != nil {
		close(m.stopHealthCheck)
		m.stopHealthCheck = nil
	}

	// 清理通道
	m.channels = make([]*SMBChannel, 0)
	m.running = false

	logInfo("多通道SMB已停止")
	return nil
}

// discoverInterfaces 发现可用网络接口
func (m *MultichannelManager) discoverInterfaces() error {
	// 如果指定了接口列表，只使用指定的
	if len(m.config.Interfaces) > 0 && !m.config.AutoDiscover {
		return m.discoverSpecificInterfaces()
	}

	// 自动发现所有合适的接口
	return m.discoverAllInterfaces()
}

// discoverSpecificInterfaces 发现指定的接口
func (m *MultichannelManager) discoverSpecificInterfaces() error {
	for _, ifaceName := range m.config.Interfaces {
		iface, err := m.getInterfaceInfo(ifaceName)
		if err != nil {
			logError("获取接口信息失败", err, "interface", ifaceName)
			continue
		}

		// 检查接口是否符合要求
		if !m.isInterfaceSuitable(iface) {
			continue
		}

		iface.Priority = len(m.interfaces) + 1
		m.interfaces = append(m.interfaces, iface)
	}

	return nil
}

// discoverAllInterfaces 发现所有可用接口
func (m *MultichannelManager) discoverAllInterfaces() error {
	// 获取所有接口
	ifaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("获取网络接口失败: %w", err)
	}

	for _, iface := range ifaces {
		// 跳过回环和虚拟接口
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// 检查接口是否up
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		info, err := m.getInterfaceInfo(iface.Name)
		if err != nil {
			continue
		}

		// 检查是否合适
		if !m.isInterfaceSuitable(info) {
			continue
		}

		info.Priority = len(m.interfaces) + 1
		m.interfaces = append(m.interfaces, info)
	}

	// 按带宽优先级排序
	m.sortInterfacesByPriority()

	// 限制最大接口数
	if len(m.interfaces) > m.config.MaxChannels {
		m.interfaces = m.interfaces[:m.config.MaxChannels]
	}

	return nil
}

// getInterfaceInfo 获取接口详细信息
func (m *MultichannelManager) getInterfaceInfo(name string) (*NetworkInterface, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}

	info := &NetworkInterface{
		Name: name,
		MAC:  iface.HardwareAddr.String(),
		Up:   iface.Flags&net.FlagUp != 0,
		MTU:  iface.MTU,
		Type: m.getInterfaceType(name, iface.Flags),
	}

	// 获取IP地址
	addrs, err := iface.Addrs()
	if err == nil {
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if ok && ipnet.IP.To4() != nil {
				info.IPAddresses = append(info.IPAddresses, ipnet.IP.String())
			}
		}
	}

	// 获取速度
	info.SpeedMbps = m.getInterfaceSpeed(name)

	// 获取丢包统计
	info.RxDropped, info.TxDropped = m.getInterfaceDropped(name)

	return info, nil
}

// getInterfaceType 获取接口类型
func (m *MultichannelManager) getInterfaceType(name string, flags net.Flags) string {
	if flags&net.FlagLoopback != 0 {
		return "loopback"
	}

	// 根据名称判断类型
	switch {
	case strings.HasPrefix(name, "eth"), strings.HasPrefix(name, "en"):
		return "ethernet"
	case strings.HasPrefix(name, "wlan"), strings.HasPrefix(name, "wl"):
		return "wifi"
	case strings.HasPrefix(name, "br"):
		return "bridge"
	case strings.HasPrefix(name, "docker"), strings.HasPrefix(name, "veth"):
		return "virtual"
	case strings.HasPrefix(name, "bond"):
		return "bond"
	default:
		return "ethernet"
	}
}

// getInterfaceSpeed 获取接口速度(Mbps)
func (m *MultichannelManager) getInterfaceSpeed(name string) int {
	speedPath := fmt.Sprintf("/sys/class/net/%s/speed", name)
	data, err := os.ReadFile(speedPath)
	if err != nil {
		return 1000 // 默认1Gbps
	}

	speedStr := strings.TrimSpace(string(data))
	speed := 0
	if speedStr != "" && speedStr != "-1" {
		_, _ = fmt.Sscanf(speedStr, "%d", &speed)
	}

	if speed <= 0 {
		return 1000 // 默认值
	}

	return speed
}

// getInterfaceDropped 获取丢包统计
func (m *MultichannelManager) getInterfaceDropped(name string) (rxDropped, txDropped int64) {
	rxPath := fmt.Sprintf("/sys/class/net/%s/statistics/rx_dropped", name)
	txPath := fmt.Sprintf("/sys/class/net/%s/statistics/tx_dropped", name)

	rxData, err1 := os.ReadFile(rxPath)
	txData, err2 := os.ReadFile(txPath)

	if err1 == nil {
		_, _ = fmt.Sscanf(strings.TrimSpace(string(rxData)), "%d", &rxDropped)
	}
	if err2 == nil {
		_, _ = fmt.Sscanf(strings.TrimSpace(string(txData)), "%d", &txDropped)
	}

	return
}

// isInterfaceSuitable 检查接口是否适合多通道
func (m *MultichannelManager) isInterfaceSuitable(iface *NetworkInterface) bool {
	// 必须有IPv4地址
	if len(iface.IPAddresses) == 0 {
		return false
	}

	// 必须是up状态
	if !iface.Up {
		return false
	}

	// 跳过回环和虚拟接口
	if iface.Type == "loopback" || iface.Type == "virtual" {
		return false
	}

	// 检查带宽要求
	if iface.SpeedMbps < m.config.MinBandwidthMbps {
		return false
	}

	// 如果要求同一子网，检查IP地址
	if m.config.RequireSameSubnet && len(m.interfaces) > 0 {
		// 检查是否在同一子网（简化判断：相同网段前缀）
		firstIP := m.interfaces[0].IPAddresses[0]
		if !m.sameSubnet(firstIP, iface.IPAddresses[0]) {
			return false
		}
	}

	return true
}

// sameSubnet 检查两个IP是否在同一子网（简化版）
func (m *MultichannelManager) sameSubnet(ip1, ip2 string) bool {
	// 简化判断：取前3段比较
	parts1 := strings.Split(ip1, ".")
	parts2 := strings.Split(ip2, ".")

	if len(parts1) < 3 || len(parts2) < 3 {
		return false
	}

	return parts1[0] == parts2[0] && parts1[1] == parts2[1] && parts1[2] == parts2[2]
}

// sortInterfacesByPriority 按优先级排序接口
func (m *MultichannelManager) sortInterfacesByPriority() {
	// 按带宽降序排序
	for i := 0; i < len(m.interfaces); i++ {
		for j := i + 1; j < len(m.interfaces); j++ {
			if m.interfaces[j].SpeedMbps > m.interfaces[i].SpeedMbps {
				m.interfaces[i], m.interfaces[j] = m.interfaces[j], m.interfaces[i]
			}
		}
	}

	// 设置优先级
	for i, iface := range m.interfaces {
		iface.Priority = i + 1
	}
}

// createChannels 创建SMB通道
func (m *MultichannelManager) createChannels() error {
	for i, iface := range m.interfaces {
		for _, ip := range iface.IPAddresses {
			channel := &SMBChannel{
				ID:            i + 1,
				InterfaceName: iface.Name,
				IPAddress:     ip,
				Port:          445, // SMB默认端口
				BandwidthMbps: iface.SpeedMbps,
				Connected:     false,
				Connections:   0,
				HealthScore:   100,
				ActiveSince:   time.Now(),
			}
			m.channels = append(m.channels, channel)
			break // 每个接口只用一个IP
		}
	}

	return nil
}

// healthCheckLoop 健康检查循环
func (m *MultichannelManager) healthCheckLoop() {
	ticker := time.NewTicker(time.Duration(m.config.HealthCheckSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopHealthCheck:
			return
		case <-ticker.C:
			m.performHealthCheck()
		}
	}
}

// performHealthCheck 执行健康检查
func (m *MultichannelManager) performHealthCheck() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, channel := range m.channels {
		health := m.checkChannelHealth(channel)
		channel.HealthScore = health

		// 如果健康分数低于阈值，触发故障切换
		if health < 50 && m.config.FailoverEnabled {
			channel.Connected = false
			channel.LastError = "健康检查失败"
			logInfo("通道健康检查失败，禁用通道", "channel_id", channel.ID, "health", health)
		} else if health >= 50 && !channel.Connected {
			// 尝试恢复
			channel.Connected = true
			channel.LastError = ""
			logInfo("通道恢复", "channel_id", channel.ID)
		}
	}
}

// checkChannelHealth 检查通道健康状态
func (m *MultichannelManager) checkChannelHealth(channel *SMBChannel) int {
	health := 100

	// 检查接口状态
	iface, err := m.getInterfaceInfo(channel.InterfaceName)
	if err != nil {
		return 0
	}

	if !iface.Up {
		return 0
	}

	// 检查丢包率
	droppedTotal := iface.RxDropped + iface.TxDropped
	if droppedTotal > 1000 {
		health -= 30
	} else if droppedTotal > 100 {
		health -= 10
	}

	// 检查接口速度
	if iface.SpeedMbps < m.config.MinBandwidthMbps {
		health -= 20
	}

	// 检查SMB连接状态
	if m.config.FailoverEnabled {
		// 尝试连接测试
		if !m.testSMBConnection(channel.IPAddress, channel.Port) {
			health -= 40
		}
	}

	return health
}

// testSMBConnection 测试SMB连接
func (m *MultichannelManager) testSMBConnection(ip string, port int) bool {
	// 简化的连接测试
	timeout := time.Second * 2
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// GetStatus 获取多通道状态
func (m *MultichannelManager) GetStatus() *ChannelStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &ChannelStatus{
		Enabled:          m.config.Enabled && m.running,
		TotalChannels:    len(m.channels),
		ActiveChannels:   0,
		TotalBandwidth:   0,
		TotalConnections: 0,
		Channels:         make([]SMBChannel, 0),
		Interfaces:       make([]NetworkInterface, 0),
		Config:           m.config,
		LastHealthCheck:  time.Now(),
	}

	for _, ch := range m.channels {
		status.Channels = append(status.Channels, *ch)
		if ch.Connected {
			status.ActiveChannels++
			status.TotalBandwidth += ch.BandwidthMbps
			status.TotalConnections += ch.Connections
		}
	}

	for _, iface := range m.interfaces {
		status.Interfaces = append(status.Interfaces, *iface)
	}

	return status
}

// UpdateConfig 更新多通道配置
func (m *MultichannelManager) UpdateConfig(config MultichannelConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	wasRunning := m.running

	// 如果之前运行，先停止
	if wasRunning {
		if m.stopHealthCheck != nil {
			close(m.stopHealthCheck)
			m.stopHealthCheck = nil
		}
		m.running = false
	}

	m.config = config

	// 如果启用，重新启动
	if config.Enabled {
		m.mu.Unlock()
		err := m.Start()
		m.mu.Lock()
		return err
	}

	return nil
}

// GetRoundRobinInterface 获取轮询接口（负载均衡）
func (m *MultichannelManager) GetRoundRobinInterface() *NetworkInterface {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.RoundRobin || len(m.channels) == 0 {
		if len(m.interfaces) > 0 {
			return m.interfaces[0]
		}
		return nil
	}

	// 找到下一个健康的通道
	startIdx := 0
	for i := 0; i < len(m.channels); i++ {
		idx := (startIdx + i) % len(m.channels)
		ch := m.channels[idx]

		if ch.Connected && ch.HealthScore >= 70 {
			// 找到对应的接口
			for _, iface := range m.interfaces {
				if iface.Name == ch.InterfaceName {
					startIdx = (idx + 1) % len(m.channels)
					return iface
				}
			}
		}
	}

	return nil
}

// GenerateMultichannelConfig 生成多通道SMB配置片段
func GenerateMultichannelConfig(config *MultichannelConfig, interfaces []*NetworkInterface) string {
	if !config.Enabled {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("    server multi channel support = yes\n")

	// 配置接口绑定
	if len(interfaces) > 0 {
		var ifaceList []string
		for _, iface := range interfaces {
			for _, ip := range iface.IPAddresses {
				ifaceList = append(ifaceList, ip)
			}
		}
		if len(ifaceList) > 0 {
			sb.WriteString(fmt.Sprintf("    interfaces = %s\n", strings.Join(ifaceList, " ")))
			sb.WriteString("    bind interfaces only = yes\n")
		}
	}

	return sb.String()
}

// GetActiveInterfaceIPs 获取所有活动接口的IP地址
func (m *MultichannelManager) GetActiveInterfaceIPs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var ips []string
	for _, ch := range m.channels {
		if ch.Connected && ch.HealthScore >= 50 {
			ips = append(ips, ch.IPAddress)
		}
	}
	return ips
}

// GetInterfaceByIP 根据IP获取接口信息
func (m *MultichannelManager) GetInterfaceByIP(ip string) *NetworkInterface {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, iface := range m.interfaces {
		for _, ifaceIP := range iface.IPAddresses {
			if ifaceIP == ip {
				return iface
			}
		}
	}
	return nil
}

// GetChannelByID 根据ID获取通道信息
func (m *MultichannelManager) GetChannelByID(id int) *SMBChannel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ch := range m.channels {
		if ch.ID == id {
			return ch
		}
	}
	return nil
}

// EnableChannel 启用指定通道
func (m *MultichannelManager) EnableChannel(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ch := range m.channels {
		if ch.ID == id {
			ch.Connected = true
			ch.HealthScore = 100
			ch.ActiveSince = time.Now()
			ch.LastError = ""
			logInfo("通道已启用", "channel_id", id)
			return nil
		}
	}

	return fmt.Errorf("通道不存在: %d", id)
}

// DisableChannel 禁用指定通道
func (m *MultichannelManager) DisableChannel(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ch := range m.channels {
		if ch.ID == id {
			ch.Connected = false
			ch.LastError = "手动禁用"
			logInfo("通道已禁用", "channel_id", id)
			return nil
		}
	}

	return fmt.Errorf("通道不存在: %d", id)
}

// GetSmbStatusOutput 获取smbstatus多通道信息
func (m *MultichannelManager) GetSmbStatusOutput() ([]MultichannelConnectionInfo, error) {
	// 执行 smbstatus -p 获取进程信息
	cmd := exec.Command("smbstatus", "-p")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行smbstatus失败: %w", err)
	}

	return m.parseSmbStatusOutput(string(output))
}

// MultichannelConnectionInfo SMB多通道连接信息
type MultichannelConnectionInfo struct {
	PID        int    `json:"pid"`
	SessionID  string `json:"session_id"`
	Username   string `json:"username"`
	ClientIP   string `json:"client_ip"`
	ChannelIP  string `json:"channel_ip"` // 服务器端通道IP
	Protocol   string `json:"protocol"`
	Encryption string `json:"encryption"`
	ShareCount int    `json:"share_count"`
}

// parseSmbStatusOutput 解析smbstatus输出
func (m *MultichannelManager) parseSmbStatusOutput(output string) ([]MultichannelConnectionInfo, error) {
	var connections []MultichannelConnectionInfo

	// 正则匹配连接信息
	// 格式示例: 1234  192.168.1.100 (ipv4:192.168.1.1:445)  SMB3_11  AES-128-GCM
	re := regexp.MustCompile(`(\d+)\s+([\d.]+)\s+\(ipv4:([\d.]+):(\d+)\)\s+(\S+)\s+(\S+)`)

	matches := re.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		if len(match) >= 7 {
			conn := MultichannelConnectionInfo{
				PID:        0,
				ClientIP:   match[2],
				ChannelIP:  match[3], // 服务器端IP
				Protocol:   match[5],
				Encryption: match[6],
			}
			_, _ = fmt.Sscanf(match[1], "%d", &conn.PID)
			connections = append(connections, conn)
		}
	}

	return connections, nil
}

// GetMultichannelMetrics 获取多通道性能指标
func (m *MultichannelManager) GetMultichannelMetrics() *MultichannelMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := &MultichannelMetrics{
		TotalChannels:  len(m.channels),
		ActiveChannels: 0,
		TotalBandwidth: 0,
		AvgHealthScore: 0,
		FailoverCount:  0,
		LastUpdate:     time.Now(),
		ChannelMetrics: make([]ChannelMetric, 0),
	}

	totalHealth := 0
	for _, ch := range m.channels {
		if ch.Connected {
			metrics.ActiveChannels++
			metrics.TotalBandwidth += ch.BandwidthMbps
		}
		totalHealth += ch.HealthScore

		metrics.ChannelMetrics = append(metrics.ChannelMetrics, ChannelMetric{
			ChannelID:     ch.ID,
			InterfaceName: ch.InterfaceName,
			BandwidthMbps: ch.BandwidthMbps,
			Connections:   ch.Connections,
			HealthScore:   ch.HealthScore,
			Connected:     ch.Connected,
		})
	}

	if len(m.channels) > 0 {
		metrics.AvgHealthScore = totalHealth / len(m.channels)
	}

	return metrics
}

// MultichannelMetrics 多通道性能指标
type MultichannelMetrics struct {
	TotalChannels  int             `json:"total_channels"`
	ActiveChannels int             `json:"active_channels"`
	TotalBandwidth int             `json:"total_bandwidth_mbps"`
	AvgHealthScore int             `json:"avg_health_score"`
	FailoverCount  int             `json:"failover_count"`
	LastUpdate     time.Time       `json:"last_update"`
	ChannelMetrics []ChannelMetric `json:"channel_metrics"`
}

// ChannelMetric 单通道指标
type ChannelMetric struct {
	ChannelID     int    `json:"channel_id"`
	InterfaceName string `json:"interface_name"`
	BandwidthMbps int    `json:"bandwidth_mbps"`
	Connections   int    `json:"connections"`
	HealthScore   int    `json:"health_score"`
	Connected     bool   `json:"connected"`
}

// ValidateMultichannelConfig 验证多通道配置
func ValidateMultichannelConfig(config *MultichannelConfig) error {
	if config.MaxChannels < 1 {
		return fmt.Errorf("最大通道数必须大于0")
	}
	if config.MaxChannels > 32 {
		return fmt.Errorf("最大通道数不能超过32")
	}
	if config.HealthCheckSec < 5 {
		return fmt.Errorf("健康检查间隔至少5秒")
	}
	if config.HealthCheckSec > 300 {
		return fmt.Errorf("健康检查间隔不能超过300秒")
	}
	if config.MinBandwidthMbps < 10 {
		return fmt.Errorf("最低带宽要求至少10Mbps")
	}

	return nil
}
