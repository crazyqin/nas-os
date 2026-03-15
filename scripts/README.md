# NAS-OS Scripts 目录

本目录包含运维和管理脚本。

## 脚本列表

### 健康检查
| 脚本 | 版本 | 用途 |
|------|------|------|
| `health-check.sh` | v2.56.0 | 完整健康检查（服务、磁盘、内存、API） |
| `healthcheck.sh` | v2.35.0 | Docker/K8s 健康检查探针（轻量级） |

### 备份管理
| 脚本 | 版本 | 用途 |
|------|------|------|
| `backup-monitor.sh` | v2.59.0 | 备份任务监控、完整性验证、告警 |
| `backup.sh` | - | 备份执行脚本 |
| `backup-rotate.sh` | - | 备份轮转管理 |
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

### 服务管理
| 脚本 | 版本 | 用途 |
|------|------|------|
| `service-status.sh` | v1.0.0 | 服务状态检查（systemd/Docker） |
| `auto-recovery.sh` | - | 自动故障恢复 |
| `service-monitor.sh` | - | 服务监控守护进程 |

### 部署运维
| 脚本 | 用途 |
|------|------|
| `deploy.sh` | 一键部署（二进制/Docker） |
| `pre-deploy-check.sh` | 部署前环境检查 |
| `install.sh` | 安装脚本 |
| `migrate.sh` | 数据迁移 |
| `rollback.sh` | 版本回滚 |

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
# 健康检查
./scripts/health-check.sh           # 完整检查
./scripts/health-check.sh --json    # JSON 输出
./scripts/health-check.sh --quick   # 快速检查

# 备份监控
./scripts/backup-monitor.sh         # 一次性检查
./scripts/backup-monitor.sh --daemon # 守护进程
./scripts/backup-monitor.sh --report # 生成报告

# 磁盘健康
./scripts/disk-health-check.sh      # 检查所有磁盘
./scripts/disk-health-check.sh --json

# 日志轮转
./scripts/log-rotate.sh             # 执行轮转
./scripts/log-rotate.sh --dry-run   # 预览模式

# 服务状态
./scripts/service-status.sh         # 检查状态
./scripts/service-status.sh --watch # 持续监控
```

## Makefile 命令

```bash
make health          # 完整健康检查
make health-quick    # 快速检查
make backup-monitor  # 备份监控
make disk-health     # 磁盘健康检查
```

## 版本说明

- v2.59.0: 备份监控、磁盘健康检查增强
- v2.58.0: 日志轮转脚本更新
- v2.56.0: 健康检查脚本完善
- v2.35.0: Docker 健康检查探针