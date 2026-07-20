# ADR-0001：目标架构 — Platform + Packages + Host SDK

| 字段 | 内容 |
|------|------|
| **状态** | Accepted — **Stage 0–3 implemented** |
| **日期** | 2026-07-20 |
| **范围** | 产品叙事、模块交付、默认表面、生态边界 |
| **不影响** | 默认 `nasd` 仍仅 Core；Stage 0–3 **不改变** v3.24 默认行为 |
| **相关** | [ARCHITECTURE.md](../ARCHITECTURE.md)（现行运行时说明） |
| **Stage 1 代码** | `internal/config/packages.go`（`ResolvePackages` / `OptionalProductsEnabled`）；生产门闸：`internal/web`、`internal/application` |
| **Stage 2 代码** | `pkg/hostapi`（Host SDK v1.0.0）；`internal/packageruntime`；web 扩展经 Runtime 启用；`GET /api/v1/packages` |
| **Stage 3 代码** | `config.SystemPackageCatalog`；文档/默认 YAML 主路径 `packages.*`；`modules.*` 弃用 warn 兼容 |
| **Community 路径** | `packages.community_dir` 磁盘发现 + `packageruntime.RegisterDiscovered`；Host SDK only；默认不加载 |

---

## 1. 背景

现行工程分层是 **Core / Extension / Lab**，另有：

- `modules.optional`：批开 Docker/VM/Photos/AI/备份等产品管理器  
- `modules.extensions`：按名加载 `internal/extensions/*`  
- `internal/plugin` + `plugins/*`：第三方/可装包骨架，未进入主架构叙事  

问题不是“缺层”，而是 **同一可选能力有多套故事**（optional / extension / plugin / lab），用户与贡献者认知成本高，长期不利于应用中心与第三方生态。

备选过的模型：

| 方案 | 结论 |
|------|------|
| Plugin / Extension / Core / Foundation 四层 | 工程可画，但 Extension≠Plugin 是工程分类，非用户心智 |
| 微服务拆分 nasd | 家用 NAS 运维成本过高，否决为默认形态 |
| 仅加强 Core/Ext/Lab | 治理够用，产品与装包语言偏弱 |

---

## 2. 决策

采用 **Platform + Packages + Host SDK + Runtime** 作为**产品与演进主叙事**；工程治理标签继续沿用 Core / Package(system) / Lab。

```text
┌──────────────────────────────────────────────┐
│              Package Surface                 │  用户可见：启用 / 安装 / 卸载
│   system（官方）· community（第三方）· local  │
├──────────────────────────────────────────────┤
│              Platform Core                   │  卸不掉：五模块生命周期
│   identity · storage · network · sharing     │
│   · system（含薄 Package 宿主）               │
├──────────────────────────────────────────────┤
│              Host SDK（稳定契约）              │  套件唯一合法扩展面
├──────────────────────────────────────────────┤
│              Runtime Libraries               │  无业务生命周期的共享库
└──────────────────────────────────────────────┘
         Lab：源码温室 / 实验构建旁路（不进应用中心）
```

### 2.1 各块定义

| 概念 | 定义 | 现行映射 |
|------|------|----------|
| **Platform Core** | 进程主生命周期必需；不可装卸 | `identity` / `storage` / `network` / `sharing` / `system` |
| **Package Surface** | 一切“可关 / 可装”的能力统一入口 | `extensions/*`、optional 产品管理器、未来 community 插件 |
| **Host SDK** | 套件可依赖的**冻结**公开 API | 目标：`pkg/hostapi`（或等价）；过渡期可用受限 internal 门面 |
| **Runtime Libraries** | 配置、日志、容器契约、纯工具库 | `pkg/*`、`internal/arch`、`internal/config`、`internal/logging`、公共 middleware |
| **Lab** | 实验/未收编实现；默认构建与默认 `nasd` 不加载 | `internal/lab/*` |

### 2.2 信任等级（正交于“层”）

| Trust | 谁提供 | 权限 | 现行对应 |
|-------|--------|------|----------|
| `platform` | 编译进 Core | 全权、进模块图 | 五 Core 模块 |
| `system` | 官方 monorepo 套件 | 可调较深 Host API；默认可关 | Extension + optional 产品 |
| `community` | 第三方签名包 | 沙箱 + 仅 Host SDK | 目标态 Plugin |
| `local` | 开发者本机 | 最严 / 仅 dev | 调试安装 |
| `lab` | 仅源码 | 不进默认路径与应用中心 | `internal/lab/*` |

**原则**：用户只面对「平台 + 套件」；`system` / `community` 是信任维，不是两套生命周期。

### 2.3 依赖方向（硬约束）

```text
community/local package  →  Host SDK only
system package           →  Host SDK（+ 经审的深 API，逐步收敛）
Platform Core            →  Runtime
Runtime                  →  标准库 / 少量第三方（禁止依赖业务 Manager）
Lab                      →  不得被生产 web/default 路径 import（已有治理测试）
```

禁止：Core 依赖 Package；Runtime 依赖业务包；community 包 import 任意 `internal/*`。

### 2.4 Core 内部横切（非全局层）

Platform Core **内部**按存储系统惯例组织，不单独做成产品层：

| 面 | 职责 |
|----|------|
| **Data plane** | 实际 I/O：文件系统、协议数据路径 |
| **Control plane** | 卷/共享/用户/网络配置与生命周期 |
| **Management plane** | API、WebUI、健康、任务与告警编排 |

套件默认挂 Management/Control；直接介入 Data plane 仅限高信任 `system` 且需评审。

---

## 3. 配置兼容策略

**本 ADR 生效后，默认行为不变**：`modules.optional=false`、`modules.extensions=[]` 仍表示“仅 Core”。

### 3.1 阶段 0 — 仅文档（已完成）

- 对外/对内文档用 Platform + Packages 叙事解释现有开关。  
- 对照表：

| 现行配置 | 目标语义 |
|----------|----------|
| （无，五模块常驻） | Platform Core |
| `modules.optional: true` | 启用**官方推荐 system 套件集**（语法糖） |
| `modules.extensions: [name, …]` | 启用具名 **system** 套件 |
| 插件目录安装（未来/部分已有） | **community/local** 套件 |
| `internal/lab/*` | 非配置项；不进列表 |

### 3.2 阶段 1 — 语义别名（**已实现**）

类型配置与默认 YAML 已支持：

```yaml
# 与 modules.* 双读兼容（configs/default.yaml + internal/config）
packages:
  recommended_system: false   # ≡ modules.optional
  enabled: []                 # 与 modules.extensions 并集
```

实现要点：

- `Config.ResolvePackages()`：纯函数合并；`RecommendedSystem` = optional ∨ recommended_system；`Enabled` = 命名列表并集，且在 RecommendedSystem 时并入 `RecommendedSystemPackages`（非空官方集）。  
- 生产门闸使用 `OptionalProductsEnabled()` / `EnabledNamedPackages(known)`，不再单独只读旧字段。  
- 双源同时贡献时 `DualSource` + warn 日志；默认仍 Core-only。

兼容规则：

1. 若仅存在 `modules.*`：行为与 v3.24 一致（optional 仍开产品管理器；extensions 仍挂 HTTP 扩展）。  
2. 若仅存在 `packages.*`：按套件列表/推荐集启用。  
3. 若两者皆存在：**并集**启用；双源 warn（避免升级后功能静默消失）。  
4. 弃用周期至少 **两个小版本** 后再考虑只读 `modules.*`（Stage 3）。

### 3.3 阶段 2 — 统一运行时（**已实现初版**）

- 单一 Package Runtime（`internal/packageruntime`）：catalog 注册 → trust 校验 → Init → Start → HTTP mount（system）→ Stop。  
- Host SDK 初版：`pkg/hostapi`（`APIVersion=1.0.0`，`Host` / `Package` / `HTTPMounter` / `Trust*`）。  
- `modules.optional` / `packages.recommended_system` 仍经 Stage 1 解析展开推荐清单；`modules.extensions` ∪ `packages.enabled` 经 Runtime `Enable`。  
- 官方 HTTP 扩展（7）注册为 `TrustSystem` catalog；生产路径 `registerConfiguredExtensions` 走 Runtime。  
- **community** 包：可注册/生命周期加载，**Stage 2 不挂 HTTP**（需更高信任或后续沙箱策略）。  
- 状态面：`GET /api/v1/packages`（host API 版本、catalog、loaded、resolved、statuses）；保留 `GET /api/v1/extensions`。  
- 签名校验与完整 community 沙箱 **未**在本阶段实现（仍属后续增强）。

### 3.4 阶段 3 — 收敛（**已实现；兼容层保留**）

- 文档与 `configs/default.yaml` **主路径只推荐** `packages.*`。  
- `modules.optional` / `modules.extensions` 在类型注释与解析结果中标 **deprecated**；使用时 `PackageResolution.ModulesDeprecated` + `Warnings`，启动 log warn。  
- **不删除** YAML 字段：双读并集仍有效，避免升级静默丢功能。彻底移除另开 ADR。  
- 官方套件编目：`config.SystemPackageCatalog`（`recommended_product` + `http_extension`）；`RecommendedSystemPackages` / HTTP 扩展名从此派生。  
- 物理目录 **未** 搬家（docker/vm/photos/… 与 `internal/extensions/*` 路径保持）。

---

## 4. 与现行目录 / 治理的映射

| 目标概念 | 目录与机制 | 治理 |
|----------|------------|------|
| Platform Core | `internal/application` 模块图；实现分布于 storage/users/smb/nfs/… | Core 名表冻结为五；`governance_test` |
| System packages | `internal/extensions/*`；optional 管理器（docker/vm/photos/ai/…） | 新官方可选能力进 extensions 或编目套件，禁止伪 Core |
| Community packages | `plugins/*`；`internal/plugin` 为**宿主**（属 Core/Platform） | 禁止插件直依赖 internal 业务包 |
| Host SDK | `pkg/hostapi`（v1.0.0） | API 版本化；变更需兼容策略 |
| Runtime | `pkg/*`、`arch`、`config`、`logging`、公共 `api` 中间件 | 不放带产品 Start/Stop 的 Manager |
| Lab | `internal/lab/*` | 禁止生产 web→lab；未知 catalog → Lab |

**毕业路径**

```text
Lab ──评审──► system package（官方套件）
第三方原型 ──► local ──签名/评审──► community package
```

---

## 5. 非目标（本 ADR 不承诺）

- 不把默认表面改为“全家桶全开”。  
- 不将 Lab 暴露为用户功能档位或 `packages.enabled` 合法名。  
- 不强制物理目录立刻重命名为 `platform/` / `packages/`。  
- 不将 nasd 拆成微服务；重负载（如 AI 推理）仍可为 **sidecar 进程**，由 system 套件编排。  
- 不在本 ADR 内规定具体套件 ID 清单与 recommended 集合内容（实现阶段维护清单文件即可）。

---

## 6. 分阶段落地清单

| 阶段 | 产出 | 代码改动 | 默认行为 | 状态 |
|------|------|----------|----------|------|
| **0** 文档 | 本 ADR + ARCHITECTURE 链接 | 无 | 不变 | **done** |
| **1** 别名 | `packages` 配置与双读兼容；生产门闸接线 | 小 | 不变（仍默认 Core-only） | **done** |
| **2** 运行时 | 统一 Package Runtime；Host SDK 初版；`/api/v1/packages` | 中 | 不变（仅新增 API/目录） | **done** |
| **3** 收敛 | 弃用 `modules.*` 文档主路径；套件编目；弃用 warn | 中 | 默认不变；兼容层保留 | **done** |

每阶段约束：可构建、可测试、可回滚；删除兼容前完成客户端/文档迁移。

---

## 7. 后果

**正向**

- 产品语言与 DSM/fnOS 类“应用中心”对齐。  
- 消除 optional / extension / plugin 三轨长期分裂。  
- Core 保持瘦默认；生态边界清晰（Host SDK）。  

**代价**

- 需维护配置双读与弃用周期。  
- Host SDK 设计与版本纪律有一次性成本。  
- system 套件从“深层 internal 调用”迁到 SDK 是渐进债。  

**风险与缓解**

| 风险 | 缓解 |
|------|------|
| 双配置并集导致意外启用 | warn 日志 + 启动摘要打印已启用套件列表 |
| Host SDK 过早冻结 / 过窄 | 阶段 2 先覆盖扩展点（路由、配置、事件），深 API 白名单审批 |
| 目录与叙事不一致 | catalog / 套件清单单源；治理测试锁 Core 与 lab 边界 |

---

## 8. 决议摘要

1. **主叙事**：Platform Core + Package Surface + Host SDK + Runtime；Lab 旁路。  
2. **不新增**与 Package 并行的 Extension/Plugin 产品层；二者合并为 Packages + trust。  
3. **现行** `modules.optional` / `modules.extensions` 在迁移完成前保持有效，语义解释为 system 套件。  
4. **默认**仍为仅 Core，诚实交付面不回退。  
5. 实现按阶段 0→3；**阶段 0–3 已完成**；默认表面仍为仅 Core。  
6. 未来若 **移除** `modules.optional` / `modules.extensions` 字段，须另开 ADR 与破坏性发版说明（当前仅弃用 + 兼容）。
