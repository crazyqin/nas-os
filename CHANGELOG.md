# 更新日志

所有重要的更改都将记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

---

## [v2.326.0] - 2026-03-30

### 🎯 六部协同开发第102轮 - RAIDZ扩容技术研究 + 竞品动态跟踪

### 修复
- ✅ **SMB SecurityManager死锁修复** - 持有写锁时调用saveConfig导致死锁
- ✅ **GPU模块补全** - 添加缺失的monitor.go和nvidia.go文件
- ✅ **HA测试修复** - TestHAManager_Events/StatePersistence测试注册node-2节点
- ✅ **文件管理测试修复** - TestListDirectory计数错误
- ✅ **编译错误修复** - HA模块和WebShare模块、passkey未使用变量
- ✅ **fmt.Errorf格式修复** - 非常量格式字符串错误

### 六部协同
| 部门 | 状态 | 主要工作 |
|------|------|----------|
| 兵部 | ✅ | RAIDZ扩容技术研究、SMB死锁修复 |
| 工部 | ✅ | GPU模块补全、编译修复 |
| 吏部 | ✅ | 版本号更新至v2.326.0 |
| 礼部 | ✅ | 竞品动态跟踪文档 |
| 刑部 | ✅ | 安全审计通过 |

---

## [v2.325.0] - 2026-03-30

### 🎯 六部协同开发第101轮 - 竞品学习深化(飞牛fnOS + 群晖DSM 7.3)

### 新增功能
- ✅ **GPU容器调度优化** - 多GPU资源池化与智能分配
  - GPU显存动态分配
  - 容器GPU独占/共享模式
  - GPU任务队列与优先级调度
- ✅ **Docker Compose增强** - 简化多容器应用部署
  - 一键部署模板库
  - 环境变量集中管理
  - 依赖关系可视化
- ✅ **SMB安全增强** - 企业级文件共享安全
  - SMB加密传输支持
  - 访问审计日志
  - IP白名单/黑名单
- ✅ **存储配额告警系统** - 空间管理智能化
  - 多级配额阈值告警
  - 用户/目录级配额
  - 历史趋势分析

### 竞品学习
- 🔍 **飞牛fnOS 1.1** 新功能分析
  - ARM架构支持42款设备 - 覆盖主流ARM开发板
  - AI人脸识别 - 本地化AI相册，无需云端
  - 元数据管理 - 文件标签与智能分类
  - QWRT路由系统 - NAS+路由一体化方案
- 🔍 **群晖DSM 7.3** 新功能分析
  - exFAT原生支持 - 无需付费授权
  - 第三方HDD解禁 - 取消硬盘限制
  - 硬盘兼容性放宽 - 支持更多消费级硬盘

### nas-os对标状态
| 功能 | 飞牛fnOS | 群晖DSM 7.3 | nas-os状态 |
|------|----------|-------------|------------|
| ARM支持 | ✅ 42款设备 | ❌ | ✅ RK3588优化 |
| AI人脸识别 | ✅ 本地化 | ✅ 私有云AI | 📋 P1规划 |
| exFAT支持 | ✅ | ✅ 原生 | ✅ 已支持 |
| 第三方HDD | ✅ | ✅ 解禁 | ✅ 无限制 |
| GPU调度 | 📋 | 📋 | ✅ 本次实现 |

### 六部协同
| 部门 | 状态 | 主要工作 |
|------|------|----------|
| 吏部 | ✅ | 版本号更新至v2.325.0、发行说明编写 |
| 兵部 | ✅ | GPU调度优化、Docker Compose增强 |
| 工部 | ✅ | SMB安全增强实现 |
| 礼部 | ✅ | 竞品学习文档更新 |
| 户部 | ✅ | 存储配额告警系统 |
| 刑部 | ✅ | 安全审计通过 |

---

## [v2.324.0] - 2026-03-30

### 🎯 六部协同开发第100轮 - 竞品学习深化(TrueNAS 25.10 Goldeye)

### 竞品学习
- 🔍 **TrueNAS 25.10 Goldeye** 新功能分析
  - NVMe over Fabric (NVMe/TCP + NVMe/RDMA) - 400GbE企业级网络存储
  - VM Secure Boot - 安全启动支持
  - VM Disk多格式导入导出 - QCOW2/QED/RAW/VDI/VHDX/VMDK
  - NVIDIA Open GPU Kernel Module - Blackwell架构GPU加速
  - ZFS Direct I/O - 虚拟化环境性能优化
  - Application Pool Migration - 应用池自动迁移
  - 灵活SMART监控 - 迁移到cron任务，支持Scrutiny App

### nas-os对标状态
| 功能 | TrueNAS 25.10 | nas-os状态 |
|------|---------------|------------|
| NVMe-oF | ✅ 企业级 | 📋 P2规划 |
| VM Secure Boot | ✅ | ✅ KVM支持 |
| VM多格式磁盘 | ✅ | ✅ 已支持 |
| NVIDIA GPU | ✅ Blackwell | 🚧 开发中 |
| ZFS Direct I/O | ✅ | 📋 P1研究 |
| 应用池迁移 | ✅ | ✅ 已支持 |

### 六部协同
| 部门 | 状态 | 主要工作 |
|------|------|----------|
| 吏部 | ✅ | 版本号更新至v2.324.0 |
| 礼部 | ✅ | CHANGELOG更新、竞品学习文档 |

---

## [v2.322.0] - 2026-03-30

### 🎯 六部协同开发第97轮 - RAIDZ Expansion竞品深化+用户文档

### 竞品学习深化
- 🔍 **TrueNAS RAIDZ Expansion**: 单盘在线扩展RAID-Z阵列
  - OpenZFS 2.3正式支持，保持原有冗余级别
  - 扩容速度提升5-10倍（TrueNAS Fangtooth优化）
  - 支持中断恢复，数据自动重分布
  - 开发投入：约3年，$100,000，核心开发者Matt Ahrens
- 🔍 **飞牛fnOS**: 无RAIDZ扩展支持，依赖重建池扩容
- 🔍 **nas-os规划**: P0优先级，btrfs RAID1/RAID10优化封装

### 文档更新
- `docs/COMPETITOR_ANALYSIS.md` - 新增RAIDZ功能对比分析章节
- `docs/user-guide/raidz-expansion-guide.md` - 用户文档框架创建

### 六部协同
| 部门 | 状态 | 主要工作 |
|------|------|----------|
| 礼部 | ✅ | CHANGELOG更新、竞品分析深化、用户文档框架 |

---

## [v2.321.0] - 2026-03-30

### 🎯 六部协同开发第96轮 - NVMe SMART监控+竞品学习深化

### 新增功能
- ✅ **NVMe S.M.A.R.T.健康监控** - 对标TrueNAS/群晖SSD监控
  - 设备自动发现与状态检测
  - 温度、寿命、备用空间实时监控
  - 多级告警机制（warning/critical）
  - Prometheus指标导出
  - Dashboard看板数据支持

### 竞品学习深化
- 🔍 **TrueNAS NVMe SMART**: UI测试界面、健康状态可视化
- 🔍 **群晖SSD健康**: 寿命预测、温度监控、告警集成
- 🔍 **飞牛fnOS**: 硬件健康中心设计参考

### 文档更新
- `docs/research/competitor-analysis-2026-03-29.md` - 新增NVMe SMART功能对比
- `docs/nvme-smart-guide.md` - 功能说明文档框架

### 六部协同
| 部门 | 状态 | 主要工作 |
|------|------|----------|
| 兵部 | ✅ | NVMe SMART监控实现 (`internal/hardware/nvme/monitor.go`) |
| 工部 | ✅ | 编译验证、依赖检查 |
| 礼部 | ✅ | CHANGELOG更新、竞品分析深化、功能文档框架 |
| 刑部 | ✅ | 安全审计通过 |
| 户部 | ✅ | 资源统计 |

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