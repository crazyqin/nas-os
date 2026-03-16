# nas-os v2.120.0 测试补充计划

## 测试覆盖率统计

**总覆盖率: 30.5%**

## 覆盖率 < 30% 的模块

### 优先级 P0 (核心功能，覆盖率 < 15%)

| 模块 | 覆盖率 | 测试文件状态 | 补充计划 |
|------|--------|-------------|---------|
| `internal/web` | 9.0% | 缺失 | 添加 server/handlers 测试 |
| `internal/ldap` | 12.9% | 部分缺失 | 添加 LDAP 认证测试 |
| `internal/disk` | 13.1% | 部分缺失 | 添加磁盘操作测试 |
| `internal/media` | 13.0% | 部分缺失 | 添加媒体库测试 |
| `internal/quota` | 13.6% | 部分缺失 | 添加配额管理测试 |
| `internal/snapshot` | 14.2% | 部分缺失 | 添加快照操作测试 |
| `internal/vm` | 14.8% | 部分缺失 | 添加 VM 管理测试 |
| `internal/compress` | 15.1% | 部分缺失 | 添加压缩测试 |

### 优先级 P1 (重要功能，覆盖率 15-25%)

| 模块 | 覆盖率 | 测试文件状态 | 补充计划 |
|------|--------|-------------|---------|
| `internal/project` | 17.2% | 部分缺失 | 添加项目管理测试 |
| `internal/cloudsync` | 18.7% | 部分缺失 | 添加云同步测试 |
| `internal/container` | 18.2% | 部分缺失 | 添加容器管理测试 |
| `internal/downloader` | 18.2% | 部分缺失 | 添加下载器测试 |
| `internal/security` | 19.2% | 部分缺失 | 添加安全模块测试 |
| `internal/office` | 20.9% | 部分缺失 | 添加 Office 测试 |
| `internal/photos` | 20.9% | 部分缺失 | 添加照片管理测试 |
| `internal/ftp` | 21.8% | 部分缺失 | 添加 FTP 测试 |
| `internal/network` | 22.4% | 部分缺失 | 添加网络配置测试 |
| `internal/logging` | 22.5% | 部分缺失 | 添加日志系统测试 |
| `internal/budget` | 22.8% | 部分缺失 | 添加预算管理测试 |
| `internal/perf` | 22.9% | 部分缺失 | 添加性能优化测试 |
| `internal/service` | 22.1% | 部分缺失 | 添加服务管理测试 |
| `internal/docker` | 23.2% | 部分缺失 | 添加 Docker 测试 |

### 优先级 P2 (辅助功能，覆盖率 25-30%)

| 模块 | 覆盖率 | 测试文件状态 | 补充计划 |
|------|--------|-------------|---------|
| `internal/monitor` | 25.6% | 部分缺失 | 添加监控测试 |
| `internal/notification` | 25.1% | 部分缺失 | 添加通知测试 |
| `internal/webdav` | 26.6% | 部分缺失 | 添加 WebDAV 测试 |
| `internal/database` | 28.0% | 部分缺失 | 添加数据库测试 |
| `internal/cluster` | 28.0% | 部分缺失 | 添加集群测试 |
| `internal/plugin` | 28.1% | 部分缺失 | 添加插件测试 |
| `internal/transfer` | 28.6% | 部分缺失 | 添加传输测试 |
| `internal/reports` | 28.9% | 部分缺失 | 添加报告测试 |
| `internal/notify` | 29.1% | 部分缺失 | 添加通知测试 |

### 0% 覆盖率模块

| 模块 | 说明 | 测试计划 |
|------|------|---------|
| `cmd/backup` | 备份命令行 | 添加 CLI 测试 |
| `cmd/nasctl` | 管理命令行 | 添加 CLI 测试 |
| `cmd/nasd` | 主守护进程 | 添加启动测试 |
| `docs` | 文档模块 | 无需测试 |
| `pkg/safeguards` | 安全防护 | 添加安全测试 |
| `plugins/*` | 插件 | 添加插件测试 |
| `tests/fixtures` | 测试工具 | 无需测试 |
| `tests/reports` | 报告工具 | 添加报告测试 |

## 测试补充策略

### 阶段 1: 核心模块 (P0)
**目标**: 将核心模块覆盖率提升至 40%+

1. `internal/web` - 添加 HTTP 处理器测试
2. `internal/disk` - 添加磁盘管理测试
3. `internal/snapshot` - 添加快照功能测试
4. `internal/quota` - 添加配额功能测试

### 阶段 2: 重要模块 (P1)
**目标**: 将重要模块覆盖率提升至 35%+

1. `internal/docker` - 添加容器操作测试
2. `internal/security` - 添加安全模块测试
3. `internal/network` - 添加网络配置测试

### 阶段 3: 辅助模块 (P2)
**目标**: 将辅助模块覆盖率提升至 30%+

1. `internal/monitor` - 添加监控测试
2. `internal/cluster` - 添加集群测试
3. `internal/plugin` - 添加插件测试

## 测试补充任务清单

### v2.120.0 目标
- [ ] P0 模块覆盖率提升至 30%+
- [ ] 总覆盖率提升至 35%+
- [ ] go vet 零错误

### 具体任务
1. **internal/web** (9.0% → 30%+)
   - [ ] server_test.go - 服务器启动/停止测试
   - [ ] handlers_test.go - HTTP 处理器测试

2. **internal/disk** (13.1% → 30%+)
   - [ ] manager_test.go - 磁盘管理器测试
   - [ ] smart_test.go - SMART 数据测试

3. **internal/snapshot** (14.2% → 30%+)
   - [ ] manager_test.go - 快照管理测试
   - [ ] policy_test.go - 快照策略测试

4. **internal/quota** (13.6% → 30%+)
   - [ ] manager_test.go - 配额管理测试
   - [ ] enforcer_test.go - 配额强制测试

## 质量指标

| 指标 | 当前值 | 目标值 |
|------|--------|--------|
| 总覆盖率 | 30.5% | 35%+ |
| P0 模块覆盖率 | ~12% | 30%+ |
| P1 模块覆盖率 | ~20% | 25%+ |
| go vet 错误 | 0 | 0 |

---

*生成时间: 2026-03-16*
*版本: v2.120.0*