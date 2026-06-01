## v2.548.0 (2026-06-01)

### 新功能
- 合规扫描器（compliancescanner）：自动化合规检查、规则引擎、修复建议、合规报告生成
- dRAID分布式RAID（draid）：分布式热备、动态重建、性能优化、容错管理
- 功率预算管理（powerbudget）：功耗实时监控、预算告警、能耗分析、节能优化
- 智能带宽预测（smartbandwidthpredict）：流量模式学习、带宽预测、QoS调度、异常检测
- 合规审计（complianceaudit）：多标准合规评估、审计日志、整改跟踪、合规报告
- 成本优化器（costoptimizer）：存储成本分析、去重压缩、成本优化建议、ROI计算
- 存储分层（storagetiering）：智能数据分层、冷热迁移、性能优化、容量规划
- WAN链路规划（wanreplanner）：链路监控、故障切换、负载均衡、QoS管理

### 改进
- 修复 filecache CleanupInterval 为零时 NewTicker panic
- 更新 VERSION 至 v2.548.0

### 竞品对齐
- 对标 TrueNAS：合规扫描器（借鉴TrueCommand合规检查）、WAN链路规划
- 对标群晖：功率预算管理（借鉴DSM电源管理）、成本优化器
- 对标飞牛：智能带宽预测（借鉴飞牛网络优化）

## v2.547.0 (2026-06-01)

### 新功能
- AI文件恢复系统（aifilerecovery）：深度扫描、AI模式识别、多文件系统支持、恢复预览、完整性校验
- 存储ROI分析（storageroi）：ROI计算、TCO总拥有成本分析、容量规划、云端对比
- 家庭实验室应用商店（homelabstore）：精选自托管应用目录、一键安装/卸载、自动更新、社区评分
- 零信任网络访问（zerotrustaccess）：身份驱动访问控制、微分段隔离、持续认证、设备态势评估

### 改进
- 更新 VERSION 至 v2.547.0

### 竞品对齐
- 对标群晖：应用商店（借鉴套件中心）、存储ROI分析（借鉴存储分析器）
- 对标 TrueNAS：零信任安全（借鉴TrueCommand安全架构）
- 对标飞牛：AI文件恢复（借鉴飞牛数据恢复功能）
