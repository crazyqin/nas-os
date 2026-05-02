# VPN Server 用户指南

> **版本**: v2.482.0+ | **适用版本**: NAS-OS v2.482.0 及以上

## 概述

NAS-OS 内置 VPN Server，支持 WireGuard 和 OpenVPN 两种协议，让您从任何地方安全访问家庭/办公网络。无需额外购买硬件或第三方服务，直接在 NAS 上一键部署 VPN 服务。

## 核心特性

- **双协议支持**：WireGuard（高速）和 OpenVPN（兼容性好）
- **用户授权管理**：按用户分配 VPN 访问权限
- **连接监控**：实时查看在线用户、流量、延迟
- **客户端配置生成**：一键导出 .conf / .ovpn 配置文件
- **自动 DNS 分配**：VPN 客户端自动获取 DNS 和路由
- **Kill Switch**：防止 VPN 断开时泄露真实 IP
- **带宽限制**：可按用户设置上行/下行带宽上限

## 配置步骤

### 1. 启用 VPN Server

进入 **网络 → VPN Server** 页面，选择协议并启用：

```
协议选择：WireGuard（推荐） / OpenVPN
监听端口：WireGuard 默认 51820 / OpenVPN 默认 1194
虚拟网络：10.8.0.0/24（可自定义）
```

### 2. 配置 WireGuard

```bash
# 服务端自动生成密钥对
# Server Public Key 显示在管理界面

# 客户端配置示例（自动生成）
[Interface]
PrivateKey = <client-private-key>
Address = 10.8.0.2/32
DNS = 10.8.0.1

[Peer]
PublicKey = <server-public-key>
Endpoint = your-nas-ip:51820
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
```

### 3. 配置 OpenVPN

```bash
# 服务端自动配置 TLS 证书
# 导出 .ovpn 客户端配置文件

# 客户端连接
openvpn --config client.ovpn
```

### 4. 用户授权

进入 **VPN Server → 用户管理**：
- 勾选允许使用 VPN 的用户
- 设置每用户最大同时连接数
- 配置带宽限制（可选）

### 5. 端口转发

确保路由器将 VPN 端口转发到 NAS：
- WireGuard: UDP 51820
- OpenVPN: UDP/TCP 1194

## 客户端安装

| 平台 | WireGuard | OpenVPN |
|------|-----------|---------|
| Windows | [WireGuard](https://www.wireguard.com/install/) | [OpenVPN](https://openvpn.net/) |
| macOS | App Store / [官网](https://www.wireguard.com/install/) | Tunnelblick |
| iOS | App Store: WireGuard | App Store: OpenVPN Connect |
| Android | Play Store: WireGuard | Play Store: OpenVPN Connect |
| Linux | `apt install wireguard` | `apt install openvpn` |

## 常见问题

### Q: WireGuard 和 OpenVPN 选哪个？
- **WireGuard**：速度更快（3-5x），延迟更低，适合日常使用
- **OpenVPN**：兼容性更好，支持 TCP 443 端口（穿透防火墙）

### Q: 无法连接怎么办？
1. 检查路由器端口转发是否正确
2. 检查 NAS 防火墙是否放行 VPN 端口
3. 确认客户端配置中的 Endpoint 地址正确
4. 如果是动态 IP，建议配合 DDNS 使用

### Q: 多人同时使用会有冲突吗？
不会。每个用户分配独立的虚拟 IP（10.8.0.x），互不影响。

### Q: VPN 会影响 NAS 本地性能吗？
WireGuard 内核级实现，CPU 占用极低（< 1%）。OpenVPN 稍高，但对日常使用无明显影响。

---

## 相关指南

- [远程桌面网关](remote-desktop-guide.md) — 配合 VPN 实现加密远程办公
- [NAT 穿透](natpierce.md) — 无需公网 IP 即可外网访问 NAS
- [NAT 隧道](../USER_GUIDE_NAT_TUNNEL.md) — Cloudflare Tunnel 配置
- [合规仪表盘](compliance-dashboard-guide.md) — VPN 连接审计与合规检查

## API 参考

### 获取 VPN 状态

```bash
curl http://localhost:8080/api/v1/vpn/status
```

### 启用/禁用 VPN

```bash
# 启用 WireGuard
curl -X POST http://localhost:8080/api/v1/vpn/wireguard/start

# 停止 WireGuard
curl -X POST http://localhost:8080/api/v1/vpn/wireguard/stop
```

### 用户管理

```bash
# 授权用户
curl -X POST http://localhost:8080/api/v1/vpn/users \
  -H "Content-Type: application/json" \
  -d '{"username": "user1", "max_connections": 3, "bandwidth_limit_mbps": 50}'

# 列出授权用户
curl http://localhost:8080/api/v1/vpn/users
```

### 获取客户端配置

```bash
# WireGuard 客户端配置
curl http://localhost:8080/api/v1/vpn/wireguard/client-config/user1

# OpenVPN 客户端配置
curl http://localhost:8080/api/v1/vpn/openvpn/client-config/user1
```

### 连接监控

```bash
# 查看在线连接
curl http://localhost:8080/api/v1/vpn/connections

# 连接统计
curl http://localhost:8080/api/v1/vpn/stats
```
