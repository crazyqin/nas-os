# NAS-OS Scripts 目录

本目录包含运维和管理脚本。

## 脚本列表

### 健康检查
| 脚本 | 版本 | 用途 |
|------|------|------|
| `health-check.sh` | v2.56.0 | 完整健康检查（服务、磁盘、内存、API） |
| `healthcheck.sh` | v2.35.0 | Docker/K8s 健康检查探针（轻量级） |
| `quick-status.sh` | v2.75.0 | 快速状态检查（运维巡检） |
| `service-health-check.sh` | v2.74.0 | 服务健康检查（HTTP探针、进程检测） |

### 服务管理
| 脚本 | 版本 | 用途 |
|------|------|------|
| `service.sh` | v2.75.0 | 统一服务管理（start/stop/restart/status/logs） |
| `service-status.sh` | v1.0.0 | 服务状态检查（systemd/Docker） |
| `service-diagnose.sh` | v2.91.0 | 服务诊断（状态、端口、依赖、日志分析、修复建议） |
| `auto-recovery.sh` | - | 自动故障恢复 |
| `service-monitor.sh` | - | 服务监控守护进程 |
| `monitor-daemon.sh` | v1.0.0 | 后台监控守护进程（CPU/内存/磁盘/服务/告警） |

### 版本管理
| 脚本 | 版本 | 用途 |
|------|------|------|
| `version-info.sh` | v2.75.0 | 版本信息查看（支持更新检查） |
| `rollback.sh` | - | 版本回滚 |

### 备份管理
| 脚本 | 版本 | 用途 |
|------|------|------|
| `backup-monitor.sh` | v2.59.0 | 备份任务监控、完整性验证、告警 |
| `backup.sh` | - | 备份执行脚本 |
| `backup-rotate.sh` | - | 备份轮转管理 |
| `backup-rotation.sh` | v2.74.0 | 备份生命周期管理（压缩、归档、清理） |
| `backup-verify.sh` | v2.70.0 | 备份完整性验证（校验和、格式、内容检查） |
| `restore.sh` | - | 数据恢复脚本 |
| `db-backup.sh` | - | 数据库备份专用 |

### 磁盘管理
| 脚本 | 版本 | 用途 |
|------|------|------|
| `disk-health-check.sh` | v2.59.0 | SMART 状态检查、温度监控、健康评分 |

### 日志管理
| 脚本 | 版本 | 用途 |
|------|------|------|
| `log-rotate.sh` | v2.58.0 | 日志轮转、压缩、清理 |
| `log-analyzer.sh` | - | 日志分析 |

### 部署运维
| 脚本 | 用途 |
|------|------|
| `deploy.sh` | 一键部署（二进制/Docker） |
| `deploy-check.sh` | 部署检查 |
| `install.sh` | 安装脚本 |
| `migrate.sh` | 数据迁移 |
| `maintenance.sh` | 系统维护（日志清理、临时文件清理） |
| `config-validator.sh` | 配置验证 |

### 测试性能
| 脚本 | 用途 |
|------|------|
| `test.sh` | 测试运行器（单元/集成/E2E） |
| `performance-test.sh` | 性能测试 |
| `load-test.sh` | 负载测试 |

### 安全
| 脚本 | 用途 |
|------|------|
| `security-check.sh` | 安全检查 |
| `security-cron.sh` | 安全定时任务 |

## 使用方式

```bash
# 快速状态（运维巡检推荐）
./scripts/quick-status.sh            # 文本输出
./scripts/quick-status.sh --json     # JSON 输出

# 服务管理
./scripts/service.sh start           # 启动服务
./scripts/service.sh stop            # 停止服务
./scripts/service.sh restart         # 重启服务
./scripts/service.sh status          # 查看状态
./scripts/service.sh logs            # 查看日志

# 健康检查
./scripts/health-check.sh            # 完整检查
./scripts/health-check.sh --json     # JSON 输出
./scripts/health-check.sh --quick    # 快速检查

# 版本信息
./scripts/version-info.sh            # 查看版本
./scripts/version-info.sh --check    # 检查更新

# 备份监控
./scripts/backup-monitor.sh          # 一次性检查
./scripts/backup-monitor.sh --daemon # 守护进程
./scripts/backup-monitor.sh --report # 生成报告

# 磁盘健康
./scripts/disk-health-check.sh       # 检查所有磁盘
./scripts/disk-health-check.sh --json

# 日志轮转
./scripts/log-rotate.sh              # 执行轮转
./scripts/log-rotate.sh --dry-run    # 预览模式

# 维护
./scripts/maintenance.sh             # 执行所有维护任务
./scripts/maintenance.sh --logs      # 仅清理日志
./scripts/maintenance.sh --check     # 系统检查
```

## Makefile 命令

```bash
make health          # 完整健康检查
make health-quick    # 快速检查
make backup-monitor  # 备份监控
make disk-health     # 磁盘健康检查
make quick-status    # 快速状态
make version         # 版本信息
```

## 版本说明

- v2.75.0: 新增 quick-status.sh、service.sh、version-info.sh
- v2.74.0: 服务健康检查、备份轮转增强
- v2.59.0: 备份监控、磁盘健康检查增强
- v2.58.0: 日志轮转脚本更新
- v2.56.0: 健康检查脚本完善
- v2.35.0: Docker 健康检查探针