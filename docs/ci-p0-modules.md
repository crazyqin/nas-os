# P0 模块 CI/CD 配置说明

## 概述

本文档说明 NAS-OS CI/CD 配置对 P0 新模块（Drive/Audio/Calendar/Contacts）的支持情况。

---

## 现有 CI 配置

### 配置文件位置

```
nas-os/.github/workflows/
├── ci-cd.yml           # 主 CI/CD 流程
├── benchmark.yml       # 性能基准测试
├── docker-publish.yml  # Docker 镜像发布
├── release.yml         # 版本发布
└── security-scan.yml   # 安全扫描
```

### CI 流程阶段

| 阶段 | 任务 | 说明 |
|------|------|------|
| 变更检测 | changes | 智能跳过无需运行的 job |
| 环境准备 | prepare | Go 缓存预热 |
| 代码质量 | lint, codeql | golangci-lint + CodeQL |
| 安全扫描 | dependency-scan | Trivy 依赖扫描 |
| 单元测试 | test | 覆盖率报告生成 |
| 集成测试 | test-integration | 条件触发 |
| 构建 | build | 多平台二进制 |
| 验证 | docker-compose-test | Docker Compose 测试 |

---

## P0 模块支持情况

### ✅ Go 版本一致性

- **CI Go 版本**: `1.26` (env.GO_VERSION)
- **go.mod 版本**: `go 1.26.1`
- **状态**: ✅ 一致

### ✅ 测试命令覆盖新模块

CI 使用以下命令运行测试：

```yaml
go test -v -race -coverprofile=coverage.out -covermode=atomic -timeout 10m \
  $(go list ./... | grep -v /tests/e2e | grep -v /tests/fixtures | grep -v /tests/reports | grep -v /tests/benchmark | grep -v /plugins/)
```

**结论**: `go list ./...` 自动发现所有模块，无需修改 CI 配置。

### ✅ 安全扫描包含新模块

| 扫描工具 | 覆盖范围 |
|----------|----------|
| golangci-lint | 全部 Go 文件 |
| CodeQL | `go build ./...` |
| Trivy | 文件系统扫描 (.) |
| gosec | 通过 golangci-lint 集成 |

**结论**: 安全扫描自动覆盖新模块。

### ✅ Docker 构建包含新依赖

Docker 构建基于 `go.mod`，新模块添加依赖后：
1. 运行 `go mod tidy`
2. CI 自动下载新依赖
3. Docker 镜像自动包含

---

## 新模块集成检查清单

### 开发阶段

- [ ] 在 `internal/` 目录下创建模块目录
- [ ] 编写模块代码和测试文件 (`*_test.go`)
- [ ] 运行 `go mod tidy` 更新依赖
- [ ] 本地运行 `make test` 验证

### 模块目录结构建议

```
internal/
├── drive/           # Drive 模块（已创建，待开发）
│   ├── handler.go
│   ├── service.go
│   ├── handler_test.go
│   └── service_test.go
├── audio/           # Audio 模块（待创建）
│   ├── handler.go
│   ├── service.go
│   ├── handler_test.go
│   └── service_test.go
├── calendar/        # Calendar 模块（待创建）
│   ├── handler.go
│   ├── service.go
│   ├── handler_test.go
│   └── service_test.go
└── contacts/        # Contacts 模块（待创建）
    ├── handler.go
    ├── service.go
    ├── handler_test.go
    └── service_test.go
```

### 测试覆盖率要求

| 模块类型 | 最低覆盖率 | 建议覆盖率 |
|----------|------------|------------|
| 核心模块 | 40% | 60%+ |
| API Handler | 50% | 70%+ |
| 工具函数 | 60% | 80%+ |

**当前 CI 阈值**: 25%（最低），50%（警告）

---

## CI 触发条件

```yaml
on:
  push:
    branches: [master, develop]
  pull_request:
    branches: [master]
  workflow_dispatch:
```

新模块代码提交到 `master` 或 `develop` 分支时自动触发 CI。

---

## 缓存策略

CI 使用多级缓存加速构建：

| 缓存类型 | Key 格式 |
|----------|----------|
| Go 模块 | `go-{CACHE_VERSION}-{go.sum hash}` |
| Go 编译 | `go-build-{CACHE_VERSION}-{os}-{arch}-{go.sum hash}` |
| Go 工具 | `go-tools-{CACHE_VERSION}-{os}` |

**CACHE_VERSION**: `v12`（重大变更时递增）

---

## 常见问题

### Q: 新模块测试未运行？

A: 检查：
1. 测试文件命名是否为 `*_test.go`
2. 测试函数是否以 `Test` 开头
3. 是否在 `tests/e2e` 或 `tests/fixtures` 目录下（会被排除）

### Q: 如何查看模块覆盖率？

A: CI 生成的覆盖率报告按包统计，可在 GitHub Actions Summary 中查看。

### Q: 如何跳过 CI？

A: 提交信息包含 `[skip ci]` 或 `[ci skip]` 可跳过 CI。

---

## 相关文档

- [CI/CD 详细配置](./CI-CD.md)
- [开发者指南](./DEVELOPER.md)
- [代码风格](./CODE_STYLE.md)
- [测试计划](./TEST_PLAN_v2.120.0.md)

---

## 更新记录

| 日期 | 版本 | 变更 |
|------|------|------|
| 2026-03-23 | v1.0 | 初始版本，确认 CI 对 P0 模块支持 |