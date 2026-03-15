# NAS-OS 贡献指南

感谢你考虑为 NAS-OS 做出贡献！本文档将帮助你了解如何参与项目开发。

## 📋 目录

- [行为准则](#行为准则)
- [如何贡献](#如何贡献)
- [开发环境](#开发环境)
- [代码规范](#代码规范)
- [提交规范](#提交规范)
- [Pull Request 流程](#pull-request-流程)
- [Issue 指南](#issue-指南)
- [项目结构](#项目结构)

---

## 行为准则

- 尊重所有贡献者
- 建设性的讨论和反馈
- 关注对项目最有利的事情
- 对新手友好和耐心

---

## 如何贡献

### 报告 Bug

1. 搜索 [现有 Issues](https://github.com/crazyqin/nas-os/issues) 确保问题未被报告
2. 使用 Bug Report 模板创建新 Issue
3. 提供详细的复现步骤和环境信息

### 提出新功能

1. 先在 [Discussions](https://github.com/crazyqin/nas-os/discussions) 讨论你的想法
2. 使用 Feature Request 模板创建 Issue
3. 说明使用场景和技术方案

### 提交代码

1. Fork 仓库
2. 创建功能分支
3. 编写代码和测试
4. 提交 Pull Request

---

## 开发环境

### 前置要求

| 工具 | 版本 | 说明 |
|------|------|------|
| Go | 1.21+ | 编程语言 |
| btrfs-progs | 最新 | 存储工具 |
| Docker | 20.10+ | 容器运行时 |
| Git | 2.x | 版本控制 |

### 安装开发工具

```bash
# Go 开发工具
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/swaggo/swag/cmd/swag@latest

# 代码格式化
go install mvdan.cc/gofumpt@latest
```

### 克隆项目

```bash
git clone https://github.com/crazyqin/nas-os.git
cd nas-os
go mod tidy
```

### 运行测试

```bash
# 单元测试
make test

# 覆盖率报告
make test-coverage

# 竞态检测
make test-race

# 完整测试套件
make test-all
```

### 本地运行

```bash
# 编译
make build

# 开发模式（需要 root）
sudo ./nasd

# 或使用 air 热重载
air
```

---

## 代码规范

### Go 代码风格

遵循 [Effective Go](https://go.dev/doc/effective_go) 和 [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)。

**核心原则**:

1. **命名规范**
   - 包名：小写单词，不使用下划线
   - 导出函数/类型：大写开头，驼峰命名
   - 私有函数/变量：小写开头，驼峰命名
   - 接口：动词或 `-er` 后缀（如 `Reader`, `Writer`）

2. **错误处理**
   ```go
   // ✅ 正确：立即处理错误
   if err := doSomething(); err != nil {
       return fmt.Errorf("failed to do something: %w", err)
   }
   
   // ❌ 错误：忽略错误
   doSomething()
   ```

3. **注释规范**
   ```go
   // DoSomething 执行某操作。
   // 参数 name 指定操作名称。
   // 返回操作结果或错误。
   func DoSomething(name string) (*Result, error) {
       // ...
   }
   ```

4. **代码组织**
   - 按功能模块组织代码
   - 相关文件放在同一包中
   - 接口定义与实现分离

### 代码质量检查

```bash
# 格式化
gofmt -w .

# 静态检查
golangci-lint run

# 代码检查
go vet ./...
```

### 文件结构

```
internal/module/
├── module.go       # 模块定义和接口
├── handler.go      # API 处理器
├── service.go      # 业务逻辑
├── repository.go   # 数据访问
├── model.go        # 数据模型
└── module_test.go  # 单元测试
```

---

## 提交规范

### Commit 信息格式

遵循 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Type 类型

| 类型 | 说明 | 示例 |
|------|------|------|
| `feat` | 新功能 | feat(storage): 添加快照恢复功能 |
| `fix` | Bug 修复 | fix(smb): 修复权限验证问题 |
| `docs` | 文档更新 | docs: 更新 API 文档 |
| `style` | 代码格式 | style: 格式化代码 |
| `refactor` | 重构 | refactor(storage): 优化卷管理逻辑 |
| `test` | 测试 | test(quota): 添加配额单元测试 |
| `chore` | 构建/工具 | chore: 更新 CI 配置 |
| `perf` | 性能优化 | perf(cache): 优化 LRU 缓存性能 |

### Scope 范围

常见 scope：
- `storage` - 存储管理
- `smb` / `nfs` - 文件共享
- `users` - 用户管理
- `monitor` - 系统监控
- `docker` - Docker 集成
- `api` - API 相关
- `webui` - Web 界面
- `docs` - 文档

### 示例

```bash
# 简单提交
git commit -m "feat(storage): 添加快照恢复功能"

# 带 Body 的提交
git commit -m "feat(storage): 添加快照恢复功能" -m "
- 支持 BTRFS 快照恢复
- 支持增量恢复
- 添加恢复进度跟踪

Closes #123"

# 破坏性变更
git commit -m "feat(api)!: 重构存储 API" -m "
BREAKING CHANGE: 存储卷 API 响应格式变更
- 使用 `volume` 替代 `vol`
- 添加 `status` 字段"
```

---

## Pull Request 流程

### 创建 PR

1. **Fork 并创建分支**
   ```bash
   git checkout -b feature/my-feature
   ```

2. **编写代码**
   - 遵循代码规范
   - 添加必要的测试
   - 更新相关文档

3. **本地验证**
   ```bash
   make test-all
   make lint
   make build
   ```

4. **提交并推送**
   ```bash
   git add .
   git commit -m "feat(module): 添加某功能"
   git push origin feature/my-feature
   ```

5. **创建 Pull Request**
   - 使用 PR 模板
   - 关联相关 Issue
   - 描述变更内容

### PR 检查清单

- [ ] 代码通过所有测试
- [ ] 代码通过 lint 检查
- [ ] 新功能有对应测试
- [ ] 文档已更新
- [ ] CHANGELOG 已更新（如适用）
- [ ] PR 标题符合提交规范

### Review 流程

1. 至少需要 1 个 Reviewer 批准
2. CI 检查必须全部通过
3. 解决所有 Review 意见
4. Squash 合并到 main 分支

---

## Issue 指南

### Bug Report

使用 Bug Report 模板，包含：
- 清晰的问题描述
- 复现步骤
- 期望行为 vs 实际行为
- 环境信息
- 相关日志

### Feature Request

使用 Feature Request 模板，包含：
- 功能描述和使用场景
- 技术方案（可选）
- 影响评估

---

## 项目结构

```
nas-os/
├── cmd/                    # 可执行程序
│   ├── nasd/              # 主服务
│   └── nasctl/            # CLI 工具
├── internal/              # 内部模块
│   ├── storage/           # 存储管理
│   ├── smb/               # SMB 服务
│   ├── nfs/               # NFS 服务
│   ├── users/             # 用户管理
│   ├── monitor/           # 系统监控
│   ├── docker/            # Docker 集成
│   └── web/               # Web 服务
├── pkg/                   # 公共库
├── webui/                 # 前端界面
├── docs/                  # 文档
├── configs/               # 配置文件
├── scripts/               # 脚本工具
└── tests/                 # 测试文件
```

### 模块说明

| 目录 | 说明 |
|------|------|
| `cmd/nasd` | 主服务入口 |
| `internal/` | 内部实现，不对外暴露 |
| `pkg/` | 可复用的公共库 |
| `docs/` | 用户和开发文档 |
| `tests/` | 集成测试和 E2E 测试 |

---

## 获取帮助

- 📖 [文档](docs/)
- 💬 [Discussions](https://github.com/crazyqin/nas-os/discussions)
- 🐛 [Issues](https://github.com/crazyqin/nas-os/issues)

---

## 发布检查清单

### 版本发布前检查

- [ ] VERSION 文件已更新
- [ ] MILESTONES.md 已更新
- [ ] ROADMAP.md 已更新
- [ ] CHANGELOG.md 已更新
- [ ] README.md 版本号同步
- [ ] 所有测试通过 (`make test-all`)
- [ ] 代码检查通过 (`golangci-lint run`)
- [ ] 安全审计通过
- [ ] 文档已同步更新

### 版本号规范

遵循语义化版本 (Semantic Versioning)：

- **主版本号 (MAJOR)**: 不兼容的 API 变更
- **次版本号 (MINOR)**: 向后兼容的功能新增
- **修订号 (PATCH)**: 向后兼容的问题修复

示例：
- `v2.89.0` - 次版本更新，新增功能
- `v2.89.1` - 修订版本，修复问题
- `v3.0.0` - 主版本更新，重大变更

### 发布流程

1. **准备阶段**
   - 更新版本号文件
   - 更新文档和 CHANGELOG
   - 运行完整测试套件

2. **审核阶段**
   - 代码审核
   - 安全审计
   - 文档审核

3. **发布阶段**
   - 创建 Git Tag
   - 构建 Release 版本
   - 发布 Release Notes

4. **发布后**
   - 监控错误报告
   - 收集用户反馈
   - 规划下一版本

---

*感谢你的贡献！* 🎉