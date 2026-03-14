# NAS-OS 代码规范

本文档定义了 NAS-OS 项目的编码标准和最佳实践。

## 📋 目录

- [Go 代码规范](#go-代码规范)
- [项目约定](#项目约定)
- [错误处理](#错误处理)
- [日志规范](#日志规范)
- [测试规范](#测试规范)
- [API 设计规范](#api-设计规范)
- [数据库规范](#数据库规范)
- [文档规范](#文档规范)

---

## Go 代码规范

### 基本原则

遵循以下官方指南：
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Go 风格指南](https://google.github.io/styleguide/go/)

### 命名规范

#### 包名

```go
// ✅ 正确：小写单词，简洁有意义
package storage
package quota

// ❌ 错误：使用下划线或驼峰
package storage_manager
package storageManager
```

#### 导出名称

```go
// ✅ 正确：大写开头，驼峰命名
type VolumeManager struct {}
func CreateVolume(name string) error {}
const MaxRetryCount = 3

// ❌ 错误：全大写或下划线
type VOLUME_MANAGER struct {}
func CREATE_VOLUME(name string) error {}
const MAX_RETRY_COUNT = 3
```

#### 接口命名

```go
// ✅ 正确：动词或 -er 后缀
type Reader interface { Read() error }
type Writer interface { Write() error }
type StorageManager interface { ... }

// ❌ 错误
type ReadInterface interface { ... }
type IStorage interface { ... }
```

#### 私有变量

```go
// ✅ 正确：小写开头，驼峰命名
type Manager struct {
    config    *Config
    cache     map[string]string
    mutex     sync.RWMutex
}
```

### 代码组织

#### 文件结构

```go
// 1. 包注释
// Package storage 提供 BTRFS 存储管理功能。
package storage

// 2. 导入（标准库、第三方、内部包）
import (
    "context"
    "fmt"
    
    "github.com/gin-gonic/gin"
    
    "github.com/crazyqin/nas-os/pkg/config"
)

// 3. 常量
const (
    defaultTimeout = 30 * time.Second
    maxRetryCount  = 3
)

// 4. 变量
var (
    ErrVolumeNotFound = errors.New("volume not found")
)

// 5. 类型定义
type Volume struct {
    Name   string
    Path   string
    Status string
}

// 6. 函数实现
func (v *Volume) Create() error {
    // ...
}
```

#### 函数长度

- 单个函数不超过 80 行
- 复杂逻辑拆分为子函数
- 每个函数只做一件事

```go
// ✅ 正确：职责单一
func (m *Manager) CreateVolume(ctx context.Context, name string) (*Volume, error) {
    if err := m.validateName(name); err != nil {
        return nil, err
    }
    
    vol, err := m.doCreate(ctx, name)
    if err != nil {
        return nil, fmt.Errorf("create volume: %w", err)
    }
    
    return vol, nil
}
```

---

## 项目约定

### 目录结构

```
internal/module/
├── module.go       # 模块定义、接口
├── handler.go      # HTTP 处理器
├── service.go      # 业务逻辑
├── repository.go   # 数据访问
├── model.go        # 数据模型
├── errors.go       # 错误定义
└── module_test.go  # 单元测试
```

### 依赖注入

```go
// ✅ 正确：通过构造函数注入
type Service struct {
    repo   Repository
    cache  Cache
    logger Logger
}

func NewService(repo Repository, cache Cache, logger Logger) *Service {
    return &Service{
        repo:   repo,
        cache:  cache,
        logger: logger,
    }
}

// ❌ 错误：全局变量
var repo Repository
var cache Cache
```

### 配置管理

```go
// ✅ 正确：配置结构体
type Config struct {
    Port    int    `yaml:"port"`
    DataDir string `yaml:"data_dir"`
}

// ❌ 错误：硬编码
func Start() {
    port := 8080  // 应该从配置读取
}
```

---

## 错误处理

### 错误定义

```go
// errors.go
package storage

import "errors"

var (
    // ErrVolumeNotFound 卷不存在
    ErrVolumeNotFound = errors.New("volume not found")
    // ErrVolumeExists 卷已存在
    ErrVolumeExists = errors.New("volume already exists")
    // ErrInvalidName 无效名称
    ErrInvalidName = errors.New("invalid volume name")
)
```

### 错误包装

```go
// ✅ 正确：使用 %w 包装错误
func (m *Manager) GetVolume(name string) (*Volume, error) {
    vol, err := m.repo.FindByName(name)
    if err != nil {
        return nil, fmt.Errorf("find volume %s: %w", name, err)
    }
    return vol, nil
}

// ❌ 错误：丢失错误信息
func (m *Manager) GetVolume(name string) (*Volume, error) {
    vol, err := m.repo.FindByName(name)
    if err != nil {
        return nil, errors.New("volume not found")  // 丢失原始错误
    }
    return vol, nil
}
```

### 错误检查

```go
// ✅ 正确：使用 errors.Is/As
if errors.Is(err, ErrVolumeNotFound) {
    // 处理卷不存在的情况
}

var customErr *CustomError
if errors.As(err, &customErr) {
    // 处理自定义错误
}
```

### Panic 使用

```go
// ✅ 正确：仅在不可恢复时使用
func MustParseConfig(path string) *Config {
    cfg, err := ParseConfig(path)
    if err != nil {
        panic(fmt.Sprintf("failed to parse config: %v", err))
    }
    return cfg
}

// ❌ 错误：用于普通错误处理
func CreateVolume(name string) error {
    if name == "" {
        panic("name is empty")  // 应该返回 error
    }
}
```

---

## 日志规范

### 日志级别

| 级别 | 用途 | 示例 |
|------|------|------|
| DEBUG | 调试信息 | `logger.Debug("processing request", "id", reqID)` |
| INFO | 正常事件 | `logger.Info("volume created", "name", name)` |
| WARN | 警告信息 | `logger.Warn("disk usage high", "usage", usage)` |
| ERROR | 错误信息 | `logger.Error("failed to create volume", "error", err)` |

### 日志格式

```go
// ✅ 正确：结构化日志
logger.Info("volume created",
    "name", vol.Name,
    "size", vol.Size,
    "duration", time.Since(start),
)

// ❌ 错误：字符串拼接
logger.Info(fmt.Sprintf("volume %s created, size: %d", vol.Name, vol.Size))
```

### 敏感信息

```go
// ✅ 正确：脱敏处理
logger.Info("user login",
    "username", user.Name,
    "password", "***",  // 脱敏
)

// ❌ 错误：记录敏感信息
logger.Info("user login",
    "username", user.Name,
    "password", user.Password,  // 危险！
)
```

---

## 测试规范

### 测试命名

```go
// ✅ 正确：Test + 函数名 + 场景
func TestCreateVolume_Success(t *testing.T) {}
func TestCreateVolume_AlreadyExists(t *testing.T) {}
func TestCreateVolume_InvalidName(t *testing.T) {}

// ❌ 错误
func TestVolume(t *testing.T) {}
func TestCreate(t *testing.T) {}
```

### 表格驱动测试

```go
func TestValidateName(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr error
    }{
        {"valid name", "myvolume", nil},
        {"empty name", "", ErrInvalidName},
        {"too long", strings.Repeat("a", 256), ErrInvalidName},
        {"special chars", "vol-1!", ErrInvalidName},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateName(tt.input)
            if !errors.Is(err, tt.wantErr) {
                t.Errorf("got err %v, want %v", err, tt.wantErr)
            }
        })
    }
}
```

### 测试覆盖率

- 核心模块覆盖率 ≥ 80%
- 整体覆盖率 ≥ 50%
- 关键路径必须覆盖

```bash
# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## API 设计规范

### RESTful 设计

```go
// ✅ 正确：资源名使用复数
GET    /api/v1/volumes
POST   /api/v1/volumes
GET    /api/v1/volumes/:name
PUT    /api/v1/volumes/:name
DELETE /api/v1/volumes/:name

// ❌ 错误：动词形式
GET    /api/v1/getVolumes
POST   /api/v1/createVolume
```

### 响应格式

```go
// 成功响应
type Response struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

// 错误响应
type ErrorResponse struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Error   string `json:"error,omitempty"`
}

// 分页响应
type PageResponse struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data"`
    Total   int64       `json:"total"`
    Page    int         `json:"page"`
    Size    int         `json:"size"`
}
```

### HTTP 状态码

| 状态码 | 用途 |
|--------|------|
| 200 OK | 成功 |
| 201 Created | 创建成功 |
| 400 Bad Request | 请求参数错误 |
| 401 Unauthorized | 未认证 |
| 403 Forbidden | 无权限 |
| 404 Not Found | 资源不存在 |
| 500 Internal Server Error | 服务器错误 |

---

## 数据库规范

### SQL 命名

```sql
-- ✅ 正确：小写、下划线
CREATE TABLE storage_volumes (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ❌ 错误：驼峰或大写
CREATE TABLE StorageVolumes (
    Id          INTEGER PRIMARY KEY,
    Name        TEXT NOT NULL,
    CreatedAt   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 事务处理

```go
// ✅ 正确：使用事务
func (r *Repository) TransferVolume(ctx context.Context, from, to string) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    if err := r.doTransfer(ctx, tx, from, to); err != nil {
        return err
    }
    
    return tx.Commit()
}
```

---

## 文档规范

### 包注释

```go
// Package storage 提供 BTRFS 存储管理功能。
//
// 支持以下操作：
//   - 卷管理：创建、删除、查询
//   - 子卷管理：创建、删除、快照
//   - 快照管理：创建、恢复、删除
//
// 示例：
//
//	mgr := storage.NewManager(config)
//	vol, err := mgr.CreateVolume("mydata", "/dev/sda1")
package storage
```

### 函数注释

```go
// CreateVolume 创建新的存储卷。
//
// 参数：
//   - name: 卷名称，必须符合命名规范
//   - device: 设备路径，如 /dev/sda1
//
// 返回：
//   - *Volume: 创建的卷信息
//   - error: 创建失败时返回错误
//
// 错误：
//   - ErrVolumeExists: 卷已存在
//   - ErrInvalidName: 名称无效
//
// 示例：
//
//	vol, err := mgr.CreateVolume("mydata", "/dev/sda1")
//	if err != nil {
//	    log.Fatal(err)
//	}
func (m *Manager) CreateVolume(name, device string) (*Volume, error) {
    // ...
}
```

---

## 工具配置

### golangci-lint

项目根目录 `.golangci.yml`:

```yaml
run:
  timeout: 5m
  skip-dirs:
    - vendor
    - tests/e2e

linters:
  enable:
    - gofmt
    - goimports
    - govet
    - errcheck
    - staticcheck
    - ineffassign
    - typecheck
    - gosimple
    - goconst
    - gocyclo
    - dupl

linters-settings:
  gocyclo:
    min-complexity: 15
  goconst:
    min-len: 3
    min-occurrences: 3
```

### 运行检查

```bash
# 格式化
gofmt -w .

# 静态检查
golangci-lint run

# 修复可自动修复的问题
golangci-lint run --fix
```

---

*最后更新: 2026-03-15*