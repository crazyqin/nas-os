// Package frp provides FRP client implementation
// FRP消息协议定义
package frp

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

// MessageType 消息类型
type MessageType uint16

const (
	// 控制消息
	MsgTypeAuth     MessageType = 0
	MsgTypeAuthResp MessageType = 1
	MsgTypePing     MessageType = 2
	MsgTypePong     MessageType = 3
	MsgTypeClose    MessageType = 4

	// 隧道消息
	MsgTypeNewProxy     MessageType = 10
	MsgTypeNewProxyResp MessageType = 11
	MsgTypeCloseProxy   MessageType = 12
	MsgTypeNewWorkConn  MessageType = 13
	MsgTypeReqWorkConn  MessageType = 14

	// 数据消息
	MsgTypeData      MessageType = 20
	MsgTypeUDPPacket MessageType = 21
)

// Message FRP消息结构
type Message struct {
	Type MessageType `json:"type"`
	Len  uint64      `json:"len"`
	Data []byte      `json:"data"`
}

// AuthRequest 认证请求
type AuthRequest struct {
	Version   string            `json:"version"`
	Token     string            `json:"token"`
	Timestamp int64             `json:"timestamp"`
	RunID     string            `json:"run_id"`
	Metas     map[string]string `json:"metas,omitempty"`
}

// AuthResponse 认证响应
type AuthResponse struct {
	Version    string `json:"version"`
	RunID      string `json:"run_id"`
	Error      string `json:"error,omitempty"`
	ServerAddr string `json:"server_addr,omitempty"`
}

// TunnelRequest 隧道请求
type TunnelRequest struct {
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	LocalIP        string            `json:"local_ip"`
	LocalPort      int               `json:"local_port"`
	RemotePort     int               `json:"remote_port,omitempty"`
	SubDomain      string            `json:"sub_domain,omitempty"`
	CustomDomains  []string          `json:"custom_domains,omitempty"`
	Locations      []string          `json:"locations,omitempty"`
	HTTPUser       string            `json:"http_user,omitempty"`
	HTTPPwd        string            `json:"http_pwd,omitempty"`
	Sk             string            `json:"sk,omitempty"`
	BandwidthLimit string            `json:"bandwidth_limit,omitempty"`
	Metas          map[string]string `json:"metas,omitempty"`
}

// TunnelResponse 隧道响应
type TunnelResponse struct {
	ProxyName  string `json:"proxy_name"`
	RemoteAddr string `json:"remote_addr,omitempty"`
	Error      string `json:"error,omitempty"`
	ProxyType  string `json:"proxy_type"`
}

// DataMessage 数据消息
type DataMessage struct {
	ProxyName string `json:"proxy_name"`
	Data      []byte `json:"data"`
}

// WorkConnRequest 工作连接请求
type WorkConnRequest struct {
	RunID     string `json:"run_id"`
	ProxyName string `json:"proxy_name"`
}

// EncodeMessage 编码消息
func EncodeMessage(msgType MessageType, payload interface{}) ([]byte, error) {
	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
	}

	// 消息头: 2字节类型 + 8字节长度
	header := make([]byte, 10)
	binary.BigEndian.PutUint16(header[0:2], uint16(msgType))
	binary.BigEndian.PutUint64(header[2:10], uint64(len(body)))

	if len(body) > 0 {
		return append(header, body...), nil
	}
	return header, nil
}

// DecodeMessage 解码消息体
func DecodeMessage(msg *Message) (interface{}, error) {
	if len(msg.Data) == 0 {
		return nil, nil
	}

	switch msg.Type {
	case MsgTypeAuth:
		var req AuthRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			return nil, err
		}
		return req, nil

	case MsgTypeAuthResp:
		var resp AuthResponse
		if err := json.Unmarshal(msg.Data, &resp); err != nil {
			return nil, err
		}
		return resp, nil

	case MsgTypeNewProxy:
		var req TunnelRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			return nil, err
		}
		return req, nil

	case MsgTypeNewProxyResp:
		var resp TunnelResponse
		if err := json.Unmarshal(msg.Data, &resp); err != nil {
			return nil, err
		}
		return resp, nil

	case MsgTypeData:
		var data DataMessage
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			return nil, err
		}
		return data, nil

	case MsgTypeNewWorkConn:
		var req WorkConnRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			return nil, err
		}
		return req, nil

	default:
		return nil, errors.New("unknown message type")
	}
}

// ParseHeader 解析消息头
func ParseHeader(header []byte) (MessageType, uint64, error) {
	if len(header) < 10 {
		return 0, 0, errors.New("header too short")
	}

	msgType := MessageType(binary.BigEndian.Uint16(header[0:2]))
	msgLen := binary.BigEndian.Uint64(header[2:10])

	return msgType, msgLen, nil
}
