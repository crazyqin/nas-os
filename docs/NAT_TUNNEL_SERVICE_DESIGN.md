# NAT穿透服务设计文档

> 版本: v2.421.0 | 编制: 工部 | 日期: 2026-04-07

## 一、概述

内网穿透服务允许用户无需公网IP即可远程访问NAS设备，对标飞牛FN Connect和群晖QuickConnect。

## 二、竞品对标

| 功能 | NAS-OS | 飞牛FN Connect | 群晖QuickConnect |
|------|:------:|:--------------:|:----------------:|
| 免费穿透 | 🔄 开发中 | ✅ 免费 | ✅ 免费 |
| 无需公网IP | ✅ 设计中 | ✅ | ✅ |
| SSL自动 | ✅ Let's Encrypt | ✅ 自动 | ✅ 自动 |
| 多协议 | ✅ HTTP/SSH | ✅ HTTP | ✅ HTTP |
| 自建服务 | ✅ 支持 | ❌ 仅官方 | ❌ 仅官方 |

## 三、支持方案

### 3.1 FRP穿透
- 自建FRP服务器选项
- 支持HTTP/HTTPS/SSH
- 带宽可控

### 3.2 Cloudflare Tunnel
- 无需公网服务器
- 零信任安全模型
- 支持HTTP/SSH/RDP

### 3.3 WireGuard VPN
- 点对点加密连接
- 高性能低延迟
- 一键配置

## 四、架构设计

```
┌─────────────────────────────────────────────┐
│            Tunnel Service Manager           │
├─────────────────────────────────────────────┤
│  ┌─────────┐ ┌─────────┐ ┌─────────────┐   │
│  │ FRP     │ │Cloudflare│ │ WireGuard   │   │
│  │ Client  │ │ Tunnel   │ │   VPN       │   │
│  └────┬────┘ └────┬────┘ └──────┬──────┘   │
│       │           │            │           │
│       └───────────┼────────────┘           │
│                   │                        │
│            ┌──────▼──────┐                 │
│            │ NAT Traversal│                │
│            │   Engine     │                │
│            └─────────────┘                 │
└─────────────────────────────────────────────┘
```

## 五、API设计

```
# 隧道管理
POST   /api/v1/tunnel/create        # 创建隧道
GET    /api/v1/tunnel/list          # 隧道列表
GET    /api/v1/tunnel/:id           # 隧道详情
DELETE /api/v1/tunnel/:id           # 删除隧道
POST   /api/v1/tunnel/:id/connect   # 连接隧道
POST   /api/v1/tunnel/:id/disconnect # 断开隧道

# 配置管理
GET    /api/v1/tunnel/config        # 获取配置
PUT    /api/v1/tunnel/config        # 更新配置
```

## 六、配置示例

```yaml
tunnel:
  enabled: true
  default_type: frp
  
  frp:
    server_addr: "frp.example.com"
    server_port: 7000
    token: "secure_token"
    
  cloudflare:
    account_id: "cf_account"
    tunnel_token: "tunnel_token"
    
  wireguard:
    endpoint: "wg.example.com:51820"
    public_key: "peer_public_key"
```

## 七、实现路线图

| 阶段 | 任务 | 周期 |
|------|------|------|
| Phase 1 | FRP集成 | M113 |
| Phase 2 | Cloudflare Tunnel | M114 |
| Phase 3 | WireGuard一键配置 | M115 |
| Phase 4 | 自有穿透服务 | M116 |

---

*文档编制: 工部运维组*