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

## 本轮开发重点

基于竞品学习，本轮优先实现：
1. RAIDZ 扩展技术研究（文档）
2. 存储效率功能对比分析
3. 全局搜索服务架构设计