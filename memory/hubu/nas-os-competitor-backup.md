# Synology Active Backup 竞品分析报告

> **版本**: v1.0.0  
> **日期**: 2026-04-16  
> **作者**: 户部（财务运营）  
> **对标版本**: Synology Active Backup for Business 2.7.x

---

## 1. Synology Active Backup for Business 功能全景

### 1.1 核心功能

| 功能分类 | 功能项 | 说明 |
|---------|--------|------|
| **备份源** | Windows PC/Server Agent | 安装代理，备份整机/卷/文件/分区 |
| | Linux Agent | 同上，支持主流发行版 |
| | macOS Agent | Time Machine 替代方案 |
| | VMware vSphere | 无代理备份虚拟机 |
| | Hyper-V | 无代理备份虚拟机 |
| | 文件服务器 | SMB/NFS 协议备份文件服务器 |
| | 物理服务器 | 裸机备份与恢复 |
| **备份类型** | 完整备份 | 全量数据备份 |
| | 增量备份 | 基于CBT（Changed Block Tracking） |
| | 差异备份 | 相对上次全量的变化 |
| **存储管理** | 全局去重 | 跨设备块级去重 |
| | 压缩 | LZ4/Zstd 可选 |
| | 加密 | AES-256 加密传输和存储 |
| | 保留策略 | 自定义保留版本数和天数 |
| **恢复** | 裸机恢复 (Bare Metal) | 通过ISO启动恢复 |
| | 文件级恢复 | 从整机备份中提取单文件 |
| | 即时虚拟机恢复 | 直接在ESXi上启动备份虚拟机 |
| | 异地恢复 | 远程站点恢复 |
| **调度** | 计划备份 | Cron式调度 |
| | 备份窗口 | 避开业务高峰 |
| | 并发控制 | 限制同时备份任务数 |
| **监控** | 仪表盘 | 全局备份状态一览 |
| | 邮件通知 | 成功/失败/告警 |
| | 审计日志 | 操作记录追踪 |
| **高级** | 不可变备份 | WORM防勒索保护 |
| | 多目标备份 | 同一任务备份到多个目标 |
| | 带宽控制 | 限速传输 |
| | 断点续传 | 网络中断后继续 |

### 1.2 关键技术特性

| 特性 | 实现 |
|------|------|
| 去重粒度 | 块级（4KB-64KB可变） |
| 增量技术 | CBT + RCT (VMware) / RCT (Hyper-V) |
| 传输协议 | TLS 1.3 |
| 最大设备数 | 无限制（依赖NAS性能） |
| 存储效率 | 去重+压缩节省60-80%空间 |
| RPO | 最短15分钟 |

---

## 2. nas-os 现有功能映射

### 2.1 已实现功能

| Active Backup功能 | nas-os实现 | 模块路径 | 成熟度 |
|------------------|-----------|---------|--------|
| 完整备份 | ✅ `advanced.Manager.executeFullBackup` | `internal/backup/advanced/manager.go` | 成熟 |
| 增量备份 | ✅ `IncrementalIndex` 基于校验和+时间戳 | `internal/backup/advanced/types.go` | 成熟 |
| 差异备份 | ✅ `executeDifferentialBackup` | `internal/backup/advanced/manager.go` | 基础 |
| 压缩(gzip/zstd/lz4/bzip2/xz) | ✅ `Compressor` 接口 | `internal/backup/advanced/compression.go` | 成熟 |
| AES-256加密 | ✅ `AES256Encryptor` (GCM模式) | `internal/backup/advanced/types.go` | 成熟 |
| Cron调度 | ✅ `Scheduler` (robfig/cron) | `internal/backup/scheduler.go` | 成熟 |
| 优先级队列 | ✅ `PriorityQueue` | `internal/backup/scheduler.go` | 成熟 |
| 备份窗口 | ✅ `BackupWindow` | `internal/backup/scheduler.go` | 成熟 |
| 并发控制 | ✅ `Semaphore` 信号量 | `internal/backup/scheduler.go` | 成熟 |
| 重试机制 | ✅ 指数退避重试 | `internal/backup/scheduler.go` | 成熟 |
| 成本分析 | ✅ `CostAnalyzer` | `internal/backup/cost_analyzer.go` | 成熟 |
| 多云支持 | ✅ S3/Aliyun/WebDAV | `internal/backup/cloud.go` | 成熟 |
| 云同步 | ✅ `cloudsync.Manager` | `internal/cloudsync/` | 成熟 |
| 版本管理 | ✅ `VersioningConfig` | `internal/backup/smart_manager.go` | 基础 |
| 验证/校验 | ✅ `Verification` SHA-256 | `internal/backup/advanced/verification.go` | 成熟 |
| S3备份 | ✅ aws-sdk-go-v2 | `internal/backup/sync.go` | 成熟 |
| WebDAV备份 | ✅ gowebdav | `internal/backup/sync.go` | 成熟 |
| 邮件告警 | ✅ notification模块 | `internal/notification/` | 基础 |
| RAIDZ存储池 | ✅ `RAIDZExpansionService` | `internal/storage/raidz_service.go` | 成熟 |
| 不可变存储 | ✅ `immutable` | `internal/storage/immutable.go` | 基础 |

### 2.2 未实现/差距功能

| Active Backup功能 | nas-os状态 | 优先级 | 开发成本 |
|------------------|-----------|--------|---------|
| **Windows/Linux/macOS Agent** | ❌ 无客户端代理 | P0 | 高（需跨平台Agent） |
| **VMware无代理备份** | ❌ 无虚拟化集成 | P1 | 高（需vSphere API） |
| **Hyper-V无代理备份** | ❌ 无虚拟化集成 | P2 | 高 |
| **块级去重（全局）** | ⚠️ 仅有文件级增量 | P0 | 中（需块级索引） |
| **CBT变更块追踪** | ❌ 无块级追踪 | P1 | 高 |
| **裸机恢复(BMR)** | ❌ 无恢复ISO生成 | P1 | 高（需WinPE/Linux Live） |
| **即时虚拟机恢复** | ❌ 无 | P2 | 极高 |
| **文件级恢复（从整机）** | ❌ 无 | P1 | 中 |
| **带宽控制/限速** | ⚠️ 云同步有限速，备份无 | P1 | 低 |
| **断点续传** | ⚠️ 基础实现 | P1 | 低 |
| **多目标备份** | ❌ 单目标 | P2 | 中 |
| **WORM不可变备份** | ⚠️ 基础实现 | P1 | 低 |
| **全局仪表盘** | ⚠️ 基础dashboard | P2 | 中 |
| **多站点/异地容灾** | ❌ 无 | P3 | 高 |

### 2.3 功能覆盖率统计

| 类别 | 总功能 | 已实现 | 部分实现 | 未实现 | 覆盖率 |
|------|--------|--------|---------|--------|--------|
| 备份源 | 6 | 1(文件) | 1(云) | 4 | 25% |
| 备份类型 | 3 | 3 | 0 | 0 | 100% |
| 存储管理 | 4 | 3 | 1 | 0 | 88% |
| 恢复 | 4 | 1 | 0 | 3 | 25% |
| 调度 | 3 | 3 | 0 | 0 | 100% |
| 监控 | 3 | 2 | 1 | 0 | 83% |
| 高级 | 5 | 1 | 2 | 2 | 40% |
| **总计** | **28** | **14** | **5** | **9** | **61%** |

---

## 3. 差异化功能规划建议

### 3.1 P0（核心差异化，6个月内）

#### 3.1.1 nas-os Agent 客户端

```
目标：Windows/Linux客户端，支持文件/目录/整机备份

技术方案：
- Go跨平台Agent，gRPC通信
- 支持文件级和卷级备份
- 后台服务常驻，低资源占用
- 首期：Windows + Linux文件级备份

预计工时：120人天
依赖：gRPC框架、跨平台编译
```

#### 3.1.2 块级去重引擎

```
目标：跨设备块级去重，存储效率提升60%+

技术方案：
- 固定/可变块切分（CDC算法）
- SHA-256指纹索引
- 参考现有dedup模块（internal/dedup/）
- 集成到advanced.Manager

预计工时：40人天
依赖：internal/dedup/模块扩展
```

### 3.2 P1（增强竞争力，12个月内）

#### 3.2.1 VMware/Hyper-V 无代理备份

```
目标：虚拟化平台无代理备份

技术方案：
- VMware: vSphere API (govmomi)
- Hyper-V: WMI/PowerShell Remote
- CBT增量备份
- 虚拟机配置导出

预计工时：90人天
依赖：govmomi库、WMI远程调用
```

#### 3.2.2 裸机恢复

```
目标：生成恢复ISO，支持裸机恢复

技术方案：
- Linux Live CD定制（Alpine/Debian）
- WinPE恢复环境
- 网络恢复模式
- USB启动盘生成

预计工时：60人天
依赖：Live CD构建工具
```

#### 3.2.3 带宽控制与断点续传增强

```
目标：细粒度带宽控制，可靠断点续传

技术方案：
- Token Bucket限速
- 块级断点续传状态持久化
- 网络质量自适应
- 热备份限速（不影响业务IO）

预计工时：20人天
依赖：已有sync模块扩展
```

### 3.3 P2（生态扩展，18个月内）

- 多目标备份（本地+云双写）
- 即时虚拟机恢复（直接挂载备份运行）
- 全局备份仪表盘增强
- 多站点异地容灾编排

### 3.4 nas-os独有优势

相比Synology Active Backup，nas-os有：

| 优势 | 说明 |
|------|------|
| **开源自托管** | 无许可证费用，数据完全自主 |
| **成本透明** | 内置CostAnalyzer，实时成本追踪 |
| **RAIDZ弹性** | 支持在线扩容，无需重建 |
| **多云后端** | 不绑定特定云厂商 |
| **可编程** | REST API全覆盖，支持自动化 |
| **不可变存储** | 内置防勒索保护 |

---

## 4. 成本对比

### 4.1 许可证成本

| 项目 | Synology ABB | nas-os |
|------|-------------|--------|
| 软件许可 | 免费（需群晖NAS） | 免费（开源自托管） |
| 硬件绑定 | 必须群晖NAS | 任意x86/ARM硬件 |
| Agent数量 | 无限制 | 无限制 |
| 功能限制 | 需购买NAS设备 | 无限制 |
| **3年TCO** | ¥3,000-30,000+ | ¥0（仅硬件成本） |

### 4.2 硬件成本

| 配置 | Synology | 自建nas-os |
|------|---------|-----------|
| 4盘位 | ¥4,000-8,000 | ¥2,000-4,000 |
| 8盘位 | ¥8,000-15,000 | ¥4,000-8,000 |
| 12盘位 | ¥15,000-30,000 | ¥6,000-12,000 |

**结论**：nas-os在硬件成本上节省约50%。

---

## 5. 路线图

```
2026 Q2 (当前)
├── 块级去重引擎设计
├── Agent协议定义
└── 带宽控制增强

2026 Q3
├── Linux Agent Alpha
├── 块级去重MVP
└── VMware无代理备份调研

2026 Q4
├── Windows Agent Alpha
├── 去重+压缩集成
└── 裸机恢复原型

2027 Q1
├── Agent Beta (Win+Linux)
├── VMware无代理备份Alpha
└── 多目标备份

2027 Q2+
├── 即时虚拟机恢复
├── 多站点容灾
└── AI智能备份策略
```

---

**报告完成 | 户部（财务运营） | 2026-04-16**
