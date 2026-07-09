# NAS-OS 资源统计

**更新日期**: 2026-07-09
**版本**: v3.16.0

## 代码规模

| 指标 | 数值 |
|------|------|
| 内部模块数 | 755 |
| Go 源文件数 | 3704 |
| 测试文件数 | 934 |
| Go 代码总行数 | 1,639,342 |
| go.mod 依赖数 | 172 |

## v3.16.0 新增模块

| 部门 | 模块 | 对标竞品 | 核心功能 |
|------|------|----------|----------|
| 兵部 | ssdcacheschedule | Synology SSD Cache Advisor / TrueNAS L2ARC | 缓存命中率分析、磨损预警、温度监控、读写放大治理 |
| 户部 | cloudcostaudit | Synology CloudRep / TrueNAS 云同步 | 预算超支、休眠账户、出口流量、生命周期分层、R2 迁移 |
| 礼部 | posterscraper | Synology Video Station / 飞牛影视墙 | 批量海报缺失、解析失败、字幕获取、低置信度审核 |
| 工部 | powermanager | Synology Power Schedule / TrueNAS | 空闲切换、待机调度、磁盘转速、夜间计划、太阳能对齐 |
| 吏部 | clusterops | Synology CMS / TrueNAS Cluster | HA 启用、故障转移、脑裂检测、复制延迟、集群健康 |
| 刑部 | datasovereigntyaudit | GDPR / PIPL / HIPAA | PII 加密、跨境复制、访问日志、保留策略、DPA 审计 |

## 竞品参考

- **Synology DSM 7.4**: Website AI Advisor、ActiveProtect、SSD Cache Advisor、Power Schedule、CMS
- **TrueNAS 26**: L2ARC tiering、Cloud Sync cost tracking、Cluster、TrueSearch AI
- **飞牛 fnOS**: 影视墙刮削、家庭影音体验、远程访问、多设备管理
