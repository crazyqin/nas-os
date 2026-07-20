# 运维手册：套件 / 应用中心 / 启用状态

面向部署与排障。架构总览见 [STRUCTURE.md](STRUCTURE.md)。

---

## 1. 默认行为（Core-only）

| 项 | 默认 |
|----|------|
| `packages.recommended_system` | `false` |
| `packages.enabled` | `[]` |
| `packages.community_dir` | `""`（不扫描第三方） |
| `packages.legacy_so_plugins` | `false` |
| 进程能力 | 仅 Core：用户 / 存储 / 网络 / SMB·NFS / 系统 |

**不要**把「仓库里有很多代码」当成「默认都开了」。

---

## 2. 启用状态单一真相源（SSOT）

```text
data_dir/app-center-enabled.json     ← 权威（UI 启停后写入）
        ↓ 启动时
Package Runtime loaded               ← 运行时是否启用
        ↑
packages.enabled（yaml）             ← 仅「首次启动种子」
                                      （尚无 SSOT 文件时生效）
```

### 规则

1. **若已存在** `app-center-enabled.json` → 以该文件为准（手改 yaml 的 `packages.enabled` **不会**覆盖文件）。
2. **若文件不存在** → 用 yaml `packages.enabled` 作为种子，启动后会生成 SSOT 文件。
3. 应用中心 **启用/停用** 会：
   - 更新 Runtime loaded  
   - 重写 `app-center-enabled.json`  
   - 同步内存中的 `cfg.Packages.Enabled`  

### 文件示例

路径：`{paths.data_dir}/app-center-enabled.json`（默认 `/var/lib/nas-os/app-center-enabled.json`）

```json
{
  "enabled": [
    "docker",
    "netdiag",
    "com.example.hello-host"
  ]
}
```

### 运维操作建议

| 目标 | 做法 |
|------|------|
| 用 UI 管理 | 应用中心点启用/停用（推荐） |
| 用配置预置 | 首次部署写 yaml `packages.enabled`，启动一次生成 SSOT 后改文件或 UI |
| 清空所有可选套件 | 删 SSOT 或写成 `"enabled": []`，并 `packages.recommended_system: false`，重启 |
| 排查「为什么开了 yaml 没生效」 | 先看是否已有 SSOT 文件抢权威 |

---

## 3. 配置参考

```yaml
packages:
  # 八件产品套件（docker/vm/photos/ai/backup/cloudsync/downloader/cluster）
  # 不拉 perf/quota/s3 等 bulk 附属包
  recommended_system: false

  # 种子列表（无 SSOT 文件时）；也可写 HTTP 扩展 / 第三方 id
  enabled: []

  # 第三方包目录（子目录各含 manifest.json）
  community_dir: ""

  # 弃用：Go .so 插件宿主，默认关
  legacy_so_plugins: false
```

### 弃用项

- `modules.optional` / `modules.extensions`：仍双读兼容，启动会 warn；新配置勿用。  
- `modules.optional: true`：仍表示旧「全家桶 bulk」产品面（与 `recommended_system` 不同）。

---

## 4. 应用中心（用户可点）

| 项 | 值 |
|----|----|
| URL | `/webui/pages/app-center.html` 或 `/app-center` |
| 权限 | **管理员登录**（Bearer token） |
| 列表 API | `GET /api/v1/packages` |
| 启用 | `POST /api/v1/packages/:id/enable` |
| 停用 | `POST /api/v1/packages/:id/disable` |

### 套件类型

| kind / source | 含义 |
|---------------|------|
| `http_extension` / system | 官方 HTTP 扩展（voicehub、netdiag…） |
| `recommended_product` / product | 产品套件（docker、photos…） |
| community | 磁盘发现的第三方包 |

### 与「容器应用商店」区别

| 页面 | 用途 |
|------|------|
| **应用中心** | 系统套件 / 第三方 Host SDK 包 |
| **容器应用商店** `apps.html` | Docker Compose 模板（需 **docker** 产品已启用） |

---

## 5. 产品套件说明

| ID | 运行时启用 | 停用 |
|----|------------|------|
| docker | 构造 Manager + 注册路由 | 释放 Manager；路由 503 |
| photos / backup / vm / ai / cloudsync / downloader | 同上（尽量构造） | 释放；路由 503 |
| **cluster** | 见下一节 | 见下一节 |

`packages.recommended_system: true` = 启动时启用上表 **八件**（不含 bulk 附属）。

---

## 6. Cluster 专项

Cluster 是 **进程级** 服务（在 `application` 组合根初始化），与纯 HTTP 扩展不同。

| 时机 | 行为 |
|------|------|
| **启动前** SSOT/`packages.enabled` 含 `cluster`（或 `recommended_system`） | 启动时 `InitializeCluster` |
| **运行中** 应用中心启用 `cluster` | 若尚未初始化则尝试初始化；成功则本进程内可用；失败则记日志并标记需检查 |
| **运行中** 停用 `cluster` | 尝试 `ShutdownCluster` 并清空句柄；SSOT 去掉 `cluster`，重启后不会再起 |
| UI 标记 | `requires_restart`：集群拓扑/配置变更仍建议重启核对 |

**建议：** 生产变更 cluster 后执行一次进程重启，并检查日志中的 `集群` / `cluster` 行。

---

## 7. 停用语义

| 类型 | 停用后 |
|------|--------|
| HTTP 扩展 | Runtime 卸载；**中间件返回 503**（路由树可能仍在，请求不可用） |
| 产品套件 | Runtime 卸载 + **释放 Manager**（尽量 Close）；API 503 |
| 第三方 community | Runtime Stop；Host SDK 清理 marker |
| 再启用 | 重新构造 Manager（如需）并恢复可用（不因 gin 重复注册 panic） |

---

## 8. 第三方包

```bash
mkdir -p /var/lib/nas-os/community-packages
cp -a examples/community-packages/hello-host \
  /var/lib/nas-os/community-packages/hello-host
```

```yaml
packages:
  community_dir: /var/lib/nas-os/community-packages
  # 或在应用中心点启用 com.example.hello-host
```

要求：`manifest.json` 的 `trust` 为 `local`/`community`，禁止 `http.admin` / `trust: system`。  
只允许依赖公共 **`pkg/hostapi`**。

---

## 9. 排障清单

| 现象 | 检查 |
|------|------|
| 应用中心空白 / 失败 | 是否管理员登录；浏览器 Network 是否 401 |
| 改了 yaml 不生效 | 是否已有 `app-center-enabled.json` |
| 第三方不出现 | `community_dir`、子目录 `manifest.json`、日志 discovery |
| 启用 docker 仍无容器 API | 日志是否构造失败；docker 守护进程是否可用 |
| 停用后接口仍 200 | 是否打到未挂 `requirePackageActive` 的旧路径；应 503 |
| cluster 启了没作用 | 日志 InitializeCluster；是否需重启；DataDir 是否可写 |

### 有用日志关键字

- `package enabled` / `package disabled`  
- `product … manager constructed` / `released`  
- `community packages discovered`  
- `modules.optional / modules.extensions are deprecated`  
- `集群` / `cluster`  

### 只读检查 API（需管理员）

```http
GET /api/v1/packages
```

关注：`items[].loaded`、`runtime_enabled`、`enablement_source`、`community_discovered`。

---

## 10. 相关路径速查

| 路径 | 用途 |
|------|------|
| `configs/default.yaml` | 默认配置模板 |
| `{data_dir}/app-center-enabled.json` | 启用 SSOT |
| `webui/pages/app-center.html` | 应用中心 UI |
| `examples/community-packages/hello-host/` | 第三方示例 |
| `docs/STRUCTURE.md` | 架构结构图 |
| `docs/ARCHITECTURE.md` | 进程与 API 边界 |
