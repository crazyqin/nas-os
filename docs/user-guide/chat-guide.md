# Chat 即时通讯使用指南

> **模块**: Chat 即时通讯 | **版本**: v2.483.0 | **API**: `/api/v1/chat/*`

---

## 1. 简介

NAS-OS Chat 是内建的即时通讯模块，对标群晖 Synology Chat。支持私聊、群组和频道三种通讯模式，提供消息管理、表情反应、成员权限等完整功能。所有数据存储在本地 NAS，保障隐私安全。

### 适用场景
- 家庭成员之间沟通
- 团队内部工作交流
- 项目频道公告发布
- 文件分享和讨论

---

## 2. 功能特性

### 频道类型
| 类型 | 说明 | 典型场景 |
|------|------|---------|
| **direct** | 私聊 | 两人一对一沟通 |
| **group** | 群组 | 小团队讨论 |
| **channel** | 频道 | 公告/通知发布 |

### 成员角色
| 角色 | 权限 |
|------|------|
| **owner** | 完全控制（创建者自动获得） |
| **admin** | 管理成员、编辑频道设置 |
| **member** | 发送消息、查看历史 |

### 消息功能
- ✅ 文本消息（text）
- ✅ 文件消息（file）
- ✅ 图片消息（image）
- ✅ 系统消息（system）
- ✅ 消息回复（Reply To）
- ✅ 表情反应（Reactions）
- ✅ 消息编辑
- ✅ 消息软删除
- ✅ 全文搜索
- ✅ 未读计数

---

## 3. 配置方法

Chat 模块开箱即用，无需额外配置。服务启动后自动初始化。

### 前置条件
- NAS-OS v2.483.0 或更高版本
- 已创建至少一个用户账户

---

## 4. 使用示例

### 4.1 创建频道

**创建群组频道：**

```bash
curl -X POST http://NAS_IP:8080/api/v1/chat/channels \
  -H "Content-Type: application/json" \
  -d '{
    "name": "家庭群",
    "description": "家庭成员沟通",
    "type": "group",
    "creator_id": "user-001"
  }'
```

**创建公告频道：**

```bash
curl -X POST http://NAS_IP:8080/api/v1/chat/channels \
  -H "Content-Type: application/json" \
  -d '{
    "name": "系统公告",
    "description": "NAS 系统公告通知",
    "type": "channel",
    "creator_id": "admin"
  }'
```

### 4.2 发送消息

```bash
curl -X POST http://NAS_IP:8080/api/v1/chat/channels/{channel_id}/messages \
  -H "Content-Type: application/json" \
  -d '{
    "sender_id": "user-001",
    "content": "大家好，NAS 系统已升级到 v2.483.0",
    "type": "text"
  }'
```

### 4.3 回复消息

```bash
curl -X POST http://NAS_IP:8080/api/v1/chat/channels/{channel_id}/messages \
  -H "Content-Type: application/json" \
  -d '{
    "sender_id": "user-002",
    "content": "收到，新功能看起来不错！",
    "type": "text",
    "reply_to": "{message_id}"
  }'
```

### 4.4 添加表情反应

```bash
curl -X POST http://NAS_IP:8080/api/v1/chat/messages/{message_id}/reactions \
  -H "Content-Type: application/json" \
  -d '{
    "emoji": "👍",
    "user_id": "user-002"
  }'
```

### 4.5 获取未读消息

```bash
curl http://NAS_IP:8080/api/v1/chat/users/{user_id}/unread
```

### 4.6 搜索消息

```bash
curl "http://NAS_IP:8080/api/v1/chat/search?q=升级&channel={channel_id}"
```

### 4.7 添加成员

```bash
curl -X POST http://NAS_IP:8080/api/v1/chat/channels/{channel_id}/members \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-003",
    "role": "member"
  }'
```

---

## 5. 常见问题

### Q: Chat 数据存储在哪里？
A: 所有频道、消息和成员数据存储在 NAS-OS 内部数据库中，完全本地化，不经过任何外部服务器。

### Q: 支持文件发送吗？
A: 支持。消息类型为 `file` 时，消息内容为文件路径或附件标识。建议配合 NAS 文件系统使用。

### Q: 消息删除后能恢复吗？
A: Chat 采用软删除机制。删除的消息会被标记 `deleted_at`，不再出现在消息列表中，但数据仍保留在数据库中。管理员可通过 API 查询已删除消息。

### Q: 频道最多支持多少成员？
A: 当前版本无硬性限制，性能取决于 NAS 硬件配置。建议单频道不超过 500 成员以保证查询性能。

### Q: 如何在群组中设置管理员？
A: 频道 Owner 可通过 `UpdateMemberRole` API 将成员角色从 `member` 提升为 `admin`：

```bash
curl -X PUT http://NAS_IP:8080/api/v1/chat/channels/{channel_id}/members/{user_id}/role \
  -H "Content-Type: application/json" \
  -d '{"role": "admin"}'
```

### Q: 支持消息推送通知吗？
A: 当前版本支持通过 NAS-OS 通知系统获取未读计数。移动端推送通知计划在未来版本中实现。
