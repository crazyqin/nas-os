# 兵部任务 - 第204轮六部协同开发

**负责人**: 兵部（软件工程）
**时间**: 2026-04-09 08:56
**版本**: v2.432.0

---

## 任务：NVMe-oF Phase2 + 磁盘智能电源管理

### 1. NVMe-oF Phase2 实现

**对标**: TrueNAS NVMe over Fabric

**实现内容**:
- NVMe/TCP服务端实现增强
- ANA（Asymmetric Namespace Access）多路径支持
- 连接管理和错误恢复

**文件**:
- `internal/nvme/server.go` - 服务端实现
- `internal/nvme/ana.go` - ANA多路径
- `internal/nvme/handlers.go` - API处理器

### 2. NVMe健康预测增强

**对标**: TrueNAS灵活磁盘健康监控

**实现内容**:
- NVMe SMART数据收集（温度、寿命、写入量）
- 三级预警机制（健康/警告/危险）
- 健康评分算法

**文件**:
- `internal/disk/nvme_health.go` - 健康监控
- `internal/disk/smart_parser.go` - SMART解析

### 3. 磁盘智能电源管理

**对标**: 飞牛按需唤醒硬盘

**实现内容**:
- 磁盘休眠策略API（standby/spindown）
- IO唤醒检测机制
- 节能报告生成
- 定时休眠调度

**文件**:
- `internal/disk/power_mgmt.go` - 电源管理
- `internal/disk/power_api.go` - API端点

---

## API端点设计

```
GET  /api/v1/disk/power          # 获取所有磁盘电源状态
POST /api/v1/disk/power/standby  # 设置磁盘休眠
POST /api/v1/disk/power/wake     # 唤醒磁盘
GET  /api/v1/disk/power/policy   # 获取电源策略
POST /api/v1/disk/power/policy   # 设置电源策略
GET  /api/v1/disk/nvme/health    # 获取NVMe健康状态
GET  /api/v1/disk/nvme/smart     # 获取NVMe SMART数据
```

---

## 验收标准

- [ ] NVMe/TCP服务端可正常启动
- [ ] ANA多路径切换正常
- [ ] NVMe健康评分算法实现
- [ ] 磁盘休眠/唤醒API正常
- [ ] 单元测试覆盖

---

**兵部交付**
**截止时间**: 2026-04-09 12:00