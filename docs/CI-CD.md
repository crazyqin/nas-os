# CI/CD 流程文档

## 📊 概览

NAS-OS 使用 GitHub Actions 实现完整的 CI/CD 自动化流程。

## 🔄 工作流程

### 1. CI/CD Pipeline (`ci-cd.yml`)

**触发条件:**
- 推送到 `main` / `develop` 分支
- 创建 `v*` 标签
- Pull Request
- 手动触发

**流程步骤:**

```
┌─────────────┐
│  代码提交   │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  代码检查   │ ← lint (golangci-lint, gofmt)
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  单元测试   │ ← test (go test -race)
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ 集成测试    │ ← test-integration
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ 多平台构建  │ ← build (amd64, arm64, armv7)
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Docker 镜像 │ ← docker (多架构)
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  发布版本   │ ← release (仅 tag 触发)
└─────────────┘
```

### 2. Docker 发布 (`docker-publish.yml`)

**功能:**
- 多架构镜像构建（amd64, arm64, arm/v7）
- 自动推送到 GitHub Container Registry
- 可选推送到 Docker Hub
- 安全扫描（Trivy）
- Docker Compose 测试

**镜像标签策略:**
- `main` 分支 → `latest`, `main-{sha}`
- `develop` 分支 → `develop-{sha}`
- 标签 `v1.2.3` → `1.2.3`, `1.2`, `1`
- PR → `pr-{number}`

### 3. 安全扫描 (`security-scan.yml`)

**触发条件:**
- 推送到 `main` / `develop`
- Pull Request
- 每周一 02:00 UTC 定时扫描

**扫描工具:**
- `gosec` - Go 代码安全分析
- `govulncheck` - Go 依赖漏洞检查
- `Trivy` - Docker 镜像扫描

## 📁 目录结构

```
.github/
├── workflows/
│   ├── ci-cd.yml           # 主 CI/CD 流程
│   ├── docker-publish.yml  # Docker 镜像构建
│   └── security-scan.yml   # 安全扫描
├── RELEASE_TEMPLATE.md     # 发布说明模板
└── RELEASE_CHECKLIST.md    # 发布检查清单
```

## 🧪 测试

### 运行测试

```bash
# 单元测试
make test

# 集成测试
make test-integration

# 所有测试
make test-all

# 覆盖率报告
make test-coverage

# 竞态检测
make test-race
```

### 测试覆盖率要求

- **最低覆盖率**: 50%
- **目标覆盖率**: 80%
- 低于 50% 会导致 CI 失败

### 测试类型

1. **单元测试** (`*_test.go`)
   - 位于各包内
   - 测试单个函数/方法
   - 使用 mock 隔离依赖

2. **集成测试** (`tests/integration/`)
   - 测试模块间交互
   - 端到端场景验证
   - 并发和边界条件测试

## 🏗️ 构建

### 多平台构建

支持以下目标平台：

| 平台 | GOOS | GOARCH | GOARM | 设备示例 |
|------|------|--------|-------|----------|
| Linux x86_64 | linux | amd64 | - | Intel/AMD 服务器 |
| Linux ARM64 | linux | arm64 | - | 树莓派 4/5, Orange Pi 5 |
| Linux ARMv7 | linux | arm | 7 | 树莓派 3/Zero |

### 构建命令

```bash
# 本地构建（当前架构）
make build

# 多平台构建
make build-all

# 调试版本
make build-debug
```

## 🐳 Docker

### 构建镜像

```bash
# 构建
make docker-build

# 运行
make docker-run

# 查看日志
make docker-logs

# 停止
make docker-stop
```

### 镜像架构

```dockerfile
# 多阶段构建
Stage 1: builder (golang:1.25-alpine)
  - 编译 Go 二进制
  
Stage 2: runtime (alpine:3.21)
  - 复制编译产物
  - 安装运行时依赖
  - 最终镜像 ~30MB
```

## 📦 发布

### 创建发布

```bash
# 1. 确保所有测试通过
make test-all

# 2. 更新版本号
# 编辑相关文件...

# 3. 提交并打标签
git commit -am "chore: release v1.0.0"
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

### 自动发布内容

GitHub Actions 会自动：
- ✅ 编译多平台二进制
- ✅ 生成 SHA256 校验和
- ✅ 构建并推送 Docker 镜像
- ✅ 创建 GitHub Release
- ✅ 上传构建产物
- ✅ 生成发布说明

### 发布文件

每个 Release 包含：
- `nasd-linux-amd64` - x86_64 二进制
- `nasd-linux-arm64` - ARM64 二进制
- `nasd-linux-armv7` - ARMv7 二进制
- `nasctl-*` - 对应平台的 CLI 工具
- `checksums.txt` - SHA256 校验和

## 🔒 安全

### 安全扫描

```bash
# 本地运行 gosec
go install github.com/securego/gosec/v2/cmd/gosec@latest
gosec ./...

# 运行 govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

### 安全最佳实践

- ✅ 定期更新依赖 (`go mod tidy`)
- ✅ 使用静态链接（CGO_ENABLED=0）
- ✅ 最小化 Docker 镜像（Alpine）
- ✅ 启用健康检查
- ✅ 扫描漏洞（Trivy, gosec）

## 📊 监控

### CI/CD 状态

- [GitHub Actions](../../actions) - 查看构建历史
- [Security](../../security) - 查看安全扫描结果
- [Packages](../../pkgs) - 查看 Docker 镜像

### 指标

- 构建成功率
- 测试覆盖率
- 平均构建时间
- 漏洞数量趋势

## 🛠️ 故障排查

### 构建失败

1. **检查日志**
   ```bash
   # 在 GitHub Actions 页面查看详细日志
   ```

2. **本地复现**
   ```bash
   make build-all
   make test-all
   ```

3. **常见问题**
   - 依赖缺失：`go mod download`
   - 测试失败：`go test -v ./...`
   - 磁盘空间：清理构建缓存

### Docker 推送失败

1. 检查登录状态
2. 验证权限（packages: write）
3. 确认镜像名称正确

### 发布失败

1. 不要删除失败的 tag
2. 修复问题后创建新 tag（如 v1.0.1）
3. 删除失败的 Release 草稿

## 📚 相关文档

- [DEPLOYMENT.md](../DEPLOYMENT.md) - 部署指南
- [RELEASE_CHECKLIST.md](../../.github/RELEASE_CHECKLIST.md) - 发布检查清单
- [CONTRIBUTING.md](../../CONTRIBUTING.md) - 贡献指南
