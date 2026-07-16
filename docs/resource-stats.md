# NAS-OS 资源统计

**更新日期**: 2026-07-16  
**版本**: v3.22.0

## 代码规模

| 指标 | 数值 |
|------|------|
| `internal/` 顶层目录 | ~330 |
| Lab 包（`internal/lab/*`） | ~467 |
| Extension 包（`internal/extensions/*`） | 7 |
| Go 源文件数 | ~3,805 |
| 测试文件数 | ~1,022 |
| Go 代码总行数（约） | ~1,670,000 |
| go.mod require 依赖数 | ~172 |

> 统计为仓库快照近似值；以 `find`/`wc` 与 `go list` 实测为准。模块数按目录计，不等于 Core 生命周期注册数。

## 架构分层（现行）

| 层级 | 数量/范围 | 说明 |
|------|-----------|------|
| **Core** | 5 | `identity` / `storage` / `network` / `sharing` / `system`（唯一进入生产生命周期主图） |
| **Extension** | 7 | `internal/extensions/{activeprotect,agentworkflow,aiguardrails,compliancescan,deployorch,netdiag,voicehub}` |
| **Lab** | ~467 | `internal/lab/*`：实验、重复、零生产引用实现 |
| 其余顶层 | 支持/领域包 | 如 `application`、`arch`、`web`、`auth`、`storage`、`api` 等生产支撑，**不得标为 Core** |

### 近期收敛波次

| 版本 | 变化 |
|------|------|
| v3.22.0 | 再降 163 个零引用伪核心 → Lab；catalog 与路径对齐（含 activebackup/reports/smartpricing 等） |
| v3.21.0 | 195+ AI/smart/backup/security/media/vm/cluster 等 → Lab |
| v3.18–v3.20 | Extension 命名空间、pro/v2/重复告警/quantum/cost 等分批降级 |

## 生产入口

| 组件 | 路径 | 说明 |
|------|------|------|
| 守护进程 | `cmd/nasd` | 仅组合配置与 `internal/application` |
| 组合根 | `internal/application` | 注册 Core 模块、启动 Web |
| 版本 | `VERSION` + `internal/version` | `GET /api/v1/system/info` 返回真实版本 |
| 架构说明 | `docs/ARCHITECTURE.md` | Core / Extension / Lab 治理规则 |

## 竞品参考（能力对标，非模块路径）

- **Synology DSM 7.4**: Website AI Advisor、ActiveProtect、SSD Cache Advisor、Power Schedule、CMS
- **TrueNAS 26**: L2ARC tiering、Cloud Sync cost tracking、Cluster、TrueSearch AI
- **飞牛 fnOS**: 影视墙刮削、家庭影音体验、远程访问、多设备管理

> 对标功能可能落在 Extension 或 Lab；是否进入生产主图以 Core 注册与 import 图为准，勿将 Lab 包描述为顶层核心模块。
