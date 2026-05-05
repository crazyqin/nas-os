# NAS-OS 用户指南索引

> **更新日期**: 2026-05-05 | **适用版本**: v2.483.0

---

## 🚀 快速入门

新用户按以下顺序阅读，10 分钟上手 NAS-OS：

1. [监控仪表板](dashboard-guide.md) — 了解系统状态和健康评分
2. [Fusion Pool 存储分层](fusion-pool-guide.md) — 配置存储池
3. [DriveSync 文件同步](DriveSync.md) — 多设备文件同步
4. [整机备份与灾难恢复](backup-disaster-recovery-guide.md) — 数据安全保障
5. [TrueSearch 全文搜索](true-search.md) — 快速找到任何文件

## 🔧 常见操作速查

| 我想要… | 查看指南 |
|---------|----------|
| 远程访问家里的电脑 | [远程桌面网关](remote-desktop-guide.md) |
| 从外网安全访问 NAS | [VPN Server](vpn-server-guide.md) / [NAT 穿透](natpierce.md) |
| 自托管代码仓库 | [Git Server](git-server-guide.md) |
| 听音乐 | [Audio Station](audio-station-guide.md) |
| 搜索文件 | [TrueSearch 全文搜索](true-search.md) |
| 同步多台设备文件 | [DriveSync](DriveSync.md) |
| 监控硬盘健康 | [NVMe 健康监控](nvme-health-guide.md) / [Scrub 调度](scrub-scheduling-guide.md) |
| 智能存储分层 | [Smart Tier 智能分层](smart-tier-guide.md) |
| 资源耗尽预测 | [资源预测告警](resource-prediction-guide.md) |
| 加密敏感数据 | [加密存储](encryption-guide.md) |
| 设置存储配额 | [智能配额与数据保留](quota-retention-guide.md) |
| 管理预算和成本 | [预算告警管理](budget-alert-guide.md) |
| 安全合规检查 | [合规仪表盘](compliance-dashboard-guide.md) |
| 隔离运行应用 | [LXC 容器沙箱](lxc-sandbox-guide.md) |
| 整理家庭照片 | [人脸识别与人物相册](face-recognition-guide.md) |
| 自定义监控面板 | [家庭仪表盘](home-dashboard-guide.md) |
| AI 对话隐私保护 | [AI Console 隐私脱敏](ai-console-guide.md) |

---

## 存储与数据管理

| 指南 | 说明 |
|------|------|
| [Fusion Pool 存储分层](fusion-pool-guide.md) | SSD/HDD 智能分层存储 |
| [RAIDZ 扩容指南](raidz-expansion-guide.md) | 在线扩展 RAIDZ 阵列 |
| [智能配额与数据保留](quota-retention-guide.md) | 配额管理、数据生命周期策略 |
| [S3 对象存储网关](s3-gateway-guide.md) | S3 兼容对象存储服务 |
| [NVMe 健康监控](nvme-health-guide.md) | NVMe SSD 状态监控与预警 |
| [磁盘电源管理](disk-power-guide.md) | 硬盘休眠与节能策略 |
| [Scrub 智能调度](scrub-scheduling-guide.md) | 数据校验避峰调度 |
| [加密存储](encryption-guide.md) | Vault 加密卷 / 文件夹级加密 / AES-256-GCM |
| [智能数据迁移](smart-migrate-guide.md) | 跨存储池迁移 / SHA-256 校验 / 带宽控制 |
| [Smart Tier 智能分层](smart-tier-guide.md) | I/O 模式感知 / 预取预测 / 自适应阈值 / 批量迁移 |
| [资源预测告警](resource-prediction-guide.md) | 线性回归预测 / 四级告警 / 置信度评分 |

## 数据同步与备份

| 指南 | 说明 |
|------|------|
| [DriveSync 文件同步](DriveSync.md) | 多平台文件同步 |
| [整机备份与灾难恢复](backup-disaster-recovery-guide.md) | 系统级备份与恢复 |
| [TrueSearch 全文搜索](true-search.md) | 亚秒级文件搜索 |

## 应用与服务

| 指南 | 说明 |
|------|------|
| [Audio Station 音乐中心](audio-station-guide.md) | 音乐管理与在线播放 |
| [监控仪表板](dashboard-guide.md) | 自定义监控与健康评分 |
| [分布式监控](distributed-monitoring-guide.md) | 多节点统一监控 |
| [Smart Cron 定时任务](smart-cron.md) | 智能定时任务管理 |

## 网络与远程访问

| 指南 | 说明 |
|------|------|
| [VPN Server](vpn-server-guide.md) | WireGuard/OpenVPN VPN 服务配置 |
| [Git Server](git-server-guide.md) | 自托管 Git 仓库管理 |
| [远程桌面网关](remote-desktop-guide.md) | 浏览器 RDP/VNC 远程访问 |
| [NAT 穿透](natpierce.md) | 外网访问 NAS |
| [NAT 隧道](../USER_GUIDE_NAT_TUNNEL.md) | Cloudflare Tunnel 配置 |

## 系统管理与安全

| 指南 | 说明 |
|------|------|
| [LXC 容器沙箱](lxc-sandbox-guide.md) | 轻量级隔离环境 / 内置模板 / 资源限制 |
| [无 Root 管理员](rootless-admin-guide.md) | 命令白名单 / 细粒度权限 / 审计日志 |
| [合规仪表盘](compliance-dashboard-guide.md) | CIS/STIG/GDPR 合规检查 / 安全评分 |
| [预算告警管理](budget-alert-guide.md) | 多级预算 / 三级告警 / 成本分析 |
| [家庭仪表盘](home-dashboard-guide.md) | 可配置Widget / 多布局 / Widget市场 / WebSocket实时刷新 |
| [AI Console 隐私脱敏](ai-console-guide.md) | 多模型管理 / PII自动脱敏 / 审计日志 |
| [人脸识别与人物相册](face-recognition-guide.md) | 人脸检测 / 特征提取 / 智能聚类 / Intel QSV加速 |

---

## 📖 文档导航提示

- **[NAT 穿透](natpierce.md)** 与 **[VPN Server](vpn-server-guide.md)** 配合使用，实现安全外网访问
- **[远程桌面网关](remote-desktop-guide.md)** 配合 **[VPN](vpn-server-guide.md)** 可实现加密远程办公
- **[Git Server](git-server-guide.md)** 的 Webhook 可触发 **[Smart Cron](smart-cron.md)** 自动化任务
- **[LXC 容器沙箱](lxc-sandbox-guide.md)** 适合在隔离环境中测试应用，不影响主系统

- **[人脸识别](face-recognition-guide.md)** 配合 **[AI 相册](../photos-guide.md)** 实现智能照片整理
- **[AI Console](ai-console-guide.md)** 为所有 AI 功能提供隐私脱敏保护
- **[家庭仪表盘](home-dashboard-guide.md)** 是统一监控入口，聚合各模块数据

---

*索引版本：v2.483.0 | 最后更新：2026-05-06*
