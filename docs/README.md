# NAS-OS 文档索引

按读者角色阅读，避免在多份文档间重复劳动。

---

## 快速入口

| 你想… | 读这个 |
|--------|--------|
| 了解仓库怎么分层、默认开什么 | [STRUCTURE.md](STRUCTURE.md) |
| 部署/启停套件/排障 | [ops-packages.md](ops-packages.md) |
| 进程生命周期、API 安全边界 | [ARCHITECTURE.md](ARCHITECTURE.md) |
| 架构决策历史（Core/Packages/Host SDK） | [adr/0001-platform-packages-host-sdk.md](adr/0001-platform-packages-host-sdk.md) |
| HTTP API 示例 | [api/](api/) |
| 竞品对照（能力面，非默认交付） | [competitive-analysis.md](competitive-analysis.md) |
| 资源/包数量统计 | [resource-stats.md](resource-stats.md) |
| 项目介绍与默认交付面 | [../README.md](../README.md) |
| 版本变更 | [../CHANGELOG.md](../CHANGELOG.md) |

---

## 文档地图

```text
docs/
├── README.md                 ← 本索引
├── STRUCTURE.md              ← 结构全景（贡献者首选）
├── ops-packages.md           ← 运维：SSOT、应用中心、cluster、停用
├── ARCHITECTURE.md           ← 进程组合、模块契约、路由安全
├── competitive-analysis.md   ← 竞品能力对照（非默认全开）
├── resource-stats.md         ← 包/模块数量与治理统计
├── adr/
│   └── 0001-platform-packages-host-sdk.md
├── api/                      ← 分域 API 说明与示例
│   ├── examples.md
│   ├── storage-api.md
│   ├── user-api.md
│   └── …
└── swagger/                  ← OpenAPI 生成物
```

---

## 阅读顺序建议

### 新贡献者
1. [../README.md](../README.md) — 默认交付面  
2. [STRUCTURE.md](STRUCTURE.md) — 分层与目录  
3. [ARCHITECTURE.md](ARCHITECTURE.md) — 改代码前的约束  

### 运维 / 部署
1. [ops-packages.md](ops-packages.md)  
2. [STRUCTURE.md](STRUCTURE.md) §4 套件启用  
3. `configs/default.yaml`  

### 第三方包开发
1. [STRUCTURE.md](STRUCTURE.md) §4.1  
2. [ops-packages.md](ops-packages.md) §8  
3. `examples/community-packages/hello-host/`  
4. `pkg/hostapi`  

---

## 关键约定（全文档一致）

| 概念 | 含义 |
|------|------|
| **Core** | 默认始终运行：identity / storage / network / sharing / system |
| **Package Surface** | 可选套件：官方 HTTP 扩展、推荐产品、第三方 community |
| **SSOT** | 启用列表权威文件：`{data_dir}/app-center-enabled.json` |
| **应用中心** | Core UI：`/webui/pages/app-center.html` |
| **容器应用商店** | Docker 模板 UI（`apps.html`），不是系统套件中心 |
| **Lab** | `internal/lab/*`，默认不进入 nasd 热路径 |
| **路由卸载** | 套件停用后请求 **404**（挂载标志关闭）；再启用重新挂载 |

---

## 维护说明

- 改架构/套件行为时：先更新 **STRUCTURE** + **ops-packages**，再改 ARCHITECTURE/ADR。  
- 新增用户可见能力时：README「默认交付面」必须写清是否默认开启。  
- OpenAPI（`swagger/`）与实现不一致时，以代码与 ops/STRUCTURE 为准并开 issue 同步生成。  
