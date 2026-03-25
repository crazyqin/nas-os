// Package tunnel 提供内网穿透服务 - TURN 协议实现
package tunnel

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

// TURN 消息类型 (RFC 5766)
const (
	TurnMessageTypeAllocate                 uint16 = 0x0003
	TurnMessageTypeAllocateResponse         uint16 = 0x0103
	TurnMessageTypeAllocateError            uint16 = 0x0113
	TurnMessageTypeCreatePermission         uint16 = 0x0008
	TurnMessageTypeCreatePermissionResponse uint16 = 0x0108
	TurnMessageTypeChannelBind              uint16 = 0x0009
	TurnMessageTypeChannelBindResponse      uint16 = 0x0109
	TurnMessageTypeSend                     uint16 = 0x0006
	TurnMessageTypeData                     uint16 = 0x0007
)

// TURN 属性类型
const (
	TurnAttrRequestedTransport uint16 = 0x0019
	TurnAttrXORRelayedAddress  uint16 = 0x0016
	TurnAttrLifetime           uint16 = 0x000D
	TurnAttrXORPeerAddress     uint16 = 0x0012
	TurnAttrData               uint16 = 0x0013
	TurnAttrChannelNumber      uint16 = 0x000C
	TurnAttrNonce              uint16 = 0x0014
	TurnAttrRealm              uint16 = 0x0014
	TurnAttrUsername           uint16 = 0x0006
	TurnAttrMessageIntegrity   uint16 = 0x0008
)

// TURN 传输协议
const (
	TurnTransportUDP = 17
	TurnTransportTCP = 6
)

// TURNClientConfig TURN 客户端配置
type TURNClientConfig struct {
	Server   string
	Username string
	Password string
	Realm    string
	Nonce    string
	Timeout  time.Duration
}

// TURNAllocation TURN 分配信息
type TURNAllocation struct {
	RelayedAddr *net.UDPAddr
	Channel     uint16
	Lifetime    time.Duration
	ExpiresAt   time.Time
}

// TURNProtocol TURN 协议实现
type TURNProtocol struct {
	config      TURNClientConfig
	logger      *zap.Logger
	conn        *net.UDPConn
	mu          sync.RWMutex
	allocation  *TURNAllocation
	credentials struct {
		realm    string
		nonce    string
		username string
		password string
	}
}

// NewTURNProtocol 创建 TURN 协议实例
func NewTURNProtocol(config TURNClientConfig, logger *zap.Logger) *TURNProtocol {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	return &TURNProtocol{
		config: config,
		logger: logger,
	}
}

// Connect 连接到 TURN 服务器
func (t *TURNProtocol) Connect(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", t.config.Server)
	if err != nil {
		return fmt.Errorf("failed to resolve TURN server: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("failed to dial TURN server: %w", err)
	}

	t.conn = conn
	return nil
}

// Close 关闭连接
func (t *TURNProtocol) Close() error {
	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}

// Allocate 分配中继地址
func (t *TURNProtocol) Allocate(ctx context.Context) (*net.UDPAddr, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// 创建 Allocate 请求
	msg := &STUNMessage{
		Header: STUNHeader{
			MessageType:   TurnMessageTypeAllocate,
			MessageLength: 0,
			MagicCookie:   StunMagicCookie,
		},
		Attributes: []STUNAttribute{
			{
				Type:   TurnAttrRequestedTransport,
				Length: 4,
				Value:  []byte{0, 0, 0, TurnTransportUDP},
			},
		},
	}

	// 生成 Transaction ID
	if _, err := rand.Read(msg.Header.TransactionID[:]); err != nil {
		return nil, err
	}

	// 如果有认证信息，添加完整性校验
	if t.credentials.username != "" {
		t.addMessageIntegrity(msg)
	}

	// 发送请求
	resp, err := t.sendRequest(ctx, msg)
	if err != nil {
		return nil, err
	}

	// 检查是否需要认证
	if resp.Header.MessageType == TurnMessageTypeAllocateError {
		// 提取 realm 和 nonce
		for _, attr := range resp.Attributes {
			if attr.Type == TurnAttrRealm {
				t.credentials.realm = string(attr.Value)
			}
			if attr.Type == TurnAttrNonce {
				t.credentials.nonce = string(attr.Value)
			}
		}

		// 重新发送带认证的请求
		t.credentials.username = t.config.Username
		t.credentials.password = t.config.Password

		msg2 := &STUNMessage{
			Header: STUNHeader{
				MessageType:   TurnMessageTypeAllocate,
				MessageLength: 0,
				MagicCookie:   StunMagicCookie,
			},
			Attributes: []STUNAttribute{
				{
					Type:   TurnAttrRequestedTransport,
					Length: 4,
					Value:  []byte{0, 0, 0, TurnTransportUDP},
				},
			},
		}
		if _, err := rand.Read(msg2.Header.TransactionID[:]); err != nil {
			return nil, err
		}
		t.addMessageIntegrity(msg2)

		resp, err = t.sendRequest(ctx, msg2)
		if err != nil {
			return nil, err
		}
	}

	if resp.Header.MessageType == TurnMessageTypeAllocateError {
		return nil, errors.New("allocate failed")
	}

	// 解析中继地址
	for _, attr := range resp.Attributes {
		if attr.Type == TurnAttrXORRelayedAddress {
			stun := NewSTUNProtocol(nil, t.logger)
			ip, port, err := stun.ParseXORMappedAddress(&attr, resp.Header.TransactionID)
			if err != nil {
				return nil, err
			}
			addr := &net.UDPAddr{
				IP:   ip,
				Port: port,
			}
			t.allocation = &TURNAllocation{
				RelayedAddr: addr,
				ExpiresAt:   time.Now().Add(10 * time.Minute),
			}
			return addr, nil
		}
	}

	return nil, errors.New("no relayed address in response")
}

// CreatePermission 创建对端权限
func (t *TURNProtocol) CreatePermission(ctx context.Context, peer *net.UDPAddr) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.allocation == nil {
		return errors.New("no active allocation")
	}

	// 创建 CreatePermission 请求
	msg := &STUNMessage{
		Header: STUNHeader{
			MessageType:   TurnMessageTypeCreatePermission,
			MessageLength: 0,
			MagicCookie:   StunMagicCookie,
		},
		Attributes: []STUNAttribute{
			t.createXORPeerAddress(peer),
		},
	}

	if _, err := rand.Read(msg.Header.TransactionID[:]); err != nil {
		return err
	}

	t.addMessageIntegrity(msg)

	resp, err := t.sendRequest(ctx, msg)
	if err != nil {
		return err
	}

	if resp.Header.MessageType != TurnMessageTypeCreatePermissionResponse {
		return errors.New("create permission failed")
	}

	return nil
}

// BindChannel 绑定通道
func (t *TURNProtocol) BindChannel(ctx context.Context, peer *net.UDPAddr) (uint16, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.allocation == nil {
		return 0, errors.New("no active allocation")
	}

	// 分配通道号 (0x4000 - 0x7FFF)
	channel := 0x4000 + t.allocation.Channel
	t.allocation.Channel++

	// 创建 ChannelBind 请求
	msg := &STUNMessage{
		Header: STUNHeader{
			MessageType:   TurnMessageTypeChannelBind,
			MessageLength: 0,
			MagicCookie:   StunMagicCookie,
		},
		Attributes: []STUNAttribute{
			t.createXORPeerAddress(peer),
			{
				Type:   TurnAttrChannelNumber,
				Length: 4,
				Value:  []byte{byte((channel >> 8) & 0xFF), byte(channel & 0xFF), 0, 0},
			},
		},
	}

	if _, err := rand.Read(msg.Header.TransactionID[:]); err != nil {
		return 0, err
	}

	t.addMessageIntegrity(msg)

	resp, err := t.sendRequest(ctx, msg)
	if err != nil {
		return 0, err
	}

	if resp.Header.MessageType != TurnMessageTypeChannelBindResponse {
		return 0, errors.New("channel bind failed")
	}

	return channel, nil
}

// SendTo 发送数据到对端
func (t *TURNProtocol) SendTo(ctx context.Context, data []byte, peer *net.UDPAddr) (int, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.allocation == nil {
		return 0, errors.New("no active allocation")
	}

	// 检查是否绑定通道
	if t.allocation.Channel > 0 {
		// 使用 ChannelData 格式发送
		return t.sendChannelData(data, peer)
	}

	// 使用 Send 指令发送
	msg := &STUNMessage{
		Header: STUNHeader{
			MessageType:   TurnMessageTypeSend,
			MessageLength: 0,
			MagicCookie:   StunMagicCookie,
		},
		Attributes: []STUNAttribute{
			t.createXORPeerAddress(peer),
			{
				Type:   TurnAttrData,
				Length: uint16(len(data)),
				Value:  data,
			},
		},
	}

	if _, err := rand.Read(msg.Header.TransactionID[:]); err != nil {
		return 0, err
	}

	t.addMessageIntegrity(msg)

	// 编码并发送
	stun := NewSTUNProtocol(nil, t.logger)
	encoded := stun.Encode(msg)
	return t.conn.Write(encoded)
}

// ReceiveFrom 接收数据
func (t *TURNProtocol) ReceiveFrom(ctx context.Context) ([]byte, *net.UDPAddr, error) {
	buf := make([]byte, 1500)
	n, err := t.conn.Read(buf)
	if err != nil {
		return nil, nil, err
	}

	data := buf[:n]

	// 检查是否是 ChannelData
	if len(data) >= 4 && (data[0]&0xC0) == 0x40 {
		// ChannelData 格式
		_ = binary.BigEndian.Uint16(data[0:2]) // channel number (unused in simplified implementation)
		length := binary.BigEndian.Uint16(data[2:4])
		if len(data) >= 4+int(length) {
			// 从通道号推断对端地址（简化实现）
			return data[4 : 4+length], nil, nil
		}
	}

	// 解析 STUN 消息
	stun := NewSTUNProtocol(nil, t.logger)
	msg, err := stun.Decode(data)
	if err != nil {
		return nil, nil, err
	}

	// 查找 DATA 属性
	for _, attr := range msg.Attributes {
		if attr.Type == TurnAttrData {
			// 查找 XOR-PEER-ADDRESS
			var peerAddr *net.UDPAddr
			for _, attr2 := range msg.Attributes {
				if attr2.Type == TurnAttrXORPeerAddress {
					ip, port, err := stun.ParseXORMappedAddress(&attr2, msg.Header.TransactionID)
					if err == nil {
						peerAddr = &net.UDPAddr{IP: ip, Port: port}
					}
					break
				}
			}
			return attr.Value, peerAddr, nil
		}
	}

	return nil, nil, errors.New("no data in message")
}

// sendChannelData 通过通道发送数据
func (t *TURNProtocol) sendChannelData(data []byte, peer *net.UDPAddr) (int, error) {
	// 简化实现：直接使用 Send 指令
	// 完整实现应该使用已绑定的通道号
	return 0, errors.New("channel data not implemented")
}

// sendRequest 发送请求并等待响应
func (t *TURNProtocol) sendRequest(ctx context.Context, msg *STUNMessage) (*STUNMessage, error) {
	stun := NewSTUNProtocol(nil, t.logger)
	data := stun.Encode(msg)

	if _, err := t.conn.Write(data); err != nil {
		return nil, err
	}

	// 设置读取超时
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(t.config.Timeout)
	}
	_ = t.conn.SetReadDeadline(deadline)

	// 读取响应
	respBuf := make([]byte, 1500)
	n, err := t.conn.Read(respBuf)
	if err != nil {
		return nil, err
	}

	resp, err := stun.Decode(respBuf[:n])
	if err != nil {
		return nil, err
	}

	// 验证 Transaction ID 匹配
	if resp.Header.TransactionID != msg.Header.TransactionID {
		return nil, errors.New("transaction ID mismatch")
	}

	return resp, nil
}

// addMessageIntegrity 添加消息完整性校验
func (t *TURNProtocol) addMessageIntegrity(msg *STUNMessage) {
	// 添加 USERNAME 属性
	if t.credentials.username != "" {
		msg.Attributes = append(msg.Attributes, STUNAttribute{
			Type:   TurnAttrUsername,
			Length: uint16(len(t.credentials.username)),
			Value:  []byte(t.credentials.username),
		})
	}

	// 添加 REALM 属性
	if t.credentials.realm != "" {
		msg.Attributes = append(msg.Attributes, STUNAttribute{
			Type:   TurnAttrRealm,
			Length: uint16(len(t.credentials.realm)),
			Value:  []byte(t.credentials.realm),
		})
	}

	// 添加 NONCE 属性
	if t.credentials.nonce != "" {
		msg.Attributes = append(msg.Attributes, STUNAttribute{
			Type:   TurnAttrNonce,
			Length: uint16(len(t.credentials.nonce)),
			Value:  []byte(t.credentials.nonce),
		})
	}

	// 计算 MESSAGE-INTEGRITY
	stun := NewSTUNProtocol(nil, t.logger)
	encoded := stun.Encode(msg)

	// 计算长期凭证 HMAC-SHA1
	key := t.credentials.username + ":" + t.credentials.realm + ":" + t.credentials.password
	mac := hmac.New(sha1.New, []byte(key))
	mac.Write(encoded)
	integrity := mac.Sum(nil)

	msg.Attributes = append(msg.Attributes, STUNAttribute{
		Type:   TurnAttrMessageIntegrity,
		Length: 20,
		Value:  integrity,
	})
}

// createXORPeerAddress 创建 XOR-PEER-ADDRESS 属性
func (t *TURNProtocol) createXORPeerAddress(peer *net.UDPAddr) STUNAttribute {
	ip := peer.IP
	port := peer.Port

	// IPv4
	if ip.To4() != nil {
		ip = ip.To4()
		value := make([]byte, 8)
		// Family (IPv4 = 0x01)
		binary.BigEndian.PutUint16(value[0:2], 0x0001)
		// XOR Port
		binary.BigEndian.PutUint16(value[2:4], uint16(port)^0x2112)
		// XOR IP
		for i := 0; i < 4; i++ {
			value[4+i] = ip[i] ^ 0x21
		}
		return STUNAttribute{
			Type:   TurnAttrXORPeerAddress,
			Length: 8,
			Value:  value,
		}
	}

	// IPv6
	value := make([]byte, 20)
	binary.BigEndian.PutUint16(value[0:2], 0x0002)
	binary.BigEndian.PutUint16(value[2:4], uint16(port)^0x2112)
	for i := 0; i < 16; i++ {
		value[4+i] = ip[i] ^ 0x21
	}
	return STUNAttribute{
		Type:   TurnAttrXORPeerAddress,
		Length: 20,
		Value:  value,
	}
}
