# NAS-OS 文档中心

> **版本**: v2.194.0
> **发布日期**: 2026-03-17
> **项目**: NAS-OS 家用 NAS 系统

---

## 🚀 快速导航

| [快速开始指南](QUICKSTART.md) | ⚡ 5 分钟快速上手 | 所有新用户 |
| [用户手册](USER_GUIDE.md) | 📘 完整使用指南 | 所有用户 |
| [常见问题](FAQ.md) | ❓ 30+ 常见问题解答 | 所有用户 |
| [API 文档](API_GUIDE.md) | 📡 REST API 参考 | ✅ v2.151.0 |
| [管理员指南](ADMIN_GUIDE_v2.5.0.md) | 🔧 高级配置 | 系统管理员 |
| [部署指南](DEPLOYMENT_GUIDE_v2.5.0.md) | 🚀 生产部署 | DevOps |
| [故障排查](TROUBLESHOOTING.md) | 🔍 问题诊断与解决 | 所有用户 |

---

## 📚 核心文档

| 文档 | 说明 | 状态 |
|------|------|------|
| [QUICKSTART.md](QUICKSTART.md) | 快速开始指南 | ✅ v2.151.0 |
| [USER_GUIDE.md](USER_GUIDE.md) | 用户使用指南 | ✅ v2.151.0 |
| [FAQ.md](FAQ.md) | 常见问题解答 | ✅ v2.151.0 |
| [ADMIN_GUIDE_v2.5.0.md](ADMIN_GUIDE_v2.5.0.md) | 管理员指南 | ✅ v2.5.0 |
| [API_GUIDE.md](API_GUIDE.md) | API 使用指南 | ✅ v2.151.0 |
| [DEPLOYMENT_GUIDE_v2.5.0.md](DEPLOYMENT_GUIDE_v2.5.0.md) | 部署指南 | ✅ v2.5.0 |
| [DEVELOPER.md](DEVELOPER.md) | 开发者文档 | ✅ 完成 |
| [NASCTL-CLI.md](NASCTL-CLI.md) | CLI 工具指南 | ✅ 完成 |
| [TROUBLESHOOTING.md](TROUBLESHOOTING.md) | 故障排查指南 | ✅ v2.153.0 |

---

## 🎯 按角色查找

### 我是新用户
1. 阅读 [快速开始指南](QUICKSTART.md) - 5 分钟上手
2. 按照 [部署指南](DEPLOYMENT_GUIDE_v2.5.0.md) 完成安装
3. 访问 Web UI：`http://<服务器 IP>:8080`
4. 创建第一个存储卷和共享文件夹

### 我是系统管理员
1. 阅读 [管理员指南](ADMIN_GUIDE_v2.5.0.md)
2. 配置 [安全管理](ADMIN_GUIDE_v2.5.0.md#安全管理)
3. 设置 [性能优化](ADMIN_GUIDE_v2.5.0.md#性能优化)
4. 配置 [监控告警](ADMIN_GUIDE_v2.5.0.md#监控与告警)
5. 设置 [备份策略](ADMIN_GUIDE_v2.5.0.md#备份策略)

### 我是开发者
1. 阅读 [API 文档](API_GUIDE.md)
2. 查看 OpenAPI 规范 [api.yaml](api.yaml)
3. 参考 [开发者文档](DEVELOPER.md)
4. 了解 [项目架构](DEVELOPER.md#架构概览)

### 我是 DevOps 工程师
1. 阅读 [部署指南](DEPLOYMENT_GUIDE_v2.5.0.md)
2. 配置 [生产环境](DEPLOYMENT_GUIDE_v2.5.0.md#生产环境部署)
3. 设置 [高可用](DEPLOYMENT_GUIDE_v2.5.0.md#高可用部署)
4. 配置 [监控日志](DEPLOYMENT_GUIDE_v2.5.0.md#监控与日志)
5. 制定 [备份恢复](DEPLOYMENT_GUIDE_v2.5.0.md#备份与恢复) 策略

---

## 📖 功能模块文档

| 模块 | 文档 | 说明 |
|------|------|------|
| 🔐 权限管理 | [user-guide/permission-guide.md](user-guide/permission-guide.md) | RBAC 权限系统指南 (v2.55.0+) |
| 📸 照片管理 | [PHOTOS_API.md](PHOTOS_API.md) | 相册、AI 分类、人脸识别 |
| 🌐 网络管理 | [NETWORK_API.md](NETWORK_API.md) | DDNS、防火墙、端口转发 |
| 🎯 iSCSI | [ISCSI_GUIDE.md](ISCSI_GUIDE.md) | iSCSI 目标配置 |
| 📈 监控 | [PERFORMANCE_MONITORING_GUIDE.md](PERFORMANCE_MONITORING_GUIDE.md) | 性能监控指南 |
| 📊 配额 | [QUOTA_AUTO_EXPAND_GUIDE.md](QUOTA_AUTO_EXPAND_GUIDE.md) | 配额自动扩展 |
| 🎬 媒体服务 | [MEDIA_SERVICE_GUIDE.md](MEDIA_SERVICE_GUIDE.md) | 流媒体服务 |
| ☁️ 云同步 | [CLOUDSYNC.md](CLOUDSYNC.md) | 多云同步配置 |
| 💾 备份恢复 | [BACKUP_GUIDE.md](BACKUP_GUIDE.md) | 备份策略与恢复操作 |
| 🐳 Docker | [DOCKER_GUIDE.md](DOCKER_GUIDE.md) | Docker 容器管理 |
| 🔒 安全审计 | [SECURITY-AUDIT-2026-03-14.md](SECURITY-AUDIT-2026-03-14.md) | 安全审计报告 |
| 🌐 LDAP/AD | [LDAP-INTEGRATION.md](LDAP-INTEGRATION.md) | 企业目录集成 |
| 📝 审计日志 | [user-guide/audit-guide.md](user-guide/audit-guide.md) | 审计日志查看与分析 |
| 💰 计费系统 | [user-guide/billing-guide.md](user-guide/billing-guide.md) | 成本管理、账单查看 |
| 📊 仪表板 | [user-guide/dashboard-guide.md](user-guide/dashboard-guide.md) | 自定义监控仪表板 |
| 📡 分布式监控 | [user-guide/distributed-monitoring-guide.md](user-guide/distributed-monitoring-guide.md) | 多节点监控配置 |

---

## 📖 文档结构

```
nas-os/docs/
├── README.md                     # 📚 本文档（索引）
├── QUICKSTART.md                 # ⚡ 快速开始指南 (新增)
├── USER_GUIDE.md                 # 📘 用户指南
├── ADMIN_GUIDE_v2.5.0.md         # 🔧 管理员指南
├── API_GUIDE.md                  # 📡 API 使用指南
├── api.yaml                      # 📋 OpenAPI 规范
│
├── DEVELOPER.md                  # 开发者指南
├── PLUGINS.md                    # 插件开发指南
├── CI-CD.md                      # CI/CD 流程
├── CHANGELOG.md                  # 版本更新日志
│
└── ...                           # 其他技术文档
```

---

## 🔗 外部资源

- **官方网站**: https://nas-os.dev
- **GitHub**: https://github.com/nas-os/nasd
- **GHCR**: https://github.com/nas-os/nas-os/pkgs/container/nas-os
- **Discord 社区**: https://discord.gg/nas-os
- **文档站点**: https://docs.nas-os.dev

---

## 📅 更新日志

### v2.193.0 (2026-03-17)
- 📝 更新：所有文档版本号同步至 v2.193.0
- 🔧 修复：版本号同步更新
- 🚀 优化：文档版本管理

### v2.192.0 (2026-03-17)
- 📝 更新：所有文档版本号同步至 v2.192.0
- 🔧 修复：errcheck 错误
- 🚀 优化：CI/CD 配置
- 🔒 安全：审计通过

### v2.191.0 (2026-03-17)
- 📝 更新：所有文档版本号同步至 v2.165.0
- 📝 更新：API 文档版本同步
- 📝 更新：英文文档版本同步
- 🔧 修复：golangci-lint v2 配置

### v2.164.0 (2026-03-17)
- 📝 更新：所有文档版本号同步至 v2.164.0
- 📝 更新：API 文档版本同步
- 📝 更新：英文文档版本同步

### v2.153.0 (2026-03-17)
- 📝 更新：所有文档版本号同步至 v2.153.0
- 📝 更新：API 文档版本同步
- 📝 更新：英文文档版本同步

### v2.151.0 (2026-03-17)
- 📝 更新：所有文档版本号同步至 v2.151.0
- 📝 更新：API 文档版本同步
- 📝 更新：英文文档版本同步

### v2.148.0 (2026-03-17)
- 📝 更新：所有文档版本号同步至 v2.148.0
- 📝 更新：API 文档版本同步
- 📝 更新：英文文档版本同步

### v2.147.0 (2026-03-17)
- 📝 更新：所有文档版本号同步至 v2.147.0
- 📝 更新：API 文档版本同步

### v2.146.0 (2026-03-17)
- 📝 更新：所有文档版本号同步至 v2.144.0
- 📝 更新：API 文档版本同步

### v2.133.0 (2026-03-16)
- 📝 更新：所有文档版本号同步至 v2.133.0
- 📝 更新：API 文档版本同步
- 🔧 CI/CD 缓存版本统一

### v2.132.0 (2026-03-16)
- 📝 更新：所有文档版本号同步至 v2.132.0
- 📝 更新：API 文档版本同步

### v2.120.0 (2026-03-16)
- 📝 更新：所有文档版本号同步至 v2.120.0
- 📝 更新：API 文档版本同步

### v2.116.0 (2026-03-16)
- 📝 更新：所有文档版本号同步至 v2.116.0
- 📝 更新：API 文档版本同步

### v2.114.0 (2026-03-16)
- 📝 更新：所有文档版本号同步至 v2.114.0
- 📝 更新：API 文档版本同步

### v2.112.0 (2026-03-16)
- 📝 更新：所有文档版本号同步至 v2.112.0
- 📝 更新：API 文档版本同步

### v2.111.0 (2026-03-16)
- 📝 更新：所有文档版本号同步至 v2.111.0
- 📝 更新：API 文档版本同步

### v2.110.0 (2026-03-16)

### v2.109.0 (2026-03-16)
- 📝 更新：所有文档版本号同步至 v2.109.0
- 🔧 优化：CI/CD 测试覆盖率阈值调整

### v2.76.0 (2026-03-16)
- ✨ 新增：六部协同开发框架
- ✨ 新增：安全审计报告 (SECURITY_AUDIT_v2.71.0.md)
- ✨ 新增：配置验证脚本、日志分析脚本
- 📝 更新：所有文档版本号同步至 v2.76.0

### v2.61.0 (2026-03-16)
- ✨ 新增：用户指南索引文件
- ✨ 新增：存储 API 文档
- ✨ 新增：用户 API 文档
- 📝 更新：所有文档版本号同步

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

*最后更新：2026-03-17*  
*文档由 NAS-OS 社区维护*
