# 工部工作报告 - 内网穿透服务（frp集成）

**日期**: 2026-03-25
**版本**: v2.275.0
**状态**: ✅ 已完成

---

## 一、实现的功能列表

### 1. frp客户端核心实现 ✅

| 功能 | 文件 | 说明 |
|------|------|------|
| FRP管理器 | `internal/tunnel/frp.go` | FRP客户端生命周期管理 |
| 代理配置 | `internal/tunnel/frp.go` | TCP/UDP/HTTP/HTTPS代理支持 |
| 配置生成 | `internal/tunnel/frp.go` | TOML配置文件自动生成 |
| 状态监控 | `internal/tunnel/frp.go` | 连接状态、流量统计、自动重连 |
| 零配置API | `internal/tunnel/frp.go` | QuickConnect一键连接 |

### 2. 隧道管理器 ✅

| 功能 | 文件 | 说明 |
|------|------|------|
| 隧道管理 | `internal/tunnel/manager.go` | 多隧道实例管理 |
| NAT检测 | `internal/tunnel/manager.go` | STUN协议NAT类型检测 |
| P2P连接 | `internal/tunnel/p2p.go` | P2P直连模式支持 |
| 信令服务 | `internal/tunnel/signaling.go` | WebSocket信令交换 |
| ICE Agent | `internal/tunnel/stun.go`, `turn.go` | STUN/TURN协议实现 |

### 3. WebUI配置界面 ✅

| 功能 | 文件 | 说明 |
|------|------|------|
| 仪表盘 | `webui/pages/tunnel.html` | 连接状态、流量统计 |
| 快速连接 | `webui/pages/tunnel.html` | 一键配置常用服务 |
| 代理管理 | `webui/pages/tunnel.html` | 代理列表、添加、删除 |
| 预设服务 | `webui/pages/tunnel.html` | Web/SSH/SMB等模板 |
| 高级配置 | `webui/pages/tunnel.html` | 服务器、认证、TLS设置 |

### 4. API接口开发 ✅

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/v1/tunnel/dashboard` | GET | 仪表盘数据 |
| `/api/v1/tunnel/frp/status` | GET | FRP状态 |
| `/api/v1/tunnel/frp/start` | POST | 启动服务 |
| `/api/v1/tunnel/frp/stop` | POST | 停止服务 |
| `/api/v1/tunnel/frp/restart` | POST | 重启服务 |
| `/api/v1/tunnel/frp/config` | GET/PUT | 配置管理 |
| `/api/v1/tunnel/frp/proxies` | GET/POST | 代理列表/创建 |
| `/api/v1/tunnel/frp/proxies/:name` | GET/PUT/DELETE | 代理详情/更新/删除 |
| `/api/v1/tunnel/frp/quick-connect` | POST | 一键连接 |
| `/api/v1/tunnel/presets` | GET | 预设服务列表 |
| `/api/v1/tunnel/p2p/status` | GET | P2P状态 |
| `/api/v1/tunnel/detect-nat` | POST | NAT类型检测 |
| `/api/v1/tunnel/public-ip` | GET | 获取公网IP |

---

## 二、架构设计

### 2.1 系统架构

```
┌─────────────────────────────────────────────────────────────────┐
│                    NAS-OS 内网穿透服务架构                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐     │
│  │  WebUI前端    │     │   API层       │     │   外部客户端  │     │
│  │ tunnel.html  │────>│ WebUIHandler │<────│   (浏览器)    │     │
│  └──────────────┘     └──────────────┘     └──────────────┘     │
│                              │                                   │
│                              ▼                                   │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                      Service Layer                         │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │  │
│  │  │  FRPManager │  │  Manager    │  │TunnelService│        │  │
│  │  │  (frp客户端) │  │ (隧道管理)   │  │ (P2P服务)   │        │  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘        │  │
│  └───────────────────────────────────────────────────────────┘  │
│                              │                                   │
│                              ▼                                   │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                    Protocol Layer                          │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │  │
│  │  │    STUN     │  │    TURN     │  │  Signaling  │        │  │
│  │  │  (NAT检测)   │  │  (中继转发)  │  │  (信令交换)  │        │  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘        │  │
│  └───────────────────────────────────────────────────────────┘  │
│                              │                                   │
│                              ▼                                   │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                    FRP Server (云端)                        │  │
│  │  公网访问地址: <server>:<remote_port>                        │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 核心模块

#### FRPManager (internal/tunnel/frp.go)
```go
type FRPManager struct {
    config       *FRPConfig           // FRP配置
    proxyConfigs map[string]*FRPProxyConfig  // 代理配置
    status       FRPStatus            // 运行状态
    cmd          *exec.Cmd            // frpc进程
    // ...
}

// 主要方法
- Start() error              // 启动FRP客户端
- Stop() error               // 停止FRP客户端
- AddProxy(proxy) error      // 添加代理
- RemoveProxy(name) error    // 移除代理
- QuickConnect(port, name)   // 一键连接
- GetStatus() FRPStatus      // 获取状态
```

#### Manager (internal/tunnel/manager.go)
```go
type Manager struct {
    config     Config           // 隧道配置
    tunnels    map[string]*Tunnel  // 隧道实例
    detector   NATDetector      // NAT检测器
    // ...
}

// 主要方法
- Start(ctx) error           // 启动管理器
- Stop() error               // 停止管理器
- Connect(ctx, req)          // 建立隧道
- Disconnect(tunnelID)       // 断开隧道
- GetStatus() ManagerStatus  // 获取状态
```

### 2.3 数据流

```
用户请求 → API层 → FRPManager → frpc进程 → FRP服务器 → 公网
         ↓
      状态监控 ← 配置文件生成 ← 代理配置
```

---

## 三、配置说明

### 3.1 FRP配置

```json
{
  "enabled": true,
  "serverAddr": "frp.example.com",
  "serverPort": 7000,
  "token": "your-auth-token",
  "deviceId": "nas-001",
  "deviceName": "My NAS",
  "autoReconnect": true,
  "logLevel": "info"
}
```

### 3.2 代理配置

```json
{
  "name": "web-proxy",
  "type": "tcp",
  "localIp": "127.0.0.1",
  "localPort": 80,
  "remotePort": 10080
}
```

支持的代理类型：
- **TCP**: 端口映射，适用于大多数服务
- **UDP**: UDP协议支持
- **HTTP**: HTTP代理，支持域名绑定
- **HTTPS**: HTTPS代理，支持TLS

### 3.3 预设服务

| 服务 | 端口 | 协议 | 说明 |
|------|------|------|------|
| Web管理 | 80 | TCP | Web管理界面 |
| HTTPS | 443 | TCP | HTTPS服务 |
| SSH | 22 | TCP | SSH远程登录 |
| SMB | 445 | TCP | SMB文件共享 |
| FTP | 21 | TCP | FTP文件传输 |
| WebDAV | 5005 | TCP | WebDAV服务 |
| MySQL | 3306 | TCP | MySQL数据库 |
| PostgreSQL | 5432 | TCP | PostgreSQL数据库 |
| Redis | 6379 | TCP | Redis缓存 |
| Transmission | 9091 | TCP | BT下载管理 |

---

## 四、部署指南

### 4.1 前置条件

1. **FRP服务器**: 需要一台有公网IP的服务器运行frps
2. **frpc二进制**: 系统需安装frpc客户端
   ```bash
   # Debian/Ubuntu
   apt install frpc
   
   # 或从GitHub下载
   wget https://github.com/fatedier/frp/releases/download/v0.52.0/frp_0.52.0_linux_arm64.tar.gz
   tar -xzf frp_0.52.0_linux_arm64.tar.gz
   mv frp_0.52.0_linux_arm64/frpc /usr/local/bin/
   ```

### 4.2 配置步骤

1. **访问WebUI**: 打开 `http://<nas-ip>/tunnel`

2. **配置服务器**:
   - 点击"⚙️ 设置"
   - 输入FRP服务器地址和端口
   - 可选：配置认证令牌

3. **添加代理**:
   - 方式一：点击"⚡ 一键连接"，输入本地端口
   - 方式二：点击"📋 预设服务"，选择常用服务
   - 方式三：点击"➕ 添加代理"，手动配置

4. **启动服务**: 点击"启动服务"按钮

5. **验证连接**: 
   - 查看仪表盘状态
   - 通过公网地址访问服务

### 4.3 Docker部署

```yaml
# docker-compose.yml
services:
  nas-os:
    image: nas-os:latest
    ports:
      - "8080:8080"
    volumes:
      - ./config:/etc/nas-os
      - ./data:/var/lib/nas-os
    environment:
      - FRP_SERVER=frp.example.com
      - FRP_PORT=7000
```

### 4.4 系统集成

内网穿透模块已集成到NAS-OS主服务：
- Web服务器初始化时自动加载（`internal/web/server.go`）
- API路由自动注册（`/api/v1/tunnel/*`）
- 页面路由自动添加（`/tunnel`）

---

## 五、与飞牛FN Connect对比

| 功能 | NAS-OS | FN Connect | 说明 |
|------|--------|------------|------|
| 零配置连接 | ✅ | ✅ | 一键快速配置 |
| 多协议支持 | ✅ TCP/UDP/HTTP/HTTPS | ✅ | 完整支持 |
| 自动重连 | ✅ | ✅ | 断线自动恢复 |
| NAT类型检测 | ✅ STUN | ✅ | 智能穿透 |
| P2P直连 | ✅ | ✅ | 优先直连 |
| 中继转发 | ✅ TURN | ✅ | 备用方案 |
| WebUI管理 | ✅ | ✅ | 图形化界面 |
| 预设服务 | ✅ 10种 | ✅ | 常用服务模板 |
| 流量统计 | ✅ | ✅ | 实时监控 |
| 免费使用 | ✅ | ✅ | 无需付费 |

---

## 六、后续优化

1. **内置FRP服务器**: 提供官方云服务选项
2. **多服务器支持**: 支持配置多个FRP服务器
3. **带宽监控**: 更详细的流量分析
4. **访问日志**: 记录远程访问日志
5. **安全增强**: IP白名单、访问频率限制

---

## 七、文件清单

```
internal/tunnel/
├── api.go                 # API类型定义
├── bandwidth.go           # 带宽监控
├── client.go              # 客户端实现
├── config_enhanced.go     # 增强配置
├── frp.go                 # FRP管理器（核心）
├── manager.go             # 隧道管理器
├── p2p.go                 # P2P连接
├── quality.go             # 连接质量
├── service.go             # 隧道服务
├── signaling.go           # 信令服务
├── stun.go                # STUN协议
├── turn.go                # TURN协议
├── types.go               # 类型定义
└── webui_api.go           # WebUI API处理器

webui/pages/
└── tunnel.html            # 内网穿透WebUI页面

internal/web/
└── server.go              # 集成tunnel模块
```

---

**完成人**: 工部
**审核人**: 待审核
**版本**: v2.275.0