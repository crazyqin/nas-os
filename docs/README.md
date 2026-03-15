# NAS-OS 文档中心

> **版本**: v2.54.0  
> **发布日期**: 2026-03-15  
> **项目**: NAS-OS 家用 NAS 系统

---

## 🚀 快速导航

| 文档 | 说明 | 适合人群 |
|------|------|----------|
| [快速开始指南](QUICKSTART.md) | ⚡ 5 分钟快速上手 | 所有新用户 |
| [用户手册](USER_GUIDE.md) | 📘 完整使用指南 | 所有用户 |
| [API 文档](API_GUIDE.md) | 📡 REST API 参考 | 开发者 |
| [管理员指南](ADMIN_GUIDE_v2.5.0.md) | 🔧 高级配置 | 系统管理员 |
| [部署指南](DEPLOYMENT_GUIDE_v2.5.0.md) | 🚀 生产部署 | DevOps |

---

## 📚 核心文档

| 文档 | 说明 | 状态 |
|------|------|------|
| [QUICKSTART.md](QUICKSTART.md) | 快速开始指南 | ✅ v2.31.0 |
| [USER_GUIDE.md](USER_GUIDE.md) | 用户使用指南 | ✅ v2.31.0 |
| [ADMIN_GUIDE_v2.5.0.md](ADMIN_GUIDE_v2.5.0.md) | 管理员指南 | ✅ v2.5.0 |
| [API_GUIDE.md](API_GUIDE.md) | API 使用指南 | ✅ v2.27.0 |
| [DEPLOYMENT_GUIDE_v2.5.0.md](DEPLOYMENT_GUIDE_v2.5.0.md) | 部署指南 | ✅ v2.5.0 |
| [DEVELOPER.md](DEVELOPER.md) | 开发者文档 | ✅ 完成 |
| [NASCTL-CLI.md](NASCTL-CLI.md) | CLI 工具指南 | ✅ 完成 |

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
| 📸 照片管理 | [PHOTOS_API.md](PHOTOS_API.md) | 相册、AI 分类、人脸识别 |
| 🌐 网络管理 | [NETWORK_API.md](NETWORK_API.md) | DDNS、防火墙、端口转发 |
| 🎯 iSCSI | [ISCSI_GUIDE.md](ISCSI_GUIDE.md) | iSCSI 目标配置 |
| 📈 监控 | [PERFORMANCE_MONITORING_GUIDE.md](PERFORMANCE_MONITORING_GUIDE.md) | 性能监控指南 |
| 📊 配额 | [QUOTA_AUTO_EXPAND_GUIDE.md](QUOTA_AUTO_EXPAND_GUIDE.md) | 配额自动扩展 |
| 🎬 媒体服务 | [MEDIA_SERVICE_GUIDE.md](MEDIA_SERVICE_GUIDE.md) | 流媒体服务 |
| ☁️ 云同步 | [CLOUDSYNC.md](CLOUDSYNC.md) | 多云同步配置 |
| 📁 文件标签 | [docs/FILE_TAGS.md](FILE_TAGS.md) | 文件标签系统 |

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

*最后更新：2026-03-15*  
*文档由 NAS-OS 社区维护*
