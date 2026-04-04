# NVMe 健康监控使用指南

**版本**: v2.396.0 | **更新日期**: 2026-04-05

---

## 快速入门

### 什么是 NVMe SMART 监控？

NVMe SSD 内置 SMART（自检健康监控）系统，可实时监测硬盘健康状态、预测故障并预警。nas-os 提供完整的监控界面和告警机制。

### 如何查看 NVMe 状态

**WebUI 方式**：
1. 登录管理界面 → 「硬件管理」→ 「NVMe 设备」
2. 查看设备列表、健康状态、温度、寿命等信息

**命令行方式**：
```bash
nas-os nvme status              # 查看所有设备
nas-os nvme status /dev/nvme0n1 # 查看单设备详情
```

---

## 健康指标说明

### 核心监控项

| 指标 | 含义 | 正常范围 |
|------|------|----------|
| **温度** | 设备运行温度 | <70°C |
| **寿命百分比** | NAND写入消耗量 | <90% |
| **备用空间** | 厂商预留备用块剩余 | >10% |
| **媒体错误** | 数据完整性错误计数 | 0 |
| **临界警告** | 控制器综合警告状态 | 0 |

### 三级预警状态

| 状态 | 颜色 | 说明 | 建议 |
|------|------|------|------|
| **健康** | 🟢 绿色 | 所有指标正常 | 日常监控 |
| **警告** | 🟡 黄色 | 达到预警阈值 | 关注趋势，计划更换 |
| **严重** | 🔴 红色 | 达到严重阈值 | 立即处理 |

---

## 预警阈值配置

### 默认阈值

| 指标 | 警告阈值 | 严重阈值 |
|------|----------|----------|
| 温度 | 70°C | 80°C |
| 寿命百分比 | 90% | 95% |
| 备用空间 | 10% | 5% |
| 媒体错误 | >0 | >10 |

### 自定义阈值

**WebUI 配置**：
「硬件管理」→ 「NVMe 设备」→ 「告警设置」

**命令行配置**：
```bash
nas-os nvme set-threshold temperature warning 65
nas-os nvme set-threshold lifespan critical 90
```

---

## 最佳实践

### ⚠️ 寿命达90%时

- 购买备盘，规划迁移
- 减少写入操作
- 创建数据快照备份

### ⚠️ 温度持续70°C+

- 检查机箱散热
- 减少高负载任务
- 必要时添加散热片

### ⚠️ 媒体错误出现

- 运行 ZFS scrub 数据校验
- 立即备份重要数据
- 考虑更换设备

### ✅ 定期检查习惯

- 每周查看 NVMe Dashboard
- 关注 Prometheus 历史趋势
- 设置邮件/Webhook 告警通知

---

## 常见问题

### Q1: 寿命百分比是什么意思？

不是「剩余时间」，而是「已写入消耗量」：
- 0-50%：新盘或轻度使用
- 50-85%：正常老化
- 85-90%：开始关注
- 90%+：准备更换

### Q2: 温度80°C必须停机吗？

是的。80°C 以上可能损坏数据，建议：
1. 立即降低负载
2. 检查散热环境
3. 必要时停机冷却

### Q3: 媒体错误很严重吗？

是的。媒体错误表示数据完整性问题：
- 出现即需关注
- 运行 scrub 校验
- >10 次立即更换

---

## API 参考

### 获取设备状态

```bash
GET /api/v1/hardware/nvme/status
```

响应示例：
```json
{
  "devices": [
    {
      "device": "/dev/nvme0n1",
      "model": "Samsung 990 Pro 2TB",
      "status": "healthy",
      "temperature": 45,
      "percent_used": 23.5
    }
  ],
  "summary": {
    "healthy": 2,
    "warning": 0,
    "critical": 0
  }
}
```

### 配置阈值

```bash
POST /api/v1/hardware/nvme/thresholds
{
  "temperature": { "warning": 65, "critical": 75 }
}
```

---

## Prometheus 监控集成

nas-os 导出标准 NVMe Prometheus 指标：

```promql
nvme_temperature{device="/dev/nvme0n1"} 45
nvme_percent_used{device="/dev/nvme0n1"} 23.5
nvme_available_spare{device="/dev/nvme0n1"} 100.0
nvme_media_errors{device="/dev/nvme0n1"} 0
```

配合 Grafana 可构建可视化监控看板。

---

## 相关文档

- [NVME_SMART_GUIDE.md](../NVME_SMART_GUIDE.md) - 完整技术文档
- [API_GUIDE.md](../API_GUIDE.md) - API 详细说明

---

**文档维护**: 礼部 | **对标**: TrueNAS 25.10 Disk 界面