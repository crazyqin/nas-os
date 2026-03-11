# 集群设置指南

## 概述

NAS-OS 集群支持提供多节点协同工作能力，包括：

- **多节点发现和管理** - 自动发现局域网内的 NAS-OS 节点
- **分布式存储同步** - 跨节点数据同步和复制
- **负载均衡** - 请求分发和流量管理
- **高可用故障转移** - 主节点自动选举和故障恢复

## 快速开始

### 1. 启用集群功能

在配置文件中启用集群：

```yaml
# /etc/nas-os/config.yaml
cluster:
  enabled: true
  node_id: "node-1"  # 每个节点唯一标识
  data_dir: "/var/lib/nas-os"
  
  # 集群发现配置
  discovery:
    port: 8081
    heartbeat_interval: 5  # 秒
    heartbeat_timeout: 15  # 秒
  
  # 高可用配置
  ha:
    bind_port: 8082
    election_timeout: 1000  # 毫秒
```

### 2. 启动第一个节点（主节点）

```bash
sudo nasd --config /etc/nas-os/config.yaml
```

第一个启动的节点自动成为主节点（Leader）。

### 3. 添加工作节点

在第二台机器上配置并启动：

```yaml
# /etc/nas-os/config-node2.yaml
cluster:
  enabled: true
  node_id: "node-2"  # 不同的节点 ID
  data_dir: "/var/lib/nas-os"
```

```bash
sudo nasd --config /etc/nas-os/config-node2.yaml
```

节点会自动通过 mDNS 发现并加入集群。

## 架构说明

```
                    ┌─────────────────┐
                    │   Load Balancer │
                    │   (Round Robin) │
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
       ┌──────▼──────┐ ┌─────▼──────┐ ┌────▼──────┐
       │   Node 1    │ │   Node 2   │ │   Node 3  │
       │   (Leader)  │ │ (Follower) │ │ (Follower)│
       │             │ │            │ │           │
       │  - Raft     │ │  - Storage │ │  - Storage│
       │  - Sync     │ │  - Sync    │ │  - Sync   │
       └──────┬──────┘ └─────┬──────┘ └────┬──────┘
              │              │              │
              └──────────────┼──────────────┘
                             │
                    ┌────────▼────────┐
                    │  Shared Storage │
                    │  (GlusterFS)    │
                    └─────────────────┘
```

## API 使用

### 节点管理

#### 获取节点列表
```bash
curl http://localhost:8080/api/v1/cluster/nodes
```

响应：
```json
{
  "success": true,
  "data": [
    {
      "id": "node-1",
      "hostname": "nas-server-1",
      "ip": "192.168.1.101",
      "port": 8080,
      "role": "master",
      "status": "online",
      "heartbeat": "2026-03-11T18:00:00Z"
    }
  ],
  "count": 1
}
```

#### 获取节点详情
```bash
curl http://localhost:8080/api/v1/cluster/nodes/node-1
```

#### 移除节点
```bash
curl -X DELETE http://localhost:8080/api/v1/cluster/nodes/node-2
```

### 存储同步

#### 创建同步规则
```bash
curl -X POST http://localhost:8080/api/v1/cluster/sync/rules \
  -H "Content-Type: application/json" \
  -d '{
    "name": "照片备份",
    "source_node": "node-1",
    "target_nodes": ["node-2", "node-3"],
    "source_path": "/data/photos",
    "target_path": "/backup/photos",
    "sync_mode": "async",
    "schedule": "0 2 * * *",
    "enabled": true
  }'
```

#### 获取同步规则列表
```bash
curl http://localhost:8080/api/v1/cluster/sync/rules
```

#### 手动触发同步
```bash
curl -X POST http://localhost:8080/api/v1/cluster/sync/trigger \
  -H "Content-Type: application/json" \
  -d '{"rule_id": "rule-123"}'
```

#### 获取同步状态
```bash
curl http://localhost:8080/api/v1/cluster/sync/status
```

### 负载均衡

#### 获取负载均衡配置
```bash
curl http://localhost:8080/api/v1/cluster/lb/config
```

#### 更新负载均衡算法
```bash
curl -X PUT http://localhost:8080/api/v1/cluster/lb/config \
  -H "Content-Type: application/json" \
  -d '{
    "algorithm": "least-conn",
    "health_check_url": "/api/v1/health",
    "health_interval": 10,
    "sticky_session": true
  }'
```

#### 获取后端节点
```bash
curl http://localhost:8080/api/v1/cluster/lb/backends
```

#### 获取统计信息
```bash
curl http://localhost:8080/api/v1/cluster/lb/stats
```

### 高可用

#### 获取 HA 状态
```bash
curl http://localhost:8080/api/v1/cluster/ha/status
```

响应：
```json
{
  "success": true,
  "data": {
    "state": "leader",
    "leader": "node-1",
    "leader_addr": "192.168.1.101:8082",
    "term": 5,
    "peers": [
      {
        "id": "node-2",
        "address": "192.168.1.102:8082",
        "voter": true,
        "healthy": true
      }
    ]
  }
}
```

#### 手动故障转移
```bash
curl -X POST http://localhost:8080/api/v1/cluster/ha/failover \
  -H "Content-Type: application/json" \
  -d '{"target_node_id": "node-2"}'
```

#### 获取故障转移历史
```bash
curl http://localhost:8080/api/v1/cluster/ha/history?limit=10
```

## 同步模式说明

### 异步同步 (async)
- 定时执行，不阻塞主进程
- 适合大数据量、非实时场景
- 配置示例：`"sync_mode": "async"`

### 同步同步 (sync)
- 写入时同步等待目标节点确认
- 数据一致性高，延迟较大
- 配置示例：`"sync_mode": "sync"`

### 实时同步 (realtime)
- 使用 inotify 监听文件变化
- 近实时复制，延迟最小
- 配置示例：`"sync_mode": "realtime"`

## 负载均衡算法

### 轮询 (round-robin)
- 默认算法
- 按顺序分发请求到各节点
- 适合节点性能相近的场景

### 最少连接 (least-conn)
- 分发到当前连接数最少的节点
- 适合长连接场景
- 配置：`"algorithm": "least-conn"`

### 加权 (weighted)
- 根据节点权重分配流量
- 适合异构集群
- 配置：`"algorithm": "weighted"`

### IP 哈希 (ip-hash)
- 同一 IP 的请求总是分发到同一节点
- 支持会话保持
- 配置：`"algorithm": "ip-hash"`

## 故障转移机制

### 自动故障检测
- 心跳超时检测（默认 15 秒）
- 健康检查（HTTP 端点）
- 连续失败阈值（默认 3 次）

### 领导者选举
- 基于 Raft 共识算法
- 自动选举新领导者
- 选举超时：1 秒

### 脑裂防护
- 法定人数（Quorum）机制
- 少数派节点自动降级
- 数据一致性保证

## 监控与告警

### 节点监控指标
- CPU 使用率
- 内存使用率
- 磁盘使用率
- 网络连接数
- 心跳状态

### 同步监控
- 同步任务状态
- 同步延迟
- 失败次数
- 最后同步时间

### 负载均衡监控
- 请求总数
- 活跃请求数
- 失败请求数
- 平均响应时间
- 各后端节点负载

## 最佳实践

### 1. 节点部署
- 至少 3 个节点以保证高可用
- 节点分布在不同的物理机器
- 使用稳定的网络连接

### 2. 存储同步
- 重要数据使用同步模式
- 大文件使用异步定时同步
- 定期验证数据一致性

### 3. 负载均衡
- 启用健康检查
- 配置合适的超时时间
- 监控后端节点状态

### 4. 高可用
- 定期测试故障转移
- 监控 Raft 日志大小
- 配置自动快照

## 故障排查

### 节点无法发现
```bash
# 检查 mDNS 服务
sudo systemctl status avahi-daemon

# 检查防火墙
sudo ufw allow 5353/udp

# 测试 mDNS 发现
avahi-browse _nasos._tcp -r
```

### 同步失败
```bash
# 检查 rsync
which rsync
rsync --version

# 检查 SSH 连接
ssh node2@192.168.1.102

# 查看同步日志
journalctl -u nas-os -f
```

### 领导者选举失败
```bash
# 检查 Raft 状态
curl http://localhost:8080/api/v1/cluster/ha/status

# 检查节点间网络连通性
ping -c 3 node2

# 查看 Raft 日志
tail -f /var/lib/nas-os/raft/log.db
```

## 性能调优

### 网络优化
```yaml
cluster:
  heartbeat_interval: 3    # 降低心跳间隔
  heartbeat_timeout: 10    # 降低超时时间
```

### 同步优化
```yaml
sync:
  parallel_jobs: 4         # 增加并行任务数
  max_retries: 5           # 增加重试次数
  retry_delay: 30          # 降低重试延迟
```

### Raft 优化
```yaml
ha:
  heartbeat_timeout: 500   # 降低心跳超时（毫秒）
  election_timeout: 500    # 降低选举超时
  snapshot_interval: 60    # 增加快照频率
```

## 安全考虑

### 1. 节点认证
- 使用 TLS 加密节点间通信
- 配置节点间共享密钥
- 限制可加入集群的节点

### 2. 数据加密
- 同步数据使用 SSH 加密
- 敏感数据加密存储
- 启用传输层加密

### 3. 访问控制
- API 访问需要认证
- 限制管理操作权限
- 审计日志记录

## 升级与迁移

### 滚动升级
1. 逐个下线工作节点
2. 升级节点软件
3. 重新启动并加入集群
4. 重复直到所有节点升级完成

### 数据迁移
1. 创建全量同步规则
2. 等待初始同步完成
3. 切换到实时同步模式
4. 验证数据一致性

---

*最后更新：2026-03-11*
