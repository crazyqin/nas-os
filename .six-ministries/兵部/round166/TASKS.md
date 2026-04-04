# 兵部第166轮任务 - 磁盘电源管理 + NVMe健康 + 勒索防护

## 优先级: P0

## 任务清单

### 1. 磁盘智能电源管理（对标飞牛fnOS）
**文件**: `internal/disk/power_mgmt.go`

功能实现:
- [ ] standby/spindown策略实现
- [ ] 按需唤醒逻辑（检测IO活动自动唤醒）
- [ ] 电源状态监控API
- [ ] 定时休眠策略配置
- [ ] 节能报告生成

参考:
- 飞牛fnOS按需唤醒硬盘
- TrueNAS磁盘电源管理

### 2. NVMe健康预测完善
**文件**: `internal/disk/nvme_health.go`

功能实现:
- [ ] 三级预警机制（正常/警告/危险/紧急）
- [ ] 寿命预测算法（基于写入量、温度、磨损）
- [ ] 温度历史记录
- [ ] 健康评分API
- [ ] 前端展示数据格式

### 3. 勒索防护增强（对标TrueNAS Ransomware Defense）
**文件**: `internal/security/ransomware/`

功能实现:
- [ ] honeypot文件检测模块
- [ ] 行为分析模块（检测批量加密、删除模式）
- [ ] 自动响应触发（隔离、快照、告警）
- [ ] 实时保护状态监控
- [ ] 保护策略配置API

## 竞品参考

### TrueNAS Ransomware Defense
- honeypot蜜罐文件
- 文件行为监控
- 实时保护开关
- 自动快照保护
- 隔离机制

### 飞牛按需唤醒
- 智能休眠策略
- IO活动检测
- 按需自动唤醒
- 节能统计

## 交付要求
- 代码实现 + 单元测试
- API文档
- 与前端联调准备

## 参考文件
- `internal/disk/disk_power_manager.go` (现有)
- `internal/security/ransomware/` (现有)

---
**创建时间**: 2026-04-05
**兵部**