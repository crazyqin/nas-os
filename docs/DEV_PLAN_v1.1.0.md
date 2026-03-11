# NAS-OS v1.1.0 开发计划

> 创建时间：2026-03-11  
> 负责人：兵部  
> 状态：进行中

---

## 📋 任务清单

### 🔴 高优先级（本周完成）

#### 1. Web UI 完善 - 优化存储/共享管理页面交互

**目标：** 将演示模式的前端页面改为真实 API 驱动

**任务分解：**
- [ ] 1.1 storage.html - 连接真实 API
  - [ ] 加载卷列表 (GET /api/v1/volumes)
  - [ ] 创建卷表单 (POST /api/v1/volumes)
  - [ ] 删除卷功能 (DELETE /api/v1/volumes/:name)
  - [ ] 子卷管理 (GET/POST/DELETE /api/v1/volumes/:name/subvolumes)
  - [ ] 快照管理 (GET/POST/DELETE /api/v1/volumes/:name/snapshots)
  - [ ] 快捷操作 (平衡/校验)

- [ ] 1.2 files.html - 共享管理优化
  - [ ] 加载所有共享 (GET /api/v1/shares)
  - [ ] 创建 SMB 共享 (POST /api/v1/shares/smb)
  - [ ] 创建 NFS 共享 (POST /api/v1/shares/nfs)
  - [ ] 编辑/删除共享
  - [ ] 权限管理界面

**文件位置：** `nas-os/webui/pages/storage.html`, `nas-os/webui/pages/files.html`

---

#### 2. 文件浏览器增强 - 支持上传/下载/预览/重命名

**目标：** 实现完整的文件管理功能

**任务分解：**
- [ ] 2.1 文件浏览
  - [ ] 目录导航 (GET /api/v1/files/browse)
  - [ ] 文件列表 (支持列表/网格视图切换)
  - [ ] 面包屑导航

- [ ] 2.2 文件操作
  - [ ] 上传文件 (POST /api/v1/files/upload)
  - [ ] 下载文件 (GET /api/v1/files/download)
  - [ ] 重命名 (PUT /api/v1/files/rename)
  - [ ] 删除 (DELETE /api/v1/files/delete)
  - [ ] 移动/复制

- [ ] 2.3 文件预览
  - [ ] 图片预览 (GET /api/v1/files/preview/image)
  - [ ] 视频缩略图 (GET /api/v1/files/preview/video)
  - [ ] 文档预览 (GET /api/v1/files/preview/document)
  - [ ] 预览模态框

**文件位置：** `nas-os/webui/pages/files.html`, `nas-os/internal/files/handlers.go`

---

#### 3. 监控告警完善 - 添加邮件/微信通知

**目标：** 实现告警通知功能

**任务分解：**
- [ ] 3.1 通知模块
  - [ ] 创建 `internal/notify/` 目录
  - [ ] 邮件通知 (SMTP)
  - [ ] 微信通知 (企业微信 Webhook)
  - [ ] 通知配置管理

- [ ] 3.2 告警集成
  - [ ] 修改 `internal/monitor/handlers.go`
  - [ ] 告警触发时发送通知
  - [ ] 通知配置 API

- [ ] 3.3 前端配置界面
  - [ ] 监控页面添加通知设置
  - [ ] 告警历史查看
  - [ ] 通知渠道配置

**文件位置：** 
- 新建：`nas-os/internal/notify/`
- 修改：`nas-os/internal/monitor/handlers.go`, `nas-os/webui/pages/monitor.html`

---

### 🟡 中优先级（下周完成）

#### 4. 备份同步功能 - 远程备份/双向同步

**任务分解：**
- [ ] 4.1 远程备份
  - [ ] 备份到远程服务器 (rsync/ssh)
  - [ ] 备份到云存储 (S3 兼容)
  - [ ] 备份计划调度
  - [ ] 增量备份支持

- [ ] 4.2 双向同步
  - [ ] 文件夹同步配置
  - [ ] 冲突解决策略
  - [ ] 实时同步监控

**文件位置：** `nas-os/internal/backup/`

---

#### 5. 媒体服务器集成 - Jellyfin 深度整合

**任务分解：**
- [ ] 5.1 Jellyfin 安装
  - [ ] Docker 容器部署
  - [ ] 自动配置媒体库路径

- [ ] 5.2 媒体管理
  - [ ] 自动扫描媒体文件
  - [ ] 元数据管理
  - [ ] 转码配置

**文件位置：** `nas-os/internal/docker/`

---

## 📅 时间安排

| 日期 | 任务 | 负责人 |
|------|------|--------|
| 3/11-3/12 | Web UI 完善 | 兵部 |
| 3/13-3/14 | 文件浏览器增强 | 兵部 |
| 3/15-3/16 | 监控告警通知 | 工部 + 兵部 |
| 3/17-3/19 | 备份同步功能 | 兵部 |
| 3/20-3/22 | Jellyfin 集成 | 工部 |

---

## 🎯 验收标准

### Web UI 完善
- [ ] 存储页面显示真实卷数据
- [ ] 可以创建/删除卷和快照
- [ ] 共享管理可以创建/编辑 SMB/NFS 共享
- [ ] 所有操作有成功/失败提示

### 文件浏览器
- [ ] 可以浏览服务器文件系统
- [ ] 支持文件上传/下载
- [ ] 支持文件重命名/删除
- [ ] 图片/视频可以预览

### 监控告警
- [ ] 告警触发时发送邮件通知
- [ ] 告警触发时发送微信通知
- [ ] 可以在 Web UI 配置通知
- [ ] 可以查看告警历史

---

## 📝 技术说明

### API 规范
- 所有 API 响应格式：`{code: number, message: string, data: any}`
- 成功：`code=0`, 失败：`code!=0`
- 认证：Bearer Token (JWT)

### 前端规范
- 使用原生 JavaScript (无框架)
- 遵循 design-system.css 设计规范
- 响应式布局

### 通知接口
```go
type Notifier interface {
    Send(title string, message string, level string) error
}

// 邮件通知
type EmailNotifier struct {
    SMTPServer string
    Port       int
    Username   string
    Password   string
    From       string
}

// 微信通知
type WeChatNotifier struct {
    WebhookURL string
}
```

---

*最后更新：2026-03-11*
