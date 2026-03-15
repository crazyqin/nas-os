# NAS-OS v2.56.0 发布公告

**发布日期**: 2026-03-15  
**版本类型**: Stable

---

## 🎉 新功能介绍

### 配额管理 API 增强

v2.56.0 带来了完善的配额管理 RESTful API，支持更精细的存储资源控制：

- **用户配额管理**
  - 设置单个用户的存储配额限制
  - 查询用户配额使用情况
  - 配额超限自动告警

- **目录配额管理**
  - 设置目录级别的存储限制
  - 实时统计目录空间占用
  - 配额使用趋势分析

- **配额统计接口**
  - 全局配额使用概览
  - 按用户/组/目录分类统计
  - 历史趋势图表数据

```bash
# 设置用户配额
curl -X POST http://localhost:8080/api/v1/quota/user/alice \
  -H "Content-Type: application/json" \
  -d '{"limit_gb": 100}'

# 查询目录配额
curl http://localhost:8080/api/v1/quota/dir/data/photos

# 获取使用趋势
curl http://localhost:8080/api/v1/quota/trends?days=30
```

### WebSocket 广播系统

全新的实时通信能力，让 NAS 状态变更即时可见：

- **全局广播机制**
  - 支持房间广播（订阅特定事件类型）
  - 支持全员广播（系统级通知）
  - 消息优先级队列，重要消息优先送达

- **实时事件推送**
  - 系统状态变更实时推送
  - 存储事件通知（卷创建/删除/快照完成）
  - 用户操作审计实时流
  - 可配置的事件订阅

- **连接管理**
  - 自动心跳检测，断线重连
  - 连接池管理，高效复用
  - 内存占用优化，降低 20%

```javascript
// WebSocket 客户端示例
const ws = new WebSocket('ws://localhost:8080/api/v1/ws');

// 订阅存储事件
ws.send(JSON.stringify({
  action: 'subscribe',
  rooms: ['storage', 'system']
}));

// 接收实时推送
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Event:', data.type, data.payload);
};
```

### API 文档增强

- 配额管理 API 完整 Swagger 注释
- WebSocket 事件类型文档
- 请求/响应示例更新
- 在线 API 文档可交互测试

---

## 📦 升级指南

### 从 v2.55.0 升级

#### 方式一：二进制文件升级

```bash
# 停止服务
sudo systemctl stop nas-os

# 备份配置
sudo cp -r /etc/nas-os /etc/nas-os.bak

# 下载新版本 (根据架构选择)
wget https://github.com/crazyqin/nas-os/releases/download/v2.56.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# 启动服务
sudo systemctl start nas-os

# 验证版本
nasd --version
```

#### 方式二：Docker 升级

```bash
# 拉取新镜像
docker pull ghcr.io/crazyqin/nas-os:v2.56.0

# 停止旧容器
docker stop nasd
docker rm nasd

# 启动新容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.56.0

# 验证
docker logs nasd | head -5
```

### 配置变更

v2.56.0 无破坏性配置变更，可直接使用现有配置。

新增可选配置项（`/etc/nas-os/config.yaml`）：

```yaml
websocket:
  enabled: true
  heartbeat_interval: 30s
  max_connections: 1000

quota:
  enabled: true
  check_interval: 5m
  alert_threshold: 90%
```

---

## ⚠️ 已知问题

1. **WebSocket 连接数限制**
   - 默认最大连接数 1000，超过后新连接会被拒绝
   - 解决方法：在配置中调高 `websocket.max_connections`

2. **配额统计延迟**
   - 大容量目录（>10TB）首次统计可能需要 30 秒以上
   - 建议：首次运行时在后台预计算配额统计

3. **ARMv7 内存占用**
   - WebSocket 开启后内存占用略有增加（约 10-20MB）
   - 低内存设备（<512MB）建议关闭 WebSocket 或减少最大连接数

---

## 📥 下载

| 平台 | 架构 | 下载链接 |
|------|------|----------|
| Linux | AMD64 | [nasd-linux-amd64](https://github.com/crazyqin/nas-os/releases/download/v2.56.0/nasd-linux-amd64) |
| Linux | ARM64 | [nasd-linux-arm64](https://github.com/crazyqin/nas-os/releases/download/v2.56.0/nasd-linux-arm64) |
| Linux | ARMv7 | [nasd-linux-armv7](https://github.com/crazyqin/nas-os/releases/download/v2.56.0/nasd-linux-armv7) |

**Docker 镜像**:
```bash
docker pull ghcr.io/crazyqin/nas-os:v2.56.0
```

---

## 🙏 贡献者

感谢以下部门的贡献：

- **户部** - 配额管理 API
- **兵部** - WebSocket 广播系统
- **礼部** - API 文档、版本发布

---

## 📚 相关链接

- [完整更新日志](../CHANGELOG.md)
- [API 文档](./API_GUIDE.md)
- [用户指南](./user-guide/)
- [GitHub Issues](https://github.com/crazyqin/nas-os/issues)

---

**NAS-OS 团队**  
2026-03-15