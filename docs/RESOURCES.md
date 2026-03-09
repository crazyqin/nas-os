# NAS-OS 系统资源需求

## 最低配置

| 组件 | 要求 |
|------|------|
| CPU | 双核 1.5GHz+ (ARM 或 x86_64) |
| 内存 | 1GB RAM |
| 存储 | 8GB 系统盘 + 数据盘 |
| 网络 | 千兆以太网 |

## 推荐配置

| 组件 | 要求 |
|------|------|
| CPU | 四核 2.0GHz+ |
| 内存 | 4GB RAM |
| 存储 | 32GB SSD 系统盘 + 多块数据盘 |
| 网络 | 2.5G 以太网或双千兆 |

## 资源消耗分析

### 空闲状态
```
CPU:    2-5%
内存：300-500MB
磁盘 I/O: <1MB/s
网络：  <10KB/s
```

### 典型负载（文件传输）
```
CPU:    20-40% (SMB/NFS 传输)
内存：800MB-1.5GB
磁盘 I/O: 100-500MB/s (取决于磁盘)
网络：  100-110MB/s (千兆饱和)
```

### 高负载（多用户 + 快照 + 校验）
```
CPU:    60-80%
内存：2-3GB
磁盘 I/O: 峰值 800MB/s+
网络：  多客户端并发
```

## 存储池规划

### 单盘模式
- 最小：120GB SSD
- 推荐：1TB+ HDD/SSD
- 文件系统：btrfs single

### RAID 模式
| RAID 级别 | 最小磁盘 | 容量利用率 | 适用场景 |
|-----------|----------|------------|----------|
| RAID1 | 2 块 | 50% | 系统盘、重要数据 |
| RAID5 | 3 块 | (N-1)/N | 通用存储 |
| RAID10 | 4 块 | 50% | 高性能需求 |
| RAID6 | 4 块 | (N-2)/N | 大容量、高可靠 |

## 内存使用分解

| 组件 | 内存占用 |
|------|----------|
| nasd 主进程 | 150-250MB |
| btrfs 缓存 | 动态 (建议 1GB+) |
| Samba | 50-100MB |
| NFS | 30-50MB |
| Web UI | 20-30MB |
| 系统开销 | 300-500MB |

## 磁盘空间预留

```
系统分区：8-16GB
日志分区：2-4GB (可选独立分区)
数据分区：剩余全部空间
交换空间：1-2GB (内存<4GB 时建议)
```

## 监控指标

### 关键阈值
| 指标 | 警告 | 严重 |
|------|------|------|
| CPU 使用率 | >70% | >90% |
| 内存使用率 | >80% | >95% |
| 磁盘使用率 | >80% | >95% |
| 磁盘温度 | >45°C | >55°C |
| 网络延迟 | >10ms | >50ms |

### 监控命令
```bash
# 系统资源
top -bn1 | head -20
free -h
df -h

# btrfs 状态
btrfs filesystem usage /mnt
btrfs device stats /mnt

# 磁盘健康
smartctl -a /dev/sdX

# 网络状态
iftop -P
ss -tulpn | grep -E '8080|445|2049'
```

## 性能优化建议

### 内核参数 (/etc/sysctl.conf)
```conf
# 网络优化
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216

# 文件系统
vm.dirty_ratio = 20
vm.dirty_background_ratio = 5
vm.vfs_cache_pressure = 50
```

### btrfs 挂载选项
```
defaults,noatime,compress=zstd,ssd
```

### Samba 优化 (/etc/samba/smb.conf)
```conf
[global]
socket options = TCP_NODELAY SO_RCVBUF=8192 SO_SNDBUF=8192
use sendfile = yes
aio read size = 16384
aio write size = 16384
```

## 扩展性

### 垂直扩展
- 增加内存：直接提升缓存能力
- 升级 CPU：提升加密/压缩性能
- 添加 SSD 缓存：btrfs 支持 SSD 缓存层

### 水平扩展
- 添加磁盘：在线扩容 btrfs 卷
- 多机集群：暂不支持（未来版本）

## 备份策略

| 数据类型 | 频率 | 保留 | 方式 |
|----------|------|------|------|
| 配置文件 | 每次变更 | 永久 | git + 远程 |
| 系统快照 | 每日 | 7 天 | btrfs snapshot |
| 用户数据 | 每日增量 | 30 天 | rsync + 远程 |
| 完整镜像 | 每周 | 4 周 | dd/clonezilla |
