# NAS-OS 部署指南

## 快速开始

### 方式一：Docker 部署（推荐开发测试）
```bash
# 1. 克隆项目
git clone https://github.com/crazyqin-org/nas-os.git
cd nas-os

# 2. 启动服务
docker-compose up -d

# 3. 访问 Web UI
# http://localhost:8080
```

### 方式二：裸机部署（推荐生产环境）
```bash
# 1. 一键安装
curl -fsSL https://raw.githubusercontent.com/your-org/nas-os/main/scripts/install.sh | sudo bash

# 2. 验证状态
systemctl status nas-os

# 3. 访问 Web UI
# http://<服务器 IP>:8080
```

### 方式三：手动编译
```bash
# 1. 安装 Go 1.25+
# 2. 编译
make build

# 3. 运行
sudo ./nasd
```

## 目录结构
```
nas-os/
├── Dockerfile              # Docker 镜像构建
├── docker-compose.yml      # 容器编排
├── Makefile               # 构建自动化
├── scripts/
│   └── install.sh         # 系统安装脚本
├── monitoring/
│   ├── prometheus.yml     # 监控配置
│   └── alerts.yml         # 告警规则
├── docs/
│   └── RESOURCES.md       # 系统资源需求
├── .github/workflows/
│   └── ci-cd.yml          # CI/CD 流程
└── configs/
    └── default.yaml       # 默认配置
```

## 配置说明

### 核心配置 (/etc/nas-os/config.yaml)
```yaml
server:
  port: 8080          # Web UI 端口
  host: 0.0.0.0       # 监听地址

storage:
  mount_base: /mnt    # 存储挂载点
  auto_scrub: true    # 自动数据校验

smb:
  enabled: true       # SMB 共享
  workgroup: WORKGROUP

nfs:
  enabled: true       # NFS 共享
  allowed_networks:
    - 192.168.1.0/24
```

## 端口说明

| 端口 | 协议 | 用途 |
|------|------|------|
| 8080 | TCP | Web 管理界面 |
| 445 | TCP | SMB/CIFS 文件共享 |
| 2049 | TCP/UDP | NFS 文件共享 |
| 111 | TCP/UDP | RPC (NFS 必需) |

## 系统服务管理

```bash
# 查看状态
systemctl status nas-os

# 启动/停止/重启
systemctl start nas-os
systemctl stop nas-os
systemctl restart nas-os

# 开机自启
systemctl enable nas-os
systemctl disable nas-os

# 查看日志
journalctl -u nas-os -f
journalctl -u nas-os --since "1 hour ago"
```

## CI/CD 流程

### 触发条件
- **Push**: 自动运行 lint + test
- **PR**: 运行完整 CI 检查
- **Tag (v*)**: 构建 + 发布 + Docker 推送

### 构建产物
- 多平台二进制 (linux/amd64, arm64, armv7)
- Docker 镜像 (GHCR)
- GitHub Release

### 手动触发
```bash
# GitHub Actions -> Run workflow
# 或本地构建
make build-all
make docker-build
```

## 监控告警

### Prometheus 指标
- `/metrics` - 系统指标
- 磁盘使用率
- CPU/内存使用
- btrfs 健康状态

### 告警规则
- 磁盘空间 <20% (警告) / <5% (严重)
- 服务宕机 >1 分钟
- CPU/内存 >80%
- btrfs 设备错误

### 查看监控
```bash
# 启动监控栈
docker-compose up -d prometheus grafana

# 访问 Grafana
# http://localhost:3000 (admin/admin123)
```

## 故障排查

### 服务无法启动
```bash
# 检查日志
journalctl -u nas-os -n 50

# 检查端口占用
ss -tulpn | grep 8080

# 检查配置
nasd --config /etc/nas-os/config.yaml --test
```

### 磁盘访问问题
```bash
# 检查 btrfs
btrfs filesystem show

# 检查挂载点
mount | grep /mnt

# 检查权限
ls -la /mnt/
```

### 网络问题
```bash
# 检查防火墙
ufw status
firewall-cmd --list-all

# 测试端口
telnet localhost 8080
curl http://localhost:8080/api/v1/health
```

## 升级流程

### Docker 升级
```bash
docker-compose pull
docker-compose up -d
```

### 裸机升级
```bash
# 下载新版本
sudo ./scripts/install.sh

# 或手动替换
sudo systemctl stop nas-os
sudo curl -L <release-url>/nasd-linux-amd64 -o /usr/local/bin/nasd
sudo chmod +x /usr/local/bin/nasd
sudo systemctl start nas-os
```

## 备份恢复

### 备份配置
```bash
tar -czf nas-os-backup-$(date +%Y%m%d).tar.gz \
    /etc/nas-os/ \
    /var/lib/nas-os/
```

### 恢复配置
```bash
tar -xzf nas-os-backup-YYYYMMDD.tar.gz -C /
systemctl restart nas-os
```

## 安全建议

1. **修改默认密码** - 首次启动后修改 admin 密码
2. **限制网络访问** - 配置防火墙只允许可信 IP
3. **启用 HTTPS** - 生产环境使用反向代理 (nginx/caddy)
4. **定期更新** - 保持系统和 NAS-OS 最新
5. **监控日志** - 配置日志告警

## 性能优化

参考 `docs/RESOURCES.md` 获取详细的:
- 系统资源需求
- 内核参数调优
- btrfs 挂载选项
- Samba/NFS 优化

## 获取帮助

- 📖 文档：`docs/` 目录
- 🐛 问题：GitHub Issues
- 💬 讨论：GitHub Discussions
- 📧 联系：support@your-org.com
