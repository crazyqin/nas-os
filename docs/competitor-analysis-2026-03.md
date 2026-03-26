# 竞品分析报告 - 2026年3月（更新）

## 群晖 Synology DSM 7.3

### 核心优势
1. **完整生态套件**：Photos、Drive、Cloud Sync、Office、MailPlus、Chat 等完整协作套件
2. **企业级功能**：
   - Synology Tiering 智能分层存储（DSM 7.3新增：跨设备热/冷数据分层）
   - Hybrid Share 混合云存储
   - Hyper Backup 多目标备份
   - Active Backup for Business/M365/Google Workspace
3. **高可用性**：Synology High Availability 主备集群
4. **安全认证**：Secure SignIn 多因素认证

### DSM 7.3 新特性（2026）
- **智能分层存储**：自动将30天未访问文件从热存储转移到冷存储
- **跨设备分层**：支持在两台Synology设备之间进行数据分层
- **BTRFS必需**：分层功能需要BTRFS文件系统支持

### 值得学习
- [x] 完整的协作办公套件集成
- [x] 企业级备份方案（M365/Google Workspace）
- [x] 分层存储优化策略（已实现智能分层）
- [ ] 跨设备分层架构
- [ ] 混合云存储架构

---

## TrueNAS Scale 25.04 Fangtooth / 25.10 Goldeye

### 核心优势
1. **ZFS 原生支持**：
   - RAID-Z Expansion 在线扩容（5X加速）
   - 无限快照、池检查点
   - 自愈校验和
   - dRAID 支持
   - Fast Deduplication 快速去重
2. **应用生态**：
   - Docker/KVM/LXC 容器支持（25.04引入LXC）
   - GPU 共享、沙箱隔离
   - 应用目录、批量更新
3. **企业存储**：
   - SMB Multichannel 多通道
   - iSCSI/NFS RDMA 加速（性能提升40-50%）
   - MinIO 对象存储
   - Fibre Channel 光纤通道
4. **高可用**：
   - 双控高可用
   - 1200盘扩展能力
   - 快速故障转移

### TrueNAS 26 计划（2026）
- **年度发布周期**：简化版本号（26.1, 26.2等）
- **Linux Kernel 6.18 LTS**
- **LXC容器完全支持**：为CORE用户提供Jail迁移路径
- **OpenZFS 2.4**：混合池改进
- **勒索软件检测防护**
- **TrueNAS Webshare**：集成搜索功能

### 值得学习
- [x] RAID-Z Expansion（已实现）
- [x] LXC 容器支持（已实现基础框架）
- [ ] GPU 共享机制
- [x] SMB Multichannel（已实现基础）
- [x] MinIO 对象存储集成（已实现）
- [ ] Fast Deduplication
- [ ] RDMA加速（iSER/NFS over RDMA）

---

## 飞牛 fnOS v1.1.x

### 定位
国产化 NAS 系统，主打易用性和本地化，免费开源

### 最新特性（2026年2-3月）
1. **ARM架构公测版**：
   - 首批适配42款设备
   - 支持瑞芯微RK3XXX、Amlogic S905x、全志平台
   - 支持苹果M系列虚拟化环境
   - 支持飞腾、华为鲲鹏等国产ARM平台

2. **VPU硬件加速**：
   - ARM平台视频解码硬件加速
   - 降低CPU负载、减少发热

3. **外设适配**：
   - USB接口2.5G/5G/10G网卡支持
   - WiFi 4/5/6/7无线芯片适配（180+款）
   - 蓝牙协议栈支持

4. **核心功能**：
   - SMB/WebDAV/FTP/NFS多协议共享
   - 手机照片自动增量备份
   - 飞牛影视：自动刮削、海报墙、4K串流
   - DLNA服务
   - 防火墙
   - AES-256加密 + 双因素认证

5. **低功耗**：整机待机约52W，年电费约300元

### 值得学习
- [ ] ARM平台广泛适配
- [ ] VPU硬件解码加速
- [ ] 手机端自动备份体验
- [ ] 影视刮削准确率（99%）
- [ ] 低功耗优化

---

## 差距分析与优先级

| 功能 | nas-os 现状 | 群晖 | TrueNAS | 飞牛 | 优先级 |
|------|------------|------|---------|------|--------|
| RAID-Z Expansion | ✅ 已实现 | N/A | ✅ 5X加速 | ❌ | - |
| SMB Multichannel | ✅ 已实现 | ✅ | ✅ | ⚠️ | - |
| LXC 容器 | ✅ 已实现 | ❌ | ✅ | ❌ | - |
| GPU 共享 | ⚠️ 基础 | ❌ | ✅ | ❌ | P1 |
| 对象存储 MinIO | ✅ 已实现 | ✅ | ✅ | ❌ | - |
| RDMA加速 | ⚠️ 基础 | ❌ | ✅ 40%+ | ❌ | P1 |
| 智能分层 | ✅ 已实现 | ✅ | ✅ | ❌ | - |
| ARM支持 | ❌ | ⚠️ | ❌ | ✅ 42款 | P2 |
| 网盘挂载 | ✅ 已实现 | ✅ | ❌ | ✅ | - |
| 影视管理 | ⚠️ 基础 | ✅ | ❌ | ✅ 99% | P1 |

---

## 下一步计划

### P0 - 本月（已完成部分）
1. ✅ SMB Multichannel 多通道优化
2. ✅ MinIO 对象存储深度集成
3. ✅ 网盘挂载功能（百度/阿里/123云盘）
4. ✅ 内网穿透frp集成

### P1 - 下季度
1. RDMA加速（iSER/NFS over RDMA）
2. GPU 调度与共享机制
3. 影视刮削优化
4. 混合云存储架构

### P2 - 未来
1. ARM平台适配
2. VPU硬件加速
3. 协作办公套件（文档、表格、演示）
4. 企业邮箱集成