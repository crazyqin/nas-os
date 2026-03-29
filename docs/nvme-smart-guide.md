# NVMe S.M.A.R.T. 监控指南

## 概述

nas-os 提供全面的 NVMe S.M.A.R.T.（Self-Monitoring, Analysis and Reporting Technology）健康监控功能，对标 TrueNAS 和群晖的 SSD 健康监控方案。

## 功能特性

### 核心监控指标

| 指标 | 说明 | 告警阈值（默认） |
|------|------|-----------------|
| 温度 | 设备运行温度 | 70°C 警告 / 80°C 严重 |
| 寿命百分比 | NAND写入消耗 | 90% 警告 / 95% 严重 |
| 备用空间 | 厂商预留备用块 | 10% 警告 / 5% 严重 |
| 媒体错误 | 数据完整性错误 | >0 警告 |
| 电源循环 | 开关机次数 | 信息性 |
| 开机时长 | 累计运行时间 | 信息性 |

### 告警机制

```
健康状态判定：
├── healthy   - 所有指标正常
├── warning   - 达到警告阈值
└── critical  - 达到严重阈值或存在严重错误
```

告警类型：
- `temperature` - 温度过高
- `lifespan` - 寿命告警
- `spare` - 备用空间不足
- `media_error` - 媒体错误

### Prometheus 集成

监控指标导出格式：
```
nvme_temperature{device="/dev/nvme0n1"} 45
nvme_percent_used{device="/dev/nvme0n1"} 23.5
nvme_available_spare{device="/dev/nvme0n1"} 100.0
nvme_power_cycles{device="/dev/nvme0n1"} 150
nvme_power_on_hours{device="/dev/nvme0n1"} 2847
nvme_media_errors{device="/dev/nvme0n1"} 0
```

## 使用方式

### API 接口

> 待实现：REST API 端点

### 命令行工具

> 待实现：`nas-os nvme status`

### Dashboard 看板

> 待实现：Web UI 集成

## 技术实现

### 依赖

- `nvme-cli` - NVMe 命令行工具
- `smartmontools` - SMART 监控工具（备选）

### 代码位置

- 监控服务：`internal/hardware/nvme/monitor.go`

### 数据采集

```
优先级：
1. nvme smart-log /dev/nvme*
2. smartctl -a /dev/nvme* (fallback)
```

## 竞品对比

详见：[竞品分析文档](./research/competitor-analysis-2026-03-29.md#nvme-smart-功能对比)

| 功能 | TrueNAS | 群晖 | nas-os |
|------|---------|------|--------|
| SMART监控 | ✅ | ✅ | ✅ |
| 温度告警 | ✅ | ✅ | ✅ |
| 寿命预测 | ✅ | ✅ | ✅ |
| UI界面 | ✅ | ✅ | 📋 |
| 历史趋势 | ⚠️ | ✅ | 📋 |

## 路线图

### v2.321.0 (当前)
- [x] NVMe 设备发现
- [x] SMART 数据采集
- [x] 多级告警机制
- [x] Prometheus 指标导出

### v2.322.0 (规划)
- [ ] REST API 端点
- [ ] Web UI 界面
- [ ] 历史数据存储

### v2.323.0 (规划)
- [ ] 趋势分析
- [ ] 预测性告警
- [ ] 多设备聚合视图

## 参考资料

- [NVMe Specification](https://nvmexpress.org/specifications/)
- [smartmontools Documentation](https://www.smartmontools.org/wiki)
- [TrueNAS NVMe Monitoring](https://www.truenas.com/docs/)