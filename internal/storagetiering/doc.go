// Package storagetiering 智能存储分层引擎
//
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// NAS-OS 智能存储分层系统 v2.0.0
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
//
// 【概述】
//
// 本模块实现 NAS-OS 的智能存储分层引擎，参考群晖 DSM 7.3 智能存储分层
// 与 TrueNAS 26 的 OpenZFS 2.4 混闪池设计，提供细粒度的数据生命周期管理。
//
// 相较于 internal/tiering 模块，本模块新增：
//   - NVMe/SSD/HDD/Archive 四级分层体系
//   - 基于访问频率、修改时间、文件大小的复合分層策略
//   - 热数据自动提升 (Hot Promote) 与冷数据自动降级 (Cold Demote)
//   - DataPlacementAdvisor 智能数据放置建议
//   - ZFS 专用策略 (L2ARC/SLOG/HybridPool/压缩/去重)
//
// 【核心组件】
//
// 1. TierManager (tiering.go)
//   - 四级分层的核心管理器
//   - 分层规则与数据迁移调度
//   - Hot Promote / Cold Demote 自动化
//
// 2. DataPlacementAdvisor (tiering.go)
//   - 基于多维特征的智能放置建议
//   - 访问模式预测与预取推荐
//
// 3. ZFSPolicyManager (zfspolicy.go)
//   - L2ARC 缓存策略配置
//   - SLOG 写入缓存配置
//   - ZFS 压缩/去重建议
//   - HybridPool 配置（TrueNAS 混闪池模式）
//
// 4. Handler (handler.go)
//   - RESTful API 端点
//   - 策略 CRUD 与迁移触发
//   - 数据放置建议查询
//
// 【存储层级】
//
//	Hot     (NVMe/SSD) — 高频访问数据，延迟 <1ms
//	Warm    (SSD)      — 中频访问数据，延迟 <10ms
//	Cold    (HDD)      — 低频访问数据，延迟 <20ms
//	Archive (磁带/云)  — 归档数据，访问频率极低
//
// 【API 端点】
//
//	GET  /api/v1/tiering/policies          获取分层策略
//	POST /api/v1/tiering/policies          创建策略
//	GET  /api/v1/tiering/status            分层状态
//	POST /api/v1/tiering/migrate           手动触发迁移
//	GET  /api/v1/tiering/recommendations   数据放置建议
//
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
package storagetiering
