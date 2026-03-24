# NAS-OS 内网穿透用户指南

**版本**: v2.266.0  
**更新日期**: 2026-03-25

本文档介绍 NAS-OS 内网穿透功能的使用方法，帮助您实现远程访问 NAS。

## 目录

- [功能概述](#功能概述)
- [配置说明](#配置说明)
- [使用指南](#使用指南)
- [最佳实践](#最佳实践)
- [常见问题](#常见问题)

---

## 功能概述

NAS-OS 内网穿透服务（NatPierce）让您可以在任何地方安全访问家中或办公室的 NAS，无需公网 IP 或复杂的路由器配置。

### 核心特性

| 特性 | 说明 |
|------|------|
| P2P 直连 | 点对点连接，低延迟，高带宽 |
| 中继模式 | 当 P2P 不可用时自动切换，保证连接稳定 |
| 自动选择 | 智能判断最优连接方式 |
| TLS 加密 | 端到端加密传输，保护数据安全 |
| 零配置 | 无需手动配置端口转发 |

### 连接模式

```
┌─────────────────────────────────────────────────────────────┐
│                    内网穿透架构                              │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│    远程设备                    NAS 设备                     │
│   ┌─────────┐                ┌─────────┐                  │
│   │  手机   │                │  NAS    │                  │
│   │  电脑   │◄──────────────►│  服务   │                  │
│   └─────────┘                └─────────┘                  │
│        │                          │                        │
│        │     ┌──────────────┐     │                        │
│        └────►│  中继服务器   │◄────┘                        │
│              │  (可选)      │                              │
│              └──────────────┘                              │
│                                                             │
│   ┌──────────────────────────────────────────────────┐     │
│   │  连接方式:                                        │     │
│   │  1. P2P 直连 (首选) - 通过 STUN 打洞              │     │
│   │  2. 中继模式 (回退) - 通过中继服务器转发           │     │
│   │  3. 反向隧道 - NAS 主动连接外部服务                │     │
│   └──────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

### 适用场景

- **远程办公**: 在外访问公司 NAS 资料
- **家庭存储**: 随时查看家庭照片、视频
- **文件共享**: 与他人分享大文件
- **媒体播放**: 远程播放 NAS 上的影音内容

---

## 配置说明

### 1. 启用内网穿透

通过 Web 管理界面启用：

1. 登录 NAS 管理后台
2. 进入 **网络** → **内网穿透**
3. 点击 **启用服务**
4. 记录生成的 **访问地址** 和 **设备 ID**

或通过配置文件 `/etc/nas-os/tunnel.yaml`：

```yaml
# 隧道服务配置
server:
  # 隧道服务器地址
  addr: "tunnel.nas-os.local"
  port: 443
  # 认证令牌（从管理后台获取）
  auth_token: "your-token-here"

# 设备配置
device:
  name: "My NAS"

# 连接配置
connection:
  mode: "auto"  # p2p, relay, reverse, auto
  heartbeat_interval: 30
  reconnect_interval: 5
  max_reconnect: 10
  timeout: 30
```

### 2. 端口映射配置

配置需要远程访问的服务端口：

```yaml
port_mapping:
  enabled: true
  
  # 本地需要映射的端口
  local_ports:
    - 80      # Web 管理界面
    - 443     # HTTPS
    - 21      # FTP
    - 22      # SSH
    - 873     # rsync
    - 445     # SMB
  
  # 远程端口（留空自动分配）
  remote_ports: []
  
  # IP 白名单（留空允许所有）
  allowed_ips: []
```

### 3. 安全配置

建议启用 TLS 加密：

```yaml
security:
  enable_tls: true
  tls_cert_file: "/etc/nas-os/ssl/tunnel.crt"
  tls_key_file: "/etc/nas-os/ssl/tunnel.key"
  allow_insecure: false  # 生产环境务必为 false
```

### 4. STUN/TURN 配置

用于 P2P 打洞和回退中继：

```yaml
ice:
  # STUN 服务器（NAT 检测和 P2P 打洞）
  stun_servers:
    - "stun.l.google.com:19302"
    - "stun1.l.google.com:19302"
  
  # TURN 服务器（中继回退）
  turn_servers:
    - "turn.nas-os.local:3478"
  turn_user: ""
  turn_pass: ""
```

---

## 使用指南

### 命令行操作

#### 启动穿透服务

```bash
# 启动内网穿透
nasctl tunnel start

# 查看连接状态
nasctl tunnel status

# 查看详细日志
nasctl tunnel logs --follow
```

#### 配置管理

```bash
# 查看当前配置
nasctl tunnel config show

# 修改连接模式
nasctl tunnel config set mode relay

# 添加端口映射
nasctl tunnel port add --local 8080 --remote 8080

# 删除端口映射
nasctl tunnel port remove --local 8080

# 查看端口映射列表
nasctl tunnel port list
```

#### 连接管理

```bash
# 查看连接状态
nasctl tunnel status

# 强制重连
nasctl tunnel reconnect

# 切换连接模式
nasctl tunnel switch --mode p2p
nasctl tunnel switch --mode relay

# 查看延迟
nasctl tunnel ping
```

### API 调用

```bash
# 获取穿透状态
curl http://localhost:8080/api/v1/tunnel/status \
  -H "Authorization: Bearer $TOKEN"

# 启动穿透服务
curl -X POST http://localhost:8080/api/v1/tunnel/start \
  -H "Authorization: Bearer $TOKEN"

# 配置端口映射
curl -X POST http://localhost:8080/api/v1/tunnel/ports \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "local_port": 80,
    "remote_port": 8080,
    "protocol": "tcp"
  }'

# 获取连接信息
curl http://localhost:8080/api/v1/tunnel/info \
  -H "Authorization: Bearer $TOKEN"
```

### 远程访问

#### 获取访问地址

```bash
# 查看访问地址
nasctl tunnel info

# 输出示例：
# Device ID: nas-abc123def456
# Access URL: https://abc123.tunnel.nas-os.local
# Public IP: 203.0.113.50 (via relay)
# Latency: 45ms
```

#### 使用访问地址

在远程设备上，使用分配的地址访问：

```
# Web 界面
https://abc123.tunnel.nas-os.local

# SSH 连接
ssh -p 2222 user@abc123.tunnel.nas-os.local

# SMB 共享
\\abc123.tunnel.nas-os.local\share
```

### 客户端配置

在远程设备上安装 NAS-OS 客户端：

```bash
# Linux/macOS
curl -sSL https://get.nas-os.local/client | sh

# 配置连接
nasctl client connect --device-id nas-abc123def456

# 查看连接状态
nasctl client status
```

---

## 最佳实践

### 1. 安全加固

```yaml
# 推荐安全配置
security:
  enable_tls: true
  allow_insecure: false

port_mapping:
  # 只开放必要端口
  local_ports:
    - 443  # 仅 HTTPS
  # 设置 IP 白名单
  allowed_ips:
    - "192.168.0.0/16"
    - "10.0.0.0/8"
```

### 2. 性能优化

```yaml
# 性能优先配置
connection:
  mode: "p2p"  # 优先 P2P 直连
  heartbeat_interval: 60  # 减少心跳频率

ice:
  stun_servers:
    - "stun.l.google.com:19302"  # 选择最近的 STUN 服务器
```

### 3. 稳定性保障

```yaml
# 高可用配置
connection:
  mode: "auto"  # 自动选择最优路径
  max_reconnect: 20  # 增加重连次数
  reconnect_interval: 3  # 缩短重连间隔

ice:
  # 配置多个 TURN 服务器作为回退
  turn_servers:
    - "turn1.nas-os.local:3478"
    - "turn2.nas-os.local:3478"
```

### 4. 监控告警

```bash
# 设置连接状态监控
nasctl tunnel monitor enable \
  --alert-disconnect \
  --webhook "https://your-webhook-url"

# 查看连接历史
nasctl tunnel history --days 7

# 导出监控数据
nasctl tunnel metrics export --format prometheus
```

---

## 常见问题

### Q1: 为什么连接速度很慢？

**A**: 可能的原因和解决方案：

1. **使用了中继模式**: 检查是否 P2P 直连成功
   ```bash
   nasctl tunnel status
   # 如果 mode 显示 relay，说明 P2P 失败
   
   # 尝试强制 P2P 模式
   nasctl tunnel switch --mode p2p
   ```

2. **网络质量差**: 检查延迟和带宽
   ```bash
   nasctl tunnel ping
   nasctl tunnel speed-test
   ```

3. **端口冲突**: 检查端口映射
   ```bash
   nasctl tunnel port list
   ```

### Q2: 连接经常断开怎么办？

**A**: 调整连接参数：

```yaml
connection:
  heartbeat_interval: 15  # 缩短心跳间隔
  max_reconnect: 30       # 增加重连次数
  reconnect_interval: 2   # 加快重连速度
```

### Q3: 如何保护穿透连接安全？

**A**: 建议措施：

1. **启用 TLS 加密**
2. **设置 IP 白名单**
3. **定期更换认证令牌**
4. **使用强密码**

```bash
# 更换令牌
nasctl tunnel token regenerate

# 设置白名单
nasctl tunnel whitelist add 192.168.1.0/24
```

### Q4: P2P 直连失败的原因？

**A**: 常见原因：

1. **对称型 NAT**: 部分运营商 NAT 不支持 P2P
2. **防火墙限制**: UDP 端口被封锁
3. **STUN 服务器不可达**: 检查网络连通性

```bash
# 检测 NAT 类型
nasctl tunnel nat-detect

# 测试 STUN 连通性
nasctl tunnel stun-test
```

### Q5: 能否自定义穿透域名？

**A**: 可以绑定自定义域名：

```bash
# 绑定域名
nasctl tunnel domain bind mynas.example.com

# 验证域名
nasctl tunnel domain verify mynas.example.com
```

### Q6: 穿透服务占用多少资源？

**A**: 资源消耗参考：

| 模式 | CPU | 内存 | 带宽开销 |
|------|-----|------|----------|
| P2P | < 1% | ~20MB | 极低 |
| 中继 | 1-3% | ~50MB | 取决于流量 |

### Q7: 多个 NAS 能否共享一个账号？

**A**: 支持，每个 NAS 有独立的设备 ID：

```bash
# 在管理后台添加设备
# 或通过命令行注册
nasctl tunnel register --name "NAS-Office"
```

---

## 相关文档

- [网络配置指南](../NETWORK_API.md)
- [安全最佳实践](../SECURITY_RESPONSE.md)
- [故障排除](../TROUBLESHOOTING.md)
- [API 参考](../API_GUIDE.md)

## 获取帮助

- **文档中心**: [docs/](../)
- **问题反馈**: [GitHub Issues](https://github.com/crazyqin/nas-os/issues)
- **社区讨论**: [GitHub Discussions](https://github.com/crazyqin/nas-os/discussions)