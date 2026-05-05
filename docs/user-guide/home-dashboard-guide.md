# 家庭仪表盘 (Home Dashboard)

> **适用版本**: v2.482.0+ | **模块**: `internal/homedashboard`

家庭仪表盘将智能家居设备状态与 NAS 管理统一到一个可自定义的面板，支持多用户个性化配置、Widget 市场和实时数据推送。

## 核心特性

| 特性 | 说明 |
|------|------|
| 🎨 可配置布局 | 网格系统，支持拖拽排列、自由调整尺寸 |
| 📦 预置 Widget | NAS 状态、Docker、天气、日历、待办、快捷操作等 |
| 🏪 Widget 市场 | 浏览社区模板，一键安装、评分 |
| 👥 多用户隔离 | 每个用户独立仪表盘配置 |
| 📡 实时刷新 | WebSocket 推送，数据变化即时更新 |
| 🔄 多布局切换 | 同一用户可创建多个布局方案 |

## 预置 Widget 类型

| Widget | 说明 | 默认尺寸 |
|--------|------|----------|
| `nas_status` | NAS 系统状态（CPU/内存/磁盘/网络/温度） | 4×2 |
| `docker_status` | Docker 容器运行状态 | 4×2 |
| `weather` | 天气预报（含 7 天预报） | 3×2 |
| `calendar` | 日历事件 | 3×3 |
| `todo_list` | 待办事项列表 | 3×2 |
| `quick_actions` | 快捷操作按钮（重启/备份/更新等） | 2×1 |
| `recent_files` | 最近访问文件 | 3×2 |
| `storage_trend` | 存储使用趋势图表 | 4×2 |
| `custom` | 自定义 Widget | 用户定义 |

## 快速开始

### 1. 创建仪表盘

```bash
curl -X POST http://localhost:8080/api/v1/homedashboard/dashboards \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "admin",
    "name": "我的家庭面板"
  }'
```

### 2. 添加 Widget

```bash
curl -X POST http://localhost:8080/api/v1/homedashboard/dashboards/{dashboard_id}/layouts/{layout_id}/widgets \
  -H "Content-Type: application/json" \
  -d '{
    "type": "nas_status",
    "title": "NAS 状态",
    "position": {"x": 0, "y": 0},
    "size": {"width": 4, "height": 2}
  }'
```

### 3. 实时订阅（WebSocket）

```
ws://localhost:8080/api/v1/homedashboard/subscribe/{dashboard_id}
```

收到的消息格式：
```json
{
  "type": "widget_update",
  "payload": { "widgetId": "hd-xxx", "data": {...} },
  "time": "2026-05-05T10:00:00Z"
}
```

## API 接口

所有接口挂载在 `/api/v1/homedashboard/` 下。

### 仪表盘管理

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/dashboards?user_id=` | 列出仪表盘 |
| `POST` | `/dashboards` | 创建仪表盘 |
| `GET` | `/dashboards/{id}` | 获取仪表盘详情 |
| `PUT` | `/dashboards/{id}` | 更新仪表盘名称 |
| `DELETE` | `/dashboards/{id}` | 删除仪表盘 |

### 布局管理

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/dashboards/{id}/layouts` | 添加布局 |
| `PUT` | `/dashboards/{id}/layouts/{lid}/active` | 设置活动布局 |
| `DELETE` | `/dashboards/{id}/layouts/{lid}` | 删除布局（默认布局不可删） |

### Widget 管理

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/dashboards/{id}/layouts/{lid}/widgets` | 添加 Widget |
| `GET` | `/dashboards/{id}/layouts/{lid}/widgets/{wid}` | 获取 Widget |
| `PUT` | `/dashboards/{id}/layouts/{lid}/widgets/{wid}` | 更新 Widget（位置/尺寸/配置） |
| `DELETE` | `/dashboards/{id}/layouts/{lid}/widgets/{wid}` | 删除 Widget |

### Widget 市场

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/templates?type=` | 浏览模板（按类型过滤） |
| `POST` | `/templates` | 注册模板 |
| `POST` | `/templates/{id}/download` | 下载模板（计数+1） |
| `POST` | `/templates/{id}/rate` | 评价模板（0-5 分） |

### 实时订阅

| 方法 | 路径 | 说明 |
|------|------|------|
| WebSocket | `/subscribe/{dashboard_id}` | 订阅仪表盘实时更新 |

## 网格布局说明

- 默认网格：**12 列 × 8 行**
- Widget 位置（`position`）和尺寸（`size`）以网格单元为单位
- 不支持重叠放置，前端应处理冲突检测

## 使用场景

- **家庭监控**：NAS 状态 + Docker 容器 + 天气 + 日历，一屏掌握
- **运维面板**：存储趋势 + 最近文件 + 快捷操作，快速运维
- **多用户**：家庭成员各有独立仪表盘，互不干扰

---

*最后更新：2026-05-06 | 礼部文档完善*
