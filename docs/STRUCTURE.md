# NAS-OS 项目结构图（贡献者入口）

**版本**: v3.24.3 · **更新日期**: 2026-07-20  
**地位**: 仓库内 **唯一** 全景结构说明。进程细节见 [ARCHITECTURE.md](ARCHITECTURE.md)；演进决策见 [ADR-0001](adr/0001-platform-packages-host-sdk.md)。

---

## 1. 一句话模型

```text
┌──────────────────────────────────────────────────────────┐
│  Package Surface（可选）                                  │
│  packages.recommended_system / packages.enabled          │
│  官方 HTTP 扩展 · 推荐产品 · community（community_dir）     │
├──────────────────────────────────────────────────────────┤
│  Platform Core（必选 · 默认唯一启用）                       │
│  identity · storage · network · sharing · system         │
├──────────────────────────────────────────────────────────┤
│  Host SDK + Package Runtime                              │
│  pkg/hostapi · internal/packageruntime                   │
├──────────────────────────────────────────────────────────┤
│  Runtime Libraries（无业务生命周期）                       │
│  pkg/* · internal/config · arch · logging · 公共 middleware │
└──────────────────────────────────────────────────────────┘
         Lab（internal/lab/*）— 源码温室，默认 nasd 不加载
```

**默认交付**：仅 Platform Core。`packages.*` 全关。  
**弃用兼容**：`modules.optional` / `modules.extensions` 仍双读，**不是**第二套架构。

---

## 2. 仓库根目录（你会碰到的）

| 路径 | 角色 |
|------|------|
| `cmd/nasd` | **生产守护进程入口**（唯一组合路径的入口） |
| `cmd/nasctl` | 运维 CLI |
| `cmd/backup` | 备份相关小工具 |
| `internal/application` | 组合根：构造依赖、注册 Core 模块、启停 |
| `internal/config` | 强类型配置、`packages` 解析、`SystemPackageCatalog` |
| `internal/web` | HTTP 入口、WebUI 门闸、HTTP 扩展经 Runtime 启用 |
| `internal/extensions/*` | 官方 HTTP 扩展实现（system packages） |
| `internal/lab/*` | 实验/未收编代码；生产 web 禁止 import |
| `internal/*` 其他 | Core 支撑与 recommended 产品实现（见 allowlist） |
| `pkg/hostapi` | Host SDK（套件可依赖的稳定契约） |
| `pkg/*` | 可复用库（存储/安全工具等），非业务 Manager |
| `plugins/*` | **遗留** `.so` 示例（已弃用）；新第三方用 `examples/community-packages` + `community_dir` |
| `webui/` | **主产品 UI**（静态 HTML/JS）；**应用中心** `pages/app-center.html`（Core 面） |
| `web/` | 实验前端（非默认交付主线） |
| `configs/default.yaml` | 默认配置（packages 关） |
| `docker-compose.yml` + `Dockerfile` | 主部署路径 |
| `docs/STRUCTURE.md` | **本文件** — 结构全景 |
| `docs/ARCHITECTURE.md` | 生命周期、API 边界、健康探针 |
| `docs/adr/0001-*.md` | 架构演进 ADR |

---

## 3. 进程与代码路径

```text
cmd/nasd
  → internal/config.Load
  → internal/application.New / Run
       → Core 五模块生命周期（arch.Container）
       → internal/web.Server
            → packages 解析 → OptionalProductsEnabled（产品管理器门闸）
            → Package Runtime + registerSystemPackageCatalog
            → packages.enabled ∩ catalog → Enable HTTP 扩展
```

| 层 | 代码真相源 |
|----|------------|
| Core 模块名 | `internal/application`（仅 identity/storage/network/sharing/system） |
| 套件编目 | `config.SystemPackageCatalog` |
| 推荐产品 ID | `config.RecommendedSystemPackageIDs()` / `KindRecommendedProduct` |
| HTTP 扩展 ID | `config.HTTPExtensionPackageIDs()` / `KindHTTPExtension` |
| 启用解析 | `config.Config.ResolvePackages()` |
| 扩展挂载 | `internal/web` → `packageruntime.Runtime.Enable` |
| 顶层包冻结 | `internal/application/toplevel_allowlist.txt` |

---

## 4. 如何启用能力（packages-first）

```yaml
# 主配置面（唯一推荐写法）
packages:
  recommended_system: false   # true → 展开推荐产品集（docker/vm/photos/…）
  enabled: []                 # 官方 HTTP 扩展 和/或 第三方包 ID
  community_dir: ""           # 第三方包扫描根目录；空 = 不发现、不加载
```

| 开关 | 效果 |
|------|------|
| 默认 | 仅 Core |
| `recommended_system: true` | 构造 Docker/VM/Photos/AI/备份等 **产品管理器**（非自动挂全部 HTTP 扩展） |
| `enabled: [name]` | 加载 **官方 HTTP 扩展**（catalog）和/或 **已发现的第三方包** |
| `community_dir: /path` | 扫描 `*/manifest.json` 注册 community/local 包（**不**自动 Start） |
| `modules.optional` / `modules.extensions` | **已弃用**；双读并入上表，启动 warn |

完整官方 ID：`GET /api/v1/packages` 或 `internal/config/system_catalog.go`。

### 4.1 第三方插件（community / local）

属于 **Package Surface**，不是第二套并行插件体系。

| 规则 | 说明 |
|------|------|
| 发现 | `packages.community_dir` 下每个子目录的 `manifest.json` |
| 信任 | 仅 `community` / `local`；禁止在 manifest 中声明 `system` |
| 能力 | 默认 `host.sdk`；**禁止** `http.admin`（系统路由挂载仅官方 system） |
| 宿主 API | 仅公共 **`pkg/hostapi`**（Host SDK）；勿 import `internal/*` |
| 启用 | 必须同时出现在 `packages.enabled`；仅发现不会 Start |
| 生命周期 | 统一 `internal/packageruntime`（Init/Start/Stop） |
| 示例 | `examples/community-packages/hello-host/` |

```yaml
packages:
  community_dir: /var/lib/nas-os/community-packages
  enabled: [com.example.hello-host]
```

### 4.2 应用中心（用户可点 · Package Surface 唯一 UI）

| 项 | 说明 |
|----|------|
| 页面 | `/webui/pages/app-center.html` 或 `/app-center`（**Core allowlist**） |
| 导航 | 主侧栏「应用中心」 |
| 列表 | `GET /api/v1/packages` → `items[]`：`http_extension` / `recommended_product` / `community` |
| 启用/停用 | `POST .../packages/:id/enable|disable` → 统一 Runtime |
| 启用真相源 | `packages.enabled` ∪ `data_dir/app-center-enabled.json` → **Runtime loaded** |
| HTTP 停用 | 路由中间件 `requirePackageActive` → **503**（非仅 loaded=false） |
| 产品套件 | `docker` 等 `recommended_product` 与扩展同一列表可点 |

默认仍 Core-only。

**勿与下列页面混淆：**

| 页面 | 角色 |
|------|------|
| **应用中心** `app-center.html` | 系统套件 / 第三方 Host SDK 包 |
| **容器应用商店** `apps.html` | Docker/Compose 模板（需 docker 产品启用） |
| **插件市场** `plugins.html` | **已废弃** → 跳转应用中心（原 mock 列表已移除） |

### 4.3 遗留 `.so` 插件宿主（弃用）

- 代码：`internal/plugin` + `plugins/*` 示例  
- 默认 **不** 构造：需 `packages.legacy_so_plugins: true` 且产品面已开  
- **不是** 第三方生态主路径；主路径 = `community_dir` + Host SDK + Runtime

---

## 5. Core 与「支撑包」

- **Core 生命周期名**只有五个（不可扩）。  
- `internal/auth`、`internal/storage`、`internal/smb` 等是 Core 的实现/支撑，**不是**可装卸套件名。  
- `recommended_product`（docker/vm/…）默认不构造；打开 recommended 后门闸。  
- 新业务：可选产品 → `internal/extensions` 或 catalog；实验 → `internal/lab`；勿新增无 allowlist 的顶层包。

---

## 6. Lab 旁路

- 目录：`internal/lab/*`  
- 默认 `nasd` / 生产 `internal/web` **不** import、不构造  
- **不能** 写进 `packages.enabled` 当正式产品  
- 毕业：Lab → 官方 system package（编目 + 评审）

---

## 7. 工程治理标签（与产品层的关系）

| 产品层（本图） | 工程标签（application catalog） |
|----------------|----------------------------------|
| Platform Core | `ModuleTierCore` |
| Package Surface（官方） | `ModuleTierExtension` 等 |
| Lab | `ModuleTierLab` |
| Host SDK / Runtime libs | 非 Module 生命周期 |

贡献者对外沟通用 **本结构图**；改生命周期/治理测试时对照 Core/Extension/Lab 标签。

---

## 8. 一致性约定（自动化）

以下由测试锁定，修改编目或 Runtime 注册时必须同步：

1. 默认 `ResolvePackages()` → `RecommendedSystem=false` 且 `Enabled` 为空。  
2. `HTTPExtensionPackageIDs()` ≡ Package Runtime 注册的 HTTP catalog ID 集合。  
3. `KnownExtensionNames` 派生自 catalog，不得手写第二份名单。  

详见 `internal/config/*_test.go`、`internal/web/structure_consistency_test.go`。
