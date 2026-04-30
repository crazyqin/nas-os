# NAS-OS 竞品动态更新 — 2026年5月

> **更新日期**: 2026-05-01  
> **调研范围**: TrueNAS 26 Beta / TrueNAS V160 / 群晖 DSM 7.3 / 飞牛 fnOS

---

## 一、TrueNAS 26 Beta（2026年4月发布）

### 核心更新
| 特性 | 说明 | nas-os 对标状态 |
|------|------|:--------------:|
| **OpenZFS 2.4 混合闪存池** | NVMe+HDD分层存储，Flash层同时作为ZIL/SLOG，10x性能提升 | ⚠️ 有tiering基础，需增强 |
| **Guided Alerts 引导告警** | 每条告警附带排查步骤和修复引导，菜单指示器引导用户 | ⚠️ 有remediation引擎，需增强 |
| **WebShare + TrueSearch** | 浏览器文件共享 + 亚秒级搜索，支持macOS Spotlight | ✅ 已有webshare+truesearch |
| **LXC容器支持** | 轻量级Linux容器部署 | ✅ 已有LXC模块 |
| **定时Scrub避峰** | 避开业务高峰时段执行Scrub | ⚠️ 有基础调度，需增加避峰 |
| **简化用户管理** | 减少点击数和滚动，更直观的用户管理 | 📋 待开发 |
| **Fast Dedup增强** | 去重性能进一步提升 | ⚠️ 有dedup模块，需优化 |
| **NVIDIA 590.48驱动** | 支持RTX 5050等新卡 | ✅ GPU模块已有基础 |
| **TrueNAS V160硬件** | 400GbE、768GB DDR5、24TiB混合缓存、20PiB容量 | N/A 硬件产品 |

### 重要趋势
- **年度发布节奏**: TrueNAS从半年发布改为年度发布，质量优先
- **混合存储回归**: NAND涨价300%推动混合闪存+HDD方案
- **企业特性下沉**: Enterprise功能通过TrueNAS Connect向社区版开放

---

## 二、TrueNAS V160 企业硬件

| 参数 | 规格 |
|------|------|
| 原始容量 | 20PiB NVMe / 35PiB HDD |
| 吞吐量 | 60GB/s |
| 网络 | 400GbE / 32Gb FC |
| 混合缓存 | 24TiB |
| 内存 | 768GB DDR5 |
| 可用性 | >99.999% |

**关键设计理念**: 混合存储不再是妥协，而是最优解。Flash做热数据+元数据+ZIL/SLOG，HDD做冷数据归档。

---

## 三、nas-os v2.479.0 开发方向

基于竞品分析，以下功能需优先开发：

1. **混合闪存池智能分层** - 对标OpenZFS 2.4 Hybrid Flash Pool
2. **引导式告警增强** - 对标TrueNAS 26 Guided Alerts
3. **Scrub智能避峰调度** - 对标TrueNAS 26 Scheduled Scrub
4. **存储成本优化分析** - 差异化优势，竞品均无此功能
5. **用户权限模板系统** - 对标群晖简化用户管理
6. **WORM合规报告** - 差异化优势
