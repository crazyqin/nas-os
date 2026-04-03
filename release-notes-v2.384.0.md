# NAS-OS v2.384.0 - UI Search + 告警增强

## 🎯 第152轮六部协同开发

本轮对标 **TrueNAS 25.10 UI Search** 和 **群晖 Active Insight 告警分组**。

## ✨ 新功能

### 🔍 UI Search（对标TrueNAS）
- API端点: `/api/v1/search/ui`
- 搜索范围: 用户、共享、应用、设置
- 结果分组显示，支持模糊匹配

### 📊 告警分组增强（对标群晖Active Insight）
- 四大分组: 存储/网络/系统/安全
- 告警级别: Critical/Warning/Info
- 静默时段配置
- 告警聚合防风暴

### 🔒 安全评估文档
- 文件锁定安全设计（防死锁）
- LXC容器安全评估

## 📋 API新增

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/search/ui | UI搜索 |
| GET | /api/v1/alerting/groups | 告警分组列表 |
| POST | /api/v1/alerting/groups | 创建告警分组 |
| PUT | /api/v1/alerting/groups/:id | 更新告警分组 |
| DELETE | /api/v1/alerting/groups/:id | 删除告警分组 |
| POST | /api/v1/alerting/silence | 配置静默时段 |

## 🏆 竞品对标矩阵

| 功能 | nas-os | TrueNAS | 群晖 |
|------|:------:|:-------:|:----:|
| UI Search | ✅ | ✅ | ❌ |
| 告警分组 | ✅ | ❌ | ✅ |
| 多系统管理 | ✅ CMS | ✅ Connect | ✅ |
| WriteOnce不可变 | ✅ **独家** | ❌ | ❌ |
| 本地LLM服务 | ✅ **独家** | ❌ | ✅ |

## 🔧 六部协同

| 部门 | 任务 | 状态 |
|------|------|:----:|
| 兵部 | UI Search API | ✅ |
| 工部 | 告警分组管理 | ✅ |
| 刑部 | 安全评估文档 | ✅ |
| 户部 | 成本报表增强 | ✅ |
| 礼部 | 文档更新 | ✅ |
| 吏部 | 版本管理 | ✅ |

---

**司礼监调度** | 2026-04-03