// Package tunnel 提供内网穿透服务 - STUN 协议实现
package tunnel

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

// STUN 消息类型 (RFC 5389)
const (
	StunMessageTypeBindingRequest  uint16 = 0x0001
	StunMessageTypeBindingResponse uint16 = 0x0101
	StunMessageTypeBindingError    uint16 = 0x0111
)

// STUN 属性类型 (RFC 5389)
const (
	StunAttrMappedAddress     uint16 = 0x0001
	StunAttrXORMappedAddress  uint16 = 0x0020
	StunAttrResponseOrigin    uint16 = 0x0002
	StunAttrOtherAddress      uint16 = 0x0003
	StunAttrErrorCode         uint16 = 0x0009
	StunAttrUnknownAttributes uint16 = 0x000A
	StunAttrMessageIntegrity  uint16 = 0x0008
	StunAttrFingerprint       uint16 = 0x8028
)

// STUN 魔数
const StunMagicCookie uint32 = 0x2112A442

// STUNHeader STUN 消息头
type STUNHeader struct {
	MessageType   uint16
	MessageLength uint16
	MagicCookie   uint32
	TransactionID [12]byte
}

// STUNAttribute STUN 属性
type STUNAttribute struct {
	Type   uint16
	Length uint16
	Value  []byte
}

// STUNMessage STUN 消息
type STUNMessage struct {
	Header     STUNHeader
	Attributes []STUNAttribute
}

// STUNResult STUN 检测结果
type STUNResult struct {
	PublicIP   net.IP
	PublicPort int
	NATType    NATType
	ServerAddr string
}

// STUNProtocol STUN 协议实现
type STUNProtocol struct {
	servers []string
	logger  *zap.Logger
	mu      sync.RWMutex
	timeout time.Duration
}

// NewSTUNProtocol 创建 STUN 协议实例
func NewSTUNProtocol(servers []string, logger *zap.Logger) *STUNProtocol {
	if len(servers) == 0 {
		servers = []string{
			"stun.l.google.com:19302",
			"stun1.l.google.com:19302",
			"stun2.l.google.com:19302",
		}
	}
	return &STUNProtocol{
		servers: servers,
		logger:  logger,
		timeout: 5 * time.Second,
	}
}

// CreateBindingRequest 创建 Binding Request 消息
func (s *STUNProtocol) CreateBindingRequest() (*STUNMessage, error) {
	msg := &STUNMessage{
		Header: STUNHeader{
			MessageType:   StunMessageTypeBindingRequest,
			MessageLength: 0,
			MagicCookie:   StunMagicCookie,
		},
		Attributes: make([]STUNAttribute, 0),
	}

	// 生成随机 Transaction ID
	if _, err := rand.Read(msg.Header.TransactionID[:]); err != nil {
		return nil, fmt.Errorf("failed to generate transaction ID: %w", err)
	}

	return msg, nil
}

// Encode 编码 STUN 消息
func (s *STUNProtocol) Encode(msg *STUNMessage) []byte {
	// 计算属性总长度
	var attrLen uint16
	for _, attr := range msg.Attributes {
		attrLen += 4 + uint16(len(attr.Value))
		// 对齐到 4 字节
		padding := (4 - len(attr.Value)%4) % 4
		attrLen += uint16(padding)
	}
	msg.Header.MessageLength = attrLen

	// 编码头部
	buf := make([]byte, 20+attrLen)
	binary.BigEndian.PutUint16(buf[0:2], msg.Header.MessageType)
	binary.BigEndian.PutUint16(buf[2:4], msg.Header.MessageLength)
	binary.BigEndian.PutUint32(buf[4:8], msg.Header.MagicCookie)
	copy(buf[8:20], msg.Header.TransactionID[:])

	// 编码属性
	offset := 20
	for _, attr := range msg.Attributes {
		binary.BigEndian.PutUint16(buf[offset:offset+2], attr.Type)
		binary.BigEndian.PutUint16(buf[offset+2:offset+4], uint16(len(attr.Value)))
		copy(buf[offset+4:], attr.Value)
		offset += 4 + len(attr.Value)

		// 添加填充
		padding := (4 - len(attr.Value)%4) % 4
		for i := 0; i < padding; i++ {
			buf[offset] = 0
			offset++
		}
	}

	return buf
}

// Decode 解码 STUN 消息
func (s *STUNProtocol) Decode(data []byte) (*STUNMessage, error) {
	if len(data) < 20 {
		return nil, errors.New("message too short")
	}

	msg := &STUNMessage{
		Header: STUNHeader{
			MessageType:   binary.BigEndian.Uint16(data[0:2]),
			MessageLength: binary.BigEndian.Uint16(data[2:4]),
			MagicCookie:   binary.BigEndian.Uint32(data[4:8]),
		},
		Attributes: make([]STUNAttribute, 0),
	}

	// 验证魔数
	if msg.Header.MagicCookie != StunMagicCookie {
		return nil, errors.New("invalid magic cookie")
	}

	// 复制 Transaction ID
	copy(msg.Header.TransactionID[:], data[8:20])

	// 解析属性
	offset := 20
	for offset < len(data) {
		if offset+4 > len(data) {
			break
		}

		attr := STUNAttribute{
			Type:   binary.BigEndian.Uint16(data[offset : offset+2]),
			Length: binary.BigEndian.Uint16(data[offset+2 : offset+4]),
		}

		if offset+4+int(attr.Length) > len(data) {
			break
		}

		attr.Value = make([]byte, attr.Length)
		copy(attr.Value, data[offset+4:offset+4+int(attr.Length)])
		msg.Attributes = append(msg.Attributes, attr)

		// 跳过属性和填充
		offset += 4 + int(attr.Length)
		padding := (4 - int(attr.Length)%4) % 4
		offset += padding
	}

	return msg, nil
}

// ParseXORMappedAddress 解析 XOR-MAPPED-ADDRESS 属性
func (s *STUNProtocol) ParseXORMappedAddress(attr *STUNAttribute, transactionID [12]byte) (net.IP, int, error) {
	if len(attr.Value) < 4 {
		return nil, 0, errors.New("attribute too short")
	}

	// 地址族: 0x01 = IPv4, 0x02 = IPv6
	family := binary.BigEndian.Uint16(attr.Value[0:2]) ^ 0x0001

	var ip net.IP
	var port int

	if family == 0x0001 {
		// IPv4
		if len(attr.Value) != 8 {
			return nil, 0, errors.New("invalid IPv4 address length")
		}
		port = int(binary.BigEndian.Uint16(attr.Value[2:4]) ^ uint16(transactionID[0])<<8 ^ uint16(transactionID[1]))
		ip = make(net.IP, 4)
		for i := 0; i < 4; i++ {
			ip[i] = attr.Value[4+i] ^ transactionID[i]
		}
	} else if family == 0x0002 {
		// IPv6
		if len(attr.Value) != 20 {
			return nil, 0, errors.New("invalid IPv6 address length")
		}
		xorPort := binary.BigEndian.Uint16(attr.Value[2:4])
		port = int(xorPort ^ uint16(transactionID[0])<<8 ^ uint16(transactionID[1]))
		ip = make(net.IP, 16)
		xorBytes := make([]byte, 16)
		binary.BigEndian.PutUint32(xorBytes[0:4], StunMagicCookie)
		copy(xorBytes[4:16], transactionID[:])
		for i := 0; i < 16; i++ {
			ip[i] = attr.Value[4+i] ^ xorBytes[i]
		}
	} else {
		return nil, 0, fmt.Errorf("unknown address family: %d", family)
	}

	return ip, port, nil
}

// ParseMappedAddress 解析 MAPPED-ADDRESS 属性
func (s *STUNProtocol) ParseMappedAddress(attr *STUNAttribute) (net.IP, int, error) {
	if len(attr.Value) < 4 {
		return nil, 0, errors.New("attribute too short")
	}

	family := binary.BigEndian.Uint16(attr.Value[0:2])

	var ip net.IP
	var port int

	if family == 0x0001 {
		// IPv4
		if len(attr.Value) != 8 {
			return nil, 0, errors.New("invalid IPv4 address length")
		}
		port = int(binary.BigEndian.Uint16(attr.Value[2:4]))
		ip = make(net.IP, 4)
		copy(ip, attr.Value[4:8])
	} else if family == 0x0002 {
		// IPv6
		if len(attr.Value) != 20 {
			return nil, 0, errors.New("invalid IPv6 address length")
		}
		port = int(binary.BigEndian.Uint16(attr.Value[2:4]))
		ip = make(net.IP, 16)
		copy(ip, attr.Value[4:20])
	} else {
		return nil, 0, fmt.Errorf("unknown address family: %d", family)
	}

	return ip, port, nil
}

// Discover 发现公网地址
func (s *STUNProtocol) Discover(ctx context.Context, server string) (*STUNResult, error) {
	// 解析服务器地址
	addr, err := net.ResolveUDPAddr("udp", server)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve STUN server: %w", err)
	}

	// 创建 UDP 连接
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial STUN server: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// 设置超时
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(s.timeout)
	}
	_ = conn.SetDeadline(deadline)

	// 创建 Binding Request
	req, err := s.CreateBindingRequest()
	if err != nil {
		return nil, err
	}

	// 发送请求
	reqData := s.Encode(req)
	if _, err := conn.Write(reqData); err != nil {
		return nil, fmt.Errorf("failed to send STUN request: %w", err)
	}

	// 接收响应
	respData := make([]byte, 1500)
	n, err := conn.Read(respData)
	if err != nil {
		return nil, fmt.Errorf("failed to read STUN response: %w", err)
	}

	// 解码响应
	resp, err := s.Decode(respData[:n])
	if err != nil {
		return nil, fmt.Errorf("failed to decode STUN response: %w", err)
	}

	// 验证响应类型
	if resp.Header.MessageType != StunMessageTypeBindingResponse {
		return nil, fmt.Errorf("unexpected response type: 0x%04x", resp.Header.MessageType)
	}

	// 查找地址属性
	for _, attr := range resp.Attributes {
		if attr.Type == StunAttrXORMappedAddress {
			ip, port, err := s.ParseXORMappedAddress(&attr, resp.Header.TransactionID)
			if err != nil {
				continue
			}
			return &STUNResult{
				PublicIP:   ip,
				PublicPort: port,
				NATType:    NATTypeUnknown, // 需要多次测试确定
				ServerAddr: server,
			}, nil
		}
		if attr.Type == StunAttrMappedAddress {
			ip, port, err := s.ParseMappedAddress(&attr)
			if err != nil {
				continue
			}
			return &STUNResult{
				PublicIP:   ip,
				PublicPort: port,
				NATType:    NATTypeUnknown,
				ServerAddr: server,
			}, nil
		}
	}

	return nil, errors.New("no address attribute found in response")
}

// DetectNATType 检测 NAT 类型 (RFC 3489)
func (s *STUNProtocol) DetectNATType(ctx context.Context) (NATType, string, int, error) {
	if len(s.servers) < 2 {
		return NATTypeUnknown, "", 0, errors.New("need at least 2 STUN servers for NAT detection")
	}

	// 第一阶段：获取映射地址
	result1, err := s.Discover(ctx, s.servers[0])
	if err != nil {
		return NATTypeUnknown, "", 0, err
	}

	localAddr, err := s.getLocalIP()
	if err != nil {
		return NATTypeUnknown, "", 0, err
	}

	// 如果映射地址和本地地址相同，说明没有 NAT
	if result1.PublicIP.Equal(localAddr) {
		return NATTypeNone, result1.PublicIP.String(), result1.PublicPort, nil
	}

	// 第二阶段：使用不同 STUN 服务器测试
	result2, err := s.Discover(ctx, s.servers[1])
	if err != nil {
		// 单服务器无法完全确定，假设为端口受限锥形
		return NATTypePortRestrictedCone, result1.PublicIP.String(), result1.PublicPort, nil
	}

	// 如果映射地址不同，说明是对称型 NAT
	if !result1.PublicIP.Equal(result2.PublicIP) || result1.PublicPort != result2.PublicPort {
		return NATTypeSymmetric, result1.PublicIP.String(), result1.PublicPort, nil
	}

	// 简化处理：默认为端口受限锥形 NAT
	// 完整实现需要更多测试步骤
	return NATTypePortRestrictedCone, result1.PublicIP.String(), result1.PublicPort, nil
}

// getLocalIP 获取本地 IP
func (s *STUNProtocol) getLocalIP() (net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP, nil
			}
		}
	}

	return nil, errors.New("no local IP found")
}

// ProbeMultiple 探测多个 STUN 服务器
func (s *STUNProtocol) ProbeMultiple(ctx context.Context) []*STUNResult {
	results := make([]*STUNResult, 0, len(s.servers))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, server := range s.servers {
		wg.Add(1)
		go func(srv string) {
			defer wg.Done()
			result, err := s.Discover(ctx, srv)
			if err != nil {
				s.logger.Debug("STUN probe failed", zap.String("server", srv), zap.Error(err))
				return
			}
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(server)
	}

	wg.Wait()
	return results
}
