# 兵部第165轮任务 - NVMe健康预测 + 磁盘电源管理

## 任务清单

### 1. NVMe健康预测完善
- 完善NVMe寿命预测算法
- 实现三级预警机制（正常/警告/危险）
- 前端展示NVMe健康状态

**交付文件**: `internal/disk/nvme_health.go`

### 2. 磁盘智能电源管理
- standby/spindown策略实现
- 按需唤醒逻辑（对标飞牛fnOS）
- 电源状态监控API

**交付文件**: `internal/disk/power_mgmt.go`

### 3. 勒索防护原型
- honeypot文件检测
- 行为分析模块
- 自动响应触发

**交付文件**: `internal/security/ransomware.go`

## 参考竞品
- TrueNAS 26: Ransomware Defense
- 飞牛fnOS: 按需唤醒硬盘