# NAS-OS v2.7.0 Release Notes

**发布日期**: 2026-03-14

## 🎯 版本亮点

v2.7.0 聚焦于 **测试覆盖完善**，大幅提升代码质量和可靠性。

## ✨ 新增功能

### 测试覆盖完善

#### API 端点集成测试
新增 `tests/integration/api_endpoints_test.go`，覆盖 40+ API 端点：

| 模块 | 测试端点 |
|------|----------|
| 系统 | `/api/v1/health`, `/api/v1/system/info` |
| 存储 | `/api/v1/volumes`, `/api/v1/subvolumes`, `/api/v1/snapshots` |
| 用户 | `/api/v1/users` (CRUD) |
| 认证 | `/api/v1/auth/login`, `/api/v1/auth/logout`, `/api/v1/auth/refresh` |
| 配额 | `/api/v1/quota` |
| 共享 | `/api/v1/shares` |
| 备份 | `/api/v1/backup/jobs` |
| 去重 | `/api/v1/dedup/status`, `/api/v1/dedup/scan` |
| 搜索 | `/api/v1/search` |
| 监控 | `/api/v1/monitor/metrics`, `/api/v1/monitor/alerts` |
| 分层存储 | `/api/v1/tiering/tiers`, `/api/v1/tiering/policies` |
| 压缩存储 | `/api/v1/compress/status` |
| 协议 | `/api/v1/webdav/*`, `/api/v1/ftp/*`, `/api/v1/iscsi/*` |
| 容器 | `/api/v1/containers` |
| 插件 | `/api/v1/plugins` |

#### WebUI 端到端测试
新增 `tests/e2e/webui_test.go`，覆盖 25+ UI 功能：

- **仪表板**: 系统信息、存储状态、网络状态、服务状态
- **存储管理**: 卷详情、子卷管理、快照管理
- **用户管理**: 用户列表、用户创建、权限管理
- **系统设置**: 网络设置、时间设置、安全设置
- **日志查看**: 日志列表、日志流
- **服务管理**: 服务列表、启动/停止/重启服务
- **监控**: 分层存储可视化、压缩管理、性能监控
- **告警**: 告警列表、告警确认

#### 性能基准测试
新增 `tests/benchmark/performance_test.go`，30+ 基准测试：

| 类别 | 测试项 |
|------|--------|
| 版本信息 | Info(), String() |
| 存储 | RAID 配置、卷创建、快照创建 |
| 缓存 | Set/Get/Delete、并发访问 |
| JSON | 编码/解码 Volume、Snapshot |
| HTTP API | 健康检查、卷列表、系统信息、性能数据 |
| 并发 | API 并发请求、缓存并发操作 |
| 内存 | 对象创建、JSON 序列化、HTTP 响应 |

### Dockerfile 优化

- 使用 `golang:1.24-alpine` 构建镜像
- 添加 UPX 二进制压缩，镜像大小减少约 15%
- 添加 `nasctl` CLI 编译
- 增强 OCI 标签和元数据
- 优化健康检查命令

### 监控增强

- 整合并优化 Prometheus 告警规则 (`alerts.yml`)
- 添加 CPU、内存、磁盘、网络、服务、Btrfs 等多维度告警
- 增强健康检查模块，添加 Btrfs 和共享服务检查
- 完善 API 处理器，支持更丰富的监控端点

## 🐛 问题修复

- 修复 `internal/quota/history.go` 编译错误
  - 修复未使用的 `quotaID` 变量
  - 修复 `TrendStatistics.GrowthRate` 字段名错误

## 📊 测试统计

| 类型 | 数量 |
|------|------|
| API 端点测试 | 40+ |
| WebUI 测试 | 25+ |
| 性能基准测试 | 30+ |
| **总计** | **95+** |

## 📦 安装

### Docker

```bash
docker pull nas-os/nasd:v2.7.0
```

### 二进制

```bash
# Linux ARM64
wget https://github.com/nas-os/nas-os/releases/download/v2.7.0/nasd-linux-arm64
chmod +x nasd-linux-arm64
./nasd-linux-arm64

# Linux AMD64
wget https://github.com/nas-os/nas-os/releases/download/v2.7.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
./nasd-linux-amd64
```

## 🔗 相关链接

- [文档](https://docs.nas-os.io)
- [API 参考](https://api.nas-os.io)
- [GitHub](https://github.com/nas-os/nas-os)

---

**完整变更日志**: [CHANGELOG.md](./CHANGELOG.md)