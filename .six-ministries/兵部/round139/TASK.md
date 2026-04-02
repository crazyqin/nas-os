# 兵部 Round139 任务

**调度时间**: 2026-04-01 23:00
**优先级**: P0

## 任务目标
**对标**: TrueNAS WebShare + TrueSearch + 群晖CMS

## 具体任务

### 1. WebShare内容搜索API
- 实现全文检索搜索API
- 支持文件名、文件内容搜索
- 搜索结果排序与权重算法
- 与internal/search模块集成

### 2. 多系统管理核心接口
- NodeManagementService实现
- FleetManager节点注册机制
- 跨节点任务调度基础框架

## 交付要求
- 代码提交到: internal/webshare/ + internal/cms/
- 完成后汇报司礼监

## 竞品学习要点
| 竞品 | 功能 | 学习方向 |
|------|------|----------|
| TrueNAS | WebShare | 浏览器文件访问无需客户端 |
| TrueNAS | TrueSearch | 内容搜索，全文检索 |
| 群晖 | CMS | 多节点集中管理接口 |