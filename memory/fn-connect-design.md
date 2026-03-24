# FN Connect 远程访问服务设计文档

## 1. 概述

FN Connect 是飞牛fnOS 1.0 提供的远程访问服务，支持用户通过公网安全访问 NAS 设备，无需公网 IP 或复杂的端口映射配置。

## 2. 系统架构

```
┌─────────────────────────────────────────────────────────────────┐
│                      FN Connect 架构                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐     │
│  │   客户端 APP  │     │   Web 浏览器  │     │  第三方应用   │     │
│  └──────┬───────┘     └──────┬───────┘     └──────┬───────┘     │
│         │                    │                    │              │
│         └────────────────────┼────────────────────┘              │
│                              │                                   │
│                              ▼                                   │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                  FN Connect Cloud Service                  │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │  │
│  │  │   信令服务器  │  │   中继服务器  │  │   认证服务   │        │  │
│  │  │  (WebSocket) │  │   (TURN)    │  │   (Auth)    │        │  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘        │  │
│  └───────────────────────────────────────────────────────────┘  │
│                              │                                   │
│                              ▼                                   │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                    NAS 设备 (fnOS)                         │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │  │
│  │  │  连接管理器   │  │  ICE Agent  │  │  服务代理    │        │  │
│  │  │ (ConnMgr)   │  │ (STUN/TURN) │  │ (Proxy)     │        │  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘        │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

## 3. 核心组件

### 3.1 连接管理器 (Connection Manager)
- 管理 NAS 与云服务的长连接
- 处理设备注册、认证、心跳
- 维护会话状态

### 3.2 ICE Agent
- STUN/TURN 协议实现（已有）
- NAT 类型检测
- 候选地址收集
- 连通性检查

### 3.3 信令服务
- WebSocket 通信
- SDP/ICE 候选交换
- 会话管理

### 3.4 WebRTC 模块
- P2P 数据通道
- 媒体流传输
- DTLS 加密

### 3.5 服务代理
- 本地服务代理
- 端口映射
- 协议转换

## 4. 连接流程

```
客户端                    云服务                      NAS设备
  │                        │                          │
  │  1. 请求连接           │                          │
  │──────────────────────>│                          │
  │                        │  2. 通知NAS              │
  │                        │─────────────────────────>│
  │                        │                          │
  │                        │  3. ICE候选收集          │
  │                        │<─────────────────────────│
  │                        │                          │
  │  4. 交换ICE候选        │                          │
  │<──────────────────────>│<─────────────────────────>│
  │                        │                          │
  │  5. P2P连接建立        │                          │
  │<────────────────────────────────────────────────>│
  │                        │                          │
  │        或 (P2P失败时)   │                          │
  │                        │                          │
  │  5'. 通过中继连接      │                          │
  │<──────────────────────>│<────────────────────────>│
  │                        │                          │
```

## 5. 模块设计

### 5.1 internal/remote/server.go
```go
// RemoteServer 远程访问服务器
type RemoteServer struct {
    config      RemoteConfig
    signal      *SignalServer
    relay       *TURNRelay
    auth        *AuthService
    connMgr     *ConnectionManager
}
```

### 5.2 internal/remote/client_manager.go
```go
// ConnectionManager 连接管理器
type ConnectionManager struct {
    deviceID    string
    signal      *SignalClient
    sessions    map[string]*RemoteSession
    iceAgent    *ICEAgent
}
```

### 5.3 internal/remote/websocket.go
```go
// WebSocketHandler WebSocket处理
type WebSocketHandler struct {
    upgrader    websocket.Upgrader
    clients     map[string]*WSClient
    onMessage   func(msg []byte)
}
```

### 5.4 internal/remote/webrtc.go
```go
// WebRTCManager WebRTC管理
type WebRTCManager struct {
    peerConnection *webrtc.PeerConnection
    dataChannels   map[string]*webrtc.DataChannel
}
```

## 6. 安全设计

### 6.1 认证机制
- 设备注册时生成唯一 DeviceID 和 Secret
- 基于 JWT 的访问令牌
- 端到端加密（DTLS）

### 6.2 访问控制
- 设备级别的访问权限
- 用户认证（手机号/邮箱）
- 可选的二次验证

### 6.3 数据安全
- TLS 1.3 传输加密
- WebRTC DTLS-SRTP
- 敏感数据本地加密存储

## 7. 配置示例

```json
{
  "device_id": "fnos-xxx-xxx",
  "device_name": "我的NAS",
  "cloud_server": "connect.fnos.com:443",
  "stun_servers": [
    "stun:stun.fnos.com:3478",
    "stun:stun.l.google.com:19302"
  ],
  "turn_servers": [
    {
      "url": "turn:turn.fnos.com:3478",
      "username": "fnos",
      "credential": "xxx"
    }
  ],
  "services": [
    {
      "name": "web",
      "local_port": 80,
      "protocol": "tcp"
    },
    {
      "name": "smb",
      "local_port": 445,
      "protocol": "tcp"
    }
  ]
}
```

## 8. API 设计

### 8.1 WebSocket 消息类型

| 类型 | 方向 | 说明 |
|------|------|------|
| register | NAS→Cloud | 设备注册 |
| connect | Client→Cloud | 请求连接 |
| offer | 双向 | SDP Offer |
| answer | 双向 | SDP Answer |
| candidate | 双向 | ICE 候选 |
| connected | 双向 | 连接成功 |
| disconnect | 双向 | 断开连接 |
| error | Cloud→Client | 错误消息 |

### 8.2 REST API

```
POST /api/v1/remote/connect    - 建立远程连接
GET  /api/v1/remote/status     - 获取连接状态
POST /api/v1/remote/disconnect - 断开连接
GET  /api/v1/remote/services   - 获取可用服务列表
```

## 9. 实现计划

### Phase 1: 基础框架 (已完成)
- [x] ICE Agent 实现 (internal/tunnel/ice.go)
- [x] STUN/TURN 协议 (internal/tunnel/stun.go, turn.go)
- [x] 信令服务 (internal/tunnel/signaling.go)

### Phase 2: 远程访问服务 (本次实现)
- [ ] 远程访问服务器 (internal/remote/server.go)
- [ ] 连接管理器 (internal/remote/client_manager.go)
- [ ] WebSocket 处理器 (internal/remote/websocket.go)
- [ ] WebRTC 管理器 (internal/remote/webrtc.go)
- [ ] 服务代理 (internal/remote/proxy.go)

### Phase 3: 高级功能
- [ ] 多设备管理
- [ ] 带宽优化
- [ ] 断点续传
- [ ] 访问日志

## 10. 依赖关系

```
internal/remote/
├── server.go          # 远程访问服务器
├── client_manager.go  # 客户端连接管理
├── websocket.go       # WebSocket通信
├── webrtc.go          # WebRTC数据通道
├── proxy.go           # 本地服务代理
├── auth.go            # 认证服务
└── types.go           # 类型定义

依赖:
├── internal/tunnel/   # 内网穿透基础
│   ├── ice.go         # ICE协议
│   ├── stun.go        # STUN协议
│   ├── turn.go        # TURN协议
│   └── signaling.go   # 信令服务
└── github.com/gorilla/websocket
```

## 11. 测试策略

### 单元测试
- 各模块独立测试
- Mock 外部依赖

### 集成测试
- 端到端连接测试
- 不同 NAT 类型穿透测试

### 压力测试
- 并发连接数测试
- 带宽性能测试

---

*设计版本: v1.0*
*创建时间: 2026-03-25*
*作者: 兵部*