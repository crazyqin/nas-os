# 竞品学习报告 - 2026-03-29

## TrueNAS Scale 24.10 (Electric Eel)

### 新功能亮点
1. **RAIDZ VDEV 扩展** ⭐
   - 支持逐盘扩展 RAIDZ 阵列，无需重建
   - 保持数据冗余
   - 扩展期间存储池可访问
   - 断电/重启后可恢复
   - 扩展后可多次扩展

2. **Docker 替代 Kubernetes**
   - App 后端从 Kubernetes 迁移到 Docker
   - 支持 Compose YAML 部署

3. **TrueCloud Backup Tasks**
   - Storj iX 云备份集成

4. **全局搜索**
   - UI 页面和设置搜索

5. **Dashboard 重构**
   - 更多 widgets
   - 数据报告增强
   - 自定义功能

6. **NVMe S.M.A.R.T. 测试**
   - UI 支持 NVMe SMART 测试

7. **ZFS Fast Deduplication**
   - OpenZFS 快速去重（实验性）

8. **SMB Alternate Data Streams**
   - 从远程服务器摄取数据时保留

## Synology DSM 7.x

### 核心功能
1. **Synology Tiering** ⭐
   - 高性能存储优化和扩展
   - 分层存储

2. **Hybrid Share**
   - 本地存储 + 可扩展云存储

3. **Presto**
   - 文件传输加速

4. **Hyper Backup**
   - 灵活目标的全面备份方案

5. **Snapshot Replication**
   - 时间点备份 + 复制策略

6. **Active Backup 系列**
   - Business：物理/虚拟环境
   - Microsoft 365
   - Google Workspace

7. **Virtual Machine Manager**
   - VM 部署、集群管理

8. **Storage Manager**
   - 集中存储管理界面

## 飞牛 fnOS

### 特色功能
1. **软路由集成** ⭐
   - NAS + 路由一体化

2. **Cloudflare Tunnel**
   - 无公网 IP 访问

---

## nas-os 差距分析与优先级

### P0 - 必须实现
- [ ] RAIDZ 扩展支持（TrueNAS 已有）
- [ ] 全局搜索（TrueNAS 已有）
- [ ] NVMe S.M.A.R.T. 监控

### P1 - 重要功能
- [ ] 分层存储（Synology Tiering）
- [ ] 软路由集成（飞牛特色）
- [ ] Cloudflare Tunnel（飞牛特色）
- [ ] Docker Compose 应用部署

### P2 - 增强功能
- [ ] Dashboard 重构
- [ ] 快速去重（实验性）
- [ ] 云备份集成

---

## NVMe S.M.A.R.T. 功能对比 (2026-03-30 更新)

### TrueNAS Scale 24.10
- ✅ NVMe SMART测试UI界面
- ✅ 健康状态可视化
- ✅ 温度、寿命、备用空间监控
- ✅ 媒体错误计数
- ✅ 集成到Dashboard

### Synology DSM 7.x
- ✅ SSD健康监控
- ✅ 寿命预测（百分比显示）
- ✅ 温度监控与告警
- ✅ 存储管理器集成
- ✅ 邮件/通知告警

### 飞牛 fnOS
- ✅ 硬件健康中心
- ✅ NVMe/SSD状态显示
- ✅ 温度监控
- ⚠️ SMART详情较少

### nas-os (v2.321.0)
- ✅ NVMe设备自动发现
- ✅ SMART数据采集（温度、寿命、备用空间）
- ✅ 多级告警机制
- ✅ Prometheus指标导出
- ✅ Dashboard看板支持
- 📋 UI界面（规划中）
- 📋 历史数据存储（规划中）

### 功能优先级建议
| 功能 | TrueNAS | 群晖 | nas-os | 优先级 |
|------|---------|------|--------|--------|
| 基础SMART监控 | ✅ | ✅ | ✅ | P0 |
| 温度告警 | ✅ | ✅ | ✅ | P0 |
| 寿命预测 | ✅ | ✅ | ✅ | P0 |
| UI测试界面 | ✅ | ✅ | 📋 | P1 |
| 历史趋势图 | ⚠️ | ✅ | 📋 | P1 |
| 多设备聚合视图 | ✅ | ✅ | 📋 | P1 |
| 自动化定期检测 | ✅ | ✅ | ✅ | P0 |

---

## 本轮开发重点

基于竞品学习，本轮优先实现：
1. RAIDZ 扩展技术研究（文档）
2. 存储效率功能对比分析
3. 全局搜索服务架构设计