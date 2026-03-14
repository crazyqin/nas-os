# Network API 文档

**版本**: v2.20.1  
**基础路径**: `/api/v1/network`

---

## 概述

NAS-OS 网络模块提供完整的网络管理功能：
- 🔌 网络接口配置
- 🌐 DDNS 动态域名
- 🔀 端口转发
- 🔥 防火墙规则

---

## 网络接口

### 列出网络接口

```bash
curl http://localhost:8080/api/v1/network/interfaces \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "name": "eth0",
      "type": "ethernet",
      "mac": "00:11:22:33:44:55",
      "ip": "192.168.1.100",
      "netmask": "255.255.255.0",
      "gateway": "192.168.1.1",
      "dns": ["8.8.8.8", "8.8.4.4"],
      "up": true,
      "speed": 1000,
      "rx_bytes": 1073741824,
      "tx_bytes": 536870912
    },
    {
      "name": "wlan0",
      "type": "wireless",
      "mac": "00:11:22:33:44:56",
      "ip": "192.168.1.101",
      "up": false
    }
  ]
}
```

### 获取接口详情

```bash
curl http://localhost:8080/api/v1/network/interfaces/eth0 \
  -H "Authorization: Bearer TOKEN"
```

### 配置接口

```bash
curl -X PUT http://localhost:8080/api/v1/network/interfaces/eth0 \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "dhcp": false,
    "ip": "192.168.1.100",
    "netmask": "255.255.255.0",
    "gateway": "192.168.1.1",
    "dns": ["8.8.8.8", "8.8.4.4"]
  }'
```

**配置参数**:
| 参数 | 类型 | 说明 |
|------|------|------|
| dhcp | bool | 是否使用 DHCP |
| ip | string | 静态 IP 地址 |
| netmask | string | 子网掩码 |
| gateway | string | 网关地址 |
| dns | []string | DNS 服务器列表 |

### 启用/禁用接口

```bash
curl -X POST http://localhost:8080/api/v1/network/interfaces/eth0/toggle \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"up": true}'
```

### 获取网络统计

```bash
curl http://localhost:8080/api/v1/network/stats \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "rx_bytes": 1073741824,
    "tx_bytes": 536870912,
    "rx_packets": 125000,
    "tx_packets": 89000,
    "rx_errors": 0,
    "tx_errors": 0,
    "connections": 45
  }
}
```

---

## DDNS 动态域名

### 列出 DDNS 配置

```bash
curl http://localhost:8080/api/v1/network/ddns \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "domain": "mynas.ddns.net",
      "provider": "noip",
      "enabled": true,
      "lastUpdate": "2026-03-14T10:00:00Z",
      "lastIP": "123.45.67.89",
      "status": "active"
    }
  ]
}
```

### 添加 DDNS 配置

```bash
curl -X POST http://localhost:8080/api/v1/network/ddns \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "mynas.ddns.net",
    "provider": "noip",
    "username": "user@example.com",
    "password": "your_password",
    "updateInterval": 300,
    "enabled": true
  }'
```

**支持的 DDNS 提供商**:
| Provider | 说明 |
|----------|------|
| noip | No-IP |
| duckdns | DuckDNS（免费） |
| cloudflare | Cloudflare DNS |
| dynu | Dynu |
| freedns | FreeDNS |
| custom | 自定义 DDNS |

### 获取 DDNS 详情

```bash
curl http://localhost:8080/api/v1/network/ddns/mynas.ddns.net \
  -H "Authorization: Bearer TOKEN"
```

### 更新 DDNS 配置

```bash
curl -X PUT http://localhost:8080/api/v1/network/ddns/mynas.ddns.net \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "newuser@example.com",
    "password": "new_password"
  }'
```

### 删除 DDNS 配置

```bash
curl -X DELETE http://localhost:8080/api/v1/network/ddns/mynas.ddns.net \
  -H "Authorization: Bearer TOKEN"
```

### 启用/禁用 DDNS

```bash
curl -X POST http://localhost:8080/api/v1/network/ddns/mynas.ddns.net/enable \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}'
```

### 手动刷新 DDNS

```bash
curl -X POST http://localhost:8080/api/v1/network/ddns/mynas.ddns.net/refresh \
  -H "Authorization: Bearer TOKEN"
```

---

## 端口转发

### 列出端口转发规则

```bash
curl http://localhost:8080/api/v1/network/portforwards \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "name": "web-server",
      "protocol": "tcp",
      "externalPort": 8080,
      "internalIP": "192.168.1.100",
      "internalPort": 80,
      "enabled": true,
      "comment": "Web Server"
    },
    {
      "name": "smb-share",
      "protocol": "tcp",
      "externalPort": 445,
      "internalIP": "192.168.1.100",
      "internalPort": 445,
      "enabled": true
    }
  ]
}
```

### 添加端口转发规则

```bash
curl -X POST http://localhost:8080/api/v1/network/portforwards \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "web-server",
    "protocol": "tcp",
    "externalPort": 8080,
    "internalIP": "192.168.1.100",
    "internalPort": 80,
    "sourceIP": "0.0.0.0/0",
    "enabled": true,
    "comment": "Web Server"
  }'
```

**参数说明**:
| 参数 | 类型 | 说明 |
|------|------|------|
| name | string | 规则名称 |
| protocol | string | 协议（tcp/udp） |
| externalPort | int | 外部端口 |
| internalIP | string | 内部 IP |
| internalPort | int | 内部端口 |
| sourceIP | string | 源 IP 限制（可选） |
| enabled | bool | 是否启用 |
| comment | string | 备注（可选） |

### 获取端口转发详情

```bash
curl http://localhost:8080/api/v1/network/portforwards/web-server \
  -H "Authorization: Bearer TOKEN"
```

### 更新端口转发规则

```bash
curl -X PUT http://localhost:8080/api/v1/network/portforwards/web-server \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "externalPort": 8888,
    "internalPort": 8080
  }'
```

### 删除端口转发规则

```bash
curl -X DELETE http://localhost:8080/api/v1/network/portforwards/web-server \
  -H "Authorization: Bearer TOKEN"
```

### 启用/禁用规则

```bash
curl -X POST http://localhost:8080/api/v1/network/portforwards/web-server/enable \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}'
```

### 获取规则状态

```bash
curl http://localhost:8080/api/v1/network/portforwards/web-server/status \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "status": "active",
    "connections": 15,
    "bytes_in": 1073741824,
    "bytes_out": 536870912
  }
}
```

### 列出活动的端口转发

```bash
curl http://localhost:8080/api/v1/network/portforwards/active \
  -H "Authorization: Bearer TOKEN"
```

---

## 防火墙

### 列出防火墙规则

```bash
curl http://localhost:8080/api/v1/network/firewall/rules \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "name": "allow-ssh",
      "chain": "INPUT",
      "protocol": "tcp",
      "port": 22,
      "source": "192.168.1.0/24",
      "target": "ACCEPT",
      "enabled": true,
      "comment": "Allow SSH from LAN"
    },
    {
      "name": "allow-web",
      "chain": "INPUT",
      "protocol": "tcp",
      "port": [80, 443],
      "source": "0.0.0.0/0",
      "target": "ACCEPT",
      "enabled": true
    },
    {
      "name": "block-country",
      "chain": "INPUT",
      "source": "geoip:CN",
      "target": "DROP",
      "enabled": false
    }
  ]
}
```

### 添加防火墙规则

```bash
curl -X POST http://localhost:8080/api/v1/network/firewall/rules \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "allow-web",
    "chain": "INPUT",
    "protocol": "tcp",
    "port": [80, 443],
    "source": "0.0.0.0/0",
    "target": "ACCEPT",
    "enabled": true,
    "comment": "Allow HTTP/HTTPS"
  }'
```

**规则参数**:
| 参数 | 类型 | 说明 |
|------|------|------|
| name | string | 规则名称 |
| chain | string | 链（INPUT/OUTPUT/FORWARD） |
| protocol | string | 协议（tcp/udp/icmp/all） |
| port | int/[]int | 端口或端口范围 |
| source | string | 源 IP/网段 |
| destination | string | 目标 IP/网段（可选） |
| target | string | 动作（ACCEPT/DROP/REJECT） |
| enabled | bool | 是否启用 |
| comment | string | 备注 |

### 获取规则详情

```bash
curl http://localhost:8080/api/v1/network/firewall/rules/allow-ssh \
  -H "Authorization: Bearer TOKEN"
```

### 更新防火墙规则

```bash
curl -X PUT http://localhost:8080/api/v1/network/firewall/rules/allow-ssh \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "10.0.0.0/8"
  }'
```

### 删除防火墙规则

```bash
curl -X DELETE http://localhost:8080/api/v1/network/firewall/rules/allow-ssh \
  -H "Authorization: Bearer TOKEN"
```

### 启用/禁用规则

```bash
curl -X POST http://localhost:8080/api/v1/network/firewall/rules/allow-ssh/enable \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}'
```

### 获取防火墙状态

```bash
curl http://localhost:8080/api/v1/network/firewall/status \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "enabled": true,
    "defaultPolicy": {
      "INPUT": "DROP",
      "OUTPUT": "ACCEPT",
      "FORWARD": "DROP"
    },
    "activeRules": 25,
    "blockedConnections": 1523
  }
}
```

### 列出活动规则

```bash
curl http://localhost:8080/api/v1/network/firewall/active \
  -H "Authorization: Bearer TOKEN"
```

### 设置默认策略

```bash
curl -X POST http://localhost:8080/api/v1/network/firewall/policy \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "chain": "INPUT",
    "policy": "DROP"
  }'
```

### 清空规则

```bash
curl -X POST http://localhost:8080/api/v1/network/firewall/flush \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"chain": "INPUT"}'
```

### 保存防火墙规则

```bash
curl -X POST http://localhost:8080/api/v1/network/firewall/save \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"path": "/etc/iptables/rules.v4"}'
```

### 恢复防火墙规则

```bash
curl -X POST http://localhost:8080/api/v1/network/firewall/restore \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"path": "/etc/iptables/rules.v4"}'
```

---

## 常用配置示例

### 允许外部访问 NAS Web 界面

```bash
# 添加防火墙规则
curl -X POST http://localhost:8080/api/v1/network/firewall/rules \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "allow-nas-web",
    "chain": "INPUT",
    "protocol": "tcp",
    "port": 8080,
    "target": "ACCEPT",
    "enabled": true
  }'
```

### 配置 DDNS 远程访问

```bash
# 1. 添加 DDNS
curl -X POST http://localhost:8080/api/v1/network/ddns \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "mynas.duckdns.org",
    "provider": "duckdns",
    "token": "your-duckdns-token",
    "enabled": true
  }'

# 2. 添加端口转发（如果需要）
curl -X POST http://localhost:8080/api/v1/network/portforwards \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "nas-web",
    "protocol": "tcp",
    "externalPort": 8080,
    "internalIP": "192.168.1.100",
    "internalPort": 8080,
    "enabled": true
  }'
```

### 限制 SSH 访问

```bash
# 仅允许内网 SSH 访问
curl -X POST http://localhost:8080/api/v1/network/firewall/rules \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "ssh-lan-only",
    "chain": "INPUT",
    "protocol": "tcp",
    "port": 22,
    "source": "192.168.1.0/24",
    "target": "ACCEPT",
    "enabled": true
  }'
```

---

## 错误处理

### 错误响应格式

```json
{
  "code": 400,
  "message": "无效的 IP 地址格式",
  "data": null
}
```

### 常见错误码

| 代码 | 说明 |
|------|------|
| 400 | 请求参数错误 |
| 401 | 未认证 |
| 403 | 权限不足（需要管理员权限） |
| 404 | 资源不存在 |
| 500 | 服务器内部错误 |

---

**最后更新**: 2026-03-14