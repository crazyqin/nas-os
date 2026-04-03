# NVMe S.M.A.R.T. 健康监控使用指南

**版本**: v2.387.0 | **更新日期**: 2026-04-03 | **作者**: 礼部

---

## 📋 目录

1. [功能概述](#功能概述)
2. [健康监控指标详解](#健康监控指标详解)
3. [预警级别体系](#预警级别体系)
4. [最佳实践](#最佳实践)
5. [API与命令行使用](#api与命令行使用)
6. [Dashboard看板](#dashboard看板)
7. [竞品对比](#竞品对比)
8. [常见问题FAQ](#常见问题faq)

---

## 功能概述

### 什么是NVMe S.M.A.R.T.？

NVMe S.M.A.R.T.（Self-Monitoring, Analysis and Reporting Technology）是NVMe SSD的自检健康监控系统，用于实时监控固态硬盘的健康状态、预测潜在故障并发出预警。

### nas-os NVMe监控能力

nas-os对标TrueNAS 25.10和群晖DSM的NVMe健康监控方案，提供：

| 能力 | 说明 |
|------|------|
| **实时监控** | 温度、寿命、备用空间等核心指标 |
| **三级预警** | 正常/警告/严重分级告警机制 |
| **历史追踪** | Prometheus指标导出，支持趋势分析 |
| **自动发现** | NVMe设备自动识别与注册 |
| **多盘聚合** | Dashboard多设备统一视图 |

---

## 健康监控指标详解

### 核心监控指标

| 指标名称 | NVMe字段 | 说明 | 单位 |
|----------|----------|------|------|
| **温度** | temperature | 设备运行温度 | °C |
| **寿命百分比** | percent_used | NAND写入消耗量百分比 | % |
| **备用空间** | available_spare | 厂商预留备用块剩余百分比 | % |
| **备用空间阈值** | available_spare_threshold | 厂商定义的备用空间告警阈值 | % |
| **媒体错误** | media_errors | 数据完整性错误计数 | 次 |
| **临界警告** | critical_warning | NVMe控制器综合警告状态 | 位掩码 |
| **电源循环** | power_cycles | 设备开关机总次数 | 次 |
| **开机时长** | power_on_hours | 累计运行时间 | 小时 |
| **数据读写量** | data_units_read/written | 读写数据总量（512B单位） | TB |

### 临界警告位掩码详解

| 位 | 说明 | 严重程度 |
|----|------|----------|
| Bit 0 | 可用备用空间低于阈值 | 🔴 严重 |
| Bit 1 | 温度超过阈值 | 🟡 警告 |
| Bit 2 | 可靠性严重降级 | 🔴 严重 |
| Bit 3 | 媒体已进入只读模式 | 🔴 严重 |
| Bit 4 | volatile memory backup device失败 | 🔴 严重 |

---

## 预警级别体系

### 三级预警定义

nas-os采用三级预警机制，对标TrueNAS Alert系统：

| 级别 | 状态 | 颜色标识 | 触发条件 | 响应动作 |
|------|------|----------|----------|----------|
| **Healthy** | 健康 | 🟢 绿色 | 所有指标正常 | 日常监控 |
| **Warning** | 警告 | 🟡 黄色 | 达到预警阈值 | 需关注，计划更换 |
| **Critical** | 严重 | 🔴 红色 | 达到严重阈值或错误 | 立即处理 |

### 预警阈值配置（默认值）

| 指标 | Warning阈值 | Critical阈值 | 说明 |
|------|-------------|--------------|------|
| **温度** | 70°C | 80°C | 设备运行温度 |
| **寿命百分比** | 90% | 95% | NAND写入消耗 |
| **备用空间** | 10% | 5% | 厂商预留备用块 |
| **媒体错误** | >0 | >10 | 数据完整性错误 |
| **临界警告Bit0** | - | 任何值 | 备用空间不足 |

### 告警类型分类

| 告警类型 | 触发条件 | 严重级别 | 建议措施 |
|----------|----------|----------|----------|
| `temperature_warning` | 温度≥70°C | Warning | 检查散热，降低负载 |
| `temperature_critical` | 温度≥80°C | Critical | 立即停机降温 |
| `lifespan_warning` | 寿命≥90% | Warning | 规划更换，减少写入 |
| `lifespan_critical` | 寿命≥95% | Critical | 紧急备份，立即更换 |
| `spare_warning` | 备用空间≤10% | Warning | 关注寿命趋势 |
| `spare_critical` | 备用空间≤5% | Critical | 设备即将失效 |
| `media_error_warning` | 媒体错误>0 | Warning | 运行数据校验 |
| `media_error_critical` | 媒体错误>10 | Critical | 立即更换设备 |

---

## 最佳实践

### ✅ 监控配置最佳实践

**日常监控**：
```bash
# 定期检查（建议每日）
nas-os nvme status

# 或通过API
GET /api/v1/hardware/nvme/status
```

**阈值调优**：
- 高负载服务器：温度Warning阈值下调至65°C
- 数据库应用：寿命Warning阈值下调至85%
- 关键业务：所有Critical阈值触发立即告警

### ✅ 故障预防最佳实践

| 场景 | 推荐措施 | 执行时机 |
|------|----------|----------|
| **寿命达90%** | 购买备盘、规划迁移 | 提前30天 |
| **温度持续70°C+** | 加强散热、降低负载 | 立即处理 |
| **媒体错误出现** | 运行scrub、备份数据 | 24小时内 |
| **备用空间<10%** | 减少写入、准备更换 | 7天内 |

### ✅ 数据保护最佳实践

**扩容前检查清单**：
- ✅ NVMe健康状态为Healthy
- ✅ 温度在正常范围
- ✅ 寿命<85%
- ✅ 无媒体错误
- ✅ 已创建数据快照

**更换设备流程**：
```bash
# 1. 检查当前状态
nas-os nvme status

# 2. 创建数据快照
nas-os snapshot create pool/data@pre-replace

# 3. 备份数据（可选）
nas-os backup create pool/data backup-target

# 4. 执行更换
nas-os disk replace /dev/nvme0n1 /dev/nvme1n1

# 5. 验证新盘状态
nas-os nvme status
```

### ⚠️ 避免的常见错误

| 错误做法 | 正确做法 | 原因 |
|----------|----------|------|
| 温度告警后继续高负载 | 降低负载、加强散热 | 温度持续过高会加速寿命消耗 |
| 寿命95%才购买备盘 | 90%时开始规划 | 更换需时间，避免紧急故障 |
| 媒体错误忽略不处理 | 立即运行数据校验 | 媒体错误可能扩散 |
| 不同品牌混用同阵列 | 使用同品牌同型号 | 性能一致，预测寿命准确 |

---

## API与命令行使用

### REST API端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/hardware/nvme/devices | 列出所有NVMe设备 |
| GET | /api/v1/hardware/nvme/status | 获取全局健康状态 |
| GET | /api/v1/hardware/nvme/:device | 获取单设备详情 |
| GET | /api/v1/hardware/nvme/:device/smart | 获取SMART原始数据 |
| GET | /api/v1/hardware/nvme/alerts | 获取活跃告警列表 |
| POST | /api/v1/hardware/nvme/thresholds | 配置告警阈值 |

### API响应示例

```json
{
  "devices": [
    {
      "device": "/dev/nvme0n1",
      "model": "Samsung 990 Pro 2TB",
      "serial": "S6Z2NX0K123456",
      "status": "healthy",
      "temperature": 45,
      "percent_used": 23.5,
      "available_spare": 100,
      "power_on_hours": 2847,
      "power_cycles": 150,
      "media_errors": 0
    }
  ],
  "summary": {
    "total_devices": 2,
    "healthy": 2,
    "warning": 0,
    "critical": 0
  }
}
```

### 命令行工具

```bash
# 查看所有NVMe设备状态
nas-os nvme status

# 查看单设备详情
nas-os nvme status /dev/nvme0n1

# 查看SMART原始数据
nas-os nvme smart-log /dev/nvme0n1

# 配置告警阈值
nas-os nvme set-threshold temperature warning 65

# 查看历史趋势
nas-os nvme history /dev/nvme0n1 --days 30
```

---

## Dashboard看板

### WebUI界面规划

nas-os NVMe Dashboard对标TrueNAS 25.10 Disk界面：

| 区域 | 功能 | 状态 |
|------|------|------|
| **设备列表** | 所有NVMe设备卡片视图 | ✅ 已实现 |
| **健康状态** | 颜色标识状态汇总 | ✅ 已实现 |
| **温度曲线** | 实时温度历史图表 | 📋 规划中 |
| **寿命预测** | 剩余寿命趋势预测线 | 📋 规划中 |
| **告警面板** | 活跃告警列表 | ✅ 已实现 |
| **阈值配置** | 用户自定义阈值界面 | 📋 规划中 |

### Prometheus监控集成

nas-os导出标准NVMe Prometheus指标：

```promql
# 温度监控
nvme_temperature{device="/dev/nvme0n1"} 45

# 寿命消耗
nvme_percent_used{device="/dev/nvme0n1"} 23.5

# 备用空间
nvme_available_spare{device="/dev/nvme0n1"} 100.0

# 电源统计
nvme_power_cycles{device="/dev/nvme0n1"} 150
nvme_power_on_hours{device="/dev/nvme0n1"} 2847

# 错误计数
nvme_media_errors{device="/dev/nvme0n1"} 0
nvme_critical_warning{device="/dev/nvme0n1"} 0
```

### Grafana看板模板

推荐Grafana Dashboard ID：待发布

---

## 竞品对比

### 功能对比矩阵

| 功能特性 | nas-os | TrueNAS 25.10 | 群晖DSM | 飞牛fnOS |
|----------|:------:|:-------------:|:-------:|:--------:|
| **NVMe SMART监控** | ✅ 已实现 | ✅ | ✅ | ⚠️ 基础 |
| **温度监控** | ✅ | ✅ | ✅ | ✅ |
| **寿命预测** | ✅ | ✅ | ✅ | ❌ |
| **备用空间监控** | ✅ | ✅ | ✅ | ❌ |
| **媒体错误检测** | ✅ | ✅ | ✅ | ❌ |
| **三级预警** | ✅ | ✅ Alert系统 | ⚠️ 二级 | ⚠️ 基础 |
| **历史趋势** | 📋 规划 | ⚠️ Scrutiny | ✅ | ❌ |
| **WebUI界面** | 📋 完善 | ✅ 完善 | ✅ 完善 | ⚠️ 基础 |
| **API接口** | ✅ | ✅ | ✅ | ❌ |
| **Prometheus导出** | ✅ | ✅ | ❌ | ❌ |

### nas-os差异化优势

| 优势 | 说明 |
|------|------|
| **三级预警精细化** | 区分Warning/Critical，响应更精准 |
| **Prometheus原生集成** | 企业级监控体系无缝对接 |
| **AI寿命预测** | 基于历史数据的智能预测（规划） |
| **多云数据同步** | NVMe作为缓存层的健康联动监控 |

---

## 常见问题FAQ

### Q1: NVMe寿命百分比如何理解？

**答**：寿命百分比反映NAND闪存写入消耗量，非剩余可用时间。

| 寿命值 | 状态 | 说明 |
|--------|------|------|
| 0-50% | 健康 | 新盘或轻度使用 |
| 50-85% | 正常 | 中度使用，正常老化 |
| 85-90% | 关注 | 重度使用，开始规划更换 |
| 90-95% | 警告 | 需尽快更换备盘 |
| 95-100% | 严重 | 立即更换 |

### Q2: 温度告警后需要立即停机吗？

**答**：视告警级别而定：

| 级别 | 温度范围 | 响应建议 |
|------|----------|----------|
| Warning (70°C) | 70-79°C | 检查散热、降低负载 |
| Critical (80°C) | ≥80°C | 立即停机检查，避免数据损坏 |

### Q3: 媒体错误是什么意思？

**答**：媒体错误表示数据完整性问题，可能原因：

| 原因 | 说明 | 处理 |
|------|------|------|
| NAND老化 | 寿命接近耗尽 | 更换设备 |
| 写入中断 | 异常断电导致 | 运行scrub校验 |
| 制造缺陷 | 固件或硬件问题 | 联系厂商 |
| 环境因素 | 温度、电压异常 | 检查环境 |

### Q4: 多NVMe设备如何统一监控？

**答**：nas-os Dashboard提供多设备聚合视图：

```bash
# API获取汇总状态
GET /api/v1/hardware/nvme/status

# 响应包含汇总信息
{
  "summary": {
    "total_devices": 4,
    "healthy": 3,
    "warning": 1,
    "critical": 0
  }
}
```

### Q5: 如何自定义告警阈值？

**答**：通过API或命令行配置：

```bash
# 命令行设置
nas-os nvme set-threshold temperature warning 65
nas-os nvme set-threshold lifespan critical 90

# API配置
POST /api/v1/hardware/nvme/thresholds
{
  "temperature": { "warning": 65, "critical": 75 },
  "lifespan": { "warning": 85, "critical": 90 }
}
```

### Q6: nas-os NVMe监控准确吗？

**答**：nas-os直接读取NVMe控制器SMART日志，与厂商工具数据一致：

- 数据来源：NVMe SMART Log Page（NVMe Spec标准）
- 采集工具：nvme-cli（Linux标准工具）
- 校验方式：与smartctl交叉验证

---

## 相关文档

| 文档 | 说明 |
|------|------|
| [nvme-smart-guide.md](./nvme-smart-guide.md) | 技术实现文档 |
| [COMPETITIVE_ANALYSIS_2026Q2.md](./COMPETITIVE_ANALYSIS_2026Q2.md) | 竞品分析报告 |
| [API_GUIDE.md](./API_GUIDE.md) | API完整文档 |

---

## 更新记录

| 版本 | 日期 | 更新内容 |
|------|------|----------|
| v1.0 | 2026-04-03 | 礼部创建完整用户指南 |

---

**文档维护**: 礼部（品牌营销） | **功能状态**: 已实现，持续完善