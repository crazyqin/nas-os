# Tunnel Module - 内网穿透模块

基于 STUN/TURN/ICE 协议的内网穿透实现，支持 P2P 直连、UDP 打洞和中继转发。

## 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                      TunnelManager                          │
│  (核心管理器 - 统一协调所有组件)                              │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐              │
│  │ STUN      │  │ TURN      │  │ Signaling │              │
│  │ Client    │  │ Client    │  │ Client    │              │
│  │ (NAT检测) │  │ (中继)    │  │ (信令)    │              │
│  └───────────┘  └───────────┘  └───────────┘              │
│                                                             │
│  ┌───────────────────────────────────────────┐             │
│  │              ICEAgent                      │             │
│  │  (连接建立 - 候选者收集与连接检查)          │             │
│  └───────────────────────────────────────────┘             │
│                                                             │
│  ┌───────────────────────────────────────────┐             │
│  │              PeerManager                   │             │
│  │  (对等连接管理 - 多客户端并发)              │             │
│  └───────────────────────────────────────────┘             │
│                                                             │
│  ┌───────────────────────────────────────────┐             │
│  │              Crypto                        │             │
│  │  (端到端加密 - ChaCha20-Poly1305/AES-GCM)  │             │
│  └───────────────────────────────────────────┘             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## 快速开始

### 基本使用

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "nas-os/internal/network/tunnel"
)

func main() {
    // 创建配置
    config := tunnel.DefaultConfig()
    config.ListenPort = 51820
    config.SignalingURL = "wss://signal.example.com/ws"
    
    // 创建隧道管理器
    manager, err := tunnel.NewTunnelManager(config)
    if err != nil {
        panic(err)
    }
    
    // 注册事件处理器
    manager.OnEvent(func(event tunnel.TunnelEvent) {
        fmt.Printf("Event: %s at %v\n", event.Type, event.Timestamp)
    })
    
    // 启动隧道
    ctx := context.Background()
    if err := manager.Start(ctx); err != nil {
        panic(err)
    }
    defer manager.Close()
    
    // 连接到远程对等端
    peerInfo := &tunnel.PeerInfo{
        ID:        "remote-peer-id",
        PublicKey: peerPublicKey,
        Endpoints: []net.UDPAddr{
            {IP: net.ParseIP("192.168.1.100"), Port: 51820},
        },
    }
    
    if err := manager.ConnectPeer(ctx, "remote-peer-id", peerInfo); err != nil {
        panic(err)
    }
    
    // 发送数据
    manager.Send("remote-peer-id", []byte("Hello, Peer!"))
    
    // 接收数据
    for data := range manager.Receive() {
        fmt.Printf("From %s: %s\n", data.PeerID, string(data.Data))
    }
}
```

### 使用配置构建器

```go
config, err := tunnel.NewConfigBuilder().
    WithListenPort(51820).
    WithSTUNServers(
        "stun:stun.l.google.com:19302",
        "stun:stun.cloudflare.com:3478",
    ).
    WithTURNServer("turn:turn.example.com:3478", "username", "password").
    WithSignaling("wss://signal.example.com/ws").
    WithKeepalive(30 * time.Second).
    WithMaxPeers(100).
    Build()
```

## 核心组件

### 1. STUN 客户端

用于 NAT 类型检测和公网地址发现：

```go
stunClient := tunnel.NewSTUNClient(config)

ctx := context.Background()
result, err := stunClient.Discover(ctx)

fmt.Printf("NAT Type: %s\n", result.NATType)
fmt.Printf("Public IP: %s:%d\n", result.PublicIP, result.PublicPort)
```

支持的 NAT 类型：
- `NATNone` - 无 NAT（公网 IP）
- `NATFullCone` - 全锥形 NAT（最易穿透）
- `NATRestrictedCone` - 受限锥形 NAT
- `NATPortRestricted` - 端口受限锥形 NAT
- `NATSymmetric` - 对称型 NAT（最难穿透）

### 2. TURN 客户端

用于中继转发（当 P2P 失败时）：

```go
turnClient := tunnel.NewTURNClient(config, tunnel.TURNServer{
    URL:      "turn:turn.example.com:3478",
    Username: "user",
    Password: "pass",
})

if err := turnClient.Connect(ctx, "turn.example.com:3478"); err != nil {
    panic(err)
}

allocation, err := turnClient.Allocate(ctx)
fmt.Printf("Relay address: %s\n", allocation.RelayAddr)

// 发送数据
turnClient.Send(ctx, []byte("Hello"), peerAddr)

// 接收数据
data, from, _ := turnClient.Receive(ctx)
```

### 3. ICE 代理

协调 STUN/TURN 进行连接建立：

```go
iceAgent := tunnel.NewICEAgent(config)
iceAgent.Initialize(ctx)

// 获取本地候选者
localSDP := iceAgent.GetLocalDescription()

// 设置远程候选者
iceAgent.SetRemoteCandidates(remoteCandidates)

// 开始连接检查
iceAgent.StartConnectivityChecks(remoteUfrag, remotePwd)

// 等待连接
iceAgent.OnConnected(func() {
    fmt.Println("ICE Connected!")
})
```

### 4. 加密模块

端到端加密传输：

```go
crypto, _ := tunnel.NewCrypto(&tunnel.CryptoConfig{
    CipherType: tunnel.CipherChaCha20Poly1305,
})

// 生成密钥对
crypto.GenerateKeyPair()

// 派生共享密钥
sharedKey, _ := crypto.DeriveSharedKey(peerPublicKey)
crypto.SetPeerKey("peer-id", sharedKey)

// 加密/解密
encrypted, _ := crypto.Encrypt(data, "peer-id")
decrypted, _ := crypto.Decrypt(encrypted, "peer-id")
```

## 连接流程

### 主动连接流程

```
┌────────┐                              ┌────────┐
│ Peer A │                              │ Peer B │
└───┬────┘                              └───┬────┘
    │                                       │
    │  1. 初始化 ICE，收集候选者            │
    │<─────────────────────────────────────│
    │                                       │
    │  2. 通过信令发送 Offer                │
    │──────────────────────────────────────>│
    │                                       │
    │  3. 初始化 ICE，收集候选者            │
    │                                       │
    │  4. 通过信令返回 Answer               │
    │<──────────────────────────────────────│
    │                                       │
    │  5. ICE 连接检查                      │
    │<─────────────────────────────────────>│
    │                                       │
    │  6. 建立连接                          │
    │<══════════════════════════════════════>│
    │                                       │
```

### NAT 穿透策略

1. **直连**：如果双方都有公网 IP，直接连接
2. **UDP 打洞**：通过 STUN 获取公网地址，双方同时发送数据包
3. **TURN 中继**：如果打洞失败，使用 TURN 服务器中继

## 配置选项

| 选项 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `ListenPort` | int | 51820 | 本地监听端口 |
| `STUNServers` | []string | Google/Cloudflare STUN | STUN 服务器列表 |
| `TURNServers` | []TURNServer | - | TURN 中继服务器 |
| `SignalingURL` | string | - | 信令服务器 URL |
| `STUNTimeout` | Duration | 5s | STUN 请求超时 |
| `TURNTimeout` | Duration | 10s | TURN 操作超时 |
| `ICETimeout` | Duration | 30s | ICE 连接超时 |
| `Keepalive` | Duration | 25s | 保活间隔 |
| `MaxPeers` | int | 100 | 最大对等连接数 |
| `MaxRetries` | int | 3 | 最大重试次数 |

## 事件类型

| 事件 | 说明 |
|------|------|
| `started` | 隧道启动 |
| `stopped` | 隧道停止 |
| `nat_discovered` | NAT 类型检测完成 |
| `stun_failed` | STUN 检测失败 |
| `signaling_connected` | 信令服务器连接成功 |
| `signaling_failed` | 信令服务器连接失败 |
| `turn_connected` | TURN 中继连接成功 |
| `peer_added` | 新对等端添加 |
| `peer_removed` | 对等端移除 |

## 性能特点

- **并发连接**：支持多客户端并发连接（默认最大100个）
- **高效加密**：使用 ChaCha20-Poly1305 或 AES-GCM
- **低延迟**：优先 P2P 直连，减少中继延迟
- **自动重连**：支持断线重连和保活机制

## 与飞牛 fnOS FN Connect 对比

| 特性 | nas-os Tunnel | fnOS FN Connect |
|------|---------------|-----------------|
| NAT 检测 | ✅ STUN | ✅ 支持 |
| P2P 打洞 | ✅ UDP Hole Punching | ✅ 支持 |
| 中继转发 | ✅ TURN | ✅ 第三方服务 |
| 端到端加密 | ✅ ChaCha20/AES | ✅ 支持 |
| 信令服务 | ✅ 可自建 | ⚠️ 依赖官方 |
| 开源 | ✅ 完全开源 | ❌ 闭源 |

## 文件结构

```
internal/network/tunnel/
├── types.go        # 类型定义
├── config.go       # 配置管理
├── stun.go         # STUN 客户端
├── turn.go         # TURN 客户端
├── ice.go          # ICE 代理
├── signaling.go    # 信令服务
├── crypto.go       # 加密模块
├── peer.go         # 对等连接管理
├── tunnel.go       # 隧道管理器
└── tunnel_test.go  # 测试文件
```

## 参考协议

- RFC 5389: STUN Protocol
- RFC 5766: TURN Relay Extensions
- RFC 5245: ICE Protocol
- RFC 8446: TLS 1.3 (用于信令加密)
- RFC 8439: ChaCha20-Poly1305