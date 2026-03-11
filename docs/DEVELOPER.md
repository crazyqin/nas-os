# NAS-OS 开发者文档

## 🏗️ 架构概览

```
┌─────────────────────────────────────────────────────────────┐
│                        Web UI (HTML/JS)                      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      HTTP API Server                         │
│                    (internal/web/handler.go)                 │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        ▼                     ▼                     ▼
┌───────────────┐    ┌───────────────┐    ┌───────────────┐
│   Storage     │    │     SMB       │    │     NFS       │
│   Manager     │    │   Service     │    │   Service     │
│ (btrfs ops)   │    │  (Samba)      │    │  (nfsd)       │
└───────────────┘    └───────────────┘    └───────────────┘
        │
        ▼
┌───────────────┐
│   btrfs       │
│  Filesystem   │
└───────────────┘
```

## 📂 项目结构

```
nas-os/
├── cmd/
│   ├── nasd/          # 主服务进程
│   │   └── main.go    # 服务启动入口
│   └── nasctl/        # 命令行工具
│       └── main.go    # CLI 入口
├── internal/
│   ├── storage/       # 存储管理核心
│   │   ├── volume.go  # 卷操作
│   │   ├── subvolume.go
│   │   ├── snapshot.go
│   │   └── btrfs.go   # btrfs 底层调用
│   ├── web/           # Web 服务
│   │   ├── server.go  # HTTP 服务器
│   │   ├── handler.go # API 处理器
│   │   └── middleware.go
│   ├── smb/           # SMB 共享
│   │   └── samba.go   # Samba 配置与管理
│   ├── nfs/           # NFS 共享
│   │   └── nfs.go     # NFS 配置与管理
│   ├── users/         # 用户管理
│   │   └── users.go   # 用户 CRUD
│   ├── monitor/       # 系统监控
│   │   └── health.go  # 健康检查
│   └── docker/        # Docker 集成（开发中）
├── pkg/               # 公共库
│   ├── config/        # 配置管理
│   ├── logger/        # 日志工具
│   └── utils/         # 通用工具函数
├── webui/             # 前端界面
│   └── index.html     # 单页应用
├── configs/           # 配置文件
│   └── nasd.yaml      # 服务配置模板
├── go.mod             # Go 模块定义
└── go.sum             # 依赖校验
```

## 🔧 开发环境搭建

### 前置要求

- Go 1.21+
- btrfs-progs
- Git

### 克隆项目

```bash
git clone https://github.com/crazyqin/nas-os.git
cd nas-os
```

### 安装依赖

```bash
go mod tidy
```

### 运行测试

```bash
# 运行所有测试
go test ./...

# 运行特定包测试
go test ./internal/storage/...

# 带覆盖率
go test -cover ./...
```

### 本地开发

```bash
# 启动服务（开发模式）
sudo go run ./cmd/nasd/main.go

# 或使用 air 进行热重载
# go install github.com/air-verse/air@latest
air
```

## 📡 API 参考

### 存储卷 API

#### GET /api/v1/volumes
获取卷列表

**响应示例**:
```json
{
  "volumes": [
    {
      "name": "myvolume",
      "device": "/dev/sdb1",
      "size": 1099511627776,
      "used": 549755813888,
      "status": "healthy"
    }
  ]
}
```

#### POST /api/v1/volumes
创建新卷

**请求**:
```json
{
  "name": "myvolume",
  "device": "/dev/sdb1"
}
```

#### GET /api/v1/volumes/:name
获取卷详情

#### DELETE /api/v1/volumes/:name
删除卷

### 子卷 API

#### POST /api/v1/volumes/:name/subvolumes
创建子卷

**请求**:
```json
{
  "name": "docker"
}
```

#### GET /api/v1/volumes/:name/subvolumes
列出子卷

### 快照 API

#### POST /api/v1/volumes/:name/snapshots
创建快照

**请求**:
```json
{
  "name": "snapshot-2026-03-10"
}
```

#### POST /api/v1/volumes/:name/snapshots/:snapshot/restore
恢复快照

### 系统 API

#### GET /api/v1/status
系统状态

#### GET /api/v1/disks
磁盘列表

#### GET /api/v1/resources
系统资源使用情况

## 🔌 扩展开发

### 添加新的 API 端点

1. 在 `internal/web/handler.go` 添加处理函数
2. 在 `internal/web/server.go` 注册路由
3. 编写测试用例
4. 更新 API 文档

### 添加新的存储后端

1. 在 `internal/storage/` 创建新文件
2. 实现 `StorageBackend` 接口
3. 在配置中支持后端选择

### 开发 Web UI 组件

1. 编辑 `webui/index.html`
2. 调用 API 获取数据
3. 遵循品牌规范（见 `BRAND.md`）

## 🧪 测试指南

### 单元测试

```go
// internal/storage/volume_test.go
package storage

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestCreateVolume(t *testing.T) {
    // 测试代码
    assert.NotNil(t, volume)
}
```

### 集成测试

```bash
# 需要 root 权限和测试设备
sudo go test -tags=integration ./...
```

### 端到端测试

```bash
# 使用测试脚本
./scripts/e2e-test.sh
```

## 📝 代码规范

### Go 代码风格

- 遵循 [Effective Go](https://go.dev/doc/effective_go)
- 使用 `gofmt` 格式化代码
- 使用 `golint` 检查代码质量

```bash
# 格式化
gofmt -w .

# 检查
golint ./...
```

### 提交信息规范

```
feat: 添加快照恢复功能
fix: 修复卷创建时的权限问题
docs: 更新 API 文档
refactor: 重构存储管理模块
test: 添加单元测试
```

### Git 工作流

```bash
# 创建功能分支
git checkout -b feature/snapshot-restore

# 提交代码
git add .
git commit -m "feat: 添加快照恢复功能"

# 推送并创建 PR
git push origin feature/snapshot-restore
```

## 🔒 安全考虑

- 所有磁盘操作需要 root 权限
- API 需要认证（开发中）
- 敏感配置需要加密存储
- 定期更新依赖

## 📦 发布流程

1. 更新版本号 (`go.mod`)
2. 更新 CHANGELOG.md
3. 打标签 `git tag v1.0.0`
4. 推送标签 `git push origin v1.0.0`
5. GitHub Actions 自动构建发布

---

*开发者文档版本：1.0 | 最后更新：2026-03-10*
