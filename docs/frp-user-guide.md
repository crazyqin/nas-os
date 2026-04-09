# FRP内网穿透用户指南

## 快速开始

### 一键连接

nas-os提供类似飞牛FN Connect的零配置内网穿透服务。

```bash
# 通过API一键连接
POST /api/v1/frp/quick-connect
{
  "local_port": 8080,
  "tunnel_name": "myweb"
}
```

系统将自动：
1. 选择最优免费节点
2. 分配公网端口
3. 建立隧道连接

### 查看连接状态

```bash
GET /api/v1/frp/status
```

返回：
```json
{
  "connected": true,
  "public_url": "connect.fnos.cn:12345",
  "uptime": "2h30m",
  "tunnels": [...]
}
```

## 隧道类型

### TCP隧道

适用于SSH、数据库等TCP服务：

```json
POST /api/v1/frp/tunnels
{
  "name": "ssh",
  "type": "tcp",
  "local_port": 22,
  "remote_port": 2222
}
```

访问: `connect.fnos.cn:2222`

### HTTP隧道

适用于Web服务：

```json
POST /api/v1/frp/tunnels
{
  "name": "web",
  "type": "http",
  "local_port": 80,
  "subdomain": "myweb"
}
```

访问: `https://myweb.connect.fnos.cn`

### HTTPS隧道

自动TLS加密：

```json
{
  "name": "secure",
  "type": "https",
  "local_port": 443,
  "enable_tls": true
}
```

## 免费节点

### 中国节点

| 节点ID | 地址 | 状态 |
|--------|------|------|
| cn-connect-1 | connect.fnos.cn:7000 | ✅ |
| cn-tunnel-1 | tunnel.fnos.cn:7000 | ✅ |

### 国际节点

| 节点ID | 地址 | 区域 |
|--------|------|------|
| us-connect-1 | connect.fnos.us:7000 | 美国 |
| eu-connect-1 | connect.fnos.eu:7000 | 欧洲 |

### 自动选择最优节点

系统会根据延迟和可用性自动选择最优节点。

## 高级配置

### 自定义节点

```json
POST /api/v1/frp/nodes
{
  "id": "custom-1",
  "server_addr": "my.server.com",
  "server_port": 7000,
  "tls_enable": true
}
```

### 认证Token

```json
PUT /api/v1/frp/config
{
  "token": "your-auth-token"
}
```

### 重连设置

```json
{
  "auto_reconnect": true,
  "max_reconnect": 5,
  "reconnect_interval": 30
}
```

## WebUI管理

访问 `https://your-nas/admin/frp` 进行可视化管理：

- 📊 连接状态监控
- 📝 隧道配置管理
- 🌐 节点选择
- 📈 流量统计

## 安全注意事项

1. **认证Token**: 建议设置认证Token防止滥用
2. **端口限制**: 不要暴露敏感服务端口
3. **访问控制**: 配合防火墙规则限制访问
4. **日志审计**: 定期检查隧道访问日志

## 故障排查

### 连接失败

检查：
1. 节点是否在线: `GET /api/v1/frp/nodes/health`
2. 本地服务是否运行
3. 网络是否通畅

### 频繁断连

调整：
- 增加心跳间隔
- 启用自动重连
- 切换其他节点

### 性能问题

优化：
- 减少隧道数量
- 选择低延迟节点
- 检查带宽限制

## API完整列表

| 端点 | 方法 | 说明 |
|------|------|------|
| /api/v1/frp/status | GET | 获取状态 |
| /api/v1/frp/quick-connect | POST | 一键连接 |
| /api/v1/frp/tunnels | GET/POST | 隧道管理 |
| /api/v1/frp/tunnels/:id | GET/DELETE | 单隧道操作 |
| /api/v1/frp/config | GET/PUT | 配置管理 |
| /api/v1/frp/nodes | GET | 节点列表 |
| /api/v1/frp/nodes/health | POST | 健康检查 |

---

**文档版本**: v1.0
**更新日期**: 2026-04-09
**部门**: 礼部品牌内容