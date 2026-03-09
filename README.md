# NAS-OS 🖥️

基于 Go 的家用 NAS 系统，支持 btrfs 存储管理、SMB/NFS 共享、Web 管理界面。

## 特性

- 💾 **btrfs 存储管理** - 卷、子卷、快照、RAID
- 🌐 **Web 管理界面** - 简洁易用的可视化操作
- 📁 **文件共享** - SMB/CIFS、NFS
- 👥 **用户权限** - 多用户、访问控制
- 📊 **监控告警** - 磁盘健康、空间预警
- 🐳 **Docker 集成** - 容器应用支持（开发中）

## 快速开始

### 依赖

```bash
# 安装 Go 1.21+
# 安装 btrfs 工具
sudo apt install btrfs-progs

# 安装 Samba（如需 SMB 共享）
sudo apt install samba

# 安装 NFS（如需 NFS 共享）
sudo apt install nfs-kernel-server
```

### 构建

```bash
cd nas-os
go mod tidy
go build -o nasd ./cmd/nasd
go build -o nasctl ./cmd/nasctl
```

### 运行

```bash
# 需要 root 权限（访问磁盘设备）
sudo ./nasd
```

访问 http://localhost:8080

## API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/volumes | 获取卷列表 |
| POST | /api/v1/volumes | 创建卷 |
| GET | /api/v1/volumes/:name | 获取卷详情 |
| POST | /api/v1/volumes/:name/subvolumes | 创建子卷 |
| POST | /api/v1/volumes/:name/snapshots | 创建快照 |
| POST | /api/v1/volumes/:name/balance | 平衡数据 |
| POST | /api/v1/volumes/:name/scrub | 数据校验 |

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

### 当前状态 (2026-03-10)
- [x] 项目骨架
- [x] btrfs 基础管理框架
- [x] Web 框架
- [x] 项目文档体系
- [ ] SMB/NFS 共享实现 (M3)
- [ ] 用户/权限系统 (M4)
- [ ] 磁盘监控告警 (M5)
- [ ] Docker 集成 (M6)
- [ ] 系统设置
- [ ] 日志审计

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

## License

MIT
