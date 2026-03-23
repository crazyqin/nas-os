# Drive 模块架构设计

## 概述

Drive 模块提供类似 Synology Drive 的文件同步服务，支持多端同步、版本控制、文件锁定和审计日志。

## 目录结构

```
internal/drive/
├── drive.go        # 服务核心
├── sync.go         # 同步引擎
├── version.go      # 版本管理 (Intelliversioning)
├── lock.go         # 文件锁定
├── audit.go        # 审计日志
├── handler.go      # HTTP 处理器 (待实现)
├── watcher.go      # 文件监控 (待实现)
├── client.go       # 客户端协议 (待实现)
└── drive_test.go   # 单元测试
```

## 核心组件

### 1. Service (drive.go)

服务入口，协调各组件工作：

```go
type Service struct {
    config   *Config
    syncer   *SyncEngine
    locker   *FileLocker
    auditor  *AuditLogger
    version  *VersionManager
}
```

主要方法：
- `Start(ctx)` - 启动服务
- `Stop()` - 停止服务
- `SyncFile(ctx, path)` - 同步文件
- `LockFile(ctx, path, userID)` - 锁定文件
- `GetVersion(ctx, path, version)` - 获取历史版本

### 2. SyncEngine (sync.go)

同步引擎，处理文件同步：

**核心特性：**
- 块级增量同步
- 按需同步 (占位符模式)
- 变更检测
- 哈希校验

**同步流程：**
```
1. 文件变更 → 加入同步队列
2. 计算文件哈希 → 检测实际变更
3. 块级差异检测 → 仅传输变化部分
4. 更新远端 → 记录同步状态
```

### 3. VersionManager (version.go)

版本管理器，实现 **Intelliversioning** 算法：

**算法原理：**
1. 根据变更类型评分 (major=3.0, minor=2.0, patch=1.0)
2. 时间衰减因子 (越近越重要)
3. 版本间隔因子 (间隔越长越重要)
4. 保留高分版本，删除低分版本

**示例：**
```
版本 v1.0 → v1.1 (minor, 1天内) → 重要性 4.0
版本 v1.1 → v1.2 (patch, 1小时内) → 重要性 2.0
版本 v1.2 → v2.0 (major, 1周后) → 重要性 4.5

保留: v2.0 (4.5), v1.0 (4.0)
删除: v1.1, v1.2
```

### 4. FileLocker (lock.go)

文件锁定器，防止协作冲突：

**锁定规则：**
- 用户锁定文件后，其他用户无法编辑
- 锁定有超时时间，防止死锁
- 锁定者可延长锁定期
- 管理员可强制解锁

**使用场景：**
```
用户A 打开文档 → 锁定文档
用户B 尝试编辑 → 提示"文档被 A 锁定"
用户A 关闭文档 → 解锁文档
用户B 可编辑
```

### 5. AuditLogger (audit.go)

审计日志记录器：

**记录的操作类型：**
- sync, lock, unlock
- read, write, delete, rename
- share, upload, download, version

**日志内容：**
- 时间戳、用户、操作、路径
- 客户端 IP、User-Agent
- 操作结果、错误信息

## API 设计

### REST API

```
GET    /api/drive/files          # 列出文件
POST   /api/drive/files          # 上传文件
GET    /api/drive/files/{path}   # 下载文件
PUT    /api/drive/files/{path}   # 更新文件
DELETE /api/drive/files/{path}   # 删除文件

POST   /api/drive/lock/{path}    # 锁定文件
DELETE /api/drive/lock/{path}    # 解锁文件

GET    /api/drive/versions/{path} # 列出版本
GET    /api/drive/versions/{path}/{version} # 获取版本

GET    /api/drive/audit          # 查询审计日志
```

### WebSocket API (实时同步)

```json
{
  "type": "file_change",
  "path": "/documents/report.docx",
  "action": "update",
  "timestamp": "2026-03-23T10:00:00Z"
}
```

## 与现有模块集成

### 1. files 模块
- 复用文件操作逻辑
- 继承权限控制

### 2. sync 模块
- 扩展现有同步能力
- 复用传输协议

### 3. webdav 模块
- 提供标准协议支持
- 兼容现有客户端

### 4. auth 模块
- 复用用户认证
- 继承会话管理

### 5. rbac 模块
- 继承权限模型
- 扩展 Drive 专属权限

## 性能优化

### 1. 增量同步
- 使用 rolling hash 检测块变化
- 仅传输变化块

### 2. 压缩传输
- 大文件自动压缩
- 支持分块压缩

### 3. 并发控制
- 多文件并行同步
- 带宽限制

### 4. 缓存策略
- 文件元数据缓存
- 版本信息缓存

## 安全考虑

### 1. 传输加密
- TLS 加密传输
- 可选端到端加密

### 2. 访问控制
- 基于 RBAC 的权限控制
- 文件级别的 ACL

### 3. 审计追踪
- 所有操作可追溯
- 日志防篡改 (HMAC)

## 下一步

1. **handler.go** - 实现 HTTP API
2. **watcher.go** - 实现文件监控 (fsnotify)
3. **client.go** - 定义客户端协议
4. **与 webdav 集成** - 提供标准协议支持
5. **桌面客户端协议** - 实现原生同步客户端

## 测试覆盖

- 单元测试: `drive_test.go`
- 集成测试: 待添加
- 性能测试: 待添加