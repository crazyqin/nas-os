# NAS-OS v2.5.0 管理员手册

> **版本**: v2.5.0  
> **发布日期**: 2026-03-14  
> **目标读者**: 系统管理员、运维工程师

---

## 目录

1. [系统架构](#系统架构)
2. [高级配置](#高级配置)
3. [安全管理](#安全管理)
4. [性能优化](#性能优化)
5. [监控与告警](#监控与告警)
6. [备份策略](#备份策略)
7. [维护操作](#维护操作)
8. [故障排查](#故障排查)
9. [升级与迁移](#升级与迁移)
10. [最佳实践](#最佳实践)
11. [v2.5.0 新特性](#v250-新特性)

---

## 系统架构

### 组件概览

```
┌─────────────────────────────────────────────────────────────────┐
│                         客户端层                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐       │
│  │ Web 浏览器 │  │ SMB 客户端 │  │ NFS 客户端 │  │  API 调用  │       │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       NAS-OS 服务层                              │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    HTTP Server (8080)                    │   │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐    │   │
│  │  │ Auth    │  │ Volume  │  │ Share   │  │ User    │    │   │
│  │  │ Handler │  │ Handler │  │ Handler │  │ Handler │    │   │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘    │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                     核心服务层                           │   │
│  │  ┌───────────┐  ┌───────────┐  ┌───────────┐           │   │
│  │  │ Storage   │  │ SMB/NFS   │  │ User/RBAC │           │   │
│  │  │ Manager   │  │ Service   │  │ Manager   │           │   │
│  │  └───────────┘  └───────────┘  └───────────┘           │   │
│  │  ┌───────────┐  ┌───────────┐  ┌───────────┐           │   │
│  │  │ Snapshot  │  │ Encrypt   │  │ HA Cluster│ 🆕        │   │
│  │  │ Replicat. │  │ Manager   │  │ Manager   │           │   │
│  │  └───────────┘  └───────────┘  └───────────┘           │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        系统层                                   │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐   │
│  │  btrfs    │  │  Samba    │  │  NFS      │  │  Systemd  │   │
│  │  Kernel   │  │  Daemon   │  │  Server   │  │  Service  │   │
│  └───────────┘  └───────────┘  └───────────┘  └───────────┘   │
│  ┌───────────┐  ┌───────────┐                                 │
│  │   LUKS    │  │ Keepalived│ 🆕                             │
│  │  加密层    │  │   HA      │                                 │
│  └───────────┘  └───────────┘                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 进程结构

```bash
# 查看 NAS-OS 相关进程
ps aux | grep -E 'nasd|smbd|nfsd|keepalived'

# 输出示例：
# root     1234  0.1  0.5 123456 78901 ?  Ssl  10:00   1:23 /usr/local/bin/nasd
# root     1235  0.0  0.1  12345  2345 ?    S    10:00   0:05 /usr/sbin/smbd
# root     1236  0.0  0.1  12345  2345 ?    S    10:00   0:03 /usr/sbin/nmbd
# root     1237  0.0  0.0      0     0 ?    S    10:00   0:00 [nfsd]
# root     1238  0.0  0.1   5678  1234 ?    Ss   10:00   0:02 /usr/sbin/keepalived
```

---

## v2.5.0 新特性

### 1. 快照复制 (Snapshot Replication)

v2.5.0 引入了跨节点快照复制功能，支持灾难恢复和数据冗余。

#### 配置快照复制

```yaml
# /etc/nas-os/config.yaml
replication:
  enabled: true
  nodes:
    - id: node-1
      address: 192.168.1.100
      port: 8080
    - id: node-2
      address: 192.168.1.101
      port: 8080
  
  tasks:
    - name: "data-replication"
      source_volume: "data"
      target_node: "node-2"
      target_path: "/backup/data"
      mode: "incremental"      # full 或 incremental
      schedule: "0 */6 * * *"  # 每6小时
      bandwidth_limit: 10485760  # 10MB/s
      retention: 7             # 保留7天
      encryption: true         # 传输加密
```

#### 增量复制原理

```
源节点                          目标节点
┌─────────┐                    ┌─────────┐
│ Snap A  │ ──────── 全量 ────►│ Snap A  │
│ Snap B  │ ──────── 增量 ────►│ Snap B  │
│ Snap C  │ ──────── 增量 ────►│ Snap C  │
└─────────┘                    └─────────┘

增量复制只传输差异块，大幅减少网络带宽和时间
```

#### 管理命令

```bash
# 列出复制任务
nasctl replication list

# 创建复制任务
nasctl replication create \
  --name data-backup \
  --source data \
  --target node-2:/backup/data \
  --schedule "0 */6 * * *"

# 手动触发复制
nasctl replication run data-backup

# 查看复制状态
nasctl replication status data-backup

# 暂停/恢复复制
nasctl replication pause data-backup
nasctl replication resume data-backup
```

---

### 2. 磁盘加密管理

v2.5.0 增强了磁盘加密功能，支持 LUKS 加密和密钥管理。

#### 启用磁盘加密

```bash
# 1. 检查设备是否支持加密
nasctl encryption check /dev/sdb1

# 2. 启用加密（会清除数据！）
nasctl encryption enable /dev/sdb1 \
  --passphrase "your-secure-passphrase" \
  --key-size 512 \
  --cipher aes-xts-plain64

# 3. 生成密钥文件（可选）
nasctl encryption generate-keyfile /etc/nas-os/encryption/keyfile

# 4. 添加密钥文件到加密设备
nasctl encryption add-key /dev/sdb1 \
  --keyfile /etc/nas-os/encryption/keyfile
```

#### 加密配置

```yaml
# /etc/nas-os/config.yaml
encryption:
  enabled: true
  default_cipher: aes-xts-plain64
  key_size: 512
  keyfile: /etc/nas-os/encryption/keyfile
  
  devices:
    - device: /dev/sdb1
      name: data_crypt
      auto_unlock: true
      
  key_rotation:
    enabled: true
    schedule: "0 0 1 * *"  # 每月1日
    reminder_days: 7
```

#### 密钥轮换

```bash
# 手动轮换密钥
nasctl encryption rotate-key /dev/sdb1 \
  --old-passphrase "old-passphrase" \
  --new-passphrase "new-passphrase"

# 自动轮换（已配置定时任务）
nasctl encryption auto-rotate --dry-run
```

#### 加密性能优化

```bash
# 检查 AES-NI 支持
cat /proc/cpuinfo | grep aes

# 启用硬件加速
echo "options dm_crypt use_tasklet=1" > /etc/modprobe.d/dm-crypt.conf

# 性能测试
cryptsetup benchmark
```

---

### 3. 高可用增强

v2.5.0 增强了高可用集群功能，优化了故障检测和自动故障转移。

#### 架构

```
                    ┌─────────────┐
                    │   虚拟 IP    │
                    │ 192.168.1.99│
                    └──────┬──────┘
                           │
         ┌─────────────────┼─────────────────┐
         │                 │                 │
         ▼                 ▼                 ▼
   ┌──────────┐      ┌──────────┐      ┌──────────┐
   │  Node 1  │◄────►│  Node 2  │◄────►│  Node 3  │
   │ (Primary)│ 心跳 │(Secondary)│ 心跳 │(Secondary)│
   │ .1.100   │      │ .1.101   │      │ .1.102   │
   └──────────┘      └──────────┘      └──────────┘
         │                 │                 │
         └─────────────────┼─────────────────┘
                           │
                    ┌──────▼──────┐
                    │  共享存储    │
                    │  (iSCSI)    │
                    └─────────────┘
```

#### 配置高可用集群

```yaml
# /etc/nas-os/config.yaml
cluster:
  enabled: true
  cluster_name: nas-cluster
  
  this_node:
    id: node-1
    hostname: nas-primary
    priority: 100
    
  nodes:
    - id: node-1
      hostname: nas-primary
      address: 192.168.1.100
      priority: 100
    - id: node-2
      hostname: nas-secondary
      address: 192.168.1.101
      priority: 90
      
  virtual_ip:
    address: 192.168.1.99
    interface: eth0
    
  failover:
    enabled: true
    detection_interval: 2s
    failure_threshold: 3
    auto_failback: true
    
  split_brain:
    detection: quorum
    quorum_votes: 2
```

#### 故障检测优化

```bash
# 配置健康检查
nasctl ha health-check configure \
  --interval 2s \
  --timeout 5s \
  --retries 3

# 自定义检查脚本
nasctl ha health-check add-script /usr/local/bin/check_nas.sh

# 查看集群状态
nasctl ha status

# 手动故障转移
nasctl ha failover --target node-2

# 查看故障历史
nasctl ha failover-history
```

---

### 4. 备份增强

v2.5.0 增强了备份功能，支持增量备份、备份加密和备份验证。

#### 增量备份

```bash
# 创建增量备份任务
nasctl backup create \
  --name daily-incremental \
  --source /data \
  --destination /backup/data \
  --mode incremental \
  --schedule "0 2 * * *" \
  --retention 30

# 基于快照的增量备份
nasctl backup create \
  --name snapshot-backup \
  --source /data \
  --destination /backup/data \
  --snapshot-based \
  --schedule "0 3 * * *"
```

#### 备份加密

```yaml
# /etc/nas-os/config.yaml
backup:
  encryption:
    enabled: true
    algorithm: aes-256-gcm
    key_derivation: argon2id
```

```bash
# 启用备份加密
nasctl backup encrypt backup-001 --enable

# 设置加密密码
nasctl backup set-passphrase backup-001
```

#### 备份验证

```bash
# 验证备份完整性
nasctl backup verify backup-001

# 验证结果示例
# ✅ 校验和验证通过: 1256/1256 文件
# ✅ 元数据完整性: 通过
# ✅ 可恢复性测试: 通过
# 验证完成，用时 5m32s

# 定期自动验证
nasctl backup auto-verify --schedule "0 4 * * 0"  # 每周日凌晨4点
```

---

## 监控与告警

### WebUI 监控大屏

v2.5.0 提供了全新的监控大屏，支持全屏展示。

访问地址：`http://<服务器IP>:8080/pages/dashboard-large.html`

功能特性：
- 实时 CPU/内存/磁盘/网络监控
- 资源使用趋势图
- 服务状态监控
- 磁盘温度和健康状态
- 告警实时展示
- 支持全屏模式

### 告警中心

访问地址：`http://<服务器IP>:8080/pages/alerts.html`

功能特性：
- 告警分类统计
- 告警规则管理
- 通知渠道配置
- 告警确认和解决
- 测试告警功能

### 告警规则配置

```yaml
# /etc/nas-os/config.yaml
monitoring:
  alerts:
    rules:
      - name: disk_usage_warning
        metric: disk_usage
        operator: ">"
        threshold: 80
        level: warning
        duration: 5m
        
      - name: disk_usage_critical
        metric: disk_usage
        operator: ">"
        threshold: 95
        level: critical
        duration: 1m
        
      - name: disk_temperature_high
        metric: disk_temperature
        operator: ">"
        threshold: 55
        level: warning
        duration: 10m
        
      - name: replication_lag
        metric: replication_lag_seconds
        operator: ">"
        threshold: 3600
        level: warning
        duration: 5m
        
  notifications:
    email:
      enabled: true
      smtp_server: smtp.example.com
      smtp_port: 587
      smtp_user: alerts@example.com
      from: nas-os@example.com
      to:
        - admin@example.com
        
    webhook:
      enabled: true
      url: https://hooks.slack.com/services/xxx
```

---

## 最佳实践

### 安全最佳实践

1. ✅ 启用 HTTPS（生产环境）
2. ✅ 修改默认密码
3. ✅ 配置防火墙规则
4. ✅ 启用审计日志
5. ✅ 定期更新系统
6. ✅ 配置失败锁定
7. ✅ 使用强密码策略
8. ✅ 定期审查访问日志
9. ✅ **启用磁盘加密（v2.5.0 新增）**
10. ✅ **配置密钥轮换（v2.5.0 新增）**

### 高可用最佳实践

1. ✅ 至少 3 节点集群（避免脑裂）
2. ✅ 配置奇数节点或仲裁服务器
3. ✅ 设置合理的故障检测间隔
4. ✅ 定期演练故障转移
5. ✅ 监控集群健康状态
6. ✅ 配置共享存储冗余
7. ✅ 定期备份集群配置

### 备份最佳实践

1. ✅ 3-2-1 备份策略
2. ✅ 本地快照（每日）
3. ✅ 远程复制（每6小时）
4. ✅ 云备份（每周）
5. ✅ 定期验证备份
6. ✅ 测试恢复流程
7. ✅ **启用备份加密（v2.5.0 新增）**
8. ✅ **使用增量备份节省空间（v2.5.0 新增）**

---

## 维护操作

### 日常维护清单

```bash
# 每日检查
sudo nasctl status              # 系统状态
sudo nasctl storage usage       # 存储空间
sudo nasctl disk health         # 磁盘健康
sudo nasctl ha status           # 集群状态 🆕
sudo nasctl replication status  # 复制状态 🆕

# 每周检查
sudo nasctl scrub start data    # 数据校验
sudo journalctl -u nas-os --since "1 week ago" | grep -i error

# 每月检查
sudo apt update && sudo apt upgrade -y
sudo nasd --version
sudo nasctl encryption status   # 加密状态 🆕
sudo nasctl backup verify --all # 验证备份 🆕
```

---

## 故障排查

### 快照复制故障

```bash
# 查看复制日志
nasctl replication logs data-backup --tail 100

# 检查网络连通性
nasctl replication test-connection node-2

# 重置复制状态
nasctl replication reset data-backup
```

### 加密设备问题

```bash
# 检查加密状态
cryptsetup status data_crypt

# 手动解锁
cryptsetup luksOpen /dev/sdb1 data_crypt

# 修复损坏的头部
cryptsetup luksHeaderBackup /dev/sdb1 --header-backup-file header.img
cryptsetup luksHeaderRestore /dev/sdb1 --header-backup-file header.img
```

### 高可用故障

```bash
# 查看集群日志
journalctl -u keepalived -n 100

# 检查脑裂状态
nasctl ha check-split-brain

# 强制切换到指定节点
nasctl ha force-failover --target node-2
```

---

*管理员手册版本：v2.5.0 | 最后更新：2026-03-14*