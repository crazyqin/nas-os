# NAS-OS 架构说明

本文描述 NAS-OS 当前的进程组合、模块生命周期、API 安全边界和渐进迁移约束。

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
- **Extension**：可选产品能力，允许保留独立 handler / manager，但不得伪装成启动主图核心；当前已收敛示例：`internal/extensions/activeprotect`、`internal/extensions/agentworkflow`、`internal/extensions/aiguardrails`、`internal/extensions/voicehub`；
- **Lab**：实验性、概念验证或待收编模块，默认不进入生产核心图，优先降级、隔离或删除重复实现；当前已收敛示例：`internal/lab/aimediatag`、`internal/lab/benchmarkpro`、`internal/lab/blockdedup2`、`internal/lab/brandinsight`、`internal/lab/cloudsync2`、`internal/lab/costbenchmark`、`internal/lab/datasovereignty2`、`internal/lab/digitalassetvault`、`internal/lab/draid2`、`internal/lab/featurematrix`、`internal/lab/familyactivityhub`、`internal/lab/filetimemachine2`、`internal/lab/forensics2`、`internal/lab/iotedgegateway`、`internal/lab/netshield`、`internal/lab/posterwallpro`、`internal/lab/releasemanager`、`internal/lab/resmonpro`、`internal/lab/safeaccess`、`internal/lab/storagecostpredict`、`internal/lab/truecloudbk`、`internal/lab/updatedirector`。

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

## 存储 API 迁移约束

当前保留两组生产契约：

- 历史契约：`/api/v1/volumes/*`
- 兼容契约：`/api/v1/storage/*`

`internal/storage.Handlers` 的字段、状态码和错误语义与历史契约不同，暂不直接替换。迁移必须逐端点进行，并用 contract/golden test 保证旧路径、HTTP 方法和响应格式不变。禁止一次性双注册或切换实现。

## 渐进迁移规则

- 冻结新增顶层 `internal` 业务模块，优先归入现有领域；
- 已完成一批目录收敛：`activeprotect`、`agentworkflow`、`aiguardrails`、`voicehub` 已迁入 `internal/extensions/`；`aimediatag`、`benchmarkpro`、`blockdedup2`、`brandinsight`、`cloudsync2`、`filetimemachine2`、`releasemanager`、`resmonpro`、`smartthermal2` 以及 `costbenchmark`、`datasovereignty2`、`digitalassetvault`、`draid2`、`featurematrix`、`familyactivityhub`、`forensics2`、`iotedgegateway`、`netshield`、`posterwallpro`、`safeaccess`、`storagecostpredict`、`truecloudbk`、`updatedirector` 已迁入 `internal/lab/`；
- 每次只迁移一个低风险领域或一组端点；
- 每个提交保持可构建、可测试、可回滚；
- 新模块优先使用小接口和显式构造函数；
- 删除兼容路由前必须先完成客户端迁移和弃用周期。
