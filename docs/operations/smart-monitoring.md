# SMART磁盘监控灵活性改造

> **工部第162轮任务** - 对标TrueNAS 25.10 SMART监控改革
> **核心变化**: 从内置调度迁移到cron任务，支持自定义监控方案

---

## 1.概述

### 1.1TrueNAS 25.10改革

TrueNAS 25.10将SMART测试调度从内置功能迁移到cron任务：
- 用户可安装Scrutiny应用进行高级监控
- 或使用自定义监控脚本
- TrueNAS仍自动告警关键磁盘健康指标

### 1.2设计目标

- **灵活性**: 支持cron/Scrutiny/自定义脚本
- **告警保持**: 关键健康指标自动告警
- **API统一**: 统一的健康状态查询接口

---

## 2.架构设计

```
┌─────────────────────────────────────────────────────┐
│                  SMART Monitor API                   │
│  (健康查询/测试调度/告警配置)                          │
├─────────────────────────────────────────────────────┤
│               Scheduler Layer                        │
│  (Cron Tasks / Scrutiny App / Custom Script)         │
├─────────────────────────────────────────────────────┤
│              Health Check Service                    │
│  (smartctl / Scrutiny / Custom Provider)             │
├─────────────────────────────────────────────────────┤
│               Alert Engine                           │
│  (Threshold Check / Multi-channel Notify)            │
└─────────────────────────────────────────────────────┘
```

---

## 3. API设计

```yaml
# 创建SMART测试调度
POST /api/v1/smart/schedules
  Body:
    - name: 调度名称
    - testType: short/long/conveyance
    - schedule: cron表达式
    - devices: 设备列表

# 查询磁盘健康
GET /api/v1/smart/health/:device

# 查询所有磁盘健康
GET /api/v1/smart/health

# 立即运行测试
POST /api/v1/smart/test/:device
  Body:
    - testType: short/long

# 查询测试进度
GET /api/v1/smart/test/:device/progress
```

---

## 4. Scrutiny集成

可选安装Scrutiny应用：
- Web UI磁盘健康仪表板
- 历史趋势分析
- 智能告警阈值
- S.M.A.R.T.属性详细分析

---

**预计完成**: Phase 1 - M108