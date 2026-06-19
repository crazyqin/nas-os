// Package remoteaccess 提供 STUN 客户端实现 (RFC 5389)
// 用于 NAT 类型检测和外部地址发现
package remoteaccess

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

// STUN 消息类型
const (
	STUNBindingRequest  uint16 = 0x0001
	STUNBindingResponse uint16 = 0x0101
	STUNBindingError    uint16 = 0x0111
)

// STUN 属性类型
const (
	STUNAttrMappedAddress     uint16 = 0x0001
	STUNAttrResponseAddress   uint16 = 0x0002
	STUNAttrChangeRequest     uint16 = 0x0003
	STUNAttrSourceAddress     uint16 = 0x0004
	STUNAttrChangedAddress    uint16 = 0x0005
	STUNAttrXORMappedAddress  uint16 = 0x0020
	STUNAttrSoftware          uint16 = 0x8022
	STUNAttrFingerprint       uint16 = 0x8028
)

// STUNMagicCookie STUN 魔术字 (RFC 5389)
const STUNMagicCookie uint32 = 0x2112A442

// STUNClient STUN 客户端
type STUNClient struct {
	logger    *zap.Logger
	timeout   time.Duration
	localAddr string
}

// NewSTUNClient 创建 STUN 客户端
func NewSTUNClient(logger *zap.Logger) *STUNClient {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &STUNClient{
		logger:  logger,
		timeout: 5 * time.Second,
	}
}

// STUNResult STUN 查询结果
type STUNResult struct {
	MappedIP    net.IP
	MappedPort  int
	ServerAddr  string
	RTT         time.Duration
	Success     bool
}

// Query 发送 STUN Binding Request 并获取映射地址
func (c *STUNClient) Query(ctx context.Context, serverAddr string) (*STUNResult, error) {
	// 解析服务器地址
	udpAddr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("解析 STUN 服务器地址失败: %w", err)
	}

	// 创建 UDP 连接
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 0})
	if err != nil {
		return nil, fmt.Errorf("创建 UDP 连接失败: %w", err)
	}
	defer conn.Close()

	// 设置读取超时
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.timeout)
	}
	conn.SetReadDeadline(deadline)

	// 获取本地地址
	localAddr := conn.LocalAddr().(*net.UDPAddr)

	// 构建 STUN Binding Request
	msg := buildSTUNBindingRequest()

	// 记录发送时间
	start := time.Now()

	// 发送请求
	_, err = conn.WriteToUDP(msg, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("发送 STUN 请求失败: %w", err)
	}

	// 读取响应
	buf := make([]byte, 1500)
	n, remoteAddr, err := conn.ReadFromUDP(buf)
	if err != nil {
		return nil, fmt.Errorf("读取 STUN 响应失败: %w", err)
	}
	rtt := time.Since(start)

	// 解析响应
	result, err := parseSTUNResponse(buf[:n])
	if err != nil {
		return nil, fmt.Errorf("解析 STUN 响应失败: %w", err)
	}

	result.ServerAddr = remoteAddr.String()
	result.RTT = rtt

	c.logger.Debug("STUN 查询成功",
		zap.String("server", serverAddr),
		zap.String("mapped_addr", fmt.Sprintf("%s:%d", result.MappedIP, result.MappedPort)),
		zap.Duration("rtt", rtt),
		zap.String("local_addr", localAddr.String()),
	)

	return result, nil
}

// QueryWithRetry 带重试的 STUN 查询
func (c *STUNClient) QueryWithRetry(ctx context.Context, serverAddr string, maxRetries int) (*STUNResult, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		result, err := c.Query(ctx, serverAddr)
		if err == nil {
			return result, nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(i+1) * 500 * time.Millisecond):
		}
	}
	return nil, fmt.Errorf("STUN 查询重试 %d 次后失败: %w", maxRetries, lastErr)
}

// DetectNATType 检测 NAT 类型 (RFC 3489 简化版)
func (c *STUNClient) DetectNATType(ctx context.Context, servers []STUNServer) (*NATDetectionResult, error) {
	if len(servers) < 2 {
		return nil, fmt.Errorf("至少需要 2 个 STUN 服务器")
	}

	// 第一步：使用第一个 STUN 服务器测试
	result1, err := c.Query(ctx, servers[0].Address)
	if err != nil {
		return nil, fmt.Errorf("第一次 STUN 查询失败: %w", err)
	}

	// 获取本地地址
	localConn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 0})
	if err != nil {
		return nil, err
	}
	localAddr := localConn.LocalAddr().(*net.UDPAddr)
	localConn.Close()

	detection := &NATDetectionResult{
		ExternalIP:   result1.MappedIP.String(),
		ExternalPort: result1.MappedPort,
		LocalIP:      localAddr.IP.String(),
		LocalPort:    localAddr.Port,
		DetectedAt:   time.Now(),
		STUNServer:   servers[0].Address,
	}

	// 判断是否在 NAT 后面
	if result1.MappedIP.String() == localAddr.IP.String() {
		// 没有 NAT，公网直连
		detection.NATType = NATTypeUnknown
		detection.MappingType = "none"
		detection.FilteringType = "none"
		detection.SymmetricNAT = false
		return detection, nil
	}

	// 第二步：使用同一服务器的不同源端口检测对称 NAT
	result2, err := c.Query(ctx, servers[0].Address)
	if err != nil {
		c.logger.Warn("第二次 STUN 查询失败，假设受限 NAT", zap.Error(err))
		detection.NATType = NATTypeRestricted
		detection.MappingType = "address_restricted"
		detection.FilteringType = "address_restricted"
		return detection, nil
	}

	// 如果映射地址不同，是对称 NAT
	if result2.MappedIP.String() != result1.MappedIP.String() ||
		result2.MappedPort != result1.MappedPort {
		detection.NATType = NATTypeSymmetric
		detection.MappingType = "endpoint_dependent"
		detection.FilteringType = "endpoint_dependent"
		detection.SymmetricNAT = true
		return detection, nil
	}

	// 第三步：使用第二个 STUN 服务器检测过滤类型
	if len(servers) >= 2 {
		result3, err := c.Query(ctx, servers[1].Address)
		if err != nil {
			c.logger.Warn("第三个 STUN 查询失败", zap.Error(err))
			detection.NATType = NATTypeRestricted
			detection.MappingType = "endpoint_independent"
			detection.FilteringType = "address_restricted"
			return detection, nil
		}

		// 如果映射地址相同，是完全锥形 NAT
		if result3.MappedIP.String() == result1.MappedIP.String() &&
			result3.MappedPort == result1.MappedPort {
			detection.NATType = NATTypeFullCone
			detection.MappingType = "endpoint_independent"
			detection.FilteringType = "endpoint_independent"
			detection.SymmetricNAT = false
		} else {
			// 映射地址不同，是对称 NAT
			detection.NATType = NATTypeSymmetric
			detection.MappingType = "endpoint_dependent"
			detection.FilteringType = "endpoint_dependent"
			detection.SymmetricNAT = true
		}
	} else {
		detection.NATType = NATTypeRestricted
		detection.MappingType = "endpoint_independent"
		detection.FilteringType = "address_restricted"
	}

	return detection, nil
}

// buildSTUNBindingRequest 构建 STUN Binding Request 消息
func buildSTUNBindingRequest() []byte {
	// 生成随机事务 ID
	transactionID := make([]byte, 12)
	rand.Read(transactionID)

	msg := make([]byte, 20)

	// 消息类型: Binding Request (0x0001)
	binary.BigEndian.PutUint16(msg[0:2], STUNBindingRequest)

	// 消息长度: 0 (无属性)
	binary.BigEndian.PutUint16(msg[2:4], 0)

	// 魔术字
	binary.BigEndian.PutUint32(msg[4:8], STUNMagicCookie)

	// 事务 ID
	copy(msg[8:20], transactionID)

	return msg
}

// parseSTUNResponse 解析 STUN Binding Response
func parseSTUNResponse(data []byte) (*STUNResult, error) {
	if len(data) < 20 {
		return nil, fmt.Errorf("STUN 响应太短: %d bytes", len(data))
	}

	// 检查消息类型
	msgType := binary.BigEndian.Uint16(data[0:2])
	if msgType == STUNBindingError {
		return nil, fmt.Errorf("STUN 服务器返回错误")
	}
	if msgType != STUNBindingResponse {
		return nil, fmt.Errorf("未知的 STUN 消息类型: 0x%04x", msgType)
	}

	// 检查魔术字
	magicCookie := binary.BigEndian.Uint32(data[4:8])
	if magicCookie != STUNMagicCookie {
		return nil, fmt.Errorf("无效的 STUN 魔术字: 0x%08x", magicCookie)
	}

	// 消息长度
	msgLength := binary.BigEndian.Uint16(data[2:4])
	if int(msgLength)+20 > len(data) {
		return nil, fmt.Errorf("STUN 消息长度不匹配")
	}

	result := &STUNResult{Success: true}

	// 解析属性
	offset := 20
	for offset+4 <= int(msgLength)+20 {
		attrType := binary.BigEndian.Uint16(data[offset : offset+2])
		attrLength := binary.BigEndian.Uint16(data[offset+2 : offset+4])

		if offset+4+int(attrLength) > len(data) {
			break
		}

		attrData := data[offset+4 : offset+4+int(attrLength)]

		switch attrType {
		case STUNAttrMappedAddress, STUNAttrXORMappedAddress:
			ip, port, err := parseMappedAddress(attrData, attrType == STUNAttrXORMappedAddress)
			if err == nil {
				result.MappedIP = ip
				result.MappedPort = port
			}
		}

		// 属性按 4 字节对齐
		offset += 4 + int(attrLength)
		if attrLength%4 != 0 {
			offset += int(4 - attrLength%4)
		}
	}

	if result.MappedIP == nil {
		return nil, fmt.Errorf("STUN 响应中未找到映射地址属性")
	}

	return result, nil
}

// parseMappedAddress 解析 MAPPED-ADDRESS 或 XOR-MAPPED-ADDRESS 属性
func parseMappedAddress(data []byte, xor bool) (net.IP, int, error) {
	if len(data) < 8 {
		return nil, 0, fmt.Errorf("映射地址属性太短")
	}

	// 地址族
	family := data[1]
	if family != 0x01 && family != 0x02 {
		return nil, 0, fmt.Errorf("不支持的地址族: %d", family)
	}

	// 端口 (XOR 时与魔术字高 16 位异或)
	port := binary.BigEndian.Uint16(data[2:4])
	if xor {
		port = port ^ uint16(STUNMagicCookie>>16)
	}

	// IP 地址
	var ip net.IP
	if family == 0x01 {
		// IPv4
		if len(data) < 8 {
			return nil, 0, fmt.Errorf("IPv4 地址数据太短")
		}
		ip = make(net.IP, 4)
		copy(ip, data[4:8])
		if xor {
			cookieBytes := make([]byte, 4)
			binary.BigEndian.PutUint32(cookieBytes, STUNMagicCookie)
			for i := 0; i < 4; i++ {
				ip[i] ^= cookieBytes[i]
			}
		}
	} else {
		// IPv6
		if len(data) < 20 {
			return nil, 0, fmt.Errorf("IPv6 地址数据太短")
		}
		ip = make(net.IP, 16)
		copy(ip, data[4:20])
		if xor {
			cookieBytes := make([]byte, 4)
			binary.BigEndian.PutUint32(cookieBytes, STUNMagicCookie)
			for i := 0; i < 4; i++ {
				ip[i] ^= cookieBytes[i]
			}
		}
	}

	return ip, int(port), nil
}

// STUNServerPool STUN 服务器池，支持并发查询和结果聚合
type STUNServerPool struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	servers  []STUNServer
	client   *STUNClient
	bestIdx  int
}

// NewSTUNServerPool 创建 STUN 服务器池
func NewSTUNServerPool(logger *zap.Logger) *STUNServerPool {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &STUNServerPool{
		logger:  logger,
		servers: make([]STUNServer, 0),
		client:  NewSTUNClient(logger),
	}
}

// AddServer 添加 STUN 服务器
func (p *STUNServerPool) AddServer(server STUNServer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.servers = append(p.servers, server)
}

// QueryBest 使用最佳服务器查询
func (p *STUNServerPool) QueryBest(ctx context.Context) (*STUNResult, error) {
	p.mu.RLock()
	servers := make([]STUNServer, len(p.servers))
	copy(servers, p.servers)
	p.mu.RUnlock()

	if len(servers) == 0 {
		// 使用默认公共 STUN 服务器
		servers = []STUNServer{
			{Address: "stun.l.google.com:19302", Protocol: "udp"},
			{Address: "stun1.l.google.com:19302", Protocol: "udp"},
		}
	}

	// 并发查询所有服务器
	type queryResult struct {
		result *STUNResult
		index  int
		err    error
	}

	ch := make(chan queryResult, len(servers))
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for i, server := range servers {
		wg.Add(1)
		go func(idx int, addr string) {
			defer wg.Done()
			result, err := p.client.Query(ctx, addr)
			ch <- queryResult{result: result, index: idx, err: err}
		}(i, server.Address)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	// 返回第一个成功的结果（优先选择延迟最低的）
	var bestResult *STUNResult
	bestRTT := time.Duration(1<<63 - 1) // max duration

	for r := range ch {
		if r.err == nil && r.result != nil && r.result.RTT < bestRTT {
			bestResult = r.result
			bestRTT = r.result.RTT
			p.mu.Lock()
			p.bestIdx = r.index
			p.mu.Unlock()
		}
	}

	if bestResult == nil {
		return nil, fmt.Errorf("所有 STUN 服务器查询失败")
	}

	return bestResult, nil
}
