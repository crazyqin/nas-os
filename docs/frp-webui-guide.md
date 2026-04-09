# FRP WebUI 用户指南

## 功能概述

nas-os FRP WebUI管理界面提供图形化的内网穿透隧道管理功能，对标飞牛FN Connect用户体验。

### 核心功能

| 功能 | 说明 | 状态 |
|------|------|------|
| 隧道列表管理 | 创建、编辑、删除隧道 | ✅ |
| 实时状态监控 | 连接状态、流量统计 | ✅ |
| 节点选择 | 自动选择最优免费节点 | ✅ |
| WebSocket推送 | 实时状态更新 | ✅ |
| 配置导入导出 | 批量配置管理 | ✅ |

## WebUI访问

访问地址：`https://your-nas/admin/frp`

### 界面布局

```
┌─────────────────────────────────────────────────┐
│ FRP内网穿透管理                    [刷新] [设置]  │
├─────────────────────────────────────────────────┤
│ 状态: 已连接 ✅                                   │
│ 节点: cn-connect-1 (connect.fnos.cn:7000)       │
│ 公网地址: https://myweb.connect.fnos.cn         │
│ 运行时间: 2小时30分                               │
├─────────────────────────────────────────────────┤
│ 障道列表                    [+ 新建隧道]         │
│ ┌───────────────────────────────────────────┐   │
│ │ 🟢 web    HTTP  本地80 → myweb.fnos.cn     │   │
│ │ 🟢 ssh    TCP   本地22 → :2222             │   │
│ │ ⚪ db     TCP   本地3306 → :3306  [已停止] │   │
│ └───────────────────────────────────────────┘   │
├─────────────────────────────────────────────────┤
│ 流量统计                                          │
│ 今日: 1.2GB | 本周: 8.5GB | 连接数: 125          │
└─────────────────────────────────────────────────┘
```

## 使用说明

### 1. 一键连接

首次使用点击"一键连接"按钮，系统自动：
- 选择最优免费节点
- 创建默认HTTP隧道
- 分配公网访问地址

### 2. 创建隧道

点击"新建隧道"按钮，填写配置：

| 配置项 | 说明 | 示例 |
|--------|------|------|
| 隧道名称 | 标识名称 | `web` |
| 隧道类型 | HTTP/TCP/HTTPS | `HTTP` |
| 本地端口 | 内网服务端口 | `8080` |
| 子域名 | HTTP隧道可选 | `myweb` |
| 远程端口 | TCP隧道必填 | `2222` |

### 3. 隧道操作

- **启动**: 点击隧道行启动按钮
- **停止**: 点击隧道行停止按钮
- **编辑**: 点击隧道行编辑按钮
- **删除**: 点击隧道行删除按钮

### 4. 状态监控

实时显示：
- 🟢 运行中 - 隧道正常工作
- 🟡 重连中 - 正在重新连接
- 🔴 已停止 - 隧道已停止
- ⚪ 禁用 - 隧道已禁用

### 5. 节点管理

点击"设置"→"节点管理"：
- 查看可用节点列表
- 测试节点延迟
- 手动切换节点
- 添加自定义节点

## 配置示例

### HTTP隧道（Web服务）

```json
{
  "name": "web",
  "type": "http",
  "local_port": 80,
  "subdomain": "myweb",
  "enable_tls": true
}
```

访问地址：`https://myweb.connect.fnos.cn`

### TCP隧道（SSH）

```json
{
  "name": "ssh",
  "type": "tcp",
  "local_port": 22,
  "remote_port": 2222
}
```

连接命令：`ssh -p 2222 user@connect.fnos.cn`

### HTTPS隧道（安全Web）

```json
{
  "name": "secure-web",
  "type": "https",
  "local_port": 443,
  "custom_domain": "mysite.example.com",
  "enable_tls": true
}
```

### 多隧道配置

```json
{
  "tunnels": [
    {"name": "web", "type": "http", "local_port": 80, "subdomain": "web"},
    {"name": "ssh", "type": "tcp", "local_port": 22, "remote_port": 2222},
    {"name": "db", "type": "tcp", "local_port": 3306, "remote_port": 3306}
  ]
}
```

## API后端说明

WebUI后端API提供完整的隧道管理接口：

### 隧道管理API

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/frp/tunnels` | GET | 获取隧道列表 |
| `/api/v1/frp/tunnels` | POST | 创建新隧道 |
| `/api/v1/frp/tunnels/:id` | GET | 获取隧道详情 |
| `/api/v1/frp/tunnels/:id` | PUT | 更新隧道配置 |
| `/api/v1/frp/tunnels/:id` | DELETE | 删除隧道 |
| `/api/v1/frp/tunnels/:id/start` | POST | 启动隧道 |
| `/api/v1/frp/tunnels/:id/stop` | POST | 停止隧道 |

### 状态监控API

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/frp/status` | GET | 全局连接状态 |
| `/api/v1/frp/nodes` | GET | 可用节点列表 |
| `/api/v1/frp/nodes/health` | GET | 节点健康状态 |
| `/ws/frp/status` | WS | WebSocket实时推送 |

### 请求示例

创建HTTP隧道：
```bash
curl -X POST https://your-nas/api/v1/frp/tunnels \
  -H "Content-Type: application/json" \
  -d '{"name":"web","type":"http","local_port":80,"subdomain":"myweb"}'
```

获取隧道状态：
```bash
curl https://your-nas/api/v1/frp/status
```

## 安全配置

### 认证Token

在"设置"→"安全配置"中设置：
- 认证Token（防止滥用）
- 白名单IP（限制访问）
- 流量限制（防止超限）

### TLS加密

默认启用TLS加密传输：
- 端到端加密
- 证书自动管理
- HTTPS隧道自动配置

### 访问控制

配置防火墙规则：
```bash
# 仅允许特定IP访问隧道
iptables -A INPUT -p tcp --dport 2222 -s 192.168.1.0/24 -j ACCEPT
```

## 故障排查

### 连接失败

检查步骤：
1. 查看WebUI状态面板错误信息
2. 检查节点健康状态
3. 确认本地服务运行
4. 检查网络连通性

### 频繁断连

解决方案：
1. 在"设置"中调整重连参数：
   - `auto_reconnect: true`
   - `reconnect_interval: 30`
   - `max_reconnect: 5`
2. 切换到其他稳定节点
3. 检查本地网络稳定性

### 性能问题

优化建议：
1. 减少同时运行的隧道数量
2. 选择低延迟节点
3. 启用流量压缩

## 与竞品对比

| 功能 | nas-os FRP WebUI | 飞牛FN Connect | 状态 |
|------|------------------|----------------|------|
| 图形化管理 | ✅ | ✅ | 对标 |
| 一键连接 | ✅ | ✅ | 对标 |
| 免费节点 | ✅ 中美欧 | ✅ | 对标 |
| WebSocket实时 | ✅ | ✅ | 对标 |
| 自定义节点 | ✅ | ❌ | **领先** |
| 多隧道管理 | ✅ | ✅ | 对标 |

## 常见问题

**Q: 如何获取公网访问地址？**
A: HTTP隧道创建后，系统自动分配子域名，如`https://myweb.connect.fnos.cn`

**Q: 免费节点有什么限制？**
A: 免费节点带宽限制10Mbps，月流量限制100GB，适合个人使用

**Q: 如何添加自定义节点？**
A: 在"设置"→"节点管理"中添加自定义FRP服务器配置

**Q: 支持哪些隧道类型？**
A: 支持HTTP、HTTPS、TCP、UDP四种隧道类型

---

**文档版本**: v2.436.0
**更新日期**: 2026-04-09
**部门**: 礼部品牌内容