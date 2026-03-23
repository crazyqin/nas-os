# NAS-OS 🖥️

**中文** | [English](docs/README_EN.md)

基于 Go 的家用 NAS 系统，支持 btrfs 存储管理、SMB/NFS 共享、Web 管理界面。

>  **最新版本**: v2.253.280 Stable (2026-03-24)
> **CI/CD**: [![CI/CD](https://github.com/crazyqin/nas-os/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/crazyqin/nas-os/actions)
> **Docker**: [![Docker](https://img.shields.io/docker/v/ghcr.io/crazyqin/nas-os/v2.253.280?label=docker)](https://github.com/crazyqin/nas-os/pkgs/container/nas-os)

## 特性

### 核心功能 ✅

| 模块 | 说明 | 状态 |
|------|------|------|
| 💾 btrfs 存储 | 卷/子卷/快照/RAID | ✅ 完成 |
| 🌐 Web 界面 | 响应式设计/移动端 | ✅ 完成 |
| 📁 文件共享 | SMB/NFS/RBAC | ✅ 完成 |
| 👥 用户权限 | RBAC/MFA/审计 | ✅ 完成 |
| 📊 监控告警 | 实时指标/多通道通知 | ✅ 完成 |
| 🔒 安全认证 | JWT/RBAC/加密 | ✅ 完成 |
| 🐳 Docker | 多架构镜像 | ✅ 完成 |
| ⚡ 性能优化 | LRU 缓存/GC 调优/工作池 | ✅ 完成 |
| 🛡️ 集群支持 | 多节点/负载均衡 | ✅ 完成 |

### 扩展功能 ✅

| 模块 | 说明 | 状态 |
|------|------|------|
| 📦 容器管理 | Docker 容器/镜像/网络/卷管理 | ✅ 完成 |
| 🖥️ 虚拟机管理 | VM 创建/ISO 挂载/快照 | ✅ 完成 |
| 🗂️ 存储分层 | 热/冷数据分层/SSD 缓存/云归档 | ✅ 完成 |
| 🗜️ 压缩存储 | 文件级/块级压缩/透明压缩 | ✅ 完成 |
| 🔄 存储复制 | 跨节点数据同步/灾备 | ✅ 完成 |
| 📸 快照策略 | 定时快照/保留策略/自动清理 | ✅ 完成 |
| 🎯 iSCSI 目标 | Target/LUN 管理/CHAP 认证 | ✅ 完成 |
| 📁 WebDAV | 完整 WebDAV 协议支持 | ✅ 完成 |
| 📡 FTP/SFTP | 被动/主动模式/SSH 密钥认证 | ✅ 完成 |
| 📊 配额管理 | 用户/组/目录三级配额 | ✅ 完成 |
| 🗑️ 回收站 | 安全删除/恢复/自动清理 | ✅ 完成 |
| 🤖 AI 分类 | 照片/文件智能分类 | ✅ 完成 |
| 📜 版本控制 | 文件历史版本/一键还原 | ✅ 完成 |
| ☁️ 云同步 | 多云存储/双向同步 | ✅ 完成 |
| 🔄 数据去重 | 文件级/块级去重 | ✅ 完成 |
| 🔍 智能搜索 | 语义搜索/标签搜索 | ✅ 完成 |
| 🏷️ 文件标签 | 标签分类/批量操作 | ✅ 完成 |
| 📈 预测分析 | 磁盘健康预测/容量趋势 | ✅ 完成 |
| 🔗 LDAP/AD | 企业目录集成/统一认证 | ✅ 完成 |
| 📋 自动化引擎 | 工作流/定时任务/触发器 | ✅ 完成 |
| 📰 下载器 | Transmission/qBittorrent 集成 | ✅ 完成 |
| 🎬 媒体服务 | HLS/DASH 流媒体/转码/字幕 | ✅ 完成 |
| 🖼️ 照片管理 | 相册/AI 分析/缩略图 | ✅ 完成 |
| 📝 在线文档 | OnlyOffice 集成/协作编辑 | ✅ 完成 |
| 🔧 网络诊断 | Ping/Traceroute/DNS/端口扫描 | ✅ 完成 |
| 🛡️ 安全增强 | 限流/MFA/密码策略/会话管理 | ✅ 完成 |
| 💾 智能备份 | 增量备份/多压缩算法/加密/版本管理 | ✅ 完成 |

## 快速开始

### 方式一：下载二进制文件 (推荐)

```bash
# 下载 (根据你的架构选择)
# AMD64 (x86_64)
wget https://github.com/crazyqin/nas-os/releases/download/v2.253.280/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# ARM64 (Orange Pi 5, Raspberry Pi 4/5)
wget https://github.com/crazyqin/nas-os/releases/download/v2.253.280/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd

# ARMv7 (Raspberry Pi 3, 旧款 ARM)
wget https://github.com/crazyqin/nas-os/releases/download/v2.253.280/nasd-linux-armv7

chmod +x nasd-linux-armv7
sudo mv nasd-linux-armv7 /usr/local/bin/nasd

# 验证安装
nasd --version
```

### 方式二：Docker 部署

```bash
# 拉取镜像
docker pull ghcr.io/crazyqin/nas-os:v2.253.280


# 运行容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.253.280


# 查看日志
docker logs -f nasd
```

### 方式三：源码编译

#### 依赖

```bash
# 安装 Go 1.26.1+
# 安装 btrfs 工具
sudo apt install btrfs-progs

# 安装 Samba（如需 SMB 共享）
sudo apt install samba

# 安装 NFS（如需 NFS 共享）
sudo apt install nfs-kernel-server
```

#### 构建

```bash
cd nas-os
go mod tidy
go build -o nasd ./cmd/nasd
go build -o nasctl ./cmd/nasctl
```

### 运行

```bash
# 需要 root 权限（访问磁盘设备）
sudo nasd
```

访问 http://localhost:8080

**默认登录凭据**：
- 用户名：`admin`
- 密码：`admin123`

⚠️ **首次登录后请立即修改默认密码！**

## API 接口

### 存储管理
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/volumes | 获取卷列表 |
| POST | /api/v1/volumes | 创建卷 |
| GET | /api/v1/volumes/:name | 获取卷详情 |
| DELETE | /api/v1/volumes/:name | 删除卷 |
| POST | /api/v1/volumes/:name/subvolumes | 创建子卷 |
| POST | /api/v1/volumes/:name/snapshots | 创建快照 |
| POST | /api/v1/volumes/:name/balance | 平衡数据 |
| POST | /api/v1/volumes/:name/scrub | 数据校验 |

### 共享管理
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/shares | 获取共享列表 |
| POST | /api/v1/shares/smb | 创建 SMB 共享 |
| POST | /api/v1/shares/nfs | 创建 NFS 共享 |
| DELETE | /api/v1/shares/:id | 删除共享 |
| PUT | /api/v1/shares/:id | 更新共享配置 |

### 配置管理
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/config | 获取配置 |
| PUT | /api/v1/config | 更新配置 |
| POST | /api/v1/config/reload | 重载配置 |

完整 API 文档请查看 [docs/API_GUIDE.md](docs/API_GUIDE.md)

## 项目结构

```
nas-os/
├── cmd/           # 可执行程序入口
├── internal/      # 内部模块
│   ├── storage/   # 存储管理
│   ├── web/       # Web 服务
│   ├── smb/       # SMB 共享
│   ├── nfs/       # NFS 共享
│   └── users/     # 用户管理
├── pkg/           # 公共库
├── webui/         # 前端界面
└── configs/       # 配置文件
```

## 开发计划

详细里程碑请查看 [MILESTONES.md](MILESTONES.md)

### 当前状态 (2026-03-20) - v1.8.0 Stable ✅

**8/8 里程碑全部完成**

- [x] 项目骨架
- [x] btrfs 完整功能 (卷/子卷/快照/balance/scrub)
- [x] Web 框架 + Web UI (响应式设计)
- [x] SMB/NFS 共享实现
- [x] 用户权限系统 (RBAC/MFA)
- [x] 系统监控告警 (多通道通知)
- [x] 性能优化 (LRU 缓存/GC 调优/工作池)
- [x] Docker 多架构镜像 (amd64/arm64/armv7)
- [x] CI/CD 自动化 (全绿通过)
- [x] **容器管理** (Docker Compose 支持)
- [x] **虚拟机管理** (ISO 挂载/快照)
- [x] **权限管理 WebUI** (角色/ACL)
- [x] **监控告警 WebUI** (实时图表)
- [x] **六部轮值系统** (自动推进开发)
- [x] **配额管理** (用户/组/目录三级)
- [x] **回收站功能** (安全删除/恢复)
- [x] **WebDAV 支持** (完整协议)
- [x] **存储复制** (跨节点同步)
- [x] **AI 智能分类** (照片/文件分类)
- [x] **文件版本控制** (自动快照/版本对比/一键还原)
- [x] **云同步增强** (多云存储/双向同步)
- [x] **数据去重** (文件级/块级去重)

### 版本路线图
| 版本 | 类型 | 发布日期 | 核心功能 | 状态 |
|------|------|----------|----------|------|
| v0.1.0 | Alpha | 2026-03-10 | 项目骨架、btrfs 基础 | ✅ 发布 |
| v0.2.0 | Alpha | 2026-03-10 | 文件共享、配置持久化 | ✅ 发布 |
| v1.0.0 | Stable | 2026-03-11 | 生产就绪版本 | ✅ 已发布 |
| v1.1.0 | Stable | 2026-03-12 | 功能大更新 (10 个新模块) | ✅ 已发布 |
| v1.2.0 | Stable | 2026-03-12 | 安全加固与性能优化 | ✅ 已发布 |
| v1.3.0 | Stable | 2026-03-12 | 容器管理和 VM 功能 | ✅ 已发布 |
| v1.4.x | Stable | 2026-03-12 | RBAC 权限系统 + WebUI | ✅ 已发布 |
| v1.5.x | Stable | 2026-03-13 | 监控告警系统 + WebUI | ✅ 已发布 |
| v1.6.0 | Stable | 2026-03-13 | 性能优化 + CI/CD 完善 | ✅ 已发布 |
| v1.7.0 | Stable | 2026-03-13 | 配额/回收站/WebDAV/复制/AI | ✅ 已发布 |
| v1.8.0 | Stable | 2026-03-20 | 版本控制/云同步/去重 | ✅ 已发布 |
| v2.0.0 | Stable | 2026-04-01 | 存储复制/回收站增强 | ✅ 已发布 |
| **v2.2.0** | **Stable** | **2026-03-21** | **iSCSI/快照策略/仪表板增强/性能监控** | ✅ **已发布** |
| **v2.3.0** | **Stable** | **2026-03-28** | **存储分层/FTP-SFTP/压缩存储/文件标签** | ✅ **已发布** |
| **v2.4.0** | **Stable** | **2026-03-14** | **集成测试完善/文档更新** | ✅ **已发布** |
| **v2.5.0** | **Stable** | **2026-03-14** | **快照复制/高可用/备份恢复集成测试** | ✅ **已发布** |
| **v2.6.0** | **Stable** | **2026-03-14** | **安全增强/性能优化/集成测试完善** | ✅ **已发布** |
| **v2.11.0** | **Stable** | **2026-03-14** | **Bug 修复/测试完善/安全审计** | ✅ 已发布 |
| **v2.11.1** | **Stable** | **2026-03-14** | **gin 依赖修复/CI/CD 修复** | ✅ 已发布 |
| **v2.11.2** | **Stable** | **2026-03-14** | **CI/CD 修复/代码质量改进** | ✅ 已发布 |
| **v2.12.0** | **Stable** | **2026-03-14** | **SMB/NFS 修复/Go 1.25 升级/安全审计** | ✅ 已发布 |
| **v2.15.0** | **Stable** | **2026-03-14** | **SQLite 驱动替换/测试覆盖率提升/WebUI 完善** | ✅ 已发布 |
| **v2.16.0** | **Stable** | **2026-03-15** | **预测分析/i18n 国际化/API 文档系统** | ✅ 已发布 |
| **v2.17.1** | **Stable** | **2026-03-14** | **文档完善/版本更新** | ✅ 已发布 |
| **v2.18.0** | **Stable** | **2026-03-15** | **功能完善/代码质量** | ✅ 已发布 |
| **v2.19.0** | **Stable** | **2026-03-15** | **数据竞争修复/并发安全** | ✅ 已发布 |
| **v2.20.0** | **Stable** | **2026-03-14** | **代码清理/CI优化/文档完善** | ✅ 已发布 |
| **v2.26.0** | **Stable** | **2026-03-15** | **网络诊断/Docker 增强/自动化完善** | ✅ 已发布 |
| **v2.27.0** | **Stable** | **2026-03-15** | **媒体服务/配额自动扩展/监控增强** | ✅ 已发布 |
| **v2.30.0** | **Stable** | **2026-03-15** | **文档完善/API 文档覆盖** | ✅ 已发布 |
| **v2.31.0** | **Stable** | **2026-03-15** | **API 文档完善/国际化更新** | ✅ 已发布 |
| **v2.35.0** | **Stable** | **2026-03-15** | **请求日志/Excel导出/开发环境增强** | ✅ 已发布 |
| **v2.36.0** | **Stable** | **2026-03-15** | **i18n框架/API中间件/成本分析** | ✅ 已发布 |
| **v2.37.0** | **Stable** | **2026-03-15** | **国际化补全/CI优化/文档更新** | ✅ 已发布 |
| **v2.38.0** | **Stable** | **2026-03-15** | **文档同步/版本号更新** | ✅ 已发布 |
| **v2.39.0** | **Stable** | **2026-03-15** | **项目治理/文档体系完善** | ✅ 已发布 |
| **v2.40.0** | **Stable** | **2026-03-15** | **安全审计/并发修复/CI优化** | ✅ 已发布 |
| **v2.42.0** | **Stable** | **2026-03-15** | **测试修复/CI优化/Swagger文档** | ✅ 已发布 |
| **v2.44.0** | **Stable** | **2026-03-15** | **测试修复/文档完善** | ✅ 已发布 |
| **v2.61.0** | **Stable** | **2026-03-15** | **文档体系完善/用户指南优化/API文档补充** | ✅ 已发布 |
| **v2.253.208** | **Stable** | **2026-03-20** | **代码质量提升/Lint修复/安全加固/六部协同** | ✅ 已发布 |
| **v2.253.208** | **Stable** | **2026-03-21** | **依赖更新/安全增强/文档同步** | ✅ **已发布** |
| **v2.253.208** | **Stable** | **2026-03-21** | **版本迭代/六部协同维护** | ✅ **已发布** |

## v2.253.208 新增功能

| 功能 | 说明 |
|------|------|
| 🔧 Lint 修复 | 修复 50+ golangci-lint revive 错误（命名规范、注释规范） |
| 🛡️ 安全加固 | 修复整数溢出漏洞 (G115)，文件权限修复 (0644→0600) |
| 🏗️ 代码重构 | 解决类型命名 stuttering 问题 (SnapshotExecutor→Executor 等) |
| 📊 六部协同 | 兵部/刑部/礼部/工部/吏部/户部自动化开发流程 |
| 📚 文档同步 | 版本号一致性维护，CHANGELOG 规范化 |

## v2.253.208 新增功能

| 功能 | 说明 |
|------|------|
| 🔄 依赖更新 | 安全依赖更新，输入验证增强 |
| 📚 文档同步 | 版本号一致性维护 |

## v2.61.0 新增功能

| 功能 | 说明 |
|------|------|
| 📚 文档体系完善 | README 版本同步、用户指南索引优化 |
| 📖 API 文档补充 | 新增存储 API、用户 API 文档 |
| 📝 用户指南优化 | 添加版本号、优化文档结构 |

## v2.44.0 新增功能

| 功能 | 说明 |
|------|------|
| 🧪 测试修复 | 告警模块测试用例优化 |
| 📚 文档完善 | 用户快速入门、API示例、FAQ更新 |

## v2.42.0 新增功能

| 功能 | 说明 |
|------|------|
| 📚 Swagger API 文档 | 完整 OpenAPI/Swagger 文档生成，覆盖所有主要模块 |
| 🧪 测试修复 | 并发测试、存储成本测试、容量规划测试完善 |
| 🚀 CI/CD 优化 | Node.js 24 支持、缓存策略优化、构建并行化 |

## v2.40.0 新增功能

| 功能 | 说明 |
|------|------|
| 🛡️ 安全审计系统 | 9 项安全检查，完整测试覆盖 |
| 🔧 并发安全修复 | WebSocket、Response、LDAP 等模块修复 |
| 🚀 CI/CD 优化 | 超时配置、测试并行化、健康检查修复 |
| 📊 配额管理优化 | 成本计算验证、资源效率分析 |

## v2.39.0 新增功能

| 功能 | 说明 |
|------|------|
| 📚 项目治理完善 | 版本号统一、CHANGELOG 规范化 |
| 📖 文档体系完善 | 文档结构优化、发布说明完善 |

## v2.38.0 新增功能

| 功能 | 说明 |
|------|------|
| 📚 文档同步 | 所有文档版本号同步至 v2.38.0 |
| 📖 README 更新 | 更新下载链接、Docker 镜像版本 |
| 📝 docs 更新 | 更新文档中心索引和英文文档 |

## v2.37.0 新增功能

| 功能 | 说明 |
|------|------|
| 🌐 国际化补全 | 日韩文翻译补全，四种语言键数一致 (286个) |
| 📚 文档更新 | CHANGELOG 添加 v2.37.0，README 版本号同步 |
| 🔧 CI/CD 优化 | 工作流优化，安全扫描增强 |
| 🧪 测试改进 | 测试用例修复，覆盖率保持稳定 |

## v2.36.0 新增功能

| 功能 | 说明 |
|------|------|
| 🌐 i18n 国际化框架 | 完整翻译系统，支持中/英/日/韩四种语言 |
| 🔌 API 中间件系统 | 统一错误处理、响应时间记录、WebSocket 增强 |
| 💰 成本分析报告 | 存储成本分析、资源计费统计、趋势预测 |
| 📊 监控配置增强 | Prometheus 集成优化、告警规则完善 |

## v2.35.0 新增功能

| 功能 | 说明 |
|------|------|
| 📝 请求日志中间件 | 完整请求日志记录、请求ID追踪、结构化输出 |
| 📊 Excel 报告导出 | 完整Excel导出器、样式设置、多工作表支持 |
| 🔧 开发环境增强 | Air热重载、Docker Compose开发环境 |
| 📖 文档完善 | API快速入门指南、发布流程文档 |

## v2.32.0 新增功能

| 功能 | 说明 |
|------|------|
| 📊 稳定性提升 | 核心模块测试覆盖率提升 |
| 📚 文档完善 | API 文档 Swagger 注释完善 |
| ⚡ 性能优化 | 缓存和并发性能优化 |
| 🔒 安全增强 | 权限检查和安全审计 |

## v2.31.0 新增功能

| 功能 | 说明 |
|------|------|
| 📚 文档完善 | 快速开始指南、用户文档更新 |
| 📡 API 文档覆盖 | 完善所有 API 模块的 Swagger 注释 |
| 📖 文档索引优化 | 更新文档中心索引，按角色导航 |

## v2.27.0 新增功能

| 功能 | 说明 |
|------|------|
| 🎬 媒体服务 | HLS/DASH 流媒体、字幕处理、视频转码、缩略图生成 |
| 📈 配额自动扩展 | 自动扩展配额策略、审批流程、回滚支持 |
| 📊 监控增强 | 健康评分系统、指标收集器、报告集成 |

<details>
<summary>v2.26.0 新增功能</summary>

| 功能 | 说明 |
|------|------|
| 🔍 网络诊断 | Ping/Traceroute/DNS 查询/端口扫描/Whois 查询 |
| 🐳 Docker 增强 | 容器批量操作、镜像管理、网络配置、卷管理 |
| ⚙️ 自动化完善 | 工作流执行优化、Action 解析增强、错误处理改进 |

</details>

<details>
<summary>v2.3.0 新增功能</summary>

| 功能 | 说明 |
|------|------|
| 🗂️ 存储分层 | 热/冷数据自动分层，SSD 缓存层加速，云存储归档 |
| 📡 FTP 服务器 | 被动/主动模式，匿名登录，带宽限制，虚拟目录 |
| 🔐 SFTP 服务器 | SSH 密钥认证，用户权限隔离，chroot 限制 |
| 🗜️ 压缩存储 | 文件级/块级压缩，透明压缩，节省空间 |
| 🏷️ 文件标签 | 标签分类，颜色图标，批量操作，标签云 |

</details>

<details>
<summary>v2.2.0 新增功能</summary>

| 功能 | 说明 |
|------|------|
| 🎯 iSCSI 目标 | iSCSI Target 服务，支持 LUN 管理和 CHAP 认证 |
| 📸 快照策略 | 自动化快照调度，支持多种保留策略 |
| 🖥️ 仪表板增强 | 全新 WebUI 仪表板，可自定义小部件布局 |
| 📊 性能监控增强 | 性能基线学习、异常检测、优化建议 |

</details>

<details>
<summary>v1.8.0 新增功能</summary>

| 功能 | 说明 |
|------|------|
| 📜 文件版本控制 | 自动保存历史版本，支持版本恢复和对比 |
| ☁️ 云同步增强 | 支持阿里云 OSS、腾讯云 COS、AWS S3、Google Drive、OneDrive、Backblaze B2 |
| 🔄 双向同步 | 本地↔云端实时/定时同步，冲突自动解决 |
| 🗜️ 数据去重 | 文件级/块级去重，节省存储空间 |
| 📊 去重报告 | 详细的空间节省统计和可视化 |
| 🌐 多云存储 | 统一管理多个云存储提供商 |

<details>
<summary>v1.7.0 新增功能</summary>

| 功能 | 说明 |
|------|------|
| 📊 存储配额 | 用户/组/目录三级配额控制 |
| 🗑️ 回收站 | 安全删除，支持恢复 |
| 📁 WebDAV | 完整 WebDAV 协议支持 |
| 🔄 存储复制 | 跨节点数据同步 |
| 🤖 AI 分类 | 照片/文件智能分类 |
| ⚡ 性能优化 | LRU 缓存/连接池/工作池 |
| 📈 报告系统 | 定时生成存储/使用报告 |

</details>

## 部署

### Docker 部署
```bash
# 快速启动（开发测试）
docker-compose up -d

# 查看日志
docker-compose logs -f
```

### 裸机安装
```bash
# 一键安装脚本
curl -fsSL https://raw.githubusercontent.com/your-org/nas-os/main/scripts/install.sh | sudo bash

# 或手动安装
sudo ./scripts/install.sh
```

### 系统服务
```bash
systemctl status nas-os
systemctl restart nas-os
journalctl -u nas-os -f
```

## 配置示例

创建配置文件 `/etc/nas-os/config.yaml`:

```yaml
version: "1.0.0"

server:
  port: 8080
  host: 0.0.0.0
  tls_enabled: false

storage:
  data_path: /data
  auto_scrub: true
  scrub_schedule: "0 3 * * 0"  # 每周日凌晨 3 点

shares:
  smb:
    enabled: true
    workgroup: WORKGROUP
    shares:
      - name: public
        path: /data/public
        guest_ok: true
      - name: home
        path: /data/home
        guest_ok: false
  
  nfs:
    enabled: true
    allowed_networks:
      - 192.168.1.0/24
    exports:
      - path: /data/nfs
        clients: ["192.168.1.0/24"]
        options: ["rw", "sync", "no_subtree_check"]

logging:
  level: info
  format: json
  file: /var/log/nas-os/nasd.log
  max_size: 100
  max_backups: 5
```

## 快速使用

### 1. 创建存储卷
```bash
sudo nasctl volume create mydata --path /dev/sda1
```

### 2. 创建 SMB 共享
```bash
sudo nasctl share create smb public --path /data/public --guest
```

### 3. 创建 NFS 共享
```bash
sudo nasctl share create nfs backup --path /data/backup --network 192.168.1.0/24
```

### 4. 从客户端访问
- **Windows**: `\\<服务器 IP>\public`
- **macOS**: `smb://<服务器 IP>/public`
- **Linux (NFS)**: `sudo mount <服务器 IP>:/backup /mnt/local_backup`

## 获取帮助

- 📖 **完整文档**: [docs/](docs/) 目录
- 🐛 **报告问题**: [GitHub Issues](https://github.com/crazyqin/nas-os/issues)
- 💬 **社区讨论**: [GitHub Discussions](https://github.com/crazyqin/nas-os/discussions)
- 📦 **Docker 镜像**: [GHCR](https://github.com/crazyqin/nas-os/pkgs/container/nas-os)

## License

MIT
