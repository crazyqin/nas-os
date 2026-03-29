# 更新日志

所有重要的更改都将记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

---

## [v2.319.0] - 2026-03-30

### 🎯 六部协同开发第94轮 - 司礼监调度竞品学习与RAIDZ规划

### 竞品学习成果整合
- 🔍 **TrueNAS RAIDZ Expansion**: 单盘在线扩展RAID-Z阵列技术调研完成
  - 扩容速度提升5-10倍（TrueNAS Fangtooth优化）
  - OpenZFS 2.3正式支持，保持原有冗余级别
  - 支持中断恢复，数据自动重分布
- 🔍 **TrueNAS全局搜索**: Global Search UI功能分析
  - 全局文件搜索界面设计要点
  - 快速定位文件，提升用户体验
- 🔍 **飞牛fnOS 1.1**: 网盘原生挂载、本地AI人脸识别成熟方案
- 🔍 **群晖DSM 7.3**: Tiering分层存储、私有云AI服务、Drive 4.0协作增强

### 文档新增
- `docs/RAIDZ_EXPANSION.md` - RAIDZ扩展功能文档框架
  - 功能概述与技术背景
  - 用户使用场景与规划
  - 与竞品对比分析

### 六部协同
| 部门 | 状态 | 主要工作 |
|------|------|----------|
| 户部 | ✅ | 资源统计完成 |
| 工部 | ✅ | DevOps检查、编译验证通过 |
| 礼部 | ✅ | 文档品牌建设、CHANGELOG更新、RAIDZ文档 |
| 刑部 | ✅ | 安全审计执行、gosec更新 |
| 兵部 | ✅ | 代码质量检查、go vet 0错误 |

---

## [v2.318.0] - 2026-03-30

### 🎯 六部协同开发第93轮 - 司礼监调度按需唤醒与内网穿透

### 新增功能
- ✅ **按需唤醒硬盘** - 延长硬盘寿命，降低功耗
- ✅ **Cloudflare Tunnel支持** - 无需开放端口实现远程访问

### 六部协同
| 部门 | 状态 | 主要工作 |
|------|------|----------|
| 吏部 | ✅ | 版本号v2.318.0、里程碑记录 |
| 兵部 | ✅ | 按需唤醒硬盘实现、Cloudflare Tunnel集成 |
| 工部 | ✅ | CI/CD验证 |
| 礼部 | ✅ | 文档更新、竞品分析更新 |
| 刑部 | ✅ | 安全审计 |
| 户部 | ✅ | 成本分析 |

---

## [v2.317.0] - 2026-03-30

### 🎯 六部协同开发第92轮 - 司礼监调度竞品学习与功能开发

### 竞品学习
- 🔍 **飞牛fnOS**: FN Connect免费内网穿透、AI相册、网盘原生挂载
- 🔍 **群晖DSM**: Synology Tiering、Drive文件锁定、AI Console、私有云AI
- 🔍 **TrueNAS**: RAIDZ逐盘扩展、LXC容器、全局搜索、NVMe健康监控
- 🔍 **铁威马TOS**: TRAID、直通挂载、SMB Multichannel

### 六部协同成果

#### 兵部（软件工程）
- ✅ **内网穿透增强**: Cloudflare Tunnel/FRP实现优化
- 📦 新增: `internal/tunnel/cloudflare_new.go`, `internal/tunnel/frp_new.go`

#### 工部（DevOps）
- ✅ **网盘挂载框架**: rclone集成、多云盘支持
- 📦 新增: `internal/cloudmount/manager.go`, `types.go`, `rclone_config.go`

---

## [v2.315.0] - 2026-03-29

### 🎯 六部协同开发第90轮 - 司礼监调度竞品学习与功能规划

### 竞品学习
- 🔍 **飞牛fnOS 1.1**: 网盘原生挂载、本地AI人脸识别、QWRT软路由、Cloudflare Tunnel
- 🔍 **群晖DSM 7.3**: Synology Tiering、AI Console、私有云AI服务、Drive 4.0
- 🔍 **TrueNAS 24.10**: RAIDZ扩展、全局搜索、Docker替代Kubernetes、NVMe S.M.A.R.T UI

### 功能规划
- 📋 RAIDZ扩展API设计（M104）
- 📋 全局搜索服务优化
- 📋 NVMe S.M.A.R.T测试UI接口