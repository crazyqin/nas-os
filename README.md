# NAS-OS 🖥️

基于 Go 的家用 NAS 系统，支持 btrfs 存储管理、SMB/NFS 共享、Web 管理界面。

> **最新版本**: v1.1.0 Stable (2026-03-12)

## 特性

- 💾 **btrfs 存储管理** - 卷、子卷、快照、RAID ✅
- 🌐 **Web 管理界面** - 简洁易用的可视化操作 ✅
- 📁 **文件共享** - SMB/CIFS、NFS ✅
- 👥 **用户权限** - 多用户、访问控制 ✅
- 📊 **监控告警** - 磁盘健康、空间预警 ✅
- 🔒 **安全认证** - JWT、RBAC、审计日志 ✅
- 🐳 **Docker 部署** - 多架构镜像支持 ✅

## 快速开始

### 方式一：下载二进制文件 (推荐)

```bash
# 下载 (根据你的架构选择)
# AMD64 (x86_64)
wget https://github.com/crazyqin/nas-os/releases/download/v1.1.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# ARM64 (Orange Pi 5, Raspberry Pi 4/5)
wget https://github.com/crazyqin/nas-os/releases/download/v1.1.0/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd

# ARMv7 (Raspberry Pi 3, 旧款 ARM)
wget https://github.com/crazyqin/nas-os/releases/download/v1.1.0/nasd-linux-armv7
chmod +x nasd-linux-armv7
sudo mv nasd-linux-armv7 /usr/local/bin/nasd

# 验证安装
nasd --version
```

### 方式二：Docker 部署

```bash
# 拉取镜像
docker pull ghcr.io/crazyqin/nas-os:v1.1.0

# 运行容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v1.1.0

# 查看日志
docker logs -f nasd
```

### 方式三：源码编译

#### 依赖

```bash
# 安装 Go 1.26.1+
# 安装 btrfs 工具
sudo apt install btrfs-progs

# 安装 Samba（如需 SMB 共享）
sudo apt install samba

# 安装 NFS（如需 NFS 共享）
sudo apt install nfs-kernel-server
```

#### 构建

```bash
cd nas-os
go mod tidy
go build -o nasd ./cmd/nasd
go build -o nasctl ./cmd/nasctl
```

### 运行

```bash
# 需要 root 权限（访问磁盘设备）
sudo nasd
```

访问 http://localhost:8080

**默认登录凭据**：
- 用户名：`admin`
- 密码：`admin123`

⚠️ **首次登录后请立即修改默认密码！**

## API 接口

### 存储管理
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/volumes | 获取卷列表 |
| POST | /api/v1/volumes | 创建卷 |
| GET | /api/v1/volumes/:name | 获取卷详情 |
| DELETE | /api/v1/volumes/:name | 删除卷 |
| POST | /api/v1/volumes/:name/subvolumes | 创建子卷 |
| POST | /api/v1/volumes/:name/snapshots | 创建快照 |
| POST | /api/v1/volumes/:name/balance | 平衡数据 |
| POST | /api/v1/volumes/:name/scrub | 数据校验 |

### 共享管理
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/shares | 获取共享列表 |
| POST | /api/v1/shares/smb | 创建 SMB 共享 |
| POST | /api/v1/shares/nfs | 创建 NFS 共享 |
| DELETE | /api/v1/shares/:id | 删除共享 |
| PUT | /api/v1/shares/:id | 更新共享配置 |

### 配置管理
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/config | 获取配置 |
| PUT | /api/v1/config | 更新配置 |
| POST | /api/v1/config/reload | 重载配置 |

完整 API 文档请查看 [docs/API.md](docs/API.md)

## 项目结构

```
nas-os/
├── cmd/           # 可执行程序入口
├── internal/      # 内部模块
│   ├── storage/   # 存储管理
│   ├── web/       # Web 服务
│   ├── smb/       # SMB 共享
│   ├── nfs/       # NFS 共享
│   └── users/     # 用户管理
├── pkg/           # 公共库
├── webui/         # 前端界面
└── configs/       # 配置文件
```

## 开发计划

详细里程碑请查看 [MILESTONES.md](MILESTONES.md)

### 当前状态 (2026-03-12) - v1.1.0 Stable ✅
- [x] 项目骨架
- [x] btrfs 完整功能 (卷/子卷/快照/balance/scrub)
- [x] Web 框架 + Web UI
- [x] SMB/NFS 共享实现
- [x] 配置持久化
- [x] 用户认证系统 (JWT + RBAC)
- [x] 系统监控告警
- [x] 日志审计
- [x] Docker 多架构镜像
- [x] 完整文档体系 (146KB)
- [x] CI/CD 自动化
- [x] **nasctl CLI 工具** (v1.1.0 新增)
- [x] **文件浏览器增强** (v1.1.0 新增)
- [x] **媒体服务器** (v1.1.0 新增)
- [x] **下载中心** (v1.1.0 新增)
- [x] **通知模块** (v1.1.0 新增)
- [x] **相册功能** (v1.1.0 新增)
- [x] **共享管理** (v1.1.0 新增)
- [x] **备份同步** (v1.1.0 新增)
- [x] **用户管理** (v1.1.0 新增)
- [x] **系统设置** (v1.1.0 新增)

### 版本路线图
| 版本 | 类型 | 发布日期 | 核心功能 | 状态 |
|------|------|----------|----------|------|
| v0.1.0 | Alpha | 2026-03-10 | 项目骨架、btrfs 基础 | ✅ 发布 |
| v0.2.0 | Alpha | 2026-03-10 | 文件共享、配置持久化 | ✅ 发布 |
| v1.0.0 | Stable | 2026-03-11 | 生产就绪版本 | ✅ 已发布 |
| **v1.1.0** | **Stable** | **2026-03-12** | **功能大更新 (10 个新模块)** | ✅ **已发布** |
| v1.2.0 | Stable | 2026-04-xx | 安全加固、性能优化 | 🚀 规划中 |

## 部署

### Docker 部署
```bash
# 快速启动（开发测试）
docker-compose up -d

# 查看日志
docker-compose logs -f
```

### 裸机安装
```bash
# 一键安装脚本
curl -fsSL https://raw.githubusercontent.com/your-org/nas-os/main/scripts/install.sh | sudo bash

# 或手动安装
sudo ./scripts/install.sh
```

### 系统服务
```bash
systemctl status nas-os
systemctl restart nas-os
journalctl -u nas-os -f
```

## 配置示例

创建配置文件 `/etc/nas-os/config.yaml`:

```yaml
version: "1.0.0"

server:
  port: 8080
  host: 0.0.0.0
  tls_enabled: false

storage:
  data_path: /data
  auto_scrub: true
  scrub_schedule: "0 3 * * 0"  # 每周日凌晨 3 点

shares:
  smb:
    enabled: true
    workgroup: WORKGROUP
    shares:
      - name: public
        path: /data/public
        guest_ok: true
      - name: home
        path: /data/home
        guest_ok: false
  
  nfs:
    enabled: true
    allowed_networks:
      - 192.168.1.0/24
    exports:
      - path: /data/nfs
        clients: ["192.168.1.0/24"]
        options: ["rw", "sync", "no_subtree_check"]

logging:
  level: info
  format: json
  file: /var/log/nas-os/nasd.log
  max_size: 100
  max_backups: 5
```

## 快速使用

### 1. 创建存储卷
```bash
sudo nasctl volume create mydata --path /dev/sda1
```

### 2. 创建 SMB 共享
```bash
sudo nasctl share create smb public --path /data/public --guest
```

### 3. 创建 NFS 共享
```bash
sudo nasctl share create nfs backup --path /data/backup --network 192.168.1.0/24
```

### 4. 从客户端访问
- **Windows**: `\\<服务器 IP>\public`
- **macOS**: `smb://<服务器 IP>/public`
- **Linux (NFS)**: `sudo mount <服务器 IP>:/backup /mnt/local_backup`

## 获取帮助

- 📖 **完整文档**: [docs/](docs/) 目录
- 🐛 **报告问题**: [GitHub Issues](https://github.com/crazyqin/nas-os/issues)
- 💬 **社区讨论**: [GitHub Discussions](https://github.com/crazyqin/nas-os/discussions)
- 📦 **Docker 镜像**: [GHCR](https://github.com/crazyqin/nas-os/pkgs/container/nas-os)

## License

MIT
