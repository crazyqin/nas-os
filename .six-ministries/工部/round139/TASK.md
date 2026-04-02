# 工部 Round139 任务

**调度时间**: 2026-04-01 23:00
**优先级**: P0

## 任务目标
**对标**: TrueNAS Connect + LXC HA + 群晖集群管理

## 具体任务

### 1. 多节点发现与注册服务
- 完善internal/cluster/discovery.go
- mDNS服务发现机制
- 静态节点注册
- 节点心跳与状态同步

### 2. 统一仪表板数据聚合
- DashboardAggregator服务
- 多节点状态汇总
- 资源使用聚合展示

### 3. Docker容器健康检查增强
- 容器状态实时监控
- 健康检查配置
- 异常自动告警

## 交付要求
- 代码提交到: internal/cluster/ + deploy/
- 完成后汇报司礼监

## 竞品学习要点
| 竞品 | 功能 | 学习方向 |
|------|------|----------|
| TrueNAS | Connect | 多节点发现与注册 |
| TrueNAS | LXC HA | 容器高可用机制 |
| 群晖 | 集群管理 | 统一仪表板聚合 |