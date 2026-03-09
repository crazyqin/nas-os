# NAS-OS 六部任务分配

> 创建时间：2026-03-10  
> 项目负责人：吏部  
> 项目位置：~/clawd/nas-os

---

## 兵部 - 软件工程

### 核心职责
- 存储管理模块开发
- 文件共享协议实现
- 用户权限系统
- API 接口开发

### 具体任务

#### M1 存储管理 (优先级：P0)
```go
// 文件：internal/storage/
- [ ] btrfs.go - btrfs 命令封装
- [ ] volume.go - 卷管理 (创建/删除/扩容)
- [ ] subvolume.go - 子卷管理
- [ ] snapshot.go - 快照管理
- [ ] raid.go - RAID 配置
- [ ] balance.go - 数据平衡
- [ ] scrub.go - 数据校验
```

#### M3 文件共享 (优先级：P1)
```go
// 文件：internal/
- [ ] smb/server.go - SMB 服务
- [ ] nfs/server.go - NFS 服务
- [ ] shares/manager.go - 共享配置管理
```

#### M4 用户权限 (优先级：P1)
```go
// 文件：internal/
- [ ] users/manager.go - 用户 CRUD
- [ ] groups/manager.go - 用户组管理
- [ ] auth/middleware.go - 认证中间件
- [ ] rbac/policies.go - 权限策略
```

### 技术规范
- 代码审查：所有 PR 需兵部审核
- 测试覆盖：核心模块 >80%
- 文档：每个公开函数需 Godoc 注释

---

## 户部 - 财务预算

### 核心职责
- 项目预算规划
- 硬件采购建议
- 云资源成本核算

### 具体任务
- [ ] 制定项目预算表
- [ ] 评估硬件需求 (硬盘/内存/网络)
- [ ] 云备份方案成本分析
- [ ] 月度资源消耗报告

### 交付物
- `docs/budget.md` - 预算文档
- `docs/hardware-requirements.md` - 硬件需求

---

## 礼部 - 品牌文档

### 核心职责
- UI/UX 设计
- 文档撰写
- 品牌视觉

### 具体任务

#### M2 Web 界面 (优先级：P1)
```
// 目录：webui/
- [ ] 设计系统规范 (颜色/字体/组件)
- [ ] 登录页面
- [ ] 仪表盘页面
- [ ] 存储管理页面
- [ ] 文件浏览器页面
- [ ] 用户管理页面
- [ ] 设置页面
```

#### 文档 (优先级：P2)
```
// 目录：docs/
- [ ] 用户手册
- [ ] 安装指南
- [ ] API 文档 (Swagger)
- [ ] 开发者指南
- [ ] FAQ
```

### 交付物
- `webui/design-system.md` - 设计规范
- `docs/user-guide.md` - 用户手册
- `docs/api.yaml` - API 文档

---

## 工部 - DevOps 运维

### 核心职责
- 部署脚本
- 监控系统
- Docker 集成
- CI/CD

### 具体任务

#### M2 Web 服务 (优先级：P1)
```go
// 文件：internal/web/
- [ ] server.go - Web 服务器
- [ ] handlers/ - 路由处理器
- [ ] middleware/ - 中间件
```

#### M5 监控告警 (优先级：P2)
```go
// 文件：internal/monitor/
- [ ] health.go - 健康检查
- [ ] metrics.go - 指标收集
- [ ] alerts.go - 告警管理
- [ ] notify.go - 通知发送
```

#### M6 Docker 集成 (优先级：P3)
```go
// 文件：internal/docker/
- [ ] manager.go - Docker 管理
- [ ] apps/catalog.go - 应用目录
- [ ] network/config.go - 网络配置
```

#### 运维脚本
```bash
# 目录：scripts/
- [ ] install.sh - 安装脚本
- [ ] backup.sh - 备份脚本
- [ ] deploy.sh - 部署脚本
- [ ] docker-compose.yaml - 容器编排
```

#### CI/CD
```yaml
# 文件：.github/workflows/
- [ ] build.yaml - 构建流程
- [ ] test.yaml - 测试流程
- [ ] release.yaml - 发布流程
```

### 交付物
- `scripts/` - 运维脚本
- `.github/workflows/` - CI/CD 配置
- `internal/monitor/` - 监控系统

---

## 吏部 - 项目管理

### 核心职责
- 项目规划
- 进度跟踪
- 资源协调
- 风险管理

### 具体任务
- [x] 创建项目里程碑
- [x] 分配六部任务
- [ ] 每周进度汇总
- [ ] 风险识别与缓解
- [ ] 跨部门协调
- [ ] 版本发布管理

### 交付物
- `MILESTONES.md` - 项目里程碑
- `TASKS.md` - 任务分配 (本文件)
- `docs/changelog.md` - 变更日志
- `docs/release-plan.md` - 发布计划

---

## 刑部 - 安全合规

### 核心职责
- 安全审计
- 权限审查
- 合规检查
- 漏洞管理

### 具体任务

#### M4 安全模块 (优先级：P1)
```go
// 文件：internal/security/
- [ ] audit/logger.go - 审计日志
- [ ] encryption/keys.go - 密钥管理
- [ ] firewall/rules.go - 防火墙规则
```

#### 安全审查
- [ ] 代码安全扫描
- [ ] 依赖漏洞检查
- [ ] 权限模型审查
- [ ] 数据加密方案
- [ ] 合规性检查 (隐私政策)

### 交付物
- `docs/security-policy.md` - 安全策略
- `docs/audit-log.md` - 审计规范
- `SECURITY.md` - 安全报告模板

---

## 任务优先级说明

| 优先级 | 说明 | 时间要求 |
|--------|------|----------|
| P0 | 核心功能，阻塞后续开发 | 立即开始 |
| P1 | 重要功能，影响用户体验 | 2 周内 |
| P2 | 增强功能，可延后 | 1 个月内 |
| P3 | 可选功能，视资源而定 | 2 个月内 |

---

## 进度汇报机制

### 每日
- 各部门在 Discord 汇报当日进展
- 吏部汇总至 `docs/daily-standup.md`

### 每周
- 周日晚 20:00 进度同步会
- 更新 `MILESTONES.md` 状态
- 识别风险与阻塞

### 每月
- 月末版本发布
- 更新 `docs/changelog.md`
- 复盘会议

---

## 联系方式

| 部门 | Discord 频道 | 负责人 |
|------|-------------|--------|
| 兵部 | #engineering | TBD |
| 户部 | #finance | TBD |
| 礼部 | #design-docs | TBD |
| 工部 | #devops | TBD |
| 吏部 | #project-mgmt | TBD |
| 刑部 | #security | TBD |

---

*最后更新：2026-03-10*
