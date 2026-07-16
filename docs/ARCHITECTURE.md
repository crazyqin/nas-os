# NAS-OS 架构说明

**版本**: v3.23.1  
**更新日期**: 2026-07-16

本文描述 NAS-OS 当前的进程组合、模块生命周期、API 安全边界和渐进迁移约束。模块分层以 **Core / Extension / Lab** 为准；目录与 `internal/application` catalog 标签必须一致，且 **运行时默认路径不得硬接线 Lab**。

## 进程组合

`cmd/nasd/main.go` 只负责：

1. 解析 `--config`；
2. 加载并校验 typed configuration；
3. 创建 `internal/application.Application`；
4. 处理进程信号和退出状态。

`internal/application` 是唯一组合根，负责显式构造依赖、注册核心模块、启动入口服务和逆序关闭资源。业务构造不得重新回到 `main.go` 或 Handler 中。

## 配置

根配置由 `internal/config` 统一加载：

- 默认配置：`configs/default.yaml`
- 启动参数：`nasd --config <path>`
- 环境变量覆盖：`NAS_OS_*`
- 路径、监听地址和端口在启动前统一校验

非法端口或相对系统路径会直接阻止启动，不再静默回退。

## 模块生命周期

生产核心模块图：

```text
identity   storage   network
    \         |        /
     \--------+-------/
              |
           sharing
              |
            system
```

其中 `sharing` 依赖 `identity`、`storage`、`network`；`system` 依赖其余核心模块。

统一模块契约：

```go
type Module interface {
    Name() string
    Tier() ModuleTier
    Dependencies() []string
    Init(context.Context) error
    Start(context.Context) error
    Stop(context.Context) error
    Health(context.Context) error
}
```

模块层级治理：

- **Core**：进程主生命周期必需能力，只允许 `identity / storage / network / sharing / system`；
- **Extension**：可选产品能力，**默认不加载**。通过配置 `modules.extensions: [name, ...]` 由 `internal/web/extensions_loader.go` 挂载路由或初始化管理器。已知名：`activeprotect`、`agentworkflow`、`aiguardrails`、`compliancescan`、`deployorch`、`netdiag`、`voicehub`（目录 `internal/extensions/*`）。未列入配置的 Extension **不会**出现在默认 `nasd` HTTP 面。
- **Lab**：实验性、概念验证或待收编模块，位于 `internal/lab/*`。**v3.23.0**：生产 `internal/web`（非测试）**禁止** import Lab；默认启动不再构造 Lab 管理器。需要实验能力时在独立分支/构建中评估，或将来以 Extension 形式显式启用。
- **未知 catalog 名**：`ModuleTierFor` 默认 **Lab**（不再默认 Extension，避免未编目包伪装成产品扩展）。
- **治理锁**：`internal/application/governance_test.go` + `toplevel_allowlist.txt` 禁止 web→lab 回流、Core 扩编、顶层业务包随意新增。

容器保证：

- 注册时拒绝 nil、空名称和重复模块；
- 初始化前检查缺失依赖和依赖环；
- 按确定性拓扑顺序初始化、启动；
- 启动失败时逆序回滚已启动模块；
- 停止时逆序执行并聚合错误；
- 健康回调在容器锁外执行，状态按名称稳定排序；
- 模块状态接口会暴露 tier，供 Core / Extension / Lab 收敛审计。

## 依赖注入原则

生命周期容器不是字符串 Service Locator。

- Manager 和 Handler 通过构造函数显式注入；
- 模块实例由 `internal/application` 创建；
- `Container.Register/Get` 仅用于兼容尚未迁移的代码，不得成为新增业务依赖的默认方式；
- 构造函数不得启动 goroutine；后台任务必须由 `Start` 启动、由 `Stop` 关闭；
- 每项后台任务只能有一个生命周期所有者。

## 路由与安全边界

API 分成三层：

1. 公开路由：仅登录和健康探针；
2. 已认证路由：账户自身操作；
3. 管理员路由：存储、共享、网络、系统管理等敏感操作。

模块状态接口返回 `name + tier + healthy + error`，便于识别伪核心模块。

模块可按需要实现：

- `PublicRouteRegistrar`
- `AuthenticatedRouteRegistrar`
- `RouteRegistrar`（管理员路由）

身份只能来自服务端认证上下文。不得信任普通客户端身份 Header，也不得通过查询参数传递认证令牌。

健康探针：

- 主路径：`GET /api/v1/system/health`
- 兼容路径：`GET /api/v1/health`
- 管理员模块状态：`GET /api/v1/system/modules`

## 启停顺序

启动：

1. 加载并校验配置；
2. 显式构造 Manager；
3. 注册并初始化核心模块；
4. 按拓扑顺序启动模块；
5. 启动 HTTP 入口。

关闭：

1. HTTP 停止接收请求并等待在途请求；
2. Web 所有 worker 停止；
3. 核心模块逆序停止；
4. 下载、集群等进程级资源关闭。

## 存储 API 迁移约束（弃用时间表）

当前保留两组生产契约：

| 契约 | 路径 | 状态 |
|------|------|------|
| 历史 | `/api/v1/volumes/*` | **兼容保留**；新客户端请勿新增依赖 |
| 现行 | `/api/v1/storage/*` | **推荐**；与新 handler 字段对齐中 |

`internal/storage.Handlers` 的字段、状态码和错误语义与历史契约不同，**禁止**一次性双注册切换。迁移必须逐端点进行，并用 contract/golden test 保证旧路径、HTTP 方法和响应格式不变。

**弃用计划（多版本）**：

1. **v3.23.x**：双契约并存；文档与 OpenAPI 优先描述 `/storage`。
2. **后续次版本**：对 `/volumes` 响应增加弃用头/日志提示（不破坏体）。
3. **未来主版本**：在客户端迁移完成后移除 `/volumes`（需 CHANGELOG 与发布说明明确）。

## 入口与部署（单一主路径）

| 角色 | 主路径 | 说明 |
|------|--------|------|
| 进程入口 | `cmd/nasd` → `internal/application` | 唯一生产守护进程组合根 |
| HTTP | `internal/web` | 路由与可选 Extension 挂载 |
| 主 UI | `webui/` | 静态 HTML/JS 管理界面（**主产品 UI**） |
| 实验前端 | `web/src` | 部分 React 视图，**非**默认交付主线 |
| 根 `api/` | **非 nasd 入口** | 见 `api/README.md`；勿当作线上 API |
| 主部署 | `docker-compose.yml` + `Dockerfile` | 开发/单机默认；`*.prod` / `*.ai` 为场景叠加 |

## 健康探针

- `GET /api/v1/system/health` 与 `GET /api/v1/health`：聚合 **Core** 模块 `Health()`；任一失败则 `data.status=unhealthy`、`code=1`（HTTP 仍 200，便于 LB 解析 body）。
- `GET /api/v1/system/modules`（管理员）：含 tier 的模块状态列表。
- `GET /api/v1/system/info`：版本来自 `internal/version`（与根 `VERSION` 同步）。

## 功能矩阵（诚实口径）

| 层级 | 默认 `nasd` | 如何启用 |
|------|-------------|----------|
| Core（5） | 始终 | 生命周期主图 |
| Extension（7） | **否** | `modules.extensions` 列表 |
| Lab（~467） | **否** | 不在默认路径；仅源码保留 |
| 其他顶层支撑包 | 视 web 硬接线 | 生产支撑（auth/storage/…），**非 Core 名** |

## 渐进迁移规则

- 冻结新增顶层 `internal` 业务模块（allowlist 测试）；新实验进 `lab/`，可选产品进 `extensions/`；
- **v3.23.0**：Lab 默认运行时剥离；Extension 按配置加载；Core 健康聚合；治理测试入仓；
- **v3.22.0**：163 伪核心目录降入 Lab；catalog 路径对齐；
- Core 生命周期主图仍仅允许 `identity` / `storage` / `network` / `sharing` / `system`；
- 每个提交保持可构建、可测试、可回滚；
- 删除兼容路由前必须先完成客户端迁移和弃用周期。
