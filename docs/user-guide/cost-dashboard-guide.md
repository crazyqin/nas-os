# 云存储成本分析指南

> **模块**: 云存储成本分析 | **版本**: v2.483.0 | **API**: `/api/v1/cost-dashboard/*`, `/api/v1/cost-analysis/*`, `/api/v1/cost-predict/*`

---

## 1. 简介

NAS-OS 提供全面的云存储成本分析功能，帮助用户统一管理多云存储的费用和使用情况。支持多云提供商对比、成本趋势分析、智能优化建议和预算告警。此功能为 NAS-OS 独占，群晖 / TrueNAS / 飞牛均无云存储成本管理能力。

### 适用场景
- 管理多云存储费用
- 对比不同云存储提供商性价比
- 预测存储成本趋势
- 识别存储优化机会
- 预算超支告警

---

## 2. 功能特性

### 2.1 多云提供商管理
| 提供商 | 类型标识 |
|--------|---------|
| 阿里云 OSS | aliyun |
| 腾讯云 COS | tencent |
| AWS S3 | aws |
| Google Drive | gdrive |
| OneDrive | onedrive |

### 2.2 成本分析能力
- **成本报告**: 日 / 周 / 月 多周期成本报告
- **趋势分析**: 成本上升 / 下降 / 稳定趋势识别
- **提供商对比**: 多云横向成本对比
- **优化建议**: 大文件、低频访问、重复文件检测
- **预算告警**: 自定义阈值，超预算自动告警

### 2.3 成本预测（高级功能）
- **线性回归预测**: 基于历史数据预测未来成本
- **指数平滑**: 短期成本趋势预测
- **容量增长预测**: 预测存储容量耗尽时间
- **多币种支持**: CNY / USD / EUR / JPY
- **预算告警**: 部门 / 项目级预算超支预警

### 2.4 存储成本仪表板
- **存储概览**: 总容量、已用、使用率、月增长率
- **成本概览**: 月度成本、环比变化、年度累计
- **成本趋势**: 历史成本数据可视化
- **成本预测**: 未来成本预测（含置信区间）
- **云存储对比**: 本地 vs 云端成本对比
- **成本分解**: 按介质类型分解（NVMe / SSD / HDD / Cloud）
- **优化建议**: 自动生成成本优化方案
- **预算对比**: 实际成本 vs 预算对比

---

## 3. 配置方法

### 3.1 添加云提供商

```bash
curl -X POST http://NAS_IP:8080/api/v1/cost-dashboard/providers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "阿里云 OSS",
    "type": "aliyun",
    "api_key": "your-api-key",
    "region": "cn-hangzhou"
  }'
```

### 3.2 注册存储池（本地存储）

```bash
curl -X POST http://NAS_IP:8080/api/v1/cost-analysis/pools \
  -H "Content-Type: application/json" \
  -d '{
    "name": "NVMe 存储池",
    "tier_type": "nvme",
    "total_capacity": 1099511627776,
    "used_capacity": 549755813888,
    "hardware_cost": 5000,
    "annual_power_cost": 600,
    "annual_maint_cost": 200,
    "expected_lifespan_years": 5
  }'
```

### 3.3 设置预算告警

```bash
curl -X POST http://NAS_IP:8080/api/v1/cost-dashboard/alerts \
  -H "Content-Type: application/json" \
  -d '{
    "provider_id": "{provider_id}",
    "threshold": 500.00,
    "severity": "warning"
  }'
```

### 3.4 添加历史成本数据

```bash
curl -X POST http://NAS_IP:8080/api/v1/cost-predict/records \
  -H "Content-Type: application/json" \
  -d '{
    "department": "技术部",
    "project": "NAS-OS",
    "storage_type": "nvme",
    "cost": 450.00,
    "used_capacity": 549755813888,
    "total_capacity": 1099511627776
  }'
```

---

## 4. 使用示例

### 4.1 同步云提供商指标

```bash
curl -X POST http://NAS_IP:8080/api/v1/cost-dashboard/providers/{provider_id}/sync
```

返回存储使用量、月度成本、传输成本等指标。

### 4.2 生成成本报告

```bash
curl -X POST http://NAS_IP:8080/api/v1/cost-dashboard/reports \
  -H "Content-Type: application/json" \
  -d '{"period": "monthly"}'
```

### 4.3 对比多云提供商

```bash
curl "http://NAS_IP:8080/api/v1/cost-dashboard/compare?provider_ids=id1&id2&id3"
```

### 4.4 获取每 TB 存储成本

```bash
curl http://NAS_IP:8080/api/v1/cost-analysis/pools/{pool_id}/cost-per-tb
```

**返回示例：**

```json
{
  "pool_id": "pool-ssd-01",
  "pool_name": "SATA SSD 存储池",
  "tier_type": "ssd",
  "total_capacity_tb": 4.0,
  "used_capacity_tb": 2.5,
  "hardware_cost_per_tb": 1250.0,
  "annual_power_cost_per_tb": 150.0,
  "annual_maint_cost_per_tb": 50.0,
  "total_annual_cost_per_tb": 400.0,
  "monthly_cost_per_tb": 33.33
}
```

### 4.5 层级成本对比

```bash
curl http://NAS_IP:8080/api/v1/cost-analysis/tier-comparison
```

对比 NVMe / SSD / HDD 三种存储层级的年度成本、IOPS、吞吐量和可靠性。

### 4.6 成本优化建议

```bash
curl http://NAS_IP:8080/api/v1/cost-analysis/pools/{pool_id}/optimization
```

**返回示例：**

```json
{
  "pool_id": "pool-nvme-01",
  "total_potential_saving": 1200.00,
  "suggestions": [
    {
      "id": "opt-1",
      "category": "tier_migrate",
      "title": "冷数据迁移到 SSD 层级",
      "description": "约 1.5 TB 非高频访问数据可迁移到 SATA SSD 层级，节省成本",
      "potential_saving": 800.00,
      "priority": 1,
      "action": "使用 Smart Tier 将冷数据降级到 SSD"
    },
    {
      "id": "opt-2",
      "category": "power",
      "title": "启用磁盘休眠",
      "description": "低访问频率的 HDD 启用休眠，年省约 400 元电费",
      "potential_saving": 400.00,
      "priority": 2,
      "action": "配置磁盘电源管理策略"
    }
  ]
}
```

### 4.7 容量规划

```bash
curl "http://NAS_IP:8080/api/v1/cost-analysis/pools/{pool_id}/capacity-plan?months=12"
```

基于历史增长率预测未来 12 个月的容量使用情况和预计满容量时间。

### 4.8 ROI 分析（本地 vs 云端）

```bash
curl "http://NAS_IP:8080/api/v1/cost-analysis/pools/{pool_id}/roi?years=3"
```

对比 3 年内本地存储与云存储的总成本，给出推荐方案和回本时间。

### 4.9 成本预测

```bash
curl -X POST http://NAS_IP:8080/api/v1/cost-predict/predict \
  -H "Content-Type: application/json" \
  -d '{
    "department": "技术部",
    "periods_ahead": 6
  }'
```

---

## 5. 常见问题

### Q: 如何计算本地存储的真实成本？
A: 本地存储的总成本包括：硬件购置 + 年度电力 + 年度维护 + 带宽。NAS-OS 的成本分析器会根据您注册存储池时填写的数据自动计算每 TB 每月成本。

### Q: 云存储价格参数可以自定义吗？
A: 可以。默认配置中云存储价格约 ¥200/TB/月、带宽 ¥0.8/GB、请求 ¥0.01/万次。您可以在创建分析器时传入自定义 `AnalysisConfig` 调整这些参数。

### Q: 成本预测的准确性如何？
A: 预测基于线性回归和指数平滑算法，需要至少 2 个历史数据点。数据越多、趋势越稳定，预测越准确。系统会返回置信区间，帮助您评估预测的可靠性。

### Q: 如何设置部门/项目级预算？
A: 使用成本预测模块的预算功能，为每个部门/项目设置月度预算金额。当预测成本超过预算时，系统自动生成 `warning` 或 `critical` 级别告警。

### Q: 支持哪些币种？
A: 当前支持 CNY（人民币）、USD（美元）、EUR（欧元）、JPY（日元），系统内置汇率换算。

### Q: 优化建议是如何生成的？
A: 系统根据存储池配置自动分析：NVMe 层级建议冷数据迁移、HDD 层级建议启用休眠、所有层级检查压缩/去重潜力、长期未访问数据建议归档。建议按潜在节省金额排序。
