# NAS-OS 项目里程碑

## 项目概述
- **目标**: 基于 Go 的家用 NAS 系统，支持 btrfs 存储管理、SMB/NFS 共享、Web 管理界面
- **位置**: ~/clawd/nas-os
- **启动日期**: 2026-03-10
- **目标完成日期**: 2026-06-30

---

## 里程碑规划

### 🎯 里程碑 1: 核心存储功能 (M1) ✅
**时间**: 2026-03-10 ~ 2026-03-10  
**负责人**: 兵部 (软件工程)
**状态**: 已完成

#### 任务清单
- [x] 项目骨架搭建
- [x] btrfs 基础管理 (卷创建/删除/列表)
- [x] 子卷管理 (创建/删除/挂载/列出)
- [x] 快照功能 (创建/恢复/删除/列出)
- [x] RAID 配置支持 (RAID0/1/5/6/10)
- [x] 存储池扩容/缩容
- [x] 数据平衡 (balance)
- [x] 数据校验 (scrub)
- [x] 完整测试用例

#### 交付物
- `internal/storage/manager.go` - 存储管理核心 ✅
- `pkg/btrfs/btrfs.go` - btrfs 命令封装 ✅
- `pkg/btrfs/btrfs_test.go` - btrfs 测试用例 ✅
- `internal/storage/manager_test.go` - 存储管理测试 ✅

---

### 🎯 里程碑 2: Web 管理界面 (M2)
**时间**: 2026-03-20 ~ 2026-04-20  
**负责人**: 工部 (DevOps) + 礼部 (UI 设计)

#### 任务清单
- [x] Web 框架搭建 (Gin/Echo)
- [ ] 存储管理页面 (卷/子卷/快照)
- [ ] 用户登录/认证
- [ ] 系统监控面板
- [ ] 文件浏览器
- [ ] 设置页面
- [ ] API 文档 (Swagger)

#### 交付物
- `internal/web/server.go` - Web 服务
- `internal/web/handlers/` - 路由处理器
- `webui/` - 前端静态资源
- `docs/api.yaml` - API 文档

---

### 🎯 里程碑 3: 文件共享服务 (M3)
**时间**: 2026-04-15 ~ 2026-05-15  
**负责人**: 兵部 (软件工程)

#### 任务清单
- [ ] SMB/CIFS 共享实现
- [ ] NFS 共享实现
- [ ] 共享权限配置
- [ ] 访客访问控制
- [ ] 共享连接监控

#### 交付物
- `internal/smb/server.go` - SMB 服务
- `internal/nfs/server.go` - NFS 服务
- `internal/shares/manager.go` - 共享管理

---

### 🎯 里程碑 4: 用户权限系统 (M4)
**时间**: 2026-05-01 ~ 2026-05-31  
**负责人**: 刑部 (安全合规) + 兵部

#### 任务清单
- [ ] 用户 CRUD 操作
- [ ] 用户组管理
- [ ] 权限模型 (RBAC)
- [ ] 共享访问控制列表
- [ ] 密码策略
- [ ] 登录审计

#### 交付物
- `internal/users/manager.go` - 用户管理
- `internal/auth/middleware.go` - 认证中间件
- `internal/audit/logger.go` - 审计日志

---

### 🎯 里程碑 5: 监控告警系统 (M5)
**时间**: 2026-05-15 ~ 2026-06-15  
**负责人**: 工部 (DevOps)

#### 任务清单
- [ ] 磁盘健康监控 (SMART)
- [ ] 空间使用告警
- [ ] 系统资源监控 (CPU/内存/网络)
- [ ] 告警通知 (邮件/微信)
- [ ] 日志收集与查询

#### 交付物
- `internal/monitor/health.go` - 健康检查
- `internal/monitor/alerts.go` - 告警管理
- `internal/log/collector.go` - 日志收集

---

### 🎯 里程碑 6: Docker 集成 (M6) ✅
**时间**: 2026-03-11 ~ 2026-03-11  
**负责人**: 工部 (DevOps)
**状态**: 已完成

#### 任务清单
- [x] Docker 守护进程集成
- [x] 容器管理界面
- [x] 应用商店 (常用 NAS 应用)
- [x] 容器网络配置
- [x] 持久化存储映射
- [x] Docker Compose 模板管理

#### 交付物
- `internal/docker/manager.go` - Docker 管理器 ✅
- `internal/docker/handlers.go` - 容器 API ✅
- `internal/docker/appstore.go` - 应用商店核心 ✅
- `internal/docker/app_handlers.go` - 应用 API ✅
- `webui/pages/apps.html` - 应用管理界面 ✅
- `docs/app-store.md` - 应用商店文档 ✅

#### 预置应用 (12款)
- Nextcloud (私有云存储)
- Jellyfin (媒体服务器)
- Home Assistant (智能家居)
- Pi-hole (广告拦截)
- Transmission (BT 下载)
- Syncthing (文件同步)
- Gitea (Git 仓库)
- Vaultwarden (密码管理)
- Immich (照片备份)
- Nginx Proxy Manager (反向代理)
- Portainer (Docker 管理)

---

## 六部任务分配

| 部门 | 职责 | 主要负责人 |
|------|------|------------|
| **兵部** | 核心功能开发 (存储/共享/权限) | 软件工程团队 |
| **户部** | 项目预算、资源采购 | 财务团队 |
| **礼部** | UI/UX 设计、文档撰写 | 内容创作团队 |
| **工部** | DevOps、部署、监控、Docker | 运维团队 |
| **吏部** | 项目管理、进度跟踪 | 本项目团队 |
| **刑部** | 安全审计、合规检查 | 法务团队 |

---

## 当前状态 (2026-03-11)

### 已完成
- ✅ 项目初始化
- ✅ Go 模块配置
- ✅ 基础目录结构
- ✅ README 文档
- ✅ btrfs 存储管理框架
- ✅ Web 服务框架
- ✅ **M1 核心存储功能 (完整实现)**
  - 卷管理 (创建/删除/列表/挂载)
  - 子卷管理 (创建/删除/挂载/列出/默认子卷)
  - 快照功能 (创建/恢复/删除/列出/回滚)
  - RAID 配置 (single/raid0/raid1/raid5/raid6/raid10)
  - 设备管理 (添加/移除/统计)
  - 维护操作 (balance/scrub)
  - 完整测试用例 (60+ 测试)
- ✅ **M6 Docker 集成 (应用商店)**
  - Docker 容器管理 (创建/启动/停止/删除/重启)
  - 镜像管理 (列表/拉取/删除)
  - 应用商店 API (列出/安装/卸载/启动/停止/重启/更新)
  - Docker Compose 模板管理
  - 12款预置应用模板
  - 应用商店 UI 界面

### 进行中
- 🔄 M2 Web 管理界面 (部分完成)
- 🔄 M3 文件服务 (SMB/NFS 共享配置后端)

### 待开始
- ⏳ 用户权限系统
- ⏳ 监控告警

---

## 风险与依赖

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| btrfs 兼容性问题 | 高 | 提前测试主流硬件 |
| SMB 性能瓶颈 | 中 | 使用成熟库 (smb2) |
| 权限模型复杂 | 中 | 参考成熟方案 (Linux ACL) |
| 前端开发人力 | 低 | 使用现成 UI 框架 |

---

## 沟通机制

- **每日站会**: 吏部汇总进度 (Discord)
- **周报**: 每周日更新里程碑状态
- **代码审查**: 兵部负责 PR 审核
- **文档更新**: 礼部维护文档

---

*最后更新: 2026-03-10*
