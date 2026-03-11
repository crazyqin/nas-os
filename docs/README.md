# NAS-OS v1.0 文档中心

> **版本**: v1.0.0  
> **发布日期**: 2026-03-11  
> **项目**: NAS-OS 家用 NAS 系统

---

## 📚 核心文档（v1.0）

| 文档 | 说明 | 适合人群 | 状态 |
|------|------|----------|------|
| [USER_MANUAL_v1.0.md](USER_MANUAL_v1.0.md) | 📘 完整用户手册，包含安装、配置、使用指南 | 所有用户 | ✅ 完成 |
| [ADMIN_GUIDE_v1.0.md](ADMIN_GUIDE_v1.0.md) | 🔧 管理员指南，高级配置、安全、性能优化 | 系统管理员 | ✅ 完成 |
| [API_REFERENCE_v1.0.md](API_REFERENCE_v1.0.md) | 📡 API 参考文档，完整 REST API 接口说明 | 开发者 | ✅ 完成 |
| [DEPLOYMENT_GUIDE_v1.0.md](DEPLOYMENT_GUIDE_v1.0.md) | 🚀 部署指南，生产环境部署、高可用配置 | DevOps 工程师 | ✅ 完成 |
| [NASCTL-CLI.md](NASCTL-CLI.md) | 💻 CLI 命令行工具使用指南 | 所有用户 | ✅ 完成 |

---

## 🎯 按角色查找

### 我是新用户
1. 阅读 [用户手册](USER_MANUAL_v1.0.md) - 快速开始章节
2. 按照 [部署指南](DEPLOYMENT_GUIDE_v1.0.md) 完成安装
3. 访问 Web UI：`http://<服务器 IP>:8080`
4. 创建第一个存储卷和共享文件夹

### 我是系统管理员
1. 阅读 [管理员指南](ADMIN_GUIDE_v1.0.md)
2. 配置 [安全管理](ADMIN_GUIDE_v1.0.md#安全管理)
3. 设置 [性能优化](ADMIN_GUIDE_v1.0.md#性能优化)
4. 配置 [监控告警](ADMIN_GUIDE_v1.0.md#监控与告警)
5. 设置 [备份策略](ADMIN_GUIDE_v1.0.md#备份策略)

### 我是开发者
1. 阅读 [API 参考](API_REFERENCE_v1.0.md)
2. 查看 [SDK 示例](API_REFERENCE_v1.0.md#sdk 示例)
3. 参考 [开发者文档](DEVELOPER.md)
4. 了解 [项目架构](DEVELOPER.md#架构概览)

### 我是 DevOps 工程师
1. 阅读 [部署指南](DEPLOYMENT_GUIDE_v1.0.md)
2. 配置 [生产环境](DEPLOYMENT_GUIDE_v1.0.md#生产环境部署)
3. 设置 [高可用](DEPLOYMENT_GUIDE_v1.0.md#高可用部署)
4. 配置 [监控日志](DEPLOYMENT_GUIDE_v1.0.md#监控与日志)
5. 制定 [备份恢复](DEPLOYMENT_GUIDE_v1.0.md#备份与恢复) 策略

---

## 📖 文档结构

```
nas-os/docs/
├── README.md                     # 📚 本文档（索引）
├── USER_MANUAL_v1.0.md           # 📘 用户手册（28KB）
├── ADMIN_GUIDE_v1.0.md           # 🔧 管理员指南（32KB）
├── API_REFERENCE_v1.0.md         # 📡 API 参考（52KB）
├── DEPLOYMENT_GUIDE_v1.0.md      # 🚀 部署指南（34KB）
│
├── DEVELOPER.md                  # 开发者指南
├── PLUGINS.md                    # 插件开发指南
├── CI-CD.md                      # CI/CD 流程
├── CHANGELOG.md                  # 版本更新日志
├── MILESTONES.md                 # 项目里程碑
│
└── ...                           # 其他技术文档
```

---

## 📋 v1.0 文档完成情况

| 文档类型 | 文档名称 | 大小 | 状态 |
|----------|----------|------|------|
| 用户手册 | USER_MANUAL_v1.0.md | 28KB | ✅ 完成 |
| 管理员指南 | ADMIN_GUIDE_v1.0.md | 32KB | ✅ 完成 |
| API 参考 | API_REFERENCE_v1.0.md | 52KB | ✅ 完成 |
| 部署指南 | DEPLOYMENT_GUIDE_v1.0.md | 34KB | ✅ 完成 |

**总计**: 4 份核心文档，约 146KB

---

## 🔗 外部资源

- **官方网站**: https://nas-os.dev
- **GitHub**: https://github.com/nas-os/nasd
- **Docker Hub**: https://hub.docker.com/r/nas-os/nasd
- **Discord 社区**: https://discord.gg/nas-os
- **文档站点**: https://docs.nas-os.dev

---

## 📅 更新日志

### v1.0.0 (2026-03-11)
- ✨ 新增：完整用户手册（USER_MANUAL_v1.0.md）
- ✨ 新增：管理员指南（ADMIN_GUIDE_v1.0.md）
- ✨ 新增：API 参考文档（API_REFERENCE_v1.0.md）
- ✨ 新增：部署指南（DEPLOYMENT_GUIDE_v1.0.md）
- 📝 更新：文档索引和导航结构

---

## 📝 文档规范

### 命名规则
- 核心文档使用 `*_v1.0.md` 格式
- 文件名使用大写，单词间用 `_` 连接
- 所有文档使用 Markdown 格式

### 更新流程
1. 文档负责人起草/更新
2. 相关部门审查
3. 更新本文档索引
4. 发布到文档站点

---

*最后更新：2026-03-11*  
*文档由 NAS-OS 社区维护*
