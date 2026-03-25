// Package natpierce 提供内网穿透服务
// STUN协议实现 - RFC 5389
package natpierce

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

// STUN消息类型.
const (
	STUNBindingRequest  uint16 = 0x0001
	STUNBindingResponse uint16 = 0x0101
	STUNBindingError    uint16 = 0x0111

	// STUN Magic Cookie.
	STUNMagicCookie uint32 = 0x2112A442

	// STUN属性类型.
	STUNAttrMappedAddress    uint16 = 0x0001
	STUNAttrXORMappedAddress uint16 = 0x0020
	STUNAttrErrorCode        uint16 = 0x0009
	STUNAttrSoftware         uint16 = 0x0022
)

// STUN错误码.
var (
	ErrSTUNTimeout       = errors.New("STUN request timeout")
	ErrSTUNResponse      = errors.New("invalid STUN response")
	ErrSTUNAttr          = errors.New("invalid STUN attribute")
	ErrSTUNAuth          = errors.New("STUN authentication failed")
	ErrSTUNServerError   = errors.New("STUN server error")
	ErrNoValidSTUNServer = errors.New("no valid STUN server available")
)

// STUNMessage STUN消息结构.
type STUNMessage struct {
	Type          uint16
	Length        uint16
	Cookie        uint32
	TransactionID [12]byte
	Attributes    []STUNAttribute
}

// STUNAttribute STUN属性.
type STUNAttribute struct {
	Type   uint16
	Length uint16
	Value  []byte
}

// STUNClient STUN客户端.
type STUNClient struct {
	timeout   time.Duration
	retries   int
	localPort int
}

// STUNResult STUN探测结果.
type STUNResult struct {
	PublicIP        net.IP
	PublicPort      int
	NATType         NATType
	ServerReflexive *net.UDPAddr
	RelayAddr       *net.UDPAddr
	RTT             time.Duration
}

// NATType NAT类型.
type NATType int

const (
	// NATTypeUnknown 未知.
	NATTypeUnknown NATType = iota
	// NATTypeNone 无NAT（公网IP）.
	NATTypeNone
	// NATTypeFullCone 完全锥形NAT.
	NATTypeFullCone
	// NATTypeRestrictedCone 受限锥形NAT.
	NATTypeRestrictedCone
	// NATTypePortRestricted 端口受限锥形NAT.
	NATTypePortRestricted
	// NATTypeSymmetric 对称NAT.
	NATTypeSymmetric
)

// String 返回NAT类型描述.
func (n NATType) String() string {
	switch n {
	case NATTypeNone:
		return "None (Public IP)"
	case NATTypeFullCone:
		return "Full Cone NAT"
	case NATTypeRestrictedCone:
		return "Restricted Cone NAT"
	case NATTypePortRestricted:
		return "Port Restricted Cone NAT"
	case NATTypeSymmetric:
		return "Symmetric NAT"
	default:
		return "Unknown NAT Type"
	}
}

// NewSTUNClient 创建STUN客户端.
func NewSTUNClient() *STUNClient {
	return &STUNClient{
		timeout:   5 * time.Second,
		retries:   3,
		localPort: 0, // 随机端口
	}
}

// WithTimeout 设置超时时间.
func (c *STUNClient) WithTimeout(timeout time.Duration) *STUNClient {
	c.timeout = timeout
	return c
}

// WithRetries 设置重试次数.
func (c *STUNClient) WithRetries(retries int) *STUNClient {
	c.retries = retries
	return c
}

// WithLocalPort 设置本地端口.
func (c *STUNClient) WithLocalPort(port int) *STUNClient {
	c.localPort = port
	return c
}

// Discover 探测公网地址.
func (c *STUNClient) Discover(serverAddr string) (*STUNResult, error) {
	// 解析服务器地址
	addr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve STUN server address: %w", err)
	}

	// 创建本地UDP连接
	var localAddr *net.UDPAddr
	if c.localPort > 0 {
		localAddr = &net.UDPAddr{Port: c.localPort}
	}

	conn, err := net.DialUDP("udp", localAddr, addr)
	if err != nil {
		return nil, fmt.Errorf("dial STUN server: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// 发送绑定请求
	start := time.Now()
	response, err := c.sendBindingRequest(conn)
	if err != nil {
		return nil, err
	}

	// 解析响应获取公网地址
	publicAddr, err := parseMappedAddress(response)
	if err != nil {
		return nil, fmt.Errorf("parse mapped address: %w", err)
	}

	rtt := time.Since(start)

	result := &STUNResult{
		PublicIP:        publicAddr.IP,
		PublicPort:      publicAddr.Port,
		ServerReflexive: publicAddr,
		RTT:             rtt,
	}

	return result, nil
}

// DiscoverWithNATType 探测公网地址并检测NAT类型.
func (c *STUNClient) DiscoverWithNATType(servers []string) (*STUNResult, error) {
	if len(servers) < 2 {
		return nil, errors.New("need at least 2 STUN servers for NAT type detection")
	}

	// 第一次探测
	result1, err := c.Discover(servers[0])
	if err != nil {
		return nil, fmt.Errorf("first STUN probe failed: %w", err)
	}

	// 检查是否公网IP
	if isPublicIP(result1.PublicIP) {
		result1.NATType = NATTypeNone
		return result1, nil
	}

	// 第二次探测（使用同一个服务器，看端口变化）
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: c.localPort})
	if err != nil {
		return nil, fmt.Errorf("listen UDP: %w", err)
	}

	// 探测第二个服务器
	addr2, err := net.ResolveUDPAddr("udp", servers[1])
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("resolve second STUN server: %w", err)
	}

	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		_ = conn.Close()
		return nil, fmt.Errorf("invalid local address type")
	}
	conn2, err := net.DialUDP("udp", localAddr, addr2)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("dial second STUN server: %w", err)
	}
	defer func() { _ = conn2.Close() }()

	response2, err := c.sendBindingRequestTo(conn2, addr2)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	publicAddr2, err := parseMappedAddress(response2)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("parse second mapped address: %w", err)
	}

	// 比较两次结果判断NAT类型
	result1.NATType = detectNATType(result1.ServerReflexive, publicAddr2)
	_ = conn.Close()

	return result1, nil
}

// sendBindingRequest 发送绑定请求.
func (c *STUNClient) sendBindingRequest(conn *net.UDPConn) (*STUNMessage, error) {
	// 创建绑定请求
	req := createBindingRequest()

	// 设置超时
	deadline := time.Now().Add(c.timeout)
	_ = conn.SetDeadline(deadline)

	// 发送请求
	reqBytes := req.Marshal()
	_, err := conn.Write(reqBytes)
	if err != nil {
		return nil, fmt.Errorf("send STUN request: %w", err)
	}

	// 接收响应
	buf := make([]byte, 1500)
	for i := 0; i < c.retries; i++ {
		n, err := conn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return nil, fmt.Errorf("read STUN response: %w", err)
		}

		// 解析响应
		resp, err := ParseSTUNMessage(buf[:n])
		if err != nil {
			continue
		}

		// 验证事务ID
		if resp.TransactionID != req.TransactionID {
			continue
		}

		// 检查响应类型
		if resp.Type != STUNBindingResponse {
			// 检查是否错误响应
			if resp.Type == STUNBindingError {
				return nil, parseSTUNError(resp)
			}
			return nil, ErrSTUNResponse
		}

		return resp, nil
	}

	return nil, ErrSTUNTimeout
}

// sendBindingRequestTo 向指定地址发送绑定请求.
func (c *STUNClient) sendBindingRequestTo(conn *net.UDPConn, addr *net.UDPAddr) (*STUNMessage, error) {
	req := createBindingRequest()

	deadline := time.Now().Add(c.timeout)
	_ = conn.SetDeadline(deadline)

	reqBytes := req.Marshal()
	_, err := conn.WriteToUDP(reqBytes, addr)
	if err != nil {
		return nil, fmt.Errorf("send STUN request: %w", err)
	}

	buf := make([]byte, 1500)
	for i := 0; i < c.retries; i++ {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return nil, fmt.Errorf("read STUN response: %w", err)
		}

		resp, err := ParseSTUNMessage(buf[:n])
		if err != nil {
			continue
		}

		if resp.TransactionID != req.TransactionID {
			continue
		}

		if resp.Type != STUNBindingResponse {
			return nil, ErrSTUNResponse
		}

		return resp, nil
	}

	return nil, ErrSTUNTimeout
}

// createBindingRequest 创建绑定请求.
func createBindingRequest() *STUNMessage {
	msg := &STUNMessage{
		Type:       STUNBindingRequest,
		Cookie:     STUNMagicCookie,
		Length:     0, // 属性长度占位
		Attributes: make([]STUNAttribute, 0),
	}

	// 生成随机事务ID
	rand.Read(msg.TransactionID[:])

	// 添加Software属性
	software := []byte("NAS-OS/1.0")
	msg.Attributes = append(msg.Attributes, STUNAttribute{
		Type:   STUNAttrSoftware,
		Length: uint16(len(software)),
		Value:  software,
	})

	// 计算属性长度
	msg.Length = 0
	for _, attr := range msg.Attributes {
		msg.Length += 4 + attr.Length
		// 对齐到4字节边界
		if attr.Length%4 != 0 {
			msg.Length += 4 - (attr.Length % 4)
		}
	}

	return msg
}

// Marshal 序列化STUN消息.
func (m *STUNMessage) Marshal() []byte {
	// 计算总长度: 20字节头 + 属性
	attrLen := 0
	for _, attr := range m.Attributes {
		attrLen += 4 + int(attr.Length)
		if attr.Length%4 != 0 {
			attrLen += 4 - (int(attr.Length) % 4)
		}
	}

	buf := make([]byte, 20+attrLen)
	offset := 0

	// 类型 (2字节)
	binary.BigEndian.PutUint16(buf[offset:], m.Type)
	offset += 2

	// 长度 (2字节)
	binary.BigEndian.PutUint16(buf[offset:], uint16(attrLen))
	offset += 2

	// Magic Cookie (4字节)
	binary.BigEndian.PutUint32(buf[offset:], m.Cookie)
	offset += 4

	// 事务ID (12字节)
	copy(buf[offset:], m.TransactionID[:])
	offset += 12

	// 属性
	for _, attr := range m.Attributes {
		// 类型 (2字节)
		binary.BigEndian.PutUint16(buf[offset:], attr.Type)
		offset += 2

		// 长度 (2字节)
		binary.BigEndian.PutUint16(buf[offset:], attr.Length)
		offset += 2

		// 值
		copy(buf[offset:], attr.Value)
		offset += int(attr.Length)

		// 填充到4字节边界
		if attr.Length%4 != 0 {
			padding := 4 - (attr.Length % 4)
			offset += int(padding)
		}
	}

	return buf
}

// ParseSTUNMessage 解析STUN消息.
func ParseSTUNMessage(data []byte) (*STUNMessage, error) {
	if len(data) < 20 {
		return nil, ErrSTUNResponse
	}

	msg := &STUNMessage{
		Attributes: make([]STUNAttribute, 0),
	}

	offset := 0

	// 类型
	msg.Type = binary.BigEndian.Uint16(data[offset:])
	offset += 2

	// 长度
	msg.Length = binary.BigEndian.Uint16(data[offset:])
	offset += 2

	// 验证Magic Cookie
	msg.Cookie = binary.BigEndian.Uint32(data[offset:])
	if msg.Cookie != STUNMagicCookie {
		return nil, fmt.Errorf("%w: invalid magic cookie", ErrSTUNResponse)
	}
	offset += 4

	// 事务ID
	copy(msg.TransactionID[:], data[offset:])
	offset += 12

	// 解析属性
	for offset < len(data) && offset < int(msg.Length)+20 {
		if offset+4 > len(data) {
			break
		}

		attr := STUNAttribute{}
		attr.Type = binary.BigEndian.Uint16(data[offset:])
		offset += 2
		attr.Length = binary.BigEndian.Uint16(data[offset:])
		offset += 2

		if offset+int(attr.Length) > len(data) {
			return nil, ErrSTUNAttr
		}

		attr.Value = make([]byte, attr.Length)
		copy(attr.Value, data[offset:])
		offset += int(attr.Length)

		// 跳过填充
		if attr.Length%4 != 0 {
			offset += 4 - (int(attr.Length) % 4)
		}

		msg.Attributes = append(msg.Attributes, attr)
	}

	return msg, nil
}

// parseMappedAddress 解析映射地址属性.
func parseMappedAddress(msg *STUNMessage) (*net.UDPAddr, error) {
	for _, attr := range msg.Attributes {
		if attr.Type == STUNAttrXORMappedAddress {
			return parseXORMappedAddress(attr.Value, msg.TransactionID[:])
		}
		if attr.Type == STUNAttrMappedAddress {
			return parseMappedAddressValue(attr.Value)
		}
	}
	return nil, fmt.Errorf("%w: no mapped address found", ErrSTUNAttr)
}

// parseMappedAddressValue 解析普通映射地址.
func parseMappedAddressValue(data []byte) (*net.UDPAddr, error) {
	if len(data) < 8 {
		return nil, ErrSTUNAttr
	}

	// 第一个字节保留，第二个字节是地址族
	family := data[1]
	var ip net.IP
	var port uint16

	switch family {
	case 0x01: // IPv4
		if len(data) < 8 {
			return nil, ErrSTUNAttr
		}
		port = binary.BigEndian.Uint16(data[2:4])
		ip = net.IP(data[4:8])
	case 0x02: // IPv6
		if len(data) < 20 {
			return nil, ErrSTUNAttr
		}
		port = binary.BigEndian.Uint16(data[2:4])
		ip = net.IP(data[4:20])
	default:
		return nil, fmt.Errorf("%w: unknown address family %d", ErrSTUNAttr, family)
	}

	return &net.UDPAddr{
		IP:   ip,
		Port: int(port),
	}, nil
}

// parseXORMappedAddress 解析XOR映射地址.
func parseXORMappedAddress(data []byte, transactionID []byte) (*net.UDPAddr, error) {
	if len(data) < 4 {
		return nil, ErrSTUNAttr
	}

	family := data[0] & 0x01 // 最低位指示地址族
	var port uint16
	var ip net.IP

	// XOR端口 (使用Magic Cookie高16位)
	xorPort := binary.BigEndian.Uint16(data[2:4])
	port = xorPort ^ uint16(STUNMagicCookie>>16)

	switch family {
	case 0x01: // IPv4
		if len(data) < 8 {
			return nil, ErrSTUNAttr
		}
		// XOR IP地址 (使用Magic Cookie)
		xorIP := binary.BigEndian.Uint32(data[4:8])
		ip = make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, xorIP^STUNMagicCookie)
	case 0x02: // IPv6
		if len(data) < 20 {
			return nil, ErrSTUNAttr
		}
		// XOR IP地址 (使用Magic Cookie + TransactionID)
		ip = make(net.IP, 16)
		for i := 0; i < 16; i++ {
			if i < 4 {
				ip[i] = data[4+i] ^ byte(STUNMagicCookie>>((3-i)*8))
			} else {
				ip[i] = data[4+i] ^ transactionID[i-4]
			}
		}
	default:
		return nil, fmt.Errorf("%w: unknown address family", ErrSTUNAttr)
	}

	return &net.UDPAddr{
		IP:   ip,
		Port: int(port),
	}, nil
}

// parseSTUNError 解析STUN错误响应.
func parseSTUNError(msg *STUNMessage) error {
	for _, attr := range msg.Attributes {
		if attr.Type == STUNAttrErrorCode {
			if len(attr.Value) < 4 {
				continue
			}
			class := attr.Value[2] & 0x07
			number := attr.Value[3]
			code := int(class)*100 + int(number)
			reason := string(attr.Value[4:])

			switch code {
			case 400:
				return fmt.Errorf("STUN bad request: %s", reason)
			case 401:
				return ErrSTUNAuth
			case 420:
				return fmt.Errorf("STUN unknown attribute: %s", reason)
			case 438:
				return fmt.Errorf("STUN stale nonce: %s", reason)
			case 500:
				return ErrSTUNServerError
			default:
				return fmt.Errorf("STUN error %d: %s", code, reason)
			}
		}
	}
	return ErrSTUNResponse
}

// detectNATType 检测NAT类型.
func detectNATType(addr1, addr2 *net.UDPAddr) NATType {
	if addr1 == nil || addr2 == nil {
		return NATTypeUnknown
	}

	// 如果两个地址完全相同，可能是锥形NAT
	if addr1.IP.Equal(addr2.IP) && addr1.Port == addr2.Port {
		// 需要进一步测试才能确定是哪种锥形NAT
		// 这里简化处理，返回受限锥形
		return NATTypeRestrictedCone
	}

	// 如果IP相同但端口不同，可能是端口受限锥形NAT
	if addr1.IP.Equal(addr2.IP) && addr1.Port != addr2.Port {
		return NATTypePortRestricted
	}

	// 如果IP或端口都不同，是对称NAT
	return NATTypeSymmetric
}

// isPublicIP 检查是否公网IP.
func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return false
	}
	// 检查运营商级NAT范围 (100.64.0.0/10)
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return false
		}
	}
	return true
}
