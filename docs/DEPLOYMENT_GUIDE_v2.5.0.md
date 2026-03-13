# NAS-OS v2.5.0 部署指南

> **版本**: v2.5.0  
> **发布日期**: 2026-03-14  
> **目标读者**: 系统管理员、DevOps 工程师

---

## 目录

1. [部署概览](#部署概览)
2. [环境准备](#环境准备)
3. [安装方式](#安装方式)
4. [配置详解](#配置详解)
5. [生产环境部署](#生产环境部署)
6. [高可用部署](#高可用部署)
7. [加密部署](#加密部署)
8. [监控与日志](#监控与日志)
9. [备份与恢复](#备份与恢复)
10. [升级指南](#升级指南)
11. [故障排查](#故障排查)

---

## 部署概览

### 部署架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        客户端层                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                      │
│  │ Web 浏览器 │  │ 移动 App  │  │ API 客户端 │                      │
│  └──────────┘  └──────────┘  └──────────┘                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     负载均衡层 (可选)                             │
│                    (nginx / HAProxy)                            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      NAS-OS 应用层                               │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    NAS-OS Server v2.5.0                  │   │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐    │   │
│  │  │   Web   │  │ Storage │  │  Share  │  │  User   │    │   │
│  │  │  Server │  │ Manager │  │ Service │  │ Manager │    │   │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘    │   │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐                 │   │
│  │  │Snapshot │  │Encrypt. │  │   HA    │ 🆕             │   │
│  │  │Repl.    │  │Manager  │  │ Cluster │                 │   │
│  │  └─────────┘  └─────────┘  └─────────┘                 │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       存储层                                     │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐   │
│  │  btrfs    │  │   SMB     │  │   NFS     │  │   LUKS    │ 🆕│
│  │  Volume   │  │  (Samba)  │  │  Server   │  │  加密层    │    │
│  └───────────┘  └───────────┘  └───────────┘  └───────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       物理磁盘                                   │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐   │
│  │  /dev/sda │  │  /dev/sdb │  │  /dev/sdc │  │  /dev/sdd │   │
│  │   (系统)   │  │  (数据)   │  │  (数据)   │  │  (备份)   │   │
│  └───────────┘  └───────────┘  └───────────┘  └───────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### 部署模式

| 模式 | 适用场景 | 复杂度 | 成本 |
|------|----------|--------|------|
| 单机部署 | 家庭/小型办公室 | 低 | 低 |
| Docker 部署 | 开发测试/容器环境 | 中 | 中 |
| 高可用部署 | 企业生产环境 | 高 | 高 |
| 加密部署 | 安全敏感环境 | 中 | 中 |

### 系统要求

#### 硬件要求

| 组件 | 最低配置 | 推荐配置 | 生产环境 |
|------|---------|---------|---------|
| CPU | 双核 1.5GHz | 四核 2.0GHz+ | 八核 2.5GHz+ |
| 内存 | 2GB | 4GB+ | 16GB+ |
| 系统盘 | 16GB | 64GB SSD | 256GB NVMe SSD |
| 数据盘 | 任意 | 根据需求 | RAID 配置 |
| 网络 | 百兆以太网 | 千兆以太网 | 双千兆/万兆 |

#### 软件要求

| 项目 | 版本 | 说明 |
|------|------|------|
| 操作系统 | Ubuntu 22.04+ / Debian 11+ / Rocky Linux 8+ | Linux |
| 内核 | 5.10+ | 支持 btrfs 和 LUKS 特性 |
| btrfs-progs | 5.10+ | btrfs 工具集 |
| cryptsetup | 2.3+ | LUKS 加密工具 🆕 |
| Go | 1.21+ | 仅编译时需要 |

---

## 环境准备

### 操作系统安装

```bash
# 1. 更新系统
sudo apt update && sudo apt upgrade -y

# 2. 安装必要工具
sudo apt install -y \
    curl \
    wget \
    git \
    vim \
    htop \
    iotop \
    net-tools \
    btrfs-progs \
    cryptsetup \
    samba \
    nfs-kernel-server

# 3. 配置静态 IP
sudo tee /etc/netplan/01-netcfg.yaml <<EOF
network:
  version: 2
  ethernets:
    eth0:
      addresses:
        - 192.168.1.100/24
      routes:
        - to: default
          via: 192.168.1.1
      nameservers:
        addresses:
          - 8.8.8.8
          - 8.8.4.4
EOF

sudo netplan apply

# 4. 配置时区
sudo timedatectl set-timezone Asia/Shanghai
```

---

## 安装方式

### 方式一：一键安装脚本（推荐）

```bash
# 下载并执行安装脚本
curl -fsSL https://raw.githubusercontent.com/nas-os/nasd/main/scripts/install.sh | sudo bash
```

### 方式二：Docker 部署

```yaml
# docker-compose.yml
version: '3.8'

services:
  nasd:
    image: nas-os/nasd:v2.5.0
    container_name: nasd
    restart: unless-stopped
    ports:
      - "8080:8080"
      - "445:445"
      - "2049:2049"
    volumes:
      - /data:/data
      - /etc/nas-os:/config
      - /var/log/nas-os:/var/log/nas-os
    environment:
      - NASD_CONFIG=/config/config.yaml
    cap_add:
      - SYS_ADMIN
      - DAC_READ_SEARCH
    devices:
      - /dev/sdb:/dev/sdb
      - /dev/mapper/data_crypt:/dev/mapper/data_crypt
```

### 方式三：手动安装

```bash
# 1. 下载二进制
sudo curl -L https://github.com/nas-os/nasd/releases/download/v2.5.0/nasd-linux-amd64 \
  -o /usr/local/bin/nasd
sudo chmod +x /usr/local/bin/nasd

# 2. 创建配置目录
sudo mkdir -p /etc/nas-os /var/lib/nas-os /var/log/nas-os

# 3. 创建系统服务
sudo tee /etc/systemd/system/nas-os.service <<EOF
[Unit]
Description=NAS-OS Storage Management Service
After=network.target local-fs.target

[Service]
Type=simple
ExecStart=/usr/local/bin/nasd --config /etc/nas-os/config.yaml
Restart=on-failure
RestartSec=5
User=root
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

# 4. 启动服务
sudo systemctl daemon-reload
sudo systemctl enable nas-os
sudo systemctl start nas-os
```

---

## 配置详解

### 主配置文件

```yaml
# /etc/nas-os/config.yaml

# ==================== 服务器配置 ====================
server:
  port: 8080
  host: 0.0.0.0
  tls_enabled: false
  tls_cert: /etc/nas-os/cert.pem
  tls_key: /etc/nas-os/key.pem

# ==================== 存储配置 ====================
storage:
  mount_base: /mnt
  auto_scrub: true
  compression: zstd

# ==================== 加密配置 🆕 ====================
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
    schedule: "0 0 1 * *"

# ==================== 快照复制配置 🆕 ====================
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
    - name: data-replication
      source_volume: data
      target_node: node-2
      target_path: /backup/data
      mode: incremental
      schedule: "0 */6 * * *"
      bandwidth_limit: 10485760
      encryption: true

# ==================== 高可用配置 🆕 ====================
cluster:
  enabled: true
  cluster_name: nas-cluster
  this_node:
    id: node-1
    hostname: nas-primary
    priority: 100
  nodes:
    - id: node-1
      address: 192.168.1.100
      priority: 100
    - id: node-2
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

# ==================== 监控配置 ====================
monitoring:
  metrics_enabled: true
  metrics_port: 9090
  alerts:
    disk_usage_warning: 80
    disk_usage_critical: 95
```

---

## 加密部署 🆕

### 启用磁盘加密

```bash
# 1. 创建加密设备
sudo cryptsetup luksFormat /dev/sdb1 \
  --cipher aes-xts-plain64 \
  --key-size 512 \
  --hash sha256 \
  --iter-time 5000

# 2. 解锁加密设备
sudo cryptsetup luksOpen /dev/sdb1 data_crypt

# 3. 创建文件系统
sudo mkfs.btrfs /dev/mapper/data_crypt

# 4. 挂载
sudo mkdir -p /mnt/data
sudo mount /dev/mapper/data_crypt /mnt/data

# 5. 配置自动解锁
sudo tee /etc/crypttab <<EOF
data_crypt /dev/sdb1 /etc/nas-os/encryption/keyfile luks
EOF

# 6. 配置自动挂载
sudo tee -a /etc/fstab <<EOF
/dev/mapper/data_crypt /mnt/data btrfs defaults,noatime 0 0
EOF
```

### 密钥管理

```bash
# 生成密钥文件
sudo dd if=/dev/urandom of=/etc/nas-os/encryption/keyfile bs=512 count=8
sudo chmod 600 /etc/nas-os/encryption/keyfile

# 添加密钥文件到 LUKS
sudo cryptsetup luksAddKey /dev/sdb1 /etc/nas-os/encryption/keyfile

# 备份 LUKS 头部
sudo cryptsetup luksHeaderBackup /dev/sdb1 \
  --header-backup-file /backup/luks-header-$(date +%Y%m%d).img
```

---

## 高可用部署 🆕

### 双节点集群

```bash
# Node 1 配置
sudo apt install -y keepalived

sudo tee /etc/keepalived/keepalived.conf <<EOF
vrrp_script check_nasd {
    script "/usr/local/bin/check_nasd.sh"
    interval 2
    weight -20
}

vrrp_instance VI_1 {
    state MASTER
    interface eth0
    virtual_router_id 51
    priority 100
    advert_int 1
    
    authentication {
        auth_type PASS
        auth_pass secret123
    }
    
    virtual_ipaddress {
        192.168.1.99
    }
    
    track_script {
        check_nasd
    }
}
EOF

# 健康检查脚本
sudo tee /usr/local/bin/check_nasd.sh <<'EOF'
#!/bin/bash
curl -sf http://localhost:8080/api/v1/health > /dev/null
exit $?
EOF
sudo chmod +x /usr/local/bin/check_nasd.sh

# Node 2 配置（priority 改为 90，state 改为 BACKUP）
```

### 启动高可用

```bash
# 启动服务
sudo systemctl enable keepalived
sudo systemctl start keepalived

# 验证 VIP
ip addr show eth0 | grep 192.168.1.99

# 测试故障转移
# 在 Node 1 上停止 nasd
sudo systemctl stop nas-os
# 观察 VIP 是否迁移到 Node 2
```

---

## 升级指南

### 从 v2.4.0 升级到 v2.5.0

```bash
# 1. 备份配置
sudo nasctl config export > /backup/config-v2.4.0.yaml

# 2. 停止服务
sudo systemctl stop nas-os

# 3. 备份数据库（如有）
sudo cp /var/lib/nas-os/nas.db /backup/nas-v2.4.0.db

# 4. 下载新版本
sudo curl -L https://github.com/nas-os/nasd/releases/download/v2.5.0/nasd-linux-amd64 \
  -o /usr/local/bin/nasd
sudo chmod +x /usr/local/bin/nasd

# 5. 更新配置文件（添加新功能配置）
sudo cp /etc/nas-os/config.yaml /etc/nas-os/config.yaml.bak
# 手动编辑添加 v2.5.0 新配置项

# 6. 启动服务
sudo systemctl start nas-os

# 7. 验证
nasd --version
curl http://localhost:8080/api/v1/health

# 8. 启用新功能
# 启用加密
nasctl encryption enable /dev/sdb1

# 配置快照复制
nasctl replication create ...

# 配置高可用
nasctl ha configure ...
```

### 数据迁移

```bash
# 从旧版本迁移数据
# 1. 在旧系统创建快照
sudo btrfs subvolume snapshot -r /mnt/data /mnt/data/snapshot-migration

# 2. 同步到新系统
rsync -avz /mnt/data/snapshot-migration/ user@new-server:/mnt/data/

# 3. 验证数据
diff -r /mnt/data user@new-server:/mnt/data
```

---

## 故障排查

### 常见问题

#### 加密设备无法解锁

```bash
# 检查 LUKS 头部
sudo cryptsetup luksDump /dev/sdb1

# 恢复头部备份
sudo cryptsetup luksHeaderRestore /dev/sdb1 \
  --header-backup-file /backup/luks-header.img

# 使用恢复密钥
sudo cryptsetup luksOpen /dev/sdb1 data_crypt \
  --key-file /backup/recovery-key
```

#### 复制任务失败

```bash
# 查看日志
nasctl replication logs task-name --tail 100

# 检查网络
nasctl replication test-connection node-2

# 重置任务
nasctl replication reset task-name
```

#### 高可用脑裂

```bash
# 检查集群状态
nasctl ha status

# 强制指定主节点
nasctl ha set-master node-1

# 重新同步数据
nasctl ha resync
```

---

*部署指南版本：v2.5.0 | 最后更新：2026-03-14*