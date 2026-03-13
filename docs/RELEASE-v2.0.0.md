# NAS-OS v2.0.0 发布说明

**发布日期**: 2026-04-01  
**版本类型**: 重大版本更新  
**代号**: 企业级存储平台

---

## 🎯 版本概述

NAS-OS v2.0.0 是一个重大版本更新，标志着项目从功能完善阶段迈入企业级存储平台阶段。本版本重点完善了存储复制和回收站两大核心模块，并显著提升了测试覆盖率。

---

## ✨ 新增功能

### 存储复制模块 (Replication)

完整的企业级存储复制解决方案：

#### 复制任务管理
- 创建、更新、删除、列出复制任务
- 支持三种复制模式：
  - **实时同步 (realtime)**: 文件变更立即同步
  - **定时复制 (scheduled)**: 按 cron 表达式定时执行
  - **双向复制 (bidirectional)**: 双向实时同步

#### 冲突检测与解决
- 自动检测文件冲突（修改时间、大小、内容哈希）
- 多种冲突解决策略：
  - 源端优先 (source_wins)
  - 目标端优先 (target_wins)
  - 较新优先 (newer_wins)
  - 较大优先 (larger_wins)
  - 重命名保留 (rename)
  - 跳过冲突 (skip)
  - 手动解决 (manual)

#### 任务调度
- 内置调度器自动触发定时任务
- 手动触发同步
- 任务暂停/恢复
- 同步状态监控

#### 技术实现
- rsync 集成实现高效文件同步
- 支持本地和远程目标
- 带宽限制配置
- 压缩传输支持
- 删除目标端多余文件选项

#### API 端点
```
GET    /api/v1/replications              # 列出所有任务
POST   /api/v1/replications              # 创建任务
GET    /api/v1/replications/:id          # 获取任务详情
PUT    /api/v1/replications/:id          # 更新任务
DELETE /api/v1/replications/:id          # 删除任务
POST   /api/v1/replications/:id/sync     # 手动触发同步
POST   /api/v1/replications/:id/pause    # 暂停任务
POST   /api/v1/replications/:id/resume   # 恢复任务
GET    /api/v1/replications/stats        # 获取统计信息
GET    /api/v1/replications/conflicts    # 列出所有冲突
GET    /api/v1/replications/:id/conflicts # 列出任务冲突
POST   /api/v1/replications/conflicts/:conflictId/resolve # 解决冲突
```

---

### 回收站模块 (Trash)

完善的企业级回收站功能：

#### 核心功能
- 安全删除文件（移动到回收站而非永久删除）
- 文件恢复到原始位置或指定位置
- 永久删除
- 清空回收站
- 回收站项目列表和统计

#### 自动管理
- 自动过期清理（按保留天数）
- 空间限制自动清理
- 可配置清理策略

#### 配置选项
- 启用/禁用回收站
- 保留天数配置
- 最大占用空间限制
- 自动清空开关

#### API 端点
```
GET    /api/v1/trash              # 列出回收站项目
GET    /api/v1/trash/stats        # 获取统计信息
GET    /api/v1/trash/config       # 获取配置
PUT    /api/v1/trash/config       # 更新配置
POST   /api/v1/trash/:id/restore  # 恢复文件
DELETE /api/v1/trash/:id          # 永久删除
DELETE /api/v1/trash              # 清空回收站
POST   /api/v1/trash/move         # 移动到回收站
```

---

## 🔧 改进优化

### 测试覆盖率提升

| 模块 | 覆盖率 | 测试文件 |
|------|--------|----------|
| replication | 61.9% | handlers_test.go, manager_test.go, conflict_test.go |
| trash | 77.1% | handlers_test.go, manager_test.go |

### 测试类型
- **API 处理器测试**: 测试所有 HTTP 端点
- **业务逻辑测试**: 测试核心功能逻辑
- **边界条件测试**: 测试空值、不存在、错误输入
- **并发安全测试**: 测试多线程访问安全性
- **错误处理测试**: 测试各种错误场景

---

## 📦 新增文件

### replication 模块
```
internal/replication/
├── handlers.go          # HTTP 处理器
├── handlers_test.go     # 处理器测试 (新增)
├── manager.go           # 业务管理器
├── manager_test.go      # 管理器测试 (新增)
├── conflict.go          # 冲突检测
├── conflict_test.go     # 冲突测试 (已存在)
└── watcher.go           # 文件监控
```

### trash 模块
```
internal/trash/
├── handlers.go          # HTTP 处理器
├── handlers_test.go     # 处理器测试 (新增)
├── manager.go           # 业务管理器
└── manager_test.go      # 管理器测试 (已存在)
```

---

## 📚 文档更新

- `docs/CHANGELOG.md` - 添加 v2.0.0 变更记录
- `docs/RELEASE-v2.0.0.md` - 本发布文档

---

## 🐛 Bug 修复

- 修复回收站恢复时目标文件存在检查
- 修复复制任务状态更新并发问题
- 改进错误消息的可读性

---

## ⚠️ 破坏性变更

无破坏性变更。v1.x 用户可平滑升级。

---

## 🔄 升级指南

### 从 v1.9.0 升级

1. 停止服务: `systemctl stop nasd`
2. 备份配置: `cp /etc/nas-os/config.yaml /etc/nas-os/config.yaml.bak`
3. 更新二进制文件
4. 启动服务: `systemctl start nasd`

配置格式保持兼容，无需手动迁移。

---

## 📋 已知问题

1. 实时同步模式依赖 inotify，在大目录下可能有性能影响
2. 远程同步依赖 SSH 密钥认证，需提前配置

---

## 🗓️ 下一步计划 (v2.1.0)

- 插件系统架构
- 第三方应用完整支持
- 多节点集群增强
- 更多云存储提供商支持

---

## 👥 贡献者

感谢所有为本版本做出贡献的开发者：

- 兵部 - 核心功能开发
- 吏部 - 测试与文档

---

**NAS-OS 团队**  
*让家庭存储更简单*