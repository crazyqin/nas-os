## v2.550.0 (2026-06-02)

### 新功能
- 智能容器资源限制器（containerreslimiter）：基于历史使用模式自动调整CPU/内存限制，支持保守/平衡/激进策略，百分位数分析，自动调优
- 智能数据放置引擎（smartdataplacement）：基于数据温度（热/温/冷/冻结）自动在NVMe/SSD/HDD/Cloud间迁移数据，放置策略可配置
- NAS功率预算管理（naspowerbudget）：设备能耗监控、功率预算控制、能耗报告、碳排放计算、节能建议、定时调度

### 改进
- 存储成本分析器增强：新增TCO分析、ROI计算、多方案对比、数据优化收益估算、增强成本预测
- Zero Trust访问控制增强：新增ABAC属性访问控制、设备信任评估、风险引擎、审计日志
- 修复Compatibility Check workflow并发问题：改为cancel-in-progress模式避免旧任务堆积

### 竞品对齐
- 对标群晖Container Manager：智能容器资源限制器（借鉴容器资源管理功能）
- 对标TrueNAS分层存储：智能数据放置引擎（借鉴ZFS分层存储策略）
- 对标飞牛节能管理：功率预算管理（借鉴飞牛功耗优化功能）

## v2.549.0 (2026-06-01)

### 新功能
- 智能存储健康预测（smarthealthpredict）：S.M.A.R.T数据分析、AI故障预测、健康评分、趋势分析、告警建议

### 改进
- 新增系统诊断中心（diagnostics）：一键诊断、健康评分、问题检测、诊断报告、历史记录
- 新增智能温控管理（thermalmanager）：温度监控、风扇控制、散热策略、过热保护
- 新增文件版本控制（fileversion）：版本历史、差异对比、回滚恢复、自动清理
- 新增功率管理器（powermanager）：功耗监控、节能策略、定时开关机、负载均衡
- 新增数据完整性校验（dataintegrity）：校验和计算、损坏检测、自动修复、完整性报告

### 竞品对齐
- 对标群晖：存储健康预测（借鉴存储管理器S.M.A.R.T监控）、系统诊断（借鉴存储分析器）
- 对标 TrueNAS：数据完整性校验（借鉴Self-Healing Checksums）
- 对标飞牛：智能温控管理（借鉴飞牛散热管理）

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
