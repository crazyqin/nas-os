# 自动化工作流系统

## 概述

自动化工作流系统允许用户创建自定义的自动化流程，通过可视化的方式配置触发器和动作，实现文件管理、系统监控、通知等自动化任务。

## 核心组件

### 1. 可视化流程编辑器

**位置**: `/webui/pages/automation.html`

**功能**:
- 拖拽式界面设计工作流
- 实时预览流程结构
- 节点属性配置面板
- 导入/导出工作流配置

**使用方式**:
1. 访问 `/pages/automation.html`
2. 点击"+ 新建工作流"
3. 从左侧拖拽触发器和动作到画布
4. 配置每个节点的属性
5. 保存并启用工作流

### 2. 触发器系统

支持四种触发器类型：

#### 文件触发器 (File Trigger)
监控文件系统变化：
```json
{
  "type": "file",
  "path": "/home/user/downloads",
  "pattern": "*.pdf",
  "events": ["created", "modified"],
  "recursive": true
}
```

#### 时间触发器 (Time Trigger)
基于 Cron 表达式的定时任务：
```json
{
  "type": "time",
  "schedule": "0 2 * * *",
  "timezone": "Asia/Shanghai",
  "once": false
}
```

常用 Cron 示例：
- `0 * * * *` - 每小时
- `0 0 * * *` - 每天午夜
- `0 2 * * *` - 每天凌晨 2 点
- `0 0 * * 0` - 每周日
- `*/5 * * * *` - 每 5 分钟

#### 事件触发器 (Event Trigger)
系统事件触发：
```json
{
  "type": "event",
  "event_type": "user.login",
  "filter": {
    "user": "admin"
  }
}
```

支持的事件类型：
- `system.startup` - 系统启动
- `system.shutdown` - 系统关闭
- `user.login` - 用户登录
- `user.logout` - 用户登出
- `file.upload` - 文件上传
- `disk.low` - 磁盘空间不足

#### Webhook 触发器 (Webhook Trigger)
HTTP 回调触发：
```json
{
  "type": "webhook",
  "path": "/hook/my-workflow",
  "method": "POST",
  "secret": "your-secret-key"
}
```

### 3. 动作库

支持的动作类型：

#### 文件操作
- **移动** (`move`) - 移动文件/文件夹
- **复制** (`copy`) - 复制文件/文件夹
- **删除** (`delete`) - 删除文件/文件夹
- **重命名** (`rename`) - 重命名文件/文件夹

#### 媒体处理
- **转换** (`convert`) - 转换文件格式（图片、视频）

#### 通知
- **通知** (`notify`) - 发送通知（Discord、邮件等）
- **邮件** (`email`) - 发送邮件

#### 系统
- **命令** (`command`) - 执行系统命令
- **Webhook** (`webhook`) - 发送 HTTP 请求

### 4. 预置模板

系统提供以下类别的预置模板：

#### 文件管理
- 文件自动备份
- 下载文件夹整理
- 临时文件清理

#### 媒体处理
- 视频格式转换
- 自动生成缩略图

#### 系统监控
- 系统健康检查
- 磁盘空间告警

#### 通知
- 欢迎消息
- 登录通知

#### 数据同步
- 云端同步

## API 接口

### 工作流管理

```bash
# 列出所有工作流
GET /api/automation/workflows

# 创建工作流
POST /api/automation/workflows
{
  "name": "我的工作流",
  "description": "描述",
  "enabled": true,
  "trigger": {...},
  "actions": [...]
}

# 获取工作流详情
GET /api/automation/workflows/{id}

# 更新工作流
PUT /api/automation/workflows/{id}

# 删除工作流
DELETE /api/automation/workflows/{id}

# 切换工作流状态
POST /api/automation/workflows/{id}/toggle

# 手动执行工作流
POST /api/automation/workflows/{id}/execute

# 导出工作流
GET /api/automation/workflows/export/{id}

# 导入工作流
POST /api/automation/workflows/import
```

### 模板管理

```bash
# 列出所有模板
GET /api/automation/templates

# 获取模板详情
GET /api/automation/templates/{id}

# 使用模板创建工作流
POST /api/automation/templates/{id}/use
```

### 统计信息

```bash
# 获取统计
GET /api/automation/stats
```

## 使用示例

### 示例 1: 自动备份重要文件

```json
{
  "name": "每日备份",
  "trigger": {
    "type": "time",
    "schedule": "0 2 * * *"
  },
  "actions": [
    {
      "type": "copy",
      "source": "/home/user/documents",
      "destination": "/backup/documents_{{timestamp}}",
      "recursive": true
    },
    {
      "type": "notify",
      "channel": "discord",
      "title": "备份完成",
      "message": "文件备份已完成"
    }
  ]
}
```

### 示例 2: 下载文件自动分类

```json
{
  "name": "下载整理",
  "trigger": {
    "type": "file",
    "path": "/home/user/downloads",
    "events": ["created"]
  },
  "actions": [
    {
      "type": "command",
      "command": "bash",
      "args": ["-c", "organize_downloads.sh"]
    }
  ]
}
```

### 示例 3: 磁盘空间监控

```json
{
  "name": "磁盘告警",
  "trigger": {
    "type": "time",
    "schedule": "0 * * * *"
  },
  "actions": [
    {
      "type": "command",
      "command": "check_disk_space.sh"
    }
  ]
}
```

## 变量替换

在工作流配置中可以使用以下变量：

- `{{timestamp}}` - 当前时间戳
- `{{event.path}}` - 触发事件的文件路径
- `{{event.filename}}` - 触发事件的文件名
- `{{event.username}}` - 触发事件的用户名
- `{{workflow.id}}` - 工作流 ID

## 最佳实践

1. **测试工作流**: 在生产环境使用前，先手动执行测试
2. **错误处理**: 为关键工作流配置失败通知
3. **日志记录**: 定期检查工作流执行日志
4. **权限控制**: 确保工作流有适当的文件系统权限
5. **资源限制**: 避免创建过于频繁或资源密集的工作流

## 故障排除

### 工作流不执行
- 检查工作流是否已启用
- 验证触发器配置是否正确
- 查看系统日志获取错误信息

### 动作执行失败
- 检查文件路径是否正确
- 验证系统命令是否存在
- 确认有足够的权限

### 性能问题
- 减少触发频率
- 优化文件监控路径
- 避免递归监控大目录

## 开发指南

### 添加新的触发器类型

1. 在 `internal/automation/trigger/` 创建新的触发器实现
2. 实现 `Trigger` 接口
3. 在 `NewTriggerFromConfig` 中注册

### 添加新的动作类型

1. 在 `internal/automation/action/` 创建新的动作实现
2. 实现 `Action` 接口
3. 在 `NewActionFromConfig` 中注册
4. 在前端添加对应的 UI 组件

## 文件结构

```
internal/automation/
├── engine/
│   └── workflow.go      # 工作流引擎核心
├── trigger/
│   └── trigger.go       # 触发器系统
├── action/
│   └── action.go        # 动作库
├── templates/
│   └── templates.go     # 预置模板
└── api/
    └── handlers.go      # API 处理器

webui/pages/
└── automation.html      # 可视化编辑器

docs/automation/
└── README.md            # 本文档
```

## 安全考虑

1. **命令注入**: 验证所有用户输入的命令参数
2. **文件访问**: 限制工作流只能访问授权目录
3. **Webhook 认证**: 使用 secret 验证 webhook 请求
4. **权限隔离**: 工作流在受限权限下运行

## 未来计划

- [ ] 工作流执行历史记录
- [ ] 条件分支和循环支持
- [ ] 工作流调试模式
- [ ] 性能监控和告警
- [ ] 更多预置模板
- [ ] 工作流分享社区
