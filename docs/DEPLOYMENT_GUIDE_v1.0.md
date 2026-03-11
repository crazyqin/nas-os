# NAS-OS v1.0 部署指南

> **版本**: v1.0.0  
> **发布日期**: 2026-03-11  
> **目标读者**: 系统管理员、DevOps 工程师

---

## 目录

1. [部署概览](#部署概览)
2. [环境准备](#环境准备)
3. [安装方式](#安装方式)
4. [配置详解](#配置详解)
5. [生产环境部署](#生产环境部署)
6. [高可用部署](#高可用部署)
7. [监控与日志](#监控与日志)
8. [备份与恢复](#备份与恢复)
9. [升级指南](#升级指南)
10. [故障排查](#故障排查)

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
│  │                    NAS-OS Server                         │   │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐    │   │
│  │  │   Web   │  │ Storage │  │  Share  │  │  User   │    │   │
│  │  │  Server │  │ Manager │  │ Service │  │ Manager │    │   │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘    │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       存储层                                     │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐   │
│  │  btrfs    │  │   SMB     │  │   NFS     │  │  Docker   │   │
│  │  Volume   │  │  (Samba)  │  │  Server   │  │  Engine   │   │
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
| 内核 | 5.10+ | 支持 btrfs 特性 |
| btrfs-progs | 5.10+ | btrfs 工具集 |
| Go | 1.21+ | 仅编译时需要 |

---

## 环境准备

### 操作系统安装

#### Ubuntu Server 22.04 LTS

```bash
# 1. 下载镜像
wget https://releases.ubuntu.com/22.04/ubuntu-22.04.3-live-server-amd64.iso

# 2. 创建启动盘
# 使用 Rufus (Windows) 或 Etcher (跨平台)

# 3. 安装系统
# - 选择最小化安装
# - 配置静态 IP
# - 启用 SSH 服务
```

#### 系统初始化

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
    net-tools

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

# 4. 配置主机名
sudo hostnamectl set-hostname nas-os
sudo tee -a /etc/hosts <<EOF
127.0.1.1 nas-os
EOF

# 5. 配置时区
sudo timedatectl set-timezone Asia/Shanghai
```

### 依赖安装

```bash
# btrfs 支持
sudo apt install -y btrfs-progs

# SMB 共享
sudo apt install -y samba samba-vfs-modules

# NFS 共享
sudo apt install -y nfs-kernel-server

# Docker（可选）
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# 监控工具（可选）
sudo apt install -y prometheus-node-exporter
```

### 磁盘准备

```bash
# 1. 查看磁盘
lsblk
sudo fdisk -l

# 2. 格式化数据盘（如需要）
# 注意：这会删除所有数据！
sudo mkfs.btrfs -f -L data /dev/sdb1

# 3. 创建 RAID 配置（多盘）
# RAID1
sudo mkfs.btrfs -f -L data -d raid1 /dev/sdb1 /dev/sdc1

# RAID5
sudo mkfs.btrfs -f -L data -d raid5 /dev/sdb1 /dev/sdc1 /dev/sdd1

# 4. 查看文件系统
sudo btrfs filesystem show
```

---

## 安装方式

### 方式一：一键安装脚本（推荐）

适合大多数用户，自动完成所有依赖安装和配置。

```bash
# 下载并执行安装脚本
curl -fsSL https://raw.githubusercontent.com/nas-os/nasd/main/scripts/install.sh | sudo bash

# 或先下载再执行（更安全）
wget https://raw.githubusercontent.com/nas-os/nasd/main/scripts/install.sh
chmod +x install.sh
sudo ./install.sh
```

**安装脚本会执行**：
1. ✅ 检查系统依赖
2. ✅ 安装 btrfs、samba、nfs 等工具
3. ✅ 下载最新版本的 nasd 二进制文件
4. ✅ 创建系统服务
5. ✅ 生成默认配置文件
6. ✅ 配置防火墙规则
7. ✅ 启动服务

**验证安装**：
```bash
# 检查服务状态
sudo systemctl status nas-os

# 检查版本
nasd --version

# 访问 Web UI
# http://<服务器 IP>:8080
```

### 方式二：Docker 部署

适合开发测试或容器化环境。

#### Docker Compose 部署

```yaml
# docker-compose.yml
version: '3.8'

services:
  nasd:
    image: nas-os/nasd:v1.0.0
    container_name: nasd
    restart: unless-stopped
    ports:
      - "8080:8080"    # Web UI
      - "445:445"      # SMB
      - "2049:2049"    # NFS
      - "111:111"      # RPC
    volumes:
      - /data:/data
      - /etc/nas-os:/config
      - /var/log/nas-os:/var/log/nas-os
      - /var/run/samba:/var/run/samba
      - /var/lib/nfs:/var/lib/nfs
    environment:
      - NASD_CONFIG=/config/config.yaml
    cap_add:
      - SYS_ADMIN
      - DAC_READ_SEARCH
    devices:
      - /dev/sdb:/dev/sdb
      - /dev/sdc:/dev/sdc
    networks:
      - nas-network

  # 可选：Prometheus 监控
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    restart: unless-stopped
    ports:
      - "9090:9090"
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    networks:
      - nas-network

  # 可选：Grafana 可视化
  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    restart: unless-stopped
    ports:
      - "3000:3000"
    volumes:
      - grafana_data:/var/lib/grafana
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin123
    networks:
      - nas-network
    depends_on:
      - prometheus

volumes:
  prometheus_data:
  grafana_data:

networks:
  nas-network:
    driver: bridge
```

**部署命令**：
```bash
# 1. 创建目录
mkdir -p /data /etc/nas-os /var/log/nas-os

# 2. 启动服务
docker-compose up -d

# 3. 查看日志
docker-compose logs -f nasd

# 4. 访问 Web UI
# http://localhost:8080
```

### 方式三：手动编译安装

适合开发者或需要自定义构建的用户。

#### 1. 安装 Go 环境

```bash
# 下载 Go
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz

# 解压
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz

# 添加到 PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 验证
go version
```

#### 2. 克隆并编译

```bash
# 克隆项目
git clone https://github.com/nas-os/nasd.git
cd nasd

# 安装依赖
go mod tidy

# 编译
make build

# 或手动编译
go build -o nasd ./cmd/nasd
go build -o nasctl ./cmd/nasctl
```

#### 3. 安装到系统

```bash
# 复制二进制文件
sudo cp nasd nasctl /usr/local/bin/
sudo chmod +x /usr/local/bin/nasd /usr/local/bin/nasctl

# 创建配置目录
sudo mkdir -p /etc/nas-os
sudo mkdir -p /var/lib/nas-os
sudo mkdir -p /var/log/nas-os

# 复制默认配置
sudo cp configs/default.yaml /etc/nas-os/config.yaml

# 创建系统服务
sudo tee /etc/systemd/system/nas-os.service <<EOF
[Unit]
Description=NAS-OS Storage Management Service
After=network.target local-fs.target btrfs.target

[Service]
Type=simple
ExecStart=/usr/local/bin/nasd --config /etc/nas-os/config.yaml
Restart=on-failure
RestartSec=5
User=root
LimitNOFILE=65536

# 安全设置
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/data /var/lib/nas-os /var/log/nas-os
NoNewPrivileges=false

# 日志
StandardOutput=journal
StandardError=journal
SyslogIdentifier=nasd

[Install]
WantedBy=multi-user.target
EOF

# 启用服务
sudo systemctl daemon-reload
sudo systemctl enable nas-os
sudo systemctl start nas-os
```

---

## 配置详解

### 配置文件结构

```yaml
# /etc/nas-os/config.yaml

# ==================== 服务器配置 ====================
server:
  port: 8080
  host: 0.0.0.0
  tls_enabled: false
  tls_cert: /etc/nas-os/cert.pem
  tls_key: /etc/nas-os/key.pem
  session_timeout: 3600
  max_connections: 100

# ==================== 存储配置 ====================
storage:
  mount_base: /mnt
  auto_scrub: true
  scrub_schedule: "0 3 * * 0"
  compression: zstd
  
# ==================== SMB 配置 ====================
smb:
  enabled: true
  workgroup: WORKGROUP
  min_protocol: SMB2
  encrypt_transport: true

# ==================== NFS 配置 ====================
nfs:
  enabled: true
  versions: ["3", "4"]
  allowed_networks:
    - 192.168.1.0/24

# ==================== 用户配置 ====================
users:
  password_min_length: 8
  default_role: viewer
  allow_guest: true

# ==================== 监控配置 ====================
monitoring:
  metrics_enabled: true
  metrics_port: 9090
  alerts:
    disk_usage_warning: 80
    disk_usage_critical: 95

# ==================== 日志配置 ====================
logging:
  level: info
  format: json
  file: /var/log/nas-os/nasd.log
  max_size: 100
  max_backups: 5
```

### 环境变量

```bash
# 敏感信息通过环境变量传递
export NASD_SERVER_PORT=8080
export NASD_SMTP_PASSWORD=secret123
export NASD_DATABASE_URL=postgres://user:pass@localhost/nasd

# 启动时加载
sudo -E nasd
```

### 配置验证

```bash
# 测试配置文件
sudo nasd --config /etc/nas-os/config.yaml --test

# 输出示例：
# ✓ 配置文件语法正确
# ✓ 所有路径可访问
# ✓ 端口 8080 可用
# ✓ SMB 配置有效
# ✓ NFS 配置有效
# 配置验证通过
```

---

## 生产环境部署

### 安全加固

#### 1. 启用 HTTPS

```bash
# 使用 Let's Encrypt 获取证书
sudo apt install certbot

# 停止 NAS-OS 临时释放端口
sudo systemctl stop nas-os

# 获取证书
sudo certbot certonly --standalone -d nas.example.com

# 更新配置
sudo tee -a /etc/nas-os/config.yaml <<EOF
server:
  tls_enabled: true
  tls_cert: /etc/letsencrypt/live/nas.example.com/fullchain.pem
  tls_key: /etc/letsencrypt/live/nas.example.com/privkey.pem
EOF

# 配置证书自动更新
sudo tee /etc/cron.d/certbot <<EOF
0 */12 * * * root certbot renew --quiet --deploy-hook "systemctl restart nas-os"
EOF

# 重启服务
sudo systemctl start nas-os
```

#### 2. 配置防火墙

```bash
# UFW 配置
sudo ufw default deny incoming
sudo ufw default allow outgoing

# 必需端口
sudo ufw allow 22/tcp         # SSH
sudo ufw allow 8080/tcp       # Web UI
sudo ufw allow 443/tcp        # HTTPS
sudo ufw allow 445/tcp        # SMB
sudo ufw allow 2049/tcp       # NFS
sudo ufw allow 111/tcp        # RPC
sudo ufw allow 111/udp        # RPC

# 限制管理接口（推荐）
sudo ufw allow from 192.168.1.0/24 to any port 8080
sudo ufw allow from 192.168.1.0/24 to any port 22

sudo ufw enable
```

#### 3. 配置 Fail2Ban

```bash
# 安装 Fail2Ban
sudo apt install -y fail2ban

# 创建 NAS-OS 过滤器
sudo tee /etc/fail2ban/filter.d/nas-os.conf <<EOF
[Definition]
failregex = ^.*Failed login attempt from <HOST>.*$
            ^.*Invalid credentials from <HOST>.*$
ignoreregex =
EOF

# 创建 jail 配置
sudo tee /etc/fail2ban/jail.d/nas-os.conf <<EOF
[nas-os]
enabled = true
port = 8080
filter = nas-os
logpath = /var/log/nas-os/nasd.log
maxretry = 5
bantime = 3600
findtime = 600
EOF

# 重启 Fail2Ban
sudo systemctl restart fail2ban
```

#### 4. 安全审计

```bash
# 启用审计日志
sudo tee -a /etc/nas-os/config.yaml <<EOF
logging:
  audit_enabled: true
  audit_file: /var/log/nas-os/audit.log
  audit_events:
    - login
    - logout
    - user_create
    - user_delete
    - volume_create
    - volume_delete
    - share_create
    - share_delete
    - config_change
EOF

# 配置日志轮转
sudo tee /etc/logrotate.d/nas-os <<EOF
/var/log/nas-os/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 0640 root root
    postrotate
        systemctl kill -s HUP nas-os
    endscript
}
EOF
```

### 性能优化

#### 1. 内核参数优化

```bash
# /etc/sysctl.d/99-nas-os.conf
# 网络优化
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216
net.ipv4.tcp_congestion_control = bbr

# 文件描述符
fs.file-max = 2097152
fs.inotify.max_user_watches = 524288

# 虚拟内存
vm.swappiness = 10
vm.dirty_ratio = 40
vm.dirty_background_ratio = 10

# 应用配置
sudo sysctl -p /etc/sysctl.d/99-nas-os.conf
```

#### 2. 资源限制

```bash
# /etc/security/limits.d/nas-os.conf
root soft nofile 65536
root hard nofile 65536
root soft nproc 4096
root hard nproc 4096
```

#### 3. btrfs 优化

```bash
# 优化挂载选项（/etc/fstab）
/dev/sdb1 /mnt/data btrfs defaults,noatime,compress=zstd,commit=30 0 0

# 应用挂载
sudo mount -o remount /mnt/data
```

### 监控配置

#### 1. Prometheus 配置

```yaml
# /etc/prometheus/prometheus.yml
global:
  scrape_interval: 30s
  evaluation_interval: 30s

scrape_configs:
  - job_name: 'nas-os'
    static_configs:
      - targets: ['localhost:9090']
    metrics_path: /metrics
    
  - job_name: 'node'
    static_configs:
      - targets: ['localhost:9100']
```

#### 2. Grafana 仪表板

导入预配置的仪表板 ID：`18666`（NAS-OS Dashboard）

```bash
# 或使用 API 导入
curl -X POST http://localhost:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <grafana_token>" \
  -d @grafana-dashboard.json
```

#### 3. 告警规则

```yaml
# /etc/prometheus/alerts.yml
groups:
  - name: nas-os
    rules:
      - alert: DiskUsageWarning
        expr: nasd_volume_usage_percent > 80
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "磁盘使用率超过 80%"
          
      - alert: DiskUsageCritical
        expr: nasd_volume_usage_percent > 95
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "磁盘使用率超过 95%"
          
      - alert: ServiceDown
        expr: up{job="nas-os"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "NAS-OS 服务宕机"
```

---

## 高可用部署

### 双机热备架构

```
┌─────────────────┐         ┌─────────────────┐
│   Primary NAS   │◄───────►│  Secondary NAS  │
│   192.168.1.100 │  心跳   │  192.168.1.101  │
└────────┬────────┘         └────────┬────────┘
         │                           │
         └───────────┬───────────────┘
                     │
              ┌──────▼──────┐
              │  共享存储    │
              │  (iSCSI)    │
              └─────────────┘
                     │
              ┌──────▼──────┐
              │  虚拟 IP     │
              │ 192.168.1.99│
              └─────────────┘
```

### Keepalived 配置

```bash
# 安装 Keepalived
sudo apt install -y keepalived

# 主节点配置
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

# 备节点配置（priority 改为 90，state 改为 BACKUP）

# 健康检查脚本
sudo tee /usr/local/bin/check_nasd.sh <<EOF
#!/bin/bash
curl -sf http://localhost:8080/api/v1/health > /dev/null
exit $?
EOF

sudo chmod +x /usr/local/bin/check_nasd.sh

# 启动 Keepalived
sudo systemctl enable keepalived
sudo systemctl start keepalived
```

### 数据同步

```bash
# 使用 rsync 同步数据
sudo tee /usr/local/bin/sync_data.sh <<EOF
#!/bin/bash
rsync -avz --delete /mnt/data/ 192.168.1.101:/mnt/data/
EOF

sudo chmod +x /usr/local/bin/sync_data.sh

# 配置定时同步
sudo crontab -e
# */5 * * * * /usr/local/bin/sync_data.sh
```

---

## 监控与日志

### 系统监控

```bash
# 安装 Node Exporter
sudo apt install -y prometheus-node-exporter

# 验证
curl http://localhost:9100/metrics
```

### 日志聚合

#### ELK Stack 配置

```yaml
# docker-compose.yml
version: '3.8'

services:
  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.11.0
    environment:
      - discovery.type=single-node
      - xpack.security.enabled=false
    volumes:
      - elasticsearch_data:/usr/share/elasticsearch/data
    ports:
      - "9200:9200"

  logstash:
    image: docker.elastic.co/logstash/logstash:8.11.0
    volumes:
      - ./logstash/pipeline:/usr/share/logstash/pipeline
      - /var/log/nas-os:/var/log/nas-os
    depends_on:
      - elasticsearch

  kibana:
    image: docker.elastic.co/kibana/kibana:8.11.0
    ports:
      - "5601:5601"
    depends_on:
      - elasticsearch

volumes:
  elasticsearch_data:
```

### 告警通知

#### Slack 通知

```yaml
# config.yaml
monitoring:
  notifications:
    slack:
      enabled: true
      webhook_url: https://hooks.slack.com/services/xxx
      channel: "#nas-alerts"
      username: "NAS-OS Bot"
```

#### 邮件通知

```yaml
# config.yaml
monitoring:
  notifications:
    email:
      enabled: true
      smtp_server: smtp.example.com
      smtp_port: 587
      smtp_user: alerts@example.com
      smtp_password: "${SMTP_PASSWORD}"
      from: nas-os@example.com
      to:
        - admin@example.com
```

---

## 备份与恢复

### 配置备份

```bash
# 创建备份脚本
sudo tee /usr/local/bin/backup_nas_config.sh <<EOF
#!/bin/bash
BACKUP_DIR="/backup/nas-os"
DATE=$(date +%Y%m%d_%H%M%S)

mkdir -p $BACKUP_DIR

# 备份配置
tar -czf $BACKUP_DIR/config-$DATE.tar.gz \
  /etc/nas-os/ \
  /var/lib/nas-os/

# 保留最近 30 天的备份
find $BACKUP_DIR -name "config-*.tar.gz" -mtime +30 -delete

echo "[$(date)] 配置备份完成" >> /var/log/nas-os/backup.log
EOF

sudo chmod +x /usr/local/bin/backup_nas_config.sh

# 配置定时任务
sudo crontab -e
# 0 2 * * * /usr/local/bin/backup_nas_config.sh
```

### 数据备份

#### 本地快照

```yaml
# config.yaml
backup:
  snapshot:
    enabled: true
    schedules:
      - name: daily
        cron: "0 2 * * *"
        retention: 7
      - name: weekly
        cron: "0 3 * * 0"
        retention: 4
```

#### 远程备份

```bash
# 使用 rclone 备份到云存储
rclone config  # 配置远程存储

# 创建备份脚本
sudo tee /usr/local/bin/backup_to_cloud.sh <<EOF
#!/bin/bash
REMOTE="remote:bucket/nas-backup"
DATE=$(date +%Y%m%d)

rclone sync /mnt/data $REMOTE/$DATE \
  --progress \
  --transfers=4 \
  --checkers=8
EOF

sudo chmod +x /usr/local/bin/backup_to_cloud.sh
```

### 灾难恢复

```bash
# 1. 安装新系统
# 2. 安装 NAS-OS
curl -fsSL https://raw.githubusercontent.com/nas-os/nasd/main/scripts/install.sh | sudo bash

# 3. 恢复配置
tar -xzf /backup/config-YYYYMMDD_HHMMSS.tar.gz -C /

# 4. 恢复数据（从远程备份）
rclone sync remote:bucket/nas-backup/latest /mnt/data

# 5. 启动服务
sudo systemctl start nas-os

# 6. 验证
curl http://localhost:8080/api/v1/health
```

---

## 升级指南

### 版本检查

```bash
# 检查当前版本
nasd --version

# 检查最新版本
curl -s https://api.github.com/repos/nas-os/nasd/releases/latest | grep tag_name
```

### Docker 升级

```bash
cd /opt/nas-os

# 拉取新镜像
docker-compose pull

# 重启服务
docker-compose up -d

# 清理旧镜像
docker images | grep nas-os | grep none | awk '{print $3}' | xargs docker rmi
```

### 裸机升级

```bash
# 1. 备份配置
sudo nasctl config export > /backup/config-backup-$(date +%Y%m%d).yaml

# 2. 停止服务
sudo systemctl stop nas-os

# 3. 下载新版本
sudo curl -L https://github.com/nas-os/nasd/releases/download/v1.0.1/nasd-linux-amd64 \
  -o /usr/local/bin/nasd
sudo chmod +x /usr/local/bin/nasd

# 4. 启动服务
sudo systemctl start nas-os

# 5. 验证
nasd --version
curl http://localhost:8080/api/v1/health
```

### 回滚

```bash
# 1. 停止服务
sudo systemctl stop nas-os

# 2. 恢复旧版本
sudo cp /usr/local/bin/nasd.bak /usr/local/bin/nasd

# 3. 恢复配置
sudo nasctl config import /backup/config-backup-YYYYMMDD.yaml

# 4. 启动服务
sudo systemctl start nas-os
```

---

## 故障排查

### 诊断工具

```bash
# 系统诊断
sudo nasctl diagnose

# 检查服务状态
sudo systemctl status nas-os

# 查看日志
journalctl -u nas-os -n 100 --no-pager

# 检查端口
sudo ss -tulpn | grep nasd

# 检查磁盘
sudo btrfs filesystem show
```

### 常见问题

#### 问题 1：服务无法启动

```bash
# 诊断
sudo systemctl status nas-os
journalctl -u nas-os -n 100

# 常见原因：
# 1. 端口被占用：sudo ss -tulpn | grep 8080
# 2. 配置错误：sudo nasd --config /etc/nas-os/config.yaml --test
# 3. 权限不足：确保以 root 运行
```

#### 问题 2：性能下降

```bash
# 诊断
top
iostat -x 1
sudo btrfs filesystem usage /mnt/data

# 优化：
# 1. 启用压缩
# 2. 运行 balance
# 3. 检查磁盘健康
```

#### 问题 3：共享访问失败

```bash
# SMB 诊断
smbclient -L //<服务器 IP> -U%
sudo systemctl status smbd

# NFS 诊断
showmount -e <服务器 IP>
sudo systemctl status nfs-server
```

### 获取帮助

- 📖 **文档**: https://docs.nas-os.dev
- 🐛 **Issues**: https://github.com/nas-os/nasd/issues
- 💬 **讨论**: https://github.com/nas-os/nasd/discussions
- 📧 **支持**: support@nas-os.dev

---

*部署指南版本：v1.0.0 | 最后更新：2026-03-11*
