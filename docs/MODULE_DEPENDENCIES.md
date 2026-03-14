# NAS-OS 模块依赖分析报告

**版本**: v2.33.0
**日期**: 2026-03-15
**负责人**: 吏部

---

## 目录结构概览

`internal/` 目录包含 **55 个模块**，按功能分类如下：

### 核心存储模块
| 模块 | 功能 | 文件数 |
|------|------|--------|
| `storage` | BTRFS 存储管理 | 5+ |
| `snapshot` | 快照管理 | 3+ |
| `tiering` | 存储分层 | 3+ |
| `compress` | 压缩存储 | 3+ |
| `dedup` | 去重功能 | 3+ |
| `replication` | 存储复制 | 3+ |
| `backup` | 备份恢复 | 5+ |
| `trash` | 回收站 | 3+ |

### 文件共享模块
| 模块 | 功能 | 文件数 |
|------|------|--------|
| `smb` | SMB/CIFS 共享 | 5+ |
| `nfs` | NFS 导出 | 5+ |
| `webdav` | WebDAV 服务 | 3+ |
| `ftp` | FTP 服务 | 3+ |
| `sftp` | SFTP 服务 | 3+ |
| `iscsi` | iSCSI 目标 | 3+ |
| `shares` | 共享管理 | 3+ |

### 容器与虚拟化
| 模块 | 功能 | 文件数 |
|------|------|--------|
| `docker` | Docker 管理 + 应用商店 | 15+ |
| `container` | 容器操作（compose/image/network） | 6+ |
| `vm` | 虚拟机管理 | 5+ |

### 系统服务模块
| 模块 | 功能 | 文件数 |
|------|------|--------|
| `monitor` | 系统监控 | 5+ |
| `health` | 健康检查 | 3+ |
| `logging` | 日志系统 | 3+ |
| `network` | 网络管理 | 5+ |
| `system` | 系统管理 | 3+ |
| `service` | 服务管理 | 3+ |
| `notify` | 通知服务 | 3+ |

### 用户与权限
| 模块 | 功能 | 文件数 |
|------|------|--------|
| `users` | 用户管理 | 5+ |
| `auth` | 认证授权 | 5+ |
| `ldap` | LDAP 集成 | 3+ |
| `security` | 安全模块 | 5+ |
| `quota` | 配额管理 | 5+ |

### 媒体与内容
| 模块 | 功能 | 文件数 |
|------|------|--------|
| `media` | 媒体服务 | 5+ |
| `photos` | 照片管理 | 5+ |
| `ai_classify` | AI 分类 | 3+ |
| `search` | 智能搜索 | 3+ |
| `files` | 文件管理 | 3+ |
| `tags` | 标签系统 | 3+ |

### 集群与高可用
| 模块 | 功能 | 文件数 |
|------|------|--------|
| `cluster` | 集群管理 | 8+ |
| `cache` | 缓存系统 | 3+ |
| `transfer` | 数据传输 | 3+ |

### 自动化与调度
| 模块 | 功能 | 文件数 |
|------|------|--------|
| `automation` | 自动化引擎 | 10+ |
| `prediction` | 预测分析 | 3+ |

### 云与同步
| 模块 | 功能 | 文件数 |
|------|------|--------|
| `cloudsync` | 云同步 | 3+ |
| `downloader` | 下载器 | 3+ |

### 其他模块
| 模块 | 功能 | 文件数 |
|------|------|--------|
| `api` | API 版本管理 | 5+ |
| `web` | Web 服务 | 5+ |
| `database` | 数据库 | 3+ |
| `plugin` | 插件系统 | 5+ |
| `reports` | 报告系统 | 5+ |
| `office` | Office 集成 | 3+ |
| `optimizer` | 优化器 | 3+ |

---

## 模块依赖分析

### 高频依赖模块（被其他模块引用最多）

| 模块 | 被引用次数 | 说明 |
|------|------------|------|
| `api` | 14 | API 响应格式、版本管理 |
| `storage` | 4 | 存储核心服务 |
| `users` | 3 | 用户认证/授权 |
| `logging` | 3 | 日志服务 |
| `automation/*` | 5 | 自动化触发器/引擎 |

### 模块依赖关系

```
api (核心)
├── storage → api
├── smb → api, storage
├── nfs → api, storage
├── docker → api
├── cluster → api, storage
├── users → api
├── auth → api, users
├── monitor → api, logging
├── backup → api, storage
├── quota → api, storage, users
└── ... (大部分模块依赖 api)
```

---

## 潜在问题与建议

### 1. 重复/相似模块

| 模块对 | 问题 | 建议 |
|--------|------|------|
| `perf/` vs `performance/` | 功能重叠（性能监控） | 合并为 `performance/` |
| `container/` vs `docker/` | 容器管理分散 | `container/` 作为基础，`docker/` 高级功能 |
| `version/` vs `versioning/` | 不同用途（版本号 vs API 版本） | 保持分离，添加注释说明 |

### 2. 依赖优化建议

1. **减少对 `api` 模块的直接依赖**
   - 考虑将通用响应类型移到 `pkg/` 或独立包

2. **避免循环依赖**
   - 使用接口解耦
   - 依赖注入模式

3. **模块分组**
   - 考虑按功能域创建子目录
   - 如 `internal/storage/...`, `internal/share/...`

---

## 模块统计

| 分类 | 模块数 | 说明 |
|------|--------|------|
| 存储相关 | 8 | storage, snapshot, tiering, compress, dedup, replication, backup, trash |
| 共享服务 | 7 | smb, nfs, webdav, ftp, sftp, iscsi, shares |
| 容器虚拟 | 3 | docker, container, vm |
| 系统服务 | 7 | monitor, health, logging, network, system, service, notify |
| 用户权限 | 5 | users, auth, ldap, security, quota |
| 媒体内容 | 6 | media, photos, ai_classify, search, files, tags |
| 集群可用 | 3 | cluster, cache, transfer |
| 自动化 | 2 | automation, prediction |
| 云同步 | 2 | cloudsync, downloader |
| 其他 | 12 | api, web, database, plugin, reports, office, optimizer, perf, performance, prediction, version, versioning |

**总计**: 55 个模块

---

## 下一步行动

1. **短期** (v2.33.x)
   - 合并 `perf/` 到 `performance/`
   - 更新相关导入路径
   - 添加模块 README

2. **中期** (v2.34.0)
   - 模块分组重构
   - 依赖注入改造
   - 接口抽象优化

3. **长期** (v2.35+)
   - 插件化架构
   - 模块热加载
   - 微服务拆分准备

---

*报告生成: 2026-03-15*
*负责人: 吏部*