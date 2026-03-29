# 更新日志

所有重要的更改都将记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

---

## [v2.315.0] - 2026-03-29

### 🎯 六部协同开发第90轮 - 司礼监调度竞品学习与功能规划

### 竞品学习
- 🔍 **飞牛fnOS 1.1**: 网盘原生挂载、本地AI人脸识别、QWRT软路由、Cloudflare Tunnel
- 🔍 **群晖DSM 7.3**: Synology Tiering、AI Console、私有云AI服务、Drive 4.0
- 🔍 **TrueNAS 24.10**: Docker Compose原生、RAIDZ Expansion、勒索软件检测

### 六部协同成果
- 📋 **兵部**: 人脸识别增强框架设计（对标飞牛fnOS本地AI人脸识别）
- 📋 **礼部**: 应用中心WebUI优化方案（分类筛选/搜索优化/安装进度可视化）
- 📋 **刑部**: 人脸识别隐私合规检查清单（《个人信息保护法》《网络安全法》）
- 📋 **工部**: Cloudflare Tunnel集成研究（对标飞牛fnOS）
- 📋 **户部**: AI服务成本分析更新

### 文档新增
- `docs/司礼监-工作汇报-20260329.md` - 司礼监工作汇报
- `docs/face-recognition-privacy-compliance.md` - 人脸识别隐私合规检查清单
- `docs/app-center-webui-optimization.md` - 应用中心WebUI优化方案

---

## [v2.314.0] - 2026-03-29

### 🎯 六部协同开发第89轮 - 司礼监调度吏部版本管理

### 版本管理
- ✅ **版本号更新**: v2.313.0 → v2.314.0
- ✅ **文档同步**: VERSION、version.go、README.md、MILESTONES.md

---

## [v2.312.0] - 2026-03-29

### 🎯 六部协同开发第86轮 - 司礼监调度竞品学习深化

### 修复
- ✅ **修复搜索模块重复声明问题** - ResultType/Highlight/LogSearchRequest类型冲突

### 竞品学习深化

#### TrueNAS 25.10 Community Edition
- **RAID-Z Expansion**: 单盘在线扩展RAID-Z阵列，无需重建
- **GPU Sharing**: GPU资源池化共享，vGPU技术
- **Self-Encrypted Drives**: TCG Opal自加密驱动器支持
- **SMB Multichannel**: 多通道SMB提升传输性能
- **勒索软件防护**: 企业级安全防护机制
- **Apps/Docker Compose/LXC**: 应用生态完善

#### 群晖 DSM 7.3
- **私有云AI服务**: 本地LLM支持
- **Synology Tiering**: 自动冷热数据分层
- **Secure SignIn**: 多因素认证增强
- **AI Console**: 数据遮罩功能

#### 飞牛fnOS v1.1
- **网盘原生挂载**: 直接挂载云端存储
- **本地AI人脸识别**: Intel核显加速
- **QWRT软路由**: 集成路由功能
- **Cloudflare Tunnel**: 免费内网穿透

### 六部协同
| 部门 | 状态 | 主要工作 |
|------|------|----------|
| 吏部 | ✅ | 版本号v2.312.0、里程碑记录 |
| 兵部 | ✅ | 修复search模块重复声明、编译验证通过 |
| 工部 | 🔄 | CI/CD检查 |
| 礼部 | 🔄 | 文档更新、竞品分析 |
| 刑部 | 🔄 | 安全审计 |
| 户部 | 🔄 | 成本分析更新 |

---

## [v2.308.0] - 2026-03-29

### 🎯 六部协同开发第84轮 - 司礼监调度竞品学习

### 竞品学习成果
- **飞牛fnOS v1.1**: 网盘原生挂载、本地AI人脸识别、QWRT软路由、Cloudflare Tunnel
- **群晖DSM 7.3**: 私有云AI服务、Tiering分层存储、AI Console数据遮罩
- **TrueNAS 26**: RAIDZ单盘扩展、勒索软件检测、Fast Dedup

### 新增功能
- ✅ **勒索软件检测** - 实时监控+自动隔离+文件恢复
- ✅ **私有云AI服务框架** - OpenAI兼容API+本地LLM支持
- ✅ **人脸识别服务框架** - 本地AI人脸检测+聚类+人物相册

### 六部协同
| 部门 | 状态 | 主要工作 |
|------|------|----------|
| 吏部 | ✅ | 版本号v2.308.0、里程碑记录、提交发布 |
| 兵部 | ✅ | 勒索检测增强、AI服务框架、人脸识别框架 |
| 工部 | ✅ | CI/CD验证、监控配置 |
| 礼部 | ✅ | 文档更新、竞品分析更新 |
| 刑部 | ✅ | 安全审计、安全设计文档 |
| 户部 | ✅ | 成本分析、资源统计 |

---

## [v2.307.0] - 2026-03-29

### 🎯 六部协同开发第83轮 - 司礼监调度竞品学习

### 竞品学习要点

#### 群晖 DSM 7.3
- **Synology Tiering**: 冷热数据自动分层，SSD缓存热数据，HDD存冷数据
- **Hybrid Share**: 混合云存储，本地+云端无缝切换
- **Secure SignIn**: 生物识别+硬件密钥多因素认证
- **AI Console**: 本地LLM服务，私有云AI

#### TrueNAS 25.10
- **RAID-Z Expansion**: 在线扩展RAID-Z阵列，无需重建
- **GPU Sharing**: GPU资源池化共享，vGPU技术
- **SMB Multichannel**: 多通道SMB提升传输性能
- **Self-Encrypted Drives**: TCG Opal自加密驱动器支持

### 六部协同
| 部门 | 状态 | 主要工作 |
|------|------|----------|
| 吏部 | ✅ | 版本号v2.307.0、里程碑记录 |
| 兵部 | ✅ | go vet/go build通过 |
| 礼部 | ✅ | README版本同步 |
| 工部 | ✅ | CI/CD配置检查 |
| 刑部 | ✅ | 安全审计通过 |
| 户部 | ✅ | 资源统计完成 |

---