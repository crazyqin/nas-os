# 六部任务分配 - v2.21.0

## 项目状态
- **当前版本**: v2.20.2
- **待处理 TODO**: 54 项
- **Actions 状态**: CI/CD 运行中

---

## 兵部 (软件工程) - 核心功能 TODO

### 1. photos 模块 (优先级: 高)
文件: `internal/photos/`
- [ ] 实现排序逻辑 (manager.go)
- [ ] 实现不同尺寸缩略图支持 (handlers.go)
- [ ] 从认证信息获取 userID (handlers.go)
- [ ] 计算实际使用空间 (handlers.go)
- [ ] 清除 AI 数据并重新分析 (handlers.go)
- [ ] ffmpeg 转换 HEIC 为 JPEG (ai.go)
- [ ] 实现日期范围匹配 (ai.go)
- [ ] 保存 AI 数据到存储 (ai.go)
- [ ] 实现回忆查询 (ai.go)
- [ ] 云端 API 集成 (Azure Face API, AWS Rekognition)

### 2. tiering 模块 (优先级: 中)
文件: `internal/tiering/manager.go`
- [ ] 计算下次执行时间

### 3. replication 模块 (优先级: 中)
文件: `internal/replication/manager.go`
- [ ] 解析 rsync 输出获取详细统计

### 4. backup 模块 (优先级: 中)
文件: `internal/backup/`
- [ ] cloud.go TODO 项
- [ ] sync.go TODO 项

---

## 工部 (DevOps) - 运维与自动化

### 1. network 模块 (优先级: 高)
文件: `internal/network/manager.go`
- [ ] 从文件加载配置
- [ ] 保存配置到文件

### 2. perf 模块 (优先级: 中)
文件: `internal/perf/manager.go`
- [ ] 发送通知 (email/webhook)

### 3. cluster 模块 (优先级: 中)
文件: `internal/cluster/`
- [ ] ha.go: 发送心跳到 peer
- [ ] manager.go TODO 项

### 4. automation 模块 (优先级: 低)
文件: `internal/automation/`
- [ ] trigger.go TODO 项
- [ ] action.go TODO 项

---

## 刑部 (安全合规) - 认证与权限

### 1. security 模块 (优先级: 高)
文件: `internal/security/v2/mfa.go`
- [ ] 移除已使用的恢复码
- [ ] 临时存储验证码（带过期时间）

### 2. auth 模块 (优先级: 高)
文件: `internal/auth/`
- [ ] sms.go: 实现阿里云短信 API 调用
- [ ] sms.go: 实现腾讯云短信 API 调用
- [ ] rbac_middleware.go: 从用户对象获取组信息
- [ ] backup.go TODO 项
- [ ] backup_test.go TODO 项

---

## 礼部 (UI/文档) - 界面与文档

### 1. shares 模块 (优先级: 中)
文件: `internal/shares/handlers.go`
- [ ] 更新全局 SMB 配置
- [ ] 更新全局 NFS 配置

### 2. web 中间件 (优先级: 高)
文件: `internal/web/middleware.go`
- [ ] CSRFKey 从环境变量读取
- [ ] 验证 token (需要从 session 或 cookie 中获取)

---

## 吏部 (项目管理) - 跟踪与协调

- [ ] 跟踪各部门进度
- [ ] 更新 MILESTONES.md
- [ ] 代码审查
- [ ] 版本发布

---

## 户部 (财务预算)

- 无待办任务

---

## 完成标准

1. 所有 TODO 项需实现完整功能
2. 添加单元测试覆盖
3. 更新相关文档
4. 代码通过 gofmt 和 golangci-lint 检查

---

## 提交方式

完成后提交 PR 到 `develop` 分支，由吏部审核后合并。

---

*创建时间: 2026-03-15*